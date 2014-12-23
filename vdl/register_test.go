package vdl

import (
	"reflect"
	"sync"
	"testing"
)

var nUnionWant = &ReflectInfo{
	WireType: reflect.TypeOf((*nUnion)(nil)).Elem(),
	WireName: "veyron.io/veyron/veyron2/vdl.nUnion",
	UnionFields: []ReflectField{
		{"A", reflect.TypeOf(false), reflect.TypeOf(nUnionA{})},
		{"B", reflect.TypeOf(string("")), reflect.TypeOf(nUnionB{})},
		{"C", reflect.TypeOf(int32(0)), reflect.TypeOf(nUnionC{})},
	},
}

var reflectInfoTests = []struct {
	rt reflect.Type
	ri *ReflectInfo
}{
	{reflect.TypeOf(int64(0)), &ReflectInfo{WireType: reflect.TypeOf(int64(0))}},
	{reflect.TypeOf(string("")), &ReflectInfo{WireType: reflect.TypeOf(string(""))}},
	{reflect.TypeOf([]byte{}), &ReflectInfo{WireType: reflect.TypeOf([]byte{})}},
	{
		reflect.TypeOf(nEnumA),
		&ReflectInfo{
			WireType:   reflect.TypeOf(nEnumA),
			WireName:   "veyron.io/veyron/veyron2/vdl.nEnum",
			EnumLabels: []string{"A", "B", "C"},
		},
	},
	{reflect.TypeOf((*nUnion)(nil)).Elem(), nUnionWant},
	{reflect.TypeOf(nUnionA{}), nUnionWant},
	{reflect.TypeOf(nUnionB{}), nUnionWant},
	{reflect.TypeOf(nUnionC{}), nUnionWant},
	{
		reflect.TypeOf(nWire{}),
		&ReflectInfo{
			WireType:   reflect.TypeOf(nWire{}),
			WireName:   "veyron.io/veyron/veyron2/vdl.nWire",
			NativeType: reflect.TypeOf(nNative(0)),
		},
	},
}

func riNorm(ri *ReflectInfo) *ReflectInfo {
	if ri != nil {
		// Clear out the funcs, since they're not guaranteed to be the same.
		ri.ToNativeFunc = reflect.Value{}
		ri.FromNativeFunc = reflect.Value{}
	}
	return ri
}

// Test DeriveReflectInfo success.
func TestDeriveReflectInfo(t *testing.T) {
	for _, test := range reflectInfoTests {
		ri, err := DeriveReflectInfo(test.rt)
		if ri == nil || err != nil {
			t.Errorf("%s DeriveReflectInfo failed: (%v, %v)", test.rt, ri, err)
			continue
		}
		if got, want := riNorm(ri), test.ri; !reflect.DeepEqual(got, want) {
			t.Errorf("%s got %v, want %v", test.rt, got, want)
		}
	}
}

// Test Register called by multiple goroutines concurrently on the same types,
// to expose locking issues in the registry.
func TestRegister(t *testing.T) {
	var done sync.WaitGroup
	for i := 0; i < 3; i++ {
		done.Add(1)
		go func() {
			testRegister(t)
			done.Done()
		}()
	}
	done.Wait()
}

func testRegister(t *testing.T) {
	for _, test := range reflectInfoTests {
		Register(reflect.New(test.rt).Interface())
		ri := ReflectInfoFromWire(test.ri.WireType)
		if got, want := riNorm(ri), test.ri; !reflect.DeepEqual(got, want) {
			t.Errorf("%s ReflectInfoFromWire got %v, want %v", test.rt, got, want)
		}
		if test.ri.WireName != "" {
			ri := ReflectInfoFromName(test.ri.WireName)
			if got, want := riNorm(ri), test.ri; !reflect.DeepEqual(got, want) {
				t.Errorf("%s ReflectInfoFromName got %v, want %v", test.rt, got, want)
			}
		}
		if test.ri.NativeType != nil {
			ri := ReflectInfoFromNative(test.ri.NativeType)
			if got, want := riNorm(ri), test.ri; !reflect.DeepEqual(got, want) {
				t.Errorf("%s ReflectInfoFromNative got %v, want %v", test.rt, got, want)
			}
		}
	}
}

type (
	nBadDescribe1 struct{}
	nBadDescribe2 struct{}
	nBadDescribe3 struct{}

	nBadEnumNoLabels int
	nBadEnumString1  int
	nBadEnumString2  int
	nBadEnumString3  int
	nBadEnumSet1     int
	nBadEnumSet2     int
	nBadEnumSet3     int
	nBadEnumSet4     int

	nBadUnionNoFields struct{}
	nBadUnionUnexp    struct{}
	nBadUnionField1   struct{}
	nBadUnionField2   struct{}
	nBadUnionField3   struct{}
	nBadUnionName1    struct{ Value bool }
	nBadUnionName2    struct{ Value bool }

	nBadWireToNative1 struct{}
	nBadWireToNative2 struct{}
	nBadWireToNative3 struct{}
	nBadWireToNative4 struct{}

	nBadWireFromNative1 struct{}
	nBadWireFromNative2 struct{}
	nBadWireFromNative3 struct{}
	nBadWireFromNative4 struct{}
	nBadWireFromNative5 struct{}

	nBadWireToFromNative1 struct{}
	nBadWireToFromNative2 struct{}
	nBadWireToFromNative3 struct{}
	nBadWireToFromNative4 struct{}
	nBadWireToFromNative5 struct{}
)

