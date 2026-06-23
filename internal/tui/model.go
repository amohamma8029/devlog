package tui

import (
	"strings"
	"time"

	internalconfig "github.com/amo/devlog/internal/config"
	internalgit "github.com/amo/devlog/internal/git"
	"github.com/amo/devlog/internal/handoff"
	"github.com/amo/devlog/internal/session"
	"github.com/amo/devlog/internal/store"
	tea "github.com/charmbracelet/bubbletea"
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
	activeRefreshInterval = time.Second
	sessionListResizeWait = 75 * time.Millisecond
)

type Model struct {
	CurrentView                View
	ActiveSession              *store.SessionRecord
	Events                     []store.SessionEvent
	Palette                    *CommandPalette
	Store                      *store.Store
	Config                     internalconfig.Config
	SessionList                SessionListModel
	Root                       string
	Width                      int
	Height                     int
	ScrollOffset               int
	activeSessionScrollOffset  int
	ShowHelp                   bool
	ErrorMessage               string
	NoSessionMsg               string
	HandoffContent             string
	HandoffCollapsedDiffs      map[string]bool
	Title                      string
	SavePromptOpen             bool
	SaveInput                  string
	Search                     SearchState
	OpenPromptOpen             bool
	OpenInput                  string
	HandoffMsg                 string
	CollapsedDiffConfirmOpen   bool
	CollapsedDiffConfirmAction string
	scrollDirection            int
	lastLineScroll             time.Time
	activeTimelineLines        []string
	activeTimelineWidth        int
	activeEventLineStarts      []int
	SelectedEvent              int
	EditingEvent               int
	DeleteConfirmEvent         int
	handoffBodyLines           []string
	handoffBodyLineWidth       int
	displayTime                internalconfig.DisplayTimeFormatter
	activeSessionMetadata      store.SessionFileMetadata
	activeSessionMetadataKnown bool
	sessionListResizing        bool
	sessionListResizeSeq       int
}

type SearchState struct {
	Open       bool
	Query      string
	CursorPos  int
	Matches    []SearchMatch
	MatchIndex int
}

type SearchMatch struct {
	Line     int
	ColStart int
	ColEnd   int
}

func NewModel(s *store.Store, root string) Model {
	return NewModelWithConfig(s, root, internalconfig.Default())
}

