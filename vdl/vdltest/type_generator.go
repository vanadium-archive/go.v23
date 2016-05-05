// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vdltest

import (
	"fmt"
	"math/rand"
	"sort"
	"strconv"
	"strings"
	"time"

	"v.io/v23/vdl"
)

// TypeGenerator generates types.
type TypeGenerator struct {
	// NamePrefix is the type name prefix for named types.
	NamePrefix string
	// BaseTypesPerKind is the number of base types of each kind to use when
	// generating composite types at each depth.  Numbers are considered to be a
	// single kind.
	//
	// Each element corresponds to the value at that depth, starting at depth 1.
	// Use -1 to indicate "unlimited".
	BaseTypesPerKind []int
	// NumStructUnion is the number of struct/union types to generate at each
	// depth.  The first struct/union type at depth N always contains fields of
	// all types from depth N-1, while other struct/union types contain randomly
	// chosen subsets of those fields.
	NumStructUnion int
	// AllFieldsPerKind is like BaseTypesPerKind, but limits the number of fields
	// to generate for All structs and unions.
	NumAllFieldsPerKind []int
	// MaxRandomFields limits the number of fields to randomly choose for non-All
	// structs and unions.
	MaxRandomFields int
	// MaxArrayLen is the maximum array length; the actual length is chosen
	// randomly up to this max.
	MaxArrayLen int

	rng *rand.Rand
}

// NewTypeGenerator returns a new TypeGenerator, which uses a random number
// generator seeded to the current time.
func NewTypeGenerator() *TypeGenerator {
	return &TypeGenerator{
		NamePrefix:          "V",
		BaseTypesPerKind:    []int{3, 1},
		NumStructUnion:      3,
		NumAllFieldsPerKind: []int{-1, 2},
		MaxRandomFields:     5,
		MaxArrayLen:         3,
		rng:                 rand.New(rand.NewSource(time.Now().Unix())),
	}
}

// RandSeed sets the seed for the random number generator used by g.
func (g *TypeGenerator) RandSeed(seed int64) {
	g.rng.Seed(seed)
}

var ttBuiltIn = []*vdl.Type{
	vdl.AnyType,
	vdl.BoolType,
	vdl.StringType,
	vdl.TypeObjectType,
	vdl.ByteType,
	vdl.Uint16Type,
	vdl.Uint32Type,
	vdl.Uint64Type,
	vdl.Int8Type,
	vdl.Int16Type,
	vdl.Int32Type,
	vdl.Int64Type,
	vdl.Float32Type,
	vdl.Float64Type,
}

// Gen generates types up to and including the given maxDepth.
//
// Depth 0 only includes scalar types.  Depth N>0 includes composite types built
// out of types from depth N-1.  Optional types are considered to be at the same
// depth as their elem type, except for depth 0; optional types are not scalar.
//
// See the TypeGenerator exported fields for additional configuration options.
func (g *TypeGenerator) Gen(maxDepth int) []*vdl.Type {
	if maxDepth < 0 {
		return nil
	}
	depth0 := append([]*vdl.Type(nil), ttBuiltIn...)
	depth0 = append(depth0, g.genNamed(depth0...)...)
	unnamedEnums := []*vdl.Type{
		vdl.EnumType("A", "B", "C"),
		vdl.EnumType("B", "C", "D"),
	}
	depth0 = append(depth0, g.genNamed(unnamedEnums...)...) // enums must be named
	// Depth 0 only includes scalars.
	if maxDepth == 0 {
		return joinTypes(splitDepth0Types(depth0))
	}
	// Special-cases that would have been included in depth 0, had we not
	// special-cased depth 0 to only include scalars.  We include a named empty
	// struct and the error type.
	depth0 = append(depth0, g.genNamed(vdl.StructType())...)
	depth0 = append(depth0, vdl.ErrorType)
	// Generate composite types for each depth N, based on the base types from
	// depth N-1.  We keep the base types in separate buckets per kind, so that
	// when we prune types we get a good distribution of different kinds.
	base := splitDepth0Types(depth0)
	base = append(base, g.genOptional(base, 1))
	result := joinTypes(base)
	for depth := 1; depth <= maxDepth; depth++ {
		var next [][]*vdl.Type
		next = append(next, g.genArray(base, depth))
		next = append(next, g.genList(base, depth))
		next = append(next, g.genSetMap(vdl.Set, base, depth))
		next = append(next, g.genSetMap(vdl.Map, base, depth))
		next = append(next, g.genStructUnion(vdl.Struct, base, depth))
		next = append(next, g.genStructUnion(vdl.Union, base, depth))
		// Make optionals out of all the composite types we've generated.  We
		// consider optionals to be at the same depth as their elem type.
		next = append(next, g.genOptional(next, depth))
		// Append to our running result, and update our next base types.
		result = append(result, joinTypes(next)...)
		base = next
	}
	return result
}

