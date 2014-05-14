package compile

import (
	"errors"
	"fmt"
	"math/big"

	"veyron/lib/toposort"
	"veyron2/idl2/parse"
	"veyron2/val"
)

// ConstDef represents a user-defined named const definition.
type ConstDef struct {
	NamePos            // name, parse position and docs
	Value   *val.Value // const value
	File    *File      // parent file that this const is defined in
}

func (x *ConstDef) String() string {
	c := *x
	c.File = nil // avoid infinite loop
	return fmt.Sprintf("%+v", c)
}

// defineConstDefs is the "entry point" to the rest of this file.  It takes
// the types defined in pfiles and defines them in pkg.
func defineConstDefs(pkg *Package, pfiles []*parse.File, env *Env) {
	cd := constDefiner{pkg, pfiles, env, make(map[string]*constBuilder)}
	if cd.Declare(); !env.Errors.IsEmpty() {
		return
	}
	cd.SortAndDefine()
}

// constDefiner defines consts in a package.  This is split into two phases:
// 1) Declare ensures local const references can be resolved.
// 2) SortAndDefine sorts in dependency order, and evaluates and defines each
//    const.
type constDefiner struct {
	pkg      *Package
	pfiles   []*parse.File
	env      *Env
	builders map[string]*constBuilder
}

type constBuilder struct {
	def   *ConstDef
	pexpr parse.ConstExpr
}

func printConstBuilderName(ibuilder interface{}) string {
	return ibuilder.(*constBuilder).def.Name
}

// Declare creates builders for each const defined in the package.
func (cd constDefiner) Declare() {
	for ix := range cd.pkg.Files {
		file, pfile := cd.pkg.Files[ix], cd.pfiles[ix]
		for _, pdef := range pfile.ConstDefs {
			if err := ValidIdent(pdef.Name); err != nil {
				cd.env.errorf(file, pdef.Pos, "const %s invalid name (%s)", pdef.Name, err)
				continue // keep going to catch more errors
			}
			if b, dup := cd.builders[pdef.Name]; dup {
				cd.env.errorf(file, pdef.Pos, "const %s redefined (previous at %s)", pdef.Name, fpString(b.def.File, b.def.Pos))
				continue // keep going to catch more errors
			}
			def := &ConstDef{NamePos: NamePos(pdef.NamePos), File: file}
			cd.builders[pdef.Name] = &constBuilder{def, pdef.Expr}
		}
	}
}

// Sort and define consts.  We sort by dependencies on other named consts in
// this package.  We don't allow cycles.  The ordering is necessary to perform
// simple single-pass evaluation.
//
// The dependency analysis is performed on consts, not the files they occur in;
// consts in the same package may be defined in any file, even if they cause
// cyclic file dependencies.
func (cd constDefiner) SortAndDefine() {
	// Populate sorter with dependency information.  The sorting ensures that the
	// list of const defs within each file is topologically sorted, and also
	// deterministic; other than dependencies, const defs are listed in the same
	// order they were defined in the parsed files.
	sorter := toposort.NewSorter()
	for _, pfile := range cd.pfiles {
		for _, pdef := range pfile.ConstDefs {
			b := cd.builders[pdef.Name]
			sorter.AddNode(b)
			for dep, _ := range cd.getLocalDeps(b.pexpr) {
				sorter.AddEdge(b, dep)
			}
		}
	}
	// Sort and check for cycles.
	sorted, cycles := sorter.Sort()
	if len(cycles) > 0 {
		cycleStr := toposort.PrintCycles(cycles, printConstBuilderName)
		first := cycles[0][0].(*constBuilder)
		cd.env.errorf(first.def.File, first.def.Pos, "package %v has cyclic consts: %v", cd.pkg.Name, cycleStr)
		return
	}
	// Define all consts.  Since we add the const defs as we go and evaluate in
	// topological order, dependencies are guaranteed to be resolvable when we get
	// around to evaluating the consts that depend on them.
	for _, ibuilder := range sorted {
		b := ibuilder.(*constBuilder)
		def, file := b.def, b.def.File
		if value := compileConst(b.pexpr, file, cd.env); value != nil {
			def.Value = value
			addConstDef(def, cd.env)
		}
	}
}

// addConstDef updates our various structures to add a new const def.
func addConstDef(def *ConstDef, env *Env) {
	def.File.ConstDefs = append(def.File.ConstDefs, def)
	def.File.Package.constDefs[def.Name] = def
	if env != nil {
		// env should only be nil during initialization of the global package;
		// NewEnv ensures new environments have the global types.
		env.constDefs[def.Value] = def
	}
}

