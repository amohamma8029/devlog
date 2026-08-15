package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/amohamma8029/devlog/internal/store"
	"github.com/spf13/cobra"
)

func executeEditCommand(args ...string) (string, error) {
	cmd := newEditCommand()
	var out bytes.Buffer
	var errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetArgs(args)

	err := cmd.Execute()
	return out.String(), err
}

func mustExecuteCorrect(t *testing.T, args ...string) string {
	t.Helper()
	out, err := executeEditCommand(args...)
	if err != nil {
		t.Fatalf("correct command failed: %v\n%s", err, out)
	}
	return out
}

func TestEditCommandAppendsCorrection(t *testing.T) {
	requireCmdTestGit(t)

	root := initCmdTestRepo(t)
	sess := writeCmdTestSession(t, root)

	s := mustNewStore(t, root)
	if err := s.AppendEvent(sess.ID, "Note", "Original note text"); err != nil {
		t.Fatalf("append note: %v", err)
	}

	t.Chdir(root)

	out := mustExecuteCorrect(t, "1", "Corrected note text")

	if !strings.Contains(out, "Edited event 1") {
		t.Errorf("output should contain 'Edited event 1', got %q", out)
	}

	content := readCmdTestSessionFile(t, root, sess.ID)
	if !strings.Contains(content, "## Edit - ") {
		t.Fatal("expected session file to contain Edit event")
	}
	if !strings.Contains(content, "Target: Note ") || !strings.Contains(content, "Action: update") {
		t.Fatalf("expected edit to include target and update action, got content:\n%s", content)
	}
	if !strings.Contains(content, "Original:\nOriginal note text") || !strings.Contains(content, "New:\nCorrected note text") {
		t.Fatalf("expected edit to include original and new body, got content:\n%s", content)
	}

	// Verify the parser applies the correction
	body, err := s.ReadSessionBody(sess.ID)
	if err != nil {
		t.Fatalf("ReadSessionBody: %v", err)
	}
	events := store.ParseSessionEvents(body)
	for _, e := range events {
		if e.Type == "Note" {
			if e.Body != "Corrected note text" {
				t.Errorf("parsed note body = %q, want %q", e.Body, "Corrected note text")
			}
			break
		}
	}
}

func TestEditCommandDeleteFlag(t *testing.T) {
	requireCmdTestGit(t)

	root := initCmdTestRepo(t)
	sess := writeCmdTestSession(t, root)

	s := mustNewStore(t, root)
	if err := s.AppendEvent(sess.ID, "Note", "Note to delete"); err != nil {
		t.Fatalf("append note: %v", err)
	}

	t.Chdir(root)

	out := mustExecuteCorrect(t, "--delete", "1")

	if !strings.Contains(out, "Deleted event 1") {
		t.Errorf("output should contain 'Deleted event 1', got %q", out)
	}

	content := readCmdTestSessionFile(t, root, sess.ID)
	if !strings.Contains(content, "## Edit - ") {
		t.Fatal("expected session file to contain Edit event")
	}
	if !strings.Contains(content, "Target: Note ") || !strings.Contains(content, "Action: delete") {
		t.Fatalf("expected edit to include target and delete action, got content:\n%s", content)
	}
	if !strings.Contains(content, "Original:\nNote to delete") {
		t.Fatalf("expected delete edit to include original body, got content:\n%s", content)
	}

	body, err := s.ReadSessionBody(sess.ID)
	if err != nil {
		t.Fatalf("ReadSessionBody: %v", err)
	}
	events := store.ParseSessionEvents(body)
	for _, e := range events {
		if e.Type == "Note" {
			if !e.IsDeleted {
				t.Error("expected note to be deleted")
			}
			break
		}
	}
}

func TestEditCommandWithMessageFlag(t *testing.T) {
	requireCmdTestGit(t)

	root := initCmdTestRepo(t)
	sess := writeCmdTestSession(t, root)

	s := mustNewStore(t, root)
	if err := s.AppendEvent(sess.ID, "Note", "Original"); err != nil {
		t.Fatalf("append note: %v", err)
	}

	t.Chdir(root)

	out := mustExecuteCorrect(t, "-m", "Flag-corrected text", "1")

	if !strings.Contains(out, "Edited event 1") {
		t.Errorf("output should contain 'Edited event 1', got %q", out)
	}

	body, err := s.ReadSessionBody(sess.ID)
	if err != nil {
		t.Fatalf("ReadSessionBody: %v", err)
	}
	events := store.ParseSessionEvents(body)
	for _, e := range events {
		if e.Type == "Note" {
			if e.Body != "Flag-corrected text" {
				t.Errorf("parsed note body = %q, want %q", e.Body, "Flag-corrected text")
			}
			break
		}
	}
}

