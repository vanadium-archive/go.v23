package compile

import (
	"errors"
	"fmt"
	"math"
	"math/big"
	"strconv"

	"veyron2/val"
)

var (
	bigIntZero     = new(big.Int)
	bigRatZero     = new(big.Rat)
	bigIntOne      = big.NewInt(1)
	bigRatAbsMin32 = new(big.Rat).SetFloat64(math.SmallestNonzeroFloat32)
	bigRatAbsMax32 = new(big.Rat).SetFloat64(math.MaxFloat32)
	bigRatAbsMin64 = new(big.Rat).SetFloat64(math.SmallestNonzeroFloat64)
	bigRatAbsMax64 = new(big.Rat).SetFloat64(math.MaxFloat64)
	maxShiftSize   = big.NewInt(463) // use the same max as Go

	errDivZero = errors.New("divide by zero")
)

// constVal represents a constant value with its associated type.  The supported
// types for Val are:
//   bool       - Represents all boolean constants.
//   string     - Represents all string constants.
//   *big.Int   - Represents all integer constants.
//   *big.Rat   - Represents all rational constants.
//   bigComplex - Represents all complex constants.
//
// If Type is nil the constant is untyped; numerical operations are performed
// with "infinite" precision.  Otherwise the underlying Val must match the kind
// of Type.
type constVal struct {
	Val  interface{}
	Type *val.Type
}

func (cv constVal) String() string {
	return cv.TypeString() + "(" + cvString(cv.Val) + ")"
}

func (cv constVal) TypeString() string {
	return cvTypeString(cv.Val, cv.Type)
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
	case bigComplex:
		return fmt.Sprintf("%v+%vi", cvString(tv.re), cvString(tv.im))
	default:
		panic(fmt.Errorf("idl: unhandled const type %T value %v", val, val))
	}
}

// cvTypeString returns a human-readable string representing the type of the
// const value.
func cvTypeString(val interface{}, t *val.Type) string {
	if t != nil {
		return typeString(t)
	}
	switch val.(type) {
	case bool:
		return "untyped bool"
	case string:
		return "untyped string"
	case *big.Int:
		return "untyped integer"
	case *big.Rat:
		return "untyped rational"
	case bigComplex:
		return "untyped complex"
	default:
		panic(fmt.Errorf("idl: unhandled const type %T value %v", val, val))
	}
}

// cvToFinalValue converts a constVal to a defined val.Value.
func cvToFinalValue(cx constVal) (*val.Value, error) {
	// All const defs must have a type.  We implicitly assign bool and string, but
	// the user must explicitly assign a type for the others.
	if cx.Type == nil {
		switch cx.Val.(type) {
		case bool:
			cx.Type = val.BoolType
		case string:
			cx.Type = val.StringType
		default:
			return nil, fmt.Errorf("%s must be assigned a type", cx)
		}
	}
	// Create a value of the appropriate type.
	vx := val.Zero(cx.Type)
	switch tx := cx.Val.(type) {
	case bool:
		switch vx.Kind() {
		case val.Bool:
			return vx.AssignBool(tx), nil
		}
	case string:
		switch vx.Kind() {
		case val.String:
			return vx.AssignString(tx), nil
		case val.Bytes:
			return vx.AssignBytes([]byte(tx)), nil
		}
	case *big.Int:
		switch vx.Kind() {
		case val.Int32, val.Int64:
			return vx.AssignInt(tx.Int64()), nil
		case val.Uint32, val.Uint64:
			return vx.AssignUint(tx.Uint64()), nil
		}
	case *big.Rat:
		switch vx.Kind() {
		case val.Float32, val.Float64:
			f64, _ := tx.Float64()
			return vx.AssignFloat(f64), nil
		}
	case bigComplex:
		switch vx.Kind() {
		case val.Complex64, val.Complex128:
			re64, _ := tx.re.Float64()
			im64, _ := tx.im.Float64()
			return vx.AssignComplex(complex(re64, im64)), nil
		}
	}
	// Type mismatches shouldn't occur, since makeConstVal always ensures the Val
	// and Type are in sync.  If something's wrong we want to know about it.
	panic(fmt.Errorf("idl: mismatched final type for %v", cx))
}

