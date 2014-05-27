// Package gen provides functions to generate code from compiled VDL packages.
package gen

import (
	"bytes"
	"fmt"
	"go/format"
	"path"
	"sort"
	"strconv"
	"strings"
	"text/template"

	"veyron2/val"
	"veyron2/vdl"
	"veyron2/vdl/compile"
	"veyron2/wiretype"
	"veyron2/wiretype/build"
)

// GoOpts specifies options for generating Go files.
type GoOpts struct {
	// Fmt specifies whether to run gofmt on the generated source.
	Fmt bool
}

type goData struct {
	File          *compile.File
	Env           *compile.Env
	UserImports   userImports
	SystemImports []string
}

// GoFile takes a populated compile.File and returns a byte slice containing the
// generated Go source code.
func GoFile(file *compile.File, env *compile.Env, opts GoOpts) []byte {
	data := goData{
		File:          file,
		Env:           env,
		UserImports:   userImportsGo(file),
		SystemImports: systemImportsGo(file),
	}
	// The implementation uses the template mechanism from text/template and
	// executes the template against the goData instance.
	var buf bytes.Buffer
	if err := goTemplate.Execute(&buf, data); err != nil {
		// We shouldn't see an error; it means our template is buggy.
		panic(fmt.Errorf("vdl: couldn't execute template: %v", err))
	}
	if opts.Fmt {
		// Use gofmt to format the generated source.
		pretty, err := format.Source(buf.Bytes())
		if err != nil {
			// We shouldn't see an error; it means we generated invalid code.
			fmt.Printf("%s", buf.Bytes())
			panic(fmt.Errorf("vdl: generated invalid Go code: %v", err))
		}
		return pretty
	}
	return buf.Bytes()
}

type userImport struct {
	Local string // Local name of the import; empty if no local name.
	Path  string // Path of the import; e.g. "veyron2/vdl"
	Pkg   string // Set to non-empty Local, otherwise the basename of Path.
}

// userImports is a slice of userImport, sorted by path name.
type userImports []*userImport

// lookupImport returns the local package name to use for the given pkgpath,
// based on the user imports.  It takes advantage of the fact that userImports
// is always sorted by path.
func (u userImports) lookupImport(pkgpath string) string {
	ix := sort.Search(len(u), func(i int) bool { return u[i].Path >= pkgpath })
	if ix <= len(u) && u[ix].Path == pkgpath {
		return u[ix].Pkg
	}
	panic(fmt.Errorf("vdl: import path %q not found in %v", pkgpath, u))
}

// userImportsGo returns the actual user imports that we need for file f.  This
// isn't just the user-supplied f.Imports since the package dependencies may
// have changed after compilation.
func userImportsGo(f *compile.File) (ret userImports) {
	// Walk through the package deps (which are sorted by path) and assign user
	// imports.  Each import must end up with a unique local name - when we see a
	// collision we simply add a "_N" suffix where N starts at 2 and increments.
	seen := make(map[string]bool)
	for _, dep := range f.PackageDeps {
		local := ""
		pkg := path.Base(dep.Path)
		for ix := 1; true; ix++ {
			test := pkg
			if ix > 1 {
				test += "_" + strconv.Itoa(ix)
				local = test
			}
			if !seen[test] {
				// We found a unique item - break out.
				seen[test] = true
				pkg = test
				break
			}
		}
		ret = append(ret, &userImport{local, dep.Path, pkg})
	}
	return
}

// systemImportsGo returns a list of required veyron system imports.
//
// TODO(toddw): Now that we have the userImports mechanism for de-duping local
// package names, we could consider using that instead of our "_gen_" prefix.
// That'll make the template code a bit messier though.
func systemImportsGo(f *compile.File) []string {
	set := make(map[string]bool)
	if f.TypeDeps[val.AnyType] {
		// Import for vdl.Any
		set[`_gen_vdl "veyron2/vdl"`] = true
	}
	if f.TypeDeps[val.TypeValType] {
		// Import for val.Type
		set[`_gen_val "veyron2/val"`] = true
	}
	if len(f.Interfaces) > 0 {
		// Imports for the generated method: Bind{interface name}.
		set[`_gen_rt "veyron2/rt"`] = true
		set[`_gen_wiretype "veyron2/wiretype"`] = true
		set[`_gen_ipc "veyron2/ipc"`] = true
		set[`_gen_veyron2 "veyron2"`] = true
		set[`_gen_vdl "veyron2/vdl"`] = true
		set[`_gen_idl "veyron2/idl"`] = true
		set[`_gen_naming "veyron2/naming"`] = true
	}
	// If the user has specified any error IDs, typically we need to import the
	// "veyron2/verror" package.  However we allow vdl code-generation in the
	// "veyron2/verror" package itself, to specify common error IDs.  Special-case
	// this scenario to avoid self-cyclic package dependencies.
	if len(f.ErrorIDs) > 0 && f.Package.Path != "veyron2/verror" {
		set[`_gen_verror "veyron2/verror"`] = true
	}
	// Convert the set of imports into a sorted list.
	var ret sort.StringSlice
	for key := range set {
		ret = append(ret, key)
	}
	ret.Sort()
	return ret
}

