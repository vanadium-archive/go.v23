package vstore_test

import (
	"reflect"
	"runtime"
	"sort"
	"testing"

	store "veyron/services/store/testutil"

	"veyron2"
	"veyron2/context"
	"veyron2/naming"
	"veyron2/query"
	"veyron2/rt"
	"veyron2/security"
	"veyron2/services/watch/types"
	"veyron2/storage"
	"veyron2/storage/vstore"
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

func newServer(t *testing.T) (storeName string, shutdown func()) {
	id := veyron2.RuntimeID(ipcID)
	r := rt.Init(id)

	server, err := r.NewServer()
	if err != nil {
		t.Fatalf("rt.NewServer() failed: %v", err)
	}
	return store.NewStore(t, server, r.Identity().PublicID())
}

func newValue() interface{} {
	return &Dir{}
}

func TestPutGetRemoveRoot(t *testing.T) {
	root, c := newServer(t) // calls rt.Init()
	defer c()

	testPutGetRemove(t, root, "/")
}

func TestPutGetRemoveChild(t *testing.T) {
	root, c := newServer(t) // calls rt.Init()
	defer c()
	ctx := rt.R().NewContext()
	st := vstore.New()

	// Create a root.
	tobj1 := st.Bind(root)
	if _, err := tobj1.Put(ctx, newValue()); err != nil {
		t.Errorf("Unexpected error: %s", err)
	}

	testPutGetRemove(t, root, "/Entries/a")
}

func testPutGetRemove(t *testing.T, txRoot string, name string) {
	value := newValue()
	ctx := rt.R().NewContext()
	st := vstore.New()
	{
		// Check that the object does not exist.
		tx1 := st.NewTransaction(ctx, txRoot)
		tobj1 := tx1.Bind(name)
		if ok, err := tobj1.Exists(ctx); ok || err != nil {
			t.Errorf("Should not exist: %s", err)
		}
		if v, err := tobj1.Get(ctx); !v.Stat.ID.IsValid() && err == nil {
			t.Errorf("Should not exist: %v, %s", v, err)
		}
	}

	{
		// Add the object.
		tx1 := st.NewTransaction(ctx, txRoot)
		tobj1 := tx1.Bind(name)
		if _, err := tobj1.Put(ctx, value); err != nil {
			t.Errorf("Unexpected error: %s", err)
		}
		if ok, err := tobj1.Exists(ctx); !ok || err != nil {
			t.Errorf("Should exist: %s", err)
		}
		if _, err := tobj1.Get(ctx); err != nil {
			t.Errorf("Object should exist: %s", err)
		}

		// Transactions are isolated.
		tx2 := st.NewTransaction(ctx, txRoot)
		tobj2 := tx2.Bind(name)
		if ok, err := tobj2.Exists(ctx); ok || err != nil {
			t.Errorf("Should not exist: %s", err)
		}
		if v, err := tobj2.Get(ctx); v.Stat.ID.IsValid() && err == nil {
			t.Errorf("Should not exist: %v, %s", v, err)
		}

		// Apply tobj1.
		if err := tx1.Commit(ctx); err != nil {
			t.Errorf("Unexpected error")
		}

		// tobj2 is still isolated.
		if ok, err := tobj2.Exists(ctx); ok || err != nil {
			t.Errorf("Should not exist: %s", err)
		}
		if v, err := tobj2.Get(ctx); v.Stat.ID.IsValid() && err == nil {
			t.Errorf("Should not exist: %v, %s", v, err)
		}

		// tobj3 observes the commit.
		tx3 := st.NewTransaction(ctx, txRoot)
		tobj3 := tx3.Bind(name)
		if ok, err := tobj3.Exists(ctx); !ok || err != nil {
			t.Errorf("Should exist: %s", err)
		}
		if _, err := tobj3.Get(ctx); err != nil {
			t.Errorf("Object should exist: %s", err)
		}
	}

	{
		// Remove the object.
		tx1 := st.NewTransaction(ctx, txRoot)
		tobj1 := tx1.Bind(name)
		if err := tobj1.Remove(ctx); err != nil {
			t.Errorf("Unexpected error: %s", err)
		}
		if ok, err := tobj1.Exists(ctx); ok || err != nil {
			t.Errorf("Should not exist: %s", err)
		}
		if v, err := tobj1.Get(ctx); v.Stat.ID.IsValid() || err == nil {
			t.Errorf("Object should exist: %v", v)
		}

		// The removal is isolated.
		tx2 := st.NewTransaction(ctx, txRoot)
		tobj2 := tx2.Bind(name)
		if ok, err := tobj2.Exists(ctx); !ok || err != nil {
			t.Errorf("Should exist: %s", err)
		}
		if _, err := tobj2.Get(ctx); err != nil {
			t.Errorf("Object should exist: %s", err)
		}

		// Apply tobj1.
		if err := tx1.Commit(ctx); err != nil {
			t.Errorf("Unexpected error: %s", err)
		}

		// The removal is isolated.
		if ok, err := tobj2.Exists(ctx); !ok || err != nil {
			t.Errorf("Should exist: %s", err)
		}
		if _, err := tobj2.Get(ctx); err != nil {
			t.Errorf("Object should exist: %s", err)
		}
	}

	{
		// Check that the object does not exist.
		tx1 := st.NewTransaction(ctx, txRoot)
		tobj1 := tx1.Bind(name)
		if ok, err := tobj1.Exists(ctx); ok || err != nil {
			t.Errorf("Should not exist")
		}
		if v, err := tobj1.Get(ctx); v.Stat.ID.IsValid() && err == nil {
			t.Errorf("Should not exist: %v, %s", v, err)
		}
	}
}

func TestPutGetRemoveNilTransaction(t *testing.T) {
	storeRoot, c := newServer(t) // calls rt.Init()
	defer c()
	ctx := rt.R().NewContext()
	st := vstore.New()

	// Create a root.
	tobj1 := st.Bind(storeRoot)
	if _, err := tobj1.Put(ctx, newValue()); err != nil {
		t.Errorf("Unexpected error: %s", err)
	}

	name := "/Entries/b"
	value := newValue()
	o := st.Bind(naming.Join(storeRoot, name))
	{
		// Check that the object does not exist.
		if ok, err := o.Exists(ctx); ok || err != nil {
			t.Errorf("Should not exist: %s", err)
		}
		if v, err := o.Get(ctx); v.Stat.ID.IsValid() && err == nil {
			t.Errorf("Should not exist: %v, %s", v, err)
		}
	}

	{
		tx1 := st.NewTransaction(ctx, storeRoot)
		tobj1 := tx1.Bind(name)
		if ok, err := tobj1.Exists(ctx); ok || err != nil {
			t.Errorf("Should not exist: %s", err)
		}

		// Add the object.
		if _, err := o.Put(ctx, value); err != nil {
			t.Errorf("Unexpected error: %s", err)
		}
		if ok, err := o.Exists(ctx); !ok || err != nil {
			t.Errorf("Should exist: %s", err)
		}
		if _, err := o.Get(ctx); err != nil {
			t.Errorf("Object should exist: %s", err)
		}

		// Transactions are isolated.
		if ok, err := tobj1.Exists(ctx); ok || err != nil {
			t.Errorf("Should not exist: %s", err)
		}
		if v, err := tobj1.Get(ctx); v.Stat.ID.IsValid() && err == nil {
			t.Errorf("Should not exist: %v, %s", v, err)
		}
		if err := tx1.Abort(ctx); err != nil {
			t.Errorf("Unexpected error: %s", err)
		}
	}

	{
		tx1 := st.NewTransaction(ctx, storeRoot)
		tobj1 := tx1.Bind(name)
		if ok, err := tobj1.Exists(ctx); !ok || err != nil {
			t.Errorf("Should exist: %s", err)
		}

		// Remove the object.
		if err := o.Remove(ctx); err != nil {
			t.Errorf("Unexpected error: %s", err)
		}
		if ok, err := o.Exists(ctx); ok || err != nil {
			t.Errorf("Should not exist: %s", err)
		}
		if v, err := o.Get(ctx); v.Stat.ID.IsValid() || err == nil {
			t.Errorf("Object should exist: %v", v)
		}

		// The removal is isolated.
		if ok, err := tobj1.Exists(ctx); !ok || err != nil {
			t.Errorf("Should exist: %s", err)
		}
		if _, err := tobj1.Get(ctx); err != nil {
			t.Errorf("Object should exist: %s", err)
		}
		if err := tx1.Abort(ctx); err != nil {
			t.Errorf("Unexpected error: %s", err)
		}
	}
}

func TestRelativeNames(t *testing.T) {
	storeRoot, c := newServer(t) // calls rt.Init()
	defer c()
	ctx := rt.R().NewContext()
	st := vstore.New()

	tx := st.NewTransaction(ctx, storeRoot)
	root := tx.Bind("")
	if _, err := root.Put(ctx, newValue()); err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}
	a := root.Bind("a")
	if _, err := a.Put(ctx, newValue()); err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}
	b := a.Bind("b")
	if _, err := b.Put(ctx, newValue()); err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}
	if ok, err := st.Bind(naming.Join(storeRoot, "a/b")).Exists(ctx); !ok || err != nil {
		t.Errorf("Should exist: %s", err)
	}
}

