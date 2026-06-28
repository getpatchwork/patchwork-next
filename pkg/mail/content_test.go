// Patchwork - automated patch tracking system
// Copyright (C) The Patchwork Contributors (see CONTRIBUTORS)
//
// SPDX-License-Identifier: GPL-2.0-or-later

package mail

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/emersion/go-mbox"
	"github.com/emersion/go-message/mail"
)

func openTestMail(t *testing.T, name string) *mail.Reader {
	t.Helper()
	data, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}

	var r io.Reader
	if strings.HasSuffix(name, ".mbox") {
		mr := mbox.NewReader(bytes.NewReader(data))
		msg, err := mr.NextMessage()
		if err != nil {
			// not a real mbox (no "From " envelope), treat as raw
			r = bytes.NewReader(data)
		} else {
			buf, err := io.ReadAll(msg)
			if err != nil {
				t.Fatal(err)
			}
			r = bytes.NewReader(buf)
		}
	} else {
		r = bytes.NewReader(data)
	}

	m, err := mail.CreateReader(r)
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func TestPullRequestParse(t *testing.T) {
	tests := []string{
		"mail/0001-git-pull-request.mbox",
		"mail/0002-git-pull-request-wrapped.mbox",
		"mail/0004-git-pull-request-git+ssh.mbox",
		"mail/0005-git-pull-request-ssh.mbox",
		"mail/0006-git-pull-request-http.mbox",
		"mail/0017-git-pull-request-git-2-14-3.mbox",
	}

	for _, name := range tests {
		t.Run(name, func(t *testing.T) {
			m := openTestMail(t, name)
			diff, comment := FindPatchContent(m)
			if diff != "" {
				t.Error("expected no diff for pull request")
			}
			if comment == "" {
				t.Error("expected comment content")
			}
			url := ParsePullRequest(comment)
			if url == "" {
				t.Error("expected pull request URL")
			}
		})
	}
}

func TestPullRequestWithDiff(t *testing.T) {
	m := openTestMail(t, "mail/0003-git-pull-request-with-diff.mbox")
	diff, comment := FindPatchContent(m)
	url := ParsePullRequest(comment)
	want := "git://git.kernel.org/pub/scm/linux/kernel/git/tip/linux-2.6-tip.git x86-fixes-for-linus"
	if url != want {
		t.Errorf("pull URL = %q, want %q", url, want)
	}
	if !strings.HasPrefix(diff, "diff --git a/arch/x86/include/asm/smp.h") {
		t.Errorf("diff should start with smp.h diff header")
	}
}

func TestGitRename(t *testing.T) {
	m := openTestMail(t, "mail/0008-git-rename.mbox")
	diff, _ := FindPatchContent(m)
	if diff == "" {
		t.Fatal("expected diff")
	}
	if strings.Count(diff, "\nrename from ") != 2 {
		t.Error("expected 2 'rename from' lines")
	}
	if strings.Count(diff, "\nrename to ") != 2 {
		t.Error("expected 2 'rename to' lines")
	}
}

func TestGitRenameWithDiff(t *testing.T) {
	m := openTestMail(t, "mail/0009-git-rename-with-diff.mbox")
	diff, comment := FindPatchContent(m)
	if diff == "" || comment == "" {
		t.Fatal("expected both diff and comment")
	}
	if strings.Count(diff, "\nrename from ") != 2 {
		t.Error("expected 2 'rename from' lines")
	}
	if strings.Count(diff, "\n-a\n+b") != 1 {
		t.Error("expected diff content")
	}
}

func TestGitBinaryFile(t *testing.T) {
	m := openTestMail(t, "mail/0025-git-add-binary-file.mbox")
	diff, comment := FindPatchContent(m)
	if diff == "" || comment == "" {
		t.Fatal("expected both diff and comment")
	}
	if !strings.HasPrefix(diff, "diff --git pixel.bmp pixel.bmp") {
		t.Error("diff should start with binary file header")
	}
	if !strings.Contains(diff, "GIT binary patch\n") {
		t.Error("diff should contain GIT binary patch marker")
	}
}

func TestGitMixedBinaryText(t *testing.T) {
	m := openTestMail(t, "mail/0026-git-add-mixed-binary-text-files.mbox")
	diff, comment := FindPatchContent(m)
	if diff == "" || comment == "" {
		t.Fatal("expected both diff and comment")
	}
	if !strings.Contains(diff, "GIT binary patch\n") {
		t.Error("missing binary patch marker")
	}
	if !strings.Contains(diff, "diff --git quit.sh quit.sh\n") {
		t.Error("missing text file diff")
	}
}

func TestNoNewlineAtEOF(t *testing.T) {
	m := openTestMail(t, "mail/0011-no-newline-at-end-of-file.mbox")
	diff, comment := FindPatchContent(m)
	if diff == "" || comment == "" {
		t.Fatal("expected both diff and comment")
	}
	if !strings.HasPrefix(diff, "diff --git a/tools/testing/selftests/powerpc/Makefile") {
		t.Error("diff should start with Makefile")
	}
	if strings.HasSuffix(strings.TrimSpace(comment), `\ No newline at end of file`) {
		t.Error("no-newline marker should not be in comment")
	}
	if !strings.HasSuffix(strings.TrimSpace(diff), `\ No newline at end of file`) {
		t.Error("no-newline marker should be at end of diff")
	}
	if strings.Count(diff, `\ No newline at end of file`) != 2 {
		t.Error("expected 2 no-newline markers in diff")
	}
}

func TestCVSFormat(t *testing.T) {
	m := openTestMail(t, "mail/0007-cvs-format-diff.mbox")
	diff, _ := FindPatchContent(m)
	if !strings.HasPrefix(diff, "Index") {
		t.Error("CVS diff should start with Index")
	}
}

func TestMultipartPatch(t *testing.T) {
	m := openTestMail(t, "mail/0019-multipart-patch.mbox")
	diff, comment := FindPatchContent(m)
	if diff == "" || comment == "" {
		t.Fatal("expected both diff and comment")
	}
	if strings.Contains(diff, "<div") {
		t.Error("HTML should not leak into diff")
	}
	if strings.Contains(comment, "<div") {
		t.Error("HTML should not leak into comment")
	}
}

func TestMultipartComment(t *testing.T) {
	m := openTestMail(t, "mail/0020-multipart-comment.mbox")
	comment := FindCommentContent(m)
	if comment == "" {
		t.Fatal("expected comment content")
	}
	if strings.Contains(comment, "<div") {
		t.Error("HTML should not leak into comment")
	}
}

func TestInvalidCharset(t *testing.T) {
	m := openTestMail(t, "mail/0010-invalid-charset.mbox")
	diff, comment := FindPatchContent(m)
	if diff == "" {
		t.Error("expected diff despite invalid charset")
	}
	if comment == "" {
		t.Error("expected comment despite invalid charset")
	}
}
