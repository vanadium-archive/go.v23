// Package build provides utilities to collect VDL build information, and
// helpers to kick off the parser and compiler.
//
// VDL Packages
//
// VDL is organized into packages, where a package is a collection of one or
// more source files.  The files in a package collectively define the types,
// constants, services and errors belonging to the package; these are called
// package elements.
//
// The package elements in package P may be used in another package Q.  First
// package Q must import package P, and then refer to the package elements in P.
// Imports define the package dependency graph, which must be acyclic.
//
// Build Strategy
//
// The steps to building a VDL package P:
//   1) Compute the transitive closure of P's dependencies DEPS.
//   2) Sort DEPS in dependency order.
//   3) Build each package D in DEPS.
//   3) Build package P.
//
// Building a package P requires that all elements used by P are understood,
// including elements defined outside of P.  The only way for a change to
// package Q to affect the build of P is if Q is in the transitive closure of
// P's package dependencies.  However there may be false positives; the change
// to Q might not actually affect P.
//
// The build process may perform more work than is strictly necessary, because
// of these false positives.  However it is simple and correct.
//
// The TransitivePackages* functions implement build steps 1 and 2.
//
// The Build* functions implement build steps 3 and 4.
//
// Other functions provide related build information and utilities.
package build

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"veyron.io/veyron/veyron/lib/toposort"
	"veyron.io/veyron/veyron2/vdl"
	"veyron.io/veyron/veyron2/vdl/compile"
	"veyron.io/veyron/veyron2/vdl/parse"
	"veyron.io/veyron/veyron2/vdl/vdlutil"
)

// Package represents the build information for a vdl package.
type Package struct {
	// Name is the name of the package, specified in the vdl files.
	// E.g. "bar"
	Name string
	// Path is the package path,
	// E.g. "foo/bar".
	Path string
	// Dir is the absolute directory containing the package files.
	// E.g. "/home/user/veyron/vdl/src/foo/bar"
	Dir string
	// BaseFileNames is the list of sorted base vdl file names for this package.
	// Join these with Dir to get absolute file names.
	BaseFileNames []string

	// OpenFilesFunc is a function that opens the files with the given filenames,
	// and returns a map from base file name to file contents.
	OpenFilesFunc func(filenames []string) (map[string]io.ReadCloser, error)

	openedFiles []io.Closer // files that need to be closed
}

// UnknownPathMode specifies the behavior when an unknown path is encountered.
type UnknownPathMode int

const (
	UnknownPathIsIgnored UnknownPathMode = iota // Silently ignore unknown paths
	UnknownPathIsError                          // Produce error for unknown paths
)

func (m UnknownPathMode) String() string {
	switch m {
	case UnknownPathIsIgnored:
		return "UnknownPathIsIgnored"
	case UnknownPathIsError:
		return "UnknownPathIsError"
	default:
		return fmt.Sprintf("UnknownPathMode(%d)", m)
	}
}

func (m UnknownPathMode) logOrErrorf(errs *vdlutil.Errors, format string, v ...interface{}) {
	if m == UnknownPathIsIgnored {
		vdlutil.Vlog.Printf(format, v...)
	} else {
		errs.Errorf(format, v...)
	}
}

func pathPrefixDotOrDotDot(path string) bool {
	// The point of this helper is to catch cases where the path starts with a
	// . or .. element; note that  ... returns false.
	spath := filepath.ToSlash(path)
	return path == "." || path == ".." || strings.HasPrefix(spath, "./") || strings.HasPrefix(spath, "../")
}

func ignorePathElem(elem string) bool {
	return (strings.HasPrefix(elem, ".") || strings.HasPrefix(elem, "_")) &&
		!pathPrefixDotOrDotDot(elem)
}

// validPackagePath returns true iff the path is valid; i.e. if none of the path
// elems is ignored.
func validPackagePath(path string) bool {
	for _, elem := range strings.Split(path, "/") {
		if ignorePathElem(elem) {
			return false
		}
	}
	return true
}

// New packages always start with an empty Name, which is filled in when we call
// ds.addPackageAndDeps.
func newPackage(path, dir string, mode UnknownPathMode, exts map[string]bool, errs *vdlutil.Errors) *Package {
	pkg := &Package{Path: path, Dir: dir, OpenFilesFunc: openFiles}
	if err := pkg.initBaseFileNames(exts); err != nil {
		mode.logOrErrorf(errs, "%s: bad package dir (%v)", pkg.Dir, err)
		return nil
	}
	return pkg
}

