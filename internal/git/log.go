package git

import (
	"strconv"
	"strings"
)

// Commit is one entry from the history.
type Commit struct {
	Hash    string
	Short   string
	Author  string
	Subject string
	Refs    []string
}

// IsHead reports whether any ref contains the HEAD pointer.
func (c Commit) IsHead() bool {
	for _, r := range c.Refs {
		if r == "HEAD" || strings.HasPrefix(r, "HEAD -> ") {
			return true
		}
	}
	return false
}

// Log returns up to limit commits across all local/remote branches and tags.
func (r *Repo) Log(limit int) ([]Commit, error) {
	format := "%H%x1f%an%x1f%s%x1f%D"
	args := []string{
		"log", "--date-order", "--branches", "--remotes", "--tags", "HEAD",
		"--pretty=format:" + format,
		"-n", strconv.Itoa(limit),
	}
	out, err := r.Runner.Output(args...)
	if err != nil {
		low := strings.ToLower(err.Error())
		if strings.Contains(low, "does not have any commits") || strings.Contains(low, "unknown revision") || strings.Contains(low, "bad revision") {
			return nil, nil
		}
		return nil, err
	}
	return parseLog(out), nil
}

func parseLog(out string) []Commit {
	lines := splitLines(out)
	commits := make([]Commit, 0, len(lines))
	for _, line := range lines {
		fields := strings.Split(line, "\x1f")
		if len(fields) < 4 {
			continue
		}
		short := fields[0]
		if len(short) > 8 {
			short = short[:8]
		}
		c := Commit{
			Hash:    fields[0],
			Short:   short,
			Author:  fields[1],
			Subject: fields[2],
		}
		for _, ref := range strings.Split(fields[3], ", ") {
			if ref = strings.TrimSpace(ref); ref != "" {
				c.Refs = append(c.Refs, ref)
			}
		}
		commits = append(commits, c)
	}
	return commits
}
