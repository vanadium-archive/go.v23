// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vom

import (
	"bytes"
	"reflect"

	"v.io/v23/vdl"
)

type RawBytes struct {
	Version    Version
	Type       *vdl.Type
	RefTypes   []*vdl.Type
	AnyLengths []uint64
	Data       []byte
}

func RawBytesOf(value interface{}) *RawBytes {
	rb, err := RawBytesFromValue(value)
	if err != nil {
		panic(err)
	}
	return rb
}

func RawBytesFromValue(value interface{}) (*RawBytes, error) {
	// TODO(bprosnitz) This implementation is temporary - we should make it faster
	dat, err := Encode(value)
	if err != nil {
		return nil, err
	}
	var rb RawBytes
	err = Decode(dat, &rb)
	return &rb, err
}

func (rb *RawBytes) ToValue(value interface{}) error {
	// TODO(bprosnitz) This implementation is temporary - we should make it faster
	dat, err := Encode(rb)
	if err != nil {
		return err
	}
	return Decode(dat, value)
}

func (rb *RawBytes) ToTarget(target vdl.Target) error {
	var buf bytes.Buffer
	enc := NewVersionedEncoder(rb.Version, &buf)
	if err := enc.enc.encodeRaw(rb); err != nil {
		return err
	}
	dec := NewDecoder(bytes.NewReader(buf.Bytes()))
	return dec.decodeToTarget(target)
}

type rvHackInterface interface {
	HackGetRv() reflect.Value
}

var rbType reflect.Type = reflect.TypeOf((*RawBytes)(nil))

// If rv is a raw bytes pointer, return a vdl target that
// modifies it. Otherwise return nil.
func makeRawBytesTarget(rv reflect.Value) vdl.Target {
	if rv.Type() == rbType {
		rv.Set(reflect.ValueOf(&RawBytes{}))
		return &rbTarget{rb: rv.Interface().(*RawBytes)}
	}
	return nil
}

func init() {
	vdl.RawBytesTargetFunc = makeRawBytesTarget
}

// vdl.Target that writes to a vom.RawBytes.
// This structure is intended to be one-time-use and
// created from makeRawBytesTarget.
type rbTarget struct {
	rb         *RawBytes     // RawBytes to write to
	enc        *encoder      // encoder to act as the underlying target
	buf        *bytes.Buffer // buffer of bytes written
	startCount int           // count the depth level of open calls, to determine when to write to rb
}

func (r *rbTarget) start(tt *vdl.Type) error {
	r.startCount++
	if r.startCount > 1 {
		return nil
	}

	r.buf = bytes.NewBuffer(nil)
	e := NewEncoder(r.buf)
	r.enc = &e.enc
	if _, err := r.enc.writer.Write([]byte{byte(r.enc.version)}); err != nil {
		return err
	}
	r.enc.sentVersionByte = true
	tid, err := r.enc.typeEnc.encode(tt)
	if err != nil {
		return err
	}
	if err := r.enc.startEncode(containsAny(tt), containsTypeObject(tt), hasChunkLen(tt), false, int64(tid)); err != nil {
		return err
	}
	return nil
}

func (r *rbTarget) finish() error {
	r.startCount--
	if r.startCount > 0 {
		return nil
	}

	if err := r.enc.finishEncode(); err != nil {
		return err
	}
	err := Decode(r.buf.Bytes(), r.rb)
	return err
}

func (r *rbTarget) FromBool(src bool, tt *vdl.Type) error {
	if err := r.start(tt); err != nil {
		return err
	}
	if err := r.enc.FromBool(src, tt); err != nil {
		return err
	}
	return r.finish()
}

func (r *rbTarget) FromUint(src uint64, tt *vdl.Type) error {
	if err := r.start(tt); err != nil {
		return err
	}
	if err := r.enc.FromUint(src, tt); err != nil {
		return err
	}
	return r.finish()
}

func (r *rbTarget) FromInt(src int64, tt *vdl.Type) error {
	if err := r.start(tt); err != nil {
		return err
	}
	if err := r.enc.FromInt(src, tt); err != nil {
		return err
	}
	return r.finish()
}

func (r *rbTarget) FromFloat(src float64, tt *vdl.Type) error {
	if err := r.start(tt); err != nil {
		return err
	}
	if err := r.enc.FromFloat(src, tt); err != nil {
		return err
	}
	return r.finish()
}

