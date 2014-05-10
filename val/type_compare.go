package val

import (
	"hash/fnv"
	"io"
)

// typeSeenPos is a helper for testing equality of recursive types.  You can
// think of type relationships as a graph, where nodes represent each type, and
// edges point from composite type to subtype.  Recursive types form a cycle in
// this graph.
//
// Testing for equality of types A and B is really just testing for equality of
// (possibly cyclic) graphs rooted at A and B.  Our strategy is to walk through
// the two graphs in lockstep, comparing each node and following the edges.
// Every node we visit is added to the seenPos{A,B} maps, along with the
// position we first saw the node; pos = 0 for the first node inserted in the
// map, pos = 1 for the second, etc.
//
// If we re-visit a node that's already in our maps, we've hit a cycle.  Since
// we've walked through the graphs in lockstep, two equal graphs must obey:
// 1) Every cycle must occur at exactly the same point in our comparison, and
// 2) Every cycle must point back to equivalent nodes in their graphs,
//    identified by the pos we first saw it.
type typeSeenPos struct {
	seenPosA, seenPosB map[*Type]int
}

func makeTypeSeenPos() typeSeenPos {
	return typeSeenPos{make(map[*Type]int), make(map[*Type]int)}
}

// Equal returns two booleans (equal, ok), where ok represents whether an equal
// value can be determined, and equal represents whether a and b are equal.
func (so typeSeenPos) Equal(a, b *Type) (bool, bool) {
	posA, seenA := so.seenPosA[a]
	posB, seenB := so.seenPosB[b]
	// If either type has been seen, we can determine equality.
	switch {
	case seenA && seenB: // Cycle detected in both types, compare pos.
		return posA == posB, true
	case seenA || seenB: // Only one type has a cycle so far, must be different.
		return false, true
	}
	// Neither type has been seen, insert into our maps and indicate that we can't
	// determine equality.
	so.seenPosA[a] = len(so.seenPosA)
	so.seenPosB[b] = len(so.seenPosB)
	return false, false
}

// equalType returns true iff a and b are equal, and should be hash-consed.
func equalType(a, b *Type) bool {
	return equalTypeHelper(a, b, makeTypeSeenPos())
}

func equalTypeHelper(a, b *Type, seenPos typeSeenPos) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	// Prevent infinite loops for recursive types via seenPos.
	if equal, ok := seenPos.Equal(a, b); ok {
		return equal
	}
	// Compare field by field.
	return a.kind == b.kind &&
		a.name == b.name &&
		equalTypeHelper(a.elem, b.elem, seenPos) &&
		equalTypeHelper(a.key, b.key, seenPos) &&
		equalStringSlice(a.labels, b.labels) &&
		equalFieldSlice(a.fields, b.fields, seenPos) &&
		equalTypeSlice(a.types, b.types, seenPos)
}

func equalStringSlice(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for ix := 0; ix < len(a); ix++ {
		if a[ix] != b[ix] {
			return false
		}
	}
	return true
}

func equalFieldSlice(a, b []StructField, seenPos typeSeenPos) bool {
	if len(a) != len(b) {
		return false
	}
	for ix := 0; ix < len(a); ix++ {
		fa, fb := a[ix], b[ix]
		if fa.Name != fb.Name || !equalTypeHelper(fa.Type, fb.Type, seenPos) {
			return false
		}
	}
	return true
}

func equalTypeSlice(a, b []*Type, seenPos typeSeenPos) bool {
	if len(a) != len(b) {
		return false
	}
	for ix := 0; ix < len(a); ix++ {
		if !equalTypeHelper(a[ix], b[ix], seenPos) {
			return false
		}
	}
	return true
}

// hashType returns a hashcode for the Type t.  Equivalent types must return the
// same hashcode, but different types need not return different hashcodes.
func hashType(t *Type) uint64 {
	hash := fnv.New64a()
	hashTypeHelper(t, hash, make(map[*Type]bool))
	return hash.Sum64()
}

// TODO(toddw): Benchmark the code and possibly make this less collision-prone.
func hashTypeHelper(t *Type, hash io.Writer, seen map[*Type]bool) {
	if t == nil || seen[t] {
		return
	}
	seen[t] = true
	hash.Write([]byte(t.kind.String()))
	hash.Write([]byte(t.name))
	hashTypeHelper(t.elem, hash, seen)
	hashTypeHelper(t.key, hash, seen)
	for _, x := range t.labels {
		hash.Write([]byte(x))
	}
	for _, x := range t.fields {
		hash.Write([]byte(x.Name))
		hashTypeHelper(x.Type, hash, seen)
	}
	for _, x := range t.types {
		hashTypeHelper(x, hash, seen)
	}
}

// typeSet represents a set of *Type, implemented as a hashtable mapping from
// hashcode to a bucket of types.  The hashcode is generated via hashType, and
// equality is determined via equalType.  We can't just use map[*Type]bool since
// Go doesn't allow slices to be used in map keys, and our Type contains slices.
type typeSet map[uint64][]*Type

func (x typeSet) insert(hashcode uint64, t *Type) {
	x[hashcode] = append(x[hashcode], t)
}

func (x typeSet) lookup(hashcode uint64, t *Type) *Type {
	bucket := x[hashcode]
	for _, candidate := range bucket {
		if equalType(t, candidate) {
			return candidate
		}
	}
	return nil
}