// No description
func (nBadDescribe1) __VDLReflect() { panic("X") }

// In-arg isn't a struct
func (nBadDescribe2) __VDLReflect(int) { panic("X") }

// Can't have out-arg
func (nBadDescribe3) __VDLReflect(struct{}) error { panic("X") }

// No enum labels
func (nBadEnumNoLabels) __VDLReflect(struct{ Enum struct{} }) { panic("X") }

// No String method
func (nBadEnumString1) __VDLReflect(struct{ Enum struct{ A string } }) { panic("X") }

// String method isn't String() string
func (nBadEnumString2) __VDLReflect(struct{ Enum struct{ A string } }) { panic("X") }
func (nBadEnumString2) String()                                        { panic("X") }

// String method isn't String() string
func (nBadEnumString3) __VDLReflect(struct{ Enum struct{ A string } }) { panic("X") }
func (nBadEnumString3) String() bool                                   { panic("X") }

// No Set method
func (nBadEnumSet1) __VDLReflect(struct{ Enum struct{ A string } }) { panic("X") }
func (nBadEnumSet1) String() string                                 { panic("X") }

// Set method isn't Set(string) error
func (nBadEnumSet2) __VDLReflect(struct{ Enum struct{ A string } }) { panic("X") }
func (nBadEnumSet2) String() string                                 { panic("X") }
func (nBadEnumSet2) Set()                                           { panic("X") }

// Set method isn't Set(string) error
func (nBadEnumSet3) __VDLReflect(struct{ Enum struct{ A string } }) { panic("X") }
func (nBadEnumSet3) String() string                                 { panic("X") }
func (nBadEnumSet3) Set(bool) error                                 { panic("X") }

// Set method receiver isn't a pointer
func (nBadEnumSet4) __VDLReflect(struct{ Enum struct{ A string } }) { panic("X") }
func (nBadEnumSet4) String() string                                 { panic("X") }
func (nBadEnumSet4) Set(string) error                               { panic("X") }

// No union fields
func (nBadUnionNoFields) __VDLReflect(struct {
	Type  nUnion
	Union struct{}
}) {
	panic("X")
}

// Field name isn't exported
func (nBadUnionUnexp) __VDLReflect(struct {
	Type  nUnion
	Union struct{ a nUnionA }
}) {
	panic("X")
}

// Field type isn't struct
func (nBadUnionField1) __VDLReflect(struct {
	Type  nUnion
	Union struct{ A bool }
}) {
	panic("X")
}

// Field type has no field
func (nBadUnionField2) __VDLReflect(struct {
	Type  nUnion
	Union struct{ A struct{} }
}) {
	panic("X")
}

// Field type name isn't "Value"
func (nBadUnionField3) __VDLReflect(struct {
	Type  nUnion
	Union struct{ A struct{ value bool } }
}) {
	panic("X")
}

// Name method isn't Name() string
func (nBadUnionName1) Name() { panic("X") }
func (nBadUnionName1) __VDLReflect(struct {
	Type  nUnion
	Union struct{ A nBadUnionName1 }
}) {
	panic("X")
}

// Name method isn't Name() string
func (nBadUnionName2) Name() bool { panic("X") }
func (nBadUnionName2) __VDLReflect(struct {
	Type  nUnion
	Union struct{ A nBadUnionName2 }
}) {
	panic("X")
}

// VDLToNative method isn't VDLToNative(*Native) error
func (nBadWireToNative1) VDLToNative() error        { panic("X") }
func (nBadWireToNative2) VDLToNative(nNative) error { panic("X") }
func (nBadWireToNative3) VDLToNative(*nNative)      { panic("X") }
func (nBadWireToNative4) VDLToNative(*nNative) bool { panic("X") }

// VDLFromNative method isn't (*) VDLFromNative(Native) error
func (nBadWireFromNative1) VDLToNative(*nNative) error  { panic("X") }
func (nBadWireFromNative2) VDLToNative(*nNative) error  { panic("X") }
func (nBadWireFromNative3) VDLToNative(*nNative) error  { panic("X") }
func (nBadWireFromNative4) VDLToNative(*nNative) error  { panic("X") }
func (nBadWireFromNative5) VDLToNative(*nNative) error  { panic("X") }
func (nBadWireFromNative1) VDLFromNative() error        { panic("X") }
func (nBadWireFromNative2) VDLFromNative(nNative) error { panic("X") }
func (*nBadWireFromNative3) VDLFromNative()             { panic("X") }
func (*nBadWireFromNative4) VDLFromNative(nNative)      { panic("X") }
func (*nBadWireFromNative5) VDLFromNative(nNative) bool { panic("X") }

