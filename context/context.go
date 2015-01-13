// Package context defines context.T an interface to carry data that
// crosses API boundaries.  The context carries deadlines and
// cancellation as well as other arbitraray values.
//
// Server method implmentations receive a context as their first
// argument, you should generally pass this context (or a derivitive)
// on to dependant opertations.  You should allocate new context.T
// objects only for operations that are semantically unrelated to any
// ongoing calls.
//
// New contexts are created via the runtime with
// veryon2.Runtime.NewContext().  New contexts represent a fresh
// operation that's unrelated to any other ongoing work.  Further
// contexts can be derived from the original context to refine the
// environment.  For example if part of your operation requires a
// finer deadline you can create a new context to handle that part:
//
//    ctx := runtime.NewContext()
//    // We'll use cacheCtx to lookup data in memcache
//    // if it takes more than a second to get data from
//    // memcache we should just skip the cache and perform
//    // the slow operation.
//    cacheCtx, cancel := ctx.WithTimeout(time.Second)
//    if err := FetchDataFromMemcache(cacheCtx, key); err == DeadlineExceeded {
//      RecomputeData(ctx, key)
//    }
//
// Contexts form a tree where derived contexts are children of the
// contexts from which they were derived.  Children inherit all the
// properties of their parent except for the property being replaced
// (the deadline in the example above).
//
// Contexts are extensible.  The Value/WithValue methods allow you to attach
// new information to the context and extend it's capabilities.
// In the same way we derive new contexts via the 'With' family of functions
// you can create methods to attach new data:
//
//    package auth
//
//    import "v.io/core/veyron2/context"
//
//    type Auth struct{...}
//
//    type authKey struct{}
//
//    function WithAuth(parent *context.T, data *Auth) *context.T {
//        return parent.WithValue(authKey{}, data)
//    }
//
//    function FromContext(ctx *context.T) *Auth {
//        data, _ := ctx.Value(authKey{}).(*Auth)
//        return data
//    }
//
// Note that a value of any type can be used as a key, but you should
// use an unexported value of an unexported type to ensure that no
// collisions can occur.
package context

import (
	"errors"
	"sync"
	"time"
)

type internalKey int

const (
	cancelKey = internalKey(iota)
	deadlineKey
)

// A CancelFunc is used to cancel a context.  The first call will
// cause the paired context and all decendants to close their Done()
// channels.  Further calls do nothing.
type CancelFunc func()

// Cancelled is returned by contexts which have been cancelled.
var Canceled = errors.New("context canceled")

// DeadlineExceeded is returned by contexts that have exceeded their
// deadlines and therefore been canceled automatically.
var DeadlineExceeded = errors.New("context deadline exceeded")

// A T object carries deadlines, cancellation and data across API
// boundaries.  It is safe to use a T from multiple goroutines simultaneously.
// The zero-type of context is uninitialized and should never be used
// directly by application code.  Only runtime implementors should
// create them directly.
type T struct {
	parent     *T
	key, value interface{}
}

// Initialized returns true if this context has been properly initialized
// by a runtime.
func (t *T) Initialized() bool {
	return t != nil && t.key != nil
}

// Value is used to carry data across API boundaries.  This should be
// used only for data that is relevant across multiple API boundaries
// and not just to pass extra parameters to functions and methods.
// Any type that supports equality can be used as a key, but an
// unexported type should be used to prevent collisions.
func (t *T) Value(key interface{}) interface{} {
	for t != nil {
		if key == t.key {
			return t.value
		}
		t = t.parent
	}
	return nil
}

// Deadline returns the time at which this context will be automatically
// canceled.
func (t *T) Deadline() (deadline time.Time, ok bool) {
	if deadline, ok := t.Value(deadlineKey).(*deadlineState); ok {
		return deadline.deadline, true
	}
	return
}

