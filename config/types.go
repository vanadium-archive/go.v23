package config

import (
	"fmt"
	"net"
	"strconv"
	"time"
)

// Format formats a Setting in a consistent manner, it is intended to be
// used when implementing the Setting interface.
func Format(s Setting) string {
	return fmt.Sprintf("%s: %s: (%T: %s)", s.Name(), s.Description(), s.Value(), s.Value())
}

type impl struct {
	name, description string
	value             interface{}
}

func (s *impl) String() string {
	return Format(s)
}

func (s *impl) Name() string {
	return s.name
}

func (s *impl) Description() string {
	return s.description
}

func (s *impl) Value() interface{} {
	return s.value
}

func NewInt(name, description string, value int) Setting {
	return &impl{name, description, value}
}

func NewInt64(name, description string, value int64) Setting {
	return &impl{name, description, value}
}

func NewBool(name, description string, value bool) Setting {
	return &impl{name, description, value}
}

func NewFloat64(name, description string, value float64) Setting {
	return &impl{name, description, value}
}

func NewString(name, description string, value string) Setting {
	return &impl{name, description, value}
}

func NewDuration(name, description string, value time.Duration) Setting {
	return &impl{name, description, value}
}

func NewAddr(name, description string, value net.Addr) Setting {
	return &impl{name, description, value}
}

func NewIP(name, description string, value net.IP) Setting {
	return &impl{name, description, value}
}

func NewIPHostPort(name, description string, value IPHostPortFlag) Setting {
	return &impl{name, description, value}
}

// TCPProtocolFlag implements flag.Value to provide validation of the
// command line values passed to it: tcp, tcp4 or tcp6 being the
// only allowed values.
type TCPProtocolFlag struct{ Protocol string }

// Implements flag.Value.Get
func (t TCPProtocolFlag) Get() interface{} {
	return t.Protocol
}

// Implements flag.Value.Set
func (t *TCPProtocolFlag) Set(s string) error {
	switch s {
	case "tcp", "tcp4", "tcp6":
		t.Protocol = s
		return nil
	default:
		return fmt.Errorf("%q is not a tcp protocol", s)
	}

}

// Implements flag.Value.String
func (t TCPProtocolFlag) String() string {
	return t.Protocol
}

// IPHostPortFlag implements flag.Value to provide validation of the
// command line value it is set to. The allowed format is <host>:<port> in
// ip4 and ip6 formats. The host may be specified as a hostname or as an IP
// address (v4 or v6). If a hostname is used and it resolves to multiple IP
// addresses then all of those addresses are stored in IPHostPort.
type IPHostPortFlag struct {
	Host string
	IP   []net.IP
	Port string
}

// Implements flag.Value.Get
func (ip IPHostPortFlag) Get() interface{} {
	return ip.String()
}

// Implements flag.Value.Set
func (ip *IPHostPortFlag) Set(s string) error {
	host, port, err := net.SplitHostPort(s)
	if err != nil {
		// no port number in s.
		host = s
		ip.Port = "0"
	} else {
		// have a port in s.
		if _, err := strconv.ParseUint(port, 10, 16); err != nil {
			return fmt.Errorf("failed to parse port number from %s", s)
		}
		ip.Port = port
	}
	// if len(host) == 0 then we have no host, just a port.
	if len(host) > 0 {
		if addr := net.ParseIP(host); addr == nil {
			// Could be a hostname.
			addrs, err := net.LookupIP(host)
			if err != nil {
				return fmt.Errorf("%s is neither an IP address nor a host name:%s", host, err)
			}
			ip.IP = addrs
			ip.Host = host
		} else {
			ip.IP = []net.IP{addr}
		}
		return nil
	}
	return nil
}

// Implements flag.Value.String
func (ip IPHostPortFlag) String() string {
	host := ip.Host
	if len(ip.Host) == 0 && ip.IP != nil && len(ip.IP) > 0 {
		// We don't have a hostname, so there should be at most one IP address.
		host = ip.IP[0].String()
	}
	return net.JoinHostPort(host, ip.Port)
}

// IPFlag implements flag.Value in order to provide validation of
// IP addresses in the flag package.
type IPFlag struct{ net.IP }

// Implements flag.Value.Get
func (ip IPFlag) Get() interface{} {
	return ip.IP
}

// Implements flag.Value.Set
func (ip *IPFlag) Set(s string) error {
	t := net.ParseIP(s)
	if t == nil {
		return fmt.Errorf("failed to parse %s as an IP address", s)
	}
	ip.IP = t
	return nil
}

// Implements flag.Value.String
func (ip IPFlag) String() string {
	return ip.IP.String()
}

// DurationFlag implements flag.Value in order to provide validation of
// duration values in the flag package.
type DurationFlag struct{ time.Duration }

// Implements flag.Value.Get
func (d DurationFlag) Get() interface{} {
	return d.Duration
}

// Implements flag.Value.Set
func (d *DurationFlag) Set(s string) error {
	duration, err := time.ParseDuration(s)
	if err != nil {
		return err
	}
	d.Duration = duration
	return nil
}

// Implements flag.Value.String
func (d DurationFlag) String() string {
	return d.Duration.String()
}
