package vom

import (
	"bytes"
	"fmt"
	"reflect"
)

// Value is an interface that parallels reflect.Value. Value is less complete than reflect.Value in
// functionality. See reflect.Value for method documentation.
// Value exists because it is not possible to create reflect.Types for arbitrary structs and arrays
// and it is necessary to create a new value type to represent the values for an object of a given
// vom.Type.
type Value interface {
	Len() int
	SetLen(x int)
	Cap() int
	SetCap(x int)
	Bool() bool
	String() string
	Bytes() []byte
	Uint() uint64
	Int() int64
	Float() float64
	IsValid() bool
	Type() Type
	Kind() reflect.Kind
	IsNil() bool
	Elem() Value
	CanSet() bool
	Convert(typ Type) Value
	Index(i int) Value
	Complex() complex128
	NumField() int
	Field(i int) Value
	Interface() interface{}
	Set(x Value)
	SetBool(x bool)
	SetString(x string)
	SetBytes(x []byte)
	SetUint(x uint64)
	SetInt(x int64)
	SetFloat(x float64)
	OverflowUint(x uint64) bool
	OverflowInt(x int64) bool
	OverflowFloat(x float64) bool
	SetMapIndex(key, val Value)
	MapKeys() []Value
	MapIndex(key Value) Value
	Addr() Value
	CanAddr() bool
	Pointer() uintptr

	trySet(x Value) error
}

// reflectValue adapts a reflect.Value object to the Value interface defined in this package.
type reflectValue reflect.Value

func (rv reflectValue) Len() int {
	return reflect.Value(rv).Len()
}

func (rv reflectValue) SetLen(x int) {
	reflect.Value(rv).SetLen(x)
}

func (rv reflectValue) Cap() int {
	return reflect.Value(rv).Cap()
}

func (rv reflectValue) SetCap(x int) {
	reflect.Value(rv).SetCap(x)
}

func (rv reflectValue) NumField() int {
	return reflect.Value(rv).NumField()
}

func (rv reflectValue) Bool() bool {
	return reflect.Value(rv).Bool()
}

func (rv reflectValue) SetBool(x bool) {
	reflect.Value(rv).SetBool(x)
}

func (rv reflectValue) String() string {
	return reflect.Value(rv).String()
}

func (rv reflectValue) SetString(x string) {
	reflect.Value(rv).SetString(x)
}

func (rv reflectValue) Bytes() []byte {
	return reflect.Value(rv).Bytes()
}

func (rv reflectValue) SetBytes(x []byte) {
	reflect.Value(rv).SetBytes(x)
}

func (rv reflectValue) Complex() complex128 {
	return reflect.Value(rv).Complex()
}

func (rv reflectValue) Uint() uint64 {
	return reflect.Value(rv).Uint()
}

func (rv reflectValue) SetUint(x uint64) {
	reflect.Value(rv).SetUint(x)
}

func (rv reflectValue) OverflowUint(x uint64) bool {
	return reflect.Value(rv).OverflowUint(x)
}

func (rv reflectValue) Int() int64 {
	return reflect.Value(rv).Int()
}

func (rv reflectValue) SetInt(x int64) {
	reflect.Value(rv).SetInt(x)
}

func (rv reflectValue) OverflowInt(x int64) bool {
	return reflect.Value(rv).OverflowInt(x)
}

func (rv reflectValue) Float() float64 {
	return reflect.Value(rv).Float()
}

func (rv reflectValue) SetFloat(x float64) {
	reflect.Value(rv).SetFloat(x)
}

func (rv reflectValue) OverflowFloat(x float64) bool {
	return reflect.Value(rv).OverflowFloat(x)
}

func (rv reflectValue) Pointer() uintptr {
	return reflect.Value(rv).Pointer()
}

func (rv reflectValue) IsValid() bool {
	return reflect.Value(rv).IsValid()
}

func (rv reflectValue) Type() Type {
	return ToVomType(reflect.Value(rv).Type())
}

func (rv reflectValue) Kind() reflect.Kind {
	return reflect.Value(rv).Kind()
}

func (rv reflectValue) IsNil() bool {
	return reflect.Value(rv).IsNil()
}

