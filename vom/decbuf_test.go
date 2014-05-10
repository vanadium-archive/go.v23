package vom

import (
	"io"
	"reflect"
	"testing"
	"testing/iotest"
)

// abcReader returns data looping from a-z, up to lim bytes.
type abcReader struct {
	n   int
	lim int
}

func newABCReader(lim int) *abcReader {
	return &abcReader{lim: lim}
}

func newABC(lim int, rtype readerType) (r io.Reader) {
	r = newABCReader(lim)
	if rtype == readerOneByte {
		r = iotest.OneByteReader(r)
	}
	return
}

func (abc *abcReader) Read(p []byte) (int, error) {
	startlen := len(p)
	lim := abc.n + startlen
	if lim > abc.lim {
		lim = abc.lim
	}
	for ; abc.n < lim; abc.n++ {
		p[0] = byte('a' + (abc.n % 26))
		p = p[1:]
	}
	if abc.n == abc.lim {
		return startlen - len(p), io.EOF
	}
	return startlen - len(p), nil
}

// readTracker tracks the size of each read call.
type readTracker struct {
	r     io.Reader
	reads []int
}

func newReadTracker(r io.Reader) *readTracker {
	return &readTracker{r: r}
}

func (r *readTracker) Read(p []byte) (int, error) {
	r.reads = append(r.reads, len(p))
	return r.r.Read(p)
}

func (r *readTracker) expectReads(t *testing.T, expect []int) {
	if !reflect.DeepEqual(r.reads, expect) {
		t.Errorf("readTracker got %v, want %v", r.reads, expect)
	}
}

func expectReadBuf(t *testing.T, b *decbuf, n int, expect string, expectErr error) {
	actual, err := b.ReadBuf(n)
	if err != expectErr {
		t.Errorf("ReadBuf err got %v, want %v", err, expectErr)
	}
	if string(actual) != expect {
		t.Errorf("ReadBuf buf got %q, want %q", actual, expect)
	}
}

func expectRead(t *testing.T, b *decbuf, n int, expect string, expectErr error) {
	actual := make([]byte, n)
	readlen, err := b.Read(actual)
	if err != expectErr {
		t.Errorf("Read err got %v, want %v", err, expectErr)
	}
	if readlen != len(expect) {
		t.Errorf("Read len got %d, want %d", readlen, len(expect))
	}
	if string(actual[:readlen]) != expect {
		t.Errorf("Read buf got %q, want %q", actual, expect)
	}
}

func expectReadByte(t *testing.T, b *decbuf, expect byte, expectErr error) {
	actual, err := b.ReadByte()
	if err != expectErr {
		t.Errorf("ReadByte err got %v, want %v", err, expectErr)
	}
	if actual != expect {
		t.Errorf("ReadByte buf got %q, want %q", actual, expect)
	}
}

func expectPeekAtLeast(t *testing.T, b *decbuf, n int, expect string, expectErr error) {
	actual, err := b.PeekAtLeast(n)
	if err != expectErr {
		t.Errorf("PeekAtLeast err got %v, want %v", err, expectErr)
	}
	if string(actual) != expect {
		t.Errorf("PeekAtLeast buf got %q, want %q", actual, expect)
	}
}

func expectSkip(t *testing.T, b *decbuf, n int, expectErr error) {
	err := b.Skip(n)
	if err != expectErr {
		t.Errorf("Skip err got %v, want %v", err, expectErr)
	}
}

func expectSkipToLimit(t *testing.T, b *decbuf, expect int, expectErr error) {
	actual, err := b.SkipToLimit()
	if err != expectErr {
		t.Errorf("SkipToLimit err got %v, want %v", err, expectErr)
	}
	if actual != expect {
		t.Errorf("SkipToLimit leftover got %v, want %v", actual, expect)
	}
}

func testDecbufReadBuf(t *testing.T, rtype readerType) {
	tracker := newReadTracker(newABC(30, rtype))
	b := newDecbuf(tracker, 5)
	// Read full buffer in one read.
	expectReadBuf(t, b, 5, "abcde", nil)
	// Read full buffer in two reads.
	expectReadBuf(t, b, 2, "fg", nil)
	expectReadBuf(t, b, 3, "hij", nil)
	// Read partial buffer, then a full buffer.
	expectReadBuf(t, b, 3, "klm", nil)
	expectReadBuf(t, b, 5, "nopqr", nil)
	// Read multiple partial buffers.
	expectReadBuf(t, b, 3, "stu", nil)
	expectReadBuf(t, b, 3, "vwx", nil)
	expectReadBuf(t, b, 3, "yza", nil)
	expectReadBuf(t, b, 3, "bcd", nil)
	expectReadBuf(t, b, 1, "", io.EOF)
	if rtype == readerRegular {
		tracker.expectReads(t, []int{5, 5, 5, 3, 5, 3, 3, 3, 2})
	}
}
func TestDecbufReadBuf(t *testing.T) {
	testDecbufReadBuf(t, readerRegular)
}
func TestDecbufReadBufOneByte(t *testing.T) {
	testDecbufReadBuf(t, readerOneByte)
}

