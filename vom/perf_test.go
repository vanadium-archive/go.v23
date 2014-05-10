package vom

// This file was based heavily on go/src/pkg/encoding/gob/timing_test.go, then
// modified to suit our needs.

import (
	"bytes"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"testing"
)

type coderType int

const (
	coderGob coderType = iota
	coderVom
	coderJSON
	coderVomJSON
)

func (ct coderType) String() string {
	switch ct {
	case coderGob:
		return "gob"
	case coderJSON:
		return "json"
	case coderVom:
		return "vom"
	case coderVomJSON:
		return "vom/json"
	default:
		panic(fmt.Errorf("vom: unknown coderType %d", ct))
	}
}

type encoder interface {
	Encode(v interface{}) error
}

type decoder interface {
	Decode(v interface{}) error
}

func newEncoderPerf(w io.Writer, ct coderType) encoder {
	switch ct {
	case coderGob:
		return gob.NewEncoder(w)
	case coderJSON:
		return json.NewEncoder(w)
	case coderVom:
		return NewEncoder(w)
	case coderVomJSON:
		return NewEncoder(w).SetFormat(FormatJSON)
	default:
		panic(fmt.Errorf("vom: unknown coderType %d", ct))
	}
}

func newDecoderPerf(r io.Reader, ct coderType) decoder {
	switch ct {
	case coderGob:
		return gob.NewDecoder(r)
	case coderJSON:
		return json.NewDecoder(r)
	case coderVom, coderVomJSON:
		return NewDecoder(r)
	default:
		panic(fmt.Errorf("vom: unknown coderType %d", ct))
	}
}

// TODO(toddw): Run tests of different types and values, e.g. each primitive
// individually, larger values, etc.
type Bench struct {
	A int
	B float64
	C string
	D []byte
	E []uint
	F [3]uint
	G map[string]uint
}

func init() {
	// TODO(toddw): This is the only way to the get the same-struct type
	// optimization in decodeStruct.  Think about other ways.
	Register(Bench{})
}

func newBench() *Bench {
	return &Bench{7, 3.2, "now is the time", []byte("for all good men"), []uint{1, 2, 3}, [3]uint{4, 5, 6}, map[string]uint{"one": 1, "two": 2, "three": 3}}
}

func makePipe(b *testing.B) (*os.File, *os.File) {
	r, w, err := os.Pipe()
	if err != nil {
		b.Fatal("os.Pipe: ", err)
	}
	return r, w
}

func benchmarkEnc(b *testing.B, w io.Writer, ct coderType) {
	b.StopTimer()
	enc := newEncoderPerf(w, ct)
	bench := newBench()
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		if err := enc.Encode(bench); err != nil {
			b.Fatal(err)
		}
	}
}
func BenchmarkEncBufGob(b *testing.B) {
	var buf bytes.Buffer
	benchmarkEnc(b, &buf, coderGob)
}
func BenchmarkEncBufVom(b *testing.B) {
	var buf bytes.Buffer
	benchmarkEnc(b, &buf, coderVom)
}
func BenchmarkEncBufJSON(b *testing.B) {
	var buf bytes.Buffer
	benchmarkEnc(b, &buf, coderJSON)
}
func BenchmarkEncBufVomJSON(b *testing.B) {
	var buf bytes.Buffer
	benchmarkEnc(b, &buf, coderVomJSON)
}

func benchmarkDec(b *testing.B, r io.Reader, w io.Writer, ct coderType) {
	b.StopTimer()
	enc := newEncoderPerf(w, ct)
	dec := newDecoderPerf(r, ct)
	bench := newBench()
	for i := 0; i < b.N; i++ {
		if err := enc.Encode(bench); err != nil {
			b.Fatal(err)
		}
	}
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		if err := dec.Decode(bench); err != nil {
			b.Fatal(err)
		}
	}
}
func BenchmarkDecBufGob(b *testing.B) {
	var buf bytes.Buffer
	benchmarkDec(b, &buf, &buf, coderGob)
}
func BenchmarkDecBufVom(b *testing.B) {
	var buf bytes.Buffer
	benchmarkDec(b, &buf, &buf, coderVom)
}
func BenchmarkDecBufJSON(b *testing.B) {
	var buf bytes.Buffer
	benchmarkDec(b, &buf, &buf, coderJSON)
}
func BenchmarkDecBufVomJSON(b *testing.B) {
	var buf bytes.Buffer
	benchmarkDec(b, &buf, &buf, coderVomJSON)
}

func benchmarkEncDec(b *testing.B, r io.Reader, w io.Writer, ct coderType) {
	b.StopTimer()
	enc := newEncoderPerf(w, ct)
	dec := newDecoderPerf(r, ct)
	bench := newBench()
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		if err := enc.Encode(bench); err != nil {
			b.Fatal(err)
		}
		if err := dec.Decode(bench); err != nil {
			b.Fatal(err)
		}
	}
}
func BenchmarkEncDecBufGob(b *testing.B) {
	var buf bytes.Buffer
	benchmarkEncDec(b, &buf, &buf, coderGob)
}
func BenchmarkEncDecBufVom(b *testing.B) {
	var buf bytes.Buffer
	benchmarkEncDec(b, &buf, &buf, coderVom)
}
func BenchmarkEncDecBufJSON(b *testing.B) {
	var buf bytes.Buffer
	benchmarkEncDec(b, &buf, &buf, coderJSON)
}
func BenchmarkEncDecBufVomJSON(b *testing.B) {
	var buf bytes.Buffer
	benchmarkEncDec(b, &buf, &buf, coderVomJSON)
}
func BenchmarkEncDecPipeGob(b *testing.B) {
	r, w := makePipe(b)
	benchmarkEncDec(b, r, w, coderGob)
}
func BenchmarkEncDecPipeVom(b *testing.B) {
	r, w := makePipe(b)
	benchmarkEncDec(b, r, w, coderVom)
}
func BenchmarkEncDecPipeJSON(b *testing.B) {
	r, w := makePipe(b)
	benchmarkEncDec(b, r, w, coderJSON)
}
func BenchmarkEncDecPipeVomJSON(b *testing.B) {
	r, w := makePipe(b)
	benchmarkEncDec(b, r, w, coderVomJSON)
}
