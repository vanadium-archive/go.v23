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
// the parameter is preceded by ": " or followed by ":"  or both,
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
import "v.io/core/veyron2/verror" // While converting from verror to verror2
import "v.io/core/veyron2/vtrace"

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

// An IDAction combines a unique identifier ID for errors with an ActionCode.  The
// ID allows stable error checking across different error messages and
// different address spaces.  By convention the format for the identifier is
// "PKGPATH.NAME" - e.g. ErrIDFoo defined in the "veyron2/verror" package has
// id "veyron2/verror.ErrIDFoo".  It is unwise ever to create two IDActions that
// associate different ActionCodes with the same ID.
type IDAction struct {
	ID     verror.ID
	Action ActionCode
}

// Register returns a IDAction with the given ID and Action fields, and
// inserts a message into the default i18n Catalogue in US English.
// Other languages can be added by adding to the Catalogue.
func Register(id verror.ID, action ActionCode, englishText string) IDAction {
	i18n.Cat().SetWithBase(defaultLangID(i18n.NoLangID), i18n.MsgID(id), englishText)
	return IDAction{id, action}
}

// E extends the regular error interface with an error ID,
// some parameters, a call stack for local debugging and the ability
// to aggregate mutliple 'subordinate', errors. The Error method returns
// a string containing the subordinate errors as well as the main one.
type E interface {
	error

	// ErrorID returns the ID.
	ErrorID() verror.ID

	// Action indicates whether a client should retry.
	Action() ActionCode

	// Params returns the list of parameters given to ExplicitMake().  The
	// expectation is that the first parameter will be a string that
	// identify the component (typically the server or binary) where the
	// error was generated, the second will be a string that identifies the
	// operation that encountered the error.  The remaining parameters
	// typically identify the object(s) on which the operation was acting.
	// A component passsing on an error from another may prefix the first
	// parameter with its name, a colon and a space.
	Params() []interface{}

	// HasMessage() returns whether a language-specific error string has
	// been created.
	HasMessage() bool

	// Stack returns list of PC values where ExplicitMake/Make/Context/ContextCtx
	// were invoked on the error.  StackToStr() converts this to a string.
	// This information is not incorporated into the error string;
	// it is intended to help debugging by developers.
	Stack() PCs

	// SubErrors returns all of the subordinate errors in the order that
	// they were added to E.
	SubErrors() []E

	// Returns a string containing the error message, stack traces etcs.
	DebugString() string
}

// ErrorID returns the ID of the given err, or Unknown if the err has no ID.
// If err is nil then ErrorID returns "".
func ErrorID(err error) verror.ID {
	if err == nil {
		return ""
	}
	if e, ok := err.(E); ok {
		return e.ErrorID()
	}

	// The following is to aid conversion from verror to verror2
	if e, _ := err.(verror.E); e != nil {
		return e.ErrorID()
	}
	// End conversion aid.

	return Unknown.ID
}

// Action returns the action of the given err, or NoRetry if the err has no Action.
func Action(err error) ActionCode {
	if err == nil {
		return NoRetry
	}
	if e, ok := err.(E); ok {
		return e.Action()
	}
	return NoRetry
}

// Is returns true iff the given err has the given ID.
func Is(err error, id verror.ID) bool {
	return ErrorID(err) == id
}

// Equal returns true iff a and b have the same error ID.
func Equal(a, b error) bool {
	return ErrorID(a) == ErrorID(b)
}

// PCs represents a list of PC locations
type PCs []uintptr