// initBaseFileNames initializes BaseFileNames from the Dir.
func (p *Package) initBaseFileNames(exts map[string]bool) error {
	infos, err := ioutil.ReadDir(p.Dir)
	if err != nil {
		return err
	}
	for _, info := range infos {
		if info.IsDir() {
			continue
		}
		if ignorePathElem(info.Name()) || !exts[filepath.Ext(info.Name())] {
			vdlutil.Vlog.Printf("%s: ignoring file", filepath.Join(p.Dir, info.Name()))
			continue
		}
		vdlutil.Vlog.Printf("%s: adding vdl file", filepath.Join(p.Dir, info.Name()))
		p.BaseFileNames = append(p.BaseFileNames, info.Name())
	}
	if len(p.BaseFileNames) == 0 {
		return fmt.Errorf("no vdl files")
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

// SrcDirs returns a list of package root source directories, based on the
// VDLPATH environment variable.
//
// VDLPATH is a list of directories separated by filepath.ListSeparator;
// e.g. the separator is ":" on UNIX, and ";" on Windows.  Each VDLPATH
// directory must have a "src/" directory that holds vdl source code.  The path
// below "src/" determines the import path.
func SrcDirs() []string {
	var ret []string
	for _, dir := range filepath.SplitList(os.Getenv("VDLPATH")) {
		if dir != "" {
			if abs, err := filepath.Abs(filepath.Join(dir, "src")); err == nil {
				ret = append(ret, abs)
			}
		}
	}
	return ret
}

// IsDirPath returns true iff the path is absolute, or begins with a . or
// .. element.  The path denotes the package in that directory.
func IsDirPath(path string) bool {
	return filepath.IsAbs(path) || pathPrefixDotOrDotDot(path)
}

// IsImportPath returns true iff !IsDirPath.  The path P denotes the package in
// directory DIR/src/P, for some DIR listed in SrcDirs.
func IsImportPath(path string) bool {
	return !IsDirPath(path)
}

// depSorter does the main work of collecting and sorting packages and their
// dependencies.  The full syntax from the go cmdline tool is supported; we
// allow both dirs and import paths, as well as the "all" and "..." wildcards.
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
// This means that if the user builds a dir (rather than an import path), we
// might not be able to deduce the package path.  Note that the error definition
// mechanism relies on the package path to create implicit error ids, and this
// must be known at the time the package is compiled.  To handle this we call
// deducePackagePath and attempt to deduce the package path even if the user
// builds a directory, and return errors if this fails.
//
// TODO(toddw): If we care about performance we could serialize the compiled
// compile.Package information and write it out as compiler-generated artifacts,
// similar to how the regular go tool generates *.a files under the top-level
// pkg directory.
type depSorter struct {
	exts    map[string]bool // file extensions of valid vdl files.
	srcDirs []string
	pathMap map[string]*Package
	dirMap  map[string]*Package
	sorter  *toposort.Sorter
	errs    *vdlutil.Errors
}

func makeExts(exts []string) map[string]bool {
	ret := make(map[string]bool)
	for _, e := range exts {
		ret[e] = true
	}
	return ret
}

func newDepSorter(exts []string, errs *vdlutil.Errors) *depSorter {
	srcDirs := SrcDirs()
	if len(srcDirs) == 0 {
		errs.Error("No src dirs; set VDLPATH to a valid value")
	}
	return &depSorter{
		exts:    makeExts(exts),
		srcDirs: srcDirs,
		pathMap: make(map[string]*Package),
		dirMap:  make(map[string]*Package),
		sorter:  &toposort.Sorter{},
		errs:    errs,
	}
}

func (ds *depSorter) errorf(format string, v ...interface{}) {
	ds.errs.Errorf(format, v...)
}

// ResolvePath resolves path into package(s) and adds them to the sorter.
// Returns true iff path could be resolved.
func (ds *depSorter) ResolvePath(path string, mode UnknownPathMode) bool {
	if path == "all" {
		// Special-case "all", with the same behavior as Go.
		path = "..."
	}
	isDirPath := IsDirPath(path)
	dots := strings.Index(path, "...")
	switch {
	case dots >= 0:
		return ds.resolveWildcardPath(isDirPath, path[:dots], path[dots:])
	case isDirPath:
		return ds.resolveDirPath(path, "", mode) != nil
	default:
		return ds.resolveImportPath(path, mode) != nil
	}
}

// resolveWildcardPath resolves wildcards for both dir and import paths.  The
// prefix is everything before the first "...", and the suffix is everything
// including and after the first "..."; note that multiple "..." wildcards may
// occur within the suffix.  Returns true iff any packages were resolved.
//
// The strategy is to compute one or more root directories that contain
// everything that could possibly be matched, along with a filename pattern to
// match against.  Then we walk through each root directory, matching against
// the pattern.
func (ds *depSorter) resolveWildcardPath(isDirPath bool, prefix, suffix string) bool {
	var rootDirs []string // root directories to walk through
	var pattern string    // pattern to match against, starting after root dir
	switch {
	case isDirPath:
		// prefix and suffix are directory paths.
		dir, pre := filepath.Split(prefix)
		pattern = filepath.Clean(pre + suffix)
		rootDirs = append(rootDirs, filepath.Clean(dir))
	default:
		// prefix and suffix are slash-separated import paths.
		slashDir, pre := path.Split(prefix)
		pattern = filepath.Clean(pre + filepath.FromSlash(suffix))
		dir := filepath.FromSlash(slashDir)
		for _, srcDir := range ds.srcDirs {
			rootDirs = append(rootDirs, filepath.Join(srcDir, dir))
		}
	}
	matcher, err := createMatcher(pattern)
	if err != nil {
		ds.errorf("%v", err)
		return false
	}
	// Walk through root dirs and subdirs, looking for matches.
	resolvedAny := false
	for _, root := range rootDirs {
		filepath.Walk(root, func(rootAndPath string, info os.FileInfo, err error) error {
			// Ignore errors and non-directory elements.
			if err != nil || !info.IsDir() {
				return nil
			}
			// Skip the dir and subdirs if the elem should be ignored.
			_, elem := filepath.Split(rootAndPath)
			if ignorePathElem(elem) {
				vdlutil.Vlog.Printf("%s: ignoring dir", rootAndPath)
				return filepath.SkipDir
			}
			// Ignore the dir if it doesn't match our pattern.  We still process the
			// subdirs since they still might match.
			//
			// TODO(toddw): We could add an optimization to skip subdirs that can't
			// possibly match the matcher.  E.g. given pattern "a..." we can skip
			// the subdirs if the dir doesn't start with "a".
			matchPath := rootAndPath[len(root):]
			if strings.HasPrefix(matchPath, pathSeparator) {
				matchPath = matchPath[len(pathSeparator):]
			}
			if !matcher.MatchString(matchPath) {
				return nil
			}
			// Finally resolve the dir.
			if ds.resolveDirPath(rootAndPath, "", UnknownPathIsIgnored) != nil {
				resolvedAny = true
			}
			return nil
		})
	}
	return resolvedAny
}

const pathSeparator = string(filepath.Separator)

// createMatcher creates a regexp matcher out of the file pattern.
func createMatcher(pattern string) (*regexp.Regexp, error) {
	rePat := regexp.QuoteMeta(pattern)
	rePat = strings.Replace(rePat, `\.\.\.`, `.*`, -1)
	// Add special-case so that x/... also matches x.
	slashDotStar := regexp.QuoteMeta(pathSeparator) + ".*"
	if strings.HasSuffix(rePat, slashDotStar) {
		rePat = rePat[:len(rePat)-len(slashDotStar)] + "(" + slashDotStar + ")?"
	}
	rePat = `^` + rePat + `$`
	matcher, err := regexp.Compile(rePat)
	if err != nil {
		return nil, fmt.Errorf("Can't compile package path regexp %s: %v", rePat, err)
	}
	return matcher, nil
}

// resolveDirPath resolves dir into a Package.  Returns the package, or nil if
// it can't be resolved.  The pkgPath is used to set the path of the returned
// package; if it is empty, we attempt to deduce the package path.
func (ds *depSorter) resolveDirPath(dir, pkgPath string, mode UnknownPathMode) *Package {
	// If the package already exists in our dir map, we can just return it.
	absDir, err := filepath.Abs(dir)
	if err != nil {
		ds.errorf("%s: can't make absolute (%v)", dir, err)
	}
	if pkg := ds.dirMap[absDir]; pkg != nil {
		return pkg
	}
	// Ensure pkgPath is initialized, and corresponds to exactly one package.
	if pkgPath == "" {
		if pkgPath, err = ds.deducePackagePath(absDir); err != nil {
			ds.errorf("%s: can't deduce package path (%v)", absDir, err)
			return nil
		}
	}
	if !validPackagePath(pkgPath) {
		mode.logOrErrorf(ds.errs, "%s: package path %q is invalid", absDir, pkgPath)
		return nil
	}
	if pkg := ds.pathMap[pkgPath]; pkg != nil {
		mode.logOrErrorf(ds.errs, "%s: package path %q already resolved from %s", absDir, pkgPath, pkg.Dir)
		return nil
	}
	// Make sure the directory really exists, and add the package and deps.
	fileInfo, err := os.Stat(absDir)
	if err != nil {
		mode.logOrErrorf(ds.errs, "%v", err)
		return nil
	}
	if !fileInfo.IsDir() {
		mode.logOrErrorf(ds.errs, "%s: package isn't a directory", absDir)
		return nil
	}
	return ds.addPackageAndDeps(pkgPath, absDir, mode)
}

// resolveImportPath resolves pkgPath into a Package.  Returns the package, or
// nil if it can't be resolved.
func (ds *depSorter) resolveImportPath(pkgPath string, mode UnknownPathMode) *Package {
	pkgPath = path.Clean(pkgPath)
	if pkg := ds.pathMap[pkgPath]; pkg != nil {
		return pkg
	}
	if !validPackagePath(pkgPath) {
		mode.logOrErrorf(ds.errs, "Import path %q is invalid", pkgPath)
		return nil
	}
	// Look through srcDirs in-order until we find a valid package dir.
	var dirs []string
	for _, srcDir := range ds.srcDirs {
		dir := filepath.Join(srcDir, filepath.FromSlash(pkgPath))
		if pkg := ds.resolveDirPath(dir, pkgPath, UnknownPathIsIgnored); pkg != nil {
			vdlutil.Vlog.Printf("%s: resolved import path %q", pkg.Dir, pkgPath)
			return pkg
		}
		dirs = append(dirs, dir)
	}
	// We can't find a valid dir corresponding to this import path.
	detail := "   " + strings.Join(dirs, "\n   ")
	mode.logOrErrorf(ds.errs, "Can't resolve import path %q in any of:\n%s", pkgPath, detail)
	return nil
}

// addPackageAndDeps adds the pkg and its dependencies to the sorter.
func (ds *depSorter) addPackageAndDeps(path, dir string, mode UnknownPathMode) *Package {
	pkg := newPackage(path, dir, mode, ds.exts, ds.errs)
	if pkg == nil {
		return nil
	}
	vdlutil.Vlog.Printf("%s: resolved package path %q", pkg.Dir, pkg.Path)
	ds.dirMap[pkg.Dir] = pkg
	ds.pathMap[pkg.Path] = pkg
	ds.sorter.AddNode(pkg)
	pfiles := ParsePackage(pkg, parse.Opts{ImportsOnly: true}, ds.errs)
	pkg.Name = parse.InferPackageName(pfiles, ds.errs)
	for _, pf := range pfiles {
		ds.addImportDeps(pkg, pf.Imports)
	}
	return pkg
}

// addImportDeps adds transitive dependencies represented by imports to the
// sorter.  If the pkg is non-nil, an edge is added between the pkg and its
// dependencies; otherwise each dependency is added as an independent node.
func (ds *depSorter) addImportDeps(pkg *Package, imports []*parse.Import) {
	for _, imp := range imports {
		if dep := ds.resolveImportPath(imp.Path, UnknownPathIsError); dep != nil {
			if pkg != nil {
				ds.sorter.AddEdge(pkg, dep)
			} else {
				ds.sorter.AddNode(dep)
			}
		}
	}
}

// AddConfigDeps takes a config file represented by its base file name and src
// data, and adds all transitive dependencies to the sorter.
func (ds *depSorter) AddConfigDeps(baseFileName string, src io.Reader) {
	if pconfig := parse.ParseConfig(baseFileName, src, parse.Opts{ImportsOnly: true}, ds.errs); pconfig != nil {
		ds.addImportDeps(nil, pconfig.Imports)
	}
}

// deducePackagePath deduces the package path for dir, by looking for prefix
// matches against the src dirs.  The resulting package path may be incorrect
// even if no errors are reported; see the depSorter comment for details.
func (ds *depSorter) deducePackagePath(dir string) (string, error) {
	for _, srcDir := range ds.srcDirs {
		if strings.HasPrefix(dir, srcDir) {
			relPath, err := filepath.Rel(srcDir, dir)
			if err != nil {
				return "", err
			}
			return path.Clean(filepath.ToSlash(relPath)), nil
		}
	}
	return "", fmt.Errorf("no matching SrcDirs")
}

// Sort sorts all targets and returns the resulting list of Packages.
func (ds *depSorter) Sort() []*Package {
	sorted, cycles := ds.sorter.Sort()
	if len(cycles) > 0 {
		cycleStr := toposort.DumpCycles(cycles, printPackagePath)
		ds.errorf("Cyclic package dependency: %v", cycleStr)
		return nil
	}
	if len(sorted) == 0 {
		return nil
	}
	targets := make([]*Package, len(sorted))
	for ix, iface := range sorted {
		targets[ix] = iface.(*Package)
	}
	return targets
}

func printPackagePath(v interface{}) string {
	return v.(*Package).Path
}

// TransitivePackages takes a list of paths, and returns the corresponding
// packages and transitive dependencies, ordered by dependency.  Each path may
// either be a directory (IsDirPath) or an import (IsImportPath).
//
// A path is a pattern if it includes one or more "..." wildcards, each of which
// can match any string, including the empty string and strings containing
// slashes.  Such a pattern expands to all packages found in SrcDirs with names
// matching the pattern.  As a special-case, x/... matches x as well as x's
// subdirectories.
//
// The special-case "all" is a synonym for "...", and denotes all packages found
// in SrcDirs.
//
// Import path elements and file names are not allowed to begin with "." or "_";
// such paths are ignored in wildcard matches, and return errors if specified
// explicitly.
//
// The exts arg specifies the file name extensions for valid vdl files;
// e.g. ".vdl".  The mode specifies whether we should ignore or produce errors
// for paths that don't resolve to any packages.
func TransitivePackages(paths, exts []string, mode UnknownPathMode, errs *vdlutil.Errors) []*Package {
	ds := newDepSorter(exts, errs)
	for _, path := range paths {
		if !ds.ResolvePath(path, mode) {
			mode.logOrErrorf(errs, "Can't resolve %q to any packages", path)
		}
	}
	return ds.Sort()
}

// TransitivePackagesForConfig takes a config file represented by its base file
// name and src data, and returns all package dependencies in transitive order.
//
// The exts arg specifies the file name extensions for valid vdl files,
// e.g. ".vdl".
func TransitivePackagesForConfig(baseFileName string, src io.Reader, exts []string, errs *vdlutil.Errors) []*Package {
	ds := newDepSorter(exts, errs)
	ds.AddConfigDeps(baseFileName, src)
	return ds.Sort()
}

// ParsePackage parses the given pkg with the given parse opts, and returns a
// slice of parsed files, sorted by name.  Errors are reported in errs.
func ParsePackage(pkg *Package, opts parse.Opts, errs *vdlutil.Errors) (pfiles []*parse.File) {
	vdlutil.Vlog.Printf("Parsing package %s %q, dir %s", pkg.Name, pkg.Path, pkg.Dir)
	files, err := pkg.OpenFiles()
	if err != nil {
		errs.Errorf("Can't open vdl files %v, %v", pkg.BaseFileNames, err)
		return nil
	}
	for filename, src := range files {
		if pf := parse.Parse(filename, src, opts, errs); pf != nil {
			pfiles = append(pfiles, pf)
		}
	}
	sort.Sort(byBaseName(pfiles))
	pkg.CloseFiles()
	return
}

// byBaseName implements sort.Interface
type byBaseName []*parse.File

func (b byBaseName) Len() int           { return len(b) }
func (b byBaseName) Less(i, j int) bool { return b[i].BaseName < b[j].BaseName }
func (b byBaseName) Swap(i, j int)      { b[i], b[j] = b[j], b[i] }

// BuildPackage parses and compiles the given pkg, updates env with the compiled
// package and returns it.  Errors are reported in env.
//
// All imports that pkg depend on must have already been compiled and populated
// into env.
func BuildPackage(pkg *Package, env *compile.Env) *compile.Package {
	pfiles := ParsePackage(pkg, parse.Opts{}, env.Errors)
	return compile.Compile(pkg.Path, pfiles, env)
}

// BuildConfig parses and compiles the given config src and returns it.  Errors
// are reported in env; baseFileName is only used for error reporting.  If
// implicit is non-nil and the exported config const is an untyped const
// literal, it is assumed to be of that type.
//
// All imports that the config src depend on must have already been compiled and
// populated into env.
func BuildConfig(baseFileName string, src io.Reader, implicit *vdl.Type, env *compile.Env) *vdl.Value {
	pconfig := parse.ParseConfig(baseFileName, src, parse.Opts{}, env.Errors)
	return compile.CompileConfig(implicit, pconfig, env)
}
