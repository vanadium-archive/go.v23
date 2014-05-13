package idl2

import (
	"io/ioutil"
	"log"
	"os"
)

const (
	logPrefix = ""
	logFlags  = log.Lshortfile | log.Ltime | log.Lmicroseconds
)

var (
	// Vlog is a logger that discards output by default, and only outputs real
	// logs when SetVerbose is called.
	Vlog = log.New(ioutil.Discard, logPrefix, logFlags)
)

// SetVerbose tells the idl package (and subpackages) to enable verbose logging.
func SetVerbose() {
	Vlog = log.New(os.Stderr, logPrefix, logFlags)
}
