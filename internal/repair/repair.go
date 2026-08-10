// Package repair fixes malformed tool-call arguments that LLM-based agents
// emit when their output deviates from the schema a provider expects.
//
// Repair operates on a JSON object map and applies a pipeline of targeted
// fixes. Each fix records a tag of the form "<pattern>:<key>" so callers can
// report exactly what was corrected. The pipeline runs in a fixed order so
// that earlier, more specific repairs take precedence over later, broader
// ones (e.g. a stringified JSON array passed through json-array-parse before
// bare-string-wrap gets a chance to wrap the still-string value).
package repair

import (
	"encoding/json"
	"regexp"
)

// Repair inspects the JSON object in input and applies the repair pipeline,
// returning the (possibly rewritten) JSON and a list of repair tags that
// describe every change made. If input is not a JSON object, or the repaired
// result cannot be re-marshaled, the original bytes are returned unchanged.
func Repair(input []byte) ([]byte, []string) {
	var repairs []string
	var data map[string]interface{}
	if err := json.Unmarshal(input, &data); err != nil {
		return input, repairs
	}
	data, r := repairJSONArrayParse(data)
	repairs = append(repairs, r...)
	data, r = repairMarkdownAutoLink(data)
	repairs = append(repairs, r...)
	data, r = repairNullForOptional(data)
	repairs = append(repairs, r...)
	data, r = repairBareStringWrap(data)
	repairs = append(repairs, r...)
	output, err := json.Marshal(data)
	if err != nil {
		return input, repairs
	}
	return output, repairs
}

// repairJSONArrayParse converts string fields whose value is a stringified JSON
// array (e.g. `"[1, 2, 3]"`) into a real JSON array. This must run before
// bare-string-wrap: a stringified array on an array-typed field would otherwise
// be wrapped as `["[1, 2, 3]"]` instead of being decoded.
func repairJSONArrayParse(data map[string]interface{}) (map[string]interface{}, []string) {
	var repairs []string
	for key, val := range data {
		s, ok := val.(string)
		if !ok || len(s) < 2 || s[0] != '[' {
			continue
		}
		var arr []interface{}
		if err := json.Unmarshal([]byte(s), &arr); err != nil {
			continue
		}
		data[key] = arr
		repairs = append(repairs, "json-array-parse:"+key)
	}
	return data, repairs
}

// mdAutoLink matches Markdown autolinks of the form [text](http://text) or
// [text](https://text) where the link text equals the URL host (the portion
// after the protocol). Go's regexp engine (RE2) does not support
// backreferences, so equality is verified in code via the replacement func.
var mdAutoLink = regexp.MustCompile(`\[([^\]]+)\]\(https?://([^)]+)\)`)

// repairMarkdownAutoLink unwraps Markdown autolinks such as
// [example.com](http://example.com) into their bare URL form
// (example.com). These wrappers frequently confuse downstream parsers that
// expect a plain string. Only links whose text equals the URL host are
// rewritten; the repair tag records the affected key.
func repairMarkdownAutoLink(data map[string]interface{}) (map[string]interface{}, []string) {
	var repairs []string
	for key, val := range data {
		s, ok := val.(string)
		if !ok {
			continue
		}
		fixed := mdAutoLink.ReplaceAllStringFunc(s, func(match string) string {
			sub := mdAutoLink.FindStringSubmatch(match)
			if len(sub) < 3 || sub[1] != sub[2] {
				return match
			}
			return sub[1]
		})
		if fixed != s {
			data[key] = fixed
			repairs = append(repairs, "markdown-auto-link:"+key)
		}
	}
	return data, repairs
}

// repairNullForOptional deletes fields whose value is JSON null. Optional
// fields represented as absent keys are preferred over explicit null, which
// some downstream consumers reject.
func repairNullForOptional(data map[string]interface{}) (map[string]interface{}, []string) {
	var repairs []string
	for key, val := range data {
		if val == nil {
			delete(data, key)
			repairs = append(repairs, "null-for-optional:"+key)
		}
	}
	return data, repairs
}

// arrayFields is the set of object keys known to expect array values. A bare
// string under one of these keys is wrapped into a single-element array.
var arrayFields = map[string]bool{
	"paths":      true,
	"files":      true,
	"args":       true,
	"deps":       true,
	"selectors":  true,
	"ids":        true,
	"targets":    true,
}

// repairBareStringWrap wraps a bare string value under an array-typed key
// (see arrayFields) into a single-element array. This runs after
// json-array-parse so that a stringified array is first decoded into a real
// array and never re-wrapped.
func repairBareStringWrap(data map[string]interface{}) (map[string]interface{}, []string) {
	var repairs []string
	for key, val := range data {
		if _, ok := val.(string); !ok {
			continue
		}
		if !arrayFields[key] {
			continue
		}
		data[key] = []interface{}{val}
		repairs = append(repairs, "bare-string-wrap:"+key)
	}
	return data, repairs
}
