package build

import (
	"bytes"
	"fmt"
	"go/format"
	"path"
	"sort"
	"strconv"
	"strings"
	"text/template"

	"veyron2/idl"
	"veyron2/wiretype"
)

type GenGoOpts struct {
	// Fmt specifies whether to run gofmt on the generated source.
	Fmt bool
}

func DefaultGenGoOpts() GenGoOpts {
	return GenGoOpts{Fmt: true}
}

type genGoData struct {
	File          *File
	UserImports   userImports
	SystemImports []string
}

// GenFileGo takes a populated idl.File and returns a string containing the
// auto-generated interfaces, wrappers and stubs as formatted Go source code.
func GenFileGo(idlFile *File, opts GenGoOpts) []byte {
	data := genGoData{
		File:          idlFile,
		UserImports:   userImportsGo(idlFile),
		SystemImports: systemImportsGo(idlFile),
	}
	// The implementation uses the template mechanism from text/template and
	// executes the template against the genGoData instance.
	var buf bytes.Buffer
	err := genGoTemplate.Execute(&buf, data)
	if err != nil {
		// We shouldn't see an error; it means our template is buggy.
		panic(fmt.Errorf("idl: couldn't execute template: %v", err))
	}
	if opts.Fmt {
		// Use gofmt to format the generated source.
		pretty, err := format.Source(buf.Bytes())
		if err != nil {
			// We shouldn't see an error; it means we generated invalid code.
			fmt.Printf("%s", buf.Bytes())
			panic(fmt.Errorf("idl: generated invalid Go code: %v", err))
		}
		return pretty
	} else {
		return buf.Bytes()
	}
}

type userImport struct {
	Local string // Local name of the import; empty if no local name.
	Path  string // Path of the import; e.g. "veyron2/idl"
	Pkg   string // Set to non-empty Local, otherwise the basename of Path.
}

// userImports is a slice of userImport, sorted by path name.
type userImports []*userImport

// lookupImportPkg returns the pkg to use for the given pkgpath, based on the
// local user imports.  It takes advantage of the fact that userImports is
// always sorted by path.
func (u userImports) lookupImportPkg(pkgpath string) string {
	ix := sort.Search(len(u), func(i int) bool { return u[i].Path >= pkgpath })
	if ix <= len(u) && u[ix].Path == pkgpath {
		return u[ix].Pkg
	}
	panic(fmt.Errorf("idl: import pkg %q not found in %v", pkgpath, u))
}

// userImportsGo returns the actual user imports that we need for file f.  This
// isn't just the user-supplied f.Imports since the package dependencies may
// have changed after compilation.
func userImportsGo(f *File) (ret userImports) {
	// First sort all the package paths so that we get a deterministic ordering;
	// it would be annoying for local package names to change on each run.
	var sortedPaths sort.StringSlice
	for dep, _ := range f.PackageDeps {
		sortedPaths = append(sortedPaths, dep.Path)
	}
	sortedPaths.Sort()
	// Now walk through the sorted paths and assign user imports.  Each import
	// must end up with a unique pkg - when we see a collision we simply add a
	// "_N" suffix where N starts at 2 and increments.
	seen := make(map[string]bool)
	for _, p := range sortedPaths {
		local := ""
		pkg := path.Base(p)
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
		ret = append(ret, &userImport{local, p, pkg})
	}
	return
}

// usesGlobalAnyData returns true iff the global "anydata" type is used within
// the file f.
func usesGlobalAnyData(f *File) bool {
	for _, tdef := range f.TypeDefs {
		for dep, _ := range tdef.Base.namedDeps() {
			if dep == globalAnyData {
				return true
			}
		}
	}
	for _, cdef := range f.ConstDefs {
		if cdef.Const.TypeDef != nil {
			for dep, _ := range cdef.Const.TypeDef.namedDeps() {
				if dep == globalAnyData {
					return true
				}
			}
		}
	}
	for _, iface := range f.Interfaces {
		for _, method := range iface.Methods() {
			for _, iarg := range method.InArgs {
				for dep, _ := range iarg.Type.namedDeps() {
					if dep == globalAnyData {
						return true
					}
				}
			}
			for _, oarg := range method.OutArgs {
				for dep, _ := range oarg.Type.namedDeps() {
					if dep == globalAnyData {
						return true
					}
				}
			}
		}
	}
	return false
}

