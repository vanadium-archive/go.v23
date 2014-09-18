package java

import (
	"bytes"
	"path"

	"veyron.io/veyron/veyron2/vdl/compile"
)

// allEmbeddedIfaces returns all unique interfaces in the embed tree
// starting at the provided interface (not including that interface).
func allEmbeddedIfaces(iface *compile.Interface) (ret []*compile.Interface) {
	added := make(map[string]bool)
	for _, eIface := range iface.Embeds {
		for _, eIface = range append(allEmbeddedIfaces(eIface), eIface) {
			path := path.Join(eIface.File.Package.Path, eIface.Name)
			if _, ok := added[path]; ok { // already added iface
				continue
			}
			ret = append(ret, eIface)
			added[path] = true
		}
	}
	return
}

// interfaceFullyQualifiedName outputs the fully qualified name of an interface
// e.g. "com.a.B"
func interfaceFullyQualifiedName(iface *compile.Interface) string {
	return path.Join(javaGenPkgPath(iface.File.Package.Path), iface.Name)
}

// javaExtendsStr creates an extends clause for an interface
// e.g. "extends com.a.B, com.d.E"
func javaExtendsStr(embeds []*compile.Interface, suffix string) string {
	if len(embeds) == 0 {
		return ""
	}

	var buf bytes.Buffer
	buf.WriteString("extends ")
	for i, embed := range embeds {
		if i > 0 {
			buf.WriteString(", ")
		}
		buf.WriteString(javaPath(interfaceFullyQualifiedName(embed)))
		buf.WriteString(suffix)
	}
	return buf.String()
}
