// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package message

import (
	"v.io/v23"
	"v.io/v23/context"
	"v.io/v23/naming"
	"v.io/v23/rpc/version"
	"v.io/v23/security"
)

// TODO(mattr): Link to protocol doc.

func Append(ctx *context.T, m Message, to []byte) ([]byte, error) {
	return m.append(ctx, to)
}

func Read(ctx *context.T, from []byte) (Message, error) {
	if len(from) == 0 {
		return nil, NewErrInvalidMsg(ctx, invalidType, 0, 0, nil)
	}
	msgType, from := from[0], from[1:]
	var m Message
	switch msgType {
	case setupType:
		m = &Setup{}
	case tearDownType:
		m = &TearDown{}
	case authType:
		m = &Auth{}
	case openFlowType:
		m = &OpenFlow{}
	case releaseType:
		m = &Release{}
	case dataType:
		m = &Data{}
	default:
		return nil, NewErrUnknownMsg(ctx, msgType)
	}
	return m, m.read(ctx, from)
}

type Message interface {
	append(ctx *context.T, data []byte) ([]byte, error)
	read(ctx *context.T, data []byte) error
}

// message types.
const (
	invalidType = iota
	setupType
	tearDownType
	authType
	openFlowType
	releaseType
	dataType
)

// setup options.
const (
	invalidOption = iota
	peerNaClPublicKeyOption
	peerRemoteEndpointOption
	peerLocalEndpointOption
)

// data flags.
const (
	CloseFlag = 1 << iota
)

// random consts.
const (
	maxVarUint64Size = 9
)

// setup is the first message over the wire.  It negotiates protocol version
// and encryption options for connection.
type Setup struct {
	Versions           version.RPCVersionRange
	PeerNaClPublicKey  *[32]byte
	PeerRemoteEndpoint naming.Endpoint
	PeerLocalEndpoint  naming.Endpoint
}

func appendSetupOption(option uint64, payload, buf []byte) []byte {
	return appendLenBytes(payload, writeVarUint64(option, buf))
}
func readSetupOption(ctx *context.T, orig []byte) (opt uint64, p, d []byte, err error) {
	var valid bool
	if opt, d, valid = readVarUint64(ctx, orig); !valid {
		err = NewErrInvalidSetupOption(ctx, invalidOption, 0)
	} else if p, d, valid = readLenBytes(ctx, d); !valid {
		err = NewErrInvalidSetupOption(ctx, opt, 1)
	}
	return
}

func (m *Setup) append(ctx *context.T, data []byte) ([]byte, error) {
	data = append(data, setupType)
	data = writeVarUint64(uint64(m.Versions.Min), data)
	data = writeVarUint64(uint64(m.Versions.Max), data)
	if m.PeerNaClPublicKey != nil {
		data = appendSetupOption(peerNaClPublicKeyOption,
			m.PeerNaClPublicKey[:], data)
	}
	if m.PeerRemoteEndpoint != nil {
		data = appendSetupOption(peerRemoteEndpointOption,
			[]byte(m.PeerRemoteEndpoint.String()), data)
	}
	if m.PeerLocalEndpoint != nil {
		data = appendSetupOption(peerLocalEndpointOption,
			[]byte(m.PeerLocalEndpoint.String()), data)
	}
	return data, nil
}
func (m *Setup) read(ctx *context.T, orig []byte) error {
	var (
		data  = orig
		valid bool
		v     uint64
	)
	if v, data, valid = readVarUint64(ctx, data); !valid {
		return NewErrInvalidMsg(ctx, setupType, uint64(len(orig)), 0, nil)
	}
	m.Versions.Min = version.RPCVersion(v)
	if v, data, valid = readVarUint64(ctx, data); !valid {
		return NewErrInvalidMsg(ctx, setupType, uint64(len(orig)), 1, nil)
	}
	m.Versions.Max = version.RPCVersion(v)
	for field := uint64(2); len(data) > 0; field++ {
		var (
			payload []byte
			option  uint64
			err     error
		)
		if option, payload, data, err = readSetupOption(ctx, data); err != nil {
			return NewErrInvalidMsg(ctx, setupType, uint64(len(orig)), field, err)
		}
		switch option {
		case peerNaClPublicKeyOption:
			m.PeerNaClPublicKey = new([32]byte)
			copy(m.PeerNaClPublicKey[:], payload)
		case peerRemoteEndpointOption:
			m.PeerRemoteEndpoint, err = v23.NewEndpoint(string(payload))
		case peerLocalEndpointOption:
			m.PeerLocalEndpoint, err = v23.NewEndpoint(string(payload))
		default:
			// Ignore unknown setup options.
		}
		if err != nil {
			return NewErrInvalidMsg(ctx, setupType, uint64(len(orig)), field, err)
		}
	}
	return nil
}

