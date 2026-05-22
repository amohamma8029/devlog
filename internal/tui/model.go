package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/amo/devlog/internal/store"
)

type View int

const (
	SessionList View = iota
	ActiveSession
	HandoffPreview
)

type Model struct {
	CurrentView   View
	ActiveSession *store.SessionRecord
	Palette       CommandPalette
	Store         *store.Store
	Width         int
	Height        int
}

func NewModel(s *store.Store) Model {
	return Model{
		CurrentView: SessionList,
		Palette:     NewCommandPalette(),
		Store:       s,
	}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.Width = msg.Width
		m.Height = msg.Height
		return m, nil

	case tea.KeyMsg:
		if m.Palette.Open {
			palette, cmd := m.Palette.Update(msg)
			m.Palette = palette
			return m, cmd
		}
		switch msg.String() {
		case "ctrl+c", "esc":
			return m, tea.Quit
		}
	}

	return m, nil
}

func (m Model) View() string {
	view := viewForState(m)
	v := "<devlog TUI> Press Esc or Ctrl+C to quit."
	if view != nil {
		v = view.View()
	}
	if m.Palette.Open {
		v += "\n" + m.Palette.View()
	}
	return v
}
