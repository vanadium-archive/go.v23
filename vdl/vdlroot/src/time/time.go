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

// VDLToNative converts from VDL time.Time to Go time.Time.
func (x Time) VDLToNative(native *gotime.Time) error {
	*native = gotime.Unix(x.Seconds-unixEpoch, int64(x.Nano)).UTC()
	return nil
}

// VDLFromNative converts from Go time.Time to VDL time.Time.
func (x *Time) VDLFromNative(native gotime.Time) error {
	x.Seconds = native.Unix() + unixEpoch
	x.Nano = int32(native.Nanosecond())
	*x = x.Normalize()
	return nil
}

// Normalize returns the normalized representation of x.  It makes a best-effort
// attempt to clean up invalid values, e.g. if Nano is outside the valid range,
// or the sign of Nano doesn't match the sign of Seconds.  The behavior is
// undefined for large invalid values, e.g. {int64max,int32max}.
func (x Time) Normalize() Time {
	return Time(Duration(x).Normalize())
}

// VDLToNative converts from VDL time.Duration to Go time.Duration.
func (x Duration) VDLToNative(native *gotime.Duration) error {
	*native = 0
	// Go represents duration as int64 nanoseconds, which has a much smaller range
	// than VDL duration, so we catch these cases and return an error.
	x = x.Normalize()
	if x.Seconds < minGoDurationSec ||
		(x.Seconds == minGoDurationSec && x.Nano < minInt64-minGoDurationSec*nanosPerSecond) ||
		x.Seconds > maxGoDurationSec ||
		(x.Seconds == maxGoDurationSec && x.Nano > maxInt64-maxGoDurationSec*nanosPerSecond) {
		return fmt.Errorf("vdl duration %+v out of range of go duration", x)
	}
	*native = gotime.Duration(x.Seconds*nanosPerSecond + int64(x.Nano))
	return nil
}

// VDLFromNative converts from Go time.Duration to VDL time.Duration.
func (x *Duration) VDLFromNative(g gotime.Duration) error {
	x.Seconds = int64(g / nanosPerSecond)
	x.Nano = int32(g % nanosPerSecond)
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
	t.VDLFromNative(gotime.Now())
	return t
}
