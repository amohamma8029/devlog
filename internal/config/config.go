// Package config loads and validates devlog's global configuration file.
package config

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	_ "time/tzdata" // Embed IANA timezone data for consistent validation.

	"gopkg.in/yaml.v3"
)

const (
	BuiltInGitProfile = "git"

	TimezoneUTC   = "UTC"
	TimezoneLocal = "local"

	ClockFormat12h = "12h"
	ClockFormat24h = "24h"

	DefaultHandoffDiffContextLines = 3
	DefaultHandoffPreviewLineLimit = 100
)

var profileIDPattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

// Config is the typed representation of ~/.config/devlog/config.yml.
type Config struct {
	Author  AuthorConfig  `yaml:"author,omitempty"`
	Editor  EditorConfig  `yaml:"editor,omitempty"`
	Display DisplayConfig `yaml:"display,omitempty"`
	Handoff HandoffConfig `yaml:"handoff,omitempty"`
	TUI     TUIConfig     `yaml:"tui,omitempty"`
}

// AuthorConfig controls the author identity profile used for new sessions.
type AuthorConfig struct {
	DefaultProfile string                   `yaml:"default_profile,omitempty"`
	Profiles       map[string]AuthorProfile `yaml:"profiles,omitempty"`
}

// AuthorProfile is a named author identity users can select later.
type AuthorProfile struct {
	Display string `yaml:"display,omitempty"`
	Email   string `yaml:"email,omitempty"`
}

// EditorConfig describes the preferred editor command for later slices.
type EditorConfig struct {
	Command string   `yaml:"command,omitempty"`
	Args    []string `yaml:"args,omitempty"`
}

// DisplayConfig controls timestamp presentation only.
type DisplayConfig struct {
	Timezone    string `yaml:"timezone,omitempty"`
	ClockFormat string `yaml:"clock_format,omitempty"`
}

// HandoffConfig controls handoff generation options.
type HandoffConfig struct {
	DiffContextLines int `yaml:"diff_context_lines,omitempty"`
}

// TUIConfig controls TUI-only settings.
type TUIConfig struct {
	HandoffPreview HandoffPreviewConfig `yaml:"handoff_preview,omitempty"`
}

// HandoffPreviewConfig controls TUI handoff preview rendering.
type HandoffPreviewConfig struct {
	DiffLineLimit int `yaml:"diff_line_limit,omitempty"`
}

// Default returns the config used when no global config file exists.
func Default() Config {
	return Config{
		Author: AuthorConfig{
			DefaultProfile: BuiltInGitProfile,
			Profiles:       map[string]AuthorProfile{},
		},
		Display: DisplayConfig{
			Timezone:    TimezoneUTC,
			ClockFormat: ClockFormat24h,
		},
		Handoff: HandoffConfig{
			DiffContextLines: DefaultHandoffDiffContextLines,
		},
		TUI: TUIConfig{
			HandoffPreview: HandoffPreviewConfig{
				DiffLineLimit: DefaultHandoffPreviewLineLimit,
			},
		},
	}
}

// Path returns the global config file path for the current user.
func Path() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("config path: resolve user home: %w", err)
	}

	return PathFromHome(home)
}

// PathFromHome returns the global config path for a specific home directory.
func PathFromHome(home string) (string, error) {
	home = strings.TrimSpace(home)
	if home == "" {
		return "", fmt.Errorf("config path: home directory is empty")
	}

	return filepath.Join(home, ".config", "devlog", "config.yml"), nil
}

// Load reads the current user's global config file, returning defaults when it is missing.
func Load() (Config, error) {
	path, err := Path()
	if err != nil {
		return Config{}, err
	}

	return LoadFile(path)
}

// LoadFile reads a config file from path, returning defaults when it is missing.
func LoadFile(path string) (Config, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return Config{}, fmt.Errorf("config: path is empty")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Default(), nil
		}
		return Config{}, fmt.Errorf("config: read %s: %w", path, err)
	}

	cfg, err := Parse(data)
	if err != nil {
		return Config{}, fmt.Errorf("config: parse %s: %w", path, err)
	}

	return cfg, nil
}

