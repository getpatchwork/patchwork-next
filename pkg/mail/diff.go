// Patchwork - automated patch tracking system
// Copyright (C) The Patchwork Contributors (see CONTRIBUTORS)
//
// SPDX-License-Identifier: GPL-2.0-or-later

package mail

import (
	"crypto/sha1"
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/getpatchwork/patchwork/pkg/db"
)

var (
	hunkRe     = regexp.MustCompile(`^\@\@ -\d+(?:,(\d+))? \+\d+(?:,(\d+))? \@\@`)
	filenameRe = regexp.MustCompile(`^(---|\+\+\+) (\S+)`)
)

var extendedHeaderPrefixes = []string{
	"old mode ",
	"new mode ",
	"deleted file mode ",
	"new file mode ",
	"copy from ",
	"copy to ",
	"rename from ",
	"rename to ",
	"similarity index ",
	"dissimilarity index ",
	"index ",
}

func HashDiff(diff string) string {
	diff = strings.ReplaceAll(diff, "\r", "")
	diff = strings.TrimSpace(diff) + "\n"

	h := sha1.New()
	for line := range strings.Lines(diff) {
		if len(line) == 0 {
			continue
		}

		if m := filenameRe.FindStringSubmatch(line); m != nil {
			var prefix string
			if m[1] == "---" {
				prefix = "a/"
			} else {
				prefix = "b/"
			}
			parts := strings.SplitN(m[2], "/", 2)
			var rest string
			if len(parts) > 1 {
				rest = parts[1]
			}
			line = m[1] + " " + prefix + rest
		} else if m := hunkRe.FindStringSubmatch(line); m != nil {
			fn := func(s string) int {
				if s == "" {
					return 1
				}
				n, _ := strconv.Atoi(s)
				return n
			}
			line = fmt.Sprintf("@@ -%d +%d @@", fn(m[1]), fn(m[2]))
		} else if len(line) > 0 && (line[0] == '-' || line[0] == '+' || line[0] == ' ') {
			// keep as-is
		} else {
			continue
		}

		fmt.Fprintf(h, "%s", line)
	}

	return fmt.Sprintf("%x", h.Sum(nil))
}

func FindFilenames(diff string) []string {
	diff = strings.ReplaceAll(diff, "\r", "")
	diff = strings.TrimSpace(diff) + "\n"

	seen := map[string]bool{}
	for line := range strings.Lines(diff) {
		if len(line) == 0 {
			continue
		}
		m := filenameRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		filename := m[2]
		if strings.HasPrefix(filename, "/dev/null") {
			continue
		}
		parts := strings.SplitN(filename, "/", 2)
		if len(parts) > 1 {
			filename = parts[1]
		}
		seen[filename] = true
	}

	result := make([]string, 0, len(seen))
	for f := range seen {
		result = append(result, f)
	}
	sort.Strings(result)
	return result
}

func isExtendedHeader(line string) bool {
	for _, prefix := range extendedHeaderPrefixes {
		if strings.HasPrefix(line, prefix) {
			return true
		}
	}
	return false
}

type patchState int

const (
	stateText        patchState = iota // plain text / comment
	stateDiffHeader                    // suspected patch header (diff, Index:)
	stateFileOld                       // patch header line 1 (---)
	stateFileNew                       // patch header line 2 (+++)
	stateHunkHeader                    // @@ line
	stateHunkBody                      // patch hunk content
	stateExtHeader                     // extended header (rename, new file, index)
	stateBinaryPatch                   // GIT binary patch content
)

