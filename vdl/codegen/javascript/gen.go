// Package javascript implements Javascript code generation from compiled VDL packages.
package javascript

// Generates the javascript source code for vdl files.  The generated output in javascript
// differs from most other languages, since we don't generate stubs. Instead generate an
// object that contains the parsed VDL structures that will be used by the Javascript code
// to valid servers.

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strconv"
	"text/template"

	"veyron.io/veyron/veyron2/vdl"
	"veyron.io/veyron/veyron2/vdl/codegen"
	"veyron.io/veyron/veyron2/vdl/compile"
	vdlroot "veyron.io/veyron/veyron2/vdl/vdlroot/src/vdl"
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
func Generate(pkg *compile.Package, env *compile.Env, genImport func(string) string, config vdlroot.JavascriptConfig) []byte {
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

func unTypedConst(d data, v *vdl.Value, unTypedFields bool) string {
	recursiveConst := func(d data, v *vdl.Value) string {
		if unTypedFields {
			return unTypedConst(d, v, unTypedFields)
		}
		return typedConst(d, v)
	}
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
			return recursiveConst(d, elem)
		}
		return "null"
	case vdl.Complex64, vdl.Complex128:
		return fmt.Sprintf("new Complex(%f, %f)", real(v.Complex()), imag(v.Complex()))
	case vdl.Enum:
		return fmt.Sprintf("'%s'", v.EnumLabel())
	case vdl.Array, vdl.List:
		s := "["
		isByteArray := v.Type().Elem().Kind() == vdl.Byte
		for ix := 0; ix < v.Len(); ix++ {
			var val string
			if isByteArray {
				val = unTypedConst(d, v.Index(ix), unTypedFields)
			} else {
				val = recursiveConst(d, v.Index(ix))
			}
			s += "\n" + val + ","
		}
		s += "\n]"
		if isByteArray {
			return "new Uint8Array(" + s + ")"
		}
		return s
	case vdl.Set:
		s := "new Set(["
		for _, key := range v.Keys() {
			s += "\n  " + unTypedConst(d, key, true) + ", "
		}
		return s + "])"
	case vdl.Map:
		s := "new Map(["
		for _, key := range v.Keys() {
			s += "\n  [" + unTypedConst(d, key, true) + ", "
			if v.Kind() == vdl.Set {
				s += "true"
			} else {
				s += recursiveConst(d, v.MapIndex(key))
			}
			s += "],"
		}
		return s + "])"
	case vdl.Struct:
		s := "{"
		t := v.Type()
		for ix := 0; ix < t.NumField(); ix++ {
			s += "\n  '" + vdlutil.ToCamelCase(t.Field(ix).Name) + "': " + recursiveConst(d, v.Field(ix)) + ","
		}
		return s + "\n}"

	}

	panic(fmt.Errorf("vdl: unTypedConst unhandled type %v %v", v.Kind(), v.Type()))
}

func primitiveWithOptionalName(primitive, name string) string {
	if name == "" {
		return "Types." + primitive
	}
	return "{kind: Kind." + primitive + ", name: '" + name + "'}"
}

func typeStruct(t *vdl.Type) string {
	nameField := ""
	if t.Name() != "" {
		nameField = "\n    name: '" + t.Name() + "',"
	}

	switch t.Kind() {
	case vdl.Any:
		return primitiveWithOptionalName("ANY", t.Name())
	case vdl.Bool:
		return primitiveWithOptionalName("BOOL", t.Name())
	case vdl.Byte:
		return primitiveWithOptionalName("BYTE", t.Name())
	case vdl.Uint16:
		return primitiveWithOptionalName("UINT16", t.Name())
	case vdl.Uint32:
		return primitiveWithOptionalName("UINT32", t.Name())
	case vdl.Uint64:
		return primitiveWithOptionalName("UINT64", t.Name())
	case vdl.Int16:
		return primitiveWithOptionalName("INT16", t.Name())
	case vdl.Int32:
		return primitiveWithOptionalName("INT32", t.Name())
	case vdl.Int64:
		return primitiveWithOptionalName("INT64", t.Name())
	case vdl.Float32:
		return primitiveWithOptionalName("FLOAT32", t.Name())
	case vdl.Float64:
		return primitiveWithOptionalName("FLOAT64", t.Name())
	case vdl.Complex64:
		return primitiveWithOptionalName("COMPLEX64", t.Name())
	case vdl.Complex128:
		return primitiveWithOptionalName("COMPLEX128", t.Name())
	case vdl.String:
		return primitiveWithOptionalName("STRING", t.Name())
	case vdl.Enum:
		labels := ""
		for i := 0; i < t.NumEnumLabel(); i++ {
			labels += "'" + t.EnumLabel(i) + "', "
		}
		return fmt.Sprintf(`{
    kind: Kind.ENUM,
    name: '%s',
    labels: [%s]
  }`, t.Name(), labels)
	case vdl.TypeObject:
		return "{}"

	// TODO(bjornick): Handle recursive types and de-duping of the same struct definitions over
	// and over again.
	case vdl.Array:
		return fmt.Sprintf(`{
    kind: Kind.ARRAY,%s
    elem: %s,
    len: %d
  }`, nameField, typeStruct(t.Elem()), t.Len())
	case vdl.List:
		return `{
    kind: Kind.LIST,` + nameField + `
    elem: ` + typeStruct(t.Elem()) + `
  }`
	case vdl.Set:
		return `{
    kind: Kind.SET,` + nameField + `
    key: ` + typeStruct(t.Key()) + `
  }`
	case vdl.Map:
		return fmt.Sprintf(`{
    kind: Kind.MAP,%s
    key: %s,
    elem: %s
  }`, nameField, typeStruct(t.Key()), typeStruct(t.Elem()))
	case vdl.Struct:
		fields := ""
		for i := 0; i < t.NumField(); i++ {
			f := t.Field(i)
			fields += fmt.Sprintf(`
    {
      name: '%s',
      type: %s
    },`, vdlutil.ToCamelCase(f.Name), typeStruct(f.Type))
		}
		return fmt.Sprintf(`{
    kind: Kind.STRUCT,
    name: '%s',
    fields: [%s
  ]}`, t.Name(), fields)
	case vdl.OneOf:
		types := ""
		for i := 0; i < t.NumOneOfType(); i++ {
			types += typeStruct(t.OneOfType(i)) + ", "
		}
		return fmt.Sprintf(`{
    kind: Kind.ONEOF,
    name: '%s',
    types: [%s]
  }`, t.Name(), types)
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
		case t == vdl.AnyType || t == vdl.TypeObjectType || t == compile.ErrorType:
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
	case vdl.Struct, vdl.Any:
		wrappedType = false
	}

	result := "function " + name + "(val) {"

	if wrappedType {
		result += "\n    this.val = val;\n"
	} else if kind == vdl.Struct {
		result += "\n    val = val || {};"
		for i := 0; i < t.Type.NumField(); i++ {
			field := t.Type.Field(i)
			name := vdlutil.ToCamelCase(field.Name)
			result += fmt.Sprintf(`
    if (val.hasOwnProperty('%s')) {
      this.%s = val.%s;
    } else {
      this.%s = %s;
    }`, name, name, name, name, defaultValue(data, field.Type))
		}
		result += "\n"
	} else if kind == vdl.Enum {
		// The default value of an enum type is the first label.
		// TODO(alexfandrianto): Isn't this case unreachable? Only vdl.Struct and
		// vdl.Any have wrappedType == false.
		result += "\n    this.val = '" + t.Type.EnumLabel(0) + "';\n"
	}
	result += "  }\n"
	result += name + ".prototype._type = " + typeStruct(t.Type) + ";\n"

	if wrappedType {
		result += name + ".prototype._wrappedType = true;\n"
	}
	result += "  types." + name + " = " + name + ";\n"

	return result
}

