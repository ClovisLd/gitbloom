package ui

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"gitbloom/internal/git"
)

func gitRun(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=Test", "GIT_AUTHOR_EMAIL=test@example.com",
		"GIT_COMMITTER_NAME=Test", "GIT_COMMITTER_EMAIL=test@example.com",
		"LC_ALL=C",
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func gitOut(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=Test", "GIT_AUTHOR_EMAIL=test@example.com",
		"GIT_COMMITTER_NAME=Test", "GIT_COMMITTER_EMAIL=test@example.com",
		"LC_ALL=C",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return string(out)
}

// keyMsg builds a KeyMsg from a friendly descriptor.
func keyMsg(s string) tea.KeyMsg {
	switch s {
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	case "tab":
		return tea.KeyMsg{Type: tea.KeyTab}
	case "space":
		return tea.KeyMsg{Type: tea.KeySpace}
	case "ctrl+s":
		return tea.KeyMsg{Type: tea.KeyCtrlS}
	default:
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
	}
}

func press(t *testing.T, m Model, keys ...string) (Model, tea.Cmd) {
	t.Helper()
	var cmd tea.Cmd
	for _, k := range keys {
		mm, c := m.Update(keyMsg(k))
		m = mm.(Model)
		cmd = c
	}
	return m, cmd
}

func write(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func scratchRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	gitRun(t, dir, "init", "-q")
	gitRun(t, dir, "symbolic-ref", "HEAD", "refs/heads/main")

	write(t, dir, "a.txt", "one\ntwo\nthree\n")
	gitRun(t, dir, "add", "-A")
	gitRun(t, dir, "commit", "-q", "-m", "first commit")

	gitRun(t, dir, "checkout", "-q", "-b", "feature/login")
	write(t, dir, "b.txt", "login code\n")
	gitRun(t, dir, "add", "-A")
	gitRun(t, dir, "commit", "-q", "-m", "add login")

	gitRun(t, dir, "checkout", "-q", "main")
	write(t, dir, "a.txt", "one\nTWO changed\nthree\n")
	gitRun(t, dir, "add", "-A")
	gitRun(t, dir, "commit", "-q", "-m", "edit a")

	// A commit touching two files, so cursor-preservation can be tested.
	write(t, dir, "a.txt", "one\nTWO changed\nthree\nfour\n")
	write(t, dir, "d.txt", "delta\n")
	gitRun(t, dir, "add", "-A")
	gitRun(t, dir, "commit", "-q", "-m", "two files")

	// Newest commit (single file), so "two files" is not index 0.
	write(t, dir, "e.txt", "epsilon\n")
	gitRun(t, dir, "add", "-A")
	gitRun(t, dir, "commit", "-q", "-m", "after")
	return dir
}

// drain runs a command and feeds its message back into the model, following
// chained commands (data → files → diff) a few steps deep.
func drain(t *testing.T, m Model, cmd tea.Cmd) Model {
	t.Helper()
	for i := 0; i < 4 && cmd != nil; i++ {
		mm, next := m.Update(cmd())
		m = mm.(Model)
		cmd = next
	}
	return m
}

func TestCommitFileDiffFlow(t *testing.T) {
	dir := scratchRepo(t)
	repo, err := git.Open(dir)
	if err != nil {
		t.Fatal(err)
	}

	m := New(repo)
	mm, _ := m.Update(tea.WindowSizeMsg{Width: 150, Height: 40})
	m = drain(t, mm.(Model), m.reload())

	if m.errText != "" {
		t.Fatalf("load error: %s", m.errText)
	}
	if len(m.commits) == 0 {
		t.Fatal("expected commits")
	}
	if len(m.files) == 0 {
		t.Fatal("expected files in HEAD commit")
	}
	if strings.TrimSpace(m.diff) == "" {
		t.Fatal("expected a non-empty diff for the selected file")
	}
	if !strings.Contains(m.diff, "+") && !strings.Contains(m.diff, "-") {
		t.Fatalf("expected +/- lines in diff, got:\n%s", m.diff)
	}

	// The first commit should touch a.txt.
	var found bool
	m.commitIdx = len(m.commitView) - 1 // oldest commit
	m = drain(t, m, m.loadFiles())
	for _, f := range m.files {
		if f.Path == "a.txt" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected a.txt in first commit, got %+v", m.files)
	}
}

func TestSearchFiltersFilesInCommit(t *testing.T) {
	dir := scratchRepo(t)
	repo, err := git.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	m := New(repo)
	mm, _ := m.Update(tea.WindowSizeMsg{Width: 150, Height: 40})
	m = drain(t, mm.(Model), m.reload())

	// Find the merge-less "edit a" commit: it only changes a.txt.
	m.searchTarget = searchCommits
	m.query = "edit a"
	m.rebuildViews()
	m.clampSelection()
	m = drain(t, m, m.loadFiles())
	if len(m.files) != 1 || m.files[0].Path != "a.txt" {
		t.Fatalf("expected only a.txt, got %+v", m.files)
	}

	m.query = ""
	m.searchTarget = searchFiles
	m.rebuildViews()
	m.clampSelection()
	if len(m.fileView) != 1 {
		t.Fatalf("expected 1 visible file, got %d", len(m.fileView))
	}
}

func TestRefreshPreservesSelection(t *testing.T) {
	dir := scratchRepo(t)
	repo, err := git.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	m := New(repo)
	mm, _ := m.Update(tea.WindowSizeMsg{Width: 150, Height: 40})
	m = drain(t, mm.(Model), m.reload())

	// Locate the two-file commit by subject (index order is not guaranteed).
	m.searchTarget = searchCommits
	m.query = "two files"
	m.rebuildViews()
	m.clampSelection()
	m = drain(t, m, m.loadFiles())

	if len(m.files) < 2 {
		t.Fatalf("expected >=2 files in the 'two files' commit, got %d", len(m.files))
	}

	// Park the cursor on the second file.
	m.fileIdx = 1
	wantCommit := m.selectedCommitHash()
	wantPath := m.selectedFilePath()

	// Simulate one background refresh tick.
	m = drain(t, m, m.reload())

	if got := m.selectedCommitHash(); got != wantCommit {
		t.Fatalf("commit cursor moved on refresh: want %s got %s", wantCommit, got)
	}
	if got := m.selectedFilePath(); got != wantPath {
		t.Fatalf("file cursor moved on refresh: want %s got %s", wantPath, got)
	}
	if m.fileIdx != 1 {
		t.Fatalf("file index reset on refresh: got %d want 1", m.fileIdx)
	}
}

func TestListFollowsCursor(t *testing.T) {
	dir := t.TempDir()
	gitRun(t, dir, "init", "-q")
	gitRun(t, dir, "symbolic-ref", "HEAD", "refs/heads/main")
	for i := 0; i < 30; i++ {
		write(t, dir, fmt.Sprintf("file%02d.txt", i), fmt.Sprintf("content %d\n", i))
	}
	gitRun(t, dir, "add", "-A")
	gitRun(t, dir, "commit", "-q", "-m", "many files")

	repo, err := git.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	m := New(repo)
	mm, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 14})
	m = drain(t, mm.(Model), m.reload())
	m.setFocus(paneFiles)

	rows := m.listRows()
	if len(m.fileView) <= rows {
		t.Fatalf("need more files (%d) than rows (%d)", len(m.fileView), rows)
	}

	for i := 0; i < rows+5; i++ {
		mm, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
		m = mm.(Model)
	}

	if m.fileIdx <= rows {
		t.Fatalf("cursor did not advance: fileIdx=%d rows=%d", m.fileIdx, rows)
	}
	if m.fileOffset == 0 {
		t.Fatal("list did not scroll with the cursor")
	}
	if m.fileIdx < m.fileOffset || m.fileIdx >= m.fileOffset+rows {
		t.Fatalf("cursor off-screen: idx=%d offset=%d rows=%d", m.fileIdx, m.fileOffset, rows)
	}
}

