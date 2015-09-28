// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package internal

// This module contains performance benchmarks.
// Run with
//    jiri go test -bench . v.io/v23/vom/internal

// The following are the results of a typical run on a desktop machine on 2015/08/04,
// together with the results from comparable tests using protocol buffers in
// C++ and Go on the same machine.
//
// Allocations per vom.Encode                         :    572
// Allocations per vom.Decode                         :    405
// Allocations per vom.Encoder.Encode    (1 Customer) :    572
// Allocations per vom.Encoder.Encode (1000 Customers):  74508
// Allocations per vom.Decoder.Decode    (1 Customer) :   4469
// Allocations per vom.Decoder.Decode (1000 Customers): 405068
// Allocations per gob.Encoder.Encode    (1 Customer) :     54
// Allocations per gob.Encoder.Encode (1000 Customers):   3066
// Allocations per gob.Decoder.Decode    (1 Customer) :    523
// Allocations per gob.Decoder.Decode (1000 Customers):  14511
//
// BenchmarkVomEncodeCustomer         10000            143809 ns/op
// BenchmarkVomDecodeCustomer         20000             89189 ns/op
// BenchmarkVomEncoder1Customer       10000            135640 ns/op
// BenchmarkVomEncoder1000Customer      100          15527851 ns/op
// BenchmarkVomDecoder1Customer        2000            832101 ns/op
// BenchmarkVomDecoder1000Customer       20          74767365 ns/op
// BenchmarkGobEncoder1Customer       50000             26574 ns/op
// BenchmarkGobEncoder1000Customer      500           3792072 ns/op
// BenchmarkGobDecoder1Customer       20000             93381 ns/op
// BenchmarkGobDecoder1000Customer      500           3604404 ns/op
//
// C++ Protocol buffer encode (1 Customer)                 83 ns/op
// C++ Protocol buffer decode (1 Customer)                168 ns/op
// Go  Protocol buffer encode (1 Customer)               2191 ns/op
// Go  Protocol buffer decode (1 Customer)               3150 ns/op

import "bytes"
import "encoding/gob"
import "fmt"
import "testing"

import "v.io/v23/vom"

// customer is the record we encode and decode during tests.
var customer Customer = Customer{
	Name:   "John Smith",
	Id:     1,
	Active: true,
	Address: AddressInfo{
		Street: "1 Main St.",
		City:   "Palo Alto",
		State:  "CA",
		Zip:    "94303",
	},
	Credit: CreditReport{
		Agency: CreditAgencyEquifax,
		Report: AgencyReportEquifaxReport{EquifaxCreditReport{'A'}},
	},
}

// ------------------------------------
// vom.Encode benchmarking

func vomEncode(t testing.TB) {
	buf, err := vom.Encode(customer)
	if err != nil || len(buf) == 0 {
		t.Fatalf("vom.Enecode failed: %v", err)
	}
}

func TestVomEncodeCustomer(t *testing.T) {
	fmt.Printf("Allocations per vom.Encode                         : %6.0f\n",
		testing.AllocsPerRun(10, func() { vomEncode(t) }))
}

func BenchmarkVomEncodeCustomer(b *testing.B) {
	for i := 0; i != b.N; i++ {
		vomEncode(b)
	}
}

// ------------------------------------
// vom.Decode benchmarking

func vomDecode(t testing.TB, buf []byte) {
	var c Customer
	err := vom.Decode(buf, &c)
	if err != nil {
		t.Fatalf("vom.Decode failed: %v", err)
	}
}

func TestVomDecodeCustomer(t *testing.T) {
	buf, err := vom.Encode(customer)
	if err != nil || len(buf) == 0 {
		t.Fatalf("vom.Encode failed: %v", err)
	}
	fmt.Printf("Allocations per vom.Decode                         : %6.0f\n",
		testing.AllocsPerRun(10, func() { vomDecode(t, buf) }))
}

func BenchmarkVomDecodeCustomer(b *testing.B) {
	buf, err := vom.Encode(customer)
	if err != nil || len(buf) == 0 {
		b.Fatalf("vom.Encode failed")
	}
	b.ResetTimer()
	for i := 0; i != b.N; i++ {
		vomDecode(b, buf)
	}
}

// ------------------------------------
// vom.Encoder.Encode benchmarking

func vomEncoderEncode(t testing.TB, n int) {
	var buf bytes.Buffer
	encoder := vom.NewEncoder(&buf)
	for j := 0; j != n; j++ {
		err := encoder.Encode(customer)
		if err != nil {
			t.Fatalf("encoder.Encode failed: %v", err)
		}
	}
}

func TestVomEncoder(t *testing.T) {
	fmt.Printf("Allocations per vom.Encoder.Encode    (1 Customer) : %6.0f\n",
		testing.AllocsPerRun(10, func() { vomEncoderEncode(t, 1) }))
	fmt.Printf("Allocations per vom.Encoder.Encode (1000 Customers): %6.0f\n",
		testing.AllocsPerRun(10, func() { vomEncoderEncode(t, 1000) }))
}

func BenchmarkVomEncoder1Customer(b *testing.B) {
	for i := 0; i != b.N; i++ {
		vomEncoderEncode(b, 1)
	}
}

func BenchmarkVomEncoder1000Customer(b *testing.B) {
	for i := 0; i != b.N; i++ {
		vomEncoderEncode(b, 1000)
	}
}