func (rv reflectValue) Elem() Value {
	return ToVomValue(reflect.Value(rv).Elem())
}

func (rv reflectValue) CanSet() bool {
	return reflect.Value(rv).CanSet()
}

func (rv reflectValue) trySet(x Value) (err error) {
	var v reflect.Value
	v, err = ToReflectValue(x)
	if err != nil {
		return err
	}

	defer func() {
		if rec := recover(); rec != nil {
			err = fmt.Errorf("error in Set on reflect.Value: %v", rec)
		}
	}()
	if v.IsValid() {
		reflect.Value(rv).Set(v)
	} else {
		reflect.Value(rv).Set(reflect.New(reflect.Value(rv).Type()).Elem())
	}
	return nil
}

// Set calls set on the underlying reflect.Value.
// It cannot take an object that cannot be converted to a reflectValue.
func (rv reflectValue) Set(x Value) {
	if err := rv.trySet(x); err != nil {
		panic(fmt.Sprintf("error while setting: %v", err))
	}
}

func (rv reflectValue) Convert(typ Type) Value {
	rt, err := ToReflectType(typ)
	if err == nil {
		return ToVomValue(reflect.Value(rv).Convert(rt))
	}
	switch typ.(type) {
	case *vPointerType:
		cconv := rv.Elem().Convert(typ.Elem())
		p := MakePtr(typ)
		p.Elem().Set(cconv)
		return p
	default:
		n := New(typ).Elem()
		err := Copy(n, rv)
		if err != nil {
			panic(fmt.Sprintf("error while copying %v: %v", rv, err))
		}
		return n
	}
}

func (rv reflectValue) Index(i int) Value {
	return ToVomValue(reflect.Value(rv).Index(i))
}

func (rv reflectValue) Field(i int) Value {
	return ToVomValue(reflect.Value(rv).Field(i))
}

func (rv reflectValue) SetMapIndex(key, val Value) {
	kv, err := ToReflectValue(key)
	if err != nil {
		panic(err)
	}
	vv, err := ToReflectValue(val)
	if err != nil {
		panic(err)
	}
	reflect.Value(rv).SetMapIndex(kv, vv)
}

func (rv reflectValue) MapKeys() []Value {
	keys := reflect.Value(rv).MapKeys()
	res := make([]Value, len(keys))
	for i, key := range keys {
		res[i] = ToVomValue(key)
	}
	return res
}

func (rv reflectValue) MapIndex(key Value) Value {
	k, err := ToReflectValue(key)
	if err != nil {
		panic(err)
	}
	return ToVomValue(reflect.Value(rv).MapIndex(k))
}

func (rv reflectValue) Interface() interface{} {
	return reflect.Value(rv).Interface()
}

func (rv reflectValue) Addr() Value {
	return ToVomValue(reflect.Value(rv).Addr())
}

func (rv reflectValue) CanAddr() bool {
	return reflect.Value(rv).CanAddr()
}

type valueBase struct {
	typ Type
}

type keyValuePair struct {
	key Value
	val Value
}

type mapLike struct {
	valueBase
	pairs []keyValuePair
}

type arrayLike struct {
	valueBase
	len   int
	cap   int
	items []Value
}

type interfaceLike struct {
	valueBase
	elem Value
}

type pointerLike struct {
	valueBase
	elem Value
}

// nilPtrProxy is returned from (*pointerLike).Elem() and enables setting nil pointers to non-nil
// values through (*pointerLike).Elem().Set()
type nilPtrProxy struct {
	valueBase
	parent *pointerLike
}

