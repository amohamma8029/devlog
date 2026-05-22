package tui

import tea "github.com/charmbracelet/bubbletea"

type sessionListView struct{}

func (v sessionListView) Init() tea.Cmd                         { return nil }
func (v sessionListView) Update(msg tea.Msg) (tea.Model, tea.Cmd) { return v, nil }
func (v sessionListView) View() string                           { return "<SessionList>" }

type activeSessionView struct{}

func (v activeSessionView) Init() tea.Cmd                         { return nil }
func (v activeSessionView) Update(msg tea.Msg) (tea.Model, tea.Cmd) { return v, nil }
func (v activeSessionView) View() string                           { return "<ActiveSession>" }

type handoffPreviewView struct{}

func (v handoffPreviewView) Init() tea.Cmd                         { return nil }
func (v handoffPreviewView) Update(msg tea.Msg) (tea.Model, tea.Cmd) { return v, nil }
func (v handoffPreviewView) View() string                           { return "<HandoffPreview>" }

func viewForState(m Model) tea.Model {
	switch m.CurrentView {
	case SessionList:
		return sessionListView{}
	case ActiveSession:
		return activeSessionView{}
	case HandoffPreview:
		return handoffPreviewView{}
	default:
		return nil
	}
}
