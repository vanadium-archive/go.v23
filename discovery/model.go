// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package discovery defines types and interfaces for discovering services.
//
// TODO(jhahn): This is a work in progress and can change without notice.
package discovery

import (
	"v.io/v23/context"
	"v.io/v23/security"
)

// T is the interface for discovery operations; it is the client side library
// for the discovery service.
type T interface {
	Advertiser
	Scanner
	Closer
}

// Advertiser is the interface for advertising services.
type Advertiser interface {
	// Advertise advertises the service to be discovered by "Scanner" implementations.
	// visibility is used to limit the principals that can see the advertisement. An
	// empty set means that there are no restrictions on visibility (i.e, equivalent
	// to []security.BlessingPattern{security.AllPrincipals}). Advertising will continue
	// until the context is canceled or exceeds its deadline.
	//
	// It is an error to have simultaneously active advertisements for two identical
	// instances (service.InstanceUuid).
	Advertise(ctx *context.T, service Service, perms []security.BlessingPattern) error
}

// AdvertiseCloser is the interface that groups the Advertise and Close methods.
type AdvertiseCloser interface {
	Advertiser
	Closer
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

// ScanCloser is the interface that groups the Scan and Close methods.
type ScanCloser interface {
	Scanner
	Closer
}

// Closer is the interface that wraps the Close method.
type Closer interface {
	// Close closes all active tasks.
	Close()
}
