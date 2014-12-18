package golang

import (
	"testing"

	"veyron.io/veyron/veyron2/vdl"
	"veyron.io/veyron/veyron2/vdl/compile"
)

func TestType(t *testing.T) {
	testingMode = true
	tests := []struct {
		T    *vdl.Type
		Want string
	}{
		{vdl.AnyType, `__vdlutil.Any`},
		{vdl.TypeObjectType, `*__vdl.Type`},
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

// TestEnumAll holds all labels for TestEnum.
var TestEnumAll = []TestEnum{TestEnumA, TestEnumB, TestEnumC}

// TestEnumFromString creates a TestEnum from a string label.
func TestEnumFromString(label string) (x TestEnum, err error) {
	err = x.Set(label)
	return
}

// Set assigns label to x.
func (x *TestEnum) Set(label string) error {
	switch label {
	case "A", "a":
		*x = TestEnumA
		return nil
	case "B", "b":
		*x = TestEnumB
		return nil
	case "C", "c":
		*x = TestEnumC
		return nil
	}
	*x = -1
	return __fmt.Errorf("unknown label %q in TestEnum", label)
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

func (TestEnum) __VDLReflect(struct{
	Name string "TestEnum"
	Enum struct{ A, B, C string }
}) {
}`},
		{tStruct, `type TestStruct struct {
	A string
	B int64
}

func (TestStruct) __VDLReflect(struct{
	Name string "TestStruct"
}) {
}`},
		{tUnion, `type (
	// TestUnion represents any single field of the TestUnion union type.
	TestUnion interface {
		// Index returns the field index.
		Index() int
		// Interface returns the field value as an interface.
		Interface() interface{}
		// Name returns the field name.
		Name() string
		// __VDLReflect describes the TestUnion union type.
		__VDLReflect(__TestUnionReflect)
	}
	// TestUnionA represents field A of the TestUnion union type.
	TestUnionA struct{ Value string }
	// TestUnionB represents field B of the TestUnion union type.
	TestUnionB struct{ Value int64 }
	// __TestUnionReflect describes the TestUnion union type.
	__TestUnionReflect struct {
		Name string "TestUnion"
		Type TestUnion
		Union struct {
			A TestUnionA
			B TestUnionB
		}
	}
)

func (x TestUnionA) Index() int { return 0 }
func (x TestUnionA) Interface() interface{} { return x.Value }
func (x TestUnionA) Name() string { return "A" }
func (x TestUnionA) __VDLReflect(__TestUnionReflect) {}

func (x TestUnionB) Index() int { return 1 }
func (x TestUnionB) Interface() interface{} { return x.Value }
func (x TestUnionB) Name() string { return "B" }
func (x TestUnionB) __VDLReflect(__TestUnionReflect) {}`},
	}
	data := goData{Env: compile.NewEnv(-1)}
	for _, test := range tests {
		def := &compile.TypeDef{
			NamePos:  compile.NamePos{Name: test.T.Name()},
			Type:     test.T,
			Exported: compile.ValidExportedIdent(test.T.Name(), compile.ReservedNormal) == nil,
		}
		switch test.T.Kind() {
		case vdl.Enum:
			def.LabelDoc = make([]string, test.T.NumEnumLabel())
			def.LabelDocSuffix = make([]string, test.T.NumEnumLabel())
		case vdl.Struct, vdl.Union:
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
		vdl.Field{"A", vdl.StringType},
		vdl.Field{"B", vdl.Int64Type},
	))
	tUnion = vdl.NamedType("TestUnion", vdl.UnionType(
		vdl.Field{"A", vdl.StringType},
		vdl.Field{"B", vdl.Int64Type},
	))
)
