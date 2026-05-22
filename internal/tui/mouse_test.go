package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestParseMouseEventClick(t *testing.T) {
	msg := tea.MouseMsg{
		X:      10,
		Y:      20,
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonLeft,
	}
	action := ParseMouseEvent(msg)
	if action != MouseClick {
		t.Errorf("ParseMouseEvent = %v, want MouseClick", action)
	}
}

func TestParseMouseEventScrollUp(t *testing.T) {
	msg := tea.MouseMsg{
		X:      5,
		Y:      10,
		Button: tea.MouseButtonWheelUp,
	}
	action := ParseMouseEvent(msg)
	if action != MouseScrollUp {
		t.Errorf("ParseMouseEvent = %v, want MouseScrollUp", action)
	}
}

func TestParseMouseEventScrollDown(t *testing.T) {
	msg := tea.MouseMsg{
		X:      5,
		Y:      10,
		Button: tea.MouseButtonWheelDown,
	}
	action := ParseMouseEvent(msg)
	if action != MouseScrollDown {
		t.Errorf("ParseMouseEvent = %v, want MouseScrollDown", action)
	}
}

func TestParseMouseEventRelease(t *testing.T) {
	msg := tea.MouseMsg{
		X:      10,
		Y:      20,
		Action: tea.MouseActionRelease,
		Button: tea.MouseButtonLeft,
	}
	action := ParseMouseEvent(msg)
	if action != MouseClick {
		t.Errorf("ParseMouseEvent = %v, want MouseClick", action)
	}
}
