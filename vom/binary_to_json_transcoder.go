package vom

import (
	"fmt"
	"io"
	"strconv"
	"unicode"

	"veyron2/wiretype"
)

// BinaryToJSONTranscoder transcodes from VOM Binary to JSON.
type BinaryToJSONTranscoder struct {
	w      io.Writer
	e      *encbuf
	d      *decbuf
	decBin *decoderBinary // for decoding type information
}

// NewBinaryToJSONTranscoder constructs a BinaryToJSONTranscoder from a reader and a writer.
func NewBinaryToJSONTranscoder(writer io.Writer, reader io.Reader) *BinaryToJSONTranscoder {
	db := newDecbuf(reader, 4<<10)
	return &BinaryToJSONTranscoder{
		w:      writer,
		e:      newEncbuf(),
		d:      db,
		decBin: newDecoderBinary(db),
	}
}

// Transcode reads the next VOM Binary message and writes a VOM JSON message.
func (t *BinaryToJSONTranscoder) Transcode() error {
	t.e.Reset()
	for {
		valMsg, err := t.transcodeMessage()
		if err != nil {
			return err
		}
		if valMsg {
			break
		}
	}
	_, err := t.w.Write(t.e.Bytes())
	return err
}

func (t *BinaryToJSONTranscoder) transcodeMessage() (bool, error) {
	id, err := rawDecodeInt(t.d)
	if err != nil {
		return false, err
	}

	isTypeMsg := id < 0
	if isTypeMsg || !primitiveTypeIDs[wiretype.TypeID(id)] {
		msglen, err := rawDecodeUint(t.d)
		if err != nil {
			return false, err
		}
		t.d.SetLimit(int(msglen))
	}

	if isTypeMsg {
		err = t.decBin.decodeWireType(wiretype.TypeID(-id))
	} else {
		var def *wireDef
		if def, err = t.decBin.lookupWireDef(wiretype.TypeID(id)); err != nil {
			t.d.SkipToLimit()
			return false, err
		}
		var rti *rtInfo
		if rti, err = lookupRTInfo(def.rt); err != nil {
			t.d.SkipToLimit()
			return false, err
		}
		err = t.transcodeValue(rti)
	}

	leftover, err2 := t.d.SkipToLimit()
	if err == nil {
		// Only propagate leftover-skipping errors if we were error-free.
		if err2 != nil {
			err = err2
		} else if leftover > 0 && isTypeMsg {
			err = fmt.Errorf("%d leftover bytes while transcoding", leftover)
		}
	}

	return !isTypeMsg, err
}

func (t *BinaryToJSONTranscoder) transcodeValue(rti *rtInfo) error {
	switch rti.kind {
	case typeKindSlice, typeKindArray:
		return t.transcodeList(rti)
	case typeKindMap:
		return t.transcodeMap(rti)
	case typeKindStruct:
		return t.transcodeStruct(rti)
	case typeKindPtr:
		// TODO(bprosnitz) Decide whether to support pointers and support them if desired.
		return fmt.Errorf("pointer transcoding not currently supported")
	case typeKindInterface:
		return t.transcodeInterface()

	// Primitives:
	case typeKindByteSlice:
		var bs []byte
		if err := decodeString(t.d, rti, ValueOf(&bs).Elem()); err != nil {
			return err
		}
		t.e.WriteString("[")
		for i, b := range bs {
			if i > 0 {
				t.e.WriteString(",")
			}
			t.e.WriteString(strconv.FormatUint(uint64(b), 10))
		}
		t.e.WriteString("]")
	case typeKindString:
		var s string
		if err := decodeString(t.d, rti, ValueOf(&s).Elem()); err != nil {
			return err
		}
		t.e.WriteString(strconv.Quote(s))
	case typeKindBool:
		var b bool
		if err := decodeBool(t.d, rti, ValueOf(&b).Elem()); err != nil {
			return err
		}
		t.e.WriteString(strconv.FormatBool(b))
	case typeKindInt:
		var i int64
		if err := decodeInt(t.d, rti, ValueOf(&i).Elem()); err != nil {
			return err
		}
		t.e.WriteString(strconv.FormatInt(i, 10))
	case typeKindUint:
		var u uint64
		if err := decodeUint(t.d, rti, ValueOf(&u).Elem()); err != nil {
			return err
		}
		t.e.WriteString(strconv.FormatUint(u, 10))
	case typeKindByte:
		var b byte
		if err := decodeByte(t.d, rti, ValueOf(&b).Elem()); err != nil {
			return err
		}
		t.e.WriteString(strconv.FormatUint(uint64(b), 10))
	case typeKindFloat:
		var f float64
		if err := decodeFloat(t.d, rti, ValueOf(&f).Elem()); err != nil {
			return err
		}
		t.e.WriteString(strconv.FormatFloat(f, 'G', -1, 64))

	default:
		return fmt.Errorf("unknown kind: %v", rti.kind)
	}

	return nil
}

