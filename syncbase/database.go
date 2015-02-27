package syncbase

import (
	wire "v.io/syncbase/v23/services/syncbase"
	"v.io/v23/context"
	"v.io/v23/naming"
	"v.io/v23/services/security/access"
)

type database struct {
	c            wire.DatabaseClientMethods
	name         string
	relativeName string
}

var _ Database = (*database)(nil)

// Name implements Database.Name.
func (d *database) Name() string {
	return d.relativeName
}

// BindTable implements Database.BindTable.
func (d *database) BindTable(relativeName string) Table {
	name := naming.Join(d.name, relativeName)
	return &table{wire.TableClient(name), name, relativeName}
}

// Create implements Database.Create.
func (d *database) Create(ctx *context.T, acl access.TaggedACLMap) error {
	return d.c.Create(ctx, acl)
}

// Delete implements Database.Delete.
func (d *database) Delete(ctx *context.T) error {
	return d.c.Delete(ctx)
}

// UpdateSchema implements Database.UpdateSchema.
func (d *database) UpdateSchema(ctx *context.T, schema wire.Schema, etag string) error {
	return d.c.UpdateSchema(ctx, schema, etag)
}

// GetSchema implements Database.GetSchema.
func (d *database) GetSchema(ctx *context.T) (schema wire.Schema, etag string, err error) {
	return d.c.GetSchema(ctx)
}

// SetACL implements Database.SetACL.
func (d *database) SetACL(ctx *context.T, acl access.TaggedACLMap, etag string) error {
	return d.c.SetACL(ctx, acl, etag)
}

// GetACL implements Database.GetACL.
func (d *database) GetACL(ctx *context.T) (acl access.TaggedACLMap, etag string, err error) {
	return d.c.GetACL(ctx)
}
