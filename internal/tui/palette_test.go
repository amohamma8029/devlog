package tui

import (
	"strings"
	"testing"

	"github.com/atotto/clipboard"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
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
	p.InputCursorPos = len(p.Input)

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
	if !strings.Contains(content, "tail") {
		t.Fatalf("overflowed palette input should keep input tail visible, got %q", content)
	}
}

func TestCommandPaletteMultiLineComposerStylesBlocker(t *testing.T) {
	p := NewCommandPalette()
	p.Open = true
	p.CursorVisible = true
	p.EnterMultiLine(true)
	p.MultiLineLines = []string{"blocked"}
	p.MultiLineCursorCol = len("blocked")

	v := p.View()
	if !strings.Contains(v, ComposerBlockerTokenStyle.Render("/block")) {
		t.Fatalf("blocker composer should render /block with blocker token style, got:\n%s", v)
	}
	if !strings.Contains(v, ComposerBodyStyle.Render("blocked")) {
		t.Fatalf("blocker composer should render body with composer body style, got:\n%s", v)
	}
}

func TestComposerBlockerTokenStyleUsesRedBoxWithWhiteText(t *testing.T) {
	if got, want := ComposerBlockerTokenStyle.GetBackground(), lipgloss.Color("#AA0000"); got != want {
		t.Fatalf("blocker token background = %v, want %v", got, want)
	}
	if got, want := ComposerBlockerTokenStyle.GetForeground(), lipgloss.Color("#FFFFFF"); got != want {
		t.Fatalf("blocker token foreground = %v, want %v", got, want)
	}
}

func TestCommandPaletteMultiLineComposerStylesNoteBody(t *testing.T) {
	p := NewCommandPalette()
	p.Open = true
	p.CursorVisible = true
	p.EnterMultiLine(false)
	p.MultiLineLines = []string{"note body"}
	p.MultiLineCursorCol = len("note body")

	v := p.View()
	if !strings.Contains(v, MenuSelectedStyle.Render("/note")) {
		t.Fatalf("note composer should keep selected token style, got:\n%s", v)
	}
	if !strings.Contains(v, ComposerBodyStyle.Render("note body")) {
		t.Fatalf("note composer should render body with composer body style, got:\n%s", v)
	}
}