func TestRelativeNamesNilTransaction(t *testing.T) {
	storeRoot, c := newServer(t) // calls rt.Init()
	defer c()
	ctx := rt.R().NewContext()
	st := vstore.New()

	root := st.Bind(storeRoot)
	if _, err := root.Put(ctx, newValue()); err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}
	a := root.Bind("a")
	if _, err := a.Put(ctx, newValue()); err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}
	b := a.Bind("b")
	if _, err := b.Put(ctx, newValue()); err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}
	if ok, err := st.Bind(naming.Join(storeRoot, "a/b")).Exists(ctx); !ok || err != nil {
		t.Errorf("Should exist: %s", err)
	}
}

func TestWatchGlob(t *testing.T) {
	storeRoot, c := newServer(t) // calls rt.Init()
	defer c()
	ctx := rt.R().NewContext()
	st := vstore.New()

	root := st.Bind(storeRoot)

	// Create the root.
	rootValue := "root-val"
	stat, err := root.Put(ctx, rootValue)
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}

	// Watch all objects under the root.
	req := types.GlobRequest{Pattern: "..."}
	stream, err := root.WatchGlob(ctx, req)
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}
	defer stream.Cancel()

	rStream := stream.RecvStream()
	// Expect a change adding /.
	changes := []types.Change{}
	if !rStream.Advance() {
		t.Error("Advance() failed: %v", rStream.Err())
	}
	change := rStream.Value()
	changes = append(changes, change)
	entry := findEntry(t, changes, "")
	if entry.Value != rootValue {
		t.Fatalf("Expected value to be %v, but was %v.", rootValue, entry.Value)
	}
	if entry.Stat.ID != stat.ID {
		t.Fatalf("Expected stat to be %v, but was %v.", stat, entry.Stat)
	}

	// Create /a.
	a := st.Bind(naming.Join(storeRoot, "a"))
	aValue := "a-val"
	stat, err = a.Put(ctx, aValue)
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}

	// Expect changes updating / and adding /a.
	changes = []types.Change{}
	if !rStream.Advance() {
		t.Error("Advance() failed: %v", rStream.Err())
	}
	change = rStream.Value()
	changes = append(changes, change)
	if !rStream.Advance() {
		t.Error("Advance() failed: %v", rStream.Err())
	}
	change = rStream.Value()
	changes = append(changes, change)
	findEntry(t, changes, "")
	entry = findEntry(t, changes, "a")
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
	t.Fatalf("Expected a change for name: %v", name)
	panic("Should not reach here")
}

