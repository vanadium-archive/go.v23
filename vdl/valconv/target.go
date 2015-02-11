// Package valconv provides conversion routines between vdl.Value and
// reflect.Value, as well as a generic conversion target interface.
package valconv

import (
	"fmt"
	"reflect"

	"v.io/core/veyron2/vdl"
	"v.io/core/veyron2/verror"
)

// Target represents a generic conversion target; objects that implement this
// interface may be used as the target of a value conversion.  E.g.
// ReflectTarget and ValueTarget create targets based on an underlying
// reflect.Value or vdl.Value respectively.  Other implementations of this
// interface include vom encoding targets, that produce binary and JSON vom
// encodings.
type Target interface {
	// FromBool converts from the src bool to the target, where tt represents the
	// concrete type of bool.
	FromBool(src bool, tt *vdl.Type) error
	// FromUint converts from the src uint to the target, where tt represents the
	// concrete type of uint.
	FromUint(src uint64, tt *vdl.Type) error
	// FromInt converts from the src int to the target, where tt represents the
	// concrete type of int.
	FromInt(src int64, tt *vdl.Type) error
	// FromFloat converts from the src float to the target, where tt represents
	// the concrete type of float.
	FromFloat(src float64, tt *vdl.Type) error
	// FromComplex converts from the src complex to the target, where tt
	// represents the concrete type of complex.
	FromComplex(src complex128, tt *vdl.Type) error
	// FromBytes converts from the src bytes to the target, where tt represents
	// the concrete type of bytes.
	FromBytes(src []byte, tt *vdl.Type) error
	// FromString converts from the src string to the target, where tt represents
	// the concrete type of string.
	FromString(src string, tt *vdl.Type) error
	// FromEnumLabel converts from the src enum label to the target, where tt
	// represents the concrete type of enum.
	FromEnumLabel(src string, tt *vdl.Type) error
	// FromTypeObject converts from the src type to the target.
	FromTypeObject(src *vdl.Type) error
	// FromNil converts from a nil (nonexistent) value of type tt, where tt must
	// be of kind Optional or Any.
	FromNil(tt *vdl.Type) error

	// StartList prepares conversion from a list or array of type tt, with the
	// given len.  FinishList must be called to finish the list.
	StartList(tt *vdl.Type, len int) (ListTarget, error)
	// FinishList finishes a prior StartList call.
	FinishList(x ListTarget) error

	// StartSet prepares conversion from a set of type tt, with the given len.
	// FinishSet must be called to finish the set.
	StartSet(tt *vdl.Type, len int) (SetTarget, error)
	// FinishSet finishes a prior StartSet call.
	FinishSet(x SetTarget) error

	// StartMap prepares conversion from a map of type tt, with the given len.
	// FinishMap must be called to finish the map.
	StartMap(tt *vdl.Type, len int) (MapTarget, error)
	// FinishMap finishes a prior StartMap call.
	FinishMap(x MapTarget) error

	// StartFields prepares conversion from a struct or union of type tt.
	// FinishFields must be called to finish the fields.
	StartFields(tt *vdl.Type) (FieldsTarget, error)
	// FinishFields finishes a prior StartFields call.
	FinishFields(x FieldsTarget) error
}

// ListTarget represents conversion from a list or array.
type ListTarget interface {
	// StartElem prepares conversion of the next list elem.  The given index must
	// start at 0, and be incremented by one by each successive StartElem call.
	// FinishElem must be called to finish the elem.
	//
	// TODO(toddw): Remove index?
	StartElem(index int) (elem Target, _ error)
	// FinishElem finishes a prior StartElem call.
	FinishElem(elem Target) error
}

// SetTarget represents conversion from a set.
type SetTarget interface {
	// StartKey prepares conversion of the next set key.  FinishKey must be called
	// to finish the key.
	StartKey() (key Target, _ error)
	// FinishKey finishes a prior StartKey call.  ErrFieldNoExist indicates the
	// key doesn't exist on the target.
	FinishKey(key Target) error
}