func (*valueBase) Len() int                     { panic("invalid call") }
func (*valueBase) Cap() int                     { panic("invalid call") }
func (*valueBase) SetLen(x int)                 { panic("invalid call") }
func (*valueBase) SetCap(x int)                 { panic("invalid call") }
func (*valueBase) Bool() bool                   { panic("invalid call") }
func (*valueBase) SetBool(x bool)               { panic("invalid call") }
func (*valueBase) SetString(x string)           { panic("invalid call") }
func (*valueBase) Bytes() []byte                { panic("invalid call") }
func (*valueBase) SetBytes(x []byte)            { panic("invalid call") }
func (*valueBase) Uint() uint64                 { panic("invalid call") }
func (*valueBase) SetUint(x uint64)             { panic("invalid call") }
func (*valueBase) OverflowUint(x uint64) bool   { panic("invalid call") }
func (*valueBase) Int() int64                   { panic("invalid call") }
func (*valueBase) SetInt(x int64)               { panic("invalid call") }
func (*valueBase) OverflowInt(x int64) bool     { panic("invalid call") }
func (*valueBase) Float() float64               { panic("invalid call") }
func (*valueBase) SetFloat(x float64)           { panic("invalid call") }
func (*valueBase) OverflowFloat(x float64) bool { panic("invalid call") }
func (*valueBase) Pointer() uintptr             { panic("invalid call") }
func (*valueBase) IsValid() bool                { panic("invalid call") }
func (*valueBase) IsNil() bool                  { panic("invalid call") }
func (*valueBase) Elem() Value                  { panic("invalid call") }
func (*valueBase) trySet(x Value) error         { panic("invalid call") }
func (*valueBase) Convert(typ Type) Value       { panic("invalid call") }
func (*valueBase) Index(i int) Value            { panic("invalid call") }
func (*valueBase) Complex() complex128          { panic("invalid call") }
func (*valueBase) NumField() int                { panic("invalid call") }
func (*valueBase) Field(i int) Value            { panic("invalid call") }
func (*valueBase) SetMapIndex(key, val Value)   { panic("invalid call") }
func (*valueBase) MapKeys() []Value             { panic("invalid call") }
func (*valueBase) MapIndex(key Value) Value     { panic("invalid call") }
func (*valueBase) Addr() Value                  { panic("invalid call") }

func (*valueBase) CanSet() bool {
	return false
}

func (*valueBase) CanAddr() bool {
	return false
}

func (rv *valueBase) Type() Type {
	return rv.typ
}

func (rv *valueBase) Kind() reflect.Kind {
	return rv.Type().Kind()
}

func (rv *valueBase) Set(x Value) {
	if err := rv.trySet(x); err != nil {
		panic(fmt.Sprintf("error while setting: %v", err))
	}
}

func newMapLike(pairs []keyValuePair, typ Type) *mapLike {
	return &mapLike{pairs: pairs, valueBase: valueBase{typ: typ}}
}

func (ml *mapLike) Len() int {
	return len(ml.pairs)
}

func (ml *mapLike) MapKeys() []Value {
	keys := make([]Value, len(ml.pairs))
	for i, kvp := range ml.pairs {
		keys[i] = kvp.key
	}
	return keys
}

func (ml *mapLike) MapIndex(key Value) Value {
	for _, kvp := range ml.pairs {
		if kvp.key == key {
			return kvp.val
		}
	}
	return nil
}

func (ml *mapLike) SetMapIndex(key, val Value) {
	for _, kvp := range ml.pairs {
		if kvp.key == key {
			kvp.val = val
			return
		}
	}
	ml.pairs = append(ml.pairs, keyValuePair{key: key, val: val})
}

func (ml *mapLike) String() string {
	var buf bytes.Buffer
	buf.WriteString("{")
	for _, pair := range ml.pairs {
		if ml.Kind() == reflect.Struct {
			if pair.val == nil {
				buf.WriteString("<nil>")
			} else {
				buf.WriteString(pair.val.String())
			}
		} else {
			if pair.key == nil {
				buf.WriteString("<nil>")
			} else {
				buf.WriteString(pair.val.String())
			}
			buf.WriteString(": ")
			if pair.val == nil {
				buf.WriteString("<nil>")
			} else {
				buf.WriteString(pair.val.String())
			}
		}
		buf.WriteString(", ")
	}
	buf.WriteString("}")
	return buf.String()
}

func (ml *mapLike) Set(x Value) {
	if err := ml.trySet(x); err != nil {
		panic(fmt.Sprintf("error while setting: %v", err))
	}
}