type player struct {
	Name string
	Age  int
}

// TODO(sadovsky): A similar helper function exists in photoalbum_test.go.
// We should unify such helper functions and put them in a common location.
func put(t *testing.T, storeRoot string, ctx context.T, name string, v interface{}) storage.ID {
	_, file, line, _ := runtime.Caller(1)
	if ctx == nil {
		ctx = rt.R().NewContext()
	}
	stat, err := vstore.New().Bind(naming.Join(storeRoot, name)).Put(ctx, v)
	if err != nil || !stat.ID.IsValid() {
		t.Fatalf("%s(%d): can't put %s: %s", file, line, name, err)
	}
	return stat.ID
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
	storeRoot, c := newServer(t) // calls rt.Init()
	defer c()
	ctx := rt.R().NewContext()

	put(t, storeRoot, ctx, "", newValue())
	put(t, storeRoot, ctx, "players", newValue())
	put(t, storeRoot, ctx, "players/alfred", player{"alfred", 17})
	put(t, storeRoot, ctx, "players/alice", player{"alice", 16})
	put(t, storeRoot, ctx, "players/betty", player{"betty", 23})
	put(t, storeRoot, ctx, "players/bob", player{"bob", 21})

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

	o := vstore.New().Bind(storeRoot)
	for _, test := range tests {
		stream := o.Query(ctx, query.Query{test.query})
		expectResultNames(t, test.query, stream, test.resultNames)
	}
}

