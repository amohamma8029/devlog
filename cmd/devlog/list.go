package main

import (
	"fmt"
	"strings"
	"time"

	internalgit "github.com/amo/devlog/internal/git"
	"github.com/amo/devlog/internal/store"
	"github.com/mattn/go-runewidth"
	"github.com/spf13/cobra"
)

func newListCommand() *cobra.Command {
	var activeOnly bool
	var branchFilter string

	cmd := &cobra.Command{
		Use:          "list",
		Short:        "List sessions.",
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := internalgit.RepoRoot()
			if err != nil {
				return err
			}

			s, err := store.New(root)
			if err != nil {
				return err
			}

			records, err := s.ListSessions()
			if err != nil {
				return err
			}

			filtered := filterListSessions(records, activeOnly, branchFilter)

			_, err = fmt.Fprint(cmd.OutOrStdout(), renderListTable(filtered, root, time.Now().UTC()))
			return err
		},
	}

	cmd.Flags().BoolVar(&activeOnly, "active", false, "Show only active sessions")
	cmd.Flags().StringVar(&branchFilter, "branch", "", "Filter sessions by branch name")

	return cmd
}

func filterListSessions(records []store.SessionRecord, activeOnly bool, branch string) []store.SessionRecord {
	var filtered []store.SessionRecord
	for _, r := range records {
		if activeOnly && r.Closed {
			continue
		}
		if branch != "" && !strings.Contains(strings.ToLower(r.Branch), strings.ToLower(branch)) {
			continue
		}
		filtered = append(filtered, r)
	}
	return filtered
}

func padField(s string, width int) string {
	w := runewidth.StringWidth(s)
	if w >= width {
		return s
	}
	return s + strings.Repeat(" ", width-w)
}

func renderListTable(records []store.SessionRecord, root string, now time.Time) string {
	if len(records) == 0 {
		return "No sessions found.\n"
	}

	s, err := store.New(root)
	if err != nil {
		s = nil
	}

	var b strings.Builder

	b.WriteString(padField("TITLE", 22) + "  " +
		padField("BRANCH", 20) + "  " +
		padField("STATUS", 7) + "  " +
		padField("STARTED", 20) + "  " +
		"DURATION\n")

	for _, r := range records {
		status := "active"
		if r.Closed {
			status = "closed"
		}

		b.WriteString(
			padField(truncateListField(listTitle(s, r.ID), 22), 22) + "  " +
				padField(truncateListField(r.Branch, 20), 20) + "  " +
				padField(status, 7) + "  " +
				padField(r.Started.Format(time.RFC3339), 20) + "  " +
				computeListDuration(r, root, now) + "\n")
	}

	return b.String()
}

func listTitle(s *store.Store, sessionID string) string {
	if s == nil {
		return sessionID
	}
	title, err := s.ReadSessionStartMessage(sessionID)
	if err != nil || title == "" {
		return sessionID
	}

	return title
}

func truncateListField(s string, max int) string {
	if runewidth.StringWidth(s) <= max {
		return s
	}
	return runewidth.Truncate(s, max, "\u2026")
}

func computeListDuration(sr store.SessionRecord, root string, now time.Time) string {
	if !sr.Closed {
		return formatStatusDuration(now.Sub(sr.Started))
	}

	s, err := store.New(root)
	if err != nil {
		return "-"
	}

	body, err := s.ReadSessionBody(sr.ID)
	if err != nil {
		return "-"
	}

	stop := lastStopTime(store.ParseSessionEvents(body))
	if stop.IsZero() {
		return "-"
	}

	d := stop.Sub(sr.Started.UTC())
	if d < 0 {
		return "-"
	}

	return formatStatusDuration(d)
}

func lastStopTime(events []store.SessionEvent) time.Time {
	var stop time.Time
	for _, event := range events {
		if event.Type == "Stop" {
			stop = event.Time
		}
	}
	return stop
}
