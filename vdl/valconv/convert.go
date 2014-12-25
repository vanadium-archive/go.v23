package valconv

// Naming conventions to distinguish this package from reflect:
//   tt - refers to *vdl.Type
//   vv - refers to *vdl.Value
//   rt - refers to reflect.Type
//   rv - refers to reflect.Value

import (
	"fmt"
	"reflect"

	"v.io/veyron/veyron2/vdl"
	"v.io/veyron/veyron2/verror"
	"v.io/veyron/veyron2/verror2"
)

var (
	errTargetInvalid    = verror.BadArgf("invalid target")
	errTargetUnsettable = verror.BadArgf("unsettable target")
	errArrayIndex       = verror.BadArgf("array index out of range")
	errFieldNotFound    = verror.NoExistf("struct field not found")
)

var (
	rtByte            = reflect.TypeOf(byte(0))
	rtBool            = reflect.TypeOf(bool(false))
	rtString          = reflect.TypeOf(string(""))
	rtType            = reflect.TypeOf(vdl.Type{})
	rtValue           = reflect.TypeOf(vdl.Value{})
	rtPtrToType       = reflect.PtrTo(rtType)
	rtPtrToValue      = reflect.PtrTo(rtValue)
	rtError           = reflect.TypeOf((*error)(nil)).Elem()
	rtVError2Standard = reflect.TypeOf(verror2.Standard{})
)

// convTarget represents the state and logic for value conversion.
//
// TODO(toddw): Split convTarget apart into reflectTarget and valueTarget.
type convTarget struct {
	// Conceptually this is a disjoint union between the two types of destination
	// values: *vdl.Value and reflect.Value.  Only one of the vv and rv fields is
	// ever valid.  We split into two fields and perform "if vv == nil" checks in
	// each method rather than using an interface for performance reasons.
	//
	// The tt field is always non-nil, and represents the type of the value.
	tt *vdl.Type
	vv *vdl.Value
	rv reflect.Value
}

// ReflectTarget returns a conversion Target based on the given reflect.Value.
// Some rules depending on the type of rv:
//   o If rv is a valid *vdl.Value, it is filled in directly.
//   o Otherwise rv must be a settable value (i.e. it must be a pointer).
//   o Pointers are followed, and logically "flattened".
//   o New values are automatically created for nil pointers.
//   o Targets convertible to *vdl.Type and *vdl.Value are filled in directly.
func ReflectTarget(rv reflect.Value) (Target, error) {
	if !rv.IsValid() {
		return nil, errTargetInvalid
	}
	if vv := extractValue(rv); vv.IsValid() {
		return valueConv(vv), nil
	}
	if !rv.CanSet() && rv.Kind() == reflect.Ptr && !rv.IsNil() {
		// Dereference the pointer a single time to make rv settable.
		rv = rv.Elem()
	}
	tt, err := vdl.TypeFromReflect(rv.Type())
	if err != nil {
		return nil, err
	}
	target, err := reflectConv(rv, tt)
	if err != nil {
		return nil, err
	}
	return target, nil
}

// ValueTarget returns a conversion Target based on the given vdl.Value.
// Returns an error if the given vdl.Value isn't valid.
func ValueTarget(vv *vdl.Value) (Target, error) {
	if !vv.IsValid() {
		return nil, errTargetInvalid
	}
	return valueConv(vv), nil
}

func reflectConv(rv reflect.Value, tt *vdl.Type) (convTarget, error) {
	if !rv.IsValid() {
		return convTarget{}, errTargetInvalid
	}
	if vv := extractValue(rv); vv.IsValid() {
		return valueConv(vv), nil
	}
	if !rv.CanSet() {
		return convTarget{}, errTargetUnsettable
	}
	return convTarget{tt: tt, rv: rv}, nil
}

func valueConv(vv *vdl.Value) convTarget {
	return convTarget{tt: vv.Type(), vv: vv}
}

func extractValue(rv reflect.Value) *vdl.Value {
	for rv.Kind() == reflect.Ptr && !rv.IsNil() {
		if rv.Type().ConvertibleTo(rtPtrToValue) {
			return rv.Convert(rtPtrToValue).Interface().(*vdl.Value)
		}
		rv = rv.Elem()
	}
	return nil
}

// startConvert prepares to fill in c, converting from type ttFrom.  Returns fin
// and fill which are used by finishConvert to finish the conversion; fin
// represents the final target to assign to, and fill represents the target to
// actually fill in.
func startConvert(c convTarget, ttFrom *vdl.Type) (fin, fill convTarget, err error) {
	if ttFrom.Kind() == vdl.Any {
		return convTarget{}, convTarget{}, fmt.Errorf("can't convert from type %q - either call target.FromNil or target.From* for the element value", ttFrom)
	}
	if !compatible(c.tt, ttFrom) {
		return convTarget{}, convTarget{}, fmt.Errorf("types %q and %q aren't compatible", c.tt, ttFrom)
	}
	fin = createFinTarget(c, ttFrom)
	fill, err = createFillTarget(fin, ttFrom)
	if err != nil {
		return convTarget{}, convTarget{}, err
	}
	return fin, fill, nil
}

// setZeroVDLValue checks whether rv is convertible to *vdl.Value, and if so,
// sets rv to the zero value of ttFrom, returning the resulting value.
func setZeroVDLValue(rv reflect.Value, ttFrom *vdl.Type) *vdl.Value {
	if rv.Type().ConvertibleTo(rtPtrToValue) {
		vv := rv.Convert(rtPtrToValue).Interface().(*vdl.Value)
		if !vv.IsValid() {
			vv = vdl.ZeroValue(ttFrom)
			rv.Set(reflect.ValueOf(vv).Convert(rv.Type()))
		}
		return vv
	}
	return nil
}

