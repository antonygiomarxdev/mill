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
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	var out bytes.Buffer
	cmd.Stdout = &out

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start cmd: %w", err)
	}

	return &liveSession{
		id:        generateID(),
		cmd:       cmd,
		outputBuf: &out,
		stderrBuf: &stderr,
		startedAt: time.Now().UTC(),
		status:    sessionStatus(domain.SessionRunning),
		budget:    opts.Budget,
	}, nil
}

// Resume reconnects to an existing CommandCode headless session by ID.
func (a *CommandCodeAdapter) Resume(sessionID string) (Session, error) {
	args := []string{
		"-p", "--resume", sessionID,
		"--output-format", "json",
	}

	cmd := exec.Command("cmd", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	var out bytes.Buffer
	cmd.Stdout = &out

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start cmd: %w", err)
	}

	return &liveSession{
		id:        sessionID,
		cmd:       cmd,
		outputBuf: &out,
		stderrBuf: &stderr,
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
	stderrBuf *bytes.Buffer
	startedAt time.Time
	status    string
	budget    *Budget
	parse     func(string) (string, string)
}

func (s *liveSession) ID() string     { return s.id }
func (s *liveSession) Status() string { return s.status }

// Wait blocks until the session completes.
// Zero-value budget fast-paths to cmd.Wait() for backward compatibility.
// With a budget, it enforces a wall-clock deadline and detects analysis
// paralysis by scanning NDJSON frames for consecutive thinking events
// with no intervening file writes.
func (s *liveSession) Wait() (SessionResult, error) {
	if s.budget == nil || (s.budget.TimeSeconds == 0 && s.budget.MaxTurns == 0) {
		return s.waitSimple()
	}
	return s.waitWithBudget()
}

// waitSimple is the original, backward-compatible Wait() path.
func (s *liveSession) waitSimple() (SessionResult, error) {
	err := s.cmd.Wait()
	exitCode := 0

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
			s.status = sessionStatus(domain.SessionError)
		} else {
			s.status = sessionStatus(domain.SessionError)
			return SessionResult{ExitCode: -1, Stderr: s.stderrBuf.String()}, err
		}
	} else {
		s.status = sessionStatus(domain.SessionDone)
	}

	output := s.outputBuf.String()
	parser := s.parse
	if parser == nil {
		parser = parseJSONOutput
	}
	finalText, _ := parser(output)
	commits := countCommits(output)

	return SessionResult{
		ExitCode: exitCode,
		Commits:  commits,
		Output:   finalText,
		Stderr:   s.stderrBuf.String(),
	}, nil
}

// waitWithBudget enforces time and analysis-paralysis budgets.
func (s *liveSession) waitWithBudget() (SessionResult, error) {
	timeout := time.Duration(s.budget.TimeSeconds) * time.Second
	exitErr, timeKilled := waitCmd(s.cmd, timeout)

	output := s.outputBuf.String()

	if timeKilled {
		s.status = sessionStatus(domain.SessionError)
		return SessionResult{
			ExitCode: -1,
			Commits:  countCommits(output),
			Stderr:   "blocked: time budget exceeded",
		}, nil
	}

	if exitErr != nil {
		s.status = sessionStatus(domain.SessionError)
		return SessionResult{
			ExitCode: exitErr.ExitCode(),
			Commits:  countCommits(output),
			Stderr:   s.stderrBuf.String(),
		}, nil
	}

	// Process succeeded — check for analysis paralysis.
	if s.budget.MaxTurns > 0 && detectAnalysisParalysis(output, s.budget.MaxTurns) {
		s.status = sessionStatus(domain.SessionError)
		return SessionResult{
			ExitCode: -2,
			Commits:  countCommits(output),
			Stderr:   "blocked: analysis paralysis detected",
		}, nil
	}

	s.status = sessionStatus(domain.SessionDone)
	parser := s.parse
	if parser == nil {
		parser = parseJSONOutput
	}
	finalText, _ := parser(output)
	commits := countCommits(output)

	return SessionResult{
		ExitCode: 0,
		Commits:  commits,
		Output:   finalText,
		Stderr:   s.stderrBuf.String(),
	}, nil
}

// waitCmd waits for a command to finish, optionally with a timeout.
// Returns (exitErr, true) when killed by the timeout.
func waitCmd(cmd *exec.Cmd, timeout time.Duration) (exitErr *exec.ExitError, timeKilled bool) {
	if timeout <= 0 {
		err := cmd.Wait()
		if err != nil {
			if ee, ok := err.(*exec.ExitError); ok {
				return ee, false
			}
		}
		return nil, false
	}

	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case err := <-done:
		if err != nil {
			if ee, ok := err.(*exec.ExitError); ok {
				return ee, false
			}
		}
		return nil, false
	case <-timer.C:
		cmd.Process.Kill()
		<-done
		return nil, true
	}
}

// ndjsonFrame is a single line of NDJSON from the agent process.
type ndjsonFrame struct {
	Type  string       `json:"type"`
	Event *frameEvent  `json:"event,omitempty"`
}

// frameEvent is the nested event object inside an NDJSON frame.
type frameEvent struct {
	Type     string `json:"type"`
	ToolName string `json:"toolName"`
}

// detectAnalysisParalysis scans NDJSON output for N consecutive thinking
// frames with no intervening file-write events.
func detectAnalysisParalysis(output string, maxConsecutive int) bool {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	consecutiveThink := 0

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		var frame ndjsonFrame
		if err := json.Unmarshal([]byte(line), &frame); err != nil {
			continue
		}

		if isThinkingFrame(frame) {
			consecutiveThink++
			if consecutiveThink >= maxConsecutive {
				return true
			}
		} else if isWriteFrame(frame) {
			consecutiveThink = 0
		}
	}

	return false
}

// isThinkingFrame reports whether an NDJSON frame represents model thinking.
func isThinkingFrame(frame ndjsonFrame) bool {
	if frame.Type == "thinking" {
		return true
	}
	if frame.Type == "event" && frame.Event != nil && frame.Event.Type == "thinking" {
		return true
	}
	return false
}

// isWriteFrame reports whether an NDJSON frame represents a file write.
func isWriteFrame(frame ndjsonFrame) bool {
	if frame.Type != "event" || frame.Event == nil {
		return false
	}
	et := frame.Event.Type
	if et != "tool_use" && et != "tool_running" {
		return false
	}
	name := strings.ToLower(frame.Event.ToolName)
	return strings.Contains(name, "write") || strings.Contains(name, "edit")
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
