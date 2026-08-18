package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func setSkillTestHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	return home
}

func TestRootCommandIncludesSkillInstall(t *testing.T) {
	cmd, _, err := newRootCommand().Find([]string{"skill", "install"})
	if err != nil {
		t.Fatalf("find skill install failed: %v", err)
	}
	if cmd == nil || cmd.Name() != "install" {
		t.Fatalf("expected skill install command, got %v", cmd)
	}
	if cmd.Parent() == nil || cmd.Parent().Name() != "skill" {
		t.Fatalf("expected parent to be skill, got %v", cmd.Parent())
	}
}

func TestSkillInstallClaudeWritesFile(t *testing.T) {
	home := setSkillTestHome(t)

	out, err := executeRootCommand("skill", "install", "claude")
	if err != nil {
		t.Fatalf("skill install claude failed: %v", err)
	}

	if !strings.Contains(out, "Skill installed") {
		t.Errorf("output missing 'Skill installed': %q", out)
	}
	if !strings.Contains(out, "claude") {
		t.Errorf("output missing tool name: %q", out)
	}

	target := filepath.Join(home, ".claude", "skills", "devlog", "SKILL.md")
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("SKILL.md not written: %v", err)
	}
	if strings.TrimSpace(string(data)) == "" {
		t.Error("SKILL.md is empty")
	}
}

func TestSkillInstallRefusesExistingWithoutForce(t *testing.T) {
	setSkillTestHome(t)

	if _, err := executeRootCommand("skill", "install", "claude"); err != nil {
		t.Fatalf("first install failed: %v", err)
	}

	_, err := executeRootCommand("skill", "install", "claude")
	if err == nil {
		t.Fatal("second install should refuse")
	}
	if !strings.Contains(err.Error(), "--force") {
		t.Errorf("error should mention --force: %v", err)
	}
}

func TestSkillInstallForceOverwrites(t *testing.T) {
	home := setSkillTestHome(t)

	if _, err := executeRootCommand("skill", "install", "claude"); err != nil {
		t.Fatalf("first install failed: %v", err)
	}

	target := filepath.Join(home, ".claude", "skills", "devlog", "SKILL.md")
	if err := os.WriteFile(target, []byte("old"), 0644); err != nil {
		t.Fatalf("write old content: %v", err)
	}

	if _, err := executeRootCommand("skill", "install", "claude", "--force"); err != nil {
		t.Fatalf("install --force failed: %v", err)
	}

	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if string(data) == "old" {
		t.Error("file was not overwritten")
	}
}

func TestSkillInstallAllWritesAllThree(t *testing.T) {
	home := setSkillTestHome(t)

	out, err := executeRootCommand("skill", "install", "all")
	if err != nil {
		t.Fatalf("skill install all failed: %v", err)
	}

	for _, tool := range []string{"claude", "opencode", "cursor"} {
		if !strings.Contains(out, tool) {
			t.Errorf("output missing tool %q: %q", tool, out)
		}
	}

	for _, target := range []string{
		filepath.Join(home, ".claude", "skills", "devlog", "SKILL.md"),
		filepath.Join(home, ".config", "opencode", "skills", "devlog", "SKILL.md"),
		filepath.Join(home, ".cursor", "skills", "devlog", "SKILL.md"),
	} {
		if _, err := os.Stat(target); err != nil {
			t.Errorf("SKILL.md not written at %s: %v", target, err)
		}
	}
}

func TestSkillInstallNoArgsReturnsError(t *testing.T) {
	setSkillTestHome(t)

	_, err := executeRootCommand("skill", "install")
	if err == nil {
		t.Fatal("skill install with no args should return error")
	}
}

func TestSkillInstallUnknownToolReturnsError(t *testing.T) {
	setSkillTestHome(t)

	_, err := executeRootCommand("skill", "install", "foo")
	if err == nil {
		t.Fatal("skill install unknown tool should return error")
	}
	if !strings.Contains(err.Error(), "foo") {
		t.Errorf("error should mention tool name: %v", err)
	}
}

func TestRootCommandIncludesSkillUninstall(t *testing.T) {
	cmd, _, err := newRootCommand().Find([]string{"skill", "uninstall"})
	if err != nil {
		t.Fatalf("find skill uninstall failed: %v", err)
	}
	if cmd == nil || cmd.Name() != "uninstall" {
		t.Fatalf("expected skill uninstall command, got %v", cmd)
	}
	if cmd.Parent() == nil || cmd.Parent().Name() != "skill" {
		t.Fatalf("expected parent to be skill, got %v", cmd.Parent())
	}
}

func TestSkillUninstallRemovesFile(t *testing.T) {
	home := setSkillTestHome(t)

	if _, err := executeRootCommand("skill", "install", "claude"); err != nil {
		t.Fatalf("install failed: %v", err)
	}

	out, err := executeRootCommand("skill", "uninstall", "claude")
	if err != nil {
		t.Fatalf("uninstall failed: %v", err)
	}

	if !strings.Contains(out, "Skill removed") {
		t.Errorf("output missing 'Skill removed': %q", out)
	}

	target := filepath.Join(home, ".claude", "skills", "devlog", "SKILL.md")
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Errorf("SKILL.md still exists: %v", err)
	}
}

func TestSkillUninstallNotInstalledNoError(t *testing.T) {
	setSkillTestHome(t)

	out, err := executeRootCommand("skill", "uninstall", "claude")
	if err != nil {
		t.Fatalf("uninstall on absent file should not error: %v", err)
	}

	if !strings.Contains(out, "Skill not installed") {
		t.Errorf("output missing 'Skill not installed': %q", out)
	}
}

func TestSkillUninstallAllRemovesAllThree(t *testing.T) {
	home := setSkillTestHome(t)

	if _, err := executeRootCommand("skill", "install", "all"); err != nil {
		t.Fatalf("install all failed: %v", err)
	}

	out, err := executeRootCommand("skill", "uninstall", "all")
	if err != nil {
		t.Fatalf("uninstall all failed: %v", err)
	}

	for _, tool := range []string{"claude", "opencode", "cursor"} {
		if !strings.Contains(out, tool) {
			t.Errorf("output missing tool %q: %q", tool, out)
		}
	}

	for _, target := range []string{
		filepath.Join(home, ".claude", "skills", "devlog", "SKILL.md"),
		filepath.Join(home, ".config", "opencode", "skills", "devlog", "SKILL.md"),
		filepath.Join(home, ".cursor", "skills", "devlog", "SKILL.md"),
	} {
		if _, err := os.Stat(target); !os.IsNotExist(err) {
			t.Errorf("SKILL.md still exists at %s: %v", target, err)
		}
	}
}

func TestSkillUninstallNoArgsReturnsError(t *testing.T) {
	setSkillTestHome(t)

	_, err := executeRootCommand("skill", "uninstall")
	if err == nil {
		t.Fatal("skill uninstall with no args should return error")
	}
}

func TestSkillUninstallUnknownToolReturnsError(t *testing.T) {
	setSkillTestHome(t)

	_, err := executeRootCommand("skill", "uninstall", "foo")
	if err == nil {
		t.Fatal("skill uninstall unknown tool should return error")
	}
}
