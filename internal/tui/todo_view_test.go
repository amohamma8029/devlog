package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/amo/devlog/internal/store"
	"github.com/amo/devlog/internal/todo"
	tea "github.com/charmbracelet/bubbletea"
	xansi "github.com/charmbracelet/x/ansi"
)

func newTodoViewTestModel(t *testing.T) (Model, *todo.Store, string) {
	t.Helper()
	s, root := newTestStore(t)
	todoStore, err := todo.NewStore(root)
	if err != nil {
		t.Fatalf("todo.NewStore failed: %v", err)
	}
	m := NewModel(s, root)
	m.Width = 80
	m.Height = 24
	m.CurrentView = ActiveSession
	m.TodoOpen = true
	return m, todoStore, root
}

func addTodoViewTestItem(t *testing.T, store *todo.Store, text string) todo.Item {
	t.Helper()
	item, err := store.Add(todo.AddInput{Text: text})
	if err != nil {
		t.Fatalf("todo add failed: %v", err)
	}
	return item
}

func applyTodoViewCommand(t *testing.T, m Model, cmd tea.Cmd) Model {
	t.Helper()
	if cmd == nil {
		t.Fatal("expected command")
	}
	msg := cmd()
	if batch, ok := msg.(tea.BatchMsg); ok {
		updated := m
		for _, batched := range batch {
			if batched == nil {
				continue
			}
			updated = applyTodoViewCommand(t, updated, batched)
		}
		return updated
	}
	updatedModel, _ := m.Update(msg)
	updated, ok := updatedModel.(Model)
	if !ok {
		t.Fatalf("expected Model after command result, got %T", updatedModel)
	}
	return updated
}

func pressTodoViewKey(t *testing.T, m Model, msg tea.KeyMsg) (Model, tea.Cmd) {
	t.Helper()
	updatedModel, cmd := m.Update(msg)
	updated, ok := updatedModel.(Model)
	if !ok {
		t.Fatalf("expected Model after key, got %T", updatedModel)
	}
	return updated, cmd
}

func TestTodoCommandOpensDedicatedView(t *testing.T) {
	m, _, _ := newTodoViewTestModel(t)
	m.CurrentView = ActiveSession
	m.ActiveSession = testActiveSession()
	m.TodoOpen = false

	updatedModel, cmd := m.Update(CommandExecutedMsg{Input: "/todo"})
	updated, ok := updatedModel.(Model)
	if !ok {
		t.Fatalf("expected Model after /todo, got %T", updatedModel)
	}
	if updated.CurrentView != ActiveSession {
		t.Fatalf("CurrentView after /todo = %v, want underlying ActiveSession", updated.CurrentView)
	}
	if !updated.TodoOpen {
		t.Fatal("/todo should open the todo overlay")
	}
	updated = applyTodoViewCommand(t, updated, cmd)
	if !updated.TodoLoaded {
		t.Fatal("/todo should load todo state")
	}
	if !strings.Contains(xansi.Strip(updated.View()), "Todo List") {
		t.Fatalf("todo view should render title, got:\n%s", updated.View())
	}
}

func TestRenderTodoViewGroupsCheckboxesAndHidesIDs(t *testing.T) {
	m, todoStore, _ := newTodoViewTestModel(t)
	open := addTodoViewTestItem(t, todoStore, "open todo")
	done := addTodoViewTestItem(t, todoStore, "done todo")
	if err := todoStore.Complete(done.ID); err != nil {
		t.Fatalf("complete failed: %v", err)
	}

	loaded := applyTodoViewCommand(t, m, m.loadTodoItemsCmd(0, "", ""))
	view := xansi.Strip(renderTodoView(loaded))

	for _, want := range []string{"Todo List", "1 open / 1 completed", "Repo-wide follow-up work", "Open 1", "Completed 1", "1  [ ]  open todo", "2  [x]  done todo"} {
		if !strings.Contains(view, want) {
			t.Fatalf("todo view missing %q:\n%s", want, view)
		}
	}
	if strings.Contains(view, open.ID) || strings.Contains(view, done.ID) {
		t.Fatalf("todo view should hide internal IDs, got:\n%s", view)
	}
}

