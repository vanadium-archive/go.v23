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
	"veyron2/services/watch"
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

func newServer(t *testing.T) (storage.Store, func()) {
	id := veyron2.RuntimeID(ipcID)
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

	testPutGetRemove(t, s, "/")
}

func TestPutGetRemoveChild(t *testing.T) {
	s, c := newServer(t) // calls rt.Init()
	defer c()
	ctx := rt.R().NewContext()

	{
		// Create a root.
		name := "/"
		value := newValue()

		tname1 := createTransaction(t, s, ctx, name)
		tobj1 := s.BindObject(tname1)
		if _, err := tobj1.Put(ctx, value); err != nil {
			t.Errorf("Unexpected error: %s", err)
		}
		if err := s.BindTransaction(tname1).Commit(ctx); err != nil {
			t.Errorf("Unexpected error: %s", err)
		}

		tname2 := createTransaction(t, s, ctx, name)
		tobj2 := s.BindObject(tname2)
		if ok, err := tobj2.Exists(ctx); !ok || err != nil {
			t.Errorf("Should exist: %s", err)
		}
		if _, err := tobj2.Get(ctx); err != nil {
			t.Errorf("Object should exist: %s", err)
		}
		if err := s.BindTransaction(tname2).Abort(ctx); err != nil {
			t.Errorf("Unexpected error: %s", err)
		}
	}

	testPutGetRemove(t, s, "/Entries/a")
}

