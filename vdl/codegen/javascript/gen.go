// Package javascript implements Javascript code generation from compiled VDL packages.
package javascript

// Generates the javascript source code for vdl files.  The generated output in javascript
// differs from most other languages, since we don't generate stubs. Instead generate an
// object that contains the parsed VDL structures that will be used by the Javascript code
// to valid servers.

import (
	"bytes"
	"fmt"
	"strconv"
	"text/template"

	"veyron.io/veyron/veyron2/vdl"
	"veyron.io/veyron/veyron2/vdl/codegen"
	"veyron.io/veyron/veyron2/vdl/compile"
	"veyron.io/veyron/veyron2/vdl/vdlutil"
)

type data struct {
	Pkg            *compile.Package
	Env            *compile.Env
	GenerateImport func(string) string
	UserImports    codegen.Imports
}

// Generate takes a populated compile.Package and produces a byte slice
// containing the generated Javascript code.
func Generate(pkg *compile.Package, env *compile.Env, genImport func(string) string) []byte {
	data := data{
		Pkg:            pkg,
		Env:            env,
		GenerateImport: genImport,
		UserImports:    codegen.ImportsForFiles(pkg.Files...),
	}
	var buf bytes.Buffer
	if err := javascriptTemplate.Execute(&buf, data); err != nil {
		// We shouldn't see an error; it means our template is buggy.
		panic(fmt.Errorf("vdl: couldn't execute template: %v", err))
	}
	return buf.Bytes()
}

var javascriptTemplate *template.Template

