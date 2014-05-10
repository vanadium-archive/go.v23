package build

import (
	"fmt"
	"math/big"
)

var maxShiftSize = big.NewInt(463) // use the same max as Go

// Const represents a constant value with its associated type.  The supported
// types for Val are:
//   bool     - Represents all boolean constants.
//   string   - Represents all string constants.
//   *big.Int - Represents all integer constants.
//   *big.Rat - Represents all float constants.
//   bigCmplx - Represents all complex constants.
//
// If TypeDef is nil the constant is untyped, otherwise the kind of typedef must
// match the type of the underlying value.
type Const struct {
	Val     interface{}
	TypeDef *TypeDef
}

// IsValid returns true iff the Const holds a valid value.
func (c Const) IsValid() bool {
	return c.Val != nil
}

// FinalCheck validates the Const for use as a const definition or tag.  All
// consts used in intermediate expressions may be untyped, but final consts used
// as definitions or tags have stricter requirements; numeric values must be typed.
func (c Const) FinalCheck() error {
	if !c.IsValid() {
		return fmt.Errorf("internal error, invalid const")
	}
	if c.TypeDef == nil {
		switch c.Val.(type) {
		case bool, string:
			return nil
		}
		return fmt.Errorf("%s not allowed as final const, value %s", cvTypeString(nil, c.Val), cvString(c.Val))
	}
	switch c.Val.(type) {
	case bool:
		if c.TypeDef.Kind == KindBool {
			return nil
		}
	case string:
		if c.TypeDef.Kind == KindString {
			return nil
		}
	case *big.Int:
		switch c.TypeDef.Kind {
		case KindByte, KindUint32, KindUint64, KindInt32, KindInt64:
			return nil
		}
	case *big.Rat:
		switch c.TypeDef.Kind {
		case KindFloat32, KindFloat64:
			return nil
		}
	case bigCmplx:
		switch c.TypeDef.Kind {
		case KindComplex64, KindComplex128:
			return nil
		}
	}
	return fmt.Errorf("internal error, %s const mismatched type %s", cvTypeString(nil, c.Val), c.TypeDef)
}

func (c Const) String() string {
	if !c.IsValid() {
		return "INVALID"
	}
	if c.TypeDef != nil {
		return fmt.Sprintf("%v(%v)", c.TypeDef, cvString(c.Val))
	}
	return cvString(c.Val)
}

// TypeString returns a human-readable string representing the type of the typed
// or untyped const.
func (c Const) TypeString() string {
	return cvTypeString(c.TypeDef, c.Val)
}

// constExpr is the interface for all nodes in an expression.
type constExpr interface {
	String() string
	pos() Pos

	// resolve traverses the expression and resolves in-place all named consts to
	// their underlying ConstDef, and all named types to their underlying TypeDef.
	// Returns the named ConstDefs that this expression depends on.  The deps
	// aren't transitive past named ConstDefs; if A depends on B which depends on
	// C, the deps of A are B (but not C).
	resolve(pkgs pkgMap, thisFile *File) (constDefSet, error)

	// eval evaluates the expression and returns the resulting Const.  Requires
	// resolve() to have already been called.
	eval() (Const, error)
}

// literalConst represents literals in const expressions.
type literalConst struct {
	lit interface{}
	p   Pos
}

func (c *literalConst) String() string {
	return cvString(c.lit)
}

func (c *literalConst) pos() Pos {
	return c.p
}

func (c *literalConst) resolve(pkgs pkgMap, thisFile *File) (constDefSet, error) {
	return nil, nil
}

func (c *literalConst) eval() (Const, error) {
	return Const{c.lit, nil}, nil // All literal constants start out untyped.
}

// namedConst represents named references to other ConstDefs.
type namedConst struct {
	packageName string
	name        string
	p           Pos
	def         *ConstDef
}

func (c *namedConst) String() string {
	if c.packageName == "" {
		return c.name
	}
	return c.packageName + "." + c.name
}

func (c *namedConst) pos() Pos {
	return c.p
}

func (c *namedConst) resolve(pkgs pkgMap, thisFile *File) (constDefSet, error) {
	if c.def != nil {
		return nil, fmt.Errorf("internal error, resolved const %s multiple times", c.String())
	}
	if c.def = c.lookupConstDef(pkgs, thisFile); c.def == nil {
		return nil, fmt.Errorf("can't resolve const %s", c.String())
	}
	return constDefSet{c.def: true}, nil
}

