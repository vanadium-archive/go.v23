package ipc

import (
	"fmt"
	"reflect"
	"sync"

	"veyron.io/veyron/veyron2/verror"
)

type reflectInvoker struct {
	rcvr    reflect.Value
	methods map[string]methodInfo
	gs      *GlobState
}

var _ Invoker = (*reflectInvoker)(nil)

type methodInfo struct {
	rvFunc   reflect.Value  // Function representing the method.
	rtInArgs []reflect.Type // In arg types, not including receiver and context.

	// rtStreamCtx holds the type of the typesafe streaming context, if any.
	// rvStreamCtxInit is the associated Init function.
	rtStreamCtx     reflect.Type
	rvStreamCtxInit reflect.Value

	// Tags are not memoized since they may change per instance.
	tags []interface{}
}

// ReflectInvoker returns an Invoker implementation that uses reflection to make
// each compatible exported method in obj available.  Each exported method must
// take ipc.ServerCall as the first in-arg. Methods not doing so are silently
// ignored.
func ReflectInvoker(obj interface{}) Invoker {
	if obj == nil {
		panic(fmt.Errorf("ipc: nil object is incompatible with ReflectInvoker"))
	}
	rcvr := reflect.ValueOf(obj)
	methods := getMethods(rcvr.Type())
	// Copy the memoized methods, so we can fill in the tags.
	methodsCopy := make(map[string]methodInfo, len(methods))
	for name, info := range methods {
		info.tags = getMethodTags(obj, name)
		methodsCopy[name] = info
	}
	// Determine whether and how the object implements Glob.
	gs := &GlobState{}
	if x, ok := obj.(VGlobber); ok {
		gs = x.VGlob()
	}
	if x, ok := obj.(VAllGlobber); ok {
		gs.VAllGlobber = x
	}
	if x, ok := obj.(VChildrenGlobber); ok {
		gs.VChildrenGlobber = x
	}
	if len(methods) == 0 && (gs == nil || (gs.VAllGlobber == nil && gs.VChildrenGlobber == nil)) {
		panic(fmt.Errorf("ipc: object type %T has no compatible methods: %q", obj, TypeCheckMethods(obj)))
	}
	return reflectInvoker{rcvr, methodsCopy, gs}
}

// serverGetMethodTags is a helper interface to check if GetMethodTags is implemented on an object
type serverGetMethodTags interface {
	GetMethodTags(c ServerContext, method string) ([]interface{}, error)
}

// getMethodTags extracts any tags associated with obj.method
func getMethodTags(obj interface{}, method string) []interface{} {
	if tagger, _ := obj.(serverGetMethodTags); tagger != nil {
		tags, _ := tagger.GetMethodTags(nil, method)
		return tags
	}
	return nil
}