func joinTypes(buckets [][]*vdl.Type) []*vdl.Type {
	var types []*vdl.Type
	for _, bucket := range buckets {
		types = append(types, bucket...)
	}
	return types
}

// splitDepth0Types splits depth0 into separate buckets grouped by kind.  All
// numbers are grouped together into a single bucket.  The relative ordering of
// types is maintained in each bucket.
func splitDepth0Types(depth0 []*vdl.Type) [][]*vdl.Type {
	// split[1] holds buckets for numbers, while split[0] holds buckets for
	// everything else.
	var split [2][][]*vdl.Type
CollectLoop:
	for _, tt := range depth0 {
		kind, isNum := tt.Kind(), 0
		// We special-case byte, to ensure byte lists and arrays are generated.
		if kind.IsNumber() && kind != vdl.Byte {
			isNum = 1
		}
		for ix, bucket := range split[isNum] {
			if kind == bucket[0].Kind() {
				// We've seen a type of this kind before, add it to this bucket
				split[isNum][ix] = append(split[isNum][ix], tt)
				continue CollectLoop
			}
		}
		// This is the first time we've seen a type of this kind, add a new bucket.
		split[isNum] = append(split[isNum], []*vdl.Type{tt})
	}
	// Add all numbers at the end in their own bucket.
	return append(split[0], joinTypes(split[1]))
}

func valueAtDepth(values []int, depth int) int {
	depth-- // values starts at depth 1
	switch len := len(values); {
	case len == 0:
		return -1
	case depth < len:
		return values[depth]
	default:
		return values[len-1] // use last element for subsequent depths.
	}
}

func (g *TypeGenerator) basePerKind(depth int) int {
	return valueAtDepth(g.BaseTypesPerKind, depth)
}

// randomChoose returns x sorted values out of [0,n).
// REQUIRES: x <= n
func (g *TypeGenerator) randomChoose(n, x int) []int {
	perm := g.rng.Perm(n)[:x]
	sort.Ints(perm)
	return perm
}

// prune returns maxPerBucket randomly picked types from each slice in buckets.
// The returned types remain in the same relative order.
func (g *TypeGenerator) prune(buckets [][]*vdl.Type, maxPerBucket int) []*vdl.Type {
	if maxPerBucket == 0 {
		return nil
	}
	var result []*vdl.Type
	for _, bucket := range buckets {
		if maxPerBucket < 0 || maxPerBucket >= len(bucket) {
			result = append(result, bucket...)
			continue
		}
		// INVARIANT: maxPerBucket > 0 && maxPerBucket < len(bucket)
		// Choose the remaining types randomly, maintaining the relative order.
		for _, p := range g.randomChoose(len(bucket), maxPerBucket) {
			result = append(result, bucket[p])
		}
	}
	return result
}

func typeName(tt *vdl.Type) string {
	// Handle the special-cases.
	switch {
	case tt == vdl.TypeObjectType:
		return "TypeObject" // by default we'd get Typeobject
	case tt == vdl.ErrorType || tt.Name() == "error":
		return "Error" // by default we'd get Opterror or Verror
	case tt.Kind() == vdl.Optional:
		return "Opt" + typeName(tt.Elem())
	case tt.Name() != "":
		return tt.Name()
	}
	name := strings.Title(tt.Kind().String())
	switch tt.Kind() {
	case vdl.Enum:
		var tmp string
		for i := 0; i < tt.NumEnumLabel(); i++ {
			tmp += tt.EnumLabel(i)
		}
		// VDL prohibits acronyms in identifiers, so we use Abc rather than ABC.
		name += strings.Title(strings.ToLower(tmp))
	case vdl.Array:
		name += strconv.Itoa(tt.Len()) + "_" + typeName(tt.Elem())
	case vdl.List:
		name += "_" + typeName(tt.Elem())
	case vdl.Set:
		name += "_" + typeName(tt.Key())
	case vdl.Map:
		name += "_" + typeName(tt.Key()) + "_" + typeName(tt.Elem())
	case vdl.Struct, vdl.Union:
		switch tt.NumField() {
		case 0:
			name += "Empty"
		case 1:
			name += "_" + typeName(tt.Field(0).Type)
		default:
			panic(fmt.Errorf("vdltest: multi-field structs must be named in genStructUnion: %v", tt))
		}
	}
	return name
}

func (g *TypeGenerator) genNamed(base ...*vdl.Type) []*vdl.Type {
	var named []*vdl.Type
	for _, tt := range base {
		// TODO(toddw): Resolve bug in vdl to disallow Optional from being named.
		if !tt.CanBeNamed() || tt.Kind() == vdl.Optional {
			continue
		}
		named = append(named, vdl.NamedType(g.NamePrefix+typeName(tt), tt))
	}
	return named
}

