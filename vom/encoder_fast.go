// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vom

import (
	"fmt"

	"v.io/v23/vdl"
	"v.io/v23/verror"
)

// This file contains the WriteValue*, NextEntryValue* and NextFieldValue*
// methods.  The implementation is faster than calling the underlying StartValue
// / Encode* / FinishValue methods, because we can avoid pushing and popping the
// encoder stack.  We can also avoid checking for startMessage / finishMessage
// for the Next*Value methods, since there must be a value on the stack, by
// definition.

// Each of the encode* types handles encoding that type of data.  The encode
// methods of these types are passed into the general writeValue, nextEntryValue
// and nextFieldValue methods.
type (
	encodeBool    struct{ Value bool }
	encodeString  struct{ Value string }
	encodeOneByte struct{ Value byte }
	encodeUint    struct{ Value uint64 }
	encodeInt     struct{ Value int64 }
	encodeFloat   struct{ Value float64 }
	encodeBytes   struct {
		Value []byte
		Kind  vdl.Kind
	}
)

func (x encodeBool) encode(buf *encbuf)    { binaryEncodeBool(buf, x.Value) }
func (x encodeString) encode(buf *encbuf)  { binaryEncodeString(buf, x.Value) }
func (x encodeOneByte) encode(buf *encbuf) { buf.WriteOneByte(x.Value) }
func (x encodeUint) encode(buf *encbuf)    { binaryEncodeUint(buf, x.Value) }
func (x encodeInt) encode(buf *encbuf)     { binaryEncodeInt(buf, x.Value) }
func (x encodeFloat) encode(buf *encbuf)   { binaryEncodeFloat(buf, x.Value) }
func (x encodeBytes) encode(buf *encbuf) {
	if x.Kind == vdl.Array {
		binaryEncodeUint(buf, 0)
	} else {
		binaryEncodeUint(buf, uint64(len(x.Value)))
	}
	buf.Write(x.Value)
}

// writeValue implements the equivalent of StartValue, Encode*, FinishValue.
func (e *encoder81) writeValue(tt *vdl.Type, encode func(*encbuf)) error {
	top := e.top()
	if top == nil {
		// Top-level value.
		msgType := tt
		if e.nextStartValueIsOptional {
			msgType = vdl.OptionalType(tt)
		}
		if err := e.startMessage(msgType); err != nil {
			return err
		}
		encode(e.buf)
		e.nextStartValueIsOptional = false
		return e.finishMessage()
	}
	// Non top-level value.
	top.NumStarted++
	isInsideAny := top.nextValueIsAny()
	var anyRef anyStartRef
	if isInsideAny {
		anyType := tt
		if e.nextStartValueIsOptional {
			anyType = vdl.OptionalType(tt)
		}
		tid, err := e.typeEnc.encode(anyType)
		if err != nil {
			return err
		}
		binaryEncodeUint(e.buf, e.tids.ReferenceTypeID(tid))
		anyRef = e.anyLens.StartAny(e.buf.Len())
		binaryEncodeUint(e.buf, uint64(anyRef.index))
	}
	encode(e.buf)
	if isInsideAny {
		e.anyLens.FinishAny(anyRef, e.buf.Len())
	}
	e.nextStartValueIsOptional = false
	return nil
}

// nextEntryValue implements the equivalent of NextEntry(false), StartValue,
// Encode*, FinishValue.
func (e *encoder81) nextEntryValue(tt *vdl.Type, encode func(*encbuf)) error {
	top := e.top()
	if top == nil {
		return errEmptyEncoderStack
	}
	// NextEntry
	top.Index++
	if top.Index == 0 {
		switch {
		case top.Type.Kind() == vdl.Array:
			binaryEncodeUint(e.buf, 0)
		case top.LenHint >= 0:
			binaryEncodeUint(e.buf, uint64(top.LenHint))
		}
	}
	// StartValue
	top.NumStarted++
	isInsideAny := top.nextValueIsAny()
	var anyRef anyStartRef
	if isInsideAny {
		anyType := tt
		if e.nextStartValueIsOptional {
			anyType = vdl.OptionalType(tt)
		}
		tid, err := e.typeEnc.encode(anyType)
		if err != nil {
			return err
		}
		binaryEncodeUint(e.buf, e.tids.ReferenceTypeID(tid))
		anyRef = e.anyLens.StartAny(e.buf.Len())
		binaryEncodeUint(e.buf, uint64(anyRef.index))
	}
	encode(e.buf)
	// FinishValue
	if isInsideAny {
		e.anyLens.FinishAny(anyRef, e.buf.Len())
	}
	e.nextStartValueIsOptional = false
	return nil
}

// nextFieldValue implements the equivalent of NextField(name), StartValue,
// Encode*, FinishValue.
func (e *encoder81) nextFieldValue(name string, tt *vdl.Type, encode func(*encbuf)) error {
	top := e.top()
	if top == nil {
		return errEmptyEncoderStack
	}
	// NextField
	if _, top.Index = top.Type.FieldByName(name); top.Index == -1 {
		return fmt.Errorf("vom: NextField called with invalid field %q", name)
	}
	binaryEncodeUint(e.buf, uint64(top.Index))
	// StartValue
	top.NumStarted++
	isInsideAny := top.nextValueIsAny()
	var anyRef anyStartRef
	if isInsideAny {
		anyType := tt
		if e.nextStartValueIsOptional {
			anyType = vdl.OptionalType(tt)
		}
		tid, err := e.typeEnc.encode(anyType)
		if err != nil {
			return err
		}
		binaryEncodeUint(e.buf, e.tids.ReferenceTypeID(tid))
		anyRef = e.anyLens.StartAny(e.buf.Len())
		binaryEncodeUint(e.buf, uint64(anyRef.index))
	}
	encode(e.buf)
	// FinishValue
	if isInsideAny {
		e.anyLens.FinishAny(anyRef, e.buf.Len())
	}
	e.nextStartValueIsOptional = false
	return nil
}

