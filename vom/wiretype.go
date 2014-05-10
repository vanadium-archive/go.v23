package vom

import (
	"fmt"
	"reflect"
	"strings"
	"sync"

	"veyron2/wiretype"
)

func floatBitSize(id wiretype.TypeID) int {
	switch id {
	case wiretype.TypeIDFloat32:
		return 32
	case wiretype.TypeIDFloat64:
		return 64
	default:
		panic(fmt.Errorf("vom: floatBitSize unhandled id %d", id))
	}
}

// typeKind describes the different kinds of encoded values that we support;
// i.e. we partition the type id space into these equivalence classes.  Each
// coder implementation must support all of these kinds.  It's never stored
// persistently, so it's safe to change the actual values.
type typeKind int

const (
	typeKindInvalid = iota
	typeKindCustom
	typeKindInterface
	typeKindBool
	typeKindString
	typeKindByteSlice
	typeKindByte
	typeKindUint
	typeKindInt
	typeKindFloat
	typeKindSlice
	typeKindArray
	typeKindMap
	typeKindStruct
	typeKindPtr
)

func (k typeKind) String() string {
	switch k {
	case typeKindInvalid:
		return "Invalid"
	case typeKindCustom:
		return "Custom"
	case typeKindInterface:
		return "Interface"
	case typeKindBool:
		return "Bool"
	case typeKindString:
		return "String"
	case typeKindByteSlice:
		return "ByteSlice"
	case typeKindByte:
		return "Byte"
	case typeKindUint:
		return "Uint"
	case typeKindInt:
		return "Int"
	case typeKindFloat:
		return "Float"
	case typeKindSlice:
		return "Slice"
	case typeKindArray:
		return "Array"
	case typeKindMap:
		return "Map"
	case typeKindStruct:
		return "Struct"
	case typeKindPtr:
		return "Ptr"
	default:
		return fmt.Sprintf("UnhandledKind(%d)", k)
	}
}

var (
	iface       interface{}
	rtInterface = TypeOf(&iface).Elem()
	rtInfoInt64 = bootRTInfo(int64(0))

	primitiveTypeIDs = map[wiretype.TypeID]bool{
		wiretype.TypeIDBool:      true,
		wiretype.TypeIDString:    true,
		wiretype.TypeIDByteSlice: true,
		wiretype.TypeIDUint:      true,
		wiretype.TypeIDUint8:     true,
		wiretype.TypeIDUint16:    true,
		wiretype.TypeIDUint32:    true,
		wiretype.TypeIDUint64:    true,
		wiretype.TypeIDUintptr:   true,
		wiretype.TypeIDInt:       true,
		wiretype.TypeIDInt8:      true,
		wiretype.TypeIDInt16:     true,
		wiretype.TypeIDInt32:     true,
		wiretype.TypeIDInt64:     true,
		wiretype.TypeIDFloat32:   true,
		wiretype.TypeIDFloat64:   true,
	}

	bootstrapTypeIDs = map[*rtInfo]wiretype.TypeID{
		bootRTInfo(nil):                           wiretype.TypeIDInterface,
		bootRTInfo(false):                         wiretype.TypeIDBool,
		bootRTInfo(string("")):                    wiretype.TypeIDString,
		bootRTInfo([]byte{}):                      wiretype.TypeIDByteSlice,
		bootRTInfo(uint(0)):                       wiretype.TypeIDUint,
		bootRTInfo(uint8(0)):                      wiretype.TypeIDUint8,
		bootRTInfo(uint16(0)):                     wiretype.TypeIDUint16,
		bootRTInfo(uint32(0)):                     wiretype.TypeIDUint32,
		bootRTInfo(uint64(0)):                     wiretype.TypeIDUint64,
		bootRTInfo(uintptr(0)):                    wiretype.TypeIDUintptr,
		bootRTInfo(int(0)):                        wiretype.TypeIDInt,
		bootRTInfo(int8(0)):                       wiretype.TypeIDInt8,
		bootRTInfo(int16(0)):                      wiretype.TypeIDInt16,
		bootRTInfo(int32(0)):                      wiretype.TypeIDInt32,
		rtInfoInt64:                               wiretype.TypeIDInt64,
		bootRTInfo(float32(0)):                    wiretype.TypeIDFloat32,
		bootRTInfo(float64(0)):                    wiretype.TypeIDFloat64,
		bootRTInfo(wiretype.NamedPrimitiveType{}): wiretype.TypeIDNamedPrimitiveType,
		bootRTInfo(wiretype.SliceType{}):          wiretype.TypeIDSliceType,
		bootRTInfo(wiretype.ArrayType{}):          wiretype.TypeIDArrayType,
		bootRTInfo(wiretype.MapType{}):            wiretype.TypeIDMapType,
		bootRTInfo(wiretype.StructType{}):         wiretype.TypeIDStructType,
		bootRTInfo(wiretype.PtrType{}):            wiretype.TypeIDPtrType,
	}
)

