package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestRepoRootCurrentBranchAndAuthorIdentity(t *testing.T) {
	requireGit(t)
	isolateGitConfig(t)

	root := initTestRepo(t)
	runTestGit(t, root, "config", "user.name", "Test Author")
	runTestGit(t, root, "config", "user.email", "test@example.com")

	nested := filepath.Join(root, "nested", "dir")
	if err := os.MkdirAll(nested, 0755); err != nil {
		t.Fatalf("create nested directory failed: %v", err)
	}
	t.Chdir(nested)

	gotRoot, err := RepoRoot()
	if err != nil {
		t.Fatalf("RepoRoot failed: %v", err)
	}
	if !samePath(gotRoot, root) {
		t.Fatalf("expected repo root %q, got %q", root, gotRoot)
	}

	branch, err := CurrentBranch()
	if err != nil {
		t.Fatalf("CurrentBranch failed: %v", err)
	}
	if branch != "feat/test" {
		t.Fatalf("expected branch %q, got %q", "feat/test", branch)
	}

	name, email, err := AuthorIdentity()
	if err != nil {
		t.Fatalf("AuthorIdentity failed: %v", err)
	}
	if name != "Test Author" {
		t.Fatalf("expected author name %q, got %q", "Test Author", name)
	}
	if email != "test@example.com" {
		t.Fatalf("expected author email %q, got %q", "test@example.com", email)
	}
}

func TestAuthorIdentityUsesEnvFallback(t *testing.T) {
	requireGit(t)
	isolateGitConfig(t)

	root := initTestRepo(t)
	t.Chdir(root)
	t.Setenv(authorNameEnv, "Env Author")
	t.Setenv(authorEmailEnv, "env@example.com")

	name, email, err := AuthorIdentity()
	if err != nil {
		t.Fatalf("AuthorIdentity failed: %v", err)
	}
	if name != "Env Author" {
		t.Fatalf("expected env author name %q, got %q", "Env Author", name)
	}
	if email != "env@example.com" {
		t.Fatalf("expected env author email %q, got %q", "env@example.com", email)
	}
}

func TestAuthorIdentityRequiresConfiguredIdentity(t *testing.T) {
	requireGit(t)
	isolateGitConfig(t)

	root := initTestRepo(t)
	t.Chdir(root)
	t.Setenv(authorNameEnv, "")
	t.Setenv(authorEmailEnv, "")

	_, _, err := AuthorIdentity()
	if err == nil || !strings.Contains(err.Error(), "author identity is not configured") {
		t.Fatalf("expected missing author identity error, got: %v", err)
	}
}

func TestRepoCommandsFailOutsideGitRepository(t *testing.T) {
	requireGit(t)
	isolateGitConfig(t)
	t.Chdir(t.TempDir())

	tests := []struct {
		name string
		fn   func() (string, error)
	}{
		{name: "RepoRoot", fn: RepoRoot},
		{name: "CurrentBranch", fn: CurrentBranch},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := tt.fn()
			if err == nil || !strings.Contains(err.Error(), "not inside a git repository") {
				t.Fatalf("expected not-in-repo error, got: %v", err)
			}
		})
	}
}

func TestRepoRootFailsWhenGitMissing(t *testing.T) {
	t.Chdir(t.TempDir())
	t.Setenv("PATH", "")

	_, err := RepoRoot()
	if err == nil || !strings.Contains(err.Error(), "git is not installed or not on your PATH") {
		t.Fatalf("expected missing git error, got: %v", err)
	}
}

func requireGit(t *testing.T) {
	t.Helper()

	if _, err := exec.LookPath("git"); err != nil {
		t.Skipf("git binary not available: %v", err)
	}
}

func isolateGitConfig(t *testing.T) {
	t.Helper()

	root := t.TempDir()
	globalConfig := filepath.Join(root, "global.gitconfig")
	if err := os.WriteFile(globalConfig, []byte{}, 0644); err != nil {
		t.Fatalf("write isolated git config failed: %v", err)
	}

	t.Setenv("GIT_CONFIG_GLOBAL", globalConfig)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "true")
	t.Setenv("HOME", root)
	t.Setenv("USERPROFILE", root)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, "xdg"))
	t.Setenv(authorNameEnv, "")
	t.Setenv(authorEmailEnv, "")
}

func initTestRepo(t *testing.T) string {
	t.Helper()

	root := t.TempDir()
	runTestGit(t, root, "init")
	runTestGit(t, root, "checkout", "-b", "feat/test")
	return root
}

func runTestGit(t *testing.T, dir string, args ...string) {
	t.Helper()

	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s failed: %v\n%s", strings.Join(args, " "), err, strings.TrimSpace(string(output)))
	}
}

func samePath(a, b string) bool {
	cleanA := filepath.Clean(a)
	cleanB := filepath.Clean(b)
	if runtime.GOOS == "windows" {
		return strings.EqualFold(cleanA, cleanB)
	}
	return cleanA == cleanB
}
