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

func expectRemoveLimit(t *testing.T, mode string, b *decbuf, expect int) {
	if got, want := b.RemoveLimit(), expect; got != want {
		t.Errorf("%s RemoveLimit got %v, want %v", mode, got, want)
	}
}

func expectReadBuf(t *testing.T, mode string, b *decbuf, n int, expect string, expectErr error) {
	buf, err := b.ReadBuf(n)
	if got, want := err, expectErr; got != want {
		t.Errorf("%s ReadBuf err got %v, want %v", mode, got, want)
	}
	if got, want := string(buf), expect; got != want {
		t.Errorf("%s ReadBuf buf got %q, want %q", mode, got, want)
	}
}

func expectSkip(t *testing.T, mode string, b *decbuf, n int, expectErr error) {
	if got, want := b.Skip(n), expectErr; got != want {
		t.Errorf("%s Skip err got %v, want %v", mode, got, want)
	}
}

func expectPeekAtLeast(t *testing.T, mode string, b *decbuf, n int, expect string, expectErr error) {
	buf, err := b.PeekAtLeast(n)
	if got, want := err, expectErr; got != want {
		t.Errorf("%s PeekAtLeast err got %v, want %v", mode, got, want)
	}
	if got, want := string(buf), expect; got != want {
		t.Errorf("%s PeekAtLeast buf got %q, want %q", mode, got, want)
	}
}

func expectReadByte(t *testing.T, mode string, b *decbuf, expect byte, expectErr error) {
	actual, err := b.ReadByte()
	if got, want := err, expectErr; got != want {
		t.Errorf("%s ReadByte err got %v, want %v", mode, got, want)
	}
	if got, want := actual, expect; got != want {
		t.Errorf("%s ReadByte buf got %q, want %q", mode, got, want)
	}
}

func expectPeekByte(t *testing.T, mode string, b *decbuf, expect byte, expectErr error) {
	actual, err := b.PeekByte()
	if got, want := err, expectErr; got != want {
		t.Errorf("%s PeekByte err got %v, want %v", mode, got, want)
	}
	if got, want := actual, expect; got != want {
		t.Errorf("%s PeekByte buf got %q, want %q", mode, got, want)
	}
}

func expectReadFull(t *testing.T, mode string, b *decbuf, n int, expect string, expectErr error) {
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
	fn := func(mode string, b *decbuf) {
		expectReadBuf(t, mode, b, 1, "a", nil)
		expectReadBuf(t, mode, b, 2, "bc", nil)
		expectReadBuf(t, mode, b, 3, "def", nil)
		expectReadBuf(t, mode, b, 4, "ghij", nil)
		expectReadBuf(t, mode, b, 1, "", io.EOF)
		expectReadBuf(t, mode, b, 1, "", io.EOF)
	}
	for _, mode := range allReadModes {
		fn(mode.String(), newDecbuf(mode.testReader(abcReader(10))))
	}
	fn("fromBytes", newDecbufFromBytes(abcBytes(10)))
}

