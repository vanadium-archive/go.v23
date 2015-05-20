// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package testutil

import (
	"fmt"
	"reflect"
	"testing"

	"v.io/syncbase/v23/syncbase"
	"v.io/syncbase/v23/syncbase/nosql"
	"v.io/syncbase/v23/syncbase/util"
	"v.io/v23/context"
	"v.io/v23/security"
	"v.io/v23/security/access"
	"v.io/v23/verror"
	"v.io/x/lib/vlog"
)

// TestCreate tests that object creation works as expected.
func TestCreate(t *testing.T, ctx *context.T, i interface{}) {
	parent := makeLayer(i)
	self := parent.Child("self")
	child := self.Child("child")

	// child.Create should fail since self does not exist.
	if err := child.Create(ctx, nil); verror.ErrorID(err) != verror.ErrNoExistOrNoAccess.ID {
		t.Fatalf("child.Create() should have failed: %v", err)
	}

	// Create self.
	if err := self.Create(ctx, nil); err != nil {
		t.Fatalf("self.Create() failed: %v", err)
	}
	if gotPerms, wantPerms := getPermsOrDie(t, ctx, self), defaultPerms(); !reflect.DeepEqual(gotPerms, wantPerms) {
		t.Errorf("Perms do not match: got %v, want %v", gotPerms, wantPerms)
	}

	// child.Create should now succeed.
	if err := child.Create(ctx, nil); err != nil {
		t.Fatalf("child.Create() failed: %v", err)
	}

	// self.Create should fail since self already exists.
	if err := self.Create(ctx, nil); verror.ErrorID(err) != verror.ErrExist.ID {
		t.Fatalf("self.Create() should have failed: %v", err)
	}

	// Test create with non-default perms.
	self2 := parent.Child("self2")
	perms := access.Permissions{}
	perms.Add(security.BlessingPattern("server/client"), string(access.Admin))
	if err := self2.Create(ctx, perms); err != nil {
		t.Fatalf("self2.Create() failed: %v", err)
	}
	if gotPerms, wantPerms := getPermsOrDie(t, ctx, self2), perms; !reflect.DeepEqual(gotPerms, wantPerms) {
		t.Errorf("Perms do not match: got %v, want %v", gotPerms, wantPerms)
	}

	// Test that create fails if the parent perms disallow access.
	perms = defaultPerms()
	perms.Blacklist("server/client", string(access.Write))
	if err := parent.SetPermissions(ctx, perms, ""); err != nil {
		t.Fatalf("parent.SetPermissions() failed: %v", err)
	}
	if err := parent.Child("self3").Create(ctx, nil); verror.ErrorID(err) != verror.ErrNoExistOrNoAccess.ID {
		t.Fatalf("self3.Create() should have failed: %v", err)
	}
}

// TestDelete tests that object deletion works as expected.
func TestDelete(t *testing.T, ctx *context.T, i interface{}) {
	parent := makeLayer(i)
	self := parent.Child("self")
	child := self.Child("child")

	// Create self.
	if err := self.Create(ctx, nil); err != nil {
		t.Fatalf("self.Create() failed: %v", err)
	}

	// self.Create should fail, since self already exists.
	if err := self.Create(ctx, nil); verror.ErrorID(err) != verror.ErrExist.ID {
		t.Fatalf("self.Create() should have failed: %v", err)
	}

	// By default, self perms are copied from parent, so self.Delete should
	// succeed.
	if err := self.Delete(ctx); err != nil {
		t.Fatalf("self.Delete() failed: %v", err)
	}

	// child.Create should fail, since self does not exist.
	if err := child.Create(ctx, nil); verror.ErrorID(err) != verror.ErrNoExistOrNoAccess.ID {
		t.Fatalf("child.Create() should have failed: %v", err)
	}

	// self.Create should succeed, since self was deleted.
	if err := self.Create(ctx, nil); err != nil {
		t.Fatalf("self.Create() failed: %v", err)
	}

	// Test that delete fails if the perms disallow access.
	self2 := parent.Child("self2")
	if err := self2.Create(ctx, nil); err != nil {
		t.Fatalf("self2.Create() failed: %v", err)
	}
	perms := defaultPerms()
	perms.Blacklist("server/client", string(access.Write))
	if err := self2.SetPermissions(ctx, perms, ""); err != nil {
		t.Fatalf("self2.SetPermissions() failed: %v", err)
	}
	if err := self2.Delete(ctx); verror.ErrorID(err) != verror.ErrNoExistOrNoAccess.ID {
		t.Fatalf("self2.Delete() should have failed: %v", err)
	}

	// Test that delete succeeds even if the parent perms disallow access.
	perms = defaultPerms()
	perms.Blacklist("server/client", string(access.Write))
	if err := parent.SetPermissions(ctx, perms, ""); err != nil {
		t.Fatalf("parent.SetPermissions() failed: %v", err)
	}
	if err := self.Delete(ctx); err != nil {
		t.Fatalf("self.Delete() failed: %v", err)
	}
}

