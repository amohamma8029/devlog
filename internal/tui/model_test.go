package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/amo/devlog/internal/store"
)

func TestModelSatisfiesTeaModel(t *testing.T) {
	var m tea.Model = Model{}
	_ = m
}

func TestNewModel(t *testing.T) {
	s := &store.Store{}
	m := NewModel(s)
	if m.CurrentView != SessionList {
		t.Errorf("NewModel CurrentView = %v, want SessionList", m.CurrentView)
	}
	if m.Store != s {
		t.Error("NewModel Store not set")
	}
}

func TestModelInit(t *testing.T) {
	m := Model{}
	cmd := m.Init()
	if cmd != nil {
		t.Errorf("Init() returned non-nil cmd: %v", cmd)
	}
}

func TestModelUpdateQuitOnEsc(t *testing.T) {
	m := Model{}
	msg := tea.KeyMsg{Type: tea.KeyEsc}
	_, cmd := m.Update(msg)
	if cmd == nil {
		t.Fatal("expected quit cmd on Esc")
	}
}

func TestModelUpdateQuitOnCtrlC(t *testing.T) {
	m := Model{}
	msg := tea.KeyMsg{Type: tea.KeyCtrlC}
	_, cmd := m.Update(msg)
	if cmd == nil {
		t.Fatal("expected quit cmd on Ctrl+C")
	}
}

func TestModelUpdateNoQuitWhenPaletteOpen(t *testing.T) {
	m := Model{
		Palette: CommandPalette{Open: true},
	}
	msg := tea.KeyMsg{Type: tea.KeyEsc}
	_, cmd := m.Update(msg)
	if cmd != nil {
		t.Fatal("expected no quit cmd when palette is open")
	}
}

func TestModelUpdateWindowSize(t *testing.T) {
	m := Model{}
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

func TestModelViewNonEmpty(t *testing.T) {
	m := Model{}
	v := m.View()
	if v == "" {
		t.Error("View() returned empty string")
	}
}

func TestModelViewIncludesPaletteWhenOpen(t *testing.T) {
	m := Model{
		Palette: CommandPalette{Open: true, Input: "test"},
	}
	v := m.View()
	if !strings.Contains(v, "test") {
		t.Errorf("View() does not contain palette input: %s", v)
	}
}
