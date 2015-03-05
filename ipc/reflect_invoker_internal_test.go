// reflect_invoker_internal_test uses internals of reflect_invoker (and hence is in the ipc package),
// whereas reflect_invoker_test is it the ipc_test package.

package ipc

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"v.io/v23/vdl"
	"v.io/v23/verror"
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
	call ServerCall
}

func (o testObj) LastCall() ServerCall { return o.call }

var errApp = errors.New("app error")

type notags struct{ testObj }

func (o *notags) Method1(c ServerCall) error               { o.call = c; return nil }
func (o *notags) Method2(c ServerCall) (int, error)        { o.call = c; return 0, nil }
func (o *notags) Method3(c ServerCall, _ int) error        { o.call = c; return nil }
func (o *notags) Method4(c ServerCall, i int) (int, error) { o.call = c; return i, nil }
func (o *notags) Error(c ServerCall) error                 { o.call = c; return errApp }

type tags struct{ testObj }

func (o *tags) Alpha(c ServerCall) error               { o.call = c; return nil }
func (o *tags) Beta(c ServerCall) (int, error)         { o.call = c; return 0, nil }
func (o *tags) Gamma(c ServerCall, _ int) error        { o.call = c; return nil }
func (o *tags) Delta(c ServerCall, i int) (int, error) { o.call = c; return i, nil }
func (o *tags) Epsilon(c ServerCall, i int, s string) (int, string, error) {
	o.call = c
	return i, s, nil
}
func (o *tags) Error(c ServerCall) error { o.call = c; return errApp }

func (o *tags) Describe__() []InterfaceDesc {
	return []InterfaceDesc{{
		Methods: []MethodDesc{
			{Name: "Alpha", Tags: []*vdl.Value{tagAlpha}},
			{Name: "Beta", Tags: []*vdl.Value{tagBeta}},
			{Name: "Gamma", Tags: []*vdl.Value{tagGamma}},
			{Name: "Delta", Tags: []*vdl.Value{tagDelta}},
			{Name: "Epsilon", Tags: []*vdl.Value{tagEpsilon}},
		},
	}}
}

type (
	badcall        struct{}
	noInitCall     struct{ ServerCall }
	badInit1Call   struct{ ServerCall }
	badInit2Call   struct{ ServerCall }
	badInit3Call   struct{ ServerCall }
	noSendRecvCall struct{ ServerCall }
	badSend1Call   struct{ ServerCall }
	badSend2Call   struct{ ServerCall }
	badSend3Call   struct{ ServerCall }
	badRecv1Call   struct{ ServerCall }
	badRecv2Call   struct{ ServerCall }
	badRecv3Call   struct{ ServerCall }

	badoutargs struct{}

	badGlobber       struct{}
	badGlob          struct{}
	badGlobChildren  struct{}
	badGlobChildren2 struct{}
)

func (badcall) notExported(ServerCall) error     { return nil }
func (badcall) NonRPC1() error                   { return nil }
func (badcall) NonRPC2(int) error                { return nil }
func (badcall) NonRPC3(int, string) error        { return nil }
func (badcall) NonRPC4(*badcall) error           { return nil }
func (badcall) NoInit(*noInitCall) error         { return nil }
func (badcall) BadInit1(*badInit1Call) error     { return nil }
func (badcall) BadInit2(*badInit2Call) error     { return nil }
func (badcall) BadInit3(*badInit3Call) error     { return nil }
func (badcall) NoSendRecv(*noSendRecvCall) error { return nil }
func (badcall) BadSend1(*badSend1Call) error     { return nil }
func (badcall) BadSend2(*badSend2Call) error     { return nil }
func (badcall) BadSend3(*badSend3Call) error     { return nil }
func (badcall) BadRecv1(*badRecv1Call) error     { return nil }
func (badcall) BadRecv2(*badRecv2Call) error     { return nil }
func (badcall) BadRecv3(*badRecv3Call) error     { return nil }

