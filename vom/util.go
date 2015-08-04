// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vom

import (
	"bytes"
	"sync"

	"v.io/v23/verror"
)

// Encode writes the value v and returns the encoded bytes.  The semantics of
// value encoding are described by Encoder.Encode.
//
// This is a "single-shot" encoding; full type information is always included in
// the returned encoding, as if a new encoder were used for each call.
func Encode(v interface{}) ([]byte, error) {
	var buf bytes.Buffer
	if err := NewEncoder(&buf).Encode(v); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// Decode reads the value from the given data, and stores it in value v.  The
// semantics of value decoding are described by Decoder.Decode.
//
// This is a "single-shot" decoding; the data must have been encoded by a call
// to vom.Encode.
func Decode(data []byte, v interface{}) error {
	// The implementation below corresponds (logically) to the following:
	//   return NewDecoder(bytes.NewReader(data)).Decode(valptr)
	//
	// However decoding type messages is expensive, so we cache typeDecoders to
	// skip the decoding in the common case.
	buf := newDecbufFromBytes(data)
	if err := readVersionByte(buf); err != nil {
		return err
	}
	startTypeMsgs := buf.beg
	// Check for hits in the typeDecoder cache.
	key, err := computeTypeDecoderCacheKey(buf)
	if err != nil {
		return err
	}
	typeDec := singleShotTypeDecoderCache.lookup(key)
	cacheMiss := false
	if typeDec == nil {
		// Cache miss; start decoding at the beginning of all type messages with a
		// new TypeDecoder.
		cacheMiss = true
		buf.beg = startTypeMsgs
		typeDec = newTypeDecoderWithoutVersionByte(nil)
	}
	// Decode the value message.
	decoder := newDecoderWithoutVersionByte(buf, typeDec)
	if err := decoder.Decode(v); err != nil {
		return err
	}
	// Populate the typeDecoder cache for future re-use.
	if cacheMiss {
		singleShotTypeDecoderCache.insert(key, typeDec)
	}
	return nil
}

// singleShotTypeDecoderCache is a global cache of TypeDecoders keyed by the
// bytes of the sequence of type messages before the value message.  A sequence
// of type messages that is byte-equal doesn't guarantee the same types in the
// general case, since type ids are scoped to a single Encoder/Decoder stream;
// different streams may represent different types using the same type ids.
// However the single-shot vom.Encode is guaranteed to generate the same bytes
// for a given vom version.
//
// TODO(toddw): This cache grows without bounds, use a fixed-size cache instead.
var singleShotTypeDecoderCache = typeDecoderCache{
	decoders: make(map[string]*TypeDecoder),
}

type typeDecoderCache struct {
	sync.RWMutex
	decoders map[string]*TypeDecoder
}

func (x *typeDecoderCache) insert(key string, dec *TypeDecoder) {
	x.Lock()
	defer x.Unlock()
	if x.decoders[key] != nil {
		// There was a race between concurrent calls to vom.Decode, and another
		// goroutine already populated the cache.  It doesn't matter which one we
		// use, so there's nothing more to do.
		return
	}
	x.decoders[key] = dec
}

func (x *typeDecoderCache) lookup(key string) *TypeDecoder {
	x.RLock()
	dec := x.decoders[key]
	x.RUnlock()
	return dec
}

// computeTypeDecoderCacheKey computes the cache key for the typeDecoderCache.
// The logic is similar to Decoder.decodeValueType.
func computeTypeDecoderCacheKey(buf *decbuf) (string, error) {
	// Walk through buf until we get to a value message.
	for {
		switch id, bytelen, err := binaryPeekInt(buf); {
		case err != nil:
			return "", err
		case id > 0:
			// This is a value message.  The bytes read so far include the version
			// byte and all type messages; use all of these bytes as the cache key.
			//
			// TODO(toddw): Take a fingerprint of these bytes to reduce memory usage.
			return string(buf.buf[0:buf.beg]), nil
		case id < 0:
			// This is a type message.  Skip the bytes for the id, and decode the
			// message length (which always exists for wireType), and skip those bytes
			// too to move to the next message.
			buf.Skip(bytelen)
			msgLen, err := binaryDecodeLen(buf)
			if err != nil {
				return "", err
			}
			if err := buf.Skip(msgLen); err != nil {
				return "", err
			}
		case id == 0:
			return "", verror.New(errDecodeZeroTypeID, nil)
		}
	}
}
