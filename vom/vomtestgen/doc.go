// This file was auto-generated via go generate.
// DO NOT UPDATE MANUALLY

/*
The vomtestgen tool generates vom test data, using the vomdata file as input,
and creating a vdl file as output.

Usage:
   vomtestgen [flags] [vomdata]

[vomdata] is the path to the vomdata input file, specified in the vdl config
file format.  It must be of the form "NAME.vdl.config", and the output vdl file
will be generated at "NAME.vdl".

The config file should export a const []any that contains all of the values that
will be tested.  Here's an example:
   config = []any{
     bool(true), uint64(123), string("abc"),
   }

If not specified, we'll try to find the file at its canonical location:
   v.io/core/veyron2/vom/testdata/vomdata.vdl.config

The vomtestgen flags are:
 -exts=.vdl
   Comma-separated list of valid VDL file name extensions.
 -max_errors=-1
   Stop processing after this many errors, or -1 for unlimited.
*/
package main
