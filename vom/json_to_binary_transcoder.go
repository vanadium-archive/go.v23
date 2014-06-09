package vom

import (
	"bytes"
	"fmt"
	"io"
	"reflect"
	"strconv"
	"strings"
)

// JSONToObject converts a JSON string to a go object of the specified type.
func JSONToObject(str string, typ Type) (result interface{}, err error) {
	var buf bytes.Buffer
	tr := NewJSONToBinaryTranscoder(&buf, strings.NewReader(str))
	if err := tr.Transcode(typ); err != nil {
		return nil, fmt.Errorf("error transcoding json to object: %v (JSON was %q)", err, str)
	}
	var target interface{}
	dec := NewDecoder(bytes.NewReader(buf.Bytes()))
	if err := dec.Decode(&target); err != nil {
		return nil, fmt.Errorf("error decoding transcoded json: %v", err)
	}
	return target, nil
}

// JSONToBinaryTranscoder reads from JSON input stream and writes the transcoded objects to a VOM
// binary stream.
type JSONToBinaryTranscoder struct {
	w         io.Writer
	e         *encbuf
	tzr       *jsonTokenizer
	nextRefID int
	te        *binaryTypeEncoder
}

// NewJSONToBinaryTranscoder creates a new transcoder object from a JSON reader and a VOM binary
// writer.
func NewJSONToBinaryTranscoder(writer io.Writer, reader io.Reader) *JSONToBinaryTranscoder {
	return &JSONToBinaryTranscoder{
		w:         writer,
		e:         &encbuf{},
		tzr:       newJSONTokenizer(reader),
		nextRefID: 1,
		te:        newBinaryTypeEncoder(writer),
	}
}

// Transcode reads the next value from the transcoder's reader of the specified type.
func (t *JSONToBinaryTranscoder) Transcode(typ Type) error {
	rti, err := lookupRTInfo(typ)
	if err != nil {
		return err
	}

	id, err := t.te.encodeUnsentTypes(rti)
	if err != nil {
		return err
	}

	// reset our buffer for this message
	t.e.Reset()
	t.e.Grow(paddingLen)

	// do the actual transcoding
	if err := t.transcodeValue(typ, t.tzr); err != nil {
		return err
	}

	// write the message
	msg := encodeBinaryMsg(t.e, +int64(id))
	if _, err = t.w.Write(msg); err != nil {
		return fmt.Errorf("Error writing message: %v\n", err)
	}
	return nil
}

func stringToJSONByteString(str string) string {
	s := "["
	for i, b := range []byte(str) {
		if i > 0 {
			s += ","
		}
		s += fmt.Sprintf("%v", b)
	}
	s += "]"
	return s
}

func (t *JSONToBinaryTranscoder) transcodeValue(typ Type, tzr *jsonTokenizer) error {
	tok, err := tzr.Peek(0)
	if err != nil {
		return err
	}

	// if we have a named composite, remove the layers of naming to get to the core type
	// otherwise if this a pointer, dereference until we no longer have a pointer
	for {
		if typ.Kind() == reflect.Ptr {
			if tok.typ == jsonNullVal {
				rawEncodeInt(t.e, 0)
				return nil
			}

			rawEncodeInt(t.e, int64(-t.nextRefID))
			t.nextRefID++

			typ = typ.Elem()
		} else {
			break
		}
	}

	// transcode based on the kind
	kind := typ.Kind()
	switch kind {
	case reflect.Map, reflect.Struct:
		if kind == reflect.Map && tok.typ == jsonNullVal {
			rawEncodeInt(t.e, 0)
			tzr.ConsumePeeked()
			return nil
		}
		if tok.typ != jsonStartObj {
			return fmt.Errorf("poorly formatted map or struct. Doesn't start with '{'. (Token seen: %v)", tok)
		}
		return t.transcodeObject(typ, tzr)
	case reflect.Slice, reflect.Array:
		if kind == reflect.Slice && tok.typ == jsonNullVal {
			rawEncodeInt(t.e, 0)
			tzr.ConsumePeeked()
			return nil
		}
		if tok.typ == jsonStringVal && typ.Elem().Kind() == reflect.Uint8 {
			// if we have a string, then encode it as a string and we will read from that
			tzr.ConsumePeeked()
			tzr = newJSONTokenizer(strings.NewReader(stringToJSONByteString(tok.val)))
		} else if tok.typ != jsonStartArr {
			return fmt.Errorf("poorly formatted array or slice. Doesn't start with '[' (Token seen: %v)", tok)
		}
		return t.transcodeArray(typ, tzr)
	case reflect.String:
		if tok.typ != jsonStringVal {
			return fmt.Errorf("cannot decode value of kind %v into string", kind)
		}
		tzr.ConsumePeeked()
		return t.transcodePrimitive(tok.val, kind)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Float32, reflect.Float64:
		if tok.typ != jsonNumberVal && tok.typ != jsonStringVal {
			return fmt.Errorf("cannot decode value of kind %v into number", tok)
		}
		tzr.ConsumePeeked()
		return t.transcodePrimitive(tok.val, kind)
	case reflect.Bool:
		if tok.typ != jsonTrueVal && tok.typ != jsonFalseVal {
			return fmt.Errorf("cannot decode value of kind %v into bool", kind)
		}
		tzr.ConsumePeeked()
		return t.transcodePrimitive(tok.val, kind)
	case reflect.Interface:
		return t.transcodeInterface(tzr, typ.Name() == "error")
	default:
		panic(fmt.Sprintf("unhandled kind: %v", kind))
	}

	return nil
}

