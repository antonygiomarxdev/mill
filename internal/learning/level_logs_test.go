package learning

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/antonygiomarxdev/mill/internal/domain"
)

func TestLevelLogger_WritesJSONL(t *testing.T) {
	millDir := filepath.Join(t.TempDir(), ".mill")
	logger := NewLevelLogger(millDir)

	entry := LevelLog{
		Depth:          2,
		Role:           "architect",
		Model:          "deepseek-v4-pro",
		SessionID:      "sess-abc",
		Classification: domain.ClassificationOK,
		Duration:       3 * time.Second,
		Verdict:        domain.VerdictApproved,
	}

	if err := logger.Log(entry); err != nil {
		t.Fatalf("Log failed: %v", err)
	}

	data, err := os.ReadFile(logger.Path())
	if err != nil {
		t.Fatalf("reading log file: %v", err)
	}

	var got LevelLog
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshalling log line: %v\nraw: %s", err, string(data))
	}

	cases := []struct {
		name string
		got  any
		want any
	}{
		{"Depth", got.Depth, entry.Depth},
		{"Role", got.Role, entry.Role},
		{"Model", got.Model, entry.Model},
		{"SessionID", got.SessionID, entry.SessionID},
		{"Classification", got.Classification, entry.Classification},
		{"Duration", got.Duration, entry.Duration},
		{"Verdict", got.Verdict, entry.Verdict},
	}
	for _, tc := range cases {
		if tc.got != tc.want {
			t.Errorf("field %s: got %v, want %v", tc.name, tc.got, tc.want)
		}
	}

	// Verify the file is valid JSONL (single line).
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	if len(lines) != 1 {
		t.Fatalf("expected 1 JSONL line, got %d", len(lines))
	}
}

func TestLevelLogger_AppendOnlyTwoAppendsTwoLines(t *testing.T) {
	millDir := filepath.Join(t.TempDir(), ".mill")
	logger := NewLevelLogger(millDir)

	entries := []LevelLog{
		{
			Depth: 0, Role: "pm", Model: "deepseek-v4-pro", SessionID: "s1",
			Classification: domain.ClassificationOK, Duration: 1 * time.Second,
			Verdict: domain.VerdictApproved,
		},
		{
			Depth: 1, Role: "arch", Model: "laguna-s-2.1-free", SessionID: "s2",
			Classification: domain.ClassificationChangesRequested, Duration: 2 * time.Second,
			Verdict: domain.VerdictChanges,
		},
	}

	for _, e := range entries {
		if err := logger.Log(e); err != nil {
			t.Fatalf("Log failed: %v", err)
		}
	}

	f, err := os.Open(logger.Path())
	if err != nil {
		t.Fatalf("opening log file: %v", err)
	}
	defer f.Close()

	var got []LevelLog
	dec := json.NewDecoder(f)
	for dec.More() {
		var e LevelLog
		if err := dec.Decode(&e); err != nil {
			t.Fatalf("decoding log line: %v", err)
		}
		got = append(got, e)
	}

	if len(got) != 2 {
		t.Fatalf("expected 2 log entries (two appends), got %d", len(got))
	}

	for i, e := range entries {
		if got[i].Role != e.Role {
			t.Errorf("entry %d: role got %q, want %q", i, got[i].Role, e.Role)
		}
		if got[i].Depth != e.Depth {
			t.Errorf("entry %d: depth got %d, want %d", i, got[i].Depth, e.Depth)
		}
		if got[i].Classification != e.Classification {
			t.Errorf("entry %d: classification got %q, want %q", i, got[i].Classification, e.Classification)
		}
		if got[i].Verdict != e.Verdict {
			t.Errorf("entry %d: verdict got %q, want %q", i, got[i].Verdict, e.Verdict)
		}
	}
}

func TestLevelLogger_CreatesDirectories(t *testing.T) {
	millDir := filepath.Join(t.TempDir(), ".mill")
	logger := NewLevelLogger(millDir)

	if err := logger.Log(LevelLog{Depth: 0, Role: "qa", SessionID: "s1"}); err != nil {
		t.Fatalf("Log failed: %v", err)
	}

	info, err := os.Stat(logger.Path())
	if err != nil {
		t.Fatalf("expected log file to exist: %v", err)
	}
	if info.Size() == 0 {
		t.Error("expected non-empty log file")
	}
}

func TestLevelLogger_ConcurrentWrites(t *testing.T) {
	millDir := filepath.Join(t.TempDir(), ".mill")
	logger := NewLevelLogger(millDir)

	const n = 20
	done := make(chan error, n)
	for i := 0; i < n; i++ {
		go func(i int) {
			done <- logger.Log(LevelLog{
				Depth:     i,
				Role:      "worker",
				SessionID: "sess",
				Duration:  time.Duration(i) * time.Millisecond,
			})
		}(i)
	}

	for i := 0; i < n; i++ {
		if err := <-done; err != nil {
			t.Fatalf("goroutine %d: %v", i, err)
		}
	}

	f, err := os.Open(logger.Path())
	if err != nil {
		t.Fatalf("opening log: %v", err)
	}
	defer f.Close()

	count := 0
	dec := json.NewDecoder(f)
	for dec.More() {
		var e LevelLog
		if err := dec.Decode(&e); err != nil {
			t.Fatalf("decode error: %v", err)
		}
		count++
	}
	if count != n {
		t.Errorf("expected %d lines, got %d", n, count)
	}
}
