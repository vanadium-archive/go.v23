// Package verror extends the regular error mechanism to work across different
// address spaces.
//
// To define a new error identifier, for example "someNewError", client code is
// expected to declare a variable like this:
//      var someNewError = verror.Register("my/package/name.someNewError", NoRetry,
//                                         "{1} {2} English text for new error")
// Error identifier strings should start with the package path to ensure uniqueness.
// Text for other languages can be added to the default i18n Catalogue.
//
// If the error should cause a client to retry, consider replacing "NoRetry" with
// one of the other Action codes below.   The purpose of Action is so
// clients not familiar with an error can retry appropriately.
//
// Errors are given parameters when used.  Conventionally, the first parameter
// is the name of the component (typically server or binary name), and the
// second is the name of the operation (such as an RPC or subcommand) that
// encountered the error.  Other parameters typically identify the object(s) on
// which the error occurred.  This convention is normally applied by New(),
// which fetches the language, component name and operation name from the
// context.T:
//      err = verror.New(someNewError, ctx, "object_on_which_error_occurred")
//
// The ExplicitNew() call can be used to specify these things explicitly:
//      err = verror.ExplicitNew(someNewError, i18n.LangIDFromContext(ctx),
//              "my_component", "op_name", "procedure_name", "object_name")
// If the language, component and/or operation name are unknown, use i18n.NoLangID
// or the empty string, respectively.
//
// Because of the convention for the first two parameters, messages in the
// catalogue typically look like this (at least for left-to-right languages):
//      {1} {2} The new error {_}
// The tokens {1}, {2}, etc.  refer to the first and second positional parameters
// respectively, while {_} is replaced by the positional parameters not
// explicitly referred to elsewhere in the message.  Thus, given the parameters
// above, this would lead to the output:
//      my_component op_name The new error object_name
//
// If a substring is of the form {:<number>}, {<number>:}, {:<number>:}, {:_},
// {_:}, or {:_:}, and the corresponding parameters are not the empty string,
// the parameter is preceded by ": " or followed by ":" or both,
// respectively.  For example, if the format:
//      {3:} foo {2} bar{:_} ({3})
// is used with the cat.Format example above, it yields:
//      3rd: foo 2nd bar: 1st 4th (3rd)
//
// The Convert() and ExplicitConvert() calls are like New() and ExplicitNew(),
// but convert existing errors (with their parameters, if applicable)
// to verror errors with given language, component name, and operation name,
// if non-empty values for these are provided.  They also add a PC to a
// list of PC values to assist developers hunting for the error.
//
// If the context.T specified with New() or Convert() is nil, a default
// context is used, set by SetDefaultContext().  This can be used in standalone
// programmes, or in anciliary threads not associated with an RPC.  The user
// might do the following to get the language from the environment, and the
// programme name from Args[0]:
//     ctx := runtime.NewContext()
//     ctx = i18n.ContextWithLangID(ctx, i18n.LangIDFromEnv())
//     ctx = verror.ContextWithComponentName(ctx, os.Args[0])
//     verror.SetDefaultContext(ctx)
// A standalone tool might set the operation name to be a subcommand name, if
// any.  If the default context has not been set, the error generated has no
// language, component and operation values; they will be filled in by the
// first Convert() call that does have these values.
package verror

import "bytes"
import "fmt"
import "io"
import "os"
import "path/filepath"
import "runtime"

import "sync"

import "v.io/v23/context"
import "v.io/v23/i18n"
import "v.io/v23/vtrace"

// ID is a unique identifier for errors.
type ID string

// An ActionCode represents the action expected to be performed by a typical client
// receiving an error that perhaps it does not understand.
type ActionCode uint32

// Codes for ActionCode.
const (
	// Retry actions are encoded in the bottom few bits.
	RetryActionMask ActionCode = 3

	NoRetry         ActionCode = 0 // Do not retry.
	RetryConnection ActionCode = 1 // Renew high-level connection/context.
	RetryRefetch    ActionCode = 2 // Refetch and retry (e.g., out of date HTTP ETag)
	RetryBackoff    ActionCode = 3 // Backoff and retry a finite number of times.
)

