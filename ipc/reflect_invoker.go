package ipc

import (
	"fmt"
	"reflect"
	"sort"
	"sync"

	"v.io/core/veyron2/naming"
	"v.io/core/veyron2/vdl"
	"v.io/core/veyron2/vdl/valconv"
	"v.io/core/veyron2/vdl/vdlroot/src/signature"
	"v.io/core/veyron2/verror"
)

// Describer may be implemented by an underlying object served by the
// ReflectInvoker, in order to describe the interfaces that the object
// implements.  This describes all data in signature.Interface that the
// ReflectInvoker cannot obtain through reflection; basically everything except
// the method names and types.
//
// Note that a single object may implement multiple interfaces; to describe such
// an object, simply return more than one elem in the returned list.
type Describer interface {
	// Describe the underlying object.  The implementation must be idempotent
	// across different instances of the same underlying type; the ReflectInvoker
	// calls this once per type and caches the results.
	Describe__() []InterfaceDesc
}

// InterfaceDesc describes an interface; it is similar to signature.Interface,
// without the information that can be obtained via reflection.
type InterfaceDesc struct {
	Name    string
	PkgPath string
	Doc     string
	Embeds  []EmbedDesc
	Methods []MethodDesc
}

// EmbedDesc describes an embedded interface; it is similar to signature.Embed,
// without the information that can be obtained via reflection.
type EmbedDesc struct {
	Name    string
	PkgPath string
	Doc     string
}

// MethodDesc describes an interface method; it is similar to signature.Method,
// without the information that can be obtained via reflection.
type MethodDesc struct {
	Name      string
	Doc       string
	InArgs    []ArgDesc    // Input arguments
	OutArgs   []ArgDesc    // Output arguments
	InStream  ArgDesc      // Input stream (client to server)
	OutStream ArgDesc      // Output stream (server to client)
	Tags      []vdl.AnyRep // Method tags
}

// ArgDesc describes an argument; it is similar to signature.Arg, without the
// information that can be obtained via reflection.
type ArgDesc struct {
	Name string
	Doc  string
}

type reflectInvoker struct {
	rcvr    reflect.Value
	methods map[string]methodInfo // used by Prepare and Invoke
	sig     []signature.Interface // used by Signature and MethodSignature
}

var _ Invoker = (*reflectInvoker)(nil)

// methodInfo holds the runtime information necessary for Prepare and Invoke.
type methodInfo struct {
	rvFunc   reflect.Value  // Function representing the method.
	rtInArgs []reflect.Type // In arg types, not including receiver and context.

	// rtStreamCtx holds the type of the typesafe streaming context, if any.
	// rvStreamCtxInit is the associated Init function.
	rtStreamCtx     reflect.Type
	rvStreamCtxInit reflect.Value

	tags []interface{} // Tags from the signature.
}

// ReflectInvoker returns an Invoker implementation that uses reflection to make
// each compatible exported method in obj available.  E.g.:
//
//   type impl struct{}
//   func (impl) NonStreaming(ctx ipc.ServerContext, ...) (...)
//   func (impl) Streaming(ctx *MyContext, ...) (...)
//
// The first in-arg must be a context.  For non-streaming methods it must be
// ipc.ServerContext.  For streaming methods, it must be a pointer to a struct
// that implements ipc.ServerCall, and also adds typesafe streaming wrappers.
// Here's an example that streams int32 from client to server, and string from
// server to client:
//
//   type MyContext struct { ipc.ServerCall }
//
//   // Init initializes MyContext via ipc.ServerCall.
//   func (*MyContext) Init(ipc.ServerCall) {...}
//
//   // RecvStream returns the receiver side of the server stream.
//   func (*MyContext) RecvStream() interface {
//     Advance() bool
//     Value() int32
//     Err() error
//   } {...}
//
//   // SendStream returns the sender side of the server stream.
//   func (*MyContext) SendStream() interface {
//     Send(item string) error
//   } {...}
//
// We require the streaming context arg to have this structure so that we can
// capture the streaming in and out arg types via reflection.  We require it to
// be a concrete type with an Init func so that we can create new instances,
// also via reflection.
//
// As a temporary special-case, we also allow generic streaming methods:
//
//   func (impl) Generic(call ipc.ServerCall, ...) (...)
//
// The problem with allowing this form is that via reflection we can no longer
// determine whether the server performs streaming, or what the streaming in and
// out types are.
// TODO(toddw): Remove this special-case.
//
// The ReflectInvoker silently ignores unexported methods, and exported methods
// whose first argument doesn't implement ipc.ServerContext.  All other methods
// must follow the above rules; bad method types cause an error to be returned.
//
// If obj implements the Describer interface, we'll use it to describe portions
// of the object signature that cannot be retrieved via reflection;
// e.g. method tags, documentation, variable names, etc.
func ReflectInvoker(obj interface{}) (Invoker, error) {
	rt := reflect.TypeOf(obj)
	info := reflectCache.lookup(rt)
	if info == nil {
		// Concurrent calls may cause reflectCache.set to be called multiple times.
		// This race is benign; the info for a given type never changes.
		var err error
		if info, err = newReflectInfo(obj); err != nil {
			return nil, err
		}
		reflectCache.set(rt, info)
	}
	return reflectInvoker{reflect.ValueOf(obj), info.methods, info.sig}, nil
}

