package vdl

// This file contains the non-trivial "rep" types, which are the representation
// of values of certain types.

// repBytes represents []byte and [N]byte values.  We special-case these kinds
// of types to easily support the Value.Bytes() and Value.CopyBytes() methods,
// which makes usage more convenient for the user.  This is also a more
// efficient representation.
type repBytes []byte

// allZeroBytes is a buffer containing all 0 bytes.  It's used for fast filling
// of 0 bytes after resizing.
var allZeroBytes = make(repBytes, 1024)

func copyRepBytes(rep repBytes) repBytes {
	cp := make(repBytes, len(rep))
	copy(cp, rep)
	return cp
}

func (rep repBytes) IsZero(t *Type) bool {
	switch t.kind {
	case List:
		return len(rep) == 0
	case Array:
		// TODO(toddw): Speed up the zero checking loop over each byte.
		for _, b := range rep {
			if b != 0 {
				return false
			}
		}
		return true
	}
	panic(t.errBytes("IsZero"))
}

// repMap represents set and map values.  There are two underlying
// representations depending on the key type.  We decide which one to use based
// on the type of the map key, and never change representations thereafter.
//   fast: Use a real Go map to associate keys with a kvPair
//   slow: Just a slice of kvPairs.
//
// Fast has the same O(1) amortized complexity as regular Go maps for insert and
// lookup, while slow is O(N).  Since fast uses a real Go map, it only works for
// maps whose key type is allowed in a real Go map; e.g. structs are implemented
// as a slice of fields, but slices aren't allowed as Go map keys, so that must
// use slow.
//
// The slowIndex is *[]kvPair rather than []kvPair because Value holds a repMap
// (and not a *repMap), and we need to update the slice.
//
// TODO(toddw): Slow is really slow; e.g. Equal is quadratic.  Speed this up.
type repMap struct {
	fastIndex map[interface{}]kvPair // maps with scalar keys use the fast rep
	slowIndex *[]kvPair              // everything else uses the slow rep
}

type kvPair struct {
	key, val *Value
}

func copyKVPair(kv kvPair) kvPair {
	return kvPair{CopyValue(kv.key), CopyValue(kv.val)}
}

func (kv kvPair) String() string {
	s := stringRep(kv.key.t, kv.key.rep)
	if kv.val != nil {
		s += ": " + stringRep(kv.val.t, kv.val.rep)
	}
	return s
}

func useFastIndex(key *Type) bool {
	// TODO(toddw): Structs with exactly 1 simple field may also use fast.
	switch key.kind {
	case Bool, Byte, Uint16, Uint32, Uint64, Int16, Int32, Int64, Float32, Float64, Complex64, Complex128, String, Enum:
		return true
	}
	if key.IsBytes() {
		return true
	}
	return false
}

// fastKeyRep returns a representation of key that may be used in a regular Go
// map.  It's only called on keys where useFastIndex returns true.
func fastKeyRep(key *Value) interface{} {
	if key.t.IsBytes() {
		return string(key.Bytes())
	}
	return key.rep
}

func zeroRepMap(key *Type) repMap {
	rep := repMap{}
	if useFastIndex(key) {
		rep.fastIndex = make(map[interface{}]kvPair)
	} else {
		slow := make([]kvPair, 0)
		rep.slowIndex = &slow
	}
	return rep
}

func copyRepMap(rep repMap) repMap {
	cp := repMap{}
	if rep.fastIndex != nil {
		cp.fastIndex = make(map[interface{}]kvPair, len(rep.fastIndex))
		for keyrep, kv := range rep.fastIndex {
			cp.fastIndex[keyrep] = copyKVPair(kv)
		}
	} else {
		slow := make([]kvPair, len(*rep.slowIndex))
		cp.slowIndex = &slow
		for index, kv := range *rep.slowIndex {
			(*cp.slowIndex)[index] = copyKVPair(kv)
		}
	}
	return cp
}

func equalRepMap(a, b repMap) bool {
	if a.Len() != b.Len() {
		return false
	}
	if a.fastIndex != nil {
		for _, akv := range a.fastIndex {
			if !b.hasKV(akv) {
				return false
			}
		}
	} else {
		for _, akv := range *a.slowIndex {
			if !b.hasKV(akv) {
				return false
			}
		}
	}
	return true
}

func (rep repMap) hasKV(kv kvPair) bool {
	val, ok := rep.Index(kv.key)
	if !ok {
		return false
	}
	return EqualValue(kv.val, val)
}

func (rep repMap) Len() int {
	if rep.fastIndex != nil {
		return len(rep.fastIndex)
	} else {
		return len(*rep.slowIndex)
	}
}

