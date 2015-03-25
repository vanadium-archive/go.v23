// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vom

// TODO(toddw): Add more exhaustive tests of decbuf SetLimit and fill.

import (
	"io"
	"strings"
	"testing"
)

func expectEncbufBytes(t *testing.T, b *encbuf, expect string) {
	if got, want := b.Len(), len(expect); got != want {
		t.Errorf("len got %d, want %d", got, want)
	}
	if got, want := string(b.Bytes()), expect; got != want {
		t.Errorf("bytes got %q, want %q", b.Bytes(), expect)
	}
}

func TestEncbuf(t *testing.T) {
	b := newEncbuf()
	expectEncbufBytes(t, b, "")
	copy(b.Grow(5), "abcde*****")
	expectEncbufBytes(t, b, "abcde")
	b.Truncate(3)
	expectEncbufBytes(t, b, "abc")
	b.WriteByte('1')
	expectEncbufBytes(t, b, "abc1")
	b.Write([]byte("def"))
	expectEncbufBytes(t, b, "abc1def")
	b.Truncate(1)
	expectEncbufBytes(t, b, "a")
	b.WriteString("XYZ")
	expectEncbufBytes(t, b, "aXYZ")
	b.Reset()
	expectEncbufBytes(t, b, "")
	b.WriteString("123")
	expectEncbufBytes(t, b, "123")
}

func testEncbufReserve(t *testing.T, base int) {
	for _, size := range []int{base - 1, base, base + 1} {
		str := strings.Repeat("x", size)
		// Test starting empty and writing size.
		b := newEncbuf()
		b.WriteString(str)
		expectEncbufBytes(t, b, str)
		b.WriteString(str)
		expectEncbufBytes(t, b, str+str)
		// Test starting with one byte and writing size.
		b = newEncbuf()
		b.WriteByte('A')
		expectEncbufBytes(t, b, "A")
		b.WriteString(str)
		expectEncbufBytes(t, b, "A"+str)
		b.WriteString(str)
		expectEncbufBytes(t, b, "A"+str+str)
	}
}

func TestEncbufReserve(t *testing.T) {
	testEncbufReserve(t, minBufFree)
	testEncbufReserve(t, minBufFree*2)
	testEncbufReserve(t, minBufFree*3)
	testEncbufReserve(t, minBufFree*4)
}

func expectRemoveLimit(t *testing.T, mode readMode, b *decbuf, expect int) {
	if got, want := b.RemoveLimit(), expect; got != want {
		t.Errorf("%s RemoveLimit got %v, want %v", mode, got, want)
	}
}

func expectReadBuf(t *testing.T, mode readMode, b *decbuf, n int, expect string, expectErr error) {
	buf, err := b.ReadBuf(n)
	if got, want := err, expectErr; got != want {
		t.Errorf("%s ReadBuf err got %v, want %v", mode, got, want)
	}
	if got, want := string(buf), expect; got != want {
		t.Errorf("%s ReadBuf buf got %q, want %q", mode, got, want)
	}
}

func expectSkip(t *testing.T, mode readMode, b *decbuf, n int, expectErr error) {
	if got, want := b.Skip(n), expectErr; got != want {
		t.Errorf("%s Skip err got %v, want %v", mode, got, want)
	}
}

func expectPeekAtLeast(t *testing.T, mode readMode, b *decbuf, n int, expect string, expectErr error) {
	buf, err := b.PeekAtLeast(n)
	if got, want := err, expectErr; got != want {
		t.Errorf("%s PeekAtLeast err got %v, want %v", mode, got, want)
	}
	if got, want := string(buf), expect; got != want {
		t.Errorf("%s PeekAtLeast buf got %q, want %q", mode, got, want)
	}
}

func expectReadByte(t *testing.T, mode readMode, b *decbuf, expect byte, expectErr error) {
	actual, err := b.ReadByte()
	if got, want := err, expectErr; got != want {
		t.Errorf("%s ReadByte err got %v, want %v", mode, got, want)
	}
	if got, want := actual, expect; got != want {
		t.Errorf("%s ReadByte buf got %q, want %q", mode, got, want)
	}
}

