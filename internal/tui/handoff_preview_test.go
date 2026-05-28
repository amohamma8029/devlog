package tui

import (
	"testing"
)

func TestIsValidFilename(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"valid alphanumeric", "my-handoff", true},
		{"valid with dots", "my.handoff.md", true},
		{"valid with underscores", "my_handoff", true},
		{"valid with hyphens", "2026-05-23T140000Z", true},
		{"empty", "", true},
		{"dot dot traversal", "..", false},
		{"leading dot dot", "../escape", false},
		{"trailing dot dot", "escape/..", false},
		{"nested dot dot", "foo/../bar", false},
		{"forward slash", "path/to/file", false},
		{"backslash", "path\\to\\file", false},
		{"double backslash", "path\\\\to\\\\file", false},
		{"mixed separators", "path/to\\file", false},
		{"dot dot with separator", "../../etc/passwd", false},
		{"just separator", "/", false},
		{"just backslash", "\\", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isValidFilename(tt.input)
			if got != tt.want {
				t.Errorf("isValidFilename(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestHandleSaveToFileRejectsPathTraversal(t *testing.T) {
	m := NewModel(nil, "/tmp/root")
	m.SavePromptOpen = true
	m.SaveInput = "../../escape"
	m.HandoffContent = "test content"

	result, _ := m.handleSaveToFile()
	updated, ok := result.(Model)
	if !ok {
		t.Fatal("expected Model from handleSaveToFile")
	}
	if updated.SavePromptOpen {
		t.Error("expected SavePromptOpen to be false after rejection")
	}
	if updated.SaveInput != "" {
		t.Errorf("expected SaveInput to be empty, got %q", updated.SaveInput)
	}
	if updated.ErrorMessage == "" {
		t.Error("expected ErrorMessage to be set")
	}
}

func TestHandleSaveToFileRejectsForwardSlash(t *testing.T) {
	m := NewModel(nil, "/tmp/root")
	m.SavePromptOpen = true
	m.SaveInput = "path/to/file"
	m.HandoffContent = "test content"

	result, _ := m.handleSaveToFile()
	updated, ok := result.(Model)
	if !ok {
		t.Fatal("expected Model from handleSaveToFile")
	}
	if updated.SavePromptOpen {
		t.Error("expected SavePromptOpen to be false after rejection")
	}
	if updated.ErrorMessage == "" {
		t.Error("expected ErrorMessage to be set")
	}
}

func TestHandleSaveToFileRejectsBackslash(t *testing.T) {
	m := NewModel(nil, "/tmp/root")
	m.SavePromptOpen = true
	m.SaveInput = "path\\to\\file"
	m.HandoffContent = "test content"

	result, _ := m.handleSaveToFile()
	updated, ok := result.(Model)
	if !ok {
		t.Fatal("expected Model from handleSaveToFile")
	}
	if updated.SavePromptOpen {
		t.Error("expected SavePromptOpen to be false after rejection")
	}
	if updated.ErrorMessage == "" {
		t.Error("expected ErrorMessage to be set")
	}
}

func TestHandleSaveToFileRejectsEmptyFilename(t *testing.T) {
	m := NewModel(nil, "/tmp/root")
	m.SavePromptOpen = true
	m.SaveInput = "   "
	m.HandoffContent = "test content"

	result, _ := m.handleSaveToFile()
	updated, ok := result.(Model)
	if !ok {
		t.Fatal("expected Model from handleSaveToFile")
	}
	if updated.SavePromptOpen {
		t.Error("expected SavePromptOpen to be false after rejection")
	}
	if updated.ErrorMessage == "" {
		t.Error("expected ErrorMessage to be set")
	}
}
