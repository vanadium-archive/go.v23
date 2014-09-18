package cmds

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"

	"veyron.io/veyron/veyron/lib/cmdline"

	"veyron.io/veyron/veyron2/vdl/build"
	"veyron.io/veyron/veyron2/vdl/codegen/golang"
	"veyron.io/veyron/veyron2/vdl/codegen/java"
	"veyron.io/veyron/veyron2/vdl/codegen/javascript"
	"veyron.io/veyron/veyron2/vdl/compile"
	"veyron.io/veyron/veyron2/vdl/vdlutil"
)

func init() {
	log.SetFlags(log.Lshortfile | log.Ltime | log.Lmicroseconds)
}

func checkErrors(w io.Writer, env *compile.Env) {
	if !env.Errors.IsEmpty() {
		fmt.Fprintf(w, "ERROR\n%v", env.Errors.ToError())
		fmt.Fprintln(w, `   (run with "vdl -v" for verbose logging or "vdl help" for help)`)
		os.Exit(2)
	}
}

// runHelper returns a function that generates a sorted list of transitive
// targets, and calls the supplied run function.
func runHelper(run func(targets []*build.Package, env *compile.Env)) func(cmd *cmdline.Command, args []string) error {
	return func(cmd *cmdline.Command, args []string) error {
		if flagVerbose {
			vdlutil.SetVerbose()
		}
		if len(args) == 0 {
			// If the user doesn't specify any targets, the cwd is implied.
			args = append(args, ".")
		}
		exts := strings.Split(flagExts, ",")
		env := compile.NewEnv(flagMaxErrors)
		if flagExperimental {
			env.EnableExperimental()
		}
		targets := build.TransitivePackages(args, exts, env.Errors)
		checkErrors(cmd.Stderr(), env)
		if len(targets) == 0 {
			// The user's probably confused if we don't end up with any targets.
			return cmd.Errorf("no target packages specified")
		}
		run(targets, env)
		checkErrors(cmd.Stderr(), env)
		return nil
	}
}

const pkgDesc = `
<packages> are a list of packages to process, specified as arguments for each
command.  The format is similar to the go tool.  In its simplest form each
package is an import path; e.g. "veyron.io/veyron/veyron/lib/vdl".  A package that is an absolute
path or that contains a "." is interpreted as a file system path and denotes the
package in that directory.  A package that ends with "..." does a wildcard match
against all directories with that prefix.  The special import path "all" expands
to all package directories found in all the GOPATH trees.

For more information use "go help packages" to see the standard go package
documentation.
`

var cmdCompile = &cmdline.Command{
	Run:   runHelper(runCompile),
	Name:  "compile",
	Short: "Compile packages and dependencies, but don't generate code",
	Long: `
Compile compiles packages and their transitive dependencies, but does not
generate code.  This is useful to sanity-check that your VDL files are valid.
`,
	ArgsName: "<packages>",
	ArgsLong: pkgDesc,
}

var cmdGenerate = &cmdline.Command{
	Run:   runHelper(runGenerate),
	Name:  "generate",
	Short: "Compile packages and dependencies, and generate code",
	Long: `
Generate compiles packages and their transitive dependencies, and generates code
in the specified languages.
`,
	ArgsName: "<packages>",
	ArgsLong: pkgDesc,
}

var cmdAudit = &cmdline.Command{
	Run:   runHelper(runAudit),
	Name:  "audit",
	Short: "Check if any packages are stale and need generation",
	Long: `
Audit runs the same logic as generate, but doesn't write out generated files.
Returns a 0 exit code if all packages are up-to-date, otherwise returns a
non-0 exit code indicating some packages need generation.
`,
	ArgsName: "<packages>",
	ArgsLong: pkgDesc,
}

var cmdList = &cmdline.Command{
	Run:   runHelper(runList),
	Name:  "list",
	Short: "List package and dependency info in transitive order",
	Long: `
List returns information about packages and their transitive dependencies, in
transitive order.  This is the same order the generate and compile commands use
for processing.  If "vdl list A" is run and A depends on B, which depends on C,
the returned order will be C, B, A.  If multiple packages are specified the
ordering is over all combined dependencies.

Reminder: cyclic dependencies between packages are not allowed.  Cyclic
dependencies between VDL files within the same package are also not allowed.
This is more strict than regular Go; it makes it easier to generate code for
other languages like C++.
`,
	ArgsName: "<packages>",
	ArgsLong: pkgDesc,
}

const (
	genLangGo         genLang = "go"
	genLangJava               = "java"
	genLangJavascript         = "js"
)

var genLangAll = genLangs{genLangGo, genLangJava, genLangJavascript}

