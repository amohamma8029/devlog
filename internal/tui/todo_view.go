package tui

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/amo/devlog/internal/handoff"
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
	lines := renderTodoLines(m, contentWidth, todoOverlayBodyRows(m.Height))

	return TodoPanelStyle.Width(cardWidth - 2).Render(strings.Join(lines, "\n"))
}

func renderTodoLines(m Model, contentWidth, maxRows int) []string {
	openCount, doneCount := todoCounts(m.TodoItems)
	header := renderTodoHeaderLines(m, contentWidth, openCount, doneCount)
	body := buildTodoBodyLines(m, contentWidth)
	supplemental := renderTodoSupplementalLines(m, contentWidth)
	bodyRows := todoScrollableBodyRows(maxRows, len(header), len(supplemental))
	if len(body) > bodyRows {
		start := clampTodoScrollOffset(m.TodoScrollOffset, len(body), bodyRows)
		end := start + bodyRows
		if end > len(body) {
			end = len(body)
		}
		body = body[start:end]
	}
	lines := append(header, body...)
	lines = append(lines, supplemental...)
	return lines
}

func todoScrollableBodyRows(maxRows, headerRows, supplementalRows int) int {
	bodyRows := maxRows - headerRows - supplementalRows
	if bodyRows < 3 {
		return 3
	}
	return bodyRows
}