// getLocalDeps returns the set of named const dependencies for pexpr that are
// in this package.
func (cd constDefiner) getLocalDeps(pexpr parse.ConstExpr) constBuilderSet {
	switch pe := pexpr.(type) {
	case *parse.ConstLit:
		return nil
	case *parse.ConstNamed:
		// Named references to other consts in this package are all we care about.
		if b := cd.builders[pe.Name]; b != nil {
			return constBuilderSet{b: true}
		}
		return nil
	case *parse.ConstTypeConv:
		return cd.getLocalDeps(pe.Expr)
	case *parse.ConstUnaryOp:
		return cd.getLocalDeps(pe.Expr)
	case *parse.ConstBinaryOp:
		l, r := cd.getLocalDeps(pe.Lexpr), cd.getLocalDeps(pe.Rexpr)
		return mergeConstBuilderSets(l, r)
	}
	panic(fmt.Errorf("idl: unhandled parse.ConstExpr %T %#v", pexpr, pexpr))
}

type constBuilderSet map[*constBuilder]bool

// mergeConstBuilderSets returns the union of a and b.  It may mutate either a
// or b and return the mutated set as a result.
func mergeConstBuilderSets(a, b constBuilderSet) constBuilderSet {
	if a != nil {
		for builder, _ := range b {
			a[builder] = true
		}
		return a
	}
	return b
}

// compileConst compiles pexpr into a *val.Value.  All named types and consts
// referenced by pexpr must already be defined.
func compileConst(pexpr parse.ConstExpr, file *File, env *Env) *val.Value {
	// Evaluate pexpr into a constVal.
	cvRes, err := evalCV(pexpr, file, env)
	if err != nil {
		env.Errors.Error(err.Error())
		return nil
	}
	// Convert constVal into a final *val.Value.
	res, err := cvToFinalValue(cvRes)
	if err != nil {
		env.errorf(file, pexpr.Pos(), "final const invalid (%s)", err)
		return nil
	}
	return res
}

// evalCV returns the result of evaluating pexpr into a constVal.  Unlike other
// compiler functions, this returns the first error it encounters, and doesn't
// update env.Errors.
func evalCV(pexpr parse.ConstExpr, file *File, env *Env) (constVal, error) {
	switch pe := pexpr.(type) {
	case *parse.ConstLit:
		// All literal constants start out untyped.
		switch tl := pe.Lit.(type) {
		case bool, string, *big.Int, *big.Rat:
			return constVal{tl, nil}, nil
		case *parse.BigImag:
			return constVal{bigComplex{re: bigRatZero, im: (*big.Rat)(tl)}, nil}, nil
		default:
			panic(fmt.Errorf("idl: unhandled parse.ConstLit %T %#v", tl, tl))
		}
	case *parse.ConstNamed:
		if def := env.ResolveConst(pe.Name, file); def != nil {
			return cvFromFinalValue(def.Value)
		}
		return constVal{}, fpErrorf(file, pe.Pos(), "const %s undefined", pe.Name)
	case *parse.ConstTypeConv:
		x, err := evalCV(pe.Expr, file, env)
		if err != nil {
			return constVal{}, err
		}
		res, err := makeConstVal(x.Val, compileType(pe.Type, file, env))
		return res, errPrefix(err, fpStringf(file, pe.Pos(), "type mismatch"))
	case *parse.ConstUnaryOp:
		x, err := evalCV(pe.Expr, file, env)
		if err != nil {
			return constVal{}, err
		}
		res, err := evalUnaryOp(pe.Op, x)
		return res, errPrefix(err, fpStringf(file, pe.Pos(), "unary %s invalid", pe.Op))
	case *parse.ConstBinaryOp:
		x, err := evalCV(pe.Lexpr, file, env)
		if err != nil {
			return constVal{}, err
		}
		y, err := evalCV(pe.Rexpr, file, env)
		if err != nil {
			return constVal{}, err
		}
		res, err := evalBinaryOp(pe.Op, x, y)
		return res, errPrefix(err, fpStringf(file, pe.Pos(), "binary %s invalid", pe.Op))
	default:
		panic(fmt.Errorf("idl: unhandled parse.ConstExpr %T %#v", pexpr, pexpr))
	}
}

func errPrefix(err error, pre string) error {
	if err != nil {
		return errors.New(pre + " (" + err.Error() + ")")
	}
	return nil
}
