package idl

import "errors"

var (
	// Error returned when the Bind<Service> name function is called with invalid options.
	ErrTooManyOptionsToBind = errors.New("too many options to Bind...., at most a single Runtime or a single ipc.Client is provided")

	// Error returns when an unrecognized option is provided to Bind<Service>.
	ErrUnrecognizedOption = errors.New("unrecognized option")
)
