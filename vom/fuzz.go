// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build gofuzz

package vom

// To use go-fuzz:
//
// $ go get github.com/dvyukov/go-fuzz/go-fuzz{,-build}
// $ cd $JIRI_ROOT/release/go/src/v.io/v23/vom
// $ $JIRI_ROOT/release/go/bin/go-fuzz-build v.io/v23/vom
// $ $JIRI_ROOT/release/go/bin/go-fuzz -bin vom-fuzz.zip -workdir workdir
//
// Inputs resulting in crashes will be in workdir/crashers.
//
// go-fuzz will explore the space of possible input faster if
// you put bigger inputs into workdir/corpus to help it. One way to
// do this is to hack testDecodeVDL to dump parameter "bin" to a
// new file on every call, then run "go test" once. With an empty
// corpus directory, go-fuzz will reverse engineer for itself what
// valid VOM looks like as it explores the input space looking
// for inputs that increase coverage. Magic!

import "bytes"

func Fuzz(data []byte) int {
	var v interface{}
	d := NewDecoder(bytes.NewReader(data))
	if err := d.Decode(&v); err != nil {
		return 0 // failed decode; fuzz is indifferent
	}
	return 1 // successful decode; give fuzz priority
}
