// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
package query

import (
	"errors"
	"fmt"
	"math/big"
	"reflect"
	"strconv"

	"v.io/syncbase/v23/syncbase/nosql/internal/query/query_checker"
	"v.io/syncbase/v23/syncbase/nosql/internal/query/query_parser"
)

func Eval(k string, v interface{}, e *query_parser.Expression) bool {
	if query_checker.IsLogicalOperator(e.Operator) {
		return EvalLogicalOperators(k, v, e)
	} else {
		return EvalComparisonOperators(k, v, e)
	}
}

func EvalLogicalOperators(k string, v interface{}, e *query_parser.Expression) bool {
	switch e.Operator.Type {
	case query_parser.And:
		return Eval(k, v, e.Operand1.Expr) && Eval(k, v, e.Operand2.Expr)
	default: // query_parser.Or
		return Eval(k, v, e.Operand1.Expr) || Eval(k, v, e.Operand2.Expr)
	}
}

func EvalComparisonOperators(k string, v interface{}, e *query_parser.Expression) bool {
	// Key and type expressions are evaluated differently from value expression.
	// Key expressions always have a string literal on the rhs and are currently
	// limited to = and <>.
	// Type expressions are limited to = and must have a stiring literal rhs.  Also,
	// they are evaluated via reflection on the value.
	if query_checker.IsKey(e.Operand1) {
		return EvalKeyExpression(e, k)
	} else if query_checker.IsType(e.Operand1) {
		return EvalTypeExpression(e, v)
	} else {
		return EvalValueExpression(k, v, e)
	}
}

func EvalValueExpression(k string, v interface{}, e *query_parser.Expression) bool {
	// Any fields that are value fields that need to be resolved.
	lhsValue := ResolveOperand(v, e.Operand1)
	if lhsValue == nil {
		return false
	}
	rhsValue := ResolveOperand(v, e.Operand2)
	if rhsValue == nil {
		return false
	}
	// Coerce operands so they are comparable
	var err error
	lhsValue, rhsValue, err = CoerceValues(lhsValue, rhsValue)
	if err != nil {
		return false // If operands can't be coerced to compare, expr evals to false.
	}
	// Do the compare
	switch lhsValue.Type {
	case query_parser.TypBigInt:
		return CompareBigInts(lhsValue, rhsValue, e.Operator)
	case query_parser.TypBigRat:
		return CompareBigRats(lhsValue, rhsValue, e.Operator)
	case query_parser.TypBool:
		return CompareBools(lhsValue, rhsValue, e.Operator)
	case query_parser.TypFloat:
		return CompareFloats(lhsValue, rhsValue, e.Operator)
	case query_parser.TypInt:
		return CompareInts(lhsValue, rhsValue, e.Operator)
	case query_parser.TypLiteral:
		return CompareStrings(lhsValue, rhsValue, e.Operator)
	case query_parser.TypUint:
		return CompareUints(lhsValue, rhsValue, e.Operator)
	case query_parser.TypObject:
		return CompareObjects(lhsValue, rhsValue, e.Operator)
	}
	return false
}

