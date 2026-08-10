package adapter

import (
	"testing"
)

// mockSession implements Session for testing.
type mockSession struct {
	id     string
	status string
	result SessionResult
}

func (m *mockSession) ID() string      { return m.id }
func (m *mockSession) Status() string  { return m.status }
func (m *mockSession) Wait() SessionResult { return m.result }

// mockAdapter implements Adapter for testing.
type mockAdapter struct {
	called       bool
	dispatchArgs struct {
		worktree string
		prompt   string
		model    string
	}
	resumeID string
	caps     Capabilities
}

func (m *mockAdapter) Dispatch(worktree, prompt, model string) (Session, error) {
	m.called = true
	m.dispatchArgs.worktree = worktree
	m.dispatchArgs.prompt = prompt
	m.dispatchArgs.model = model
	return &mockSession{
		id:     "mock-session-1",
		status: "done",
		result: SessionResult{ExitCode: 0, Commits: 5, Verdict: "approved"},
	}, nil
}

func (m *mockAdapter) Resume(sessionID string) (Session, error) {
	m.resumeID = sessionID
	return &mockSession{
		id:     sessionID,
		status: "done",
		result: SessionResult{ExitCode: 0, Commits: 3, Verdict: "changes"},
	}, nil
}

func (m *mockAdapter) Capabilities() Capabilities {
	return m.caps
}

func TestMockAdapterDispatch(t *testing.T) {
	a := &mockAdapter{
		caps: Capabilities{Models: []string{"gpt-5", "deepseek-v4-pro"}},
	}

	s, err := a.Dispatch("wt-1", "fix the bug", "gpt-5")
	if err != nil {
		t.Fatalf("Dispatch returned error: %v", err)
	}

	if !a.called {
		t.Error("expected Dispatch to be called")
	}
	if a.dispatchArgs.worktree != "wt-1" {
		t.Errorf("expected worktree %q, got %q", "wt-1", a.dispatchArgs.worktree)
	}
	if a.dispatchArgs.prompt != "fix the bug" {
		t.Errorf("expected prompt %q, got %q", "fix the bug", a.dispatchArgs.prompt)
	}
	if a.dispatchArgs.model != "gpt-5" {
		t.Errorf("expected model %q, got %q", "gpt-5", a.dispatchArgs.model)
	}

	if s.ID() != "mock-session-1" {
		t.Errorf("expected session ID %q, got %q", "mock-session-1", s.ID())
	}
	if s.Status() != "done" {
		t.Errorf("expected status %q, got %q", "done", s.Status())
	}

	result := s.Wait()
	if result.ExitCode != 0 {
		t.Errorf("expected exit code 0, got %d", result.ExitCode)
	}
	if result.Commits != 5 {
		t.Errorf("expected commits 5, got %d", result.Commits)
	}
	if result.Verdict != "approved" {
		t.Errorf("expected verdict %q, got %q", "approved", result.Verdict)
	}
}

func TestMockAdapterResume(t *testing.T) {
	a := &mockAdapter{}

	s, err := a.Resume("session-123")
	if err != nil {
		t.Fatalf("Resume returned error: %v", err)
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
		Verdict:  "rejected",
	}

	if r.ExitCode != 42 {
		t.Errorf("expected exit code 42, got %d", r.ExitCode)
	}
	if r.Commits != 7 {
		t.Errorf("expected commits 7, got %d", r.Commits)
	}
	if r.Verdict != "rejected" {
		t.Errorf("expected verdict %q, got %q", "rejected", r.Verdict)
	}
}

func TestCapabilitiesModelsField(t *testing.T) {
	c := Capabilities{Models: []string{"x", "y", "z"}}
	if len(c.Models) != 3 {
		t.Errorf("expected 3 models, got %d", len(c.Models))
	}
}
