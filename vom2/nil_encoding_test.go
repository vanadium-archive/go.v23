package vom2_test

import (
	"bytes"
	"reflect"
	"testing"

	"veyron.io/veyron/veyron2/vdl"
	"veyron.io/veyron/veyron2/vom2"
)

// TestNilEncoding tests that go nil and empty composite type values are both encoded as
// corresponding vdl zero values.
func TestNilEncoding(t *testing.T) {
	tests := []struct {
		input          interface{}
		expectedResult *vdl.Value
	}{
		{
			[]int64(nil),
			vdl.ZeroValue(vdl.TypeOf([]int64{})),
		},
		{
			[]int64{},
			vdl.ZeroValue(vdl.TypeOf([]int64{})),
		},
		{
			map[string]int64(nil),
			vdl.ZeroValue(vdl.TypeOf(map[string]int64{})),
		},
		{
			map[string]int64{},
			vdl.ZeroValue(vdl.TypeOf(map[string]int64{})),
		},
	}

	for _, test := range tests {
		var buf bytes.Buffer
		enc, err := vom2.NewBinaryEncoder(&buf)
		if err != nil {
			t.Fatalf("Error creating new encoder: %v", err)
		}
		if err := enc.Encode(test.input); err != nil {
			t.Errorf("Error encoding %v: %v", test.input, err)
			continue
		}

		dec, err := vom2.NewDecoder(bytes.NewBuffer(buf.Bytes()))
		if err != nil {
			t.Fatalf("Error creating new decoder: %v", err)
		}

		var result interface{}
		if err := dec.Decode(&result); err != nil {
			t.Errorf("Error decoding %v", err)
		}

		if !reflect.DeepEqual(test.expectedResult, result) {
			t.Errorf("Expected %v, but got %v", test.expectedResult, result)
		}
	}
}