type genLang string

func (l genLang) String() string { return string(l) }

func genLangFromString(str string) (genLang, error) {
	for _, l := range genLangAll {
		if l == genLang(str) {
			return l, nil
		}
	}
	return "", fmt.Errorf("unknown language %s", str)
}

type genLangs []genLang

func (gls genLangs) String() string {
	var ret string
	for i, gl := range gls {
		if i > 0 {
			ret += ","
		}
		ret += gl.String()
	}
	return ret
}

func (gls *genLangs) Set(value string) error {
	// If the flag is repeated on the cmdline it is overridden.  Duplicates within
	// the comma separated list are ignored, and retain their original ordering.
	*gls = genLangs{}
	seen := make(map[genLang]bool)
	for _, str := range strings.Split(value, ",") {
		gl, err := genLangFromString(str)
		if err != nil {
			return err
		}
		if !seen[gl] {
			seen[gl] = true
			*gls = append(*gls, gl)
		}
	}
	return nil
}

// genOutDir has three modes:
//   1) If dir is non-empty, we use it as the out dir.
//   2) If xlate.rules is non-empty, we translate using the xlate rules.
//   3) If everything is empty, we generate in-place.
type genOutDir struct {
	dir   string
	xlate xlateRules
}

// xlateSrcDst specifies a translation rule, where src must match the suffix of
// the path just before the package path, and dst is the replacement for src.
// If dst is the special string "SKIP" we'll skip generation of packages
// matching the src.
type xlateSrcDst struct {
	src, dst string
}

// xlateRules specifies a collection of translation rules.
type xlateRules []xlateSrcDst

func (x *xlateRules) String() (ret string) {
	for _, srcdst := range *x {
		if len(ret) > 0 {
			ret += ","
		}
		ret += srcdst.src + "->" + srcdst.dst
	}
	return
}

func (x *xlateRules) Set(value string) error {
	for _, rule := range strings.Split(value, ",") {
		srcdst := strings.Split(rule, "->")
		if len(srcdst) != 2 {
			return fmt.Errorf("invalid out dir xlate rule %q (not src->dst format)", rule)
		}
		*x = append(*x, xlateSrcDst{srcdst[0], srcdst[1]})
	}
	return nil
}

func (x *genOutDir) String() string {
	if x.dir != "" {
		return x.dir
	}
	return x.xlate.String()
}

func (x *genOutDir) Set(value string) error {
	if strings.Contains(value, "->") {
		x.dir = ""
		return x.xlate.Set(value)
	}
	x.dir = value
	return nil
}

var (
	// Common flags for the tool itself, applicable to all commands.
	flagVerbose      bool
	flagMaxErrors    int
	flagExts         string
	flagExperimental bool

	// Options for each command.
	optCompileStatus bool
	optGenStatus     bool
	optGenGoFmt      bool
	optGenGoOutDir   = genOutDir{}
	optGenJavaOutDir = genOutDir{
		xlate: xlateRules{
			{"veyron.io/veyron/veyron/go/src", "veyron.io/veyron/veyron.new/java/src/main/java"},
			{"roadmap/go/src", "veyron.io/veyron/veyron.new/java/src/main/java"},
			{"third_party/go/src", "SKIP"},
		},
	}
	optGenJavascriptOutDir = genOutDir{
		xlate: xlateRules{
			{"veyron.io/veyron/veyron/go/src", "veyron.io/veyron/veyron.js/src"},
			{"roadmap/go/src", "veyron.io/veyron/veyron.js/java"},
			{"third_party/go/src", "SKIP"},
		},
	}
	optGenJavaPkgOut = xlateRules{
		{"veyron.io", "io/veyron"},
		{"veyron.io/veyron/veyron", "com/veyron"},
	}
	optGenLangs = genLangs{genLangGo, genLangJava} // TODO: javascript
)

