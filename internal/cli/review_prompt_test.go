package cli

import (
	"strings"
	"testing"
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
				"APPROVED:",
				"CHANGES_REQUESTED:",
				"BLOCKED:",
				"Quality rules",
				"Every CHANGES_REQUESTED item MUST reference",
				"No vague feedback",
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
				"APPROVED:",
				"CHANGES_REQUESTED:",
				"BLOCKED:",
			},
		},
		{
			name:               "empty criteria slice",
			issueBody:          "Fix the bug",
			diffOutput:         "diff output here",
			acceptanceCriteria: []string{},
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
		{
			name:               "long diff",
			issueBody:          "Issue text",
			diffOutput:         strings.Repeat("x", 10000),
			acceptanceCriteria: []string{"AC1"},
			checks: []string{
				strings.Repeat("x", 10000),
			},
		},
		{
			name:               "all three verdict templates present",
			issueBody:          "Test",
			diffOutput:         "diff",
			acceptanceCriteria: []string{"AC1"},
			checks: []string{
				"- APPROVED: (work meets all acceptance criteria)",
				"- CHANGES_REQUESTED: (numbered, specific, criteria-referencing feedback items)",
				"- BLOCKED: (cannot proceed — missing info or external dependency)",
			},
		},
		{
			name:               "quality rules included",
			issueBody:          "Test",
			diffOutput:         "diff",
			acceptanceCriteria: []string{"AC1"},
			checks: []string{
				"Quality rules:",
				"Every CHANGES_REQUESTED item MUST reference a specific acceptance criterion",
				"No vague feedback like",
				"If all criteria met, MUST output APPROVED",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildReviewPrompt54(tt.issueBody, tt.diffOutput, tt.acceptanceCriteria)
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
			name: "single criterion",
			issueBody: `## Acceptance Criteria
- [ ] Implement the login page`,
			want: []string{"Implement the login page"},
		},
		{
			name: "multiple criteria",
			issueBody: `## Task
- [ ] Add login page
- [ ] Add error handling
- [ ] Write tests`,
			want: []string{"Add login page", "Add error handling", "Write tests"},
		},
		{
			name: "empty checklist items skipped",
			issueBody: `- [ ] 
- [ ] Real task
- [ ] `,
			want: []string{"Real task"},
		},
		{
			name:      "empty body",
			issueBody: "",
			want:      nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractAcceptanceCriteria(tt.issueBody)
			if tt.want == nil && got != nil {
				t.Errorf("expected nil, got %v", got)
				return
			}
			if tt.want == nil && got == nil {
				return
			}
			if len(got) != len(tt.want) {
				t.Errorf("got %d criteria, want %d: %v", len(got), len(tt.want), got)
				return
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("criteria[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}
