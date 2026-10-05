package git

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Diff returns a plain unified diff for a path. When path is empty the whole
// working tree is diffed. staged selects the index rather than the worktree.
func (r *Repo) Diff(path string, staged bool) (string, error) {
	args := []string{"diff", "--no-color", "--no-ext-diff"}
	if staged {
		args = append(args, "--cached")
	}
	if path != "" {
		args = append(args, "--", path)
	}
	return r.Runner.Output(args...)
}

// ReadWorkingFile returns the contents of a working-tree file (untracked files
// have no diff, so we display their contents instead). The path is confined to
// the repository root so a crafted repository cannot make the reader escape it
// (e.g. via an absolute path or ".." segments).
func (r *Repo) ReadWorkingFile(path string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("empty path")
	}
	root, err := filepath.Abs(r.Root)
	if err != nil {
		return "", err
	}
	full := filepath.Join(root, filepath.FromSlash(path))
	rel, err := filepath.Rel(root, full)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return "", fmt.Errorf("path escapes repository: %s", path)
	}
	b, err := os.ReadFile(full)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
