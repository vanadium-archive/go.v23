// Package parse provides utilities to parse vdl files into a parse tree.  The
// Parse function is the main entry point.
package parse

// This is the only file in this package that uses the yacc-generated parser
// with entrypoint yyParse.  The result of the parse is the simple vdl.File
// representation, which is used by the compilation and code-generation stages.
//
// TODO(toddw): The yacc-generated parser returns pretty lousy error messages;
// basically "syntax error" is the only string returned.  Improve them.
import (
	"fmt"
	"io"
	"log"
	"math/big"
	"strconv"
	"strings"
	"text/scanner"

	"veyron2/vdl"
)

// Opts specifies vdl parsing options.
type Opts struct {
	ImportsOnly bool // Only parse imports; skip everything else.
}

// Parse takes an vdl base file name, the contents of the file src, and the
// accumulated errors, and parses the vdl into a parse.File, containing the
// parse tree.  Returns nil if any errors are encountered, with errs containing
// more information.  Otherwise returns the parsed File.
func Parse(baseFileName string, src io.Reader, opts Opts, errs *vdl.Errors) *File {
	if errs == nil {
		log.Fatal("Nil errors specified for Parse")
	}
	startErrs := errs.NumErrors()
	lex := newLexer(baseFileName, src, opts, errs)
	if errCode := yyParse(lex); errCode != 0 {
		errs.Errorf("%s yyParse returned error code %v", baseFileName, errCode)
	}
	lex.attachComments()
	if !opts.ImportsOnly {
		vdl.Vlog.Printf("PARSE RESULTS\n\n%v\n\n", lex.vdlFile)
	}
	if startErrs != errs.NumErrors() {
		return nil
	}
	return lex.vdlFile
}

// lexer implements the yyLexer interface for the yacc-generated parser.
//
// An oddity: lexer also holds the result of the parse.  Most yacc examples hold
// parse results in package-scoped (global) variables, but doing that would mean
// we wouldn't be able to run separate parses concurrently.  To enable that we'd
// need each invocation of yyParse to mutate its own result, but unfortunately
// the Go yacc tool doesn't provide any way to pass extra arguments to yyParse.
//
// So we cheat and hold the parse result in the lexer, and in the yacc rules we
// call lexVDLFile(yylex) to convert from the yyLexer interface back to the
// concrete lexer type, and retrieve a pointer to the parse result.
type lexer struct {
	// Fields for lexing / scanning the input source file.
	scanner scanner.Scanner
	opts    Opts
	errs    *vdl.Errors
	started bool  // Has the dummy start token already been emitted?
	sawEOF  bool  // Have we already seen the end-of-file?
	prevTok token // Previous token, used for auto-semicolons and errors.

	// Fields holding the result of the parse.
	comments commentMap
	vdlFile  *File
}

func newLexer(baseFileName string, src io.Reader, opts Opts, errs *vdl.Errors) *lexer {
	l := &lexer{opts: opts, errs: errs, vdlFile: &File{BaseName: baseFileName}}
	l.comments.init()
	l.scanner.Init(src)
	// Don't produce character literal tokens, but do scan comments.
	l.scanner.Mode = scanner.ScanIdents | scanner.ScanFloats | scanner.ScanStrings | scanner.ScanRawStrings | scanner.ScanComments
	// Don't treat '\n' as whitespace, so we can auto-insert semicolons.
	l.scanner.Whitespace = 1<<'\t' | 1<<'\r' | 1<<' '
	l.scanner.Error = func(s *scanner.Scanner, msg string) {
		l.Error(msg)
	}
	return l
}

type token struct {
	t    rune
	text string
	pos  Pos
}

func (t token) String() string {
	return fmt.Sprintf("%v %U %s", t.pos, t.t, t.text)
}

// The lex* functions below all convert the yyLexer input arg into a concrete
// lexer as their first step.  The type conversion is always safe since we're
// the ones who called yyParse, and thus know the concrete type is always lexer.

// lexVDLFile retrieves the vdl.File parse result from the yyLexer interface.
// This is called in the yacc rules to fill in the parse result.
func lexVDLFile(yylex yyLexer) *File {
	return yylex.(*lexer).vdlFile
}

