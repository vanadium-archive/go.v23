package verror2_test

import (
	"errors"
	"fmt"
	"runtime"
	"strings"
	"testing"

	"v.io/core/veyron2"
	"v.io/core/veyron2/i18n"
	"v.io/core/veyron2/verror"
	"v.io/core/veyron2/verror2"
	"v.io/core/veyron2/vtrace"

	_ "v.io/core/veyron/profiles"
)

var (
	// Some error IDActions.
	idActionA = verror2.IDAction{"A", verror2.NoRetry}
	idActionB = verror2.IDAction{"B", verror2.RetryBackoff}
	idActionC = verror2.IDAction{"C", verror2.NoRetry}

	// Some languages
	en = i18n.LangID("en")
	fr = i18n.LangID("fr")
	de = i18n.LangID("de")
)

var (
	aEN0 verror2.E
	aEN1 verror2.E
	aFR0 verror2.E
	aFR1 verror2.E
	aDE0 verror2.E
	aDE1 verror2.E

	bEN0 verror2.E
	bEN1 verror2.E
	bFR0 verror2.E
	bFR1 verror2.E
	bDE0 verror2.E
	bDE1 verror2.E

	uEN0 verror2.E
	uEN1 verror2.E
	uFR0 verror2.E
	uFR1 verror2.E
	uDE0 verror2.E
	uDE1 verror2.E

	nEN0 verror2.E
	nEN1 verror2.E
	nFR0 verror2.E
	nFR1 verror2.E
	nDE0 verror2.E
	nDE1 verror2.E

	vEN verror2.E
	vFR verror2.E
	vDE verror2.E

	gEN verror2.E
	gFR verror2.E
	gDE verror2.E

	v2EN  verror2.E
	v2FR0 verror2.E
	v2FR1 verror2.E
	v2DE  verror2.E
)

