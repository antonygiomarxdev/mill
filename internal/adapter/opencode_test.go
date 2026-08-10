package adapter

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestOpenCodeCapabilities(t *testing.T) {
	a := &OpenCodeAdapter{}
	caps := a.Capabilities()

	if len(caps.Models) == 0 {
		t.Error("expected non-empty models list")
	}

	expected := []string{
		"opencode/claude-sonnet-5",
		"opencode/claude-sonnet-4-6",
		"opencode/deepseek-v4-pro",
		"opencode/deepseek-v4-flash",
		"opencode/gpt-5",
	}
	for _, want := range expected {
		found := false
		for _, got := range caps.Models {
			if got == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected model %q in capabilities", want)
		}
	}
}

func TestBuildOpenCodeArgs(t *testing.T) {
	opts := DispatchOpts{
		Prompt:   "fix the bug",
		Model:    "opencode/gpt-5",
		MaxTurns: 50,
	}

	args := buildOpenCodeArgs(opts)

	expected := []string{
		"run", "--format", "json", "--auto",
		"-m", "opencode/gpt-5",
		"fix the bug",
	}

	if !reflect.DeepEqual(args, expected) {
		t.Errorf("expected %v, got %v", expected, args)
	}
}

func TestOpenCodeDispatchSpawnsProcess(t *testing.T) {
	dir := t.TempDir()

	fakeBin := filepath.Join(dir, "opencode")
	script := `#!/bin/sh
echo '{"type":"step_start","timestamp":1786330761373,"sessionID":"ses_016640327ffeR1oUUt43JSl66M","part":{"type":"step-start"}}'
echo '{"type":"text","timestamp":1786330761374,"sessionID":"ses_016640327ffeR1oUUt43JSl66M","part":{"type":"text","text":"APPROVED - task complete"}}'
echo '{"type":"step_finish","timestamp":1786330761374,"sessionID":"ses_016640327ffeR1oUUt43JSl66M","part":{"type":"step-finish","reason":"stop"}}'
`
	if err := os.WriteFile(fakeBin, []byte(script), 0o755); err != nil {
		t.Fatalf("failed to write fake binary: %v", err)
	}

	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	a := &OpenCodeAdapter{}

	opts := DispatchOpts{
		Worktree: t.TempDir(),
		Prompt:   "fix issue 31",
		Model:    "opencode/claude-sonnet-5",
		MaxTurns: 10,
	}

	s, err := a.Dispatch(opts)
	if err != nil {
		t.Fatalf("Dispatch failed: %v", err)
	}

	if s.Status() != "running" {
		t.Errorf("expected status %q after dispatch, got %q", "running", s.Status())
	}

	result, err := s.Wait()
	if err != nil {
		t.Fatalf("Wait failed: %v", err)
	}

	if result.ExitCode != 0 {
		t.Errorf("expected exit code 0, got %d", result.ExitCode)
	}

	if !strings.Contains(result.Output, "APPROVED") {
		t.Errorf("expected output to contain APPROVED, got: %q", result.Output)
	}
}

func TestOpenCodeDispatchParseJSONOutput(t *testing.T) {
	dir := t.TempDir()

	fakeBin := filepath.Join(dir, "opencode")
	script := `#!/bin/sh
echo '{"type":"step_start","sessionID":"ses_test","part":{"type":"step-start"}}'
echo '{"type":"tool","sessionID":"ses_test","part":{"type":"tool","tool":{"name":"shell","input":{"command":"git commit -m fix1"}}}}'
echo '{"type":"text","sessionID":"ses_test","part":{"type":"text","text":"REJECTED"}}'
echo '{"type":"step_finish","sessionID":"ses_test","part":{"type":"step-finish","reason":"stop"}}'
`
	if err := os.WriteFile(fakeBin, []byte(script), 0o755); err != nil {
		t.Fatalf("failed to write fake binary: %v", err)
	}

	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	a := &OpenCodeAdapter{}

	opts := DispatchOpts{
		Worktree: t.TempDir(),
		Prompt:   "do work",
		Model:    "opencode/gpt-5",
		MaxTurns: 5,
	}

	s, err := a.Dispatch(opts)
	if err != nil {
		t.Fatalf("Dispatch failed: %v", err)
	}

	result, err := s.Wait()
	if err != nil {
		t.Fatalf("Wait failed: %v", err)
	}

	if result.Commits != 1 {
		t.Errorf("expected 1 commit, got %d", result.Commits)
	}

	if !strings.Contains(result.Output, "REJECTED") {
		t.Errorf("expected output to contain REJECTED, got: %q", result.Output)
	}
}