// VDL{To,From}Native are mismatched.
func (nBadWireToFromNative1) VDLToNative(*nNative) error    { panic("X") }
func (*nBadWireToFromNative2) VDLFromNative(nNative) error  { panic("X") }
func (nBadWireToFromNative3) VDLToNative(*nNative) error    { panic("X") }
func (*nBadWireToFromNative3) VDLFromNative(int64) error    { panic("X") }
func (nBadWireToFromNative4) VDLToNative(*int64) error      { panic("X") }
func (*nBadWireToFromNative4) VDLFromNative(nNative) error  { panic("X") }
func (nBadWireToFromNative5) VDLToNative(*nNative) error    { panic("X") }
func (*nBadWireToFromNative5) VDLFromNative(*nNative) error { panic("X") }

// rtErrorTest describes a test case with rt as input, and errstr as output.
type rtErrorTest struct {
	rt     reflect.Type
	errstr string
}

const (
	badDescribe     = `invalid __VDLReflect (want __VDLReflect(struct{...}))`
	badEnumString   = `must have method String() string`
	badEnumSet      = `must have pointer method Set(string) error`
	badUnionField   = `bad concrete field type`
	badUnionName    = `must have method Name() string`
	badToNative     = `want method VDLToNative(*Native) error`
	badFromNative   = `want pointer method VDLFromNative(Native) error`
	badToFromNative = `must specify both VDLToNative and VDLFromNative with the same Native type`
)

var reflectInfoErrorTests = []rtErrorTest{
	{reflect.TypeOf(nBadDescribe1{}), badDescribe},
	{reflect.TypeOf(nBadDescribe2{}), badDescribe},
	{reflect.TypeOf(nBadDescribe3{}), badDescribe},
	{reflect.TypeOf(nBadEnumNoLabels(0)), `no labels`},
	{reflect.TypeOf(nBadEnumString1(0)), badEnumString},
	{reflect.TypeOf(nBadEnumString2(0)), badEnumString},
	{reflect.TypeOf(nBadEnumString3(0)), badEnumString},
	{reflect.TypeOf(nBadEnumSet1(0)), badEnumSet},
	{reflect.TypeOf(nBadEnumSet2(0)), badEnumSet},
	{reflect.TypeOf(nBadEnumSet3(0)), badEnumSet},
	{reflect.TypeOf(nBadEnumSet4(0)), badEnumSet},
	{reflect.TypeOf(nBadUnionNoFields{}), `no fields`},
	{reflect.TypeOf(nBadUnionUnexp{}), `must be exported`},
	{reflect.TypeOf(nBadUnionField1{}), badUnionField},
	{reflect.TypeOf(nBadUnionField2{}), badUnionField},
	{reflect.TypeOf(nBadUnionField3{}), badUnionField},
	{reflect.TypeOf(nBadUnionName1{}), badUnionName},
	{reflect.TypeOf(nBadUnionName2{}), badUnionName},
	{reflect.TypeOf(nBadWireToNative1{}), badToNative},
	{reflect.TypeOf(nBadWireToNative2{}), badToNative},
	{reflect.TypeOf(nBadWireToNative3{}), badToNative},
	{reflect.TypeOf(nBadWireToNative4{}), badToNative},
	{reflect.TypeOf(nBadWireFromNative1{}), badFromNative},
	{reflect.TypeOf(nBadWireFromNative2{}), badFromNative},
	{reflect.TypeOf(nBadWireFromNative3{}), badFromNative},
	{reflect.TypeOf(nBadWireFromNative4{}), badFromNative},
	{reflect.TypeOf(nBadWireFromNative5{}), badFromNative},
	{reflect.TypeOf(nBadWireToFromNative1{}), badToFromNative},
	{reflect.TypeOf(nBadWireToFromNative2{}), badToFromNative},
	{reflect.TypeOf(nBadWireToFromNative3{}), badToFromNative},
	{reflect.TypeOf(nBadWireToFromNative4{}), badToFromNative},
	{reflect.TypeOf(nBadWireToFromNative5{}), badToFromNative},
}

// Test DeriveReflectInfo errors.
func TestDeriveReflectInfoError(t *testing.T) {
	for _, test := range reflectInfoErrorTests {
		got, err := DeriveReflectInfo(test.rt)
		expectErr(t, err, test.errstr, "DeriveReflectInfo(%v)", test.rt)
		if got != nil {
			t.Errorf("DeriveReflectInfo(%v) got %v, want nil", test.rt, got)
		}
	}
}
