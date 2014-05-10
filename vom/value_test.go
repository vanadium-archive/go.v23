package vom

import (
	"bytes"
	"fmt"
	"reflect"
	"testing"
)

var rt_val reflect.Type = reflect.TypeOf(make([]Value, 0)).Elem()
var rt_rval reflect.Type = reflect.TypeOf(make([]reflect.Value, 0)).Elem()

func TestSettabilityOfReflectValues(t *testing.T) {
	x := uint(0)
	targ := uint(1)
	v := ValueOf(&x).Elem()
	vt := ValueOf(targ)
	v.Set(vt)
	if v.Interface() != targ {
		t.Fatalf("Unexpected value for reflect value: %v expected %v", v.Interface(), targ)
	}
}

func TestValueConversion(t *testing.T) {
	for _, val := range vomTestVals {
		rtOrig := reflect.ValueOf(val.data)
		vtNew := ToVomValue(rtOrig)
		rtNew, err := ToReflectValue(vtNew)
		if err != nil {
			t.Fatalf("Error when converting to reflect value: %v", err)
		}
		if rtOrig != rtNew {
			t.Fatalf("RT values differ: %v %v", rtOrig.Interface(), rtNew.Interface())
		}
	}
}

func TestConversionOfUnregisteredValue(t *testing.T) {
	type exampleType struct {
		val int
	}
	et := exampleType{val: 8}
	vv := ValueOf(et)
	rt, err := ToReflectValue(vv)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if rt.Interface() != et {
		t.Fatalf("Unexpected value")
	}
}

func TestNew(t *testing.T) {
	for _, val := range vomTestVals {
		if val.data == nil {
			continue
		}
		typ := TypeOf(val.data)
		nv := New(typ)
		if nv.Type() != PtrTo(typ) {
			t.Fatalf("Types differ %v %v", nv.Type(), PtrTo(typ))
		}
		actual := nv.Elem()
		if actual.Type() != typ {
			t.Fatalf("Types differ %v %v", actual.Type(), typ)
		}
		if actual.Kind() == reflect.Slice || actual.Kind() == reflect.Array || actual.Kind() == reflect.Struct {
			continue // TODO(bprosnitz) make it possible to call Set() on more types
		}
		if !actual.CanSet() || !actual.CanAddr() {
			t.Fatalf("Can't set or address new value of type: %v", typ)
		}
		zv := Zero(typ)
		if zv.Type() != actual.Type() {
			t.Fatalf("Actual and zero values don't share types")
		}
		actual.Set(zv)
	}
}

func TestZero(t *testing.T) {
	for _, val := range vomTestVals {
		if val.data == nil {
			continue
		}
		typ := TypeOf(val.data)
		z := Zero(typ)
		rtyp := reflect.TypeOf(val.data)
		rz := reflect.Zero(rtyp)
		if _, err := ToReflectType(typ); err != nil {
			continue // some of the examples will fail. not testing type conversion here
		}
		v := z.Interface()
		rv := rz.Interface()
		switch rtyp.Kind() {
		case reflect.Slice:
			if z.Len() != rz.Len() || reflect.TypeOf(v).Elem() != rz.Type().Elem() {
				t.Fatalf("Zero slice values differ")
			}
		case reflect.Map:
			if len(z.MapKeys()) != len(rz.MapKeys()) || reflect.TypeOf(v).Elem() != rz.Type().Elem() || reflect.TypeOf(v).Key() != rz.Type().Key() {
				t.Fatalf("Zero map values differ")
			}
		default:
			if v != rv {
				t.Fatalf("Zero values differ: %v %v Types: %v %v", v, rv, reflect.TypeOf(v), reflect.TypeOf(rv))
			}
		}
	}
}

func TestInvokingValueMethods(t *testing.T) {
	for _, val := range vomTestVals {
		if val.data == nil {
			continue
		}
		vv := ValueOf(val.data)
		rv := reflect.ValueOf(val.data)

		invokeObjectMethods(vv, rv, val.canCallEquals, t)
	}
}

func invokeObjectMethods(vv Value, rv reflect.Value, canCallEquals bool, t *testing.T) {
	for i := 0; i < rt_val.NumMethod(); i++ {
		name := rt_val.Method(i).Name
		if !isExportedName(name) {
			continue
		}
		invokeMethod(vv, rv, name, canCallEquals, t)
	}
}

