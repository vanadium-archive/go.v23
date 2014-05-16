package val

import (
	"fmt"
	"math/big"
	"testing"
)

const (
	noType           = "must be assigned a type"
	cantConvert      = "can't convert"
	overflows        = "overflows"
	underflows       = "underflows"
	losesPrecision   = "loses precision"
	nonzeroImaginary = "nonzero imaginary"
	notSupported     = "not supported"
	divByZero        = "divide by zero"
)

var (
	boolTypeA      = NamedType("BoolA", BoolType)
	stringTypeA    = NamedType("StringA", StringType)
	bytesTypeA     = NamedType("BytesA", BytesType)
	int32TypeA     = NamedType("Int32A", Int32Type)
	uint32TypeA    = NamedType("Uint32A", Uint32Type)
	float32TypeA   = NamedType("Float32A", Float32Type)
	complex64TypeA = NamedType("Complex64A", Complex64Type)
	structTypeA    = StructType("StructA", []StructField{{"A", Int32Type}})

	bi0              = new(big.Int)
	bi1, bi2, bi3    = big.NewInt(1), big.NewInt(2), big.NewInt(3)
	bi4, bi5, bi6    = big.NewInt(4), big.NewInt(5), big.NewInt(6)
	bi7, bi8, bi9    = big.NewInt(7), big.NewInt(8), big.NewInt(9)
	bi_neg1, bi_neg2 = big.NewInt(-1), big.NewInt(-2)

	br0              = new(big.Rat)
	br1, br2, br3    = big.NewRat(1, 1), big.NewRat(2, 1), big.NewRat(3, 1)
	br4, br5, br6    = big.NewRat(4, 1), big.NewRat(5, 1), big.NewRat(6, 1)
	br7, br8, br9    = big.NewRat(7, 1), big.NewRat(8, 1), big.NewRat(9, 1)
	br_neg1, br_neg2 = big.NewRat(-1, 1), big.NewRat(-2, 1)
)

func boolValue(t *Type, x bool) *Value          { return Zero(t).AssignBool(x) }
func stringValue(t *Type, x string) *Value      { return Zero(t).AssignString(x) }
func bytesValue(t *Type, x string) *Value       { return Zero(t).CopyBytes([]byte(x)) }
func intValue(t *Type, x int64) *Value          { return Zero(t).AssignInt(x) }
func uintValue(t *Type, x uint64) *Value        { return Zero(t).AssignUint(x) }
func floatValue(t *Type, x float64) *Value      { return Zero(t).AssignFloat(x) }
func complexValue(t *Type, x complex128) *Value { return Zero(t).AssignComplex(x) }
func structValue(t *Type, x int64) *Value {
	st := Zero(t)
	st.Field(0).AssignInt(x)
	return st
}

func boolConst(t *Type, x bool) Const          { return ConstFromValue(boolValue(t, x)) }
func stringConst(t *Type, x string) Const      { return ConstFromValue(stringValue(t, x)) }
func bytesConst(t *Type, x string) Const       { return ConstFromValue(bytesValue(t, x)) }
func intConst(t *Type, x int64) Const          { return ConstFromValue(intValue(t, x)) }
func uintConst(t *Type, x uint64) Const        { return ConstFromValue(uintValue(t, x)) }
func floatConst(t *Type, x float64) Const      { return ConstFromValue(floatValue(t, x)) }
func complexConst(t *Type, x complex128) Const { return ConstFromValue(complexValue(t, x)) }
func structConst(t *Type, x int64) Const       { return ConstFromValue(structValue(t, x)) }

func constEqual(a, b Const) bool {
	if !a.IsValid() && !b.IsValid() {
		return true
	}
	res, err := EvalBinary(EQ, a, b)
	if err != nil || !res.IsValid() {
		return false
	}
	val, err := res.ToValue()
	return err == nil && val != nil && val.Kind() == Bool && val.Bool()
}

