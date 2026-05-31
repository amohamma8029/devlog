package handoff

import (
	"strings"
	"testing"
)

// ── Session content fixtures ─────────────────────────────────────────────────

const sessionNormal = `---
id: 2026-01-15T143022Z
author: Ayman
email: ayman@example.com
started: 2026-01-15T14:30:22Z
branch: feat/auth
status: active
---

## Start

Starting work on the OAuth implementation.

## Note - 2026-01-15 15:00 UTC

Explored PKCE flow options in the auth middleware

## Blocker - 2026-01-15 16:00 UTC

Waiting for security review on token rotation

## Note - 2026-01-15 17:00 UTC

Added unit tests for the refresh token path
`

const sessionNoNotes = `---
id: 2026-01-15T143022Z
author: Ayman
email: ayman@example.com
started: 2026-01-15T14:30:22Z
branch: feat/auth
status: active
---

## Start

Starting work.

## Blocker - 2026-01-15 16:00 UTC

Waiting for security review on token rotation
`

const sessionNoBlockers = `---
id: 2026-01-15T143022Z
author: Ayman
email: ayman@example.com
started: 2026-01-15T14:30:22Z
branch: feat/auth
status: active
---

## Start

Starting work.

## Note - 2026-01-15 15:00 UTC

Explored PKCE flow options.
`

const sessionNoEntries = `---
id: 2026-01-15T143022Z
author: Ayman
email: ayman@example.com
started: 2026-01-15T14:30:22Z
branch: feat/auth
status: active
---

## Start

Starting work.
`

const sessionClosed = `---
id: 2026-01-15T143022Z
author: Ayman
email: ayman@example.com
started: 2026-01-15T14:30:22Z
branch: feat/auth
status: active
---

## Start

Starting work.

## Note - 2026-01-15 15:00 UTC

Implemented PKCE flow.

## Stop - 2026-01-15 18:00 UTC

Session closed.
`

// ── Diff fixtures ────────────────────────────────────────────────────────────

const diffSimple = `diff --git a/src/auth.go b/src/auth.go
index 1234567..abcdefg 100644
--- a/src/auth.go
+++ b/src/auth.go
@@ -10,7 +10,12 @@ func handleAuth() {
-	oldAuthLogic()
+	newPKCEFlow()
+	setupTokenRotation()
 unchanged context
+	validateSession()
`

const diffNewFile = `diff --git a/src/middleware.go b/src/middleware.go
new file mode 100644
index 0000000..1234567
--- /dev/null
+++ b/src/middleware.go
@@ -0,0 +1,5 @@
+package auth
+
+func newMiddleware() {
+	return nil
+}
`

const diffDeletedFile = `diff --git a/src/legacy.go b/src/legacy.go
deleted file mode 100644
index 1234567..0000000
--- a/src/legacy.go
+++ /dev/null
@@ -1,25 +0,0 @@
-package auth
-
-func oldAuth() error {
-	return errors.New("deprecated")
-}
-
-func legacyFlow() string {
-	return "legacy"
-}
`

const diffRenamed = `diff --git a/src/old.go b/src/new.go
similarity index 100%
rename from old.go
rename to new.go
--- a/src/old.go
+++ b/src/new.go
@@ -1,5 +1,5 @@
 package auth
 
-func oldName() {}
+func newName() {}
`

const diffBinary = `diff --git a/assets/logo.png b/assets/logo.png
index 1234567..abcdefg 100644
--- a/assets/logo.png
+++ b/assets/logo.png
Binary files a/assets/logo.png and b/assets/logo.png differ
`

const diffMultipleFiles = `diff --git a/src/auth.go b/src/auth.go
index 1234567..abcdefg 100644
--- a/src/auth.go
+++ b/src/auth.go
@@ -10,7 +10,12 @@ func handleAuth() {
-	oldAuthLogic()
+	newPKCEFlow()
+	setupTokenRotation()
 unchanged context
+	validateSession()
diff --git a/src/token.go b/src/token.go
index 2234567..vwxyz 100644
--- a/src/token.go
+++ b/src/token.go
@@ -5,4 +5,6 @@ import "fmt"
 func refreshToken() string {
+	newRefresh()
+	return "token"
 }
diff --git a/src/session.go b/src/session.go
index 3234567..ghijkl 100644
--- a/src/session.go
+++ b/src/session.go
@@ -1,3 +1,4 @@
 package auth
+func trackSession() {
+}
`

