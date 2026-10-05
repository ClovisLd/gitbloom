package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"gitbloom/internal/git"
	"gitbloom/internal/ui"
)

func main() {
	dir, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(os.Stderr, "gitbloom:", err)
		os.Exit(1)
	}
	if len(os.Args) > 1 {
		dir = os.Args[1]
	}

	repo, err := git.Open(dir)
	if err != nil {
		fmt.Fprintln(os.Stderr, "gitbloom: not a git repository:", err)
		os.Exit(1)
	}

	p := tea.NewProgram(
		ui.New(repo).WithInteractive(true),
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
		tea.WithReportFocus(),
	)
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "gitbloom:", err)
		os.Exit(1)
	}
}
