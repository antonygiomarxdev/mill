package issue

import (
	"testing"
)

func TestParseValid(t *testing.T) {
	tests := []struct {
		input    string
		expected int
	}{
		{"390", 390},
		{"1", 1},
		{"12345", 12345},
	}

	for _, tc := range tests {
		got, err := Parse(tc.input)
		if err != nil {
			t.Errorf("Parse(%q) returned error: %v", tc.input, err)
			continue
		}
		if got != tc.expected {
			t.Errorf("Parse(%q) = %d, want %d", tc.input, got, tc.expected)
		}
	}
}

func TestParseNegative(t *testing.T) {
	_, err := Parse("-1")
	if err == nil {
		t.Error("Parse(\"-1\") should return error")
	}
}

func TestParseZero(t *testing.T) {
	_, err := Parse("0")
	if err == nil {
		t.Error("Parse(\"0\") should return error")
	}
}

func TestParseNonNumeric(t *testing.T) {
	tests := []string{"abc", "", "12abc", "issue 390"}

	for _, tc := range tests {
		_, err := Parse(tc)
		if err == nil {
			t.Errorf("Parse(%q) should return error", tc)
		}
	}
}

func TestMustParseValid(t *testing.T) {
	got := MustParse("390")
	if got != 390 {
		t.Errorf("MustParse(\"390\") = %d, want 390", got)
	}
}

func TestMustParsePanicsOnInvalid(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("MustParse should panic on invalid input")
		}
	}()

	MustParse("abc")
}

func TestMustParsePanicsOnNegative(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("MustParse should panic on negative input")
		}
	}()

	MustParse("-1")
}

func TestMustParsePanicsOnZero(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("MustParse should panic on zero input")
		}
	}()

	MustParse("0")
}
