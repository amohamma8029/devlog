package tui

import (
	"strings"
	"time"

	internalgit "github.com/amo/devlog/internal/git"
	"github.com/amo/devlog/internal/handoff"
	"github.com/amo/devlog/internal/session"
	"github.com/amo/devlog/internal/store"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type View int

const (
	SessionList View = iota
	ActiveSession
	HandoffPreview
)

const (
	scrollDirectionUp     = -1
	scrollDirectionDown   = 1
	lineScrollMinInterval = 16 * time.Millisecond
)

type Model struct {
	CurrentView               View
	ActiveSession             *store.SessionRecord
	Events                    []store.SessionEvent
	Palette                   *CommandPalette
	Store                     *store.Store
	SessionList               SessionListModel
	Root                      string
	Width                     int
	Height                    int
	ScrollOffset              int
	activeSessionScrollOffset int
	ShowHelp                  bool
	ErrorMessage              string
	NoSessionMsg              string
	HandoffContent            string
	HandoffCollapsedDiffs     map[string]bool
	Title                     string
	SavePromptOpen            bool
	SaveInput                 string
	OpenPromptOpen            bool
	OpenInput                 string
	HandoffMsg                string
	scrollDirection           int
	lastLineScroll            time.Time
	activeTimelineLines       []string
	activeTimelineWidth       int
	handoffBodyLines          []string
	handoffBodyLineWidth      int
}

func NewModel(s *store.Store, root string) Model {
	p := NewCommandPalette()
	return Model{
		CurrentView: SessionList,
		Palette:     &p,
		Store:       s,
		SessionList: NewSessionListModel(s, root, 80, 24),
		Root:        root,
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		func() tea.Msg {
			active, err := session.FindActiveSession(m.Store)
			if err != nil {
				return ActiveSessionLoadedMsg{}
			}

			body, err := m.Store.ReadSessionBody(active.ID)
			if err != nil {
				return ActiveSessionLoadedMsg{Session: active}
			}

			events := store.ParseSessionEvents(body)
			title := extractStartMessage(events)

			return ActiveSessionLoadedMsg{Session: active, Events: events, Title: title}
		},
		m.SessionList.Init(),
	)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.Width = msg.Width
		m.Height = msg.Height
		sl, _ := m.SessionList.Update(msg)
		m.SessionList = sl.(SessionListModel)
		m.refreshScrollBodyCaches()
		return m, nil

	case ActiveSessionLoadedMsg:
		m.ActiveSession = msg.Session
		m.Events = msg.Events
		m.Title = msg.Title
		m.ScrollOffset = 0
		m.OpenPromptOpen = false
		m.OpenInput = ""
		m.refreshActiveTimelineCache()
		changed := m.setView(ActiveSession)
		return m, clearScreenIfChanged(changed)

	case NavigationMsg:
		m.setView(msg.Target)
		m.ActiveSession = msg.Session
		if msg.Session == nil {
			return m, func() tea.Msg { return ActiveSessionLoadedMsg{} }
		}

		selected := msg.Session
		return m, func() tea.Msg {
			body, err := m.Store.ReadSessionBody(selected.ID)
			if err != nil {
				return ActiveSessionLoadedMsg{Session: selected}
			}

			events := store.ParseSessionEvents(body)
			title := extractStartMessage(events)
			return ActiveSessionLoadedMsg{Session: selected, Events: events, Title: title}
		}

	case CommandExecutedMsg:
		return m.handleCommand(msg)

	case CommandErrorMsg:
		m.ErrorMessage = msg.Error.Error()
		return m, nil

	case SessionsLoadedMsg:
		sl, cmd := m.SessionList.Update(msg)
		m.SessionList = sl.(SessionListModel)
		return m, cmd

	case HandoffGeneratedMsg:
		if msg.Error != nil {
			m.ErrorMessage = msg.Error.Error()
			return m, nil
		} else {
			m.activeSessionScrollOffset = m.ScrollOffset
			m.HandoffContent = msg.Content
			m.HandoffCollapsedDiffs = nil
			m.ScrollOffset = 0
			m.refreshHandoffBodyCache()
			m.setView(HandoffPreview)
		}
		return m, tea.ClearScreen

	case HandoffSavedMsg:
		m.SavePromptOpen = false
		m.SaveInput = ""
		if msg.Error != nil {
			m.ErrorMessage = msg.Error.Error()
		} else {
			m.HandoffMsg = "Saved to " + msg.Path
		}
		return m, nil

	case HandoffCopiedMsg:
		if msg.Error != nil {
			m.ErrorMessage = msg.Error.Error()
		} else {
			m.HandoffMsg = "Copied to clipboard"
		}
		return m, nil

	case CursorTickMsg:
		return m.handleCursorTick()

	case tea.MouseMsg:
		return m.handleMouse(msg)

	case tea.KeyMsg:
		if m.Palette != nil && m.Palette.Open {
			return m.handlePaletteKey(msg)
		}

		if m.ShowHelp {
			m.ShowHelp = false
			return m, nil
		}

		if m.ErrorMessage != "" {
			m.ErrorMessage = ""
			return m, nil
		}

		if m.NoSessionMsg != "" {
			m.NoSessionMsg = ""
			return m, nil
		}

		if !m.supportsLineScroll() || !isLineScrollKey(msg.String()) {
			m.stopLineScroll()
		}

		return m.handleViewKey(msg)
	}

	return m, nil
}

