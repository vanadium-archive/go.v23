// Package verror2 extends the regular error mechanism to work across different
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
// which the error occurred.  This convention is normally applied by Make(),
// which fetches the language, component name and operation name from the
// context.T:
//      err = verror2.Make(someNewError, ctx, "object_on_which_error_occurred")
//
// The ExplicitMake() call can be used to specify these things explicitly:
//      err = verror2.ExplicitMake(someNewError, i18n.LangIDFromContext(ctx),
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
// The Convert() and ExplicitConvert() calls are like Make() and ExplicitMake(),
// but convert existing errors (with their parameters, if applicable)
// to verror2 errors with given language, component name, and operation name,
// if non-empty values for these are provided.  They also add a PC to a
// list of PC values to assist developers hunting for the error.
//
// If the context.T specified with Make() or Convert() is nil, a default
// context is used, set by SetDefaultContext().  This can be used in standalone
// programmes, or in anciliary threads not associated with an RPC.  The user
// might do the following to get the language from the environment, and the
// programme name from Args[0]:
//     ctx := runtime.NewContext()
//     ctx = i18n.ContextWithLangID(ctx, i18n.LangIDFromEnv())
//     ctx = verror2.ContextWithComponentName(ctx, os.Args[0])
//     verror2.SetDefaultContext(ctx)
// A standalone tool might set the operation name to be a subcommand name, if
// any.  If the default context has not been set, the error generated has no
// language, component and operation values; they will be filled in by the
// first Convert() call that does have these values.
package verror2

import "bytes"
import "fmt"
import "io"
import "os"
import "path/filepath"
import "runtime"

import "sync"

import "v.io/core/veyron2/context"
import "v.io/core/veyron2/i18n"
import "v.io/core/veyron2/vtrace"

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
	return ActionCode(0), Make(BadArg, nil, label)
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

// An E is an error interface, but with an extra method that nother but a
// Standard implements.  It exists solely to aid in conversion, because the E
// interface existed before.
type E interface {
	error
	UniqueMethodName()
}

// A Standard is the representation of a verror2 error.
//
// This must be kept in sync with the vdl.ErrorType defined in
// v.io/core/veyron2/vdl.
//
// TODO(toddw): Move this definition to a common vdl file, and change it to be a
// better wire format (e.g. no nested structs).
type Standard struct {
	IDAction  IDAction
	Msg       string        // Error message; empty if no language known.
	ParamList []interface{} // The variadic parameters given to ExplicitMake().
	stackPCs  []uintptr     // PCs of callers of *Make() and *Convert().
	subErrs   []error       // Subordinate errors
}

// UniqueMethodName exists so that Standard will be the lone implementer of E.
func (x Standard) UniqueMethodName() {
}

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
	return Unknown.ID
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

// Stack returns the list of PC locations where the error was created and transferred
// within this address space, or an empty list if err is not a Standard.
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