func TestConstInvalid(t *testing.T) {
	x := Const{}
	if x.IsValid() {
		t.Errorf("zero Const IsValid")
	}
	if got, want := x.String(), "invalid()"; got != want {
		t.Errorf("ToValue got string %v, want %v", got, want)
	}
	{
		value, err := x.ToValue()
		if value != nil {
			t.Errorf("ToValue got valid value %v, want nil", value)
		}
		if got, want := fmt.Sprint(err), "invalid const"; got != want {
			t.Errorf("ToValue got error %q, want %q", got, want)
		}
	}
	{
		result, err := x.Convert(BoolType)
		if result.IsValid() {
			t.Errorf("Convert got valid result %v, want invalid", result)
		}
		if got, want := fmt.Sprint(err), "invalid const"; got != want {
			t.Errorf("Convert got error %q, want %q", got, want)
		}
	}
	unary := []UnaryOp{LogicNot, Pos, Neg, BitNot}
	for _, op := range unary {
		result, err := EvalUnary(op, Const{})
		if result.IsValid() {
			t.Errorf("EvalUnary got valid result %v, want invalid", result)
		}
		if got, want := fmt.Sprint(err), "invalid const"; got != want {
			t.Errorf("EvalUnary got error %q, want %q", got, want)
		}
	}
	binary := []BinaryOp{LogicAnd, LogicOr, EQ, NE, LT, LE, GT, GE, Add, Sub, Mul, Div, Mod, BitAnd, BitOr, BitXor, LeftShift, RightShift}
	for _, op := range binary {
		result, err := EvalBinary(op, Const{}, Const{})
		if result.IsValid() {
			t.Errorf("EvalBinary got valid result %v, want invalid", result)
		}
		if got, want := fmt.Sprint(err), "invalid const"; got != want {
			t.Errorf("EvalBinary got error %q, want %q", got, want)
		}
	}
}

func TestConstToValueOK(t *testing.T) {
	tests := []*Value{
		boolValue(BoolType, true), boolValue(boolTypeA, true),
		stringValue(StringType, "abc"), stringValue(stringTypeA, "abc"),
		bytesValue(BytesType, "abc"), bytesValue(bytesTypeA, "abc"),
		intValue(Int32Type, 123), intValue(int32TypeA, 123),
		uintValue(Uint32Type, 123), uintValue(uint32TypeA, 123),
		floatValue(Float32Type, 123), floatValue(float32TypeA, 123),
		complexValue(Complex64Type, 123), complexValue(complex64TypeA, 123),
		structValue(structTypeA, 123),
	}
	for _, test := range tests {
		c := ConstFromValue(test)
		v, err := c.ToValue()
		if got, want := v, test; !Equal(got, want) {
			t.Errorf("%v.ToValue got %v, want %v", c, got, want)
		}
		expectErr(t, err, "", "%v.ToValue", c)
	}
}

func TestConstToValueImplicit(t *testing.T) {
	tests := []struct {
		C Const
		V *Value
	}{
		{BooleanConst(true), BoolValue(true)},
		{StringConst("abc"), StringValue("abc")},
	}
	for _, test := range tests {
		v, err := test.C.ToValue()
		if got, want := v, test.V; !Equal(got, want) {
			t.Errorf("%v.ToValue got %v, want %v", test.C, got, want)
		}
		expectErr(t, err, "", "%v.ToValue", test.C)
	}
}

func TestConstToValueError(t *testing.T) {
	tests := []struct {
		C      Const
		errstr string
	}{
		{IntegerConst(bi1), noType},
		{RationalConst(br1), noType},
		{ComplexConst(br1, br0), noType},
	}
	for _, test := range tests {
		v, err := test.C.ToValue()
		if v != nil {
			t.Errorf("%v.ToValue got %v, want nil", test.C, v)
		}
		expectErr(t, err, test.errstr, "%v.ToValue", test.C)
	}
}

type c []Const
type v []*Value