func init() {
	rootCtx, shutdown := veyron2.Init()
	defer shutdown()

	def := i18n.ContextWithLangID(rootCtx, en)
	def = verror2.ContextWithComponentName(def, "verror2.test")
	verror2.SetDefaultContext(def)

	cat := i18n.Cat()
	// Set messages for English and French.  Do not set messages for
	// German, to test the case where the messages are not present.
	cat.Set(en, i18n.MsgID(idActionA.ID), "{1} {2} error A {_}")
	cat.Set(fr, i18n.MsgID(idActionA.ID), "{1} {2} erreur A {_}")

	cat.Set(en, i18n.MsgID(idActionB.ID), "{1} {2} problem B {_}")
	cat.Set(fr, i18n.MsgID(idActionB.ID), "{1} {2} problème B {_}")

	// Set English and French messages for Unknown and NoExist
	// to ones the test can predict.
	// Delete any German messages that may be present.
	cat.Set(en, i18n.MsgID(verror2.Unknown.ID), "{1} {2} unknown error {_}")
	cat.Set(fr, i18n.MsgID(verror2.Unknown.ID), "{1} {2} erreur inconnu {_}")
	cat.Set(de, i18n.MsgID(verror2.Unknown.ID), "")

	cat.Set(en, i18n.MsgID(verror2.NoExist.ID), "{1} {2} not found {_}")
	cat.Set(fr, i18n.MsgID(verror2.NoExist.ID), "{1} {2} pas trouvé {_}")
	cat.Set(de, i18n.MsgID(verror2.NoExist.ID), "")

	// Set up a context that advertises French, on a server called FooServer,
	// running an operation called aFR0.
	ctx := i18n.ContextWithLangID(rootCtx, fr)
	ctx = verror2.ContextWithComponentName(ctx, "FooServer")
	ctx, _ = vtrace.SetNewSpan(ctx, "aFR1")

	// A first IDAction in various languages.
	aEN0 = verror2.ExplicitMake(idActionA, en, "server", "aEN0", 0)
	aEN1 = verror2.ExplicitMake(idActionA, en, "server", "aEN1", 1, 2)
	aFR0 = verror2.ExplicitMake(idActionA, fr, "server", "aFR0", 0)
	aFR1 = verror2.Make(idActionA, ctx, 1, 2)
	aDE0 = verror2.ExplicitMake(idActionA, de, "server", "aDE0", 0)
	aDE1 = verror2.ExplicitMake(idActionA, de, "server", "aDE1", 1, 2)

	// A second IDAction in various languages.
	bEN0 = verror2.ExplicitMake(idActionB, en, "server", "bEN0", 0)
	bEN1 = verror2.ExplicitMake(idActionB, en, "server", "bEN1", 1, 2)
	bFR0 = verror2.ExplicitMake(idActionB, fr, "server", "bFR0", 0)
	bFR1 = verror2.ExplicitMake(idActionB, fr, "server", "bFR1", 1, 2)
	bDE0 = verror2.ExplicitMake(idActionB, de, "server", "bDE0", 0)
	bDE1 = verror2.ExplicitMake(idActionB, de, "server", "bDE1", 1, 2)

	// The Unknown error in various languages.
	uEN0 = verror2.ExplicitMake(verror2.Unknown, en, "server", "uEN0", 0)
	uEN1 = verror2.ExplicitMake(verror2.Unknown, en, "server", "uEN1", 1, 2)
	uFR0 = verror2.ExplicitMake(verror2.Unknown, fr, "server", "uFR0", 0)
	uFR1 = verror2.ExplicitMake(verror2.Unknown, fr, "server", "uFR1", 1, 2)
	uDE0 = verror2.ExplicitMake(verror2.Unknown, de, "server", "uDE0", 0)
	uDE1 = verror2.ExplicitMake(verror2.Unknown, de, "server", "uDE1", 1, 2)

	// The NoExist error in various languages.
	nEN0 = verror2.ExplicitMake(verror2.NoExist, en, "server", "nEN0", 0)
	nEN1 = verror2.ExplicitMake(verror2.NoExist, en, "server", "nEN1", 1, 2)
	nFR0 = verror2.ExplicitMake(verror2.NoExist, fr, "server", "nFR0", 0)
	nFR1 = verror2.ExplicitMake(verror2.NoExist, fr, "server", "nFR1", 1, 2)
	nDE0 = verror2.ExplicitMake(verror2.NoExist, de, "server", "nDE0", 0)
	nDE1 = verror2.ExplicitMake(verror2.NoExist, de, "server", "nDE1", 1, 2)

	// Errors derived from verror (as opposed to verror2)
	verr := verror.NoExistOrNoAccessf("verror %s", "NoExistOrNoAccess")
	// Set the French for verror.NoExist.
	cat.Set(fr, i18n.MsgID(verror.NoExistOrNoAccess), "{1} {2} n'existe pas ou accès refusé {_}")
	vEN = verror2.ExplicitConvert(verror2.Unknown, en, "server", "op", verr)
	vFR = verror2.ExplicitConvert(verror2.Unknown, fr, "server", "op", verr)
	vDE = verror2.ExplicitConvert(verror2.Unknown, de, "server", "op", verr)

	// Errors derived from Go errors.
	gerr := errors.New("Go error")
	gEN = verror2.ExplicitConvert(verror2.Unknown, en, "server", "op", gerr)
	gFR = verror2.ExplicitConvert(verror2.Unknown, fr, "server", "op", gerr)
	gDE = verror2.ExplicitConvert(verror2.Unknown, de, "server", "op", gerr)

	// Errors derived from other verror2 errors.
	// eEN1 has an English message.
	v2EN = verror2.ExplicitConvert(verror2.Unknown, en, "", "", aEN1)        // still in English.
	v2FR0 = verror2.ExplicitConvert(verror2.Unknown, fr, "", "", aEN1)       // converted to French, with original server and op.
	v2FR1 = verror2.Convert(verror2.Unknown, ctx, aEN1)                      // converted to French, but still with param[1]==aEN1.
	v2DE = verror2.ExplicitConvert(verror2.Unknown, de, "other", "op", aEN1) // left as English, since we lack German.
}

