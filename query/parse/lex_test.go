package parse

import (
	"math/big"
	"reflect"
	"regexp"
	"testing"
)

type token2 struct {
	tokType int
	yys     yySymType
}

type lexTest struct {
	query  string
	tokens []token2
}

func TestLexing(t *testing.T) {
	tests := []lexTest{
		// Identifiers.
		{"a", []token2{{tIDENT, yySymType{strpos: strPos{"a", Pos{1, 1}}}}}},
		{"a0", []token2{{tIDENT, yySymType{strpos: strPos{"a0", Pos{1, 1}}}}}},
		{"abc123", []token2{{tIDENT, yySymType{strpos: strPos{"abc123", Pos{1, 1}}}}}},
		{"ABC123", []token2{{tIDENT, yySymType{strpos: strPos{"ABC123", Pos{1, 1}}}}}},
		{"_", []token2{{tIDENT, yySymType{strpos: strPos{"_", Pos{1, 1}}}}}},
		{"foo_bar", []token2{{tIDENT, yySymType{strpos: strPos{"foo_bar", Pos{1, 1}}}}}},
		{"foobar_", []token2{{tIDENT, yySymType{strpos: strPos{"foobar_", Pos{1, 1}}}}}},
		{"_foobar", []token2{{tIDENT, yySymType{strpos: strPos{"_foobar", Pos{1, 1}}}}}},
		{"äöü", []token2{{tIDENT, yySymType{strpos: strPos{"äöü", Pos{1, 1}}}}}},
		{"本", []token2{{tIDENT, yySymType{strpos: strPos{"本", Pos{1, 1}}}}}},
		{"a۰۱۸", []token2{{tIDENT, yySymType{strpos: strPos{"a۰۱۸", Pos{1, 1}}}}}},
		{"foo६४", []token2{{tIDENT, yySymType{strpos: strPos{"foo६४", Pos{1, 1}}}}}},
		// Skip whitespace.
		{" foo", []token2{{tIDENT, yySymType{strpos: strPos{"foo", Pos{1, 2}}}}}},
		{"\tfoo", []token2{{tIDENT, yySymType{strpos: strPos{"foo", Pos{1, 2}}}}}},
		{"\rfoo", []token2{{tIDENT, yySymType{strpos: strPos{"foo", Pos{1, 2}}}}}},
		{"\nfoo", []token2{{tIDENT, yySymType{strpos: strPos{"foo", Pos{2, 1}}}}}},
		{"\n  \r\n  \t  foo  \t", []token2{{tIDENT, yySymType{strpos: strPos{"foo", Pos{3, 6}}}}}},

		// Punctuation/whitespace should end an identifier.
		{
			"foo.bar",
			[]token2{
				{tIDENT, yySymType{strpos: strPos{"foo", Pos{1, 1}}}},
				{'.', yySymType{pos: Pos{1, 4}}},
				{tIDENT, yySymType{strpos: strPos{"bar", Pos{1, 5}}}},
			},
		},
		{
			"foo bar",
			[]token2{
				{tIDENT, yySymType{strpos: strPos{"foo", Pos{1, 1}}}},
				{tIDENT, yySymType{strpos: strPos{"bar", Pos{1, 5}}}},
			},
		},
		{
			"foo\tbar",
			[]token2{
				{tIDENT, yySymType{strpos: strPos{"foo", Pos{1, 1}}}},
				{tIDENT, yySymType{strpos: strPos{"bar", Pos{1, 5}}}},
			},
		},

		// Decimal ints:
		{"1", []token2{{tINTLIT, yySymType{intpos: intPos{big.NewInt(1), Pos{1, 1}}}}}},
		{"42", []token2{{tINTLIT, yySymType{intpos: intPos{big.NewInt(42), Pos{1, 1}}}}}},
		{"1234567890", []token2{{tINTLIT, yySymType{intpos: intPos{big.NewInt(1234567890), Pos{1, 1}}}}}},

		// Octal ints:
		{"00", []token2{{tINTLIT, yySymType{intpos: intPos{big.NewInt(0), Pos{1, 1}}}}}},
		{"01", []token2{{tINTLIT, yySymType{intpos: intPos{big.NewInt(1), Pos{1, 1}}}}}},
		{"07", []token2{{tINTLIT, yySymType{intpos: intPos{big.NewInt(7), Pos{1, 1}}}}}},
		{"042", []token2{{tINTLIT, yySymType{intpos: intPos{big.NewInt(34), Pos{1, 1}}}}}},
		{"01234567", []token2{{tINTLIT, yySymType{intpos: intPos{big.NewInt(342391), Pos{1, 1}}}}}},

		// Hexadecimal ints:
		{"0x0", []token2{{tINTLIT, yySymType{intpos: intPos{big.NewInt(0), Pos{1, 1}}}}}},
		{"0x1", []token2{{tINTLIT, yySymType{intpos: intPos{big.NewInt(1), Pos{1, 1}}}}}},
		{"0xf", []token2{{tINTLIT, yySymType{intpos: intPos{big.NewInt(15), Pos{1, 1}}}}}},
		{"0x42", []token2{{tINTLIT, yySymType{intpos: intPos{big.NewInt(66), Pos{1, 1}}}}}},
		{"0x123456789", []token2{{tINTLIT, yySymType{intpos: intPos{big.NewInt(4886718345), Pos{1, 1}}}}}},
		{"0xabcDEF", []token2{{tINTLIT, yySymType{intpos: intPos{big.NewInt(11259375), Pos{1, 1}}}}}},
		{"0X1", []token2{{tINTLIT, yySymType{intpos: intPos{big.NewInt(1), Pos{1, 1}}}}}},

		// Floats:
		{"0.", []token2{{tRATLIT, yySymType{ratpos: ratPos{new(big.Rat).SetFloat64(0.), Pos{1, 1}}}}}},
		{"1.", []token2{{tRATLIT, yySymType{ratpos: ratPos{new(big.Rat).SetFloat64(1.), Pos{1, 1}}}}}},
		{"42.", []token2{{tRATLIT, yySymType{ratpos: ratPos{new(big.Rat).SetFloat64(42.), Pos{1, 1}}}}}},
		{"01234567890.", []token2{{tRATLIT, yySymType{ratpos: ratPos{new(big.Rat).SetFloat64(1234567890.), Pos{1, 1}}}}}},
		{".0", []token2{{tRATLIT, yySymType{ratpos: ratPos{new(big.Rat).SetFloat64(.0), Pos{1, 1}}}}}},
		{".1", []token2{{tRATLIT, yySymType{ratpos: ratPos{new(big.Rat).SetFloat64(.1), Pos{1, 1}}}}}},

		{".42", []token2{{tRATLIT, yySymType{ratpos: ratPos{new(big.Rat).SetFloat64(0.42), Pos{1, 1}}}}}},
		{".0123456789", []token2{{tRATLIT, yySymType{ratpos: ratPos{new(big.Rat).SetFloat64(0.0123456789), Pos{1, 1}}}}}},
		{"0.0", []token2{{tRATLIT, yySymType{ratpos: ratPos{new(big.Rat).SetFloat64(0.0), Pos{1, 1}}}}}},
		{"1.0", []token2{{tRATLIT, yySymType{ratpos: ratPos{new(big.Rat).SetFloat64(1.0), Pos{1, 1}}}}}},
		{"42.0", []token2{{tRATLIT, yySymType{ratpos: ratPos{new(big.Rat).SetFloat64(42.0), Pos{1, 1}}}}}},
		{"01234567890.0", []token2{{tRATLIT, yySymType{ratpos: ratPos{new(big.Rat).SetFloat64(1234567890.0), Pos{1, 1}}}}}},

		// Strings:
		{`"foo"`, []token2{{tSTRLIT, yySymType{strpos: strPos{"foo", Pos{1, 1}}}}}},
		{`'foo'`, []token2{{tSTRLIT, yySymType{strpos: strPos{"foo", Pos{1, 1}}}}}},
		{"`foo`", []token2{{tSTRLIT, yySymType{strpos: strPos{"foo", Pos{1, 1}}}}}},
		{`"本"`, []token2{{tSTRLIT, yySymType{strpos: strPos{"本", Pos{1, 1}}}}}},
		{`"\a"`, []token2{{tSTRLIT, yySymType{strpos: strPos{"\a", Pos{1, 1}}}}}},
		{`"\n"`, []token2{{tSTRLIT, yySymType{strpos: strPos{"\n", Pos{1, 1}}}}}},
		{`"\000"`, []token2{{tSTRLIT, yySymType{strpos: strPos{"\000", Pos{1, 1}}}}}},
		{`"\x00"`, []token2{{tSTRLIT, yySymType{strpos: strPos{"\x00", Pos{1, 1}}}}}},
		{`"\xff"`, []token2{{tSTRLIT, yySymType{strpos: strPos{"\xff", Pos{1, 1}}}}}},
		{`"\u0000"`, []token2{{tSTRLIT, yySymType{strpos: strPos{"\u0000", Pos{1, 1}}}}}},
		{`"\ufA16"`, []token2{{tSTRLIT, yySymType{strpos: strPos{"\ufA16", Pos{1, 1}}}}}},
		{`"\U00000000"`, []token2{{tSTRLIT, yySymType{strpos: strPos{"\U00000000", Pos{1, 1}}}}}},
		{`"\U0000ffAB"`, []token2{{tSTRLIT, yySymType{strpos: strPos{"\U0000ffAB", Pos{1, 1}}}}}},
		// Raw strings should not be unescaped.  The test's input string
		// is escaped because it is in double quotes.
		{"`\\n`", []token2{{tSTRLIT, yySymType{strpos: strPos{`\n`, Pos{1, 1}}}}}},

		// Multi-rune characters:
		{`<`, []token2{{'<', yySymType{pos: Pos{1, 1}}}}},
		{`<=`, []token2{{tLE, yySymType{pos: Pos{1, 1}}}}},
		{`.`, []token2{{'.', yySymType{pos: Pos{1, 1}}}}},

		// Keywords:
		{`true`, []token2{{tTRUE, yySymType{pos: Pos{1, 1}}}}},
		{`false`, []token2{{tFALSE, yySymType{pos: Pos{1, 1}}}}},
		{`TRUE`, []token2{{tTRUE, yySymType{pos: Pos{1, 1}}}}},
		{`TrUe`, []token2{{tTRUE, yySymType{pos: Pos{1, 1}}}}},
		{`trueish`, []token2{{tIDENT, yySymType{strpos: strPos{"trueish", Pos{1, 1}}}}}},
	}
	for _, test := range tests {
		l := newLexer(test.query)
		for i, tok := range test.tokens {
			var yys yySymType
			tokType := l.Lex(&yys)
			if l.err != nil {
				t.Errorf("query: %s; error: %s", test.query, l.err)
				break
			}
			if tokType != tok.tokType {
				t.Errorf("query: %s, token: %d; got tokType: %d (%c), want: %d", test.query, i, tokType, rune(tokType), tok.tokType)
				break
			}
			// We can't compare floats with reflect.DeepEqual() since
			// different parsers produce different representations.
			if tokType == tRATLIT {
				got, _ := yys.ratpos.rat.Float64()
				want, _ := tok.yys.ratpos.rat.Float64()
				if got != want {
					t.Errorf("query: %s, token: %d; got: %v, want: %v", test.query, i, got, want)
					break
				}
				if !reflect.DeepEqual(yys.ratpos.pos, tok.yys.ratpos.pos) {
					t.Errorf("query: %s, token: %d;\ngot\t: %+v\nwant\t%+v", test.query, i, yys.ratpos.pos, tok.yys.ratpos.pos)
					break
				}
			} else {
				if !reflect.DeepEqual(yys, tok.yys) {
					t.Errorf("query: %s, token: %d;\ngot\t: %+v\nwant\t%+v", test.query, i, yys, tok.yys)
					break
				}
			}
		}
	}
}

