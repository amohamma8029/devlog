package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	internalgit "github.com/amo/devlog/internal/git"
	"github.com/amo/devlog/internal/handoff"
	"github.com/amo/devlog/internal/session"
	"github.com/amo/devlog/internal/store"
	"github.com/spf13/cobra"
)

func newHandoffCommand() *cobra.Command {
	var output string
	var noDiff bool

	cmd := &cobra.Command{
		Use:          "handoff [session-id]",
		Short:        "Generate a narrative handoff summary of a session.",
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

			var rec store.SessionRecord

			if len(args) > 0 {
				if len(args) > 1 {
					return fmt.Errorf(`too many arguments. Run "devlog handoff" with no arguments for the active session, or "devlog handoff <session-id>" for a specific session.`)
				}
				rec, err = s.GetSession(args[0])
				if err != nil {
					return err
				}
			} else {
				active, err := session.FindActiveSession(s)
				if err != nil {
					return err
				}
				rec = *active
			}

			sessionContent, err := readSessionContent(root, rec.ID)
			if err != nil {
				return err
			}

			diff, err := internalgit.DiffSince(rec.Started.UTC())
			if err != nil {
				return err
			}

			handoffText, err := handoff.GenerateWithOptions(sessionContent, diff, handoff.GenerateOptions{ExcludeRawDiff: noDiff})
			if err != nil {
				return fmt.Errorf("handoff: generate: %w", err)
			}

			savePath, err := resolveHandoffOutputPath(root, rec.ID, output)
			if err != nil {
				return err
			}

			if _, err := os.Stat(savePath); err == nil {
				return fmt.Errorf("handoff: file already exists: %s", savePath)
			}

			dir := filepath.Dir(savePath)
			if err := os.MkdirAll(dir, 0755); err != nil {
				return fmt.Errorf("handoff: create output directory: %w", err)
			}
			f, err := os.OpenFile(savePath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
			if err != nil {
				if os.IsExist(err) {
					return fmt.Errorf("handoff: file already exists: %s", savePath)
				}
				return fmt.Errorf("handoff: create output file: %w", err)
			}
			if _, err := f.WriteString(handoffText); err != nil {
				f.Close()
				return fmt.Errorf("handoff: write output file: %w", err)
			}
			if err := f.Close(); err != nil {
				return fmt.Errorf("handoff: close output file: %w", err)
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "Handoff written to %s\n", savePath)
			return err
		},
	}

	cmd.Flags().StringVarP(&output, "output", "o", "", "Write handoff to a file instead of stdout")
	cmd.Flags().BoolVar(&noDiff, "no-diff", false, "Exclude raw diff blocks from the handoff")

	return cmd
}

func resolveHandoffOutputPath(root, sessionID, output string) (string, error) {
	handoffsDir := filepath.Join(root, ".devlog", "handoffs")

	if output == "" {
		return filepath.Join(handoffsDir, sessionID+".md"), nil
	}

	name := strings.TrimSpace(output)
	if name == "" {
		return "", fmt.Errorf("handoff: filename cannot be empty")
	}
	if !isValidFilename(name) {
		return "", fmt.Errorf("handoff: invalid filename: path separators or '..' not allowed")
	}

	return filepath.Join(handoffsDir, name+".md"), nil
}

func isValidFilename(name string) bool {
	if strings.Contains(name, "..") {
		return false
	}
	if strings.ContainsAny(name, `/\\`) {
		return false
	}
	return true
}

func readSessionContent(root, sessionID string) (string, error) {
	path, err := sessionFilePath(root, sessionID)
	if err != nil {
		return "", err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("handoff: session file not found: %s", path)
		}
		return "", fmt.Errorf("handoff: read session file: %w", err)
	}

	content := strings.ReplaceAll(string(data), "\r\n", "\n")
	return content, nil
}

func sessionFilePath(root, sessionID string) (string, error) {
	if strings.TrimSpace(sessionID) == "" {
		return "", fmt.Errorf("handoff: session ID is empty")
	}
	return filepath.Join(root, ".devlog", "sessions", sessionID+".md"), nil
}
