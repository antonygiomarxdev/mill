package ledger

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestAppendCreatesFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".mill", "ledger", "390.jsonl")

	entry := Entry{
		Timestamp: "2026-08-09T10:00:00Z",
		Issue:     390,
		Event:     "dispatch",
		Status:    "pending",
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
		{Timestamp: "2026-08-09T10:00:00Z", Issue: 390, Event: "dispatch", Status: "pending"},
		{Timestamp: "2026-08-09T10:01:00Z", Issue: 390, Event: "review", Status: "done", Verdict: "approved"},
	}

	for _, e := range entries {
		if err := Append(path, e); err != nil {
			t.Fatalf("Append failed: %v", err)
		}
	}

	// Read back and verify
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
	if got[1].Event != "review" {
		t.Errorf("expected second event %q, got %q", "review", got[1].Event)
	}
	if got[1].Verdict != "approved" {
		t.Errorf("expected verdict %q, got %q", "approved", got[1].Verdict)
	}
}

func TestAppendEachEntryOnOwnLine(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ledger", "1.jsonl")

	for i := 0; i < 3; i++ {
		if err := Append(path, Entry{Issue: 1, Event: "tick"}); err != nil {
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
		Timestamp: "2026-08-09T10:00:00Z",
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

	expected := []string{"timestamp", "issue", "event", "status"}
	for _, f := range expected {
		if _, ok := fields[f]; !ok {
			t.Errorf("expected field %q in entry JSON", f)
		}
	}

	// verdict should be omitempty — present when set
	if _, ok := fields["verdict"]; !ok {
		t.Errorf("expected verdict field when set")
	}
}

func TestAppendVerdictOmitEmpty(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ledger", "1.jsonl")

	entry := Entry{
		Timestamp: "2026-08-09T10:00:00Z",
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
