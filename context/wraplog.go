// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package context

import "fmt"

func (t *T) Info(args ...interface{}) {
	t.logger.InfoDepth(1, args...)
}
func (t *T) InfoDepth(depth int, args ...interface{}) {
	t.logger.InfoDepth(depth+1, args...)
}
func (t *T) Infof(format string, args ...interface{}) {
	t.logger.InfoDepth(1, fmt.Sprintf(format, args...))
}
func (t *T) InfoStack(all bool) { t.logger.InfoStack(all) }

func (t *T) Error(args ...interface{}) {
	t.logger.ErrorDepth(1, args...)
}
func (t *T) ErrorDepth(depth int, args ...interface{}) {
	t.logger.ErrorDepth(depth+1, args...)
}
func (t *T) Errorf(format string, args ...interface{}) {
	t.logger.ErrorDepth(1, fmt.Sprintf(format, args...))
}

func (t *T) Fatal(args ...interface{}) {
	t.logger.FatalDepth(1, args...)
}
func (t *T) FatalDepth(depth int, args ...interface{}) {
	t.logger.FatalDepth(depth+1, args...)
}
func (t *T) Fatalf(format string, args ...interface{}) {
	t.logger.FatalDepth(1, fmt.Sprintf(format, args...))
}

func (t *T) Panic(args ...interface{}) {
	t.logger.PanicDepth(1, args...)
}

func (t *T) PanicDepth(depth int, args ...interface{}) {
	t.logger.PanicDepth(depth+1, args...)
}

func (t *T) Panicf(format string, args ...interface{}) {
	t.logger.PanicDepth(1, fmt.Sprintf(format, args...))
}

func (t *T) V(level int) bool                 { return t.logger.VDepth(1, level) }
func (t *T) VDepth(depth int, level int) bool { return t.logger.VDepth(depth+1, level) }

func (t *T) VI(level int) interface {
	Info(args ...interface{})
	Infof(format string, args ...interface{})
	InfoDepth(depth int, args ...interface{})
	InfoStack(all bool)
} {
	return t.logger.VIDepth(1, level)
}
func (t *T) VIDepth(depth int, level int) interface {
	Info(args ...interface{})
	Infof(format string, args ...interface{})
	InfoDepth(depth int, args ...interface{})
	InfoStack(all bool)
} {
	return t.logger.VIDepth(depth+1, level)
}

func (t *T) FlushLog() { t.logger.FlushLog() }
