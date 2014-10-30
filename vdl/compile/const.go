package compile

import (
	"fmt"
	"math/big"

	"veyron.io/veyron/veyron/lib/toposort"
	"veyron.io/veyron/veyron2/vdl"
	"veyron.io/veyron/veyron2/vdl/opconst"
	"veyron.io/veyron/veyron2/vdl/parse"
)

// ConstDef represents a user-defined named const definition in the compiled
// results.
type ConstDef struct {
	NamePos             // name, parse position and docs
	Exported bool       // is this const definition exported?
	Value    *vdl.Value // const value
	File     *File      // parent file that this const is defined in
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
			export, err := ValidIdent(pdef.Name)
			if err != nil {
				cd.env.prefixErrorf(file, pdef.Pos, err, "const %s invalid name", pdef.Name)
				continue // keep going to catch more errors
			}
			detail := identDetail("const", file, pdef.Pos)
			if err := file.DeclareIdent(pdef.Name, detail); err != nil {
				cd.env.prefixErrorf(file, pdef.Pos, err, "const %s name conflict", pdef.Name)
				continue
			}
			def := &ConstDef{NamePos: NamePos(pdef.NamePos), Exported: export, File: file}
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
	var sorter toposort.Sorter
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
		cycleStr := toposort.DumpCycles(cycles, printConstBuilderName)
		first := cycles[0][0].(*constBuilder)
		cd.env.Errorf(first.def.File, first.def.Pos, "package %v has cyclic consts: %v", cd.pkg.Name, cycleStr)
		return
	}
	// Define all consts.  Since we add the const defs as we go and evaluate in
	// topological order, dependencies are guaranteed to be resolvable when we get
	// around to evaluating the consts that depend on them.
	for _, ibuilder := range sorted {
		b := ibuilder.(*constBuilder)
		def, file := b.def, b.def.File
		if value := compileConst(nil, b.pexpr, file, cd.env); value != nil {
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
		// env should only be nil during initialization of the built-in package;
		// NewEnv ensures new environments have the built-in consts.
		env.constDefs[def.Value] = def
	}
}

