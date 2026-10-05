package git

// StageFile adds all changes to a path to the index.
func (r *Repo) StageFile(path string) error {
	_, err := r.Runner.Output("add", "--", path)
	return err
}

// StageAll stages every change, including deletions and untracked files.
func (r *Repo) StageAll() error {
	_, err := r.Runner.Output("add", "-A")
	return err
}

// UnstageFile removes a path from the index while keeping the working tree.
func (r *Repo) UnstageFile(path string) error {
	_, err := r.Runner.Output("restore", "--staged", "--", path)
	return err
}

// UnstageAll clears the index, falling back to `git reset` (and to
// `git rm --cached` before the first commit) when HEAD does not exist yet.
func (r *Repo) UnstageAll() error {
	if _, err := r.Runner.Output("restore", "--staged", "."); err == nil {
		return nil
	}
	if _, err := r.Runner.Output("reset"); err == nil {
		return nil
	}
	_, err := r.Runner.Output("rm", "--cached", "-r", "--quiet", ".")
	return err
}

// ApplyCached applies a single-file patch to the index only (the working tree
// is left untouched).
func (r *Repo) ApplyCached(patch string) error {
	_, err := r.Runner.OutputWithStdin([]byte(patch),
		"apply", "--cached", "--whitespace=nowarn", "-")
	return err
}

// ApplyCachedReverse reverses a single-file patch in the index, which is how a
// staged hunk is taken back out without touching the working tree.
func (r *Repo) ApplyCachedReverse(patch string) error {
	_, err := r.Runner.OutputWithStdin([]byte(patch),
		"apply", "--cached", "--reverse", "--whitespace=nowarn", "-")
	return err
}
