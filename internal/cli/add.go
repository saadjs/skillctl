package cli

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/saadjs/agent-skills/internal/git"
	"github.com/saadjs/agent-skills/internal/prompts"
	"github.com/saadjs/agent-skills/internal/security"
	"github.com/saadjs/agent-skills/internal/skills"
	"github.com/saadjs/agent-skills/internal/utils"
)

type addOptions struct {
	tool      string
	scope     string
	dest      string
	ref       string
	path      string
	skills    multiString
	overwrite bool
	skip      bool
	force     bool
	yes       bool
	dryRun    bool
}

var cloneRepo = git.Clone
var scanRepo = security.Scan

func newAddCommand() *Command {
	opts := &addOptions{}
	fs := flag.NewFlagSet("add", flag.ContinueOnError)
	fs.StringVar(&opts.tool, "tool", "", "Tool name (agents|codex|claude|cursor|windsurf|copilot)")
	fs.StringVar(&opts.scope, "scope", "", "Scope (global|project)")
	fs.StringVar(&opts.dest, "dest", "", "Destination path (overrides tool/scope)")
	fs.StringVar(&opts.ref, "ref", "", "Git ref (branch, tag, or sha)")
	fs.StringVar(&opts.path, "path", "skills", "Path in repo where skills live")
	fs.Var(&opts.skills, "skill", "Skill to install (repeatable)")
	fs.BoolVar(&opts.overwrite, "overwrite", false, "Overwrite existing skills")
	fs.BoolVar(&opts.skip, "skip", false, "Skip skills that already exist")
	fs.BoolVar(&opts.force, "force", false, "Bypass security findings and continue")
	fs.BoolVar(&opts.yes, "yes", false, "Non-interactive mode")
	fs.BoolVar(&opts.dryRun, "dry-run", false, "Print actions without changes")

	cmd := &Command{
		Name:        "add",
		Short:       "Install skills from a GitHub repo or local path",
		Usage:       "skillctl add <repo|path>",
		FlagSet:     fs,
		AllowNoArgs: false,
		Run: func(args []string) error {
			if len(args) < 1 {
				return errors.New("repo or path is required")
			}
			if opts.overwrite && opts.skip {
				return errors.New("--overwrite and --skip cannot be used together")
			}
			dest, err := resolveDestination(opts.tool, opts.scope, opts.dest, opts.yes)
			if err != nil {
				return err
			}

			source := args[0]
			repoPath, isLocal, err := utils.ResolveLocalPath(source)
			if err != nil {
				return err
			}

			if isLocal {
				if opts.ref != "" {
					return errors.New("--ref is not supported for local paths")
				}
			} else {
				repo, err := utils.NormalizeRepo(source)
				if err != nil {
					return err
				}
				repoURL := utils.RepoURL(repo)
				repoPath, err = cloneRepo(repoURL, opts.ref)
				if err != nil {
					return err
				}
			}

			securityReport, err := scanRepo(repoPath)
			if err != nil {
				return fmt.Errorf("security scan failed: %w", err)
			}
			printSecurityScanReport(securityReport)
			if opts.dryRun {
				utils.PrintInfo("Dry run: security scan executed.")
			}
			if err := enforceSecurityDecision(securityReport, opts.force, opts.yes); err != nil {
				return err
			}

			skillsDir := filepath.Join(repoPath, opts.path)
			allSkills, err := skills.Discover(skillsDir)
			if err != nil {
				return fmt.Errorf("unable to read skills at %s: %w", skillsDir, err)
			}
			selected, missing := chooseSkills(allSkills, opts.skills.values, opts.yes)
			if len(missing) > 0 {
				return fmt.Errorf("skills not found: %s", strings.Join(missing, ", "))
			}
			if len(selected) == 0 {
				return fmt.Errorf("no skills found in %s", skillsDir)
			}
			selectedNames := skillNames(selected)

			if opts.dryRun {
				if _, err := os.Stat(dest); err != nil {
					if os.IsNotExist(err) {
						utils.PrintInfo("Dry run: would create destination directory %s", dest)
					} else {
						return err
					}
				}
				utils.PrintInfo("Dry run: would install %d skill(s) to %s", len(selectedNames), dest)
				for _, name := range selectedNames {
					if exists(dest, name) {
						if opts.overwrite {
							utils.PrintInfo("Would overwrite %s", name)
						} else if opts.skip {
							utils.PrintInfo("Would skip %s", name)
						} else {
							utils.PrintInfo("Would prompt for %s (already exists)", name)
						}
					} else {
						utils.PrintInfo("Would install %s", name)
					}
				}
				return nil
			}

			if err := utils.EnsureDir(dest); err != nil {
				return err
			}
			if len(selected) == 0 {
				return errors.New("no skills selected")
			}
			utils.PrintInfo("Installing %d skill(s) to %s", len(selected), dest)
			mode := "ask"
			if opts.overwrite {
				mode = "overwrite"
			}
			if opts.skip {
				mode = "skip"
			}
			for _, skill := range selected {
				overwrite := mode == "overwrite"
				skip := mode == "skip"
				if !overwrite && !skip {
					if exists(dest, skill.Name) {
						choice, err := promptConflict(skill.Name)
						if err != nil {
							return err
						}
						switch choice {
						case "overwrite":
							overwrite = true
						case "skip":
							skip = true
						case "overwrite-all":
							overwrite = true
							mode = "overwrite"
						case "skip-all":
							skip = true
							mode = "skip"
						case "cancel":
							return errors.New("canceled")
						}
					}
				}
				if skip {
					utils.PrintInfo("Skipping %s", skill.Name)
					continue
				}
				err := skills.CopySkill(skill, dest, overwrite)
				if errors.Is(err, os.ErrExist) {
					utils.PrintWarn("Skill %s already exists", skill.Name)
					continue
				}
				if err != nil {
					return err
				}
				utils.PrintInfo("Installed %s", skill.Name)
			}
			return nil
		},
	}
	return cmd
}

