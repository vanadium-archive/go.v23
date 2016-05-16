// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Command vomforever is a tool that searches for bugs in vom.
// It vom encodes and decodes in a loop and reports the errors that are found.
package main

import (
	"fmt"
	"math/rand"
	"os"

	"flag"
	"v.io/v23/vdl"
	"v.io/v23/vdl/vdltest"
	"v.io/v23/vom"
)

const (
	maxDepth = 4

	numValuesPerTypeList = 100
)

var verbose = flag.Bool("v", false, "verbose output")

func genValues() chan *vdl.Value {
	typegen := vdltest.NewTypeGenerator()
	out := make(chan *vdl.Value, 1)
	go func() {
		for {
			types := typegen.Gen(maxDepth)
			valgen := vdltest.NewValueGenerator(types)
			modes := []vdltest.GenMode{vdltest.GenFull, vdltest.GenPosMax, vdltest.GenNegMax, vdltest.GenPosMin, vdltest.GenNegMin, vdltest.GenRandom}
			for i := 0; i < numValuesPerTypeList; i++ {
				out <- valgen.Gen(types[rand.Intn(len(types))], modes[rand.Intn(len(modes))])
			}
		}
	}()
	return out
}

func main() {
	flag.Parse()
	for vv := range genValues() {
		if *verbose {
			fmt.Printf("testing %v\n", vv)
		}
		bytes, err := vom.Encode(vv)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%v: error on encode %v\n", vv, err)
			continue
		}
		var vvOut *vdl.Value
		if err := vom.Decode(bytes, &vvOut); err != nil {
			fmt.Fprintf(os.Stderr, "%v: error on decode %v\n", vv, err)
			continue
		}
		if !vdl.EqualValue(vv, vvOut) {
			fmt.Fprintf(os.Stderr, "%v: decoded value %v did not match input\n", vv, vvOut)
		}
	}
}
