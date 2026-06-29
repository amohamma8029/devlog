package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/atotto/clipboard"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	glamouransi "github.com/charmbracelet/glamour/ansi"
	"github.com/charmbracelet/glamour/styles"
	"github.com/charmbracelet/lipgloss"
	xansi "github.com/charmbracelet/x/ansi"

	internalconfig "github.com/amo/devlog/internal/config"
)

const (
	handoffCopyButton          = "[y Copy]"
	handoffSaveButton          = "[s Save]"
	maxHandoffPreviewWidth     = 72
	defaultHandoffPreviewRows  = 18
	terminalSafetyCols         = 4
	terminalSafetyRows         = 1
	handoffDiffExpandedMarker  = "▼"
	handoffDiffCollapsedMarker = "▶"
	inputOverflowMarker        = "…"
)

func renderHandoffPreview(m Model) string {
	if m.HandoffContent == "" {
		return "No handoff content available."
	}

	var b strings.Builder

	lineWidth := previewLineWidth(m.Width)
	header := clampPreviewLine(strings.TrimRight(renderHandoffHeader(m), "\n"), lineWidth)
	footer := clampPreviewLine(renderFooter(m), lineWidth)
	prompt := ""
	if m.SavePromptOpen {
		prompt = renderSavePrompt(m)
	}
	if m.CollapsedDiffConfirmOpen {
		prompt = renderCollapsedDiffPrompt(m, lineWidth)
	}
	searchPrompt := ""
	if m.Search.Open {
		searchPrompt = renderSearchPrompt(m, 0)
	}
	messages := renderHandoffMessages(m, lineWidth)

	height := previewViewportHeight(m.Height)
	reservedLines := countLines(header) + countLines(footer)
	if prompt != "" {
		reservedLines += countLines(prompt)
	}
	if searchPrompt != "" {
		reservedLines += countLines(searchPrompt)
	}
	reservedLines += len(messages)

	contentLines := height - reservedLines
	if contentLines < 1 {
		contentLines = 1
	}

	bodyLines := handoffBodyLines(m)

	scrollOffset := clampHandoffScrollOffset(m.ScrollOffset, len(bodyLines), contentLines)
	bodyLines = applyHandoffDiffCursorStyle(bodyLines, m.HandoffSelectedDiff)

	end := scrollOffset + contentLines
	if end > len(bodyLines) {
		end = len(bodyLines)
	}
	visible := bodyLines[scrollOffset:end]

	searchMatches := findSearchMatches(m.Search.Query, bodyLines)
	matchCount := len(searchMatches)

	if len(searchMatches) > 0 && m.Search.Open {
		highlighted := make([]string, len(visible))
		copy(highlighted, visible)
		activeMatchIndex, hasActiveMatch := selectedSearchMatchIndex(m.Search, matchCount)
		for i, line := range highlighted {
			lineIdx := scrollOffset + i
			var lineMatches []SearchMatch
			var activeMatch *SearchMatch
			for matchIdx, match := range searchMatches {
				if match.Line == lineIdx {
					lineMatches = append(lineMatches, match)
					if hasActiveMatch && matchIdx == activeMatchIndex {
						active := match
						activeMatch = &active
					}
				}
			}
			if len(lineMatches) > 0 {
				highlighted[i] = highlightLineWithActiveMatch(line, lineMatches, activeMatch)
			}
		}
		visible = highlighted
	}

	if m.Search.Open {
		searchPrompt = renderSearchPrompt(m, matchCount)
	}

	remaining := contentLines - len(visible)
	if remaining < 0 {
		remaining = 0
	}
	if remaining > 0 {
		visible = append(visible, make([]string, remaining)...)
	}

	b.WriteString(header)
	b.WriteByte('\n')
	b.WriteString(strings.Join(visible, "\n"))
	if prompt != "" {
		b.WriteByte('\n')
		b.WriteString(prompt)
	}
	if len(messages) > 0 {
		b.WriteByte('\n')
		b.WriteString(strings.Join(messages, "\n"))
	}
	if searchPrompt != "" {
		b.WriteByte('\n')
		b.WriteString(searchPrompt)
	}
	b.WriteByte('\n')
	b.WriteString(footer)

	return indentPreviewLines(b.String(), previewLeftPadding(m.Width))
}

func previewLineWidth(width int) int {
	if width <= 0 {
		return maxHandoffPreviewWidth
	}
	if width > maxHandoffPreviewWidth+terminalSafetyCols {
		return maxHandoffPreviewWidth
	}
	safeWidth := width - terminalSafetyCols
	if safeWidth > 0 {
		return safeWidth
	}
	return 1
}

func previewViewportHeight(height int) int {
	if height <= 0 {
		return defaultHandoffPreviewRows
	}
	if height <= terminalSafetyRows {
		return height
	}
	return height - terminalSafetyRows
}

func previewLeftPadding(width int) int {
	if width <= 0 {
		return 0
	}
	availableWidth := width - terminalSafetyCols
	lineWidth := previewLineWidth(width)
	if availableWidth <= lineWidth {
		return 0
	}
	return (availableWidth - lineWidth) / 2
}

func indentPreviewLines(s string, padding int) string {
	if padding <= 0 || s == "" {
		return s
	}

	prefix := strings.Repeat(" ", padding)
	lines := strings.Split(s, "\n")
	for i := range lines {
		lines[i] = prefix + lines[i]
	}
	return strings.Join(lines, "\n")
}

func renderHandoffMessages(m Model, width int) []string {
	var lines []string
	if m.ErrorMessage != "" {
		lines = append(lines, clampPreviewLine(ErrorBannerStyle.Render(" ERROR: "+m.ErrorMessage), width))
	}
	if m.HandoffMsg != "" {
		lines = append(lines, clampPreviewLine(HintStyle.Render(formatHandoffConfirmation(m.HandoffMsg)), width))
	}
	return lines
}

func clampHandoffScrollOffset(offset, bodyLines, contentLines int) int {
	if offset < 0 {
		return 0
	}
	maxOffset := bodyLines - contentLines
	if maxOffset < 0 {
		maxOffset = 0
	}
	if offset > maxOffset {
		return maxOffset
	}
	return offset
}

func clampPreviewLines(lines []string, width int) []string {
	clamped := make([]string, len(lines))
	for i, line := range lines {
		clamped[i] = clampPreviewLine(line, width)
	}
	return clamped
}

func clampPreviewLine(line string, width int) string {
	line = strings.TrimRight(strings.ReplaceAll(line, "\t", "    "), "\r")
	if width < 1 {
		width = 1
	}
	if xansi.StringWidth(line) <= width {
		return line
	}
	return xansi.Truncate(line, width, "")
}

func splitRenderedLines(rendered string) []string {
	rendered = strings.Trim(rendered, "\n")
	if rendered == "" {
		return []string{""}
	}
	return strings.Split(rendered, "\n")
}

