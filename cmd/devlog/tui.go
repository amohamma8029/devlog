package main

import (
	internalgit "github.com/amo/devlog/internal/git"
	"github.com/amo/devlog/internal/store"
	"github.com/amo/devlog/internal/tui"
	tea "github.com/charmbracelet/bubbletea"
)

var launchTUI = func() error {
	root, err := internalgit.RepoRoot()
	if err != nil {
		return err
	}

	s, err := store.New(root)
	if err != nil {
		return err
	}

	m := tui.NewModel(s)
	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())
	if _, err := p.Run(); err != nil {
		return err
	}
	return nil
}
