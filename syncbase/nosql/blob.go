// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package nosql

import (
	"sync"

	wire "v.io/syncbase/v23/services/syncbase/nosql"
	"v.io/v23/context"
	"v.io/v23/verror"
)

var _ Blob = (*blob)(nil)

type blob struct {
	c  wire.DatabaseClientMethods
	br wire.BlobRef
}

func newBlob(dbName string, br wire.BlobRef) Blob {
	return &blob{
		c:  wire.DatabaseClient(dbName),
		br: br,
	}
}

func createBlob(ctx *context.T, dbName string) (Blob, error) {
	b := &blob{
		c: wire.DatabaseClient(dbName),
	}
	var err error
	b.br, err = b.c.CreateBlob(ctx)
	return b, err
}

// Ref implements Blob.Ref.
func (b *blob) Ref() wire.BlobRef {
	return b.br
}

// Put implements Blob.Put.
func (b *blob) Put(ctx *context.T) (BlobWriter, error) {
	call, err := b.c.PutBlob(ctx, b.br)
	if err != nil {
		return nil, err
	}
	return newBlobWriter(call), nil
}

// Commit implements Blob.Commit.
func (b *blob) Commit(ctx *context.T) error {
	return b.c.CommitBlob(ctx, b.br)
}

// Size implements Blob.Size.
func (b *blob) Size(ctx *context.T) (int64, error) {
	return b.c.GetBlobSize(ctx, b.br)
}

// Delete implements Blob.Delete.
func (b *blob) Delete(ctx *context.T) error {
	return b.c.DeleteBlob(ctx, b.br)
}

// Get implements Blob.Get.
func (b *blob) Get(ctx *context.T, offset int64) (BlobReader, error) {
	ctx, cancel := context.WithCancel(ctx)
	call, err := b.c.GetBlob(ctx, b.br, offset)
	if err != nil {
		cancel()
		return nil, err
	}
	return newBlobReader(cancel, call), nil
}

// Fetch implements Blob.Fetch.
func (b *blob) Fetch(ctx *context.T, priority uint64) (BlobStatus, error) {
	ctx, cancel := context.WithCancel(ctx)
	call, err := b.c.FetchBlob(ctx, b.br, priority)
	if err != nil {
		cancel()
		return nil, err
	}
	return newBlobStatus(cancel, call), nil
}

// Pin implements Blob.Pin.
func (b *blob) Pin(ctx *context.T) error {
	return b.c.PinBlob(ctx, b.br)
}

// Unpin implements Blob.Unpin.
func (b *blob) Unpin(ctx *context.T) error {
	return b.c.UnpinBlob(ctx, b.br)
}

// Keep implements Blob.Keep.
func (b *blob) Keep(ctx *context.T, rank uint64) error {
	return b.c.KeepBlob(ctx, b.br, rank)
}

////////////////////////////////////////
// BlobWriter methods.

type blobWriter struct {
	mu sync.Mutex
	// call is the RPC stream object.
	call wire.BlobManagerPutBlobClientCall
	// finished records whether we have called call.Finish().
	finished bool
}

var _ BlobWriter = (*blobWriter)(nil)

func newBlobWriter(call wire.BlobManagerPutBlobClientCall) *blobWriter {
	return &blobWriter{
		call: call,
	}
}

// Send implements BlobWriter.Send.
func (bw *blobWriter) Send(buf []byte) error {
	return bw.call.SendStream().Send(buf)
}

// Close implements BlobWriter.Close.
func (bw *blobWriter) Close() error {
	bw.mu.Lock()
	defer bw.mu.Unlock()
	if !bw.finished {
		// No need to call Close explicitly. Finish will take care of
		// that.
		bw.finished = true
		return bw.call.Finish()
	}
	return nil
}

////////////////////////////////////////
// BlobReader methods.
// (similar to methods in stream.go).

