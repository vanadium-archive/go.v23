// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package query_parser is a parser to parse a simplified select statement (a la SQL) for the
// Vanadium key value store (a.k.a., syncbase).
//
// The select is of the form:
//
// <query_specification> ::=
//   SELECT <field_clause> <from_clause> [<where_clause>] [<limit_offset_clause>]
//
// <field_clause> ::= <column_field>[{<comma><column_field>}...]
//
// <column_field> ::= <field>[<period><asterisk>]
//
// <field> ::= <segment>[{<period><segment>}...]
//
// <segment> ::= <identifier>
//
// <from_clause> ::= FROM <table>
//
// <table> ::= <identifier>
//
// <where_clause> ::= WHERE <expression>
//
// <limit_offset_clause> ::=
// <limit_clause> [<offset_clause>]
// | <offset_clause> [<limit_clause>]
//
// <limit_clause> ::= LIMIT <int_literal>
//
// <offset_clause> ::= OFFSET <int_literal>
//
// <expression> ::=
//   ( <expression> )
//   | <logical_expression>
//   | <binary_expression>
//
// <logical_expression> ::=
//   <expression> <logical_op> <expression>
//
// <logical_op> ::=
//   AND
//   | OR
//
// <binary_expression> ::=
//   <operand> <binary_op> <operand>
//
// <operand> ::=
//   <field>
//   | <literal>
//
// <binary_op> ::=
//   =
//   | EQUAL
//   | <>
//   | NOT EQUAL
//   | LIKE
//   | NOT LIKE
//
// <literal> ::= <string_literal> | <char_literal> | <int_literal> | <float_literal>
//
// Example:
// select foo.bar, baz from foobarbaz where foo = 42 and bar not like "abc%"
//

package query_parser

import (
	"fmt"
	"io"
	"strconv"
	"strings"
	"text/scanner"
	"unicode/utf8"
)

type TokenType int

const (
	TokASTERISK TokenType = 1 + iota
	TokCHAR
	TokCOMMA
	TokEOF
	TokEQUAL
	TokFLOAT
	TokIDENT
	TokINT
	TokLEFTANGLEBRACKET
	TokLEFTPAREN
	TokPERIOD
	TokRIGHTANGLEBRACKET
	TokRIGHTPAREN
	TokSEMICOLON
	TokSTRING
)

type Token struct {
	Tok   TokenType
	Value string
	Off   int64
}

type SyntaxError struct {
	Msg string
	Off int64
}

func (e *SyntaxError) Error() string {
	return fmt.Sprintf("[Off:%d] %s", e.Off, e.Msg)
}

func Error(offset int64, msg string) *SyntaxError {
	return &SyntaxError{msg, offset}
}

type Node struct {
	Off int64
}

type Statement interface {
	String() string
}

type Segment struct {
	Value string
	Node
}

type Field struct {
	Segments []Segment
	Node
}

type BinaryOperatorType int

const (
	And BinaryOperatorType = 1 + iota
	Equal
	Like
	NotEqual
	NotLike
	Or
)

type BinaryOperator struct {
	Type BinaryOperatorType
	Node
}

type OperandType int

const (
	OpChar OperandType = 1 + iota
	OpField
	OpInt
	OpFloat
	OpLiteral
	OpExpr
)

type Operand struct {
	Type    OperandType
	Char    rune
	Column  *Field
	Int     int64
	Float   float64
	Literal string
	Expr    *Expression
	Node
}

type Expression struct {
	Operand1 *Operand
	Operator *BinaryOperator
	Operand2 *Operand
	Node
}

type SelectClause struct {
	Columns []Field
	Node
}

type FromClause struct {
	Table TableEntry
	Node
}

type TableEntry struct {
	Name string
	Node
}

type WhereClause struct {
	Expr *Expression
	Node
}

type Int64Value struct {
	Value int64
	Node
}

type LimitClause struct {
	Limit *Int64Value
	Node
}

type ResultsOffsetClause struct {
	ResultsOffset *Int64Value
	Node
}

type SelectStatement struct {
	Select        *SelectClause
	From          *FromClause
	Where         *WhereClause
	Limit         *LimitClause
	ResultsOffset *ResultsOffsetClause
	Node
}

