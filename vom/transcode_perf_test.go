package vom

import (
	"bytes"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

var transcodeBenches = []struct {
	json string
	bin  string
	typ  Type
}{
	{
		`["A", "B", "C", "D", "E", "F", "G", "H", "I", "J", "K", "L", "M", "N", "O", "P", "Q", "R"]`,
		toBin("ff810412010300ff822512014101420143014401450146014701480149014a014b014c014d014e014f015001510152"),
		TypeOf([]string{}),
	},
	{
		`{"A": 1, "B": 2, "C": 3, "D": 4, "E": 5, "F": 6, "G": 7, "E": 8, "F": 9, "G": 10}`,
		toBin("ff8106160103012100ff821f0a01410201420401430601440801450a01460c01470e014510014612014714"),
		TypeOf(map[string]int{}),
	},
	{
		`{"4": 5, "5.4": "X", "\"A\"": [4]}`,
		toBin("ff8106160101010100ff830412010100ff82200334fe104034fe144034f89a99999999991540060158060141ff840134fe1040"),
		TypeOf(map[interface{}]interface{}{}),
	},
	{
		`{"body" : ["<html>", "<body>", "</body>", "</html>"],
            "headers" : {"first" : "1", "second": "2"}}`,
		toBin("ff811918010201420104426f64790001430107486561646572730000ff830412010300ff8506160103010300ff82340104063c68746d6c3e063c626f64793e073c2f626f64793e073c2f68746d6c3e01020566697273740131067365636f6e64013200"),
		TypeOf(struct {
			Body    []string
			Headers map[string]string
		}{}),
	},
}

// toBin converts a hex string to a binary string.
func toBin(hexPat string) string {
	bin, err := binFromHexPat(hexPat)
	if err != nil {
		panic(fmt.Sprintf("error converting %v to binary: %v", hexPat, err))
	}

	return bin
}

// Benchmarks transcoding JSON to binary VOM.
func BenchmarkJSONToVom(b *testing.B) {
	var buf bytes.Buffer
	for i := 0; i < b.N; i++ {
		for _, bench := range transcodeBenches {
			if err := NewJSONToBinaryTranscoder(&buf, strings.NewReader(bench.json)).Transcode(bench.typ); err != nil {
				b.Fatal(err)
			}
			buf.Reset()
		}
	}
}

// Benchmarks transcoding binary VOM to JSON.
func BenchmarkVomToJSON(b *testing.B) {
	for _, bench := range transcodeBenches {
		var buf bytes.Buffer
		for i := 0; i < b.N; i++ {
			VOMBinaryToJSON(&buf, strings.NewReader(bench.bin))
			buf.Reset()
		}
	}
}

// Tests that the transcoding from JSON to VOM works properly.
func TestJSONToVomBenchmark(t *testing.T) {
	for _, bench := range transcodeBenches {
		// JSON -> VOM:
		var buf bytes.Buffer
		if err := NewJSONToBinaryTranscoder(&buf, strings.NewReader(bench.json)).Transcode(bench.typ); err != nil {
			t.Fatalf("error during json to vom transcode: %v (input: %v)", err, bench.json)
		}

		// Decode transcoded VOM:
		var v interface{}
		if err := NewDecoder(bytes.NewReader(buf.Bytes())).Decode(&v); err != nil {
			t.Fatalf("error decoding vom binary")
		}

		// Decode expected binary:
		var vExp interface{}
		if err := NewDecoder(strings.NewReader(bench.bin)).Decode(&vExp); err != nil {
			t.Fatalf("error decoding expected vom binary")
		}

		// Compare:
		if !reflect.DeepEqual(v, vExp) {
			t.Fatalf("decoded objects didn't match %v and %v.", v, vExp)
		}
	}
}

// Tests that the transcoding from VOM to JSON works properly.
func TestVomToJSONBenchmark(t *testing.T) {
	for _, bench := range transcodeBenches {

		var buf bytes.Buffer
		err := VOMBinaryToJSON(&buf, strings.NewReader(bench.bin))
		if err != nil {
			t.Error("error converting vom binary to JSON: ", err)
			continue
		}

		if err := equivalentJSON(buf.String(), bench.json); err != nil {
			t.Error(err)
		}
	}
}
