// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package util

import (
	"sort"
	"strings"

	"v.io/v23"
	"v.io/v23/context"
	"v.io/v23/naming"
	"v.io/v23/verror"
)

// ListChildren returns the relative names of all children of parentFullName.
func ListChildren(ctx *context.T, parentFullName string) ([]string, error) {
	ns := v23.GetNamespace(ctx)
	ch, err := ns.Glob(ctx, naming.Join(parentFullName, "*"))
	if err != nil {
		return nil, err
	}
	names := []string{}
	for globReply := range ch {
		switch v := globReply.(type) {
		case *naming.GlobReplyEntry:
			escName := v.Value.Name[strings.LastIndex(v.Value.Name, "/")+1:]
			// Component names within object names are always escaped. See comment in
			// server/dispatcher.go for explanation.
			name, ok := Unescape(escName)
			if !ok {
				// If this happens, there's a bug in the Syncbase server. Glob should
				// return names with escaped components.
				return nil, verror.New(verror.ErrInternal, ctx, escName)
			}
			names = append(names, name)
		case *naming.GlobReplyError:
			// TODO(sadovsky): Surface these errors somehow.
		}
	}
	sort.Strings(names)
	return names, nil
}