// WriteValue* methods

func (e *encoder81) WriteValueBool(tt *vdl.Type, value bool) error {
	return e.writeValue(tt, encodeBool{value}.encode)
}

func (e *encoder81) WriteValueString(tt *vdl.Type, value string) error {
	if tt.Kind() == vdl.Enum {
		index := tt.EnumIndex(value)
		if index < 0 {
			return verror.New(errLabelNotInType, nil, value, tt)
		}
		return e.writeValue(tt, encodeUint{uint64(index)}.encode)
	} else {
		return e.writeValue(tt, encodeString{value}.encode)
	}
}

func (e *encoder81) WriteValueUint(tt *vdl.Type, value uint64) error {
	if top := e.top(); top != nil && top.Type.IsBytes() {
		return e.writeValue(tt, encodeOneByte{byte(value)}.encode)
	} else {
		return e.writeValue(tt, encodeUint{value}.encode)
	}
}

func (e *encoder81) WriteValueInt(tt *vdl.Type, value int64) error {
	return e.writeValue(tt, encodeInt{value}.encode)
}

func (e *encoder81) WriteValueFloat(tt *vdl.Type, value float64) error {
	return e.writeValue(tt, encodeFloat{value}.encode)
}

func (e *encoder81) WriteValueTypeObject(value *vdl.Type) error {
	// TypeObject is hard to implement, so we call the methods in sequence.
	if err := e.StartValue(vdl.TypeObjectType); err != nil {
		return err
	}
	if err := e.EncodeTypeObject(value); err != nil {
		return err
	}
	return e.FinishValue()
}

func (e *encoder81) WriteValueBytes(tt *vdl.Type, value []byte) error {
	return e.writeValue(tt, encodeBytes{value, tt.Kind()}.encode)
}

// NextEntryValue* methods

func (e *encoder81) NextEntryValueBool(tt *vdl.Type, value bool) error {
	return e.nextEntryValue(tt, encodeBool{value}.encode)
}

func (e *encoder81) NextEntryValueString(tt *vdl.Type, value string) error {
	if tt.Kind() == vdl.Enum {
		index := tt.EnumIndex(value)
		if index < 0 {
			return verror.New(errLabelNotInType, nil, value, tt)
		}
		return e.nextEntryValue(tt, encodeUint{uint64(index)}.encode)
	} else {
		return e.nextEntryValue(tt, encodeString{value}.encode)
	}
}

func (e *encoder81) NextEntryValueUint(tt *vdl.Type, value uint64) error {
	if top := e.top(); top != nil && top.Type.IsBytes() {
		return e.nextEntryValue(tt, encodeOneByte{byte(value)}.encode)
	} else {
		return e.nextEntryValue(tt, encodeUint{value}.encode)
	}
}

func (e *encoder81) NextEntryValueInt(tt *vdl.Type, value int64) error {
	return e.nextEntryValue(tt, encodeInt{value}.encode)
}

func (e *encoder81) NextEntryValueFloat(tt *vdl.Type, value float64) error {
	return e.nextEntryValue(tt, encodeFloat{value}.encode)
}

func (e *encoder81) NextEntryValueTypeObject(value *vdl.Type) error {
	// TypeObject is hard to implement, so we call the methods in sequence.
	if err := e.NextEntry(false); err != nil {
		return err
	}
	return e.WriteValueTypeObject(value)
}

func (e *encoder81) NextEntryValueBytes(tt *vdl.Type, value []byte) error {
	return e.nextEntryValue(tt, encodeBytes{value, tt.Kind()}.encode)
}

// NextFieldValue* methods

func (e *encoder81) NextFieldValueBool(name string, tt *vdl.Type, value bool) error {
	return e.nextFieldValue(name, tt, encodeBool{value}.encode)
}

func (e *encoder81) NextFieldValueString(name string, tt *vdl.Type, value string) error {
	if tt.Kind() == vdl.Enum {
		index := tt.EnumIndex(value)
		if index < 0 {
			return verror.New(errLabelNotInType, nil, value, tt)
		}
		return e.nextFieldValue(name, tt, encodeUint{uint64(index)}.encode)
	} else {
		return e.nextFieldValue(name, tt, encodeString{value}.encode)
	}
}

func (e *encoder81) NextFieldValueUint(name string, tt *vdl.Type, value uint64) error {
	if top := e.top(); top != nil && top.Type.IsBytes() {
		return e.nextFieldValue(name, tt, encodeOneByte{byte(value)}.encode)
	} else {
		return e.nextFieldValue(name, tt, encodeUint{value}.encode)
	}
}

func (e *encoder81) NextFieldValueInt(name string, tt *vdl.Type, value int64) error {
	return e.nextFieldValue(name, tt, encodeInt{value}.encode)
}

func (e *encoder81) NextFieldValueFloat(name string, tt *vdl.Type, value float64) error {
	return e.nextFieldValue(name, tt, encodeFloat{value}.encode)
}

func (e *encoder81) NextFieldValueTypeObject(name string, value *vdl.Type) error {
	// TypeObject is hard to implement, so we call the methods in sequence.
	if err := e.NextField(name); err != nil {
		return err
	}
	return e.WriteValueTypeObject(value)
}

func (e *encoder81) NextFieldValueBytes(name string, tt *vdl.Type, value []byte) error {
	return e.nextFieldValue(name, tt, encodeBytes{value, tt.Kind()}.encode)
}
