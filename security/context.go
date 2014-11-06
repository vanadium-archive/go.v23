package security

import (
	"fmt"
	"time"
	"veyron.io/veyron/veyron2/naming"
)

// NewContext creates a Context.
func NewContext(params *ContextParams) Context {
	return &context{*params}
}

// ContextParams is used to create Context objects using the NewContext
// function.
type ContextParams struct {
	Timestamp  time.Time     // Time at which the authorization is to be checked.
	Method     string        // Method being invoked.
	MethodTags []interface{} // Tags attached to the method, typically through interface specification in VDL
	Name       string        // Object name on which the method is being invoked.

	LocalPrincipal Principal       // Principal at the local end of a request.
	LocalBlessings Blessings       // Blessings presented to the remote end.
	LocalEndpoint  naming.Endpoint // Endpoint of local end of communication.

	RemoteBlessings  Blessings            // Blessings presented by the remote end.
	RemoteDischarges map[string]Discharge // Map a ThirdPartyCaveat identifier to corresponding discharges shared by the remote end.
	RemoteEndpoint   naming.Endpoint      // Endpoint of the remote end of communication
}

type context struct{ params ContextParams }

func (c *context) Timestamp() time.Time      { return c.params.Timestamp }
func (c *context) Method() string            { return c.params.Method }
func (c *context) MethodTags() []interface{} { return c.params.MethodTags }
func (c *context) Name() string              { return c.params.Name }
func (c *context) Suffix() string            { return c.params.Name }
func (c *context) Label() Label {
	for _, tag := range c.params.MethodTags {
		if l, ok := tag.(Label); ok {
			return l
		}
	}
	return AdminLabel
}
func (c *context) LocalPrincipal() Principal        { return c.params.LocalPrincipal }
func (c *context) LocalBlessings() Blessings        { return c.params.LocalBlessings }
func (c *context) RemoteBlessings() Blessings       { return c.params.RemoteBlessings }
func (c *context) LocalEndpoint() naming.Endpoint   { return c.params.LocalEndpoint }
func (c *context) RemoteEndpoint() naming.Endpoint  { return c.params.RemoteEndpoint }
func (c *context) Discharges() map[string]Discharge { return c.params.RemoteDischarges }
func (c *context) String() string                   { return fmt.Sprintf("%+v", c.params) }
