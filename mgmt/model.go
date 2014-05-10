package mgmt

// LocalStop is the message received on WaitForStop when the stop was initiated
// by the process itself.
const LocalStop = "localstop"

const (
	UnhandledStopExitCode = 1
	ForceStopExitCode     = 1
)

// TODO(caprita): make Stop return chan Tick and expose the ability to convey
// shutdown progress to application layer.
// type Tick struct {
// 	Processed, Goal int32
// }

// Runtime defines the interface for stopping the application.
type Runtime interface {
	// Stop causes all the channels returned by WaitForStop to return the
	// LocalStop message, to give the application a chance to shut down.
	// Stop does not block.  If any of the channels are not receiving,
	// the message is not sent on them.
	// If WaitForStop had never been called, Stop acts like ForceStop.
	Stop()
	// ForceStop causes the application to exit immediately with an error
	// code.
	ForceStop()
	// WaitForStop takes in a channel on which a stop event will be
	// conveyed.  The stop event is represented by a string identifying the
	// source of the event.  For example, when Stop is called locally, the
	// LocalStop message will be received on the channel.  If the channel is
	// not being received on, or is full, no message is sent on it.
	WaitForStop(chan<- string)
}