func TestConstConvertOK(t *testing.T) {
	// Each test has a set of consts C and values V that are all convertible to
	// each other and equivalent.
	tests := []struct {
		C c
		V v
	}{
		{c{BooleanConst(true)},
			v{boolValue(BoolType, true), boolValue(boolTypeA, true)}},
		{c{StringConst("abc")},
			v{stringValue(StringType, "abc"), stringValue(stringTypeA, "abc"),
				bytesValue(BytesType, "abc"), bytesValue(bytesTypeA, "abc")}},
		{c{IntegerConst(bi1), RationalConst(br1), ComplexConst(br1, br0)},
			v{intValue(Int32Type, 1), intValue(int32TypeA, 1),
				uintValue(Uint32Type, 1), uintValue(uint32TypeA, 1),
				floatValue(Float32Type, 1), floatValue(float32TypeA, 1),
				complexValue(Complex64Type, 1), complexValue(complex64TypeA, 1)}},
		{c{IntegerConst(bi_neg1), RationalConst(br_neg1), ComplexConst(br_neg1, br0)},
			v{intValue(Int32Type, -1), intValue(int32TypeA, -1),
				floatValue(Float32Type, -1), floatValue(float32TypeA, -1),
				complexValue(Complex64Type, -1), complexValue(complex64TypeA, -1)}},
		{c{RationalConst(big.NewRat(1, 2)), ComplexConst(big.NewRat(1, 2), br0)},
			v{floatValue(Float32Type, 0.5), floatValue(float32TypeA, 0.5),
				complexValue(Complex64Type, 0.5), complexValue(complex64TypeA, 0.5)}},
		{c{ComplexConst(br1, br1)},
			v{complexValue(Complex64Type, 1+1i), complexValue(complex64TypeA, 1+1i)}},
	}
	for _, test := range tests {
		// Create a slice of consts containing everything in C and V.
		consts := make([]Const, len(test.C))
		copy(consts, test.C)
		for _, v := range test.V {
			consts = append(consts, ConstFromValue(v))
		}
		// Loop through the consts, and convert each one to each item in V.
		for _, c := range consts {
			for _, v := range test.V {
				vt := v.Type()
				result, err := c.Convert(vt)
				if got, want := result, ConstFromValue(v); !constEqual(got, want) {
					t.Errorf("%v.Convert(%v) result got %v, want %v", c, vt, got, want)
				}
				expectErr(t, err, "", "%v.Convert(%v)", c, vt)
			}
		}
	}
}

type ty []*Type

func TestConstConvertError(t *testing.T) {
	// Each test has a single const C that returns an error that contains errstr
	// when converted to any of the types in the set T.
	tests := []struct {
		C      Const
		T      ty
		errstr string
	}{
		{BooleanConst(true),
			ty{StringType, stringTypeA, BytesType, bytesTypeA,
				Int32Type, int32TypeA, Uint32Type, uint32TypeA,
				Float32Type, float32TypeA, Complex64Type, complex64TypeA, structTypeA},
			cantConvert},
		{StringConst("abc"),
			ty{BoolType, boolTypeA,
				Int32Type, int32TypeA, Uint32Type, uint32TypeA,
				Float32Type, float32TypeA, Complex64Type, complex64TypeA, structTypeA},
			cantConvert},
		{IntegerConst(bi1),
			ty{BoolType, boolTypeA, StringType, stringTypeA, BytesType, bytesTypeA, structTypeA},
			cantConvert},
		{RationalConst(br1),
			ty{BoolType, boolTypeA, StringType, stringTypeA, BytesType, bytesTypeA, structTypeA},
			cantConvert},
		{ComplexConst(br1, br0),
			ty{BoolType, boolTypeA, StringType, stringTypeA, BytesType, bytesTypeA, structTypeA},
			cantConvert},
		// Bounds tests
		{IntegerConst(bi_neg1), ty{Uint32Type, uint32TypeA}, overflows},
		{IntegerConst(big.NewInt(1 << 32)), ty{Int32Type, int32TypeA}, overflows},
		{IntegerConst(big.NewInt(1 << 33)), ty{Uint32Type, uint32TypeA}, overflows},
		{RationalConst(br_neg1), ty{Uint32Type, uint32TypeA}, overflows},
		{RationalConst(big.NewRat(1<<32, 1)), ty{Int32Type, int32TypeA}, overflows},
		{RationalConst(big.NewRat(1<<33, 1)), ty{Uint32Type, uint32TypeA}, overflows},
		{RationalConst(big.NewRat(1, 2)),
			ty{Int32Type, int32TypeA, Uint32Type, uint32TypeA},
			losesPrecision},
		{RationalConst(bigRatAbsMin64), ty{Float32Type, float32TypeA}, underflows},
		{RationalConst(bigRatAbsMax64), ty{Float32Type, float32TypeA}, overflows},
		{ComplexConst(br_neg1, br0), ty{Uint32Type, uint32TypeA}, overflows},
		{ComplexConst(big.NewRat(1<<32, 1), br0), ty{Int32Type, int32TypeA}, overflows},
		{ComplexConst(big.NewRat(1<<33, 1), br0), ty{Uint32Type, uint32TypeA}, overflows},
		{ComplexConst(big.NewRat(1, 2), br0),
			ty{Int32Type, int32TypeA, Uint32Type, uint32TypeA},
			losesPrecision},
		{ComplexConst(bigRatAbsMin64, br0), ty{Float32Type, float32TypeA}, underflows},
		{ComplexConst(bigRatAbsMax64, br0), ty{Float32Type, float32TypeA}, overflows},
		{ComplexConst(br0, br1),
			ty{Int32Type, int32TypeA, Uint32Type, uint32TypeA, Float32Type, float32TypeA},
			nonzeroImaginary},
	}
	for _, test := range tests {
		for _, ct := range test.T {
			result, err := test.C.Convert(ct)
			if result.IsValid() {
				t.Errorf("%v.Convert(%v) result got %v, want invalid", test.C, ct, result)
			}
			expectErr(t, err, test.errstr, "%v.Convert(%v)", test.C, ct)
		}
	}
}

