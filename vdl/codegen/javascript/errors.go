package javascript

import (
	"fmt"

	"v.io/v23/vdl/compile"
	"v.io/v23/vdl/vdlutil"
)

func generateErrorConstructor(names typeNames, e *compile.ErrorDef) string {
	name := e.Name + "Error"
	result := fmt.Sprintf("module.exports.%s = makeError('%s', actions.%s, ", name, e.ID, vdlutil.ToConstCase(e.RetryCode.String()))
	result += "{\n"
	for _, f := range e.Formats {
		result += fmt.Sprintf("  '%s': '%s',\n", f.Lang, f.Fmt)
	}
	result += "}, [\n"
	for _, param := range e.Params {
		result += "  " + names.LookupType(param.Type) + ",\n"
	}
	return result + "]);\n"
}