func TestRenderTodoViewUsesCappedOverlayAndStyledCheckboxes(t *testing.T) {
	m, todoStore, _ := newTodoViewTestModel(t)
	m.Width = 96
	addTodoViewTestItem(t, todoStore, "open todo")
	done := addTodoViewTestItem(t, todoStore, "done todo")
	if err := todoStore.Complete(done.ID); err != nil {
		t.Fatalf("complete failed: %v", err)
	}
	m = applyTodoViewCommand(t, m, m.loadTodoItemsCmd(0, "", ""))

	rendered := renderTodoView(m)
	lines := strings.Split(rendered, "\n")
	wantWidth := todoOverlayWidth(m.Width)
	if got := xansi.StringWidth(lines[0]); got != wantWidth {
		t.Fatalf("todo panel width = %d, want capped overlay width %d:\n%s", got, wantWidth, rendered)
	}
	if wantWidth >= m.Width {
		t.Fatalf("overlay should be narrower than wide terminal, got overlay=%d terminal=%d", wantWidth, m.Width)
	}
	if !strings.Contains(rendered, TodoOpenCheckboxStyle.Render("[ ]")) {
		t.Fatalf("open checkbox should use TodoOpenCheckboxStyle, got:\n%s", rendered)
	}
	if !strings.Contains(rendered, TodoDoneCheckboxStyle.Render("[x]")) {
		t.Fatalf("done checkbox should use TodoDoneCheckboxStyle, got:\n%s", rendered)
	}
	stripped := xansi.Strip(rendered)
	if !strings.Contains(stripped, "a add") || !strings.Contains(stripped, "enter toggle") || !strings.Contains(stripped, "q close") {
		t.Fatalf("todo card should include compact footer action hints, got:\n%s", stripped)
	}
	firstLines := strings.Join(strings.Split(stripped, "\n")[:3], "\n")
	if strings.Contains(firstLines, "a add") {
		t.Fatalf("todo header should not include shortcut row, got:\n%s", firstLines)
	}
}

func TestActiveSessionFooterDoesNotAdvertiseTodoShortcut(t *testing.T) {
	m := testModel()
	m.CurrentView = ActiveSession
	m.ActiveSession = testActiveSession()
	m.Width = 120

	footer := xansi.Strip(renderFooter(m))
	if strings.Contains(footer, "/todo") {
		t.Fatalf("active footer should stay compact without /todo shortcut, got %q", footer)
	}
}

func TestTodoViewSelectionMovesWithArrowKeys(t *testing.T) {
	m, todoStore, _ := newTodoViewTestModel(t)
	addTodoViewTestItem(t, todoStore, "first")
	addTodoViewTestItem(t, todoStore, "second")
	m = applyTodoViewCommand(t, m, m.loadTodoItemsCmd(0, "", ""))

	updated, cmd := pressTodoViewKey(t, m, tea.KeyMsg{Type: tea.KeyDown})
	if cmd != nil {
		t.Fatal("down should not return a command")
	}
	if updated.TodoSelected != 1 {
		t.Fatalf("TodoSelected after down = %d, want 1", updated.TodoSelected)
	}

	updated, cmd = pressTodoViewKey(t, updated, tea.KeyMsg{Type: tea.KeyUp})
	if cmd != nil {
		t.Fatal("up should not return a command")
	}
	if updated.TodoSelected != 0 {
		t.Fatalf("TodoSelected after up = %d, want 0", updated.TodoSelected)
	}
}

