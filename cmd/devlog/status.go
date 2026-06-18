package main

import (
	"fmt"
	"strings"
	"time"

	internalgit "github.com/amo/devlog/internal/git"
	"github.com/amo/devlog/internal/session"
	"github.com/amo/devlog/internal/store"
	"github.com/spf13/cobra"
)

const statusEventTimeLayout = "2006-01-02 15:04 UTC"

func newStatusCommand() *cobra.Command {
	var number int

	cmd := &cobra.Command{
		Use:          "status",
		Short:        "Show active session status.",
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if number < 0 {
				return fmt.Errorf(`number must be 0 or greater. Run "devlog status -n <number>" with a non-negative value.`)
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

			body, err := s.ReadSessionBody(active.ID)
			if err != nil {
				return err
			}

			events := store.ParseSessionEvents(body)
			_, err = fmt.Fprint(cmd.OutOrStdout(), renderStatus(active, events, number, time.Now().UTC()))
			return err
		},
	}

	cmd.Flags().IntVarP(&number, "number", "n", 10, "Number of recent events to show (0 shows all)")

	return cmd
}

func renderStatus(active *store.SessionRecord, events []store.SessionEvent, number int, now time.Time) string {
	var b strings.Builder

	b.WriteString(cliSessionTitleWithID(statusSessionTitle(events, active.ID), active.ID))
	b.WriteByte('\n')
	writeCLIBranchField(&b, active.Branch, false)
	writeCLIField(&b, "author", formatStatusAuthor(active.Author, active.Email))
	writeCLIField(&b, "started", active.Started.UTC().Format(time.RFC3339))
	writeCLIField(&b, "duration", formatStatusDuration(now.Sub(active.Started.UTC())))

	b.WriteString("\n")
	if number == 0 {
		b.WriteString(cliTitleStyle.Render("Recent events (all)"))
		b.WriteByte('\n')
	} else {
		b.WriteString(cliTitleStyle.Render(fmt.Sprintf("Recent events (last %d)", number)))
		b.WriteByte('\n')
	}
	writeRecentStatusEvents(&b, recentStatusEvents(events, number))

	b.WriteString("\n")
	b.WriteString(cliBlockerTitle("Blockers"))
	b.WriteByte('\n')
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

func recentStatusEvents(events []store.SessionEvent, number int) []store.SessionEvent {
	var visible []store.SessionEvent
	for _, event := range events {
		if event.Type == "Start" {
			continue
		}
		visible = append(visible, event)
	}

	if number == 0 || number >= len(visible) {
		return visible
	}

	return visible[len(visible)-number:]
}

func writeRecentStatusEvents(b *strings.Builder, events []store.SessionEvent) {
	if len(events) == 0 {
		b.WriteString("  None\n")
		return
	}

	for i := len(events) - 1; i >= 0; i-- {
		writeStatusEvent(b, events[i])
	}
}

func writeStatusEvent(b *strings.Builder, event store.SessionEvent) {
	var line string
	if event.Time.IsZero() {
		line = cliBulletLine(fmt.Sprintf("%s: %s", event.Type, oneLineStatusBody(event.Body)))
		b.WriteString(cliEventText(event.Type, line))
		b.WriteByte('\n')
		return
	}

	line = cliBulletLine(fmt.Sprintf("%s %s: %s", formatStatusEventTime(event.Time), event.Type, oneLineStatusBody(event.Body)))
	b.WriteString(cliEventText(event.Type, line))
	b.WriteByte('\n')
}

func writeStatusBlockers(b *strings.Builder, events []store.SessionEvent) {
	found := false
	for _, event := range events {
		if event.Type != "Blocker" {
			continue
		}

		found = true
		var line string
		if event.Time.IsZero() {
			line = cliBulletLine(oneLineStatusBody(event.Body))
			b.WriteString(cliBlockerStyle.Render(line))
			b.WriteByte('\n')
			continue
		}
		line = cliBulletLine(fmt.Sprintf("%s: %s", formatStatusEventTime(event.Time), oneLineStatusBody(event.Body)))
		b.WriteString(cliBlockerStyle.Render(line))
		b.WriteByte('\n')
	}

	if !found {
		b.WriteString("  None\n")
	}
}

func formatStatusEventTime(t time.Time) string {
	return t.UTC().Format(statusEventTimeLayout)
}

func statusSessionTitle(events []store.SessionEvent, fallback string) string {
	for _, event := range events {
		if event.Type == "Start" {
			for _, line := range strings.Split(event.Body, "\n") {
				line = strings.TrimSpace(line)
				if line != "" {
					return line
				}
			}
		}
	}
	return fallback
}

func oneLineStatusBody(body string) string {
	fields := strings.Fields(body)
	if len(fields) == 0 {
		return "(empty)"
	}

	return strings.Join(fields, " ")
}
