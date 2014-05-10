package vom

import (
	"testing"
)

func TestStructSet(t *testing.T) {
	sm := newStructSet()

	sfa1 := &vStructType{fields: []StructField{StructField{Name: "A", Type: TypeOf(4)}}}
	sfa2 := &vStructType{fields: []StructField{StructField{Name: "A", Type: TypeOf(5)}}}
	sfb := &vStructType{fields: []StructField{StructField{Name: "C", Type: TypeOf(5)}}}
	sfc1 := &vStructType{fields: []StructField{sfa1.fields[0], StructField{Name: "D", Type: TypeOf("")}}}
	sfc2 := &vStructType{fields: []StructField{sfa1.fields[0], StructField{Name: "D", Type: TypeOf("")}}}
	sfd := &vStructType{fields: []StructField{StructField{Name: "F", Type: TypeOf("")}, sfa1.fields[0]}}

	empty := &vStructType{fields: []StructField{}}

	if sm.HashCons(sfa1) != sfa1 {
		t.Fatalf("should get same object on initial add")
	}
	if sm.HashCons(sfa2) != sfa1 {
		t.Fatalf("should get initial object when looking up equivilent")
	}
	if sm.HashCons(sfb) != sfb {
		t.Fatalf("should get second object")
	}
	if sm.HashCons(sfa1) != sfa1 || sm.HashCons(sfa1) == sfb {
		t.Fatalf("insert of second object shouldn't have messed up the store of the first")
	}
	if sm.HashCons(sfc1) != sfc1 {
		t.Fatalf("insert of two element struct failed")
	}
	if sm.HashCons(sfc2) != sfc1 {
		t.Fatalf("look up of equivilent two element struct should have succeeded")
	}
	if sm.HashCons(sfd) != sfd {
		// if this fails, then we may be returning the matched sfa
		t.Fatalf("ensure recursive procedure is isolated by initial size")
	}
	if sm.HashCons(empty) != empty {
		t.Fatalf("empty structs not hash consed properly")
	}
}