func (ml *mapLike) trySet(v Value) error {
	if ml.Kind() != v.Kind() {
		return fmt.Errorf("type mismatch: %v does not match %v", ml.Kind(), v.Kind())
	}

	if ml.Kind() == reflect.Map {
		keys := v.MapKeys()
		ml.pairs = make([]keyValuePair, len(keys))
		for i, key := range keys {
			ml.pairs[i] = keyValuePair{key, v.MapIndex(key)}
		}
	} else if ml.Kind() == reflect.Struct {
		ml.pairs = make([]keyValuePair, v.Len())
		st := v.Type()
		for i := 0; i < v.Len(); i++ {
			ml.pairs[i] = keyValuePair{ValueOf(st.Field(i).Name), v.Field(i)}
		}
	} else {
		return fmt.Errorf("unknown map like kind: %v", ml.Kind())
	}

	return nil
}

func (ml *mapLike) Field(i int) Value {
	return ml.pairs[i].val
}

func (ml *mapLike) NumField() int {
	return len(ml.pairs)
}

func (il *mapLike) IsValid() bool {
	return true
}

func (ml *mapLike) Interface() interface{} {
	rv, err := ToReflectValue(ml)
	if err != nil {
		panic(err)
	}
	return rv.Interface()
}

func newArrayLike(items []Value, typ Type) *arrayLike {
	return &arrayLike{items: items, len: len(items), cap: len(items), valueBase: valueBase{typ: typ}}
}

func (al *arrayLike) Len() int {
	return al.len
}

func (al *arrayLike) Index(i int) Value {
	return al.items[i]
}

func (al *arrayLike) SetLen(x int) {
	al.len = x
	items := make([]Value, x)
	var m int
	if len(items) < len(al.items) {
		m = len(items)
	} else {
		m = len(al.items)
	}
	i := 0
	for ; i < m; i++ {
		items[i] = al.items[i]
	}
	for ; i < len(items); i++ {
		items[i] = Zero(al.Type().Elem())
	}
	al.items = items
}

func (al *arrayLike) Cap() int {
	return al.cap
}

func (al *arrayLike) SetCap(x int) {
	al.cap = x
}

func (il *arrayLike) IsValid() bool {
	return true
}

func (il *arrayLike) String() string {
	var buf bytes.Buffer
	buf.WriteString("[")
	for _, el := range il.items {
		if el == nil {
			buf.WriteString("<nil>")
		} else {
			buf.WriteString(el.String())
		}
		buf.WriteString(", ")
	}
	buf.WriteString("]")
	return buf.String()
}

func (al *arrayLike) CanSet() bool {
	return true
}

func (al *arrayLike) Set(x Value) {
	if err := al.trySet(x); err != nil {
		panic(fmt.Sprintf("error while setting: %v", err))
	}
}

func (al *arrayLike) trySet(v Value) error {
	if al.Kind() == reflect.Array {
		if v.Kind() != reflect.Array || v.Len() != al.Len() {
			return fmt.Errorf("incompatible array type")
		}
	} else if al.Kind() != reflect.Slice || v.Kind() != reflect.Slice {
		return fmt.Errorf("type mismatch")
	}

	al.len = v.Len()
	al.cap = v.Cap()
	al.items = make([]Value, v.Len(), v.Cap())
	for i := 0; i < v.Len(); i++ {
		al.items[i] = v.Index(i)
	}
	for i := v.Len(); i < v.Cap(); i++ {
		al.items[i] = Zero(v.Type().Elem())
	}
	return nil
}

func (al *arrayLike) Interface() interface{} {
	rv, err := ToReflectValue(al)
	if err != nil {
		panic(err)
	}
	return rv.Interface()
}

func newInterfaceLike(elem Value, typ Type) *interfaceLike {
	return &interfaceLike{elem: elem, valueBase: valueBase{typ: typ}}
}

func (il *interfaceLike) Elem() Value {
	return il.elem
}

func (il *interfaceLike) IsValid() bool {
	return true
}

func (il *interfaceLike) IsNil() bool {
	return il.elem == nil
}

func (il *interfaceLike) String() string {
	if il.elem == nil {
		return "I(<nil>)"
	}
	return il.Elem().String()
}