func TestDecbufReadBufLimit(t *testing.T) {
	fn := func(mode string, b *decbuf) {
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
	for _, mode := range allReadModes {
		fn(mode.String(), newDecbuf(mode.testReader(abcReader(10))))
	}
	fn("fromBytes", newDecbufFromBytes(abcBytes(10)))
}

func TestDecbufSkip(t *testing.T) {
	fn := func(mode string, b *decbuf) {
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
	for _, mode := range allReadModes {
		fn(mode.String(), newDecbuf(mode.testReader(abcReader(10))))
	}
	fn("fromBytes", newDecbufFromBytes(abcBytes(10)))
}

func TestDecbufSkipLimit(t *testing.T) {
	fn := func(mode string, b *decbuf) {
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
	for _, mode := range allReadModes {
		fn(mode.String(), newDecbuf(mode.testReader(abcReader(10))))
	}
	fn("fromBytes", newDecbufFromBytes(abcBytes(10)))
}

func TestDecbufPeekAtLeast(t *testing.T) {
	fn1 := func(mode string, b *decbuf) {
		// Start peeking at beginning.
		expectPeekAtLeast(t, mode, b, 1, "abc", nil)
		expectPeekAtLeast(t, mode, b, 2, "abc", nil)
		expectPeekAtLeast(t, mode, b, 3, "abc", nil)
		expectPeekAtLeast(t, mode, b, 4, "", io.EOF)
		expectReadBuf(t, mode, b, 3, "abc", nil)
		expectReadBuf(t, mode, b, 1, "", io.EOF)
		expectPeekAtLeast(t, mode, b, 1, "", io.EOF)
	}
	fn2 := func(mode string, b *decbuf) {
		// Start peeking after reading 1 byte, which fills the buffer
		expectReadBuf(t, mode, b, 1, "a", nil)
		expectPeekAtLeast(t, mode, b, 1, "bcd", nil)
		expectPeekAtLeast(t, mode, b, 2, "bcd", nil)
		expectPeekAtLeast(t, mode, b, 3, "bcd", nil)
		expectPeekAtLeast(t, mode, b, 4, "", io.EOF)
		expectReadBuf(t, mode, b, 3, "bcd", nil)
		expectReadBuf(t, mode, b, 1, "", io.EOF)
		expectPeekAtLeast(t, mode, b, 1, "", io.EOF)
	}
	for _, mode := range []readMode{readAll, readHalf, readAllEOF, readHalfEOF} {
		fn1(mode.String(), newDecbuf(mode.testReader(abcReader(3))))
		fn2(mode.String(), newDecbuf(mode.testReader(abcReader(4))))
	}
	fn1("fromBytes", newDecbufFromBytes(abcBytes(3)))
	fn2("fromBytes", newDecbufFromBytes(abcBytes(4)))
}

func TestDecbufPeekAtLeast1(t *testing.T) {
	fn1 := func(mode string, b *decbuf) {
		// Start peeking at beginning.
		expectPeekAtLeast(t, mode, b, 1, "a", nil)
		expectPeekAtLeast(t, mode, b, 2, "ab", nil)
		expectPeekAtLeast(t, mode, b, 3, "abc", nil)
		expectPeekAtLeast(t, mode, b, 2, "abc", nil)
		expectPeekAtLeast(t, mode, b, 1, "abc", nil)
		expectPeekAtLeast(t, mode, b, 4, "", io.EOF)
		expectReadBuf(t, mode, b, 3, "abc", nil)
		expectReadBuf(t, mode, b, 1, "", io.EOF)
		expectPeekAtLeast(t, mode, b, 1, "", io.EOF)
	}
	fn2 := func(mode string, b *decbuf) {
		// Start peeking after reading 1 byte, which fills the buffer
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
	for _, mode := range []readMode{readOneByte, readOneByteEOF} {
		fn1(mode.String(), newDecbuf(mode.testReader(abcReader(3))))
		fn2(mode.String(), newDecbuf(mode.testReader(abcReader(4))))
	}
	// Don't try newDecbufFromBytes for this test, since it requires filling the
	// buffer one byte at a time.
}

func TestDecbufReadByte(t *testing.T) {
	fn := func(mode string, b *decbuf) {
		expectReadByte(t, mode, b, 'a', nil)
		expectReadByte(t, mode, b, 'b', nil)
		expectReadByte(t, mode, b, 'c', nil)
		expectReadByte(t, mode, b, 0, io.EOF)
		expectReadByte(t, mode, b, 0, io.EOF)
	}
	for _, mode := range allReadModes {
		fn(mode.String(), newDecbuf(mode.testReader(abcReader(3))))
	}
	fn("fromBytes", newDecbufFromBytes(abcBytes(3)))
}

func TestDecbufReadByteLimit(t *testing.T) {
	fn := func(mode string, b *decbuf) {
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
	for _, mode := range allReadModes {
		fn(mode.String(), newDecbuf(mode.testReader(abcReader(10))))
	}
	fn("fromBytes", newDecbufFromBytes(abcBytes(10)))
}

func TestDecbufPeekByte(t *testing.T) {
	fn := func(mode string, b *decbuf) {
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
	for _, mode := range allReadModes {
		fn(mode.String(), newDecbuf(mode.testReader(abcReader(3))))
	}
	fn("fromBytes", newDecbufFromBytes(abcBytes(3)))
}

func TestDecbufPeekByteLimit(t *testing.T) {
	fn := func(mode string, b *decbuf) {
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
	for _, mode := range allReadModes {
		fn(mode.String(), newDecbuf(mode.testReader(abcReader(10))))
	}
	fn("fromBytes", newDecbufFromBytes(abcBytes(10)))
}

func TestDecbufReadFull(t *testing.T) {
	fn1 := func(mode string, b *decbuf) {
		// Start ReadFull from beginning.
		expectReadFull(t, mode, b, 3, "abc", nil)
		expectReadFull(t, mode, b, 3, "def", nil)
		expectReadFull(t, mode, b, 1, "", io.EOF)
		expectReadFull(t, mode, b, 1, "", io.EOF)
	}
	fn2 := func(mode string, b *decbuf) {
		// Start ReadFull after reading 1 byte, which fills the buffer.
		expectReadBuf(t, mode, b, 1, "a", nil)
		expectReadFull(t, mode, b, 2, "bc", nil)
		expectReadFull(t, mode, b, 3, "def", nil)
		expectReadFull(t, mode, b, 1, "", io.EOF)
		expectReadFull(t, mode, b, 1, "", io.EOF)
	}
	for _, mode := range allReadModes {
		fn1(mode.String(), newDecbuf(mode.testReader(abcReader(6))))
		fn2(mode.String(), newDecbuf(mode.testReader(abcReader(6))))
	}
	fn1("fromBytes", newDecbufFromBytes(abcBytes(6)))
	fn2("fromBytes", newDecbufFromBytes(abcBytes(6)))
}
