---
name: devlog
description: Use when starting any coding task in a repository with devlog installed. Guides the agent through the devlog session lifecycle — check status and read prior handoffs for context, open a session for the current task, log notes and blockers during work, generate a handoff at the end, and present it with insights in the final reply. Never closes sessions; the human reviews and closes them.
---

# Devlog Session Skill

## When to use
Activate this skill at the start of any coding task in a repository that has devlog installed. Verify devlog is installed with `devlog --version` before activating. This skill requires a git repository — devlog stores session data under `.devlog/` in the repo root. Devlog captures structured session journals — context, notes, blockers, and handoffs — so you and your human collaborators can pick up where the last session left off.

## Session lifecycle

### 1. Start of work — pick up context
- Run `devlog status` to check for an active session and see recent events, blockers, and todos.
- If a handoff exists from a previous session, read the newest file under `.devlog/handoffs/` for context. Reading is fine; never write to `.devlog/` directly — the CLI is the only writer.
- If `devlog status` reports no active session, open one:
  ```
  devlog open "<one-line description of the task>"
  ```
- If a session is already active from a prior task, check `devlog status` — if it's for a different task, ask the human whether to close it and reopen or continue under the existing session. The agent never runs `devlog close` itself. devlog allows only one active session at a time, repo-wide — not per-branch.
- Re-run `devlog status` to confirm the session is active and see its initial state.
- Review the todo list for outstanding work from prior sessions:
  ```
  devlog todo list
  ```
- To see past sessions and their status:
  ```
  devlog list
  ```
  Use `devlog list --active` to show only open sessions.

### 2. During work — log milestones, blockers, and todos
- Run `devlog status` periodically to verify the session state — recent events, blockers, and todos all appear in one view. Use `devlog status -n 0` to see all events or `devlog status -n 20` to see more recent ones (default is 10).
- Add notes at meaningful milestones (after completing a subtask, making a decision, or discovering something worth recording):
  ```
  devlog note -m "<what you did or decided>"
  ```
- Log blockers immediately when you hit one — don't wait:
  ```
  devlog block -m "<what is blocking you and what you tried>"
  ```
- Fix a typo in a note or blocker using its event number from `devlog status`. Event numbers are most-recent-first — `[1]` is the newest event, `[2]` is the one before it, etc. The numbering is the same regardless of the `-n` flag value:
  ```
  devlog edit 2 -m "<corrected text>"
  ```
- Remove an erroneous note or blocker entry:
  ```
  devlog edit 2 --delete
  ```
- Use **todos** for outstanding work that should or will be revisited later — bugs, follow-ups, deferred items, things noticed but not addressed now. Todos added during an active session are auto-attributed to that session and branch:
  ```
  devlog todo add -m "Bug: locale fallback returns empty string on unknown code"
  ```
- Mark todos done as you complete them. Use the number from `devlog todo list`:
  ```
  devlog todo done 2
  ```
- Reopen a todo if something needs revisiting:
  ```
  devlog todo reopen 1
  ```
- Fix a typo in a todo's text (only open todos can be edited — reopen first if you need to fix a completed one):
  ```
  devlog todo edit 1 -m "<corrected text>"
  ```
- Remove a wrongly-added todo:
  ```
  devlog todo delete 1
  ```
- Before editing or deleting, run `devlog todo list` and confirm the todo text matches the number you're targeting — completed todos are numbered after open ones, so the list position may not match your mental model.
- If you need stable identifiers across sessions (list numbers reset per `todo list` call), use `devlog todo list --ids` to see internal IDs.
- Filter todos by branch or session: `devlog todo list --branch feat/auth` or `devlog todo list --session <session-id>`.

### 3. End of work — generate handoff
- When the task is complete or you're handing off, run:
  ```
  devlog handoff
  ```
- This writes a narrative summary to `.devlog/handoffs/<session-id>.md` and prints the path.
- The handoff's Changes section captures uncommitted file changes only. If you've committed your work, the Changes section may show "No code changes" — your notes are the primary record. Run `devlog handoff` before committing if you want the diff to show your changes.
- If you need to regenerate the handoff after more work (the default path already exists), use `devlog handoff -o <name>` to write a new file with a different name.
- Read the full contents of the handoff file at the printed path. You will include this summary in your final reply (step 4).

### 4. Final reply — present handoff plus your insights
In your final reply to the human, include:
- The handoff summary (from the file you just read).
- Your own insights: what was done, remaining risks, and suggested next steps.
- Note that the session remains open for the human to review and close.

## Rules
- **Never run `devlog` without a subcommand.** It launches an interactive TUI that will hang. Use `devlog --version` to check installation or `devlog --help` to see available commands.
- **Never run `devlog close`.** The human reviews and closes sessions; the agent never closes them.
- **Never run `devlog todo prune`.** Pruning bulk-deletes completed todos permanently; leave that to the human. `devlog todo delete <ref>` is allowed — it removes a single wrongly-added todo.
- **Never write to `.devlog/` directly.** The devlog CLI is the only writer. You may read files under `.devlog/` (sessions, handoffs) for context.
- **Never auto-commit or stage git changes.** Devlog writes files only; it never touches git.
- If `devlog status` or `devlog handoff` fails with a todo parse error, the `.devlog/todos.md` file may be corrupted. Tell the human — they can restore or delete the file. Do not attempt to fix `.devlog/` files yourself.
- If `devlog status` or `devlog handoff` fails because no session is active, run `devlog open "<task>"` first.
- Keep note and blocker messages concise and factual.