// createFinTarget returns the fin target for the conversion, flattening
// pointers and creating new non-nil values as necessary.
func createFinTarget(c convTarget, ttFrom *vdl.Type) convTarget {
	if c.vv == nil {
		// Create new pointers down to the final non-pointer value.
		for c.rv.Kind() == reflect.Ptr {
			// Handle case where rv is *vdl.Value; fill it in directly.
			if vv := setZeroVDLValue(c.rv, ttFrom); vv.IsValid() {
				if vv.Kind() == vdl.Optional {
					vv.Assign(vdl.NonNilZeroValue(vv.Type()))
					return convTarget{tt: ttFrom, vv: vv.Elem()}
				}
				return convTarget{tt: ttFrom, vv: vv}
			}
			// Set nil pointers to new pointers.
			if c.rv.IsNil() {
				c.rv.Set(reflect.New(c.rv.Type().Elem()))
			}
			// Stop at *vdl.Type to allow TypeObject values to be assigned.
			if c.rv.Type().Elem() == rtType {
				return c
			}
			c.rv = c.rv.Elem()
		}
		if c.tt.Kind() == vdl.Optional {
			c.tt = c.tt.Elem() // flatten c.tt to match c.rv
		}
	} else {
		// Create a non-nil value for optional types.
		if c.tt.Kind() == vdl.Optional {
			c.vv.Assign(vdl.NonNilZeroValue(c.tt))
			c.tt, c.vv = c.tt.Elem(), c.vv.Elem()
		}
	}
	return c
}

// createFillTarget returns the fill target for the conversion, creating new
// values of the appropriate type if necessary.  The fill target must be a
// concrete type; if fin has type interface/any, we'll create a concrete type,
// and finishConvert will assign fin from the fill target.  The type we choose
// to create is either a special-case (error or union), or based on the ttFrom
// type we're converting *from*.
//
// If fin is already a concrete type it's used directly as the fill target.
func createFillTarget(fin convTarget, ttFrom *vdl.Type) (convTarget, error) {
	if fin.vv == nil {
		// Handle case where fin.rv is a native type; return the wire type as the
		// fill target, and rely on finishConvert to perform the final step of
		// converting from the wire type to the native type.
		//
		// TODO(toddw): This doesn't handle pointer native types.
		if ri := vdl.ReflectInfoFromNative(fin.rv.Type()); ri != nil {
			tt, err := vdl.TypeFromReflect(ri.WireType)
			if err != nil {
				return convTarget{}, err
			}
			return reflectConv(reflect.New(ri.WireType).Elem(), tt)
		}
		if fin.rv.Kind() == reflect.Interface {
			// We're converting into an interface, so we can't just fill into the fin
			// target directly.  Return the appropriate fill value.
			switch {
			case fin.rv.Type() == rtError || ttFrom == vdl.ErrorType:
				// Create the standard verror struct to fill in, with the pointer.
				return reflectConv(reflect.New(rtVError2Standard).Elem(), vdl.ErrorType)
			case ttFrom == vdl.ErrorType.Elem():
				// Create the standard verror struct to fill in, without the pointer.
				return reflectConv(reflect.New(rtVError2Standard).Elem(), vdl.ErrorType.Elem())
			case fin.tt.Kind() == vdl.Union:
				// The fin target is a union interface.  Since we don't know the union
				// field name yet, we don't know which concrete type to use.  So we
				// cheat and use *vdl.Value, so that we don't need to choose the type.
				//
				// TODO(toddw): This is probably very slow.  We should benchmark and
				// probably re-design our conversion strategy.
				return valueConv(vdl.ZeroValue(fin.tt)), nil
			case fin.tt.Kind() != vdl.Any:
				return convTarget{}, fmt.Errorf("internal error - cannot convert to Go type %v vdl type %v from %v", fin.rv.Type(), fin.tt, ttFrom)
			}
			// We're converting into any, and ttFrom is the type we're converting
			// *from*.  First handle the special-case *vdl.Type.
			if ttFrom.Kind() == vdl.TypeObject {
				return reflectConv(reflect.New(rtPtrToType).Elem(), vdl.TypeObjectType)
			}
			// Try to create a reflect.Type out of ttFrom, and if it exists, create a real
			// object to fill in.
			if rt := vdl.ReflectFromType(ttFrom); rt != nil {
				tt, err := vdl.TypeFromReflect(rt)
				if err != nil {
					return convTarget{}, err
				}
				rv := reflect.New(rt).Elem()
				for rv.Kind() == reflect.Ptr {
					rv.Set(reflect.New(rv.Type().Elem()))
					rv = rv.Elem()
				}
				return reflectConv(rv, tt)
			}
			// We don't have the type name in our registry, so create a concrete
			// *vdl.Value to fill in, based on ttFrom.
			if ttFrom.Kind() == vdl.Optional {
				return convTarget{tt: ttFrom, vv: vdl.ZeroValue(ttFrom.Elem())}, nil
			}
			return convTarget{tt: ttFrom, vv: vdl.ZeroValue(ttFrom)}, nil
		}
		// We're not converting into an interface, so we can fill into the fin
		// target directly.  Assign an appropriate zero value.
		fin.rv.Set(rvSettableZeroValue(fin.rv.Type(), ttFrom))
	} else {
		switch fin.vv.Kind() {
		case vdl.Any:
			if ttFrom.Kind() == vdl.Optional {
				return convTarget{tt: ttFrom, vv: vdl.ZeroValue(ttFrom.Elem())}, nil
			}
			return convTarget{tt: ttFrom, vv: vdl.ZeroValue(ttFrom)}, nil
		default:
			fin.vv.Assign(nil) // start with zero item
		}
	}
	return fin, nil
}

