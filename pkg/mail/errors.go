// Patchwork - automated patch tracking system
// Copyright (C) The Patchwork Contributors (see CONTRIBUTORS)
//
// SPDX-License-Identifier: GPL-2.0-or-later

package mail

import (
	"fmt"
)

type DuplicateMailError struct {
	MsgID string
}

func (e *DuplicateMailError) Error() string {
	return fmt.Sprintf("duplicate message: %s", e.MsgID)
}

func DuplicateMailErr(msgid string) error {
	return &DuplicateMailError{msgid}
}

type ParseError struct {
	err error
}

func (e *ParseError) Error() string { return e.err.Error() }
func (e *ParseError) Unwrap() error { return e.err }

func ParseErr(format string, args ...any) error {
	return &ParseError{fmt.Errorf(format, args...)}
}
