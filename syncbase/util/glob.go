// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package util

import (
	"io"
	"sort"

	"v.io/v23"
	"v.io/v23/context"
	"v.io/v23/naming"
	"v.io/v23/rpc"
)

// List does client.Glob("*") and returns a sorted slice of results or a
// VDL-compatible error.
func List(ctx *context.T, name string) ([]string, error) {
	client := v23.GetClient(ctx)
	// TODO(sadovsky): This is nuts. Why is Glob not a method on the stub, just
	// like every other streaming method?
	call, err := client.StartCall(ctx, name, rpc.GlobMethod, []interface{}{"*"})
	if err != nil {
		return nil, err
	}
	res := []string{}
	done := false
	for !done {
		var gr naming.GlobReply
		switch err := call.Recv(&gr); err {
		case nil:
			switch v := gr.(type) {
			case naming.GlobReplyEntry:
				res = append(res, v.Value.Name)
			case naming.GlobReplyError:
				return nil, v.Value.Error
			}
		case io.EOF:
			done = true
		default:
			return nil, err
		}
	}
	if err := call.Finish(); err != nil {
		return nil, err
	}
	sort.Strings(res)
	return res, nil
}
