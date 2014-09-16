// This file contains utility functions and types for tests for the security package.

package security

import (
	"veyron2/naming"
)

type context struct {
	method, name, suffix string
	label                Label
	localID, remoteID    PublicID
}

func (c *context) Method() string                   { return c.method }
func (c *context) Name() string                     { return c.name }
func (c *context) Suffix() string                   { return c.suffix }
func (c *context) Label() Label                     { return c.label }
func (c *context) Discharges() map[string]Discharge { return nil }
func (c *context) LocalID() PublicID                { return c.localID }
func (c *context) RemoteID() PublicID               { return c.remoteID }
func (c *context) LocalEndpoint() naming.Endpoint   { return nil }
func (c *context) RemoteEndpoint() naming.Endpoint  { return nil }
