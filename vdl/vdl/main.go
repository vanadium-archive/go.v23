// The following enables go generate to generate the doc.go file.
//go:generate go run $VANADIUM_ROOT/veyron/go/src/veyron.io/lib/cmdline/testdata/gendoc.go .

package main

import "veyron.io/veyron/veyron2/vdl/vdl/cmds"

func main() {
	cmds.Root().Main()
}
