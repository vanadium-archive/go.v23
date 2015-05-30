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
)

// List does namespace.Glob("name/*") and returns a sorted slice of results or
// a VDL-compatible error.
func List(ctx *context.T, name string) ([]string, error) {
	// TODO(sadovsky): Why can't Glob be a method on the stub, just like every
	// other streaming method? Even if we encourage clients to use the namespace
	// library to glob, we ought to make the underlying RPC implementation
	// understandable and familiar.
	ns := v23.GetNamespace(ctx)
	ch, err := ns.Glob(ctx, naming.Join(name, "*"))
	if err != nil {
		return nil, err
	}

	names := []string{}
	for globReply := range ch {
		switch v := globReply.(type) {
		case *naming.GlobReplyEntry:
			// NOTE(nlacasse): The names that come back from Glob are all
			// rooted.  We only want the last part of the name, so we must chop
			// off everything before the final '/'.  Since endpoints can
			// themselves contain slashes, we have to remove the endpoint from
			// the name first.
			_, name := naming.SplitAddressName(v.Value.Name)
			names = append(names, name[strings.LastIndex(name, "/")+1:])
		case *naming.GlobReplyError:
			return nil, v.Value.Error
		}
	}
	sort.Strings(names)
	return names, nil
}