func TestConstUnaryOpOK(t *testing.T) {
	tests := []struct {
		op        UnaryOp
		x, expect Const
	}{
		{LogicNot, BooleanConst(true), BooleanConst(false)},
		{LogicNot, boolConst(BoolType, false), boolConst(BoolType, true)},
		{LogicNot, boolConst(boolTypeA, true), boolConst(boolTypeA, false)},

		{Pos, IntegerConst(bi1), IntegerConst(bi1)},
		{Pos, RationalConst(br1), RationalConst(br1)},
		{Pos, ComplexConst(br1, br1), ComplexConst(br1, br1)},
		{Pos, intConst(Int32Type, 1), intConst(Int32Type, 1)},
		{Pos, floatConst(float32TypeA, 1), floatConst(float32TypeA, 1)},
		{Pos, complexConst(complex64TypeA, 1), complexConst(complex64TypeA, 1)},

		{Neg, IntegerConst(bi1), IntegerConst(bi_neg1)},
		{Neg, RationalConst(br1), RationalConst(br_neg1)},
		{Neg, ComplexConst(br1, br1), ComplexConst(br_neg1, br_neg1)},
		{Neg, intConst(Int32Type, 1), intConst(Int32Type, -1)},
		{Neg, floatConst(float32TypeA, 1), floatConst(float32TypeA, -1)},
		{Neg, complexConst(complex64TypeA, 1), complexConst(complex64TypeA, -1)},

		{BitNot, IntegerConst(bi1), IntegerConst(bi_neg2)},
		{BitNot, RationalConst(br1), IntegerConst(bi_neg2)},
		{BitNot, ComplexConst(br1, br0), IntegerConst(bi_neg2)},
		{BitNot, intConst(Int32Type, 1), intConst(Int32Type, -2)},
		{BitNot, uintConst(uint32TypeA, 1), uintConst(uint32TypeA, 1<<32-2)},
	}
	for _, test := range tests {
		result, err := EvalUnary(test.op, test.x)
		if got, want := result, test.expect; !constEqual(got, want) {
			t.Errorf("EvalUnary(%v, %v) result got %v, want %v", test.op, test.x, got, want)
		}
		expectErr(t, err, "", "EvalUnary(%v, %v)", test.op, test.x)
	}
}

