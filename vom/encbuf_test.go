package vom

import (
	"strings"
	"testing"
)

func expectEncbufBytes(t *testing.T, b *encbuf, expect string) {
	if b.Len() != len(expect) {
		t.Errorf("len got %d, want %d", b.Len(), len(expect))
	}
	if string(b.Bytes()) != expect {
		t.Errorf("bytes got %q, want %q", b.Bytes(), expect)
	}
}

func TestEncbuf(t *testing.T) {
	b := newEncbuf()
	expectEncbufBytes(t, b, "")
	copy(b.Grow(5), "abcde")
	expectEncbufBytes(t, b, "abcde")
	b.WriteByte('Z')
	expectEncbufBytes(t, b, "abcdeZ")
	b.Write([]byte("xxx"))
	expectEncbufBytes(t, b, "abcdeZxxx")
	b.WriteString("12345")
	expectEncbufBytes(t, b, "abcdeZxxx12345")
	copy(b.Grow(3), "===")
	expectEncbufBytes(t, b, "abcdeZxxx12345===")
}

func TestBigEncbuf(t *testing.T) {
	const bigsize = 102400 // 100KiB
	b := newEncbuf()
	expectEncbufBytes(t, b, "")
	bigstr := strings.Repeat("a", bigsize)
	b.WriteString(bigstr)
	expectEncbufBytes(t, b, bigstr)
}
