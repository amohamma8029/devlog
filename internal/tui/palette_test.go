package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	xansi "github.com/charmbracelet/x/ansi"
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

func TestCommandPaletteViewConstrainsInputBoxWidth(t *testing.T) {
	p := NewCommandPalette()
	p.Open = true
	p.CursorVisible = true
	p.Input = strings.Repeat("a", 100) + "tail"

	v := p.View()
	lines := strings.Split(v, "\n")
	if len(lines) < 4 {
		t.Fatalf("palette view returned too few lines:\n%s", v)
	}

	menuWidth := xansi.StringWidth(lines[0])
	inputLines := lines[len(lines)-3:]
	for i, line := range inputLines {
		if got := xansi.StringWidth(line); got != menuWidth {
			t.Fatalf("input line %d width = %d, want %d: %q", i, got, menuWidth, line)
		}
	}

	if got := strings.Count(inputLines[1], "│"); got != 2 {
		t.Fatalf("palette input line has %d side borders, want 2: %q", got, inputLines[1])
	}

	content := xansi.Strip(inputLines[1])
	if !strings.Contains(content, inputOverflowMarker) {
		t.Fatalf("overflowed palette input should show overflow marker, got %q", content)
	}
	if !strings.Contains(content, "tail|") {
		t.Fatalf("overflowed palette input should keep input tail and cursor visible, got %q", content)
	}
}

func TestSlashOpensCommandPaletteWithSlashPrefilled(t *testing.T) {
	m := testModel()
	m.CurrentView = ActiveSession
	m.ActiveSession = testActiveSession()

	updatedModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	updated, ok := updatedModel.(Model)
	if !ok {
		t.Fatalf("expected Model from Update, got %T", updatedModel)
	}
	if updated.Palette == nil || !updated.Palette.Open {
		t.Fatal("pressing / should open the command palette")
	}
	if updated.Palette.Input != "/" {
		t.Fatalf("Palette.Input = %q, want /", updated.Palette.Input)
	}
	if !strings.Contains(updated.View(), "/") {
		t.Fatalf("rendered view should show prefilled slash, got:\n%s", updated.View())
	}
	if cmd == nil {
		t.Fatal("pressing / should start cursor tick command")
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
	if !np.MultiLine {
		t.Error("/note selection should enter multi-line mode")
	}
	if np.MultiLineIsBlocker {
		t.Error("/note should not be a blocker")
	}
	if len(np.MultiLineLines) != 1 || np.MultiLineLines[0] != "" {
		t.Errorf("MultiLineLines = %v, want [\"\"]", np.MultiLineLines)
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

func TestVisiblePaletteCommandsActiveSession(t *testing.T) {
	p := NewCommandPalette()
	p.Open = true
	p.Input = "/"
	p.SessionClosed = false
	visible := visiblePaletteCommands(p)
	if len(visible) != len(PaletteCommands) {
		t.Errorf("active session should show all commands, got %d, want %d", len(visible), len(PaletteCommands))
	}
}

func TestVisiblePaletteCommandsClosedSession(t *testing.T) {
	p := NewCommandPalette()
	p.Open = true
	p.Input = "/"
	p.SessionClosed = true
	visible := visiblePaletteCommands(p)
	if len(visible) != 2 {
		t.Errorf("closed session should show 2 commands, got %d", len(visible))
	}
	for _, cmd := range visible {
		if cmd.Command == "/note" || cmd.Command == "/block" || cmd.Command == "/close" {
			t.Errorf("closed session should not show %s", cmd.Command)
		}
	}
}

func TestVisiblePaletteCommandsFilter(t *testing.T) {
	p := NewCommandPalette()
	p.Open = true
	p.Input = "/n"
	p.SessionClosed = false
	visible := visiblePaletteCommands(p)
	if len(visible) != 1 {
		t.Fatalf("filter /n should show 1 command, got %d", len(visible))
	}
	if visible[0].Command != "/note" {
		t.Errorf("filter /n should show /note, got %s", visible[0].Command)
	}
}

func TestVisiblePaletteCommandsNoMatch(t *testing.T) {
	p := NewCommandPalette()
	p.Open = true
	p.Input = "/xyz"
	p.SessionClosed = false
	visible := visiblePaletteCommands(p)
	if len(visible) != 0 {
		t.Errorf("filter /xyz should show 0 commands, got %d", len(visible))
	}
}