// MapTarget represents conversion from a map.
type MapTarget interface {
	// StartKey prepares conversion of the next map key.  FinishKeyStartField must
	// be called to finish the key.
	StartKey() (key Target, _ error)
	// FinishKeyStartField finishes a prior StartKey call, and starts the
	// associated field.  ErrFieldNoExist indicates the key doesn't exist on the
	// target.
	FinishKeyStartField(key Target) (field Target, _ error)
	// FinishField finishes a prior FinishKeyStartField call.
	FinishField(key, field Target) error
}

// FieldsTarget represents conversion from struct or union fields.
type FieldsTarget interface {
	// StartField prepares conversion of the field with the given name.
	// FinishField must be called to finish the field.  ErrFieldNoExist indicates
	// the field name doesn't exist on the target.
	StartField(name string) (key, field Target, _ error)
	// FinishField finishes a prior StartField call.
	FinishField(key, field Target) error
}

// Convert converts from src to target - it is a helper for calling
// ReflectTarget(reflect.ValueOf(target)).FromReflect(reflect.ValueOf(src)).
func Convert(target, src interface{}) error {
	rtarget, err := ReflectTarget(reflect.ValueOf(target))
	if err != nil {
		return err
	}
	return FromReflect(rtarget, reflect.ValueOf(src))
}

