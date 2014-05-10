package build

import (
	"fmt"
	"math"
	"math/big"
	"strconv"
)

var (
	bigIntZero     = new(big.Int)
	bigRatZero     = new(big.Rat)
	bigIntOne      = big.NewInt(1)
	bigRatAbsMin32 = new(big.Rat).SetFloat64(math.SmallestNonzeroFloat32)
	bigRatAbsMax32 = new(big.Rat).SetFloat64(math.MaxFloat32)
	bigRatAbsMin64 = new(big.Rat).SetFloat64(math.SmallestNonzeroFloat64)
	bigRatAbsMax64 = new(big.Rat).SetFloat64(math.MaxFloat64)
)

func unaryOpToString(op string) string {
	switch op {
	case "!":
		return "not"
	case "+":
		return "pos"
	case "-":
		return "neg"
	case "^":
		return "bit-complement"
	default:
		panic(fmt.Errorf("idl: unhandled unary op %s", op))
	}
}

func binaryOpToString(op string) string {
	switch op {
	case "||":
		return "or"
	case "&&":
		return "and"
	case "==":
		return "eq"
	case "!=":
		return "ne"
	case "<":
		return "lt"
	case ">":
		return "gt"
	case "<=":
		return "le"
	case ">=":
		return "ge"
	case "+":
		return "add"
	case "-":
		return "sub"
	case "*":
		return "mul"
	case "/":
		return "div"
	case "%":
		return "mod"
	case "|":
		return "bit-or"
	case "&":
		return "bit-and"
	case "^":
		return "bit-xor"
	case "<<":
		return "lsh"
	case ">>":
		return "rsh"
	default:
		panic(fmt.Errorf("idl: unhandled binary op %s", op))
	}
}

// cvString returns a human-readable string representing the const value.
func cvString(val interface{}) string {
	switch tv := val.(type) {
	case bool:
		return strconv.FormatBool(tv)
	case string:
		return `"` + tv + `"`
	case *big.Int:
		return tv.String()
	case *big.Rat:
		if tv.IsInt() {
			return tv.Num().String() + ".0"
		}
		fv, _ := tv.Float64()
		return strconv.FormatFloat(fv, 'g', -1, 64)
	case bigCmplx:
		return fmt.Sprintf("%v+%vi", cvString(tv.re), cvString(tv.im))
	default:
		panic(fmt.Errorf("idl: unhandled const type %T value %v", val, val))
	}
}

// cvTypeString returns a human-readable string representing the type of the
// const value.
func cvTypeString(t *TypeDef, val interface{}) string {
	if t != nil {
		return t.String()
	}
	switch val.(type) {
	case bool:
		return "untyped bool"
	case string:
		return "untyped string"
	case *big.Int:
		return "untyped integer"
	case *big.Rat:
		return "untyped float"
	case bigCmplx:
		return "untyped complex"
	default:
		panic(fmt.Errorf("idl: unhandled const type %T value %v", val, val))
	}
}

// bigRatToInt converts rational to integer values as long as there isn't any
// loss in precision, checking restype to make sure the conversion is allowed.
func bigRatToInt(rat *big.Rat, restype *TypeDef, errmsg string) (*big.Int, error) {
	// As a special-case we allow untyped float consts to be converted to
	// integers, as long as they can do so without loss of precision.  This is
	// safe since untyped float consts have "unbounded" precision.  Typed float
	// consts may have been rounded at some point, so we don't allow this.  This
	// is the same behavior as Go.
	if restype != nil {
		return nil, fmt.Errorf("%s, can't convert typed %s const %s to integer", errmsg, cvTypeString(restype, rat), cvString(rat))
	}
	if !rat.IsInt() {
		return nil, fmt.Errorf("%s, converting %s const %s to integer will lose precision", errmsg, cvTypeString(restype, rat), cvString(rat))
	}
	return new(big.Int).Set(rat.Num()), nil
}

// bigCmplxToRat converts a bigCmplex to a big rational after ensuring that the value has a zero
// imaginary component.
func bigCmplxToRat(b bigCmplx) (*big.Rat, error) {
	if b.im.Cmp(bigRatZero) != 0 {
		return nil, fmt.Errorf("Cannot convert to rational. Imaginary component of %v is nonzero",
			cvString(b))
	}
	return b.re, nil
}

// constToInt converts cv to an integer value as long as there isn't any loss in
// precision.
func constToInt(cv Const, errmsg string) (*big.Int, error) {
	switch tv := cv.Val.(type) {
	case *big.Int:
		return tv, nil
	case *big.Rat:
		return bigRatToInt(tv, cv.TypeDef, errmsg)
	}
	return nil, fmt.Errorf("%s, can't convert %s const %s to integer", errmsg, cvTypeString(cv.TypeDef, cv.Val), cvString(cv.Val))
}