type errorTest struct {
	query string
	errRE string // Expected error message.
}

func TestError(t *testing.T) {
	errors := []errorTest{
		{`"foo`, "unterminated string"},
		{`'foo` + "\n' bar", "unterminated string"},
		{`0xabc.def`, `Couldn't convert token \[0xabc.def\] to number`},

		// Bad encodings.
		{"\x00", "unexpected NULL"},
		{"\x80", "illegal UTF-8 encoding"},
		{"\xff", "illegal UTF-8 encoding"},
		{"a\x00", "unexpected NULL"},
		{"ab\x80", "illegal UTF-8 encoding"},
		{"abc\xff", "illegal UTF-8 encoding"},
		{`"a` + "\x00\"", "unexpected NULL"},
		{`"ab` + "\x80\"", "illegal UTF-8 encoding"},
		{`"abc` + "\xff\"", "illegal UTF-8 encoding"},
	}
	for _, test := range errors {
		l := newLexer(test.query)
		var yys yySymType
		l.Lex(&yys)
		if l.err == nil {
			t.Errorf("No error for query: %s", test.query)
			continue
		}
		re := regexp.MustCompile(test.errRE)
		if !re.MatchString(l.err.Error()) {
			t.Errorf("Could not find regexp '%s' in error: %s", test.errRE, l.err.Error())
			continue
		}
	}
}
