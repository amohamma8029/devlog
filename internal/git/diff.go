package git

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// DiffSince returns a combined git diff string showing all changes from the
// most recent commit at or before started to the current working tree.
// It includes committed changes, uncommitted changes (staged and unstaged),
// and untracked text files. A warning line is prepended if the working tree
// has merge conflicts. Entries under .devlog/ are stripped from the result.
func DiffSince(started time.Time) (string, error) {
	commit, err := findCommitBefore(started)
	if err != nil {
		return "", err
	}

	var parts []string

	if commit != "" {
		diff, stderr, err := runGit("diff", commit)
		if diff != "" {
			parts = append(parts, diff)
		} else if err != nil && !isDiffExitCode(stderr, err) {
			return "", fmt.Errorf("DiffSince: diff from %s: %s", shortHash(commit), commandFailure(stderr, err))
		}
	} else {
		// No commits before session start — capture uncommitted changes.
		diff, stderr, err := runGit("diff", "HEAD")
		if diff != "" {
			parts = append(parts, diff)
		} else if err != nil && !isDiffExitCode(stderr, err) {
			return "", fmt.Errorf("DiffSince: diff HEAD: %s", commandFailure(stderr, err))
		}
	}

	untrackedDiff, err := diffUntrackedFiles()
	if err != nil {
		return "", err
	}
	if untrackedDiff != "" {
		parts = append(parts, untrackedDiff)
	}

	result := strings.Join(parts, "\n")
	result = strings.TrimSpace(result)
	result = stripDevlogEntries(result)
	result = stripSecretEntries(result)

	if hasMergeConflicts() {
		result = "WARNING: Working tree has merge conflicts / unmerged files.\n" + result
	}

	return result, nil
}

func findCommitBefore(t time.Time) (string, error) {
	commit, stderr, err := runGit("log", "--before="+t.Format(time.RFC3339), "-1", "--format=%H")
	if err != nil {
		if isDiffExitCode(stderr, err) || strings.TrimSpace(commit) == "" {
			return "", nil
		}
		return "", fmt.Errorf("DiffSince: find commit before %s: %s", t.Format(time.RFC3339), commandFailure(stderr, err))
	}
	return commit, nil
}

func isDiffExitCode(stderr string, err error) bool {
	if err == nil {
		return false
	}
	if strings.TrimSpace(stderr) == "" {
		return true
	}
	if strings.Contains(stderr, "does not have any commits") {
		return true
	}
	if strings.Contains(stderr, "unknown revision") || strings.Contains(stderr, "ambiguous argument") {
		return true
	}
	return false
}

func diffUntrackedFiles() (string, error) {
	untracked, stderr, err := runGit("ls-files", "--others", "--exclude-standard")
	if err != nil {
		return "", fmt.Errorf("DiffSince: list untracked files: %s", commandFailure(stderr, err))
	}
	untracked = strings.TrimSpace(untracked)
	if untracked == "" {
		return "", nil
	}

	files := strings.Split(untracked, "\n")
	textFiles := filterDevlogAndBinary(files)
	if len(textFiles) == 0 {
		return "", nil
	}

	indexFile := filepath.Join(os.TempDir(), fmt.Sprintf("devlog-index-%d", time.Now().UnixNano()))

	// Try to copy HEAD tree to temp index.
	// If HEAD does not exist (empty repo), use an empty temp index instead.
	readTree := exec.Command("git", "read-tree", "HEAD")
	readTree.Env = append(os.Environ(), "GIT_INDEX_FILE="+indexFile)
	if out, err := readTree.CombinedOutput(); err != nil {
		// Empty repo — just use an empty index.
		if strings.Contains(string(out), "Not a valid object name") || strings.Contains(string(out), "HEAD") {
			emptyIndex := exec.Command("git", "read-tree", "--empty")
			emptyIndex.Env = append(os.Environ(), "GIT_INDEX_FILE="+indexFile)
			if out2, err2 := emptyIndex.CombinedOutput(); err2 != nil {
				_ = os.Remove(indexFile)
				return "", fmt.Errorf("DiffSince: read-tree --empty for untracked: %s", out2)
			}
		} else {
			_ = os.Remove(indexFile)
			return "", fmt.Errorf("DiffSince: read-tree for untracked: %s", string(out))
		}
	}

	addArgs := append([]string{"add", "--"}, textFiles...)
	addCmd := exec.Command("git", addArgs...)
	addCmd.Env = append(os.Environ(), "GIT_INDEX_FILE="+indexFile)
	if out, err := addCmd.CombinedOutput(); err != nil {
		_ = os.Remove(indexFile)
		return "", fmt.Errorf("DiffSince: add untracked to temp index: %s", out)
	}

	var diffOut bytes.Buffer
	var diffErr bytes.Buffer
	// If HEAD doesn't exist, we diff against the empty tree.
	diffCmd := exec.Command("git", "diff", "--cached")
	diffCmd.Env = append(os.Environ(), "GIT_INDEX_FILE="+indexFile)
	diffCmd.Stdout = &diffOut
	diffCmd.Stderr = &diffErr
	_ = diffCmd.Run()

	_ = os.Remove(indexFile)
	return strings.TrimSpace(diffOut.String()), nil
}

