package parse

// A note regarding the String functions below: for debugging, it is
// nice to be able to print out the entire AST.  Without the
// type-specific String functions, the printer will stop at the first
// pointer or interface.  If the type is a pointer within another
// type, you can simply do:
//  	return fmt.Sprintf("%+v", *q)
// However, if the type is hidden behind an interface within another
// type, you need to print out the fields individually.  We tried to
// figure out a better way but failed.

import (
	"fmt"
	"math/big"
)

// Pipeline is the root of the AST. Call parse.Parse() to build one.
// This interface is implemented by the Pipeline* types in this package.
type Pipeline interface {
}

// PipelineName implements the Pipeline interface.  It contains only a
// WildcardName.
type PipelineName struct {
	// WildcardName specifies the initial flow of data.
	WildcardName *WildcardName
	// Pos specifies where in the query string this component started.
	Pos Pos
}

func (p *PipelineName) String() string {
	return fmt.Sprintf("%+v", *p)
}

// PipelineType implements the Pipeline interface.  It filters the results
// of a source pipeline by type.
type PipelineType struct {
	// Src produces the results to be filtered by type.
	Src Pipeline
	// Type restricts the results to a specific type of object.
	Type string
	// Pos specifies where in the query string this component started.
	Pos Pos
}

func (p *PipelineType) String() string {
	return fmt.Sprintf("PipelineType{Src:%s Type:%s Pos:%s", p.Src, p.Type, p.Pos)
}

// PipelineFilter implements the Pipeline interface.  It consists of a
// pipeline that produces a set of results that a predicate filters to produce
// a subset of the results.
type PipelineFilter struct {
	// Src produces the results to be filtered by Pred.
	Src Pipeline
	// Pred contains the criteria to filter the results produced by Src.
	Pred Predicate
	// Pos specifies where in the query string this component started.
	Pos Pos
}

func (p *PipelineFilter) String() string {
	return fmt.Sprintf("Filter{Src:%s Pred:%s Pos:%s}", p.Src, p.Pred, p.Pos)
}

// PipelineSelection implements the Pipeline interface.  It consists of a
// source pipeline and a list of sub-pipelines.  For each entry in the result
// set produced by the source pipeline, the list of sub-pipelines will run and
// produce a new result set.
type PipelineSelection struct {
	// Src produces the results to be further processed by SubPipelines
	Src Pipeline
	// SubPipelines is the list of pipelines to run for each result produced
	// by Src.
	SubPipelines []Alias
	// Pos specifies where in the query string this component started.
	Pos Pos
}

func (q *PipelineSelection) String() string {
	str := fmt.Sprintf("Selection{Src:%s SubPipelines:[", q.Src)
	for i, p := range q.SubPipelines {
		if i != 0 {
			str += ", "
		}
		str += p.String()
	}
	str += fmt.Sprintf("] Pos:%s}", q.Pos)
	return str
}

// PipelineFunc implements the Pipeline interface.  It consists of a source pipeline
// and a function that should be applied to the results of the source.
type PipelineFunc struct {
	// Src produces the results that will be processed by the function.
	Src Pipeline
	// FuncName is the name of the function to apply to the results of Src.
	FuncName string
	// Args is the list of arguments passed to the function.
	Args []Expr
	// Pos specifies where in the query string this component started.
	Pos Pos
}

func (q *PipelineFunc) String() string {
	return fmt.Sprintf("Func{Src:%s FuncName:%s Args:%s Pos:%s}", q.Src, q.FuncName, q.Args, q.Pos)
}

// Expansion expands an Object name to include its children.
type Expansion int

const (
	// Self means that just the named object should be queried.
	// Examples: "foo/bar", "foo/bar/.", "."
	Self Expansion = iota
	// Star means that all immediate fields and children of the
	// named object should be queried.
	// Examples: "foo/bar/*", "*"
	Star
)

// String returns the name of the constant.
func (e Expansion) String() string {
	switch e {
	case Self:
		return "Self"
	case Star:
		return "*"
	default:
		return fmt.Sprintf("unknown expansion %d", e)
	}
}

// WildcardName is the first part of a query and specifies where in the
// namespace to begin the search.
type WildcardName struct {
	// VName is an Object name.  Any query results must have this name as
	// a prefix.
	VName string
	// Exp possibly expands the query to the children of VName.
	Exp Expansion
	// Pos specifies where in the query string this component started.
	Pos Pos
}

func (sn *WildcardName) String() string {
	return fmt.Sprintf("%+v", *sn)
}

// Predicate represents boolean logic that filters the result set.
// This interface is implemented by the Predicate* types in this
// package.
type Predicate interface {
}

// PredicateBool is a Predicate that represents the use of "true" or
// "false" in a query string.
type PredicateBool struct {
	// Bool contains the literal value that was used in the query.
	Bool bool
	// Pos specifies where in the query string this component started.
	Pos Pos
}

func (p *PredicateBool) String() string {
	return fmt.Sprintf("%+v", *p)
}

// Comparator is the comparison operator.
type Comparator int

const (
	CompEQ Comparator = iota // ==
	CompNE                   // !=
	CompLT                   // <
	CompGT                   // >
	CompLE                   // <=
	CompGE                   // >=
)

func (c Comparator) String() string {
	switch c {
	case CompEQ:
		return "=="
	case CompNE:
		return "!="
	case CompLT:
		return "<"
	case CompGT:
		return ">"
	case CompLE:
		return "<="
	case CompGE:
		return ">="
	default:
		return fmt.Sprintf("unknown comparator %d", c)
	}
}

