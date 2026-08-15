package main

import (
	internalgit "github.com/amohamma8029/devlog/internal/git"
	"github.com/amohamma8029/devlog/internal/store"
	"github.com/amohamma8029/devlog/internal/tui"
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

	cfg, err := loadRuntimeConfig()
	if err != nil {
		return err
	}

	m := tui.NewModelWithConfig(s, root, cfg)
	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())
	if _, err := p.Run(); err != nil {
		return err
	}
	return nil
}
