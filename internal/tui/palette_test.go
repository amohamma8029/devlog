package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestCommandPaletteViewClosed(t *testing.T) {
	p := NewCommandPalette()
	p.Input = "test"
	v := p.View()
	if v != "" {
		t.Errorf("View() when closed = %q, want empty string", v)
	}
}

func TestCommandPaletteViewOpen(t *testing.T) {
	p := NewCommandPalette()
	p.Open = true
	p.Input = "hello"
	v := p.View()
	if !strings.Contains(v, "hello") {
		t.Errorf("View() = %q, should contain 'hello'", v)
	}
}

func TestCommandPaletteUpdateEscCloses(t *testing.T) {
	p := NewCommandPalette()
	p.Open = true
	p.Input = "abc"
	msg := tea.KeyMsg{Type: tea.KeyEsc}
	_, np := p.Update(msg)
	if np.Open {
		t.Error("palette should be closed after Esc")
	}
}

func TestCommandPaletteUpdateEnterAppendsHistory(t *testing.T) {
	p := NewCommandPalette()
	p.Open = true
	p.Input = "/note test"
	p.SelectedIndex = -1
	msg := tea.KeyMsg{Type: tea.KeyEnter}
	_, np := p.Update(msg)
	if np.Open {
		t.Error("palette should be closed after Enter")
	}
	if np.Input != "" {
		t.Errorf("Input after Enter = %q, want empty", np.Input)
	}
	if len(np.History) != 1 {
		t.Fatalf("History length = %d, want 1", len(np.History))
	}
	if np.History[0] != "/note test" {
		t.Errorf("History[0] = %q, want '/note test'", np.History[0])
	}
}

func TestCommandPaletteUpdateEnterEmptySkips(t *testing.T) {
	p := NewCommandPalette()
	p.Open = true
	p.SelectedIndex = -1
	msg := tea.KeyMsg{Type: tea.KeyEnter}
	_, np := p.Update(msg)
	if np.Open != true {
		t.Error("palette should stay open on empty input")
	}
	if len(np.History) != 0 {
		t.Error("empty input should not be added to history")
	}
}

func TestCommandPaletteUpdateBackspace(t *testing.T) {
	p := NewCommandPalette()
	p.Open = true
	p.Input = "abc"
	msg := tea.KeyMsg{Type: tea.KeyBackspace}
	_, np := p.Update(msg)
	if np.Input != "ab" {
		t.Errorf("Input after backspace = %q, want 'ab'", np.Input)
	}
}

func TestCommandPaletteUpdateIgnoreWhenClosed(t *testing.T) {
	p := NewCommandPalette()
	p.Input = "abc"
	msg := tea.KeyMsg{Type: tea.KeyBackspace}
	_, np := p.Update(msg)
	if np.Input != "abc" {
		t.Error("Input should not change when palette is closed")
	}
}

func TestCommandPaletteUpdateWithRunes(t *testing.T) {
	p := NewCommandPalette()
	p.Open = true
	p.Input = "a"
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'b'}}
	_, np := p.Update(msg)
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

func TestCommandPaletteTypingDeselectsMenu(t *testing.T) {
	p := NewCommandPalette()
	p.Open = true
	p.SelectedIndex = 0
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}}
	_, np := p.Update(msg)
	if np.SelectedIndex != -1 {
		t.Errorf("SelectedIndex after typing = %d, want -1", np.SelectedIndex)
	}
}

func TestCommandPaletteArrowKeysSelectMenu(t *testing.T) {
	p := NewCommandPalette()
	p.Open = true
	p.SelectedIndex = -1
	msg := tea.KeyMsg{Type: tea.KeyDown}
	_, np := p.Update(msg)
	if np.SelectedIndex != 0 {
		t.Errorf("SelectedIndex after down = %d, want 0", np.SelectedIndex)
	}

	msg = tea.KeyMsg{Type: tea.KeyDown}
	_, np = np.Update(msg)
	if np.SelectedIndex != 1 {
		t.Errorf("SelectedIndex after down x2 = %d, want 1", np.SelectedIndex)
	}

	msg = tea.KeyMsg{Type: tea.KeyUp}
	_, np = np.Update(msg)
	if np.SelectedIndex != 0 {
		t.Errorf("SelectedIndex after up = %d, want 0", np.SelectedIndex)
	}
}

func TestCommandPaletteEnterWithMenuSelectionExecutes(t *testing.T) {
	p := NewCommandPalette()
	p.Open = true
	p.SelectedIndex = 2
	msg := tea.KeyMsg{Type: tea.KeyEnter}
	cmd, np := p.Update(msg)
	if np.Open {
		t.Error("palette should close when executing no-arg command from menu")
	}
	if cmd == nil {
		t.Fatal("expected a CommandExecutedMsg cmd")
	}
	if np.SelectedIndex != -1 {
		t.Errorf("SelectedIndex should be -1 after execute, got %d", np.SelectedIndex)
	}
}

func TestCommandPaletteEnterWithMenuSelectionFillsInput(t *testing.T) {
	p := NewCommandPalette()
	p.Open = true
	p.SelectedIndex = 0
	msg := tea.KeyMsg{Type: tea.KeyEnter}
	_, np := p.Update(msg)
	if np.Input != "/note " {
		t.Errorf("Input = %q, want '/note '", np.Input)
	}
	if np.SelectedIndex != -1 {
		t.Errorf("SelectedIndex = %d, want -1 after filling input", np.SelectedIndex)
	}
}

func TestCommandPaletteViewShowsCursor(t *testing.T) {
	p := NewCommandPalette()
	p.Open = true
	p.Input = "/note"
	v := p.View()
	if !strings.Contains(v, "/note") {
		t.Error("view should contain input text")
	}
}

func TestCommandPaletteEnterWithoutSelectionExecutesTypedInput(t *testing.T) {
	p := NewCommandPalette()
	p.Open = true
	p.SelectedIndex = -1
	p.Input = "/note hello world"
	msg := tea.KeyMsg{Type: tea.KeyEnter}
	cmd, np := p.Update(msg)
	if np.Open {
		t.Error("palette should close after Enter with typed input")
	}
	if cmd == nil {
		t.Fatal("expected a CommandExecutedMsg cmd")
	}
	if np.Input != "" {
		t.Errorf("Input should be cleared, got %q", np.Input)
	}
}
