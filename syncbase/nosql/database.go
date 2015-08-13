// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package nosql

import (
	wire "v.io/syncbase/v23/services/syncbase/nosql"
	"v.io/syncbase/v23/syncbase/util"
	"v.io/v23/context"
	"v.io/v23/naming"
	"v.io/v23/security/access"
	"v.io/v23/verror"
	"v.io/x/lib/vlog"
)

func NewDatabase(parentFullName, relativeName string, schema *Schema) *database {
	fullName := naming.Join(parentFullName, relativeName)
	return &database{
		c:              wire.DatabaseClient(fullName),
		parentFullName: parentFullName,
		fullName:       fullName,
		name:           relativeName,
		schema:         schema,
	}
}

type database struct {
	c              wire.DatabaseClientMethods
	parentFullName string
	fullName       string
	name           string
	schema         *Schema
}

var _ Database = (*database)(nil)

// TODO(sadovsky): Validate names before sending RPCs.

// Name implements Database.Name.
func (d *database) Name() string {
	return d.name
}

// FullName implements Database.FullName.
func (d *database) FullName() string {
	return d.fullName
}

// Exists implements Database.Exists.
func (d *database) Exists(ctx *context.T) (bool, error) {
	return d.c.Exists(ctx, d.schemaVersion())
}

// Table implements Database.Table.
func (d *database) Table(relativeName string) Table {
	return newTable(d.fullName, relativeName, d.schemaVersion())
}

// ListTables implements Database.ListTables.
func (d *database) ListTables(ctx *context.T) ([]string, error) {
	return util.List(ctx, d.fullName)
}

// Create implements Database.Create.
func (d *database) Create(ctx *context.T, perms access.Permissions) error {
	var schemaMetadata *wire.SchemaMetadata = nil
	if d.schema != nil {
		schemaMetadata = &d.schema.Metadata
	}
	return d.c.Create(ctx, schemaMetadata, perms)
}

// Delete implements Database.Delete.
func (d *database) Delete(ctx *context.T) error {
	return d.c.Delete(ctx, d.schemaVersion())
}

// CreateTable implements Database.CreateTable.
func (d *database) CreateTable(ctx *context.T, relativeName string, perms access.Permissions) error {
	return wire.TableClient(naming.Join(d.fullName, relativeName)).Create(ctx, d.schemaVersion(), perms)
}

// DeleteTable implements Database.DeleteTable.
func (d *database) DeleteTable(ctx *context.T, relativeName string) error {
	return wire.TableClient(naming.Join(d.fullName, relativeName)).Delete(ctx, d.schemaVersion())
}

// Exec implements Database.Exec.
func (d *database) Exec(ctx *context.T, query string) ([]string, ResultStream, error) {
	ctx, cancel := context.WithCancel(ctx)
	call, err := d.c.Exec(ctx, d.schemaVersion(), query)
	if err != nil {
		return nil, nil, err
	}
	resultStream := newResultStream(cancel, call)
	// The first row contains headers, pull them off the stream
	// and return them separately.
	var headers []string
	if !resultStream.Advance() {
		if err = resultStream.Err(); err != nil {
			// Since there was an error, can't get headers.
			// Just return the error.
			return nil, nil, err
		}
	}
	for _, header := range resultStream.Result() {
		headers = append(headers, header.RawString())
	}
	return headers, resultStream, nil
}

// BeginBatch implements Database.BeginBatch.
func (d *database) BeginBatch(ctx *context.T, opts wire.BatchOptions) (BatchDatabase, error) {
	relativeName, err := d.c.BeginBatch(ctx, d.schemaVersion(), opts)
	if err != nil {
		return nil, err
	}
	return &batch{database: *NewDatabase(d.parentFullName, relativeName, d.schema)}, nil
}

// SetPermissions implements Database.SetPermissions.
func (d *database) SetPermissions(ctx *context.T, perms access.Permissions, version string) error {
	return d.c.SetPermissions(ctx, perms, version)
}

// GetPermissions implements Database.GetPermissions.
func (d *database) GetPermissions(ctx *context.T) (perms access.Permissions, version string, err error) {
	return d.c.GetPermissions(ctx)
}

// SyncGroup implements Database.SyncGroup.
func (d *database) SyncGroup(sgName string) SyncGroup {
	return newSyncGroup(d.fullName, sgName)
}

// GetSyncGroupNames implements Database.GetSyncGroupNames.
func (d *database) GetSyncGroupNames(ctx *context.T) ([]string, error) {
	return d.c.GetSyncGroupNames(ctx)
}

// Blob implements Database.Blob.
func (d *database) Blob(br wire.BlobRef) Blob {
	return newBlob(d.fullName, br)
}

// CreateBlob implements Database.CreateBlob.
func (d *database) CreateBlob(ctx *context.T) (Blob, error) {
	return createBlob(ctx, d.fullName)
}

// UpgradeIfOutdated implements Database.UpgradeIfOutdated.
func (d *database) UpgradeIfOutdated(ctx *context.T) (bool, error) {
	var schema *Schema = d.schema
	if schema == nil {
		return false, verror.New(verror.ErrBadState, ctx, "Schema or SchemaMetadata cannot be nil. A valid Schema needs to be used when creating DB handle.")
	}

	if schema.Metadata.Version < 0 {
		return false, verror.New(verror.ErrBadState, ctx, "Schema version cannot be less than zero.")
	}

	schemaMgr := d.getSchemaManager()
	currMeta, err := schemaMgr.getSchemaMetadata(ctx)
	if err != nil {
		// If the client app did not set a schema as part of create db
		// getSchemaMetadata() will return ErrNoExist. If so we set the schema
		// here.
		if verror.ErrorID(err) == verror.ErrNoExist.ID {
			err := schemaMgr.setSchemaMetadata(ctx, schema.Metadata)
			// The database may not yet exist. If so above call will return
			// ErrNoExist and we return db without error. If the error
			// is different then return the error to the caller.
			if (err != nil) && (verror.ErrorID(err) != verror.ErrNoExist.ID) {
				return false, err
			}
			return false, nil
		}
		return false, err
	}

	if currMeta.Version >= schema.Metadata.Version {
		return false, nil
	}
	// Call the Upgrader provided by the app to upgrade the schema.
	//
	// TODO(jlodhia): disable sync before running Upgrader and reenable
	// once Upgrader is finished.
	//
	// TODO(jlodhia): prevent other processes (local/remote) from accessing
	// the database while upgrade is in progress.
	upgradeErr := schema.Upgrader.Run(d, currMeta.Version, schema.Metadata.Version)
	if upgradeErr != nil {
		vlog.Error(upgradeErr)
		return false, upgradeErr
	}
	// Update the schema metadata in db to the latest version.
	metadataErr := schemaMgr.setSchemaMetadata(ctx, schema.Metadata)
	if metadataErr != nil {
		vlog.Error(metadataErr)
		return false, metadataErr
	}
	return true, nil
}

func (d *database) getSchemaManager() schemaManagerImpl {
	return newSchemaManager(d.fullName)
}

func (d *database) schemaVersion() int32 {
	if d.schema == nil {
		return -1
	}
	return d.schema.Metadata.Version
}
