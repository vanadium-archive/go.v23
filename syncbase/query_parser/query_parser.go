// Package query_parser is a parser to parse a simplified select statement (a la SQL) for the
// Vanadium key value store (a.k.a., syncbase).
//
// The select is of the form:
//
// <query_specification> ::=
//   SELECT <field_clause> FROM <from_clause> [WHERE <where_clause>]
//
// <field_clause> ::= <column_field>[{<comma><column_field>}...]
//
// <column_field> ::= <field>[<period><asterisk>]
//
// <field> ::= <segment>[{<period><segment>}...]
//
// <segment> ::= <identifier>
//
// <from_clause> ::= <table>[{<comma><table>}...]
//
// <table> ::= <identifier> [AS <identifier>]
//
// <where_clause> ::= <expression>
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
// <literal> ::= <string_literal> | <int_literal> | <float_literal>
//
// Example:
// select foo.bar, baz from foobarbaz, bazbarfoo where foo = 42 and bar not like "abc%"
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
	Pos   scanner.Position
}

type SyntaxError struct {
	Msg string
	Pos scanner.Position
}

func (e *SyntaxError) Error() string {
	return fmt.Sprintf("[O:%d,R:%d,C:%d] %s", e.Pos.Offset, e.Pos.Line, e.Pos.Column, e.Msg)
}

func Error(pos scanner.Position, msg string) *SyntaxError {
	return &SyntaxError{msg, pos}
}

type Statement interface {
	String() string
}

type Field struct {
	Segments []string
}

type Table struct {
	Name string
	As   string
}

type BinaryOperator int

const (
	And BinaryOperator = 1 + iota
	Equal
	Like
	NotEqual
	NotLike
	Or
)

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
}

type Expression struct {
	Operand1 *Operand
	Operator *BinaryOperator
	Operand2 *Operand
}

type SelectStatement struct {
	Columns []Field
	Tables  []Table
	Where   *Expression
}

func ScanToken(s *scanner.Scanner, st Statement) *Token {
	var token Token
	tok := s.Scan()
	token.Value = s.TokenText()
	token.Pos = s.Pos()
	token.Pos.Offset = token.Pos.Offset - len(token.Value)
	token.Pos.Column = token.Pos.Column - len(token.Value) // TODO(jkline): Column could be negative. Find another way or drop line/column.

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
	token := ScanToken(&s, nil)
	for token.Tok != TokEOF {
		for token.Tok != TokEOF && token.Tok == TokSEMICOLON {
			// eat semicolons
			token = ScanToken(&s, nil)
		}
		if token.Tok == TokEOF {
			break
		}
		st, token, err = ParseStatement(&s, token)
		if err != nil {
			return statements, err
		}
		statements = append(statements, st)
		token = ScanToken(&s, nil)
	}
	return statements, nil
}

// Parse a single statement.  Return a *Statement and the next token (or SyntaxError)
func ParseStatement(s *scanner.Scanner, token *Token) (*Statement, *Token, *SyntaxError) {
	if token.Tok != TokIDENT {
		return nil, nil, Error(token.Pos, fmt.Sprintf("Expected identifier, found '%s'", token.Value))
	}
	switch strings.ToLower(token.Value) {
	case "select":
		var st Statement
		var err *SyntaxError
		st, token, err = Select(s, token)
		return &st, token, err
	default:
		return nil, nil, Error(token.Pos, fmt.Sprintf("Unknown identifier: %s", token.Value))
	}
}

// Parse the [currently] one and only supported statement: select.
func Select(s *scanner.Scanner, token *Token) (Statement, *Token, *SyntaxError) {
	var st SelectStatement

	// parse columns (a.k.a., fields)
	var err *SyntaxError
	token, err = ParseColumns(s, &st, token)
	if err != nil {
		return nil, nil, err
	}

	token, err = ParseFrom(s, &st, token)
	if err != nil {
		return nil, nil, err
	}

	token, err = ParseWhere(s, &st, token)
	if err != nil {
		return nil, nil, err
	}

	// There can be nothing after the where clause for the current statement
	if token.Tok != TokEOF && token.Tok != TokSEMICOLON {
		return nil, nil, Error(token.Pos, fmt.Sprintf("Expected end of statement, found '%s'", token.Value))
	}

	return st, token, nil
}

