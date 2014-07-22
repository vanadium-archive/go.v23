package primitives

import (
	"reflect"
	"runtime"
	"testing"

	"veyron2/services/store"
	"veyron2/storage"
	"veyron2/vdl/vdlutil"
)

type mockObjectQueryStream struct {
	results []interface{}
	curr    int
	err     error
}

func (m *mockObjectQueryStream) Advance() bool {
	if m.curr >= len(m.results)-1 || m.err != nil {
		return false
	}
	m.curr++
	switch r := m.results[m.curr].(type) {
	case error:
		m.err = r
		return false
	case store.QueryResult:
		return true
	default:
		return false
	}
}

func (m *mockObjectQueryStream) Value() store.QueryResult {
	return m.results[m.curr].(store.QueryResult)
}

func (m *mockObjectQueryStream) Err() error {
	return m.err
}

func (m *mockObjectQueryStream) Cancel() {
	m.results = nil
}

func (m *mockObjectQueryStream) Finish() error {
	return nil
}

func newMockObjectQueryStream(results []interface{}) *mockObjectQueryStream {
	return &mockObjectQueryStream{results: results, curr: -1}
}

func advance(t *testing.T, stream storage.QueryStream) {
	if !stream.Advance() {
		_, file, line, _ := runtime.Caller(1)
		t.Fatalf("%s:%d unable to advance", file, line)
	}
}

func assertDone(t *testing.T, stream storage.QueryStream) {
	if stream.Advance() {
		_, file, line, _ := runtime.Caller(1)
		t.Fatalf("%s:%d unexpectedly able to advance", file, line)
	}
}

func assertName(t *testing.T, res storage.QueryResult, expectedName string) {
	_, file, line, _ := runtime.Caller(1)
	if got, want := res.Name(), expectedName; got != want {
		t.Errorf("%s:%d wrong name; got %s, want %s", file, line, got, want)
	}
}

func assertNameAndValue(t *testing.T, res storage.QueryResult,
	expectedName string, expectedVal interface{}) {
	_, file, line, _ := runtime.Caller(1)
	if got, want := res.Name(), expectedName; got != want {
		t.Errorf("%s:%d wrong name; got %s, want %s", file, line, got, want)
	}
	if got, want := res.Value(), expectedVal; got != want {
		t.Errorf("%s:%d wrong value; got %d, want %d", file, line, got, want)
	}
	if res.Fields() != nil {
		t.Errorf("%s:%d non-nil fields", file, line)
	}
}

func assertNameAndFields(t *testing.T, res storage.QueryResult,
	expectedName string, expectedFields map[string]interface{}) {
	_, file, line, _ := runtime.Caller(1)
	if got, want := res.Name(), expectedName; got != want {
		t.Errorf("%s:%d wrong name; got %s, want %s", file, line, got, want)
	}
	if res.Value() != nil {
		t.Errorf("%s:%d non-nil Value", file, line)
	}
	if got, want := res.Fields(), expectedFields; !reflect.DeepEqual(got, want) {
		t.Errorf("%s:%d wrong value; got %v, want %v", file, line, got, want)
	}
}

func TestFlatQueryStream(t *testing.T) {
	mockResults := []interface{}{
		store.QueryResult{
			NestedResult: 0,
			Name:         "result1",
			Value:        10,
			Fields:       nil,
		},
		store.QueryResult{
			NestedResult: 0,
			Name:         "result2",
			Value:        nil,
			Fields: map[string]vdlutil.Any{
				"field1": 1,
				"field2": "value2",
			},
		},
		store.QueryResult{
			NestedResult: 0,
			Name:         "result3",
			Value:        30,
			Fields:       nil,
		},
	}

	stream := newQueryStream(newMockObjectQueryStream(mockResults))
	advance(t, stream)
	assertNameAndValue(t, stream.Value(), "result1", 10)
	advance(t, stream)
	assertNameAndFields(t, stream.Value(), "result2", map[string]interface{}{
		"field1": 1,
		"field2": "value2",
	})
	advance(t, stream)
	assertNameAndValue(t, stream.Value(), "result3", 30)
	assertDone(t, stream)

	// Cancel right away.
	stream = newQueryStream(newMockObjectQueryStream(mockResults))
	stream.Cancel()
	assertDone(t, stream)

	// Cancel after one iteration.
	stream = newQueryStream(newMockObjectQueryStream(mockResults))
	advance(t, stream)
	stream.Cancel()
	assertDone(t, stream)

	// Cancel after all done.
	stream = newQueryStream(newMockObjectQueryStream(mockResults))
	advance(t, stream)
	advance(t, stream)
	advance(t, stream)
	assertDone(t, stream)
	stream.Cancel()
	assertDone(t, stream)

	// Async cancel to uncover race conditions.
	stream = newQueryStream(&mockObjectQueryStream{results: mockResults})
	go stream.Cancel()
	stream.Advance()
	stream.Advance()
	stream.Advance()
}