// Parse decodes and validates config YAML.
func Parse(data []byte) (Config, error) {
	cfg := Default()
	if strings.TrimSpace(string(data)) == "" {
		return cfg, nil
	}

	if err := rejectDuplicateKeys(data); err != nil {
		return Config{}, err
	}

	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&cfg); err != nil {
		return Config{}, fmt.Errorf("decode YAML: %w", err)
	}

	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		if err == nil {
			return Config{}, fmt.Errorf("multiple YAML documents are not supported")
		}
		return Config{}, fmt.Errorf("decode YAML: %w", err)
	}

	cfg.applyDefaults()
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func (c *Config) applyDefaults() {
	if strings.TrimSpace(c.Author.DefaultProfile) == "" {
		c.Author.DefaultProfile = BuiltInGitProfile
	}
	if c.Author.Profiles == nil {
		c.Author.Profiles = map[string]AuthorProfile{}
	}
	if strings.TrimSpace(c.Display.Timezone) == "" {
		c.Display.Timezone = TimezoneUTC
	}
	if strings.TrimSpace(c.Display.ClockFormat) == "" {
		c.Display.ClockFormat = ClockFormat24h
	}
}

// Validate checks semantic config rules after YAML decoding.
func (c Config) Validate() error {
	if err := validateAuthor(c.Author); err != nil {
		return err
	}
	if err := validateDisplay(c.Display); err != nil {
		return err
	}
	if c.Handoff.DiffContextLines < 0 {
		return fmt.Errorf("handoff.diff_context_lines must be 0 or greater")
	}
	if c.TUI.HandoffPreview.DiffLineLimit < 0 {
		return fmt.Errorf("tui.handoff_preview.diff_line_limit must be 0 or greater")
	}

	return nil
}

func validateAuthor(author AuthorConfig) error {
	defaultProfile := strings.TrimSpace(author.DefaultProfile)
	if defaultProfile == "" {
		defaultProfile = BuiltInGitProfile
	}

	for id, profile := range author.Profiles {
		if id == BuiltInGitProfile {
			return fmt.Errorf("author.profiles.%s is reserved for the built-in git profile", BuiltInGitProfile)
		}
		if !profileIDPattern.MatchString(id) {
			return fmt.Errorf("author profile ID %q must be kebab-case", id)
		}
		if strings.TrimSpace(profile.Display) == "" {
			return fmt.Errorf("author.profiles.%s.display is required", id)
		}
	}

	if defaultProfile == BuiltInGitProfile {
		return nil
	}
	if _, ok := author.Profiles[defaultProfile]; !ok {
		return fmt.Errorf("author.default_profile %q must be %q or a configured author profile", defaultProfile, BuiltInGitProfile)
	}

	return nil
}

func validateDisplay(display DisplayConfig) error {
	timezone := strings.TrimSpace(display.Timezone)
	switch timezone {
	case "", TimezoneUTC, TimezoneLocal:
		// Valid. Empty is normalized to UTC before callers observe it.
	default:
		if _, err := time.LoadLocation(timezone); err != nil {
			return fmt.Errorf("display.timezone %q must be %q, %q, or a valid IANA timezone", timezone, TimezoneUTC, TimezoneLocal)
		}
	}

	switch strings.TrimSpace(display.ClockFormat) {
	case "", ClockFormat12h, ClockFormat24h:
		return nil
	default:
		return fmt.Errorf("display.clock_format %q must be %q or %q", display.ClockFormat, ClockFormat12h, ClockFormat24h)
	}
}

func rejectDuplicateKeys(data []byte) error {
	var node yaml.Node
	if err := yaml.Unmarshal(data, &node); err != nil {
		return fmt.Errorf("decode YAML: %w", err)
	}
	if len(node.Content) == 0 {
		return nil
	}

	return rejectDuplicateKeysInNode(node.Content[0], "")
}

func rejectDuplicateKeysInNode(node *yaml.Node, path string) error {
	switch node.Kind {
	case yaml.MappingNode:
		seen := map[string]struct{}{}
		for i := 0; i < len(node.Content); i += 2 {
			key := node.Content[i]
			value := node.Content[i+1]
			if _, ok := seen[key.Value]; ok {
				if path == "" {
					return fmt.Errorf("duplicate config key %q", key.Value)
				}
				return fmt.Errorf("duplicate config key %q in %s", key.Value, path)
			}
			seen[key.Value] = struct{}{}

			childPath := key.Value
			if path != "" {
				childPath = path + "." + key.Value
			}
			if err := rejectDuplicateKeysInNode(value, childPath); err != nil {
				return err
			}
		}
	case yaml.SequenceNode:
		for _, child := range node.Content {
			if err := rejectDuplicateKeysInNode(child, path); err != nil {
				return err
			}
		}
	}

	return nil
}
