package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestCommandPaletteViewClosed(t *testing.T) {
	p := CommandPalette{Open: false, Input: "test"}
	v := p.View()
	if v != "" {
		t.Errorf("View() when closed = %q, want empty string", v)
	}
}

func TestCommandPaletteViewOpen(t *testing.T) {
	p := CommandPalette{Open: true, Input: "hello"}
	v := p.View()
	if !strings.Contains(v, "hello") {
		t.Errorf("View() = %q, should contain 'hello'", v)
	}
}

func TestCommandPaletteUpdateEscCloses(t *testing.T) {
	p := CommandPalette{Open: true, Input: "abc"}
	msg := tea.KeyMsg{Type: tea.KeyEsc}
	np, _ := p.Update(msg)
	if np.Open {
		t.Error("palette should be closed after Esc")
	}
}

func TestCommandPaletteUpdateEnterAppendsHistory(t *testing.T) {
	p := CommandPalette{Open: true, Input: "note test"}
	msg := tea.KeyMsg{Type: tea.KeyEnter}
	np, _ := p.Update(msg)
	if np.Open {
		t.Error("palette should be closed after Enter")
	}
	if np.Input != "" {
		t.Errorf("Input after Enter = %q, want empty", np.Input)
	}
	if len(np.History) != 1 {
		t.Fatalf("History length = %d, want 1", len(np.History))
	}
	if np.History[0] != "note test" {
		t.Errorf("History[0] = %q, want 'note test'", np.History[0])
	}
}

func TestCommandPaletteUpdateEnterEmptySkips(t *testing.T) {
	p := CommandPalette{Open: true, Input: ""}
	msg := tea.KeyMsg{Type: tea.KeyEnter}
	np, _ := p.Update(msg)
	if np.Open != true {
		t.Error("palette should stay open on empty input")
	}
	if len(np.History) != 0 {
		t.Error("empty input should not be added to history")
	}
}

func TestCommandPaletteUpdateBackspace(t *testing.T) {
	p := CommandPalette{Open: true, Input: "abc"}
	msg := tea.KeyMsg{Type: tea.KeyBackspace}
	np, _ := p.Update(msg)
	if np.Input != "ab" {
		t.Errorf("Input after backspace = %q, want 'ab'", np.Input)
	}
}

func TestCommandPaletteUpdateIgnoreWhenClosed(t *testing.T) {
	p := CommandPalette{Open: false, Input: "abc"}
	msg := tea.KeyMsg{Type: tea.KeyBackspace}
	np, _ := p.Update(msg)
	if np.Input != "abc" {
		t.Error("Input should not change when palette is closed")
	}
}

func TestCommandPaletteUpdateWithRunes(t *testing.T) {
	p := CommandPalette{Open: true, Input: "a"}
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'b'}}
	np, _ := p.Update(msg)
	if np.Input != "ab" {
		t.Errorf("Input = %q, want 'ab'", np.Input)
	}
}

func TestNewCommandPalette(t *testing.T) {
	p := NewCommandPalette()
	if p.Open {
		t.Error("NewCommandPalette should start closed")
	}
	if p.Input != "" {
		t.Error("NewCommandPalette should start with empty input")
	}
	if len(p.History) != 0 {
		t.Error("NewCommandPalette should start with empty history")
	}
}
