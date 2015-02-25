package ipc_test

import (
	"errors"
	"fmt"
	"os"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"

	"v.io/core/veyron/lib/testutil"

	"v.io/v23/context"
	"v.io/v23/ipc"
	"v.io/v23/naming"
	"v.io/v23/security"
	"v.io/v23/vdl"
	"v.io/v23/vdlroot/signature"
	"v.io/v23/verror"
)

// TODO(toddw): Add multi-goroutine tests of reflectCache locking.

// We call our own TestMain here because v23 test generate causes an import cycle
// with this package.
func TestMain(m *testing.M) {
	testutil.Init()
	os.Exit(m.Run())
}

// FakeServerCall implements ipc.ServerContext.
type FakeServerCall struct {
	context  *context.T
	security security.Context
}

func testContext() *context.T {
	ctx, _ := context.RootContext()
	return ctx
}

func NewFakeServerCall() *FakeServerCall {
	return &FakeServerCall{
		security: security.NewContext(&security.ContextParams{
			Context: testContext(),
		}),
	}
}

func (*FakeServerCall) Server() ipc.Server             { return nil }
func (*FakeServerCall) Blessings() security.Blessings  { return nil }
func (*FakeServerCall) Closed() <-chan struct{}        { return nil }
func (*FakeServerCall) IsClosed() bool                 { return false }
func (*FakeServerCall) Send(item interface{}) error    { return nil }
func (*FakeServerCall) Recv(itemptr interface{}) error { return nil }
func (call *FakeServerCall) Timestamp() time.Time      { return call.security.Timestamp() }
func (call *FakeServerCall) Method() string            { return call.security.Method() }
func (call *FakeServerCall) MethodTags() []*vdl.Value  { return call.security.MethodTags() }
func (call *FakeServerCall) Suffix() string            { return call.security.Suffix() }
func (call *FakeServerCall) RemoteDischarges() map[string]security.Discharge {
	return call.security.RemoteDischarges()
}
func (call *FakeServerCall) LocalPrincipal() security.Principal {
	return call.security.LocalPrincipal()
}
func (call *FakeServerCall) LocalBlessings() security.Blessings {
	return call.security.LocalBlessings()
}
func (call *FakeServerCall) RemoteBlessings() security.Blessings {
	return call.security.RemoteBlessings()
}
func (call *FakeServerCall) LocalEndpoint() naming.Endpoint  { return call.security.LocalEndpoint() }
func (call *FakeServerCall) RemoteEndpoint() naming.Endpoint { return call.security.RemoteEndpoint() }
func (call *FakeServerCall) Context() *context.T             { return call.security.Context() }

var (
	call1 = NewFakeServerCall()
	call2 = NewFakeServerCall()
	call3 = NewFakeServerCall()
	call4 = NewFakeServerCall()
	call5 = NewFakeServerCall()
	call6 = NewFakeServerCall()
)

// test tags.
var (
	tagAlpha   = vdl.StringValue("alpha")
	tagBeta    = vdl.StringValue("beta")
	tagGamma   = vdl.StringValue("gamma")
	tagDelta   = vdl.StringValue("gamma")
	tagEpsilon = vdl.StringValue("epsilon")
)

// All objects used for success testing are based on testObj, which captures the
// state from each invocation, so that we may test it against our expectations.
type testObj struct {
	ctx ipc.ServerContext
}

func (o testObj) LastContext() ipc.ServerContext { return o.ctx }

type testObjIface interface {
	LastContext() ipc.ServerContext
}

var errApp = errors.New("app error")

type notags struct{ testObj }

func (o *notags) Method1(c ipc.ServerContext) error               { o.ctx = c; return nil }
func (o *notags) Method2(c ipc.ServerContext) (int, error)        { o.ctx = c; return 0, nil }
func (o *notags) Method3(c ipc.ServerContext, _ int) error        { o.ctx = c; return nil }
func (o *notags) Method4(c ipc.ServerContext, i int) (int, error) { o.ctx = c; return i, nil }
func (o *notags) Error(c ipc.ServerContext) error                 { o.ctx = c; return errApp }

type tags struct{ testObj }