// ReflectInvokerOrDie is the same as ReflectInvoker, but panics on all errors.
func ReflectInvokerOrDie(obj interface{}) Invoker {
	invoker, err := ReflectInvoker(obj)
	if err != nil {
		panic(err)
	}
	return invoker
}

// Prepare implements the Invoker.Prepare method.
func (ri reflectInvoker) Prepare(method string, _ int) ([]interface{}, []interface{}, error) {
	info, ok := ri.methods[method]
	if !ok {
		return nil, nil, NewErrUnknownMethod(nil, method)
	}
	// Return the tags and new in-arg objects.
	var argptrs []interface{}
	if len(info.rtInArgs) > 0 {
		argptrs = make([]interface{}, len(info.rtInArgs))
		for ix, rtInArg := range info.rtInArgs {
			argptrs[ix] = reflect.New(rtInArg).Interface()
		}
	}

	return argptrs, info.tags, nil
}

// Invoke implements the Invoker.Invoke method.
func (ri reflectInvoker) Invoke(method string, call ServerCall, argptrs []interface{}) ([]interface{}, error) {
	info, ok := ri.methods[method]
	if !ok {
		return nil, NewErrUnknownMethod(nil, method)
	}
	// Create the reflect.Value args for the invocation.  The receiver of the
	// method is always first, followed by the required context arg.
	rvArgs := make([]reflect.Value, len(argptrs)+2)
	rvArgs[0] = ri.rcvr
	if info.rtStreamCtx == nil {
		// There isn't a typesafe streaming context, just use the call.
		rvArgs[1] = reflect.ValueOf(call)
	} else {
		// There is a typesafe streaming context with type rtStreamCtx.  We perform
		// the equivalent of the following:
		//   ctx := new(rtStreamCtx)
		//   ctx.Init(call)
		ctx := reflect.New(info.rtStreamCtx)
		info.rvStreamCtxInit.Call([]reflect.Value{ctx, reflect.ValueOf(call)})
		rvArgs[1] = ctx
	}
	// Positional user args follow.
	for ix, argptr := range argptrs {
		rvArgs[ix+2] = reflect.ValueOf(argptr).Elem()
	}
	// Invoke the method, and convert results into interface{}.
	rvResults := info.rvFunc.Call(rvArgs)
	if len(rvResults) == 0 {
		return nil, nil
	}
	results := make([]interface{}, len(rvResults))
	for ix, r := range rvResults {
		results[ix] = r.Interface()
	}
	return results, nil
}

// Signature implements the Invoker.Signature method.
func (ri reflectInvoker) Signature(ctx ServerContext) ([]signature.Interface, error) {
	return signature.CopyInterfaces(ri.sig), nil
}

// MethodSignature implements the Invoker.MethodSignature method.
func (ri reflectInvoker) MethodSignature(ctx ServerContext, method string) (signature.Method, error) {
	// Return the first method in any interface with the given method name.
	for _, iface := range ri.sig {
		if msig, ok := iface.FindMethod(method); ok {
			return signature.CopyMethod(msig), nil
		}
	}
	return signature.Method{}, NewErrUnknownMethod(nil, method)
}

