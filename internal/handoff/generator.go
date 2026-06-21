package handoff

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// GenerateOptions controls optional handoff output sections.
type GenerateOptions struct {
	ExcludeRawDiff bool
	DiffLineLimit  int
}

// Generate produces a narrative handoff summary from raw session file content and a git diff string.
// Both inputs are plain strings — the generator never touches the filesystem or executes git.
func Generate(sessionContent, diff string) (string, error) {
	return GenerateWithOptions(sessionContent, diff, GenerateOptions{})
}

// GenerateWithOptions produces a narrative handoff summary with caller-controlled diff output.
func GenerateWithOptions(sessionContent, diff string, opts GenerateOptions) (string, error) {
	meta, events, err := parseSession(sessionContent)
	if err != nil {
		return "", fmt.Errorf("GenerateWithOptions: parse session: %w", err)
	}

	status := deriveStatus(events)
	diffInfo := parseDiff(diff)

	var buf strings.Builder

	buf.WriteString(formatHeader(meta, status))
	if len(diffInfo.warnings) > 0 {
		for _, w := range diffInfo.warnings {
			buf.WriteString(w)
			buf.WriteByte('\n')
		}
		buf.WriteByte('\n')
	}
	buf.WriteString("## Summary\n")
	buf.WriteString(formatSummary(events))
	buf.WriteString(formatProseChanges(diffInfo))
	if !opts.ExcludeRawDiff {
		buf.WriteString(formatRawDiff(diffInfo, opts.DiffLineLimit))
	}

	return buf.String(), nil
}

// ── Session parsing ──────────────────────────────────────────────────────────

type sessionMeta struct {
	ID     string `yaml:"id"`
	Author string `yaml:"author"`
	Branch string `yaml:"branch"`
}

type event struct {
	Type string // Start, Note, Blocker, Stop
	Time string // YYYY-MM-DD HH:MM (empty for Start events)
	Body string
}

func parseSession(content string) (*sessionMeta, []event, error) {
	meta, body, err := extractFrontMatter(content)
	if err != nil {
		return nil, nil, err
	}

	events := parseEvents(body)
	return meta, events, nil
}

func extractFrontMatter(content string) (*sessionMeta, string, error) {
	const delim = "---\n"
	if !strings.HasPrefix(content, delim) {
		return nil, "", fmt.Errorf("missing opening front-matter delimiter")
	}

	rest := content[len(delim):]
	parts := strings.SplitN(rest, delim, 2)
	if len(parts) < 2 {
		return nil, "", fmt.Errorf("missing closing front-matter delimiter")
	}

	var meta sessionMeta
	if err := yaml.Unmarshal([]byte(parts[0]), &meta); err != nil {
		return nil, "", fmt.Errorf("unmarshal front-matter: %w", err)
	}

	if meta.ID == "" {
		return nil, "", fmt.Errorf("front-matter missing required field: id")
	}

	return &meta, parts[1], nil
}

var eventHeaderRe = regexp.MustCompile(`^## (Start|Note|Blocker|Stop)(?: - (\d{4}-\d{2}-\d{2} \d{2}:\d{2}) UTC)?$`)

func parseEvents(body string) []event {
	lines := strings.Split(body, "\n")
	var events []event
	var currentEvent *event

	for _, line := range lines {
		matches := eventHeaderRe.FindStringSubmatch(strings.TrimSpace(line))
		if matches != nil {
			if currentEvent != nil {
				currentEvent.Body = strings.TrimSpace(currentEvent.Body)
				events = append(events, *currentEvent)
			}
			currentEvent = &event{
				Type: matches[1],
				Time: matches[2],
			}
			continue
		}
		if currentEvent != nil {
			if currentEvent.Body != "" {
				currentEvent.Body += "\n"
			}
			currentEvent.Body += line
		}
	}

	if currentEvent != nil {
		currentEvent.Body = strings.TrimSpace(currentEvent.Body)
		events = append(events, *currentEvent)
	}

	return events
}

func deriveStatus(events []event) string {
	for _, e := range events {
		if e.Type == "Stop" {
			return "closed"
		}
	}
	return "active"
}

// ── Output formatting ────────────────────────────────────────────────────────

func formatHeader(meta *sessionMeta, status string) string {
	return fmt.Sprintf("# Handoff: %s — %s (%s) [%s]\n\n", meta.Branch, meta.ID, meta.Author, status)
}

