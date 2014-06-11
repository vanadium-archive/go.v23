package security

import (
	"io/ioutil"
	"os"
	"runtime"
	"testing"
	"time"
	"veyron2/naming"
)

type authMap map[PublicID]LabelSet

// context implements Context.
type context struct {
	localID, remoteID    PublicID
	discharges           CaveatDischargeMap
	method, name, suffix string
	label                Label
}

func (c *context) Method() string                       { return c.method }
func (c *context) Name() string                         { return c.name }
func (c *context) Suffix() string                       { return c.suffix }
func (c *context) Label() Label                         { return c.label }
func (c *context) CaveatDischarges() CaveatDischargeMap { return c.discharges }
func (c *context) LocalID() PublicID                    { return c.localID }
func (c *context) RemoteID() PublicID                   { return c.remoteID }
func (c *context) LocalEndpoint() naming.Endpoint       { return nil }
func (c *context) RemoteEndpoint() naming.Endpoint      { return nil }

func saveACLToTempFile(acl ACL) string {
	f, err := ioutil.TempFile("", "saved_acl")
	if err != nil {
		panic(err)
	}
	defer f.Close()
	if err := SaveACL(f, acl); err != nil {
		defer os.Remove(f.Name())
		panic(err)
	}
	return f.Name()
}

func updateACLInFile(fileName string, acl ACL) {
	f, err := os.OpenFile(fileName, os.O_WRONLY, 0600)
	if err != nil {
		panic(err)
	}
	defer f.Close()
	if err := SaveACL(f, acl); err != nil {
		panic(err)
	}
}

func bless(blessee PublicID, blesser PrivateID, name string) PublicID {
	blessed, err := blesser.Bless(blessee, name, 5*time.Minute, nil)
	if err != nil {
		panic(err)
	}
	return blessed
}

func derive(pub PublicID, priv PrivateID) PrivateID {
	d, err := priv.Derive(pub)
	if err != nil {
		panic(err)
	}
	return d
}

func testSelfRPCs(t *testing.T, authorizer Authorizer) {
	_, file, line, _ := runtime.Caller(1)
	var (
		veyron      = fakeID("veyron")
		alice       = fakeID("alice")
		veyronAlice = bless(alice.PublicID(), veyron, "alice")
	)
	testData := []struct {
		localID, remoteID PublicID
		isAuthorized      bool
	}{
		{alice.PublicID(), alice.PublicID(), true},
		{veyron.PublicID(), veyron.PublicID(), true},
		{veyron.PublicID(), alice.PublicID(), false},
		{veyronAlice, veyronAlice, true},
		{veyronAlice, alice.PublicID(), false},
		{veyronAlice, veyron.PublicID(), false},
	}
	for _, d := range testData {
		ctx := &context{localID: d.localID, remoteID: d.remoteID}
		if got, want := authorizer.Authorize(ctx), d.isAuthorized; (got == nil) != want {
			t.Errorf("%s:%d: %+v.Authorize(&context{localID: %v, remoteID: %v}) returned error: %v, want error: %v", file, line, authorizer, d.localID, d.remoteID, got, !want)
		}
	}
}

func testAuthorizations(t *testing.T, authorizer Authorizer, authorizations authMap) {
	_, file, line, _ := runtime.Caller(1)
	for user, labels := range authorizations {
		for _, l := range ValidLabels {
			ctx := &context{remoteID: user, label: l}
			if got, want := authorizer.Authorize(ctx), labels.HasLabel(l); (got == nil) != want {
				t.Errorf("%s:%d: %+v.Authorize(&context{remoteID: %v, label: %v}) returned error: %v, want error: %v", file, line, authorizer, user, l, got, !want)
			}
		}
	}
}

