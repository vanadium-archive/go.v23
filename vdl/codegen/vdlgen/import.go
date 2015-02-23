// Package vdlgen implements VDL code generation from compiled VDL packages.
package vdlgen

// TODO(toddw): Add tests

import (
	"v.io/core/veyron2/vdl/codegen"
)

// Imports returns the vdl imports clause corresponding to imports; empty if
// there are no imports.
func Imports(imports codegen.Imports) string {
	var s string
	if len(imports) > 0 {
		s += "import ("
		for _, imp := range imports {
			s += "\n\t"
			if imp.Name != "" {
				s += imp.Name + " "
			}
			s += imp.Path
		}
		s += "\n)"
	}
	return s
}