func TestSimpleNestedQueryStream(t *testing.T) {
	mockResults := []interface{}{
		store.QueryResult{
			NestedResult: 0,
			Name:         "result1",
			Value:        10,
			Fields:       nil,
		},
		store.QueryResult{
			NestedResult: 0,
			Name:         "result2",
			Value:        nil,
			Fields: map[string]vdlutil.Any{
				"nested1": store.NestedResult(1),
				"field1":  6,
			},
		},
		store.QueryResult{
			NestedResult: 1,
			Name:         "child1",
			Value:        "little1",
			Fields:       nil,
		},
		store.QueryResult{
			NestedResult: 0,
			Name:         "result3",
			Value:        30,
			Fields:       nil,
		},
	}

	stream := newQueryStream(newMockObjectQueryStream(mockResults))
	advance(t, stream)
	assertNameAndValue(t, stream.Value(), "result1", 10)

	advance(t, stream)
	assertName(t, stream.Value(), "result2")
	fields := stream.Value().Fields()
	if got, want := fields["field1"], 6; got != want {
		t.Fatalf("wrong value; got %d, want %d", got, want)
	}
	{
		childStream, ok := fields["nested1"].(storage.QueryStream)
		if !ok {
			t.Fatalf("nested1 does not exist, %v", fields)
		}
		advance(t, childStream)
		assertNameAndValue(t, childStream.Value(), "result2/child1", "little1")
	}
	advance(t, stream)
	assertNameAndValue(t, stream.Value(), "result3", 30)
	assertDone(t, stream)

	// Cancel before encountering the child stream.
	stream = newQueryStream(newMockObjectQueryStream(mockResults))
	advance(t, stream)
	stream.Cancel()
	assertDone(t, stream)

	// Cancel after encountering the child stream.
	stream = newQueryStream(newMockObjectQueryStream(mockResults))
	advance(t, stream)
	advance(t, stream)
	stream.Cancel()
	assertDone(t, stream)

	// Cancel the child stream right away.
	stream = newQueryStream(newMockObjectQueryStream(mockResults))
	advance(t, stream)
	advance(t, stream)
	childStream := stream.Value().Fields()["nested1"].(storage.QueryStream)
	childStream.Cancel()
	assertDone(t, childStream)
	advance(t, stream)
	assertDone(t, stream)

	// Advancing the parent stream should invalidate the child stream.
	stream = newQueryStream(newMockObjectQueryStream(mockResults))
	advance(t, stream)
	advance(t, stream)
	childStream = stream.Value().Fields()["nested1"].(storage.QueryStream)
	advance(t, stream)
	assertDone(t, childStream)
	assertDone(t, stream)

	// Canceling the parent stream should invalidate both streams.
	stream = newQueryStream(newMockObjectQueryStream(mockResults))
	advance(t, stream)
	advance(t, stream)
	childStream = stream.Value().Fields()["nested1"].(storage.QueryStream)
	stream.Cancel()
	assertDone(t, childStream)
	assertDone(t, stream)

	// Async cancel of the child stream to uncover race conditions.
	stream = newQueryStream(newMockObjectQueryStream(mockResults))
	advance(t, stream)
	advance(t, stream)
	childStream = stream.Value().Fields()["nested1"].(storage.QueryStream)
	go childStream.Cancel()
	childStream.Advance()
	advance(t, stream)
	assertDone(t, stream)
}

func TestEmptyNestedQueryStream(t *testing.T) {
	mockResults := []interface{}{
		store.QueryResult{
			NestedResult: 0,
			Name:         "result1",
			Value:        10,
			Fields:       nil,
		},
		store.QueryResult{
			NestedResult: 0,
			Name:         "result2",
			Value:        nil,
			Fields: map[string]vdlutil.Any{
				"nested1": store.NestedResult(1),
			},
		},
		store.QueryResult{
			NestedResult: 0,
			Name:         "result3",
			Value:        30,
			Fields:       nil,
		},
	}

	stream := newQueryStream(newMockObjectQueryStream(mockResults))
	advance(t, stream)
	assertNameAndValue(t, stream.Value(), "result1", 10)

	advance(t, stream)
	assertName(t, stream.Value(), "result2")
	fields := stream.Value().Fields()
	{
		childStream, ok := fields["nested1"].(storage.QueryStream)
		if !ok {
			t.Fatalf("nested1 does not exist, %v", fields)
		}
		assertDone(t, childStream)
	}
	advance(t, stream)
	assertNameAndValue(t, stream.Value(), "result3", 30)
	assertDone(t, stream)
}

