// Package vstore implements a client interface to a Veyron store.
// The API is defined in veyron2/storage.
package vstore

import (
	"veyron2/storage"
)

// BindDir binds to a Dir in the Store.  The given name must either
// be of Dir type or not exist (i.e. it can't be an Object).  BindDir
// is intended to be called early in the lifetime of the application
// and the resulting Dir can be passed around.  Tests can pass around
// mock Dirs and the application code should remain unchanged.
func BindDir(name string) storage.Dir {
	return newDir(name)
}
