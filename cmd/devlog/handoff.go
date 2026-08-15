package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	internalgit "github.com/amohamma8029/devlog/internal/git"
	"github.com/amohamma8029/devlog/internal/handoff"
	"github.com/amohamma8029/devlog/internal/session"
	"github.com/amohamma8029/devlog/internal/store"
	"github.com/amohamma8029/devlog/internal/todo"
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

			cfg, err := loadRuntimeConfig()
			if err != nil {
				return err
			}

			diff, err := internalgit.DiffSinceWithContext(rec.Started.UTC(), cfg.Handoff.DiffContextLines)
			if err != nil {
				return err
			}

			todos, err := loadHandoffTodos(root, rec.ID, rec.Branch)
			if err != nil {
				return err
			}

			handoffText, err := handoff.GenerateWithOptions(sessionContent, diff, handoff.GenerateOptions{ExcludeRawDiff: noDiff, Todos: todos})
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
			title := sessionTitle(s, rec.ID)
			_, err = fmt.Fprint(cmd.OutOrStdout(), renderCLIHandoffConfirmation(displayHandoffPath(root, savePath), title, rec.ID, rec.Branch, rec.Closed))
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

// loadHandoffTodos returns all todos relevant to the given session/branch,
// ordered completed-first. A missing todo file is treated as "no todos" rather
// than an error so the handoff command never fails solely because the todo log
// has not been initialised yet.
func loadHandoffTodos(root, sessionID, branch string) ([]todo.Item, error) {
	store, err := todo.NewStore(root)
	if err != nil {
		return nil, err
	}
	items, err := store.List(todo.Filter{
		IncludeOpen:     true,
		IncludeDone:     true,
		SessionID:       sessionID,
		Branch:          branch,
		MatchSessionAny: sessionID == "",
		MatchBranchAny:  branch == "",
	})
	if err != nil {
		return nil, err
	}
	return orderByCompletedFirst(items), nil
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

func displayHandoffPath(root, path string) string {
	if rel, err := filepath.Rel(root, path); err == nil && rel != "." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && rel != ".." && !filepath.IsAbs(rel) {
		return filepath.ToSlash(rel)
	}

	home, err := os.UserHomeDir()
	if err == nil && home != "" {
		if rel, err := filepath.Rel(home, path); err == nil && rel != "." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && rel != ".." && !filepath.IsAbs(rel) {
			return "~/" + filepath.ToSlash(rel)
		}
		if samePath(home, path) {
			return "~"
		}
	}

	return path
}

func samePath(a, b string) bool {
	absA, errA := filepath.Abs(a)
	absB, errB := filepath.Abs(b)
	if errA != nil || errB != nil {
		return false
	}
	return absA == absB
}