func renderHandoffHeader(m Model) string {
	buttonsText := handoffCopyButton + " " + handoffSaveButton

	width := previewLineWidth(m.Width)

	labelText := "Handoff Preview"
	label := HandoffHeaderStyle.Render(labelText)
	if !handoffButtonsVisible(m.Width) {
		return label + "\n"
	}

	buttons := HandoffButtonStyle.Render(buttonsText)
	spacerLen := width - len(labelText) - len(buttonsText) - 2
	if spacerLen < 1 {
		spacerLen = 1
	}
	spacer := strings.Repeat(" ", spacerLen)

	return label + spacer + buttons + "\n"
}

func renderHandoffBody(m Model) string {
	width := previewLineWidth(m.Width) - 2
	if width < 1 {
		width = 1
	}

	previewMarkdown := prepareHandoffPreviewMarkdownForModel(m)

	r, err := glamour.NewTermRenderer(
		glamour.WithStyles(handoffGlamourStyle()),
		glamour.WithChromaFormatter("terminal256"),
		glamour.WithWordWrap(width),
	)
	if err != nil {
		return previewMarkdown
	}

	rendered, err := r.Render(previewMarkdown)
	if err != nil {
		return previewMarkdown
	}

	return styleTodoListRendered(rendered)
}

func handoffBodyLines(m Model) []string {
	lineWidth := previewLineWidth(m.Width)
	if m.handoffBodyLines != nil && m.handoffBodyLineWidth == lineWidth {
		return m.handoffBodyLines
	}
	return clampPreviewLines(splitRenderedLines(renderHandoffBody(m)), lineWidth)
}

func prepareHandoffPreviewMarkdown(content string) string {
	return transformTodoListSection(formatHandoffMarkdown(content, nil, handoffMarkdownOptions{
		DiffLineLimit:               internalconfig.DefaultHandoffPreviewLineLimit,
		IncludeCollapsedPlaceholder: true,
		ShowDisclosureArrows:        true,
	}))
}

func prepareHandoffPreviewMarkdownForModel(m Model) string {
	return transformTodoListSection(formatHandoffMarkdown(m.HandoffContent, m.HandoffCollapsedDiffs, handoffMarkdownOptions{
		DiffLineLimit:               m.Config.TUI.HandoffPreview.DiffLineLimit,
		IncludeCollapsedPlaceholder: true,
		ShowDisclosureArrows:        true,
	}))
}

func applyHandoffDiffCursorStyle(bodyLines []string, selected string) []string {
	if selected == "" || len(bodyLines) == 0 {
		return bodyLines
	}
	start, end := selectedDiffLineRange(bodyLines, selected)
	if start < 0 {
		return bodyLines
	}
	styled := make([]string, len(bodyLines))
	for i, line := range bodyLines {
		if i >= start && i < end && strings.Contains(line, glamourDiffBgSGR) {
			styled[i] = strings.ReplaceAll(line, glamourDiffBgSGR, handoffSelectedBgOpen)
			continue
		}
		styled[i] = line
	}
	return styled
}

const (
	glamourDiffBgSGR      = "\x1b[48;5;235m"
	handoffSelectedBgOpen = "\x1b[48;5;236m"
)

func selectedDiffLineRange(bodyLines []string, selected string) (int, int) {
	start := -1
	end := len(bodyLines)
	for i, line := range bodyLines {
		path := extractDiffPathFromRenderedLine(line)
		if path == "" {
			continue
		}
		if start >= 0 {
			end = i
			return start, end
		}
		if path == selected {
			start = i
		}
	}
	if start < 0 {
		return -1, -1
	}
	return start, end
}

func extractDiffPathFromRenderedLine(line string) string {
	stripped := strings.TrimSpace(xansi.Strip(line))
	if strings.HasPrefix(stripped, "•") {
		stripped = strings.TrimSpace(strings.TrimPrefix(stripped, "•"))
	} else if strings.HasPrefix(stripped, "#### ") {
		stripped = handoffDiffPathFromHeading(stripped)
	} else if strings.HasPrefix(stripped, handoffDiffExpandedMarker+" ") || strings.HasPrefix(stripped, handoffDiffCollapsedMarker+" ") {
		// H4 headings render without a bullet prefix; the disclosure arrow is the marker.
	} else {
		return ""
	}
	return trimHandoffDiffDisclosureMarker(stripped)
}

func handoffMarkdownForSave(m Model) string {
	return formatHandoffMarkdown(m.HandoffContent, m.HandoffCollapsedDiffs, handoffMarkdownOptions{
		OmitCollapsed: true,
	})
}

type handoffMarkdownOptions struct {
	DiffLineLimit               int
	OmitCollapsed               bool
	IncludeCollapsedPlaceholder bool
	ShowDisclosureArrows        bool
}

func formatHandoffMarkdown(content string, collapsed map[string]bool, opts handoffMarkdownOptions) string {
	content = normalizeLegacyDiffFences(content)
	lines := strings.Split(content, "\n")
	var out []string

	for i := 0; i < len(lines); i++ {
		line := lines[i]
		if !isHandoffDiffHeading(line) {
			out = append(out, line)
			continue
		}

		fenceStart := nextNonBlankLine(lines, i+1)
		if fenceStart >= len(lines) || strings.TrimSpace(lines[fenceStart]) != "```diff" {
			out = append(out, line)
			continue
		}

		fenceEnd := fenceStart + 1
		for ; fenceEnd < len(lines); fenceEnd++ {
			if strings.TrimSpace(lines[fenceEnd]) == "```" {
				break
			}
		}
		if fenceEnd >= len(lines) {
			out = append(out, line)
			continue
		}

		path := handoffDiffPathFromHeading(line)
		if collapsed != nil && collapsed[path] {
			if opts.OmitCollapsed {
				i = fenceEnd
				continue
			}
			out = append(out, formatHandoffDiffHeading(path, true, opts), "")
			if opts.IncludeCollapsedPlaceholder {
				out = append(out, "_Diff collapsed. Click heading to expand._", "")
			}
			i = fenceEnd
			continue
		}

		out = append(out, formatHandoffDiffHeading(path, false, opts), "", "```diff")
		diffLines := lines[fenceStart+1 : fenceEnd]
		if opts.DiffLineLimit > 0 && len(diffLines) > opts.DiffLineLimit {
			out = append(out, diffLines[:opts.DiffLineLimit]...)
			out = append(out, fmt.Sprintf("... (truncated, %d more lines)", len(diffLines)-opts.DiffLineLimit))
		} else {
			out = append(out, diffLines...)
		}
		out = append(out, "```", "")
		i = fenceEnd
	}

	return strings.Join(out, "\n")
}

