package valconv

// TODO(toddw): test values of recursive types.

import (
	"fmt"
	"math"
	"reflect"
	"testing"

	"v.io/core/veyron2/vdl"
	"v.io/core/veyron2/verror"
	"v.io/core/veyron2/verror2"
)

type nValue *vdl.Value

// Each group of values in vvNAME and rvNAME are all mutually convertible.
var (
	rvError1 = verror2.Standard{
		IDAction:  verror2.IDAction{verror.ID("id1"), verror2.NoRetry},
		Msg:       "msg1",
		ParamList: nil,
	}
	rvError2 = verror2.Standard{
		IDAction:  verror2.IDAction{verror.ID("id2"), verror2.RetryConnection},
		Msg:       "msg2",
		ParamList: []interface{}{"abc", int32(123)},
	}
	rvError3 = verror2.Standard{
		IDAction:  verror2.IDAction{verror.ID("id3"), verror2.RetryBackoff},
		Msg:       "msg3",
		ParamList: []interface{}{rvError1, &rvError2},
	}
	vvError1 = errorValue(rvError1)
	vvError2 = errorValue(rvError2)
	vvError3 = errorValue(rvError3)

	vvBoolTrue = []*vdl.Value{
		boolValue(vdl.BoolType, true), boolValue(boolTypeN, true),
	}
	vvStrABC = []*vdl.Value{
		stringValue(vdl.StringType, "ABC"), stringValue(stringTypeN, "ABC"),
		bytesValue(bytesType, "ABC"), bytesValue(bytesTypeN, "ABC"),
		bytes3Value(bytes3Type, "ABC"), bytes3Value(bytes3TypeN, "ABC"),
		vdl.ZeroValue(enumTypeN).AssignEnumLabel("ABC"),
	}
	vvTypeObjectBool = []*vdl.Value{
		vdl.TypeObjectValue(vdl.BoolType),
	}
	vvSeq123 = []*vdl.Value{
		seqNumValue(array3Uint64Type, 1, 2, 3),
		seqNumValue(array3Uint64TypeN, 1, 2, 3),
		seqNumValue(array3Int64Type, 1, 2, 3),
		seqNumValue(array3Int64TypeN, 1, 2, 3),
		seqNumValue(array3Float64Type, 1, 2, 3),
		seqNumValue(array3Float64TypeN, 1, 2, 3),
		seqNumValue(array3Complex64Type, 1, 2, 3),
		seqNumValue(array3Complex64TypeN, 1, 2, 3),
		seqNumValue(listUint64Type, 1, 2, 3),
		seqNumValue(listUint64TypeN, 1, 2, 3),
		seqNumValue(listInt64Type, 1, 2, 3),
		seqNumValue(listInt64TypeN, 1, 2, 3),
		seqNumValue(listFloat64Type, 1, 2, 3),
		seqNumValue(listFloat64TypeN, 1, 2, 3),
		seqNumValue(listComplex64Type, 1, 2, 3),
		seqNumValue(listComplex64TypeN, 1, 2, 3),
	}
	vvSet123 = []*vdl.Value{
		setNumValue(setUint64Type, 1, 2, 3),
		setNumValue(setUint64TypeN, 1, 2, 3),
		setNumValue(setInt64Type, 1, 2, 3),
		setNumValue(setInt64TypeN, 1, 2, 3),
		setNumValue(setFloat64Type, 1, 2, 3),
		setNumValue(setFloat64TypeN, 1, 2, 3),
		setNumValue(setComplex64Type, 1, 2, 3),
		setNumValue(setComplex64TypeN, 1, 2, 3),
	}
	vvMap123True = []*vdl.Value{
		mapNumBoolValue(mapUint64BoolType, nb{1, true}, nb{2, true}, nb{3, true}),
		mapNumBoolValue(mapUint64BoolTypeN, nb{1, true}, nb{2, true}, nb{3, true}),
		mapNumBoolValue(mapInt64BoolType, nb{1, true}, nb{2, true}, nb{3, true}),
		mapNumBoolValue(mapInt64BoolTypeN, nb{1, true}, nb{2, true}, nb{3, true}),
		mapNumBoolValue(mapFloat64BoolType, nb{1, true}, nb{2, true}, nb{3, true}),
		mapNumBoolValue(mapFloat64BoolTypeN, nb{1, true}, nb{2, true}, nb{3, true}),
		mapNumBoolValue(mapComplex64BoolType, nb{1, true}, nb{2, true}, nb{3, true}),
		mapNumBoolValue(mapComplex64BoolTypeN, nb{1, true}, nb{2, true}, nb{3, true}),
	}
	vvSetMap123       = append(vvSet123, vvMap123True...)
	vvMap123FalseTrue = []*vdl.Value{
		mapNumBoolValue(mapUint64BoolType, nb{1, false}, nb{2, true}, nb{3, false}),
		mapNumBoolValue(mapUint64BoolTypeN, nb{1, false}, nb{2, true}, nb{3, false}),
		mapNumBoolValue(mapInt64BoolType, nb{1, false}, nb{2, true}, nb{3, false}),
		mapNumBoolValue(mapInt64BoolTypeN, nb{1, false}, nb{2, true}, nb{3, false}),
		mapNumBoolValue(mapFloat64BoolType, nb{1, false}, nb{2, true}, nb{3, false}),
		mapNumBoolValue(mapFloat64BoolTypeN, nb{1, false}, nb{2, true}, nb{3, false}),
		mapNumBoolValue(mapComplex64BoolType, nb{1, false}, nb{2, true}, nb{3, false}),
		mapNumBoolValue(mapComplex64BoolTypeN, nb{1, false}, nb{2, true}, nb{3, false}),
	}
	vvSetXYZ = []*vdl.Value{
		setStringValue(setStringType, "X", "Y", "Z"),
		setStringValue(setStringTypeN, "X", "Y", "Z"),
	}
	vvMapXYZTrue = []*vdl.Value{
		mapStringBoolValue(mapStringBoolType, sb{"X", true}, sb{"Y", true}, sb{"Z", true}),
		mapStringBoolValue(mapStringBoolTypeN, sb{"X", true}, sb{"Y", true}, sb{"Z", true}),
	}
	vvStructXYZTrue = []*vdl.Value{
		structBoolValue(structXYZBoolType, sb{"X", true}, sb{"Y", true}, sb{"Z", true}),
		structBoolValue(structXYZBoolTypeN, sb{"X", true}, sb{"Y", true}, sb{"Z", true}),
	}
	vvSetMapStructXYZ       = append(append(vvSetXYZ, vvMapXYZTrue...), vvStructXYZTrue...)
	vvMapStructXYZFalseTrue = []*vdl.Value{
		mapStringBoolValue(mapStringBoolType, sb{"X", false}, sb{"Y", true}, sb{"Z", false}),
		mapStringBoolValue(mapStringBoolTypeN, sb{"X", false}, sb{"Y", true}, sb{"Z", false}),
		structBoolValue(structXYZBoolType, sb{"X", false}, sb{"Y", true}, sb{"Z", false}),
		structBoolValue(structXYZBoolTypeN, sb{"X", false}, sb{"Y", true}, sb{"Z", false}),
	}
	vvMapStructXYZEmpty = []*vdl.Value{
		mapStringEmptyValue(mapStringEmptyType, "X", "Y", "Z"),
		mapStringEmptyValue(mapStringEmptyTypeN, "X", "Y", "Z"),
		vdl.ZeroValue(structXYZEmptyType), vdl.ZeroValue(structXYZEmptyTypeN),
	}
	vvStructWXFalseTrue = []*vdl.Value{
		structBoolValue(structWXBoolType, sb{"W", false}, sb{"X", true}),
		structBoolValue(structWXBoolTypeN, sb{"W", false}, sb{"X", true}),
	}
	vvMapVWX123 = []*vdl.Value{
		mapStringNumValue(mapStringUint64Type, sn{"V", 1}, sn{"W", 2}, sn{"X", 3}),
		mapStringNumValue(mapStringUint64TypeN, sn{"V", 1}, sn{"W", 2}, sn{"X", 3}),
		mapStringNumValue(mapStringInt64Type, sn{"V", 1}, sn{"W", 2}, sn{"X", 3}),
		mapStringNumValue(mapStringInt64TypeN, sn{"V", 1}, sn{"W", 2}, sn{"X", 3}),
		mapStringNumValue(mapStringFloat64Type, sn{"V", 1}, sn{"W", 2}, sn{"X", 3}),
		mapStringNumValue(mapStringFloat64TypeN, sn{"V", 1}, sn{"W", 2}, sn{"X", 3}),
		mapStringNumValue(mapStringComplex64Type, sn{"V", 1}, sn{"W", 2}, sn{"X", 3}),
		mapStringNumValue(mapStringComplex64TypeN, sn{"V", 1}, sn{"W", 2}, sn{"X", 3}),
	}
	vvStructVWX123 = []*vdl.Value{
		structNumValue(structVWXUint64Type, sn{"V", 1}, sn{"W", 2}, sn{"X", 3}),
		structNumValue(structVWXUint64TypeN, sn{"V", 1}, sn{"W", 2}, sn{"X", 3}),
		structNumValue(structVWXInt64Type, sn{"V", 1}, sn{"W", 2}, sn{"X", 3}),
		structNumValue(structVWXInt64TypeN, sn{"V", 1}, sn{"W", 2}, sn{"X", 3}),
		structNumValue(structVWXFloat64Type, sn{"V", 1}, sn{"W", 2}, sn{"X", 3}),
		structNumValue(structVWXFloat64TypeN, sn{"V", 1}, sn{"W", 2}, sn{"X", 3}),
		structNumValue(structVWXComplex64Type, sn{"V", 1}, sn{"W", 2}, sn{"X", 3}),
		structNumValue(structVWXComplex64TypeN, sn{"V", 1}, sn{"W", 2}, sn{"X", 3}),
	}
	vvMapStructVWX123 = append(vvMapVWX123, vvStructVWX123...)
	vvStructUV01      = []*vdl.Value{
		structNumValue(structUVUint64Type, sn{"U", 0}, sn{"V", 1}),
		structNumValue(structUVUint64TypeN, sn{"U", 0}, sn{"V", 1}),
		structNumValue(structUVInt64Type, sn{"U", 0}, sn{"V", 1}),
		structNumValue(structUVInt64TypeN, sn{"U", 0}, sn{"V", 1}),
		structNumValue(structUVFloat64Type, sn{"U", 0}, sn{"V", 1}),
		structNumValue(structUVFloat64TypeN, sn{"U", 0}, sn{"V", 1}),
		structNumValue(structUVComplex64Type, sn{"U", 0}, sn{"V", 1}),
		structNumValue(structUVComplex64TypeN, sn{"U", 0}, sn{"V", 1}),
	}
	vvEmptyStruct = []*vdl.Value{vdl.ZeroValue(emptyType), vdl.ZeroValue(emptyTypeN)}

	rvBoolTrue = []interface{}{
		bool(true), nBool(true),
	}
	rvStrABC = []interface{}{
		string("ABC"), []byte("ABC"), [3]byte{'A', 'B', 'C'},
		nString("ABC"), nSliceUint8("ABC"), nArray3Uint8{'A', 'B', 'C'},
		nEnumABC,
	}
	rvTypeObjectBool = []interface{}{
		vdl.BoolType, nType(vdl.BoolType),
	}
	rvSeq123 = []interface{}{
		[3]uint64{1, 2, 3}, []uint64{1, 2, 3}, nArray3Uint64{1, 2, 3}, nSliceUint64{1, 2, 3},
		[3]int64{1, 2, 3}, []int64{1, 2, 3}, nArray3Int64{1, 2, 3}, nSliceInt64{1, 2, 3},
		[3]float64{1, 2, 3}, []float64{1, 2, 3}, nArray3Float64{1, 2, 3}, nSliceFloat64{1, 2, 3},
		[3]complex64{1, 2, 3}, []complex64{1, 2, 3}, nArray3Complex64{1, 2, 3}, nSliceComplex64{1, 2, 3},
	}
	rvSet123 = []interface{}{
		map[uint64]struct{}{1: struct{}{}, 2: struct{}{}, 3: struct{}{}},
		map[int64]struct{}{1: struct{}{}, 2: struct{}{}, 3: struct{}{}},
		map[float64]struct{}{1: struct{}{}, 2: struct{}{}, 3: struct{}{}},
		map[complex64]struct{}{1: struct{}{}, 2: struct{}{}, 3: struct{}{}},
		nMapUint64Empty{1: struct{}{}, 2: struct{}{}, 3: struct{}{}},
		nMapInt64Empty{1: struct{}{}, 2: struct{}{}, 3: struct{}{}},
		nMapFloat64Empty{1: struct{}{}, 2: struct{}{}, 3: struct{}{}},
		nMapComplex64Empty{1: struct{}{}, 2: struct{}{}, 3: struct{}{}},
	}
	rvMap123True = []interface{}{
		map[uint64]bool{1: true, 2: true, 3: true},
		map[int64]bool{1: true, 2: true, 3: true},
		map[float64]bool{1: true, 2: true, 3: true},
		map[complex64]bool{1: true, 2: true, 3: true},
		nMapUint64Bool{1: true, 2: true, 3: true},
		nMapInt64Bool{1: true, 2: true, 3: true},
		nMapFloat64Bool{1: true, 2: true, 3: true},
		nMapComplex64Bool{1: true, 2: true, 3: true},
	}
	rvSetMap123       = append(rvSet123, rvMap123True...)
	rvMap123FalseTrue = []interface{}{
		map[uint64]bool{1: false, 2: true, 3: false},
		map[int64]bool{1: false, 2: true, 3: false},
		map[float64]bool{1: false, 2: true, 3: false},
		map[complex64]bool{1: false, 2: true, 3: false},
		nMapUint64Bool{1: false, 2: true, 3: false},
		nMapInt64Bool{1: false, 2: true, 3: false},
		nMapFloat64Bool{1: false, 2: true, 3: false},
		nMapComplex64Bool{1: false, 2: true, 3: false},
	}
	rvSetXYZ = []interface{}{
		map[string]struct{}{"X": struct{}{}, "Y": struct{}{}, "Z": struct{}{}},
		nMapStringEmpty{"X": struct{}{}, "Y": struct{}{}, "Z": struct{}{}},
	}
	rvMapXYZTrue = []interface{}{
		map[string]bool{"X": true, "Y": true, "Z": true},
		nMapStringBool{"X": true, "Y": true, "Z": true},
	}
	rvStructXYZTrue = []interface{}{
		struct{ X, Y, Z bool }{X: true, Y: true, Z: true},
		struct{ a, X, b, Y, c, Z, d bool }{X: true, Y: true, Z: true},
		nStructXYZBool{X: true, Y: true, Z: true},
		nStructXYZBoolUnexported{X: true, Y: true, Z: true},
	}
	rvSetMapStructXYZ       = append(append(rvSetXYZ, rvMapXYZTrue...), rvStructXYZTrue...)
	rvMapStructXYZFalseTrue = []interface{}{
		map[string]bool{"X": false, "Y": true, "Z": false},
		nMapStringBool{"X": false, "Y": true, "Z": false},
		struct{ X, Y, Z bool }{X: false, Y: true, Z: false},
		struct{ a, X, b, Y, c, Z, d bool }{X: false, Y: true, Z: false},
		nStructXYZBool{X: false, Y: true, Z: false},
		nStructXYZBoolUnexported{X: false, Y: true, Z: false},
	}
	rvMapStructXYZEmpty = []interface{}{
		map[string]nEmpty{"X": nEmpty{}, "Y": nEmpty{}, "Z": nEmpty{}},
		nMapStringnEmpty{"X": nEmpty{}, "Y": nEmpty{}, "Z": nEmpty{}},
		nStructXYZEmpty{}, nStructXYZnEmpty{},
	}
	rvStructWXFalseTrue = []interface{}{
		struct{ W, X bool }{W: false, X: true},
		nStructWXBool{W: false, X: true},
	}
	rvMapVWX123 = []interface{}{
		map[string]uint64{"V": 1, "W": 2, "X": 3},
		map[string]int64{"V": 1, "W": 2, "X": 3},
		map[string]float64{"V": 1, "W": 2, "X": 3},
		map[string]complex64{"V": 1, "W": 2, "X": 3},
		nMapStringUint64{"V": 1, "W": 2, "X": 3},
		nMapStringInt64{"V": 1, "W": 2, "X": 3},
		nMapStringFloat64{"V": 1, "W": 2, "X": 3},
		nMapStringComplex64{"V": 1, "W": 2, "X": 3},
	}
	rvStructVWX123 = []interface{}{
		struct{ V, W, X uint64 }{V: 1, W: 2, X: 3},
		struct{ V, W, X int64 }{V: 1, W: 2, X: 3},
		struct{ V, W, X float64 }{V: 1, W: 2, X: 3},
		struct{ V, W, X complex64 }{V: 1, W: 2, X: 3},
		struct {
			// Interleave unexported fields, which are ignored.
			a bool
			V int64
			b string
			W float64
			c []byte
			X complex64
			d interface{}
		}{V: 1, W: 2, X: 3},
		nStructVWXUint64{V: 1, W: 2, X: 3},
		nStructVWXInt64{V: 1, W: 2, X: 3},
		nStructVWXFloat64{V: 1, W: 2, X: 3},
		nStructVWXComplex64{V: 1, W: 2, X: 3},
		nStructVWXMixed{V: 1, W: 2, X: 3},
	}
	rvMapStructVWX123 = append(rvMapVWX123, rvStructVWX123...)
	rvStructUV01      = []interface{}{
		struct{ U, V uint64 }{U: 0, V: 1},
		struct{ U, V int64 }{U: 0, V: 1},
		struct{ U, V float64 }{U: 0, V: 1},
		struct{ U, V complex64 }{U: 0, V: 1},
		struct {
			// Interleave unexported fields, which are ignored.
			a bool
			U int64
			b string
			V float64
			c []byte
		}{U: 0, V: 1},
		nStructUVUint64{U: 0, V: 1},
		nStructUVInt64{U: 0, V: 1},
		nStructUVFloat64{U: 0, V: 1},
		nStructUVComplex64{U: 0, V: 1},
		nStructUVMixed{U: 0, V: 1},
	}
	rvEmptyStruct = []interface{}{struct{}{}, nEmpty{}}
	rvNative0     = nNative(0)
	rvNative1     = nNative(1)
	rvNativeMin   = nNative(-(1 << 63))
	rvNativeMax   = nNative((1 << 63) - 1)

	ttBools           = ttTypes(vvBoolTrue)
	ttStrs            = ttTypes(vvStrABC)
	ttTypeObjects     = ttTypes(vvTypeObjectBool)
	ttSeq123          = ttTypes(vvSeq123)
	ttSet123          = ttTypes(vvSet123)
	ttSetMap123       = ttTypes(vvSetMap123)
	ttSetXYZ          = ttTypes(vvSetXYZ)
	ttMapXYZBool      = ttTypes(vvMapXYZTrue)
	ttStructXYZBool   = ttTypes(vvStructXYZTrue)
	ttSetMapStructXYZ = ttTypes(vvSetMapStructXYZ)
	ttStructWXBool    = ttTypes(vvStructWXFalseTrue)
	ttMapVWXNum       = ttTypes(vvMapVWX123)
	ttStructVWXNum    = ttTypes(vvStructVWX123)
	ttMapStructVWXNum = ttTypes(vvMapStructVWX123)
	ttStructUVNum     = ttTypes(vvStructUV01)
	ttUints           = []*vdl.Type{
		vdl.ByteType, nByteType,
		vdl.Uint16Type, uint16TypeN,
		vdl.Uint32Type, uint32TypeN,
		vdl.Uint64Type, uint64TypeN,
	}
	ttInts = []*vdl.Type{
		vdl.Int16Type, int16TypeN,
		vdl.Int32Type, int32TypeN,
		vdl.Int64Type, int64TypeN,
	}
	ttFloat32s    = []*vdl.Type{vdl.Float32Type, float32TypeN}
	ttFloat64s    = []*vdl.Type{vdl.Float64Type, float64TypeN}
	ttFloats      = ttJoin(ttFloat32s, ttFloat64s)
	ttComplex64s  = []*vdl.Type{vdl.Complex64Type, complex64TypeN}
	ttComplex128s = []*vdl.Type{vdl.Complex128Type, complex128TypeN}
	ttComplexes   = ttJoin(ttComplex64s, ttComplex128s)
	ttIntegers    = ttJoin(ttUints, ttInts)
	ttNumbers     = ttJoin(ttIntegers, ttFloats, ttComplexes)
	ttAllTypes    = ttJoin(ttBools, ttStrs, ttTypeObjects, ttNumbers, ttSeq123, ttSetMap123, ttSetMapStructXYZ, ttMapStructVWXNum)

	rtBools           = rtTypes(rvBoolTrue)
	rtStrs            = rtTypes(rvStrABC)
	rtTypeObjects     = rtTypes(rvTypeObjectBool)
	rtSeq123          = rtTypes(rvSeq123)
	rtSet123          = rtTypes(rvSet123)
	rtSetMap123       = rtTypes(rvSetMap123)
	rtSetXYZ          = rtTypes(rvSetXYZ)
	rtMapXYZBool      = rtTypes(rvMapXYZTrue)
	rtStructXYZBool   = rtTypes(rvStructXYZTrue)
	rtSetMapStructXYZ = rtTypes(rvSetMapStructXYZ)
	rtStructWXBool    = rtTypes(rvStructWXFalseTrue)
	rtMapVWXNum       = rtTypes(rvMapVWX123)
	rtStructVWXNum    = rtTypes(rvStructVWX123)
	rtMapStructVWXNum = rtTypes(rvMapStructVWX123)
	rtStructUVNum     = rtTypes(rvStructUV01)
	rtUints           = []reflect.Type{
		reflect.TypeOf(uint8(0)), reflect.TypeOf(nUint8(0)),
		reflect.TypeOf(uint16(0)), reflect.TypeOf(nUint16(0)),
		reflect.TypeOf(uint32(0)), reflect.TypeOf(nUint32(0)),
		reflect.TypeOf(uint64(0)), reflect.TypeOf(nUint64(0)),
	}
	rtInts = []reflect.Type{
		reflect.TypeOf(int8(0)), reflect.TypeOf(nInt8(0)),
		reflect.TypeOf(int16(0)), reflect.TypeOf(nInt16(0)),
		reflect.TypeOf(int32(0)), reflect.TypeOf(nInt32(0)),
		reflect.TypeOf(int64(0)), reflect.TypeOf(nInt64(0)),
	}
	rtFloat32s = []reflect.Type{
		reflect.TypeOf(float32(0)), reflect.TypeOf(nFloat32(0)),
	}
	rtFloat64s = []reflect.Type{
		reflect.TypeOf(float64(0)), reflect.TypeOf(nFloat64(0)),
	}
	rtFloats     = rtJoin(rtFloat32s, rtFloat64s)
	rtComplex64s = []reflect.Type{
		reflect.TypeOf(complex64(0)), reflect.TypeOf(nComplex64(0)),
	}
	rtComplex128s = []reflect.Type{
		reflect.TypeOf(complex128(0)), reflect.TypeOf(nComplex128(0)),
	}
	rtComplexes = rtJoin(rtComplex64s, rtComplex128s)
	rtIntegers  = rtJoin(rtUints, rtInts)
	rtNumbers   = rtJoin(rtIntegers, rtFloats, rtComplexes)
	rtAllTypes  = rtJoin(rtBools, rtStrs, rtTypeObjects, rtNumbers, rtSeq123, rtSetMap123, rtSetMapStructXYZ, rtMapStructVWXNum)

	rtInterface = reflect.TypeOf((*interface{})(nil)).Elem()
)

