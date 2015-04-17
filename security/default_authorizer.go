// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package security

import "v.io/v23/context"

// DefaultAuthorizer returns an Authorizer implementation that should be used
// when it doubt. It has the conservative policy that requires one end of the
// RPC to have a blessing that is extended from the blessing presented by the
// other end.
func DefaultAuthorizer() Authorizer {
	return defaultAuthorizer{}
}

type defaultAuthorizer struct{}

func (defaultAuthorizer) Authorize(ctx *context.T, call Call) error {
	var (
		localNames             = LocalBlessingNames(ctx, call)
		remoteNames, remoteErr = RemoteBlessingNames(ctx, call)
	)
	// Authorize if any element in localNames is a "delegate of" (i.e., has been
	// blessed by) any element in remoteNames, OR vice-versa.
	for _, l := range localNames {
		if BlessingPattern(l).MatchedBy(remoteNames...) {
			// One of remoteNames is an extension of l.
			return nil
		}
	}
	for _, r := range remoteNames {
		if BlessingPattern(r).MatchedBy(localNames...) {
			// One of localNames is an extension of r.
			return nil
		}
	}

	return NewErrAuthorizationFailed(ctx, remoteNames, remoteErr, localNames)
}