func NewModelWithConfig(s *store.Store, root string, cfg internalconfig.Config) Model {
	p := NewCommandPalette()
	formatter, err := internalconfig.NewDisplayTimeFormatter(cfg.Display)
	if err != nil {
		formatter = internalconfig.DefaultDisplayTimeFormatter()
	}
	m := Model{
		CurrentView:   SessionList,
		Palette:       &p,
		Store:         s,
		Config:        cfg,
		SessionList:   NewSessionListModelWithConfig(s, root, 80, 24, cfg),
		Root:          root,
		displayTime:   formatter,
		SelectedEvent: -1,
		EditingEvent:  -1,
	}
	if err != nil {
		m.ErrorMessage = err.Error()
	}
	return m
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
		activeSessionRefreshTickCmd(),
	)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		blankSessionList := m.shouldBlankSessionListForResize(msg)
		restoreSessionList := m.shouldRestoreSessionListOnResize(msg)
		m.Width = msg.Width
		m.Height = msg.Height
		sl, _ := m.SessionList.Update(msg)
		m.SessionList = sl.(SessionListModel)
		m.refreshScrollBodyCaches()
		if restoreSessionList {
			m.sessionListResizing = false
		}
		if blankSessionList {
			return m, m.startSessionListResizeSettle()
		}
		return m, nil

	case sessionListResizeSettledMsg:
		if msg.Seq == m.sessionListResizeSeq {
			m.sessionListResizing = false
		}
		return m, nil

	case ActiveSessionLoadedMsg:
		m.ActiveSession = msg.Session
		m.Events = msg.Events
		m.Title = msg.Title
		m.ScrollOffset = 0
		m.OpenPromptOpen = false
		m.OpenInput = ""
		m.activeSessionMetadataKnown = false
		m.SelectedEvent = -1
		m.DeleteConfirmEvent = 0
		m.refreshActiveTimelineCache()
		changed := m.setView(ActiveSession)
		return m, clearScreenIfChanged(changed)

	case ActiveSessionRefreshTickMsg:
		return m, m.checkActiveSessionRefreshCmd()

	case ActiveSessionRefreshResultMsg:
		m.applyActiveSessionRefresh(msg)
		return m, activeSessionRefreshTickCmd()

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

	case MultiLineNoteMsg:
		return m.handleMultiLineSubmit(msg)

	case pasteMsg:
		if m.Palette != nil && m.Palette.Open {
			cmd, _ := m.Palette.Update(msg)
			return m, cmd
		}
		return m, nil

	case ClipboardActionMsg:
		if msg.Action == "copy" {
			m.HandoffMsg = "Copied to clipboard"
		} else if msg.Action == "cut" {
			m.HandoffMsg = "Cut to clipboard"
		}
		return m, nil

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
		if m.ShowHelp {
			return m, nil
		}
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
	if m.CurrentView == HandoffPreview && m.Search.Open {
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
	m.activeTimelineLines, m.activeEventLineStarts = buildActiveSessionTimelineLines(*m)
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

func (m Model) shouldBlankSessionListForResize(msg tea.WindowSizeMsg) bool {
	return m.CurrentView == SessionList &&
		m.SessionList.loaded &&
		m.Width > 0 &&
		m.Height > 0 &&
		msg.Width > 0 &&
		msg.Width < m.Width
}

func (m Model) shouldRestoreSessionListOnResize(msg tea.WindowSizeMsg) bool {
	return m.CurrentView == SessionList &&
		m.sessionListResizing &&
		m.Width > 0 &&
		msg.Width > m.Width
}

func (m *Model) startSessionListResizeSettle() tea.Cmd {
	m.sessionListResizing = true
	m.sessionListResizeSeq++
	seq := m.sessionListResizeSeq
	return tea.Tick(sessionListResizeWait, func(t time.Time) tea.Msg {
		return sessionListResizeSettledMsg{Seq: seq}
	})
}

func activeSessionRefreshTickCmd() tea.Cmd {
	return tea.Tick(activeRefreshInterval, func(t time.Time) tea.Msg {
		return ActiveSessionRefreshTickMsg{}
	})
}

func (m Model) checkActiveSessionRefreshCmd() tea.Cmd {
	if m.Store == nil || m.ActiveSession == nil {
		return func() tea.Msg { return ActiveSessionRefreshResultMsg{} }
	}

	sessionID := m.ActiveSession.ID
	known := m.activeSessionMetadataKnown
	previous := m.activeSessionMetadata
	return func() tea.Msg {
		metadata, err := m.Store.ReadSessionFileMetadata(sessionID)
		if err != nil {
			return ActiveSessionRefreshResultMsg{SessionID: sessionID, Error: err}
		}
		if known && metadata.Equal(previous) {
			return ActiveSessionRefreshResultMsg{SessionID: sessionID, Metadata: metadata}
		}

		body, err := m.Store.ReadSessionBody(sessionID)
		if err != nil {
			return ActiveSessionRefreshResultMsg{SessionID: sessionID, Metadata: metadata, Error: err}
		}

		events := store.ParseSessionEvents(body)
		return ActiveSessionRefreshResultMsg{
			SessionID: sessionID,
			Metadata:  metadata,
			Changed:   true,
			Events:    events,
			Title:     extractStartMessage(events),
			Closed:    sessionEventsClosed(events),
		}
	}
}

func (m *Model) applyActiveSessionRefresh(msg ActiveSessionRefreshResultMsg) {
	if msg.SessionID == "" || m.ActiveSession == nil || msg.SessionID != m.ActiveSession.ID {
		return
	}
	if msg.Error != nil {
		m.ErrorMessage = msg.Error.Error()
		return
	}

	m.activeSessionMetadata = msg.Metadata
	m.activeSessionMetadataKnown = true
	if !msg.Changed {
		return
	}

	m.Events = msg.Events
	m.Title = msg.Title
	record := *m.ActiveSession
	record.Closed = msg.Closed
	m.ActiveSession = &record
	if m.Palette != nil {
		m.Palette.SessionClosed = msg.Closed
	}
	m.refreshActiveTimelineCache()
}

func sessionEventsClosed(events []store.SessionEvent) bool {
	for _, event := range events {
		if event.Type == "Stop" {
			return true
		}
	}
	return false
}

func (m Model) handlePaletteKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	wasOpen := m.Palette.Open
	cmd, p := m.Palette.Update(msg)
	m.HandoffMsg = ""
	if wasOpen && !p.Open && cmd == nil {
		m.EditingEvent = -1
	}
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
		if m.CurrentView == ActiveSession && m.DeleteConfirmEvent > 0 {
			m.DeleteConfirmEvent = 0
			return m, nil
		}
		if m.CurrentView == ActiveSession && m.SelectedEvent >= 0 {
			m.SelectedEvent = -1
			return m, nil
		}
		if m.CurrentView == HandoffPreview && m.CollapsedDiffConfirmOpen {
			m.CollapsedDiffConfirmOpen = false
			m.CollapsedDiffConfirmAction = ""
			return m, nil
		}
		if m.CurrentView == HandoffPreview && m.SavePromptOpen {
			m.SavePromptOpen = false
			m.SaveInput = ""
			return m, nil
		}
		if m.CurrentView == HandoffPreview && m.Search.Open {
			clearSearchState(&m.Search)
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
		if key == "?" {
			m.ShowHelp = true
			return m, nil
		}
		sl, cmd := m.SessionList.Update(msg)
		m.SessionList = sl.(SessionListModel)
		return m, cmd

	case HandoffPreview:
		return handleHandoffKey(&m, key)
	}

	return m, nil
}

