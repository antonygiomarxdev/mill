package learning

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

// DefaultMaxLessons is the default cap on the number of active (non-compressed)
// lesson entries retained per role before older ones are folded into the
// summary block.
const DefaultMaxLessons = 50

// lessonEntryRe matches the header line of a persisted lesson entry, e.g.
// `## Lesson 42 · 2026-08-12T10:00:00Z`. Capture group 1 is the entry index,
// group 2 is the recorded-at timestamp.
var lessonEntryRe = regexp.MustCompile(`(?m)^## Lesson (\d+) · (.+)$`)

// Lesson captures the structured learnings from a single agent session for a
// role: the patterns it corrected, the gaps it detected, and the acceptance
// criteria it satisfied.
type Lesson struct {
	CorrectedPatterns  []string
	GapsDetected       []string
	AcceptanceCriteria []string
}

// lessonEntry is a persisted lesson with its sequence number and timestamp.
// raw holds the verbatim rendered body (without the header line); lesson is
// the structured projection parsed from raw, used to build summary lines when
// the entry is compressed.
type lessonEntry struct {
	index  int
	ts     string
	raw    string
	lesson Lesson
}

// Option configures a LessonsRecorder.
type Option func(*LessonsRecorder)

// WithMaxLessons overrides the active-entry cap (DefaultMaxLessons = 50).
// A non-positive value falls back to the default.
func WithMaxLessons(n int) Option {
	return func(r *LessonsRecorder) {
		if n > 0 {
			r.maxEntries = n
		}
	}
}

// LessonsRecorder appends per-role lessons to .mill/roles/<role>/lessons.md.
//
// Lessons accumulate in a single markdown file per role. The active window is
// capped at maxEntries (50 by default); once the cap is exceeded, the oldest
// entries are compressed into a summary block at the top of the file rather
// than discarded.
//
// Active entries are preserved verbatim: only entries that have aged out of the
// active window are folded into the summary, so lesson content is never lost or
// silently overwritten by a later append.
type LessonsRecorder struct {
	millDir    string
	maxEntries int
	mu         sync.Mutex
}

// NewLessonsRecorder returns a recorder that writes to
// .mill/roles/<role>/lessons.md beneath millDir.
func NewLessonsRecorder(millDir string, opts ...Option) *LessonsRecorder {
	r := &LessonsRecorder{
		millDir:    millDir,
		maxEntries: DefaultMaxLessons,
	}
	for _, opt := range opts {
		opt(r)
	}
	return r
}

// Path returns the lessons file for role. Exposed for tests and callers that
// need to inspect the output.
func (r *LessonsRecorder) Path(role string) string {
	return filepath.Join(r.millDir, "roles", role, "lessons.md")
}

// Record appends a lesson for role, enforcing the active-entry cap by
// compressing aged entries into the summary block.
func (r *LessonsRecorder) Record(role string, l Lesson) error {
	if role == "" {
		return fmt.Errorf("learning: role must not be empty")
	}
	if r.maxEntries <= 0 {
		r.maxEntries = DefaultMaxLessons
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	path := r.Path(role)

	raw, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("learning: reading lessons: %w", err)
	}

	title, summary, entries := parseLessonsFile(string(raw), role)

	// Determine the next sequence index.
	maxIdx := 0
	for _, e := range entries {
		if e.index > maxIdx {
			maxIdx = e.index
		}
	}

	entries = append(entries, lessonEntry{
		index:  maxIdx + 1,
		ts:     time.Now().UTC().Format(time.RFC3339),
		raw:    renderLesson(l),
		lesson: l,
	})

	// Enforce the cap: compress the oldest entries into the summary block.
	// Active entries are preserved verbatim; only entries that have aged out
	// of the window are summarized.
	if len(entries) > r.maxEntries {
		toCompress := entries[:len(entries)-r.maxEntries]
		entries = entries[len(entries)-r.maxEntries:]
		for _, e := range toCompress {
			summary = append(summary, compressEntry(e))
		}
	}

	out := renderLessons(title, summary, entries)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("learning: creating role dir: %w", err)
	}
	if err := os.WriteFile(path, []byte(out), 0o644); err != nil {
		return fmt.Errorf("learning: writing lessons: %w", err)
	}
	return nil
}

