package javascript

import (
	"fmt"
	"v.io/core/veyron2/vdl/compile"
	"v.io/core/veyron2/vdl/vdlutil"
)

func generateErrorConstructor(names typeNames, e *compile.ErrorDef) string {
	name := e.Name
	result := fmt.Sprintf("module.exports.%s = makeError('%s', actions.%s, ", name, e.ID, vdlutil.ToConstCase(e.Action.String()))
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
