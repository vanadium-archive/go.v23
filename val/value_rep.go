package val

import (
	"fmt"
)

// This file contains the non-trivial "rep" types, which are the representation
// of values of certain types.

// repMap represents map values.  There are two underlying representations
// depending on the key type.  We decide which one to use based on the type of
// the map key, and never change representations thereafter.
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
	return stringRep(kv.key.t, kv.key.rep, true) + ": " + stringRep(kv.val.t, kv.val.rep, true)
}

func zeroRepMap(t *Type) repMap {
	rep := repMap{}
	switch t.key.kind {
	case Bool, Int32, Int64, Uint32, Uint64, Float32, Float64, Complex64, Complex128, String, Bytes, Enum:
		// TODO(toddw): Structs with exactly 1 simple field may also use fast.
		rep.fastIndex = make(map[interface{}]kvPair)
	default:
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
			if !b.equalKV(maptype, akv) {
				return false
			}
		}
	} else {
		for _, akv := range *a.slowIndex {
			if !b.equalKV(maptype, akv) {
				return false
			}
		}
	}
	return true
}

func (rep repMap) equalKV(maptype *Type, kv kvPair) bool {
	val := rep.Index(maptype, kv.key)
	if val == nil {
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

func fastKeyRep(key *Value) interface{} {
	switch key.t.kind {
	case Bytes:
		return string(key.rep.([]byte))
	}
	return key.rep
}

func (rep repMap) Index(maptype *Type, key *Value) *Value {
	if !maptype.key.AssignableFrom(key.t) {
		panic(fmt.Errorf("val: map type %v can't be indexed with key type %v", maptype, key.t))
	}
	if rep.fastIndex != nil {
		if kv, ok := rep.fastIndex[fastKeyRep(key)]; ok {
			return kv.val
		}
	} else {
		for _, kv := range *rep.slowIndex {
			if Equal(kv.key, key) {
				return kv.val
			}
		}
	}
	return nil
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
		if val == nil {
			delete(rep.fastIndex, fastKeyRep(key))
		} else {
			rep.fastIndex[fastKeyRep(key)] = kvPair{key, val}
		}
	} else {
		for ix := 0; ix < len(*rep.slowIndex); ix++ {
			if Equal((*rep.slowIndex)[ix].key, key) {
				if val == nil {
					// Delete entry ix by copying last entry into ix and shrinking by 1.
					last := len(*rep.slowIndex) - 1
					(*rep.slowIndex)[ix] = (*rep.slowIndex)[last]
					(*rep.slowIndex) = (*rep.slowIndex)[:last]
				} else {
					(*rep.slowIndex)[ix].val = val
				}
				return
			}
		}
		// The key doesn't exist in the slow index; append it.
		*rep.slowIndex = append(*rep.slowIndex, kvPair{key, val})
	}
}

// repStruct represents the fields of a struct in a slice.  Structs always
// exist, and consequently the slice is always sized to the number of fields in
// the slice.  Each field is lazily initialized, as a minor optimization.
// Regardless of the representation, the semantics are that nil fields are
// indistinguishable from zero valued fields.
type repStruct []*Value

func zeroRepStruct(t *Type) repStruct {
	return make(repStruct, len(t.fields))
}

func copyRepStruct(rep repStruct) repStruct {
	return repStruct(copySliceOfValues([]*Value(rep)))
}

func equalRepStruct(a, b repStruct) bool {
	for index := 0; index < len(a); index++ {
		// Handle cases where we're using nil to represent zero fields.
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

func (rep repStruct) IsZero() bool {
	for _, field := range rep {
		if field != nil && !field.IsZero() {
			return false
		}
	}
	return true
}

func (rep repStruct) String(t *Type) string {
	s := "{"
	for index, field := range rep {
		if index > 0 {
			s += ", "
		}
		fieldt := t.fields[index]
		s += fieldt.Name
		s += ": "
		if field == nil {
			s += stringRep(fieldt.Type, zeroRep(fieldt.Type), true)
		} else {
			s += stringRep(field.t, field.rep, true)
		}
	}
	return s + "}"
}

func (rep repStruct) Field(t *Type, index int) *Value {
	if oldf := rep[index]; oldf != nil {
		return oldf
	}
	// Lazy initialization; create the new field upon first access.
	newf := Zero(t.fields[index].Type)
	rep[index] = newf
	return newf
}
