package tui

import tea "github.com/charmbracelet/bubbletea"

type MouseAction int

const (
	MouseClick MouseAction = iota
	MouseScrollUp
	MouseScrollDown
)

func ParseMouseEvent(msg tea.MouseMsg) MouseAction {
	switch msg.Button {
	case tea.MouseButtonWheelUp:
		return MouseScrollUp
	case tea.MouseButtonWheelDown:
		return MouseScrollDown
	default:
		return MouseClick
	}
}