var goTemplate *template.Template

// The template mechanism is great at high-level formatting and simple
// substitution, but is bad at more complicated logic.  We define some functions
// that we can use in the template so that when things get complicated we back
// off to a regular function.
func init() {
	funcMap := template.FuncMap{
		"genpkg":                   genpkg,
		"typeGo":                   typeGo,
		"typeDefGo":                typeDefGo,
		"constDefGo":               constDefGo,
		"tagsGo":                   tagsGo,
		"embedGo":                  embedGo,
		"isStreamingMethodGo":      isStreamingMethodGo,
		"inArgsServiceGo":          inArgsServiceGo,
		"inArgsGo":                 inArgsGo,
		"inArgsWithOptsGo":         inArgsWithOptsGo,
		"outArgsGo":                outArgsGo,
		"finishOutArgsGo":          finishOutArgsGo,
		"finishInArgsGo":           finishInArgsGo,
		"streamArgInterfaceTypeGo": streamArgInterfaceTypeGo,
		"streamArgTypeGo":          streamArgTypeGo,
		"clientStubImplGo":         clientStubImplGo,
		"serverStubImplGo":         serverStubImplGo,
		"hasStreamingInput":        hasStreamingInputGo,
		"hasStreamingOutput":       hasStreamingOutputGo,
		"prefixName":               prefixName,
		"signatureMethods":         signatureMethods,
		"signatureTypeDefs":        signatureTypeDefs,
	}
	goTemplate = template.Must(template.New("genGo").Funcs(funcMap).Parse(genGo))
}

func genpkg(file *compile.File, pkg string) string {
	// Special-case code generation for the veyron2/verror package, to avoid
	// adding the "_gen_verror." package qualifier.
	if file.Package.Path == "veyron2/verror" && pkg == "verror" {
		return ""
	}
	return "_gen_" + pkg + "."
}

// Returns true iff the method has a streaming reply return value or a streaming arg input.
func isStreamingMethodGo(method *compile.Method) bool {
	return hasStreamingInputGo(method) || hasStreamingOutputGo(method)
}

func hasStreamingInputGo(method *compile.Method) bool {
	return method.InStream != nil
}

func hasStreamingOutputGo(method *compile.Method) bool {
	return method.OutStream != nil
}

func qualifiedName(data goData, name string, file *compile.File) string {
	if file.Package == data.File.Package {
		// The name is from the same package - just use it.
		return name
	}
	// The name is defined in a different package - print the import package to
	// use for this file, along with the name.
	return data.UserImports.lookupImport(file.Package.Path) + "." + name
}

// typeGo translates val.Type into a Go type.
func typeGo(data goData, t *val.Type) string {
	// Terminate recursion at defined types, which include both user-defined types
	// (enum, struct, oneof) and built-in types.
	if def := data.Env.FindTypeDef(t); def != nil {
		switch {
		case t == val.AnyType:
			return "_gen_vdl.Any"
		case t == val.TypeValType:
			return "*_gen_val.Type"
		case def.File == compile.GlobalFile:
			// Global primitives just use their name.
			return def.Name
		}
		return qualifiedName(data, def.Name, def.File)
	}
	// Otherwise recurse through the type.
	switch t.Kind() {
	case val.Array:
		return "[" + strconv.Itoa(t.Len()) + "]" + typeGo(data, t.Elem())
	case val.List:
		return "[]" + typeGo(data, t.Elem())
	case val.Set:
		// TODO(toddw): Should we use map[X]bool instead?
		return "map[" + typeGo(data, t.Key()) + "]struct{}"
	case val.Map:
		return "map[" + typeGo(data, t.Key()) + "]" + typeGo(data, t.Elem())
	default:
		panic(fmt.Errorf("vdl: typeGo unhandled type %v %v", t.Kind(), t))
	}
}

