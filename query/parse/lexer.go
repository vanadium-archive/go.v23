package parse

// This lexer is heavily based on the lexers in the standard libarary
// text/scanner/scanner.go and text/template/parse/lex.go.  We wrote our own
// because text/scanner/scanner.go parses Go code and doesn't exactly do
// what we want.  For example, it requires that single quote means character,
// while we want it to mean string.

import (
	"container/list"
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"text/scanner"
	"unicode"
	"unicode/utf8"
)

const yaccEOF = 0 // yacc interprets 0 as the end-of-file marker

func init() {
	// yyDebug is defined in the yacc-generated grammar.go file.  Setting it to 1
	// only produces output on syntax errors; set it to 4 to generate full debug
	// output.  Sadly yacc doesn't give position information describing the error.
	yyDebug = 1
}

// maxBackup is the maximum number of times that backup can be called
// in a row.  It limits the depth of the lexer2.prevWidths stack.
const maxBackup = 2

// lexer implements the yyLexer interface for the yacc-generated parser.
//
// An oddity: lexer also holds the result of the parse.  Most yacc examples hold
// parse results in package-scoped (global) variables, but doing that would mean
// we wouldn't be able to run separate parses concurrently.  To enable that we'd
// need each invocation of yyParse to mutate its own result, but unfortunately
// the Go yacc tool doesn't provide any way to pass extra arguments to yyParse.
//
// So we cheat and hold the parse result in the lexer, and in the yacc rules we
// call lexASTResult(yylex) to convert from the yyLexer interface back to the
// concrete lexer type, and retrieve a pointer to the parse result.
type lexer struct {
	input      string // The query to be parsed.
	cursor     int    // The zero-based offset of the next rune to be parsed.
	cursorLine int    // The one-based line number of the cursor.
	cursorCol  int    // The one-based column number of the cursor.
	tokStart   int    // The zero-based offset of the token being parsed.
	tokID      int    // The yacc token ID of the token just parsed.
	tokLine    int    // The one-based line number of the token.
	tokCol     int    // The one-based column number of the token.

	// prevWidths is a stack (front is the top of the stack)
	// of previously parsed runes.  This allows us to peek/backup
	// multiple times.
	prevWidths list.List

	// lval is passed to each call to Lex.  When a token is successfully
	// parsed, the parsing method populates lval appropriately.
	lval *yySymType

	// err records the last error encountered by the parser.
	err error
	// pipeline holds the result of the parse.
	pipeline Pipeline
}

// newLexer returns a new lexer from the given input.
func newLexer(input string) *lexer {
	l := &lexer{input: input, cursorLine: 1, cursorCol: 1}
	l.prevWidths.Init()
	return l
}

// Error is part of the yyLexer interface, called by the yacc-generated parser.
// Unfortunately yacc doesn't give good error information - we dump the position
// of the previous scanned token as an approximation of where the error is.
func (l *lexer) Error(s string) {
	if s == "syntax error" {
		switch l.tokID {
		case scanner.EOF:
			s = "unexpected EOF"
		default:
			s = fmt.Sprintf("syntax error at token '%s'", l.tokText())
		}
	}
	l.posErrorf(l.tokPos(), "%s", s)
}

// posErrorf generates an error with file and pos info.
func (l *lexer) posErrorf(pos Pos, format string, v ...interface{}) {
	// Report the first error encountered.
	if l.err == nil {
		l.err = fmt.Errorf(pos.String()+": "+format, v...)
		l.tokID = yaccEOF
	}
}

// Lex implements the yyLexer interface.  Each call produces a single
// token.  yySymType is a type generated from the "%union" in
// grammar.y.  Lex populates lval and returns the yacc identifier for
// the token.
func (l *lexer) Lex(lval *yySymType) int {
	// Stop lexing after hitting the first error.
	if l.err != nil {
		return yaccEOF
	}
	l.lval = lval
	l.scan()
	l.lval = nil
	return l.tokID
}

// next extracts the rune at the current cursor position and then advances the
// cursor.
func (l *lexer) next() rune {
	if l.cursor >= len(l.input) {
		return rune(yaccEOF)
	}
	r, w := utf8.DecodeRuneInString(l.input[l.cursor:])
	if r == 0 {
		l.posErrorf(l.tokPos(), "unexpected NULL")
		return rune(yaccEOF)
	}
	if r == utf8.RuneError {
		l.posErrorf(l.tokPos(), "illegal UTF-8 encoding")
		return rune(yaccEOF)
	}
	l.prevWidths.PushFront(w)
	if l.prevWidths.Len() > maxBackup {
		l.prevWidths.Remove(l.prevWidths.Back())
	}
	l.cursor += w
	l.cursorCol++
	return r
}

// peek extracts the rune at the current cursor position without advancing the
// cursor.
func (l *lexer) peek() rune {
	r := l.next()
	// Don't backup if next() didn't advance the cursor.
	if int(r) != yaccEOF {
		l.backup()
	}
	return r
}

// backup moves the cursor back one rune.  You can't call this more than
// maxBackup times in a row.
func (l *lexer) backup() {
	if l.prevWidths.Len() == 0 {
		panic("can't backup any further")
	}
	l.cursor -= l.prevWidths.Remove(l.prevWidths.Front()).(int)
	l.cursorCol--
}

// accept consumes the next rune if it's from the valid set.
func (l *lexer) accept(valid string) bool {
	ch := l.next()
	if strings.IndexRune(valid, ch) >= 0 {
		return true
	}
	if ch > 0 {
		l.backup()
	}
	return false
}

