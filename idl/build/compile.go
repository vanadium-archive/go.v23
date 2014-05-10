package build

import (
	"fmt"

	"veyron/lib/toposort"
)

// CompilePackage compiles a BuildPackage into a Package.  Returns the compiled
// package on success, or returns nil and guarantees !env.Errors.IsEmpty().
//
// All imports that this build package depend on must already have been compiled
// and populated into env, so that we can resolve dependencies.  See
// GetTransitiveTargets for a simple way to get all dependent build packages in
// the right order.
func CompilePackage(build *BuildPackage, env *Env) *Package {
	vlog.Printf("Compiling package name %q path %q dir %q", build.Name, build.Path, build.Dir)
	if len(build.Name) == 0 {
		env.Errors.Errorf("empty package name")
		return nil
	}
	// Open all idl files.
	idlFiles, err := build.OpenIDLFiles()
	defer CloseIDLFiles(idlFiles)
	if err != nil {
		env.Errors.Errorf("Couldn't open idl files %v, %v", idlFiles, err)
		return nil
	}
	// Parse each file and put it in pkg, collecting errors in env.
	pkg := NewPackage(build.Name, build.Path, build.Dir)
	for baseFileName, src := range idlFiles {
		file := parseFile(baseFileName, src, parseOpts{ImportsOnly: false}, env)
		if file != nil {
			file.Package = pkg
			pkg.Files = append(pkg.Files, file)
		}
	}
	if !env.Errors.IsEmpty() {
		return nil
	}
	// Define data types, check dependencies, resolve arg types and verify.  The
	// order of these operations matters; e.g. we must define data types before
	// checking dependencies or resolving arg types, since they depend on the
	// defined data types.
	if defineInterfaces(pkg, env); !env.Errors.IsEmpty() {
		return nil
	}
	if defineDataTypes(pkg, env); !env.Errors.IsEmpty() {
		return nil
	}
	if defineConsts(pkg, env); !env.Errors.IsEmpty() {
		return nil
	}
	if defineTags(pkg, env); !env.Errors.IsEmpty() {
		return nil
	}
	if resolveArgTypes(pkg, env); !env.Errors.IsEmpty() {
		return nil
	}
	if computePerFilePackageDeps(pkg, env); !env.Errors.IsEmpty() {
		return nil
	}
	if pkg.Verify(env); !env.Errors.IsEmpty() {
		return nil
	}
	env.pkgs[pkg.Path] = pkg
	return pkg
}

func defineInterfaces(pkg *Package, env *Env) {
	for _, file := range pkg.Files {
		for _, iface := range file.Interfaces {
			def := iface.Def
			if err := def.declare(); err != nil {
				env.Errors.Errorf("%v:%v package %v invalid interface def %v: %v",
					file.BaseName, def.Pos, pkg.Name, iface.Name, err.Error())
			}
			if err := def.define(env.pkgs); err != nil {
				env.Errors.Errorf("%v:%v package %v invalid interface def %v: %v",
					file.BaseName, def.Pos, pkg.Name, iface.Name, err.Error())
			}
		}
	}
	// TODO(caprita): Perform dependency sorting, as we do for types.
	// Not needed for Go, so skipping for now.
}
func defineDataTypes(pkg *Package, env *Env) {
	// This is a two step process: 1) declare all the types that are defined in
	// the files in this package, and 2) define the types.  We need two steps to
	// handle recursive types.
	for _, file := range pkg.Files {
		for _, def := range file.TypeDefs {
			if err := def.declare(); err != nil {
				env.Errors.Errorf("%v:%v package %v invalid type def %v: %v",
					file.BaseName, def.Pos, pkg.Name, def.Name, err.Error())
				continue
			}
		}
	}
	for _, file := range pkg.Files {
		for _, def := range file.TypeDefs {
			if err := def.define(env.pkgs); err != nil {
				env.Errors.Errorf("%v:%v package %v invalid type def %v: %v",
					file.BaseName, def.Pos, pkg.Name, def.Name, err.Error())
				continue
			}
		}
	}
	if !env.Errors.IsEmpty() {
		return
	}
	// Sort the typedefs by their intra-file dependencies.  This isn't necessary
	// for generated Go, but is for C++.
	for _, file := range pkg.Files {
		sorter := toposort.NewSorter()
		for _, def := range file.TypeDefs {
			for dep, _ := range def.Base.namedDeps() {
				sorter.AddEdge(def, dep)
			}
		}
		// We don't care if the graph is cyclic, we just want the ordering of the
		// acyclic portions of the graph.
		sorted, _ := sorter.Sort()
		file.TypeDefs = nil
		for _, idef := range sorted {
			if def := idef.(*TypeDef); def.File == file {
				// The sorted defs may include type defs from other files, so we must
				// filter down to just the ones in this file.
				file.TypeDefs = append(file.TypeDefs, def)
			}
		}
	}
	// Check that the type dependencies don't form a cycle between files.  This
	// isn't necessary for generated Go, but is for C++.
	//
	// TODO(toddw): This is too strict; we should ignore type dependencies past a
	// pointer boundary, since those would work fine.
	fileSorter := toposort.NewSorter()
	for _, file := range pkg.Files {
		for _, def := range file.TypeDefs {
			for dep, _ := range def.Base.namedDeps() {
				if file != dep.File {
					// We only care about inter-file dependencies, not self dependencies.
					fileSorter.AddEdge(file, dep.File)
				}
			}
		}
	}
	// We only care whether the graph is cyclic.
	_, cycles := fileSorter.Sort()
	if len(cycles) > 0 {
		cycleStr := toposort.PrintCycles(cycles, printFileBaseName)
		env.Errors.Errorf("Package %v has cyclic dependencies between type definitions in different files.  This is allowed in regular Go, but disallowed in the IDL since it's not implementable in C++. %v", pkg.Name, cycleStr)
	}
}

