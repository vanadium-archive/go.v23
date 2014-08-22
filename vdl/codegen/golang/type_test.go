package golang

import (
	"testing"

	"veyron2/vdl"
	"veyron2/vdl/compile"
)

func TestType(t *testing.T) {
	testingMode = true
	tests := []struct {
		T    *vdl.Type
		Want string
	}{
		{vdl.AnyType, `_gen_vdlutil.Any`},
		{vdl.TypeValType, `*_gen_vdl.Type`},
		{vdl.BoolType, `bool`},
		{vdl.StringType, `string`},
		{vdl.ListType(vdl.ByteType), `[]byte`},
		{vdl.ByteType, `byte`},
		{vdl.Uint16Type, `uint16`},
		{vdl.Uint32Type, `uint32`},
		{vdl.Uint64Type, `uint64`},
		{vdl.Int16Type, `int16`},
		{vdl.Int32Type, `int32`},
		{vdl.Int64Type, `int64`},
		{vdl.Float32Type, `float32`},
		{vdl.Float64Type, `float64`},
		{vdl.Complex64Type, `complex64`},
		{vdl.Complex128Type, `complex128`},
		{tArray, `[3]string`},
		{tList, `[]string`},
		{tSet, `map[string]struct{}`},
		{tMap, `map[string]int64`},
	}
	data := goData{Env: compile.NewEnv(-1)}
	for _, test := range tests {
		if got, want := typeGo(data, test.T), test.Want; got != want {
			t.Errorf("%s\nGOT %s\nWANT %s", test.T, got, want)
		}
	}
}

func TestTypeDef(t *testing.T) {
	testingMode = true
	tests := []struct {
		T    *vdl.Type
		Want string
	}{
		{tEnum, `type TestEnum int
const (
	TestEnumA TestEnum = iota
	TestEnumB
	TestEnumC
)

// AllTestEnum holds all labels for TestEnum.
var AllTestEnum = []TestEnum{TestEnumA, TestEnumB, TestEnumC}

// MakeTestEnum creates a TestEnum from a string label.
// Returns true iff the label is valid.
func MakeTestEnum(label string) (x TestEnum, ok bool) {
	ok = x.Assign(label)
	return
}

// Assign assigns label to x.
// Returns true iff the label is valid.
func (x *TestEnum) Assign(label string) bool {
	switch label {
	case "A":
		*x = TestEnumA
		return true
	case "B":
		*x = TestEnumB
		return true
	case "C":
		*x = TestEnumC
		return true
	}
	*x = -1
	return false
}

// String returns the string label of x.
func (x TestEnum) String() string {
	switch x {
	case TestEnumA:
		return "A"
	case TestEnumB:
		return "B"
	case TestEnumC:
		return "C"
	}
	return ""
}

// vdlEnumLabels identifies TestEnum as an enum.
func (TestEnum) vdlEnumLabels(A, B, C struct{}) {}`},
		{tStruct, `type TestStruct struct {
	A string
	B int64
}`},
		{tOneOf, `type TestOneOf struct{ oneof interface{} }

// MakeTestOneOf creates a TestOneOf.
// Returns true iff the oneof value has a valid type.
func MakeTestOneOf(oneof interface{}) (x TestOneOf, ok bool) {
	ok = x.Assign(oneof)
	return
}

// Assign assigns oneof to x.
// Returns true iff the oneof value has a valid type.
func (x *TestOneOf) Assign(oneof interface{}) bool {
	switch oneof.(type) {
	case string, int64:
		x.oneof = oneof
		return true
	}
	x.oneof = nil
	return false
}

// OneOf returns the underlying typed value of x.
func (x TestOneOf) OneOf() interface{} {
	return x.oneof
}

// vdlOneOfTypes identifies TestOneOf as a oneof.
func (TestOneOf) vdlOneOfTypes(_ string, _ int64) {}`},
	}
	data := goData{Env: compile.NewEnv(-1)}
	for _, test := range tests {
		def := &compile.TypeDef{
			NamePos:  compile.NamePos{Name: test.T.Name()},
			Type:     test.T,
			Exported: compile.ValidExportedIdent(test.T.Name()) == nil,
		}
		switch test.T.Kind() {
		case vdl.Enum:
			def.LabelDoc = make([]string, test.T.NumEnumLabel())
			def.LabelDocSuffix = make([]string, test.T.NumEnumLabel())
		case vdl.Struct:
			def.FieldDoc = make([]string, test.T.NumField())
			def.FieldDocSuffix = make([]string, test.T.NumField())
		}
		if got, want := typeDefGo(data, def), test.Want; got != want {
			t.Errorf("%s\n GOT %s\nWANT %s", test.T, got, want)
		}
	}
}

var (
	tEnum   = vdl.NamedType("TestEnum", vdl.EnumType("A", "B", "C"))
	tArray  = vdl.ArrayType(3, vdl.StringType)
	tList   = vdl.ListType(vdl.StringType)
	tSet    = vdl.SetType(vdl.StringType)
	tMap    = vdl.MapType(vdl.StringType, vdl.Int64Type)
	tStruct = vdl.NamedType("TestStruct", vdl.StructType(
		vdl.StructField{"A", vdl.StringType},
		vdl.StructField{"B", vdl.Int64Type},
	))
	tOneOf = vdl.NamedType("TestOneOf", vdl.OneOfType(vdl.StringType, vdl.Int64Type))
)
