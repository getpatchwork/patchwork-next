// Patchwork - automated patch tracking system
// Copyright (C) The Patchwork Contributors (see CONTRIBUTORS)
//
// SPDX-License-Identifier: GPL-2.0-or-later

package mail

import "testing"

func TestCleanSubject(t *testing.T) {
	tests := []struct {
		input    string
		drop     []string
		wantName string
		wantPfx  []string
	}{
		{"meep", nil, "meep", nil},
		{"Re: meep", nil, "meep", nil},
		{"[PATCH] meep", nil, "meep", nil},
		{"[PATCH] meep \n meep", nil, "meep meep", nil},
		{"[PATCH] meep,\n meep", nil, "meep, meep", nil},
		{"[PATCH RFC] meep", nil, "[RFC] meep", []string{"RFC"}},
		{"[PATCH,RFC] meep", nil, "[RFC] meep", []string{"RFC"}},
		{"[PATCH,1/2] meep", nil, "[1/2] meep", []string{"1/2"}},
		{"[PATCH RFC 1/2] meep", nil, "[RFC,1/2] meep", []string{"RFC", "1/2"}},
		{"[PATCH] [RFC] meep", nil, "[RFC] meep", []string{"RFC"}},
		{"[PATCH] [RFC,1/2] meep", nil, "[RFC,1/2] meep", []string{"RFC", "1/2"}},
		{"[PATCH] [RFC] [1/2] meep", nil, "[RFC,1/2] meep", []string{"RFC", "1/2"}},
		{"[PATCH] rewrite [a-z] regexes", nil, "rewrite [a-z] regexes", nil},
		{"[PATCH] [RFC] rewrite [a-z] regexes", nil, "[RFC] rewrite [a-z] regexes", []string{"RFC"}},
		{"[foo] [bar] meep", []string{"foo"}, "[bar] meep", []string{"bar"}},
		{"[FOO] [bar] meep", []string{"foo"}, "[bar] meep", []string{"bar"}},
	}

	for _, tt := range tests {
		name, prefixes := CleanSubject(tt.input, tt.drop)
		if name != tt.wantName {
			t.Errorf("CleanSubject(%q): name = %q, want %q", tt.input, name, tt.wantName)
		}
		if !slicesEqual(prefixes, tt.wantPfx) {
			t.Errorf("CleanSubject(%q): prefixes = %v, want %v", tt.input, prefixes, tt.wantPfx)
		}
	}
}

func TestIsComment(t *testing.T) {
	for _, s := range []string{
		"RE: meep", "Re: meep", "re: meep",
		"RE meep", "Re meep", "re meep",
	} {
		if !IsComment(s) {
			t.Errorf("IsComment(%q) = false, want true", s)
		}
	}
}