func (c *namedConst) lookupConstDef(pkgs pkgMap, thisFile *File) *ConstDef {
	if c.packageName == "" {
		// The name must be in this package.
		return thisFile.Package.constResolver[c.name]
	}
	// Translate the package name into its path and lookup in pkgs.
	pkgPath := thisFile.LookupImportPath(c.packageName)
	if pkgPath == "" {
		return nil
	}
	if pkg, pkgExists := pkgs[pkgPath]; pkgExists {
		return pkg.constResolver[c.name]
	}
	return nil
}

func (c *namedConst) eval() (Const, error) {
	// Simply return the const value from the resolved def.
	if c.def == nil || !c.def.Const.IsValid() {
		return Const{}, fmt.Errorf("internal error, invalid const")
	}
	return c.def.Const, nil
}

// typeConvConst represents explicit type conversions.
type typeConvConst struct {
	t    *NamedType
	expr constExpr
	p    Pos
}

func (c *typeConvConst) String() string {
	return c.t.Name() + "(" + c.expr.String() + ")"
}

func (c *typeConvConst) pos() Pos {
	return c.p
}

func (c *typeConvConst) resolve(pkgs pkgMap, thisFile *File) (constDefSet, error) {
	// Resolve the type of the type conversion.  This must occur after all types
	// in this package and dependent packages have been defined.
	if err := c.t.resolve(pkgs, thisFile); err != nil {
		return nil, err
	}
	return c.expr.resolve(pkgs, thisFile)
}

func (c *typeConvConst) eval() (Const, error) {
	cv, err := c.expr.eval()
	if err != nil {
		return Const{}, err
	}
	return makeConst(cv.Val, c.t.Def(), fmt.Sprintf("%v invalid type conversion", c.p))
}

// unaryOpConst represents all unary operations.
type unaryOpConst struct {
	op   string
	expr constExpr
	p    Pos
}

func (c *unaryOpConst) String() string {
	return c.op + c.expr.String()
}

func (c *unaryOpConst) pos() Pos {
	return c.p
}

func (c *unaryOpConst) resolve(pkgs pkgMap, thisFile *File) (constDefSet, error) {
	return c.expr.resolve(pkgs, thisFile)
}

func (c *unaryOpConst) eval() (Const, error) {
	cv, err := c.expr.eval()
	if err != nil {
		return Const{}, err
	}
	errmsg := fmt.Sprintf("%v invalid unary %s %q", c.p, unaryOpToString(c.op), c.op)
	switch c.op {
	case "!":
		switch tv := cv.Val.(type) {
		case bool:
			return makeConst(!tv, cv.TypeDef, errmsg)
		}
	case "+":
		switch cv.Val.(type) {
		case *big.Int, *big.Rat, bigCmplx:
			return cv, nil
		}
	case "-":
		switch tv := cv.Val.(type) {
		case *big.Int:
			return makeConst(new(big.Int).Neg(tv), cv.TypeDef, errmsg)
		case *big.Rat:
			return makeConst(new(big.Rat).Neg(tv), cv.TypeDef, errmsg)
		case bigCmplx:
			return makeConst(bigCmplx{
				re: new(big.Rat).Neg(tv.re),
				im: new(big.Rat).Neg(tv.im),
			}, cv.TypeDef, errmsg)
		}
	case "^":
		switch tv := cv.Val.(type) {
		case *big.Int:
			return makeConst(new(big.Int).Not(tv), cv.TypeDef, errmsg)
		case *big.Rat:
			iv, err := bigRatToInt(tv, cv.TypeDef, errmsg)
			if err != nil {
				return Const{}, err
			}
			return makeConst(iv.Not(iv), cv.TypeDef, errmsg)
		}
	}
	return Const{}, fmt.Errorf("%s on type %s", errmsg, cv.TypeString())
}

// binaryOpConst represents all binary operations.
type binaryOpConst struct {
	op  string
	lhs constExpr
	rhs constExpr
	p   Pos
}

func (c *binaryOpConst) String() string {
	return "(" + c.lhs.String() + c.op + c.rhs.String() + ")"
}

