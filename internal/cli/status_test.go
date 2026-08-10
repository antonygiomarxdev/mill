package cli

import (
	"bytes"
	"testing"

	"github.com/spf13/cobra"
)

func executeCommand(root *cobra.Command, args ...string) (string, error) {
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs(args)
	err := root.Execute()
	return buf.String(), err
}

func TestStatusPrintsHeader(t *testing.T) {
	output, err := executeCommand(rootCmd, "status")
	if err != nil {
		t.Fatalf("status returned error: %v", err)
	}

	expected := "ID  ISSUE  STATUS  COMMITS  VERDICT"
	if !contains(output, expected) {
		t.Errorf("expected output to contain header %q\n got: %q", expected, output)
	}
}

func TestStatusExitsZero(t *testing.T) {
	_, err := executeCommand(rootCmd, "status")
	if err != nil {
		t.Errorf("status should exit zero, got error: %v", err)
	}
}

func contains(s, substr string) bool {
	return bytes.Contains([]byte(s), []byte(substr))
}