// cvFromFinalValue converts a defined val.Value to a constVal.
func cvFromFinalValue(v *val.Value) (constVal, error) {
	switch v.Kind() {
	case val.Bool:
		return makeConstVal(v.Bool(), v.Type())
	case val.String:
		return makeConstVal(v.String(), v.Type())
	case val.Bytes:
		return makeConstVal(string(v.Bytes()), v.Type())
	case val.Int32, val.Int64:
		return makeConstVal(new(big.Int).SetInt64(v.Int()), v.Type())
	case val.Uint32, val.Uint64:
		return makeConstVal(new(big.Int).SetUint64(v.Uint()), v.Type())
	case val.Float32, val.Float64:
		return makeConstVal(new(big.Rat).SetFloat64(v.Float()), v.Type())
	case val.Complex64, val.Complex128:
		bc := bigComplex{
			re: new(big.Rat).SetFloat64(real(v.Complex())),
			im: new(big.Rat).SetFloat64(imag(v.Complex())),
		}
		return makeConstVal(bc, v.Type())
	}
	panic(fmt.Errorf("idl: unhandled final value %v(%v)", v.Type(), v))
}

func errNotSupported(val interface{}, t *val.Type) error {
	return fmt.Errorf("%s not supported", cvTypeString(val, t))
}

func evalUnaryOp(op string, cx constVal) (constVal, error) {
	switch op {
	case "!":
		switch x := cx.Val.(type) {
		case bool:
			return makeConstVal(!x, cx.Type)
		}
	case "+":
		switch cx.Val.(type) {
		case *big.Int, *big.Rat, bigComplex:
			return cx, nil
		}
	case "-":
		switch x := cx.Val.(type) {
		case *big.Int:
			return makeConstVal(new(big.Int).Neg(x), cx.Type)
		case *big.Rat:
			return makeConstVal(new(big.Rat).Neg(x), cx.Type)
		case bigComplex:
			return makeConstVal(makeComplex().Neg(x), cx.Type)
		}
	case "^":
		ix, err := constToInt(cx)
		if err != nil {
			return constVal{}, err
		}
		return makeConstVal(new(big.Int).Not(ix), cx.Type)
	default:
		panic(fmt.Errorf("idl: unhandled unary op %q", op))
	}
	return constVal{}, errNotSupported(cx.Val, cx.Type)
}

func evalBinaryOp(op string, cx, cy constVal) (constVal, error) {
	if op == "<<" || op == ">>" {
		// Shift ops are special since they require an integer lhs and unsigned rhs.
		return evalShift(op, cx, cy)
	}
	// All other binary ops behave similarly.  First we perform implicit
	// conversion of x and y.  If either side is untyped, we may need to
	// implicitly convert it to the type of the other side.  If both sides are
	// typed they need to match.  The resulting tx and ty are guaranteed to have
	// the same type, and restype tells us which type we need to convert the
	// result into when we're done.
	x, y, restype, err := implicitConvertConstVals(cx, cy)
	if err != nil {
		return constVal{}, err
	}
	// Now we perform the actual binary op.
	var res interface{}
	switch op {
	case "||", "&&":
		res, err = opLogic(op, x, y, restype)
	case "==", "!=", "<", "<=", ">", ">=":
		res, err = opComp(op, x, y, restype)
		restype = nil // comparisons always result in untyped bool.
	case "+", "-", "*", "/":
		res, err = opArith(op, x, y, restype)
	case "%", "|", "&", "^":
		res, err = opIntArith(op, x, y, restype)
	default:
		panic(fmt.Errorf("idl: unhandled binary op %q", op))
	}
	if err != nil {
		return constVal{}, err
	}
	// As a final step we convert to the result type.
	return makeConstVal(res, restype)
}

