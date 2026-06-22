package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/amo/devlog/internal/store"
)

func TestStatusCommandShowsActiveSessionEventsAndBlockers(t *testing.T) {
	requireCmdTestGit(t)
	setConfigTestHome(t)

	root := initCmdTestRepo(t)
	sess := writeCmdTestSession(t, root)
	appendCmdTestEvent(t, root, sess.ID, "Note", "finished parser")
	appendCmdTestEvent(t, root, sess.ID, "Blocker", "waiting for review")
	appendCmdTestEvent(t, root, sess.ID, "Note", "wrote status tests")
	t.Chdir(root)

	out, err := executeStatusCommand()
	if err != nil {
		t.Fatalf("status command failed: %v", err)
	}

	assertContains(t, out, "start message")
	assertContains(t, out, "("+sess.ID+")")
	assertContains(t, out, cliMutedStyle.Render("("+sess.ID+")"))
	assertNotContains(t, out, "id: "+sess.ID)
	assertContains(t, out, "author: Test Author")
	assertContains(t, out, "branch: feat/test (active)")
	assertContains(t, out, "started: 2026-01-15T14:00:00Z")
	assertContains(t, out, "duration: ")
	assertContains(t, out, "Recent events (last 10)")
	assertContains(t, out, "•")
	assertContains(t, out, " UTC Note: wrote status tests")
	assertContains(t, out, "Note: wrote status tests")
	assertContains(t, out, "Blocker: waiting for review")
	assertNotContains(t, out, "Start: start message")
	assertContains(t, out, "Blockers")
	assertContains(t, out, cliBlockerStyle.Render("Blockers"))
	assertContains(t, out, " UTC: waiting for review")
	assertContains(t, out, "waiting for review")
}

func TestStatusCommandFailsWithoutActiveSession(t *testing.T) {
	requireCmdTestGit(t)

	root := initCmdTestRepo(t)
	t.Chdir(root)

	_, err := executeStatusCommand()
	if err == nil || !strings.Contains(err.Error(), "no active session is in progress") {
		t.Fatalf("expected no active session error, got: %v", err)
	}
}

func TestStatusCommandLimitsRecentEvents(t *testing.T) {
	requireCmdTestGit(t)
	setConfigTestHome(t)

	root := initCmdTestRepo(t)
	sess := writeCmdTestSession(t, root)
	appendCmdTestEvent(t, root, sess.ID, "Blocker", "old blocker")
	appendCmdTestEvent(t, root, sess.ID, "Note", "first note")
	appendCmdTestEvent(t, root, sess.ID, "Note", "second note")
	appendCmdTestEvent(t, root, sess.ID, "Note", "third note")
	t.Chdir(root)

	out, err := executeStatusCommand("-n", "2")
	if err != nil {
		t.Fatalf("status command failed: %v", err)
	}

	assertContains(t, out, "Recent events (last 2)")
	assertContains(t, out, "third note")
	assertContains(t, out, "second note")
	assertContains(t, out, "old blocker")
	assertNotContains(t, out, "first note")
	assertNotContains(t, out, "Start: start message")
}

func TestStatusCommandShowsAllRecentEventsWhenNumberIsZero(t *testing.T) {
	requireCmdTestGit(t)
	setConfigTestHome(t)

	root := initCmdTestRepo(t)
	sess := writeCmdTestSession(t, root)
	appendCmdTestEvent(t, root, sess.ID, "Note", "first note")
	appendCmdTestEvent(t, root, sess.ID, "Note", "second note")
	t.Chdir(root)

	out, err := executeStatusCommand("-n", "0")
	if err != nil {
		t.Fatalf("status command failed: %v", err)
	}

	assertContains(t, out, "Recent events (all)")
	assertContains(t, out, "first note")
	assertContains(t, out, "second note")
	assertContains(t, out, "start message")
	assertNotContains(t, out, "Start: start message")
}

func TestStatusCommandHidesDeletedEvents(t *testing.T) {
	requireCmdTestGit(t)
	setConfigTestHome(t)

	root := initCmdTestRepo(t)
	sess := writeCmdTestSession(t, root)
	s := mustNewStore(t, root)
	if err := s.AppendEvent(sess.ID, "Note", "first note"); err != nil {
		t.Fatalf("append first note: %v", err)
	}
	body, err := s.ReadSessionBody(sess.ID)
	if err != nil {
		t.Fatalf("ReadSessionBody: %v", err)
	}
	events := store.ParseSessionEvents(body)
	if err := s.AppendEvent(sess.ID, "Edit", store.FormatEditBody(events[1], "delete", "")); err != nil {
		t.Fatalf("append delete edit: %v", err)
	}
	t.Chdir(root)

	out, err := executeStatusCommand("-n", "0")
	if err != nil {
		t.Fatalf("status command failed: %v", err)
	}

	assertNotContains(t, out, "first note")
}

