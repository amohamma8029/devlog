package config

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestPathFromHome(t *testing.T) {
	home := filepath.Join("tmp", "home")
	path, err := PathFromHome(home)
	if err != nil {
		t.Fatalf("PathFromHome failed: %v", err)
	}

	want := filepath.Join(home, ".config", "devlog", "config.yml")
	if path != want {
		t.Fatalf("PathFromHome = %q, want %q", path, want)
	}
}

func TestPathFromHomeRejectsEmptyHome(t *testing.T) {
	_, err := PathFromHome("  ")
	if err == nil || !strings.Contains(err.Error(), "home directory is empty") {
		t.Fatalf("expected empty home error, got: %v", err)
	}
}

func TestLoadFileMissingReturnsDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.yml")
	cfg, err := LoadFile(path)
	if err != nil {
		t.Fatalf("LoadFile failed: %v", err)
	}

	if !reflect.DeepEqual(cfg, Default()) {
		t.Fatalf("LoadFile missing config = %#v, want defaults %#v", cfg, Default())
	}
}

func TestLoadFileParsesConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yml")
	if err := os.WriteFile(path, []byte(validConfigYAML()), 0644); err != nil {
		t.Fatalf("write config failed: %v", err)
	}

	cfg, err := LoadFile(path)
	if err != nil {
		t.Fatalf("LoadFile failed: %v", err)
	}

	assertValidConfig(t, cfg)
}

func TestParseValidConfig(t *testing.T) {
	cfg, err := Parse([]byte(validConfigYAML()))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	assertValidConfig(t, cfg)
}

func TestParseEmptyConfigReturnsDefaults(t *testing.T) {
	cfg, err := Parse([]byte("\n  \n"))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if !reflect.DeepEqual(cfg, Default()) {
		t.Fatalf("Parse empty config = %#v, want defaults %#v", cfg, Default())
	}
}

func TestParsePartialConfigPreservesDefaults(t *testing.T) {
	cfg, err := Parse([]byte(`display:
  timezone: local
`))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if cfg.Author.DefaultProfile != BuiltInGitProfile {
		t.Fatalf("Author.DefaultProfile = %q, want %q", cfg.Author.DefaultProfile, BuiltInGitProfile)
	}
	if cfg.Display.Timezone != TimezoneLocal {
		t.Fatalf("Display.Timezone = %q, want %q", cfg.Display.Timezone, TimezoneLocal)
	}
	if cfg.Display.ClockFormat != ClockFormat24h {
		t.Fatalf("Display.ClockFormat = %q, want %q", cfg.Display.ClockFormat, ClockFormat24h)
	}
	if cfg.Handoff.DiffContextLines != DefaultHandoffDiffContextLines {
		t.Fatalf("Handoff.DiffContextLines = %d, want %d", cfg.Handoff.DiffContextLines, DefaultHandoffDiffContextLines)
	}
	if cfg.TUI.HandoffPreview.DiffLineLimit != DefaultHandoffPreviewLineLimit {
		t.Fatalf("TUI.HandoffPreview.DiffLineLimit = %d, want %d", cfg.TUI.HandoffPreview.DiffLineLimit, DefaultHandoffPreviewLineLimit)
	}
}

func TestParseRejectsUnknownField(t *testing.T) {
	_, err := Parse([]byte(`display:
  timezone: UTC
  format: 12h
`))
	if err == nil || !strings.Contains(err.Error(), "field format not found") {
		t.Fatalf("expected unknown field error, got: %v", err)
	}
}

func TestParseRejectsDuplicateKeys(t *testing.T) {
	_, err := Parse([]byte(`display:
  timezone: UTC
  timezone: local
`))
	if err == nil || !strings.Contains(err.Error(), `duplicate config key "timezone"`) {
		t.Fatalf("expected duplicate key error, got: %v", err)
	}
}

func TestParseRejectsInvalidAuthorProfiles(t *testing.T) {
	tests := []struct {
		name string
		yaml string
		want string
	}{
		{
			name: "reserved git profile",
			yaml: `author:
  profiles:
    git:
      display: "Git"
`,
			want: "reserved",
		},
		{
			name: "invalid profile id",
			yaml: `author:
  profiles:
    OpenCode:
      display: "OpenCode"
`,
			want: "kebab-case",
		},
		{
			name: "missing display",
			yaml: `author:
  profiles:
    opencode:
      email: "agent@example.com"
`,
			want: "display is required",
		},
		{
			name: "unknown default profile",
			yaml: `author:
  default_profile: opencode
`,
			want: "author.default_profile",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Parse([]byte(tt.yaml))
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("expected error containing %q, got: %v", tt.want, err)
			}
		})
	}
}

func TestParseAcceptsDisplayTimezones(t *testing.T) {
	for _, timezone := range []string{TimezoneUTC, TimezoneLocal, "America/New_York"} {
		t.Run(timezone, func(t *testing.T) {
			cfg, err := Parse([]byte(`display:
  timezone: "` + timezone + `"
  clock_format: 12h
`))
			if err != nil {
				t.Fatalf("Parse failed: %v", err)
			}
			if cfg.Display.Timezone != timezone {
				t.Fatalf("Display.Timezone = %q, want %q", cfg.Display.Timezone, timezone)
			}
			if cfg.Display.ClockFormat != ClockFormat12h {
				t.Fatalf("Display.ClockFormat = %q, want %q", cfg.Display.ClockFormat, ClockFormat12h)
			}
		})
	}
}

func TestParseRejectsInvalidDisplaySettings(t *testing.T) {
	tests := []struct {
		name string
		yaml string
		want string
	}{
		{
			name: "invalid timezone",
			yaml: `display:
  timezone: "New York"
`,
			want: "display.timezone",
		},
		{
			name: "invalid clock format",
			yaml: `display:
  clock_format: ampm
`,
			want: "display.clock_format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Parse([]byte(tt.yaml))
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("expected error containing %q, got: %v", tt.want, err)
			}
		})
	}
}

