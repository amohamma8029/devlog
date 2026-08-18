---
name: devlog
description: Use when starting any coding task in a repository with devlog installed. Guides the agent through the devlog session lifecycle — check status and read prior handoffs for context, open a session for the current task, log notes and blockers during work, generate a handoff at the end, and present it with insights in the final reply. Never closes sessions; the human reviews and closes them.
---

# Devlog Session Skill

## When to use
Activate this skill at the start of any coding task in a repository that has devlog installed. Devlog captures structured session journals — context, notes, blockers, and handoffs — so you and your human collaborators can pick up where the last session left off.

## Session lifecycle

### 1. Start of work — pick up context
- Run `devlog status` to check for an active session and see recent events, blockers, and todos.
- If a handoff exists from a previous session, read the newest file under `.devlog/handoffs/` for context. Reading is fine; never write to `.devlog/` directly — the CLI is the only writer.
- If `devlog status` reports no active session, open one:
  ```
  devlog open "<one-line description of the task>"
  ```

### 2. During work — log milestones and blockers
- Add notes at meaningful milestones (after completing a subtask, making a decision, or discovering something worth recording):
  ```
  devlog note -m "<what you did or decided>"
  ```
- Log blockers immediately when you hit one — don't wait:
  ```
  devlog block -m "<what is blocking you and what you tried>"
  ```

### 3. End of work — generate handoff
- When the task is complete or you're handing off, run:
  ```
  devlog handoff
  ```
- This writes a narrative summary to `.devlog/handoffs/<session-id>.md` and prints the path.
- Read the handoff file at the printed path.

### 4. Final reply — present handoff plus your insights
In your final reply to the human, include:
- The handoff summary (from the file you just read).
- Your own insights: what was done, remaining risks, and suggested next steps.
- Note that the session remains open for the human to review and close.

## Rules
- **Never run `devlog close`.** The human reviews and closes sessions; the agent never closes them.
- **Never write to `.devlog/` directly.** The devlog CLI is the only writer. You may read files under `.devlog/` (sessions, handoffs) for context.
- **Never auto-commit or stage git changes.** Devlog writes files only; it never touches git.
- If `devlog status` or `devlog handoff` fails because no session is active, run `devlog open "<task>"` first.
- Keep note and blocker messages concise and factual.