// Root returns the root command for the VDL tool.
func Root() *cmdline.Command {
	vdlcmd := &cmdline.Command{
		Name:  "vdl",
		Short: "Manage veyron VDL source code",
		Long: `
The vdl tool manages veyron VDL source code.  It's similar to the go tool used
for managing Go source code.
`,
		Children: []*cmdline.Command{cmdGenerate, cmdCompile, cmdAudit, cmdList},
	}

	// Common flags for the tool itself, applicable to all commands.
	vdlcmd.Flags.BoolVar(&flagVerbose, "v", false, "Turn on verbose logging.")
	vdlcmd.Flags.IntVar(&flagMaxErrors, "max_errors", -1, "Stop processing after this many errors, or -1 for unlimited.")
	vdlcmd.Flags.StringVar(&flagExts, "exts", ".vdl", "Comma-separated list of valid VDL file name extensions.")
	vdlcmd.Flags.BoolVar(&flagExperimental, "experimental", false, "Enable experimental features that may crash the compiler and change without notice.  Intended for VDL compiler developers.")

	// Options for compile.
	cmdCompile.Flags.BoolVar(&optCompileStatus, "status", true, "Show package names while we compile")

	// Options for generate.
	cmdGenerate.Flags.Var(&optGenLangs, "lang", "Comma-separated list of languages to generate, currently supporting "+genLangAll.String())
	cmdGenerate.Flags.BoolVar(&optGenGoFmt, "go_fmt", true, "Format generated Go code")
	cmdGenerate.Flags.BoolVar(&optGenStatus, "status", true, "Show package names as they are updated")
	cmdGenerate.Flags.Var(&optGenGoOutDir, "go_out_dir",
		`Go output directory.  There are three modes:
         ""                     : Generate output in-place in the source tree
         "dir"                  : Generate output rooted at dir
         "src->dst[,s2->d2...]" : Generate output using translation rules
      Assume your source tree is organized as follows:
      GOPATH=/home/me/code/go
         /home/me/code/go/src/veyron2/vdl/test_base/base1.vdl
         /home/me/code/go/src/veyron2/vdl/test_base/base2.vdl
      Here's example output under the different modes:
      --go_out_dir=""
         /home/me/code/go/src/veyron2/vdl/test_base/base1.vdl.go
         /home/me/code/go/src/veyron2/vdl/test_base/base2.vdl.go
      --go_out_dir="/tmp/foo"
         /tmp/foo/veyron2/vdl/test_base/base1.vdl.go
         /tmp/foo/veyron2/vdl/test_base/base2.vdl.go
      --go_out_dir="go/src->foo/bar/src"
         /home/me/code/foo/bar/src/veyron2/vdl/test_base/base1.vdl.go
         /home/me/code/foo/bar/src/veyron2/vdl/test_base/base2.vdl.go
      When the src->dst form is used, src must match the suffix of the path
      just before the package path, and dst is the replacement for src.
      Use commas to separate multiple rules; the first rule matching src is
      used.  The special dst SKIP indicates all matching packages are skipped.`)
	cmdGenerate.Flags.Var(&optGenJavaOutDir, "java_out_dir",
		"Same semantics as --go_out_dir but applies to java code generation.")
	cmdGenerate.Flags.Var(&optGenJavaPkgOut, "java_out_pkg",
		`Java package translation rules.  Must be of the form:
           "src->dst[,s2->d2...]"
        If a VDL package has a prefix src, the prefix will be replaced with dst.
        Commas are used to separate multiple rules.  The first rule matching the
        package is used and if no rule matches the package the package remains
        intact.`)
	cmdGenerate.Flags.Var(&optGenJavascriptOutDir, "js_out_dir",
		"Same semantics as --go_out_dir but applies to js code generation.")
	// Options for audit are identical to generate.
	cmdAudit.Flags = cmdGenerate.Flags
	return vdlcmd
}

func runCompile(targets []*build.Package, env *compile.Env) {
	for _, target := range targets {
		pkg := build.CompilePackage(target, env)
		if pkg != nil && optCompileStatus {
			fmt.Println(pkg.Path)
		}
	}
}

func runGenerate(targets []*build.Package, env *compile.Env) {
	gen(false, targets, env)
}

func runAudit(targets []*build.Package, env *compile.Env) {
	if gen(true, targets, env) && env.Errors.IsEmpty() {
		// Some packages are stale, and there were no errors; return an arbitrary
		// non-0 exit code.  Errors are handled in runHelper, as usual.
		os.Exit(10)
	}
}

