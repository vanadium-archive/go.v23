package build

import (
	gobuild "go/build"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	"veyron/lib/toposort"
)

// OpenIDLFilesFunc represents a function that will open all the idl files in
// a BuildPackage.
type OpenIDLFilesFunc func(p *BuildPackage) (map[string]io.ReadCloser, error)

// BuildPackage represents the build information for an idl package.  The idl
// cmdline tool is the main initial user.
type BuildPackage struct {
	// Dir is the absolute directory containing the package files.
	// E.g. "/home/user/veyron/v/src/veyron/rt/base"
	Dir string
	// Name is the name of the package, specified in the idl files.
	// E.g. "base"
	Name string
	// Path is the package path, e.g. "veyron/idl/lib".  It may be empty if the
	// path isn't known - e.g. if we're building a directory.
	// E.g. "veyron/rt/base"
	Path string
	// IDLBaseFileNames is an unordered list of base idl file names for this
	// package.  Join these with Dir to get absolute file names.
	IDLBaseFileNames []string

	// Open lets us open files either on disk or in memory.
	Open OpenIDLFilesFunc
}

type missingMode bool

const (
	missingIsOk    missingMode = true
	missingIsError missingMode = false
)

// New packages always start with an empty Name and Path.  The Name is filled in
// when we walkDeps, and the Path is only filled in if we process this package
// via resolvePkgPath.
func newBuildPackage(dir string, mode missingMode, env *Env) *BuildPackage {
	pkg := &BuildPackage{Dir: dir, Open: openIDLFiles}
	if err := pkg.initIDLBaseFileNames(); err != nil {
		env.Errors.Errorf("Couldn't init idl file names in package dir %v, %v",
			pkg.Dir, err)
		return nil
	}
	if len(pkg.IDLBaseFileNames) == 0 {
		if mode == missingIsError {
			env.Errors.Errorf("No idl files in dir %v", pkg.Dir)
		}
		return nil
	}
	return pkg
}

// initIDLBaseFileNames initializes IDLBaseFileNames from the Dir.
func (p *BuildPackage) initIDLBaseFileNames() error {
	vlog.Printf("Looking for IDL files in package dir %v", p.Dir)
	fd, err := os.Open(p.Dir)
	if err != nil {
		return err
	}
	dirFileNames, err := fd.Readdirnames(-1)
	if err != nil {
		return err
	}
	for _, baseFile := range dirFileNames {
		if filepath.Ext(baseFile) == ".idl" {
			p.IDLBaseFileNames = append(p.IDLBaseFileNames, baseFile)
		}
	}
	return nil
}

// OpenIDLFiles opens all idl files in the package and returns a map from base
// file name to file contents.
func (p *BuildPackage) OpenIDLFiles() (files map[string]io.ReadCloser, err error) {
	return p.Open(p)
}

func openIDLFiles(p *BuildPackage) (files map[string]io.ReadCloser, err error) {
	files = make(map[string]io.ReadCloser)
	for _, baseName := range p.IDLBaseFileNames {
		file, err := os.Open(filepath.Join(p.Dir, baseName))
		if err != nil {
			CloseIDLFiles(files)
			return nil, err
		}
		files[baseName] = file
	}
	return files, nil
}

// CloseIDLFiles closes the files returned by OpenIDLFiles.  Returns nil if all
// files were closed successfully, otherwise returns one of the errors, dropping
// the others.  Regardless of whether an error is returned, Close will be called
// on all files.
func CloseIDLFiles(files map[string]io.ReadCloser) error {
	var err error
	for _, file := range files {
		if err2 := file.Close(); err == nil {
			err = err2
		}
	}
	return err
}

// depSorter does the main work of collecting and sorting packages and their
// dependencies.  We support most of the syntax from the go cmdline tool; both
// dirs and package paths are supported, and we allow special cases for the
// "all" package and "..." wildcards.
//
// This is slightly complicated because of dirs, and the potential for symlinks.
// E.g. let's say we have two directories, one a symlink to the other:
//   /home/user/veyron/v/src/veyron/rt/base
//   /home/user/veyron/v/src/veyron/rt2     symlink to rt
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
// might have the same logical package listed under different BuildPackages with
// different absolute dirnames, but that's fine; we'll just generate some
// packages multiple times.
//
// TODO(toddw): If we care about performance we could serialize the compiled
// idl.Package information and write it out as compiler-generated artifacts,
// similar to how the regular go tool generates *.a files under the top-level
// pkg directory.
type depSorter struct {
	srcDirs []string
	pathMap map[string]*BuildPackage
	dirMap  map[string]*BuildPackage
	sorter  toposort.Sorter
	env     *Env
}

