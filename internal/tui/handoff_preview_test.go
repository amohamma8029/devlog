package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	xansi "github.com/charmbracelet/x/ansi"

	internalconfig "github.com/amohamma8029/devlog/internal/config"
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
	if !strings.Contains(content, "tail") {
		t.Fatalf("overflowed prompt should keep input tail visible, got %q", content)
	}
	if !strings.Contains(prompt, CursorStyle.Render(" ")) {
		t.Fatalf("overflowed prompt should keep block cursor visible, got %q", prompt)
	}
	if strings.Contains(prompt, CursorStyle.Render("|")) {
		t.Fatalf("overflowed prompt should not render the old bar cursor, got %q", prompt)
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
	for i := 0; i < internalconfig.DefaultHandoffPreviewLineLimit+2; i++ {
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
	if strings.Count(preview, "+line") != internalconfig.DefaultHandoffPreviewLineLimit {
		t.Fatalf("preview should include exactly %d diff lines, got %d", internalconfig.DefaultHandoffPreviewLineLimit, strings.Count(preview, "+line"))
	}
}

func TestPrepareHandoffPreviewMarkdownForModelHonorsConfiguredLimit(t *testing.T) {
	var b strings.Builder
	b.WriteString("## Changes\n\n")
	b.WriteString("#### src/big.go\n\n")
	b.WriteString("```diff\n")
	for i := 0; i < 10; i++ {
		b.WriteString("+line\n")
	}
	b.WriteString("```\n")

	cfg := internalconfig.Default()
	cfg.TUI.HandoffPreview.DiffLineLimit = 3
	m := NewModelWithConfig(nil, "/tmp/root", cfg)
	m.HandoffContent = b.String()

	preview := prepareHandoffPreviewMarkdownForModel(m)
	if strings.Count(preview, "+line") != 3 {
		t.Fatalf("configured limit 3 should truncate to 3 diff lines, got %d", strings.Count(preview, "+line"))
	}
	if !strings.Contains(preview, "... (truncated, 7 more lines)") {
		t.Fatalf("expected truncation marker for 7 remaining lines, got:\n%s", preview)
	}
}

func TestPrepareHandoffPreviewMarkdownForModelUsesDefaultWhenAbsent(t *testing.T) {
	var b strings.Builder
	b.WriteString("## Changes\n\n")
	b.WriteString("#### src/big.go\n\n")
	b.WriteString("```diff\n")
	for i := 0; i < internalconfig.DefaultHandoffPreviewLineLimit+2; i++ {
		b.WriteString("+line\n")
	}
	b.WriteString("```\n")

	m := NewModel(nil, "/tmp/root")
	m.HandoffContent = b.String()

	preview := prepareHandoffPreviewMarkdownForModel(m)
	if strings.Count(preview, "+line") != internalconfig.DefaultHandoffPreviewLineLimit {
		t.Fatalf("default config should truncate to %d diff lines, got %d", internalconfig.DefaultHandoffPreviewLineLimit, strings.Count(preview, "+line"))
	}
	if !strings.Contains(preview, "... (truncated, 2 more lines)") {
		t.Fatalf("expected per-file truncation message with default limit, got:\n%s", preview)
	}
}

func TestPrepareHandoffPreviewMarkdownForModelZeroLimitDisablesTruncation(t *testing.T) {
	var b strings.Builder
	b.WriteString("## Changes\n\n")
	b.WriteString("#### src/big.go\n\n")
	b.WriteString("```diff\n")
	for i := 0; i < 10; i++ {
		b.WriteString("+line\n")
	}
	b.WriteString("```\n")

	cfg := internalconfig.Default()
	cfg.TUI.HandoffPreview.DiffLineLimit = 0
	m := NewModelWithConfig(nil, "/tmp/root", cfg)
	m.HandoffContent = b.String()

	preview := prepareHandoffPreviewMarkdownForModel(m)
	if strings.Count(preview, "+line") != 10 {
		t.Fatalf("zero limit should show all 10 diff lines, got %d", strings.Count(preview, "+line"))
	}
	if strings.Contains(preview, "truncated") {
		t.Fatalf("zero limit should not produce truncation marker, got:\n%s", preview)
	}
}

func TestHandoffMarkdownForSaveNotTruncatedByConfiguredPreviewLimit(t *testing.T) {
	var b strings.Builder
	b.WriteString("## Changes\n\n")
	b.WriteString("#### src/keep.go\n\n")
	b.WriteString("```diff\n")
	for i := 0; i < 10; i++ {
		b.WriteString("+keep\n")
	}
	b.WriteString("```\n")

	cfg := internalconfig.Default()
	cfg.TUI.HandoffPreview.DiffLineLimit = 3
	m := NewModelWithConfig(nil, "/tmp/root", cfg)
	m.HandoffContent = b.String()

	preview := prepareHandoffPreviewMarkdownForModel(m)
	if strings.Count(preview, "+keep") != 3 {
		t.Fatalf("preview should truncate to 3 lines with configured limit, got %d", strings.Count(preview, "+keep"))
	}

	saved := handoffMarkdownForSave(m)
	if strings.Count(saved, "+keep") != 10 {
		t.Fatalf("save output should not be truncated by preview limit, got %d lines", strings.Count(saved, "+keep"))
	}
	if strings.Contains(saved, "truncated") {
		t.Fatalf("save output should not include truncation marker, got:\n%s", saved)
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
	for i := 0; i < internalconfig.DefaultHandoffPreviewLineLimit+1; i++ {
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
	if strings.Count(saved, "+keep") != internalconfig.DefaultHandoffPreviewLineLimit+1 {
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

func TestSearchPromptOpensWithSlash(t *testing.T) {
	m := NewModel(nil, "/tmp/root")
	m.CurrentView = HandoffPreview
	m.HandoffContent = "test content"
	m.Width = 80
	m.Height = 24

	result, _ := handleHandoffKey(&m, "/")
	updated, ok := result.(Model)
	if !ok {
		t.Fatal("expected Model from handleHandoffKey")
	}
	if !updated.Search.Open {
		t.Error("expected Search.Open to be true after /")
	}
	if updated.Search.Query != "" {
		t.Errorf("expected empty Search.Query, got %q", updated.Search.Query)
	}
}

func TestSearchPromptClosesWithEsc(t *testing.T) {
	m := NewModel(nil, "/tmp/root")
	m.CurrentView = HandoffPreview
	m.HandoffContent = "test content"
	m.Search.Open = true
	m.Search.Query = "hello"

	result, _ := handleHandoffKey(&m, "esc")
	updated, ok := result.(Model)
	if !ok {
		t.Fatal("expected Model from handleHandoffKey")
	}
	if updated.Search.Open {
		t.Error("expected Search.Open to be false after esc")
	}
	if updated.Search.Query != "" {
		t.Errorf("expected Search.Query to be cleared, got %q", updated.Search.Query)
	}
}

func TestSearchPromptTyping(t *testing.T) {
	m := NewModel(nil, "/tmp/root")
	m.CurrentView = HandoffPreview
	m.HandoffContent = "test content"
	m.Search.Open = true

	for _, ch := range []string{"h", "e", "l", "l", "o"} {
		result, _ := handleHandoffKey(&m, ch)
		updated, ok := result.(Model)
		if !ok {
			t.Fatal("expected Model from handleHandoffKey")
		}
		m = updated
	}

	if m.Search.Query != "hello" {
		t.Errorf("expected Search.Query to be 'hello', got %q", m.Search.Query)
	}
}

func TestSearchPromptBackspace(t *testing.T) {
	m := NewModel(nil, "/tmp/root")
	m.CurrentView = HandoffPreview
	m.HandoffContent = "test content"
	m.Search.Open = true
	m.Search.Query = "hello"
	m.Search.CursorPos = len(m.Search.Query)

	result, _ := handleHandoffKey(&m, "backspace")
	updated, ok := result.(Model)
	if !ok {
		t.Fatal("expected Model from handleHandoffKey")
	}

	if updated.Search.Query != "hell" {
		t.Errorf("expected Search.Query to be 'hell', got %q", updated.Search.Query)
	}
}

func TestSearchPromptNotWhenSavePromptOpen(t *testing.T) {
	m := NewModel(nil, "/tmp/root")
	m.CurrentView = HandoffPreview
	m.HandoffContent = "test content"
	m.SavePromptOpen = true
	m.SaveInput = "test"

	result, _ := handleHandoffKey(&m, "/")
	updated, ok := result.(Model)
	if !ok {
		t.Fatal("expected Model from handleHandoffKey")
	}
	if updated.Search.Open {
		t.Error("expected Search.Open to remain false when save prompt is open")
	}
}

func TestSearchPromptNotWhenConfirmOpen(t *testing.T) {
	m := NewModel(nil, "/tmp/root")
	m.CurrentView = HandoffPreview
	m.HandoffContent = "test content"
	m.CollapsedDiffConfirmOpen = true

	result, _ := handleHandoffKey(&m, "/")
	updated, ok := result.(Model)
	if !ok {
		t.Fatal("expected Model from handleHandoffKey")
	}
	if updated.Search.Open {
		t.Error("expected Search.Open to remain false when confirm is open")
	}
}

func TestRenderSearchPrompt(t *testing.T) {
	p := NewCommandPalette()
	p.CursorVisible = true
	m := NewModel(nil, "/tmp/root")
	m.Palette = &p
	m.Width = 80
	m.Search.Open = true
	m.Search.Query = "test"

	rendered := renderSearchPrompt(m, 0)
	if !strings.Contains(rendered, "Search:") {
		t.Errorf("expected 'Search:' in search prompt, got %q", rendered)
	}
	if !strings.Contains(rendered, "test") {
		t.Errorf("expected query 'test' in search prompt, got %q", rendered)
	}
}

func TestRenderSearchPromptUsesBlockCursorAtEnd(t *testing.T) {
	p := NewCommandPalette()
	p.CursorVisible = true
	m := NewModel(nil, "/tmp/root")
	m.Palette = &p
	m.Width = 80
	m.Search.Open = true
	m.Search.Query = "test"
	m.Search.CursorPos = len([]rune(m.Search.Query))

	rendered := renderSearchPrompt(m, 1)
	if !strings.Contains(rendered, CursorStyle.Render(" ")) {
		t.Fatalf("search prompt should render the shared block cursor at end of input, got %q", rendered)
	}
	if strings.Contains(rendered, CursorStyle.Render("|")) {
		t.Fatalf("search prompt should not render the old bar cursor, got %q", rendered)
	}
}

func TestHandoffPreviewSearchAbsorbsQAsInput(t *testing.T) {
	m := NewModel(nil, "/tmp/root")
	m.CurrentView = HandoffPreview
	m.ActiveSession = nil
	m.HandoffContent = "test content"
	m.Search.Open = true

	result, _ := handleHandoffKey(&m, "q")
	updated, ok := result.(Model)
	if !ok {
		t.Fatal("expected Model from handleHandoffKey")
	}
	if updated.CurrentView != HandoffPreview {
		t.Error("expected to stay in handoff preview; q should be search input, not exit")
	}
	if updated.Search.Query != "q" {
		t.Errorf("expected Search.Query to be 'q', got %q", updated.Search.Query)
	}
}

func TestHandoffPreviewContentLinesReservesSearchPrompt(t *testing.T) {
	m := NewModel(nil, "/tmp/root")
	m.HandoffContent = "## test\n\nsome content"
	m.Width = 80
	m.Height = 24

	withoutSearch := handoffPreviewContentLines(m)

	m.Search.Open = true
	m.Search.Query = "test"
	withSearch := handoffPreviewContentLines(m)

	if withSearch >= withoutSearch {
		t.Errorf("expected content lines to shrink when search prompt is open: without=%d, with=%d", withoutSearch, withSearch)
	}
}

func TestHandoffPreviewSearchEnterAdvancesMatches(t *testing.T) {
	m := testHandoffSearchNavigationModel([]string{
		"intro",
		"first needle",
		"filler",
		"second needle",
		"more filler",
		"third needle",
	})

	result, _ := handleHandoffKey(&m, "enter")
	updated, ok := result.(Model)
	if !ok {
		t.Fatal("expected Model from handleHandoffKey")
	}
	assertSearchMatchPosition(t, updated, 0, 1, "1/3 matches")

	result, _ = handleHandoffKey(&updated, "enter")
	updated = result.(Model)
	assertSearchMatchPosition(t, updated, 1, 3, "2/3 matches")

	result, _ = handleHandoffKey(&updated, "enter")
	updated = result.(Model)
	assertSearchMatchPosition(t, updated, 2, 5, "3/3 matches")

	result, _ = handleHandoffKey(&updated, "enter")
	updated = result.(Model)
	assertSearchMatchPosition(t, updated, 0, 1, "1/3 matches")
}

func TestHandoffPreviewSearchEnterNoMatchesDoesNotJump(t *testing.T) {
	m := testHandoffSearchNavigationModel([]string{"intro", "body", "tail"})
	m.Search.Query = "missing"
	m.Search.CursorPos = len([]rune(m.Search.Query))
	m.ScrollOffset = 2

	result, _ := handleHandoffKey(&m, "enter")
	updated, ok := result.(Model)
	if !ok {
		t.Fatal("expected Model from handleHandoffKey")
	}
	if updated.ScrollOffset != 2 {
		t.Fatalf("ScrollOffset = %d, want unchanged 2", updated.ScrollOffset)
	}
	if updated.Search.MatchIndex != -1 {
		t.Fatalf("MatchIndex = %d, want -1", updated.Search.MatchIndex)
	}
	if len(updated.Search.Matches) != 0 {
		t.Fatalf("Matches length = %d, want 0", len(updated.Search.Matches))
	}
	if prompt := renderSearchPrompt(updated, len(updated.Search.Matches)); !strings.Contains(prompt, "No matches") {
		t.Fatalf("search prompt should show no matches, got %q", prompt)
	}
}

func TestHandoffPreviewSearchEditResetsMatchSelection(t *testing.T) {
	m := testHandoffSearchNavigationModel([]string{"needle", "needle"})
	m.Search.Matches = findSearchMatches(m.Search.Query, handoffBodyLines(m))
	m.Search.MatchIndex = 1

	result, _ := handleHandoffKey(&m, "s")
	updated, ok := result.(Model)
	if !ok {
		t.Fatal("expected Model from handleHandoffKey")
	}
	if updated.Search.Query != "needles" {
		t.Fatalf("Search.Query = %q, want %q", updated.Search.Query, "needles")
	}
	if updated.Search.MatchIndex != -1 {
		t.Fatalf("MatchIndex = %d, want -1", updated.Search.MatchIndex)
	}
	if updated.Search.Matches != nil {
		t.Fatalf("Matches = %#v, want nil", updated.Search.Matches)
	}
}

func TestRenderHandoffPreviewUsesActiveSearchMatchStyle(t *testing.T) {
	m := testHandoffSearchNavigationModel([]string{"first needle", "second needle"})
	m.Height = 10
	m.Search.Matches = findSearchMatches(m.Search.Query, handoffBodyLines(m))
	m.Search.MatchIndex = 1

	rendered := renderHandoffPreview(m)
	matchPrefix, _ := searchMatchStyleCodes()
	activePrefix, _ := activeSearchMatchStyleCodes()
	if !strings.Contains(rendered, matchPrefix) {
		t.Fatalf("rendered preview should include regular match style, got %q", rendered)
	}
	if !strings.Contains(rendered, activePrefix) {
		t.Fatalf("rendered preview should include active match style, got %q", rendered)
	}
	if stripped := xansi.Strip(rendered); !strings.Contains(stripped, "first needle") || !strings.Contains(stripped, "second needle") {
		t.Fatalf("active styling should preserve preview text, got %q", stripped)
	}
}

func testHandoffSearchNavigationModel(bodyLines []string) Model {
	m := NewModel(nil, "/tmp/root")
	m.CurrentView = HandoffPreview
	m.HandoffContent = "content"
	m.Width = 80
	m.Height = 6
	m.Search.Open = true
	m.Search.Query = "needle"
	m.Search.CursorPos = len([]rune(m.Search.Query))
	m.Search.MatchIndex = -1
	m.handoffBodyLines = bodyLines
	m.handoffBodyLineWidth = previewLineWidth(m.Width)
	return m
}

func assertSearchMatchPosition(t *testing.T, m Model, wantIndex, wantScroll int, wantPrompt string) {
	t.Helper()
	if len(m.Search.Matches) != 3 {
		t.Fatalf("Matches length = %d, want 3", len(m.Search.Matches))
	}
	if m.Search.MatchIndex != wantIndex {
		t.Fatalf("MatchIndex = %d, want %d", m.Search.MatchIndex, wantIndex)
	}
	if m.ScrollOffset != wantScroll {
		t.Fatalf("ScrollOffset = %d, want %d", m.ScrollOffset, wantScroll)
	}
	if prompt := renderSearchPrompt(m, len(m.Search.Matches)); !strings.Contains(prompt, wantPrompt) {
		t.Fatalf("search prompt should show %q, got %q", wantPrompt, prompt)
	}
}

func TestBuildVisibleRuneMapPlainLine(t *testing.T) {
	text, indices := buildVisibleRuneMap("hello world")
	if text != "hello world" {
		t.Errorf("plain text = %q, want %q", text, "hello world")
	}
	if len(indices) != 11 {
		t.Errorf("got %d indices, want 11", len(indices))
	}
	for i, idx := range indices {
		if idx != i {
			t.Errorf("index[%d] = %d, want %d", i, idx, i)
		}
	}
}

func TestBuildVisibleRuneMapWithANSI(t *testing.T) {
	text, indices := buildVisibleRuneMap("\x1b[34mhello\x1b[0m world")
	if text != "hello world" {
		t.Errorf("plain text = %q, want %q", text, "hello world")
	}
	if len(indices) != 11 {
		t.Errorf("got %d indices, want 11", len(indices))
	}
}

func TestFindSearchMatches(t *testing.T) {
	bodyLines := []string{
		"hello world",
		"Hello there",
		"no match here",
		"world hello",
	}

	matches := findSearchMatches("hello", bodyLines)
	if len(matches) != 3 {
		t.Errorf("got %d matches, want 3", len(matches))
	}

	if matches[0].Line != 0 || matches[0].ColStart != 0 || matches[0].ColEnd != 5 {
		t.Errorf("match[0]: line=%d, col=%d-%d; want line=0, col=0-5", matches[0].Line, matches[0].ColStart, matches[0].ColEnd)
	}
	if matches[1].Line != 1 || matches[1].ColStart != 0 || matches[1].ColEnd != 5 {
		t.Errorf("match[1]: line=%d, col=%d-%d; want line=1, col=0-5", matches[1].Line, matches[1].ColStart, matches[1].ColEnd)
	}
	if matches[2].Line != 3 || matches[2].ColStart != 6 || matches[2].ColEnd != 11 {
		t.Errorf("match[2]: line=%d, col=%d-%d; want line=3, col=6-11", matches[2].Line, matches[2].ColStart, matches[2].ColEnd)
	}
}

func TestFindSearchMatchesWithANSI(t *testing.T) {
	bodyLines := []string{
		"\x1b[34mhello\x1b[0m world",
		"prefix \x1b[1mhello\x1b[0m suffix",
	}

	matches := findSearchMatches("hello", bodyLines)
	if len(matches) != 2 {
		t.Fatalf("got %d matches, want 2", len(matches))
	}

	if matches[0].Line != 0 || matches[0].ColStart < 0 || matches[0].ColEnd < 0 {
		t.Errorf("match[0] on ANSI line has invalid bounds: col=%d-%d", matches[0].ColStart, matches[0].ColEnd)
	}
	if matches[1].Line != 1 || matches[1].ColStart < 0 || matches[1].ColEnd < 0 {
		t.Errorf("match[1] on ANSI line has invalid bounds: col=%d-%d", matches[1].ColStart, matches[1].ColEnd)
	}
}

func TestFindSearchMatchesEmptyQuery(t *testing.T) {
	matches := findSearchMatches("", []string{"hello", "world"})
	if matches != nil {
		t.Errorf("expected nil matches for empty query, got %d", len(matches))
	}
}

func TestFindSearchMatchesNoMatches(t *testing.T) {
	matches := findSearchMatches("xyz", []string{"hello", "world"})
	if len(matches) != 0 {
		t.Errorf("expected 0 matches, got %d", len(matches))
	}
}

func TestHighlightLineWithMatches(t *testing.T) {
	line := "hello world hello"
	matches := []SearchMatch{
		{0, 0, 5},
		{0, 12, 17},
	}

	result := highlightLineWithMatches(line, matches)
	if len(result) < len(line) {
		t.Errorf("highlighted line should be at least as long as original: got len %d, want >= %d", len(result), len(line))
	}
	stylePrefix, _ := searchMatchStyleCodes()
	if stylePrefix == "" || !strings.Contains(result, stylePrefix) {
		t.Errorf("highlighted line should include search match styling, got %q", result)
	}
	// The plain text content should still be findable
	stripped := xansi.Strip(result)
	if stripped != line {
		t.Errorf("highlighted line content should match original: got %q, want %q", stripped, line)
	}
}

func TestHighlightLineWithActiveMatch(t *testing.T) {
	line := "hello world hello"
	matches := []SearchMatch{
		{0, 0, 5},
		{0, 12, 17},
	}
	active := matches[1]

	result := highlightLineWithActiveMatch(line, matches, &active)
	matchPrefix, _ := searchMatchStyleCodes()
	activePrefix, _ := activeSearchMatchStyleCodes()
	if !strings.Contains(result, matchPrefix) {
		t.Fatalf("highlighted line should include regular match styling, got %q", result)
	}
	if !strings.Contains(result, activePrefix) {
		t.Fatalf("highlighted line should include active match styling, got %q", result)
	}
	if stripped := xansi.Strip(result); stripped != line {
		t.Fatalf("highlighted line content should match original: got %q, want %q", stripped, line)
	}
}

func TestHighlightLineWithMatchesOverlapping(t *testing.T) {
	line := "aaaaa"
	matches := []SearchMatch{
		{0, 0, 3},
		{0, 1, 4},
		{0, 2, 5},
	}

	result := highlightLineWithMatches(line, matches)
	if len(result) < len(line) {
		t.Errorf("highlighted line should be at least as long as original: got len %d, want >= %d", len(result), len(line))
	}
	stripped := xansi.Strip(result)
	if stripped != line {
		t.Errorf("highlighted line content should match original: got %q, want %q", stripped, line)
	}
}

func TestHighlightLineNoMatches(t *testing.T) {
	line := "hello world"
	result := highlightLineWithMatches(line, nil)
	if result != line {
		t.Errorf("expected unmodified line, got %q", result)
	}
	result = highlightLineWithMatches(line, []SearchMatch{})
	if result != line {
		t.Errorf("expected unmodified line, got %q", result)
	}
}

func TestRenderSearchPromptShowsMatchCount(t *testing.T) {
	p := NewCommandPalette()
	p.CursorVisible = true
	m := NewModel(nil, "/tmp/root")
	m.Palette = &p
	m.Width = 80
	m.Search.Open = true
	m.Search.Query = "test"

	result := renderSearchPrompt(m, 3)
	if !strings.Contains(result, "3 matches") {
		t.Errorf("expected '3 matches' in prompt, got %q", result)
	}
}

func TestRenderSearchPromptSingleMatch(t *testing.T) {
	p := NewCommandPalette()
	p.CursorVisible = true
	m := NewModel(nil, "/tmp/root")
	m.Palette = &p
	m.Width = 80
	m.Search.Open = true
	m.Search.Query = "test"

	result := renderSearchPrompt(m, 1)
	if !strings.Contains(result, "1 match") {
		t.Errorf("expected '1 match' in prompt, got %q", result)
	}
	if strings.Contains(result, "1 matches") {
		t.Errorf("expected singular 'match' not 'matches', got %q", result)
	}
}

func stringsHasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}

// ── Todo List section rendering ──────────────────────────────────────────────

func TestTransformTodoListSectionInjectsCheckboxMarkers(t *testing.T) {
	input := "## Todo List\n\n**Completed**\n\n- [x] removed stale entries\n- [x] wired up CLI\n\n**Open**\n\n- [ ] follow up from handoff\n- [ ] add drag support\n"
	result := transformTodoListSection(input)

	if !strings.Contains(result, "**Completed**") {
		t.Errorf("expected Completed subheading preserved, got:\n%s", result)
	}
	if !strings.Contains(result, "**Open**") {
		t.Errorf("expected Open subheading preserved, got:\n%s", result)
	}
	if !strings.Contains(result, "DONE_CHKremoved stale entries") {
		t.Errorf("expected completed item with DONE_CHK marker, got:\n%s", result)
	}
	if !strings.Contains(result, "OPEN_CHKfollow up from handoff") {
		t.Errorf("expected open item with OPEN_CHK marker, got:\n%s", result)
	}
	if strings.Contains(result, "- DONE_CHK") || strings.Contains(result, "- OPEN_CHK") {
		t.Errorf("preview transform should not emit markdown bullets for todo items, got:\n%s", result)
	}
}

func TestTransformTodoListSectionSeparatesMultilineItems(t *testing.T) {
	input := "## Todo List\n\n**Completed**\n\n- [x] fix bug where todo list does not transfer over to new session when\n  you close one\n- [x] add entry composer type input to todo for\n  multi-line todo items/messages\n\n**Open**\n\n- [ ] verify preview rendering\n  after multiline wrapping\n- [ ] confirm saved markdown stays semantic\n"
	result := transformTodoListSection(input)

	if !strings.Contains(result, "  you close one\n\nDONE_CHKadd entry composer") {
		t.Fatalf("expected completed multiline item to be separated from the next item, got:\n%s", result)
	}
	if !strings.Contains(result, "  after multiline wrapping\n\nOPEN_CHKconfirm saved markdown") {
		t.Fatalf("expected open multiline item to be separated from the next item, got:\n%s", result)
	}
	if strings.Contains(result, "you close one\nDONE_CHK") || strings.Contains(result, "after multiline wrapping\nOPEN_CHK") {
		t.Fatalf("multiline todo continuation should not run into the next marker, got:\n%s", result)
	}
}

func TestTransformTodoListSectionSkipsNonTodoSections(t *testing.T) {
	input := "## Summary\n\nProgress: some note.\n\n## Todo List\n\n**Open**\n\n- [ ] my todo\n\n## Changes\n\nNo code changes.\n"
	result := transformTodoListSection(input)

	if !strings.Contains(result, "## Summary") {
		t.Errorf("expected Summary section preserved, got:\n%s", result)
	}
	if !strings.Contains(result, "## Changes") {
		t.Errorf("expected Changes section preserved, got:\n%s", result)
	}
	if !strings.Contains(result, "OPEN_CHK") {
		t.Errorf("expected OPEN_CHK marker in Todo List section, got:\n%s", result)
	}
}

func TestStyleTodoListRenderedStylesHeadingAndCheckboxes(t *testing.T) {
	input := "▌ Todo List\n\nCompleted\n\n☑ removed stale entries\n\nOpen\n\n☐ follow up from handoff\n\n▌ Changes\n"
	result := styleTodoListRendered(input)

	if !strings.Contains(result, TodoListHeadingStyle.Render("▌ Todo List")) {
		t.Errorf("expected styled Todo List heading, got:\n%s", result)
	}
	if !strings.Contains(result, ChangesHeadingStyle.Render("▌ Changes")) {
		t.Errorf("expected styled Changes heading, got:\n%s", result)
	}
	if !strings.Contains(result, "  "+TodoListSubheadingStyle.Render("Completed")) {
		t.Errorf("expected indented styled Completed subheading, got:\n%s", result)
	}
	if !strings.Contains(result, "  "+TodoListSubheadingStyle.Render("Open")) {
		t.Errorf("expected indented styled Open subheading, got:\n%s", result)
	}
	if !strings.Contains(result, TodoDoneCheckboxStyle.Render("☑")) {
		t.Errorf("expected styled completed checkbox, got:\n%s", result)
	}
	if !strings.Contains(result, TodoOpenCheckboxStyle.Render("☐")) {
		t.Errorf("expected styled open checkbox, got:\n%s", result)
	}
	stripped := xansi.Strip(result)
	for _, want := range []string{"▌ Todo List", "  Completed", "    ☑ removed stale entries", "  Open", "    ☐ follow up from handoff", "▌ Changes"} {
		if !strings.Contains(stripped, want) {
			t.Errorf("expected stripped output to contain %q, got:\n%s", want, stripped)
		}
	}
}

func TestStyleTodoListRenderedDoesNotLeakToNonHeadingText(t *testing.T) {
	input := "▌ Summary\n\nThis Todo List phrase is body text.\n\n▌ Todo List\n\nOpen\n\n☐ real todo\n"
	result := styleTodoListRendered(input)
	for _, line := range strings.Split(result, "\n") {
		if strings.Contains(xansi.Strip(line), "This Todo List phrase") && strings.Contains(line, "\x1b[") {
			t.Fatalf("body text containing Todo List should not be recolored, got line %q in:\n%s", line, result)
		}
	}
}

func TestRenderHandoffBodyTodoListUsesCheckboxRowsWithoutBullets(t *testing.T) {
	m := testModel()
	m.Width = 80
	m.HandoffContent = "# Handoff: feat/test -- session (Alice) [active]\n\n## Summary\nProgress: done.\n\n## Todo List\n\n**Completed**\n\n- [x] removed stale entries\n- [x] wired up CLI\n\n**Open**\n\n- [ ] follow up from handoff\n- [ ] add drag support\n\n## Changes\nNo code changes.\n"

	rendered := renderHandoffBody(m)
	stripped := xansi.Strip(rendered)
	if strings.Contains(stripped, "• ☑") || strings.Contains(stripped, "• ☐") {
		t.Fatalf("todo preview should not include list bullets before checkboxes, got:\n%s", stripped)
	}
	lines := strings.Split(stripped, "\n")
	checkboxLines := 0
	for _, line := range lines {
		if strings.Contains(line, "☑") || strings.Contains(line, "☐") {
			checkboxLines++
			if strings.Count(line, "☑")+strings.Count(line, "☐") != 1 {
				t.Fatalf("each todo row should contain one checkbox, got line %q in:\n%s", line, stripped)
			}
		}
	}
	if checkboxLines != 4 {
		t.Fatalf("expected 4 vertical checkbox rows, got %d in:\n%s", checkboxLines, stripped)
	}
	if !strings.Contains(stripped, "  Completed") || !strings.Contains(stripped, "  Open") {
		t.Fatalf("todo subheadings should be indented, got:\n%s", stripped)
	}
	if !strings.Contains(stripped, "    ☑ removed stale entries") || !strings.Contains(stripped, "    ☐ follow up from handoff") {
		t.Fatalf("todo rows should be indented, got:\n%s", stripped)
	}
}

func TestRenderHandoffBodyTodoListKeepsMultilineItemsSeparate(t *testing.T) {
	m := testModel()
	m.Width = 56
	m.HandoffContent = "# Handoff: feat/test -- session (Alice) [active]\n\n## Summary\nProgress: done.\n\n## Todo List\n\n**Completed**\n\n- [x] fix bug where todo list does not transfer over to new session when\n  you close one\n- [x] add entry composer type input to todo for\n  multi-line todo items/messages\n\n**Open**\n\n- [ ] verify preview rendering\n  after multiline wrapping\n- [ ] confirm saved markdown stays semantic\n\n## Changes\nNo code changes.\n"

	rendered := renderHandoffBody(m)
	stripped := xansi.Strip(rendered)
	if strings.Contains(stripped, "DONE_CHK") || strings.Contains(stripped, "OPEN_CHK") {
		t.Fatalf("todo preview should not leak internal checkbox markers, got:\n%s", stripped)
	}
	if strings.Contains(stripped, "you close one add entry") {
		t.Fatalf("multiline completed todo should not merge with next item, got:\n%s", stripped)
	}
	if strings.Contains(stripped, "after multiline wrapping confirm saved") {
		t.Fatalf("multiline open todo should not merge with next item, got:\n%s", stripped)
	}

	checkboxLines := 0
	for _, line := range strings.Split(stripped, "\n") {
		if strings.Contains(line, "☑") || strings.Contains(line, "☐") {
			checkboxLines++
			if strings.Count(line, "☑")+strings.Count(line, "☐") != 1 {
				t.Fatalf("each todo row should contain one checkbox, got line %q in:\n%s", line, stripped)
			}
		}
	}
	if checkboxLines != 4 {
		t.Fatalf("expected 4 checkbox rows, got %d in:\n%s", checkboxLines, stripped)
	}
}

func TestRenderHandoffBodyTodoListDoesNotSplitWordsFromSourceWrapping(t *testing.T) {
	m := testModel()
	m.Width = 56
	m.HandoffContent = "# Handoff: feat/test -- session (Alice) [active]\n\n## Summary\nProgress: done.\n\n## Todo List\n\n**Completed**\n\n- [x] fix bug where todo list does not transfer over to new session when you close one\n\n## Changes\nNo code changes.\n"

	rendered := renderHandoffBody(m)
	stripped := xansi.Strip(rendered)
	if strings.Contains(stripped, "DONE_CHK") || strings.Contains(stripped, "OPEN_CHK") {
		t.Fatalf("todo preview should not leak internal checkbox markers, got:\n%s", stripped)
	}
	if strings.Contains(stripped, "c lose") {
		t.Fatalf("todo preview should not split the word close, got:\n%s", stripped)
	}
	if !strings.Contains(stripped, "close one") {
		t.Fatalf("todo preview should preserve the word close in wrapped output, got:\n%s", stripped)
	}
}

func TestTransformTodoListSectionPassesThroughWhenNoTodoSection(t *testing.T) {
	input := "## Summary\n\nProgress: some note.\n\n## Changes\n\nNo code changes.\n"
	result := transformTodoListSection(input)

	if result != input {
		t.Errorf("expected pass-through when no Todo List section, got:\n%s", result)
	}
}

// ── Diff cursor ──────────────────────────────────────────────────────────────

func diffCursorTestContent() string {
	return "# Handoff: feat/test -- session (Test) [active]\n\n" +
		"## Summary\n\nWorked on the diff cursor feature. Encountered several edge\n" +
		"cases around scrolling and cursor activation. Need to verify the\n" +
		"tint is visible and not too intense.\n\n" +
		"## Changes\n\n" +
		"#### src/a.go\n\n```diff\n+a1\n+a2\n+a3\n+a4\n```\n\n" +
		"#### src/b.go\n\n```diff\n+b1\n+b2\n+b3\n+b4\n```\n\n" +
		"#### src/c.go\n\n```diff\n+c1\n+c2\n+c3\n+c4\n```"
}

func newDiffCursorModel(t *testing.T) Model {
	t.Helper()
	m := NewModel(nil, "/tmp/root")
	m.CurrentView = HandoffPreview
	m.HandoffContent = diffCursorTestContent()
	m.Width = 80
	m.Height = 6
	m.refreshHandoffBodyCache()
	return m
}

func TestDiffCursorInitialSelectionIsEmpty(t *testing.T) {
	m := newDiffCursorModel(t)
	if m.HandoffSelectedDiff != "" {
		t.Fatalf("HandoffSelectedDiff = %q, want empty at preview start", m.HandoffSelectedDiff)
	}
}

func TestDiffCursorSyncFollowsScrolling(t *testing.T) {
	m := newDiffCursorModel(t)
	m.Width = 80
	m.Height = 8
	m.refreshHandoffBodyCache()
	bodyLines := handoffBodyLines(m)
	contentLines := handoffPreviewContentLines(m)
	headingIdx := func(path string) int {
		return handoffDiffHeadingLineIndex(bodyLines, path)
	}

	// At scroll=0 the cursor should be empty — no diff heading is visible.
	m.ScrollOffset = 0
	m.syncHandoffDiffCursor()
	if m.HandoffSelectedDiff != "" {
		t.Fatalf("cursor at scroll=0 = %q, want empty", m.HandoffSelectedDiff)
	}

	// Scrolling to make the first diff heading visible should activate.
	m.ScrollOffset = headingIdx("src/a.go")
	m.syncHandoffDiffCursor()
	if m.HandoffSelectedDiff != "src/a.go" {
		t.Fatalf("cursor at a heading = %q, want src/a.go", m.HandoffSelectedDiff)
	}

	m.ScrollOffset = headingIdx("src/b.go")
	m.syncHandoffDiffCursor()
	if m.HandoffSelectedDiff != "src/b.go" {
		t.Fatalf("cursor at b heading = %q, want src/b.go", m.HandoffSelectedDiff)
	}

	// Scrolling past the b heading into its body (but before c) should keep b.
	m.ScrollOffset = headingIdx("src/b.go") + 3
	m.syncHandoffDiffCursor()
	if m.HandoffSelectedDiff != "src/b.go" {
		t.Fatalf("cursor in b body = %q, want src/b.go", m.HandoffSelectedDiff)
	}

	// End key: scrolled to the bottom, last diff heading is above viewport.
	m.ScrollOffset = len(bodyLines) - contentLines
	if m.ScrollOffset < 0 {
		m.ScrollOffset = 0
	}
	m.syncHandoffDiffCursor()
	if m.HandoffSelectedDiff != "src/c.go" {
		t.Fatalf("cursor at end = %q, want src/c.go", m.HandoffSelectedDiff)
	}
}

func TestDiffCursorJumpNextAndPrevious(t *testing.T) {
	m := newDiffCursorModel(t)
	m.CurrentView = HandoffPreview
	m.HandoffSelectedDiff = "src/a.go"

	result, _ := handleHandoffKey(&m, "]")
	updated := result.(Model)
	if updated.HandoffSelectedDiff != "src/b.go" {
		t.Fatalf("after ']' selected = %q, want src/b.go", updated.HandoffSelectedDiff)
	}

	result, _ = handleHandoffKey(&updated, "]")
	updated = result.(Model)
	if updated.HandoffSelectedDiff != "src/c.go" {
		t.Fatalf("after second ']' selected = %q, want src/c.go", updated.HandoffSelectedDiff)
	}

	result, _ = handleHandoffKey(&updated, "]")
	updated = result.(Model)
	if updated.HandoffSelectedDiff != "src/a.go" {
		t.Fatalf("']' at end should wrap to first, got %q", updated.HandoffSelectedDiff)
	}

	result, _ = handleHandoffKey(&updated, "[")
	updated = result.(Model)
	if updated.HandoffSelectedDiff != "src/c.go" {
		t.Fatalf("'[' at start should wrap to last, got %q", updated.HandoffSelectedDiff)
	}

	result, _ = handleHandoffKey(&updated, "[")
	updated = result.(Model)
	if updated.HandoffSelectedDiff != "src/b.go" {
		t.Fatalf("after '[' selected = %q, want src/b.go", updated.HandoffSelectedDiff)
	}

	result, _ = handleHandoffKey(&updated, "[")
	updated = result.(Model)
	if updated.HandoffSelectedDiff != "src/a.go" {
		t.Fatalf("after second '[' selected = %q, want src/a.go", updated.HandoffSelectedDiff)
	}
}

func TestDiffCursorEnterTogglesSelectedDiff(t *testing.T) {
	m := newDiffCursorModel(t)
	m.CurrentView = HandoffPreview
	m.HandoffSelectedDiff = "src/b.go"

	result, _ := handleHandoffKey(&m, "enter")
	updated := result.(Model)
	if !updated.HandoffCollapsedDiffs["src/b.go"] {
		t.Fatalf("Enter should collapse src/b.go, got %#v", updated.HandoffCollapsedDiffs)
	}

	result, _ = handleHandoffKey(&updated, "enter")
	updated = result.(Model)
	if updated.HandoffCollapsedDiffs["src/b.go"] {
		t.Fatalf("Enter should expand src/b.go back, got %#v", updated.HandoffCollapsedDiffs)
	}
}

func TestDiffCursorEnterRespectsActivePrompts(t *testing.T) {
	m := newDiffCursorModel(t)
	m.CurrentView = HandoffPreview
	m.HandoffSelectedDiff = "src/b.go"
	m.Search.Open = true

	result, _ := handleHandoffKey(&m, "enter")
	updated := result.(Model)
	if _, ok := updated.HandoffCollapsedDiffs["src/b.go"]; ok {
		t.Fatalf("Enter during search should not toggle selected diff, got %#v", updated.HandoffCollapsedDiffs)
	}

	m.Search.Open = false
	m.SavePromptOpen = true
	result, _ = handleHandoffKey(&m, "enter")
	updated = result.(Model)
	if _, ok := updated.HandoffCollapsedDiffs["src/b.go"]; ok {
		t.Fatalf("Enter during save prompt should not toggle selected diff, got %#v", updated.HandoffCollapsedDiffs)
	}

	m.SavePromptOpen = false
	m.CollapsedDiffConfirmOpen = true
	result, _ = handleHandoffKey(&m, "enter")
	updated = result.(Model)
	if _, ok := updated.HandoffCollapsedDiffs["src/b.go"]; ok {
		t.Fatalf("Enter during collapsed-diff confirm should not toggle selected diff, got %#v", updated.HandoffCollapsedDiffs)
	}
}

func TestDiffCursorJumpScrollsIntoView(t *testing.T) {
	m := newDiffCursorModel(t)
	m.CurrentView = HandoffPreview
	m.ScrollOffset = 0
	m.HandoffSelectedDiff = "src/a.go"

	result, _ := handleHandoffKey(&m, "]")
	updated := result.(Model)
	if updated.HandoffSelectedDiff != "src/b.go" {
		t.Fatalf("after ']' selected = %q, want src/b.go", updated.HandoffSelectedDiff)
	}
	if updated.ScrollOffset == 0 {
		t.Fatalf("'['/'[' should scroll to keep selected heading visible, ScrollOffset still 0")
	}
}

func TestDiffCursorScrollKeysInvokeSync(t *testing.T) {
	m := newDiffCursorModel(t)
	m.CurrentView = HandoffPreview
	m.Width = 80
	m.Height = 24
	m.refreshHandoffBodyCache()
	m.HandoffSelectedDiff = "src/a.go"

	for i := 0; i < 30; i++ {
		result, _ := handleHandoffKey(&m, "pgdown")
		m = result.(Model)
	}

	if m.HandoffSelectedDiff == "src/a.go" {
		t.Fatalf("paging down should advance cursor past src/a.go, got %q", m.HandoffSelectedDiff)
	}
}

func TestDiffCursorHomeClearsSelectionAtTop(t *testing.T) {
	m := newDiffCursorModel(t)
	m.CurrentView = HandoffPreview
	m.Width = 80
	m.Height = 8
	m.refreshHandoffBodyCache()
	m.ScrollOffset = handoffMaxScrollOffset(m)
	m.syncHandoffDiffCursor()
	if m.HandoffSelectedDiff == "" {
		t.Fatal("expected cursor to select a diff before returning home")
	}

	result, _ := handleHandoffKey(&m, "home")
	updated := result.(Model)
	if updated.ScrollOffset != 0 {
		t.Fatalf("home should return to top, got ScrollOffset=%d", updated.ScrollOffset)
	}
	if updated.HandoffSelectedDiff != "" {
		t.Fatalf("home should clear selected diff at top, got %q", updated.HandoffSelectedDiff)
	}
}

func TestDiffCursorResetsWhenHandoffContentChanges(t *testing.T) {
	m := newDiffCursorModel(t)
	m.HandoffSelectedDiff = "src/b.go"
	m.HandoffContent = "## Changes\n\nNo code changes."

	m.refreshHandoffBodyCache()
	m.syncHandoffDiffCursor()

	if m.HandoffSelectedDiff != "" {
		t.Fatalf("cursor should reset when no diffs remain, got %q", m.HandoffSelectedDiff)
	}
}

func TestDiffCursorStyleTintsSelectedDiffBody(t *testing.T) {
	m := newDiffCursorModel(t)
	m.Width = 120
	m.Height = 40
	m.refreshHandoffBodyCache()
	m.HandoffSelectedDiff = "src/b.go"
	m.refreshHandoffBodyCache()

	bodyLines := handoffBodyLines(m)
	start, end := selectedDiffLineRange(bodyLines, m.HandoffSelectedDiff)
	if start < 0 {
		t.Fatalf("expected to find a diff range for src/b.go, got none")
	}
	if end <= start {
		t.Fatalf("expected end > start, got start=%d end=%d", start, end)
	}

	styled := applyHandoffDiffCursorStyle(bodyLines, m.HandoffSelectedDiff)

	// Lines inside the selected range that have Glamour's code background
	// should be tinted; heading/border/blank lines should not.
	for i := start; i < end; i++ {
		hasGlamourBg := strings.Contains(bodyLines[i], "\x1b[48;5;235m")
		hasTint := hasBackgroundTint(styled[i])
		if hasGlamourBg && !hasTint {
			t.Fatalf("expected tint on diff body line %d (has Glamour bg), got %q", i, styled[i])
		}
		if !hasGlamourBg && hasTint {
			stripped := strings.TrimSpace(xansi.Strip(bodyLines[i]))
			t.Fatalf("expected no tint on non-body line %d (%q), got %q", i, stripped, styled[i])
		}
	}

	// Lines outside the selected range should never be tinted.
	for i := 0; i < len(styled); i++ {
		if i >= start && i < end {
			continue
		}
		if hasBackgroundTint(styled[i]) {
			t.Fatalf("expected no background tint on line %d outside selected diff, got %q", i, styled[i])
		}
	}

	// Glamour's original background should be fully replaced on tinted lines.
	for i := start; i < end; i++ {
		if hasBackgroundTint(styled[i]) && strings.Contains(styled[i], "\x1b[48;5;235m") {
			t.Fatalf("expected Glamour background (48;5;235) to be replaced on line %d, got %q", i, styled[i])
		}
	}
}

func hasBackgroundTint(line string) bool {
	return strings.Contains(line, "\x1b[48;5;236m")
}

func TestDiffCursorNoDiffsKeysAreNoop(t *testing.T) {
	m := NewModel(nil, "/tmp/root")
	m.CurrentView = HandoffPreview
	m.HandoffContent = "## Changes\n\nNo code changes."
	m.Width = 80
	m.Height = 24
	m.refreshHandoffBodyCache()
	m.syncHandoffDiffCursor()

	if m.HandoffSelectedDiff != "" {
		t.Fatalf("expected empty cursor when no diffs, got %q", m.HandoffSelectedDiff)
	}

	result, _ := handleHandoffKey(&m, "]")
	updated := result.(Model)
	if updated.HandoffSelectedDiff != "" {
		t.Fatalf("']' on no-diff preview should not set cursor, got %q", updated.HandoffSelectedDiff)
	}

	result, _ = handleHandoffKey(&updated, "enter")
	updated = result.(Model)
	if updated.HandoffCollapsedDiffs != nil {
		t.Fatalf("Enter on no-diff preview should not collapse anything, got %#v", updated.HandoffCollapsedDiffs)
	}
}
