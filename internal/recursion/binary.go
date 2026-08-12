package recursion

import (
	"io"
	"os"
	"path/filepath"

	"github.com/antonygiomarxdev/mill/internal/domain"
)

// BinaryCopier copies the mill binary into a child worktree before spawning
// a child session, so the child can re-invoke mill recursively.
type BinaryCopier struct {
	// SourcePath is the path to the mill binary to copy. If empty, the
	// executable backing the current process is used.
	SourcePath string
}

// ExePath is the in-worktree location the binary is installed at.
const exePath = "mill"

// Copy installs the mill binary into worktree's .mill/bin/mill and makes it
// executable. It returns CLASS_OK on success, ENVIRONMENT_FAILURE on
// permission denied / cross-device link / disk full / missing binary —
// environment issues by the failure taxonomy, never FATAL.
func (c BinaryCopier) Copy(worktree string) domain.FailureClass {
	src := c.SourcePath
	if src == "" {
		src = defaultMillBinary()
	}
	if src == "" {
		return domain.ENVIRONMENT_FAILURE
	}

	info, err := os.Stat(src)
	if err != nil {
		return domain.ENVIRONMENT_FAILURE
	}
	// A directory is not a usable binary.
	if info.IsDir() {
		return domain.ENVIRONMENT_FAILURE
	}

	dstDir := filepath.Join(worktree, ".mill", "bin")
	if err := os.MkdirAll(dstDir, 0o755); err != nil {
		return domain.ENVIRONMENT_FAILURE
	}
	dst := filepath.Join(dstDir, exePath)
	if err := copyFile(src, dst); err != nil {
		return domain.ENVIRONMENT_FAILURE
	}
	// Executable for the child re-invocation.
	if err := os.Chmod(dst, 0o755); err != nil {
		return domain.ENVIRONMENT_FAILURE
	}
	return domain.CLASS_OK
}

// defaultMillBinary resolves the mill binary path, or "" if unavailable.
func defaultMillBinary() string {
	if exe, err := os.Executable(); err == nil && exe != "" {
		return exe
	}
	return ""
}

// copyFile copies src to dst. The fast path tries a hard link (no data copy);
// on a cross-device link (EXDEV) it falls back to a streaming io.Copy, which
// surfaces disk-full as a write error. All errors cascade to ENVIRONMENT_FAILURE.
func copyFile(src, dst string) error {
	if err := os.Link(src, dst); err == nil {
		return nil
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Sync()
}
