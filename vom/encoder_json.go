package vom

import (
	"encoding/base64"
	"fmt"
	"io"
	"strconv"
	"unicode/utf8"

	"v.io/core/veyron2/wiretype"
)

// encoderJSON handles encoding type and value messages in json format.
type encoderJSON struct {
	writer io.Writer
	stateT *encState
	stateV *encState
	// For managing type definitions.
	sentTypes *nameResolver
	// Are we doing nested encoding of a map key?
	encodingMapKey bool
}

func newEncoderJSON(w io.Writer, stT, stV *encState) *encoderJSON {
	return &encoderJSON{
		writer:    w,
		stateT:    stT,
		stateV:    stV,
		sentTypes: newNameResolver(),
	}
}

func (e *encoderJSON) Format() Format { return FormatJSON }

// EncodeReflectValue encodes rv as a sequence of json type and value messages.
func (e *encoderJSON) EncodeReflectValue(rv Value) error {
	rti, err := lookupRTInfo(rv.Type())
	if err != nil {
		return err
	}
	if err := e.encodeUnsentTypes(rti); err != nil {
		return err
	}
	e.stateV.reset()
	e.stateV.buf.Reset()
	e.encodingMapKey = false
	if err := e.encodeTypedValue(e.stateV, rti, rv); err != nil {
		return err
	}
	e.stateV.buf.WriteByte('\n')
	_, err = e.writer.Write(e.stateV.buf.Bytes())
	return err
}

func (e *encoderJSON) encodeUnsentTypes(rti *rtInfo) error {
	if _, ok := bootstrapTypeIDs[rti]; ok {
		// No need to send bootstrap types.
		return nil
	}
	if e.sentTypes.contains(rti) {
		// Either we've already sent this type and its children, or this is a
		// recursive type and the type will be sent as the stack is unwound.
		return nil
	}
	e.sentTypes.insert(rti)

	// Set up rtiDFS, which we'll use to perform our post-order DFS.  We keep the
	// original rti unchanged so that we can encode the type later.
	rtiDFS := rti
	if c := rti.customJSON; c != nil {
		// The type has a custom coder method; continue DFS on the custom type.
		rtiDFS = c.rti
	}

	// Perform the recursive traversal of children.  The types are encoded in
	// post-order DFS; the ordering doesn't matter to the decoder, but the output
	// is nicer this way since non-cyclic types are in dependency order, so we can
	// use short names most of the time.
	switch rtiDFS.kind {
	case typeKindSlice, typeKindArray, typeKindPtr:
		if err := e.encodeUnsentTypes(rtiDFS.elem); err != nil {
			return err
		}
	case typeKindMap:
		if err := e.encodeUnsentTypes(rtiDFS.key); err != nil {
			return err
		}
		if err := e.encodeUnsentTypes(rtiDFS.elem); err != nil {
			return err
		}
	case typeKindStruct:
		for _, field := range rtiDFS.fields {
			if err := e.encodeUnsentTypes(field.info); err != nil {
				return err
			}
		}
	}

	// Encode and write the type message for user-defined types.  Since type
	// messages are never nested, we don't need to worry about escaping quotes.
	if rti.isNamed {
		// Enable short names hereafter - by enabling before we actually write the
		// type, we allow self-recursive types to use their own short name.
		e.sentTypes.enableShortNames(rti)
		e.stateT.buf.Reset()
		e.stateT.buf.WriteString(`["type","`)
		e.stateT.buf.WriteString(rti.name)
		e.stateT.buf.WriteByte(' ')
		e.encodeType(e.stateT.buf, rti, dontUseName)
		e.stateT.buf.WriteByte('"')
		e.encodeTypeTags(e.stateT.buf, rti.tags)
		e.stateT.buf.WriteString("]\n")
		if _, err := e.writer.Write(e.stateT.buf.Bytes()); err != nil {
			return err
		}
	}
	return nil
}

type nameMode bool

const (
	useName     nameMode = true
	dontUseName nameMode = false
)

