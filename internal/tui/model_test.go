package tui

import (
	"strings"
	"testing"

	"github.com/amo/devlog/internal/store"
	tea "github.com/charmbracelet/bubbletea"
)

func TestModelSatisfiesTeaModel(t *testing.T) {
	var m tea.Model = Model{}
	_ = m
}

func TestNewModel(t *testing.T) {
	s := &store.Store{}
	m := NewModel(s, "/tmp/test")
	if m.CurrentView != SessionList {
		t.Errorf("NewModel CurrentView = %v, want SessionList", m.CurrentView)
	}
	if m.Store != s {
		t.Error("NewModel Store not set")
	}
	if m.Root != "/tmp/test" {
		t.Errorf("NewModel Root = %s, want /tmp/test", m.Root)
	}
}

func TestModelInit(t *testing.T) {
	s := &store.Store{}
	m := NewModel(s, "/tmp")
	cmd := m.Init()
	if cmd == nil {
		t.Error("Init() returned nil cmd; expected a command to load active session")
	}
}

func TestModelUpdateQuitOnEsc(t *testing.T) {
	s := &store.Store{}
	m := NewModel(s, "/tmp")
	msg := tea.KeyMsg{Type: tea.KeyEsc}
	_, cmd := m.Update(msg)
	if cmd == nil {
		t.Fatal("expected quit cmd on Esc")
	}
}

func TestModelUpdateQuitOnCtrlC(t *testing.T) {
	s := &store.Store{}
	m := NewModel(s, "/tmp")
	msg := tea.KeyMsg{Type: tea.KeyCtrlC}
	_, cmd := m.Update(msg)
	if cmd == nil {
		t.Fatal("expected quit cmd on Ctrl+C")
	}
}

func TestModelUpdateNoQuitWhenPaletteOpen(t *testing.T) {
	p := NewCommandPalette()
	p.Open = true
	m := Model{
		Palette: &p,
	}
	msg := tea.KeyMsg{Type: tea.KeyEsc}
	_, cmd := m.Update(msg)
	if cmd != nil {
		t.Fatal("expected no quit cmd when palette is open")
	}
}

func TestModelUpdateWindowSize(t *testing.T) {
	s := &store.Store{}
	m := NewModel(s, "/tmp")
	msg := tea.WindowSizeMsg{Width: 120, Height: 40}
	nm, _ := m.Update(msg)
	updated, ok := nm.(Model)
	if !ok {
		t.Fatal("expected Model from Update")
	}
	if updated.Width != 120 {
		t.Errorf("Width = %d, want 120", updated.Width)
	}
	if updated.Height != 40 {
		t.Errorf("Height = %d, want 40", updated.Height)
	}
}

func TestModelUpdateHandoffGeneratedClearsScreen(t *testing.T) {
	m := testModel()
	m.CurrentView = ActiveSession
	m.ScrollOffset = 5

	updatedModel, cmd := m.Update(HandoffGeneratedMsg{Content: "# Handoff"})
	updated, ok := updatedModel.(Model)
	if !ok {
		t.Fatalf("expected Model from Update, got %T", updatedModel)
	}
	if updated.CurrentView != HandoffPreview {
		t.Fatalf("CurrentView = %v, want HandoffPreview", updated.CurrentView)
	}
	if updated.ScrollOffset != 0 {
		t.Errorf("ScrollOffset = %d, want 0", updated.ScrollOffset)
	}
	if cmd == nil {
		t.Fatal("expected clear-screen command when entering handoff preview")
	}
}

func TestModelViewNonEmpty(t *testing.T) {
	s := &store.Store{}
	m := NewModel(s, "/tmp")
	v := m.View()
	if v == "" {
		t.Error("View() returned empty string")
	}
}

func TestModelViewIncludesPaletteWhenOpen(t *testing.T) {
	p := NewCommandPalette()
	p.Open = true
	p.Input = "test"
	sess := &store.SessionRecord{
		Session: store.Session{ID: "x", Author: "a", Branch: "b"},
	}
	m := Model{
		CurrentView:   ActiveSession,
		ActiveSession: sess,
		Palette:       &p,
		Width:         80,
		Height:        24,
	}
	v := m.View()
	if !strings.Contains(v, "test") {
		t.Errorf("View() does not contain palette input: %s", v)
	}
}