// PredicateCompare is a Predicate that compares two Expr objects.
type PredicateCompare struct {
	// LHS is the left hand side of the comparison.
	LHS Expr
	// RHS is the right hand side of the comparison.
	RHS Expr
	// Comp specifies which operator to use in the comparison.
	Comp Comparator
	// Pos specifies where in the query string this component started.
	Pos Pos
}

func (p *PredicateCompare) String() string {
	return fmt.Sprintf("{LHS:%s RHS:%s Comp:%s Pos:%s}", p.LHS, p.RHS, p.Comp, p.Pos)
}

// PredicateAnd is a Predicate that is the logical conjunction of two
// predicates.
type PredicateAnd struct {
	// LHS is the left hand side of the conjuction.
	LHS Predicate
	// RHS is the right hand side of the conjunction.
	RHS Predicate
	// Pos specifies where in the query string this component started.
	Pos Pos
}

func (p *PredicateAnd) String() string {
	return fmt.Sprintf("And{LHS:%s RHS:%s Pos:%s}", p.LHS, p.RHS, p.Pos)
}

// PredicateOr is a Predicate that is the logical disjunction of two
// predicates.
type PredicateOr struct {
	// LHS is the left hand side of the disjunction.
	LHS Predicate
	// RHS is the right hand side of the disjunction.
	RHS Predicate
	// Pos specifies where in the query string this component started.
	Pos Pos
}

func (p *PredicateOr) String() string {
	return fmt.Sprintf("Or{LHS:%s RHS:%s, Pos:%s}", p.LHS, p.RHS, p.Pos)
}

// PredicateNot is a Predicate that is the logical negation of another
// predicate.
type PredicateNot struct {
	// Pred is the predicate to be negated.
	Pred Predicate
	// Pos specifies where in the query string this component started.
	Pos Pos
}

func (p *PredicateNot) String() string {
	return fmt.Sprintf("Not{Pred:%s Pos:%s}", p.Pred, p.Pos)
}

// PredicateFunc is a Predicate represented as a function call that
// returns a bool.
type PredicateFunc struct {
	// FuncName is the name of the function.
	FuncName string
	// Args is the list of arguments passed to the function.
	Args []Expr
	// Pos specifies where in the query string this component started.
	Pos Pos
}

func (p *PredicateFunc) String() string {
	return fmt.Sprintf("{FuncName:%s Args:%s Pos:%s}", p.FuncName, p.Args, p.Pos)
}

// Expr is an expression (e.g. string, int, variable).  It is
// implemented by the Expr* types in this package.
type Expr interface {
}

// ExprString is an Expr for a literal string.
type ExprString struct {
	// Str is the literal string used in the query.
	Str string
	// Pos specifies where in the query string this component started.
	Pos Pos
}

func (e *ExprString) String() string {
	return fmt.Sprintf("{Str:%s Pos:%s}", e.Str, e.Pos)
}

// ExprRat is an Expr for a literal rational number.
type ExprRat struct {
	// Rat is the literal rational number used in the query.
	Rat *big.Rat
	// Pos specifies where in the query string this component started.
	Pos Pos
}

func (e *ExprRat) String() string {
	return fmt.Sprintf("{Rat:%s Pos:%s}", e.Rat, e.Pos)
}

// ExprInt is an Expr for a literal integer.
type ExprInt struct {
	// Int is the literal integer used in the query.
	Int *big.Int
	// Pos specifies where in the query string this component started.
	Pos Pos
}

func (e *ExprInt) String() string {
	return fmt.Sprintf("{Int:%s Pos:%s}", e.Int, e.Pos)
}

// ExprName is an Expr for an Object name literal.
type ExprName struct {
	// Name is the Object name used in the query.
	Name string
	// Pos specifies where in the query string this component started.
	Pos Pos
}

func (e *ExprName) String() string {
	return fmt.Sprintf("{Name:%s Pos:%s}", e.Name, e.Pos)
}

// ExprFunc is an Expr representing a function call with a list of args.
type ExprFunc struct {
	// FuncName is the name of the function.
	FuncName string
	// Args is the list of arguments passed to the function.
	Args []Expr
	// Pos specifies where in the query string this component started.
	Pos Pos
}

func (e *ExprFunc) String() string {
	return fmt.Sprintf("{FuncName:%s Args:%s Pos:%s}", e.FuncName, e.Args, e.Pos)
}

// Operator represents a unary operator.
type Operator int

const (
	OpNeg Operator = iota // -
	OpPos                 // +
)

func (o Operator) String() string {
	switch o {
	case OpNeg:
		return "-"
	case OpPos:
		return "+"
	default:
		return fmt.Sprintf("unknown operator %d", o)
	}
}

// ExprUnary is an Expr with a unary operator.
type ExprUnary struct {
	// Operand is the expression to be modified by Op.
	Operand Expr
	// Op is the operator that modifies Operand.
	Op Operator
	// Pos specifies where in the query string this component started.
	Pos Pos
}

func (e *ExprUnary) String() string {
	return fmt.Sprintf("{Operand:%s Op:%s Pos:%s}", e.Operand, e.Op, e.Pos)
}

// Alias represents a pipeline that has an alternate name inside of a
// selection using the 'as' keyword.
type Alias struct {
	// Pipeline is the pipeline to be aliased.
	Pipeline Pipeline
	// Alias is the new name for the output of the pipeline.
	Alias string
	// Hidden is true if this field in the selection should not be included
	// in the results sent to the client.
	Hidden bool
}

func (a *Alias) String() string {
	return fmt.Sprintf("{Pipeline:%s Alias:%s Hidden:%v}", a.Pipeline, a.Alias, a.Hidden)
}
