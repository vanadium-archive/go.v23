// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vdltest

import (
	"fmt"
	"hash"
	"hash/fnv"
	"math/rand"
	"strconv"
	"time"

	"v.io/v23/vdl"
)

// TODO(toddw): Generate single-field structs, and favor big structs.
// TODO(toddw): Add unnamed list,set,map to potential elem types.
// TODO(toddw): Choose how to handle enum and array lens.

// EntryValue is like Entry, but represents the target and source values as
// *vdl.Value, rather than interface{}.
type EntryValue struct {
	Label  string
	Target *vdl.Value
	Source *vdl.Value
}

// IsCanonical returns true iff e.Target == e.Source.
func (e EntryValue) IsCanonical() bool {
	return vdl.EqualValue(e.Target, e.Source)
}

// EntryGenerator generates test entries.
type EntryGenerator struct {
	valueGen *ValueGenerator
	hasher   hash.Hash64
	randSeed int64
	rng      *rand.Rand
	// Keep the total set of types separated by kind, for quick filtering.
	ttBool, ttStringEnum, ttNumber, ttArrayList, ttSet, ttMap, ttStruct, ttUnion []*vdl.Type
}

// NewEntryGenerator returns a new EntryGenerator, which uses a random number
// generator seeded to the current time.
func NewEntryGenerator(types []*vdl.Type) *EntryGenerator {
	now := time.Now().Unix()
	g := &EntryGenerator{
		valueGen: NewValueGenerator(types),
		hasher:   fnv.New64a(),
		randSeed: now,
		rng:      rand.New(rand.NewSource(now)),
	}
	for _, tt := range types {
		kind := tt.NonOptional().Kind()
		if kind.IsNumber() {
			g.ttNumber = append(g.ttNumber, tt)
		}
		switch kind {
		case vdl.Bool:
			g.ttBool = append(g.ttBool, tt)
		case vdl.String, vdl.Enum:
			g.ttStringEnum = append(g.ttStringEnum, tt)
		case vdl.Array, vdl.List:
			g.ttArrayList = append(g.ttArrayList, tt)
		case vdl.Set:
			g.ttSet = append(g.ttSet, tt)
		case vdl.Map:
			g.ttMap = append(g.ttMap, tt)
		case vdl.Struct:
			g.ttStruct = append(g.ttStruct, tt)
		case vdl.Union:
			g.ttUnion = append(g.ttUnion, tt)
		}
	}
	return g
}

// RandSeed sets the seed for the random number generator used by g.
func (g *EntryGenerator) RandSeed(seed int64) {
	g.randSeed = seed
}

// candidateTypes returns the types whose values might be convertible to values
// of the tt type.  In theory we could run a compatibility check here for fewer
// false positives, but we'll need to check compatibility again when generating
// values anyways, to handle inner types of nested any, so we don't bother.
func (g *EntryGenerator) candidateTypes(tt *vdl.Type, mode entryMode) []*vdl.Type {
	var candidates []*vdl.Type
	kind := tt.NonOptional().Kind()
	if kind.IsNumber() {
		candidates = g.ttNumber
	}
	switch kind {
	case vdl.Bool:
		candidates = g.ttBool
	case vdl.String, vdl.Enum:
		candidates = g.ttStringEnum
	case vdl.Array, vdl.List:
		candidates = g.ttArrayList
	case vdl.Set:
		candidates = g.ttSet
	case vdl.Map:
		candidates = g.ttMap
	case vdl.Struct:
		candidates = g.ttStruct
	case vdl.Union:
		candidates = g.ttUnion
	case vdl.TypeObject:
		candidates = []*vdl.Type{vdl.TypeObjectType}
	}
	if mode == entryAll {
		return candidates
	}
	var filtered []*vdl.Type
	for _, c := range candidates {
		if (mode == entryUnnamed) == (c.Name() == "") {
			filtered = append(filtered, c)
		}
	}
	return filtered
}

// GenAllPass generates a list of passing entries for all types.
func (g *EntryGenerator) GenAllPass() []EntryValue {
	var entries []EntryValue
	for _, tt := range g.valueGen.Types {
		entries = append(entries, g.GenPass(tt)...)
	}
	return entries
}

// GenPass generates a list of passing entries for the tt type.  Each entry has
// a target value of type tt.  The source value of each entry is created with
// the property that, if the source value is converted to the target type, the
// result is exactly the target value.
func (g *EntryGenerator) GenPass(tt *vdl.Type) []EntryValue {
	// Add entries for zero values.
	ev := g.genPass("Zero", vdl.ZeroValue(tt), 3, entryAll)
	if tt.Kind() == vdl.Optional {
		// Add entry to convert from any(nil) to optional(nil).
		ev = append(ev, EntryValue{"NilAny", vdl.ZeroValue(tt), vdl.ZeroValue(vdl.AnyType)})
	}
	// Handle some special-cases.
	switch {
	case tt == vdl.AnyType:
		// Don't create non-nil any values.
		return ev
	case tt.Kind() == vdl.Enum:
		// Test all enum values exhaustively.
		for ix := 1; ix < tt.NumEnumLabel(); ix++ {
			ev = append(ev, g.genPass("Full", vdl.EnumValue(tt, ix), -1, entryAll)...)
		}
		return ev
	}
	// Add full entries, which are deterministic and recursively non-zero.  As a
	// special-case, for types that are part of a cycle, the values are still
	// deterministic but will contain zero items.
	if needsGenFull(tt) {
		full := g.makeValue(tt, GenFull, 0)
		ev = append(ev, g.genPass("Full", full, 3, entryAll)...)
	}
	// Add entries for max/min number testing.
	if needsGenPos(tt) {
		max := g.makeValue(tt, GenPosMax, 0)
		min := g.makeValue(tt, GenPosMin, 0)
		ev = append(ev, g.genPassNumbers("PosMax", max)...)
		ev = append(ev, g.genPassNumbers("PosMin", min)...)
	}
	if needsGenNeg(tt) {
		max := g.makeValue(tt, GenNegMax, 0)
		min := g.makeValue(tt, GenNegMin, 0)
		ev = append(ev, g.genPassNumbers("NegMax", max)...)
		ev = append(ev, g.genPassNumbers("NegMin", min)...)
	}
	// Add some random entries.
	if needsGenRandom(tt) {
		for ix := 0; ix < 3; ix++ {
			random := g.makeValue(tt, GenRandom, ix)
			ev = append(ev, g.genPass("Random", random, 3, entryAll)...)
		}
	}
	return ev
}

