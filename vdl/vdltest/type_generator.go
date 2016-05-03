// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vdltest

import (
	"math/rand"
	"strconv"
	"strings"
	"time"

	"v.io/v23/vdl"
)

// TypeGenerator generates types.
type TypeGenerator struct {
	MaxPerKind      int // keep in mind that unnamed/named count as well!
	Prefix          string
	AllFieldsSuffix string
	rng             *rand.Rand
}

// NewTypeGenerator returns a new TypeGenerator, which uses a random number
// generator seeded to the current time.
func NewTypeGenerator() *TypeGenerator {
	return &TypeGenerator{
		MaxPerKind:      -1,
		Prefix:          "V",
		AllFieldsSuffix: "All",
		rng:             rand.New(rand.NewSource(time.Now().Unix())),
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
	vdl.TypeObjectType,
}

// Gen generates types up to the given depthLimit.  Depth 0 only includes scalar
// types.  Depth N>0 includes composite types built out of all types at depth
// N-1.  Optional types are considered to be at the same depth as their elem
// type, except for depth 0; optional types are not scalar.
func (g *TypeGenerator) Gen(depthLimit int) []*vdl.Type {
	if depthLimit < 0 {
		return nil
	}
	base := append([]*vdl.Type(nil), ttBuiltIn...)
	base = append(base, g.allNamed(base...)...)
	// TODO: random enums?
	unnamedEnums := []*vdl.Type{
		vdl.EnumType("A", "B", "C"),
		vdl.EnumType("B", "C", "D"),
	}
	base = append(base, g.allNamed(unnamedEnums...)...) // enums must be named
	// Depth 0 only includes scalars, so return them now.
	if depthLimit == 0 {
		return base
	}
	// Special-cases that would have been included in depth 0, had we not
	// special-cased depth 0 to only include scalars.  We include a named empty
	// struct, the error type, and all optionals.
	base = append(base, g.allNamed(vdl.StructType())...)
	base = append(base, vdl.ErrorType)
	//base = append(base, g.allNamed(vdl.ErrorType.Elem())...)
	base = append(base, g.allOptional(base...)...)
	// Generate composite types for each depth N, based on the base types from
	// depth N-1.
	origAllFieldsSuffix := g.AllFieldsSuffix
	result := base
	for depth := 1; depth <= depthLimit; depth++ {
		// Make sure the all-suffix is unique.  Otherwise we'll get name collisions;
		// e.g. StructAll would be used at each depth, rather than StructDepth1,
		// StructDepth2, etc.
		g.AllFieldsSuffix = "Depth" + strconv.Itoa(depth)
		var next []*vdl.Type
		next = append(next, g.GenArray(3, base...)...) // TODO(toddw): random len?
		next = append(next, g.GenList(base...)...)
		next = append(next, g.GenSet(base...)...)
		next = append(next, g.GenMap(base...)...)
		next = append(next, g.GenStruct(base...)...)
		next = append(next, g.GenUnion(base...)...)
		// Make optionals out of all the composite types we've generated.  We
		// consider optionals to be at the same depth as their elem type.
		next = append(next, g.GenOptional(next...)...)
		// Append to our running result, and update our next base types.
		result = append(result, next...)
		base = next
	}
	g.AllFieldsSuffix = origAllFieldsSuffix
	return result
}

func (g *TypeGenerator) prune(types ...[]*vdl.Type) []*vdl.Type {
	var result []*vdl.Type
	for _, t := range types {
		result = append(result, t...)
	}
	switch max := g.MaxPerKind; {
	case max == 0:
		result = nil
	case max > 0 && max < len(result):
		pruned := make([]*vdl.Type, max)
		for i, p := range g.rng.Perm(len(result))[:max] {
			pruned[i] = result[p]
		}
		result = pruned
	}
	return result
}

// Name returns the name of type tt.
func (g *TypeGenerator) Name(tt *vdl.Type) string {
	return g.Prefix + g.name(tt)
}

func (g *TypeGenerator) name(tt *vdl.Type) string {
	// Handle the special-cases.
	switch {
	case tt == vdl.TypeObjectType:
		return "TypeObject" // by default we'd get Typeobject
	case tt == vdl.ErrorType || tt.Name() == "error":
		return "Error" // by default we'd get Opterror or Verror
	case tt.Kind() == vdl.Optional:
		return "Opt" + g.name(tt.Elem())
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
		name += strconv.Itoa(tt.Len()) + "_" + g.name(tt.Elem())
	case vdl.List:
		name += "_" + g.name(tt.Elem())
	case vdl.Set:
		name += "_" + g.name(tt.Key())
	case vdl.Map:
		name += "_" + g.name(tt.Key()) + "_" + g.name(tt.Elem())
	case vdl.Struct, vdl.Union:
		switch tt.NumField() {
		case 0:
			name += "Empty"
		case 1:
			name += "_" + g.name(tt.Field(0).Type)
		default:
			name += g.AllFieldsSuffix
		}
	}
	return name
}

func (g *TypeGenerator) allNamed(base ...*vdl.Type) []*vdl.Type {
	var named []*vdl.Type
	for _, tt := range base {
		// TODO(toddw): Resolve bug in vdl to disallow Optional from being named.
		if !tt.CanBeNamed() || tt.Kind() == vdl.Optional {
			continue
		}
		named = append(named, vdl.NamedType(g.Name(tt), tt))
	}
	return named
}

// GenArray returns array types built out of the given base types, with the
// given len.
func (g *TypeGenerator) GenArray(len int, base ...*vdl.Type) []*vdl.Type {
	var unnamed []*vdl.Type
	for _, tt := range base {
		unnamed = append(unnamed, vdl.ArrayType(len, tt))
	}
	return g.prune(g.allNamed(unnamed...)) // Array must be named.
}

// GenList returns list types built out of the given base types.
func (g *TypeGenerator) GenList(base ...*vdl.Type) []*vdl.Type {
	var unnamed []*vdl.Type
	for _, tt := range base {
		unnamed = append(unnamed, vdl.ListType(tt))
	}
	return g.prune(unnamed, g.allNamed(unnamed...))
}

// GenSet returns set types built out of the given base types.
func (g *TypeGenerator) GenSet(base ...*vdl.Type) []*vdl.Type {
	var unnamed []*vdl.Type
	for _, tt := range base {
		if tt.CanBeKey() {
			unnamed = append(unnamed, vdl.SetType(tt))
		}
	}
	return g.prune(unnamed, g.allNamed(unnamed...))
}

// GenMap returns map types built out of the given base types.
func (g *TypeGenerator) GenMap(base ...*vdl.Type) []*vdl.Type {
	var unnamed []*vdl.Type
	for _, tt := range base {
		if tt.CanBeKey() {
			unnamed = append(unnamed, vdl.MapType(tt, tt))
		} else {
			unnamed = append(unnamed, vdl.MapType(vdl.StringType, tt))
		}
	}
	return g.prune(unnamed, g.allNamed(unnamed...))
}

// GenStruct returns struct types built out of the given base types.
func (g *TypeGenerator) GenStruct(base ...*vdl.Type) []*vdl.Type {
	return g.prune(g.allFields(vdl.Struct, base...))
}

// GenUnion returns union types built out of the given base types.
func (g *TypeGenerator) GenUnion(base ...*vdl.Type) []*vdl.Type {
	return g.prune(g.allFields(vdl.Union, base...))
}

func (g *TypeGenerator) allFields(kind vdl.Kind, base ...*vdl.Type) []*vdl.Type {
	// TODO: Add single-field structs, and prioritize keeping all-fields.
	var fields []vdl.Field
	for i, tt := range base {
		fieldName := "F" + strconv.Itoa(i)
		fields = append(fields, vdl.Field{Name: fieldName, Type: tt})
	}
	var unnamed []*vdl.Type
	if kind == vdl.Struct {
		unnamed = append(unnamed, vdl.StructType(fields...))
	} else {
		unnamed = append(unnamed, vdl.UnionType(fields...))
	}
	return g.allNamed(unnamed...) // Struct and Union must be named.
}

// GenOptional returns optional types built out of the given base types.
func (g *TypeGenerator) GenOptional(base ...*vdl.Type) []*vdl.Type {
	return g.prune(g.allOptional(base...))
}

func (g *TypeGenerator) allOptional(base ...*vdl.Type) []*vdl.Type {
	var unnamed []*vdl.Type
	for _, tt := range base {
		if tt.CanBeOptional() {
			unnamed = append(unnamed, vdl.OptionalType(tt))
		}
	}
	return unnamed // Optional types cannot be named.
}
