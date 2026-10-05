package git

import (
	"os/exec"
	"strings"
)

// Push pushes the current branch, setting upstream on first push.
func (r *Repo) Push() (string, error) {
	out, err := r.Runner.Combined("push")
	if err != nil {
		low := strings.ToLower(out)
		if strings.Contains(low, "no upstream") || strings.Contains(low, "set-upstream") ||
			strings.Contains(low, "has no upstream branch") {
			if b, berr := r.HeadBranch(); berr == nil && b != "" && b != "HEAD" {
				return r.Runner.Combined("push", "--set-upstream", "origin", b)
			}
		}
	}
	return out, err
}

// PushCmd builds an interactive push command (terminal prompts allowed), using
// --set-upstream when the branch has no upstream yet.
func (r *Repo) PushCmd() *exec.Cmd {
	if _, err := r.Runner.Output("rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{u}"); err != nil {
		if b, berr := r.HeadBranch(); berr == nil && b != "" && b != "HEAD" {
			return r.InteractiveCmd("push", "--set-upstream", "origin", b)
		}
	}
	return r.InteractiveCmd("push")
}
