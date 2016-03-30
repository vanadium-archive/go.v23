// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vdl

import (
	"errors"
)

var (
	errFromBool            = errors.New("method FromBool invalid for this target type")
	errFromUint            = errors.New("method FromUint invalid for this target type")
	errFromInt             = errors.New("method FromInt invalid for this target type")
	errFromFloat           = errors.New("method FromFloat invalid for this target type")
	errFromComplex         = errors.New("method FromComplex invalid for this target type")
	errFromBytes           = errors.New("method FromBytes invalid for this target type")
	errFromString          = errors.New("method FromString invalid for this target type")
	errFromEnumLabel       = errors.New("method FromEnumLabel invalid for this target type")
	errFromTypeObject      = errors.New("method FromTypeObject invalid for this target type")
	errFromZero            = errors.New("method FromZero invalid for this target type")
	errStartList           = errors.New("method StartList invalid for this target type")
	errFinishList          = errors.New("method FinishList invalid for this target type")
	errStartSet            = errors.New("method StartSet invalid for this target type")
	errFinishSet           = errors.New("method FinishSet invalid for this target type")
	errStartMap            = errors.New("method StartMap invalid for this target type")
	errFinishMap           = errors.New("method FinishMap invalid for this target type")
	errStartFields         = errors.New("method StartFields invalid for this target type")
	errFinishFields        = errors.New("method FinishFields invalid for this target type")
	errStartElem           = errors.New("method StartElem invalid for this target type")
	errFinishElem          = errors.New("method FinishElem invalid for this target type")
	errStartKey            = errors.New("method StartKey invalid for this target type")
	errFinishKey           = errors.New("method FinishKey invalid for this target type")
	errFinishKeyStartField = errors.New("method FinishKeyStartField invalid for this target type")
	errStartField          = errors.New("method StartField invalid for this target type")
	errFinishField         = errors.New("method FinishField invalid for this target type")
)

type TargetBase struct{}

var _ Target = (*TargetBase)(nil)

func (*TargetBase) FromBool(_ bool, _ *Type) error               { return errFromBool }
func (*TargetBase) FromUint(_ uint64, _ *Type) error             { return errFromUint }
func (*TargetBase) FromInt(_ int64, _ *Type) error               { return errFromInt }
func (*TargetBase) FromFloat(_ float64, _ *Type) error           { return errFromFloat }
func (*TargetBase) FromComplex(_ complex128, _ *Type) error      { return errFromComplex }
func (*TargetBase) FromBytes(_ []byte, _ *Type) error            { return errFromBytes }
func (*TargetBase) FromString(_ string, _ *Type) error           { return errFromString }
func (*TargetBase) FromEnumLabel(_ string, _ *Type) error        { return errFromEnumLabel }
func (*TargetBase) FromTypeObject(_ *Type) error                 { return errFromTypeObject }
func (*TargetBase) FromZero(_ *Type) error                       { return errFromZero }
func (*TargetBase) StartList(_ *Type, _ int) (ListTarget, error) { return nil, errStartList }
func (*TargetBase) FinishList(_ ListTarget) error                { return errFinishList }
func (*TargetBase) StartSet(_ *Type, _ int) (SetTarget, error)   { return nil, errStartSet }
func (*TargetBase) FinishSet(_ SetTarget) error                  { return errFinishSet }
func (*TargetBase) StartMap(_ *Type, _ int) (MapTarget, error)   { return nil, errStartMap }
func (*TargetBase) FinishMap(_ MapTarget) error                  { return errFinishMap }
func (*TargetBase) StartFields(_ *Type) (FieldsTarget, error)    { return nil, errStartFields }
func (*TargetBase) FinishFields(_ FieldsTarget) error            { return errFinishFields }

type ListTargetBase struct{}

var _ ListTarget = (*ListTargetBase)(nil)

func (*ListTargetBase) StartElem(_ int) (Target, error) { return nil, errStartElem }
func (*ListTargetBase) FinishElem(_ Target) error       { return errFinishElem }

type SetTargetBase struct{}

var _ SetTarget = (*SetTargetBase)(nil)

func (*SetTargetBase) StartKey() (Target, error) { return nil, errStartKey }
func (*SetTargetBase) FinishKey(_ Target) error  { return errFinishKey }

type MapTargetBase struct{}

var _ MapTarget = (*MapTargetBase)(nil)

func (*MapTargetBase) StartKey() (Target, error) { return nil, errStartKey }
func (*MapTargetBase) FinishKeyStartField(_ Target) (Target, error) {
	return nil, errFinishKeyStartField
}
func (*MapTargetBase) FinishField(_, _ Target) error { return errFinishField }

type FieldsTargetBase struct{}

var _ FieldsTarget = (*FieldsTargetBase)(nil)

func (*FieldsTargetBase) StartField(string) (Target, Target, error) { return nil, nil, errStartField }
func (*FieldsTargetBase) FinishField(_, _ Target) error             { return errFinishField }
