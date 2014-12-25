package javascript

import (
	"fmt"
	"testing"

	"v.io/veyron/veyron2/vdl"
	"v.io/veyron/veyron2/vdl/compile"
)

const unnamedTypeFieldName = "UnnamedTypeField"

func getTestTypes() (names typeNames, tyStruct, tyList, tyBool *vdl.Type, outErr error) {
	var builder vdl.TypeBuilder
	namedBool := builder.Named("NamedBool").AssignBase(vdl.BoolType)
	listType := builder.List()
	namedList := builder.Named("NamedList").AssignBase(listType)
	structType := builder.Struct()
	namedStruct := builder.Named("NamedStruct").AssignBase(structType)
	structType.AppendField("List", namedList)
	structType.AppendField(unnamedTypeFieldName, builder.List().AssignElem(vdl.StringType))
	listType.AssignElem(namedStruct)
	if builder.Build() != true {
		outErr = fmt.Errorf("Failed to build test types")
		return
	}

	builtBool, err := namedBool.Built()
	if err != nil {
		outErr = fmt.Errorf("Error creating NamedBool: %v", err)
		return
	}

	builtList, err := namedList.Built()
	if err != nil {
		outErr = fmt.Errorf("Error creating NamedList %v", err)
		return
	}

	builtStruct, err := namedStruct.Built()
	if err != nil {
		outErr = fmt.Errorf("Error creating NamedStruct: %v", err)
		return
	}

	pkg := &compile.Package{
		Files: []*compile.File{
			&compile.File{
				TypeDefs: []*compile.TypeDef{
					{
						Type: builtBool,
					},
					{
						Type: builtList,
					},
					{
						Type: builtStruct,
					},
				},
			},
		},
	}

	return newTypeNames(pkg), builtStruct, builtList, builtBool, nil
}

// TestType tests that the output string of generated types is what we expect.
func TestType(t *testing.T) {
	jsnames, _, _, _, err := getTestTypes()
	if err != nil {
		t.Fatalf("Error in getTestTypes(): %v", err)
	}
	result := makeTypeDefinitionsString(jsnames)

	expectedResult := `var _type1 = new Type();
var _typeNamedBool = new Type();
var _typeNamedList = new Type();
var _typeNamedStruct = new Type();
_type1.kind = Kind.LIST;
_type1.name = "";
_type1.elem = Types.STRING;
_typeNamedBool.kind = Kind.BOOL;
_typeNamedBool.name = "NamedBool";
_typeNamedList.kind = Kind.LIST;
_typeNamedList.name = "NamedList";
_typeNamedList.elem = _typeNamedStruct;
_typeNamedStruct.kind = Kind.STRUCT;
_typeNamedStruct.name = "NamedStruct";
_typeNamedStruct.fields = [{name: "List", type: _typeNamedList}, {name: "UnnamedTypeField", type: _type1}];
types.NamedBool = Registry.lookupOrCreateConstructor(_typeNamedBool, "NamedBool");
types.NamedList = Registry.lookupOrCreateConstructor(_typeNamedList, "NamedList");
types.NamedStruct = Registry.lookupOrCreateConstructor(_typeNamedStruct, "NamedStruct");
`

	if result != expectedResult {
		t.Errorf("Expected %q, but got %q", expectedResult, result)
	}
}
