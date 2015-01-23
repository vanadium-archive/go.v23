// Package javascript implements Javascript code generation from compiled VDL packages.
package javascript

// Generates the javascript source code for vdl files.  The generated output in javascript
// differs from most other languages, since we don't generate stubs. Instead generate an
// object that contains the parsed VDL structures that will be used by the Javascript code
// to valid servers.

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"text/template"

	"v.io/core/veyron2/vdl"
	"v.io/core/veyron2/vdl/codegen"
	"v.io/core/veyron2/vdl/compile"
	"v.io/core/veyron2/vdl/vdlroot/src/vdltool"
	"v.io/core/veyron2/vdl/vdlutil"
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
func Generate(pkg *compile.Package, env *compile.Env, genImport func(string) string, config vdltool.JavascriptConfig) []byte {
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

// Format the given int64 into a JS BigInt.
func formatUint64BigInt(v uint64) string {
	buffer := make([]byte, 8)
	binary.BigEndian.PutUint64(buffer, v)
	sign := "0"
	if v > 0 {
		sign = "1"
	}
	return fmt.Sprintf("new BigInt(%s, %s)", sign, formatByteBuffer(buffer))
}

// Format the given int64 into a JS BigInt.
func formatInt64BigInt(v int64) string {
	buffer := make([]byte, 8)
	var sign int64 = 0
	if v > 0 {
		sign = 1
	} else if v < 0 {
		sign = -1
	}
	binary.BigEndian.PutUint64(buffer, uint64(v*sign)) // Adjust value by sign.

	return fmt.Sprintf("new BigInt(%d, %s)", sign, formatByteBuffer(buffer))
}

// Given a buffer of bytes, create the JS Uint8Array that corresponds to it (to be used with BigInt).
func formatByteBuffer(buffer []byte) string {
	buf := bytes.TrimLeft(buffer, "\x00") // trim leading zeros
	str := "new Uint8Array(["
	for i, b := range buf {
		if i > 0 {
			str += ", "
		}
		str += fmt.Sprintf("%#x", b)
	}
	str += "])"
	return str
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
	case vdl.Uint16, vdl.Uint32:
		return strconv.FormatUint(v.Uint(), 10)
	case vdl.Int16, vdl.Int32:
		return strconv.FormatInt(v.Int(), 10)
	case vdl.Uint64:
		return formatUint64BigInt(v.Uint())
	case vdl.Int64:
		return formatInt64BigInt(v.Int())
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
		for ix := 0; ix < v.Len(); ix++ {
			val := untypedConst(names, v.Index(ix))
			result += "\n" + val + ","
		}
		result += "\n]"
		if v.Type().Elem().Kind() == vdl.Byte {
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
	case vdl.Union:
		ix, innerVal := v.UnionField()
		return fmt.Sprintf("{ %q: %v }", vdlutil.ToCamelCase(v.Type().Field(ix).Name), untypedConst(names, innerVal))
	case vdl.TypeObject:
		return names.LookupType(v.TypeObject())
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
	case vdl.Any, vdl.TypeObject:
		return untypedConst(names, v)
	default:
		return fmt.Sprintf("new %s(%s)",
			names.LookupConstructor(v.Type()),
			untypedConst(names, v))
	}
}

// Remove the error out arg.
// TODO(bprosnitz) Remove this function when error is removed from the compiler out arg list.
func outArgsWithoutError(outArgs []*compile.Arg) []*compile.Arg {
	return outArgs[:len(outArgs)-1]
}

// Returns the JS version of the method signature.
func generateMethodSignature(method *compile.Method, names typeNames) string {
	return fmt.Sprintf(`{
    name: '%s',
    doc: %s,
    inArgs: %s,
    outArgs: %s,
    inStream: %s,
    outStream: %s,
    tags: %s
  }`,
		method.Name,
		quoteStripDoc(method.Doc),
		generateMethodArguments(method.InArgs, names),
		generateMethodArguments(outArgsWithoutError(method.OutArgs), names),
		generateMethodStreaming(method.InStream, names),
		generateMethodStreaming(method.OutStream, names),
		genMethodTags(names, method))
}

// Returns a slice describing the method's arguments.
func generateMethodArguments(args []*compile.Arg, names typeNames) string {
	ret := "["
	for _, arg := range args {
		ret += fmt.Sprintf(
			`{
      name: '%s',
      doc: %s,
      type: %s
    },
    `, arg.Name, quoteStripDoc(arg.Doc), names.LookupType(arg.Type))
	}
	ret += "]"
	return ret
}

// Returns the VOM type of the stream.
func generateMethodStreaming(streaming *vdl.Type, names typeNames) string {
	if streaming == nil {
		return "null"
	}
	return fmt.Sprintf(
		`{
      name: '',
      doc: '',
      type: %s
    }`,
		names.LookupType(streaming))
}

// Returns a slice of embeddings with the proper qualified identifiers.
func generateEmbeds(embeds []*compile.Interface) string {
	result := "["
	for _, embed := range embeds {
		result += fmt.Sprintf(`{
      name: '%s',
      pkgPath: '%s',
      doc: %s
    },
    `, embed.Name, embed.File.Package.Path, quoteStripDoc(embed.Doc))
	}
	result += "]"
	return result
}

func importPath(data data, path string) string {
	// We need to prefix all of these paths with a ./ to tell node that the path is relative to
	// the current directory.  Sadly filepath.Join(".", foo) == foo, so we have to do it
	// explicitly.
	return "." + string(filepath.Separator) + data.GenerateImport(path)
}

func quoteStripDoc(doc string) string {
	// TODO(alexfandrianto): We need to handle '// ' and '\n' in the docstring.
	// It would also be nice to single-quote the whole string.
	trimmed := strings.Trim(doc, "\n")
	return strconv.Quote(trimmed)
}

func init() {
	funcMap := template.FuncMap{
		"toCamelCase":               vdlutil.ToCamelCase,
		"numOutArgs":                numOutArgs,
		"genMethodTags":             genMethodTags,
		"makeTypeDefinitionsString": makeTypeDefinitionsString,
		"typedConst":                typedConst,
		"generateEmbeds":            generateEmbeds,
		"generateMethodSignature":   generateMethodSignature,
		"importPath":                importPath,
		"quoteStripDoc":             quoteStripDoc,
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
var BigInt = vom.BigInt;
var Complex = vom.Complex;
var Builtins = vom.Builtins;
var Registry = vom.Registry;

{{/* Define additional imported modules. */}}
{{$pkg := $data.Pkg}}
{{if $data.UserImports}}{{range $imp := $data.UserImports}}
var {{$imp.Local}} = require('{{importPath $data $imp.Path}}');{{end}}{{end}}

module.exports = {};


{{/* Define any types introduced by the VDL file. */}}
// Types:
{{makeTypeDefinitionsString $data.TypeNames }}


{{/* Define all constants as typed constants. */}}
// Consts:
{{range $file := $pkg.Files}}{{range $const := $file.ConstDefs}}
  module.exports.{{$const.Name}} = {{typedConst $data.TypeNames $const.Value}};
{{end}}{{end}}


{{/* TODO(alexfandrianto): Find a common place to put NotImplementedMethod. */}}
function NotImplementedMethod(name) {
  throw new Error('Method ' + name + ' not implemented');
}

{{/* Define each of those service interfaces here, including method stubs and
     service signature. */}}
// Services:
{{range $file := $pkg.Files}}
  {{range $iface := $file.Interfaces}}
    {{/* Define the service interface. */}}
function {{$iface.Name}}(){}
module.exports.{{$iface.Name}} = {{$iface.Name}}

    {{range $method := $iface.AllMethods}}
      {{/* Add each method to the service prototype. */}}
{{$iface.Name}}.prototype.{{$method.Name}} = NotImplementedMethod;
    {{end}} {{/* end range $iface.AllMethods */}}

    {{/* The service signature encodes the same info as signature.Interface.
         TODO(alexfandrianto): We want to associate the signature type here, but
         it's complicated. https://github.com/veyron/release-issues/issues/432
         For now, we need to pass the type in manually into encode. */}}
{{$iface.Name}}.prototype._serviceDescription = {
  name: '{{$iface.Name}}',
  pkgPath: '{{$pkg.Path}}',
  doc: {{quoteStripDoc $iface.Doc}},
  embeds: {{generateEmbeds $iface.Embeds}},
  methods: [
    {{range $method := $iface.AllMethods}}
      {{/* Each method signature contains the information in ipc.MethodSig. */}}
    {{generateMethodSignature $method $data.TypeNames}},
    {{end}} {{/*end range $iface.AllMethods*/}}
  ]
};

  {{end}} {{/* end range $files.Interfaces */}}
{{end}} {{/* end range $pkg.Files */}}


{{end}}`
