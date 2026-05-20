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

func newStartCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "start <message>",
		Short:        "Start a new devlog session.",
		Args:         cobra.MinimumNArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			message := strings.TrimSpace(strings.Join(args, " "))
			if message == "" {
				return fmt.Errorf("start message is empty")
			}

			root, err := internalgit.RepoRoot()
			if err != nil {
				return err
			}

			branch, err := internalgit.CurrentBranch()
			if err != nil {
				return err
			}

			name, email, err := internalgit.AuthorIdentity()
			if err != nil {
				return err
			}

			s, err := store.New(root)
			if err != nil {
				return err
			}

			now := time.Now().UTC()
			sess := store.Session{
				ID:      now.Format("2006-01-02T150405Z"),
				Author:  name,
				Email:   email,
				Started: now,
				Branch:  branch,
				Status:  "active",
			}

			if err := session.StartSession(s, sess, message); err != nil {
				return err
			}

			_, err = fmt.Fprintf(cmd.OutOrStdout(), "Started devlog session %s on branch %s\n", sess.ID, sess.Branch)
			return err
		},
	}

	return cmd
}
