package ipc_test

import (
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"testing"
	"time"

	vtest "veyron.io/veyron/veyron/lib/testutil"

	"veyron.io/veyron/veyron2/context"
	"veyron.io/veyron/veyron2/ipc"
	"veyron.io/veyron/veyron2/naming"
	"veyron.io/veyron/veyron2/security"
	"veyron.io/veyron/veyron2/verror"
)

// FakeServerCall implements ipc.ServerContext.
type FakeServerCall struct {
	id security.PublicID
}

func NewFakeServerCall(id security.PublicID) *FakeServerCall {
	return &FakeServerCall{
		id: id,
	}
}

func (*FakeServerCall) Deadline() (deadline time.Time, ok bool) {
	var t time.Time
	return t, false
}
func (*FakeServerCall) Done() <-chan struct{}                                  { return nil }
func (*FakeServerCall) Err() error                                             { return nil }
func (*FakeServerCall) Value(key interface{}) interface{}                      { return nil }
func (*FakeServerCall) Runtime() interface{}                                   { return nil }
func (*FakeServerCall) WithCancel() (ctx context.T, cancel context.CancelFunc) { return nil, nil }
func (*FakeServerCall) WithDeadline(deadline time.Time) (context.T, context.CancelFunc) {
	return nil, nil
}
func (*FakeServerCall) WithTimeout(timeout time.Duration) (context.T, context.CancelFunc) {
	return nil, nil
}
func (*FakeServerCall) WithValue(key interface{}, val interface{}) context.T { return nil }
func (*FakeServerCall) Server() ipc.Server                                   { return nil }
func (*FakeServerCall) Method() string                                       { return "" }
func (*FakeServerCall) Name() string                                         { return "" }
func (*FakeServerCall) Suffix() string                                       { return "" }
func (*FakeServerCall) Label() (l security.Label)                            { return }
func (*FakeServerCall) Discharges() map[string]security.Discharge            { return nil }
func (ctx *FakeServerCall) LocalID() security.PublicID                       { return ctx.id }
func (ctx *FakeServerCall) RemoteID() security.PublicID                      { return ctx.id }
func (*FakeServerCall) Blessing() security.PublicID                          { return nil }
func (*FakeServerCall) LocalEndpoint() naming.Endpoint                       { return nil }
func (*FakeServerCall) RemoteEndpoint() naming.Endpoint                      { return nil }
func (*FakeServerCall) Closed() <-chan struct{}                              { return nil }
func (*FakeServerCall) IsClosed() bool                                       { return false }
func (*FakeServerCall) Send(item interface{}) error                          { return nil }
func (*FakeServerCall) Recv(itemptr interface{}) error                       { return nil }

var testPublicID = security.FakePublicID("test")

var (
	call1 = NewFakeServerCall(testPublicID)
	call2 = NewFakeServerCall(testPublicID)
	call3 = NewFakeServerCall(testPublicID)
	call4 = NewFakeServerCall(testPublicID)
	call5 = NewFakeServerCall(testPublicID)
	call6 = NewFakeServerCall(testPublicID)
)

const defaultLabel = security.AdminLabel

// All objects used for success testing are based on testObj, which captures the
// state from each invocation, so that we may test it against our expectations.
type testObj struct {
	call ipc.ServerCall
}

func (o testObj) LastServerCall() ipc.ServerCall { return o.call }

type testObjIface interface {
	LastServerCall() ipc.ServerCall
}

var errApp = errors.New("app error")

type notags struct{ testObj }

func (o *notags) Method1(c ipc.ServerCall) error               { o.call = c; return nil }
func (o *notags) Method2(c ipc.ServerCall) (int, error)        { o.call = c; return 0, nil }
func (o *notags) Method3(c ipc.ServerCall, _ int) error        { o.call = c; return nil }
func (o *notags) Method4(c ipc.ServerCall, i int) (int, error) { o.call = c; return i, nil }
func (o *notags) Method5(c ipc.ServerCall) int                 { o.call = c; return 1 }
func (o *notags) Error(c ipc.ServerCall) error                 { o.call = c; return errApp }

