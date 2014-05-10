package vom

// TODO(toddw): Add an standalone test for scannerJSON.

import (
	"errors"
	"fmt"
	"strconv"
	"unicode/utf8"
)

// DefaultMaxStringLength is the default max string length for decoding.
const DefaultMaxStringLength = 10 << 20 // 10MiB

var (
	// ErrStringTooLong represents decoding errors where the string is too long.
	ErrStringTooLong    = errors.New("vom: string too long")
	errNumberTooLong    = errors.New("vom: number too long")
	errInvalidRune      = errors.New("vom: invalid rune")
	errMismatchedQuotes = errors.New("vom: mismatched quotes")
)

// scannerJSON understands JSON syntax and produces jsonTokens.  An unusual
// feature is that we allow scanning of (possibly escaped) tokens from within
// strings; this is used to decode map keys.
type scannerJSON struct {
	// We hold the unread portion of the decbuf buffer in buf, along with index
	// telling us our read position.  This gives us better performance since we
	// can scan through buf directly.
	buf   []byte
	index int
	// While scanning we maintain a stack of states, for error-checking and to
	// allow SkipToEnd to skip forwards until the end of the current message.
	stack []jsonState
	// The stringDepth holds the depth of nested strings that we're scanning; this
	// is used to allow one-pass unescaping of strings.  This is necessary since
	// we encode map keys as JSON-escaped strings.  Since map keys can't contain
	// maps, the maximum depth is 2.
	stringDepth int
	// Hold the last error associated with the jsonError token.
	tokerr error

	decbuf          *decbuf
	maxStringLength int
}

func makeScannerJSON(decbuf *decbuf) scannerJSON {
	return scannerJSON{decbuf: decbuf, maxStringLength: DefaultMaxStringLength}
}

func (s *scannerJSON) SetMaxStringLength(max int) {
	s.maxStringLength = max
}

// jsonToken represents a JSON lexical token.
type jsonToken int

const (
	jsonError jsonToken = iota
	jsonCommaSym
	jsonColonSym
	jsonNull
	jsonTrue
	jsonFalse
	jsonStartNumber
	jsonStartString
	jsonStartArray
	jsonEndArray
	jsonStartObject
	jsonEndObject

	// Internal tokens - only returned by peekToken, never by ReadToken*.
	jsonStartNull
	jsonStartTrue
	jsonStartFalse
)

func (tok jsonToken) String() string {
	switch tok {
	case jsonError:
		return "error"
	case jsonCommaSym:
		return "comma"
	case jsonColonSym:
		return "colon"
	case jsonNull:
		return "null"
	case jsonTrue:
		return "true"
	case jsonFalse:
		return "false"
	case jsonStartNumber:
		return "start number"
	case jsonStartString:
		return "start string"
	case jsonStartArray:
		return "start array"
	case jsonEndArray:
		return "end array"
	case jsonStartObject:
		return "start object"
	case jsonEndObject:
		return "end object"
	case jsonStartNull:
		return "start null"
	case jsonStartTrue:
		return "start true"
	case jsonStartFalse:
		return "start false"
	default:
		panic(fmt.Errorf("vom: json unhandled tok %d", tok))
	}
}

// jsonState represents the state of our scanner, held in a stack.
type jsonState int

const (
	jsonStateArray jsonState = iota
	jsonStateObject
	jsonStateString
)

func (state jsonState) String() string {
	switch state {
	case jsonStateArray:
		return "array"
	case jsonStateObject:
		return "object"
	case jsonStateString:
		return "string"
	default:
		panic(fmt.Errorf("vom: json unhandled state %d", state))
	}
}

// TokenErr is similar to fmt.Errorf, but also appends the description of tok to
// the returned error.
func (s *scannerJSON) TokenErr(tok jsonToken, format string, v ...interface{}) error {
	msg := fmt.Sprintf(format, v...)
	if tok == jsonError {
		return fmt.Errorf("%s; %s", msg, s.tokerr.Error())
	}
	return fmt.Errorf("%s; saw %s", msg, tok)
}

