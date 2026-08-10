package adapter

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/antonygiomarxdev/mill/internal/domain"
)

// CommandCodeAdapter implements the Adapter interface for the CommandCode CLI.
// It spawns `cmd -p` in headless mode with JSON output.
type CommandCodeAdapter struct{}

// Capabilities returns the models supported by the CommandCode adapter.
func (a *CommandCodeAdapter) Capabilities() Capabilities {
	return Capabilities{
		Models: []string{
			"claude-sonnet-5",
			"claude-sonnet-4-6",
			"deepseek-v4-pro",
			"deepseek-v4-flash",
			"gpt-5",
		},
	}
}

// Dispatch starts a `cmd -p` session in the given worktree.
// The process is started asynchronously; call Wait() on the returned
// Session to block until completion and collect the result.
func (a *CommandCodeAdapter) Dispatch(opts DispatchOpts) (Session, error) {
	args := buildArgs(opts)

	cmd := exec.Command("cmd", args...)
	if opts.Worktree != "" {
		if err := os.MkdirAll(opts.Worktree, 0o755); err != nil {
			return nil, fmt.Errorf("failed to create worktree %s: %w", opts.Worktree, err)
		}
		cmd.Dir = opts.Worktree
	}
	cmd.Stderr = os.Stderr

	var out bytes.Buffer
	cmd.Stdout = &out

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start cmd: %w", err)
	}

	return &liveSession{
		id:        generateID(),
		cmd:       cmd,
		outputBuf: &out,
		startedAt: time.Now().UTC(),
		status:    sessionStatus(domain.SessionRunning),
	}, nil
}

// Resume reconnects to an existing CommandCode headless session by ID.
func (a *CommandCodeAdapter) Resume(sessionID string) (Session, error) {
	args := []string{
		"-p", "--resume", sessionID,
		"--output-format", "json",
	}

	cmd := exec.Command("cmd", args...)
	cmd.Stderr = os.Stderr

	var out bytes.Buffer
	cmd.Stdout = &out

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start cmd: %w", err)
	}

	return &liveSession{
		id:        sessionID,
		cmd:       cmd,
		outputBuf: &out,
		startedAt: time.Now().UTC(),
		status:    sessionStatus(domain.SessionRunning),
	}, nil
}

// buildArgs constructs the argument list for `cmd -p`.
func buildArgs(opts DispatchOpts) []string {
	args := []string{
		"-p", opts.Prompt,
		"-m", opts.Model,
		"--output-format", "json",
	}
	if opts.MaxTurns > 0 {
		args = append(args, "--max-turns", strconv.Itoa(opts.MaxTurns))
	}
	return args
}

// liveSession implements Session for a running exec.Cmd process.
type liveSession struct {
	id        string
	cmd       *exec.Cmd
	outputBuf *bytes.Buffer
	startedAt time.Time
	status    string
}

func (s *liveSession) ID() string    { return s.id }
func (s *liveSession) Status() string { return s.status }

func (s *liveSession) Wait() (SessionResult, error) {
	err := s.cmd.Wait()
	exitCode := 0

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
			s.status = sessionStatus(domain.SessionError)
		} else {
			s.status = sessionStatus(domain.SessionError)
			return SessionResult{ExitCode: -1}, err
		}
	} else {
		s.status = sessionStatus(domain.SessionDone)
	}

	output := s.outputBuf.String()
	finalText, _ := parseJSONOutput(output)
	commits := countCommits(output)

	return SessionResult{
		ExitCode: exitCode,
		Commits:  commits,
		Output:   finalText,
	}, nil
}

// parseJSONOutput scans NDJSON output from `cmd -p --output-format json`
// and extracts the finalText field and sessionId from the result frame.
func parseJSONOutput(output string) (finalText string, sessionID string) {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		var frame struct {
			Type      string          `json:"type"`
			FinalText string          `json:"finalText"`
			SessionID string          `json:"sessionId"`
		}
		if err := json.Unmarshal([]byte(line), &frame); err != nil {
			continue
		}

		if frame.Type == "result" {
			finalText = frame.FinalText
			sessionID = frame.SessionID
		}
	}
	return finalText, sessionID
}

// countCommits counts occurrences of "git commit" in the output text.
// This captures tool_running events that invoke git commits.
func countCommits(output string) int {
	return strings.Count(strings.ToLower(output), "git commit")
}

// generateID creates a unique session ID.
func generateID() string {
	return "sess-" + strconv.FormatInt(time.Now().UnixNano(), 36)
}
