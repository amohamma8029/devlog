// Package git provides lightweight git repository helpers.
package git

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

const (
	authorNameEnv  = "DEVLOG_AUTHOR_NAME"
	authorEmailEnv = "DEVLOG_AUTHOR_EMAIL"
)

// RepoRoot returns the absolute root path of the current git repository.
func RepoRoot() (string, error) {
	root, stderr, err := runGit("rev-parse", "--show-toplevel")
	if err != nil {
		return "", repoCommandError("RepoRoot", stderr, err)
	}
	if root == "" {
		return "", fmt.Errorf("RepoRoot: git did not return a repository root")
	}

	return root, nil
}

// CurrentBranch returns the current git branch name.
func CurrentBranch() (string, error) {
	branch, stderr, err := runGit("branch", "--show-current")
	if err != nil {
		return "", repoCommandError("CurrentBranch", stderr, err)
	}
	if branch == "" {
		return "", fmt.Errorf("CurrentBranch: unable to determine current branch. devlog requires a named git branch; detached HEAD is not supported")
	}

	return branch, nil
}

// AuthorIdentity returns the configured author name and email, with env fallback.
func AuthorIdentity() (string, string, error) {
	name, nameErr := gitConfigValue("user.name")
	if isGitUnavailable(nameErr) {
		return "", "", fmt.Errorf("AuthorIdentity: %w", nameErr)
	}

	email, emailErr := gitConfigValue("user.email")
	if isGitUnavailable(emailErr) {
		return "", "", fmt.Errorf("AuthorIdentity: %w", emailErr)
	}

	if name == "" {
		name = strings.TrimSpace(os.Getenv(authorNameEnv))
	}
	if email == "" {
		email = strings.TrimSpace(os.Getenv(authorEmailEnv))
	}

	if name == "" && email == "" {
		if nameErr != nil {
			return "", "", fmt.Errorf("AuthorIdentity: read git config user.name: %w", nameErr)
		}
		if emailErr != nil {
			return "", "", fmt.Errorf("AuthorIdentity: read git config user.email: %w", emailErr)
		}
		return "", "", fmt.Errorf("AuthorIdentity: author identity is not configured. Set git config user.name/user.email or DEVLOG_AUTHOR_NAME/DEVLOG_AUTHOR_EMAIL")
	}

	return name, email, nil
}

func gitConfigValue(key string) (string, error) {
	value, stderr, err := runGit("config", key)
	if err != nil {
		if unavailable := gitUnavailableError(err); unavailable != nil {
			return "", unavailable
		}
		if value == "" && stderr == "" {
			return "", nil
		}
		return "", fmt.Errorf("git config %s failed: %s", key, commandFailure(stderr, err))
	}

	return value, nil
}

func runGit(args ...string) (string, string, error) {
	cmd := exec.Command("git", args...)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	return strings.TrimSpace(stdout.String()), strings.TrimSpace(stderr.String()), err
}

func repoCommandError(op, stderr string, err error) error {
	if unavailable := gitUnavailableError(err); unavailable != nil {
		return fmt.Errorf("%s: %w", op, unavailable)
	}
	if isNotRepository(stderr) {
		return fmt.Errorf("%s: you are not inside a git repository. devlog must be run from within a repo to anchor .devlog/sessions/", op)
	}

	return fmt.Errorf("%s: git command failed: %s", op, commandFailure(stderr, err))
}

func commandFailure(stderr string, err error) string {
	if stderr != "" {
		return fmt.Sprintf("%s (%v)", stderr, err)
	}
	return err.Error()
}

func isNotRepository(stderr string) bool {
	return strings.Contains(strings.ToLower(stderr), "not a git repository")
}

type gitUnavailable struct{}

func (gitUnavailable) Error() string {
	return "git is not installed or not on your PATH. devlog requires git to record session context"
}

func gitUnavailableError(err error) error {
	if err == nil {
		return nil
	}

	var execErr *exec.Error
	if errors.As(err, &execErr) || errors.Is(err, exec.ErrNotFound) {
		return gitUnavailable{}
	}

	return nil
}

func isGitUnavailable(err error) bool {
	if err == nil {
		return false
	}

	var unavailable gitUnavailable
	return errors.As(err, &unavailable)
}
