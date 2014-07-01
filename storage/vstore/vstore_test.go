package vstore_test

import (
	"testing"

	store "veyron/services/store/testutil"

	"veyron2"
	"veyron2/rt"
	"veyron2/security"
	"veyron2/services/watch"
	"veyron2/storage"
	"veyron2/storage/vstore"
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
	st, err := vstore.New(name)
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
	ctx := rt.R().NewContext()
	s, c := newServer(t) // calls rt.Init()
	defer c()

	{
		// Create a root.
		o := s.Bind("/")
		value := newValue()
		tr1 := primitives.NewTransaction(ctx)
		if _, err := o.Put(ctx, tr1, value); err != nil {
			t.Errorf("Unexpected error: %s", err)
		}
		if err := tr1.Commit(ctx); err != nil {
			t.Errorf("Unexpected error")
		}

		tr2 := primitives.NewTransaction(ctx)
		if ok, err := o.Exists(ctx, tr2); !ok || err != nil {
			t.Errorf("Should exist: %s", err)
		}
		if _, err := o.Get(ctx, tr2); err != nil {
			t.Errorf("Object should exist: %s", err)
		}
	}

	o := s.Bind("/Entries/a")
	testPutGetRemove(t, s, o)
}

func testPutGetRemove(t *testing.T, s storage.Store, o storage.Object) {
	value := newValue()
	ctx := rt.R().NewContext()
	{
		// Check that the object does not exist.
		tr := primitives.NewTransaction(ctx)
		if ok, err := o.Exists(ctx, tr); ok || err != nil {
			t.Errorf("Should not exist: %s", err)
		}
		if v, err := o.Get(ctx, tr); !v.Stat.ID.IsValid() && err == nil {
			t.Errorf("Should not exist: %v, %s", v, err)
		}
	}

	{
		// Add the object.
		tr1 := primitives.NewTransaction(ctx)
		if _, err := o.Put(ctx, tr1, value); err != nil {
			t.Errorf("Unexpected error: %s", err)
		}
		if ok, err := o.Exists(ctx, tr1); !ok || err != nil {
			t.Errorf("Should exist: %s", err)
		}
		if _, err := o.Get(ctx, tr1); err != nil {
			t.Errorf("Object should exist: %s", err)
		}

		// Transactions are isolated.
		tr2 := primitives.NewTransaction(ctx)
		if ok, err := o.Exists(ctx, tr2); ok || err != nil {
			t.Errorf("Should not exist: %s", err)
		}
		if v, err := o.Get(ctx, tr2); v.Stat.ID.IsValid() && err == nil {
			t.Errorf("Should not exist: %v, %s", v, err)
		}

		// Apply tr1.
		if err := tr1.Commit(ctx); err != nil {
			t.Errorf("Unexpected error")
		}

		// tr2 is still isolated.
		if ok, err := o.Exists(ctx, tr2); ok || err != nil {
			t.Errorf("Should not exist: %s", err)
		}
		if v, err := o.Get(ctx, tr2); v.Stat.ID.IsValid() && err == nil {
			t.Errorf("Should not exist: %v, %s", v, err)
		}

		// tr3 observes the commit.
		tr3 := primitives.NewTransaction(ctx)
		if ok, err := o.Exists(ctx, tr3); !ok || err != nil {
			t.Errorf("Should exist: %s", err)
		}
		if _, err := o.Get(ctx, tr3); err != nil {
			t.Errorf("Object should exist: %s", err)
		}
	}

	{
		// Remove the object.
		tr1 := primitives.NewTransaction(ctx)
		if err := o.Remove(ctx, tr1); err != nil {
			t.Errorf("Unexpected error: %s", err)
		}
		if ok, err := o.Exists(ctx, tr1); ok || err != nil {
			t.Errorf("Should not exist: %s", err)
		}
		if v, err := o.Get(ctx, tr1); v.Stat.ID.IsValid() || err == nil {
			t.Errorf("Object should exist: %v", v)
		}

		// The removal is isolated.
		tr2 := primitives.NewTransaction(ctx)
		if ok, err := o.Exists(ctx, tr2); !ok || err != nil {
			t.Errorf("Should exist: %s", err)
		}
		if _, err := o.Get(ctx, tr2); err != nil {
			t.Errorf("Object should exist: %s", err)
		}

		// Apply tr1.
		if err := tr1.Commit(ctx); err != nil {
			t.Errorf("Unexpected error: %s", err)
		}

		// The removal is isolated.
		if ok, err := o.Exists(ctx, tr2); !ok || err != nil {
			t.Errorf("Should exist: %s", err)
		}
		if _, err := o.Get(ctx, tr2); err != nil {
			t.Errorf("Object should exist: %s", err)
		}
	}

	{
		// Check that the object does not exist.
		tr1 := primitives.NewTransaction(ctx)
		if ok, err := o.Exists(ctx, tr1); ok || err != nil {
			t.Errorf("Should not exist")
		}
		if v, err := o.Get(ctx, tr1); v.Stat.ID.IsValid() && err == nil {
			t.Errorf("Should not exist: %v, %s", v, err)
		}
	}
}

