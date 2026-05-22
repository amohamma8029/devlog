package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type CommandPalette struct {
	Open    bool
	Input   string
	History []string
}

func NewCommandPalette() CommandPalette {
	return CommandPalette{}
}

func (p CommandPalette) Update(msg tea.Msg) (CommandPalette, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok || !p.Open {
		return p, nil
	}

	switch keyMsg.String() {
	case "esc":
		p.Open = false
	case "enter":
		if p.Input != "" {
			p.History = append(p.History, p.Input)
			p.Input = ""
			p.Open = false
		}
	case "backspace":
		if len(p.Input) > 0 {
			p.Input = p.Input[:len(p.Input)-1]
		}
	default:
		if len(keyMsg.Runes) == 1 {
			p.Input += string(keyMsg.Runes[0])
		}
	}
	return p, nil
}

func (p CommandPalette) View() string {
	if !p.Open {
		return ""
	}
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Render(": " + p.Input)
}
