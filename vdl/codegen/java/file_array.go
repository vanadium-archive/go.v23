package java

import (
	"bytes"
	"log"

	"veyron.io/veyron/veyron2/vdl/compile"
)

const arrayTmpl = `// This file was auto-generated by the veyron vdl tool.
// Source: {{.SourceFile}}

package {{.Package}};

/**
 * type {{.Name}} {{.VdlTypeString}} {{.Doc}}
 **/
@io.veyron.veyron.veyron2.vdl.GeneratedFromVdlType("{{.VdlTypeName}}")
{{ .AccessModifier }} final class {{.Name}} extends io.veyron.veyron.veyron2.vdl.VdlArray<{{.ElemType}}> {
    public static final int LENGTH = {{.Length}};

    public static final io.veyron.veyron.veyron2.vdl.VdlType VDL_TYPE =
            io.veyron.veyron.veyron2.vdl.Types.getVdlTypeFromReflection({{.Name}}.class);

    public {{.Name}}({{.ElemType}}[] impl) {
        super(VDL_TYPE, impl);
    }

    @Override
    public void writeToParcel(android.os.Parcel out, int flags) {
        java.lang.reflect.Type elemType =
                new com.google.common.reflect.TypeToken<{{.ElemType}}>(){}.getType();
        io.veyron.veyron.veyron2.vdl.ParcelUtil.writeList(out, this, elemType);
    }

    @SuppressWarnings("hiding")
    public static final android.os.Parcelable.Creator<{{.Name}}> CREATOR =
            new android.os.Parcelable.Creator<{{.Name}}>() {
        @SuppressWarnings("unchecked")
        @Override
        public {{.Name}} createFromParcel(android.os.Parcel in) {
            java.lang.reflect.Type elemType =
                    new com.google.common.reflect.TypeToken<{{.ElemType}}>(){}.getType();
            Object[] array = io.veyron.veyron.veyron2.vdl.ParcelUtil.readList(
                    in, getClass().getClassLoader(), elemType).toArray();
            return new {{.Name}}(({{.ElemType}}[]) array);
        }

        @Override
        public {{.Name}}[] newArray(int size) {
            return new {{.Name}}[size];
        }
    };
}
`

// genJavaArrayFile generates the Java class file for the provided named array type.
func genJavaArrayFile(tdef *compile.TypeDef, env *compile.Env) JavaFileInfo {
	javaTypeName := toUpperCamelCase(tdef.Name)
	data := struct {
		AccessModifier string
		Doc            string
		ElemType       string
		Length         int
		Name           string
		Package        string
		SourceFile     string
		VdlTypeName    string
		VdlTypeString  string
	}{
		AccessModifier: accessModifierForName(tdef.Name),
		Doc:            javaDocInComment(tdef.Doc),
		ElemType:       javaType(tdef.Type.Elem(), true, env),
		Length:         tdef.Type.Len(),
		Name:           javaTypeName,
		Package:        javaPath(javaGenPkgPath(tdef.File.Package.Path)),
		SourceFile:     tdef.File.BaseName,
		VdlTypeName:    tdef.Type.Name(),
		VdlTypeString:  tdef.Type.String(),
	}
	var buf bytes.Buffer
	err := parseTmpl("array", arrayTmpl).Execute(&buf, data)
	if err != nil {
		log.Fatalf("vdl: couldn't execute array template: %v", err)
	}
	return JavaFileInfo{
		Name: javaTypeName + ".java",
		Data: buf.Bytes(),
	}
}
