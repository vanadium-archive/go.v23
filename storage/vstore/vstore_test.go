package vstore_test

import (
	"reflect"
	"runtime"
	"sort"
	"testing"
	"veyron2/vom"

	store "veyron/services/store/testutil"

	"veyron2"
	"veyron2/context"
	"veyron2/query"
	"veyron2/rt"
	"veyron2/security"
	"veyron2/services/watch/types"
	"veyron2/storage"
	"veyron2/storage/vstore"
)

var (
	// ipcID is the identity used by the Runtime for all IPC
	// authentication (both the server and client).
	ipcID = security.FakePrivateID("user")
)

func init() {
	vom.Register(&SimpleValue{})
	vom.Register(&storage.Entry{})
}

type SimpleValue struct {
	Desc string
}

// Open a storage server in this same test process.
func newServer(t *testing.T) (root storage.Dir, shutdown func()) {
	id := veyron2.RuntimeID(ipcID)
	r := rt.Init(id)

	server, err := r.NewServer()
	if err != nil {
		t.Fatalf("rt.NewServer() failed: %v", err)
	}
	storeRoot, shutdown := store.NewStore(t, server, r.Identity().PublicID())
	root = vstore.BindDir(storeRoot)
	return root, shutdown
}

func newValue() interface{} {
	return &SimpleValue{"foo"}
}

func TestPutGetRemoveObject(t *testing.T) {
	root, c := newServer(t) // calls rt.Init()
	defer c()
	value := newValue()
	ctx := rt.R().NewContext()
	name := "a"
	{
		// Check that the object does not exist.
		tobj1, _ := root.BindObject(name).NewTransaction(ctx)
		if ok, err := tobj1.Exists(ctx); ok || err != nil {
			t.Fatalf("Should not exist: %s", err)
		}
		if v, err := tobj1.Get(ctx); !v.Stat.ID.IsValid() && err == nil {
			t.Fatalf("Should not exist: %v, %s", v, err)
		}
	}

	{
		// Add the object.
		tobj1, tx1 := root.BindObject(name).NewTransaction(ctx)
		if _, err := tobj1.Put(ctx, value); err != nil {
			t.Fatalf("Unexpected error: %s", err)
		}
		if ok, err := tobj1.Exists(ctx); !ok || err != nil {
			t.Fatalf("Should exist: %s", err)
		}
		if _, err := tobj1.Get(ctx); err != nil {
			t.Fatalf("Object should exist: %s", err)
		}

		// Transactions are isolated.
		tobj2, _ := root.BindObject(name).NewTransaction(ctx)
		if ok, err := tobj2.Exists(ctx); ok || err != nil {
			t.Fatalf("Should not exist: %s", err)
		}
		if v, err := tobj2.Get(ctx); v.Stat.ID.IsValid() && err == nil {
			t.Fatalf("Should not exist: %v, %s", v, err)
		}

		// Apply tobj1.
		if err := tx1.Commit(ctx); err != nil {
			t.Fatalf("Unexpected error")
		}

		// tobj2 is still isolated.
		if ok, err := tobj2.Exists(ctx); ok || err != nil {
			t.Fatalf("Should not exist: %s", err)
		}
		if v, err := tobj2.Get(ctx); v.Stat.ID.IsValid() && err == nil {
			t.Fatalf("Should not exist: %v, %s", v, err)
		}

		// tobj3 observes the commit.
		tobj3, _ := root.BindObject(name).NewTransaction(ctx)
		if ok, err := tobj3.Exists(ctx); !ok || err != nil {
			t.Fatalf("Should exist: %s", err)
		}
		if _, err := tobj3.Get(ctx); err != nil {
			t.Fatalf("Object should exist: %s", err)
		}
	}

	{
		// Remove the object.
		tobj1, tx1 := root.BindObject(name).NewTransaction(ctx)
		if err := tobj1.Remove(ctx); err != nil {
			t.Fatalf("Unexpected error: %s", err)
		}
		if ok, err := tobj1.Exists(ctx); ok || err != nil {
			t.Fatalf("Should not exist: %s", err)
		}
		if v, err := tobj1.Get(ctx); v.Stat.ID.IsValid() || err == nil {
			t.Fatalf("Object should exist: %v", v)
		}

		// The removal is isolated.
		tobj2, _ := root.BindObject(name).NewTransaction(ctx)
		if ok, err := tobj2.Exists(ctx); !ok || err != nil {
			t.Fatalf("Should exist: %s", err)
		}
		if _, err := tobj2.Get(ctx); err != nil {
			t.Fatalf("Object should exist: %s", err)
		}

		// Apply tobj1.
		if err := tx1.Commit(ctx); err != nil {
			t.Fatalf("Unexpected error: %s", err)
		}

		// The removal is isolated.
		if ok, err := tobj2.Exists(ctx); !ok || err != nil {
			t.Fatalf("Should exist: %s", err)
		}
		if _, err := tobj2.Get(ctx); err != nil {
			t.Fatalf("Object should exist: %s", err)
		}
	}

	{
		// Check that the object does not exist.
		tobj1, _ := root.BindObject(name).NewTransaction(ctx)
		if ok, err := tobj1.Exists(ctx); ok || err != nil {
			t.Fatalf("Should not exist")
		}
		if v, err := tobj1.Get(ctx); v.Stat.ID.IsValid() && err == nil {
			t.Fatalf("Should not exist: %v, %s", v, err)
		}
	}
}