func expectPeekByte(t *testing.T, mode readMode, b *decbuf, expect byte, expectErr error) {
	actual, err := b.PeekByte()
	if got, want := err, expectErr; got != want {
		t.Errorf("%s PeekByte err got %v, want %v", mode, got, want)
	}
	if got, want := actual, expect; got != want {
		t.Errorf("%s PeekByte buf got %q, want %q", mode, got, want)
	}
}

func expectReadFull(t *testing.T, mode readMode, b *decbuf, n int, expect string, expectErr error) {
	buf := make([]byte, n)
	err := b.ReadFull(buf)
	if got, want := err, expectErr; got != want {
		t.Errorf("%s ReadFull err got %v, want %v", mode, got, want)
	}
	if err == nil {
		if got, want := string(buf), expect; got != want {
			t.Errorf("%s ReadFull buf got %q, want %q", mode, got, want)
		}
	}
}

func TestDecbufReadBuf(t *testing.T) {
	for _, mode := range allReadModes {
		b := newDecbuf(mode.testReader(abcReader(10)))
		expectReadBuf(t, mode, b, 1, "a", nil)
		expectReadBuf(t, mode, b, 2, "bc", nil)
		expectReadBuf(t, mode, b, 3, "def", nil)
		expectReadBuf(t, mode, b, 4, "ghij", nil)
		expectReadBuf(t, mode, b, 1, "", io.EOF)
		expectReadBuf(t, mode, b, 1, "", io.EOF)
	}
}

func TestDecbufReadBufLimit(t *testing.T) {
	for _, mode := range allReadModes {
		b := newDecbuf(mode.testReader(abcReader(10)))
		// Read exactly up to limit.
		b.SetLimit(3)
		expectReadBuf(t, mode, b, 1, "a", nil)
		expectReadBuf(t, mode, b, 2, "bc", nil)
		expectRemoveLimit(t, mode, b, 0)
		// Read less than the limit.
		b.SetLimit(3)
		expectReadBuf(t, mode, b, 2, "de", nil)
		expectRemoveLimit(t, mode, b, 1)
		// Read more than the limit.
		b.SetLimit(3)
		expectReadBuf(t, mode, b, 4, "", io.EOF)
		expectRemoveLimit(t, mode, b, 0)

		expectReadBuf(t, mode, b, 1, "f", nil)
	}
}

func TestDecbufSkip(t *testing.T) {
	for _, mode := range allReadModes {
		b := newDecbuf(mode.testReader(abcReader(10)))
		expectSkip(t, mode, b, 1, nil)
		expectReadBuf(t, mode, b, 2, "bc", nil)
		expectSkip(t, mode, b, 3, nil)
		expectReadBuf(t, mode, b, 2, "gh", nil)
		expectSkip(t, mode, b, 2, nil)
		expectSkip(t, mode, b, 1, io.EOF)
		expectSkip(t, mode, b, 1, io.EOF)
		expectReadBuf(t, mode, b, 1, "", io.EOF)
		expectReadBuf(t, mode, b, 1, "", io.EOF)
	}
}

func TestDecbufSkipLimit(t *testing.T) {
	for _, mode := range allReadModes {
		b := newDecbuf(mode.testReader(abcReader(10)))
		// Skip exactly up to limit.
		b.SetLimit(3)
		expectSkip(t, mode, b, 1, nil)
		expectSkip(t, mode, b, 2, nil)
		expectRemoveLimit(t, mode, b, 0)
		// Skip less than the limit.
		b.SetLimit(3)
		expectSkip(t, mode, b, 2, nil)
		expectRemoveLimit(t, mode, b, 1)
		// Skip more than the limit.
		b.SetLimit(3)
		expectSkip(t, mode, b, 4, io.EOF)
		expectRemoveLimit(t, mode, b, 0)

		expectReadBuf(t, mode, b, 1, "f", nil)
	}
}

