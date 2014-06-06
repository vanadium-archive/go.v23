package context

// T defines a context under which outgoing RPC calls are made.  It
// carries some setting information, but also creates relationships between RPCs
// executed under the same T.
// TODO(mattr): Add Deadline and other settings.
type T interface {
}