// transformTodoListSection turns semantic task-list markdown into Unicode
// checkbox paragraphs for preview rendering. It intentionally avoids markdown
// list syntax so Glamour does not add bullet glyphs before the checkboxes. Each
// checkbox row is emitted as its own short paragraph so Glamour keeps rows
// vertical.
func transformTodoListSection(content string) string {
	lines := strings.Split(content, "\n")
	var out []string
	inSection := false

	for _, line := range lines {
		if strings.HasPrefix(line, "## ") {
			inSection = line == "## Todo List"
			out = append(out, line)
			continue
		}
		if !inSection {
			out = append(out, line)
			continue
		}
		if strings.TrimSpace(line) == "" {
			if len(out) == 0 || out[len(out)-1] != "" {
				out = append(out, "")
			}
			continue
		}

		if strings.HasPrefix(line, "**Completed**") {
			out = append(out, line)
			continue
		}
		if strings.HasPrefix(line, "**Open**") {
			out = append(out, line)
			continue
		}
		if strings.HasPrefix(line, "- [x] ") || strings.HasPrefix(line, "- [X] ") {
			out = append(out, "\u2611 "+line[len("- [x] "):], "")
			continue
		}
		if strings.HasPrefix(line, "- [ ] ") {
			out = append(out, "\u2610 "+strings.TrimPrefix(line, "- [ ] "), "")
			continue
		}

		out = append(out, line)
	}

	return strings.Join(out, "\n")
}

// styleTodoListRendered applies Lipgloss styling to the already-rendered
// Glamour output. It only styles actual section heading lines, then indents and
// styles rows while inside the Todo List section.
func styleTodoListRendered(rendered string) string {
	lines := strings.Split(rendered, "\n")
	inTodoList := false
	for i, line := range lines {
		stripped := xansi.Strip(line)
		if heading, ok := renderedHeadingName(stripped); ok {
			switch heading {
			case "Todo List":
				lines[i] = TodoListHeadingStyle.Render(strings.TrimSpace(stripped))
				inTodoList = true
				continue
			case "Changes":
				lines[i] = ChangesHeadingStyle.Render(strings.TrimSpace(stripped))
				inTodoList = false
				continue
			default:
				inTodoList = false
			}
		}
		if !inTodoList {
			continue
		}

		trimmed := strings.TrimSpace(stripped)
		if trimmed == "Completed" || trimmed == "Open" {
			lines[i] = "  " + TodoListSubheadingStyle.Render(trimmed)
			continue
		}
		if strings.HasPrefix(trimmed, "\u2611 ") {
			text := strings.TrimSpace(strings.TrimPrefix(trimmed, "\u2611"))
			lines[i] = "    " + TodoDoneCheckboxStyle.Render("\u2611") + " " + TodoCompletedTextStyle.Render(text)
			continue
		}
		if strings.HasPrefix(trimmed, "\u2610 ") {
			text := strings.TrimSpace(strings.TrimPrefix(trimmed, "\u2610"))
			lines[i] = "    " + TodoOpenCheckboxStyle.Render("\u2610") + " " + text
		}
	}
	return strings.Join(lines, "\n")
}

func renderedHeadingName(stripped string) (string, bool) {
	trimmed := strings.TrimSpace(stripped)
	if !strings.HasPrefix(trimmed, "▌ ") {
		return "", false
	}
	heading := strings.TrimSpace(strings.TrimPrefix(trimmed, "▌ "))
	if heading == "" {
		return "", false
	}
	return heading, true
}

func normalizeLegacyDiffFences(content string) string {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	lines := strings.Split(content, "\n")
	var out []string

	for i := 0; i < len(lines); i++ {
		line := lines[i]
		if strings.TrimSpace(line) != "```diff" || previousNonBlankIsDiffHeading(lines, i) {
			out = append(out, line)
			continue
		}

		var diffLines []string
		j := i + 1
		for ; j < len(lines); j++ {
			if strings.TrimSpace(lines[j]) == "```" {
				break
			}
			diffLines = append(diffLines, lines[j])
		}

		if j >= len(lines) {
			out = append(out, line)
			out = append(out, diffLines...)
			continue
		}

		out = append(out, splitDiffFenceByFile(diffLines)...)
		i = j
	}

	return strings.Join(out, "\n")
}

func splitDiffFenceByFile(lines []string) []string {
	type diffSection struct {
		path string
		body []string
	}

	var sections []diffSection
	current := diffSection{}

	flush := func() {
		if current.path == "" && len(current.body) == 0 {
			return
		}
		sections = append(sections, current)
		current = diffSection{}
	}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if isPreviewDiffFileHeader(line) {
			flush()
			current.path = trimmed
			continue
		}
		current.body = append(current.body, line)
	}
	flush()

	if len(sections) == 0 {
		return []string{"```diff", "```"}
	}

	var out []string
	for _, section := range sections {
		if section.path != "" {
			out = append(out, "#### "+section.path, "")
		}
		out = append(out, "```diff")
		out = append(out, section.body...)
		out = append(out, "```", "")
	}
	return out
}

func isPreviewDiffFileHeader(line string) bool {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return false
	}
	return !strings.HasPrefix(trimmed, "+") && !strings.HasPrefix(trimmed, "-")
}

func isHandoffDiffHeading(line string) bool {
	return strings.HasPrefix(strings.TrimSpace(line), "#### ")
}

func handoffDiffPathFromHeading(line string) string {
	path := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "#### "))
	return strings.TrimSpace(trimHandoffDiffDisclosureMarker(path))
}

func formatHandoffDiffHeading(path string, collapsed bool, opts handoffMarkdownOptions) string {
	if !opts.ShowDisclosureArrows {
		return "#### " + path
	}
	marker := handoffDiffExpandedMarker
	if collapsed {
		marker = handoffDiffCollapsedMarker
	}
	return "#### " + marker + " " + path
}

func trimHandoffDiffDisclosureMarker(path string) string {
	path = strings.TrimSpace(path)
	for _, marker := range []string{handoffDiffExpandedMarker, handoffDiffCollapsedMarker} {
		if strings.HasPrefix(path, marker+" ") {
			return strings.TrimSpace(strings.TrimPrefix(path, marker))
		}
	}
	return path
}

func nextNonBlankLine(lines []string, start int) int {
	for i := start; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) != "" {
			return i
		}
	}
	return len(lines)
}

func previousNonBlankIsDiffHeading(lines []string, index int) bool {
	for i := index - 1; i >= 0; i-- {
		if strings.TrimSpace(lines[i]) == "" {
			continue
		}
		return isHandoffDiffHeading(lines[i])
	}
	return false
}