// RetryAction returns the part of the ActionCode that indicates retry behaviour.
func (ac ActionCode) RetryAction() ActionCode {
	return ac & RetryActionMask
}

// String returns the string label of x.
func (x ActionCode) String() string {
	switch x {
	case NoRetry:
		return "NoRetry"
	case RetryConnection:
		return "RetryConnection"
	case RetryRefetch:
		return "RetryRefetch"
	case RetryBackoff:
		return "RetryBackoff"
	}
	return fmt.Sprintf("ActionCode(%d)", x)
}

// RetryActionFromString creates a retry ActionCode from the string label.
func RetryActionFromString(label string) (ActionCode, error) {
	switch label {
	case "NoRetry":
		return NoRetry, nil
	case "RetryConnection":
		return RetryConnection, nil
	case "RetryRefetch":
		return RetryRefetch, nil
	case "RetryBackoff":
		return RetryBackoff, nil
	}
	return ActionCode(0), New(ErrBadArg, nil, label)
}

// An IDAction combines a unique identifier ID for errors with an ActionCode.
// The ID allows stable error checking across different error messages and
// different address spaces.  By convention the format for the identifier is
// "PKGPATH.NAME" - e.g. ErrIDFoo defined in the "veyron2/verror" package has id
// "veyron2/verror.ErrIDFoo".  It is unwise ever to create two IDActions that
// associate different ActionCodes with the same ID.
type IDAction struct {
	ID     ID
	Action ActionCode
}

// Register returns a IDAction with the given ID and Action fields, and
// inserts a message into the default i18n Catalogue in US English.
// Other languages can be added by adding to the Catalogue.
func Register(id ID, action ActionCode, englishText string) IDAction {
	i18n.Cat().SetWithBase(defaultLangID(i18n.NoLangID), i18n.MsgID(id), englishText)
	return IDAction{id, action}
}

// Standard is the in-memory representation of a verror error.
//
// The wire representation is defined as vdl.WireError; values of the Standard
// type are automatically converted to/from vdl.WireError by VDL and VOM.
type Standard struct {
	IDAction  IDAction
	Msg       string        // Error message; empty if no language known.
	ParamList []interface{} // The variadic parameters given to ExplicitNew().
	stackPCs  []uintptr     // PCs of creators of Standard
}

const maxPCs = 40 // Maximum number of PC values we'll include in a stack trace.

// A SubErrs is a special type that allows clients to include a list of
// subordinate errors to an error's parameter list.  Clients can add a SubErrs
// to the parameter list directly, via New() of include one in an existing
// error using AddSubErrs().  Each element of the slice has a name, an error,
// and an integer that encodes options such as verror.Print as bits set within
// it.  By convention, clients are expected to use name of the form "X=Y" to
// distinguish their subordinate errors from those of other abstraction layers.
// For example, a layer reporting on errors in individual blessings in an RPC
// might use strings like "blessing=<bessing_name>".
type SubErrs []SubErr

type SubErrOpts uint32

// A SubErr represents a (string, error, int32) triple,  It is the element type for SubErrs.
type SubErr struct {
	Name    string
	Err     error
	Options SubErrOpts
}

const (
	// Print, when set in SubErr.Options, tells Error() to print this SubErr.
	Print SubErrOpts = 0x1
)

func assertStandard(err error) (Standard, bool) {
	if e, ok := err.(Standard); ok {
		return e, true
	}
	if e, ok := err.(*Standard); ok && e != nil {
		return *e, true
	}
	return Standard{}, false
}

// ErrorID returns the ID of the given err, or Unknown if the err has no ID.
// If err is nil then ErrorID returns "".
func ErrorID(err error) ID {
	if err == nil {
		return ""
	}
	if e, ok := assertStandard(err); ok {
		return e.IDAction.ID
	}
	return ErrUnknown.ID
}

