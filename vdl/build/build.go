// Package build provides utilities to collect vdl build information, and
// helpers to kick off the parser and compiler.
package build

import (
	gobuild "go/build"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	"veyron/lib/toposort"
	"veyron2/vdl"
	"veyron2/vdl/compile"
	"veyron2/vdl/parse"
	"veyron2/vdl/vdlutil"
)

// Package represents the build information for an vdl package.
type Package struct {
	// Dir is the absolute directory containing the package files.
	// E.g. "/home/user/veyron/go/src/veyron/rt/base"
	Dir string
	// Name is the name of the package, specified in the vdl files.
	// E.g. "base"
	Name string
	// Path is the package path, e.g. "veyron/vdl/lib".  It may be empty if the
	// path isn't known - e.g. if we're building a directory.
	// E.g. "veyron/rt/base"
	Path string
	// BaseFileNames is an unordered list of base vdl file names for this
	// package.  Join these with Dir to get absolute file names.
	BaseFileNames []string

	// OpenFilesFunc is a function that opens the files with the given filenames,
	// and returns a map from base file name to file contents.
	OpenFilesFunc func(filenames []string) (map[string]io.ReadCloser, error)

	openedFiles []io.Closer // files that need to be closed
}

type missingMode bool

const (
	missingIsOk    missingMode = true
	missingIsError missingMode = false
)

// New packages always start with an empty Name and Path.  The Name is filled in
// when we walkDeps, and the Path is only filled in if we process this package
// via resolvePkgPath.
func newPackage(dir string, mode missingMode, exts map[string]bool, errs *vdlutil.Errors) *Package {
	pkg := &Package{Dir: dir, OpenFilesFunc: openFiles}
	if err := pkg.initBaseFileNames(exts); err != nil {
		errs.Errorf("Couldn't init vdl file names in package dir %v, %v", pkg.Dir, err)
		return nil
	}
	if len(pkg.BaseFileNames) == 0 {
		if mode == missingIsError {
			errs.Errorf("No vdl files in dir %v", pkg.Dir)
		}
		return nil
	}
	return pkg
}

// initBaseFileNames initializes BaseFileNames from the Dir.
func (p *Package) initBaseFileNames(exts map[string]bool) error {
	vdlutil.Vlog.Printf("Looking for vdl files in package dir %v", p.Dir)
	fd, err := os.Open(p.Dir)
	if err != nil {
		return err
	}
	dirFileNames, err := fd.Readdirnames(-1)
	if err != nil {
		return err
	}
	for _, baseFile := range dirFileNames {
		if exts[filepath.Ext(baseFile)] {
			p.BaseFileNames = append(p.BaseFileNames, baseFile)
		}
	}
	return nil
}

// OpenFiles opens all files in the package and returns a map from base file
// name to file contents.  CloseFiles must be called to close the files.
func (p *Package) OpenFiles() (map[string]io.Reader, error) {
	var filenames []string
	for _, baseName := range p.BaseFileNames {
		filenames = append(filenames, filepath.Join(p.Dir, baseName))
	}
	files, err := p.OpenFilesFunc(filenames)
	if err != nil {
		for _, c := range files {
			c.Close()
		}
		return nil, err
	}
	// Convert map elem type from io.ReadCloser to io.Reader.
	res := make(map[string]io.Reader, len(files))
	for n, f := range files {
		res[n] = f
		p.openedFiles = append(p.openedFiles, f)
	}
	return res, nil
}

func openFiles(filenames []string) (map[string]io.ReadCloser, error) {
	files := make(map[string]io.ReadCloser, len(filenames))
	for _, filename := range filenames {
		file, err := os.Open(filename)
		if err != nil {
			for _, c := range files {
				c.Close()
			}
			return nil, err
		}
		files[path.Base(filename)] = file
	}
	return files, nil
}

// CloseFiles closes all files returned by OpenFiles.  Returns nil if all files
// were closed successfully, otherwise returns one of the errors, dropping the
// others.  Regardless of whether an error is returned, Close will be called on
// all files.
func (p *Package) CloseFiles() error {
	var err error
	for _, c := range p.openedFiles {
		if err2 := c.Close(); err == nil {
			err = err2
		}
	}
	p.openedFiles = nil
	return err
}

