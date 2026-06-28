// Patchwork - automated patch tracking system
// Copyright (C) The Patchwork Contributors (see CONTRIBUTORS)
//
// SPDX-License-Identifier: GPL-2.0-or-later

package mail

import (
	"errors"
	"io"
	"strings"

	"github.com/emersion/go-message"
	_ "github.com/emersion/go-message/charset" // register common charsets
	"github.com/emersion/go-message/mail"

	"github.com/getpatchwork/patchwork/pkg/log"
)

type textPart struct {
	payload string
	subtype string
}

func findTextParts(m *mail.Reader) []textPart {
	var results []textPart
	var buf strings.Builder

	for {
		part, err := m.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			if !message.IsUnknownCharset(err) {
				log.Warnf("NextPart: %v", err)
			}
			if part == nil {
				continue
			}
		}

		var ctype string
		switch h := part.Header.(type) {
		case *mail.InlineHeader:
			ctype, _, _ = h.ContentType()
		case *mail.AttachmentHeader:
			ctype, _, _ = h.ContentType()
		default:
			continue
		}

		subtype, isText := strings.CutPrefix(ctype, "text/")
		if !isText {
			continue
		}

		if n, err := io.Copy(&buf, part.Body); err != nil {
			log.Warnf("failed to read part body: %s", err)
			continue
		} else if n == 0 {
			continue
		}

		results = append(results, textPart{
			payload: strings.ReplaceAll(buf.String(), "\r\n", "\n"),
			subtype: subtype,
		})

		buf.Reset()
	}

	return results
}

func FindPatchContent(m *mail.Reader) (string, string) {
	var comment strings.Builder
	var patch string

	for _, part := range findTextParts(m) {
		switch part.subtype {
		case "x-patch", "x-diff":
			patch = part.payload
		case "plain":
			c := part.payload
			if patch == "" {
				p, rest := ParsePatch(c)
				if p != "" {
					patch = p
				}
				c = rest
			}
			if c != "" {
				comment.WriteString(strings.TrimSpace(c))
				comment.WriteByte('\n')
			}
		}
	}

	return patch, CleanContent(comment.String())
}

func FindCommentContent(m *mail.Reader) string {
	var buf strings.Builder

	for _, part := range findTextParts(m) {
		if part.payload == "" || part.subtype != "plain" {
			continue
		}
		buf.WriteString(strings.TrimSpace(part.payload))
		buf.WriteByte('\n')
	}

	return CleanContent(buf.String())
}
