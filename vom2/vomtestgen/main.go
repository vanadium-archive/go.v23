// The following enables go generate to generate the doc.go file.
//go:generate go run $VEYRON_ROOT/lib/cmdline/testdata/gendoc.go .

package main

func main() {
	cmdGenerate.Main()
}
