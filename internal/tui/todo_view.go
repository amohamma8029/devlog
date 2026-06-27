package tui

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/amo/devlog/internal/todo"
	tea "github.com/charmbracelet/bubbletea"
	xansi "github.com/charmbracelet/x/ansi"
)

const (
	todoPromptAdd       = "add"
	todoPromptEdit      = "edit"
	todoOverlayMinWidth = 48
	todoOverlayMaxWidth = 84
	todoOverlayMaxRows  = 18
)

func renderTodoOverView(m Model, base string) string {
	return overlayBlock(xansi.Strip(base), renderTodoView(m), m.Width, m.Height)
}

func renderTodoView(m Model) string {
	cardWidth := todoOverlayWidth(m.Width)
	contentWidth := cardWidth - TodoPanelStyle.GetHorizontalFrameSize()
	if contentWidth < 1 {
		contentWidth = 1
	}
	maxRows := todoOverlayBodyRows(m.Height)

	lines := buildTodoContentLines(m, contentWidth)
	if len(lines) > maxRows {
		start := clampTodoScrollOffset(m.TodoScrollOffset, len(lines), maxRows)
		end := start + maxRows
		if end > len(lines) {
			end = len(lines)
		}
		lines = lines[start:end]
	}
	lines = append(lines, renderTodoSupplementalLines(m, contentWidth)...)

	return TodoPanelStyle.Width(cardWidth - 2).Render(strings.Join(lines, "\n"))
}

func renderTodoSupplementalLines(m Model, width int) []string {
	var lines []string
	if m.TodoPromptOpen {
		lines = append(lines, "", renderTodoPrompt(m, width))
	}
	if m.TodoDeleteConfirm {
		lines = append(lines, "", renderTodoDeletePrompt(width))
	}
	if m.Palette != nil && m.Palette.Open {
		m.Palette.SetWidth(width)
		lines = append(lines, "", m.Palette.View())
	}
	if m.ErrorMessage != "" {
		line := ErrorBannerStyle.Width(width).Render(" ERROR: " + m.ErrorMessage)
		lines = append(lines, "", fitTodoLine(line, width))
	}
	if m.HandoffMsg != "" {
		lines = append(lines, "", fitTodoLine(HintStyle.Render(formatHandoffConfirmation(m.HandoffMsg)), width))
	}
	lines = append(lines, "", fitTodoLine(TodoMutedStyle.Render("↑/↓ select  a add  e edit  enter toggle  d delete  q close"), width))
	return lines
}

func renderTodoPrompt(m Model, width int) string {
	label := "Add todo"
	if m.TodoPromptMode == todoPromptEdit {
		label = "Edit todo"
	}

	input := m.TodoInput
	cursorVisible := true
	if m.Palette != nil {
		cursorVisible = m.Palette.CursorVisible
	}
	if cursorVisible {
		input += CursorStyle.Render(" ")
	}
	contentWidth := todoPromptContentWidth(width, TodoPromptStyle.GetHorizontalFrameSize())
	content := clampPreviewLine(label+": "+input, contentWidth)
	return TodoPromptStyle.Width(contentWidth).Render(content)
}

func renderTodoDeletePrompt(width int) string {
	contentWidth := todoPromptContentWidth(width, WarningPromptStyle.GetHorizontalFrameSize())
	content := clampPreviewLine("Delete selected todo? y/n", contentWidth)
	return WarningPromptStyle.Width(contentWidth).Render(content)
}

func renderTodoContent(m Model, height int) string {
	cardWidth := todoOverlayWidth(m.Width)
	contentWidth := cardWidth - TodoPanelStyle.GetHorizontalFrameSize()
	if contentWidth < 1 {
		contentWidth = 1
	}
	lines := buildTodoContentLines(m, contentWidth)
	maxRows := height
	if maxRows < 1 {
		maxRows = 1
	}
	if len(lines) > maxRows {
		start := clampTodoScrollOffset(m.TodoScrollOffset, len(lines), maxRows)
		end := start + maxRows
		if end > len(lines) {
			end = len(lines)
		}
		lines = lines[start:end]
	}
	return TodoPanelStyle.Width(cardWidth - 2).Render(strings.Join(lines, "\n"))
}

