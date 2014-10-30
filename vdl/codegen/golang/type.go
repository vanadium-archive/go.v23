package golang

import (
	"fmt"
	"strconv"

	"veyron.io/veyron/veyron2/vdl"
	"veyron.io/veyron/veyron2/vdl/compile"
	"veyron.io/veyron/veyron2/vdl/vdlutil"
)

// testingMode is set to true to simplify tests.
var testingMode = false

func qualifiedIdent(data goData, ident string, file *compile.File) string {
	if file.Package == data.File.Package {
		// The identifier is from the same package - just use it.
		return ident
	}
	// The identifier is defined in a different package - qualify with the local
	// import package name.
	//
	// TODO(toddw): handle error if LookupLocal returns "".
	return data.UserImports.LookupLocal(file.Package.Path) + "." + ident
}

// typeGo translates vdl.Type into a Go type.
func typeGo(data goData, t *vdl.Type) string {
	if testingMode {
		if t.Name() != "" {
			return t.Name()
		}
	}
	// Terminate recursion at defined types, which include both user-defined types
	// (enum, struct, oneof) and built-in types.
	if def := data.Env.FindTypeDef(t); def != nil {
		switch {
		case t == vdl.AnyType:
			return "_gen_vdlutil.Any"
		case t == vdl.TypeObjectType:
			return "*_gen_vdl.Type"
		case def.File == compile.BuiltInFile:
			// Built-in primitives just use their name.
			return def.Name
		}
		return qualifiedIdent(data, def.Name, def.File)
	}
	// Otherwise recurse through the type.
	switch t.Kind() {
	case vdl.Nilable:
		return "*" + typeGo(data, t.Elem())
	case vdl.Array:
		return "[" + strconv.Itoa(t.Len()) + "]" + typeGo(data, t.Elem())
	case vdl.List:
		return "[]" + typeGo(data, t.Elem())
	case vdl.Set:
		return "map[" + typeGo(data, t.Key()) + "]struct{}"
	case vdl.Map:
		return "map[" + typeGo(data, t.Key()) + "]" + typeGo(data, t.Elem())
	default:
		panic(fmt.Errorf("vdl: typeGo unhandled type %v %v", t.Kind(), t))
	}
}

// typeDefGo prints the type definition for a type.
func typeDefGo(data goData, def *compile.TypeDef) string {
	s := fmt.Sprintf("%stype %s ", def.Doc, def.Name)
	switch t := def.Type; t.Kind() {
	case vdl.Enum:
		s += fmt.Sprintf("int%s\nconst (", def.DocSuffix)
		for ix := 0; ix < t.NumEnumLabel(); ix++ {
			s += fmt.Sprintf("\n\t%s%s%s", def.LabelDoc[ix], def.Name, t.EnumLabel(ix))
			if ix == 0 {
				s += fmt.Sprintf(" %s = iota", def.Name)
			}
			s += def.LabelDocSuffix[ix]
		}
		s += fmt.Sprintf("\n)"+
			"\n\n// %[2]s%[1]s holds all labels for %[1]s."+
			"\nvar %[2]s%[1]s = []%[1]s{%[4]s}"+
			"\n\n// %[3]s%[1]s creates a %[1]s from a string label."+
			"\n// Returns true iff the label is valid."+
			"\nfunc %[3]s%[1]s(label string) (x %[1]s, ok bool) {"+
			"\n\tok = x.Assign(label)"+
			"\n\treturn"+
			"\n}"+
			"\n\n// Assign assigns label to x."+
			"\n// Returns true iff the label is valid."+
			"\nfunc (x *%[1]s) Assign(label string) bool {"+
			"\n\tswitch label {",
			def.Name,
			vdlutil.FirstRuneToExportCase("all", def.Exported),
			vdlutil.FirstRuneToExportCase("make", def.Exported),
			commaEnumLabels(def.Name, t))
		for ix := 0; ix < t.NumEnumLabel(); ix++ {
			s += fmt.Sprintf("\n\tcase %[2]q:"+
				"\n\t\t*x = %[1]s%[2]s"+
				"\n\t\treturn true", def.Name, t.EnumLabel(ix))
		}
		s += fmt.Sprintf("\n\t}"+
			"\n\t*x = -1"+
			"\n\treturn false"+
			"\n}"+
			"\n\n// String returns the string label of x."+
			"\nfunc (x %[1]s) String() string {"+
			"\n\tswitch x {", def.Name)
		for ix := 0; ix < t.NumEnumLabel(); ix++ {
			s += fmt.Sprintf("\n\tcase %[1]s%[2]s:"+
				"\n\t\treturn %[2]q", def.Name, t.EnumLabel(ix))
		}
		s += fmt.Sprintf("\n\t}"+
			"\n\treturn \"\""+
			"\n}"+
			"\n\n// vdlEnumLabels identifies %[1]s as an enum."+
			"\nfunc (%[1]s) vdlEnumLabels(struct{ %[2]s bool }) {}",
			def.Name, commaEnumLabels("", t))
		return s
	case vdl.Struct:
		s += "struct {"
		for ix := 0; ix < t.NumField(); ix++ {
			f := t.Field(ix)
			s += "\n\t" + def.FieldDoc[ix] + f.Name + " "
			s += typeGo(data, f.Type) + def.FieldDocSuffix[ix]
		}
		s += "\n}"
		return s + def.DocSuffix
	case vdl.OneOf:
		s += fmt.Sprintf("struct{ oneof interface{} }%[2]s"+
			"\n\n// %[3]s%[1]s creates a %[1]s."+
			"\n// Returns true iff the oneof value has a valid type."+
			"\nfunc %[3]s%[1]s(oneof interface{}) (x %[1]s, ok bool) {"+
			"\n\tok = x.Assign(oneof)"+
			"\n\treturn"+
			"\n}"+
			"\n\n// Assign assigns oneof to x."+
			"\n// Returns true iff the oneof value has a valid type."+
			"\nfunc (x *%[1]s) Assign(oneof interface{}) bool {"+
			"\n\tswitch oneof.(type) {"+
			"\n\tcase %[4]s:"+
			"\n\t\tx.oneof = oneof"+
			"\n\t\treturn true"+
			"\n\t}"+
			"\n\tx.oneof = nil"+
			"\n\treturn false"+
			"\n}"+
			"\n\n// OneOf returns the underlying typed value of x."+
			"\nfunc (x %[1]s) OneOf() interface{} {"+
			"\n\treturn x.oneof"+
			"\n}"+
			"\n\n// vdlOneOfTypes identifies %[1]s as a oneof."+
			"\nfunc (%[1]s) vdlOneOfTypes(%[5]s) {}",
			def.Name,
			def.DocSuffix,
			vdlutil.FirstRuneToExportCase("make", def.Exported),
			commaOneOfTypes(data, "", t),
			commaOneOfTypes(data, "_ ", t))
		return s
	default:
		return s + typeGo(data, def.BaseType) + def.DocSuffix
	}
}

func commaEnumLabels(prefix string, t *vdl.Type) (s string) {
	for ix := 0; ix < t.NumEnumLabel(); ix++ {
		if ix > 0 {
			s += ", "
		}
		s += prefix
		s += t.EnumLabel(ix)
	}
	return
}

func commaOneOfTypes(data goData, prefix string, t *vdl.Type) (s string) {
	for ix := 0; ix < t.NumOneOfType(); ix++ {
		if ix > 0 {
			s += ", "
		}
		s += prefix
		s += typeGo(data, t.OneOfType(ix))
	}
	return
}

func embedGo(data goData, embed *compile.Interface) string {
	return qualifiedIdent(data, embed.Name, embed.File)
}