func formatSummary(events []event) string {
	var notes []string
	var blockers []string

	for _, e := range events {
		if e.Body == "" {
			continue
		}
		switch e.Type {
		case "Note":
			notes = append(notes, e.Body)
		case "Blocker":
			blockers = append(blockers, e.Body)
		}
	}

	if len(notes) == 0 && len(blockers) == 0 {
		return "No entries recorded.\n\n"
	}

	var buf strings.Builder

	if len(notes) > 0 {
		buf.WriteString("Progress: ")
		buf.WriteString(joinProse(notes))
		buf.WriteString("\n")
	}

	if len(blockers) > 0 {
		buf.WriteString("Blockers: ")
		buf.WriteString(joinProse(blockers))
		buf.WriteString("\n")
	}

	buf.WriteString("\n")
	return buf.String()
}

func joinProse(bodies []string) string {
	parts := make([]string, len(bodies))
	for i, b := range bodies {
		parts[i] = strings.TrimSpace(b)
	}
	result := strings.Join(parts, "; ")
	if result != "" && !strings.HasSuffix(result, ".") && !strings.HasSuffix(result, "!") && !strings.HasSuffix(result, "?") {
		result += "."
	}
	return result
}

// ── Diff parsing ─────────────────────────────────────────────────────────────

type diffSummary struct {
	warnings []string
	files    []diffFile
}

type diffFile struct {
	path         string
	oldPath      string
	isNew        bool
	isDeleted    bool
	isBinary     bool
	isSubmodule  bool
	isRenamed    bool
	renameFrom   string
	renameTo     string
	addedLines   int
	deletedLines int
	hunkCount    int
	isWhitespace bool
	rawLines     []string // cleaned lines for raw diff block
}

func parseDiff(diff string) diffSummary {
	if strings.TrimSpace(diff) == "" {
		return diffSummary{}
	}

	var warnings []string
	remaining := diff

	for {
		idx := strings.Index(remaining, "diff --git")
		if idx < 0 {
			break
		}
		prefix := remaining[:idx]
		if strings.TrimSpace(prefix) == "" {
			remaining = remaining[idx:]
			break
		}
		for _, line := range strings.Split(strings.TrimSpace(prefix), "\n") {
			line = strings.TrimSpace(line)
			if line != "" && strings.HasPrefix(line, "WARNING:") {
				warnings = append(warnings, line)
			}
		}
		remaining = remaining[idx:]
		break
	}

	sections := splitDiffSections(remaining)
	var files []diffFile

	for _, section := range sections {
		f := parseDiffSection(section)
		files = append(files, f)
	}

	return diffSummary{warnings: warnings, files: files}
}

func splitDiffSections(diff string) []string {
	var sections []string
	lines := strings.Split(diff, "\n")

	var current []string
	started := false

	for _, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "diff --git") {
			if started && len(current) > 0 {
				sections = append(sections, strings.Join(current, "\n"))
			}
			current = []string{line}
			started = true
			continue
		}
		if !started && strings.HasPrefix(strings.TrimSpace(line), "Binary files") && strings.Contains(line, "differ") {
			current = []string{line}
			started = true
			continue
		}
		if started {
			current = append(current, line)
		}
	}

	if started && len(current) > 0 {
		sections = append(sections, strings.Join(current, "\n"))
	}

	return sections
}

