package javascript

import (
	"testing"

	"v.io/v23/i18n"
	"v.io/v23/vdl"
	"v.io/v23/vdl/compile"
)

func TestError(t *testing.T) {
	e := &compile.ErrorDef{
		NamePos: compile.NamePos{
			Name: "Test",
		},
		ID:        "v.io/v23/vdl/codegen/javascript.Test",
		RetryCode: vdl.WireRetryCodeNoRetry,
		Params: []*compile.Field{
			&compile.Field{
				NamePos: compile.NamePos{
					Name: "x",
				},
				Type: vdl.BoolType,
			},
			&compile.Field{
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
	expected := `module.exports.TestError = makeError('v.io/v23/vdl/codegen/javascript.Test', actions.NO_RETRY, {
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
