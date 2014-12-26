// +build ignore

package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"v.io/core/veyron2/naming"
	"v.io/core/veyron2/rt"

	_ "v.io/core/veyron/profiles"
)

var protocolFlag = protocolFlagVar{"tcp"}
var hostPortFlags = hostPortFlagVar{}

type protocolFlagVar struct {
	protocol string
}

func (pf *protocolFlagVar) Set(v string) error {
	pf.protocol = v
	return nil
}

func (pf *protocolFlagVar) String() string {
	return pf.protocol
}

type addrSpec struct {
	protocol, address string
}

type hostPortFlagVar struct {
	addrs []addrSpec
}

func (hpf *hostPortFlagVar) Set(v string) error {
	hpf.addrs = append(hpf.addrs, addrSpec{protocol: protocolFlag.String(), address: v})
	return nil
}

func (hpf *hostPortFlagVar) String() string {
	r := ""
	for _, a := range hpf.addrs {
		r += fmt.Sprintf("%s %s\n", a.protocol, a.address)
	}
	return strings.TrimRight(r, "\n")
}

func init() {
	flag.Var(&protocolFlag, "protocol", "protocol to use for subsequent host args")
	flag.Var(&hostPortFlags, "address", "<host:port>...")
}

func main() {
	runtime, err := rt.New()
	if err != nil {
		vlog.Fatalf("Could not initialize runtime: %v", err)
	}
	defer runtime.Cleanup()

	for _, a := range hostPortFlags.addrs {
		ep, err := runtime.NewEndpoint(naming.FormatEndpoint(a.protocol, a.address))
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: %s\n", ep, err)
			os.Exit(1)
		}
		fmt.Printf("%s\n", ep)
	}
}