func TestPutGetRemoveNilTransaction(t *testing.T) {
	ctx := rt.R().NewContext()
	s, c := newServer(t) // calls rt.Init()
	defer c()

	{
		// Create a root.
		o := s.Bind("/")
		value := newValue()
		if _, err := o.Put(ctx, nil, value); err != nil {
			t.Errorf("Unexpected error: %s", err)
		}
		if ok, err := o.Exists(ctx, nil); !ok || err != nil {
			t.Errorf("Should exist: %s", err)
		}
		if _, err := o.Get(ctx, nil); err != nil {
			t.Errorf("Object should exist: %s", err)
		}
	}

	o := s.Bind("/Entries/b")
	value := newValue()
	{
		// Check that the object does not exist.
		if ok, err := o.Exists(ctx, nil); ok || err != nil {
			t.Errorf("Should not exist: %s", err)
		}
		if v, err := o.Get(ctx, nil); v.Stat.ID.IsValid() && err == nil {
			t.Errorf("Should not exist: %v, %s", v, err)
		}
	}

	{
		tr := primitives.NewTransaction(ctx)
		if ok, err := o.Exists(ctx, tr); ok || err != nil {
			t.Errorf("Should not exist: %s", err)
		}

		// Add the object.
		if _, err := o.Put(ctx, nil, value); err != nil {
			t.Errorf("Unexpected error: %s", err)
		}
		if ok, err := o.Exists(ctx, nil); !ok || err != nil {
			t.Errorf("Should exist: %s", err)
		}
		if _, err := o.Get(ctx, nil); err != nil {
			t.Errorf("Object should exist: %s", err)
		}

		// Transactions are isolated.
		if ok, err := o.Exists(ctx, tr); ok || err != nil {
			t.Errorf("Should not exist: %s", err)
		}
		if v, err := o.Get(ctx, tr); v.Stat.ID.IsValid() && err == nil {
			t.Errorf("Should not exist: %v, %s", v, err)
		}
		if err := tr.Abort(ctx); err != nil {
			t.Errorf("Unexpected error: %s", err)
		}
	}

	{
		tr := primitives.NewTransaction(ctx)
		if ok, err := o.Exists(ctx, tr); !ok || err != nil {
			t.Errorf("Should exist: %s", err)
		}

		// Remove the object.
		if err := o.Remove(ctx, nil); err != nil {
			t.Errorf("Unexpected error: %s", err)
		}
		if ok, err := o.Exists(ctx, nil); ok || err != nil {
			t.Errorf("Should not exist: %s", err)
		}
		if v, err := o.Get(ctx, nil); v.Stat.ID.IsValid() || err == nil {
			t.Errorf("Object should exist: %v", v)
		}

		// The removal is isolated.
		if ok, err := o.Exists(ctx, tr); !ok || err != nil {
			t.Errorf("Should exist: %s", err)
		}
		if _, err := o.Get(ctx, tr); err != nil {
			t.Errorf("Object should exist: %s", err)
		}
		if err := tr.Abort(ctx); err != nil {
			t.Errorf("Unexpected error: %s", err)
		}
	}
}

func TestWatchGlob(t *testing.T) {
	ctx := rt.R().NewContext()
	s, c := newServer(t) // calls rt.Init()
	defer c()

	root := s.Bind("/")

	// Create the root.
	rootValue := "root-val"
	stat, err := root.Put(ctx, nil, rootValue)
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}

	// Watch all objects under the root.
	req := watch.GlobRequest{Pattern: "..."}
	stream, err := root.WatchGlob(ctx, req)
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}
	defer stream.Cancel()

	// Expect a change adding /.
	cb, err := stream.Recv()
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}
	changes := cb.Changes
	if len(changes) != 1 {
		t.Fatalf("Expected 1 change, but got %d", len(changes))
	}
	entry := findEntry(t, changes, "")
	if entry.Value != rootValue {
		t.Fatalf("Expected value to be %v, but was %v.", rootValue, entry.Value)
	}
	if entry.Stat.ID != stat.ID {
		t.Fatalf("Expected stat to be %v, but was %v.", stat, entry.Stat)
	}

	// Create /a.
	a := s.Bind("/a")
	aValue := "a-val"
	stat, err = a.Put(ctx, nil, aValue)
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}

	// Expect changes updating / and adding /a.
	cb, err = stream.Recv()
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}
	changes = cb.Changes
	if len(changes) != 2 {
		t.Fatalf("Expected 2 changes, but got %d", len(changes))
	}
	findEntry(t, changes, "")
	entry = findEntry(t, changes, "a")
	if entry.Value != aValue {
		t.Fatalf("Expected value to be %v, but was %v.", aValue, entry.Value)
	}
	if entry.Stat.ID != stat.ID {
		t.Fatalf("Expected stat to be %v, but was %v.", stat, entry.Stat)
	}
}

func findEntry(t *testing.T, changes []watch.Change, name string) *storage.Entry {
	for _, change := range changes {
		if change.Name == name {
			entry, ok := change.Value.(*storage.Entry)
			if !ok {
				t.Fatalf("Expected value to be an entry, but was %#v", change.Value)
			}
			return entry
		}
	}
	t.Fatalf("Expected a change for name: %v", name)
	panic("Should not reach here")
}