func TestConstUnaryOpError(t *testing.T) {
	tests := []struct {
		op     UnaryOp
		x      Const
		errstr string
	}{
		{LogicNot, StringConst("abc"), notSupported},
		{LogicNot, IntegerConst(bi1), notSupported},
		{LogicNot, RationalConst(br1), notSupported},
		{LogicNot, ComplexConst(br1, br1), notSupported},
		{LogicNot, structConst(structTypeA, 999), notSupported},

		{Pos, BooleanConst(false), notSupported},
		{Pos, StringConst("abc"), notSupported},
		{Pos, structConst(structTypeA, 999), notSupported},

		{Neg, BooleanConst(false), notSupported},
		{Neg, StringConst("abc"), notSupported},
		{Neg, structConst(structTypeA, 999), notSupported},
		{Neg, intConst(Int32Type, 1<<32-1), overflows},

		{BitNot, BooleanConst(false), cantConvert},
		{BitNot, StringConst("abc"), cantConvert},
		{BitNot, RationalConst(big.NewRat(1, 2)), losesPrecision},
		{BitNot, ComplexConst(br1, br1), nonzeroImaginary},
		{BitNot, structConst(structTypeA, 999), notSupported},
		{BitNot, floatConst(float32TypeA, 1), cantConvert},
		{BitNot, complexConst(complex64TypeA, 1), cantConvert},
	}
	for _, test := range tests {
		result, err := EvalUnary(test.op, test.x)
		if result.IsValid() {
			t.Errorf("EvalUnary(%v, %v) result got %v, want invalid", test.op, test.x, result)
		}
		expectErr(t, err, test.errstr, "EvalUnary(%v, %v)", test.op, test.x)
	}
}

