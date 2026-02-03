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
	yes       bool
	dryRun    bool
}

func newAddCommand() *Command {
	opts := &addOptions{}
	fs := flag.NewFlagSet("add", flag.ContinueOnError)
	fs.StringVar(&opts.tool, "tool", "", "Tool name (codex|claude|cursor|windsurf|copilot)")
	fs.StringVar(&opts.scope, "scope", "", "Scope (global|project)")
	fs.StringVar(&opts.dest, "dest", "", "Destination path (overrides tool/scope)")
	fs.StringVar(&opts.ref, "ref", "", "Git ref (branch, tag, or sha)")
	fs.StringVar(&opts.path, "path", "skills", "Path in repo where skills live")
	fs.Var(&opts.skills, "skill", "Skill to install (repeatable)")
	fs.BoolVar(&opts.overwrite, "overwrite", false, "Overwrite existing skills")
	fs.BoolVar(&opts.skip, "skip", false, "Skip skills that already exist")
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
			if err := utils.EnsureDir(dest); err != nil {
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
				repoPath, err = git.Clone(repoURL, opts.ref)
				if err != nil {
					return err
				}
			}
			skillsDir := filepath.Join(repoPath, opts.path)
			allSkills, err := skills.Discover(skillsDir)
			if err != nil {
				return fmt.Errorf("unable to read skills at %s: %w", skillsDir, err)
			}
			selected, missing := skills.Filter(allSkills, opts.skills.values)
			if len(missing) > 0 {
				return fmt.Errorf("skills not found: %s", strings.Join(missing, ", "))
			}
			if len(selected) == 0 {
				return fmt.Errorf("no skills found in %s", skillsDir)
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
				if opts.dryRun {
					utils.PrintInfo("Would install %s", skill.Name)
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
