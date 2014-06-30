package compile

import (
	"veyron2/vdl/util"
)

// reservedWord checks if identifiers are reserved after they are converted to the native form for the language.
func reservedWord(ident string) bool {
	return reservedWordJava(ident) ||
		reservedWordJavascript(ident) ||
		reservedWordGo(ident)
	// TODO(bprosnitz) Other identifiers? (set, assert, raise, with, etc)
}

func reservedWordJava(ident string) bool {
	_, isReserved := javaReservedWords[util.ToCamelCase(ident)]
	return isReserved
}

var javaReservedWords = map[string]bool{
	"abstract":     true,
	"continue":     true,
	"for":          true,
	"new":          true,
	"switch":       true,
	"assert":       true,
	"default":      true,
	"goto":         true,
	"package":      true,
	"synchronized": true,
	"boolean":      true,
	"do":           true,
	"if":           true,
	"private":      true,
	"this":         true,
	"break":        true,
	"double":       true,
	"implements":   true,
	"protected":    true,
	"throw":        true,
	"byte":         true,
	"else":         true,
	"import":       true,
	"public":       true,
	"throws":       true,
	"case":         true,
	"enum":         true,
	"instanceof":   true,
	"return":       true,
	"transient":    true,
	"catch":        true,
	"extends":      true,
	"int":          true,
	"short":        true,
	"try":          true,
	"char":         true,
	"final":        true,
	"interface":    true,
	"static":       true,
	"void":         true,
	"class":        true,
	"finally":      true,
	"long":         true,
	"strictfp":     true,
	"volatile":     true,
	"const":        true,
	"float":        true,
	"native":       true,
	"super":        true,
	"while":        true,
}

func reservedWordGo(ident string) bool {
	_, isReserved := goReservedWords[ident]
	return isReserved
}

var goReservedWords = map[string]bool{
	"break":       true,
	"default":     true,
	"func":        true,
	"interface":   true,
	"select":      true,
	"case":        true,
	"defer":       true,
	"go":          true,
	"map":         true,
	"struct":      true,
	"chan":        true,
	"else":        true,
	"goto":        true,
	"package":     true,
	"switch":      true,
	"const":       true,
	"fallthrough": true,
	"if":          true,
	"range":       true,
	"type":        true,
	"continue":    true,
	"for":         true,
	"import":      true,
	"return":      true,
	"var":         true,
}

func reservedWordJavascript(ident string) bool {
	_, isReserved := javascriptReservedWords[util.ToCamelCase(ident)]
	return isReserved
}

var javascriptReservedWords = map[string]bool{
	"break":    true,
	"case":     true,
	"catch":    true,
	"continue": true,
	"debugger": true,
	"default":  true,
	//"delete":     true, // TODO(bprosnitz) Look into adding this back. This conflicts with Delete() on Content in repository.vdl.
	"do":       true,
	"else":     true,
	"finally":  true,
	"for":      true,
	"function": true,
	"if":       true,
	//"in":         true, // TODO(bprosnitz) Look into addint this back. It conflicts with In in access/service.vdl.
	"instanceof": true,
	"new":        true,
	"return":     true,
	"switch":     true,
	"this":       true,
	"throw":      true,
	"try":        true,
	"typeof":     true,
	"var":        true,
	"void":       true,
	"while":      true,
	"with":       true,
}
