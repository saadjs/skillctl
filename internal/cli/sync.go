package cli

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/saadjs/agent-skills/internal/config"
	"github.com/saadjs/agent-skills/internal/paths"
	"github.com/saadjs/agent-skills/internal/prompts"
	"github.com/saadjs/agent-skills/internal/security"
	"github.com/saadjs/agent-skills/internal/skills"
	"github.com/saadjs/agent-skills/internal/utils"
)

type syncOptions struct {
	source string
	tools  multiString
	skills multiString
	force  bool
	yes    bool
	dryRun bool
	all    bool
}

var resolvePath = paths.Resolve

type syncResult struct {
	tool    string
	dest    string
	synced  []string
	skipped []string
}

func newSyncCommand() *Command {
	opts := &syncOptions{}
	fs := flag.NewFlagSet("sync", flag.ContinueOnError)
	fs.StringVar(&opts.source, "source", "", "Source skills directory (overrides config)")
	fs.Var(&opts.tools, "tool", "Target tool (repeatable, overrides config)")
	fs.Var(&opts.skills, "skill", "Skill to sync (repeatable)")
	fs.BoolVar(&opts.force, "force", false, "Bypass security findings")
	fs.BoolVar(&opts.yes, "yes", false, "Non-interactive mode")
	fs.BoolVar(&opts.dryRun, "dry-run", false, "Print actions without changes")
	fs.BoolVar(&opts.all, "all", false, "Sync all skills, ignore checksums")

	cmd := &Command{
		Name:        "sync",
		Short:       "Sync skills from source to all configured tools",
		Usage:       "skillctl sync [flags]",
		FlagSet:     fs,
		AllowNoArgs: true,
		Run: func(args []string) error {
			return runSync(opts)
		},
	}
	return cmd
}