func TestTodoViewAddPromptMutatesStoreWithAttribution(t *testing.T) {
	m, todoStore, _ := newTodoViewTestModel(t)
	m.ActiveSession = &store.SessionRecord{Session: store.Session{ID: "2026-01-15T143022Z", Branch: "feat/todo", Status: "active"}}
	m = applyTodoViewCommand(t, m, m.loadTodoItemsCmd(0, "", ""))

	updated, cmd := pressTodoViewKey(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	if cmd == nil {
		t.Fatal("a should start cursor tick for add prompt")
	}
	if !updated.TodoPromptOpen || updated.TodoPromptMode != todoPromptAdd {
		t.Fatalf("add prompt state open=%v mode=%q", updated.TodoPromptOpen, updated.TodoPromptMode)
	}

	for _, r := range "ship tui todos" {
		updated, _ = pressTodoViewKey(t, updated, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	updated, cmd = pressTodoViewKey(t, updated, tea.KeyMsg{Type: tea.KeyEnter})
	updated = applyTodoViewCommand(t, updated, cmd)

	items, err := todoStore.Load()
	if err != nil {
		t.Fatalf("todo load failed: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("todo count = %d, want 1", len(items))
	}
	if items[0].Text != "ship tui todos" || items[0].SessionID != "2026-01-15T143022Z" || items[0].Branch != "feat/todo" {
		t.Fatalf("added item = %+v, want text with active session attribution", items[0])
	}
	if updated.TodoSelected != 0 || !strings.Contains(xansi.Strip(updated.View()), "ship tui todos") {
		t.Fatalf("updated view should select and render new todo, selected=%d view=\n%s", updated.TodoSelected, updated.View())
	}
}

func TestTodoViewEditToggleAndDeleteFlowsMutateStore(t *testing.T) {
	m, todoStore, _ := newTodoViewTestModel(t)
	item := addTodoViewTestItem(t, todoStore, "original text")
	m = applyTodoViewCommand(t, m, m.loadTodoItemsCmd(0, "", ""))

	updated, _ := pressTodoViewKey(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	if !updated.TodoPromptOpen || updated.TodoEditingID != item.ID {
		t.Fatalf("edit prompt open=%v editingID=%q, want %q", updated.TodoPromptOpen, updated.TodoEditingID, item.ID)
	}
	for len(updated.TodoInput) > 0 {
		updated, _ = pressTodoViewKey(t, updated, tea.KeyMsg{Type: tea.KeyBackspace})
	}
	for _, r := range "updated text" {
		updated, _ = pressTodoViewKey(t, updated, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	updated, cmd := pressTodoViewKey(t, updated, tea.KeyMsg{Type: tea.KeyEnter})
	updated = applyTodoViewCommand(t, updated, cmd)

	items, err := todoStore.Load()
	if err != nil {
		t.Fatalf("load after edit failed: %v", err)
	}
	if items[0].Text != "updated text" {
		t.Fatalf("Text after edit = %q, want updated text", items[0].Text)
	}

	updated, cmd = pressTodoViewKey(t, updated, tea.KeyMsg{Type: tea.KeyEnter})
	updated = applyTodoViewCommand(t, updated, cmd)
	items, err = todoStore.Load()
	if err != nil {
		t.Fatalf("load after enter toggle failed: %v", err)
	}
	if items[0].Status != todo.StatusDone {
		t.Fatalf("Status after enter toggle = %q, want done", items[0].Status)
	}

	updated, cmd = pressTodoViewKey(t, updated, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
	updated = applyTodoViewCommand(t, updated, cmd)
	items, err = todoStore.Load()
	if err != nil {
		t.Fatalf("load after reopen failed: %v", err)
	}
	if items[0].Status != todo.StatusOpen {
		t.Fatalf("Status after second toggle = %q, want open", items[0].Status)
	}

	updated, cmd = pressTodoViewKey(t, updated, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	if cmd != nil {
		t.Fatal("delete prompt should not mutate immediately")
	}
	if !updated.TodoDeleteConfirm {
		t.Fatal("delete key should open confirmation")
	}
	updated, cmd = pressTodoViewKey(t, updated, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	if cmd != nil || updated.TodoDeleteConfirm {
		t.Fatalf("n should cancel delete without command, confirm=%v cmd nil=%v", updated.TodoDeleteConfirm, cmd == nil)
	}
	items, err = todoStore.Load()
	if err != nil {
		t.Fatalf("load after cancel failed: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("delete cancel item count = %d, want 1", len(items))
	}

	updated, _ = pressTodoViewKey(t, updated, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	updated, cmd = pressTodoViewKey(t, updated, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	updated = applyTodoViewCommand(t, updated, cmd)
	items, err = todoStore.Load()
	if err != nil {
		t.Fatalf("load after delete failed: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("delete item count = %d, want 0", len(items))
	}
	if updated.TodoSelected != -1 {
		t.Fatalf("TodoSelected after deleting last item = %d, want -1", updated.TodoSelected)
	}
}

func TestTodoViewPaletteCommandsUseRenderedNumbers(t *testing.T) {
	m, todoStore, _ := newTodoViewTestModel(t)
	first := addTodoViewTestItem(t, todoStore, "first")
	second := addTodoViewTestItem(t, todoStore, "second")
	if err := todoStore.Complete(first.ID); err != nil {
		t.Fatalf("complete first failed: %v", err)
	}
	m = applyTodoViewCommand(t, m, m.loadTodoItemsCmd(0, "", ""))

	updatedModel, cmd := m.Update(CommandExecutedMsg{Input: "/todo done 1"})
	updated, ok := updatedModel.(Model)
	if !ok {
		t.Fatalf("expected Model after command, got %T", updatedModel)
	}
	updated = applyTodoViewCommand(t, updated, cmd)

	items, err := todoStore.Load()
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}
	for _, item := range items {
		if item.ID == second.ID && item.Status != todo.StatusDone {
			t.Fatalf("/todo done 1 should complete rendered first open item, got %+v", item)
		}
	}
}

func TestTodoViewEmptyAndErrorStatesAreVisible(t *testing.T) {
	m, _, _ := newTodoViewTestModel(t)
	m = applyTodoViewCommand(t, m, m.loadTodoItemsCmd(0, "", ""))
	if got := xansi.Strip(renderTodoView(m)); !strings.Contains(got, "No todos yet") {
		t.Fatalf("empty todo view should be actionable, got:\n%s", got)
	}

	updatedModel, cmd := m.Update(CommandExecutedMsg{Input: "/todo done 1"})
	updated, ok := updatedModel.(Model)
	if !ok {
		t.Fatalf("expected Model after invalid command, got %T", updatedModel)
	}
	if cmd != nil {
		t.Fatal("invalid rendered number should not return command")
	}
	if !strings.Contains(updated.ErrorMessage, "out of range") {
		t.Fatalf("ErrorMessage = %q, want out of range", updated.ErrorMessage)
	}
	if got := xansi.Strip(renderTodoView(updated)); !strings.Contains(got, "ERROR:") || !strings.Contains(got, "out of range") {
		t.Fatalf("error should be visible in todo view, got:\n%s", got)
	}
}

func TestTodoViewBackReturnsToActiveSession(t *testing.T) {
	m, _, _ := newTodoViewTestModel(t)
	m.ActiveSession = &store.SessionRecord{Session: store.Session{ID: "2026-01-15T143022Z", Started: time.Date(2026, 1, 15, 14, 30, 22, 0, time.UTC), Status: "active"}}

	updated, cmd := pressTodoViewKey(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if updated.CurrentView != ActiveSession {
		t.Fatalf("CurrentView after q = %v, want ActiveSession", updated.CurrentView)
	}
	if updated.TodoOpen {
		t.Fatal("q should close the todo overlay")
	}
	if cmd != nil {
		t.Fatal("closing overlay should not require a command")
	}
}
