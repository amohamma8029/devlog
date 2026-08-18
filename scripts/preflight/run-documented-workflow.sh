#!/usr/bin/env bash
# Test-only fixture: asserts the version contract and runs the documented
# first-use workflow (open -> note -> status -> handoff) in a temporary git
# repository against an installed devlog binary. Used by
# .github/workflows/preflight.yml (archive-matrix and linux-package-matrix
# jobs). Not shipped; not part of the devlog binary.
set -euo pipefail

bin="${1:?usage: run-documented-workflow.sh <devlog-binary>}"
if [[ ! -f "$bin" ]]; then
    echo "FAIL: binary not found: $bin"
    exit 1
fi
bin="$(cd "$(dirname "$bin")" && pwd)/$(basename "$bin")"

step() { echo "== $1"; }

step "version output"
ver=$("$bin" --version 2>&1) || { echo "FAIL: --version exited non-zero"; echo "$ver"; exit 1; }
if ! grep -qE 'version:[[:space:]]*[0-9]+\.[0-9]+\.[0-9]+' <<<"$ver"; then
    echo "FAIL: --version did not report a semantic version:"
    echo "$ver"
    exit 1
fi
if grep -qE 'version:[[:space:]]*v[0-9]' <<<"$ver"; then
    echo "FAIL: --version reports a leading v:"
    echo "$ver"
    exit 1
fi
echo "OK: $(grep -oE 'version:[[:space:]]*[^ ]+' <<<"$ver" | head -1)"

step "documented workflow"
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

repo="$tmp/repo"
mkdir -p "$repo"
cd "$repo"
git init -q -b main
git config user.name "Preflight Agent"
git config user.email "preflight@devlog.local"

marker="PREFLIGHT-NOTE-$(date +%s)"

"$bin" open "Preflight workflow test" >/dev/null 2>&1 || { echo "FAIL: devlog open"; exit 1; }
"$bin" note -m "$marker" >/dev/null 2>&1 || { echo "FAIL: devlog note"; exit 1; }

status=$("$bin" status 2>&1) || { echo "FAIL: devlog status exited non-zero"; exit 1; }
grep -q "(active)" <<<"$status" || { echo "FAIL: status does not report an active session"; exit 1; }
grep -qF "$marker" <<<"$status" || { echo "FAIL: status does not show the note"; exit 1; }

handoff=$("$bin" handoff 2>&1) || { echo "FAIL: devlog handoff exited non-zero"; exit 1; }
path=$(grep -oE '\.devlog/handoffs/[0-9TZ-]+\.md' <<<"$handoff" | head -1)
if [[ -z "$path" || ! -f "$repo/$path" ]]; then
    echo "FAIL: handoff file not found at $path"
    echo "$handoff"
    exit 1
fi
grep -qF "$marker" "$repo/$path" || { echo "FAIL: handoff does not contain the note"; exit 1; }

sessions=("$repo"/.devlog/sessions/*.md)
[[ -e "${sessions[0]}" ]] || { echo "FAIL: no session file written"; exit 1; }

echo "OK: open -> note -> status -> handoff complete in temp repo"
echo "PASS: documented workflow verified"