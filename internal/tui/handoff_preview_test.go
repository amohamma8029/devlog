package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	xansi "github.com/charmbracelet/x/ansi"
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

func TestRenderSavePromptConstrainsBorderedWidth(t *testing.T) {
	p := NewCommandPalette()
	p.CursorVisible = true
	m := Model{
		Width:     40,
		SaveInput: strings.Repeat("a", 100) + "tail",
		Palette:   &p,
	}

	prompt := renderSavePrompt(m)
	lines := strings.Split(prompt, "\n")
	if len(lines) != 3 {
		t.Fatalf("renderSavePrompt returned %d lines, want 3:\n%s", len(lines), prompt)
	}

	wantWidth := previewLineWidth(m.Width)
	for i, line := range lines {
		if got := xansi.StringWidth(line); got != wantWidth {
			t.Fatalf("line %d width = %d, want %d: %q", i, got, wantWidth, line)
		}
	}

	if got := strings.Count(lines[1], "│"); got != 2 {
		t.Fatalf("prompt content line has %d side borders, want 2: %q", got, lines[1])
	}

	content := xansi.Strip(lines[1])
	if !strings.Contains(content, inputOverflowMarker) {
		t.Fatalf("overflowed prompt should show overflow marker, got %q", content)
	}
	if !strings.Contains(content, "tail|") {
		t.Fatalf("overflowed prompt should keep input tail and cursor visible, got %q", content)
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

func TestHandleSaveToFileRejectsExistingFile(t *testing.T) {
	dir := t.TempDir()

	handoffDir := filepath.Join(dir, ".devlog", "handoffs")
	if err := os.MkdirAll(handoffDir, 0755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}

	savePath := filepath.Join(handoffDir, "my-handoff.md")
	if err := os.WriteFile(savePath, []byte("existing content"), 0644); err != nil {
		t.Fatalf("write fixture failed: %v", err)
	}

	m := NewModel(nil, dir)
	m.SavePromptOpen = true
	m.SaveInput = "my-handoff"
	m.HandoffContent = "new content"

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
	if !stringsHasPrefix(updated.ErrorMessage, "Handoff file already exists") {
		t.Errorf("expected error about file existing, got %q", updated.ErrorMessage)
	}

	fi, err := os.Stat(savePath)
	if err != nil {
		t.Fatalf("expected existing file to still exist: %v", err)
	}
	if fi.Size() != int64(len("existing content")) {
		t.Errorf("expected existing file content to be unchanged, got size %d", fi.Size())
	}
}

func TestPrepareHandoffPreviewMarkdownTruncatesPerFile(t *testing.T) {
	var b strings.Builder
	b.WriteString("## Changes\n\n")
	b.WriteString("#### src/big.go\n\n")
	b.WriteString("```diff\n")
	for i := 0; i < handoffPreviewDiffLineLimit+2; i++ {
		b.WriteString("+line\n")
	}
	b.WriteString("```\n")

	preview := prepareHandoffPreviewMarkdown(b.String())
	if !strings.Contains(preview, "#### "+handoffDiffExpandedMarker+" src/big.go") {
		t.Fatalf("expected expanded disclosure marker in preview, got:\n%s", preview)
	}
	if !strings.Contains(preview, "... (truncated, 2 more lines)") {
		t.Fatalf("expected per-file truncation message, got:\n%s", preview)
	}
	if strings.Count(preview, "+line") != handoffPreviewDiffLineLimit {
		t.Fatalf("preview should include exactly %d diff lines, got %d", handoffPreviewDiffLineLimit, strings.Count(preview, "+line"))
	}
}

func TestRenderHandoffBodyUsesDisclosureArrowWithoutBullet(t *testing.T) {
	m := NewModel(nil, "/tmp/root")
	m.HandoffContent = "#### src/file.go\n\n```diff\n+line\n```"
	m.Width = 80
	m.Height = 24

	rendered := xansi.Strip(renderHandoffBody(m))
	if strings.Contains(rendered, "• "+handoffDiffExpandedMarker+" src/file.go") {
		t.Fatalf("preview heading should not include bullet before disclosure arrow, got:\n%s", rendered)
	}
	if !strings.Contains(rendered, handoffDiffExpandedMarker+" src/file.go") {
		t.Fatalf("preview heading should include disclosure arrow, got:\n%s", rendered)
	}
}

func TestHandoffMarkdownForSaveOmitsCollapsedDiffsWithoutTruncatingExpanded(t *testing.T) {
	var b strings.Builder
	b.WriteString("## Changes\n\n")
	b.WriteString("#### src/keep.go\n\n")
	b.WriteString("```diff\n")
	for i := 0; i < handoffPreviewDiffLineLimit+1; i++ {
		b.WriteString("+keep\n")
	}
	b.WriteString("```\n\n")
	b.WriteString("#### src/drop.go\n\n")
	b.WriteString("```diff\n")
	b.WriteString("+drop\n")
	b.WriteString("```\n")

	m := NewModel(nil, "/tmp/root")
	m.HandoffContent = b.String()
	m.HandoffCollapsedDiffs = map[string]bool{"src/drop.go": true}

	preview := prepareHandoffPreviewMarkdownForModel(m)
	if !strings.Contains(preview, "#### "+handoffDiffExpandedMarker+" src/keep.go") {
		t.Fatalf("preview should mark expanded diff, got:\n%s", preview)
	}
	if !strings.Contains(preview, "#### "+handoffDiffCollapsedMarker+" src/drop.go") {
		t.Fatalf("preview should mark collapsed diff, got:\n%s", preview)
	}
	if !strings.Contains(preview, "Click heading to expand") {
		t.Fatalf("collapsed preview should include expansion hint, got:\n%s", preview)
	}

	saved := handoffMarkdownForSave(m)
	if !strings.Contains(saved, "#### src/keep.go") {
		t.Fatalf("save output should include expanded diff heading, got:\n%s", saved)
	}
	if strings.Contains(saved, handoffDiffExpandedMarker) || strings.Contains(saved, handoffDiffCollapsedMarker) {
		t.Fatalf("save output should not include preview disclosure markers, got:\n%s", saved)
	}
	if strings.Count(saved, "+keep") != handoffPreviewDiffLineLimit+1 {
		t.Fatalf("save output should not truncate expanded diff, got %d lines", strings.Count(saved, "+keep"))
	}
	if strings.Contains(saved, "src/drop.go") || strings.Contains(saved, "+drop") {
		t.Fatalf("save output should omit collapsed diff, got:\n%s", saved)
	}
	if strings.Contains(saved, "truncated") {
		t.Fatalf("save output should not include preview truncation marker, got:\n%s", saved)
	}
}

func TestToggleAllHandoffDiffsCollapsesAndExpands(t *testing.T) {
	m := NewModel(nil, "/tmp/root")
	m.HandoffContent = "#### a.go\n\n```diff\n+a\n```\n\n#### b.go\n\n```diff\n+b\n```"

	toggleAllHandoffDiffs(&m)
	if len(m.HandoffCollapsedDiffs) != 2 || !m.HandoffCollapsedDiffs["a.go"] || !m.HandoffCollapsedDiffs["b.go"] {
		t.Fatalf("expected all diffs collapsed, got %#v", m.HandoffCollapsedDiffs)
	}

	toggleAllHandoffDiffs(&m)
	if m.HandoffCollapsedDiffs != nil {
		t.Fatalf("expected all diffs expanded, got %#v", m.HandoffCollapsedDiffs)
	}
}

func stringsHasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}
