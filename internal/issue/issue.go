// Package issue provides parsing of GitHub issue numbers from CLI arguments.
package issue

import (
	"fmt"
	"strconv"
)

// Parse parses an issue number from a string. The input must be a positive
// integer. Returns an error otherwise.
func Parse(s string) (int, error) {
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, fmt.Errorf("invalid issue number %q: %w", s, err)
	}
	if n <= 0 {
		return 0, fmt.Errorf("issue number must be positive, got %d", n)
	}
	return n, nil
}

// MustParse parses an issue number and panics on error.
// Intended for internal use where the input is already validated.
func MustParse(s string) int {
	n, err := Parse(s)
	if err != nil {
		panic(err)
	}
	return n
}
