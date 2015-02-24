package access

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"fmt"
	"io/ioutil"
	"os"
	"reflect"
	"testing"

	"v.io/v23/security"
	"v.io/v23/services/security/access/test"
	"v.io/v23/vdl"
)

// TestTaggedACLAuthorizer is both a test and a demonstration of the use of the
// TaggedACLAuthorizer and interaction with interface specification in VDL.
func TestTaggedACLAuthorizer(t *testing.T) {
	type P []security.BlessingPattern
	type S []string
	// TaggedACLMap to test against.
	acl := TaggedACLMap{
		"R": {
			In: P{security.AllPrincipals},
		},
		"W": {
			In:    P{"ali/family", "bob", "che/$"},
			NotIn: S{"bob/acquaintances"},
		},
		"X": {
			In: P{"ali/family/boss/$", "superman/$"},
		},
	}
	type testcase struct {
		Method string
		Client security.Blessings
	}
	var (
		authorizer, _ = TaggedACLAuthorizer(acl, reflect.TypeOf(test.Read))
		// Two principals: The "server" and the "client"
		pserver   = newPrincipal(t)
		pclient   = newPrincipal(t)
		server, _ = pserver.BlessSelf("server")

		// B generates the provided blessings for the client and ensures
		// that the server will recognize them.
		B = func(names ...string) security.Blessings {
			var ret security.Blessings
			for _, name := range names {
				b, err := pclient.BlessSelf(name)
				if err != nil {
					t.Fatalf("%q: %v", name, err)
				}
				// Since this test uses trustAllRoots, no need
				// to call pserver.AddToRoots(b) to get the server
				// to recognize the client.
				if ret, err = security.UnionOfBlessings(ret, b); err != nil {
					t.Fatal(err)
				}
			}
			return ret
		}

		run = func(test testcase) error {
			ctx := security.NewContext(&security.ContextParams{
				LocalPrincipal:  pserver,
				LocalBlessings:  server,
				RemoteBlessings: test.Client,
				Method:          test.Method,
				MethodTags:      methodTags(test.Method),
			})
			return authorizer.Authorize(ctx)
		}
	)

	// Test cases where access should be granted to methods with tags on
	// them.
	for _, test := range []testcase{
		{"Get", nil},
		{"Get", B("ali")},
		{"Get", B("bob/friend", "che/enemy")},

		{"Put", B("ali/family/mom")},
		{"Put", B("bob/friends")},
		{"Put", B("bob/acquantainces/carol", "che")}, // Access granted because of "che"

		{"Resolve", B("superman")},
		{"Resolve", B("ali/family/boss")},

		{"AllTags", B("ali/family/boss")},
	} {
		if err := run(test); err != nil {
			t.Errorf("Access denied to method %q to %v: %v", test.Method, test.Client, err)
		}
	}
	// Test cases where access should be denied.
	for _, test := range []testcase{
		// Nobody is denied access to "Get"
		{"Put", B("ali", "bob/acquaintances", "bob/acquaintances/dave", "che/friend", "dave")},
		{"Resolve", B("ali", "ali/friend", "ali/family", "ali/family/friend", "alice/family/boss/friend", "superman/friend")},
		// Since there are no tags on the NoTags method, it has an
		// empty ACL.  No client will have access.
		{"NoTags", B("ali", "ali/family/boss", "bob", "che", "superman")},
		// On a method with multiple tags on it, all must be satisfied.
		{"AllTags", B("che")},               // In R and W, but not in X
		{"AllTags", B("superman", "clark")}, // In R and X, but not W
	} {
		if err := run(test); err == nil {
			t.Errorf("Access to %q granted to %v", test.Method, test.Client)
		}
	}
}

func TestTaggedACLAuthorizerSelfRPCs(t *testing.T) {
	var (
		// Client and server are the same principal, though have
		// different blessings.
		p         = newPrincipal(t)
		client, _ = p.BlessSelf("client")
		server, _ = p.BlessSelf("server")
		// Authorizer with a TaggedACLMap that grants read access to
		// anyone, write/execute access to noone.
		typ           test.MyTag
		authorizer, _ = TaggedACLAuthorizer(TaggedACLMap{"R": {In: []security.BlessingPattern{"nobody/$"}}}, reflect.TypeOf(typ))
	)
	for _, test := range []string{"Put", "Get", "Resolve", "NoTags", "AllTags"} {
		ctx := security.NewContext(&security.ContextParams{
			LocalPrincipal:  p,
			LocalBlessings:  server,
			RemoteBlessings: client,
			Method:          test,
			MethodTags:      methodTags(test),
		})
		if err := authorizer.Authorize(ctx); err != nil {
			t.Errorf("Got error %v for method %q", err, test)
		}
	}
}

