package repair

import (
	"encoding/json"
	"testing"
)

// containsRepair reports whether tag appears in repairs.
func containsRepair(repairs []string, tag string) bool {
	for _, r := range repairs {
		if r == tag {
			return true
		}
	}
	return false
}

// decode unmarshals JSON bytes into a freshly allocated object map, failing
// the test on any error.
func decode(t *testing.T, b []byte) map[string]interface{} {
	t.Helper()
	var data map[string]interface{}
	if err := json.Unmarshal(b, &data); err != nil {
		t.Fatalf("unmarshal result: %v (output=%s)", err, b)
	}
	return data
}

// TestJSONArrayParse verifies that a stringified JSON array is decoded into a
// real array and that the json-array-parse repair tag is recorded.
func TestJSONArrayParse(t *testing.T) {
	input := []byte(`{"field": "[1, 2, 3]"}`)
	output, repairs := Repair(input)

	data := decode(t, output)
	arr, ok := data["field"].([]interface{})
	if !ok {
		t.Fatalf("expected field to be an array, got %T", data["field"])
	}
	if len(arr) != 3 {
		t.Fatalf("expected 3 elements, got %d", len(arr))
	}
	if !containsRepair(repairs, "json-array-parse:field") {
		t.Errorf("expected json-array-parse:field repair, got %v", repairs)
	}
}

// TestNullForOptional verifies that a JSON null field is deleted while a
// populated field is preserved, and that the null-for-optional tag is recorded.
func TestNullForOptional(t *testing.T) {
	input := []byte(`{"present": "x", "absent": null}`)
	output, repairs := Repair(input)

	data := decode(t, output)
	if _, ok := data["absent"]; ok {
		t.Errorf("expected absent field to be deleted, got %v", data["absent"])
	}
	if _, ok := data["present"]; !ok {
		t.Errorf("expected present field to remain")
	}
	if !containsRepair(repairs, "null-for-optional:absent") {
		t.Errorf("expected null-for-optional:absent repair, got %v", repairs)
	}
}

// TestBareStringWrap verifies that a bare string under an array-typed key is
// wrapped into a single-element array and that the bare-string-wrap tag is
// recorded.
func TestBareStringWrap(t *testing.T) {
	input := []byte(`{"args": "--help"}`)
	output, repairs := Repair(input)

	data := decode(t, output)
	arr, ok := data["args"].([]interface{})
	if !ok {
		t.Fatalf("expected args to be an array, got %T", data["args"])
	}
	if len(arr) != 1 {
		t.Fatalf("expected 1 element, got %d", len(arr))
	}
	if arr[0] != "--help" {
		t.Errorf("expected --help, got %v", arr[0])
	}
	if !containsRepair(repairs, "bare-string-wrap:args") {
		t.Errorf("expected bare-string-wrap:args repair, got %v", repairs)
	}
}

// TestMarkdownAutoLink verifies that a Markdown autolink where the link text
// matches the URL host is unwrapped into its bare URL form.
func TestMarkdownAutoLink(t *testing.T) {
	input := []byte(`{"link": "see [example.com](http://example.com) here"}`)
	output, repairs := Repair(input)

	data := decode(t, output)
	if data["link"] != "see example.com here" {
		t.Errorf("expected bare url, got %q", data["link"])
	}
	if !containsRepair(repairs, "markdown-auto-link:link") {
		t.Errorf("expected markdown-auto-link:link repair, got %v", repairs)
	}
}

// TestRepairOrdering proves that bare-string-wrap does NOT run before
// json-array-parse. When "args" holds a stringified array, json-array-parse
// must run first and decode it; bare-string-wrap must then leave the now-real
// array untouched rather than wrapping the still-string value into
// ["[1, 2, 3]"].
func TestRepairOrdering(t *testing.T) {
	input := []byte(`{"args": "[1, 2, 3]"}`)
	output, repairs := Repair(input)

	data := decode(t, output)
	arr, ok := data["args"].([]interface{})
	if !ok {
		t.Fatalf("expected args to be an array, got %T", data["args"])
	}
	if len(arr) != 3 {
		t.Fatalf("expected 3 elements, got %d (bare-string-wrap likely ran first)", len(arr))
	}
	// If bare-string-wrap ran first, args would be a 1-element array holding
	// the original stringified array.
	if s, isStr := arr[0].(string); isStr && s == "[1, 2, 3]" {
		t.Errorf("args was double-wrapped as %v; json-array-parse did not run first", arr)
	}
	if !containsRepair(repairs, "json-array-parse:args") {
		t.Errorf("expected json-array-parse:args repair, got %v", repairs)
	}
}

// TestRepairNonObjectInput verifies that non-object input (e.g. an array or a
// scalar) is returned unchanged with no repairs recorded.
func TestRepairNonObjectInput(t *testing.T) {
	input := []byte(`[1, 2, 3]`)
	output, repairs := Repair(input)
	if string(output) != string(input) {
		t.Errorf("expected unchanged output, got %s", output)
	}
	if len(repairs) != 0 {
		t.Errorf("expected no repairs, got %v", repairs)
	}
}
