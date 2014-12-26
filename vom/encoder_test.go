package vom

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"unsafe"
)

// cleanVOMJSON takes a string containing vom json output and converts it to
// syntactically correct json.  See TestCleanVOMJSON.
func cleanVOMJSON(t *testing.T, m string) string {
	jsonized := "["
	nestDepth := 0
	addComma := false
	for _, c := range strings.TrimSpace(m) {
		if addComma {
			addComma = false
			jsonized += ","
		}
		jsonized += string(c)
		switch c {
		case '[':
			nestDepth++
		case ']':
			nestDepth--
			if nestDepth == 0 {
				addComma = true
			}
		}
	}
	jsonized += "]"
	if nestDepth != 0 {
		t.Errorf("invalid vom json %s: mismatched square brackets", m)
		return ""
	}
	return jsonized
}

func TestCleanVOMJSON(t *testing.T) {
	vomJSON := `["type","v.io/core/veyron2/vom.Slice []uint"]
["type","v.io/core/veyron2/vom.OuterMap map[uint]Slice"]
["OuterMap",{"3":[4,5],"6":[7,8]}]`
	expectedOutput := `[["type","v.io/core/veyron2/vom.Slice []uint"],
["type","v.io/core/veyron2/vom.OuterMap map[uint]Slice"],
["OuterMap",{"3":[4,5],"6":[7,8]}]]`
	if cleanVOMJSON(t, vomJSON) != expectedOutput {
		t.Errorf("clean JSON '%s' did not match expected output '%s'", cleanVOMJSON(t, vomJSON), expectedOutput)
	}
}

// parseJSON takes a string containing vom json output, converts it to
// syntactically correct json, and unmarshals it.
// It's meant to canonicalize vom json output so it can be compared in tests (if
// we were to compare the strings directly, we'd be tripped up by maps whose
// keys may be printed in arbitrary order).
func parseVOMJSON(t *testing.T, m string) interface{} {
	var v interface{}
	js := cleanVOMJSON(t, m)
	if err := json.Unmarshal([]byte(js), &v); err != nil {
		t.Errorf("failed parsing %s: %v", js, err)
		return nil
	}
	return v
}

func testEncoder(t *testing.T, format Format) {
	for _, test := range coderTests {
		var buf bytes.Buffer
		encoder := NewEncoder(&buf).SetFormat(format)
		skipMatch := false
		for _, testVal := range test.Values {
			if err := encoder.Encode(testVal); err != nil {
				t.Errorf("%s: Encode(%#v) failed: %v", test.Name, testVal, err)
				skipMatch = true
			}
		}
		if skipMatch {
			continue // Move on to the next test
		}
		switch format {
		case FormatBinary:
			hex := fmt.Sprintf("%x", buf.String())
			matched, err := matchHexPat(hex, test.HexPat)
			if err != nil {
				t.Error(err)
			}
			if !matched {
				t.Errorf("%s: Encode(%#v)\ngot  %s\nwant %s", test.Name, test.Values, hex, test.HexPat)
			}
		case FormatJSON:
			json := buf.String()
			if want, got := parseVOMJSON(t, test.VomJSON), parseVOMJSON(t, json); !reflect.DeepEqual(got, want) {
				t.Errorf("%s: Encode(%#v) GOT\n%s\nWANT\n%s", test.Name, test.Values, got, want)
			}
		}
	}
}
func DISABLEDTestEncoderBinary(t *testing.T) {
	testEncoder(t, FormatBinary)
}
func DISABLEDTestEncoderJSON(t *testing.T) {
	testEncoder(t, FormatJSON)
}

func testEncodeErrors(t *testing.T, format Format) {
	for _, test := range encodeErrorTests {
		var buf bytes.Buffer
		encoder := NewEncoder(&buf).SetFormat(format)
		for ix, encVal := range test.EncValues {
			eerr := encoder.Encode(encVal)
			if _, err := matchIndexedErrorRE(eerr, test.EncRE, ix); err != nil {
				t.Errorf("%s: Encode(%#v) %v, generated %s", test.Name, encVal, err, fmt.Sprintf("%x", buf.String()))
			}
		}
	}
}
func TestEncodeErrorsBinary(t *testing.T) {
	testEncodeErrors(t, FormatBinary)
}
func TestEncodeErrorsJSON(t *testing.T) {
	testEncodeErrors(t, FormatJSON)
}

type StructNoExports struct {
	a uint
}

type VomEncodeBadInArgs uint

func (v VomEncodeBadInArgs) VomEncode(i int) (string, error) {
	return "", nil
}

type VomEncodeBadOutArgs uint

func (v VomEncodeBadOutArgs) VomEncode() error {
	return nil
}

type VomEncodeNotErrorOutArg uint

func (v VomEncodeNotErrorOutArg) VomEncode() (string, string) {
	return "", ""
}

type VomEncodeA uint

func (v VomEncodeA) VomEncode() (string, error) {
	return "", nil
}

type VomEncodeB uint

func (v VomEncodeB) VomEncode() (VomEncodeA, error) {
	return VomEncodeA(v), nil
}

type VomEncodeCycle uint

func (v VomEncodeCycle) VomEncode() (VomEncodeCycle, error) {
	return v, nil
}

// encodeErrorTests tests encoding errors.
var encodeErrorTests = []struct {
	Name      string
	EncValues v
	EncRE     e
}{
	// Test disallowed types.
	{
		"Nil",
		v{nil},
		e{`vom: can't encode untyped nil value`}},
	{
		"Chan",
		v{make(chan int)},
		e{`vom: unsupported kind "chan" type "chan int"`}},
	{
		"Func",
		v{func() {}},
		e{`vom: unsupported kind "func" type "func\(\)"`}},
	{
		"UnsafePointer",
		v{unsafe.Pointer(nil)},
		e{`vom: unsupported kind "unsafe.Pointer" type "unsafe.Pointer"`}},

	// Test bad structs
	{
		"StructNoExports",
		v{StructNoExports{}},
		e{`vom: struct "vom.StructNoExports" has no exported fields`}},

	// Test encoding errors for VomEncode
	{
		"VomEncodeBadInArgs",
		v{VomEncodeBadInArgs(9)},
		e{`vom: VomEncode must have no in-args`}},
	{
		"VomEncodeBadOutArgs",
		v{VomEncodeBadOutArgs(9)},
		e{`vom: VomEncode must have two out-args`}},
	{
		"VomEncodeNotErrorOutArg",
		v{VomEncodeNotErrorOutArg(9)},
		e{`vom: VomEncode last out-arg must be "error"`}},
	{
		"VomEncodeMulti",
		v{VomEncodeB(9)},
		e{`vom: VomEncode arg can't itself have VomEncode method`}},
	{
		"VomEncodeCycle",
		v{VomEncodeCycle(9)},
		e{`vom: VomEncode arg can't itself have VomEncode method`}},
	{
		"VomEncodeBaseWithPtrReceiver",
		v{UserCoderPtrEncode{9}},
		e{`vom: custom Encode defined with pointer receiver, but base value encoded`}},
}
