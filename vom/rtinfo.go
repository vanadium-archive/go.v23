package vom

import (
	"fmt"
	"reflect"
	"sync"

	"v.io/veyron/veyron2/wiretype"
)

// rtInfo holds type information derived from Type, used during encoding
// and decoding.  We pre-compute this information as a performance optimization
// since some of the operations on Type are slow.
//
// rtInfo is a single struct that holds the union of all fields we need for all
// kinds of types.  An alternative is to make rtInfo an interface, and have
// different underlying structs for each kind of type.  That costs us ~10%
// performance, so we go with this.
type rtInfo struct {
	// Common fields for all kinds of types.
	rt         Type
	zero       Value    // zero value for type rt.
	numMethods int      // number of methods in rt's method set.
	name       string   // name of the type, in standard format.
	isNamed    bool     // is this a user-defined named type?
	snames     []string // short names
	tags       []string
	kind       typeKind
	numStars   int // number of stars in Type, or 0 if not a pointer.

	// Valid for primitive types.
	id wiretype.TypeID

	// Valid for uint, int, float - e.g. "8", "16", "ptr", etc.
	suffix string

	// Valid for slice, array, map, ptr.
	elem *rtInfo

	// Valid for array.
	len int

	// Valid for map.
	key *rtInfo

	// Valid for struct.
	fields       []*rtInfoField
	fieldsByName map[string]*rtInfoField

	// Valid for types with custom coders.
	customBinary *customCoder
	customJSON   *customCoder
	// We pre-compute the number of stars of the custom coder arg, and propagate
	// it up for pointer types.  This is used for pointer alignment.
	numStarsBinary int
	numStarsJSON   int
}

type rtInfoField struct {
	name  string
	index int
	info  *rtInfo
}

// customCoder holds the information for types with custom encoding and decoding
// methods - e.g. VomEncode / VomDecode.
type customCoder struct {
	rti        *rtInfo       // rtInfo for custom coder argument
	encPtrRcvr bool          // Encode defined on a pointer receiver?
	encFunc    reflect.Value // Encode function
	decFunc    reflect.Value // Decode function
	rcvrConv   Type          // Convert receiver to this type for calls.

	// The JSON coders have arg type []byte, but we convert to string so that the
	// regular JSON string logic is used for encoding and decoding.  MarshalJSON
	// has an additional requirement that it must produce a valid JSON value; we
	// only support JSON strings, which are surrounded by double-quotes.
	isJSON          bool // Is this a custom JSON coder?
	extraQuotesJSON bool // Does the arg have extra quotes?
}

func (coder *customCoder) convertEncodeArg(rcvr, arg Value) (Value, error) {
	if coder.isJSON {
		arg = arg.Convert(rtString)
		if coder.extraQuotesJSON {
			str := arg.String()
			len := len(str)
			if len < 2 || str[0] != '"' || str[len-1] != '"' {
				return nil, fmt.Errorf("vom: custom JSON Encode expected quotes but saw %s, type %q", str, rcvr.Type())
			}
			arg = ValueOf(str[1 : len-1])
		}
	}
	return arg, nil
}

func (coder *customCoder) convertDecodeArg(arg Value) Value {
	if coder.isJSON {
		if coder.extraQuotesJSON {
			arg = ValueOf(`"` + arg.String() + `"`)
		}
		arg = arg.Convert(rtByteSlice)
	}
	return arg
}

var (
	rtByte       = TypeOf(byte(0))
	rtByteSlice  = TypeOf([]byte{})
	rtString     = TypeOf(string(""))
	rtComplex64  = TypeOf(complex64(0))
	rtComplex128 = TypeOf(complex128(0))
	rtError      = TypeOf((*error)(nil)).Elem()
	rvZero       = Value(nil)
)

func (rti *rtInfo) shortNames() []string { return rti.snames }

func (rti *rtInfo) numCustomStars(format Format) int {
	switch format {
	case FormatBinary:
		return rti.numStarsBinary
	case FormatJSON:
		return rti.numStarsJSON
	}
	return 0
}

