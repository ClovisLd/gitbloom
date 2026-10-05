package ui

import (
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/fsnotify/fsnotify"
)

// setupWatcher creates a watcher on the repository's git dir. Working-tree
// edits don't touch .git, so those still rely on the poll/focus refresh.
func setupWatcher(gitDir string) tea.Cmd {
	return func() tea.Msg {
		w, err := fsnotify.NewWatcher()
		if err != nil {
			return watcherReadyMsg{}
		}
		addGitWatches(w, gitDir)
		return watcherReadyMsg{w: w}
	}
}

func addGitWatches(w *fsnotify.Watcher, gitDir string) {
	if gitDir == "" {
		return
	}
	// The git dir itself catches index/HEAD creation on a fresh repo.
	_ = w.Add(gitDir)
	for _, name := range []string{"index", "HEAD", "ORIG_HEAD", "MERGE_HEAD", "packed-refs"} {
		p := filepath.Join(gitDir, name)
		if _, err := os.Stat(p); err == nil {
			_ = w.Add(p)
		}
	}
	refs := filepath.Join(gitDir, "refs")
	_ = filepath.WalkDir(refs, func(path string, d os.DirEntry, err error) error {
		if err == nil && d.IsDir() {
			_ = w.Add(path)
		}
		return nil
	})
}

// waitForChange blocks until the next filesystem event and reports it.
func waitForChange(w *fsnotify.Watcher) tea.Cmd {
	return func() tea.Msg {
		select {
		case _, ok := <-w.Events:
			if !ok {
				return nil
			}
			return fsChangeMsg{}
		case _, ok := <-w.Errors:
			if !ok {
				return nil
			}
			return fsChangeMsg{}
		}
	}
}
