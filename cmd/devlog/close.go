package main

import (
	"fmt"

	internalgit "github.com/amohamma8029/devlog/internal/git"
	"github.com/amohamma8029/devlog/internal/session"
	"github.com/amohamma8029/devlog/internal/store"
	"github.com/amohamma8029/devlog/internal/todo"
	"github.com/spf13/cobra"
)

func newCloseCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "close",
		Short:        "Close the active session.",
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

			active, err := session.FindActiveSession(s)
			if err != nil {
				return err
			}

			title := sessionTitle(s, active.ID)

			sessionID := active.ID

			if err := session.CloseActiveSession(s); err != nil {
				return err
			}

			if ts, err := todo.NewStore(root); err == nil {
				ts.ClearSessionAttribution(sessionID)
			}

			_, err = fmt.Fprint(cmd.OutOrStdout(), renderCLISessionConfirmation("Closed session", "", false, title, sessionID, active.Branch, true))
			return err
		},
	}

	return cmd
}
