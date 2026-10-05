package git

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// Runner executes git commands inside a fixed directory.
type Runner struct {
	Dir string
}

func (r Runner) args(extra ...string) []string {
	base := make([]string, 0, len(extra)+3)
	base = append(base, "-C", r.Dir, "--no-pager")
	base = append(base, extra...)
	return base
}

func (r Runner) env() []string {
	return append(os.Environ(),
		"LC_ALL=C",
		"LANG=C",
		"GIT_OPTIONAL_LOCKS=0",
		"GIT_PAGER=cat",
		"GIT_TERMINAL_PROMPT=0",
	)
}

// Output runs git and returns stdout. The trailing newline is preserved so
// callers can decide how to split.
func (r Runner) Output(args ...string) (string, error) {
	return r.OutputWithStdin(nil, args...)
}

// OutputWithStdin runs git feeding stdin to the process.
func (r Runner) OutputWithStdin(stdin []byte, args ...string) (string, error) {
	cmd := exec.Command("git", r.args(args...)...)
	cmd.Env = r.env()
	if stdin != nil {
		cmd.Stdin = strings.NewReader(string(stdin))
	}
	var out, errb strings.Builder
	cmd.Stdout = &out
	cmd.Stderr = &errb
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(errb.String())
		if msg == "" {
			msg = err.Error()
		}
		// Keep the whole reason on one line so it survives the footer.
		return out.String(), fmt.Errorf("%s", collapse(msg))
	}
	return out.String(), nil
}

// Combined runs git and returns stdout and stderr merged. Used for network
// operations where progress output matters.
func (r Runner) Combined(args ...string) (string, error) {
	cmd := exec.Command("git", r.args(args...)...)
	cmd.Env = r.env()
	var buf strings.Builder
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	err := cmd.Run()
	out := strings.TrimRight(strings.ReplaceAll(buf.String(), "\r\n", "\n"), "\n")
	if err != nil && out == "" {
		out = err.Error()
	}
	return out, err
}

// InteractiveCmd builds a git command that may need a real terminal (GPG
// pinentry, SSH/credential prompts). It does NOT disable terminal prompts, so
// it is meant to be run through tea.ExecProcess.
func (r Runner) InteractiveCmd(args ...string) *exec.Cmd {
	cmd := exec.Command("git", r.args(args...)...)
	cmd.Env = append(os.Environ(),
		"LC_ALL=C",
		"LANG=C",
		"GIT_PAGER=cat",
	)
	return cmd
}

// InteractiveCmd builds a terminal-attached git command for this repo.
func (r *Repo) InteractiveCmd(args ...string) *exec.Cmd {
	return r.Runner.InteractiveCmd(args...)
}

func collapse(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	var parts []string
	for _, p := range strings.Split(s, "\n") {
		if p = strings.TrimSpace(p); p != "" {
			parts = append(parts, p)
		}
	}
	return strings.Join(parts, " | ")
}

// splitLines normalises CRLF and splits, dropping a single trailing empty line.
func splitLines(s string) []string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.TrimRight(s, "\n")
	if s == "" {
		return nil
	}
	return strings.Split(s, "\n")
}
