package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/amo/devlog/internal/store"
)

func testActiveSession() *store.SessionRecord {
	return &store.SessionRecord{
		Session: store.Session{
			ID:      "2026-01-15T143022Z",
			Author:  "Test Author",
			Email:   "test@example.com",
			Started: time.Date(2026, 1, 15, 14, 30, 22, 0, time.UTC),
			Branch:  "main",
			Status:  "active",
		},
		Closed: false,
	}
}

func testModel() Model {
	p := NewCommandPalette()
	return Model{
		Palette: &p,
	}
}

func TestRenderActiveSessionShowsID(t *testing.T) {
	m := testModel()
	m.CurrentView = ActiveSession
	m.ActiveSession = testActiveSession()
	m.Title = "Implement auth middleware"
	m.Width = 80
	m.Height = 24
	v := renderActiveSession(m)
	if !strings.Contains(v, "2026-01-15T143022Z") {
		t.Error("renderActiveSession should show session ID")
	}
	if !strings.Contains(v, "Implement auth middleware") {
		t.Error("renderActiveSession should show title")
	}
	titlePos := strings.Index(v, "Implement auth middleware")
	idPos := strings.Index(v, "2026-01-15T143022Z")
	if titlePos < 0 || idPos < 0 {
		t.Fatal("title or ID not found in render output")
	}
	if titlePos > idPos {
		t.Error("title should appear before session ID in header")
	}
}

func TestRenderActiveSessionShowsMetadata(t *testing.T) {
	m := testModel()
	m.CurrentView = ActiveSession
	m.ActiveSession = testActiveSession()
	m.Title = "Test title"
	m.Width = 80
	m.Height = 24
	v := renderActiveSession(m)
	if !strings.Contains(v, "Test Author") {
		t.Error("renderActiveSession should show author")
	}
	if !strings.Contains(v, "main") {
		t.Error("renderActiveSession should show branch")
	}
	if !strings.Contains(v, "Duration") {
		t.Error("renderActiveSession should show duration")
	}
}

func TestRenderActiveSessionNarrowLayout(t *testing.T) {
	m := testModel()
	m.CurrentView = ActiveSession
	m.ActiveSession = testActiveSession()
	m.Title = "Test title"
	m.Width = 60
	m.Height = 24
	v := renderActiveSession(m)
	if !strings.Contains(v, "Author:") {
		t.Error("narrow layout should show Author label")
	}
	if !strings.Contains(v, "Branch:") {
		t.Error("narrow layout should show Branch label")
	}
}

func TestRenderNoSession(t *testing.T) {
	m := testModel()
	m.CurrentView = ActiveSession
	m.Width = 80
	m.Height = 24
	v := renderNoSession(m)
	if !strings.Contains(v, "No active session") {
		t.Error("renderNoSession should show 'No active session'")
	}
}

func TestRenderHelpOverlay(t *testing.T) {
	m := testModel()
	m.Width = 80
	m.Height = 24
	v := renderHelpOverlay(m)
	if !strings.Contains(v, "Keybindings") {
		t.Error("renderHelpOverlay should show Keybindings")
	}
	if !strings.Contains(v, "/") {
		t.Error("renderHelpOverlay should show slash command")
	}
	if !strings.Contains(v, "q") {
		t.Error("renderHelpOverlay should show quit key")
	}
	if !strings.Contains(v, "/note") {
		t.Error("renderHelpOverlay should show /note command")
	}
	if !strings.Contains(v, "/block") {
		t.Error("renderHelpOverlay should show /block command")
	}
	if !strings.Contains(v, "/close") {
		t.Error("renderHelpOverlay should show /close command")
	}
	if !strings.Contains(v, "/handoff") {
		t.Error("renderHelpOverlay should show /handoff command")
	}
}

func TestRenderHandoffPreview(t *testing.T) {
	m := testModel()
	m.CurrentView = HandoffPreview
	m.HandoffContent = "# Handoff Summary\nSome content here"
	m.Width = 80
	m.Height = 24
	v := renderHandoffPreview(m)
	if !strings.Contains(v, "Handoff Preview") {
		t.Error("renderHandoffPreview should show label")
	}
	if !strings.Contains(v, "# Handoff Summary") {
		t.Error("renderHandoffPreview should show content")
	}
}

func TestRenderHandoffPreviewEmpty(t *testing.T) {
	m := testModel()
	m.CurrentView = HandoffPreview
	m.Width = 80
	m.Height = 24
	v := renderHandoffPreview(m)
	if !strings.Contains(v, "No handoff content") {
		t.Error("renderHandoffPreview should show empty message when no content")
	}
}

func TestExtractStartMessage(t *testing.T) {
	events := []store.SessionEvent{
		{Type: "Start", Body: "  Implement auth middleware  "},
		{Type: "Note", Time: "14:30", Body: "Added JWT"},
	}
	title := extractStartMessage(events)
	if title != "Implement auth middleware" {
		t.Errorf("extractStartMessage = %q, want 'Implement auth middleware'", title)
	}
}