func buildTodoContentLines(m Model, width int) []string {
	openCount, doneCount := todoCounts(m.TodoItems)
	lines := renderTodoHeaderLines(m, width, openCount, doneCount)
	if !m.TodoLoaded {
		return append(lines, "", fitTodoLine(HintStyle.Render("Loading todos..."), width))
	}
	if len(m.TodoItems) == 0 {
		lines = append(lines, "")
		return append(lines, renderTodoEmptyState(width)...)
	}

	openStarted := false
	doneStarted := false
	for idx, item := range m.TodoItems {
		if item.Status == todo.StatusOpen && !openStarted {
			lines = append(lines, "", renderTodoSectionHeader("Open", openCount, width))
			openStarted = true
		}
		if item.Status == todo.StatusDone && !doneStarted {
			if openStarted {
				lines = append(lines, "")
			}
			lines = append(lines, renderTodoSectionHeader("Completed", doneCount, width))
			doneStarted = true
		}

		lines = append(lines, renderTodoRow(item, idx, idx == m.TodoSelected, width))
	}

	return lines
}

func renderTodoHeaderLines(m Model, width, openCount, doneCount int) []string {
	title := TodoHeaderStyle.Render("Todo List")
	summary := TodoAccentStyle.Render(fmt.Sprintf("%d open / %d completed", openCount, doneCount))
	scope := TodoMutedStyle.Render("Repo-wide follow-up work")
	if len(m.TodoItems) > 0 && m.TodoSelected >= 0 {
		scope = TodoMutedStyle.Render(fmt.Sprintf("Repo-wide follow-up work  selected %d/%d", m.TodoSelected+1, len(m.TodoItems)))
	}
	return []string{
		alignTodoLine(title, summary, width),
		fitTodoLine(scope, width),
	}
}

func renderTodoSectionHeader(label string, count, width int) string {
	title := TodoAccentStyle.Render(label)
	countText := TodoMutedStyle.Render(fmt.Sprintf("%d", count))
	return fitTodoLine(title+" "+countText, width)
}

func renderTodoRow(item todo.Item, idx int, selected bool, width int) string {
	rail := " "
	if selected {
		rail = TodoAccentStyle.Render(">")
	}
	number := TodoNumberStyle.Render(strconv.Itoa(idx + 1))
	checkbox := todoCheckbox(item)
	text := oneLineTodoText(item.Text)
	if item.Status == todo.StatusDone {
		text = TodoCompletedTextStyle.Render(text)
	} else {
		text = EventStyle.Render(text)
	}

	prefix := rail + " " + number + "  " + checkbox + "  "
	available := width - xansi.StringWidth(prefix)
	if available < 1 {
		available = 1
	}
	line := fitTodoLine(prefix+clampPreviewLine(text, available), width)
	if selected {
		return TodoSelectedRowStyle.Width(width).Render(line)
	}
	return line
}

func renderTodoEmptyState(width int) []string {
	return []string{
		centerTodoLine(TodoEmptyTitleStyle.Render("No todos yet"), width),
		centerTodoLine(TodoMutedStyle.Render("Press a to add your first repo-wide follow-up."), width),
		centerTodoLine(TodoActionStyle.Render("Press a"), width),
	}
}

func alignTodoLine(left, right string, width int) string {
	if width < 1 {
		return ""
	}
	if right == "" {
		return fitTodoLine(left, width)
	}
	if xansi.StringWidth(right) >= width {
		return fitTodoLine(right, width)
	}
	availableLeft := width - xansi.StringWidth(right) - 2
	if availableLeft < 1 {
		availableLeft = 1
	}
	left = clampPreviewLine(left, availableLeft)
	spaces := width - xansi.StringWidth(left) - xansi.StringWidth(right)
	if spaces < 1 {
		spaces = 1
	}
	return fitTodoLine(left+strings.Repeat(" ", spaces)+right, width)
}