func ScanToken(s *scanner.Scanner) *Token {
	var token Token
	tok := s.Scan()
	token.Value = s.TokenText()
	token.Off = int64(s.Position.Offset)

	switch tok {
	case '.':
		token.Tok = TokPERIOD
	case ',':
		token.Tok = TokCOMMA
	case '*':
		token.Tok = TokASTERISK
	case ';':
		token.Tok = TokSEMICOLON
	case '(':
		token.Tok = TokLEFTPAREN
	case ')':
		token.Tok = TokRIGHTPAREN
	case '=':
		token.Tok = TokEQUAL
	case '<':
		token.Tok = TokLEFTANGLEBRACKET
	case '>':
		token.Tok = TokRIGHTANGLEBRACKET
	case scanner.EOF:
		token.Tok = TokEOF
	case scanner.Ident:
		token.Tok = TokIDENT
	case scanner.Int:
		token.Tok = TokINT
	case scanner.Float:
		token.Tok = TokFLOAT
	case scanner.Char:
		token.Tok = TokCHAR
		token.Value = token.Value[1 : len(token.Value)-1]
	case scanner.String:
		token.Tok = TokSTRING
		token.Value = token.Value[1 : len(token.Value)-1]
	}
	return &token
}

// Parse statements (which are separated by semicolons).  Return statements or a SyntaxError.
func Parse(src io.Reader) ([]*Statement, *SyntaxError) {
	var s scanner.Scanner
	var statements []*Statement
	s.Init(src)

	var st *Statement
	var err *SyntaxError
	token := ScanToken(&s)
	for token.Tok != TokEOF {
		for token.Tok != TokEOF && token.Tok == TokSEMICOLON {
			// eat semicolons
			token = ScanToken(&s)
		}
		if token.Tok == TokEOF {
			break
		}
		st, token, err = ParseStatement(&s, token)
		if err != nil {
			return statements, err
		}
		statements = append(statements, st)
		token = ScanToken(&s)
	}
	return statements, nil
}

// Parse a single statement.  Return a *Statement and the next token (or SyntaxError)
func ParseStatement(s *scanner.Scanner, token *Token) (*Statement, *Token, *SyntaxError) {
	if token.Tok != TokIDENT {
		return nil, nil, Error(token.Off, fmt.Sprintf("Expected identifier, found '%s'", token.Value))
	}
	switch strings.ToLower(token.Value) {
	case "select":
		var st Statement
		var err *SyntaxError
		st, token, err = Select(s, token)
		return &st, token, err
	default:
		return nil, nil, Error(token.Off, fmt.Sprintf("Unknown identifier: %s", token.Value))
	}
}

// Parse the [currently] one and only supported statement: select.
func Select(s *scanner.Scanner, token *Token) (Statement, *Token, *SyntaxError) {
	var st SelectStatement
	st.Off = token.Off

	// parse SelectClause
	var err *SyntaxError
	st.Select, token, err = ParseSelectClause(s, token)
	if err != nil {
		return nil, nil, err
	}

	st.From, token, err = ParseFromClause(s, token)
	if err != nil {
		return nil, nil, err
	}

	st.Where, token, err = ParseWhereClause(s, token)
	if err != nil {
		return nil, nil, err
	}

	st.Limit, st.ResultsOffset, token, err = ParseLimitResultsOffsetClauses(s, token)
	if err != nil {
		return nil, nil, err
	}

	// There can be nothing remaining for the current statement
	if token.Tok != TokEOF && token.Tok != TokSEMICOLON {
		return nil, nil, Error(token.Off, fmt.Sprintf("Unexpected: '%s'.", token.Value))
	}

	return st, token, nil
}

// Parse the select clause (fields). Return *SelectClause, next token (or SyntaxError).
func ParseSelectClause(s *scanner.Scanner, token *Token) (*SelectClause, *Token, *SyntaxError) {
	// must be at least one column or it is an error
	// columns may be in dot notation
	// columns are separated by commas
	var selectClause SelectClause
	selectClause.Off = token.Off
	token = ScanToken(s) // eat the select
	if token.Tok == TokEOF {
		return nil, nil, Error(token.Off, "Unexpected end of statement.")
	}
	var err *SyntaxError
	// scan first column
	if token, err = ParseColumn(s, &selectClause, token); err != nil {
		return nil, nil, err
	}

	// More columns?
	for token.Tok == TokCOMMA {
		token = ScanToken(s)
		if token, err = ParseColumn(s, &selectClause, token); err != nil {
			return nil, nil, err
		}
	}

	return &selectClause, token, nil
}

