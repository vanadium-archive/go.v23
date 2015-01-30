package context

import (
	"sync"
	"testing"
	"time"
)

func testCancel(t *testing.T, ctx *T, cancel CancelFunc) {
	select {
	case <-ctx.Done():
		t.Errorf("Done closed when deadline not yet passed")
	default:
	}
	ch := make(chan bool, 0)
	go func() {
		cancel()
		close(ch)
	}()
	select {
	case <-ch:
	case <-time.After(3 * time.Second):
		t.Fatal("timed out witing for cancel.")
	}

	select {
	case <-ctx.Done():
	case <-time.After(3 * time.Second):
		t.Fatal("timed out witing for cancellation.")
	}
	if err := ctx.Err(); err != Canceled {
		t.Errorf("Unexpected error want %v, got %v", Canceled, err)
	}
}

func TestRootContext(t *testing.T) {
	var ctx *T

	if ctx.Initialized() {
		t.Error("Nil context should be uninitialized")
	}
	if got := ctx.Err(); got != nil {
		t.Errorf("Expected nil error, got: %v", got)
	}
	ctx = &T{}
	if ctx.Initialized() {
		t.Error("Zero context should be uninitialized")
	}
	if got := ctx.Err(); got != nil {
		t.Errorf("Expected nil error, got: %v", got)
	}
}

func TestCancelContext(t *testing.T) {
	root, _ := RootContext()
	ctx, cancel := WithCancel(root)
	testCancel(t, ctx, cancel)

	// Test cancelling a cancel context which is the child
	// of a cancellable context.
	parent, _ := WithCancel(root)
	child, cancel := WithCancel(parent)
	cancel()
	<-child.Done()

	// Test adding a cancellable child context after the parent is
	// already cancelled.
	parent, cancel = WithCancel(root)
	cancel()
	child, _ = WithCancel(parent)
	<-child.Done() // The child should have been cancelled right away.
}

func TestMultiLevelCancelContext(t *testing.T) {
	root, _ := RootContext()
	c0, c0Cancel := WithCancel(root)
	c1, _ := WithCancel(c0)
	c2, _ := WithCancel(c1)
	c3, _ := WithCancel(c2)
	testCancel(t, c3, c0Cancel)
}

func testDeadline(t *testing.T, ctx *T, start time.Time, desiredTimeout time.Duration) {
	<-ctx.Done()
	if delta := time.Now().Sub(start); delta < desiredTimeout {
		t.Errorf("Deadline too short want %s got %s", desiredTimeout, delta)
	}
	if err := ctx.Err(); err != DeadlineExceeded {
		t.Errorf("Unexpected error want %s, got %s", DeadlineExceeded, err)
	}
}

func TestDeadlineContext(t *testing.T) {
	cases := []time.Duration{
		3 * time.Millisecond,
		0,
	}
	rootCtx, _ := RootContext()
	cancelCtx, _ := WithCancel(rootCtx)
	deadlineCtx, _ := WithDeadline(rootCtx, time.Now().Add(time.Hour))

	for _, desiredTimeout := range cases {
		// Test all the various ways of getting deadline contexts.
		start := time.Now()
		ctx, _ := WithDeadline(rootCtx, start.Add(desiredTimeout))
		testDeadline(t, ctx, start, desiredTimeout)

		start = time.Now()
		ctx, _ = WithDeadline(cancelCtx, start.Add(desiredTimeout))
		testDeadline(t, ctx, start, desiredTimeout)

		start = time.Now()
		ctx, _ = WithDeadline(deadlineCtx, start.Add(desiredTimeout))
		testDeadline(t, ctx, start, desiredTimeout)

		start = time.Now()
		ctx, _ = WithTimeout(rootCtx, desiredTimeout)
		testDeadline(t, ctx, start, desiredTimeout)

		start = time.Now()
		ctx, _ = WithTimeout(cancelCtx, desiredTimeout)
		testDeadline(t, ctx, start, desiredTimeout)

		start = time.Now()
		ctx, _ = WithTimeout(deadlineCtx, desiredTimeout)
		testDeadline(t, ctx, start, desiredTimeout)
	}

	ctx, cancel := WithDeadline(rootCtx, time.Now().Add(100*time.Hour))
	testCancel(t, ctx, cancel)
}

func TestDeadlineContextWithRace(t *testing.T) {
	root, _ := RootContext()
	ctx, cancel := WithDeadline(root, time.Now().Add(100*time.Hour))
	var wg sync.WaitGroup
	wg.Add(10)
	for i := 0; i < 10; i++ {
		go func() {
			cancel()
			wg.Done()
		}()
	}
	wg.Wait()
	<-ctx.Done()
	if err := ctx.Err(); err != Canceled {
		t.Errorf("Unexpected error want %v, got %v", Canceled, err)
	}
}

func TestValueContext(t *testing.T) {
	type testContextKey int
	const (
		key1 = testContextKey(iota)
		key2
		key3
		key4
	)
	const (
		val1 = iota
		val2
		val3
	)
	root, _ := RootContext()
	ctx1 := WithValue(root, key1, val1)
	ctx2 := WithValue(ctx1, key2, val2)
	ctx3 := WithValue(ctx2, key3, val3)

	expected := map[interface{}]interface{}{
		key1: val1,
		key2: val2,
		key3: val3,
		key4: nil,
	}
	for k, v := range expected {
		if got := ctx3.Value(k); got != v {
			t.Errorf("Got wrong value for %v: want %v got %v", k, v, got)
		}
	}

}

func TestRootCancel(t *testing.T) {
	root, rootcancel := RootContext()
	a, acancel := WithCancel(root)
	b := WithValue(a, "key", "value")

	c, ccancel := WithRootCancel(b)
	d, _ := WithCancel(c)

	e, _ := WithRootCancel(b)

	if s, ok := d.Value("key").(string); !ok || s != "value" {
		t.Error("Lost a value but shouldn't have.")
	}

	// If we cancel a, b will get canceled but c will not.
	acancel()
	<-a.Done()
	<-b.Done()
	select {
	case <-c.Done():
		t.Error("C should not yet be canceled.")
	case <-e.Done():
		t.Error("E should not yet be canceled.")
	case <-time.After(100 * time.Millisecond):
	}

	// Cancelling c should still cancel d.
	ccancel()
	<-c.Done()
	<-d.Done()

	// Cancelling the root should cancel e.
	rootcancel()
	<-root.Done()
	<-e.Done()
}
