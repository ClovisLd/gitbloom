package git

import (
	"strings"
	"testing"
)

func TestParseHunksSplitsTwo(t *testing.T) {
	patch := strings.Join([]string{
		"diff --git a/f.txt b/f.txt",
		"index 1111111..2222222 100644",
		"--- a/f.txt",
		"+++ b/f.txt",
		"@@ -1,3 +1,3 @@",
		" a",
		"-b",
		"+B",
		" c",
		"@@ -10,3 +10,3 @@",
		" x",
		"-y",
		"+Y",
		" z",
	}, "\n") + "\n"

	fd := ParseHunks(patch)
	if got := len(fd.Header); got != 4 {
		t.Fatalf("header lines = %d, want 4 (%v)", got, fd.Header)
	}
	if len(fd.Hunks) != 2 {
		t.Fatalf("hunks = %d, want 2", len(fd.Hunks))
	}
	if fd.Hunks[0].Line != 4 {
		t.Fatalf("first hunk line = %d, want 4", fd.Hunks[0].Line)
	}
	if fd.Hunks[1].Line != 9 {
		t.Fatalf("second hunk line = %d, want 9", fd.Hunks[1].Line)
	}
	if len(fd.Hunks[0].Lines) != 4 {
		t.Fatalf("first hunk body = %d lines, want 4: %v", len(fd.Hunks[0].Lines), fd.Hunks[0].Lines)
	}
}

func TestBuildPatchOnlySelectedHunk(t *testing.T) {
	patch := "diff --git a/f b/f\n--- a/f\n+++ b/f\n@@ -1 +1 @@\n-a\n+A\n@@ -9 +9 @@\n-b\n+B\n"
	fd := ParseHunks(patch)

	got := BuildPatch(fd, []Hunk{fd.Hunks[1]})
	want := "diff --git a/f b/f\n--- a/f\n+++ b/f\n@@ -9 +9 @@\n-b\n+B\n"
	if got != want {
		t.Fatalf("BuildPatch =\n%q\nwant\n%q", got, want)
	}
	if strings.Contains(got, "+A") {
		t.Fatalf("the unselected hunk leaked into the patch:\n%s", got)
	}
}

func TestParseHunksNoHunks(t *testing.T) {
	patch := "diff --git a/x b/x\nBinary files a/x and b/x differ\n"
	fd := ParseHunks(patch)
	if len(fd.Hunks) != 0 {
		t.Fatalf("expected no hunks, got %d", len(fd.Hunks))
	}
	if len(fd.Header) != 2 {
		t.Fatalf("header = %v", fd.Header)
	}
}