// Action returns the action of the given err, or NoRetry if the err has no Action.
func Action(err error) ActionCode {
	if err == nil {
		return NoRetry
	}
	if e, ok := assertStandard(err); ok {
		return e.IDAction.Action
	}
	return NoRetry
}

// Is returns true iff the given err has the given ID.
func Is(err error, id ID) bool {
	return ErrorID(err) == id
}

// Equal returns true iff a and b have the same error ID.
func Equal(a, b error) bool {
	return ErrorID(a) == ErrorID(b)
}

// PCs represents a list of PC locations
type PCs []uintptr

// Stack returns the list of PC locations on the stack when this error was
// first generated within this address space, or an empty list if err is not a
// Standard.
func Stack(err error) PCs {
	if err != nil {
		if e, ok := assertStandard(err); ok {
			stackIntPtr := make([]uintptr, len(e.stackPCs))
			copy(stackIntPtr, e.stackPCs)
			return stackIntPtr
		}
	}
	return nil
}

func (st PCs) String() string {
	buf := bytes.NewBufferString("")
	StackToText(buf, st)
	return buf.String()
}

// stackToTextIndent emits on w a text representation of stack, which is typically
// obtained from Stack() and represents the source location(s) where an
// error was generated or passed through in the local address space.
// indent is added to a prefix of each line printed.
func stackToTextIndent(w io.Writer, stack []uintptr, indent string) (err error) {
	for i := 0; i != len(stack) && err == nil; i++ {
		fnc := runtime.FuncForPC(stack[i])
		file, line := fnc.FileLine(stack[i])
		_, err = fmt.Fprintf(w, "%s%s:%d: %s\n", indent, file, line, fnc.Name())
	}
	return err
}

// StackToText emits on w a text representation of stack, which is typically
// obtained from Stack() and represents the source location(s) where an
// error was generated or passed through in the local address space.
func StackToText(w io.Writer, stack []uintptr) error {
	return stackToTextIndent(w, stack, "")
}

// defaultLangID returns langID is it is not i18n.NoLangID, and the default
// value of US English otherwise.
func defaultLangID(langID i18n.LangID) i18n.LangID {
	if langID == i18n.NoLangID {
		langID = "en-US"
	}
	return langID
}

func isDefaultIDAction(idAction IDAction) bool {
	return idAction.ID == "" && idAction.Action == 0
}

// makeInternal is like ExplicitNew(), but takes a slice of PC values as an argument,
// rather than constructing one from the caller's PC.
func makeInternal(idAction IDAction, langID i18n.LangID, componentName string, opName string, stack []uintptr, v ...interface{}) Standard {
	msg := ""
	params := append([]interface{}{componentName, opName}, v...)
	if langID != i18n.NoLangID {
		id := idAction.ID
		if id == "" {
			id = ErrUnknown.ID
		}
		msg = i18n.Cat().Format(langID, i18n.MsgID(id), params...)
	}
	return Standard{idAction, msg, params, stack}
}

// ExplicitNew returns an error with the given ID, with an error string in the chosen
// language.  The component and operation name are included the first and second
// parameters of the error.  Other parameters are taken from v[].  The
// parameters are formatted into the message according to i18n.Cat().Format.
// The caller's PC is added to the error's stack.
func ExplicitNew(idAction IDAction, langID i18n.LangID, componentName string, opName string, v ...interface{}) error {
	stack := make([]uintptr, maxPCs)
	stack = stack[:runtime.Callers(2, stack)]
	return makeInternal(idAction, langID, componentName, opName, stack, v...)
}

// A componentKey is used as a key for context.T's Value() map.
type componentKey struct{}

// ContextWithComponentName returns a context based on ctx that has the
// componentName that New() and Convert() can use.
func ContextWithComponentName(ctx *context.T, componentName string) *context.T {
	return context.WithValue(ctx, componentKey{}, componentName)
}

