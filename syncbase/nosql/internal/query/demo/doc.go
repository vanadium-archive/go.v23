// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This demo program exercises the syncbase query language.
// The program's in-memory database implements the following
// interfaces in query_db:
//     Database
//     Table
//     KeyValueStream
//
// The user can enter the following at the command line:
//     1. dump - to get a dump of the database
//     2. a syncbase select statement - which is executed and results printed to stdout
//     3. exit (or empty line) - to exit the program
//
// To build example:
//     v23 go install v.io/syncbase/v23/syncbase/nosql/internal/query/demo
//
// To run example:
//     $V23_ROOT/roadmap/go/bin/demo
//
// Sample run:
//     > $V23_ROOT/roadmap/go/bin/demo
//     Enter query (or 'dump' or 'exit')? select v.Name, v.Address.State from Customer where t = "Customer"
//     John Smith,CA
//     Bat Masterson,IA
//     Enter query (or 'dump' or 'exit')? select v.CustID, v.InvoiceNum, v.ShipTo.Zip, v.Amount from Customer where t = "Invoice" and v.Amount > 100
//     2,1001,50055,166
//     2,1002,50055,243
//     2,1004,50055,787
//     Enter query (or 'dump' or 'exit')? select k, v fro Customer
//     select k, v fro Customer
//                 ^
//     Error: [Off:12] Expected 'from', found 'fro'
//     Enter query (or 'dump' or 'exit')? select k, v from Customer
//     001,{John Smith 1 true 65 {1 Main St. Palo Alto CA 94303}
//     001001,{1 1000 42 {1 Main St. Palo Alto CA 94303}}
//     001002,{1 1003 7 {2 Main St. Palo Alto CA 94303}}
//     001003,{1 1005 88 {3 Main St. Palo Alto CA 94303}}
//     002,{Bat Masterson 2 true 66 {777 Any St. Collins IA 50055}
//     002001,{2 1001 166 {777 Any St. collins IA 50055}}
//     002002,{2 1002 243 {888 Any St. collins IA 50055}}
//     002003,{2 1004 787 {999 Any St. collins IA 50055}}
//     002004,{2 1006 88 {101010 Any St. collins IA 50055}}
//     exit
//     >
package main
