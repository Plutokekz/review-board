package main

import "testing"

func TestDiffStats(t *testing.T) {
	diff := "diff --git a/x.txt b/x.txt\n" +
		"--- a/x.txt\n+++ b/x.txt\n@@ -1,2 +1,2 @@\n" +
		" keep\n-old line\n+new line\n" +
		"diff --git a/y.txt b/y.txt\n" +
		"--- a/y.txt\n+++ b/y.txt\n@@ -0,0 +1 @@\n+added\n"
	got := DiffStats(diff)
	want := Stats{Files: 2, Additions: 2, Deletions: 1}
	if got != want {
		t.Fatalf("DiffStats = %+v, want %+v", got, want)
	}
	if empty := DiffStats(""); empty != (Stats{}) {
		t.Fatalf("DiffStats(\"\") = %+v, want zero", empty)
	}
}

// TestDiffStatsContentLinesLookingLikeHeaders guards against a bug where
// content lines that happen to start with "---"/"+++" (e.g. a deleted SQL
// comment "-- drop table" or an added "++counter" line) were mistaken for
// diff file headers and dropped from the +/- counts.
func TestDiffStatsContentLinesLookingLikeHeaders(t *testing.T) {
	diff := "diff --git a/q.sql b/q.sql\n" +
		"--- a/q.sql\n+++ b/q.sql\n@@ -1,1 +1,1 @@\n" +
		"--- drop table\n+++counter\n"
	got := DiffStats(diff)
	want := Stats{Files: 1, Additions: 1, Deletions: 1}
	if got != want {
		t.Fatalf("DiffStats = %+v, want %+v", got, want)
	}
}