func TestCommandPaletteMultiLineShortcutHintAdaptsToWidth(t *testing.T) {
	p := NewCommandPalette()
	p.Open = true
	p.EnterMultiLine(false)
	p.SetWidth(30)

	v := p.View()
	if strings.Contains(xansi.Strip(v), "Alt+Enter new line") {
		t.Fatalf("narrow composer should use a shorter shortcut hint, got:\n%s", v)
	}
	if !strings.Contains(xansi.Strip(v), "Alt+Enter") || !strings.Contains(xansi.Strip(v), "Esc") {
		t.Fatalf("narrow composer should keep essential shortcut labels, got:\n%s", v)
	}

	for i, line := range strings.Split(v, "\n") {
		if got := xansi.StringWidth(line); got > 30 {
			t.Fatalf("composer line %d width = %d, want <= 30: %q", i, got, line)
		}
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
	p.InputCursorPos = len(p.Input)
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
	p.InputCursorPos = len(p.Input)
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

func TestMultiLineShiftLeftSelectsText(t *testing.T) {
	p := NewCommandPalette()
	p.Open = true
	p.EnterMultiLine(false)
	p.MultiLineLines = []string{"hello"}
	p.MultiLineCursorCol = 5

	msg := tea.KeyMsg{Type: tea.KeyShiftLeft}
	_, np := p.Update(msg)
	if !np.HasSelection {
		t.Fatal("shift+left should start selection")
	}
	if np.MultiLineCursorCol != 4 {
		t.Fatalf("cursor col = %d, want 4", np.MultiLineCursorCol)
	}

	sr, sc, er, ec := np.normalizedSelection()
	if sr != 0 || sc != 4 || er != 0 || ec != 5 {
		t.Fatalf("normalized selection = (%d, %d, %d, %d), want (0, 4, 0, 5)", sr, sc, er, ec)
	}
}

func TestMultiLineShiftRightSelectsText(t *testing.T) {
	p := NewCommandPalette()
	p.Open = true
	p.EnterMultiLine(false)
	p.MultiLineLines = []string{"hello"}
	p.MultiLineCursorCol = 0

	msg := tea.KeyMsg{Type: tea.KeyShiftRight}
	_, np := p.Update(msg)
	if !np.HasSelection {
		t.Fatal("shift+right should start selection")
	}
	if np.MultiLineCursorCol != 1 {
		t.Fatalf("cursor col = %d, want 1", np.MultiLineCursorCol)
	}
}

func TestMultiLineUnshiftedArrowClearsSelection(t *testing.T) {
	p := NewCommandPalette()
	p.Open = true
	p.EnterMultiLine(false)
	p.MultiLineLines = []string{"hello"}
	p.MultiLineCursorCol = 5

	_, np := p.Update(tea.KeyMsg{Type: tea.KeyShiftLeft})
	if !np.HasSelection {
		t.Fatal("shift+left should start selection")
	}

	_, np = np.Update(tea.KeyMsg{Type: tea.KeyLeft})
	if np.HasSelection {
		t.Fatal("unshifted left should clear selection")
	}
}

func TestMultiLineBackspaceDeletesSelection(t *testing.T) {
	p := NewCommandPalette()
	p.Open = true
	p.EnterMultiLine(false)
	p.MultiLineLines = []string{"hello"}
	p.MultiLineCursorCol = 4
	p.HasSelection = true
	p.SelectionAnchorRow = 0
	p.SelectionAnchorCol = 1

	msg := tea.KeyMsg{Type: tea.KeyBackspace}
	_, np := p.Update(msg)
	if np.HasSelection {
		t.Fatal("backspace should clear selection after delete")
	}
	if np.MultiLineLines[0] != "ho" {
		t.Fatalf("lines[0] = %q, want 'ho'", np.MultiLineLines[0])
	}
	if np.MultiLineCursorCol != 1 {
		t.Fatalf("cursor col = %d, want 1", np.MultiLineCursorCol)
	}
}

func TestMultiLineDeleteDeletesSelection(t *testing.T) {
	p := NewCommandPalette()
	p.Open = true
	p.EnterMultiLine(false)
	p.MultiLineLines = []string{"hello"}
	p.MultiLineCursorCol = 3
	p.HasSelection = true
	p.SelectionAnchorRow = 0
	p.SelectionAnchorCol = 3

	_, np := p.Update(tea.KeyMsg{Type: tea.KeyShiftRight})
	// Now selection: row 0, col 3 to row 0, col 4
	if !np.HasSelection || np.MultiLineCursorCol != 4 {
		t.Fatal("shift+right should extend selection")
	}

	msg := tea.KeyMsg{Type: tea.KeyDelete}
	_, np = np.Update(msg)
	if np.HasSelection {
		t.Fatal("delete should clear selection after delete")
	}
	if np.MultiLineLines[0] != "helo" {
		t.Fatalf("lines[0] = %q, want 'helo'", np.MultiLineLines[0])
	}
}

func TestMultiLineTypingReplacesSelection(t *testing.T) {
	p := NewCommandPalette()
	p.Open = true
	p.EnterMultiLine(false)
	p.MultiLineLines = []string{"hello"}
	p.MultiLineCursorCol = 0
	p.HasSelection = true
	p.SelectionAnchorRow = 0
	p.SelectionAnchorCol = 0

	_, np := p.Update(tea.KeyMsg{Type: tea.KeyShiftRight})
	_, np = np.Update(tea.KeyMsg{Type: tea.KeyShiftRight})
	if !np.HasSelection {
		t.Fatal("two shift+right should select 'he'")
	}

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'X'}}
	_, np = np.Update(msg)
	if np.HasSelection {
		t.Fatal("typing should clear selection")
	}
	if np.MultiLineLines[0] != "Xllo" {
		t.Fatalf("lines[0] = %q, want 'Xllo'", np.MultiLineLines[0])
	}
}