func opLogic(op string, x, y interface{}, restype *val.Type) (interface{}, error) {
	switch tx := x.(type) {
	case bool:
		switch op {
		case "||":
			return tx || y.(bool), nil
		case "&&":
			return tx && y.(bool), nil
		}
	}
	return nil, errNotSupported(x, restype)
}

func opComp(op string, x, y interface{}, restype *val.Type) (interface{}, error) {
	switch tx := x.(type) {
	case bool:
		switch op {
		case "==":
			return tx == y.(bool), nil
		case "!=":
			return tx != y.(bool), nil
		}
	case string:
		return compString(op, tx, y.(string)), nil
	case *big.Int:
		return opCmpToBool(op, tx.Cmp(y.(*big.Int))), nil
	case *big.Rat:
		return opCmpToBool(op, tx.Cmp(y.(*big.Rat))), nil
	case bigComplex:
		switch op {
		case "==":
			return tx.Equal(y.(bigComplex)), nil
		case "!=":
			return !tx.Equal(y.(bigComplex)), nil
		}
	}
	return nil, errNotSupported(x, restype)
}

func opArith(op string, x, y interface{}, restype *val.Type) (interface{}, error) {
	switch tx := x.(type) {
	case string:
		if op == "+" {
			return tx + y.(string), nil
		}
	case *big.Int:
		return arithBigInt(op, tx, y.(*big.Int))
	case *big.Rat:
		return arithBigRat(op, tx, y.(*big.Rat))
	case bigComplex:
		return arithBigComplex(op, tx, y.(bigComplex))
	}
	return nil, errNotSupported(x, restype)
}

func opIntArith(op string, x, y interface{}, restype *val.Type) (interface{}, error) {
	ix, err := constToInt(constVal{x, restype})
	if err != nil {
		return nil, err
	}
	iy, err := constToInt(constVal{y, restype})
	if err != nil {
		return nil, err
	}
	return arithBigInt(op, ix, iy)
}

func evalShift(op string, cx, cy constVal) (constVal, error) {
	// lhs must be an integer.
	ix, err := constToInt(cx)
	if err != nil {
		return constVal{}, err
	}
	// rhs must be a small unsigned integer.
	iy, err := constToInt(cy)
	if err != nil {
		return constVal{}, err
	}
	if iy.Sign() < 0 {
		return constVal{}, fmt.Errorf("shift amount %v isn't unsigned", cvString(iy))
	}
	if iy.Cmp(maxShiftSize) > 0 {
		return constVal{}, fmt.Errorf("shift amount %v greater than max allowed %v", cvString(iy), cvString(maxShiftSize))
	}
	// Perform the shift and convert it back to the lhs type.
	return makeConstVal(shiftBigInt(op, ix, uint(iy.Uint64())), cx.Type)
}

// bigRatToInt converts rational to integer values as long as there isn't any
// loss in precision, checking restype to make sure the conversion is allowed.
func bigRatToInt(rat *big.Rat, restype *val.Type) (*big.Int, error) {
	// As a special-case we allow untyped rat consts to be converted to integers,
	// as long as they can do so without loss of precision.  This is safe since
	// untyped rat consts have "unbounded" precision.  Typed float consts may have
	// been rounded at some point, so we don't allow this.  This is the same
	// behavior as Go.
	if restype != nil {
		return nil, fmt.Errorf("can't convert typed %s to integer", cvTypeString(rat, restype))
	}
	if !rat.IsInt() {
		return nil, fmt.Errorf("converting %s %s to integer loses precision", cvTypeString(rat, restype), cvString(rat))
	}
	return new(big.Int).Set(rat.Num()), nil
}

