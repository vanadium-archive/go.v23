package java

import (
	"fmt"
	"strconv"
	"strings"

	"veyron.io/veyron/veyron2/vdl"
	"veyron.io/veyron/veyron2/vdl/compile"
)

// javaConstVal returns the value string for the provided constant value.
func javaConstVal(v *vdl.Value, env *compile.Env) (ret string) {
	ret = javaVal(v, env)
	switch v.Type().Kind() {
	case vdl.Complex64, vdl.Complex128, vdl.Enum, vdl.OneOf, vdl.Uint16, vdl.Uint32, vdl.Uint64:
		return
	}
	if def := env.FindTypeDef(v.Type()); def != nil && def.File != compile.BuiltInFile { // User-defined type.
		ret = fmt.Sprintf("new %s(%s)", javaType(v.Type(), false, env), ret)
	}
	return
}

// javaVal returns the value string for the provided Value.
func javaVal(v *vdl.Value, env *compile.Env) string {
	const longSuffix = "L"
	const floatSuffix = "f"

	switch v.Kind() {
	case vdl.Bool:
		if v.Bool() {
			return "true"
		} else {
			return "false"
		}
	case vdl.Byte:
		return "(byte)" + strconv.FormatUint(uint64(v.Byte()), 10)
	case vdl.Uint16:
		return fmt.Sprintf("new %s((short) %s)", javaType(v.Type(), true, env), strconv.FormatUint(v.Uint(), 10))
	case vdl.Int16:
		return "(short)" + strconv.FormatInt(v.Int(), 10)
	case vdl.Uint32:
		return fmt.Sprintf("new %s(%s)", javaType(v.Type(), true, env), strconv.FormatUint(v.Uint(), 10))
	case vdl.Int32:
		return strconv.FormatInt(v.Int(), 10)
	case vdl.Uint64:
		return fmt.Sprintf("new %s(%s)", javaType(v.Type(), true, env), strconv.FormatUint(v.Uint(), 10)+longSuffix)
	case vdl.Int64:
		return strconv.FormatInt(v.Int(), 10) + longSuffix
	case vdl.Float32, vdl.Float64:
		c := strconv.FormatFloat(v.Float(), 'g', -1, bitlen(v.Kind()))
		if strings.Index(c, ".") == -1 {
			c += ".0"
		}
		if v.Kind() == vdl.Float32 {
			return c + floatSuffix
		}
		return c
	case vdl.Complex64, vdl.Complex128:
		r := strconv.FormatFloat(real(v.Complex()), 'g', -1, bitlen(v.Kind()))
		i := strconv.FormatFloat(imag(v.Complex()), 'g', -1, bitlen(v.Kind()))
		if v.Kind() == vdl.Complex64 {
			r = r + "f"
			i = i + "f"
		}
		return fmt.Sprintf("new %s(%s, %s)", javaType(v.Type(), true, env), r, i)
	case vdl.String:
		return strconv.Quote(v.RawString())
	case vdl.Any:
		elemReflectTypeStr := javaReflectType(v.Elem().Type(), env)
		elemStr := javaConstVal(v.Elem(), env)
		return fmt.Sprintf("new %s(%s, %s)", javaType(v.Type(), false, env), elemReflectTypeStr, elemStr)
	case vdl.Array:
		ret := fmt.Sprintf("new %s[] {", javaType(v.Type().Elem(), true, env))
		for i := 0; i < v.Len(); i++ {
			if i > 0 {
				ret = ret + ", "
			}
			ret = ret + javaConstVal(v.Index(i), env)
		}
		return ret + "}"
	case vdl.Enum:
		return fmt.Sprintf("%s.%s", javaType(v.Type(), false, env), v.EnumLabel())
	case vdl.List:
		elemTypeStr := javaType(v.Type().Elem(), true, env)
		ret := fmt.Sprintf("new com.google.common.collect.ImmutableList.Builder<%s>()", elemTypeStr)
		for i := 0; i < v.Len(); i++ {
			ret = fmt.Sprintf("%s.add(%s)", ret, javaConstVal(v.Index(i), env))
		}
		return ret + ".build()"
	case vdl.Map:
		keyTypeStr := javaType(v.Type().Key(), true, env)
		elemTypeStr := javaType(v.Type().Elem(), true, env)
		ret := fmt.Sprintf("new com.google.common.collect.ImmutableMap.Builder<%s, %s>()", keyTypeStr, elemTypeStr)
		for _, key := range v.Keys() {
			keyStr := javaConstVal(key, env)
			elemStr := javaConstVal(v.MapIndex(key), env)
			ret = fmt.Sprintf("%s.put(%s, %s)", ret, keyStr, elemStr)
		}
		return ret + ".build()"
	case vdl.OneOf:
		elemReflectTypeStr := javaReflectType(v.Elem().Type(), env)
		elemStr := javaConstVal(v.Elem(), env)
		return fmt.Sprintf("new %s().assignValue(%s, %s)", javaType(v.Type(), false, env), elemReflectTypeStr, elemStr)
	case vdl.Set:
		keyTypeStr := javaType(v.Type().Key(), true, env)
		ret := fmt.Sprintf("new com.google.common.collect.ImmutableSet.Builder<%s>()", keyTypeStr)
		for _, key := range v.Keys() {
			ret = fmt.Sprintf("%s.add(%s)", ret, javaConstVal(key, env))
		}
		return ret + ".build()"
	case vdl.Struct:
		var ret string
		for i := 0; i < v.Type().NumField(); i++ {
			if i > 0 {
				ret = ret + ", "
			}
			ret = ret + javaConstVal(v.Field(i), env)
		}
		return ret
	case vdl.TypeObject:
		return fmt.Sprintf("new %s(%s)", javaType(v.Type(), false, env), javaReflectType(v.TypeObject(), env))
	}
	if v.Type().IsBytes() {
		return strconv.Quote(string(v.Bytes()))
	}
	panic(fmt.Errorf("vdl: javaVal unhandled type %v %v", v.Kind(), v.Type()))
}
