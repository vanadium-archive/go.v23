package vstore

import (
	"testing"

	store "veyron/services/store/testutil"

	"veyron2"
	"veyron2/rt"
	"veyron2/security"
	"veyron2/storage"
	"veyron2/storage/vstore/primitives"
	"veyron2/vom"
)

var (
	// ipcID is the identity used by the Runtime for all IPC
	// authentication (both the server and client).
	ipcID = security.FakePrivateID("user")
)

// Open a storage.server in this same test process.
func init() {
	vom.Register(&Dir{})
}

// Dir is a simple directory.
type Dir struct {
	Entries map[string]storage.ID
}

func newServer(t *testing.T) (storage.Store, func()) {
	id := veyron2.LocalID(ipcID)
	r := rt.Init(id)

	server, err := r.NewServer()
	if err != nil {
		t.Fatalf("rt.NewServer() failed: %v", err)
	}
	name, cl := store.NewStore(t, server, r.Identity().PublicID())
	st, err := New(name)
	if err != nil {
		t.Fatalf("vstore.New() failed: %v", err)
	}
	return st, cl
}

func newValue() interface{} {
	return &Dir{}
}

func TestPutGetRemoveRoot(t *testing.T) {
	s, c := newServer(t) // calls rt.Init()
	defer c()

	o := s.Bind("/")
	testPutGetRemove(t, s, o)
}

func TestPutGetRemoveChild(t *testing.T) {
	s, c := newServer(t) // calls rt.Init()
	defer c()

	{
		// Create a root.
		o := s.Bind("/")
		value := newValue()
		tr1 := primitives.NewTransaction()
		if _, err := o.Put(tr1, value); err != nil {
			t.Errorf("Unexpected error: %s", err)
		}
		if err := tr1.Commit(); err != nil {
			t.Errorf("Unexpected error")
		}

		tr2 := primitives.NewTransaction()
		if ok, err := o.Exists(tr2); !ok || err != nil {
			t.Errorf("Should exist: %s", err)
		}
		if _, err := o.Get(tr2); err != nil {
			t.Errorf("Object should exist: %s", err)
		}
	}

	o := s.Bind("/Entries/a")
	testPutGetRemove(t, s, o)
}

func testPutGetRemove(t *testing.T, s storage.Store, o storage.Object) {
	value := newValue()
	{
		// Check that the object does not exist.
		tr := primitives.NewTransaction()
		if ok, err := o.Exists(tr); ok || err != nil {
			t.Errorf("Should not exist: %s", err)
		}
		if v, err := o.Get(tr); !v.Stat.ID.IsValid() && err == nil {
			t.Errorf("Should not exist: %v, %s", v, err)
		}
	}

	{
		// Add the object.
		tr1 := primitives.NewTransaction()
		if _, err := o.Put(tr1, value); err != nil {
			t.Errorf("Unexpected error: %s", err)
		}
		if ok, err := o.Exists(tr1); !ok || err != nil {
			t.Errorf("Should exist: %s", err)
		}
		if _, err := o.Get(tr1); err != nil {
			t.Errorf("Object should exist: %s", err)
		}

		// Transactions are isolated.
		tr2 := primitives.NewTransaction()
		if ok, err := o.Exists(tr2); ok || err != nil {
			t.Errorf("Should not exist: %s", err)
		}
		if v, err := o.Get(tr2); v.Stat.ID.IsValid() && err == nil {
			t.Errorf("Should not exist: %v, %s", v, err)
		}

		// Apply tr1.
		if err := tr1.Commit(); err != nil {
			t.Errorf("Unexpected error")
		}

		// tr2 is still isolated.
		if ok, err := o.Exists(tr2); ok || err != nil {
			t.Errorf("Should not exist: %s", err)
		}
		if v, err := o.Get(tr2); v.Stat.ID.IsValid() && err == nil {
			t.Errorf("Should not exist: %v, %s", v, err)
		}

		// tr3 observes the commit.
		tr3 := primitives.NewTransaction()
		if ok, err := o.Exists(tr3); !ok || err != nil {
			t.Errorf("Should exist: %s", err)
		}
		if _, err := o.Get(tr3); err != nil {
			t.Errorf("Object should exist: %s", err)
		}
	}

	{
		// Remove the object.
		tr1 := primitives.NewTransaction()
		if err := o.Remove(tr1); err != nil {
			t.Errorf("Unexpected error: %s", err)
		}
		if ok, err := o.Exists(tr1); ok || err != nil {
			t.Errorf("Should not exist: %s", err)
		}
		if v, err := o.Get(tr1); v.Stat.ID.IsValid() || err == nil {
			t.Errorf("Object should exist: %v", v)
		}

		// The removal is isolated.
		tr2 := primitives.NewTransaction()
		if ok, err := o.Exists(tr2); !ok || err != nil {
			t.Errorf("Should exist: %s", err)
		}
		if _, err := o.Get(tr2); err != nil {
			t.Errorf("Object should exist: %s", err)
		}

		// Apply tr1.
		if err := tr1.Commit(); err != nil {
			t.Errorf("Unexpected error: %s", err)
		}

		// The removal is isolated.
		if ok, err := o.Exists(tr2); !ok || err != nil {
			t.Errorf("Should exist: %s", err)
		}
		if _, err := o.Get(tr2); err != nil {
			t.Errorf("Object should exist: %s", err)
		}
	}

	{
		// Check that the object does not exist.
		tr1 := primitives.NewTransaction()
		if ok, err := o.Exists(tr1); ok || err != nil {
			t.Errorf("Should not exist")
		}
		if v, err := o.Get(tr1); v.Stat.ID.IsValid() && err == nil {
			t.Errorf("Should not exist: %v, %s", v, err)
		}
	}
}

