package vtrace

import (
	"testing"

	"v.io/v23/context"
	"v.io/v23/uniqueid"
)

func spanNoPanic(span Span) {
	span.Name()
	span.ID()
	span.Parent()
	span.Annotate("")
	span.Annotatef("")
	span.Finish()
	span.Trace()
}

func storeNoPanic(store Store) {
	store.TraceRecords()
	store.TraceRecord(uniqueid.Id{})
	store.ForceCollect(uniqueid.Id{})
	store.Merge(Response{})
}

func TestNoPanic(t *testing.T) {
	ctx, cancel := context.RootContext()
	defer cancel()
	initialctx := ctx

	ctx, span := SetNewTrace(ctx)
	spanNoPanic(span)

	ctx, span = SetContinuedTrace(ctx, "", Request{})
	spanNoPanic(span)
	spanNoPanic(GetSpan(ctx))
	GetRequest(ctx)
	GetResponse(ctx)

	storeNoPanic(GetStore(ctx))

	if ctx != initialctx {
		t.Errorf("context was unexpectedly changed.")
	}
}
