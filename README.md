# devlog

![devlog banner](docs/assets/banner.png)

A CLI/TUI tool for structured session journals inside git repos. Record context, notes, blockers, and handoffs as you work so a partner, human or AI, can pick up exactly where you left off.

<p><img src="docs/assets/demo.gif" width="600" alt="devlog demo"></p>

## Why devlog?

- **Session context stays in the repo.** No external service, no account.
- **Handoffs are deterministic.** Run `devlog status` and see exactly what's blocked and what's next.
- **Git-friendly.** Session logs are plain Markdown, so `git diff` is meaningful.
- **Never stages or commits.** devlog writes files only; it never stages, commits, or pushes your git changes.

## Installation

### macOS

```bash
brew install amohamma8029/tap/devlog
```

### Linux

```bash
brew install amohamma8029/tap/devlog
```

Or download a native package from the [latest release](https://github.com/amohamma8029/devlog/releases/latest):

- Debian/Ubuntu: `devlog_<version>_linux_amd64.deb` or `devlog_<version>_linux_arm64.deb`
- Fedora/RHEL: `devlog_<version>_linux_amd64.rpm` or `devlog_<version>_linux_arm64.rpm`
- Alpine: `devlog_<version>_linux_amd64.apk` or `devlog_<version>_linux_arm64.apk`

### Windows

PowerShell installer (recommended):

```powershell
irm https://raw.githubusercontent.com/amohamma8029/devlog/main/scripts/install.ps1 | iex
```

The installer verifies the download against the release checksums, installs to `%LOCALAPPDATA%\Programs\devlog\bin`, and adds it to your user PATH. Pin a version with `-Version 1.0.0` or skip PATH changes with `-NoModifyPath`.

WinGet (available after the first public release):

```powershell
winget install amohamma8029.devlog
```

Scoop:

```powershell
scoop bucket add amohamma8029 https://github.com/amohamma8029/scoop-bucket
scoop install devlog
```

### Go

```bash
go install github.com/amohamma8029/devlog@latest
```

### Direct download

Archives for Windows, macOS, and Linux (`amd64` and `arm64`) are attached to every [release](https://github.com/amohamma8029/devlog/releases/latest), alongside a `checksums.txt` file.

## Quick start

```bash
# Open a session
devlog open "Implement auth middleware"

# Add a note
devlog note "Refactored JWT package"

# Log a blocker
devlog block "Waiting on design decision for refresh-token rotation"

# See where things stand
devlog status

# Generate a handoff artifact
devlog handoff

# Close the session
devlog close
```

## Storage

Sessions live in `.devlog/sessions/<timestamp>.md` as Markdown files with a YAML front-matter block for structured metadata.

## Configuration

devlog reads an optional global config file at `~/.config/devlog/config.yml` for author identity, editor selection, display timezone and clock format, handoff diff context, and the TUI handoff preview line limit. See [docs/configuration.md](./docs/configuration.md) for the full schema, defaults, fallback precedence, and validation rules.

## Compatibility

See [docs/compatibility.md](./docs/compatibility.md) for supported platforms, requirements, and what is not supported.

## Contributing

See [AGENTS.md](./AGENTS.md) for the full project conventions, build commands, test instructions, and architecture boundaries.

## License

[MIT](./LICENSE)
=======
## Agent skills

devlog ships a skill that teaches coding agents (Claude Code, opencode, Cursor) the full devlog session lifecycle — pick up context, open a session, log notes and blockers, generate a handoff, and present it in the final reply. The agent never closes sessions; the human reviews and closes them.

### Install with the CLI

```bash
# One tool
devlog skill install claude
devlog skill install opencode
devlog skill install cursor

# All three at once
devlog skill install all

# Overwrite an existing install
devlog skill install claude --force

# Remove
devlog skill uninstall claude
devlog skill uninstall all
```

The command writes `SKILL.md` into the tool's skill directory:

| Tool | Path |
|------|------|
| Claude Code | `~/.claude/skills/devlog/SKILL.md` |
| opencode | `~/.config/opencode/skills/devlog/SKILL.md` |
| Cursor | `~/.cursor/skills/devlog/SKILL.md` |

Restart the tool after installing — skills load at startup.

### Install manually

If `devlog skill install` isn't available (no binary, sandboxed environment), copy the embedded skill file into the tool's skill directory by hand.

The skill source lives at [`internal/skill/SKILL.md`](./internal/skill/SKILL.md) in this repo. It is the single source of truth — the `devlog skill install` command embeds this exact file via `go:embed`.

1. Create the skill directory for your tool:
   ```bash
   # Claude Code
   mkdir -p ~/.claude/skills/devlog

   # opencode
   mkdir -p ~/.config/opencode/skills/devlog

   # Cursor
   mkdir -p ~/.cursor/skills/devlog
   ```

2. Copy the file into it:
   ```bash
   # From a clone of this repo
   cp internal/skill/SKILL.md ~/.claude/skills/devlog/SKILL.md

   # Or download just the file (replace <branch> with main or a tag)
   curl -fsSL https://raw.githubusercontent.com/amohmma8029/devlog/<branch>/internal/skill/SKILL.md \
     -o ~/.claude/skills/devlog/SKILL.md
   ```

3. Restart the tool.

The file must be named `SKILL.md` (uppercase) and live in a `devlog/` folder under the tool's skill root. Both are required — opencode and Claude Code filter out skills without matching folder/file names.