// Helpers to manipulate slices of *Type
func ttSetToSlice(set map[*vdl.Type]bool) (result []*vdl.Type) {
	for tt, _ := range set {
		result = append(result, tt)
	}
	return
}
func ttTypes(values []*vdl.Value) []*vdl.Type {
	uniq := make(map[*vdl.Type]bool)
	for _, v := range values {
		uniq[v.Type()] = true
	}
	return ttSetToSlice(uniq)
}
func ttJoin(types ...[]*vdl.Type) []*vdl.Type {
	uniq := make(map[*vdl.Type]bool)
	for _, ttSlice := range types {
		for _, tt := range ttSlice {
			uniq[tt] = true
		}
	}
	return ttSetToSlice(uniq)
}

// ttOtherThan returns all types from types that aren't represented in other.
func ttOtherThan(types []*vdl.Type, other ...[]*vdl.Type) (result []*vdl.Type) {
	otherMap := make(map[*vdl.Type]bool)
	for _, oo := range other {
		for _, o := range oo {
			otherMap[o] = true
		}
	}
	for _, t := range types {
		if !otherMap[t] {
			result = append(result, t)
		}
	}
	return
}

// Helpers to manipulate slices of reflect.Type
func rtSetToSlice(set map[reflect.Type]bool) (result []reflect.Type) {
	for rt, _ := range set {
		result = append(result, rt)
	}
	return
}
func rtTypes(values []interface{}) []reflect.Type {
	uniq := make(map[reflect.Type]bool)
	for _, v := range values {
		uniq[reflect.TypeOf(v)] = true
	}
	return rtSetToSlice(uniq)
}
func rtJoin(types ...[]reflect.Type) (result []reflect.Type) {
	uniq := make(map[reflect.Type]bool)
	for _, rtSlice := range types {
		for _, rt := range rtSlice {
			uniq[rt] = true
		}
	}
	return rtSetToSlice(uniq)
}