func (rep repMap) String() string {
	s := "{"
	if rep.fastIndex != nil {
		first := true
		for _, kv := range rep.fastIndex {
			if !first {
				s += ", "
			}
			s += kv.String()
			first = false
		}
	} else {
		for index, kv := range *rep.slowIndex {
			if index > 0 {
				s += ", "
			}
			s += kv.String()
		}
	}
	return s + "}"
}

func (rep repMap) Keys() []*Value {
	// TODO(toddw): Make returned keys immutable?
	if rep.fastIndex != nil {
		keys := make([]*Value, len(rep.fastIndex))
		index := 0
		for _, kv := range rep.fastIndex {
			keys[index] = kv.key
			index++
		}
		return keys
	} else {
		keys := make([]*Value, len(*rep.slowIndex))
		for index, kv := range *rep.slowIndex {
			keys[index] = kv.key
		}
		return keys
	}
}

func (rep repMap) Index(key *Value) (*Value, bool) {
	if rep.fastIndex != nil {
		if kv, ok := rep.fastIndex[fastKeyRep(key)]; ok {
			return kv.val, true
		}
	} else {
		for _, kv := range *rep.slowIndex {
			if EqualValue(kv.key, key) {
				return kv.val, true
			}
		}
	}
	return nil, false
}

func (rep repMap) Assign(key, elem *Value) {
	if rep.fastIndex != nil {
		rep.fastIndex[fastKeyRep(key)] = kvPair{key, elem}
	} else {
		for ix := 0; ix < len(*rep.slowIndex); ix++ {
			if EqualValue((*rep.slowIndex)[ix].key, key) {
				(*rep.slowIndex)[ix].val = elem
				return
			}
		}
		// The key doesn't exist in the slow index; append it.
		*rep.slowIndex = append(*rep.slowIndex, kvPair{key, elem})
	}
}

func (rep repMap) Delete(key *Value) {
	if rep.fastIndex != nil {
		delete(rep.fastIndex, fastKeyRep(key))
	} else {
		for ix := 0; ix < len(*rep.slowIndex); ix++ {
			if EqualValue((*rep.slowIndex)[ix].key, key) {
				// Delete entry ix by copying last entry into ix and shrinking by 1.
				last := len(*rep.slowIndex) - 1
				(*rep.slowIndex)[ix] = (*rep.slowIndex)[last]
				(*rep.slowIndex) = (*rep.slowIndex)[:last]
				return
			}
		}
	}
}

// repSequence represents the elements of a list and array, and the fields of a
// struct.  Each value is lazily initialized; the semantics dictate that nil is
// indistinguishable from zero values.
//
// Arrays and structs are initialized to their fixed-length, while lists are
// initialized to nil.
type repSequence []*Value

func copyRepSequence(rep repSequence) repSequence {
	if rep == nil {
		return nil
	}
	cp := make(repSequence, len(rep))
	for index, val := range rep {
		cp[index] = CopyValue(val)
	}
	return cp
}

func equalRepSequence(a, b repSequence) bool {
	if len(a) != len(b) {
		return false
	}
	for index, aval := range a {
		// Handle cases where we're using nil to represent zero values.
		switch bval := b[index]; {
		case aval == nil && bval != nil:
			if !bval.IsZero() {
				return false
			}
		case aval != nil && bval == nil:
			if !aval.IsZero() {
				return false
			}
		case !EqualValue(aval, bval):
			return false
		}
	}
	return true
}

func (rep repSequence) AllValuesZero() bool {
	for _, val := range rep {
		if val != nil && !val.IsZero() {
			return false
		}
	}
	return true
}

func (rep repSequence) String(t *Type) string {
	s := "{"
	for index, val := range rep {
		if index > 0 {
			s += ", "
		}
		valtype := t.elem
		if t.Kind() == Struct {
			valtype = t.fields[index].Type
			s += t.fields[index].Name
			s += ": "
		}
		if val == nil {
			s += stringRep(valtype, zeroRep(valtype))
		} else {
			s += stringRep(val.t, val.rep)
		}
	}
	return s + "}"
}

func (rep repSequence) Index(t *Type, index int) *Value {
	if oldval := rep[index]; oldval != nil {
		return oldval
	}
	// Lazy initialization; create the new value upon first access.
	newval := ZeroValue(t)
	rep[index] = newval
	return newval
}

// repOneOf represents a OneOf value, which includes the field index of its
// type, and the underlying elem.
type repOneOf struct {
	index int
	value *Value
}

func copyRepOneOf(rep repOneOf) repOneOf {
	return repOneOf{rep.index, CopyValue(rep.value)}
}

func equalRepOneOf(a, b repOneOf) bool {
	return a.index == b.index && EqualValue(a.value, b.value)
}

func (rep repOneOf) IsZero() bool {
	return rep.index == 0 && rep.value.IsZero()
}

func (rep repOneOf) String(t *Type) string {
	v := rep.value
	return "{" + t.fields[rep.index].Name + ": " + stringRep(v.t, v.rep) + "}"
}
