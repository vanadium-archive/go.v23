// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package query

import (
	"errors"
	"fmt"
	"reflect"

	"v.io/syncbase/v23/syncbase/nosql/internal/query/conversions"
	"v.io/syncbase/v23/syncbase/nosql/internal/query/query_checker"
	"v.io/syncbase/v23/syncbase/nosql/internal/query/query_db"
	"v.io/syncbase/v23/syncbase/nosql/internal/query/query_functions"
	"v.io/syncbase/v23/syncbase/nosql/internal/query/query_parser"
	"v.io/syncbase/v23/syncbase/nosql/syncql"
	"v.io/v23/vdl"
)

func Eval(db query_db.Database, k string, v *vdl.Value, e *query_parser.Expression) bool {
	if query_checker.IsLogicalOperator(e.Operator) {
		return evalLogicalOperators(db, k, v, e)
	} else {
		return evalComparisonOperators(db, k, v, e)
	}
}

func evalLogicalOperators(db query_db.Database, k string, v *vdl.Value, e *query_parser.Expression) bool {
	switch e.Operator.Type {
	case query_parser.And:
		return Eval(db, k, v, e.Operand1.Expr) && Eval(db, k, v, e.Operand2.Expr)
	case query_parser.Or:
		return Eval(db, k, v, e.Operand1.Expr) || Eval(db, k, v, e.Operand2.Expr)
	default:
		// TODO(jkline): Log this logic error and all other similar cases.
		return false
	}
}

func evalComparisonOperators(db query_db.Database, k string, v *vdl.Value, e *query_parser.Expression) bool {
	lhsValue := resolveOperand(db, k, v, e.Operand1)
	// Check for an is nil epression (i.e., v[.<field>...] is nil).
	// These expressions evaluate to true if the field cannot be resolved.
	if e.Operator.Type == query_parser.Is && e.Operand2.Type == query_parser.TypNil {
		return lhsValue == nil
	}
	if e.Operator.Type == query_parser.IsNot && e.Operand2.Type == query_parser.TypNil {
		return lhsValue != nil
	}
	// For anything but "is[not] nil" (which is handled above), an unresolved operator
	// results in the expression evaluating to false.
	if lhsValue == nil {
		return false
	}
	rhsValue := resolveOperand(db, k, v, e.Operand2)
	if rhsValue == nil {
		return false
	}
	// coerce operands so they are comparable
	var err error
	lhsValue, rhsValue, err = coerceValues(lhsValue, rhsValue)
	if err != nil {
		return false // If operands can't be coerced to compare, expr evals to false.
	}
	// Do the compare
	switch lhsValue.Type {
	case query_parser.TypBigInt:
		return compareBigInts(lhsValue, rhsValue, e.Operator)
	case query_parser.TypBigRat:
		return compareBigRats(lhsValue, rhsValue, e.Operator)
	case query_parser.TypBool:
		return compareBools(lhsValue, rhsValue, e.Operator)
	case query_parser.TypComplex:
		return compareComplex(lhsValue, rhsValue, e.Operator)
	case query_parser.TypFloat:
		return compareFloats(lhsValue, rhsValue, e.Operator)
	case query_parser.TypInt:
		return compareInts(lhsValue, rhsValue, e.Operator)
	case query_parser.TypStr:
		return compareStrings(lhsValue, rhsValue, e.Operator)
	case query_parser.TypUint:
		return compareUints(lhsValue, rhsValue, e.Operator)
	case query_parser.TypTime:
		return compareTimes(lhsValue, rhsValue, e.Operator)
	case query_parser.TypObject:
		return compareObjects(lhsValue, rhsValue, e.Operator)
	}
	return false
}

