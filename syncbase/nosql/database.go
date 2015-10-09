// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package nosql

import (
	"sync"
	"time"

	"v.io/v23/context"
	"v.io/v23/naming"
	"v.io/v23/security/access"
	svcwire "v.io/v23/services/syncbase"
	wire "v.io/v23/services/syncbase/nosql"
	"v.io/v23/services/watch"
	"v.io/v23/syncbase/util"
	"v.io/v23/verror"
	"v.io/x/lib/vlog"
)

const (
	// Wait time before we try to reconnect a broken conflict resolution stream.
	waitBeforeReconnectInMillis = 2 * time.Second
	reconnectionCount           = "rcc"
)

func newDatabaseImpl(parentFullName, relativeName, batchSuffix string, schema *Schema) *database {
	// Escape relativeName so that any forward slashes get dropped, thus ensuring
	// that the server will interpret fullName as referring to a database object.
	// Note that the server will still reject this name if util.ValidDatabaseName
	// returns false.
	fullName := naming.Join(parentFullName, util.Escape(relativeName)+batchSuffix)
	return &database{
		c:              wire.DatabaseClient(fullName),
		parentFullName: parentFullName,
		fullName:       fullName,
		name:           relativeName,
		schema:         schema,
		crState: conflictResolutionState{
			reconnectWaitTime: waitBeforeReconnectInMillis,
		},
	}
}

func NewDatabase(parentFullName, relativeName string, schema *Schema) *database {
	return newDatabaseImpl(parentFullName, relativeName, "", schema)
}

type database struct {
	c              wire.DatabaseClientMethods
	parentFullName string
	fullName       string
	name           string
	schema         *Schema
	crState        conflictResolutionState
}

// conflictResolutionState maintains data about the connection of a conflict
// resolution stream with syncbase. It provides a way to disconnect an existing
// open stream.
type conflictResolutionState struct {
	mu                sync.Mutex // guards access to all fields in this struct
	crContext         *context.T
	cancelFn          context.CancelFunc
	isClosed          bool
	reconnectWaitTime time.Duration
}

func (crs *conflictResolutionState) disconnect() {
	crs.mu.Lock()
	defer crs.mu.Unlock()
	crs.isClosed = true
	crs.cancelFn()
}

func (crs *conflictResolutionState) isDisconnected() bool {
	crs.mu.Lock()
	defer crs.mu.Unlock()
	return crs.isClosed
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
	// See comment in v.io/v23/services/syncbase/nosql/service.vdl for why we
	// can't implement ListTables using Glob (via util.ListChildren).
	return d.c.ListTables(ctx)
}

// Create implements Database.Create.
func (d *database) Create(ctx *context.T, perms access.Permissions) error {
	var schemaMetadata *wire.SchemaMetadata = nil
	if d.schema != nil {
		schemaMetadata = &d.schema.Metadata
	}
	return d.c.Create(ctx, schemaMetadata, perms)
}

// Destroy implements Database.Destroy.
func (d *database) Destroy(ctx *context.T) error {
	return d.c.Destroy(ctx, d.schemaVersion())
}

// Exec implements Database.Exec.
func (d *database) Exec(ctx *context.T, query string) ([]string, ResultStream, error) {
	ctx, cancel := context.WithCancel(ctx)
	call, err := d.c.Exec(ctx, d.schemaVersion(), query)
	if err != nil {
		return nil, nil, err
	}
	resultStream := newResultStream(cancel, call)
	// The first row contains column headers. Pull them off the stream
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
	batchSuffix, err := d.c.BeginBatch(ctx, d.schemaVersion(), opts)
	if err != nil {
		return nil, err
	}
	return &batch{database: *newDatabaseImpl(d.parentFullName, d.name, batchSuffix, d.schema)}, nil
}

// SetPermissions implements Database.SetPermissions.
func (d *database) SetPermissions(ctx *context.T, perms access.Permissions, version string) error {
	return d.c.SetPermissions(ctx, perms, version)
}

// GetPermissions implements Database.GetPermissions.
func (d *database) GetPermissions(ctx *context.T) (perms access.Permissions, version string, err error) {
	return d.c.GetPermissions(ctx)
}

// Watch implements the Database interface.
func (d *database) Watch(ctx *context.T, table, prefix string, resumeMarker watch.ResumeMarker) (WatchStream, error) {
	if !util.ValidTableName(table) {
		return nil, verror.New(svcwire.ErrInvalidName, ctx, table)
	}
	ctx, cancel := context.WithCancel(ctx)
	call, err := d.c.WatchGlob(ctx, watch.GlobRequest{
		Pattern:      naming.Join(table, prefix+"*"),
		ResumeMarker: resumeMarker,
	})
	if err != nil {
		return nil, err
	}
	return newWatchStream(cancel, call), nil
}