func bootRTInfo(v interface{}) (rti *rtInfo) {
	var err error
	if v == nil {
		rti, err = lookupRTInfo(rtInterface)
	} else {
		rti, err = lookupRTInfo(TypeOf(v))
	}
	if err != nil {
		panic(fmt.Errorf("vom: boostrap error %v", err))
	}
	return
}

// Register records a type, identified by a value for that type.  The type name
// is based on the package path, e.g. "veyron/lib/vom.StructType".  Type
// registration is only necessary when performing generic decoding into nil
// interface values.  Both interface and concrete types may be registered, and
// inner types of composite types are automatically registered.
// TODO(bprosnitz) Move to a separate package -- this forces many packages to depend on VOM.
func Register(v interface{}) {
	if v == nil {
		return
	}
	registerRecursive(reflect.TypeOf(v), make(map[reflect.Type]bool))
}

func registerRecursive(rt reflect.Type, seen map[reflect.Type]bool) {
	if seen[rt] {
		return // Break infinite loops from recursive types.
	}
	seen[rt] = true

	// Register the inner types of composite types.
	switch rt.Kind() {
	case reflect.Array:
		registerRecursive(rt.Elem(), seen)
	case reflect.Slice:
		registerRecursive(rt.Elem(), seen)
	case reflect.Map:
		registerRecursive(rt.Key(), seen)
		registerRecursive(rt.Elem(), seen)
	case reflect.Struct:
		for fx := 0; fx < rt.NumField(); fx++ {
			registerRecursive(rt.Field(fx).Type, seen)
		}
	case reflect.Ptr:
		registerRecursive(rt.Elem(), seen)
	}
	name, isNamed := reflectTypeName(rt)
	if err := typeRegistry.register(nameInfo{name, isNamed}, rt); err != nil {
		panic(err)
	}
}

// typeReg maintains a bijective mapping between names and types.  It's used to
// ensure the generic decoder can recover the actual type sent over the wire;
// the Go reflect package doesn't let us generate named, array and struct types
// dynamically, so the type information must be registered.
type typeReg struct {
	sync.RWMutex
	toType map[string]reflect.Type
	toName map[Type]nameInfo
}

type nameInfo struct {
	name    string
	isNamed bool
}

var typeRegistry = typeReg{
	toType: make(map[string]reflect.Type),
	toName: make(map[Type]nameInfo),
}

func (reg *typeReg) register(info nameInfo, rt reflect.Type) error {
	if info.name == "" {
		return fmt.Errorf("vom: can't register with an empty name %q", rt)
	}
	reg.Lock()
	defer reg.Unlock()
	// Enforce a bijective mapping between names and types, but allow the same
	// name/rt pair to be registered multiple times.
	if exist, ok := reg.toType[info.name]; ok && exist != rt {
		return fmt.Errorf("vom: duplicate name %q registered under %q and %q", info.name, exist, rt)
	}
	vt := ToVomType(rt)
	if exist, ok := reg.toName[vt]; ok && exist != info {
		return fmt.Errorf("vom: duplicate type %q registered under %q and %q", rt, exist, info)
	}
	reg.toType[info.name] = rt
	reg.toName[vt] = info
	return nil
}

func (reg *typeReg) nameToType(name string) reflect.Type {
	reg.RLock()
	defer reg.RUnlock()
	return reg.toType[name]
}

func (reg *typeReg) typeToName(rt Type) nameInfo {
	reg.RLock()
	defer reg.RUnlock()
	return reg.toName[rt]
}

func reflectTypeName(rt reflect.Type) (string, bool) {
	return typeName(ToVomType(rt))
}

