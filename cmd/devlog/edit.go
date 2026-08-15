package main

import (
	"fmt"
	"strconv"
	"strings"

	internalgit "github.com/amohamma8029/devlog/internal/git"
	"github.com/amohamma8029/devlog/internal/session"
	"github.com/amohamma8029/devlog/internal/store"
	"github.com/spf13/cobra"
)

func newEditCommand() *cobra.Command {
	var message string
	var deleteFlag bool

	cmd := &cobra.Command{
		Use:          "edit <event-index> [message]",
		Short:        "Edit an existing note or blocker in the active session.",
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

			var editable []store.SessionEvent
			for _, e := range events {
				if e.Type == "Start" || e.Type == "Stop" || e.IsDeleted {
					continue
				}
				editable = append(editable, e)
			}

			if index > len(editable) {
				return fmt.Errorf("event index %d out of range (session has %d editable events)", index, len(editable))
			}

			target := editable[len(editable)-index]
			if target.Type == "Start" || target.Type == "Stop" {
				return fmt.Errorf("event at index %d is a %s and cannot be edited", index, target.Type)
			}

			if deleteFlag {
				editBody := store.FormatEditBody(target, "delete", "")
				if err := session.AppendEventToActiveSession(s, "Edit", editBody); err != nil {
					return err
				}
				title := sessionTitle(s, active.ID)
				_, err = fmt.Fprint(cmd.OutOrStdout(), renderCLISessionConfirmation(renderCLIDeletedEvent(index), editBody, false, title, active.ID, active.Branch, false))
				return err
			}

			positional := strings.TrimSpace(strings.Join(args[1:], " "))
			newBody, err := resolveBody(cmd, message, []string{positional}, "edit "+target.Type)
			if err != nil {
				return err
			}

			editBody := store.FormatEditBody(target, "update", newBody)
			if err := session.AppendEventToActiveSession(s, "Edit", editBody); err != nil {
				return err
			}

			title := sessionTitle(s, active.ID)
			_, err = fmt.Fprint(cmd.OutOrStdout(), renderCLISessionConfirmation(renderCLIEditedEvent(index), editBody, false, title, active.ID, active.Branch, false))
			return err
		},
	}

	cmd.Flags().StringVarP(&message, "message", "m", "", "Edited body text")
	cmd.Flags().BoolVar(&deleteFlag, "delete", false, "Delete the event instead of editing it")

	return cmd
}

func renderCLIEditedEvent(index int) string {
	return fmt.Sprintf("Edited event %d", index)
}

func renderCLIDeletedEvent(index int) string {
	return fmt.Sprintf("Deleted event %d", index)
}
