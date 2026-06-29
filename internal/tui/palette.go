package tui

import (
	"sort"
	"strings"

	"github.com/atotto/clipboard"
	tea "github.com/charmbracelet/bubbletea"
	xansi "github.com/charmbracelet/x/ansi"
)

type CommandEntry struct {
	Command     string
	Description string
	NoArg       bool
}

var PaletteCommands = []CommandEntry{
	{Command: "/exit", Description: "Exit the TUI", NoArg: true},
	{Command: "/close", Description: "Close the session", NoArg: true},
	{Command: "/list", Description: "Go to session list", NoArg: true},
	{Command: "/todo", Description: "Open todo list", NoArg: true},
	{Command: "/handoff", Description: "Generate handoff summary", NoArg: true},
	{Command: "/block", Description: "Log a blocker", NoArg: false},
	{Command: "/note", Description: "Add a note", NoArg: false},
}

type CommandPalette struct {
	Open               bool
	Input              string
	History            []string
	SelectedIndex      int
	HoverIndex         int
	CursorVisible      bool
	SessionClosed      bool
	offsetY            int
	width              int
	MultiLine          bool
	MultiLineLines     []string
	MultiLineCursorRow int
	MultiLineCursorCol int
	MultiLineIsBlocker bool
	MultiLineIsTodo    bool
	SelectionAnchorRow int
	SelectionAnchorCol int
	HasSelection       bool
	InputCursorPos     int
	InputHasSelection  bool
	InputSelAnchor     int
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
	p.HasSelection = false
	p.InputCursorPos = 0
	p.InputHasSelection = false
}

func (p *CommandPalette) EnterMultiLine(isBlocker bool) {
	p.MultiLine = true
	p.MultiLineLines = []string{""}
	p.MultiLineCursorRow = 0
	p.MultiLineCursorCol = 0
	p.MultiLineIsBlocker = isBlocker
	p.MultiLineIsTodo = false
	p.Input = ""
	p.HasSelection = false
}

func (p *CommandPalette) clearSelection() {
	p.HasSelection = false
}

func (p *CommandPalette) inputSelBounds() (start, end int) {
	if p.InputSelAnchor < p.InputCursorPos {
		return p.InputSelAnchor, p.InputCursorPos
	}
	return p.InputCursorPos, p.InputSelAnchor
}

func (p *CommandPalette) startSelection() {
	p.SelectionAnchorRow = p.MultiLineCursorRow
	p.SelectionAnchorCol = p.MultiLineCursorCol
	p.HasSelection = true
}

func (p *CommandPalette) normalizedSelection() (startRow, startCol, endRow, endCol int) {
	if !p.HasSelection {
		return -1, -1, -1, -1
	}
	ar, ac := p.SelectionAnchorRow, p.SelectionAnchorCol
	cr, cc := p.MultiLineCursorRow, p.MultiLineCursorCol
	if ar < cr || (ar == cr && ac <= cc) {
		return ar, ac, cr, cc
	}
	return cr, cc, ar, ac
}

func (p *CommandPalette) selectedText() string {
	if !p.HasSelection {
		return ""
	}
	sr, sc, er, ec := p.normalizedSelection()
	if sr == er {
		return p.MultiLineLines[sr][sc:ec]
	}
	var parts []string
	parts = append(parts, p.MultiLineLines[sr][sc:])
	for r := sr + 1; r < er; r++ {
		parts = append(parts, p.MultiLineLines[r])
	}
	parts = append(parts, p.MultiLineLines[er][:ec])
	return strings.Join(parts, "\n")
}

