package build

import (
	"math/big"
	"reflect"
	"testing"
)

func TestConst(t *testing.T) {
	for _, test := range constTests {
		for _, tpkg := range test.Pkgs {
			// Compile the package with a single file, and adding the "package foo"
			// prefix to the source data automatically.
			files := map[string]string{
				tpkg.Name + ".idl": "package " + tpkg.Name + "\n" + tpkg.Data,
			}
			build := newFakeBuildPackage(tpkg.Name, tpkg.Name, files)
			env := NewEnv(-1)
			pkg := CompilePackage(build, env)
			// Check expected results.
			expectResult(t, env, test.Name, tpkg.ErrRE)
			if pkg == nil || tpkg.ErrRE != "" {
				continue
			}
			matchRes(t, test.Name, tpkg, pkg.Files[0].ConstDefs)
		}
	}
}

func matchRes(t *testing.T, tname string, tpkg testPkg, cdefs []*ConstDef) {
	if tpkg.ExpectRes.Val == nil {
		return
	}
	// Look for a ConstDef called "res" to compare our expected results.
	for _, cdef := range cdefs {
		if cdef.Name == "res" {
			// reflect.DeepEqual isn't used for all cases because it treats nil and empty slices
			// as different. We really care about differences with respect to Cmp anyways.
			var equal bool
			switch v := cdef.Const.Val.(type) {
			case *big.Int:
				if other, ok := tpkg.ExpectRes.Val.(*big.Int); ok {
					equal = v.Cmp(other) == 0
				}
			case *big.Rat:
				if other, ok := tpkg.ExpectRes.Val.(*big.Rat); ok {
					equal = v.Cmp(other) == 0
				}
			case bigCmplx:
				if other, ok := tpkg.ExpectRes.Val.(bigCmplx); ok {
					equal = v.Equal(other)
				}
			default:
				equal = reflect.DeepEqual(cdef.Const.Val, tpkg.ExpectRes.Val)
			}
			if !equal {
				t.Errorf("%s value got %s, want %s", tname,
					cvString(cdef.Const.Val), cvString(tpkg.ExpectRes.Val))
			}
			actual := cvTypeString(cdef.Const.TypeDef, cdef.Const.Val)
			expect := tpkg.ExpectRes.TypeName
			if actual != expect {
				t.Errorf("%s typedef got %q, want %q", tname, actual, expect)
			}
			return
		}
	}
	t.Errorf("%s couldn't find res in package %s", tname, tpkg.Name)
}

// c represents our expected Const
type c struct {
	Val      interface{}
	TypeName string
}

type testPkg struct {
	Name      string
	Data      string
	ExpectRes c
	ErrRE     string
}

type p []testPkg

