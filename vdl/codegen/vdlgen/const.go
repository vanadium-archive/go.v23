package vdlgen

// TODO(toddw): Add tests

import (
	"fmt"
	"strconv"

	"veyron.io/veyron/veyron2/vdl"
	"veyron.io/veyron/veyron2/vdl/codegen"
)

// TypedConst returns the explicitly-typed vdl const corresponding to v, in the
// given pkgPath, with the given imports.
func TypedConst(v *vdl.Value, pkgPath string, imports codegen.Imports) (string, error) {
	if v == nil {
		return "nil", nil
	}
	k, t := v.Kind(), v.Type()
	valstr, err := constValue(v, pkgPath, imports)
	if err != nil {
		return "", err
	}
	switch {
	case k == vdl.Enum || k == vdl.TypeObject || t == vdl.BoolType || t == vdl.StringType:
		// Enum and TypeObject already include the type in their value.
		// Built-in bool and string are implicitly convertible from literals.
		return valstr, nil
	}
	typestr, err := Type(t, pkgPath, imports)
	if err != nil {
		return "", err
	}
	switch k {
	case vdl.Array, vdl.List, vdl.Set, vdl.Map, vdl.Struct:
		// { } are used instead of ( ) for composites, except for []byte and [N]byte
		if !t.IsBytes() {
			return typestr + valstr, nil
		}
	}
	return typestr + "(" + valstr + ")", nil
}

// constValue returns the untyped vdl const corresponding to v, in the given
// pkgPath, with the given imports.
func constValue(v *vdl.Value, pkgPath string, imports codegen.Imports) (string, error) {
	k, t := v.Kind(), v.Type()
	if t.IsBytes() {
		return strconv.Quote(string(v.Bytes())), nil
	}
	switch k {
	case vdl.Any, vdl.OneOf, vdl.Optional:
		return TypedConst(v.Elem(), pkgPath, imports)
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
		s := "{"
		for ix := 0; ix < v.Len(); ix++ {
			elemstr, err := constValue(v.Index(ix), pkgPath, imports)
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
			keystr, err := constValue(key, pkgPath, imports)
			if err != nil {
				return "", err
			}
			if ix > 0 {
				s += ", "
			}
			s += keystr
			if k == vdl.Map {
				elemstr, err := constValue(v.MapIndex(key), pkgPath, imports)
				if err != nil {
					return "", err
				}
				s += ": " + elemstr
			}
		}
		return s + "}", nil
	case vdl.Struct:
		s := "{"
		for ix := 0; ix < t.NumField(); ix++ {
			fieldstr, err := constValue(v.Field(ix), pkgPath, imports)
			if err != nil {
				return "", err
			}
			if ix > 0 {
				s += ", "
			}
			s += t.Field(ix).Name + ": " + fieldstr
		}
		return s + "}", nil
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
