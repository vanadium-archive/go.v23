package javascript

import (
	"fmt"
	"sort"
	"strings"
	"veyron.io/veyron/veyron2/vdl/compile"

	"veyron.io/veyron/veyron2/vdl"
)

// typeNames holds a mapping between VDL type and generated type name.
type typeNames map[*vdl.Type]string

// LookupName looks up the name of a type.
// If it is already generated (in tvdn), it looks up the name. Otherwise it
// produces a name referencing a type in another package of the form
// "[Package].[Name]".
func (tn typeNames) LookupName(t *vdl.Type) string {
	if name, ok := tn[t]; ok {
		return name
	}

	pkgPath, name := vdl.SplitIdent(t.Name())
	pkgParts := strings.Split(pkgPath, "/")
	pkgName := pkgParts[len(pkgParts)-1]
	return fmt.Sprintf("%s.%s", pkgName, name)
}

// SortedList returns a list of type and name pairs, sorted by name.
// This is needed to make the output stable.
func (tn typeNames) SortedList() typeNamePairList {
	pairs := typeNamePairList{}
	for t, name := range tn {
		pairs = append(pairs, typeNamePair{t, name})
	}
	sort.Sort(pairs)
	return pairs
}

func newTypeNames(pkg *compile.Package) typeNames {
	nextIndex := 1
	names := typeNames{}
	for _, file := range pkg.Files {
		for _, def := range file.TypeDefs {
			nextIndex = getInnerTypes(def.Type, names, nextIndex)
		}
		for _, constdef := range file.ConstDefs {
			nextIndex = getInnerTypes(constdef.Value.Type(), names, nextIndex)
			nextIndex = addTypesInConst(constdef.Value, names, nextIndex)
		}
		for _, interfacedef := range file.Interfaces {
			for _, method := range interfacedef.AllMethods() {
				for _, inarg := range method.InArgs {
					nextIndex = getInnerTypes(inarg.Type, names, nextIndex)
				}
				for _, outarg := range method.OutArgs {
					nextIndex = getInnerTypes(outarg.Type, names, nextIndex)
				}
				if method.InStream != nil {
					nextIndex = getInnerTypes(method.InStream, names, nextIndex)
				}
				if method.OutStream != nil {
					nextIndex = getInnerTypes(method.OutStream, names, nextIndex)
				}
			}
		}
	}
	return names
}

type typeNamePairList []typeNamePair
type typeNamePair struct {
	Type *vdl.Type
	Name string
}

func (l typeNamePairList) Len() int           { return len(l) }
func (l typeNamePairList) Less(i, j int) bool { return l[i].Name < l[j].Name }
func (l typeNamePairList) Swap(i, j int)      { l[i], l[j] = l[j], l[i] }

func makeName(t *vdl.Type, nextIndex int) (string, int) {
	var name string
	if t.Name() != "" {
		_, n := vdl.SplitIdent(t.Name())
		name = "_type" + n
	} else {
		name = fmt.Sprintf("_type%d", nextIndex)
		nextIndex++
	}
	return name, nextIndex
}

func getInnerTypes(t *vdl.Type, names typeNames, nextIndex int) int {
	if _, ok := names[t]; ok {
		return nextIndex
	}
	names[t], nextIndex = makeName(t, nextIndex)

	switch t.Kind() {
	case vdl.Optional, vdl.Array, vdl.List, vdl.Map:
		nextIndex = getInnerTypes(t.Elem(), names, nextIndex)
	}

	switch t.Kind() {
	case vdl.Set, vdl.Map:
		nextIndex = getInnerTypes(t.Key(), names, nextIndex)
	}

	switch t.Kind() {
	case vdl.Struct, vdl.OneOf:
		for i := 0; i < t.NumField(); i++ {
			nextIndex = getInnerTypes(t.Field(i).Type, names, nextIndex)
		}
	}

	return nextIndex
}

func addTypesInConst(v *vdl.Value, names typeNames, nextIndex int) int {
	// Generate the type if it is a typeobject or any.
	switch v.Kind() {
	case vdl.TypeObject:
		nextIndex = getInnerTypes(v.TypeObject(), names, nextIndex)
	case vdl.Any:
		if !v.IsNil() {
			nextIndex = getInnerTypes(v.Elem().Type(), names, nextIndex)
		}
	}

	// Recurse.
	switch v.Kind() {
	case vdl.List, vdl.Array:
		for i := 0; i < v.Len(); i++ {
			nextIndex = addTypesInConst(v.Index(i), names, nextIndex)
		}
	case vdl.Set:
		for _, key := range v.Keys() {
			nextIndex = addTypesInConst(key, names, nextIndex)
		}
	case vdl.Map:
		for _, key := range v.Keys() {
			nextIndex = addTypesInConst(key, names, nextIndex)
			nextIndex = addTypesInConst(v.MapIndex(key), names, nextIndex)

		}
	case vdl.Struct:
		for i := 0; i < v.Type().NumField(); i++ {
			nextIndex = addTypesInConst(v.Field(i), names, nextIndex)
		}
	case vdl.OneOf:
		_, innerVal := v.OneOfField()
		nextIndex = addTypesInConst(innerVal, names, nextIndex)
	case vdl.Any, vdl.Optional:
		if !v.IsNil() {
			nextIndex = addTypesInConst(v.Elem(), names, nextIndex)

		}
	}

	return nextIndex
}
