package cli

import (
	"strings"
	"testing"

	"github.com/antonygiomarxdev/mill/internal/adapter"
)

func TestBuildReviewPrompt54_Structure(t *testing.T) {
	tests := []struct {
		name               string
		issueBody          string
		diffOutput         string
		acceptanceCriteria []string
		checks             []string
	}{
		{
			name:               "all fields populated",
			issueBody:          "As a user, I want to log in",
			diffOutput:         "diff --git a/login.go b/login.go",
			acceptanceCriteria: []string{"User can authenticate", "Error message on failure"},
			checks: []string{
				"You are a code reviewer",
				"## Issue",
				"As a user, I want to log in",
				"## Acceptance Criteria",
				"- User can authenticate",
				"- Error message on failure",
				"## Changes (diff)",
				"diff --git a/login.go b/login.go",
				"## Build Results (go build ./...)",
				"## Test Results (go test ./...)",
				"If all criteria are met:",
				"If changes are needed:",
				"If blocked by external dependency:",
			},
		},
		{
			name:               "nil criteria",
			issueBody:          "Fix the bug",
			diffOutput:         "diff output here",
			acceptanceCriteria: nil,
			checks: []string{
				"## Acceptance Criteria",
				"(no acceptance criteria provided",
			},
		},
		{
			name:               "empty issue body",
			issueBody:          "",
			diffOutput:         "some diff",
			acceptanceCriteria: []string{"Criterion 1"},
			checks: []string{
				"## Issue",
				"(no issue body provided)",
				"- Criterion 1",
			},
		},
		{
			name:               "empty diff",
			issueBody:          "Issue text",
			diffOutput:         "",
			acceptanceCriteria: []string{"Criterion 1"},
			checks: []string{
				"## Changes (diff)",
				"(no diff available)",
				"Issue text",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildReviewPrompt54(tt.issueBody, tt.diffOutput, "", "", tt.acceptanceCriteria, adapter.Capabilities{})
			for _, check := range tt.checks {
				if !strings.Contains(result, check) {
					t.Errorf("expected output to contain %q", check)
				}
			}
		})
	}
}

func TestExtractAcceptanceCriteria_54(t *testing.T) {
	tests := []struct {
		name      string
		issueBody string
		want      []string
	}{
		{
			name:      "no criteria",
			issueBody: "Just a plain issue body with no checklists.",
			want:      nil,
		},
		{
			name:      "single criterion",
			issueBody: "- [ ] User can authenticate",
			want:      []string{"User can authenticate"},
		},
		{
			name:      "multiple criteria",
			issueBody: "- [ ] Login works\n- [ ] Logout works\n- [ ] Error shown",
			want:      []string{"Login works", "Logout works", "Error shown"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractAcceptanceCriteria(tt.issueBody)
			if len(got) != len(tt.want) {
				t.Fatalf("expected %d criteria, got %d: %v", len(tt.want), len(got), got)
			}
			for i := range tt.want {
				if got[i] != tt.want[i] {
					t.Errorf("criteria[%d]: expected %q, got %q", i, tt.want[i], got[i])
				}
			}
		})
	}
}