func coerceValues(lhsValue, rhsValue *query_parser.Operand) (*query_parser.Operand, *query_parser.Operand, error) {
	// TODO(jkline): explore using vdl for coercions ( https://v.io/designdocs/vdl-spec.html#conversions ).
	var err error
	// If either operand is a string, convert the other to a string.
	if lhsValue.Type == query_parser.TypStr || rhsValue.Type == query_parser.TypStr {
		if lhsValue, err = conversions.ConvertValueToString(lhsValue); err != nil {
			return nil, nil, err
		}
		if rhsValue, err = conversions.ConvertValueToString(rhsValue); err != nil {
			return nil, nil, err
		}
		return lhsValue, rhsValue, nil
	}
	// If either operand is Complex, promote numerics to Complex.
	// Comparing complex to string is handled above.
	if lhsValue.Type == query_parser.TypComplex || rhsValue.Type == query_parser.TypComplex {
		// If both complex, just return them.
		if lhsValue.Type == query_parser.TypComplex && rhsValue.Type == query_parser.TypComplex {
			return lhsValue, rhsValue, nil
		}
		var err error
		if lhsValue, err = conversions.ConvertValueToComplex(lhsValue); err != nil {
			return nil, nil, err
		}
		if rhsValue, err = conversions.ConvertValueToComplex(rhsValue); err != nil {
			return nil, nil, err
		}
		return lhsValue, rhsValue, nil
	}
	// If either operand is a big rat, convert both to a big rat.
	// Also, if one operand is a float and the other is a big int,
	// convert both to big rats.
	if lhsValue.Type == query_parser.TypBigRat || rhsValue.Type == query_parser.TypBigRat || (lhsValue.Type == query_parser.TypBigInt && rhsValue.Type == query_parser.TypFloat) || (lhsValue.Type == query_parser.TypFloat && rhsValue.Type == query_parser.TypBigInt) {
		if lhsValue, err = conversions.ConvertValueToBigRat(lhsValue); err != nil {
			return nil, nil, err
		}
		if rhsValue, err = conversions.ConvertValueToBigRat(rhsValue); err != nil {
			return nil, nil, err
		}
		return lhsValue, rhsValue, nil
	}
	// If either operand is a float, convert the other to a float.
	if lhsValue.Type == query_parser.TypFloat || rhsValue.Type == query_parser.TypFloat {
		if lhsValue, err = conversions.ConvertValueToFloat(lhsValue); err != nil {
			return nil, nil, err
		}
		if rhsValue, err = conversions.ConvertValueToFloat(rhsValue); err != nil {
			return nil, nil, err
		}
		return lhsValue, rhsValue, nil
	}
	// If either operand is a big int, convert both to a big int.
	// Also, if one operand is a uint64 and the other is an int64, convert both to big ints.
	if lhsValue.Type == query_parser.TypBigInt || rhsValue.Type == query_parser.TypBigInt || (lhsValue.Type == query_parser.TypUint && rhsValue.Type == query_parser.TypInt) || (lhsValue.Type == query_parser.TypInt && rhsValue.Type == query_parser.TypUint) {
		if lhsValue, err = conversions.ConvertValueToBigInt(lhsValue); err != nil {
			return nil, nil, err
		}
		if rhsValue, err = conversions.ConvertValueToBigInt(rhsValue); err != nil {
			return nil, nil, err
		}
		return lhsValue, rhsValue, nil
	}
	// If either operand is an int64, convert the other to int64.
	if lhsValue.Type == query_parser.TypInt || rhsValue.Type == query_parser.TypInt {
		if lhsValue, err = conversions.ConvertValueToInt(lhsValue); err != nil {
			return nil, nil, err
		}
		if rhsValue, err = conversions.ConvertValueToInt(rhsValue); err != nil {
			return nil, nil, err
		}
		return lhsValue, rhsValue, nil
	}
	// If either operand is an uint64, convert the other to uint64.
	if lhsValue.Type == query_parser.TypUint || rhsValue.Type == query_parser.TypUint {
		if lhsValue, err = conversions.ConvertValueToUint(lhsValue); err != nil {
			return nil, nil, err
		}
		if rhsValue, err = conversions.ConvertValueToUint(rhsValue); err != nil {
			return nil, nil, err
		}
		return lhsValue, rhsValue, nil
	}
	// Must be the same at this point.
	if lhsValue.Type != rhsValue.Type {
		return nil, nil, errors.New(fmt.Sprintf("Logic error: expected like types, got: %v, %v", lhsValue, rhsValue))
	}

	return lhsValue, rhsValue, nil
}

func compareBools(lhsValue, rhsValue *query_parser.Operand, oper *query_parser.BinaryOperator) bool {
	switch oper.Type {
	case query_parser.Equal:
		return lhsValue.Bool == rhsValue.Bool
	case query_parser.NotEqual:
		return lhsValue.Bool != rhsValue.Bool
	default:
		// TODO(jkline): Log this logic error and all other similar cases.
		return false
	}
}