// Parse a column (field). Return SelectClause and next token (or SyntaxError).
func ParseColumn(s *scanner.Scanner, selectClause *SelectClause, token *Token) (*Token, *SyntaxError) {
	if token.Tok != TokIDENT && token.Tok != TokASTERISK {
		return nil, Error(token.Off, fmt.Sprintf("Expected identifier or '*', found '%s'", token.Value))
	}
	var col Field
	col.Off = token.Off
	var segment Segment
	segment.Value = token.Value
	segment.Off = token.Off
	col.Segments = append(col.Segments, segment)

	saveToken := token
	token = ScanToken(s)
	// TODO(jkline): Move following check to semantic analysis
	if saveToken.Tok == TokASTERISK && token.Tok != TokEOF && token.Tok == TokPERIOD {
		// If segment is a '*', don't allow more segments.
		return nil, Error(token.Off, "No segments may follow an asterisk in a field.")
	}

	for token.Tok != TokEOF && token.Tok == TokPERIOD {
		token = ScanToken(s)
		if token.Tok != TokIDENT && token.Tok != TokASTERISK {
			return nil, Error(token.Off, fmt.Sprintf("Expected identifier or '*', found '%s'", token.Value))
		}
		var segment Segment
		segment.Value = token.Value
		segment.Off = token.Off
		col.Segments = append(col.Segments, segment)
		saveToken = token
		token = ScanToken(s)
		// TODO(jkline): Move following check to semantic analysis
		if saveToken.Tok == TokASTERISK && token.Tok != TokEOF && token.Tok == TokPERIOD {
			// If segment is a '*', don't allow more segments.
			return nil, Error(token.Off, "No segments may follow an asterisk in a field.")
		}
	}

	selectClause.Columns = append(selectClause.Columns, col)
	return token, nil
}

// Parse the from clause, Return FromClause and next Token or SyntaxError.
func ParseFromClause(s *scanner.Scanner, token *Token) (*FromClause, *Token, *SyntaxError) {
	if strings.ToLower(token.Value) != "from" {
		return nil, nil, Error(token.Off, fmt.Sprintf("Expected 'from', found '%s'", token.Value))
	}
	var fromClause FromClause
	fromClause.Off = token.Off
	token = ScanToken(s) // eat from
	// must be a table specified
	if token.Tok == TokEOF {
		return nil, nil, Error(token.Off, "Unexpected end of statement.")
	}
	if token.Tok != TokIDENT {
		return nil, nil, Error(token.Off, fmt.Sprintf("Expected identifier, found '%s'", token.Value))
	}
	fromClause.Table.Off = token.Off
	fromClause.Table.Name = token.Value
	token = ScanToken(s)
	return &fromClause, token, nil
}

// Parse the where clause (if any).  Return WhereClause (could be nil) and and next Token or SyntaxError.
func ParseWhereClause(s *scanner.Scanner, token *Token) (*WhereClause, *Token, *SyntaxError) {
	// parse Optional where clause
	if token.Tok != TokEOF && token.Tok != TokSEMICOLON {
		if strings.ToLower(token.Value) != "where" {
			return nil, token, nil
		}
		var where WhereClause
		where.Off = token.Off
		token = ScanToken(s)
		// parse expression
		var err *SyntaxError
		where.Expr, token, err = ParseExpression(s, token)
		if err != nil {
			return nil, nil, err
		}
		return &where, token, nil
	} else {
		return nil, token, nil
	}
}

// Parse a parenthesized expression.  Return expression and next token (or SyntaxError)
func ParseParenthesizedExpression(s *scanner.Scanner, token *Token) (*Expression, *Token, *SyntaxError) {
	// Only called when token == TokLEFTPAREN
	token = ScanToken(s) // eat '('
	var expr *Expression
	var err *SyntaxError
	expr, token, err = ParseExpression(s, token)
	if err != nil {
		return nil, nil, err
	}
	// Expect right paren
	if token.Tok == TokEOF || token.Tok == TokSEMICOLON {
		return nil, nil, Error(token.Off, "Unexpected end of statement.")
	}
	if token.Tok != TokRIGHTPAREN {
		return nil, nil, Error(token.Off, "Expected ')'.")
	}
	token = ScanToken(s) // eat ')'
	return expr, token, nil
}