// Prepare implements the Invoker.Prepare method.
func (ri reflectInvoker) Prepare(method string, _ int) ([]interface{}, []interface{}, error) {
	info, ok := ri.methods[method]
	if !ok {
		return nil, nil, verror.NoExistf("ipc: unknown method '%s'", method)
	}
	// Return the memoized tags and new in-arg objects.
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
		return nil, verror.NoExistf("ipc: unknown method '%s'", method)
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

// VGlob returns GlobState for the object, e.g. whether and how it implements
// the Glob interface.
func (ri reflectInvoker) VGlob() *GlobState {
	return ri.gs
}

// We memoize the results of makeMethods in a registry, to avoid expensive
// reflection calls.  There is no GC; the total set of types in a single address
// space is expected to be bounded and small.
var (
	methodsRegistry   map[reflect.Type]map[string]methodInfo
	methodsRegistryMu sync.RWMutex
)

func init() {
	methodsRegistry = make(map[reflect.Type]map[string]methodInfo)
}

func getMethods(rt reflect.Type) map[string]methodInfo {
	methodsRegistryMu.RLock()
	exist, ok := methodsRegistry[rt]
	methodsRegistryMu.RUnlock()
	if ok {
		// Return previously memoized result.
		return exist
	}
	// There is a race here; if getMethods is called concurrently, multiple
	// goroutines may each make their own method map.  This is wasted work, but
	// still correct - each newly created method map is identical.
	methods := makeMethods(rt)
	methodsRegistryMu.Lock()
	if exist, ok := methodsRegistry[rt]; ok {
		methodsRegistryMu.Unlock()
		return exist
	}
	methodsRegistry[rt] = methods
	methodsRegistryMu.Unlock()
	return methods
}

func makeMethods(rt reflect.Type) map[string]methodInfo {
	methods := make(map[string]methodInfo, rt.NumMethod())
	for mx := 0; mx < rt.NumMethod(); mx++ {
		method := rt.Method(mx)
		// Silently skip incompatible methods, except for Aborted errors.
		info, verr := makeMethodInfo(method)
		if verr != nil {
			if verror.Is(verr, verror.Aborted) {
				panic(fmt.Errorf("%s.%s: %v", rt.String(), method.Name, verr))
			}
			continue
		}
		methods[method.Name] = info
	}
	return methods
}

// makeMethodInfo makes a methodInfo from a method type.
func makeMethodInfo(method reflect.Method) (methodInfo, verror.E) {
	if verr := typeCheckMethod(method); verr != nil {
		return methodInfo{}, verr
	}
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
	return info, nil
}

var (
	rtServerCall     = reflect.TypeOf((*ServerCall)(nil)).Elem()
	rtServerContext  = reflect.TypeOf((*ServerContext)(nil)).Elem()
	rtBool           = reflect.TypeOf(bool(false))
	rtError          = reflect.TypeOf((*error)(nil)).Elem()
	rtSliceOfString  = reflect.TypeOf([]string{})
	rtPtrToGlobState = reflect.TypeOf((*GlobState)(nil))

	// ReflectInvoker will panic iff the error is Aborted, otherwise it will
	// silently ignore the error.
	ErrReservedMethod    = verror.Internalf("Reserved method")
	ErrMethodNotExported = verror.BadArgf("Method not exported")
	ErrNonRPCMethod      = verror.BadArgf("Non-rpc method.  We require at least 1 in-arg." + useContext)
	ErrInServerCall      = verror.Abortedf("First in-arg ipc.ServerCall is invalid; cannot determine streaming types." + forgotWrap)
	ErrBadVGlob          = verror.Abortedf("VGlob must have signature VGlob() *ipc.GlobState")
	ErrBadVGlobChildren  = verror.Abortedf("VGlobChildren must have signature VGlobChildren() ([]string, error)")
)

func typeCheckMethod(method reflect.Method) verror.E {
	if verr := typeCheckReservedMethod(method); verr != nil {
		return verr
	}
	// Unexported methods always have a non-empty pkg path.
	if method.PkgPath != "" {
		return ErrMethodNotExported
	}
	mtype := method.Type
	// Method must have at least 2 in args (receiver, context).
	if in := mtype.NumIn(); in < 2 {
		return ErrNonRPCMethod
	}
	switch in1 := mtype.In(1); {
	case in1 == rtServerCall:
		// If the first context arg is ipc.ServerCall, there's no way for us to
		// figure out whether the method performs streaming, or what the stream
		// types are.
		//
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
		if verr := typeCheckStreamingContext(in1); verr != nil {
			return verr
		}
	default:
		return ErrNonRPCMethod
	}
	// TODO(toddw): Ensure all arg types are valid VDL types.
	return nil
}

func typeCheckReservedMethod(method reflect.Method) verror.E {
	switch method.Name {
	case "VGlob":
		// VGlob() *GlobState
		if t := method.Type; t.NumIn() != 1 || t.NumOut() != 1 ||
			t.Out(0) != rtPtrToGlobState {
			return ErrBadVGlob
		}
		return ErrReservedMethod
	case "VGlobChildren":
		// VGlobChildren() ([]string, error)
		if t := method.Type; t.NumIn() != 1 || t.NumOut() != 2 ||
			t.Out(0) != rtSliceOfString || t.Out(1) != rtError {
			return ErrBadVGlobChildren
		}
		return ErrReservedMethod
	}
	return nil
}

func typeCheckStreamingContext(ctx reflect.Type) verror.E {
	// The context must be a pointer to a struct.
	if ctx.Kind() != reflect.Ptr || ctx.Elem().Kind() != reflect.Struct {
		return verror.Abortedf("First in-arg %s is invalid streaming context; must be pointer to a struct representing the typesafe streaming context."+forgotWrap, ctx)
	}
	// Must have Init(ipc.ServerCall) method.
	mInit, hasInit := ctx.MethodByName("Init")
	if !hasInit {
		return verror.Abortedf("First in-arg %s is invalid streaming context; must have Init method."+forgotWrap, ctx)
	}
	if t := mInit.Type; t.NumIn() != 2 || t.In(0).Kind() != reflect.Ptr || t.In(1) != rtServerCall || t.NumOut() != 0 {
		return verror.Abortedf("First in-arg %s is invalid streaming context; Init must have signature func (*) Init(ipc.ServerCall)."+forgotWrap, ctx)
	}
	// Must have either RecvStream or SendStream method.
	mRecvStream, hasRecvStream := ctx.MethodByName("RecvStream")
	mSendStream, hasSendStream := ctx.MethodByName("SendStream")
	if !hasRecvStream && !hasSendStream {
		return verror.Abortedf("First in-arg %s is invalid streaming context; must have at least one of RecvStream or SendStream methods."+forgotWrap, ctx)
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
		if tA, tV, tE := mA.Type, mV.Type, mE.Type; !hasA || !hasV || !hasE ||
			tA.NumIn() != 0 || tA.NumOut() != 1 || tA.Out(0) != rtBool ||
			tV.NumIn() != 0 || tV.NumOut() != 1 || // tV.Out(0) is out-stream type
			tE.NumIn() != 0 || tE.NumOut() != 1 || tE.Out(0) != rtError {
			return errRecvStream(ctx)
		}
	}
	if hasSendStream {
		// func (*) SendStream() interface{ Send(_) error }
		tSend := mSendStream.Type
		if tSend.NumIn() != 1 || tSend.In(0).Kind() != reflect.Ptr ||
			tSend.NumOut() != 1 || tSend.Out(0).Kind() != reflect.Interface {
			return errSendStream(ctx)
		}
		mS, hasS := tSend.Out(0).MethodByName("Send")
		if tS := mS.Type; !hasS ||
			tS.NumIn() != 1 || // tS.In(0) is in-stream type
			tS.NumOut() != 1 || tS.Out(0) != rtError {
			return errSendStream(ctx)
		}
	}
	return nil
}

const (
	useContext = "  Use either ipc.ServerContext for non-streaming methods, or use a non-interface typesafe context for streaming methods."
	forgotWrap = useContext + "  Perhaps you forgot to wrap your server with the VDL-generated server stub."
)

func errRecvStream(rt reflect.Type) verror.E {
	return verror.Abortedf("First in-arg %s is invalid streaming context; RecvStream must have signature func (*) RecvStream() interface{ Advance() bool; Value() _; Err() error }."+forgotWrap, rt)
}

func errSendStream(rt reflect.Type) verror.E {
	return verror.Abortedf("First in-arg %s is invalid streaming context; SendStream must have signature func (*) SendStream() interface{ Send(_) error }."+forgotWrap, rt)
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
	rt := reflect.TypeOf(obj)
	var check map[string]error
	if rt != nil && rt.NumMethod() > 0 {
		check = make(map[string]error, rt.NumMethod())
		for mx := 0; mx < rt.NumMethod(); mx++ {
			method := rt.Method(mx)
			check[method.Name] = typeCheckMethod(method)
		}
	}
	return check
}