func TestModelHandlesKeys(t *testing.T) {
	dir := scratchRepo(t)
	repo, err := git.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	m := New(repo)
	mm, _ := m.Update(tea.WindowSizeMsg{Width: 140, Height: 42})
	m = drain(t, mm.(Model), m.reload())

	keys := []tea.KeyMsg{
		{Type: tea.KeyTab},
		{Type: tea.KeyRunes, Runes: []rune("j")},
		{Type: tea.KeyRunes, Runes: []rune("k")},
		{Type: tea.KeyEnter},
		{Type: tea.KeyRunes, Runes: []rune("j")},
		{Type: tea.KeyTab},
		{Type: tea.KeyRunes, Runes: []rune("j")},
		{Type: tea.KeyRunes, Runes: []rune("/")},
		{Type: tea.KeyRunes, Runes: []rune("txt")},
		{Type: tea.KeyEsc},
		{Type: tea.KeyRunes, Runes: []rune("?")},
		{Type: tea.KeyEsc},
	}
	for i, k := range keys {
		mm, _ := m.Update(k)
		m = mm.(Model)
		if v := m.View(); strings.TrimSpace(v) == "" {
			t.Fatalf("key %d (%v) produced empty view", i, k)
		}
	}
}

// dirtyRepo has one modified tracked file and one untracked file.
func dirtyRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	gitRun(t, dir, "init", "-q")
	gitRun(t, dir, "symbolic-ref", "HEAD", "refs/heads/main")
	gitRun(t, dir, "config", "user.name", "Test")
	gitRun(t, dir, "config", "user.email", "test@example.com")
	write(t, dir, "a.txt", "one\n")
	gitRun(t, dir, "add", "-A")
	gitRun(t, dir, "commit", "-q", "-m", "init")
	write(t, dir, "a.txt", "one changed\n")
	write(t, dir, "b.txt", "brand new\n")
	return dir
}