// New is like ExplicitNew(), but obtains the language, component name, and operation
// name from the specified context.T.   ctx may be nil.
func New(idAction IDAction, ctx *context.T, v ...interface{}) error {
	langID, componentName, opName := dataFromContext(ctx)
	stack := make([]uintptr, maxPCs)
	stack = stack[:runtime.Callers(2, stack)]
	return makeInternal(idAction, langID, componentName, opName, stack, v...)
}

// isEmptyString() returns whether v is an empty string.
func isEmptyString(v interface{}) bool {
	str, isAString := v.(string)
	return isAString && str == ""
}

// convertInternal is like ExplicitConvert(), but takes a slice of PC values as an argument,
// rather than constructing one from the caller's PC.
func convertInternal(idAction IDAction, langID i18n.LangID, componentName string, opName string, stack []uintptr, err error) Standard {
	// If err is already a verror.Standard, we wish to:
	//  - retain all set parameters.
	//  - if not yet set, set parameters 0 and 1 from componentName and
	//    opName.
	//  - if langID is set, and we have the appropriate language's format
	//    in our catalogue, use it.  Otheriwse, retain a message (assuming
	//    additional parameters were not set) even if the language is not
	//    correct.
	if e, ok := assertStandard(err); ok {
		oldParams := e.ParamList

		// Convert all embedded Standard erors, recursively.
		for i := range oldParams {
			if subErr, isStandard := oldParams[i].(Standard); isStandard {
				oldParams[i] = convertInternal(idAction, langID, componentName, opName, stack, subErr)
			} else if subErrs, isSubErrs := oldParams[i].(SubErrs); isSubErrs {
				for j := range subErrs {
					if subErr, isStandard := subErrs[j].Err.(Standard); isStandard {
						subErrs[j].Err = convertInternal(idAction, langID, componentName, opName, stack, subErr)
					}
				}
				oldParams[i] = subErrs
			}
		}

		// Create a non-empty format string if we have the language in the catalogue.
		var formatStr string
		if langID != i18n.NoLangID {
			id := e.IDAction.ID
			if id == "" {
				id = ErrUnknown.ID
			}
			formatStr = i18n.Cat().Lookup(langID, i18n.MsgID(id))
		}

		// Ignore the caller-supplied component and operation if we already have them.
		if componentName != "" && len(oldParams) >= 1 && !isEmptyString(oldParams[0]) {
			componentName = ""
		}
		if opName != "" && len(oldParams) >= 2 && !isEmptyString(oldParams[1]) {
			opName = ""
		}

		var msg string
		var newParams []interface{}
		if componentName == "" && opName == "" {
			if formatStr == "" {
				return e // Nothing to change.
			} else { // Parameter list does not change.
				newParams = e.ParamList
				msg = i18n.FormatParams(formatStr, newParams...)
			}
		} else { // Replace at least one of the first two parameters.
			newLen := len(oldParams)
			if newLen < 2 {
				newLen = 2
			}
			newParams = make([]interface{}, newLen)
			copy(newParams, oldParams)
			if componentName != "" {
				newParams[0] = componentName
			}
			if opName != "" {
				newParams[1] = opName
			}
			if formatStr != "" {
				msg = i18n.FormatParams(formatStr, newParams...)
			}
		}
		return Standard{e.IDAction, msg, newParams, e.stackPCs}
	}
	return makeInternal(idAction, langID, componentName, opName, stack, err.Error())
}

// ExplicitConvert converts a regular err into a Standard error, setting its id to id.  If
// err is already a Standard, it returns err or an equivalent value without changing its type, but
// potentially changing the language, component or operation if langID!=i18n.NoLangID,
// componentName!="" or opName!="" respectively.  The caller's PC is added to the
// error's stack.
func ExplicitConvert(idAction IDAction, langID i18n.LangID, componentName string, opName string, err error) error {
	if err == nil {
		return nil
	} else {
		var stack []uintptr
		if _, isStandard := assertStandard(err); !isStandard { // Walk the stack only if convertInternal will allocate a Standard.
			stack = make([]uintptr, maxPCs)
			stack = stack[:runtime.Callers(2, stack)]
		}
		return convertInternal(idAction, langID, componentName, opName, stack, err)
	}
}

