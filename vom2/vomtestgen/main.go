// The following enables go generate to generate the doc.go file.
//go:generate go run $VANADIUM_ROOT/veyron/go/src/v.io/lib/cmdline/testdata/gendoc.go . -help

package main

func main() {
	cmdGenerate.Main()
}