// Parse columns (fields). Update the select statement directly.  Return next token (or SyntaxError).
func ParseColumns(s *scanner.Scanner, st *SelectStatement, token *Token) (*Token, *SyntaxError) {
	// must be at least one column or it is an error
	// columns may be in dot notation
	// columns are separated by commas
	token = ScanToken(s, st) // eat the select
	if token.Tok == TokEOF {
		return nil, Error(token.Pos, "Unexpected EOF.")
	}
	var err *SyntaxError
	// scan first column
	if token, err = ParseColumn(s, st, token); err != nil {
		return nil, err
	}

	// More columns?
	for token.Tok == TokCOMMA {
		token = ScanToken(s, st)
		if token, err = ParseColumn(s, st, token); err != nil {
			return nil, err
		}
	}

	return token, nil
}

// Parse a column (field). Update the select statement directly.  Return next token (or SyntaxError).
func ParseColumn(s *scanner.Scanner, st *SelectStatement, token *Token) (*Token, *SyntaxError) {
	if token.Tok != TokIDENT && token.Tok != TokASTERISK {
		return nil, Error(token.Pos, fmt.Sprintf("Expected identifier or '*', found '%s'", token.Value))
	}
	var col Field
	col.Segments = append(col.Segments, token.Value)
	saveToken := token

	token = ScanToken(s, st)
	if saveToken.Tok == TokASTERISK && token.Tok != TokEOF && token.Tok == TokPERIOD {
		// If segment is a '*', don't allow more segments.
		return nil, Error(token.Pos, "No segments may follow an asterisk in a field.")
	}

	for token.Tok != TokEOF && token.Tok == TokPERIOD {
		token = ScanToken(s, st)
		if token.Tok != TokIDENT && token.Tok != TokASTERISK {
			return nil, Error(token.Pos, fmt.Sprintf("Expected identifier or '*', found '%s'", token.Value))
		}
		col.Segments = append(col.Segments, token.Value)
		saveToken = token
		token = ScanToken(s, st)
		if saveToken.Tok == TokASTERISK && token.Tok != TokEOF && token.Tok == TokPERIOD {
			// If segment is a '*', don't allow more segments.
			return nil, Error(token.Pos, "No segments may follow an asterisk in a field.")
		}
	}

	st.Columns = append(st.Columns, col)
	return token, nil
}

// Parse tables and update SelectStatement directly.  Return next Token or SyntaxError.
func ParseFrom(s *scanner.Scanner, st *SelectStatement, token *Token) (*Token, *SyntaxError) {
	if strings.ToLower(token.Value) != "from" {
		return nil, Error(token.Pos, fmt.Sprintf("Expected 'from', found '%s'", token.Value))
	}
	token = ScanToken(s, st) // eat from
	// must be at least one table or it is an error
	// tables may contain an AS clause.
	// tables are separated by commas
	if token.Tok == TokEOF {
		return nil, Error(token.Pos, "Unexpected EOF.")
	}
	token, err := ParseTable(s, st, token)
	if err != nil {
		return nil, err
	}

	// More tables?
	for token.Tok == TokCOMMA {
		token = ScanToken(s, st)
		token, err = ParseTable(s, st, token)
		if err != nil {
			return nil, err
		}
	}

	return token, nil
}