// FromReflect converts from rv to the target, by walking through rv and calling
// the appropriate methods on the target.
func FromReflect(target Target, rv reflect.Value) error {
	// Special-case to treat interface{}(nil) as any(nil).
	if !rv.IsValid() {
		return target.FromNil(vdl.AnyType)
	}
	// Flatten pointers and interfaces in rv, and handle special-cases.  We track
	// whether the final flattened value had any pointers via hasPtr, in order to
	// track optional types correctly.
	hasPtr := false
	for rv.Kind() == reflect.Ptr || rv.Kind() == reflect.Interface {
		// Handle marshaling from native type to wire type.
		switch rvWire, err := vdl.WireValueFromNative(rv); {
		case err != nil:
			return err
		case rvWire.IsValid():
			if hasPtr {
				return FromReflect(target, rvWire.Addr())
			}
			return FromReflect(target, rvWire)
		}
		hasPtr = rv.Kind() == reflect.Ptr
		switch rt := rv.Type(); {
		case rv.IsNil():
			tt, err := vdl.TypeFromReflect(rv.Type())
			if err != nil {
				return err
			}
			switch {
			case tt.Kind() == vdl.TypeObject:
				// Treat nil *vdl.Type as vdl.AnyType.
				return target.FromTypeObject(vdl.AnyType)
			case tt.Kind() == vdl.Union && rt.Kind() == reflect.Interface:
				// Treat nil Union interface as the value of the type at index 0.
				return FromValue(target, vdl.ZeroValue(tt))
			}
			return target.FromNil(tt)
		case rt.ConvertibleTo(rtPtrToType):
			// If rv is convertible to *vdl.Type, fill from it directly.
			return target.FromTypeObject(rv.Convert(rtPtrToType).Interface().(*vdl.Type))
		case rt.ConvertibleTo(rtPtrToValue):
			// If rv is convertible to *vdl.Value, fill from it directly.
			return FromValue(target, rv.Convert(rtPtrToValue).Interface().(*vdl.Value))
		case rt.ConvertibleTo(rtError):
			return fromError(target, rv.Interface().(error), rt)
		}
		rv = rv.Elem()
	}
	// Handle marshaling from native type to wire type.
	switch rvWire, err := vdl.WireValueFromNative(rv); {
	case err != nil:
		return err
	case rvWire.IsValid():
		if hasPtr {
			return FromReflect(target, rvWire.Addr())
		}
		return FromReflect(target, rvWire)
	}
	// Initialize type information.  The optionality is a bit tricky.  Both rt and
	// rv refer to the flattened value, with no pointers or interfaces.  But tt
	// refers to the optional type, iff the original value had a pointer.  This is
	// required so that each of the Target.From* methods can get full type
	// information, including optionality.
	rt := rv.Type()
	if rt.ConvertibleTo(rtError) {
		return fromError(target, rv.Interface().(error), rt)
	}
	tt, err := vdl.TypeFromReflect(rv.Type())
	if err != nil {
		return err
	}
	ttFrom := tt
	if tt.CanBeOptional() && hasPtr {
		ttFrom = vdl.OptionalType(tt)
	}
	// Recursive walk through the reflect value to fill in target.
	//
	// First handle special-cases enum and union.  Note that vdl.TypeFromReflect
	// has already validated the methods, so we can call without error checking.
	switch tt.Kind() {
	case vdl.Enum:
		label := rv.MethodByName("String").Call(nil)[0].String()
		return target.FromEnumLabel(label, ttFrom)
	case vdl.Union:
		// We're guaranteed rv is the concrete field struct.
		name := rv.MethodByName("Name").Call(nil)[0].String()
		fieldsTarget, err := target.StartFields(ttFrom)
		if err != nil {
			return err
		}
		key, field, err := fieldsTarget.StartField(name)
		if err != nil {
			return err // no ErrFieldNoExist special-case; union field is required
		}
		// Grab the "Value" field of the concrete field struct.
		rvFieldValue := rv.Field(0)
		if err := FromReflect(field, rvFieldValue); err != nil {
			return err
		}
		if err := fieldsTarget.FinishField(key, field); err != nil {
			return err
		}
		return target.FinishFields(fieldsTarget)
	}
	// Now handle special-case bytes.
	if isRTBytes(rt) {
		return target.FromBytes(rtBytes(rv), ttFrom)
	}
	// Handle standard kinds.
	switch rt.Kind() {
	case reflect.Bool:
		return target.FromBool(rv.Bool(), ttFrom)
	case reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uint, reflect.Uintptr:
		return target.FromUint(rv.Uint(), ttFrom)
	case reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Int:
		return target.FromInt(rv.Int(), ttFrom)
	case reflect.Float32, reflect.Float64:
		return target.FromFloat(rv.Float(), ttFrom)
	case reflect.Complex64, reflect.Complex128:
		return target.FromComplex(rv.Complex(), ttFrom)
	case reflect.String:
		return target.FromString(rv.String(), ttFrom)
	case reflect.Array, reflect.Slice:
		listTarget, err := target.StartList(ttFrom, rv.Len())
		if err != nil {
			return err
		}
		for ix := 0; ix < rv.Len(); ix++ {
			elem, err := listTarget.StartElem(ix)
			if err != nil {
				return err
			}
			if err := FromReflect(elem, rv.Index(ix)); err != nil {
				return err
			}
			if err := listTarget.FinishElem(elem); err != nil {
				return err
			}
		}
		return target.FinishList(listTarget)
	case reflect.Map:
		if tt.Kind() == vdl.Set {
			setTarget, err := target.StartSet(ttFrom, rv.Len())
			if err != nil {
				return err
			}
			for _, rvkey := range rv.MapKeys() {
				key, err := setTarget.StartKey()
				if err != nil {
					return err
				}
				if err := FromReflect(key, rvkey); err != nil {
					return err
				}
				switch err := setTarget.FinishKey(key); {
				case err == ErrFieldNoExist:
					continue // silently drop unknown fields
				case err != nil:
					return err
				}
			}
			return target.FinishSet(setTarget)
		}
		mapTarget, err := target.StartMap(ttFrom, rv.Len())
		if err != nil {
			return err
		}
		for _, rvkey := range rv.MapKeys() {
			key, err := mapTarget.StartKey()
			if err != nil {
				return err
			}
			if err := FromReflect(key, rvkey); err != nil {
				return err
			}
			field, err := mapTarget.FinishKeyStartField(key)
			switch {
			case err == ErrFieldNoExist:
				continue // silently drop unknown fields
			case err != nil:
				return err
			}
			if err := FromReflect(field, rv.MapIndex(rvkey)); err != nil {
				return err
			}
			if err := mapTarget.FinishField(key, field); err != nil {
				return err
			}
		}
		return target.FinishMap(mapTarget)
	case reflect.Struct:
		fieldsTarget, err := target.StartFields(ttFrom)
		if err != nil {
			return err
		}
		// Loop through tt fields rather than rt fields, since the VDL type tt might
		// have ignored some of the fields in rt, e.g. unexported fields.
		for fx := 0; fx < tt.NumField(); fx++ {
			name := tt.Field(fx).Name
			key, field, err := fieldsTarget.StartField(name)
			switch {
			case err == ErrFieldNoExist:
				continue // silently drop unknown fields
			case err != nil:
				return err
			}
			rvField := rv.FieldByName(name)
			if !rvField.IsValid() {
				// This case occurs if the VDL type tt has a field name that doesn't
				// eixst in the Go reflect.Type rt.  This should never occur; it
				// indicates a bug in the vdl.TypeOf logic.  Panic here to give us a
				// stack trace to make it easier to debug.
				panic(fmt.Errorf("missing struct field %q, tt: %v, rt: %v", name, tt, rt))
			}
			if err := FromReflect(field, rvField); err != nil {
				return err
			}
			if err := fieldsTarget.FinishField(key, field); err != nil {
				return err
			}
		}
		return target.FinishFields(fieldsTarget)
	default:
		return fmt.Errorf("FromReflect invalid type %v", rt)
	}
}