// Reset resets the scanner, initialized with state as the only item on the
// state stack.
func (s *scannerJSON) Reset(state jsonState) {
	// TODO(toddw): Handle utf16?
	//   http://tools.ietf.org/html/rfc4627#section-3
	s.buf = nil
	s.index = 0
	s.stack = s.stack[0:0]
	s.stringDepth = 0
	s.tokerr = nil
	s.pushState(state)
}

// SkipToEnd tries to skip to the end of the current value we've scanned.  This
// allows the rest of the code to return errors freely as they occur, without
// worrying about exactly where it is in the scan.
func (s *scannerJSON) SkipToEnd() {
	// Keep scanning until the stack is empty.  Don't worry about checking exact
	// JSON syntax, but try to leave the read position on a good position for
	// subsequent decoding.
skiploop:
	for len(s.stack) > 0 {
		if s.stack[len(s.stack)-1] == jsonStateString {
			// Don't bother trying to recover from unscannable strings.
			break skiploop
		}
		switch s.ReadToken() {
		case jsonError:
			break skiploop
		case jsonStartNumber:
			if _, err := s.ScanNumber(); err != nil {
				break skiploop
			}
		case jsonStartString:
			if _, _, err := s.ReadStringAsSlice(); err != nil {
				break skiploop
			}
		}
	}
	s.decbuf.SkipPeek(s.index)
}

func (s *scannerJSON) pushState(state jsonState) {
	s.stack = append(s.stack, state)
	if state == jsonStateString {
		s.stringDepth++
	}
}

func (s *scannerJSON) popStateOnlyIf(expect jsonState) error {
	last := len(s.stack) - 1
	if last < 0 {
		return fmt.Errorf("mismatched end %s in state invalid", expect)
	}
	state := s.stack[last]
	if expect != state {
		return fmt.Errorf("mismatched end %s in state %s", expect, state)
	}
	s.stack = s.stack[:last]
	if state == jsonStateString {
		s.stringDepth--
	}
	return nil
}

// peekToken returns the next JSON token, without advancing the read position.
// The stringDepth tells us the nested string depth, used to determine how to
// handle escape sequences.  Returns the next json token, and the byte size of
// that token; incrementing s.buf by the byte size essentially reads the token.
func (s *scannerJSON) peekToken(stringDepth int) (jsonToken, int) {
	for {
		for buflen := len(s.buf); s.index < buflen; {
			switch b := s.buf[s.index]; b {
			case ',':
				return jsonCommaSym, 1
			case ':':
				return jsonColonSym, 1
			case '-', '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
				return jsonStartNumber, 0
			case '"':
				return jsonStartString, 1
			case '[':
				return jsonStartArray, 1
			case ']':
				return jsonEndArray, 1
			case '{':
				return jsonStartObject, 1
			case '}':
				return jsonEndObject, 1
			case 'n':
				return jsonStartNull, 0
			case 't':
				return jsonStartTrue, 0
			case 'f':
				return jsonStartFalse, 0
			case '\\':
				return s.peekEscape(stringDepth)
			case ' ', '\n', '\r', '\t':
				s.index++ // Skip whitespace.
			default:
				s.tokerr = fmt.Errorf("syntax error char %q", b)
				return jsonError, 0
			}
		}
		s.decbuf.SkipPeek(s.index)
		s.index = 0
		if s.buf, s.tokerr = s.decbuf.PeekAtLeast(1); s.tokerr != nil {
			return jsonError, 0
		}
	}
}