// typeDefGo prints the type definition for a type.
func typeDefGo(data goData, def *compile.TypeDef) string {
	s := fmt.Sprintf("%stype %s ", def.Doc, def.Name)
	switch base, t := def.BaseType, def.Type; {
	case base != nil:
		s += typeGo(data, base)
	case t.Kind() == val.Enum:
		// We turn the VDL:
		//   type X enum{A;B}
		// into Go:
		//   type X uint
		//   const (
		//     A X = iota
		//     B
		//   )
		s += "int" + def.DocSuffix
		s += "\nconst ("
		for x := 0; x < t.NumEnumLabel(); x++ {
			s += "\n" + def.LabelDoc[x] + t.EnumLabel(x)
			if x == 0 {
				s += " " + def.Name + " = iota"
			}
			s += def.LabelDocSuffix[x]
		}
		return s + "\n)"
	case t.Kind() == val.Struct:
		s += "struct {"
		for x := 0; x < t.NumField(); x++ {
			f := t.Field(x)
			s += "\n" + def.FieldDoc[x] + f.Name + " "
			s += typeGo(data, f.Type) + def.FieldDocSuffix[x]
		}
		s += "\n}"
	case t.Kind() == val.OneOf:
		// We turn the VDL:
		//   type X oneof{bool;string}
		// into Go:
		//   type X interface{}
		s += "interface{}"
	default:
		panic(fmt.Errorf("vdl: typeDefGo unhandled type %v %v", t.Kind(), t))
	}
	return s + def.DocSuffix
}

// prefixName takes a name (potentially qualified with package name, as in
// "pkg.name") and prepends the given prefix to the last component of the name,
// as in "pkg.prefixname".
func prefixName(name, prefix string) string {
	path := strings.Split(name, ".")
	path[len(path)-1] = prefix + path[len(path)-1]
	return strings.Join(path, ".")
}

func constDefGo(data goData, def *compile.ConstDef) string {
	return fmt.Sprintf("%s%s = %s%s", def.Doc, def.Name, constGo(data, def.Value), def.DocSuffix)
}

func constGo(data goData, v *val.Value) string {
	switch v.Type() {
	case val.BoolType, val.StringType:
		// Treat the standard bool and string types as untyped constants in Go.
		// We turn the VDL:
		//   type NamedBool   bool
		//   type NamedString string
		//   const (
		//     B1 = true
		//     B2 = bool(true)
		//     B3 = NamedBool(true)
		//     S1 = "abc"
		//     S2 = string("abc")
		//     S3 = NamedString("abc")
		//   )
		// into Go:
		//   const (
		//     B1 = true
		//     B2 = true
		//     B3 = NamedBool(true)
		//     S1 = "abc"
		//     S2 = "abc"
		//     S3 = NamedString("abc")
		//   )
		return valueGo(v)
	}
	return typeGo(data, v.Type()) + "(" + valueGo(v) + ")"
}

func valueGo(v *val.Value) string {
	switch v.Kind() {
	case val.Bool:
		if v.Bool() {
			return "true"
		} else {
			return "false"
		}
	case val.Byte:
		return strconv.FormatUint(uint64(v.Byte()), 10)
	case val.Uint16, val.Uint32, val.Uint64:
		return strconv.FormatUint(v.Uint(), 10)
	case val.Int16, val.Int32, val.Int64:
		return strconv.FormatInt(v.Int(), 10)
	case val.Float32, val.Float64:
		return strconv.FormatFloat(v.Float(), 'g', -1, bitlen(v.Kind()))
	case val.Complex64, val.Complex128:
		s := strconv.FormatFloat(real(v.Complex()), 'g', -1, bitlen(v.Kind())) + "+"
		s += strconv.FormatFloat(imag(v.Complex()), 'g', -1, bitlen(v.Kind())) + "i"
		return s
	case val.String:
		return strconv.Quote(v.RawString())
	}
	if v.Type().IsBytes() {
		return strconv.Quote(string(v.Bytes()))
	}
	// TODO(toddw): Handle Enum, List, Map, Struct, OneOf, Any
	panic(fmt.Errorf("vdl: valueGo unhandled type %v %v", v.Kind(), v.Type()))
}

func bitlen(kind val.Kind) int {
	switch kind {
	case val.Float32, val.Complex64:
		return 32
	case val.Float64, val.Complex128:
		return 64
	}
	panic(fmt.Errorf("vdl: bitLen unhandled kind %v", kind))
}

func tagsGo(data goData, tags []*val.Value) string {
	str := "[]interface{}{"
	for ix, tag := range tags {
		if ix > 0 {
			str += ", "
		}
		str += constGo(data, tag)
	}
	return str + "}"
}

func embedGo(data goData, embed *compile.Interface) string {
	return qualifiedName(data, embed.Name, embed.File)
}

// Returns a field variable, useful for defining in/out args.
func fieldVarGo(data goData, arg *compile.Arg) string {
	var result string
	if len(arg.Name) > 0 {
		result += arg.Name + " "
	}
	result += typeGo(data, arg.Type)
	return result
}

// Returns the in-args of an interface's client stub method.
func inArgsWithOptsGo(firstArg string, data goData, method *compile.Method) string {
	result := inArgsGo(firstArg, data, method)
	if len(result) > 0 {
		result += ", "
	}
	return result + "opts ..._gen_ipc.ClientCallOpt"
}

