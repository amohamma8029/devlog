package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/amo/devlog/internal/store"
)

func TestStatusCommandShowsActiveSessionEventsAndBlockers(t *testing.T) {
	requireCmdTestGit(t)

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

	assertContains(t, out, "Active session")
	assertContains(t, out, "ID: "+sess.ID)
	assertContains(t, out, "Author: Test Author")
	assertContains(t, out, "Branch: feat/test")
	assertContains(t, out, "Started: 2026-01-15T14:00:00Z")
	assertContains(t, out, "Duration: ")
	assertContains(t, out, "Recent events (last 10)")
	assertContains(t, out, " UTC Note: wrote status tests")
	assertContains(t, out, "Note: wrote status tests")
	assertContains(t, out, "Blocker: waiting for review")
	assertContains(t, out, "Start: start message")
	assertContains(t, out, "Blockers")
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
	assertNotContains(t, out, "start message")
}

func TestStatusCommandShowsAllRecentEventsWhenNumberIsZero(t *testing.T) {
	requireCmdTestGit(t)

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
