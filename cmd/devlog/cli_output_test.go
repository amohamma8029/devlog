package main

import (
	"strings"
	"testing"
)

func TestRenderCLIConfirmationUsesStaticScrollbackLayout(t *testing.T) {
	out := renderCLIConfirmation("Opened session", cliField{"ID", "2026-01-15T140000Z"}, cliField{"Branch", "feat/test"})

	assertContains(t, out, "Opened session")
	assertContains(t, out, "ID: 2026-01-15T140000Z")
	assertContains(t, out, "Branch: feat/test")
	for _, boxed := range []string{"╭", "╮", "╰", "╯", "│"} {
		if strings.Contains(out, boxed) {
			t.Fatalf("static CLI output should not use boxed layout %q, got:\n%s", boxed, out)
		}
	}
}

func TestBlockerOutputUsesBlockerStyle(t *testing.T) {
	confirmation := renderCLISessionConfirmation("Logged blocker", "blocked", true, "start message", "2026-01-15T140000Z", "feat/test", false)
	assertContains(t, confirmation, cliBlockerStyle.Render("Logged blocker"))
	assertContains(t, confirmation, cliMutedStyle.Render("→"))
	assertContains(t, confirmation, cliBlockerTextStyle.Render("blocked"))

	if got := cliBlockerTitle("Blockers"); got != cliBlockerStyle.Render("Blockers") {
		t.Fatalf("blocker title should use blocker style, got %q", got)
	}
	if got := cliEventText("Blocker", "  - Blocker: blocked"); got != cliBlockerStyle.Render("  - Blocker: blocked") {
		t.Fatalf("blocker event should use blocker style, got %q", got)
	}
}

func TestSessionConfirmationUsesPreviewAndMetadataShape(t *testing.T) {
	out := renderCLISessionConfirmation("Added note", "line one\nline two", false, "start message", "2026-01-15T140000Z", "feat/test", false)
	assertContains(t, out, cliTitleStyle.Render("Added note"))
	assertContains(t, out, cliMutedStyle.Render("→"))
	assertContains(t, out, cliValueStyle.Render("line one line two"))
	assertContains(t, out, "session: start message (2026-01-15T140000Z)")
	assertContains(t, out, cliMutedStyle.Render("(2026-01-15T140000Z)"))
	assertNotContains(t, out, "id: 2026-01-15T140000Z")
	assertContains(t, out, "branch: feat/test (active)")
}

func TestHandoffConfirmationRendersPathLast(t *testing.T) {
	out := renderCLIHandoffConfirmation(".devlog/handoffs/session.md", "start message", "2026-01-15T140000Z", "feat/test", false)
	if strings.Index(out, "path:") < strings.Index(out, "branch:") {
		t.Fatalf("path should render after session metadata, got:\n%s", out)
	}
}

func TestBranchStateUsesStatusStyles(t *testing.T) {
	if got := cliStateText(false); got != cliActiveStyle.Render("(active)") {
		t.Fatalf("active state should use active style, got %q", got)
	}
	if got := cliStateText(true); got != cliClosedStyle.Render("(closed)") {
		t.Fatalf("closed state should use closed style, got %q", got)
	}
}