// Returns the in-args of an interface's server method.
func inArgsServiceGo(firstArg string, data goData, iface *compile.Interface, method *compile.Method) string {
	result := inArgsGo(firstArg, data, method)
	if isStreamingMethodGo(method) {
		if len(result) > 0 {
			result += ", "
		}
		result += "stream " + streamArgInterfaceTypeGo("Service", iface, method)
	}
	return result
}

// Returns the in-args of an interface method.
func inArgsGo(firstArg string, data goData, method *compile.Method) string {
	result := firstArg
	for _, arg := range method.InArgs {
		if len(result) > 0 {
			result += ", "
		}
		result += fieldVarGo(data, arg)
	}

	return result
}

// Returns the out args of an interface method, wrapped in parens.  We always
// name the last error arg "err error" to simplify stub generation.
func outArgsGo(data goData, iface *compile.Interface, method *compile.Method) string {
	if isStreamingMethodGo(method) {
		interfaceType := streamArgInterfaceTypeGo("", iface, method)
		return "(reply " + interfaceType + ", err error)"
	}
	return nonStreamingOutArgs(data, method)
}

func finishOutArgsGo(data goData, method *compile.Method) string {
	return nonStreamingOutArgs(data, method)
}

// Returns the non streaming parts of the return types.  This will the return
// types for the server interface and the Finish method on the client stream.
func nonStreamingOutArgs(data goData, method *compile.Method) string {
	switch len := len(method.OutArgs); {
	case len > 2:
		result := "("
		for ax, arg := range method.OutArgs {
			if ax > 0 {
				result += ", "
			}
			if ax == len-1 {
				result += "err error"
			} else {
				result += fieldVarGo(data, arg)
			}
		}
		result += ")"
		return result
	case len == 2:
		return "(reply " + typeGo(data, method.OutArgs[0].Type) + ", err error)"
	default:
		return "(err error)"
	}
}

// The pointers of the return values of an vdl method.  This will be passed
// into ipc.ClientCall.Finish.
func finishInArgsGo(data goData, method *compile.Method) string {
	switch len := len(method.OutArgs); {
	case len > 2:
		result := ""
		for ax, arg := range method.OutArgs {
			if ax > 0 {
				result += ", "
			}
			name := arg.Name
			if ax == len-1 {
				name = "err"
			}
			result += "&" + name
		}
		return result
	case len == 2:
		return "&reply, &err"
	default:
		return "&err"
	}
}

// Returns the type name representing the Go interface of the stream arg of an
// interface method.  There is a different type for the server and client portion of
// the stream since the stream defined might not be bidirectional.
func streamArgInterfaceTypeGo(streamType string, iface *compile.Interface, method *compile.Method) string {
	if method.OutStream == nil && method.InStream == nil {
		return ""
	}
	return fmt.Sprintf("%s%s%sStream", iface.Name, streamType, method.Name)
}

// Returns the concrete type name (not interface) representing the stream arg
// of an interface methods. There is a different type for the server and client
// portion of the stream since the stream defined might not be bidirectional.
func streamArgTypeGo(streamType string, iface *compile.Interface, method *compile.Method) string {
	n := streamArgInterfaceTypeGo(streamType, iface, method)
	if len(n) == 0 {
		return ""
	}
	return "impl" + n
}

// Returns the client stub implementation for an interface method.
func clientStubImplGo(data goData, iface *compile.Interface, method *compile.Method) string {
	var buf bytes.Buffer
	buf.WriteString("\tvar call _gen_ipc.ClientCall\n")
	var args string

	if len(method.InArgs) == 0 {
		args = "nil"
	} else {
		args = "[]interface{}{"
		for ax, arg := range method.InArgs {
			if ax > 0 {
				args += ", "
			}
			args += arg.Name
		}
		args += "}"
	}

	fmt.Fprintf(&buf, "\tif call, err = __gen_c.client.StartCall(__gen_c.name, %q, %s, opts...); err != nil {\n return \n }\n", method.Name, args)

	if !isStreamingMethodGo(method) {
		fmt.Fprintf(&buf,
			`if ierr := call.Finish(%s); ierr != nil {
	err = ierr
}`, finishInArgsGo(data, method))
	} else {
		fmt.Fprintf(&buf, "reply = &%s{ clientCall: call}", streamArgTypeGo("", iface, method))
	}

	buf.WriteString("\nreturn")
	// Don't write the trailing newline; the caller adds it.
	return buf.String()
}

