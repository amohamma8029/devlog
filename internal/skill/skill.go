package skill

import (
	_ "embed"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

//go:embed SKILL.md
var skillContent string

// SkillContent returns the embedded SKILL.md content.
func SkillContent() string {
	return skillContent
}

// Tool describes a coding agent tool and where its skills are loaded from.
type Tool struct {
	Name     string
	SkillDir string
}

var tools = map[string]Tool{
	"claude":   {Name: "claude", SkillDir: ".claude/skills/devlog"},
	"opencode": {Name: "opencode", SkillDir: ".config/opencode/skills/devlog"},
	"cursor":   {Name: "cursor", SkillDir: ".cursor/skills/devlog"},
}

// AllToolNames returns the names of all supported tools in sorted order.
func AllToolNames() []string {
	names := make([]string, 0, len(tools))
	for name := range tools {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// ValidToolNames returns all supported tool names plus "all" for display in error messages.
func ValidToolNames() string {
	return strings.Join(append(AllToolNames(), "all"), ", ")
}

// ResolveTool looks up a tool by name.
func ResolveTool(name string) (Tool, error) {
	tool, ok := tools[name]
	if !ok {
		return Tool{}, fmt.Errorf("skill: unknown tool %q; valid tools are %s", name, ValidToolNames())
	}
	return tool, nil
}

// Installer writes the embedded SKILL.md into a tool's skill directory.
type Installer struct {
	homeDir string
}

// NewInstaller creates an Installer using the user's home directory.
func NewInstaller() (*Installer, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("skill: resolve home directory: %w", err)
	}
	return &Installer{homeDir: home}, nil
}

// NewInstallerWithHome creates an Installer with an explicit home directory.
func NewInstallerWithHome(homeDir string) *Installer {
	return &Installer{homeDir: homeDir}
}

// InstallResult records the outcome of installing for one tool.
type InstallResult struct {
	Tool string
	Path string
	Err  error
}

// UninstallResult records the outcome of uninstalling for one tool.
type UninstallResult struct {
	Tool    string
	Path    string
	Removed bool
	Err     error
}

// Install writes the SKILL.md for the named tool. It returns the path written.
// If the target file already exists and force is false, it returns an error.
func (i *Installer) Install(name string, force bool) (string, error) {
	tool, err := ResolveTool(name)
	if err != nil {
		return "", err
	}

	skillDir := filepath.Join(i.homeDir, tool.SkillDir)
	target := filepath.Join(skillDir, "SKILL.md")

	if !force {
		if _, err := os.Stat(target); err == nil {
			return "", fmt.Errorf("skill: SKILL.md already exists at %s; run with --force to overwrite", target)
		}
	}

	if err := os.MkdirAll(skillDir, 0755); err != nil {
		return "", fmt.Errorf("skill: create skill directory: %w", err)
	}

	if err := os.WriteFile(target, []byte(skillContent), 0644); err != nil {
		return "", fmt.Errorf("skill: write SKILL.md: %w", err)
	}

	return target, nil
}

// InstallAll writes the SKILL.md for all supported tools in sorted order.
// If any tool fails, subsequent tools still run and the returned error combines all failures.
func (i *Installer) InstallAll(force bool) ([]InstallResult, error) {
	var results []InstallResult
	var errs []error

	for _, name := range AllToolNames() {
		path, err := i.Install(name, force)
		results = append(results, InstallResult{Tool: name, Path: path, Err: err})
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", name, err))
		}
	}

	if len(errs) > 0 {
		return results, fmt.Errorf("skill: one or more installations failed: %w", errors.Join(errs...))
	}

	return results, nil
}

// Uninstall removes the SKILL.md for the named tool. It returns the path
// removed and removed=true if the file existed and was deleted; path="" and
// removed=false if the file was not present (idempotent, no error). It also
// removes the empty skill directory if it contains no other files.
func (i *Installer) Uninstall(name string) (string, bool, error) {
	tool, err := ResolveTool(name)
	if err != nil {
		return "", false, err
	}

	skillDir := filepath.Join(i.homeDir, tool.SkillDir)
	target := filepath.Join(skillDir, "SKILL.md")

	if _, err := os.Stat(target); err != nil {
		if os.IsNotExist(err) {
			return "", false, nil
		}
		return "", false, fmt.Errorf("skill: stat SKILL.md: %w", err)
	}

	if err := os.Remove(target); err != nil {
		return "", false, fmt.Errorf("skill: remove SKILL.md: %w", err)
	}

	if entries, err := os.ReadDir(skillDir); err == nil && len(entries) == 0 {
		if rerr := os.Remove(skillDir); rerr != nil {
			return target, true, fmt.Errorf("skill: remove empty skill directory: %w", rerr)
		}
	}

	return target, true, nil
}

// UninstallAll removes the SKILL.md for all supported tools in sorted order.
// If any tool fails, subsequent tools still run and the returned error combines all failures.
func (i *Installer) UninstallAll() ([]UninstallResult, error) {
	var results []UninstallResult
	var errs []error

	for _, name := range AllToolNames() {
		path, removed, err := i.Uninstall(name)
		results = append(results, UninstallResult{Tool: name, Path: path, Removed: removed, Err: err})
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", name, err))
		}
	}

	if len(errs) > 0 {
		return results, fmt.Errorf("skill: one or more uninstalls failed: %w", errors.Join(errs...))
	}

	return results, nil
}
