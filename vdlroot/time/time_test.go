// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package time

import (
	"fmt"
	"math/rand"
	"reflect"
	"strings"
	"testing"
	gotime "time"

	"v.io/v23/vdl"
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

func TestTimeDurationNativeConversion(t *testing.T) {
	tests := []interface{}{
		// Test various times.
		gotime.Time{}.UTC(),
		gotime.Unix(0, 0).UTC(),
		gotime.Unix(0, +1).UTC(),
		gotime.Unix(0, -1).UTC(),
		gotime.Unix(1, 0).UTC(),
		gotime.Unix(1, +1).UTC(),
		gotime.Unix(1, -1).UTC(),
		gotime.Unix(-1, 0).UTC(),
		gotime.Unix(-1, +1).UTC(),
		gotime.Unix(-1, -1).UTC(),
		gotime.Now().UTC(),
		// Test various durations.
		gotime.Duration(0),
		gotime.Duration(+1),
		gotime.Duration(-1),
		gotime.Now().Sub(gotime.Time{}),
		gotime.Now().Sub(gotime.Unix(0, 0)),
	}
	for _, test := range tests {
		// Try converting to a value of the actual native type.
		rv := reflect.New(reflect.TypeOf(test))
		if err := vdl.Convert(rv.Interface(), test); err != nil {
			t.Errorf("%v Convert(%v) failed: %q", test, rv.Elem().Type(), err)
		}
		if got, want := rv.Elem().Interface(), test; !reflect.DeepEqual(got, want) {
			t.Errorf("%v Convert(%v) got %v, want %v", test, rv.Elem().Type(), got, want)
		}
		// Try converting to an empty interface.
		var any interface{}
		if err := vdl.Convert(&any, test); err != nil {
			t.Errorf("%v Convert(interface) failed: %q", test, err)
		}
		if got, want := any, test; !reflect.DeepEqual(got, want) {
			t.Errorf("%v Convert(interface) got %v, want %v", test, got, want)
		}
	}
}

func TestDeadline(t *testing.T) {
	// The deadline wire<->native conversions involve computations against
	// time.Now(), and there's no guarantee that that time.Now() is strictly
	// increasing, or very accurate.  So we define a fudge factor offset for our
	// comparisons.
	const off = 10 * gotime.Second
	tests := []gotime.Duration{
		30 * gotime.Second,
		gotime.Minute,
		gotime.Hour,
		12 * gotime.Hour,
		24 * gotime.Hour,
	}
	for _, test := range tests {
		now := gotime.Now()
		loD, hiD := test-off, test+off
		loT, hiT := now.Add(loD), now.Add(hiD)
		// Test conversion from wire to native.
		var native Deadline
		if err := wireDeadlineToNative(WireDeadline{FromNow: test}, &native); err != nil {
			t.Errorf("%v wireDeadlineToNative failed: %v", test, err)
		}
		if got := native; got.Before(loT) || got.After(hiT) {
			t.Errorf("%v wireDeadlineToNative got %v, want range [%v, %v]", test, got, loT, hiT)
		}
		// Test conversion from native to wire.
		var wire WireDeadline
		if err := wireDeadlineFromNative(&wire, Deadline{now.Add(test)}); err != nil {
			t.Errorf("%v wireDeadlineFromNative failed: %v", test, err)
		}
		if got := wire.FromNow; got < loD || got > hiD {
			t.Errorf("%v wireDeadlineFromNative got %v, want range [%v, %v]", test, got, loD, hiD)
		}
		// Test vdl.Convert from native to native type.
		if err := vdl.Convert(&native, Deadline{now.Add(test)}); err != nil {
			t.Errorf("%v Convert failed: %v", test, err)
		}
		if got := native; got.Before(loT) || got.After(hiT) {
			t.Errorf("%v Convert got %v, want range [%v, %v]", test, got, loT, hiT)
		}
		// Test vdl.Convert from native to interface type.
		var any interface{}
		if err := vdl.Convert(&any, Deadline{now.Add(test)}); err != nil {
			t.Errorf("%v Convert(interface) failed: %v", test, err)
		}
		if got, ok := any.(Deadline); !ok || got.Before(loT) || got.After(hiT) {
			t.Errorf("%v Convert(interface) got %T %v, want range [%v, %v]", test, any, any, loT, hiT)
		}
		// Test vdl.Convert from wire to interface type.  This ensures that the
		// conversion routines are really registered, and automatically applied.
		any = nil
		if err := vdl.Convert(&any, WireDeadline{FromNow: test}); err != nil {
			t.Errorf("%v Convert(interface) failed: %v", test, err)
		}
		if got, ok := any.(Deadline); !ok || got.Before(loT) || got.After(hiT) {
			t.Errorf("%v Convert(interface) got %T %v, want range [%v, %v]", test, any, any, loT, hiT)
		}
	}
}

func TestNoDeadline(t *testing.T) {
	// Make sure conversions between the wire and native formats respect the
	// special sentries for "no deadline".

	// Test conversion from wire to native.
	var native Deadline
	if err := wireDeadlineToNative(WireDeadline{NoDeadline: true}, &native); err != nil {
		t.Errorf("wireDeadlineToNative failed: %v", err)
	}
	if got := native; !got.IsZero() {
		t.Errorf("wireDeadlineToNative got %v, want zero", got)
	}
	// Test conversion from native to wire.
	var wire WireDeadline
	if err := wireDeadlineFromNative(&wire, Deadline{}); err != nil {
		t.Errorf("wireDeadlineFromNative failed: %v", err)
	}
	if !wire.NoDeadline {
		t.Errorf("wireDeadlineFromNative got %v, expected NoDeadline", wire)
	}
	if got, want := wire.FromNow, gotime.Duration(0); got != want {
		t.Errorf("wireDeadlineFromNative got FromNow %v, want %v", got, want)
	}
}