// Parse a single table and update SelectStatement directly.  Return next Token or SyntaxError.
func ParseTable(s *scanner.Scanner, st *SelectStatement, token *Token) (*Token, *SyntaxError) {
	if token.Tok != TokIDENT {
		return nil, Error(token.Pos, fmt.Sprintf("Expected identifier, found '%s'", token.Value))
	}
	var table Table
	table.Name = token.Value

	// check for As clause
	token = ScanToken(s, st)

	if strings.ToLower(token.Value) == "as" {
		token = ScanToken(s, st)
		if token.Tok == TokEOF {
			return nil, Error(token.Pos, "Unexpected EOF.")
		}
		if token.Tok != TokIDENT {
			return nil, Error(token.Pos, fmt.Sprintf("Expected identifier, found '%s'", token.Value))
		}
		table.As = token.Value
		token = ScanToken(s, st)
	}

	st.Tables = append(st.Tables, table)
	return token, nil
}

// Parse the where clause (if any).  Set the where directly in the SelectStatemewnt.  Return the next Token or SyntaxError.
func ParseWhere(s *scanner.Scanner, st *SelectStatement, token *Token) (*Token, *SyntaxError) {
	// parse Optional where clause
	if token.Tok != TokEOF && token.Tok != TokSEMICOLON {
		// Since there is more to the statement, it must be a where clause.
		if strings.ToLower(token.Value) != "where" {
			return nil, Error(token.Pos, fmt.Sprintf("Expected 'where', found '%s'", token.Value))
		}
		token = ScanToken(s, st)
		// parse expression
		var err *SyntaxError
		st.Where, token, err = ParseExpression(s, st, token)
		if err != nil {
			return nil, err
		}
	}
	return token, nil
}

// Parse a parenthesized expression.  Return expression and next token (or SyntaxError)
func ParseParenthesizedExpression(s *scanner.Scanner, st *SelectStatement, token *Token) (*Expression, *Token, *SyntaxError) {
	// Only called when token == TokLEFTPAREN
	token = ScanToken(s, st) // eat '('
	var expr *Expression
	var err *SyntaxError
	expr, token, err = ParseExpression(s, st, token)
	if err != nil {
		return nil, nil, err
	}
	// Expect right paren
	if token.Tok == TokEOF || token.Tok == TokSEMICOLON {
		return nil, nil, Error(token.Pos, "Unexpected end of statement.")
	}
	if token.Tok != TokRIGHTPAREN {
		return nil, nil, Error(token.Pos, "Expected ')'.")
	}
	token = ScanToken(s, st) // eat ')'
	return expr, token, nil
}

// Parse an expression.  Return expression and next token (or SyntaxError)
func ParseExpression(s *scanner.Scanner, st *SelectStatement, token *Token) (*Expression, *Token, *SyntaxError) {
	if token.Tok == TokEOF || token.Tok == TokSEMICOLON {
		return nil, nil, Error(token.Pos, "Unexpected end of statement.")
	}

	var err *SyntaxError
	var expr *Expression

	if token.Tok == TokLEFTPAREN {
		expr, token, err = ParseParenthesizedExpression(s, st, token)
	} else {
		// We expect a like/equal expression
		expr, token, err = ParseLikeEqualExpression(s, st, token)
	}
	if err != nil {
		return nil, nil, err
	}

	for token.Tok != TokEOF && token.Tok != TokSEMICOLON && token.Tok != TokRIGHTPAREN {
		// There is more.  We expect 'and', 'or' or ')'.
		if strings.ToLower(token.Value) != "and" && strings.ToLower(token.Value) != "or" {
			return nil, nil, Error(token.Pos, fmt.Sprintf("Expected 'and' or 'or', found '%s'.", token.Value))
		}
		var newExpression Expression
		var operand1 Operand
		operand1.Type = OpExpr
		operand1.Expr = expr
		newExpression.Operand1 = &operand1

		newExpression.Operator, token, err = ParseLogicalOperator(s, st, token)
		if err != nil {
			return nil, nil, err
		}

		expr = &newExpression
		// Need to set operand2.
		var operand2 Operand
		expr.Operand2 = &operand2
		if token.Tok == TokLEFTPAREN {
			expr.Operand2.Type = OpExpr
			expr.Operand2.Expr, token, err = ParseParenthesizedExpression(s, st, token)
		} else {
			expr.Operand2.Type = OpExpr
			expr.Operand2.Expr, token, err = ParseLikeEqualExpression(s, st, token)
		}
		if err != nil {
			return nil, nil, err
		}
	}

	return expr, token, nil
}