func TestStagingIsEmptyByDefault(t *testing.T) {
	dir := dirtyRepo(t)
	repo, err := git.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	m := New(repo)
	mm, _ := m.Update(tea.WindowSizeMsg{Width: 150, Height: 40})
	m = drain(t, mm.(Model), m.reload())

	if len(m.workFiles) != 2 {
		t.Fatalf("expected 2 working changes, got %d", len(m.workFiles))
	}

	m, _ = press(t, m, "k") // up from newest commit -> New commit
	if !m.stagingMode {
		t.Fatal("k from the top commit should enter the New commit tool")
	}
	m, _ = press(t, m, "enter")
	if m.focus != paneTool || m.commitTab != tabFiles {
		t.Fatalf("expected Files tab, got focus=%v tab=%v", m.focus, m.commitTab)
	}
	if got := m.stagedCount(); got != 0 {
		t.Fatalf("expected nothing staged by default, got %d", got)
	}
}

func TestStageFileAndCommit(t *testing.T) {
	dir := dirtyRepo(t)
	repo, err := git.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	m := New(repo)
	mm, _ := m.Update(tea.WindowSizeMsg{Width: 150, Height: 40})
	m = drain(t, mm.(Model), m.reload())

	m, _ = press(t, m, "k", "enter") // New commit -> Files tab

	// Stage a.txt (the modified tracked file), leave b.txt unstaged.
	if idx := m.findWork("a.txt"); idx >= 0 {
		m.workIdx = idx
	} else {
		t.Fatalf("a.txt not in work view: %+v", m.workFiles)
	}
	m, cmd := press(t, m, " ")
	if cmd == nil {
		t.Fatal("space produced no staging command")
	}
	m = drain(t, m, cmd)
	if m.stagedCount() != 1 {
		t.Fatalf("expected 1 staged file, got %d", m.stagedCount())
	}

	// Switch to Message and type a message.
	m, _ = press(t, m, "tab")
	if m.commitTab != tabMessage {
		t.Fatalf("tab should switch to Message, got %v", m.commitTab)
	}
	m, _ = press(t, m, "only a")
	if got := m.msgEditor.Value(); got != "only a" {
		t.Fatalf("message editor = %q", got)
	}

	m, cmd = press(t, m, "ctrl+s")
	if cmd == nil {
		t.Fatal("ctrl+s produced no commit command")
	}
	m = drain(t, m, cmd)

	subject := strings.TrimSpace(gitOut(t, dir, "log", "-1", "--format=%s"))
	if subject != "only a" {
		t.Fatalf("HEAD subject = %q, want %q", subject, "only a")
	}
	names := gitOut(t, dir, "show", "--name-only", "--format=", "-1")
	if !strings.Contains(names, "a.txt") {
		t.Fatalf("expected a.txt committed, got:\n%s", names)
	}
	if strings.Contains(names, "b.txt") {
		t.Fatalf("b.txt should NOT have been committed, got:\n%s", names)
	}
	status := gitOut(t, dir, "status", "--porcelain")
	if !strings.Contains(status, "b.txt") {
		t.Fatalf("b.txt should still be an uncommitted change, got:\n%s", status)
	}
}