// Parse an expression.  Return expression and next token (or SyntaxError)
func ParseExpression(s *scanner.Scanner, token *Token) (*Expression, *Token, *SyntaxError) {
	if token.Tok == TokEOF || token.Tok == TokSEMICOLON {
		return nil, nil, Error(token.Off, "Unexpected end of statement.")
	}

	var err *SyntaxError
	var expr *Expression

	if token.Tok == TokLEFTPAREN {
		expr, token, err = ParseParenthesizedExpression(s, token)
	} else {
		// We expect a like/equal expression
		expr, token, err = ParseLikeEqualExpression(s, token)
	}
	if err != nil {
		return nil, nil, err
	}

	for token.Tok != TokEOF && token.Tok != TokSEMICOLON && token.Tok != TokRIGHTPAREN {
		// There is more.  If not 'and', 'or' or ')', the where is over.
		if strings.ToLower(token.Value) != "and" && strings.ToLower(token.Value) != "or" {
			return expr, token, nil
		}
		var newExpression Expression
		var operand1 Operand
		operand1.Type = OpExpr
		operand1.Expr = expr
		operand1.Off = operand1.Expr.Off
		newExpression.Operand1 = &operand1
		newExpression.Off = operand1.Off

		newExpression.Operator, token, err = ParseLogicalOperator(s, token)
		if err != nil {
			return nil, nil, err
		}

		expr = &newExpression
		// Need to set operand2.
		var operand2 Operand
		expr.Operand2 = &operand2
		if token.Tok == TokLEFTPAREN {
			expr.Operand2.Type = OpExpr
			expr.Operand2.Expr, token, err = ParseParenthesizedExpression(s, token)
		} else {
			expr.Operand2.Type = OpExpr
			expr.Operand2.Expr, token, err = ParseLikeEqualExpression(s, token)
		}
		if err != nil {
			return nil, nil, err
		}
		expr.Operand2.Off = expr.Operand2.Expr.Off
	}

	return expr, token, nil
}

// Parse a binary expression.  Return expression and next token (or SyntaxError)
func ParseLikeEqualExpression(s *scanner.Scanner, token *Token) (*Expression, *Token, *SyntaxError) {
	var expression Expression
	expression.Off = token.Off

	// operand 1
	var operand1 *Operand
	var err *SyntaxError
	operand1, token, err = ParseOperand(s, token)
	if err != nil {
		return nil, nil, err
	}

	// operator
	var operator *BinaryOperator
	operator, token, err = ParseBinaryOperator(s, token)
	if err != nil {
		return nil, nil, err
	}

	// operand 2
	var operand2 *Operand
	if token.Tok == TokEOF || token.Tok == TokSEMICOLON {
		return nil, nil, Error(token.Off, "Unexpected end of statement, expected operator.")
	}
	operand2, token, err = ParseOperand(s, token)
	if err != nil {
		return nil, nil, err
	}

	expression.Operand1 = operand1
	expression.Operator = operator
	expression.Operand2 = operand2

	return &expression, token, nil
}

// Parse an operand (field or literal) and return it and the next Token (or SyntaxError)
func ParseOperand(s *scanner.Scanner, token *Token) (*Operand, *Token, *SyntaxError) {
	if token.Tok == TokEOF || token.Tok == TokSEMICOLON {
		return nil, nil, Error(token.Off, "Unexpected end of statement, expected operand.")
	}
	var operand Operand
	operand.Off = token.Off
	switch token.Tok {
	case TokIDENT:
		operand.Type = OpField
		var field Field
		field.Off = token.Off
		var segment Segment
		segment.Off = token.Off
		segment.Value = token.Value
		field.Segments = append(field.Segments, segment)

		// Get the next token.  See if it is a '.'.
		token = ScanToken(s)
		for token.Tok != TokEOF && token.Tok != TokSEMICOLON && token.Tok == TokPERIOD {
			token = ScanToken(s)
			if token.Tok != TokIDENT {
				return nil, nil, Error(token.Off, fmt.Sprintf("Expected identifier, found '%s'", token.Value))
			}
			var segment Segment
			segment.Off = token.Off
			segment.Value = token.Value
			field.Segments = append(field.Segments, segment)
			token = ScanToken(s)
		}
		operand.Column = &field
	case TokINT:
		operand.Type = OpInt
		i, err := strconv.ParseInt(token.Value, 0, 64)
		if err != nil {
			return nil, nil, Error(token.Off, fmt.Sprintf("Logic error, could not convert %s to int64.", token.Value))
		}
		operand.Int = i
		token = ScanToken(s)
	case TokFLOAT:
		operand.Type = OpFloat
		f, err := strconv.ParseFloat(token.Value, 64)
		if err != nil {
			return nil, nil, Error(token.Off, fmt.Sprintf("Logic error, could not convert %s to float64.", token.Value))
		}
		operand.Float = f
		token = ScanToken(s)
	case TokCHAR:
		operand.Type = OpChar
		operand.Char, _ = utf8.DecodeRuneInString(token.Value)
		token = ScanToken(s)
	case TokSTRING:
		operand.Type = OpLiteral
		operand.Literal = token.Value
		token = ScanToken(s)
	default:
		return nil, nil, Error(token.Off, fmt.Sprintf("Expected operand, found '%s'.", token.Value))
	}
	return &operand, token, nil
}