// bigComplexToRat converts complex to rational values as long as the complex
// value has a zero imaginary component.
func bigComplexToRat(b bigComplex) (*big.Rat, error) {
	if b.im.Cmp(bigRatZero) != 0 {
		return nil, fmt.Errorf("can't convert complex %s to rational, nonzero imaginary", cvString(b))
	}
	return b.re, nil
}

// constToInt converts cv to an integer value as long as there isn't any loss in
// precision.
func constToInt(cv constVal) (*big.Int, error) {
	switch tv := cv.Val.(type) {
	case *big.Int:
		return tv, nil
	case *big.Rat:
		return bigRatToInt(tv, cv.Type)
	case bigComplex:
		rat, err := bigComplexToRat(tv)
		if err != nil {
			return nil, err
		}
		return bigRatToInt(rat, cv.Type)
	}
	return nil, fmt.Errorf("can't convert %s to integer", cvTypeString(cv.Val, cv.Type))
}

// makeConstVal creates a constVal with value val and type totype, performing
// overflow and conversion checks on numeric values.  If totype is nil the
// resulting const is untyped.
func makeConstVal(cval interface{}, totype *val.Type) (constVal, error) {
	switch tv := cval.(type) {
	case bool:
		if totype == nil || totype.Kind() == val.Bool {
			return constVal{tv, totype}, nil
		}
	case string:
		if totype == nil || totype.Kind() == val.String {
			return constVal{tv, totype}, nil
		}
	case *big.Int:
		if totype == nil {
			return constVal{tv, nil}, nil
		}
		switch totype.Kind() {
		case val.Int32, val.Int64, val.Uint32, val.Uint64:
			if err := checkOverflowInt(tv, totype); err != nil {
				return constVal{}, err
			}
			return constVal{tv, totype}, nil
		case val.Float32, val.Float64, val.Complex64, val.Complex128:
			return makeConstVal(new(big.Rat).SetInt(tv), totype)
		}
	case *big.Rat:
		if totype == nil {
			return constVal{tv, nil}, nil
		}
		switch totype.Kind() {
		case val.Int32, val.Int64, val.Uint32, val.Uint64:
			// The only way we reach this conversion from big.Rat to a typed integer
			// is for explicit type conversions.  We pass a nil Type to bigRatToInt
			// indicating tv is untyped, to allow all conversions from float to int as
			// long as tv is actually an integer.
			iv, err := bigRatToInt(tv, nil)
			if err != nil {
				return constVal{}, err
			}
			return makeConstVal(iv, totype)
		case val.Float32, val.Float64:
			fv, err := convertFloat(tv, totype.Kind())
			if err != nil {
				return constVal{}, err
			}
			return constVal{fv, totype}, nil
		case val.Complex64, val.Complex128:
			fv, err := convertFloat(tv, totype.Kind())
			if err != nil {
				return constVal{}, err
			}
			return constVal{bigComplex{re: fv, im: bigRatZero}, totype}, nil
		}
	case bigComplex:
		if totype == nil {
			return constVal{tv, nil}, nil
		}
		switch totype.Kind() {
		case val.Int32, val.Int64, val.Uint32, val.Uint64, val.Float32, val.Float64:
			v, err := bigComplexToRat(tv)
			if err != nil {
				return constVal{}, err
			}
			return makeConstVal(v, totype)
		case val.Complex64, val.Complex128:
			v, err := convertComplex(tv, totype.Kind())
			if err != nil {
				return constVal{}, err
			}
			return constVal{v, totype}, nil
		}
	}
	return constVal{}, fmt.Errorf("can't convert %s to %v", cvString(cval), typeString(totype))
}

func bitLen(t *val.Type) int {
	if t == TypeByte {
		// This special-case doesn't work for named byte types.
		// TODO(toddw): Remove the byte type.
		return 8
	}
	switch t.Kind() {
	case val.Int32:
		return 32
	case val.Int64:
		return 64
	case val.Uint32:
		return 32
	case val.Uint64:
		return 64
	default:
		panic(fmt.Errorf("idl: bitLen unhandled type %v", t))
	}
}

