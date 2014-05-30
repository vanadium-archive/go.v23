package compile

import (
	"veyron2/val"
)

var (
	// The BuiltInPackage and BuiltInFile are used to hold the built-ins.
	BuiltInPackage = newPackage("", "builtin")
	BuiltInFile    = &File{BaseName: "builtin.vdl"}
)

func init() {
	// Link the BuiltIn{Package,File} to each other before defining built-ins.
	BuiltInPackage.Files = []*File{BuiltInFile}
	BuiltInFile.Package = BuiltInPackage
	// Built-in types
	builtInType("any", val.AnyType)
	builtInType("bool", val.BoolType)
	builtInType("byte", val.ByteType)
	builtInType("uint16", val.Uint16Type)
	builtInType("uint32", val.Uint32Type)
	builtInType("uint64", val.Uint64Type)
	builtInType("int16", val.Int16Type)
	builtInType("int32", val.Int32Type)
	builtInType("int64", val.Int64Type)
	builtInType("float32", val.Float32Type)
	builtInType("float64", val.Float64Type)
	builtInType("complex64", val.Complex64Type)
	builtInType("complex128", val.Complex128Type)
	builtInType("string", val.StringType)
	builtInType("typeval", val.TypeValType)
	builtInType("error", ErrorType)
	// Built-in consts
	builtInConst("true", TrueConst)
	builtInConst("false", FalseConst)
}

func builtInType(name string, t *val.Type) {
	def := &TypeDef{
		NamePos:  NamePos{Name: name},
		Exported: true,
		Type:     t,
		File:     BuiltInFile,
	}
	addTypeDef(def, nil)
}

func builtInConst(name string, v *val.Value) {
	def := &ConstDef{
		NamePos:  NamePos{Name: name},
		Exported: true,
		Value:    v,
		File:     BuiltInFile,
	}
	addConstDef(def, nil)
}
