package vdlgen

// TODO(toddw): Add tests

import (
	"fmt"
	"strconv"

	"v.io/core/veyron2/vdl"
	"v.io/core/veyron2/vdl/codegen"
)

// TypedConst returns the explicitly-typed vdl const corresponding to v, in the
// given pkgPath, with the given imports.
func TypedConst(v *vdl.Value, pkgPath string, imports codegen.Imports) (string, error) {
	if v == nil {
		return "nil", nil
	}
	k, t := v.Kind(), v.Type()
	typestr, err := Type(t, pkgPath, imports)
	if err != nil {
		return "", err
	}
	if k == vdl.Optional {
		// TODO(toddw): This only works if the optional elem is a composite literal.
		if elem := v.Elem(); elem != nil {
			elemstr, err := untypedConst(elem, pkgPath, imports)
			if err != nil {
				return "", err
			}
			return typestr + elemstr, nil
		}
		return typestr + "(nil)", nil
	}
	valstr, err := untypedConst(v, pkgPath, imports)
	if err != nil {
		return "", err
	}
	switch {
	case k == vdl.Enum || k == vdl.TypeObject || t == vdl.BoolType || t == vdl.StringType:
		// Enum and TypeObject already include the type in their value.
		// Built-in bool and string are implicitly convertible from literals.
		return valstr, nil
	}
	switch k {
	case vdl.Array, vdl.List, vdl.Set, vdl.Map, vdl.Struct, vdl.Union:
		// { } are used instead of ( ) for composites, except for []byte and [N]byte
		if !t.IsBytes() {
			return typestr + valstr, nil
		}
	}
	return typestr + "(" + valstr + ")", nil
}

// untypedConst returns the untyped vdl const corresponding to v, in the given
// pkgPath, with the given imports.
func untypedConst(v *vdl.Value, pkgPath string, imports codegen.Imports) (string, error) {
	k, t := v.Kind(), v.Type()
	if t.IsBytes() {
		return strconv.Quote(string(v.Bytes())), nil
	}
	switch k {
	case vdl.Any:
		if elem := v.Elem(); elem != nil {
			return TypedConst(elem, pkgPath, imports)
		}
		return "nil", nil
	case vdl.Optional:
		if elem := v.Elem(); elem != nil {
			return untypedConst(elem, pkgPath, imports)
		}
		return "nil", nil
	case vdl.Bool:
		return strconv.FormatBool(v.Bool()), nil
	case vdl.Byte:
		return strconv.FormatUint(uint64(v.Byte()), 10), nil
	case vdl.Uint16, vdl.Uint32, vdl.Uint64:
		return strconv.FormatUint(v.Uint(), 10), nil
	case vdl.Int16, vdl.Int32, vdl.Int64:
		return strconv.FormatInt(v.Int(), 10), nil
	case vdl.Float32, vdl.Float64:
		return formatFloat(v.Float(), k), nil
	case vdl.Complex64, vdl.Complex128:
		switch re, im := real(v.Complex()), imag(v.Complex()); {
		case im > 0:
			return formatFloat(re, k) + "+" + formatFloat(im, k) + "i", nil
		case im < 0:
			return formatFloat(re, k) + formatFloat(im, k) + "i", nil
		default:
			return formatFloat(re, k), nil
		}
	case vdl.String:
		return strconv.Quote(v.RawString()), nil
	case vdl.Array, vdl.List:
		if v.IsZero() {
			return "{}", nil
		}
		s := "{"
		for ix := 0; ix < v.Len(); ix++ {
			elemstr, err := untypedConst(v.Index(ix), pkgPath, imports)
			if err != nil {
				return "", err
			}
			if ix > 0 {
				s += ", "
			}
			s += elemstr
		}
		return s + "}", nil
	case vdl.Set, vdl.Map:
		// TODO(toddw): Sort keys to get a deterministic ordering.
		s := "{"
		for ix, key := range v.Keys() {
			keystr, err := untypedConst(key, pkgPath, imports)
			if err != nil {
				return "", err
			}
			if ix > 0 {
				s += ", "
			}
			s += keystr
			if k == vdl.Map {
				elemstr, err := untypedConst(v.MapIndex(key), pkgPath, imports)
				if err != nil {
					return "", err
				}
				s += ": " + elemstr
			}
		}
		return s + "}", nil
	case vdl.Struct:
		s := "{"
		hasFields := false
		for ix := 0; ix < t.NumField(); ix++ {
			vf := v.Field(ix)
			if vf.IsZero() {
				continue
			}
			fieldstr, err := untypedConst(vf, pkgPath, imports)
			if err != nil {
				return "", err
			}
			if hasFields {
				s += ", "
			}
			s += t.Field(ix).Name + ": " + fieldstr
			hasFields = true
		}
		return s + "}", nil
	case vdl.Union:
		index, value := v.UnionField()
		valuestr, err := untypedConst(value, pkgPath, imports)
		if err != nil {
			return "", err
		}
		return "{" + t.Field(index).Name + ": " + valuestr + "}", nil
	}
	// Enum and TypeObject always require the typestr.
	switch k {
	case vdl.Enum:
		typestr, err := Type(t, pkgPath, imports)
		if err != nil {
			return "", err
		}
		return typestr + "." + v.EnumLabel(), nil
	case vdl.TypeObject:
		typestr, err := Type(v.TypeObject(), pkgPath, imports)
		if err != nil {
			return "", err
		}
		return "typeobject(" + typestr + ")", nil
	default:
		return "", fmt.Errorf("vdlgen.Const unhandled type: %v %v", k, t)
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