// stringError makes it easy to convert from json to an error type
type stringError string

func (se stringError) Error() string {
	return string(se)
}

func (t *JSONToBinaryTranscoder) transcodeInterface(tzr *jsonTokenizer, isError bool) error {
	tok, err := tzr.Peek(0)
	if err != nil {
		return err
	}

	if tok.typ == jsonNullVal {
		// if the interface is null, just send an id of 0
		rawEncodeInt(t.e, 0)
		tzr.ConsumePeeked()
		return nil
	}

	var typ Type
	if isError {
		// treat errors as stringError
		if tok.typ == jsonStringVal {
			if tok.val == "" {
				rawEncodeInt(t.e, 0)
				tzr.ConsumePeeked()
				return nil
			}
			typ = TypeOf(stringError(""))
		} else {
			fmt.Errorf("error must be a string. was %v", tok)
		}
	} else {
		// when passing objects into interfaces with json, we don't support types and just store the
		// in one possible lossless format.
		switch tok.typ {
		case jsonStartObj:
			typ = TypeOf(map[string]interface{}{})
		case jsonStartArr:
			typ = TypeOf([]interface{}{})
		case jsonStringVal:
			typ = TypeOf("")
		case jsonNumberVal:
			typ = TypeOf(float64(0))
		case jsonTrueVal, jsonFalseVal:
			typ = TypeOf(true)
		default:
			fmt.Errorf("invalid token starting interface: %v", tok)
		}
	}

	// encode any new types in the type buffer
	rti, err := lookupRTInfo(typ)
	if err != nil {
		return err
	}

	id, err := t.te.encodeUnsentTypes(rti)
	if err != nil {
		return err
	}

	// send the type id of the value we are transcoding along with the value itself
	rawEncodeInt(t.e, +int64(id))
	return t.transcodeValue(typ, tzr)
}

