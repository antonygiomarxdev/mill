// Package compact implements structural conversation compaction for Mill
// agent sessions. Compaction trims old conversation context before it
// exhausts the model's context window, preserving the original prompt,
// active role, last 3 turns, and unresolved items.
//
// Compaction is structural (drop old, keep recent + state), not semantic
// (no LLM-powered summarization). Every compaction event is logged as
// JSONL to .mill/compact.log.
package compact

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"
)

// Mode represents a compaction strategy.
type Mode string

const (
	// ModeFast is the only currently supported compaction mode.
	ModeFast Mode = "fast"
)

// Config mirrors config.CompactConfig for use within the compact package.
type Config struct {
	Enabled bool
	Mode    Mode
}

// ContextWindow maps model tiers to their context-window size in tokens.
type ContextWindow int

// Known model-tier context windows.
const (
	WindowFree ContextWindow = 128_000
	WindowPaid ContextWindow = 200_000
	WindowPro  ContextWindow = 200_000
)

// windowForTier returns the context window for a model tier string.
// Unknown tiers default to 128K (conservative).
func windowForTier(tier string) ContextWindow {
	switch tier {
	case "free":
		return WindowFree
	case "paid":
		return WindowPaid
	case "pro":
		return WindowPro
	default:
		return WindowFree
	}
}

// Event is a single compaction event serialized to .mill/compact.log.
type Event struct {
	Timestamp  time.Time `json:"timestamp"`
	Issue      int       `json:"issue"`
	PreTokens  int       `json:"pre_tokens"`
	PostTokens int       `json:"post_tokens"`
	Saved      int       `json:"saved"`
	Trigger    string    `json:"trigger"`
}

// Threshold is the fraction of the context window at which compaction fires.
const Threshold = 0.80

// ShouldCompact returns true when estimated tokens reach 80% of the
// context window for tier. It also returns the token estimate.
// Zero-length context returns (false, 0).
func ShouldCompact(contextText string, tier string) (bool, int) {
	if len(contextText) == 0 {
		return false, 0
	}

	window := windowForTier(tier)
	estimated := estimateTokens(contextText)
	threshold := int(float64(window) * Threshold)

	return estimated >= threshold, estimated
}

// estimateTokens returns a rough token count: length / 4.
func estimateTokens(text string) int {
	return len(text) / 4
}

// turn represents a single turn in the conversation.
type turn struct {
	text string
	idx  int
}

// Compact compacts the conversation context for issueNum using tier to
// determine the context window. It returns the compacted text and the log event.
func Compact(contextText string, tier string, issueNum int) (string, Event) {
	preTokens := estimateTokens(contextText)

	turns := splitTurns(contextText)

	originalPrompt := ""
	if len(turns) > 0 {
		idx := strings.Index(contextText, turns[0].text)
		if idx > 0 {
			originalPrompt = contextText[:idx]
		}
	} else {
		return contextText, Event{
			Timestamp:  time.Now().UTC(),
			Issue:      issueNum,
			PreTokens:  preTokens,
			PostTokens: preTokens,
			Saved:      0,
			Trigger:    "auto",
		}
	}

	n := len(turns)
	keepSet := make(map[int]bool)

	// Always keep last 3 turns.
	for i := n - 1; i >= 0 && i >= n-3; i-- {
		keepSet[i] = true
	}

	// Keep unresolved items.
	for i := range n - 3 {
		t := strings.ToLower(turns[i].text)
		if strings.Contains(t, "blocked") || strings.Contains(t, "unresolved") {
			keepSet[i] = true
		}
	}

	// Keep role/capability boundaries.
	for i := range n - 3 {
		if strings.Contains(turns[i].text, ".mill/role") || strings.Contains(turns[i].text, "ROLE.md") {
			keepSet[i] = true
		}
	}

	// Count summary stats for discarded turns.
	var exploredCount, changedCount, errorCount int
	for i := range n {
		if keepSet[i] {
			continue
		}
		t := turns[i].text
		if strings.Contains(t, "Tool: read") || strings.Contains(t, "Tool: grep") ||
			strings.Contains(t, "Tool: glob") || strings.Contains(t, "explore") {
			exploredCount++
		}
		if strings.Contains(t, "Tool: write") || strings.Contains(t, "Tool: edit") ||
			strings.Contains(t, "Tool: bash") || strings.Contains(t, "file change") {
			changedCount++
		}
		if strings.Contains(strings.ToLower(t), "error") || strings.Contains(t, "failed") ||
			strings.Contains(t, "FAIL") {
			errorCount++
		}
	}

	var b strings.Builder

	if originalPrompt != "" {
		b.WriteString(strings.TrimSpace(originalPrompt))
		b.WriteString("\n\n")
	}

	discardedCount := 0
	for i := range n {
		if !keepSet[i] {
			discardedCount++
		}
	}
	if discardedCount > 0 {
		b.WriteString(fmt.Sprintf(
			"[COMPACTED: explored %d paths, made %d changes, resolved %d errors. Full history in .mill/compact.log]\n\n",
			exploredCount, changedCount, errorCount,
		))
	}

	for i := range n {
		if keepSet[i] {
			b.WriteString(strings.TrimSpace(turns[i].text))
			b.WriteString("\n\n")
		}
	}

	compacted := strings.TrimSpace(b.String())
	postTokens := estimateTokens(compacted)

	event := Event{
		Timestamp:  time.Now().UTC(),
		Issue:      issueNum,
		PreTokens:  preTokens,
		PostTokens: postTokens,
		Saved:      preTokens - postTokens,
		Trigger:    "auto",
	}

	return compacted, event
}

// splitTurns splits the conversation context into individual turns.
func splitTurns(text string) []turn {
	lines := strings.Split(text, "\n")
	var turns []turn

	var current strings.Builder
	turnIdx := 0

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		isBoundary := strings.HasPrefix(trimmed, "{\"role\":\"user\"") ||
			strings.Contains(trimmed, "\"role\":\"user\"") ||
			strings.HasPrefix(trimmed, "user>") ||
			strings.HasPrefix(trimmed, "User:") ||
			strings.HasPrefix(trimmed, "Human:")

		if isBoundary && current.Len() > 0 {
			turns = append(turns, turn{text: current.String(), idx: turnIdx})
			turnIdx++
			current.Reset()
		}
		current.WriteString(line)
		current.WriteString("\n")
	}

	if current.Len() > 0 {
		turns = append(turns, turn{text: current.String(), idx: turnIdx})
	}

	return turns
}

// WriteLog appends a compaction event as a JSONL line to .mill/compact.log.
func WriteLog(event Event) error {
	f, err := os.OpenFile(".mill/compact.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("failed to open compact.log: %w", err)
	}
	defer f.Close()

	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal compaction event: %w", err)
	}

	if _, err := f.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("failed to write compaction event: %w", err)
	}

	return nil
}
