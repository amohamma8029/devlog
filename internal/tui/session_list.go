package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	internalgit "github.com/amo/devlog/internal/git"
	"github.com/amo/devlog/internal/handoff"
	"github.com/amo/devlog/internal/session"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/amo/devlog/internal/store"
)

type SessionsLoadedMsg struct {
	Sessions []store.SessionRecord
	Error    error
}

type SessionListModel struct {
	sessions      []store.SessionRecord
	startMessages map[string]string
	filtered      []int
	cursor        int
	scrollOffset  int
	filterText    string
	filterMode    bool
	loaded        bool
	err           error
	store         *store.Store
	root          string
	width         int
	height        int
}

const (
	colWidthID      = 20
	colWidthBranch  = 20
	colWidthAuthor  = 15
	colWidthStarted = 20
	colWidthStatus  = 8
	colSep          = "  "
)

var (
	headerStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FFFFFF"))
	cursorRowStyle = lipgloss.NewStyle().Reverse(true)
)

func NewSessionListModel(s *store.Store, root string, width, height int) SessionListModel {
	return SessionListModel{
		store:  s,
		root:   root,
		width:  width,
		height: height,
	}
}

func (m SessionListModel) Init() tea.Cmd {
	return func() tea.Msg {
		records, err := m.store.ListSessions()
		return SessionsLoadedMsg{Sessions: records, Error: err}
	}
}

func (m SessionListModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case SessionsLoadedMsg:
		if msg.Error != nil {
			m.err = msg.Error
			return m, nil
		}
		m.sessions = msg.Sessions
		m.filtered = make([]int, len(msg.Sessions))
		for i := range msg.Sessions {
			m.filtered[i] = i
		}
		m.loaded = true
		m.loadStartMessages()
		m.clampCursor()
		m.clampScroll()
		return m, nil

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.clampScroll()
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)

	case tea.MouseMsg:
		return m.handleMouse(msg)
	}

	return m, nil
}

func (m SessionListModel) View() string {
	if m.err != nil {
		return m.err.Error()
	}
	if !m.loaded {
		return "Loading sessions..."
	}
	if len(m.sessions) == 0 {
		return "No sessions found."
	}

	headerHeight := 1
	if m.filterMode {
		headerHeight = 2
	}
	borderOverhead := 2
	available := m.height - headerHeight - borderOverhead
	if available < 1 {
		available = 1
	}
	visible := available
	if len(m.filtered) < visible {
		visible = len(m.filtered)
	}
	if visible < 1 {
		visible = 1
	}

	m.clampScroll()

	end := m.scrollOffset + visible
	if end > len(m.filtered) {
		end = len(m.filtered)
	}

	var b strings.Builder

	b.WriteString(m.renderHeader())
	b.WriteString("\n")

	for i := m.scrollOffset; i < end; i++ {
		idx := m.filtered[i]
		s := m.sessions[idx]
		line := m.renderRow(s)
		if i == m.cursor {
			line = cursorRowStyle.Render(line)
		}
		b.WriteString(line)
		b.WriteString("\n")
	}

	body := b.String()

	if m.filterMode {
		filterBar := m.renderFilterBar()
		body = filterBar + "\n" + body
	}

	return BorderStyle.Render(body)
}

func (m SessionListModel) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.filterMode {
		return m.handleFilterKey(msg)
	}

	switch msg.String() {
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
		m.clampScroll()
		return m, nil

	case "down", "j":
		if m.cursor < len(m.filtered)-1 {
			m.cursor++
		}
		m.clampScroll()
		return m, nil

	case "enter":
		if len(m.filtered) == 0 {
			return m, nil
		}
		idx := m.filtered[m.cursor]
		session := m.sessions[idx]
		return m, func() tea.Msg {
			return NavigationMsg{Target: ActiveSession, Session: &session}
		}

	case "/":
		m.filterMode = true
		m.filterText = ""
		return m, nil

	case "h":
		return m.generateHandoff()
	}

	return m, nil
}

func (m SessionListModel) handleFilterKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.filterMode = false
		m.filterText = ""
		m.applyFilter()
		m.clampCursor()
		m.clampScroll()
		return m, nil

	case "enter":
		m.filterMode = false
		return m, nil

	case "backspace":
		if len(m.filterText) > 0 {
			m.filterText = m.filterText[:len(m.filterText)-1]
		}
		m.applyFilter()
		m.clampCursor()
		m.clampScroll()
		return m, nil

	default:
		if len(msg.Runes) == 1 {
			m.filterText += string(msg.Runes[0])
		}
		m.applyFilter()
		m.clampCursor()
		m.clampScroll()
		return m, nil
	}
}

func (m SessionListModel) handleMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	action := ParseMouseEvent(msg)

	switch action {
	case MouseScrollUp:
		if m.cursor > 0 {
			m.cursor--
		}
		m.clampScroll()
		return m, nil

	case MouseScrollDown:
		if m.cursor < len(m.filtered)-1 {
			m.cursor++
		}
		m.clampScroll()
		return m, nil

	case MouseClick:
		headerOffset := 1
		borderOffset := 1
		if m.filterMode {
			headerOffset = 2
		}
		rowY := msg.Y - headerOffset - borderOffset
		if rowY < 0 || rowY >= len(m.filtered)-m.scrollOffset {
			return m, nil
		}
		newCursor := m.scrollOffset + rowY
		if newCursor < len(m.filtered) {
			m.cursor = newCursor
		}
		return m, nil
	}

	return m, nil
}

