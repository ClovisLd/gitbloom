package git

import (
	"path/filepath"
	"strings"
)

// Repo is an opened git repository.
type Repo struct {
	Root   string
	GitDir string
	Name   string
	Runner Runner
}

// Open discovers the repository containing dir.
func Open(dir string) (*Repo, error) {
	r := Runner{Dir: dir}
	out, err := r.Output("rev-parse", "--show-toplevel")
	if err != nil {
		return nil, err
	}
	root := filepath.FromSlash(strings.TrimSpace(out))
	if root == "" {
		root = dir
	}
	if abs, aerr := filepath.Abs(root); aerr == nil {
		root = abs
	}

	gitDir := filepath.Join(root, ".git")
	if gd, gerr := r.Output("rev-parse", "--absolute-git-dir"); gerr == nil {
		if gd = strings.TrimSpace(gd); gd != "" {
			gitDir = filepath.FromSlash(gd)
		}
	}

	return &Repo{
		Root:   root,
		GitDir: gitDir,
		Name:   filepath.Base(root),
		Runner: Runner{Dir: root},
	}, nil
}

// HeadBranch returns the current branch name, or "HEAD" when detached.
func (r *Repo) HeadBranch() (string, error) {
	out, err := r.Runner.Output("rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

// HeadShort returns the abbreviated HEAD hash.
func (r *Repo) HeadShort() string {
	out, err := r.Runner.Output("rev-parse", "--short", "HEAD")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(out)
}