// checkOverflowInt returns an error iff converting b to the typed integer will
// cause overflow.
func checkOverflowInt(b *big.Int, t *val.Type) error {
	switch bitlen := bitLen(t); t.Kind() {
	case val.Int32, val.Int64:
		// Account for two's complement, where e.g. int8 ranges from -128 to 127
		if b.Sign() >= 0 {
			// Positives and 0 - just check bitlen, accounting for the sign bit.
			if b.BitLen() >= bitlen {
				return fmt.Errorf("const %v overflows int%d", cvString(b), bitlen)
			}
		} else {
			// Negatives need to take an extra value into account (e.g. -128 for int8)
			bplus1 := new(big.Int).Add(b, bigIntOne)
			if bplus1.BitLen() >= bitlen {
				return fmt.Errorf("const %v overflows int%d", cvString(b), bitlen)
			}
		}
	case val.Uint32, val.Uint64:
		if b.Sign() < 0 || b.BitLen() > bitlen {
			return fmt.Errorf("const %v overflows uint%d", cvString(b), bitlen)
		}
	default:
		panic(fmt.Errorf("idl: checkOverflowInt unhandled type %v", t))
	}
	return nil
}

// checkOverflowRat returns an error iff converting b to the typed rat will
// cause overflow or underflow.
func checkOverflowRat(b *big.Rat, kind val.Kind) error {
	// Exact zero is special cased in ieee754.
	if b.Cmp(bigRatZero) == 0 {
		return nil
	}
	// TODO(toddw): perhaps allow slightly smaller and larger values, to account
	// for ieee754 round-to-even rules.
	switch abs := new(big.Rat).Abs(b); kind {
	case val.Float32, val.Complex64:
		if abs.Cmp(bigRatAbsMin32) < 0 {
			return fmt.Errorf("const %v underflows float32", cvString(b))
		}
		if abs.Cmp(bigRatAbsMax32) > 0 {
			return fmt.Errorf("const %v overflows float32", cvString(b))
		}
	case val.Float64, val.Complex128:
		if abs.Cmp(bigRatAbsMin64) < 0 {
			return fmt.Errorf("const %v underflows float64", cvString(b))
		}
		if abs.Cmp(bigRatAbsMax64) > 0 {
			return fmt.Errorf("const %v overflows float64", cvString(b))
		}
	default:
		panic(fmt.Errorf("idl: checkOverflowRat unhandled kind %v", kind))
	}
	return nil
}

// convertFloat converts b to the typed rat, rounding as necessary.
func convertFloat(b *big.Rat, kind val.Kind) (*big.Rat, error) {
	if err := checkOverflowRat(b, kind); err != nil {
		return nil, err
	}
	switch f64, _ := b.Float64(); kind {
	case val.Float32, val.Complex64:
		return new(big.Rat).SetFloat64(float64(float32(f64))), nil
	case val.Float64, val.Complex128:
		return new(big.Rat).SetFloat64(f64), nil
	default:
		panic(fmt.Errorf("idl: convertFloat unhandled kind %v", kind))
	}
}

// convertComplex converts b to the typed complex, rounding as necessary.
func convertComplex(b bigComplex, kind val.Kind) (bigComplex, error) {
	re, err := convertFloat(b.re, kind)
	if err != nil {
		return bigComplex{}, err
	}
	im, err := convertFloat(b.im, kind)
	if err != nil {
		return bigComplex{}, err
	}
	return bigComplex{re: re, im: im}, nil
}