func TestMultiLineCtrlCWithSelection(t *testing.T) {
	p := NewCommandPalette()
	p.Open = true
	p.EnterMultiLine(false)
	p.MultiLineLines = []string{"copy me"}
	p.MultiLineCursorCol = 0
	p.HasSelection = true
	p.SelectionAnchorRow = 0
	p.SelectionAnchorCol = 0

	_, np := p.Update(tea.KeyMsg{Type: tea.KeyShiftRight})
	_, np = np.Update(tea.KeyMsg{Type: tea.KeyShiftRight})
	_, np = np.Update(tea.KeyMsg{Type: tea.KeyShiftRight})
	_, np = np.Update(tea.KeyMsg{Type: tea.KeyShiftRight})
	if !np.HasSelection {
		t.Fatal("shift+right x4 should select 'copy'")
	}

	cmd, np := p.Update(tea.KeyMsg{Type: tea.KeyCtrlC})

	testSkipIfClipboardUnavailable(t, cmd, np)
	if !np.HasSelection {
		t.Fatal("ctrl+c should preserve selection")
	}
}

func TestMultiLineCtrlCWithoutSelection(t *testing.T) {
	p := NewCommandPalette()
	p.Open = true
	p.EnterMultiLine(false)
	p.MultiLineLines = []string{"hello"}
	p.MultiLineCursorCol = 0

	cmd, np := p.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if np.Open != true {
		t.Fatal("ctrl+c without selection should keep palette open")
	}
	if cmd != nil {
		t.Fatal("ctrl+c without selection should return nil cmd")
	}
}

func TestMultiLineCtrlXWithSelection(t *testing.T) {
	p := NewCommandPalette()
	p.Open = true
	p.EnterMultiLine(false)
	p.MultiLineLines = []string{"cut me"}
	p.MultiLineCursorCol = 0

	_, np := p.Update(tea.KeyMsg{Type: tea.KeyShiftRight})
	_, np = np.Update(tea.KeyMsg{Type: tea.KeyShiftRight})
	_, np = np.Update(tea.KeyMsg{Type: tea.KeyShiftRight})
	if !np.HasSelection {
		t.Fatal("shift+right x3 should select 'cut'")
	}

	cmd, np := p.Update(tea.KeyMsg{Type: tea.KeyCtrlX})

	testSkipIfClipboardUnavailable(t, cmd, np)
	if np.HasSelection {
		t.Fatal("ctrl+x should clear selection")
	}
	if np.MultiLineLines[0] != " me" {
		t.Fatalf("lines[0] = %q, want ' me'", np.MultiLineLines[0])
	}
}

func TestMultiLineCtrlVInsertsClipboard(t *testing.T) {
	p := NewCommandPalette()
	p.Open = true
	p.EnterMultiLine(false)
	p.MultiLineLines = []string{"before"}
	p.MultiLineCursorCol = 3

	testSetClipboard(t, "PASTE")
	cmd, _ := p.Update(tea.KeyMsg{Type: tea.KeyCtrlV})
	if cmd == nil {
		t.Skip("clipboard unavailable; skipping paste test")
	}

	msg := <-testCmdResult(cmd)
	if paste, ok := msg.(pasteMsg); ok {
		_, np := p.Update(paste)
		expected := "befPASTEore"
		if np.MultiLineLines[0] != expected {
			t.Fatalf("lines[0] = %q, want %q", np.MultiLineLines[0], expected)
		}
		if np.MultiLineCursorCol != 8 {
			t.Fatalf("cursor col = %d, want 8", np.MultiLineCursorCol)
		}
	}
}

