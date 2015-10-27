// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This benchmark measures time spent on unsynced Put/Get/Delete/Scan/Query
// operations.
//
// All tests use 9-digit keys and values of type sampleStruct.
// 100K benchmarks use values with 100K byte values inside the struct.
//
// Results (jiri go test v.io/v23/syncbase/nosql -bench . -run Benchmark):
// BenchmarkPut-12       	     500	   3681219 ns/op
// BenchmarkGet-12       	     500	   2595279 ns/op
// BenchmarkDelete-12    	     500	   3380728 ns/op
// BenchmarkScan-12      	    3000	    402552 ns/op
// BenchmarkExec-12      	   20000	     71024 ns/op
// BenchmarkPut100K-12   	     500	   4435239 ns/op
// BenchmarkGet100K-12   	     500	   3828251 ns/op
// BenchmarkDelete100K-12	     500	   3379342 ns/op
// BenchmarkScan100K-12  	    1000	   1741592 ns/op
// BenchmarkExec100K-12  	    2000	   1079510 ns/op
package nosql_test

import (
	"fmt"
	"math/rand"
	"testing"

	"v.io/v23/context"
	"v.io/v23/syncbase"
	"v.io/v23/syncbase/nosql"
	_ "v.io/x/ref/runtime/factories/generic"
	tu "v.io/x/ref/services/syncbase/testutil"
)

// prepare creates a service, app, database and table "tb" within the database,
// returning the context, handle to the database and table, and the clean up
// function.
func prepare() (*context.T, nosql.Database, nosql.Table, func()) {
	ctx, sName, cleanup := tu.SetupOrDie(nil)
	s := syncbase.NewService(sName)
	a := s.App("a")
	if err := a.Create(ctx, nil); err != nil {
		panic(fmt.Sprintf("can't create an app: %v", err))
	}
	d := a.NoSQLDatabase("d", nil)
	if err := d.Create(ctx, nil); err != nil {
		panic(fmt.Sprintf("can't create a database: %v", err))
	}
	tb := d.Table("tb")
	if err := tb.Create(ctx, nil); err != nil {
		panic(fmt.Sprintf("can't create a table: %v", err))
	}
	return ctx, d, tb, cleanup
}

type sampleStruct struct {
	A string
	B int
	C *testStruct
	D []byte
}

func generateSampleStruct() interface{} {
	return sampleStruct{A: "hello, world!", B: 42}
}

func generate100KStruct() interface{} {
	var byteSlice []byte
	r := rand.New(rand.NewSource(23917))
	for i := 0; i < 100*1000; i++ {
		byteSlice = append(byteSlice, byte(r.Intn(256)))
	}
	return sampleStruct{A: "hello, world!", B: 42, D: byteSlice}
}

func generateTestData(b *testing.B, ctx *context.T, tb nosql.Table, value interface{}) {
	for i := 0; i < b.N; i++ {
		if err := tb.Put(ctx, fmt.Sprintf("%09d", i), value); err != nil {
			b.Fatalf("can't generate test data: %v", err)
		}
	}
}

func runPutBenchmark(b *testing.B, value interface{}) {
	ctx, _, tb, cleanup := prepare()
	defer cleanup()
	b.ResetTimer()
	generateTestData(b, ctx, tb, value)
	b.StopTimer()
}

func runGetBenchmark(b *testing.B, value interface{}) {
	ctx, _, tb, cleanup := prepare()
	defer cleanup()
	generateTestData(b, ctx, tb, value)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var got sampleStruct
		if err := tb.Get(ctx, fmt.Sprintf("%09d", i), &got); err != nil {
			b.Fatalf("tb.Get failed: %v", err)
		}
	}
	b.StopTimer()
}

func runDeleteBenchmark(b *testing.B, value interface{}) {
	ctx, _, tb, cleanup := prepare()
	defer cleanup()
	generateTestData(b, ctx, tb, value)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := tb.Delete(ctx, fmt.Sprintf("%09d", i)); err != nil {
			b.Fatalf("tb.Delete failed: %v", err)
		}
	}
	b.StopTimer()
}

func runScanBenchmark(b *testing.B, value interface{}) {
	ctx, _, tb, cleanup := prepare()
	defer cleanup()
	generateTestData(b, ctx, tb, value)
	b.ResetTimer()
	s := tb.Scan(ctx, nosql.Prefix(""))
	for i := 0; i < b.N; i++ {
		var got sampleStruct
		s.Advance()
		s.Value(&got)
	}
	b.StopTimer()
}

func runExecBenchmark(b *testing.B, value interface{}) {
	ctx, d, tb, cleanup := prepare()
	defer cleanup()
	generateTestData(b, ctx, tb, value)
	b.ResetTimer()
	_, s, _ := d.Exec(ctx, "select v from tb")
	for i := 0; i < b.N; i++ {
		s.Advance()
		s.Result()
	}
	b.StopTimer()
}

func BenchmarkPut(b *testing.B) {
	runPutBenchmark(b, generateSampleStruct())
}

func BenchmarkGet(b *testing.B) {
	runGetBenchmark(b, generateSampleStruct())
}

func BenchmarkDelete(b *testing.B) {
	runDeleteBenchmark(b, generateSampleStruct())
}

func BenchmarkScan(b *testing.B) {
	runScanBenchmark(b, generateSampleStruct())
}

func BenchmarkExec(b *testing.B) {
	runExecBenchmark(b, generateSampleStruct())
}

func BenchmarkPut100K(b *testing.B) {
	runPutBenchmark(b, generate100KStruct())
}

func BenchmarkGet100K(b *testing.B) {
	runGetBenchmark(b, generate100KStruct())
}

func BenchmarkDelete100K(b *testing.B) {
	runDeleteBenchmark(b, generate100KStruct())
}

func BenchmarkScan100K(b *testing.B) {
	runScanBenchmark(b, generate100KStruct())
}

func BenchmarkExec100K(b *testing.B) {
	runExecBenchmark(b, generate100KStruct())
}