func TestConstBinaryOpOK(t *testing.T) {
	tests := []struct {
		op           BinaryOp
		x, y, expect Const
	}{
		{LogicAnd, BooleanConst(true), BooleanConst(true), BooleanConst(true)},
		{LogicAnd, BooleanConst(true), BooleanConst(false), BooleanConst(false)},
		{LogicAnd, BooleanConst(false), BooleanConst(true), BooleanConst(false)},
		{LogicAnd, BooleanConst(false), BooleanConst(false), BooleanConst(false)},
		{LogicAnd, boolConst(boolTypeA, true), boolConst(boolTypeA, true), boolConst(boolTypeA, true)},
		{LogicAnd, boolConst(boolTypeA, true), boolConst(boolTypeA, false), boolConst(boolTypeA, false)},
		{LogicAnd, boolConst(boolTypeA, false), boolConst(boolTypeA, true), boolConst(boolTypeA, false)},
		{LogicAnd, boolConst(boolTypeA, false), boolConst(boolTypeA, false), boolConst(boolTypeA, false)},

		{LogicOr, BooleanConst(true), BooleanConst(true), BooleanConst(true)},
		{LogicOr, BooleanConst(true), BooleanConst(false), BooleanConst(true)},
		{LogicOr, BooleanConst(false), BooleanConst(true), BooleanConst(true)},
		{LogicOr, BooleanConst(false), BooleanConst(false), BooleanConst(false)},
		{LogicOr, boolConst(boolTypeA, true), boolConst(boolTypeA, true), boolConst(boolTypeA, true)},
		{LogicOr, boolConst(boolTypeA, true), boolConst(boolTypeA, false), boolConst(boolTypeA, true)},
		{LogicOr, boolConst(boolTypeA, false), boolConst(boolTypeA, true), boolConst(boolTypeA, true)},
		{LogicOr, boolConst(boolTypeA, false), boolConst(boolTypeA, false), boolConst(boolTypeA, false)},

		{Add, StringConst("abc"), StringConst("def"), StringConst("abcdef")},
		{Add, IntegerConst(bi1), IntegerConst(bi1), IntegerConst(bi2)},
		{Add, RationalConst(br1), RationalConst(br1), RationalConst(br2)},
		{Add, ComplexConst(br1, br1), ComplexConst(br1, br1), ComplexConst(br2, br2)},
		{Add, stringConst(stringTypeA, "abc"), stringConst(stringTypeA, "def"), stringConst(stringTypeA, "abcdef")},
		{Add, bytesConst(bytesTypeA, "abc"), bytesConst(bytesTypeA, "def"), bytesConst(bytesTypeA, "abcdef")},
		{Add, intConst(int32TypeA, 1), intConst(int32TypeA, 1), intConst(int32TypeA, 2)},
		{Add, uintConst(uint32TypeA, 1), uintConst(uint32TypeA, 1), uintConst(uint32TypeA, 2)},
		{Add, floatConst(float32TypeA, 1), floatConst(float32TypeA, 1), floatConst(float32TypeA, 2)},
		{Add, complexConst(complex64TypeA, 1), complexConst(complex64TypeA, 1), complexConst(complex64TypeA, 2)},

		{Sub, IntegerConst(bi2), IntegerConst(bi1), IntegerConst(bi1)},
		{Sub, RationalConst(br2), RationalConst(br1), RationalConst(br1)},
		{Sub, ComplexConst(br2, br2), ComplexConst(br1, br1), ComplexConst(br1, br1)},
		{Sub, intConst(int32TypeA, 2), intConst(int32TypeA, 1), intConst(int32TypeA, 1)},
		{Sub, uintConst(uint32TypeA, 2), uintConst(uint32TypeA, 1), uintConst(uint32TypeA, 1)},
		{Sub, floatConst(float32TypeA, 2), floatConst(float32TypeA, 1), floatConst(float32TypeA, 1)},
		{Sub, complexConst(complex64TypeA, 2), complexConst(complex64TypeA, 1), complexConst(complex64TypeA, 1)},

		{Mul, IntegerConst(bi2), IntegerConst(bi2), IntegerConst(bi4)},
		{Mul, RationalConst(br2), RationalConst(br2), RationalConst(br4)},
		{Mul, ComplexConst(br2, br2), ComplexConst(br2, br2), ComplexConst(br0, br8)},
		{Mul, intConst(int32TypeA, 2), intConst(int32TypeA, 2), intConst(int32TypeA, 4)},
		{Mul, uintConst(uint32TypeA, 2), uintConst(uint32TypeA, 2), uintConst(uint32TypeA, 4)},
		{Mul, floatConst(float32TypeA, 2), floatConst(float32TypeA, 2), floatConst(float32TypeA, 4)},
		{Mul, complexConst(complex64TypeA, 2+2i), complexConst(complex64TypeA, 2+2i), complexConst(complex64TypeA, 8i)},

		{Div, IntegerConst(bi4), IntegerConst(bi2), IntegerConst(bi2)},
		{Div, RationalConst(br4), RationalConst(br2), RationalConst(br2)},
		{Div, ComplexConst(br4, br4), ComplexConst(br2, br2), ComplexConst(br2, br0)},
		{Div, intConst(int32TypeA, 4), intConst(int32TypeA, 2), intConst(int32TypeA, 2)},
		{Div, uintConst(uint32TypeA, 4), uintConst(uint32TypeA, 2), uintConst(uint32TypeA, 2)},
		{Div, floatConst(float32TypeA, 4), floatConst(float32TypeA, 2), floatConst(float32TypeA, 2)},
		{Div, complexConst(complex64TypeA, 4+4i), complexConst(complex64TypeA, 2+2i), complexConst(complex64TypeA, 2)},

		{Mod, IntegerConst(bi3), IntegerConst(bi2), IntegerConst(bi1)},
		{Mod, RationalConst(br3), RationalConst(br2), RationalConst(br1)},
		{Mod, ComplexConst(br3, br0), ComplexConst(br2, br0), ComplexConst(br1, br0)},
		{Mod, intConst(int32TypeA, 3), intConst(int32TypeA, 2), intConst(int32TypeA, 1)},
		{Mod, uintConst(uint32TypeA, 3), uintConst(uint32TypeA, 2), uintConst(uint32TypeA, 1)},

		{BitAnd, IntegerConst(bi3), IntegerConst(bi2), IntegerConst(bi2)},
		{BitAnd, RationalConst(br3), RationalConst(br2), RationalConst(br2)},
		{BitAnd, ComplexConst(br3, br0), ComplexConst(br2, br0), ComplexConst(br2, br0)},
		{BitAnd, intConst(int32TypeA, 3), intConst(int32TypeA, 2), intConst(int32TypeA, 2)},
		{BitAnd, uintConst(uint32TypeA, 3), uintConst(uint32TypeA, 2), uintConst(uint32TypeA, 2)},

		{BitOr, IntegerConst(bi5), IntegerConst(bi3), IntegerConst(bi7)},
		{BitOr, RationalConst(br5), RationalConst(br3), RationalConst(br7)},
		{BitOr, ComplexConst(br5, br0), ComplexConst(br3, br0), ComplexConst(br7, br0)},
		{BitOr, intConst(int32TypeA, 5), intConst(int32TypeA, 3), intConst(int32TypeA, 7)},
		{BitOr, uintConst(uint32TypeA, 5), uintConst(uint32TypeA, 3), uintConst(uint32TypeA, 7)},

		{BitXor, IntegerConst(bi5), IntegerConst(bi3), IntegerConst(bi6)},
		{BitXor, RationalConst(br5), RationalConst(br3), RationalConst(br6)},
		{BitXor, ComplexConst(br5, br0), ComplexConst(br3, br0), ComplexConst(br6, br0)},
		{BitXor, intConst(int32TypeA, 5), intConst(int32TypeA, 3), intConst(int32TypeA, 6)},
		{BitXor, uintConst(uint32TypeA, 5), uintConst(uint32TypeA, 3), uintConst(uint32TypeA, 6)},

		{LeftShift, IntegerConst(bi3), IntegerConst(bi1), IntegerConst(bi6)},
		{LeftShift, RationalConst(br3), RationalConst(br1), RationalConst(br6)},
		{LeftShift, ComplexConst(br3, br0), ComplexConst(br1, br0), ComplexConst(br6, br0)},
		{LeftShift, intConst(int32TypeA, 3), intConst(int32TypeA, 1), intConst(int32TypeA, 6)},
		{LeftShift, uintConst(uint32TypeA, 3), uintConst(uint32TypeA, 1), uintConst(uint32TypeA, 6)},

		{RightShift, IntegerConst(bi5), IntegerConst(bi1), IntegerConst(bi2)},
		{RightShift, RationalConst(br5), RationalConst(br1), RationalConst(br2)},
		{RightShift, ComplexConst(br5, br0), ComplexConst(br1, br0), ComplexConst(br2, br0)},
		{RightShift, intConst(int32TypeA, 5), intConst(int32TypeA, 1), intConst(int32TypeA, 2)},
		{RightShift, uintConst(uint32TypeA, 5), uintConst(uint32TypeA, 1), uintConst(uint32TypeA, 2)},
	}
	for _, test := range tests {
		result, err := EvalBinary(test.op, test.x, test.y)
		if got, want := result, test.expect; !constEqual(got, want) {
			t.Errorf("EvalBinary(%v, %v, %v) result got %v, want %v", test.op, test.x, test.y, got, want)
		}
		expectErr(t, err, "", "EvalBinary(%v, %v, %v)", test.op, test.x, test.y)
	}
}

