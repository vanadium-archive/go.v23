package security

import (
	"fmt"
	"time"

	"v.io/veyron/veyron2/naming"
)

// NewContext creates a Context.
func NewContext(params *ContextParams) Context {
	ctx := &context{*params}
	if params.Timestamp.IsZero() {
		ctx.params.Timestamp = time.Now()
	}
	return ctx
}

// ContextParams is used to create Context objects using the NewContext
// function.
type ContextParams struct {
	Timestamp  time.Time     // Time at which the authorization is to be checked.
	Method     string        // Method being invoked.
	MethodTags []interface{} // Tags attached to the method, typically through interface specification in VDL
	Suffix     string        // Object suffix on which the method is being invoked.

	LocalPrincipal Principal       // Principal at the local end of a request.
	LocalBlessings Blessings       // Blessings presented to the remote end.
	LocalEndpoint  naming.Endpoint // Endpoint of local end of communication.

	RemoteBlessings  Blessings            // Blessings presented by the remote end.
	RemoteDischarges map[string]Discharge // Map a ThirdPartyCaveat identifier to corresponding discharges shared by the remote end.
	RemoteEndpoint   naming.Endpoint      // Endpoint of the remote end of communication
}

// Copy fills in p with a copy of the values in c.
func (p *ContextParams) Copy(c Context) {
	p.Timestamp = c.Timestamp()
	p.Method = c.Method()
	p.MethodTags = make([]interface{}, len(c.MethodTags()))
	for ix, tag := range c.MethodTags() {
		p.MethodTags[ix] = tag
	}
	p.Suffix = c.Suffix()
	p.LocalPrincipal = c.LocalPrincipal()
	p.LocalBlessings = c.LocalBlessings()
	p.LocalEndpoint = c.LocalEndpoint()
	p.RemoteBlessings = c.RemoteBlessings()
	p.RemoteDischarges = make(map[string]Discharge, len(c.RemoteDischarges()))
	for id, dis := range c.RemoteDischarges() {
		p.RemoteDischarges[id] = dis
	}
	p.RemoteEndpoint = c.RemoteEndpoint()
}

type context struct{ params ContextParams }

func (c *context) Timestamp() time.Time                   { return c.params.Timestamp }
func (c *context) Method() string                         { return c.params.Method }
func (c *context) MethodTags() []interface{}              { return c.params.MethodTags }
func (c *context) Name() string                           { return c.params.Suffix }
func (c *context) Suffix() string                         { return c.params.Suffix }
func (c *context) LocalPrincipal() Principal              { return c.params.LocalPrincipal }
func (c *context) LocalBlessings() Blessings              { return c.params.LocalBlessings }
func (c *context) RemoteBlessings() Blessings             { return c.params.RemoteBlessings }
func (c *context) LocalEndpoint() naming.Endpoint         { return c.params.LocalEndpoint }
func (c *context) RemoteEndpoint() naming.Endpoint        { return c.params.RemoteEndpoint }
func (c *context) RemoteDischarges() map[string]Discharge { return c.params.RemoteDischarges }
func (c *context) String() string                         { return fmt.Sprintf("%+v", c.params) }
