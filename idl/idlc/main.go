// Below is the output from $(idlc help -style=godoc ...)

/*
The idlc tool manages veyron IDL source code.  It's similar to the go tool used
for managing Go source code.

Usage:
   idlc [flags] <command>

The idlc commands are:
   generate    Compile packages and dependencies, and generate code
   compile     Compile packages and dependencies, but don't generate code
   listinfo    List package and dependency info in transitive order
   help        Display help for commands

The idlc flags are:
   -max_errors=-1: Stop processing after this many errors, or -1 for unlimited.
   -v=false: Turn on verbose logging.

Idlc Generate

Generate compiles packages and their transitive dependencies, and generates code
in the specified languages.

Usage:
   idlc generate [flags] <packages>

<packages> are a list of packages to process, specified as arguments for each
command.  The format is similar to the go tool.  In its simplest form each
package is an import path; e.g. "veyron/lib/idl".  A package that is an absolute
path or that contains a "." is interpreted as a file system path and denotes the
package in that directory.  A package that ends with "..." does a wildcard match
against all directories with that prefix.  The special import path "all" expands
to all package directories found in all the GOPATH trees.

For more information use "go help packages" to see the standard go package
documentation.

The generate flags are:
   -go_fmt=true: Format generated Go code
   -lang=[go]: Comma-separated list of languages to generate, currently supporting "go"
   -status=true: Show package names while we compile

Idlc Compile

Compile compiles packages and their transitive dependencies, but does not
generate code.  This is useful to sanity-check that your IDL files are valid.

Usage:
   idlc compile [flags] <packages>

<packages> are a list of packages to process, specified as arguments for each
command.  The format is similar to the go tool.  In its simplest form each
package is an import path; e.g. "veyron/lib/idl".  A package that is an absolute
path or that contains a "." is interpreted as a file system path and denotes the
package in that directory.  A package that ends with "..." does a wildcard match
against all directories with that prefix.  The special import path "all" expands
to all package directories found in all the GOPATH trees.

For more information use "go help packages" to see the standard go package
documentation.

The compile flags are:
   -status=true: Show package names while we compile

Idlc Listinfo

Listinfo returns information about packages and their transitive dependencies,
in transitive order.  This is the same order the generate and compile commands
use for processing.  If "idlc listinfo A" is run and A depends on B, which
depends on C, the returned order will be C, B, A.  If multiple packages are
specified the ordering is over all combined dependencies.

Reminder: cyclic dependencies between packages are not allowed.  Cyclic
dependencies between IDL files within the same package are also not allowed.
This is more strict than regular Go; it makes it easier to generate code for
other languages like C++.

Usage:
   idlc listinfo <packages>

<packages> are a list of packages to process, specified as arguments for each
command.  The format is similar to the go tool.  In its simplest form each
package is an import path; e.g. "veyron/lib/idl".  A package that is an absolute
path or that contains a "." is interpreted as a file system path and denotes the
package in that directory.  A package that ends with "..." does a wildcard match
against all directories with that prefix.  The special import path "all" expands
to all package directories found in all the GOPATH trees.

For more information use "go help packages" to see the standard go package
documentation.

Idlc Help

Help displays usage descriptions for this command, or usage descriptions for
sub-commands.

Usage:
   idlc help [flags] <command>

<command> is an optional sequence of commands to display detailed per-command
usage.  The special-case "help ..." recursively displays help for this command
and all sub-commands.

The help flags are:
   -style=text: The formatting style for help output, either "text" or "godoc".
*/
package main

import "veyron2/idl/idlc/cmds"

func main() {
	cmds.Root().Main()
}