// getLocalDeps returns the set of named const dependencies for pexpr that are
// in this package.
func (cd constDefiner) getLocalDeps(pexpr parse.ConstExpr) constBuilderSet {
	switch pe := pexpr.(type) {
	case nil, *parse.ConstLit, *parse.ConstTypeObject:
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
	case *parse.ConstIndexed:
		e, i := cd.getLocalDeps(pe.Expr), cd.getLocalDeps(pe.IndexExpr)
		return mergeConstBuilderSets(e, i)
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

// compileConst compiles pexpr into a *vdl.Value.  All named types and consts
// referenced by pexpr must already be defined.  If implicit is non-nil, untyped
// composite literals are assumed to be of that type.
func compileConst(implicit *vdl.Type, pexpr parse.ConstExpr, file *File, env *Env) *vdl.Value {
	// Evaluate pexpr into opconst.Const, and turn that into vdl.Value.
	vc := evalConstExpr(implicit, pexpr, file, env)
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

// evalConstExpr returns the result of evaluating pexpr into a opconst.Const.
// If implicit is non-nil, untyped composite literals are assumed to be of that
// type.
func evalConstExpr(implicit *vdl.Type, pexpr parse.ConstExpr, file *File, env *Env) opconst.Const {
	switch pe := pexpr.(type) {
	case *parse.ConstLit:
		// All literal constants start out untyped.
		switch tlit := pe.Lit.(type) {
		case string:
			return opconst.String(tlit)
		case *big.Int:
			return opconst.Integer(tlit)
		case *big.Rat:
			return opconst.Rational(tlit)
		case *parse.BigImag:
			return opconst.Complex(bigRatZero, (*big.Rat)(tlit))
		default:
			panic(fmt.Errorf("vdl: unhandled parse.ConstLit %T %#v", tlit, tlit))
		}
	case *parse.ConstCompositeLit:
		t := implicit
		if pe.Type != nil {
			// If an explicit type is specified for the composite literal, it
			// overrides the implicit type.
			t = compileType(pe.Type, file, env)
			if t == nil {
				break
			}
		}
		v := evalCompLit(t, pe, file, env)
		if v == nil {
			break
		}
		return opconst.FromValue(v)
	case *parse.ConstNamed:
		c, err := env.EvalConst(pe.Name, file)
		if err != nil {
			env.prefixErrorf(file, pe.Pos(), err, "const %s invalid", pe.Name)
			break
		}
		return c
	case *parse.ConstIndexed:
		expr := evalConstExpr(nil, pe.Expr, file, env)
		if !expr.IsValid() {
			break
		}
		value, err := expr.ToValue()
		if err != nil {
			env.prefixErrorf(file, pe.Pos(), err, "error converting expression to value")
			break
		}
		// TODO(bprosnitz) Should indexing on set also be supported?
		switch value.Kind() {
		case vdl.Array, vdl.List:
			v := evalListIndex(value, pe.IndexExpr, file, env)
			if v != nil {
				return opconst.FromValue(v)
			}
		case vdl.Map:
			v := evalMapIndex(value, pe.IndexExpr, file, env)
			if v != nil {
				return opconst.FromValue(v)
			}
		default:
			env.Errorf(file, pe.Pos(), "illegal use of index operator with unsupported type")
		}
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
	case *parse.ConstTypeObject:
		t := compileType(pe.Type, file, env)
		if t == nil {
			break
		}
		return opconst.FromValue(vdl.TypeObjectValue(t))
	case *parse.ConstUnaryOp:
		x := evalConstExpr(nil, pe.Expr, file, env)
		op := opconst.ToUnaryOp(pe.Op)
		if op == opconst.InvalidUnaryOp {
			env.Errorf(file, pe.Pos(), "unary %s undefined", pe.Op)
			break
		}
		if !x.IsValid() {
			break
		}
		res, err := opconst.EvalUnary(op, x)
		if err != nil {
			env.prefixErrorf(file, pe.Pos(), err, "unary %s invalid", pe.Op)
			break
		}
		return res
	case *parse.ConstBinaryOp:
		x := evalConstExpr(nil, pe.Lexpr, file, env)
		y := evalConstExpr(nil, pe.Rexpr, file, env)
		op := opconst.ToBinaryOp(pe.Op)
		if op == opconst.InvalidBinaryOp {
			env.Errorf(file, pe.Pos(), "binary %s undefined", pe.Op)
			break
		}
		if !x.IsValid() || !y.IsValid() {
			break
		}
		res, err := opconst.EvalBinary(op, x, y)
		if err != nil {
			env.prefixErrorf(file, pe.Pos(), err, "binary %s invalid", pe.Op)
			break
		}
		return res
	default:
		panic(fmt.Errorf("vdl: unhandled parse.ConstExpr %T %#v", pexpr, pexpr))
	}
	return opconst.Const{}
}

// Evaluate an indexed list or array to a constant.
// Inputs are representative of expr[index].
func evalListIndex(exprVal *vdl.Value, indexExpr parse.ConstExpr, file *File, env *Env) *vdl.Value {
	index := evalConstExpr(nil, indexExpr, file, env)
	if !index.IsValid() {
		return nil
	}
	convertedIndex, err := index.Convert(vdl.Uint64Type)
	if err != nil {
		env.prefixErrorf(file, indexExpr.Pos(), err, "error converting index")
		return nil
	}
	indexVal, err := convertedIndex.ToValue()
	if err != nil {
		env.prefixErrorf(file, indexExpr.Pos(), err, "error converting index to value")
		return nil
	}

	if indexVal.Uint() >= uint64(exprVal.Len()) {
		env.Errorf(file, indexExpr.Pos(), "index out of bounds of array")
		return nil
	}
	return exprVal.Index(int(indexVal.Uint()))
}

// Evaluate an indexed map to a constant.
// Inputs are representative of expr[index].
func evalMapIndex(exprVal *vdl.Value, indexExpr parse.ConstExpr, file *File, env *Env) *vdl.Value {
	indexVal := evalTypedValue("map index", exprVal.Type().Key(), indexExpr, file, env)
	if indexVal == nil {
		return nil
	}

	mapItemVal := exprVal.MapIndex(indexVal)
	if mapItemVal == nil {
		// Unlike normal go code, it is probably undesirable to return nil here.
		// It is very likely this is an error.
		env.Errorf(file, indexExpr.Pos(), "map key not in map")
		return nil
	}
	return mapItemVal
}

// evalCompLit evaluates a composite literal, returning it as a vdl.Value.  The
// type t is required, but note that subtypes enclosed in a composite type can
// always use the implicit type from the parent composite type.
func evalCompLit(t *vdl.Type, lit *parse.ConstCompositeLit, file *File, env *Env) *vdl.Value {
	if t == nil {
		env.Errorf(file, lit.Pos(), "missing type for composite literal")
		return nil
	}
	switch t.Kind() {
	case vdl.Array, vdl.List:
		return evalListLit(t, lit, file, env)
	case vdl.Set:
		return evalSetLit(t, lit, file, env)
	case vdl.Map:
		return evalMapLit(t, lit, file, env)
	case vdl.Struct:
		return evalStructLit(t, lit, file, env)
	}
	env.Errorf(file, lit.Pos(), "%v invalid type for composite literal", t)
	return nil
}

func evalListLit(t *vdl.Type, lit *parse.ConstCompositeLit, file *File, env *Env) *vdl.Value {
	listv := vdl.ZeroValue(t)
	desc := fmt.Sprintf("%v %s literal", t, t.Kind())
	var index int
	assigned := make(map[int]bool)
	for _, kv := range lit.KVList {
		if kv.Value == nil {
			env.Errorf(file, lit.Pos(), "missing value in %s", desc)
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
			ckey, err := key.Convert(vdl.Uint64Type)
			if err != nil {
				env.prefixErrorf(file, kv.Key.Pos(), err, "invalid index in %s", desc)
				return nil
			}
			vkey, err := ckey.ToValue()
			if err != nil {
				env.prefixErrorf(file, kv.Key.Pos(), err, "invalid index in %s", desc)
				return nil
			}
			index = int(vkey.Uint())
		}
		// Make sure the index hasn't been assigned already, and adjust the list
		// length as necessary.
		if assigned[index] {
			env.Errorf(file, kv.Value.Pos(), "duplicate index %d in %s", index, desc)
			return nil
		}
		assigned[index] = true
		if index >= listv.Len() {
			if t.Kind() == vdl.Array {
				env.Errorf(file, kv.Value.Pos(), "index %d out of range in %s", index, desc)
				return nil
			}
			listv.AssignLen(index + 1)
		}
		// Evaluate the value and perform the assignment.
		value := evalTypedValue(t.Kind().String()+" value", t.Elem(), kv.Value, file, env)
		if value == nil {
			return nil
		}
		listv.Index(index).Assign(value)
		index++
	}
	return listv
}

func evalSetLit(t *vdl.Type, lit *parse.ConstCompositeLit, file *File, env *Env) *vdl.Value {
	setv := vdl.ZeroValue(t)
	desc := fmt.Sprintf("%v set literal", t)
	for _, kv := range lit.KVList {
		if kv.Key != nil {
			env.Errorf(file, kv.Key.Pos(), "invalid index in %s", desc)
			return nil
		}
		if kv.Value == nil {
			env.Errorf(file, lit.Pos(), "missing key in %s", desc)
			return nil
		}
		// Evaluate the key and make sure it hasn't been assigned already.
		key := evalTypedValue("set key", t.Key(), kv.Value, file, env)
		if key == nil {
			return nil
		}
		if setv.ContainsKey(key) {
			env.Errorf(file, kv.Value.Pos(), "duplicate key %v in %s", key, desc)
			return nil
		}
		setv.AssignSetKey(key)
	}
	return setv
}

func evalMapLit(t *vdl.Type, lit *parse.ConstCompositeLit, file *File, env *Env) *vdl.Value {
	mapv := vdl.ZeroValue(t)
	desc := fmt.Sprintf("%v map literal", t)
	for _, kv := range lit.KVList {
		if kv.Key == nil {
			env.Errorf(file, lit.Pos(), "missing key in %s", desc)
			return nil
		}
		if kv.Value == nil {
			env.Errorf(file, lit.Pos(), "missing elem in %s", desc)
			return nil
		}
		// Evaluate the key and make sure it hasn't been assigned already.
		key := evalTypedValue("map key", t.Key(), kv.Key, file, env)
		if key == nil {
			return nil
		}
		if mapv.ContainsKey(key) {
			env.Errorf(file, kv.Key.Pos(), "duplicate key %v in %s", key, desc)
			return nil
		}
		// Evaluate the value and perform the assignment.
		value := evalTypedValue("map value", t.Elem(), kv.Value, file, env)
		if value == nil {
			return nil
		}
		mapv.AssignMapIndex(key, value)
	}
	return mapv
}

func evalStructLit(t *vdl.Type, lit *parse.ConstCompositeLit, file *File, env *Env) *vdl.Value {
	// We require that either all items have keys, or none of them do.
	structv := vdl.ZeroValue(t)
	desc := fmt.Sprintf("%v struct literal", t)
	haskeys := len(lit.KVList) > 0 && lit.KVList[0].Key != nil
	assigned := make(map[int]bool)
	for index, kv := range lit.KVList {
		if kv.Value == nil {
			env.Errorf(file, lit.Pos(), "missing field value in %s", desc)
			return nil
		}
		if haskeys != (kv.Key != nil) {
			env.Errorf(file, kv.Value.Pos(), "mixed key:value and value in %s", desc)
			return nil
		}
		// Get the field description, either from the key or the index.
		var field vdl.StructField
		if kv.Key != nil {
			// There is an explicit field name specified.
			fname, ok := kv.Key.(*parse.ConstNamed)
			if !ok {
				env.Errorf(file, kv.Key.Pos(), "invalid field name %q in %s", kv.Key.String(), desc)
				return nil
			}
			field, index = t.FieldByName(fname.Name)
			if index < 0 {
				env.Errorf(file, kv.Key.Pos(), "unknown field %q in %s", fname.Name, desc)
				return nil
			}
		} else {
			// No field names, just use the index position.
			if index >= t.NumField() {
				env.Errorf(file, kv.Value.Pos(), "too many fields in %s", desc)
				return nil
			}
			field = t.Field(index)
		}
		// Make sure the field hasn't been assigned already.
		if assigned[index] {
			env.Errorf(file, kv.Value.Pos(), "duplicate field %q in %s", field.Name, desc)
			return nil
		}
		assigned[index] = true
		// Evaluate the value and perform the assignment.
		value := evalTypedValue("struct field", field.Type, kv.Value, file, env)
		if value == nil {
			return nil
		}
		structv.Field(index).Assign(value)
	}
	if !haskeys && 0 < len(assigned) && len(assigned) < t.NumField() {
		env.Errorf(file, lit.Pos(), "too few fields in %s", desc)
		return nil
	}
	return structv
}

// evalTypedValue evaluates pexpr into a vdl.Value.  If a non-nil value is
// returned, it's guaranteed that the type of the returned value is assignable
// to the given target type.
func evalTypedValue(what string, target *vdl.Type, pexpr parse.ConstExpr, file *File, env *Env) *vdl.Value {
	c := evalConstExpr(target, pexpr, file, env)
	if !c.IsValid() {
		return nil
	}
	if c.Type() != nil {
		// Typed const - confirm it may be assigned to the target type.
		if !target.AssignableFrom(c.Type()) {
			env.Errorf(file, pexpr.Pos(), "invalid %v (%v not assignable from %v)", what, target, c)
			return nil
		}
	} else {
		// Untyped const - make it typed by converting to the target type.
		convert, err := c.Convert(target)
		if err != nil {
			env.prefixErrorf(file, pexpr.Pos(), err, "invalid %v", what)
			return nil
		}
		c = convert
	}
	// ToValue should always succeed, since the const is now typed.
	v, err := c.ToValue()
	if err != nil {
		env.prefixErrorf(file, pexpr.Pos(), err, "internal error: invalid %v")
	}
	return v
}

var (
	// Built-in consts defined by the compiler.
	TrueConst  = vdl.BoolValue(true)
	FalseConst = vdl.BoolValue(false)
)
