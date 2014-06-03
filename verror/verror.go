// Package verror extends the regular error mechanism to work across different
// address spaces.
package verror

import (
	"fmt"
	"regexp"
)

// ID is a unique identifier for errors, allowing stable error checking across
// different error messages and different address spaces.  By convention the
// format is "PKGPATH.NAME" - e.g. ErrIDFoo defined in the "veyron2/verror"
// package has id "veyron2/verror.ErrIDFoo".
type ID string

const Unknown = ID("")

// E extends the regular error interface with an error id.
type E interface {
	error
	// ErrorID returns the error id.
	ErrorID() ID
}

// ErrorID returns the ID of the given err, or Unknown if the err has no ID.
func ErrorID(err error) ID {
	if e, ok := err.(E); ok {
		return e.ErrorID()
	}
	return Unknown
}

// Is returns true iff the given err has the given id.
func Is(err error, id ID) bool {
	return ErrorID(err) == id
}

// IsUnknown is a helper that calls Is(err, Unknown).
func IsUnknown(err error) bool {
	return Is(err, Unknown)
}

// Equal returns true iff a and b have the same error id and message.
func Equal(a, b error) bool {
	if a == nil || b == nil {
		return a == b
	}
	if ErrorID(a) != ErrorID(b) {
		return false
	}
	if a.Error() != b.Error() {
		return false
	}
	return true
}

// Make returns an error implementing E with the given id and msg.
func Make(id ID, msg string) E {
	return Standard{id, msg}
}

// Makef is similar to Make, but constructs the msg via printf-style arguments.
func Makef(id ID, format string, v ...interface{}) E {
	return Standard{id, fmt.Sprintf(format, v...)}
}

// Helper functions to easily make errors with a particular ID.

func Abortedf(fmt string, v ...interface{}) E       { return Makef(Aborted, fmt, v...) }
func BadArgf(fmt string, v ...interface{}) E        { return Makef(BadArg, fmt, v...) }
func BadProtocolf(fmt string, v ...interface{}) E   { return Makef(BadProtocol, fmt, v...) }
func Internalf(fmt string, v ...interface{}) E      { return Makef(Internal, fmt, v...) }
func NotAuthorizedf(fmt string, v ...interface{}) E { return Makef(NotAuthorized, fmt, v...) }
func NotFoundf(fmt string, v ...interface{}) E      { return Makef(NotFound, fmt, v...) }
func Unknownf(fmt string, v ...interface{}) E       { return Makef(Unknown, fmt, v...) }

// Converts a regular err into an E error.  Returns the err unchanged if it's
// already an E error or nil, otherwise returns a new E error with Unknown id.
func Convert(err error) E {
	return ConvertWithDefault(Unknown, err)
}

// ConvertWithDefault converts a regular err into an E error, setting its id to id.
// If err is already an E, it returns err without changing its type.
func ConvertWithDefault(id ID, err error) E {
	if err == nil {
		return nil
	}
	if e, _ := err.(E); e != nil {
		return e
	}
	return Make(id, err.Error())
}

// Standard is the standard implementation of E.
type Standard struct {
	ID  ID
	Msg string
}

// ErrorID returns the error id.
func (e Standard) ErrorID() ID {
	return e.ID
}

// Error returns the error message.
func (e Standard) Error() string {
	return e.Msg
}

// ToStandard converts a regular err into a Standard error.  Returns the err
// unchanged if it's already Standard or nil, otherwise makes a Standard with
// either the original error id (if err had one) or an unknown id.
func ToStandard(err error) E {
	if err == nil {
		return nil
	}
	if s, ok := err.(Standard); ok {
		return s
	}
	if e, _ := err.(E); e != nil {
		return Make(e.ErrorID(), e.Error())
	}
	return Make(Unknown, err.Error())
}

// Translator implements error translation, driven by a set of rules.  Each rule
// specifies how to match an input error, and upon finding a match, the ID and
// prefix of the resulting translated error.
//
// There are three types of input error matching:
//   Error - Matches if input error == target error.
//   ID    - Matches if input error id matches target id.
//   Msg   - Matches if input error message matches target regexp.
//
// The translator processes rules in the order above: all exact error matches
// first, then all ID matches, then all message regexp matches.  A default rule
// may be provided to handle input errors that don't match any of the above.
type Translator struct {
	xlateErr map[error]rule
	xlateID  map[ID]rule
	msgRules []msgRule
	def      *rule
}

type rule struct {
	to     ID
	prefix string
}

func (r rule) newError(msg string) E {
	return Make(r.to, r.prefix+msg)
}

func (r rule) match(target ID) bool {
	return r.to == target
}

type msgRule struct {
	re   *regexp.Regexp
	rule rule
}

// NewTranslator returns a new translator, with no rules.
func NewTranslator() *Translator {
	return &Translator{
		xlateErr: make(map[error]rule),
		xlateID:  make(map[ID]rule),
	}
}

// SetErrorRule sets a translation rule for 'from' input errors to a new error
// with 'to' id and 'prefix' prepended to the error message.
func (t *Translator) SetErrorRule(from error, to ID, prefix string) *Translator {
	t.xlateErr[from] = rule{to, prefix}
	return t
}

// SetIDRule sets a translation rule for input errors with id 'from' to a new
// error with 'to' id and 'prefix' prepended to the error message.
func (t *Translator) SetIDRule(from, to ID, prefix string) *Translator {
	t.xlateID[from] = rule{to, prefix}
	return t
}

// AppendMsgRule appends a translation rule for input errors with message
// matching 're' to a new error with 'to' id and 'prefix' prepended to the error
// message.  Each msg rule is evaluated in FIFO order.
func (t *Translator) AppendMsgRule(re *regexp.Regexp, to ID, prefix string) *Translator {
	t.msgRules = append(t.msgRules, msgRule{re, rule{to, prefix}})
	return t
}

// SetDefaultRule sets a default translation rule for input errors not matching
// any other rule, creating a new error with 'to' id and 'prefix' prepended to
// the error message.
func (t *Translator) SetDefaultRule(to ID, prefix string) *Translator {
	t.def = &rule{to, prefix}
	return t
}

// Translate translates the input err and returns the result.
func (t *Translator) Translate(err error) error {
	if err == nil {
		return nil
	}
	// Always create a new error upon successful translation; it might be
	// confusing if we changed err via side-effect instead.
	msg := err.Error()
	if rule, ok := t.xlateErr[err]; ok {
		return rule.newError(msg)
	}
	if e, _ := err.(E); e != nil {
		if rule, ok := t.xlateID[e.ErrorID()]; ok {
			return rule.newError(msg)
		}
	}
	for _, m := range t.msgRules {
		if m.re.MatchString(msg) {
			return m.rule.newError(msg)
		}
	}
	if t.def != nil {
		return t.def.newError(msg)
	}
	return err
}

// Match translates the input err and returns true iff the resulting id matches
// target.  It is logically equivalent to Is(Translate(err), target), but is
// more efficient.
func (t *Translator) Match(err error, target ID) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	if rule, ok := t.xlateErr[err]; ok {
		return rule.match(target)
	}
	if e, _ := err.(E); e != nil {
		if rule, ok := t.xlateID[e.ErrorID()]; ok {
			return rule.match(target)
		}
	}
	for _, m := range t.msgRules {
		if m.re.MatchString(msg) {
			return m.rule.match(target)
		}
	}
	if t.def != nil {
		return t.def.match(target)
	}
	return false
}