func TestListChildren(t *testing.T, ctx *context.T, i interface{}) {
	self := makeLayer(i)

	var got, want []string
	var err error

	got, err = self.ListChildren(ctx)
	want = []string{}
	if err != nil {
		t.Fatalf("self.ListChildren() failed: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Lists do not match: got %v, want %v", got, want)
	}

	if err := self.Child("y").Create(ctx, nil); err != nil {
		t.Fatalf("y.Create() failed: %v", err)
	}
	got, err = self.ListChildren(ctx)
	want = []string{"y"}
	if err != nil {
		t.Fatalf("self.ListChildren() failed: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Lists do not match: got %v, want %v", got, want)
	}

	if err := self.Child("x").Create(ctx, nil); err != nil {
		t.Fatalf("x.Create() failed: %v", err)
	}
	got, err = self.ListChildren(ctx)
	want = []string{"x", "y"}
	if err != nil {
		t.Fatalf("self.ListChildren() failed: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Lists do not match: got %v, want %v", got, want)
	}
}

// TestPerms tests that {Set,Get}Permissions work as expected.
// TODO(sadovsky): All Vanadium {Set,Get}Permissions tests ought to share this
// test implementation. :)
// Mirrors v.io/groups/x/ref/services/groups/internal/server/server_test.go.
func TestPerms(t *testing.T, ctx *context.T, ac util.AccessController) {
	myperms := access.Permissions{}
	myperms.Add(security.BlessingPattern("server/client"), string(access.Admin))
	// Demonstrate that myperms differs from the current perms.
	if reflect.DeepEqual(myperms, getPermsOrDie(t, ctx, ac)) {
		t.Fatalf("Permissions should not match: %v", myperms)
	}

	var permsBefore, permsAfter access.Permissions
	var versionBefore, versionAfter string

	getPermsAndVersionOrDie := func() (access.Permissions, string) {
		perms, version, err := ac.GetPermissions(ctx)
		if err != nil {
			// Use Fatalf rather than t.Fatalf so we get a stack trace.
			Fatalf(t, "GetPermissions failed: %v", err)
		}
		return perms, version
	}

	// SetPermissions with bad version should fail.
	permsBefore, versionBefore = getPermsAndVersionOrDie()
	if err := ac.SetPermissions(ctx, myperms, "20"); verror.ErrorID(err) != verror.ErrBadVersion.ID {
		t.Fatal("SetPermissions should have failed with version error")
	}
	// Since SetPermissions failed, perms and version should not have changed.
	permsAfter, versionAfter = getPermsAndVersionOrDie()
	if !reflect.DeepEqual(permsAfter, permsBefore) {
		t.Errorf("Perms do not match: got %v, want %v", permsAfter, permsBefore)
	}
	if versionAfter != versionBefore {
		t.Errorf("Versions do not match: got %v, want %v", versionAfter, versionBefore)
	}

	// SetPermissions with correct version should succeed.
	permsBefore, versionBefore = permsAfter, versionAfter
	if err := ac.SetPermissions(ctx, myperms, versionBefore); err != nil {
		t.Fatalf("SetPermissions failed: %v", err)
	}
	// Check that perms and version actually changed.
	permsAfter, versionAfter = getPermsAndVersionOrDie()
	if !reflect.DeepEqual(permsAfter, myperms) {
		t.Errorf("Perms do not match: got %v, want %v", permsAfter, myperms)
	}
	if versionBefore == versionAfter {
		t.Errorf("Versions should not match: %v", versionBefore)
	}

	// SetPermissions with empty version should succeed.
	permsBefore, versionBefore = permsAfter, versionAfter
	myperms.Add(security.BlessingPattern("server/client"), string(access.Read))
	if err := ac.SetPermissions(ctx, myperms, ""); err != nil {
		t.Fatalf("SetPermissions failed: %v", err)
	}
	// Check that perms and version actually changed.
	permsAfter, versionAfter = getPermsAndVersionOrDie()
	if !reflect.DeepEqual(permsAfter, myperms) {
		t.Errorf("Perms do not match: got %v, want %v", permsAfter, myperms)
	}
	if versionBefore == versionAfter {
		t.Errorf("Versions should not match: %v", versionBefore)
	}

	// SetPermissions with unchanged perms should succeed, and version should
	// still change.
	permsBefore, versionBefore = permsAfter, versionAfter
	if err := ac.SetPermissions(ctx, myperms, ""); err != nil {
		t.Fatalf("SetPermissions failed: %v", err)
	}
	// Check that perms did not change and version did change.
	permsAfter, versionAfter = getPermsAndVersionOrDie()
	if !reflect.DeepEqual(permsAfter, permsBefore) {
		t.Errorf("Perms do not match: got %v, want %v", permsAfter, permsBefore)
	}
	if versionBefore == versionAfter {
		t.Errorf("Versions should not match: %v", versionBefore)
	}

	// Take away our access. SetPermissions and GetPermissions should fail.
	if err := ac.SetPermissions(ctx, access.Permissions{}, ""); err != nil {
		t.Fatalf("SetPermissions failed: %v", err)
	}
	if _, _, err := ac.GetPermissions(ctx); verror.ErrorID(err) != verror.ErrNoExistOrNoAccess.ID {
		t.Fatal("GetPermissions should have failed with access error")
	}
	if err := ac.SetPermissions(ctx, myperms, ""); verror.ErrorID(err) != verror.ErrNoExistOrNoAccess.ID {
		t.Fatal("SetPermissions should have failed with access error")
	}
}

