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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func openTestMail(t *testing.T, name string) *mail.Reader {
	t.Helper()
	data, err := os.ReadFile("testdata/" + name)
	require.NoError(t, err)

	var r io.Reader
	if strings.HasSuffix(name, ".mbox") {
		mr := mbox.NewReader(bytes.NewReader(data))
		msg, err := mr.NextMessage()
		if err != nil {
			// not a real mbox (no "From " envelope), treat as raw
			r = bytes.NewReader(data)
		} else {
			buf, err := io.ReadAll(msg)
			require.NoError(t, err)
			r = bytes.NewReader(buf)
		}
	} else {
		r = bytes.NewReader(data)
	}

	m, err := mail.CreateReader(r)
	require.NoError(t, err)
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
			assert.Empty(t, diff, "expected no diff for pull request")
			assert.NotEmpty(t, comment, "expected comment content")
			url := ParsePullRequest(comment)
			assert.NotEmpty(t, url, "expected pull request URL")
		})
	}
}

func TestPullRequestWithDiff(t *testing.T) {
	m := openTestMail(t, "mail/0003-git-pull-request-with-diff.mbox")
	diff, comment := FindPatchContent(m)
	url := ParsePullRequest(comment)
	want := "git://git.kernel.org/pub/scm/linux/kernel/git/tip/linux-2.6-tip.git x86-fixes-for-linus"
	assert.Equal(t, want, url)
	assert.True(t, strings.HasPrefix(diff, "diff --git a/arch/x86/include/asm/smp.h"), "diff should start with smp.h diff header")
}

func TestGitRename(t *testing.T) {
	m := openTestMail(t, "mail/0008-git-rename.mbox")
	diff, _ := FindPatchContent(m)
	require.NotEmpty(t, diff, "expected diff")
	assert.Equal(t, 2, strings.Count(diff, "\nrename from "), "expected 2 'rename from' lines")
	assert.Equal(t, 2, strings.Count(diff, "\nrename to "), "expected 2 'rename to' lines")
}

func TestGitRenameWithDiff(t *testing.T) {
	m := openTestMail(t, "mail/0009-git-rename-with-diff.mbox")
	diff, comment := FindPatchContent(m)
	require.NotEmpty(t, diff, "expected diff")
	require.NotEmpty(t, comment, "expected comment")
	assert.Equal(t, 2, strings.Count(diff, "\nrename from "), "expected 2 'rename from' lines")
	assert.Equal(t, 1, strings.Count(diff, "\n-a\n+b"), "expected diff content")
}

func TestGitBinaryFile(t *testing.T) {
	m := openTestMail(t, "mail/0025-git-add-binary-file.mbox")
	diff, comment := FindPatchContent(m)
	require.NotEmpty(t, diff, "expected diff")
	require.NotEmpty(t, comment, "expected comment")
	assert.True(t, strings.HasPrefix(diff, "diff --git pixel.bmp pixel.bmp"), "diff should start with binary file header")
	assert.Contains(t, diff, "GIT binary patch\n", "diff should contain GIT binary patch marker")
}

func TestGitMixedBinaryText(t *testing.T) {
	m := openTestMail(t, "mail/0026-git-add-mixed-binary-text-files.mbox")
	diff, comment := FindPatchContent(m)
	require.NotEmpty(t, diff, "expected diff")
	require.NotEmpty(t, comment, "expected comment")
	assert.Contains(t, diff, "GIT binary patch\n", "missing binary patch marker")
	assert.Contains(t, diff, "diff --git quit.sh quit.sh\n", "missing text file diff")
}

func TestNoNewlineAtEOF(t *testing.T) {
	m := openTestMail(t, "mail/0011-no-newline-at-end-of-file.mbox")
	diff, comment := FindPatchContent(m)
	require.NotEmpty(t, diff, "expected diff")
	require.NotEmpty(t, comment, "expected comment")
	assert.True(t, strings.HasPrefix(diff, "diff --git a/tools/testing/selftests/powerpc/Makefile"), "diff should start with Makefile")
	assert.False(t, strings.HasSuffix(strings.TrimSpace(comment), `\ No newline at end of file`), "no-newline marker should not be in comment")
	assert.True(t, strings.HasSuffix(strings.TrimSpace(diff), `\ No newline at end of file`), "no-newline marker should be at end of diff")
	assert.Equal(t, 2, strings.Count(diff, `\ No newline at end of file`), "expected 2 no-newline markers in diff")
}

func TestCVSFormat(t *testing.T) {
	m := openTestMail(t, "mail/0007-cvs-format-diff.mbox")
	diff, _ := FindPatchContent(m)
	assert.True(t, strings.HasPrefix(diff, "Index"), "CVS diff should start with Index")
}

func TestMultipartPatch(t *testing.T) {
	m := openTestMail(t, "mail/0019-multipart-patch.mbox")
	diff, comment := FindPatchContent(m)
	require.NotEmpty(t, diff, "expected diff")
	require.NotEmpty(t, comment, "expected comment")
	assert.NotContains(t, diff, "<div", "HTML should not leak into diff")
	assert.NotContains(t, comment, "<div", "HTML should not leak into comment")
}

func TestMultipartComment(t *testing.T) {
	m := openTestMail(t, "mail/0020-multipart-comment.mbox")
	comment := FindCommentContent(m)
	require.NotEmpty(t, comment, "expected comment content")
	assert.NotContains(t, comment, "<div", "HTML should not leak into comment")
}

func TestInvalidCharset(t *testing.T) {
	m := openTestMail(t, "mail/0010-invalid-charset.mbox")
	diff, comment := FindPatchContent(m)
	assert.NotEmpty(t, diff, "expected diff despite invalid charset")
	assert.NotEmpty(t, comment, "expected comment despite invalid charset")
}

func TestMultipartWithoutClosingBoundary(t *testing.T) {
	m := openTestMail(t, "mail/0028-multipart-without-closing-boundary.mbox")
	diff, comment := FindPatchContent(m)
	assert.Empty(t, diff, "expected no diff")
	assert.NotEmpty(t, comment, "expected a comment despite multipart without closing boundary")
}