// makeConst creates a const with value val and type totype, performing overflow
// and conversion checks on numeric values.  If totype is nil the resulting
// const is untyped.
func makeConst(val interface{}, totype *TypeDef, errmsg string) (Const, error) {
	switch tv := val.(type) {
	case bool:
		if totype == nil || totype.Kind == KindBool {
			return Const{tv, totype}, nil
		}
	case string:
		if totype == nil || totype.Kind == KindString {
			return Const{tv, totype}, nil
		}
	case *big.Int:
		if totype == nil {
			return Const{tv, nil}, nil
		}
		switch totype.Kind {
		case KindByte, KindInt32, KindInt64, KindUint32, KindUint64:
			if err := checkOverflowInt(tv, totype.Kind, errmsg); err != nil {
				return Const{}, err
			}
			return Const{tv, totype}, nil
		case KindFloat32, KindFloat64, KindComplex64, KindComplex128:
			return makeConst(new(big.Rat).SetInt(tv), totype, errmsg)
		}
	case *big.Rat:
		if totype == nil {
			return Const{tv, nil}, nil
		}
		switch totype.Kind {
		case KindInt32, KindInt64, KindUint32, KindUint64:
			// The only way we reach this conversion from big.Rat to a typed integer
			// is for explicit type conversions.  We pass a nil TypeDef to bigRatToInt
			// indicating tv is untyped, to allow all conversions from float to int as
			// long as tv is actually an integer.
			iv, err := bigRatToInt(tv, nil, errmsg)
			if err != nil {
				return Const{}, err
			}
			return makeConst(iv, totype, errmsg)
		case KindFloat32, KindFloat64:
			fv, err := convertFloat(tv, totype.Kind, errmsg)
			if err != nil {
				return Const{}, err
			}
			return Const{fv, totype}, nil
		case KindComplex64, KindComplex128:
			fv, err := convertFloat(tv, complexToFloatKind(totype.Kind), errmsg)
			if err != nil {
				return Const{}, err
			}
			cv := bigCmplx{
				re: fv,
				im: bigRatZero,
			}
			return Const{cv, totype}, nil
		}
	case bigCmplx:
		if totype == nil {
			return Const{tv, nil}, nil
		}
		switch totype.Kind {
		case KindInt32, KindInt64, KindUint32, KindUint64, KindFloat32, KindFloat64:
			v, err := bigCmplxToRat(tv)
			if err != nil {
				return Const{}, err
			}
			return makeConst(v, totype, errmsg)
		case KindComplex64, KindComplex128:
			cv, err := convertComplex(tv, totype.Kind, errmsg)
			if err != nil {
				return Const{}, err
			}
			return Const{cv, totype}, nil
		}
	}
	return Const{}, fmt.Errorf("%s, can't convert value %s to type %v", errmsg, cvString(val), totype)
}

func bitLen(kind Kind) int {
	switch kind {
	case KindByte:
		return 8
	case KindInt32:
		return 32
	case KindInt64:
		return 64
	case KindUint32:
		return 32
	case KindUint64:
		return 64
	default:
		panic(fmt.Errorf("idl: unhandled kind %v", kind))
	}
}

// checkOverflowInt returns an error iff converting b to the typed integer kind
// will cause overflow.
func checkOverflowInt(b *big.Int, kind Kind, errmsg string) error {
	switch bits := bitLen(kind); kind {
	case KindInt32, KindInt64:
		// Account for two's complement, where e.g. int8 ranges from -128 to 127
		if b.Sign() >= 0 {
			// Positives and 0 - just check bitlen, accounting for the sign bit.
			if b.BitLen() >= bits {
				return fmt.Errorf("%s, constant %v overflows int%d", errmsg, cvString(b), bits)
			}
		} else {
			// Negatives need to take an extra value into account (e.g. -128 for int8)
			bplus1 := new(big.Int).Add(b, bigIntOne)
			if bplus1.BitLen() >= bits {
				return fmt.Errorf("%s, constant %v overflows int%d", errmsg, cvString(b), bits)
			}
		}
	case KindByte, KindUint32, KindUint64:
		if b.Sign() < 0 || b.BitLen() > bits {
			return fmt.Errorf("%s, constant %v overflows uint%d", errmsg, cvString(b), bits)
		}
	default:
		panic(fmt.Errorf("idl: unhandled kind %v", kind))
	}
	return nil
}

