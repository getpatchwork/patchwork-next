// Patchwork - automated patch tracking system
// Copyright (C) The Patchwork Contributors (see CONTRIBUTORS)
//
// SPDX-License-Identifier: GPL-2.0-or-later

package config

import (
	"fmt"
	"regexp"
	"strconv"
	"time"
)

type Duration time.Duration

var durationRe = regexp.MustCompile(`^(\d+)(w|d|h|m|s|ms|us|ns)$`)

func ParseDuration(s string) (Duration, error) {
	m := durationRe.FindStringSubmatch(s)
	if m != nil {
		n, err := strconv.ParseInt(m[1], 10, 64)
		if err != nil {
			return 0, err
		}
		switch m[2] {
		case "w":
			return Duration(time.Duration(n) * 7 * 24 * time.Hour), nil
		case "d":
			return Duration(time.Duration(n) * 24 * time.Hour), nil
		}
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return 0, fmt.Errorf("invalid duration %q", s)
	}
	return Duration(d), nil
}

func (d Duration) Duration() time.Duration {
	return time.Duration(d)
}

func (d Duration) String() string {
	td := time.Duration(d)
	if td == 0 {
		return "0s"
	}
	week := 7 * 24 * time.Hour
	day := 24 * time.Hour
	if td%week == 0 {
		return fmt.Sprintf("%dw", td/week)
	}
	if td%day == 0 {
		return fmt.Sprintf("%dd", td/day)
	}
	return td.String()
}

func (d *Duration) UnmarshalText(b []byte) error {
	v, err := ParseDuration(string(b))
	if err != nil {
		return err
	}
	*d = v
	return nil
}