func testNothingPermitted(t *testing.T, authorizer Authorizer) {
	_, file, line, _ := runtime.Caller(1)
	var (
		veyron            = fakeID("veyron")
		random            = fakeID("random").PublicID()
		alice             = fakeID("alice")
		veyronAlice       = bless(alice.PublicID(), veyron, "alice")
		veyronAliceFriend = bless(random, derive(veyronAlice, alice), "friend")
		veyronBob         = bless(random, veyron, "bob")
	)
	users := []PublicID{
		veyron.PublicID(),
		random,
		alice.PublicID(),

		// Blessed principals
		veyronAlice,
		veyronAliceFriend,
		veyronBob,
	}
	// No principal (whether the identity provider is trusted or not)
	// should have access to any valid or invalid label.
	for _, u := range users {
		for _, l := range ValidLabels {
			ctx := &context{remoteID: u, label: l}
			if got := authorizer.Authorize(ctx); got == nil {
				t.Errorf("%s:%d: %+v.Authorize(%v) returns nil, want error", file, line, authorizer, ctx)
			}
		}
		invalidLabel := Label(3)
		ctx := &context{remoteID: u, label: invalidLabel}
		if got := authorizer.Authorize(ctx); got == nil {
			t.Errorf("%s:%d: %+v.Authorize(%v) returns nil, want error", file, line, authorizer, ctx)
		}
	}
}

func TestACLAuthorizer(t *testing.T) {
	const (
		// Shorthands
		R = ReadLabel
		W = WriteLabel
		A = AdminLabel
		D = DebugLabel
		M = MonitoringLabel
	)
	// Principals to test
	var (
		veyron = fakeID("veyron")
		alice  = fakeID("alice")
		bob    = fakeID("bob").PublicID()

		// Blessed principals
		veyronAlice       = bless(alice, veyron, "alice")
		veyronBob         = bless(bob, veyron, "bob")
		veyronAliceFriend = bless(bob, derive(veyronAlice, alice), "friend")
	)
	// Convenience function for combining Labels into a LabelSet.
	LS := func(labels ...Label) LabelSet {
		var ret LabelSet
		for _, l := range labels {
			ret = ret | LabelSet(l)
		}
		return ret
	}

	// ACL for testing
	acl := ACL{
		"*":              LS(R),
		"veyron/alice/*": LS(W, R),
		"veyron/alice":   LS(A, D, M),
		"veyron/bob":     LS(D, M),
	}

	// Authorizations for the above ACL.
	authorizations := authMap{
		// alice and bob have only what "*" has.
		alice: LS(R),
		bob:   LS(R),
		// veyron and veyronAlice have R, W, A, D, M from the "veyron/alice" and
		// "veyron/alice/*" ACL entries.
		veyron:      LS(R, W, A, D, M),
		veyronAlice: LS(R, W, A, D, M),
		// veyronBob has R, D, M from "*" and "veyron/bob" ACL entries.
		veyronBob: LS(R, D, M),
		// veyronAliceFriend has W, R from the "veyron/alice/*" ACL entry.
		veyronAliceFriend: LS(W, R),
		// nil PublicIDs are not authorized.
		nil: LS(),
	}
	// Create an aclAuthorizer based on the ACL and verify the authorizations.
	authorizer := NewACLAuthorizer(acl)
	testAuthorizations(t, authorizer, authorizations)
	testSelfRPCs(t, authorizer)

	// Create a fileACLAuthorizer by saving the ACL in a file, and verify the
	// authorizations.
	fileName := saveACLToTempFile(acl)
	defer os.Remove(fileName)
	fileAuthorizer := NewFileACLAuthorizer(fileName)
	testAuthorizations(t, fileAuthorizer, authorizations)
	testSelfRPCs(t, fileAuthorizer)

	// Modify the ACL stored in the file and verify that the authorizations appropriately
	// change for the fileACLAuthorizer.
	acl["veyron/bob"] = LS(R, W, A, D, M)
	updateACLInFile(fileName, acl)

	authorizations[veyronBob] = LS(R, W, A, D, M)
	testAuthorizations(t, fileAuthorizer, authorizations)
	testSelfRPCs(t, fileAuthorizer)

	// Update the ACL file with invalid contents and verify that no requests are
	// authorized.
	f, err := os.OpenFile(fileName, os.O_WRONLY, 0600)
	if err != nil {
		panic(err)
	}
	f.Write([]byte("invalid ACL"))
	f.Close()
	testNothingPermitted(t, fileAuthorizer)

	// Verify that a fileACLAuthorizer based on a nonexistent file does not authorize any
	// requests.
	fileAuthorizer = NewFileACLAuthorizer("fileDoesNotExist")
	testNothingPermitted(t, fileAuthorizer)
}

func TestNilACLAuthorizer(t *testing.T) {
	authorizer := NewACLAuthorizer(nil)
	testNothingPermitted(t, authorizer)
	testSelfRPCs(t, authorizer)
}