// peekEscape processes string escape sequences.
//
// REQUIRES: Must be called with s.buf[s.index] == '\\'
func (s *scannerJSON) peekEscape(depth int) (jsonToken, int) {
	if depth != 1 {
		// Escaped scanning is currently only used for map keys, and maps cannot be
		// used as map keys.  Thus we can simplify and disregard the nested case.
		s.tokerr = fmt.Errorf("vom: json invalid string escape at depth %d", depth)
		return jsonError, 0
	}
	s.decbuf.SkipPeek(s.index)
	s.index = 0
	if s.buf, s.tokerr = s.decbuf.PeekAtLeast(2); s.tokerr != nil {
		return jsonError, 0
	}
	// We only handle escaped quotes to simplify the logic.  In theory we should
	// handle other escape sequences, e.g. \u0022 == '"', \u005b == '['
	// TODO(toddw): Handle other escape sequences.
	switch b := s.buf[1]; b {
	case '"':
		return jsonStartString, 2 // Skip two bytes \"
	default:
		s.tokerr = fmt.Errorf("json unhandled escape char %q", b)
		return jsonError, 0
	}
}

func (s *scannerJSON) consumeToken(tok jsonToken, skip int) jsonToken {
	s.index += skip
	switch tok {
	case jsonCommaSym, jsonColonSym, jsonStartNumber, jsonError:
		return tok
	case jsonStartString:
		s.pushState(jsonStateString)
		return tok
	case jsonStartArray:
		s.pushState(jsonStateArray)
		return tok
	case jsonEndArray:
		if s.tokerr = s.popStateOnlyIf(jsonStateArray); s.tokerr != nil {
			s.index -= skip
			return jsonError
		}
		return tok
	case jsonStartObject:
		s.pushState(jsonStateObject)
		return tok
	case jsonEndObject:
		if s.tokerr = s.popStateOnlyIf(jsonStateObject); s.tokerr != nil {
			s.index -= skip
			return jsonError
		}
		return tok
	case jsonStartNull:
		return s.consumeNull()
	case jsonStartTrue:
		return s.consumeTrue()
	case jsonStartFalse:
		return s.consumeFalse()
	default:
		panic(fmt.Errorf("vom: json unhandled tok %d", tok))
	}
}

// REQUIRES: Must be called with s.buf[s.index] == 'n'
func (s *scannerJSON) consumeNull() jsonToken {
	s.decbuf.SkipPeek(s.index)
	s.index = 0
	if s.buf, s.tokerr = s.decbuf.PeekAtLeast(4); s.tokerr != nil {
		return jsonError
	}
	s.index = 4
	if s.buf[1] != 'u' || s.buf[2] != 'l' || s.buf[3] != 'l' {
		s.tokerr = fmt.Errorf(`expected "null", saw %q`, s.buf[:4])
		return jsonError
	}
	return jsonNull
}

// REQUIRES: Must be called with s.buf[s.index] == 't'
func (s *scannerJSON) consumeTrue() jsonToken {
	s.decbuf.SkipPeek(s.index)
	s.index = 0
	if s.buf, s.tokerr = s.decbuf.PeekAtLeast(4); s.tokerr != nil {
		return jsonError
	}
	s.index = 4
	if s.buf[1] != 'r' || s.buf[2] != 'u' || s.buf[3] != 'e' {
		s.tokerr = fmt.Errorf(`expected "true", saw %q`, s.buf[:4])
		return jsonError
	}
	return jsonTrue
}

// REQUIRES: Must be called with s.buf[s.index] == 'f'
func (s *scannerJSON) consumeFalse() jsonToken {
	s.decbuf.SkipPeek(s.index)
	s.index = 0
	if s.buf, s.tokerr = s.decbuf.PeekAtLeast(5); s.tokerr != nil {
		return jsonError
	}
	s.index = 5
	if s.buf[1] != 'a' || s.buf[2] != 'l' || s.buf[3] != 's' || s.buf[4] != 'e' {
		s.tokerr = fmt.Errorf(`expected "false", saw %q`, s.buf[:5])
		return jsonError
	}
	return jsonFalse
}

// ReadToken returns the next JSON token.  The read position is advanced past
// the token, except for jsonStartNumber where the read position is left on the
// first character of the number.
func (s *scannerJSON) ReadToken() jsonToken {
	return s.consumeToken(s.peekToken(s.stringDepth))
}

// ReadTokenOnlyIf returns true and reads the next JSON token iff it is expect.
// Returns false if the next JSON token isn't expect, without advancing the read
// position.
func (s *scannerJSON) ReadTokenOnlyIf(expect jsonToken) bool {
	if tok, skip := s.peekToken(s.stringDepth); tok == expect {
		return s.consumeToken(tok, skip) == expect
	}
	return false
}

