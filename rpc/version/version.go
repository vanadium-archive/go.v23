// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package version defines a mechanism for versioning the RPC protocol.
package version

// RPCVersion represents a version of the RPC protocol.
type RPCVersion uint32

const (
	// UnknownRPCVersion is used for Min/MaxRPCVersion in an Endpoint when
	// we don't know the relevant version numbers.  In this case the RPC
	// implementation will have to guess the correct values.
	UnknownRPCVersion RPCVersion = iota

	// DeprecatedRPCVersion is used to signal that a version number is no longer
	// relevant and that version information should be obtained elsewhere.
	DeprecatedRPCVersion

	rPCVersion2
	rPCVersion3
	rPCVersion4
	rPCVersion5
	rPCVersion6
	rPCVersion7
	rPCVersion8
	rPCVersion9

	// Open a special flow over which discharges for third-party
	// caveats on the server's blessings are sent.
	RPCVersion10

	// Optimized authentication.
	RPCVersion11
)

// RPCVersionRange allows you to optionally specify a range of versions to
// use when calling FormatEndpoint
type RPCVersionRange struct {
	Min, Max RPCVersion
}
