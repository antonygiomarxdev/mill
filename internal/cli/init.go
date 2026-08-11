package cli

import (
	"bufio"
	"embed"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"text/template"
)

//go:embed static static/scaffold/.omp static/scaffold/.claude static/scaffold/.github static/scaffold/.mill
var staticFS embed.FS

// initConfig holds the values used to render the mill.yml template.
type initConfig struct {
	Name        string
	Provider    string
	Model       string
	MaxRounds   int
	ProjectType string // "go", "js", "monorepo", "generic"
	Minimal     bool   // true when --minimal is set
}

// ProjectType classifies the project for scaffolding decisions.
type ProjectType int

const (
	ProjectGeneric  ProjectType = iota // no recognized sentinel
	ProjectGoModule                      // go.mod found, no package.json
	ProjectJSProject                     // package.json found, no go.mod
	ProjectMonoRepo                      // BOTH go.mod AND package.json at different levels
)

func (p ProjectType) String() string {
	switch p {
	case ProjectGoModule:
		return "go"
	case ProjectJSProject:
		return "js"
	case ProjectMonoRepo:
		return "monorepo"
	default:
		return "generic"
	}
}

func parseProjectType(s string) (ProjectType, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "go", "gomodule":
		return ProjectGoModule, nil
	case "js", "javascript", "node", "typescript":
		return ProjectJSProject, nil
	case "monorepo", "mono":
		return ProjectMonoRepo, nil
	case "generic", "":
		return ProjectGeneric, nil
	default:
		return ProjectGeneric, fmt.Errorf("unknown project type: %q (valid: go, js, monorepo, generic)", s)
	}
}

