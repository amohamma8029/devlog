# devlog

![devlog banner](docs/assets/banner.png)

A CLI/TUI tool for structured session journals inside git repos. Record context, notes, blockers, and handoffs as you work so a partner, human or AI, can pick up exactly where you left off.

![devlog demo](docs/assets/demo.gif)

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
