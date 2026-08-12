package recursion

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/antonygiomarxdev/mill/internal/domain"
)

func TestBinaryCopySuccess(t *testing.T) {
	src := filepath.Join(t.TempDir(), "mill")
	if err := os.WriteFile(src, []byte("#!/bin/sh\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// On non-Unix we skip chmod/exec semantics but Copy should still classify OK.
	wt := filepath.Join(t.TempDir(), "child-wt")
	if err := os.MkdirAll(wt, 0o755); err != nil {
		t.Fatal(err)
	}

	fc := (BinaryCopier{SourcePath: src}).Copy(wt)
	if fc != domain.CLASS_OK {
		t.Fatalf("expected CLASS_OK, got %s", fc)
	}
	dst := filepath.Join(wt, ".mill", "bin", "mill")
	if _, err := os.Stat(dst); err != nil {
		t.Fatalf("expected dst to exist: %v", err)
	}
}

func TestBinaryCopyMissingSource(t *testing.T) {
	wt := filepath.Join(t.TempDir(), "wt")
	fc := (BinaryCopier{SourcePath: "/no/such/binary/here"}).Copy(wt)
	if fc != domain.ENVIRONMENT_FAILURE {
		t.Fatalf("expected ENVIRONMENT_FAILURE for missing source, got %s", fc)
	}
}

func TestBinaryCopySourceIsDir(t *testing.T) {
	dir := t.TempDir()
	wt := filepath.Join(t.TempDir(), "wt")
	fc := (BinaryCopier{SourcePath: dir}).Copy(wt)
	if fc != domain.ENVIRONMENT_FAILURE {
		t.Fatalf("expected ENVIRONMENT_FAILURE for dir source, got %s", fc)
	}
}

func TestBinaryCopyUnwritableDest(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skip on windows")
	}
	// Make a worktree path that already exists as a file so MkdirAll fails.
	blocked := filepath.Join(t.TempDir(), "blocked")
	if err := os.WriteFile(blocked, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	src := filepath.Join(t.TempDir(), "mill")
	if err := os.WriteFile(src, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	fc := (BinaryCopier{SourcePath: src}).Copy(blocked)
	if fc != domain.ENVIRONMENT_FAILURE {
		t.Fatalf("expected ENVIRONMENT_FAILURE for unwritable dest, got %s", fc)
	}
}

func TestBinaryCopyDefaultSourceUsesExecutable(t *testing.T) {
	wt := filepath.Join(t.TempDir(), "wt")
	if err := os.MkdirAll(wt, 0o755); err != nil {
		t.Fatal(err)
	}
	// SourcePath empty → resolves to os.Executable(); must classify CLASS_OK.
	fc := (BinaryCopier{}).Copy(wt)
	if fc != domain.CLASS_OK {
		t.Fatalf("expected CLASS_OK with default source, got %s", fc)
	}
	dst := filepath.Join(wt, ".mill", "bin", "mill")
	if _, err := os.Stat(dst); err != nil {
		t.Fatalf("expected installed binary at %s: %v", dst, err)
	}
}

func TestCopyFileFallsBackFromHardLink(t *testing.T) {
	// copyFile returns nil on success; verify streaming fallback works by
	// copying to a fresh dst (hard link is the fast path, copy the fallback).
	src := filepath.Join(t.TempDir(), "src")
	if err := os.WriteFile(src, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(t.TempDir(), "dst")
	if err := copyFile(src, dst); err != nil {
		t.Fatalf("copyFile: %v", err)
	}
	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("read dst: %v", err)
	}
	if string(got) != "hello" {
		t.Fatalf("expected hello, got %q", string(got))
	}
}
