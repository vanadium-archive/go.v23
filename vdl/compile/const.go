package compile

import (
	"fmt"
	"math/big"

	"veyron/lib/toposort"
	"veyron2/vdl/parse"
	"veyron2/val"
)

// ConstDef represents a user-defined named const definition in the compiled
// results.
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

// compileConstDefs is the "entry point" to the rest of this file.  It takes the
// consts defined in pfiles and compiles them into ConstDefs in pkg.
func compileConstDefs(pkg *Package, pfiles []*parse.File, env *Env) {
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
//
// It holds a builders map from const name to constBuilder, where the
// constBuilder is responsible for compiling and defining a single const.
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
	case nil, *parse.ConstLit:
		return nil
	case *parse.ConstCompositeLit:
		var deps constBuilderSet
		for _, kv := range pe.KVList {
			deps = mergeConstBuilderSets(deps, cd.getLocalDeps(kv.Key))
			deps = mergeConstBuilderSets(deps, cd.getLocalDeps(kv.Value))
		}
		return deps
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
	panic(fmt.Errorf("vdl: unhandled parse.ConstExpr %T %#v", pexpr, pexpr))
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
	// Evaluate pexpr into val.Const, and turn that into val.Value.
	vc := evalConstExpr(nil, pexpr, file, env)
	if !vc.IsValid() {
		return nil
	}
	vv, err := vc.ToValue()
	if err != nil {
		env.prefixErrorf(file, pexpr.Pos(), err, "final const invalid")
		return nil
	}
	return vv
}

var bigRatZero = new(big.Rat)

// evalConstExpr returns the result of evaluating pexpr into a val.Const.  The
// given implicit type is used to enable evaluation of untyped composite
// literals contained within an enclosing composite literal.
func evalConstExpr(implicit *val.Type, pexpr parse.ConstExpr, file *File, env *Env) val.Const {
	switch pe := pexpr.(type) {
	case *parse.ConstLit:
		// All literal constants start out untyped.
		switch tlit := pe.Lit.(type) {
		case bool:
			return val.BooleanConst(tlit)
		case string:
			return val.StringConst(tlit)
		case *big.Int:
			return val.IntegerConst(tlit)
		case *big.Rat:
			return val.RationalConst(tlit)
		case *parse.BigImag:
			return val.ComplexConst(bigRatZero, (*big.Rat)(tlit))
		default:
			panic(fmt.Errorf("vdl: unhandled parse.ConstLit %T %#v", tlit, tlit))
		}
	case *parse.ConstCompositeLit:
		t := implicit
		if pe.Type != nil {
			// An explicit type specified for the composite literal overrides the
			// implicit type inferred from the enclosing type.
			t = compileType(pe.Type, file, env)
			if t == nil {
				break
			}
		}
		v := evalCompLit(t, pe, file, env)
		if v == nil {
			break
		}
		return val.ConstFromValue(v)
	case *parse.ConstNamed:
		def := env.ResolveConst(pe.Name, file)
		if def == nil {
			env.errorf(file, pe.Pos(), "const %s undefined", pe.Name)
			break
		}
		return val.ConstFromValue(def.Value)
	case *parse.ConstTypeConv:
		t := compileType(pe.Type, file, env)
		x := evalConstExpr(nil, pe.Expr, file, env)
		if t == nil || !x.IsValid() {
			break
		}
		res, err := x.Convert(t)
		if err != nil {
			env.prefixErrorf(file, pe.Pos(), err, "invalid type conversion")
			break
		}
		return res
	case *parse.ConstUnaryOp:
		x := evalConstExpr(nil, pe.Expr, file, env)
		op := val.ToUnaryOp(pe.Op)
		if op == val.InvalidUnaryOp {
			env.errorf(file, pe.Pos(), "unary %s undefined", pe.Op)
			break
		}
		if !x.IsValid() {
			break
		}
		res, err := val.EvalUnary(op, x)
		if err != nil {
			env.prefixErrorf(file, pe.Pos(), err, "unary %s invalid", pe.Op)
			break
		}
		return res
	case *parse.ConstBinaryOp:
		x := evalConstExpr(nil, pe.Lexpr, file, env)
		y := evalConstExpr(nil, pe.Rexpr, file, env)
		op := val.ToBinaryOp(pe.Op)
		if op == val.InvalidBinaryOp {
			env.errorf(file, pe.Pos(), "binary %s undefined", pe.Op)
			break
		}
		if !x.IsValid() || !y.IsValid() {
			break
		}
		res, err := val.EvalBinary(op, x, y)
		if err != nil {
			env.prefixErrorf(file, pe.Pos(), err, "binary %s invalid", pe.Op)
			break
		}
		return res
	default:
		panic(fmt.Errorf("vdl: unhandled parse.ConstExpr %T %#v", pexpr, pexpr))
	}
	return val.Const{}
}

// evalCompLit evaluates a composite literal, returning it as a val.Value.  The
// type t is required, but note that subtypes enclosed in a composite type can
// always use the implicit type from the parent composite type.
func evalCompLit(t *val.Type, lit *parse.ConstCompositeLit, file *File, env *Env) *val.Value {
	if t == nil {
		env.errorf(file, lit.Pos(), "missing type for composite literal")
		return nil
	}
	switch t.Kind() {
	case val.List:
		return evalListLit(t, lit, file, env)
	case val.Map:
		return evalMapLit(t, lit, file, env)
	case val.Struct:
		return evalStructLit(t, lit, file, env)
	}
	env.errorf(file, lit.Pos(), "%v invalid type for composite literal", t)
	return nil
}

