package product

import (
	"log"
	"net"
)

// Return the hosts IPv4 addresses.
func hostIPv4Addrs() []net.Addr {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		log.Fatalf("Failed to find any network interfaces: %s", err)
	}
	ipv4addrs := make([]net.Addr, 0, len(addrs))
	for _, addr := range addrs {
		ipaddr, ok := addr.(*net.IPNet)
		if !ok {
			continue
		}
		if ipaddr.IP.IsLoopback() {
			continue
		}
		if ip4 := ipaddr.IP.To4(); ip4 != nil {
			ipv4addrs = append(ipv4addrs, addr)
		}
	}
	return ipv4addrs
}
