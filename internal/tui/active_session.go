package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/amo/devlog/internal/store"
	"github.com/charmbracelet/lipgloss"
)

func renderActiveSession(m Model) string {
	header := renderHeader(m)
	bottom := renderBottomSection(m, true)

	bottomHeight := countLines(bottom)
	available := m.Height - bottomHeight
	if available < 0 {
		available = 0
	}

	timeline := ""
	if len(m.Events) > 1 {
		timelineHeight := activeSessionTimelineHeight(m)
		timeline = renderTimeline(m, timelineHeight)
	}

	contentTop := header
	if timeline != "" {
		contentTop += "\n" + timeline
	}

	contentLines := countLines(contentTop)
	padding := available - contentLines
	if padding < 0 {
		padding = 0
	}

	var b strings.Builder
	b.WriteString(contentTop)
	b.WriteString(strings.Repeat("\n", padding))

	if bottom != "" {
		b.WriteString("\n")
		b.WriteString(bottom)
	}

	return b.String()
}

func bottomSectionHeight(m Model) int {
	return countLines(renderBottomSection(m, true))
}

func renderBottomSection(m Model, includePalette bool) string {
	var parts []string
	if m.CurrentView == ActiveSession && m.ActiveSession == nil && m.OpenPromptOpen {
		parts = append(parts, renderOpenSessionPrompt(m))
	}
	if includePalette && m.Palette != nil && m.Palette.Open {
		parts = append(parts, m.Palette.View())
	}
	parts = append(parts, renderTransientMessages(m, m.Width)...)
	parts = append(parts, renderFooter(m))
	return strings.Join(parts, "\n")
}

func renderTransientMessages(m Model, width int) []string {
	if width <= 0 {
		width = 80
	}

	var lines []string
	if m.ErrorMessage != "" {
		lines = append(lines, clampPreviewLine(ErrorBannerStyle.Render(" ERROR: "+m.ErrorMessage), width))
	}
	if m.HandoffMsg != "" {
		lines = append(lines, clampPreviewLine(HintStyle.Render(formatHandoffConfirmation(m.HandoffMsg)), width))
	}
	if m.NoSessionMsg != "" {
		lines = append(lines, clampPreviewLine(HintStyle.Render(m.NoSessionMsg), width))
	}
	return lines
}

func renderHeader(m Model) string {
	sess := m.ActiveSession
	if sess == nil {
		return ""
	}

	var b strings.Builder

	if m.Title != "" {
		titleLine := TitleStyle.Render(m.Title) + " " + IDParenStyle.Render("("+sess.ID+")")
		b.WriteString(titleLine)
		b.WriteByte('\n')
		b.WriteByte('\n')
	}

	now := time.Now().UTC()
	dur := formatDuration(now.Sub(sess.Started.UTC()))
	author := formatAuthor(sess.Author, sess.Email)

	if m.Width >= 80 {
		b.WriteString(MetadataStyle.Render(
			fmt.Sprintf("Author: %s  ·  Branch: %s  ·  Started: %s  ·  Duration: %s",
				author, sess.Branch, sess.Started.UTC().Format(time.RFC3339), dur),
		))
	} else {
		b.WriteString(MetadataStyle.Render("Author: " + author))
		b.WriteByte('\n')
		b.WriteString(MetadataStyle.Render("Branch: " + sess.Branch))
		b.WriteByte('\n')
		b.WriteString(MetadataStyle.Render("Started: " + sess.Started.UTC().Format(time.RFC3339)))
		b.WriteByte('\n')
		b.WriteString(MetadataStyle.Render("Duration: " + dur))
	}

	b.WriteByte('\n')
	return BorderStyle.Render(b.String())
}

func renderFooter(m Model) string {
	var text string
	if m.CurrentView == ActiveSession {
		if m.ActiveSession != nil {
			text = "/? help  ·  ↑/↓ line  ·  pgup/pgdn page  ·  home/end jump  ·  q quit"
		} else {
			text = "l: session list  ·  o: open new session  ·  q quit"
		}
	} else if m.CurrentView == HandoffPreview {
		text = "y copy  ·  s save  ·  d diffs  ·  ↑/↓ line  ·  pgup/pgdn page  ·  home/end jump  ·  q back"
	} else {
		text = "q quit"
	}
	return HintStyle.Render(text)
}