func CoerceValues(lhsValue, rhsValue *query_parser.Operand) (*query_parser.Operand, *query_parser.Operand, error) {
	var err error
	// If either operand is a string, convert the other to a string.
	if lhsValue.Type == query_parser.TypLiteral || rhsValue.Type == query_parser.TypLiteral {
		if lhsValue, err = ConvertValueToString(lhsValue); err != nil {
			return nil, nil, err
		}
		if rhsValue, err = ConvertValueToString(rhsValue); err != nil {
			return nil, nil, err
		}
	}
	// If either operand is a big rat, convert both to a big rat.
	// Also, if one operand is a float and the other is a big int,
	// convert both to big rats.
	if lhsValue.Type == query_parser.TypBigRat || rhsValue.Type == query_parser.TypBigRat || (lhsValue.Type == query_parser.TypBigInt && rhsValue.Type == query_parser.TypFloat) || (lhsValue.Type == query_parser.TypFloat && rhsValue.Type == query_parser.TypBigInt) {
		if lhsValue, err = ConvertValueToBigRat(lhsValue); err != nil {
			return nil, nil, err
		}
		if rhsValue, err = ConvertValueToBigRat(rhsValue); err != nil {
			return nil, nil, err
		}
	}
	// If either operand is a float, convert the other to a float.
	if lhsValue.Type == query_parser.TypFloat || rhsValue.Type == query_parser.TypFloat {
		if lhsValue, err = ConvertValueToFloat(lhsValue); err != nil {
			return nil, nil, err
		}
		if rhsValue, err = ConvertValueToFloat(rhsValue); err != nil {
			return nil, nil, err
		}
	}
	// If either operand is a big int, convert both to a big int.
	// Also, if one operand is a uint64 and the other is an int64, convert both to big ints.
	if lhsValue.Type == query_parser.TypBigInt || rhsValue.Type == query_parser.TypBigInt || (lhsValue.Type == query_parser.TypUint && rhsValue.Type == query_parser.TypInt) || (lhsValue.Type == query_parser.TypInt && rhsValue.Type == query_parser.TypUint) {
		if lhsValue, err = ConvertValueToBigInt(lhsValue); err != nil {
			return nil, nil, err
		}
		if rhsValue, err = ConvertValueToBigInt(rhsValue); err != nil {
			return nil, nil, err
		}
	}
	// If either operand is an int64, convert the other to int64.
	if lhsValue.Type == query_parser.TypInt || rhsValue.Type == query_parser.TypInt {
		if lhsValue, err = ConvertValueToInt(lhsValue); err != nil {
			return nil, nil, err
		}
		if rhsValue, err = ConvertValueToInt(rhsValue); err != nil {
			return nil, nil, err
		}
	}
	// If either operand is an uint64, convert the other to uint64.
	if lhsValue.Type == query_parser.TypUint || rhsValue.Type == query_parser.TypUint {
		if lhsValue, err = ConvertValueToUint(lhsValue); err != nil {
			return nil, nil, err
		}
		if rhsValue, err = ConvertValueToUint(rhsValue); err != nil {
			return nil, nil, err
		}
	}
	// Must be the same at this point.
	if lhsValue.Type != rhsValue.Type {
		return nil, nil, errors.New(fmt.Sprintf("Logic error: expeced like types, got: %v, %v", lhsValue, rhsValue))
	}

	return lhsValue, rhsValue, nil
}

func ConvertValueToString(o *query_parser.Operand) (*query_parser.Operand, error) {
	var c query_parser.Operand
	c.Type = query_parser.TypLiteral
	switch o.Type {
	case query_parser.TypBigInt:
		c.Literal = o.BigInt.String()
	case query_parser.TypBigRat:
		c.Literal = o.BigRat.String()
	case query_parser.TypBool:
		c.Literal = strconv.FormatBool(o.Bool)
	case query_parser.TypFloat:
		c.Literal = strconv.FormatFloat(o.Float, 'f', -1, 64)
	case query_parser.TypInt:
		c.Literal = strconv.FormatInt(o.Int, 10)
	case query_parser.TypLiteral:
		c.Literal = o.Literal
		c.Regex = o.Regex         // non-nil for rhs of like expressions
		c.CompRegex = o.CompRegex // non-nil for rhs of like expressions
	case query_parser.TypUint:
		c.Literal = strconv.FormatUint(o.Uint, 10)
	default: // query_parser.TypObject
		return nil, errors.New("Cannot convert object to string for comparison.")
	}
	return &c, nil
}

func ConvertValueToBigRat(o *query_parser.Operand) (*query_parser.Operand, error) {
	// operand cannot be literal.
	var c query_parser.Operand
	c.Type = query_parser.TypBigRat
	switch o.Type {
	case query_parser.TypBigInt:
		var b big.Rat
		c.BigRat = b.SetInt(o.BigInt)
	case query_parser.TypBigRat:
		c.BigRat = o.BigRat
	case query_parser.TypBool:
		return nil, errors.New("Cannot convert bool to big.Rat for comparison.")
	case query_parser.TypFloat:
		var b big.Rat
		c.BigRat = b.SetFloat64(o.Float)
	case query_parser.TypInt:
		c.BigRat = big.NewRat(o.Int, 1)
	case query_parser.TypUint:
		var bi big.Int
		bi.SetUint64(o.Uint)
		var br big.Rat
		c.BigRat = br.SetInt(&bi)
	default: // query_parser.TypObject
		return nil, errors.New("Cannot convert object to big.Rat for comparison.")
	}
	return &c, nil
}

