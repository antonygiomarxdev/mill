package ledger

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestAppendCreatesFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".mill", "ledger", "390.jsonl")

	entry := Entry{
		Timestamp: time.Date(2026, 8, 9, 10, 0, 0, 0, time.UTC),
		Issue:     390,
		Event:     "dispatch",
		Status:    "running",
	}

	if err := Append(path, entry); err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatal("expected ledger file to be created")
	}
}

func TestAppendMultipleEntries(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".mill", "ledger", "390.jsonl")

	entries := []Entry{
		{Timestamp: time.Date(2026, 8, 9, 10, 0, 0, 0, time.UTC), Issue: 390, Event: "dispatch", Status: "running"},
		{Timestamp: time.Date(2026, 8, 9, 10, 1, 0, 0, time.UTC), Issue: 390, Event: "complete", Status: "done", Verdict: "approved"},
	}

	for _, e := range entries {
		if err := Append(path, e); err != nil {
			t.Fatalf("Append failed: %v", err)
		}
	}

	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open failed: %v", err)
	}
	defer f.Close()

	var got []Entry
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var e Entry
		if err := json.Unmarshal(scanner.Bytes(), &e); err != nil {
			t.Fatalf("unmarshal failed: %v", err)
		}
		got = append(got, e)
	}

	if len(got) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(got))
	}

	if got[0].Event != "dispatch" {
		t.Errorf("expected first event %q, got %q", "dispatch", got[0].Event)
	}
	if got[1].Event != "complete" {
		t.Errorf("expected second event %q, got %q", "complete", got[1].Event)
	}
	if got[1].Verdict != "approved" {
		t.Errorf("expected verdict %q, got %q", "approved", got[1].Verdict)
	}
}

func TestAppendEachEntryOnOwnLine(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ledger", "1.jsonl")

	ts := time.Date(2026, 8, 9, 10, 0, 0, 0, time.UTC)
	for i := 0; i < 3; i++ {
		if err := Append(path, Entry{
			Timestamp: ts,
			Issue:     1,
			Event:     "tick",
		}); err != nil {
			t.Fatalf("Append failed: %v", err)
		}
	}

	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open failed: %v", err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	lines := 0
	for scanner.Scan() {
		lines++
	}

	if lines != 3 {
		t.Errorf("expected 3 lines, got %d", lines)
	}
}

func TestAppendEntryStructure(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ledger", "1.jsonl")

	entry := Entry{
		Timestamp: time.Date(2026, 8, 9, 10, 0, 0, 0, time.UTC),
		Issue:     42,
		Event:     "dispatch",
		Status:    "running",
		Verdict:   "changes",
	}

	if err := Append(path, entry); err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}

	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	for _, f := range []string{"timestamp", "issue", "event", "status"} {
		if _, ok := fields[f]; !ok {
			t.Errorf("expected field %q in entry JSON", f)
		}
	}

	if _, ok := fields["verdict"]; !ok {
		t.Errorf("expected verdict field when set")
	}
}

func TestAppendVerdictOmitEmpty(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ledger", "1.jsonl")

	entry := Entry{
		Timestamp: time.Date(2026, 8, 9, 10, 0, 0, 0, time.UTC),
		Issue:     1,
		Event:     "dispatch",
		Status:    "pending",
	}

	if err := Append(path, entry); err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}

	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if _, ok := fields["verdict"]; ok {
		t.Errorf("verdict should be omitted when empty")
	}
}

func TestEntryTimestampIsTime(t *testing.T) {
	ts := time.Date(2026, 8, 10, 12, 30, 0, 0, time.UTC)
	e := Entry{
		Timestamp: ts,
		Issue:     1,
		Event:     "dispatch",
		Status:    "running",
	}

	if !e.Timestamp.Equal(ts) {
		t.Errorf("expected timestamp %v, got %v", ts, e.Timestamp)
	}

	// Verify JSON round-trips as RFC3339
	data, err := json.Marshal(e)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	tsBytes, ok := raw["timestamp"]
	if !ok {
		t.Fatal("expected timestamp field in JSON")
	}
	var tsStr string
	if err := json.Unmarshal(tsBytes, &tsStr); err != nil {
		t.Fatalf("expected string timestamp: %v", err)
	}
	if _, err := time.Parse(time.RFC3339, tsStr); err != nil {
		t.Errorf("timestamp %q is not valid RFC3339: %v", tsStr, err)
	}

	// Round-trip: unmarshal back
	var e2 Entry
	if err := json.Unmarshal(data, &e2); err != nil {
		t.Fatalf("round-trip unmarshal failed: %v", err)
	}
	if !e2.Timestamp.Equal(ts) {
		t.Errorf("round-trip timestamp mismatch: %v vs %v", e2.Timestamp, ts)
	}
}

func TestAppendFailsWhenParentPathIsFile(t *testing.T) {
	dir := t.TempDir()
	// Place a regular file where a directory needs to be created.
	blocker := filepath.Join(dir, "blocker")
	if err := os.WriteFile(blocker, []byte("block"), 0o644); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	path := filepath.Join(dir, "blocker", "ledger", "1.jsonl")

	err := Append(path, Entry{
		Timestamp: time.Date(2026, 8, 10, 12, 0, 0, 0, time.UTC),
		Issue:     1,
		Event:     "dispatch",
		Status:    "running",
	})
	if err == nil {
		t.Fatal("expected MkdirAll error when parent path is a file, got nil")
	}
}

