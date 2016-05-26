// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Generator for VOM benchmarks.

// The following generates the benchmarks
//go:generate $JIRI_ROOT/release/go/src/v.io/v23/vom/internal/gen/gen.sh

package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func main() {
	flag.Parse()
	file, err := os.Create(flag.Arg(0))
	if err != nil {
		panic(err)
	}

	file.WriteString(`// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Do not modify! This file is generated by:
// jiri go generate v.io/v23/vom/internal/gen

package internal

import (
	"testing"
	"errors"
	"time"

	"v.io/v23/rpc"
	"v.io/v23/vom"
	"v.io/v23/vtrace"
	"v.io/v23/uniqueid"
	"v.io/v23/security"
	wiretime "v.io/v23/vdlroot/time"
)

	`)
	for _, entry := range benchmarks {
		file.WriteString(genBenchmark(entry))
	}
	file.Close()
	exec.Command("go", "go")
}

// TODO(bprosnitz) Also generate JSON benchmarks with both our JSON encoder / decoder
// and the built-in go one for comparison.
type GeneratorEntry struct {
	Name  string
	Type  string
	Value string
	Gob   bool
}

var benchmarks []GeneratorEntry = []GeneratorEntry{
	{
		Name:  `XNumber`,
		Type:  `XNumber`,
		Value: `XNumber(2)`,
	},
	{
		Name:  `VNumber`,
		Type:  `VNumber`,
		Value: `VNumber(2)`,
	},
	{
		Name:  `XStringSmall`,
		Type:  `XString`,
		Value: `XString("abc")`,
	},
	{
		Name:  `VStringSmall`,
		Type:  `VString`,
		Value: `VString("abc")`,
	},
	{
		Name:  `XStringLarge`,
		Type:  `XString`,
		Value: `XString(createString(65536))`,
	},
	{
		Name:  `VStringLarge`,
		Type:  `VString`,
		Value: `VString(createString(65536))`,
	},
	{
		Name:  `VEnum`,
		Type:  `VEnum`,
		Value: `VEnumA`,
	},
	{
		Name:  `XByteListSmall`,
		Type:  `XByteList`,
		Value: `XByteList{1, 2, 3}`,
	},
	{
		Name:  `VByteListSmall`,
		Type:  `VByteList`,
		Value: `VByteList{1, 2, 3}`,
	},
	{
		Name:  `XByteListLarge`,
		Type:  `XByteList`,
		Value: `XByteList(createByteList(65536))`,
	},
	{
		Name:  `VByteListLarge`,
		Type:  `VByteList`,
		Value: `VByteList(createByteList(65536))`,
	},
	{
		Name:  `XByteArray`,
		Type:  `XByteArray`,
		Value: `XByteArray{1, 2, 3}`,
	},
	{
		Name:  `VByteArray`,
		Type:  `VByteArray`,
		Value: `VByteArray{1, 2, 3}`,
	},
	{
		Name:  `XArray`,
		Type:  `XArray`,
		Value: `XArray{1, 2, 3}`,
	},
	{
		Name:  `VArray`,
		Type:  `VArray`,
		Value: `VArray{1, 2, 3}`,
	},
	{
		Name:  `XListSmall`,
		Type:  `XList`,
		Value: `XList{1, 2, 3}`,
	},
	{
		Name:  `VListSmall`,
		Type:  `VList`,
		Value: `VList{1, 2, 3}`,
	},
	{
		Name:  `XListLarge`,
		Type:  `XList`,
		Value: `XList(createList(65536))`,
	},
	{
		Name:  `VListLarge`,
		Type:  `VList`,
		Value: `VList(createList(65536))`,
	},
	// We will be handling data structures with a lot of anys for JSON:
	{
		Name:  `XListAnySmall`,
		Type:  `XListAny`,
		Value: `XListAny{vom.RawBytesOf(1), vom.RawBytesOf(2), vom.RawBytesOf(3)}`,
	},
	{
		Name:  `VListAnySmall`,
		Type:  `VListAny`,
		Value: `VListAny{vom.RawBytesOf(1), vom.RawBytesOf(2), vom.RawBytesOf(3)}`,
	},
	{
		Name:  `XListAnyLarge`,
		Type:  `XListAny`,
		Value: `XListAny(createListAny(65536))`,
	},
	{
		Name:  `VListAnyLarge`,
		Type:  `VListAny`,
		Value: `VListAny(createListAny(65536))`,
	},
	{
		Name:  `VSet`,
		Type:  `VSet`,
		Value: `VSet{"A": struct{}{}, "B": struct{}{}, "C": struct{}{}}`,
	},
	{
		Name:  `XMap`,
		Type:  `XMap`,
		Value: `XMap{"A": true, "B": false, "C": true}`,
	},
	{
		Name:  `VMap`,
		Type:  `VMap`,
		Value: `VMap{"A": true, "B": false, "C": true}`,
	},
	{
		Name:  `XSmallStruct`,
		Type:  `XSmallStruct`,
		Value: `XSmallStruct{1, "A", true}`,
	},
	{
		Name:  `VSmallStruct`,
		Type:  `VSmallStruct`,
		Value: `VSmallStruct{1, "A", true}`,
	},
	{
		Name:  `XLargeStruct`,
		Type:  `XLargeStruct`,
		Value: `XLargeStruct{1,2,3,4,5,6,7,8,9,10,11,12,13,14,15,16,17,18,19,20,21,22,23,24,25,26,27,28,29,30,31,32,33,34,35,36,37,38,39,40,41,42,43,44,45,46,47,48,49,50}`,
	},
	{
		Name:  `VLargeStruct`,
		Type:  `VLargeStruct`,
		Value: `VLargeStruct{1,2,3,4,5,6,7,8,9,10,11,12,13,14,15,16,17,18,19,20,21,22,23,24,25,26,27,28,29,30,31,32,33,34,35,36,37,38,39,40,41,42,43,44,45,46,47,48,49,50}`,
	},
	{
		Name:  `XLargeStructZero`,
		Type:  `XLargeStruct`,
		Value: `XLargeStruct{}`,
	},
	{
		Name:  `VLargeStructZero`,
		Type:  `VLargeStruct`,
		Value: `VLargeStruct{}`,
	},
	{
		Name:  `VSmallUnion`,
		Type:  `VSmallUnion`,
		Value: `VSmallUnionA{1}`,
	},
	{
		Name:  `Time`,
		Type:  `time.Time`,
		Value: `time.Date(2009, time.November, 10, 23, 0, 0, 0, time.UTC)`,
		Gob:   true,
	},
	{
		Name:  `Blessings`,
		Type:  `security.Blessings`,
		Value: `createTypicalBlessings()`,
	},
	{
		Name:  `RPCRequestZero`,
		Type:  `rpc.Request`,
		Value: `rpc.Request{}`,
	},
	{
		Name: `RPCRequestFull`,
		Type: `rpc.Request`,
		Value: `rpc.Request{
	Suffix: "a suffix",
	Method: "a method",
	NumPosArgs: 23,
	EndStreamArgs: true,
	Deadline: wiretime.Deadline{
		time.Date(2009, time.November, 10, 23, 0, 0, 0, time.UTC),
	},
	GrantedBlessings: createTypicalBlessings(),
	TraceRequest: vtrace.Request{
		SpanId: uniqueid.Id{1,2,3,4},
		TraceId: uniqueid.Id{5,6,7,8},
		Flags: vtrace.CollectInMemory,
		LogLevel: 3,
	},
	Language: "en-us",
}`,
	},
	{
		Name:  `RPCResponseZero`,
		Type:  `rpc.Response`,
		Value: `rpc.Response{}`,
	},
	{
		Name: `RPCResponseFull`,
		Type: `rpc.Response`,
		Value: `rpc.Response{
	Error: errors.New("testerror"),
	EndStreamResults: true,
	NumPosResults: 4,
	TraceResponse: vtrace.Response{
		Flags: vtrace.CollectInMemory,
		Trace: vtrace.TraceRecord{
			Id: uniqueid.Id{1,2,3,4},
			Spans: []vtrace.SpanRecord{
				vtrace.SpanRecord{
					Id: uniqueid.Id{1,2,3,4},
					Parent: uniqueid.Id{4,3,2,1},
					Name: "span name",
					Start: time.Date(2009, time.November, 10, 23, 0, 0, 0, time.UTC),
					End: time.Date(2009, time.November, 11, 23, 0, 0, 0, time.UTC),
					Annotations: []vtrace.Annotation{
						vtrace.Annotation {
							When: time.Date(2009, time.November, 10, 23, 0, 0, 4, time.UTC),
							Message: "Annotation Message",
						},
					},
				},
			},
		},
	},
}`,
	},
}