// Globber implements the ipc.Globber interface.
func (ri reflectInvoker) Globber() *GlobState {
	return determineGlobState(ri.rcvr.Interface())
}

// reflectRegistry is a locked map from reflect.Type to reflection info, which
// is expensive to compute.  The only instance is reflectCache, which is a
// global cache to speed up repeated lookups.  There is no GC; the total set of
// types in a single address space is expected to be bounded and small.
type reflectRegistry struct {
	sync.RWMutex
	infoMap map[reflect.Type]*reflectInfo
}

type reflectInfo struct {
	methods map[string]methodInfo
	sig     []signature.Interface
}

func (reg *reflectRegistry) lookup(rt reflect.Type) *reflectInfo {
	reg.RLock()
	info := reg.infoMap[rt]
	reg.RUnlock()
	return info
}

// set the entry for (rt, info).  Is a no-op if rt already exists in the map.
func (reg *reflectRegistry) set(rt reflect.Type, info *reflectInfo) {
	reg.Lock()
	if exist := reg.infoMap[rt]; exist == nil {
		reg.infoMap[rt] = info
	}
	reg.Unlock()
}

var reflectCache = &reflectRegistry{infoMap: make(map[reflect.Type]*reflectInfo)}

// newReflectInfo returns reflection information that is expensive to compute.
// Although it is passed an object rather than a type, it guarantees that the
// returned information is always the same for all instances of a given type.
func newReflectInfo(obj interface{}) (*reflectInfo, error) {
	if obj == nil {
		return nil, fmt.Errorf("ipc: ReflectInvoker(nil) is invalid")
	}
	// First make methodInfos, based on reflect.Type, which also captures the name
	// and in, out and streaming types of each method in methodSigs.  This
	// information is guaranteed to be correct, since it's based on reflection on
	// the underlying object.
	rt := reflect.TypeOf(obj)
	methodInfos, methodSigs, err := makeMethods(rt)
	switch {
	case err != nil:
		return nil, err
	case len(methodInfos) == 0 && determineGlobState(obj) == nil:
		return nil, fmt.Errorf("ipc: type %v has no compatible methods: %q", rt, TypeCheckMethods(obj))
	}
	// Now attach method tags to each methodInfo.  Since this is based on the desc
	// provided by the user, there's no guarantee it's "correct", but if the same
	// method is described by multiple interfaces, we check the tags are the same.
	desc := describe(obj)
	if verr := attachMethodTags(methodInfos, desc); verror.Is(verr, verror.Aborted.ID) {
		return nil, fmt.Errorf("ipc: type %v tag error: %v", rt, verr)
	}
	// Finally create the signature.  This combines the desc provided by the user
	// with the methodSigs computed via reflection.  We ensure that the method
	// names and types computed via reflection always remains in the final sig;
	// the desc is merely used to augment the signature.
	sig := makeSig(desc, methodSigs)
	return &reflectInfo{methodInfos, sig}, nil
}

// determineGlobState determines whether and how obj implements Glob.  Returns
// nil iff obj doesn't implement Glob, based solely on the type of obj.
func determineGlobState(obj interface{}) *GlobState {
	if x, ok := obj.(Globber); ok {
		return x.Globber()
	}
	return NewGlobState(obj)
}

func describe(obj interface{}) []InterfaceDesc {
	if d, ok := obj.(Describer); ok {
		// Describe__ must not vary across instances of the same underlying type.
		return d.Describe__()
	}
	return nil
}

func makeMethods(rt reflect.Type) (map[string]methodInfo, map[string]signature.Method, error) {
	infos := make(map[string]methodInfo, rt.NumMethod())
	sigs := make(map[string]signature.Method, rt.NumMethod())
	for mx := 0; mx < rt.NumMethod(); mx++ {
		method := rt.Method(mx)
		// Silently skip incompatible methods, except for Aborted errors.
		var sig signature.Method
		if verr := typeCheckMethod(method, &sig); verr != nil {
			if verror.Is(verr, verror.Aborted.ID) {
				return nil, nil, fmt.Errorf("ipc: %s.%s: %v", rt.String(), method.Name, verr)
			}
			continue
		}
		infos[method.Name] = makeMethodInfo(method)
		sigs[method.Name] = sig
	}
	return infos, sigs, nil
}

