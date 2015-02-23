package javascript

import (
	"testing"

	"v.io/core/veyron2/i18n"
	"v.io/core/veyron2/vdl"
	"v.io/core/veyron2/vdl/compile"
	"v.io/core/veyron2/verror"
)

func TestError(t *testing.T) {
	e := &compile.ErrorDef{
		NamePos: compile.NamePos{
			Name: "Test",
		},
		ID:     verror.ID("v.io/core/veyron2/vdl/codegen/javascript.Test"),
		Action: verror.NoRetry,
		Params: []*compile.Arg{
			&compile.Arg{
				NamePos: compile.NamePos{
					Name: "x",
				},
				Type: vdl.BoolType,
			},
			&compile.Arg{
				NamePos: compile.NamePos{
					Name: "y",
				},
				Type: vdl.Int32Type,
			},
		},
		Formats: []compile.LangFmt{
			compile.LangFmt{
				Lang: i18n.LangID("en-US"),
				Fmt:  "english string",
			},
			compile.LangFmt{
				Lang: i18n.LangID("fr"),
				Fmt:  "french string",
			},
		},
	}
	var names typeNames
	result := generateErrorConstructor(names, e)
	expected := `module.exports.TestError = makeError('v.io/core/veyron2/vdl/codegen/javascript.Test', actions.NO_RETRY, {
  'en-US': 'english string',
  'fr': 'french string',
}, [
  vdl.Types.BOOL,
  vdl.Types.INT32,
]);
`
	if result != expected {
		t.Errorf("got %s, expected %s", result, expected)
	}
}