// finishConvert finishes converting a value, taking the fin and fill returned
// by startConvert.  This is necessary since interface/any values are assigned
// by value, and can't be filled in by reference.
func finishConvert(fin, fill convTarget) error {
	// The logic here mirrors the logic in createFillTarget.
	if fin.vv == nil {
		// Handle mirrored case in createFillTarget where fin.rv is a native type;
		// fill.rv is the wire type that has been filled in.
		if ri := vdl.ReflectInfoFromNative(fin.rv.Type()); ri != nil {
			ierr := ri.ToNativeFunc.Call([]reflect.Value{fill.rv, fin.rv.Addr()})[0].Interface()
			if ierr != nil {
				return ierr.(error)
			}
			return nil
		}
		switch fin.rv.Kind() {
		case reflect.Interface:
			// The fill value may be set to either rv or vv in startConvert above.
			var rvFill reflect.Value
			if fill.vv == nil {
				rvFill = fill.rv
				if ri := vdl.ReflectInfoFromWire(fill.rv.Type()); ri != nil && ri.NativeType != nil {
					// Handle case where fill.rv is a wire type with a native type; set
					// rvFill to a new native type and call ToNative to fill it in.
					newNative := reflect.New(ri.NativeType)
					ierr := ri.ToNativeFunc.Call([]reflect.Value{fill.rv, newNative})[0].Interface()
					if ierr != nil {
						return ierr.(error)
					}
					rvFill = newNative.Elem()
				}
				if fill.tt.Kind() == vdl.Optional {
					rvFill = rvFill.Addr()
				}
			} else {
				switch fin.tt.Kind() {
				case vdl.Union:
					// Special-case: the fin target is a union interface.  This is a bit
					// weird; since we didn't know the union field name yet, we created a
					// *vdl.Value as the fill target in createFillTarget above.  But now
					// that it's filled in, we convert it back into the appropriate
					// concrete field struct, so that it can be used normally.
					var err error
					if rvFill, err = makeReflectUnion(fin.rv, fill.vv); err != nil {
						return err
					}
				default:
					if fill.tt.Kind() == vdl.Optional {
						rvFill = reflect.ValueOf(vdl.OptionalValue(fill.vv))
					} else {
						rvFill = reflect.ValueOf(fill.vv)
					}
				}
			}
			if to, from := fin.rv.Type(), rvFill.Type(); !from.AssignableTo(to) {
				return fmt.Errorf("%v not assignable from %v", to, from)
			}
			fin.rv.Set(rvFill)
		}
	} else {
		switch fin.vv.Kind() {
		case vdl.Any:
			if fill.tt.Kind() == vdl.Optional {
				fin.vv.Assign(vdl.OptionalValue(fill.vv))
			} else {
				fin.vv.Assign(fill.vv)
			}
		}
	}
	return nil
}

// rvSettableZeroValue returns a settable zero value corresponding to rt / tt.
// This isn't trivial since VDL and Go define zero values slightly differently.
// In particular in VDL:
//    TypeObject: AnyType
//    Union:      zero value of the type at index 0
// These are translated into Go as follows, with their standard Go zero values:
//    *vdl.Type: nil
//    interface: nil
// Thus we must special-case values of these types.
//
// TODO(toddw): This logic is recursive and complicated because our convTarget
// methods don't take the convTarget as a pointer receiver, so we can't mutate
// the target as we decode.  When we split convTarget into separate valueTarget
// and reflectTarget objects, we should also take the target as a pointer
// receiver.  That'll simplify this logic.
func rvSettableZeroValue(rt reflect.Type, tt *vdl.Type) reflect.Value {
	rv := reflect.New(rt).Elem()
	// Easy fastpath; if the type doesn't contain inline typeobject or union, the
	// regular Go zero value is good enough.
	if !tt.ContainsKind(vdl.WalkInline, vdl.TypeObject, vdl.Union) {
		return rv
	}
	// Handle typeobject, which has the zero value of AnyType.
	if rtPtrToType.ConvertibleTo(rt) {
		return reflect.ValueOf(vdl.AnyType).Convert(rt)
	}
	// Handle composite types with inline subtypes.
	switch {
	case tt.Kind() == vdl.Union:
		if rt.Kind() == reflect.Struct {
			// Union struct, which represents a single field.
			rv.Field(0).Set(rvSettableZeroValue(rt.Field(0).Type, tt.Field(0).Type))
			return rv
		}
		// Union interface, which represents one of the fields.  Initialize with the
		// zero value of the type at index 0.
		ri, err := vdl.DeriveReflectInfo(rt)
		if err != nil {
			panic(fmt.Errorf("valconv: invalid union type rt: %v tt: %v err: %v", rt, tt, err))
		}
		rv.Set(rvSettableZeroValue(ri.UnionFields[0].RepType, tt.Field(0).Type))
		return rv
	case rt.Kind() == reflect.Array:
		for ix := 0; ix < rt.Len(); ix++ {
			rv.Index(ix).Set(rvSettableZeroValue(rt.Elem(), tt.Elem()))
		}
		return rv
	case rt.Kind() == reflect.Struct:
		for ix := 0; ix < rt.NumField(); ix++ {
			rtField := rt.Field(ix)
			ttField, index := tt.FieldByName(rtField.Name)
			if index < 0 {
				// Ignore fields that aren't described in tt; e.g. unexported fields.
				continue
			}
			rv.Field(ix).Set(rvSettableZeroValue(rtField.Type, ttField.Type))
		}
		return rv
	}
	panic(fmt.Errorf("valconv: rvSettableZeroValue unhandled rt: %v tt: %v", rt, tt))
}

