// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vdltool

import (
	"reflect"
	"testing"

	"v.io/v23/vdl"
)

// The Config's MakeVdlTarget is used to decode values in the vdl file.
// Ensure that it decodes the values correctly. If this is not the case,
// the vdl generator behaves incorrectly.
func TestConfigMakeVdlTarget(t *testing.T) {
	config := Config{
		Go: GoConfig{
			WireToNativeTypes: map[string]GoType{
				"WireString":       {Type: "string"},
				"WireMapStringInt": {Type: "map[string]int"},
				"WireTime": {
					Type:    "time.Time",
					Imports: []GoImport{{Path: "time", Name: "time"}},
				},
				"WireSamePkg": {
					Type:    "nativetest.NativeSamePkg",
					Imports: []GoImport{{Path: "v.io/v23/vdl/testdata/nativetest", Name: "nativetest"}},
				},
				"WireMultiImport": {
					Type: "map[nativetest.NativeSamePkg]time.Time",
					Imports: []GoImport{
						{Path: "v.io/v23/vdl/testdata/nativetest", Name: "nativetest"},
						{Path: "time", Name: "time"},
					},
				},
			},
		},
		Java: JavaConfig{
			WireTypeRenames: map[string]string{
				"WireRenameMe": "WireRenamed",
			},

			WireToNativeTypes: map[string]string{
				"WireString":       "java.lang.String",
				"WireMapStringInt": "java.util.Map<java.lang.String, java.lang.Integer>",
				"WireTime":         "org.joda.time.DateTime",
				"WireSamePkg":      "io.v.v23.vdl.testdata.nativetest.NativeSamePkg",
				"WireMultiImport":  "java.util.Map<io.v.v23.vdl.testdata.nativetest.NativeSamePkg, org.joda.time.DateTime>",
				"WireRenamed":      "java.lang.Long",
			},
		},
	}

	configVdlValue := vdl.ValueOf(config)

	var finalConfig Config
	if err := vdl.FromValue(finalConfig.MakeVDLTarget(), configVdlValue); err != nil {
		t.Fatalf("error while using ConfigTarget: %v", err)
	}

	if got, want := finalConfig, config; !reflect.DeepEqual(got, want) {
		t.Errorf("\n got: %#v\nwant: %#v", got, want)
	}
}
