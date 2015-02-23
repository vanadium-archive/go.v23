package java

import (
	"text/template"
)

var tmplCache = map[string]*template.Template{}

// parseTmpl parses a template and caches the parsed value.
// Each template body must be associated with a unique name.
func parseTmpl(name string, body string) *template.Template {
	if tmpl, ok := tmplCache[name]; ok {
		return tmpl
	}

	tmpl := template.Must(template.New(name).Parse(body))

	tmplCache[name] = tmpl
	return tmpl
}