// depSorter does the main work of collecting and sorting packages and their
// dependencies.  We support most of the syntax from the go cmdline tool; both
// dirs and package paths are supported, and we allow special cases for the
// "all" package and "..." wildcards.
//
// This is slightly complicated because of dirs, and the potential for symlinks.
// E.g. let's say we have two directories, one a symlink to the other:
//   /home/user/veyron/go/src/veyron/rt/base
//   /home/user/veyron/go/src/veyron/rt2     symlink to rt
//
// The problem is that if the user has cwd pointing at one of the two "base"
// dirs and specifies a relative directory ".." it's ambiguous which absolute
// dir we'll end up with; file paths form a graph rather than a tree.  For more
// details see http://plan9.bell-labs.com/sys/doc/lexnames.html
//
// This means there's no way to deduce what the package path should be if a user
// builds a directory rather than a package path.  Luckily we don't always need
// to know the package path; the only time we must have a package path for a
// package is if the package is imported by another package (since the lookup is
// based on package path), but in that case we're performing a forward lookup of
// the package path, and will guarantee that the package path is filled in.
// Dirs that the user is building that aren't depended on by other packages
// won't have a package path, but we don't need one.
//
// Alas there's a caveat.  The error definition mechanism currently relies on
// package path to create implicit error ids, and must be known at the time the
// package is compiled.  To handle this we cheat in DeduceUnknownPackagePaths
// and attempt to deduce the package path even if the user builds a directory.
// This may sometimes be wrong - to ensure correctness the veyron rule is to
// disallow symlinks within the source tree.  TODO(toddw): Decide whether to
// keep this restriction, or generate implicit error ids via some other
// mechanism.
//
// The strategy is to compute the topological ordering based on absolute
// directory names, and fill in package paths if available.  This means that we
// might have the same logical package listed under different Packages with
// different absolute dirnames, but that's fine; we'll just generate some
// packages multiple times.
//
// TODO(toddw): If we care about performance we could serialize the compiled
// vdlutil.Package information and write it out as compiler-generated artifacts,
// similar to how the regular go tool generates *.a files under the top-level
// pkg directory.
type depSorter struct {
	exts    map[string]bool // file extensions of valid vdl files.
	srcDirs []string
	pathMap map[string]*Package
	dirMap  map[string]*Package
	sorter  toposort.Sorter
	errs    *vdlutil.Errors
}

func makeExts(exts []string) map[string]bool {
	ret := make(map[string]bool)
	for _, e := range exts {
		ret[e] = true
	}
	return ret
}

func toAbs(dirs []string, errs *vdlutil.Errors) (ret []string) {
	for _, d := range dirs {
		if abs, err := filepath.Abs(d); err != nil {
			errs.Errorf("Couldn't make dir %q absolute: %v", d, err)
		} else {
			ret = append(ret, abs)
		}
	}
	return
}

func newDepSorter(exts []string, errs *vdlutil.Errors) *depSorter {
	return &depSorter{
		exts:    makeExts(exts),
		srcDirs: toAbs(gobuild.Default.SrcDirs(), errs),
		pathMap: make(map[string]*Package),
		dirMap:  make(map[string]*Package),
		sorter:  toposort.NewSorter(),
		errs:    errs,
	}
}

// AddCmdLineArg resolves the cmdLineArg into a package and adds it to the
// sorter, along with all transitive dependencies.
func (ds *depSorter) AddCmdLineArg(cmdLineArg string) {
	// The "all" package is special-cased.
	if cmdLineArg == "all" {
		for _, srcDir := range ds.srcDirs {
			ds.addAllDirs(srcDir)
		}
	} else if filepath.IsAbs(cmdLineArg) || strings.Contains(cmdLineArg, ".") {
		// It's a package dir or pattern.
		if strings.HasSuffix(cmdLineArg, "...") {
			// TODO(toddw): Support ... syntax for partial dirnames.
			ds.addAllDirs(filepath.Clean(strings.TrimSuffix(cmdLineArg, "...")))
		} else {
			if pkg, isNew := ds.resolvePkgDir(cmdLineArg, missingIsError); isNew {
				ds.walkDeps(pkg)
			}
		}
	} else {
		// It's a package path.
		if pkg, isNew := ds.resolvePkgPath(cmdLineArg); isNew {
			ds.walkDeps(pkg)
		}
	}
}