func (m SessionListModel) renderHeader() string {
	return headerStyle.Render(fmt.Sprintf(
		"%-*s%s%-*s%s%-*s%s%-*s%s%-*s",
		colWidthID, "ID",
		colSep,
		colWidthBranch, "BRANCH",
		colSep,
		colWidthAuthor, "AUTHOR",
		colSep,
		colWidthStarted, "STARTED",
		colSep,
		colWidthStatus, "STATUS",
	))
}

func (m SessionListModel) renderRow(s store.SessionRecord) string {
	status := "active"
	style := ActiveStyle
	if s.Closed {
		status = "closed"
		style = InactiveStyle
	}

	return fmt.Sprintf(
		"%-*s%s%-*s%s%-*s%s%-*s%s%-*s",
		colWidthID, s.ID,
		colSep,
		colWidthBranch, truncateCell(s.Branch, colWidthBranch),
		colSep,
		colWidthAuthor, truncateCell(s.Author, colWidthAuthor),
		colSep,
		colWidthStarted, s.Started.Format("2006-01-02T15:04:05Z"),
		colSep,
		colWidthStatus, style.Render(status),
	)
}

func (m SessionListModel) renderFilterBar() string {
	var b strings.Builder
	b.WriteString("Filter: ")
	b.WriteString(m.filterText)
	if len(m.filtered) > 0 || m.filterText == "" {
		b.WriteString(fmt.Sprintf(" (%d matches)", len(m.filtered)))
	} else {
		b.WriteString(" (no matches)")
	}
	return b.String()
}

func truncateCell(s string, width int) string {
	if len(s) <= width {
		return s
	}
	return s[:width-1] + "\u2026"
}

func (m *SessionListModel) applyFilter() {
	lowerFilter := strings.ToLower(m.filterText)
	if lowerFilter == "" {
		m.filtered = make([]int, len(m.sessions))
		for i := range m.sessions {
			m.filtered[i] = i
		}
		return
	}

	m.filtered = nil
	for i, s := range m.sessions {
		idMatch := strings.HasPrefix(strings.ToLower(s.ID), lowerFilter)
		msgMatch := strings.Contains(strings.ToLower(m.startMessages[s.ID]), lowerFilter)
		if idMatch || msgMatch {
			m.filtered = append(m.filtered, i)
		}
	}
}

func (m *SessionListModel) clampCursor() {
	if len(m.filtered) == 0 {
		m.cursor = 0
		return
	}
	if m.cursor >= len(m.filtered) {
		m.cursor = len(m.filtered) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
}

func (m *SessionListModel) clampScroll() {
	headerHeight := 1
	if m.filterMode {
		headerHeight = 2
	}
	borderOverhead := 2
	available := m.height - headerHeight - borderOverhead
	if available < 1 {
		available = 1
	}

	if m.cursor < m.scrollOffset {
		m.scrollOffset = m.cursor
	}
	if m.cursor >= m.scrollOffset+available {
		m.scrollOffset = m.cursor - available + 1
	}
	if m.scrollOffset < 0 {
		m.scrollOffset = 0
	}
	maxScroll := len(m.filtered) - available
	if maxScroll < 0 {
		maxScroll = 0
	}
	if m.scrollOffset > maxScroll {
		m.scrollOffset = maxScroll
	}
}

func (m *SessionListModel) loadStartMessages() {
	m.startMessages = make(map[string]string, len(m.sessions))
	for _, s := range m.sessions {
		msg, err := readSessionStartMessage(s.ID, m.root)
		if err != nil {
			m.startMessages[s.ID] = ""
			continue
		}
		m.startMessages[s.ID] = msg
	}
}

func readSessionStartMessage(sessionID, root string) (string, error) {
	path := filepath.Join(root, ".devlog", "sessions", sessionID+".md")
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}

	content := strings.ReplaceAll(string(data), "\r\n", "\n")
	const delim = "---\n"
	if !strings.HasPrefix(content, delim) {
		return "", fmt.Errorf("missing opening front-matter delimiter")
	}

	parts := strings.SplitN(content[len(delim):], delim, 2)
	if len(parts) < 2 {
		return "", fmt.Errorf("missing closing front-matter delimiter")
	}

	body := parts[1]
	lines := strings.Split(body, "\n")
	for i, line := range lines {
		if line == "## Start" && i+1 < len(lines) {
			for j := i + 1; j < len(lines); j++ {
				trimmed := strings.TrimSpace(lines[j])
				if trimmed == "" {
					continue
				}
				return trimmed, nil
			}
		}
	}

	return "", nil
}

func (m SessionListModel) generateHandoff() (tea.Model, tea.Cmd) {
	return m, func() tea.Msg {
		active, err := session.FindActiveSession(m.store)
		if err != nil {
			return HandoffGeneratedMsg{Error: fmt.Errorf("No active session to generate handoff from")}
		}
		sessionID := active.ID
		started := active.Started.UTC()
		content, err := m.store.ReadSessionContent(sessionID)
		if err != nil {
			return HandoffGeneratedMsg{Error: err}
		}
		diff, err := internalgit.DiffSince(started)
		if err != nil {
			return HandoffGeneratedMsg{Error: err}
		}
		handoffText, err := handoff.Generate(content, diff)
		if err != nil {
			return HandoffGeneratedMsg{Error: err}
		}
		return HandoffGeneratedMsg{Content: handoffText}
	}
}