// runInit handles the "init" command.
// It scaffolds a new mill project by generating mill.yml, creating the
// directory structure (roles, checks, skills, docs), and copying bundled
// starter files from the mill binary.
func (a *App) runInit(args []string) error {
	flagSet := flag.NewFlagSet("init", flag.ContinueOnError)
	flagSet.SetOutput(a.Err)

	var cfg initConfig
	var yes bool
	var force bool
	var dryRun bool
	var target string
	var minimal bool
	var typeFlag string

	flagSet.StringVar(&cfg.Name, "name", "", "project name (default: current directory name)")
	flagSet.StringVar(&cfg.Provider, "provider", "commandcode", "AI provider (commandcode|claude)")
	flagSet.StringVar(&cfg.Model, "model", "laguna-free", "provider model identifier")
	flagSet.IntVar(&cfg.MaxRounds, "max-rounds", 4, "max review rounds before REJECTED")
	flagSet.BoolVar(&yes, "yes", false, "skip interactive prompts, use defaults/flags")
	flagSet.BoolVar(&force, "force", false, "overwrite .mill/ without confirmation (DESTRUCTIVE)")
	flagSet.BoolVar(&dryRun, "dry-run", false, "show what would be created without writing")
	flagSet.StringVar(&target, "target", "", "target directory (default: current directory)")
	flagSet.BoolVar(&minimal, "minimal", false, "create only mill.yml + bootstrap roles (no full scaffold)")
	flagSet.StringVar(&typeFlag, "type", "", "project type override: go, js, monorepo, generic")

	if err := flagSet.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return nil
		}
		return err
	}

	if target == "" {
		target = "."
	}

	cfg.Minimal = minimal

	// Pre-init validation — checks git is on PATH, detects project type.
	// In --minimal mode, skip detection entirely: default to "generic" unless
	// --type overrides explicitly.
	var detected ProjectType
	if cfg.Minimal {
		if typeFlag != "" {
			pt, err := parseProjectType(typeFlag)
			if err != nil {
				return err
			}
			cfg.ProjectType = pt.String()
		} else {
			cfg.ProjectType = "generic"
		}
		// Use detected for validatePreInit — minimal mode still needs a type
		// for binary validation (Go projects need go on PATH).
		detected = detectProjectType(target)
	} else {
		detected = detectProjectType(target)
		cfg.ProjectType = detected.String()

		// --type flag overrides detection.
		if typeFlag != "" {
			pt, err := parseProjectType(typeFlag)
			if err != nil {
				return err
			}
			cfg.ProjectType = pt.String()
			if pt != detected {
				fmt.Fprintf(a.Err, "Using explicit project type: %s (detected: %s)\n", pt, detected)
			}
		}

		if detected == ProjectGeneric {
			fmt.Fprintf(a.Err, "Warning: no go.mod or package.json detected. Scaffolding generic project.\n")
		}
	}

	// In --minimal mode, skip detection messaging entirely (spec: detection is not called).
	// We still ran detectProjectType above for --type-override-aware logic, but
	// we suppress the detection messages.
	if !cfg.Minimal {
		if detected == ProjectGeneric {
			fmt.Fprintf(a.Err, "Warning: no go.mod or package.json detected. Scaffolding generic project.\n")
		}
	}

	if err := validatePreInit(target, detected); err != nil {
		return err
	}

	// --dry-run: report what would happen without writing anything.
	if dryRun {
		return a.dryRunInit(target, cfg, force)
	}

	if cfg.Name == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get working directory: %w", err)
		}
		cfg.Name = filepath.Base(cwd)
	}

	// Create buffered input reader — reused for overwrite check and config prompts.
	in := bufio.NewReader(a.In)

	// Overwrite check — before any filesystem writes.
	switch {
	case force && yes:
		// CI mode: skip all confirmation.
	case force:
		// --force without --yes: skip interactive prompt but warn.
		fmt.Fprintf(a.Err, "Warning: --force will overwrite existing files without confirmation\n")
	default:
		if err := promptOverwrite(target, cfg, in, a.Out, force); err != nil {
			return err
		}
	}

	// Interactive prompts (skipped with -yes)
	if !yes {
		cfg.Name = prompt(in, a.Out, "Project name", cfg.Name)
		cfg.Provider = prompt(in, a.Out, "Provider", cfg.Provider)
		cfg.Model = prompt(in, a.Out, "Model", cfg.Model)

		mr := prompt(in, a.Out, "Max rounds", strconv.Itoa(cfg.MaxRounds))
		if n, err := strconv.Atoi(strings.TrimSpace(mr)); err == nil {
			cfg.MaxRounds = n
		}
	}

	// Ensure target directory exists
	if err := os.MkdirAll(target, 0o755); err != nil {
		return fmt.Errorf("failed to create target directory %s: %w", target, err)
	}

	// Generate mill.yml from template
	if err := a.generateMillYAML(target, cfg); err != nil {
		return err
	}

	// Copy bundled starter files (roles, checks, skills, docs)
	if err := a.copyScaffold(target, cfg, force); err != nil {
		return err
	}

	// Create .mill/ runtime directories
	for _, sub := range []string{"ledger", "worktrees", "phases", "artifacts", "memory"} {
		os.MkdirAll(filepath.Join(target, ".mill", sub), 0o755)
	}

	// Clean up empty static/ dir from embedded FS walk
	os.Remove(filepath.Join(target, "static", "scaffold"))
	os.Remove(filepath.Join(target, "static"))

	fmt.Fprintf(a.Out, "mill project initialized in %s\n", target)
	fmt.Fprintf(a.Out, "Next steps:\n")
	fmt.Fprintf(a.Out, "  1. Open this project in your harness (omp/claude/codex)\n")
	fmt.Fprintf(a.Out, "  2. The agent loads @.mill/skills/mill.md automatically\n")
	fmt.Fprintf(a.Out, "  3. Start delegating: just tell your agent what to build\n")

	return nil
}

// detectProjectType walks up from target toward the filesystem root looking for
// sentinel files (go.mod, package.json). The first go.mod found wins; package.json
// is noted but the walk continues in case go.mod exists higher up.
func detectProjectType(target string) ProjectType {
	abs, err := filepath.Abs(target)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: cannot resolve %s: %v — defaulting to generic project\n", target, err)
		return ProjectGeneric
	}

	root := abs
	foundGoMod := false
	foundPackageJSON := false

	for {
		if _, err := os.Stat(filepath.Join(root, "go.mod")); err == nil {
			foundGoMod = true
			break
		}
		if _, err := os.Stat(filepath.Join(root, "package.json")); err == nil {
			foundPackageJSON = true
			// DON'T break — continue looking for go.mod higher up
		}
		parent := filepath.Dir(root)
		if parent == root {
			break
		}
		root = parent
	}

	if foundGoMod {
		if foundPackageJSON {
			return ProjectMonoRepo // has BOTH go.mod AND package.json at different levels
		}
		return ProjectGoModule
	}

	if foundPackageJSON {
		return ProjectJSProject // only package.json, no go.mod
	}

	return ProjectGeneric
}

