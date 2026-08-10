package compact

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestShouldCompactBelowThreshold(t *testing.T) {
	half := strings.Repeat("x", 128_000)
	should, est := ShouldCompact(half, "free")
	if should {
		t.Errorf("expected false at ~25%%, got true (est=%d)", est)
	}
}

func TestShouldCompactAtThreshold(t *testing.T) {
	atThreshold := strings.Repeat("x", 409_600)
	should, _ := ShouldCompact(atThreshold, "free")
	if !should {
		t.Error("expected true at exactly 80%")
	}
}

func TestShouldCompactAboveThreshold(t *testing.T) {
	above := strings.Repeat("x", 500_000)
	should, _ := ShouldCompact(above, "free")
	if !should {
		t.Error("expected true above threshold")
	}
}

func TestShouldCompactEmptyContext(t *testing.T) {
	should, est := ShouldCompact("", "free")
	if should {
		t.Error("expected false for empty context")
	}
	if est != 0 {
		t.Errorf("expected 0 tokens, got %d", est)
	}
}

func TestShouldCompactTierMapping(t *testing.T) {
	tests := []struct {
		tier   string
		window int
	}{
		{"free", 128_000},
		{"paid", 200_000},
		{"pro", 200_000},
		{"unknown", 128_000},
	}
	for _, tt := range tests {
		w := windowForTier(tt.tier)
		if int(w) != tt.window {
			t.Errorf("tier %q: expected %d, got %d", tt.tier, tt.window, w)
		}
	}
}

func TestCompactPreservesOriginalPrompt(t *testing.T) {
	ctx := "ROLE: sr-dev-be\n\nOriginal prompt.\n\nuser> Task 1\nTool: read\noutput\n\n"
	compacted, _ := Compact(ctx, "free", 55)
	if !strings.Contains(compacted, "ROLE: sr-dev-be") {
		t.Error("should preserve role in original prompt")
	}
}

func TestCompactPreservesLastThreeTurns(t *testing.T) {
	var b strings.Builder
	b.WriteString("Original.\n\n")
	for i := range 10 {
		b.WriteString("user> Task " + string(rune('0'+i)) + "\nTool: read\noutput\n\n")
	}
	compacted, _ := Compact(b.String(), "free", 55)
	if !strings.Contains(compacted, "Task 7") {
		t.Error("last 3rd turn preserved")
	}
	if !strings.Contains(compacted, "Task 9") {
		t.Error("last turn preserved")
	}
	if strings.Contains(compacted, "Task 0") {
		t.Error("first turn discarded")
	}
}

func TestCompactPreservesUnresolvedItems(t *testing.T) {
	ctx := "Original.\n\n" +
		"user> Task 1\nBLOCKED: cannot proceed\n\n" +
		"user> Task 2\nTool: read\noutput\n\n" +
		"user> Task 3\nTool: read\noutput\n\n" +
		"user> Task 4\nunresolved dep\n\n" +
		"user> Task 5\nTool: read\noutput\n\n"
	compacted, _ := Compact(ctx, "free", 55)
	if !strings.Contains(compacted, "BLOCKED") {
		t.Error("blocked turn preserved")
	}
	if !strings.Contains(compacted, "unresolved") {
		t.Error("unresolved turn preserved")
	}
}

func TestCompactProducesSummaryLine(t *testing.T) {
	var b strings.Builder
	b.WriteString("Original.\n\n")
	for i := range 10 {
		b.WriteString("user> Task " + string(rune('0'+i)) + "\nTool: read\nerror: fail\n\n")
	}
	compacted, _ := Compact(b.String(), "free", 55)
	if !strings.Contains(compacted, "[COMPACTED:") {
		t.Error("should contain summary line")
	}
}

func TestWriteLogCreatesFile(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(origDir)
	os.MkdirAll(".mill", 0o755)

	event := Event{PreTokens: 1000, PostTokens: 500, Saved: 500, Trigger: "auto", Issue: 55}
	if err := WriteLog(event); err != nil {
		t.Fatalf("WriteLog: %v", err)
	}
	data, _ := os.ReadFile(".mill/compact.log")
	var decoded Event
	json.Unmarshal(data, &decoded)
	if decoded.Saved != 500 {
		t.Errorf("saved=%d", decoded.Saved)
	}
}

func TestWriteLogAppendsMultiple(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(origDir)
	os.MkdirAll(".mill", 0o755)

	WriteLog(Event{PreTokens: 100, PostTokens: 50, Saved: 50, Trigger: "auto", Issue: 55})
	WriteLog(Event{PreTokens: 80, PostTokens: 40, Saved: 40, Trigger: "manual", Issue: 55})

	data, _ := os.ReadFile(".mill/compact.log")
	lines := 0
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		if line != "" {
			lines++
		}
	}
	if lines != 2 {
		t.Errorf("expected 2 lines, got %d", lines)
	}
}

func TestCompactNoTurns(t *testing.T) {
	ctx := "Just a prompt."
	compacted, event := Compact(ctx, "free", 55)
	if compacted != ctx {
		t.Error("no-turn context unchanged")
	}
	if event.Saved != 0 {
		t.Errorf("saved=0, got %d", event.Saved)
	}
}

func TestCompactPreservesRoleReferences(t *testing.T) {
	ctx := "Original.\n\n" +
		"user> Read ROLE.md\nROLE.md says delegate to sr-dev\n\n" +
		"user> Task 1\nTool: read\noutput\n\n" +
		"user> Task 2\noutput\n\n" +
		"user> Task 3\noutput\n\n"
	compacted, _ := Compact(ctx, "free", 55)
	if !strings.Contains(compacted, "ROLE.md") {
		t.Error("role ref preserved")
	}
}

func TestCompactDiscardsResolvedErrors(t *testing.T) {
	var b strings.Builder
	b.WriteString("Original.\n\n")
	for i := range 3 {
		b.WriteString("user> Task " + string(rune('0'+i)) + "\nTool: read\nerror: broke\n\n")
	}
	for i := range 3 {
		b.WriteString("user> Fix " + string(rune('0'+i)) + "\nTool: edit\nok\n\n")
	}
	b.WriteString("user> Final\nDone\n\n")
	ctx := b.String()
	compacted, _ := Compact(ctx, "free", 55)
	if strings.Contains(compacted, "Task 0") {
		t.Error("resolved error turns discarded")
	}
	if !strings.Contains(compacted, "Final") {
		t.Error("last turn preserved")
	}
}

func TestCompactSummaryCounts(t *testing.T) {
	var b strings.Builder
	b.WriteString("Original.\n\n")
	for i := range 5 {
		b.WriteString("user> Explore " + string(rune('0'+i)) + "\nTool: read\nerror: fail\n\n")
	}
	for i := range 5 {
		b.WriteString("user> Change " + string(rune('0'+i)) + "\nTool: edit\n\n")
	}
	ctx := b.String()
	_, event := Compact(ctx, "free", 55)
	if event.PreTokens <= 0 {
		t.Error("pre_tokens > 0")
	}
	if event.PostTokens <= 0 {
		t.Error("post_tokens > 0")
	}
	if event.Saved <= 0 {
		t.Error("saved > 0")
	}
}