// fromError handles all rv values that implement the error interface.
// TODO(toddw): box our interface values to constrain this?
func fromError(target Target, err error, rt reflect.Type) error {
	// Convert from err into verror, and then into the vdl.Value representation,
	// and then fill the target.  We can't just call FromReflect since that'll
	// cause a recursive loop.
	v, err := vdlValueFromError(err)
	if err != nil {
		return err
	}
	// The rt arg is the original type of err, and we know that rt is convertible
	// to error.  If rt is a non-pointer type, or if only the pointer type is
	// convertible to error, we convert from the non-pointer error struct.  This
	// ensures that if target is interface{}, we'll create a verror.Standard with
	// the right optionality.
	if rt.Kind() != reflect.Ptr ||
		(rt.Kind() == reflect.Ptr && !rt.Elem().ConvertibleTo(rtError)) {
		v = v.Elem()
	}
	return FromValue(target, v)
}

func vdlValueFromError(err error) (*vdl.Value, error) {
	e := verror.ExplicitConvert(verror.Unknown, "", "", "", err)
	// The verror types are defined like this:
	//
	// type ID         string
	// type ActionCode uint32
	// type IDAction struct {
	//   ID     verror.ID
	//   Action ActionCode
	// }
	// type Standard struct {
	//   IDAction  IDAction
	//   Msg       string
	//   ParamList []interface{}
	// }
	verr := vdl.NonNilZeroValue(vdl.ErrorType)
	vv := verr.Elem()
	vv.Field(0).Field(0).AssignString(string(verror.ErrorID(e)))
	vv.Field(0).Field(1).AssignUint(uint64(verror.Action(e)))
	vv.Field(1).AssignString(e.Error())
	vv.Field(2).AssignLen(len(verror.Params(e)))
	for ix, p := range verror.Params(e) {
		var pVDL *vdl.Value
		if err := Convert(&pVDL, p); err != nil {
			return nil, err
		}
		vv.Field(2).Index(ix).Assign(pVDL)
	}
	return verr, nil
}