// defaultCtx is the context used when a nil context.T is passed to New() or Convert().
var (
	defaultCtx     *context.T
	defaultCtxLock sync.RWMutex // Protects defaultCtx.
)

// SetDefaultContext sets the default context used when a nil context.T is
// passed to New() or Convert().  It is typically used in standalone
// programmes that have no RPC context, or in servers for the context of
// ancillary threads not associated with any particular RPC.
func SetDefaultContext(ctx *context.T) {
	defaultCtxLock.Lock()
	defaultCtx = ctx
	defaultCtxLock.Unlock()
}

// Convert is like ExplicitConvert(), but obtains the language, component and operation names
// from the specified context.T.   ctx may be nil.
func Convert(idAction IDAction, ctx *context.T, err error) error {
	if err == nil {
		return nil
	}
	langID, componentName, opName := dataFromContext(ctx)
	var stack []uintptr
	if _, isStandard := assertStandard(err); !isStandard { // Walk the stack only if convertInternal will allocate a Standard.
		stack = make([]uintptr, maxPCs)
		stack = stack[:runtime.Callers(2, stack)]
	}
	return convertInternal(idAction, langID, componentName, opName, stack, err)
}

// dataFromContext reads the languageID, component name, and operation name
// from the context, using defaults as appropriate.
func dataFromContext(ctx *context.T) (langID i18n.LangID, componentName string, opName string) {
	// Use a default context if ctx is nil.  defaultCtx may also be nil, so
	// further nil checks are required below.
	if ctx == nil {
		defaultCtxLock.RLock()
		ctx = defaultCtx
		defaultCtxLock.RUnlock()
	}
	if ctx != nil {
		langID = i18n.LangIDFromContext(ctx)
		value := ctx.Value(componentKey{})
		componentName, _ = value.(string)
		opName = vtrace.GetSpan(ctx).Name()
	}
	if componentName == "" {
		componentName = filepath.Base(os.Args[0])
	}
	return defaultLangID(langID), componentName, opName
}

// Error returns the error message; if it has not been formatted for a specific
// language, a default message containing the error ID and parameters is
// generated.  This method is required to fulfil the error interface.
func (e Standard) Error() string {
	msg := e.Msg
	if isDefaultIDAction(e.IDAction) && msg == "" {
		msg = i18n.Cat().Format(i18n.NoLangID, i18n.MsgID(ErrUnknown.ID), e.ParamList...)
	} else if msg == "" {
		msg = i18n.Cat().Format(i18n.NoLangID, i18n.MsgID(e.IDAction.ID), e.ParamList...)
	}
	return msg
}

// String is the default printing function for SubErrs
func (subErrs SubErrs) String() (result string) {
	if len(subErrs) > 0 {
		sep := ""
		for _, s := range subErrs {
			if (s.Options & Print) != 0 {
				result += fmt.Sprintf("%s[%s: %s]", sep, s.Name, s.Err.Error())
				sep = ", "
			}
		}
	}
	return result
}

// params returns the variadic arguments to ExplicitNew().  We do not export it
// to discourage its abuse as a general-purpose way to pass alternate return
// values.
func params(err error) []interface{} {
	if e, ok := assertStandard(err); ok {
		return e.ParamList
	}
	return nil
}

// subErrorIndex returns index of the first SubErrs in e.ParamList
// or len(e.ParamList) if there is no such parameter.
func (e Standard) subErrorIndex() (i int) {
	for i = range e.ParamList {
		if _, isSubErrs := e.ParamList[i].(SubErrs); isSubErrs {
			return i
		}
	}
	return len(e.ParamList)
}

