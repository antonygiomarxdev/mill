package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestRunMain(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		wantExit  int
		stdoutHas string
		stderrHas string
	}{
		{
			name:      "no args shows usage and exits non-zero",
			args:      nil,
			wantExit:  1,
			stderrHas: "Usage",
		},
		{
			name:      "status shows table header",
			args:      []string{"status"},
			wantExit:  0,
			stdoutHas: "VERDICT",
		},
		{
			name:      "delegate without issue shows usage",
			args:      []string{"delegate"},
			wantExit:  1,
			stderrHas: "Usage",
		},
		{
			name:      "land without target shows usage",
			args:      []string{"land"},
			wantExit:  1,
			stderrHas: "Usage",
		},
		{
			name:      "unknown command",
			args:      []string{"unknown"},
			wantExit:  1,
			stderrHas: "unknown command",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			got := runMain(tt.args, &stdout, &stderr)

			if got != tt.wantExit {
				t.Errorf("runMain exit = %d, want %d", got, tt.wantExit)
			}
			if tt.stdoutHas != "" && !strings.Contains(stdout.String(), tt.stdoutHas) {
				t.Errorf("stdout missing %q\nstdout:\n%s", tt.stdoutHas, stdout.String())
			}
			if tt.stderrHas != "" && !strings.Contains(strings.ToLower(stderr.String()), strings.ToLower(tt.stderrHas)) {
				t.Errorf("stderr missing %q\nstderr:\n%s", tt.stderrHas, stderr.String())
			}
		})
	}
}
