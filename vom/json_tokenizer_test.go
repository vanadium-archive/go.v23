package vom

import (
	"io"
	"reflect"
	"strings"
	"testing"
)

// Test json tokenization tests the tokenization of various json inputs
func TestJSONTokenization(t *testing.T) {
	tests := []struct {
		input  string
		output []token
		isErr  bool
	}{
		{"", []token{}, false},
		{"{", []token{*tokStartObj}, false},
		{"}", []token{*tokEndObj}, false},
		{":", []token{*tokColon}, false},
		{",", []token{*tokComma}, false},
		{"[", []token{*tokStartArr}, false},
		{"]", []token{*tokEndArr}, false},
		{"true", []token{*tokTrueVal}, false},
		{"TRUE", []token{}, true},
		{"tRUE", []token{}, true},
		{"false", []token{*tokFalseVal}, false},
		{"null", []token{*tokNullVal}, false},
		{`""`, []token{token{jsonStringVal, ""}}, false},
		{`"teststr"`, []token{token{jsonStringVal, "teststr"}}, false},
		{"0", []token{token{jsonNumberVal, "0"}}, false},
		{"100", []token{token{jsonNumberVal, "100"}}, false},
		{"-20", []token{token{jsonNumberVal, "-20"}}, false},
		{"1e-4", []token{token{jsonNumberVal, "1e-4"}}, false},
		{"4.5e+4", []token{token{jsonNumberVal, "4.5e+4"}}, false},
		{"10e3", []token{}, true},
		{"10e+", []token{}, true},
		{"+100.4", []token{}, true},
		{` {
			"A"		:  "B"}`, []token{
			*tokStartObj,
			token{jsonStringVal, "A"},
			*tokColon,
			token{jsonStringVal, "B"},
			*tokEndObj}, false},
		{"%", []token{}, true},
	}
	for _, test := range tests {
		tzr := newJSONTokenizer(strings.NewReader(test.input))

		outputTokens := []*token{}
		var err error
		for {
			var tok *token
			if tok, err = tzr.Next(); err != nil {
				break
			}
			outputTokens = append(outputTokens, tok)
		}

		if err != io.EOF {
			if !test.isErr {
				t.Errorf("error while tokenizing '%v': ", test.input, err)
			}
			continue
		}

		if test.isErr {
			t.Errorf("expected tokenization of %v to error", test.input)
		}

		if reflect.DeepEqual(test.output, outputTokens) {
			t.Errorf("for input %q unexpected token set: %v. expected: %v\n", test.input, test.output, outputTokens)
		}
	}
}

// TestJSONTokenizationMethods tests the methods of the tokenizer
// e.g. Peek, Next, ConsumePeeked
func TestJSONTokenizationMethods(t *testing.T) {
	tzr := newJSONTokenizer(strings.NewReader("true false 5 8"))

	// peek 'true'
	tru, err := tzr.Peek(0)
	if err != nil {
		t.Fatal("error in peek token: ", err)
	}
	if tru.typ != jsonTrueVal {
		t.Errorf("expected peeked token to be 'true'. It was: %v", tru)
	}

	// next 'true'
	nxt, err := tzr.Next()
	if err != nil {
		t.Fatal("error in next token: ", err)
	}
	if tru != nxt {
		t.Errorf("tokens didn't match %v and %v", tru, nxt)
	}

	// peek further to 5
	fiv, err := tzr.Peek(1)
	if err != nil {
		t.Fatal("error in peek token: ", err)
	}
	if fiv.typ != jsonNumberVal || fiv.val != "5" {
		t.Errorf("unexpected token. Got: %v", fiv)
	}

	tzr.ConsumePeeked()

	// next token should be 5
	fiv2, err := tzr.Next()
	if err != nil {
		t.Fatal("error reading 5 with next token: ", err)
	}
	if fiv2 != fiv {
		t.Errorf("tokens unexpectedly differ: %v and %v", tru, nxt)
	}
}