func runSync(opts *syncOptions) error {
	cfgPath := config.ConfigPath()
	statePath := config.StatePath()

	cfg, err := loadOrCreateConfig(cfgPath, opts.yes, &config.Config{
		Source: opts.source,
		Tools:  append([]string(nil), opts.tools.values...),
	})
	if err != nil {
		return err
	}

	source := cfg.Source
	if opts.source != "" {
		source = opts.source
	}
	if source == "" {
		return errors.New("source directory is not configured; update config or use --source")
	}
	source, err = utils.ExpandHome(source)
	if err != nil {
		return fmt.Errorf("expanding source path: %w", err)
	}
	info, err := os.Stat(source)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("source directory does not exist: %s", source)
		}
		return fmt.Errorf("cannot access source directory %s: %w", source, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("source path is not a directory: %s", source)
	}

	toolNames := cfg.Tools
	if len(opts.tools.values) > 0 {
		toolNames = opts.tools.values
	}
	toolList, err := parseToolList(toolNames)
	if err != nil {
		return err
	}
	if len(toolList) == 0 {
		return errors.New("no tools configured")
	}

	allSkills, err := skills.Discover(source)
	if err != nil {
		return fmt.Errorf("discovering skills in %s: %w", source, err)
	}
	if len(allSkills) == 0 {
		return fmt.Errorf("no skills found in %s", source)
	}

	selected := allSkills
	if len(opts.skills.values) > 0 {
		var missing []string
		selected, missing = skills.Filter(allSkills, opts.skills.values)
		if len(missing) > 0 {
			return fmt.Errorf("skills not found: %s", strings.Join(missing, ", "))
		}
	}

	report, err := scanSelectedSkills(selected)
	if err != nil {
		return fmt.Errorf("security scan failed: %w", err)
	}
	printSecurityScanReport(report)
	if err := enforceSecurityDecision(report, opts.force, opts.yes); err != nil {
		return err
	}

	st, err := config.LoadState(statePath)
	if err != nil {
		return fmt.Errorf("loading state: %w", err)
	}

	checksums := map[string]string{}
	for _, skill := range selected {
		cs, err := config.ChecksumSkill(skill.Path)
		if err != nil {
			return fmt.Errorf("checksumming %s: %w", skill.Name, err)
		}
		checksums[skill.Name] = cs
	}

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting working directory: %w", err)
	}

	var results []syncResult

	for _, tool := range toolList {
		dest, err := resolvePath(tool, paths.ScopeGlobal, cwd)
		if err != nil {
			utils.PrintWarn("Skipping %s: %v", tool, err)
			continue
		}
		parent := filepath.Dir(dest)
		parentInfo, parentErr := os.Stat(parent)
		if parentErr != nil {
			if os.IsNotExist(parentErr) {
				utils.PrintWarn("Skipping %s: %s does not exist (tool not installed?)", tool, parent)
			} else {
				utils.PrintWarn("Skipping %s: cannot access %s: %v", tool, parent, parentErr)
			}
			continue
		}
		if !parentInfo.IsDir() {
			utils.PrintWarn("Skipping %s: %s is not a directory", tool, parent)
			continue
		}

		result := syncResult{tool: string(tool), dest: dest}

		toolKey := string(tool)
		toolState := st.Tools[toolKey]

		if opts.dryRun {
			for _, skill := range selected {
				cs := checksums[skill.Name]
				prev, hasPrev := toolState[skill.Name]
				if !opts.all && hasPrev && prev.Checksum == cs {
					result.skipped = append(result.skipped, skill.Name)
				} else {
					result.synced = append(result.synced, skill.Name)
				}
			}
			results = append(results, result)
			continue
		}

		if err := utils.EnsureDir(dest); err != nil {
			return fmt.Errorf("creating %s: %w", dest, err)
		}

		for _, skill := range selected {
			cs := checksums[skill.Name]
			prev, hasPrev := toolState[skill.Name]
			if !opts.all && hasPrev && prev.Checksum == cs {
				result.skipped = append(result.skipped, skill.Name)
				continue
			}
			if err := skills.CopySkill(skill, dest, true); err != nil {
				return fmt.Errorf("copying %s to %s: %w", skill.Name, dest, err)
			}
			result.synced = append(result.synced, skill.Name)
		}
		results = append(results, result)
	}

	if !opts.dryRun && len(results) > 0 {
		now := config.NowTimestamp()
		skillFiltered := len(opts.skills.values) > 0
		selectedNames := map[string]bool{}
		for _, s := range selected {
			selectedNames[s.Name] = true
		}
		for _, r := range results {
			if st.Tools[r.tool] == nil {
				st.Tools[r.tool] = map[string]config.SkillState{}
			}
			for _, name := range r.synced {
				st.Tools[r.tool][name] = config.SkillState{
					Checksum: checksums[name],
					SyncedAt: now,
				}
			}
			if !skillFiltered {
				for name := range st.Tools[r.tool] {
					if !selectedNames[name] {
						delete(st.Tools[r.tool], name)
					}
				}
			}
		}
		st.LastSync = now
		if err := config.SaveState(statePath, st); err != nil {
			return fmt.Errorf("saving state: %w", err)
		}
	}

	printSyncSummary(results, opts.dryRun)
	return nil
}

