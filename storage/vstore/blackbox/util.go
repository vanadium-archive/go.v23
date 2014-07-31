package blackbox

import (
	"crypto/rand"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"testing"

	// TODO(cnicolaou): should probably remove this dependency on an external
	// implementation.
	"veyron/services/store/server"

	"veyron2"
	"veyron2/ipc"
	"veyron2/naming"
	"veyron2/rt"
	"veyron2/security"
	"veyron2/storage"
	"veyron2/storage/vstore"
)

var storeID = security.FakePrivateID("store")

// startServer opens a server, then creates and returns a client.  Also returns
// a function to close everything at the end of the test.
func startServer(t *testing.T) (storage.Store, func()) {
	r := rt.Init(veyron2.RuntimeID(storeID))

	var buf [16]byte
	if _, err := rand.Read(buf[:]); err != nil {
		log.Fatal("rand.Read() failed: ", err)
	}
	mount := fmt.Sprintf("test/%x", buf)
	tempDir, err := ioutil.TempDir("", "vstore")
	if err != nil {
		log.Fatal("ioutil.TempDir(vstorage. failed: ", err)
	}
	dbName, err := ioutil.TempDir(tempDir, "test.db")
	if err != nil {
		log.Fatal("ioutil.TempDir(test.db) failed: ", err)
	}
	t.Logf("Mount: %s", mount)

	// Create a new server instance.
	s, err := r.NewServer()
	if err != nil {
		log.Fatal("r.NewServer() failed: ", err)
	}

	// Create a new StoreService.
	storeService, err := server.New(server.ServerConfig{Admin: storeID.PublicID(), DBName: dbName})
	if err != nil {
		log.Fatal("server.New() failed: ", err)
	}

	// Register the services.
	storeDisp := server.NewStoreDispatcher(storeService, nil)
	if err := s.Serve(mount, storeDisp); err != nil {
		log.Fatal("s.Register(storeDisp) failed: ", err)
	}

	// Create an endpoint and start listening.
	ep, err := s.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		log.Fatal("s.Listen() failed: ", err)
	}

	// We're running without a MountTable so we use the endpoint as the name.
	name := naming.JoinAddressName(ep.String(), "")
	st, err := vstore.New(name)
	if err != nil {
		log.Fatal("vstorage.New() failed: ", err)
	}

	cl := func() { closeServer(t, s, dbName, st) }
	return st, cl
}

func closeServer(t *testing.T, s ipc.Server, dbName string, st storage.Store) {
	s.Stop()
	os.Remove(dbName)
}
