package tui

import (
	"strings"
	"time"

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

type Model struct {
	CurrentView    View
	ActiveSession  *store.SessionRecord
	Events         []store.SessionEvent
	Palette        *CommandPalette
	Store          *store.Store
	SessionList    SessionListModel
	Root           string
	Width          int
	Height         int
	ScrollOffset   int
	ShowHelp       bool
	ErrorMessage   string
	NoSessionMsg   string
	HandoffContent string
	Title          string
	SavePromptOpen bool
	SaveInput      string
	HandoffMsg     string
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
		return m, nil

	case ActiveSessionLoadedMsg:
		m.ActiveSession = msg.Session
		m.Events = msg.Events
		m.Title = msg.Title
		m.ScrollOffset = 0
		m.CurrentView = ActiveSession
		return m, nil

	case NavigationMsg:
		m.CurrentView = msg.Target
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
			m.HandoffContent = msg.Content
			m.ScrollOffset = 0
			m.CurrentView = HandoffPreview
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
	return m, nil
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

	if key == "esc" {
		if m.CurrentView == HandoffPreview && m.SavePromptOpen {
			m.SavePromptOpen = false
			m.SaveInput = ""
			return m, nil
		}
		return m, tea.Quit
	}

	if key == "q" {
		if m.CurrentView == HandoffPreview {
			if m.ActiveSession != nil {
				m.CurrentView = ActiveSession
			} else {
				m.CurrentView = SessionList
			}
			return m, nil
		}
		return m, tea.Quit
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

	case "j", "down":
		m.ScrollOffset++
		return m, nil

	case "k", "up":
		if m.ScrollOffset > 0 {
			m.ScrollOffset--
		}
		return m, nil
	}

	return m, nil
}

func (m Model) noSessionKeyHandler(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "l":
		m.CurrentView = SessionList
		return m, nil

	case "o":
		m.NoSessionMsg = "Use `devlog open` to start a session"
		return m, nil
	}

	return m, nil
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
		return m, func() tea.Msg {
			err := session.AppendEventToActiveSession(m.Store, "Note", args)
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
		return m, func() tea.Msg {
			err := session.AppendEventToActiveSession(m.Store, "Blocker", args)
			if err != nil {
				return CommandErrorMsg{Error: err}
			}
			return m.reloadEvents()
		}

	case "/close":
		return m, func() tea.Msg {
			err := session.CloseActiveSession(m.Store)
			if err != nil {
				return CommandErrorMsg{Error: err}
			}
			return ActiveSessionLoadedMsg{}
		}

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
		v = m.SessionList.View()
	case HandoffPreview:
		v = renderHandoffPreview(m)
	default:
		v = "<devlog TUI>"
	}

	if m.ErrorMessage != "" {
		v += "\n" + ErrorBannerStyle.Render(" ERROR: "+m.ErrorMessage)
	}

	if m.HandoffMsg != "" {
		v += "\n" + HintStyle.Render(formatHandoffConfirmation(m.HandoffMsg))
	}

	if m.NoSessionMsg != "" {
		v += "\n" + HintStyle.Render(m.NoSessionMsg)
	}

	return v
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