func (m Model) handleCursorTick() (tea.Model, tea.Cmd) {
	if m.Palette != nil && m.Palette.Open {
		m.Palette.CursorVisible = !m.Palette.CursorVisible
		return m, tea.Tick(500*time.Millisecond, func(t time.Time) tea.Msg {
			return CursorTickMsg{}
		})
	}
	if m.CurrentView == HandoffPreview && m.SavePromptOpen {
		if m.Palette != nil {
			m.Palette.CursorVisible = !m.Palette.CursorVisible
		}
		return m, tea.Tick(500*time.Millisecond, func(t time.Time) tea.Msg {
			return CursorTickMsg{}
		})
	}
	if m.CurrentView == ActiveSession && m.ActiveSession == nil && m.OpenPromptOpen {
		if m.Palette != nil {
			m.Palette.CursorVisible = !m.Palette.CursorVisible
		}
		return m, tea.Tick(500*time.Millisecond, func(t time.Time) tea.Msg {
			return CursorTickMsg{}
		})
	}
	return m, nil
}

func (m Model) handleLineScrollKey(direction int, now time.Time) (tea.Model, tea.Cmd) {
	if direction != scrollDirectionUp && direction != scrollDirectionDown {
		return m, nil
	}
	if now.IsZero() {
		now = time.Now()
	}

	if m.scrollDirection == direction && !m.lastLineScroll.IsZero() && now.Sub(m.lastLineScroll) < lineScrollMinInterval {
		// Drop buffered repeat events that arrive faster than the UI can use them.
		return m, nil
	}

	m.scrollDirection = direction
	m.lastLineScroll = now
	m.scrollByLines(direction)
	return m, nil
}

func (m *Model) stopLineScroll() {
	m.scrollDirection = 0
	m.lastLineScroll = time.Time{}
}

func (m Model) supportsLineScroll() bool {
	return m.CurrentView == ActiveSession || m.CurrentView == HandoffPreview
}

func isLineScrollKey(key string) bool {
	return key == "up" || key == "down"
}

func (m *Model) scrollByLines(lines int) {
	switch m.CurrentView {
	case ActiveSession:
		m.ScrollOffset += lines
		clampActiveSessionModelScroll(m)
	case HandoffPreview:
		m.ScrollOffset += lines
		clampHandoffModelScroll(m)
	}
}

func (m *Model) refreshScrollBodyCaches() {
	m.refreshActiveTimelineCache()
	m.refreshHandoffBodyCache()
}

func (m *Model) refreshActiveTimelineCache() {
	m.activeTimelineLines = buildActiveSessionTimelineLines(*m)
	m.activeTimelineWidth = m.Width
}

func (m *Model) refreshHandoffBodyCache() {
	if m.HandoffContent == "" {
		m.handoffBodyLines = nil
		m.handoffBodyLineWidth = 0
		return
	}
	lineWidth := previewLineWidth(m.Width)
	m.handoffBodyLines = clampPreviewLines(splitRenderedLines(renderHandoffBody(*m)), lineWidth)
	m.handoffBodyLineWidth = lineWidth
}

func (m Model) handlePaletteKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	cmd, _ := m.Palette.Update(msg)
	if cmd != nil {
		return m, cmd
	}
	return m, nil
}

