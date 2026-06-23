package tui

import (
	"fmt"
	"strings"

	internalconfig "github.com/amo/devlog/internal/config"
	internalgit "github.com/amo/devlog/internal/git"
	"github.com/amo/devlog/internal/handoff"
	"github.com/amo/devlog/internal/session"
	"github.com/amo/devlog/internal/store"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	xansi "github.com/charmbracelet/x/ansi"
)

type SessionsLoadedMsg struct {
	Sessions []store.SessionRecord
	Error    error
}

type columnWidths struct {
	id         int
	branch     int
	author     int
	started    int
	status     int
	showAuthor bool
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
	config        internalconfig.Config
	root          string
	width         int
	height        int
	cw            columnWidths
	displayTime   internalconfig.DisplayTimeFormatter
}

const (
	colSep = "  "
)

const (
	minColID      = 10
	minColBranch  = 8
	minColAuthor  = 6
	minColStarted = 12
	minColStatus  = 6
)

const (
	weightColID      = 3
	weightColBranch  = 3
	weightColAuthor  = 2
	weightColStarted = 3
	weightColStatus  = 1
)

const borderOverhead = 4

var (
	headerStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FFFFFF"))
	cursorRowStyle = lipgloss.NewStyle().Reverse(true)
)

func calcColumnWidths(termWidth int) columnWidths {
	minTotal := minColID + minColBranch + minColAuthor + minColStarted + minColStatus
	totalOverhead := borderOverhead + 4*len(colSep)
	showAuthor := termWidth >= minTotal+totalOverhead

	numCols := 5
	totalWeight := weightColID + weightColBranch + weightColAuthor + weightColStarted + weightColStatus
	if !showAuthor {
		numCols = 4
		totalWeight = weightColID + weightColBranch + weightColStarted + weightColStatus
	}

	sepOverhead := (numCols - 1) * len(colSep)
	available := termWidth - borderOverhead - sepOverhead
	if available < 1 {
		available = 1
	}

	unit := float64(available) / float64(totalWeight)

	cw := columnWidths{showAuthor: showAuthor}
	cw.id = max(minColID, int(float64(weightColID)*unit))
	cw.branch = max(minColBranch, int(float64(weightColBranch)*unit))
	if showAuthor {
		cw.author = max(minColAuthor, int(float64(weightColAuthor)*unit))
	}
	cw.started = max(minColStarted, int(float64(weightColStarted)*unit))
	cw.status = max(minColStatus, int(float64(weightColStatus)*unit))

	sum := cw.id + cw.branch + cw.started + cw.status
	if showAuthor {
		sum += cw.author
	}

	if sum > available {
		ratio := float64(available) / float64(sum)
		cw.id = max(1, int(float64(cw.id)*ratio))
		cw.branch = max(1, int(float64(cw.branch)*ratio))
		if showAuthor {
			cw.author = max(1, int(float64(cw.author)*ratio))
		}
		cw.started = max(1, int(float64(cw.started)*ratio))
		cw.status = max(1, int(float64(cw.status)*ratio))
	} else if diff := available - sum; diff > 0 {
		cw.id += diff
	}

	return cw
}

func NewSessionListModel(s *store.Store, root string, width, height int) SessionListModel {
	return NewSessionListModelWithConfig(s, root, width, height, internalconfig.Default())
}

func NewSessionListModelWithConfig(s *store.Store, root string, width, height int, cfg internalconfig.Config) SessionListModel {
	if width < 1 {
		width = 80
	}
	formatter, err := internalconfig.NewDisplayTimeFormatter(cfg.Display)
	if err != nil {
		formatter = internalconfig.DefaultDisplayTimeFormatter()
	}
	m := SessionListModel{
		store:       s,
		config:      cfg,
		root:        root,
		width:       width,
		height:      height,
		cw:          calcColumnWidths(width),
		displayTime: formatter,
	}
	if err != nil {
		m.err = err
	}
	return m
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
		m.cw = calcColumnWidths(msg.Width)
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

	m.clampScroll()

	end := m.scrollOffset + visible
	if end > len(m.filtered) {
		end = len(m.filtered)
	}

	lines := []string{m.renderHeader()}

	renderedRows := 0
	for i := m.scrollOffset; i < end; i++ {
		idx := m.filtered[i]
		s := m.sessions[idx]
		line := m.renderRow(s)
		if i == m.cursor {
			line = cursorRowStyle.Render(line)
		}
		lines = append(lines, line)
		renderedRows++
	}
	for renderedRows < visible {
		lines = append(lines, "")
		renderedRows++
	}

	body := strings.Join(lines, "\n")

	width := terminalRenderWidth(m.width)
	boxWidth := width - 2
	if boxWidth < 1 {
		boxWidth = 1
	}
	bodyWidth := width - BorderStyle.GetHorizontalFrameSize()
	if bodyWidth < 1 {
		bodyWidth = 1
	}

	if m.filterMode {
		filterBar := m.renderFilterBar(bodyWidth)
		body = filterBar + "\n" + body
	}

	return clampRenderedBlock(BorderStyle.Width(boxWidth).Render(body), width)
}

