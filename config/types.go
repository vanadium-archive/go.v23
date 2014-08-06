package config

import (
	"fmt"
	"net"
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