// lexPosErrorf adds an error with positional information, on a type
// implementing the yyLexer interface.  This is called in the yacc rules to
// throw errors.
func lexPosErrorf(yylex yyLexer, pos Pos, format string, v ...interface{}) {
	yylex.(*lexer).posErrorf(pos, format, v...)
}

// lexGenEOF tells the lexer to generate EOF tokens from now on, as if the end
// of file had been seen.  This is called in the yacc rules to terminate the
// parse even if the file still has tokens.
func lexGenEOF(yylex yyLexer) {
	yylex.(*lexer).sawEOF = true
}

var keywords = map[string]int{
	"package":   tPACKAGE,
	"import":    tIMPORT,
	"type":      tTYPE,
	"enum":      tENUM,
	"set":       tSET,
	"map":       tMAP,
	"struct":    tSTRUCT,
	"oneof":     tONEOF,
	"interface": tINTERFACE,
	"stream":    tSTREAM,
	"const":     tCONST,
	"true":      tTRUE,
	"false":     tFALSE,
	"errorid":   tERRORID,
}

type nextRune struct {
	t  rune
	id int
}

// knownPunct is a map of our known punctuation.  We support 1 and 2 rune
// combinations, where 2 rune combos must be immediately adjacent with no
// intervening whitespace.  The 2-rune combos always take precedence over the
// 1-rune combos.  Every entry is a valid 1-rune combo, which is returned as-is
// without a special token id; the ascii value represents itself.
var knownPunct = map[rune][]nextRune{
	';': nil,
	':': nil,
	',': nil,
	'.': nil,
	'*': nil,
	'(': nil,
	')': nil,
	'[': nil,
	']': nil,
	'{': nil,
	'}': nil,
	'+': nil,
	'-': nil,
	'/': nil,
	'%': nil,
	'^': nil,
	'!': {{'=', tNE}},
	'=': {{'=', tEQEQ}},
	'<': {{'=', tLE}, {'<', tLSH}},
	'>': {{'=', tGE}, {'>', tRSH}},
	'|': {{'|', tOROR}},
	'&': {{'&', tANDAND}},
}

// autoSemi determines whether to automatically add a semicolon, based on the
// rule that semicolons are always added at the end of each line after certain
// tokens.  The Go auto-semicolon rule is described here:
//   http://golang.org/ref/spec#Semicolons
func autoSemi(prevTok token) bool {
	return prevAutoSemi[prevTok.t] && prevTok.pos.IsValid()
}

var prevAutoSemi = map[rune]bool{
	scanner.Ident:     true,
	scanner.Int:       true,
	scanner.Float:     true,
	scanner.String:    true,
	scanner.RawString: true,
	')':               true,
	']':               true,
	'}':               true,
	'>':               true,
}

const yaccEOF int = 0 // yacc interprets 0 as the end-of-file marker

func init() {
	// yyDebug is defined in the yacc-generated grammar.go file.  Setting it to 1
	// only produces output on syntax errors; set it to 4 to generate full debug
	// output.  Sadly yacc doesn't give position information describing the error.
	yyDebug = 1
}

// A note on the comment-tracking strategy.  During lexing we generate
// commentBlocks, defined as a sequence of adjacent or abutting comments (either
// // or /**/) with no intervening tokens.  Adjacent means that the previous
// comment ends on the line immediately before the next one starts, and abutting
// means that the previous comment ends on the same line as the next one starts.
//
// At the end of the parse we try to attach comment blocks to parse tree items.
// We use a heuristic that works for common cases, but isn't perfect - it
// mis-associates some styles of comments, and we don't ensure all comment
// blocks will be associated to an item.

type commentBlock struct {
	text      string
	firstLine int
	lastLine  int
}

// update returns true and adds tok to this block if tok is adjacent or
// abutting, otherwise it returns false without mutating the block.  Since we're
// handling newlines explicitly in the lexer, we never get comment tokens with
// trailing newlines.  We can get embedded newlines via /**/ style comments.
func (cb *commentBlock) update(tok token) bool {
	if cb.text == "" {
		// First update in this block.
		cb.text = tok.text
		cb.firstLine = tok.pos.Line
		cb.lastLine = tok.pos.Line + strings.Count(tok.text, "\n")
		return true
	}
	if cb.lastLine >= tok.pos.Line-1 {
		// The tok is adjacent or abutting.
		if cb.lastLine == tok.pos.Line-1 {
			// The tok is adjacent - need a newline.
			cb.text += "\n"
			cb.lastLine++
		}
		cb.text += tok.text
		cb.lastLine += strings.Count(tok.text, "\n")
		return true
	}
	return false
}