func (ds *depSorter) errorf(format string, v ...interface{}) {
	ds.errs.Errorf(format, v...)
}

// addAllDirs adds all package dirs with the given prefix.
func (ds *depSorter) addAllDirs(prefix string) {
	// Try looking in the prefix itself.
	if pkg, isNew := ds.resolvePkgDir(prefix, missingIsOk); isNew {
		ds.walkDeps(pkg)
	}
	// Now try looking for all dirs under the prefix.
	fd, err := os.Open(prefix)
	if err != nil {
		return // Silently skip this src dir.
	}
	fileInfos, err := fd.Readdir(-1)
	if err != nil {
		return // Silently skip this src dir.
	}
	// TODO(toddw): Should we break infinite loops from symlinks / hardlinks?
	for _, fi := range fileInfos {
		if fi.IsDir() {
			ds.addAllDirs(filepath.Join(prefix, fi.Name()))
		}
	}
}

// resolvePkgDir resolves the pkgDir into a Package.  Returns the package
// or nil if it couldn't be resolved, along with a bool telling us whether this
// is the first time we've seen the package.  The missingMode controls whether
// it's ok for the underlying directory to be missing.
func (ds *depSorter) resolvePkgDir(pkgDir string, mode missingMode) (*Package, bool) {
	absDir, err := filepath.Abs(pkgDir)
	if err != nil {
		ds.errorf("Couldn't make package dir %v absolute, %v", pkgDir, err)
	}
	if existPkg, exists := ds.dirMap[absDir]; exists {
		return existPkg, false
	}
	fileInfo, err := os.Stat(absDir)
	if err != nil {
		if mode == missingIsError {
			ds.errorf(err.Error())
		}
		return nil, false
	}
	if !fileInfo.IsDir() {
		if mode == missingIsError {
			ds.errorf("Expected %v to be a directory", absDir)
		}
		return nil, false
	}
	newPkg := newPackage(absDir, mode, ds.exts, ds.errs)
	if newPkg == nil {
		return nil, false
	}
	ds.dirMap[newPkg.Dir] = newPkg
	return newPkg, true
}

// resolvePkgPath resolves the pkgPath into a Package.  Returns the package
// or nil if it couldn't be resolved, along with a bool telling us whether this
// is the first time we've seen the package.  We don't support relative package
// paths.
func (ds *depSorter) resolvePkgPath(pkgPath string) (*Package, bool) {
	pkgPath = path.Clean(pkgPath)
	if existPkg, exists := ds.pathMap[pkgPath]; exists {
		return existPkg, false
	}

	// Look through srcDirs in-order until we find a valid package dir.
	for _, srcDir := range ds.srcDirs {
		candidateDir := filepath.Join(srcDir, pkgPath)
		if pkg, isNew := ds.resolvePkgDir(candidateDir, missingIsOk); pkg != nil {
			vdlutil.Vlog.Printf("Resolved pkg path %v to abs dir %v", pkgPath, pkg.Dir)
			// Update the pkg path since it might not have been known before.
			//
			// TODO(toddw): Should we handle hardlinks / symlinks?
			pkg.Path = pkgPath
			ds.pathMap[pkg.Path] = pkg
			return pkg, isNew
		}
	}

	// We couldn't find a valid package dir corresponding to this path.
	ds.errorf("Couldn't resolve package path %v", pkgPath)
	return nil, false
}

// walkDeps adds the pkg and its dependencies to the sorter.  This just does
// DFS, so technically we could determine the transitive order without using the
// sorter, but it's nice to use the sorter since it detects cycles for us;
// otherwise we'd need to keep a separate set.
func (ds *depSorter) walkDeps(pkg *Package) {
	ds.sorter.AddNode(pkg)
	pfiles := ParsePackage(pkg, parse.Opts{ImportsOnly: true}, ds.errs)
	pkg.Name = parse.InferPackageName(pfiles, ds.errs)
	for _, pf := range pfiles {
		for _, imp := range pf.Imports {
			if depPkg, isNew := ds.resolvePkgPath(imp.Path); depPkg != nil {
				ds.sorter.AddEdge(pkg, depPkg)
				if isNew {
					ds.walkDeps(depPkg)
				}
			}
		}
	}
}

