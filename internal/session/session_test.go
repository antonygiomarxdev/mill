package session

import (
	"testing"
)

func TestStatusValues(t *testing.T) {
	if Pending != "pending" {
		t.Errorf("expected Pending to be %q, got %q", "pending", Pending)
	}
	if Running != "running" {
		t.Errorf("expected Running to be %q, got %q", "running", Running)
	}
	if Done != "done" {
		t.Errorf("expected Done to be %q, got %q", "done", Done)
	}
	if Error != "error" {
		t.Errorf("expected Error to be %q, got %q", "error", Error)
	}
}

func TestVerdictValues(t *testing.T) {
	if Approved != "approved" {
		t.Errorf("expected Approved to be %q, got %q", "approved", Approved)
	}
	if Changes != "changes" {
		t.Errorf("expected Changes to be %q, got %q", "changes", Changes)
	}
	if Rejected != "rejected" {
		t.Errorf("expected Rejected to be %q, got %q", "rejected", Rejected)
	}
}

func TestNewSession(t *testing.T) {
	s := NewSession(390)

	if s.Issue != 390 {
		t.Errorf("expected issue %d, got %d", 390, s.Issue)
	}
	if s.Status != Pending {
		t.Errorf("expected status %q, got %q", Pending, s.Status)
	}
	if s.ID == "" {
		t.Error("expected non-empty ID")
	}
	if s.Commits != 0 {
		t.Errorf("expected commits 0, got %d", s.Commits)
	}
}

func TestNewSessionIDIsUnique(t *testing.T) {
	s1 := NewSession(1)
	s2 := NewSession(1)

	if s1.ID == s2.ID {
		t.Error("expected session IDs to be unique")
	}
}

func TestSessionSetFields(t *testing.T) {
	s := NewSession(42)

	s.Status = Running
	s.Commits = 3
	s.Verdict = Changes

	if s.Status != Running {
		t.Errorf("expected status %q, got %q", Running, s.Status)
	}
	if s.Commits != 3 {
		t.Errorf("expected commits 3, got %d", s.Commits)
	}
	if s.Verdict != Changes {
		t.Errorf("expected verdict %q, got %q", Changes, s.Verdict)
	}
}
