package vdlgen

// TODO(toddw): Add tests

import (
	"fmt"
	"strconv"

	"veyron.io/veyron/veyron2/vdl"
	"veyron.io/veyron/veyron2/vdl/codegen"
)

// Type returns the vdl type corresponding to t, in the given pkgPath, with the
// given imports.
func Type(t *vdl.Type, pkgPath string, imports codegen.Imports) (string, error) {
	// Named types are always referred to by their name.
	if t.Name() != "" {
		path, name := vdl.SplitIdent(t.Name())
		if path == "" || path == pkgPath {
			return name, nil
		}
		if local := imports.LookupLocal(path); local != "" {
			return local + "." + name, nil
		}
		return "", fmt.Errorf("vdl: type %q has unknown pkg path %q in imports %v", t.Name(), path, imports)
	}
	// Otherwise deal with built-in and composite types.
	switch t.Kind() {
	case vdl.Any, vdl.Bool, vdl.Byte, vdl.Uint16, vdl.Uint32, vdl.Uint64, vdl.Int16, vdl.Int32, vdl.Int64, vdl.Float32, vdl.Float64, vdl.Complex64, vdl.Complex128, vdl.String, vdl.TypeObject:
		// Built-in types are named the same as their kind.
		return t.Kind().String(), nil
	case vdl.Nilable:
		elem, err := Type(t.Elem(), pkgPath, imports)
		if err != nil {
			return "", err
		}
		return "?" + elem, nil
	case vdl.Array:
		elem, err := Type(t.Elem(), pkgPath, imports)
		if err != nil {
			return "", err
		}
		return "[" + strconv.Itoa(t.Len()) + "]" + elem, nil
	case vdl.List:
		elem, err := Type(t.Elem(), pkgPath, imports)
		if err != nil {
			return "", err
		}
		return "[]" + elem, nil
	case vdl.Set:
		key, err := Type(t.Key(), pkgPath, imports)
		if err != nil {
			return "", err
		}
		return "set[" + key + "]", nil
	case vdl.Map:
		key, err := Type(t.Key(), pkgPath, imports)
		if err != nil {
			return "", err
		}
		elem, err := Type(t.Elem(), pkgPath, imports)
		if err != nil {
			return "", err
		}
		return "map[" + key + "]" + elem, nil
	}
	// Enum, OneOf and Struct must be named.
	return "", fmt.Errorf("vdl: types of kind %q must be named", t.Kind())
}