func centerTodoLine(line string, width int) string {
	line = clampPreviewLine(line, width)
	padding := width - xansi.StringWidth(line)
	if padding <= 0 {
		return line
	}
	left := padding / 2
	return fitTodoLine(strings.Repeat(" ", left)+line, width)
}

func fitTodoLine(line string, width int) string {
	line = clampPreviewLine(line, width)
	padding := width - xansi.StringWidth(line)
	if padding > 0 {
		line += strings.Repeat(" ", padding)
	}
	return line
}

func todoCounts(items []todo.Item) (openCount, doneCount int) {
	for _, item := range items {
		if item.Status == todo.StatusDone {
			doneCount++
		} else {
			openCount++
		}
	}
	return openCount, doneCount
}

func todoPromptContentWidth(width, frame int) int {
	if width <= 0 {
		width = 80
	}
	contentWidth := width - frame
	if contentWidth < 1 {
		contentWidth = 1
	}
	return contentWidth
}

func todoOverlayWidth(termWidth int) int {
	if termWidth <= 0 {
		return todoOverlayMaxWidth
	}
	width := termWidth - 8
	if width > todoOverlayMaxWidth {
		width = todoOverlayMaxWidth
	}
	if width < todoOverlayMinWidth {
		width = termWidth - 2
		if width < 20 {
			width = 20
		}
	}
	return width
}

func todoOverlayBodyRows(termHeight int) int {
	if termHeight <= 0 {
		return todoOverlayMaxRows
	}
	rows := termHeight - 10
	if rows > todoOverlayMaxRows {
		rows = todoOverlayMaxRows
	}
	if rows < 8 {
		rows = 8
	}
	return rows
}

func countTodoContentLines(m Model) int {
	return countLines(renderTodoContent(m, m.Height))
}

func (m Model) handleTodoCommand(args string) (tea.Model, tea.Cmd) {
	args = strings.TrimSpace(args)
	if args == "" {
		return m.openTodoView()
	}

	command, rest, _ := strings.Cut(args, " ")
	command = strings.ToLower(strings.TrimSpace(command))
	rest = strings.TrimSpace(rest)

	switch command {
	case "add":
		if rest == "" {
			m.ErrorMessage = "Usage: /todo add <text>"
			return m, nil
		}
		if !m.TodoOpen {
			m.TodoOpen = true
			m.TodoLoaded = false
		}
		return m, m.todoAddCmd(rest)

	case "edit":
		ref, text, ok := strings.Cut(rest, " ")
		if !ok || strings.TrimSpace(ref) == "" || strings.TrimSpace(text) == "" {
			m.ErrorMessage = "Usage: /todo edit <number> <text>"
			return m, nil
		}
		idx, err := m.todoIndexByRef(ref)
		if err != nil {
			m.ErrorMessage = err.Error()
			return m, nil
		}
		item := m.TodoItems[idx]
		return m, m.todoMutationCmd(idx, item.ID, "Edited todo", func(store *todo.Store) (string, error) {
			return item.ID, store.UpdateText(item.ID, text)
		})

	case "done", "reopen":
		if rest == "" {
			m.ErrorMessage = "Usage: /todo " + command + " <number>"
			return m, nil
		}
		idx, err := m.todoIndexByRef(rest)
		if err != nil {
			m.ErrorMessage = err.Error()
			return m, nil
		}
		item := m.TodoItems[idx]
		message := "Completed todo"
		mutate := func(store *todo.Store) (string, error) {
			return item.ID, store.Complete(item.ID)
		}
		if command == "reopen" {
			message = "Reopened todo"
			mutate = func(store *todo.Store) (string, error) {
				return item.ID, store.Reopen(item.ID)
			}
		}
		return m, m.todoMutationCmd(idx, item.ID, message, mutate)

	case "delete":
		if rest == "" {
			m.ErrorMessage = "Usage: /todo delete <number>"
			return m, nil
		}
		idx, err := m.todoIndexByRef(rest)
		if err != nil {
			m.ErrorMessage = err.Error()
			return m, nil
		}
		m.TodoSelected = idx
		m.TodoDeleteConfirm = true
		ensureTodoSelectionVisible(&m)
		return m, nil

	default:
		m.ErrorMessage = "Unknown todo command: " + command
		return m, nil
	}
}

