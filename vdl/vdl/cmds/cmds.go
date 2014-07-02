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

	"veyron/lib/cmdline"

	"veyron2/vdl"
	"veyron2/vdl/build"
	"veyron2/vdl/compile"
	"veyron2/vdl/gen"
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
			vdl.SetVerbose()
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
package is an import path; e.g. "veyron/lib/vdl".  A package that is an absolute
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

var cmdGen = &cmdline.Command{
	Run:   runHelper(runGen),
	Name:  "generate",
	Short: "Compile packages and dependencies, and generate code",
	Long: `
Generate compiles packages and their transitive dependencies, and generates code
in the specified languages.
`,
	ArgsName: "<packages>",
	ArgsLong: pkgDesc,
}

var cmdListInfo = &cmdline.Command{
	Run:   runHelper(runListInfo),
	Name:  "listinfo",
	Short: "List package and dependency info in transitive order",
	Long: `
Listinfo returns information about packages and their transitive dependencies,
in transitive order.  This is the same order the generate and compile commands
use for processing.  If "vdl listinfo A" is run and A depends on B, which
depends on C, the returned order will be C, B, A.  If multiple packages are
specified the ordering is over all combined dependencies.

Reminder: cyclic dependencies between packages are not allowed.  Cyclic
dependencies between VDL files within the same package are also not allowed.
This is more strict than regular Go; it makes it easier to generate code for
other languages like C++.
`,
	ArgsName: "<packages>",
	ArgsLong: pkgDesc,
}

const (
	genLangGo genLang = iota
	genLangJava
	genLangJavascript
	numGenLang
)

type genLang int

func (l genLang) String() string {
	switch l {
	case genLangGo:
		return "go"
	case genLangJava:
		return "java"
	case genLangJavascript:
		return "js"
	}
	panic(fmt.Errorf("Unhandled language %d", l))
}

func genLangFromString(str string) genLang {
	switch str {
	case "go":
		return genLangGo
	case "java":
		return genLangJava
	case "js":
		return genLangJavascript
	}
	panic(fmt.Errorf("Unknown language %s", str))
}

type genLangs map[genLang]bool

func (gls genLangs) String() string {
	ret := "["
	for gl, _ := range gls {
		if ret != "[" {
			ret += ", "
		}
		ret += gl.String()
	}
	ret += "]"
	return ret
}

func (gls genLangs) Set(value string) error {
	// We allow this flag to be repeated on the cmdline.
	for _, str := range strings.Split(value, ",") {
		gls[genLangFromString(str)] = true
	}
	return nil
}

// There are three modes for genOutDir:
//   1) If dir is non-empty, we use it as the out dir.
//   2) If src or dst are non-empty, we translate from src to dst suffix.
//   3) If everything is empty, we generate in-place.
type genOutDir struct {
	dir      string
	src, dst string
}

func (x *genOutDir) String() string {
	switch {
	case x.dir != "":
		return x.dir
	case x.src != "" || x.dst != "":
		return fmt.Sprintf("%s->%s", x.src, x.dst)
	}
	return ""
}

func (x *genOutDir) Set(value string) error {
	if strs := strings.Split(value, "->"); len(strs) == 2 {
		x.dir = ""
		x.src = strs[0]
		x.dst = strs[1]
		return nil
	}
	x.dir = value
	x.src = ""
	x.dst = ""
	return nil
}

var (
	// Common flags for the tool itself, applicable to all commands.
	flagVerbose      bool
	flagMaxErrors    int
	flagExts         string
	flagExperimental bool

	// Options for each command.
	optCompileStatus       bool
	optGenStatus           bool
	optGenGoFmt            bool
	optGenGoOutDir         = genOutDir{}
	optGenJavaOutDir       = genOutDir{src: "go/src", dst: "java/src"}
	optGenJavascriptOutDir = genOutDir{src: "go/src", dst: "javascript/src"}
	optGenJavaPkgPrefix    string
	optGenLangs            = genLangs{genLangGo: true}
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
		Children: []*cmdline.Command{cmdGen, cmdCompile, cmdListInfo},
	}

	// Common flags for the tool itself, applicable to all commands.
	vdlcmd.Flags.BoolVar(&flagVerbose, "v", false, "Turn on verbose logging.")
	vdlcmd.Flags.IntVar(&flagMaxErrors, "max_errors", -1, "Stop processing after this many errors, or -1 for unlimited.")
	vdlcmd.Flags.StringVar(&flagExts, "exts", ".vdl", "Comma-separated list of valid VDL file name extensions.")
	vdlcmd.Flags.BoolVar(&flagExperimental, "experimental", false, "Enable experimental features that may crash the compiler and change without notice.  Intended for VDL compiler developers.")

	// Options for compile.
	cmdCompile.Flags.BoolVar(&optCompileStatus, "status", true, "Show package names while we compile")

	// Options for generate.
	var allLangs string
	for lx := 0; lx < int(numGenLang); lx++ {
		if lx > 0 {
			allLangs += ", "
		}
		allLangs += `"` + genLang(lx).String() + `"`
	}
	cmdGen.Flags.Var(&optGenLangs, "lang", "Comma-separated list of languages to generate, currently supporting "+allLangs)
	cmdGen.Flags.BoolVar(&optGenGoFmt, "go_fmt", true, "Format generated Go code")
	cmdGen.Flags.BoolVar(&optGenStatus, "status", true, "Show package names while we compile")
	cmdGen.Flags.StringVar(&optGenJavaPkgPrefix, "java_pkg_prefix", "com",
		"Package prefix that will be added to the VDL package prefixes when generating Java files. ")
	cmdGen.Flags.Var(&optGenGoOutDir, "go_out_dir",
		`Go output directory.  There are three modes:
			""         : Generate output in-place in the source tree
			"dir"      : Generate output rooted at dir
			"src->dst" : Generate output rooted at x, with translation from src to dst
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
	 --go_out_dir="go/src->bar/src"
			/home/me/code/bar/src/veyron2/vdl/test_base/base1.vdl.go
			/home/me/code/bar/src/veyron2/vdl/test_base/base2.vdl.go
	 When the src->dst form is used, src must match the suffix of the path just
	 before the package path, and dst is the replacement for src.`)
	cmdGen.Flags.Var(&optGenJavaOutDir, "java_out_dir",
		"Same semantics as --go_out_dir but applies to java code generation.")
	cmdGen.Flags.Var(&optGenJavascriptOutDir, "js_out_dir",
		"Same semantics as --go_out_dir but applies to js code generation.")
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