func (m Model) activeSessionKeyHandler(key string) (tea.Model, tea.Cmd) {
	if m.DeleteConfirmEvent > 0 {
		switch key {
		case "y":
			return m.handleDeleteConfirm()
		case "n", "esc":
			m.DeleteConfirmEvent = 0
			return m, nil
		default:
			return m, nil
		}
	}

	switch key {
	case "/":
		if m.Palette != nil {
			m.Palette.OpenPalette()
			m.Palette.Input = "/"
			m.Palette.InputCursorPos = 1
			m.Palette.SessionClosed = m.ActiveSession == nil || m.ActiveSession.Closed
			return m, tea.Tick(500*time.Millisecond, func(t time.Time) tea.Msg {
				return CursorTickMsg{}
			})
		}
		return m, nil

	case "?":
		m.ShowHelp = true
		return m, nil

	case "tab":
		return m.handleEventNavigation(1)

	case "shift+tab":
		return m.handleEventNavigation(-1)

	case "e", "enter":
		if m.SelectedEvent >= 0 {
			return m.handleEditEvent()
		}
		return m, nil

	case "d":
		if m.SelectedEvent >= 0 {
			return m.handleDeleteEvent()
		}
		return m, nil

	case "down":
		m.SelectedEvent = -1
		return m.handleLineScrollKey(scrollDirectionDown, time.Now())

	case "up":
		m.SelectedEvent = -1
		return m.handleLineScrollKey(scrollDirectionUp, time.Now())

	case "pgdown":
		m.SelectedEvent = -1
		m.ScrollOffset += activeSessionPageSize(m)
		clampActiveSessionModelScroll(&m)
		return m, nil

	case "pgup":
		m.SelectedEvent = -1
		m.ScrollOffset -= activeSessionPageSize(m)
		clampActiveSessionModelScroll(&m)
		return m, nil

	case "home":
		m.SelectedEvent = -1
		m.ScrollOffset = 0
		return m, nil

	case "end":
		m.SelectedEvent = -1
		m.ScrollOffset = activeSessionMaxScrollOffset(m)
		return m, nil
	}

	return m, nil
}

func (m Model) handleEventNavigation(delta int) (tea.Model, tea.Cmd) {
	events := m.Events
	count := 0
	for _, e := range events {
		if e.Type == "Start" || e.IsDeleted {
			continue
		}
		count++
	}
	if count == 0 {
		return m, nil
	}

	newIdx := m.SelectedEvent + delta
	if newIdx < 0 {
		newIdx = count - 1
	}
	if newIdx >= count {
		newIdx = 0
	}
	m.SelectedEvent = newIdx
	m.activeTimelineWidth = -1 // invalidate cache so highlight renders

	if len(m.activeEventLineStarts) > newIdx {
		targetLine := m.activeEventLineStarts[newIdx]
		pageSize := activeSessionPageSize(m)
		maxOffset := activeSessionMaxScrollOffset(m)
		if targetLine < m.ScrollOffset {
			m.ScrollOffset = targetLine
		} else if targetLine >= m.ScrollOffset+pageSize {
			m.ScrollOffset = targetLine - pageSize + 1
		}
		clampActiveSessionModelScroll(&m)
		_ = maxOffset
	}

	return m, nil
}

func (m Model) handleEditEvent() (tea.Model, tea.Cmd) {
	if m.SelectedEvent < 0 || m.Palette == nil {
		return m, nil
	}

	fullIdx := m.selectedEditableEventIndex()
	if fullIdx < 0 || fullIdx >= len(m.Events) {
		return m, nil
	}

	event := m.Events[fullIdx]

	m.EditingEvent = fullIdx
	m.Palette.OpenPalette()
	lines := strings.Split(event.Body, "\n")
	if len(lines) == 1 && lines[0] == "" {
		lines = []string{""}
	}
	m.Palette.MultiLine = true
	m.Palette.MultiLineLines = lines
	m.Palette.MultiLineCursorRow = len(lines) - 1
	m.Palette.MultiLineCursorCol = len(lines[len(lines)-1])
	m.Palette.MultiLineIsBlocker = event.Type == "Blocker"
	m.Palette.Input = ""
	return m, nil
}

