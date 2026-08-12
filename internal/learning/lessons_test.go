package learning

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestLessonsRecorder_RecordCreatesFile(t *testing.T) {
	dir := filepath.Join(t.TempDir(), ".mill")
	r := NewLessonsRecorder(dir)

	role := "architect"
	lesson := Lesson{
		CorrectedPatterns:  []string{"prefer interfaces over implementations"},
		GapsDetected:       []string{"no ADR for the cache boundary"},
		AcceptanceCriteria: []string{"ADR-001 committed"},
	}
	if err := r.Record(role, lesson); err != nil {
		t.Fatalf("Record failed: %v", err)
	}

	content, err := os.ReadFile(r.Path(role))
	if err != nil {
		t.Fatalf("reading lessons file: %v", err)
	}
	s := string(content)

	if !strings.HasPrefix(s, "# Lessons for architect") {
		t.Errorf("expected title preamble, got:\n%s", s)
	}
	if !strings.Contains(s, "## Summary") {
		t.Errorf("expected a summary section, got:\n%s", s)
	}
	if !strings.Contains(s, "## Lesson 1 · ") {
		t.Errorf("expected first lesson entry, got:\n%s", s)
	}
	if !strings.Contains(s, "**Corrected patterns:**") {
		t.Errorf("expected corrected patterns section, got:\n%s", s)
	}
	for _, p := range lesson.CorrectedPatterns {
		if !strings.Contains(s, "- "+p) {
			t.Errorf("expected pattern %q in file", p)
		}
	}
}

func TestLessonsRecorder_EmptyRoleError(t *testing.T) {
	r := NewLessonsRecorder(t.TempDir())
	err := r.Record("", Lesson{CorrectedPatterns: []string{"x"}})
	if err == nil {
		t.Fatal("expected error for empty role, got nil")
	}
	if !strings.Contains(err.Error(), "role must not be empty") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestLessonsRecorder_AppendOnlyPreservesPreamble(t *testing.T) {
	dir := filepath.Join(t.TempDir(), ".mill")
	r := NewLessonsRecorder(dir)

	role := "pm"
	sentinel := "# Pre-existing Leadership Notes\n\nSome hand-authored context."

	path := r.Path(role)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(sentinel+"\n"), 0o644); err != nil {
		t.Fatalf("writing pre-existing file: %v", err)
	}

	if err := r.Record(role, Lesson{CorrectedPatterns: []string{"delegate earlier"}}); err != nil {
		t.Fatalf("Record failed: %v", err)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading lessons file: %v", err)
	}
	if !strings.HasPrefix(string(content), "# Pre-existing Leadership Notes") {
		t.Errorf("pre-existing content was not preserved at start of file:\n%s", string(content))
	}
}

// TestLessonsRecorder_CapEnforcedAndCompressesToSummary uses a small cap to
// exercise the compression path deterministically and fast.
func TestLessonsRecorder_CapEnforcedAndCompressesToSummary(t *testing.T) {
	dir := filepath.Join(t.TempDir(), ".mill")
	const capN = 3
	r := NewLessonsRecorder(dir, WithMaxLessons(capN))
	role := "sr-dev-be"

	for i := 0; i < 5; i++ {
		if err := r.Record(role, Lesson{
			CorrectedPatterns:  []string{"pattern"},
			GapsDetected:       []string{"gap"},
			AcceptanceCriteria: []string{"criterion"},
		}); err != nil {
			t.Fatalf("Record %d failed: %v", i+1, err)
		}
	}

	content, err := os.ReadFile(r.Path(role))
	if err != nil {
		t.Fatalf("reading lessons file: %v", err)
	}
	s := string(content)

	if got := countLessons(s); got != capN {
		t.Fatalf("expected %d active lesson entries, got %d:\n%s", capN, got, s)
	}

	if !strings.Contains(s, "## Summary") {
		t.Errorf("expected a summary section:\n%s", s)
	}
	// The two oldest entries (indices 1 and 2) were compressed into the summary.
	for _, idx := range []int{1, 2} {
		if !strings.Contains(s, fmtCompressFragment(idx)) {
			t.Errorf("expected summary to reference compressed lesson #%d:\n%s", idx, s)
		}
	}
	// The newest three entries must remain as active entries.
	for _, idx := range []int{3, 4, 5} {
		if !strings.Contains(s, fmt.Sprintf("## Lesson %d · ", idx)) {
			t.Errorf("expected active lesson #%d in entries:\n%s", idx, s)
		}
	}
}

