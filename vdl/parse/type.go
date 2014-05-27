package parse

import (
	"fmt"
)

// Type is an interface representing symbolic occurrences of types in VDL files.
type Type interface {
	// String returns a human-readable description of the type.
	String() string
	// Kind returns a short human-readable string describing the kind of type.
	Kind() string
	// Pos returns the position of the first character in the type.
	Pos() Pos
}

// TypeNamed captures named references to other types.  Both built-in primitives
// and user-defined named types use this representation.
type TypeNamed struct {
	Name string
	P    Pos
}

// TypeEnum represents enum types.
type TypeEnum struct {
	Labels []NamePos
	P      Pos
}

// TypeArray represents array types.
type TypeArray struct {
	Len  int
	Elem Type
	P    Pos
}

// TypeList represents list types.
type TypeList struct {
	Elem Type
	P    Pos
}

// TypeSet represents set types.
type TypeSet struct {
	Key Type
	P   Pos
}

// TypeMap represents map types.
type TypeMap struct {
	Key  Type
	Elem Type
	P    Pos
}

// TypeStruct represents struct types.
type TypeStruct struct {
	Fields []*Field
	P      Pos
}

// TypeOneOf represents oneof types.
type TypeOneOf struct {
	Types []Type
	P     Pos
}

// TypeDef represents a user-defined named type.
type TypeDef struct {
	NamePos      // name assigned by the user, pos and doc
	Type    Type // the underlying type of the type definition.
}

func (t *TypeNamed) Pos() Pos  { return t.P }
func (t *TypeEnum) Pos() Pos   { return t.P }
func (t *TypeArray) Pos() Pos  { return t.P }
func (t *TypeList) Pos() Pos   { return t.P }
func (t *TypeSet) Pos() Pos    { return t.P }
func (t *TypeMap) Pos() Pos    { return t.P }
func (t *TypeStruct) Pos() Pos { return t.P }
func (t *TypeOneOf) Pos() Pos  { return t.P }

func (t *TypeNamed) Kind() string  { return "named" }
func (t *TypeEnum) Kind() string   { return "enum" }
func (t *TypeArray) Kind() string  { return "array" }
func (t *TypeList) Kind() string   { return "list" }
func (t *TypeSet) Kind() string    { return "set" }
func (t *TypeMap) Kind() string    { return "map" }
func (t *TypeStruct) Kind() string { return "struct" }
func (t *TypeOneOf) Kind() string  { return "oneof" }

func (t *TypeNamed) String() string { return t.Name }
func (t *TypeEnum) String() string {
	result := "enum{"
	for index, label := range t.Labels {
		if index > 0 {
			result += ";"
		}
		result += label.Name
	}
	return result + "}"
}
func (t *TypeArray) String() string { return fmt.Sprintf("[%v]%v", t.Len, t.Elem) }
func (t *TypeList) String() string  { return fmt.Sprintf("[]%v", t.Elem) }
func (t *TypeSet) String() string   { return fmt.Sprintf("set[%v]", t.Key) }
func (t *TypeMap) String() string   { return fmt.Sprintf("map[%v]%v", t.Key, t.Elem) }
func (t *TypeStruct) String() string {
	result := "struct{"
	for index, field := range t.Fields {
		if index > 0 {
			result += ";"
		}
		result += field.Name + " " + field.Type.String()
	}
	return result + "}"
}
func (t *TypeOneOf) String() string {
	result := "oneof{"
	for index, t := range t.Types {
		if index > 0 {
			result += ";"
		}
		result += t.String()
	}
	return result + "}"
}

func (t *TypeDef) String() string {
	return fmt.Sprintf("(%v %v %v)", t.Pos, t.Name, t.Type)
}
