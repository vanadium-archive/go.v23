package verror2

import (
	"errors"
	"runtime"
	"strings"
	"testing"
	"time"
	"veyron.io/veyron/veyron2/context"
	"veyron.io/veyron/veyron2/i18n"
	"veyron.io/veyron/veyron2/verror"
)

var (
	// Some error IDActions.
	idActionA = IDAction{"A", NoRetry}
	idActionB = IDAction{"B", RetryBackoff}
	idActionC = IDAction{"C", NoRetry}

	// Some languages
	en = i18n.LangID("en")
	fr = i18n.LangID("fr")
	de = i18n.LangID("de")
)

var (
	aEN0 E
	aEN1 E
	aFR0 E
	aFR1 E
	aDE0 E
	aDE1 E

	bEN0 E
	bEN1 E
	bFR0 E
	bFR1 E
	bDE0 E
	bDE1 E

	uEN0 E
	uEN1 E
	uFR0 E
	uFR1 E
	uDE0 E
	uDE1 E

	nEN0 E
	nEN1 E
	nFR0 E
	nFR1 E
	nDE0 E
	nDE1 E

	vEN E
	vFR E
	vDE E

	gEN E
	gFR E
	gDE E

	v2EN  E
	v2FR0 E
	v2FR1 E
	v2DE  E
)

func init() {
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
	cat.Set(en, i18n.MsgID(Unknown.ID), "{1} {2} unknown error {_}")
	cat.Set(fr, i18n.MsgID(Unknown.ID), "{1} {2} erreur inconnu {_}")
	cat.Set(de, i18n.MsgID(Unknown.ID), "")

	cat.Set(en, i18n.MsgID(NoExist.ID), "{1} {2} not found {_}")
	cat.Set(fr, i18n.MsgID(NoExist.ID), "{1} {2} pas trouvé {_}")
	cat.Set(de, i18n.MsgID(NoExist.ID), "")

	// Set up a context that advertises French, on a server called FooServer,
	// running an operation called aFR0.
	ctx := context.T(new(dummyContext))
	ctx = i18n.ContextWithLangID(ctx, fr)
	ctx = ContextWithComponentName(ctx, "FooServer")
	ctx = ContextWithOpName(ctx, "aFR1")

	// A first IDAction in various languages.
	aEN0 = ExplicitMake(idActionA, en, "server", "aEN0", 0)
	aEN1 = ExplicitMake(idActionA, en, "server", "aEN1", 1, 2)
	aFR0 = ExplicitMake(idActionA, fr, "server", "aFR0", 0)
	aFR1 = Make(idActionA, ctx, 1, 2)
	aDE0 = ExplicitMake(idActionA, de, "server", "aDE0", 0)
	aDE1 = ExplicitMake(idActionA, de, "server", "aDE1", 1, 2)

	// A second IDAction in various languages.
	bEN0 = ExplicitMake(idActionB, en, "server", "bEN0", 0)
	bEN1 = ExplicitMake(idActionB, en, "server", "bEN1", 1, 2)
	bFR0 = ExplicitMake(idActionB, fr, "server", "bFR0", 0)
	bFR1 = ExplicitMake(idActionB, fr, "server", "bFR1", 1, 2)
	bDE0 = ExplicitMake(idActionB, de, "server", "bDE0", 0)
	bDE1 = ExplicitMake(idActionB, de, "server", "bDE1", 1, 2)

	// The Unknown error in various languages.
	uEN0 = ExplicitMake(Unknown, en, "server", "uEN0", 0)
	uEN1 = ExplicitMake(Unknown, en, "server", "uEN1", 1, 2)
	uFR0 = ExplicitMake(Unknown, fr, "server", "uFR0", 0)
	uFR1 = ExplicitMake(Unknown, fr, "server", "uFR1", 1, 2)
	uDE0 = ExplicitMake(Unknown, de, "server", "uDE0", 0)
	uDE1 = ExplicitMake(Unknown, de, "server", "uDE1", 1, 2)

	// The NoExist error in various languages.
	nEN0 = ExplicitMake(NoExist, en, "server", "nEN0", 0)
	nEN1 = ExplicitMake(NoExist, en, "server", "nEN1", 1, 2)
	nFR0 = ExplicitMake(NoExist, fr, "server", "nFR0", 0)
	nFR1 = ExplicitMake(NoExist, fr, "server", "nFR1", 1, 2)
	nDE0 = ExplicitMake(NoExist, de, "server", "nDE0", 0)
	nDE1 = ExplicitMake(NoExist, de, "server", "nDE1", 1, 2)

	// Errors derived from verror (as opposed to verror2)
	verr := verror.Existsf("verror %s", "Exists")
	// Set the French for verror.Exists.
	cat.Set(fr, i18n.MsgID(verror.Exists), "{1} {2} déjà en existence {_}")
	vEN = ExplicitConvert(Unknown, en, "server", "op", verr)
	vFR = ExplicitConvert(Unknown, fr, "server", "op", verr)
	vDE = ExplicitConvert(Unknown, de, "server", "op", verr)

	// Errors derived from Go errors.
	gerr := errors.New("Go error")
	gEN = ExplicitConvert(Unknown, en, "server", "op", gerr)
	gFR = ExplicitConvert(Unknown, fr, "server", "op", gerr)
	gDE = ExplicitConvert(Unknown, de, "server", "op", gerr)

	// Errors derived from other verror2 errors.
	// eEN1 has an English message.
	v2EN = ExplicitConvert(Unknown, en, "", "", aEN1)        // still in English.
	v2FR0 = ExplicitConvert(Unknown, fr, "", "", aEN1)       // converted to French, with original server and op.
	v2FR1 = Convert(Unknown, ctx, aEN1)                      // converted to French, with FooServer aFR1
	v2DE = ExplicitConvert(Unknown, de, "other", "op", aEN1) // converted to generic, since we lack German.
}