func TestPutGetRemoveNoTransaction(t *testing.T) {
	root, c := newServer(t) // calls rt.Init()
	defer c()
	ctx := rt.R().NewContext()

	name := "b"
	value := newValue()
	o := root.BindObject(name)
	{
		// Check that the object does not exist.
		if ok, err := o.Exists(ctx); ok || err != nil {
			t.Fatalf("Should not exist: %s", err)
		}
		if v, err := o.Get(ctx); v.Stat.ID.IsValid() && err == nil {
			t.Fatalf("Should not exist: %v, %s", v, err)
		}
	}

	{
		tobj1, tx1 := o.NewTransaction(ctx)
		if ok, err := tobj1.Exists(ctx); ok || err != nil {
			t.Fatalf("Should not exist: %s", err)
		}

		// Add the object.
		if _, err := o.Put(ctx, value); err != nil {
			t.Fatalf("Unexpected error: %s", err)
		}
		if ok, err := o.Exists(ctx); !ok || err != nil {
			t.Fatalf("Should exist: %s", err)
		}
		if _, err := o.Get(ctx); err != nil {
			t.Fatalf("Object should exist: %s", err)
		}

		// Transactions are isolated.
		if ok, err := tobj1.Exists(ctx); ok || err != nil {
			t.Fatalf("Should not exist: %s", err)
		}
		if v, err := tobj1.Get(ctx); v.Stat.ID.IsValid() && err == nil {
			t.Fatalf("Should not exist: %v, %s", v, err)
		}
		if err := tx1.Abort(ctx); err != nil {
			t.Fatalf("Unexpected error: %s", err)
		}
	}

	{
		tobj1, tx1 := o.NewTransaction(ctx)
		if ok, err := tobj1.Exists(ctx); !ok || err != nil {
			t.Fatalf("Should exist: %s", err)
		}

		// Remove the object.
		if err := o.Remove(ctx); err != nil {
			t.Fatalf("Unexpected error: %s", err)
		}
		if ok, err := o.Exists(ctx); ok || err != nil {
			t.Fatalf("Should not exist: %s", err)
		}
		if v, err := o.Get(ctx); v.Stat.ID.IsValid() || err == nil {
			t.Fatalf("Object should exist: %v", v)
		}

		// The removal is isolated.
		if ok, err := tobj1.Exists(ctx); !ok || err != nil {
			t.Fatalf("Should exist: %s", err)
		}
		if _, err := tobj1.Get(ctx); err != nil {
			t.Fatalf("Object should exist: %s", err)
		}
		if err := tx1.Abort(ctx); err != nil {
			t.Fatalf("Unexpected error: %s", err)
		}
	}
}

func TestRelativeNames(t *testing.T) {
	root, c := newServer(t) // calls rt.Init()
	defer c()
	ctx := rt.R().NewContext()

	troot, tx := root.NewTransaction(ctx)
	a := troot.BindDir("a")
	if err := a.Make(ctx); err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}
	b := a.BindObject("b")
	if _, err := b.Put(ctx, newValue()); err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}
	if ok, err := root.BindObject("a/b").Exists(ctx); !ok || err != nil {
		t.Fatalf("Should exist: %s", err)
	}
}

func TestRelativeNamesNilTransaction(t *testing.T) {
	root, c := newServer(t) // calls rt.Init()
	defer c()
	ctx := rt.R().NewContext()

	a := root.BindDir("a")
	if err := a.Make(ctx); err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}
	b := a.BindObject("b")
	if _, err := b.Put(ctx, newValue()); err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}
	if ok, err := root.BindObject("a/b").Exists(ctx); !ok || err != nil {
		t.Fatalf("Should exist: %s", err)
	}
}

