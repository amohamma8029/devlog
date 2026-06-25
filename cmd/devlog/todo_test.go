package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/amo/devlog/internal/todo"
)

func TestRootCommandIncludesTodoCommand(t *testing.T) {
	cmd, _, err := newRootCommand().Find([]string{"todo"})
	if err != nil {
		t.Fatalf("find todo command failed: %v", err)
	}
	if cmd == nil || cmd.Name() != "todo" {
		t.Fatalf("expected root command to include todo, got %v", cmd)
	}

	help := todoRootHelp(t)
	assertContains(t, help, "todo")
	assertContains(t, help, "Tasks")
}

func TestTodoAddAttributesActiveSession(t *testing.T) {
	requireCmdTestGit(t)

	root := initCmdTestRepo(t)
	sess := writeCmdTestSession(t, root)
	t.Chdir(root)

	out, err := executeTodoCommand("add", "ship", "todo", "CLI")
	if err != nil {
		t.Fatalf("todo add failed: %v", err)
	}

	assertContains(t, out, "Added todo")
	assertContains(t, out, "[ ]")
	assertContains(t, out, "ship todo CLI")
	assertContains(t, out, "branch: feat/test")
	assertContains(t, out, "session: "+sess.ID)

	items := loadCmdTestTodos(t, root)
	if len(items) != 1 {
		t.Fatalf("expected 1 todo, got %d", len(items))
	}
	if items[0].Text != "ship todo CLI" {
		t.Fatalf("todo text = %q, want ship todo CLI", items[0].Text)
	}
	if items[0].SessionID != sess.ID {
		t.Fatalf("SessionID = %q, want %q", items[0].SessionID, sess.ID)
	}
	if items[0].Branch != "feat/test" {
		t.Fatalf("Branch = %q, want feat/test", items[0].Branch)
	}

	log := readCmdTestTodoLog(t, root)
	assertContains(t, log, "action: add")
	assertContains(t, log, "session_id: "+sess.ID)
	assertContains(t, log, "branch: feat/test")
}

func TestTodoAddWithoutActiveSessionKeepsAttributionEmpty(t *testing.T) {
	requireCmdTestGit(t)

	root := initCmdTestRepo(t)
	t.Chdir(root)

	out, err := executeTodoCommand("add", "unscoped", "todo")
	if err != nil {
		t.Fatalf("todo add failed without active session: %v", err)
	}
	assertContains(t, out, "Added todo")
	assertNotContains(t, out, "branch:")
	assertNotContains(t, out, "session:")

	items := loadCmdTestTodos(t, root)
	if len(items) != 1 {
		t.Fatalf("expected 1 todo, got %d", len(items))
	}
	if items[0].SessionID != "" || items[0].Branch != "" {
		t.Fatalf("expected empty attribution, got session=%q branch=%q", items[0].SessionID, items[0].Branch)
	}
}

func TestTodoCRUDWithListNumbers(t *testing.T) {
	requireCmdTestGit(t)

	root := initCmdTestRepo(t)
	t.Chdir(root)

	out, err := executeTodoCommand("add", "first", "todo")
	if err != nil {
		t.Fatalf("add first failed: %v\n%s", err, out)
	}
	out, err = executeTodoCommand("add", "second", "todo")
	if err != nil {
		t.Fatalf("add second failed: %v\n%s", err, out)
	}

	out, err = executeTodoCommand("list")
	if err != nil {
		t.Fatalf("list failed: %v", err)
	}
	assertContains(t, out, "1.")
	assertContains(t, out, "2.")
	assertContains(t, out, "[ ]")
	assertContains(t, out, "first todo")
	assertContains(t, out, "second todo")

	out, err = executeTodoCommand("done", "1")
	if err != nil {
		t.Fatalf("done failed: %v", err)
	}
	assertContains(t, out, "Completed todo")
	assertContains(t, out, "[x]")
	assertContains(t, out, "first todo")

	out, err = executeTodoCommand("list")
	if err != nil {
		t.Fatalf("list after done failed: %v", err)
	}
	assertContains(t, out, "second todo")
	assertNotContains(t, out, "first todo")

	out, err = executeTodoCommand("list", "--done")
	if err != nil {
		t.Fatalf("done list failed: %v", err)
	}
	assertContains(t, out, "[x]")
	assertContains(t, out, "first todo")
	assertNotContains(t, out, "second todo")

	out, err = executeTodoCommand("reopen", "1", "--done")
	if err != nil {
		t.Fatalf("reopen failed: %v", err)
	}
	assertContains(t, out, "Reopened todo")
	assertContains(t, out, "[ ]")
	assertContains(t, out, "first todo")

	out, err = executeTodoCommand("edit", "1", "first", "edited")
	if err != nil {
		t.Fatalf("edit failed: %v", err)
	}
	assertContains(t, out, "Edited todo")
	assertContains(t, out, "first edited")

	out, err = executeTodoCommand("list")
	if err != nil {
		t.Fatalf("list after edit failed: %v", err)
	}
	assertContains(t, out, "first edited")
	assertContains(t, out, "second todo")

	out, err = executeTodoCommand("delete", "1")
	if err != nil {
		t.Fatalf("delete failed: %v", err)
	}
	assertContains(t, out, "Deleted todo")
	assertContains(t, out, "first edited")

	out, err = executeTodoCommand("list")
	if err != nil {
		t.Fatalf("list after delete failed: %v", err)
	}
	assertContains(t, out, "second todo")
	assertNotContains(t, out, "first edited")
}