type blobReader struct {
	mu sync.Mutex
	// cancel cancels the RPC stream.
	cancel context.CancelFunc
	// call is the RPC stream object.
	call wire.BlobManagerGetBlobClientCall
	// curr is the currently staged bytes, or nil if nothing is staged.
	curr []byte
	// err is the first error encountered during streaming. It may also be
	// populated by a call to Cancel.
	err error
	// finished records whether we have called call.Finish().
	finished bool
}

var _ BlobReader = (*blobReader)(nil)

func newBlobReader(cancel context.CancelFunc, call wire.BlobManagerGetBlobClientCall) *blobReader {
	return &blobReader{
		cancel: cancel,
		call:   call,
	}
}

// Advance implements BlobReader.Advance.
func (br *blobReader) Advance() bool {
	br.mu.Lock()
	defer br.mu.Unlock()
	if br.err != nil || br.finished {
		return false
	}
	if br.call.RecvStream().Advance() {
		br.curr = br.call.RecvStream().Value()
		return true
	}

	br.err = br.call.RecvStream().Err()
	err := br.call.Finish()
	if br.err == nil {
		br.err = err
	}
	br.cancel()
	br.finished = true
	return false
}

// Value implements BlobReader.Value.
func (br *blobReader) Value() []byte {
	br.mu.Lock()
	defer br.mu.Unlock()
	if br.curr == nil {
		panic("nothing staged")
	}
	return br.curr
}

// Err implements BlobReader.Err.
func (br *blobReader) Err() error {
	br.mu.Lock()
	defer br.mu.Unlock()
	return br.err
}

// Cancel implements BlobReader.Cancel.
// TODO(hpucha): Make Cancel non-blocking. Copied from stream.go
func (br *blobReader) Cancel() {
	br.mu.Lock()
	defer br.mu.Unlock()
	br.cancel()
	br.call.Finish()
	br.err = verror.New(verror.ErrCanceled, nil)
}

////////////////////////////////////////
// BlobStatus methods.
// (similar to methods in stream.go).

type blobStatus struct {
	mu sync.Mutex
	// cancel cancels the RPC stream.
	cancel context.CancelFunc
	// call is the RPC stream object.
	call wire.BlobManagerFetchBlobClientCall
	// curr is the currently staged item, or nil if nothing is staged.
	curr *wire.BlobFetchStatus
	// err is the first error encountered during streaming. It may also be
	// populated by a call to Cancel.
	err error
	// finished records whether we have called call.Finish().
	finished bool
}

var _ BlobStatus = (*blobStatus)(nil)

func newBlobStatus(cancel context.CancelFunc, call wire.BlobManagerFetchBlobClientCall) *blobStatus {
	return &blobStatus{
		cancel: cancel,
		call:   call,
	}
}

// Advance implements BlobStatus.Advance.
func (bs *blobStatus) Advance() bool {
	bs.mu.Lock()
	defer bs.mu.Unlock()
	if bs.err != nil || bs.finished {
		return false
	}
	if bs.call.RecvStream().Advance() {
		val := bs.call.RecvStream().Value()
		bs.curr = &val
		return true
	}

	bs.err = bs.call.RecvStream().Err()
	err := bs.call.Finish()
	if bs.err == nil {
		bs.err = err
	}
	bs.cancel()
	bs.finished = true
	return false
}

// Value implements BlobStatus.Value.
func (bs *blobStatus) Value() wire.BlobFetchStatus {
	bs.mu.Lock()
	defer bs.mu.Unlock()
	if bs.curr == nil {
		panic("nothing staged")
	}
	return *bs.curr
}

// Err implements BlobStatus.Err.
func (bs *blobStatus) Err() error {
	bs.mu.Lock()
	defer bs.mu.Unlock()
	return bs.err
}

// Cancel implements BlobStatus.Cancel.
// TODO(hpucha): Make Cancel non-blocking. Copied from stream.go
func (bs *blobStatus) Cancel() {
	bs.mu.Lock()
	defer bs.mu.Unlock()
	bs.cancel()
	bs.call.Finish()
	bs.err = verror.New(verror.ErrCanceled, nil)
}
