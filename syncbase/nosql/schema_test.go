// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package nosql_test

import (
	"testing"

	wire "v.io/syncbase/v23/services/syncbase/nosql"
	"v.io/syncbase/v23/syncbase"
	tu "v.io/syncbase/v23/syncbase/testutil"
	"v.io/v23/context"
)

// Tests schema checking logic within App.NoSQLDatabase() method.
// This test as following steps:
// 1) Call NoSQLDatabase() for a non existent db.
// 2) Create the database, and verify if Schema got stored properly.
// 3) Call UpgradeIfOutdated() to make sure that the method is no-op and is
//    able to read the schema from db.
// 4) Call NoSQLDatabase() on the same db to create a new handle with an
//    upgraded schema, call UpgradeIfOutdated() and check if SchemaUpgrader
//    is called and if the new schema is stored appropriately.
func TestSchemaCheck(t *testing.T) {
	ctx, sName, cleanup := tu.SetupOrDie(nil)
	defer cleanup()
	a := tu.CreateApp(t, ctx, syncbase.NewService(sName), "a")
	schema := tu.DefaultSchema()
	mockUpgrader := schema.Upgrader.(*tu.MockSchemaUpgrader)

	db1 := a.NoSQLDatabase("db1", schema)

	// Verify that calling Upgrade on a non existing database does not throw
	// errors.
	_, err := db1.UpgradeIfOutdated(ctx)
	if err != nil {
		t.Fatalf("db1.UpgradeIfOutdated() failed: %v", err)
	}
	if mockUpgrader.CallCount > 0 {
		t.Fatal("Call to upgrader was not expected.")
	}

	// Create db1, this step also stores the schema provided above
	if err := db1.Create(ctx, nil); err != nil {
		t.Fatalf("db1.Create() failed: %v", err)
	}
	// verify if schema was stored as part of create
	if _, err := getSchemaMetadata(ctx, db1.FullName()); err != nil {
		t.Fatalf("Failed to lookup schema after create: %v", err)
	}

	// Make redundant call to Upgrade to verify that it is a no-op
	result, err1 := db1.UpgradeIfOutdated(ctx)
	if result {
		t.Fatalf("db1.UpgradeIfOutdated() should not return true")
	}
	if err1 != nil {
		t.Fatalf("db1.UpgradeIfOutdated() failed: %v", err1)
	}
	if mockUpgrader.CallCount > 0 {
		t.Fatal("Call to upgrader was not expected.")
	}

	// try to make a new database object for the same database but this time
	// with a new schema version
	schema.Metadata.Version = 1
	rule := wire.CrRule{"table1", "foo", "", wire.ResolverTypeLastWins}
	policy := wire.CrPolicy{
		Rules: []wire.CrRule{rule},
	}
	schema.Metadata.Policy = policy
	otherdb1 := a.NoSQLDatabase("db1", schema)
	otherresult, othererr := otherdb1.UpgradeIfOutdated(ctx)

	if !otherresult {
		t.Fatalf("otherdb1.UpgradeIfOutdated() expected to return true")
	}
	if othererr != nil {
		t.Fatalf("otherdb1.UpgradeIfOutdated() failed: %v", othererr)
	}
	if mockUpgrader.CallCount != 1 {
		t.Fatalf("Unexpected number of calls to upgrader. Expected: %d, Actual: %d.", 1, mockUpgrader.CallCount)
	}

	// check if the contents of SchemaMetadata are correctly stored in the db.
	metadata, err3 := getSchemaMetadata(ctx, otherdb1.FullName())
	if err3 != nil {
		t.Fatalf("GetSchemaMetadata failed: %v", err3)
	}
	if metadata.Version != 1 {
		t.Fatalf("Unexpected version number: %d", metadata.Version)
	}
	if len(metadata.Policy.Rules) != 1 {
		t.Fatalf("Unexpected number of rules: %d", len(metadata.Policy.Rules))
	}
	if metadata.Policy.Rules[0] != rule {
		t.Fatalf("Unexpected number of rules: %d", len(metadata.Policy.Rules))
	}
}

func getSchemaMetadata(ctx *context.T, dbName string) (wire.SchemaMetadata, error) {
	return wire.DatabaseClient(dbName).GetSchemaMetadata(ctx)
}