func TestStatusCommandHidesDeletedBlockers(t *testing.T) {
	requireCmdTestGit(t)
	setConfigTestHome(t)

	root := initCmdTestRepo(t)
	sess := writeCmdTestSession(t, root)
	s := mustNewStore(t, root)
	if err := s.AppendEvent(sess.ID, "Blocker", "obsolete blocker"); err != nil {
		t.Fatalf("append blocker: %v", err)
	}
	body, err := s.ReadSessionBody(sess.ID)
	if err != nil {
		t.Fatalf("ReadSessionBody: %v", err)
	}
	events := store.ParseSessionEvents(body)
	if err := s.AppendEvent(sess.ID, "Edit", store.FormatEditBody(events[1], "delete", "")); err != nil {
		t.Fatalf("append delete edit: %v", err)
	}
	t.Chdir(root)

	out, err := executeStatusCommand("-n", "0")
	if err != nil {
		t.Fatalf("status command failed: %v", err)
	}

	assertNotContains(t, out, "obsolete blocker")
	assertContains(t, out, "Blockers")
	assertContains(t, out, "None")
}

func TestStatusCommandUsesConfiguredDisplayTime(t *testing.T) {
	requireCmdTestGit(t)
	_, configPath := setConfigTestHome(t)
	writeConfigTestFile(t, configPath, `display:
  timezone: America/New_York
  clock_format: 12h
`)

	root := initCmdTestRepo(t)
	sess := writeCmdTestSession(t, root)
	appendCmdTestSessionBody(t, root, sess.ID, "\n## Note - 2026-01-15 15:30 UTC\n\nfinished parser\n")
	appendCmdTestSessionBody(t, root, sess.ID, "\n## Blocker - 2026-01-15 16:45 UTC\n\nwaiting for review\n")
	t.Chdir(root)

	out, err := executeStatusCommand()
	if err != nil {
		t.Fatalf("status command failed: %v", err)
	}

	assertContains(t, out, "started: 2026-01-15 9:00:00 AM EST")
	assertContains(t, out, "2026-01-15 10:30 AM EST Note: finished parser")
	assertContains(t, out, "2026-01-15 11:45 AM EST: waiting for review")
}

func TestStatusCommandRejectsInvalidDisplayConfig(t *testing.T) {
	requireCmdTestGit(t)
	_, configPath := setConfigTestHome(t)
	writeConfigTestFile(t, configPath, `display:
  timezone: "New York"
`)

	root := initCmdTestRepo(t)
	writeCmdTestSession(t, root)
	t.Chdir(root)

	_, err := executeStatusCommand()
	if err == nil || !strings.Contains(err.Error(), "display.timezone") {
		t.Fatalf("expected display timezone error, got: %v", err)
	}
}

func TestRootCommandIncludesStatusCommand(t *testing.T) {
	cmd, _, err := newRootCommand().Find([]string{"status"})
	if err != nil {
		t.Fatalf("find status command failed: %v", err)
	}
	if cmd == nil || cmd.Name() != "status" {
		t.Fatalf("expected root command to include status, got %v", cmd)
	}
}

func executeStatusCommand(args ...string) (string, error) {
	cmd := newStatusCommand()
	var out bytes.Buffer
	var errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetArgs(args)

	err := cmd.Execute()
	return out.String(), err
}

func appendCmdTestEvent(t *testing.T, root, sessionID, eventType, body string) {
	t.Helper()

	s, err := store.New(root)
	if err != nil {
		t.Fatalf("store.New failed: %v", err)
	}
	if err := s.AppendEvent(sessionID, eventType, body); err != nil {
		t.Fatalf("AppendEvent failed: %v", err)
	}
}

func assertContains(t *testing.T, got, want string) {
	t.Helper()

	if !strings.Contains(got, want) {
		t.Fatalf("expected output to contain %q, got:\n%s", want, got)
	}
}

func assertNotContains(t *testing.T, got, want string) {
	t.Helper()

	if strings.Contains(got, want) {
		t.Fatalf("expected output not to contain %q, got:\n%s", want, got)
	}
}
