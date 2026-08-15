package main

import (
	"fmt"
	"strings"
	"time"

	internalconfig "github.com/amohamma8029/devlog/internal/config"
	internalgit "github.com/amohamma8029/devlog/internal/git"
	"github.com/amohamma8029/devlog/internal/session"
	"github.com/amohamma8029/devlog/internal/store"
	"github.com/amohamma8029/devlog/internal/todo"
	"github.com/spf13/cobra"
)

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
			cfg, err := loadRuntimeConfig()
			if err != nil {
				return err
			}
			formatter, err := internalconfig.NewDisplayTimeFormatter(cfg.Display)
			if err != nil {
				return err
			}

			todos, err := loadStatusTodos(root, active.ID, active.Branch)
			if err != nil {
				return err
			}

			_, err = fmt.Fprint(cmd.OutOrStdout(), renderStatus(active, events, number, time.Now().UTC(), formatter, todos))
			return err
		},
	}

	cmd.Flags().IntVarP(&number, "number", "n", 10, "Number of recent events to show (0 shows all)")

	return cmd
}

func renderStatus(active *store.SessionRecord, events []store.SessionEvent, number int, now time.Time, formatter internalconfig.DisplayTimeFormatter, todos []todo.Item) string {
	var b strings.Builder

	b.WriteString(cliSessionTitleWithID(statusSessionTitle(events, active.ID), active.ID))
	b.WriteByte('\n')
	writeCLIBranchField(&b, active.Branch, false)
	writeCLIField(&b, "author", formatStatusAuthor(active.Author, active.Email))
	writeCLIField(&b, "started", formatter.DateTime(active.Started))
	writeCLIField(&b, "duration", formatStatusDuration(now.Sub(active.Started.UTC())))

	b.WriteString("\n")
	if number == 0 {
		b.WriteString(cliTitleStyle.Render("Recent events (all)"))
		b.WriteByte('\n')
	} else {
		b.WriteString(cliTitleStyle.Render(fmt.Sprintf("Recent events (last %d)", number)))
		b.WriteByte('\n')
	}
	writeRecentStatusEvents(&b, recentStatusEvents(events, number), formatter)

	b.WriteString("\n")
	b.WriteString(cliBlockerTitle("Blockers"))
	b.WriteByte('\n')
	writeStatusBlockers(&b, events, formatter)

	b.WriteString("\n")
	b.WriteString(cliTodoListHeadingStyle.Render("Todo List"))
	b.WriteByte('\n')
	writeStatusTodos(&b, todos)

	return b.String()
}

// loadStatusTodos returns all todos relevant to the active session/branch,
// ordered completed-first. A missing todo file is treated as "no todos" rather
// than an error so the status command never fails solely because the todo log
// has not been initialised yet.
func loadStatusTodos(root, sessionID, branch string) ([]todo.Item, error) {
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
	return orderByCompletedFirst(items), nil
}

func orderByCompletedFirst(items []todo.Item) []todo.Item {
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
	return ordered
}

func writeStatusTodos(b *strings.Builder, items []todo.Item) {
	if len(items) == 0 {
		b.WriteString("  None\n")
		return
	}
	completedStarted := false
	openStarted := false
	for _, item := range items {
		if item.Status == todo.StatusDone && !completedStarted {
			b.WriteString("  " + cliTodoListSubheadingStyle.Render("Completed") + "\n")
			completedStarted = true
		}
		if item.Status == todo.StatusOpen && !openStarted {
			if completedStarted {
				b.WriteByte('\n')
			}
			b.WriteString("  " + cliTodoListSubheadingStyle.Render("Open") + "\n")
			openStarted = true
		}
		writeStatusTodoRow(b, item)
	}
}

func writeStatusTodoRow(b *strings.Builder, item todo.Item) {
	b.WriteString("    ")
	text := oneLineTodoText(item.Text)
	if item.Status == todo.StatusDone {
		b.WriteString(cliTodoDoneCheckboxStyle.Render("[x]"))
		b.WriteByte(' ')
		b.WriteString(cliTodoCompletedTextStyle.Render(text))
		b.WriteByte('\n')
		return
	}
	b.WriteString(cliTodoOpenCheckboxStyle.Render("[ ]"))
	b.WriteByte(' ')
	b.WriteString(text)
	b.WriteByte('\n')
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
		if event.Type == "Start" || event.IsDeleted {
			continue
		}
		visible = append(visible, event)
	}

	if number == 0 || number >= len(visible) {
		return visible
	}

	return visible[len(visible)-number:]
}

func writeRecentStatusEvents(b *strings.Builder, events []store.SessionEvent, formatter internalconfig.DisplayTimeFormatter) {
	if len(events) == 0 {
		b.WriteString("  None\n")
		return
	}

	for i := len(events) - 1; i >= 0; i-- {
		index := len(events) - i
		writeStatusEvent(b, events[i], formatter, index)
	}
}

func writeStatusEvent(b *strings.Builder, event store.SessionEvent, formatter internalconfig.DisplayTimeFormatter, index int) {
	var line string
	if event.Time.IsZero() {
		line = cliBulletLine(fmt.Sprintf("[%d] %s: %s", index, event.Type, oneLineStatusBody(event.Body)))
	} else {
		line = cliBulletLine(fmt.Sprintf("[%d] %s %s: %s", index, formatStatusEventTime(event.Time, formatter), event.Type, oneLineStatusBody(event.Body)))
	}
	if !event.CorrectedAt.IsZero() {
		line += " " + cliMutedStyle.Render(fmt.Sprintf("(modified %s)", formatStatusEventTime(event.CorrectedAt, formatter)))
	}
	b.WriteString(cliEventText(event.Type, line))
	b.WriteByte('\n')
}

func writeStatusBlockers(b *strings.Builder, events []store.SessionEvent, formatter internalconfig.DisplayTimeFormatter) {
	found := false
	for _, event := range events {
		if event.Type != "Blocker" || event.IsDeleted {
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
		line = cliBulletLine(fmt.Sprintf("%s: %s", formatStatusEventTime(event.Time, formatter), oneLineStatusBody(event.Body)))
		b.WriteString(cliBlockerStyle.Render(line))
		b.WriteByte('\n')
	}

	if !found {
		b.WriteString("  None\n")
	}
}

func formatStatusEventTime(t time.Time, formatter internalconfig.DisplayTimeFormatter) string {
	return formatter.EventTime(t)
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