// GetResumeMarker implements the Database interface.
func (d *database) GetResumeMarker(ctx *context.T) (watch.ResumeMarker, error) {
	return d.c.GetResumeMarker(ctx)
}

// Syncgroup implements Database.Syncgroup.
func (d *database) Syncgroup(sgName string) Syncgroup {
	return newSyncgroup(d.fullName, sgName)
}

// GetSyncgroupNames implements Database.GetSyncgroupNames.
func (d *database) GetSyncgroupNames(ctx *context.T) ([]string, error) {
	return d.c.GetSyncgroupNames(ctx)
}

// Blob implements Database.Blob.
func (d *database) Blob(br wire.BlobRef) Blob {
	return newBlob(d.fullName, br)
}

// CreateBlob implements Database.CreateBlob.
func (d *database) CreateBlob(ctx *context.T) (Blob, error) {
	return createBlob(ctx, d.fullName)
}

// EnforceSchema implements Database.EnforceSchema.
func (d *database) EnforceSchema(ctx *context.T) error {
	var schema *Schema = d.schema
	if schema == nil {
		return verror.New(verror.ErrBadState, ctx, "Schema or SchemaMetadata cannot be nil. A valid Schema needs to be used when creating DB handle.")
	}

	if schema.Metadata.Version < 0 {
		return verror.New(verror.ErrBadState, ctx, "Schema version cannot be less than zero.")
	}

	if needsResolver(d.schema.Metadata) && d.schema.Resolver == nil {
		return verror.New(verror.ErrBadState, ctx, "ResolverTypeAppResolves cannot be used in CrRule without providing a ConflictResolver in Schema.")
	}

	if _, err := d.upgradeIfOutdated(ctx); err != nil {
		return err
	}

	if d.schema.Resolver == nil {
		return nil
	}

	childCtx, cancelFn := context.WithCancel(ctx)
	d.crState.crContext = childCtx
	d.crState.cancelFn = cancelFn

	go d.establishConflictResolution(childCtx)
	return nil
}

// Close implements Database.Close.
func (d *database) Close() {
	d.crState.disconnect()
}