func TestTodoListShowsCheckboxAndNumbers(t *testing.T) {
	requireCmdTestGit(t)

	root := initCmdTestRepo(t)
	t.Chdir(root)

	executeTodoCommand("add", "open", "item")
	executeTodoCommand("add", "done", "item")
	executeTodoCommand("done", "2")

	out, err := executeTodoCommand("list", "--all")
	if err != nil {
		t.Fatalf("list --all failed: %v", err)
	}
	assertContains(t, out, "1.")
	assertContains(t, out, "[ ]")
	assertContains(t, out, "open item")
	assertContains(t, out, "2.")
	assertContains(t, out, "[x]")
	assertContains(t, out, "done item")
}

func TestTodoListIdsShowsInternalIDs(t *testing.T) {
	requireCmdTestGit(t)

	root := initCmdTestRepo(t)
	t.Chdir(root)

	executeTodoCommand("add", "tracked", "todo")
	items := loadCmdTestTodos(t, root)
	if len(items) != 1 {
		t.Fatalf("expected 1 todo, got %d", len(items))
	}
	internalID := items[0].ID

	out, err := executeTodoCommand("list", "--ids")
	if err != nil {
		t.Fatalf("list --ids failed: %v", err)
	}
	assertContains(t, out, "id: "+internalID)

	out, err = executeTodoCommand("list")
	if err != nil {
		t.Fatalf("list failed: %v", err)
	}
	assertNotContains(t, out, "id: "+internalID)
}

func TestTodoListFilters(t *testing.T) {
	requireCmdTestGit(t)

	root := initCmdTestRepo(t)
	sess := writeCmdTestSession(t, root)
	t.Chdir(root)

	executeTodoCommand("add", "active", "todo")
	store := newCmdTestTodoStore(t, root)
	if _, err := store.Add(todo.AddInput{Text: "other todo", SessionID: "other-session", Branch: "feat/other"}); err != nil {
		t.Fatalf("add other todo failed: %v", err)
	}

	out, err := executeTodoCommand("list", "--branch", "feat/test")
	if err != nil {
		t.Fatalf("branch list failed: %v", err)
	}
	assertContains(t, out, "active todo")
	assertNotContains(t, out, "other todo")

	out, err = executeTodoCommand("list", "--session", sess.ID)
	if err != nil {
		t.Fatalf("session list failed: %v", err)
	}
	assertContains(t, out, "active todo")
	assertNotContains(t, out, "other todo")

	out, err = executeTodoCommand("list", "--all")
	if err != nil {
		t.Fatalf("all list failed: %v", err)
	}
	assertContains(t, out, "active todo")
	assertContains(t, out, "other todo")
}

func TestTodoCommandAcceptsInternalID(t *testing.T) {
	requireCmdTestGit(t)

	root := initCmdTestRepo(t)
	t.Chdir(root)

	executeTodoCommand("add", "tracked", "todo")
	items := loadCmdTestTodos(t, root)
	if len(items) != 1 {
		t.Fatalf("expected 1 todo, got %d", len(items))
	}
	internalID := items[0].ID

	out, err := executeTodoCommand("done", internalID)
	if err != nil {
		t.Fatalf("done by internal ID failed: %v", err)
	}
	assertContains(t, out, "Completed todo")
	assertContains(t, out, "[x]")
	assertContains(t, out, "tracked todo")

	out, err = executeTodoCommand("list", "--done")
	if err != nil {
		t.Fatalf("done list failed: %v", err)
	}
	assertContains(t, out, "tracked todo")
}

