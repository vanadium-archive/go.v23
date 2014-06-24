// Generates the javascript source code for vdl files.  The generated output in javascript
// differs from most other languages, since we don't generate stubs. Instead generate an
// object that contains the parsed VDL structures that will be used by the Javascript code
// to valid servers.

package gen

import (
	"bytes"
	"fmt"
	"text/template"

	"veyron2/vdl/compile"
)

// GenJavascriptFile takes a populated compile.Package and produces a byte slice
// containing the generated Javascript code.
func GenJavascriptFiles(pkg *compile.Package) []byte {
	var buf bytes.Buffer
	if err := javascriptTemplate.Execute(&buf, pkg); err != nil {
		// We shouldn't see an error; it means our template is buggy.
		panic(fmt.Errorf("vdl: couldn't execute template: %v", err))
	}
	return buf.Bytes()
}

var javascriptTemplate *template.Template

func jsNumOutArgs(method *compile.Method) int {
	return len(method.OutArgs) - 1
}
func init() {
	funcMap := template.FuncMap{
		"toCamelCase":  toCamelCase,
		"jsNumOutArgs": jsNumOutArgs,
	}
	javascriptTemplate = template.Must(template.New("genJS").Funcs(funcMap).Parse(genJS))
}

// The template that we execute against a compile.Package instance to generate our
// code.  Most of this is fairly straightforward substitution and ranges; more
// complicated logic is delegated to the helper functions above.
//
// We try to generate code that has somewhat reasonable formatting.
const genJS = `{{with $pkg := .}}// This file was auto-generatead by the veyron vdl tool.
(function (name, context, definition) {
  if (typeof module !== 'undefined' && module.exports) {
    module.exports = definition();
  } else {
    context.vdls = context.vdls || {};
    context[name] = definition();
  }
})('{{$pkg.Path}}', this, function() {
  var services = {
    package: '{{$pkg.Path}}',
  {{range $file := $pkg.Files}}{{range $iface := $file.Interfaces}}  {{$iface.Name}}: {
  {{range $method := $iface.AllMethods}}    {{toCamelCase $method.Name}}: {
	    numInArgs: {{len $method.InArgs}},
	    numOutArgs: {{jsNumOutArgs $method}},
	    inputStreaming: {{if $method.InStream}}true{{else}}false{{end}},
	    outputStreaming: {{if $method.OutStream}}true{{else}}false{{end}}
      },
  {{end}}
    },
  {{end}}{{end}}
  };
  return services;
});{{end}}`