func TestOpenCodeDispatchExitCodeOnError(t *testing.T) {
	dir := t.TempDir()

	fakeBin := filepath.Join(dir, "opencode")
	script := `#!/bin/sh
echo '{"type":"error","sessionID":"ses_err","error":{"name":"APIError","data":{"message":"auth failure"}}}'
exit 3
`
	if err := os.WriteFile(fakeBin, []byte(script), 0o755); err != nil {
		t.Fatalf("failed to write fake binary: %v", err)
	}

	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	a := &OpenCodeAdapter{}

	opts := DispatchOpts{
		Worktree: t.TempDir(),
		Prompt:   "test",
		Model:    "opencode/gpt-5",
		MaxTurns: 5,
	}

	s, err := a.Dispatch(opts)
	if err != nil {
		t.Fatalf("Dispatch failed: %v", err)
	}

	result, err := s.Wait()
	if err != nil {
		t.Fatalf("Wait should not return error for non-zero exit: %v", err)
	}

	if result.ExitCode != 3 {
		t.Errorf("expected exit code 3, got %d", result.ExitCode)
	}

	if !strings.Contains(result.Output, "auth failure") {
		t.Errorf("expected output to contain auth failure, got: %q", result.Output)
	}
}

func TestOpenCodeResumeSpawnsProcess(t *testing.T) {
	dir := t.TempDir()

	fakeBin := filepath.Join(dir, "opencode")
	script := `#!/bin/sh
echo '{"type":"text","sessionID":"resumed-sess","part":{"type":"text","text":"NEEDS CHANGES"}}'
echo '{"type":"step_finish","sessionID":"resumed-sess","part":{"type":"step-finish","reason":"stop"}}'
`
	if err := os.WriteFile(fakeBin, []byte(script), 0o755); err != nil {
		t.Fatalf("failed to write fake binary: %v", err)
	}

	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	a := &OpenCodeAdapter{}

	s, err := a.Resume("session-123")
	if err != nil {
		t.Fatalf("Resume failed: %v", err)
	}

	if s.ID() != "session-123" {
		t.Errorf("expected session ID %q, got %q", "session-123", s.ID())
	}

	result, err := s.Wait()
	if err != nil {
		t.Fatalf("Wait failed: %v", err)
	}

	if !strings.Contains(result.Output, "NEEDS CHANGES") {
		t.Errorf("expected output to contain NEEDS CHANGES, got: %q", result.Output)
	}
}

func TestParseOpenCodeOutputExtractsText(t *testing.T) {
	output := `{"type":"text","sessionID":"ses-123","part":{"type":"text","text":"APPROVED - done"}}`

	finalText, sessionID := parseOpenCodeOutput(output)

	if sessionID != "ses-123" {
		t.Errorf("expected session ID %q, got %q", "ses-123", sessionID)
	}
	if !strings.Contains(finalText, "APPROVED") {
		t.Errorf("expected finalText to contain APPROVED, got: %q", finalText)
	}
}

func TestParseOpenCodeOutputMultipleLines(t *testing.T) {
	output := `{"type":"step_start","sessionID":"ses-456","part":{"type":"step-start"}}` + "\n" +
		`{"type":"text","sessionID":"ses-456","part":{"type":"text","text":"Part 1 "}}` + "\n" +
		`{"type":"text","sessionID":"ses-456","part":{"type":"text","text":"Part 2"}}` + "\n"

	finalText, sessionID := parseOpenCodeOutput(output)

	if sessionID != "ses-456" {
		t.Errorf("expected session ID %q, got %q", "ses-456", sessionID)
	}
	if !strings.Contains(finalText, "Part 1") {
		t.Errorf("expected finalText to contain Part 1, got: %q", finalText)
	}
	if !strings.Contains(finalText, "Part 2") {
		t.Errorf("expected finalText to contain Part 2, got: %q", finalText)
	}
}

func TestParseOpenCodeOutputError(t *testing.T) {
	output := `{"type":"error","sessionID":"ses-err","error":{"name":"APIError","data":{"message":"rate limit exceeded"}}}`

	finalText, sessionID := parseOpenCodeOutput(output)

	if sessionID != "ses-err" {
		t.Errorf("expected session ID %q, got %q", "ses-err", sessionID)
	}
	if !strings.Contains(finalText, "rate limit exceeded") {
		t.Errorf("expected finalText to contain error message, got: %q", finalText)
	}
}

func TestParseOpenCodeOutputInvalidJSON(t *testing.T) {
	finalText, sessionID := parseOpenCodeOutput("not json at all")

	if finalText != "" {
		t.Errorf("expected empty finalText for invalid JSON, got %q", finalText)
	}
	if sessionID != "" {
		t.Errorf("expected empty sessionID for invalid JSON, got %q", sessionID)
	}
}