// rtOtherThan returns all types from types that aren't represented in other.
func rtOtherThan(types []reflect.Type, other ...[]reflect.Type) (result []reflect.Type) {
	otherMap := make(map[reflect.Type]bool)
	for _, oo := range other {
		for _, o := range oo {
			otherMap[o] = true
		}
	}
	for _, t := range types {
		if !otherMap[t] {
			result = append(result, t)
		}
	}
	return
}

// vvOnlyFrom returns all values from values that are represented in from.
func vvOnlyFrom(values []*vdl.Value, from []*vdl.Type) (result []*vdl.Value) {
	fromMap := make(map[*vdl.Type]bool)
	for _, f := range from {
		fromMap[f] = true
	}
	for _, v := range values {
		if fromMap[v.Type()] {
			result = append(result, v)
		}
	}
	return
}

// rvOnlyFrom returns all values from values that are represented in from.
func rvOnlyFrom(values []interface{}, from []reflect.Type) (result []interface{}) {
	fromMap := make(map[reflect.Type]bool)
	for _, f := range from {
		fromMap[f] = true
	}
	for _, v := range values {
		if fromMap[reflect.TypeOf(v)] {
			result = append(result, v)
		}
	}
	return
}

// vvFromUint returns all *Values that can represent u without loss of precision.
func vvFromUint(u uint64) (result []*vdl.Value) {
	b, i, f, c := byte(u), int64(u), float64(u), complex(float64(u), 0)
	switch {
	case u <= math.MaxInt8:
		fallthrough
	case u <= math.MaxUint8:
		result = append(result, byteValue(vdl.ByteType, b), byteValue(nByteType, b))
		fallthrough
	case u <= math.MaxInt16:
		result = append(result, intValue(vdl.Int16Type, i), intValue(int16TypeN, i))
		fallthrough
	case u <= math.MaxUint16:
		result = append(result, uintValue(vdl.Uint16Type, u), uintValue(uint16TypeN, u))
		fallthrough
	case u <= 1<<24:
		result = append(result, floatValue(vdl.Float32Type, f), floatValue(float32TypeN, f))
		result = append(result, complexValue(vdl.Complex64Type, c), complexValue(complex64TypeN, c))
		fallthrough
	case u <= math.MaxInt32:
		result = append(result, intValue(vdl.Int32Type, i), intValue(int32TypeN, i))
		fallthrough
	case u <= math.MaxUint32:
		result = append(result, uintValue(vdl.Uint32Type, u), uintValue(uint32TypeN, u))
		fallthrough
	case u <= 1<<53:
		result = append(result, floatValue(vdl.Float64Type, f), floatValue(float64TypeN, f))
		result = append(result, complexValue(vdl.Complex128Type, c), complexValue(complex128TypeN, c))
		fallthrough
	case u <= math.MaxInt64:
		result = append(result, intValue(vdl.Int64Type, i), intValue(int64TypeN, i))
		fallthrough
	default:
		result = append(result, uintValue(vdl.Uint64Type, u), uintValue(uint64TypeN, u))
	}
	return result
}

