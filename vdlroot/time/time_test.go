package time

import (
	"fmt"
	"math/rand"
	"strings"
	"testing"
	gotime "time"
)

func TestTimeToFromNative(t *testing.T) {
	goTime0 := gotime.Time{}
	goTime0 = goTime0.UTC()
	tests := []struct {
		Wire   Time
		Native gotime.Time
	}{
		{Time{}, goTime0},
		// func Date(year int, month Month, day, hour, min, sec, nsec int, loc *Location) Time
		{Time{0, 1}, gotime.Date(1, 1, 1, 0, 0, 0, 1, gotime.UTC)},
		{Time{0, -1}, gotime.Date(1, 1, 1, 0, 0, 0, -1, gotime.UTC)},
		{Time{1, 0}, gotime.Date(1, 1, 1, 0, 0, 1, 0, gotime.UTC)},
		{Time{-1, 0}, gotime.Date(1, 1, 1, 0, 0, -1, 0, gotime.UTC)},
		{Time{1, 1}, gotime.Date(1, 1, 1, 0, 0, 1, 1, gotime.UTC)},
		{Time{1, -1}, gotime.Date(1, 1, 1, 0, 0, 1, -1, gotime.UTC)},
		{Time{-1, 1}, gotime.Date(1, 1, 1, 0, 0, -1, 1, gotime.UTC)},
		// Ensure normalization is as expected
		{Time{0, nanosPerSecond}, gotime.Date(1, 1, 1, 0, 0, 1, 0, gotime.UTC)},
		{Time{0, -nanosPerSecond}, gotime.Date(1, 1, 1, 0, 0, -1, 0, gotime.UTC)},
		{Time{0, nanosPerSecond + 1}, gotime.Date(1, 1, 1, 0, 0, 1, 1, gotime.UTC)},
		{Time{0, -nanosPerSecond - 1}, gotime.Date(1, 1, 1, 0, 0, -1, -1, gotime.UTC)},
		{Time{1, nanosPerSecond + 1}, gotime.Date(1, 1, 1, 0, 0, 2, 1, gotime.UTC)},
		{Time{1, -nanosPerSecond - 1}, gotime.Date(1, 1, 1, 0, 0, 0, -1, gotime.UTC)},
		{Time{-1, nanosPerSecond + 1}, gotime.Date(1, 1, 1, 0, 0, 0, 1, gotime.UTC)},
		{Time{-1, -nanosPerSecond - 1}, gotime.Date(1, 1, 1, 0, 0, -2, -1, gotime.UTC)},
		{Time{0, 2 * nanosPerSecond}, gotime.Date(1, 1, 1, 0, 0, 2, 0, gotime.UTC)},
		{Time{0, -2 * nanosPerSecond}, gotime.Date(1, 1, 1, 0, 0, -2, 0, gotime.UTC)},
		{Time{0, 2*nanosPerSecond + 1}, gotime.Date(1, 1, 1, 0, 0, 2, 1, gotime.UTC)},
		{Time{0, -2*nanosPerSecond - 1}, gotime.Date(1, 1, 1, 0, 0, -2, -1, gotime.UTC)},
	}
	for _, test := range tests {
		native := gotime.Now() // Start with an arbitrary value.
		if err := timeToNative(test.Wire, &native); err != nil {
			t.Errorf("%v timeToNative failed: %v", test.Wire, err)
		}
		if got, want := native, test.Native; got != want {
			t.Errorf("%v timeFromNative got %v, want %v", test.Wire, got, want)
		}
		wire := Now() // Start with an arbitrary value.
		if err := timeFromNative(&wire, test.Native); err != nil {
			t.Errorf("%v timeFromNative failed: %v", test.Wire, err)
		}
		if got, want := wire, test.Wire.Normalize(); got != want {
			t.Errorf("%v timeFromNative got %v, want %v", test.Wire, got, want)
		}
	}
}