func renderTodoSupplementalLines(m Model, width int) []string {
	var lines []string
	if m.TodoPromptOpen {
		lines = append(lines, "", renderTodoPrompt(m, width))
	}
	if m.TodoDeleteConfirm {
		lines = append(lines, "", renderTodoDeletePrompt(width))
	}
	if m.TodoPruneConfirm {
		lines = append(lines, "", renderTodoPrunePrompt(width, m.TodoPruneCount))
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

func renderTodoPrunePrompt(width, count int) string {
	contentWidth := todoPromptContentWidth(width, WarningPromptStyle.GetHorizontalFrameSize())
	content := clampPreviewLine("Prune "+completedTodoCount(count)+"? Open todos will be kept. y/n", contentWidth)
	return WarningPromptStyle.Width(contentWidth).Render(content)
}

func renderTodoContent(m Model, height int) string {
	cardWidth := todoOverlayWidth(m.Width)
	contentWidth := cardWidth - TodoPanelStyle.GetHorizontalFrameSize()
	if contentWidth < 1 {
		contentWidth = 1
	}
	maxRows := height
	if maxRows < 1 {
		maxRows = 1
	}
	lines := renderTodoLines(m, contentWidth, maxRows)
	return TodoPanelStyle.Width(cardWidth - 2).Render(strings.Join(lines, "\n"))
}

func buildTodoContentLines(m Model, width int) []string {
	openCount, doneCount := todoCounts(m.TodoItems)
	lines := renderTodoHeaderLines(m, width, openCount, doneCount)
	return append(lines, buildTodoBodyLines(m, width)...)
}

func buildTodoBodyLines(m Model, width int) []string {
	if !m.TodoLoaded {
		return []string{"", fitTodoLine(HintStyle.Render("Loading todos..."), width)}
	}
	if len(m.TodoItems) == 0 {
		return append([]string{""}, renderTodoEmptyState(width)...)
	}

	openCount, doneCount := todoCounts(m.TodoItems)
	numberWidth := len(strconv.Itoa(len(m.TodoItems)))
	var lines []string
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

		lines = append(lines, renderTodoRow(item, idx, idx == m.TodoSelected, width, numberWidth))
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

func renderTodoRow(item todo.Item, idx int, selected bool, width, numberWidth int) string {
	rail := " "
	if selected {
		rail = TodoAccentStyle.Render(">")
	}
	number := TodoNumberStyle.Render(fmt.Sprintf("%*d", numberWidth, idx+1))
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

func completedTodoCount(count int) string {
	if count == 1 {
		return "1 completed todo"
	}
	return fmt.Sprintf("%d completed todos", count)
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

func todoOverlayContentWidth(termWidth int) int {
	contentWidth := todoOverlayWidth(termWidth) - TodoPanelStyle.GetHorizontalFrameSize()
	if contentWidth < 1 {
		return 1
	}
	return contentWidth
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
		m.TodoPruneConfirm = false
		ensureTodoSelectionVisible(&m)
		return m, nil

	case "prune":
		if rest != "" {
			m.ErrorMessage = "Usage: /todo prune"
			return m, nil
		}
		m.TodoOpen = true
		m.TodoLoaded = false
		m.TodoPromptOpen = false
		m.TodoDeleteConfirm = false
		m.TodoPruneConfirm = false
		m.TodoPruneCount = 0
		m.HandoffMsg = ""
		return m, m.todoPrunePromptCmd()

	default:
		m.ErrorMessage = "Unknown todo command: " + command
		return m, nil
	}
}

func (m Model) openTodoView() (tea.Model, tea.Cmd) {
	m.TodoOpen = true
	m.TodoPromptOpen = false
	m.TodoDeleteConfirm = false
	m.TodoPruneConfirm = false
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
	if m.TodoPruneConfirm {
		m.TodoPruneConfirm = false
		m.TodoPruneCount = 0
		return m, nil
	}
	m.TodoOpen = false
	m.clearTodoPrompt()
	m.TodoDeleteConfirm = false
	m.TodoPruneConfirm = false
	m.TodoPruneCount = 0
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

// loadHandoffTodos returns all todos relevant to the given session/branch for
// inclusion in a handoff artifact. A missing todo file is treated as "no
// todos" rather than an error so the handoff preview never fails solely
// because the todo log has not been initialised yet.
func loadHandoffTodos(root, sessionID, branch string) ([]todo.Item, error) {
	store, err := todo.NewStore(root)
	if err != nil {
		return nil, err
	}
	items, err := store.List(todo.Filter{
		IncludeOpen:     true,
		IncludeDone:     true,
		SessionID:       sessionID,
		Branch:          branch,
		MatchSessionAny: sessionID == "",
		MatchBranchAny:  branch == "",
	})
	if err != nil {
		return nil, err
	}
	ordered := make([]todo.Item, 0, len(items))
	for _, item := range items {
		if item.Status == todo.StatusDone {
			ordered = append(ordered, item)
		}
	}
	for _, item := range items {
		if item.Status == todo.StatusOpen {
			ordered = append(ordered, item)
		}
	}
	return ordered, nil
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
	m.refreshHandoffTodosFromItems(msg.Items)
}

func (m *Model) refreshHandoffTodosFromItems(items []todo.Item) {
	if m.CurrentView != HandoffPreview || m.HandoffContent == "" {
		return
	}

	sessionID, branch := "", ""
	if m.ActiveSession != nil {
		sessionID = m.ActiveSession.ID
		branch = m.ActiveSession.Branch
	}

	section := handoff.FormatTodosSection(relevantHandoffTodos(items, sessionID, branch))
	nextContent := replaceHandoffTodoListSection(m.HandoffContent, section)
	if nextContent == m.HandoffContent {
		return
	}

	m.HandoffContent = nextContent
	m.refreshHandoffBodyCache()
	resetSearchSelection(&m.Search)
	clampHandoffModelScroll(m)
}

func relevantHandoffTodos(items []todo.Item, sessionID, branch string) []todo.Item {
	filter := todo.Filter{
		IncludeOpen:     true,
		IncludeDone:     true,
		SessionID:       sessionID,
		Branch:          branch,
		MatchSessionAny: sessionID == "",
		MatchBranchAny:  branch == "",
	}
	ordered := make([]todo.Item, 0, len(items))
	for _, item := range items {
		if item.Status == todo.StatusDone && filter.Matches(item) {
			ordered = append(ordered, item)
		}
	}
	for _, item := range items {
		if item.Status != todo.StatusDone && filter.Matches(item) {
			ordered = append(ordered, item)
		}
	}
	return ordered
}

func replaceHandoffTodoListSection(content, section string) string {
	lines := strings.Split(content, "\n")
	start := handoffSectionLine(lines, "## Todo List")
	if start >= 0 {
		end := nextHandoffSectionLine(lines, start+1)
		return replaceLineRange(lines, start, end, sectionLines(section))
	}
	if strings.TrimSpace(section) == "" {
		return content
	}

	insertAt := handoffSectionLine(lines, "## Changes")
	if insertAt < 0 {
		insertAt = len(lines)
	}
	return replaceLineRange(lines, insertAt, insertAt, sectionLines(section))
}

func handoffSectionLine(lines []string, heading string) int {
	for i, line := range lines {
		if line == heading {
			return i
		}
	}
	return -1
}

func nextHandoffSectionLine(lines []string, start int) int {
	for i := start; i < len(lines); i++ {
		if strings.HasPrefix(lines[i], "## ") {
			return i
		}
	}
	return len(lines)
}

func sectionLines(section string) []string {
	if strings.TrimSpace(section) == "" {
		return nil
	}
	return strings.Split(section, "\n")
}

func replaceLineRange(lines []string, start, end int, replacement []string) string {
	if start < 0 {
		start = 0
	}
	if end < start {
		end = start
	}
	if end > len(lines) {
		end = len(lines)
	}

	result := make([]string, 0, len(lines)-(end-start)+len(replacement)+2)
	result = append(result, lines[:start]...)
	if len(replacement) > 0 {
		if len(result) > 0 && strings.TrimSpace(result[len(result)-1]) != "" {
			result = append(result, "")
		}
		result = append(result, replacement...)
		if end < len(lines) && strings.TrimSpace(lines[end]) != "" && strings.TrimSpace(result[len(result)-1]) != "" {
			result = append(result, "")
		}
	}
	result = append(result, lines[end:]...)
	return strings.Join(result, "\n")
}

func (m *Model) applyTodoPrunePrompt(msg TodoPrunePromptMsg) {
	if msg.Error != nil {
		m.ErrorMessage = msg.Error.Error()
		return
	}

	m.TodoItems = orderedTodoViewItems(msg.Items)
	m.TodoLoaded = true
	m.TodoPromptOpen = false
	m.TodoDeleteConfirm = false
	m.TodoPruneConfirm = false
	m.TodoPruneCount = 0
	clampTodoSelection(m)
	ensureTodoSelectionVisible(m)
	if msg.CompletedCount == 0 {
		m.HandoffMsg = "No completed todos to prune."
		return
	}
	m.TodoPruneConfirm = true
	m.TodoPruneCount = msg.CompletedCount
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
		m.TodoPruneConfirm = false
		return m, nil

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

func (m Model) todoPruneConfirmKeyHandler(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "y":
		return m.pruneCompletedTodos()
	case "n", "esc":
		m.TodoPruneConfirm = false
		m.TodoPruneCount = 0
		m.HandoffMsg = "Prune cancelled."
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
	m.TodoPruneConfirm = false
	m.TodoPruneCount = 0
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

func (m Model) pruneCompletedTodos() (tea.Model, tea.Cmd) {
	selection := m.TodoSelected
	selectedID := ""
	if item, ok := m.selectedTodoItem(); ok && item.Status == todo.StatusOpen {
		selectedID = item.ID
	}
	m.TodoPruneConfirm = false
	m.TodoPruneCount = 0
	return m, m.todoPruneCmd(selection, selectedID)
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

func (m Model) todoPrunePromptCmd() tea.Cmd {
	root := m.Root
	return func() tea.Msg {
		store, err := todo.NewStore(root)
		if err != nil {
			return TodoPrunePromptMsg{Error: err}
		}
		items, err := store.List(todo.AllFilter())
		if err != nil {
			return TodoPrunePromptMsg{Error: err}
		}
		return TodoPrunePromptMsg{Items: items, CompletedCount: countCompletedTodos(items)}
	}
}

func (m Model) todoPruneCmd(selection int, selectedID string) tea.Cmd {
	root := m.Root
	return func() tea.Msg {
		store, err := todo.NewStore(root)
		if err != nil {
			return CommandErrorMsg{Error: err}
		}
		removed, err := store.PruneCompleted()
		if err != nil {
			return CommandErrorMsg{Error: err}
		}
		items, err := store.List(todo.AllFilter())
		return TodoLoadedMsg{Items: items, Error: err, Selection: selection, SelectedID: selectedID, Message: todoPruneMessage(removed)}
	}
}

func countCompletedTodos(items []todo.Item) int {
	count := 0
	for _, item := range items {
		if item.Status == todo.StatusDone {
			count++
		}
	}
	return count
}

func todoPruneMessage(removed int) string {
	if removed == 0 {
		return "No completed todos to prune."
	}
	return "Pruned " + completedTodoCount(removed) + "."
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
	lines := buildTodoBodyLines(m, todoOverlayContentWidth(m.Width))
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
	contentWidth := todoOverlayContentWidth(m.Width)
	maxRows := todoOverlayBodyRows(m.Height)
	openCount, doneCount := todoCounts(m.TodoItems)
	headerRows := len(renderTodoHeaderLines(m, contentWidth, openCount, doneCount))
	supplementRows := len(renderTodoSupplementalLines(m, contentWidth))
	return todoScrollableBodyRows(maxRows, headerRows, supplementRows)
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
