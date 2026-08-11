package role

import (
	"os"
	"path/filepath"
	"strings"
)

// Frontmatter holds the parsed YAML frontmatter from a ROLE.md file.
type Frontmatter struct {
	Role              string
	Model             string
	Agent             string
	ReviewedBy        string
	DelegatesTo       []string
	AllowedFiles      []string
	ForbiddenPatterns []string
	Skills            []string
}

// Load reads roles/COMMON.md + roles/<name>/ROLE.md and returns the
// concatenated content suitable as a system prompt.
// Load reads roles/COMMON.md + roles/<name>/ROLE.md relative to root.
// If root is empty, the current working directory is used.
func LoadFrom(root, name string) (string, error) {
	if root == "" {
		root = "."
	}
	var parts []string

	common, err := os.ReadFile(filepath.Join(root, ".mill", "roles", "COMMON.md"))
	if err == nil {
		parts = append(parts, string(common))
	}

	rolePath := filepath.Join(root, ".mill", "roles", name, "ROLE.md")
	roleContent, err := os.ReadFile(rolePath)
	if err != nil {
		return "", err
	}

	parts = append(parts, string(roleContent))
	return strings.Join(parts, "\n"), nil
}

// Load reads roles/COMMON.md + roles/<name>/ROLE.md from the current
// working directory. Prefer LoadFrom with an explicit root.
func Load(name string) (string, error) {
	return LoadFrom("", name)
}

// ParseFrontmatter extracts the YAML frontmatter from a ROLE.md file.
// Frontmatter is delimited by --- lines at the start of the file.
func ParseFrontmatter(path string) (Frontmatter, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Frontmatter{}, err
	}
	return parseFrontmatter(string(data))
}

// ParseFrontmatterString parses frontmatter from a string (useful for tests).
func ParseFrontmatterString(content string) (Frontmatter, error) {
	return parseFrontmatter(content)
}

func parseFrontmatter(content string) (Frontmatter, error) {
	var fm Frontmatter

	lines := strings.Split(content, "\n")
	if len(lines) < 3 || strings.TrimSpace(lines[0]) != "---" {
		return fm, nil // no frontmatter
	}

	// Find closing ---
	end := 0
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			end = i
			break
		}
	}
	if end == 0 {
		return fm, nil
	}

	var currentList *[]string
	for i := 1; i < end; i++ {
		line := lines[i]
		trimmed := strings.TrimSpace(line)

		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		// List item: "  - value"
		if strings.HasPrefix(trimmed, "- ") {
			val := strings.TrimPrefix(trimmed, "- ")
			if currentList != nil {
				*currentList = append(*currentList, val)
			}
			continue
		}

		// Key: value
		parts := strings.SplitN(trimmed, ":", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])

		switch key {
		case "role":
			fm.Role = val
		case "model":
			fm.Model = val
		case "agent":
			fm.Agent = val
		case "reviewed_by":
			fm.ReviewedBy = val
		case "delegates_to":
			fm.DelegatesTo = []string{}
			currentList = &fm.DelegatesTo
		case "allowed_files":
			fm.AllowedFiles = []string{}
			currentList = &fm.AllowedFiles
		case "forbidden_patterns":
			fm.ForbiddenPatterns = []string{}
			currentList = &fm.ForbiddenPatterns
		case "skills":
			fm.Skills = []string{}
			currentList = &fm.Skills
		default:
			currentList = nil
		}
	}

	return fm, nil
}