func typedConst(data data, v *vdl.Value) string {
	valstr := unTypedConst(data, v, false)
	t := v.Type()
	if def := data.Env.FindTypeDef(t); def != nil {
		if def.File != compile.BuiltInFile {
			return fmt.Sprintf("new %s(%s)", qualifiedIdent(data, def.Name, def.File), valstr)
		}
	}
	switch t.Kind() {
	case vdl.Bool:
		return "new Builtins.Bool(" + valstr + ")"
	case vdl.Byte:
		return "new Builtins.Byte(" + valstr + ")"
	case vdl.Uint16:
		return "new Builtins.Uint16(" + valstr + ")"
	case vdl.Uint32:
		return "new Builtins.Uint32(" + valstr + ")"
	case vdl.Uint64:
		return "new Builtins.Uint64(" + valstr + ")"
	case vdl.Int16:
		return "new Builtins.Int16(" + valstr + ")"
	case vdl.Int32:
		return "new Builtins.Int32(" + valstr + ")"
	case vdl.Int64:
		return "new Builtins.Int64(" + valstr + ")"
	case vdl.Float32:
		return "new Builtins.Float32(" + valstr + ")"
	case vdl.Float64:
		return "new Builtins.Float64(" + valstr + ")"
	case vdl.Complex64:
		return "new Builtins.Complex64(" + valstr + ")"
	case vdl.Complex128:
		return "new Builtins.Complex128(" + valstr + ")"
	case vdl.Array:
		return "new Builtins.Array(" + valstr + ", " + typeStruct(t) + ")"
	case vdl.List:
		return "new Builtins.List(" + valstr + ", " + typeStruct(t) + ")"
	case vdl.Set:
		return "new Builtins.Set(" + valstr + ", " + typeStruct(t) + ")"
	case vdl.Map:
		return "new Builtins.Map(" + valstr + ", " + typeStruct(t) + ")"
	}
	return valstr
}

func generateConstDefinition(data data, c *compile.ConstDef, typed bool) string {
	val := ""
	if typed {
		val = typedConst(data, c.Value)
	} else {
		val = unTypedConst(data, c.Value, true)
	}
	return fmt.Sprintf("  %s: %s,", c.Name, val)
}

func importPath(data data, path string) string {
	// We need to prefix all of these paths with a ./ to tell node that the path is relative to
	// the current directory.  Sadly filepath.Join(".", foo) == foo, so we have to do it
	// explicitly.
	return "." + string(filepath.Separator) + data.GenerateImport(path)

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
var vom = require('vom');
// TODO(bjornick): Remove unused imports.
var Types = vom.Types;
var Kind = vom.Kind;
var Complex = vom.Complex;
var Builtins = vom.Builtins;

{{$pkg := $data.Pkg}}
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

var types = {};
{{range $file := $pkg.Files}}{{range $type := $file.TypeDefs}}
{{generateTypeStub $data $type}}
{{end}}{{end}}

var typedConsts = { {{range $file := $pkg.Files}}{{range $const := $file.ConstDefs}}
{{generateConstDefinition $data $const true}}{{end}}{{end}}
};
var consts = { {{range $file := $pkg.Files}}{{range $const := $file.ConstDefs}}
{{generateConstDefinition $data $const false}}{{end}}{{end}}
};

module.exports = {
  types: types,
  services: services,
  consts: consts,
  typedConsts: typedConsts,
};
{{end}}`
