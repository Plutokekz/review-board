package main

import "strings"

type Stats struct {
	Files     int `json:"files"`
	Additions int `json:"additions"`
	Deletions int `json:"deletions"`
}

// DiffStats counts changed files and +/- lines in a unified git diff.
func DiffStats(diff string) Stats {
	var st Stats
	inHunk := false
	for _, line := range strings.Split(diff, "\n") {
		switch {
		case strings.HasPrefix(line, "diff --git "):
			st.Files++
			inHunk = false
		case strings.HasPrefix(line, "@@"):
			inHunk = true
		case !inHunk:
			// file header lines (---, +++, index, mode) — not content
		case strings.HasPrefix(line, "+"):
			st.Additions++
		case strings.HasPrefix(line, "-"):
			st.Deletions++
		}
	}
	return st
}