func TestQueryInTransaction(t *testing.T) {
	storeRoot, c := newServer(t) // calls rt.Init()
	defer c()
	ctx := rt.R().NewContext()
	st := vstore.New()

	put(t, storeRoot, ctx, "", newValue())
	put(t, storeRoot, ctx, "players", newValue())
	put(t, storeRoot, ctx, "players/alfred", player{"alfred", 17})
	put(t, storeRoot, ctx, "players/alice", player{"alice", 16})

	tx1 := st.NewTransaction(ctx, storeRoot)
	tobj1 := tx1.Bind("")

	const allPlayers = "players/* | type player | sort()"
	stream := tobj1.Query(ctx, query.Query{allPlayers})
	expectResultNames(t, allPlayers, stream, []string{"players/alfred", "players/alice"})

	// Query should see mutations that are part of the transaction.
	if _, err := tx1.Bind("players/betty").Put(ctx, player{"betty", 23}); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if _, err := tx1.Bind("players/bob").Put(ctx, player{"bob", 21}); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	stream = tobj1.Query(ctx, query.Query{allPlayers})
	expectResultNames(t, allPlayers, stream,
		[]string{"players/alfred", "players/alice", "players/betty", "players/bob"})

	// Query should not see mutations that are part of an uncommitted transaction.
	tx2 := st.NewTransaction(ctx, storeRoot)
	tobj2 := tx2.Bind("")
	stream = tobj2.Query(ctx, query.Query{allPlayers})
	expectResultNames(t, allPlayers, stream, []string{"players/alfred", "players/alice"})

	if err := tx1.Commit(ctx); err != nil {
		t.Errorf("Unexpected error")
	}

	// tobj2 should still not see the mutations from tobj1.
	stream = tobj2.Query(ctx, query.Query{allPlayers})
	expectResultNames(t, allPlayers, stream, []string{"players/alfred", "players/alice"})

	// A new transaction should see the mutations from tobj1.
	tx3 := st.NewTransaction(ctx, storeRoot)
	stream = tx3.Bind("").Query(ctx, query.Query{allPlayers})
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
	storeRoot, c := newServer(t) // calls rt.Init()
	defer c()
	ctx := rt.R().NewContext()

	const pattern = "players/*"

	put(t, storeRoot, ctx, "", newValue())
	put(t, storeRoot, ctx, "players", newValue())
	put(t, storeRoot, ctx, "players/alice", player{"alice", 16})
	put(t, storeRoot, ctx, "players/bob", player{"bob", 21})

	o := vstore.New().Bind(storeRoot)
	stream := o.Glob(ctx, pattern)
	expectValues(t, pattern, stream, []string{"players/alice", "players/bob"})
}

func TestGlobInTransaction(t *testing.T) {
	storeRoot, c := newServer(t) // calls rt.Init()
	defer c()
	ctx := rt.R().NewContext()
	st := vstore.New()

	const pattern = "players/*"

	put(t, storeRoot, ctx, "", newValue())
	put(t, storeRoot, ctx, "players", newValue())
	put(t, storeRoot, ctx, "players/alfred", player{"alfred", 17})
	put(t, storeRoot, ctx, "players/alice", player{"alice", 16})

	tx1 := st.NewTransaction(ctx, storeRoot)
	tobj1 := tx1.Bind("")

	stream := tobj1.Glob(ctx, pattern)
	expectValues(t, pattern, stream, []string{"players/alfred", "players/alice"})

	// Glob should see mutations that are part of the transaction.
	if _, err := tx1.Bind("players/betty").Put(ctx, player{"betty", 23}); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if _, err := tx1.Bind("players/bob").Put(ctx, player{"bob", 21}); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	stream = tobj1.Glob(ctx, pattern)
	expectValues(t, pattern, stream,
		[]string{"players/alfred", "players/alice", "players/betty", "players/bob"})

	// Glob should not see mutations that are part of an uncommitted transaction.
	tx2 := st.NewTransaction(ctx, storeRoot)
	tobj2 := tx2.Bind("")
	stream = tobj2.Glob(ctx, pattern)
	expectValues(t, pattern, stream, []string{"players/alfred", "players/alice"})

	if err := tx1.Commit(ctx); err != nil {
		t.Errorf("Unexpected error")
	}

	// tobj2 should still not see the mutations from tobj1.
	stream = tobj2.Glob(ctx, pattern)
	expectValues(t, pattern, stream, []string{"players/alfred", "players/alice"})

	// A new transaction should see the mutations from tobj2.
	tx3 := st.NewTransaction(ctx, storeRoot)
	stream = tx3.Bind("").Glob(ctx, pattern)
	expectValues(t, pattern, stream,
		[]string{"players/alfred", "players/alice", "players/betty", "players/bob"})
}
