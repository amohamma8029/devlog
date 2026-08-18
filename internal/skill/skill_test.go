package skill

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSkillContentIsNonEmpty(t *testing.T) {
	content := SkillContent()
	if strings.TrimSpace(content) == "" {
		t.Fatal("SkillContent() returned empty content")
	}
}

func TestSkillContentHasFrontmatter(t *testing.T) {
	content := SkillContent()
	if !strings.HasPrefix(content, "---\n") {
		t.Fatal("SkillContent() must start with YAML frontmatter (---)")
	}
	end := strings.Index(content[4:], "\n---\n")
	if end < 0 {
		t.Fatal("SkillContent() frontmatter is not closed")
	}
	frontmatter := content[4 : 4+end]
	if !strings.Contains(frontmatter, "name: devlog") {
		t.Errorf("frontmatter missing 'name: devlog':\n%s", frontmatter)
	}
	if !strings.Contains(frontmatter, "description:") {
		t.Errorf("frontmatter missing 'description:':\n%s", frontmatter)
	}
}

func TestSkillContentContainsLifecycle(t *testing.T) {
	content := SkillContent()
	markers := []string{
		"devlog status",
		"devlog open",
		"devlog note",
		"devlog block",
		"devlog handoff",
		"devlog todo add",
		"devlog todo done",
		"devlog todo list",
		"devlog todo reopen",
		"devlog todo edit",
		"devlog todo delete",
		"devlog todo list --ids",
		"completed todos are numbered after open ones",
		"only open todos can be edited",
		"devlog handoff -o <name>",
		"ask the human whether to close it and reopen",
		"devlog --version",
		"devlog edit 2 -m",
		"devlog edit 2 --delete",
		"devlog list",
		"Never run `devlog` without a subcommand",
		"uncommitted file changes only",
		"repo-wide — not per-branch",
		"devlog status -n 0",
		"devlog todo list --branch feat/auth",
		"devlog todo list --session <session-id>",
		"requires a git repository",
		"todo parse error",
		"most-recent-first",
		"Never run `devlog close`",
		"Never run `devlog todo prune`",
		"periodically",
		"full contents of the handoff file",
	}
	for _, marker := range markers {
		if !strings.Contains(content, marker) {
			t.Errorf("SkillContent() missing %q", marker)
		}
	}
}

func TestAllToolNamesSorted(t *testing.T) {
	names := AllToolNames()
	want := []string{"claude", "cursor", "opencode"}
	if len(names) != len(want) {
		t.Fatalf("AllToolNames() = %v, want %v", names, want)
	}
	for i, name := range names {
		if name != want[i] {
			t.Errorf("AllToolNames()[%d] = %q, want %q", i, name, want[i])
		}
	}
}

func TestResolveToolKnown(t *testing.T) {
	cases := []struct {
		name     string
		skillDir string
	}{
		{"claude", ".claude/skills/devlog"},
		{"opencode", ".config/opencode/skills/devlog"},
		{"cursor", ".cursor/skills/devlog"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tool, err := ResolveTool(tc.name)
			if err != nil {
				t.Fatalf("ResolveTool(%q) error: %v", tc.name, err)
			}
			if tool.Name != tc.name {
				t.Errorf("tool.Name = %q, want %q", tool.Name, tc.name)
			}
			if tool.SkillDir != tc.skillDir {
				t.Errorf("tool.SkillDir = %q, want %q", tool.SkillDir, tc.skillDir)
			}
		})
	}
}

func TestResolveToolUnknown(t *testing.T) {
	_, err := ResolveTool("foo")
	if err == nil {
		t.Fatal("ResolveTool unknown should return error")
	}
	if !strings.Contains(err.Error(), "foo") {
		t.Errorf("error should mention tool name, got: %v", err)
	}
	for _, name := range AllToolNames() {
		if !strings.Contains(err.Error(), name) {
			t.Errorf("error should mention %q, got: %v", name, err)
		}
	}
}