// callCustomEncode calls the custom coder method on rcvr, if it exists.
// Returns the rtInfo and reflect.Value that should be used for encoding.
func (rti *rtInfo) callCustomEncode(rcvr Value, format Format) (*rtInfo, Value, error) {
	var coder *customCoder
	switch format {
	case FormatBinary:
		coder = rti.customBinary
	case FormatJSON:
		coder = rti.customJSON
	}
	if coder == nil {
		// No custom coder, this is just the identity function.
		return rti, rcvr, nil
	}
	// Call the custom coder method to get the actual value to encode.
	if coder.encPtrRcvr {
		if !rcvr.CanAddr() {
			return nil, rvZero, fmt.Errorf("vom: custom Encode defined with pointer receiver, but base value encoded, type %q", rcvr.Type())
		}
		rcvr = rcvr.Addr()
	}
	if coder.rcvrConv != nil {
		rcvr = rcvr.Convert(coder.rcvrConv)
	}
	rvv, err := ToReflectValue(rcvr)
	if err != nil {
		return nil, rvZero, err
	}
	oargs := coder.encFunc.Call([]reflect.Value{rvv})
	if ierr := oargs[1].Interface(); ierr != nil {
		return nil, rvZero, ierr.(error)
	}
	result, err := coder.convertEncodeArg(rcvr, ToVomValue(oargs[0]))
	if err != nil {
		return nil, rvZero, err
	}
	return coder.rti, result, nil
}

// initCustomDecode prepares for calling a custom decoder method, if it exists.
// Returns the rtInfo and reflect.Value that should be used for decoding.  If
// the returned customDecode is nil there's no custom decoder method, otherwise
// customDecode.call() must be called after regular decoding to call the custom
// decoder.
func (rti *rtInfo) initCustomDecode(rcvr Value, format Format) (*rtInfo, Value, *customDecode) {
	var coder *customCoder
	switch format {
	case FormatBinary:
		coder = rti.customBinary
	case FormatJSON:
		coder = rti.customJSON
	}

	if coder == nil {
		// No custom coder, this is just the identity function.
		return rti, rcvr, nil
	}
	// The type has a custom coder method, so set up the extra state.
	dec := &customDecode{coder, rcvr, New(coder.rti.rt).Elem()}
	return coder.rti, dec.arg, dec
}

// customDecode holds the state for calling a custom decode method.
type customDecode struct {
	coder     *customCoder
	rcvr, arg Value
}

// Call the custom decoder method on rcvr with arg.
func (dec *customDecode) call() error {
	rcvr := dec.rcvr
	arg := dec.arg

	if !rcvr.CanAddr() {
		return fmt.Errorf("vom: can't take address of custom Decode receiver, type %q", rcvr.Type())
	}

	rcvr = rcvr.Addr()
	if dec.coder.rcvrConv != nil {
		rcvr = rcvr.Convert(PtrTo(dec.coder.rcvrConv))
	}

	cvv, err := ToReflectValue(rcvr)
	if err != nil {
		return err
	}

	arg = dec.coder.convertDecodeArg(arg)
	avv, err := ToReflectValue(arg)
	if err != nil {
		return err
	}
	if ierr := dec.coder.decFunc.Call([]reflect.Value{cvv, avv})[0].Interface(); ierr != nil {
		return ierr.(error)
	}
	return nil
}

func (rti *rtInfo) fieldByName(name string) (int, *rtInfo) {
	if rti.fieldsByName != nil {
		if field, ok := rti.fieldsByName[name]; ok {
			return field.index, field.info
		}
	}
	return -1, nil
}

var (
	rtInfoMutex sync.RWMutex
	rtInfoReg   = map[Type]*rtInfo{}
)

// lookupRTInfo returns the hash-consed rtInfo corresponding to rt.  The
// hash-consing guarantees that each unique type will map to a single *rtInfo
// that may be compared for pointer equality, e.g. used as a the key in a map.
// The same mechanism also gives us fast repeated access.
func lookupRTInfo(rt Type) (*rtInfo, error) {
	// Fastpath lookup in our registry.
	rtInfoMutex.RLock()
	rti, ok := rtInfoReg[rt]
	rtInfoMutex.RUnlock()
	if ok {
		return rti, nil
	}
	// Not found, need an update.  This is racy, but that's fine - multiple
	// goroutines may attempt to update the map, and the first one will win, while
	// the rest of them will just use the previously updated value.
	rtInfoMutex.Lock()
	defer rtInfoMutex.Unlock()
	return lookupRTInfoLocked(rt)
}

