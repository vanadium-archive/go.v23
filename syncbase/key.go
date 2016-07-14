// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package syncbase

import (
	"crypto/rand"
	"fmt"
	"io"
)

// UUID generates a Version 4 UUID, based on random numbers (RFC 4122).
// Returned as hex-encoded String in format 'xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx'.
func UUID() string {
	uuid := make([]byte, 16)
	io.ReadFull(rand.Reader, uuid)

	// Set the version 4 bit.
	uuid[6] = (uuid[6] & 0x0f) | 0x40

	// Set the varint bit to 10 as required by RFC 4122.
	uuid[8] = (uuid[8] & 0x3f) | 0x80

	return fmt.Sprintf("%x-%x-%x-%x-%x", uuid[:4], uuid[4:6], uuid[6:8], uuid[8:10], uuid[10:])
}