func expectComp(t *testing.T, op BinaryOp, x, y Const, expect bool) {
	result, err := EvalBinary(op, x, y)
	if got, want := result, BooleanConst(expect); !constEqual(got, want) {
		t.Errorf("EvalBinary(%v, %v, %v) result got %v, want %v", op, x, y, got, want)
	}
	expectErr(t, err, "", "EvalBinary(%v, %v, %v)", op, x, y)
}

func TestConstEQNE(t *testing.T) {
	tests := []struct {
		x, y Const // x != y
	}{
		{BooleanConst(false), BooleanConst(true)},
		{StringConst("abc"), StringConst("def")},
		{ComplexConst(br1, br1), ComplexConst(br2, br2)},

		{boolConst(boolTypeA, false), boolConst(boolTypeA, true)},
		{complexConst(complex64TypeA, 1), complexConst(complex64TypeA, 2)},
		{structConst(structTypeA, 1), structConst(structTypeA, 2)},
	}
	for _, test := range tests {
		expectComp(t, EQ, test.x, test.x, true)
		expectComp(t, EQ, test.x, test.y, false)
		expectComp(t, EQ, test.y, test.x, false)
		expectComp(t, EQ, test.y, test.y, true)

		expectComp(t, NE, test.x, test.x, false)
		expectComp(t, NE, test.x, test.y, true)
		expectComp(t, NE, test.y, test.x, true)
		expectComp(t, NE, test.y, test.y, false)
	}
}