func (*badInit1Call) Init()                      {}
func (*badInit2Call) Init(int)                   {}
func (*badInit3Call) Init(StreamServerCall, int) {}
func (*noSendRecvCall) Init(StreamServerCall)    {}
func (*badSend1Call) Init(StreamServerCall)      {}
func (*badSend1Call) SendStream()                {}
func (*badSend2Call) Init(StreamServerCall)      {}
func (*badSend2Call) SendStream() interface {
	Send() error
} {
	return nil
}
func (*badSend3Call) Init(StreamServerCall) {}
func (*badSend3Call) SendStream() interface {
	Send(int)
} {
	return nil
}
func (*badRecv1Call) Init(StreamServerCall) {}
func (*badRecv1Call) RecvStream()           {}
func (*badRecv2Call) Init(StreamServerCall) {}
func (*badRecv2Call) RecvStream() interface {
	Advance() bool
	Value() int
} {
	return nil
}
func (*badRecv3Call) Init(StreamServerCall) {}
func (*badRecv3Call) RecvStream() interface {
	Advance()
	Value() int
	Error() error
} {
	return nil
}

func (badoutargs) NoFinalError1(ServerCall)                 {}
func (badoutargs) NoFinalError2(ServerCall) string          { return "" }
func (badoutargs) NoFinalError3(ServerCall) (bool, string)  { return false, "" }
func (badoutargs) NoFinalError4(ServerCall) (error, string) { return nil, "" }

func (badGlobber) Globber()                                     {}
func (badGlob) Glob__()                                         {}
func (badGlobChildren) GlobChildren__(ServerCall)               {}
func (badGlobChildren2) GlobChildren__() (<-chan string, error) { return nil, nil }

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
			"Method1":  "",
			"Method2":  "",
			"Method3":  "",
			"Method4":  "",
			"Error":    "",
			"LastCall": verror.New(verror.ErrBadArg, nil, verror.New(errNonRPCMethod, nil)).Error(),
		}},
		{&tags{}, map[string]string{
			"Alpha":      "",
			"Beta":       "",
			"Gamma":      "",
			"Delta":      "",
			"Epsilon":    "",
			"Error":      "",
			"LastCall":   verror.New(verror.ErrBadArg, nil, verror.New(errNonRPCMethod, nil)).Error(),
			"Describe__": verror.New(verror.ErrInternal, nil, verror.New(errReservedMethod, nil)).Error(),
		}},
		{badcall{}, map[string]string{
			"notExported": verror.New(verror.ErrBadArg, nil, verror.New(errMethodNotExported, nil)).Error(),
			"NonRPC1":     verror.New(errNonRPCMethod, nil).Error(),
			"NonRPC2":     verror.New(errNonRPCMethod, nil).Error(),
			"NonRPC3":     verror.New(errNonRPCMethod, nil).Error(),
			"NonRPC4":     verror.New(errNonRPCMethod, nil).Error(),
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
			"NoFinalError1": verror.New(verror.ErrAborted, nil, verror.New(errNoFinalErrorOutArg, nil)).Error(),
			"NoFinalError2": verror.New(verror.ErrAborted, nil, verror.New(errNoFinalErrorOutArg, nil)).Error(),
			"NoFinalError3": verror.New(verror.ErrAborted, nil, verror.New(errNoFinalErrorOutArg, nil)).Error(),
			"NoFinalError4": verror.New(verror.ErrAborted, nil, verror.New(errNoFinalErrorOutArg, nil)).Error(),
		}},
		{&badGlobber{}, map[string]string{
			"Globber": verror.New(verror.ErrAborted, nil, verror.New(errBadGlobber, nil)).Error(),
		}},
		{&badGlob{}, map[string]string{
			"Glob__": verror.New(verror.ErrAborted, nil, verror.New(errBadGlob, nil)).Error(),
		}},
		{&badGlobChildren{}, map[string]string{
			"GlobChildren__": verror.New(verror.ErrAborted, nil, verror.New(errBadGlobChildren, nil)).Error(),
		}},
		{&badGlobChildren2{}, map[string]string{
			"GlobChildren__": verror.New(verror.ErrAborted, nil, verror.New(errBadGlobChildren, nil)).Error(),
		}},
	}
	for _, test := range tests {
		typecheck := TypeCheckMethods(test.obj)
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