func TestAppendFailsWhenPathIsDirectory(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ledger", "1.jsonl")
	// Create a directory at the exact path where a file is expected.
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}

	err := Append(path, Entry{
		Timestamp: time.Date(2026, 8, 10, 12, 0, 0, 0, time.UTC),
		Issue:     1,
		Event:     "dispatch",
		Status:    "running",
	})
	if err == nil {
		t.Fatal("expected OpenFile error when path is a directory, got nil")
	}
}

func TestAppendClassificationField(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ledger", "1.jsonl")

	entry := Entry{
		Timestamp:      time.Date(2026, 8, 10, 12, 0, 0, 0, time.UTC),
		Issue:          7,
		Event:          "classify",
		Status:         "done",
		Classification: "bug",
	}

	if err := Append(path, entry); err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}

	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if _, ok := fields["classification"]; !ok {
		t.Fatal("expected classification field when set")
	}
	var cls string
	if err := json.Unmarshal(fields["classification"], &cls); err != nil {
		t.Fatalf("unmarshal classification failed: %v", err)
	}
	if cls != "bug" {
		t.Errorf("expected classification %q, got %q", "bug", cls)
	}
}

func TestAppendClassificationOmitEmpty(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ledger", "1.jsonl")

	entry := Entry{
		Timestamp: time.Date(2026, 8, 10, 12, 0, 0, 0, time.UTC),
		Issue:     7,
		Event:     "dispatch",
		Status:    "running",
	}

	if err := Append(path, entry); err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}

	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if _, ok := fields["classification"]; ok {
		t.Error("classification should be omitted when empty")
	}
}

func TestAppendAllFieldsRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ledger", "1.jsonl")

	entry := Entry{
		Timestamp:      time.Date(2026, 8, 10, 9, 15, 0, 0, time.UTC),
		Issue:          404,
		Event:          "resolve",
		Status:         "closed",
		Verdict:        "rejected",
		Classification: "security",
	}

	if err := Append(path, entry); err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}

	var got Entry
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if !got.Timestamp.Equal(entry.Timestamp) {
		t.Errorf("timestamp mismatch: expected %v, got %v", entry.Timestamp, got.Timestamp)
	}
	if got.Issue != entry.Issue {
		t.Errorf("issue: expected %d, got %d", entry.Issue, got.Issue)
	}
	if got.Event != entry.Event {
		t.Errorf("event: expected %q, got %q", entry.Event, got.Event)
	}
	if got.Status != entry.Status {
		t.Errorf("status: expected %q, got %q", entry.Status, got.Status)
	}
	if got.Verdict != entry.Verdict {
		t.Errorf("verdict: expected %q, got %q", entry.Verdict, got.Verdict)
	}
	if got.Classification != entry.Classification {
		t.Errorf("classification: expected %q, got %q", entry.Classification, got.Classification)
	}
}

func TestAppendEmptyStringFields(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ledger", "1.jsonl")

	entry := Entry{
		Timestamp: time.Date(2026, 8, 10, 0, 0, 0, 0, time.UTC),
		Issue:     0,
		Event:     "",
		Status:    "",
	}

	if err := Append(path, entry); err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}

	var got Entry
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if got.Issue != 0 {
		t.Errorf("expected issue 0, got %d", got.Issue)
	}
	if got.Event != "" {
		t.Errorf("expected empty event, got %q", got.Event)
	}
	if got.Status != "" {
		t.Errorf("expected empty status, got %q", got.Status)
	}
}

func TestAppendZeroValueTimestampRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ledger", "1.jsonl")

	entry := Entry{
		Timestamp: time.Time{}, // zero value
		Issue:     1,
		Event:     "dispatch",
		Status:    "running",
	}

	if err := Append(path, entry); err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}

	var got Entry
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if !got.Timestamp.Equal(time.Time{}) {
		t.Errorf("expected zero timestamp, got %v", got.Timestamp)
	}
}

func TestAppendPreservesExistingContent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ledger", "1.jsonl")

	first := Entry{
		Timestamp: time.Date(2026, 8, 9, 10, 0, 0, 0, time.UTC),
		Issue:     1,
		Event:     "start",
		Status:    "running",
	}
	second := Entry{
		Timestamp: time.Date(2026, 8, 9, 11, 0, 0, 0, time.UTC),
		Issue:     1,
		Event:     "end",
		Status:    "done",
		Verdict:   "approved",
	}

	if err := Append(path, first); err != nil {
		t.Fatalf("first Append failed: %v", err)
	}
	if err := Append(path, second); err != nil {
		t.Fatalf("second Append failed: %v", err)
	}

	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open failed: %v", err)
	}
	defer f.Close()

	var got []Entry
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var e Entry
		if err := json.Unmarshal(scanner.Bytes(), &e); err != nil {
			t.Fatalf("unmarshal failed: %v", err)
		}
		got = append(got, e)
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scanner error: %v", err)
	}

	if len(got) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(got))
	}
	if got[0].Event != "start" {
		t.Errorf("expected first event %q, got %q", "start", got[0].Event)
	}
	if got[1].Event != "end" {
		t.Errorf("expected second event %q, got %q", "end", got[1].Event)
	}
	if got[1].Verdict != "approved" {
		t.Errorf("expected verdict %q, got %q", "approved", got[1].Verdict)
	}
}

func TestAppendFilePermissions(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ledger", "1.jsonl")

	entry := Entry{
		Timestamp: time.Date(2026, 8, 10, 12, 0, 0, 0, time.UTC),
		Issue:     1,
		Event:     "dispatch",
		Status:    "running",
	}

	if err := Append(path, entry); err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat failed: %v", err)
	}

	mode := info.Mode().Perm()
	if mode != 0o644 {
		t.Errorf("expected file permissions 0o644, got %o", mode)
	}
}
