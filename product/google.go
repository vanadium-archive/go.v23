package product

import (
	"fmt"
	"net"
	"os"
	"strings"

	"veyron2/security"
)

func googleProduct(model string) (T, error) {
	hostname, err := os.Hostname()
	// This is a temporary hack. For now, servers run on GCE, or AWS,
	// everything else is essentially a raspberry pi.
	switch model {
	case "nameserver", "mounttable", "proxy":
		if err != nil {
			return nil, err
		}
		if strings.HasPrefix(hostname, "vcloud") {
			return newGCEProduct(model, hostname), nil
		}
		return nil, fmt.Errorf("unrecognised google product model: %q", model)
	default:
		return newPiProduct(model, hostname), nil
	}
}

// GCE products
type gceProduct struct{ name string }

func newGCEProduct(model, name string) T {
	return &gceProduct{name}
}

func (gce *gceProduct) Description() (vendor, model, name string) {
	return "google", "gce", gce.name
}

func (gce *gceProduct) Addresses() []net.Addr {
	return hostIPv4Addrs()
}

func (gce *gceProduct) ID() security.PublicID {
	// TODO(cnicolaou): fill the real value when the security code is ready
	v, m, n := gce.Description()
	return security.FakePublicID(fmt.Sprintf("%s/%s/%s", v, m, n))
}

// Raspberry Pi products
type piProduct struct{ name string }

func newPiProduct(model, hostname string) T {
	return &piProduct{hostname}
}

func (pi *piProduct) Description() (vendor, model, name string) {
	return "google", "raspberry_pi", pi.name
}

func (pi *piProduct) Addresses() []net.Addr {
	ipv4addrs := hostIPv4Addrs()
	for _, a := range ipv4addrs {
		// This is a hack to make sure we return only externally
		// routeable addresses. Note that this doesn't work in the
		// general case since it doesn't test for all of the
		// non-routable IP subnets, just the 10. that we use on
		// google corp. It wouldn't work on my home network for
		// instance which uses 172.16. etc.
		// TODO(cnicolaou): test for the full set of
		// non-routeable subnets.
		if !strings.HasPrefix(a.String(), "10.") {
			return []net.Addr{a}
		}
	}
	return []net.Addr{}
}

func (pi *piProduct) ID() security.PublicID {
	// TODO(cnicolaou): fill the real value when the security code is ready
	v, m, n := pi.Description()
	return security.FakePublicID(fmt.Sprintf("%s/%s/%s", v, m, n))
}
