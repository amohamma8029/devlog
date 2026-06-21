package main

import (
	"fmt"
	"strconv"
	"strings"

	internalgit "github.com/amo/devlog/internal/git"
	"github.com/amo/devlog/internal/session"
	"github.com/amo/devlog/internal/store"
	"github.com/spf13/cobra"
)

func newCorrectCommand() *cobra.Command {
	var message string
	var deleteFlag bool

	cmd := &cobra.Command{
		Use:          "correct <event-index> [message]",
		Short:        "Correct an existing note or blocker in the active session.",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 1 {
				return fmt.Errorf("event index is required")
			}

			index, err := strconv.Atoi(args[0])
			if err != nil || index < 1 {
				return fmt.Errorf("event index must be a positive integer")
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

			var correctable []store.SessionEvent
			for _, e := range events {
				if e.Type == "Start" || e.Type == "Stop" || e.IsDeleted {
					continue
				}
				correctable = append(correctable, e)
			}

			if index > len(correctable) {
				return fmt.Errorf("event index %d out of range (session has %d correctable events)", index, len(correctable))
			}

			target := correctable[len(correctable)-index]
			if target.Type == "Start" || target.Type == "Stop" {
				return fmt.Errorf("event at index %d is a %s and cannot be corrected", index, target.Type)
			}

			if deleteFlag {
				correctionBody := fmt.Sprintf("%s %02d:%02d", target.Type, target.Time.UTC().Hour(), target.Time.UTC().Minute())
				if err := session.AppendEventToActiveSession(s, "Correction", correctionBody); err != nil {
					return err
				}
				title := sessionTitle(s, active.ID)
				_, err = fmt.Fprint(cmd.OutOrStdout(), renderCLISessionConfirmation(renderCLIDeletedEvent(index), correctionBody, false, title, active.ID, active.Branch, false))
				return err
			}

			positional := strings.TrimSpace(strings.Join(args[1:], " "))
			newBody, err := resolveBody(cmd, message, []string{positional}, "corrected "+target.Type)
			if err != nil {
				return err
			}

			correctionBody := fmt.Sprintf("%s %02d:%02d\n%s", target.Type, target.Time.UTC().Hour(), target.Time.UTC().Minute(), newBody)
			if err := session.AppendEventToActiveSession(s, "Correction", correctionBody); err != nil {
				return err
			}

			title := sessionTitle(s, active.ID)
			_, err = fmt.Fprint(cmd.OutOrStdout(), renderCLISessionConfirmation(renderCLICorrectedEvent(index), correctionBody, false, title, active.ID, active.Branch, false))
			return err
		},
	}

	cmd.Flags().StringVarP(&message, "message", "m", "", "Corrected body text")
	cmd.Flags().BoolVar(&deleteFlag, "delete", false, "Delete the event instead of correcting it")

	return cmd
}

func renderCLICorrectedEvent(index int) string {
	return fmt.Sprintf("Corrected event %d", index)
}

func renderCLIDeletedEvent(index int) string {
	return fmt.Sprintf("Deleted event %d", index)
}
