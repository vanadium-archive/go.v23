package vom

import (
	"bufio"
	"bytes"
	"container/list"
	"fmt"
	"io"
	"strconv"
	"unicode"
)

type jsonTokenType int

const (
	jsonStartObj jsonTokenType = iota
	jsonEndObj
	jsonColon
	jsonComma
	jsonStartArr
	jsonEndArr
	jsonStringVal
	jsonNumberVal
	jsonTrueVal
	jsonFalseVal
	jsonNullVal
)

func (tt jsonTokenType) String() string {
	switch tt {
	case jsonStartObj:
		return "{"
	case jsonEndObj:
		return "}"
	case jsonColon:
		return ":"
	case jsonComma:
		return ","
	case jsonStartArr:
		return "["
	case jsonEndArr:
		return "]"
	case jsonStringVal:
		return "STRING"
	case jsonNumberVal:
		return "NUMBER"
	case jsonTrueVal:
		return "true"
	case jsonFalseVal:
		return "false"
	case jsonNullVal:
		return "null"
	default:
		return fmt.Sprintf("IllegalToken{%d}", tt)
	}
}

type token struct {
	typ jsonTokenType
	val string
}

func (tk token) String() string {
	switch tk.typ {
	case jsonStringVal, jsonNumberVal:
		return fmt.Sprintf("{%s,%q}", tk.typ, tk.val)
	default:
		return fmt.Sprintf("%s", tk.val)
	}
}

var (
	tokStartObj = &token{jsonStartObj, "{"}
	tokEndObj   = &token{jsonEndObj, "}"}
	tokColon    = &token{jsonColon, ":"}
	tokComma    = &token{jsonComma, ","}
	tokStartArr = &token{jsonStartArr, "["}
	tokEndArr   = &token{jsonEndArr, "]"}
	tokTrueVal  = &token{jsonTrueVal, "true"}
	tokFalseVal = &token{jsonFalseVal, "false"}
	tokNullVal  = &token{jsonNullVal, "null"}
)

// jsonTokenizer reads token objects from a JSON-formatted stream
type jsonTokenizer struct {
	r              *bufio.Reader
	bufferedTokens list.List // the next values if they were read in advance
}

// newJSONTokenizer creates a new jsonTokenizer that will read from the specified reader.
func newJSONTokenizer(reader io.Reader) *jsonTokenizer {
	return &jsonTokenizer{
		r: bufio.NewReader(reader),
	}
}

// Peek returns the ith token after the current read position (or nil or none), but does not
// consume any tokens.
func (t *jsonTokenizer) Peek(index int) (*token, error) {
	if t.bufferedTokens.Front() == nil {
		if err := t.bufferNext(); err != nil {
			return nil, err
		}
	}

	curr := t.bufferedTokens.Front()
	for i := 1; i <= index; i++ {
		if curr.Next() == nil {
			if err := t.bufferNext(); err != nil {
				return nil, err
			}
		}

		curr = curr.Next()
	}

	return curr.Value.(*token), nil
}

// Next returns the next token (or nil if none) and consumes it.
func (t *jsonTokenizer) Next() (*token, error) {
	if t.bufferedTokens.Front() == nil {
		if err := t.bufferNext(); err != nil {
			return nil, err
		}
	}

	return t.bufferedTokens.Remove(t.bufferedTokens.Front()).(*token), nil
}

// ConsumePeeked consumes the next token but does to return it.
func (t *jsonTokenizer) ConsumePeeked() {
	if t.bufferedTokens.Front() == nil {
		panic(fmt.Sprintf("attempted to consume peeked token, but no token was peeked."))
	}

	t.bufferedTokens.Remove(t.bufferedTokens.Front())
}

func (t *jsonTokenizer) bufferNext() error {
	tok, err := t.readNext()
	if err != nil {
		return err
	}
	t.bufferedTokens.PushBack(tok)
	return nil
}