////////////////////////////////////////
// Internal helpers

const notAvailable = "not available"

type layer interface {
	util.AccessController
	Create(ctx *context.T, perms access.Permissions) error
	Delete(ctx *context.T) error
	ListChildren(ctx *context.T) ([]string, error)
	Child(childName string) layer
}

type service struct {
	syncbase.Service
}

func (s *service) Create(ctx *context.T, perms access.Permissions) error {
	panic(notAvailable)
}
func (s *service) Delete(ctx *context.T) error {
	panic(notAvailable)
}
func (s *service) ListChildren(ctx *context.T) ([]string, error) {
	return s.ListApps(ctx)
}
func (s *service) Child(childName string) layer {
	return makeLayer(s.App(childName))
}

type app struct {
	syncbase.App
}

func (a *app) ListChildren(ctx *context.T) ([]string, error) {
	return a.ListDatabases(ctx)
}
func (a *app) Child(childName string) layer {
	return makeLayer(a.NoSQLDatabase(childName))
}

type database struct {
	nosql.Database
}

func (d *database) ListChildren(ctx *context.T) ([]string, error) {
	return d.ListTables(ctx)
}
func (d *database) Child(childName string) layer {
	return &table{Table: d.Table(childName), d: d}
}

type table struct {
	nosql.Table
	d nosql.Database
}

func (t *table) Create(ctx *context.T, perms access.Permissions) error {
	return t.d.CreateTable(ctx, t.Name(), perms)
}
func (t *table) Delete(ctx *context.T) error {
	return t.d.DeleteTable(ctx, t.Name())
}
func (t *table) SetPermissions(ctx *context.T, perms access.Permissions, version string) error {
	return t.Table.SetPermissions(ctx, nosql.Prefix(""), perms)
}
func (t *table) GetPermissions(ctx *context.T) (perms access.Permissions, version string, err error) {
	permsList, err := t.Table.GetPermissions(ctx, "")
	if len(permsList) != 1 || permsList[0].Prefix.Prefix() != "" {
		panic(fmt.Sprintf("unexpected perms list: %v", permsList))
	}
	return permsList[0].Perms, "", nil
}
func (t *table) ListChildren(ctx *context.T) ([]string, error) {
	panic(notAvailable)
}
func (t *table) Child(childName string) layer {
	return &row{t.Row(childName)}
}

type row struct {
	nosql.Row
}

func (r *row) Create(ctx *context.T, perms access.Permissions) error {
	if perms != nil {
		panic(fmt.Sprintf("bad perms: %v", perms))
	}
	return r.Put(ctx, true)
}
func (r *row) Delete(ctx *context.T) error {
	return r.Delete(ctx)
}
func (r *row) SetPermissions(ctx *context.T, perms access.Permissions, version string) error {
	panic(notAvailable)
}
func (r *row) GetPermissions(ctx *context.T) (perms access.Permissions, version string, err error) {
	panic(notAvailable)
}
func (r *row) ListChildren(ctx *context.T) ([]string, error) {
	panic(notAvailable)
}
func (r *row) Child(childName string) layer {
	panic(notAvailable)
}

func makeLayer(i interface{}) layer {
	switch t := i.(type) {
	case syncbase.Service:
		return &service{t}
	case syncbase.App:
		return &app{t}
	case nosql.Database:
		return &database{t}
	default:
		vlog.Fatalf("unexpected type: %T", t)
	}
	return nil
}