func (g *TypeGenerator) genArray(base [][]*vdl.Type, depth int) []*vdl.Type {
	// Create arrays out of pruned buckets.  There's no need to collect all arrays
	// that can be created first, since all types may be used as array elems.
	var unnamed []*vdl.Type
	for _, tt := range g.prune(base, g.basePerKind(depth)) {
		len := 1 + g.rng.Intn(g.MaxArrayLen)
		unnamed = append(unnamed, vdl.ArrayType(len, tt))
	}
	return g.genNamed(unnamed...) // arrays must be named
}

func (g *TypeGenerator) genList(base [][]*vdl.Type, depth int) []*vdl.Type {
	// Create lists out of pruned buckets.  There's no need to collect all lists
	// that can be created first, since all types may be used as list elems.
	var unnamed, named []*vdl.Type
	for _, tt := range g.prune(base, g.basePerKind(depth)) {
		unnamed = append(unnamed, vdl.ListType(tt))
	}
	for _, tt := range g.prune(base, g.basePerKind(depth)) {
		named = append(named, g.genNamed(vdl.ListType(tt))...)
	}
	return append(unnamed, named...)
}

func (g *TypeGenerator) genSetMap(kind vdl.Kind, base [][]*vdl.Type, depth int) []*vdl.Type {
	// First collect all sets and maps that can be created, by bucket.  We can't
	// prune before this, since many types can't be used as keys.
	var allUnnamed, allNamed [][]*vdl.Type
	for _, bucket := range base {
		var unnamed []*vdl.Type
		for _, tt := range bucket {
			if tt.CanBeKey() {
				if kind == vdl.Set {
					unnamed = append(unnamed, vdl.SetType(tt))
				} else {
					unnamed = append(unnamed, vdl.MapType(tt, tt))
				}
			}
		}
		allUnnamed = append(allUnnamed, unnamed)
		allNamed = append(allNamed, g.genNamed(unnamed...))
	}
	// Return pruned types.
	max := g.basePerKind(depth)
	return append(g.prune(allUnnamed, max), g.prune(allNamed, max)...)
}

func (g *TypeGenerator) genStructUnion(kind vdl.Kind, base [][]*vdl.Type, depth int) []*vdl.Type {
	num := g.NumStructUnion
	if num <= 0 {
		return nil
	}
	var res []*vdl.Type
	prefix := g.NamePrefix
	prefix += fmt.Sprintf("%sDepth%d_", strings.Title(kind.String()), depth)
	// First create the All struct, which contains all fields.
	var allFields []vdl.Field
	fieldsPerKind := valueAtDepth(g.NumAllFieldsPerKind, depth)
	for i, tt := range g.prune(base, fieldsPerKind) {
		fieldName := "F" + strconv.Itoa(i)
		allFields = append(allFields, vdl.Field{Name: fieldName, Type: tt})
	}
	num--
	res = append(res, vdl.NamedType(prefix+"All", makeStructUnion(kind, allFields)))
	// Now create some random structs with randomly chosen fields.  Field names in
	// these structs match the field names in the All struct, to provide
	// opportunities for more valid conversions.
	if maxFields := g.MaxRandomFields; maxFields > 0 {
		if len := len(allFields); maxFields >= len {
			maxFields = len - 1
		}
		for ix := 0; ix < num; ix++ {
			var fields []vdl.Field
			for _, f := range g.randomChoose(len(allFields), 1+g.rng.Intn(maxFields)) {
				fields = append(fields, allFields[f])
			}
			name := prefix + "Rand" + strconv.Itoa(ix)
			res = append(res, vdl.NamedType(name, makeStructUnion(kind, fields)))
		}
	}
	return res
}

func makeStructUnion(kind vdl.Kind, fields []vdl.Field) *vdl.Type {
	if kind == vdl.Struct {
		return vdl.StructType(fields...)
	}
	return vdl.UnionType(fields...)
}

func (g *TypeGenerator) genOptional(base [][]*vdl.Type, depth int) []*vdl.Type {
	// First collect all optionals that can be created, by bucket.  We can't prune
	// before this, since many types can't be optional.  Optionals are unnamed.
	var allUnnamed [][]*vdl.Type
	for _, bucket := range base {
		var unnamed []*vdl.Type
		for _, tt := range bucket {
			if tt.CanBeOptional() {
				unnamed = append(unnamed, vdl.OptionalType(tt))
			}
		}
		allUnnamed = append(allUnnamed, unnamed)
	}
	// Return pruned items.
	return g.prune(allUnnamed, g.basePerKind(depth))
}