func TestExtractStartMessageEmpty(t *testing.T) {
	events := []store.SessionEvent{
		{Type: "Note", Time: "14:30", Body: "Added JWT"},
	}
	title := extractStartMessage(events)
	if title != "" {
		t.Errorf("extractStartMessage = %q, want empty string", title)
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		d    time.Duration
		want string
	}{
		{0, "less than 1m"},
		{29 * time.Second, "less than 1m"},
		{30 * time.Second, "1m"},
		{5 * time.Minute, "5m"},
		{90 * time.Minute, "1h 30m"},
		{2 * time.Hour, "2h"},
		{25 * time.Hour, "1d 1h"},
		{48 * time.Hour, "2d"},
	}
	for _, tt := range tests {
		got := formatDuration(tt.d)
		if got != tt.want {
			t.Errorf("formatDuration(%v) = %q, want %q", tt.d, got, tt.want)
		}
	}
}

func TestFormatAuthor(t *testing.T) {
	tests := []struct {
		author, email string
		want          string
	}{
		{"Test", "test@example.com", "Test <test@example.com>"},
		{"Test", "", "Test"},
		{"", "test@example.com", "test@example.com"},
		{"", "", "(unknown)"},
	}
	for _, tt := range tests {
		got := formatAuthor(tt.author, tt.email)
		if got != tt.want {
			t.Errorf("formatAuthor(%q, %q) = %q, want %q", tt.author, tt.email, got, tt.want)
		}
	}
}

func TestFilterNonStartEvents(t *testing.T) {
	events := []store.SessionEvent{
		{Type: "Start", Body: "title"},
		{Type: "Note", Time: "14:30", Body: "note 1"},
		{Type: "Start", Body: "another"},
		{Type: "Blocker", Time: "15:00", Body: "blocked"},
		{Type: "Stop", Time: "16:00", Body: "done"},
	}
	filtered := filterNonStartEvents(events)
	if len(filtered) != 3 {
		t.Fatalf("expected 3 non-Start events, got %d", len(filtered))
	}
	for _, e := range filtered {
		if e.Type == "Start" {
			t.Errorf("filterNonStartEvents should not include Start events, got: %v", e)
		}
	}
}

func TestSplitLines(t *testing.T) {
	lines := splitLines("hello world this is a test of word wrapping", 15)
	if len(lines) < 2 {
		t.Fatalf("expected at least 2 lines for long text, got %d", len(lines))
	}
}

func TestSplitLinesEmpty(t *testing.T) {
	lines := splitLines("", 40)
	if len(lines) == 0 {
		t.Fatal("expected at least 1 line for empty input")
	}
}

func TestModelViewActiveSession(t *testing.T) {
	m := testModel()
	m.CurrentView = ActiveSession
	m.ActiveSession = testActiveSession()
	m.Title = "Test Title"
	m.Width = 80
	m.Height = 24
	v := m.View()
	if !strings.Contains(v, "2026-01-15T143022Z") {
		t.Error("Model.View() should show session ID for active session")
	}
}

func TestModelViewNoSession(t *testing.T) {
	m := testModel()
	m.CurrentView = ActiveSession
	m.Width = 80
	m.Height = 24
	v := m.View()
	if !strings.Contains(v, "No active session") {
		t.Error("Model.View() should show no-session message")
	}
}

func TestModelViewErrorBanner(t *testing.T) {
	m := testModel()
	m.CurrentView = ActiveSession
	m.ErrorMessage = "Something went wrong"
	m.Width = 80
	m.Height = 24
	v := m.View()
	if !strings.Contains(v, "ERROR") {
		t.Error("Model.View() should show error banner")
	}
	if !strings.Contains(v, "Something went wrong") {
		t.Error("Model.View() should show error message")
	}
}

func TestModelViewHelpOverlay(t *testing.T) {
	m := testModel()
	m.CurrentView = ActiveSession
	m.ShowHelp = true
	m.Width = 80
	m.Height = 24
	v := m.View()
	if !strings.Contains(v, "Keybindings") {
		t.Error("Model.View() should show help when ShowHelp is true")
	}
}

func TestModelViewNoSessionMsg(t *testing.T) {
	m := testModel()
	m.CurrentView = ActiveSession
	m.NoSessionMsg = "Use `devlog open` to start a session"
	m.Width = 80
	m.Height = 24
	v := m.View()
	if !strings.Contains(v, "devlog open") {
		t.Error("Model.View() should show no-session hint message")
	}
}

func TestModelViewSessionList(t *testing.T) {
	m := testModel()
	m.CurrentView = SessionList
	m.Width = 80
	m.Height = 24
	v := m.View()
	if v == "" {
		t.Error("Model.View() should return non-empty for SessionList")
	}
}
