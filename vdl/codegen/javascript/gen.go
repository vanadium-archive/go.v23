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
	"veyron.io/veyron/veyron2/vdl/compile"
	"veyron.io/veyron/veyron2/vdl/vdlutil"
)

type data struct {
	Pkg *compile.Package
	Env *compile.Env
}

// Generate takes a populated compile.Package and produces a byte slice
// containing the generated Javascript code.
func Generate(pkg *compile.Package, env *compile.Env) []byte {
	data := data{
		Pkg: pkg,
		Env: env,
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

func typeJS(data data, t *vdl.Type) string {
	// This only handles primitive types.
	if def := data.Env.FindTypeDef(t); def != nil {
		if def.File == compile.BuiltInFile {
			return def.Name
		}
		return def.File.Package.Path + "." + def.Name
	}
	// TODO(bjornick): Handle anonymous/recursive types.
	panic(fmt.Errorf("vdl: typeJS unhandled type %v %v", t.Kind(), t))
}

func genMethodTags(data data, method *compile.Method) string {
	tags := method.Tags
	result := "["
	for _, tag := range tags {
		result += "{type: \"" + typeJS(data, tag.Type()) + "\", value:" + valueJS(tag) + "},"
	}
	result += "]"
	return result
}

func valueJS(v *vdl.Value) string {
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
	case vdl.OneOf:
		return "{}"
	}

	panic(fmt.Errorf("vdl: valueJS unhandled type %v %v", v.Kind(), v.Type()))
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
func generateTypeStub(t *compile.TypeDef) string {
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

	result := "function " + name + "("
	if wrappedType {
		result += "val"
	}
	result += ") {"

	if wrappedType {
		result += "\n    this.val = val;\n"
	} else if kind == vdl.Struct {
		// TODO(bjornick): Fill these out with default values after we figure
		// out how to import definitions from other files.
		result += "\n"
	}
	result += "  }\n"
	result += name + ".prototype._type = " + typeStruct(t.Type) + ";\n"

	result += "  types." + name + " = " + name + ";\n"

	return result
}

func init() {
	funcMap := template.FuncMap{
		"toCamelCase":      vdlutil.ToCamelCase,
		"numOutArgs":       numOutArgs,
		"genMethodTags":    genMethodTags,
		"generateTypeStub": generateTypeStub,
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
(function (name, context, definition) {
  if (typeof module !== 'undefined' && module.exports) {
    module.exports = definition(require('veyron'));
  } else {
    context.vdls = context.vdls || {};
    context.vdls[name] = definition(context.veyron);
  }
})('{{$pkg.Path}}', this, function(veyron) {
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
  var Types = veyron.Types;
  var Kind = veyron.Kind;

  var types = {};
  {{range $file := $pkg.Files}}{{range $type := $file.TypeDefs}}
  {{generateTypeStub $type}}
  {{end}}{{end}}
  return services;
});{{end}}`
