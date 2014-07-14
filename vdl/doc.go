// Package vdl provides generic representations of types and values in veyron.
//
// There are differences between veyron type / value semantics and regular Go.
// There is no notion of pointers in the veyron type system.  Go slices are
// called val lists.  There is also no concept of nil list or map values; these
// values start out empty, and you cannot distinguish non-existence from
// emptiness.  There is a special type called "typeval" that allows a type to be
// used as a value; types are first-class in veyron.  You can express
// optionality through values of the union and any types, which may be zero
// (non-existent), or nonzero (existent).
//
// Every value has an associated zero value.  The semantics are similar to Go,
// but take the above into account.  Scalars are the same as Go: false for
// bools, 0 for numerics and "" for strings.  The zero value for typevals is an
// unnamed Any type.  The zero value for enums is the label at index 0.  The
// zero value for list and map types is the empty value; there's no need (or
// way) to "make" one.  The zero value for struct types is a struct with all
// fields set recursively to their zero values.  The zero value for union and
// any types is nil, and represents non-existence.
package vdl
