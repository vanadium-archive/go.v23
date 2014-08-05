package viewer

import (
	"bytes"
	"fmt"
	"reflect"

	"veyron2/storage"
)

// printer defines a pretty printer for Go values, using reflection to traverse
// the values.
type printer struct {
	buf bytes.Buffer
}

var (
	tyID = reflect.TypeOf(storage.ID{})
)

// stringTypePointers removes the outer Ptr and Interface types.
func stripTypePointers(ty reflect.Type) reflect.Type {
	kind := ty.Kind()
	for kind == reflect.Ptr || kind == reflect.Interface {
		ty = ty.Elem()
		kind = ty.Kind()
	}
	return ty
}

// templatePath returns the path to the templates value, defined as
// /templates/<pkgPath>/<typeName>.
func templatePath(v interface{}) string {
	ty := stripTypePointers(reflect.TypeOf(v))
	pkgPath := ty.PkgPath()
	tyName := ty.Name()
	if pkgPath == "" || tyName == "" {
		return ""
	}
	return fmt.Sprintf("templates/%s/%s", pkgPath, tyName)
}

// print formats the argument.
func (p *printer) print(v interface{}) {
	p.printValue(0, reflect.ValueOf(v))
}

// printType formats the type.
func (p *printer) printType(indent int, v reflect.Type) {
	p.printString(v.Name())
}

// printValue is the main pretty-printer method.  It formats the value at the
// indentation level specified by the argument.
func (p *printer) printValue(indent int, v reflect.Value) {
	if !v.IsValid() {
		p.printString("<invalid>")
		return
	}
	ty := v.Type()
	if ty == tyID {
		p.printString(v.Interface().(storage.ID).String())
		return
	}
	switch ty.Kind() {
	case reflect.Ptr:
		p.printString("&")
		fallthrough
	case reflect.Interface:
		p.printValue(indent, v.Elem())
	case reflect.Array, reflect.Slice:
		p.printSliceValue(indent, v)
	case reflect.Map:
		p.printMapValue(indent, v)
	case reflect.Struct:
		p.printStructValue(indent, v)
	case reflect.String:
		fmt.Fprintf(&p.buf, "%q", v.Interface())
	default:
		fmt.Fprintf(&p.buf, "%+v", v.Interface())
	}
}

// printSliceValue formats a slice or array.
func (p *printer) printSliceValue(indent int, v reflect.Value) {
	p.printType(indent, v.Type())
	p.printString("{")
	l := v.Len()
	for i := 0; i < l; i++ {
		p.printIndent(indent + 1)
		p.printValue(indent+1, v.Index(i))
		p.printString(",")
	}
	p.printIndent(indent)
	p.printString("}")
}

// printMapValue formats a map.
func (p *printer) printMapValue(indent int, v reflect.Value) {
	p.printType(indent, v.Type())
	p.printString("{")
	for _, key := range v.MapKeys() {
		p.printIndent(indent + 1)
		p.printValue(indent+1, key)
		p.printString(": ")
		p.printValue(indent+2, v.MapIndex(key))
		p.printString(",")
	}
	p.printIndent(indent)
	p.printString("}")
}

// printStructValue formats a struct.
func (p *printer) printStructValue(indent int, v reflect.Value) {
	ty := v.Type()
	p.printType(indent, ty)
	p.printString("{")
	l := ty.NumField()
	for i := 0; i < l; i++ {
		field := ty.Field(i)
		if field.PkgPath != "" {
			continue
		}
		p.printIndent(indent + 1)
		p.printString(field.Name)
		p.printString(": ")
		p.printValue(indent+1, v.Field(i))
		p.printString(",")
	}
	p.printIndent(indent)
	p.printString("}")
}

// printString adds a string to the output.
func (p *printer) printString(s string) {
	p.buf.WriteString(s)
}

// printIndent prints a newline and then indents to the specified indentation.
func (p *printer) printIndent(indent int) {
	p.buf.WriteRune('\n')
	for i := 0; i < indent; i++ {
		p.buf.WriteString("  ")
	}
}

// String returns the formatted text.
func (p *printer) String() string {
	return p.buf.String()
}