// implicitConvertConstVals performs implicit conversion of cl and cr based on
// their respective types.  Returns the converted values vl and vr which are
// guaranteed to be of the same type represented by the returned Type, which may
// be nil if both consts are untyped.
func implicitConvertConstVals(cl, cr constVal) (interface{}, interface{}, *val.Type, error) {
	var err error
	if cl.Type != nil && cr.Type != nil {
		// Both consts are typed - their types must match (no implicit conversion).
		if cl.Type != cr.Type {
			return nil, nil, nil, fmt.Errorf("type mismatch %v != %v", cl.TypeString(), cr.TypeString())
		}
		return cl.Val, cr.Val, cl.Type, nil
	}
	if cl.Type != nil {
		// Convert rhs to the type of the lhs.
		cr, err = makeConstVal(cr.Val, cl.Type)
		if err != nil {
			return nil, nil, nil, err
		}
		return cl.Val, cr.Val, cl.Type, nil
	}
	if cr.Type != nil {
		// Convert lhs to the type of the rhs.
		cl, err = makeConstVal(cl.Val, cr.Type)
		if err != nil {
			return nil, nil, nil, err
		}
		return cl.Val, cr.Val, cr.Type, nil
	}
	// Both consts are untyped, might need to implicitly promote untyped consts.
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
			// Promote lhs to rat
			return new(big.Rat).SetInt(vl), vr, nil, nil
		case bigComplex:
			// Promote lhs to complex
			return realComplex(new(big.Rat).SetInt(vl)), vr, nil, nil
		}
	case *big.Rat:
		switch vr := cr.Val.(type) {
		case *big.Int:
			// Promote rhs to rat
			return vl, new(big.Rat).SetInt(vr), nil, nil
		case *big.Rat:
			return vl, vr, nil, nil
		case bigComplex:
			// Promote lhs to complex
			return realComplex(vl), vr, nil, nil
		}
	case bigComplex:
		switch vr := cr.Val.(type) {
		case *big.Int:
			// Promote rhs to complex
			return vl, realComplex(new(big.Rat).SetInt(vr)), nil, nil
		case *big.Rat:
			// Promote rhs to complex
			return vl, realComplex(vr), nil, nil
		case bigComplex:
			return vl, vr, nil, nil
		}
	}
	return nil, nil, nil, fmt.Errorf("mismatched lhs: %s, rhs: %s", cl.TypeString(), cr.TypeString())
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

func arithBigInt(op string, l, r *big.Int) (*big.Int, error) {
	switch op {
	case "+":
		return new(big.Int).Add(l, r), nil
	case "-":
		return new(big.Int).Sub(l, r), nil
	case "*":
		return new(big.Int).Mul(l, r), nil
	case "/":
		if r.Cmp(bigIntZero) == 0 {
			return nil, errDivZero
		}
		return new(big.Int).Quo(l, r), nil
	case "%":
		if r.Cmp(bigIntZero) == 0 {
			return nil, errDivZero
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

func arithBigRat(op string, l, r *big.Rat) (*big.Rat, error) {
	switch op {
	case "+":
		return new(big.Rat).Add(l, r), nil
	case "-":
		return new(big.Rat).Sub(l, r), nil
	case "*":
		return new(big.Rat).Mul(l, r), nil
	case "/":
		if r.Cmp(bigRatZero) == 0 {
			return nil, errDivZero
		}
		return new(big.Rat).Mul(l, new(big.Rat).Inv(r)), nil
	default:
		panic(fmt.Errorf("idl: unhandled op %q", op))
	}
}

func arithBigComplex(op string, l, r bigComplex) (bigComplex, error) {
	switch op {
	case "+":
		return makeComplex().Add(l, r), nil
	case "-":
		return makeComplex().Sub(l, r), nil
	case "*":
		return makeComplex().Mul(l, r), nil
	case "/":
		return makeComplex().Div(l, r)
	default:
		panic(fmt.Errorf("idl: unhandled op %q", op))
	}
}

func shiftBigInt(op string, l *big.Int, n uint) *big.Int {
	switch op {
	case "<<":
		return new(big.Int).Lsh(l, n)
	case ">>":
		return new(big.Int).Rsh(l, n)
	default:
		panic(fmt.Errorf("idl: unhandled op %q", op))
	}
}