// validatePreInit checks that the target is a viable git project before
// any scaffold files are written. Returns an actionable error on failure.
// The Go requirement is relaxed: non-Go projects only need git.
func validatePreInit(target string, pt ProjectType) error {
	// Git must always be on PATH. Go is only required for Go projects.
	if err := validateBinaries(pt == ProjectGoModule || pt == ProjectMonoRepo); err != nil {
		return err
	}

	// Walk up from target to find the project root — the nearest ancestor
	// containing a go.mod. For non-Go projects, this is relaxed.
	abs, err := filepath.Abs(target)
	if err != nil {
		return fmt.Errorf("cannot resolve target path %s: %w", target, err)
	}

	// For Go projects: find the go.mod root and verify .git exists there.
	if pt == ProjectGoModule || pt == ProjectMonoRepo {
		root := abs
		for {
			if _, err := os.Stat(filepath.Join(root, "go.mod")); err == nil {
				break
			}
			parent := filepath.Dir(root)
			if parent == root {
				return fmt.Errorf("no go.mod found at target. Run `go mod init <module>` first.")
			}
			root = parent
		}

		gitDir := filepath.Join(root, ".git")
		if info, err := os.Stat(gitDir); err != nil || !info.IsDir() {
			return fmt.Errorf("target is not a git repository. Run `git init` first.")
		}
		return nil
	}

	// For non-Go projects: git can be anywhere up the tree or in target.
	root := abs
	for {
		gitDir := filepath.Join(root, ".git")
		if info, err := os.Stat(gitDir); err == nil && info.IsDir() {
			return nil
		}
		parent := filepath.Dir(root)
		if parent == root {
			return fmt.Errorf("target is not a git repository. Run `git init` first.")
		}
		root = parent
	}
}

// validateBinaries checks that required toolchain binaries are on PATH.
// requireGo controls whether the go binary is required (only for GoModule/MonoRepo).
func validateBinaries(requireGo bool) error {
	if _, err := exec.LookPath("git"); err != nil {
		return fmt.Errorf("git not found in PATH. Install git to continue.")
	}
	if requireGo {
		if _, err := exec.LookPath("go"); err != nil {
			return fmt.Errorf("go not found in PATH. Install Go to continue.")
		}
	}
	return nil
}

// preserveDir checks whether a scaffold destination should be preserved
// because the top-level harness directory already exists on disk with content.
func preserveDir(target, dest string) bool {
	rel, err := filepath.Rel(target, dest)
	if err != nil {
		return false
	}
	topDir, _, _ := strings.Cut(rel, string(filepath.Separator))
	if topDir == "" {
		topDir = rel
	}

	// Only preserve known harness directories.
	switch topDir {
	case ".github", ".claude", ".omp", ".mill", "checks":
	default:
		return false
	}

	topPath := filepath.Join(target, topDir)
	info, err := os.Stat(topPath)
	if err != nil || !info.IsDir() {
		return false
	}
	return dirIsNonEmpty(target, topDir)
}

// dirIsNonEmpty returns true if the directory at root/dir contains at least one entry.
func dirIsNonEmpty(root, dir string) bool {
	entries, err := os.ReadDir(filepath.Join(root, dir))
	if err != nil {
		return false
	}
	return len(entries) > 0
}

// promptOverwrite checks whether scaffold files would be overwritten at target.
// It returns nil if safe to proceed, or an error if the user declines.
// Preserved directories are excluded from the conflict list.
func promptOverwrite(target string, cfg initConfig, in *bufio.Reader, out io.Writer, force bool) error {
	conflicts, err := collisionReport(target, cfg, force)
	if err != nil {
		return err
	}
	if len(conflicts) == 0 {
		return nil
	}

	fmt.Fprintf(out, "⚠ %d file(s) would be overwritten:\n", len(conflicts))
	show := len(conflicts)
	if show > 5 {
		show = 5
	}
	for i := range show {
		fmt.Fprintf(out, "  %s\n", conflicts[i])
	}
	if len(conflicts) > 5 {
		fmt.Fprintf(out, "  ... and %d more\n", len(conflicts)-5)
	}

	fmt.Fprintf(out, "Overwrite? [y/N]: ")
	line, err := in.ReadString('\n')
	if err != nil {
		return fmt.Errorf("init aborted")
	}
	line = strings.TrimSpace(strings.ToLower(line))
	if line != "y" && line != "yes" {
		return fmt.Errorf("init aborted")
	}
	return nil
}

