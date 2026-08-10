package adapter

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/antonygiomarxdev/mill/internal/domain"
	"github.com/antonygiomarxdev/mill/internal/repair"
)

// OpenCodeAdapter implements the Adapter interface for the OpenCode CLI.
// It spawns `opencode run --format json --auto`.
type OpenCodeAdapter struct{}

// Capabilities returns the models supported by the OpenCode adapter.
func (a *OpenCodeAdapter) Capabilities() Capabilities {
	return Capabilities{
		Models: []string{
			"opencode/claude-sonnet-5",
			"opencode/claude-sonnet-4-6",
			"opencode/deepseek-v4-pro",
			"opencode/deepseek-v4-flash",
			"opencode/gpt-5",
		},
	}
}

// Dispatch starts an `opencode run` session in the given worktree.
// The process is started asynchronously; call Wait() on the returned
// Session to block until completion and collect the result.
func (a *OpenCodeAdapter) Dispatch(opts DispatchOpts) (Session, error) {
	// Apply repair pipeline to tool call inputs before spawning.
	input, err := json.Marshal(opts)
	if err == nil {
		repaired, _ := repair.Repair(input)
		_ = json.Unmarshal(repaired, &opts)
	}

	args := buildOpenCodeArgs(opts)

	cmd := exec.Command("opencode", args...)
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
		return nil, fmt.Errorf("failed to start opencode: %w", err)
	}

	return &liveSession{
		id:        generateID(),
		cmd:       cmd,
		outputBuf: &out,
		stderrBuf: &stderr,
		startedAt: time.Now().UTC(),
		status:    sessionStatus(domain.SessionRunning),
		parse:     parseOpenCodeOutput,
	}, nil
}

// Resume reconnects to an existing OpenCode session by ID.
func (a *OpenCodeAdapter) Resume(sessionID string) (Session, error) {
	args := []string{
		"run", "--format", "json", "--auto",
		"-s", sessionID,
	}

	cmd := exec.Command("opencode", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	var out bytes.Buffer
	cmd.Stdout = &out

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start opencode: %w", err)
	}

	return &liveSession{
		id:        sessionID,
		cmd:       cmd,
		outputBuf: &out,
		stderrBuf: &stderr,
		startedAt: time.Now().UTC(),
		status:    sessionStatus(domain.SessionRunning),
		parse:     parseOpenCodeOutput,
	}, nil
}

// buildOpenCodeArgs constructs the argument list for `opencode run`.
func buildOpenCodeArgs(opts DispatchOpts) []string {
	args := []string{
		"run", "--format", "json", "--auto",
		"-m", opts.Model,
		opts.Prompt,
	}
	return args
}

// parseOpenCodeOutput scans NDJSON output from `opencode run --format json`
// and extracts the accumulated text from text events and the sessionID.
func parseOpenCodeOutput(output string) (finalText string, sessionID string) {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		var frame struct {
			Type      string `json:"type"`
			SessionID string `json:"sessionID"`
			Part      struct {
				Text string `json:"text"`
			} `json:"part"`
			Error struct {
				Data struct {
					Message string `json:"message"`
				} `json:"data"`
			} `json:"error"`
		}
		if err := json.Unmarshal([]byte(line), &frame); err != nil {
			continue
		}

		if frame.SessionID != "" {
			sessionID = frame.SessionID
		}

		if frame.Type == "text" {
			finalText += frame.Part.Text
		}

		if frame.Type == "error" && frame.Error.Data.Message != "" {
			if finalText != "" {
				finalText += "\n"
			}
			finalText += frame.Error.Data.Message
		}
	}
	return finalText, sessionID
}
