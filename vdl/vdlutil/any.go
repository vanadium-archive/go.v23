package vdlutil

// TODO(toddw): Move the contents of this file to the vdl package after the vom
// transition.  We can't just move it now since vom has too many bad
// dependencies that we don't want to pull in to the vdl package.

// Any represents a value of the Any type in generated Go code.  We define a
// special type rather than just using interface{} in generated code, to make it
// easy to identify and add special-casing later.
//
// TODO(toddw): Rename to AnyRep
type Any interface{}