func TestDecbufPeekAtLeast(t *testing.T) {
	for _, mode := range []readMode{readAll, readHalf, readAllEOF, readHalfEOF} {
		// Start peeking at beginning.
		b := newDecbuf(mode.testReader(abcReader(3)))
		expectPeekAtLeast(t, mode, b, 1, "abc", nil)
		expectPeekAtLeast(t, mode, b, 2, "abc", nil)
		expectPeekAtLeast(t, mode, b, 3, "abc", nil)
		expectPeekAtLeast(t, mode, b, 4, "", io.EOF)
		expectReadBuf(t, mode, b, 3, "abc", nil)
		expectReadBuf(t, mode, b, 1, "", io.EOF)
		expectPeekAtLeast(t, mode, b, 1, "", io.EOF)
		// Start peeking after reading 1 byte, which fills the buffer
		b = newDecbuf(mode.testReader(abcReader(4)))
		expectReadBuf(t, mode, b, 1, "a", nil)
		expectPeekAtLeast(t, mode, b, 1, "bcd", nil)
		expectPeekAtLeast(t, mode, b, 2, "bcd", nil)
		expectPeekAtLeast(t, mode, b, 3, "bcd", nil)
		expectPeekAtLeast(t, mode, b, 4, "", io.EOF)
		expectReadBuf(t, mode, b, 3, "bcd", nil)
		expectReadBuf(t, mode, b, 1, "", io.EOF)
		expectPeekAtLeast(t, mode, b, 1, "", io.EOF)
	}
	for _, mode := range []readMode{readOneByte, readOneByteEOF} {
		// Start peeking at beginning.
		b := newDecbuf(mode.testReader(abcReader(3)))
		expectPeekAtLeast(t, mode, b, 1, "a", nil)
		expectPeekAtLeast(t, mode, b, 2, "ab", nil)
		expectPeekAtLeast(t, mode, b, 3, "abc", nil)
		expectPeekAtLeast(t, mode, b, 2, "abc", nil)
		expectPeekAtLeast(t, mode, b, 1, "abc", nil)
		expectPeekAtLeast(t, mode, b, 4, "", io.EOF)
		expectReadBuf(t, mode, b, 3, "abc", nil)
		expectReadBuf(t, mode, b, 1, "", io.EOF)
		expectPeekAtLeast(t, mode, b, 1, "", io.EOF)
		// Start peeking after reading 1 byte, which fills the buffer
		b = newDecbuf(mode.testReader(abcReader(4)))
		expectReadBuf(t, mode, b, 1, "a", nil)
		expectPeekAtLeast(t, mode, b, 1, "b", nil)
		expectPeekAtLeast(t, mode, b, 2, "bc", nil)
		expectPeekAtLeast(t, mode, b, 3, "bcd", nil)
		expectPeekAtLeast(t, mode, b, 2, "bcd", nil)
		expectPeekAtLeast(t, mode, b, 1, "bcd", nil)
		expectPeekAtLeast(t, mode, b, 4, "", io.EOF)
		expectReadBuf(t, mode, b, 3, "bcd", nil)
		expectReadBuf(t, mode, b, 1, "", io.EOF)
		expectPeekAtLeast(t, mode, b, 1, "", io.EOF)
	}
}

func TestDecbufReadByte(t *testing.T) {
	for _, mode := range allReadModes {
		b := newDecbuf(mode.testReader(abcReader(3)))
		expectReadByte(t, mode, b, 'a', nil)
		expectReadByte(t, mode, b, 'b', nil)
		expectReadByte(t, mode, b, 'c', nil)
		expectReadByte(t, mode, b, 0, io.EOF)
		expectReadByte(t, mode, b, 0, io.EOF)
	}
}

