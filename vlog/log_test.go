package vlog_test

import (
	"bufio"
	"bytes"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"veyron2/vlog"
)

func ExampleConfigure() {
	vlog.ConfigureLogger()
}

func ExampleInfo() {
	vlog.Info("hello")
}

func ExampleError() {
	vlog.Errorf("%s", "error")
	if vlog.V(2) {
		vlog.Info("some spammy message")
	}
	vlog.VI(2).Infof("another spammy message")
}

func readLogFiles(dir string) (string, error) {
	files, err := ioutil.ReadDir(dir)
	if err != nil {
		return "", err
	}
	contents := ""
	for _, fi := range files {
		file, err := os.Open(filepath.Join(dir, fi.Name()))
		if err != nil {
			continue
		}
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			contents = contents + scanner.Text() + "\n"
		}
	}
	return contents, nil
}

func TestHeaders(t *testing.T) {
	dir, err := ioutil.TempDir(".", "logtest")
	defer os.RemoveAll(dir)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	logger, err := vlog.NewLogger("testHeader", vlog.LogDir(dir), vlog.Level(2))
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	logger.Infof("abc\n")
	logger.Infof("wombats\n")
	logger.VI(1).Infof("wombats again\n")
	logger.FlushLog()
	contents, err := readLogFiles(dir)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	scanner := bufio.NewScanner(bytes.NewBufferString(contents))
	fileRE := regexp.MustCompile(`\S+ \S+ \S+ (.*):.*`)
	got := 0
	for scanner.Scan() {
		line := scanner.Text()
		if line[0] != 'I' {
			continue
		}
		name := fileRE.FindStringSubmatch(line)
		if len(name) == 0 {
			t.Errorf("failed to find file in %s", line)
			continue
		}
		if name[1] != "log_test.go" {
			t.Errorf("unexpected file name: %s", name[1])
			continue
		}
		got++
	}
	if got != 3 {
		t.Errorf("failed to match the expected number of lines: %d", got)
	}
}

func TestVModule(t *testing.T) {
	dir, err := ioutil.TempDir(".", "logtest")
	defer os.RemoveAll(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	logger, err := vlog.NewLogger("testVmodule", vlog.LogDir(dir))
	if logger.V(2) || logger.V(3) {
		t.Errorf("Logging should not be enabled at levels 2 & 3")
	}
	spec := vlog.ModuleSpec{}
	if err := spec.Set("*log_test=2"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := logger.ConfigureLogger(spec); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !logger.V(2) {
		t.Errorf("logger.V(2) should be true")
	}
	if logger.V(3) {
		t.Errorf("logger.V(3) should be false")
	}
	if vlog.V(2) || vlog.V(3) {
		t.Errorf("Logging should not be enabled at levels 2 & 3")
	}
	vlog.Log.ConfigureLogger(spec)
	if !vlog.V(2) {
		t.Errorf("vlog.V(2) should be true")
	}
	if vlog.V(3) {
		t.Errorf("vlog.V(3) should be false")
	}
	if vlog.VI(2) != vlog.Log {
		t.Errorf("vlog.V(2) should be vlog.Log")
	}
	if vlog.VI(3) == vlog.Log {
		t.Errorf("vlog.V(3) should not be vlog.Log")
	}
}
