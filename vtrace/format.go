package vtrace

import (
	"fmt"
	"io"
	"sort"
	"time"

	"v.io/core/veyron2/uniqueid"
)

const indentStep = "    "

type children []*node

// children implements sort.Interface
func (c children) Len() int           { return len(c) }
func (c children) Less(i, j int) bool { return c[i].span.Start < c[j].span.Start }
func (c children) Swap(i, j int)      { c[i], c[j] = c[j], c[i] }

type annotations []Annotation

// annotations implements sort.Interface
func (a annotations) Len() int           { return len(a) }
func (a annotations) Less(i, j int) bool { return a[i].When < a[j].When }
func (a annotations) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }

type node struct {
	span     *SpanRecord
	children children
}

// TODO(mattr): It is useful in general to make a tree of spans
// for analysis as well as formatting.  This interface should
// be cleaned up and exported.
func buildTree(trace *TraceRecord) *node {
	var root *node
	var earliestTime int64
	nodes := make(map[uniqueid.ID]*node, len(trace.Spans))

	for i := range trace.Spans {
		span := &trace.Spans[i]
		if earliestTime == 0 || span.Start < earliestTime {
			earliestTime = span.Start
		}

		n := nodes[span.ID]
		if n == nil {
			n = &node{}
			nodes[span.ID] = n
		}

		n.span = span

		if span.Parent == trace.ID {
			root = n
		} else {
			p := nodes[span.Parent]
			if p == nil {
				p = &node{}
				nodes[span.Parent] = p
			}
			p.children = append(p.children, n)
		}
	}

	// Sort the children of each node in start-time order, and the
	// annotation in time-order.
	for _, node := range nodes {
		sort.Sort(node.children)
		if node.span != nil {
			sort.Sort(annotations(node.span.Annotations))
		}
	}

	// If we didn't find the root span of the trace
	// create a stand-in.
	if root == nil {
		root = &node{
			span: &SpanRecord{
				Name:  "Missing Root Span",
				Start: earliestTime,
			},
		}
	}

	// Find all nodes that have no span.  These represent missing data
	// in the tree.  We invent fake "missing" spans to represent
	// (perhaps several) layers of missing spans.  Then we add these as
	// children of the root.
	var missing []*node
	for _, n := range nodes {
		if n.span == nil {
			n.span = &SpanRecord{
				Name: "Missing Data",
			}
			missing = append(missing, n)
		}
	}

	if len(missing) > 0 {
		root.children = append(root.children, missing...)
	}

	return root
}

func formatDelta(when, start int64) string {
	if when == 0 {
		return "??"
	}
	return time.Duration(when - start).String()
}

func formatNode(w io.Writer, n *node, traceStart int64, indent string) {
	fmt.Fprintf(w, "%sSpan - %s [id: %x parent %x] (%s, %s)\n",
		indent,
		n.span.Name,
		n.span.ID[12:],
		n.span.Parent[12:],
		formatDelta(n.span.Start, traceStart),
		formatDelta(n.span.End, traceStart))
	indent += indentStep
	for _, a := range n.span.Annotations {
		fmt.Fprintf(w, "%s@%s %s\n", indent, formatDelta(a.When, traceStart), a.Message)
	}
	for _, c := range n.children {
		formatNode(w, c, traceStart, indent)
	}
}

func formatTime(when int64, loc *time.Location) string {
	if when == 0 {
		return "??"
	}
	t := time.Unix(0, when)
	if loc != nil {
		t = t.In(loc)
	}
	return t.Format("2006-01-02 15:04:05.000000 MST")
}

// FormatTrace writes a text description of the given trace to the
// given writer.  Times will be formatted according to the given
// location, if loc is nil local times will be used.
func FormatTrace(w io.Writer, record *TraceRecord, loc *time.Location) {
	if root := buildTree(record); root != nil {
		fmt.Fprintf(w, "Trace - %x (%s, %s)\n",
			record.ID,
			formatTime(root.span.Start, loc),
			formatTime(root.span.End, loc))
		for _, c := range root.children {
			formatNode(w, c, root.span.Start, indentStep)
		}
	}
}

// FormatTraces writes a text description of all the given traces to
// the given writer.  Times will be formatted according to the given
// location, if loc is nil local times will be used.
func FormatTraces(w io.Writer, records []TraceRecord, loc *time.Location) {
	fmt.Fprintf(w, "Vtrace traces:\n")
	for i := range records {
		FormatTrace(w, &records[i], loc)
	}
}