func genVomEncode(name, value string) string {
	return fmt.Sprintf(`
func BenchmarkVom___Encode_____%[1]s(b *testing.B) {
	 vomEncode(b, %[2]s)
}
func BenchmarkVom___EncodeMany_%[1]s(b *testing.B) {
	vomEncodeMany(b, %[2]s)
}`, name, value)
}

func genVomDecode(name, value, typ string) string {
	return fmt.Sprintf(`
func BenchmarkVom___Decode_____%[1]s(b *testing.B) {
	vomDecode(b, %[2]s, func() interface{} { return new(%[3]s) })
}
func BenchmarkVom___DecodeMany_%[1]s(b *testing.B) {
	vomDecodeMany(b, %[2]s, func() interface{} { return new(%[3]s) })
}`, name, value, typ)
}

func genGobEncode(name, value string) string {
	return fmt.Sprintf(`
func BenchmarkGob___Encode_____%[1]s(b *testing.B) {
	 gobEncode(b, %[2]s)
}
func BenchmarkGob___EncodeMany_%[1]s(b *testing.B) {
	 gobEncodeMany(b, %[2]s)
}`, name, value)
}

func genGobDecode(name, value, typ string) string {
	return fmt.Sprintf(`
func BenchmarkGob___Decode_____%[1]s(b *testing.B) {
	gobDecode(b, %[2]s, func() interface{} { return new(%[3]s) })
}
func BenchmarkGob___DecodeMany_%[1]s(b *testing.B) {
	gobDecodeMany(b, %[2]s, func() interface{} { return new(%[3]s) })
}`, name, value, typ)
}

func genBenchmark(entry GeneratorEntry) string {
	var str string
	str += genVomEncode(entry.Name, entry.Value)
	if shouldGenGob(entry) {
		str += genGobEncode(entry.Name, entry.Value)
	}
	str += genVomDecode(entry.Name, entry.Value, entry.Type)
	if shouldGenGob(entry) {
		str += genGobDecode(entry.Name, entry.Value, entry.Type)
	}
	return str
}

func shouldGenGob(entry GeneratorEntry) bool {
	if entry.Gob {
		return true
	}
	return strings.HasPrefix(entry.Name, "X") && !strings.Contains(entry.Name, "Any")
}
