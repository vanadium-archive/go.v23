package vtrace_test

import (
	"bytes"
	"encoding/binary"
	"testing"
	"time"

	"v.io/core/veyron2/uniqueid"
	"v.io/core/veyron2/vtrace"
)

var nextid = uint64(1)

func id() uniqueid.ID {
	var out uniqueid.ID
	binary.BigEndian.PutUint64(out[8:], nextid)
	nextid++
	return out
}

func TestFormat(t *testing.T) {
	trid := id()
	trstart := time.Date(2014, 11, 6, 13, 1, 22, 400000000, time.UTC)
	spanIDs := make([]uniqueid.ID, 4)
	for i := range spanIDs {
		spanIDs[i] = id()
	}
	tr := vtrace.TraceRecord{
		ID: trid,
		Spans: []vtrace.SpanRecord{
			{
				ID:     spanIDs[0],
				Parent: trid,
				Name:   "",
				Start:  trstart.UnixNano(),
			},
			{
				ID:     spanIDs[1],
				Parent: spanIDs[0],
				Name:   "Child1",
				Start:  trstart.Add(time.Second).UnixNano(),
				End:    trstart.Add(10 * time.Second).UnixNano(),
			},
			{
				ID:     spanIDs[2],
				Parent: spanIDs[0],
				Name:   "Child2",
				Start:  trstart.Add(20 * time.Second).UnixNano(),
				End:    trstart.Add(30 * time.Second).UnixNano(),
			},
			{
				ID:     spanIDs[3],
				Parent: spanIDs[1],
				Name:   "GrandChild1",
				Start:  trstart.Add(3 * time.Second).UnixNano(),
				End:    trstart.Add(8 * time.Second).UnixNano(),
				Annotations: []vtrace.Annotation{
					{
						Message: "First Annotation",
						When:    trstart.Add(4 * time.Second).UnixNano(),
					},
					{
						Message: "Second Annotation",
						When:    trstart.Add(6 * time.Second).UnixNano(),
					},
				},
			},
		},
	}

	var buf bytes.Buffer
	vtrace.FormatTrace(&buf, &tr, time.UTC)
	want := `Trace - 00000000000000000000000000000001 (2014-11-06 13:01:22.400000 UTC, ??)
    Span - Child1 [id: 00000003 parent 00000002] (1s, 10s)
        Span - GrandChild1 [id: 00000005 parent 00000003] (3s, 8s)
            @4s First Annotation
            @6s Second Annotation
    Span - Child2 [id: 00000004 parent 00000002] (20s, 30s)
`
	if got := buf.String(); got != want {
		t.Errorf("Incorrect output, want\n%sgot\n%s", want, got)
	}
}

func TestFormatWithMissingSpans(t *testing.T) {
	trid := id()
	trstart := time.Date(2014, 11, 6, 13, 1, 22, 400000000, time.UTC)
	spanIDs := make([]uniqueid.ID, 6)
	for i := range spanIDs {
		spanIDs[i] = id()
	}
	tr := vtrace.TraceRecord{
		ID: trid,
		Spans: []vtrace.SpanRecord{
			{
				ID:     spanIDs[0],
				Parent: trid,
				Name:   "",
				Start:  trstart.UnixNano(),
			},
			{
				ID:     spanIDs[1],
				Parent: spanIDs[0],
				Name:   "Child1",
				Start:  trstart.Add(time.Second).UnixNano(),
				End:    trstart.Add(10 * time.Second).UnixNano(),
			},
			{
				ID:     spanIDs[3],
				Parent: spanIDs[2],
				Name:   "Decendant1",
				Start:  trstart.Add(15 * time.Second).UnixNano(),
				End:    trstart.Add(24 * time.Second).UnixNano(),
			},
			{
				ID:     spanIDs[4],
				Parent: spanIDs[2],
				Name:   "Decendant2",
				Start:  trstart.Add(12 * time.Second).UnixNano(),
				End:    trstart.Add(18 * time.Second).UnixNano(),
			},
			{
				ID:     spanIDs[5],
				Parent: spanIDs[1],
				Name:   "GrandChild1",
				Start:  trstart.Add(3 * time.Second).UnixNano(),
				End:    trstart.Add(8 * time.Second).UnixNano(),
				Annotations: []vtrace.Annotation{
					{
						Message: "First Annotation",
						When:    trstart.Add(4 * time.Second).UnixNano(),
					},
					{
						Message: "Second Annotation",
						When:    trstart.Add(6 * time.Second).UnixNano(),
					},
				},
			},
		},
	}

	var buf bytes.Buffer
	vtrace.FormatTrace(&buf, &tr, time.UTC)
	want := `Trace - 00000000000000000000000000000006 (2014-11-06 13:01:22.400000 UTC, ??)
    Span - Child1 [id: 00000008 parent 00000007] (1s, 10s)
        Span - GrandChild1 [id: 0000000c parent 00000008] (3s, 8s)
            @4s First Annotation
            @6s Second Annotation
    Span - Missing Data [id: 00000000 parent 00000000] (??, ??)
        Span - Decendant1 [id: 0000000a parent 00000009] (15s, 24s)
        Span - Decendant2 [id: 0000000b parent 00000009] (12s, 18s)
`

	if got := buf.String(); got != want {
		t.Errorf("Incorrect output, want\n%sgot\n%s", want, got)
	}
}
