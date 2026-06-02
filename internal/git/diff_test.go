package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDiffSinceWithCommitsAfterStart(t *testing.T) {
	requireGit(t)

	dir := t.TempDir()
	t.Chdir(dir)
	runGitIn(t, dir, "init")
	runGitIn(t, dir, "checkout", "-b", "feat/test")

	writeTestFile(t, dir, "README.md", "# Test Repo")
	commitWithDate(t, dir, "2000-01-01T00:00:00Z", "initial commit")

	// Ensure session start is strictly before the second commit.
	sessionStart := time.Now().UTC()

	time.Sleep(1500 * time.Millisecond)

	writeTestFile(t, dir, "src/file.go", "package main\n")
	runGitIn(t, dir, "add", "src/file.go")
	runGitIn(t, dir, "commit", "-m", "add file.go")

	diff, err := DiffSince(sessionStart)
	if err != nil {
		t.Fatalf("DiffSince failed: %v", err)
	}

	if !strings.Contains(diff, "src/file.go") {
		t.Errorf("expected diff to contain src/file.go, got:\n%s", diff)
	}
	if strings.Contains(diff, "README.md") {
		t.Errorf("README.md was committed before session start, should not appear in diff")
	}
}

func TestDiffSinceUncommittedChanges(t *testing.T) {
	requireGit(t)

	dir := t.TempDir()
	t.Chdir(dir)
	runGitIn(t, dir, "init")
	runGitIn(t, dir, "checkout", "-b", "feat/test")
	writeTestFile(t, dir, "README.md", "# Repo")
	commitWithDate(t, dir, "2000-01-01T00:00:00Z", "initial")

	sessionStart := time.Now().UTC()

	writeTestFile(t, dir, "README.md", "# Modified Repo")

	diff, err := DiffSince(sessionStart)
	if err != nil {
		t.Fatalf("DiffSince failed: %v", err)
	}

	if !strings.Contains(diff, "README.md") {
		t.Errorf("expected diff to contain uncommitted change, got:\n%s", diff)
	}
}

func TestDiffSinceUntrackedFile(t *testing.T) {
	requireGit(t)

	dir := t.TempDir()
	t.Chdir(dir)
	runGitIn(t, dir, "init")
	runGitIn(t, dir, "checkout", "-b", "feat/test")
	writeTestFile(t, dir, "README.md", "# Repo")
	commitWithDate(t, dir, "2000-01-01T00:00:00Z", "initial")

	sessionStart := time.Now().UTC()

	writeTestFile(t, dir, "newfile.go", "package main\nfunc main() {}")

	diff, err := DiffSince(sessionStart)
	if err != nil {
		t.Fatalf("DiffSince failed: %v", err)
	}

	if !strings.Contains(diff, "newfile.go") {
		t.Errorf("expected diff to contain untracked file, got:\n%s", diff)
	}
}

func TestDiffSinceUntrackedSkippedDevlog(t *testing.T) {
	requireGit(t)

	dir := t.TempDir()
	t.Chdir(dir)
	runGitIn(t, dir, "init")
	runGitIn(t, dir, "checkout", "-b", "feat/test")
	writeTestFile(t, dir, "README.md", "# Repo")
	commitWithDate(t, dir, "2000-01-01T00:00:00Z", "initial")

	sessionStart := time.Now().UTC()

	writeTestFile(t, dir, ".devlog/sessions/test.md", "test content")

	diff, err := DiffSince(sessionStart)
	if err != nil {
		t.Fatalf("DiffSince failed: %v", err)
	}

	if strings.Contains(diff, ".devlog") {
		t.Errorf(".devlog/ entries should be excluded from diff, got:\n%s", diff)
	}
}

func TestDiffSinceUntrackedSkippedSecretFile(t *testing.T) {
	requireGit(t)

	dir := t.TempDir()
	t.Chdir(dir)
	runGitIn(t, dir, "init")
	runGitIn(t, dir, "checkout", "-b", "feat/test")
	writeTestFile(t, dir, "README.md", "# Repo")
	commitWithDate(t, dir, "2000-01-01T00:00:00Z", "initial")

	sessionStart := time.Now().UTC()

	writeTestFile(t, dir, ".env", "DEVLOG_TEST_VALUE=example\n")
	writeTestFile(t, dir, "notes.txt", "safe note\n")

	diff, err := DiffSince(sessionStart)
	if err != nil {
		t.Fatalf("DiffSince failed: %v", err)
	}

	if !strings.Contains(diff, "notes.txt") {
		t.Errorf("expected diff to contain non-secret untracked file, got:\n%s", diff)
	}
	if strings.Contains(diff, ".env") || strings.Contains(diff, "DEVLOG_TEST_VALUE") {
		t.Errorf("untracked .env file should be excluded from diff, got:\n%s", diff)
	}
}

