package parse

import (
	"fmt"

	"veyron2/query"
)

// Parse transforms a Query into an AST.  If there is an error in parsing, it
// is returned.
func Parse(q query.Query) (Pipeline, error) {
	lex := newLexer(q.Stmt)
	if errCode := yyParse(lex); errCode != 0 {
		if lex.err != nil {
			return nil, lex.err
		}
		return nil, fmt.Errorf("yyParse returned error code %v", errCode)
	}
	return lex.pipeline, lex.err
}