func handoffDiffPaths(content string) []string {
	content = normalizeLegacyDiffFences(content)
	lines := strings.Split(content, "\n")
	var paths []string

	for i := 0; i < len(lines); i++ {
		if !isHandoffDiffHeading(lines[i]) {
			continue
		}
		fenceStart := nextNonBlankLine(lines, i+1)
		if fenceStart < len(lines) && strings.TrimSpace(lines[fenceStart]) == "```diff" {
			paths = append(paths, handoffDiffPathFromHeading(lines[i]))
		}
	}

	return paths
}

func toggleAllHandoffDiffs(m *Model) {
	paths := handoffDiffPaths(m.HandoffContent)
	if len(paths) == 0 {
		return
	}

	shouldCollapse := false
	for _, path := range paths {
		if m.HandoffCollapsedDiffs == nil || !m.HandoffCollapsedDiffs[path] {
			shouldCollapse = true
			break
		}
	}

	if shouldCollapse {
		m.HandoffCollapsedDiffs = make(map[string]bool, len(paths))
		for _, path := range paths {
			m.HandoffCollapsedDiffs[path] = true
		}
		return
	}

	m.HandoffCollapsedDiffs = nil
}

func toggleHandoffDiff(m *Model, path string) {
	if strings.TrimSpace(path) == "" {
		return
	}
	if m.HandoffCollapsedDiffs == nil {
		m.HandoffCollapsedDiffs = make(map[string]bool)
	}
	if m.HandoffCollapsedDiffs[path] {
		delete(m.HandoffCollapsedDiffs, path)
		if len(m.HandoffCollapsedDiffs) == 0 {
			m.HandoffCollapsedDiffs = nil
		}
		return
	}
	m.HandoffCollapsedDiffs[path] = true
}

func handoffDiffPathLineIndex(m Model, path string) int {
	bodyLines := handoffBodyLines(m)
	headingLine := handoffDiffHeadingLineIndex(bodyLines, path)
	if headingLine >= 0 {
		return headingLine
	}
	for i, line := range bodyLines {
		if handoffDiffPathFromRenderedLine(line, handoffDiffPaths(m.HandoffContent)) == path {
			return i
		}
	}
	return -1
}

func handoffDiffHeadingLineIndex(bodyLines []string, path string) int {
	if path == "" {
		return -1
	}
	for i, line := range bodyLines {
		stripped := strings.TrimSpace(xansi.Strip(line))
		if !strings.HasPrefix(stripped, "▌ ") && !strings.HasPrefix(strings.TrimSpace(stripped), "#### ") && !strings.HasPrefix(strings.TrimSpace(stripped), handoffDiffExpandedMarker+" ") && !strings.HasPrefix(strings.TrimSpace(stripped), handoffDiffCollapsedMarker+" ") {
			continue
		}
		if handoffDiffPathFromRenderedLine(line, []string{path}) == path {
			return i
		}
	}
	return -1
}

func (m *Model) syncHandoffDiffCursor() {
	paths := handoffDiffPaths(m.HandoffContent)
	if len(paths) == 0 {
		m.HandoffSelectedDiff = ""
		return
	}

	bodyLines := handoffBodyLines(*m)
	contentLines := handoffPreviewContentLines(*m)
	scrollOffset := clampHandoffScrollOffset(m.ScrollOffset, len(bodyLines), contentLines)
	if scrollOffset == 0 {
		m.HandoffSelectedDiff = ""
		return
	}
	visible := bodyLines[scrollOffset:]
	if len(visible) > contentLines {
		visible = visible[:contentLines]
	}

	// Scan forward for the first visible heading.
	for _, line := range visible {
		if path := handoffDiffPathFromRenderedLine(line, paths); path != "" {
			m.HandoffSelectedDiff = path
			return
		}
	}

	// No heading visible — scan backward from the scroll offset to find the
	// nearest preceding heading. This covers the case where the user has
	// scrolled past a heading into its diff body.
	for i := scrollOffset - 1; i >= 0; i-- {
		if path := handoffDiffPathFromRenderedLine(bodyLines[i], paths); path != "" {
			m.HandoffSelectedDiff = path
			return
		}
	}

	// No heading visible forward or backward. If the cursor was empty, keep
	// it empty — the user hasn't scrolled into the diffs area yet. If it was
	// set, keep it only if still valid.
	if m.HandoffSelectedDiff != "" {
		for _, p := range paths {
			if p == m.HandoffSelectedDiff {
				return
			}
		}
	}
	m.HandoffSelectedDiff = ""
}

func ensureHandoffDiffVisible(m *Model, path string) {
	if path == "" {
		return
	}
	bodyLines := handoffBodyLines(*m)
	if len(bodyLines) == 0 {
		return
	}
	contentLines := handoffPreviewContentLines(*m)
	headingLine := handoffDiffHeadingLineIndex(bodyLines, path)
	if headingLine < 0 {
		return
	}
	if headingLine >= m.ScrollOffset && headingLine < m.ScrollOffset+contentLines {
		return
	}
	if headingLine < m.ScrollOffset {
		m.ScrollOffset = headingLine
	} else {
		m.ScrollOffset = headingLine - contentLines + 1
		if m.ScrollOffset < 0 {
			m.ScrollOffset = 0
		}
	}
	clampHandoffModelScroll(m)
}

func moveHandoffDiffCursor(m *Model, delta int) {
	paths := handoffDiffPaths(m.HandoffContent)
	if len(paths) == 0 {
		return
	}
	currentIdx := -1
	if m.HandoffSelectedDiff != "" {
		for i, p := range paths {
			if p == m.HandoffSelectedDiff {
				currentIdx = i
				break
			}
		}
	}
	var nextIdx int
	if currentIdx < 0 {
		if delta >= 0 {
			nextIdx = 0
		} else {
			nextIdx = len(paths) - 1
		}
	} else {
		nextIdx = currentIdx + delta
		if nextIdx < 0 {
			nextIdx = len(paths) - 1
		}
		if nextIdx >= len(paths) {
			nextIdx = 0
		}
	}
	m.HandoffSelectedDiff = paths[nextIdx]
	ensureHandoffDiffVisible(m, m.HandoffSelectedDiff)
}

func toggleHandoffSelectedDiff(m *Model) {
	if m.HandoffSelectedDiff == "" {
		return
	}
	if len(handoffDiffPaths(m.HandoffContent)) == 0 {
		return
	}
	toggleHandoffDiff(m, m.HandoffSelectedDiff)
	m.refreshHandoffBodyCache()
	clampHandoffModelScroll(m)
	ensureHandoffDiffVisible(m, m.HandoffSelectedDiff)
}

