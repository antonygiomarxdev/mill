package adapter

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
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
			"claude-fable-5",
			"claude-opus-5",
			"claude-haiku-4-5",
			"deepseek-v4-pro",
			"deepseek-v4-flash",
			"laguna-s-2.1-free",
		},
		ReadTool: ReadToolCapabilities{
			LineCeiling:        2000,
			ByteCeiling:        128 * 1024, // 128KB
			CharCeiling:        500,
			HasSelectorSupport: true,
			HasRecoveryNotes:   true,
		},
	}
}

// DefaultModel returns the recommended default model for the CommandCode adapter.
func (a *CommandCodeAdapter) DefaultModel() string {
	return "laguna-s-2.1-free"
}

// DefaultFallbackChain returns bidirectional model fallback chains per tier.
// Each tier ensures at least two models from different providers so a
// rate-limited model falls back to a different provider.
func (a *CommandCodeAdapter) DefaultFallbackChain() map[string][]string {
	return map[string][]string{
		"free": {"laguna-s-2.1-free", "deepseek-v4-flash"},
		"paid": {"deepseek-v4-pro", "claude-sonnet-5", "deepseek-v4-flash", "laguna-s-2.1-free"},
		"pro":  {"deepseek-v4-pro", "claude-fable-5", "claude-sonnet-5"},
	}
}

// FailureSignals returns the shared declarative failure-classification signal
// table used to classify completed sessions into a domain.FailureClass.
func (a *CommandCodeAdapter) FailureSignals() []domain.Signal {
	return domain.NewSignalRegistry().Signals()
}

// BinaryPath returns the path to the mill executable so BinaryCopier can
// copy it into child worktrees. The resolved path is cached.
func (a *CommandCodeAdapter) BinaryPath() string {
	return resolveBinaryPath()
}

var cachedBinaryPath string

func resolveBinaryPath() string {
	if cachedBinaryPath != "" {
		return cachedBinaryPath
	}
	path, err := os.Executable()
	if err != nil {
		return "mill"
	}
	cachedBinaryPath = path
	return path
}

// Dispatch starts a `cmd -p` session in the given worktree.
// The process is started asynchronously; call Wait() on the returned
// Session to block until completion and collect the result.
func (a *CommandCodeAdapter) Dispatch(opts DispatchOpts) (Session, error) {
	args := buildArgs(opts)
	cmd := exec.Command("cmd", args...)
	cmd.Env = append(os.Environ(), "NODE_OPTIONS=--dns-result-order=ipv4first")
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
	ls := &liveSession{
		id:        generateID(),
		cmd:       cmd,
		outputBuf: &out,
		stderrBuf: &stderr,
		startedAt: time.Now().UTC(),
		status:    sessionStatus(domain.SessionRunning),
		budget:    opts.Budget,
		worktree:  opts.Worktree,
	}
	if opts.Worktree != "" {
		ls.startHeartbeat()
	}
	return ls, nil
}

// Resume reconnects to an existing CommandCode headless session by ID.
func (a *CommandCodeAdapter) Resume(sessionID string) (Session, error) {
	args := []string{
		"-p", "--resume", sessionID,
		"--yolo",
		"--skip-onboarding",
		"--output-format", "json",
	}

	cmd := exec.Command("cmd", args...)
	cmd.Env = append(os.Environ(), "NODE_OPTIONS=--dns-result-order=ipv4first")
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
		"--yolo",
		"--skip-onboarding",
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
	worktree  string
	parse     func(string) (string, string)

	// Heartbeat tracking: a goroutine writes a liveness file every second
	// while the session process is running.
	heartbeatStop   chan struct{}
	heartbeatExited chan struct{}
	lastHeartbeat   time.Time
	heartbeatMu     sync.Mutex
}

func (s *liveSession) ID() string     { return s.id }
func (s *liveSession) Status() string { return s.status }

// HeartbeatPath returns the filesystem path to the session heartbeat file,
// resolved within the worktree's .mill directory.
func (s *liveSession) HeartbeatPath() string {
	return filepath.Join(s.worktree, ".mill", "heartbeat")
}

// resolveAgentID determines the agent_id written in heartbeat frontmatter.
// It reads from .mill/agent_id, falls back to .mill/role, then to the session id.
func (s *liveSession) resolveAgentID() string {
	if s.worktree == "" {
		return s.id
	}
	agentIDPath := filepath.Join(s.worktree, ".mill", "agent_id")
	if data, err := os.ReadFile(agentIDPath); err == nil {
		if id := strings.TrimSpace(string(data)); id != "" {
			return id
		}
	}
	rolePath := filepath.Join(s.worktree, ".mill", "role")
	if data, err := os.ReadFile(rolePath); err == nil {
		if id := strings.TrimSpace(string(data)); id != "" {
			return id
		}
	}
	return s.id
}

