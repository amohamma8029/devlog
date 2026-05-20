package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	internalgit "github.com/amo/devlog/internal/git"
	"github.com/amo/devlog/internal/session"
	"github.com/amo/devlog/internal/store"
	"github.com/spf13/cobra"
)

type statusEvent struct {
	Type string
	At   string
	Body string
}

func newStatusCommand() *cobra.Command {
	var number int

	cmd := &cobra.Command{
		Use:          "status",
		Short:        "Show the active devlog session status.",
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if number < 0 {
				return fmt.Errorf("number must be greater than or equal to 0")
			}

			root, err := internalgit.RepoRoot()
			if err != nil {
				return err
			}

			s, err := store.New(root)
			if err != nil {
				return err
			}

			active, err := session.FindActiveSession(s)
			if err != nil {
				return err
			}

			body, err := readStatusSessionBody(root, active.ID)
			if err != nil {
				return err
			}

			events := parseStatusEvents(body)
			_, err = fmt.Fprint(cmd.OutOrStdout(), renderStatus(active, events, number, time.Now().UTC()))
			return err
		},
	}

	cmd.Flags().IntVarP(&number, "number", "n", 10, "Number of recent events to show (0 shows all)")

	return cmd
}

func readStatusSessionBody(root, sessionID string) (string, error) {
	path, err := statusSessionPath(root, sessionID)
	if err != nil {
		return "", err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("status: read session file: %w", err)
	}

	body, err := statusMarkdownBody(string(data))
	if err != nil {
		return "", err
	}

	return body, nil
}

func statusSessionPath(root, sessionID string) (string, error) {
	if strings.TrimSpace(sessionID) == "" {
		return "", fmt.Errorf("status: session ID is empty")
	}
	if sessionID == "." || sessionID == ".." || strings.ContainsAny(sessionID, `/\`) {
		return "", fmt.Errorf("status: invalid session ID: %s", sessionID)
	}

	return filepath.Join(root, ".devlog", "sessions", sessionID+".md"), nil
}

func statusMarkdownBody(content string) (string, error) {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	const delim = "---\n"
	if !strings.HasPrefix(content, delim) {
		return "", fmt.Errorf("status: missing opening front-matter delimiter")
	}

	parts := strings.SplitN(content[len(delim):], delim, 2)
	if len(parts) < 2 {
		return "", fmt.Errorf("status: missing closing front-matter delimiter")
	}

	return parts[1], nil
}

func parseStatusEvents(markdown string) []statusEvent {
	lines := strings.Split(strings.ReplaceAll(markdown, "\r\n", "\n"), "\n")
	var events []statusEvent
	var current *statusEvent
	var bodyLines []string

	flush := func() {
		if current == nil {
			return
		}
		current.Body = strings.TrimSpace(strings.Join(bodyLines, "\n"))
		events = append(events, *current)
	}

	for _, line := range lines {
		eventType, at, ok := parseStatusEventHeading(line)
		if ok {
			flush()
			current = &statusEvent{Type: eventType, At: at}
			bodyLines = nil
			continue
		}

		if current != nil {
			bodyLines = append(bodyLines, line)
		}
	}

	flush()
	return events
}

func parseStatusEventHeading(line string) (string, string, bool) {
	if !strings.HasPrefix(line, "## ") {
		return "", "", false
	}

	heading := strings.TrimSpace(strings.TrimPrefix(line, "## "))
	for _, eventType := range []string{"Start", "Note", "Blocker", "Stop"} {
		if heading == eventType {
			return eventType, "", true
		}

		prefix := eventType + " - "
		if strings.HasPrefix(heading, prefix) {
			return eventType, strings.TrimSpace(strings.TrimPrefix(heading, prefix)), true
		}
	}

	return "", "", false
}

func renderStatus(active *store.SessionRecord, events []statusEvent, number int, now time.Time) string {
	var b strings.Builder

	fmt.Fprintf(&b, "Active session\n")
	fmt.Fprintf(&b, "ID: %s\n", active.ID)
	fmt.Fprintf(&b, "Author: %s\n", formatStatusAuthor(active.Author, active.Email))
	fmt.Fprintf(&b, "Branch: %s\n", active.Branch)
	fmt.Fprintf(&b, "Started: %s\n", active.Started.UTC().Format(time.RFC3339))
	fmt.Fprintf(&b, "Duration: %s\n", formatStatusDuration(now.Sub(active.Started.UTC())))

	b.WriteString("\n")
	if number == 0 {
		b.WriteString("Recent events (all)\n")
	} else {
		fmt.Fprintf(&b, "Recent events (last %d)\n", number)
	}
	writeRecentStatusEvents(&b, recentStatusEvents(events, number))

	b.WriteString("\nBlockers\n")
	writeStatusBlockers(&b, events)

	return b.String()
}

func formatStatusAuthor(author, email string) string {
	author = strings.TrimSpace(author)
	email = strings.TrimSpace(email)
	if author == "" && email == "" {
		return "(unknown)"
	}
	if author == "" {
		return email
	}
	if email == "" {
		return author
	}

	return fmt.Sprintf("%s <%s>", author, email)
}

func formatStatusDuration(d time.Duration) string {
	if d < 0 {
		d = 0
	}

	d = d.Round(time.Minute)
	if d < time.Minute {
		return "less than 1m"
	}

	days := d / (24 * time.Hour)
	d -= days * 24 * time.Hour
	hours := d / time.Hour
	d -= hours * time.Hour
	minutes := d / time.Minute

	var parts []string
	if days > 0 {
		parts = append(parts, fmt.Sprintf("%dd", days))
	}
	if hours > 0 {
		parts = append(parts, fmt.Sprintf("%dh", hours))
	}
	if days == 0 && minutes > 0 {
		parts = append(parts, fmt.Sprintf("%dm", minutes))
	}
	if len(parts) == 0 {
		return "less than 1m"
	}

	return strings.Join(parts, " ")
}

func recentStatusEvents(events []statusEvent, number int) []statusEvent {
	if number == 0 || number >= len(events) {
		return events
	}

	return events[len(events)-number:]
}

func writeRecentStatusEvents(b *strings.Builder, events []statusEvent) {
	if len(events) == 0 {
		b.WriteString("None\n")
		return
	}

	for i := len(events) - 1; i >= 0; i-- {
		writeStatusEvent(b, events[i])
	}
}

func writeStatusEvent(b *strings.Builder, event statusEvent) {
	if event.At == "" {
		fmt.Fprintf(b, "- %s: %s\n", event.Type, oneLineStatusBody(event.Body))
		return
	}

	fmt.Fprintf(b, "- %s %s: %s\n", event.At, event.Type, oneLineStatusBody(event.Body))
}

func writeStatusBlockers(b *strings.Builder, events []statusEvent) {
	found := false
	for _, event := range events {
		if event.Type != "Blocker" {
			continue
		}

		found = true
		if event.At == "" {
			fmt.Fprintf(b, "- %s\n", oneLineStatusBody(event.Body))
			continue
		}
		fmt.Fprintf(b, "- %s: %s\n", event.At, oneLineStatusBody(event.Body))
	}

	if !found {
		b.WriteString("None\n")
	}
}

func oneLineStatusBody(body string) string {
	fields := strings.Fields(body)
	if len(fields) == 0 {
		return "(empty)"
	}

	return strings.Join(fields, " ")
}
