package vom

import (
	"bytes"
	"fmt"
	"io"
	"reflect"
	"strconv"
)

// VOMBinaryToJSON reads from a VOM binary input and converts it to a JSON output.
func VOMBinaryToJSON(w io.Writer, r io.Reader) error {
	tr := NewBinaryToJSONTranscoder(w, r)
	return tr.Transcode()
}

// ObjToJSON writes a Value to a writer as a JSON object. This is more robust than encoding/json
// because it can write objects as key strings and also is able to detect and error on cycles and
// other desired behaviors.
func ObjToJSON(w io.Writer, v Value) error {
	_, err := objToJSONInternal(w, v, map[uintptr]bool{})
	return err
}

// objToJSONInternal is the main recursive function used by ObjToJSON. It differs from ObjToJSON in
// that it keeps a map of seen values to ensure the outputted structure is a tree and returns
// whether the object was wrapped by quotes (string, error).
func objToJSONInternal(w io.Writer, v Value, seenptr map[uintptr]bool) (outerQuotes bool, err error) {
	kind := reflect.Invalid
	if v != nil {
		kind = v.Kind()
	}

	switch kind {
	// objects:
	case reflect.Struct:
		return false, structToJSONInternal(w, v, seenptr)
	case reflect.Map:
		return false, mapToJSONInternal(w, v, seenptr)

	// lists:
	case reflect.Array, reflect.Slice:
		return false, listToJSONInternal(w, v, seenptr)

	// indirection:
	case reflect.Ptr:
		if _, ok := seenptr[v.Pointer()]; ok {
			return false, fmt.Errorf("can only convert trees to JSON")
		}
		seenptr[v.Pointer()] = true
		return objToJSONInternal(w, v.Elem(), seenptr)
	case reflect.Interface:
		_, err := objToJSONInternal(w, v.Elem(), seenptr)
		// return outerQuotes = false, so that we output strings as "\"str\"" rather than "str"
		// so we can later detect the type in the interface.
		return false, err

	// primitives:
	case reflect.String:
		if _, err := io.WriteString(w, strconv.Quote(v.String())); err != nil {
			return false, err
		}
		return true, nil
	default:
		var s string
		switch kind {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			s = strconv.FormatInt(v.Int(), 10)
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
			s = strconv.FormatUint(v.Uint(), 10)
		case reflect.Float32, reflect.Float64:
			s = fmt.Sprintf("%v", v.Float())
		case reflect.Bool:
			s = strconv.FormatBool(v.Bool())
		case reflect.Invalid:
			s = "null"
		default:
			panic(fmt.Sprintf("unhandled kind %v", v.Kind()))
		}
		if _, err := io.WriteString(w, s); err != nil {
			return false, err
		}
		return false, nil
	}
}

// structToJSONInternal generates JSON representations of structs.
func structToJSONInternal(w io.Writer, v Value, seenptr map[uintptr]bool) error {
	if _, err := io.WriteString(w, "{"); err != nil {
		return err
	}
	typ := v.Type()
	firstWrite := true
	for i := 0; i < v.NumField(); i++ {
		fld := typ.Field(i)
		if fld.PkgPath == "" {
			if !firstWrite {
				if _, err := io.WriteString(w, ", "); err != nil {
					return err
				}
			}

			// write the field name
			if _, err := io.WriteString(w, "\""); err != nil {
				return err
			}
			if _, err := io.WriteString(w, lowercaseFirstCharacter(fld.Name)); err != nil {
				return err
			}
			if _, err := io.WriteString(w, "\": "); err != nil {
				return err
			}

			// write the value
			if _, err := objToJSONInternal(w, v.Field(i), seenptr); err != nil {
				return err
			}

			firstWrite = false
		}
	}
	_, err := io.WriteString(w, "}")
	return err
}

// mapToJSONInternal generates JSON representations of maps.
func mapToJSONInternal(w io.Writer, v Value, seenptr map[uintptr]bool) error {
	if _, err := io.WriteString(w, "{"); err != nil {
		return err
	}
	for i, key := range v.MapKeys() {
		if i > 0 {
			if _, err := io.WriteString(w, ", "); err != nil {
				return err
			}
		}

		// write the key
		var outerQuotes bool
		var err error
		var buf bytes.Buffer
		if outerQuotes, err = objToJSONInternal(&buf, key, seenptr); err != nil {
			return err
		}
		keyStr := buf.String()
		if !outerQuotes {
			keyStr = strconv.Quote(keyStr)
		}
		if _, err := io.WriteString(w, keyStr); err != nil {
			return err
		}
		if _, err := io.WriteString(w, ": "); err != nil {
			return err
		}

		// write the value
		if _, err := objToJSONInternal(w, v.MapIndex(key), seenptr); err != nil {
			return err
		}
	}
	_, err := io.WriteString(w, "}")
	return err
}

// listToJSONInternal generates JSON representations of arrays and slices.
func listToJSONInternal(w io.Writer, v Value, seenptr map[uintptr]bool) error {
	if _, err := io.WriteString(w, "["); err != nil {
		return err
	}

	for i := 0; i < v.Len(); i++ {
		if i > 0 {
			if _, err := io.WriteString(w, ", "); err != nil {
				return err
			}
		}

		// write the element
		if _, err := objToJSONInternal(w, v.Index(i), seenptr); err != nil {
			return err
		}
	}
	_, err := io.WriteString(w, "]")
	return err
}
