// This file was auto-generated via go generate.
// DO NOT UPDATE MANUALLY

/*
The vdl tool manages veyron VDL source code.  It's similar to the go tool used
for managing Go source code.

Usage:
   vdl [flags] <command>

The vdl commands are:
   generate    Compile packages and dependencies, and generate code
   compile     Compile packages and dependencies, but don't generate code
   audit       Check if any packages are stale and need generation
   list        List package and dependency info in transitive order
   help        Display help for commands or topics
Run "vdl help [command]" for command usage.

The vdl additional help topics are:
   packages    Description of package lists
   vdlpath     Description of VDLPATH environment variable
   vdlroot     Description of VDLROOT environment variable
   vdl.config  Description of vdl.config files
Run "vdl help [topic]" for topic details.

The vdl flags are:
 -exts=.vdl
   Comma-separated list of valid VDL file name extensions.
 -ignore_unknown=false
   Ignore unknown packages provided on the command line.
 -max_errors=-1
   Stop processing after this many errors, or -1 for unlimited.
 -v=false
   Turn on verbose logging.
 -vdl.config=vdl.config
   Basename of the optional per-package config file.

Vdl Generate

Generate compiles packages and their transitive dependencies, and generates code
in the specified languages.

Usage:
   vdl generate [flags] <packages>

<packages> are a list of packages to process, similar to the standard go tool.
For more information, run "vdl help packages".

The vdl generate flags are:
 -go_out_dir=
   Go output directory.  There are three modes:
      ""                     : Generate output in-place in the source tree
      "dir"                  : Generate output rooted at dir
      "src->dst[,s2->d2...]" : Generate output using translation rules
   Assume your source tree is organized as follows:
      VDLPATH=/home/vdl
         /home/vdl/src/veyron/test_base/base1.vdl
         /home/vdl/src/veyron/test_base/base2.vdl
   Here's example output under the different modes:
      --go_out_dir=""
         /home/vdl/src/veyron/test_base/base1.vdl.go
         /home/vdl/src/veyron/test_base/base2.vdl.go
      --go_out_dir="/tmp/foo"
         /tmp/foo/veyron/test_base/base1.vdl.go
         /tmp/foo/veyron/test_base/base2.vdl.go
      --go_out_dir="vdl/src->foo/bar/src"
         /home/foo/bar/src/veyron/test_base/base1.vdl.go
         /home/foo/bar/src/veyron/test_base/base2.vdl.go
   When the src->dst form is used, src must match the suffix of the path just
   before the package path, and dst is the replacement for src.  Use commas to
   separate multiple rules; the first rule matching src is used.  The special
   dst SKIP indicates matching packages are skipped.
 -java_out_dir=go/src->java/src/vdl/java,release/go/src/v.io/core/veyron2/vdl/vdlroot/src->SKIP
   Same semantics as --go_out_dir but applies to java code generation.
 -java_out_pkg=v.io->io/v
   Java output package translation rules.  Must be of the form:
      "src->dst[,s2->d2...]"
   If a VDL package has a prefix src, the prefix will be replaced with dst.  Use
   commas to separate multiple rules; the first rule matching src is used, and
   if there are no matching rules, the package remains unchanged.  The special
   dst SKIP indicates matching packages are skipped.
 -js_out_dir=release/go/src->release/javascript/core/src,roadmap/go/src->release/javascript/core/src,third_party/go/src->SKIP,tools/go/src->SKIP,release/go/src/v.io/core/veyron2/vdl/vdlroot/src->SKIP
   Same semantics as --go_out_dir but applies to js code generation.
 -lang=Go,Java
   Comma-separated list of languages to generate, currently supporting
   Go,Java,Javascript
 -status=true
   Show package names as they are updated

Vdl Compile

Compile compiles packages and their transitive dependencies, but does not
generate code.  This is useful to sanity-check that your VDL files are valid.

Usage:
   vdl compile [flags] <packages>

<packages> are a list of packages to process, similar to the standard go tool.
For more information, run "vdl help packages".

The vdl compile flags are:
 -status=true
   Show package names while we compile

Vdl Audit

Audit runs the same logic as generate, but doesn't write out generated files.
Returns a 0 exit code if all packages are up-to-date, otherwise returns a non-0
exit code indicating some packages need generation.

Usage:
   vdl audit [flags] <packages>

<packages> are a list of packages to process, similar to the standard go tool.
For more information, run "vdl help packages".

The vdl audit flags are:
 -go_out_dir=
   Go output directory.  There are three modes:
      ""                     : Generate output in-place in the source tree
      "dir"                  : Generate output rooted at dir
      "src->dst[,s2->d2...]" : Generate output using translation rules
   Assume your source tree is organized as follows:
      VDLPATH=/home/vdl
         /home/vdl/src/veyron/test_base/base1.vdl
         /home/vdl/src/veyron/test_base/base2.vdl
   Here's example output under the different modes:
      --go_out_dir=""
         /home/vdl/src/veyron/test_base/base1.vdl.go
         /home/vdl/src/veyron/test_base/base2.vdl.go
      --go_out_dir="/tmp/foo"
         /tmp/foo/veyron/test_base/base1.vdl.go
         /tmp/foo/veyron/test_base/base2.vdl.go
      --go_out_dir="vdl/src->foo/bar/src"
         /home/foo/bar/src/veyron/test_base/base1.vdl.go
         /home/foo/bar/src/veyron/test_base/base2.vdl.go
   When the src->dst form is used, src must match the suffix of the path just
   before the package path, and dst is the replacement for src.  Use commas to
   separate multiple rules; the first rule matching src is used.  The special
   dst SKIP indicates matching packages are skipped.
 -java_out_dir=go/src->java/src/vdl/java,release/go/src/v.io/core/veyron2/vdl/vdlroot/src->SKIP
   Same semantics as --go_out_dir but applies to java code generation.
 -java_out_pkg=v.io->io/v
   Java output package translation rules.  Must be of the form:
      "src->dst[,s2->d2...]"
   If a VDL package has a prefix src, the prefix will be replaced with dst.  Use
   commas to separate multiple rules; the first rule matching src is used, and
   if there are no matching rules, the package remains unchanged.  The special
   dst SKIP indicates matching packages are skipped.
 -js_out_dir=release/go/src->release/javascript/core/src,roadmap/go/src->release/javascript/core/src,third_party/go/src->SKIP,tools/go/src->SKIP,release/go/src/v.io/core/veyron2/vdl/vdlroot/src->SKIP
   Same semantics as --go_out_dir but applies to js code generation.
 -lang=Go,Java
   Comma-separated list of languages to generate, currently supporting
   Go,Java,Javascript
 -status=true
   Show package names as they are updated

Vdl List

List returns information about packages and their transitive dependencies, in
transitive order.  This is the same order the generate and compile commands use
for processing.  If "vdl list A" is run and A depends on B, which depends on C,
the returned order will be C, B, A.  If multiple packages are specified the
ordering is over all combined dependencies.

Reminder: cyclic dependencies between packages are not allowed.  Cyclic
dependencies between VDL files within the same package are also not allowed.
This is more strict than regular Go; it makes it easier to generate code for
other languages like C++.

Usage:
   vdl list <packages>

<packages> are a list of packages to process, similar to the standard go tool.
For more information, run "vdl help packages".

Vdl Help

Help with no args displays the usage of the parent command.

Help with args displays the usage of the specified sub-command or help topic.

"help ..." recursively displays help for all commands and topics.

The output is formatted to a target width in runes.  The target width is
determined by checking the environment variable CMDLINE_WIDTH, falling back on
the terminal width from the OS, falling back on 80 chars.  By setting
CMDLINE_WIDTH=x, if x > 0 the width is x, if x < 0 the width is unlimited, and
if x == 0 or is unset one of the fallbacks is used.

Usage:
   vdl help [flags] [command/topic ...]

[command/topic ...] optionally identifies a specific sub-command or help topic.

The vdl help flags are:
 -style=text
   The formatting style for help output, either "text" or "godoc".

Vdl Packages - help topic

Most vdl commands apply to a list of packages:

   vdl command <packages>

<packages> are a list of packages to process, similar to the standard go tool.
In its simplest form each package is an import path; e.g.
   "v.io/core/veyron/lib/vdl"

A package that is an absolute path or that begins with a . or .. element is
interpreted as a file system path, and denotes the package in that directory.

A package is a pattern if it includes one or more "..." wildcards, each of which
can match any string, including the empty string and strings containing slashes.
Such a pattern expands to all packages found in VDLPATH with names matching the
pattern.  As a special-case, x/... matches x as well as x's subdirectories.

The special-case "all" is a synonym for "...", and denotes all packages found in
VDLPATH.

Import path elements and file names are not allowed to begin with "." or "_";
such paths are ignored in wildcard matches, and return errors if specified
explicitly.

 Run "vdl help vdlpath" to see docs on VDLPATH.
 Run "go help packages" to see the standard go package docs.

Vdl Vdlpath - help topic

The VDLPATH environment variable is used to resolve import statements. It must
be set to compile and generate vdl packages.

The format is a colon-separated list of directories, where each directory must
have a "src/" directory that holds vdl source code.  The path below 'src'
determines the import path.  If VDLPATH specifies multiple directories, imports
are resolved by picking the first directory with a matching import name.

An example:

   VDPATH=/home/user/vdlA:/home/user/vdlB

   /home/user/vdlA/
      src/
         foo/                 (import "foo" refers here)
            foo1.vdl
   /home/user/vdlB/
      src/
         foo/                 (this package is ignored)
            foo2.vdl
         bar/
            baz/              (import "bar/baz" refers here)
               baz.vdl

Vdl Vdlroot - help topic

The VDLROOT environment variable is similar to VDLPATH, but instead of pointing
to multiple user source directories, it points at a single source directory
containing the standard vdl packages.

Setting VDLROOT is optional.

If VDLROOT is empty, we try to construct it out of the VANADIUM_ROOT environment
variable.  It is an error if both VDLROOT and VANADIUM_ROOT are empty.

Vdl Vdl.Config - help topic

Each vdl source package P may contain an optional file "vdl.config" within the P
directory.  This file specifies additional configuration for the vdl tool.

The format of this file is described by the vdl.Config type in the "vdl"
standard package, located at VDLROOT/src/vdl/config.go.

If the file does not exist, we use the zero value of vdl.Config.
*/
package main
