package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

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
// a reference to the spec file when it exists.
func buildIssueContextPrompt(issueNum int, body string, ac []string, targetRole string) string {
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

	return strings.Join(parts, "\n")
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
