// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build fuzzdump

package vom

import (
	"crypto/sha1"
	"fmt"
	"io/ioutil"
	"os"
	"testing"

	"v.io/v23/vom/testdata/data80"
	"v.io/v23/vom/testdata/data81"
)

func TestFuzzDump(t *testing.T) {
	if err := os.MkdirAll("workdir/corpus", 0755); err != nil {
		t.Fatal(err)
	}

	seen := make(map[[sha1.Size]byte]bool)

	tests := append(data80.Tests, data81.Tests...)

	for _, test := range tests {
		binversion, err := binFromHexPat(test.HexVersion)
		if err != nil {
			t.Fatal(err)
		}

		bintype, err := binFromHexPat(test.HexType)
		if err != nil {
			t.Fatal(err)
		}

		binvalue, err := binFromHexPat(test.HexValue)
		if err != nil {
			t.Fatal(err)
		}

		data := []byte(binversion + bintype + binvalue)
		hash := sha1.Sum(data)

		if seen[hash] {
			continue
		}
		seen[hash] = true

		fn := fmt.Sprintf("workdir/corpus/%x", hash)
		if err := ioutil.WriteFile(fn, data, 0644); err != nil {
			t.Fatal(err)
		}
	}
}
