// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package syncbase_test

import (
	"fmt"
	"reflect"
	"testing"
	"time"

	"v.io/v23"
	"v.io/v23/context"
	"v.io/v23/discovery"
	"v.io/v23/security"
	"v.io/v23/security/access"
	wire "v.io/v23/services/syncbase"
	"v.io/v23/syncbase"
	"v.io/v23/verror"
	tu "v.io/x/ref/services/syncbase/testutil"
	"v.io/x/ref/test/testutil"
)

func TestSyncgroupDiscovery(t *testing.T) {
	_, ctx, sName, rootp, cleanup := tu.SetupOrDieCustom("o:app1:client1", "server",
		tu.DefaultPerms(access.AllTypicalTags(), "root:o:app1:client1"))
	defer cleanup()
	d := tu.CreateDatabase(t, ctx, syncbase.NewService(sName), "d")
	collection1 := tu.CreateCollection(t, ctx, d, "c1")
	collection2 := tu.CreateCollection(t, ctx, d, "c2")

	c1Updates, err := scanAs(ctx, rootp, "o:app1:client1")
	if err != nil {
		panic(err)
	}
	c2Updates, err := scanAs(ctx, rootp, "o:app1:client2")
	if err != nil {
		panic(err)
	}

	sgId := d.Syncgroup(ctx, "sg1").Id()
	spec := wire.SyncgroupSpec{
		Description: "test syncgroup sg1",
		Perms:       tu.DefaultPerms(wire.AllSyncgroupTags, "root:server", "root:o:app1:client1"),
		Collections: []wire.Id{collection1.Id()},
	}
	createSyncgroup(t, ctx, d, sgId, spec, verror.ID(""))

	// First update is for syncbase, and not a specific syncgroup.
	u := <-c1Updates
	attrs := u.Advertisement().Attributes
	peer := attrs[wire.DiscoveryAttrPeer]
	if peer == "" || len(attrs) != 1 {
		t.Errorf("Got %v, expected only a peer name.", attrs)
	}
	// Client2 should see the same.
	if err := expect(c2Updates, &discovery.AdId{}, find, discovery.Attributes{wire.DiscoveryAttrPeer: peer}); err != nil {
		t.Error(err)
	}

	sg1Attrs := discovery.Attributes{
		wire.DiscoveryAttrDatabaseName:      "d",
		wire.DiscoveryAttrDatabaseBlessing:  "root:o:app1",
		wire.DiscoveryAttrSyncgroupName:     "sg1",
		wire.DiscoveryAttrSyncgroupBlessing: "root:o:app1:client1",
	}
	sg2Attrs := discovery.Attributes{
		wire.DiscoveryAttrDatabaseName:      "d",
		wire.DiscoveryAttrDatabaseBlessing:  "root:o:app1",
		wire.DiscoveryAttrSyncgroupName:     "sg2",
		wire.DiscoveryAttrSyncgroupBlessing: "root:o:app1:client1",
	}

	// Then we should see an update for the created syncgroup.
	var sg1AdId discovery.AdId
	if err := expect(c1Updates, &sg1AdId, find, sg1Attrs); err != nil {
		t.Error(err)
	}

	// Now update the spec to add client2 to the permissions.
	spec.Perms = tu.DefaultPerms(wire.AllSyncgroupTags, "root:server", "root:o:app1:client1", "root:o:app1:client2")
	if err := d.SyncgroupForId(sgId).SetSpec(ctx, spec, ""); err != nil {
		t.Fatalf("sg.SetSpec failed: %v", err)
	}

	// Client1 should see a lost and a found message.
	if err := expect(c1Updates, &sg1AdId, both, sg1Attrs); err != nil {
		t.Error(err)
	}
	// Client2 should just now see the found message.
	if err := expect(c2Updates, &sg1AdId, find, sg1Attrs); err != nil {
		t.Error(err)
	}

	// Now create a second syncgroup.
	sg2Id := d.Syncgroup(ctx, "sg2").Id()
	spec2 := wire.SyncgroupSpec{
		Description: "test syncgroup sg2",
		Perms:       tu.DefaultPerms(wire.AllSyncgroupTags, "root:server", "root:o:app1:client1", "root:o:app1:client2"),
		Collections: []wire.Id{collection2.Id()},
	}
	createSyncgroup(t, ctx, d, sg2Id, spec2, verror.ID(""))

	// Both clients should see the new syncgroup.
	var sg2AdId discovery.AdId
	if err := expect(c1Updates, &sg2AdId, find, sg2Attrs); err != nil {
		t.Error(err)
	}
	if err := expect(c2Updates, &sg2AdId, find, sg2Attrs); err != nil {
		t.Error(err)
	}

	spec2.Perms = tu.DefaultPerms(wire.AllSyncgroupTags, "root:server", "root:o:app1:client1")
	if err := d.SyncgroupForId(sg2Id).SetSpec(ctx, spec2, ""); err != nil {
		t.Fatalf("sg.SetSpec failed: %v", err)
	}
	if err := expect(c2Updates, &sg2AdId, lose, sg2Attrs); err != nil {
		t.Error(err)
	}
}

func scanAs(ctx *context.T, rootp security.Principal, as string) (<-chan discovery.Update, error) {
	idp := testutil.IDProviderFromPrincipal(rootp)
	p := testutil.NewPrincipal()
	if err := idp.Bless(p, as); err != nil {
		return nil, err
	}
	ctx, err := v23.WithPrincipal(ctx, p)
	if err != nil {
		return nil, err
	}
	dis, err := syncbase.NewDiscovery(ctx)
	if err != nil {
		return nil, err
	}
	return dis.Scan(ctx, `v.InterfaceName="v.io/x/ref/services/syncbase/server/interfaces/Sync"`)
}

const (
	lose = "lose"
	find = "find"
	both = "both"
)

func expect(ch <-chan discovery.Update, id *discovery.AdId, typ string, want discovery.Attributes) error {
	select {
	case u := <-ch:
		if (u.IsLost() && typ == find) || (!u.IsLost() && typ == lose) {
			return fmt.Errorf("IsLost mismatch.  Got %v, wanted %v", u, typ)
		}
		ad := u.Advertisement()
		got := ad.Attributes
		if id.IsValid() {
			if *id != ad.Id {
				return fmt.Errorf("mismatched id, got %v, want %v", ad.Id, id)
			}
		} else {
			*id = ad.Id
		}
		if !reflect.DeepEqual(got, want) {
			return fmt.Errorf("got %v, want %v", got, want)
		}
		if typ == both {
			typ = lose
			if u.IsLost() {
				typ = find
			}
			return expect(ch, id, typ, want)
		}
		return nil
	case <-time.After(2 * time.Second):
		return fmt.Errorf("timed out")
	}
}