func TestDiffSinceDevlogStrippedFromCommittedDiff(t *testing.T) {
	requireGit(t)

	dir := t.TempDir()
	t.Chdir(dir)
	runGitIn(t, dir, "init")
	runGitIn(t, dir, "checkout", "-b", "feat/test")
	writeTestFile(t, dir, "README.md", "# Repo")
	commitWithDate(t, dir, "2000-01-01T00:00:00Z", "initial")

	sessionStart := time.Now().UTC()

	writeTestFile(t, dir, ".devlog/sessions/sess.md", "test")
	runGitIn(t, dir, "add", ".devlog/sessions/sess.md")
	runGitIn(t, dir, "commit", "-m", "add devlog file")

	diff, err := DiffSince(sessionStart)
	if err != nil {
		t.Fatalf("DiffSince failed: %v", err)
	}

	if strings.Contains(diff, ".devlog") {
		t.Errorf(".devlog/ entries should be stripped, got:\n%s", diff)
	}
}

func TestDiffSinceNoChanges(t *testing.T) {
	requireGit(t)

	dir := t.TempDir()
	t.Chdir(dir)
	runGitIn(t, dir, "init")
	runGitIn(t, dir, "checkout", "-b", "feat/test")
	writeTestFile(t, dir, "README.md", "# Repo")
	commitWithDate(t, dir, "2000-01-01T00:00:00Z", "initial")

	sessionStart := time.Now().UTC()

	diff, err := DiffSince(sessionStart)
	if err != nil {
		t.Fatalf("DiffSince failed: %v", err)
	}

	if diff != "" {
		t.Errorf("expected empty diff, got:\n%s", diff)
	}
}

func TestDiffSinceBeforeFirstCommit(t *testing.T) {
	requireGit(t)

	dir := t.TempDir()
	t.Chdir(dir)
	runGitIn(t, dir, "init")
	runGitIn(t, dir, "checkout", "-b", "feat/test")

	sessionStart := time.Now().UTC()

	writeTestFile(t, dir, "file.go", "package main\n")

	diff, err := DiffSince(sessionStart)
	if err != nil {
		t.Fatalf("DiffSince failed: %v", err)
	}

	if !strings.Contains(diff, "file.go") {
		t.Errorf("expected diff to contain file.go when no prior commits, got:\n%s", diff)
	}
}

func TestDiffSinceTrackedModifiedSecretEnv(t *testing.T) {
	requireGit(t)

	dir := t.TempDir()
	t.Chdir(dir)
	runGitIn(t, dir, "init")
	runGitIn(t, dir, "checkout", "-b", "feat/test")

	writeTestFile(t, dir, ".env", "SECRET_KEY=old-value\n")
	writeTestFile(t, dir, "src/main.go", "package main\n")
	commitWithDate(t, dir, "2000-01-01T00:00:00Z", "initial commit")

	sessionStart := time.Now().UTC()

	time.Sleep(1500 * time.Millisecond)

	writeTestFile(t, dir, ".env", "SECRET_KEY=new-leaked-value\n")
	writeTestFile(t, dir, "src/main.go", "package main\nfunc main() {}\n")

	diff, err := DiffSince(sessionStart)
	if err != nil {
		t.Fatalf("DiffSince failed: %v", err)
	}

	if !strings.Contains(diff, "src/main.go") {
		t.Errorf("expected diff to contain src/main.go, got:\n%s", diff)
	}
	if strings.Contains(diff, ".env") || strings.Contains(diff, "SECRET_KEY") || strings.Contains(diff, "old-value") || strings.Contains(diff, "new-leaked-value") {
		t.Errorf("tracked .env modification should be excluded from diff, got:\n%s", diff)
	}
}

