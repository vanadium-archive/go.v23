// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file was auto-generated via go generate.
// DO NOT UPDATE MANUALLY

/*
Command vomtestgen generates test cases for the vomtest package.  The following
file is generated:

   data81_gen.vdl - Golden file containing test cases.

This tool does not run the vdl tool on the generated *.vdl files; you must do
that yourself, typically via "jiri go install".

Instead of running this tool manually, it is typically invoked via:

   $ jiri run go generate v.io/v23/vom/vomtest

Usage:
   vomtestgen [flags]

The vomtestgen flags are:
 -data81=data81_gen.vdl
   Name of the generated data file for version 81.

The global flags are:
 -metadata=<just specify -metadata to activate>
   Displays metadata for the program and exits.
 -time=false
   Dump timing information to stderr before exiting the program.
 -vdltest=
   Filter vdltest.All to only return entries that contain the given substring.
*/
package main