const diffThreeFilesSameDir = `diff --git a/pkg/utils/string.go b/pkg/utils/string.go
index 1000000..2000000 100644
--- a/pkg/utils/string.go
+++ b/pkg/utils/string.go
@@ -1,0 +1,3 @@
+func trim(s string) string {
+	return strings.TrimSpace(s)
+}
diff --git a/pkg/utils/number.go b/pkg/utils/number.go
index 3000000..4000000 100644
--- a/pkg/utils/number.go
+++ b/pkg/utils/number.go
@@ -2,1 +2,2 @@ import "math"
+func round(f float64) float64 { return math.Round(f) }
diff --git a/pkg/utils/slice.go b/pkg/utils/slice.go
index 5000000..6000000 100644
--- a/pkg/utils/slice.go
+++ b/pkg/utils/slice.go
@@ -1,0 +1,2 @@
+func contains(s []string, v string) bool {
+	return false
+}
`

const diffWithWarning = `WARNING: Working tree has merge conflicts / unmerged files.
diff --git a/src/auth.go b/src/auth.go
index 1234567..abcdefg 100644
--- a/src/auth.go
+++ b/src/auth.go
@@ -10,7 +10,12 @@ func handleAuth() {
-	oldAuthLogic()
+	newPKCEFlow()
`

// ── Tests ────────────────────────────────────────────────────────────────────

func TestGenerateHeader(t *testing.T) {
	out, err := Generate(sessionNormal, "")
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	if !strings.Contains(out, "# Handoff: feat/auth — 2026-01-15T143022Z (Ayman) [active]") {
		t.Errorf("expected header with branch, id, author, status, got: %s", firstLine(out))
	}
}

func TestGenerateClosedSession(t *testing.T) {
	out, err := Generate(sessionClosed, "")
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	if !strings.Contains(out, "[closed]") {
		t.Errorf("expected closed status in header, got: %s", firstLine(out))
	}
}

func TestGenerateSummaryWithNotesAndBlockers(t *testing.T) {
	out, err := Generate(sessionNormal, "")
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	if !strings.Contains(out, "Progress: ") {
		t.Error("expected Progress line in summary")
	}
	if !strings.Contains(out, "Blockers: ") {
		t.Error("expected Blockers line in summary")
	}
	if !strings.Contains(out, "Explored PKCE flow options") {
		t.Error("expected first note body")
	}
	if !strings.Contains(out, "Added unit tests for the refresh token path") {
		t.Error("expected second note body")
	}
	if !strings.Contains(out, "Waiting for security review on token rotation") {
		t.Error("expected blocker body")
	}
}

func TestGenerateSummaryNoNotes(t *testing.T) {
	out, err := Generate(sessionNoNotes, "")
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	if strings.Contains(out, "Progress:") {
		t.Error("Progress line should not appear when there are no notes")
	}
	if !strings.Contains(out, "Blockers: ") {
		t.Error("expected Blockers line in summary")
	}
}

func TestGenerateSummaryNoBlockers(t *testing.T) {
	out, err := Generate(sessionNoBlockers, "")
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	if !strings.Contains(out, "Progress: ") {
		t.Error("expected Progress line in summary")
	}
	if strings.Contains(out, "Blockers:") {
		t.Error("Blockers line should not appear when there are no blockers")
	}
}

func TestGenerateSummaryNoEntries(t *testing.T) {
	out, err := Generate(sessionNoEntries, "")
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	if !strings.Contains(out, "No entries recorded.") {
		t.Error("expected 'No entries recorded.' when there are no notes or blockers")
	}
}

func TestGenerateEmptyDiff(t *testing.T) {
	out, err := Generate(sessionNormal, "")
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	if !strings.Contains(out, "No code changes during this session.") {
		t.Error("expected 'No code changes' for empty diff")
	}
	if strings.Contains(out, "```diff") {
		t.Error("raw diff block should not appear when diff is empty")
	}
}

func TestGenerateWithDiff(t *testing.T) {
	out, err := Generate(sessionNormal, diffSimple)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	if !strings.Contains(out, "```diff") {
		t.Error("expected raw diff block")
	}
	if !strings.Contains(out, "src/auth.go") {
		t.Error("expected file path in raw diff")
	}
	if strings.Contains(out, "@@") {
		t.Error("hunk headers should be stripped from raw diff")
	}
	if strings.Contains(out, "diff --git") {
		t.Error("diff --git headers should be stripped from raw diff")
	}
	if strings.Contains(out, "index ") {
		t.Error("index lines should be stripped from raw diff")
	}
}

func TestGenerateNewFile(t *testing.T) {
	out, err := Generate(sessionNormal, diffNewFile)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	if !strings.Contains(out, "Created `src/middleware.go` (+5 lines)") {
		t.Errorf("expected Created prose for new file, got:\n%s", out)
	}
}

