package naming

import (
	"strconv"

	"veyron.io/veyron/veyron2/ipc/version"
)

// EndpointOpt must be implemented by all optional parameters to FormatEndpoint
type EndpointOpt interface {
	EndpointOpt()
}

// FormatEndpoint creates a string representation of an Endpoint using
// the supplied parameters. Network and address are always required,
// RoutingID, and IPCVersionRange can be used as options.
func FormatEndpoint(network, address string, opts ...EndpointOpt) string {
	rid := "@"

	versions := "@@"

	for _, o := range opts {
		switch v := o.(type) {
		case RoutingID:
			rid = "@" + v.String()
		case version.IPCVersionRange:
			versions = "@" + strconv.FormatUint(uint64(v.Min), 10) +
				"@" + strconv.FormatUint(uint64(v.Max), 10)
		}
	}

	return "@2@" + network + "@" + address + rid + versions + "@@"
}
