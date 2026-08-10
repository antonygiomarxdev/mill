package cli

import (
	"bytes"
	"os"
	"testing"

	"github.com/antonygiomarxdev/mill/internal/state"
)

func executeCommandWithMillDir(t *testing.T, args ...string) (string, error) {
	t.Helper()

	dir := t.TempDir()
	oldMillDir := millDir
	millDir = dir
	t.Cleanup(func() { millDir = oldMillDir })

	rootCmd.SetArgs(args)
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)

	err := rootCmd.Execute()
	return buf.String(), err
}

func TestDelegateNoArgsReturnsError(t *testing.T) {
	_, err := executeCommandWithMillDir(t, "delegate")
	if err == nil {
		t.Fatal("delegate with no args should return error")
	}
}

func TestDelegateValidIssueCreatesState(t *testing.T) {
	output, err := executeCommandWithMillDir(t, "delegate", "390")
	if err != nil {
		t.Fatalf("delegate returned error: %v", err)
	}

	// Verify state.json was created
	if _, err := os.Stat(statePath()); os.IsNotExist(err) {
		t.Fatal("expected .mill/state.json to be created")
	}

	// Verify the state has the task
	s, err := state.Load(statePath())
	if err != nil {
		t.Fatalf("failed to load state: %v", err)
	}

	task, ok := s.Task("task-390")
	if !ok {
		t.Fatal("expected task 'task-390' to exist in state")
	}

	if task.Issue != 390 {
		t.Errorf("expected issue %d, got %d", 390, task.Issue)
	}
	if task.Status != "pending" {
		t.Errorf("expected status %q, got %q", "pending", task.Status)
	}

	// Verify output mentions the issue
	if !bytes.Contains([]byte(output), []byte("390")) {
		t.Errorf("expected output to mention issue 390, got: %q", output)
	}
}

func TestDelegateCreatesLedgerEntry(t *testing.T) {
	_, err := executeCommandWithMillDir(t, "delegate", "42")
	if err != nil {
		t.Fatalf("delegate returned error: %v", err)
	}

	ledgerFile := ledgerPath(42)
	if _, err := os.Stat(ledgerFile); os.IsNotExist(err) {
		t.Fatal("expected ledger file to be created")
	}

	data, err := os.ReadFile(ledgerFile)
	if err != nil {
		t.Fatalf("failed to read ledger: %v", err)
	}

	if len(data) == 0 {
		t.Fatal("expected ledger file to have content")
	}
}

func TestDelegateInvalidIssueReturnsError(t *testing.T) {
	_, err := executeCommandWithMillDir(t, "delegate", "abc")
	if err == nil {
		t.Fatal("delegate with invalid issue should return error")
	}
}
