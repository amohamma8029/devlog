# AGENTS.md — devlog
# A CLI/TUI tool for structured session journals inside git repos.
# Used by developers (human or AI) to capture session state, blockers,
# and handoffs. Written in Go; binary is repo-local, no server.

## ── REPO FACTS (3–5 lines — read in 10 seconds) ────────────────────────────
# What this is, who uses it, one key constraint or rule a new contributor needs first.
A CLI tool that converts Figma design tokens to platform-specific theme files
(iOS, Android, CSS). Used by the design-systems team. ESM-only, no framework.
A CLI/TUI tool that records structured coding session journals inside a git repo.
Used by solo developers or async coding partners (human or AI) to capture context,
blockers, and handoffs. One global devlog per repo. Never auto-commits.

## ── COMMANDS (put these first — agents reference them constantly) ────────────
mise install                             # install pinned Go version (if using mise)
go build -o devlog.exe ./cmd/devlog       # compile
go test ./...                              # run after EVERY change; stop on first failure
go test ./... -v                           # verbose output for debugging

## Stack
Go 1.22+, Cobra (CLI), Bubble Tea + Bubbles + Lipgloss + Glamour (TUI Phase 2).
Storage: YAML front-matter + Markdown body, one file per session in .devlog/sessions/.

## ── PROJECT STRUCTURE ────────────────────────────────────────────────────────
cmd/devlog/          — CLI entrypoint (main.go + Cobra command wiring)
internal/session/    — Session lifecycle (start, stop, list, find active)
internal/store/      — File I/O for .devlog/sessions/*.md (YAML front-matter + Markdown body)
internal/handoff/    — Summary artifact generation from session data
internal/git/        — Repo root detection, branch name, author identity from git config
internal/tui/        — Bubble Tea interactive interface (Phase 2)

## ── CODE STYLE ───────────────────────────────────────────────────────────────
# Conventions — concrete rules beat vague descriptions
- Return explicit errors; never swallow them with blank identifiers
- One session = one file in .devlog/sessions/; append-only during a session
- Never auto-commit or stage git changes; the tool writes files only
- Author identity from git config user.name/email; env fallback DEVLOG_AUTHOR_NAME / DEVLOG_AUTHOR_EMAIL
- Go standard naming: PascalCase exported, camelCase unexported
- No runtime dependencies added without asking first

# Code example
// Good: explicit error, clear guard, exported type
func (s *Store) WriteSession(sess Session) error {
	if sess.ID == "" {
		return fmt.Errorf("WriteSession: session ID is empty")
	}
	// write file...
}

// Avoid: swallowing errors, unclear variable names
func (s *Store) Write(sess Session) {
	_ = os.WriteFile(path, data, 0644)  // silent failure
}

## ── ARCHITECTURE BOUNDARIES ──────────────────────────────────────────────────
# Describe the data flow so the agent understands where things belong.
1. User runs CLI command → cmd/devlog/ parses flags → dispatches to internal/
2. internal/session/ validates business rules → calls internal/store/ for I/O
3. internal/store/ reads/writes .devlog/sessions/*.md (YAML front-matter + Markdown body)
4. internal/handoff/ consumes session data from store/ → generates summary artifact
5. internal/git/ provides repo root, branch, and author metadata (no git mutations)
6. internal/tui/ (Phase 2) wraps the same internal/ packages in a Bubble Tea interface

## ── GIT WORKFLOW ─────────────────────────────────────────────────────────────
# Commit message format — write about user impact, not implementation detail
# Good
feat(session): add stop command to close active session
fix(store): prevent overwriting existing session file on duplicate start

# Avoid
refactor: update WriteSession to use os.Create instead of os.OpenFile

Branch naming: feat/<short-slug>, fix/<short-slug>, chore/<short-slug>
Parent feature branches: feat/<feature-name> (e.g., feat/session-storage)
Slice branches: feat/<feature-name>-<slice> (e.g., feat/session-storage-yaml)
Never push directly to main. Feature branch merges to main only when end-to-end complete.

## ── BOUNDARIES ───────────────────────────────────────────────────────────────
# Three-tier format: always / ask first / never

# Always
- Run go test ./... after every change; stop immediately if any test fails
- Return explicit errors; never swallow them
- Keep store/ append-only during a session; never rewrite the full file for a single event

# Ask first
- Adding any new third-party dependency
- Changing the storage format (file layout, front-matter schema, Markdown structure)
- Modifying the public API of internal/session/ or internal/store/
- Introducing a config file or changing author identity resolution logic

# Never
- Auto-commit, auto-stage, or run any git mutation (add, commit, push, etc.)
- Read from or write to files outside the repo root
- Hardcode author identity or paths
- Modify generated binary artifacts by hand

## ── ENV / SECRETS ────────────────────────────────────────────────────────────
No secrets needed for local dev.
Optional env vars: DEVLOG_AUTHOR_NAME, DEVLOG_AUTHOR_EMAIL (fallback if git config unset).

## ── TROUBLESHOOTING ─────────────────────────────────────────────────────────
"cannot find package" after adding import → run go mod tidy
"no such file or directory" on devlog.exe → run go build -o devlog.exe ./cmd/devlog first
Test failure after store change → review store_test.go; store/ logic must be pure and deterministic
Type error on Session struct → ensure Session fields match the YAML front-matter schema in store/