// countElements peeks forward in the tokenizer from the start of an array or object and counts the
// number of elements until the array or object is closed.
func (t *JSONToBinaryTranscoder) countElements(tzr *jsonTokenizer) (int, error) {
	tok, err := tzr.Peek(0)
	if err != nil {
		return 0, err
	}
	if tok == nil {
		return 0, fmt.Errorf("improperly formatted JSON. Stream terminated before array ends.")
	}
	if tok.typ != jsonStartObj && tok.typ != jsonStartArr {
		return 0, fmt.Errorf("expected array or object start tokens")
	}

	initialType := tok.typ

	// handle the case where we have an empty array or object ([] has same number of commas as [2],
	// but a different number of elements.
	if tok.typ == jsonStartArr {
		tok, err := tzr.Peek(1)
		if err != nil {
			return 0, err
		}
		if tok.typ == jsonEndArr {
			return 0, nil
		}
	}
	if tok.typ == jsonStartObj {
		tok, err := tzr.Peek(1)
		if err != nil {
			return 0, err
		}
		if tok.typ == jsonEndObj {
			return 0, nil
		}
	}

	// keep counts of objects and arrays open to determine whether the element is in the current
	// object or a child object
	arraysOpen := 0
	objOpen := 0
	commaCount := 0
	for i := 0; ; i++ {
		tok, err := tzr.Peek(i)
		if err != nil {
			return 0, fmt.Errorf("error during Peek(): %v", err)
		}
		if tok == nil {
			return 0, fmt.Errorf("improperly formatted JSON. Stream terminated before array or object ends.")
		}
		switch tok.typ {
		case jsonStartObj:
			objOpen++
		case jsonEndObj:
			objOpen--
			if objOpen == 0 && arraysOpen == 0 {
				return commaCount + 1, nil
			}
		case jsonStartArr:
			arraysOpen++
		case jsonEndArr:
			arraysOpen--
			if objOpen == 0 && arraysOpen == 0 {
				return commaCount + 1, nil
			}
		case jsonComma:
			if initialType == jsonStartArr && arraysOpen == 1 && objOpen == 0 {
				commaCount++
			}
			if initialType == jsonStartObj && objOpen == 1 && arraysOpen == 0 {
				commaCount++
			}
		}
	}
}

// transcodePrimitive is helper to write a primitive to the stream based on its string representation
func (t *JSONToBinaryTranscoder) transcodePrimitive(s string, kind reflect.Kind) error {
	switch kind {
	case reflect.String:
		rawEncodeString(t.e, s)
	case reflect.Bool:
		v, err := strconv.ParseBool(s)
		if err != nil {
			return err
		}
		rawEncodeBool(t.e, v)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		var bitSize int
		switch kind {
		case reflect.Int8:
			bitSize = 8
		case reflect.Int16:
			bitSize = 16
		case reflect.Int32:
			bitSize = 32
		case reflect.Int64, reflect.Int:
			bitSize = 64
		}
		v, err := strconv.ParseInt(s, 10, bitSize)
		if err != nil {
			return err
		}
		rawEncodeInt(t.e, v)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		var bitSize int
		switch kind {
		case reflect.Uint8:
			bitSize = 8
		case reflect.Uint16:
			bitSize = 16
		case reflect.Uint32:
			bitSize = 32
		case reflect.Uint64:
			bitSize = 64
		}
		v, err := strconv.ParseUint(s, 10, bitSize)
		if err != nil {
			return err
		}
		rawEncodeUint(t.e, v)
	case reflect.Float32, reflect.Float64:
		var bitSize int
		switch kind {
		case reflect.Float32:
			bitSize = 32
		case reflect.Float64:
			bitSize = 64
		}
		v, err := strconv.ParseFloat(s, bitSize)
		if err != nil {
			return err
		}
		rawEncodeFloat(t.e, v)
	default:
		panic(fmt.Sprintf("cannot transcode non-primitive kind %v with transcodePrimitive.", kind))
	}
	return nil
}

// readArray reads an array from the transcoder into rv. The array must start with '[', end with ']'
// and be comma separated.
func (t *JSONToBinaryTranscoder) transcodeArray(typ Type, tzr *jsonTokenizer) error {
	// get the number of items in the array
	itemCount, err := t.countElements(tzr)
	if err != nil {
		return err
	}

	// read the []
	tok, err := tzr.Next()
	if err != nil {
		return err
	}
	if tok.typ != jsonStartArr {
		return fmt.Errorf("incorrect start of array '%s' expected '%s'", tok.typ, jsonStartArr)
	}

	// determine the element type and output the length if a slice
	if typ.Kind() == reflect.Slice {
		rawEncodeUint(t.e, uint64(itemCount))
	} else if typ.Kind() != reflect.Array {
		return fmt.Errorf("unexpected kind for json array: %v", typ.Kind())
	}
	elemType := typ.Elem()

	// iterate through the items and transcode them
	for i := 0; i < itemCount; i++ {
		// read inner value
		if err := t.transcodeValue(elemType, tzr); err != nil {
			return err
		}

		// make sure next token is , and consume it
		if i < itemCount-1 {
			tok, err = tzr.Next()
			if err != nil {
				return err
			}
			if tok.typ != jsonComma {
				return fmt.Errorf("expected comma while parsing array, got: %v", tok)
			}
		}
	}

	// we expect a ] to be the next character
	tok, err = tzr.Next()
	if err != nil {
		return err
	}

	if tok.typ != jsonEndArr {
		return fmt.Errorf("expected array terminator ']'. Saw '%s'", tok)
	}

	return nil
}

