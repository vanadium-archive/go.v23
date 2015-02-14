package signature

// TODO(toddw): Add tests.

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"v.io/core/veyron2/vdl"
	"v.io/core/veyron2/vdl/codegen/vdlgen"
	"v.io/lib/textutil"
)

// NamedTypes represents a set of unique named types.  The main usage is to
// collect the named types from one or more signatures, and to print all of the
// named types.
type NamedTypes struct {
	types map[*vdl.Type]bool
}

// Add adds to x all named types from the type graph rooted at t.
func (x *NamedTypes) Add(t *vdl.Type) {
	if t == nil {
		return
	}
	t.Walk(vdl.WalkAll, func(t *vdl.Type) bool {
		if t.Name() != "" {
			if x.types == nil {
				x.types = make(map[*vdl.Type]bool)
			}
			x.types[t] = true
		}
		return true
	})
}

// Print pretty-prints x to w.
func (x NamedTypes) Print(w io.Writer) {
	var sorted typesByPkgAndName
	for t, _ := range x.types {
		sorted = append(sorted, t)
	}
	sort.Sort(sorted)
	for _, t := range sorted {
		path, name := vdl.SplitIdent(t.Name())
		fmt.Fprintf(w, "\ntype %s %s\n", qualifiedName(path, name), vdlgen.BaseType(t, "", nil))
	}
}

// typesByPkgAndName orders types by package path, then by name.
type typesByPkgAndName []*vdl.Type

func (x typesByPkgAndName) Len() int      { return len(x) }
func (x typesByPkgAndName) Swap(i, j int) { x[i], x[j] = x[j], x[i] }
func (x typesByPkgAndName) Less(i, j int) bool {
	ipkg, iname := vdl.SplitIdent(x[i].Name())
	jpkg, jname := vdl.SplitIdent(x[j].Name())
	if ipkg != jpkg {
		return ipkg < jpkg
	}
	return iname < jname
}

// Print pretty-prints x to w.  The types are used to collect named types, so
// that they may be printed later.
func (x Interface) Print(w io.Writer, types *NamedTypes) {
	printDoc(w, x.Doc)
	fmt.Fprintf(w, "type %s interface {", qualifiedName(x.PkgPath, x.Name))
	indentW := textutil.ByteReplaceWriter(w, '\n', "\n\t")
	for _, embed := range x.Embeds {
		fmt.Fprint(indentW, "\n")
		embed.Print(indentW)
	}
	for _, method := range x.Methods {
		fmt.Fprint(indentW, "\n")
		method.Print(indentW, types)
	}
	fmt.Fprintf(w, "\n}")
}

// Print pretty-prints x to w.
func (x Embed) Print(w io.Writer) {
	printDoc(w, x.Doc)
	fmt.Fprint(w, qualifiedName(x.PkgPath, x.Name))
}

// Print pretty-prints x to w.  The types are used to collect named types, so
// that they may be printed later.
func (x Method) Print(w io.Writer, types *NamedTypes) {
	printDoc(w, x.Doc)
	fmt.Fprintf(w, "%s(%s)%s%s%s", x.Name, inArgsStr(x.InArgs, types), streamArgsStr(x.InStream, x.OutStream, types), outArgsStr(x.OutArgs, types), tagsStr(x.Tags, types))
}

func inArgsStr(args []Arg, types *NamedTypes) string {
	var ret []string
	for _, arg := range args {
		ret = append(ret, argStr(arg, types))
	}
	return strings.Join(ret, ", ")
}

func outArgsStr(args []Arg, types *NamedTypes) string {
	var ret []string
	for i, arg := range args {
		if i == len(args)-1 && arg.Type == vdl.ErrorType {
			// TODO(toddw): At the moment we don't have a consistent error-handling
			// strategy for out-args, so we have this hack.  Make the error-handling
			// strategy consistent, and remove this hack.
			continue
		}
		ret = append(ret, argStr(arg, types))
	}
	if len(ret) == 0 {
		return " error"
	}
	return fmt.Sprintf(" (%s | error)", strings.Join(ret, ", "))
}

func argStr(arg Arg, types *NamedTypes) string {
	// TODO(toddw): Print arg.Doc somewhere?
	var ret string
	if arg.Name != "" {
		ret += arg.Name + " "
	}
	types.Add(arg.Type)
	return ret + vdlgen.Type(arg.Type, "", nil)
}

func streamArgsStr(in, out *Arg, types *NamedTypes) string {
	if in == nil && out == nil {
		return ""
	}
	return fmt.Sprintf(" stream<%s, %s>", streamArgStr(in, types), streamArgStr(out, types))
}

func streamArgStr(arg *Arg, types *NamedTypes) string {
	// TODO(toddw): Print arg.Name and arg.Doc?
	if arg == nil {
		return "_"
	}
	types.Add(arg.Type)
	return vdlgen.Type(arg.Type, "", nil)
}

func tagsStr(tags []vdl.AnyRep, types *NamedTypes) string {
	if len(tags) == 0 {
		return ""
	}
	var ret []string
	for _, tag := range tags {
		var tagval *vdl.Value
		if err := vdl.Convert(&tagval, tag); err != nil {
			// We shouldn't get an error in conversion, but if we do, do our best and
			// print the tag as a Go value.
			ret = append(ret, fmt.Sprintf("%T(%v)", tag, tag))
		} else {
			types.Add(tagval.Type())
			ret = append(ret, vdlgen.TypedConst(tagval, "", nil))
		}
	}
	return fmt.Sprintf(" {%s}", strings.Join(ret, ", "))
}

func qualifiedName(pkgpath, name string) string {
	if pkgpath == "" {
		if name == "" {
			return "<empty>"
		}
		return name
	}
	return fmt.Sprintf("%q.%s", pkgpath, name)
}

func printDoc(w io.Writer, doc string) {
	if doc == "" {
		return
	}
	// TODO(toddw): Normalize the doc strings so that it never has the // or /**/
	// comment markers, and add them consistently here.  And use LineWriter to
	// pretty-print the doc.
	if !strings.HasPrefix(doc, "//") {
		fmt.Fprintln(w, "// "+doc)
	} else {
		fmt.Fprintln(w, doc)
	}
}