func (c *binaryOpConst) pos() Pos {
	return c.p
}

func (c *binaryOpConst) resolve(pkgs pkgMap, thisFile *File) (constDefSet, error) {
	deps1, err := c.lhs.resolve(pkgs, thisFile)
	if err != nil {
		return nil, err
	}
	deps2, err := c.rhs.resolve(pkgs, thisFile)
	if err != nil {
		return nil, err
	}
	return mergeConstDefSets(deps1, deps2), nil
}

func (c *binaryOpConst) eval() (Const, error) {
	cl, err := c.lhs.eval()
	if err != nil {
		return Const{}, err
	}
	// TODO(toddw): If we want to support short-circuit logical evaluation we'll
	// need to delay the rhs eval.  If we add short-circuiting we should also
	// consider adding conditional (a ? b : c) expressions.
	cr, err := c.rhs.eval()
	if err != nil {
		return Const{}, err
	}
	errmsg := fmt.Sprintf("%v invalid binary %s %q", c.p, binaryOpToString(c.op), c.op)
	if c.op == "<<" || c.op == ">>" {
		// Shift ops are special since they require an integer lhs and unsigned rhs.
		return evalShift(c.op, cl, cr, errmsg)
	}
	// All other binary ops behave similarly.  First we perform implicit
	// conversion of the lhs and rhs.  If either side is untyped, we may need to
	// implicitly convert it to the type of the other side.  If both sides are
	// typed they need to match.  The resulting lv and rv are guaranteed to have
	// the same type, and restype tells us which type we need to convert the
	// result into when we're done.
	lv, rv, restype, err := implicitConvertConstVals(cl, cr, errmsg)
	if err != nil {
		return Const{}, err
	}
	// Now we perform the actual binary op.
	var resv interface{}
	switch c.op {
	case "||", "&&":
		resv, err = opLogic(c.op, lv, rv, restype, errmsg)
	case "==", "!=", "<", "<=", ">", ">=":
		resv, err = opComp(c.op, lv, rv, restype, errmsg)
		restype = nil // comparisons always result in untyped bool.
	case "+", "-", "*", "/":
		resv, err = opArith(c.op, lv, rv, restype, errmsg)
	case "%", "|", "&", "^":
		resv, err = opIntArith(c.op, lv, rv, restype, errmsg)
	default:
		panic(fmt.Errorf("idl: unhandled binary op %q", c.op))
	}
	if err != nil {
		return Const{}, err
	}
	// As a final step we convert to the result type.
	return makeConst(resv, restype, errmsg)
}

func opLogic(op string, l, r interface{}, restype *TypeDef, errmsg string) (interface{}, error) {
	switch tl := l.(type) {
	case bool:
		switch op {
		case "||":
			return tl || r.(bool), nil
		case "&&":
			return tl && r.(bool), nil
		}
	}
	return nil, fmt.Errorf("%s on type %s", errmsg, cvTypeString(restype, l))
}

func opComp(op string, l, r interface{}, restype *TypeDef, errmsg string) (interface{}, error) {
	switch tl := l.(type) {
	case bool:
		switch op {
		case "==":
			return tl == r.(bool), nil
		case "!=":
			return tl != r.(bool), nil
		}
	case string:
		return compString(op, tl, r.(string)), nil
	case *big.Int:
		return opCmpToBool(op, tl.Cmp(r.(*big.Int))), nil
	case *big.Rat:
		return opCmpToBool(op, tl.Cmp(r.(*big.Rat))), nil
	case bigCmplx:
		switch op {
		case "==":
			return tl.Equal(r.(bigCmplx)), nil
		case "!=":
			return !tl.Equal(r.(bigCmplx)), nil
		}
	}
	return nil, fmt.Errorf("%s on type %s", errmsg, cvTypeString(restype, l))
}

func opArith(op string, l, r interface{}, restype *TypeDef, errmsg string) (interface{}, error) {
	switch tl := l.(type) {
	case string:
		if op == "+" {
			return tl + r.(string), nil
		}
	case *big.Int:
		return arithBigInt(op, tl, r.(*big.Int), errmsg)
	case *big.Rat:
		return arithBigRat(op, tl, r.(*big.Rat), errmsg)
	case bigCmplx:
		return arithBigCmplx(op, tl, r.(bigCmplx), errmsg)
	}
	return nil, fmt.Errorf("%s on type %s", errmsg, cvTypeString(restype, l))
}

