package vom

import (
	"fmt"
	"io"
	"math"
	"reflect"
	"strings"
	"testing"
)

func TestRawEncodeDecode(t *testing.T) {
	tests := []struct {
		v   interface{}
		hex string
	}{
		{false, "00"},
		{true, "01"},

		{uint64(0), "00"},
		{uint64(1), "01"},
		{uint64(2), "02"},
		{uint64(127), "7f"},
		{uint64(128), "ff80"},
		{uint64(255), "ffff"},
		{uint64(256), "fe0100"},
		{uint64(257), "fe0101"},
		{uint64(0xffff), "feffff"},
		{uint64(0xffffff), "fdffffff"},
		{uint64(0xffffffff), "fcffffffff"},
		{uint64(0xffffffffff), "fbffffffffff"},
		{uint64(0xffffffffffff), "faffffffffffff"},
		{uint64(0xffffffffffffff), "f9ffffffffffffff"},
		{uint64(0xffffffffffffffff), "f8ffffffffffffffff"},

		{int64(0), "00"},
		{int64(1), "02"},
		{int64(2), "04"},
		{int64(63), "7e"},
		{int64(64), "ff80"},
		{int64(65), "ff82"},
		{int64(127), "fffe"},
		{int64(128), "fe0100"},
		{int64(129), "fe0102"},
		{int64(math.MaxInt16), "fefffe"},
		{int64(math.MaxInt32), "fcfffffffe"},
		{int64(math.MaxInt64), "f8fffffffffffffffe"},

		{int64(-1), "01"},
		{int64(-2), "03"},
		{int64(-64), "7f"},
		{int64(-65), "ff81"},
		{int64(-66), "ff83"},
		{int64(-128), "ffff"},
		{int64(-129), "fe0101"},
		{int64(-130), "fe0103"},
		{int64(math.MinInt16), "feffff"},
		{int64(math.MinInt32), "fcffffffff"},
		{int64(math.MinInt64), "f8ffffffffffffffff"},

		{float64(0), "00"},
		{float64(1), "fef03f"},
		{float64(17), "fe3140"},
		{float64(18), "fe3240"},

		{"", "00"},
		{"abc", "03616263"},
		{"defghi", "06646566676869"},
		{[]byte{}, "00"},
		{[]byte("abc"), "03616263"},
		{[]byte("defghi"), "06646566676869"},
	}
	for _, test := range tests {
		// Test encode
		encbuf := newEncbuf()
		var buf []byte
		switch val := test.v.(type) {
		case uint64:
			rawEncodeUint(encbuf, val)
			buf = make([]byte, maxEncodedUintBytes)
			buf = buf[rawEncodeUintEnd(buf, val):]
		case int64:
			rawEncodeInt(encbuf, val)
			buf = make([]byte, maxEncodedUintBytes)
			buf = buf[rawEncodeIntEnd(buf, val):]
		case float64:
			rawEncodeFloat(encbuf, val)
		case bool:
			rawEncodeBool(encbuf, val)
		case []byte:
			rawEncodeByteSlice(encbuf, val)
		case string:
			rawEncodeString(encbuf, val)
		}
		hex := fmt.Sprintf("%x", encbuf.Bytes())
		if hex != test.hex {
			t.Errorf("raw encode %T(%v): ACTUAL 0x%v EXPECT 0x%v", test.v, test.v, hex, test.hex)
		}
		if buf != nil {
			hex = fmt.Sprintf("%x", buf)
			if hex != test.hex {
				t.Errorf("raw encode end %T(%v): ACTUAL 0x%v EXPECT 0x%v", test.v, test.v, hex, test.hex)
			}
		}
		// Test decode
		var bin string
		if _, err := fmt.Sscanf(test.hex, "%x", &bin); err != nil {
			t.Errorf("couldn't scan 0x%v as hex: %v", test.hex, err)
			continue
		}
		reader := newDecbuf(strings.NewReader(bin), uint64Size)
		reader2 := newDecbuf(strings.NewReader(bin), uint64Size)
		var v interface{}
		var err, err2 error
		switch test.v.(type) {
		case uint64:
			v, err = rawDecodeUint(reader)
			err2 = rawIgnoreUint(reader2)
		case int64:
			v, err = rawDecodeInt(reader)
			err2 = rawIgnoreUint(reader2)
		case float64:
			v, err = rawDecodeFloat(reader)
			err2 = rawIgnoreUint(reader2)
		case bool:
			v, err = rawDecodeBool(reader)
			err2 = rawIgnoreUint(reader2)
		case []byte, string:
			err = rawIgnoreByteSlice(reader)
			err2 = rawIgnoreByteSlice(reader2)
			v = ignored(true)
		}
		if err != nil || err2 != nil {
			t.Errorf("raw decode %T(0x%v): %v %v", test.v, test.hex, err, err2)
			continue
		}
		if b, err := reader.ReadByte(); err != io.EOF {
			t.Errorf("raw decode %T(0x%v) leftover byte r=%x", test.v, test.hex, b)
			continue
		}
		if b, err := reader2.ReadByte(); err != io.EOF {
			t.Errorf("raw decode %T(0x%v) leftover byte r2=%x", test.v, test.hex, b)
			continue
		}
		if _, ok := v.(ignored); ok {
			continue
		}
		if !reflect.DeepEqual(v, test.v) {
			t.Errorf("raw decode %T(0x%v): ACTUAL %v EXPECT %v", test.v, test.hex, v, test.v)
		}
	}
}

type ignored bool