// checkOverflowRat returns an error iff converting b to the typed float kind
// will cause overflow or underflow.
func checkOverflowRat(b *big.Rat, kind Kind, errmsg string) error {
	// Exact zero is special cased in ieee754.
	if b.Cmp(bigRatZero) == 0 {
		return nil
	}
	// TODO(toddw): perhaps allow slightly smaller and larger values, to account
	// for ieee754 round-to-even rules.
	switch ab := new(big.Rat).Abs(b); kind {
	case KindFloat32:
		if ab.Cmp(bigRatAbsMin32) < 0 {
			return fmt.Errorf("%s, constant %v underflows float32", errmsg, cvString(b))
		}
		if ab.Cmp(bigRatAbsMax32) > 0 {
			return fmt.Errorf("%s, constant %v overflows float32", errmsg, cvString(b))
		}
	case KindFloat64:
		if ab.Cmp(bigRatAbsMin64) < 0 {
			return fmt.Errorf("%s, constant %v underflows float64", errmsg, cvString(b))
		}
		if ab.Cmp(bigRatAbsMax64) > 0 {
			return fmt.Errorf("%s, constant %v overflows float64", errmsg, cvString(b))
		}
	default:
		panic(fmt.Errorf("idl: unhandled kind %v", kind))
	}
	return nil
}

// convertFloat converts b to the typed float kind, rounding as necessary.
func convertFloat(b *big.Rat, kind Kind, errmsg string) (*big.Rat, error) {
	if err := checkOverflowRat(b, kind, errmsg); err != nil {
		return nil, err
	}
	switch f64, _ := b.Float64(); kind {
	case KindFloat32:
		return new(big.Rat).SetFloat64(float64(float32(f64))), nil
	case KindFloat64:
		return new(big.Rat).SetFloat64(f64), nil
	default:
		panic(fmt.Errorf("idl: unhandled kind %v", kind))
	}
}

// complexToFloatKind converts the kind of a complex number to the kind of the floats stored in it.
func complexToFloatKind(cKind Kind) Kind {
	switch cKind {
	case KindComplex128:
		return KindFloat64
	case KindComplex64:
		return KindFloat32
	default:
		panic(fmt.Sprintf("invalid complex kind: %v", cKind))
	}
}

// convertComplex converts b to the appropriate typed complex kind.
func convertComplex(b bigCmplx, kind Kind, errmsg string) (bigCmplx, error) {
	var reRat, imRat *big.Rat
	var err error
	fKind := complexToFloatKind(kind)
	if reRat, err = convertFloat(b.re, fKind, errmsg); err != nil {
		return bigCmplx{}, err
	}
	if imRat, err = convertFloat(b.im, fKind, errmsg); err != nil {
		return bigCmplx{}, err
	}

	return bigCmplx{re: reRat, im: imRat}, nil
}

// implicitConvertConstVals performs implicit conversion of cl and cr based on
// their respective types.  Returns the converted values vl and vr which are
// guaranteed to be of the same type represented by the returned TypeDef, which
// may be nil if both consts are untyped.
func implicitConvertConstVals(cl, cr Const, errmsg string) (interface{}, interface{}, *TypeDef, error) {
	var err error
	if cl.TypeDef != nil && cr.TypeDef != nil {
		// Both consts are typed - their types must match (no implicit conversion).
		if cl.TypeDef != cr.TypeDef {
			return nil, nil, nil, fmt.Errorf("%s, typed const mismatch lhs %v rhs %v", errmsg, cl.TypeDef, cr.TypeDef)
		}
		return cl.Val, cr.Val, cl.TypeDef, nil
	}
	if cl.TypeDef != nil {
		// Convert rhs to the type of the lhs.
		cr, err = makeConst(cr.Val, cl.TypeDef, errmsg)
		if err != nil {
			return nil, nil, nil, err
		}
		return cl.Val, cr.Val, cl.TypeDef, nil
	}
	if cr.TypeDef != nil {
		// Convert lhs to the type of the rhs.
		cl, err = makeConst(cl.Val, cr.TypeDef, errmsg)
		if err != nil {
			return nil, nil, nil, err
		}
		return cl.Val, cr.Val, cr.TypeDef, nil
	}
	// Both consts are untyped - we might need to implicitly promote untyped int
	// consts to rat.
	switch vl := cl.Val.(type) {
	case bool:
		switch vr := cr.Val.(type) {
		case bool:
			return vl, vr, nil, nil
		}
	case string:
		switch vr := cr.Val.(type) {
		case string:
			return vl, vr, nil, nil
		}
	case *big.Int:
		switch vr := cr.Val.(type) {
		case *big.Int:
			return vl, vr, nil, nil
		case *big.Rat:
			return new(big.Rat).SetInt(vl), vr, nil, nil // promote lhs to rat
		case bigCmplx:
			return bigCmplx{
				re: new(big.Rat).SetInt(vl),
				im: bigRatZero,
			}, vr, nil, nil // promote lhs to complex
		}
	case *big.Rat:
		switch vr := cr.Val.(type) {
		case *big.Int:
			return vl, new(big.Rat).SetInt(vr), nil, nil // promote rhs to rat
		case *big.Rat:
			return vl, vr, nil, nil
		case bigCmplx:
			return bigCmplx{
				re: vl,
				im: bigRatZero,
			}, vr, nil, nil // promote lhs to complex
		}
	case bigCmplx:
		switch vr := cr.Val.(type) {
		case *big.Int:
			return vl, bigCmplx{
				re: new(big.Rat).SetInt(vr),
				im: bigRatZero,
			}, nil, nil // promote rhs to complex
		case *big.Rat:
			return vl, bigCmplx{
				re: vr,
				im: bigRatZero,
			}, nil, nil // promote rhs to complex
		case bigCmplx:
			return vl, vr, nil, nil
		}
	}
	return nil, nil, nil, fmt.Errorf("%s, untyped const mismatch lhs %s rhs %s", errmsg, cl.TypeString(), cr.TypeString())
}