func (m Model) handleDeleteEvent() (tea.Model, tea.Cmd) {
	fullIdx := m.selectedEditableEventIndex()
	if fullIdx < 0 || fullIdx >= len(m.Events) {
		return m, nil
	}
	m.DeleteConfirmEvent = fullIdx + 1
	return m, nil
}

func (m Model) selectedEditableEventIndex() int {
	if m.SelectedEvent < 0 {
		return -1
	}

	visibleIdx := 0
	for i, e := range m.Events {
		if e.Type == "Start" || e.IsDeleted {
			continue
		}
		if visibleIdx == m.SelectedEvent {
			if e.Type == "Stop" {
				return -1
			}
			return i
		}
		visibleIdx++
	}
	return -1
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

		name, email, err := m.Config.ResolveAuthorIdentity(internalgit.AuthorIdentity)
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
			diff, err := internalgit.DiffSinceWithContext(started, m.Config.Handoff.DiffContextLines)
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

	// Force every frame to the latest terminal size so resize redraws clear stale cells.
	v = fitRenderedBlock(v, m.Width, m.Height)

	if m.ShowHelp {
		return fitRenderedBlock(renderHelpOverView(m, v), m.Width, m.Height)
	}

	return v
}

func (m *Model) setView(view View) bool {
	changed := m.CurrentView != view
	if changed {
		m.stopLineScroll()
		m.clearTransientMessages()
		if view != SessionList {
			m.sessionListResizing = false
		}
		if view != HandoffPreview {
			m.SavePromptOpen = false
			m.SaveInput = ""
			clearSearchState(&m.Search)
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

	if m.sessionListResizing {
		content := fitRenderedBlock("", m.Width, contentHeight)
		if bottom == "" {
			return content
		}
		return content + "\n" + bottom
	}

	content := fitRenderedBlock(sl.View(), m.Width, contentHeight)
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

func (m Model) handleMultiLineSubmit(msg MultiLineNoteMsg) (tea.Model, tea.Cmd) {
	body := msg.Body
	if body == "" {
		return m, nil
	}
	if m.ActiveSession == nil {
		m.ErrorMessage = "No session displayed"
		return m, nil
	}
	if m.ActiveSession.Closed {
		eventLabel := "notes"
		if msg.IsBlocker {
			eventLabel = "blockers"
		}
		m.ErrorMessage = "Cannot add " + eventLabel + " to a closed session"
		return m, nil
	}

	if m.EditingEvent >= 0 {
		return m.handleCorrectionSubmit(body)
	}

	eventType := "Note"
	if msg.IsBlocker {
		eventType = "Blocker"
	}
	sessionID := m.ActiveSession.ID
	return m, func() tea.Msg {
		err := m.Store.AppendEvent(sessionID, eventType, body)
		if err != nil {
			return CommandErrorMsg{Error: err}
		}
		return m.reloadEvents()
	}
}

func (m Model) handleCorrectionSubmit(body string) (tea.Model, tea.Cmd) {
	if m.EditingEvent < 0 || m.EditingEvent >= len(m.Events) {
		m.EditingEvent = -1
		return m, nil
	}

	event := m.Events[m.EditingEvent]
	correctionBody := store.FormatEditBody(event, "update", body)
	sessionID := m.ActiveSession.ID
	m.SelectedEvent = -1
	m.EditingEvent = -1
	return m, func() tea.Msg {
		err := m.Store.AppendEvent(sessionID, "Edit", correctionBody)
		if err != nil {
			return CommandErrorMsg{Error: err}
		}
		return m.reloadEvents()
	}
}

func (m Model) handleDeleteConfirm() (tea.Model, tea.Cmd) {
	idx := m.DeleteConfirmEvent - 1
	if idx < 0 || idx >= len(m.Events) {
		m.DeleteConfirmEvent = 0
		return m, nil
	}
	if m.ActiveSession == nil {
		m.DeleteConfirmEvent = 0
		m.ErrorMessage = "No session displayed"
		return m, nil
	}
	if m.ActiveSession.Closed {
		m.DeleteConfirmEvent = 0
		m.ErrorMessage = "Cannot delete events from a closed session"
		return m, nil
	}

	event := m.Events[idx]
	editBody := store.FormatEditBody(event, "delete", "")
	sessionID := m.ActiveSession.ID
	m.SelectedEvent = -1
	m.DeleteConfirmEvent = 0
	return m, func() tea.Msg {
		err := m.Store.AppendEvent(sessionID, "Edit", editBody)
		if err != nil {
			return CommandErrorMsg{Error: err}
		}
		return m.reloadEvents()
	}
}