func TestGenerateDeletedFile(t *testing.T) {
	out, err := Generate(sessionNormal, diffDeletedFile)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	if !strings.Contains(out, "Deleted `src/legacy.go` (9 lines)") {
		t.Errorf("expected Deleted prose for deleted file, got:\n%s", out)
	}
}

func TestGenerateRenamedFile(t *testing.T) {
	out, err := Generate(sessionNormal, diffRenamed)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	if !strings.Contains(out, "Renamed `old.go` to `new.go`") {
		t.Errorf("expected Renamed prose for renamed file, got:\n%s", out)
	}
}

func TestGenerateBinaryFile(t *testing.T) {
	out, err := Generate(sessionNormal, diffBinary)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	if !strings.Contains(out, "Binary file `assets/logo.png` changed.") {
		t.Errorf("expected binary file mention in prose, got:\n%s", out)
	}
	if strings.Contains(out, "Binary files") && strings.Contains(out, "```diff") {
		t.Error("binary files should be excluded from raw diff block")
	}
}

func TestGenerateDirectoryClustering(t *testing.T) {
	out, err := Generate(sessionNormal, diffThreeFilesSameDir)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	if !strings.Contains(out, "Updated 3 files in `pkg/utils/`") {
		t.Errorf("expected directory clustering for 3+ files in same dir, got:\n%s", out)
	}
}

func TestGenerateMultipleFilesNoClustering(t *testing.T) {
	out, err := Generate(sessionNormal, diffMultipleFiles)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	// 3 files in src/ should trigger clustering.
	if !strings.Contains(out, "Updated 3 files in `src/`") {
		t.Errorf("expected directory clustering for 3 files in src/, got:\n%s", out)
	}
}

func TestGenerateWithWarning(t *testing.T) {
	out, err := Generate(sessionNormal, diffWithWarning)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	if !strings.Contains(out, "WARNING: Working tree has merge conflicts") {
		t.Errorf("expected warning line in handoff header, got:\n%s", out)
	}
}

func TestGenerateLargeDiffTruncation(t *testing.T) {
	var sb strings.Builder
	sb.WriteString("diff --git a/src/big.go b/src/big.go\n")
	sb.WriteString("new file mode 100644\n")
	sb.WriteString("index 0000000..1234567\n")
	sb.WriteString("--- /dev/null\n")
	sb.WriteString("+++ b/src/big.go\n")
	sb.WriteString("@@ -0,0 +1,300 @@\n")

	for i := 0; i < 300; i++ {
		sb.WriteString("+func doSomething() { return }\n")
	}

	out, err := Generate(sessionNormal, sb.String())
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	if !strings.Contains(out, "diff truncated") {
		t.Errorf("expected diff truncation message for large diff, got:\n%s", out)
	}
}

func TestGenerateWhitespaceOnlyChangesExcludedFromProse(t *testing.T) {
	wsDiff := `diff --git a/src/format.go b/src/format.go
index 1234567..abcdefg 100644
--- a/src/format.go
+++ b/src/format.go
@@ -1,5 +1,5 @@
 package auth
+
-
+
 func main() {
 }
`

	out, err := Generate(sessionNormal, wsDiff)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	if strings.Contains(out, "src/format.go") && !strings.Contains(out, "```diff") {
		t.Errorf("whitespace-only file should not appear in prose but may in diff, got:\n%s", out)
	}
	// The file should be excluded from prose due to >80% whitespace.
	if strings.Contains(out, "Tweaked") || strings.Contains(out, "Modified") || strings.Contains(out, "Created") {
		t.Errorf("whitespace-only file should be excluded from prose, got:\n%s", out)
	}
}

func TestGenerateErrorOnMissingFrontMatter(t *testing.T) {
	_, err := Generate("just text, no front-matter", "")
	if err == nil {
		t.Fatal("expected error for missing front-matter")
	}
	if !strings.Contains(err.Error(), "missing opening front-matter delimiter") {
		t.Errorf("expected front-matter error, got: %v", err)
	}
}

func TestGenerateErrorOnMissingID(t *testing.T) {
	badSession := `---
author: Ayman
branch: feat/auth
---

## Start

hello
`
	_, err := Generate(badSession, "")
	if err == nil {
		t.Fatal("expected error for missing id")
	}
	if !strings.Contains(err.Error(), "missing required field: id") {
		t.Errorf("expected missing id error, got: %v", err)
	}
}

func firstLine(s string) string {
	idx := strings.Index(s, "\n")
	if idx < 0 {
		return s
	}
	return s[:idx]
}