func renderNoSession(m Model) string {
	bottom := renderBottomSection(m, true)
	bottomHeight := countLines(bottom)
	availHeight := m.Height - bottomHeight
	if availHeight < 1 {
		availHeight = 1
	}

	msg := "No active session"
	centered := lipgloss.Place(
		m.Width, availHeight,
		lipgloss.Center, lipgloss.Center,
		msg,
	)

	var b strings.Builder
	b.WriteString(centered)

	remaining := availHeight - countLines(centered)
	if remaining < 0 {
		remaining = 0
	}
	b.WriteString(strings.Repeat("\n", remaining))

	if bottom != "" {
		b.WriteString("\n")
		b.WriteString(bottom)
	}

	return b.String()
}

func renderOpenSessionPrompt(m Model) string {
	input := m.OpenInput
	cursorVisible := true
	if m.Palette != nil {
		cursorVisible = m.Palette.CursorVisible
	}
	if cursorVisible {
		input += CursorStyle.Render("|")
	}
	return SavePromptStyle.Render(" Open session: " + input + " ")
}

func renderHelpOverlay(m Model) string {
	content := `Keybindings
===========
/      Open command palette
?      Show this help (dismiss with any key)
↓      Scroll down one line
↑      Scroll up one line
pgdn   Scroll down one page
pgup   Scroll up one page
home   Jump to top
end    Jump to bottom
q      Quit

Session List
============
h      Generate handoff for active session
Enter  Open selected session
/      Filter sessions

Slash Commands
==============
/note <text>    Add a note to the session
/block <text>   Log a blocker
/close          Close the active session
/handoff        Generate handoff summary
/list           Go to session list

Handoff Preview
===============
y      Copy to clipboard
s      Save to file
↓      Scroll down one line
↑      Scroll up one line
pgdn   Scroll down one page
pgup   Scroll up one page
home   Jump to top
end    Jump to bottom
d      Toggle all diffs
click  Toggle file diff
q      Go back

No Active Session
=================
l      View session list
o      Hint to open new session
q      Quit

Note: Use terminal selection (Shift+click) to select text.`

	rendered := HelpOverlayStyle.Render(content)
	return lipgloss.Place(
		m.Width, m.Height,
		lipgloss.Center, lipgloss.Center,
		rendered,
	)
}

func countLines(s string) int {
	if s == "" {
		return 0
	}
	return strings.Count(s, "\n") + 1
}

func formatAuthor(author, email string) string {
	author = strings.TrimSpace(author)
	email = strings.TrimSpace(email)
	if author == "" && email == "" {
		return "(unknown)"
	}
	if author == "" {
		return email
	}
	if email == "" {
		return author
	}
	return fmt.Sprintf("%s <%s>", author, email)
}

func renderTimeline(m Model, maxLines int) string {
	var b strings.Builder

	renderedLines := activeSessionTimelineLines(m)
	if len(renderedLines) == 0 {
		return ""
	}

	startLine := m.ScrollOffset
	if startLine < 0 {
		startLine = 0
	}

	if startLine >= len(renderedLines) {
		startLine = len(renderedLines) - 1
		if startLine < 0 {
			startLine = 0
		}
	}

	endLine := len(renderedLines)
	if maxLines > 0 && startLine+maxLines < endLine {
		endLine = startLine + maxLines
	}

	for i := startLine; i < endLine; i++ {
		b.WriteString(renderedLines[i])
		if i < endLine-1 {
			b.WriteByte('\n')
		}
	}

	return b.String()
}

func activeSessionTimelineHeight(m Model) int {
	header := renderHeader(m)
	bottomHeight := bottomSectionHeight(m)
	available := m.Height - bottomHeight
	if available < 0 {
		available = 0
	}
	timelineHeight := available - countLines(header) - 1
	if timelineHeight < 0 {
		return 0
	}
	return timelineHeight
}