func TestMultiLinePasteReplacesFullSelection(t *testing.T) {
	p := NewCommandPalette()
	p.Open = true
	p.EnterMultiLine(false)
	p.MultiLineLines = []string{"old first", "old second"}
	p.MultiLineCursorRow = 0
	p.MultiLineCursorCol = 3

	_, np := p.Update(tea.KeyMsg{Type: tea.KeyCtrlA})
	if !np.HasSelection {
		t.Fatal("Ctrl+A should select the full multi-line body")
	}

	cmd, np := np.Update(pasteMsg{text: "new first\nnew second"})
	if cmd != nil {
		t.Fatal("paste should not return a submit command")
	}
	if np.HasSelection {
		t.Fatal("paste should clear the replaced selection")
	}
	if got, want := strings.Join(np.MultiLineLines, "\n"), "new first\nnew second"; got != want {
		t.Fatalf("multi-line body = %q, want %q", got, want)
	}
	if np.MultiLineCursorRow != 1 || np.MultiLineCursorCol != len("new second") {
		t.Fatalf("cursor = (%d, %d), want (1, %d)", np.MultiLineCursorRow, np.MultiLineCursorCol, len("new second"))
	}
}

func TestMultiLineBracketedPasteInsertsWithoutSubmitting(t *testing.T) {
	p := NewCommandPalette()
	p.Open = true
	p.EnterMultiLine(false)

	cmd, np := p.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("first\nsecond"), Paste: true})
	if cmd != nil {
		t.Fatal("bracketed paste should not submit the multi-line body")
	}
	if !np.Open || !np.MultiLine {
		t.Fatal("bracketed paste should keep the multi-line composer open")
	}
	if got, want := strings.Join(np.MultiLineLines, "\n"), "first\nsecond"; got != want {
		t.Fatalf("multi-line body = %q, want %q", got, want)
	}
	if np.MultiLineCursorRow != 1 || np.MultiLineCursorCol != len("second") {
		t.Fatalf("cursor = (%d, %d), want (1, %d)", np.MultiLineCursorRow, np.MultiLineCursorCol, len("second"))
	}
}

func TestMultiLineShiftHomeSelectsToLineStart(t *testing.T) {
	p := NewCommandPalette()
	p.Open = true
	p.EnterMultiLine(false)
	p.MultiLineLines = []string{"hello"}
	p.MultiLineCursorCol = 3

	_, np := p.Update(tea.KeyMsg{Type: tea.KeyShiftHome})
	if !np.HasSelection {
		t.Fatal("shift+home should start selection")
	}
	if np.MultiLineCursorCol != 0 {
		t.Fatalf("cursor col = %d, want 0", np.MultiLineCursorCol)
	}
	sr, sc, er, ec := np.normalizedSelection()
	if sr != 0 || sc != 0 || er != 0 || ec != 3 {
		t.Fatalf("normalized selection = (%d, %d, %d, %d), want (0, 0, 0, 3)", sr, sc, er, ec)
	}
}

func TestMultiLineShiftEndSelectsToLineEnd(t *testing.T) {
	p := NewCommandPalette()
	p.Open = true
	p.EnterMultiLine(false)
	p.MultiLineLines = []string{"hello"}
	p.MultiLineCursorCol = 1

	_, np := p.Update(tea.KeyMsg{Type: tea.KeyShiftEnd})
	if !np.HasSelection {
		t.Fatal("shift+end should start selection")
	}
	if np.MultiLineCursorCol != 5 {
		t.Fatalf("cursor col = %d, want 5", np.MultiLineCursorCol)
	}
}

func TestMultiLineHomeMovesToLineStart(t *testing.T) {
	p := NewCommandPalette()
	p.Open = true
	p.EnterMultiLine(false)
	p.MultiLineLines = []string{"hello"}
	p.MultiLineCursorCol = 3

	_, np := p.Update(tea.KeyMsg{Type: tea.KeyHome})
	if np.HasSelection {
		t.Fatal("home should clear selection")
	}
	if np.MultiLineCursorCol != 0 {
		t.Fatalf("cursor col = %d, want 0", np.MultiLineCursorCol)
	}
}

