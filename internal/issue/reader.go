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

// stageLabel returns the first stage:* label found, or empty string.
// If multiple stage:* labels exist, only the first is used.
func StageLabel(labels []string) string {
	for _, l := range labels {
		if strings.HasPrefix(l, "stage:") {
			return l
		}
	}
	return ""
}