func (g *EntryGenerator) genPassNumbers(label string, target *vdl.Value) []EntryValue {
	// We test the boundaries for all unnamed (i.e. built-in) numbers, and limit
	// the entries otherwise.
	var ev []EntryValue
	if tt := target.Type(); tt.Kind().IsNumber() && tt.Name() == "" {
		ev = append(ev, g.genPass(label, target, -1, entryUnnamed)...)
		ev = append(ev, g.genPass(label, target, 3, entryNamedNoCanonical)...)
	} else {
		ev = append(ev, g.genPass(label, target, 3, entryAll)...)
	}
	return ev
}

type entryMode int

const (
	entryAll              entryMode = iota // Consider all types when generating.
	entryUnnamed                           // Only consider unnamed (anonymous) types.
	entryNamedNoCanonical                  // Only consider named types, no canonical.
)

func (m entryMode) String() string {
	switch m {
	case entryAll:
		return "All"
	case entryUnnamed:
		return "Unnamed"
	case entryNamedNoCanonical:
		return "NamedNoCanonical"
	}
	panic(fmt.Errorf("vdltest: unhandled mode %d", m))
}

// genPass generates a list of passing entries, where each entry has the given
// target value.  The given max limits the number of returned entries; -1
// returns all entries.
func (g *EntryGenerator) genPass(label string, target *vdl.Value, max int, mode entryMode) []EntryValue {
	var ev []EntryValue
	if mode != entryNamedNoCanonical {
		// Add the canonical identity conversion for each target value.
		ev = append(ev, EntryValue{label, target, target})
	}
	// Add up to max conversion entries.  The general strategy is to add an entry
	// for each source type where we can create a value that can convert to the
	// target.  We filter out all types that cannot possibly be convertible, and
	// are left with candidates.  The candidates still might not be convertible,
	// so we try to mimic values for each type, and add the entry if it succeeds.
	candidates := g.candidateTypes(target.Type(), mode)
	switch {
	case max == 0:
		candidates = nil
	case max != -1:
		// Randomly permute the candidates if we're returning a limited number of
		// entries, to cover more cases.
		shuffled := make([]*vdl.Type, len(candidates))
		for i, p := range g.perm(len(candidates), label, target, mode) {
			shuffled[i] = candidates[p]
		}
		candidates = shuffled
	}
	num := 0
	for _, ttSource := range candidates {
		if ttSource == target.Type() {
			continue // Skip the canonical case, which was handled above.
		}
		if source := MimicValue(ttSource, target); source != nil {
			if max >= 0 && num >= max {
				break
			}
			num++
			ev = append(ev, EntryValue{label, target, source})
		}
	}
	return ev
}

func (g *EntryGenerator) makeValue(tt *vdl.Type, mode GenMode, i int) *vdl.Value {
	// ValueGenerator creates random values for us, but we'd like to ensure that
	// the values don't change spuriously.  I.e. adding new types or generating
	// more values shouldn't change any existing values.  To this end, we seed the
	// random source with a hash of the unique type string, gen mode and iteration
	// counter i.
	g.hasher.Reset()
	g.hasher.Write([]byte(tt.Unique()))
	g.hasher.Write([]byte(mode.String()))
	g.hasher.Write([]byte(strconv.Itoa(i)))
	g.valueGen.RandSeed(g.randSeed + int64(g.hasher.Sum64()))
	return g.valueGen.Gen(tt, mode)
}

func (g *EntryGenerator) perm(n int, label string, target *vdl.Value, mode entryMode) []int {
	// Similar to makeValue, we'd like to ensure that our choice of random
	// candidate permutations don't change our test values spuriously.
	g.hasher.Reset()
	g.hasher.Write([]byte(label))
	// TODO(toddw): The target string changes spuriously because of map ordering.
	// Add vdl.Value.UniqueString() or something like that, which will also be
	// useful for maintaining sets of all vdl values.
	//
	//g.hasher.Write([]byte(target.String()))
	g.hasher.Write([]byte(mode.String()))
	g.rng.Seed(g.randSeed + int64(g.hasher.Sum64()))
	return g.rng.Perm(n)
}

// GenAllFail generates a list of failing entries for all types.
func (g *EntryGenerator) GenAllFail() []EntryValue {
	var entries []EntryValue
	for _, tt := range g.valueGen.Types {
		entries = append(entries, g.GenFail(tt)...)
	}
	return entries
}

// GenFail generates a list of failing entries for the tt type.  Each entry has
// a target value of type tt.  The source value of each entry is created with
// the property that, if the source value is converted to the target type, the
// conversion fails.
func (g *EntryGenerator) GenFail(tt *vdl.Type) []EntryValue {
	// TODO(toddw): Implement this!
	return nil
}
