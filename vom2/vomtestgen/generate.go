package main

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"veyron.io/lib/cmdline"
	"veyron.io/veyron/veyron2/vdl"
	"veyron.io/veyron/veyron2/vdl/build"
	"veyron.io/veyron/veyron2/vdl/codegen"
	"veyron.io/veyron/veyron2/vdl/codegen/vdlgen"
	"veyron.io/veyron/veyron2/vdl/compile"
	"veyron.io/veyron/veyron2/vom2"
)

const (
	testpkg          = "veyron.io/veyron/veyron2/vom2/testdata"
	vomdataCanonical = testpkg + "/" + vomdataConfig
	vomdataConfig    = "vomdata.vdl.config"
)

var cmdGenerate = &cmdline.Command{
	Run:   runGenerate,
	Name:  "vomtestgen",
	Short: "Generate test data for the vom encoder / decoder",
	Long: `
The vomtestgen tool generates vom test data, using the vomdata file as input,
and creating a vdl file as output.
`,
	ArgsName: "[vomdata]",
	ArgsLong: `
[vomdata] is the path to the vomdata input file, specified in the vdl config
file format.  It must be of the form "NAME.vdl.config", and the output vdl
file will be generated at "NAME.vdl".

The config file should export a const []any that contains all of the values
that will be tested.  Here's an example:
   config = []any{
     bool(true), uint64(123), string("abc"),
   }

If not specified, we'll try to find the file at its canonical location:
   ` + vomdataCanonical,
}

var (
	optGenMaxErrors int
	optGenExts      string
)

func init() {
	cmdGenerate.Flags.IntVar(&optGenMaxErrors, "max_errors", -1, "Stop processing after this many errors, or -1 for unlimited.")
	cmdGenerate.Flags.StringVar(&optGenExts, "exts", ".vdl", "Comma-separated list of valid VDL file name extensions.")
}

func runGenerate(cmd *cmdline.Command, args []string) error {
	debug := new(bytes.Buffer)
	defer dumpDebug(cmd.Stderr(), debug)
	env := compile.NewEnv(optGenMaxErrors)
	// Get the input datafile path.
	var path string
	switch len(args) {
	case 0:
		srcDirs := build.SrcDirs(env.Errors)
		if err := env.Errors.ToError(); err != nil {
			return cmd.UsageErrorf("%v", err)
		}
		path = guessDataFilePath(debug, srcDirs)
		if path == "" {
			return cmd.UsageErrorf("couldn't find vomdata file in src dirs: %v", srcDirs)
		}
	case 1:
		path = args[0]
	default:
		return cmd.UsageErrorf("too many args (expecting exactly 1 vomdata file)")
	}
	inName := filepath.Clean(path)
	if !strings.HasSuffix(inName, ".vdl.config") {
		return cmd.UsageErrorf(`vomdata file doesn't end in ".vdl.config": %s`, inName)
	}
	outName := inName[:len(inName)-len(".config")]
	// Remove the generated file, so that it doesn't interfere with compiling the
	// config.  Ignore errors since it might not exist yet.
	if err := os.Remove(outName); err == nil {
		fmt.Fprintf(debug, "Removed output file %v\n", outName)
	}
	config, err := compileConfig(debug, inName, env)
	if err != nil {
		return err
	}
	data, err := generate(config)
	if err != nil {
		return err
	}
	if err := writeFile(data, outName); err != nil {
		return err
	}
	debug.Reset() // Don't dump debugging information on success
	fmt.Fprintf(cmd.Stdout(), "Wrote output file %v\n", outName)
	return nil
}

func dumpDebug(stderr io.Writer, debug *bytes.Buffer) {
	if d := debug.Bytes(); len(d) > 0 {
		io.Copy(stderr, debug)
	}
}

func guessDataFilePath(debug io.Writer, srcDirs []string) string {
	// Try to guess the data file path by looking for the canonical vomdata input
	// file in each of our source directories.
	for _, dir := range srcDirs {
		guess := filepath.Join(dir, vomdataCanonical)
		if stat, err := os.Stat(guess); err == nil && !stat.IsDir() {
			fmt.Fprintf(debug, "Found vomdata file %s\n", guess)
			return guess
		}
	}
	return ""
}