func compString(op string, l, r string) bool {
	switch op {
	case "==":
		return l == r
	case "!=":
		return l != r
	case "<":
		return l < r
	case "<=":
		return l <= r
	case ">":
		return l > r
	case ">=":
		return l >= r
	default:
		panic(fmt.Errorf("idl: unhandled op %q", op))
	}
}

func opCmpToBool(op string, cmp int) bool {
	switch op {
	case "==":
		return cmp == 0
	case "!=":
		return cmp != 0
	case "<":
		return cmp < 0
	case "<=":
		return cmp <= 0
	case ">":
		return cmp > 0
	case ">=":
		return cmp >= 0
	default:
		panic(fmt.Errorf("idl: unhandled op %q", op))
	}
}

func arithBigInt(op string, l, r *big.Int, errmsg string) (*big.Int, error) {
	switch op {
	case "+":
		return new(big.Int).Add(l, r), nil
	case "-":
		return new(big.Int).Sub(l, r), nil
	case "*":
		return new(big.Int).Mul(l, r), nil
	case "/":
		if r.Cmp(bigIntZero) == 0 {
			return nil, fmt.Errorf("%s, divide by zero", errmsg)
		}
		return new(big.Int).Quo(l, r), nil
	case "%":
		if r.Cmp(bigIntZero) == 0 {
			return nil, fmt.Errorf("%s, divide by zero", errmsg)
		}
		return new(big.Int).Rem(l, r), nil
	case "|":
		return new(big.Int).Or(l, r), nil
	case "&":
		return new(big.Int).And(l, r), nil
	case "^":
		return new(big.Int).Xor(l, r), nil
	default:
		panic(fmt.Errorf("idl: unhandled op %q", op))
	}
}

func arithBigRat(op string, l, r *big.Rat, errmsg string) (*big.Rat, error) {
	switch op {
	case "+":
		return new(big.Rat).Add(l, r), nil
	case "-":
		return new(big.Rat).Sub(l, r), nil
	case "*":
		return new(big.Rat).Mul(l, r), nil
	case "/":
		if r.Cmp(bigRatZero) == 0 {
			return nil, fmt.Errorf("%s, divide by zero", errmsg)
		}
		return new(big.Rat).Mul(l, new(big.Rat).Inv(r)), nil
	default:
		panic(fmt.Errorf("idl: unhandled op %q", op))
	}
}

func arithBigCmplx(op string, l, r bigCmplx, errmsg string) (bigCmplx, error) {
	switch op {
	case "+":
		return l.Add(r), nil
	case "-":
		return l.Sub(r), nil
	case "*":
		return l.Mul(r), nil
	case "/":
		return l.Div(r)
	default:
		panic(fmt.Errorf("idl: unhandled op %q", op))
	}
}

func shiftBigInt(op string, l *big.Int, n uint, errmsg string) (*big.Int, error) {
	switch op {
	case "<<":
		return new(big.Int).Lsh(l, n), nil
	case ">>":
		return new(big.Int).Rsh(l, n), nil
	default:
		panic(fmt.Errorf("idl: unhandled op %q", op))
	}
}
