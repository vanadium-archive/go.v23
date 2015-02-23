package java

import (
	"strings"
)

// javaPath converts the provided Go path into the Java path.  It replaces all "/"
// with "." in the path.
func javaPath(goPath string) string {
	return strings.Replace(goPath, "/", ".", -1)
}