func (m Model) handleMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	if m.CurrentView == HandoffPreview {
		return handleHandoffMouse(&m, msg)
	}

	action := ParseMouseEvent(msg)

	if m.Palette != nil && m.Palette.Open {
		contentLines := countContentLines(m)
		bottomHeight := bottomSectionHeight(m)
		available := m.Height - bottomHeight
		if available < 0 {
			available = 0
		}
		padding := available - contentLines
		if padding < 0 {
			padding = 0
		}
		menuOffset := contentLines + padding + 2
		m.Palette.SetOffset(menuOffset)
		cmd, _ := m.Palette.Update(msg)
		return m, cmd
	}

	switch action {
	case MouseScrollUp:
		if m.ScrollOffset > 0 {
			m.ScrollOffset--
		}
	case MouseScrollDown:
		m.ScrollOffset++
	}

	return m, nil
}

func (m Model) handleViewKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	if key == "ctrl+c" {
		return m, tea.Quit
	}

	if m.CurrentView == SessionList && m.SessionList.filterMode {
		sl, cmd := m.SessionList.Update(msg)
		m.SessionList = sl.(SessionListModel)
		return m, cmd
	}

	if m.CurrentView == ActiveSession && m.ActiveSession == nil && m.OpenPromptOpen {
		return m.noSessionKeyHandler(key)
	}

	if key == "esc" {
		if m.CurrentView == HandoffPreview && m.SavePromptOpen {
			m.SavePromptOpen = false
			m.SaveInput = ""
			return m, nil
		}
		if m.CurrentView == HandoffPreview {
			if m.ActiveSession != nil {
				changed := m.setView(ActiveSession)
				m.restoreActiveSessionScroll()
				return m, clearScreenIfChanged(changed)
			} else {
				changed := m.setView(SessionList)
				return m, tea.Batch(clearScreenIfChanged(changed), m.SessionList.Init())
			}
		}
		if m.CurrentView == SessionList {
			changed := m.setView(ActiveSession)
			return m, clearScreenIfChanged(changed)
		}
		return m, tea.Quit
	}

	if key == "q" {
		if m.CurrentView == HandoffPreview && m.SavePromptOpen {
			// save prompt open: fall through to handleHandoffKey below
		} else if m.CurrentView == HandoffPreview {
			if m.ActiveSession != nil {
				changed := m.setView(ActiveSession)
				m.restoreActiveSessionScroll()
				return m, clearScreenIfChanged(changed)
			} else {
				m.setView(SessionList)
				return m, m.SessionList.Init()
			}
		} else if m.CurrentView == SessionList {
			changed := m.setView(ActiveSession)
			return m, clearScreenIfChanged(changed)
		} else {
			return m, tea.Quit
		}
	}

	switch m.CurrentView {
	case ActiveSession:
		if m.ActiveSession != nil {
			return m.activeSessionKeyHandler(key)
		}
		return m.noSessionKeyHandler(key)

	case SessionList:
		sl, cmd := m.SessionList.Update(msg)
		m.SessionList = sl.(SessionListModel)
		return m, cmd

	case HandoffPreview:
		return handleHandoffKey(&m, key)
	}

	return m, nil
}

func (m Model) activeSessionKeyHandler(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "/":
		if m.Palette != nil {
			m.Palette.OpenPalette()
			m.Palette.Input = "/"
			return m, tea.Tick(500*time.Millisecond, func(t time.Time) tea.Msg {
				return CursorTickMsg{}
			})
		}
		return m, nil

	case "?":
		m.ShowHelp = true
		return m, nil

	case "down":
		return m.handleLineScrollKey(scrollDirectionDown, time.Now())

	case "up":
		return m.handleLineScrollKey(scrollDirectionUp, time.Now())

	case "pgdown":
		m.ScrollOffset += activeSessionPageSize(m)
		clampActiveSessionModelScroll(&m)
		return m, nil

	case "pgup":
		m.ScrollOffset -= activeSessionPageSize(m)
		clampActiveSessionModelScroll(&m)
		return m, nil

	case "home":
		m.ScrollOffset = 0
		return m, nil

	case "end":
		m.ScrollOffset = activeSessionMaxScrollOffset(m)
		return m, nil
	}

	return m, nil
}

func (m Model) noSessionKeyHandler(key string) (tea.Model, tea.Cmd) {
	if m.OpenPromptOpen {
		switch key {
		case "esc":
			m.OpenPromptOpen = false
			m.OpenInput = ""
			return m, nil

		case "enter":
			return m.handleOpenSessionPrompt()

		case "backspace":
			if len(m.OpenInput) > 0 {
				m.OpenInput = m.OpenInput[:len(m.OpenInput)-1]
			}
			return m, nil

		default:
			if len(key) == 1 {
				m.OpenInput += key
			}
			return m, nil
		}
	}

	switch key {
	case "l":
		m.setView(SessionList)
		return m, m.SessionList.Init()

	case "o":
		return openSessionPrompt(&m)
	}

	return m, nil
}