// Returns the server stub implementation for an interface method.
func serverStubImplGo(data goData, iface *compile.Interface, method *compile.Method) string {
	var buf bytes.Buffer
	var args string
	for ax, arg := range method.InArgs {
		if ax > 0 {
			args += ", "
		}
		args += arg.Name
	}

	if isStreamingMethodGo(method) {
		fmt.Fprintf(&buf, "\tstream := &%s{ serverCall: call }\n", streamArgTypeGo("Service", iface, method))
		if len(args) > 0 {
			args += ", "
		}
		args += "stream "
	}
	buf.WriteString("\t")
	switch len := len(method.OutArgs); {
	case len > 2:
		for ax, arg := range method.OutArgs {
			if ax > 0 {
				buf.WriteString(", ")
			}

			if ax == len-1 {
				buf.WriteString("err")
			} else {
				buf.WriteString(arg.Name)
			}
		}
	case len == 2:
		buf.WriteString("reply, err")
	default:
		buf.WriteString("err")

	}
	fmt.Fprintf(&buf, " = __gen_s.service.%s(call, %s)", method.Name, args)
	buf.WriteString("\n\treturn")
	// Don't write the trailing newline; the caller adds it.
	return buf.String()
}

type methodArgument struct {
	Name string // Argument name
	Type wiretype.TypeID
}

type methodSignature struct {
	InArgs    []methodArgument // Positional Argument information.
	OutArgs   []methodArgument
	InStream  wiretype.TypeID // Type of streaming arguments (or TypeIDInvalid if none). The type IDs here use the definitions in ServiceSigature.TypeDefs.
	OutStream wiretype.TypeID
}

type serviceSignature struct {
	TypeDefs build.TypeDefs // A slice of wiretype structures form the type definition.
	Methods  map[string]methodSignature
}

// signature generates the service signature of the interface.
func signature(iface *compile.Interface) *serviceSignature {

	sig := &serviceSignature{Methods: map[string]methodSignature{}}
	wtc := wireTypeConverter{}
	for _, method := range iface.Methods {
		ms := methodSignature{}
		for _, inarg := range method.InArgs {
			ms.InArgs = append(ms.InArgs, methodArgument{
				Name: inarg.Name,
				Type: wtc.WireTypeID(inarg.Type),
			})
		}
		for _, outarg := range method.OutArgs {
			ms.OutArgs = append(ms.OutArgs, methodArgument{
				Name: outarg.Name,
				Type: wtc.WireTypeID(outarg.Type),
			})
		}
		if method.InStream != nil {
			ms.InStream = wtc.WireTypeID(method.InStream)
		}
		if method.OutStream != nil {
			ms.OutStream = wtc.WireTypeID(method.OutStream)
		}
		sig.Methods[method.Name] = ms
	}
	sig.TypeDefs = wtc.Defs
	return sig
}

func signatureMethods(iface *compile.Interface) map[string]methodSignature {
	return signature(iface).Methods
}

func signatureTypeDefs(iface *compile.Interface) string {
	return typeDefsCode(signature(iface).TypeDefs)
}

// generate the go code for type defs
func typeDefsCode(td []vdl.Any) string {
	var buf bytes.Buffer
	buf.WriteString("[]_gen_idl.AnyData{\n")
	for _, wt := range td {
		switch t := wt.(type) {
		case wiretype.StructType:
			buf.WriteString("_gen_wiretype.StructType{\n")
			if len(t.Fields) > 0 {
				buf.WriteString("[]_gen_wiretype.FieldType{\n")
				for _, f := range t.Fields {
					buf.WriteString(fmt.Sprintf("_gen_%#v,\n", f))
				}
				buf.WriteString("},\n")
			} else {
				buf.WriteString("nil,\n")
			}
			buf.WriteString(fmt.Sprintf("%q, %#v},\n", t.Name, t.Tags))
		default:
			buf.WriteString(fmt.Sprintf("_gen_%#v,", wt))
		}
	}
	buf.WriteString("}")
	return buf.String()
}

