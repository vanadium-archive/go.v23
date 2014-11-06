package ipc

import (
	"reflect"
	"testing"

	"veyron.io/veyron/veyron/lib/testutil"
)

func init() { testutil.Init() }

type vGlobberObject struct {
	gs *GlobState
}

func (o *vGlobberObject) VGlob() *GlobState {
	return o.gs
}

type vAllGlobberObject struct{}

func (vAllGlobberObject) Glob(call ServerCall, pattern string) error {
	return nil
}

type vChildrenGlobberObject struct{}

func (vChildrenGlobberObject) VGlobChildren() ([]string, error) {
	return nil, nil
}

func TestReflectInvokerVGlob(t *testing.T) {
	vAllGlobber := vAllGlobberObject{}
	vChildrenGlobber := vChildrenGlobberObject{}
	gs := &GlobState{VAllGlobber: vAllGlobber}
	vGlobber := &vGlobberObject{gs}

	testcases := []struct {
		obj      interface{}
		expected *GlobState
	}{
		{vGlobber, gs},
		{vAllGlobber, &GlobState{VAllGlobber: vAllGlobber}},
		{vChildrenGlobber, &GlobState{VChildrenGlobber: vChildrenGlobber}},
	}

	for _, tc := range testcases {
		// TODO(rthellend): Move this whole test to reflect_invoker_test.go
		// after the VGlobber interface is added to Invoker.
		ri := ReflectInvoker(tc.obj).(reflectInvoker)
		if got := ri.VGlob(); !reflect.DeepEqual(got, tc.expected) {
			t.Errorf("Unexpected result for %#v. Got %#v, want %#v", tc.obj, got, tc.expected)
		}
	}
}
