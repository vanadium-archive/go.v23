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
	TypeNames      typeNames
}

// Generate takes a populated compile.Package and produces a byte slice
// containing the generated Javascript code.
func Generate(pkg *compile.Package, env *compile.Env, genImport func(string) string, config vdlroot.JavascriptConfig) []byte {
	data := data{
		Pkg:            pkg,
		Env:            env,
		GenerateImport: genImport,
		UserImports:    codegen.ImportsForFiles(pkg.Files...),
		TypeNames:      newTypeNames(pkg),
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

func genMethodTags(names typeNames, method *compile.Method) string {
	tags := method.Tags
	result := "["
	for _, tag := range tags {
		result += typedConst(names, tag) + ", "
	}
	result += "]"
	return result
}

// untypedConst generates a javascript string representing a constant that is
// not wrapped with type information.
func untypedConst(names typeNames, v *vdl.Value) string {
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
	case vdl.Any:
		if v.Elem() != nil {
			return typedConst(names, v.Elem())
		}
		return "null"
	case vdl.Optional:
		if v.Elem() != nil {
			return untypedConst(names, v.Elem())
		}
		return "null"
	case vdl.Complex64, vdl.Complex128:
		return fmt.Sprintf("new Complex(%f, %f)", real(v.Complex()), imag(v.Complex()))
	case vdl.Enum:
		return fmt.Sprintf("'%s'", v.EnumLabel())
	case vdl.Array, vdl.List:
		result := "["
		isByteArray := v.Type().Elem().Kind() == vdl.Byte
		for ix := 0; ix < v.Len(); ix++ {
			var val string
			val = untypedConst(names, v.Index(ix))
			result += "\n" + val + ","
		}
		result += "\n]"
		if isByteArray {
			return "new Uint8Array(" + result + ")"
		}
		return result
	case vdl.Set:
		result := "new Set(["
		for _, key := range v.Keys() {
			result += "\n  " + untypedConst(names, key) + ", "
		}
		result += "])"
		return result
	case vdl.Map:
		result := "new Map(["
		for i, key := range v.Keys() {
			if i > 0 {
				result += ","
			}
			result += fmt.Sprintf("\n  [%s, %s]",
				untypedConst(names, key),
				untypedConst(names, v.MapIndex(key)))

		}
		result += "])"
		return result
	case vdl.Struct:
		result := "{"
		t := v.Type()
		for ix := 0; ix < t.NumField(); ix++ {
			result += "\n  '" +
				vdlutil.ToCamelCase(t.Field(ix).Name) +
				"': " +
				untypedConst(names, v.Field(ix)) +
				","
		}
		return result + "\n}"
	case vdl.OneOf:
		ix, innerVal := v.OneOfField()
		return fmt.Sprintf("{ %q: %v }", v.Type().Field(ix).Name, untypedConst(names, innerVal))
	case vdl.TypeObject:
		return names.LookupName(v.TypeObject())
	default:
		panic(fmt.Errorf("vdl: untypedConst unhandled type %v %v", v.Kind(), v.Type()))
	}
}

func primitiveWithOptionalName(primitive, name string) string {
	if name == "" {
		return "Types." + primitive
	}
	return "new vom.Type({kind: Kind." + primitive + ", name: '" + name + "'})"
}

// typedConst returns a javascript string representing a const that is always
// wrapped with type information
func typedConst(names typeNames, v *vdl.Value) string {
	switch v.Kind() {
	case vdl.Any:
		return untypedConst(names, v)
	default:
		return fmt.Sprintf("new (Registry.lookupOrCreateConstructor(%s))(%s)",
			names.LookupName(v.Type()),
			untypedConst(names, v))
	}
}

func importPath(data data, path string) string {
	// We need to prefix all of these paths with a ./ to tell node that the path is relative to
	// the current directory.  Sadly filepath.Join(".", foo) == foo, so we have to do it
	// explicitly.
	return "." + string(filepath.Separator) + data.GenerateImport(path)

}

func init() {
	funcMap := template.FuncMap{
		"toCamelCase":               vdlutil.ToCamelCase,
		"numOutArgs":                numOutArgs,
		"genMethodTags":             genMethodTags,
		"makeTypeDefinitionsString": makeTypeDefinitionsString,
		"typedConst":                typedConst,
		"importPath":                importPath,
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
var Type = vom.Type;
var Kind = vom.Kind;
var Complex = vom.Complex;
var Builtins = vom.Builtins;
var Registry = vom.Registry;

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
    tags: {{genMethodTags $data.TypeNames $method }}
},
{{end}}
},
{{end}}{{end}}
};

var types = {};
{{makeTypeDefinitionsString $data.TypeNames }}

var consts = { {{range $file := $pkg.Files}}{{range $const := $file.ConstDefs}}
  {{$const.Name}}: {{typedConst $data.TypeNames $const.Value}},{{end}}{{end}}
};

module.exports = {
  types: types,
  services: services,
  consts: consts,
};
{{end}}`