func (m Model) openTodoView() (tea.Model, tea.Cmd) {
	m.TodoOpen = true
	m.TodoPromptOpen = false
	m.TodoDeleteConfirm = false
	m.TodoLoaded = false
	return m, m.loadTodoItemsCmd(m.TodoSelected, "", "")
}

func (m Model) closeTodoView() (tea.Model, tea.Cmd) {
	if m.TodoPromptOpen {
		m.clearTodoPrompt()
		return m, nil
	}
	if m.TodoDeleteConfirm {
		m.TodoDeleteConfirm = false
		return m, nil
	}
	m.TodoOpen = false
	m.clearTodoPrompt()
	m.TodoDeleteConfirm = false
	return m, nil
}

func (m Model) loadTodoItemsCmd(selection int, selectedID, message string) tea.Cmd {
	root := m.Root
	return func() tea.Msg {
		store, err := todo.NewStore(root)
		if err != nil {
			return TodoLoadedMsg{Error: err, Selection: selection, SelectedID: selectedID}
		}
		items, err := store.List(todo.AllFilter())
		return TodoLoadedMsg{Items: items, Error: err, Selection: selection, SelectedID: selectedID, Message: message}
	}
}

func (m *Model) applyTodoLoaded(msg TodoLoadedMsg) {
	if msg.Error != nil {
		m.ErrorMessage = msg.Error.Error()
		return
	}

	m.TodoItems = orderedTodoViewItems(msg.Items)
	m.TodoLoaded = true
	m.TodoSelected = msg.Selection
	if msg.SelectedID != "" {
		for i, item := range m.TodoItems {
			if item.ID == msg.SelectedID {
				m.TodoSelected = i
				break
			}
		}
	}
	clampTodoSelection(m)
	ensureTodoSelectionVisible(m)
	if msg.Message != "" {
		m.HandoffMsg = msg.Message
	}
}

func (m Model) todoKeyHandler(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "up":
		if m.TodoSelected > 0 {
			m.TodoSelected--
			ensureTodoSelectionVisible(&m)
		}
		return m, nil

	case "down":
		if m.TodoSelected < len(m.TodoItems)-1 {
			m.TodoSelected++
			ensureTodoSelectionVisible(&m)
		}
		return m, nil

	case "a":
		return m.openTodoPrompt(todoPromptAdd)

	case "e":
		if _, ok := m.selectedTodoItem(); !ok {
			m.ErrorMessage = "No todo selected"
			return m, nil
		}
		return m.openTodoPrompt(todoPromptEdit)

	case "enter", " ", "space":
		return m.toggleSelectedTodo()

	case "d", "x":
		if _, ok := m.selectedTodoItem(); !ok {
			m.ErrorMessage = "No todo selected"
			return m, nil
		}
		m.TodoDeleteConfirm = true
		return m, nil

	case "/":
		if m.Palette != nil {
			m.Palette.OpenPalette()
			m.Palette.Input = "/"
			m.Palette.InputCursorPos = 1
			return m, cursorTickCmd()
		}
	}

	return m, nil
}

func (m Model) todoPromptKeyHandler(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.clearTodoPrompt()
		return m, nil

	case "enter":
		return m.submitTodoPrompt()

	case "backspace":
		if len(m.TodoInput) > 0 {
			m.TodoInput = m.TodoInput[:len(m.TodoInput)-1]
		}
		return m, nil
	}

	if len(msg.Runes) > 0 {
		m.TodoInput += string(msg.Runes)
	}
	return m, nil
}

func (m Model) todoDeleteConfirmKeyHandler(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "y":
		return m.deleteSelectedTodo()
	case "n", "esc":
		m.TodoDeleteConfirm = false
		return m, nil
	}
	return m, nil
}