func (o *tags) Alpha(c ipc.ServerContext) error               { o.ctx = c; return nil }
func (o *tags) Beta(c ipc.ServerContext) (int, error)         { o.ctx = c; return 0, nil }
func (o *tags) Gamma(c ipc.ServerContext, _ int) error        { o.ctx = c; return nil }
func (o *tags) Delta(c ipc.ServerContext, i int) (int, error) { o.ctx = c; return i, nil }
func (o *tags) Epsilon(c ipc.ServerContext, i int, s string) (int, string, error) {
	o.ctx = c
	return i, s, nil
}
func (o *tags) Error(c ipc.ServerContext) error { o.ctx = c; return errApp }

func (o *tags) Describe__() []ipc.InterfaceDesc {
	return []ipc.InterfaceDesc{{
		Methods: []ipc.MethodDesc{
			{Name: "Alpha", Tags: []*vdl.Value{tagAlpha}},
			{Name: "Beta", Tags: []*vdl.Value{tagBeta}},
			{Name: "Gamma", Tags: []*vdl.Value{tagGamma}},
			{Name: "Delta", Tags: []*vdl.Value{tagDelta}},
			{Name: "Epsilon", Tags: []*vdl.Value{tagEpsilon}},
		},
	}}
}

func TestReflectInvoker(t *testing.T) {
	type v []interface{}
	type testcase struct {
		obj    testObjIface
		method string
		call   ipc.ServerCall
		// Expected results:
		tag     *vdl.Value
		args    v
		results v
		err     error
	}
	tests := []testcase{
		{&notags{}, "Method1", call1, nil, nil, nil, nil},
		{&notags{}, "Method2", call2, nil, nil, v{0}, nil},
		{&notags{}, "Method3", call3, nil, v{0}, nil, nil},
		{&notags{}, "Method4", call4, nil, v{11}, v{11}, nil},
		{&notags{}, "Error", call6, nil, nil, nil, errApp},
		{&tags{}, "Alpha", call1, tagAlpha, nil, nil, nil},
		{&tags{}, "Beta", call2, tagBeta, nil, v{0}, nil},
		{&tags{}, "Gamma", call3, tagGamma, v{0}, nil, nil},
		{&tags{}, "Delta", call4, tagDelta, v{11}, v{11}, nil},
		{&tags{}, "Epsilon", call5, tagEpsilon, v{11, "b"}, v{11, "b"}, nil},
		{&tags{}, "Error", call1, nil, nil, nil, errApp},
	}
	name := func(test testcase) string {
		return fmt.Sprintf("%T.%s()", test.obj, test.method)
	}
	testInvoker := func(test testcase, invoker ipc.Invoker) {
		// Call Invoker.Prepare and check results.
		argptrs, tags, err := invoker.Prepare(test.method, len(test.args))
		if err != nil {
			t.Errorf("%s Prepare unexpected error: %v", name(test), err)
		}
		if !equalPtrValTypes(argptrs, test.args) {
			t.Errorf("%s Prepare got argptrs %v, want args %v", name(test), printTypes(argptrs), printTypes(toPtrs(test.args)))
		}
		var tag *vdl.Value
		if len(tags) > 0 {
			tag = tags[0]
		}
		if tag != test.tag {
			t.Errorf("%s Prepare got tags %v, want %v", name(test), tags, []*vdl.Value{test.tag})
		}
		// Call Invoker.Invoke and check results.
		results, err := invoker.Invoke(test.method, test.call, toPtrs(test.args))
		if err != test.err {
			t.Errorf(`%s Invoke got error "%v", want "%v"`, name(test), err, test.err)
		}
		if !reflect.DeepEqual(v(results), test.results) {
			t.Errorf("%s Invoke got results %v, want %v", name(test), results, test.results)
		}
		if ctx := test.obj.LastContext(); ctx != test.call {
			t.Errorf("%s Invoke got ctx %v, want %v", name(test), ctx, test.call)
		}
	}
	for _, test := range tests {
		invoker := ipc.ReflectInvokerOrDie(test.obj)
		testInvoker(test, invoker)
		invoker, err := ipc.ReflectInvoker(test.obj)
		if err != nil {
			t.Errorf("%s ReflectInvoker got error: %v", name(test), err)
		}
		testInvoker(test, invoker)
	}
}

// equalPtrValTypes returns true iff the types of each value in valptrs is
// identical to the types of the pointers to each value in vals.
func equalPtrValTypes(valptrs, vals []interface{}) bool {
	if len(valptrs) != len(vals) {
		return false
	}
	for ix, val := range vals {
		valptr := valptrs[ix]
		if reflect.TypeOf(valptr) != reflect.PtrTo(reflect.TypeOf(val)) {
			return false
		}
	}
	return true
}