// makeReflectUnion returns the union concrete struct based on the union
// interface struct rv, corresponding to the field that is set in vv.  The point
// of this machinery is to ensure union can always be created, without any type
// registration.
func makeReflectUnion(rv reflect.Value, vv *vdl.Value) (reflect.Value, error) {
	// TODO(toddw): Cache the field types for faster access, after merging this
	// into the vdl package.
	ri, err := vdl.DeriveReflectInfo(rv.Type())
	switch {
	case err != nil:
		return reflect.Value{}, err
	case len(ri.UnionFields) == 0 || vv.Kind() != vdl.Union:
		return reflect.Value{}, fmt.Errorf("vdl: makeReflectUnion(%v, %v) must only be called on Union", rv.Type(), vv.Type())
	}
	index, vvField := vv.UnionField()
	if index >= len(ri.UnionFields) {
		return reflect.Value{}, fmt.Errorf("vdl: makeReflectUnion(%v, %v) field index %d out of range, len=%d", rv.Type(), vv.Type(), index, len(ri.UnionFields))
	}
	// Run our regular conversion from vvField to rvField.  Keep in mind that
	// rvField is the concrete union struct, so we convert into Field(0), which
	// corresponds to the "Value" field.
	rvField := reflect.New(ri.UnionFields[index].RepType).Elem()
	target, err := ReflectTarget(rvField.Field(0))
	if err != nil {
		return reflect.Value{}, err
	}
	if err := FromValue(target, vvField); err != nil {
		return reflect.Value{}, err
	}
	return rvField, nil
}

func removeOptional(tt *vdl.Type) *vdl.Type {
	if tt.Kind() == vdl.Optional {
		tt = tt.Elem()
	}
	return tt
}

// FromNil implements the Target interface method.
func (c convTarget) FromNil(tt *vdl.Type) error {
	if !compatible(c.tt, tt) {
		return fmt.Errorf("types %q and %q aren't compatible", c.tt, tt)
	}
	if !tt.CanBeNil() || !c.tt.CanBeNil() {
		return fmt.Errorf("invalid conversion from %v(nil) to %v", tt, c.tt)
	}
	if c.vv == nil {
		// The strategy is to create new pointers down to either a single pointer,
		// or the final interface, and set that to nil.  We create all the pointers
		// to be consistent with our behavior in the non-nil case, where we
		// similarly create all the pointers.  It also makes the tests simpler.
		for c.rv.Kind() == reflect.Ptr {
			if vv := setZeroVDLValue(c.rv, tt); vv.IsValid() {
				return nil
			}
			if k := c.rv.Type().Elem().Kind(); k != reflect.Ptr && k != reflect.Interface {
				break
			}
			// Next elem is a pointer or interface, keep looping.
			if c.rv.IsNil() {
				c.rv.Set(reflect.New(c.rv.Type().Elem()))
			}
			c.rv = c.rv.Elem()
		}
		// Now rv.Type() is either a single pointer or an interface; either way
		// setting its zero value will give us nil.
		c.rv.Set(reflect.Zero(c.rv.Type()))
	} else {
		vvNil := vdl.ZeroValue(tt)
		if to, from := c.vv.Type(), vvNil; !to.AssignableFrom(from) {
			return fmt.Errorf("%v not assignable from %v", to, from)
		}
		c.vv.Assign(vvNil)
	}
	return nil
}

// FromBool implements the Target interface method.
func (c convTarget) FromBool(src bool, tt *vdl.Type) error {
	fin, fill, err := startConvert(c, tt)
	if err != nil {
		return err
	}
	if err := fill.fromBool(src); err != nil {
		return err
	}
	return finishConvert(fin, fill)
}

// FromUint implements the Target interface method.
func (c convTarget) FromUint(src uint64, tt *vdl.Type) error {
	fin, fill, err := startConvert(c, tt)
	if err != nil {
		return err
	}
	if err := fill.fromUint(src); err != nil {
		return err
	}
	return finishConvert(fin, fill)
}

// FromInt implements the Target interface method.
func (c convTarget) FromInt(src int64, tt *vdl.Type) error {
	fin, fill, err := startConvert(c, tt)
	if err != nil {
		return err
	}
	if err := fill.fromInt(src); err != nil {
		return err
	}
	return finishConvert(fin, fill)
}

// FromFloat implements the Target interface method.
func (c convTarget) FromFloat(src float64, tt *vdl.Type) error {
	fin, fill, err := startConvert(c, tt)
	if err != nil {
		return err
	}
	if err := fill.fromFloat(src); err != nil {
		return err
	}
	return finishConvert(fin, fill)
}

// FromComplex implements the Target interface method.
func (c convTarget) FromComplex(src complex128, tt *vdl.Type) error {
	fin, fill, err := startConvert(c, tt)
	if err != nil {
		return err
	}
	if err := fill.fromComplex(src); err != nil {
		return err
	}
	return finishConvert(fin, fill)
}

// FromBytes implements the Target interface method.
func (c convTarget) FromBytes(src []byte, tt *vdl.Type) error {
	fin, fill, err := startConvert(c, tt)
	if err != nil {
		return err
	}
	if err := fill.fromBytes(src); err != nil {
		return err
	}
	return finishConvert(fin, fill)
}

// FromString implements the Target interface method.
func (c convTarget) FromString(src string, tt *vdl.Type) error {
	// TODO(toddw): Speed this up.
	return c.FromBytes([]byte(src), tt)
}

