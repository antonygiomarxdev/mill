package cli

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"github.com/antonygiomarxdev/mill/internal/compact"
	"github.com/antonygiomarxdev/mill/internal/config"
	"github.com/antonygiomarxdev/mill/internal/domain"
	"github.com/antonygiomarxdev/mill/internal/slots"
	"github.com/antonygiomarxdev/mill/internal/state"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type errorReader struct{}

func (e errorReader) Read([]byte) (int, error) { return 0, fmt.Errorf("read error") }

func TestPromptReadError(t *testing.T) {
	in := bufio.NewReader(errorReader{})
	var buf bytes.Buffer
	result := prompt(in, &buf, "Label", "default")
	if result != "default" {
		t.Errorf("expected 'default' on read error, got %q", result)
	}
}

func TestRunWatchStateLoadError(t *testing.T) {
	dir := t.TempDir()
	statePath := filepath.Join(dir, "state.json")
	if err := os.WriteFile(statePath, []byte("{}"), 0o000); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0o444); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(dir, 0o755)

	buf := new(bytes.Buffer)
	app := &App{MillDir: dir, Out: buf, Err: buf}
	err := app.runWatch(nil)
	if err == nil {
		t.Fatal("expected error from state load failure")
	}
}

func TestRunStatusStateLoadError(t *testing.T) {
	dir := t.TempDir()
	statePath := filepath.Join(dir, "state.json")
	if err := os.WriteFile(statePath, []byte("{}"), 0o000); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0o444); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(dir, 0o755)

	buf := new(bytes.Buffer)
	app := &App{MillDir: dir, Out: buf, Err: buf}
	err := app.runStatus(nil)
	if err == nil {
		t.Fatal("expected error from state load failure")
	}
}

func TestRunRoleReadError(t *testing.T) {
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o000); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(dir, 0o755)

	buf := new(bytes.Buffer)
	app := &App{MillDir: dir, Out: buf, Err: buf}
	err := app.roleGet()
	if err == nil {
		t.Fatal("expected error from role read failure")
	}
}

func TestResolveModelEscalateError(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test\n"), 0o644)
	origDir, _ := os.Getwd()
	os.Chdir(dir)
	t.Cleanup(func() { os.Chdir(origDir) })

	origFn := modelAvailableFn
	defer func() { modelAvailableFn = origFn }()
	// Mark paid-model as unavailable; pro isn't in the config → error
	modelAvailableFn = func(model string) bool { return model != "paid-model" }

	app := &App{MillDir: dir}
	cfg := config.Config{Models: map[string]string{"paid": "paid-model"}}
	_, err := app.resolveModel("staff", "", cfg)
	if err == nil {
		t.Fatal("expected error when tier escalation fails")
	}
}

func TestAcquireSlotError(t *testing.T) {
	mgr := slots.NewManager(1)
	ctx, cancel := context.WithCancel(context.Background())
	_, err := mgr.Acquire(ctx, 1, "staff", false)
	if err != nil {
		t.Fatal(err)
	}
	cancel()
	buf := new(bytes.Buffer)
	_, err = AcquireSlot(ctx, mgr, buf, 2, "staff", false, 1)
	if err == nil {
		t.Fatal("expected error from cancelled context")
	}
	mgr.Release()
}

func TestResolveVersionEmptyOutput(t *testing.T) {
	dir := t.TempDir()
	fakeGit := filepath.Join(dir, "git")
	if err := os.WriteFile(fakeGit, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	origPath := os.Getenv("PATH")
	defer os.Setenv("PATH", origPath)
	os.Setenv("PATH", dir+":"+origPath)

	result := resolveVersion()
	if result != "dev" {
		t.Errorf("expected 'dev' for empty git output, got %q", result)
	}
}

func TestResolveRepoRefLocalPath(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(dir)
	t.Cleanup(func() { os.Chdir(origDir) })

	runGit(t, dir, "init")
	runGit(t, dir, "config", "user.email", "test@test.com")
	runGit(t, dir, "config", "user.name", "Test")
	otherDir := t.TempDir()
	runGit(t, otherDir, "init", "--bare")
	runGit(t, dir, "remote", "add", "origin", otherDir)

	got := resolveRepoRef()
	if got == "OWNER/REPO" {
		t.Errorf("expected non-placeholder repo ref, got %q", got)
	}
}

func TestCleanRemoveAllError(t *testing.T) {
	dir := t.TempDir()
	d := filepath.Join(dir, ".mill")
	s := state.New()
	s.UpsertTask(domain.Task{ID: "t1", Issue: 1, Status: domain.TaskDone})
	if err := s.Save(filepath.Join(d, "state.json")); err != nil {
		t.Fatal(err)
	}

	wt := filepath.Join(d, "worktrees", "issue-1")
	if err := os.MkdirAll(wt, 0755); err != nil {
		t.Fatal(err)
	}
	protected := filepath.Join(wt, "protected")
	if err := os.MkdirAll(protected, 0o444); err != nil {
		t.Fatal(err)
	}

	buf := new(bytes.Buffer)
	app := &App{MillDir: d, Out: buf, Err: buf}
	_ = app.runClean(nil)
}

func TestLogCostMkdirFail(t *testing.T) {
	dir := t.TempDir()
	costsParent := filepath.Join(dir, "sub")
	if err := os.WriteFile(costsParent, []byte("block"), 0o444); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(costsParent, 0o644)

	app := &App{MillDir: costsParent, Out: new(bytes.Buffer), Err: new(bytes.Buffer)}
	cfg := config.Config{Rate: 0}
	app.logCost(cfg, 1, "staff", "free", "free", 100, "test")
}

func TestLogCostOpenFail(t *testing.T) {
	dir := t.TempDir()
	costsFile := filepath.Join(dir, "costs.jsonl")
	if err := os.MkdirAll(costsFile, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(costsFile, 0o444); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(costsFile, 0o755)

	app := &App{MillDir: dir, Out: new(bytes.Buffer), Err: new(bytes.Buffer)}
	cfg := config.Config{Rate: 0}
	app.logCost(cfg, 1, "staff", "free", "free", 100, "test")
}

func TestCompactSessionMkdirError(t *testing.T) {
	dir := t.TempDir()
	worktree := filepath.Join(dir, "worktree")
	if err := os.MkdirAll(worktree, 0o755); err != nil {
		t.Fatal(err)
	}
	millAsFile := filepath.Join(worktree, ".mill")
	if err := os.WriteFile(millAsFile, []byte("block"), 0o444); err != nil {
		t.Fatal(err)
	}
	defer os.Remove(millAsFile)

	buf := new(bytes.Buffer)
	app := &App{Out: buf, Err: buf}
	session := &fakeSession{ctxText: strings.Repeat("x", 500000)}
	app.compactSession(session, "laguna-free", 1, compact.ModeFast, worktree)
}

func TestCleanRemoveAllPermissionError(t *testing.T) {
	dir := t.TempDir()
	d := filepath.Join(dir, ".mill")
	s := state.New()
	s.UpsertTask(domain.Task{ID: "t1", Issue: 1, Status: domain.TaskDone})
	if err := s.Save(filepath.Join(d, "state.json")); err != nil {
		t.Fatal(err)
	}

	wt := filepath.Join(d, "worktrees", "issue-1")
	if err := os.MkdirAll(wt, 0o755); err != nil {
		t.Fatal(err)
	}
	// Make worktree directory immutable so RemoveAll fails
	if err := os.Chmod(wt, 0o444); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(wt, 0o755)

	buf := new(bytes.Buffer)
	app := &App{MillDir: d, Out: buf, Err: buf}
	_ = app.runClean(nil)
}
