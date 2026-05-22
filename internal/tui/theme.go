package tui

import "github.com/charmbracelet/lipgloss"

var TitleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FFFFFF"))

var ActiveStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#00FF00"))

var InactiveStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#808080"))

var BorderStyle = lipgloss.NewStyle().
	Border(lipgloss.RoundedBorder()).
	Padding(0, 1)