func compileConfig(debug io.Writer, inName string, env *compile.Env) (*vdl.Value, error) {
	basename := filepath.Base(inName)
	data, err := os.Open(inName)
	if err != nil {
		return nil, fmt.Errorf("couldn't open vomdata file %s: %v", inName, err)
	}
	defer func() { data.Close() }() // make defer use the latest value of data
	if stat, err := data.Stat(); err != nil || stat.IsDir() {
		return nil, fmt.Errorf("vomdata file %s is a directory", inName)
	}
	var opts build.Opts
	opts.Extensions = strings.Split(optGenExts, ",")
	// Compile package dependencies in transitive order.
	deps := build.TransitivePackagesForConfig(basename, data, opts, env.Errors)
	for _, dep := range deps {
		if pkg := build.BuildPackage(dep, env); pkg != nil {
			fmt.Fprintf(debug, "Built package %s\n", pkg.Path)
		}
	}
	// Try to seek back to the beginning of the data file, or if that fails try to
	// open the data file again.
	if off, err := data.Seek(0, 0); off != 0 || err != nil {
		if err := data.Close(); err != nil {
			return nil, fmt.Errorf("couldn't close vomdata file %s: %v", inName, err)
		}
		data, err = os.Open(inName)
		if err != nil {
			return nil, fmt.Errorf("couldn't re-open vomdata file %s: %v", inName, err)
		}
	}
	// Compile config into our test values.
	config := build.BuildConfig(basename, data, nil, env)
	if err := env.Errors.ToError(); err != nil {
		return nil, err
	}
	fmt.Fprintf(debug, "Compiled vomdata file %s\n", inName)
	return config, err
}

func generate(config *vdl.Value) ([]byte, error) {
	if got, want := config.Type(), vdl.ListType(vdl.AnyType); got != want {
		return nil, fmt.Errorf("got config type %v, want %v", got, want)
	}
	buf := new(bytes.Buffer)
	fmt.Fprintf(buf, `// This file was auto-generated via "vomtest generate".
// DO NOT UPDATE MANUALLY; read the comments in `+vomdataConfig+`.

package testdata
`)
	imports := codegen.ImportsForValue(config, testpkg)
	if len(imports) > 0 {
		fmt.Fprintf(buf, "\n%s\n", vdlgen.Imports(imports))
	}
	fmt.Fprintf(buf, `
// TestCase represents an individual testcase for vom encoding and decoding.
type TestCase struct {
	Name  string // Name of the testcase
	Value any    // Value to test
	Hex   string // Hex pattern representing vom encoding of Value
}

// Tests contains the testcases to use to test vom encoding and decoding.
const Tests = []TestCase {`)
	for ix := 0; ix < config.Len(); ix++ {
		value := config.Index(ix)
		if !value.IsNil() {
			// The config has type []any, and there's no need for our values to
			// include the "any" type explicitly, so we descend into the elem value.
			value = value.Elem()
		}
		valstr, err := vdlgen.TypedConst(value, testpkg, imports)
		if err != nil {
			return nil, fmt.Errorf("couldn't generate const %v: %v", value, err)
		}
		vomhex, vomdump, err := toVomHex(value)
		if err != nil {
			return nil, err
		}
		fmt.Fprintf(buf, `
%[3]s
	{
		%#[1]q,
		%[1]s,
		%[2]q,
	},`, valstr, vomhex, vomdump)
	}
	fmt.Fprintf(buf, `
}
`)
	return buf.Bytes(), nil
}

func toVomHex(value *vdl.Value) (string, string, error) {
	buf := new(bytes.Buffer)
	enc, err := vom2.NewBinaryEncoder(buf)
	if err != nil {
		return "", "", fmt.Errorf("vom.NewBinaryEncoder failed: %v", err)
	}
	if err := enc.Encode(value); err != nil {
		return "", "", fmt.Errorf("vom.Encode(%v) failed: %v", value, err)
	}
	vombytes := buf.String()
	const pre = "\t// "
	vomdump := pre + strings.Replace(vom2.Dump(buf.Bytes()), "\n", "\n"+pre, -1)
	if strings.HasSuffix(vomdump, "\n"+pre) {
		vomdump = vomdump[:len(vomdump)-len("\n"+pre)]
	}
	// TODO(toddw): Add hex pattern bracketing for map and set.
	return fmt.Sprintf("%x", vombytes), vomdump, nil
}

func writeFile(data []byte, outName string) error {
	// Create containing directory and write the file.
	dirName := filepath.Dir(outName)
	if err := os.MkdirAll(dirName, os.FileMode(0777)); err != nil {
		return fmt.Errorf("couldn't create directory %s: %v", dirName, err)
	}
	if err := ioutil.WriteFile(outName, data, os.FileMode(0666)); err != nil {
		return fmt.Errorf("couldn't write file %s: %v", outName, err)
	}
	return nil
}