func TestBasic(t *testing.T) {
	var tests = []struct {
		err      error
		idAction IDAction
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

		{nEN0, NoExist, "server nEN0 not found 0"},
		{nEN1, NoExist, "server nEN1 not found 1 2"},
		{nFR0, NoExist, "server nFR0 pas trouvé 0"},
		{nFR1, NoExist, "server nFR1 pas trouvé 1 2"},
		{nDE0, NoExist, "veyron.io/veyron/veyron2/verror.NoExist: server nDE0 0"},
		{nDE1, NoExist, "veyron.io/veyron/veyron2/verror.NoExist: server nDE1 1 2"},

		{vEN, Exists, "server op Already exists verror Exists"},
		{vFR, Exists, "server op déjà en existence verror Exists"},
		{vDE, Exists, "veyron.io/veyron/veyron2/verror.Exists: server op verror Exists"},

		{gEN, Unknown, "server op unknown error Go error"},
		{gFR, Unknown, "server op erreur inconnu Go error"},
		{gDE, Unknown, "veyron.io/veyron/veyron2/verror.Unknown: server op Go error"},

		{v2EN, idActionA, "server aEN1 error A 1 2"},
		{v2FR0, idActionA, "server aEN1 erreur A 1 2"},
		{v2FR1, idActionA, "FooServer aFR1 erreur A 1 2"},
		{v2DE, idActionA, "A: other op 1 2"},
	}

	for i, test := range tests {
		if ErrorID(test.err) != test.idAction.ID {
			t.Errorf("%d: ErrorID(%#v); got %v, want %v", i, test.err, ErrorID(test.err), test.idAction.ID)
		}
		if Action(test.err) != test.idAction.Action {
			t.Errorf("%d: Action(%#v); got %v, want %v", i, test.err, Action(test.err), test.idAction.Action)
		}
		if test.err.Error() != test.msg {
			t.Errorf("%d: %#v.Error(); got %q, want %q", i, test.err, test.err.Error(), test.msg)
		}
		if !Is(test.err, test.idAction.ID) {
			t.Errorf("%d: Is(%#v, %s); got true, want false", i, test.err, test.idAction.ID)
		}
		if Is(test.err, idActionC.ID) {
			t.Errorf("%d: Is(%#v, %s); got true, want false", i, test.err, idActionC.ID)
		}

		stack := Stack(test.err)
		if stack == nil {
			t.Errorf("Stack(%q) got nil, want non-nil", ErrorID(test.err))
		} else if len(stack) < 1 || 2 < len(stack) {
			t.Errorf("len(Stack(%q)) got %d, want 1 or 2", ErrorID(test.err), len(stack))
		} else {
			fnc := runtime.FuncForPC(stack[0])
			if !strings.Contains(fnc.Name(), "verror2.init") {
				t.Errorf("Func.Name(Stack(%q)[0]) got %q, want \"verror2.init\"",
					ErrorID(test.err), fnc.Name())
			}
		}
	}
}

func TestEqual(t *testing.T) {
	var equivalanceClasses = [][]E{
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
					if Equal(e0, e1) != (x == y) {
						t.Errorf("Equal(%#v, %#v) == %v", e0, e1, x == y)
					}
				}
			}
		}
	}
}

// A dummyContext is a basic implementation of context.T for testing.
type dummyContext struct {
	value map[interface{}]interface{}
}

// Deadline stubs out context.T's call of the same name.
func (dc *dummyContext) Deadline() (deadline time.Time, ok bool) {
	return time.Time{}, false
}

// Done stubs out context.T's call of the same name.
func (dc *dummyContext) Done() (c <-chan struct{}) {
	return c
}

// Err stubs out context.T's call of the same name.
func (dc *dummyContext) Err() error {
	return nil
}

// Runtime stubs out context.T's call of the same name.
func (dc *dummyContext) Runtime() interface{} {
	return nil
}

// Value returns the value corresponding to key in dc's Value map.
// It implements context.T's Value function.
func (dc *dummyContext) Value(key interface{}) interface{} {
	return dc.value[key]
}

// WithCancel stubs out context.T's call of the same name.
func (dc *dummyContext) WithCancel() (ctx context.T, cancel context.CancelFunc) {
	return nil, nil
}

// WithDeadline stubs out context.T's call of the same name.
func (dc *dummyContext) WithDeadline(deadline time.Time) (context.T, context.CancelFunc) {
	return nil, nil
}

// WithTimeout stubs out context.T's call of the same name.
func (dc *dummyContext) WithTimeout(timeout time.Duration) (context.T, context.CancelFunc) {
	return nil, nil
}

// WithValue returns a dummyContext with Value(key)==val.
func (dc *dummyContext) WithValue(key interface{}, val interface{}) context.T {
	newDC := &dummyContext{make(map[interface{}]interface{})}
	for key, value := range dc.value {
		newDC.value[key] = value
	}
	newDC.value[key] = val
	return newDC
}