// numStreamingMethods returns the number of streaming methods in the provided
// interface.
func numStreamingMethods(iface *Interface) (n int) {
	for _, m := range iface.Methods() {
		if isStreamingMethodGo(m) {
			n++
		}
	}
	return
}

// systemImportsGo returns a list of required veyron system imports.
//
// TODO(toddw): Now that we have the userImports mechanism for de-duping local
// package names, we could consider using that instead of our "_gen_" prefix.
// That'll make the template code a bit messier though.
func systemImportsGo(f *File) []string {
	set := make(map[string]bool)
	if usesGlobalAnyData(f) {
		// Import for idl.AnyData
		set[`_gen_idl "veyron2/idl"`] = true
	}
	if len(f.Interfaces) > 0 {
		// Imports for the generated method: Bind{interface name}.
		set[`_gen_rt "veyron2/rt/r"`] = true
		set[`_gen_wiretype "veyron2/wiretype"`] = true
		set[`_gen_ipc "veyron2/ipc"`] = true
		set[`_gen_idl "veyron2/idl"`] = true
		set[`_gen_naming "veyron2/naming"`] = true
	}
	// If the user has specified any error IDs, typically we need to import the
	// "veyron2/verror" package.  However we allow idl code-generation in the
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

var genGoTemplate *template.Template

// The template mechanism is great at high-level formatting and simple
// substitution, but is bad at more complicated logic.  We define some functions
// that we can use in the template so that when things get complicated we back
// off to a regular function.
func init() {
	funcMap := template.FuncMap{
		"genpkg":                   genpkg,
		"notLastField":             notLastField,
		"errorIDGo":                errorIDGo,
		"typeGo":                   typeGo,
		"typeDefGo":                typeDefGo,
		"constDefGo":               constDefGo,
		"tagsGo":                   tagsGo,
		"fieldVarGo":               fieldVarGo,
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
		"methodOrNil":              methodOrNil,
		"interfaceOrNil":           interfaceOrNil,
		"typeDefsCode":             typeDefsCode,
	}
	genGoTemplate = template.Must(template.New("genGo").Funcs(funcMap).Parse(genGo))
}

func genpkg(file *File, pkg string) string {
	// Special-case code generation for the veyron2/verror package, to avoid
	// adding the "_gen_verror." package qualifier.
	if file.Package.Path == "veyron2/verror" && pkg == "verror" {
		return ""
	}
	return "_gen_" + pkg + "."
}

func notLastField(fields []*Field, ix int) bool {
	return ix < len(fields)-1
}

// Returns true iff the method has a streaming reply return value or a streaming arg input.
func isStreamingMethodGo(method *Method) bool {
	return hasStreamingInputGo(method) || hasStreamingOutputGo(method)
}

func hasStreamingInputGo(method *Method) bool {
	return method.InStream != nil
}

func hasStreamingOutputGo(method *Method) bool {
	return method.OutStream != nil
}

func errorIDGo(file *File, eid *ErrorID) string {
	if eid.ID != "" {
		return eid.ID
	}
	// The implicit error ID is generated based on the package path and type name.
	return file.Package.Path + "." + eid.Name
}

// namedTypeGo prints the typename for a named type, represented by def.  The
// data argument is the context that we're printing the typename in; this is
// necessary since each file may assign local names for imported packages.
func namedTypeGo(data genGoData, def *TypeDef) string {
	switch {
	case def == globalAnyData:
		// Special-case unnamed anydata.
		return "_gen_idl.AnyData"
	case def.File == globalFile:
		// Global primitives just use their name.
		return def.Name
	case def.File.Package == data.File.Package:
		// The type is from the same package - just use its name.
		return def.Name
	default:
		// The type is defined in a different package - print the import package to
		// use for this file, along with the name.
		return data.UserImports.lookupImportPkg(def.File.Package.Path) + "." + def.Name
	}
}

// typeGo prints a type represented by its typedef.
func typeGo(data genGoData, def *TypeDef) string {
	// Terminate the recursion at named typedefs.
	if def.Name != "" {
		return namedTypeGo(data, def)
	}
	// Otherwise recurse through the base type.
	switch t := def.Base.(type) {
	case *ArrayType:
		return fmt.Sprintf("[%d]%s", t.Len, typeGo(data, t.Elem.Def()))
	case *SliceType:
		return fmt.Sprintf("[]%s", typeGo(data, t.Elem.Def()))
	case *MapType:
		return fmt.Sprintf("map[%s]%s", typeGo(data, t.Key.Def()), typeGo(data, t.Elem.Def()))
	case *StructType:
		result := "struct{"
		for _, field := range t.Fields {
			result += field.Name + " " + typeGo(data, field.Type.Def())
		}
		return result + "}"
	case *NamedType:
		return typeGo(data, t.Def())
	default:
		// Catches TypeDefType, which should never appear here.
		panic(fmt.Errorf("idl: unhandled type %#v", t))
	}
}

// typeDefGo prints the type definition for a type.
func typeDefGo(data genGoData, def *TypeDef) string {
	return fmt.Sprintf("%stype %s %s", def.Doc, def.Name, typeWithDocGo(data, def.Base))
}

// typeWithDocGo prints the type represented by typ, including any documentation
// associated with struct fields.  All recursion is performed on the Type rather
// than the TypeDef, since documentation is dropped from the typedefs.
func typeWithDocGo(data genGoData, typ Type) string {
	switch t := typ.(type) {
	case *ArrayType:
		return fmt.Sprintf("[%d]%s", t.Len, typeWithDocGo(data, t.Elem))
	case *SliceType:
		return fmt.Sprintf("[]%s", typeWithDocGo(data, t.Elem))
	case *MapType:
		return fmt.Sprintf("map[%s]%s", typeWithDocGo(data, t.Key), typeWithDocGo(data, t.Elem))
	case *StructType:
		result := "struct{"
		for _, field := range t.Fields {
			result += "\n" + field.Doc
			result += field.Name + " " + typeWithDocGo(data, field.Type)
			result += field.DocSuffix
		}
		result += "\n}"
		return result
	case *NamedType:
		// Terminate the recursion at named types.
		return namedTypeGo(data, t.Def())
	default:
		// Catches TypeDefType, which should never appear here.
		panic(fmt.Errorf("idl: unhandled type %#v", t))
	}
}

// prefixName takes a name (potentially qualified with package name, as in
// "pkg.name") and prepends the given prefix to the last component of the name,
// as in "pkg.prefixname".
func prefixName(name, prefix string) string {
	path := strings.Split(name, ".")
	path[len(path)-1] = prefix + path[len(path)-1]
	return strings.Join(path, ".")
}

// methodOrNil returns the Method if the InterfaceComponent is a Method,
// otherwise returns nil.
func methodOrNil(c InterfaceComponent) *Method {
	if m, ok := c.(*Method); ok {
		return m
	}
	return nil
}

// interfaceOrNil returns the EmbeddedInterface if the InterfaceComponent is an
// EmbeddedInterface, otherwise returns nil.
func interfaceOrNil(c InterfaceComponent) *EmbeddedInterface {
	if e, ok := c.(*EmbeddedInterface); ok {
		return e
	}
	return nil
}

func constDefGo(data genGoData, def *ConstDef) string {
	return fmt.Sprintf("%s%s = %s%s", def.Doc, def.Name, constGo(data, def.Const), def.DocSuffix)
}

func constGo(data genGoData, c Const) (str string) {
	if c.TypeDef != nil {
		str += typeGo(data, c.TypeDef) + "("
	}
	str += cvString(c.Val)
	if c.TypeDef != nil {
		str += ")"
	}
	return str
}

func tagsGo(data genGoData, tags []Const) string {
	str := "[]interface{}{"
	for ix, tag := range tags {
		if ix > 0 {
			str += ", "
		}
		str += constGo(data, tag)
	}
	return str + "}"
}

// Returns a field variable, useful for defining in/out args.
func fieldVarGo(data genGoData, field *Field) string {
	var result string
	if len(field.Name) > 0 {
		result += field.Name + " "
	}
	result += typeGo(data, field.Type.Def())
	return result
}

// Returns the in-args of an interface's client stub method.
func inArgsWithOptsGo(firstArg string, data genGoData, method *Method) string {
	result := inArgsGo(firstArg, data, method)
	if len(result) > 0 {
		result += ", "
	}
	return result + "opts ..._gen_ipc.ClientCallOpt"
}

// Returns the in-args of an interface's server method.
func inArgsServiceGo(firstArg string, data genGoData, iface *Interface, method *Method) string {
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
func inArgsGo(firstArg string, data genGoData, method *Method) string {
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
func outArgsGo(data genGoData, iface *Interface, method *Method) string {
	if isStreamingMethodGo(method) {
		interfaceType := streamArgInterfaceTypeGo("", iface, method)
		return "(reply " + interfaceType + ", err error)"
	}
	return nonStreamingOutArgs(data, method)
}

func finishOutArgsGo(data genGoData, method *Method) string {
	return nonStreamingOutArgs(data, method)
}

// Returns the non streaming parts of the return types.  This will the return
// types for the server interface and the Finish method on the client stream.
func nonStreamingOutArgs(data genGoData, method *Method) string {
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
		return "(reply " + typeGo(data, method.OutArgs[0].Type.Def()) + ", err error)"
	default:
		return "(err error)"
	}
}

// The pointers of the return values of an idl method.  This will be passed
// into ipc.ClientCall.Finish.
func finishInArgsGo(data genGoData, method *Method) string {
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
func streamArgInterfaceTypeGo(streamType string, iface *Interface, method *Method) string {
	if method.OutStream == nil && method.InStream == nil {
		return ""
	}
	return fmt.Sprintf("%s%s%sStream", iface.Name, streamType, method.Name)
}

// Returns the concrete type name (not interface) representing the stream arg
// of an interface methods. There is a different type for the server and client
// portion of the stream since the stream defined might not be bidirectional.
func streamArgTypeGo(streamType string, iface *Interface, method *Method) string {
	n := streamArgInterfaceTypeGo(streamType, iface, method)
	if len(n) == 0 {
		return ""
	}
	return "impl" + n
}

// Returns the client stub implementation for an interface method.
func clientStubImplGo(data genGoData, iface *Interface, method *Method) string {
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
func serverStubImplGo(data genGoData, iface *Interface, method *Method) string {
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

// generate the go code for type defs
func typeDefsCode(td []idl.AnyData) string {
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

// The template that we execute against a genGoData instance to generate our
// code.  Most of this is fairly straightforward substitution and ranges; more
// complicated logic is delegated to the helper functions above.
//
// We try to generate code that has somewhat reasonable formatting, and leave
// the fine-tuning to the go/format package.  Note that go/format won't fix
// some instances of spurious newlines, so we try to keep it reasonable.
const genGo = `
{{with $data := .}}{{with $file := $data.File}}
// This file was auto-generated by the veyron idl tool.
// Source: {{$file.BaseName}}

{{$file.PackageDoc}}package {{$file.PackageName}}
{{if or $data.UserImports $data.SystemImports}}
import ({{range $imp := $data.UserImports}}
{{if $imp.Local}}{{$imp.Local}} {{end}}"{{$imp.Path}}"
{{end}}{{if $data.SystemImports}}
	// The non-user imports are prefixed with "_gen_" to prevent collisions.
	{{range $imp := $data.SystemImports}}{{$imp}}
{{end}}{{end}})
{{end}}
{{if $file.TypeDefs}}{{range $tdef := $file.TypeDefs}}
{{typeDefGo $data $tdef}}{{end}}
{{end}}
{{if $file.ConstDefs}}const ({{range $cdef := $file.ConstDefs}}
	{{constDefGo $data $cdef}}
{{end}}){{end}}
{{range $eid := $file.ErrorIDs}}
{{$eid.Doc}}const {{$eid.Name}} = {{genpkg $file "verror"}}ID("{{errorIDGo $file $eid}}"){{$eid.DocSuffix}}
{{end}}
{{range $iface := $file.Interfaces}}
{{$iface.Doc}}// {{$iface.Name}} is the interface the client binds and uses.
// {{$iface.Name}}_InternalNoTagGetter is the interface without the TagGetter
// and UnresolveStep methods (both framework-added, rathern than user-defined),
// to enable embedding without method collisions.  Not to be used directly by
// clients.
type {{$iface.Name}}_InternalNoTagGetter interface {
{{range $component := $iface.Components}}
{{with $method := methodOrNil $component}}
	{{$method.Doc}}{{$method.Name}}({{inArgsWithOptsGo "" $data $method}}) {{outArgsGo $data $iface $method}}{{$method.DocSuffix}}{{end}}{{with $interface := interfaceOrNil $component}}
	{{$interface.Doc}}{{typeGo $data $interface.Type.Def}}_InternalNoTagGetter{{$interface.DocSuffix}}{{end}}
{{end}} }
type {{$iface.Name}} interface {
	_gen_idl.TagGetter
	// UnresolveStep returns the names for the remote service, rooted at the
	// service's immediate namespace ancestor.
	UnresolveStep(opts ..._gen_ipc.ClientCallOpt) ([]string, error)
	{{$iface.Name}}_InternalNoTagGetter
}

// {{$iface.Name}}Service is the interface the server implements.
type {{$iface.Name}}Service interface {
{{range $component := $iface.Components}}
{{with $method := methodOrNil $component}}
	{{$method.Doc}}{{$method.Name}}({{inArgsServiceGo "context _gen_ipc.Context" $data $iface $method}}) {{finishOutArgsGo $data $method}}{{$method.DocSuffix}}{{end}}{{with $interface := interfaceOrNil $component}}
	{{$interface.Doc}}{{typeGo $data $interface.Type.Def}}Service{{$interface.DocSuffix}}
{{end}}
{{end}} }
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
	Send(item {{typeGo $data $method.InStream.Type.Def}}) error

	// CloseSend indicates to the server that no more items will be sent; server
	// Recv calls will receive io.EOF after all sent items.  Subsequent calls to
	// Send on the client will fail.  This is an optional call - it's used by
	// streaming clients that need the server to receive the io.EOF terminator.
	CloseSend() error
	{{end}}

	{{if hasStreamingOutput $method}}
	// Recv returns the next item in the input stream, blocking until
	// an item is available.  Returns io.EOF to indicate graceful end of input.
	Recv() (item {{typeGo $data $method.OutStream.Type.Def}}, err error)
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
func (c *{{$clientStreamType}}) Send(item {{typeGo $data $method.InStream.Type.Def}}) error {
	return c.clientCall.Send(item)
}

func (c *{{$clientStreamType}}) CloseSend() error {
	return c.clientCall.CloseSend()
}
{{end}}

{{if hasStreamingOutput $method}}
func (c *{{$clientStreamType}}) Recv() (item {{typeGo $data $method.OutStream.Type.Def}}, err error) {
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
	Send(item {{typeGo $data $method.OutStream.Type.Def}}) error
	{{end}}

	{{if hasStreamingInput $method}}
	// Recv fills itemptr with the next item in the input stream, blocking until
	// an item is available.  Returns io.EOF to indicate graceful end of input.
	Recv() (item {{typeGo $data $method.InStream.Type.Def}}, err error)
	{{end}}
}

// Implementation of the {{$serverStreamIfaceType}} interface that is not exported.
type {{$serverStreamType}} struct {
	serverCall _gen_ipc.ServerCall
}
{{if hasStreamingOutput $method}}
func (s *{{$serverStreamType}}) Send(item {{typeGo $data $method.OutStream.Type.Def}}) error {
	return s.serverCall.Send(item)
}
{{end}}

{{if hasStreamingInput $method}}
func (s *{{$serverStreamType}}) Recv() (item {{typeGo $data $method.InStream.Type.Def}}, err error) {
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
		case _gen_ipc.Runtime:
			client = o.Client()
		case _gen_ipc.Client:
			client = o
		default:
			return nil, _gen_idl.ErrUnrecognizedOption
		}
	default:
		return nil, _gen_idl.ErrTooManyOptionsToBind
	}
	stub := &clientStub{{$iface.Name}}{client: client, name: name}
{{range $interface := $iface.Embeds}}	stub.{{$interface.Type.Def.Name}}_InternalNoTagGetter, _ = {{prefixName (typeGo $data $interface.Type.Def) "Bind"}}(name, client)
{{end}}
	return stub, nil
}

// NewServer{{$iface.Name}} creates a new server stub.
//
// It takes a regular server implementing the {{$iface.Name}}Service
// interface, and returns a new server stub.
func NewServer{{$iface.Name}}(server {{$iface.Name}}Service) interface{} {
	return &ServerStub{{$iface.Name}}{
{{range $interface := $iface.Embeds}}		{{prefixName $interface.Type.Def.Name "ServerStub"}}: *{{prefixName (typeGo $data $interface.Type.Def) "NewServer"}}(server).(*{{prefixName (typeGo $data $interface.Type.Def) "ServerStub"}}),
{{end}}		service: server,
	}
}

// clientStub{{$iface.Name}} implements {{$iface.Name}}.
type clientStub{{$iface.Name}} struct {
{{range $interface := $iface.Embeds}}	{{typeGo $data $interface.Type.Def}}_InternalNoTagGetter
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
{{range $interface := $iface.Embeds}}	{{prefixName (typeGo $data $interface.Type.Def) "ServerStub"}}
{{end}}
	service {{$iface.Name}}Service
}

func (s *ServerStub{{$iface.Name}}) GetMethodTags(method string) []interface{} {
	return Get{{$iface.Name}}MethodTags(method)
}

func (s *ServerStub{{$iface.Name}}) Signature(call _gen_ipc.ServerCall) (_gen_ipc.ServiceSignature, error) {
	result := _gen_ipc.ServiceSignature{Methods: make(map[string]_gen_ipc.MethodSignature)}
{{range $mname, $method := $iface.Signature.Methods}}{{printf "\tresult.Methods[%q] = _gen_ipc.MethodSignature{" $mname}}
		InArgs:[]_gen_ipc.MethodArgument{
{{range $arg := $method.InArgs}}{{printf "\t\t\t{Name:%q, Type:%d},\n" ($arg.Name) ($arg.Type)}}{{end}}{{printf "\t\t},"}}
		OutArgs:[]_gen_ipc.MethodArgument{
{{range $arg := $method.OutArgs}}{{printf "\t\t\t{Name:%q, Type:%d},\n" ($arg.Name) ($arg.Type)}}{{end}}{{printf "\t\t},"}}
{{if $method.InStream}}{{printf "\t\t"}}InStream: {{$method.InStream}},{{end}}
{{if $method.OutStream}}{{printf "\t\t"}}OutStream: {{$method.OutStream}},{{end}}
	}
{{end}}
result.TypeDefs = {{typeDefsCode $iface.Signature.TypeDefs}}
{{if $iface.Embeds}}	var ss _gen_ipc.ServiceSignature
var firstAdded int
{{range $interface := $iface.Embeds}}	ss, _ = s.{{prefixName $interface.Type.Def.Name "ServerStub"}}.Signature(call)
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
{{range $interface := $iface.Embeds}}	if resp := {{prefixName (typeGo $data $interface.Type.Def) "Get"}}MethodTags(method); resp != nil {
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
