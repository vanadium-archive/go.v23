// +build android

package main

import (
	"flag"
	"unsafe"

	_ "veyron/runtimes/google/ipc/jni"
)

// #include <jni.h>
import "C"

//export JNI_OnLoad
func JNI_OnLoad(jVM *C.JavaVM, reserved unsafe.Pointer) C.jint {
	return C.JNI_VERSION_1_6
}

func main() {
	// Send all logging to stderr, so that the output is visible in Android.  Note that if this
	// flag is removed, the process will likely crash as android requires that all logs are written
	// into a specific directory.
	flag.Set("logtostderr", "true")
}