// hunkRepo has a single tracked file with two well-separated edits, so the
// working-tree diff splits into two hunks.
func hunkRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	gitRun(t, dir, "init", "-q")
	gitRun(t, dir, "symbolic-ref", "HEAD", "refs/heads/main")
	gitRun(t, dir, "config", "user.name", "Test")
	gitRun(t, dir, "config", "user.email", "test@example.com")

	var b strings.Builder
	for i := 1; i <= 12; i++ {
		fmt.Fprintf(&b, "line %02d\n", i)
	}
	write(t, dir, "f.txt", b.String())
	gitRun(t, dir, "add", "-A")
	gitRun(t, dir, "commit", "-q", "-m", "init")

	b.Reset()
	for i := 1; i <= 12; i++ {
		switch i {
		case 2:
			b.WriteString("line 02 CHANGED\n")
		case 11:
			b.WriteString("line 11 CHANGED\n")
		default:
			fmt.Fprintf(&b, "line %02d\n", i)
		}
	}
	write(t, dir, "f.txt", b.String())
	return dir
}

func TestStageOneHunk(t *testing.T) {
	dir := hunkRepo(t)
	repo, err := git.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	m := New(repo)
	mm, _ := m.Update(tea.WindowSizeMsg{Width: 150, Height: 40})
	m = drain(t, mm.(Model), m.reload())

	// New commit -> Files tab, then preview f.txt.
	m, _ = press(t, m, "k", "enter")
	if idx := m.findWork("f.txt"); idx >= 0 {
		m.workIdx = idx
	} else {
		t.Fatalf("f.txt not in work view: %+v", m.workFiles)
	}
	m, cmd := press(t, m, "enter")
	m = drain(t, m, cmd)

	fd := git.ParseHunks(m.diff)
	if len(fd.Hunks) < 2 {
		t.Fatalf("expected >=2 hunks, got %d:\n%s", len(fd.Hunks), m.diff)
	}

	// Stage only the first hunk.
	m.hunkIdx = 0
	m, cmd = press(t, m, " ")
	if cmd == nil {
		t.Fatal("space on a hunk produced no command")
	}
	m = drain(t, m, cmd)

	cached := gitOut(t, dir, "diff", "--cached")
	if !strings.Contains(cached, "line 02 CHANGED") {
		t.Fatalf("first hunk should be staged, cached diff:\n%s", cached)
	}
	if strings.Contains(cached, "line 11 CHANGED") {
		t.Fatalf("second hunk should NOT be staged, cached diff:\n%s", cached)
	}

	// Leave the preview, write a message, commit.
	m, _ = press(t, m, "esc")
	if m.previewing {
		t.Fatal("esc should leave the preview")
	}
	m, _ = press(t, m, "tab")
	m, _ = press(t, m, "partial")
	m, cmd = press(t, m, "ctrl+s")
	m = drain(t, m, cmd)

	show := gitOut(t, dir, "show", "--format=", "--patch", "-1")
	if !strings.Contains(show, "line 02 CHANGED") {
		t.Fatalf("HEAD should contain the staged hunk, got:\n%s", show)
	}
	if strings.Contains(show, "line 11 CHANGED") {
		t.Fatalf("HEAD should not contain the unstaged hunk, got:\n%s", show)
	}
	if status := gitOut(t, dir, "status", "--porcelain"); !strings.Contains(status, "f.txt") {
		t.Fatalf("the unstaged hunk should remain in the working tree, got:\n%s", status)
	}
}

func TestContextFooterSwitches(t *testing.T) {
	dir := dirtyRepo(t)
	repo, err := git.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	m := New(repo)
	mm, _ := m.Update(tea.WindowSizeMsg{Width: 150, Height: 40})
	m = drain(t, mm.(Model), m.reload())

	// Commits pane context mentions the tool entry point.
	if !containsStr(m.contextKeys(), "k -> New commit") {
		t.Fatalf("commits context = %v", m.contextKeys())
	}
	m, _ = press(t, m, "k", "enter")
	if !containsStr(m.contextKeys(), "space stage") {
		t.Fatalf("files-tab context = %v", m.contextKeys())
	}
	m, _ = press(t, m, "tab")
	if !containsStr(m.contextKeys(), "type message") {
		t.Fatalf("message-tab context = %v", m.contextKeys())
	}
}

func TestNoWideGlyphsInRows(t *testing.T) {
	dir := scratchRepo(t)
	repo, err := git.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	m := New(repo)
	mm, _ := m.Update(tea.WindowSizeMsg{Width: 150, Height: 40})
	m = drain(t, mm.(Model), m.reload())

	for _, bad := range []string{"\u25c6", "\u25b8", "\u2713", "\u2717", "\u21c4"} {
		if strings.Contains(m.View(), bad) {
			t.Fatalf("view still contains wide/ambiguous glyph %q", bad)
		}
	}
}