func parseDiffSection(section string) diffFile {
	lines := strings.Split(section, "\n")
	f := diffFile{}
	f.rawLines = []string{}

	var oldPath, newPath string
	totalWsChanges := 0
	totalChanges := 0
	inHunk := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Binary file marker.
		if strings.Contains(trimmed, "Binary files") && strings.Contains(trimmed, "differ") {
			f.isBinary = true
			f.path = extractBinaryPath(line)
			continue
		}

		// diff --git header.
		if strings.HasPrefix(trimmed, "diff --git ") {
			parts := strings.Fields(trimmed)
			if len(parts) >= 4 {
				oldPath = strings.TrimPrefix(parts[2], "a/")
				newPath = strings.TrimPrefix(parts[3], "b/")
			}
			continue
		}

		// Extended headers — strip from output.
		if strings.HasPrefix(trimmed, "index ") ||
			strings.HasPrefix(trimmed, "similarity index ") ||
			strings.HasPrefix(trimmed, "dissimilarity index ") ||
			strings.HasPrefix(trimmed, "old mode ") ||
			strings.HasPrefix(trimmed, "new mode ") ||
			strings.HasPrefix(trimmed, "copy from ") ||
			strings.HasPrefix(trimmed, "copy to ") {
			continue
		}
		if strings.HasPrefix(trimmed, "new file mode") {
			f.isNew = true
			continue
		}
		if strings.HasPrefix(trimmed, "deleted file mode") {
			f.isDeleted = true
			continue
		}
		if strings.HasPrefix(trimmed, "rename from ") {
			f.isRenamed = true
			f.renameFrom = strings.TrimPrefix(trimmed, "rename from ")
			continue
		}
		if strings.HasPrefix(trimmed, "rename to ") {
			f.renameTo = strings.TrimPrefix(trimmed, "rename to ")
			continue
		}

		// --- a/path and +++ b/path — strip from output.
		if strings.HasPrefix(trimmed, "--- ") || strings.HasPrefix(trimmed, "+++ ") {
			continue
		}

		// Hunk header — count but strip from raw output.
		if strings.HasPrefix(trimmed, "@@") {
			f.hunkCount++
			inHunk = true
			continue
		}

		// Content lines.
		if strings.HasPrefix(line, "+") {
			f.addedLines++
			totalChanges++
			content := line[1:]
			if strings.TrimSpace(content) == "" {
				totalWsChanges++
			}
			f.rawLines = append(f.rawLines, line)
			continue
		}
		if strings.HasPrefix(line, "-") {
			f.deletedLines++
			totalChanges++
			content := line[1:]
			if strings.TrimSpace(content) == "" {
				totalWsChanges++
			}
			f.rawLines = append(f.rawLines, line)
			continue
		}
		if inHunk && strings.HasPrefix(line, " ") {
			f.rawLines = append(f.rawLines, line)
			continue
		}

		// \ No newline at end of file.
		if strings.HasPrefix(trimmed, `\ No newline`) {
			continue
		}

		// Submodule detection.
		if strings.Contains(trimmed, "Subproject commit") {
			f.isSubmodule = true
		}
	}

	// Determine display path.
	if f.isRenamed {
		f.path = f.renameFrom + " → " + f.renameTo
		f.oldPath = f.renameFrom
	} else if f.isNew {
		f.path = newPath
	} else if f.isDeleted {
		f.path = oldPath
	} else {
		f.path = newPath
		if f.path == "" {
			f.path = oldPath
		}
	}

	if f.path == "" {
		f.path = oldPath
	}

	// Detect whitespace-only changes (>80% of changed lines are blank).
	if totalChanges > 0 && totalWsChanges > 0 && (totalWsChanges*100/totalChanges) > 80 {
		f.isWhitespace = true
	}

	return f
}

func extractBinaryPath(line string) string {
	parts := strings.Fields(line)
	for _, p := range parts {
		if strings.Contains(p, "/") && !strings.Contains(p, "and") && !strings.Contains(p, "Binary") && !strings.Contains(p, "differ") {
			trimmed := strings.TrimPrefix(p, "a/")
			trimmed = strings.TrimPrefix(trimmed, "b/")
			return trimmed
		}
	}
	return "unknown"
}

// ── Prose diff formatting ────────────────────────────────────────────────────

