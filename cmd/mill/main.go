// Command mill is the entry point for the mill agent delegation harness.
// It performs flag parsing only and delegates to the cli package.
package main

import (
	"fmt"
	"io"
	"os"

	"github.com/antonygiomarxdev/mill/internal/cli"
)

func runMain(args []string, stdout, stderr io.Writer) int {
	app := cli.NewApp()
	app.Out = stdout
	app.Err = stderr
	if err := app.Run(args...); err != nil {
		fmt.Fprintf(stderr, "mill: %v\n", err)
		return 1
	}
	if len(args) == 0 {
		return 1
	}
	return 0
}

func main() {
	os.Exit(runMain(os.Args[1:], os.Stdout, os.Stderr))
}