// FromEnumLabel implements the Target interface method.
func (c convTarget) FromEnumLabel(src string, tt *vdl.Type) error {
	return c.FromString(src, tt)
}

// FromTypeObject implements the Target interface method.
func (c convTarget) FromTypeObject(src *vdl.Type) error {
	fin, fill, err := startConvert(c, vdl.TypeObjectType)
	if err != nil {
		return err
	}
	if err := fill.fromTypeObject(src); err != nil {
		return err
	}
	return finishConvert(fin, fill)
}

func (c convTarget) fromBool(src bool) error {
	if c.vv == nil {
		if c.rv.Kind() == reflect.Bool {
			c.rv.SetBool(src)
			return nil
		}
	} else {
		if c.vv.Kind() == vdl.Bool {
			c.vv.AssignBool(src)
			return nil
		}
	}
	return fmt.Errorf("invalid conversion from bool to %v", c.tt)
}

func (c convTarget) fromUint(src uint64) error {
	if c.vv == nil {
		switch kind := c.rv.Kind(); kind {
		case reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uint, reflect.Uintptr:
			if !overflowUint(src, bitlenR(kind)) {
				c.rv.SetUint(src)
				return nil
			}
		case reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Int:
			if isrc, ok := convertUintToInt(src, bitlenR(kind)); ok {
				c.rv.SetInt(isrc)
				return nil
			}
		case reflect.Float32, reflect.Float64:
			if fsrc, ok := convertUintToFloat(src, bitlenR(kind)); ok {
				c.rv.SetFloat(fsrc)
				return nil
			}
		case reflect.Complex64, reflect.Complex128:
			if fsrc, ok := convertUintToFloat(src, bitlenR(kind)); ok {
				c.rv.SetComplex(complex(fsrc, 0))
				return nil
			}
		}
	} else {
		switch kind := c.vv.Kind(); kind {
		case vdl.Byte:
			if !overflowUint(src, bitlenV(kind)) {
				c.vv.AssignByte(byte(src))
				return nil
			}
		case vdl.Uint16, vdl.Uint32, vdl.Uint64:
			if !overflowUint(src, bitlenV(kind)) {
				c.vv.AssignUint(src)
				return nil
			}
		case vdl.Int16, vdl.Int32, vdl.Int64:
			if isrc, ok := convertUintToInt(src, bitlenV(kind)); ok {
				c.vv.AssignInt(isrc)
				return nil
			}
		case vdl.Float32, vdl.Float64:
			if fsrc, ok := convertUintToFloat(src, bitlenV(kind)); ok {
				c.vv.AssignFloat(fsrc)
				return nil
			}
		case vdl.Complex64, vdl.Complex128:
			if fsrc, ok := convertUintToFloat(src, bitlenV(kind)); ok {
				c.vv.AssignComplex(complex(fsrc, 0))
				return nil
			}
		}
	}
	return fmt.Errorf("invalid conversion from uint(%d) to %v", src, c.tt)
}

func (c convTarget) fromInt(src int64) error {
	if c.vv == nil {
		switch kind := c.rv.Kind(); kind {
		case reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uint, reflect.Uintptr:
			if usrc, ok := convertIntToUint(src, bitlenR(kind)); ok {
				c.rv.SetUint(usrc)
				return nil
			}
		case reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Int:
			if !overflowInt(src, bitlenR(kind)) {
				c.rv.SetInt(src)
				return nil
			}
		case reflect.Float32, reflect.Float64:
			if fsrc, ok := convertIntToFloat(src, bitlenR(kind)); ok {
				c.rv.SetFloat(fsrc)
				return nil
			}
		case reflect.Complex64, reflect.Complex128:
			if fsrc, ok := convertIntToFloat(src, bitlenR(kind)); ok {
				c.rv.SetComplex(complex(fsrc, 0))
				return nil
			}
		}
	} else {
		switch kind := c.vv.Kind(); kind {
		case vdl.Byte:
			if usrc, ok := convertIntToUint(src, bitlenV(kind)); ok {
				c.vv.AssignByte(byte(usrc))
				return nil
			}
		case vdl.Uint16, vdl.Uint32, vdl.Uint64:
			if usrc, ok := convertIntToUint(src, bitlenV(kind)); ok {
				c.vv.AssignUint(usrc)
				return nil
			}
		case vdl.Int16, vdl.Int32, vdl.Int64:
			if !overflowInt(src, bitlenV(kind)) {
				c.vv.AssignInt(src)
				return nil
			}
		case vdl.Float32, vdl.Float64:
			if fsrc, ok := convertIntToFloat(src, bitlenV(kind)); ok {
				c.vv.AssignFloat(fsrc)
				return nil
			}
		case vdl.Complex64, vdl.Complex128:
			if fsrc, ok := convertIntToFloat(src, bitlenV(kind)); ok {
				c.vv.AssignComplex(complex(fsrc, 0))
				return nil
			}
		}
	}
	return fmt.Errorf("invalid conversion from int(%d) to %v", src, c.tt)
}