// The template that we execute against a goData instance to generate our
// code.  Most of this is fairly straightforward substitution and ranges; more
// complicated logic is delegated to the helper functions above.
//
// We try to generate code that has somewhat reasonable formatting, and leave
// the fine-tuning to the go/format package.  Note that go/format won't fix
// some instances of spurious newlines, so we try to keep it reasonable.
const genGo = `
{{with $data := .}}{{with $file := $data.File}}
// This file was auto-generated by the veyron vdl tool.
// Source: {{$file.BaseName}}

{{$file.PackageDef.Doc}}package {{$file.PackageDef.Name}}{{$file.PackageDef.DocSuffix}}
{{if or $data.UserImports $data.SystemImports}}
import ({{range $imp := $data.UserImports}}
{{if $imp.Local}}{{$imp.Local}} {{end}}"{{$imp.Path}}"
{{end}}{{if $data.SystemImports}}
	// The non-user imports are prefixed with "_gen_" to prevent collisions.
	{{range $imp := $data.SystemImports}}{{$imp}}
{{end}}{{end}})
{{end}}
{{if $file.TypeDefs}}{{range $tdef := $file.TypeDefs}}
{{typeDefGo $data $tdef}}
{{end}}{{end}}
{{if $file.ConstDefs}}const ({{range $cdef := $file.ConstDefs}}
	{{constDefGo $data $cdef}}
{{end}}){{end}}
{{range $eid := $file.ErrorIDs}}
{{$eid.Doc}}const {{$eid.Name}} = {{genpkg $file "verror"}}ID("{{$eid.ID}}"){{$eid.DocSuffix}}
{{end}}
{{range $iface := $file.Interfaces}}
{{$iface.Doc}}// {{$iface.Name}} is the interface the client binds and uses.
// {{$iface.Name}}_InternalNoTagGetter is the interface without the TagGetter
// and UnresolveStep methods (both framework-added, rathern than user-defined),
// to enable embedding without method collisions.  Not to be used directly by
// clients.
type {{$iface.Name}}_InternalNoTagGetter interface {
{{range $embed := $iface.Embeds}}
	{{$embed.Doc}}{{embedGo $data $embed}}_InternalNoTagGetter{{$embed.DocSuffix}}{{end}}{{range $method := $iface.Methods}}
	{{$method.Doc}}{{$method.Name}}({{inArgsWithOptsGo "" $data $method}}) {{outArgsGo $data $iface $method}}{{$method.DocSuffix}}{{end}}
}
type {{$iface.Name}} interface {
	_gen_vdl.TagGetter
	// UnresolveStep returns the names for the remote service, rooted at the
	// service's immediate namespace ancestor.
	UnresolveStep(opts ..._gen_ipc.ClientCallOpt) ([]string, error)
	{{$iface.Name}}_InternalNoTagGetter
}

// {{$iface.Name}}Service is the interface the server implements.
type {{$iface.Name}}Service interface {
{{range $embed := $iface.Embeds}}
	{{$embed.Doc}}{{embedGo $data $embed}}Service{{$embed.DocSuffix}}{{end}}{{range $method := $iface.Methods}}
	{{$method.Doc}}{{$method.Name}}({{inArgsServiceGo "context _gen_ipc.Context" $data $iface $method}}) {{finishOutArgsGo $data $method}}{{$method.DocSuffix}}{{end}}
}
{{range $method := $iface.Methods}}{{if isStreamingMethodGo $method}}
{{$clientStreamIfaceType := streamArgInterfaceTypeGo "" $iface $method}}
{{$clientStreamType := streamArgTypeGo "" $iface $method}}
{{$serverStreamIfaceType := streamArgInterfaceTypeGo "Service" $iface $method}}
{{$serverStreamType := streamArgTypeGo "Service" $iface $method}}

// {{$clientStreamIfaceType}} is the interface for streaming responses of the method
// {{$method.Name}} in the service interface {{$iface.Name}}.
type {{$clientStreamIfaceType}} interface {
	{{if hasStreamingInput $method}}
	// Send places the item onto the output stream, blocking if there is no buffer
	// space available.
	Send(item {{typeGo $data $method.InStream}}) error

	// CloseSend indicates to the server that no more items will be sent; server
	// Recv calls will receive io.EOF after all sent items.  Subsequent calls to
	// Send on the client will fail.  This is an optional call - it's used by
	// streaming clients that need the server to receive the io.EOF terminator.
	CloseSend() error
	{{end}}

	{{if hasStreamingOutput $method}}
	// Recv returns the next item in the input stream, blocking until
	// an item is available.  Returns io.EOF to indicate graceful end of input.
	Recv() (item {{typeGo $data $method.OutStream}}, err error)
	{{end}}

	// Finish closes the stream and returns the positional return values for
	// call.
	Finish() {{finishOutArgsGo $data $method}}

  // Cancel cancels the RPC, notifying the server to stop processing.
  Cancel()
}

// Implementation of the {{$clientStreamIfaceType}} interface that is not exported.
type {{$clientStreamType}} struct {
	clientCall _gen_ipc.ClientCall
}
{{if hasStreamingInput $method}}
func (c *{{$clientStreamType}}) Send(item {{typeGo $data $method.InStream}}) error {
	return c.clientCall.Send(item)
}

func (c *{{$clientStreamType}}) CloseSend() error {
	return c.clientCall.CloseSend()
}
{{end}}

{{if hasStreamingOutput $method}}
func (c *{{$clientStreamType}}) Recv() (item {{typeGo $data $method.OutStream}}, err error) {
	err = c.clientCall.Recv(&item)
	return
}
{{end}}

func (c *{{$clientStreamType}}) Finish() {{finishOutArgsGo $data $method}} {
	if ierr := c.clientCall.Finish({{finishInArgsGo $data $method}}); ierr != nil {
		err = ierr
	}
	return
}

func (c *{{$clientStreamType}}) Cancel() {
  c.clientCall.Cancel()
}

// {{$serverStreamIfaceType}} is the interface for streaming responses of the method
// {{$method.Name}} in the service interface {{$iface.Name}}.
type {{$serverStreamIfaceType}} interface { {{if hasStreamingOutput $method}}
	// Send places the item onto the output stream, blocking if there is no buffer
	// space available.
	Send(item {{typeGo $data $method.OutStream}}) error
	{{end}}

	{{if hasStreamingInput $method}}
	// Recv fills itemptr with the next item in the input stream, blocking until
	// an item is available.  Returns io.EOF to indicate graceful end of input.
	Recv() (item {{typeGo $data $method.InStream}}, err error)
	{{end}}
}

// Implementation of the {{$serverStreamIfaceType}} interface that is not exported.
type {{$serverStreamType}} struct {
	serverCall _gen_ipc.ServerCall
}
{{if hasStreamingOutput $method}}
func (s *{{$serverStreamType}}) Send(item {{typeGo $data $method.OutStream}}) error {
	return s.serverCall.Send(item)
}
{{end}}

{{if hasStreamingInput $method}}
func (s *{{$serverStreamType}}) Recv() (item {{typeGo $data $method.InStream}}, err error) {
	err = s.serverCall.Recv(&item)
	return
}
{{end}}

{{end}}
{{end}}

// Bind{{$iface.Name}} returns the client stub implementing the {{$iface.Name}}
// interface.
//
// If no _gen_ipc.Client is specified, the default _gen_ipc.Client in the
// global Runtime is used.
func Bind{{$iface.Name}}(name string, opts ..._gen_ipc.BindOpt) ({{$iface.Name}}, error) {
	var client _gen_ipc.Client
	switch len(opts) {
	case 0:
		client = _gen_rt.R().Client()
	case 1:
		switch o := opts[0].(type) {
		case _gen_veyron2.Runtime:
			client = o.Client()
		case _gen_ipc.Client:
			client = o
		default:
			return nil, _gen_vdl.ErrUnrecognizedOption
		}
	default:
		return nil, _gen_vdl.ErrTooManyOptionsToBind
	}
	stub := &clientStub{{$iface.Name}}{client: client, name: name}
{{range $embed := $iface.Embeds}}	stub.{{$embed.Name}}_InternalNoTagGetter, _ = {{prefixName (embedGo $data $embed) "Bind"}}(name, client)
{{end}}
	return stub, nil
}

// NewServer{{$iface.Name}} creates a new server stub.
//
// It takes a regular server implementing the {{$iface.Name}}Service
// interface, and returns a new server stub.
func NewServer{{$iface.Name}}(server {{$iface.Name}}Service) interface{} {
	return &ServerStub{{$iface.Name}}{
{{range $embed := $iface.Embeds}}	{{prefixName $embed.Name "ServerStub"}}: *{{prefixName (embedGo $data $embed) "NewServer"}}(server).(*{{prefixName (embedGo $data $embed) "ServerStub"}}),
{{end}}	service: server,
	}
}

// clientStub{{$iface.Name}} implements {{$iface.Name}}.
type clientStub{{$iface.Name}} struct {
{{range $embed := $iface.Embeds}}	{{embedGo $data $embed}}_InternalNoTagGetter
{{end}}
	client _gen_ipc.Client
	name string
}

func (c *clientStub{{$iface.Name}}) GetMethodTags(method string) []interface{} {
	return Get{{$iface.Name}}MethodTags(method)
}
{{range $method := $iface.Methods}}
func (__gen_c *clientStub{{$iface.Name}}) {{$method.Name}}({{inArgsWithOptsGo "" $data $method}}) {{outArgsGo $data $iface $method}} {
{{clientStubImplGo $data $iface $method}}
}
{{end}}

func (c *clientStub{{$iface.Name}}) UnresolveStep(opts ..._gen_ipc.ClientCallOpt) (reply []string, err error) {
	var call _gen_ipc.ClientCall
	if call, err = c.client.StartCall(c.name, "UnresolveStep", nil, opts...); err != nil {
		return
	}
	if ierr := call.Finish(&reply, &err); ierr != nil {
		err = ierr
	}
	return
}

// ServerStub{{$iface.Name}} wraps a server that implements
// {{$iface.Name}}Service and provides an object that satisfies
// the requirements of veyron2/ipc.ReflectInvoker.
type ServerStub{{$iface.Name}} struct {
{{range $embed := $iface.Embeds}}	{{prefixName (embedGo $data $embed) "ServerStub"}}
{{end}}
	service {{$iface.Name}}Service
}

func (s *ServerStub{{$iface.Name}}) GetMethodTags(method string) []interface{} {
	return Get{{$iface.Name}}MethodTags(method)
}

func (s *ServerStub{{$iface.Name}}) Signature(call _gen_ipc.ServerCall) (_gen_ipc.ServiceSignature, error) {
	result := _gen_ipc.ServiceSignature{Methods: make(map[string]_gen_ipc.MethodSignature)}
{{range $mname, $method := signatureMethods $iface}}{{printf "\tresult.Methods[%q] = _gen_ipc.MethodSignature{" $mname}}
		InArgs:[]_gen_ipc.MethodArgument{
{{range $arg := $method.InArgs}}{{printf "\t\t\t{Name:%q, Type:%d},\n" ($arg.Name) ($arg.Type)}}{{end}}{{printf "\t\t},"}}
		OutArgs:[]_gen_ipc.MethodArgument{
{{range $arg := $method.OutArgs}}{{printf "\t\t\t{Name:%q, Type:%d},\n" ($arg.Name) ($arg.Type)}}{{end}}{{printf "\t\t},"}}
{{if $method.InStream}}{{printf "\t\t"}}InStream: {{$method.InStream}},{{end}}
{{if $method.OutStream}}{{printf "\t\t"}}OutStream: {{$method.OutStream}},{{end}}
	}
{{end}}
result.TypeDefs = {{signatureTypeDefs $iface}}
{{if $iface.Embeds}}	var ss _gen_ipc.ServiceSignature
var firstAdded int
{{range $interface := $iface.Embeds}}	ss, _ = s.{{prefixName $interface.NamePos.Name "ServerStub"}}.Signature(call)
	firstAdded = len(result.TypeDefs)
	for k, v := range ss.Methods {
		for i, _ := range v.InArgs {
			if v.InArgs[i].Type >= _gen_wiretype.TypeIDFirst {
				v.InArgs[i].Type += _gen_wiretype.TypeID(firstAdded)
			}
		}
		for i, _ := range v.OutArgs {
			if v.OutArgs[i].Type >= _gen_wiretype.TypeIDFirst {
				v.OutArgs[i].Type += _gen_wiretype.TypeID(firstAdded)
			}
		}
		if v.InStream >= _gen_wiretype.TypeIDFirst {
			v.InStream += _gen_wiretype.TypeID(firstAdded)
		}
		if v.OutStream >= _gen_wiretype.TypeIDFirst {
			v.OutStream += _gen_wiretype.TypeID(firstAdded)
		}
		result.Methods[k] = v
	}
	//TODO(bprosnitz) combine type definitions from embeded interfaces in a way that doesn't cause duplication.
	for _, d := range ss.TypeDefs {
		switch wt := d.(type) {
		case _gen_wiretype.SliceType:
			if wt.Elem >= _gen_wiretype.TypeIDFirst {
				wt.Elem += _gen_wiretype.TypeID(firstAdded)
			}
			d = wt
		case _gen_wiretype.ArrayType:
			if wt.Elem >= _gen_wiretype.TypeIDFirst {
				wt.Elem += _gen_wiretype.TypeID(firstAdded)
			}
			d = wt
		case _gen_wiretype.MapType:
			if wt.Key >= _gen_wiretype.TypeIDFirst {
				wt.Key += _gen_wiretype.TypeID(firstAdded)
			}
			if wt.Elem >= _gen_wiretype.TypeIDFirst {
				wt.Elem += _gen_wiretype.TypeID(firstAdded)
			}
			d = wt
		case _gen_wiretype.StructType:
			for _, fld := range wt.Fields {
				if fld.Type >= _gen_wiretype.TypeIDFirst {
					fld.Type += _gen_wiretype.TypeID(firstAdded)
				}
			}
			d = wt
		}
		result.TypeDefs = append(result.TypeDefs, d)
	}
{{end}}{{end}}

	return result, nil
}

func (s *ServerStub{{$iface.Name}}) UnresolveStep(call _gen_ipc.ServerCall) (reply []string, err error) {
	if unresolver, ok := s.service.(_gen_ipc.Unresolver); ok {
		return unresolver.UnresolveStep(call)
	}
	if call.Server() == nil {
		return
	}
	var published []string
	if published, err = call.Server().Published(); err != nil || published == nil {
		return
	}
	reply = make([]string, len(published))
	for i, p := range(published) {
		reply[i] = _gen_naming.Join(p, call.Name())
	}
	return
}

{{range $method := $iface.Methods}}
func (__gen_s *ServerStub{{$iface.Name}}) {{$method.Name}}({{inArgsGo "call _gen_ipc.ServerCall" $data $method}}) {{finishOutArgsGo $data $method}} {
{{serverStubImplGo $data $iface $method}}
}
{{end}}

func Get{{$iface.Name}}MethodTags(method string) []interface{} {
{{range $embed := $iface.Embeds}}	if resp := {{prefixName (embedGo $data $embed) "Get"}}MethodTags(method); resp != nil {
		return resp
	}
{{end}}{{if $iface.Methods}}	switch method { {{range $method := $iface.Methods}}
	case "{{$method.Name}}":
		return {{tagsGo $data $method.Tags}}{{end}}
	default:
		return nil
	}{{else}}	return nil{{end}}
}
{{end}}{{end}}{{end}}
`
