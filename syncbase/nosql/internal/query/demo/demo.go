// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	isatty "github.com/mattn/go-isatty"

	"v.io/syncbase/v23/syncbase/nosql/internal/query"
	"v.io/syncbase/v23/syncbase/nosql/internal/query/demo/db"
	"v.io/syncbase/v23/syncbase/nosql/internal/query/demo/reader"
	"v.io/syncbase/v23/syncbase/nosql/internal/query/demo/writer"
	"v.io/syncbase/v23/syncbase/nosql/query_db"
	_ "v.io/x/ref/runtime/factories/generic"
)

var (
	format = flag.String("format", "table",
		"Output format.  'table': human-readable table; 'csv': comma-separated values, use -csv-delimiter to control the delimiter; 'json': JSON arrays.")
	csvDelimiter = flag.String("csv-delimiter", ",", "Delimiter to use when printing data as CSV (e.g. \"\t\", \",\")")
)

func dumpDB(d query_db.Database) {
	for _, table := range db.GetTableNames() {
		fmt.Printf("table: %s\n", table)
		queryExec(d, fmt.Sprintf("select k, v from %s", table))
	}
}

// Split an error message into an offset and the remaining (i.e., rhs of offset) message.
// The convention for syncql is "<module><optional-rpc>[offset}<remaining-message>".
func splitError(err error) (int64, string) {
	errMsg := err.Error()
	idx1 := strings.Index(errMsg, "[")
	idx2 := strings.Index(errMsg, "]")
	if idx1 == -1 || idx2 == -1 {
		return 0, errMsg
	}
	offsetString := errMsg[idx1+1 : idx2]
	offset, err := strconv.ParseInt(offsetString, 10, 64)
	if err != nil {
		return 0, errMsg
	}
	return offset, errMsg[idx2+1:]
}

func queryExec(d query_db.Database, q string) {
	if columnNames, rs, err := query.Exec(d, q); err != nil {
		off, msg := splitError(err)
		fmt.Fprintf(os.Stderr, "%s\n", q)
		fmt.Fprintf(os.Stderr, "%s^\n", strings.Repeat(" ", int(off)))
		fmt.Fprintf(os.Stderr, "Error: %s\n", msg)
	} else {
		if *format == "table" {
			if err := writer.WriteTable(os.Stdout, columnNames, rs); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %s\n", err.Error())
			}
		} else if *format == "csv" {
			if err := writer.WriteCSV(os.Stdout, columnNames, rs, *csvDelimiter); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %s\n", err.Error())
			}
		} else if *format == "json" {
			if err := writer.WriteJson(os.Stdout, columnNames, rs); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %s\n", err.Error())
			}
		} else {
			panic(fmt.Sprintf("invalid format flag value: %v", *format))
		}
	}
}

func main() {
	shutdown := db.InitDB()
	defer shutdown()

	if *format != "table" && *format != "csv" && *format != "json" {
		fmt.Fprintf(os.Stderr, "Unsupported -format %q.  Must be one of 'table', 'csv', or 'json'.\n", *format)
		os.Exit(-1)
	}

	var input *reader.T
	isTerminal := isatty.IsTerminal(os.Stdin.Fd())
	if isTerminal {
		input = reader.NewInteractive()
	} else {
		input = reader.NewNonInteractive()
	}
	defer input.Close()

	d := db.GetDatabase()
	for true {
		if q, err := input.GetQuery(); err != nil {
			if err == io.EOF {
				if isTerminal {
					// ctrl-d
					fmt.Println()
				}
				break
			} else {
				// ctrl-c
				break
			}
		} else {
			tq := strings.ToLower(strings.TrimSpace(q))
			if tq == "" || tq == "exit" || tq == "quit" {
				break
			}
			if tq == "dump" {
				dumpDB(d)
			} else {
				queryExec(d, q)
			}
		}
	}
}
