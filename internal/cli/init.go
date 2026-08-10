package cli

import (
	"bufio"
	"embed"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"text/template"
)

//go:embed static static/scaffold/.omp static/scaffold/.claude static/scaffold/.github static/scaffold/.mill
var staticFS embed.FS
// initConfig holds the values used to render the mill.yml template.
type initConfig struct {
	Name      string
	Provider  string
	Model     string
	MaxRounds int
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
	var target string

	flagSet.StringVar(&cfg.Name, "name", "", "project name (default: current directory name)")
	flagSet.StringVar(&cfg.Provider, "provider", "commandcode", "AI provider (commandcode|claude)")
	flagSet.StringVar(&cfg.Model, "model", "laguna-free", "provider model identifier")
	flagSet.IntVar(&cfg.MaxRounds, "max-rounds", 4, "max review rounds before REJECTED")
	flagSet.BoolVar(&yes, "yes", false, "skip interactive prompts, use defaults/flags")
	flagSet.BoolVar(&force, "force", false, "overwrite .mill/ without confirmation (DESTRUCTIVE)")
	flagSet.StringVar(&target, "target", "", "target directory (default: current directory)")

	if err := flagSet.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return nil
		}
		return err
	}

	if target == "" {
		target = "."
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

	// Overwrite check — before any filesystem writes, unless --force.
	if !force {
		if err := promptOverwrite(target, in, a.Out); err != nil {
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
	if err := a.copyScaffold(target); err != nil {
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

// promptOverwrite checks whether .mill/ exists at target and warns before overwriting.
// It returns nil if safe to proceed, or an error if the user declines or .mill/ is a file.
func promptOverwrite(target string, in *bufio.Reader, out io.Writer) error {
	millPath := filepath.Join(target, ".mill")
	entry, err := os.Stat(millPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to check %s: %w", millPath, err)
	}

	if !entry.IsDir() {
		return fmt.Errorf(".mill/ exists but is a file, not a directory — remove it manually and retry")
	}

	fmt.Fprintf(out, "⚠ .mill/ already exists.\n")
	fmt.Fprintf(out, "This will overwrite:\n")

	// Walk .mill/ subdirectories and files to show what will be lost.
	for _, sub := range []string{"state.json", "ledger", "worktrees", "roles", "checks", "skills", "docs"} {
		p := filepath.Join(millPath, sub)
		info, err := os.Stat(p)
		if err != nil {
			continue
		}
		if info.IsDir() {
			entries, _ := os.ReadDir(p)
			count := len(entries)
			label := "entries"
			switch sub {
			case "worktrees":
				label = "worktrees"
			}
			fmt.Fprintf(out, "  .mill/%s/ (%d %s)\n", sub, count, label)
		} else {
			fmt.Fprintf(out, "  .mill/%s (%s)\n", sub, formatSize(info.Size()))
		}
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
func (a *App) copyScaffold(target string) error {
	return fs.WalkDir(staticFS, "static/scaffold", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Strip the "static/scaffold" prefix to get the relative path.
		rel := strings.TrimPrefix(path, "static/scaffold/")
		if rel == "." || rel == "" {
			return nil
		}

		// Normalize path separators for the host OS.
		dest := filepath.Join(target, filepath.FromSlash(rel))

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