// DeduceUnknownPackagePaths attempts to deduce unknown package paths, by
// looking for prefix matches against the src dirs.  The resulting package path
// may be incorrect even if no errors are reported; see the main depSorter
// comment for details.
func (ds *depSorter) DeduceUnknownPackagePaths() {
	for _, pkg := range ds.dirMap {
		if len(pkg.Path) > 0 {
			continue
		}
		for _, srcDir := range ds.srcDirs {
			if strings.HasPrefix(pkg.Dir, srcDir) {
				relPath, err := filepath.Rel(srcDir, pkg.Dir)
				if err != nil {
					ds.errorf("Couldn't compute relative path src=%s dir=%s", srcDir, pkg.Dir)
					continue
				}
				pkg.Path = path.Clean(filepath.ToSlash(relPath))
				break
			}
		}
		if len(pkg.Path) == 0 {
			ds.errorf("Couldn't deduce package path for %s", pkg.Dir)
		}
	}
}

// Sort sorts all targets and returns the resulting list of Packages.
func (ds *depSorter) Sort() (targets []*Package) {
	// The topoSort does all the work for us - we just need to unpack the
	// results from interface{} back into *Package.
	sorted, cycles := ds.sorter.Sort()
	if len(cycles) > 0 {
		cycleStr := toposort.PrintCycles(cycles, printPackagePath)
		ds.errorf("Cyclic package dependency detected: %v", cycleStr)
		return
	}
	targets = make([]*Package, len(sorted))
	for ix, iface := range sorted {
		targets[ix] = iface.(*Package)
	}
	return
}

func printPackagePath(v interface{}) string {
	return v.(*Package).Path
}

// TransitivePackages takes a list of cmdline args representing packages, and
// returns all packages (either explicitly added or a transitive dependency) in
// transitive order.  The given exts specifies the file name extensions for
// valid vdl files, e.g. ".vdl".
func TransitivePackages(cmdLineArgs, exts []string, errs *vdlutil.Errors) []*Package {
	ds := newDepSorter(exts, errs)
	for _, cmdLineArg := range cmdLineArgs {
		ds.AddCmdLineArg(cmdLineArg)
	}
	ds.DeduceUnknownPackagePaths()
	return ds.Sort()
}

// ParsePackage parses the given pkg with the given parse opts, and returns a
// slice of parsed files.  Errors are reported in errs.
func ParsePackage(pkg *Package, opts parse.Opts, errs *vdlutil.Errors) (pfiles []*parse.File) {
	vdlutil.Vlog.Printf("Parsing package %s %q, dir %s", pkg.Name, pkg.Path, pkg.Dir)
	files, err := pkg.OpenFiles()
	if err != nil {
		errs.Errorf("Couldn't open vdl files %v, %v", pkg.BaseFileNames, err)
		return nil
	}
	for filename, src := range files {
		if pf := parse.Parse(filename, src, opts, errs); pf != nil {
			pfiles = append(pfiles, pf)
		}
	}
	pkg.CloseFiles()
	return
}

// CompilePackage parses and compiles the given pkg, updates env with the
// compiled package and returns it.  Errors are reported in env.
//
// All imports that pkg depend on must already have been compiled and populated
// into env.  See TransitivePackages for an easy way to retrieve packages in
// their transitive order.
func CompilePackage(pkg *Package, env *compile.Env) *compile.Package {
	pfiles := ParsePackage(pkg, parse.Opts{}, env.Errors)
	return compile.Compile(pkg.Path, pfiles, env)
}

// CompileConfig parses and compiles the given config src and returns it.
// Errors are reported in env; baseFileName is only used for error reporting.
// If implicit is non-nil and the exported config const is an untyped const
// literal, it is assumed to be of that type.
//
// All imports that the config src depend on must already have been compiled and
// populated into env.  See TransitivePackages for an easy way to retrieve
// packages in their transitive order.
func CompileConfig(baseFileName string, src io.Reader, implicit *vdl.Type, env *compile.Env) *vdl.Value {
	pconfig := parse.ParseConfig(baseFileName, src, parse.Opts{}, env.Errors)
	return compile.CompileConfig(implicit, pconfig, env)
}
