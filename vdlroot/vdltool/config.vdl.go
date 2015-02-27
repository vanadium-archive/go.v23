// This file was auto-generated by the veyron vdl tool.
// Source: config.vdl

// Package vdltool defines types used by the vdl tool itself, including the
// format of vdl.config files.
package vdltool

import (
	// VDL system imports
	"fmt"
	"v.io/v23/vdl"
)

// Config specifies the configuration for the vdl tool.  This is typically
// represented in optional "vdl.config" files in each vdl source package.  Each
// vdl.config file implicitly imports this package.  E.g. you may refer to
// vdltool.Config in the "vdl.config" file without explicitly importing vdltool.
type Config struct {
	// GenLanguages restricts the set of code generation languages.  If the set is
	// empty, all supported languages are allowed to be generated.
	GenLanguages map[GenLanguage]struct{}
	// Language-specific configurations.
	Go         GoConfig
	Java       JavaConfig
	Javascript JavascriptConfig
}

func (Config) __VDLReflect(struct {
	Name string "vdltool.Config"
}) {
}

// GenLanguage enumerates the known code generation languages.
type GenLanguage int

const (
	GenLanguageGo GenLanguage = iota
	GenLanguageJava
	GenLanguageJavascript
)

// GenLanguageAll holds all labels for GenLanguage.
var GenLanguageAll = []GenLanguage{GenLanguageGo, GenLanguageJava, GenLanguageJavascript}

// GenLanguageFromString creates a GenLanguage from a string label.
func GenLanguageFromString(label string) (x GenLanguage, err error) {
	err = x.Set(label)
	return
}

// Set assigns label to x.
func (x *GenLanguage) Set(label string) error {
	switch label {
	case "Go", "go":
		*x = GenLanguageGo
		return nil
	case "Java", "java":
		*x = GenLanguageJava
		return nil
	case "Javascript", "javascript":
		*x = GenLanguageJavascript
		return nil
	}
	*x = -1
	return fmt.Errorf("unknown label %q in vdltool.GenLanguage", label)
}

// String returns the string label of x.
func (x GenLanguage) String() string {
	switch x {
	case GenLanguageGo:
		return "Go"
	case GenLanguageJava:
		return "Java"
	case GenLanguageJavascript:
		return "Javascript"
	}
	return ""
}

func (GenLanguage) __VDLReflect(struct {
	Name string "vdltool.GenLanguage"
	Enum struct{ Go, Java, Javascript string }
}) {
}

// GoConfig specifies go specific configuration.
type GoConfig struct {
	// WireToNativeTypes specifies the mapping from a VDL wire type to its Go
	// native type representation.  This is rarely used and easy to configure
	// incorrectly; usage is currently restricted to packages that are explicitly
	// whitelisted.
	//
	// WireToNativeTypes are meant for scenarios where there is an idiomatic Go
	// type used in your code, but you need a standard VDL representation for wire
	// compatibility.  E.g. the VDL time package defines Duration and Time for
	// wire compatibility, but we want the generated code to use the standard Go
	// time package.
	//
	// The key of the map is the name of the VDL type (aka WireType), which must
	// be defined in the vdl package associated with the vdl.config file.
	//
	// The code generator assumes the existence of a pair of conversion functions
	// converting between the wire and native types, and will automatically call
	// vdl.RegisterNative with these function names.
	//
	// Assuming the name of the WireType is Foo:
	//   func fooToNative(x Foo, n *Native) error
	//   func fooFromNative(x *Foo, n Native) error
	WireToNativeTypes map[string]GoType
}

func (GoConfig) __VDLReflect(struct {
	Name string "vdltool.GoConfig"
}) {
}

// GoType describes the Go type information associated with a VDL type.
// See v.io/core/veyron/lib/vdl/testdata/native for examples.
type GoType struct {
	// Type is the Go type to use in generated code, instead of the VDL type.  If
	// the Go type requires additional imports, specify the type using the
	// standard local package name here, and also specify the import package in
	// Imports.
	Type string
	// Imports are the Go imports to use in generated code, required by the Type.
	Imports []GoImport
}

func (GoType) __VDLReflect(struct {
	Name string "vdltool.GoType"
}) {
}

// GoImport describes Go import information.
type GoImport struct {
	// Path is the package path that uniquely identifies the imported package.
	Path string
	// Name is the name of the package identified by Path.  Due to Go conventions,
	// it is typically just the basename of Path, but may be set to something
	// different if the imported package doesn't follow Go conventions.
	Name string
}

func (GoImport) __VDLReflect(struct {
	Name string "vdltool.GoImport"
}) {
}

// JavaConfig specifies java specific configuration.
type JavaConfig struct {
	// WireToNativeTypes specifies the mapping from a VDL wire type to its Java
	// native type representation.  This is rarely used and easy to configure
	// incorrectly; usage is currently restricted to packages that are explicitly
	// whitelisted.
	//
	// WireToNativeTypes are meant for scenarios where there is an idiomatic Java
	// type used in your code, but you need a standard VDL representation for wire
	// compatibility.  E.g. the VDL time package defines Duration and Time for
	// wire compatibility, but we want the generated code to use the org.joda.time
	// package.
	//
	// The key of the map is the name of the VDL type (aka WireType), which must
	// be defined in the vdl package associated with the vdl.config file.
	//
	// The code generator assumes that the conversion functions will be registered
	// in java vdl package.
	WireToNativeTypes map[string]string
}

func (JavaConfig) __VDLReflect(struct {
	Name string "vdltool.JavaConfig"
}) {
}

// JavascriptConfig specifies javascript specific configuration.
type JavascriptConfig struct {
}

func (JavascriptConfig) __VDLReflect(struct {
	Name string "vdltool.JavascriptConfig"
}) {
}

func init() {
	vdl.Register((*Config)(nil))
	vdl.Register((*GenLanguage)(nil))
	vdl.Register((*GoConfig)(nil))
	vdl.Register((*GoType)(nil))
	vdl.Register((*GoImport)(nil))
	vdl.Register((*JavaConfig)(nil))
	vdl.Register((*JavascriptConfig)(nil))
}