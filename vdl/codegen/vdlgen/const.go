package vdlgen

// TODO(toddw): Add tests

import (
	"fmt"
	"strconv"

	"v.io/v23/vdl"
	"v.io/v23/vdl/codegen"
)

// TypedConst returns the explicitly-typed vdl const corresponding to v, in the
// given pkgPath, with the given imports.
func TypedConst(v *vdl.Value, pkgPath string, imports codegen.Imports) string {
	if v == nil {
		return "nil"
	}
	k, t := v.Kind(), v.Type()
	typestr := Type(t, pkgPath, imports)
	if k == vdl.Optional {
		// TODO(toddw): This only works if the optional elem is a composite literal.
		if elem := v.Elem(); elem != nil {
			return typestr + UntypedConst(elem, pkgPath, imports)
		}
		return typestr + "(nil)"
	}
	valstr := UntypedConst(v, pkgPath, imports)
	if k == vdl.Enum || k == vdl.TypeObject || t == vdl.BoolType || t == vdl.StringType {
		// Enum and TypeObject already include the type in their value.
		// Built-in bool and string are implicitly convertible from literals.
		return valstr
	}
	switch k {
	case vdl.Array, vdl.List, vdl.Set, vdl.Map, vdl.Struct, vdl.Union:
		// { } are used instead of ( ) for composites, except for []byte and [N]byte
		if !t.IsBytes() {
			return typestr + valstr
		}
	}
	return typestr + "(" + valstr + ")"
}

// UntypedConst returns the untyped vdl const corresponding to v, in the given
// pkgPath, with the given imports.
func UntypedConst(v *vdl.Value, pkgPath string, imports codegen.Imports) string {
	k, t := v.Kind(), v.Type()
	if t.IsBytes() {
		return strconv.Quote(string(v.Bytes()))
	}
	switch k {
	case vdl.Any:
		if elem := v.Elem(); elem != nil {
			return TypedConst(elem, pkgPath, imports)
		}
		return "nil"
	case vdl.Optional:
		if elem := v.Elem(); elem != nil {
			return UntypedConst(elem, pkgPath, imports)
		}
		return "nil"
	case vdl.Bool:
		return strconv.FormatBool(v.Bool())
	case vdl.Byte:
		return strconv.FormatUint(uint64(v.Byte()), 10)
	case vdl.Uint16, vdl.Uint32, vdl.Uint64:
		return strconv.FormatUint(v.Uint(), 10)
	case vdl.Int16, vdl.Int32, vdl.Int64:
		return strconv.FormatInt(v.Int(), 10)
	case vdl.Float32, vdl.Float64:
		return formatFloat(v.Float(), k)
	case vdl.Complex64, vdl.Complex128:
		switch re, im := real(v.Complex()), imag(v.Complex()); {
		case im > 0:
			return formatFloat(re, k) + "+" + formatFloat(im, k) + "i"
		case im < 0:
			return formatFloat(re, k) + formatFloat(im, k) + "i"
		default:
			return formatFloat(re, k)
		}
	case vdl.String:
		return strconv.Quote(v.RawString())
	case vdl.Array, vdl.List:
		if v.IsZero() {
			return "{}"
		}
		s := "{"
		for ix := 0; ix < v.Len(); ix++ {
			if ix > 0 {
				s += ", "
			}
			s += UntypedConst(v.Index(ix), pkgPath, imports)
		}
		return s + "}"
	case vdl.Set, vdl.Map:
		s := "{"
		for ix, key := range vdl.SortValuesAsString(v.Keys()) {
			if ix > 0 {
				s += ", "
			}
			s += UntypedConst(key, pkgPath, imports)
			if k == vdl.Map {
				s += ": " + UntypedConst(v.MapIndex(key), pkgPath, imports)
			}
		}
		return s + "}"
	case vdl.Struct:
		s := "{"
		hasFields := false
		for ix := 0; ix < t.NumField(); ix++ {
			vf := v.StructField(ix)
			if vf.IsZero() {
				continue
			}
			if hasFields {
				s += ", "
			}
			s += t.Field(ix).Name + ": " + UntypedConst(vf, pkgPath, imports)
			hasFields = true
		}
		return s + "}"
	case vdl.Union:
		index, value := v.UnionField()
		return "{" + t.Field(index).Name + ": " + UntypedConst(value, pkgPath, imports) + "}"
	}
	// Enum and TypeObject always require the typestr.
	switch k {
	case vdl.Enum:
		return Type(t, pkgPath, imports) + "." + v.EnumLabel()
	case vdl.TypeObject:
		return "typeobject(" + Type(v.TypeObject(), pkgPath, imports) + ")"
	default:
		panic(fmt.Errorf("vdlgen.Const unhandled type: %v %v", k, t))
	}
}

func formatFloat(x float64, kind vdl.Kind) string {
	var bitSize int
	switch kind {
	case vdl.Float32, vdl.Complex64:
		bitSize = 32
	case vdl.Float64, vdl.Complex128:
		bitSize = 64
	default:
		panic(fmt.Errorf("formatFloat unhandled kind: %v", kind))
	}
	return strconv.FormatFloat(x, 'g', -1, bitSize)
}
