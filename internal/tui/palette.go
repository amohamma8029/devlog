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
	Open              bool
	Input             string
	History           []string
	SelectedIndex     int
	HoverIndex        int
	CursorVisible     bool
	SessionClosed     bool
	offsetY           int
	MultiLine         bool
	MultiLineLines    []string
	MultiLineCursorRow int
	MultiLineCursorCol int
	MultiLineIsBlocker bool
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
	p.SessionClosed = false
	p.MultiLine = false
	p.MultiLineLines = nil
}

func (p *CommandPalette) EnterMultiLine(isBlocker bool) {
	p.MultiLine = true
	p.MultiLineLines = []string{""}
	p.MultiLineCursorRow = 0
	p.MultiLineCursorCol = 0
	p.MultiLineIsBlocker = isBlocker
	p.Input = ""
}

func visiblePaletteCommands(p CommandPalette) []CommandEntry {
	var filtered []CommandEntry
	for _, cmd := range PaletteCommands {
		if p.SessionClosed && (cmd.Command == "/note" || cmd.Command == "/block" || cmd.Command == "/close") {
			continue
		}
		if p.Input == "" || p.Input == "/" || strings.HasPrefix(strings.ToLower(cmd.Command), strings.ToLower(p.Input)) {
			filtered = append(filtered, cmd)
		}
	}
	return filtered
}

func (p *CommandPalette) ClosePalette() {
	p.Open = false
	p.SelectedIndex = -1
	p.HoverIndex = -1
	p.MultiLine = false
	p.MultiLineLines = nil
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

	visible := visiblePaletteCommands(*p)
	menuY := msg.Y - p.offsetY

	if msg.Button == tea.MouseButtonNone || msg.Action == tea.MouseActionMotion {
		if menuY >= 0 && menuY < len(visible) {
			p.HoverIndex = menuY
		} else {
			p.HoverIndex = -1
		}
		return nil, p
	}

	if msg.Action != tea.MouseActionPress {
		return nil, p
	}

	if menuY >= 0 && menuY < len(visible) {
		p.SelectedIndex = menuY
		entry := visible[menuY]
		if entry.NoArg {
			p.ClosePalette()
			return func() tea.Msg { return CommandExecutedMsg{Input: entry.Command} }, p
		}
		p.Input = entry.Command + " "
		p.SelectedIndex = -1
		p.HoverIndex = -1
		if entry.Command == "/note" {
			p.EnterMultiLine(false)
		} else if entry.Command == "/block" {
			p.EnterMultiLine(true)
		}
		return nil, p
	}

	return nil, p
}

