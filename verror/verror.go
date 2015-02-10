// Package verror extends the regular error mechanism to work across different
// address spaces.
package verror

import (
	"fmt"
)

// ID is a unique identifier for errors, allowing stable error
// checking across different error messages and different address
// spaces.  By convention the format is "PKGPATH.NAME" - e.g. ErrIDFoo
// defined in the "v.io/core/veyron2/verror" package has id
// "v.io/core/veyron2/verror.ErrIDFoo".
type ID string

const unknown = ID("")

// E extends the regular error interface with an error id.
type E interface {
	error
	// ErrorID returns the error id.
	ErrorID() ID
}

// Make returns an error implementing E with the given id and msg.
func Make(id ID, msg string) E {
	return standard{id, msg}
}

// Makef is similar to Make, but constructs the msg via printf-style arguments.
func Makef(id ID, format string, v ...interface{}) E {
	return standard{id, fmt.Sprintf(format, v...)}
}

// Helper functions to easily make errors with a particular ID.

func BadProtocolf(fmt string, v ...interface{}) E { return Makef(BadProtocol, fmt, v...) }
func Internalf(fmt string, v ...interface{}) E    { return Makef(Internal, fmt, v...) }
func NoAccessf(fmt string, v ...interface{}) E    { return Makef(NoAccess, fmt, v...) }

// Converts a regular err into an E error.  Returns the err unchanged if it's
// already an E error or nil, otherwise returns a new E error with unknown id.
func Convert(err error) E {
	return convertWithDefault(unknown, err)
}

// convertWithDefault converts a regular err into an E error, setting its id to id.
// If err is already an E, it returns err without changing its type.
func convertWithDefault(id ID, err error) E {
	if err == nil {
		return nil
	}
	if e, _ := err.(E); e != nil {
		return e
	}
	return Make(id, err.Error())
}

// standard is the standard implementation of E.
type standard struct {
	// The field names and order must be kept in sync with vdl.ErrorType defined
	// in v.io/core/veyron2/vdl/type_builder.go.
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