// After the channel returned by Done() is closed, Err() will return
// either Canceled or DeadlineExceeded.
func (t *T) Err() error {
	if cancel, ok := t.Value(cancelKey).(*cancelState); ok {
		cancel.mu.Lock()
		defer cancel.mu.Unlock()
		return cancel.err
	}
	return nil
}

// Done returns a channel which will be closed when this context.T
// is canceled or exceeds its deadline.  Successive calls will
// return the same value.  Implementations may return nil if they can
// never be canceled.
func (t *T) Done() <-chan struct{} {
	if cancel, ok := t.Value(cancelKey).(*cancelState); ok {
		return cancel.done
	}
	return nil
}

// cancelState helps pass cancellation down the context tree.
type cancelState struct {
	done chan struct{}

	mu       sync.Mutex
	err      error                 // GUARDED_BY(mu)
	children map[*cancelState]bool // GUARDED_BY(mu)
}

func (c *cancelState) addChild(child *cancelState) {
	c.mu.Lock()

	if c.err != nil {
		err := c.err
		c.mu.Unlock()
		child.cancel(err)
		return
	}

	if c.children == nil {
		c.children = make(map[*cancelState]bool)
	}
	c.children[child] = true
	c.mu.Unlock()
}

func (c *cancelState) removeChild(child *cancelState) {
	c.mu.Lock()
	delete(c.children, c)
	c.mu.Unlock()
}

func (c *cancelState) cancel(err error) {
	var children map[*cancelState]bool

	c.mu.Lock()
	if c.err == nil {
		c.err = err
		children = c.children
		c.children = nil
		close(c.done)
	}
	c.mu.Unlock()

	for child, _ := range children {
		child.cancel(err)
	}
}

// A deadlineState helps cancel contexts when a deadline expires.
type deadlineState struct {
	deadline time.Time
	timer    *time.Timer
}

// WithValue returns a child of the current context that will return
// the given val when Value(key) is called.
func WithValue(parent *T, key interface{}, val interface{}) *T {
	if key == nil {
		panic("Attempting to store a context value with an untyped nil key.")
	}
	return &T{parent, key, val}
}

func withCancelState(parent *T) (*T, func(error)) {
	cs := &cancelState{done: make(chan struct{})}
	cancelParent, ok := parent.Value(cancelKey).(*cancelState)
	if ok {
		cancelParent.addChild(cs)
	}
	return WithValue(parent, cancelKey, cs), func(err error) {
		if ok {
			cancelParent.removeChild(cs)
		}
		cs.cancel(err)
	}
}

// WithCancel returns a child of the current context along with
// a function that can be used to cancel it.  After cancel() is
// called the channels returned by the Done() methods of the new context
// (and all context further derived from it) will be closed.
func WithCancel(parent *T) (*T, CancelFunc) {
	t, cancel := withCancelState(parent)
	return t, func() { cancel(Canceled) }
}

func withDeadlineState(parent *T, deadline time.Time, timeout time.Duration) (*T, CancelFunc) {
	t, cancel := withCancelState(parent)
	ds := &deadlineState{deadline, time.AfterFunc(timeout, func() { cancel(DeadlineExceeded) })}
	return WithValue(t, deadlineKey, ds), func() {
		ds.timer.Stop()
		cancel(Canceled)
	}
}

// WithDeadline returns a child of the current context along with a
// function that can be used to cancel it at any time (as from
// WithCancel).  When the deadline is reached the context will be
// automatically cancelled.
// Contexts should be cancelled when they are no longer needed
// so that resources associated with their timers may be released.
func WithDeadline(parent *T, deadline time.Time) (*T, CancelFunc) {
	return withDeadlineState(parent, deadline, deadline.Sub(time.Now()))
}

// WithTimeout is similar to WithDeadline except a Duration is given
// that represents a relative point in time from now.
func WithTimeout(parent *T, timeout time.Duration) (*T, CancelFunc) {
	return withDeadlineState(parent, time.Now().Add(timeout), timeout)
}
