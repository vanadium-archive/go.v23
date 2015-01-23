package javascript

import (
	"testing"

	"v.io/core/veyron2/vdl"
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

	tests := []struct {
		name       string
		inputValue *vdl.Value
		expected   string
	}{
		{
			name:       "struct test",
			inputValue: structValue,
			expected: `new (Registry.lookupOrCreateConstructor(_typeNamedStruct))({
  'list': [
],
  'bool': false,
  'unnamedTypeField': [
"AStringVal",
],
})`,
		},
		{
			name:       "bytes test",
			inputValue: vdl.BytesValue([]byte{1, 2, 3, 4}),
			expected: `new (Registry.lookupOrCreateConstructor(_type2))(new Uint8Array([
1,
2,
3,
4,
]))`,
		},
	}
	for _, test := range tests {
		strVal := typedConst(names, test.inputValue)
		if strVal != test.expected {
			t.Errorf("In %q, expected %q, but got %q", test.name, test.expected, strVal)
		}
	}
}

// TODO(bjornick) Add more thorough tests.
