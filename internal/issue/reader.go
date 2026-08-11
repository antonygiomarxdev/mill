package issue

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// ReadBody reads the body and labels of a GitHub issue via `gh issue view`.
// Returns the body text and a list of label names.
// Returns an error if gh is not found or the issue cannot be read.
func ReadBody(issueNum int) (body string, labels []string, err error) {
	if _, err := exec.LookPath("gh"); err != nil {
		return "", nil, fmt.Errorf("gh CLI not found — install github.com/cli/cli")
	}

	args := []string{
		"issue", "view", strconv.Itoa(issueNum),
		"--json", "body,labels",
	}

	cmd := exec.Command("gh", args...)
	out, err := cmd.Output()
	if err != nil {
		return "", nil, fmt.Errorf("gh issue view %d: %w", issueNum, err)
	}

	var result struct {
		Body   string `json:"body"`
		Labels []struct {
			Name string `json:"name"`
		} `json:"labels"`
	}
	if err := json.Unmarshal(out, &result); err != nil {
		return "", nil, fmt.Errorf("parse gh output: %w", err)
	}

	for _, l := range result.Labels {
		labels = append(labels, l.Name)
	}

	return result.Body, labels, nil
}

// AddLabel adds a label to a GitHub issue via `gh issue edit --add-label`.
// Returns an error if gh is not found or the command fails.
func AddLabel(issueNum int, label string) error {
	if _, err := exec.LookPath("gh"); err != nil {
		return fmt.Errorf("gh CLI not found — install github.com/cli/cli")
	}

	args := []string{
		"issue", "edit",
		strconv.Itoa(issueNum),
		"--add-label", label,
	}

	cmd := exec.Command("gh", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("gh issue edit %d --add-label %s: %w\n%s", issueNum, label, err, out)
	}
	return nil
}

// StageLabel returns the first stage:* label found, or empty string.
// If multiple stage:* labels exist, only the first is used.
func StageLabel(labels []string) string {
	for _, l := range labels {
		if strings.HasPrefix(l, "stage:") {
			return l
		}
	}
	return ""
}

// ExtractAcceptanceCriteria scans an issue body for acceptance criteria.
// It detects three patterns:
//  1. Checkbox lists: "- [ ]" or "- [x]" items
//  2. Numbered bold criteria: "1. **Label**"
//  3. Section headers: "## Acceptance Criteria" / "## Acceptance criteria"
//     followed by list items until the next heading
//
// Returns a deduplicated, ordered list of criterion strings.
// Returns nil if no criteria matched.
func ExtractAcceptanceCriteria(body string) []string {
	if body == "" {
		return nil
	}

	var criteria []string
	seen := make(map[string]bool)
	lines := strings.Split(body, "\n")
	inSection := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		// Track section headers: "## Acceptance Criteria" (case-insensitive)
		if strings.HasPrefix(trimmed, "#") && !strings.HasPrefix(trimmed, "## ") && !strings.HasPrefix(trimmed, "### ") {
			continue
		}
		if strings.HasPrefix(trimmed, "## ") || strings.HasPrefix(trimmed, "### ") {
			heading := strings.TrimPrefix(trimmed, "## ")
			heading = strings.TrimPrefix(heading, "### ")
			heading = strings.TrimSpace(heading)
			if strings.EqualFold(heading, "Acceptance Criteria") {
				inSection = true
				continue
			} else {
				inSection = false
				continue
			}
		}

		// If we're in an acceptance criteria section, collect list items
		if inSection {
			if strings.HasPrefix(trimmed, "- [ ] ") || strings.HasPrefix(trimmed, "- [x] ") || strings.HasPrefix(trimmed, "- [X] ") {
				item := trimmed[6:]
				if item != "" && !seen[item] {
					seen[item] = true
					criteria = append(criteria, item)
				}
			} else if strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "* ") {
				item := strings.TrimPrefix(trimmed, "- ")
				item = strings.TrimPrefix(item, "* ")
				item = strings.TrimSpace(item)
				if item != "" && !seen[item] {
					seen[item] = true
					criteria = append(criteria, item)
				}
			} else if len(trimmed) > 0 && trimmed[0] >= '0' && trimmed[0] <= '9' {
				if idx := strings.Index(trimmed, ". "); idx > 0 {
					item := strings.TrimSpace(trimmed[idx+2:])
					if item != "" && !seen[item] {
						seen[item] = true
						criteria = append(criteria, item)
					}
				}
			}
			continue
		}

		// Pattern 1: Checkbox lists
		if strings.HasPrefix(trimmed, "- [ ] ") || strings.HasPrefix(trimmed, "- [x] ") || strings.HasPrefix(trimmed, "- [X] ") {
			item := trimmed[6:]
			if item != "" && !seen[item] {
				seen[item] = true
				criteria = append(criteria, item)
			}
			continue
		}

		// Pattern 2: Numbered bold criteria: "1. **Label**"
		if len(trimmed) > 0 && trimmed[0] >= '0' && trimmed[0] <= '9' {
			if idx := strings.Index(trimmed, ". **"); idx > 0 {
				rest := trimmed[idx+4:]
				if end := strings.Index(rest, "**"); end >= 0 {
					item := strings.TrimSpace(rest[:end])
					if item != "" && !seen[item] {
						seen[item] = true
						criteria = append(criteria, item)
					}
				}
			}
		}
	}

	return criteria
}