// printTypes returns a string representing the type of each value in vals.
func printTypes(vals []interface{}) string {
	s := "["
	for ix, val := range vals {
		if ix > 0 {
			s += ", "
		}
		s += reflect.TypeOf(val).String()
	}
	return s + "]"
}

// toPtrs takes the given vals and returns a new slice, where each item V at
// index I in vals has been copied into a new pointer value P at index I in the
// result.  The type of P is a pointer to the type of V.
func toPtrs(vals []interface{}) []interface{} {
	valptrs := make([]interface{}, len(vals))
	for ix, val := range vals {
		rvValPtr := reflect.New(reflect.TypeOf(val))
		rvValPtr.Elem().Set(reflect.ValueOf(val))
		valptrs[ix] = rvValPtr.Interface()
	}
	return valptrs
}

type (
	sigTest           struct{}
	stringBoolContext struct{ ipc.ServerContext }
)

func (*stringBoolContext) Init(ipc.ServerCall) {}
func (*stringBoolContext) RecvStream() interface {
	Advance() bool
	Value() string
	Err() error
} {
	return nil
}
func (*stringBoolContext) SendStream() interface {
	Send(item bool) error
} {
	return nil
}

func (sigTest) Sig1(ipc.ServerContext) error                              { return nil }
func (sigTest) Sig2(ipc.ServerContext, int32, string) error               { return nil }
func (sigTest) Sig3(*stringBoolContext, float64) ([]uint32, error)        { return nil, nil }
func (sigTest) Sig4(ipc.ServerCall, int32, string) (int32, string, error) { return 0, "", nil }
func (sigTest) Describe__() []ipc.InterfaceDesc {
	return []ipc.InterfaceDesc{
		{
			Name:    "Iface1",
			PkgPath: "a/b/c",
			Doc:     "Doc Iface1",
			Embeds: []ipc.EmbedDesc{
				{Name: "Iface1Embed1", PkgPath: "x/y", Doc: "Doc embed1"},
			},
			Methods: []ipc.MethodDesc{
				{
					Name:      "Sig3",
					Doc:       "Doc Sig3",
					InArgs:    []ipc.ArgDesc{{Name: "i0_3", Doc: "Doc i0_3"}},
					OutArgs:   []ipc.ArgDesc{{Name: "o0_3", Doc: "Doc o0_3"}},
					InStream:  ipc.ArgDesc{Name: "is_3", Doc: "Doc is_3"},
					OutStream: ipc.ArgDesc{Name: "os_3", Doc: "Doc os_3"},
					Tags:      []*vdl.Value{tagAlpha, tagBeta},
				},
			},
		},
		{
			Name:    "Iface2",
			PkgPath: "d/e/f",
			Doc:     "Doc Iface2",
			Methods: []ipc.MethodDesc{
				{
					// The same Sig3 method is described here in a different interface.
					Name:      "Sig3",
					Doc:       "Doc Sig3x",
					InArgs:    []ipc.ArgDesc{{Name: "i0_3x", Doc: "Doc i0_3x"}},
					OutArgs:   []ipc.ArgDesc{{Name: "o0_3x", Doc: "Doc o0_3x"}},
					InStream:  ipc.ArgDesc{Name: "is_3x", Doc: "Doc is_3x"},
					OutStream: ipc.ArgDesc{Name: "os_3x", Doc: "Doc os_3x"},
					// Must have the same tags as every other definition of this method.
					Tags: []*vdl.Value{tagAlpha, tagBeta},
				},
				{
					Name: "Sig4",
					Doc:  "Doc Sig4",
					InArgs: []ipc.ArgDesc{
						{Name: "i0_4", Doc: "Doc i0_4"}, {Name: "i1_4", Doc: "Doc i1_4"}},
					OutArgs: []ipc.ArgDesc{
						{Name: "o0_4", Doc: "Doc o0_4"}, {Name: "o1_4", Doc: "Doc o1_4"}},
				},
			},
		},
	}
}