// rvFromUint returns all values that can represent u without loss of precision.
func rvFromUint(u uint64) (result []interface{}) {
	c64, c128 := complex(float32(u), 0), complex(float64(u), 0)
	switch {
	case u <= math.MaxInt8:
		result = append(result, int8(u), nInt8(u))
		fallthrough
	case u <= math.MaxUint8:
		result = append(result, uint8(u), nUint8(u))
		fallthrough
	case u <= math.MaxInt16:
		result = append(result, int16(u), nInt16(u))
		fallthrough
	case u <= math.MaxUint16:
		result = append(result, uint16(u), nUint16(u))
		fallthrough
	case u <= 1<<24:
		result = append(result, float32(u), nFloat32(u))
		result = append(result, c64, nComplex64(c64))
		fallthrough
	case u <= math.MaxInt32:
		result = append(result, int32(u), nInt32(u))
		fallthrough
	case u <= math.MaxUint32:
		result = append(result, uint32(u), nUint32(u))
		fallthrough
	case u <= 1<<53:
		result = append(result, float64(u), nFloat64(u))
		result = append(result, c128, nComplex128(c128))
		fallthrough
	case u <= math.MaxInt64:
		result = append(result, int64(u), nInt64(u))
		fallthrough
	default:
		result = append(result, uint64(u), nUint64(u))
	}
	return result
}

// vvFromInt returns all *Values that can represent i without loss of precision.
func vvFromInt(i int64) (result []*vdl.Value) {
	b, u, f, c := byte(i), uint64(i), float64(i), complex(float64(i), 0)
	switch {
	case math.MinInt8 <= i && i <= math.MaxInt8:
		fallthrough
	case math.MinInt16 <= i && i <= math.MaxInt16:
		result = append(result, intValue(vdl.Int16Type, i), intValue(int16TypeN, i))
		fallthrough
	case -1<<24 <= i && i <= 1<<24:
		result = append(result, floatValue(vdl.Float32Type, f), floatValue(float32TypeN, f))
		result = append(result, complexValue(vdl.Complex64Type, c), complexValue(complex64TypeN, c))
		fallthrough
	case math.MinInt32 <= i && i <= math.MaxInt32:
		result = append(result, intValue(vdl.Int32Type, i), intValue(int32TypeN, i))
		fallthrough
	case -1<<53 <= i && i <= 1<<53:
		result = append(result, floatValue(vdl.Float64Type, f), floatValue(float64TypeN, f))
		result = append(result, complexValue(vdl.Complex128Type, c), complexValue(complex128TypeN, c))
		fallthrough
	default:
		result = append(result, intValue(vdl.Int64Type, i), intValue(int64TypeN, i))
	}
	if i < 0 {
		return
	}
	switch {
	case i <= math.MaxUint8:
		result = append(result, byteValue(vdl.ByteType, b), byteValue(nByteType, b))
		fallthrough
	case i <= math.MaxUint16:
		result = append(result, uintValue(vdl.Uint16Type, u), uintValue(uint16TypeN, u))
		fallthrough
	case i <= math.MaxUint32:
		result = append(result, uintValue(vdl.Uint32Type, u), uintValue(uint32TypeN, u))
		fallthrough
	default:
		result = append(result, uintValue(vdl.Uint64Type, u), uintValue(uint64TypeN, u))
	}
	return
}

// rvFromInt returns all values that can represent i without loss of precision.
func rvFromInt(i int64) (result []interface{}) {
	c64, c128 := complex(float32(i), 0), complex(float64(i), 0)
	switch {
	case math.MinInt8 <= i && i <= math.MaxInt8:
		result = append(result, int8(i), nInt8(i))
		fallthrough
	case math.MinInt16 <= i && i <= math.MaxInt16:
		result = append(result, int16(i), nInt16(i))
		fallthrough
	case -1<<24 <= i && i <= 1<<24:
		result = append(result, float32(i), nFloat32(i))
		result = append(result, c64, nComplex64(c64))
		fallthrough
	case math.MinInt32 <= i && i <= math.MaxInt32:
		result = append(result, int32(i), nInt32(i))
		fallthrough
	case -1<<53 <= i && i <= 1<<53:
		result = append(result, float64(i), nFloat64(i))
		result = append(result, c128, nComplex128(c128))
		fallthrough
	default:
		result = append(result, int64(i), nInt64(i))
	}
	if i < 0 {
		return
	}
	switch {
	case i <= math.MaxUint8:
		result = append(result, uint8(i), nUint8(i))
		fallthrough
	case i <= math.MaxUint16:
		result = append(result, uint16(i), nUint16(i))
		fallthrough
	case i <= math.MaxUint32:
		result = append(result, uint32(i), nUint32(i))
		fallthrough
	default:
		result = append(result, uint64(i), nUint64(i))
	}
	return
}

