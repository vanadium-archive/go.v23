package security

import (
	"fmt"
	"time"

	"v.io/v23/context"
	"v.io/v23/naming"
	"v.io/v23/vdl"
)

// NewCall creates a Call.
func NewCall(params *CallParams) Call {
	ctx := &ctxImpl{*params}
	if params.Timestamp.IsZero() {
		ctx.params.Timestamp = time.Now()
	}
	return ctx
}

// CallParams is used to create Call objects using the NewCall
// function.
type CallParams struct {
	Timestamp  time.Time    // Time at which the authorization is to be checked.
	Method     string       // Method being invoked.
	MethodTags []*vdl.Value // Method tags, typically specified in VDL.
	Suffix     string       // Object suffix on which the method is being invoked.

	LocalPrincipal Principal       // Principal at the local end of a request.
	LocalBlessings Blessings       // Blessings presented to the remote end.
	LocalEndpoint  naming.Endpoint // Endpoint of local end of communication.

	RemoteBlessings  Blessings            // Blessings presented by the remote end.
	RemoteDischarges map[string]Discharge // Map a ThirdPartyCaveat identifier to corresponding discharges shared by the remote end.
	RemoteEndpoint   naming.Endpoint      // Endpoint of the remote end of communication

	Context *context.T // The context.T.
}

// Copy fills in p with a copy of the values in c.
func (p *CallParams) Copy(c Call) {
	p.Timestamp = c.Timestamp()
	p.Method = c.Method()
	if tagslen := len(c.MethodTags()); tagslen == 0 {
		p.MethodTags = nil
	} else {
		p.MethodTags = make([]*vdl.Value, tagslen)
		for ix, tag := range c.MethodTags() {
			p.MethodTags[ix] = vdl.CopyValue(tag)
		}
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
	p.Context = c.Context()
}

type ctxImpl struct{ params CallParams }

func (c *ctxImpl) Timestamp() time.Time                   { return c.params.Timestamp }
func (c *ctxImpl) Method() string                         { return c.params.Method }
func (c *ctxImpl) MethodTags() []*vdl.Value               { return c.params.MethodTags }
func (c *ctxImpl) Name() string                           { return c.params.Suffix }
func (c *ctxImpl) Suffix() string                         { return c.params.Suffix }
func (c *ctxImpl) LocalPrincipal() Principal              { return c.params.LocalPrincipal }
func (c *ctxImpl) LocalBlessings() Blessings              { return c.params.LocalBlessings }
func (c *ctxImpl) RemoteBlessings() Blessings             { return c.params.RemoteBlessings }
func (c *ctxImpl) LocalEndpoint() naming.Endpoint         { return c.params.LocalEndpoint }
func (c *ctxImpl) RemoteEndpoint() naming.Endpoint        { return c.params.RemoteEndpoint }
func (c *ctxImpl) RemoteDischarges() map[string]Discharge { return c.params.RemoteDischarges }
func (c *ctxImpl) String() string                         { return fmt.Sprintf("%+v", c.params) }
func (c *ctxImpl) Context() *context.T                    { return c.params.Context }