func TestConstOrdered(t *testing.T) {
	tests := []struct {
		x, y Const // x < y
	}{
		{StringConst("abc"), StringConst("def")},
		{IntegerConst(bi1), IntegerConst(bi2)},
		{RationalConst(br1), RationalConst(br2)},

		{stringConst(stringTypeA, "abc"), stringConst(stringTypeA, "def")},
		{bytesConst(bytesTypeA, "abc"), bytesConst(bytesTypeA, "def")},
		{intConst(int32TypeA, 1), intConst(int32TypeA, 2)},
		{uintConst(uint32TypeA, 1), uintConst(uint32TypeA, 2)},
		{floatConst(float32TypeA, 1), floatConst(float32TypeA, 2)},
	}
	for _, test := range tests {
		expectComp(t, EQ, test.x, test.x, true)
		expectComp(t, EQ, test.x, test.y, false)
		expectComp(t, EQ, test.y, test.x, false)
		expectComp(t, EQ, test.y, test.y, true)

		expectComp(t, NE, test.x, test.x, false)
		expectComp(t, NE, test.x, test.y, true)
		expectComp(t, NE, test.y, test.x, true)
		expectComp(t, NE, test.y, test.y, false)

		expectComp(t, LT, test.x, test.x, false)
		expectComp(t, LT, test.x, test.y, true)
		expectComp(t, LT, test.y, test.x, false)
		expectComp(t, LT, test.y, test.y, false)

		expectComp(t, LE, test.x, test.x, true)
		expectComp(t, LE, test.x, test.y, true)
		expectComp(t, LE, test.y, test.x, false)
		expectComp(t, LE, test.y, test.y, true)

		expectComp(t, GT, test.x, test.x, false)
		expectComp(t, GT, test.x, test.y, false)
		expectComp(t, GT, test.y, test.x, true)
		expectComp(t, GT, test.y, test.y, false)

		expectComp(t, GE, test.x, test.x, true)
		expectComp(t, GE, test.x, test.y, false)
		expectComp(t, GE, test.y, test.x, true)
		expectComp(t, GE, test.y, test.y, true)
	}
}

type bo []BinaryOp

func TestConstBinaryOpError(t *testing.T) {
	// For each op in Bops and each x in C, (x op x) returns errstr.
	tests := []struct {
		Bops   bo
		C      c
		errstr string
	}{
		// Type not supported / can't convert errors
		{bo{LogicAnd, LogicOr},
			c{StringConst("abc"),
				stringConst(stringTypeA, "abc"), bytesConst(bytesTypeA, "abc"),
				IntegerConst(bi1), intConst(int32TypeA, 1), uintConst(uint32TypeA, 1),
				RationalConst(br1), floatConst(float32TypeA, 1),
				ComplexConst(br1, br1), complexConst(complex64TypeA, 1),
				structConst(structTypeA, 1)},
			notSupported},
		{bo{LT, LE, GT, GE},
			c{BooleanConst(true), boolConst(boolTypeA, false),
				ComplexConst(br1, br1), complexConst(complex64TypeA, 1),
				structConst(structTypeA, 1)},
			notSupported},
		{bo{Add}, c{structConst(structTypeA, 1)}, notSupported},
		{bo{Sub, Mul, Div},
			c{StringConst("abc"),
				stringConst(stringTypeA, "abc"), bytesConst(bytesTypeA, "abc"),
				structConst(structTypeA, 1)},
			notSupported},
		{bo{Mod, BitAnd, BitOr, BitXor, LeftShift, RightShift},
			c{StringConst("abc"),
				stringConst(stringTypeA, "abc"), bytesConst(bytesTypeA, "abc"),
				structConst(structTypeA, 1)},
			cantConvert},
		// Bounds checking
		{bo{Add}, c{uintConst(uint32TypeA, 1<<31)}, overflows},
		{bo{Mul}, c{uintConst(uint32TypeA, 1<<16)}, overflows},
		{bo{Div}, c{uintConst(uint32TypeA, 0)}, divByZero},
		{bo{LeftShift}, c{uintConst(uint32TypeA, 32)}, overflows},
	}
	for _, test := range tests {
		for _, op := range test.Bops {
			for _, c := range test.C {
				result, err := EvalBinary(op, c, c)
				if result.IsValid() {
					t.Errorf("EvalBinary(%v, %v, %v) result got %v, want invalid", op, c, c, result)
				}
				expectErr(t, err, test.errstr, "EvalBinary(%v, %v, %v)", op, c, c)
			}
		}
	}
}