func TestPutGetRemoveNilTransaction(t *testing.T) {
	s, c := newServer(t) // calls rt.Init()
	defer c()

	{
		// Create a root.
		o := s.Bind("/")
		value := newValue()
		if _, err := o.Put(nil, value); err != nil {
			t.Errorf("Unexpected error: %s", err)
		}
		if ok, err := o.Exists(nil); !ok || err != nil {
			t.Errorf("Should exist: %s", err)
		}
		if _, err := o.Get(nil); err != nil {
			t.Errorf("Object should exist: %s", err)
		}
	}

	o := s.Bind("/Entries/b")
	value := newValue()
	{
		// Check that the object does not exist.
		if ok, err := o.Exists(nil); ok || err != nil {
			t.Errorf("Should not exist: %s", err)
		}
		if v, err := o.Get(nil); v.Stat.ID.IsValid() && err == nil {
			t.Errorf("Should not exist: %v, %s", v, err)
		}
	}

	{
		tr := primitives.NewTransaction()
		if ok, err := o.Exists(tr); ok || err != nil {
			t.Errorf("Should not exist: %s", err)
		}

		// Add the object.
		if _, err := o.Put(nil, value); err != nil {
			t.Errorf("Unexpected error: %s", err)
		}
		if ok, err := o.Exists(nil); !ok || err != nil {
			t.Errorf("Should exist: %s", err)
		}
		if _, err := o.Get(nil); err != nil {
			t.Errorf("Object should exist: %s", err)
		}

		// Transactions are isolated.
		if ok, err := o.Exists(tr); ok || err != nil {
			t.Errorf("Should not exist: %s", err)
		}
		if v, err := o.Get(tr); v.Stat.ID.IsValid() && err == nil {
			t.Errorf("Should not exist: %v, %s", v, err)
		}
		if err := tr.Abort(); err != nil {
			t.Errorf("Unexpected error: %s", err)
		}
	}

	{
		tr := primitives.NewTransaction()
		if ok, err := o.Exists(tr); !ok || err != nil {
			t.Errorf("Should exist: %s", err)
		}

		// Remove the object.
		if err := o.Remove(nil); err != nil {
			t.Errorf("Unexpected error: %s", err)
		}
		if ok, err := o.Exists(nil); ok || err != nil {
			t.Errorf("Should not exist: %s", err)
		}
		if v, err := o.Get(nil); v.Stat.ID.IsValid() || err == nil {
			t.Errorf("Object should exist: %v", v)
		}

		// The removal is isolated.
		if ok, err := o.Exists(tr); !ok || err != nil {
			t.Errorf("Should exist: %s", err)
		}
		if _, err := o.Get(tr); err != nil {
			t.Errorf("Object should exist: %s", err)
		}
		if err := tr.Abort(); err != nil {
			t.Errorf("Unexpected error: %s", err)
		}
	}
}
