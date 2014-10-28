// Package vtrace extends the veyron2/context to allow you to attach
// various types of debugging information.  This debugging information
// accumulates along the various parts of the context tree and can be
// inspected to help you understand the performance and behavior of
// your system even across servers and processes.
//
// A new root context, as created by veyron2.Runtime.NewContext()
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
// context using WithNewSpan(), you create a Span thats a child of
// the currently active span in the context.  Note that spans that
// share a parent may overlap in time.
//
// In this case the tree would have been created with code like this:
//
//    function MakeBlogPost(ctx context.T) {
//        authCtx, _ := vtrace.WithNewSpan(ctx, "Authenticate")
//        Authenticate(authCtx)
//        writeCtx, _ := vtrace.WithNewSpan(ctx, "Write To DB")
//        Write(writeCtx)
//        notifyCtx, _ := vtrace.WithNewSpan(ctx, "Notify")
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
	"veyron.io/veyron/veyron2/context"
	"veyron.io/veyron/veyron2/uniqueid"
)

// Trace represents an entire computation which is composed of a number
// of Spans arranged in a tree.
// Traces are safe to use from multiple goroutines simultaneously.
type Trace interface {
	// uniqueid.ID returns the trace's uniqueid.ID.
	ID() uniqueid.ID

	// ForceCollect forces the collection of the trace.
	ForceCollect()

	// Record produces all data that has been collected in memory so far for
	// the current trace.
	Record() TraceRecord
}

// Spans represent a named time period.  You can create new spans
// to represent new parts of your computation.
// Spans are safe to use from multiple goroutines simultaneously.
type Span interface {
	// Name returns the name of the span.
	Name() string

	// ID returns the uniqueid.ID of the span.
	ID() uniqueid.ID

	// Parent returns the uniqueid.ID of this spans parent span.
	Parent() uniqueid.ID

	// Annotate adds an annotation to the trace.  Where Spans represent
	// time periods Annotations represent data thats relevant at a
	// specific moment.  TODO(mattr): Allow richer annotations with
	// structured data.
	Annotate(msg string)

	// Finish ends the span, marking the end time.  The span should
	// not be used after Finish is called.
	Finish()

	// Trace returns the Trace this Span is a member of.
	Trace() Trace
}

// spanManager is implemented by veyron2.Runtime, but we can't
// depend on that here.
type spanManager interface {
	WithNewSpan(ctx context.T, name string) (context.T, Span)
	SpanFromContext(ctx context.T) Span
}

// Derive a new context from ctx that has a new Span attached.
func WithNewSpan(ctx context.T, name string) (context.T, Span) {
	return ctx.Runtime().(spanManager).WithNewSpan(ctx, name)
}

// Get the currently active span.
func FromContext(ctx context.T) Span {
	return ctx.Runtime().(spanManager).SpanFromContext(ctx)
}