func lookupRTInfoLocked(rt Type) (*rtInfo, error) {
	// Already in our registry.  This handles races, and also terminates lookups
	// for recursive types.
	if rti, ok := rtInfoReg[rt]; ok {
		return rti, nil
	}
	// Create a new rtInfo and add it to our registry.  We must add a new entry
	// into rtInfoReg before lookupRTIInfoLocked is called recursively, to ensure
	// recursive types will find the correct pointer, which will be updated as the
	// recursive stack is unwound.
	rti := new(rtInfo)
	rtInfoReg[rt] = rti
	if err := fillRTInfoLocked(rt, rti); err != nil {
		// Remove bad rti from registry to ensure future lookups can't use it.
		delete(rtInfoReg, rt)
		return nil, err
	}
	// Fill in common fields.
	rti.rt = rt
	rti.zero = Zero(rt)
	rti.name, rti.isNamed = typeName(rt)
	if rti.isNamed {
		rti.snames = computeShortTypeNames(rti.name)
	}
	rti.numMethods = rt.NumMethod()
	return rti, nil
}

func fillRTInfoLocked(rt Type, rti *rtInfo) error {
	var err error
	// Pointers are handled first to simplify the checking for custom coders.
	if rt.Kind() == reflect.Ptr {
		rti.kind = typeKindPtr
		if rti.elem, err = lookupRTInfoLocked(rt.Elem()); err != nil {
			return err
		}
		// Pointer kinds track both the total number of stars until a non-pointer
		// type is reached, and the number of stars on custom coder types.  This
		// pre-computation lets us easily deal with mixed numbers of stars in
		// decoding, via alignStars.
		rti.numStars = 1 + rti.elem.numStars
		rti.numStarsBinary = rti.elem.numStarsBinary
		rti.numStarsJSON = rti.elem.numStarsJSON
		return nil
	}
	var rtt reflect.Type
	rtt, err = ToReflectType(rt)
	if err == nil {
		// Check for custom coder methods.
		if rti.customBinary, err = newCustomCoderLocked(rtt, encNamesBinary, decNamesBinary); err != nil {
			return err
		}
		if rti.customJSON, err = newCustomCoderLocked(rtt, encNamesJSON, decNamesJSON); err != nil {
			return err
		}
	}
	if rti.customBinary != nil {
		rti.numStarsBinary = rti.customBinary.rti.numStars
	}
	if rti.customJSON != nil {
		rti.numStarsJSON = rti.customJSON.rti.numStars
	}
	if rti.customBinary != nil && rti.customJSON != nil {
		// If we have custom coders for both binary and JSON, we shouldn't fill in
		// the rest of the regular rti.  This is necessary for correctness - e.g. if
		// rt is otherwise invalid for vom (e.g. no exported struct fields, contains
		// functions or channels, etc), we don't want to return an error.
		rti.kind = typeKindCustom
		return nil
	}
	// Either there are no custom coders, or they're only defined for one of the
	// two formats.  Fill in the rti as usual.  A common user error is for rt to
	// be invalid for vom, but for a custom coder to be defined only for one of
	// the formats; we annotate the error to make this easier to understand.
	if err := fillRTInfoHelperLocked(rt, rti); err != nil {
		if rti.customBinary != nil {
			err = fmt.Errorf("%s - perhaps add JSON custom coder methods, e.g. MarshalJSON / UnmarshalJSON", err.Error())
		}
		if rti.customJSON != nil {
			err = fmt.Errorf("%s - perhaps add binary custom coder methods, e.g. MarshalBinary / UnmarshalBinary", err.Error())
		}
		return err
	}
	return nil
}