func TestReflectInvokerSignature(t *testing.T) {
	tests := []struct {
		Method string // empty to invoke Signature rather than MethodSignature
		Want   interface{}
	}{
		// Tests of MethodSignature.
		{"Sig1", signature.Method{Name: "Sig1"}},
		{"Sig2", signature.Method{
			Name:   "Sig2",
			InArgs: []signature.Arg{{Type: vdl.Int32Type}, {Type: vdl.StringType}},
		}},
		{"Sig3", signature.Method{
			Name: "Sig3",
			Doc:  "Doc Sig3",
			InArgs: []signature.Arg{
				{Name: "i0_3", Doc: "Doc i0_3", Type: vdl.Float64Type}},
			OutArgs: []signature.Arg{
				{Name: "o0_3", Doc: "Doc o0_3", Type: vdl.ListType(vdl.Uint32Type)}},
			InStream: &signature.Arg{
				Name: "is_3", Doc: "Doc is_3", Type: vdl.StringType},
			OutStream: &signature.Arg{
				Name: "os_3", Doc: "Doc os_3", Type: vdl.BoolType},
			Tags: []*vdl.Value{tagAlpha, tagBeta},
		}},
		{"Sig4", signature.Method{
			Name: "Sig4",
			Doc:  "Doc Sig4",
			InArgs: []signature.Arg{
				{Name: "i0_4", Doc: "Doc i0_4", Type: vdl.Int32Type},
				{Name: "i1_4", Doc: "Doc i1_4", Type: vdl.StringType}},
			OutArgs: []signature.Arg{
				{Name: "o0_4", Doc: "Doc o0_4", Type: vdl.Int32Type},
				{Name: "o1_4", Doc: "Doc o1_4", Type: vdl.StringType}},
			// Since ipc.ServerCall is used, we must assume streaming with any.
			InStream:  &signature.Arg{Type: vdl.AnyType},
			OutStream: &signature.Arg{Type: vdl.AnyType},
		}},
		// Test Signature, which always returns the "true" information collected via
		// reflection, and enhances it with user-provided descriptions.
		{"", []signature.Interface{
			{
				Name:    "Iface1",
				PkgPath: "a/b/c",
				Doc:     "Doc Iface1",
				Embeds: []signature.Embed{
					{Name: "Iface1Embed1", PkgPath: "x/y", Doc: "Doc embed1"},
				},
				Methods: []signature.Method{
					{
						Name: "Sig3",
						Doc:  "Doc Sig3",
						InArgs: []signature.Arg{
							{Name: "i0_3", Doc: "Doc i0_3", Type: vdl.Float64Type},
						},
						OutArgs: []signature.Arg{
							{Name: "o0_3", Doc: "Doc o0_3", Type: vdl.ListType(vdl.Uint32Type)},
						},
						InStream: &signature.Arg{
							Name: "is_3", Doc: "Doc is_3", Type: vdl.StringType},
						OutStream: &signature.Arg{
							Name: "os_3", Doc: "Doc os_3", Type: vdl.BoolType},
						Tags: []*vdl.Value{tagAlpha, tagBeta},
					},
				},
			},
			{
				Name:    "Iface2",
				PkgPath: "d/e/f",
				Doc:     "Doc Iface2",
				Methods: []signature.Method{
					{
						Name: "Sig3",
						Doc:  "Doc Sig3x",
						InArgs: []signature.Arg{
							{Name: "i0_3x", Doc: "Doc i0_3x", Type: vdl.Float64Type},
						},
						OutArgs: []signature.Arg{
							{Name: "o0_3x", Doc: "Doc o0_3x", Type: vdl.ListType(vdl.Uint32Type)},
						},
						InStream: &signature.Arg{
							Name: "is_3x", Doc: "Doc is_3x", Type: vdl.StringType},
						OutStream: &signature.Arg{
							Name: "os_3x", Doc: "Doc os_3x", Type: vdl.BoolType},
						Tags: []*vdl.Value{tagAlpha, tagBeta},
					},
					{
						Name: "Sig4",
						Doc:  "Doc Sig4",
						InArgs: []signature.Arg{
							{Name: "i0_4", Doc: "Doc i0_4", Type: vdl.Int32Type},
							{Name: "i1_4", Doc: "Doc i1_4", Type: vdl.StringType},
						},
						OutArgs: []signature.Arg{
							{Name: "o0_4", Doc: "Doc o0_4", Type: vdl.Int32Type},
							{Name: "o1_4", Doc: "Doc o1_4", Type: vdl.StringType},
						},
						InStream:  &signature.Arg{Type: vdl.AnyType},
						OutStream: &signature.Arg{Type: vdl.AnyType},
					},
				},
			},
			{
				Doc: "The empty interface contains methods not attached to any interface.",
				Methods: []signature.Method{
					{Name: "Sig1"},
					{
						Name:   "Sig2",
						InArgs: []signature.Arg{{Type: vdl.Int32Type}, {Type: vdl.StringType}},
					},
				},
			},
		}},
	}
	for _, test := range tests {
		invoker := ipc.ReflectInvokerOrDie(sigTest{})
		var got interface{}
		var err error
		if test.Method == "" {
			got, err = invoker.Signature(nil)
		} else {
			got, err = invoker.MethodSignature(nil, test.Method)
		}
		if err != nil {
			t.Errorf("%q got error %v", test.Method, err)
		}
		if want := test.Want; !reflect.DeepEqual(got, want) {
			t.Errorf("%q got %#v, want %#v", test.Method, got, want)
		}
	}
}