// typeName returns the type name of rt, and a bool telling us whether the type
// is named, or a built-in / unnamed type.
func typeName(rt Type) (string, bool) {
	// Fastpath check for an already-registered name.
	if info := typeRegistry.typeToName(rt); info.name != "" {
		return info.name, info.isNamed
	}
	// User-defined named types get their regular full name.
	if rt.Name() != "" && rt.PkgPath() != "" {
		return fullName(rt), true
	}
	// Special-case complex64 and complex128 to get their regular full name; they
	// aren't built-in to vom and are treated like named types.
	if rt.Kind() == reflect.Complex64 || rt.Kind() == reflect.Complex128 {
		return fullName(rt), true
	}
	// Built-in and unnamed types.
	return fullName(rt), false
}

// fullName computes the full name of rt, used for built-in and unnamed types.
// This code must be kept in-sync with wireDef.fullName.
// TODO(bprosnitz): This function can loop infinitely with certain arguments (recursive structures
// without names). Fix this.
func fullName(rt Type) string {
	if rt.Name() != "" {
		if rt.PkgPath() != "" {
			// The name is "pkgpath.name", e.g. "veyron/lib/vom.MapType"
			return rt.PkgPath() + "." + rt.Name()
		}
		// The only way to have a name but no pkgpath is for built-in types,
		// e.g. int32, string, etc.
		if rt.Kind() == reflect.Uint8 {
			return "byte" // The name of uint8 is byte.
		}
		return rt.Name()
	}
	switch rt.Kind() {
	case reflect.Array:
		return fmt.Sprintf("[%d]%s", rt.Len(), fullName(rt.Elem()))
	case reflect.Slice:
		return "[]" + fullName(rt.Elem())
	case reflect.Map:
		return "map[" + fullName(rt.Key()) + "]" + fullName(rt.Elem())
	case reflect.Struct:
		name := "struct{"
		isFirst := true
		for fx := 0; fx < rt.NumField(); fx++ {
			field := rt.Field(fx)
			if !isFieldExported(field) {
				continue
			}
			if !isFirst {
				name += ";"
			}
			name += field.Name + " " + fullName(field.Type)
			isFirst = false
		}
		return name + "}"
	case reflect.Ptr:
		return "*" + fullName(rt.Elem())
	case reflect.Interface:
		return "interface"
	default:
		panic(fmt.Errorf("vom: can't take fullName of kind %v type %q", rt.Kind(), rt))
	}
}

// isFieldExported returns true iff field is exported (i.e. first letter caps).
func isFieldExported(field StructField) bool {
	// An empty pkgpath means the field is exported; See
	// http://golang.org/pkg/reflect/#StructField for details.
	return field.PkgPath == ""
}

// computeShortTypeNames is given a fullname as generated by fullName above, and
// returns a slice of shortnames.  Some examples:
//   "veyron/lib/vom.Type" -> ["Type", "vom.Type", "veyron/lib/vom.Type"]
//   "veyron.Type"         -> ["Type", "veyron.Type"]
//   "Type"                -> ["Type"]
//
// Note that we generate at most three short names; we don't return
// "lib/vom.Type" in the first example.  This seems to strike a good balance
// between compactness and ease of understanding.
func computeShortTypeNames(fullname string) (ret []string) {
	lastDot := strings.LastIndex(fullname, ".")
	ret = append(ret, fullname[lastDot+1:])
	if lastDot == -1 {
		return
	}
	lastSlash := strings.LastIndex(fullname, "/")
	ret = append(ret, fullname[lastSlash+1:])
	if lastSlash == -1 {
		return
	}
	ret = append(ret, fullname)
	return
}

// shortNamer allows us to use nameResolver for both encoding and decoding.
type shortNamer interface {
	// shortNames returns a slice of short names, ordered from shortest to longest
	// name.  The last item in the slice must be the full name.  We allow the
	// slice to be empty, signifying an unnamed type.
	shortNames() []string
}