func handoffDiffPathAtScreenLine(m Model, screenY int) string {
	paths := handoffDiffPaths(m.HandoffContent)
	if len(paths) == 0 {
		return ""
	}

	lineWidth := previewLineWidth(m.Width)
	header := clampPreviewLine(strings.TrimRight(renderHandoffHeader(m), "\n"), lineWidth)
	headerLines := countLines(header)
	if screenY < headerLines {
		return ""
	}

	bodyLines := handoffBodyLines(m)
	contentLines := handoffPreviewContentLines(m)
	scrollOffset := clampHandoffScrollOffset(m.ScrollOffset, len(bodyLines), contentLines)
	bodyLineIndex := scrollOffset + screenY - headerLines
	if bodyLineIndex < 0 || bodyLineIndex >= len(bodyLines) {
		return ""
	}

	return handoffDiffPathFromRenderedLine(bodyLines[bodyLineIndex], paths)
}

func handoffDiffPathFromRenderedLine(line string, paths []string) string {
	stripped := strings.TrimSpace(xansi.Strip(line))
	if strings.HasPrefix(stripped, "•") {
		stripped = strings.TrimSpace(strings.TrimPrefix(stripped, "•"))
	} else if strings.HasPrefix(stripped, "#### ") {
		stripped = handoffDiffPathFromHeading(stripped)
	} else if strings.HasPrefix(stripped, handoffDiffExpandedMarker+" ") || strings.HasPrefix(stripped, handoffDiffCollapsedMarker+" ") {
		// H4 headings render without a bullet prefix; the disclosure arrow is the marker.
	} else {
		return ""
	}
	stripped = trimHandoffDiffDisclosureMarker(stripped)
	for _, path := range paths {
		if stripped == path {
			return path
		}
	}
	return ""
}

func renderSavePrompt(m Model) string {
	lineWidth := previewLineWidth(m.Width)
	cursorChar := " "
	if m.Palette == nil || m.Palette.CursorVisible {
		cursorChar = CursorStyle.Render(" ")
	}

	contentWidth := lineWidth - SavePromptStyle.GetHorizontalFrameSize()
	if contentWidth < 1 {
		contentWidth = 1
	}
	style := SavePromptStyle.Width(contentWidth)
	content := renderBoundedInputContent(" Save as: ", m.SaveInput, cursorChar, " ", contentWidth)
	return style.Render(content)
}

func renderSearchPrompt(m Model, matchCount int) string {
	lineWidth := previewLineWidth(m.Width)
	contentWidth := lineWidth - SearchPromptStyle.GetHorizontalFrameSize()
	if contentWidth < 1 {
		contentWidth = 1
	}
	style := SearchPromptStyle.Width(contentWidth)

	cursorVisible := m.Palette == nil || m.Palette.CursorVisible

	matchText := ""
	if m.Search.Open && m.Search.Query != "" {
		if matchCount > 0 {
			label := "match"
			if matchCount != 1 {
				label = "matches"
			}
			if idx, ok := selectedSearchMatchIndex(m.Search, matchCount); ok {
				matchText = " " + formatInt(int64(idx+1)) + "/" + formatInt(int64(matchCount)) + " " + label
			} else {
				matchText = " " + formatInt(int64(matchCount)) + " " + label
			}
		} else {
			matchText = " No matches"
		}
	}

	prefix := " Search: "
	query := m.Search.Query
	runes := []rune(query)
	pos := m.Search.CursorPos
	if pos > len(runes) {
		pos = len(runes)
	}

	var display string
	if cursorVisible && pos < len(runes) {
		display = prefix + string(runes[:pos]) +
			CursorStyle.Render(string(runes[pos])) +
			string(runes[pos+1:]) + " "
	} else {
		display = prefix + query
		if cursorVisible {
			display += CursorStyle.Render(" ")
		} else {
			display += " "
		}
	}

	content := truncateInputToWidth(display+matchText, contentWidth)
	return style.Render(content)
}

func selectedSearchMatchIndex(search SearchState, matchCount int) (int, bool) {
	if matchCount <= 0 || search.MatchIndex < 0 || search.MatchIndex >= matchCount {
		return 0, false
	}
	if len(search.Matches) != matchCount {
		return 0, false
	}
	return search.MatchIndex, true
}

func resetSearchSelection(search *SearchState) {
	search.Matches = nil
	search.MatchIndex = -1
}

func clearSearchState(search *SearchState) {
	search.Open = false
	search.Query = ""
	search.CursorPos = 0
	resetSearchSelection(search)
}

func advanceHandoffSearchMatch(m *Model) {
	bodyLines := handoffBodyLines(*m)
	matches := findSearchMatches(m.Search.Query, bodyLines)
	m.Search.Matches = matches
	if len(matches) == 0 {
		m.Search.MatchIndex = -1
		return
	}
	if m.Search.MatchIndex < 0 || m.Search.MatchIndex >= len(matches) {
		m.Search.MatchIndex = 0
	} else {
		m.Search.MatchIndex = (m.Search.MatchIndex + 1) % len(matches)
	}

	match := matches[m.Search.MatchIndex]
	m.ScrollOffset = clampHandoffScrollOffset(match.Line, len(bodyLines), handoffPreviewContentLines(*m))
	m.syncHandoffDiffCursor()
}

func buildVisibleRuneMap(line string) (plainText string, runeIndices []int) {
	runes := []rune(line)
	var plain strings.Builder

	for i := 0; i < len(runes); i++ {
		if runes[i] == '\x1b' && i+1 < len(runes) && runes[i+1] == '[' {
			end := i + 2
			for end < len(runes) && runes[end] >= '@' && runes[end] <= '~' {
				end--
			}
			for end < len(runes) && (runes[end] < '@' || runes[end] > '~') {
				end++
			}
			if end < len(runes) {
				i = end
			}
			continue
		}
		plain.WriteRune(runes[i])
		runeIndices = append(runeIndices, i)
	}

	return plain.String(), runeIndices
}

func findSearchMatches(query string, bodyLines []string) []SearchMatch {
	if query == "" {
		return nil
	}
	queryLowerRunes := []rune(strings.ToLower(query))

	var matches []SearchMatch
	for lineIdx, line := range bodyLines {
		plainText, _ := buildVisibleRuneMap(line)
		plainLowerRunes := []rune(strings.ToLower(plainText))

		for start := 0; start <= len(plainLowerRunes)-len(queryLowerRunes); start++ {
			match := true
			for j := 0; j < len(queryLowerRunes); j++ {
				if plainLowerRunes[start+j] != queryLowerRunes[j] {
					match = false
					break
				}
			}
			if match {
				matches = append(matches, SearchMatch{
					Line:     lineIdx,
					ColStart: start,
					ColEnd:   start + len(queryLowerRunes),
				})
			}
		}
	}

	return matches
}

func highlightLineWithMatches(line string, matches []SearchMatch) string {
	return highlightLineWithActiveMatch(line, matches, nil)
}

