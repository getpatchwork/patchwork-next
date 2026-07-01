// Patchwork - automated patch tracking system
// Copyright (C) The Patchwork Contributors (see CONTRIBUTORS)
//
// SPDX-License-Identifier: GPL-2.0-or-later

package log

import (
	"fmt"
	"log"
	"log/syslog"
	"os"

	"golang.org/x/term"
)

const flags = log.Lshortfile | log.Ldate | log.Ltime

var (
	dbg    = log.New(os.Stderr, "DEBUG ", flags)
	info   = log.New(os.Stderr, "INFO ", flags)
	notice = log.New(os.Stderr, "NOTICE ", flags)
	warn   = log.New(os.Stderr, "WARN ", flags)
	err    = log.New(os.Stderr, "ERROR ", flags)
)

func newSyslog(tag string, prio syslog.Priority) *log.Logger {
	w, e := syslog.New(prio, tag)
	if e != nil {
		log.Fatalf("syslog.New: %v", e)
	}
	return log.New(w, "", log.Lshortfile)
}

func InitSyslog(tag string) {
	if term.IsTerminal(int(os.Stderr.Fd())) {
		return
	}
	dbg = newSyslog(tag, syslog.LOG_DEBUG)
	notice = newSyslog(tag, syslog.LOG_NOTICE)
	info = newSyslog(tag, syslog.LOG_INFO)
	warn = newSyslog(tag, syslog.LOG_WARNING)
	err = newSyslog(tag, syslog.LOG_ERR)
}

func ErrLogger() *log.Logger {
	return err
}

func logfmt(message string, args ...any) string {
	if len(args) > 0 {
		message = fmt.Sprintf(message, args...)
	}
	return message
}

func Debugf(message string, args ...any) {
	dbg.Output(2, logfmt(message, args...))
}

func Infof(message string, args ...any) {
	info.Output(2, logfmt(message, args...))
}

func Noticef(message string, args ...any) {
	notice.Output(2, logfmt(message, args...))
}

func Warnf(message string, args ...any) {
	warn.Output(2, logfmt(message, args...))
}

func Errorf(message string, args ...any) {
	err.Output(2, logfmt(message, args...))
}

func Fatalf(message string, args ...any) {
	err.Output(2, logfmt(message, args...))
	os.Exit(1)
}