func TestMultiLineEndMovesToLineEnd(t *testing.T) {
	p := NewCommandPalette()
	p.Open = true
	p.EnterMultiLine(false)
	p.MultiLineLines = []string{"hello"}
	p.MultiLineCursorCol = 2

	_, np := p.Update(tea.KeyMsg{Type: tea.KeyEnd})
	if np.HasSelection {
		t.Fatal("end should clear selection")
	}
	if np.MultiLineCursorCol != 5 {
		t.Fatalf("cursor col = %d, want 5", np.MultiLineCursorCol)
	}
}

func TestMultiLineCrossLineSelectionUp(t *testing.T) {
	p := NewCommandPalette()
	p.Open = true
	p.EnterMultiLine(false)
	p.MultiLineLines = []string{"line1", "line2"}
	p.MultiLineCursorRow = 1
	p.MultiLineCursorCol = 2

	_, np := p.Update(tea.KeyMsg{Type: tea.KeyShiftUp})
	if !np.HasSelection {
		t.Fatal("shift+up should start selection")
	}
	if np.MultiLineCursorRow != 0 {
		t.Fatalf("cursor row = %d, want 0", np.MultiLineCursorRow)
	}
}

func TestMultiLineCrossLineSelectionDown(t *testing.T) {
	p := NewCommandPalette()
	p.Open = true
	p.EnterMultiLine(false)
	p.MultiLineLines = []string{"line1", "line2"}
	p.MultiLineCursorRow = 0
	p.MultiLineCursorCol = 2

	_, np := p.Update(tea.KeyMsg{Type: tea.KeyShiftDown})
	if !np.HasSelection {
		t.Fatal("shift+down should start selection")
	}
	if np.MultiLineCursorRow != 1 {
		t.Fatalf("cursor row = %d, want 1", np.MultiLineCursorRow)
	}
}

func TestMultiLineSelectedTextSingleLine(t *testing.T) {
	p := NewCommandPalette()
	setMultiLineSelection(&p, "hello", 4, 0, 1)
	got := p.selectedText()
	if got != "ell" {
		t.Fatalf("selectedText = %q, want 'ell'", got)
	}
}

func TestMultiLineSelectedTextMultiLine(t *testing.T) {
	p := NewCommandPalette()
	p.MultiLineLines = []string{"abc", "def", "ghi"}
	p.HasSelection = true
	p.SelectionAnchorRow = 0
	p.SelectionAnchorCol = 1
	p.MultiLineCursorRow = 2
	p.MultiLineCursorCol = 2

	got := p.selectedText()
	if got != "bc\ndef\ngh" {
		t.Fatalf("selectedText = %q, want 'bc\ndef\ngh'", got)
	}
}

func TestMultiLineDeleteSelectionCrossLine(t *testing.T) {
	p := NewCommandPalette()
	p.MultiLineLines = []string{"abc", "def", "ghi"}
	p.HasSelection = true
	p.SelectionAnchorRow = 0
	p.SelectionAnchorCol = 1
	p.MultiLineCursorRow = 2
	p.MultiLineCursorCol = 2

	p.deleteSelection()
	if p.HasSelection {
		t.Fatal("deleteSelection should clear selection")
	}
	if len(p.MultiLineLines) != 1 {
		t.Fatalf("lines length = %d, want 1", len(p.MultiLineLines))
	}
	if p.MultiLineLines[0] != "ai" {
		t.Fatalf("lines[0] = %q, want 'ai'", p.MultiLineLines[0])
	}
}

