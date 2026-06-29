package tui

import (
	"strings"
	"testing"
	"time"

	internalconfig "github.com/amo/devlog/internal/config"
	"github.com/amo/devlog/internal/store"
	tea "github.com/charmbracelet/bubbletea"
	xansi "github.com/charmbracelet/x/ansi"
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

func testEventTime(hour, minute int) time.Time {
	return time.Date(2026, 1, 15, hour, minute, 0, 0, time.UTC)
}

func testModel() Model {
	p := NewCommandPalette()
	return Model{
		Palette: &p,
	}
}

func testScrollableActiveModel() Model {
	m := testModel()
	m.CurrentView = ActiveSession
	m.ActiveSession = testActiveSession()
	m.Width = 80
	m.Height = 10
	m.Events = []store.SessionEvent{{Type: "Start", Body: "Test title"}}
	for i := 0; i < 12; i++ {
		m.Events = append(m.Events, store.SessionEvent{Type: "Note", Time: testEventTime(14, 30), Body: "scrollable note body with enough text to create a multi-line event"})
	}
	return m
}

func TestLineScrollKeyCoalescesHeldRepeats(t *testing.T) {
	m := testScrollableActiveModel()
	now := time.Date(2026, 6, 13, 12, 0, 0, 0, time.UTC)

	updatedModel, cmd := m.handleLineScrollKey(scrollDirectionDown, now)
	updated, ok := updatedModel.(Model)
	if !ok {
		t.Fatalf("expected Model from handleLineScrollKey, got %T", updatedModel)
	}
	if cmd != nil {
		t.Fatal("direct line scroll should not start a command")
	}
	if updated.ScrollOffset != 1 {
		t.Fatalf("first scroll key ScrollOffset = %d, want 1", updated.ScrollOffset)
	}

	repeatedModel, cmd := updated.handleLineScrollKey(scrollDirectionDown, now.Add(10*time.Millisecond))
	repeated, ok := repeatedModel.(Model)
	if !ok {
		t.Fatalf("expected Model from repeated scroll key, got %T", repeatedModel)
	}
	if cmd != nil {
		t.Fatal("same-direction repeat should not start a command")
	}
	if repeated.ScrollOffset != updated.ScrollOffset {
		t.Fatalf("same-direction repeat ScrollOffset = %d, want %d", repeated.ScrollOffset, updated.ScrollOffset)
	}
	if repeated.scrollDirection != scrollDirectionDown || !repeated.lastLineScroll.Equal(now) {
		t.Fatalf("same-direction repeat state direction=%d last=%s, want direction down and unchanged timestamp", repeated.scrollDirection, repeated.lastLineScroll)
	}
}

func TestLineScrollKeyAcceptsRepeatAfterMinInterval(t *testing.T) {
	m := testScrollableActiveModel()
	now := time.Date(2026, 6, 13, 12, 0, 0, 0, time.UTC)

	updatedModel, _ := m.handleLineScrollKey(scrollDirectionDown, now)
	updated := updatedModel.(Model)

	repeatedModel, cmd := updated.handleLineScrollKey(scrollDirectionDown, now.Add(lineScrollMinInterval))
	repeated, ok := repeatedModel.(Model)
	if !ok {
		t.Fatalf("expected Model from repeated scroll key, got %T", repeatedModel)
	}
	if cmd != nil {
		t.Fatal("direct line scroll should not start a command")
	}
	if repeated.ScrollOffset != 2 {
		t.Fatalf("accepted repeat ScrollOffset = %d, want 2", repeated.ScrollOffset)
	}
	if !repeated.lastLineScroll.Equal(now.Add(lineScrollMinInterval)) {
		t.Fatalf("lastLineScroll = %s, want accepted repeat timestamp", repeated.lastLineScroll)
	}
}

func TestLineScrollKeyOppositeDirectionIsImmediate(t *testing.T) {
	m := testScrollableActiveModel()
	now := time.Date(2026, 6, 13, 12, 0, 0, 0, time.UTC)
	m.ScrollOffset = 3
	m.scrollDirection = scrollDirectionDown
	m.lastLineScroll = now

	updatedModel, cmd := m.handleLineScrollKey(scrollDirectionUp, now.Add(time.Millisecond))
	updated, ok := updatedModel.(Model)
	if !ok {
		t.Fatalf("expected Model from opposite-direction scroll key, got %T", updatedModel)
	}
	if cmd != nil {
		t.Fatal("direct line scroll should not start a command")
	}
	if updated.ScrollOffset != 2 {
		t.Fatalf("opposite-direction ScrollOffset = %d, want 2", updated.ScrollOffset)
	}
	if updated.scrollDirection != scrollDirectionUp {
		t.Fatalf("scrollDirection = %d, want up", updated.scrollDirection)
	}
}

func TestHandoffLineScrollKeyCoalescesHeldRepeats(t *testing.T) {
	m := testModel()
	m.CurrentView = HandoffPreview
	m.HandoffContent = strings.Repeat("## Section\n\nLong paragraph body for scrolling.\n\n", 30)
	m.Width = 80
	m.Height = 8
	now := time.Date(2026, 6, 13, 12, 0, 0, 0, time.UTC)

	updatedModel, cmd := m.handleLineScrollKey(scrollDirectionDown, now)
	updated, ok := updatedModel.(Model)
	if !ok {
		t.Fatalf("expected Model from handoff scroll key, got %T", updatedModel)
	}
	if cmd != nil {
		t.Fatal("handoff direct line scroll should not start a command")
	}
	if updated.ScrollOffset != 1 {
		t.Fatalf("handoff first scroll ScrollOffset = %d, want 1", updated.ScrollOffset)
	}

	repeatedModel, cmd := updated.handleLineScrollKey(scrollDirectionDown, now.Add(10*time.Millisecond))
	repeated, ok := repeatedModel.(Model)
	if !ok {
		t.Fatalf("expected Model from handoff repeated scroll key, got %T", repeatedModel)
	}
	if cmd != nil {
		t.Fatal("handoff same-direction repeat should not start a command")
	}
	if repeated.ScrollOffset != updated.ScrollOffset {
		t.Fatalf("handoff same-direction repeat ScrollOffset = %d, want %d", repeated.ScrollOffset, updated.ScrollOffset)
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

func TestRenderActiveSessionUsesConfiguredDisplayTime(t *testing.T) {
	m := testModel()
	m.CurrentView = ActiveSession
	m.ActiveSession = testActiveSession()
	m.Events = []store.SessionEvent{
		{Type: "Start", Body: "Test title"},
		{Type: "Note", Time: time.Date(2026, 1, 15, 15, 30, 0, 0, time.UTC), Body: "finished parser"},
	}
	m.Config = internalconfig.Default()
	m.Config.Display.Timezone = "America/New_York"
	m.Config.Display.ClockFormat = internalconfig.ClockFormat12h
	formatter, err := internalconfig.NewDisplayTimeFormatter(m.Config.Display)
	if err != nil {
		t.Fatalf("NewDisplayTimeFormatter failed: %v", err)
	}
	m.displayTime = formatter
	m.Title = "Test title"
	m.Width = 80
	m.Height = 24

	v := renderActiveSession(m)
	if !strings.Contains(v, "Started: 2026-01-15 9:30:22 AM EST") {
		t.Fatalf("active session header should use configured display time, got:\n%s", v)
	}
	if !strings.Contains(v, "[1] Note · 2026-01-15 10:30 AM EST") {
		t.Fatalf("active session timeline should use configured display time, got:\n%s", v)
	}
}

func TestRenderEventLinesShowsRecencyIndexes(t *testing.T) {
	events := []store.SessionEvent{
		{Type: "Note", Time: testEventTime(14, 30), Body: "old note"},
		{Type: "Blocker", Time: testEventTime(14, 35), Body: "middle blocker"},
		{Type: "Note", Time: testEventTime(14, 40), Body: "new note"},
	}

	lines, _ := renderEventLines(events, 80, internalconfig.DefaultDisplayTimeFormatter(), -1)
	plain := xansi.Strip(strings.Join(lines, "\n"))

	if !strings.Contains(plain, "[3] Note") {
		t.Fatalf("oldest visible event should render as [3], got:\n%s", plain)
	}
	if !strings.Contains(plain, "[2] Blocker") {
		t.Fatalf("middle visible event should render as [2], got:\n%s", plain)
	}
	if !strings.Contains(plain, "[1] Note") {
		t.Fatalf("newest visible event should render as [1], got:\n%s", plain)
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

func TestRenderActiveSessionDoesNotOverflowHeight(t *testing.T) {
	m := testModel()
	m.CurrentView = ActiveSession
	m.ActiveSession = testActiveSession()
	m.Title = "Test title"
	m.Width = 80
	m.Height = 12
	m.Events = []store.SessionEvent{{Type: "Start", Body: "Test title"}}
	for i := 0; i < 20; i++ {
		m.Events = append(m.Events, store.SessionEvent{Type: "Note", Time: testEventTime(14, 30), Body: "long note body that should be clipped to the available viewport height"})
	}

	v := renderActiveSession(m)
	if got := countLines(v); got > m.Height {
		t.Fatalf("renderActiveSession returned %d lines, want at most %d", got, m.Height)
	}
}

func TestBottomSectionHeightIncludesTransientMessages(t *testing.T) {
	m := testModel()
	m.CurrentView = ActiveSession
	m.Width = 80
	m.Height = 24

	withoutMessage := bottomSectionHeight(m)
	m.ErrorMessage = "Something went wrong"
	withMessage := bottomSectionHeight(m)

	if withMessage <= withoutMessage {
		t.Fatalf("bottomSectionHeight with message = %d, want greater than %d", withMessage, withoutMessage)
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
	m.Height = 40
	v := renderHelpOverlay(m)
	if !strings.Contains(v, "Press any key to dismiss") {
		t.Error("renderHelpOverlay should show dismiss footer")
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
	if !strings.Contains(v, "/exit") {
		t.Error("renderHelpOverlay should show /exit command")
	}
	if !strings.Contains(v, "/handoff") {
		t.Error("renderHelpOverlay should show /handoff command")
	}
	if !strings.Contains(v, "Search preview") {
		t.Error("renderHelpOverlay should show handoff preview search shortcut")
	}
	if !strings.Contains(v, "Session List") {
		t.Error("renderHelpOverlay should show Session List section")
	}
}

func TestHandoffPreviewHelpEntriesIncludeSearch(t *testing.T) {
	if !keyEntriesContain(handoffPreviewEntries(), "/", "Search preview") {
		t.Fatal("handoff preview help entries should include search shortcut")
	}
	if !keyEntriesContain(handoffPreviewEntries(), "Enter", "Next search match") {
		t.Fatal("handoff preview help entries should include next match shortcut")
	}
	if !keyEntriesContain(compactHandoffPreviewEntries(), "/", "Search preview") {
		t.Fatal("compact handoff preview help entries should include search shortcut")
	}
	if !keyEntriesContain(compactHandoffPreviewEntries(), "Enter", "Next match") {
		t.Fatal("compact handoff preview help entries should include next match shortcut")
	}
}

func TestHandoffPreviewHelpEntriesIncludeTodoShortcut(t *testing.T) {
	if !keyEntriesContain(handoffPreviewEntries(), "t", "Open todo list") {
		t.Fatal("handoff preview help entries should include todo shortcut")
	}
	if !keyEntriesContain(compactHandoffPreviewEntries(), "t", "Open todo list") {
		t.Fatal("compact handoff preview help entries should include todo shortcut")
	}
}

func keyEntriesContain(entries []keyEntry, key, desc string) bool {
	for _, entry := range entries {
		if entry.key == key && entry.desc == desc {
			return true
		}
	}
	return false
}

func TestRenderHandoffPreview(t *testing.T) {
	m := testModel()
	m.CurrentView = HandoffPreview
	m.HandoffContent = "# Handoff Summary\nSome content here"
	m.Width = 80
	m.Height = 24
	v := renderHandoffPreview(m)
	if !strings.Contains(v, "Handoff Preview") {
		t.Error("renderHandoffPreview should show header label")
	}
	if !strings.Contains(v, "y copy") {
		t.Error("renderHandoffPreview should show y copy footer")
	}
	if !strings.Contains(v, "s save") {
		t.Error("renderHandoffPreview should show s save footer")
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

func TestRenderHandoffPreviewShowsContent(t *testing.T) {
	m := testModel()
	m.CurrentView = HandoffPreview
	m.HandoffContent = "Some content text"
	m.Width = 80
	m.Height = 24
	v := renderHandoffPreview(m)
	if v == "" {
		t.Error("renderHandoffPreview should return non-empty view")
	}
	if !strings.Contains(v, "Handoff Preview") {
		t.Error("renderHandoffPreview should show header")
	}
}

func TestRenderHandoffPreviewUsesCachedBodyLines(t *testing.T) {
	m := testModel()
	m.CurrentView = HandoffPreview
	m.HandoffContent = "# Uncached content"
	m.handoffBodyLineWidth = previewLineWidth(80)
	m.handoffBodyLines = []string{"cached body line"}
	m.Width = 80
	m.Height = 8

	v := renderHandoffPreview(m)
	if !strings.Contains(v, "cached body line") {
		t.Fatalf("renderHandoffPreview should use cached body lines, got:\n%s", v)
	}
	if strings.Contains(v, "Uncached content") {
		t.Fatalf("renderHandoffPreview should not render uncached body content when cache is valid, got:\n%s", v)
	}
}

func TestRenderHandoffBodyStylesHeadingsAndDiff(t *testing.T) {
	m := testModel()
	m.CurrentView = HandoffPreview
	m.HandoffContent = "# Title\n\n## Subheading\n\n```diff\nfile_a.go\n+added\n-removed\n\nfile_b.go\n+more\n```"
	m.Width = 80
	m.Height = 24

	v := renderHandoffBody(m)
	if strings.Contains(v, "## Subheading") {
		t.Error("renderHandoffBody should not render raw markdown heading prefixes")
	}
	if strings.Contains(v, "```diff") {
		t.Error("renderHandoffBody should not show raw fenced code markers")
	}
	if !strings.Contains(v, "Subheading") {
		t.Error("renderHandoffBody should preserve heading text")
	}
	if strings.Count(v, "╭─ code ─") != 2 {
		t.Errorf("renderHandoffBody should frame each file diff separately, got %d frames", strings.Count(v, "╭─ code ─"))
	}
	if !strings.Contains(v, "\x1b[") {
		t.Error("renderHandoffBody should include ANSI styling")
	}
}

func TestPrepareHandoffPreviewMarkdownSplitsDiffByFile(t *testing.T) {
	content := "## Changes\n\n```diff\nfile_a.go\n+added\n\nfile_b.go\n-removed\n```"
	preview := prepareHandoffPreviewMarkdown(content)
	if strings.Count(preview, "```diff") != 2 {
		t.Errorf("expected two diff fences, got %d", strings.Count(preview, "```diff"))
	}
	if !strings.Contains(preview, "#### "+handoffDiffExpandedMarker+" file_a.go") || !strings.Contains(preview, "#### "+handoffDiffExpandedMarker+" file_b.go") {
		t.Errorf("expected per-file headings in preview markdown, got:\n%s", preview)
	}
}

func TestRenderHandoffPreviewKeepsHeaderAndFooterWhenScrolled(t *testing.T) {
	m := testModel()
	m.CurrentView = HandoffPreview
	m.HandoffContent = "# Handoff Summary\n\nline 1 with enough text to wrap if not clamped properly\nline 2\nline 3\nline 4\nline 5\nline 6"
	m.Width = 80
	m.Height = 6
	m.ScrollOffset = 4

	v := renderHandoffPreview(m)
	lines := strings.Split(v, "\n")
	wantLines := previewViewportHeight(m.Height)
	if len(lines) != wantLines {
		t.Fatalf("renderHandoffPreview returned %d lines, want %d:\n%s", len(lines), wantLines, v)
	}
	if !strings.Contains(v, "Handoff Preview") {
		t.Error("renderHandoffPreview should keep the header visible while scrolled")
	}
	if !strings.Contains(v, "[y Copy]") || !strings.Contains(v, "[s Save]") {
		t.Error("renderHandoffPreview should keep action buttons visible while scrolled")
	}
	if !strings.Contains(v, "y copy") || !strings.Contains(v, "s save") {
		t.Error("renderHandoffPreview should keep footer shortcuts visible while scrolled")
	}
	if !strings.Contains(xansi.Strip(lines[len(lines)-1]), "y copy") {
		t.Fatalf("renderHandoffPreview should keep footer on the bottom row, got %q", xansi.Strip(lines[len(lines)-1]))
	}
}

func TestRenderHandoffPreviewDoesNotOverflowViewportWidth(t *testing.T) {
	m := testModel()
	m.CurrentView = HandoffPreview
	m.HandoffContent = "# Handoff Summary\n\n```diff\ninternal/tui/very_long_file_name_that_should_not_wrap_in_the_terminal.go\n+this\tis a very long diff line that should be truncated before it can force terminal autowrap and push the header out of view\n```"
	m.Width = 24
	m.Height = 8

	v := renderHandoffPreview(m)
	lines := strings.Split(v, "\n")
	if len(lines) > m.Height {
		t.Fatalf("renderHandoffPreview returned %d lines, want at most %d:\n%s", len(lines), m.Height, v)
	}
	for i, line := range lines {
		if got := xansi.StringWidth(line); got > previewLineWidth(m.Width) {
			t.Fatalf("line %d width = %d, want <= %d: %q", i, got, previewLineWidth(m.Width), xansi.Strip(line))
		}
	}
	if !strings.Contains(v, "Handoff Preview") {
		t.Error("renderHandoffPreview should keep header visible in narrow view")
	}
	if !strings.Contains(v, "y/s/d") {
		t.Error("renderHandoffPreview should keep compact footer visible in narrow view")
	}
}

func TestHandoffButtonsHiddenWhenPreviewIsTooNarrow(t *testing.T) {
	m := testModel()
	m.Width = 36

	v := renderHandoffHeader(m)
	if strings.Contains(v, handoffCopyButton) || strings.Contains(v, handoffSaveButton) {
		t.Fatalf("narrow header should omit action buttons to prevent terminal autowrap, got %q", xansi.Strip(v))
	}

	copyStart, copyEnd, saveStart, saveEnd := handoffButtonBounds(m.Width)
	if copyStart != -1 || copyEnd != -1 || saveStart != -1 || saveEnd != -1 {
		t.Fatalf("narrow button bounds = %d %d %d %d, want all -1", copyStart, copyEnd, saveStart, saveEnd)
	}
}

func TestRenderHandoffPreviewUsesCompactViewportAtWideWidths(t *testing.T) {
	m := testModel()
	m.CurrentView = HandoffPreview
	m.HandoffContent = "# Handoff Summary\n\nline 1\nline 2\nline 3\nline 4\nline 5\nline 6\nline 7\nline 8\nline 9\nline 10\nline 11\nline 12\nline 13\nline 14\nline 15\nline 16\nline 17\nline 18\nline 19\nline 20"
	m.Width = 200
	m.Height = 60

	v := renderHandoffPreview(m)
	lines := strings.Split(v, "\n")
	wantLines := previewViewportHeight(m.Height)
	if len(lines) != wantLines {
		t.Fatalf("renderHandoffPreview returned %d lines, want %d", len(lines), wantLines)
	}
	prefix := strings.Repeat(" ", previewLeftPadding(m.Width))
	for i, line := range lines {
		if !strings.HasPrefix(line, prefix) {
			t.Fatalf("line %d should include centered preview padding", i)
		}
		unpadded := strings.TrimPrefix(line, prefix)
		if got := xansi.StringWidth(unpadded); got > maxHandoffPreviewWidth {
			t.Fatalf("line %d content width = %d, want <= %d", i, got, maxHandoffPreviewWidth)
		}
		if got := xansi.StringWidth(line); got > m.Width-terminalSafetyCols {
			t.Fatalf("line %d total width = %d, want <= %d", i, got, m.Width-terminalSafetyCols)
		}
	}
	if !strings.Contains(v, "Handoff Preview") || !strings.Contains(v, "y copy") {
		t.Fatalf("compact preview should keep header and footer visible:\n%s", v)
	}
	if !strings.Contains(xansi.Strip(lines[len(lines)-1]), "y copy") {
		t.Fatalf("compact preview should keep footer on the bottom row, got %q", xansi.Strip(lines[len(lines)-1]))
	}
}

func TestPreviewLeftPaddingCentersWideViewport(t *testing.T) {
	if got := previewLeftPadding(200); got <= 0 {
		t.Fatalf("previewLeftPadding(200) = %d, want positive padding", got)
	}
	if got := previewLeftPadding(36); got != 0 {
		t.Fatalf("previewLeftPadding(36) = %d, want 0 for narrow viewport", got)
	}
}

func TestClampHandoffScrollOffsetKeepsFullPage(t *testing.T) {
	got := clampHandoffScrollOffset(99, 10, 4)
	if got != 6 {
		t.Fatalf("clampHandoffScrollOffset() = %d, want last full page offset 6", got)
	}

	got = clampHandoffScrollOffset(99, 3, 4)
	if got != 0 {
		t.Fatalf("clampHandoffScrollOffset() = %d, want 0 when content fits", got)
	}
}

func TestRenderHandoffPreviewShowsSavePrompt(t *testing.T) {
	m := testModel()
	m.CurrentView = HandoffPreview
	m.HandoffContent = "# Handoff Summary"
	m.SavePromptOpen = true
	m.SaveInput = "2026-01-15T143022Z"
	m.Width = 80
	m.Height = 24

	v := renderHandoffPreview(m)
	if !strings.Contains(v, "Save as:") {
		t.Error("renderHandoffPreview should show save prompt when SavePromptOpen is true")
	}
	if !strings.Contains(v, "2026-01-15T143022Z") {
		t.Error("renderHandoffPreview should show default save filename")
	}
}

func TestHandleHandoffKeyOpensSavePrompt(t *testing.T) {
	m := testModel()
	m.CurrentView = HandoffPreview
	m.ActiveSession = testActiveSession()
	m.HandoffContent = "# Handoff Summary"

	updatedModel, cmd := handleHandoffKey(&m, "s")
	updated, ok := updatedModel.(Model)
	if !ok {
		t.Fatalf("expected Model from handleHandoffKey, got %T", updatedModel)
	}
	if !updated.SavePromptOpen {
		t.Fatal("pressing s should open save prompt")
	}
	if updated.SaveInput != "2026-01-15T143022Z" {
		t.Errorf("SaveInput = %q, want active session ID", updated.SaveInput)
	}
	if cmd == nil {
		t.Error("pressing s should start cursor tick command")
	}
}

func TestHandleHandoffKeyQRestoresActiveSessionScroll(t *testing.T) {
	m := testModel()
	m.CurrentView = HandoffPreview
	m.ActiveSession = testActiveSession()
	m.Events = scrollRestorationEvents()
	m.Width = 80
	m.Height = 12
	m.ScrollOffset = 9
	m.activeSessionScrollOffset = 4

	updatedModel, cmd := handleHandoffKey(&m, "q")
	if cmd == nil {
		t.Fatal("pressing q should clear the screen when returning to active session")
	}
	updated, ok := updatedModel.(Model)
	if !ok {
		t.Fatalf("expected Model from handleHandoffKey, got %T", updatedModel)
	}
	if updated.CurrentView != ActiveSession {
		t.Fatalf("CurrentView = %v, want ActiveSession", updated.CurrentView)
	}
	if updated.ScrollOffset != 4 {
		t.Fatalf("ScrollOffset = %d, want restored active session offset 4", updated.ScrollOffset)
	}
}

func TestHandleHandoffKeyTOpensTodoOverlay(t *testing.T) {
	m := testModel()
	m.CurrentView = HandoffPreview
	m.ActiveSession = testActiveSession()
	m.TodoOpen = false

	updatedModel, cmd := handleHandoffKey(&m, "t")
	updated, ok := updatedModel.(Model)
	if !ok {
		t.Fatalf("expected Model from handleHandoffKey, got %T", updatedModel)
	}
	if !updated.TodoOpen {
		t.Fatal("pressing t should open the todo overlay")
	}
	if cmd == nil {
		t.Fatal("pressing t should return a load items command")
	}
}

func TestHandleHandoffMouseIgnoresHover(t *testing.T) {
	m := testModel()
	m.CurrentView = HandoffPreview
	m.ActiveSession = testActiveSession()
	m.HandoffContent = "# Handoff Summary"
	m.Width = 80

	copyStart, _, _, _ := handoffButtonBounds(m.Width)
	updatedModel, cmd := handleHandoffMouse(&m, tea.MouseMsg{
		X:      copyStart,
		Y:      0,
		Action: tea.MouseActionMotion,
		Button: tea.MouseButtonNone,
	})
	updated, ok := updatedModel.(Model)
	if !ok {
		t.Fatalf("expected Model from handleHandoffMouse, got %T", updatedModel)
	}
	if cmd != nil {
		t.Error("hovering copy button should not execute copy command")
	}
	if updated.SavePromptOpen {
		t.Error("hovering buttons should not open save prompt")
	}
}

func TestHandleHandoffMouseSavePressOpensPrompt(t *testing.T) {
	m := testModel()
	m.CurrentView = HandoffPreview
	m.ActiveSession = testActiveSession()
	m.HandoffContent = "# Handoff Summary"
	m.Width = 80

	_, _, saveStart, _ := handoffButtonBounds(m.Width)
	updatedModel, cmd := handleHandoffMouse(&m, tea.MouseMsg{
		X:      saveStart,
		Y:      0,
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonLeft,
	})
	updated, ok := updatedModel.(Model)
	if !ok {
		t.Fatalf("expected Model from handleHandoffMouse, got %T", updatedModel)
	}
	if !updated.SavePromptOpen {
		t.Fatal("clicking save button should open save prompt")
	}
	if updated.SaveInput != "2026-01-15T143022Z" {
		t.Errorf("SaveInput = %q, want active session ID", updated.SaveInput)
	}
	if cmd == nil {
		t.Error("clicking save should start cursor tick command")
	}
}

func TestExtractStartMessage(t *testing.T) {
	events := []store.SessionEvent{
		{Type: "Start", Body: "  Implement auth middleware  "},
		{Type: "Note", Time: testEventTime(14, 30), Body: "Added JWT"},
	}
	title := extractStartMessage(events)
	if title != "Implement auth middleware" {
		t.Errorf("extractStartMessage = %q, want 'Implement auth middleware'", title)
	}
}

func TestExtractStartMessageEmpty(t *testing.T) {
	events := []store.SessionEvent{
		{Type: "Note", Time: testEventTime(14, 30), Body: "Added JWT"},
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
		{Type: "Note", Time: testEventTime(14, 30), Body: "note 1"},
		{Type: "Start", Body: "another"},
		{Type: "Blocker", Time: testEventTime(15, 0), Body: "blocked"},
		{Type: "Stop", Time: testEventTime(16, 0), Body: "done"},
	}
	filtered := filterVisibleEvents(events)
	if len(filtered) != 3 {
		t.Fatalf("expected 3 non-Start events, got %d", len(filtered))
	}
	for _, e := range filtered {
		if e.Type == "Start" {
			t.Errorf("filterVisibleEvents should not include Start events, got: %v", e)
		}
		if e.IsDeleted {
			t.Errorf("filterVisibleEvents should not include deleted events, got: %v", e)
		}
	}
}

func TestFilterVisibleEventsExcludesDeleted(t *testing.T) {
	events := []store.SessionEvent{
		{Type: "Start", Body: "title"},
		{Type: "Note", Time: testEventTime(14, 30), Body: "note 1"},
		{Type: "Note", Time: testEventTime(14, 35), Body: "deleted", IsDeleted: true},
		{Type: "Blocker", Time: testEventTime(15, 0), Body: "blocked"},
	}
	filtered := filterVisibleEvents(events)
	if len(filtered) != 2 {
		t.Fatalf("expected 2 visible events, got %d", len(filtered))
	}
	for _, e := range filtered {
		if e.IsDeleted {
			t.Errorf("filterVisibleEvents should exclude deleted events, got: %v", e)
		}
		if e.Type == "Start" {
			t.Errorf("filterVisibleEvents should exclude Start events, got: %v", e)
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
	m.ActiveSession = testActiveSession()
	m.Title = "Visible base title"
	m.ShowHelp = true
	m.Width = 80
	m.Height = 42
	v := m.View()
	if !strings.Contains(v, "Press any key to dismiss") {
		t.Error("Model.View() should show help when ShowHelp is true")
	}
	if !strings.Contains(v, "Visible base title") {
		t.Error("Model.View() should preserve the underlying view behind help")
	}
}

func TestHelpOverlayMouseDoesNotScrollUnderlyingView(t *testing.T) {
	m := testScrollableActiveModel()
	m.ShowHelp = true
	m.ScrollOffset = 3

	updatedModel, cmd := m.Update(tea.MouseMsg{Button: tea.MouseButtonWheelDown})
	updated, ok := updatedModel.(Model)
	if !ok {
		t.Fatalf("expected Model after mouse update, got %T", updatedModel)
	}
	if cmd != nil {
		t.Fatal("help mouse handling should not start a command")
	}
	if updated.ScrollOffset != 3 {
		t.Fatalf("ScrollOffset = %d, want unchanged 3", updated.ScrollOffset)
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