var constTests = []struct {
	Name string
	Pkgs p
}{

	// Test literals.
	{
		"UntypedBool",
		p{{"a", `const res = true`, c{true, "untyped bool"}, ""}}},
	{
		"UntypedString",
		p{{"a", `const res = "abc"`, c{"abc", "untyped string"}, ""}}},
	{
		"UntypedInteger",
		p{{"a", `const res = 123`, c{},
			"untyped integer not allowed as final const, value 123"}}},
	{
		"UntypedFloat",
		p{{"a", `const res = 1.5`, c{},
			"untyped float not allowed as final const, value 1.5"}}},
	{
		"UntypedComplex",
		p{{"a", `const res = 3.4+9.8i`, c{},
			"untyped complex not allowed as final const, value 3[.]4[+]9[.]8i"}}},

	// Test explicit primitive type conversions.
	{
		"TypedBool",
		p{{"a", `const res = bool(false)`, c{false, "bool"}, ""}}},
	{
		"TypedString",
		p{{"a", `const res = string("abc")`, c{"abc", "string"}, ""}}},
	{
		"TypedInt32",
		p{{"a", `const res = int32(123)`, c{big.NewInt(123), "int32"}, ""}}},
	{
		"TypedFloat32",
		p{{"a", `const res = float32(1.5)`, c{big.NewRat(3, 2), "float32"}, ""}}},
	{
		"TypedComplex64",
		p{{"a", `const res = complex64(2+1.5i)`, c{bigCmplx{
			re: big.NewRat(2, 1),
			im: big.NewRat(3, 2),
		}, "complex64"}, ""}}},
	{
		"TypedBoolMismatch",
		p{{"a", `const res = bool(1)`, c{},
			"invalid type conversion, can't convert value 1 to type bool"}}},
	{
		"TypedStringMismatch",
		p{{"a", `const res = string(1)`, c{},
			"invalid type conversion, can't convert value 1 to type string"}}},
	{
		"TypedInt32Mismatch",
		p{{"a", `const res = int32(true)`, c{},
			"invalid type conversion, can't convert value true to type int32"}}},
	{
		"TypedFloat32Mismatch",
		p{{"a", `const res = float32(true)`, c{},
			"invalid type conversion, can't convert value true to type float32"}}},

	// Test explicit user type conversions.
	{
		"TypedUserBool",
		p{{"a", `type Bool bool;const res = Bool(true)`, c{true, "a.Bool"}, ""}}},
	{
		"TypedUserString",
		p{{"a", `type Str string;const res = Str("abc")`, c{"abc", "a.Str"}, ""}}},
	{
		"TypedUserInt32",
		p{{"a", `type Int int32;const res = Int(123)`, c{big.NewInt(123), "a.Int"}, ""}}},
	{
		"TypedUserFloat32",
		p{{"a", `type Flt float32;const res = Flt(1.5)`, c{big.NewRat(3, 2), "a.Flt"}, ""}}},
	{
		"TypedUserComplex64",
		p{{"a", `type Cpx complex64;const res = Cpx(1.5+2i)`, c{bigCmplx{
			re: big.NewRat(3, 2),
			im: big.NewRat(2, 1),
		}, "a.Cpx"}, ""}}},
	{
		"TypedUserBoolMismatch",
		p{{"a", `type Bool bool;const res = Bool(1)`, c{},
			"invalid type conversion, can't convert value 1 to type a.Bool"}}},
	{
		"TypedUserStringMismatch",
		p{{"a", `type Str string;const res = Str(1)`, c{},
			"invalid type conversion, can't convert value 1 to type a.Str"}}},
	{
		"TypedUserInt32Mismatch",
		p{{"a", `type Int int32;const res = Int(true)`, c{},
			"invalid type conversion, can't convert value true to type a.Int"}}},
	{
		"TypedUserFloat32Mismatch",
		p{{"a", `type Flt float32;const res = Flt(true)`, c{},
			"invalid type conversion, can't convert value true to type a.Flt"}}},

	// Test named consts.
	{
		"NamedBool",
		p{{"a", `const foo = true;const res = foo`, c{true, "untyped bool"}, ""}}},
	{
		"NamedString",
		p{{"a", `const foo = "abc";const res = foo`, c{"abc", "untyped string"}, ""}}},
	{
		"NamedInt32",
		p{{"a", `const foo = int32(123);const res = foo`, c{big.NewInt(123), "int32"}, ""}}},
	{
		"NamedFloat32",
		p{{"a", `const foo = float32(1.5);const res = foo`, c{big.NewRat(3, 2), "float32"}, ""}}},
	{
		"NamedComplex64",
		p{{"a", `const foo = complex64(3+2i);const res = foo`, c{bigCmplx{
			re: big.NewRat(3, 1),
			im: big.NewRat(2, 1),
		}, "complex64"}, ""}}},
	{
		"NamedUserBool",
		p{{"a", `type Bool bool;const foo = Bool(true);const res = foo`,
			c{true, "a.Bool"}, ""}}},
	{
		"NamedUserString",
		p{{"a", `type Str string;const foo = Str("abc");const res = foo`,
			c{"abc", "a.Str"}, ""}}},
	{
		"NamedUserInt32",
		p{{"a", `type Int int32;const foo = Int(123);const res = foo`,
			c{big.NewInt(123), "a.Int"}, ""}}},
	{
		"NamedUserFloat32",
		p{{"a", `type Flt float32;const foo = Flt(1.5);const res = foo`,
			c{big.NewRat(3, 2), "a.Flt"}, ""}}},

	{
		"ConstNamedI",
		p{{"a", `const i = true;const res = i`, c{true, "untyped bool"}, ""}}},

	// Test unary ops.
	{
		"Not",
		p{{"a", `const res = !true`, c{false, "untyped bool"}, ""}}},
	{
		"Pos",
		p{{"a", `const res = int32(+123)`, c{big.NewInt(123), "int32"}, ""}}},
	{
		"Neg",
		p{{"a", `const res = int32(-123)`, c{big.NewInt(-123), "int32"}, ""}}},
	{
		"Complement",
		p{{"a", `const res = int32(^1)`, c{big.NewInt(-2), "int32"}, ""}}},
	{
		"TypedNot",
		p{{"a", `type Bool bool;const res = !Bool(true)`, c{false, "a.Bool"}, ""}}},
	{
		"TypedPos",
		p{{"a", `type Int int32;const res = Int(+123)`, c{big.NewInt(123), "a.Int"}, ""}}},
	{
		"TypedNeg",
		p{{"a", `type Int int32;const res = Int(-123)`, c{big.NewInt(-123), "a.Int"}, ""}}},
	{
		"TypedComplement",
		p{{"a", `type Int int32;const res = Int(^1)`, c{big.NewInt(-2), "a.Int"}, ""}}},
	{
		"NamedNot",
		p{{"a", `const foo = bool(true);const res = !foo`, c{false, "bool"}, ""}}},
	{
		"NamedPos",
		p{{"a", `const foo = int32(123);const res = +foo`, c{big.NewInt(123), "int32"}, ""}}},
	{
		"NamedNeg",
		p{{"a", `const foo = int32(123);const res = -foo`, c{big.NewInt(-123), "int32"}, ""}}},
	{
		"NamedComplement",
		p{{"a", `const foo = int32(1);const res = ^foo`, c{big.NewInt(-2), "int32"}, ""}}},

	// Test logical and comparison ops.
	{
		"Or",
		p{{"a", `const res = true || false`, c{true, "untyped bool"}, ""}}},
	{
		"And",
		p{{"a", `const res = true && false`, c{false, "untyped bool"}, ""}}},
	{
		"Lt11",
		p{{"a", `const res = 1 < 1`, c{false, "untyped bool"}, ""}}},
	{
		"Lt12",
		p{{"a", `const res = 1 < 2`, c{true, "untyped bool"}, ""}}},
	{
		"Lt21",
		p{{"a", `const res = 2 < 1`, c{false, "untyped bool"}, ""}}},
	{
		"Gt11",
		p{{"a", `const res = 1 > 1`, c{false, "untyped bool"}, ""}}},
	{
		"Gt12",
		p{{"a", `const res = 1 > 2`, c{false, "untyped bool"}, ""}}},
	{
		"Gt21",
		p{{"a", `const res = 2 > 1`, c{true, "untyped bool"}, ""}}},
	{
		"Le11",
		p{{"a", `const res = 1 <= 1`, c{true, "untyped bool"}, ""}}},
	{
		"Le12",
		p{{"a", `const res = 1 <= 2`, c{true, "untyped bool"}, ""}}},
	{
		"Le21",
		p{{"a", `const res = 2 <= 1`, c{false, "untyped bool"}, ""}}},
	{
		"Ge11",
		p{{"a", `const res = 1 >= 1`, c{true, "untyped bool"}, ""}}},
	{
		"Ge12",
		p{{"a", `const res = 1 >= 2`, c{false, "untyped bool"}, ""}}},
	{
		"Ge21",
		p{{"a", `const res = 2 >= 1`, c{true, "untyped bool"}, ""}}},
	{
		"Ne11",
		p{{"a", `const res = 1 != 1`, c{false, "untyped bool"}, ""}}},
	{
		"Ne12",
		p{{"a", `const res = 1 != 2`, c{true, "untyped bool"}, ""}}},
	{
		"Ne21",
		p{{"a", `const res = 2 != 1`, c{true, "untyped bool"}, ""}}},
	{
		"Eq11",
		p{{"a", `const res = 1 == 1`, c{true, "untyped bool"}, ""}}},
	{
		"Eq12",
		p{{"a", `const res = 1 == 2`, c{false, "untyped bool"}, ""}}},
	{
		"Eq21",
		p{{"a", `const res = 2 == 1`, c{false, "untyped bool"}, ""}}},

	// Test arithmetic ops.
	{
		"IntPlus",
		p{{"a", `const res = int32(1) + 1`, c{big.NewInt(2), "int32"}, ""}}},
	{
		"IntMinus",
		p{{"a", `const res = int32(2) - 1`, c{big.NewInt(1), "int32"}, ""}}},
	{
		"IntTimes",
		p{{"a", `const res = int32(3) * 2`, c{big.NewInt(6), "int32"}, ""}}},
	{
		"IntDivide",
		p{{"a", `const res = int32(5) / 2`, c{big.NewInt(2), "int32"}, ""}}},
	{
		"FloatPlus",
		p{{"a", `const res = float32(1) + 1`, c{big.NewRat(2, 1), "float32"}, ""}}},
	{
		"FloatMinus",
		p{{"a", `const res = float32(2) - 1`, c{big.NewRat(1, 1), "float32"}, ""}}},
	{
		"FloatTimes",
		p{{"a", `const res = float32(3) * 2`, c{big.NewRat(6, 1), "float32"}, ""}}},
	{
		"FloatDivide",
		p{{"a", `const res = float32(5) / 2`, c{big.NewRat(5, 2), "float32"}, ""}}},
	{
		"ComplexPlus",
		p{{"a", `const res = 3i + complex64(1+2i) + 1`, c{bigCmplx{
			re: big.NewRat(2, 1),
			im: big.NewRat(5, 1),
		}, "complex64"}, ""}}},
	{
		"ComplexMinus",
		p{{"a", `const res = complex64(1+2i) -4 -1i`, c{bigCmplx{
			re: big.NewRat(-3, 1),
			im: big.NewRat(1, 1),
		}, "complex64"}, ""}}},
	{
		"ComplexTimes",
		p{{"a", `const res = complex64(1+3i) * (5+1i)`, c{bigCmplx{
			re: big.NewRat(2, 1),
			im: big.NewRat(16, 1),
		}, "complex64"}, ""}}},
	{
		"ComplexDivide",
		p{{"a", `const res = complex64(2+16i) / (5+1i)`, c{bigCmplx{
			re: big.NewRat(1, 1),
			im: big.NewRat(3, 1),
		}, "complex64"}, ""}}},

	// Test integer arithmetic ops.
	{
		"Mod",
		p{{"a", `const res = int32(8) % 3`, c{big.NewInt(2), "int32"}, ""}}},
	{
		"BitOr",
		p{{"a", `const res = int32(8) | 7`, c{big.NewInt(15), "int32"}, ""}}},
	{
		"BitAnd",
		p{{"a", `const res = int32(8) & 15`, c{big.NewInt(8), "int32"}, ""}}},
	{
		"BitXor",
		p{{"a", `const res = int32(8) ^ 5`, c{big.NewInt(13), "int32"}, ""}}},
	{
		"UntypedFloatMod",
		p{{"a", `const res = int32(8.0 % 3.0)`, c{big.NewInt(2), "int32"}, ""}}},
	{
		"UntypedFloatBitOr",
		p{{"a", `const res = int32(8.0 | 7.0)`, c{big.NewInt(15), "int32"}, ""}}},
	{
		"UntypedFloatBitAnd",
		p{{"a", `const res = int32(8.0 & 15.0)`, c{big.NewInt(8), "int32"}, ""}}},
	{
		"UntypedFloatBitXor",
		p{{"a", `const res = int32(8.0 ^ 5.0)`, c{big.NewInt(13), "int32"}, ""}}},
	{
		"TypedFloatMod",
		p{{"a", `const res = int32(float32(8.0) % 3.0)`, c{},
			`invalid binary mod "%", can't convert typed float32 const 8.0 to integer`}}},
	{
		"TypedFloatBitOr",
		p{{"a", `const res = int32(float32(8.0) | 7.0)`, c{},
			`invalid binary bit-or "|", can't convert typed float32 const 8.0 to integer`}}},
	{
		"TypedFloatBitAnd",
		p{{"a", `const res = int32(float32(8.0) & 15.0)`, c{},
			`invalid binary bit-and "&", can't convert typed float32 const 8.0 to integer`}}},
	{
		"TypedFloatBitXor",
		p{{"a", `const res = int32(float32(8.0) ^ 5.0)`, c{},
			`invalid binary bit-xor "\^", can't convert typed float32 const 8.0 to integer`}}},

	// Test shift ops.
	{
		"Lsh",
		p{{"a", `const res = int32(8) << 2`, c{big.NewInt(32), "int32"}, ""}}},
	{
		"Rsh",
		p{{"a", `const res = int32(8) >> 2`, c{big.NewInt(2), "int32"}, ""}}},
	{
		"UntypedFloatLsh",
		p{{"a", `const res = int32(8.0 << 2.0)`, c{big.NewInt(32), "int32"}, ""}}},
	{
		"UntypedFloatRsh",
		p{{"a", `const res = int32(8.0 >> 2.0)`, c{big.NewInt(2), "int32"}, ""}}},

	// Test mixed ops.
	{
		"Mixed",
		p{{"a", `const f = "f";const res = "f" == f && (1+2) == 3`, c{true, "untyped bool"}, ""}}},
	{
		"MixedPrecedence",
		p{{"a", `const res = int32(1+2*3-4)`, c{big.NewInt(3), "int32"}, ""}}},

	// Test uint conversion.
	{
		"MaxUint32",
		p{{"a", `const res = uint32(4294967295)`,
			c{big.NewInt(4294967295), "uint32"}, ""}}},
	{
		"MaxUint64",
		p{{"a", `const res = uint64(18446744073709551615)`,
			c{new(big.Int).SetUint64(18446744073709551615), "uint64"}, ""}}},
	{
		"OverflowUint32",
		p{{"a", `const res = uint32(4294967296)`, c{},
			"constant 4294967296 overflows uint32"}}},
	{
		"OverflowUint64",
		p{{"a", `const res = uint64(18446744073709551616)`, c{},
			"constant 18446744073709551616 overflows uint64"}}},
	{
		"NegUint32",
		p{{"a", `const res = uint32(-3)`, c{},
			"constant -3 overflows uint32"}}},
	{
		"NegUint64",
		p{{"a", `const res = uint64(-4)`, c{},
			"constant -4 overflows uint64"}}},
	{
		"ZeroUint32",
		p{{"a", `const res = uint32(0)`, c{bigIntZero, "uint32"},
			""}}},

	// Test int conversion.
	{
		"MinInt32",
		p{{"a", `const res = int32(-2147483648)`,
			c{big.NewInt(-2147483648), "int32"}, ""}}},
	{
		"MinInt64",
		p{{"a", `const res = int64(-9223372036854775808)`,
			c{big.NewInt(-9223372036854775808), "int64"}, ""}}},
	{
		"MinOverflowInt32",
		p{{"a", `const res = int32(-2147483649)`, c{},
			"constant -2147483649 overflows int32"}}},
	{
		"MinOverflowInt64",
		p{{"a", `const res = int64(-9223372036854775809)`, c{},
			"constant -9223372036854775809 overflows int64"}}},
	{
		"MaxInt32",
		p{{"a", `const res = int32(2147483647)`,
			c{big.NewInt(2147483647), "int32"}, ""}}},
	{
		"MaxInt64",
		p{{"a", `const res = int64(9223372036854775807)`,
			c{big.NewInt(9223372036854775807), "int64"}, ""}}},
	{
		"MaxOverflowInt32",
		p{{"a", `const res = int32(2147483648)`, c{},
			"constant 2147483648 overflows int32"}}},
	{
		"MaxOverflowInt64",
		p{{"a", `const res = int64(9223372036854775808)`, c{},
			"constant 9223372036854775808 overflows int64"}}},
	{
		"ZeroInt32",
		p{{"a", `const res = int32(0)`, c{bigIntZero, "int32"},
			""}}},

	// Test float conversion.
	{
		"SmallestFloat32",
		p{{"a", `const res = float32(1.401298464324817070923729583289916131281e-45)`,
			c{new(big.Rat).SetFloat64(1.401298464324817070923729583289916131281e-45), "float32"}, ""}}},
	{
		"SmallestFloat64",
		p{{"a", `const res = float64(4.940656458412465441765687928682213723651e-324)`,
			c{new(big.Rat).SetFloat64(4.940656458412465441765687928682213723651e-324), "float64"}, ""}}},
	{
		"MaxFloat32",
		p{{"a", `const res = float32(3.40282346638528859811704183484516925440e+38)`,
			c{new(big.Rat).SetFloat64(3.40282346638528859811704183484516925440e+38), "float32"}, ""}}},
	{
		"MaxFloat64",
		p{{"a", `const res = float64(1.797693134862315708145274237317043567980e+308)`,
			c{new(big.Rat).SetFloat64(1.797693134862315708145274237317043567980e+308), "float64"}, ""}}},
	{
		"UnderflowFloat32",
		p{{"a", `const res = float32(1.401298464324817070923729583289916131280e-45)`,
			c{}, "underflows float32"}}},
	{
		"UnderflowFloat64",
		p{{"a", `const res = float64(4.940656458412465441765687928682213723650e-324)`,
			c{}, "underflows float64"}}},
	{
		"OverflowFloat32",
		p{{"a", `const res = float32(3.40282346638528859811704183484516925441e+38)`,
			c{}, "overflows float32"}}},
	{
		"OverflowFloat64",
		p{{"a", `const res = float64(1.797693134862315708145274237317043567981e+308)`,
			c{}, "overflows float64"}}},
	{
		"ZeroFloat32",
		p{{"a", `const res = float32(0)`, c{bigRatZero, "float32"},
			""}}},

	// Test complex conversion.
	{
		"RealComplexToFloat",
		p{{"a", `const res = float64(1+0i)`,
			c{big.NewRat(1, 1), "float64"}, ""}}},
	{
		"RealComplexToInt",
		p{{"a", `const res = int32(1+0i)`,
			c{big.NewInt(1), "int32"}, ""}}},
	{
		"FloatToRealComplex",
		p{{"a", `const res = complex64(1.5)`, c{bigCmplx{
			re: big.NewRat(3, 2),
			im: bigRatZero,
		}, "complex64"}, ""}}},
	{
		"IntToRealComplex",
		p{{"a", `const res = complex64(2)`, c{bigCmplx{
			re: big.NewRat(2, 1),
			im: bigRatZero,
		}, "complex64"}, ""}}},

	// Test float rounding - note that 1.1 incurs loss of precision.
	{
		"RoundedCompareFloat32",
		p{{"a", `const res = float32(1.1) == 1.1`, c{true, "untyped bool"}, ""}}},
	{
		"RoundedCompareFloat64",
		p{{"a", `const res = float64(1.1) == 1.1`, c{true, "untyped bool"}, ""}}},
	{
		"RoundedTruncation",
		p{{"a", `const res = float64(float32(1.1)) != 1.1`, c{true, "untyped bool"}, ""}}},
}
