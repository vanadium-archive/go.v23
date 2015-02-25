package time

import (
	"fmt"
	gotime "time"
)

const (
	nanosPerSecond = 1000 * 1000 * 1000
	secondsPerDay  = 24 * 60 * 60

	// Represent the unix epoch 1970-01-01 in terms of our epoch 0001-01-01, for
	// easy conversions.  Note that we use a proleptic Gregorian calendar; there
	// is a leap year every 4 years, except for years divisible by 100, but
	// including years divisible by 400.
	unixEpoch = (1969*365 + 1969/4 - 1969/100 + 1969/400) * secondsPerDay

	minInt64         = -(1 << 63)
	maxInt64         = ((1 << 63) - 1)
	minGoDurationSec = minInt64 / nanosPerSecond
	maxGoDurationSec = maxInt64 / nanosPerSecond
)

// timeToNative converts from VDL time.Time to Go time.Time.
func timeToNative(wire Time, native *gotime.Time) error {
	*native = gotime.Unix(wire.Seconds-unixEpoch, int64(wire.Nano)).UTC()
	return nil
}

// timeFromNative converts from Go time.Time to VDL time.Time.
func timeFromNative(wire *Time, native gotime.Time) error {
	wire.Seconds = native.Unix() + unixEpoch
	wire.Nano = int32(native.Nanosecond())
	*wire = wire.Normalize()
	return nil
}

// Normalize returns the normalized representation of x.  It makes a best-effort
// attempt to clean up invalid values, e.g. if Nano is outside the valid range,
// or the sign of Nano doesn't match the sign of Seconds.  The behavior is
// undefined for large invalid values, e.g. {int64max,int32max}.
func (x Time) Normalize() Time {
	return Time(Duration(x).Normalize())
}

// durationToNative converts from VDL time.Duration to Go time.Duration.
func durationToNative(wire Duration, native *gotime.Duration) error {
	*native = 0
	// Go represents duration as int64 nanoseconds, which has a much smaller range
	// than VDL duration, so we catch these cases and return an error.
	wire = wire.Normalize()
	if wire.Seconds < minGoDurationSec ||
		(wire.Seconds == minGoDurationSec && wire.Nano < minInt64-minGoDurationSec*nanosPerSecond) ||
		wire.Seconds > maxGoDurationSec ||
		(wire.Seconds == maxGoDurationSec && wire.Nano > maxInt64-maxGoDurationSec*nanosPerSecond) {
		return fmt.Errorf("vdl duration %+v out of range of go duration", wire)
	}
	*native = gotime.Duration(wire.Seconds*nanosPerSecond + int64(wire.Nano))
	return nil
}

// durationFromNative converts from Go time.Duration to VDL time.Duration.
func durationFromNative(wire *Duration, native gotime.Duration) error {
	wire.Seconds = int64(native / nanosPerSecond)
	wire.Nano = int32(native % nanosPerSecond)
	return nil
}

// Normalize returns the normalized representation of x.  It makes a best-effort
// attempt to clean up invalid values, e.g. if Nano is outside the valid range,
// or the sign of Nano doesn't match the sign of Seconds.  The behavior is
// undefined for large invalid values, e.g. {int64max,int32max}.
func (x Duration) Normalize() Duration {
	x.Seconds += int64(x.Nano / nanosPerSecond)
	x.Nano = x.Nano % nanosPerSecond
	switch {
	case x.Seconds < 0 && x.Nano > 0:
		x.Seconds += 1
		x.Nano -= nanosPerSecond
	case x.Seconds > 0 && x.Nano < 0:
		x.Seconds -= 1
		x.Nano += nanosPerSecond
	}
	return x
}

// Now returns the current time.
func Now() Time {
	var t Time
	timeFromNative(&t, gotime.Now())
	return t
}
