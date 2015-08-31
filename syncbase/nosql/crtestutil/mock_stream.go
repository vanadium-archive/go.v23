// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package crtestutil

import (
	"sync"

	wire "v.io/syncbase/v23/services/syncbase/nosql"
)

var _ wire.ConflictManagerStartConflictResolverClientCall = (*CrStreamImpl)(nil)

type State struct {
	// ConflictStream state
	IsBlocked    bool
	Mu           sync.Mutex
	Val          wire.ConflictInfo
	AdvanceCount int
	ValIndex     int
	// ResolutionStream state variables
	Result []wire.ResolutionInfo
}

type CrStreamImpl struct {
	C ConflictStream
	R ResolutionStream
}

func (s *CrStreamImpl) RecvStream() interface {
	Advance() bool
	Value() wire.ConflictInfo
	Err() error
} {
	return recvStreamImpl{s.C}
}

func (s *CrStreamImpl) SendStream() interface {
	Send(item wire.ResolutionInfo) error
	Close() error
} {
	return sendStreamImpl{s.R}
}

func (s *CrStreamImpl) Finish() error {
	return nil
}

type recvStreamImpl struct {
	c ConflictStream
}

func (rs recvStreamImpl) Advance() bool {
	return rs.c.Advance()
}
func (rs recvStreamImpl) Value() wire.ConflictInfo {
	return rs.c.Value()
}
func (rs recvStreamImpl) Err() error {
	return rs.c.Err()
}

type sendStreamImpl struct {
	r ResolutionStream
}

func (ss sendStreamImpl) Send(item wire.ResolutionInfo) error {
	return ss.r.Send(item)
}
func (c sendStreamImpl) Close() error {
	return nil
}

type ConflictStream interface {
	Advance() bool
	Value() wire.ConflictInfo
	Err() error
}

type ResolutionStream interface {
	Send(item wire.ResolutionInfo) error
}

type ConflictStreamImpl struct {
	St        *State
	AdvanceFn func(*State) bool
}

func (cs *ConflictStreamImpl) Advance() bool {
	return cs.AdvanceFn(cs.St)
}
func (cs *ConflictStreamImpl) Value() wire.ConflictInfo {
	return cs.St.Val
}
func (cs *ConflictStreamImpl) Err() error {
	return &TestError{"Stream broken"}
}

type ResolutionStreamImpl struct {
	St *State
}

func (rs *ResolutionStreamImpl) Send(item wire.ResolutionInfo) error {
	if rs.St.Result == nil {
		rs.St.Result = []wire.ResolutionInfo{item}
		return nil
	}
	rs.St.Result = append(rs.St.Result, item)
	return nil
}

type TestError struct {
	str string
}

func (e *TestError) Error() string {
	return e.str
}