// ------------------------------------
// vom.Decoder.Decode benchmarking

func vomDecoderDecode(t testing.TB, n int, buf []byte) {
	var c Customer
	decoder := vom.NewDecoder(bytes.NewReader(buf))
	for j := 0; j != n; j++ {
		err := decoder.Decode(&c)
		if err != nil {
			t.Fatalf("decoder.Decode failed: %v", err)
		}
	}
}

func TestVomDecoder(t *testing.T) {
	var buf bytes.Buffer
	encoder := vom.NewEncoder(&buf)
	for j := 0; j != 1000; j++ {
		err := encoder.Encode(customer)
		if err != nil {
			t.Fatalf("encoder.Encode failed")
		}
	}
	data := buf.Bytes()
	fmt.Printf("Allocations per vom.Decoder.Decode    (1 Customer) : %6.0f\n",
		testing.AllocsPerRun(10, func() { vomDecoderDecode(t, 1, data) }))
	fmt.Printf("Allocations per vom.Decoder.Decode (1000 Customers): %6.0f\n",
		testing.AllocsPerRun(10, func() { vomDecoderDecode(t, 1000, data) }))
}

func BenchmarkVomDecoder1Customer(b *testing.B) {
	var buf bytes.Buffer
	encoder := vom.NewEncoder(&buf)
	err := encoder.Encode(customer)
	if err != nil {
		b.Fatalf("encoder.Encode failed")
	}
	data := buf.Bytes()
	b.ResetTimer()
	for i := 0; i != b.N; i++ {
		vomDecoderDecode(b, 1, data)
	}
}

func BenchmarkVomDecoder1000Customer(b *testing.B) {
	var buf bytes.Buffer
	encoder := vom.NewEncoder(&buf)
	for j := 0; j != 1000; j++ {
		err := encoder.Encode(customer)
		if err != nil {
			b.Fatalf("encoder.Encode failed")
		}
	}
	data := buf.Bytes()
	b.ResetTimer()
	for i := 0; i != b.N; i++ {
		vomDecoderDecode(b, 1000, data)
	}
}

// ------------------------------------
// gob.Encoder.Encode benchmarking

func gobEncoderEncode(t testing.TB, n int) {
	var buf bytes.Buffer
	encoder := gob.NewEncoder(&buf)
	for j := 0; j != n; j++ {
		err := encoder.Encode(customer)
		if err != nil {
			t.Fatalf("encoder.Encode failed: %v", err)
		}
	}
}

func TestGobEncoder(t *testing.T) {
	fmt.Printf("Allocations per gob.Encoder.Encode    (1 Customer) : %6.0f\n",
		testing.AllocsPerRun(10, func() { gobEncoderEncode(t, 1) }))
	fmt.Printf("Allocations per gob.Encoder.Encode (1000 Customers): %6.0f\n",
		testing.AllocsPerRun(10, func() { gobEncoderEncode(t, 1000) }))
}

func BenchmarkGobEncoder1Customer(b *testing.B) {
	for i := 0; i != b.N; i++ {
		gobEncoderEncode(b, 1)
	}
}

func BenchmarkGobEncoder1000Customer(b *testing.B) {
	for i := 0; i != b.N; i++ {
		gobEncoderEncode(b, 1000)
	}
}

// ------------------------------------
// gob.Decoder.Decode benchmarking

func init() {
	gob.Register(AgencyReportEquifaxReport{})
}

func gobDecoderDecode(t testing.TB, n int, buf []byte) {
	var c Customer
	decoder := gob.NewDecoder(bytes.NewReader(buf))
	for j := 0; j != n; j++ {
		err := decoder.Decode(&c)
		if err != nil {
			t.Fatalf("decoder.Decode failed: %v", err)
		}
	}
}

func TestGobDecoder(t *testing.T) {
	var buf bytes.Buffer
	encoder := gob.NewEncoder(&buf)
	for j := 0; j != 1000; j++ {
		err := encoder.Encode(customer)
		if err != nil {
			t.Fatalf("encoder.Encode failed")
		}
	}
	data := buf.Bytes()
	fmt.Printf("Allocations per gob.Decoder.Decode    (1 Customer) : %6.0f\n",
		testing.AllocsPerRun(10, func() { gobDecoderDecode(t, 1, data) }))
	fmt.Printf("Allocations per gob.Decoder.Decode (1000 Customers): %6.0f\n",
		testing.AllocsPerRun(10, func() { gobDecoderDecode(t, 1000, data) }))
}

func BenchmarkGobDecoder1Customer(b *testing.B) {
	var buf bytes.Buffer
	encoder := gob.NewEncoder(&buf)
	err := encoder.Encode(customer)
	if err != nil {
		b.Fatalf("encoder.Encode failed")
	}
	data := buf.Bytes()
	b.ResetTimer()
	for i := 0; i != b.N; i++ {
		gobDecoderDecode(b, 1, data)
	}
}

func BenchmarkGobDecoder1000Customer(b *testing.B) {
	var buf bytes.Buffer
	encoder := gob.NewEncoder(&buf)
	for j := 0; j != 1000; j++ {
		err := encoder.Encode(customer)
		if err != nil {
			b.Fatalf("encoder.Encode failed")
		}
	}
	data := buf.Bytes()
	b.ResetTimer()
	for i := 0; i != b.N; i++ {
		gobDecoderDecode(b, 1000, data)
	}
}