func runGen(targets []*build.Package, env *compile.Env) {
	for _, target := range targets {
		pkg := build.CompilePackage(target, env)
		if pkg == nil {
			continue
		}
		// TODO(toddw): Skip code generation if the semantic contents of the
		// generated file haven't changed.
		changed := false
		for gl, _ := range optGenLangs {
			switch gl {
			case genLangGo:
				dir, err := xlateOutDir(target, optGenGoOutDir, "")
				if err != nil {
					env.Errors.Errorf("--go_out_dir error: %v", err)
					continue
				}
				for _, file := range pkg.Files {
					opts := gen.GoOpts{Fmt: optGenGoFmt}
					data := gen.GoFile(file, env, opts)
					if writeFile(data, dir, file.BaseName+".go", env) {
						changed = true
					}
				}
			case genLangJava:
				gen.SetJavaGenPkgPrefix(optGenJavaPkgPrefix)
				files := gen.GenJavaFiles(pkg, env)
				dir, err := xlateOutDir(target, optGenJavaOutDir, optGenJavaPkgPrefix)
				if err != nil {
					env.Errors.Errorf("--java_out_dir error: %v", err)
					continue
				}
				for _, file := range files {
					fileDir := filepath.Join(dir, file.Dir)
					if writeFile(file.Data, fileDir, file.Name, env) {
						changed = true
					}
				}
			case genLangJavascript:
				dir, err := xlateOutDir(target, optGenJavascriptOutDir, "")
				if err != nil {
					env.Errors.Errorf("--js_out_dir error: %v", err)
					continue
				}
				data := gen.GenJavascriptFiles(pkg)
				if writeFile(data, dir, pkg.Name+".js", env) {
					changed = true
				}
			default:
				env.Errors.Errorf("Generating code for language %v isn't supported", gl)
			}
		}
		if changed && optGenStatus {
			fmt.Println(pkg.Path)
		}
	}
}

// writeFile writes data into the standard location for file, using the given
// suffix.  Errors are reported via env.  Returns true iff a new file was
// written; returns false if the file already exists with the given data.
func writeFile(data []byte, dirName, baseName string, env *compile.Env) bool {
	// Create containing directory, if it doesn't already exist.
	if err := os.MkdirAll(dirName, os.FileMode(0777)); err != nil {
		env.Errors.Errorf("Couldn't create directory %s: %v", dirName, err)
		return false
	}
	dstName := filepath.Join(dirName, baseName)
	// Don't change anything if old and new are the same.
	if oldData, err := ioutil.ReadFile(dstName); err == nil && bytes.Equal(oldData, data) {
		return false
	}
	if err := ioutil.WriteFile(dstName, data, os.FileMode(0666)); err != nil {
		env.Errors.Errorf("Couldn't write file %s: %v", dstName, err)
		return false
	}

	return true
}

func xlateOutDir(pkg *build.Package, xlate genOutDir, pkgPrefix string) (string, error) {
	switch {
	case xlate.dir != "":
		return filepath.Join(xlate.dir, pkg.Path), nil
	case xlate.src == "" && xlate.dst == "":
		return pkg.Dir, nil
	}
	// Translate src suffix to dst suffix.
	d := pkg.Dir
	if !strings.HasSuffix(d, pkg.Path) {
		return "", fmt.Errorf("package dir %q doesn't end with package path %q", d, pkg.Path)
	}
	d = filepath.Clean(d[:len(d)-len(pkg.Path)])
	if !strings.HasSuffix(d, xlate.src) {
		return "", fmt.Errorf("package dir %q doesn't end with xlate src %q", d, xlate.src)
	}
	d = filepath.Clean(d[:len(d)-len(xlate.src)])
	return filepath.Join(d, xlate.dst, pkgPrefix, pkg.Path), nil
}

func runListInfo(targets []*build.Package, env *compile.Env) {
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