// Stack returns the list of PC locations where the error was created and transferred
// within this address space, or an empty list if err does not implement E.
func Stack(err error) PCs {
	if err != nil {
		if e, ok := err.(E); ok {
			return e.Stack()
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
func makeInternal(idAction IDAction, langID i18n.LangID, componentName string, opName string, stack []uintptr, v ...interface{}) E {
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
func ExplicitMake(idAction IDAction, langID i18n.LangID, componentName string, opName string, v ...interface{}) E {
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
func Make(idAction IDAction, ctx *context.T, v ...interface{}) E {
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
func convertInternal(idAction IDAction, langID i18n.LangID, componentName string, opName string, stack []uintptr, err error) E {
	if err == nil {
		return nil
	}

	// If err is already a verror2.E, we wish to:
	//  - retain all set parameters.
	//  - if not yet set, set parameters 0 and 1 from componentName and
	//    opName.
	//  - if langID is set, and we have the appropriate language's format
	//    in our catalogue, use it.  Otheriwse, retain a message (assuming
	//    additional parameters were not set) even if the language is not
	//    correct.
	if e, _ := err.(E); e != nil {
		oldParams := e.Params()

		// Create a non-empty format string if we have the language in the catalogue.
		var formatStr string
		if langID != i18n.NoLangID {
			id := e.ErrorID()
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
				newParams = e.Params()
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
		eStack := e.Stack()
		newStack := append(make([]uintptr, 0, len(eStack)+len(stack)), eStack...)
		newStack = append(newStack, stack...)
		return Standard{IDAction{e.ErrorID(), e.Action()}, msg, newParams, newStack, nil}
	}

	// The following is to aid conversion from verror to verror2
	if _, ok := err.(verror.E); ok {
		return makeInternal(IDAction{ErrorID(err), NoRetry}, langID, componentName, opName, stack, err.Error())
	}
	// End conversion aid.

	return makeInternal(idAction, langID, componentName, opName, stack, err.Error())
}

// ExplicitConvert converts a regular err into an E error, setting its id to id.  If
// err is already an E, it returns err without changing its type, but
// potentially changing the language, component or operation if langID!=i18n.NoLangID,
// componentName!="" or opName!="" respectively.  The caller's PC is added to the
// error's stack.
func ExplicitConvert(idAction IDAction, langID i18n.LangID, componentName string, opName string, err error) E {
	stack := make([]uintptr, 1)
	runtime.Callers(2, stack)
	return convertInternal(idAction, langID, componentName, opName, stack, err)
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
func Convert(idAction IDAction, ctx *context.T, err error) E {
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
		opName = vtrace.FromContext(ctx).Name()
	}
	if componentName == "" {
		componentName = filepath.Base(os.Args[0])
	}
	return defaultLangID(langID), componentName, opName
}

// Standard is a standard implementation of E.
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
	subErrs   []E           // Subordinate errors
}

// ErrorID returns the error id.
func (e Standard) ErrorID() verror.ID {
	return e.IDAction.ID
}

// Action returns the Action.
func (e Standard) Action() ActionCode {
	return e.IDAction.Action
}

// Error returns the error message; if it has not been formatted for a specific
// language, a default message containing the error ID and parameters is
// generated.
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

// Params returns the variadic arguments to E.ExplicitMake().
func (e Standard) Params() []interface{} {
	return e.ParamList
}

// HasMessage returns whether the error message has been formatted for a
// specific language.
func (e Standard) HasMessage() bool {
	return e.Msg != ""
}

// Stack returns the set of PCs of callers of *Make() and *Convert().
func (e Standard) Stack() PCs {
	stackIntPtr := make([]uintptr, len(e.stackPCs))
	copy(stackIntPtr, e.stackPCs)
	return stackIntPtr
}

// SubErrors returns the subordinate errors accumulated into E, or nil
// if there are no such errors.
func (e Standard) SubErrors() []E {
	if len(e.subErrs) == 0 {
		return nil
	}
	r := make([]E, len(e.subErrs))
	copy(r, e.subErrs)
	return r
}

// Append returns a copy of err with the supplied errors appended
// as subordinate errors.
func Append(err E, errors ...error) E {
	e, ok := err.(Standard)
	if !ok {
		return err
	}
	n := Standard{
		IDAction:  e.IDAction,
		Msg:       e.Msg,
		ParamList: e.ParamList,
		stackPCs:  e.stackPCs,
		subErrs:   make([]E, len(e.subErrs), len(e.subErrs)+len(errors)),
	}
	copy(n.subErrs, e.subErrs)
	for _, err := range errors {
		if err == nil {
			// nothing
		} else if s, ok := err.(E); ok {
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

func (e Standard) DebugString() string {
	str := e.Error()
	str += "\n" + e.Stack().String()
	for _, s := range e.SubErrors() {
		str += s.Error() + "\n" + s.Stack().String()
	}
	return str
}

const pkgPath = "v.io/core/veyron2/verror"

var (
	// Unknown or Internal are intended for general use by
	// all system components. Unknown and Internal should only be used
	// when a more specific error is not available.
	// The default value of IDAction is mapped internally to Unknown.
	Unknown  = Register(pkgPath+".Unknown", NoRetry, "{1:}{2:} Error{:_}") // The unknown error.
	Internal = Register(pkgPath+".Internal", NoRetry, "{1:}{2:} Internal error{:_}")
	EOF      = Register(pkgPath+".EOF", NoRetry, "{1:}{2:} EOF{:_}")

	// BadArg is used when the parameters to an operation are invalid or
	// incorrectly formatted.
	// BadState is used when an method is called on an object with
	BadArg   = Register(pkgPath+".BadArg", NoRetry, "{1:}{2:} Bad argument{:_}")
	BadState = Register(pkgPath+".BadState", NoRetry, "{1:}{2:} Invalid state: {:_}")

	// Exist and NoExist are intended for use all system components that
	// have the notion of a 'name' or 'key' that may or may not exist. The
	// operation generating these errors should be retried with the same
	// parameters unless some other remediating action is taken first.
	// NoExistOrNoAccess is intended for use when the component does not
	// want to reveal the distinction between an object existing and being
	// inaccessible and an object existing at all.
	Exist             = Register(pkgPath+".Exist", NoRetry, "{1:}{2:} Already exists{:_}")
	NoExist           = Register(pkgPath+".NoExist", NoRetry, "{1:}{2:} Does not exist{:_}")
	NoExistOrNoAccess = Register(pkgPath+".NoExistOrNoAccess", NoRetry, "{1:}{2:} Does not exist or access denied{:_}")

	// The following errors can occur during the process of establishing
	// an RPC connection.
	// NoExist (see above) is returned if the name of the server fails to
	// resolve any addresses.
	// NoServers is returned when the servers returned for the supplied name
	// are somehow unusable or unreachable by the client.
	// NoAccess is returned when a server does not authorize a client.
	// NotTrusted is returned when a client does not trust a server.
	NoServers        = Register(pkgPath+".NoServers", RetryRefetch, "{1:}{2:} No usable servers found{:_}")
	NoAccess         = Register(pkgPath+".NoAccess", RetryRefetch, "{1:}{2:} Access denied{:_}")
	NotTrusted       = Register(pkgPath+"NotTrusted", RetryRefetch, "{1:}{2:} Client does not trust server{:_}")
	NoServersAndAuth = Register(pkgPath+".NoServersAndAuth", RetryRefetch, "{1:}{2:} Has no usable servers and is either not trusted or access was denied{:_}")

	// The following errors can occur for an RPC that is in process.
	// Aborted is returned when the framework or the server abort the
	// in process RPC.
	// BadProtocol is returned when some form of protocol or codec error
	// is encountered.
	// Cancelled is returned when the client or server cancel an RPC.
	// Timeout is returned when the client times out waiting for a response.
	Aborted     = Register(pkgPath+".Aborted", NoRetry, "{1:}{2:} Aborted{:_}")
	BadProtocol = Register(pkgPath+".BadProtocol", NoRetry, "{1:}{2:} Bad protocol or type{:_}")
	Cancelled   = Register(pkgPath+".Cancelled", NoRetry, "{1:}{2:} Cancelled{:_}")
	Timeout     = Register(pkgPath+".Timeout", NoRetry, "{1:}{2:} Timeout{:_}")
)