func TestDurationToFromNative(t *testing.T) {
	tests := []struct {
		Wire   Duration
		Native gotime.Duration
	}{
		{Duration{}, gotime.Duration(0)},
		{Duration{0, 1}, gotime.Duration(1)},
		{Duration{0, -1}, gotime.Duration(-1)},
		{Duration{1, 0}, gotime.Duration(nanosPerSecond)},
		{Duration{-1, 0}, gotime.Duration(-nanosPerSecond)},
		{Duration{1, 1}, gotime.Duration(nanosPerSecond + 1)},
		{Duration{-1, -1}, gotime.Duration(-(nanosPerSecond + 1))},
		{Duration{1, -1}, gotime.Duration(nanosPerSecond - 1)},
		{Duration{-1, 1}, gotime.Duration(-(nanosPerSecond - 1))},
		{Duration{minGoDurationSec, 0}, gotime.Duration(minGoDurationSec * nanosPerSecond)},
		{Duration{maxGoDurationSec, 0}, gotime.Duration(maxGoDurationSec * nanosPerSecond)},
		// Ensure normalization is as expected
		{Duration{0, nanosPerSecond}, gotime.Duration(nanosPerSecond)},
		{Duration{0, -nanosPerSecond}, gotime.Duration(-nanosPerSecond)},
		{Duration{0, nanosPerSecond + 1}, gotime.Duration(nanosPerSecond + 1)},
		{Duration{0, -nanosPerSecond - 1}, gotime.Duration(-nanosPerSecond - 1)},
		{Duration{1, nanosPerSecond + 1}, gotime.Duration(2*nanosPerSecond + 1)},
		{Duration{1, -nanosPerSecond - 1}, gotime.Duration(-1)},
		{Duration{-1, nanosPerSecond + 1}, gotime.Duration(1)},
		{Duration{-1, -nanosPerSecond - 1}, gotime.Duration(-2*nanosPerSecond - 1)},
		{Duration{0, 2 * nanosPerSecond}, gotime.Duration(2 * nanosPerSecond)},
		{Duration{0, -2 * nanosPerSecond}, gotime.Duration(-2 * nanosPerSecond)},
		{Duration{0, 2*nanosPerSecond + 1}, gotime.Duration(2*nanosPerSecond + 1)},
		{Duration{0, -2*nanosPerSecond - 1}, gotime.Duration(-2*nanosPerSecond - 1)},
	}
	for _, test := range tests {
		native := randGoDuration() // Start with an arbitrary value.
		if err := durationToNative(test.Wire, &native); err != nil {
			t.Errorf("%v durationToNative failed: %v", test.Wire, err)
		}
		if got, want := native, test.Native; got != want {
			t.Errorf("%v durationToNative got %v, want %v", test.Wire, got, want)
		}
		wire := randomDuration() // Start with an arbitrary value.
		if err := durationFromNative(&wire, test.Native); err != nil {
			t.Errorf("%v durationFromNative failed: %v", test.Wire, err)
		}
		if got, want := wire, test.Wire.Normalize(); got != want {
			t.Errorf("%v durationFromNative got %v, want %v", test.Wire, got, want)
		}
	}
}

func randGoDuration() gotime.Duration {
	return gotime.Duration(rand.Int63())
}

func randomDuration() Duration {
	return Duration{rand.Int63(), int32(rand.Intn(nanosPerSecond))}
}

func TestDurationToNativeError(t *testing.T) {
	tests := []struct {
		Wire Duration
		Err  string
	}{
		{Duration{minGoDurationSec, -999999999}, "out of range"},
		{Duration{minGoDurationSec - 1, 0}, "out of range"},
		{Duration{maxGoDurationSec + 1, 0}, "out of range"},
		{Duration{maxGoDurationSec, 999999999}, "out of range"},
	}
	for _, test := range tests {
		native := randGoDuration() // Start with an arbitrary value.
		err := durationToNative(test.Wire, &native)
		if got, want := fmt.Sprint(err), test.Err; !strings.Contains(got, want) {
			t.Errorf("%v durationToNative got error %q, want substr %q", got, want)
		}
		if got, want := native, gotime.Duration(0); got != want {
			t.Errorf("%v durationToNative got %v, want %v", test.Wire, got, want)
		}
	}
}
