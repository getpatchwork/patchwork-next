// Patchwork - automated patch tracking system
// Copyright (C) The Patchwork Contributors (see CONTRIBUTORS)
//
// SPDX-License-Identifier: GPL-2.0-or-later

package mail

import "strings"

func nameSimilarity(a, b string) float64 {
	a = strings.ToLower(stripPrefixes(a))
	b = strings.ToLower(stripPrefixes(b))
	if a == "" || b == "" {
		return 0.0
	}
	return sequenceMatcherRatio(a, b)
}

// sequenceMatcherRatio computes a similarity ratio between two strings,
// equivalent to Python's difflib.SequenceMatcher.ratio().
func sequenceMatcherRatio(a, b string) float64 {
	if a == b {
		return 1.0
	}
	total := len(a) + len(b)
	if total == 0 {
		return 1.0
	}
	matches := longestCommonSubsequenceLen(a, b)
	return 2.0 * float64(matches) / float64(total)
}

func longestCommonSubsequenceLen(a, b string) int {
	m, n := len(a), len(b)
	prev := make([]int, n+1)
	curr := make([]int, n+1)
	total := 0

	for i := 1; i <= m; i++ {
		for j := 1; j <= n; j++ {
			switch {
			case a[i-1] == b[j-1]:
				curr[j] = prev[j-1] + 1
			case prev[j] > curr[j-1]:
				curr[j] = prev[j]
			default:
				curr[j] = curr[j-1]
			}
		}
		total = curr[n]
		prev, curr = curr, prev
		for j := range curr {
			curr[j] = 0
		}
	}

	return total
}