func ParsePatch(content string) (patch, comment string) {
	var patchBuf, commentBuf, buf strings.Builder
	state := stateText
	lc := [2]int{0, 0}
	hunk := 0

	for line := range strings.Lines(content) {
		switch state {
		case stateText: // text
			switch {
			case strings.HasPrefix(line, "diff ") || strings.HasPrefix(line, "Index: "):
				state = stateDiffHeader
				buf.WriteString(line)
			case strings.HasPrefix(line, "--- "):
				state = stateFileOld
				buf.WriteString(line)
			default:
				commentBuf.WriteString(line)
			}
		case stateDiffHeader: // suspected patch header
			buf.WriteString(line)
			if strings.HasPrefix(line, "--- ") {
				state = stateFileOld
			} else if isExtendedHeader(line) {
				state = stateExtHeader
			}
		case stateFileOld: // patch header line 1 (---)
			switch {
			case strings.HasPrefix(line, "+++ "):
				state = stateFileNew
				buf.WriteString(line)
			case hunk > 0:
				state = stateDiffHeader
				buf.WriteString(line)
			default:
				state = stateText
				commentBuf.WriteString(buf.String())
				commentBuf.WriteString(line)
				buf.Reset()
			}
		case stateFileNew: // patch header line 2 (+++)
			m := hunkRe.FindStringSubmatch(line)
			switch {
			case m != nil:
				fn := func(s string) int {
					if s == "" {
						return 1
					}
					n, _ := strconv.Atoi(s)
					return n
				}
				lc = [2]int{fn(m[1]), fn(m[2])}
				state = stateHunkHeader
				patchBuf.WriteString(buf.String())
				patchBuf.WriteString(line)
				buf.Reset()
			case strings.HasPrefix(line, "--- "):
				patchBuf.WriteString(buf.String())
				patchBuf.WriteString(line)
				buf.Reset()
				state = stateFileOld
			case hunk > 0 && strings.HasPrefix(line, `\ No newline at end of file`):
				patchBuf.WriteString(line)
			case hunk > 0:
				state = stateDiffHeader
				buf.WriteString(line)
			default:
				state = stateText
				commentBuf.WriteString(buf.String())
				commentBuf.WriteString(line)
				buf.Reset()
			}
		case stateHunkHeader, stateHunkBody: // hunk header / hunk content
			switch {
			case strings.HasPrefix(line, "-"):
				lc[0]--
			case strings.HasPrefix(line, "+"):
				lc[1]--
			case strings.HasPrefix(line, `\ No newline at end of file`):
				// not counted
			default:
				lc[0]--
				lc[1]--
			}
			patchBuf.WriteString(line)
			if lc[0] <= 0 && lc[1] <= 0 {
				state = stateFileNew
				hunk++
			} else {
				state = stateHunkBody
			}
		case stateExtHeader: // extended header
			switch {
			case isExtendedHeader(line):
				patchBuf.WriteString(buf.String())
				patchBuf.WriteString(line)
				buf.Reset()
			case strings.HasPrefix(line, "--- "):
				patchBuf.WriteString(buf.String())
				patchBuf.WriteString(line)
				buf.Reset()
				state = stateFileOld
			case strings.HasPrefix(line, "GIT binary patch"):
				patchBuf.WriteString(buf.String())
				patchBuf.WriteString(line)
				buf.Reset()
				state = stateBinaryPatch
			default:
				buf.WriteString(line)
				state = stateDiffHeader
			}
		case stateBinaryPatch: // binary patch
			if strings.HasPrefix(line, "diff") {
				buf.WriteString(line)
				state = stateText
			} else {
				patchBuf.WriteString(buf.String())
				patchBuf.WriteString(line)
				buf.Reset()
			}
		}
	}

	commentBuf.WriteString(buf.String())

	return patchBuf.String(), commentBuf.String()
}

func FindDelegateByFilename(rules []db.DelegationRule, filenames []string) *int {
	if len(filenames) == 0 || len(rules) == 0 {
		return nil
	}

	var delegate *int
	for _, filename := range filenames {
		var matched bool
		for _, rule := range rules {
			if ok, _ := filepath.Match(rule.Path, filename); ok {
				if delegate != nil && *delegate != rule.UserID {
					return nil
				}
				delegate = db.Ptr(rule.UserID)
				matched = true
				break
			}
		}
		if !matched {
			return nil
		}
	}
	return delegate
}
