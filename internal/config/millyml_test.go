package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadAndValidate(t *testing.T) {
	tests := []struct {
		name    string
		yml     string
		wantErr string
	}{
		{
			name: "valid mill.yml",
			yml: `project: mill
provider: commandcode
`,
			wantErr: "",
		},
		{
			name: "valid with extra fields ignored",
			yml: `project: mill
provider: commandcode
model: laguna-free
max-rounds: 4
`,
			wantErr: "",
		},
		{
			name:    "invalid YAML syntax",
			yml:     "project: mill\nprovider: [unclosed\n",
			wantErr: "invalid YAML",
		},
		{
			name: "missing project field",
			yml: `provider: commandcode
`,
			wantErr: "missing required field 'project'",
		},
		{
			name: "missing provider field",
			yml: `project: mill
`,
			wantErr: "missing required field 'provider'",
		},
		{
			name: "empty project value",
			yml: `project:
provider: commandcode
`,
			wantErr: "missing required field 'project'",
		},
		{
			name: "empty provider value",
			yml: `project: mill
provider:
`,
			wantErr: "missing required field 'provider'",
		},
		{
			name:    "file not found",
			yml:     "",
			wantErr: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var path string
			if tt.name == "file not found" {
				path = filepath.Join(t.TempDir(), "nonexistent.yml")
			} else {
				path = filepath.Join(t.TempDir(), "mill.yml")
				if err := os.WriteFile(path, []byte(tt.yml), 0o644); err != nil {
					t.Fatal(err)
				}
			}

			cfg, err := LoadAndValidate(path)

			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if cfg == nil {
					t.Fatal("expected non-nil config")
				}
				// file not found returns empty defaults — populated fields not required
				if tt.name != "file not found" && (cfg.Project == "" || cfg.Provider == "") {
					t.Fatalf("expected project and provider to be populated, got project=%q provider=%q", cfg.Project, cfg.Provider)
				}
				return
			}

			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("expected error containing %q, got: %v", tt.wantErr, err)
			}
		})
	}
}

func TestLoadAndValidateLineNumberInError(t *testing.T) {
	// yaml.v3 parse errors include line numbers. Verify the error message
	// contains a line number for invalid YAML.
	yml := "project: mill\n  bad: [indent\n"
	path := filepath.Join(t.TempDir(), "mill.yml")
	if err := os.WriteFile(path, []byte(yml), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadAndValidate(path)
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}

	errStr := err.Error()
	if !strings.Contains(errStr, "line") {
		t.Fatalf("expected error to contain 'line' (yaml.v3 includes line numbers), got: %v", errStr)
	}
}