// StackToText emits on w a text representation of stack, which is typically
// obtained from Stack() and represents the source location(s) where an
// error was generated or passed through in the local address space.
func StackToText(w io.Writer, stack []uintptr) (err error) {
	for _, pc := range stack {
		fnc := runtime.FuncForPC(pc)
		file, line := fnc.FileLine(pc)
		_, err = fmt.Fprintf(w, "%s:%d: %s\n", file, line, fnc.Name())
	}
	return err
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

// makeInternal is like ExplicitMake(), but takes a slice of PC values as an argument,
// rather than constructing one from the caller's PC.
func makeInternal(idAction IDAction, langID i18n.LangID, componentName string, opName string, stack []uintptr, v ...interface{}) Standard {
	msg := ""
	params := append([]interface{}{componentName, opName}, v...)
	if langID != i18n.NoLangID {
		id := idAction.ID
		if id == "" {
			id = Unknown.ID
		}
		msg = i18n.Cat().Format(langID, i18n.MsgID(id), params...)
	}
	return Standard{idAction, msg, params, stack, nil}
}

// ExplicitMake returns an error with the given ID, with an error string in the chosen
// language.  The component and operation name are included the first and second
// parameters of the error.  Other parameters are taken from v[].  The
// parameters are formatted into the message according to i18n.Cat().Format.
// The caller's PC is added to the error's stack.
func ExplicitMake(idAction IDAction, langID i18n.LangID, componentName string, opName string, v ...interface{}) error {
	stack := make([]uintptr, 1)
	runtime.Callers(2, stack)
	return makeInternal(idAction, langID, componentName, opName, stack, v...)
}

// A componentKey is used as a key for context.T's Value() map.
type componentKey struct{}

// ContextWithComponentName returns a context based on ctx that has the
// componentName that Make() and Convert() can use.
func ContextWithComponentName(ctx *context.T, componentName string) *context.T {
	return context.WithValue(ctx, componentKey{}, componentName)
}

// Make is like ExplicitMake(), but obtains the language, component name, and operation
// name from the specified context.T.   ctx may be nil.
func Make(idAction IDAction, ctx *context.T, v ...interface{}) error {
	langID, componentName, opName := dataFromContext(ctx)
	stack := make([]uintptr, 1)
	runtime.Callers(2, stack)
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
	// If err is already a verror2.Standard, we wish to:
	//  - retain all set parameters.
	//  - if not yet set, set parameters 0 and 1 from componentName and
	//    opName.
	//  - if langID is set, and we have the appropriate language's format
	//    in our catalogue, use it.  Otheriwse, retain a message (assuming
	//    additional parameters were not set) even if the language is not
	//    correct.
	if e, ok := assertStandard(err); ok {
		oldParams := e.ParamList

		// Create a non-empty format string if we have the language in the catalogue.
		var formatStr string
		if langID != i18n.NoLangID {
			id := e.IDAction.ID
			if id == "" {
				id = Unknown.ID
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
		newStack := append(make([]uintptr, 0, len(e.stackPCs)+len(stack)), e.stackPCs...)
		newStack = append(newStack, stack...)
		return Standard{e.IDAction, msg, newParams, newStack, nil}
	}
	return makeInternal(idAction, langID, componentName, opName, stack, err.Error())
}

// ExplicitConvert converts a regular err into a Standard error, setting its id to id.  If
// err is already a Standard, it returns err without changing its type, but
// potentially changing the language, component or operation if langID!=i18n.NoLangID,
// componentName!="" or opName!="" respectively.  The caller's PC is added to the
// error's stack.
func ExplicitConvert(idAction IDAction, langID i18n.LangID, componentName string, opName string, err error) error {
	if err == nil {
		return nil
	} else {
		stack := make([]uintptr, 1)
		runtime.Callers(2, stack)
		return convertInternal(idAction, langID, componentName, opName, stack, err)
	}
}

// defaultCtx is the context used when a nil context.T is passed to Make() or Convert().
var (
	defaultCtx     *context.T
	defaultCtxLock sync.RWMutex // Protects defaultCtx.
)

// SetDefaultContext sets the default context used when a nil context.T is
// passed to Make() or Convert().  It is typically used in standalone
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
	stack := make([]uintptr, 1)
	runtime.Callers(2, stack)
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
		return i18n.Cat().Format(i18n.NoLangID, i18n.MsgID(Unknown.ID), e.ParamList...)
	}
	if msg == "" {
		msg = i18n.Cat().Format(i18n.NoLangID, i18n.MsgID(e.IDAction.ID), e.ParamList...)
	}
	if len(e.subErrs) > 0 {
		str := fmt.Sprintf(" [%s]", e.subErrs[0].Error())
		for _, s := range e.subErrs[1:] {
			str += fmt.Sprintf(", [%s]", s.Error())
		}
		msg += str
	}
	return msg
}

// Params returns the variadic arguments to ExplicitMake().
func Params(err error) []interface{} {
	if e, ok := assertStandard(err); ok {
		return e.ParamList
	}
	return nil
}

// SubErrors returns the subordinate errors accumulated into err, or nil
// if there are no such errors.
func SubErrors(err error) (r []error) {
	if e, ok := assertStandard(err); ok && len(e.subErrs) != 0 {
		r = make([]error, len(e.subErrs))
		copy(r, e.subErrs)
	}
	return r
}

// Append returns a copy of err with the supplied errors appended
// as subordinate errors.
func Append(err error, errors ...error) error {
	e, ok := assertStandard(err)
	if !ok {
		return err
	}
	n := Standard{
		IDAction:  e.IDAction,
		Msg:       e.Msg,
		ParamList: e.ParamList,
		stackPCs:  e.stackPCs,
		subErrs:   make([]error, len(e.subErrs), len(e.subErrs)+len(errors)),
	}
	copy(n.subErrs, e.subErrs)
	for _, err := range errors {
		if err == nil {
			// nothing
		} else if s, ok := assertStandard(err); ok {
			n.subErrs = append(n.subErrs, s)
		} else {
			langID, componentName, opName := dataFromContext(nil)
			stack := make([]uintptr, 1)
			runtime.Callers(2, stack)
			s := convertInternal(IDAction{}, langID, componentName, opName, stack, err)
			n.subErrs = append(n.subErrs, s)
		}
	}
	return n
}

// DebugString returns a more full string representation of an error, perhaps
// more thorough than one might present to an end user, but useful for
// debugging by a developer.
func DebugString(err error) string {
	str := err.Error()
	str += "\n" + Stack(err).String()
	for _, s := range SubErrors(err) {
		str += s.Error() + "\n" + Stack(s).String()
	}
	return str
}
