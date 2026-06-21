# Configuration

devlog reads a single global YAML config file at `~/.config/devlog/config.yml`. When the file is missing, devlog preserves its default behavior — no config is required to use the tool. When the file exists but is invalid, devlog returns an explicit error and does not fall back silently.

The config path uses the user's home directory explicitly on all platforms (it does not use Go's platform-specific `os.UserConfigDir()`), so the path is the same on Windows, macOS, and Linux.

## Discovering and editing the config

```bash
# Print the resolved global config path
devlog config path

# Open the config in your editor; creates a starter file if missing
devlog config edit
```

`devlog config edit` opens `~/.config/devlog/config.yml` in the configured editor (see [Editor](#editor)). If the file does not exist, devlog creates the parent directory and writes a starter file with `0600` permissions, then opens it. After the editor exits, devlog re-reads and validates the file, reporting any parse or validation errors. If `editor.command` is set but not found on `PATH`, devlog prints a warning suggesting you verify the command or remove it to fall back to `$EDITOR`.

## Full schema

```yaml
author:
  default_profile: git
  profiles: {}

editor:
  command: ""
  args: []

display:
  timezone: UTC
  clock_format: 24h

handoff:
  diff_context_lines: 3

tui:
  handoff_preview:
    diff_line_limit: 100
```

### `author`

Controls the author identity recorded on new sessions.

| Field | Default | Valid values |
| --- | --- | --- |
| `default_profile` | `git` | `git`, or the ID of a profile defined under `profiles` |
| `profiles` | `{}` | A map of profile IDs to profile entries |

Each entry under `profiles` has two fields:

| Field | Required | Description |
| --- | --- | --- |
| `display` | yes | Display name written to session front-matter |
| `email` | no | Email written to session front-matter |

Profile IDs must be unique and kebab-case (lowercase letters and digits separated by single hyphens, e.g. `personal`, `opencode`, `work-2`). The ID `git` is reserved for the built-in profile and cannot be redefined.

#### Author identity fallback

When `default_profile` is `git` (the default), devlog resolves the author identity in this order:

1. `git config user.name` and `git config user.email` from the current repository
2. `DEVLOG_AUTHOR_NAME` and `DEVLOG_AUTHOR_EMAIL` environment variables

When `default_profile` names a custom profile, devlog uses that profile's `display` and `email` directly.

```yaml
author:
  default_profile: personal
  profiles:
    personal:
      display: "Ayman"
      email: "ayman@example.com"

    opencode:
      display: "OpenCode"
```

### `editor`

Controls the editor used by `devlog config edit` and multi-line note/block entry in the CLI.

| Field | Default | Description |
| --- | --- | --- |
| `command` | `""` | Editor executable name or path |
| `args` | `[]` | Extra arguments passed to the editor |

#### Editor fallback

devlog resolves the editor in this order:

1. `editor.command` in the config, if set and found on `PATH`
2. `$VISUAL` environment variable
3. `$EDITOR` environment variable
4. `notepad.exe` on Windows, `vi` elsewhere

When `editor.command` is set but fails to launch, devlog falls back to `$VISUAL`/`$EDITOR` and prints a notice. If no editor resolves, the launch fails with an explicit error.

```yaml
editor:
  command: "code"
  args: ["--wait"]
```

### `display`

Controls how timestamps are rendered in CLI output and the TUI. Display config affects **rendered timestamps only** — stored session timestamps, event timestamps, and session IDs remain UTC.

| Field | Default | Valid values |
| --- | --- | --- |
| `timezone` | `UTC` | `UTC`, `local`, or a valid IANA timezone name (e.g. `America/New_York`) |
| `clock_format` | `24h` | `24h` or `12h` |

`local` uses the user's OS timezone at display time. An empty value is normalized to `UTC` before any output is rendered.

```yaml
display:
  timezone: "America/New_York"
  clock_format: "12h"
```

### `handoff`

Controls handoff artifact generation.

| Field | Default | Valid values |
| --- | --- | --- |
| `diff_context_lines` | `3` | Any non-negative integer |

`diff_context_lines` controls the number of surrounding unchanged lines shown around each change in a handoff diff. `3` preserves git's default context. `0` shows no surrounding unchanged lines. Higher values show more surrounding code.

This setting controls surrounding context only. It does not include or exclude files, and it does not weaken devlog's filtering of secret-looking paths or `.devlog/` entries.

```yaml
handoff:
  diff_context_lines: 5
```

### `tui.handoff_preview`

Controls TUI handoff preview rendering only.

| Field | Default | Valid values |
| --- | --- | --- |
| `diff_line_limit` | `100` | Any non-negative integer |

`diff_line_limit` truncates each file's diff block in the TUI handoff preview at the given number of lines, appending a `... (truncated, N more lines)` marker when exceeded. `0` disables truncation entirely — all diff lines are shown in the preview without a marker.

This setting applies to **TUI preview rendering only**. Saved and copied handoff content is not truncated by this setting; existing collapsed-diff omission behavior is unchanged.

```yaml
tui:
  handoff_preview:
    diff_line_limit: 50
```

## Validation rules

devlog validates the config file on load. The following are rejected:

- **Duplicate keys** — the same key appearing twice at any level (e.g. two `display:` blocks).
- **Unknown fields** — any key not in the schema above. This catches typos like `clockformat` instead of `clock_format`.
- **Multiple YAML documents** — a single config file must contain one YAML document, not a multi-document stream.
- **Invalid author profile IDs** — IDs must be kebab-case; `git` is reserved; `display` is required on every custom profile.
- **Invalid `display.timezone`** — must be `UTC`, `local`, or a loadable IANA timezone name.
- **Invalid `display.clock_format`** — must be `24h` or `12h`.
- **Negative integers** — `handoff.diff_context_lines` and `tui.handoff_preview.diff_line_limit` must be `0` or greater.

When validation fails, devlog prints the path and the reason. Run `devlog config edit` again to fix the file.

## Not yet configurable

The following are intentionally deferred from v1 config and are not configurable today:

- **Diff exclusion patterns** — filtering which files appear in handoff diffs beyond the built-in secret and `.devlog/` filters.
- **TUI theme and colors** — the terminal color palette is not configurable.
- **TUI keybindings** — keys are not remappable.
- **TUI external editor integration** — the inline composer is the only entry surface in v1.
- **Session open structure/templates** — the Start event text is used as the implicit session title across CLI/TUI displays and is not templatable.
- **`devlog config timezones`** — a timezone browser command is deferred; use `display.timezone` with an explicit IANA name in v1.