func testDecbufSkip(t *testing.T, rtype readerType) {
	tracker := newReadTracker(newABC(30, rtype))
	b := newDecbuf(tracker, 5)
	// Skip full buffer in one skip.
	expectSkip(t, b, 5, nil)
	// Skip full buffer in two skips.
	expectSkip(t, b, 2, nil)
	expectSkip(t, b, 3, nil)
	// Skip partial buffer, then a full buffer.
	expectSkip(t, b, 3, nil)
	expectSkip(t, b, 5, nil)
	// Skip multiple partial buffers.
	expectSkip(t, b, 3, nil)
	expectSkip(t, b, 3, nil)
	expectSkip(t, b, 3, nil)
	expectReadBuf(t, b, 3, "bcd", nil) // Make sure we're at the right place.
	expectSkip(t, b, 1, io.EOF)
	if rtype == readerRegular {
		tracker.expectReads(t, []int{5, 5, 5, 3, 5, 3, 3, 3, 2})
	}
}
func TestDecbufSkip(t *testing.T) {
	testDecbufSkip(t, readerRegular)
}
func TestDecbufSkipOneByte(t *testing.T) {
	testDecbufSkip(t, readerOneByte)
}

func testDecbufBigSkip(t *testing.T, rtype readerType) {
	tracker := newReadTracker(newABC(30, rtype))
	b := newDecbuf(tracker, 5)
	// Skip multiple buffers in one skip.
	expectSkip(t, b, 27, nil)
	expectReadBuf(t, b, 3, "bcd", nil) // Make sure we're at the right place.
	expectSkip(t, b, 1, io.EOF)
	if rtype == readerRegular {
		tracker.expectReads(t, []int{5, 5, 5, 5, 5, 5, 5})
	}
}
func TestDecbufBigSkip(t *testing.T) {
	testDecbufBigSkip(t, readerRegular)
}
func TestDecbufBigSkipOneByte(t *testing.T) {
	testDecbufBigSkip(t, readerOneByte)
}

func testDecbufRead(t *testing.T, rtype readerType) {
	tracker := newReadTracker(newABC(30, rtype))
	b := newDecbuf(tracker, 5)
	// Read full buffer in one read.
	expectRead(t, b, 5, "abcde", nil)
	// Read full buffer in two reads.
	expectRead(t, b, 2, "fg", nil)
	expectRead(t, b, 3, "hij", nil)
	// Read partial buffer, then a full buffer.
	expectRead(t, b, 3, "klm", nil)
	expectRead(t, b, 5, "nopqr", nil)
	// Read multiple partial buffers.
	expectRead(t, b, 3, "stu", nil)
	expectRead(t, b, 3, "vwx", nil)
	expectRead(t, b, 3, "yza", nil)
	expectRead(t, b, 3, "bcd", nil)
	expectRead(t, b, 1, "", io.EOF)
	if rtype == readerRegular {
		tracker.expectReads(t, []int{5, 5, 5, 3, 5, 3, 3, 3, 2})
	}
}
func TestDecbufRead(t *testing.T) {
	testDecbufRead(t, readerRegular)
}
func TestDecbufReadOneByte(t *testing.T) {
	testDecbufRead(t, readerOneByte)
}

func testDecbufBigRead(t *testing.T, rtype readerType) {
	tracker := newReadTracker(newABC(30, rtype))
	b := newDecbuf(tracker, 5)
	// Read multiple buffers in one read.
	expectRead(t, b, 27, "abcdefghijklmnopqrstuvwxyza", nil)
	expectRead(t, b, 3, "bcd", nil)
	expectRead(t, b, 1, "", io.EOF)
	if rtype == readerRegular {
		tracker.expectReads(t, []int{5, 5, 5, 5, 5, 5, 5})
	}
}
func TestDecbufBigRead(t *testing.T) {
	testDecbufBigRead(t, readerRegular)
}
func TestDecbufBigReadOneByte(t *testing.T) {
	testDecbufBigRead(t, readerOneByte)
}

func testDecbufReadByte(t *testing.T, rtype readerType) {
	tracker := newReadTracker(newABC(7, rtype))
	b := newDecbuf(tracker, 5)
	expectReadByte(t, b, 'a', nil)
	expectReadByte(t, b, 'b', nil)
	expectReadByte(t, b, 'c', nil)
	expectReadByte(t, b, 'd', nil)
	expectReadByte(t, b, 'e', nil)
	expectReadByte(t, b, 'f', nil)
	expectReadByte(t, b, 'g', nil)
	expectReadByte(t, b, 0, io.EOF)
	if rtype == readerRegular {
		tracker.expectReads(t, []int{5, 5, 3})
	}
}
func TestDecbufReadByte(t *testing.T) {
	testDecbufReadByte(t, readerRegular)
}
func TestDecbufReadByteOneByte(t *testing.T) {
	testDecbufReadByte(t, readerOneByte)
}

