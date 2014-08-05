// package viewer exports a store through an HTTP server, with the following
// features.
//
// URL paths correspond to store paths.  For example, if the URL is
// http://myhost/a/b/c, the store value that is fetched is /a/b/c.
//
// Values can be formatted using html templates (documented in the
// text/templates package).  Templates are named according to the type of the
// value that they format, using a path /templates/<pkgPath>/<typeName>.  For
// example, suppose we are viewing the page for /movies/Inception, and it
// contains a value of type *examples/store/mdb/schema.Movie.  We fetch the
// template /templates/examples/store/mdb/schema/Movie, which must be a string
// in html/template format.  If it exists, the template is compiled and used to
// print the value.  If the template does not exist, the value is formatted in
// raw form.
//
// String values that have a path ending with suffix .css are printed in raw
// form.
package viewer

import (
	"fmt"
	"html"
	"html/template"
	"io"
	"net/http"
	"path/filepath"

	"veyron2/rt"
	"veyron2/storage"
	"veyron2/vlog"
)

// server is the HTTP server handler.
type server struct {
	store storage.Store
}

var _ http.Handler = (*server)(nil)

const (
	// rawTemplateText is used to format the output in a raw textual form.
	rawTemplateText = `<html>
{{$value := .}}
<head>
<title>{{.Name}}</title>
</head>
<body>
<h1>{{.Name}}</h1>
<pre>{{.Value}}</pre>
{{with .Subdirs}}
<h3>Subdirectories</h3>
{{range .}}
<p><a href="{{.}}">{{$value.Base .}}</a></p>
{{end}}
{{end}}
</body>
</html>`
)

var (
	rawTemplate = mustParse("raw", rawTemplateText)
)

// mustParse parses the template text.  It panics on error.
func mustParse(name, text string) *template.Template {
	tmpl, err := template.New(name).Parse(text)
	if err != nil {
		panic(fmt.Sprintf("Error parsing template %q: %s", text, err))
	}
	return tmpl
}

// loadTemplate fetches the template for the value from the store.  The template
// is based on the type of the value, under /template/<pkgPath>/<typeName>.
func (s *server) loadTemplate(v interface{}) *template.Template {
	path := templatePath(v)
	e, err := s.store.BindObject(path).Get(rt.R().TODOContext())
	if err != nil {
		return nil
	}
	str, ok := e.Value.(string)
	if !ok {
		return nil
	}
	tmpl, err := template.New(path).Parse(str)
	if err != nil {
		vlog.Infof("Template error: %s: %s", path, err)
		return nil
	}
	return tmpl
}

// printRawValuePage prints the value in raw format.
func (s *server) printRawValuePage(w http.ResponseWriter, path string, v interface{}) {
	var p printer
	p.print(v)
	subdirs, _ := glob(s.store, path, "*")
	x := &Value{Name: path, Value: p.String(), Subdirs: subdirs}
	if err := rawTemplate.Execute(w, x); err != nil {
		w.Write([]byte(html.EscapeString(err.Error())))
	}
}

// printValuePage prints the value using a template if possible.  If a template
// is not found, the value is printed in raw format instead.
func (s *server) printValuePage(w http.ResponseWriter, path string, v interface{}) {
	if tmpl := s.loadTemplate(v); tmpl != nil {
		if err := tmpl.Execute(w, &Value{store: s.store, Name: path, Value: v}); err != nil {
			w.Write([]byte(html.EscapeString(err.Error())))
		}
		return
	}
	s.printRawValuePage(w, path, v)
}

// printRawPage prints a string value directly, without processing.
func (s *server) printRawPage(w http.ResponseWriter, v interface{}) {
	str, ok := v.(string)
	if !ok {
		fmt.Fprintf(w, "%s", v)
	} else {
		io.WriteString(w, str)
	}
}

// ServeHTTP is the main HTTP handler.
func (s *server) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	path := req.URL.Path
	e, err := s.store.BindObject(path).Get(rt.R().TODOContext())
	if err != nil {
		msg := fmt.Sprintf("<html><body><h1>%s</h1><h2>Error: %s</h2></body></html>",
			html.EscapeString(path),
			html.EscapeString(err.Error()))
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(msg))
		return
	}

	q := req.URL.Query()
	switch filepath.Ext(path) {
	case ".css":
		s.printRawPage(w, e.Value)
	default:
		if q["raw"] != nil {
			s.printRawValuePage(w, path, e.Value)
		} else {
			s.printValuePage(w, path, e.Value)
		}
	}
}

// ListenAndServe is the main entry point.  It serves store at the specified
// network address.
func ListenAndServe(addr string, st storage.Store) error {
	s := &server{store: st}
	vlog.Infof("Viewer running at http://localhost%s", addr)
	return http.ListenAndServe(addr, s)
}
