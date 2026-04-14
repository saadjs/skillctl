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

	cfg, err := loadOrCreateConfig(cfgPath, opts.yes)
	if err != nil {
		// If both --source and --tool are provided, config is not required.
		if opts.source != "" && len(opts.tools.values) > 0 {
			cfg = &config.Config{}
		} else {
			return err
		}
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

	report, err := scanRepo(source)
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
		return err
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
				utils.PrintInfo("Skipping %s: %s does not exist (tool not installed?)", tool, parent)
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

	if !opts.dryRun {
		anySynced := false
		now := config.NowTimestamp()
		for _, r := range results {
			if len(r.synced) == 0 {
				continue
			}
			anySynced = true
			if st.Tools[r.tool] == nil {
				st.Tools[r.tool] = map[string]config.SkillState{}
			}
			for _, name := range r.synced {
				st.Tools[r.tool][name] = config.SkillState{
					Checksum: checksums[name],
					SyncedAt: now,
				}
			}
		}
		if anySynced {
			st.LastSync = now
			if err := config.SaveState(statePath, st); err != nil {
				return fmt.Errorf("saving state: %w", err)
			}
		}
	}

	printSyncSummary(results, opts.dryRun)
	return nil
}

func loadOrCreateConfig(cfgPath string, yes bool) (*config.Config, error) {
	cfg, err := config.LoadConfig(cfgPath)
	if err == nil {
		return cfg, nil
	}
	if !os.IsNotExist(err) {
		return nil, fmt.Errorf("reading config: %w", err)
	}
	if yes {
		return nil, fmt.Errorf("no config found at %s; run sync interactively first or create config manually", cfgPath)
	}

	utils.PrintInfo("No config found. Let's set one up.")

	sourceInput, err := prompts.AskInput("Source directory (where your skills live)")
	if err != nil {
		return nil, err
	}
	if sourceInput == "" {
		return nil, errors.New("source directory is required")
	}
	expanded, err := utils.ExpandHome(sourceInput)
	if err != nil {
		return nil, fmt.Errorf("invalid source path: %w", err)
	}
	expandedInfo, expandedErr := os.Stat(expanded)
	if expandedErr != nil {
		return nil, fmt.Errorf("source directory not accessible: %s: %v", sourceInput, expandedErr)
	}
	if !expandedInfo.IsDir() {
		return nil, fmt.Errorf("source path is not a directory: %s", sourceInput)
	}

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

	cfg = &config.Config{
		Source: sourceInput,
		Tools:  toolSelections,
	}
	if err := config.SaveConfig(cfgPath, cfg); err != nil {
		return nil, fmt.Errorf("saving config: %w", err)
	}
	utils.PrintInfo("Config saved to %s", cfgPath)
	return cfg, nil
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