func highlightLineWithActiveMatch(line string, matches []SearchMatch, activeMatch *SearchMatch) string {
	if len(matches) == 0 {
		return line
	}

	sort.Slice(matches, func(i, j int) bool {
		return matches[i].ColStart < matches[j].ColStart
	})

	plainText, _ := buildVisibleRuneMap(line)
	matched := make([]bool, len([]rune(plainText)))
	activeMatched := make([]bool, len(matched))
	for _, m := range matches {
		for v := m.ColStart; v < m.ColEnd && v < len(matched); v++ {
			matched[v] = true
		}
	}
	if activeMatch != nil {
		for v := activeMatch.ColStart; v < activeMatch.ColEnd && v < len(activeMatched); v++ {
			activeMatched[v] = true
		}
	}

	matchOpenCode, matchCloseCode := searchMatchStyleCodes()
	activeOpenCode, activeCloseCode := activeSearchMatchStyleCodes()

	runes := []rune(line)
	var result strings.Builder
	visIdx := 0
	activeStyle := 0

	for i := 0; i < len(runes); i++ {
		if runes[i] == '\x1b' && i+1 < len(runes) && runes[i+1] == '[' {
			result.WriteRune(runes[i])
			i++
			result.WriteRune(runes[i])
			i++
			for i < len(runes) {
				result.WriteRune(runes[i])
				if runes[i] >= '@' && runes[i] <= '~' {
					break
				}
				i++
			}
			continue
		}

		style := 0
		if visIdx < len(activeMatched) && activeMatched[visIdx] {
			style = 2
		} else if visIdx < len(matched) && matched[visIdx] {
			style = 1
		}
		if style != activeStyle {
			if activeStyle == 1 {
				result.WriteString(matchCloseCode)
			} else if activeStyle == 2 {
				result.WriteString(activeCloseCode)
			}
			if style == 1 {
				result.WriteString(matchOpenCode)
			} else if style == 2 {
				result.WriteString(activeOpenCode)
			}
			activeStyle = style
		}

		result.WriteRune(runes[i])
		visIdx++
	}

	if activeStyle == 1 {
		result.WriteString(matchCloseCode)
	} else if activeStyle == 2 {
		result.WriteString(activeCloseCode)
	}

	return result.String()
}

func searchMatchStyleCodes() (string, string) {
	return styleCodes(SearchMatchStyle, "\x1b[1;30;43m")
}

func activeSearchMatchStyleCodes() (string, string) {
	return styleCodes(ActiveSearchMatchStyle, "\x1b[1;30;46m")
}

func styleCodes(style lipgloss.Style, fallbackOpen string) (string, string) {
	const marker = "x"
	rendered := style.Render(marker)
	idx := strings.Index(rendered, marker)
	if idx < 0 || rendered == marker {
		// Lipgloss can no-op colors under non-TTY renderers; keep search matches visible.
		return fallbackOpen, "\x1b[0m"
	}
	return rendered[:idx], rendered[idx+len(marker):]
}

func renderBoundedInputContent(prefix, input, cursor, suffix string, width int) string {
	inputWidth := width - xansi.StringWidth(prefix) - xansi.StringWidth(cursor) - xansi.StringWidth(suffix)
	input = renderInputTail(input, inputWidth)
	return truncateInputToWidth(prefix+input+cursor+suffix, width)
}

func renderInputTail(input string, width int) string {
	if width <= 0 {
		return ""
	}
	if xansi.StringWidth(input) <= width {
		return input
	}

	markerWidth := xansi.StringWidth(inputOverflowMarker)
	if width <= markerWidth {
		return truncateInputToWidth(inputOverflowMarker, width)
	}
	return inputOverflowMarker + truncateInputSuffixToWidth(input, width-markerWidth)
}

func truncateInputSuffixToWidth(input string, width int) string {
	if width <= 0 {
		return ""
	}
	if xansi.StringWidth(input) <= width {
		return input
	}

	runes := []rune(input)
	start := len(runes)
	used := 0
	for i := len(runes) - 1; i >= 0; i-- {
		runeWidth := xansi.StringWidth(string(runes[i]))
		if used+runeWidth > width {
			break
		}
		used += runeWidth
		start = i
	}
	return string(runes[start:])
}

func truncateInputToWidth(input string, width int) string {
	if width <= 0 {
		return ""
	}
	if xansi.StringWidth(input) <= width {
		return input
	}
	return xansi.Truncate(input, width, "")
}

func renderCollapsedDiffPrompt(m Model, lineWidth int) string {
	count := countCollapsedDiffs(m)
	if count == 0 {
		return ""
	}
	contentWidth := lineWidth - WarningPromptStyle.GetHorizontalFrameSize()
	if contentWidth < 10 {
		contentWidth = 10
	}
	style := WarningPromptStyle.Width(contentWidth)
	var suffix string
	if count == 1 {
		suffix = "diff"
	} else {
		suffix = "diffs"
	}
	text := fmt.Sprintf(" ⚠ Warning: %d %s context excluded — unrecoverable later. Proceed? [y/n] ", count, suffix)
	return style.Render(text)
}

func handoffGlamourStyle() glamouransi.StyleConfig {
	style := styles.DarkStyleConfig

	style.Document.StylePrimitive.BlockPrefix = ""
	style.Document.StylePrimitive.BlockSuffix = ""
	style.Document.Margin = styleUintPtr(0)

	style.Heading.StylePrimitive.BlockSuffix = "\n"
	style.Heading.StylePrimitive.Bold = styleBoolPtr(true)
	style.Heading.StylePrimitive.Color = styleStringPtr("75")

	style.H1.StylePrimitive.Prefix = "▌ "
	style.H1.StylePrimitive.Suffix = ""
	style.H1.StylePrimitive.Color = styleStringPtr("228")
	style.H1.StylePrimitive.BackgroundColor = nil
	style.H1.StylePrimitive.Bold = styleBoolPtr(true)

	style.H2.StylePrimitive.Prefix = "▌ "
	style.H2.StylePrimitive.Color = styleStringPtr("81")
	style.H2.StylePrimitive.Bold = styleBoolPtr(true)

	style.H3.StylePrimitive.Prefix = "┃ "
	style.H3.StylePrimitive.Color = styleStringPtr("116")
	style.H3.StylePrimitive.Bold = styleBoolPtr(true)

	style.H4.StylePrimitive.Prefix = ""
	style.H4.StylePrimitive.Color = styleStringPtr("244")
	style.H4.StylePrimitive.Bold = styleBoolPtr(true)

	style.CodeBlock.StyleBlock.StylePrimitive.BlockPrefix = "\n╭─ code ─\n"
	style.CodeBlock.StyleBlock.StylePrimitive.BlockSuffix = "\n╰────────\n"
	style.CodeBlock.StyleBlock.StylePrimitive.BackgroundColor = styleStringPtr("#1F2430")
	style.CodeBlock.StyleBlock.StylePrimitive.Color = styleStringPtr("#D7D7D7")
	style.CodeBlock.StyleBlock.Margin = styleUintPtr(0)
	if style.CodeBlock.Chroma != nil {
		style.CodeBlock.Chroma.Background.BackgroundColor = styleStringPtr("#1F2430")
		style.CodeBlock.Chroma.Text.BackgroundColor = styleStringPtr("#1F2430")
		style.CodeBlock.Chroma.GenericDeleted.BackgroundColor = styleStringPtr("#1F2430")
		style.CodeBlock.Chroma.GenericInserted.BackgroundColor = styleStringPtr("#1F2430")
	}

	return style
}