// ---------------------------------------------------------------------------
// parsing
// ---------------------------------------------------------------------------

// parseLessonsFile splits a lessons.md into its preserved title preamble, the
// list of already-compressed summary lines, and the ordered active entries.
func parseLessonsFile(content, role string) (title string, summary []string, entries []lessonEntry) {
	content = strings.TrimRight(content, "\n")
	if content == "" {
		return fmt.Sprintf("# Lessons for %s", role), nil, nil
	}

	summaryIdx := strings.Index(content, "## Summary")
	if summaryIdx < 0 {
		// No managed summary section: preserve everything as the title
		// preamble (e.g. pre-existing manual notes) and parse any entries.
		return content, nil, parseEntries(content)
	}

	// Title is the preserved preamble preceding the summary section.
	title = content[:summaryIdx]
	rest := content[summaryIdx+len("## Summary"):]

	// Summary body ends at the first "---" separator or first lesson heading.
	body, entriesPart := splitAtSeparator(rest)
	summary = parseSummaryLines(body)
	entries = parseEntries(entriesPart)
	return title, summary, entries
}

// splitAtSeparator returns the summary body and the remainder of s (the entries
// section), cutting at the earliest of a "---" separator or a "## Lesson" heading.
func splitAtSeparator(s string) (body, after string) {
	sep1 := strings.Index(s, "\n---")
	sep2 := strings.Index(s, "\n## Lesson")
	cut := -1
	switch {
	case sep1 < 0:
		cut = sep2
	case sep2 < 0:
		cut = sep1
	default:
		if sep1 < sep2 {
			cut = sep1
		} else {
			cut = sep2
		}
	}
	if cut < 0 {
		return s, ""
	}
	return s[:cut], s[cut:]
}

// parseSummaryLines extracts the non-empty summary lines from a summary body.
func parseSummaryLines(body string) []string {
	var lines []string
	for _, ln := range strings.Split(body, "\n") {
		ln = strings.TrimSpace(ln)
		if ln == "" || strings.HasPrefix(ln, "## ") {
			continue
		}
		lines = append(lines, ln)
	}
	return lines
}

// parseEntries extracts lesson entries from the entries section of the file.
// The raw body of each entry is preserved verbatim.
func parseEntries(s string) []lessonEntry {
	matches := lessonEntryRe.FindAllStringSubmatchIndex(s, -1)
	if len(matches) == 0 {
		return nil
	}
	var entries []lessonEntry
	for i, m := range matches {
		idx, err := strconv.Atoi(s[m[2]:m[3]])
		if err != nil {
			continue
		}
		ts := strings.TrimSpace(s[m[4]:m[5]])

		contentStart := m[1]
		var contentEnd int
		if i+1 < len(matches) {
			contentEnd = matches[i+1][0]
		} else {
			contentEnd = len(s)
		}
		raw := cleanEntryRaw(s[contentStart:contentEnd])

		entries = append(entries, lessonEntry{
			index:  idx,
			ts:     ts,
			raw:    raw,
			lesson: parseLesson(raw),
		})
	}
	return entries
}

// cleanEntryRaw strips a trailing inter-entry "---" separator (and any
// separator whitespace) from the raw body of a parsed lesson entry, so each
// entry is stored verbatim exactly as rendered by renderLesson — without the
// separator that renderLessons inserts between entries at render time.
func cleanEntryRaw(s string) string {
	body := strings.TrimSpace(s)
	// The separator is a line consisting solely of "---"; cut the body at the
	// first such line, preserving everything before it verbatim.
	if i := strings.Index(body, "\n---"); i >= 0 {
		body = strings.TrimSpace(body[:i])
	}
	if body == "" {
		return "\n"
	}
	return body + "\n"
}