// nameResolver keeps track of type information and names, and is used during
// both JSON encoding and decoding.  During encoding we use rtInfo as the
// shortNamer; during decoding we use wireDef as the shortNamer.
//
// The idea is it keeps naming information for each type, and detects collisions
// between short names.  You can ask it for the shortest name for a given type,
// or perform lookups of a type by an unambiguous shortening of its name.
//
// TODO(toddw): Deal with unavoidable collisions, e.g. "a/b.T" and "b.T"
type nameResolver struct {
	// toShortest maps from shortNamer to its shortest unambiguous name.
	toShortest map[shortNamer]string

	// fromName maps unambiguous names to their corresponding shortNamer.  We
	// distinguish nil values from an entry not existing in the map; a nil value
	// means that we've seen collisions on that particular short name, while a
	// non-existent entry means that the short name doesn't exist.
	fromName map[string]*shortNamer
}

func newNameResolver() *nameResolver {
	return &nameResolver{
		toShortest: make(map[shortNamer]string),
		fromName:   make(map[string]*shortNamer),
	}
}

func (nr *nameResolver) String() string {
	return fmt.Sprintf("to: %v from: %v", nr.toShortest, nr.fromName)
}

// insert inserts sn into the resolver under its full name.  To enable
// resolution of short names enableShortName must be called.
func (nr *nameResolver) insert(sn shortNamer) {
	if nr.contains(sn) {
		panic(fmt.Errorf("vom: nameResolver duplicate insert %#v", sn))
	}
	shortNames := sn.shortNames()
	len := len(shortNames)
	if len == 0 {
		// If there are no names we add sn to toShortest with an empty name, so that
		// contains will still be able to find it.
		nr.toShortest[sn] = ""
		return
	}
	// Otherwise the last item in shortNames is the full name.  Update our maps as
	// if the full name were the only name, so that lookups work on that.
	fullname := shortNames[len-1]
	if exist := nr.addName(fullname, sn); exist != nil {
		panic(fmt.Errorf("vom: duplicate name %v between\n%#v\nand\n%#v", fullname, sn, *exist))
	}
	nr.toShortest[sn] = fullname
}

// enableShortNames enables short names for sn, updating all other shortNamers
// that might have colliding short names to use unambiguous names.
func (nr *nameResolver) enableShortNames(sn shortNamer) {
	if !nr.contains(sn) {
		panic(fmt.Errorf("vom: unknown shortNamer %#v", sn))
	}
	// Update our maps and loop through all shortNamers that need their short name
	// updated, due to collisions.
	collisions := nr.updateCollisions(sn)
outer:
	for coll, _ := range collisions {
		shortNames := coll.shortNames()
		for _, short := range shortNames {
			if exist := nr.fromName[short]; exist != nil && *exist == coll {
				nr.toShortest[coll] = short
				continue outer
			}
		}
		panic(fmt.Errorf("vom: duplicate name %v, map %v", shortNames, nr.fromName))
	}
}

func (nr *nameResolver) updateCollisions(sn shortNamer) map[shortNamer]bool {
	shortNames := sn.shortNames()
	len := len(shortNames)
	if len == 0 || len == 1 {
		// Nothing to do - everything was already done in insert.
		return nil
	}

	colls := map[shortNamer]bool{sn: true}
	// Skip iteration of the last item (full name); already added in insert.
	for ix := 0; ix < len-1; ix++ {
		if c := nr.addName(shortNames[ix], sn); c != nil {
			colls[*c] = true
		}
	}
	return colls
}

func (nr *nameResolver) addName(name string, sn shortNamer) *shortNamer {
	if name == "" {
		panic(fmt.Errorf("vom: invalid empty name: %v", sn.shortNames()))
	}
	exist, ok := nr.fromName[name]
	if !ok {
		// First time name has been added - add sn.
		nr.fromName[name] = &sn
		return nil
	}
	// Name has collisions - update fromName to nil as a reminder.  This way we
	// only need to fix-up the first collision we see; thereafter we won't add the
	// colliding item at all.
	nr.fromName[name] = nil
	return exist
}

// toShortestName returns the shortest unambiguous name associated with sn.
func (nr *nameResolver) toShortestName(sn shortNamer) string {
	return nr.toShortest[sn]
}

// resolveName returns the shortNamer associated with name, which may be any
// unambiguous form of the name.
func (nr *nameResolver) resolveName(name string) shortNamer {
	if sn := nr.fromName[name]; sn != nil {
		return *sn
	}
	return nil
}

// contains returns true iff sn has been added via insert, regardless of whether
// sn has any names associated with it.
func (nr *nameResolver) contains(sn shortNamer) bool {
	_, ok := nr.toShortest[sn]
	return ok
}