func (t *BinaryToJSONTranscoder) transcodeList(rti *rtInfo) error {
	var ulen uint64

	switch rti.kind {
	case typeKindSlice:
		var err error
		if ulen, err = rawDecodeUint(t.d); err != nil {
			return err
		}
	case typeKindArray:
		ulen = uint64(rti.len)
	default:
		fmt.Errorf("unexpected list kind %v", rti.kind)
	}

	t.e.WriteString("[")
	for i := uint64(0); i < ulen; i++ {
		if i > 0 {
			t.e.WriteString(",")
		}
		if err := t.transcodeValue(rti.elem); err != nil {
			return err
		}
	}
	t.e.WriteString("]")

	return nil
}

func (t *BinaryToJSONTranscoder) shoudBeQuoted(rti *rtInfo) bool {
	if rti.kind == typeKindString {
		return false
	}
	if rti.kind == typeKindInterface && rti.name == "error" {
		return false
	}
	return true
}

func (t *BinaryToJSONTranscoder) transcodeMap(rti *rtInfo) error {
	numEntries, err := rawDecodeUint(t.d)
	if err != nil {
		return err
	}

	t.e.WriteString("{")
	for i := uint64(0); i < numEntries; i++ {
		if i > 0 {
			t.e.WriteString(",")
		}

		if t.shoudBeQuoted(rti.key) {
			eb := newEncbuf()
			tr := &BinaryToJSONTranscoder{
				e:      eb,
				d:      t.d,
				decBin: t.decBin,
			}
			if err := tr.transcodeValue(rti.key); err != nil {
				return err
			}

			t.e.WriteString(strconv.Quote(string(eb.Bytes())))
		} else {
			if err := t.transcodeValue(rti.key); err != nil {
				return err
			}
		}

		t.e.WriteString(":")

		if err := t.transcodeValue(rti.elem); err != nil {
			return err
		}
	}
	t.e.WriteString("}")

	return nil
}

func lowercaseFirstCharacter(s string) string {
	for _, r := range s {
		return string(unicode.ToLower(r)) + s[1:]
	}
	return ""
}

func (t *BinaryToJSONTranscoder) transcodeStruct(rti *rtInfo) error {
	fx := -1

	t.e.WriteString("{")
	for {
		delta, err := rawDecodeUint(t.d)
		if err != nil {
			return err
		}
		if delta == 0 {
			break
		}
		if fx != -1 {
			t.e.WriteString(",")
		}

		fx += int(delta)
		if fx >= len(rti.fields) {
			return fmt.Errorf("struct %q field index %d out of bounds (max: %d)", rti, fx, len(rti.fields))
		}
		fld := rti.fields[fx]

		t.e.WriteString(strconv.Quote(lowercaseFirstCharacter(fld.name)))
		t.e.WriteString(":")
		if err := t.transcodeValue(fld.info); err != nil {
			return err
		}
	}
	t.e.WriteString("}")

	return nil
}

func (t *BinaryToJSONTranscoder) transcodeInterface() error {
	concreteID, err := rawDecodeInt(t.d)
	if err != nil {
		return err
	}

	if concreteID == 0 {
		t.e.WriteString("null")
		return nil
	}

	def, err := t.decBin.lookupWireDef(wiretype.TypeID(concreteID))
	if err != nil {
		return err
	}

	rti, err := lookupRTInfo(def.rt)
	if err != nil {
		return err
	}

	return t.transcodeValue(rti)
}
