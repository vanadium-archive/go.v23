package build

import (
	"testing"

	"veyron2/wiretype"
)

// Tests that adding new wiretypes and looking up existing wiretypes with Put() works correctly.
func TestTypeDefsPut(t *testing.T) {
	td := TypeDefs{}

	td, id1 := td.Put(wiretype.SliceType{wiretype.TypeIDFloat32, "float32", nil})
	td, id2 := td.Put(wiretype.SliceType{wiretype.TypeIDFloat64, "decimal", nil})
	td, id3 := td.Put(wiretype.SliceType{id2, "decimalSlice", nil})

	td, id2a := td.Put(wiretype.SliceType{wiretype.TypeIDFloat64, "decimal", nil})

	if len(td) != 3 {
		t.Fatal("Expected 3 elements: ", td)
	}

	if id1 != wiretype.TypeIDFirst || id2 != wiretype.TypeIDFirst+1 || id3 != wiretype.TypeIDFirst+2 {
		t.Fatalf("Unexpected id: %v %v %v", id1, id2, id3)
	}

	if id2 != id2a {
		t.Fatalf("Expect ids for same types to match")
	}
}
