// Package vtrace extends the v23/context to allow you to attach
// various types of debugging information.  This debugging information
// accumulates along the various parts of the context tree and can be
// inspected to help you understand the performance and behavior of
// your system even across servers and processes.
//
// A new root context, as created by v23.Runtime.NewContext()
// represents a new operation unconnected to any other.  Vtrace
// represents all the debugging information collected about this
// operation, even across servers and processes, as a single Trace.
// The Trace will be divided into a hierarchy of timespans (Spans).
// For example, imagine our high level operation is making a new
// blog post.  We may have to first authentiate with an auth server,
// then write the new post to a database, and finally notify subscribers
// of the new content.  The trace might look like this:
//
//    Trace:
//    <---------------- Make a new blog post ----------->
//    |                  |                   |
//    <- Authenticate -> |                   |
//                       |                   |
//                       <-- Write to DB --> |
//                                           <- Notify ->
//    0s                      1.5s                      3s
//
// Here we have a single trace with four Spans.  Note that some Spans
// are children of other Spans.  This structure falls directly out of
// our building off of the context.T tree.  When you derive a new
// context using SetNewSpan(), you create a Span thats a child of
// the currently active span in the context.  Note that spans that
// share a parent may overlap in time.
//
// In this case the tree would have been created with code like this:
//
//    function MakeBlogPost(ctx *context.T) {
//        authCtx, _ := vtrace.SetNewSpan(ctx, "Authenticate")
//        Authenticate(authCtx)
//        writeCtx, _ := vtrace.SetNewSpan(ctx, "Write To DB")
//        Write(writeCtx)
//        notifyCtx, _ := vtrace.SetNewSpan(ctx, "Notify")
//        Notify(notifyCtx)
//    }
//
// Just as we have Spans to represent timesspans we have Annotations
// to attach debugging information to the current span that is relevant
// to the current moment.  Currently we only support string annotations.
// You can add an annotation to the current span by calling the Spans
// Annotate method:
//
//    span := vtrace.FromContext(ctx)
//    span.Annotate("Just got an error")
//
// When you make an annotation we record the annotation and the time
// when it was attached.
// TODO(mattr): Allow other types of annotations, for example server
// information, start and stop events.
//
// Traces can be composed of large numbers of spans containing
// data collected from large numbers of different processes.  Because
// this data is large we don't collect it for every context.  By
// default we collect trace data on only a small random sample of
// the contexts that are created.  If a particular operation is of
// special importants you can force it to be collected by calling the
// Trace's ForceCollect method.  All the spans and annotations that
// are added after ForceCollect is called will be collected.
//
// If your trace has collected information you can retrieve the data
// collected so far by calling the Trace.Record() method, which gives
// you a dump in the form of a TraceRecord.
package vtrace

import (
	"v.io/v23/context"
	"v.io/v23/uniqueid"
)

// Spans represent a named time period.  You can create new spans
// to represent new parts of your computation.
// Spans are safe to use from multiple goroutines simultaneously.
type Span interface {
	// Name returns the name of the span.
	Name() string

	// ID returns the uniqueid.ID of the span.
	ID() uniqueid.Id

	// Parent returns the uniqueid.ID of this spans parent span.
	Parent() uniqueid.Id

	// Annotate adds a string annotation to the trace.  Where Spans
	// represent time periods Annotations represent data thats relevant
	// at a specific moment.
	Annotate(s string)

	// Annotatef adds an annotation to the trace.  Where Spans represent
	// time periods Annotations represent data thats relevant at a
	// specific moment.
	// format and a are interpreted as with fmt.Printf.
	Annotatef(format string, a ...interface{})

	// Finish ends the span, marking the end time.  The span should
	// not be used after Finish is called.
	Finish()

	// Trace returns the id of the trace this Span is a member of.
	Trace() uniqueid.Id
}

// Store selectively collects information about traces in the system.
type Store interface {
	// TraceRecords returns TraceRecords for all traces saved in the store.
	TraceRecords() []TraceRecord

	// TraceRecord returns a TraceRecord for a given ID.  Returns
	// nil if the given id is not present.
	TraceRecord(traceid uniqueid.Id) *TraceRecord

	// ForceCollect forces the store to collect all information about a given trace.
	ForceCollect(traceid uniqueid.Id)

	// Merge merges a vtrace.Response into the current store.
	Merge(response Response)
}

type Manager interface {
	// SetNewTrace creates a new vtrace context that is not the child of any
	// other span.  This is useful when starting operations that are
	// disconnected from the activity ctx is performing.  For example
	// this might be used to start background tasks.
	SetNewTrace(ctx *context.T) (*context.T, Span)

	// SetContinuedTrace creates a span that represents a continuation of
	// a trace from a remote server.  name is the name of the new span and
	// req contains the parameters needed to connect this span with it's
	// trace.
	SetContinuedTrace(ctx *context.T, name string, req Request) (*context.T, Span)

	// SetNewSpan derives a context with a new Span that can be used to
	// trace and annotate operations across process boundaries.
	SetNewSpan(ctx *context.T, name string) (*context.T, Span)

	// Span finds the currently active span.
	GetSpan(ctx *context.T) Span

	// Store returns the current Store.
	GetStore(ctx *context.T) Store

	// Generate a Request from the current context.
	GetRequest(ctx *context.T) Request

	// Generate a Response from the current context.
	GetResponse(ctx *context.T) Response
}

// managerKey is used to store a Manger in the context.
type managerKey struct{}

// WithManager returns a new context with a Vtrace manager attached.
func WithManager(ctx *context.T, manager Manager) *context.T {
	return context.WithValue(ctx, managerKey{}, manager)
}

func manager(ctx *context.T) Manager {
	manager, _ := ctx.Value(managerKey{}).(Manager)
	if manager == nil {
		panic(`Vtrace is uninitialized.
You are calling a Vtrace function but vtrace has not been initialized.
This is normally handled by the runtime initialization.  You should call
v23.Init() in your main or test before performing this function.`)
	}
	return manager
}

// SetNewTrace creates a new vtrace context that is not the child of any
// other span.  This is useful when starting operations that are
// disconnected from the activity ctx is performing.  For example
// this might be used to start background tasks.
func SetNewTrace(ctx *context.T) (*context.T, Span) {
	return manager(ctx).SetNewTrace(ctx)
}

// SetContinuedTrace creates a span that represents a continuation of
// a trace from a remote server.  name is the name of the new span and
// req contains the parameters needed to connect this span with it's
// trace.
func SetContinuedTrace(ctx *context.T, name string, req Request) (*context.T, Span) {
	return manager(ctx).SetContinuedTrace(ctx, name, req)
}

// SetNewSpan derives a context with a new Span that can be used to
// trace and annotate operations across process boundaries.
func SetNewSpan(ctx *context.T, name string) (*context.T, Span) {
	return manager(ctx).SetNewSpan(ctx, name)
}

// Span finds the currently active span.
func GetSpan(ctx *context.T) Span {
	return manager(ctx).GetSpan(ctx)
}

// VtraceStore returns the current Store.
func GetStore(ctx *context.T) Store {
	return manager(ctx).GetStore(ctx)
}

// ForceCollect forces the store to collect all information about the
// current trace.
func ForceCollect(ctx *context.T) {
	m := manager(ctx)
	m.GetStore(ctx).ForceCollect(m.GetSpan(ctx).Trace())
}

// Generate a Request from the current context.
func GetRequest(ctx *context.T) Request {
	return manager(ctx).GetRequest(ctx)
}

// Generate a Response from the current context.
func GetResponse(ctx *context.T) Response {
	return manager(ctx).GetResponse(ctx)
}
