package val

import (
	"fmt"
)

// This file contains the non-trivial "rep" types, which are the representation
// of values of certain types.

// repBytes represents []byte and [N]byte values.  We special-case these kinds
// of types to easily support the Value.Bytes() and Value.CopyBytes() methods,
// which makes usage more convenient for the user.  This is also a more
// efficient representation.
type repBytes []byte

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
	return kvPair{Copy(kv.key), Copy(kv.val)}
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

func equalRepMap(maptype *Type, a, b repMap) bool {
	if a.Len() != b.Len() {
		return false
	}
	if a.fastIndex != nil {
		for _, akv := range a.fastIndex {
			if !b.hasKV(maptype, akv) {
				return false
			}
		}
	} else {
		for _, akv := range *a.slowIndex {
			if !b.hasKV(maptype, akv) {
				return false
			}
		}
	}
	return true
}

func (rep repMap) hasKV(maptype *Type, kv kvPair) bool {
	val, ok := rep.Index(maptype, kv.key)
	if !ok {
		return false
	}
	return Equal(kv.val, val)
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

func (rep repMap) Index(maptype *Type, key *Value) (*Value, bool) {
	if !maptype.key.AssignableFrom(key.t) {
		panic(fmt.Errorf("val: map type %v can't be indexed with key type %v", maptype, key.t))
	}
	if rep.fastIndex != nil {
		if kv, ok := rep.fastIndex[fastKeyRep(key)]; ok {
			return kv.val, true
		}
	} else {
		for _, kv := range *rep.slowIndex {
			if Equal(kv.key, key) {
				return kv.val, true
			}
		}
	}
	return nil, false
}

func (rep repMap) Assign(maptype *Type, key, val *Value) {
	if !maptype.key.AssignableFrom(key.t) {
		panic(fmt.Errorf("val: map type %v can't be assigned with key type %v", maptype, key.t))
	}
	if val != nil && !maptype.elem.AssignableFrom(val.t) {
		panic(fmt.Errorf("val: map type %v can't be assigned with elem type %v", maptype, val.t))
	}
	// TODO(toddw): Copy key and val?
	if rep.fastIndex != nil {
		rep.fastIndex[fastKeyRep(key)] = kvPair{key, val}
	} else {
		for ix := 0; ix < len(*rep.slowIndex); ix++ {
			if Equal((*rep.slowIndex)[ix].key, key) {
				(*rep.slowIndex)[ix].val = val
				return
			}
		}
		// The key doesn't exist in the slow index; append it.
		*rep.slowIndex = append(*rep.slowIndex, kvPair{key, val})
	}
}

func (rep repMap) Delete(maptype *Type, key *Value) {
	if !maptype.key.AssignableFrom(key.t) {
		panic(fmt.Errorf("val: map type %v can't be assigned with key type %v", maptype, key.t))
	}
	if rep.fastIndex != nil {
		delete(rep.fastIndex, fastKeyRep(key))
	} else {
		for ix := 0; ix < len(*rep.slowIndex); ix++ {
			if Equal((*rep.slowIndex)[ix].key, key) {
				// Delete entry ix by copying last entry into ix and shrinking by 1.
				last := len(*rep.slowIndex) - 1
				(*rep.slowIndex)[ix] = (*rep.slowIndex)[last]
				(*rep.slowIndex) = (*rep.slowIndex)[:last]
				return
			}
		}
	}
}

// repFixedLen represents the elements of an array, and the fields of a struct.
// Both are conceptually fixed-length ordered sequences of values, so we
// represent them as a slice of values initialized to that fixed length.  Each
// item is lazily initialized.  The semantics are that nil items are
// indistinguishable from zero valued items.
type repFixedLen []*Value

func equalRepFixedLen(a, b repFixedLen) bool {
	for index := 0; index < len(a); index++ {
		// Handle cases where we're using nil to represent zero items.
		switch af, bf := a[index], b[index]; {
		case af == nil && bf != nil:
			if !bf.IsZero() {
				return false
			}
		case af != nil && bf == nil:
			if !af.IsZero() {
				return false
			}
		case !Equal(af, bf):
			return false
		}
	}
	return true
}

func (rep repFixedLen) IsZero() bool {
	for _, item := range rep {
		if item != nil && !item.IsZero() {
			return false
		}
	}
	return true
}

func (rep repFixedLen) String(t *Type) string {
	s := "{"
	for index, item := range rep {
		if index > 0 {
			s += ", "
		}
		itemtype := t.elem
		if t.Kind() == Struct {
			itemtype = t.fields[index].Type
			s += t.fields[index].Name
			s += ": "
		}
		if item == nil {
			s += stringRep(itemtype, zeroRep(itemtype))
		} else {
			s += stringRep(item.t, item.rep)
		}
	}
	return s + "}"
}

func (rep repFixedLen) Index(t *Type, index int) *Value {
	if oldf := rep[index]; oldf != nil {
		return oldf
	}
	// Lazy initialization; create the new item upon first access.
	newf := Zero(t)
	rep[index] = newf
	return newf
}
