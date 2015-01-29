// Package golang implements Go code generation from compiled VDL packages.
package golang

import (
	"bytes"
	"fmt"
	"go/format"
	"sort"
	"strings"
	"text/template"

	"v.io/core/veyron2/vdl"
	"v.io/core/veyron2/vdl/codegen"
	"v.io/core/veyron2/vdl/compile"
	"v.io/core/veyron2/vdl/parse"
	"v.io/core/veyron2/vdl/vdlroot/src/vdltool"
)

type goData struct {
	File          *compile.File
	Env           *compile.Env
	UserImports   codegen.Imports
	SystemImports []string
}

// Generate takes a populated compile.File and returns a byte slice containing
// the generated Go source code.
func Generate(file *compile.File, env *compile.Env, config vdltool.GoConfig) []byte {
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
	// Use gofmt to format the generated source.
	pretty, err := format.Source(buf.Bytes())
	if err != nil {
		// We shouldn't see an error; it means we generated invalid code.
		fmt.Printf("%s", buf.Bytes())
		panic(fmt.Errorf("vdl: generated invalid Go code: %v", err))
	}
	return pretty
}

// systemImportsGo returns a list of required veyron system imports.
//
// TODO(toddw): Now that we have the userImports mechanism for de-duping local
// package names, we could consider using that instead of our "__" prefix.
// That'll make the template code a bit messier though.
func systemImportsGo(f *compile.File) []string {
	set := make(map[string]bool)
	if f.TypeDeps[vdl.AnyType] {
		// Import for vdl.AnyRep
		set[`__vdl "v.io/core/veyron2/vdl"`] = true
	}
	if f.TypeDeps[vdl.TypeObjectType] {
		// Import for vdl.Type
		set[`__vdl "v.io/core/veyron2/vdl"`] = true
	}
	if len(f.TypeDefs) > 0 {
		// Import for vdl.Register to register all defined types.
		set[`__vdl "v.io/core/veyron2/vdl"`] = true
	}
	for _, td := range f.TypeDefs {
		if td.Type.Kind() == vdl.Enum {
			// Import for fmt.Errorf to define Enum types.
			set[`__fmt "fmt"`] = true
		}
	}
	if len(f.Interfaces) > 0 {
		// Imports for the generated method: {interface name}Client.
		set[`__veyron2 "v.io/core/veyron2"`] = true
		set[`__ipc "v.io/core/veyron2/ipc"`] = true
		set[`__context "v.io/core/veyron2/context"`] = true
		if fileHasStreamingMethods(f) {
			set[`__io "io"`] = true
		}
		if fileHasMethodTags(f) {
			// Import for vdl.AnyRep
			set[`__vdl "v.io/core/veyron2/vdl"`] = true
		}
	}
	// If the user has specified any errors, typically we need to import the
	// "v.io/core/veyron2/verror2" package.  However we allow vdl code-generation
	// in the "v.io/core/veyron2/verror2" package itself, to specify common
	// errors.  Special-case this scenario to avoid self-cyclic package
	// dependencies.
	if len(f.ErrorDefs) > 0 && f.Package.Path != "v.io/core/veyron2/verror2" {
		set[`__verror2 "v.io/core/veyron2/verror2"`] = true
	}
	if len(f.ErrorDefs) > 0 {
		set[`__context "v.io/core/veyron2/context"`] = true
		set[`__i18n "v.io/core/veyron2/i18n"`] = true
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

// fileHasMethodTags returns true iff f contains an interface with a method with
// non-empty tags.
func fileHasMethodTags(f *compile.File) bool {
	for _, i := range f.Interfaces {
		for _, m := range i.Methods {
			if len(m.Tags) > 0 {
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
		"typedConst":            typedConst,
		"typedZeroValue":        typedZeroValue,
		"embedGo":               embedGo,
		"isStreamingMethod":     isStreamingMethod,
		"hasStreamingMethods":   hasStreamingMethods,
		"docBreak":              docBreak,
		"quoteStripDoc":         parse.QuoteStripDoc,
		"argNames":              argNames,
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
		"reInitStreamValue":     reInitStreamValue,
	}
	goTemplate = template.Must(template.New("genGo").Funcs(funcMap).Parse(genGo))
}

func genpkg(file *compile.File, pkg string) string {
	// Special-case code generation for the veyron2/verror2 package, to avoid
	// adding the "__verror2." package qualifier.
	if file.Package.Path == "v.io/core/veyron2/verror2" && pkg == "verror2" {
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

// argNames returns a comma-separated list of each name from args.  If argPrefix
// is empty, the name specified in args is used; otherwise the name is prefixD,
// where D is the position of the argument.
func argNames(boxPrefix, argPrefix, first, last string, args []*compile.Arg) string {
	var result []string
	if first != "" {
		result = append(result, first)
	}
	for ix, arg := range args {
		name := arg.Name
		if argPrefix != "" {
			name = fmt.Sprintf("%s%d", argPrefix, ix)
		}
		if arg.Type == vdl.ErrorType {
			// TODO(toddw): Also need to box user-defined external interfaces.  Or can
			// we remove this special-case now?
			name = boxPrefix + name
		}
		result = append(result, name)
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
	return len(args) > 0 && args[len(args)-1].Type == vdl.ErrorType
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
	if isStreamingMethod(method) {
		return prefix + uniqueName(iface, method, "Context")
	}
	return prefix + "__ipc.ServerContext"
}

// The first arg of every server stub method is a context; the type is either a
// typed context stub for streams, or ipc.ServerContext for non-streams.
func serverContextStubType(prefix string, iface *compile.Interface, method *compile.Method) string {
	if isStreamingMethod(method) {
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
		inargs = "[]interface{}{" + argNames("&", "i", "", "", method.InArgs) + "}"
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
	outargs := argNames("", "&o", "", lastArg, stripFinalError(method.OutArgs))
	return fmt.Sprintf("\tif ierr := %s.Finish(%s); ierr != nil {\n\t\terr = ierr\n\t}", varname, outargs)
}

// serverStubImpl returns the interface method server stub implementation.
func serverStubImpl(data goData, iface *compile.Interface, method *compile.Method) string {
	var buf bytes.Buffer
	inargs := argNames("", "i", "ctx", "", method.InArgs)
	fmt.Fprint(&buf, "\t")
	if len(method.OutArgs) > 0 {
		fmt.Fprint(&buf, "return ")
	}
	fmt.Fprintf(&buf, "s.impl.%s(%s)", method.Name, inargs)
	return buf.String() // the caller writes the trailing newline
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

{{if $file.TypeDefs}}{{range $tdef := $file.TypeDefs}}
{{typeDefGo $data $tdef}}
{{end}}{{end}}
{{if $file.TypeDefs}}func init() { {{range $tdef := $file.TypeDefs}}
	__vdl.Register({{typedZeroValue $data $tdef}}){{end}}
}{{end}}

{{range $cdef := $file.ConstDefs}}
{{constDefGo $data $cdef}}
{{end}}

{{if $file.ErrorDefs}}var ( {{range $edef := $file.ErrorDefs}}
	{{$edef.Doc}}{{$edef.Name}} = {{genpkg $file "verror2"}}Register("{{$edef.ID}}", {{genpkg $file "verror2"}}{{$edef.Action}}, "{{$edef.English}}"){{end}}
)

{{/* TODO(toddw): Don't set "en-US" or "en" again, since it's already set by Register */}}
func init() { {{range $edef := $file.ErrorDefs}}{{range $lf := $edef.Formats}}
	__i18n.Cat().SetWithBase(__i18n.LangID("{{$lf.Lang}}"), __i18n.MsgID({{$edef.Name}}.ID), "{{$lf.Fmt}}"){{end}}{{end}}
}
{{range $edef := $file.ErrorDefs}}
// Make{{$edef.Name}} returns an error with the {{$edef.Name}} ID.
func Make{{$edef.Name}}(ctx {{argNameTypes "" "*__context.T" "" $data $edef.Params}}) {{genpkg $file "verror2"}}E {
	return {{genpkg $file "verror2"}}Make({{$edef.Name}}, {{argNames "" "" "ctx" "" $edef.Params}})
}
{{end}}{{end}}

{{range $iface := $file.Interfaces}}{{$ifaceStreaming := hasStreamingMethods $iface.AllMethods}}
// {{$iface.Name}}ClientMethods is the client interface
// containing {{$iface.Name}} methods.
{{docBreak $iface.Doc}}type {{$iface.Name}}ClientMethods interface { {{range $embed := $iface.Embeds}}
	{{$embed.Doc}}{{embedGo $data $embed}}ClientMethods{{$embed.DocSuffix}}{{end}}{{range $method := $iface.Methods}}
	{{$method.Doc}}{{$method.Name}}({{argNameTypes "" "ctx *__context.T" "opts ...__ipc.CallOpt" $data $method.InArgs}}) {{outArgsClient "" $data $iface $method}}{{$method.DocSuffix}}{{end}}
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

func (c impl{{$iface.Name}}ClientStub) c(ctx *__context.T) __ipc.Client {
	if c.client != nil {
		return c.client
	}
	return __veyron2.GetClient(ctx)
}

{{range $method := $iface.Methods}}
func (c impl{{$iface.Name}}ClientStub) {{$method.Name}}({{argNameTypes "i" "ctx *__context.T" "opts ...__ipc.CallOpt" $data $method.InArgs}}) {{outArgsClient "o" $data $iface $method}} {
{{clientStubImpl $data $iface $method}}
}
{{end}}

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
		// Send places the item onto the output stream.  Returns errors
		// encountered while sending, or if Send is called after Close or
		// the stream has been canceled.  Blocks if there is no buffer
		// space; will unblock when buffer space is available or after
		// the stream has been canceled.
		Send(item {{typeGo $data $method.InStream}}) error
		// Close indicates to the server that no more items will be sent;
		// server Recv calls will receive io.EOF after all sent items.
		// This is an optional call - e.g. a client might call Close if it
		// needs to continue receiving items from the server after it's
		// done sending.  Returns errors encountered while closing, or if
		// Close is called after the stream has been canceled.  Like Send,
		// blocks if there is no buffer space available.
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
	// Finish returns immediately if the call has been canceled; depending on the
	// timing the output could either be an error signaling cancelation, or the
	// valid positional return values from the server.
	//
	// Calling Finish is mandatory for releasing stream resources, unless the call
	// has been canceled or any of the other methods return an error.  Finish should
	// be called at most once.
	Finish() {{argParens (argNameTypes "" "" "err error" $data (stripFinalError $method.OutArgs))}}
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
	// Describe the {{$iface.Name}} interfaces.
	Describe__() []__ipc.InterfaceDesc
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

func (s impl{{$iface.Name}}ServerStub) Globber() *__ipc.GlobState {
	return s.gs
}

func (s impl{{$iface.Name}}ServerStub) Describe__() []__ipc.InterfaceDesc {
	return []__ipc.InterfaceDesc{ {{$iface.Name}}Desc{{range $embed := $iface.TransitiveEmbeds}}, {{embedGo $data $embed}}Desc{{end}} }
}

// {{$iface.Name}}Desc describes the {{$iface.Name}} interface.
var {{$iface.Name}}Desc __ipc.InterfaceDesc = desc{{$iface.Name}}

// desc{{$iface.Name}} hides the desc to keep godoc clean.
var desc{{$iface.Name}} = __ipc.InterfaceDesc{ {{if $iface.Name}}
	Name: "{{$iface.Name}}",{{end}}{{if $iface.File.Package.Path}}
	PkgPath: "{{$iface.File.Package.Path}}",{{end}}{{if $iface.Doc}}
	Doc: {{quoteStripDoc $iface.Doc}},{{end}}{{if $iface.Embeds}}
	Embeds: []__ipc.EmbedDesc{ {{range $embed := $iface.Embeds}}
		{ "{{$embed.Name}}", "{{$embed.File.Package.Path}}", {{quoteStripDoc $embed.Doc}} },{{end}}
	},{{end}}{{if $iface.Methods}}
	Methods: []__ipc.MethodDesc{ {{range $method := $iface.Methods}}
		{ {{if $method.Name}}
			Name: "{{$method.Name}}",{{end}}{{if $method.Doc}}
			Doc: {{quoteStripDoc $method.Doc}},{{end}}{{if $method.InArgs}}
			InArgs: []__ipc.ArgDesc{ {{range $arg := $method.InArgs}}
				{ "{{$arg.Name}}", {{quoteStripDoc $arg.Doc}} }, // {{typeGo $data $arg.Type}}{{end}}
			},{{end}}{{if $method.OutArgs}}
			OutArgs: []__ipc.ArgDesc{ {{range $arg := $method.OutArgs}}
				{ "{{$arg.Name}}", {{quoteStripDoc $arg.Doc}} }, // {{typeGo $data $arg.Type}}{{end}}
			},{{end}}{{if $method.Tags}}
			Tags: []__vdl.AnyRep{ {{range $tag := $method.Tags}}{{typedConst $data $tag}} ,{{end}} },{{end}}
		},{{end}}
	},{{end}}
}

{{range $method := $iface.Methods}}
{{if isStreamingMethod $method}}
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
{{end}}{{end}}{{end}}

{{end}}{{end}}
`
