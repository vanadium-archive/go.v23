package parse

import (
	"fmt"
	"math/big"
	"reflect"
	"regexp"
	"testing"

	"veyron2/query"
)

type parseTest struct {
	query    string
	ast      Pipeline
	errRegex string
}

func TestParsing(t *testing.T) {
	basic := []parseTest{
		///////////////////////////////////////////////////////////////////////
		// PipelineName
		{
			"*",
			&PipelineName{
				&WildcardName{"", Star, Pos{1, 1}},
				Pos{1, 1},
			},
			"",
		},
		{
			".",
			&PipelineName{
				&WildcardName{"", Self, Pos{1, 1}},
				Pos{1, 1},
			},
			"",
		},
		{
			"foo", // Single component paths do not need to be escaped.
			&PipelineName{
				&WildcardName{"foo", Self, Pos{1, 1}},
				Pos{1, 1},
			},
			"",
		},
		{
			"`foo/bar`", // String literal using backtick.
			&PipelineName{
				&WildcardName{"foo/bar", Self, Pos{1, 1}},
				Pos{1, 1},
			},
			"",
		},
		{
			"\"foo/bar\"", // String literal using double quote.
			&PipelineName{
				&WildcardName{"foo/bar", Self, Pos{1, 1}},
				Pos{1, 1},
			},
			"",
		},
		{
			"'foo/bar'", // String literal using single quote.
			&PipelineName{
				&WildcardName{"foo/bar", Self, Pos{1, 1}},
				Pos{1, 1},
			},
			"",
		},
		{
			"`foo/bar/.`",
			&PipelineName{
				&WildcardName{"foo/bar", Self, Pos{1, 1}},
				Pos{1, 1},
			},
			"",
		},
		{
			"foo/.", // Single component paths do not need to be escaped.
			&PipelineName{
				&WildcardName{"foo", Self, Pos{1, 1}},
				Pos{1, 1},
			},
			"",
		},
		{
			"foo/*", // Single component paths do not need to be escaped.
			&PipelineName{
				&WildcardName{"foo", Star, Pos{1, 1}},
				Pos{1, 1},
			},
			"",
		},
		{
			"`foo/bar/*`",
			&PipelineName{
				&WildcardName{"foo/bar", Star, Pos{1, 1}},
				Pos{1, 1},
			},
			"",
		},
		// Single dots are stripped from the name.
		{
			"`./foo`",
			&PipelineName{
				&WildcardName{"foo", Self, Pos{1, 1}},
				Pos{1, 1},
			},
			"",
		},
		{
			"`foo/./bar`",
			&PipelineName{
				&WildcardName{"foo/bar", Self, Pos{1, 1}},
				Pos{1, 1},
			},
			"",
		},
		// Double dots remove the previous component of the name.
		{
			"`foo/../bar`",
			&PipelineName{
				&WildcardName{"bar", Self, Pos{1, 1}},
				Pos{1, 1},
			},
			"",
		},
		{
			"`foo/..`",
			&PipelineName{
				&WildcardName{"", Self, Pos{1, 1}},
				Pos{1, 1},
			},
			"",
		},
		{
			"`foo/../*`",
			&PipelineName{
				&WildcardName{"", Star, Pos{1, 1}},
				Pos{1, 1},
			},
			"",
		},
		{
			"`foo/bar/..`",
			&PipelineName{
				&WildcardName{"foo", Self, Pos{1, 1}},
				Pos{1, 1},
			},
			"",
		},
		// One case where double dots are supported outside of a string literal.
		{
			"foo/..",
			&PipelineName{
				&WildcardName{"", Self, Pos{1, 1}},
				Pos{1, 1},
			},
			"",
		},

		{"``", nil, "Must provide a name, an expansion"},

		{"foo/bar", nil, "Multi-component names must be passed as string literals"},

		{"./foo", nil, "Multi-component names must be passed as string literals"},
		{"foo/./bar", nil, "Multi-component names must be passed as string literals"},

		{"..", nil, "Not possible to use '..' to traverse above the root"},
		{"foo/../../bar", nil, "Multi-component names must be passed as string literals"},
		{"`foo/../../bar`", nil, "Not possible to use '..' to traverse above the root"},
		{"`foo/bar/../../..`", nil, "Not possible to use '..' to traverse above the root"},

		{"*/foo", nil, "'*' is supported only as the last component"},
		{"foo/*/bar", nil, "'*' is supported only as the last component"},
		{"`*/foo`", nil, "'*' is supported only as the last component"},
		{"`foo/*/bar`", nil, "'*' is supported only as the last component"},

		{"/foo", nil, "Names must be relative"},
		{"/.", nil, "Names must be relative"},
		{"`/foo`", nil, "Names must be relative"},
		{"`/.`", nil, "Names must be relative"},

		{"foo/", nil, "Found spurious trailing '/'"},

		//////////////////////////////////////////////////////////////////////////
		// Type filter
		{
			"* | type Team",
			&PipelineType{
				&PipelineName{
					&WildcardName{"", Star, Pos{1, 1}},
					Pos{1, 1},
				},
				"Team",
				Pos{1, 3},
			},
			"",
		},
		{
			"`foo/bar/*` | type Team",
			&PipelineType{
				&PipelineName{
					&WildcardName{"foo/bar", Star, Pos{1, 1}},
					Pos{1, 1},
				},
				"Team",
				Pos{1, 13},
			},
			"",
		},
		{
			"foo | type Team",
			&PipelineType{
				&PipelineName{
					&WildcardName{"foo", Self, Pos{1, 1}},
					Pos{1, 1},
				},
				"Team",
				Pos{1, 5},
			},
			"",
		},
		{"type Team", nil, "'type' cannot start a pipeline"},

		///////////////////////////////////////////////////////////////////////
		// Filters (predicates)
		{
			"* | ?true",
			&PipelineFilter{
				&PipelineName{&WildcardName{"", Star, Pos{1, 1}}, Pos{1, 1}},
				&PredicateBool{true, Pos{1, 6}},
				Pos{1, 3},
			},
			"",
		},
		{
			"* | type Team | ?false",
			&PipelineFilter{
				&PipelineType{&PipelineName{&WildcardName{"", Star, Pos{1, 1}}, Pos{1, 1}}, "Team", Pos{1, 3}},
				&PredicateBool{false, Pos{1, 18}},
				Pos{1, 15},
			},
			"",
		},
		{
			"* | type Team | ?5=7",
			&PipelineFilter{
				&PipelineType{&PipelineName{&WildcardName{"", Star, Pos{1, 1}}, Pos{1, 1}}, "Team", Pos{1, 3}},
				&PredicateCompare{
					&ExprInt{big.NewInt(5), Pos{1, 18}},
					&ExprInt{big.NewInt(7), Pos{1, 20}},
					CompEQ,
					Pos{1, 19},
				},
				Pos{1, 15},
			},
			"",
		},
		{
			"* | type Team | ?5==7",
			&PipelineFilter{
				&PipelineType{&PipelineName{&WildcardName{"", Star, Pos{1, 1}}, Pos{1, 1}}, "Team", Pos{1, 3}},
				&PredicateCompare{
					&ExprInt{big.NewInt(5), Pos{1, 18}},
					&ExprInt{big.NewInt(7), Pos{1, 21}},
					CompEQ,
					Pos{1, 19},
				},
				Pos{1, 15},
			},
			"",
		},
		{
			"* | type Team | ?5!=7",
			&PipelineFilter{
				&PipelineType{&PipelineName{&WildcardName{"", Star, Pos{1, 1}}, Pos{1, 1}}, "Team", Pos{1, 3}},
				&PredicateCompare{
					&ExprInt{big.NewInt(5), Pos{1, 18}},
					&ExprInt{big.NewInt(7), Pos{1, 21}},
					CompNE,
					Pos{1, 19},
				},
				Pos{1, 15},
			},
			"",
		},
		{
			"* | type Team | ?5<7",
			&PipelineFilter{
				&PipelineType{&PipelineName{&WildcardName{"", Star, Pos{1, 1}}, Pos{1, 1}}, "Team", Pos{1, 3}},
				&PredicateCompare{
					&ExprInt{big.NewInt(5), Pos{1, 18}},
					&ExprInt{big.NewInt(7), Pos{1, 20}},
					CompLT,
					Pos{1, 19},
				},
				Pos{1, 15},
			},
			"",
		},
		{
			"* | type Team | ?5>7",
			&PipelineFilter{
				&PipelineType{&PipelineName{&WildcardName{"", Star, Pos{1, 1}}, Pos{1, 1}}, "Team", Pos{1, 3}},
				&PredicateCompare{
					&ExprInt{big.NewInt(5), Pos{1, 18}},
					&ExprInt{big.NewInt(7), Pos{1, 20}},
					CompGT,
					Pos{1, 19},
				},
				Pos{1, 15},
			},
			"",
		},
		{
			"* | type Team | ?5<=7",
			&PipelineFilter{
				&PipelineType{&PipelineName{&WildcardName{"", Star, Pos{1, 1}}, Pos{1, 1}}, "Team", Pos{1, 3}},
				&PredicateCompare{
					&ExprInt{big.NewInt(5), Pos{1, 18}},
					&ExprInt{big.NewInt(7), Pos{1, 21}},
					CompLE,
					Pos{1, 19},
				},
				Pos{1, 15},
			},
			"",
		},
		{
			"* | type Team | ?5>=7",
			&PipelineFilter{
				&PipelineType{&PipelineName{&WildcardName{"", Star, Pos{1, 1}}, Pos{1, 1}}, "Team", Pos{1, 3}},
				&PredicateCompare{
					&ExprInt{big.NewInt(5), Pos{1, 18}},
					&ExprInt{big.NewInt(7), Pos{1, 21}},
					CompGE,
					Pos{1, 19},
				},
				Pos{1, 15},
			},
			"",
		},
		{
			"* | type Team | ? 5 > 7", // Test that whitespace is allowed.
			&PipelineFilter{
				&PipelineType{&PipelineName{&WildcardName{"", Star, Pos{1, 1}}, Pos{1, 1}}, "Team", Pos{1, 3}},
				&PredicateCompare{
					&ExprInt{big.NewInt(5), Pos{1, 19}},
					&ExprInt{big.NewInt(7), Pos{1, 23}},
					CompGT,
					Pos{1, 21},
				},
				Pos{1, 15},
			},
			"",
		},
		{
			"* | type Team | ?`foo`>=`bar`", // Compare string literals.
			&PipelineFilter{
				&PipelineType{&PipelineName{&WildcardName{"", Star, Pos{1, 1}}, Pos{1, 1}}, "Team", Pos{1, 3}},
				&PredicateCompare{
					&ExprString{"foo", Pos{1, 18}},
					&ExprString{"bar", Pos{1, 25}},
					CompGE,
					Pos{1, 23},
				},
				Pos{1, 15},
			},
			"",
		},
		{
			"* | type Team | ?2.5>=7", // Compare rational literals.
			&PipelineFilter{
				&PipelineType{&PipelineName{&WildcardName{"", Star, Pos{1, 1}}, Pos{1, 1}}, "Team", Pos{1, 3}},
				&PredicateCompare{
					&ExprRat{big.NewRat(10, 4), Pos{1, 18}},
					&ExprInt{big.NewInt(7), Pos{1, 23}},
					CompGE,
					Pos{1, 21},
				},
				Pos{1, 15},
			},
			"",
		},
		// Propositional ops.
		{
			"* | type Team | ?5=7&&2=3",
			&PipelineFilter{
				&PipelineType{&PipelineName{&WildcardName{"", Star, Pos{1, 1}}, Pos{1, 1}}, "Team", Pos{1, 3}},
				&PredicateAnd{
					&PredicateCompare{
						&ExprInt{big.NewInt(5), Pos{1, 18}},
						&ExprInt{big.NewInt(7), Pos{1, 20}},
						CompEQ,
						Pos{1, 19},
					},
					&PredicateCompare{
						&ExprInt{big.NewInt(2), Pos{1, 23}},
						&ExprInt{big.NewInt(3), Pos{1, 25}},
						CompEQ,
						Pos{1, 24},
					},
					Pos{1, 21},
				},
				Pos{1, 15},
			},
			"",
		},
		{
			"* | type Team | ?true&&false",
			&PipelineFilter{
				&PipelineType{&PipelineName{&WildcardName{"", Star, Pos{1, 1}}, Pos{1, 1}}, "Team", Pos{1, 3}},
				&PredicateAnd{
					&PredicateBool{true, Pos{1, 18}},
					&PredicateBool{false, Pos{1, 24}},
					Pos{1, 22},
				},
				Pos{1, 15},
			},
			"",
		},
		{
			"* | type Team | ?true||false",
			&PipelineFilter{
				&PipelineType{&PipelineName{&WildcardName{"", Star, Pos{1, 1}}, Pos{1, 1}}, "Team", Pos{1, 3}},
				&PredicateOr{
					&PredicateBool{true, Pos{1, 18}},
					&PredicateBool{false, Pos{1, 24}},
					Pos{1, 22},
				},
				Pos{1, 15},
			},
			"",
		},
		{
			"* | type Team | ?!true",
			&PipelineFilter{
				&PipelineType{&PipelineName{&WildcardName{"", Star, Pos{1, 1}}, Pos{1, 1}}, "Team", Pos{1, 3}},
				&PredicateNot{
					&PredicateBool{true, Pos{1, 19}},
					Pos{1, 18},
				},
				Pos{1, 15},
			},
			"",
		},
		{
			"* | type Team | ?!true&&false",
			&PipelineFilter{
				&PipelineType{&PipelineName{&WildcardName{"", Star, Pos{1, 1}}, Pos{1, 1}}, "Team", Pos{1, 3}},
				&PredicateAnd{
					&PredicateNot{
						&PredicateBool{true, Pos{1, 19}},
						Pos{1, 18},
					},
					&PredicateBool{false, Pos{1, 25}},
					Pos{1, 23},
				},
				Pos{1, 15},
			},
			"",
		},
		{
			"* | type Team | ?!!true&&false",
			&PipelineFilter{
				&PipelineType{&PipelineName{&WildcardName{"", Star, Pos{1, 1}}, Pos{1, 1}}, "Team", Pos{1, 3}},
				&PredicateAnd{
					&PredicateNot{
						&PredicateNot{
							&PredicateBool{true, Pos{1, 20}},
							Pos{1, 19},
						},
						Pos{1, 18},
					},
					&PredicateBool{false, Pos{1, 26}},
					Pos{1, 24},
				},
				Pos{1, 15},
			},
			"",
		},
		{
			"* | type Team | ?!true&&!false",
			&PipelineFilter{
				&PipelineType{&PipelineName{&WildcardName{"", Star, Pos{1, 1}}, Pos{1, 1}}, "Team", Pos{1, 3}},
				&PredicateAnd{
					&PredicateNot{
						&PredicateBool{true, Pos{1, 19}},
						Pos{1, 18},
					},
					&PredicateNot{
						&PredicateBool{false, Pos{1, 26}},
						Pos{1, 25},
					},
					Pos{1, 23},
				},
				Pos{1, 15},
			},
			"",
		},
		// Parenthesis should change operator precedence.
		{
			"* | type Team | ?!(true&&false)",
			&PipelineFilter{
				&PipelineType{&PipelineName{&WildcardName{"", Star, Pos{1, 1}}, Pos{1, 1}}, "Team", Pos{1, 3}},
				&PredicateNot{
					&PredicateAnd{
						&PredicateBool{true, Pos{1, 20}},
						&PredicateBool{false, Pos{1, 26}},
						Pos{1, 24},
					},
					Pos{1, 18},
				},
				Pos{1, 15},
			},
			"",
		},
		{
			"* | type Team | ?true&&false||true",
			&PipelineFilter{
				&PipelineType{&PipelineName{&WildcardName{"", Star, Pos{1, 1}}, Pos{1, 1}}, "Team", Pos{1, 3}},
				&PredicateOr{
					&PredicateAnd{
						&PredicateBool{true, Pos{1, 18}},
						&PredicateBool{false, Pos{1, 24}},
						Pos{1, 22},
					},
					&PredicateBool{true, Pos{1, 31}},
					Pos{1, 29},
				},
				Pos{1, 15},
			},
			"",
		},
		{
			"* | type Team | ?true&&(false||true)",
			&PipelineFilter{
				&PipelineType{&PipelineName{&WildcardName{"", Star, Pos{1, 1}}, Pos{1, 1}}, "Team", Pos{1, 3}},
				&PredicateAnd{
					&PredicateBool{true, Pos{1, 18}},
					&PredicateOr{
						&PredicateBool{false, Pos{1, 25}},
						&PredicateBool{true, Pos{1, 32}},
						Pos{1, 30},
					},
					Pos{1, 22},
				},
				Pos{1, 15},
			},
			"",
		},

		///////////////////////////////////////////////////////////////////////////
		// Expressions
		{
			"* | type Team | ?Loc=`CA`",
			&PipelineFilter{
				&PipelineType{&PipelineName{&WildcardName{"", Star, Pos{1, 1}}, Pos{1, 1}}, "Team", Pos{1, 3}},
				&PredicateCompare{
					&ExprName{"Loc", Pos{1, 18}},
					&ExprString{"CA", Pos{1, 22}},
					CompEQ,
					Pos{1, 21},
				},
				Pos{1, 15},
			},
			"",
		},
		{
			"* | type Team | ?Loc/foo=`CA`",
			&PipelineFilter{
				&PipelineType{&PipelineName{&WildcardName{"", Star, Pos{1, 1}}, Pos{1, 1}}, "Team", Pos{1, 3}},
				&PredicateCompare{
					&ExprName{"Loc/foo", Pos{1, 18}},
					&ExprString{"CA", Pos{1, 26}},
					CompEQ,
					Pos{1, 25},
				},
				Pos{1, 15},
			},
			"",
		},
		{ // Make sure a name can go on the RHS of a predicate.
			"* | type Team | ?`CA`==Loc",
			&PipelineFilter{
				&PipelineType{&PipelineName{&WildcardName{"", Star, Pos{1, 1}}, Pos{1, 1}}, "Team", Pos{1, 3}},
				&PredicateCompare{
					&ExprString{"CA", Pos{1, 18}},
					&ExprName{"Loc", Pos{1, 24}},
					CompEQ,
					Pos{1, 22},
				},
				Pos{1, 15},
			},
			"",
		},
		{
			"* | type Team | ?/`Loc`=`CA`",
			&PipelineFilter{
				&PipelineType{&PipelineName{&WildcardName{"", Star, Pos{1, 1}}, Pos{1, 1}}, "Team", Pos{1, 3}},
				&PredicateCompare{
					&ExprName{"Loc", Pos{1, 18}},
					&ExprString{"CA", Pos{1, 25}},
					CompEQ,
					Pos{1, 24},
				},
				Pos{1, 15},
			},
			"",
		},
		{
			"* | type Team | ?isString(Loc/foo)",
			&PipelineFilter{
				&PipelineType{&PipelineName{&WildcardName{"", Star, Pos{1, 1}}, Pos{1, 1}}, "Team", Pos{1, 3}},
				&PredicateFunc{
					"isString",
					[]Expr{&ExprName{"Loc/foo", Pos{1, 27}}},
					Pos{1, 18},
				},
				Pos{1, 15},
			},
			"",
		},
		{
			"* | type Team | ?strip(Loc/foo, `\\n`)=`CA`",
			&PipelineFilter{
				&PipelineType{&PipelineName{&WildcardName{"", Star, Pos{1, 1}}, Pos{1, 1}}, "Team", Pos{1, 3}},
				&PredicateCompare{
					&ExprFunc{
						"strip",
						[]Expr{
							&ExprName{"Loc/foo", Pos{1, 24}},
							&ExprString{"\\n", Pos{1, 33}},
						},
						Pos{1, 18},
					},
					&ExprString{"CA", Pos{1, 39}},
					CompEQ,
					Pos{1, 38},
				},
				Pos{1, 15},
			},
			"",
		},

		///////////////////////////////////////////////////////////////////////////
		// Selection
		{
			"* | type Team | {Name, Location}",
			&PipelineSelection{
				&PipelineType{&PipelineName{&WildcardName{"", Star, Pos{1, 1}}, Pos{1, 1}}, "Team", Pos{1, 3}},
				[]Alias{
					Alias{&PipelineName{&WildcardName{"Name", Self, Pos{1, 18}}, Pos{1, 18}}, "", false},
					Alias{&PipelineName{&WildcardName{"Location", Self, Pos{1, 24}}, Pos{1, 24}}, "", false},
				},
				Pos{1, 15},
			},
			"",
		},
		{
			"* | type Team | {Name, * | type Player | {Age}}",
			&PipelineSelection{
				&PipelineType{&PipelineName{&WildcardName{"", Star, Pos{1, 1}}, Pos{1, 1}}, "Team", Pos{1, 3}},
				[]Alias{
					Alias{&PipelineName{&WildcardName{"Name", Self, Pos{1, 18}}, Pos{1, 18}}, "", false},
					Alias{
						&PipelineSelection{
							&PipelineType{&PipelineName{&WildcardName{"", Star, Pos{1, 24}}, Pos{1, 24}}, "Player", Pos{1, 26}},
							[]Alias{
								Alias{&PipelineName{&WildcardName{"Age", Self, Pos{1, 43}}, Pos{1, 43}}, "", false},
							},
							Pos{1, 40},
						},
						"",
						false,
					},
				},
				Pos{1, 15},
			},
			"",
		},

		///////////////////////////////////////////////////////////////////////////
		// Pipes
		{
			"* | type Team | sort",
			&PipelineFunc{
				&PipelineType{&PipelineName{&WildcardName{"", Star, Pos{1, 1}}, Pos{1, 1}}, "Team", Pos{1, 3}},
				"sort",
				nil,
				Pos{1, 15},
			},
			"",
		},
		{
			"* | type Team | sort()",
			&PipelineFunc{
				&PipelineType{&PipelineName{&WildcardName{"", Star, Pos{1, 1}}, Pos{1, 1}}, "Team", Pos{1, 3}},
				"sort",
				nil,
				Pos{1, 15},
			},
			"",
		},
		{
			"* | type Team | sort(Name)",
			&PipelineFunc{
				&PipelineType{&PipelineName{&WildcardName{"", Star, Pos{1, 1}}, Pos{1, 1}}, "Team", Pos{1, 3}},
				"sort",
				[]Expr{
					&ExprName{"Name", Pos{1, 22}},
				},
				Pos{1, 15},
			},
			"",
		},
		{
			"* | type Team | sort(-Name, +Age)",
			&PipelineFunc{
				&PipelineType{&PipelineName{&WildcardName{"", Star, Pos{1, 1}}, Pos{1, 1}}, "Team", Pos{1, 3}},
				"sort",
				[]Expr{
					&ExprUnary{&ExprName{"Name", Pos{1, 23}}, OpNeg, Pos{1, 22}},
					&ExprUnary{&ExprName{"Age", Pos{1, 30}}, OpPos, Pos{1, 29}},
				},
				Pos{1, 15},
			},
			"",
		},
		{
			"* | type Team | sort(Name) | limit(5)",
			&PipelineFunc{
				&PipelineFunc{
					&PipelineType{&PipelineName{&WildcardName{"", Star, Pos{1, 1}}, Pos{1, 1}}, "Team", Pos{1, 3}},
					"sort",
					[]Expr{
						&ExprName{"Name", Pos{1, 22}},
					},
					Pos{1, 15},
				},
				"limit",
				[]Expr{
					&ExprInt{big.NewInt(5), Pos{1, 36}},
				},
				Pos{1, 28},
			},
			"",
		},
		{
			". | { teams/* | type Team | {NumPlayers} | avg as 'avg age'," +
				"    teams/* | type Team | {NumPlayers} | sum as total_players hidden}",
			&PipelineSelection{
				&PipelineName{&WildcardName{"", Self, Pos{1, 1}}, Pos{1, 1}},
				[]Alias{
					Alias{
						&PipelineFunc{
							&PipelineSelection{
								&PipelineType{&PipelineName{&WildcardName{"teams", Star, Pos{1, 7}}, Pos{1, 7}}, "Team", Pos{1, 15}},
								[]Alias{
									Alias{&PipelineName{&WildcardName{"NumPlayers", Self, Pos{1, 30}}, Pos{1, 30}}, "", false},
								},
								Pos{1, 27},
							},
							"avg",
							nil,
							Pos{1, 42},
						},
						"avg age",
						false,
					},
					Alias{
						&PipelineFunc{
							&PipelineSelection{
								&PipelineType{&PipelineName{&WildcardName{"teams", Star, Pos{1, 65}}, Pos{1, 65}}, "Team", Pos{1, 73}},
								[]Alias{
									Alias{&PipelineName{&WildcardName{"NumPlayers", Self, Pos{1, 88}}, Pos{1, 88}}, "", false},
								},
								Pos{1, 85},
							},
							"sum",
							nil,
							Pos{1, 100},
						},
						"total_players",
						true,
					},
				},
				Pos{1, 3},
			},
			"",
		},
	}

	for _, test := range basic {
		fmt.Println("Testing", test.query)
		ast, err := Parse(query.Query{test.query})
		if test.errRegex == "" {
			if err != nil {
				t.Errorf("query: %s; unexpected error: %v", test.query, err)
			}
		} else {
			if err == nil {
				t.Errorf("query: %s; got success, expected error '%s'", test.query, test.errRegex)
			} else {
				re := regexp.MustCompile(test.errRegex)
				if !re.MatchString(err.Error()) {
					t.Errorf("query: %s; could not find regexp '%s' in output: %v", test.query, test.errRegex, err)
				}
			}
		}
		if test.ast != nil && !reflect.DeepEqual(test.ast, ast) {
			t.Errorf("query: %s;\nGOT  %s\nWANT %s", test.query, ast, test.ast)
		}
	}
}