type (
	badcontext        struct{}
	noInitContext     struct{ ipc.ServerContext }
	badInit1Context   struct{ ipc.ServerContext }
	badInit2Context   struct{ ipc.ServerContext }
	badInit3Context   struct{ ipc.ServerContext }
	noSendRecvContext struct{ ipc.ServerContext }
	badSend1Context   struct{ ipc.ServerContext }
	badSend2Context   struct{ ipc.ServerContext }
	badSend3Context   struct{ ipc.ServerContext }
	badRecv1Context   struct{ ipc.ServerContext }
	badRecv2Context   struct{ ipc.ServerContext }
	badRecv3Context   struct{ ipc.ServerContext }

	badoutargs struct{}

	badGlobber       struct{}
	badGlob          struct{}
	badGlobChildren  struct{}
	badGlobChildren2 struct{}
)

func (badcontext) notExported(ipc.ServerContext) error { return nil }
func (badcontext) NonRPC1() error                      { return nil }
func (badcontext) NonRPC2(int) error                   { return nil }
func (badcontext) NonRPC3(int, string) error           { return nil }
func (badcontext) NonRPC4(*badcontext) error           { return nil }
func (badcontext) NoInit(*noInitContext) error         { return nil }
func (badcontext) BadInit1(*badInit1Context) error     { return nil }
func (badcontext) BadInit2(*badInit2Context) error     { return nil }
func (badcontext) BadInit3(*badInit3Context) error     { return nil }
func (badcontext) NoSendRecv(*noSendRecvContext) error { return nil }
func (badcontext) BadSend1(*badSend1Context) error     { return nil }
func (badcontext) BadSend2(*badSend2Context) error     { return nil }
func (badcontext) BadSend3(*badSend3Context) error     { return nil }
func (badcontext) BadRecv1(*badRecv1Context) error     { return nil }
func (badcontext) BadRecv2(*badRecv2Context) error     { return nil }
func (badcontext) BadRecv3(*badRecv3Context) error     { return nil }

func (*badInit1Context) Init()                    {}
func (*badInit2Context) Init(int)                 {}
func (*badInit3Context) Init(ipc.ServerCall, int) {}
func (*noSendRecvContext) Init(ipc.ServerCall)    {}
func (*badSend1Context) Init(ipc.ServerCall)      {}
func (*badSend1Context) SendStream()              {}
func (*badSend2Context) Init(ipc.ServerCall)      {}
func (*badSend2Context) SendStream() interface {
	Send() error
} {
	return nil
}
func (*badSend3Context) Init(ipc.ServerCall) {}
func (*badSend3Context) SendStream() interface {
	Send(int)
} {
	return nil
}
func (*badRecv1Context) Init(ipc.ServerCall) {}
func (*badRecv1Context) RecvStream()         {}
func (*badRecv2Context) Init(ipc.ServerCall) {}
func (*badRecv2Context) RecvStream() interface {
	Advance() bool
	Value() int
} {
	return nil
}
func (*badRecv3Context) Init(ipc.ServerCall) {}
func (*badRecv3Context) RecvStream() interface {
	Advance()
	Value() int
	Error() error
} {
	return nil
}

func (badoutargs) NoFinalError1(ipc.ServerContext)                 {}
func (badoutargs) NoFinalError2(ipc.ServerContext) string          { return "" }
func (badoutargs) NoFinalError3(ipc.ServerContext) (bool, string)  { return false, "" }
func (badoutargs) NoFinalError4(ipc.ServerContext) (error, string) { return nil, "" }

