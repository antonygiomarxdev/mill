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
