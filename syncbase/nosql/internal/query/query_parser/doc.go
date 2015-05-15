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
// <field_clause> ::= <column>[{<comma><field>}...]
//
// <column> ::= <field> [AS <string_literal>]
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
//   | <>
//   | <
//   | >
//   | <=
//   | >=
//   | EQUAL
//   | NOT EQUAL
//   | LIKE
//   | NOT LIKE
//
// <literal> ::= <string_literal> | <char_literal> | <int_literal> | <float_literal>
//
// Example:
// select foo.bar, baz from foobarbaz where foo = 42 and bar not like "abc%"
package query_parser