// collisionReport walks the embedded scaffold tree and returns file paths
// that already exist at target (files that would be overwritten by init).
// Preserved directories are excluded from the conflict list.
// Returns (nil, nil) when target is empty, .git-only, or has no collisions.
func collisionReport(target string, cfg initConfig, force bool) ([]string, error) {
	// Check for .mill as a non-directory — hard error.
	millPath := filepath.Join(target, ".mill")
	if info, err := os.Stat(millPath); err == nil && !info.IsDir() {
		return nil, fmt.Errorf(".mill/ exists but is a file, not a directory — remove it manually and retry")
	}

	// Target empty or .git-only? No collision to warn about.
	entries, err := os.ReadDir(target)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read target directory %s: %w", target, err)
	}
	nonGit := false
	for _, e := range entries {
		if e.Name() != ".git" {
			nonGit = true
			break
		}
	}
	if !nonGit {
		return nil, nil
	}

	var conflicts []string

	// Include .mill/ directory itself if it exists (backward-compat with old prompt).
	// Skip if it would be preserved.
	if info, err := os.Stat(millPath); err == nil && info.IsDir() {
		if force || !preserveDir(target, millPath) {
			conflicts = append(conflicts, ".mill/")
		}
	}

	// Walk the embedded scaffold tree for file-level collisions.
	err = fs.WalkDir(staticFS, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel := strings.TrimPrefix(path, "static/scaffold/")
		if rel == path || rel == "" {
			return nil
		}

		// In minimal mode, only check files that would be created.
		if cfg.Minimal && !minimalFile(rel) {
			return nil
		}

		// For minimal mode COMMON.md, adjust the destination path.
		dest := filepath.Join(target, filepath.FromSlash(rel))
		if cfg.Minimal && rel == ".mill/roles/COMMON.md" {
			dest = filepath.Join(target, ".mill", "COMMON.md")
		}

		// Skip preserved directories.
		if !force && preserveDir(target, dest) {
			return nil
		}

		if _, err := os.Stat(dest); err == nil {
			conflicts = append(conflicts, rel)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to walk scaffold: %w", err)
	}

	// Check mill.yml (generated separately by generateMillYAML).
	millYML := filepath.Join(target, "mill.yml")
	if _, err := os.Stat(millYML); err == nil {
		conflicts = append(conflicts, "mill.yml")
	}

	if len(conflicts) == 0 {
		return nil, nil
	}
	return conflicts, nil
}

// minimalFile returns true if the given scaffold-relative path should be
// included in --minimal mode.
func minimalFile(rel string) bool {
	switch rel {
	case ".mill/roles/staff/ROLE.md",
		".mill/roles/pm/ROLE.md",
		".mill/roles/COMMON.md":
		return true
	}
	return false
}

// dryRunInit reports what init would do without writing any files.
func (a *App) dryRunInit(target string, cfg initConfig, force bool) error {
	conflicts, err := collisionReport(target, cfg, force)
	if err != nil {
		return err
	}
	if len(conflicts) > 0 {
		fmt.Fprintf(a.Out, "Would overwrite %d existing file(s):\n", len(conflicts))
		for _, c := range conflicts {
			fmt.Fprintf(a.Out, "  %s\n", c)
		}
	} else {
		fmt.Fprintf(a.Out, "No existing files would be overwritten.\n")
	}

	// Report preserved directories.
	type preserveInfo struct {
		dir   string
		count int
	}
	var preserved []preserveInfo
	for _, dir := range []string{".github", ".claude", ".omp", ".mill", "checks"} {
		dirPath := filepath.Join(target, dir)
		if info, err := os.Stat(dirPath); err == nil && info.IsDir() && dirIsNonEmpty(target, dir) {
			if !force {
				entries, _ := os.ReadDir(dirPath)
				preserved = append(preserved, preserveInfo{dir, len(entries)})
			}
		}
	}
	for _, p := range preserved {
		fmt.Fprintf(a.Out, "Would preserve existing directory: %s/ (%d files)\n", p.dir, p.count)
	}

	// Count scaffold files that would be created.
	count := 0
	_ = fs.WalkDir(staticFS, "static/scaffold", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if cfg.Minimal && !minimalFile(strings.TrimPrefix(path, "static/scaffold/")) {
			return nil
		}
		if !force && preserveDir(target, filepath.Join(target, filepath.FromSlash(strings.TrimPrefix(path, "static/scaffold/")))) {
			return nil
		}
		count++
		return nil
	})
	fmt.Fprintf(a.Out, "Would create: %d scaffold files\n", count)

	return nil
}

