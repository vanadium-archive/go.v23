package syncbase

import (
	wire "v.io/syncbase/v23/services/syncbase"
	"v.io/v23/context"
	"v.io/v23/vdl"
)

type item struct {
	c    wire.ItemClientMethods
	name string
	key  *vdl.Value
}

var _ Item = (*item)(nil)

// Key implements Item.Key.
func (i *item) Key() *vdl.Value {
	return i.key
}

// Get implements Item.Get.
func (i *item) Get(ctx *context.T) (*vdl.Value, error) {
	return i.c.Get(ctx)
}

// Put implements Item.Put.
func (i *item) Put(ctx *context.T, value *vdl.Value) error {
	return i.c.Put(ctx, value)
}

// Delete implements Item.Delete.
func (i *item) Delete(ctx *context.T) error {
	return i.c.Delete(ctx)
}
