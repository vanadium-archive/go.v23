// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build newvdltests

package vom_test

import (
	"bytes"
	"fmt"
	"io"
	"reflect"
	"sync"
	"testing"

	"v.io/v23/vdl"
	"v.io/v23/vom"
	"v.io/v23/vom/vomtest"

	// Import verror to ensure that interface tests result in *verror.E
	_ "v.io/v23/verror"
)

// TODO(toddw): Add all useful tests from decoder_test.go

var (
	rtIface = reflect.TypeOf((*interface{})(nil)).Elem()
	rtValue = reflect.TypeOf(vdl.Value{})
)

func TestXDecoder(t *testing.T) {
	// The decoder tests take a long time, so we run them concurrently.
	var pending sync.WaitGroup
	for _, test := range vomtest.AllPass() {
		pending.Add(1)
		go func(test vomtest.Entry) {
			defer pending.Done()
			testXDecoder(t, "[go value]", test, rvPtrValue(test.Value))
			testXDecoder(t, "[go iface]", test, rvPtrIface(test.Value))
			vv, err := vdl.ValueFromReflect(test.Value)
			if err != nil {
				t.Errorf("%s: ValueFromReflect failed: %v", test.Name(), err)
				continue
			}
			vvWant := reflect.ValueOf(vv)
			testXDecoder(t, "[new *vdl.Value]", test, vvWant)
			testXDecoderFunc(t, "[zero vdl.Value]", test, vvWant, func() reflect.Value {
				return reflect.ValueOf(vdl.ZeroValue(vv.Type()))
			})
		}(test)
	}
	pending.Wait()
}

func rvPtrValue(rv reflect.Value) reflect.Value {
	result := reflect.New(rv.Type())
	result.Elem().Set(rv)
	return result
}

func rvPtrIface(rv reflect.Value) reflect.Value {
	result := reflect.New(rtIface)
	result.Elem().Set(rv)
	return result
}

func testXDecoder(t *testing.T, pre string, test vomtest.Entry, rvWant reflect.Value) {
	testXDecoderFunc(t, pre, test, rvWant, func() reflect.Value {
		return reflect.New(rvWant.Type().Elem())
	})
	// TODO(toddw): Add tests that start with a randomly-set value.
}

func testXDecoderFunc(t *testing.T, pre string, test vomtest.Entry, rvWant reflect.Value, rvNew func() reflect.Value) {
	readEOF := make([]byte, 1)
	for _, mode := range vom.AllReadModes {
		// Test vom.NewXDecoder.
		{
			name := fmt.Sprintf("%s (%s) %s", pre, mode, test.Name())
			rvGot := rvNew()
			reader := mode.TestReader(bytes.NewReader(test.Bytes()))
			dec := vom.NewXDecoder(reader)
			if err := dec.Decode(rvGot.Interface()); err != nil {
				t.Errorf("%s: Decode failed: %v", name, err)
				return
			}
			if !vdl.DeepEqualReflect(rvGot, rvWant) {
				t.Errorf("%s\nGOT  %v\nWANT %v", name, rvGot, rvWant)
				return
			}
			if n, err := reader.Read(readEOF); n != 0 || err != io.EOF {
				t.Errorf("%s: reader got (%d,%v), want (0,EOF)", name, n, err)
			}
		}
		// Test vom.NewXDecoderWithTypeDecoder
		{
			name := fmt.Sprintf("%s (%s with TypeDecoder) %s", pre, mode, test.Name())
			rvGot := rvNew()
			readerT := mode.TestReader(bytes.NewReader(test.TypeBytes()))
			decT := vom.NewTypeDecoder(readerT)
			decT.Start()
			reader := mode.TestReader(bytes.NewReader(test.ValueBytes()))
			dec := vom.NewXDecoderWithTypeDecoder(reader, decT)
			err := dec.Decode(rvGot.Interface())
			decT.Stop()
			if err != nil {
				t.Errorf("%s: Decode failed: %v", name, err)
				return
			}
			if !vdl.DeepEqualReflect(rvGot, rvWant) {
				t.Errorf("%s\nGOT  %v\nWANT %v", name, rvGot, rvWant)
				return
			}
			if n, err := reader.Read(readEOF); n != 0 || err != io.EOF {
				t.Errorf("%s: reader got (%d,%v), want (0,EOF)", name, n, err)
			}
			if n, err := readerT.Read(readEOF); n != 0 || err != io.EOF {
				t.Errorf("%s: readerT got (%d,%v), want (0,EOF)", name, n, err)
			}
		}
	}
	// Test single-shot vom.Decode twice, to ensure we test the cache hit case.
	for i := 0; i < 2; i++ {
		name := fmt.Sprintf("%s (single-shot %d) %s", pre, i, test.Name())
		rvGot := rvNew()
		if err := vom.XDecode(test.Bytes(), rvGot.Interface()); err != nil {
			t.Errorf("%s: Decode failed: %v", name, err)
			return
		}
		if !vdl.DeepEqualReflect(rvGot, rvWant) {
			t.Errorf("%s\nGOT  %v\nWANT %v", name, rvGot, rvWant)
			return
		}
	}
}
