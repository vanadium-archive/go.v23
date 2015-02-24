package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const (
	testDir    = "../testdata/base"
	outPkgPath = "v.io/v23/vdl/testdata/base"
)

// Compares generated VDL files against the copy in the repo.
func TestVDLGenerator(t *testing.T) {
	// Setup the vdl command-line.
	cmdVDL.Init(nil, os.Stdout, os.Stderr)
	// Use vdl to generate Go code from input, into a temporary directory.
	outDir, err := ioutil.TempDir("", "vdltest")
	if err != nil {
		t.Fatalf("TempDir() failed: %v", err)
	}
	defer os.RemoveAll(outDir)
	// TODO(toddw): test the generated java and javascript files too.
	outOpt := fmt.Sprintf("--go_out_dir=%s", outDir)
	if err := cmdVDL.Execute([]string{"generate", "--lang=go", outOpt, testDir}); err != nil {
		t.Fatalf("Execute() failed: %v", err)
	}
	// Check that each *.vdl.go file in the testDir matches the generated output.
	entries, err := ioutil.ReadDir(testDir)
	if err != nil {
		t.Fatalf("ReadDir(%v) failed: %v", testDir, err)
	}
	numEqual := 0
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".vdl.go") {
			continue
		}
		testFile := filepath.Join(testDir, entry.Name())
		testBytes, err := ioutil.ReadFile(testFile)
		if err != nil {
			t.Fatalf("ReadFile(%v) failed: %v", testFile, err)
		}
		outFile := filepath.Join(outDir, outPkgPath, entry.Name())
		outBytes, err := ioutil.ReadFile(outFile)
		if err != nil {
			t.Fatalf("ReadFile(%v) failed: %v", outFile, err)
		}
		if !bytes.Equal(outBytes, testBytes) {
			t.Fatalf("GOT:\n%v\n\nWANT:\n%v\n", string(outBytes), string(testBytes))
		}
		numEqual++
	}
	if numEqual == 0 {
		t.Fatalf("testDir %s has no golden files *.vdl.go", testDir)
	}
}