// tearDown is sent over the wire before a connection is closed.
type TearDown struct {
	Message string
}

func (m *TearDown) append(ctx *context.T, data []byte) ([]byte, error) {
	data = append(data, tearDownType)
	return append(data, []byte(m.Message)...), nil
}
func (m *TearDown) read(ctx *context.T, data []byte) error {
	m.Message = string(data)
	return nil
}

// auth is used to complete the auth handshake.
type Auth struct {
	BlessingsKey, DischargeKey uint64
	ChannelBinding             security.Signature
	PublicKey                  security.PublicKey
}

func (m *Auth) append(ctx *context.T, data []byte) ([]byte, error) {
	data = append(data, authType)
	data = writeVarUint64(m.BlessingsKey, data)
	data = writeVarUint64(m.DischargeKey, data)
	data = appendLenBytes(m.ChannelBinding.Purpose, data)
	data = appendLenBytes([]byte(m.ChannelBinding.Hash), data)
	data = appendLenBytes(m.ChannelBinding.R, data)
	data = appendLenBytes(m.ChannelBinding.S, data)
	if m.PublicKey != nil {
		pk, err := m.PublicKey.MarshalBinary()
		if err != nil {
			return data, err
		}
		data = append(data, pk...)
	}
	return data, nil
}
func (m *Auth) read(ctx *context.T, orig []byte) error {
	var data, tmp []byte
	var valid bool
	if m.BlessingsKey, data, valid = readVarUint64(ctx, orig); !valid {
		return NewErrInvalidMsg(ctx, openFlowType, uint64(len(orig)), 0, nil)
	}
	if m.DischargeKey, data, valid = readVarUint64(ctx, data); !valid {
		return NewErrInvalidMsg(ctx, openFlowType, uint64(len(orig)), 1, nil)
	}
	if m.ChannelBinding.Purpose, data, valid = readLenBytes(ctx, data); !valid {
		return NewErrInvalidMsg(ctx, openFlowType, uint64(len(orig)), 2, nil)
	}
	if tmp, data, valid = readLenBytes(ctx, data); !valid {
		return NewErrInvalidMsg(ctx, openFlowType, uint64(len(orig)), 3, nil)
	}
	m.ChannelBinding.Hash = security.Hash(tmp)
	if m.ChannelBinding.R, data, valid = readLenBytes(ctx, data); !valid {
		return NewErrInvalidMsg(ctx, openFlowType, uint64(len(orig)), 4, nil)
	}
	if m.ChannelBinding.S, data, valid = readLenBytes(ctx, data); !valid {
		return NewErrInvalidMsg(ctx, openFlowType, uint64(len(orig)), 5, nil)
	}
	if len(data) > 0 {
		var err error
		m.PublicKey, err = security.UnmarshalPublicKey(data)
		return err
	}
	return nil
}

// openFlow is sent at the beginning of every new flow.
type OpenFlow struct {
	ID                         uint64
	InitialCounters            uint64
	BlessingsKey, DischargeKey uint64
}

func (m *OpenFlow) append(ctx *context.T, data []byte) ([]byte, error) {
	data = append(data, openFlowType)
	data = writeVarUint64(m.ID, data)
	data = writeVarUint64(m.InitialCounters, data)
	data = writeVarUint64(m.BlessingsKey, data)
	return writeVarUint64(m.DischargeKey, data), nil
}
func (m *OpenFlow) read(ctx *context.T, orig []byte) error {
	var (
		data  []byte
		valid bool
	)
	if m.ID, data, valid = readVarUint64(ctx, orig); !valid {
		return NewErrInvalidMsg(ctx, openFlowType, uint64(len(orig)), 0, nil)
	}
	if m.InitialCounters, data, valid = readVarUint64(ctx, data); !valid {
		return NewErrInvalidMsg(ctx, openFlowType, uint64(len(orig)), 1, nil)
	}
	if m.BlessingsKey, data, valid = readVarUint64(ctx, data); !valid {
		return NewErrInvalidMsg(ctx, openFlowType, uint64(len(orig)), 2, nil)
	}
	if m.DischargeKey, data, valid = readVarUint64(ctx, data); !valid {
		return NewErrInvalidMsg(ctx, openFlowType, uint64(len(orig)), 3, nil)
	}
	return nil
}

