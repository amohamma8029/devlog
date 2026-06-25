# devlog

A CLI/TUI tool for structured session journals inside git repos.

Record context, notes, blockers, and handoffs as you work so that a partner — human or AI — can pick up exactly where you left off.

## Quick start

```bash
# Build
go build -o devlog.exe ./cmd/devlog

# Launch the interactive TUI
./devlog.exe

# Or use CLI subcommands directly

# Open a session
./devlog.exe open "Implement auth middleware"

# Add a note
./devlog.exe note "Refactored JWT package"

# Log a blocker
./devlog.exe block "Waiting on design decision for refresh-token rotation"

# See where things stand
./devlog.exe status

# Track repo-wide follow-up work
./devlog.exe todo add "Follow up on docs"
./devlog.exe todo list
./devlog.exe todo done 1
./devlog.exe todo list
./devlog.exe todo prune --yes

# Generate a handoff artifact
./devlog.exe handoff

# Close the session
./devlog.exe close
```

## Why devlog?

- **Session context stays in the repo.** No external service, no account needed.
- **Handoffs are deterministic.** Run `devlog status` and see exactly what's blocked and what's next.
- **Git-friendly.** Session logs are plain Markdown; `git diff` is meaningful.

## Storage

Sessions live in `.devlog/sessions/<timestamp>.md` as Markdown files with a YAML front-matter block for structured metadata.

## Configuration

devlog reads an optional global config file at `~/.config/devlog/config.yml` for author identity, editor selection, display timezone and clock format, handoff diff context, and the TUI handoff preview line limit. See [docs/configuration.md](./docs/configuration.md) for the full schema, defaults, fallback precedence, and validation rules.

## Contributing

See [AGENTS.md](./AGENTS.md) for the full project conventions, build commands, test instructions, and architecture boundaries.