func TestInstallWritesSkillFile(t *testing.T) {
	home := t.TempDir()
	installer := NewInstallerWithHome(home)

	path, err := installer.Install("claude", false)
	if err != nil {
		t.Fatalf("Install failed: %v", err)
	}

	wantPath := filepath.Join(home, ".claude", "skills", "devlog", "SKILL.md")
	if path != wantPath {
		t.Errorf("path = %q, want %q", path, wantPath)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read installed file: %v", err)
	}
	if string(data) != SkillContent() {
		t.Error("installed content does not match SkillContent()")
	}
}

func TestInstallCreatesParentDirectories(t *testing.T) {
	home := t.TempDir()
	installer := NewInstallerWithHome(home)

	if _, err := installer.Install("opencode", false); err != nil {
		t.Fatalf("Install failed: %v", err)
	}

	dir := filepath.Join(home, ".config", "opencode", "skills", "devlog")
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("skill directory not created: %v", err)
	}
	if !info.IsDir() {
		t.Fatal("skill directory path is not a directory")
	}
}

func TestInstallRefusesExistingWithoutForce(t *testing.T) {
	home := t.TempDir()
	installer := NewInstallerWithHome(home)

	if _, err := installer.Install("claude", false); err != nil {
		t.Fatalf("first Install failed: %v", err)
	}

	_, err := installer.Install("claude", false)
	if err == nil {
		t.Fatal("second Install should refuse")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("error should mention 'already exists', got: %v", err)
	}
	if !strings.Contains(err.Error(), "--force") {
		t.Errorf("error should mention --force, got: %v", err)
	}
}

func TestInstallOverwritesWithForce(t *testing.T) {
	home := t.TempDir()
	installer := NewInstallerWithHome(home)

	if _, err := installer.Install("claude", false); err != nil {
		t.Fatalf("first Install failed: %v", err)
	}

	target := filepath.Join(home, ".claude", "skills", "devlog", "SKILL.md")
	if err := os.WriteFile(target, []byte("old content"), 0644); err != nil {
		t.Fatalf("write old content: %v", err)
	}

	if _, err := installer.Install("claude", true); err != nil {
		t.Fatalf("Install --force failed: %v", err)
	}

	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if string(data) != SkillContent() {
		t.Error("file content was not overwritten with SkillContent()")
	}
}

func TestInstallUnknownTool(t *testing.T) {
	home := t.TempDir()
	installer := NewInstallerWithHome(home)

	_, err := installer.Install("foo", false)
	if err == nil {
		t.Fatal("Install unknown tool should return error")
	}
}

func TestInstallAllWritesAllThree(t *testing.T) {
	home := t.TempDir()
	installer := NewInstallerWithHome(home)

	results, err := installer.InstallAll(false)
	if err != nil {
		t.Fatalf("InstallAll failed: %v", err)
	}

	if len(results) != 3 {
		t.Fatalf("results count = %d, want 3", len(results))
	}

	for _, r := range results {
		if r.Err != nil {
			t.Errorf("tool %s error: %v", r.Tool, r.Err)
			continue
		}
		if _, err := os.Stat(r.Path); err != nil {
			t.Errorf("tool %s: file not at %s: %v", r.Tool, r.Path, err)
		}
	}
}

func TestInstallAllRefusesAllExistingWithoutForce(t *testing.T) {
	home := t.TempDir()
	installer := NewInstallerWithHome(home)

	if _, err := installer.InstallAll(false); err != nil {
		t.Fatalf("first InstallAll failed: %v", err)
	}

	_, err := installer.InstallAll(false)
	if err == nil {
		t.Fatal("second InstallAll should refuse")
	}
}

func TestInstallAllOverwritesWithForce(t *testing.T) {
	home := t.TempDir()
	installer := NewInstallerWithHome(home)

	if _, err := installer.InstallAll(false); err != nil {
		t.Fatalf("first InstallAll failed: %v", err)
	}

	if _, err := installer.InstallAll(true); err != nil {
		t.Fatalf("InstallAll --force failed: %v", err)
	}
}