func (m Model) openTodoPrompt(mode string) (tea.Model, tea.Cmd) {
	m.TodoPromptOpen = true
	m.TodoPromptMode = mode
	m.TodoInput = ""
	m.TodoEditingID = ""
	m.TodoDeleteConfirm = false
	if mode == todoPromptEdit {
		item, ok := m.selectedTodoItem()
		if !ok {
			m.clearTodoPrompt()
			m.ErrorMessage = "No todo selected"
			return m, nil
		}
		m.TodoInput = item.Text
		m.TodoEditingID = item.ID
	}
	if m.Palette != nil {
		m.Palette.CursorVisible = true
	}
	return m, cursorTickCmd()
}

func (m Model) submitTodoPrompt() (tea.Model, tea.Cmd) {
	text := strings.TrimSpace(m.TodoInput)
	mode := m.TodoPromptMode
	editingID := m.TodoEditingID
	selection := m.TodoSelected
	m.clearTodoPrompt()
	if text == "" {
		m.ErrorMessage = "Todo text is required"
		return m, nil
	}

	if mode == todoPromptAdd {
		return m, m.todoAddCmd(text)
	}
	if editingID == "" {
		m.ErrorMessage = "No todo selected"
		return m, nil
	}
	return m, m.todoMutationCmd(selection, editingID, "Edited todo", func(store *todo.Store) (string, error) {
		return editingID, store.UpdateText(editingID, text)
	})
}

func (m *Model) clearTodoPrompt() {
	m.TodoPromptOpen = false
	m.TodoPromptMode = ""
	m.TodoInput = ""
	m.TodoEditingID = ""
}

func (m Model) todoAddCmd(text string) tea.Cmd {
	sessionID, branch := m.todoAttribution()
	selection := m.TodoSelected
	return m.todoMutationCmd(selection, "", "Added todo", func(store *todo.Store) (string, error) {
		item, err := store.Add(todo.AddInput{Text: text, SessionID: sessionID, Branch: branch})
		if err != nil {
			return "", err
		}
		return item.ID, nil
	})
}

func (m Model) toggleSelectedTodo() (tea.Model, tea.Cmd) {
	item, ok := m.selectedTodoItem()
	if !ok {
		m.ErrorMessage = "No todo selected"
		return m, nil
	}
	message := "Completed todo"
	mutate := func(store *todo.Store) (string, error) {
		return item.ID, store.Complete(item.ID)
	}
	if item.Status == todo.StatusDone {
		message = "Reopened todo"
		mutate = func(store *todo.Store) (string, error) {
			return item.ID, store.Reopen(item.ID)
		}
	}
	return m, m.todoMutationCmd(m.TodoSelected, item.ID, message, mutate)
}

func (m Model) deleteSelectedTodo() (tea.Model, tea.Cmd) {
	item, ok := m.selectedTodoItem()
	if !ok {
		m.TodoDeleteConfirm = false
		m.ErrorMessage = "No todo selected"
		return m, nil
	}
	selection := m.TodoSelected
	m.TodoDeleteConfirm = false
	return m, m.todoMutationCmd(selection, "", "Deleted todo", func(store *todo.Store) (string, error) {
		return "", store.Delete(item.ID)
	})
}

func (m Model) todoMutationCmd(selection int, selectedID, message string, mutate func(*todo.Store) (string, error)) tea.Cmd {
	root := m.Root
	return func() tea.Msg {
		store, err := todo.NewStore(root)
		if err != nil {
			return CommandErrorMsg{Error: err}
		}
		nextSelectedID, err := mutate(store)
		if err != nil {
			return CommandErrorMsg{Error: err}
		}
		if nextSelectedID != "" {
			selectedID = nextSelectedID
		}
		items, err := store.List(todo.AllFilter())
		return TodoLoadedMsg{Items: items, Error: err, Selection: selection, SelectedID: selectedID, Message: message}
	}
}

func (m Model) selectedTodoItem() (todo.Item, bool) {
	if m.TodoSelected < 0 || m.TodoSelected >= len(m.TodoItems) {
		return todo.Item{}, false
	}
	return m.TodoItems[m.TodoSelected], true
}