func TestDecbufPeekAtLeast(t *testing.T) {
	tracker := newReadTracker(newABC(30, readerRegular))
	b := newDecbuf(tracker, 5)
	// Peek full buffer in one peek.
	expectPeekAtLeast(t, b, 5, "abcde", nil)
	b.SkipPeek(5)
	// Peek full buffer in two peeks.
	expectPeekAtLeast(t, b, 2, "fghij", nil)
	b.SkipPeek(2)
	expectPeekAtLeast(t, b, 3, "hij", nil)
	b.SkipPeek(3)
	// Peek partial buffer, then a full buffer.
	expectPeekAtLeast(t, b, 3, "klmno", nil)
	b.SkipPeek(3)
	expectPeekAtLeast(t, b, 5, "nopqr", nil)
	b.SkipPeek(5)
	// Peek multiple partial buffers.
	expectPeekAtLeast(t, b, 3, "stuvw", nil)
	b.SkipPeek(3)
	expectPeekAtLeast(t, b, 3, "vwxyz", nil)
	b.SkipPeek(3)
	expectPeekAtLeast(t, b, 3, "yzabc", nil)
	b.SkipPeek(3)
	expectPeekAtLeast(t, b, 3, "bcd", nil)
	b.SkipPeek(3)
	expectPeekAtLeast(t, b, 1, "", io.EOF)
	tracker.expectReads(t, []int{5, 5, 5, 3, 5, 3, 3, 3, 2})
}

func TestDecbufPeekAtLeastOneByte(t *testing.T) {
	tracker := newReadTracker(newABC(30, readerOneByte))
	b := newDecbuf(tracker, 5)
	// Peek full buffer in one peek.
	expectPeekAtLeast(t, b, 5, "abcde", nil)
	b.SkipPeek(5)
	// Peek full buffer in two peeks.
	expectPeekAtLeast(t, b, 2, "fg", nil)
	b.SkipPeek(2)
	expectPeekAtLeast(t, b, 3, "hij", nil)
	b.SkipPeek(3)
	// Peek partial buffer, then a full buffer.
	expectPeekAtLeast(t, b, 3, "klm", nil)
	b.SkipPeek(3)
	expectPeekAtLeast(t, b, 5, "nopqr", nil)
	b.SkipPeek(5)
	// Peek multiple partial buffers.
	expectPeekAtLeast(t, b, 3, "stu", nil)
	b.SkipPeek(3)
	expectPeekAtLeast(t, b, 3, "vwx", nil)
	b.SkipPeek(3)
	expectPeekAtLeast(t, b, 3, "yza", nil)
	b.SkipPeek(3)
	expectPeekAtLeast(t, b, 3, "bcd", nil)
	b.SkipPeek(3)
	expectPeekAtLeast(t, b, 1, "", io.EOF)
}

func testDecbufSkipToLimit(t *testing.T, rtype readerType) {
	tracker := newReadTracker(newABC(24, rtype))
	b := newDecbuf(tracker, 5)
	// Read full buffer, nothing to skip.
	b.SetLimit(5)
	expectRead(t, b, 5, "abcde", nil)
	expectSkipToLimit(t, b, 0, nil)
	// Read full buffer in two reads, nothing to skip.
	b.SetLimit(5)
	expectRead(t, b, 2, "fg", nil)
	expectRead(t, b, 3, "hij", nil)
	expectSkipToLimit(t, b, 0, nil)
	// Skip partial buffer.
	b.SetLimit(5)
	expectRead(t, b, 3, "klm", nil)
	expectSkipToLimit(t, b, 2, nil)
	// Skip more than one buffer.
	b.SetLimit(8)
	expectRead(t, b, 1, "p", nil)
	expectSkipToLimit(t, b, 7, nil)
	// Make sure we're at the right place.
	expectRead(t, b, 1, "x", nil)
	// EOF
	b.SetLimit(1)
	expectSkipToLimit(t, b, 0, io.EOF)
	if rtype == readerRegular {
		tracker.expectReads(t, []int{5, 5, 5, 5, 1, 5, 2})
	}
}
func TestDecbufSkipToLimit(t *testing.T) {
	testDecbufSkipToLimit(t, readerRegular)
}
func TestDecbufSkipToLimitOneByte(t *testing.T) {
	testDecbufSkipToLimit(t, readerOneByte)
}
