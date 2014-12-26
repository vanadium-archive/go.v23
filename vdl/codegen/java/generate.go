// Package java implements Java code generation from compiled VDL packages.
package java

import (
	"v.io/core/veyron2/vdl"
	"v.io/core/veyron2/vdl/compile"
	"v.io/core/veyron2/vdl/vdlroot/src/vdltool"
)

// pkgPathXlator is the function used to translate a VDL package path
// into a Java package path.  If nil, no translation takes place.
var pkgPathXlator func(path string) string

// SetPkgPathXlator sets the function used to translate a VDL package
// path into a Java package path.
func SetPkgPathXlator(xlator func(path string) string) {
	pkgPathXlator = xlator
}

// javaGenPkgPath returns the Java package path given the VDL package path.
func javaGenPkgPath(vdlPkgPath string) string {
	if pkgPathXlator == nil {
		return vdlPkgPath
	}
	return pkgPathXlator(vdlPkgPath)
}

// JavaFileInfo stores the name and contents of the generated Java file.
type JavaFileInfo struct {
	Dir  string
	Name string
	Data []byte
}

// Generate generates Java files for all VDL files in the provided package,
// returning the list of generated Java files as a slice.  Since Java requires
// that each public class/interface gets defined in a separate file, this method
// will return one generated file per struct.  (Interfaces actually generate
// two files because we create separate interfaces for clients and servers.)
// In addition, since Java doesn't support global variables (i.e., variables
// defined outside of a class), all constants are moved into a special "Consts"
// class and stored in a separate file.  All client bindings are stored in a
// separate Client.java file. Finally, package documentation (if any) is stored
// in a "package-info.java" file.
//
// The current generator doesn't yet support the full set of VDL features.  In
// particular, we don't yet support error ids and types Complex64 and Complex128.
//
// TODO(spetrovic): Run Java formatters on the generated files.
func Generate(pkg *compile.Package, env *compile.Env, config vdltool.JavaConfig) (ret []JavaFileInfo) {
	// One file for package documentation (if any).
	if g := genJavaPackageFile(pkg, env); g != nil {
		ret = append(ret, *g)
	}
	// Single file for all constants' definitions.
	if g := genJavaConstFile(pkg, env); g != nil {
		ret = append(ret, *g)
	}
	for _, file := range pkg.Files {
		// Separate file for all typedefs.
		for _, tdef := range file.TypeDefs {
			switch tdef.Type.Kind() {
			case vdl.Array:
				ret = append(ret, genJavaArrayFile(tdef, env))
			case vdl.Complex64, vdl.Complex128:
				ret = append(ret, genJavaComplexFile(tdef, env))
			case vdl.Enum:
				ret = append(ret, genJavaEnumFile(tdef, env))
			case vdl.List:
				ret = append(ret, genJavaListFile(tdef, env))
			case vdl.Map:
				ret = append(ret, genJavaMapFile(tdef, env))
			case vdl.Union:
				ret = append(ret, genJavaUnionFile(tdef, env))
			case vdl.Set:
				ret = append(ret, genJavaSetFile(tdef, env))
			case vdl.Struct:
				ret = append(ret, genJavaStructFile(tdef, env))
			default:
				ret = append(ret, genJavaPrimitiveFile(tdef, env))
			}
		}
		// Separate file for all interface definitions.
		for _, iface := range file.Interfaces {
			ret = append(ret, genJavaClientFactoryFile(iface, env))
			ret = append(ret, genJavaClientInterfaceFile(iface, env)) // client interface
			ret = append(ret, genJavaClientStubFile(iface, env))
			ret = append(ret, genJavaServerInterfaceFile(iface, env)) // server interface
			ret = append(ret, genJavaServerWrapperFile(iface, env))
		}
	}
	return
}