func fillRTInfoHelperLocked(rt Type, rti *rtInfo) error {
	var err error

	// Handle primitive types.
	if p, ok := primitiveKindMap[rt.Kind()]; ok {
		rti.kind = p.kind
		rti.id = p.id
		rti.suffix = p.suffix
		return nil
	}

	// Now handle composite types.
	switch rt.Kind() {
	case reflect.Array:
		rti.kind = typeKindArray
		rti.len = rt.Len()
		if rti.elem, err = lookupRTInfoLocked(rt.Elem()); err != nil {
			return err
		}
		return nil
	case reflect.Slice:
		if isByteSlice(rt) {
			// Byte slices have their own type id.
			rti.kind = typeKindByteSlice
			rti.id = wiretype.TypeIDByteSlice
			if rti.elem, err = lookupRTInfoLocked(rt.Elem()); err != nil {
				return err
			}
			return nil
		}
		rti.kind = typeKindSlice
		if rti.elem, err = lookupRTInfoLocked(rt.Elem()); err != nil {
			return err
		}
		return nil
	case reflect.Map:
		rti.kind = typeKindMap
		if rti.key, err = lookupRTInfoLocked(rt.Key()); err != nil {
			return err
		}
		if rti.elem, err = lookupRTInfoLocked(rt.Elem()); err != nil {
			return err
		}
		return nil
	case reflect.Struct:
		rti.kind = typeKindStruct
		rti.fieldsByName = make(map[string]*rtInfoField)
		for fx := 0; fx < rt.NumField(); fx++ {
			rtField := rt.Field(fx)
			if !isFieldExported(rtField) {
				continue
			}
			info, err := lookupRTInfoLocked(rtField.Type)
			if err != nil {
				return err
			}
			rtf := &rtInfoField{rtField.Name, fx, info}
			rti.fields = append(rti.fields, rtf)
			rti.fieldsByName[rtField.Name] = rtf
		}
		if len(rti.fields) == 0 && rt.NumField() > 0 {
			return fmt.Errorf("vom: struct %q has no exported fields", rt)
		}
		return nil
	}

	// Encode named and unnamed complex as if it were struct{R float; I float}.
	// We tag these types specially so that Go can distinguish the types when
	// performing generic decoding into a nil interface.  Marshalers in other
	// languages that don't have special support for these types can still
	// understand them, although they might not treat them specially.
	switch rt.Kind() {
	case reflect.Complex64:
		rti.kind = typeKindCustom
		custom := &customCoder{
			encFunc:  rvVomEncodeComplex64,
			decFunc:  rvVomDecodeComplex64,
			rcvrConv: rtComplex64,
		}
		if custom.rti, err = lookupRTInfoLocked(rtWrapComplex64); err != nil {
			return err
		}
		custom.rti.name = "complex64"
		custom.rti.snames = nil
		if !containsTag(rti.tags, "complex64") {
			rti.tags = append(rti.tags, "complex64")
		}
		if !containsTag(custom.rti.tags, "complex64") {
			custom.rti.tags = append(custom.rti.tags, "complex64")
		}
		rti.customBinary = custom
		rti.customJSON = custom
		return nil
	case reflect.Complex128:
		rti.kind = typeKindCustom
		custom := &customCoder{
			encFunc:  rvVomEncodeComplex128,
			decFunc:  rvVomDecodeComplex128,
			rcvrConv: rtComplex128,
		}
		if custom.rti, err = lookupRTInfoLocked(rtWrapComplex128); err != nil {
			return err
		}
		custom.rti.name = "complex128"
		custom.rti.snames = nil
		if !containsTag(rti.tags, "complex128") {
			rti.tags = append(rti.tags, "complex128")
		}
		if !containsTag(custom.rti.tags, "complex128") {
			custom.rti.tags = append(custom.rti.tags, "complex128")
		}
		rti.customBinary = custom
		rti.customJSON = custom
		return nil
	}

	// The remaining kinds are Invalid, Func, Chan and UnsafePointer.
	return fmt.Errorf("vom: unsupported kind %q type %q", rt.Kind(), rt)
}

type wrapComplex64 struct {
	R float32
	I float32
}

func vomEncodeComplex64(c complex64) (wrapComplex64, error) {
	return wrapComplex64{real(c), imag(c)}, nil
}
func vomDecodeComplex64(c *complex64, w wrapComplex64) error {
	*c = complex(w.R, w.I)
	return nil
}

type wrapComplex128 struct {
	R float64
	I float64
}

func vomEncodeComplex128(c complex128) (wrapComplex128, error) {
	return wrapComplex128{real(c), imag(c)}, nil
}
func vomDecodeComplex128(c *complex128, w wrapComplex128) error {
	*c = complex(w.R, w.I)
	return nil
}

var (
	rtWrapComplex64       = TypeOf(wrapComplex64{})
	rtWrapComplex128      = TypeOf(wrapComplex128{})
	rvVomEncodeComplex64  = reflect.ValueOf(vomEncodeComplex64)
	rvVomDecodeComplex64  = reflect.ValueOf(vomDecodeComplex64)
	rvVomEncodeComplex128 = reflect.ValueOf(vomEncodeComplex128)
	rvVomDecodeComplex128 = reflect.ValueOf(vomDecodeComplex128)

	// Names of the supported custom coder methods.  We always look for matching
	// pairs of encoder / decoder names, and return the first pair we find.
	encNamesBinary = []string{"VomEncode", "GobEncode", "MarshalBinary"}
	decNamesBinary = []string{"VomDecode", "GobDecode", "UnmarshalBinary"}
	encNamesJSON   = []string{"VomEncode", "MarshalJSON", "MarshalText"}
	decNamesJSON   = []string{"VomDecode", "UnmarshalJSON", "UnmarshalText"}
)

