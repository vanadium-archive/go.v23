package javascript

import (
	"fmt"

	"v.io/core/veyron2/vdl"
)

// makeTypeDefinitionsString generates a string that defines the specified types.
// It consists of the following sections:
// - Definitions. e.g. "var _typeNamedBool = new Type();"
// - Field assignments. e.g. "_typeNamedBool.name = \"NamedBool\";"
// - Type Freezes, e.g. "_typedNamedBool.freeze();"
// - Constructor definitions. e.g. "types.NamedBool = Registry.lookupOrCreateConstructor(_typeNamedBool)"
func makeTypeDefinitionsString(jsnames typeNames) string {
	str := ""
	sortedDefs := jsnames.SortedList()

	for _, def := range sortedDefs {
		str += makeDefString(def.Name)
	}

	for _, def := range sortedDefs {
		str += makeTypeFieldAssignmentString(def.Name, def.Type, jsnames)
	}

	for _, def := range sortedDefs {
		str += makeTypeFreezeString(def.Name)
	}

	for _, def := range sortedDefs {
		if def.Type.Name() != "" {
			str += makeConstructorDefinitionString(def.Type, jsnames)
		}
	}

	return str
}

// makeDefString generates a type definition for the specified type name.
// e.g. "var _typeNamedBool = new Type();"
func makeDefString(jsname string) string {
	return fmt.Sprintf("var %s = new vdl.Type();\n", jsname)
}

// makeTypeFreezeString calls the type's freeze function to finalize it.
// e.g. "typeNamedBool.freeze();"
func makeTypeFreezeString(jsname string) string {
	return fmt.Sprintf("%s.freeze();\n", jsname)
}

// makeTypeFieldAssignmentString generates assignments for type fields.
// e.g. "_typeNamedBool.name = \"NamedBool\";"
func makeTypeFieldAssignmentString(jsname string, t *vdl.Type, jsnames typeNames) string {
	// kind
	str := fmt.Sprintf("%s.kind = %s;\n", jsname, jsKind(t.Kind()))

	// name
	str += fmt.Sprintf("%s.name = %q;\n", jsname, t.Name())

	// labels
	if t.Kind() == vdl.Enum {
		str += fmt.Sprintf("%s.labels = [", jsname)
		for i := 0; i < t.NumEnumLabel(); i++ {
			if i > 0 {
				str += ", "
			}
			str += fmt.Sprintf("%q", t.EnumLabel(i))
		}
		str += "];\n"
	}

	// len
	if t.Kind() == vdl.Array { // Array is the only type where len is valid.
		str += fmt.Sprintf("%s.len = %d;\n", jsname, t.Len())
	}

	// elem
	switch t.Kind() {
	case vdl.Optional, vdl.Array, vdl.List, vdl.Map:
		str += fmt.Sprintf("%s.elem = %s;\n", jsname, jsnames.LookupType(t.Elem()))
	}

	// key
	switch t.Kind() {
	case vdl.Set, vdl.Map:
		str += fmt.Sprintf("%s.key = %s;\n", jsname, jsnames.LookupType(t.Key()))
	}

	// fields
	switch t.Kind() {
	case vdl.Struct, vdl.Union:
		str += fmt.Sprintf("%s.fields = [", jsname)
		for i := 0; i < t.NumField(); i++ {
			if i > 0 {
				str += ", "
			}
			field := t.Field(i)
			str += fmt.Sprintf("{name: %q, type: %s}", field.Name, jsnames.LookupType(field.Type))
		}
		str += "];\n"
	}

	return str
}

// makeConstructorDefinitionString creates a string that defines the constructor for the type.
// e.g. "module.exports.NamedBool = Registry.lookupOrCreateConstructor(_typeNamedBool)"
func makeConstructorDefinitionString(t *vdl.Type, jsnames typeNames) string {
	_, name := vdl.SplitIdent(t.Name())
	ctorName := jsnames.LookupConstructor(t)
	return fmt.Sprintf("module.exports.%s = %s;\n", name, ctorName)
}

func jsKind(k vdl.Kind) string {
	switch k {
	case vdl.Any:
		return "vdl.Kind.ANY"
	case vdl.Union:
		return "vdl.Kind.UNION"
	case vdl.Optional:
		return "vdl.Kind.OPTIONAL"
	case vdl.Bool:
		return "vdl.Kind.BOOL"
	case vdl.Byte:
		return "vdl.Kind.BYTE"
	case vdl.Uint16:
		return "vdl.Kind.UINT16"
	case vdl.Uint32:
		return "vdl.Kind.UINT32"
	case vdl.Uint64:
		return "vdl.Kind.UINT64"
	case vdl.Int16:
		return "vdl.Kind.INT16"
	case vdl.Int32:
		return "vdl.Kind.INT32"
	case vdl.Int64:
		return "vdl.Kind.INT64"
	case vdl.Float32:
		return "vdl.Kind.FLOAT32"
	case vdl.Float64:
		return "vdl.Kind.FLOAT64"
	case vdl.Complex64:
		return "vdl.Kind.COMPLEX64"
	case vdl.Complex128:
		return "vdl.Kind.COMPLEX128"
	case vdl.String:
		return "vdl.Kind.STRING"
	case vdl.Enum:
		return "vdl.Kind.ENUM"
	case vdl.TypeObject:
		return "vdl.Kind.TYPEOBJECT"
	case vdl.Array:
		return "vdl.Kind.ARRAY"
	case vdl.List:
		return "vdl.Kind.LIST"
	case vdl.Set:
		return "vdl.Kind.SET"
	case vdl.Map:
		return "vdl.Kind.MAP"
	case vdl.Struct:
		return "vdl.Kind.STRUCT"
	}
	panic(fmt.Errorf("val: unhandled kind: %d", k))
}

// builtinJSType indicates whether a vdl.Type has built-in type definition in vdl.js
// If true, then it returns a pointer to the type definition in javascript/types.js
// It assumes a variable named "vdl.Types" is already pointing to vom.Types
func builtinJSType(t *vdl.Type) (string, bool) {
	_, n := vdl.SplitIdent(t.Name())

	if t == vdl.ErrorType {
		return "vdl.Types.ERROR", true
	}

	// named types are not built-in.
	if n != "" {
		return "", false
	}

	// switch on supported types in vdl.js
	switch t.Kind() {
	case vdl.Any:
		return "vdl.Types.ANY", true
	case vdl.Bool:
		return "vdl.Types.BOOL", true
	case vdl.Byte:
		return "vdl.Types.BYTE", true
	case vdl.Uint16:
		return "vdl.Types.UINT16", true
	case vdl.Uint32:
		return "vdl.Types.UINT32", true
	case vdl.Uint64:
		return "vdl.Types.UINT64", true
	case vdl.Int16:
		return "vdl.Types.INT16", true
	case vdl.Int32:
		return "vdl.Types.INT32", true
	case vdl.Int64:
		return "vdl.Types.INT64", true
	case vdl.Float32:
		return "vdl.Types.FLOAT32", true
	case vdl.Float64:
		return "vdl.Types.FLOAT64", true
	case vdl.Complex64:
		return "vdl.Types.COMPLEX64", true
	case vdl.Complex128:
		return "vdl.Types.COMPLEX128", true
	case vdl.String:
		return "vdl.Types.STRING", true
	case vdl.TypeObject:
		return "vdl.Types.TYPEOBJECT", true
	}

	return "", false
}