func TestTodoCommandValidationErrors(t *testing.T) {
	requireCmdTestGit(t)

	root := initCmdTestRepo(t)
	t.Chdir(root)

	if _, err := executeTodoCommand("add"); err == nil || !strings.Contains(err.Error(), "todo text is required") {
		t.Fatalf("expected missing text error, got: %v", err)
	}
	if _, err := executeTodoCommand("done"); err == nil || !strings.Contains(err.Error(), "todo ref is required") {
		t.Fatalf("expected missing ref error, got: %v", err)
	}
	if _, err := executeTodoCommand("done", "99"); err == nil || !strings.Contains(err.Error(), "out of range") {
		t.Fatalf("expected out of range error, got: %v", err)
	}
	if _, err := executeTodoCommand("list", "--all", "--done"); err == nil || !strings.Contains(err.Error(), "cannot be used together") {
		t.Fatalf("expected conflicting filter error, got: %v", err)
	}
	if _, err := executeTodoCommand("done", "nonexistent-id-123"); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("expected not found error, got: %v", err)
	}

	out, err := executeTodoCommand("add", "stateful", "todo")
	if err != nil {
		t.Fatalf("add stateful todo failed: %v\n%s", err, out)
	}
	items := loadCmdTestTodos(t, root)
	if len(items) != 1 {
		t.Fatalf("expected 1 todo, got %d", len(items))
	}
	id := items[0].ID

	if _, err := executeTodoCommand("done", "1"); err != nil {
		t.Fatalf("first done failed: %v", err)
	}
	if _, err := executeTodoCommand("done", "1", "--done"); err == nil || !strings.Contains(err.Error(), "already done") {
		t.Fatalf("expected repeated done error, got: %v", err)
	}
	if _, err := executeTodoCommand("reopen", "1", "--done"); err != nil {
		t.Fatalf("reopen failed: %v", err)
	}
	if _, err := executeTodoCommand("delete", "1"); err != nil {
		t.Fatalf("delete failed: %v", err)
	}
	if _, err := executeTodoCommand("edit", id, "new", "text"); err == nil || !strings.Contains(err.Error(), "deleted") {
		t.Fatalf("expected deleted edit error, got: %v", err)
	}
}

func TestTodoAddUpdatesTodoLog(t *testing.T) {
	requireCmdTestGit(t)

	root := initCmdTestRepo(t)
	t.Chdir(root)

	_, err := executeTodoCommand("add", "log", "entry")
	if err != nil {
		t.Fatalf("todo add failed: %v", err)
	}

	log := readCmdTestTodoLog(t, root)
	assertContains(t, log, "action: add")
	assertContains(t, log, "log entry")
}

func TestTodoAddWithMessageFlag(t *testing.T) {
	requireCmdTestGit(t)

	root := initCmdTestRepo(t)
	t.Chdir(root)

	out, err := executeTodoCommand("add", "-m", "flagged todo")
	if err != nil {
		t.Fatalf("todo add -m failed: %v", err)
	}
	assertContains(t, out, "Added todo")
	assertContains(t, out, "flagged todo")

	items := loadCmdTestTodos(t, root)
	if len(items) != 1 || items[0].Text != "flagged todo" {
		t.Fatalf("expected flagged todo, got: %+v", items)
	}
}

func todoRootHelp(t *testing.T) string {
	t.Helper()

	cmd := newRootCommand()
	var out, errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetArgs([]string{"--help"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("root help failed: %v", err)
	}
	return out.String()
}

func executeTodoCommand(args ...string) (string, error) {
	cmd := newTodoCommand()
	var out bytes.Buffer
	var errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetArgs(args)

	err := cmd.Execute()
	return out.String(), err
}

func newCmdTestTodoStore(t *testing.T, root string) *todo.Store {
	t.Helper()

	store, err := todo.NewStore(root)
	if err != nil {
		t.Fatalf("todo.NewStore failed: %v", err)
	}
	return store
}

func loadCmdTestTodos(t *testing.T, root string) []todo.Item {
	t.Helper()

	items, err := newCmdTestTodoStore(t, root).Load()
	if err != nil {
		t.Fatalf("todo.Store.Load failed: %v", err)
	}
	return items
}

func readCmdTestTodoLog(t *testing.T, root string) string {
	t.Helper()

	data, err := os.ReadFile(filepath.Join(root, todo.LogPath()))
	if err != nil {
		t.Fatalf("read todo log failed: %v", err)
	}
	return string(data)
}