func TestEditCommandInvalidIndex(t *testing.T) {
	requireCmdTestGit(t)

	root := initCmdTestRepo(t)
	writeCmdTestSession(t, root)
	t.Chdir(root)

	_, err := executeEditCommand("abc", "text")
	if err == nil {
		t.Fatal("expected error for non-integer index")
	}
	if !strings.Contains(err.Error(), "positive integer") {
		t.Errorf("error should mention positive integer, got %q", err.Error())
	}
}

func TestEditCommandIndexOutOfRange(t *testing.T) {
	requireCmdTestGit(t)

	root := initCmdTestRepo(t)
	writeCmdTestSession(t, root)
	t.Chdir(root)

	_, err := executeEditCommand("99", "text")
	if err == nil {
		t.Fatal("expected error for out-of-range index")
	}
	if !strings.Contains(err.Error(), "out of range") {
		t.Errorf("error should mention out of range, got %q", err.Error())
	}
}

func TestEditCommandNoActiveSession(t *testing.T) {
	requireCmdTestGit(t)

	root := initCmdTestRepo(t)
	t.Chdir(root)

	_, err := executeEditCommand("1", "text")
	if err == nil {
		t.Fatal("expected error when no active session")
	}
	if !strings.Contains(err.Error(), "no active session") {
		t.Errorf("error should mention no active session, got %q", err.Error())
	}
}

func TestEditCommandRequiresIndex(t *testing.T) {
	cmd := newEditCommand()
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error when no index provided")
	}
}

func TestEditCommandCorrectsBlocker(t *testing.T) {
	requireCmdTestGit(t)

	root := initCmdTestRepo(t)
	sess := writeCmdTestSession(t, root)

	s := mustNewStore(t, root)
	if err := s.AppendEvent(sess.ID, "Blocker", "Original blocker"); err != nil {
		t.Fatalf("append blocker: %v", err)
	}

	t.Chdir(root)

	out := mustExecuteCorrect(t, "1", "Updated blocker text")

	if !strings.Contains(out, "Edited event 1") {
		t.Errorf("output should contain 'Edited event 1', got %q", out)
	}

	body, err := s.ReadSessionBody(sess.ID)
	if err != nil {
		t.Fatalf("ReadSessionBody: %v", err)
	}
	events := store.ParseSessionEvents(body)
	found := false
	for _, e := range events {
		if e.Type == "Blocker" {
			found = true
			if e.Body != "Updated blocker text" {
				t.Errorf("blocker body = %q, want %q", e.Body, "Updated blocker text")
			}
			break
		}
	}
	if !found {
		t.Fatal("blocker event not found in parsed events")
	}
}

func TestEditCommandMultipleEventsIndexing(t *testing.T) {
	requireCmdTestGit(t)

	root := initCmdTestRepo(t)
	sess := writeCmdTestSession(t, root)

	s := mustNewStore(t, root)
	if err := s.AppendEvent(sess.ID, "Note", "First note"); err != nil {
		t.Fatalf("append note 1: %v", err)
	}
	if err := s.AppendEvent(sess.ID, "Note", "Second note"); err != nil {
		t.Fatalf("append note 2: %v", err)
	}

	t.Chdir(root)

	// Index 1 = most recent (Second note)
	out := mustExecuteCorrect(t, "1", "Corrected second note")

	body, err := s.ReadSessionBody(sess.ID)
	if err != nil {
		t.Fatalf("ReadSessionBody: %v", err)
	}
	events := store.ParseSessionEvents(body)
	notes := []store.SessionEvent{}
	for _, e := range events {
		if e.Type == "Note" && !e.IsDeleted {
			notes = append(notes, e)
		}
	}
	if len(notes) < 2 {
		t.Fatal("expected 2 notes")
	}
	if notes[1].Body != "Corrected second note" {
		t.Errorf("second note (index 1) = %q, want %q", notes[1].Body, "Corrected second note")
	}
	if notes[0].Body != "First note" {
		t.Errorf("first note should be unchanged, got %q", notes[0].Body)
	}

	_ = out
}

func mustNewStore(t *testing.T, root string) *store.Store {
	t.Helper()
	s, err := store.New(root)
	if err != nil {
		t.Fatalf("store.New: %v", err)
	}
	return s
}

// Ensure correct command is a cobra command
var _ = func() *cobra.Command { return newEditCommand() }