// writeHeartbeat writes a YAML frontmatter heartbeat file to the worktree's
// .mill/heartbeat. On success it records the current time as the last heartbeat.
func (s *liveSession) writeHeartbeat() {
	if s.worktree == "" {
		return
	}
	millDir := filepath.Join(s.worktree, ".mill")
	os.MkdirAll(millDir, 0o755)
	now := time.Now().UTC()
	content := fmt.Sprintf("---\nagent_id: %s\ntimestamp: %s\n---\n",
		s.resolveAgentID(), now.Format(time.RFC3339))
	if err := os.WriteFile(filepath.Join(millDir, "heartbeat"), []byte(content), 0o644); err == nil {
		s.heartbeatMu.Lock()
		s.lastHeartbeat = now
		s.heartbeatMu.Unlock()
	}
}

// startHeartbeat launches a goroutine that writes a heartbeat file every
// second. The goroutine must be stopped via stopHeartbeat before Wait returns.
func (s *liveSession) startHeartbeat() {
	s.heartbeatStop = make(chan struct{})
	s.heartbeatExited = make(chan struct{})
	go func() {
		defer close(s.heartbeatExited)
		s.writeHeartbeat()
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				s.writeHeartbeat()
			case <-s.heartbeatStop:
				return
			}
		}
	}()
}

// stopHeartbeat signals the heartbeat goroutine to stop, waits for it to exit,
// and returns the duration since the last successful heartbeat write.
func (s *liveSession) stopHeartbeat() time.Duration {
	if s.heartbeatStop == nil {
		return time.Since(time.Time{})
	}
	close(s.heartbeatStop)
	<-s.heartbeatExited
	s.heartbeatMu.Lock()
	staleness := time.Since(s.lastHeartbeat)
	s.heartbeatMu.Unlock()
	return staleness
}

// ContextText returns the full NDJSON session context for compaction.
func (s *liveSession) ContextText() (string, error) {
	return s.outputBuf.String(), nil
}

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
	staleness := s.stopHeartbeat()
	exitCode := 0

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
			s.status = sessionStatus(domain.SessionError)
		} else {
			s.status = sessionStatus(domain.SessionError)
			return SessionResult{
				ExitCode:           -1,
				Stderr:             s.stderrBuf.String(),
				HeartbeatStaleness: staleness,
			}, err
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
		ExitCode:           exitCode,
		Commits:            commits,
		Output:             finalText,
		Stderr:             s.stderrBuf.String(),
		HeartbeatStaleness: staleness,
	}, nil
}

// waitWithBudget enforces time and analysis-paralysis budgets.
func (s *liveSession) waitWithBudget() (SessionResult, error) {
	timeout := time.Duration(s.budget.TimeSeconds) * time.Second
	exitErr, timeKilled := waitCmd(s.cmd, timeout)
	staleness := s.stopHeartbeat()

	output := s.outputBuf.String()

	if timeKilled {
		s.status = sessionStatus(domain.SessionError)
		return SessionResult{
			ExitCode:           -1,
			Commits:            countCommits(output),
			Stderr:             "blocked: time budget exceeded",
			HeartbeatStaleness: staleness,
		}, nil
	}

	if exitErr != nil {
		s.status = sessionStatus(domain.SessionError)
		return SessionResult{
			ExitCode:           exitErr.ExitCode(),
			Commits:            countCommits(output),
			Stderr:             s.stderrBuf.String(),
			HeartbeatStaleness: staleness,
		}, nil
	}

	// Process succeeded — check for analysis paralysis.
	if s.budget.MaxTurns > 0 && detectAnalysisParalysis(output, s.budget.MaxTurns) {
		s.status = sessionStatus(domain.SessionError)
		return SessionResult{
			ExitCode:           -2,
			Commits:            countCommits(output),
			Stderr:             "blocked: analysis paralysis detected",
			HeartbeatStaleness: staleness,
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
		ExitCode:           0,
		Commits:            commits,
		Output:             finalText,
		Stderr:             s.stderrBuf.String(),
		HeartbeatStaleness: staleness,
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
	Type  string      `json:"type"`
	Event *frameEvent `json:"event,omitempty"`
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
			Type      string `json:"type"`
			FinalText string `json:"finalText"`
			SessionID string `json:"sessionId"`
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