func generateArgs(vvmty reflect.Type, rvmty reflect.Type) ([]reflect.Value, []reflect.Value, error) {
	if vvmty.NumIn() != rvmty.NumIn() {
		return nil, nil, fmt.Errorf("Arg counts don't match")
	}
	vvargs := make([]reflect.Value, vvmty.NumIn())
	rvargs := make([]reflect.Value, rvmty.NumIn())
	for i := 0; i < vvmty.NumIn(); i++ {
		vvmin := vvmty.In(i)
		rvmin := rvmty.In(i)

		if vvmin != rvmin {
			if vvmin == rt_val {
				if rvmin != rt_rval {
					return nil, nil, fmt.Errorf("One type is val, other is not.")
				}
				val := 0
				vvargs = append(vvargs, reflect.ValueOf(ValueOf(val)))
				rvargs = append(rvargs, reflect.ValueOf(reflect.ValueOf(val)))
			} else if vvmin == rt_type {
				if rvmin != rt_rtype {
					return nil, nil, fmt.Errorf("One type is type, other is not.")
				}
				val := 0
				vvargs = append(vvargs, reflect.ValueOf(TypeOf(val)))
				rvargs = append(rvargs, reflect.ValueOf(reflect.TypeOf(val)))
			} else {
				return nil, nil, fmt.Errorf("Types differ in unexpected way: %v %v", vvmin, rvmin)
			}
		} else {
			vvargs = append(vvargs, reflect.Zero(vvmin))
			rvargs = append(rvargs, reflect.Zero(rvmin))
		}
	}

	return vvargs, rvargs, nil
}

func invokeMethod(vv Value, rv reflect.Value, name string, canCallEquals bool, t *testing.T) {
	vvm := reflect.ValueOf(vv).MethodByName(name)
	rvm := reflect.ValueOf(rv).MethodByName(name)

	vvmty := vvm.Type()
	rvmty := rvm.Type()

	vvargs, rvargs, err := generateArgs(vvmty, rvmty)
	if err != nil {
		t.Errorf("%v(): %v", name, err)
		return
	}

	vvret, vverr := tryCall(vvm, vvargs)
	rvret, rverr := tryCall(rvm, rvargs)

	if vverr != nil {
		if rverr != nil {
			return
		}
		t.Errorf("%v(): Error from calling with vom value: %v", name, vv)
		return
	} else if rverr != nil {
		t.Errorf("%v(): Error from calling with reflect value: %v", name, rv)
		return
	}

	if len(vvret) != len(rvret) {
		t.Errorf("%v(): Args different lengths", name)
		return
	}
	for i := 0; i < len(vvret); i++ {
		vvr := vvret[i]
		rvr := rvret[i]
		if vvr.Type() != rvr.Type() {
			if vvr.Type() == rt_val {
				if rvr.Type() != rt_rval {
					t.Errorf("%v(): One type is value. The other is not", name)
				}
			} else if vvr.Type() == rt_type {
				if rvr.Type() != rt_rtype {
					t.Errorf("%v(): One is type. The other is not.", name)
				}
			} else if canCallEquals {
				t.Errorf("%v(): Types don't match %v and %v", name, vvr.Type(), rvr.Type())
			}
		} else {
			if !reflect.DeepEqual(vvr.Interface(), rvr.Interface()) {
				t.Errorf("%v(): Return values don't match %v and %v", name, vvr.Interface(), rvr.Interface())
			}
		}
	}
	return
}

func TestSetIndex(t *testing.T) {
	z := MakeSlice(TypeOf([]int{}), 1, 1)
	z.Index(0).Set(ValueOf(1))
	if z.Index(0).Interface() != 1 {
		t.Fatalf("Index value not appropriately set")
	}
}

func TestSetNilPtrElem(t *testing.T) {
	z := MakePtr(PtrTo(TypeOf(0)))
	z.Elem().Set(ValueOf(1))
	if z.Elem().Interface() != 1 {
		t.Fatalf("Pointer elem value not appropriately set")
	}
}

func TestCopyToZero(t *testing.T) {
	for _, val := range vomTestVals {
		if val.data == nil {
			continue
		}
		v := ValueOf(val.data)
		ty := TypeOf(val.data)
		z := Zero(ty)
		err := Copy(z, v)
		if err != nil {
			t.Fatalf("Error during copy: %v\n", err)
		}
		if val.canCreateObj && val.canCallEquals && !reflect.DeepEqual(z.Interface(), val.data) {
			t.Fatalf("Values differ: %v %v   %v %v", z.Type(), z.Interface(), reflect.TypeOf(val.data), val.data)
		}
	}
}

func TestComplexEncodeAndDecode(t *testing.T) {
	c := complex(4, 3)

	var buf bytes.Buffer
	e := NewEncoder(&buf)
	e.Encode(c)

	buf2 := bytes.NewBuffer(buf.Bytes())
	d := NewDecoder(buf2)
	var v interface{}
	d.Decode(&v)

	if v != c {
		t.Fatalf("Complex was not decoded correctly", c)
	}
}

func TestSetValueChangeActual(t *testing.T) {
	v := 5
	vv := ValueOf(&v)
	vv.Elem().Set(ValueOf(7))
	if v != 7 {
		t.Fatalf("Expected original value to change")
	}
	rv, err := ToReflectValue(vv)
	if err != nil {
		t.Fatalf("Error converting to reflect value: %v", err)
	}
	rv.Elem().Set(reflect.ValueOf(9))
	if v != 9 {
		t.Fatalf("Expected original value to change when setting reflect value")
	}
}

func TestSetValueAfterConvert(t *testing.T) {
	v := 5
	vv := ValueOf(&v)
	vv = vv.Convert(PtrTo(vv.Type().Elem()))
	vv.Elem().Set(ValueOf(8))
	if v != 8 {
		t.Fatalf("Expected original value to change")
	}
}