func toAbs(dirs []string, errors Errors) (ret []string) {
	for _, d := range dirs {
		if abs, err := filepath.Abs(d); err != nil {
			errors.Errorf("Couldn't make dir %q absolute: %v", d, err)
		} else {
			ret = append(ret, abs)
		}
	}
	return
}

func newDepSorter(env *Env) *depSorter {
	return &depSorter{
		srcDirs: toAbs(gobuild.Default.SrcDirs(), env.Errors),
		pathMap: make(map[string]*BuildPackage),
		dirMap:  make(map[string]*BuildPackage),
		sorter:  toposort.NewSorter(),
		env:     env,
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
	ds.env.Errors.Errorf(format, v...)
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

// resolvePkgDir resolves the pkgDir into a BuildPackage.  Returns the package
// or nil if it couldn't be resolved, along with a bool telling us whether this
// is the first time we've seen the package.  The missingMode controls whether
// it's ok for the underlying directory to be missing.
func (ds *depSorter) resolvePkgDir(pkgDir string, mode missingMode) (*BuildPackage, bool) {
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
	newPkg := newBuildPackage(absDir, mode, ds.env)
	if newPkg == nil {
		return nil, false
	}
	ds.dirMap[newPkg.Dir] = newPkg
	return newPkg, true
}

// resolvePkgPath resolves the pkgPath into a BuildPackage.  Returns the package
// or nil if it couldn't be resolved, along with a bool telling us whether this
// is the first time we've seen the package.  We don't support relative package
// paths.
func (ds *depSorter) resolvePkgPath(pkgPath string) (*BuildPackage, bool) {
	pkgPath = path.Clean(pkgPath)
	if existPkg, exists := ds.pathMap[pkgPath]; exists {
		return existPkg, false
	}

	// Look through srcDirs in-order until we find a valid package dir.
	for _, srcDir := range ds.srcDirs {
		candidateDir := filepath.Join(srcDir, pkgPath)
		if pkg, isNew := ds.resolvePkgDir(candidateDir, missingIsOk); pkg != nil {
			vlog.Printf("Resolved pkg path %v to abs dir %v", pkgPath, pkg.Dir)
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
func (ds *depSorter) walkDeps(pkg *BuildPackage) {
	ds.sorter.AddNode(pkg)
	idlFiles, err := pkg.OpenIDLFiles()
	defer CloseIDLFiles(idlFiles)
	if err != nil {
		ds.errorf("Couldn't open idl files %v, %v", pkg.IDLBaseFileNames, err)
		return
	}
	// Set the pkg Name while we're at it.
	var depPkgPaths []string
	pkg.Name, depPkgPaths = ParsePackageImports(idlFiles, ds.env)
	for _, depPath := range depPkgPaths {
		if depPkg, isNew := ds.resolvePkgPath(depPath); depPkg != nil {
			ds.sorter.AddEdge(pkg, depPkg)
			if isNew {
				ds.walkDeps(depPkg)
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

// Sort sorts all targets and returns the resulting list of BuildPackages.
func (ds *depSorter) Sort() (targets []*BuildPackage) {
	// The topoSort does all the work for us - we just need to unpack the
	// results from interface{} back into *BuildPackage.
	sorted, cycles := ds.sorter.Sort()
	if len(cycles) > 0 {
		cycleStr := toposort.PrintCycles(cycles, printBuildPackagePath)
		ds.errorf("Cyclic package dependency detected: %v", cycleStr)
		return
	}
	targets = make([]*BuildPackage, len(sorted))
	for ix, iface := range sorted {
		targets[ix] = iface.(*BuildPackage)
	}
	return
}

func printBuildPackagePath(v interface{}) string {
	return v.(*BuildPackage).Path
}

// GetTransitiveTargets takes a list of cmdline args representing packages, and
// returns all targets (either explicitly added or a transitive dependency) in
// transitive order.
func GetTransitiveTargets(cmdLineArgs []string, env *Env) []*BuildPackage {
	ds := newDepSorter(env)
	for _, cmdLineArg := range cmdLineArgs {
		ds.AddCmdLineArg(cmdLineArg)
	}
	ds.DeduceUnknownPackagePaths()
	return ds.Sort()
}
