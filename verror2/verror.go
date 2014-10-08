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
// or the emptry string, respectively.
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

import "fmt"
import "io"
import "runtime"
import "sync"
import "veyron.io/veyron/veyron2/vom" // For type registration
import "veyron.io/veyron/veyron2/context"
import "veyron.io/veyron/veyron2/i18n"
import "veyron.io/veyron/veyron2/verror" // While converting from verror to verror2

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
	i18n.Cat().SetWithBase(i18n.LangID("en-US"), i18n.MsgID(id), englishText)
	return IDAction{id, action}
}

// E extends the regular error interface with an error ID,
// some parameters, and a call stack for local debugging.
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
	// This information is not incorporating into the error string;
	// it is intended to help debugging by developers.
	Stack() []uintptr
}

// ErrorID returns the ID of the given err, or Unknown if the err has no ID.
func ErrorID(err error) verror.ID {
	if err == nil {
		return Success.ID
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

// Stack returns the list of PC locations where the error was created and transferred
// within this address space, or an empty list if err does not implement E.
func Stack(err error) []uintptr {
	if err != nil {
		if e, ok := err.(E); ok {
			return e.Stack()
		}
	}
	return nil
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

// makeInternal is like ExplicitMake(), but takes a slice of PC values as an argument,
// rather than constructing one from the caller's PC.
func makeInternal(idAction IDAction, langID i18n.LangID, componentName string, opName string, stack []uintptr, v ...interface{}) E {
	msg := ""
	params := append([]interface{}{componentName, opName}, v...)
	if langID != i18n.NoLangID {
		msg = i18n.Cat().Format(langID, i18n.MsgID(idAction.ID), params...)
	}
	return Standard{idAction, msg, params, stack}
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

// An opKey is used as a key for context.T's Value() map.
type opKey struct{}

// ContextWithComponentName returns a context based on ctx that has the
// componentName that Make() and Convert() can use.
func ContextWithComponentName(ctx context.T, componentName string) context.T {
	return ctx.WithValue(componentKey{}, componentName)
}

// ContextWithOpName returns a context based on ctx that has the
// opName that Make() and Convert() can use.
func ContextWithOpName(ctx context.T, opName string) context.T {
	return ctx.WithValue(opKey{}, opName)
}

// Make is like ExplicitMake(), but obtains the language, component name, and operation
// name from the specified context.T.   ctx may be nil.
func Make(idAction IDAction, ctx context.T, v ...interface{}) E {
	langID := i18n.LangIDFromContext(ctx)

	// Use a default context if ctx is nil.  defaultCtx may also be nil, so
	// further nil checks are required below.
	if ctx == nil {
		defaultCtxLock.RLock()
		ctx = defaultCtx
		defaultCtxLock.RUnlock()
	}

	// Attempt to get component name and operation from the context.
	var componentName string
	var opName string
	if ctx != nil {
		value := ctx.Value(componentKey{})
		componentName, _ = value.(string)
		value = ctx.Value(opKey{})
		opName, _ = value.(string)
	}

	stack := make([]uintptr, 1)
	runtime.Callers(2, stack)
	return makeInternal(idAction, langID, componentName, opName, stack, v...)
}

// convertInternal is like ExplicitConvert(), but takes a slice of PC values as an argument,
// rather than constructing one from the caller's PC.
func convertInternal(idAction IDAction, langID i18n.LangID, componentName string, opName string, stack []uintptr, err error) E {
	if err == nil {
		return nil
	}

	// if err is already a verror2.E:
	if e, _ := err.(E); e != nil {
		var newParams []interface{}
		if componentName == "" && opName == "" {
			if langID == i18n.NoLangID {
				return e // Nothing to change.
			} else { // Parameter list does not change.
				newParams = e.Params()
			}
		} else { // Replace first two parameters.
			oldParams := e.Params()
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
		}
		msg := ""
		if langID != i18n.NoLangID {
			msg = i18n.Cat().Format(langID, i18n.MsgID(e.ErrorID()), newParams...)
		}
		eStack := e.Stack()
		newStack := append(make([]uintptr, 0, len(eStack)+len(stack)), eStack...)
		newStack = append(newStack, stack...)
		return Standard{IDAction{e.ErrorID(), e.Action()}, msg, newParams, newStack}
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
	defaultCtx     context.T
	defaultCtxLock sync.RWMutex // Protects defaultCtx.
)

// SetDefaultContext sets the default context used when a nil context.T is
// passed to Make() or Convert().  It is typically used in standalone
// programmes that have no RPC context, or in servers for the context of
// ancillary threads not associated with any particular RPC.
func SetDefaultContext(ctx context.T) {
	defaultCtxLock.Lock()
	defaultCtx = ctx
	defaultCtxLock.Unlock()
}

// Convert is like ExplicitConvert(), but obtains the language, component and operation names
// from the specified context.T.   ctx may be nil.
func Convert(idAction IDAction, ctx context.T, err error) E {
	if err == nil {
		return nil
	}

	// Use a default context if ctx is nil.  defaultCtx may also be nil, so
	// further nil checks are required below.
	if ctx == nil {
		defaultCtxLock.RLock()
		ctx = defaultCtx
		defaultCtxLock.RUnlock()
	}

	// Attempt to get component name and operation from the context.
	var componentName string
	var opName string
	if ctx != nil {
		value := ctx.Value(componentKey{})
		componentName, _ = value.(string)
		value = ctx.Value(opKey{})
		opName, _ = value.(string)
	}
	stack := make([]uintptr, 1)
	runtime.Callers(2, stack)
	return convertInternal(idAction, i18n.LangIDFromContext(ctx), componentName, opName, stack, err)
}

// Standard is a standard implementation of E.
type Standard struct {
	IDAction  IDAction
	Msg       string        // Error message; empty if no language known.
	ParamList []interface{} // The variadic parameters given to ExplicitMake().
	stackPCs  []uintptr     // PCs of callers of *Make() and *Convert().
}

func init() {
	vom.Register(Standard{})
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
	if msg == "" {
		msg = i18n.Cat().Format(i18n.NoLangID, i18n.MsgID(e.IDAction.ID), e.ParamList...)
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
func (e Standard) Stack() []uintptr {
	stackIntPtr := make([]uintptr, len(e.stackPCs))
	copy(stackIntPtr, e.stackPCs)
	return stackIntPtr
}

const pkgPath = "veyron.io/veyron/veyron2/verror"

// Some standard error codes.
var (
	Success           = Register(pkgPath+".Success", NoRetry, "{1} {2} Success {_}") // Success; the nil error.
	Unknown           = Register(pkgPath+".Unknown", NoRetry, "{1} {2} Error {_}")   // The unknown error.
	Aborted           = Register(pkgPath+".Aborted", NoRetry, "{1} {2} Aborted {_}")
	BadArg            = Register(pkgPath+".BadArg", NoRetry, "{1} {2} Bad argument {_}")
	BadProtocol       = Register(pkgPath+".BadProtocol", NoRetry, "{1} {2} Bad protocol or type {_}")
	Exists            = Register(pkgPath+".Exists", NoRetry, "{1} {2} Already exists {_}")
	Internal          = Register(pkgPath+".Internal", NoRetry, "{1} {2} Internal error {_}")
	NoAccess          = Register(pkgPath+".NoAccess", NoRetry, "{1} {2} Access denied {_}")
	NoExist           = Register(pkgPath+".NoExist", NoRetry, "{1} {2} Does not exist {_}")
	NoExistOrNoAccess = Register(pkgPath+".NoExistOrNoAccess", NoRetry, "{1} {2} Does not exist or access denied {_}")
	Timeout           = Register(pkgPath+".Timeout", NoRetry, "{1} {2} Timeout {_}")
)