func (e *encoderJSON) encodeType(buf *encbuf, rti *rtInfo, mode nameMode) {
	if mode == useName && rti.isNamed {
		buf.WriteString(e.sentTypes.toShortestName(rti))
		return
	}
	if c := rti.customJSON; c != nil {
		// The type has a custom coder method, so encode the wire type as if it were
		// of the custom type.
		rti = c.rti
	}
	switch rti.kind {
	case typeKindInterface:
		buf.WriteString("interface")
	case typeKindBool:
		buf.WriteString("bool")
	case typeKindString:
		buf.WriteString("string")
	case typeKindByte:
		buf.WriteString("byte")
	case typeKindUint:
		buf.WriteString("uint")
		buf.WriteString(rti.suffix)
	case typeKindInt:
		buf.WriteString("int")
		buf.WriteString(rti.suffix)
	case typeKindFloat:
		buf.WriteString("float")
		buf.WriteString(rti.suffix)
	case typeKindByteSlice:
		buf.WriteString("[]byte")
	case typeKindSlice:
		buf.WriteString("[]")
		e.encodeType(buf, rti.elem, useName)
	case typeKindArray:
		buf.WriteByte('[')
		e.encodeInt(buf, int64(rti.len), false)
		buf.WriteByte(']')
		e.encodeType(buf, rti.elem, useName)
	case typeKindMap:
		buf.WriteString("map[")
		e.encodeType(buf, rti.key, useName)
		buf.WriteByte(']')
		e.encodeType(buf, rti.elem, useName)
	case typeKindStruct:
		buf.WriteString("struct{")
		for fx, field := range rti.fields {
			if fx > 0 {
				buf.WriteByte(';')
			}
			buf.WriteString(field.name)
			buf.WriteByte(' ')
			e.encodeType(buf, field.info, useName)
		}
		buf.WriteByte('}')
	case typeKindPtr:
		buf.WriteByte('*')
		e.encodeType(buf, rti.elem, useName)
	default:
		panic(fmt.Errorf("vom: e.encodeType unhandled rti %+v", rti))
	}
}

func (e *encoderJSON) encodeTypeTags(buf *encbuf, tags []string) {
	if len(tags) > 0 {
		buf.WriteString(",[")
		for tx, tag := range tags {
			if tx > 0 {
				buf.WriteByte(',')
			}
			e.encodeString(buf, tag)
		}
		buf.WriteByte(']')
	}
}

// encodeTypedValue encodes a value with its explicit type wrapper.
func (e *encoderJSON) encodeTypedValue(st *encState, rti *rtInfo, rv Value) error {
	st.buf.WriteByte('[')
	e.encodeQuote(st.buf)
	e.encodeType(st.buf, rti, useName)
	e.encodeQuote(st.buf)
	st.buf.WriteByte(',')
	if err := e.encodeValue(st, rti, rv); err != nil {
		return err
	}
	st.buf.WriteByte(']')
	return nil
}

func (e *encoderJSON) encodeValue(st *encState, rti *rtInfo, rv Value) error {
	var err error
	if rti, rv, err = rti.callCustomEncode(rv, FormatJSON); err != nil {
		return err
	}
	switch rti.kind {
	case typeKindBool:
		e.encodeBool(st.buf, rv.Bool())
	case typeKindString:
		e.encodeString(st.buf, rv.String())
	case typeKindByteSlice:
		e.encodeByteSlice(st.buf, rv.Bytes())
	case typeKindUint, typeKindByte:
		// We add quotes for numbers that javascript can't handle.
		uval := rv.Uint()
		e.encodeUint(st.buf, uval, uval > float64MaxInt)
	case typeKindInt:
		// We add quotes for numbers that javascript can't handle.
		ival := rv.Int()
		e.encodeInt(st.buf, ival, ival < float64MinInt || ival > float64MaxInt)
	case typeKindFloat:
		e.encodeFloat(st.buf, rv.Float(), floatBitSize(rti.id))
	case typeKindSlice:
		switch {
		case rti.elem.id == wiretype.TypeIDUint8:
			e.encodeByteSlice(st.buf, rv.Bytes())
		default:
			return e.encodeSlice(st, rti, rv)
		}
	case typeKindArray:
		switch {
		case rti.elem.id == wiretype.TypeIDUint8:
			e.encodeByteSlice(st.buf, bytesToByteSlice(rv))
		default:
			return e.encodeSlice(st, rti, rv)
		}
	case typeKindMap:
		return e.encodeMap(st, rti, rv)
	case typeKindStruct:
		return e.encodeStruct(st, rti, rv)
	case typeKindPtr:
		return e.encodePtr(st, rti, rv)
	case typeKindInterface:
		return e.encodeInterface(st, rv)
	default:
		panic(fmt.Errorf("vom: encodeValue unhandled rti %#v", rti))
	}
	return nil
}

