package java

import (
	"fmt"
	"log"
	"path"
	"strings"

	"veyron.io/veyron/veyron2/vdl"
	"veyron.io/veyron/veyron2/vdl/compile"
)

func javaFullyQualifiedNamedType(def *compile.TypeDef, forceClass bool, env *compile.Env) string {
	if def.File == compile.BuiltInFile {
		name, _ := javaBuiltInType(def.Type, forceClass)
		return name
	}
	return javaPath(path.Join(javaGenPkgPath(def.File.Package.Path), def.Name))
}

// javaBuiltInType returns the type name for the provided built in type
// definition, forcing the use of a java class (e.g., java.lang.Integer) if so
// desired.  This method also returns a boolean value indicating whether the
// returned type is a class.
func javaBuiltInType(typ *vdl.Type, forceClass bool) (string, bool) {
	if typ == nil {
		if forceClass {
			return "java.lang.Void", true
		} else {
			return "void", false
		}
	}
	if typ.Name() == "error" {
		return "io.veyron.veyron.veyron2.VeyronException", true
	}
	switch typ.Kind() {
	case vdl.Bool:
		if forceClass {
			return "java.lang.Boolean", true
		} else {
			return "boolean", false
		}
	case vdl.Byte:
		if forceClass {
			return "java.lang.Byte", true
		} else {
			return "byte", false
		}
	case vdl.Uint16, vdl.Int16:
		if forceClass {
			return "java.lang.Short", true
		} else {
			return "short", false
		}
	case vdl.Uint32, vdl.Int32:
		if forceClass {
			return "java.lang.Integer", true
		} else {
			return "int", false
		}
	case vdl.Uint64, vdl.Int64:
		if forceClass {
			return "java.lang.Long", true
		} else {
			return "long", false
		}
	case vdl.Float32:
		if forceClass {
			return "java.lang.Float", true
		} else {
			return "float", false
		}
	case vdl.Float64:
		if forceClass {
			return "java.lang.Double", true
		} else {
			return "double", false
		}
	case vdl.Complex64, vdl.Complex128:
		return "org.apache.commons.math3.complex.Complex", true
	case vdl.String:
		return "java.lang.String", true
	// TODO(spetrovic): handle typeobject correctly.
	case vdl.TypeObject:
		return "java.lang.Object", true
	case vdl.Any:
		return "io.veyron.veyron.veyron2.vdl.Any", true
	default:
		return "", false
	}
}

func javaType(t *vdl.Type, forceClass bool, env *compile.Env) string {
	if t == nil {
		name, _ := javaBuiltInType(nil, forceClass)
		return name
	}
	if def := env.FindTypeDef(t); def != nil {
		return javaFullyQualifiedNamedType(def, forceClass, env)
	}
	switch t.Kind() {
	case vdl.Array:
		return fmt.Sprintf("%s[]", javaType(t.Elem(), false, env))
	case vdl.List:
		// NOTE(spetrovic): We represent byte lists as Java byte arrays, as it's doubtful anybody
		// would want to use them as Java lists.
		if javaType(t.Elem(), false, env) == "byte" {
			return fmt.Sprintf("byte[]")
		}
		return fmt.Sprintf("%s<%s>", "java.util.List", javaType(t.Elem(), true, env))
	case vdl.Set:
		return fmt.Sprintf("%s<%s>", "java.util.Set", javaType(t.Key(), true, env))
	case vdl.Map:
		return fmt.Sprintf("%s<%s, %s>", "java.util.Map", javaType(t.Key(), true, env), javaType(t.Elem(), true, env))
	default:
		log.Fatalf("vdl: javaType unhandled type %v %v", t.Kind(), t)
		return ""
	}
}

// javaHashCode returns the java code for the hashCode() computation for a given type.
func javaHashCode(name string, ty *vdl.Type, env *compile.Env) string {
	if def := env.FindTypeDef(ty); def != nil && def.File == compile.BuiltInFile {
		switch ty.Kind() {
		case vdl.Bool:
			return fmt.Sprintf("java.lang.Boolean.valueOf(%s).hashCode()", name)
		case vdl.Byte, vdl.Uint16, vdl.Int16:
			return "(int)" + name
		case vdl.Uint32, vdl.Int32:
			return name
		case vdl.Uint64, vdl.Int64:
			return fmt.Sprintf("java.lang.Long.valueOf(%s).hashCode()", name)
		case vdl.Float32:
			return fmt.Sprintf("java.lang.Float.valueOf(%s).hashCode()", name)
		case vdl.Float64:
			return fmt.Sprintf("java.lang.Double.valueOf(%s).hashCode()", name)
		case vdl.Array:
			return fmt.Sprintf("java.util.Arrays.hashCode(%s)", name)
		}
	}
	return fmt.Sprintf("(%s == null ? 0 : %s.hashCode())", name, name)
}

// isClass returns true iff the provided type is represented by a Java class.
func isClass(t *vdl.Type, env *compile.Env) bool {
	if t == nil { // void type
		return false
	}
	if def := env.FindTypeDef(t); def != nil && def.File == compile.BuiltInFile {
		// Built-in type.  See if it's represented by a class.
		if tname, isClass := javaBuiltInType(t, false); tname != "" && !isClass {
			return false
		}
	}
	return true
}

// isJavaNativeArray returns true iff the provided type is represented by a Java array.
func isJavaNativeArray(t *vdl.Type, env *compile.Env) bool {
	typeStr := javaType(t, false, env)
	return strings.HasSuffix(typeStr, "[]")
}

func bitlen(kind vdl.Kind) int {
	switch kind {
	case vdl.Float32, vdl.Complex64:
		return 32
	case vdl.Float64, vdl.Complex128:
		return 64
	}
	panic(fmt.Errorf("vdl: bitLen unhandled kind %v", kind))
}