func TestDiffSinceTrackedModifiedSecretPem(t *testing.T) {
	requireGit(t)

	dir := t.TempDir()
	t.Chdir(dir)
	runGitIn(t, dir, "init")
	runGitIn(t, dir, "checkout", "-b", "feat/test")

	writeTestFile(t, dir, "certs/server.pem", "-----BEGIN CERTIFICATE-----\nold\n-----END CERTIFICATE-----\n")
	writeTestFile(t, dir, "src/main.go", "package main\n")
	commitWithDate(t, dir, "2000-01-01T00:00:00Z", "initial commit")

	sessionStart := time.Now().UTC()

	time.Sleep(1500 * time.Millisecond)

	writeTestFile(t, dir, "certs/server.pem", "-----BEGIN CERTIFICATE-----\nleaked\n-----END CERTIFICATE-----\n")
	writeTestFile(t, dir, "src/main.go", "package main\nfunc main() {}\n")

	diff, err := DiffSince(sessionStart)
	if err != nil {
		t.Fatalf("DiffSince failed: %v", err)
	}

	if !strings.Contains(diff, "src/main.go") {
		t.Errorf("expected diff to contain src/main.go, got:\n%s", diff)
	}
	if strings.Contains(diff, "server.pem") || strings.Contains(diff, "CERTIFICATE") {
		t.Errorf("tracked .pem modification should be excluded from diff, got:\n%s", diff)
	}
}

func TestDiffSinceCommitAfterStartInEmptyRepo(t *testing.T) {
	requireGit(t)

	dir := t.TempDir()
	t.Chdir(dir)
	runGitIn(t, dir, "init")

	sessionStart := time.Now().UTC()

	time.Sleep(1500 * time.Millisecond)

	writeTestFile(t, dir, "src/app.go", "package main\nfunc main() {}\n")
	runGitIn(t, dir, "add", "src/app.go")
	runGitIn(t, dir, "commit", "-m", "first commit after session start")

	diff, err := DiffSince(sessionStart)
	if err != nil {
		t.Fatalf("DiffSince failed: %v", err)
	}

	if !strings.Contains(diff, "src/app.go") {
		t.Errorf("expected diff to contain src/app.go committed after session start, got:\n%s", diff)
	}
}

func TestDiffSinceTrackedModifiedSecretDirectory(t *testing.T) {
	requireGit(t)

	dir := t.TempDir()
	t.Chdir(dir)
	runGitIn(t, dir, "init")
	runGitIn(t, dir, "checkout", "-b", "feat/test")

	writeTestFile(t, dir, "secrets/config.yml", "api_key: old-directory-secret\n")
	writeTestFile(t, dir, "src/main.go", "package main\n")
	commitWithDate(t, dir, "2000-01-01T00:00:00Z", "initial commit")

	sessionStart := time.Now().UTC()

	writeTestFile(t, dir, "secrets/config.yml", "api_key: new-directory-secret\n")
	writeTestFile(t, dir, "src/main.go", "package main\nfunc main() {}\n")

	diff, err := DiffSince(sessionStart)
	if err != nil {
		t.Fatalf("DiffSince failed: %v", err)
	}

	if !strings.Contains(diff, "src/main.go") {
		t.Errorf("expected diff to contain src/main.go, got:\n%s", diff)
	}
	if strings.Contains(diff, "secrets/config.yml") || strings.Contains(diff, "old-directory-secret") || strings.Contains(diff, "new-directory-secret") {
		t.Errorf("tracked file under secret-bearing directory should be excluded from diff, got:\n%s", diff)
	}
}

// ── Helpers ──────────────────────────────────────────────────────────────────

func runGitIn(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s failed: %v\n%s", strings.Join(args, " "), err, string(out))
	}
}

func commitWithDate(t *testing.T, dir, date, message string) {
	t.Helper()
	runGitIn(t, dir, "add", ".")
	cmd := exec.Command("git", "commit", "-m", message)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_COMMITTER_DATE="+date,
		"GIT_AUTHOR_DATE="+date,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git commit at %s failed: %v\n%s", date, err, string(out))
	}
}

func writeTestFile(t *testing.T, dir, path, content string) {
	t.Helper()
	fullPath := filepath.Join(dir, path)
	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
		t.Fatalf("write file failed: %v", err)
	}
}
