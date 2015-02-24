package compile

import (
	"v.io/v23/vdl/vdlutil"
)

// ReservedMode indicates which mode to perform reserved-word checking:
//   ReservedNormal    - Check the given identifier.
//   ReservedCamelCase - Check the given identifier in lower-camel-case.
type ReservedMode int

const (
	ReservedNormal ReservedMode = iota
	ReservedCamelCase
)

// reservedWord checks if identifiers are reserved after they are converted to the native form for the language.
func reservedWord(ident string, mode ReservedMode) bool {
	return reservedWordJava(ident, mode) ||
		reservedWordJavascript(ident, mode) ||
		reservedWordGo(ident)
	// TODO(bprosnitz) Other identifiers? (set, assert, raise, with, etc)
}

func reservedWordJava(ident string, mode ReservedMode) bool {
	if mode == ReservedCamelCase {
		ident = vdlutil.ToCamelCase(ident)
	}
	_, isReserved := javaReservedWords[ident]
	return isReserved
}

var javaReservedWords = map[string]bool{
	"abstract":     true,
	"assert":       true,
	"boolean":      true,
	"break":        true,
	"byte":         true,
	"case":         true,
	"catch":        true,
	"char":         true,
	"class":        true,
	"const":        true,
	"continue":     true,
	"default":      true,
	"do":           true,
	"double":       true,
	"else":         true,
	"enum":         true,
	"extends":      true,
	"final":        true,
	"finally":      true,
	"float":        true,
	"for":          true,
	"goto":         true,
	"if":           true,
	"implements":   true,
	"import":       true,
	"instanceof":   true,
	"int":          true,
	"interface":    true,
	"long":         true,
	"native":       true,
	"new":          true,
	"package":      true,
	"private":      true,
	"protected":    true,
	"public":       true,
	"return":       true,
	"short":        true,
	"static":       true,
	"strictfp":     true,
	"super":        true,
	"switch":       true,
	"synchronized": true,
	"this":         true,
	"throw":        true,
	"throws":       true,
	"transient":    true,
	"try":          true,
	"void":         true,
	"volatile":     true,
	"while":        true,
}

func reservedWordGo(ident string) bool {
	_, isReserved := goReservedWords[ident]
	return isReserved
}

var goReservedWords = map[string]bool{
	"break":       true,
	"case":        true,
	"chan":        true,
	"const":       true,
	"continue":    true,
	"default":     true,
	"defer":       true,
	"else":        true,
	"fallthrough": true,
	"for":         true,
	"func":        true,
	"go":          true,
	"goto":        true,
	"if":          true,
	"import":      true,
	"interface":   true,
	"map":         true,
	"package":     true,
	"range":       true,
	"return":      true,
	"select":      true,
	"struct":      true,
	"switch":      true,
	"type":        true,
	"var":         true,
}

func reservedWordJavascript(ident string, mode ReservedMode) bool {
	if mode == ReservedCamelCase {
		ident = vdlutil.ToCamelCase(ident)
	}
	_, isReserved := javascriptReservedWords[ident]
	return isReserved
}

var javascriptReservedWords = map[string]bool{
	"break":    true,
	"case":     true,
	"catch":    true,
	"continue": true,
	"debugger": true,
	"default":  true,
	//"delete":     true, // TODO(bprosnitz) Look into adding this back. This conflicts with Delete() on Content in repository.vdlutil.
	"do":       true,
	"else":     true,
	"finally":  true,
	"for":      true,
	"function": true,
	"if":       true,
	//"in":         true, // TODO(bprosnitz) Look into addint this back. It conflicts with In in access/service.vdlutil.
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