func defineConsts(pkg *Package, env *Env) {
	// Declare all const definitions in this package, so that we'll be able to
	// resolve named consts.
	for _, file := range pkg.Files {
		for _, def := range file.ConstDefs {
			if err := def.declare(); err != nil {
				env.Errors.Errorf("%v:%v package %v invalid const def %v: %v",
					file.BaseName, def.Pos, pkg.Name, def.Name, err.Error())
			}
		}
	}
	// Resolve all const definitions in this package.  Types used in type
	// conversions are resolved, and named consts are resolved.
	sorter := toposort.NewSorter()
	for _, file := range pkg.Files {
		for _, def := range file.ConstDefs {
			vlog.Printf("Resolving const %v", def)
			deps, err := def.resolve(env.pkgs)
			if err != nil {
				env.Errors.Errorf("%v:%v package %v invalid const def %v: %v",
					file.BaseName, def.Pos, pkg.Name, def.Name, err.Error())
				continue
			}
			sorter.AddNode(def)
			for dep, _ := range deps {
				sorter.AddEdge(def, dep)
			}
		}
	}
	// Sort const definitions by their dependencies on other named consts.  We
	// don't allow cycles.  The dependency analysis is performed on const defs,
	// not the files they occur in; const defs in the same package may be defined
	// in any file, even if they cause cyclic file dependencies.
	sorted, cycles := sorter.Sort()
	if len(cycles) > 0 {
		cycleStr := toposort.PrintCycles(cycles, printConstDefName)
		env.Errors.Errorf("Package %v has cyclic dependencies between const definitions: %v", pkg.Name, cycleStr)
	}
	// Define const definitions in topological order.  The ordering is necessary
	// since each const is fully evaluated to a final result.
	for _, idef := range sorted {
		if def := idef.(*ConstDef); def.File.Package == pkg {
			vlog.Printf("Defining const %v", def)
			if err := def.define(); err != nil {
				env.Errors.Errorf("%v:%v package %v invalid const def %v: %v",
					def.File.BaseName, def.Pos, pkg.Name, def.Name, err.Error())
			}
		}
	}
}

func defineTags(pkg *Package, env *Env) {
	// Resolve and evaluate all method tags
	for _, file := range pkg.Files {
		for _, iface := range file.Interfaces {
			for _, method := range iface.Methods() {
				for _, expr := range method.tagExprs {
					errmsg := fmt.Sprintf("%v:%v package %v interface %v method %v",
						file.BaseName, expr.pos(), pkg.Name, iface.Name, method.Name)
					vlog.Printf("Resolving tag %v", expr)
					if _, err := expr.resolve(env.pkgs, file); err != nil {
						env.Errors.Errorf("%s: %s", errmsg, err.Error())
						continue
					}
					vlog.Printf("Evaluating tag %v", expr)
					c, err := expr.eval()
					if err != nil {
						env.Errors.Errorf("%s: %s", errmsg, err.Error())
						continue
					}
					if err := c.FinalCheck(); err != nil {
						env.Errors.Errorf("%s: %s", errmsg, err.Error())
						continue
					}
					method.Tags = append(method.Tags, c)
				}
			}
		}
	}
}