// readObject reads a javascript object into rv. The input should be formatted as comma separated
// key-value pairs between {}.
func (t *JSONToBinaryTranscoder) transcodeObject(typ Type, tzr *jsonTokenizer) error {
	// reads the length of the object
	itemCount, err := t.countElements(tzr)
	if err != nil {
		return err
	}

	var tok *token
	// read {
	if tok, err = tzr.Next(); err != nil {
		return err
	}
	if tok.typ != jsonStartObj {
		return fmt.Errorf("incorrect start of object '%s' expected '%s'", tok.typ, jsonStartObj)
	}

	// output the length of the object if a map
	if typ.Kind() == reflect.Map {
		rawEncodeUint(t.e, uint64(itemCount))
	} else if typ.Kind() != reflect.Struct {
		return fmt.Errorf("unexpected kind for json array: %v", typ.Kind())
	}

	prevFx := -1
	for i := 0; i < itemCount; i++ {
		// test for }
		tok, err := tzr.Peek(0)
		if err != nil {
			return err
		}

		if tok.typ == jsonEndObj {
			break
		}

		// read the key
		tzr.ConsumePeeked()
		if tok.typ != jsonStringVal {
			return fmt.Errorf("JSON object keys may only be strings. Found %s", tok.typ)
		}
		key := tok.val

		// transcode the key if this is a map
		if typ.Kind() == reflect.Map {
			keyType := typ.Key()
			switch keyType.Kind() {
			case reflect.String, reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Float32, reflect.Float64:
				// attempt to special case primitives when possible for performance
				if err := t.transcodePrimitive(key, keyType.Kind()); err != nil {
					return fmt.Errorf("error in transcoding primitive key string: %v", err)
				}
			default:
				// transcode the key string
				keyTokenizer := newJSONTokenizer(strings.NewReader(key))
				if err := t.transcodeValue(keyType, keyTokenizer); err != nil {
					return fmt.Errorf("error in transcoding key string '%v': %v", key, err)
				}
			}
		} else {
			key = strings.Title(key)
		}

		// read the colon
		tok, err = tzr.Next()
		if err != nil {
			return err
		}
		if tok.typ != jsonColon {
			return fmt.Errorf("expected ':' between key and value. Found '%s'", tok.typ)
		}

		// transcode the inner value
		var valueType Type
		switch typ.Kind() {
		case reflect.Struct:
			var fieldIndex int
			for i := 0; i < typ.NumField(); i++ {
				fld := typ.Field(i)
				if fld.Name == key {
					valueType = fld.Type
					fieldIndex = i
					break
				}
			}
			if valueType == nil {
				for i := 0; i < typ.NumField(); i++ {
					fld := typ.Field(i)
					if strings.ToLower(fld.Name) == strings.ToLower(key) {
						valueType = fld.Type
						fieldIndex = i
						break
					}
				}
			}
			if valueType == nil {
				return fmt.Errorf("no field named '%v' found in struct %v.", key, typ.Name())
			}
			// TODO(bprosnitz) Skip zero fields. This involves reading all tokens of the object
			// being skipped.
			rawEncodeUint(t.e, uint64(fieldIndex-prevFx))
			prevFx = fieldIndex
		case reflect.Map:
			valueType = typ.Elem()
		}

		if err := t.transcodeValue(valueType, tzr); err != nil {
			return err
		}

		// read the comma
		if i < itemCount-1 {
			tok, err = tzr.Next()
			if err != nil {
				return err
			}
			if tok.typ != jsonComma {
				return fmt.Errorf("expected comma while parsing array, got: %v", tok)
			}
		}
	}

	// we expect a } to be the next character
	tok, err = tzr.Next()
	if err != nil {
		return err
	}
	if tok.typ != jsonEndObj {
		return fmt.Errorf("expected object terminator '}'. Saw '%s'", tok.typ)
	}

	if typ.Kind() == reflect.Struct {
		rawEncodeUint(t.e, uint64(0))
	}

	return nil
}
