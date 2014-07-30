package java

import (
	"strings"
)

// javaRawComment extracts a raw language-independent comment from a VDL comment.
func javaRawComment(vdlComment string) string {
	if vdlComment == "" {
		return ""
	}
	comment := strings.Replace(vdlComment, "/**", "", -1)
	comment = strings.Replace(comment, "/*", "", -1)
	comment = strings.Replace(comment, "*/", "", -1)
	comment = strings.Replace(comment, "//", "", -1)
	comment = strings.Replace(comment, "\n *", "\n", -1)
	splitComment := strings.Split(comment, "\n")
	for i, _ := range splitComment {
		splitComment[i] = strings.TrimSpace(splitComment[i])
	}
	return strings.TrimSpace(strings.Join(splitComment, "\n"))
}

// javaDocInComment transforms a VDL comment to javadoc style, but without starting a new comment.
// (i.e. this assumes that the output will be within a /**  */ comment).
func javaDocInComment(vdlComment string) string {
	if vdlComment == "" {
		return ""
	}
	return "\n * " + strings.Replace(javaRawComment(vdlComment), "\n", "\n * ", -1)
}

// javaDoc transforms the provided VDL comment into the JavaDoc format.
// This starts a new javadoc comment block.
func javaDoc(vdlComment string) string {
	if vdlComment == "" {
		return ""
	}
	return "/**" + javaDocInComment(vdlComment) + "\n */\n"
}
