package javascript

import (
	"testing"

	"v.io/veyron/veyron2/vdl"
)

func TestTypedConst(t *testing.T) {
	names, structType, _, _, err := getTestTypes()
	if err != nil {
		t.Fatalf("Error in getTestTypes(): %v", err)
	}
	structValue := vdl.ZeroValue(structType)
	_, index := structValue.Type().FieldByName(unnamedTypeFieldName)
	structValue.Field(index).AssignLen(1)
	structValue.Field(index).Index(0).AssignString("AStringVal")

	strVal := typedConst(names, structValue)
	expected := `new (Registry.lookupOrCreateConstructor(_typeNamedStruct))({
  'list': [
],
  'unnamedTypeField': [
"AStringVal",
],
})`
	if strVal != expected {
		t.Errorf("Expected %q, but got %q", expected, strVal)
	}
}

// TODO(bjornick) Add more thorough tests.