func ConvertValueToFloat(o *query_parser.Operand) (*query_parser.Operand, error) {
	// Operand cannot be literal, big.Rat or big.Int
	var c query_parser.Operand
	c.Type = query_parser.TypFloat
	switch o.Type {
	case query_parser.TypBool:
		return nil, errors.New("Cannot convert bool to float64 for comparison.")
	case query_parser.TypFloat:
		c.Float = o.Float
	case query_parser.TypInt:
		c.Float = float64(o.Int)
	case query_parser.TypUint:
		c.Float = float64(o.Uint)
	default: // query_parser.TypObject
		return nil, errors.New("Cannot convert object to float64 for comparison.")
	}
	return &c, nil
}

func ConvertValueToBigInt(o *query_parser.Operand) (*query_parser.Operand, error) {
	// Operand cannot be literal, big.Rat or float.
	var c query_parser.Operand
	c.Type = query_parser.TypBigInt
	switch o.Type {
	case query_parser.TypBigInt:
		c.BigInt = o.BigInt
	case query_parser.TypBool:
		return nil, errors.New("Cannot convert bool to big.Int for comparison.")
	case query_parser.TypInt:
		c.BigInt = big.NewInt(o.Int)
	case query_parser.TypUint:
		var b big.Int
		b.SetUint64(o.Uint)
		c.BigInt = &b
	default: // case query_parser.TypObject
		return nil, errors.New("Cannot convert object to big.Int for comparison.")
	}
	return &c, nil
}

func ConvertValueToInt(o *query_parser.Operand) (*query_parser.Operand, error) {
	// Operand cannot be literal, big.Rat or float or uint64.
	var c query_parser.Operand
	c.Type = query_parser.TypInt
	switch o.Type {
	case query_parser.TypBool:
		return nil, errors.New("Cannot convert bool to int64 for comparison.")
	case query_parser.TypInt:
		c.Int = o.Int
	default: //case query_parser.TypObject
		return nil, errors.New("Cannot convert object to int64 for comparison.")
	}
	return &c, nil
}

func ConvertValueToUint(o *query_parser.Operand) (*query_parser.Operand, error) {
	// Operand cannot be literal, big.Rat or float or int64.
	var c query_parser.Operand
	c.Type = query_parser.TypUint
	switch o.Type {
	case query_parser.TypBool:
		return nil, errors.New("Cannot convert bool to int64 for comparison.")
	case query_parser.TypUint:
		c.Uint = o.Uint
	default: //case query_parser.TypObject
		return nil, errors.New("Cannot convert object to int64 for comparison.")
	}
	return &c, nil
}

func CompareBools(lhsValue, rhsValue *query_parser.Operand, oper *query_parser.BinaryOperator) bool {
	switch oper.Type {
	case query_parser.Equal:
		return lhsValue.Bool == rhsValue.Bool
	default: // query_parser.NotEqual
		return lhsValue.Bool != rhsValue.Bool
	}
}

func CompareBigInts(lhsValue, rhsValue *query_parser.Operand, oper *query_parser.BinaryOperator) bool {
	switch oper.Type {
	case query_parser.Equal:
		return lhsValue.BigInt.Cmp(rhsValue.BigInt) == 0
	case query_parser.NotEqual:
		return lhsValue.BigInt.Cmp(rhsValue.BigInt) != 0
	case query_parser.LessThan:
		return lhsValue.BigInt.Cmp(rhsValue.BigInt) < 0
	case query_parser.LessThanOrEqual:
		return lhsValue.BigInt.Cmp(rhsValue.BigInt) <= 0
	case query_parser.GreaterThan:
		return lhsValue.BigInt.Cmp(rhsValue.BigInt) > 0
	default: // case query_parser.GreaterThanOrEqual
		return lhsValue.BigInt.Cmp(rhsValue.BigInt) >= 0
	}
}