// commentMap keeps track of blocks of comments in a file.  We store comment
// blocks in maps by first line, and by last line.  Note that technically there
// could be more than one commentBlock ending on the same line, due to /**/
// style comments.  We ignore this rare case and just keep the first one.
type commentMap struct {
	byFirst      map[int]commentBlock
	byLast       map[int]commentBlock
	cur          commentBlock
	prevTokenPos Pos
}

func (cm *commentMap) init() {
	cm.byFirst = make(map[int]commentBlock)
	cm.byLast = make(map[int]commentBlock)
}

// addComment adds a comment token to the map, either appending to the current
// block or ending the current block and starting a new one.
func (cm *commentMap) addComment(tok token) {
	if !cm.cur.update(tok) {
		cm.endBlock()
		if !cm.cur.update(tok) {
			panic(fmt.Errorf("vdl: couldn't update current comment block with token %v", tok))
		}
	}
	// Here's an example of why we need the special case endBlock logic.
	//
	//   type Foo struct {
	//     // doc1
	//     A int // doc2
	//     // doc3
	//     B int
	//   }
	//
	// The problem is that without the special-case, we'd group doc2 and doc3
	// together into the same block.  That may actually be correct some times, but
	// it's more common for doc3 to be semantically associated with field B.  Thus
	// if we've already seen any token on the same line as this comment block, we
	// end the block immediately.  This means that comments appearing on the same
	// line as any other token are forced to be a single comment block.
	if cm.prevTokenPos.Line == tok.pos.Line {
		cm.endBlock()
	}
}

func (cm *commentMap) handleToken(tok token) {
	cm.endBlock()
	cm.prevTokenPos = tok.pos
}

// endBlock adds the the current comment block to the map, and resets it in
// preparation for new comments to be added.  In the rare case where we see
// comment blocks that either start or end on the same line, we just keep the
// first comment block that was inserted.
func (cm *commentMap) endBlock() {
	_, inFirst := cm.byFirst[cm.cur.firstLine]
	_, inLast := cm.byLast[cm.cur.lastLine]
	if cm.cur.text != "" && !inFirst && !inLast {
		cm.byFirst[cm.cur.firstLine] = cm.cur
		cm.byLast[cm.cur.lastLine] = cm.cur
	}
	cm.cur.text = ""
	cm.cur.firstLine = 0
	cm.cur.lastLine = 0
}

// getDoc returns the documentation string associated with pos.  Our rule is the
// last line of the documentation must end on the line immediately before pos.
// Once a comment block has been returned it isn't eligible to be attached to
// any other item, and is deleted from the map.
//
// The returned string is either empty, or is newline terminated.
func (cm *commentMap) getDoc(pos Pos) string {
	block := cm.byLast[pos.Line-1]
	if block.text == "" {
		return ""
	}
	doc := block.text + "\n"
	delete(cm.byFirst, block.firstLine)
	delete(cm.byLast, block.lastLine)
	return doc
}

// getDocSuffix returns the suffix documentation associated with pos.  Our rule
// is the first line of the documentation must be on the same line as pos.  Once
// a comment block as been returned it isn't eligible to be attached to any
// other item, and is deleted from the map.
//
// The returned string is either empty, or has a leading space.
func (cm *commentMap) getDocSuffix(pos Pos) string {
	block := cm.byFirst[pos.Line]
	if block.text == "" {
		return ""
	}
	doc := " " + block.text
	delete(cm.byFirst, block.firstLine)
	delete(cm.byLast, block.lastLine)
	return doc
}