func evalListLit(t *val.Type, lit *parse.ConstCompositeLit, file *File, env *Env) *val.Value {
	listv := val.Zero(t)
	var index int
	assigned := make(map[int]bool)
	for _, kv := range lit.KVList {
		if kv.Value == nil {
			env.errorf(file, lit.Pos(), "missing value in list literal")
			return nil
		}
		// Set the index to the key, if it exists.  Semantics are looser than
		// values; we allow any key that's convertible to uint64, even if the key is
		// already typed.
		if kv.Key != nil {
			key := evalConstExpr(nil, kv.Key, file, env)
			if !key.IsValid() {
				return nil
			}
			ckey, err := key.Convert(val.Uint64Type)
			if err != nil {
				env.prefixErrorf(file, kv.Key.Pos(), err, "invalid list key")
				return nil
			}
			vkey, err := ckey.ToValue()
			if err != nil {
				env.prefixErrorf(file, kv.Key.Pos(), err, "invalid list key")
				return nil
			}
			index = int(vkey.Uint())
		}
		// Make sure the index hasn't been assigned already, and adjust the list
		// length as necessary.
		if assigned[index] {
			env.errorf(file, kv.Value.Pos(), "duplicate index %d in list literal", index)
			return nil
		}
		assigned[index] = true
		if index >= listv.Len() {
			listv.AssignLen(index + 1)
		}
		// Evaluate the value and perform the assignment.
		value := evalTypedValue(t.Elem(), kv.Value, file, env)
		if value == nil {
			return nil
		}
		listv.Index(index).Assign(value)
		index++
	}
	return listv
}

func evalMapLit(t *val.Type, lit *parse.ConstCompositeLit, file *File, env *Env) *val.Value {
	mapv := val.Zero(t)
	for _, kv := range lit.KVList {
		if kv.Key == nil {
			env.errorf(file, lit.Pos(), "missing key in map literal")
			return nil
		}
		if kv.Value == nil {
			env.errorf(file, lit.Pos(), "missing elem in map literal")
			return nil
		}
		// Evaluate the key and make sure it hasn't been assigned already.
		key := evalTypedValue(t.Key(), kv.Key, file, env)
		if key == nil {
			return nil
		}
		if mapv.MapIndex(key) != nil {
			env.errorf(file, kv.Key.Pos(), "duplicate key %v in map literal", key)
			return nil
		}
		// Evaluate the value and perform the assignment.
		value := evalTypedValue(t.Elem(), kv.Value, file, env)
		if value == nil {
			return nil
		}
		mapv.AssignMapIndex(key, value)
	}
	return mapv
}

func evalStructLit(t *val.Type, lit *parse.ConstCompositeLit, file *File, env *Env) *val.Value {
	// We require that either all items have keys, or none of them do.
	structv := val.Zero(t)
	haskeys := len(lit.KVList) > 0 && lit.KVList[0].Key != nil
	assigned := make(map[int]bool)
	for index, kv := range lit.KVList {
		if kv.Value == nil {
			env.errorf(file, lit.Pos(), "missing field value in %s struct literal", t.Name())
			return nil
		}
		if haskeys != (kv.Key != nil) {
			env.errorf(file, kv.Value.Pos(), "mixed key:value and value in %s struct literal", t.Name())
			return nil
		}
		// Get the field description, either from the key or the index.
		var field val.StructField
		if kv.Key != nil {
			// There is an explicit field name specified.
			fname, ok := kv.Key.(*parse.ConstNamed)
			if !ok {
				env.errorf(file, kv.Key.Pos(), "invalid field name %q in %s struct literal", kv.Key.String(), t.Name())
				return nil
			}
			field, index = t.FieldByName(fname.Name)
			if index < 0 {
				env.errorf(file, kv.Key.Pos(), "unknown field %q in %s struct literal", fname.Name, t.Name())
				return nil
			}
		} else {
			// No field names, just use the index position.
			if index >= t.NumField() {
				env.errorf(file, kv.Value.Pos(), "too many fields in %s struct literal", t.Name())
				return nil
			}
			field = t.Field(index)
		}
		// Make sure the field hasn't been assigned already.
		if assigned[index] {
			env.errorf(file, kv.Value.Pos(), "duplicate field %q in %s struct literal", field.Name, t.Name())
			return nil
		}
		assigned[index] = true
		// Evaluate the value and perform the assignment.
		value := evalTypedValue(field.Type, kv.Value, file, env)
		if value == nil {
			return nil
		}
		structv.Field(index).Assign(value)
	}
	if !haskeys && 0 < len(assigned) && len(assigned) < t.NumField() {
		env.errorf(file, lit.Pos(), "too few fields in %s struct literal", t.Name())
		return nil
	}
	return structv
}

// evalTypedValue evaluates pexpr into a val.Value.  If a non-nil value is
// returned, it's guaranteed that the type of the returned value is assignable
// to the given target type.
func evalTypedValue(target *val.Type, pexpr parse.ConstExpr, file *File, env *Env) *val.Value {
	c := evalConstExpr(target, pexpr, file, env)
	if !c.IsValid() {
		return nil
	}
	if c.Type() != nil {
		// Typed const - confirm it may be assigned to the target type.
		if !target.AssignableFrom(c.Type()) {
			env.errorf(file, pexpr.Pos(), "%v not assignable from %v", target, c)
			return nil
		}
	} else {
		// Untyped const - make it typed by converting to the target type.
		convert, err := c.Convert(target)
		if err != nil {
			env.prefixErrorf(file, pexpr.Pos(), err, "invalid value")
			return nil
		}
		c = convert
	}
	// ToValue should always succeed, since the const is now typed.
	v, err := c.ToValue()
	if err != nil {
		env.prefixErrorf(file, pexpr.Pos(), err, "internal error")
	}
	return v
}
