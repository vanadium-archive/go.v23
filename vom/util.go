// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vom

import (
	"bytes"
	"io"
	"os"
)

// Encode encodes the provided value using a new instance of a VOM encoder.
func Encode(value interface{}) ([]byte, error) {
	var buf bytes.Buffer
	encoder := NewEncoder(&buf)
	if err := encoder.Encode(value); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// Decode VOM-decodes the given data into the provided value using a new
// instance of a VOM decoder.
func Decode(data []byte, valptr interface{}) error {
	decoder := NewDecoder(bytes.NewReader(data))
	return decoder.Decode(valptr)
}

// This is only used for debugging; add this as the first line of NewDecoder to
// dump formatted vom bytes to stdout:
//   r = teeDump(r)
func teeDump(r io.Reader) io.Reader {
	return io.TeeReader(r, NewDumper(NewDumpWriter(os.Stdout)))
}
