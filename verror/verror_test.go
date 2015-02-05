package verror

import (
	"errors"
	"reflect"
	"regexp"
	"testing"
)

var (
	e1 = errors.New("msg 1")
	e2 = errors.New("msg 2")
)

type special struct{}

func (s *special) ErrorID() ID {
	return ID("special")
}

func (s *special) Error() string {
	return "special msg"
}

func TestConvertNil(t *testing.T) {
	// Nil must remain nil.
	if Convert(nil) != nil {
		t.Errorf("Convert(nil) != nil")
	}
}

func TestConvertRawError(t *testing.T) {
	// Raw error converted to unknown error id.
	actual := Convert(errors.New("msg"))
	expect := Make(Unknown, "msg")
	if !reflect.DeepEqual(actual, expect) {
		t.Errorf(`Convert(errors.New("msg")) got %#v, want %#v`, actual, expect)
	}
}

func TestConvertStandard(t *testing.T) {
	// Standard error results in no change.
	stand := Standard{ID("A"), "msg"}
	actual := Convert(stand)
	if stand != actual {
		t.Errorf(`Convert(%#v) got %#v`, stand, actual)
	}
}

func TestConvertSpecial(t *testing.T) {
	// Special error results in identical pointers (no change).
	spec := &special{}
	actual := Convert(spec)
	if spec != actual {
		t.Errorf(`Convert(special) pointer mismatch`)
	}
}

func TestConvertWithID(t *testing.T) {
	err := errors.New("foo")
	verr := Make(Aborted, "aborted")
	if got := ConvertWithDefault(Internal, nil); got != nil {
		t.Errorf("ConvertWithDefault(..., nil) = %v", got)
	}
	if got := ConvertWithDefault(Internal, err); got.ErrorID() != Internal || got.Error() != err.Error() {
		t.Errorf("Got (%v, %v) want (Internal, %v)", got.ErrorID(), got.Error(), err)
	}
	if got := ConvertWithDefault(Internal, verr); got != verr {
		t.Errorf("Got (%v, %v) want (%v, %v)", got.ErrorID(), got.Error(), verr.ErrorID(), verr.Error())
	}
}

func TestToStandardNil(t *testing.T) {
	// Nil must remain nil.  We use reflect.DeepEqual to ensure ToStandard()
	// doesn't return a typed nil pointer.
	if !reflect.DeepEqual(ToStandard(nil), nil) {
		t.Errorf("ToStandard(nil) != nil")
	}
}

func TestToStandardRawError(t *testing.T) {
	// Raw error converted to unknown error id.
	actual := ToStandard(errors.New("msg"))
	expect := Make(Unknown, "msg")
	if !reflect.DeepEqual(actual, expect) {
		t.Errorf(`ToStandard(errors.New("msg")) got %#v, want %#v`, actual, expect)
	}
}

func TestToStandardStandard(t *testing.T) {
	// Standard error results in no change.
	stand := Standard{ID("A"), "msg"}
	actual := ToStandard(stand)
	if stand != actual {
		t.Errorf(`ToStandard(%#v) got %#v`, stand, actual)
	}
}

func TestToStandardSpecial(t *testing.T) {
	// Special error retains id and msg.
	actual := ToStandard(&special{})
	expect := Make(ID("special"), "special msg")
	if !reflect.DeepEqual(actual, expect) {
		t.Errorf(`ToStandard(&special{})) got %#v, want %#v`, actual, expect)
	}
}

type exp [6]error

func TestTranslator(t *testing.T) {
	e1 := errors.New("1")
	e2 := errors.New("2")
	eA3 := Make(ID("A"), "3")
	eB4 := Make(ID("B"), "4")
	foo5 := Make(ID("C"), "foo5")
	bar6 := Make(ID("D"), "bar6")
	xlate := [6]*Translator{
		NewTranslator(),
		NewTranslator().
			SetErrorRule(e1, ID("Z1"), "PRE1 "),
		NewTranslator().
			SetIDRule(ID("A"), ID("Z2"), "PRE2 "),
		NewTranslator().
			AppendMsgRule(regexp.MustCompile(".*foo.*"), ID("Z3"), "PRE3 "),
		NewTranslator().
			SetDefaultRule(ID("Z4"), "PRE4 "),
		NewTranslator().
			SetErrorRule(e1, ID("Z1"), "PRE1 ").
			SetIDRule(ID("A"), ID("Z2"), "PRE2 ").
			AppendMsgRule(regexp.MustCompile(".*foo.*"), ID("Z3"), "PRE3 ").
			SetDefaultRule(ID("Z4"), "PRE4 "),
	}
	tests := []struct {
		err error
		exp exp
	}{
		{e1, exp{e1, Make(ID("Z1"), "PRE1 1"), e1, e1, Make(ID("Z4"), "PRE4 1"), Make(ID("Z1"), "PRE1 1")}},
		{e2, exp{e2, e2, e2, e2, Make(ID("Z4"), "PRE4 2"), Make(ID("Z4"), "PRE4 2")}},
		{eA3, exp{eA3, eA3, Make(ID("Z2"), "PRE2 3"), eA3, Make(ID("Z4"), "PRE4 3"), Make(ID("Z2"), "PRE2 3")}},
		{eB4, exp{eB4, eB4, eB4, eB4, Make(ID("Z4"), "PRE4 4"), Make(ID("Z4"), "PRE4 4")}},
		{foo5, exp{foo5, foo5, foo5, Make(ID("Z3"), "PRE3 foo5"), Make(ID("Z4"), "PRE4 foo5"), Make(ID("Z3"), "PRE3 foo5")}},
		{bar6, exp{bar6, bar6, bar6, bar6, Make(ID("Z4"), "PRE4 bar6"), Make(ID("Z4"), "PRE4 bar6")}},
	}
	for _, test := range tests {
		for ex, expect := range test.exp {
			actual := xlate[ex].Translate(test.err)
			if !reflect.DeepEqual(actual, expect) {
				t.Errorf(`xlate[%d].Translate(%#v) got %#v, want %#v`, ex, test.err, actual, expect)
			}
		}
	}
}