func printSecurityScanReport(report security.Report) {
	utils.PrintInfo("%s", security.Summary(report))
	if len(report.Findings) == 0 {
		return
	}
	for _, line := range security.DetailLines(report) {
		utils.PrintWarn("%s", line)
	}
}

func enforceSecurityDecision(report security.Report, force, yes bool) error {
	if len(report.Findings) == 0 {
		return nil
	}
	if force {
		utils.PrintWarn("Proceeding despite security findings because --force was provided.")
		return nil
	}
	if yes {
		return errors.New("security scan found potential malicious content; rerun with --force to continue")
	}
	approved, err := prompts.AskYesNo("Continue despite security findings?", false)
	if err != nil {
		return err
	}
	if !approved {
		return errors.New("canceled due to security findings")
	}
	utils.PrintWarn("Proceeding despite security findings by user confirmation.")
	return nil
}

func exists(destRoot, name string) bool {
	_, err := os.Stat(filepath.Join(destRoot, name))
	return err == nil
}

func promptConflict(skillName string) (string, error) {
	options := []string{
		"overwrite",
		"skip",
		"overwrite-all",
		"skip-all",
		"cancel",
	}
	selection, err := prompts.AskSelect(fmt.Sprintf("Skill %s exists. Choose action", skillName), options)
	if err != nil {
		return "", err
	}
	return selection, nil
}

func skillNames(items []skills.Skill) []string {
	if len(items) == 0 {
		return nil
	}
	names := make([]string, 0, len(items))
	for _, item := range items {
		names = append(names, item.Name)
	}
	return names
}

func chooseSkills(all []skills.Skill, requested []string, yes bool) ([]skills.Skill, []string) {
	if len(requested) > 0 {
		return skills.Filter(all, requested)
	}
	if yes {
		return all, nil
	}
	options := []string{"[all]"}
	for _, item := range all {
		options = append(options, item.Name)
	}
	selection, err := prompts.AskMulti("Select skills to install", options)
	if err != nil {
		return nil, []string{err.Error()}
	}
	if len(selection) == 0 {
		return nil, nil
	}
	for _, pick := range selection {
		if pick == "[all]" {
			return all, nil
		}
	}
	return skills.Filter(all, selection)
}

// multiString is a repeatable flag for --skill.
type multiString struct {
	values []string
}

func (m *multiString) String() string {
	return strings.Join(m.values, ",")
}

func (m *multiString) Set(value string) error {
	m.values = append(m.values, value)
	return nil
}