func compareBigInts(lhsValue, rhsValue *query_parser.Operand, oper *query_parser.BinaryOperator) bool {
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
	case query_parser.GreaterThanOrEqual:
		return lhsValue.BigInt.Cmp(rhsValue.BigInt) >= 0
	default:
		// TODO(jkline): Log this logic error and all other similar cases.
		return false
	}
}

func compareBigRats(lhsValue, rhsValue *query_parser.Operand, oper *query_parser.BinaryOperator) bool {
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
	case query_parser.GreaterThanOrEqual:
		return lhsValue.BigRat.Cmp(rhsValue.BigRat) >= 0
	default:
		// TODO(jkline): Log this logic error and all other similar cases.
		return false
	}
}

func compareComplex(lhsValue, rhsValue *query_parser.Operand, oper *query_parser.BinaryOperator) bool {
	switch oper.Type {
	case query_parser.Equal:
		return lhsValue.Complex == rhsValue.Complex
	case query_parser.NotEqual:
		return lhsValue.Complex != rhsValue.Complex
	default:
		// Complex values are not ordered.  All other operands return false.
		return false
	}
}

func compareFloats(lhsValue, rhsValue *query_parser.Operand, oper *query_parser.BinaryOperator) bool {
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
	case query_parser.GreaterThanOrEqual:
		return lhsValue.Float >= rhsValue.Float
	default:
		// TODO(jkline): Log this logic error and all other similar cases.
		return false
	}
}

func compareInts(lhsValue, rhsValue *query_parser.Operand, oper *query_parser.BinaryOperator) bool {
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
	case query_parser.GreaterThanOrEqual:
		return lhsValue.Int >= rhsValue.Int
	default:
		// TODO(jkline): Log this logic error and all other similar cases.
		return false
	}
}

func compareUints(lhsValue, rhsValue *query_parser.Operand, oper *query_parser.BinaryOperator) bool {
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
	case query_parser.GreaterThanOrEqual:
		return lhsValue.Uint >= rhsValue.Uint
	default:
		// TODO(jkline): Log this logic error and all other similar cases.
		return false
	}
}

func compareStrings(lhsValue, rhsValue *query_parser.Operand, oper *query_parser.BinaryOperator) bool {
	switch oper.Type {
	case query_parser.Equal:
		r := lhsValue.Str == rhsValue.Str
		// Handle special case for type equal clauses.
		// Only the lhs can have the AltStr field set.
		if !r && lhsValue.HasAltStr {
			r = lhsValue.AltStr == rhsValue.Str
		}
		return r
	case query_parser.NotEqual:
		return lhsValue.Str != rhsValue.Str
	case query_parser.LessThan:
		return lhsValue.Str < rhsValue.Str
	case query_parser.LessThanOrEqual:
		return lhsValue.Str <= rhsValue.Str
	case query_parser.GreaterThan:
		return lhsValue.Str > rhsValue.Str
	case query_parser.GreaterThanOrEqual:
		return lhsValue.Str >= rhsValue.Str
	case query_parser.Like:
		return rhsValue.CompRegex.MatchString(lhsValue.Str)
	case query_parser.NotLike:
		return !rhsValue.CompRegex.MatchString(lhsValue.Str)
	default:
		// TODO(jkline): Log this logic error and all other similar cases.
		return false
	}
}

func compareTimes(lhsValue, rhsValue *query_parser.Operand, oper *query_parser.BinaryOperator) bool {
	switch oper.Type {
	case query_parser.Equal:
		return lhsValue.Time.Equal(rhsValue.Time)
	case query_parser.NotEqual:
		return !lhsValue.Time.Equal(rhsValue.Time)
	case query_parser.LessThan:
		return lhsValue.Time.Before(rhsValue.Time)
	case query_parser.LessThanOrEqual:
		return lhsValue.Time.Before(rhsValue.Time) || lhsValue.Time.Equal(rhsValue.Time)
	case query_parser.GreaterThan:
		return lhsValue.Time.After(rhsValue.Time)
	case query_parser.GreaterThanOrEqual:
		return lhsValue.Time.After(rhsValue.Time) || lhsValue.Time.Equal(rhsValue.Time)
	default:
		// TODO(jkline): Log this logic error and all other similar cases.
		return false
	}
}