// readNext gets the next token from the stream and returns it.
func (t *jsonTokenizer) readNext() (*token, error) {
	// read leading whitespace
	var ch rune
	for {
		var err error
		ch, _, err = t.r.ReadRune()
		if err != nil {
			return nil, err
		}

		if !unicode.IsSpace(ch) {
			break
		}
	}

	// create a token based on the next rune
	switch ch {
	case '{':
		return tokStartObj, nil
	case '}':
		return tokEndObj, nil
	case ':':
		return tokColon, nil
	case ',':
		return tokComma, nil
	case '[':
		return tokStartArr, nil
	case ']':
		return tokEndArr, nil
	case '"':
		if str, err := readUntilUnescapedQuote(t.r); err != nil {
			return nil, err
		} else {
			return &token{typ: jsonStringVal, val: str}, nil
		}
	case '-', '0', '1', '2', '3', '4', '5', '6', '7', '8', '9', '.':
		t.r.UnreadRune()
		if str, err := readNumberAsString(t.r); err != nil {
			return nil, err
		} else {
			return &token{typ: jsonNumberVal, val: str}, nil
		}
	case 't':
		if err := readExpectedSequence("rue", t.r); err != nil {
			return nil, err
		}
		return tokTrueVal, nil
	case 'f':
		if err := readExpectedSequence("alse", t.r); err != nil {
			return nil, err
		}
		return tokFalseVal, nil
	case 'n':
		if err := readExpectedSequence("ull", t.r); err != nil {
			return nil, err
		}
		return tokNullVal, nil
	default:
		return nil, fmt.Errorf("invalid rune while tokenizing %v", ch)
	}
}

// readExpectedSequence reads characters from the reader and errors if the read characters don't match the string
func readExpectedSequence(expected string, r *bufio.Reader) error {
	for _, ch := range expected {
		ch2, _, err := r.ReadRune()
		if err != nil {
			return err
		}
		if ch2 != ch {
			return fmt.Errorf("unexpected rune %v. Expected %v", ch2, ch)
		}
	}

	return nil
}

// readUntilUnescapedQuote reads characters from the reader until there is an unescaped double quote and returns the unquoted result as a string.
// This is used to read complete strings from the input.
func readUntilUnescapedQuote(r *bufio.Reader) (string, error) {
	var buf bytes.Buffer

	var escaped bool
	for {
		ch, _, err := r.ReadRune()

		if err != nil {
			return "", err
		}

		if !escaped {
			switch ch {
			case '\\':
				escaped = true
			case '"':
				return strconv.Unquote(`"` + buf.String() + `"`)
			}
		} else {
			escaped = false
		}

		buf.WriteRune(ch)
	}
}

// readNaturalNumberAsString reads into a string until the read character is outside of [0-9]
func readNaturalNumberAsString(r *bufio.Reader) (string, error) {
	var buf bytes.Buffer

	for {
		ch, _, err := r.ReadRune()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}

		if ch < '0' || ch > '9' {
			r.UnreadRune()
			break
		}

		buf.WriteRune(ch)
	}

	if buf.Len() == 0 {
		return "", fmt.Errorf("empty natural number")
	}

	return buf.String(), nil
}

// readNumberAsAString reads a number into a string. This includes floating point and exponential format.
// The definition of what can be in a number is based on the automata diagram on json.org.
func readNumberAsString(r *bufio.Reader) (string, error) {
	var buf bytes.Buffer

	ch, _, err := r.ReadRune()
	if err != nil {
		return "", err
	}

	if ch == '-' {
		buf.WriteRune('-')
	} else {
		r.UnreadRune()
	}

	str, err := readNaturalNumberAsString(r)
	if err != nil {
		return "", err
	}
	buf.WriteString(str)

	ch, _, err = r.ReadRune()
	if err == io.EOF {
		return buf.String(), nil
	}
	if err != nil {
		return "", err
	}

	if ch == '.' {
		buf.WriteRune('.')
		str, err := readNaturalNumberAsString(r)
		if err != nil {
			return "", err
		}
		buf.WriteString(str)
	} else {
		r.UnreadRune()
	}

	ch, _, err = r.ReadRune()
	if err == io.EOF {
		return buf.String(), nil
	}
	if err != nil {
		return "", err
	}
	if ch != 'e' && ch != 'E' {
		r.UnreadRune()
		return buf.String(), nil
	}
	buf.WriteRune(ch)

	ch, _, err = r.ReadRune()
	if err != nil {
		return "", err
	}
	if ch != '+' && ch != '-' {
		return "", fmt.Errorf("exponential number with no +/- afterwards")
	}
	buf.WriteRune(ch)

	str, err = readNaturalNumberAsString(r)
	if err != nil {
		return "", err
	}
	buf.WriteString(str)

	return buf.String(), nil
}