func styleStringPtr(s string) *string { return &s }
func styleBoolPtr(b bool) *bool       { return &b }
func styleUintPtr(u uint) *uint       { return &u }

func countCollapsedDiffs(m Model) int {
	count := 0
	for _, collapsed := range m.HandoffCollapsedDiffs {
		if collapsed {
			count++
		}
	}
	return count
}

func handleHandoffKey(m *Model, key string) (tea.Model, tea.Cmd) {
	if m.CollapsedDiffConfirmOpen {
		switch key {
		case "y":
			m.CollapsedDiffConfirmOpen = false
			action := m.CollapsedDiffConfirmAction
			m.CollapsedDiffConfirmAction = ""
			if action == "copy" {
				return m.handleCopyToClipboard()
			}
			if action == "save" {
				return openHandoffSavePrompt(m)
			}
			return *m, nil

		case "n", "esc":
			m.CollapsedDiffConfirmOpen = false
			m.CollapsedDiffConfirmAction = ""
			return *m, nil

		default:
			return *m, nil
		}
	}

	if m.SavePromptOpen {
		switch key {
		case "esc":
			m.SavePromptOpen = false
			m.SaveInput = ""
			return *m, nil

		case "enter":
			return m.handleSaveToFile()

		case "backspace":
			if len(m.SaveInput) > 0 {
				m.SaveInput = m.SaveInput[:len(m.SaveInput)-1]
			}
			return *m, nil

		default:
			if len(key) == 1 {
				m.SaveInput += key
			}
			return *m, nil
		}
	}

	if m.Search.Open {
		switch key {
		case "esc":
			clearSearchState(&m.Search)
			return *m, nil

		case "left":
			if m.Search.CursorPos > 0 {
				m.Search.CursorPos--
			}
			return *m, nil

		case "right":
			runes := []rune(m.Search.Query)
			if m.Search.CursorPos < len(runes) {
				m.Search.CursorPos++
			}
			return *m, nil

		case "home":
			m.Search.CursorPos = 0
			return *m, nil

		case "end":
			m.Search.CursorPos = len([]rune(m.Search.Query))
			return *m, nil

		case "backspace":
			runes := []rune(m.Search.Query)
			if len(runes) > 0 && m.Search.CursorPos > 0 {
				m.Search.Query = string(runes[:m.Search.CursorPos-1]) + string(runes[m.Search.CursorPos:])
				m.Search.CursorPos--
				resetSearchSelection(&m.Search)
			}
			return *m, nil

		case "enter":
			advanceHandoffSearchMatch(m)
			return *m, nil

		default:
			if len(key) == 1 {
				runes := []rune(m.Search.Query)
				pos := m.Search.CursorPos
				if pos > len(runes) {
					pos = len(runes)
				}
				m.Search.Query = string(runes[:pos]) + key + string(runes[pos:])
				m.Search.CursorPos = pos + 1
				resetSearchSelection(&m.Search)
				return *m, nil
			}
		}
	}

	switch key {
	case "?":
		m.ShowHelp = true
		return *m, nil

	case "]":
		moveHandoffDiffCursor(m, 1)
		m.refreshHandoffBodyCache()
		return *m, nil

	case "[":
		moveHandoffDiffCursor(m, -1)
		m.refreshHandoffBodyCache()
		return *m, nil

	case "enter":
		if m.Search.Open || m.SavePromptOpen || m.CollapsedDiffConfirmOpen {
			return *m, nil
		}
		if m.HandoffSelectedDiff == "" {
			return *m, nil
		}
		toggleHandoffSelectedDiff(m)
		return *m, nil

	case "/":
		if !m.CollapsedDiffConfirmOpen && !m.SavePromptOpen {
			m.Search.Open = true
			m.Search.Query = ""
			m.Search.CursorPos = 0
			resetSearchSelection(&m.Search)
			if m.Palette != nil {
				m.Palette.CursorVisible = true
			}
			return *m, tea.Tick(500*time.Millisecond, func(t time.Time) tea.Msg {
				return CursorTickMsg{}
			})
		}
		return *m, nil

	case "y":
		if countCollapsedDiffs(*m) > 0 {
			m.CollapsedDiffConfirmOpen = true
			m.CollapsedDiffConfirmAction = "copy"
			return *m, nil
		}
		return m.handleCopyToClipboard()

	case "s":
		if countCollapsedDiffs(*m) > 0 {
			m.CollapsedDiffConfirmOpen = true
			m.CollapsedDiffConfirmAction = "save"
			return *m, nil
		}
		return openHandoffSavePrompt(m)

	case "d":
		toggleAllHandoffDiffs(m)
		m.refreshHandoffBodyCache()
		clampHandoffModelScroll(m)
		m.syncHandoffDiffCursor()
		return *m, nil

	case "down":
		updated, cmd := m.handleLineScrollKey(scrollDirectionDown, time.Now())
		if next, ok := updated.(Model); ok {
			next.syncHandoffDiffCursor()
			return next, cmd
		}
		return updated, cmd

	case "up":
		updated, cmd := m.handleLineScrollKey(scrollDirectionUp, time.Now())
		if next, ok := updated.(Model); ok {
			next.syncHandoffDiffCursor()
			return next, cmd
		}
		return updated, cmd

	case "pgdown":
		m.ScrollOffset += handoffPreviewContentLines(*m)
		clampHandoffModelScroll(m)
		m.syncHandoffDiffCursor()
		return *m, nil

	case "pgup":
		m.ScrollOffset -= handoffPreviewContentLines(*m)
		clampHandoffModelScroll(m)
		m.syncHandoffDiffCursor()
		return *m, nil

	case "home":
		m.ScrollOffset = 0
		m.syncHandoffDiffCursor()
		return *m, nil

	case "end":
		m.ScrollOffset = handoffMaxScrollOffset(*m)
		m.syncHandoffDiffCursor()
		return *m, nil

	case "t":
		return m.openTodoView()

	case "q":
		if m.ActiveSession != nil {
			changed := m.setView(ActiveSession)
			m.restoreActiveSessionScroll()
			return *m, clearScreenIfChanged(changed)
		} else {
			changed := m.setView(SessionList)
			return *m, clearScreenIfChanged(changed)
		}
	}

	return *m, nil
}