// newCustomCoderLocked returns a new custom coder representing a type with
// custom coding methods (e.g. VomEncode, VomDecode).  Returns nil if no custom
// coding methods are defined.
func newCustomCoderLocked(rt reflect.Type, encNames, decNames []string) (*customCoder, error) {
	// Look for matching custom encoder and decoder methods.  The first pair we
	// find is used.  Note that typically gob, json, etc. will silently ignore the
	// methods if they have the right name but the wrong signature, and allow only
	// one of the two to be defined.  We return errors in these cases to ease
	// debugging of common mistakes.
	var encName, decName string
	var enc, dec *reflect.Method
	var encPtrRcvr bool
	var err error
	for ix := 0; ix < len(encNames); ix++ {
		encName = encNames[ix]
		decName = decNames[ix]
		if enc, encPtrRcvr, err = customEncodeLocked(rt, encName); err != nil {
			return nil, err
		}
		if dec, err = customDecodeLocked(rt, decName); err != nil {
			return nil, err
		}
		if (enc == nil) != (dec == nil) {
			return nil, fmt.Errorf("vom: %s and %s must both be defined or both be undefined, type %q", encName, decName, rt)
		}
		if enc != nil {
			break // Both methods are defined.
		}
	}
	if enc == nil {
		return nil, nil // None of the methods are defined.
	}
	encType := ToVomType(enc.Type.Out(0))
	decType := ToVomType(dec.Type.In(1))
	if encType != decType {
		return nil, fmt.Errorf("vom: %s uses arg type %q, but %s uses arg type %q", encName, encType, decType, decType)
	}
	if encName != "VomEncode" && encType != rtByteSlice {
		return nil, fmt.Errorf("vom: %s and %s must use arg type []byte, but defined with arg %q", encName, decName, encType)
	}
	var rti *rtInfo
	var isJSON, extraQuotesJSON bool
	if encName == "MarshalJSON" || encName == "MarshalText" {
		if rti, err = lookupRTInfoLocked(rtString); err != nil {
			return nil, err
		}
		isJSON = true
		extraQuotesJSON = (encName == "MarshalJSON")
	} else {
		if rti, err = lookupRTInfoLocked(encType); err != nil {
			return nil, err
		}
	}
	return &customCoder{
		rti:             rti,
		encPtrRcvr:      encPtrRcvr,
		encFunc:         enc.Func,
		decFunc:         dec.Func,
		isJSON:          isJSON,
		extraQuotesJSON: extraQuotesJSON,
	}, nil
}

// The VomEncode method might have been defined as taking either a non-pointer
// receiver or a pointer receiver.  E.g.
//   type Foo struct { A int }
//   func (f Foo) VomEncode() (string, error) {...}
//
//   type FooPtr struct { A int }
//   func (f *FooPtr) VomEncode() (string, error) {...}
//
// The first MethodByName call below will return a valid method for the Foo case
// if rt is either Foo or *Foo.  For the FooPtr case we only get a valid method
// if rt is *FooPtr.
//
// Note that GobEncode / Marshal{JSON,Text,Binary} are the same, but are
// restricted to a []byte arg.
func customEncodeLocked(rt reflect.Type, name string) (*reflect.Method, bool, error) {
	encPtrRcvr := false
	encFunc, ok := rt.MethodByName(name)
	if !ok {
		if encFunc, ok = reflect.PtrTo(rt).MethodByName(name); !ok {
			// No VomEncode method is defined.
			return nil, false, nil
		}
		encPtrRcvr = true
	}
	// Recall that in-arg 0 is the receiver.
	if encFunc.Type.NumIn() != 1 {
		return nil, false, fmt.Errorf("vom: %s must have no in-args, type %q", name, rt)
	}
	if encFunc.Type.NumOut() != 2 {
		return nil, false, fmt.Errorf("vom: %s must have two out-args, type %q", name, rt)
	}
	if newVReflectType(encFunc.Type.Out(1)) != rtError {
		return nil, false, fmt.Errorf("vom: %s last out-arg must be \"error\", type %q", name, rt)
	}
	// Don't allow the vom type to itself have a VomEncode method.  There's no
	// technical reason to disallow it, but it's simpler this way.
	rtVom := encFunc.Type.Out(0)
	for rtVom.Kind() == reflect.Ptr {
		rtVom = rtVom.Elem()
	}
	_, hasEncode1 := rtVom.MethodByName(name)
	_, hasEncode2 := reflect.PtrTo(rtVom).MethodByName(name)
	if hasEncode1 || hasEncode2 {
		return nil, false, fmt.Errorf("vom: %s arg can't itself have %s method, type %q, arg type %q", name, name, rt, rtVom)
	}
	return &encFunc, encPtrRcvr, nil
}

