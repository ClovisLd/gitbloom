package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func testRepo(t *testing.T) *Repo {
	t.Helper()
	dir := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=Test", "GIT_AUTHOR_EMAIL=t@e.com",
			"GIT_COMMITTER_NAME=Test", "GIT_COMMITTER_EMAIL=t@e.com",
			"LC_ALL=C",
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "-q")
	run("symbolic-ref", "HEAD", "refs/heads/main")
	run("config", "core.autocrlf", "false")
	repo, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	return repo
}

func read(t *testing.T, repo *Repo, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(repo.Root, name))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func writeFile(t *testing.T, repo *Repo, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(repo.Root, name), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestApplyCachedHunkThenReverse(t *testing.T) {
	repo := testRepo(t)

	var lines []string
	for i := 1; i <= 12; i++ {
		lines = append(lines, "line "+itoaPadded(i))
	}
	writeFile(t, repo, "f.txt", strings.Join(lines, "\n")+"\n")
	if err := repo.StageAll(); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.CommitStdin("init"); err != nil {
		t.Fatal(err)
	}

	lines[1] = "line 02 CHANGED"
	lines[10] = "line 11 CHANGED"
	writeFile(t, repo, "f.txt", strings.Join(lines, "\n")+"\n")

	patch, err := repo.Diff("f.txt", false)
	if err != nil {
		t.Fatal(err)
	}
	fd := ParseHunks(patch)
	if len(fd.Hunks) < 2 {
		t.Fatalf("expected >=2 hunks:\n%s", patch)
	}

	one := BuildPatch(fd, []Hunk{fd.Hunks[0]})
	if err := repo.ApplyCached(one); err != nil {
		t.Fatalf("apply --cached: %v", err)
	}
	cached := gitShowCached(t, repo)
	if !strings.Contains(cached, "line 02 CHANGED") || strings.Contains(cached, "line 11 CHANGED") {
		t.Fatalf("wrong hunk staged:\n%s", cached)
	}

	if err := repo.ApplyCachedReverse(one); err != nil {
		t.Fatalf("apply --cached --reverse: %v", err)
	}
	if cached := gitShowCached(t, repo); strings.TrimSpace(cached) != "" {
		t.Fatalf("index should be clean after reverse, got:\n%s", cached)
	}
}

func TestApplyCachedUntracked(t *testing.T) {
	repo := testRepo(t)
	writeFile(t, repo, "seed.txt", "seed\n")
	if err := repo.StageAll(); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.CommitStdin("init"); err != nil {
		t.Fatal(err)
	}

	writeFile(t, repo, "new.txt", "alpha\nbeta\n")
	content := read(t, repo, "new.txt")
	patch := "diff --git a/new.txt b/new.txt\n" +
		"new file mode 100644\n--- /dev/null\n+++ b/new.txt\n" +
		"@@ -0,0 +1,2 @@\n+alpha\n+beta\n"
	if err := repo.ApplyCached(patch); err != nil {
		t.Fatalf("apply untracked --cached: %v", err)
	}
	if cached := gitShowCached(t, repo); !strings.Contains(cached, "new.txt") {
		t.Fatalf("new file not staged, cached diff:\n%s", cached)
	}
	if content != "alpha\nbeta\n" {
		t.Fatalf("working tree changed unexpectedly: %q", content)
	}
}

func itoaPadded(n int) string {
	if n < 10 {
		return "0" + string(rune('0'+n))
	}
	return string(rune('0'+n/10)) + string(rune('0'+n%10))
}

func gitShowCached(t *testing.T, repo *Repo) string {
	t.Helper()
	out, err := repo.Runner.Output("diff", "--cached")
	if err != nil {
		t.Fatal(err)
	}
	return out
}