func vvFloat(f float64) []*vdl.Value {
	c := complex(f, 0)
	return []*vdl.Value{
		floatValue(vdl.Float32Type, f), floatValue(float32TypeN, f),
		floatValue(vdl.Float64Type, f), floatValue(float64TypeN, f),
		complexValue(vdl.Complex64Type, c), complexValue(complex64TypeN, c),
		complexValue(vdl.Complex128Type, c), complexValue(complex128TypeN, c),
	}
}

func vvComplex(c complex128) []*vdl.Value {
	return []*vdl.Value{
		complexValue(vdl.Complex64Type, c), complexValue(complex64TypeN, c),
		complexValue(vdl.Complex128Type, c), complexValue(complex128TypeN, c),
	}
}

func rvFloat(f float64) []interface{} {
	c64, c128 := complex(float32(f), 0), complex(f, 0)
	return []interface{}{
		float32(f), nFloat32(f),
		float64(f), nFloat64(f),
		c64, nComplex64(c64),
		c128, nComplex128(c128),
	}
}

func rvComplex(c128 complex128) []interface{} {
	c64 := complex64(c128)
	return []interface{}{
		c64, nComplex64(c64),
		c128, nComplex128(c128),
	}
}

// Test successful conversions.  Each test contains a set of values and
// interfaces that are all equivalent and convertible to each other.
func TestConverter(t *testing.T) {
	tests := []struct {
		vv []*vdl.Value
		rv []interface{}
	}{
		{[]*vdl.Value{vvError1}, []interface{}{rvError1}},
		{[]*vdl.Value{vvError2}, []interface{}{rvError2}},
		{[]*vdl.Value{vvError3}, []interface{}{rvError3}},
		{vvBoolTrue, rvBoolTrue},
		{vvStrABC, rvStrABC},
		{vvFromUint(math.MaxUint8), rvFromUint(math.MaxUint8)},
		{vvFromUint(math.MaxUint16), rvFromUint(math.MaxUint16)},
		{vvFromUint(math.MaxUint32), rvFromUint(math.MaxUint32)},
		{vvFromUint(math.MaxUint64), rvFromUint(math.MaxUint64)},
		{vvFromInt(math.MaxInt8), rvFromInt(math.MaxInt8)},
		{vvFromInt(math.MaxInt16), rvFromInt(math.MaxInt16)},
		{vvFromInt(math.MaxInt32), rvFromInt(math.MaxInt32)},
		{vvFromInt(math.MaxInt64), rvFromInt(math.MaxInt64)},
		{vvFromInt(math.MinInt8), rvFromInt(math.MinInt8)},
		{vvFromInt(math.MinInt16), rvFromInt(math.MinInt16)},
		{vvFromInt(math.MinInt32), rvFromInt(math.MinInt32)},
		{vvFromInt(math.MinInt64), rvFromInt(math.MinInt64)},
		{vvFromInt(float32MaxInt), rvFromInt(float32MaxInt)},
		{vvFromInt(float64MaxInt), rvFromInt(float64MaxInt)},
		{vvFromInt(float32MinInt), rvFromInt(float32MinInt)},
		{vvFromInt(float64MinInt), rvFromInt(float64MinInt)},
		{vvTypeObjectBool, rvTypeObjectBool},
		{vvSeq123, rvSeq123},
		{vvSetMap123, rvSetMap123},
		{vvMap123FalseTrue, rvMap123FalseTrue},
		{vvSetMapStructXYZ, rvSetMapStructXYZ},
		{vvMapStructXYZFalseTrue, rvMapStructXYZFalseTrue},
		{vvMapStructXYZEmpty, rvMapStructXYZEmpty},
		{vvStructWXFalseTrue, rvStructWXFalseTrue},
		{vvMapStructVWX123, rvMapStructVWX123},
		{vvStructUV01, rvStructUV01},
		{nil, []interface{}{rvNative0}},
		{nil, []interface{}{rvNative1}},
		{nil, []interface{}{rvNativeMin}},
		{nil, []interface{}{rvNativeMax}},
	}
	for _, test := range tests {
		testConverterWantSrc(t, vvrv{test.vv, test.rv}, vvrv{test.vv, test.rv})
	}
}

// Test successful conversions that drop and ignore fields in the dst struct.
func TestConverterStructDropIgnore(t *testing.T) {
	tests := []struct {
		vvWant []*vdl.Value
		rvWant []interface{}
		vvSrc  []*vdl.Value
		rvSrc  []interface{}
	}{
		{vvStructWXFalseTrue, rvStructWXFalseTrue, vvSetMapStructXYZ, rvSetMapStructXYZ},
		{vvStructUV01, rvStructUV01, vvMapStructVWX123, rvMapStructVWX123},
	}
	for _, test := range tests {
		testConverterWantSrc(t, vvrv{test.vvWant, test.rvWant}, vvrv{test.vvSrc, test.rvSrc})
	}
}

// Test successful conversions to and from union values.
func TestConverterUnion(t *testing.T) {
	// values for union component types
	vvTrue := vdl.BoolValue(true)
	vv123 := vdl.Int64Value(123)
	vvAbc := vdl.StringValue("Abc")
	vvStruct123 := vdl.ZeroValue(structInt64TypeN)
	vvStruct123.Field(0).Assign(vv123)
	rvTrue := bool(true)
	rv123 := int64(123)
	rvAbc := string("Abc")
	rvStruct123 := nStructInt64{123}
	// values for union{A bool;B string;C struct}
	vvTrueABC := vdl.ZeroValue(unionABCTypeN).AssignUnionField(0, vvTrue)
	vvAbcABC := vdl.ZeroValue(unionABCTypeN).AssignUnionField(1, vvAbc)
	vvStruct123ABC := vdl.ZeroValue(unionABCTypeN).AssignUnionField(2, vvStruct123)
	rvTrueABC := nUnionABCA{rvTrue}
	rvAbcABC := nUnionABCB{rvAbc}
	rvStruct123ABC := nUnionABCC{rvStruct123}
	rvTrueABCi := nUnionABC(rvTrueABC)
	rvAbcABCi := nUnionABC(rvAbcABC)
	rvStruct123ABCi := nUnionABC(rvStruct123ABC)
	// values for union{B string;C struct;D int64}
	vvAbcBCD := vdl.ZeroValue(unionBCDTypeN).AssignUnionField(0, vvAbc)
	vvStruct123BCD := vdl.ZeroValue(unionBCDTypeN).AssignUnionField(1, vvStruct123)
	vv123BCD := vdl.ZeroValue(unionBCDTypeN).AssignUnionField(2, vv123)
	rvAbcBCD := nUnionBCDB{rvAbc}
	rvStruct123BCD := nUnionBCDC{rvStruct123}
	rv123BCD := nUnionBCDD{rv123}
	rvAbcBCDi := nUnionBCD(rvAbcBCD)
	rvStruct123BCDi := nUnionBCD(rvStruct123BCD)
	rv123BCDi := nUnionBCD(rv123BCD)
	// values for union{X string;Y struct}, which has no Go equivalent.
	vvAbcXY := vdl.ZeroValue(unionXYTypeN).AssignUnionField(0, vvAbc)
	vvStruct123XY := vdl.ZeroValue(unionXYTypeN).AssignUnionField(1, vvStruct123)

	tests := []struct {
		vvWant *vdl.Value
		rvWant interface{}
		vvSrc  *vdl.Value
		rvSrc  interface{}
	}{
		// Convert source and target same union.
		{vvTrueABC, rvTrueABC, vvTrueABC, rvTrueABC},
		{vv123BCD, rv123BCD, vv123BCD, rv123BCD},
		{vvAbcABC, rvAbcABC, vvAbcABC, rvAbcABC},
		{vvAbcBCD, rvAbcBCD, vvAbcBCD, rvAbcBCD},
		{vvStruct123ABC, rvStruct123ABC, vvStruct123ABC, rvStruct123ABC},
		{vvStruct123BCD, rvStruct123BCD, vvStruct123BCD, rvStruct123BCD},
		// Same thing, but with pointers to the interface type.
		{vvTrueABC, &rvTrueABCi, vvTrueABC, &rvTrueABCi},
		{vv123BCD, &rv123BCDi, vv123BCD, &rv123BCD},
		{vvAbcABC, &rvAbcABCi, vvAbcABC, &rvAbcABC},
		{vvAbcBCD, &rvAbcBCDi, vvAbcBCD, &rvAbcBCD},
		{vvStruct123ABC, &rvStruct123ABCi, vvStruct123ABC, &rvStruct123ABCi},
		{vvStruct123BCD, &rvStruct123BCDi, vvStruct123BCD, &rvStruct123BCDi},

		// Convert source and target different union.
		{vvAbcABC, rvAbcABC, vvAbcBCD, rvAbcBCD},
		{vvAbcBCD, rvAbcBCD, vvAbcABC, rvAbcABC},
		{vvStruct123ABC, rvStruct123ABC, vvStruct123BCD, rvStruct123BCD},
		{vvStruct123BCD, rvStruct123BCD, vvStruct123ABC, rvStruct123ABC},
		// Same thing, but with pointers to the interface type.
		{vvAbcABC, &rvAbcABCi, vvAbcBCD, &rvAbcBCDi},
		{vvAbcBCD, &rvAbcBCDi, vvAbcABC, &rvAbcABCi},
		{vvStruct123ABC, &rvStruct123ABCi, vvStruct123BCD, &rvStruct123BCDi},
		{vvStruct123BCD, &rvStruct123BCDi, vvStruct123ABC, &rvStruct123ABCi},

		// Test unions that have no Go equivalent.
		{vvAbcXY, nil, vvAbcXY, nil},
		{vvStruct123XY, nil, vvStruct123XY, nil},
	}
	for _, test := range tests {
		testConverterWantSrc(t,
			vvrv{vvSlice(test.vvWant), rvSlice(test.rvWant)},
			vvrv{vvSlice(test.vvSrc), rvSlice(test.rvSrc)})
	}
}