func (c convTarget) fromFloat(src float64) error {
	if c.vv == nil {
		switch kind := c.rv.Kind(); kind {
		case reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uint, reflect.Uintptr:
			if usrc, ok := convertFloatToUint(src, bitlenR(kind)); ok {
				c.rv.SetUint(usrc)
				return nil
			}
		case reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Int:
			if isrc, ok := convertFloatToInt(src, bitlenR(kind)); ok {
				c.rv.SetInt(isrc)
				return nil
			}
		case reflect.Float32, reflect.Float64:
			c.rv.SetFloat(convertFloatToFloat(src, bitlenR(kind)))
			return nil
		case reflect.Complex64, reflect.Complex128:
			c.rv.SetComplex(complex(convertFloatToFloat(src, bitlenR(kind)), 0))
			return nil
		}
	} else {
		switch kind := c.vv.Kind(); kind {
		case vdl.Byte:
			if usrc, ok := convertFloatToUint(src, bitlenV(kind)); ok {
				c.vv.AssignByte(byte(usrc))
				return nil
			}
		case vdl.Uint16, vdl.Uint32, vdl.Uint64:
			if usrc, ok := convertFloatToUint(src, bitlenV(kind)); ok {
				c.vv.AssignUint(usrc)
				return nil
			}
		case vdl.Int16, vdl.Int32, vdl.Int64:
			if isrc, ok := convertFloatToInt(src, bitlenV(kind)); ok {
				c.vv.AssignInt(isrc)
				return nil
			}
		case vdl.Float32, vdl.Float64:
			c.vv.AssignFloat(convertFloatToFloat(src, bitlenV(kind)))
			return nil
		case vdl.Complex64, vdl.Complex128:
			c.vv.AssignComplex(complex(convertFloatToFloat(src, bitlenV(kind)), 0))
			return nil
		}
	}
	return fmt.Errorf("invalid conversion from float(%g) to %v", src, c.tt)
}

func (c convTarget) fromComplex(src complex128) error {
	if c.vv == nil {
		switch kind := c.rv.Kind(); kind {
		case reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uint, reflect.Uintptr:
			if usrc, ok := convertComplexToUint(src, bitlenR(kind)); ok {
				c.rv.SetUint(usrc)
				return nil
			}
		case reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Int:
			if isrc, ok := convertComplexToInt(src, bitlenR(kind)); ok {
				c.rv.SetInt(isrc)
				return nil
			}
		case reflect.Float32, reflect.Float64:
			if imag(src) == 0 {
				c.rv.SetFloat(convertFloatToFloat(real(src), bitlenR(kind)))
				return nil
			}
		case reflect.Complex64, reflect.Complex128:
			re := convertFloatToFloat(real(src), bitlenR(kind))
			im := convertFloatToFloat(imag(src), bitlenR(kind))
			c.rv.SetComplex(complex(re, im))
			return nil
		}
	} else {
		switch kind := c.vv.Kind(); kind {
		case vdl.Byte:
			if usrc, ok := convertComplexToUint(src, bitlenV(kind)); ok {
				c.vv.AssignByte(byte(usrc))
				return nil
			}
		case vdl.Uint16, vdl.Uint32, vdl.Uint64:
			if usrc, ok := convertComplexToUint(src, bitlenV(kind)); ok {
				c.vv.AssignUint(usrc)
				return nil
			}
		case vdl.Int16, vdl.Int32, vdl.Int64:
			if isrc, ok := convertComplexToInt(src, bitlenV(kind)); ok {
				c.vv.AssignInt(isrc)
				return nil
			}
		case vdl.Float32, vdl.Float64:
			if imag(src) == 0 {
				c.vv.AssignFloat(convertFloatToFloat(real(src), bitlenV(kind)))
				return nil
			}
		case vdl.Complex64, vdl.Complex128:
			re := convertFloatToFloat(real(src), bitlenV(kind))
			im := convertFloatToFloat(imag(src), bitlenV(kind))
			c.vv.AssignComplex(complex(re, im))
			return nil
		}
	}
	return fmt.Errorf("invalid conversion from complex(%g) to %v", src, c.tt)
}

func (c convTarget) fromBytes(src []byte) error {
	if c.vv == nil {
		tt := removeOptional(c.tt)
		switch {
		case tt.Kind() == vdl.Enum:
			// Handle special-case enum first, by calling the Assign method.  Note
			// that vdl.TypeFromReflect has already validated the Assign method, so we
			// can call without error checking.
			if c.rv.CanAddr() {
				in := []reflect.Value{reflect.ValueOf(string(src))}
				out := c.rv.Addr().MethodByName("Set").Call(in)
				if ierr := out[0].Interface(); ierr != nil {
					return ierr.(error)
				}
				return nil
			}
		case c.rv.Kind() == reflect.String:
			c.rv.SetString(string(src)) // TODO(toddw): check utf8
			return nil
		case c.rv.Kind() == reflect.Array:
			if c.rv.Type().Elem() == rtByte && c.rv.Len() == len(src) {
				reflect.Copy(c.rv, reflect.ValueOf(src))
				return nil
			}
		case c.rv.Kind() == reflect.Slice:
			if c.rv.Type().Elem() == rtByte {
				cp := make([]byte, len(src))
				copy(cp, src)
				c.rv.SetBytes(cp)
				return nil
			}
		}
	} else {
		switch c.vv.Kind() {
		case vdl.String:
			c.vv.AssignString(string(src)) // TODO(toddw): check utf8
			return nil
		case vdl.Array:
			if c.vv.Type().IsBytes() && c.vv.Type().Len() == len(src) {
				c.vv.AssignBytes(src)
				return nil
			}
		case vdl.List:
			if c.vv.Type().IsBytes() {
				c.vv.AssignBytes(src)
				return nil
			}
		case vdl.Enum:
			if index := c.vv.Type().EnumIndex(string(src)); index >= 0 {
				c.vv.AssignEnumIndex(index)
				return nil
			}
		}
	}
	return fmt.Errorf("invalid conversion from []byte to %v", c.tt)
}