// formatSize returns a human-readable size string.
func formatSize(n int64) string {
	if n < 1024 {
		return fmt.Sprintf("%dB", n)
	}
	if n < 1024*1024 {
		return fmt.Sprintf("%.1fKB", float64(n)/1024)
	}
	return fmt.Sprintf("%.1fMB", float64(n)/(1024*1024))
}

// prompt reads a line from in, displaying a label with a default value.
// An empty response returns the default.
func prompt(in *bufio.Reader, out io.Writer, label, def string) string {
	fmt.Fprintf(out, "%s [%s]: ", label, def)
	line, err := in.ReadString('\n')
	if err != nil && err != io.EOF {
		return def
	}
	line = strings.TrimSpace(line)
	if line == "" {
		return def
	}
	return line
}

// generateMillYAML renders the mill.yml template with the given config
// and writes it to target/mill.yml.
func (a *App) generateMillYAML(target string, cfg initConfig) error {
	tmplBytes, err := staticFS.ReadFile("static/mill.yml.tmpl")
	if err != nil {
		return fmt.Errorf("failed to read mill.yml template: %w", err)
	}

	tmpl, err := template.New("mill.yml").Parse(string(tmplBytes))
	if err != nil {
		return fmt.Errorf("failed to parse mill.yml template: %w", err)
	}

	var buf strings.Builder
	if err := tmpl.Execute(&buf, cfg); err != nil {
		return fmt.Errorf("failed to render mill.yml template: %w", err)
	}

	path := filepath.Join(target, "mill.yml")
	if err := os.WriteFile(path, []byte(buf.String()), 0o644); err != nil {
		return fmt.Errorf("failed to write mill.yml: %w", err)
	}

	fmt.Fprintf(a.Out, "Created %s\n", path)
	return nil
}

// copyScaffold walks the embedded scaffold directory and copies every file
// to target, preserving the relative directory structure.
// When cfg.Minimal is true, only the bootstrap files are copied.
// When force is false, existing non-empty harness directories are preserved.
func (a *App) copyScaffold(target string, cfg initConfig, force bool) error {
	// Pre-compute which top-level harness directories are already present
	// on disk with content. We must do this BEFORE the walk because the walk
	// creates directories eagerly, which would make subsequent preserve checks
	// falsely detect the directories we just created.
	preserved := make(map[string]bool)
	if !force {
		for _, dir := range []string{".github", ".claude", ".omp", ".mill", "checks"} {
			dirPath := filepath.Join(target, dir)
			if info, err := os.Stat(dirPath); err == nil && info.IsDir() && dirIsNonEmpty(target, dir) {
				preserved[dir] = true
				fmt.Fprintf(a.Err, "Preserving existing %s/ directory (use --force to overwrite)\n", dir)
			}
		}
	}

	return fs.WalkDir(staticFS, "static/scaffold", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Strip the "static/scaffold" prefix to get the relative path.
		rel := strings.TrimPrefix(path, "static/scaffold/")
		if rel == "." || rel == "" {
			return nil
		}

		// In minimal mode, only copy bootstrap files.
		if cfg.Minimal && !minimalFile(rel) {
			return nil
		}

		// For minimal mode, COMMON.md goes to .mill/COMMON.md (not .mill/roles/COMMON.md).
		destRel := rel
		if cfg.Minimal && rel == ".mill/roles/COMMON.md" {
			destRel = ".mill/COMMON.md"
		}

		// Check if this path is under a preserved top-level directory.
		topDir, _, _ := strings.Cut(destRel, "/")
		if topDir == "" {
			topDir = destRel
		}
		if preserved[topDir] {
			return nil
		}

		// Normalize path separators for the host OS.
		dest := filepath.Join(target, filepath.FromSlash(destRel))

		if d.IsDir() {
			return os.MkdirAll(dest, 0o755)
		}

		content, err := staticFS.ReadFile(path)
		if err != nil {
			return fmt.Errorf("failed to read bundled file %s: %w", path, err)
		}

		// Ensure parent directory exists
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return fmt.Errorf("failed to create directory for %s: %w", dest, err)
		}

		// Script files get executable mode
		mode := os.FileMode(0o644)
		if strings.HasPrefix(filepath.Base(dest), "pre-") || strings.HasSuffix(dest, ".sh") {
			mode = 0o755
		}

		if err := os.WriteFile(dest, content, mode); err != nil {
			return fmt.Errorf("failed to write %s: %w", dest, err)
		}

		fmt.Fprintf(a.Out, "Created %s\n", dest)
		return nil
	})
}
