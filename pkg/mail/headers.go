// Patchwork - automated patch tracking system
// Copyright (C) The Patchwork Contributors (see CONTRIBUTORS)
//
// SPDX-License-Identifier: GPL-2.0-or-later

package mail

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/emersion/go-message/mail"

	"github.com/getpatchwork/patchwork/pkg/log"
)

var (
	commentRe     = regexp.MustCompile(`(?i)^(re)[:\s]\s*`)
	reRe          = regexp.MustCompile(`(?i)^(re|fwd?)[:\s]\s*`)
	prefixRe      = regexp.MustCompile(`(?s)^\[([^\]]*)\]\s*(.*)$`)
	spaceRe       = regexp.MustCompile(`\s+`)
	hgSeriesRe    = regexp.MustCompile(`(?s)^PATCH (\d+ of \d+)(.*)$`)
	prefixSplitRe = regexp.MustCompile(`[,\s]+`)
)

func NormalizeSpace(s string) string {
	return strings.TrimSpace(spaceRe.ReplaceAllString(s, " "))
}

func IsComment(subject string) bool {
	return commentRe.MatchString(strings.TrimSpace(subject))
}

// Turn a prefix string into a list of prefix tokens.
func SplitPrefixes(prefix string) []string {
	var tokens []string
	// detect mercurial series marker (M of N)
	if m := hgSeriesRe.FindStringSubmatch(prefix); m != nil {
		tokens = append(tokens, "PATCH", m[1])
		prefix = m[2]
	}
	for _, s := range prefixSplitRe.Split(prefix, -1) {
		if s != "" {
			tokens = append(tokens, strings.TrimSpace(s))
		}
	}
	return tokens
}

// Clean a Subject: header from an incoming patch.
//
// Removes Re: and Fwd: strings, as well as [PATCH]-style prefixes. By
// default, only [PATCH] is removed, and we keep any other bracketed
// data in the subject. If drop_prefixes is provided, remove those
// too, comparing case-insensitively.
func CleanSubject(subject string, dropPrefixes []string) (string, []string) {
	drop := map[string]bool{"patch": true}
	for _, p := range dropPrefixes {
		drop[strings.ToLower(p)] = true
	}

	subject = reRe.ReplaceAllString(strings.TrimSpace(subject), "")

	var prefixes []string
	for {
		m := prefixRe.FindStringSubmatch(subject)
		if m == nil {
			break
		}
		for _, p := range SplitPrefixes(m[1]) {
			if !drop[strings.ToLower(p)] {
				prefixes = append(prefixes, p)
			}
		}
		subject = m[2]
	}

	subject = NormalizeSpace(subject)
	if len(prefixes) > 0 {
		subject = fmt.Sprintf("[%s] %s", strings.Join(prefixes, ","), subject)
	}

	return subject, prefixes
}

var seriesMarkerRe = regexp.MustCompile(`.*?([0-9]+)(?:/| of )([0-9]+)$`)

func ParseSeriesMarker(prefixes []string) (int, int) {
	for _, p := range prefixes {
		m := seriesMarkerRe.FindStringSubmatch(p)
		if m != nil {
			x, _ := strconv.Atoi(m[1])
			n, _ := strconv.Atoi(m[2])
			return x, n
		}
	}
	return 0, 0
}

var (
	versionPrefixRe  = regexp.MustCompile(`^[vV](\d+)$`)
	versionSubjectRe = regexp.MustCompile(`\([vV](\d+)\)`)
)

func ParseVersion(subject string, prefixes []string) int {
	for _, p := range prefixes {
		if m := versionPrefixRe.FindStringSubmatch(p); m != nil {
			v, _ := strconv.Atoi(m[1])
			return v
		}
	}
	if m := versionSubjectRe.FindStringSubmatch(subject); m != nil {
		v, _ := strconv.Atoi(m[1])
		return v
	}
	return 1
}

var msgIdRe = regexp.MustCompile(`<([^>]+)>`)

func FindReferences(h *mail.Header) []string {
	irp, err := h.MsgIDList("In-Reply-To")
	if err != nil {
		v := h.Get("In-Reply-To")
		log.Warnf("In-Reply-To: %q: %s", v, err)
		for _, m := range msgIdRe.FindAllStringSubmatch(v, -1) {
			irp = append(irp, m[1])
		}
	}
	refs, err := h.MsgIDList("References")
	if err != nil {
		v := h.Get("References")
		log.Warnf("References: %q: %s", v, err)
		for _, m := range msgIdRe.FindAllStringSubmatch(v, -1) {
			refs = append(refs, m[1])
		}
	}
	// Add angle brackets back.
	all := make([]string, 0, len(irp)+len(refs))
	for _, msgid := range irp {
		all = append(all, "<"+msgid+">")
	}
	for _, msgid := range refs {
		all = append(all, "<"+msgid+">")
	}
	return all
}

func FormatHeaders(h *mail.Header) string {
	var buf strings.Builder
	fields := h.Fields()
	for fields.Next() {
		txt, err := fields.Text()
		if txt == "" && err != nil {
			txt = fields.Value()
		}
		fmt.Fprintf(&buf, "%s: %s\n", fields.Key(), txt)
	}
	return buf.String()
}

func GetOriginalSender(h *mail.Header, from *mail.Address) *mail.Address {
	strippedName, _, _ := strings.Cut(from.Name, " via ")
	strippedName, _ = strings.CutSuffix(strippedName, " via")
	strippedName = strings.Trim(strippedName, `"' `)

	xOriginal, _ := mail.ParseAddress(h.Get("X-Original-From"))
	if xOriginal != nil {
		if xOriginal.Name == "" {
			xOriginal.Name = strippedName
		}
		return xOriginal
	}

	for _, key := range []string{"Reply-To", "Cc"} {
		addrs, _ := h.AddressList(key)
		for _, addr := range addrs {
			if addr.Name == strippedName {
				return addr
			}
		}
	}

	return from
}

var sigRe = regexp.MustCompile(`(?ms)^(-- |_+)\n.*`)

func CleanContent(content string) string {
	return strings.TrimSpace(sigRe.ReplaceAllString(content, ""))
}

var pullRequestRe = regexp.MustCompile(
	`(?mi)^The following changes since commit(?s:.*?)` +
		`^are available in the git repository at:\s*\n` +
		`^\s*([^\n]+)$`,
)

func ParsePullRequest(content string) string {
	m := pullRequestRe.FindStringSubmatch(content)
	if m == nil {
		return ""
	}
	return strings.TrimSpace(spaceRe.ReplaceAllString(m[1], " "))
}

func stripPrefixes(name string) string {
	for {
		m := prefixRe.FindStringSubmatch(name)
		if m == nil {
			break
		}
		name = m[2]
	}
	return NormalizeSpace(name)
}