func CompareBigRats(lhsValue, rhsValue *query_parser.Operand, oper *query_parser.BinaryOperator) bool {
	switch oper.Type {
	case query_parser.Equal:
		return lhsValue.BigRat.Cmp(rhsValue.BigRat) == 0
	case query_parser.NotEqual:
		return lhsValue.BigRat.Cmp(rhsValue.BigRat) != 0
	case query_parser.LessThan:
		return lhsValue.BigRat.Cmp(rhsValue.BigRat) < 0
	case query_parser.LessThanOrEqual:
		return lhsValue.BigRat.Cmp(rhsValue.BigRat) <= 0
	case query_parser.GreaterThan:
		return lhsValue.BigRat.Cmp(rhsValue.BigRat) > 0
	default: // case query_parser.GreaterThanOrEqual
		return lhsValue.BigRat.Cmp(rhsValue.BigRat) >= 0
	}
}

func CompareFloats(lhsValue, rhsValue *query_parser.Operand, oper *query_parser.BinaryOperator) bool {
	switch oper.Type {
	case query_parser.Equal:
		return lhsValue.Float == rhsValue.Float
	case query_parser.NotEqual:
		return lhsValue.Float != rhsValue.Float
	case query_parser.LessThan:
		return lhsValue.Float < rhsValue.Float
	case query_parser.LessThanOrEqual:
		return lhsValue.Float <= rhsValue.Float
	case query_parser.GreaterThan:
		return lhsValue.Float > rhsValue.Float
	default: // case query_parser.GreaterThanOrEqual
		return lhsValue.Float >= rhsValue.Float
	}
}

func CompareInts(lhsValue, rhsValue *query_parser.Operand, oper *query_parser.BinaryOperator) bool {
	switch oper.Type {
	case query_parser.Equal:
		return lhsValue.Int == rhsValue.Int
	case query_parser.NotEqual:
		return lhsValue.Int != rhsValue.Int
	case query_parser.LessThan:
		return lhsValue.Int < rhsValue.Int
	case query_parser.LessThanOrEqual:
		return lhsValue.Int <= rhsValue.Int
	case query_parser.GreaterThan:
		return lhsValue.Int > rhsValue.Int
	default: // case query_parser.GreaterThanOrEqual
		return lhsValue.Int >= rhsValue.Int
	}
}

func CompareUints(lhsValue, rhsValue *query_parser.Operand, oper *query_parser.BinaryOperator) bool {
	switch oper.Type {
	case query_parser.Equal:
		return lhsValue.Uint == rhsValue.Uint
	case query_parser.NotEqual:
		return lhsValue.Uint != rhsValue.Uint
	case query_parser.LessThan:
		return lhsValue.Uint < rhsValue.Uint
	case query_parser.LessThanOrEqual:
		return lhsValue.Uint <= rhsValue.Uint
	case query_parser.GreaterThan:
		return lhsValue.Uint > rhsValue.Uint
	default: // case query_parser.GreaterThanOrEqual
		return lhsValue.Uint >= rhsValue.Uint
	}
}

func CompareStrings(lhsValue, rhsValue *query_parser.Operand, oper *query_parser.BinaryOperator) bool {
	switch oper.Type {
	case query_parser.Equal:
		return lhsValue.Literal == rhsValue.Literal
	case query_parser.NotEqual:
		return lhsValue.Literal != rhsValue.Literal
	case query_parser.LessThan:
		return lhsValue.Literal < rhsValue.Literal
	case query_parser.LessThanOrEqual:
		return lhsValue.Literal <= rhsValue.Literal
	case query_parser.GreaterThan:
		return lhsValue.Literal > rhsValue.Literal
	case query_parser.GreaterThanOrEqual:
		return lhsValue.Literal >= rhsValue.Literal
	case query_parser.Like:
		return rhsValue.CompRegex.MatchString(lhsValue.Literal)
	default: // query_parser.NotLike:
		return !rhsValue.CompRegex.MatchString(lhsValue.Literal)
	}
}