// Parse a binary expression.  Return expression and next token (or SyntaxError)
func ParseLikeEqualExpression(s *scanner.Scanner, st *SelectStatement, token *Token) (*Expression, *Token, *SyntaxError) {
	// operand 1
	var operand1 *Operand
	var err *SyntaxError
	operand1, token, err = ParseOperand(s, st, token)
	if err != nil {
		return nil, nil, err
	}

	// operator
	var operator *BinaryOperator
	operator, token, err = ParseBinaryOperator(s, st, token)
	if err != nil {
		return nil, nil, err
	}

	// operand 2
	var operand2 *Operand
	if token.Tok == TokEOF || token.Tok == TokSEMICOLON {
		return nil, nil, Error(token.Pos, "Unexpected EOF, expected operator.")
	}
	operand2, token, err = ParseOperand(s, st, token)
	if err != nil {
		return nil, nil, err
	}

	var expression Expression
	expression.Operand1 = operand1
	expression.Operator = operator
	expression.Operand2 = operand2

	return &expression, token, nil
}

// Parse an operand (field or literal) and return it and the next Token (or SyntaxError)
func ParseOperand(s *scanner.Scanner, st *SelectStatement, token *Token) (*Operand, *Token, *SyntaxError) {
	if token.Tok == TokEOF || token.Tok == TokSEMICOLON {
		return nil, nil, Error(token.Pos, "Unexpected EOF, expected operand.")
	}
	var operand Operand
	switch token.Tok {
	case TokIDENT:
		operand.Type = OpField
		var field Field
		field.Segments = append(field.Segments, token.Value)

		// Get the next token.  See if it is a '.'.
		token = ScanToken(s, st)
		for token.Tok != TokEOF && token.Tok != TokSEMICOLON && token.Tok == TokPERIOD {
			token = ScanToken(s, st)
			if token.Tok != TokIDENT {
				return nil, nil, Error(token.Pos, fmt.Sprintf("Expected identifier, found '%s'", token.Value))
			}
			field.Segments = append(field.Segments, token.Value)
			token = ScanToken(s, st)
		}
		operand.Column = &field
	case TokINT:
		operand.Type = OpInt
		i, err := strconv.ParseInt(token.Value, 0, 64)
		if err != nil {
			return nil, nil, Error(token.Pos, fmt.Sprintf("Logic error, could not convert %s to int64.", token.Value))
		}
		operand.Int = i
		token = ScanToken(s, st)
	case TokFLOAT:
		operand.Type = OpFloat
		f, err := strconv.ParseFloat(token.Value, 64)
		if err != nil {
			return nil, nil, Error(token.Pos, fmt.Sprintf("Logic error, could not convert %s to float64.", token.Value))
		}
		operand.Float = f
		token = ScanToken(s, st)
	case TokCHAR:
		operand.Type = OpChar
		operand.Char, _ = utf8.DecodeRuneInString(token.Value)
		token = ScanToken(s, st)
	case TokSTRING:
		operand.Type = OpLiteral
		operand.Literal = token.Value
		token = ScanToken(s, st)
	default:
		return nil, nil, Error(token.Pos, fmt.Sprintf("Expected operand, found '%s'.", token.Value))
	}
	return &operand, token, nil
}

