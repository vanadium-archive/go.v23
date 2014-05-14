package cmds

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
	testDir    = "../../test_base"
	outPkgPath = "veyron2/idl2/test_base"
)

func TestIDLGenerator(t *testing.T) {
	// Setup the idlc command-line.
	cmd := Root()
	cmd.Init(nil, os.Stdout, os.Stderr)
	// Use idlc to generate Go code from input, into a temporary directory.
	outDir, err := ioutil.TempDir("", "idltest")
	if err != nil {
		t.Fatalf("TempDir() failed: %v", err)
	}
	outOpt := fmt.Sprintf("--go_out_dir=%s", outDir)
	if err := cmd.Execute([]string{"generate", outOpt, testDir}); err != nil {
		t.Fatalf("Execute() failed: %v", err)
	}
	// Check that each *.idl.go file in the testDir matches the generated output.
	entries, err := ioutil.ReadDir(testDir)
	if err != nil {
		t.Fatalf("ReadDir(%v) failed: %v", testDir, err)
	}
	numEqual := 0
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".idl.go") {
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
		t.Fatalf("testDir %s has no golden files *.idl.go", testDir)
	}
}