func (il *interfaceLike) CanSet() bool {
	return true
}

func (il *interfaceLike) Set(x Value) {
	if err := il.trySet(x); err != nil {
		panic(fmt.Sprintf("error while setting: %v", err))
	}
}

func (il *interfaceLike) trySet(x Value) error {
	il.elem = x
	return nil
}

func (il *interfaceLike) Kind() reflect.Kind {
	return reflect.Interface
}

func (il *interfaceLike) Interface() interface{} {
	rv, err := ToReflectValue(il)
	if err != nil {
		panic(err)
	}
	return rv.Interface()
}

func newPointerLike(elem Value, typ Type) *pointerLike {
	return &pointerLike{elem: elem, valueBase: valueBase{typ: typ}}
}

func (pl *pointerLike) Elem() Value {
	if pl.elem != nil {
		return pl.elem
	}
	return &nilPtrProxy{parent: pl, valueBase: valueBase{typ: TypeOf(nil)}}
}

func (pl *pointerLike) IsValid() bool {
	return pl.elem != nil
}

func (pl *pointerLike) IsNil() bool {
	return pl.elem == nil
}

func (il *pointerLike) String() string {
	if il.elem == nil {
		return "PTR(<nil>)"
	}
	return fmt.Sprintf("PTR(%s)", il.Elem().String())
}

func (pl *pointerLike) Interface() interface{} {
	if pl.elem == nil {
		rt, err := ToReflectType(pl.Type())
		if err != nil {
			return nil
		}
		return reflect.Zero(rt).Interface()
	}
	rv, err := ToReflectValue(pl)
	if err != nil {
		panic(err)
	}
	return rv.Interface()
}

func (npp *nilPtrProxy) CanAddr() bool {
	return true
}

func (npp *nilPtrProxy) Addr() Value {
	return npp.parent
}

func (npp *nilPtrProxy) CanSet() bool {
	return true
}

func (npp *nilPtrProxy) Set(x Value) {
	if err := npp.trySet(x); err != nil {
		panic(fmt.Sprintf("error while setting: %v", err))
	}
}

func (npp *nilPtrProxy) trySet(x Value) error {
	npp.parent.elem = x
	return nil
}

func (npp *nilPtrProxy) Interface() interface{} {
	return nil
}

func (npp *nilPtrProxy) String() string {
	return "val[<nil>]"
}

func (npp *nilPtrProxy) Kind() reflect.Kind {
	return reflect.Invalid
}

type valuePair struct {
	first, second Value
}

func copyInternal(out, in Value, seen map[valuePair]bool, valMap map[Value]Value, pointersToSet map[Value]Value) error {
	key := valuePair{first: in, second: out}
	if _, ok := seen[key]; ok {
		return nil
	}
	seen[key] = true

	if in.Kind() != out.Kind() && out.Kind() != reflect.Interface {
		return fmt.Errorf("cannot copy %v to different kind %v\n", in.Kind(), out.Kind())
	}

	switch out.Kind() {
	case reflect.Slice, reflect.Array:
		m := in.Len()
		if out.Len() < in.Len() {
			m = out.Len()
		}
		for i := 0; i < m; i++ {
			if err := copyInternal(out.Index(i), in.Index(i), seen, valMap, pointersToSet); err != nil {
				return err
			}
		}
	case reflect.Map:
		for _, key := range in.MapKeys() {
			val := in.MapIndex(key)
			oval := New(val.Type()).Elem()
			if err := copyInternal(oval, val, seen, valMap, pointersToSet); err != nil {
				return err
			}
			out.SetMapIndex(key, oval)
		}
	case reflect.Struct:
		for i := 0; i < in.NumField(); i++ {
			if in.Type().Field(i).Name[0] >= 'A' && in.Type().Field(i).Name[0] <= 'Z' {
				fld := in.Field(i)
				ofld := out.Field(i)
				if err := copyInternal(ofld, fld, seen, valMap, pointersToSet); err != nil {
					return err
				}
			}
		}
	case reflect.Ptr:
		if in.IsNil() {
			if err := out.trySet(nil); err != nil {
				return err
			}
		} else {
			pointersToSet[out.Elem()] = in.Elem()
		}
	case reflect.Interface:
		nv := New(in.Elem().Type()).Elem()
		if err := copyInternal(nv, in.Elem(), seen, valMap, pointersToSet); err != nil {
			return err
		}
		if err := out.trySet(nv); err != nil {
			return err
		}
	default:
		if err := out.trySet(in); err != nil {
			return err
		}
	}

	valMap[in] = out
	return nil
}