func activeSessionPageSize(m Model) int {
	pageSize := activeSessionTimelineHeight(m)
	if pageSize < 1 {
		return 1
	}
	return pageSize
}

func activeSessionMaxScrollOffset(m Model) int {
	maxOffset := len(activeSessionTimelineLines(m)) - activeSessionPageSize(m)
	if maxOffset < 0 {
		return 0
	}
	return maxOffset
}

func clampActiveSessionModelScroll(m *Model) {
	if m.ScrollOffset < 0 {
		m.ScrollOffset = 0
		return
	}
	maxOffset := activeSessionMaxScrollOffset(*m)
	if m.ScrollOffset > maxOffset {
		m.ScrollOffset = maxOffset
	}
}

func activeSessionTimelineLines(m Model) []string {
	width := m.Width
	if width < 40 {
		width = 40
	}
	contentWidth := width - 2

	nonStartEvents := filterNonStartEvents(m.Events)
	if len(nonStartEvents) == 0 {
		return nil
	}
	return renderEventLines(nonStartEvents, contentWidth)
}

func filterNonStartEvents(events []store.SessionEvent) []store.SessionEvent {
	var result []store.SessionEvent
	for _, e := range events {
		if e.Type != "Start" {
			result = append(result, e)
		}
	}
	return result
}

func renderEventLines(events []store.SessionEvent, maxWidth int) []string {
	var lines []string
	if len(events) == 0 {
		return lines
	}

	eventWidth := maxWidth - 4
	if eventWidth < 20 {
		eventWidth = 20
	}

	timelineChar := "│"

	for _, event := range events {
		eventStyle := PanelStyle
		labelStyle := EventStyle
		connectorStyle := ConnectorStyle.Foreground(lipgloss.Color("#555555"))

		if event.Type == "Blocker" {
			eventStyle = BlockerStyle.Foreground(lipgloss.Color("#FF6600"))
			labelStyle = BlockerLabelStyle
			connectorStyle = ConnectorStyle.Foreground(lipgloss.Color("#FF6600"))
		}

		timeStr := event.Time.UTC().Format("2006-01-02 15:04 UTC")
		if event.Time.IsZero() {
			timeStr = "     "
		}

		headerLine := fmt.Sprintf("%s · %s", event.Type, timeStr)
		header := labelStyle.Render(headerLine)

		bodyLines := splitLines(event.Body, eventWidth-4)
		var paneLines []string
		paneLines = append(paneLines, header)

		if len(bodyLines) == 1 && bodyLines[0] == "" {
		} else {
			for _, bl := range bodyLines {
				if strings.TrimSpace(bl) != "" {
					paneLines = append(paneLines, EventStyle.Render(bl))
				}
			}
		}

		pane := eventStyle.Render(strings.Join(paneLines, "\n"))
		eventLines := strings.Split(pane, "\n")

		for i, el := range eventLines {
			if i == 0 {
				connHead := "┌"
				if len(events) > 0 {
					connHead = "├"
				}
				lines = append(lines, connectorStyle.Render(timelineChar+" "+connHead)+el)
			} else {
				connector := connectorStyle.Render(timelineChar + " │")
				lines = append(lines, connector+el)
			}
		}

		bottomBar := connectorStyle.Render(timelineChar + " └" + strings.Repeat("─", eventWidth) + "┘")
		lines = append(lines, bottomBar)
	}

	return lines
}

func splitLines(text string, maxWidth int) []string {
	if maxWidth < 1 {
		maxWidth = 1
	}

	var result []string
	para := strings.TrimSpace(text)
	if para == "" {
		return []string{""}
	}

	words := strings.Fields(para)
	if len(words) == 0 {
		return []string{""}
	}

	var line string
	for _, word := range words {
		if line == "" {
			line = word
		} else if len(line)+1+len(word) <= maxWidth {
			line += " " + word
		} else {
			result = append(result, line)
			line = word
		}
	}
	if line != "" {
		result = append(result, line)
	}

	if len(result) == 0 {
		result = append(result, "")
	}
	return result
}