func makeMethodInfo(method reflect.Method) methodInfo {
	info := methodInfo{rvFunc: method.Func}
	mtype := method.Type
	for ix := 2; ix < mtype.NumIn(); ix++ { // Skip receiver and context
		info.rtInArgs = append(info.rtInArgs, mtype.In(ix))
	}
	// Initialize info for typesafe streaming contexts.  Note that we've already
	// type-checked the method.  We memoize the stream type and Init function, so
	// that we can create and initialize the stream type in Invoke.
	if in1 := mtype.In(1); in1 != rtServerCall && in1 != rtServerContext && in1.Kind() == reflect.Ptr {
		info.rtStreamCtx = in1.Elem()
		mInit, _ := in1.MethodByName("Init")
		info.rvStreamCtxInit = mInit.Func
	}
	return info
}

func abortedf(format string, v ...interface{}) error {
	return verror.New(verror.Aborted, nil, fmt.Sprintf(format, v...))
}

var (
	rtServerCall           = reflect.TypeOf((*ServerCall)(nil)).Elem()
	rtServerContext        = reflect.TypeOf((*ServerContext)(nil)).Elem()
	rtBool                 = reflect.TypeOf(bool(false))
	rtError                = reflect.TypeOf((*error)(nil)).Elem()
	rtString               = reflect.TypeOf("")
	rtStringChan           = reflect.TypeOf((<-chan string)(nil))
	rtMountEntryChan       = reflect.TypeOf((<-chan naming.VDLMountEntry)(nil))
	rtPtrToGlobState       = reflect.TypeOf((*GlobState)(nil))
	rtSliceOfInterfaceDesc = reflect.TypeOf([]InterfaceDesc{})

	// ReflectInvoker will panic iff the error is Aborted, otherwise it will
	// silently ignore the error.
	ErrReservedMethod    = verror.New(verror.Internal, nil, "Reserved method")
	ErrMethodNotExported = verror.New(verror.BadArg, nil, "Method not exported")
	ErrNonRPCMethod      = verror.New(verror.BadArg, nil, "Non-rpc method.  We require at least 1 in-arg."+useContext)
	ErrInServerCall      = abortedf("Context arg ipc.ServerCall is invalid; cannot determine streaming types." + forgotWrap)
	ErrBadDescribe       = abortedf("Describe__ must have signature Describe__() []ipc.InterfaceDesc")
	ErrBadGlobber        = abortedf("Globber must have signature Globber() *ipc.GlobState")
	ErrBadGlob           = abortedf("Glob__ must have signature Glob__(ipc.ServerContext, pattern string) (<-chan naming.VDLMountEntry, error)")
	ErrBadGlobChildren   = abortedf("GlobChildren__ must have signature GlobChildren__(ipc.ServerContext) (<-chan string, error)")
)

func typeCheckMethod(method reflect.Method, sig *signature.Method) error {
	if verr := typeCheckReservedMethod(method); verr != nil {
		return verr
	}
	// Unexported methods always have a non-empty pkg path.
	if method.PkgPath != "" {
		return ErrMethodNotExported
	}
	sig.Name = method.Name
	mtype := method.Type
	// Method must have at least 2 in args (receiver, context).
	if in := mtype.NumIn(); in < 2 {
		return ErrNonRPCMethod
	}
	switch in1 := mtype.In(1); {
	case in1 == rtServerCall:
		// If the first context arg is ipc.ServerCall, we do not know whether the
		// method performs streaming, or what the stream types are.
		sig.InStream = &signature.Arg{Type: vdl.AnyType}
		sig.OutStream = &signature.Arg{Type: vdl.AnyType}
		// We can either disallow ipc.ServerCall, at the expense of more boilerplate
		// for users that don't use the VDL but want to perform streaming.  Or we
		// can allow it, but won't be able to determine whether the server uses the
		// stream, or what the streaming types are.
		//
		// At the moment we allow it; we can easily disallow by enabling this error.
		//   return ErrInServerCall
	case in1 == rtServerContext:
		// Non-streaming method.
	case in1.Implements(rtServerContext):
		// Streaming method, validate context argument.
		if verr := typeCheckStreamingContext(in1, sig); verr != nil {
			return verr
		}
	default:
		return ErrNonRPCMethod
	}
	return typeCheckMethodArgs(mtype, sig)
}