// FromValue converts from vv to the target, by walking through vv and calling
// the appropriate methods on the target.
func FromValue(target Target, vv *vdl.Value) error {
	tt := vv.Type()
	if tt.Kind() == vdl.Any {
		if vv.IsNil() {
			return target.FromNil(tt)
		}
		// Non-nil any simply converts from the elem.
		vv = vv.Elem()
		tt = vv.Type()
	}
	if tt.Kind() == vdl.Optional {
		if vv.IsNil() {
			return target.FromNil(tt)
		}
		// Non-nil optional is special - we keep tt as the optional type, but use
		// the elem value for the actual value below.
		vv = vv.Elem()
	}
	if vv.Type().IsBytes() {
		return target.FromBytes(vv.Bytes(), tt)
	}
	switch vv.Kind() {
	case vdl.Bool:
		return target.FromBool(vv.Bool(), tt)
	case vdl.Byte:
		return target.FromUint(uint64(vv.Byte()), tt)
	case vdl.Uint16, vdl.Uint32, vdl.Uint64:
		return target.FromUint(vv.Uint(), tt)
	case vdl.Int16, vdl.Int32, vdl.Int64:
		return target.FromInt(vv.Int(), tt)
	case vdl.Float32, vdl.Float64:
		return target.FromFloat(vv.Float(), tt)
	case vdl.Complex64, vdl.Complex128:
		return target.FromComplex(vv.Complex(), tt)
	case vdl.String:
		return target.FromString(vv.RawString(), tt)
	case vdl.Enum:
		return target.FromEnumLabel(vv.EnumLabel(), tt)
	case vdl.TypeObject:
		return target.FromTypeObject(vv.TypeObject())
	case vdl.Array, vdl.List:
		listTarget, err := target.StartList(tt, vv.Len())
		if err != nil {
			return err
		}
		for ix := 0; ix < vv.Len(); ix++ {
			elem, err := listTarget.StartElem(ix)
			if err != nil {
				return err
			}
			if err := FromValue(elem, vv.Index(ix)); err != nil {
				return err
			}
			if err := listTarget.FinishElem(elem); err != nil {
				return err
			}
		}
		return target.FinishList(listTarget)
	case vdl.Set:
		setTarget, err := target.StartSet(tt, vv.Len())
		if err != nil {
			return err
		}
		for _, vvkey := range vv.Keys() {
			key, err := setTarget.StartKey()
			if err != nil {
				return err
			}
			if err := FromValue(key, vvkey); err != nil {
				return err
			}
			switch err := setTarget.FinishKey(key); {
			case err == ErrFieldNoExist:
				continue // silently drop unknown fields
			case err != nil:
				return err
			}
		}
		return target.FinishSet(setTarget)
	case vdl.Map:
		mapTarget, err := target.StartMap(tt, vv.Len())
		if err != nil {
			return err
		}
		for _, vvkey := range vv.Keys() {
			key, err := mapTarget.StartKey()
			if err != nil {
				return err
			}
			if err := FromValue(key, vvkey); err != nil {
				return err
			}
			field, err := mapTarget.FinishKeyStartField(key)
			switch {
			case err == ErrFieldNoExist:
				continue // silently drop unknown fields
			case err != nil:
				return err
			}
			if err := FromValue(field, vv.MapIndex(vvkey)); err != nil {
				return err
			}
			if err := mapTarget.FinishField(key, field); err != nil {
				return err
			}
		}
		return target.FinishMap(mapTarget)
	case vdl.Struct:
		fieldsTarget, err := target.StartFields(tt)
		if err != nil {
			return err
		}
		for fx := 0; fx < vv.Type().NumField(); fx++ {
			key, field, err := fieldsTarget.StartField(vv.Type().Field(fx).Name)
			switch {
			case err == ErrFieldNoExist:
				continue // silently drop unknown fields
			case err != nil:
				return err
			}
			if err := FromValue(field, vv.Field(fx)); err != nil {
				return err
			}
			if err := fieldsTarget.FinishField(key, field); err != nil {
				return err
			}
		}
		return target.FinishFields(fieldsTarget)
	case vdl.Union:
		fieldsTarget, err := target.StartFields(tt)
		if err != nil {
			return err
		}
		fx, vvFieldValue := vv.UnionField()
		key, field, err := fieldsTarget.StartField(vv.Type().Field(fx).Name)
		if err != nil {
			return err // no ErrFieldNoExist special-case; union field is required
		}
		if err := FromValue(field, vvFieldValue); err != nil {
			return err
		}
		if err := fieldsTarget.FinishField(key, field); err != nil {
			return err
		}
		return target.FinishFields(fieldsTarget)
	default:
		panic(fmt.Errorf("FromValue unhandled %v %v", vv.Kind(), tt))
	}
}