func containsStr(xs []string, want string) bool {
	for _, x := range xs {
		if x == want {
			return true
		}
	}
	return false
}

// findWork returns the view position of a working-tree path, or -1.
func (m Model) findWork(path string) int {
	for vi, fi := range m.workView {
		if m.workFiles[fi].Path == path {
			return vi
		}
	}
	return -1
}

func TestCtrlCQuitsFromSearch(t *testing.T) {
	dir := scratchRepo(t)
	repo, err := git.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	m := New(repo)
	mm, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = drain(t, mm.(Model), m.reload())

	m, _ = press(t, m, "/")
	if !m.searching {
		t.Fatal("expected search to be focused")
	}
	_, cmd := m.Update(keyMsg("ctrl+c"))
	if cmd == nil {
		t.Fatal("ctrl+c produced no command")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatalf("ctrl+c in search did not quit (got %T)", cmd())
	}
}

func TestPushAsksForConfirmation(t *testing.T) {
	dir := dirtyRepo(t)
	repo, err := git.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	m := New(repo)
	mm, _ := m.Update(tea.WindowSizeMsg{Width: 150, Height: 40})
	m = drain(t, mm.(Model), m.reload())

	m, _ = press(t, m, "k", "enter") // New commit -> Files tab
	m, _ = press(t, m, "p")
	if m.confirm == "" {
		t.Fatal("p should ask for confirmation before pushing")
	}
	m, _ = press(t, m, "n")
	if m.confirm != "" {
		t.Fatal("n should cancel the push confirmation")
	}
}

func TestEmptyRepoCursorStaysOnSyntheticRow(t *testing.T) {
	dir := t.TempDir()
	gitRun(t, dir, "init", "-q")
	gitRun(t, dir, "symbolic-ref", "HEAD", "refs/heads/main")

	repo, err := git.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	m := New(repo)
	mm, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = drain(t, mm.(Model), m.reload())
	if len(m.commits) != 0 {
		t.Fatalf("expected no commits, got %d", len(m.commits))
	}

	m, _ = press(t, m, "k")
	if !m.stagingMode {
		t.Fatal("k should select the New commit row")
	}
	for i := 0; i < 5; i++ {
		m, _ = press(t, m, "j")
	}
	if !m.stagingMode {
		t.Fatal("cursor left the synthetic row although there are no commits")
	}
	m, _ = press(t, m, "G")
	if !m.stagingMode {
		t.Fatal("G left the synthetic row although there are no commits")
	}
	if strings.TrimSpace(m.View()) == "" {
		t.Fatal("empty view")
	}
}

func TestSanitizeStripsTerminalEscapes(t *testing.T) {
	cases := map[string]string{
		"plain":                          "plain",
		"\x1b[31mred\x1b[0m":             "red",
		"\x1b]8;;http://evil.example\x07link": "link",
		"a\x07b":                         "a?b",
		"cr\rline":                       "cr?line",
	}
	for in, want := range cases {
		if got := sanitize(in); got != want {
			t.Fatalf("sanitize(%q) = %q, want %q", in, got, want)
		}
	}
	if got := sanitize("multi\nline"); got != "multi\nline" {
		t.Fatalf("sanitize must preserve newlines, got %q", got)
	}
}

func TestDiffColorizerHunkContext(t *testing.T) {
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(termenv.Ascii)

	patch := strings.Join([]string{
		"diff --git a/x b/x",
		"index 1111111..2222222 100644",
		"--- a/x",
		"+++ b/x",
		"@@ -1,2 +1,2 @@",
		"--- foo",
		"+++ bar",
	}, "\n")
	lines := diffLines(patch, OpenCodeTheme(), 30)

	const red = "38;2;224;108;117"    // Bad  #E06C75
	const accent = "38;2;157;124;216" // Accent #9D7CD8
	if !strings.Contains(lines[5], red) {
		t.Fatalf("in-hunk deletion should be red, got %q", lines[5])
	}
	if strings.Contains(lines[5], accent) {
		t.Fatalf("in-hunk deletion was treated as a file header: %q", lines[5])
	}
	if !strings.Contains(lines[2], accent) {
		t.Fatalf("file header should be accent, got %q", lines[2])
	}
}