type tags struct{ testObj }

func (o *tags) Admin(c ipc.ServerCall) error                    { o.call = c; return nil }
func (o *tags) Read(c ipc.ServerCall) (int, error)              { o.call = c; return 0, nil }
func (o *tags) Write(c ipc.ServerCall, _ int) error             { o.call = c; return nil }
func (o *tags) Monitoring(c ipc.ServerCall, i int) (int, error) { o.call = c; return i, nil }
func (o *tags) Debug(c ipc.ServerCall, i int, s string) (int, string, error) {
	o.call = c
	return i, s, nil
}
func (o *tags) Error(c ipc.ServerCall) error { o.call = c; return errApp }

func (o *tags) GetMethodTags(c ipc.ServerCall, method string) ([]interface{}, error) {
	switch method {
	case "Admin":
		return []interface{}{security.AdminLabel}, nil
	case "Read":
		return []interface{}{security.ReadLabel}, nil
	case "Write":
		return []interface{}{security.WriteLabel}, nil
	case "Monitoring":
		return []interface{}{security.MonitoringLabel}, nil
	case "Debug":
		return []interface{}{security.DebugLabel}, nil
	default:
		return []interface{}{}, nil
	}
}

func TestReflectInvoker(t *testing.T) {
	type v []interface{}
	type testcase struct {
		obj    testObjIface
		method string
		call   ipc.ServerCall
		// Expected results:
		label   security.Label
		args    v
		results v
		err     error
	}
	tests := []testcase{
		{&notags{}, "Method1", call1, defaultLabel, nil, v{nil}, nil},
		{&notags{}, "Method2", call2, defaultLabel, nil, v{0, nil}, nil},
		{&notags{}, "Method3", call3, defaultLabel, v{0}, v{nil}, nil},
		{&notags{}, "Method4", call4, defaultLabel, v{11}, v{11, nil}, nil},
		{&notags{}, "Method5", call5, defaultLabel, nil, v{1}, nil},
		{&notags{}, "Error", call6, defaultLabel, nil, v{errApp}, nil},
		{&tags{}, "Admin", call1, security.AdminLabel, nil, v{nil}, nil},
		{&tags{}, "Read", call2, security.ReadLabel, nil, v{0, nil}, nil},
		{&tags{}, "Write", call3, security.WriteLabel, v{0}, v{nil}, nil},
		{&tags{}, "Monitoring", call4, security.MonitoringLabel, v{11}, v{11, nil}, nil},
		{&tags{}, "Debug", call5, security.DebugLabel, v{11, "b"}, v{11, "b", nil}, nil},
		{&tags{}, "Error", call1, defaultLabel, nil, v{errApp}, nil},
	}
	name := func(t testcase) string {
		return fmt.Sprintf("%T.%s()", t.obj, t.method)
	}
	for _, test := range tests {
		invoker := ipc.ReflectInvoker(test.obj)
		// Call Invoker.Prepare and check results.
		argptrs, label, err := invoker.Prepare(test.method, len(test.args))
		if err != nil {
			t.Errorf("%s Prepare unexpected error: %v", name(test), err)
		}
		if !equalPtrValTypes(argptrs, test.args) {
			t.Errorf("%s Prepare got argptrs %v, want args %v", name(test), printTypes(argptrs), printTypes(toPtrs(test.args)))
		}
		if label != test.label {
			t.Errorf("%s Prepare got label %q, want %q", name(test), label, test.label)
		}
		// Call Invoker.Invoke and check results.
		results, err := invoker.Invoke(test.method, test.call, toPtrs(test.args))
		if err != test.err {
			t.Errorf(`%s Invoke got error "%v", want "%v"`, name(test), err, test.err)
		}
		if !reflect.DeepEqual(v(results), test.results) {
			t.Errorf("%s Invoke got results %v, want %v", name(test), results, test.results)
		}
		if call := test.obj.LastServerCall(); call != test.call {
			t.Errorf("%s Invoke got call %v, want %v", name(test), call, test.call)
		}
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

type nocompat struct{}

func (nocompat) notExported(ipc.ServerCall) error  { return nil }
func (nocompat) NumInArgs() error                  { return nil }
func (nocompat) NoInServerCall1(int) error         { return nil }
func (nocompat) NoInServerCall2(int, string) error { return nil }

type nostub struct{}

// NoStub takes ipc.ServerContext rather than ipc.ServerCall as the first arg, a
// common mistake where the server isn't wrapped with the IDL-generated stub.
func (nostub) NoStub(ipc.ServerContext) error { return nil }

func TestReflectInvokerPanic(t *testing.T) {
	type testcase struct {
		obj    interface{}
		regexp string
	}
	tests := []testcase{
		{nil, "no compatible methods"},
		{struct{}{}, "no compatible methods"},
		{nocompat{}, "no compatible methods"},
		{nostub{}, "forgot to wrap your server with the IDL-generated stub"},
	}
	for _, test := range tests {
		got := vtest.CallAndRecover(func() { ipc.ReflectInvoker(test.obj) })
		if !regexp.MustCompile(test.regexp).MatchString(fmt.Sprint(got)) {
			t.Errorf(`ReflectInvoker(%T) got panic "%v", want regexp "%v"`, test.obj, got, test.regexp)
		}
	}
}

func TestReflectInvokerErrors(t *testing.T) {
	type v []interface{}
	type testcase struct {
		obj        interface{}
		method     string
		args       v
		prepareErr error
		invokeErr  error
	}
	expectedError := verror.NotFoundf("ipc: unknown method 'UnknownMethod'")
	tests := []testcase{
		{&notags{}, "UnknownMethod", v{}, expectedError, expectedError},
		{&tags{}, "UnknownMethod", v{}, expectedError, expectedError},
	}
	name := func(t testcase) string {
		return fmt.Sprintf("%T.%s()", t.obj, t.method)
	}
	for _, test := range tests {
		invoker := ipc.ReflectInvoker(test.obj)
		// Call Invoker.Prepare and check error.
		_, _, err := invoker.Prepare(test.method, len(test.args))
		if err != test.prepareErr {
			t.Errorf(`%s Prepare got error "%v", want "%v"`, name(test), err, test.prepareErr)
		}
		// Call Invoker.Invoke and check error.
		_, err = invoker.Invoke(test.method, call1, test.args)
		if err != test.invokeErr {
			t.Errorf(`%s Invoke got error "%v", want "%v"`, name(test), err, test.invokeErr)
		}
	}
}

func TestTypeCheckMethods(t *testing.T) {
	type testcase struct {
		obj    interface{}
		expect map[string]error
	}
	tests := []testcase{
		{struct{}{}, nil},
		{&notags{}, map[string]error{
			"Method1":        nil,
			"Method2":        nil,
			"Method3":        nil,
			"Method4":        nil,
			"Method5":        nil,
			"Error":          nil,
			"LastServerCall": ipc.ErrNumInArgs,
		}},
		{&tags{}, map[string]error{
			"Admin":          nil,
			"Read":           nil,
			"Write":          nil,
			"Monitoring":     nil,
			"Debug":          nil,
			"Error":          nil,
			"GetMethodTags":  nil,
			"LastServerCall": ipc.ErrNumInArgs,
		}},
		{nocompat{}, map[string]error{
			"notExported":     ipc.ErrMethodNotExported,
			"NumInArgs":       ipc.ErrNumInArgs,
			"NoInServerCall1": ipc.ErrInServerCall,
			"NoInServerCall2": ipc.ErrInServerCall,
		}},
	}
	for _, test := range tests {
		actual := ipc.TypeCheckMethods(test.obj)
		if !reflect.DeepEqual(actual, test.expect) {
			t.Errorf("TypeCheckMethods(%T) got %v, want %v", test.obj, actual, test.expect)
		}
	}
}
