package vdlutil

import (
	"encoding/gob"

	"veyron.io/veyron/veyron2/vom"
)

// TODO(toddw): Move Any into veyron2/vdl as AnyRep, and remove this file
// completely, when vom2 is deployed.  We don't need RegisterType in vom2, but
// still need it for vom1 to work.

// Any represents a value of the Any type in generated Go code.  We define a
// special type rather than just using interface{} in generated code, to make it
// easy to identify and add special-casing later.
type Any interface{}

// RegisterType is like gob.Register() - it must be called to register the
// concrete types you'll be sending through the Any type.
func RegisterType(value interface{}) {
	gob.Register(value)
	vom.Register(value)
}

func init() {
	RegisterType((*Any)(nil))
}