func TestWatchGlob(t *testing.T) {
	root, c := newServer(t) // calls rt.Init()
	defer c()
	ctx := rt.R().NewContext()

	// Watch all objects under the root.
	req := types.GlobRequest{Pattern: "..."}
	stream, err := root.WatchGlob(ctx, req)
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}
	defer stream.Cancel()
	rStream := stream.RecvStream()

	// Create /a.
	a := root.BindObject("a")
	aValue := "a-val"
	stat, err := a.Put(ctx, aValue)
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}

	// Expect changes: 1) creating /, 2) updating /, 3) adding /a.
	changes := []types.Change{}
	for i := 0; i < 3; i++ {
		if !rStream.Advance() {
			t.Fatalf("Advance() failed: %v", rStream.Err())
		}
		changes = append(changes, rStream.Value())
	}
	findEntry(t, changes, "")
	entry := findEntry(t, changes, "a")
	if entry.Value != aValue {
		t.Fatalf("Expected value to be %v, but was %v.", aValue, entry.Value)
	}
	if entry.Stat.ID != stat.ID {
		t.Fatalf("Expected stat to be %v, but was %v.", stat, entry.Stat)
	}
}

func findEntry(t *testing.T, changes []types.Change, name string) *storage.Entry {
	for _, change := range changes {
		if change.Name == name {
			entry, ok := change.Value.(*storage.Entry)
			if !ok {
				t.Fatalf("Expected value to be an entry, but was %#v", change.Value)
			}
			return entry
		}
	}
	t.Fatalf("Expected a change for name: %v; got %v", name, changes)
	panic("Should not reach here")
}

type player struct {
	Name string
	Age  int
}

// TODO(sadovsky): A similar helper function exists in photoalbum_test.go.
// We should unify such helper functions and put them in a common location.
func put(t *testing.T, root storage.Dir, ctx context.T, name string, v interface{}) {
	_, file, line, _ := runtime.Caller(1)
	_, err := root.BindObject(name).Put(ctx, v)
	if err != nil {
		t.Fatalf("%s(%d): can't put %s: %s", file, line, name, err)
	}
}

func mkdir(t *testing.T, root storage.Dir, ctx context.T, name string) {
	_, file, line, _ := runtime.Caller(1)
	if err := root.BindDir(name).Make(ctx); err != nil {
		t.Fatalf("%s(%d): can't mkdir %s: %s", file, line, name, err)
	}
}

func expectResultNames(t *testing.T, query string, stream storage.QueryStream, expectedNames []string) {
	_, file, line, _ := runtime.Caller(1)
	var resultNames []string
	for stream.Advance() {
		result := stream.Value()
		resultNames = append(resultNames, result.Name())
	}
	if err := stream.Err(); err != nil {
		t.Errorf("%s:%d stream error: %v", err)
	}
	if !reflect.DeepEqual(resultNames, expectedNames) {
		t.Errorf("%s:%d query: %s\nGOT  %v\nWANT %v", file, line, query, resultNames, expectedNames)
	}
}

func TestQuery(t *testing.T) {
	root, c := newServer(t) // calls rt.Init()
	defer c()
	ctx := rt.R().NewContext()

	mkdir(t, root, ctx, "players")
	put(t, root, ctx, "players/alfred", player{"alfred", 17})
	put(t, root, ctx, "players/alice", player{"alice", 16})
	put(t, root, ctx, "players/betty", player{"betty", 23})
	put(t, root, ctx, "players/bob", player{"bob", 21})

	type testCase struct {
		query       string
		resultNames []string
	}

	tests := []testCase{
		{
			"players/* | type player | sort()",
			[]string{"players/alfred", "players/alice", "players/betty", "players/bob"},
		},
		{
			"players/* | type player | ? Age > 20 | sort(Age)",
			[]string{"players/bob", "players/betty"},
		},
	}

	for _, test := range tests {
		stream := root.Query(ctx, query.Query{test.query})
		expectResultNames(t, test.query, stream, test.resultNames)
	}
}