// Copy deep copies the input value to the output value
func Copy(out, in Value) error {
	valueMap := make(map[Value]Value)
	toSet := make(map[Value]Value)
	cpy := copyInternal(out, in, make(map[valuePair]bool), valueMap, toSet)
	for ptr, val := range toSet {
		if newVal, ok := valueMap[val]; ok {
			ptr.Set(newVal)
		} else {
			ptr.Set(val)
		}
	}
	return cpy
}

// ToVomValue converts a reflect.Value to a vom.Value
func ToVomValue(in reflect.Value) Value {
	rv := reflectValue(in)
	return &rv
}

// ValueOf creates a vom.Value that represents the given object.
func ValueOf(in interface{}) Value {
	return ToVomValue(reflect.ValueOf(in))
}

var (
	rvNil = reflect.ValueOf(nil)
)

func toReflectValueInternal(in Value, rt reflect.Type, seen map[Value]reflect.Value) (reflect.Value, error) {
	if val, ok := seen[in]; ok {
		return val, nil
	}

	switch v := in.(type) {
	case *reflectValue:
		return reflect.Value(*v), nil
	case *nilPtrProxy:
		return reflect.ValueOf(nil), nil
	}

	if rt == nil || rt.Kind() == reflect.Interface {
		var err error
		rt, err = ToReflectType(in.Type())
		if err != nil {
			return rvNil, fmt.Errorf("failure trying to convert: %v to reflect type: %v", in.Type(), err)
		}
	}

	var rv reflect.Value
	switch in.Kind() {
	case reflect.Array, reflect.Struct:
		rv = reflect.New(rt).Elem()
	case reflect.Slice:
		rv = reflect.MakeSlice(rt, in.Len(), in.Cap())
	case reflect.Map:
		rv = reflect.MakeMap(rt)
	case reflect.Ptr:
		rs := reflect.MakeSlice(reflect.SliceOf(rt.Elem()), 1, 1)
		rp := reflect.MakeSlice(reflect.SliceOf(rt), 1, 1)
		rp.Index(0).Set(rs.Index(0).Addr())
		rv = rp.Index(0)
	}
	seen[in] = rv

	switch in.Kind() {
	case reflect.Array, reflect.Slice:
		for i, item := range in.(*arrayLike).items {
			child, err := toReflectValueInternal(item, rt.Elem(), seen)
			if err != nil {
				return rvNil, err
			}
			rv.Index(i).Set(child)
		}
	case reflect.Ptr:
		elem, err := toReflectValueInternal(in.Elem(), rt.Elem(), seen)
		if err != nil {
			return rvNil, err
		}
		if elem == reflect.ValueOf(nil) {
			rv.Set(reflect.Zero(rv.Type()))
		} else {
			rv.Elem().Set(elem)
		}
	case reflect.Map:
		for _, pair := range in.(*mapLike).pairs {
			kc, err := toReflectValueInternal(pair.key, rt.Key(), seen)
			if err != nil {
				return rvNil, err
			}
			vc, err := toReflectValueInternal(pair.val, rt.Elem(), seen)
			if err != nil {
				return rvNil, err
			}
			rv.SetMapIndex(kc, vc)
		}
	case reflect.Struct:
		for i, pair := range in.(*mapLike).pairs {
			kc, err := toReflectValueInternal(pair.key, nil, seen)
			if err != nil {
				return rvNil, err
			}
			vc, err := toReflectValueInternal(pair.val, rt.Field(i).Type, seen)
			if err != nil {
				return rvNil, err
			}
			if kc.String()[0] >= 'A' && kc.String()[0] <= 'Z' {
				f := rv.FieldByName(kc.String())
				f.Set(vc)
			}
		}
	}

	return rv, nil
}

