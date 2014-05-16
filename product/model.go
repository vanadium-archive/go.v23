package product

import (
	"net"
	"os"

	"veyron2/security"
)

// T must be implemented by all products that run Veyron.
type T interface {
	// Vendor and model are the names for the vendor and model.
	// Name is a user or vendor assigned insecure identifier
	// for this device, it need not be unique across devices.
	Description() (vendor, model, name string)

	// ID is a cryptographically secure ID for this device.  It is unique across
	// all devices from all vendors.
	ID() security.PublicID

	// Addresses returns a slice of network addresses that can be
	// used to serve requests. The order is important, being in order
	// of preference; that is, the first address is the most preferred,
	// the second less preferred and so on.
	Addresses() []net.Addr
}

// DetermineProduct will be called by the runtime Init routine to determine
// the product that this process is running on. The idea is for product
// specific configuration to be encapsulated by this and to provide a single
// location for vendor's to encapsulate product specific functionality required
// by the runtime.
// TODO(cnicolaou): this still feels a little klunky, maybe there's a more
// pluggable mechanism.
func DetermineProduct() (T, error) {
	return DetermineProductFromEnv()
}

// DetermineProductFromEnv examines the current processes environment variables
// to determine what the product vendor and model are
func DetermineProductFromEnv() (T, error) {
	vendor := os.Getenv("VEYRON_VENDOR")
	model := os.Getenv("VEYRON_MODEL")
	switch vendor {
	// Vendors should fill this in as appopriate.
	case "google":
		fallthrough
	default:
		return googleProduct(model)
	}
}