func (c convTarget) fromTypeObject(src *vdl.Type) error {
	if c.vv == nil {
		if rtPtrToType.ConvertibleTo(c.rv.Type()) {
			c.rv.Set(reflect.ValueOf(src).Convert(c.rv.Type()))
			return nil
		}
	} else {
		if c.vv.Kind() == vdl.TypeObject {
			c.vv.AssignTypeObject(src)
			return nil
		}
	}
	return fmt.Errorf("invalid conversion from typeobject to %v", c.tt)
}

// StartList implements the Target interface method.
func (c convTarget) StartList(tt *vdl.Type, len int) (ListTarget, error) {
	// TODO(bprosnitz) Re-think allocation strategy and possibly use len (currently unused).
	fin, fill, err := startConvert(c, tt)
	return compConvTarget{fin, fill}, err
}

// FinishList implements the Target interface method.
func (c convTarget) FinishList(x ListTarget) error {
	cc := x.(compConvTarget)
	return finishConvert(cc.fin, cc.fill)
}

// StartSet implements the Target interface method.
func (c convTarget) StartSet(tt *vdl.Type, len int) (SetTarget, error) {
	fin, fill, err := startConvert(c, tt)
	return compConvTarget{fin, fill}, err
}

// FinishSet implements the Target interface method.
func (c convTarget) FinishSet(x SetTarget) error {
	cc := x.(compConvTarget)
	return finishConvert(cc.fin, cc.fill)
}

// StartMap implements the Target interface method.
func (c convTarget) StartMap(tt *vdl.Type, len int) (MapTarget, error) {
	fin, fill, err := startConvert(c, tt)
	return compConvTarget{fin, fill}, err
}

// FinishMap implements the Target interface method.
func (c convTarget) FinishMap(x MapTarget) error {
	cc := x.(compConvTarget)
	return finishConvert(cc.fin, cc.fill)
}

// StartFields implements the Target interface method.
func (c convTarget) StartFields(tt *vdl.Type) (FieldsTarget, error) {
	fin, fill, err := startConvert(c, tt)
	return compConvTarget{fin, fill}, err
}

// FinishFields implements the Target interface method.
func (c convTarget) FinishFields(x FieldsTarget) error {
	cc := x.(compConvTarget)
	return finishConvert(cc.fin, cc.fill)
}

// compConvTarget represents the state and logic for composite value conversion.
type compConvTarget struct {
	fin, fill convTarget // fields returned by startConvert.
}

// StartElem implements the ListTarget interface method.
func (cc compConvTarget) StartElem(index int) (elem Target, _ error) {
	return cc.fill.startElem(index)
}

// FinishElem implements the ListTarget interface method.
func (cc compConvTarget) FinishElem(elem Target) error {
	return cc.fill.finishElem(elem.(convTarget))
}

// StartKey implements the SetTarget and MapTarget interface method.
func (cc compConvTarget) StartKey() (key Target, _ error) {
	return cc.fill.startKey()
}

// FinishKeyStartField implements the MapTarget interface method.
func (cc compConvTarget) FinishKeyStartField(key Target) (field Target, _ error) {
	return cc.fill.finishKeyStartField(key.(convTarget))
}

// FinishField implements the MapTarget and FieldsTarget interface method.
func (cc compConvTarget) FinishField(key, field Target) error {
	return cc.fill.finishField(key.(convTarget), field.(convTarget))
}

// StartField implements the FieldsTarget interface method.
func (cc compConvTarget) StartField(name string) (key, field Target, _ error) {
	var err error
	if key, err = cc.StartKey(); err != nil {
		return nil, nil, err
	}
	if err = key.FromString(name, vdl.StringType); err != nil {
		return nil, nil, err
	}
	if field, err = cc.FinishKeyStartField(key); err != nil {
		return nil, nil, err
	}
	return
}

// FinishKey implements the SetTarget interface method.
func (cc compConvTarget) FinishKey(key Target) error {
	field, err := cc.FinishKeyStartField(key)
	if err != nil {
		return err
	}
	return cc.FinishField(key, field)
}

func (c convTarget) startElem(index int) (convTarget, error) {
	if c.vv == nil {
		tt := removeOptional(c.tt)
		switch c.rv.Kind() {
		case reflect.Array:
			if index >= c.rv.Len() {
				return convTarget{}, errArrayIndex
			}
			return reflectConv(c.rv.Index(index), tt.Elem())
		case reflect.Slice:
			newlen := index + 1
			if newlen < c.rv.Len() {
				newlen = c.rv.Len()
			}
			if newlen > c.rv.Cap() {
				rvNew := reflect.MakeSlice(c.rv.Type(), newlen, newlen*2)
				reflect.Copy(rvNew, c.rv)
				c.rv.Set(rvNew)
			} else {
				c.rv.SetLen(newlen)
			}
			return reflectConv(c.rv.Index(index), tt.Elem())
		}
	} else {
		switch c.vv.Kind() {
		case vdl.Array:
			if index >= c.vv.Len() {
				return convTarget{}, errArrayIndex
			}
			return valueConv(c.vv.Index(index)), nil
		case vdl.List:
			newlen := index + 1
			if newlen < c.vv.Len() {
				newlen = c.vv.Len()
			}
			return valueConv(c.vv.AssignLen(newlen).Index(index)), nil
		}
	}
	return convTarget{}, fmt.Errorf("type %v doesn't support StartElem", c.tt)
}

func (c convTarget) finishElem(elem convTarget) error {
	if c.vv == nil {
		switch c.rv.Kind() {
		case reflect.Array, reflect.Slice:
			return nil
		}
	} else {
		switch c.vv.Kind() {
		case vdl.Array, vdl.List:
			return nil
		}
	}
	return fmt.Errorf("type %v doesn't support FinishElem", c.tt)
}