func typeCheckReservedMethod(method reflect.Method) error {
	switch method.Name {
	case "Describe__":
		// Describe__() []InterfaceDesc
		if t := method.Type; t.NumIn() != 1 || t.NumOut() != 1 ||
			t.Out(0) != rtSliceOfInterfaceDesc {
			return ErrBadDescribe
		}
		return ErrReservedMethod
	case "Globber":
		// Globber() *GlobState
		if t := method.Type; t.NumIn() != 1 || t.NumOut() != 1 ||
			t.Out(0) != rtPtrToGlobState {
			return ErrBadGlobber
		}
		return ErrReservedMethod
	case "Glob__":
		// Glob__(ipc.ServerContext, string) (<-chan naming.VDLMountEntry, error)
		if t := method.Type; t.NumIn() != 3 || t.NumOut() != 2 ||
			t.In(1) != rtServerContext || t.In(2) != rtString ||
			t.Out(0) != rtMountEntryChan || t.Out(1) != rtError {
			return ErrBadGlob
		}
		return ErrReservedMethod
	case "GlobChildren__":
		// GlobChildren__(ipc.ServerContext) (<-chan string, error)
		if t := method.Type; t.NumIn() != 2 || t.NumOut() != 2 ||
			t.In(1) != rtServerContext ||
			t.Out(0) != rtStringChan || t.Out(1) != rtError {
			return ErrBadGlobChildren
		}
		return ErrReservedMethod
	}
	return nil
}

func typeCheckStreamingContext(ctx reflect.Type, sig *signature.Method) error {
	// The context must be a pointer to a struct.
	if ctx.Kind() != reflect.Ptr || ctx.Elem().Kind() != reflect.Struct {
		return abortedf("Context arg %s is invalid streaming context; must be pointer to a struct representing the typesafe streaming context."+forgotWrap, ctx)
	}
	// Must have Init(ipc.ServerCall) method.
	mInit, hasInit := ctx.MethodByName("Init")
	if !hasInit {
		return abortedf("Context arg %s is invalid streaming context; must have Init method."+forgotWrap, ctx)
	}
	if t := mInit.Type; t.NumIn() != 2 || t.In(0).Kind() != reflect.Ptr || t.In(1) != rtServerCall || t.NumOut() != 0 {
		return abortedf("Context arg %s is invalid streaming context; Init must have signature func (*) Init(ipc.ServerCall)."+forgotWrap, ctx)
	}
	// Must have either RecvStream or SendStream method, or both.
	mRecvStream, hasRecvStream := ctx.MethodByName("RecvStream")
	mSendStream, hasSendStream := ctx.MethodByName("SendStream")
	if !hasRecvStream && !hasSendStream {
		return abortedf("Context arg %s is invalid streaming context; must have at least one of RecvStream or SendStream methods."+forgotWrap, ctx)
	}
	if hasRecvStream {
		// func (*) RecvStream() interface{ Advance() bool; Value() _; Err() error }
		tRecv := mRecvStream.Type
		if tRecv.NumIn() != 1 || tRecv.In(0).Kind() != reflect.Ptr ||
			tRecv.NumOut() != 1 || tRecv.Out(0).Kind() != reflect.Interface {
			return errRecvStream(ctx)
		}
		mA, hasA := tRecv.Out(0).MethodByName("Advance")
		mV, hasV := tRecv.Out(0).MethodByName("Value")
		mE, hasE := tRecv.Out(0).MethodByName("Err")
		tA, tV, tE := mA.Type, mV.Type, mE.Type
		if !hasA || !hasV || !hasE ||
			tA.NumIn() != 0 || tA.NumOut() != 1 || tA.Out(0) != rtBool ||
			tV.NumIn() != 0 || tV.NumOut() != 1 || // tV.Out(0) is in-stream type
			tE.NumIn() != 0 || tE.NumOut() != 1 || tE.Out(0) != rtError {
			return errRecvStream(ctx)
		}
		inType, err := vdl.TypeFromReflect(tV.Out(0))
		if err != nil {
			return abortedf("Invalid in-stream type: %v", err)
		}
		sig.InStream = &signature.Arg{Type: inType}
	}
	if hasSendStream {
		// func (*) SendStream() interface{ Send(_) error }
		tSend := mSendStream.Type
		if tSend.NumIn() != 1 || tSend.In(0).Kind() != reflect.Ptr ||
			tSend.NumOut() != 1 || tSend.Out(0).Kind() != reflect.Interface {
			return errSendStream(ctx)
		}
		mS, hasS := tSend.Out(0).MethodByName("Send")
		tS := mS.Type
		if !hasS ||
			tS.NumIn() != 1 || // tS.In(0) is out-stream type
			tS.NumOut() != 1 || tS.Out(0) != rtError {
			return errSendStream(ctx)
		}
		outType, err := vdl.TypeFromReflect(tS.In(0))
		if err != nil {
			return abortedf("Invalid out-stream type: %v", err)
		}
		sig.OutStream = &signature.Arg{Type: outType}
	}
	return nil
}