func TestMultipleNestedQueryStreams(t *testing.T) {
	mockResults := []interface{}{
		store.QueryResult{
			NestedResult: 0,
			Name:         "result1",
			Value:        10,
			Fields:       nil,
		},
		store.QueryResult{
			NestedResult: 0,
			Name:         "result2",
			Value:        nil,
			Fields: map[string]vdlutil.Any{
				"nested1": store.NestedResult(1),
				"nested2": store.NestedResult(2),
			},
		},
		store.QueryResult{
			NestedResult: 1,
			Name:         "child1",
			Value:        "little1",
			Fields:       nil,
		},
		store.QueryResult{
			NestedResult: 1,
			Name:         "child2",
			Value:        "little2",
			Fields:       nil,
		},
		store.QueryResult{
			NestedResult: 2,
			Name:         "child10",
			Value:        "buddy1",
			Fields:       nil,
		},
		store.QueryResult{
			NestedResult: 2,
			Name:         "child20",
			Value:        "buddy2",
			Fields:       nil,
		},
		store.QueryResult{
			NestedResult: 0,
			Name:         "result3",
			Value:        30,
			Fields:       nil,
		},
	}
	stream := newQueryStream(newMockObjectQueryStream(mockResults))
	advance(t, stream)
	assertNameAndValue(t, stream.Value(), "result1", 10)
	advance(t, stream)
	assertName(t, stream.Value(), "result2")
	fields := stream.Value().Fields()
	{
		nested1Stream := fields["nested1"].(storage.QueryStream)
		advance(t, nested1Stream)
		assertNameAndValue(t, nested1Stream.Value(), "result2/child1", "little1")
		advance(t, nested1Stream)
		assertNameAndValue(t, nested1Stream.Value(), "result2/child2", "little2")
		nested2Stream := fields["nested2"].(storage.QueryStream)
		advance(t, nested2Stream)
		assertNameAndValue(t, nested2Stream.Value(), "result2/child10", "buddy1")
		advance(t, nested2Stream)
		assertNameAndValue(t, nested2Stream.Value(), "result2/child20", "buddy2")
	}
	advance(t, stream)
	assertNameAndValue(t, stream.Value(), "result3", 30)
	assertDone(t, stream)

	// It should be ok to access "nested2" stream before "nested1".
	stream = newQueryStream(newMockObjectQueryStream(mockResults))
	advance(t, stream)
	assertNameAndValue(t, stream.Value(), "result1", 10)
	advance(t, stream)
	assertName(t, stream.Value(), "result2")
	fields = stream.Value().Fields()
	{
		nested2Stream := fields["nested2"].(storage.QueryStream)
		advance(t, nested2Stream)
		assertNameAndValue(t, nested2Stream.Value(), "result2/child10", "buddy1")
		advance(t, nested2Stream)
		assertNameAndValue(t, nested2Stream.Value(), "result2/child20", "buddy2")
		nested1Stream := fields["nested1"].(storage.QueryStream)
		advance(t, nested1Stream)
		assertNameAndValue(t, nested1Stream.Value(), "result2/child1", "little1")
		advance(t, nested1Stream)
		assertNameAndValue(t, nested1Stream.Value(), "result2/child2", "little2")
	}
	advance(t, stream)
	assertNameAndValue(t, stream.Value(), "result3", 30)
	assertDone(t, stream)

	// Canceling the parent stream should invalidate the nested streams.
	stream = newQueryStream(newMockObjectQueryStream(mockResults))
	advance(t, stream)
	assertNameAndValue(t, stream.Value(), "result1", 10)
	advance(t, stream)
	assertName(t, stream.Value(), "result2")
	fields = stream.Value().Fields()
	nested1Stream := fields["nested1"].(storage.QueryStream)
	nested2Stream := fields["nested2"].(storage.QueryStream)
	stream.Cancel()
	assertDone(t, stream)
	assertDone(t, nested1Stream)
	assertDone(t, nested2Stream)

	// Canceling one nested stream should not affect the other.
	stream = newQueryStream(newMockObjectQueryStream(mockResults))
	advance(t, stream)
	assertNameAndValue(t, stream.Value(), "result1", 10)
	advance(t, stream)
	assertName(t, stream.Value(), "result2")
	fields = stream.Value().Fields()
	{
		nested1Stream := fields["nested1"].(storage.QueryStream)
		nested2Stream := fields["nested2"].(storage.QueryStream)
		advance(t, nested1Stream)
		assertNameAndValue(t, nested1Stream.Value(), "result2/child1", "little1")
		nested2Stream.Cancel()
		advance(t, nested1Stream)
		assertNameAndValue(t, nested1Stream.Value(), "result2/child2", "little2")
		assertDone(t, nested1Stream)
		assertDone(t, nested2Stream)
	}
	advance(t, stream)
	assertNameAndValue(t, stream.Value(), "result3", 30)
	assertDone(t, stream)
}
