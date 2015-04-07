// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package nosql

func Key(parts ...string) string {
	// TODO(kash): Join the parts together with a delimiter.
	// Do we need to ensure that the parts do not contain the delimiter?
	// Should the delimiter be '\0' or '/'?  If we decide that the
	// delimiter should be '/', we should probably just use naming.Join()
	// instead.
	return ""
}

func UUID() string {
	// TODO(kash): Implement me.
	return ""
}
