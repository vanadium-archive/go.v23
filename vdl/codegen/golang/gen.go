// Package golang implements Go code generation from compiled VDL packages.
package golang

import (
	"bytes"
	"fmt"
	"go/format"
	"sort"
	"strings"
	"text/template"

	"veyron.io/veyron/veyron2/vdl"
	"veyron.io/veyron/veyron2/vdl/codegen"
	"veyron.io/veyron/veyron2/vdl/compile"
	vdlroot "veyron.io/veyron/veyron2/vdl/vdlroot/src/vdl"
	"veyron.io/veyron/veyron2/vdl/vdlutil"
	"veyron.io/veyron/veyron2/wiretype"
	"veyron.io/veyron/veyron2/wiretype/build"
)

type goData struct {
	File          *compile.File
	Env           *compile.Env
	UserImports   codegen.Imports
	SystemImports []string
}

// Generate takes a populated compile.File and returns a byte slice containing
// the generated Go source code.
func Generate(file *compile.File, env *compile.Env, config vdlroot.GoConfig) []byte {
	data := goData{
		File:          file,
		Env:           env,
		UserImports:   codegen.ImportsForFiles(file),
		SystemImports: systemImportsGo(file),
	}
	// The implementation uses the template mechanism from text/template and
	// executes the template against the goData instance.
	var buf bytes.Buffer
	if err := goTemplate.Execute(&buf, data); err != nil {
		// We shouldn't see an error; it means our template is buggy.
		panic(fmt.Errorf("vdl: couldn't execute template: %v", err))
	}
	if !config.NoFmt {
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

// systemImportsGo returns a list of required veyron system imports.
//
// TODO(toddw): Now that we have the userImports mechanism for de-duping local
// package names, we could consider using that instead of our "__" prefix.
// That'll make the template code a bit messier though.
func systemImportsGo(f *compile.File) []string {
	set := make(map[string]bool)
	if f.TypeDeps[vdl.AnyType] {
		// Import for vdlutil.Any
		set[`__vdlutil "veyron.io/veyron/veyron2/vdl/vdlutil"`] = true
	}
	if f.TypeDeps[vdl.TypeObjectType] {
		// Import for vdl.Type
		set[`__vdl "veyron.io/veyron/veyron2/vdl"`] = true
	}
	if len(f.Interfaces) > 0 {
		// Imports for the generated method: {interface name}Client.
		set[`__veyron2 "veyron.io/veyron/veyron2"`] = true
		set[`__wiretype "veyron.io/veyron/veyron2/wiretype"`] = true
		set[`__ipc "veyron.io/veyron/veyron2/ipc"`] = true
		set[`__context "veyron.io/veyron/veyron2/context"`] = true
		set[`__vdlutil "veyron.io/veyron/veyron2/vdl/vdlutil"`] = true
		if fileHasStreamingMethods(f) {
			set[`__io "io"`] = true
		}
	}
	// If the user has specified any error IDs, typically we need to import the
	// "veyron.io/veyron/veyron2/verror" package.  However we allow vdl code-generation in the
	// "veyron.io/veyron/veyron2/verror" package itself, to specify common error IDs.  Special-case
	// this scenario to avoid self-cyclic package dependencies.
	if len(f.ErrorIDs) > 0 && f.Package.Path != "veyron.io/veyron/veyron2/verror" {
		set[`__verror "veyron.io/veyron/veyron2/verror"`] = true
	}
	// Convert the set of imports into a sorted list.
	var ret sort.StringSlice
	for key := range set {
		ret = append(ret, key)
	}
	ret.Sort()
	return ret
}

// fileHasStreamingMethods returns true iff f contains an interface with a
// streaming method, disregarding embedded interfaces.
func fileHasStreamingMethods(f *compile.File) bool {
	for _, i := range f.Interfaces {
		for _, m := range i.Methods {
			if isStreamingMethod(m) {
				return true
			}
		}
	}
	return false
}

var goTemplate *template.Template

// The template mechanism is great at high-level formatting and simple
// substitution, but is bad at more complicated logic.  We define some functions
// that we can use in the template so that when things get complicated we back
// off to a regular function.
func init() {
	funcMap := template.FuncMap{
		"genpkg":                genpkg,
		"typeGo":                typeGo,
		"typeDefGo":             typeDefGo,
		"constDefGo":            constDefGo,
		"tagsGo":                tagsGo,
		"embedGo":               embedGo,
		"isStreamingMethod":     isStreamingMethod,
		"hasStreamingMethods":   hasStreamingMethods,
		"isGlobbableGlob":       isGlobbableGlob,
		"docBreak":              docBreak,
		"argTypes":              argTypes,
		"argNameTypes":          argNameTypes,
		"argParens":             argParens,
		"uniqueName":            uniqueName,
		"uniqueNameImpl":        uniqueNameImpl,
		"hasFinalError":         hasFinalError,
		"stripFinalError":       stripFinalError,
		"serverContextType":     serverContextType,
		"serverContextStubType": serverContextStubType,
		"outArgsClient":         outArgsClient,
		"clientStubImpl":        clientStubImpl,
		"clientFinishImpl":      clientFinishImpl,
		"serverStubImpl":        serverStubImpl,
		"signatureMethods":      signatureMethods,
		"signatureTypeDefs":     signatureTypeDefs,
		"reInitStreamValue":     reInitStreamValue,
	}
	goTemplate = template.Must(template.New("genGo").Funcs(funcMap).Parse(genGo))
}

func genpkg(file *compile.File, pkg string) string {
	// Special-case code generation for the veyron2/verror package, to avoid
	// adding the "__verror." package qualifier.
	if file.Package.Path == "veyron.io/veyron/veyron2/verror" && pkg == "verror" {
		return ""
	}
	return "__" + pkg + "."
}

func isStreamingMethod(method *compile.Method) bool {
	return method.InStream != nil || method.OutStream != nil
}

func hasStreamingMethods(methods []*compile.Method) bool {
	for _, method := range methods {
		if isStreamingMethod(method) {
			return true
		}
	}
	return false
}

// isGlobbableGlob returns true iff iface and method represent the standard
// Globbable.Glob method.  This is special-cased to use the standard
// ipc.GlobContext, rather than generating one, so that our framework code can
// use it as well.
func isGlobbableGlob(iface *compile.Interface, method *compile.Method) bool {
	const globPath = "veyron.io/veyron/veyron2/services/mounttable"
	return iface.File.Package.Path == globPath && iface.Name == "Globbable" && method.Name == "Glob"
}

// docBreak adds a "//\n" break to separate previous comment lines and doc.  If
// doc is empty it returns the empty string.
func docBreak(doc string) string {
	if doc == "" {
		return ""
	}
	return "//\n" + doc
}

// argTypes returns a comma-separated list of each type from args.
func argTypes(data goData, args []*compile.Arg) string {
	var result []string
	for _, arg := range args {
		result = append(result, typeGo(data, arg.Type))
	}
	return strings.Join(result, ", ")
}

// argNames returns a comma-separated list of each name from args.  The names
// are of the form prefixD, where D is the position of the argument.
func argNames(prefix, first, last string, args []*compile.Arg) string {
	var result []string
	if first != "" {
		result = append(result, first)
	}
	for ix := 0; ix < len(args); ix++ {
		result = append(result, fmt.Sprintf("%s%d", prefix, ix))
	}
	if last != "" {
		result = append(result, last)
	}
	return strings.Join(result, ", ")
}

// argNameTypes returns a comma-separated list of "name type" from args.  If
// argPrefix is empty, the name specified in args is used; otherwise the name is
// prefixD, where D is the position of the argument.  If argPrefix is empty and
// no names are specified in args, no names will be output.
func argNameTypes(argPrefix, first, last string, data goData, args []*compile.Arg) string {
	noNames := argPrefix == "" && !hasArgNames(args)
	var result []string
	if first != "" {
		result = append(result, maybeStripArgName(first, noNames))
	}
	for ax, arg := range args {
		var name string
		switch {
		case noNames:
			break
		case argPrefix == "":
			name = arg.Name + " "
		default:
			name = fmt.Sprintf("%s%d ", argPrefix, ax)
		}
		result = append(result, name+typeGo(data, arg.Type))
	}
	if last != "" {
		result = append(result, maybeStripArgName(last, noNames))
	}
	return strings.Join(result, ", ")
}

func hasArgNames(args []*compile.Arg) bool {
	// VDL guarantees that either all args are named, or none of them are.
	return len(args) > 0 && args[0].Name != ""
}

// maybeStripArgName strips away the first space-terminated token from arg, only
// if strip is true.
func maybeStripArgName(arg string, strip bool) string {
	if index := strings.Index(arg, " "); index != -1 && strip {
		return arg[index+1:]
	}
	return arg
}

// argParens takes a list of 0 or more arguments, and adds parens only when
// necessary; if args contains any commas or spaces, we must add parens.
func argParens(argList string) string {
	if strings.IndexAny(argList, ", ") > -1 {
		return "(" + argList + ")"
	}
	return argList
}

// uniqueName returns a unique name based on the interface, method and suffix.
func uniqueName(iface *compile.Interface, method *compile.Method, suffix string) string {
	return iface.Name + method.Name + suffix
}

// uniqueNameImpl returns uniqueName with an "impl" prefix.
func uniqueNameImpl(iface *compile.Interface, method *compile.Method, suffix string) string {
	return "impl" + uniqueName(iface, method, suffix)
}

// hasFinalError returns true iff the last arg in args is an error.
func hasFinalError(args []*compile.Arg) bool {
	return len(args) > 0 && args[len(args)-1].Type == compile.ErrorType
}

// stripFinalError returns args without a final error arg.
func stripFinalError(args []*compile.Arg) []*compile.Arg {
	if hasFinalError(args) {
		return args[:len(args)-1]
	}
	return args
}

// The first arg of every server method is a context; the type is either a typed
// context for streams, or ipc.ServerContext for non-streams.
func serverContextType(prefix string, iface *compile.Interface, method *compile.Method) string {
	switch {
	case isGlobbableGlob(iface, method):
		return prefix + "__ipc.GlobContext"
	case isStreamingMethod(method):
		return prefix + uniqueName(iface, method, "Context")
	}
	return prefix + "__ipc.ServerContext"
}

// The first arg of every server stub method is a context; the type is either a
// typed context stub for streams, or ipc.ServerContext for non-streams.
func serverContextStubType(prefix string, iface *compile.Interface, method *compile.Method) string {
	switch {
	case isGlobbableGlob(iface, method):
		return prefix + "*__ipc.GlobContextStub"
	case isStreamingMethod(method):
		return prefix + "*" + uniqueName(iface, method, "ContextStub")
	}
	return prefix + "__ipc.ServerContext"
}

// outArgsClient returns the out args of an interface method on the client,
// wrapped in parens if necessary.  The client side always returns a final
// error, regardless of whether the method actually includes a final error.
func outArgsClient(argPrefix string, data goData, iface *compile.Interface, method *compile.Method) string {
	first, args := "", stripFinalError(method.OutArgs)
	if isStreamingMethod(method) {
		first, args = "ocall "+uniqueName(iface, method, "Call"), nil
	}
	return argParens(argNameTypes(argPrefix, first, "err error", data, args))
}

// clientStubImpl returns the interface method client stub implementation.
func clientStubImpl(data goData, iface *compile.Interface, method *compile.Method) string {
	var buf bytes.Buffer
	inargs := "nil"
	if len(method.InArgs) > 0 {
		inargs = "[]interface{}{" + argNames("i", "", "", method.InArgs) + "}"
	}
	fmt.Fprint(&buf, "\tvar call __ipc.Call\n")
	fmt.Fprintf(&buf, "\tif call, err = c.c(ctx).StartCall(ctx, c.name, %q, %s, opts...); err != nil {\n\t\treturn\n\t}\n", method.Name, inargs)
	switch {
	case isStreamingMethod(method):
		fmt.Fprintf(&buf, "ocall = &%s{Call: call}\n", uniqueNameImpl(iface, method, "Call"))
	default:
		fmt.Fprintf(&buf, "%s\n", clientFinishImpl("call", method))
	}
	fmt.Fprint(&buf, "\treturn")
	return buf.String() // the caller writes the trailing newline
}

// clientFinishImpl returns the client finish implementation for method.
func clientFinishImpl(varname string, method *compile.Method) string {
	// The client side always returns a final error, regardless of whether the
	// method actually includes a final error.  But the actual Finish call should
	// only include "&err" if method.OutArgs includes it.
	lastArg := ""
	if hasFinalError(method.OutArgs) {
		lastArg = "&err"
	}
	outargs := argNames("&o", "", lastArg, stripFinalError(method.OutArgs))
	return fmt.Sprintf("\tif ierr := %s.Finish(%s); ierr != nil {\n\t\terr = ierr\n\t}", varname, outargs)
}

// serverStubImpl returns the interface method server stub implementation.
func serverStubImpl(data goData, iface *compile.Interface, method *compile.Method) string {
	var buf bytes.Buffer
	inargs := argNames("i", "ctx", "", method.InArgs)
	fmt.Fprint(&buf, "\t")
	if len(method.OutArgs) > 0 {
		fmt.Fprint(&buf, "return ")
	}
	fmt.Fprintf(&buf, "s.impl.%s(%s)", method.Name, inargs)
	return buf.String() // the caller writes the trailing newline
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
func typeDefsCode(td []vdlutil.Any) string {
	var buf bytes.Buffer
	buf.WriteString("[]__vdlutil.Any{\n")
	for _, wt := range td {
		switch t := wt.(type) {
		case wiretype.StructType:
			buf.WriteString("__wiretype.StructType{\n")
			if len(t.Fields) > 0 {
				buf.WriteString("[]__wiretype.FieldType{\n")
				for _, f := range t.Fields {
					buf.WriteString(fmt.Sprintf("__%#v,\n", f))
				}
				buf.WriteString("},\n")
			} else {
				buf.WriteString("nil,\n")
			}
			buf.WriteString(fmt.Sprintf("%q, %#v},\n", t.Name, t.Tags))
		default:
			buf.WriteString(fmt.Sprintf("__%#v,", wt))
		}
	}
	buf.WriteString("}")
	return buf.String()
}

func reInitStreamValue(data goData, t *vdl.Type, name string) string {
	switch t.Kind() {
	case vdl.Struct:
		return name + " = " + typeGo(data, t) + "{}\n"
	case vdl.Any:
		return name + " = nil\n"
	}
	return ""
}

// The template that we execute against a goData instance to generate our
// code.  Most of this is fairly straightforward substitution and ranges; more
// complicated logic is delegated to the helper functions above.
//
// We try to generate code that has somewhat reasonable formatting, and leave
// the fine-tuning to the go/format package.  Note that go/format won't fix
// some instances of spurious newlines, so we try to keep it reasonable.
const genGo = `
{{with $data := .}}{{$file := $data.File}}
// This file was auto-generated by the veyron vdl tool.
// Source: {{$file.BaseName}}

{{$file.PackageDef.Doc}}package {{$file.PackageDef.Name}}{{$file.PackageDef.DocSuffix}}

{{if or $data.UserImports $data.SystemImports}}
import ( {{range $imp := $data.UserImports}}
{{if $imp.Name}}{{$imp.Name}} {{end}}"{{$imp.Path}}"
{{end}}{{if $data.SystemImports}}
	// The non-user imports are prefixed with "__" to prevent collisions.
	{{range $imp := $data.SystemImports}}{{$imp}}
{{end}}{{end}})
{{end}}

{{if $file.Interfaces}}
// TODO(toddw): Remove this line once the new signature support is done.
// It corrects a bug where __wiretype is unused in VDL pacakges where only
// bootstrap types are used on interfaces.
const _ = __wiretype.TypeIDInvalid
{{end}}

{{if $file.TypeDefs}}{{range $tdef := $file.TypeDefs}}
{{typeDefGo $data $tdef}}
{{end}}{{end}}

{{range $cdef := $file.ConstDefs}}
{{constDefGo $data $cdef}}
{{end}}

{{range $eid := $file.ErrorIDs}}
{{$eid.Doc}}const {{$eid.Name}} = {{genpkg $file "verror"}}ID("{{$eid.ID}}"){{$eid.DocSuffix}}
{{end}}

{{range $iface := $file.Interfaces}}{{$ifaceStreaming := hasStreamingMethods $iface.AllMethods}}
// {{$iface.Name}}ClientMethods is the client interface
// containing {{$iface.Name}} methods.
{{docBreak $iface.Doc}}type {{$iface.Name}}ClientMethods interface { {{range $embed := $iface.Embeds}}
	{{$embed.Doc}}{{embedGo $data $embed}}ClientMethods{{$embed.DocSuffix}}{{end}}{{range $method := $iface.Methods}}
	{{$method.Doc}}{{$method.Name}}({{argNameTypes "" "ctx __context.T" "opts ...__ipc.CallOpt" $data $method.InArgs}}) {{outArgsClient "" $data $iface $method}}{{$method.DocSuffix}}{{end}}
}

// {{$iface.Name}}ClientStub adds universal methods to {{$iface.Name}}ClientMethods.
type {{$iface.Name}}ClientStub interface {
	{{$iface.Name}}ClientMethods
	__ipc.UniversalServiceMethods
}

// {{$iface.Name}}Client returns a client stub for {{$iface.Name}}.
func {{$iface.Name}}Client(name string, opts ...__ipc.BindOpt) {{$iface.Name}}ClientStub {
	var client __ipc.Client
	for _, opt := range opts {
		if clientOpt, ok := opt.(__ipc.Client); ok {
			client = clientOpt
		}
	}
	return impl{{$iface.Name}}ClientStub{ name, client{{range $embed := $iface.Embeds}}, {{embedGo $data $embed}}Client(name, client){{end}} }
}

type impl{{$iface.Name}}ClientStub struct {
	name   string
	client __ipc.Client
{{range $embed := $iface.Embeds}}
	{{embedGo $data $embed}}ClientStub{{end}}
}

func (c impl{{$iface.Name}}ClientStub) c(ctx __context.T) __ipc.Client {
	if c.client != nil {
		return c.client
	}
	return __veyron2.RuntimeFromContext(ctx).Client()
}

{{range $method := $iface.Methods}}
func (c impl{{$iface.Name}}ClientStub) {{$method.Name}}({{argNameTypes "i" "ctx __context.T" "opts ...__ipc.CallOpt" $data $method.InArgs}}) {{outArgsClient "o" $data $iface $method}} {
{{clientStubImpl $data $iface $method}}
}
{{end}}

func (c impl{{$iface.Name}}ClientStub) Signature(ctx __context.T, opts ...__ipc.CallOpt) (o0 __ipc.ServiceSignature, err error) {
	var call __ipc.Call
	if call, err = c.c(ctx).StartCall(ctx, c.name, "Signature", nil, opts...); err != nil {
		return
	}
	if ierr := call.Finish(&o0, &err); ierr != nil {
		err = ierr
	}
	return
}

func (c impl{{$iface.Name}}ClientStub) GetMethodTags(ctx __context.T, method string, opts ...__ipc.CallOpt) (o0 []interface{}, err error) {
	var call __ipc.Call
	if call, err = c.c(ctx).StartCall(ctx, c.name, "GetMethodTags", []interface{}{method}, opts...); err != nil {
		return
	}
	if ierr := call.Finish(&o0, &err); ierr != nil {
		err = ierr
	}
	return
}

{{range $method := $iface.Methods}}{{if isStreamingMethod $method}}
{{$clientStream := uniqueName $iface $method "ClientStream"}}
{{$clientCall := uniqueName $iface $method "Call"}}
{{$clientCallImpl := uniqueNameImpl $iface $method "Call"}}
{{$clientRecvImpl := uniqueNameImpl $iface $method "CallRecv"}}
{{$clientSendImpl := uniqueNameImpl $iface $method "CallSend"}}

// {{$clientStream}} is the client stream for {{$iface.Name}}.{{$method.Name}}.
type {{$clientStream}} interface { {{if $method.OutStream}}
	// RecvStream returns the receiver side of the {{$iface.Name}}.{{$method.Name}} client stream.
	RecvStream() interface {
		// Advance stages an item so that it may be retrieved via Value.  Returns
		// true iff there is an item to retrieve.  Advance must be called before
		// Value is called.  May block if an item is not available.
		Advance() bool
		// Value returns the item that was staged by Advance.  May panic if Advance
		// returned false or was not called.  Never blocks.
		Value() {{typeGo $data $method.OutStream}}
		// Err returns any error encountered by Advance.  Never blocks.
		Err() error
	} {{end}}{{if $method.InStream}}
	// SendStream returns the send side of the {{$iface.Name}}.{{$method.Name}} client stream.
	SendStream() interface {
		// Send places the item onto the output stream.  Returns errors encountered
		// while sending, or if Send is called after Close or Cancel.  Blocks if
		// there is no buffer space; will unblock when buffer space is available or
		// after Cancel.
		Send(item {{typeGo $data $method.InStream}}) error
		// Close indicates to the server that no more items will be sent; server
		// Recv calls will receive io.EOF after all sent items.  This is an optional
		// call - e.g. a client might call Close if it needs to continue receiving
		// items from the server after it's done sending.  Returns errors
		// encountered while closing, or if Close is called after Cancel.  Like
		// Send, blocks if there is no buffer space available.
		Close() error
	} {{end}}
}

// {{$clientCall}} represents the call returned from {{$iface.Name}}.{{$method.Name}}.
type {{$clientCall}} interface {
	{{$clientStream}} {{if $method.InStream}}
	// Finish performs the equivalent of SendStream().Close, then blocks until
	// the server is done, and returns the positional return values for the call.{{else}}
	// Finish blocks until the server is done, and returns the positional return
	// values for call.{{end}}
	//
	// Finish returns immediately if Cancel has been called; depending on the
	// timing the output could either be an error signaling cancelation, or the
	// valid positional return values from the server.
	//
	// Calling Finish is mandatory for releasing stream resources, unless Cancel
	// has been called or any of the other methods return an error.  Finish should
	// be called at most once.
	Finish() {{argParens (argNameTypes "" "" "err error" $data (stripFinalError $method.OutArgs))}}
	// Cancel cancels the RPC, notifying the server to stop processing.  It is
	// safe to call Cancel concurrently with any of the other stream methods.
	// Calling Cancel after Finish has returned is a no-op.
	Cancel()
}

type {{$clientCallImpl}} struct {
	__ipc.Call{{if $method.OutStream}}
	valRecv {{typeGo $data $method.OutStream}}
	errRecv error{{end}}
}

{{if $method.OutStream}}func (c *{{$clientCallImpl}}) RecvStream() interface {
	Advance() bool
	Value() {{typeGo $data $method.OutStream}}
	Err() error
} {
	return {{$clientRecvImpl}}{c}
}

type {{$clientRecvImpl}} struct {
	c *{{$clientCallImpl}}
}

func (c {{$clientRecvImpl}}) Advance() bool {
	{{reInitStreamValue $data $method.OutStream "c.c.valRecv"}}c.c.errRecv = c.c.Recv(&c.c.valRecv)
	return c.c.errRecv == nil
}
func (c {{$clientRecvImpl}}) Value() {{typeGo $data $method.OutStream}} {
	return c.c.valRecv
}
func (c {{$clientRecvImpl}}) Err() error {
	if c.c.errRecv == __io.EOF {
		return nil
	}
	return c.c.errRecv
}
{{end}}{{if $method.InStream}}func (c *{{$clientCallImpl}}) SendStream() interface {
	Send(item {{typeGo $data $method.InStream}}) error
	Close() error
} {
	return {{$clientSendImpl}}{c}
}

type {{$clientSendImpl}} struct {
	c *{{$clientCallImpl}}
}

func (c {{$clientSendImpl}}) Send(item {{typeGo $data $method.InStream}}) error {
	return c.c.Send(item)
}
func (c {{$clientSendImpl}}) Close() error {
	return c.c.CloseSend()
}
{{end}}func (c *{{$clientCallImpl}}) Finish() {{argParens (argNameTypes "o" "" "err error" $data (stripFinalError $method.OutArgs))}} { {{if not (hasFinalError $method.OutArgs)}}
	// No final "&err" in the Finish call; vdl didn't have a final error.{{end}}
{{clientFinishImpl "c.Call" $method}}
	return
}
{{end}}{{end}}

// {{$iface.Name}}ServerMethods is the interface a server writer
// implements for {{$iface.Name}}.
{{docBreak $iface.Doc}}type {{$iface.Name}}ServerMethods interface { {{range $embed := $iface.Embeds}}
	{{$embed.Doc}}{{embedGo $data $embed}}ServerMethods{{$embed.DocSuffix}}{{end}}{{range $method := $iface.Methods}}
	{{$method.Doc}}{{$method.Name}}({{argNameTypes "" (serverContextType "ctx " $iface $method) "" $data $method.InArgs}}) {{argParens (argNameTypes "" "" "" $data $method.OutArgs)}}{{$method.DocSuffix}}{{end}}
}

// {{$iface.Name}}ServerStubMethods is the server interface containing
// {{$iface.Name}} methods, as expected by ipc.Server.{{if $ifaceStreaming}}
// The only difference between this interface and {{$iface.Name}}ServerMethods
// is the streaming methods.{{else}}
// There is no difference between this interface and {{$iface.Name}}ServerMethods
// since there are no streaming methods.{{end}}
type {{$iface.Name}}ServerStubMethods {{if $ifaceStreaming}}interface { {{range $embed := $iface.Embeds}}
	{{$embed.Doc}}{{embedGo $data $embed}}ServerStubMethods{{$embed.DocSuffix}}{{end}}{{range $method := $iface.Methods}}
	{{$method.Doc}}{{$method.Name}}({{argNameTypes "" (serverContextStubType "ctx " $iface $method) "" $data $method.InArgs}}) {{argParens (argNameTypes "" "" "" $data $method.OutArgs)}}{{$method.DocSuffix}}{{end}}
}
{{else}}{{$iface.Name}}ServerMethods
{{end}}

// {{$iface.Name}}ServerStub adds universal methods to {{$iface.Name}}ServerStubMethods.
type {{$iface.Name}}ServerStub interface {
	{{$iface.Name}}ServerStubMethods
	// GetMethodTags will be replaced with DescribeInterfaces.
	GetMethodTags(ctx __ipc.ServerContext, method string) ([]interface{}, error)
	// Signature will be replaced with DescribeInterfaces.
	Signature(ctx __ipc.ServerContext) (__ipc.ServiceSignature, error)
}

// {{$iface.Name}}Server returns a server stub for {{$iface.Name}}.
// It converts an implementation of {{$iface.Name}}ServerMethods into
// an object that may be used by ipc.Server.
func {{$iface.Name}}Server(impl {{$iface.Name}}ServerMethods) {{$iface.Name}}ServerStub {
	stub := impl{{$iface.Name}}ServerStub{
		impl: impl,{{range $embed := $iface.Embeds}}
		{{$embed.Name}}ServerStub: {{embedGo $data $embed}}Server(impl),{{end}}
	}
	// Initialize GlobState; always check the stub itself first, to handle the
	// case where the user has the Glob method defined in their VDL source.
	if gs := __ipc.NewGlobState(stub); gs != nil {
		stub.gs = gs
	} else if gs := __ipc.NewGlobState(impl); gs != nil {
		stub.gs = gs
	}
	return stub
}

type impl{{$iface.Name}}ServerStub struct {
	impl {{$iface.Name}}ServerMethods{{range $embed := $iface.Embeds}}
	{{embedGo $data $embed}}ServerStub{{end}}
	gs *__ipc.GlobState
}

{{range $method := $iface.Methods}}
func (s impl{{$iface.Name}}ServerStub) {{$method.Name}}({{argNameTypes "i" (serverContextStubType "ctx " $iface $method) "" $data $method.InArgs}}) {{argParens (argTypes $data $method.OutArgs)}} {
{{serverStubImpl $data $iface $method}}
}
{{end}}

func (s impl{{$iface.Name}}ServerStub) VGlob() *__ipc.GlobState {
	return s.gs
}

func (s impl{{$iface.Name}}ServerStub) GetMethodTags(ctx __ipc.ServerContext, method string) ([]interface{}, error) {
	// TODO(toddw): Replace with new DescribeInterfaces implementation.
	{{range $embed := $iface.Embeds}}	if resp, err := s.{{$embed.Name}}ServerStub.GetMethodTags(ctx, method); resp != nil || err != nil {
			return resp, err
		}
	{{end}}{{if $iface.Methods}}	switch method { {{range $method := $iface.Methods}}
		case "{{$method.Name}}":
			return {{tagsGo $data $method.Tags}}, nil{{end}}
		default:
			return nil, nil
		}{{else}}	return nil, nil{{end}}
}

func (s impl{{$iface.Name}}ServerStub) Signature(ctx __ipc.ServerContext) (__ipc.ServiceSignature, error) {
	// TODO(toddw) Replace with new DescribeInterfaces implementation.
	result := __ipc.ServiceSignature{Methods: make(map[string]__ipc.MethodSignature)}
{{range $mname, $method := signatureMethods $iface}}{{printf "\tresult.Methods[%q] = __ipc.MethodSignature{" $mname}}
		InArgs:[]__ipc.MethodArgument{
{{range $arg := $method.InArgs}}{{printf "\t\t\t{Name:%q, Type:%d},\n" ($arg.Name) ($arg.Type)}}{{end}}{{printf "\t\t},"}}
		OutArgs:[]__ipc.MethodArgument{
{{range $arg := $method.OutArgs}}{{printf "\t\t\t{Name:%q, Type:%d},\n" ($arg.Name) ($arg.Type)}}{{end}}{{printf "\t\t},"}}
{{if $method.InStream}}{{printf "\t\t"}}InStream: {{$method.InStream}},{{end}}
{{if $method.OutStream}}{{printf "\t\t"}}OutStream: {{$method.OutStream}},{{end}}
	}
{{end}}
result.TypeDefs = {{signatureTypeDefs $iface}}
{{if $iface.Embeds}}	var ss __ipc.ServiceSignature
var firstAdded int
{{range $embeds := $iface.Embeds}}	ss, _ = s.{{$embeds.Name}}ServerStub.Signature(ctx)
	firstAdded = len(result.TypeDefs)
	for k, v := range ss.Methods {
		for i, _ := range v.InArgs {
			if v.InArgs[i].Type >= __wiretype.TypeIDFirst {
				v.InArgs[i].Type += __wiretype.TypeID(firstAdded)
			}
		}
		for i, _ := range v.OutArgs {
			if v.OutArgs[i].Type >= __wiretype.TypeIDFirst {
				v.OutArgs[i].Type += __wiretype.TypeID(firstAdded)
			}
		}
		if v.InStream >= __wiretype.TypeIDFirst {
			v.InStream += __wiretype.TypeID(firstAdded)
		}
		if v.OutStream >= __wiretype.TypeIDFirst {
			v.OutStream += __wiretype.TypeID(firstAdded)
		}
		result.Methods[k] = v
	}
	//TODO(bprosnitz) combine type definitions from embeded interfaces in a way that doesn't cause duplication.
	for _, d := range ss.TypeDefs {
		switch wt := d.(type) {
		case __wiretype.SliceType:
			if wt.Elem >= __wiretype.TypeIDFirst {
				wt.Elem += __wiretype.TypeID(firstAdded)
			}
			d = wt
		case __wiretype.ArrayType:
			if wt.Elem >= __wiretype.TypeIDFirst {
				wt.Elem += __wiretype.TypeID(firstAdded)
			}
			d = wt
		case __wiretype.MapType:
			if wt.Key >= __wiretype.TypeIDFirst {
				wt.Key += __wiretype.TypeID(firstAdded)
			}
			if wt.Elem >= __wiretype.TypeIDFirst {
				wt.Elem += __wiretype.TypeID(firstAdded)
			}
			d = wt
		case __wiretype.StructType:
			for i, fld := range wt.Fields {
				if fld.Type >= __wiretype.TypeIDFirst {
					wt.Fields[i].Type += __wiretype.TypeID(firstAdded)
				}
			}
			d = wt
		// NOTE: other types are missing, but we are upgrading anyways.
		}
		result.TypeDefs = append(result.TypeDefs, d)
	}
{{end}}{{end}}

	return result, nil
}

{{range $method := $iface.Methods}}
{{if isStreamingMethod $method}}
{{if not (isGlobbableGlob $iface $method)}}
{{$serverStream := uniqueName $iface $method "ServerStream"}}
{{$serverContext := uniqueName $iface $method "Context"}}
{{$serverContextStub := uniqueName $iface $method "ContextStub"}}
{{$serverRecvImpl := uniqueNameImpl $iface $method "ContextRecv"}}
{{$serverSendImpl := uniqueNameImpl $iface $method "ContextSend"}}

// {{$serverStream}} is the server stream for {{$iface.Name}}.{{$method.Name}}.
type {{$serverStream}} interface { {{if $method.InStream}}
	// RecvStream returns the receiver side of the {{$iface.Name}}.{{$method.Name}} server stream.
	RecvStream() interface {
		// Advance stages an item so that it may be retrieved via Value.  Returns
		// true iff there is an item to retrieve.  Advance must be called before
		// Value is called.  May block if an item is not available.
		Advance() bool
		// Value returns the item that was staged by Advance.  May panic if Advance
		// returned false or was not called.  Never blocks.
		Value() {{typeGo $data $method.InStream}}
		// Err returns any error encountered by Advance.  Never blocks.
		Err() error
	} {{end}}{{if $method.OutStream}}
	// SendStream returns the send side of the {{$iface.Name}}.{{$method.Name}} server stream.
	SendStream() interface {
		// Send places the item onto the output stream.  Returns errors encountered
		// while sending.  Blocks if there is no buffer space; will unblock when
		// buffer space is available.
		Send(item {{typeGo $data $method.OutStream}}) error
	} {{end}}
}

// {{$serverContext}} represents the context passed to {{$iface.Name}}.{{$method.Name}}.
type {{$serverContext}} interface {
	__ipc.ServerContext
	{{$serverStream}}
}

// {{$serverContextStub}} is a wrapper that converts ipc.ServerCall into
// a typesafe stub that implements {{$serverContext}}.
type {{$serverContextStub}} struct {
	__ipc.ServerCall{{if $method.InStream}}
	valRecv {{typeGo $data $method.InStream}}
	errRecv error{{end}}
}

// Init initializes {{$serverContextStub}} from ipc.ServerCall.
func (s *{{$serverContextStub}}) Init(call __ipc.ServerCall) {
	s.ServerCall = call
}

{{if $method.InStream}}// RecvStream returns the receiver side of the {{$iface.Name}}.{{$method.Name}} server stream.
func (s  *{{$serverContextStub}}) RecvStream() interface {
	Advance() bool
	Value() {{typeGo $data $method.InStream}}
	Err() error
} {
	return {{$serverRecvImpl}}{s}
}

type {{$serverRecvImpl}} struct {
	s *{{$serverContextStub}}
}

func (s {{$serverRecvImpl}}) Advance() bool {
	{{reInitStreamValue $data $method.InStream "s.s.valRecv"}}s.s.errRecv = s.s.Recv(&s.s.valRecv)
	return s.s.errRecv == nil
}
func (s {{$serverRecvImpl}}) Value() {{typeGo $data $method.InStream}} {
	return s.s.valRecv
}
func (s {{$serverRecvImpl}}) Err() error {
	if s.s.errRecv == __io.EOF {
		return nil
	}
	return s.s.errRecv
}
{{end}}{{if $method.OutStream}}// SendStream returns the send side of the {{$iface.Name}}.{{$method.Name}} server stream.
func (s *{{$serverContextStub}}) SendStream() interface {
	Send(item {{typeGo $data $method.OutStream}}) error
} {
	return {{$serverSendImpl}}{s}
}

type {{$serverSendImpl}} struct {
	s *{{$serverContextStub}}
}

func (s {{$serverSendImpl}}) Send(item {{typeGo $data $method.OutStream}}) error {
	return s.s.Send(item)
}
{{end}}
{{end}}{{end}}{{end}}

{{end}}{{end}}
`