const (
	useContext = "  Use either ipc.ServerContext for non-streaming methods, or use a non-interface typesafe context for streaming methods."
	forgotWrap = useContext + "  Perhaps you forgot to wrap your server with the VDL-generated server stub."
)

func errRecvStream(rt reflect.Type) error {
	return abortedf("Context arg %s is invalid streaming context; RecvStream must have signature func (*) RecvStream() interface{ Advance() bool; Value() _; Err() error }."+forgotWrap, rt)
}

func errSendStream(rt reflect.Type) error {
	return abortedf("Context arg %s is invalid streaming context; SendStream must have signature func (*) SendStream() interface{ Send(_) error }."+forgotWrap, rt)
}

func typeCheckMethodArgs(mtype reflect.Type, sig *signature.Method) error {
	// Start in-args from 2 to skip receiver and context arguments.
	for index := 2; index < mtype.NumIn(); index++ {
		vdlType, err := vdl.TypeFromReflect(mtype.In(index))
		if err != nil {
			return abortedf("Invalid in-arg %d type: %v", index-2, err)
		}
		(*sig).InArgs = append((*sig).InArgs, signature.Arg{Type: vdlType})
	}
	for index := 0; index < mtype.NumOut(); index++ {
		vdlType, err := vdl.TypeFromReflect(mtype.Out(index))
		if err != nil {
			return abortedf("Invalid out-arg %d type: %v", index, err)
		}
		(*sig).OutArgs = append((*sig).OutArgs, signature.Arg{Type: vdlType})
	}
	return nil
}

func makeSig(desc []InterfaceDesc, methods map[string]signature.Method) []signature.Interface {
	var sig []signature.Interface
	used := make(map[string]bool, len(methods))
	// Loop through the user-provided desc, attaching descriptions to the actual
	// method types to create our final signatures.  Ignore user-provided
	// descriptions of interfaces or methods that don't exist.
	for _, descIface := range desc {
		var sigMethods []signature.Method
		for _, descMethod := range descIface.Methods {
			sigMethod, ok := methods[descMethod.Name]
			if ok {
				// The method name and all types are already populated in sigMethod;
				// fill in the rest of the description.
				sigMethod.Doc = descMethod.Doc
				sigMethod.InArgs = makeArgSigs(sigMethod.InArgs, descMethod.InArgs)
				sigMethod.OutArgs = makeArgSigs(sigMethod.OutArgs, descMethod.OutArgs)
				sigMethod.InStream = fillArgSig(sigMethod.InStream, descMethod.InStream)
				sigMethod.OutStream = fillArgSig(sigMethod.OutStream, descMethod.OutStream)
				sigMethod.Tags = descMethod.Tags
				sigMethods = append(sigMethods, sigMethod)
				used[sigMethod.Name] = true
			}
		}
		if len(sigMethods) > 0 {
			sort.Sort(signature.SortableMethods(sigMethods))
			sigIface := signature.Interface{
				Name:    descIface.Name,
				PkgPath: descIface.PkgPath,
				Doc:     descIface.Doc,
				Methods: sigMethods,
			}
			for _, descEmbed := range descIface.Embeds {
				sigEmbed := signature.Embed{
					Name:    descEmbed.Name,
					PkgPath: descEmbed.PkgPath,
					Doc:     descEmbed.Doc,
				}
				sigIface.Embeds = append(sigIface.Embeds, sigEmbed)
			}
			sig = append(sig, sigIface)
		}
	}
	// Add all unused methods into the catch-all empty interface.
	var unusedMethods []signature.Method
	for _, method := range methods {
		if !used[method.Name] {
			unusedMethods = append(unusedMethods, method)
		}
	}
	if len(unusedMethods) > 0 {
		const unusedDoc = "The empty interface contains methods not attached to any interface."
		sort.Sort(signature.SortableMethods(unusedMethods))
		sig = append(sig, signature.Interface{Doc: unusedDoc, Methods: unusedMethods})
	}
	return sig
}

