package verror2

import (
	"errors"
	"testing"
	"veyron2/i18n"
	"veyron2/verror"
)

var (
	// Some error ids.
	idA      = ID{"A", Failed}
	idB      = ID{"B", Backoff}
	idC      = ID{"C", Failed}
	idAPrime = ID{"A", Backoff}

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
)

func init() {
	cat := i18n.Cat()
	// Set messages for English and French.  Do not set messages for
	// German, to test the case where the messages are not present.
	cat.Set(en, idA.MsgID, "{1}: error A: {_}")
	cat.Set(fr, idA.MsgID, "{1}: erreur A: {_}")

	cat.Set(en, idB.MsgID, "{1}: problem B: {_}")
	cat.Set(fr, idB.MsgID, "{1}: problème B: {_}")

	// Set English and French messages for Unknown and NotFound
	// to ones the test can predict.
	// Delete any German messages that may be present.
	cat.Set(en, Unknown.MsgID, "{1}: unknown error: {_}")
	cat.Set(fr, Unknown.MsgID, "{1}: erreur inconnu: {_}")
	cat.Set(de, Unknown.MsgID, "")

	cat.Set(en, NotFound.MsgID, "{1}: not found: {_}")
	cat.Set(fr, NotFound.MsgID, "{1}: pas trouvé: {_}")
	cat.Set(de, NotFound.MsgID, "")

	// A first error ID.
	aEN0 = Make(idA, en, "aEN0", 0)
	aEN1 = Make(idA, en, "aEN1", 1, 2)
	aFR0 = Make(idA, fr, "aFR0", 0)
	aFR1 = Make(idA, fr, "aFR1", 1, 2)
	aDE0 = Make(idA, de, "aDE0", 0)
	aDE1 = Make(idA, de, "aDE1", 1, 2)

	// A second error ID.
	bEN0 = Make(idB, en, "bEN0", 0)
	bEN1 = Make(idB, en, "bEN1", 1, 2)
	bFR0 = Make(idB, fr, "bFR0", 0)
	bFR1 = Make(idB, fr, "bFR1", 1, 2)
	bDE0 = Make(idB, de, "bDE0", 0)
	bDE1 = Make(idB, de, "bDE1", 1, 2)

	// The Unknown error ID.
	uEN0 = Make(Unknown, en, "uEN0", 0)
	uEN1 = Make(Unknown, en, "uEN1", 1, 2)
	uFR0 = Make(Unknown, fr, "uFR0", 0)
	uFR1 = Make(Unknown, fr, "uFR1", 1, 2)
	uDE0 = Make(Unknown, de, "uDE0", 0)
	uDE1 = Make(Unknown, de, "uDE1", 1, 2)

	// The NotFound error ID.
	nEN0 = Make(NotFound, en, "nEN0", 0)
	nEN1 = Make(NotFound, en, "nEN1", 1, 2)
	nFR0 = Make(NotFound, fr, "nFR0", 0)
	nFR1 = Make(NotFound, fr, "nFR1", 1, 2)
	nDE0 = Make(NotFound, de, "nDE0", 0)
	nDE1 = Make(NotFound, de, "nDE1", 1, 2)

	// Errors derived from verror (as opposed to verror2)
	verr := verror.Existsf("verror %s", "Exists")
	vEN = Convert(Unknown, en, verr)
	vFR = Convert(Unknown, fr, verr)
	vDE = Convert(Unknown, de, verr)

	// Errors derived from Go errors.
	gerr := errors.New("Go error")
	gEN = Convert(Unknown, en, gerr)
	gFR = Convert(Unknown, fr, gerr)
	gDE = Convert(Unknown, de, gerr)
}

func TestBasic(t *testing.T) {
	var tests = []struct {
		err error
		id  ID
		msg string
	}{
		{aEN0, idA, "aEN0: error A: 0"},
		{aEN1, idA, "aEN1: error A: 1 2"},
		{aFR0, idA, "aFR0: erreur A: 0"},
		{aFR1, idA, "aFR1: erreur A: 1 2"},
		{aDE0, idA, "A: aDE0 0"},
		{aDE1, idA, "A: aDE1 1 2"},

		{bEN0, idB, "bEN0: problem B: 0"},
		{bEN1, idB, "bEN1: problem B: 1 2"},
		{bFR0, idB, "bFR0: problème B: 0"},
		{bFR1, idB, "bFR1: problème B: 1 2"},
		{bDE0, idB, "B: bDE0 0"},
		{bDE1, idB, "B: bDE1 1 2"},

		{nEN0, NotFound, "nEN0: not found: 0"},
		{nEN1, NotFound, "nEN1: not found: 1 2"},
		{nFR0, NotFound, "nFR0: pas trouvé: 0"},
		{nFR1, NotFound, "nFR1: pas trouvé: 1 2"},
		{nDE0, NotFound, "veyron2/verror.NotFound: nDE0 0"},
		{nDE1, NotFound, "veyron2/verror.NotFound: nDE1 1 2"},

		{vEN, Exists, "verror Exists"},
		{vFR, Exists, "verror Exists"},
		{vDE, Exists, "verror Exists"},

		{gEN, Unknown, "Error: unknown error: Go error"},
		{gFR, Unknown, "Error: erreur inconnu: Go error"},
		{gDE, Unknown, "veyron2/verror.Unknown: Error Go error"},
	}

	for _, test := range tests {
		if ErrorID(test.err) != test.id {
			t.Errorf("ErrorID(%#v); got %v, want %v", test.err, ErrorID(test.err), test.id)
		}
		if test.err.Error() != test.msg {
			t.Errorf("%#v.Error(); got %q, want %q", test.err, test.err.Error(), test.msg)
		}
		if !Is(test.err, test.id) {
			t.Errorf("Is(%#v, %s); got true, want false", test.err, test.id)
		}
		if Is(test.err, idC) {
			t.Errorf("Is(%#v, %s); got true, want false", test.err, idC)
		}
		if Is(test.err, idAPrime) {
			t.Errorf("Is(%#v, %s); got true, want false", test.err, idAPrime)
		}
	}
}

func TestEqual(t *testing.T) {
	var equivalanceClasses = [][]E{
		{aEN0, aEN1, aDE0, aDE1, aDE0, aDE1},
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
