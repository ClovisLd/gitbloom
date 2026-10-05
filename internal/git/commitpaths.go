package git

import (
	"os/exec"
	"strings"
)

// CommitStdin commits the current index with the message supplied on stdin
// (git commit -F -), so long or multi-line messages need no argv quoting.
func (r *Repo) CommitStdin(message string) (string, error) {
	return r.Runner.OutputWithStdin([]byte(message), "commit", "-F", "-")
}

// CommitCmd builds an interactive index commit (-F -) for use with
// tea.ExecProcess, so GPG pinentry and hooks can use the real terminal.
func (r *Repo) CommitCmd(message string) *exec.Cmd {
	cmd := r.InteractiveCmd("commit", "-F", "-")
	cmd.Stdin = strings.NewReader(message)
	return cmd
}
