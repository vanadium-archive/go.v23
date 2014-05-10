package blackbox

import (
	"runtime"
	"testing"

	"veyron2/storage"
	"veyron2/storage/vstore"
	"veyron2/vom"
)

func init() {
	vom.Register(&Dir{})
	vom.Register(&User{})
	vom.Register(&Photo{})
	vom.Register(&Album{})
}

// Store:
//
//
//                        / : Dir
//                          |
//                          |
//                     User : Dir
//                          |
//                          |
//                      jyh : User
//                      /        \
//                     /          \
//              ByDate : Dir     Albums : Dir
//                   |              \
//                   |               \
//            2014_01_01 : Dir   Yosemite : Album
//                   .                /     \
//                  ... (all 5)      /       \  (2 Photos)
//                 .....            /         \
//         DSC1000, DSC1001, DSC1002, DSC1003, DSC1004 : Photo

// Dir is a "directory" containg a dictionaries of entries.
type Dir struct{}

// User represents a "user", with a username and a "home" directory.
// The name of the user is part of the path to the object.
type User struct {
	Dir
	SSN int
}

// Photo represents an image.  It contains the Veyron name for the data,
// stored elsewhere on some content server.
type Photo struct {
	Dir
	Comment string
	Content string // Veyron name
	Edits   []Edit
}

// Edit is an edit to a Photo.
type Edit struct {
	// ...
}

// Album is a photoalbum.
type Album struct {
	Title  string
	Photos map[string]storage.ID
}

func newDir() *Dir {
	return &Dir{}
}

func newUser(ssn int) *User {
	return &User{SSN: ssn}
}

func newAlbum(title string) *Album {
	return &Album{Title: title}
}

func newPhoto(content, comment string, edits ...Edit) *Photo {
	return &Photo{Content: content, Comment: comment}
}

func get(t *testing.T, st storage.Store, tr storage.Transaction, path string) *storage.Entry {
	_, file, line, _ := runtime.Caller(1)
	entry, err := st.Bind(path).Get(tr)
	if err != nil {
		t.Fatalf("%s(%d): can't get %s: %s", file, line, path, err)
	}
	return &entry
}

func getPhoto(t *testing.T, st storage.Store, tr storage.Transaction, path string) *Photo {
	_, file, line, _ := runtime.Caller(1)
	e := get(t, st, tr, path)
	v := e.Value
	p, ok := v.(*Photo)
	if !ok {
		t.Fatalf("%s(%d): %s: not a Photo: %v", file, line, path, v)
	}
	return p
}

func put(t *testing.T, st storage.Store, tr storage.Transaction, path string, v interface{}) storage.ID {
	stat, err := st.Bind(path).Put(tr, v)
	if err != nil {
		_, file, line, _ := runtime.Caller(1)
		t.Errorf("%s(%d): can't put %s: %s", file, line, path, err)
	}
	if stat.ID.IsValid() {
		return stat.ID
	}
	if id, ok := v.(storage.ID); ok {
		return id
	}
	return storage.ID{}
}

func commit(t *testing.T, tr storage.Transaction) {
	if err := tr.Commit(); err != nil {
		t.Fatalf("Transaction aborted: %s", err)
	}
}

func TestPhotoAlbum(t *testing.T) {
	st, c := startServer(t) // calls rt.Init()
	defer c()

	// Create directories.
	{
		tr := vstore.NewTransaction()
		put(t, st, tr, "/", newDir())
		put(t, st, tr, "/Users", newDir())
		put(t, st, tr, "/Users/jyh", newUser(1234567890))
		put(t, st, tr, "/Users/jyh/ByDate", newDir())
		put(t, st, tr, "/Users/jyh/ByDate/2014_01_01", newDir())
		put(t, st, tr, "/Users/jyh/Albums", newDir())
		commit(t, tr)
	}

	// Add some photos by date.
	{
		p1 := newPhoto("/global/contentd/DSC1000.jpg", "Half Dome")
		p2 := newPhoto("/global/contentd/DSC1001.jpg", "I don't want to hike")
		p3 := newPhoto("/global/contentd/DSC1002.jpg", "Crying kids")
		p4 := newPhoto("/global/contentd/DSC1003.jpg", "Ice cream")
		p5 := newPhoto("/global/contentd/DSC1004.jpg", "Let's go home")

		tr := vstore.NewTransaction()
		put(t, st, tr, "/Users/jyh/ByDate/2014_01_01/09:00", p1)
		put(t, st, tr, "/Users/jyh/ByDate/2014_01_01/09:15", p2)
		put(t, st, tr, "/Users/jyh/ByDate/2014_01_01/09:16", p3)
		put(t, st, tr, "/Users/jyh/ByDate/2014_01_01/10:00", p4)
		put(t, st, tr, "/Users/jyh/ByDate/2014_01_01/10:05", p5)
		commit(t, tr)
	}

	// Add an Album with some of the photos.
	{
		tr := vstore.NewTransaction()
		put(t, st, tr, "/Users/jyh/Albums/Yosemite", newAlbum("Yosemite selected photos"))
		p5 := get(t, st, tr, "/Users/jyh/ByDate/2014_01_01/10:05")
		put(t, st, tr, "/Users/jyh/Albums/Yosemite/Photos/1", p5.Stat.ID)
		p3 := get(t, st, tr, "/Users/jyh/ByDate/2014_01_01/09:16")
		put(t, st, tr, "/Users/jyh/Albums/Yosemite/Photos/2", p3.Stat.ID)
		commit(t, tr)
	}

	// Verify some of the photos.
	{
		tr := vstore.NewTransaction()
		p1 := getPhoto(t, st, tr, "/Users/jyh/ByDate/2014_01_01/09:00")
		if p1.Comment != "Half Dome" {
			t.Errorf("Expected %q, got %q", "Half Dome", p1.Comment)
		}
	}

	{
		tr := vstore.NewTransaction()
		p3 := getPhoto(t, st, tr, "/Users/jyh/Albums/Yosemite/Photos/2")
		if p3.Comment != "Crying kids" {
			t.Errorf("Expected %q, got %q", "Crying kids", p3.Comment)
		}
	}

	// Update p3.Comment to "Happy".
	{
		tr := vstore.NewTransaction()
		p3 := getPhoto(t, st, tr, "/Users/jyh/ByDate/2014_01_01/09:16")
		p3.Comment = "Happy"
		put(t, st, tr, "/Users/jyh/ByDate/2014_01_01/09:16", p3)
		commit(t, tr)
	}

	// Verify that the photo in the album has also changed.
	{
		tr := vstore.NewTransaction()
		p3 := getPhoto(t, st, tr, "/Users/jyh/Albums/Yosemite/Photos/2")
		if p3.Comment != "Happy" {
			t.Errorf("Expected %q, got %q", "Happy", p3.Comment)
		}
	}
}
