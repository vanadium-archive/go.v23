package val

import (
	"fmt"
	"strings"
	"testing"

	testutil "veyron/lib/testutil"
)

func expectErr(t *testing.T, err error, wantstr string, format string, args ...interface{}) {
	gotstr := fmt.Sprint(err)
	msg := fmt.Sprintf(format, args...)
	if wantstr != "" && !strings.Contains(gotstr, wantstr) {
		t.Errorf(`%s got error %q, want substr %q`, msg, gotstr, wantstr)
	}
	if wantstr == "" && err != nil {
		t.Errorf(`%s got error %q, want nil`, msg, gotstr)
	}
}

func expectPanic(t *testing.T, f func(), wantstr string, format string, args ...interface{}) {
	got := testutil.CallAndRecover(f)
	gotstr := fmt.Sprint(got)
	msg := fmt.Sprintf(format, args...)
	if wantstr != "" && !strings.Contains(gotstr, wantstr) {
		t.Errorf(`%s got panic %q, want substr %q`, msg, gotstr, wantstr)
	}
	if wantstr == "" && got != nil {
		t.Errorf(`%s got panic %q, want nil`, msg, gotstr)
	}
}

func expectMismatchedKind(t *testing.T, f func()) {
	expectPanic(t, f, "mismatched kind", "")
}