// release is sent as flows are read from locally.  The counters
// inform remote writers that there is local buffer space available.
type Release struct {
	Counters map[uint64]uint64
}

func (m *Release) append(ctx *context.T, data []byte) ([]byte, error) {
	data = append(data, releaseType)
	for fid, val := range m.Counters {
		data = writeVarUint64(fid, data)
		data = writeVarUint64(val, data)
	}
	return data, nil
}
func (m *Release) read(ctx *context.T, orig []byte) error {
	var (
		data     = orig
		valid    bool
		fid, val uint64
		n        uint64
	)
	if len(data) == 0 {
		return nil
	}
	m.Counters = map[uint64]uint64{}
	for len(data) > 0 {
		if fid, data, valid = readVarUint64(ctx, data); !valid {
			return NewErrInvalidMsg(ctx, releaseType, uint64(len(orig)), n, nil)
		}
		if val, data, valid = readVarUint64(ctx, data); !valid {
			return NewErrInvalidMsg(ctx, releaseType, uint64(len(orig)), n+1, nil)
		}
		m.Counters[fid] = val
		n += 2
	}
	return nil
}

// data carries encrypted data for a specific flow.
type Data struct {
	ID      uint64
	Flags   uint64
	Payload [][]byte
}

func (m *Data) append(ctx *context.T, data []byte) ([]byte, error) {
	data = append(data, dataType)
	data = writeVarUint64(m.ID, data)
	data = writeVarUint64(m.Flags, data)
	for _, p := range m.Payload {
		data = append(data, p...)
	}
	return data, nil
}
func (m *Data) read(ctx *context.T, orig []byte) error {
	var (
		data  []byte
		valid bool
	)
	if m.ID, data, valid = readVarUint64(ctx, orig); !valid {
		return NewErrInvalidMsg(ctx, dataType, uint64(len(orig)), 0, nil)
	}
	if m.Flags, data, valid = readVarUint64(ctx, data); !valid {
		return NewErrInvalidMsg(ctx, dataType, uint64(len(orig)), 1, nil)
	}
	if len(data) > 0 {
		m.Payload = [][]byte{data}
	}
	return nil
}

func appendLenBytes(b []byte, buf []byte) []byte {
	buf = writeVarUint64(uint64(len(b)), buf)
	return append(buf, b...)
}

func readLenBytes(ctx *context.T, data []byte) (b, rest []byte, valid bool) {
	l, data, valid := readVarUint64(ctx, data)
	if !valid || uint64(len(data)) < l {
		return nil, data, false
	}
	if l > 0 {
		return data[:l], data[l:], true
	}
	return nil, data, true
}

func readVarUint64(ctx *context.T, data []byte) (uint64, []byte, bool) {
	if len(data) == 0 {
		return 0, data, false
	}
	l := data[0]
	if l <= 0x7f {
		return uint64(l), data[1:], true
	}
	l = 0xff - l + 1
	if l > 8 || len(data)-1 < int(l) {
		return 0, data, false
	}
	var out uint64
	for i := 1; i < int(l+1); i++ {
		out = out<<8 | uint64(data[i])
	}
	return out, data[l+1:], true
}

func writeVarUint64(u uint64, buf []byte) []byte {
	if u <= 0x7f {
		return append(buf, byte(u))
	}
	shift, l := 56, byte(7)
	for ; shift >= 0 && (u>>uint(shift))&0xff == 0; shift, l = shift-8, l-1 {
	}
	buf = append(buf, 0xff-l)
	for ; shift >= 0; shift -= 8 {
		buf = append(buf, byte(u>>uint(shift))&0xff)
	}
	return buf
}