func TestDecbufReadByteLimit(t *testing.T) {
	for _, mode := range allReadModes {
		b := newDecbuf(mode.testReader(abcReader(10)))
		// Read exactly up to limit.
		b.SetLimit(2)
		expectReadByte(t, mode, b, 'a', nil)
		expectReadByte(t, mode, b, 'b', nil)
		expectRemoveLimit(t, mode, b, 0)
		// Read less than the limit.
		b.SetLimit(2)
		expectReadByte(t, mode, b, 'c', nil)
		expectRemoveLimit(t, mode, b, 1)
		// Read more than the limit.
		b.SetLimit(2)
		expectReadByte(t, mode, b, 'd', nil)
		expectReadByte(t, mode, b, 'e', nil)
		expectReadByte(t, mode, b, 0, io.EOF)
		expectReadByte(t, mode, b, 0, io.EOF)
		expectRemoveLimit(t, mode, b, 0)

		expectReadByte(t, mode, b, 'f', nil)
	}
}

func TestDecbufPeekByte(t *testing.T) {
	for _, mode := range allReadModes {
		b := newDecbuf(mode.testReader(abcReader(3)))
		expectPeekByte(t, mode, b, 'a', nil)
		expectPeekByte(t, mode, b, 'a', nil)
		expectReadByte(t, mode, b, 'a', nil)

		expectPeekByte(t, mode, b, 'b', nil)
		expectPeekByte(t, mode, b, 'b', nil)
		expectReadByte(t, mode, b, 'b', nil)

		expectPeekByte(t, mode, b, 'c', nil)
		expectPeekByte(t, mode, b, 'c', nil)
		expectReadByte(t, mode, b, 'c', nil)

		expectPeekByte(t, mode, b, 0, io.EOF)
		expectPeekByte(t, mode, b, 0, io.EOF)
		expectReadByte(t, mode, b, 0, io.EOF)
	}
}

func TestDecbufPeekByteLimit(t *testing.T) {
	for _, mode := range allReadModes {
		b := newDecbuf(mode.testReader(abcReader(10)))
		// Read exactly up to limit.
		b.SetLimit(2)
		expectPeekByte(t, mode, b, 'a', nil)
		expectPeekByte(t, mode, b, 'a', nil)
		expectReadByte(t, mode, b, 'a', nil)

		expectPeekByte(t, mode, b, 'b', nil)
		expectPeekByte(t, mode, b, 'b', nil)
		expectReadByte(t, mode, b, 'b', nil)
		expectRemoveLimit(t, mode, b, 0)
		// Read less than the limit.
		b.SetLimit(2)
		expectPeekByte(t, mode, b, 'c', nil)
		expectPeekByte(t, mode, b, 'c', nil)
		expectReadByte(t, mode, b, 'c', nil)
		expectRemoveLimit(t, mode, b, 1)
		// Read more than the limit.
		b.SetLimit(2)
		expectPeekByte(t, mode, b, 'd', nil)
		expectPeekByte(t, mode, b, 'd', nil)
		expectReadByte(t, mode, b, 'd', nil)

		expectPeekByte(t, mode, b, 'e', nil)
		expectPeekByte(t, mode, b, 'e', nil)
		expectReadByte(t, mode, b, 'e', nil)

		expectPeekByte(t, mode, b, 0, io.EOF)
		expectPeekByte(t, mode, b, 0, io.EOF)
		expectReadByte(t, mode, b, 0, io.EOF)
		expectRemoveLimit(t, mode, b, 0)

		expectPeekByte(t, mode, b, 'f', nil)
	}
}

func TestDecbufReadFull(t *testing.T) {
	for _, mode := range allReadModes {
		// Start ReadFull from beginning.
		b := newDecbuf(mode.testReader(abcReader(6)))
		expectReadFull(t, mode, b, 3, "abc", nil)
		expectReadFull(t, mode, b, 3, "def", nil)
		expectReadFull(t, mode, b, 1, "", io.EOF)
		expectReadFull(t, mode, b, 1, "", io.EOF)
		// Start ReadFull after reading 1 byte, which fills the buffer.
		b = newDecbuf(mode.testReader(abcReader(6)))
		expectReadBuf(t, mode, b, 1, "a", nil)
		expectReadFull(t, mode, b, 2, "bc", nil)
		expectReadFull(t, mode, b, 3, "def", nil)
		expectReadFull(t, mode, b, 1, "", io.EOF)
		expectReadFull(t, mode, b, 1, "", io.EOF)
	}
}