func (e *encoderJSON) encodeSlice(st *encState, rti *rtInfo, rv Value) error {
	st.buf.WriteByte('[')
	for ix := 0; ix < rv.Len(); ix++ {
		if ix > 0 {
			st.buf.WriteByte(',')
		}
		if err := e.encodeValue(st, rti.elem, rv.Index(ix)); err != nil {
			return err
		}
	}
	st.buf.WriteByte(']')
	return nil
}

func (e *encoderJSON) encodeMap(st *encState, rti *rtInfo, rv Value) error {
	st.buf.WriteByte('{')
	for kx, key := range rv.MapKeys() {
		if kx > 0 {
			st.buf.WriteByte(',')
		}
		// Map keys are recursively encoded as usual, but always represented as a
		// JSON string, so we need additional quoting and escaping.  Strings and
		// byte slices are already represented as json strings, so don't need any
		// additional quoting.
		quote := rti.key.kind != typeKindString && rti.key.kind != typeKindByteSlice
		if quote {
			e.encodeQuote(st.buf)
			if e.encodingMapKey {
				panic(fmt.Errorf("vom: maps can't be used as map keys: %#v", rti))
			}
			e.encodingMapKey = true
		}
		if err := e.encodeValue(st, rti.key, key); err != nil {
			return err
		}
		if quote {
			e.encodingMapKey = false
			e.encodeQuote(st.buf)
		}
		st.buf.WriteByte(':')
		if err := e.encodeValue(st, rti.elem, rv.MapIndex(key)); err != nil {
			return err
		}
	}
	st.buf.WriteByte('}')
	return nil
}

func (e *encoderJSON) encodeStruct(st *encState, rti *rtInfo, rv Value) error {
	st.buf.WriteByte('{')
	isFirst := true
	for _, rtf := range rti.fields {
		field := rv.Field(rtf.index)
		if isZeroValue(field) {
			continue
		}
		if !isFirst {
			st.buf.WriteByte(',')
		}
		isFirst = false
		e.encodeQuote(st.buf)
		st.buf.WriteString(rtf.name)
		e.encodeQuote(st.buf)
		st.buf.WriteByte(':')
		if err := e.encodeValue(st, rtf.info, field); err != nil {
			return err
		}
	}
	st.buf.WriteByte('}')
	return nil
}

func (e *encoderJSON) encodePtr(st *encState, rti *rtInfo, rv Value) error {
	if rv.IsNil() {
		st.buf.WriteString(jsonNullString)
		return nil
	}
	ptr := rv.Pointer()
	refID, lookup := st.lookupOrAssignRefID(ptr)
	st.buf.WriteByte('[')
	e.encodeInt(st.buf, refID, false)
	if lookup {
		// Already seen, just encode the [refID].
		st.buf.WriteByte(']')
		return nil
	}
	// Otherwise we just assigned a new refID, encode [refID,value].
	st.buf.WriteByte(',')
	if err := e.encodeValue(st, rti.elem, rv.Elem()); err != nil {
		return err
	}
	st.buf.WriteByte(']')
	return nil
}

func (e *encoderJSON) encodeInterface(st *encState, rv Value) error {
	if rv.IsNil() {
		st.buf.WriteString(jsonNullString)
		return nil
	}
	// Non-nil interfaces are encoded with both the type and value of the concrete
	// underlying value.  Any types we haven't seen yet will be encoded and sent
	// from the type buffer, to ensure all types are written out before any part
	// of the value is written.
	rv = rv.Elem()
	rti, err := lookupRTInfo(rv.Type())
	if err != nil {
		return err
	}
	if err := e.encodeUnsentTypes(rti); err != nil {
		return err
	}
	return e.encodeTypedValue(st, rti, rv)
}

// encodeQuote encodes a double quote (") into buf.  If we're performing nested
// encoding of a map key, we need to \-escape the quote.
//
// IMPORTANT: Use this method anytime we're encoding a double-quote.
func (e *encoderJSON) encodeQuote(buf *encbuf) {
	if e.encodingMapKey {
		buf.WriteByte('\\')
	}
	buf.WriteByte('"')
}

// encodeBackslash encodes a backslash (\) into buf.  If we're performing nested
// encoding of a map key, we need to \-escape the backslash.
//
// IMPORTANT: Use this method anytime we're encoding a backslash.
func (e *encoderJSON) encodeBackslash(buf *encbuf) {
	if e.encodingMapKey {
		buf.WriteByte('\\')
	}
	buf.WriteByte('\\')
}