func TestParseRejectsNegativeLimits(t *testing.T) {
	tests := []struct {
		name string
		yaml string
		want string
	}{
		{
			name: "handoff diff context",
			yaml: `handoff:
  diff_context_lines: -1
`,
			want: "handoff.diff_context_lines",
		},
		{
			name: "TUI preview limit",
			yaml: `tui:
  handoff_preview:
    diff_line_limit: -1
`,
			want: "tui.handoff_preview.diff_line_limit",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Parse([]byte(tt.yaml))
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("expected error containing %q, got: %v", tt.want, err)
			}
		})
	}
}

func TestResolveAuthorIdentityUsesGitFallback(t *testing.T) {
	tests := []struct {
		name string
		cfg  Config
	}{
		{name: "default config", cfg: Default()},
		{name: "empty config", cfg: Config{}},
		{name: "explicit git profile", cfg: Config{Author: AuthorConfig{DefaultProfile: BuiltInGitProfile}}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			called := false
			name, email, err := tt.cfg.ResolveAuthorIdentity(func() (string, string, error) {
				called = true
				return "Git Author", "git@example.com", nil
			})
			if err != nil {
				t.Fatalf("ResolveAuthorIdentity failed: %v", err)
			}
			if !called {
				t.Fatal("expected git fallback to be called")
			}
			if name != "Git Author" || email != "git@example.com" {
				t.Fatalf("identity = %q <%s>, want Git Author <git@example.com>", name, email)
			}
		})
	}
}

func TestResolveAuthorIdentityUsesCustomProfile(t *testing.T) {
	tests := []struct {
		name      string
		profile   AuthorProfile
		wantName  string
		wantEmail string
	}{
		{name: "with email", profile: AuthorProfile{Display: "OpenCode", Email: "opencode@example.com"}, wantName: "OpenCode", wantEmail: "opencode@example.com"},
		{name: "without email", profile: AuthorProfile{Display: "OpenCode"}, wantName: "OpenCode", wantEmail: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Config{
				Author: AuthorConfig{
					DefaultProfile: "opencode",
					Profiles: map[string]AuthorProfile{
						"opencode": tt.profile,
					},
				},
			}

			name, email, err := cfg.ResolveAuthorIdentity(func() (string, string, error) {
				return "", "", nil
			})
			if err != nil {
				t.Fatalf("ResolveAuthorIdentity failed: %v", err)
			}
			if name != tt.wantName || email != tt.wantEmail {
				t.Fatalf("identity = %q <%s>, want %q <%s>", name, email, tt.wantName, tt.wantEmail)
			}
		})
	}
}

func TestResolveAuthorIdentityRejectsInvalidConfig(t *testing.T) {
	cfg := Config{Author: AuthorConfig{DefaultProfile: "missing"}}

	_, _, err := cfg.ResolveAuthorIdentity(func() (string, string, error) {
		return "Git Author", "git@example.com", nil
	})
	if err == nil || !strings.Contains(err.Error(), "author.default_profile") {
		t.Fatalf("expected author.default_profile error, got: %v", err)
	}
}

func validConfigYAML() string {
	return `author:
  default_profile: opencode
  profiles:
    personal:
      display: "Ayman"
      email: "ayman@example.com"
    opencode:
      display: "OpenCode"

editor:
  command: "code"
  args: ["--wait"]

display:
  timezone: "America/New_York"
  clock_format: "12h"

handoff:
  diff_context_lines: 0

tui:
  handoff_preview:
    diff_line_limit: 42
`
}

func assertValidConfig(t *testing.T, cfg Config) {
	t.Helper()

	if cfg.Author.DefaultProfile != "opencode" {
		t.Fatalf("Author.DefaultProfile = %q, want opencode", cfg.Author.DefaultProfile)
	}
	if cfg.Author.Profiles["personal"].Display != "Ayman" {
		t.Fatalf("personal display = %q, want Ayman", cfg.Author.Profiles["personal"].Display)
	}
	if cfg.Author.Profiles["personal"].Email != "ayman@example.com" {
		t.Fatalf("personal email = %q, want ayman@example.com", cfg.Author.Profiles["personal"].Email)
	}
	if cfg.Author.Profiles["opencode"].Display != "OpenCode" {
		t.Fatalf("opencode display = %q, want OpenCode", cfg.Author.Profiles["opencode"].Display)
	}
	if cfg.Editor.Command != "code" {
		t.Fatalf("Editor.Command = %q, want code", cfg.Editor.Command)
	}
	if !reflect.DeepEqual(cfg.Editor.Args, []string{"--wait"}) {
		t.Fatalf("Editor.Args = %#v, want [--wait]", cfg.Editor.Args)
	}
	if cfg.Display.Timezone != "America/New_York" {
		t.Fatalf("Display.Timezone = %q, want America/New_York", cfg.Display.Timezone)
	}
	if cfg.Display.ClockFormat != ClockFormat12h {
		t.Fatalf("Display.ClockFormat = %q, want %q", cfg.Display.ClockFormat, ClockFormat12h)
	}
	if cfg.Handoff.DiffContextLines != 0 {
		t.Fatalf("Handoff.DiffContextLines = %d, want 0", cfg.Handoff.DiffContextLines)
	}
	if cfg.TUI.HandoffPreview.DiffLineLimit != 42 {
		t.Fatalf("TUI.HandoffPreview.DiffLineLimit = %d, want 42", cfg.TUI.HandoffPreview.DiffLineLimit)
	}
}
