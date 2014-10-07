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
Run "vdl help [topic]" for topic details.

The vdl flags are:
   -experimental=false: Enable experimental features that may crash the compiler and change without notice.  Intended for VDL compiler developers.
   -exts=.vdl: Comma-separated list of valid VDL file name extensions.
   -max_errors=-1: Stop processing after this many errors, or -1 for unlimited.
   -v=false: Turn on verbose logging.

Vdl Generate

Generate compiles packages and their transitive dependencies, and generates code
in the specified languages.

Usage:
   vdl generate [flags] <packages>

<packages> are a list of packages to process, similar to the standard go tool.
For more information, run "vdl help packages".

The generate flags are:
   -go_fmt=true: Format generated Go code
   -go_out_dir=: Go output directory.  There are three modes:
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
      When the src->dst form is used, src must match the suffix of the path
      just before the package path, and dst is the replacement for src.
      Use commas to separate multiple rules; the first rule matching src is
      used.  The special dst SKIP indicates matching packages are skipped.
   -java_out_dir=veyron/go/src->veyron/java/src/vdl/java,roadmap/go/src->veyron/java/src/vdl/java,third_party/go/src->SKIP,tools/go/src->SKIP: Same semantics as --go_out_dir but applies to java code generation.
   -java_out_pkg=veyron.io/veyron/veyron2/vom2/testdata->SKIP,veyron.io->io/veyron: Java output package translation rules.  Must be of the form:
         "src->dst[,s2->d2...]"
      If a VDL package has a prefix src, the prefix will be replaced with dst.
      Use commas to separate multiple rules; the first rule matching src is
      used, and if there are no matching rules, the package remains unchanged.
      The special dst SKIP indicates matching packages are skipped.
   -js_out_dir=veyron/go/src->veyron.js/src,roadmap/go/src->veyron.js/src,third_party/go/src->SKIP,tools/go/src->SKIP: Same semantics as --go_out_dir but applies to js code generation.
   -lang=go,java: Comma-separated list of languages to generate, currently supporting go,java,js
   -status=true: Show package names as they are updated

Vdl Compile

Compile compiles packages and their transitive dependencies, but does not
generate code.  This is useful to sanity-check that your VDL files are valid.

Usage:
   vdl compile [flags] <packages>

<packages> are a list of packages to process, similar to the standard go tool.
For more information, run "vdl help packages".

The compile flags are:
   -status=true: Show package names while we compile

Vdl Audit

Audit runs the same logic as generate, but doesn't write out generated files.
Returns a 0 exit code if all packages are up-to-date, otherwise returns a
non-0 exit code indicating some packages need generation.

Usage:
   vdl audit [flags] <packages>

<packages> are a list of packages to process, similar to the standard go tool.
For more information, run "vdl help packages".

The audit flags are:
   -go_fmt=true: Format generated Go code
   -go_out_dir=: Go output directory.  There are three modes:
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
      When the src->dst form is used, src must match the suffix of the path
      just before the package path, and dst is the replacement for src.
      Use commas to separate multiple rules; the first rule matching src is
      used.  The special dst SKIP indicates matching packages are skipped.
   -java_out_dir=veyron/go/src->veyron/java/src/vdl/java,roadmap/go/src->veyron/java/src/vdl/java,third_party/go/src->SKIP,tools/go/src->SKIP: Same semantics as --go_out_dir but applies to java code generation.
   -java_out_pkg=veyron.io/veyron/veyron2/vom2/testdata->SKIP,veyron.io->io/veyron: Java output package translation rules.  Must be of the form:
         "src->dst[,s2->d2...]"
      If a VDL package has a prefix src, the prefix will be replaced with dst.
      Use commas to separate multiple rules; the first rule matching src is
      used, and if there are no matching rules, the package remains unchanged.
      The special dst SKIP indicates matching packages are skipped.
   -js_out_dir=veyron/go/src->veyron.js/src,roadmap/go/src->veyron.js/src,third_party/go/src->SKIP,tools/go/src->SKIP: Same semantics as --go_out_dir but applies to js code generation.
   -lang=go,java: Comma-separated list of languages to generate, currently supporting go,java,js
   -status=true: Show package names as they are updated

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

Usage:
   vdl help [flags] [command/topic ...]

[command/topic ...] optionally identifies a specific sub-command or help topic.

The help flags are:
   -style=text: The formatting style for help output, either "text" or "godoc".

Vdl Packages - help topic

Most vdl commands apply to a list of packages:

  vdl command <packages>

<packages> are a list of packages to process, similar to the standard go tool.
In its simplest form each package is an import path; e.g.
   "veyron.io/veyron/veyron/lib/vdl"

A package that is an absolute path or that contains a "." is interpreted as a
file system path and denotes the package in that directory.

A package that ends with "..." does a wildcard match against all directories
with that prefix.

The special import path "all" expands to all package directories found in all
the VDLPATH trees.

For more information, run "go help packages" to see the standard go package
documentation.

Vdl Vdlpath - help topic

The VDLPATH environment variable is used to resolve import statements.
It must be set to compile and generate vdl packages.

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
*/
package main
