package tui

import (
	"strings"
	"testing"
)

func TestViewForStateSessionList(t *testing.T) {
	m := Model{CurrentView: SessionList}
	v := viewForState(m)
	if v != nil {
		t.Fatalf("viewForState returned non-nil for SessionList after stub removal: %v", v)
	}
}

func TestViewForStateActiveSession(t *testing.T) {
	m := Model{CurrentView: ActiveSession}
	v := viewForState(m)
	if v == nil {
		t.Fatal("viewForState returned nil for ActiveSession")
	}
	if !strings.Contains(v.View(), "<ActiveSession>") {
		t.Errorf("viewForState View = %s, want <ActiveSession>", v.View())
	}
}

func TestViewForStateHandoffPreview(t *testing.T) {
	m := Model{CurrentView: HandoffPreview}
	v := viewForState(m)
	if v == nil {
		t.Fatal("viewForState returned nil for HandoffPreview")
	}
	if !strings.Contains(v.View(), "<HandoffPreview>") {
		t.Errorf("viewForState View = %s, want <HandoffPreview>", v.View())
	}
}

func TestViewForStateUnknown(t *testing.T) {
	m := Model{CurrentView: View(999)}
	v := viewForState(m)
	if v != nil {
		t.Errorf("viewForState returned non-nil for unknown view: %v", v)
	}
}
