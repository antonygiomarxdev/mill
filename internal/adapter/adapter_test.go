package adapter

import (
	"testing"

	"github.com/antonygiomarxdev/mill/internal/domain"
)

// mockSession implements Session for testing.
type mockSession struct {
	id     string
	status string
	result SessionResult
	err    error
}

func (m *mockSession) ID() string                   { return m.id }
func (m *mockSession) Status() string               { return m.status }
func (m *mockSession) Wait() (SessionResult, error) { return m.result, m.err }
func (m *mockSession) ContextText() (string, error) { return "", nil }
func (m *mockSession) HeartbeatPath() string        { return ".mill/heartbeat" }

// mockAdapter implements Adapter for testing.
type mockAdapter struct {
	dispatchCalled bool
	dispatchOpts   DispatchOpts
	resumeCalled   bool
	resumeID       string
	caps           Capabilities
}

func (m *mockAdapter) Dispatch(opts DispatchOpts) (Session, error) {
	m.dispatchCalled = true
	m.dispatchOpts = opts
	return &mockSession{
		id:     "mock-session-1",
		status: sessionStatus(domain.SessionDone),
		result: SessionResult{ExitCode: 0, Commits: 5, Output: "APPROVED"},
	}, nil
}

func (m *mockAdapter) Resume(sessionID string) (Session, error) {
	m.resumeCalled = true
	m.resumeID = sessionID
	return &mockSession{
		id:     sessionID,
		status: sessionStatus(domain.SessionDone),
		result: SessionResult{ExitCode: 0, Commits: 3, Output: "Needs changes"},
	}, nil
}

func (m *mockAdapter) Capabilities() Capabilities {
	return m.caps
}

func (m *mockAdapter) DefaultModel() string {
	return "mock-default-model"
}

func (m *mockAdapter) DefaultFallbackChain() map[string][]string {
	return map[string][]string{
		"free": {"mock-model"},
	}
}

func (m *mockAdapter) FailureSignals() []domain.Signal { return nil }

func (m *mockAdapter) BinaryPath() string { return "/usr/local/bin/mill" }

func TestMockAdapterDispatchUsesDispatchOpts(t *testing.T) {
	a := &mockAdapter{
		caps: Capabilities{Models: []string{"gpt-5", "deepseek-v4-pro"}},
	}

	opts := DispatchOpts{
		Worktree: "/tmp/worktree",
		Prompt:   "fix the bug",
		Model:    "gpt-5",
		MaxTurns: 50,
	}

	s, err := a.Dispatch(opts)
	if err != nil {
		t.Fatalf("Dispatch returned error: %v", err)
	}

	if !a.dispatchCalled {
		t.Error("expected Dispatch to be called")
	}
	if a.dispatchOpts.Worktree != "/tmp/worktree" {
		t.Errorf("expected worktree %q, got %q", "/tmp/worktree", a.dispatchOpts.Worktree)
	}
	if a.dispatchOpts.Prompt != "fix the bug" {
		t.Errorf("expected prompt %q, got %q", "fix the bug", a.dispatchOpts.Prompt)
	}
	if a.dispatchOpts.Model != "gpt-5" {
		t.Errorf("expected model %q, got %q", "gpt-5", a.dispatchOpts.Model)
	}
	if a.dispatchOpts.MaxTurns != 50 {
		t.Errorf("expected max turns %d, got %d", 50, a.dispatchOpts.MaxTurns)
	}

	if s.ID() != "mock-session-1" {
		t.Errorf("expected session ID %q, got %q", "mock-session-1", s.ID())
	}

	result, err := s.Wait()
	if err != nil {
		t.Fatalf("Wait returned error: %v", err)
	}
	if result.ExitCode != 0 {
		t.Errorf("expected exit code 0, got %d", result.ExitCode)
	}
	if result.Commits != 5 {
		t.Errorf("expected commits 5, got %d", result.Commits)
	}
	if result.Output != "APPROVED" {
		t.Errorf("expected output %q, got %q", "APPROVED", result.Output)
	}
}

func TestMockAdapterResume(t *testing.T) {
	a := &mockAdapter{}

	s, err := a.Resume("session-123")
	if err != nil {
		t.Fatalf("Resume returned error: %v", err)
	}

	if !a.resumeCalled {
		t.Error("expected Resume to be called")
	}
	if a.resumeID != "session-123" {
		t.Errorf("expected resumeID %q, got %q", "session-123", a.resumeID)
	}

	if s.ID() != "session-123" {
		t.Errorf("expected session ID %q, got %q", "session-123", s.ID())
	}
}

func TestMockAdapterCapabilities(t *testing.T) {
	a := &mockAdapter{
		caps: Capabilities{Models: []string{"model-a", "model-b"}},
	}

	caps := a.Capabilities()
	if len(caps.Models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(caps.Models))
	}
	if caps.Models[0] != "model-a" {
		t.Errorf("expected model-a, got %q", caps.Models[0])
	}
}

func TestSessionResultFields(t *testing.T) {
	r := SessionResult{
		ExitCode: 42,
		Commits:  7,
		Output:   "some output",
	}

	if r.ExitCode != 42 {
		t.Errorf("expected exit code 42, got %d", r.ExitCode)
	}
	if r.Commits != 7 {
		t.Errorf("expected commits 7, got %d", r.Commits)
	}
	if r.Output != "some output" {
		t.Errorf("expected output %q, got %q", "some output", r.Output)
	}
}

func TestCapabilitiesModelsField(t *testing.T) {
	c := Capabilities{Models: []string{"x", "y", "z"}}
	if len(c.Models) != 3 {
		t.Errorf("expected 3 models, got %d", len(c.Models))
	}
}

func TestMockAdapterDefaultModel(t *testing.T) {
	a := &mockAdapter{}
	model := a.DefaultModel()
	if model != "mock-default-model" {
		t.Errorf("expected mock-default-model, got %q", model)
	}
}

func TestMockAdapterDefaultFallbackChain(t *testing.T) {
	a := &mockAdapter{}
	chain := a.DefaultFallbackChain()
	if len(chain) != 1 {
		t.Fatalf("expected 1 tier, got %d", len(chain))
	}
	if _, ok := chain["free"]; !ok {
		t.Fatal("expected 'free' tier in fallback chain")
	}
	if len(chain["free"]) != 1 || chain["free"][0] != "mock-model" {
		t.Errorf("expected 'free' tier to be [mock-model], got %v", chain["free"])
	}
}
