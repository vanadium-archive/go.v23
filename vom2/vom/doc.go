// This file was auto-generated via go generate.
// DO NOT UPDATE MANUALLY

/*
The vom tool helps debug the Veyron Object Marshaling (vom) protocol.

Usage:
   vom <command>

The vom commands are:
   decode      Decode data encoded in the vom format
   dump        Dump data encoded in the vom format into formatted output
   help        Display help for commands or topics
Run "vom help [command]" for command usage.

Vom Decode

Decode decodes data encoded in the vom format.  If no arguments are provided,
decode reads the data from stdin, otherwise the argument is the data.

By default the data is assumed to be represented in hex, with all whitespace
anywhere in the data ignored.  Use the -data flag to specify other data
representations.

Usage:
   vom decode [flags] [data]

[data] is the data to decode; if not specified, reads from stdin

The vom decode flags are:
 -data=Hex
   Data representation, one of [Hex Binary]

Vom Dump

Dump dumps data encoded in the vom format, generating formatted output
describing each portion of the encoding.  If no arguments are provided, dump
reads the data from stdin, otherwise the argument is the data.

By default the data is assumed to be represented in hex, with all whitespace
anywhere in the data ignored.  Use the -data flag to specify other data
representations.

Calling "vom dump" with no flags and no arguments combines the default stdin
mode with the default hex mode.  This default mode is special; certain non-hex
characters may be input to represent commands:
  . (period)    Calls Dumper.Status to get the current decoding status.
  ; (semicolon) Calls Dumper.Flush to flush output and start a new message.

This lets you cut-and-paste hex strings into your terminal, and use the commands
to trigger status or flush calls; i.e. a rudimentary debugging UI.

See v.io/core/veyron2/vom2.Dumper for details on the dump output.

Usage:
   vom dump [flags] [data]

[data] is the data to dump; if not specified, reads from stdin

The vom dump flags are:
 -data=Hex
   Data representation, one of [Hex Binary]

Vom Help

Help with no args displays the usage of the parent command.

Help with args displays the usage of the specified sub-command or help topic.

"help ..." recursively displays help for all commands and topics.

The output is formatted to a target width in runes.  The target width is
determined by checking the environment variable CMDLINE_WIDTH, falling back on
the terminal width from the OS, falling back on 80 chars.  By setting
CMDLINE_WIDTH=x, if x > 0 the width is x, if x < 0 the width is unlimited, and
if x == 0 or is unset one of the fallbacks is used.

Usage:
   vom help [flags] [command/topic ...]

[command/topic ...] optionally identifies a specific sub-command or help topic.

The vom help flags are:
 -style=text
   The formatting style for help output, either "text" or "godoc".
*/
package main
