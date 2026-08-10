package issue

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

func TestReadBodyValid(t *testing.T) {
	if _, err := exec.LookPath("gh"); err != nil {
		t.Skip("gh not installed")
	}
	// Integration test: only runs when gh is available and authenticated.
	// Uses a known open issue to validate the reader works.
	body, labels, err := ReadBody(1)
	if err != nil {
		t.Skipf("gh issue view failed (likely no auth): %v", err)
	}
	if body == "" {
		t.Error("expected non-empty body for issue #1")
	}
	// Labels may be empty, that's valid.
	_ = labels
}

func TestReadBodyGhostNotFound(t *testing.T) {
	origPath := os.Getenv("PATH")
	defer os.Setenv("PATH", origPath)

	// Use empty PATH to make gh unfindable
	os.Setenv("PATH", t.TempDir())

	_, _, err := ReadBody(1)
	if err == nil {
		t.Fatal("expected error when gh is not in PATH")
	}
	if err.Error() != "gh CLI not found — install github.com/cli/cli" {
		t.Errorf("expected gh-not-found message, got: %v", err)
	}
}

func TestReadBodyFakeGh(t *testing.T) {
	dir := t.TempDir()
	origPath := os.Getenv("PATH")
	defer os.Setenv("PATH", origPath)

	// Create a fake gh script that outputs valid JSON
	fakeGh := filepath.Join(dir, "gh")
	if runtime.GOOS == "windows" {
		fakeGh += ".bat"
	}
	script := `#!/bin/sh
echo '{"body":"Fix the bug with login","labels":[{"name":"stage:produce"},{"name":"bug"},{"name":"priority:high"}]}'
`
	if err := os.WriteFile(fakeGh, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	os.Setenv("PATH", dir)

	body, labels, err := ReadBody(42)
	if err != nil {
		t.Fatalf("ReadBody returned error: %v", err)
	}
	if body != "Fix the bug with login" {
		t.Errorf("expected body 'Fix the bug with login', got %q", body)
	}
	if len(labels) != 3 {
		t.Fatalf("expected 3 labels, got %d: %v", len(labels), labels)
	}
	expected := []string{"stage:produce", "bug", "priority:high"}
	for i, want := range expected {
		if labels[i] != want {
			t.Errorf("labels[%d] = %q, want %q", i, labels[i], want)
		}
	}
}

func TestReadBodyFakeGhEmptyBody(t *testing.T) {
	dir := t.TempDir()
	origPath := os.Getenv("PATH")
	defer os.Setenv("PATH", origPath)

	fakeGh := filepath.Join(dir, "gh")
	if runtime.GOOS == "windows" {
		fakeGh += ".bat"
	}
	if err := os.WriteFile(fakeGh, []byte("#!/bin/sh\necho '{\"body\":\"\",\"labels\":[]}'\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	os.Setenv("PATH", dir)

	body, labels, err := ReadBody(99)
	if err != nil {
		t.Fatalf("ReadBody returned error: %v", err)
	}
	if body != "" {
		t.Errorf("expected empty body, got %q", body)
	}
	if len(labels) != 0 {
		t.Errorf("expected 0 labels, got %d", len(labels))
	}
}

func TestReadBodyFakeGhFailure(t *testing.T) {
	dir := t.TempDir()
	origPath := os.Getenv("PATH")
	defer os.Setenv("PATH", origPath)

	fakeGh := filepath.Join(dir, "gh")
	if runtime.GOOS == "windows" {
		fakeGh += ".bat"
	}
	if err := os.WriteFile(fakeGh, []byte("#!/bin/sh\necho 'issue not found' >&2\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	os.Setenv("PATH", dir)

	_, _, err := ReadBody(999999)
	if err == nil {
		t.Fatal("expected error when gh exits non-zero")
	}
}

func TestStageLabel(t *testing.T) {
	tests := []struct {
		name   string
		labels []string
		want   string
	}{
		{"single stage", []string{"stage:produce", "bug"}, "stage:produce"},
		{"no stage", []string{"bug", "feature"}, ""},
		{"multiple stages takes first", []string{"stage:review", "stage:produce"}, "stage:review"},
		{"empty", nil, ""},
		{"stage:implement", []string{"stage:implement"}, "stage:implement"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := StageLabel(tc.labels)
			if got != tc.want {
				t.Errorf("StageLabel(%v) = %q, want %q", tc.labels, got, tc.want)
			}
		})
	}
}

func TestExtractCheckboxCriteria(t *testing.T) {
	body := "- [ ] do thing\n- [x] did thing\n- [ ] other\n"
	got := ExtractAcceptanceCriteria(body)
	if len(got) != 3 {
		t.Fatalf("expected 3 criteria, got %d: %v", len(got), got)
	}
	if got[0] != "do thing" {
		t.Errorf("got[0] = %q, want %q", got[0], "do thing")
	}
	if got[1] != "did thing" {
		t.Errorf("got[1] = %q, want %q", got[1], "did thing")
	}
	if got[2] != "other" {
		t.Errorf("got[2] = %q, want %q", got[2], "other")
	}
}

func TestExtractNumberedBoldCriteria(t *testing.T) {
	body := "1. **Do X**\n2. **Do Y**\n"
	got := ExtractAcceptanceCriteria(body)
	if len(got) != 2 {
		t.Fatalf("expected 2 criteria, got %d: %v", len(got), got)
	}
	if got[0] != "Do X" {
		t.Errorf("got[0] = %q, want %q", got[0], "Do X")
	}
	if got[1] != "Do Y" {
		t.Errorf("got[1] = %q, want %q", got[1], "Do Y")
	}
}

func TestExtractSectionCriteria(t *testing.T) {
	body := "## Acceptance Criteria\n- item one\n- item two\n## Next Section\n- ignored\n"
	got := ExtractAcceptanceCriteria(body)
	if len(got) != 2 {
		t.Fatalf("expected 2 criteria, got %d: %v", len(got), got)
	}
	if got[0] != "item one" {
		t.Errorf("got[0] = %q, want %q", got[0], "item one")
	}
	if got[1] != "item two" {
		t.Errorf("got[1] = %q, want %q", got[1], "item two")
	}
}

func TestExtractNoCriteria(t *testing.T) {
	got := ExtractAcceptanceCriteria("just some text\nno criteria here")
	if got != nil {
		t.Errorf("expected nil, got %v", got)
	}
}

func TestExtractDeduplication(t *testing.T) {
	body := "## Acceptance Criteria\n- do thing\n- other\n\n- [ ] do thing\n- [x] already done\n"
	got := ExtractAcceptanceCriteria(body)
	if len(got) != 3 {
		t.Fatalf("expected 3 unique criteria, got %d: %v", len(got), got)
	}
	if got[0] != "do thing" {
		t.Errorf("got[0] = %q, want %q", got[0], "do thing")
	}
	if got[1] != "other" {
		t.Errorf("got[1] = %q, want %q", got[1], "other")
	}
	if got[2] != "already done" {
		t.Errorf("got[2] = %q, want %q", got[2], "already done")
	}
}

func TestExtractEmptyBody(t *testing.T) {
	got := ExtractAcceptanceCriteria("")
	if got != nil {
		t.Errorf("expected nil for empty body, got %v", got)
	}
}

func TestExtractMixedPatterns(t *testing.T) {
	body := `# Issue Title

Some description.

## Acceptance Criteria

1. First numbered item
2. Second item

- [ ] checkbox item
- [x] completed item

3. **Bold criteria**
`
	got := ExtractAcceptanceCriteria(body)
	if len(got) != 5 {
		t.Fatalf("expected 5 criteria, got %d: %v", len(got), got)
	}
}

func TestExtractCaseInsensitiveSectionHeader(t *testing.T) {
	body := "### Acceptance criteria\n- some item\n"
	got := ExtractAcceptanceCriteria(body)
	if len(got) != 1 {
		t.Fatalf("expected 1 criterion, got %d: %v", len(got), got)
	}
	if got[0] != "some item" {
		t.Errorf("got[0] = %q, want %q", got[0], "some item")
	}
}

func TestExtractCapitalizedCheckbox(t *testing.T) {
	body := "- [X] Important task\n"
	got := ExtractAcceptanceCriteria(body)
	if len(got) != 1 {
		t.Fatalf("expected 1 criterion, got %d: %v", len(got), got)
	}
	if got[0] != "Important task" {
		t.Errorf("got[0] = %q, want %q", got[0], "Important task")
	}
}
