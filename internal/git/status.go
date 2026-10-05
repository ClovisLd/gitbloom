package git

import (
	"strconv"
	"strings"
)

// FileStatus is one entry from `git status --porcelain`.
type FileStatus struct {
	Index byte // staged (X)
	Work  byte // worktree (Y)
	Path  string
	Orig  string // original path for renames/copies
}

func (f FileStatus) Untracked() bool { return f.Index == '?' && f.Work == '?' }

// Staged reports whether the index differs from HEAD for this path.
func (f FileStatus) Staged() bool { return f.Index != ' ' && f.Index != '?' }

// Unstaged reports whether the working tree differs from the index for this
// path (untracked files count as unstaged).
func (f FileStatus) Unstaged() bool { return f.Untracked() || f.Work != ' ' }

// Letter is a single status character for display: untracked '?', otherwise
// the worktree change if any, else the staged change.
func (f FileStatus) Letter() byte {
	switch {
	case f.Untracked():
		return '?'
	case f.Work != ' ':
		return f.Work
	default:
		return f.Index
	}
}

// Status is a snapshot of the working tree.
type Status struct {
	Branch   string
	Upstream string
	Ahead    int
	Behind   int
	Detached bool
	Files    []FileStatus
}

// Status loads the working tree state.
func (r *Repo) Status() (Status, error) {
	out, err := r.Runner.Output("status", "--porcelain=v1", "--branch", "--untracked-files=all")
	if err != nil {
		return Status{}, err
	}
	var s Status
	lines := splitLines(out)
	for i, line := range lines {
		if i == 0 && strings.HasPrefix(line, "## ") {
			s.Branch, s.Upstream, s.Ahead, s.Behind, s.Detached = parseBranchLine(line)
			continue
		}
		if len(line) < 4 {
			continue
		}
		f := FileStatus{Index: line[0], Work: line[1]}
		rest := line[3:]
		if i := strings.Index(rest, " -> "); i >= 0 && (f.Index == 'R' || f.Index == 'C' || f.Work == 'R' || f.Work == 'C') {
			f.Orig = unquote(rest[:i])
			rest = rest[i+4:]
		}
		f.Path = unquote(rest)
		s.Files = append(s.Files, f)
	}
	return s, nil
}

func parseBranchLine(line string) (branch, upstream string, ahead, behind int, detached bool) {
	rest := strings.TrimPrefix(line, "## ")
	if rest == "HEAD (no branch)" || strings.HasPrefix(rest, "HEAD (no branch)") {
		return "HEAD", "", 0, 0, true
	}
	if i := strings.Index(rest, "..."); i >= 0 {
		branch = rest[:i]
		up := rest[i+3:]
		if j := strings.Index(up, " ["); j >= 0 {
			upstream = up[:j]
			ahead, behind = parseAheadBehind(up[j+2:])
		} else {
			upstream = up
		}
	} else {
		branch = rest
	}
	branch = strings.TrimPrefix(branch, "No commits yet on ")
	return branch, upstream, ahead, behind, false
}

func parseAheadBehind(s string) (ahead, behind int) {
	s = strings.TrimSuffix(strings.TrimSpace(s), "]")
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		switch {
		case strings.HasPrefix(part, "ahead "):
			ahead, _ = strconv.Atoi(strings.TrimPrefix(part, "ahead "))
		case strings.HasPrefix(part, "behind "):
			behind, _ = strconv.Atoi(strings.TrimPrefix(part, "behind "))
		}
	}
	return ahead, behind
}

func unquote(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		if u, err := strconv.Unquote(s); err == nil {
			return u
		}
	}
	return s
}