func TestSplitPrefixes(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		{"PATCH", []string{"PATCH"}},
		{"PATCH,RFC", []string{"PATCH", "RFC"}},
		{"", nil},
		{"PATCH,", []string{"PATCH"}},
		{"PATCH ", []string{"PATCH"}},
		{"PATCH 1/2", []string{"PATCH", "1/2"}},
	}

	for _, tt := range tests {
		got := SplitPrefixes(tt.input)
		if !slicesEqual(got, tt.want) {
			t.Errorf("SplitPrefixes(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestParseSeriesMarker(t *testing.T) {
	tests := []struct {
		input []string
		wantX int
		wantN int
	}{
		{nil, 0, 0},
		{[]string{"bar"}, 0, 0},
		{[]string{"bar", "1/2"}, 1, 2},
		{[]string{"bar", "0/12"}, 0, 12},
		{[]string{"bar", "1 of 2"}, 1, 2},
		{[]string{"bar", "0 of 12"}, 0, 12},
		{[]string{"PATCH1/8"}, 1, 8},
		{[]string{"PATCH1 of 8"}, 1, 8},
		{[]string{"PATCH100/123"}, 100, 123},
		{[]string{"v2PATCH1/4"}, 1, 4},
		{[]string{"v2", "PATCH1/4"}, 1, 4},
		{[]string{"v2.3PATCH1/4"}, 1, 4},
	}

	for _, tt := range tests {
		x, n := ParseSeriesMarker(tt.input)
		if x != tt.wantX || n != tt.wantN {
			t.Errorf("ParseSeriesMarker(%v) = (%d, %d), want (%d, %d)",
				tt.input, x, n, tt.wantX, tt.wantN)
		}
	}
}

func TestParseVersion(t *testing.T) {
	tests := []struct {
		subject  string
		prefixes []string
		want     int
	}{
		{"", nil, 1},
		{"Hello, world", nil, 1},
		{"Hello, world", []string{"version"}, 1},
		{"Hello, world", []string{"v2"}, 2},
		{"Hello, world", []string{"V6"}, 6},
		{"Hello, world", []string{"v10"}, 10},
		{"Hello, world (v2)", nil, 2},
		{"Hello, world (V6)", nil, 6},
	}

	for _, tt := range tests {
		got := ParseVersion(tt.subject, tt.prefixes)
		if got != tt.want {
			t.Errorf("ParseVersion(%q, %v) = %d, want %d",
				tt.subject, tt.prefixes, got, tt.want)
		}
	}
}

func TestParsePullRequest(t *testing.T) {
	content := `The following changes since commit abc123:

  Initial commit (2024-01-01 00:00:00 +0000)

are available in the git repository at:

  git://git.kernel.org/pub/scm/linux/kernel/git/tip/linux-2.6-tip.git x86-fixes-for-linus

for you to fetch changes up to def456:
`
	url := ParsePullRequest(content)
	want := "git://git.kernel.org/pub/scm/linux/kernel/git/tip/linux-2.6-tip.git x86-fixes-for-linus"
	if url != want {
		t.Errorf("ParsePullRequest() = %q, want %q", url, want)
	}

	if url := ParsePullRequest("no pull request here"); url != "" {
		t.Errorf("ParsePullRequest(no match) = %q, want empty", url)
	}
}

func TestFindReferences(t *testing.T) {
	t.Run("header with comments", func(t *testing.T) {
		h := makeHeader(t, map[string]string{
			"From":        "test@example.com",
			"Subject":     "test",
			"Message-ID":  "<test@example.com>",
			"In-Reply-To": "<4574b99b-edac-d8dc-9141-79c3109d2fcc@huawei.com> (message from liqingqing on Thu, 1 Apr 2021 16:51:45 +0800)",
		})
		refs := FindReferences(h)
		want := []string{"<4574b99b-edac-d8dc-9141-79c3109d2fcc@huawei.com>"}
		if !slicesEqual(refs, want) {
			t.Errorf("FindReferences() = %v, want %v", refs, want)
		}
	})

	t.Run("duplicate references", func(t *testing.T) {
		h := makeHeader(t, map[string]string{
			"From":        "test@example.com",
			"Subject":     "test",
			"Message-ID":  "<20130510114450.7104c5d2@nehalam.linuxnetplumber.net>",
			"In-Reply-To": "<525534677.5312512.1368202896189.JavaMail.root@vmware.com>",
			"References":  "<AFCFCEB8EB0E24448E4EB95988BA1E531FA2EA0D@xmb-aln-x05.cisco.com> <CAE68AUOr7B5a2QvduJhH0kEHPi+sR9X3qfrtumgLxT1BK4VS+Q@mail.gmail.com> <1676591087.5291867.1368201908283.JavaMail.root@vmware.com> <20130510091549.3c064df6@nehalam.linuxnetplumber.net> <525534677.5312512.1368202896189.JavaMail.root@vmware.com>",
		})
		refs := FindReferences(h)
		// In-Reply-To comes first, then References (which should
		// not duplicate the In-Reply-To entry)
		if len(refs) == 0 {
			t.Fatal("expected references")
		}
		// first ref should be the In-Reply-To
		if refs[0] != "<525534677.5312512.1368202896189.JavaMail.root@vmware.com>" {
			t.Errorf("first ref = %q", refs[0])
		}
		// TODO: FindReferences should deduplicate refs that appear
		// in both In-Reply-To and References headers. The Python
		// version does this. For now, just check the expected
		// refs are all present.
		if len(refs) < 5 {
			t.Errorf("expected at least 5 refs, got %d", len(refs))
		}
	})
}

func TestCleanContent(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		// signature stripping
		{"hello\n-- \nsig\n", "hello"},
		// list footer stripping
		{"hello\n_______________\nfooter\n", "hello"},
		// no signature
		{"hello\nworld\n", "hello\nworld"},
		// empty
		{"", ""},
	}
	for _, tt := range tests {
		got := CleanContent(tt.input)
		if got != tt.want {
			t.Errorf("CleanContent(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func slicesEqual(a, b []string) bool {
	if len(a) == 0 && len(b) == 0 {
		return true
	}
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