// gen generates the given targets with env.  If audit is true, only checks
// whether any packages are stale; otherwise files will actually be written out.
// Returns true if any packages are stale.
func gen(audit bool, targets []*build.Package, env *compile.Env) bool {
	anychanged := false
	for _, target := range targets {
		pkg := build.CompilePackage(target, env)
		if pkg == nil {
			// Stop at the first package that fails to compile.
			if env.Errors.IsEmpty() {
				env.Errors.Errorf("%s: internal error (compiled into nil package)", target.Path)
			}
			return true
		}
		// TODO(toddw): Skip code generation if the semantic contents of the
		// generated file haven't changed.
		pkgchanged := false
		for _, gl := range optGenLangs {
			switch gl {
			case genLangGo:
				dir, err := xlateOutDir(target, optGenGoOutDir, pkg.Path)
				if err != nil {
					if err != errSkip {
						env.Errors.Errorf("--go_out_dir error: %v", err)
					}
					continue
				}
				for _, file := range pkg.Files {
					opts := golang.Opts{Fmt: optGenGoFmt}
					data := golang.Generate(file, env, opts)
					if writeFile(audit, data, dir, file.BaseName+".go", env) {
						pkgchanged = true
					}
				}
			case genLangJava:
				java.SetPkgPathXlator(func(pkgPath string) string {
					return xlatePkgPath(pkgPath, optGenJavaPkgOut)
				})
				files := java.Generate(pkg, env)
				pkgPath := xlatePkgPath(pkg.Path, optGenJavaPkgOut)
				dir, err := xlateOutDir(target, optGenJavaOutDir, pkgPath)
				if err != nil {
					if err != errSkip {
						env.Errors.Errorf("--java_out_dir error: %v", err)
					}
					continue
				}
				for _, file := range files {
					fileDir := filepath.Join(dir, file.Dir)
					if writeFile(audit, file.Data, fileDir, file.Name, env) {
						pkgchanged = true
					}
				}
			case genLangJavascript:
				dir, err := xlateOutDir(target, optGenJavascriptOutDir, pkg.Path)
				if err != nil {
					if err != errSkip {
						env.Errors.Errorf("--js_out_dir error: %v", err)
					}
					continue
				}
				data := javascript.Generate(pkg, env)
				if writeFile(audit, data, dir, pkg.Name+".js", env) {
					pkgchanged = true
				}
			default:
				env.Errors.Errorf("Generating code for language %v isn't supported", gl)
			}
		}
		if pkgchanged {
			anychanged = true
			if optGenStatus {
				fmt.Println(pkg.Path)
			}
		}
	}
	return anychanged
}

// writeFile writes data into the standard location for file, using the given
// suffix.  Errors are reported via env.  Returns true iff the file doesn't
// already exist with the given data.
func writeFile(audit bool, data []byte, dirName, baseName string, env *compile.Env) bool {
	dstName := filepath.Join(dirName, baseName)
	// Don't change anything if old and new are the same.
	if oldData, err := ioutil.ReadFile(dstName); err == nil && bytes.Equal(oldData, data) {
		return false
	}
	if !audit {
		// Create containing directory, if it doesn't already exist.
		if err := os.MkdirAll(dirName, os.FileMode(0777)); err != nil {
			env.Errors.Errorf("Couldn't create directory %s: %v", dirName, err)
			return true
		}
		if err := ioutil.WriteFile(dstName, data, os.FileMode(0666)); err != nil {
			env.Errors.Errorf("Couldn't write file %s: %v", dstName, err)
			return true
		}
	}
	return true
}

var errSkip = fmt.Errorf("SKIP")

func xlateOutDir(pkg *build.Package, outdir genOutDir, outPkgPath string) (string, error) {
	// Strip package path from the directory.
	dir := pkg.Dir
	if !strings.HasSuffix(dir, pkg.Path) {
		return "", fmt.Errorf("package dir %q doesn't end with package path %q", dir, pkg.Path)
	}
	dir = filepath.Clean(dir[:len(dir)-len(pkg.Path)])

	switch {
	case outdir.dir != "":
		return filepath.Join(outdir.dir, outPkgPath), nil
	case len(outdir.xlate) == 0:
		return filepath.Join(dir, outPkgPath), nil
	}
	// Try translation rules in order.
	for _, xlate := range outdir.xlate {
		d := dir
		if !strings.HasSuffix(d, xlate.src) {
			continue
		}
		if xlate.dst == "SKIP" {
			return "", errSkip
		}
		d = filepath.Clean(d[:len(d)-len(xlate.src)])
		return filepath.Join(d, xlate.dst, outPkgPath), nil
	}
	return "", fmt.Errorf("package prefix %q doesn't match translation rules %q", dir, outdir)
}

func xlatePkgPath(pkgPath string, xRules xlateRules) string {
	for _, xlate := range xRules {
		if strings.HasPrefix(pkgPath, xlate.src) {
			return xlate.dst + pkgPath[len(xlate.src):]
		}
	}
	return pkgPath
}

func runList(targets []*build.Package, env *compile.Env) {
	for tx, target := range targets {
		num := fmt.Sprintf("%d", tx)
		fmt.Println(num, strings.Repeat("=", 80-len(num)))
		fmt.Printf("Name: %v\n", target.Name)
		fmt.Printf("Path: %v\n", target.Path)
		fmt.Printf("Dir:  %v\n", target.Dir)
		if len(target.BaseFileNames) > 0 {
			fmt.Print("Files:\n")
			for _, file := range target.BaseFileNames {
				fmt.Printf("   %v\n", file)
			}
		}
	}
}