func TestDefaultValues(t *testing.T) {
	if got, want := verror2.ErrorID(nil), verror.ID(""); got != want {
		t.Errorf("got: %d, want: %d", got, want)
	}
	if got, want := verror2.Action(nil), verror2.NoRetry; got != want {
		t.Errorf("got: %d, want: %d", got, want)
	}

	if got, want := verror2.ErrorID(verror2.Standard{}), verror.ID(""); got != want {
		t.Errorf("got: %d, want: %d", got, want)
	}
	if got, want := verror2.Action(verror2.Standard{}), verror2.NoRetry; got != want {
		t.Errorf("got: %d, want: %d", got, want)
	}

	if !verror2.Is(nil, verror2.IDAction{}.ID) {
		t.Errorf("is test failed")
	}
	if verror2.Is(nil, verror2.BadArg.ID) {
		t.Errorf("is test succeeded")
	}
	if verror2.Is(nil, verror2.Unknown.ID) {
		t.Errorf("is test succeeded")
	}

	if verror2.Is(verror2.ExplicitMake(verror2.Unknown, i18n.NoLangID, "", ""), verror2.IDAction{}.ID) {
		t.Errorf("is test succeeded")
	}

	if !verror2.Equal(nil, nil) {
		t.Errorf("equality test failed")
	}

	// nil and default IDAction get mapped to Unknown.
	if !verror2.Equal(nil, verror2.ExplicitMake(verror2.IDAction{}, i18n.NoLangID, "", "")) {
		t.Errorf("equality test failed")
	}
	if verror2.Equal(nil, verror2.ExplicitMake(verror2.Unknown, i18n.NoLangID, "", "")) {
		t.Errorf("equality test succeeded")
	}
	if verror2.Equal(nil, verror2.ExplicitMake(verror2.BadArg, i18n.NoLangID, "", "")) {
		t.Errorf("equality test succeeded")
	}

	unknown := verror2.Standard{}
	if got, want := unknown.Error(), "v.io/core/veyron2/verror.Unknown"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestBasic(t *testing.T) {
	var tests = []struct {
		err      error
		idAction verror2.IDAction
		msg      string
	}{
		{aEN0, idActionA, "server aEN0 error A 0"},
		{aEN1, idActionA, "server aEN1 error A 1 2"},
		{aFR0, idActionA, "server aFR0 erreur A 0"},
		{aFR1, idActionA, "FooServer aFR1 erreur A 1 2"},
		{aDE0, idActionA, "A: server aDE0 0"},
		{aDE1, idActionA, "A: server aDE1 1 2"},

		{bEN0, idActionB, "server bEN0 problem B 0"},
		{bEN1, idActionB, "server bEN1 problem B 1 2"},
		{bFR0, idActionB, "server bFR0 problème B 0"},
		{bFR1, idActionB, "server bFR1 problème B 1 2"},
		{bDE0, idActionB, "B: server bDE0 0"},
		{bDE1, idActionB, "B: server bDE1 1 2"},

		{nEN0, verror2.NoExist, "server nEN0 not found 0"},
		{nEN1, verror2.NoExist, "server nEN1 not found 1 2"},
		{nFR0, verror2.NoExist, "server nFR0 pas trouvé 0"},
		{nFR1, verror2.NoExist, "server nFR1 pas trouvé 1 2"},
		{nDE0, verror2.NoExist, "v.io/core/veyron2/verror.NoExist: server nDE0 0"},
		{nDE1, verror2.NoExist, "v.io/core/veyron2/verror.NoExist: server nDE1 1 2"},

		{vEN, verror2.NoExistOrNoAccess, "server:op: Does not exist or access denied: verror NoExistOrNoAccess"},
		{vFR, verror2.NoExistOrNoAccess, "server op n'existe pas ou accès refusé verror NoExistOrNoAccess"},
		{vDE, verror2.NoExistOrNoAccess, "v.io/core/veyron2/verror.NoExistOrNoAccess: server op verror NoExistOrNoAccess"},

		{gEN, verror2.Unknown, "server op unknown error Go error"},
		{gFR, verror2.Unknown, "server op erreur inconnu Go error"},
		{gDE, verror2.Unknown, "v.io/core/veyron2/verror.Unknown: server op Go error"},

		{v2EN, idActionA, "server aEN1 error A 1 2"},
		{v2FR0, idActionA, "server aEN1 erreur A 1 2"},
		{v2FR1, idActionA, "server aEN1 erreur A 1 2"},
		{v2DE, idActionA, "server aEN1 error A 1 2"},
	}

	for i, test := range tests {
		if verror2.ErrorID(test.err) != test.idAction.ID {
			t.Errorf("%d: ErrorID(%#v); got %v, want %v", i, test.err, verror2.ErrorID(test.err), test.idAction.ID)
		}
		if verror2.Action(test.err) != test.idAction.Action {
			t.Errorf("%d: Action(%#v); got %v, want %v", i, test.err, verror2.Action(test.err), test.idAction.Action)
		}
		if test.err.Error() != test.msg {
			t.Errorf("%d: %#v.Error(); got %q, want %q", i, test.err, test.err.Error(), test.msg)
		}
		if !verror2.Is(test.err, test.idAction.ID) {
			t.Errorf("%d: Is(%#v, %s); got true, want false", i, test.err, test.idAction.ID)
		}
		if verror2.Is(test.err, idActionC.ID) {
			t.Errorf("%d: Is(%#v, %s); got true, want false", i, test.err, idActionC.ID)
		}

		stack := verror2.Stack(test.err)
		if stack == nil {
			t.Errorf("Stack(%q) got nil, want non-nil", verror2.ErrorID(test.err))
		} else if len(stack) < 1 || 2 < len(stack) {
			t.Errorf("len(Stack(%q)) got %d, want 1 or 2", verror2.ErrorID(test.err), len(stack))
		} else {
			fnc := runtime.FuncForPC(stack[0])
			if !strings.Contains(fnc.Name(), "verror2_test.init") {
				t.Errorf("Func.Name(Stack(%q)[0]) got %q, want \"verror2.init\"",
					verror2.ErrorID(test.err), fnc.Name())
			}
		}
	}
}

func tester() (verror2.E, verror2.E) {
	l1 := verror2.ExplicitMake(idActionA, en, "server", "aEN0", 0)
	return l1, verror2.ExplicitMake(idActionA, en, "server", "aEN0", 1)
}

func TestStack(t *testing.T) {
	l1, l2 := tester()
	stack1 := l1.Stack().String()
	stack2 := l2.Stack().String()
	if stack1 == stack2 {
		t.Errorf("expected %q and %q to differ", stack1, stack2)
	}
	for _, stack := range []string{stack1, stack2} {
		if got, want := strings.Count(stack, "\n"), 1; got != want {
			t.Errorf("got %d, want %d", got, want)
		}
		if !strings.Contains(stack, "verror2_test.tester") {
			t.Errorf("got %q, doesn't contain 'verror2_test.tester", stack)
		}
	}
}

func TestEqual(t *testing.T) {
	var equivalanceClasses = [][]verror2.E{
		{aEN0, aEN1, aDE0, aDE1, aDE0, aDE1, v2EN, v2FR0, v2FR1, v2DE},
		{bEN0, bEN1, bDE0, bDE1, bDE0, bDE1},
		{nEN0, nEN1, nDE0, nDE1, nDE0, nDE1},
		{vEN, vFR, vDE},
		{gEN, gFR, gDE},
	}

	for x, class0 := range equivalanceClasses {
		for y, class1 := range equivalanceClasses {
			for _, e0 := range class0 {
				for _, e1 := range class1 {
					if verror2.Equal(e0, e1) != (x == y) {
						t.Errorf("Equal(%#v, %#v) == %v", e0, e1, x == y)
					}
				}
			}
		}
	}
}

func TestSubordinateErrors(t *testing.T) {
	p := verror2.ExplicitMake(idActionA, en, "server", "aEN0", 0)
	if p.SubErrors() != nil {
		t.Errorf("expected nil")
	}
	p1 := verror2.Append(p, aEN1, aFR0)
	r1 := "server aEN0 error A 0 [server aEN1 error A 1 2], [server aFR0 erreur A 0]"
	if got, want := p1.Error(), r1; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	if got, want := len(p1.SubErrors()), 2; got != want {
		t.Errorf("got %d, want %d", got, want)
	}
	p2 := verror2.Append(p, nil, nil, aEN1)
	if got, want := len(p2.SubErrors()), 1; got != want {
		t.Errorf("got %d, want %d", got, want)
	}
	p2 = verror2.Append(p, fmt.Errorf("Oh"))
	if got, want := len(p2.SubErrors()), 1; got != want {
		t.Errorf("got %d, want %d", got, want)
	}
	r2 := "server aEN0 error A 0 [verror2.test  unknown error Oh]"
	if got, want := p2.Error(), r2; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	p2str := p2.DebugString()
	if !strings.Contains(p2str, r2) {
		t.Errorf("debug string missing error message: %q, %q", p2str, r2)
	}
	if !(strings.Contains(p2str, "verror_test.go:330") && strings.Contains(p2str, "verror_test.go:346")) {
		t.Errorf("debug string missing correct line #: %s", p2str)
	}
}

func TestRetryActionFromString(t *testing.T) {
	tests := []struct {
		Label  string
		Action verror2.ActionCode
		ID     verror.ID
	}{
		{"NoRetry", verror2.NoRetry, ""},
		{"RetryConnection", verror2.RetryConnection, ""},
		{"RetryRefetch", verror2.RetryRefetch, ""},
		{"RetryBackoff", verror2.RetryBackoff, ""},
		{"foobar", 0, verror2.BadArg.ID},
	}
	for _, test := range tests {
		action, err := verror2.RetryActionFromString(test.Label)
		if got, want := action, test.Action; got != want {
			t.Errorf("got action %d, want %d", got, want)
		}
		if got, want := verror2.ErrorID(err), test.ID; got != want {
			t.Errorf("got error id %v, want %v", got, want)
		}
	}
}
