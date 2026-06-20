package cli

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/saadjs/skillctl/internal/config"
	"github.com/saadjs/skillctl/internal/git"
	"github.com/saadjs/skillctl/internal/prompts"
	"github.com/saadjs/skillctl/internal/security"
	"github.com/saadjs/skillctl/internal/skills"
	"github.com/saadjs/skillctl/internal/utils"
)

type addOptions struct {
	tools     multiString
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
	list      bool
}

var cloneRepo = git.Clone
var scanRepo = security.Scan

func newAddCommand() *Command {
	opts := &addOptions{}
	fs := flag.NewFlagSet("add", flag.ContinueOnError)
	fs.Var(&opts.tools, "tool", "Tool name (repeatable: agents|codex|claude|cursor|windsurf|copilot)")
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
	fs.BoolVar(&opts.list, "list", false, "List skills available in source without installing")

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
			source := args[0]
			if opts.list {
				if err := validateListOptions(opts); err != nil {
					return err
				}
				return listSourceSkills(source, opts)
			}
			if opts.overwrite && opts.skip {
				return errors.New("--overwrite and --skip cannot be used together")
			}
			destinations, err := resolveAddDestinations(opts.tools.values, opts.scope, opts.dest, opts.yes)
			if err != nil {
				return err
			}

			repoPath, isLocal, err := utils.ResolveLocalPath(source)
			if err != nil {
				return err
			}
			performScan := true
			var selected []skills.Skill
			var selectedNames []string
			var remoteSource string

			if isLocal {
				if opts.ref != "" {
					return errors.New("--ref is not supported for local paths")
				}
			} else {
				repo, err := utils.NormalizeRepo(source)
				if err != nil {
					return err
				}
				repoURL, err := utils.CloneURL(source)
				if err != nil {
					return err
				}
				remoteSource = repo
				if opts.dryRun {
					performScan = false
					if len(opts.skills.values) == 0 {
						return errors.New("--dry-run with remote source requires at least one --skill")
					}
					selectedNames = append(selectedNames, opts.skills.values...)
				} else {
					repoPath, err = cloneRepo(repoURL, opts.ref)
					if err != nil {
						return err
					}
					cleanupCloneDir := filepath.Dir(repoPath)
					defer func() {
						if cleanupCloneDir == "" {
							return
						}
						_ = os.RemoveAll(cleanupCloneDir)
					}()
				}
			}

			if performScan {
				skillsDir := filepath.Join(repoPath, opts.path)
				skillsLocation := displaySkillsLocation(source, repoPath, opts.path, isLocal)
				allSkills, err := skills.Discover(skillsDir)
				if err != nil {
					return fmt.Errorf("unable to read skills from %s: %w", skillsLocation, err)
				}
				missing := []string(nil)
				selected, missing = chooseSkills(allSkills, opts.skills.values, opts.yes)
				if len(missing) > 0 {
					return fmt.Errorf("skills not found: %s", strings.Join(missing, ", "))
				}
				if len(selected) == 0 {
					return fmt.Errorf("no skills found in %s", skillsLocation)
				}
				selectedNames = skillNames(selected)

				securityReport, err := scanSelectedSkills(selected)
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
			}

			if opts.dryRun {
				if !performScan {
					utils.PrintInfo("Dry run: remote source was not cloned; security scan skipped.")
				}
				for _, dest := range destinations {
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
				}
				return nil
			}

			if len(selected) == 0 {
				return errors.New("no skills selected")
			}
			for _, dest := range destinations {
				if err := utils.EnsureDir(dest); err != nil {
					return err
				}
				utils.PrintInfo("Installing %d skill(s) to %s", len(selected), dest)
				mode := "ask"
				if opts.overwrite {
					mode = "overwrite"
				}
				if opts.skip {
					mode = "skip"
				}
				var installed []string
				for _, skill := range selected {
					overwrite := false
					skip := false
					if exists(dest, skill.Name) {
						switch mode {
						case "overwrite":
							overwrite = true
						case "skip":
							skip = true
						default:
							choice, err := promptConflict(skill.Name, dest)
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
					installed = append(installed, skill.Name)
					utils.PrintInfo("Installed %s", skill.Name)
				}
				if !isLocal && len(installed) > 0 {
					if err := recordRemoteInstalls(remoteSource, opts.ref, opts.path, dest, installed); err != nil {
						return fmt.Errorf("recording remote install state: %w", err)
					}
				}
			}
			return nil
		},
	}
	return cmd
}