func openSessionPrompt(m *Model) (tea.Model, tea.Cmd) {
	m.OpenPromptOpen = true
	m.OpenInput = ""
	if m.Palette != nil {
		m.Palette.CursorVisible = true
	}
	return *m, tea.Tick(500*time.Millisecond, func(t time.Time) tea.Msg {
		return CursorTickMsg{}
	})
}

func (m Model) handleOpenSessionPrompt() (tea.Model, tea.Cmd) {
	message := strings.TrimSpace(m.OpenInput)
	m.OpenPromptOpen = false
	m.OpenInput = ""
	if message == "" {
		m.ErrorMessage = "Usage: type a session message"
		return m, nil
	}

	return m, func() tea.Msg {
		branch, err := internalgit.CurrentBranch()
		if err != nil {
			return CommandErrorMsg{Error: err}
		}

		name, email, err := internalgit.AuthorIdentity()
		if err != nil {
			return CommandErrorMsg{Error: err}
		}

		now := time.Now().UTC()
		sess := store.Session{
			ID:      now.Format("2006-01-02T150405Z"),
			Author:  name,
			Email:   email,
			Started: now,
			Branch:  branch,
			Status:  "active",
		}

		if err := session.OpenSession(m.Store, sess, message); err != nil {
			return CommandErrorMsg{Error: err}
		}

		record, err := m.Store.GetSession(sess.ID)
		if err != nil {
			return CommandErrorMsg{Error: err}
		}

		body, err := m.Store.ReadSessionBody(sess.ID)
		if err != nil {
			return CommandErrorMsg{Error: err}
		}

		events := store.ParseSessionEvents(body)
		title := extractStartMessage(events)
		return ActiveSessionLoadedMsg{Session: &record, Events: events, Title: title}
	}
}

func (m Model) handleCommand(msg CommandExecutedMsg) (tea.Model, tea.Cmd) {
	parts := strings.SplitN(strings.TrimSpace(msg.Input), " ", 2)
	cmdStr := parts[0]
	args := ""
	if len(parts) > 1 {
		args = parts[1]
	}

	switch cmdStr {
	case "/note":
		if args == "" {
			m.ErrorMessage = "Usage: /note <text>"
			return m, nil
		}
		if m.ActiveSession == nil {
			m.ErrorMessage = "No session displayed"
			return m, nil
		}
		if m.ActiveSession.Closed {
			m.ErrorMessage = "Cannot add notes to a closed session"
			return m, nil
		}
		sessionID := m.ActiveSession.ID
		return m, func() tea.Msg {
			err := m.Store.AppendEvent(sessionID, "Note", args)
			if err != nil {
				return CommandErrorMsg{Error: err}
			}
			return m.reloadEvents()
		}

	case "/block":
		if args == "" {
			m.ErrorMessage = "Usage: /block <text>"
			return m, nil
		}
		if m.ActiveSession == nil {
			m.ErrorMessage = "No session displayed"
			return m, nil
		}
		if m.ActiveSession.Closed {
			m.ErrorMessage = "Cannot add blockers to a closed session"
			return m, nil
		}
		sessionID := m.ActiveSession.ID
		return m, func() tea.Msg {
			err := m.Store.AppendEvent(sessionID, "Blocker", args)
			if err != nil {
				return CommandErrorMsg{Error: err}
			}
			return m.reloadEvents()
		}

	case "/close":
		if m.ActiveSession == nil {
			m.ErrorMessage = "No session displayed"
			return m, nil
		}
		if m.ActiveSession.Closed {
			m.ErrorMessage = "Session is already closed"
			return m, nil
		}
		sessionID := m.ActiveSession.ID
		return m, tea.Sequence(
			func() tea.Msg {
				err := m.Store.CloseSession(sessionID)
				if err != nil {
					return CommandErrorMsg{Error: err}
				}
				return ActiveSessionLoadedMsg{}
			},
			m.SessionList.Init(),
		)

	case "/handoff":
		if m.ActiveSession == nil {
			m.ErrorMessage = "No active session to generate handoff from"
			return m, nil
		}
		sessionID := m.ActiveSession.ID
		started := m.ActiveSession.Started.UTC()
		return m, func() tea.Msg {
			content, err := m.Store.ReadSessionContent(sessionID)
			if err != nil {
				return CommandErrorMsg{Error: err}
			}
			diff, err := internalgit.DiffSince(started)
			if err != nil {
				return CommandErrorMsg{Error: err}
			}
			handoffText, err := handoff.Generate(content, diff)
			if err != nil {
				return CommandErrorMsg{Error: err}
			}
			return HandoffGeneratedMsg{Content: handoffText}
		}

	case "/list":
		m.setView(SessionList)
		return m, m.SessionList.Init()

	default:
		m.ErrorMessage = "Unknown command: " + cmdStr
		return m, nil
	}
}