// TestLessonsRecorder_DefaultCapIs50 verifies the documented 50-entry cap using
// the default recorder. Recording 51 lessons must leave exactly 50 active
// entries and compress the single oldest into the summary.
func TestLessonsRecorder_DefaultCapIs50(t *testing.T) {
	dir := filepath.Join(t.TempDir(), ".mill")
	r := NewLessonsRecorder(dir) // default 50
	role := "tech-lead"

	for i := 0; i < 51; i++ {
		if err := r.Record(role, Lesson{
			CorrectedPatterns:  []string{"pattern"},
			GapsDetected:       []string{"gap"},
			AcceptanceCriteria: []string{"criterion"},
		}); err != nil {
			t.Fatalf("Record %d failed: %v", i+1, err)
		}
	}

	content, err := os.ReadFile(r.Path(role))
	if err != nil {
		t.Fatalf("reading lessons file: %v", err)
	}
	s := string(content)

	if got := countLessons(s); got != DefaultMaxLessons {
		t.Fatalf("expected %d active lesson entries, got %d:\n%s", DefaultMaxLessons, got, s)
	}
	if !strings.Contains(s, fmtCompressFragment(1)) {
		t.Errorf("expected the default-cap test to compress lesson #1 into the summary:\n%s", s)
	}
	if !strings.Contains(s, "## Lesson 51 · ") {
		t.Errorf("expected the newest active lesson #51 present:\n%s", s)
	}
}

// TestLessonsRecorder_ActiveEntriesPreservedVerbatim verifies that surviving
// active entries are written back byte-for-byte after a compaction that ages
// out the oldest entry.
func TestLessonsRecorder_ActiveEntriesPreservedVerbatim(t *testing.T) {
	dir := filepath.Join(t.TempDir(), ".mill")
	r := NewLessonsRecorder(dir, WithMaxLessons(3))
	role := "qa-docs"

	by := func(i int) Lesson {
		return Lesson{
			CorrectedPatterns:  []string{fmt.Sprintf("pattern-%d", i)},
			GapsDetected:       []string{fmt.Sprintf("gap-%d", i)},
			AcceptanceCriteria: []string{fmt.Sprintf("criterion-%d", i)},
		}
	}
	// Populate to the cap with distinct, identifiable content.
	for i := 1; i <= 3; i++ {
		if err := r.Record(role, by(i)); err != nil {
			t.Fatalf("Record %d failed: %v", i, err)
		}
	}
	content, err := os.ReadFile(r.Path(role))
	if err != nil {
		t.Fatalf("reading: %v", err)
	}
	// Snapshot the verbatim bodies of the two oldest active entries (#2, #3).
	body2Before := extractEntryBody(string(content), 2)
	body3Before := extractEntryBody(string(content), 3)
	if body2Before == "" || body3Before == "" {
		t.Fatalf("could not snapshot active entry bodies:\n%s", string(content))
	}

	// Adding a fourth entry forces #1 out of the active window; #2 and #3 must
	// survive verbatim.
	if err := r.Record(role, by(4)); err != nil {
		t.Fatalf("Record 4 failed: %v", err)
	}
	content, err = os.ReadFile(r.Path(role))
	if err != nil {
		t.Fatalf("reading: %v", err)
	}
	s := string(content)

	if strings.Contains(s, "## Lesson 1 · ") {
		t.Errorf("lesson #1 should have been compressed, but header still present:\n%s", s)
	}
	if !strings.Contains(s, fmtCompressFragment(1)) {
		t.Errorf("expected #1 compressed into the summary:\n%s", s)
	}

	body2After := extractEntryBody(s, 2)
	body3After := extractEntryBody(s, 3)
	if body2After != body2Before {
		t.Errorf("active entry #2 body was not preserved verbatim\nbefore: %q\nafter:  %q", body2Before, body2After)
	}
	if body3After != body3Before {
		t.Errorf("active entry #3 body was not preserved verbatim\nbefore: %q\nafter:  %q", body3Before, body3After)
	}
	if !strings.Contains(s, "## Lesson 4 · ") {
		t.Errorf("newest active lesson #4 missing:\n%s", s)
	}
}

