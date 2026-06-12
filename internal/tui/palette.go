package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	xansi "github.com/charmbracelet/x/ansi"
)

type CommandEntry struct {
	Command     string
	Description string
	NoArg       bool
}

var PaletteCommands = []CommandEntry{
	{Command: "/note", Description: "Add a note", NoArg: false},
	{Command: "/block", Description: "Log a blocker", NoArg: false},
	{Command: "/close", Description: "Close the session", NoArg: true},
	{Command: "/handoff", Description: "Generate handoff summary", NoArg: true},
	{Command: "/list", Description: "Go to session list", NoArg: true},
}

type CommandPalette struct {
	Open          bool
	Input         string
	History       []string
	SelectedIndex int
	HoverIndex    int
	CursorVisible bool
	offsetY       int
}

func NewCommandPalette() CommandPalette {
	return CommandPalette{
		SelectedIndex: -1,
		HoverIndex:    -1,
	}
}

func (p *CommandPalette) OpenPalette() {
	p.Open = true
	p.Input = ""
	p.SelectedIndex = -1
	p.HoverIndex = -1
	p.CursorVisible = true
}

func (p *CommandPalette) ClosePalette() {
	p.Open = false
	p.SelectedIndex = -1
	p.HoverIndex = -1
}

func (p *CommandPalette) Update(msg tea.Msg) (tea.Cmd, *CommandPalette) {
	if !p.Open {
		return nil, p
	}

	switch msg := msg.(type) {
	case tea.MouseMsg:
		return p.handleMouse(msg)

	case tea.KeyMsg:
		return p.handleKey(msg)
	}

	return nil, p
}

func (p *CommandPalette) handleMouse(msg tea.MouseMsg) (tea.Cmd, *CommandPalette) {
	if msg.Button != tea.MouseButtonLeft && msg.Button != tea.MouseButtonNone {
		return nil, p
	}

	menuY := msg.Y - p.offsetY

	if msg.Button == tea.MouseButtonNone || msg.Action == tea.MouseActionMotion {
		if menuY >= 0 && menuY < len(PaletteCommands) {
			p.HoverIndex = menuY
		} else {
			p.HoverIndex = -1
		}
		return nil, p
	}

	if msg.Action != tea.MouseActionPress {
		return nil, p
	}

	if menuY >= 0 && menuY < len(PaletteCommands) {
		p.SelectedIndex = menuY
		entry := PaletteCommands[menuY]
		if entry.NoArg {
			p.ClosePalette()
			return func() tea.Msg { return CommandExecutedMsg{Input: entry.Command} }, p
		}
		p.Input = entry.Command + " "
		p.SelectedIndex = -1
		p.HoverIndex = -1
		return nil, p
	}

	return nil, p
}

func (p *CommandPalette) handleKey(msg tea.KeyMsg) (tea.Cmd, *CommandPalette) {
	switch msg.String() {
	case "esc":
		p.ClosePalette()
		return nil, p

	case "enter":
		if p.SelectedIndex >= 0 && p.SelectedIndex < len(PaletteCommands) {
			entry := PaletteCommands[p.SelectedIndex]
			if entry.NoArg {
				p.ClosePalette()
				return func() tea.Msg { return CommandExecutedMsg{Input: entry.Command} }, p
			}
			p.Input = entry.Command + " "
			p.SelectedIndex = -1
			p.HoverIndex = -1
			return nil, p
		}
		if p.Input != "" {
			p.History = append(p.History, p.Input)
			input := p.Input
			p.Input = ""
			p.ClosePalette()
			return func() tea.Msg { return CommandExecutedMsg{Input: input} }, p
		}

	case "backspace":
		if len(p.Input) > 0 {
			p.Input = p.Input[:len(p.Input)-1]
		}

	case "up":
		if p.SelectedIndex > 0 {
			p.SelectedIndex--
		} else {
			p.SelectedIndex = len(PaletteCommands) - 1
		}
		p.HoverIndex = -1

	case "down":
		if p.SelectedIndex < len(PaletteCommands)-1 {
			p.SelectedIndex++
		}
		p.HoverIndex = -1

	case "tab":
		if p.SelectedIndex < len(PaletteCommands)-1 {
			p.SelectedIndex++
		} else {
			p.SelectedIndex = 0
		}
		p.HoverIndex = -1

	default:
		if len(msg.Runes) == 1 {
			p.Input += string(msg.Runes[0])
			p.SelectedIndex = -1
			p.HoverIndex = -1
		}
	}

	return nil, p
}

func (p *CommandPalette) SetOffset(offset int) {
	p.offsetY = offset
}

func (p CommandPalette) View() string {
	if !p.Open {
		return ""
	}

	var menu strings.Builder

	for i, cmd := range PaletteCommands {
		line := cmd.Command + "  " + HintStyle.Render(cmd.Description)
		if i == p.SelectedIndex {
			menu.WriteString(MenuSelectedStyle.Render(line))
		} else if i == p.HoverIndex {
			menu.WriteString(MenuHoverStyle.Render(line))
		} else {
			menu.WriteString(MenuStyle.Render(line))
		}
		menu.WriteByte('\n')
	}

	menuBox := MenuBoxStyle.Render(strings.TrimRight(menu.String(), "\n"))
	inputWidth := maxRenderedLineWidth(menuBox)

	cursorChar := " "
	if p.CursorVisible {
		cursorChar = CursorStyle.Render("|")
	}

	contentWidth := inputWidth - PaletteInputStyle.GetHorizontalFrameSize()
	if contentWidth < 1 {
		contentWidth = 1
	}
	style := PaletteInputStyle.Width(contentWidth)
	inputDisplay := renderBoundedInputContent(" ", p.Input, cursorChar, "", contentWidth)

	return menuBox + "\n" + style.Render(inputDisplay)
}

func maxRenderedLineWidth(rendered string) int {
	maxWidth := 1
	for _, line := range strings.Split(rendered, "\n") {
		if width := xansi.StringWidth(line); width > maxWidth {
			maxWidth = width
		}
	}
	return maxWidth
}