func (m Model) reloadEvents() tea.Msg {
	if m.ActiveSession == nil {
		return ActiveSessionLoadedMsg{}
	}
	body, err := m.Store.ReadSessionBody(m.ActiveSession.ID)
	if err != nil {
		return CommandErrorMsg{Error: err}
	}
	events := store.ParseSessionEvents(body)
	title := extractStartMessage(events)
	return ActiveSessionLoadedMsg{Session: m.ActiveSession, Events: events, Title: title}
}

func (m Model) View() string {
	if m.ShowHelp {
		return renderHelpOverlay(m)
	}

	var v string
	switch m.CurrentView {
	case ActiveSession:
		if m.ActiveSession != nil {
			v = renderActiveSession(m)
		} else {
			v = renderNoSession(m)
		}
	case SessionList:
		v = renderSessionList(m)
	case HandoffPreview:
		v = renderHandoffPreview(m)
	default:
		v = "<devlog TUI>"
	}

	// Force view to fill terminal height so no stale content persists on resize.
	// lipgloss.Place pads with newlines — ANSI-aware, zero flicker.
	v = lipgloss.Place(m.Width, m.Height, lipgloss.Left, lipgloss.Top, v)

	return v
}

func (m *Model) setView(view View) bool {
	changed := m.CurrentView != view
	if changed {
		m.stopLineScroll()
		m.clearTransientMessages()
		if view != HandoffPreview {
			m.SavePromptOpen = false
			m.SaveInput = ""
		}
	}
	m.CurrentView = view
	return changed
}

func (m *Model) clearTransientMessages() {
	m.ErrorMessage = ""
	m.HandoffMsg = ""
	m.NoSessionMsg = ""
}

func clearScreenIfChanged(changed bool) tea.Cmd {
	if changed {
		return tea.ClearScreen
	}
	return nil
}

func (m *Model) restoreActiveSessionScroll() {
	m.ScrollOffset = m.activeSessionScrollOffset
	clampActiveSessionModelScroll(m)
}

func renderSessionList(m Model) string {
	bottom := renderBottomSection(m, false)
	bottomHeight := countLines(bottom)
	contentHeight := m.Height - bottomHeight
	if contentHeight < 1 {
		contentHeight = 1
	}

	sl := m.SessionList
	sl.width = m.Width
	sl.height = contentHeight
	sl.cw = calcColumnWidths(m.Width)

	content := lipgloss.Place(m.Width, contentHeight, lipgloss.Left, lipgloss.Top, sl.View())
	if bottom == "" {
		return content
	}
	return content + "\n" + bottom
}

func extractStartMessage(events []store.SessionEvent) string {
	for _, e := range events {
		if e.Type == "Start" {
			return strings.TrimSpace(e.Body)
		}
	}
	return ""
}

func formatDuration(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	d = d.Round(time.Minute)
	if d < time.Minute {
		return "less than 1m"
	}

	days := d / (24 * time.Hour)
	d -= days * 24 * time.Hour
	hours := d / time.Hour
	d -= hours * time.Hour
	minutes := d / time.Minute

	var parts []string
	if days > 0 {
		parts = append(parts, formatInt(int64(days))+"d")
	}
	if hours > 0 {
		parts = append(parts, formatInt(int64(hours))+"h")
	}
	if days == 0 && minutes > 0 {
		parts = append(parts, formatInt(int64(minutes))+"m")
	}
	if len(parts) == 0 {
		return "less than 1m"
	}
	return strings.Join(parts, " ")
}

func countContentLines(m Model) int {
	h := countLines(renderHeader(m))
	if len(m.Events) > 1 {
		tl := renderTimeline(m, 0)
		if tl != "" {
			h += 1 + countLines(tl)
		}
	}
	return h
}

func formatInt(n int64) string {
	v := n
	if v < 0 {
		v = -v
	}
	var buf [20]byte
	i := len(buf)
	if v == 0 {
		return "0"
	}
	for v > 0 {
		i--
		buf[i] = byte(v%10) + '0'
		v /= 10
	}
	if n < 0 {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