func attachTypeComments(t Type, cm *commentMap, suffix bool) {
	switch tu := t.(type) {
	case *TypeEnum:
		for _, label := range tu.Labels {
			if suffix {
				label.DocSuffix = cm.getDocSuffix(label.Pos)
			} else {
				label.Doc = cm.getDoc(label.Pos)
			}
		}
	case *TypeArray:
		attachTypeComments(tu.Elem, cm, suffix)
	case *TypeList:
		attachTypeComments(tu.Elem, cm, suffix)
	case *TypeSet:
		attachTypeComments(tu.Key, cm, suffix)
	case *TypeMap:
		attachTypeComments(tu.Key, cm, suffix)
		attachTypeComments(tu.Elem, cm, suffix)
	case *TypeStruct:
		for _, field := range tu.Fields {
			if suffix {
				field.DocSuffix = cm.getDocSuffix(field.Pos)
			} else {
				field.Doc = cm.getDoc(field.Pos)
			}
			attachTypeComments(field.Type, cm, suffix)
		}
	case *TypeOneOf:
		for _, t := range tu.Types {
			attachTypeComments(t, cm, suffix)
		}
	case *TypeNamed:
		// Terminate the recursion at named types.
	default:
		panic(fmt.Errorf("vdl: unhandled type %#v", t))
	}
}

// attachComments causes all comments collected during the parse to be attached
// to the appropriate parse tree items.  This should only be called after the
// parse has completed.
func (l *lexer) attachComments() {
	f := l.vdlFile
	// First attach all suffix docs - these occur on the same line.
	f.PackageDef.DocSuffix = l.comments.getDocSuffix(f.PackageDef.Pos)
	for _, x := range f.Imports {
		x.DocSuffix = l.comments.getDocSuffix(x.Pos)
	}
	for _, x := range f.ErrorIDs {
		x.DocSuffix = l.comments.getDocSuffix(x.Pos)
	}
	for _, x := range f.TypeDefs {
		x.DocSuffix = l.comments.getDocSuffix(x.Pos)
		attachTypeComments(x.Type, &l.comments, true)
	}
	for _, x := range f.ConstDefs {
		x.DocSuffix = l.comments.getDocSuffix(x.Pos)
	}
	for _, x := range f.Interfaces {
		x.DocSuffix = l.comments.getDocSuffix(x.Pos)
		for _, y := range x.Embeds {
			y.DocSuffix = l.comments.getDocSuffix(y.Pos)
		}
		for _, y := range x.Methods {
			y.DocSuffix = l.comments.getDocSuffix(y.Pos)
		}
	}
	// Now attach the docs - these occur on the line immediately before.
	f.PackageDef.Doc = l.comments.getDoc(f.PackageDef.Pos)
	for _, x := range f.Imports {
		x.Doc = l.comments.getDoc(x.Pos)
	}
	for _, x := range f.ErrorIDs {
		x.Doc = l.comments.getDoc(x.Pos)
	}
	for _, x := range f.TypeDefs {
		x.Doc = l.comments.getDoc(x.Pos)
		attachTypeComments(x.Type, &l.comments, false)
	}
	for _, x := range f.ConstDefs {
		x.Doc = l.comments.getDoc(x.Pos)
	}
	for _, x := range f.Interfaces {
		x.Doc = l.comments.getDoc(x.Pos)
		for _, y := range x.Embeds {
			y.Doc = l.comments.getDoc(y.Pos)
		}
		for _, y := range x.Methods {
			y.Doc = l.comments.getDoc(y.Pos)
		}
	}
}

// nextToken uses the text/scanner package to scan the input for the next token.
func (l *lexer) nextToken() (tok token) {
	tok.t = l.scanner.Scan()
	tok.text = l.scanner.TokenText()
	// Both Pos and scanner.Position start line and column numbering at 1.
	tok.pos = Pos{Line: l.scanner.Position.Line, Col: l.scanner.Position.Column}
	return
}

// handleImag handles imaginary literals "[number]i" by peeking ahead.
func (l *lexer) handleImag(tok token, lval *yySymType) bool {
	if l.scanner.Peek() != 'i' {
		return false
	}
	l.scanner.Next()

	rat := new(big.Rat)
	if _, ok := rat.SetString(tok.text); !ok {
		l.posErrorf(tok.pos, "can't convert token [%v] to imaginary literal", tok)
	}
	lval.imagpos.pos = tok.pos
	lval.imagpos.imag = (*BigImag)(rat)
	return true
}

