package ipc

import (
	"fmt"
	"reflect"
	"sync"

	"veyron.io/veyron/veyron2/verror"
)

var (
	errWrongNumInArgs = verror.BadArgf("ipc: wrong number of in-args")
)

type reflectInvoker struct {
	rcvr    reflect.Value
	methods map[string]methodInfo
	gs      *GlobState
}

var _ Invoker = (*reflectInvoker)(nil)

type methodInfo struct {
	funcVal  reflect.Value
	rtInArgs []reflect.Type
	// Tags are special - they are not memoized since they may change per instance.
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
		panic(fmt.Errorf("ipc: object type %T has no compatible methods: %v", obj, TypeCheckMethods(obj)))
	}
	return reflectInvoker{rcvr, methodsCopy, gs}
}

// serverGetMethodTags is a helper interface to check if GetMethodTags is implemented on an object
type serverGetMethodTags interface {
	GetMethodTags(c ServerCall, method string) ([]interface{}, error)
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
	// method is always first, followed by the first method arg which is always
	// the call.  Positional user args follow.
	rvArgs := make([]reflect.Value, len(argptrs)+2)
	rvArgs[0] = ri.rcvr
	rvArgs[1] = reflect.ValueOf(call)
	for ix, argptr := range argptrs {
		rvArgs[ix+2] = reflect.ValueOf(argptr).Elem()
	}
	// Invoke the method, and convert results into interface{}.
	rvResults := info.funcVal.Call(rvArgs)
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
		if err := typeCheckMethod(method); err != nil {
			if verror.Is(err, verror.Aborted) {
				panic(err)
			}
			continue
		}
		// Add a new method entry.
		info := methodInfo{funcVal: method.Func}
		for ix := 2; ix < method.Type.NumIn(); ix++ { // Skip receiver and ServerCall.
			info.rtInArgs = append(info.rtInArgs, method.Type.In(ix))
		}
		methods[method.Name] = info
	}
	return methods
}

var (
	rtServerCall = reflect.TypeOf((*ServerCall)(nil)).Elem()
	rtContext    = reflect.TypeOf((*ServerContext)(nil)).Elem()

	// ReflectInvoker will panic iff the error is Aborted, otherwise it will
	// silently ignore the error.

	ErrMethodNotExported = verror.BadArgf("method not exported")
	ErrNumInArgs         = verror.BadArgf("require at least 1 in-arg (ipc.ServerCall)")
	ErrInContext         = verror.Abortedf("first in-arg got ipc.ServerContext, want ipc.ServerCall - you probably forgot to wrap your server with the IDL-generated stub")
	ErrInServerCall      = verror.BadArgf("first in-arg must be ipc.ServerCall")
)

func typeCheckMethod(method reflect.Method) error {
	// Unexported methods always have a non-empty pkg path.
	if method.PkgPath != "" {
		return ErrMethodNotExported
	}
	mtype := method.Type
	// Method must have at least 2 in args (receiver, ServerCall).
	if in := mtype.NumIn(); in < 2 {
		return ErrNumInArgs
	}
	if in1 := mtype.In(1); in1 != rtServerCall {
		if in1 == rtContext {
			return ErrInContext
		}
		return ErrInServerCall
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