func (c convTarget) startKey() (convTarget, error) {
	if c.vv == nil {
		tt := removeOptional(c.tt)
		switch c.rv.Kind() {
		case reflect.Map:
			return reflectConv(rvSettableZeroValue(c.rv.Type().Key(), tt.Key()), tt.Key())
		case reflect.Struct:
			// The key for struct is the field name, which is a string.
			return reflectConv(reflect.New(rtString).Elem(), vdl.StringType)
		}
	} else {
		switch c.vv.Kind() {
		case vdl.Set, vdl.Map:
			return valueConv(vdl.ZeroValue(c.vv.Type().Key())), nil
		case vdl.Struct, vdl.Union:
			// The key for struct and union is the field name, which is a string.
			return valueConv(vdl.ZeroValue(vdl.StringType)), nil
		}
	}
	return convTarget{}, fmt.Errorf("type %v doesn't support StartKey", c.tt)
}

func (c convTarget) finishKeyStartField(key convTarget) (convTarget, error) {
	// There are various special-cases regarding bool values below.  These are to
	// handle different representations of sets; the following types are all
	// convertible to each other:
	//   set[string], map[string]bool, struct{X, Y, Z bool}
	//
	// To deal with these cases in a uniform manner, we return a bool field for
	// set[string], and initialize bool fields to true.
	if c.vv == nil {
		tt := removeOptional(c.tt)
		switch c.rv.Kind() {
		case reflect.Map:
			var ttField *vdl.Type
			var rvField reflect.Value
			switch rtField := c.rv.Type().Elem(); {
			case tt.Kind() == vdl.Set:
				// The map actually represents a set
				ttField = vdl.BoolType
				rvField = reflect.New(rtBool).Elem()
				rvField.SetBool(true)
			case rtField.Kind() == reflect.Bool:
				ttField = tt.Elem()
				rvField = reflect.New(rtField).Elem()
				rvField.SetBool(true)
			default:
				// TODO(toddw): This doesn't work correctly for map[_]any.
				ttField = tt.Elem()
				rvField = rvSettableZeroValue(rtField, ttField)
			}
			return reflectConv(rvField, ttField)
		case reflect.Struct:
			if tt.Kind() == vdl.Union {
				// Special-case: the fill target is a union concrete field struct.  This
				// means that we should only return a field if the field name matches.
				name := c.rv.MethodByName("Name").Call(nil)[0].String()
				if name != key.rv.String() {
					return convTarget{}, errFieldNotFound
				}
				ttField, _ := tt.FieldByName(name)
				return reflectConv(c.rv.FieldByName("Value"), ttField.Type)
			}
			rvField := c.rv.FieldByName(key.rv.String())
			ttField, index := tt.FieldByName(key.rv.String())
			if !rvField.IsValid() || index < 0 {
				// TODO(toddw): Add a way to track extra and missing fields.
				return convTarget{}, errFieldNotFound
			}
			if rvField.Kind() == reflect.Bool {
				rvField.SetBool(true)
			}
			return reflectConv(rvField, ttField.Type)
		}
	} else {
		switch c.vv.Kind() {
		case vdl.Set:
			return valueConv(vdl.BoolValue(true)), nil
		case vdl.Map:
			vvField := vdl.ZeroValue(c.vv.Type().Elem())
			if vvField.Kind() == vdl.Bool {
				vvField.AssignBool(true)
			}
			return valueConv(vvField), nil
		case vdl.Struct:
			_, index := c.vv.Type().FieldByName(key.vv.RawString())
			if index < 0 {
				// TODO(toddw): Add a way to track extra and missing fields.
				return convTarget{}, errFieldNotFound
			}
			vvField := c.vv.Field(index)
			if vvField.Kind() == vdl.Bool {
				vvField.AssignBool(true)
			}
			return valueConv(vvField), nil
		case vdl.Union:
			f, index := c.vv.Type().FieldByName(key.vv.RawString())
			if index < 0 {
				return convTarget{}, errFieldNotFound
			}
			vvField := vdl.ZeroValue(f.Type)
			return valueConv(vvField), nil
		}
	}
	return convTarget{}, fmt.Errorf("type %v doesn't support FinishKeyStartField", c.tt)
}

func (c convTarget) finishField(key, field convTarget) error {
	// The special-case handling of bool fields matches the special-cases in
	// FinishKeyStartField.
	if c.vv == nil {
		tt := removeOptional(c.tt)
		switch c.rv.Kind() {
		case reflect.Map:
			rvField := field.rv
			if tt.Kind() == vdl.Set {
				// The map actually represents a set
				if !field.rv.Bool() {
					return fmt.Errorf("%v can only be converted from true fields", tt)
				}
				rvField = reflect.Zero(c.rv.Type().Elem())
			}
			if c.rv.IsNil() {
				c.rv.Set(reflect.MakeMap(c.rv.Type()))
			}
			c.rv.SetMapIndex(key.rv, rvField)
			return nil
		case reflect.Struct:
			return nil
		}
	} else {
		switch c.vv.Kind() {
		case vdl.Set:
			if !field.vv.Bool() {
				return fmt.Errorf("%v can only be converted from true fields", c.vv.Type())
			}
			c.vv.AssignSetKey(key.vv)
			return nil
		case vdl.Map:
			c.vv.AssignMapIndex(key.vv, field.vv)
			return nil
		case vdl.Struct:
			return nil
		case vdl.Union:
			_, index := c.vv.Type().FieldByName(key.vv.RawString())
			c.vv.AssignUnionField(index, field.vv)
			return nil
		}
	}
	return fmt.Errorf("type %v doesn't support FinishField", c.tt)
}