// encodeString encodes str into buf, performing usual JSON escaping.  If we're
// performing nested encoding of a map key, we need extra escaping since we're
// already inside a JSON string.
//
// Heavily based on logic in the standard Go encoding/json package.
func (e *encoderJSON) encodeString(buf *encbuf, str string) {
	const hextable = "0123456789abcdef"
	e.encodeQuote(buf)
	start := 0
	strlen := len(str)
	for ix := 0; ix < strlen; {
		if b := str[ix]; b < utf8.RuneSelf {
			if 0x20 <= b && b != '\\' && b != '"' && b != '<' && b != '>' && b != '&' {
				// Make the common case of ASCII non-control or special chars fast.
				ix++
				continue
			}
			if start < ix {
				buf.WriteString(str[start:ix])
			}
			switch b {
			case '\\':
				e.encodeBackslash(buf)
				e.encodeBackslash(buf)
			case '"':
				e.encodeBackslash(buf)
				e.encodeQuote(buf)
			case '\b':
				e.encodeBackslash(buf)
				buf.WriteByte('b')
			case '\f':
				e.encodeBackslash(buf)
				buf.WriteByte('f')
			case '\n':
				e.encodeBackslash(buf)
				buf.WriteByte('n')
			case '\r':
				e.encodeBackslash(buf)
				buf.WriteByte('r')
			case '\t':
				e.encodeBackslash(buf)
				buf.WriteByte('t')
			default:
				// All control chars not handled above, '<', '>' and '&' are encoded
				// using their unicode code point.
				e.encodeBackslash(buf)
				buf.WriteString("u00")
				buf.WriteByte(hextable[b>>4])
				buf.WriteByte(hextable[b&0xf])
			}
			ix++
			start = ix
			continue
		}
		// The first byte isn't ASCII - decode as utf8.
		r, size := utf8.DecodeRuneInString(str[ix:])
		if r == utf8.RuneError && size == 1 {
			// Invalid utf8 sequences are encoded as the replacement char U+fffd.
			if start < ix {
				buf.WriteString(str[start:ix])
			}
			e.encodeBackslash(buf)
			buf.WriteString("ufffd")
			ix += size
			start = ix
			continue
		}
		// Special-case U+2028 line separator and U+2029 paragraph separator, which
		// are valid in JSON strings, but invalid in javascript strings.
		//   http://timelessrepo.com/json-isnt-a-javascript-subset
		if r == '\u2028' || r == '\u2029' {
			if start < ix {
				buf.WriteString(str[start:ix])
			}
			e.encodeBackslash(buf)
			buf.WriteString("u202")
			buf.WriteByte(hextable[r&0xf])
			ix += size
			start = ix
			continue
		}
		// Valid utf8 character - just continue the loop.
		ix += size
	}
	if start < strlen {
		buf.WriteString(str[start:])
	}
	e.encodeQuote(buf)
}

func (e *encoderJSON) encodeByteSlice(buf *encbuf, bs []byte) {
	e.encodeQuote(buf)
	b64buf := buf.Grow(base64.StdEncoding.EncodedLen(len(bs)))
	base64.StdEncoding.Encode(b64buf, bs)
	e.encodeQuote(buf)
}

func (e *encoderJSON) encodeBool(buf *encbuf, b bool) {
	if b {
		buf.WriteString("true")
	} else {
		buf.WriteString("false")
	}
}

func (e *encoderJSON) encodeInt(buf *encbuf, ival int64, quotes bool) {
	if quotes {
		e.encodeQuote(buf)
	}
	uval := uint64(ival)
	if ival < 0 {
		buf.WriteByte('-')
		uval = -uval
	}
	e.encodeUint(buf, uval, false)
	if quotes {
		e.encodeQuote(buf)
	}
}

func (e *encoderJSON) encodeUint(buf *encbuf, uval uint64, quotes bool) {
	if quotes {
		e.encodeQuote(buf)
	}
	const intMaxBytes = 20 // max uint64 = 18446744073709551615
	start := buf.Len()
	res := strconv.AppendUint(buf.Grow(intMaxBytes)[:0], uval, 10)
	buf.Truncate(start + len(res))
	if quotes {
		e.encodeQuote(buf)
	}
}

func (e *encoderJSON) encodeFloat(buf *encbuf, fval float64, bits int) {
	const floatMaxBytes = 64
	start := buf.Len()
	res := strconv.AppendFloat(buf.Grow(floatMaxBytes)[:0], fval, 'g', -1, bits)
	buf.Truncate(start + len(res))
}
