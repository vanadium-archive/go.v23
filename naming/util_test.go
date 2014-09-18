package naming

import (
	"testing"

	"veyron.io/veyron/veyron2/ipc/version"
)

func TestFormat(t *testing.T) {
	testcases := []struct {
		network, address string
		opts             []EndpointOpt
		output           string
	}{
		{"tcp", "127.0.0.1:21", []EndpointOpt{}, "@2@tcp@127.0.0.1:21@@@@@"},
		{"tcp", "127.0.0.1:21", []EndpointOpt{NullRoutingID}, "@2@tcp@127.0.0.1:21@00000000000000000000000000000000@@@@"},
		{"tcp", "127.0.0.1:21", []EndpointOpt{version.IPCVersionRange{3, 5}}, "@2@tcp@127.0.0.1:21@@3@5@@"},
	}
	for _, test := range testcases {
		str := FormatEndpoint(test.network, test.address, test.opts...)
		if str != test.output {
			t.Errorf("unexpected endpoint string: got %q != %q", str, test.output)
		}
	}
}
