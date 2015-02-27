package syncbase

import (
	wire "v.io/syncbase/v23/services/syncbase"
	"v.io/v23/context"
	"v.io/v23/naming"
	"v.io/v23/services/security/access"
)

type universe struct {
	c            wire.UniverseClientMethods
	name         string
	relativeName string
}

var _ Universe = (*universe)(nil)

// Name implements Universe.Name.
func (u *universe) Name() string {
	return u.relativeName
}

// BindDatabase implements Universe.BindDatabase.
func (u *universe) BindDatabase(relativeName string) Database {
	name := naming.Join(u.name, relativeName)
	return &database{wire.DatabaseClient(name), name, relativeName}
}

// ListDatabases implements Universe.ListDatabases.
func (u *universe) ListDatabases(ctx *context.T) ([]string, error) {
	// TODO(sadovsky): Implement on top of Glob.
	return nil, nil
}

// Create implements Universe.Create.
func (u *universe) Create(ctx *context.T, acl access.TaggedACLMap) error {
	return u.c.Create(ctx, acl)
}

// Delete implements Universe.Delete.
func (u *universe) Delete(ctx *context.T) error {
	return u.c.Delete(ctx)
}

// SetACL implements Universe.SetACL.
func (u *universe) SetACL(ctx *context.T, acl access.TaggedACLMap, etag string) error {
	return u.c.SetACL(ctx, acl, etag)
}

// GetACL implements Universe.GetACL.
func (u *universe) GetACL(ctx *context.T) (acl access.TaggedACLMap, etag string, err error) {
	return u.c.GetACL(ctx)
}