func (badGlobber) Globber()                                     {}
func (badGlob) Glob__()                                         {}
func (badGlobChildren) GlobChildren__(ipc.ServerContext)        {}
func (badGlobChildren2) GlobChildren__() (<-chan string, error) { return nil, nil }

func TestReflectInvokerPanic(t *testing.T) {
	type testcase struct {
		obj    interface{}
		regexp string
	}
	tests := []testcase{
		{nil, `ReflectInvoker\(nil\) is invalid`},
		{struct{}{}, "no compatible methods"},
		{badcontext{}, "invalid streaming context"},
		{badoutargs{}, "final out-arg must be error"},
		{badGlobber{}, "Globber must have signature"},
		{badGlob{}, "Glob__ must have signature"},
		{badGlobChildren{}, "GlobChildren__ must have signature"},
		{badGlobChildren2{}, "GlobChildren__ must have signature"},
	}
	for _, test := range tests {
		re := regexp.MustCompile(test.regexp)
		invoker, err := ipc.ReflectInvoker(test.obj)
		if invoker != nil {
			t.Errorf(`ReflectInvoker(%T) got non-nil invoker`, test.obj)
		}
		if !re.MatchString(fmt.Sprint(err)) {
			t.Errorf(`ReflectInvoker(%T) got error %v, want regexp "%v"`, test.obj, err, test.regexp)
		}
		recov := testutil.CallAndRecover(func() { ipc.ReflectInvokerOrDie(test.obj) })
		if !re.MatchString(fmt.Sprint(recov)) {
			t.Errorf(`ReflectInvokerOrDie(%T) got panic %v, want regexp "%v"`, test.obj, recov, test.regexp)
		}
	}
}

func TestReflectInvokerErrors(t *testing.T) {
	type v []interface{}
	type testcase struct {
		obj        interface{}
		method     string
		args       v
		prepareErr verror.ID
		invokeErr  verror.ID
	}
	tests := []testcase{
		{&notags{}, "UnknownMethod", v{}, verror.ErrNoExist.ID, verror.ErrNoExist.ID},
		{&tags{}, "UnknownMethod", v{}, verror.ErrNoExist.ID, verror.ErrNoExist.ID},
	}
	name := func(test testcase) string {
		return fmt.Sprintf("%T.%s()", test.obj, test.method)
	}
	testInvoker := func(test testcase, invoker ipc.Invoker) {
		// Call Invoker.Prepare and check error.
		_, _, err := invoker.Prepare(test.method, len(test.args))
		if !verror.Is(err, test.prepareErr) {
			t.Errorf(`%s Prepare got error "%v", want id %v`, name(test), err, test.prepareErr)
		}
		// Call Invoker.Invoke and check error.
		_, err = invoker.Invoke(test.method, call1, test.args)
		if !verror.Is(err, test.invokeErr) {
			t.Errorf(`%s Invoke got error "%v", want id %v`, name(test), err, test.invokeErr)
		}
	}
	for _, test := range tests {
		invoker := ipc.ReflectInvokerOrDie(test.obj)
		testInvoker(test, invoker)
		invoker, err := ipc.ReflectInvoker(test.obj)
		if err != nil {
			t.Errorf(`%s ReflectInvoker got error %v`, name(test), err)
		}
		testInvoker(test, invoker)
	}
}

const (
	badInit = `Init must have signature`
	badSend = `SendStream must have signature`
	badRecv = `RecvStream must have signature`
)