func vvSlice(v *vdl.Value) []*vdl.Value {
	if v != nil {
		return []*vdl.Value{v}
	}
	return nil
}

func rvSlice(v interface{}) []interface{} {
	if v != nil {
		return []interface{}{v}
	}
	return nil
}

// Test successful conversions to and from nil values.
func TestConverterNil(t *testing.T) {
	vvNil := vdl.ZeroValue(vdl.AnyType)
	rvNil := new(interface{})
	vvNilError := vdl.ZeroValue(vdl.ErrorType)
	rvNilError := new(error)
	vvNilPtrStruct := vdl.ZeroValue(vdl.TypeOf((*nStructInt)(nil)))
	rvNilPtrStruct := (*nStructInt)(nil)
	vvStructNilStructField := vdl.ZeroValue(vdl.TypeOf(nStructOptionalStruct{}))
	rvStructNilStructField := nStructOptionalStruct{X: nil}
	vvStructNilAnyField := vdl.ZeroValue(vdl.TypeOf(nStructOptionalAny{}))
	rvStructNilAnyField := nStructOptionalAny{X: nil}
	tests := []struct {
		vvWant *vdl.Value
		rvWant interface{}
		vvSrc  *vdl.Value
		rvSrc  interface{}
	}{
		// Conversion source and target are the same.
		{vvNil, rvNil, vvNil, rvNil},
		{vvNilError, rvNilError, vvNilError, rvNilError},
		{vvNilPtrStruct, rvNilPtrStruct, vvNilPtrStruct, rvNilPtrStruct},
		{vvStructNilStructField, rvStructNilStructField, vvStructNilStructField, rvStructNilStructField},
		{vvStructNilAnyField, rvStructNilAnyField, vvStructNilAnyField, rvStructNilAnyField},
		// All typed nil targets may be converted from any(nil).
		{vvNilError, rvNilError, vvNil, rvNil},
		{vvNilPtrStruct, rvNilPtrStruct, vvNil, rvNil},
		{vvStructNilStructField, rvStructNilStructField, vvStructNilAnyField, rvStructNilAnyField},
	}
	for _, test := range tests {
		testConverterWantSrc(t,
			vvrv{[]*vdl.Value{test.vvWant}, []interface{}{test.rvWant}},
			vvrv{[]*vdl.Value{test.vvSrc}, []interface{}{test.rvSrc}})
	}
}

type vvrv struct {
	vv []*vdl.Value
	rv []interface{}
}

func testConverterWantSrc(t *testing.T, vvrvWant, vvrvSrc vvrv) {
	// We run each testConvert helper twice; the first call tests filling in a
	// zero dst, and the second call tests filling in a non-zero dst.

	// Tests of filling from *Value
	for _, vvSrc := range vvrvSrc.vv {
		for _, vvWant := range vvrvWant.vv {
			// Test filling *Value from *Value
			vvDst := vdl.ZeroValue(vvWant.Type())
			testConvert(t, "vv1", vvDst, vvSrc, vvWant, 0, false)
			testConvert(t, "vv2", vvDst, vvSrc, vvWant, 0, false)
		}
		for _, want := range vvrvWant.rv {
			// Test filling reflect.Value from *Value
			dst := reflect.New(reflect.TypeOf(want)).Interface()
			testConvert(t, "vv3", dst, vvSrc, want, 1, false)
			testConvert(t, "vv4", dst, vvSrc, want, 1, false)
		}
		// Test filling Any from *Value
		vvDst := vdl.ZeroValue(vdl.AnyType)
		testConvert(t, "vv5", vvDst, vvSrc, anyValue(vvSrc), 0, false)
		testConvert(t, "vv6", vvDst, vvSrc, anyValue(vvSrc), 0, false)
		// Test filling Optional from *Value
		if vvSrc.Type().CanBeOptional() {
			ttNil := vdl.OptionalType(vvSrc.Type())
			vvNil := vdl.ZeroValue(ttNil)
			vvOptWant := vdl.OptionalValue(vvSrc)
			testConvert(t, "vv7", vvNil, vvSrc, vvOptWant, 0, false)
			testConvert(t, "vv8", vvNil, vvSrc, vvOptWant, 0, false)
		}
		// Test filling **Value(nil) from *Value
		var vvValue *vdl.Value
		testConvert(t, "vv9", &vvValue, vvSrc, vvSrc, 1, false)
		testConvert(t, "vv10", &vvValue, vvSrc, vvSrc, 1, false)
		// Test filling interface{} from *Value
		var dst interface{}
		var want interface{} = vvSrc
		if !canCreateGoObject(vvSrc.Type()) {
			// We only run these tests if we *can't* create an actual Go object for
			// this type, so we'll end up with *vdl.Value.
			//
			// The problem is that we don't know what value to expect.  It seems
			// pointless to run a conversion to the rv src, since it's a
			// self-fulfilling test.  So we just skip this, and let the rv tests below
			// check Go object generation.
			//
			// TODO(toddw): Test with parallel rv and vv, to get better coverage.
			if vvSrc.Type().CanBeNil() && vvSrc.IsNil() {
				want = nil // filling interface{} from any(nil) yields nil.
			}
			testConvert(t, "vv11", &dst, vvSrc, want, 1, false)
			testConvert(t, "vv12", &dst, vvSrc, want, 1, false)
		}
		if vvSrc.Kind() == vdl.Struct {
			// Every struct may be converted to the empty struct
			testConvert(t, "vv13", vdl.ZeroValue(emptyType), vvSrc, vdl.ZeroValue(emptyType), 0, false)
			testConvert(t, "vv14", vdl.ZeroValue(emptyTypeN), vvSrc, vdl.ZeroValue(emptyTypeN), 0, false)
			var empty struct{}
			var emptyN nEmpty
			testConvert(t, "vv15", &empty, vvSrc, struct{}{}, 1, false)
			testConvert(t, "vv16", &emptyN, vvSrc, nEmpty{}, 1, false)
			// The empty struct may be converted to the zero value of any struct
			vvZeroSrc := vdl.ZeroValue(vvSrc.Type())
			testConvert(t, "vv17", vdl.ZeroValue(vvSrc.Type()), vdl.ZeroValue(emptyType), vvZeroSrc, 0, false)
			testConvert(t, "vv18", vdl.ZeroValue(vvSrc.Type()), vdl.ZeroValue(emptyTypeN), vvZeroSrc, 0, false)
			testConvert(t, "vv19", vdl.ZeroValue(vvSrc.Type()), struct{}{}, vvZeroSrc, 0, false)
			testConvert(t, "vv20", vdl.ZeroValue(vvSrc.Type()), nEmpty{}, vvZeroSrc, 0, false)
		}
	}

	// Tests of filling from reflect.Value
	for _, src := range vvrvSrc.rv {
		rtSrc := reflect.TypeOf(src)
		for _, vvWant := range vvrvWant.vv {
			// Test filling *Value from reflect.Value
			vvDst := vdl.ZeroValue(vvWant.Type())
			testConvert(t, "rv1", vvDst, src, vvWant, 0, false)
			testConvert(t, "rv2", vvDst, src, vvWant, 0, false)
		}
		for _, want := range vvrvWant.rv {
			// Test filling reflect.Value from reflect.Value
			dst := reflect.New(reflect.TypeOf(want)).Interface()
			testConvert(t, "rv3", dst, src, want, 1, false)
			testConvert(t, "rv4", dst, src, want, 1, false)
		}
		var vvWant *vdl.Value
		if err := Convert(&vvWant, src); err != nil {
			t.Errorf("Convert(*Value, %T) error: %v", src, err)
			continue
		}
		// Test filling Any from reflect.Value
		vvDst := vdl.ZeroValue(vdl.AnyType)
		testConvert(t, "rv5", vvDst, src, anyValue(vvWant), 0, true)
		testConvert(t, "rv6", vvDst, src, anyValue(vvWant), 0, true)
		// Test filling **Value(nil) from reflect.Value
		var vvValue *vdl.Value
		testConvert(t, "rv7", &vvValue, src, vvWant, 1, false)
		testConvert(t, "rv8", &vvValue, src, vvWant, 1, false)
		// Test filling interface{} from reflect.Value
		var dst interface{}
		var want interface{} = src
		// Handle special-cases for want values.
		srcInt8, isInt8 := src.(int8)
		switch {
		case isInt8:
			// VDL represents int8 as int16, so set our expectations accordingly.
			want = int16(srcInt8)
		case rtSrc != rtPtrToType && rtSrc.ConvertibleTo(rtPtrToType):
			// VDL doesn't represent named TypeObject, so our converter automatically
			// creates *vdl.Type.
			want = reflect.ValueOf(src).Convert(rtPtrToType).Interface().(*vdl.Type)
		case vvWant.Type().CanBeNil() && vvWant.IsNil():
			want = nil // filling interface{} from any(nil) yields nil.
		case !canCreateGoObject(vvWant.Type()):
			// We can't create an actual Go object for this type; e.g. perhaps it's
			// named, and isn't registered.  We should get a *vdl.Value back.
			want = vvWant
		}
		testConvert(t, "rv9", &dst, src, want, 1, true)
		testConvert(t, "rv10", &dst, src, want, 1, true)
		ttSrc, err := vdl.TypeFromReflect(rtSrc)
		if err != nil {
			t.Error(err)
			continue
		}
		if rtSrc.Kind() == reflect.Struct && ttSrc.Kind() != vdl.Union {
			// Every struct may be converted to the empty struct
			testConvert(t, "rv11", vdl.ZeroValue(emptyType), src, vdl.ZeroValue(emptyType), 0, false)
			testConvert(t, "rv12", vdl.ZeroValue(emptyTypeN), src, vdl.ZeroValue(emptyTypeN), 0, false)
			var empty struct{}
			var emptyN nEmpty
			testConvert(t, "rv13", &empty, src, struct{}{}, 1, false)
			testConvert(t, "rv14", &emptyN, src, nEmpty{}, 1, false)
			// The empty struct may be converted to the zero value of any struct
			rvZeroSrc := reflect.Zero(rtSrc).Interface()
			testConvert(t, "rv15", reflect.New(rtSrc).Interface(), vdl.ZeroValue(emptyType), rvZeroSrc, 1, false)
			testConvert(t, "rv16", reflect.New(rtSrc).Interface(), vdl.ZeroValue(emptyTypeN), rvZeroSrc, 1, false)
			testConvert(t, "rv17", reflect.New(rtSrc).Interface(), struct{}{}, rvZeroSrc, 1, false)
			testConvert(t, "rv18", reflect.New(rtSrc).Interface(), nEmpty{}, rvZeroSrc, 1, false)
		}
	}
}