func testPutGetRemove(t *testing.T, s storage.Store, name string) {
	value := newValue()
	ctx := rt.R().NewContext()
	{
		// Check that the object does not exist.
		tname := createTransaction(t, s, ctx, name)
		tobj := s.BindObject(tname)
		if ok, err := tobj.Exists(ctx); ok || err != nil {
			t.Errorf("Should not exist: %s", err)
		}
		if v, err := tobj.Get(ctx); !v.Stat.ID.IsValid() && err == nil {
			t.Errorf("Should not exist: %v, %s", v, err)
		}
	}

	{
		// Add the object.
		tname1 := createTransaction(t, s, ctx, name)
		tobj1 := s.BindObject(tname1)
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
		tname2 := createTransaction(t, s, ctx, name)
		tobj2 := s.BindObject(tname2)
		if ok, err := tobj2.Exists(ctx); ok || err != nil {
			t.Errorf("Should not exist: %s", err)
		}
		if v, err := tobj2.Get(ctx); v.Stat.ID.IsValid() && err == nil {
			t.Errorf("Should not exist: %v, %s", v, err)
		}

		// Apply tobj1.
		if err := s.BindTransaction(tname1).Commit(ctx); err != nil {
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
		tname3 := createTransaction(t, s, ctx, name)
		tobj3 := s.BindObject(tname3)
		if ok, err := tobj3.Exists(ctx); !ok || err != nil {
			t.Errorf("Should exist: %s", err)
		}
		if _, err := tobj3.Get(ctx); err != nil {
			t.Errorf("Object should exist: %s", err)
		}
	}

	{
		// Remove the object.
		tname1 := createTransaction(t, s, ctx, name)
		tobj1 := s.BindObject(tname1)
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
		tname2 := createTransaction(t, s, ctx, name)
		tobj2 := s.BindObject(tname2)
		if ok, err := tobj2.Exists(ctx); !ok || err != nil {
			t.Errorf("Should exist: %s", err)
		}
		if _, err := tobj2.Get(ctx); err != nil {
			t.Errorf("Object should exist: %s", err)
		}

		// Apply tobj1.
		if err := s.BindTransaction(tname1).Commit(ctx); err != nil {
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
		tname1 := createTransaction(t, s, ctx, name)
		tobj1 := s.BindObject(tname1)
		if ok, err := tobj1.Exists(ctx); ok || err != nil {
			t.Errorf("Should not exist")
		}
		if v, err := tobj1.Get(ctx); v.Stat.ID.IsValid() && err == nil {
			t.Errorf("Should not exist: %v, %s", v, err)
		}
	}
}

func TestPutGetRemoveNilTransaction(t *testing.T) {
	s, c := newServer(t) // calls rt.Init()
	defer c()
	ctx := rt.R().NewContext()

	{
		// Create a root.
		o := s.BindObject("/")
		value := newValue()
		if _, err := o.Put(ctx, value); err != nil {
			t.Errorf("Unexpected error: %s", err)
		}
		if ok, err := o.Exists(ctx); !ok || err != nil {
			t.Errorf("Should exist: %s", err)
		}
		if _, err := o.Get(ctx); err != nil {
			t.Errorf("Object should exist: %s", err)
		}
	}

	name := "/Entries/b"
	value := newValue()
	o := s.BindObject(name)
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
		tname := createTransaction(t, s, ctx, name)
		tobj := s.BindObject(tname)
		if ok, err := tobj.Exists(ctx); ok || err != nil {
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
		if ok, err := tobj.Exists(ctx); ok || err != nil {
			t.Errorf("Should not exist: %s", err)
		}
		if v, err := tobj.Get(ctx); v.Stat.ID.IsValid() && err == nil {
			t.Errorf("Should not exist: %v, %s", v, err)
		}
		if err := s.BindTransaction(tname).Abort(ctx); err != nil {
			t.Errorf("Unexpected error: %s", err)
		}
	}

	{
		tname := createTransaction(t, s, ctx, name)
		tobj := s.BindObject(tname)
		if ok, err := tobj.Exists(ctx); !ok || err != nil {
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
		if ok, err := tobj.Exists(ctx); !ok || err != nil {
			t.Errorf("Should exist: %s", err)
		}
		if _, err := tobj.Get(ctx); err != nil {
			t.Errorf("Object should exist: %s", err)
		}
		if err := s.BindTransaction(tname).Abort(ctx); err != nil {
			t.Errorf("Unexpected error: %s", err)
		}
	}
}

func TestWatchGlob(t *testing.T) {
	s, c := newServer(t) // calls rt.Init()
	defer c()
	ctx := rt.R().NewContext()

	root := s.BindObject("/")

	// Create the root.
	rootValue := "root-val"
	stat, err := root.Put(ctx, rootValue)
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
	if !stream.Advance() {
		t.Fatalf("Unexpected error: %s", stream.Err())
	}
	cb := stream.Value()
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
	a := s.BindObject("/a")
	aValue := "a-val"
	stat, err = a.Put(ctx, aValue)
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}

	// Expect changes updating / and adding /a.
	if !stream.Advance() {
		t.Fatalf("Unexpected error: %s", stream.Err())
	}
	cb = stream.Value()
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

type player struct {
	Name string
	Age  int
}

// TODO(sadovsky): Similar helper functions exist in photoalbum_test.go,
// examples/todos/test/store_util.go, and elsewhere. We should unify them and
// put them in a common location.

func createTransaction(t *testing.T, st storage.Store, ctx context.T, name string) string {
	_, file, line, _ := runtime.Caller(1)
	tid, err := st.BindTransactionRoot(name).CreateTransaction(ctx)
	if err != nil {
		t.Fatalf("%s(%d): can't create transaction %s: %s", file, line, name, err)
	}
	return naming.Join(name, tid)
}

func put(t *testing.T, st storage.Store, ctx context.T, name string, v interface{}) storage.ID {
	_, file, line, _ := runtime.Caller(1)
	if ctx == nil {
		ctx = rt.R().NewContext()
	}
	stat, err := st.BindObject(name).Put(ctx, v)
	if err != nil || !stat.ID.IsValid() {
		t.Fatalf("%s(%d): can't put %s: %s", file, line, name, err)
	}
	return stat.ID
}

func commit(t *testing.T, st storage.Store, ctx context.T, name string) {
	_, file, line, _ := runtime.Caller(1)
	if ctx == nil {
		ctx = rt.R().NewContext()
	}
	if err := st.BindTransaction(name).Commit(ctx); err != nil {
		t.Fatalf("%s(%d): commit failed: %s", file, line, err)
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
	s, c := newServer(t) // calls rt.Init()
	defer c()
	ctx := rt.R().NewContext()

	put(t, s, ctx, "", newValue())
	put(t, s, ctx, "players", newValue())
	put(t, s, ctx, "players/alfred", player{"alfred", 17})
	put(t, s, ctx, "players/alice", player{"alice", 16})
	put(t, s, ctx, "players/betty", player{"betty", 23})
	put(t, s, ctx, "players/bob", player{"bob", 21})

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

	o := s.BindObject("/")
	for _, test := range tests {
		stream := o.Query(ctx, query.Query{test.query})
		expectResultNames(t, test.query, stream, test.resultNames)
	}
}

func TestQueryInTransaction(t *testing.T) {
	s, c := newServer(t) // calls rt.Init()
	defer c()
	ctx := rt.R().NewContext()

	tname1 := createTransaction(t, s, ctx, "")
	put(t, s, ctx, tname1, newValue())
	put(t, s, ctx, naming.Join(tname1, "players"), newValue())
	put(t, s, ctx, naming.Join(tname1, "players/alfred"), player{"alfred", 17})
	put(t, s, ctx, naming.Join(tname1, "players/alice"), player{"alice", 16})
	commit(t, s, ctx, tname1)

	tname2 := createTransaction(t, s, ctx, "")
	tobj2 := s.BindObject(tname2)

	const allPlayers = "players/* | type player | sort()"
	stream := tobj2.Query(ctx, query.Query{allPlayers})
	expectResultNames(t, allPlayers, stream, []string{"players/alfred", "players/alice"})

	// Query should see mutations that are part of the transaction.
	put(t, s, ctx, naming.Join(tname2, "players/betty"), player{"betty", 23})
	put(t, s, ctx, naming.Join(tname2, "players/bob"), player{"bob", 21})
	stream = tobj2.Query(ctx, query.Query{allPlayers})
	expectResultNames(t, allPlayers, stream,
		[]string{"players/alfred", "players/alice", "players/betty", "players/bob"})

	// Query should not see mutations that are part of an uncommitted transaction.
	tname3 := createTransaction(t, s, ctx, "")
	tobj3 := s.BindObject(tname3)
	stream = tobj3.Query(ctx, query.Query{allPlayers})
	expectResultNames(t, allPlayers, stream, []string{"players/alfred", "players/alice"})

	commit(t, s, ctx, tname2)

	// tobj3 should still not see the mutations from tobj2.
	stream = tobj3.Query(ctx, query.Query{allPlayers})
	expectResultNames(t, allPlayers, stream, []string{"players/alfred", "players/alice"})

	// A new transaction should see the mutations from tobj2.
	tname4 := createTransaction(t, s, ctx, "")
	stream = s.BindObject(tname4).Query(ctx, query.Query{allPlayers})
	expectResultNames(t, allPlayers, stream,
		[]string{"players/alfred", "players/alice", "players/betty", "players/bob"})
}

func expectValues(t *testing.T, pattern string, stream storage.GlobStream, expectedValues []string) {
	_, file, line, _ := runtime.Caller(1)
	var values []string
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
	s, c := newServer(t) // calls rt.Init()
	defer c()
	ctx := rt.R().NewContext()

	const pattern = "players/*"

	put(t, s, ctx, "", newValue())
	put(t, s, ctx, "players", newValue())
	put(t, s, ctx, "players/alice", player{"alice", 16})
	put(t, s, ctx, "players/bob", player{"bob", 21})

	o := s.BindObject("/")
	stream := o.Glob(ctx, pattern)
	expectValues(t, pattern, stream, []string{"players/alice", "players/bob"})
}

func TestGlobInTransaction(t *testing.T) {
	s, c := newServer(t) // calls rt.Init()
	defer c()
	ctx := rt.R().NewContext()

	const pattern = "players/*"

	tname1 := createTransaction(t, s, ctx, "")
	put(t, s, ctx, tname1, newValue())
	put(t, s, ctx, naming.Join(tname1, "players"), newValue())
	put(t, s, ctx, naming.Join(tname1, "players/alfred"), player{"alfred", 17})
	put(t, s, ctx, naming.Join(tname1, "players/alice"), player{"alice", 16})
	commit(t, s, ctx, tname1)

	tname2 := createTransaction(t, s, ctx, "")
	tobj2 := s.BindObject(tname2)

	stream := tobj2.Glob(ctx, pattern)
	expectValues(t, pattern, stream, []string{"players/alfred", "players/alice"})

	// Glob should see mutations that are part of the transaction.
	put(t, s, ctx, naming.Join(tname2, "players/betty"), player{"betty", 23})
	put(t, s, ctx, naming.Join(tname2, "players/bob"), player{"bob", 21})
	stream = tobj2.Glob(ctx, pattern)
	expectValues(t, pattern, stream,
		[]string{"players/alfred", "players/alice", "players/betty", "players/bob"})

	// Glob should not see mutations that are part of an uncommitted transaction.
	tname3 := createTransaction(t, s, ctx, "")
	tobj3 := s.BindObject(tname3)
	stream = tobj3.Glob(ctx, pattern)
	expectValues(t, pattern, stream, []string{"players/alfred", "players/alice"})

	commit(t, s, ctx, tname2)

	// tobj3 should still not see the mutations from tobj2.
	stream = tobj3.Glob(ctx, pattern)
	expectValues(t, pattern, stream, []string{"players/alfred", "players/alice"})

	// A new transaction should see the mutations from tobj2.
	tname4 := createTransaction(t, s, ctx, "")
	stream = s.BindObject(tname4).Glob(ctx, pattern)
	expectValues(t, pattern, stream,
		[]string{"players/alfred", "players/alice", "players/betty", "players/bob"})
}
