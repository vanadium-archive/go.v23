// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package naming

import (
	"strings"
)

// EndpointOpt must be implemented by all optional parameters to FormatEndpoint
type EndpointOpt interface {
	EndpointOpt()
}

// FormatEndpoint creates a string representation of an Endpoint using
// the supplied parameters. Network and address are always required,
// RoutingID, RPCVersionRange and ServesMountTable can be specified
// as options.
func FormatEndpoint(network, address string, opts ...EndpointOpt) string {
	rid := "@"
	var blessings []string
	mounttable := ""
	for _, o := range opts {
		switch v := o.(type) {
		case RoutingID:
			rid = "@" + v.String()
		case ServesMountTable:
			if bool(v) {
				mounttable = "m"
			} else {
				mounttable = "s"
			}
		case BlessingOpt:
			blessings = append(blessings, string(v))
		}
	}

	return "@5@" + network + "@" + address + rid + "@" + mounttable + "@" + strings.Join(blessings, ",") + "@@"
}
