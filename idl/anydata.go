package idl

import (
	"encoding/gob"

	"veyron2/vom"
)

// AnyData is a special type used by the veyron idl compiler.  The built-in
// "anydata" idl type gets translated into idl.AnyData when generating Go code.
// We define a special type rather than just using "interface{}" to make it easy
// to add special-case logic when necessary.
type AnyData interface{}

// RegisterType is like gob.Register() - it must be called to register the
// concrete types you'll be sending through the AnyData type.
func RegisterType(value interface{}) {
	gob.Register(value)
	vom.Register(value)
}

func init() {
	RegisterType((*AnyData)(nil))
}