func formatProseChanges(info diffSummary) string {
	var buf strings.Builder
	buf.WriteString("## Changes\n")

	// Filter out whitespace-only files.
	var files []diffFile
	for _, f := range info.files {
		if !f.isWhitespace {
			files = append(files, f)
		}
	}

	if len(files) == 0 && len(info.files) == 0 {
		buf.WriteString("No code changes during this session.\n\n")
		return buf.String()
	}

	if len(files) == 0 {
		buf.WriteString("No code changes during this session.\n\n")
		return buf.String()
	}

	// Cluster by directory.
	dirCounts := make(map[string][]diffFile)
	dirOrder := make([]string, 0) // preserve order
	seenDirs := make(map[string]bool)
	for _, f := range files {
		dir := dirName(f.path)
		dirCounts[dir] = append(dirCounts[dir], f)
		if !seenDirs[dir] {
			seenDirs[dir] = true
			dirOrder = append(dirOrder, dir)
		}
	}
	sort.Strings(dirOrder)

	var proseLines []string
	count := 0
	remaining := 0

	for _, dir := range dirOrder {
		group := dirCounts[dir]
		if len(group) >= 3 && dir != "." {
			if count >= 10 {
				remaining += len(group)
				continue
			}
			count++
			proseLines = append(proseLines, fmt.Sprintf("- Updated %d files in `%s/`", len(group), dir))
			for _, f := range group {
				proseLines = append(proseLines, fmt.Sprintf("  - %s", fileProse(f)))
			}
			continue
		}
		for _, f := range group {
			if count >= 10 {
				remaining++
				continue
			}
			count++
			proseLines = append(proseLines, "- "+fileProse(f))
		}
	}

	for _, line := range proseLines {
		buf.WriteString(line)
		buf.WriteByte('\n')
	}

	if remaining > 0 {
		buf.WriteString(fmt.Sprintf("- +%d more files changed.\n", remaining))
	}

	buf.WriteByte('\n')
	return buf.String()
}

func dirName(path string) string {
	idx := strings.LastIndex(path, "/")
	if idx < 0 {
		return "."
	}
	return path[:idx]
}

func fileProse(f diffFile) string {
	if f.isBinary {
		return fmt.Sprintf("Binary file `%s` changed.", f.path)
	}
	if f.isSubmodule {
		return fmt.Sprintf("Submodule `%s` updated.", f.path)
	}
	if f.isRenamed {
		return fmt.Sprintf("Renamed `%s` to `%s`.", f.renameFrom, f.renameTo)
	}

	if f.isNew {
		if f.hunkCount > 1 {
			return fmt.Sprintf("Created `%s` (+%d lines in %d locations).", f.path, f.addedLines, f.hunkCount)
		}
		return fmt.Sprintf("Created `%s` (+%d lines).", f.path, f.addedLines)
	}
	if f.isDeleted {
		return fmt.Sprintf("Deleted `%s` (%d lines).", f.path, f.deletedLines)
	}

	totalChanged := f.addedLines + f.deletedLines
	locInfo := fmt.Sprintf("+%d/-%d", f.addedLines, f.deletedLines)

	ratio := 0.0
	if f.deletedLines > 0 && f.addedLines > 0 {
		ratio = float64(f.addedLines) / float64(f.deletedLines)
	}

	if ratio > 3.0 {
		return fmt.Sprintf("Added substantial content to `%s` (%s).", f.path, locInfo)
	}
	if f.deletedLines > f.addedLines*3 {
		return fmt.Sprintf("Removed substantial content from `%s` (%s).", f.path, locInfo)
	}

	if totalChanged < 10 {
		return fmt.Sprintf("Tweaked `%s` (%s).", f.path, locInfo)
	}
	if totalChanged > 100 {
		return fmt.Sprintf("Major rewrite of `%s` (%s).", f.path, locInfo)
	}

	if f.hunkCount > 1 {
		return fmt.Sprintf("Modified `%s` (%s in %d locations).", f.path, locInfo, f.hunkCount)
	}
	return fmt.Sprintf("Modified `%s` (%s).", f.path, locInfo)
}

// ── Raw diff formatting ──────────────────────────────────────────────────────

func formatRawDiff(info diffSummary, lineLimit int) string {
	var nonBinary []diffFile
	for _, f := range info.files {
		if !f.isBinary && !f.isSubmodule {
			nonBinary = append(nonBinary, f)
		}
	}

	if len(nonBinary) == 0 {
		return ""
	}

	var buf strings.Builder

	for _, f := range nonBinary {
		path := f.path
		if path == "" {
			path = f.oldPath
		}

		buf.WriteString("#### ")
		buf.WriteString(path)
		buf.WriteString("\n\n")
		buf.WriteString("```diff\n")

		lines := f.rawLines
		if lineLimit > 0 && len(lines) > lineLimit {
			lines = lines[:lineLimit]
		}

		for _, rl := range lines {
			buf.WriteString(rl)
			buf.WriteByte('\n')
		}

		if lineLimit > 0 && len(f.rawLines) > lineLimit {
			buf.WriteString(fmt.Sprintf("... (truncated, %d more lines)\n", len(f.rawLines)-lineLimit))
		}

		buf.WriteString("```\n\n")
	}

	return buf.String()
}