func makeArgSigs(sigs []signature.Arg, descs []ArgDesc) []signature.Arg {
	result := make([]signature.Arg, len(sigs))
	for index, sig := range sigs {
		if index < len(descs) {
			sig = *fillArgSig(&sig, descs[index])
		}
		result[index] = sig
	}
	return result
}

func fillArgSig(sig *signature.Arg, desc ArgDesc) *signature.Arg {
	if sig == nil {
		return nil
	}
	ret := *sig
	ret.Name = desc.Name
	ret.Doc = desc.Doc
	return &ret
}

// convertTags tries to convert each tag into a vdl.Value, to capture any
// conversion error.  Returns a single vdl.Value containing a slice of the tag
// values, to make it easy to perform equality-comparison of lists of tags.
func convertTags(tags []vdl.AnyRep) (*vdl.Value, error) {
	result := vdl.ZeroValue(vdl.ListType(vdl.AnyType)).AssignLen(len(tags))
	for index, tag := range tags {
		if err := valconv.Convert(result.Index(index), tag); err != nil {
			return nil, abortedf("invalid tag %d: %v", index, err)
		}
	}
	return result, nil
}

// extractTagsForMethod returns the tags associated with the given method name.
// If the desc lists the same method under multiple interfaces, we require all
// versions to have an identical list of tags.
func extractTagsForMethod(desc []InterfaceDesc, name string) ([]vdl.AnyRep, error) {
	var first *vdl.Value
	var tags []vdl.AnyRep
	for _, descIface := range desc {
		for _, descMethod := range descIface.Methods {
			if name == descMethod.Name {
				switch vdlTags, verr := convertTags(descMethod.Tags); {
				case verr != nil:
					return nil, verr
				case first == nil:
					first = vdlTags
					tags = descMethod.Tags
				case !vdl.EqualValue(first, vdlTags):
					return nil, abortedf("different tags %q and %q", first, vdlTags)
				}
			}
		}
	}
	return tags, nil
}

// attachMethodTags sets methodInfo.tags to the tags that will be returned in
// Prepare.  This also performs type checking on the tags.
func attachMethodTags(infos map[string]methodInfo, desc []InterfaceDesc) error {
	for name, info := range infos {
		tags, verr := extractTagsForMethod(desc, name)
		if verr != nil {
			return abortedf("method %q: %v", name, verr)
		}
		// TODO(toddw): Change tags to []vdl.AnyRep and remove this conversion.
		if len(tags) > 0 {
			info.tags = make([]interface{}, len(tags))
			for index, tag := range tags {
				info.tags[index] = tag
			}
			infos[name] = info
		}
	}
	return nil
}

// TypeCheckMethods type checks each method in obj, and returns a map from
// method name to the type check result.  Nil errors indicate the method is
// invocable by the Invoker returned by ReflectInvoker(obj).  Non-nil errors
// contain details of the type mismatch - any error with the "Aborted" id will
// cause a panic in a ReflectInvoker() call.
//
// This is useful for debugging why a particular method isn't available via
// ReflectInvoker.
func TypeCheckMethods(obj interface{}) map[string]error {
	rt, desc := reflect.TypeOf(obj), describe(obj)
	var check map[string]error
	if rt != nil && rt.NumMethod() > 0 {
		check = make(map[string]error, rt.NumMethod())
		for mx := 0; mx < rt.NumMethod(); mx++ {
			method := rt.Method(mx)
			var sig signature.Method
			verr := typeCheckMethod(method, &sig)
			if verr == nil {
				_, verr = extractTagsForMethod(desc, method.Name)
			}
			check[method.Name] = verr
		}
	}
	return check
}