// ScanNumber returns a string representing the next JSON number.
//
// REQUIRES: Must be called with s.buf[s.index] on the start of a JSON number.
func (s *scannerJSON) ScanNumber() ([]byte, error) {
	start := s.index
	s.index++ // We know the initial byte is valid.
	for {
		for buflen := len(s.buf); s.index < buflen; s.index++ {
			switch s.buf[s.index] {
			case '.', 'e', 'E', '-', '+', '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
				continue
			default:
				return s.buf[start:s.index], nil
			}
		}
		nextindex := s.index - start
		// We rely on s.decbuf being large enough to hold any valid number.
		if nextindex >= s.decbuf.Size() {
			s.decbuf.SkipPeek(nextindex)
			return nil, errNumberTooLong
		}
		s.decbuf.SkipPeek(start)
		start = 0
		s.index = nextindex
		var err error
		if s.buf, err = s.decbuf.PeekAtLeast(nextindex + 1); err != nil {
			return nil, err
		}
	}
}

// ReadString returns the next JSON string.
func (s *scannerJSON) ReadString() (string, error) {
	slice, _, err := s.ReadStringAsSlice()
	if err != nil {
		return "", err
	}
	return string(slice), err
}

// ReadString returns the next JSON string as a byte slice, along with a bool
// which is true iff the returned byte slice was newly allocated.  If the bool
// is false the caller must be careful to deep copy the slice when necessary.
func (s *scannerJSON) ReadStringAsSlice() ([]byte, bool, error) {
	// Handle simple cases where we can just return a slice from the decbuf,
	// backing off to readStringNew which creates a new byteslice.
	start := s.index
	if s.stringDepth > 1 {
		// Backoff to readStringNew for nested strings, to handle mismatched quotes.
		str, err := s.readStringNew(start)
		return str, true, err
	}
	for {
		for buflen := len(s.buf); s.index < buflen; s.index++ {
			switch b := s.buf[s.index]; {
			case b == '"':
				str := s.buf[start:s.index]
				s.index++
				if err := s.popStateOnlyIf(jsonStateString); err != nil {
					return nil, false, err
				}
				return str, false, nil
			case b == '\\' || b > 0x7f:
				// Escape sequences and multi-byte utf8 are hard to process in-place;
				// backoff to readStringNew.
				str, err := s.readStringNew(start)
				return str, true, err
			}
		}
		nextindex := s.index - start
		if nextindex >= s.decbuf.Size() {
			// The string is larger than the decbuf; we must use readStringNew.
			str, err := s.readStringNew(start)
			return str, true, err
		}
		s.decbuf.SkipPeek(start)
		start = 0
		s.index = nextindex
		var err error
		if s.buf, err = s.decbuf.PeekAtLeast(nextindex + 1); err != nil {
			return nil, false, err
		}
	}
}

// readStringNew reads the next JSON string and returns it as a newly allocated
// byte slice.
func (s *scannerJSON) readStringNew(start int) ([]byte, error) {
	// Create return str and copy the prefix s.buf[start:s.index] into it.
	strlen := s.index - start
	strcap := 1024
	if strlen > strcap {
		strcap = strlen * 2
	}
	str := make([]byte, strcap)
	copy(str, s.buf[start:s.index])
	s.decbuf.SkipPeek(s.index)
	s.index = 0
	// Fill str one rune at a time, converting escape sequences.
	var err error
	for {
		var r rune
		var end bool
		if r, end, err = s.readRune(s.stringDepth); err != nil {
			break
		}
		if end {
			err = s.popStateOnlyIf(jsonStateString)
			break
		}
		if strcap-strlen < utf8.UTFMax {
			// Double str size, copying existing str contents.
			strcap *= 2
			newstr := make([]byte, strcap)
			copy(newstr, str[:strlen])
			str = newstr
		}
		// Encode rune r into str.
		strlen += utf8.EncodeRune(str[strlen:], r)
		if strlen > s.maxStringLength {
			err = ErrStringTooLong
			break
		}
	}
	// Reset s.buf for subsequent scanning.
	var err2 error
	s.buf, err2 = s.decbuf.PeekAtLeast(1)
	if err == nil {
		err = err2
	}
	if err != nil {
		str = nil
	} else {
		str = str[:strlen]
	}
	return str, err
}