// Parse binary operator and return it and the next Token (or SyntaxError)
func ParseBinaryOperator(s *scanner.Scanner, token *Token) (*BinaryOperator, *Token, *SyntaxError) {
	if token.Tok == TokEOF || token.Tok == TokSEMICOLON {
		return nil, nil, Error(token.Off, "Unexpected end of statement, expected operator.")
	}
	var operator BinaryOperator
	operator.Off = token.Off
	if token.Tok == TokIDENT {
		switch strings.ToLower(token.Value) {
		case "equal":
			operator.Type = Equal
		case "like":
			operator.Type = Like
		case "not":
			token = ScanToken(s)
			if token.Tok == TokEOF || (strings.ToLower(token.Value) != "equal" && strings.ToLower(token.Value) != "like") {
				return nil, nil, Error(token.Off, "Expected 'equal' or 'like'")
			}
			switch strings.ToLower(token.Value) {
			case "equal":
				operator.Type = NotEqual
			default: //case "like":
				operator.Type = NotLike
			}
		default:
			return nil, nil, Error(token.Off, fmt.Sprintf("Expected operator ('like', 'not like', '=', '<>', 'equal' or 'not equal', found '%s'.", token.Value))
		}
	} else {
		switch token.Tok {
		case TokEQUAL:
			operator.Type = Equal
		case TokLEFTANGLEBRACKET:
			token = ScanToken(s)
			if token.Tok == TokEOF || token.Tok != TokRIGHTANGLEBRACKET {
				return nil, nil, Error(token.Off, "Expected '>'")
			}
			operator.Type = NotEqual
		default:
			return nil, nil, Error(token.Off, fmt.Sprintf("Expected operator ('like', 'not like', '=', '<>', 'equal' or 'not equal', found '%s'.", token.Value))
		}
	}

	token = ScanToken(s)
	return &operator, token, nil
}

// Parse logical operator and return it and the next Token (or SyntaxError)
func ParseLogicalOperator(s *scanner.Scanner, token *Token) (*BinaryOperator, *Token, *SyntaxError) {
	if token.Tok == TokEOF || token.Tok == TokSEMICOLON {
		return nil, nil, Error(token.Off, "Unexpected end of statement, expected operator.")
	}
	var operator BinaryOperator
	operator.Off = token.Off
	switch strings.ToLower(token.Value) {
	case "and":
		operator.Type = And
	case "or":
		operator.Type = Or
	default:
		return nil, nil, Error(token.Off, fmt.Sprintf("Expected operator ('and' or 'or', found '%s'.", token.Value))
	}

	token = ScanToken(s)
	return &operator, token, nil
}

// Parse and return LimitClause and ResultsOffsetClause (one or both can be nil) and next token (or SyntaxError)
func ParseLimitResultsOffsetClauses(s *scanner.Scanner, token *Token) (*LimitClause, *ResultsOffsetClause, *Token, *SyntaxError) {
	var err *SyntaxError
	var lc *LimitClause
	var oc *ResultsOffsetClause
	for token.Tok != TokEOF && token.Tok != TokSEMICOLON {
		// Note: Can be in any order.  If more than one limit or offset clause, the last one wins
		if token.Tok == TokIDENT && strings.ToLower(token.Value) == "limit" {
			lc, token, err = ParseLimitClause(s, token)
		} else if token.Tok == TokIDENT && strings.ToLower(token.Value) == "offset" {
			oc, token, err = ParseResultsOffsetClause(s, token)
		} else {
			return lc, oc, token, nil
		}
		if err != nil {
			return nil, nil, nil, err
		}
	}
	return lc, oc, token, nil
}

// Parse the limit clause.  Return the LimitClause and the next Token (or SyntaxError).
func ParseLimitClause(s *scanner.Scanner, token *Token) (*LimitClause, *Token, *SyntaxError) {
	var lc LimitClause
	lc.Off = token.Off
	token = ScanToken(s)
	var err *SyntaxError
	lc.Limit, token, err = ParseInt64(s, token)
	if err != nil {
		return nil, nil, err
	}
	return &lc, token, nil
}