func CompareObjects(lhsValue, rhsValue *query_parser.Operand, oper *query_parser.BinaryOperator) bool {
	switch oper.Type {
	case query_parser.Equal:
		return reflect.DeepEqual(lhsValue.Object, rhsValue.Object)
	case query_parser.NotEqual:
		return !reflect.DeepEqual(lhsValue.Object, rhsValue.Object)
	default: // other operands are non-sensical
		return false
	}
}

func ResolveOperand(v interface{}, o *query_parser.Operand) *query_parser.Operand {
	if o.Type != query_parser.TypField {
		return o
	}
	value := ResolveField(v, o.Column)

	// Convert value to an operand
	var newOp query_parser.Operand
	newOp.Off = o.Off

	switch value := value.(type) {
	case bool:
		newOp.Type = query_parser.TypBool
		newOp.Bool = value
	case int:
		newOp.Type = query_parser.TypInt
		newOp.Int = int64(value)
	case int8:
		newOp.Type = query_parser.TypInt
		newOp.Int = int64(value)
	case int16:
		newOp.Type = query_parser.TypInt
		newOp.Int = int64(value)
	case int32: // rune
		newOp.Type = query_parser.TypInt
		newOp.Int = int64(value)
	case int64:
		newOp.Type = query_parser.TypInt
		newOp.Int = value
	case uint:
		newOp.Type = query_parser.TypBigInt
		var b big.Int
		b.SetUint64(uint64(value))
		newOp.BigInt = &b
	case uint8: // byte
		newOp.Type = query_parser.TypInt
		newOp.Int = int64(value)
	case uint16:
		newOp.Type = query_parser.TypInt
		newOp.Int = int64(value)
	case uint32:
		newOp.Type = query_parser.TypInt
		newOp.Int = int64(value)
	case uint64:
		newOp.Type = query_parser.TypBigInt
		var b big.Int
		b.SetUint64(value)
		newOp.BigInt = &b
	case string:
		newOp.Type = query_parser.TypLiteral
		newOp.Literal = value
	case *big.Int:
		newOp.Type = query_parser.TypBigInt
		newOp.BigInt = value
	case *big.Rat:
		newOp.Type = query_parser.TypBigRat
		newOp.BigRat = value
	default: // OpObject for structs, arrays, maps, ...
		newOp.Type = query_parser.TypObject
		newOp.Object = value
	}
	return &newOp
}

func ResolveField(v interface{}, f *query_parser.Field) interface{} {
	var object interface{}
	object = v
	segments := f.Segments
	// The first segment will always be v itself, skip it.
	for i := 1; i < len(segments); i++ {
		// object must be a struct in order to look for the next segment.
		if reflect.ValueOf(object).Kind() != reflect.Struct {
			return nil // field does not exist
		}
		// Look up the segment in object.
		_, ok := reflect.ValueOf(object).Type().FieldByName(segments[i].Value)
		if !ok {
			return nil // field does not exist
		}
		if !reflect.ValueOf(object).FieldByName(segments[i].Value).CanInterface() {
			return nil
		}
		object = reflect.ValueOf(object).FieldByName(segments[i].Value).Interface()
	}
	return object
}

// Evaluate an expression where the first operand refers to the key.
func EvalKeyExpression(e *query_parser.Expression, k string) bool {
	// Need to evaluate the key expression.
	// Currently, only = and like are allowed.
	// Operand2 must be a string literal.
	switch e.Operator.Type {
	case query_parser.Equal:
		return k == e.Operand2.Literal
	default: // query_parse.Like
		return e.Operand2.CompRegex.MatchString(k)
	}
}

func EvalTypeExpression(e *query_parser.Expression, v interface{}) bool {
	if v == nil {
		// The type expression does not match.
		return false
	}
	// First try to match on the full type.
	if reflect.ValueOf(v).Type().PkgPath()+"."+reflect.ValueOf(v).Type().Name() == e.Operand2.Literal {
		return true
	}
	// Try to match on just the name.
	return reflect.ValueOf(v).Type().Name() == e.Operand2.Literal
}
