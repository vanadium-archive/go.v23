// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package naming

import (
	"strconv"
	"strings"

	"v.io/v23/rpc/version"
)

// EndpointOpt must be implemented by all optional parameters to FormatEndpoint
type EndpointOpt interface {
	EndpointOpt()
}

// FormatEndpoint creates a string representation of an Endpoint using
// the supplied parameters. Network and address are always required,
// RoutingID, RPCVersionRange and ServesMountTableOpt can be specified
// as options.
func FormatEndpoint(network, address string, opts ...EndpointOpt) string {
	rid := "@"
	versions := "@@"
	var blessings []string
	servesMountTable := false
	for _, o := range opts {
		switch v := o.(type) {
		case RoutingID:
			rid = "@" + v.String()
		case version.RPCVersionRange:
			versions = "@" + strconv.FormatUint(uint64(v.Min), 10) +
				"@" + strconv.FormatUint(uint64(v.Max), 10)
		case ServesMountTableOpt:
			servesMountTable = bool(v)
		case BlessingOpt:
			blessings = append(blessings, string(v))
		}
	}
	if len(blessings) > 0 {
		mORs := "s"
		if servesMountTable {
			mORs = "m"
		}
		// "," is chosen as the separator for blessings
		// because it is an invalid substring in blessings.
		return "@4@" + network + "@" + address + rid + versions + "@" + mORs + "@" + strings.Join(blessings, ",") + "@@"
	}
	if servesMountTable {
		return "@3@" + network + "@" + address + rid + versions + "@" + "m" + "@@"
	}
	// For now, only use the v3 endpoint when we need to - i.e. only
	// mount tables will export v3 endpoints.
	return "@2@" + network + "@" + address + rid + versions + "@@"
}
