// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package discovery defines types and interfaces for discovering services.
//
// TODO(jhahn): This is a work in progress and can change without notice.
package discovery

import (
	"v.io/v23/context"
	"v.io/v23/security/access"
)

// T is the interface for discovery operations; it is the client side library
// for the discovery service.
type T interface {
	Advertiser
	Scanner
}

// Advertiser is the interface for advertising services.
type Advertiser interface {
	// Advertise advertises the service. perms is used to limit the advertisement
	// of the service. Advertising will continue until the context is canceled or
	// exceeds its deadline.
	Advertise(ctx *context.T, service Service, perms access.Permissions) error
}

// Scanner is the interface for scanning services.
type Scanner interface {
	// Scan scans services that match the query and returns the channel on which
	// new discovered services can be read. Scanning will continue until the context
	// is canceled or exceeds its deadline.
	//
	// TODO(jhahn): Add query syntax and examples.
	Scan(ctx *context.T, query string) (<-chan Update, error)
}