func recordRemoteInstalls(source, ref, skillsPath, dest string, installed []string) error {
	st, err := config.LoadState(config.StatePath())
	if err != nil {
		return fmt.Errorf("loading state: %w", err)
	}
	if st.RemoteInstalls == nil {
		st.RemoteInstalls = map[string]config.RemoteInstallState{}
	}
	now := config.NowTimestamp()
	for _, name := range installed {
		key := remoteInstallKey(dest, name)
		existing := st.RemoteInstalls[key]
		installedAt := existing.InstalledAt
		if installedAt == "" {
			installedAt = now
		}
		st.RemoteInstalls[key] = config.RemoteInstallState{
			Source:      source,
			Ref:         ref,
			Path:        skillsPath,
			Skills:      []string{name},
			Destination: dest,
			InstalledAt: installedAt,
			UpdatedAt:   now,
		}
	}
	return config.SaveState(config.StatePath(), st)
}

func validateListOptions(opts *addOptions) error {
	if opts.overwrite && opts.skip {
		return errors.New("--overwrite and --skip cannot be used together")
	}
	var unsupported []string
	if len(opts.tools.values) > 0 {
		unsupported = append(unsupported, "--tool")
	}
	if opts.scope != "" {
		unsupported = append(unsupported, "--scope")
	}
	if opts.dest != "" {
		unsupported = append(unsupported, "--dest")
	}
	if opts.overwrite {
		unsupported = append(unsupported, "--overwrite")
	}
	if opts.skip {
		unsupported = append(unsupported, "--skip")
	}
	if opts.force {
		unsupported = append(unsupported, "--force")
	}
	if opts.dryRun {
		unsupported = append(unsupported, "--dry-run")
	}
	if len(unsupported) > 0 {
		return fmt.Errorf("%s cannot be used with --list", strings.Join(unsupported, ", "))
	}
	return nil
}

func listSourceSkills(source string, opts *addOptions) error {
	repoPath, isLocal, err := utils.ResolveLocalPath(source)
	if err != nil {
		return err
	}
	if isLocal {
		if opts.ref != "" {
			return errors.New("--ref is not supported for local paths")
		}
	} else {
		repoURL, err := utils.CloneURL(source)
		if err != nil {
			return err
		}
		repoPath, err = cloneRepo(repoURL, opts.ref)
		if err != nil {
			return err
		}
		cleanupCloneDir := filepath.Dir(repoPath)
		defer func() {
			_ = os.RemoveAll(cleanupCloneDir)
		}()
	}

	skillsDir := filepath.Join(repoPath, opts.path)
	skillsLocation := displaySkillsLocation(source, repoPath, opts.path, isLocal)
	allSkills, err := skills.Discover(skillsDir)
	if err != nil {
		return fmt.Errorf("unable to read skills from %s: %w", skillsLocation, err)
	}
	if len(allSkills) == 0 {
		return fmt.Errorf("no skills found in %s", skillsLocation)
	}

	selected := allSkills
	if len(opts.skills.values) > 0 {
		var missing []string
		selected, missing = skills.Filter(allSkills, opts.skills.values)
		if len(missing) > 0 {
			return fmt.Errorf("skills not found: %s", strings.Join(missing, ", "))
		}
	}
	for _, skill := range selected {
		fmt.Println(skill.Name)
	}
	return nil
}

func displaySkillsLocation(source, repoPath, skillsPath string, isLocal bool) string {
	if isLocal {
		return filepath.Join(repoPath, skillsPath)
	}
	if normalized, err := utils.NormalizeRepo(source); err == nil {
		source = normalized
	}
	return fmt.Sprintf("%s at %s", source, filepath.ToSlash(filepath.Clean(skillsPath)))
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

func promptConflict(skillName, dest string) (string, error) {
	options := []string{
		"overwrite",
		"skip",
		"overwrite-all",
		"skip-all",
		"cancel",
	}
	selection, err := prompts.AskSelect(fmt.Sprintf("Skill %s exists in %s. Choose action", skillName, dest), options)
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