func TestQueryInTransaction(t *testing.T) {
	root, c := newServer(t) // calls rt.Init()
	defer c()
	ctx := rt.R().NewContext()

	mkdir(t, root, ctx, "players")
	put(t, root, ctx, "players/alfred", player{"alfred", 17})
	put(t, root, ctx, "players/alice", player{"alice", 16})

	tdir1, tx1 := root.NewTransaction(ctx)

	const allPlayers = "players/* | type player | sort()"
	stream := tdir1.Query(ctx, query.Query{allPlayers})
	expectResultNames(t, allPlayers, stream, []string{"players/alfred", "players/alice"})

	// Query should see mutations that are part of the transaction.
	if _, err := tdir1.BindObject("players/betty").Put(ctx, player{"betty", 23}); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if _, err := tdir1.BindObject("players/bob").Put(ctx, player{"bob", 21}); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	stream = tdir1.Query(ctx, query.Query{allPlayers})
	expectResultNames(t, allPlayers, stream,
		[]string{"players/alfred", "players/alice", "players/betty", "players/bob"})

	// Query should not see mutations that are part of an uncommitted transaction.
	tdir2, _ := root.NewTransaction(ctx)
	stream = tdir2.Query(ctx, query.Query{allPlayers})
	expectResultNames(t, allPlayers, stream, []string{"players/alfred", "players/alice"})

	if err := tx1.Commit(ctx); err != nil {
		t.Fatalf("Unexpected error")
	}

	// tdir2 should still not see the mutations from tobj1.
	stream = tdir2.Query(ctx, query.Query{allPlayers})
	expectResultNames(t, allPlayers, stream, []string{"players/alfred", "players/alice"})

	// A new transaction should see the mutations from tobj1.
	tdir3, _ := root.NewTransaction(ctx)
	stream = tdir3.Query(ctx, query.Query{allPlayers})
	expectResultNames(t, allPlayers, stream,
		[]string{"players/alfred", "players/alice", "players/betty", "players/bob"})
}

func expectValues(t *testing.T, pattern string, call storage.GlobCall, expectedValues []string) {
	_, file, line, _ := runtime.Caller(1)
	var values []string
	stream := call.RecvStream()
	for stream.Advance() {
		values = append(values, stream.Value())
	}
	if err := stream.Err(); err != nil {
		t.Errorf("%s:%d stream error: %v", err)
	}
	// TODO(sadovsky): For now, we do not test that items have the same
	// order. Should Glob return results in deterministic (e.g. alphabetical)
	// order?
	sort.Strings(values)
	sort.Strings(expectedValues)
	if !reflect.DeepEqual(values, expectedValues) {
		t.Errorf("%s:%d pattern: %s\nGOT  %v\nWANT %v", file, line, pattern, values, expectedValues)
	}
}

func TestGlob(t *testing.T) {
	root, c := newServer(t) // calls rt.Init()
	defer c()
	ctx := rt.R().NewContext()

	const pattern = "players/*"

	mkdir(t, root, ctx, "players")
	put(t, root, ctx, "players/alice", player{"alice", 16})
	put(t, root, ctx, "players/bob", player{"bob", 21})

	stream := root.Glob(ctx, pattern)
	expectValues(t, pattern, stream, []string{"players/alice", "players/bob"})
}

func TestGlobInTransaction(t *testing.T) {
	root, c := newServer(t) // calls rt.Init()
	defer c()
	ctx := rt.R().NewContext()

	const pattern = "players/*"

	mkdir(t, root, ctx, "players")
	put(t, root, ctx, "players/alfred", player{"alfred", 17})
	put(t, root, ctx, "players/alice", player{"alice", 16})

	tdir1, tx1 := root.NewTransaction(ctx)
	stream := tdir1.Glob(ctx, pattern)
	expectValues(t, pattern, stream, []string{"players/alfred", "players/alice"})

	// Glob should see mutations that are part of the transaction.
	if _, err := tdir1.BindObject("players/betty").Put(ctx, player{"betty", 23}); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if _, err := tdir1.BindObject("players/bob").Put(ctx, player{"bob", 21}); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	stream = tdir1.Glob(ctx, pattern)
	expectValues(t, pattern, stream,
		[]string{"players/alfred", "players/alice", "players/betty", "players/bob"})

	// Glob should not see mutations that are part of an uncommitted transaction.
	tdir2, _ := root.NewTransaction(ctx)
	stream = tdir2.Glob(ctx, pattern)
	expectValues(t, pattern, stream, []string{"players/alfred", "players/alice"})

	if err := tx1.Commit(ctx); err != nil {
		t.Fatalf("Unexpected error")
	}

	// tdir2 should still not see the mutations from tdir1.
	stream = tdir2.Glob(ctx, pattern)
	expectValues(t, pattern, stream, []string{"players/alfred", "players/alice"})

	// A new transaction should see the mutations from tdir2.
	tdir3, _ := root.NewTransaction(ctx)
	stream = tdir3.Glob(ctx, pattern)
	expectValues(t, pattern, stream,
		[]string{"players/alfred", "players/alice", "players/betty", "players/bob"})
}
