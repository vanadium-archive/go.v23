package wiretype

import (
	"bytes"
	"reflect"
	"strings"
	"testing"

	"veyron2/vom"
)

// stringError makes it easy to convert from json to an error type
type stringError string

func (se stringError) Error() string {
	return string(se)
}

// Tests the JSON to VOM binary transcoder using wiretype-derived types.
func TestJSONToBinaryTranscoderWithWiretypeType(t *testing.T) {
	type tint64 int64
	type tstring string
	type tslice []string
	type tarray [1]int16
	type tmap map[string]int32
	type tstruct struct {
		A int8
		B tslice
	}
	type tstructcopy tstruct
	type tinterface interface{}

	var (
		uint8val uint8  = 6
		int64val int64  = 4
		int64ptr *int64 = &int64val
		nilSlice []int16
		nilMap   map[string]string
	)

	vom.Register(tstruct{})
	vom.Register(tstructcopy{})
	vom.Register([0]string{})
	vom.Register([2]bool{true, false})
	vom.Register([2]int32{})
	// This is what struct{ x int8, Y string } looks like when decoded into an interface:
	vom.Register(struct {
		Y string
		x uint8
	}{})

	// Values to test.
	tests := [][]interface{}{
		{int8(0)},
		{uint16(0)},
		{int32(0)},
		{uint64(0)},
		{uint8(1)},
		{int16(1)},
		{uint32(1)},
		{uint64(1)},
		{float32(0)},
		{float64(3.14159265)},
		{false},
		{true},
		{""},
		{"string"},
		{(interface{})(uint64(4))},
		{[]int64{3, 0, 4}},
		{nilSlice},
		{[]interface{}{float64(4)}},
		{[]interface{}{float64(4), "STR"}},
		{[]interface{}{"A", nil}},
		{[0]string{}},
		{[2]bool{true, false}},
		{map[string]string{"A": "B"}},
		{nilMap},
		{map[string]string{"": ""}},
		{map[string]interface{}{"X": float64(3), "Y": "Z"}},
		{map[uint32]float64{4: 4.5, 6: 6.5, 8: 8.5}},
		{map[[2]int32]int16{[2]int32{3, 7}: 10, [2]int32{9, 2}: 11}},
		{struct{}{}},
		{struct {
			X int64
		}{9}},
		{struct {
			X uint64
			Y string
		}{4, "W"}},
		{struct {
			Y string
			x uint8
		}{"Q", 0}},
		{struct{}{}},
		{struct {
			X *uint8
			Y **int64
			Z string
		}{&uint8val, &int64ptr, "A"}},

		{[]int64(nil)},
		{map[string]int64(nil)},

		// multiple messages
		{int32(1), int32(2), int32(3)},
		{map[string]int64{"A": 4, "B": 3}, []int64{4, 5}},

		// ensure named types in the same stream are emitted as different from unnamed types
		{int64(1), int64(2), int64(3)},
		{tstruct{4, tslice{"A"}}, tstructcopy{5, tslice{"B"}}},
		{[]string{"A"}, tslice{"B"}, []string{"C"}},
	}

	for _, test := range tests {
		var jsonBuf bytes.Buffer
		for _, val := range test {
			if err := vom.ObjToJSON(&jsonBuf, vom.ValueOf(val)); err != nil {
				t.Fatalf("cannot convert object to json")
			}
			jsonBuf.WriteRune(' ')
		}
		jsonMsg := jsonBuf.String()

		var transcodeBuf bytes.Buffer
		transcoder := vom.NewJSONToBinaryTranscoder(&transcodeBuf, strings.NewReader(jsonMsg))

		tc := TypeConverter{}
		for _, val := range test {
			ty := tc.FromVOMType(vom.TypeOf(val))
			if err := transcoder.Transcode(ty); err != nil {
				t.Errorf("error transcoding value for '%v': %v", jsonMsg, err)
			}
		}

		var transDecodeBuf *bytes.Buffer = bytes.NewBuffer(transcodeBuf.Bytes())
		transDec := vom.NewDecoder(transDecodeBuf)

		for _, val := range test {
			var av interface{}
			if err := transDec.Decode(&av); err != nil {
				t.Errorf("error decoding actual input %v for case %v: %v\n", transDecodeBuf.String(), test, err)
				break
			}

			if !reflect.DeepEqual(val, av) {
				t.Errorf("values differed. Expected %v. Got %v.", val, av)
				break
			}
		}
	}
}
