package java

import (
	"bytes"
	"log"

	"veyron.io/veyron/veyron2/vdl/compile"
)

// TODO(rogulenko): Rename to file_union.go, and rename java class to VdlUnion.

const unionTmpl = `
// This file was auto-generated by the veyron vdl tool.
// Source: {{.Source}}
package {{.PackagePath}};

/**
 * type {{.Name}} {{.VdlTypeString}} {{.Doc}}
 **/
@io.veyron.veyron.veyron2.vdl.GeneratedFromVdl(name = "{{.VdlTypeName}}")
{{ .AccessModifier }} class {{.Name}} extends io.veyron.veyron.veyron2.vdl.VdlOneOf {
    {{ range $index, $field := .Fields }}
    @io.veyron.veyron.veyron2.vdl.GeneratedFromVdl(name = "{{$field.Name}}", index = {{$index}})
    public static class {{$field.Name}} extends {{$.Name}} {
        private {{$field.Type}} elem;

        public {{$field.Name}}({{$field.Type}} elem) {
            super({{$index}}, elem);
            this.elem = elem;
        }

        @Override
        public {{$field.Class}} getElem() {
            return elem;
        }

        @Override
        public int hashCode() {
            return {{$field.HashcodeComputation}};
        }
    }
    {{ end }}

    public static final io.veyron.veyron.veyron2.vdl.VdlType VDL_TYPE =
            io.veyron.veyron.veyron2.vdl.Types.getVdlTypeFromReflect({{.Name}}.class);

    public {{.Name}}(int index, java.io.Serializable value) {
        super(VDL_TYPE, index, value);
    }
}
`

type unionDefinitionField struct {
	Class               string
	HashcodeComputation string
	Name                string
	Type                string
}

// genJavaUnionFile generates the Java class file for the provided user-defined union type.
func genJavaUnionFile(tdef *compile.TypeDef, env *compile.Env) JavaFileInfo {
	fields := make([]unionDefinitionField, tdef.Type.NumField())
	for i := 0; i < tdef.Type.NumField(); i++ {
		fld := tdef.Type.Field(i)
		fields[i] = unionDefinitionField{
			Class:               javaType(fld.Type, true, env),
			HashcodeComputation: javaHashCode("elem", fld.Type, env),
			Name:                fld.Name,
			Type:                javaType(fld.Type, false, env),
		}
	}
	javaTypeName := toUpperCamelCase(tdef.Name)
	data := struct {
		AccessModifier string
		Doc            string
		Fields         []unionDefinitionField
		Name           string
		PackagePath    string
		Source         string
		VdlTypeName    string
		VdlTypeString  string
	}{
		AccessModifier: accessModifierForName(tdef.Name),
		Doc:            javaDocInComment(tdef.Doc),
		Fields:         fields,
		Name:           javaTypeName,
		PackagePath:    javaPath(javaGenPkgPath(tdef.File.Package.Path)),
		Source:         tdef.File.BaseName,
		VdlTypeName:    tdef.Type.Name(),
		VdlTypeString:  tdef.Type.String(),
	}
	var buf bytes.Buffer
	err := parseTmpl("union", unionTmpl).Execute(&buf, data)
	if err != nil {
		log.Fatalf("vdl: couldn't execute union template: %v", err)
	}
	return JavaFileInfo{
		Name: javaTypeName + ".java",
		Data: buf.Bytes(),
	}
}
