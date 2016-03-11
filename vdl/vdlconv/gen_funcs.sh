#!/bin/bash
# Copyright 2016 The Vanadium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.


cat << EOF > vdlconv_funcs_test.go
// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vdlconv_test

import (
    "reflect"

    "v.io/v23/vdl/vdlconv"
)

var functions = []reflect.Value{
EOF

grep "func.*(" vdlconv.go | sed 's/func \([A-Za-z0-9]*\)(.*$/reflect.ValueOf(vdlconv.\1),/' >> vdlconv_funcs_test.go

echo "}" >> vdlconv_funcs_test.go

go fmt vdlconv_funcs_test.go