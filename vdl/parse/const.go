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

// ConstLit represents scalar literals in const expressions.  The supported
// types for Lit are:
//   bool       - Represents all boolean constants.
//   string     - Represents all string constants.
//   *big.Int   - Represents all integer constants.
//   *big.Rat   - Represents all rational constants.
//   *BigImag   - Represents all imaginary constants.
type ConstLit struct {
	Lit interface{}
	P   Pos
}

// BigImag represents a literal imaginary number.
type BigImag big.Rat

// ConstCompositeLit represents composite literals in const expressions.
type ConstCompositeLit struct {
	Type   Type
	KVList []KVLit
	P      Pos
}

// KVLit represents a key/value literal in composite literals.
type KVLit struct {
	Key   ConstExpr
	Value ConstExpr
}

// ConstNamed represents named references to other consts.
type ConstNamed struct {
	Name string
	P    Pos
}

// ConstTypeConv represents explicit type conversions.
type ConstTypeConv struct {
	Type Type
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
		return strconv.Quote(tv)
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
		panic(fmt.Errorf("vdl: unhandled const type %T value %v", val, val))
	}
}

func (c *ConstLit) String() string {
	return cvString(c.Lit)
}
func (c *ConstCompositeLit) String() string {
	var s string
	if c.Type != nil {
		s += c.Type.String()
	}
	s += "{"
	for index, kv := range c.KVList {
		if index > 0 {
			s += ", "
		}
		if kv.Key != nil {
			s += kv.Key.String() + ": "
		}
		s += kv.Value.String()
	}
	return s + "}"
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

func (c *ConstLit) Pos() Pos          { return c.P }
func (c *ConstCompositeLit) Pos() Pos { return c.P }
func (c *ConstNamed) Pos() Pos        { return c.P }
func (c *ConstTypeConv) Pos() Pos     { return c.P }
func (c *ConstUnaryOp) Pos() Pos      { return c.P }
func (c *ConstBinaryOp) Pos() Pos     { return c.P }
