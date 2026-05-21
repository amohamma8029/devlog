package main

import (
	"fmt"

	internalgit "github.com/amo/devlog/internal/git"
	"github.com/amo/devlog/internal/session"
	"github.com/amo/devlog/internal/store"
	"github.com/spf13/cobra"
)

func newBlockCommand() *cobra.Command {
	var message string

	cmd := &cobra.Command{
		Use:          "block [message]",
		Short:        "Log a blocker in the active session.",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			body, err := resolveBody(cmd, message, args, "blocker")
			if err != nil {
				return err
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

			if err := session.AppendEventToActiveSession(s, "Blocker", body); err != nil {
				return err
			}

			_, err = fmt.Fprintf(cmd.OutOrStdout(), "Logged blocker in session %s\n", active.ID)
			return err
		},
	}

	cmd.Flags().StringVarP(&message, "message", "m", "", "Blocker message to append")

	return cmd
}