func (p *CommandPalette) handleKey(msg tea.KeyMsg) (tea.Cmd, *CommandPalette) {
	if p.MultiLine {
		return p.handleMultiLineKey(msg)
	}

	visible := visiblePaletteCommands(*p)

	switch msg.String() {
	case "esc":
		p.ClosePalette()
		return nil, p

	case "enter":
		if p.SelectedIndex >= 0 && p.SelectedIndex < len(visible) {
			entry := visible[p.SelectedIndex]
			if entry.NoArg {
				p.ClosePalette()
				return func() tea.Msg { return CommandExecutedMsg{Input: entry.Command} }, p
			}
			p.Input = entry.Command + " "
			p.SelectedIndex = -1
			p.HoverIndex = -1
			if entry.Command == "/note" {
				p.EnterMultiLine(false)
			} else if entry.Command == "/block" {
				p.EnterMultiLine(true)
			}
			return nil, p
		}
		if p.Input != "" {
			trimmed := strings.TrimSpace(p.Input)
			if trimmed == "/note" {
				p.EnterMultiLine(false)
				return nil, p
			}
			if trimmed == "/block" {
				p.EnterMultiLine(true)
				return nil, p
			}
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
		if p.SelectedIndex >= len(visible) {
			p.SelectedIndex = len(visible) - 1
		}
		if len(visible) == 0 {
			p.SelectedIndex = -1
		}

	case "up":
		if len(visible) == 0 {
			p.SelectedIndex = -1
		} else if p.SelectedIndex > 0 {
			p.SelectedIndex--
		} else {
			p.SelectedIndex = len(visible) - 1
		}
		p.HoverIndex = -1

	case "down":
		if len(visible) == 0 {
			p.SelectedIndex = -1
		} else if p.SelectedIndex < len(visible)-1 {
			p.SelectedIndex++
		}
		p.HoverIndex = -1

	case "tab":
		if len(visible) == 0 {
			p.SelectedIndex = -1
		} else if p.SelectedIndex < len(visible)-1 {
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
			if msg.Runes[0] == ' ' {
				trimmed := strings.TrimSpace(p.Input)
				if trimmed == "/note" {
					p.EnterMultiLine(false)
				} else if trimmed == "/block" {
					p.EnterMultiLine(true)
				}
			}
		}
	}

	return nil, p
}

func (p *CommandPalette) handleMultiLineKey(msg tea.KeyMsg) (tea.Cmd, *CommandPalette) {
	if msg.Alt && msg.Type == tea.KeyEnter {
		line := p.MultiLineLines[p.MultiLineCursorRow]
		left := line[:p.MultiLineCursorCol]
		right := line[p.MultiLineCursorCol:]
		p.MultiLineLines[p.MultiLineCursorRow] = left
		p.MultiLineLines = append(p.MultiLineLines[:p.MultiLineCursorRow+1],
			append([]string{right}, p.MultiLineLines[p.MultiLineCursorRow+1:]...)...)
		p.MultiLineCursorRow++
		p.MultiLineCursorCol = 0
		return nil, p
	}

	switch msg.String() {
	case "enter":
		body := strings.TrimSpace(strings.Join(p.MultiLineLines, "\n"))
		isBlocker := p.MultiLineIsBlocker
		p.ClosePalette()
		if body == "" {
			return nil, p
		}
		return func() tea.Msg {
			return MultiLineNoteMsg{Body: body, IsBlocker: isBlocker}
		}, p

	case "esc":
		p.ClosePalette()
		return nil, p

	case "backspace":
		if p.MultiLineCursorCol > 0 {
			line := p.MultiLineLines[p.MultiLineCursorRow]
			p.MultiLineLines[p.MultiLineCursorRow] = line[:p.MultiLineCursorCol-1] + line[p.MultiLineCursorCol:]
			p.MultiLineCursorCol--
		} else if p.MultiLineCursorRow > 0 {
			p.MultiLineCursorCol = len(p.MultiLineLines[p.MultiLineCursorRow-1])
			p.MultiLineLines[p.MultiLineCursorRow-1] += p.MultiLineLines[p.MultiLineCursorRow]
			p.MultiLineLines = append(p.MultiLineLines[:p.MultiLineCursorRow],
				p.MultiLineLines[p.MultiLineCursorRow+1:]...)
			p.MultiLineCursorRow--
		} else if len(p.MultiLineLines[0]) == 0 {
			p.MultiLine = false
			p.MultiLineLines = nil
			if p.MultiLineIsBlocker {
				p.Input = "/block "
			} else {
				p.Input = "/note "
			}
		}
		return nil, p

	case "left":
		if p.MultiLineCursorCol > 0 {
			p.MultiLineCursorCol--
		} else if p.MultiLineCursorRow > 0 {
			p.MultiLineCursorRow--
			p.MultiLineCursorCol = len(p.MultiLineLines[p.MultiLineCursorRow])
		}
		return nil, p

	case "right":
		if p.MultiLineCursorCol < len(p.MultiLineLines[p.MultiLineCursorRow]) {
			p.MultiLineCursorCol++
		} else if p.MultiLineCursorRow < len(p.MultiLineLines)-1 {
			p.MultiLineCursorRow++
			p.MultiLineCursorCol = 0
		}
		return nil, p

	case "up":
		if p.MultiLineCursorRow > 0 {
			p.MultiLineCursorRow--
			if p.MultiLineCursorCol > len(p.MultiLineLines[p.MultiLineCursorRow]) {
				p.MultiLineCursorCol = len(p.MultiLineLines[p.MultiLineCursorRow])
			}
		}
		return nil, p

	case "down":
		if p.MultiLineCursorRow < len(p.MultiLineLines)-1 {
			p.MultiLineCursorRow++
			if p.MultiLineCursorCol > len(p.MultiLineLines[p.MultiLineCursorRow]) {
				p.MultiLineCursorCol = len(p.MultiLineLines[p.MultiLineCursorRow])
			}
		}
		return nil, p

	default:
		if len(msg.Runes) == 1 {
			r := msg.Runes[0]
			if r >= 32 {
				line := p.MultiLineLines[p.MultiLineCursorRow]
				p.MultiLineLines[p.MultiLineCursorRow] = line[:p.MultiLineCursorCol] + string(r) + line[p.MultiLineCursorCol:]
				p.MultiLineCursorCol++
			}
		}
		return nil, p
	}
}

func (p *CommandPalette) SetOffset(offset int) {
	p.offsetY = offset
}

func (p CommandPalette) View() string {
	if !p.Open {
		return ""
	}

	if p.MultiLine {
		return p.viewMultiLine()
	}

	visible := visiblePaletteCommands(p)
	var menu strings.Builder

	if len(visible) == 0 {
		menu.WriteString(MenuStyle.Render("No matching commands"))
	} else {
		for i, cmd := range visible {
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

func (p CommandPalette) viewMultiLine() string {
	var b strings.Builder

	token := "/note"
	if p.MultiLineIsBlocker {
		token = "/block"
	}
	tokenWidth := xansi.StringWidth(token) + 1 + MenuSelectedStyle.GetHorizontalFrameSize()
	indent := strings.Repeat(" ", tokenWidth)

	for i, line := range p.MultiLineLines {
		if i == 0 {
			b.WriteString(MenuSelectedStyle.Render(token))
			b.WriteString(" ")
		} else {
			b.WriteString(MetadataStyle.Render(indent))
		}

		cursorChar := " "
		if i == p.MultiLineCursorRow && p.CursorVisible {
			cursorChar = CursorStyle.Render("|")
		}

		if i == p.MultiLineCursorRow {
			left := line[:p.MultiLineCursorCol]
			right := line[p.MultiLineCursorCol:]
			b.WriteString(MetadataStyle.Render(left))
			b.WriteString(cursorChar)
			if right == "" {
				right = " "
			}
			b.WriteString(MetadataStyle.Render(right))
		} else {
			display := line
			if display == "" {
				display = " "
			}
			b.WriteString(MetadataStyle.Render(display))
		}
		b.WriteByte('\n')
	}

	b.WriteString(HintStyle.Render("Alt+Enter new line  ·  Enter submit  ·  Esc cancel"))
	return MenuBoxStyle.Render(b.String())
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
