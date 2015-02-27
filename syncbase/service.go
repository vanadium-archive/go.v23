package syncbase

import (
	wire "v.io/syncbase/v23/services/syncbase"
	"v.io/v23/context"
	"v.io/v23/naming"
	"v.io/v23/services/security/access"
)

// TODO(sadovsky): Maybe put this elsewhere.
func BindService(name string) Service {
	return &service{wire.ServiceClient(name), name}
}

type service struct {
	c    wire.ServiceClientMethods
	name string
}

var _ Service = (*service)(nil)

// BindUniverse implements Service.BindUniverse.
func (s *service) BindUniverse(relativeName string) Universe {
	name := naming.Join(s.name, relativeName)
	return &universe{wire.UniverseClient(name), name, relativeName}
}

// ListUniverses implements Service.ListUniverses.
func (s *service) ListUniverses(ctx *context.T) ([]string, error) {
	// TODO(sadovsky): Implement on top of Glob.
	return nil, nil
}

// SetACL implements Service.SetACL.
func (s *service) SetACL(ctx *context.T, acl access.TaggedACLMap, etag string) error {
	return s.c.SetACL(ctx, acl, etag)
}

// GetACL implements Service.GetACL.
func (s *service) GetACL(ctx *context.T) (acl access.TaggedACLMap, etag string, err error) {
	return s.c.GetACL(ctx)
}
