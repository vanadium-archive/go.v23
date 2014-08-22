package golang

import (
	"fmt"
	"strconv"

	"veyron2/vdl"
	"veyron2/vdl/compile"
)

func constDefGo(data goData, def *compile.ConstDef) string {
	return def.Doc + constGo(data, def.Name, def.Value) + def.DocSuffix
}

func constGo(data goData, name string, v *vdl.Value) string {
	// Handle nil values.
	if v.IsNil() {
		return fmt.Sprintf("var %s %s = nil", name, typeGo(data, v.Type()))
	}
	// Handle values that are actually defined as Go constants.
	if k := v.Kind(); isBoolStringOrNumber(k) || k == vdl.Enum {
		return fmt.Sprintf("const %s = %s", name, typedValueGo(data, v))
	}
	// Handle values that are actually defined as Go variables.
	return fmt.Sprintf("var %s = %s", name, valueGo(data, v))
}

func isBoolStringOrNumber(k vdl.Kind) bool {
	switch k {
	case vdl.Bool, vdl.String, vdl.Byte, vdl.Uint16, vdl.Uint32, vdl.Uint64, vdl.Int16, vdl.Int32, vdl.Int64, vdl.Float32, vdl.Float64, vdl.Complex64, vdl.Complex128:
		return true
	}
	return false
}

func valueGo(data goData, v *vdl.Value) string {
	k, t := v.Kind(), v.Type()
	if t.IsBytes() && k == vdl.List {
		return typeGo(data, t) + "(" + strconv.Quote(string(v.Bytes())) + ")"
	}
	switch k {
	case vdl.Any:
		panic("TODO: Any constants aren't supported yet!")
	case vdl.OneOf:
		panic("TODO: OneOf constants aren't supported yet!")
	case vdl.Nilable:
		panic("TODO: Nilable constants aren't supported yet!")
	case vdl.Bool:
		return strconv.FormatBool(v.Bool())
	case vdl.Byte:
		return strconv.FormatUint(uint64(v.Byte()), 10)
	case vdl.Uint16, vdl.Uint32, vdl.Uint64:
		return strconv.FormatUint(v.Uint(), 10)
	case vdl.Int16, vdl.Int32, vdl.Int64:
		return strconv.FormatInt(v.Int(), 10)
	case vdl.Float32, vdl.Float64:
		return strconv.FormatFloat(v.Float(), 'g', -1, bitlen(k))
	case vdl.Complex64, vdl.Complex128:
		c := v.Complex()
		s := strconv.FormatFloat(real(c), 'g', -1, bitlen(k)) + "+"
		s += strconv.FormatFloat(imag(c), 'g', -1, bitlen(k)) + "i"
		return s
	case vdl.String:
		return strconv.Quote(v.RawString())
	case vdl.Enum:
		return typeGo(data, t) + v.EnumLabel()
	case vdl.TypeVal:
		panic("TODO: TypeVal constants aren't supported yet!")
	case vdl.Array, vdl.List:
		s := typeGo(data, t) + "{"
		for ix := 0; ix < v.Len(); ix++ {
			if ix > 0 {
				s += ", "
			}
			s += valueGo(data, v.Index(ix))
		}
		return s + "}"
	case vdl.Set, vdl.Map:
		// TODO(toddw): Sort keys to get a deterministic ordering.
		s := typeGo(data, t) + "{"
		for ix, key := range v.Keys() {
			if ix > 0 {
				s += ", "
			}
			s += valueGo(data, key)
			if k == vdl.Set {
				s += ": struct{}{}"
			} else {
				elem := v.MapIndex(key)
				s += ": " + valueGo(data, elem)
			}
		}
		return s + "}"
	case vdl.Struct:
		s := typeGo(data, t) + "{"
		for ix := 0; ix < t.NumField(); ix++ {
			if ix > 0 {
				s += ", "
			}
			s += t.Field(ix).Name + ": " + valueGo(data, v.Field(ix))
		}
		return s + "}"
	default:
		panic(fmt.Errorf("vdl: valueGo unhandled kind %v", k))
	}
}

// TODO(bprosnitz): Generate the full tag name e.g. security.Read instead of
// security.Label(1)
func typedValueGo(data goData, v *vdl.Value) string {
	// Bool, string and numbers aren't given an explicit type in valueGo, so we
	// add the type here.  We don't add the type for the built-in bool and string
	// types - when they're used as top-level constants they're untyped, and when
	// they're used as top-level values they are implicitly converted to bool and
	// string anyways.
	if isBoolStringOrNumber(v.Kind()) {
		if t := v.Type(); t != vdl.BoolType && t != vdl.StringType {
			return typeGo(data, t) + "(" + valueGo(data, v) + ")"
		}
	}
	return valueGo(data, v)
}

func bitlen(kind vdl.Kind) int {
	switch kind {
	case vdl.Float32, vdl.Complex64:
		return 32
	case vdl.Float64, vdl.Complex128:
		return 64
	}
	panic(fmt.Errorf("vdl: bitLen unhandled kind %v", kind))
}

func tagsGo(data goData, tags []*vdl.Value) string {
	str := "[]interface{}{"
	for ix, tag := range tags {
		if ix > 0 {
			str += ", "
		}
		str += typedValueGo(data, tag)
	}
	return str + "}"
}
