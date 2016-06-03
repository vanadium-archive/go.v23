// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package syncbase

import (
	"strings"

	"v.io/v23"
	"v.io/v23/context"
	"v.io/v23/discovery"
	"v.io/v23/security"
)

const visibilityKey = "vis"

// Discovery implements v.io/v23/discovery.T for syncbase based
// applications.
// TODO(mattr): Actually this is not syncbase specific.  At some
// point we should just replace the result of v23.NewDiscovery
// with this.
type Discovery struct {
	nhDiscovery discovery.T
	// TODO(mattr): Add global discovery.
}

// NewDiscovery creates a new syncbase discovery object.
func NewDiscovery(ctx *context.T) (discovery.T, error) {
	nhDiscovery, err := v23.NewDiscovery(ctx)
	if err != nil {
		return nil, err
	}
	return &Discovery{nhDiscovery: nhDiscovery}, nil
}

// Scan implements v.io/v23/discovery/T.Scan.
func (d *Discovery) Scan(ctx *context.T, query string) (<-chan discovery.Update, error) {
	nhUpdates, err := d.nhDiscovery.Scan(ctx, query)
	if err != nil {
		return nil, err
	}

	// Currently setting visibility on the neighborhood discovery
	// service turns IBE encryption on.  We currently don't have the
	// infrastructure support for IBE, so that would make our advertisements
	// unreadable by everyone.
	// Instead we add the visibility list to the attributes of the advertisement
	// and filter on the client side.  This is a temporary measure until
	// IBE is set up.  See v.io/i/1345.
	updates := make(chan discovery.Update)
	go func() {
		for u := range nhUpdates {
			patterns := splitPatterns(u.Attribute(visibilityKey))
			if len(patterns) > 0 && !matchesPatterns(ctx, patterns) {
				continue
			}
			updates <- update{u}
		}
		close(updates)
	}()

	return updates, nil
}

// Advertise implements v.io/v23/discovery/T.Advertise.
func (d *Discovery) Advertise(ctx *context.T, ad *discovery.Advertisement, visibility []security.BlessingPattern) (<-chan struct{}, error) {
	// Currently setting visibility on the neighborhood discovery
	// service turns IBE encryption on.  We currently don't have the
	// infrastructure support for IBE, so that would make our advertisements
	// unreadable by everyone.
	// Instead we add the visibility list to the attributes of the advertisement
	// and filter on the client side.  This is a temporary measure until
	// IBE is set up.  See v.io/i/1345.
	adCopy := *ad
	if len(visibility) > 0 {
		adCopy.Attributes = make(discovery.Attributes, len(ad.Attributes)+1)
		for k, v := range ad.Attributes {
			adCopy.Attributes[k] = v
		}
		patterns := joinPatterns(visibility)
		adCopy.Attributes[visibilityKey] = patterns
	}
	ch, err := d.nhDiscovery.Advertise(ctx, &adCopy, nil)
	ad.Id = adCopy.Id
	return ch, err
}

func matchesPatterns(ctx *context.T, patterns []security.BlessingPattern) bool {
	p := v23.GetPrincipal(ctx)
	blessings := p.BlessingStore().PeerBlessings()
	for _, b := range blessings {
		names := security.BlessingNames(p, b)
		for _, pattern := range patterns {
			if pattern.MatchedBy(names...) {
				return true
			}
		}
	}
	return false
}

// update wraps the discovery.Update to remove the visibility attribute which we add.
type update struct {
	discovery.Update
}

func (u update) Attribute(name string) string {
	if name == visibilityKey {
		return ""
	}
	return u.Update.Attribute(name)
}

func (u update) Advertisement() discovery.Advertisement {
	cp := u.Update.Advertisement()
	orig := cp.Attributes
	cp.Attributes = make(discovery.Attributes, len(orig))
	for k, v := range orig {
		if k != visibilityKey {
			cp.Attributes[k] = v
		}
	}
	return cp
}

// blessingSeparator is used to join multiple blessings into a
// single string.
// Note that comma cannot appear in blessings, see:
// v.io/v23/security/certificate.go
const blessingsSeparator = ','

// joinPatterns concatenates the elements of a to create a single string.
// The string can be split again with SplitPatterns.
func joinPatterns(a []security.BlessingPattern) string {
	if len(a) == 0 {
		return ""
	}
	if len(a) == 1 {
		return string(a[0])
	}
	n := (len(a) - 1)
	for i := 0; i < len(a); i++ {
		n += len(a[i])
	}

	b := make([]byte, n)
	bp := copy(b, a[0])
	for _, s := range a[1:] {
		b[bp] = blessingsSeparator
		bp++
		bp += copy(b[bp:], s)
	}
	return string(b)
}

// splitPatterns splits BlessingPatterns that were joined with
// JoinBlessingPattern.
func splitPatterns(patterns string) []security.BlessingPattern {
	if patterns == "" {
		return nil
	}
	n := strings.Count(patterns, string(blessingsSeparator)) + 1
	out := make([]security.BlessingPattern, n)
	last, start := 0, 0
	for i, r := range patterns {
		if r == blessingsSeparator {
			out[last] = security.BlessingPattern(patterns[start:i])
			last++
			start = i + 1
		}
	}
	out[last] = security.BlessingPattern(patterns[start:])
	return out
}
