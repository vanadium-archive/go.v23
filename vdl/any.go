package vdl

import (
	"encoding/gob"

	"veyron2/vom"
)

// Any is a special type used by the veyron vdl compiler.  The built-in "any"
// vdl type gets translated into vdl.Any when generating Go code.  We define a
// special type rather than just using "interface{}" to make it easy to add
// special-case logic when necessary.
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
