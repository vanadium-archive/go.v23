// Package verror2 extends the regular error mechanism to work across different
// address spaces.
package verror2

import "veyron.io/veyron/veyron2/i18n"
import "veyron.io/veyron/veyron2/verror" // While converting from verror to verror2

// An Action represents the action expected to be performed by a typical client
// receiving an error that perhaps it does not understand.
type Action uint32

// Codes for Action.
const (
	// Retry actions are encoded in the bottom few bits.
	RetryActionMask Action = 3

	Failed            Action = 0 // Do not retry.
	ConnectionFailure Action = 1 // Renew high-level connection/context.
	OutDated          Action = 2 // Refetch and retry (e.g., out of date HTTP ETag)
	Backoff           Action = 3 // Backoff and retry a finite number of times.
)

// ID is a unique identifier for errors tied to an Action.  This allows stable
// error checking across different error messages and different address spaces.
// By convention the format for the identifier is "PKGPATH.NAME" - e.g.
// ErrIDFoo defined in the "veyron.io/veyron/veyron2/verror" package has id
// "veyron.io/veyron/veyron2/verror.ErrIDFoo".
type ID struct {
	MsgID  i18n.MsgID
	Action Action
}

// E extends the regular error interface with an error ID.
type E interface {
	error
	// ErrorID returns the error id.
	ErrorID() ID
}

// ErrorID returns the ID of the given err, or Unknown if the err has no ID.
func ErrorID(err error) ID {
	if err == nil {
		return Success
	}

	// The following is to aid conversion from verror to verror2
	if e, _ := err.(verror.E); e != nil {
		if verror.IsUnknown(e) {
			return Unknown
		} else {
			return ID{i18n.MsgID(e.ErrorID()), Failed}
		}
	}
	// End conversion aid.

	if e, ok := err.(E); ok {
		return e.ErrorID()
	}
	return Unknown
}

// Is returns true iff the given err has the given id.
func Is(err error, id ID) bool {
	return ErrorID(err) == id
}

// Equal returns true iff a and b have the same error ID.
func Equal(a, b error) bool {
	return ErrorID(a) == ErrorID(b)
}

// Make returns an error with the given ID, with an error string in the
// chosen language.   Positional parameters are placed in the message
// according to i18n.Cat().Format.
func Make(id ID, langID i18n.LangID, v ...interface{}) E {
	return standard{id, i18n.Cat().Format(langID, id.MsgID, v...)}
}

// Convert converts a regular err into an E error, setting its id to id.
// If err is already an E, it returns err without changing its type.
func Convert(id ID, langID i18n.LangID, err error) E {
	if err == nil {
		return nil
	}
	if e, _ := err.(E); e != nil {
		return e
	}

	// The following is to aid conversion from verror to verror2
	if _, ok := err.(verror.E); ok {
		return standard{ErrorID(err), err.Error()}
	}
	// End conversion aid.

	return Make(id, langID, "Error", err.Error())
}

// standard is a standard implementation of E.
type standard struct {
	ID  ID
	Msg string
}

// ErrorID returns the error id.
func (e standard) ErrorID() ID {
	return e.ID
}

// Error returns the error message.
func (e standard) Error() string {
	return e.Msg
}

// Some standard error codes.
var (
	Success       = ID{"veyron.io/veyron/veyron2/verror.Success", Failed}       // Success; the ErrorID of the nil error.
	Unknown       = ID{"veyron.io/veyron/veyron2/verror.Unknown", Failed}       // The unknown error.
	Aborted       = ID{"veyron.io/veyron/veyron2/verror.Aborted", Failed}       // Operation aborted, e.g. connection closed.
	BadArg        = ID{"veyron.io/veyron/veyron2/verror.BadArg", Failed}        // Requester specified an invalid argument.
	BadProtocol   = ID{"veyron.io/veyron/veyron2/verror.BadProtocol", Failed}   // Protocol mismatch, including type or argument errors.
	Exists        = ID{"veyron.io/veyron/veyron2/verror.Exists", Failed}        // Requested entity already exists.
	Internal      = ID{"veyron.io/veyron/veyron2/verror.Internal", Failed}      // Internal invariants broken; something is very wrong.
	NotAuthorized = ID{"veyron.io/veyron/veyron2/verror.NotAuthorized", Failed} // Requester isn't authorized to access the entity.
	NotFound      = ID{"veyron.io/veyron/veyron2/verror.NotFound", Failed}      // Requested entity (e.g. object, method) not found.
)

// Install English messages for the error codes above.
func init() {
	cat := i18n.Cat()
	lang := i18n.LangID("en-US")
	cat.SetWithBase(lang, Success.MsgID, "{1}: Success: {_}")
	cat.SetWithBase(lang, Unknown.MsgID, "{1}: Error: {_}")
	cat.SetWithBase(lang, Aborted.MsgID, "{1}: Aborted: {_}")
	cat.SetWithBase(lang, BadArg.MsgID, "{1}: Bad argument: {_}")
	cat.SetWithBase(lang, BadProtocol.MsgID, "{1}: Bad protocol or type: {_}")
	cat.SetWithBase(lang, Exists.MsgID, "{1}: Already exists: {_}")
	cat.SetWithBase(lang, Internal.MsgID, "{1}: Internal error: {_}")
	cat.SetWithBase(lang, NotAuthorized.MsgID, "{1}: Not authorized: {_}")
	cat.SetWithBase(lang, NotFound.MsgID, "{1}: Not found: {_}")
}