// readRune reads and returns the next rune, along with a bool that's true iff
// the string has ended.  This handles unescaping of nested strings by calling
// itself recursively.
func (s *scannerJSON) readRune(depth int) (rune, bool, error) {
	// Base case just reads a single rune without handling escapes.
	if depth == 0 {
		r, err := s.readRuneDepth0()
		return r, false, err
	}
	// Read the next rune, and return it if it's not an escape sequence.
	r, _, err := s.readRune(depth - 1)
	if err != nil {
		return 0, false, err
	}
	if r != '\\' {
		if r == '"' {
			if depth != s.stringDepth {
				return 0, false, errMismatchedQuotes
			}
			return '"', true, nil
		}
		return r, false, nil
	}
	// Read next rune from the escape sequence.
	esc, _, err := s.readRune(depth - 1)
	if err != nil {
		return 0, false, err
	}
	switch esc {
	case '"':
		return '"', false, nil
	case '\\', '/':
		return esc, false, nil
	case 'b':
		return '\b', false, nil
	case 'f':
		return '\f', false, nil
	case 'n':
		return '\n', false, nil
	case 'r':
		return '\r', false, nil
	case 't':
		return '\t', false, nil
	case 'u':
		// Handle \u1234 escape sequences.
		ubuf, err := s.decbuf.ReadBuf(4)
		if err != nil {
			return 0, false, err
		}
		uval, err := strconv.ParseUint(string(ubuf), 16, 64)
		if err != nil {
			return 0, false, err
		}
		r := rune(uval)
		if !utf8.ValidRune(r) {
			return 0, false, errInvalidRune
		}
		return r, false, nil
	default:
		return 0, false, fmt.Errorf("vom: json unknown escape char %q", esc)
	}
}

// readRuneDepth0() is the implementation of readRune() at depth==0.
func (s *scannerJSON) readRuneDepth0() (rune, error) {
	// Based on the first byte, we know how many bytes are in the rune:
	// http://en.wikipedia.org/wiki/UTF-8#Codepage_layout
	b, err := s.decbuf.PeekByte()
	if err != nil {
		return 0, err
	}
	var runebuf []byte
	switch {
	case b <= 0x7f: // 1-byte rune
		s.decbuf.SkipPeek(1)
		return rune(b), nil
	case b >= 0xc2 && b <= 0xdf: // 2-byte rune (c0,c1 are invalid)
		runebuf, err = s.decbuf.ReadBuf(2)
	case b >= 0xe0 && b <= 0xef: // 3-byte rune
		runebuf, err = s.decbuf.ReadBuf(3)
	case b >= 0xf0 && b <= 0xf4: // 4-byte rune (f5-ff are invalid)
		runebuf, err = s.decbuf.ReadBuf(4)
	default:
		return 0, errInvalidRune
	}
	if err != nil {
		return 0, err
	}
	r, rlen := utf8.DecodeRune(runebuf)
	if r == utf8.RuneError && rlen == 1 {
		return 0, errInvalidRune
	}
	return r, nil
}

// ReadEndString is the logical equivalent of ReadTokenOnlyIf(jsonEndString),
// and is necessary since jsonEndString doesn't exist.  Basically we expect that
// the next item to read is an end-string sequence (a possibly-escaped
// double-quote '"') and read it.
func (s *scannerJSON) ReadEndString() error {
	tok, skip := s.peekToken(s.stringDepth)
	if tok != jsonStartString {
		return s.TokenErr(tok, "vom: expected end string")
	}
	if err := s.popStateOnlyIf(jsonStateString); err != nil {
		return err
	}
	s.index += skip
	return nil
}
