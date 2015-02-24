package java

import (
	"bytes"

	"v.io/v23/vdl/compile"
)

// javaFileNames constructs a comma separated string with the short (basename) of the input files
func javaFileNames(files []*compile.File) string {
	var buf bytes.Buffer
	for i, file := range files {
		if i > 0 {
			buf.WriteString(", ")
		}
		buf.WriteString(file.BaseName)
	}
	return buf.String()
}