func TestUninstallRemovesSkillFile(t *testing.T) {
	home := t.TempDir()
	installer := NewInstallerWithHome(home)

	if _, err := installer.Install("claude", false); err != nil {
		t.Fatalf("Install failed: %v", err)
	}

	target := filepath.Join(home, ".claude", "skills", "devlog", "SKILL.md")
	path, removed, err := installer.Uninstall("claude")
	if err != nil {
		t.Fatalf("Uninstall failed: %v", err)
	}
	if !removed {
		t.Fatal("removed should be true")
	}
	if path != target {
		t.Errorf("path = %q, want %q", path, target)
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Errorf("SKILL.md still exists: %v", err)
	}
}

func TestUninstallRemovesEmptySkillDir(t *testing.T) {
	home := t.TempDir()
	installer := NewInstallerWithHome(home)

	if _, err := installer.Install("claude", false); err != nil {
		t.Fatalf("Install failed: %v", err)
	}

	if _, _, err := installer.Uninstall("claude"); err != nil {
		t.Fatalf("Uninstall failed: %v", err)
	}

	skillDir := filepath.Join(home, ".claude", "skills", "devlog")
	if _, err := os.Stat(skillDir); !os.IsNotExist(err) {
		t.Errorf("empty skill dir still exists: %v", err)
	}
}

func TestUninstallNotInstalledIsIdempotent(t *testing.T) {
	home := t.TempDir()
	installer := NewInstallerWithHome(home)

	path, removed, err := installer.Uninstall("claude")
	if err != nil {
		t.Fatalf("Uninstall on absent file should not error: %v", err)
	}
	if removed {
		t.Error("removed should be false when nothing was installed")
	}
	if path != "" {
		t.Errorf("path = %q, want empty", path)
	}
}

func TestUninstallKeepsDirIfNotEmpty(t *testing.T) {
	home := t.TempDir()
	installer := NewInstallerWithHome(home)

	if _, err := installer.Install("claude", false); err != nil {
		t.Fatalf("Install failed: %v", err)
	}

	skillDir := filepath.Join(home, ".claude", "skills", "devlog")
	extra := filepath.Join(skillDir, "extra.txt")
	if err := os.WriteFile(extra, []byte("user file"), 0644); err != nil {
		t.Fatalf("write extra file: %v", err)
	}

	if _, _, err := installer.Uninstall("claude"); err != nil {
		t.Fatalf("Uninstall failed: %v", err)
	}

	if _, err := os.Stat(extra); err != nil {
		t.Errorf("user-added file should remain: %v", err)
	}
	if _, err := os.Stat(skillDir); err != nil {
		t.Errorf("skill dir should remain (not empty): %v", err)
	}
}

func TestUninstallUnknownTool(t *testing.T) {
	home := t.TempDir()
	installer := NewInstallerWithHome(home)

	_, _, err := installer.Uninstall("foo")
	if err == nil {
		t.Fatal("Uninstall unknown tool should return error")
	}
}

func TestUninstallAllRemovesAllThree(t *testing.T) {
	home := t.TempDir()
	installer := NewInstallerWithHome(home)

	if _, err := installer.InstallAll(false); err != nil {
		t.Fatalf("InstallAll failed: %v", err)
	}

	results, err := installer.UninstallAll()
	if err != nil {
		t.Fatalf("UninstallAll failed: %v", err)
	}

	if len(results) != 3 {
		t.Fatalf("results count = %d, want 3", len(results))
	}

	for _, r := range results {
		if r.Err != nil {
			t.Errorf("tool %s error: %v", r.Tool, r.Err)
			continue
		}
		if !r.Removed {
			t.Errorf("tool %s should have been removed", r.Tool)
		}
		if _, err := os.Stat(r.Path); !os.IsNotExist(err) {
			t.Errorf("tool %s: file still exists at %s", r.Tool, r.Path)
		}
	}
}

func TestUninstallAllIdempotent(t *testing.T) {
	home := t.TempDir()
	installer := NewInstallerWithHome(home)

	results, err := installer.UninstallAll()
	if err != nil {
		t.Fatalf("UninstallAll on empty should not error: %v", err)
	}
	for _, r := range results {
		if r.Removed {
			t.Errorf("tool %s should not be removed", r.Tool)
		}
	}
}