func (p *CommandPalette) deleteSelection() {
	if !p.HasSelection {
		return
	}
	sr, sc, er, ec := p.normalizedSelection()
	if sr == er {
		line := p.MultiLineLines[sr]
		p.MultiLineLines[sr] = line[:sc] + line[ec:]
		p.MultiLineCursorRow = sr
		p.MultiLineCursorCol = sc
		p.HasSelection = false
		return
	}
	left := p.MultiLineLines[sr][:sc]
	right := p.MultiLineLines[er][ec:]
	p.MultiLineLines[sr] = left + right
	p.MultiLineLines = append(
		p.MultiLineLines[:sr+1],
		p.MultiLineLines[er+1:]...,
	)
	p.MultiLineCursorRow = sr
	p.MultiLineCursorCol = sc
	p.HasSelection = false
}

func normalizePastedText(text string) string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	return strings.ReplaceAll(text, "\r", "\n")
}

func (p *CommandPalette) insertMultiLineBreak() {
	p.HasSelection = false
	line := p.MultiLineLines[p.MultiLineCursorRow]
	left := line[:p.MultiLineCursorCol]
	right := line[p.MultiLineCursorCol:]
	p.MultiLineLines[p.MultiLineCursorRow] = left
	p.MultiLineLines = append(p.MultiLineLines[:p.MultiLineCursorRow+1],
		append([]string{right}, p.MultiLineLines[p.MultiLineCursorRow+1:]...)...)
	p.MultiLineCursorRow++
	p.MultiLineCursorCol = 0
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
	p.MultiLineIsBlocker = false
	p.MultiLineIsTodo = false
	p.HasSelection = false
	p.InputHasSelection = false
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

	case pasteMsg:
		return p.handlePaste(msg)
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
	if msg.Paste {
		return p.handlePaste(pasteMsg{text: string(msg.Runes)})
	}

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
			p.InputCursorPos = len(p.Input)
			p.InputHasSelection = false
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
		if p.InputHasSelection {
			start, end := p.inputSelBounds()
			p.Input = p.Input[:start] + p.Input[end:]
			p.InputCursorPos = start
			p.InputHasSelection = false
		} else if p.InputCursorPos > 0 {
			p.Input = p.Input[:p.InputCursorPos-1] + p.Input[p.InputCursorPos:]
			p.InputCursorPos--
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

	case "left":
		p.InputHasSelection = false
		if p.InputCursorPos > 0 {
			p.InputCursorPos--
		}

	case "right":
		p.InputHasSelection = false
		if p.InputCursorPos < len(p.Input) {
			p.InputCursorPos++
		}

	case "home":
		p.InputHasSelection = false
		p.InputCursorPos = 0

	case "end":
		p.InputHasSelection = false
		p.InputCursorPos = len(p.Input)

	case "shift+left":
		if !p.InputHasSelection {
			p.InputSelAnchor = p.InputCursorPos
			p.InputHasSelection = true
		}
		if p.InputCursorPos > 0 {
			p.InputCursorPos--
		}

	case "shift+right":
		if !p.InputHasSelection {
			p.InputSelAnchor = p.InputCursorPos
			p.InputHasSelection = true
		}
		if p.InputCursorPos < len(p.Input) {
			p.InputCursorPos++
		}

	case "shift+home":
		if !p.InputHasSelection {
			p.InputSelAnchor = p.InputCursorPos
			p.InputHasSelection = true
		}
		p.InputCursorPos = 0

	case "shift+end":
		if !p.InputHasSelection {
			p.InputSelAnchor = p.InputCursorPos
			p.InputHasSelection = true
		}
		p.InputCursorPos = len(p.Input)

	case "ctrl+a":
		if p.Input != "" {
			p.InputSelAnchor = 0
			p.InputCursorPos = len(p.Input)
			p.InputHasSelection = true
		}
		return nil, p

	case "ctrl+c":
		if p.InputHasSelection {
			start, end := p.inputSelBounds()
			text := p.Input[start:end]
			if text != "" {
				return func() tea.Msg {
					clipboard.WriteAll(text)
					return ClipboardActionMsg{Action: "copy"}
				}, p
			}
		}
		return nil, p

	case "ctrl+x":
		if p.InputHasSelection {
			start, end := p.inputSelBounds()
			text := p.Input[start:end]
			if text != "" {
				p.Input = p.Input[:start] + p.Input[end:]
				p.InputCursorPos = start
				p.InputHasSelection = false
				return func() tea.Msg {
					clipboard.WriteAll(text)
					return ClipboardActionMsg{Action: "cut"}
				}, p
			}
		}
		return nil, p

	case "ctrl+v":
		return func() tea.Msg {
			text, err := clipboard.ReadAll()
			if err != nil || text == "" {
				return nil
			}
			return pasteMsg{text: text}
		}, p

	default:
		if len(msg.Runes) == 1 {
			r := msg.Runes[0]
			if r >= 32 {
				if p.InputHasSelection {
					start, end := p.inputSelBounds()
					p.Input = p.Input[:start] + p.Input[end:]
					p.InputCursorPos = start
					p.InputHasSelection = false
				}
				p.Input = p.Input[:p.InputCursorPos] + string(r) + p.Input[p.InputCursorPos:]
				p.InputCursorPos++
			}
			p.SelectedIndex = -1
			p.HoverIndex = -1
			if r == ' ' {
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

func (p *CommandPalette) handlePaste(msg pasteMsg) (tea.Cmd, *CommandPalette) {
	text := msg.text
	if text == "" {
		return nil, p
	}
	text = normalizePastedText(text)

	if !p.MultiLine {
		pasteLines := strings.Split(text, "\n")
		if p.InputHasSelection {
			start, end := p.inputSelBounds()
			p.Input = p.Input[:start] + p.Input[end:]
			p.InputCursorPos = start
			p.InputHasSelection = false
		}
		p.Input = p.Input[:p.InputCursorPos] + pasteLines[0] + p.Input[p.InputCursorPos:]
		p.InputCursorPos += len(pasteLines[0])
		return nil, p
	}

	if p.HasSelection {
		p.deleteSelection()
	}

	pasteLines := strings.Split(text, "\n")
	if len(pasteLines) == 0 {
		return nil, p
	}

	cursorLine := p.MultiLineLines[p.MultiLineCursorRow]
	left := cursorLine[:p.MultiLineCursorCol]
	right := cursorLine[p.MultiLineCursorCol:]

	p.MultiLineLines[p.MultiLineCursorRow] = left + pasteLines[0]

	numMiddle := len(pasteLines) - 2
	for j := 0; j < numMiddle; j++ {
		insertIdx := p.MultiLineCursorRow + 1 + j
		p.MultiLineLines = append(
			p.MultiLineLines[:insertIdx],
			append([]string{pasteLines[1+j]}, p.MultiLineLines[insertIdx:]...)...,
		)
	}

	if len(pasteLines) > 1 {
		insertIdx := p.MultiLineCursorRow + 1 + numMiddle
		p.MultiLineLines = append(
			p.MultiLineLines[:insertIdx],
			append([]string{pasteLines[len(pasteLines)-1] + right}, p.MultiLineLines[insertIdx:]...)...,
		)
	} else {
		p.MultiLineLines[p.MultiLineCursorRow] += right
	}

	p.MultiLineCursorRow += len(pasteLines) - 1
	if len(pasteLines) > 1 {
		p.MultiLineCursorCol = len(pasteLines[len(pasteLines)-1])
	} else {
		p.MultiLineCursorCol = len(left) + len(pasteLines[0])
	}

	return nil, p
}

func (p *CommandPalette) handleMultiLineKey(msg tea.KeyMsg) (tea.Cmd, *CommandPalette) {
	if msg.Alt && msg.Type == tea.KeyEnter {
		p.insertMultiLineBreak()
		return nil, p
	}

	switch msg.String() {
	case "enter":
		body := strings.TrimSpace(strings.Join(p.MultiLineLines, "\n"))
		isBlocker := p.MultiLineIsBlocker
		isTodo := p.MultiLineIsTodo
		p.ClosePalette()
		if body == "" {
			return nil, p
		}
		return func() tea.Msg {
			return MultiLineNoteMsg{Body: body, IsBlocker: isBlocker, IsTodo: isTodo}
		}, p

	case "esc":
		p.ClosePalette()
		return nil, p

	case "ctrl+c":
		if p.HasSelection {
			text := p.selectedText()
			if text != "" {
				return func() tea.Msg {
					clipboard.WriteAll(text)
					return ClipboardActionMsg{Action: "copy"}
				}, p
			}
		}
		return nil, p

	case "ctrl+x":
		if p.HasSelection {
			text := p.selectedText()
			if text != "" {
				p.deleteSelection()
				return func() tea.Msg {
					clipboard.WriteAll(text)
					return ClipboardActionMsg{Action: "cut"}
				}, p
			}
		}
		return nil, p

	case "ctrl+v":
		return func() tea.Msg {
			text, err := clipboard.ReadAll()
			if err != nil || text == "" {
				return nil
			}
			return pasteMsg{text: text}
		}, p

	case "backspace":
		if p.HasSelection {
			p.deleteSelection()
			return nil, p
		}
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
			if p.MultiLineIsTodo {
				return nil, p
			}
			p.MultiLine = false
			p.MultiLineLines = nil
			if p.MultiLineIsBlocker {
				p.Input = "/block "
			} else {
				p.Input = "/note "
			}
		}
		return nil, p

	case "delete":
		if p.HasSelection {
			p.deleteSelection()
			return nil, p
		}
		line := p.MultiLineLines[p.MultiLineCursorRow]
		if p.MultiLineCursorCol < len(line) {
			p.MultiLineLines[p.MultiLineCursorRow] = line[:p.MultiLineCursorCol] + line[p.MultiLineCursorCol+1:]
		} else if p.MultiLineCursorRow < len(p.MultiLineLines)-1 {
			p.MultiLineLines[p.MultiLineCursorRow] += p.MultiLineLines[p.MultiLineCursorRow+1]
			p.MultiLineLines = append(p.MultiLineLines[:p.MultiLineCursorRow+1],
				p.MultiLineLines[p.MultiLineCursorRow+2:]...)
		}
		return nil, p

	case "home":
		p.HasSelection = false
		p.MultiLineCursorCol = 0
		return nil, p

	case "end":
		p.HasSelection = false
		p.MultiLineCursorCol = len(p.MultiLineLines[p.MultiLineCursorRow])
		return nil, p

	case "shift+left":
		if !p.HasSelection {
			p.startSelection()
		}
		if p.MultiLineCursorCol > 0 {
			p.MultiLineCursorCol--
		} else if p.MultiLineCursorRow > 0 {
			p.MultiLineCursorRow--
			p.MultiLineCursorCol = len(p.MultiLineLines[p.MultiLineCursorRow])
		}
		return nil, p

	case "shift+right":
		if !p.HasSelection {
			p.startSelection()
		}
		if p.MultiLineCursorCol < len(p.MultiLineLines[p.MultiLineCursorRow]) {
			p.MultiLineCursorCol++
		} else if p.MultiLineCursorRow < len(p.MultiLineLines)-1 {
			p.MultiLineCursorRow++
			p.MultiLineCursorCol = 0
		}
		return nil, p

	case "shift+up":
		if !p.HasSelection {
			p.startSelection()
		}
		if p.MultiLineCursorRow > 0 {
			p.MultiLineCursorRow--
			if p.MultiLineCursorCol > len(p.MultiLineLines[p.MultiLineCursorRow]) {
				p.MultiLineCursorCol = len(p.MultiLineLines[p.MultiLineCursorRow])
			}
		}
		return nil, p

	case "shift+down":
		if !p.HasSelection {
			p.startSelection()
		}
		if p.MultiLineCursorRow < len(p.MultiLineLines)-1 {
			p.MultiLineCursorRow++
			if p.MultiLineCursorCol > len(p.MultiLineLines[p.MultiLineCursorRow]) {
				p.MultiLineCursorCol = len(p.MultiLineLines[p.MultiLineCursorRow])
			}
		}
		return nil, p

	case "shift+home":
		if !p.HasSelection {
			p.startSelection()
		}
		p.MultiLineCursorRow = 0
		p.MultiLineCursorCol = 0
		return nil, p

	case "shift+end":
		if !p.HasSelection {
			p.startSelection()
		}
		p.MultiLineCursorRow = len(p.MultiLineLines) - 1
		p.MultiLineCursorCol = len(p.MultiLineLines[p.MultiLineCursorRow])
		return nil, p

	case "ctrl+a":
		if len(p.MultiLineLines) > 0 && (len(p.MultiLineLines) > 1 || p.MultiLineLines[0] != "") {
			p.SelectionAnchorRow = 0
			p.SelectionAnchorCol = 0
			p.MultiLineCursorRow = len(p.MultiLineLines) - 1
			p.MultiLineCursorCol = len(p.MultiLineLines[p.MultiLineCursorRow])
			p.HasSelection = true
		}
		return nil, p

	case "left":
		p.HasSelection = false
		if p.MultiLineCursorCol > 0 {
			p.MultiLineCursorCol--
		} else if p.MultiLineCursorRow > 0 {
			p.MultiLineCursorRow--
			p.MultiLineCursorCol = len(p.MultiLineLines[p.MultiLineCursorRow])
		}
		return nil, p

	case "right":
		p.HasSelection = false
		if p.MultiLineCursorCol < len(p.MultiLineLines[p.MultiLineCursorRow]) {
			p.MultiLineCursorCol++
		} else if p.MultiLineCursorRow < len(p.MultiLineLines)-1 {
			p.MultiLineCursorRow++
			p.MultiLineCursorCol = 0
		}
		return nil, p

	case "up":
		p.HasSelection = false
		if p.MultiLineCursorRow > 0 {
			p.MultiLineCursorRow--
			if p.MultiLineCursorCol > len(p.MultiLineLines[p.MultiLineCursorRow]) {
				p.MultiLineCursorCol = len(p.MultiLineLines[p.MultiLineCursorRow])
			}
		}
		return nil, p

	case "down":
		p.HasSelection = false
		if p.MultiLineCursorRow < len(p.MultiLineLines)-1 {
			p.MultiLineCursorRow++
			if p.MultiLineCursorCol > len(p.MultiLineLines[p.MultiLineCursorRow]) {
				p.MultiLineCursorCol = len(p.MultiLineLines[p.MultiLineCursorRow])
			}
		}
		return nil, p

	default:
		if len(msg.Runes) > 0 {
			text := string(msg.Runes)
			if strings.ContainsAny(text, "\r\n") {
				return p.handlePaste(pasteMsg{text: text})
			}
			if strings.IndexFunc(text, func(r rune) bool { return r < 32 }) < 0 {
				if p.HasSelection {
					p.deleteSelection()
				}
				line := p.MultiLineLines[p.MultiLineCursorRow]
				p.MultiLineLines[p.MultiLineCursorRow] = line[:p.MultiLineCursorCol] + text + line[p.MultiLineCursorCol:]
				p.MultiLineCursorCol += len(text)
			}
		}
		return nil, p
	}
}

func (p *CommandPalette) SetOffset(offset int) {
	p.offsetY = offset
}

func (p *CommandPalette) SetWidth(width int) {
	p.width = width
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

	contentWidth := inputWidth - PaletteInputStyle.GetHorizontalFrameSize()
	if contentWidth < 1 {
		contentWidth = 1
	}
	style := PaletteInputStyle.Width(contentWidth)

	var inputDisplay string
	if p.CursorVisible {
		if p.InputCursorPos < len(p.Input) {
			pos := p.InputCursorPos
			cursorChar := string([]rune(p.Input)[pos])
			inputWithCursor := p.Input[:pos] + CursorStyle.Render(cursorChar) + p.Input[pos+1:]
			inputDisplay = renderBoundedInputContent(" ", inputWithCursor, "", "", contentWidth)
		} else {
			inputDisplay = renderBoundedInputContent(" ", p.Input, CursorStyle.Render(" "), "", contentWidth)
		}
	} else {
		inputDisplay = renderBoundedInputContent(" ", p.Input, " ", "", contentWidth)
	}

	return menuBox + "\n" + style.Render(inputDisplay)
}

func (p CommandPalette) viewMultiLine() string {
	var b strings.Builder

	token := "/note"
	tokenStyle := MenuSelectedStyle
	if p.MultiLineIsBlocker {
		token = "/block"
		tokenStyle = ComposerBlockerTokenStyle
	}
	if p.MultiLineIsTodo {
		token = "Todo"
		tokenStyle = ComposerTodoTokenStyle
	}
	tokenWidth := xansi.StringWidth(token) + 1 + MenuSelectedStyle.GetHorizontalFrameSize()
	indent := strings.Repeat(" ", tokenWidth)

	contentWidth := p.width - MenuBoxStyle.GetHorizontalFrameSize()
	if contentWidth < 10 {
		contentWidth = 10
	}
	wrapWidth := contentWidth - tokenWidth
	if wrapWidth < 10 {
		wrapWidth = 10
	}

	sRow, sCol, eRow, eCol := p.normalizedSelection()
	hasSel := sRow >= 0

	for i, line := range p.MultiLineLines {
		wrapped := wrapVisualLine(line, wrapWidth)
		isCursorLine := i == p.MultiLineCursorRow && p.CursorVisible
		cursorCol := p.MultiLineCursorCol
		for wi, chunk := range wrapped {
			if i > 0 || wi > 0 {
				b.WriteByte('\n')
			}

			isFirst := wi == 0
			if i == 0 && isFirst {
				b.WriteString(tokenStyle.Render(token))
				b.WriteString(" ")
			} else if isFirst {
				b.WriteString(MetadataStyle.Render(indent))
			} else {
				b.WriteString(indent)
			}

			chunkStart := wi * wrapWidth
			chunkCol := cursorCol - chunkStart
			cursorInThisChunk := isCursorLine && chunkCol >= 0 && chunkCol < len(chunk)
			if cursorInThisChunk && chunkCol > len(chunk) {
				chunkCol = len(chunk)
			}

			localSelFrom, localSelTo := -1, -1
			if hasSel {
				if i > sRow && i < eRow {
					localSelFrom, localSelTo = 0, len(chunk)
				} else if i == sRow && i == eRow {
					oFrom := sCol - chunkStart
					oTo := eCol - chunkStart
					if oTo > 0 && oFrom < len(chunk) {
						if oFrom < 0 {
							oFrom = 0
						}
						if oTo > len(chunk) {
							oTo = len(chunk)
						}
						localSelFrom, localSelTo = oFrom, oTo
					}
				} else if i == sRow {
					oFrom := sCol - chunkStart
					if oFrom < len(chunk) {
						if oFrom < 0 {
							oFrom = 0
						}
						localSelFrom, localSelTo = oFrom, len(chunk)
					}
				} else if i == eRow {
					oTo := eCol - chunkStart
					if oTo > 0 {
						if oTo > len(chunk) {
							oTo = len(chunk)
						}
						localSelFrom, localSelTo = 0, oTo
					}
				}
			}

			if cursorInThisChunk {
				renderLineWithCursorAndSelection(&b, chunk, chunkCol, localSelFrom, localSelTo)
			} else if localSelFrom >= 0 && localSelFrom < localSelTo {
				if len(chunk) == 0 && localSelFrom == 0 && localSelTo == 0 {
					b.WriteString(ComposerSelectionStyle.Render(" "))
				} else {
					b.WriteString(ComposerBodyStyle.Render(chunk[:localSelFrom]))
					b.WriteString(ComposerSelectionStyle.Render(chunk[localSelFrom:localSelTo]))
					b.WriteString(ComposerBodyStyle.Render(chunk[localSelTo:]))
				}
			} else {
				display := chunk
				if display == "" {
					display = " "
				}
				b.WriteString(ComposerBodyStyle.Render(display))
			}
		}
	}

	b.WriteByte('\n')
	b.WriteString(HintStyle.Render(composerShortcutHint(contentWidth)))
	return MenuBoxStyle.Render(b.String())
}

func wrapVisualLine(line string, width int) []string {
	if width < 1 {
		return []string{line}
	}
	runes := []rune(line)
	var chunks []string
	for len(runes) > width {
		chunks = append(chunks, string(runes[:width]))
		runes = runes[width:]
	}
	if len(runes) > 0 || len(chunks) == 0 {
		chunks = append(chunks, string(runes))
	}
	return chunks
}

func renderLineWithCursorAndSelection(b *strings.Builder, line string, cursorCol int, selFrom, selTo int) {
	points := []int{0}
	if selFrom >= 0 && selFrom > 0 {
		points = append(points, selFrom)
	}
	if selTo >= 0 && selTo < len(line) {
		points = append(points, selTo)
	}
	if cursorCol > 0 && cursorCol < len(line) {
		points = append(points, cursorCol)
	}
	points = append(points, len(line))
	sort.Ints(points)

	unique := points[:1]
	for i := 1; i < len(points); i++ {
		if points[i] != unique[len(unique)-1] {
			unique = append(unique, points[i])
		}
	}

	cursorPlaced := false
	for j := 0; j < len(unique)-1; j++ {
		from := unique[j]
		to := unique[j+1]
		if from > to {
			break
		}

		baseStyle := ComposerBodyStyle
		inSelection := false
		if selFrom >= 0 && selTo >= 0 && from >= selFrom && from < selTo {
			baseStyle = ComposerSelectionStyle
			inSelection = true
		}

		if !cursorPlaced && cursorCol >= from && cursorCol < to {
			pos := cursorCol
			char := string(line[pos])
			cursorStyle := CursorStyle
			if inSelection {
				cursorStyle = CursorSelectionStyle
			}
			b.WriteString(baseStyle.Render(line[from:pos]))
			b.WriteString(cursorStyle.Render(char))
			b.WriteString(baseStyle.Render(line[pos+1 : to]))
			cursorPlaced = true
		} else {
			b.WriteString(baseStyle.Render(line[from:to]))
		}
	}

	if !cursorPlaced && cursorCol >= len(line) {
		b.WriteString(CursorStyle.Render(" "))
	}
}

func composerShortcutHint(width int) string {
	options := []string{
		"Ctrl+A select all  ·  Ctrl+C copy  ·  Ctrl+X cut  ·  Ctrl+V paste  ·  Alt+Enter new line  ·  Enter submit  ·  Esc cancel",
		"Ctrl+A all  ·  Ctrl+C/X/V  ·  Alt+Enter new line  ·  Enter submit  ·  Esc cancel",
		"Alt+Enter new line  ·  Enter submit  ·  Esc cancel",
		"Alt+Enter line  ·  Enter submit  ·  Esc cancel",
		"Alt+Enter line  ·  Enter submit  ·  Esc",
		"Alt+Enter  ·  Enter  ·  Esc",
	}
	if width <= 0 {
		return options[0]
	}
	for _, option := range options {
		if xansi.StringWidth(option) <= width {
			return option
		}
	}
	return truncateInputToWidth(options[len(options)-1], width)
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
