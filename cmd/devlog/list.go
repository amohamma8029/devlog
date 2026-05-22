package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	internalgit "github.com/amo/devlog/internal/git"
	"github.com/amo/devlog/internal/store"
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

func renderListTable(records []store.SessionRecord, root string, now time.Time) string {
	if len(records) == 0 {
		return "No sessions found.\n"
	}

	var b strings.Builder

	b.WriteString(fmt.Sprintf("%-22s  %-20s  %-7s  %-20s  %s\n", "ID", "BRANCH", "STATUS", "STARTED", "DURATION"))

	for _, r := range records {
		status := "active"
		if r.Closed {
			status = "closed"
		}

		b.WriteString(fmt.Sprintf(
			"%-22s  %-20s  %-7s  %-20s  %s\n",
			r.ID,
			truncateListField(r.Branch, 20),
			status,
			r.Started.Format(time.RFC3339),
			computeListDuration(r, root, now),
		))
	}

	return b.String()
}

func truncateListField(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-1] + "\u2026"
}

func computeListDuration(sr store.SessionRecord, root string, now time.Time) string {
	if !sr.Closed {
		return formatStatusDuration(now.Sub(sr.Started))
	}

	stopAt, err := readSessionStopTime(root, sr.ID)
	if err != nil || stopAt.IsZero() {
		return "-"
	}

	stop := time.Date(
		sr.Started.Year(), sr.Started.Month(), sr.Started.Day(),
		stopAt.Hour(), stopAt.Minute(), 0, 0, time.UTC,
	)

	if stop.Before(sr.Started) || stop.Equal(sr.Started) {
		stop = stop.Add(24 * time.Hour)
	}

	d := stop.Sub(sr.Started)
	if d < 0 {
		return "-"
	}

	return formatStatusDuration(d)
}

func readSessionStopTime(root, sessionID string) (time.Time, error) {
	body, err := readListSessionBody(root, sessionID)
	if err != nil {
		return time.Time{}, err
	}

	events := parseStatusEvents(body)
	var lastStopAt string
	for _, e := range events {
		if e.Type == "Stop" {
			lastStopAt = e.At
		}
	}
	if lastStopAt == "" {
		return time.Time{}, nil
	}

	return parseListUTCTime(lastStopAt)
}

func readListSessionBody(root, sessionID string) (string, error) {
	path := filepath.Join(root, ".devlog", "sessions", sessionID+".md")
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("list: read session file: %w", err)
	}

	content := strings.ReplaceAll(string(data), "\r\n", "\n")
	const delim = "---\n"
	if !strings.HasPrefix(content, delim) {
		return "", fmt.Errorf("list: missing opening front-matter delimiter")
	}

	parts := strings.SplitN(content[len(delim):], delim, 2)
	if len(parts) < 2 {
		return "", fmt.Errorf("list: missing closing front-matter delimiter")
	}

	return parts[1], nil
}

func parseListUTCTime(s string) (time.Time, error) {
	s = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(s), " UTC"))
	t, err := time.Parse("15:04", s)
	if err != nil {
		return time.Time{}, fmt.Errorf("list: parse stop time: %w", err)
	}
	return t, nil
}