// Parse binary operator and return it and the next Token (or SyntaxError)
func ParseBinaryOperator(s *scanner.Scanner, st *SelectStatement, token *Token) (*BinaryOperator, *Token, *SyntaxError) {
	if token.Tok == TokEOF || token.Tok == TokSEMICOLON {
		return nil, nil, Error(token.Pos, "Unexpected EOF, expected operator.")
	}
	var operator BinaryOperator
	if token.Tok == TokIDENT {
		switch strings.ToLower(token.Value) {
		case "equal":
			operator = Equal
		case "like":
			operator = Like
		case "not":
			token = ScanToken(s, st)
			if token.Tok == TokEOF || (strings.ToLower(token.Value) != "equal" && strings.ToLower(token.Value) != "like") {
				return nil, nil, Error(token.Pos, "Expected 'equal' or 'like'")
			}
			switch strings.ToLower(token.Value) {
			case "equal":
				operator = NotEqual
			default: //case "like":
				operator = NotLike
			}
		default:
			return nil, nil, Error(token.Pos, fmt.Sprintf("Expected operator ('like', 'not like', '=', '<>', 'equal' or 'not equal', found '%s'.", token.Value))
		}
	} else {
		switch token.Tok {
		case TokEQUAL:
			operator = Equal
		case TokLEFTANGLEBRACKET:
			token = ScanToken(s, st)
			if token.Tok == TokEOF || token.Tok != TokRIGHTANGLEBRACKET {
				return nil, nil, Error(token.Pos, "Expected '>'")
			}
			operator = NotEqual
		default:
			return nil, nil, Error(token.Pos, fmt.Sprintf("Expected operator ('like', 'not like', '=', '<>', 'equal' or 'not equal', found '%s'.", token.Value))
		}
	}

	token = ScanToken(s, st)
	return &operator, token, nil
}

// Parse logical operator and return it and the next Token (or SyntaxError)
func ParseLogicalOperator(s *scanner.Scanner, st *SelectStatement, token *Token) (*BinaryOperator, *Token, *SyntaxError) {
	if token.Tok == TokEOF || token.Tok == TokSEMICOLON {
		return nil, nil, Error(token.Pos, "Unexpected EOF, expected operator.")
	}
	var operator BinaryOperator
	switch strings.ToLower(token.Value) {
	case "and":
		operator = And
	case "or":
		operator = Or
	default:
		return nil, nil, Error(token.Pos, fmt.Sprintf("Expected operator ('and' or 'or', found '%s'.", token.Value))
	}

	token = ScanToken(s, st)
	return &operator, token, nil
}

// Pretty string of select statement.
func (st SelectStatement) String() string {
	val := "SELECT Columns("
	for i := range st.Columns {
		if i != 0 {
			val += ","
		}
		for j := range st.Columns[i].Segments {
			if j != 0 {
				val += "."
			}
			val += st.Columns[i].Segments[j]
		}
	}
	val += ") Tables("
	for i := range st.Tables {
		if i != 0 {
			val += ","
		}
		val += st.Tables[i].Name
		if st.Tables[i].As != "" {
			val += fmt.Sprintf(" AS %s", st.Tables[i].As)
		}
	}
	val += ") Where"
	val += ExpressionToString(st.Where)
	return val
}

func OperandToString(operand *Operand) string {
	var val string
	if operand == nil {
		return "<nil>"
	}
	switch operand.Type {
	case OpField:
		for i := range operand.Column.Segments {
			if i == 0 {
				val += "(field)"
			} else {
				val += "."
			}
			val += operand.Column.Segments[i]
		}
	case OpInt:
		val += "(int)"
		val += strconv.FormatInt(operand.Int, 10)
	case OpFloat:
		val += "(float)"
		val += strconv.FormatFloat(operand.Float, 'f', -1, 64)
	case OpLiteral:
		val += "(literal)"
		val += operand.Literal
	case OpExpr:
		val += ExpressionToString(operand.Expr)
	default:
		val += "<operand-type-undefined>"

	}
	return val
}

func OperatorToString(op *BinaryOperator) string {
	if op == nil {
		return "<nil>"
	}
	var val string
	switch *op {
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

func ExpressionToString(expr *Expression) string {
	if expr == nil {
		return "(<none>)"
	}
	val := "("
	val += OperandToString(expr.Operand1)
	val += " "
	val += OperatorToString(expr.Operator)
	val += " "
	val += OperandToString(expr.Operand2)
	val += ")"
	return val
}