// canCreateGoObject returns true iff we can create a regular Go object from a
// value of type tt.  We can create a real Go object if the Go type for tt has
// been registered, or if tt is the special-cased error type.
func canCreateGoObject(tt *vdl.Type) bool {
	return vdl.ReflectFromType(tt) != nil || tt == vdl.ErrorType || tt == vdl.ErrorType.Elem()
}

func testConvert(t *testing.T, prefix string, dst, src, want interface{}, deref int, optWant bool) {
	const ptrDepth = 3
	rvDst := reflect.ValueOf(dst)
	for dstptrs := 0; dstptrs < ptrDepth; dstptrs++ {
		rvSrc := reflect.ValueOf(src)
		for srcptrs := 0; srcptrs < ptrDepth; srcptrs++ {
			tname := fmt.Sprintf("%s ReflectTarget(%v).From(%v)", prefix, rvDst.Type(), rvSrc.Type())
			// This is tricky - if optWant is set, we might need to change the want
			// value to become optional or non-optional.
			eWant, rvWant, ttWant := want, reflect.ValueOf(want), vdl.TypeOf(want)
			if optWant {
				vvWant, wantIsVV := want.(*vdl.Value)
				if srcptrs > 0 {
					if wantIsVV {
						switch {
						case vvWant.Kind() == vdl.Any && !vvWant.IsNil() && vvWant.Elem().Type().CanBeOptional():
							// Turn any(struct{...}) into any(?struct{...})
							eWant = anyValue(vdl.OptionalValue(vvWant.Elem()))
						case vvWant.Type().CanBeOptional():
							// Turn struct{...} into ?struct{...}
							eWant = vdl.OptionalValue(vvWant)
						}
					} else if ttWant.Kind() == vdl.Optional || ttWant.CanBeOptional() {
						// Add a pointer to anything that can be optional.
						rvPtrWant := reflect.New(rvWant.Type())
						rvPtrWant.Elem().Set(rvWant)
						eWant = rvPtrWant.Interface()
					}
				}
				if !wantIsVV && ttWant.Kind() != vdl.TypeObject && !ttWant.CanBeOptional() && rvWant.Kind() == reflect.Ptr {
					// Remove a pointer from  anything that can't be optional.
					eWant = rvWant.Elem().Interface()
				}
			}
			target, err := ReflectTarget(rvDst)
			expectErr(t, err, "", tname)
			err = FromReflect(target, rvSrc)
			expectErr(t, err, "", tname)
			expectConvert(t, tname, dst, eWant, deref)
			// Next iteration adds a pointer to src.
			rvNewSrc := reflect.New(rvSrc.Type())
			rvNewSrc.Elem().Set(rvSrc)
			rvSrc = rvNewSrc
		}
		// Next iteration adds a pointer to dst.
		rvNewDst := reflect.New(rvDst.Type())
		rvNewDst.Elem().Set(rvDst)
		rvDst = rvNewDst
	}
}

func expectConvert(t *testing.T, tname string, got, want interface{}, deref int) {
	rvGot := reflect.ValueOf(got)
	for d := 0; d < deref; d++ {
		if rvGot.Kind() != reflect.Ptr || rvGot.IsNil() {
			t.Errorf("%s can't deref %d %T %v", deref, got, got)
			return
		}
		rvGot = rvGot.Elem()
	}
	got = rvGot.Interface()
	vvGot, ok1 := got.(*vdl.Value)
	vvWant, ok2 := want.(*vdl.Value)
	if ok1 && ok2 {
		if !vdl.EqualValue(vvGot, vvWant) {
			t.Errorf("%s\nGOT  %v\nWANT %v", tname, vvGot, vvWant)
		}
		return
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("%s\nGOT  %#v\nWANT %#v", tname, got, want)
	}
}

