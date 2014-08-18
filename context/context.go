// Package context defines context.T an interface to carry data that crosses
// API boundaries.  The context carries deadlines and cancellation
// as well as other arbitraray values.
//
// Server method implmentations receive a context as their first argument, you
// should generally pass this context (or a derivitive) on to dependant
// opertations.  You should allocate new context.T objects only for operations
// that are semantically unrelated to any ongoing calls.
//
// The context.T type is safe to use from multiple goroutines simultaneously.
package context

import (
	"errors"
	"time"
)

// A T object carries deadlines, cancellation and data across API
// boundaries.  It is safe to use from multiple goroutines.
type T interface {
	// Deadline returns the time at which this context will be automatically
	// canceled.
	Deadline() (deadline time.Time, ok bool)

	// Done returns a channel which will be closed when this context.T
	// is canceled or exceeds its deadline.  Successive calls will
	// return the same value.  Implementations may return nil if they can
	// never be canceled.
	Done() <-chan struct{}

	// After the channel returned by Done() is closed, Err() will return
	// either Canceled or DeadlineExceeded.
	Err() error

	// Value is used to carry data across API boundaries.  You should use this
	// only for data that is relevant across multiple API boundaries and not
	// just to pass extra parameters to functions and methods.
	// Any type that supports equality can be used as a key, but you should
	// use an unexported type to prevent collisions.
	Value(key interface{}) interface{}

	// WithCancel returns a child of the current context along with
	// a function that can be used to cancel it.  After cancel() is
	// called the channels returned by the Done() methods of the new context
	// (and all context further derived from it) will be closed.
	WithCancel() (ctx T, cancel CancelFunc)

	// WithDeadline returns a child of the current context along with a
	// function that can be used to cancel it at any time (as from
	// WithCancel).  When the deadline is reached the context will be
	// automatically cancelled.
	// You should cancel these contexts when you are finished with them
	// so that resources associated with their timers may be released.
	WithDeadline(deadline time.Time) (T, CancelFunc)

	// WithTimeout is similar to WithDeadline except you give a Duration
	// that represents a relative point in time from now.
	WithTimeout(timeout time.Duration) (T, CancelFunc)

	// WithValue returns a child of the current context that will return
	// the given val when Value(key) is called.
	WithValue(key interface{}, val interface{}) T
}

// A CancelFunc is used to cancel a context.  The first call will
// cause the paired context and all decendants to close their Done()
// channels.  Further calls do nothing.
type CancelFunc func()

// Cancelled is returned by contexts which have been cancelled.
var Canceled = errors.New("context canceled")

// DeadlineExceeded is returned by contexts that have exceeded their
// deadlines and therefore been canceled automatically.
var DeadlineExceeded = errors.New("context deadline exceeded")