func (d *database) upgradeIfOutdated(ctx *context.T) (bool, error) {
	var schema *Schema = d.schema
	schemaMgr := d.getSchemaManager()
	currMeta, err := schemaMgr.getSchemaMetadata(ctx)
	if err != nil {
		// If the client app did not set a schema as part of database creation,
		// getSchemaMetadata() will return ErrNoExist. In this case, we set the
		// schema here.
		if verror.ErrorID(err) == verror.ErrNoExist.ID {
			err := schemaMgr.setSchemaMetadata(ctx, schema.Metadata)
			// The database may not yet exist. If so, the above call will return
			// ErrNoExist, and here we return (false, nil). If the error is anything
			// other than ErrNoExist, here we return (false, err).
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

func (d *database) establishConflictResolution(ctx *context.T) {
	count := 0
	for {
		count++
		vlog.Infof("Starting a new conflict resolution connection. Reconnection count: %d", count)
		childCtx := context.WithValue(ctx, reconnectionCount, count)
		// listenForConflicts is a blocking method that returns only when the
		// conflict stream is broken.
		if err := d.listenForConflicts(childCtx); err != nil {
			vlog.Errorf("Conflict resolution connection ended with error: %v", err)
		}

		// Check if database is closed and if we need to shutdown conflict
		// resolution.
		if d.crState.isDisconnected() {
			vlog.Infof("Shutting down conflict resolution connection.")
			break
		}

		// The connection might have broken because the syncbase server went down.
		// Sleep for a few seconds to allow syncbase to come back up.
		time.Sleep(d.crState.reconnectWaitTime)
	}
}

func (d *database) listenForConflicts(ctx *context.T) error {
	resolver, err := d.c.StartConflictResolver(ctx)
	if err != nil {
		return err
	}
	conflictStream := resolver.RecvStream()
	resolutionStream := resolver.SendStream()
	var c *Conflict = &Conflict{}
	for conflictStream.Advance() {
		row := conflictStream.Value()
		addRowToConflict(c, &row)
		if !row.Continued {
			resolution := d.schema.Resolver.OnConflict(ctx, c)
			if err := sendResolution(resolutionStream, resolution); err != nil {
				return err
			}
			c = &Conflict{} // create a new conflict object for the next batch
		}
	}
	if err := conflictStream.Err(); err != nil {
		return err
	}
	return resolver.Finish()
}

// TODO(jlodhia): Should we check that the Resolution provided by the
// application resolves all conflicts in the write set?
func sendResolution(stream interface {
	Send(item wire.ResolutionInfo) error
}, resolution Resolution) error {
	size := len(resolution.ResultSet)
	count := 0
	for _, v := range resolution.ResultSet {
		count++
		ri := toResolutionInfo(v, count != size)
		if err := stream.Send(ri); err != nil {
			vlog.Error("Error while sending resolution")
			return err
		}
	}
	return nil
}

func addRowToConflict(c *Conflict, ci *wire.ConflictInfo) {
	switch v := ci.Data.(type) {
	case wire.ConflictDataBatch:
		if c.Batches == nil {
			c.Batches = map[uint16]wire.BatchInfo{}
		}
		c.Batches[v.Value.Id] = v.Value
	case wire.ConflictDataRow:
		rowInfo := v.Value
		switch op := rowInfo.Op.(type) {
		case wire.OperationWrite:
			if c.WriteSet == nil {
				c.WriteSet = &ConflictRowSet{map[string]ConflictRow{}, map[uint16][]ConflictRow{}}
			}
			cr := toConflictRow(op.Value, rowInfo.BatchIds)
			c.WriteSet.ByKey[cr.Key] = cr
			for _, bid := range rowInfo.BatchIds {
				c.WriteSet.ByBatch[bid] = append(c.WriteSet.ByBatch[bid], cr)
			}
		case wire.OperationRead:
			if c.ReadSet == nil {
				c.ReadSet = &ConflictRowSet{map[string]ConflictRow{}, map[uint16][]ConflictRow{}}
			}
			cr := toConflictRow(op.Value, rowInfo.BatchIds)
			c.ReadSet.ByKey[cr.Key] = cr
			for _, bid := range rowInfo.BatchIds {
				c.ReadSet.ByBatch[bid] = append(c.ReadSet.ByBatch[bid], cr)
			}
		case wire.OperationScan:
			if c.ScanSet == nil {
				c.ScanSet = &ConflictScanSet{map[uint16][]wire.ScanOp{}}
			}
			for _, bid := range rowInfo.BatchIds {
				c.ScanSet.ByBatch[bid] = append(c.ScanSet.ByBatch[bid], op.Value)
			}
		}
	}
}

func toConflictRow(op wire.RowOp, batchIds []uint16) ConflictRow {
	var local, remote, ancestor *Value
	if op.LocalValue != nil {
		local = &Value{
			val:       op.LocalValue.Bytes,
			WriteTs:   toTime(op.LocalValue.WriteTs),
			selection: wire.ValueSelectionLocal,
		}
	}
	if op.RemoteValue != nil {
		remote = &Value{
			val:       op.RemoteValue.Bytes,
			WriteTs:   toTime(op.RemoteValue.WriteTs),
			selection: wire.ValueSelectionRemote,
		}
	}
	if op.AncestorValue != nil {
		ancestor = &Value{
			val:       op.AncestorValue.Bytes,
			WriteTs:   toTime(op.AncestorValue.WriteTs),
			selection: wire.ValueSelectionOther,
		}
	}
	return ConflictRow{
		Key:           op.Key,
		LocalValue:    local,
		RemoteValue:   remote,
		AncestorValue: ancestor,
		BatchIds:      batchIds,
	}
}

// TODO(jlodhia): remove this method once time is stored as time.Time instead
// of int64.
func toTime(unixNanos int64) time.Time {
	return time.Unix(
		unixNanos/1e9, // seconds
		unixNanos%1e9) // nanoseconds
}

func toResolutionInfo(r ResolvedRow, lastRow bool) wire.ResolutionInfo {
	sel := wire.ValueSelectionOther
	resVal := (*wire.Value)(nil)
	if r.Result != nil {
		sel = r.Result.selection
		resVal = &wire.Value{
			Bytes:   r.Result.val,
			WriteTs: r.Result.WriteTs.UnixNano(), // ignored by syncbase
		}
	}
	return wire.ResolutionInfo{
		Key:       r.Key,
		Selection: sel,
		Result:    resVal,
		Continued: lastRow,
	}
}

func needsResolver(metadata wire.SchemaMetadata) bool {
	for _, rule := range metadata.Policy.Rules {
		if rule.Resolver == wire.ResolverTypeAppResolves {
			return true
		}
	}
	return false
}

func (d *database) getSchemaManager() schemaManagerImpl {
	return newSchemaManager(d.c)
}

func (d *database) schemaVersion() int32 {
	if d.schema == nil {
		return -1
	}
	return d.schema.Metadata.Version
}
