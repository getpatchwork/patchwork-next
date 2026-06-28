// Patchwork - automated patch tracking system
// Copyright (C) The Patchwork Contributors (see CONTRIBUTORS)
//
// SPDX-License-Identifier: GPL-2.0-or-later

package mail

import (
	"os"
	"strings"
	"testing"

	"github.com/getpatchwork/patchwork/pkg/db"
)

func TestHashDiff(t *testing.T) {
	tests := []struct {
		name string
		diff string
		want string
	}{
		{
			"simple patch",
			"diff --git a/meep.text b/meep.text\n" +
				"index 3d75d48..a57f4dd 100644\n" +
				"--- a/meep.text\n" +
				"+++ b/meep.text\n" +
				"@@ -1,1 +1,2 @@\n" +
				" meep\n" +
				"+meep",
			"63fd8612c682c247a637f44bf9bf1c91f8f6c54e",
		},
		{
			"multiline patch",
			"diff --git a/lib/foo.c b/lib/foo.c\n" +
				"index 1234567..abcdef0 100644\n" +
				"--- a/lib/foo.c\n" +
				"+++ b/lib/foo.c\n" +
				"@@ -10,3 +10,4 @@\n" +
				" context\n" +
				"-old line\n" +
				"+new line\n" +
				"+added line",
			"1acb11c44305c1e0d6070cbe54b117696d8b078c",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := HashDiff(tt.diff)
			if got != tt.want {
				t.Errorf("HashDiff() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestHashDiffFromFile(t *testing.T) {
	data, err := os.ReadFile("testdata/patches/0001-add-line.patch")
	if err != nil {
		t.Fatal(err)
	}
	hash := HashDiff(string(data))
	if hash == "" {
		t.Error("HashDiff returned empty string for patch file")
	}
}

func TestParsePatch(t *testing.T) {
	t.Run("inline patch", func(t *testing.T) {
		patchData, err := os.ReadFile("testdata/patches/0001-add-line.patch")
		if err != nil {
			t.Fatal(err)
		}
		content := "Test for attached patch\n" + string(patchData)
		diff, comment := ParsePatch(content)
		if diff == "" {
			t.Fatal("expected diff, got empty")
		}
		if !strings.Contains(comment, "Test for attached patch") {
			t.Errorf("comment should contain original text, got %q", comment)
		}
		if !strings.HasPrefix(diff, "diff --git") {
			t.Errorf("diff should start with 'diff --git', got %q", diff[:40])
		}
	})

	t.Run("no diff", func(t *testing.T) {
		diff, comment := ParsePatch("just a plain message\nno diff here\n")
		if diff != "" {
			t.Errorf("expected no diff, got %q", diff)
		}
		if !strings.Contains(comment, "just a plain message") {
			t.Errorf("comment mismatch: %q", comment)
		}
	})
}

func TestFindFilenames(t *testing.T) {
	diff := "diff --git a/lib/foo.c b/lib/foo.c\n" +
		"--- a/lib/foo.c\n" +
		"+++ b/lib/foo.c\n" +
		"@@ -1,1 +1,2 @@\n" +
		" old\n" +
		"+new\n" +
		"diff --git a/include/bar.h b/include/bar.h\n" +
		"--- a/include/bar.h\n" +
		"+++ b/include/bar.h\n" +
		"@@ -1,1 +1,1 @@\n" +
		"-old\n" +
		"+new\n"

	filenames := FindFilenames(diff)
	if len(filenames) != 2 {
		t.Fatalf("expected 2 filenames, got %d: %v", len(filenames), filenames)
	}
	if filenames[0] != "include/bar.h" || filenames[1] != "lib/foo.c" {
		t.Errorf("filenames = %v, want [include/bar.h lib/foo.c]", filenames)
	}
}

func TestFindDelegateByFilename(t *testing.T) {
	rules := []db.DelegationRule{
		{Path: "drivers/*", UserID: 1},
		{Path: "arch/*", UserID: 2},
	}

	t.Run("all match same delegate", func(t *testing.T) {
		got := FindDelegateByFilename(rules, []string{"drivers/foo.c", "drivers/bar.c"})
		if got == nil || *got != 1 {
			t.Errorf("expected delegate 1, got %v", got)
		}
	})

	t.Run("mixed delegates", func(t *testing.T) {
		got := FindDelegateByFilename(rules, []string{"drivers/foo.c", "arch/x86.c"})
		if got != nil {
			t.Errorf("expected no delegate for mixed, got %d", *got)
		}
	})

	t.Run("no match", func(t *testing.T) {
		got := FindDelegateByFilename(rules, []string{"lib/util.c"})
		if got != nil {
			t.Errorf("expected no delegate for unmatched, got %d", *got)
		}
	})

	t.Run("empty filenames", func(t *testing.T) {
		got := FindDelegateByFilename(rules, nil)
		if got != nil {
			t.Error("expected no delegate for empty filenames")
		}
	})
}