func TestMultiLineOpenPaletteClearsSelection(t *testing.T) {
	p := NewCommandPalette()
	p.HasSelection = true
	p.SelectionAnchorRow = 0
	p.SelectionAnchorCol = 0

	p.OpenPalette()
	if p.HasSelection {
		t.Fatal("OpenPalette should clear selection")
	}
}

func TestMultiLineClosePaletteClearsSelection(t *testing.T) {
	p := NewCommandPalette()
	p.Open = true
	p.HasSelection = true
	p.SelectionAnchorRow = 0
	p.SelectionAnchorCol = 0

	p.ClosePalette()
	if p.HasSelection {
		t.Fatal("ClosePalette should clear selection")
	}
}

func TestMultiLineEnterMultiLineClearsSelection(t *testing.T) {
	p := NewCommandPalette()
	p.HasSelection = true
	p.SelectionAnchorRow = 0
	p.SelectionAnchorCol = 0

	p.EnterMultiLine(false)
	if p.HasSelection {
		t.Fatal("EnterMultiLine should clear selection")
	}
}

func TestMultiLineViewRendersSelectionStyle(t *testing.T) {
	p := NewCommandPalette()
	p.Open = true
	p.CursorVisible = false
	p.EnterMultiLine(false)
	p.MultiLineLines = []string{"select me"}
	p.HasSelection = true
	p.SelectionAnchorRow = 0
	p.SelectionAnchorCol = 0
	p.MultiLineCursorRow = 0
	p.MultiLineCursorCol = 6

	v := p.View()
	selStyled := ComposerSelectionStyle.Render("select")
	if !strings.Contains(v, selStyled) {
		t.Fatalf("view should contain selected text with selection style, got:\n%s", v)
	}
}

func TestMultiLineViewRendersSelectionAndCursor(t *testing.T) {
	p := NewCommandPalette()
	p.Open = true
	p.CursorVisible = true
	p.EnterMultiLine(false)
	p.MultiLineLines = []string{"select me"}
	p.HasSelection = true
	p.SelectionAnchorRow = 0
	p.SelectionAnchorCol = 0
	p.MultiLineCursorRow = 0
	p.MultiLineCursorCol = 6

	v := p.View()
	if !strings.Contains(v, CursorStyle.Render(" ")) {
		t.Fatalf("view should contain cursor-styled character, got:\n%s", v)
	}
	selStyled := ComposerSelectionStyle.Render("select")
	if !strings.Contains(v, selStyled) {
		t.Fatalf("view should contain selected text with selection style, got:\n%s", v)
	}
}

func TestComposerShortcutHintIncludesClipboard(t *testing.T) {
	hint := composerShortcutHint(200)
	if !strings.Contains(hint, "Ctrl+C") || !strings.Contains(hint, "Ctrl+X") || !strings.Contains(hint, "Ctrl+V") {
		t.Fatalf("wide hint should include clipboard shortcuts, got: %q", hint)
	}
}

func setMultiLineSelection(p *CommandPalette, line string, cursorCol, anchorRow, anchorCol int) {
	p.MultiLineLines = []string{line}
	p.MultiLineCursorRow = 0
	p.MultiLineCursorCol = cursorCol
	p.HasSelection = true
	p.SelectionAnchorRow = anchorRow
	p.SelectionAnchorCol = anchorCol
}

func testSkipIfClipboardUnavailable(t *testing.T, cmd tea.Cmd, p *CommandPalette) {
	t.Helper()
	if cmd == nil {
		t.Skip("clipboard unavailable; skipping clipboard test")
	}
}

func testSetClipboard(t *testing.T, text string) {
	t.Helper()
	original, readErr := clipboard.ReadAll()
	if err := clipboard.WriteAll(text); err != nil {
		t.Skipf("clipboard write unavailable: %v", err)
	}
	t.Cleanup(func() {
		if readErr != nil {
			return
		}
		_ = clipboard.WriteAll(original)
	})
}

