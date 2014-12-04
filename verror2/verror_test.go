package verror2_test

import (
	"errors"
	"runtime"
	"strings"
	"testing"

	"veyron.io/veyron/veyron2"
	"veyron.io/veyron/veyron2/i18n"
	"veyron.io/veyron/veyron2/rt"
	"veyron.io/veyron/veyron2/verror"
	"veyron.io/veyron/veyron2/verror2"
	"veyron.io/veyron/veyron2/vtrace"

	_ "veyron.io/veyron/veyron/profiles"
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

var globalRT veyron2.Runtime

func init() {
	var err error
	globalRT, err = rt.New()
	if err != nil {
		panic(err)
	}

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
	ctx := globalRT.NewContext()
	ctx = i18n.ContextWithLangID(ctx, fr)
	ctx = verror2.ContextWithComponentName(ctx, "FooServer")
	ctx, _ = vtrace.WithNewSpan(ctx, "aFR1")

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
		{nDE0, verror2.NoExist, "veyron.io/veyron/veyron2/verror.NoExist: server nDE0 0"},
		{nDE1, verror2.NoExist, "veyron.io/veyron/veyron2/verror.NoExist: server nDE1 1 2"},

		{vEN, verror2.NoExistOrNoAccess, "server:op: Does not exist or access denied: verror NoExistOrNoAccess"},
		{vFR, verror2.NoExistOrNoAccess, "server op n'existe pas ou accès refusé verror NoExistOrNoAccess"},
		{vDE, verror2.NoExistOrNoAccess, "veyron.io/veyron/veyron2/verror.NoExistOrNoAccess: server op verror NoExistOrNoAccess"},

		{gEN, verror2.Unknown, "server op unknown error Go error"},
		{gFR, verror2.Unknown, "server op erreur inconnu Go error"},
		{gDE, verror2.Unknown, "veyron.io/veyron/veyron2/verror.Unknown: server op Go error"},

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
