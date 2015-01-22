// The following enables go generate to generate the doc.go file.
//go:generate go run $VANADIUM_ROOT/release/go/src/v.io/lib/cmdline/testdata/gendoc.go .

package main

import (
	"os"

	"v.io/core/veyron2/vdl/vdl/cmds"
)

func main() {
	os.Exit(cmds.Root().Main())
}