// translateToken takes the token we just scanned, and translates it into a
// token usable by yacc (lval and id).  The done return arg is true when a real
// yacc token was generated, or false if we need another next/translate pass.
func (l *lexer) translateToken(tok token, lval *yySymType) (id int, done bool) {
	switch tok.t {
	case scanner.EOF:
		l.sawEOF = true
		if autoSemi(l.prevTok) {
			return ';', true
		}
		return yaccEOF, true

	case '\n':
		if autoSemi(l.prevTok) {
			return ';', true
		}
		// Returning done=false ensures next/translate will be called again so that
		// this newline is skipped; id=yaccEOF is a dummy value that's ignored.
		return yaccEOF, false

	case scanner.String, scanner.RawString:
		var err error
		lval.strpos.pos = tok.pos
		lval.strpos.str, err = strconv.Unquote(tok.text)
		if err != nil {
			l.posErrorf(tok.pos, "can't convert token [%v] to string literal", tok)
		}
		return tSTRLIT, true

	case scanner.Int:
		if l.handleImag(tok, lval) {
			return tIMAGLIT, true
		}
		lval.intpos.pos = tok.pos
		lval.intpos.int = new(big.Int)
		if _, ok := lval.intpos.int.SetString(tok.text, 0); !ok {
			l.posErrorf(tok.pos, "can't convert token [%v] to integer literal", tok)
		}
		return tINTLIT, true

	case scanner.Float:
		if l.handleImag(tok, lval) {
			return tIMAGLIT, true
		}
		lval.ratpos.pos = tok.pos
		lval.ratpos.rat = new(big.Rat)
		if _, ok := lval.ratpos.rat.SetString(tok.text); !ok {
			l.posErrorf(tok.pos, "can't convert token [%v] to float literal", tok)
		}
		return tRATLIT, true

	case scanner.Ident:
		// Either the identifier is a known keyword, or we pass it through as IDENT.
		if keytok, ok := keywords[tok.text]; ok {
			lval.pos = tok.pos
			return keytok, true
		}
		lval.strpos.pos = tok.pos
		lval.strpos.str = tok.text
		return tIDENT, true

	case scanner.Comment:
		l.comments.addComment(tok)
		// Comments aren't considered tokens, just like the '\n' case.
		return yaccEOF, false

	default:
		// Either the rune is in our known punctuation whitelist, or we've hit a
		// syntax error.
		if nextRunes, ok := knownPunct[tok.t]; ok {
			// Peek at the next rune and compare against our list of next runes.  If
			// we find a match we return the id in next, otherwise just return the
			// original rune.  This means that 2-rune tokens always take precedence
			// over 1-rune tokens.  Either way the pos is set to the original rune.
			lval.pos = tok.pos
			peek := l.scanner.Peek()
			for _, next := range nextRunes {
				if peek == next.t {
					l.scanner.Next()
					return next.id, true
				}
			}
			return int(tok.t), true
		}
		l.posErrorf(tok.pos, "unexpected token [%v]", tok)
		l.sawEOF = true
		return yaccEOF, true
	}
}

// Lex is part of the yyLexer interface, called by the yacc-generated parser.
func (l *lexer) Lex(lval *yySymType) int {
	// Emit a dummy start token indicating what type of parse we're performing.
	if !l.started {
		l.started = true
		if l.opts.ImportsOnly {
			return startImportsOnly
		}
		return startFullFile
	}
	// Always return EOF after we've scanned it.  This ensures we emit EOF on the
	// next Lex call after scanning EOF and adding an auto-semicolon.
	if l.sawEOF {
		return yaccEOF
	}
	// Run next/translate in a loop to handle newline-triggered auto-semicolons;
	// nextToken needs to generate newline tokens so that we can trigger the
	// auto-semicolon logic, but if the newline doesn't generate an auto-semicolon
	// we should skip the token and move on to the next one.
	for {
		tok := l.nextToken()
		if id, done := l.translateToken(tok, lval); done {
			l.prevTok = tok
			l.comments.handleToken(tok)
			return id
		}
	}
}

// Error is part of the yyLexer interface, called by the yacc-generated parser.
// Unfortunately yacc doesn't give good error information - we dump the position
// of the previous scanned token as an approximation of where the error is.
func (l *lexer) Error(s string) {
	l.posErrorf(l.prevTok.pos, "%s", s)
}

// posErrorf generates an error with file and pos info.
func (l *lexer) posErrorf(pos Pos, format string, v ...interface{}) {
	var posstr string
	if pos.IsValid() {
		posstr = pos.String()
	}
	l.errs.Errorf(l.vdlFile.BaseName+":"+posstr+" "+format, v...)
}