// Test failed conversions.
func TestConverterError(t *testing.T) {
	tests := []struct {
		ttDst []*vdl.Type
		rtDst []reflect.Type
		vvSrc []*vdl.Value
		rvSrc []interface{}
	}{
		{ttOtherThan(ttAllTypes, ttBools), rtOtherThan(rtAllTypes, rtBools),
			vvBoolTrue, rvBoolTrue},
		{ttOtherThan(ttAllTypes, ttStrs), rtOtherThan(rtAllTypes, rtStrs),
			vvStrABC, rvStrABC},
		{ttOtherThan(ttAllTypes, ttTypeObjects), rtOtherThan(rtAllTypes, rtTypeObjects),
			vvTypeObjectBool, rvTypeObjectBool},
		{ttOtherThan(ttAllTypes, ttSeq123), rtOtherThan(rtAllTypes, rtSeq123),
			vvSeq123, rvSeq123},
		{ttOtherThan(ttAllTypes, ttSetMap123), rtOtherThan(rtAllTypes, rtSetMap123),
			vvSetMap123, rvSetMap123},
		{ttOtherThan(ttAllTypes, ttSetMapStructXYZ), rtOtherThan(rtAllTypes, rtSetMapStructXYZ),
			vvSetMapStructXYZ, rvSetMapStructXYZ},
		{ttOtherThan(ttAllTypes, ttMapStructVWXNum), rtOtherThan(rtAllTypes, rtMapStructVWXNum),
			vvMapStructVWX123, rvMapStructVWX123},
		// Test invalid conversions to set types
		{ttSet123, rtSet123, vvMap123FalseTrue, rvMap123FalseTrue},
		{ttSetXYZ, rtSetXYZ, vvMapStructXYZFalseTrue, rvMapStructXYZFalseTrue},
		{ttSetMapStructXYZ, rtSetMapStructXYZ, vvMapStructXYZEmpty, rvMapStructXYZEmpty},
		// Test invalid conversions to struct types: mismatched field types
		{ttStructXYZBool, rtStructXYZBool, vvMapStructVWX123, rvMapStructVWX123},
		{ttStructVWXNum, rtStructVWXNum, vvSetMapStructXYZ, rvSetMapStructXYZ},
		// Test invalid conversions to struct types: no fields in common
		{ttStructWXBool, rtStructWXBool, vvStructUV01, rvStructUV01},
		{ttStructUVNum, rtStructUVNum, vvStructWXFalseTrue, rvStructWXFalseTrue},
		// Test uint values one past the max bound.
		{ttOtherThan(ttIntegers, ttTypes(vvFromUint(math.MaxUint8))),
			rtOtherThan(rtIntegers, rtTypes(rvFromUint(math.MaxUint8))),
			vvFromUint(math.MaxUint8 + 1),
			rvFromUint(math.MaxUint8 + 1)},
		{ttOtherThan(ttIntegers, ttTypes(vvFromUint(math.MaxUint16))),
			rtOtherThan(rtIntegers, rtTypes(rvFromUint(math.MaxUint16))),
			vvFromUint(math.MaxUint16 + 1),
			rvFromUint(math.MaxUint16 + 1)},
		{ttOtherThan(ttIntegers, ttTypes(vvFromUint(math.MaxUint32))),
			rtOtherThan(rtIntegers, rtTypes(rvFromUint(math.MaxUint32))),
			vvFromUint(math.MaxUint32 + 1),
			rvFromUint(math.MaxUint32 + 1)},
		// Test int values one past the max bound.
		{ttOtherThan(ttIntegers, ttTypes(vvFromUint(math.MaxInt8))),
			rtOtherThan(rtIntegers, rtTypes(rvFromUint(math.MaxInt8))),
			vvFromUint(math.MaxInt8 + 1),
			rvFromUint(math.MaxInt8 + 1)},
		{ttOtherThan(ttIntegers, ttTypes(vvFromUint(math.MaxInt16))),
			rtOtherThan(rtIntegers, rtTypes(rvFromUint(math.MaxInt16))),
			vvFromUint(math.MaxInt16 + 1),
			rvFromUint(math.MaxInt16 + 1)},
		{ttOtherThan(ttIntegers, ttTypes(vvFromUint(math.MaxInt32))),
			rtOtherThan(rtIntegers, rtTypes(rvFromUint(math.MaxInt32))),
			vvFromUint(math.MaxInt32 + 1),
			rvFromUint(math.MaxInt32 + 1)},
		{ttOtherThan(ttIntegers, ttTypes(vvFromUint(math.MaxInt64))),
			rtOtherThan(rtIntegers, rtTypes(rvFromUint(math.MaxInt64))),
			vvFromUint(math.MaxInt64 + 1),
			rvFromUint(math.MaxInt64 + 1)},
		// Test int values one past the min bound.
		{ttOtherThan(ttIntegers, ttTypes(vvFromInt(math.MinInt8))),
			rtOtherThan(rtIntegers, rtTypes(rvFromInt(math.MinInt8))),
			vvFromInt(math.MinInt8 - 1),
			rvFromInt(math.MinInt8 - 1)},
		{ttOtherThan(ttIntegers, ttTypes(vvFromInt(math.MinInt16))),
			rtOtherThan(rtIntegers, rtTypes(rvFromInt(math.MinInt16))),
			vvFromInt(math.MinInt16 - 1),
			rvFromInt(math.MinInt16 - 1)},
		{ttOtherThan(ttIntegers, ttTypes(vvFromInt(math.MinInt32))),
			rtOtherThan(rtIntegers, rtTypes(rvFromInt(math.MinInt32))),
			vvFromInt(math.MinInt32 - 1),
			rvFromInt(math.MinInt32 - 1)},
		// Test int to float max bound.
		{ttJoin(ttFloat32s, ttComplex64s), rtJoin(rtFloat32s, rtComplex64s),
			vvOnlyFrom(vvFromInt(float32MaxInt+1), ttIntegers),
			rvOnlyFrom(rvFromInt(float32MaxInt+1), rtIntegers)},
		{ttJoin(ttFloat64s, ttComplex128s), rtJoin(rtFloat64s, rtComplex128s),
			vvOnlyFrom(vvFromInt(float64MaxInt+1), ttIntegers),
			rvOnlyFrom(rvFromInt(float64MaxInt+1), rtIntegers)},
		// Test int to float min bound.
		{ttJoin(ttFloat32s, ttComplex64s), rtJoin(rtFloat32s, rtComplex64s),
			vvOnlyFrom(vvFromInt(float32MinInt-1), ttIntegers),
			rvOnlyFrom(rvFromInt(float32MinInt-1), rtIntegers)},
		{ttJoin(ttFloat64s, ttComplex128s), rtJoin(rtFloat64s, rtComplex128s),
			vvOnlyFrom(vvFromInt(float64MinInt-1), ttIntegers),
			rvOnlyFrom(rvFromInt(float64MinInt-1), rtIntegers)},
		// Test negative uints, fractional integers, imaginary non-complex numbers.
		{ttUints, rtUints, vvFromInt(-1), rvFromInt(-1)},
		{ttIntegers, rtIntegers, vvFloat(1.5), rvFloat(1.5)},
		{ttOtherThan(ttNumbers, ttComplexes), rtOtherThan(rtNumbers, rtComplexes),
			vvComplex(1 + 2i), rvComplex(1 + 2i)},
	}
	for _, test := range tests {
		for _, ttDst := range test.ttDst {
			tname := fmt.Sprintf("ValueTarget(%v)", ttDst)
			vvDst := vdl.ZeroValue(ttDst)
			target, err := ValueTarget(vvDst)
			if !expectErr(t, err, "", tname) {
				continue
			}
			for _, vvSrc := range test.vvSrc {
				if err := FromValue(target, vvSrc); err == nil {
					t.Errorf("%s FromValue(%v) got %v, want error", tname, vvSrc.Type(), vvDst)
				}
			}
			for _, src := range test.rvSrc {
				rvSrc := reflect.ValueOf(src)
				if err := FromReflect(target, rvSrc); err == nil {
					t.Errorf("%s FromReflect(%v) got %v, want error", tname, rvSrc.Type(), vvDst)
				}
			}
		}
		for _, rtDst := range test.rtDst {
			tname := fmt.Sprintf("ReflectTarget(%v)", rtDst)
			rvDst := reflect.New(rtDst)
			target, err := ReflectTarget(rvDst)
			if !expectErr(t, err, "", tname) {
				continue
			}
			got := rvDst.Elem().Interface()
			for _, vvSrc := range test.vvSrc {
				if err := FromValue(target, vvSrc); err == nil {
					t.Errorf("%s FromValue(%v) got %v, want error", tname, vvSrc.Type(), got)
				}
			}
			for _, src := range test.rvSrc {
				rvSrc := reflect.ValueOf(src)
				if err := FromReflect(target, rvSrc); err == nil {
					t.Errorf("%s FromReflect(%v) got %v, want error", tname, rvSrc.Type(), got)
				}
			}
		}
	}
}