func TestTypeCheckMethods(t *testing.T) {
	type testcase struct {
		obj  interface{}
		want map[string]string
	}
	tests := []testcase{
		{struct{}{}, nil},
		{&notags{}, map[string]string{
			"Method1":     "",
			"Method2":     "",
			"Method3":     "",
			"Method4":     "",
			"Error":       "",
			"LastContext": ipc.ErrNonRPCMethod.Error(),
		}},
		{&tags{}, map[string]string{
			"Alpha":       "",
			"Beta":        "",
			"Gamma":       "",
			"Delta":       "",
			"Epsilon":     "",
			"Error":       "",
			"LastContext": ipc.ErrNonRPCMethod.Error(),
			"Describe__":  ipc.ErrReservedMethod.Error(),
		}},
		{badcontext{}, map[string]string{
			"notExported": ipc.ErrMethodNotExported.Error(),
			"NonRPC1":     ipc.ErrNonRPCMethod.Error(),
			"NonRPC2":     ipc.ErrNonRPCMethod.Error(),
			"NonRPC3":     ipc.ErrNonRPCMethod.Error(),
			"NonRPC4":     ipc.ErrNonRPCMethod.Error(),
			"NoInit":      "must have Init method",
			"BadInit1":    badInit,
			"BadInit2":    badInit,
			"BadInit3":    badInit,
			"NoSendRecv":  "must have at least one of RecvStream or SendStream",
			"BadSend1":    badSend,
			"BadSend2":    badSend,
			"BadSend3":    badSend,
			"BadRecv1":    badRecv,
			"BadRecv2":    badRecv,
			"BadRecv3":    badRecv,
		}},
		{badoutargs{}, map[string]string{
			"NoFinalError1": ipc.ErrNoFinalErrorOutArg.Error(),
			"NoFinalError2": ipc.ErrNoFinalErrorOutArg.Error(),
			"NoFinalError3": ipc.ErrNoFinalErrorOutArg.Error(),
			"NoFinalError4": ipc.ErrNoFinalErrorOutArg.Error(),
		}},
		{&badGlobber{}, map[string]string{
			"Globber": ipc.ErrBadGlobber.Error(),
		}},
		{&badGlob{}, map[string]string{
			"Glob__": ipc.ErrBadGlob.Error(),
		}},
		{&badGlobChildren{}, map[string]string{
			"GlobChildren__": ipc.ErrBadGlobChildren.Error(),
		}},
		{&badGlobChildren2{}, map[string]string{
			"GlobChildren__": ipc.ErrBadGlobChildren.Error(),
		}},
	}
	for _, test := range tests {
		typecheck := ipc.TypeCheckMethods(test.obj)
		if got, want := typecheck, test.want; (got == nil) != (want == nil) {
			t.Errorf("TypeCheckMethods(%T) got %v, want %v", test.obj, got, want)
		}
		if got, want := len(typecheck), len(test.want); got != want {
			t.Errorf("TypeCheckMethods(%T) got len %d, want %d (got %q, want %q)", test.obj, got, want, typecheck, test.want)
		}
		for wantKey, wantVal := range test.want {
			gotVal, ok := typecheck[wantKey]
			if !ok {
				t.Errorf("TypeCheckMethods(%T) got %v, want key %q", test.obj, typecheck, wantKey)
			}
			if wantVal == "" && gotVal != nil {
				t.Errorf("TypeCheckMethods(%T) got method %q %q, want %q", test.obj, wantKey, gotVal, wantVal)
			}
			if got, want := fmt.Sprint(gotVal), wantVal; !strings.Contains(got, want) {
				t.Errorf("TypeCheckMethods(%T) got method %q %q, want substr %q", test.obj, wantKey, got, want)
			}
		}
	}
}

type vGlobberObject struct {
	gs *ipc.GlobState
}

func (o *vGlobberObject) Globber() *ipc.GlobState {
	return o.gs
}

type allGlobberObject struct{}

func (allGlobberObject) Glob__(ctx ipc.ServerContext, pattern string) (<-chan naming.VDLGlobReply, error) {
	return nil, nil
}

type childrenGlobberObject struct{}

func (childrenGlobberObject) GlobChildren__(ipc.ServerContext) (<-chan string, error) {
	return nil, nil
}

func TestReflectInvokerGlobber(t *testing.T) {
	allGlobber := allGlobberObject{}
	childrenGlobber := childrenGlobberObject{}
	gs := &ipc.GlobState{AllGlobber: allGlobber}
	vGlobber := &vGlobberObject{gs}

	testcases := []struct {
		obj      interface{}
		expected *ipc.GlobState
	}{
		{vGlobber, gs},
		{allGlobber, &ipc.GlobState{AllGlobber: allGlobber}},
		{childrenGlobber, &ipc.GlobState{ChildrenGlobber: childrenGlobber}},
	}

	for _, tc := range testcases {
		ri := ipc.ReflectInvokerOrDie(tc.obj)
		if got := ri.Globber(); !reflect.DeepEqual(got, tc.expected) {
			t.Errorf("Unexpected result for %#v. Got %#v, want %#v", tc.obj, got, tc.expected)
		}
	}
}