// The VomDecode method must be defined as taking a pointer receiver.  E.g.
//   type Foo struct { A int }
//   func (f *Foo) VomDecode(s string) error {...}
//
// Note that GobDecode / Unmarshal{JSON,Text,Binary} are the same, but are
// restricted to a []byte arg.
func customDecodeLocked(rt reflect.Type, name string) (*reflect.Method, error) {
	if _, ok := rt.MethodByName(name); ok {
		return nil, fmt.Errorf("vom: %s can't be defined on non-pointer receiver, type %q", name, rt)
	}
	decFunc, ok := reflect.PtrTo(rt).MethodByName(name)
	if !ok {
		return nil, nil
	}
	// Recall that in-arg 0 is the receiver.
	if decFunc.Type.NumIn() != 2 {
		return nil, fmt.Errorf("vom: %s must have one in-arg, type %q", name, rt)
	}
	if decFunc.Type.NumOut() != 1 {
		return nil, fmt.Errorf("vom: %s must have one out-arg, type %q", name, rt)
	}
	if newVReflectType(decFunc.Type.Out(0)) != rtError {
		return nil, fmt.Errorf("vom: %s out-arg must be \"error\", type %q", name, rt)
	}
	// Don't allow the vom type to itself have a VomDecode method.  There's no
	// technical reason to disallow it, but it's simpler this way.
	rtVom := decFunc.Type.In(1)
	for rtVom.Kind() == reflect.Ptr {
		rtVom = rtVom.Elem()
	}
	_, hasDecode1 := rtVom.MethodByName(name)
	_, hasDecode2 := reflect.PtrTo(rtVom).MethodByName(name)
	if hasDecode1 || hasDecode2 {
		return nil, fmt.Errorf("vom: %s arg can't itself have %s method, type %q, arg type %q", name, name, rt, rtVom)
	}
	return &decFunc, nil
}

var primitiveKindMap = map[reflect.Kind]struct {
	kind   typeKind
	id     wiretype.TypeID
	suffix string
}{
	reflect.Interface: {typeKindInterface, wiretype.TypeIDInterface, ""},
	reflect.Bool:      {typeKindBool, wiretype.TypeIDBool, ""},
	reflect.String:    {typeKindString, wiretype.TypeIDString, ""},
	reflect.Uint:      {typeKindUint, wiretype.TypeIDUint, ""},
	reflect.Uint8:     {typeKindByte, wiretype.TypeIDUint8, "8"},
	reflect.Uint16:    {typeKindUint, wiretype.TypeIDUint16, "16"},
	reflect.Uint32:    {typeKindUint, wiretype.TypeIDUint32, "32"},
	reflect.Uint64:    {typeKindUint, wiretype.TypeIDUint64, "64"},
	reflect.Uintptr:   {typeKindUint, wiretype.TypeIDUintptr, "ptr"},
	reflect.Int:       {typeKindInt, wiretype.TypeIDInt, ""},
	reflect.Int8:      {typeKindInt, wiretype.TypeIDInt8, "8"},
	reflect.Int16:     {typeKindInt, wiretype.TypeIDInt16, "16"},
	reflect.Int32:     {typeKindInt, wiretype.TypeIDInt32, "32"},
	reflect.Int64:     {typeKindInt, wiretype.TypeIDInt64, "64"},
	reflect.Float32:   {typeKindFloat, wiretype.TypeIDFloat32, "32"},
	reflect.Float64:   {typeKindFloat, wiretype.TypeIDFloat64, "64"},
}