// acceptRun consumes a run of runes from the valid set.
func (l *lexer) acceptRun(valid string) {
	var ch rune
	for ch = l.next(); strings.IndexRune(valid, ch) >= 0; ch = l.next() {
	}
	if ch > 0 {
		l.backup()
	}
}

// tokText returns the text of the current token.
func (l *lexer) tokText() string {
	return l.input[l.tokStart:l.cursor]
}

// tokPos returns the position of the current token.
func (l *lexer) tokPos() Pos {
	return Pos{l.tokLine, l.tokCol}
}

// multiRune specifies the second rune in a 2-rune token.
type multiRune struct {
	// r is the second rune in a 2-rune token.
	r rune
	// id is the yacc ID of the 2-rune token.
	id int
}

var multiRunes = map[rune]multiRune{
	'=': {'=', tEQEQ},
	'<': {'=', tLE},
	'>': {'=', tGE},
	'|': {'|', tOROR},
	'&': {'&', tANDAND},
	'!': {'=', tNE},
	'.': {'.', tDOTDOT},
}

// scan extracts a single token from l.input and stores it in l.lval.
func (l *lexer) scan() {
	var ch rune
whitespace:
	for {
		l.tokStart = l.cursor
		l.tokLine = l.cursorLine
		l.tokCol = l.cursorCol
		ch = l.next()
		switch ch {
		case ' ', '\t', '\r':
			break
		case '\n':
			l.cursorLine++
			l.cursorCol = 1
		default:
			break whitespace
		}
	}
	switch {
	case ch == '_' || unicode.IsLetter(ch):
		l.scanIdentifier()
	case isDecimal(ch):
		l.scanNumber(ch)
	default:
		switch ch {
		case '"', '\'', '`':
			l.scanString(ch)
		case '.':
			if isDecimal(l.peek()) {
				l.scanNumber(ch)
				return
			}
			fallthrough
		default:
			l.lval.pos = l.tokPos()
			// If the rune starts one of the multi-rune combos in our language, peek
			// ahead to try to match that combo.  2-rune tokens always take precendence
			// over 1-rune tokens.
			if m, ok := multiRunes[ch]; ok {
				if l.peek() == m.r {
					l.next()
					l.tokID = m.id
					return
				}
			}
			l.tokID = int(ch)
		}
	}
}

// scanString extracts a single string literal.  quote is the rune used
// to start the string (e.g. ' or ").
func (l *lexer) scanString(quote rune) {
	for ch := l.next(); ch != quote; ch = l.next() {
		if ch == '\\' {
			ch = l.next()
		}
		if ch == '\n' || ch <= 0 {
			l.Error("unterminated string")
			return
		}
	}
	tokText := l.tokText()
	// If the string was quoted with single quote, replace that with double
	// quotes so strconv.Unquote will treat it as a string, not a character.
	if tokText[0] == '\'' {
		tokText = strings.Join([]string{"\"", tokText[1 : len(tokText)-1], "\""}, "")
	}
	l.tokID = tSTRLIT
	l.lval.strpos.pos = l.tokPos()
	var err error
	l.lval.strpos.str, err = strconv.Unquote(tokText)
	if err != nil {
		l.posErrorf(l.tokPos(), "Couldn't convert token [%v] to string literal", tokText)
		return
	}
}

// scanNumber extracts a single number literal (int or float).  ch is the
// first character in the number.
func (l *lexer) scanNumber(ch rune) {
	digits := "0123456789"

	if ch == '0' && l.accept("xX") {
		digits = "0123456789abcdefABCDEF"
	}
	l.acceptRun(digits)
	l.accept(".")
	l.acceptRun(digits)
	// TODO(kash): Handle numbers like "1e7".

	tokText := l.tokText()

	// Attempt to convert to an int.  If that doesn't work, convert to a float.
	i := new(big.Int)
	if _, ok := i.SetString(tokText, 0); ok {
		l.tokID = tINTLIT
		l.lval.intpos = intPos{i, l.tokPos()}
		return
	}
	r := new(big.Rat)
	if _, ok := r.SetString(tokText); ok {
		l.tokID = tRATLIT
		l.lval.ratpos = ratPos{r, l.tokPos()}
		return
	}
	l.posErrorf(l.tokPos(), "Couldn't convert token [%v] to number", tokText)
}

var keywords = map[string]int{
	"true":   tTRUE,
	"false":  tFALSE,
	"type":   tTYPE,
	"as":     tAS,
	"hidden": tHIDDEN,
}

// scanIdentifier extracts a single identifier.
func (l *lexer) scanIdentifier() {
	ch := l.next()
	for ch == '_' || unicode.IsLetter(ch) || unicode.IsDigit(ch) {
		ch = l.next()
	}
	if ch != rune(yaccEOF) {
		l.backup()
	}
	// Either the identifier is a known keyword, or we pass it through as IDENT.
	if keytok, ok := keywords[strings.ToLower(l.tokText())]; ok {
		l.tokID = keytok
		l.lval.pos = l.tokPos()
		return
	}
	l.tokID = tIDENT
	l.lval.strpos = strPos{l.tokText(), l.tokPos()}
}

func isDecimal(ch rune) bool {
	return '0' <= ch && ch <= '9'
}

// The lex* functions below all convert the yyLexer input arg into a concrete
// lexer as their first step.  The type conversion is always safe since we're
// the ones who called yyParse, and thus know the concrete type is always lexer.

// lexAbort records an error, s.  This has the side-effect of forcing
// the lexer to stop.
func lexAbort(yylex yyLexer, s string) {
	yylex.(*lexer).Error(s)
}

// lexASTResult stores the result of the parse, <pipeline>, in the lexer.
func lexASTResult(yylex yyLexer, pipeline Pipeline) {
	yylex.(*lexer).pipeline = pipeline
}