func opIntArith(op string, l, r interface{}, restype *TypeDef, errmsg string) (interface{}, error) {
	switch tl := l.(type) {
	case *big.Int:
		return arithBigInt(op, tl, r.(*big.Int), errmsg)
	case *big.Rat:
		il, err := bigRatToInt(tl, restype, errmsg)
		if err != nil {
			return nil, err
		}
		ir, err := bigRatToInt(r.(*big.Rat), restype, errmsg)
		if err != nil {
			return nil, err
		}
		return arithBigInt(op, il, ir, errmsg)
	}
	return nil, fmt.Errorf("%s on type %s", errmsg, cvTypeString(restype, l))
}

func evalShift(op string, cl, cr Const, errmsg string) (Const, error) {
	// lhs must be an integer.
	bl, err := constToInt(cl, errmsg)
	if err != nil {
		return Const{}, err
	}
	// rhs must be a small unsigned integer.
	smsg := errmsg + " shift amount"
	br, err := constToInt(cr, smsg)
	if err != nil {
		return Const{}, err
	}
	if br.Sign() < 0 {
		return Const{}, fmt.Errorf("%s, value %v isn't unsigned", smsg, cvString(br))
	}
	if br.Cmp(maxShiftSize) > 0 {
		return Const{}, fmt.Errorf("%s, value %v greater than max allowed %v", smsg, cvString(br), cvString(maxShiftSize))
	}
	// Perform the shift.
	resv, err := shiftBigInt(op, bl, uint(br.Uint64()), errmsg)
	if err != nil {
		return Const{}, err
	}
	// Convert it back to the lhs type.
	return makeConst(resv, cl.TypeDef, errmsg)
}

// constResolver converts a string const identifier into a resolved ConstDef.
type constResolver map[string]*ConstDef

// ConstDef represents a const definition.  After parsing ConstDef contains an
// invalid Const, with expr filled in with the const expression.  After a
// sequence of declare, resolve and define the Const is filled in with the final
// evaluated value.
type ConstDef struct {
	Name      string // Name of the constdef.
	Const     Const  // Type and value of the constdef.
	Pos       Pos    // Position of occurrence in idl source.
	File      *File  // File the constdef occurs in.
	Doc       string // Documentation string (before the def).
	DocSuffix string // Trailing same-line documentation (after the def).

	expr constExpr
}

type constDefSet map[*ConstDef]bool

// mergeConstDefSets returns the union of a and b.  It may mutate either a or b
// and return the mutated set as a result.
func mergeConstDefSets(a, b constDefSet) (r constDefSet) {
	if a != nil {
		for def, _ := range b {
			a[def] = true
		}
		return a
	}
	return b
}

func (def *ConstDef) String() string {
	str := "(" + def.Pos.String() + " " + def.Name
	if def.Const.IsValid() {
		str += " " + def.Const.String()
	} else {
		str += " " + def.expr.String()
	}
	return str + ")"
}

// declare declares the const definition in its package - it must be called
// before resolve.
func (def *ConstDef) declare() error {
	if exist := def.File.Package.constResolver[def.Name]; exist != nil {
		return fmt.Errorf("already defined at %v:%v", exist.File.BaseName, exist.Pos)
	}
	def.File.Package.constResolver[def.Name] = def
	return nil
}

// resolve resolves the const definition expression - it must be called after
// declare and before define.
func (def *ConstDef) resolve(pkgs pkgMap) (constDefSet, error) {
	if exist := def.File.Package.constResolver[def.Name]; exist != def {
		return nil, fmt.Errorf("internal error, type declared with wrong name")
	}
	return def.expr.resolve(pkgs, def.File)
}

// define evalutes the const definition expression and sets the Const value - it
// must be called after resolve.
func (def *ConstDef) define() error {
	if exist := def.File.Package.constResolver[def.Name]; exist != def {
		return fmt.Errorf("internal error, type declared with wrong name")
	}
	var err error
	if def.Const, err = def.expr.eval(); err != nil {
		return err
	}
	if err = def.Const.FinalCheck(); err != nil {
		return err
	}
	return nil
}