func (r *rbTarget) FromComplex(src complex128, tt *vdl.Type) error {
	if err := r.start(tt); err != nil {
		return err
	}
	if err := r.enc.FromComplex(src, tt); err != nil {
		return err
	}
	return r.finish()
}

func (r *rbTarget) FromBytes(src []byte, tt *vdl.Type) error {
	if err := r.start(tt); err != nil {
		return err
	}
	if err := r.enc.FromBytes(src, tt); err != nil {
		return err
	}
	return r.finish()
}

func (r *rbTarget) FromString(src string, tt *vdl.Type) error {
	if err := r.start(tt); err != nil {
		return err
	}
	if err := r.enc.FromString(src, tt); err != nil {
		return err
	}
	return r.finish()
}

func (r *rbTarget) FromEnumLabel(src string, tt *vdl.Type) error {
	if err := r.start(tt); err != nil {
		return err
	}
	if err := r.enc.FromEnumLabel(src, tt); err != nil {
		return err
	}
	return r.finish()
}

func (r *rbTarget) FromTypeObject(src *vdl.Type) error {
	if err := r.start(vdl.TypeObjectType); err != nil {
		return err
	}
	if err := r.enc.FromTypeObject(src); err != nil {
		return err
	}
	return r.finish()
}

func (r *rbTarget) FromNil(tt *vdl.Type) error {
	if err := r.start(tt); err != nil {
		return err
	}
	if err := r.enc.FromNil(tt); err != nil {
		return err
	}
	return r.finish()
}

func (r *rbTarget) StartList(tt *vdl.Type, len int) (vdl.ListTarget, error) {
	if err := r.start(tt); err != nil {
		return nil, err
	}
	if _, err := r.enc.StartList(tt, len); err != nil {
		return nil, err
	}
	return r, nil
}

func (r *rbTarget) FinishList(_ vdl.ListTarget) error {
	if err := r.enc.FinishList(nil); err != nil {
		return err
	}
	return r.finish()
}

func (r *rbTarget) StartSet(tt *vdl.Type, len int) (vdl.SetTarget, error) {
	if err := r.start(tt); err != nil {
		return nil, err
	}
	if _, err := r.enc.StartSet(tt, len); err != nil {
		return nil, err
	}
	return r, nil
}

func (r *rbTarget) FinishSet(_ vdl.SetTarget) error {
	if err := r.enc.FinishSet(nil); err != nil {
		return err
	}
	return r.finish()
}

func (r *rbTarget) StartMap(tt *vdl.Type, len int) (vdl.MapTarget, error) {
	if err := r.start(tt); err != nil {
		return nil, err
	}
	if _, err := r.enc.StartMap(tt, len); err != nil {
		return nil, err
	}
	return r, nil
}

func (r *rbTarget) FinishMap(_ vdl.MapTarget) error {
	if err := r.enc.FinishMap(nil); err != nil {
		return err
	}
	return r.finish()
}

func (r *rbTarget) StartFields(tt *vdl.Type) (vdl.FieldsTarget, error) {
	if err := r.start(tt); err != nil {
		return nil, err
	}
	if _, err := r.enc.StartFields(tt); err != nil {
		return nil, err
	}
	return r, nil
}

func (r *rbTarget) FinishFields(_ vdl.FieldsTarget) error {
	if err := r.enc.FinishFields(nil); err != nil {
		return err
	}
	return r.finish()
}

func (r *rbTarget) StartElem(index int) (vdl.Target, error) {
	if _, err := r.enc.StartElem(index); err != nil {
		return nil, err
	}
	return r, nil
}

func (r *rbTarget) FinishElem(_ vdl.Target) error {
	return r.enc.FinishElem(nil)
}

func (r *rbTarget) StartKey() (vdl.Target, error) {
	if _, err := r.enc.StartKey(); err != nil {
		return nil, err
	}
	return r, nil
}

func (r *rbTarget) FinishKey(_ vdl.Target) error {
	return r.enc.FinishKey(nil)
}

func (r *rbTarget) FinishKeyStartField(_ vdl.Target) (vdl.Target, error) {
	if _, err := r.enc.FinishKeyStartField(nil); err != nil {
		return nil, err
	}
	return r, nil
}

func (r *rbTarget) StartField(key string) (vdl.Target, vdl.Target, error) {
	if _, _, err := r.enc.StartField(key); err != nil {
		return nil, nil, err
	}
	return r, r, nil
}

func (r *rbTarget) FinishField(_, _ vdl.Target) error {
	return r.enc.FinishField(nil, nil)
}

func (r *rbTarget) RawBytesTargetHack() {}
