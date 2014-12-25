// The following enables go generate to generate the doc.go file.
//go:generate go run $VANADIUM_ROOT/veyron/go/src/v.io/lib/cmdline/testdata/gendoc.go .

package main

import "v.io/veyron/veyron2/vdl/vdl/cmds"

func main() {
	cmds.Root().Main()
}
