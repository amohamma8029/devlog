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

func newOpenCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "open <message>",
		Short:        "Open a new session.",
		Args:         cobra.ArbitraryArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			message := strings.TrimSpace(strings.Join(args, " "))
			if message == "" {
				return fmt.Errorf(`open requires a message describing what you will work on. Run "devlog open <message>" to start a session.`)
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

			if err := session.OpenSession(s, sess, message); err != nil {
				return err
			}

			_, err = fmt.Fprintf(cmd.OutOrStdout(), "Opened session %s on branch %s\n", sess.ID, sess.Branch)
			return err
		},
	}

	return cmd
}
