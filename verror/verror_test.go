package verror

import (
	"errors"
	"reflect"
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
	expect := Make(unknown, "msg")
	if !reflect.DeepEqual(actual, expect) {
		t.Errorf(`Convert(errors.New("msg")) got %#v, want %#v`, actual, expect)
	}
}

func TestConvertStandard(t *testing.T) {
	// Standard error results in no change.
	stand := standard{ID("A"), "msg"}
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
	if got := convertWithDefault(Internal, nil); got != nil {
		t.Errorf("convertWithDefault(..., nil) = %v", got)
	}
	if got := convertWithDefault(Internal, err); got.ErrorID() != Internal || got.Error() != err.Error() {
		t.Errorf("Got (%v, %v) want (Internal, %v)", got.ErrorID(), got.Error(), err)
	}
	if got := convertWithDefault(Internal, verr); got != verr {
		t.Errorf("Got (%v, %v) want (%v, %v)", got.ErrorID(), got.Error(), verr.ErrorID(), verr.Error())
	}
}
