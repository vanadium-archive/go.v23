package parse

import (
	"fmt"
	"math/big"
	"strconv"
)

// ConstExpr is the interface for all nodes in an expression.
type ConstExpr interface {
	String() string
	Pos() Pos
}

// ConstLit represents literals in const expressions.  The supported types for
// Lit are:
//   bool     - Represents all boolean constants.
//   string   - Represents all string constants.
//   *big.Int - Represents all integer constants.
//   *big.Rat - Represents all rational constants.
//   *BigImag - Represents all imaginary constants.
type ConstLit struct {
	Lit interface{}
	P   Pos
}

// BigImag represents a literal imaginary number.
type BigImag big.Rat

// ConstNamed represents named references to other consts.
type ConstNamed struct {
	Name string
	P    Pos
}

// ConstTypeConv represents explicit type conversions.
type ConstTypeConv struct {
	Type *TypeNamed
	Expr ConstExpr
	P    Pos
}

// ConstUnaryOp represents all unary operations.
type ConstUnaryOp struct {
	Op   string
	Expr ConstExpr
	P    Pos
}

// ConstBinaryOp represents all binary operations.
type ConstBinaryOp struct {
	Op    string
	Lexpr ConstExpr
	Rexpr ConstExpr
	P     Pos
}

// ConstDef represents a user-defined named const.
type ConstDef struct {
	NamePos
	Expr ConstExpr
}

// cvString returns a human-readable string representing the const value.
func cvString(val interface{}) string {
	switch tv := val.(type) {
	case bool:
		if tv {
			return "true"
		}
		return "false"
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
	case *BigImag:
		return cvString((*big.Rat)(tv)) + "i"
	default:
		panic(fmt.Errorf("idl: unhandled const type %T value %v", val, val))
	}
}

func (c *ConstLit) String() string {
	return cvString(c.Lit)
}
func (c *ConstNamed) String() string {
	return c.Name
}
func (c *ConstTypeConv) String() string {
	return c.Type.String() + "(" + c.Expr.String() + ")"
}
func (c *ConstUnaryOp) String() string {
	return c.Op + c.Expr.String()
}
func (c *ConstBinaryOp) String() string {
	return "(" + c.Lexpr.String() + c.Op + c.Rexpr.String() + ")"
}

func (c *ConstLit) Pos() Pos      { return c.P }
func (c *ConstNamed) Pos() Pos    { return c.P }
func (c *ConstTypeConv) Pos() Pos { return c.P }
func (c *ConstUnaryOp) Pos() Pos  { return c.P }
func (c *ConstBinaryOp) Pos() Pos { return c.P }