// Parse the results offset clause.  Return the ResultsOffsetClause and the next Token (or SyntaxError).
func ParseResultsOffsetClause(s *scanner.Scanner, token *Token) (*ResultsOffsetClause, *Token, *SyntaxError) {
	var oc ResultsOffsetClause
	oc.Off = token.Off
	token = ScanToken(s)
	var err *SyntaxError
	oc.ResultsOffset, token, err = ParseInt64(s, token)
	if err != nil {
		return nil, nil, err
	}
	return &oc, token, nil
}

// Parse and return an Int64Value and next token (or SyntaxError).
func ParseInt64(s *scanner.Scanner, token *Token) (*Int64Value, *Token, *SyntaxError) {
	// We expect an integer literal
	if token.Tok == TokEOF || token.Tok == TokSEMICOLON {
		return nil, nil, Error(token.Off, "Unexpected end of statement, expected integer literal.")
	}
	if token.Tok != TokINT {
		return nil, nil, Error(token.Off, fmt.Sprintf("Expected integer literal., found '%s'.", token.Value))
	}
	var v Int64Value
	v.Off = token.Off
	var err error
	v.Value, err = strconv.ParseInt(token.Value, 0, 64)
	if err != nil {
		return nil, nil, Error(token.Off, fmt.Sprintf("Logic error, could not convert %s to int64.", token.Value))
	}
	token = ScanToken(s)
	return &v, token, nil
}

// Pretty string of select statement.
func (st SelectStatement) String() string {
	val := fmt.Sprintf("Off(%d):", st.Off)
	if st.Select != nil {
		val += st.Select.String()
	}
	if st.From != nil {
		val += " " + st.From.String()
	}
	if st.Where != nil {
		val += " " + st.Where.String()
	}
	if st.Limit != nil {
		val += " " + st.Limit.String()
	}
	if st.ResultsOffset != nil {
		val += " " + st.ResultsOffset.String()
	}
	return val
}

func (sel SelectClause) String() string {
	val := fmt.Sprintf(" Off(%d):SELECT Columns(", sel.Off)
	for i := range sel.Columns {
		if i != 0 {
			val += ","
		}
		val += sel.Columns[i].String()
	}
	val += ")"
	return val
}

func (f Field) String() string {
	val := fmt.Sprintf(" Off(%d):", f.Off)
	for i := range f.Segments {
		if i != 0 {
			val += "."
		}
		val += f.Segments[i].String()
	}
	return val
}

func (s Segment) String() string {
	return fmt.Sprintf(" Off(%d):%s", s.Off, s.Value)
}

func (f FromClause) String() string {
	return fmt.Sprintf("Off(%d):FROM %s", f.Off, f.Table.String())
}

func (t TableEntry) String() string {
	return fmt.Sprintf("Off(%d):%s", t.Off, t.Name)
}

func (w WhereClause) String() string {
	return fmt.Sprintf(" Off(%d):WHERE %s", w.Off, w.Expr.String())
}

func (l LimitClause) String() string {
	return fmt.Sprintf(" Off(%d):LIMIT %d", l.Off, l.Limit)
}

func (l ResultsOffsetClause) String() string {
	return fmt.Sprintf(" Off(%d):OFFSET %d", l.Off, l.ResultsOffset)
}

func (o Operand) String() string {
	val := fmt.Sprintf("Off(%d):", o.Off)
	switch o.Type {
	case OpField:
		val += o.Column.String()
	case OpChar:
		val += "(char)"
		val += strconv.QuoteRune(o.Char)
	case OpInt:
		val += "(int)"
		val += strconv.FormatInt(o.Int, 10)
	case OpFloat:
		val += "(float)"
		val += strconv.FormatFloat(o.Float, 'f', -1, 64)
	case OpLiteral:
		val += "(literal)"
		val += o.Literal
	case OpExpr:
		val += o.Expr.String()
	default:
		val += "<operand-type-undefined>"

	}
	return val
}

func (o BinaryOperator) String() string {
	val := fmt.Sprintf("Off(%d):", o.Off)
	switch o.Type {
	case And:
		val += "AND"
	case Equal:
		val += "="
	case Like:
		val += "LIKE"
	case NotEqual:
		val += "<>"
	case NotLike:
		val += "NOT LIKE"
	case Or:
		val += "OR"
	default:
		val += "<operator-undefined>"
	}
	return val
}

func (e Expression) String() string {
	return fmt.Sprintf("(Off(%d):%s %s %s)", e.Off, e.Operand1.String(), e.Operator.String(), e.Operand2.String())
}
