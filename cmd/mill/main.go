// Command mill is the entry point for the mill agent delegation harness.
// It performs flag parsing only and delegates to the cli package.
package main

import (
	"fmt"
	"os"

	"github.com/antonygiomarxdev/mill/internal/cli"
)

func main() {
	app := cli.NewApp()
	if err := app.Run(os.Args[1:]...); err != nil {
		fmt.Fprintf(os.Stderr, "mill: %v\n", err)
		os.Exit(1)
	}
}