func numOutArgs(method *compile.Method) int {
	return len(method.OutArgs) - 1
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

func genMethodTags(data data, method *compile.Method) string {
	tags := method.Tags
	result := "["
	for _, tag := range tags {
		result += typedConst(data, tag) + ", "
	}
	result += "]"
	return result
}

func unTypedConst(data data, v *vdl.Value) string {
	switch v.Kind() {
	case vdl.Bool:
		if v.Bool() {
			return "true"
		} else {
			return "false"
		}
	case vdl.Byte:
		return strconv.FormatUint(uint64(v.Byte()), 10)
	case vdl.Uint16, vdl.Uint32, vdl.Uint64:
		return strconv.FormatUint(v.Uint(), 10)
	case vdl.Int16, vdl.Int32, vdl.Int64:
		return strconv.FormatInt(v.Int(), 10)
	case vdl.Float32, vdl.Float64:
		return strconv.FormatFloat(v.Float(), 'g', -1, bitlen(v.Kind()))
	case vdl.String:
		return strconv.Quote(v.RawString())
	case vdl.OneOf, vdl.Any, vdl.Nilable:
		if elem := v.Elem(); elem != nil {
			return typedConst(data, elem)
		}
		return "null"
	case vdl.Complex64, vdl.Complex128:
		return fmt.Sprintf("new Complex(%f, %f)", real(v.Complex()), imag(v.Complex()))
	case vdl.Enum:
		return strconv.FormatInt(int64(v.EnumIndex()), 10)
	case vdl.Array, vdl.List:
		s := "["
		for ix := 0; ix < v.Len(); ix++ {
			s += "\n" + typedConst(data, v.Index(ix)) + ","
		}
		return s + "\n]"
	case vdl.Set, vdl.Map:
		s := "{"
		for _, key := range v.Keys() {
			s += "\n  " + unTypedConst(data, key) + ":"
			if v.Kind() == vdl.Set {
				s += "true"
			} else {
				s += typedConst(data, v.MapIndex(key))
			}
			s += ","
			// TODO(bjornick): Deal with non-string keys.
		}
		s += "}"
		return s
	case vdl.Struct:
		s := "{"
		t := v.Type()
		for ix := 0; ix < t.NumField(); ix++ {
			s += "\n  '" + t.Field(ix).Name + "': " + typedConst(data, v.Field(ix)) + ","
		}
		return s + "\n}"

	}

	panic(fmt.Errorf("vdl: unTypedConst unhandled type %v %v", v.Kind(), v.Type()))
}

func typeStruct(t *vdl.Type) string {
	switch t.Kind() {
	case vdl.Any:
		return "Types.ANY"
	case vdl.Bool:
		return "Types.BOOL"
	case vdl.Byte:
		return "Types.BYTE"
	case vdl.Uint16:
		return "Types.UINT16"
	case vdl.Uint32:
		return "Types.UINT32"
	case vdl.Uint64:
		return "Types.UINT64"
	case vdl.Int16:
		return "Types.INT16"
	case vdl.Int32:
		return "Types.INT32"
	case vdl.Int64:
		return "Types.INT64"
	case vdl.Float32:
		return "Types.FLOAT32"
	case vdl.Float64:
		return "Types.FLOAT64"
	case vdl.Complex64:
		return "Types.COMPLEX64"
	case vdl.Complex128:
		return "Types.COMPLEX128"
	case vdl.String:
		return "Types.STRING"
	case vdl.Enum:
		enumRes := `{
    kind: Kind.ENUM,
    name: '` + t.Name() + `',
    labels: [`
		for i := 0; i < t.NumEnumLabel(); i++ {
			enumRes += "'" + t.EnumLabel(i) + "', "
		}
		enumRes += `]
  }`
		return enumRes
	case vdl.TypeVal:
		return "{}"

	// TODO(bjornick): Handle recursive types and de-duping of the same struct definitions over
	// and over again.
	case vdl.Array:
		return fmt.Sprintf(`{
    kind: Kind.ARRAY,
    elem: %s,
    len: %d
  }`, typeStruct(t.Elem()), t.Len())
	case vdl.List:
		return `{
    kind: Kind.LIST,
    elem: ` + typeStruct(t.Elem()) + `
  }`
	case vdl.Set:
		return `{
    kind: Kind.SET,
    key: ` + typeStruct(t.Key()) + `
  }`
	case vdl.Map:
		return fmt.Sprintf(`{
    kind: Kind.MAP,
    key: %s,
    elem: %s
  }`, typeStruct(t.Key()), typeStruct(t.Elem()))
	case vdl.Struct:
		result := `{
    kind: Kind.STRUCT,
    name: '` + t.Name() + `',
    fields: [`
		for i := 0; i < t.NumField(); i++ {
			f := t.Field(i)
			result += fmt.Sprintf(`
    {
      name: '%s',
      type: %s
    },`, f.Name, typeStruct(f.Type))
		}
		result += "\n  ]}"
		return result
	}
	return ""
}

func qualifiedIdent(data data, name string, file *compile.File) string {
	if data.Pkg.Path == file.Package.Path {
		return name
	}
	return data.UserImports.LookupLocal(file.Package.Path) + "." + name
}

func defaultValue(data data, t *vdl.Type) string {
	if def := data.Env.FindTypeDef(t); def != nil {
		switch {
		case t == vdl.AnyType || t == vdl.TypeValType || t == compile.ErrorType:
			return "null"
		case def.File != compile.BuiltInFile:
			return "new " + qualifiedIdent(data, def.Name, def.File) + "()"
		}
	}
	switch t.Kind() {
	case vdl.Nilable:
		return "null"
	case vdl.Array, vdl.List:
		return "[]"
	case vdl.Set, vdl.Map:
		return "{}"
	case vdl.Byte, vdl.Uint16, vdl.Uint32, vdl.Uint64, vdl.Int16, vdl.Int32, vdl.Int64, vdl.Float32, vdl.Float64:
		return "0"
	case vdl.Complex64, vdl.Complex128:
		return "new Complex(0, 0)"
	case vdl.String:
		return "''"
	case vdl.Bool:
		return "false"
	}
	panic(fmt.Sprintf("Unhandled type %v", t))
}

func generateTypeStub(data data, t *compile.TypeDef) string {
	name := t.Name
	wrappedType := true
	kind := t.Type.Kind()
	// We can't always have Map be a wrapped type, since the key doesn't
	// have to be a string.
	// TODO(bjornick): Handle maps without string keys.
	switch kind {
	case vdl.Struct, vdl.Any, vdl.Enum:
		wrappedType = false
	}

	result := "function " + name + "("
	if wrappedType {
		result += "val"
	}
	result += ") {"

	if wrappedType {
		result += "\n    this.val = val;\n"
	} else if kind == vdl.Struct {
		for i := 0; i < t.Type.NumField(); i++ {
			field := t.Type.Field(i)
			result += "\n    this." + field.Name + " = " + defaultValue(data, field.Type) + ";"
		}
		result += "\n"
	} else if kind == vdl.Enum {
		// The default value of an enum type is the first label which is mapped
		// to 0.
		result += "\n    this.val = 0;\n"
	}
	result += "  }\n"
	result += name + ".prototype._type = " + typeStruct(t.Type) + ";\n"

	result += "  types." + name + " = " + name + ";\n"

	return result
}

func typedConst(data data, v *vdl.Value) string {
	valstr := unTypedConst(data, v)
	t := v.Type()
	if def := data.Env.FindTypeDef(t); def != nil {
		if def.File != compile.BuiltInFile {
			return fmt.Sprintf("new %s(%s)", qualifiedIdent(data, def.Name, def.File), valstr)
		}
	}
	return valstr
}

func generateConstDefinition(data data, c *compile.ConstDef) string {
	return fmt.Sprintf("  %s: %s,", c.Name, typedConst(data, c.Value))
}

func importPath(data data, path string) string {
	return data.GenerateImport(path)

}

func init() {
	funcMap := template.FuncMap{
		"toCamelCase":             vdlutil.ToCamelCase,
		"numOutArgs":              numOutArgs,
		"genMethodTags":           genMethodTags,
		"generateTypeStub":        generateTypeStub,
		"generateConstDefinition": generateConstDefinition,
		"importPath":              importPath,
	}
	javascriptTemplate = template.Must(template.New("genJS").Funcs(funcMap).Parse(genJS))
}

// The template that we execute against a compile.Package instance to generate our
// code.  Most of this is fairly straightforward substitution and ranges; more
// complicated logic is delegated to the helper functions above.
//
// We try to generate code that has somewhat reasonable formatting.
const genJS = `{{with $data := .}}// This file was auto-generated by the veyron vdl tool.
{{$pkg := $data.Pkg}}
var veryon = require('veyron');
{{if $data.UserImports}}{{range $imp := $data.UserImports}}
var {{$imp.Local}} = require('{{importPath $data $imp.Path}}');{{end}}{{end}}
var services = {
package: '{{$pkg.Path}}',
{{range $file := $pkg.Files}}{{range $iface := $file.Interfaces}}  {{$iface.Name}}: {
{{range $method := $iface.AllMethods}}    {{toCamelCase $method.Name}}: {
    numInArgs: {{len $method.InArgs}},
    numOutArgs: {{numOutArgs $method}},
    inputStreaming: {{if $method.InStream}}true{{else}}false{{end}},
    outputStreaming: {{if $method.OutStream}}true{{else}}false{{end}},
    tags: {{genMethodTags $data $method}}
},
{{end}}
},
{{end}}{{end}}
};
var Types = veyron.vom.Types;
var Kind = veyron.vom.Kind;
var Complex = veyron.vom.Complex;

var types = {};
{{range $file := $pkg.Files}}{{range $type := $file.TypeDefs}}
{{generateTypeStub $data $type}}
{{end}}{{end}}

var consts = { {{range $file := $pkg.Files}}{{range $const := $file.ConstDefs}}
{{generateConstDefinition $data $const}}{{end}}{{end}}
};
module.exports = {
  types: types,
  services: services,
  consts: consts,
};
{{end}}`
