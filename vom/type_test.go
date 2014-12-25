package vom

import (
	"reflect"
	"testing"
)

var (
	rt_type  = reflect.TypeOf(make([]Type, 0)).Elem()
	rt_rtype = reflect.TypeOf(make([]reflect.Type, 0)).Elem()
	rt_sf    = reflect.TypeOf(StructField{})
	rt_rsf   = reflect.TypeOf(reflect.StructField{})
)

func TestVomTypeMethods(t *testing.T) {
	for _, val := range vomTestVals {
		vt := TypeOf(val.data)
		rt := reflect.TypeOf(val.data)

		compareTypes(vt, rt, t)
	}
}

func compareTypes(vt Type, rt reflect.Type, t *testing.T) {
	if (vt == nil) != (rt == nil) {
		t.Errorf("vt is %v but rt is %v", vt, rt)
		return
	}
	if vt == nil {
		return
	}

	for i := 0; i < rt_type.NumMethod(); i++ {
		name := rt_type.Method(i).Name

		vtMeth := reflect.ValueOf(vt).MethodByName(name)
		rtMeth := reflect.ValueOf(rt).MethodByName(name)

		compareMethodCalls(vtMeth, rtMeth, name, t)
	}
}

func compareMethodCalls(vtMeth reflect.Value, rtMeth reflect.Value, name string, t *testing.T) {
	vtMethTy := vtMeth.Type()
	rtMethTy := rtMeth.Type()

	if vtMethTy.NumIn() != rtMethTy.NumIn() || vtMethTy.NumOut() != rtMethTy.NumOut() {
		t.Errorf("%v(): Number of input or output arguments differ for %v and %v", name, vtMethTy, rtMethTy)
		return
	}
	for j := 0; j < vtMethTy.NumOut(); j++ {
		if (vtMethTy.Out(j) == rt_type && rtMethTy.Out(j) != rt_rtype) ||
			(vtMethTy.Out(j) == rt_sf && rtMethTy.Out(j) != rt_rsf) ||
			(vtMethTy.Out(j) != rt_type && vtMethTy.Out(j) != rt_sf && rtMethTy.Out(j) != vtMethTy.Out(j)) {
			t.Errorf("%v(): Return types differ: %v and %v", name, vtMethTy.Out(j), rtMethTy.Out(j))
			return
		}
	}
	vtArgs := make([]reflect.Value, 0)
	rtArgs := make([]reflect.Value, 0)
	for j := 0; j < vtMethTy.NumIn(); j++ {
		if (vtMethTy.In(j) == rt_type && rtMethTy.In(j) != rt_rtype) ||
			(vtMethTy.In(j) == rt_sf && rtMethTy.In(j) != rt_rsf) ||
			(vtMethTy.In(j) != rt_type && vtMethTy.In(j) != rt_sf && rtMethTy.In(j) != vtMethTy.In(j)) {
			t.Errorf("%v(): Input arg types differ: %v and %v", name, vtMethTy.In(j), rtMethTy.In(j))
			return
		}
		// create some value for input args
		if vtMethTy.In(j) == rt_type {
			vtArgs = append(vtArgs, reflect.ValueOf(TypeOf(0)))
			rtArgs = append(rtArgs, reflect.ValueOf(reflect.TypeOf(0)))
		} else {
			z := reflect.Zero(vtMethTy.In(j))
			vtArgs = append(vtArgs, z)
			rtArgs = append(rtArgs, z)
		}
	}

	vtOut, vtErr := tryCall(vtMeth, vtArgs)
	rtOut, rtErr := tryCall(rtMeth, rtArgs)

	if vtErr != nil {
		if rtErr == nil {
			t.Errorf("%v(): Error from calling with vom value: %v", name, vtErr)
			return
		}
	} else if rtErr != nil {
		t.Errorf("%v(): Error from calling with rt value: %v", name, rtErr)
		return
	}

	for i, vtOutI := range vtOut {
		rtOutI := rtOut[i]
		if vtMethTy.Out(i) == rt_type {
			if vtOutI.Kind() != vtOutI.Kind() {
				t.Errorf("%v(): Kinds differ %v %v", name, vtOutI.Kind(), rtOutI.Kind())
			}
		} else if vtMethTy.Out(i) == rt_sf {
			if vtOutI.Kind() != vtOutI.Kind() {
				t.Errorf("%v(): Kinds differ %v %v", name, vtOutI.Kind(), rtOutI.Kind())
			}
		} else {
			if vtOutI.Interface() != rtOutI.Interface() {
				t.Errorf("%v(): Output values differ: %v %v", name, vtOutI.Interface(), rtOutI.Interface())
			}
		}
	}
}

func TestTypeConversion(t *testing.T) {
	for _, val := range vomTestVals {
		rtOrig := reflect.TypeOf(val.data)
		vtNew := ToVomType(rtOrig)
		if !val.canCreateObj {
			continue
		}
		rtNew, err := ToReflectType(vtNew)
		if err != nil {
			t.Fatalf("%v ToReflectType resulted in an unexpected error: %v", rtOrig, err)
		}
		vtFinal := ToVomType(rtNew)
		if rtOrig != rtNew {
			t.Fatalf("Reflect types changed after translation into vom type: %v %v", rtOrig, rtNew)
		}
		if vtNew != vtFinal {
			t.Fatalf("VOM types were not the same: %v %v", rtOrig, rtNew)
		}
	}
}

func TestTypeHashCons(t *testing.T) {
	for _, val := range vomTestVals {
		ta := TypeOf(val.data)
		tb := TypeOf(val.data)

		if ta != tb {
			t.Fatalf("Hashcons failed for %v %v\n", ta, tb)
		}
	}
}

// Test that StructOf returns a reflect type when possible.
func TestStructOfSometimesReturnsReflectType(t *testing.T) {
	type S struct {
		x int
		y string
	}

	goal := TypeOf(S{})
	other := StructOf([]StructField{StructField{Type: TypeOf(4), PkgPath: "v.io/veyron/veyron2/vom", Name: "x"},
		StructField{Type: TypeOf(""), PkgPath: "v.io/veyron/veyron2/vom", Name: "y"}})

	if vrt, ok := other.(*vReflectType); !ok {
		t.Fatalf("expected struct to be of type vReflectType")
	} else if vrt != goal {
		t.Fatalf("Failed to construct original reflect type")
	}
}

// Test that ArrayOf returns a reflect type when possible.
func TestArrayOfSometimesReturnsReflectType(t *testing.T) {
	goal := TypeOf([3]int{4, 3, 2})
	other := ArrayOf(3, TypeOf(3))

	if vrt, ok := other.(*vReflectType); !ok {
		t.Fatalf("expected array to be of type vReflectType")
	} else if vrt != goal {
		t.Fatalf("Failed to construct original reflect type")
	}
}