func compareObjects(lhsValue, rhsValue *query_parser.Operand, oper *query_parser.BinaryOperator) bool {
	switch oper.Type {
	case query_parser.Equal:
		return reflect.DeepEqual(lhsValue.Object, rhsValue.Object)
	case query_parser.NotEqual:
		return !reflect.DeepEqual(lhsValue.Object, rhsValue.Object)
	default: // other operands are non-sensical
		return false
	}
}

func resolveArgsAndExecFunction(db query_db.Database, k string, v *vdl.Value, f *query_parser.Function) (*query_parser.Operand, error) {
	// Resolve the function's arguments
	callingArgs := []*query_parser.Operand{}
	for _, arg := range f.Args {
		resolvedArg := resolveOperand(db, k, v, arg)
		if resolvedArg == nil {
			return nil, syncql.NewErrFunctionArgBad(db.GetContext(), arg.Off, f.Name, arg.String())
		}
		callingArgs = append(callingArgs, resolvedArg)
	}
	// Exec the function
	retValue, err := query_functions.ExecFunction(db, f, callingArgs)
	if err != nil {
		return nil, err
	}
	return retValue, nil
}

func resolveOperand(db query_db.Database, k string, v *vdl.Value, o *query_parser.Operand) *query_parser.Operand {
	if o.Type == query_parser.TypFunction {
		// Note: if the function was computed at check time, the operand is replaced
		// in the parse tree with the return value.  As such, thre is no need to check
		// the computed field.
		if retValue, err := resolveArgsAndExecFunction(db, k, v, o.Function); err == nil {
			return retValue
		} else {
			// Per spec, function errors resolve to nil
			return nil
		}
	}
	if o.Type != query_parser.TypField {
		return o
	}
	value, hasAltStr, altStr := ResolveField(k, v, o.Column)
	if value.IsNil() {
		return nil
	}

	// Convert value to an operand
	var newOp query_parser.Operand
	newOp.Off = o.Off

	switch value.Kind() {
	case vdl.Bool:
		newOp.Type = query_parser.TypBool
		newOp.Bool = value.Bool()
	case vdl.Byte:
		newOp.Type = query_parser.TypInt
		newOp.Int = int64(value.Byte())
	case vdl.Enum:
		newOp.Type = query_parser.TypStr
		newOp.Str = value.EnumLabel()
		newOp.HasAltStr = false
	case vdl.Int16, vdl.Int32, vdl.Int64:
		newOp.Type = query_parser.TypInt
		newOp.Int = value.Int()
	case vdl.Uint16, vdl.Uint32, vdl.Uint64:
		newOp.Type = query_parser.TypInt
		newOp.Int = int64(value.Uint())
	case vdl.Float32, vdl.Float64:
		newOp.Type = query_parser.TypFloat
		newOp.Float = value.Float()
	case vdl.String:
		newOp.Type = query_parser.TypStr
		newOp.Str = value.RawString()
		newOp.HasAltStr = hasAltStr
		newOp.AltStr = altStr
	case vdl.Complex64, vdl.Complex128:
		newOp.Type = query_parser.TypComplex
		newOp.Complex = value.Complex()
	default: // OpObject for structs, arrays, maps, ...
		if value.Kind() == vdl.Struct && value.Type().Name() == "time.Time" {
			newOp.Type = query_parser.TypTime
			err := vdl.Convert(&newOp.Time, value)
			if err != nil {
				return nil
			}
		} else {
			newOp.Type = query_parser.TypObject
			newOp.Object = value
		}
	}
	return &newOp
}