func TestCtrlASelectsAllSingleLine(t *testing.T) {
	p := NewCommandPalette()
	p.Open = true
	p.Input = "hello world"
	p.InputCursorPos = 3

	_, np := p.Update(tea.KeyMsg{Type: tea.KeyCtrlA})
	if !np.InputHasSelection {
		t.Fatal("Ctrl+A should set InputHasSelection")
	}
	start, end := np.inputSelBounds()
	if start != 0 || end != 11 {
		t.Fatalf("selection bounds = (%d, %d), want (0, 11)", start, end)
	}
}

func TestCtrlANoOpOnEmptySingleLine(t *testing.T) {
	p := NewCommandPalette()
	p.Open = true
	p.Input = ""

	_, np := p.Update(tea.KeyMsg{Type: tea.KeyCtrlA})
	if np.InputHasSelection {
		t.Fatal("Ctrl+A on empty input should not select")
	}
}

func TestCtrlASelectsAllMultiLine(t *testing.T) {
	p := NewCommandPalette()
	p.Open = true
	p.EnterMultiLine(false)
	p.MultiLineLines = []string{"hello", "world"}
	p.MultiLineCursorRow = 1
	p.MultiLineCursorCol = 2

	_, np := p.Update(tea.KeyMsg{Type: tea.KeyCtrlA})
	if !np.HasSelection {
		t.Fatal("Ctrl+A should set HasSelection")
	}
	if np.SelectionAnchorRow != 0 || np.SelectionAnchorCol != 0 {
		t.Fatalf("anchor = (%d, %d), want (0, 0)", np.SelectionAnchorRow, np.SelectionAnchorCol)
	}
	if np.MultiLineCursorRow != 1 || np.MultiLineCursorCol != 5 {
		t.Fatalf("cursor = (%d, %d), want (1, 5)", np.MultiLineCursorRow, np.MultiLineCursorCol)
	}
	sr, sc, er, ec := np.normalizedSelection()
	if sr != 0 || sc != 0 || er != 1 || ec != 5 {
		t.Fatalf("normalized selection = (%d, %d, %d, %d), want (0, 0, 1, 5)", sr, sc, er, ec)
	}
}

func TestCtrlANoOpOnEmptyMultiLine(t *testing.T) {
	p := NewCommandPalette()
	p.Open = true
	p.EnterMultiLine(false)

	_, np := p.Update(tea.KeyMsg{Type: tea.KeyCtrlA})
	if np.HasSelection {
		t.Fatal("Ctrl+A on empty multi-line should not select")
	}
}

func TestCtrlASelectsAllMultiLineThenCopy(t *testing.T) {
	p := NewCommandPalette()
	p.Open = true
	p.EnterMultiLine(false)
	p.MultiLineLines = []string{"hello", "world"}

	_, np := p.Update(tea.KeyMsg{Type: tea.KeyCtrlA})
	if !np.HasSelection {
		t.Fatal("Ctrl+A should set HasSelection")
	}
	if np.selectedText() != "hello\nworld" {
		t.Fatalf("selected text = %q, want %q", np.selectedText(), "hello\nworld")
	}
}

func TestCtrlASingleLineThenShiftHome(t *testing.T) {
	p := NewCommandPalette()
	p.Open = true
	p.Input = "hello world"
	p.InputCursorPos = 0

	_, np := p.Update(tea.KeyMsg{Type: tea.KeyCtrlA})
	if !np.InputHasSelection {
		t.Fatal("Ctrl+A should set InputHasSelection")
	}

	_, np = np.Update(tea.KeyMsg{Type: tea.KeyShiftHome})
	start, end := np.inputSelBounds()
	if start != 0 || end != 0 {
		t.Fatalf("shift+home after Ctrl+A: bounds = (%d, %d), want (0, 0)", start, end)
	}
}

func testCmdResult(cmd tea.Cmd) <-chan tea.Msg {
	ch := make(chan tea.Msg, 1)
	go func() {
		ch <- cmd()
	}()
	return ch
}
