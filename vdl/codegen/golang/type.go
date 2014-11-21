package golang

import (
	"fmt"
	"strconv"

	"veyron.io/veyron/veyron2/vdl"
	"veyron.io/veyron/veyron2/vdl/compile"
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
			return "__vdlutil.Any"
		case t == vdl.TypeObjectType:
			return "*__vdl.Type"
		case def.File == compile.BuiltInFile:
			// Built-in primitives just use their name.
			return def.Name
		}
		return qualifiedIdent(data, def.Name, def.File)
	}
	// Otherwise recurse through the type.
	switch t.Kind() {
	case vdl.Optional:
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
			"\n\n// %[1]sAll holds all labels for %[1]s."+
			"\nvar %[1]sAll = []%[1]s{%[2]s}"+
			"\n\n// %[1]sFromString creates a %[1]s from a string label."+
			"\n// Returns true iff the label is valid."+
			"\nfunc %[1]sFromString(label string) (x %[1]s, ok bool) {"+
			"\n\tok = x.Assign(label)"+
			"\n\treturn"+
			"\n}"+
			"\n\n// Assign assigns label to x."+
			"\n// Returns true iff the label is valid."+
			"\nfunc (x *%[1]s) Assign(label string) bool {"+
			"\n\tswitch label {",
			def.Name,
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
			"\n\n// __DescribeEnum describes the %[1]s enum type."+
			"\nfunc (%[1]s) __DescribeEnum(struct{ %[2]s %[1]s }) {}",
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
		s = fmt.Sprintf("type ("+
			"\n\t// %[1]s represents any single field of the %[1]s oneof type."+
			"\n\t%[2]s%[1]s interface {"+
			"\n\t\t// Index returns the field index."+
			"\n\t\tIndex() int"+
			"\n\t\t// Name returns the field name."+
			"\n\t\tName() string"+
			"\n\t\t// __DescribeOneOf describes the %[1]s oneof type."+
			"\n\t\t__DescribeOneOf(__%[1]sDesc)"+
			"\n\t}%[3]s", def.Name, docBreak(def.Doc), def.DocSuffix)
		for ix := 0; ix < t.NumField(); ix++ {
			f := t.Field(ix)
			s += fmt.Sprintf("\n\t// %[1]s%[2]s represents field %[2]s of the %[1]s oneof type."+
				"\n\t%[4]s%[1]s%[2]s struct{ Value %[3]s }%[5]s",
				def.Name, f.Name, typeGo(data, f.Type),
				docBreak(def.FieldDoc[ix]), def.FieldDocSuffix[ix])
		}
		s += fmt.Sprintf("\n\t// __%[1]sDesc describes the %[1]s oneof type."+
			"\n\t__%[1]sDesc struct {"+
			"\n\t\t%[1]s", def.Name)
		for ix := 0; ix < t.NumField(); ix++ {
			s += fmt.Sprintf("\n\t\t%[2]s %[1]s%[2]s", def.Name, t.Field(ix).Name)
		}
		s += fmt.Sprintf("\n\t}\n)")
		for ix := 0; ix < t.NumField(); ix++ {
			s += fmt.Sprintf("\n\nfunc (%[1]s%[2]s) Index() int { return %[3]d }"+
				"\nfunc (%[1]s%[2]s) Name() string { return \"%[2]s\" }"+
				"\nfunc (%[1]s%[2]s) __DescribeOneOf(__%[1]sDesc) {}",
				def.Name, t.Field(ix).Name, ix)
		}
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

func embedGo(data goData, embed *compile.Interface) string {
	return qualifiedIdent(data, embed.Name, embed.File)
}
