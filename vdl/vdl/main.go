// The following enables go generate to generate the doc.go file.
// Things to look out for:
// 1) go:generate evaluates double-quoted strings into a single argument.
// 2) go:generate performs $NAME expansion, so the bash cmd can't contain '$'.
// 3) We generate into a *.tmp file first, otherwise go run will pick up the
//    initially empty *.go file, and fail.
//
//go:generate bash -c "{ echo -e '// This file was auto-generated via go generate.\n// DO NOT UPDATE MANUALLY\n\n/*' && veyron go run *.go help -style=godoc ... && echo -e '*/\npackage main'; } > ./doc.go.tmp && mv ./doc.go.tmp ./doc.go"

package main

import "veyron.io/veyron/veyron2/vdl/vdl/cmds"

func main() {
	cmds.Root().Main()
}