// parseLesson extracts the structured Lesson from a rendered entry body.
func parseLesson(content string) Lesson {
	l := Lesson{
		CorrectedPatterns:  parseSection(content, "Corrected patterns"),
		GapsDetected:       parseSection(content, "Gaps detected"),
		AcceptanceCriteria: parseSection(content, "Acceptance criteria"),
	}
	return l
}

// parseSection returns the bulleted items following a `**Name:**` section header
// within content, up to the next section header or end of content.
func parseSection(content, name string) []string {
	header := "**" + name + ":**"
	idx := strings.Index(content, header)
	if idx < 0 {
		return nil
	}
	body := content[idx+len(header):]
	var items []string
	for _, ln := range strings.Split(body, "\n") {
		trimmed := strings.TrimSpace(ln)
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "**") {
			break // start of the next section
		}
		if strings.HasPrefix(trimmed, "- ") {
			items = append(items, strings.TrimSpace(strings.TrimPrefix(trimmed, "- ")))
		}
	}
	return items
}

// ---------------------------------------------------------------------------
// rendering
// ---------------------------------------------------------------------------

// renderLessons assembles the full file from its preserved title, summary lines,
// and active entries.
func renderLessons(title string, summary []string, entries []lessonEntry) string {
	var b strings.Builder
	b.WriteString(strings.TrimSpace(title))
	b.WriteString("\n\n## Summary\n\n")
	if len(summary) == 0 {
		b.WriteString("_No older lessons compressed yet._")
	} else {
		for _, s := range summary {
			b.WriteString(s)
			b.WriteString("\n")
		}
	}
	if len(entries) > 0 {
		b.WriteString("\n---\n\n")
		for i, e := range entries {
			fmt.Fprintf(&b, "## Lesson %d · %s\n", e.index, e.ts)
			// The raw body is written verbatim (it already ends in "\n").
			// A single blank line separates consecutive entries — no
			// inter-entry "---" is emitted, so the body region never
			// carries a trailing separator.
			b.WriteString(e.raw)
			if i < len(entries)-1 {
				b.WriteString("\n")
			}
		}
	}
	b.WriteString("\n")
	return b.String()
}

// renderLesson renders a Lesson's body (without the header line).
func renderLesson(l Lesson) string {
	var b strings.Builder
	writeSection := func(label string, items []string) {
		if len(items) == 0 {
			return
		}
		fmt.Fprintf(&b, "**%s:**\n", label)
		for _, it := range items {
			fmt.Fprintf(&b, "- %s\n", it)
		}
		b.WriteString("\n")
	}
	writeSection("Corrected patterns", l.CorrectedPatterns)
	writeSection("Gaps detected", l.GapsDetected)
	writeSection("Acceptance criteria", l.AcceptanceCriteria)

	if b.Len() == 0 {
		return "_No structured data captured._\n"
	}
	return strings.TrimRight(b.String(), "\n") + "\n"
}

// compressEntry renders a summary line for a lesson being folded into the
// summary block.
func compressEntry(e lessonEntry) string {
	p := len(e.lesson.CorrectedPatterns)
	g := len(e.lesson.GapsDetected)
	c := len(e.lesson.AcceptanceCriteria)

	var details []string
	if p > 0 {
		details = append(details, fmt.Sprintf("%d corrected pattern(s)", p))
	}
	if g > 0 {
		details = append(details, fmt.Sprintf("%d gap(s) detected", g))
	}
	if c > 0 {
		details = append(details, fmt.Sprintf("%d acceptance criteria", c))
	}
	detail := strings.Join(details, ", ")
	if detail == "" {
		detail = "no structured data"
	}
	return fmt.Sprintf("- #%d · %s — compressed (%s)", e.index, e.ts, detail)
}