// subErrors returns a copy of the subordinate errors accumulated into err, or
// nil if there are no such errors.  We do not export it to discourage its
// abuse as a general-purpose way to pass alternate return values.
func subErrors(err error) (r SubErrs) {
	if e, ok := assertStandard(err); ok {
		size := 0
		for i := range e.ParamList {
			if subErrsParam, isSubErrs := e.ParamList[i].(SubErrs); isSubErrs {
				size += len(subErrsParam)
			}
		}
		if size != 0 {
			r = make(SubErrs, 0, size)
			for i := range e.ParamList {
				if subErrsParam, isSubErrs := e.ParamList[i].(SubErrs); isSubErrs {
					r = append(r, subErrsParam...)
				}
			}
		}
	}
	return r
}

// addSubErrsInternal returns a copy of err with supplied errors appended as
// subordinate errors.  Requires that errors[i].Err!=nil for 0<=i<len(errors).
func addSubErrsInternal(err error, langID i18n.LangID, componentName string, opName string, stack []uintptr, errors SubErrs) error {
	var pe *Standard
	var e Standard
	var ok bool
	if pe, ok = err.(*Standard); ok {
		e = *pe
	} else if e, ok = err.(Standard); !ok {
		panic("non-verror.Standard passed to verror.AddSubErrs")
	}
	var subErrs SubErrs
	index := e.subErrorIndex()
	copy(e.ParamList, e.ParamList)
	if index == len(e.ParamList) {
		e.ParamList = append(e.ParamList, subErrs)
	} else {
		copy(subErrs, e.ParamList[index].(SubErrs))
	}
	for _, subErr := range errors {
		if _, ok := assertStandard(subErr.Err); ok {
			subErrs = append(subErrs, subErr)
		} else {
			subErr.Err = convertInternal(IDAction{}, langID, componentName, opName, stack, subErr.Err)
			subErrs = append(subErrs, subErr)
		}
	}
	e.ParamList[index] = subErrs
	if langID != i18n.NoLangID {
		e.Msg = i18n.Cat().Format(langID, i18n.MsgID(e.IDAction.ID), e.ParamList...)
	}
	return e
}

// ExplicitAddSubErrs returns a copy of err with supplied errors appended as
// subordinate errors.  Requires that errors[i].Err!=nil for 0<=i<len(errors).
func ExplicitAddSubErrs(err error, langID i18n.LangID, componentName string, opName string, errors ...SubErr) error {
	stack := make([]uintptr, maxPCs)
	stack = stack[:runtime.Callers(2, stack)]
	return addSubErrsInternal(err, langID, componentName, opName, stack, errors)
}

// AddSubErrs is like ExplicitAddSubErrs, but uses the provided context
// to obtain the langID, componentName, and opName values.
func AddSubErrs(err error, ctx *context.T, errors ...SubErr) error {
	stack := make([]uintptr, maxPCs)
	stack = stack[:runtime.Callers(2, stack)]
	langID, componentName, opName := dataFromContext(ctx)
	return addSubErrsInternal(err, langID, componentName, opName, stack, errors)
}

// debugStringInternal returns a more verbose string representation of an
// error, perhaps more thorough than one might present to an end user, but
// useful for debugging by a developer.  It prefixes all lines output with
// "prefix" and "name" (if non-empty) and adds intent to prefix wen recursing.
func debugStringInternal(err error, prefix string, name string) string {
	str := prefix
	if len(name) > 0 {
		str += name + " "
	}
	str += err.Error()
	// Append err's stack, indented a little.
	prefix += "  "
	buf := bytes.NewBufferString("")
	stackToTextIndent(buf, Stack(err), prefix)
	str += "\n" + buf.String()
	// Print all the subordinate errors, even the ones that were not
	// printed by Error(), indented a bit further.
	prefix += "  "
	if subErrs := subErrors(err); len(subErrs) > 0 {
		for _, s := range subErrs {
			str += debugStringInternal(s.Err, prefix, s.Name)
		}
	}
	return str
}

// DebugString returns a more verbose string representation of an error,
// perhaps more thorough than one might present to an end user, but useful for
// debugging by a developer.
func DebugString(err error) string {
	return debugStringInternal(err, "", "")
}