// TestLessonsRecorder_ConcurrentWrites verifies the recorder is safe under
// concurrent Record calls for the same role.
func TestLessonsRecorder_ConcurrentWrites(t *testing.T) {
	dir := filepath.Join(t.TempDir(), ".mill")
	r := NewLessonsRecorder(dir, WithMaxLessons(50))
	role := "reviewer"

	const n = 20
	done := make(chan error, n)
	for i := 0; i < n; i++ {
		go func(i int) {
			done <- r.Record(role, Lesson{
				CorrectedPatterns:  []string{"pattern"},
				GapsDetected:       []string{"gap"},
				AcceptanceCriteria: []string{"criterion"},
			})
		}(i)
	}
	for i := 0; i < n; i++ {
		if err := <-done; err != nil {
			t.Fatalf("goroutine %d: %v", i, err)
		}
	}

	content, err := os.ReadFile(r.Path(role))
	if err != nil {
		t.Fatalf("reading lessons file: %v", err)
	}
	if got := countLessons(string(content)); got != n {
		t.Errorf("expected %d active lessons (under cap), got %d", n, got)
	}
}

// TestLessonsRecorder_NeverExceedsCapEvenUnderCompression verifies the cap is a
// hard ceiling: many more records than the cap still leave exactly capN active
// entries, all folded older ones being summarized.
func TestLessonsRecorder_NeverExceedsCapEvenUnderCompression(t *testing.T) {
	dir := filepath.Join(t.TempDir(), ".mill")
	const capN = 5
	r := NewLessonsRecorder(dir, WithMaxLessons(capN))
	role := "architect"

	for i := 0; i < 100; i++ {
		if err := r.Record(role, Lesson{
			CorrectedPatterns:  []string{"p"},
			GapsDetected:       []string{"g"},
			AcceptanceCriteria: []string{"c"},
		}); err != nil {
			t.Fatalf("Record %d failed: %v", i+1, err)
		}
	}

	content, err := os.ReadFile(r.Path(role))
	if err != nil {
		t.Fatalf("reading lessons file: %v", err)
	}
	s := string(content)

	if got := countLessons(s); got != capN {
		t.Fatalf("cap not enforced: expected %d active entries, got %d:\n%s", capN, got, s)
	}
	if !strings.Contains(s, "## Summary") {
		t.Errorf("expected summary block:\n%s", s)
	}
	// 100 - capN oldest were compressed.
	if !strings.Contains(s, fmtCompressFragment(1)) {
		t.Errorf("expected #1 compressed in summary:\n%s", s)
	}
	if !strings.Contains(s, fmtCompressFragment(95)) {
		t.Errorf("expected #95 compressed in summary:\n%s", s)
	}
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// lessonHeadingRe counts active lesson entries (## Lesson headers only). Summary
// lines use "- #N ·" and must not be counted.
var lessonHeadingRe = regexp.MustCompile(`(?m)^## Lesson \d+ · `)

func countLessons(s string) int {
	return len(lessonHeadingRe.FindAllString(s, -1))
}

func fmtCompressFragment(idx int) string {
	return fmt.Sprintf("- #%d · ", idx)
}

// entryBodyRe matches a single `## Lesson <idx> · <ts>` header line.
func entryBodyRe(idx int) *regexp.Regexp {
	return regexp.MustCompile(`(?m)^## Lesson ` + regexp.QuoteMeta(fmt.Sprintf("%d", idx)) + ` · .+$\n`)
}

// extractEntryBody returns the verbatim body (content after the header line) of
// the active lesson with the given index, up to the next lesson heading.
func extractEntryBody(s string, idx int) string {
	re := entryBodyRe(idx)
	loc := re.FindStringIndex(s)
	if loc == nil {
		return ""
	}
	rest := s[loc[1]:]
	next := regexp.MustCompile(`(?m)^## Lesson `).FindStringIndex(rest)
	if next == nil {
		return strings.TrimSpace(rest)
	}
	return strings.TrimSpace(rest[:next[0]])
}