// Resolves all arguments in interface methods.
func resolveArgTypes(pkg *Package, env *Env) {
	for _, file := range pkg.Files {
		for _, iface := range file.Interfaces {
			for _, c := range iface.Components {
				switch c := c.(type) {
				case *Method:
					for _, iarg := range c.InArgs {
						if err := iarg.Type.resolve(env.pkgs, file); err != nil {
							env.Errors.Errorf("%v:%v package %v interface %v method %v: %v",
								file.BaseName, iarg.Pos, pkg.Name, iface.Name, c.Name, err.Error())
							continue
						}
					}
					for _, oarg := range c.OutArgs {
						if err := oarg.Type.resolve(env.pkgs, file); err != nil {
							env.Errors.Errorf("%v:%v package %v interface %v method %v: %v",
								file.BaseName, oarg.Pos, pkg.Name, iface.Name, c.Name, err.Error())
							continue
						}
					}
					if istr := c.InStream; istr != nil {
						if istr.Type.Name() == "_" {
							c.InStream = nil
						} else if err := istr.Type.resolve(env.pkgs, file); err != nil {
							env.Errors.Errorf("%v:%v package %v interface %v method %v: %v",
								file.BaseName, istr.Pos, pkg.Name, iface.Name, c.Name, err.Error())
							continue
						}
					}
					if ostr := c.OutStream; ostr != nil {
						if ostr.Type.Name() == "_" {
							c.OutStream = nil
						} else if err := ostr.Type.resolve(env.pkgs, file); err != nil {
							env.Errors.Errorf("%v:%v package %v interface %v method %v: %v",
								file.BaseName, ostr.Pos, pkg.Name, iface.Name, c.Name, err.Error())
							continue
						}
					}
				case *EmbeddedInterface:
					if err := c.Type.resolve(env.pkgs, file); err != nil {
						env.Errors.Errorf("%v:%v package %v interface %v embedded interface %v: %v",
							file.BaseName, c.Pos, pkg.Name, iface.Name, c.Type.Name(), err.Error())
						continue
					}
				}
			}
		}
	}
}

func computePerFilePackageDeps(pkg *Package, env *Env) {
	// Check for unused user-supplied imports.
	for _, file := range pkg.Files {
		for _, imp := range file.Imports {
			if !imp.used {
				env.Errors.Errorf("%v:%v package %v unused import %q",
					file.BaseName, imp.Pos, pkg.Name, imp.Path)
			}
		}
	}
	// Compute per-file package deps based on the types that are actually used.
	// Note that named const dependencies have been flattened since we've
	// evaluated the const expressions, but type dependencies on the final const
	// types remain.
	for _, file := range pkg.Files {
		file.PackageDeps = make(map[*Package]bool)
		// Typedefs contribute the packages of their base named deps.
		for _, tdef := range file.TypeDefs {
			for dep, _ := range tdef.Base.namedDeps() {
				file.PackageDeps[dep.File.Package] = true
			}
		}
		// Untyped consts don't contribute any packages.  Typed consts contribute
		// the packages of its type's named deps.
		for _, cdef := range file.ConstDefs {
			if cdef.Const.TypeDef != nil {
				for dep, _ := range cdef.Const.TypeDef.namedDeps() {
					file.PackageDeps[dep.File.Package] = true
				}
			}
		}
		// Interfaces contribute the packages of the arg type named deps, and the
		// tag type named deps.
		for _, iface := range file.Interfaces {
			for _, c := range iface.Components {
				switch c := c.(type) {
				case *Method:
					for _, iarg := range c.InArgs {
						for dep, _ := range iarg.Type.namedDeps() {
							file.PackageDeps[dep.File.Package] = true
						}
					}
					for _, oarg := range c.OutArgs {
						for dep, _ := range oarg.Type.namedDeps() {
							file.PackageDeps[dep.File.Package] = true
						}
					}
					for _, tag := range c.Tags {
						if tag.TypeDef != nil {
							for dep, _ := range tag.TypeDef.namedDeps() {
								file.PackageDeps[dep.File.Package] = true
							}
						}
					}
				case *EmbeddedInterface:
					for dep, _ := range c.Type.namedDeps() {
						file.PackageDeps[dep.File.Package] = true
					}
				}
			}
		}
		// Now remove self and global dependencies.  Every package can use itself
		// and the global package so we don't need to record this.
		delete(file.PackageDeps, pkg)
		delete(file.PackageDeps, globalPackage)
	}
}

func printFileBaseName(v interface{}) string {
	return v.(*File).BaseName
}

func printConstDefName(v interface{}) string {
	return v.(*ConstDef).Name
}
