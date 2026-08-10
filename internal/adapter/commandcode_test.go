package adapter

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestCommandCodeCapabilities(t *testing.T) {
	a := &CommandCodeAdapter{}
	caps := a.Capabilities()

	if len(caps.Models) == 0 {
		t.Error("expected non-empty models list")
	}

	expected := []string{"claude-sonnet-5", "claude-sonnet-4-6", "deepseek-v4-pro", "deepseek-v4-flash", "gpt-5"}
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

func TestBuildArgs(t *testing.T) {
	opts := DispatchOpts{
		Prompt:   "fix the bug",
		Model:    "gpt-5",
		MaxTurns: 50,
	}

	args := buildArgs(opts)

	expected := []string{
		"-p", "fix the bug",
		"-m", "gpt-5",
		"--output-format", "json",
		"--max-turns", "50",
	}

	if !reflect.DeepEqual(args, expected) {
		t.Errorf("expected %v, got %v", expected, args)
	}
}

func TestBuildArgsOmitsMaxTurnsWhenZero(t *testing.T) {
	opts := DispatchOpts{
		Prompt:   "hello",
		Model:    "laguna-free",
		MaxTurns: 0,
	}

	args := buildArgs(opts)

	for i, a := range args {
		if a == "--max-turns" {
			t.Errorf("did not expect --max-turns in args at index %d", i)
		}
	}
}

func TestCommandCodeDispatchSpawnsProcess(t *testing.T) {
	dir := t.TempDir()

	// Create a fake "cmd" binary that mimics `cmd -p --output-format json`
	fakeBin := filepath.Join(dir, "cmd")
	script := `#!/bin/sh
echo '{"type":"result","subtype":"success","sessionId":"real-session-1","stopReason":"end_turn","finalText":"APPROVED - task complete","durationMs":100}'
`
	if err := os.WriteFile(fakeBin, []byte(script), 0o755); err != nil {
		t.Fatalf("failed to write fake binary: %v", err)
	}

	// Put fake binary first in PATH so exec.Command finds it
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	a := &CommandCodeAdapter{}

	opts := DispatchOpts{
		Worktree: t.TempDir(),
		Prompt:   "fix issue 390",
		Model:    "laguna-free",
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

func TestCommandCodeDispatchParseJSONOutput(t *testing.T) {
	dir := t.TempDir()

	fakeBin := filepath.Join(dir, "cmd")
	script := `#!/bin/sh
echo '{"type":"event","event":{"type":"tool_running","toolName":"shell","description":"git commit -m test"}}'
echo '{"type":"result","subtype":"success","sessionId":"sess-abc","stopReason":"end_turn","finalText":"APPROVED"}'
`
	if err := os.WriteFile(fakeBin, []byte(script), 0o755); err != nil {
		t.Fatalf("failed to write fake binary: %v", err)
	}

	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	a := &CommandCodeAdapter{}

	opts := DispatchOpts{
		Worktree: t.TempDir(),
		Prompt:   "do work",
		Model:    "gpt-5",
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

	if !strings.Contains(result.Output, "APPROVED") {
		t.Errorf("expected output to contain APPROVED, got: %q", result.Output)
	}
}

func TestCommandCodeDispatchExitCodeOnError(t *testing.T) {
	dir := t.TempDir()

	fakeBin := filepath.Join(dir, "cmd")
	script := `#!/bin/sh
echo '{"type":"result","subtype":"error","finalText":"auth failure"}'
exit 3
`
	if err := os.WriteFile(fakeBin, []byte(script), 0o755); err != nil {
		t.Fatalf("failed to write fake binary: %v", err)
	}

	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	a := &CommandCodeAdapter{}

	opts := DispatchOpts{
		Worktree: t.TempDir(),
		Prompt:   "test",
		Model:    "gpt-5",
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
}

func TestCommandCodeResumeSpawnsProcess(t *testing.T) {
	dir := t.TempDir()

	fakeBin := filepath.Join(dir, "cmd")
	script := `#!/bin/sh
echo '{"type":"result","subtype":"success","sessionId":"resumed-sess","stopReason":"end_turn","finalText":"NEEDS CHANGES"}'
`
	if err := os.WriteFile(fakeBin, []byte(script), 0o755); err != nil {
		t.Fatalf("failed to write fake binary: %v", err)
	}

	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	a := &CommandCodeAdapter{}

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

func TestParseJSONOutputExtractsFinalText(t *testing.T) {
	output := `{"type":"result","subtype":"success","sessionId":"sess-123","stopReason":"end_turn","finalText":"APPROVED - done","durationMs":100}`

	finalText, sessionID := parseJSONOutput(output)

	if sessionID != "sess-123" {
		t.Errorf("expected session ID %q, got %q", "sess-123", sessionID)
	}
	if !strings.Contains(finalText, "APPROVED") {
		t.Errorf("expected finalText to contain APPROVED, got: %q", finalText)
	}
}

func TestParseJSONOutputMultipleLines(t *testing.T) {
	output := `{"type":"event","event":{"type":"tool_running"}}` + "\n" +
		`{"type":"result","subtype":"success","sessionId":"sess-456","finalText":"REJECTED"}` + "\n"

	finalText, sessionID := parseJSONOutput(output)

	if sessionID != "sess-456" {
		t.Errorf("expected session ID %q, got %q", "sess-456", sessionID)
	}
	if !strings.Contains(finalText, "REJECTED") {
		t.Errorf("expected finalText to contain REJECTED, got: %q", finalText)
	}
}

func TestParseJSONOutputInvalidJSON(t *testing.T) {
	finalText, sessionID := parseJSONOutput("not json at all")

	if finalText != "" {
		t.Errorf("expected empty finalText for invalid JSON, got %q", finalText)
	}
	if sessionID != "" {
		t.Errorf("expected empty sessionID for invalid JSON, got %q", sessionID)
	}
}

func TestCountCommits(t *testing.T) {
	output := `some text
{"type":"event","event":{"type":"tool_running","toolName":"shell","description":"git commit -m fix1"}}
{"type":"event","event":{"type":"tool_running","toolName":"shell","description":"git commit -m fix2"}}
more text`

	if got := countCommits(output); got != 2 {
		t.Errorf("expected 2 commits, got %d", got)
	}
}

func TestCountCommitsZero(t *testing.T) {
	output := "no commits here"

	if got := countCommits(output); got != 0 {
		t.Errorf("expected 0 commits, got %d", got)
	}
}