func TestTaggedACLAuthorizerWithNilACL(t *testing.T) {
	var (
		authorizer, _ = TaggedACLAuthorizer(nil, reflect.TypeOf(test.Read))
		pserver       = newPrincipal(t)
		pclient       = newPrincipal(t)
		server, _     = pserver.BlessSelf("server")
		client, _     = pclient.BlessSelf("client")
	)
	for _, test := range []string{"Put", "Get", "Resolve", "NoTags", "AllTags"} {
		ctx := security.NewContext(&security.ContextParams{
			LocalPrincipal:  pserver,
			LocalBlessings:  server,
			RemoteBlessings: client,
			Method:          test,
			MethodTags:      methodTags(test),
		})
		if err := authorizer.Authorize(ctx); err == nil {
			t.Errorf("nil TaggedACLMap authorized method %q", test)
		}
	}
}

func TestTaggedACLAuthorizerFromFile(t *testing.T) {
	file, err := ioutil.TempFile("", "TestTaggedACLAuthorizerFromFile")
	if err != nil {
		t.Fatal(err)
	}
	filename := file.Name()
	file.Close()
	defer os.Remove(filename)

	var (
		authorizer, _  = TaggedACLAuthorizerFromFile(filename, reflect.TypeOf(test.Read))
		pserver        = newPrincipal(t)
		pclient        = newPrincipal(t)
		server, _      = pserver.BlessSelf("alice")
		alicefriend, _ = pserver.Bless(pclient.PublicKey(), server, "friend/bob", security.UnconstrainedUse())
		ctx            = security.NewContext(&security.ContextParams{
			LocalPrincipal:  pserver,
			LocalBlessings:  server,
			RemoteBlessings: alicefriend,
			Method:          "Get",
			MethodTags:      methodTags("Get"),
		})
	)
	// Since this test is using trustAllRoots{}, do not need
	// pserver.AddToRoots(server) to make pserver recognize itself as an
	// authority on blessings matching "alice".

	// "alice/friend/bob" should not have access to test.Read methods like Get.
	if err := authorizer.Authorize(ctx); err == nil {
		t.Fatalf("Expected authorization error as %v is not on the ACL for Read operations", ctx.RemoteBlessings())
	}
	// Rewrite the file giving access
	if err := ioutil.WriteFile(filename, []byte(`{"R": { "In":["alice/friend"] }}`), 0600); err != nil {
		t.Fatal(err)
	}
	// Now should have access
	if err := authorizer.Authorize(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestTagTypeMustBeString(t *testing.T) {
	type I int
	if auth, err := TaggedACLAuthorizer(TaggedACLMap{}, reflect.TypeOf(I(0))); err == nil || auth != nil {
		t.Errorf("Got (%v, %v), wanted error since tag type is not a string", auth, err)
	}
	if auth, err := TaggedACLAuthorizerFromFile("does_not_matter", reflect.TypeOf(I(0))); err == nil || auth != nil {
		t.Errorf("Got (%v, %v), wanted error since tag type is not a string", auth, err)
	}
}

func methodTags(name string) []*vdl.Value {
	server := test.MyObjectServer(nil)
	for _, iface := range server.Describe__() {
		for _, method := range iface.Methods {
			if method.Name == name {
				return method.Tags
			}
		}
	}
	return nil
}

func newPrincipal(t *testing.T) security.Principal {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	p, err := security.CreatePrincipal(
		security.NewInMemoryECDSASigner(key),
		nil,
		trustAllRoots{})
	if err != nil {
		t.Fatal(err)
	}
	return p
}

type trustAllRoots struct{}

func (trustAllRoots) Add(security.PublicKey, security.BlessingPattern) error { return nil }
func (trustAllRoots) Recognized(security.PublicKey, string) error            { return nil }
func (trustAllRoots) DebugString() string                                    { return fmt.Sprintf("%T", trustAllRoots{}) }
