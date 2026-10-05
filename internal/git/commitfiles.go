package git

import "strings"

// CommitFile is one path touched by a commit.
type CommitFile struct {
	Status byte // A, M, D, R, C, T
	Path   string
	Orig   string // original path for renames/copies
}

// CommitFiles lists the paths changed by a commit. For merge commits it
// compares against the first parent, which is what people usually mean by
// "what this commit changed".
func (r *Repo) CommitFiles(hash string) ([]CommitFile, error) {
	out, err := r.Runner.Output(
		"show", "--no-color", "--name-status", "--format=",
		"--first-parent", "--root", hash,
	)
	if err != nil {
		return nil, err
	}
	var files []CommitFile
	for _, line := range splitLines(out) {
		line = strings.TrimRight(line, "\r")
		if line == "" {
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) < 2 || fields[0] == "" {
			continue
		}
		st := fields[0][0]
		f := CommitFile{Status: st}
		if (st == 'R' || st == 'C') && len(fields) >= 3 {
			f.Orig = fields[1]
			f.Path = fields[2]
		} else {
			f.Path = fields[1]
		}
		files = append(files, f)
	}
	return files, nil
}

// CommitFileDiff returns the patch for a single path within a commit.
func (r *Repo) CommitFileDiff(hash, path string) (string, error) {
	return r.Runner.Output(
		"show", "--no-color", "--no-ext-diff", "--format=",
		"--first-parent", "--patch", hash, "--", path,
	)
}
