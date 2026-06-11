package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/atotto/clipboard"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	glamouransi "github.com/charmbracelet/glamour/ansi"
	"github.com/charmbracelet/glamour/styles"
	xansi "github.com/charmbracelet/x/ansi"
)

const (
	handoffCopyButton         = "[y Copy]"
	handoffSaveButton         = "[s Save]"
	maxHandoffPreviewWidth    = 72
	defaultHandoffPreviewRows = 18
	terminalSafetyCols        = 4
	terminalSafetyRows        = 1
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
		prompt = clampPreviewLine(renderSavePrompt(m), lineWidth)
	}
	messages := renderHandoffMessages(m, lineWidth)

	height := previewViewportHeight(m.Height)
	reservedLines := countLines(header) + countLines(footer)
	if prompt != "" {
		reservedLines += countLines(prompt)
	}
	reservedLines += len(messages)

	contentLines := height - reservedLines
	if contentLines < 1 {
		contentLines = 1
	}

	rendered := renderHandoffBody(m)
	bodyLines := clampPreviewLines(splitRenderedLines(rendered), lineWidth)

	scrollOffset := clampHandoffScrollOffset(m.ScrollOffset, len(bodyLines), contentLines)

	end := scrollOffset + contentLines
	if end > len(bodyLines) {
		end = len(bodyLines)
	}
	visible := bodyLines[scrollOffset:end]

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

	previewMarkdown := prepareHandoffPreviewMarkdown(m.HandoffContent)

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

	return rendered
}

func prepareHandoffPreviewMarkdown(content string) string {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	lines := strings.Split(content, "\n")
	var out []string

	for i := 0; i < len(lines); i++ {
		line := lines[i]
		if strings.TrimSpace(line) != "```diff" {
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

func renderSavePrompt(m Model) string {
	input := m.SaveInput
	cursorVisible := true
	if m.Palette != nil {
		cursorVisible = m.Palette.CursorVisible
	}
	if cursorVisible {
		input += CursorStyle.Render("|")
	}
	return SavePromptStyle.Render(" Save as: " + input + " ")
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

	style.H4.StylePrimitive.Prefix = "• "
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

func handleHandoffKey(m *Model, key string) (tea.Model, tea.Cmd) {
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

	switch key {
	case "y":
		return m.handleCopyToClipboard()

	case "s":
		return openHandoffSavePrompt(m)

	case "j", "down":
		m.ScrollOffset++
		clampHandoffModelScroll(m)
		return *m, nil

	case "k", "up":
		if m.ScrollOffset > 0 {
			m.ScrollOffset--
		}
		clampHandoffModelScroll(m)
		return *m, nil

	case "q":
		if m.ActiveSession != nil {
			m.CurrentView = ActiveSession
		} else {
			m.CurrentView = SessionList
		}
		return *m, nil
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
	bodyLines := len(clampPreviewLines(splitRenderedLines(renderHandoffBody(*m)), previewLineWidth(m.Width)))
	m.ScrollOffset = clampHandoffScrollOffset(m.ScrollOffset, bodyLines, contentLines)
}

func handoffPreviewContentLines(m Model) int {
	lineWidth := previewLineWidth(m.Width)
	header := clampPreviewLine(strings.TrimRight(renderHandoffHeader(m), "\n"), lineWidth)
	footer := clampPreviewLine(renderFooter(m), lineWidth)
	prompt := ""
	if m.SavePromptOpen {
		prompt = clampPreviewLine(renderSavePrompt(m), lineWidth)
	}
	reservedLines := countLines(header) + countLines(footer) + len(renderHandoffMessages(m, lineWidth))
	if prompt != "" {
		reservedLines += countLines(prompt)
	}
	contentLines := previewViewportHeight(m.Height) - reservedLines
	if contentLines < 1 {
		return 1
	}
	return contentLines
}

func (m Model) handleCopyToClipboard() (tea.Model, tea.Cmd) {
	return m, func() tea.Msg {
		err := clipboard.WriteAll(m.HandoffContent)
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
		if _, err := f.WriteString(m.HandoffContent); err != nil {
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
	action := ParseMouseEvent(msg)

	switch action {
	case MouseScrollUp:
		if m.ScrollOffset > 0 {
			m.ScrollOffset--
		}
		clampHandoffModelScroll(m)
		return *m, nil

	case MouseScrollDown:
		m.ScrollOffset++
		clampHandoffModelScroll(m)
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