func isSecretPath(path string) bool {
	name := strings.ToLower(filepath.Base(path))
	if isSecretPathComponent(name) {
		return true
	}
	for _, component := range strings.FieldsFunc(path, func(r rune) bool {
		return r == '/' || r == '\\'
	}) {
		if isSecretPathComponent(strings.ToLower(component)) {
			return true
		}
	}
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".pem", ".key", ".p12", ".pfx", ".crt", ".cer":
		return true
	}
	return false
}

func isSecretPathComponent(name string) bool {
	if name == ".env" || strings.HasPrefix(name, ".env.") || name == "id_rsa" || name == "id_ed25519" {
		return true
	}
	return strings.Contains(name, "secret") || strings.Contains(name, "token") || strings.Contains(name, "credential") || strings.Contains(name, "password") || strings.Contains(name, "private")
}

func filterDevlogAndBinary(files []string) []string {
	var result []string
	for _, f := range files {
		f = strings.TrimSpace(f)
		if f == "" {
			continue
		}
		if strings.HasPrefix(f, ".devlog/") || f == ".devlog" {
			continue
		}
		if isSecretPath(f) {
			continue
		}
		ext := strings.ToLower(filepath.Ext(f))
		switch ext {
		case ".png", ".jpg", ".jpeg", ".gif", ".bmp", ".ico", ".webp":
			continue
		case ".exe", ".dll", ".so", ".dylib", ".bin":
			continue
		case ".zip", ".tar", ".gz", ".bz2", ".xz":
			continue
		case ".pdf", ".mp3", ".mp4", ".wav", ".avi":
			continue
		}
		result = append(result, f)
	}
	return result
}

func hasMergeConflicts() bool {
	unmerged, _, err := runGit("ls-files", "--unmerged")
	if err != nil {
		return false
	}
	return strings.TrimSpace(unmerged) != ""
}

func stripDevlogEntries(diff string) string {
	if diff == "" {
		return diff
	}

	sections := splitDiffSections(diff)
	var filtered []string
	for _, section := range sections {
		if strings.Contains(section, ".devlog/") || strings.Contains(section, ".devlog") {
			continue
		}
		filtered = append(filtered, section)
	}
	return strings.TrimSpace(strings.Join(filtered, ""))
}

func stripSecretEntries(diff string) string {
	if diff == "" {
		return diff
	}

	sections := splitDiffSections(diff)
	var filtered []string
	for _, section := range sections {
		path := extractDiffPath(section)
		if isSecretPath(path) {
			continue
		}
		filtered = append(filtered, section)
	}
	return strings.TrimSpace(strings.Join(filtered, ""))
}

func extractDiffPath(section string) string {
	idx := strings.IndexByte(section, '\n')
	if idx < 0 {
		idx = len(section)
	}
	firstLine := section[:idx]

	aIdx := strings.Index(firstLine, " a/")
	if aIdx < 0 {
		return ""
	}
	afterA := firstLine[aIdx+3:]
	bIdx := strings.Index(afterA, " b/")
	if bIdx < 0 {
		return afterA
	}
	return afterA[:bIdx]
}

func splitDiffSections(diff string) []string {
	diff = strings.ReplaceAll(diff, "\r\n", "\n")
	var sections []string
	lines := strings.Split(diff, "\n")

	var current []string
	started := false

	for _, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "diff --git") {
			if started && len(current) > 0 {
				sections = append(sections, strings.Join(current, "\n")+"\n")
			}
			current = []string{line}
			started = true
			continue
		}
		if !started && strings.Contains(line, "Binary files") {
			current = []string{line}
			started = true
			continue
		}
		if started {
			current = append(current, line)
		}
	}

	if started && len(current) > 0 {
		sections = append(sections, strings.Join(current, "\n")+"\n")
	}

	return sections
}

func shortHash(hash string) string {
	h := strings.TrimSpace(hash)
	if len(h) > 8 {
		return h[:8]
	}
	return h
}
