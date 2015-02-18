package golang

import (
	"fmt"
	"strconv"

	"v.io/core/veyron2/vdl"
	"v.io/core/veyron2/vdl/compile"
	"v.io/core/veyron2/vdl/parse"
)

func constDefGo(data goData, def *compile.ConstDef) string {
	v := def.Value
	return fmt.Sprintf("%s%s %s = %s%s", def.Doc, constOrVar(v.Kind()), def.Name, typedConst(data, v), def.DocSuffix)
}

func constOrVar(k vdl.Kind) string {
	switch k {
	case vdl.Bool, vdl.Byte, vdl.Uint16, vdl.Uint32, vdl.Uint64, vdl.Int16, vdl.Int32, vdl.Int64, vdl.Float32, vdl.Float64, vdl.Complex64, vdl.Complex128, vdl.String, vdl.Enum:
		return "const"
	}
	return "var"
}

func isByteList(t *vdl.Type) bool {
	return t.Kind() == vdl.List && t.Elem().Kind() == vdl.Byte
}

// TODO(bprosnitz): Generate the full tag name e.g. security.Read instead of
// security.Label(1)
//
// TODO(toddw): This doesn't work at all if v.Type() is a native type, or has
// subtypes that are native types.  It's also broken for optional types that
// can't be represented using a composite literal (e.g. optional primitives).
//
// https://github.com/veyron/release-issues/issues/1017
func typedConst(data goData, v *vdl.Value) string {
	k, t := v.Kind(), v.Type()
	if k == vdl.Optional {
		if elem := v.Elem(); elem != nil {
			return "&" + typedConst(data, elem)
		}
		return "(" + typeGo(data, t) + ")(nil)" // results in (*Foo)(nil)
	}
	valstr := untypedConst(data, v)
	// Enum and TypeObject already include the type in their values.
	// Built-in bool and string are implicitly convertible from literals.
	if k == vdl.Enum || k == vdl.TypeObject || t == vdl.BoolType || t == vdl.StringType {
		return valstr
	}
	// Everything else requires an explicit type.
	typestr := typeGo(data, t)
	// { } are used instead of ( ) for composites
	switch k {
	case vdl.Array, vdl.Struct:
		return typestr + valstr
	case vdl.List, vdl.Set, vdl.Map:
		// Special-case []byte, which we generate as a type conversion from string,
		// and empty variable-length collections, which we generate as a type
		// conversion from nil.
		if !isByteList(t) && !v.IsZero() {
			return typestr + valstr
		}
	}
	return typestr + "(" + valstr + ")"
}

func untypedConst(data goData, v *vdl.Value) string {
	k, t := v.Kind(), v.Type()
	if isByteList(t) {
		if v.IsZero() {
			return "nil"
		}
		return strconv.Quote(string(v.Bytes()))
	}
	switch k {
	case vdl.Any:
		if elem := v.Elem(); elem != nil {
			return typedConst(data, elem)
		}
		return "nil"
	case vdl.Optional:
		if elem := v.Elem(); elem != nil {
			return untypedConst(data, elem)
		}
		return "nil"
	case vdl.TypeObject:
		// We special-case Any and TypeObject, since they cannot be named by the
		// user, and are simple to return statically.
		switch v.TypeObject() {
		case vdl.AnyType:
			return data.Pkg("v.io/core/veyron2/vdl") + "AnyType"
		case vdl.TypeObjectType:
			return data.Pkg("v.io/core/veyron2/vdl") + "TypeObjectType"
		}
		// The strategy is either brilliant or a huge hack.  The "proper" way to do
		// this would be to generate the Go code that builds a *vdl.Type directly
		// using vdl.TypeBuilder, But the rest of this file can already generate the
		// Go code to construct a value of any other type, and vdl.TypeOf converts
		// from a Go value back into *vdl.Type.
		//
		// So we perform a roundtrip, converting from an in-memory *vdl.Type into
		// the Go code that represents the zero value for the type, back into a
		// *vdl.Type in the generated code via vdl.TypeOf.  This results in less
		// generator and generated code.
		zero := vdl.ZeroValue(v.TypeObject())
		return data.Pkg("v.io/core/veyron2/vdl") + "TypeOf(" + typedConst(data, zero) + ")"
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
	case vdl.Enum:
		return typeGo(data, t) + v.EnumLabel()
	case vdl.Array:
		if v.IsZero() && !t.ContainsKind(vdl.WalkInline, vdl.TypeObject, vdl.Union) {
			// We can't rely on the golang zero-value array if t contains inline
			// typeobject or union, since the golang zero-value for these types is
			// different from the vdl zero-value for these types.
			return "{}"
		}
		s := "{"
		for ix := 0; ix < v.Len(); ix++ {
			s += "\n" + untypedConst(data, v.Index(ix)) + ","
		}
		return s + "\n}"
	case vdl.List:
		if v.IsZero() {
			return "nil"
		}
		s := "{"
		for ix := 0; ix < v.Len(); ix++ {
			s += "\n" + untypedConst(data, v.Index(ix)) + ","
		}
		return s + "\n}"
	case vdl.Set, vdl.Map:
		if v.IsZero() {
			return "nil"
		}
		s := "{"
		for _, key := range vdl.SortValuesAsString(v.Keys()) {
			s += "\n" + subConst(data, key)
			if k == vdl.Set {
				s += ": struct{}{},"
			} else {
				s += ": " + untypedConst(data, v.MapIndex(key)) + ","
			}
		}
		return s + "\n}"
	case vdl.Struct:
		s := "{"
		hasFields := false
		for ix := 0; ix < t.NumField(); ix++ {
			vf := v.StructField(ix)
			if !vf.IsZero() || vf.Type().ContainsKind(vdl.WalkInline, vdl.TypeObject, vdl.Union) {
				// We can't rely on the golang zero-value for this field, even if it's a
				// vdl zero value, if the field contains inline typeobject or union,
				// since the golang zero-value for these types is different from the vdl
				// zero-value for these types.
				s += "\n" + t.Field(ix).Name + ": " + subConst(data, vf) + ","
				hasFields = true
			}
		}
		if hasFields {
			s += "\n"
		}
		return s + "}"
	case vdl.Union:
		ix, field := v.UnionField()
		return typeGo(data, t) + t.Field(ix).Name + "{" + typedConst(data, field) + "}"
	default:
		data.Env.Errorf(data.File, parse.Pos{}, "%v untypedConst not implemented for %v %v", t, k)
		return "INVALID"
	}
}

// subConst deals with a quirk regarding Go composite literals.  Go allows us to
// elide the type from composite literal Y when the type is implied; basically
// when Y is contained in another composite literal X.  However it requires the
// type for Y when X is a struct, and when X is a map and Y is the key.  As such
// subConst is called for map keys and struct fields.
func subConst(data goData, v *vdl.Value) string {
	switch v.Kind() {
	case vdl.Array, vdl.List, vdl.Set, vdl.Map, vdl.Struct, vdl.Optional:
		return typedConst(data, v)
	}
	return untypedConst(data, v)
}

func formatFloat(x float64, kind vdl.Kind) string {
	var bitSize int
	switch kind {
	case vdl.Float32, vdl.Complex64:
		bitSize = 32
	case vdl.Float64, vdl.Complex128:
		bitSize = 64
	default:
		panic(fmt.Errorf("vdl: formatFloat unhandled kind: %v", kind))
	}
	return strconv.FormatFloat(x, 'g', -1, bitSize)
}