func loadOrCreateConfig(cfgPath string, yes bool, preset *config.Config) (*config.Config, error) {
	cfg, err := config.LoadConfig(cfgPath)
	if err == nil {
		return cfg, nil
	}
	if !os.IsNotExist(err) {
		return nil, fmt.Errorf("reading config: %w", err)
	}

	cfg = &config.Config{}
	if preset != nil {
		cfg.Source = preset.Source
		cfg.Tools = append(cfg.Tools, preset.Tools...)
	}
	if cfg.Source != "" {
		absSource, err := normalizeBootstrapSource(cfg.Source)
		if err != nil {
			return nil, err
		}
		cfg.Source = absSource
	}
	if cfg.Source != "" && len(cfg.Tools) > 0 {
		// Flags fully specify the sync; skip bootstrap and avoid creating config.yaml.
		return cfg, nil
	}
	if yes {
		switch {
		case cfg.Source == "" && len(cfg.Tools) == 0:
			return nil, fmt.Errorf("no config found at %s; run sync interactively first or create config manually", cfgPath)
		case cfg.Source == "":
			return nil, errors.New("source directory is not configured; update config or use --source")
		default:
			return nil, errors.New("no tools configured")
		}
	}

	utils.PrintInfo("No config found. Let's set one up.")

	if cfg.Source == "" {
		sourceInput, err := prompts.AskInput("Source directory (where your skills live)")
		if err != nil {
			return nil, err
		}
		if sourceInput == "" {
			return nil, errors.New("source directory is required")
		}
		absSource, err := normalizeBootstrapSource(sourceInput)
		if err != nil {
			return nil, err
		}
		cfg.Source = absSource
	}

	if len(cfg.Tools) == 0 {
		var toolOptions []string
		for _, t := range paths.Tools() {
			toolOptions = append(toolOptions, string(t))
		}
		toolSelections, err := prompts.AskMulti("Select tools to sync to", toolOptions)
		if err != nil {
			return nil, err
		}
		if len(toolSelections) == 0 {
			return nil, errors.New("at least one tool is required")
		}
		cfg.Tools = toolSelections
	}

	if err := config.SaveConfig(cfgPath, cfg); err != nil {
		return nil, fmt.Errorf("saving config: %w", err)
	}
	utils.PrintInfo("Config saved to %s", cfgPath)
	return cfg, nil
}

func normalizeBootstrapSource(sourceInput string) (string, error) {
	expanded, err := utils.ExpandHome(sourceInput)
	if err != nil {
		return "", fmt.Errorf("invalid source path: %w", err)
	}
	absSource, err := filepath.Abs(expanded)
	if err != nil {
		return "", fmt.Errorf("resolving source path: %w", err)
	}
	expandedInfo, expandedErr := os.Stat(absSource)
	if expandedErr != nil {
		return "", fmt.Errorf("source directory not accessible: %s: %v", sourceInput, expandedErr)
	}
	if !expandedInfo.IsDir() {
		return "", fmt.Errorf("source path is not a directory: %s", sourceInput)
	}
	return absSource, nil
}

func parseToolList(names []string) ([]paths.Tool, error) {
	var tools []paths.Tool
	for _, name := range names {
		tool, err := paths.ParseTool(name)
		if err != nil {
			return nil, err
		}
		tools = append(tools, tool)
	}
	return tools, nil
}

func scanSelectedSkills(selected []skills.Skill) (security.Report, error) {
	merged := security.Report{SkipReasons: map[string]int{}}
	for _, skill := range selected {
		r, err := scanRepo(skill.Path)
		if err != nil {
			return security.Report{}, err
		}
		for _, f := range r.Findings {
			f.Path = filepath.ToSlash(filepath.Join(skill.Name, f.Path))
			merged.Findings = append(merged.Findings, f)
		}
		merged.FilesScanned += r.FilesScanned
		merged.FilesSkipped += r.FilesSkipped
		for k, v := range r.SkipReasons {
			merged.SkipReasons[k] += v
		}
	}
	return merged, nil
}

func printSyncSummary(results []syncResult, dryRun bool) {
	if len(results) == 0 {
		utils.PrintInfo("No tools to sync to.")
		return
	}
	prefix := ""
	if dryRun {
		prefix = "Dry run: "
	}
	for _, r := range results {
		if len(r.synced) == 0 && len(r.skipped) == 0 {
			continue
		}
		utils.PrintInfo("%s%s (%s):", prefix, r.tool, r.dest)
		for _, name := range r.synced {
			if dryRun {
				utils.PrintInfo("  would sync %s", name)
			} else {
				utils.PrintInfo("  synced %s", name)
			}
		}
		for _, name := range r.skipped {
			utils.PrintInfo("  unchanged %s", name)
		}
	}
}
