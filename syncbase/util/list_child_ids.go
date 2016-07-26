// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package util

import (
	"strings"

	"v.io/v23"
	"v.io/v23/context"
	"v.io/v23/naming"
	wire "v.io/v23/services/syncbase"
	"v.io/v23/verror"
)

// ListChildIds returns a list of ids of all children of parentFullName. Glob is
// assumed to return the child ids sorted by blessing, then by name.
func ListChildIds(ctx *context.T, parentFullName string) ([]wire.Id, error) {
	ns := v23.GetNamespace(ctx)
	ch, err := ns.Glob(ctx, naming.Join(parentFullName, "*"))
	if err != nil {
		return nil, err
	}
	ids := []wire.Id{}
	for reply := range ch {
		switch v := reply.(type) {
		case *naming.GlobReplyEntry:
			encId := v.Value.Name[strings.LastIndex(v.Value.Name, "/")+1:]
			// Component ids within object names are always encoded. See comment in
			// server/dispatcher.go for explanation.
			id, err := DecodeId(encId)
			if err != nil {
				// If this happens, there's a bug in the Syncbase server. Glob should
				// return names with escaped components.
				return nil, verror.New(verror.ErrInternal, ctx, err)
			}
			ids = append(ids, id)
		case *naming.GlobReplyError:
			// Glob for a layer is currently all-or-nothing, so we fail on error.
			return nil, v.Value.Error
		}
	}
	return ids, nil
}
