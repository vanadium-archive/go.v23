// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package namespace supports resolving object names to server object addresses.
package namespace

import (
	"time"

	"v.io/v23/context"
	"v.io/v23/naming"
	"v.io/v23/security/access"
)

// T is the interface for namespace operations; it is the client side library
// for the MountTable service
type T interface {
	// Mount the server object address under the object name, expiring after
	// the ttl. ttl of zero implies an implementation-specific high value
	// (essentially, forever).
	Mount(ctx *context.T, name, server string, ttl time.Duration, opts ...naming.NamespaceOpt) error

	// Unmount the server object address from the object name, or if server
	// is empty, unmount all server OAs from the object name.
	Unmount(ctx *context.T, name, server string, opts ...naming.NamespaceOpt) error

	// Delete the name from a mount table.  If the name has any children in its mount table,
	// it (and its chidren) will only be removed if deleteSubtree is true.
	Delete(ctx *context.T, name string, deleteSubtree bool, opts ...naming.NamespaceOpt) error

	// Resolve the object name into its mounted servers.
	Resolve(ctx *context.T, name string, opts ...naming.NamespaceOpt) (entry *naming.MountEntry, err error)

	// ResolveToMountTable resolves the object name into the mounttables
	// directly responsible for the name.
	ResolveToMountTable(ctx *context.T, name string, opts ...naming.NamespaceOpt) (entry *naming.MountEntry, err error)

	// FlushCacheEntry flushes resolution information cached for the name.  If
	// anything was flushed it returns true.
	FlushCacheEntry(name string) bool

	// CacheCtl sets controls and returns the current control values.
	CacheCtl(ctls ...naming.CacheCtl) []naming.CacheCtl

	// Glob returns MountEntry's whose name matches the pattern and GlobError's
	// for any piece of the space that can't be traversed.
	//
	// Two special patterns:
	//   prefix/... means all names below prefix.
	//   prefix/*** is like prefix/... but doesn't traverse into non-mounttable servers.
	//
	// Example:
	//	rc, err := ns.Glob(ctx, pattern)
	//	if err != nil {
	//		boom(t, "Glob(%s): %s", pattern, err)
	//	}
	//	for s := range rc {
	//		switch v := s.(type) {
	//		case *naming.MountEntry:
	//			fmt.Printf("%s: %v\n", v.Name, v.Servers)
	//		case *naming.GlobError:
	//			fmt.Fprintf(stderr, "%s can't be traversed: %s\n", v.Name, v.Error)
	//		}
	//	}
	Glob(ctx *context.T, pattern string, opts ...naming.NamespaceOpt) (chan interface{}, error)

	// SetRoots sets the roots that the local Namespace is
	// relative to. All relative names passed to the methods above
	// will be interpreted as relative to these roots. The roots
	// will be tried in the order that they are specified in the parameter
	// list for SetRoots. Calling SetRoots with no arguments will clear the
	// currently configured set of roots.
	SetRoots(roots ...string) error

	// Roots returns the currently configured roots. An empty slice is
	// returned if no roots are configured.
	Roots() []string

	// SetPermissions sets the AccessList in a node in a mount table.
	SetPermissions(ctx *context.T, name string, acl access.Permissions, etag string, opts ...naming.NamespaceOpt) error

	// GetPermissions returns the AccessList in a node in a mount table.
	GetPermissions(ctx *context.T, name string, opts ...naming.NamespaceOpt) (acl access.Permissions, etag string, err error)
}
