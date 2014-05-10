package vlog_test

import (
	"flag"
	"fmt"
	"testing"

	_ "veyron/lib/testutil"
	"veyron/lib/testutil/blackbox"

	"veyron2/vlog"
)

func TestHelperProcess(t *testing.T) {
	blackbox.HelperProcess(t)
}

func init() {
	blackbox.CommandTable["child"] = child
}

func child(args []string) {
	flag.Set("log_dir", "/tmp/foo")
	flag.Set("vmodule", "foo=2")
	flags := vlog.Log.ExplicitlySetFlags()
	if v, ok := flags["log_dir"]; !ok || v != "/tmp/foo" {
		panic(fmt.Sprintf("log_dir was supposed to be /tmp/foo"))
	}
	if v, ok := flags["vmodule"]; !ok || v != "foo=2" {
		panic(fmt.Sprintf("vmodule was supposed to be foo=2"))
	}
	if f := flag.Lookup("max_stack_buf_size"); f == nil {
		panic("max_stack_buf_size is not a flag")
	}
	maxStackBufSizeSet := false
	flag.Visit(func(f *flag.Flag) {
		if f.Name == "max_stack_buf_size" {
			maxStackBufSizeSet = true
		}
	})
	if v, ok := flags["max_stack_buf_size"]; ok && !maxStackBufSizeSet {
		panic(fmt.Sprintf("max_stack_buf_size unexpectedly set to %v", v))
	}
}

func TestFlags(t *testing.T) {
	c := blackbox.HelperCommand(t, "child")
	c.Cmd.Start()
	c.ExpectEOFAndWait()
	c.Cleanup()
}
