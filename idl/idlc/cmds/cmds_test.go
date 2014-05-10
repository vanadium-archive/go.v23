package cmds

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const (
	inputFile          = "../../build/test_base/base.idl"
	expectedOutputFile = "../../build/test_base/base.idl.go"
)

func TestIDLGenerator(t *testing.T) {
	// Setup the idlc command-line.
	cmd := Root()
	cmd.Init(nil, os.Stdout, os.Stderr)
	// Read the input file and expected output file into memory.
	input, err := ioutil.ReadFile(inputFile)
	if err != nil {
		t.Fatalf("ReadFile(%v) failed: %v\n", inputFile, err)
	}
	expectedOutput, err := ioutil.ReadFile(expectedOutputFile)
	if err != nil {
		t.Fatalf("ReadFile(%v) failed: %v\n", expectedOutputFile, err)
	}
	// Use idlc to generate Go code from input.
	cwd, _ := os.Getwd()
	f, err := ioutil.TempFile(cwd, "")
	if err != nil {
		t.Fatalf("TempFile() failed: %v\n", err)
	}
	defer os.Remove(f.Name())
	defer f.Close()
	if _, err := f.Write(input); err != nil {
		t.Fatalf("WriteString() failed: %v\n", err)
	}
	newName := f.Name() + ".idl"
	if err := os.Rename(f.Name(), newName); err != nil {
		t.Fatalf("Rename() failed: %v\n", err)
	}
	defer os.Remove(newName)
	dir, file := filepath.Split(newName)
	if err := cmd.Execute([]string{"generate", dir}); err != nil {
		t.Fatalf("Execute() failed: %v\n", err)
	}
	newGoName := newName + ".go"
	defer os.Remove(newGoName)
	// Check the generated file matches <expectedOutput>.
	bytes, err := ioutil.ReadFile(newGoName)
	if err != nil {
		t.Fatalf("ReadFile(%v) failed: %v\n", newGoName, err)
	}
	output := strings.Replace(string(bytes), file, "base.idl", -1)
	if string(expectedOutput) != output {
		t.Fatalf("Unexpected output:\nexpected=%q\ngot=%q\n", string(expectedOutput), output)
	}
}