func openHandoffSavePrompt(m *Model) (tea.Model, tea.Cmd) {
	m.SavePromptOpen = true
	m.SaveInput = handoffDefaultFilename(*m)
	if m.Palette != nil {
		m.Palette.CursorVisible = true
	}
	return *m, tea.Tick(500*time.Millisecond, func(t time.Time) tea.Msg {
		return CursorTickMsg{}
	})
}

func handoffDefaultFilename(m Model) string {
	if m.ActiveSession != nil && strings.TrimSpace(m.ActiveSession.ID) != "" {
		return m.ActiveSession.ID
	}
	return "handoff"
}

func clampHandoffModelScroll(m *Model) {
	contentLines := handoffPreviewContentLines(*m)
	bodyLines := len(handoffBodyLines(*m))
	m.ScrollOffset = clampHandoffScrollOffset(m.ScrollOffset, bodyLines, contentLines)
}

func handoffPreviewContentLines(m Model) int {
	lineWidth := previewLineWidth(m.Width)
	header := clampPreviewLine(strings.TrimRight(renderHandoffHeader(m), "\n"), lineWidth)
	footer := clampPreviewLine(renderFooter(m), lineWidth)
	prompt := ""
	if m.SavePromptOpen {
		prompt = renderSavePrompt(m)
	}
	if m.CollapsedDiffConfirmOpen {
		prompt = renderCollapsedDiffPrompt(m, lineWidth)
	}
	searchPrompt := ""
	if m.Search.Open {
		searchPrompt = renderSearchPrompt(m, 0)
	}
	reservedLines := countLines(header) + countLines(footer) + len(renderHandoffMessages(m, lineWidth))
	if prompt != "" {
		reservedLines += countLines(prompt)
	}
	if searchPrompt != "" {
		reservedLines += countLines(searchPrompt)
	}
	contentLines := previewViewportHeight(m.Height) - reservedLines
	if contentLines < 1 {
		return 1
	}
	return contentLines
}

func handoffMaxScrollOffset(m Model) int {
	contentLines := handoffPreviewContentLines(m)
	bodyLines := len(handoffBodyLines(m))
	maxOffset := bodyLines - contentLines
	if maxOffset < 0 {
		return 0
	}
	return maxOffset
}

func (m Model) handleCopyToClipboard() (tea.Model, tea.Cmd) {
	return m, func() tea.Msg {
		err := clipboard.WriteAll(handoffMarkdownForSave(m))
		return HandoffCopiedMsg{Error: err}
	}
}

func (m Model) handleSaveToFile() (tea.Model, tea.Cmd) {
	name := strings.TrimSpace(m.SaveInput)
	if name == "" {
		m.ErrorMessage = "Filename cannot be empty"
		m.SavePromptOpen = false
		m.SaveInput = ""
		return m, nil
	}

	if !isValidFilename(name) {
		m.ErrorMessage = "Invalid filename: path separators or '..' not allowed"
		m.SavePromptOpen = false
		m.SaveInput = ""
		return m, nil
	}

	root := m.Root
	savePath := filepath.Join(root, ".devlog", "handoffs", name+".md")

	if _, err := os.Stat(savePath); err == nil {
		m.ErrorMessage = "Handoff file already exists: " + name + ".md"
		m.SavePromptOpen = false
		m.SaveInput = ""
		return m, nil
	}

	content := handoffMarkdownForSave(m)

	return m, func() tea.Msg {
		dir := filepath.Dir(savePath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return HandoffSavedMsg{Path: savePath, Error: err}
		}
		f, err := os.OpenFile(savePath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
		if err != nil {
			if os.IsExist(err) {
				return HandoffSavedMsg{Path: savePath, Error: fmt.Errorf("handoff file already exists: %s.md", name)}
			}
			return HandoffSavedMsg{Path: savePath, Error: err}
		}
		if _, err := f.WriteString(content); err != nil {
			f.Close()
			return HandoffSavedMsg{Path: savePath, Error: err}
		}
		if err := f.Close(); err != nil {
			return HandoffSavedMsg{Path: savePath, Error: err}
		}
		return HandoffSavedMsg{Path: savePath}
	}
}

func isValidFilename(name string) bool {
	if strings.Contains(name, "..") {
		return false
	}
	if strings.ContainsAny(name, `/\\`) {
		return false
	}
	return true
}

func handleHandoffMouse(m *Model, msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	if m.Palette != nil && m.Palette.Open {
		cmd, _ := m.Palette.Update(msg)
		return *m, cmd
	}

	action := ParseMouseEvent(msg)

	switch action {
	case MouseScrollUp:
		if m.ScrollOffset > 0 {
			m.ScrollOffset--
		}
		clampHandoffModelScroll(m)
		m.syncHandoffDiffCursor()
		return *m, nil

	case MouseScrollDown:
		m.ScrollOffset++
		clampHandoffModelScroll(m)
		m.syncHandoffDiffCursor()
		return *m, nil

	case MouseClick:
		if msg.Action != tea.MouseActionPress || msg.Button != tea.MouseButtonLeft {
			return *m, nil
		}
		by := msg.Y
		if by == 0 {
			bx := msg.X
			copyStart, copyEnd, saveStart, saveEnd := handoffButtonBounds(m.Width)
			if bx >= copyStart && bx < copyEnd {
				return m.handleCopyToClipboard()
			}
			if bx >= saveStart && bx < saveEnd {
				return openHandoffSavePrompt(m)
			}
		}
		if path := handoffDiffPathAtScreenLine(*m, by); path != "" {
			toggleHandoffDiff(m, path)
			m.HandoffSelectedDiff = path
			m.refreshHandoffBodyCache()
			clampHandoffModelScroll(m)
			return *m, nil
		}
	}

	return *m, nil
}

func handoffButtonBounds(width int) (copyStart, copyEnd, saveStart, saveEnd int) {
	if !handoffButtonsVisible(width) {
		return -1, -1, -1, -1
	}

	leftPadding := previewLeftPadding(width)
	width = previewLineWidth(width)
	buttonsText := handoffCopyButton + " " + handoffSaveButton
	copyStart = leftPadding + width - len(buttonsText) - 2
	if copyStart < 0 {
		copyStart = 0
	}
	copyEnd = copyStart + len(handoffCopyButton)
	saveStart = copyEnd + 1
	saveEnd = saveStart + len(handoffSaveButton)
	return copyStart, copyEnd, saveStart, saveEnd
}

func handoffButtonsVisible(width int) bool {
	buttonsText := handoffCopyButton + " " + handoffSaveButton
	return previewLineWidth(width) >= len("Handoff Preview")+len(buttonsText)+2
}

func formatHandoffConfirmation(msg string) string {
	return fmt.Sprintf(" %s", msg)
}