func (m SessionListModel) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.filterMode {
		return m.handleFilterKey(msg)
	}

	switch msg.String() {
	case "up":
		if m.cursor > 0 {
			m.cursor--
		}
		m.clampScroll()
		return m, nil

	case "down":
		if m.cursor < len(m.filtered)-1 {
			m.cursor++
		}
		m.clampScroll()
		return m, nil

	case "pgup":
		m.cursor -= m.visibleRows()
		m.clampCursor()
		m.clampScroll()
		return m, nil

	case "pgdown":
		m.cursor += m.visibleRows()
		m.clampCursor()
		m.clampScroll()
		return m, nil

	case "home":
		m.cursor = 0
		m.clampCursor()
		m.clampScroll()
		return m, nil

	case "end":
		if len(m.filtered) > 0 {
			m.cursor = len(m.filtered) - 1
		}
		m.clampCursor()
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

func padCell(s string, width int) string {
	sw := xansi.StringWidth(s)
	if sw >= width {
		return s
	}
	return s + strings.Repeat(" ", width-sw)
}

func (m SessionListModel) renderHeader() string {
	w := m.cw
	cells := []string{
		padCell(truncateCell("TITLE", w.id), w.id),
		padCell(truncateCell("BRANCH", w.branch), w.branch),
	}
	if w.showAuthor {
		cells = append(cells, padCell(truncateCell("AUTHOR", w.author), w.author))
	}
	cells = append(cells,
		padCell(truncateCell("STARTED", w.started), w.started),
		padCell(truncateCell("STATUS", w.status), w.status),
	)
	return headerStyle.Render(strings.Join(cells, colSep))
}

func (m SessionListModel) renderRow(s store.SessionRecord) string {
	status := "active"
	style := ActiveStyle
	if s.Closed {
		status = "closed"
		style = InactiveStyle
	}

	w := m.cw
	title := m.startMessages[s.ID]
	if title == "" {
		title = s.ID
	}
	cells := []string{
		padCell(truncateCell(title, w.id), w.id),
		padCell(truncateCell(s.Branch, w.branch), w.branch),
	}
	if w.showAuthor {
		cells = append(cells, padCell(truncateCell(s.Author, w.author), w.author))
	}
	cells = append(cells,
		padCell(truncateCell(m.displayTime.DateTime(s.Started), w.started), w.started),
		padCell(style.Render(truncateCell(status, w.status)), w.status),
	)
	return strings.Join(cells, colSep)
}

func (m SessionListModel) renderFilterBar(width int) string {
	var b strings.Builder
	b.WriteString("Filter: ")
	b.WriteString(m.filterText)
	if len(m.filtered) > 0 || m.filterText == "" {
		b.WriteString(fmt.Sprintf(" (%d matches)", len(m.filtered)))
	} else {
		b.WriteString(" (no matches)")
	}
	return clampRenderedLine(b.String(), width)
}

func truncateCell(s string, width int) string {
	if xansi.StringWidth(s) <= width {
		return s
	}
	ellipsis := "\u2026"
	maxContent := width - xansi.StringWidth(ellipsis)
	if maxContent < 0 {
		maxContent = 0
	}
	return xansi.Truncate(s, maxContent, "") + ellipsis
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

func (m SessionListModel) visibleRows() int {
	headerHeight := 1
	if m.filterMode {
		headerHeight = 2
	}
	borderOverhead := 2
	available := m.height - headerHeight - borderOverhead
	if available < 1 {
		available = 1
	}
	return available
}

func (m *SessionListModel) clampScroll() {
	available := m.visibleRows()

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
		msg, err := m.store.ReadSessionStartMessage(s.ID)
		if err != nil {
			m.startMessages[s.ID] = ""
			continue
		}
		m.startMessages[s.ID] = msg
	}
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
		diff, err := internalgit.DiffSinceWithContext(started, m.config.Handoff.DiffContextLines)
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