// Resolve a field.  In the special case where a type is evaluated, in addition
// to a string being returned, and alternate string is returned.  In this case,
// <string-value>, true, <alt-string> is returned.  In all other cases,
// <value>,false,"" is returned.
func ResolveField(k string, v *vdl.Value, f *query_parser.Field) (*vdl.Value, bool, string) {
	if query_checker.IsKeyField(f) {
		return vdl.StringValue(k), false, ""
	}
	t := v.Type()
	if query_checker.IsTypeField(f) {
		// Types evaluate to two strings, Str and AltStr.
		// This is because types match on full path or just the name.
		pkg, name := vdl.SplitIdent(t.Name())
		return vdl.StringValue(pkg + "." + name), true, name
	}

	object := v
	segments := f.Segments
	// The first segment will always be v (itself), skip it.
	for i := 1; i < len(segments); i++ {
		// object must be a struct in order to look for the next segment.
		if object.Kind() == vdl.Struct {
			if object = object.StructFieldByName(segments[i].Value); object == nil {
				return vdl.ValueOf(nil), false, "" // field does not exist
			}
		} else if object.Kind() == vdl.Union {
			unionType := object.Type()
			idx, tempValue := object.UnionField()
			if segments[i].Value == unionType.Field(idx).Name {
				object = tempValue
			} else {
				return vdl.ValueOf(nil), false, "" // union field does not exist or is not set
			}
		} else {
			return vdl.ValueOf(nil), false, "" // can only traverse into structs and unions
		}
	}
	return object, false, ""
}

// Evaluate the where clause, substituting false for all expressions involving the key and
// true for all other expressions.  If the answer is true, it is possible to satisfy the
// expression for any key.  As such, all keys must be fetched.
func CheckIfAllKeysMustBeFetched(e *query_parser.Expression) bool {
	switch e.Operator.Type {
	case query_parser.And:
		return CheckIfAllKeysMustBeFetched(e.Operand1.Expr) && CheckIfAllKeysMustBeFetched(e.Operand2.Expr)
	case query_parser.Or:
		return CheckIfAllKeysMustBeFetched(e.Operand1.Expr) || CheckIfAllKeysMustBeFetched(e.Operand2.Expr)
	default: // =, > >=, <, <=, Like, <>, NotLike
		if query_checker.IsKey(e.Operand1) {
			return false
		} else {
			return true
		}
	}
}

// EvalWhereUsingOnlyKey return type.  See that function for details.
type EvalWithKeyResult int

const (
	INCLUDE EvalWithKeyResult = iota
	EXCLUDE
	FETCH_VALUE
)

// Evaluate the where clause to determine if the row should be selected, but do so using only
// the key.  Possible returns are:
// INCLUDE: the row should included in the results
// EXCLUDE: the row should NOT be included
// FETCH_VALUE: the value and/or type of the value are required to determine if row should be included.
// The above decision is accomplished by evaluating all expressions which reference the key and
// substituing false for all other expressions.  If the result is true, INCLUDE is returned.
// If the result is false, but no other expressions (i.e., expressions which refer to the type
// of the value or the value itself) were encountered, EXCLUDE is returned; else, FETCH_VALUE is
// returned indicating the value must be fetched in order to determine if the row should be included
// in the results.
func EvalWhereUsingOnlyKey(db query_db.Database, s *query_parser.SelectStatement, k string) EvalWithKeyResult {
	if s.Where == nil { // all rows will be in result
		return INCLUDE
	}
	return evalExprUsingOnlyKey(db, s.Where.Expr, k)
}

func evalExprUsingOnlyKey(db query_db.Database, e *query_parser.Expression, k string) EvalWithKeyResult {
	switch e.Operator.Type {
	case query_parser.And:
		op1Result := evalExprUsingOnlyKey(db, e.Operand1.Expr, k)
		op2Result := evalExprUsingOnlyKey(db, e.Operand2.Expr, k)
		if op1Result == INCLUDE && op2Result == INCLUDE {
			return INCLUDE
		} else if op1Result == EXCLUDE || op2Result == EXCLUDE {
			// One of the operands evaluated to EXCLUDE.
			// As such, the value is not needed to reject the row.
			return EXCLUDE
		} else {
			return FETCH_VALUE
		}
	case query_parser.Or:
		op1Result := evalExprUsingOnlyKey(db, e.Operand1.Expr, k)
		op2Result := evalExprUsingOnlyKey(db, e.Operand2.Expr, k)
		if op1Result == INCLUDE || op2Result == INCLUDE {
			return INCLUDE
		} else if op1Result == EXCLUDE && op2Result == EXCLUDE {
			return EXCLUDE
		} else {
			return FETCH_VALUE
		}
	default: // =, > >=, <, <=, Like, <>, NotLike
		if !query_checker.IsKey(e.Operand1) {
			return FETCH_VALUE
		} else {
			if evalComparisonOperators(db, k, nil, e) {
				return INCLUDE
			} else {
				return EXCLUDE
			}
		}
	}
}
