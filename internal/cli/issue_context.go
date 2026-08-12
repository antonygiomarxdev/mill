package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/antonygiomarxdev/mill/internal/adapter"
	"github.com/antonygiomarxdev/mill/internal/issue"
	"github.com/antonygiomarxdev/mill/internal/role"
)

// resolveRepoRef returns "owner/repo" from the git remote.
// Returns "OWNER/REPO" as placeholder if git is unavailable.
func resolveRepoRef() string {
	cmd := exec.Command("git", "remote", "get-url", "origin")
	out, err := cmd.Output()
	if err != nil {
		return "OWNER/REPO"
	}
	url := strings.TrimSpace(string(out))
	url = strings.TrimSuffix(url, ".git")
	if idx := strings.LastIndex(url, ":"); idx >= 0 {
		return url[idx+1:]
	}
	if idx := strings.LastIndex(url, "/"); idx >= 0 {
		beforeSlash := url[:idx]
		if idx2 := strings.LastIndex(beforeSlash, "/"); idx2 >= 0 {
			return url[idx2+1:]
		}
	}
	return "OWNER/REPO"
}

// buildIssueContextPrompt constructs a role-aware prompt for a given issue and target role.
// The prompt includes the full issue body, extracted acceptance criteria, and
func buildIssueContextPrompt(issueNum int, body string, ac []string, targetRole string, caps adapter.Capabilities) string {
	root, err := projectRoot()
	if err != nil {
		root = "."
	}

	// Extract title from first # heading in body
	title := ""
	for _, line := range strings.Split(body, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "# ") {
			title = strings.TrimSpace(strings.TrimPrefix(trimmed, "# "))
			break
		}
	}

	var parts []string

	// Header: "Issue #N" with optional title
	header := fmt.Sprintf("# Issue #%d", issueNum)
	if title != "" {
		header += ": " + title
	}
	parts = append(parts, header)

	// Issue body
	if body != "" {
		parts = append(parts, "", body)
	}

	// Acceptance Criteria
	if len(ac) > 0 {
		var acLines []string
		acLines = append(acLines, "", "## Acceptance Criteria")
		for i, c := range ac {
			acLines = append(acLines, fmt.Sprintf("%d. %s", i+1, c))
		}
		parts = append(parts, strings.Join(acLines, "\n"))
	}

	// Spec Reference — only include if the file exists
	specPath := filepath.Join(root, ".mill", "phases", fmt.Sprintf("%d", issueNum), "spec.md")
	if _, err := os.Stat(specPath); err == nil {
		parts = append(parts, "", "## Spec Reference",
			fmt.Sprintf(".mill/phases/%d/spec.md (read this file for architecture details)", issueNum))
	}

	// Role
	rolePrompt, roleErr := role.LoadFrom(root, targetRole)
	if roleErr != nil {
		parts = append(parts, "", "## Role",
			fmt.Sprintf("You are: %s. Read .mill/roles/%s/ROLE.md before acting.", targetRole, targetRole))
	} else {
		parts = append(parts, "", "## Role", rolePrompt)
	}

	// Read Tool Capabilities (when non-zero)
	if caps.ReadTool != (adapter.ReadToolCapabilities{}) {
		parts = append(parts, "", "## Read Tool Capabilities", buildReadToolCapSection(caps.ReadTool))
	}
	return strings.Join(parts, "\n")
}

// buildReadToolCapSection builds the capabilities prompt section.
// When caps is non-zero, it produces guidance for the agent about
// the read tool's limits and features.
func buildReadToolCapSection(caps adapter.ReadToolCapabilities) string {
	var lines []string

	lines = append(lines, "Your harness provides a read tool with the following features:")
	lines = append(lines, "")

	if caps.LineCeiling > 0 {
		lines = append(lines, fmt.Sprintf("- **Line ceiling:** %d lines per read. Files longer than this are truncated.", caps.LineCeiling))
	}
	if caps.ByteCeiling > 0 {
		kb := caps.ByteCeiling / 1024
		if caps.ByteCeiling%1024 != 0 {
			lines = append(lines, fmt.Sprintf("- **Byte ceiling:** %d bytes per read.", caps.ByteCeiling))
		} else {
			lines = append(lines, fmt.Sprintf("- **Byte ceiling:** %dKB per read.", kb))
		}
	}
	if caps.CharCeiling > 0 {
		lines = append(lines, fmt.Sprintf("- **Char ceiling:** %d chars per displayed line. Longer lines are truncated mid-display.", caps.CharCeiling))
	}
	if caps.HasSelectorSupport {
		lines = append(lines, "- **Selectors:** Supported. Append `:<range>` to file paths to read specific portions:")
		lines = append(lines, "  - `path:50-200` — lines 50 through 200 (inclusive)")
		lines = append(lines, "  - `path:50` — line 50 only")
		lines = append(lines, "  - `path:50-` — from line 50 to end")
		lines = append(lines, "  - `path:50+150` — 150 lines starting at line 50")
		lines = append(lines, "  - `path:raw` — verbatim output (no line numbers)")
		lines = append(lines, "  - `path:50-200:raw` — combined selector + raw mode")
	}
	if caps.HasRecoveryNotes {
		lines = append(lines, "- **Recovery notes:** YES — when output is truncated, the harness appends a note like")
		lines = append(lines, "  `[TRUNCATED: 1200 lines omitted — re-read with a narrower selector]`.")
		lines = append(lines, "  If you see a recovery note, narrow your selector range and re-read.")
	}

	lines = append(lines, "")
	lines = append(lines, "**Guidance:**")
	if caps.HasSelectorSupport {
		lines = append(lines, "- Prefer reading specific ranges with selectors over whole-file reads.")
	}
	if caps.HasRecoveryNotes {
		lines = append(lines, "- When recovery notes indicate truncation, re-read with a narrower selector.")
	}
	if !caps.HasSelectorSupport && (caps.LineCeiling > 0 || caps.ByteCeiling > 0) {
		lines = append(lines, "- Be aware that large files may be truncated. If a file appears incomplete, focus on specific sections.")
	}
	lines = append(lines, "- Re-read a file before writing it to ensure you have the latest version.")

	return strings.Join(lines, "\n")
}

// readIssueWithFallback reads an issue body and returns body, labels, and extracted
// acceptance criteria. On read failure, returns a degraded prompt instead of erroring.
// This is used by runDelegate to avoid blocking delegation when gh is unavailable.
func readIssueWithFallback(reader func(int) (string, []string, error), issueNum int) (body string, labels []string, ac []string) {
	body, labels, readErr := reader(issueNum)
	if readErr != nil {
		repoRef := resolveRepoRef()
		body = fmt.Sprintf("# Issue #%d\n\n\u26a0 Issue body could not be read: %v\nProceeding with limited context. Read the issue at: https://github.com/%s/issues/%d", issueNum, readErr, repoRef, issueNum)
		labels = nil
	}
	ac = issue.ExtractAcceptanceCriteria(body)
	return body, labels, ac
}
