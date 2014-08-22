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

	"veyron2/vdl"
	"veyron2/vdl/compile"
	"veyron2/vdl/vdlutil"
)

type jsData struct {
	Pkg *compile.Package
	Env *compile.Env
}

// Generate takes a populated compile.Package and produces a byte slice
// containing the generated Javascript code.
func Generate(pkg *compile.Package, env *compile.Env) []byte {
	data := jsData{
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

func jsNumOutArgs(method *compile.Method) int {
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

func typeJS(data jsData, t *vdl.Type) string {
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

func jsGenMethodTags(data jsData, method *compile.Method) string {
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
	}

	// TODO(bjornick): Handle Enum, List, Map, Struct, OneOf, Any, Complex*, Bytes
	panic(fmt.Errorf("vdl: valueJS unhandled type %v %v", v.Kind(), v.Type()))
}

func init() {
	funcMap := template.FuncMap{
		"toCamelCase":     vdlutil.ToCamelCase,
		"jsNumOutArgs":    jsNumOutArgs,
		"jsGenMethodTags": jsGenMethodTags,
	}
	javascriptTemplate = template.Must(template.New("genJS").Funcs(funcMap).Parse(genJS))
}

// The template that we execute against a compile.Package instance to generate our
// code.  Most of this is fairly straightforward substitution and ranges; more
// complicated logic is delegated to the helper functions above.
//
// We try to generate code that has somewhat reasonable formatting.
const genJS = `{{with $data := .}}// This file was auto-generatead by the veyron vdl tool.
{{$pkg := $data.Pkg}}
(function (name, context, definition) {
  if (typeof module !== 'undefined' && module.exports) {
    module.exports = definition();
  } else {
    context.vdls = context.vdls || {};
    context.vdls[name] = definition();
  }
})('{{$pkg.Path}}', this, function() {
  var services = {
    package: '{{$pkg.Path}}',
  {{range $file := $pkg.Files}}{{range $iface := $file.Interfaces}}  {{$iface.Name}}: {
  {{range $method := $iface.AllMethods}}    {{toCamelCase $method.Name}}: {
	    numInArgs: {{len $method.InArgs}},
	    numOutArgs: {{jsNumOutArgs $method}},
	    inputStreaming: {{if $method.InStream}}true{{else}}false{{end}},
	    outputStreaming: {{if $method.OutStream}}true{{else}}false{{end}},
	    tags: {{jsGenMethodTags $data $method}}
      },
  {{end}}
    },
  {{end}}{{end}}
  };
  return services;
});{{end}}`
