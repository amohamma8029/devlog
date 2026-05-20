package main

import (
	"fmt"

	internalgit "github.com/amo/devlog/internal/git"
	"github.com/amo/devlog/internal/session"
	"github.com/amo/devlog/internal/store"
	"github.com/spf13/cobra"
)

func newCloseCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "close",
		Short:        "Close the active devlog session.",
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

			if err := session.CloseActiveSession(s); err != nil {
				return err
			}

			_, err = fmt.Fprintf(cmd.OutOrStdout(), "Session %s closed.\n", active.ID)
			return err
		},
	}

	return cmd
}
