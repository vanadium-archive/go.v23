package javascript

import (
	"fmt"
	"testing"

	"v.io/core/veyron2/vdl"
	"v.io/core/veyron2/vdl/compile"
)

const unnamedTypeFieldName = "UnnamedTypeField"

func getTestTypes() (names typeNames, tyStruct, tyList, tyBool *vdl.Type, outErr error) {
	var builder vdl.TypeBuilder
	namedBool := builder.Named("otherPkg.NamedBool").AssignBase(vdl.BoolType)
	listType := builder.List()
	namedList := builder.Named("NamedList").AssignBase(listType)
	structType := builder.Struct()
	namedStruct := builder.Named("NamedStruct").AssignBase(structType)
	structType.AppendField("List", namedList)
	structType.AppendField("Bool", namedBool)
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
						Type: builtList,
					},
					{
						Type: builtStruct,
					},
					{
						Type: vdl.ListType(vdl.ByteType),
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

	expectedResult := `var _type1 = new vom.Type();
var _type2 = new vom.Type();
var _typeNamedList = new vom.Type();
var _typeNamedStruct = new vom.Type();
_type1.kind = vom.Kind.LIST;
_type1.name = "";
_type1.elem = vom.Types.STRING;
_type2.kind = vom.Kind.LIST;
_type2.name = "";
_type2.elem = vom.Types.BYTE;
_typeNamedList.kind = vom.Kind.LIST;
_typeNamedList.name = "NamedList";
_typeNamedList.elem = _typeNamedStruct;
_typeNamedStruct.kind = vom.Kind.STRUCT;
_typeNamedStruct.name = "NamedStruct";
_typeNamedStruct.fields = [{name: "List", type: _typeNamedList}, {name: "Bool", type: new otherPkg.NamedBool()._type}, {name: "UnnamedTypeField", type: _type1}];
module.exports.NamedList = (vom.Registry.lookupOrCreateConstructor(_typeNamedList));
module.exports.NamedStruct = (vom.Registry.lookupOrCreateConstructor(_typeNamedStruct));
`

	if result != expectedResult {
		t.Errorf("Expected %q, but got %q", expectedResult, result)
	}
}