// ToReflectValue converts a vom.Value to a reflect.Value. If this is not possible (it is of a
// struct or array type and not cached), it returns an error.
func ToReflectValue(in Value) (reflect.Value, error) {
	if in == nil {
		return reflect.ValueOf(nil), nil
	}
	if rv, ok := in.(*reflectValue); ok {
		return reflect.Value(*rv), nil
	}
	return toReflectValueInternal(in, nil, make(map[Value]reflect.Value))
}

func newInternal(typ Type) Value {
	var el Value
	switch typ.Kind() {
	case reflect.Slice:
		el = MakeSlice(typ, 0, 0)
	case reflect.Array:
		el = MakeArray(typ, typ.Len())
	case reflect.Map:
		el = MakeMap(typ)
	case reflect.Struct:
		el = MakeStruct(typ)
		for i := 0; i < typ.NumField(); i++ {
			el.(*mapLike).pairs[i].val = newInternal(typ.Field(i).Type).Elem()
		}
	case reflect.Ptr:
		el = MakePtr(typ)
	default:
		rt, err := ToReflectType(typ)
		if err != nil {
			panic(fmt.Sprintf("error converting to reflect type: %v", err))
		}
		return ToVomValue(reflect.New(rt))
	}

	ptr := MakePtr(PtrTo(typ)).(*pointerLike)
	ptr.elem = el

	return ptr
}

// New creates a Value of the specified type.
func New(typ Type) Value {
	if typ == nil {
		panic("can't construct a new object of nil type")
	}

	rt, err := ToReflectType(typ)
	if err == nil {
		return ToVomValue(reflect.New(rt))
	}

	return newInternal(typ)
}

// MakeSlice creates a slice of the specified size and type.
func MakeSlice(typ Type, l, c int) Value {
	if rt, ok := typ.(*vReflectType); ok {
		return ToVomValue(reflect.MakeSlice(reflect.Type(rt.rt), l, c))
	}
	sl := newArrayLike(make([]Value, l, c), typ)
	sl.SetCap(c)
	sl.SetLen(l)
	for i := 0; i < len(sl.items); i++ {
		sl.items[i] = Zero(typ.Elem())
	}
	return sl
}

// MakeArray creates an array of the specified type.
func MakeArray(typ Type, l int) Value {
	al := newArrayLike(make([]Value, l), typ)
	for i := 0; i < len(al.items); i++ {
		al.items[i] = Zero(typ.Elem())
	}
	return al
}

// MakeMap creates a map of the specified type.
func MakeMap(typ Type) Value {
	if rt, ok := typ.(*vReflectType); ok {
		return ToVomValue(reflect.MakeMap(reflect.Type(rt.rt)))
	}
	return newMapLike([]keyValuePair{}, typ)
}

// MakeStuct creates a struct of the specified type.
func MakeStruct(typ Type) Value {
	str := newMapLike(make([]keyValuePair, typ.NumField()), typ)
	for i := 0; i < len(str.pairs); i++ {
		str.pairs[i].key = ValueOf(typ.Field(i).Name)
		str.pairs[i].val = Zero(typ.Field(i).Type)
	}
	return str
}

// MakePtr creates a pointer of the specified type.
func MakePtr(typ Type) Value {
	return newPointerLike(nil, typ)
}

// Zero creates a zero value of the specified type.
func Zero(typ Type) Value {
	if typ == nil {
		panic("Nil type passed to zero")
	}
	switch typ.Kind() {
	case reflect.Slice:
		return MakeSlice(typ, 0, 0)
	case reflect.Array:
		return MakeArray(typ, typ.Len())
	case reflect.Map:
		return MakeMap(typ)
	case reflect.Struct:
		return MakeStruct(typ)
	case reflect.Ptr:
		return MakePtr(typ)
	default:
		rt, err := ToReflectType(typ)
		if err != nil {
			panic(fmt.Sprintf("error converting to reflect type in Zero(): %v", err))
		}
		rv := reflect.New(rt).Elem()
		return ToVomValue(rv)
	}
}