func (m Model) todoIndexByRef(ref string) (int, error) {
	if !m.TodoOpen || !m.TodoLoaded {
		return -1, fmt.Errorf("open /todo before using todo numbers")
	}
	n, err := strconv.Atoi(strings.TrimSpace(ref))
	if err != nil || n < 1 {
		return -1, fmt.Errorf("todo number %q is invalid", ref)
	}
	if n > len(m.TodoItems) {
		return -1, fmt.Errorf("todo number %d out of range (list has %d items)", n, len(m.TodoItems))
	}
	return n - 1, nil
}

func (m Model) todoAttribution() (string, string) {
	if m.ActiveSession == nil || m.ActiveSession.Closed {
		return "", ""
	}
	return m.ActiveSession.ID, m.ActiveSession.Branch
}

func (m Model) handleTodoMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	switch ParseMouseEvent(msg) {
	case MouseScrollUp:
		if m.TodoSelected > 0 {
			m.TodoSelected--
			ensureTodoSelectionVisible(&m)
		}
	case MouseScrollDown:
		if m.TodoSelected < len(m.TodoItems)-1 {
			m.TodoSelected++
			ensureTodoSelectionVisible(&m)
		}
	}
	return m, nil
}

func orderedTodoViewItems(items []todo.Item) []todo.Item {
	ordered := make([]todo.Item, 0, len(items))
	for _, item := range items {
		if item.Status == todo.StatusOpen {
			ordered = append(ordered, item)
		}
	}
	for _, item := range items {
		if item.Status == todo.StatusDone {
			ordered = append(ordered, item)
		}
	}
	return ordered
}

func todoCheckbox(item todo.Item) string {
	if item.Status == todo.StatusDone {
		return TodoDoneCheckboxStyle.Render("[x]")
	}
	return TodoOpenCheckboxStyle.Render("[ ]")
}

func oneLineTodoText(text string) string {
	fields := strings.Fields(text)
	if len(fields) == 0 {
		return "(empty)"
	}
	return strings.Join(fields, " ")
}

func clampTodoSelection(m *Model) {
	if len(m.TodoItems) == 0 {
		m.TodoSelected = -1
		m.TodoScrollOffset = 0
		return
	}
	if m.TodoSelected < 0 {
		m.TodoSelected = 0
	}
	if m.TodoSelected >= len(m.TodoItems) {
		m.TodoSelected = len(m.TodoItems) - 1
	}
}

func clampTodoModelScroll(m *Model) {
	clampTodoSelection(m)
	ensureTodoSelectionVisible(m)
}

func ensureTodoSelectionVisible(m *Model) {
	if len(m.TodoItems) == 0 {
		m.TodoScrollOffset = 0
		return
	}
	lines, itemLines := todoLineIndexes(*m)
	if m.TodoSelected < 0 || m.TodoSelected >= len(itemLines) {
		return
	}
	visible := todoVisibleLineCount(*m)
	if visible < 1 {
		visible = 1
	}
	selectedLine := itemLines[m.TodoSelected]
	if selectedLine < m.TodoScrollOffset {
		m.TodoScrollOffset = selectedLine
	} else if selectedLine >= m.TodoScrollOffset+visible {
		m.TodoScrollOffset = selectedLine - visible + 1
	}
	m.TodoScrollOffset = clampTodoScrollOffset(m.TodoScrollOffset, len(lines), visible)
}

func todoLineIndexes(m Model) ([]string, []int) {
	lines := buildTodoContentLines(m, max(20, m.Width-4))
	var indexes []int
	for i, line := range lines {
		trimmed := strings.TrimSpace(xansi.Strip(line))
		if strings.Contains(trimmed, "[ ]") || strings.Contains(trimmed, "[x]") {
			indexes = append(indexes, i)
		}
	}
	return lines, indexes
}

func todoVisibleLineCount(m Model) int {
	return todoOverlayBodyRows(m.Height)
}

func clampTodoScrollOffset(offset, lineCount, visible int) int {
	if offset < 0 {
		return 0
	}
	maxOffset := lineCount - visible
	if maxOffset < 0 {
		maxOffset = 0
	}
	if offset > maxOffset {
		return maxOffset
	}
	return offset
}

func cursorTickCmd() tea.Cmd {
	return tea.Tick(500*time.Millisecond, func(t time.Time) tea.Msg {
		return CursorTickMsg{}
	})
}
