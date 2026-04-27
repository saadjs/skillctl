package cli

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/saadjs/skillctl/internal/config"
	"github.com/saadjs/skillctl/internal/skills"
	"github.com/saadjs/skillctl/internal/utils"
)

type updateOptions struct {
	force  bool
	yes    bool
	dryRun bool
}

type remoteInstallSelection struct {
	key   string
	entry config.RemoteInstallState
	skill string
}

type remoteUpdateGroup struct {
	source string
	ref    string
	path   string
	items  []remoteInstallSelection
}

func newUpdateCommand() *Command {
	opts := &updateOptions{}
	fs := flag.NewFlagSet("update", flag.ContinueOnError)
	fs.BoolVar(&opts.force, "force", false, "Bypass security findings and continue")
	fs.BoolVar(&opts.yes, "yes", false, "Non-interactive mode")
	fs.BoolVar(&opts.dryRun, "dry-run", false, "Print actions without changes")

	return &Command{
		Name:        "update",
		Short:       "Update tracked remote skills",
		Usage:       "skillctl update [skills...] [flags]",
		FlagSet:     fs,
		AllowNoArgs: true,
		Run: func(args []string) error {
			return runUpdate(opts, args)
		},
	}
}

func runUpdate(opts *updateOptions, requested []string) error {
	st, err := config.LoadState(config.StatePath())
	if err != nil {
		return fmt.Errorf("loading state: %w", err)
	}
	if len(st.RemoteInstalls) == 0 {
		return errors.New("no remote installs are tracked")
	}

	selections, missing := selectRemoteInstalls(st.RemoteInstalls, requested)
	if len(missing) > 0 {
		return fmt.Errorf("tracked remote skills not found: %s", strings.Join(missing, ", "))
	}
	if len(selections) == 0 {
		return errors.New("no remote installs are tracked")
	}

	groups := groupRemoteInstalls(selections)
	for _, group := range groups {
		if err := updateRemoteGroup(group, opts, st); err != nil {
			return err
		}
	}

	if !opts.dryRun {
		if err := config.SaveState(config.StatePath(), st); err != nil {
			return fmt.Errorf("saving state: %w", err)
		}
	}
	return nil
}

func updateRemoteGroup(group remoteUpdateGroup, opts *updateOptions, st *config.State) error {
	repoPath, err := cloneRepo(utils.RepoURL(group.source), group.ref)
	if err != nil {
		return err
	}
	cleanupCloneDir := filepath.Dir(repoPath)
	defer func() {
		if cleanupCloneDir != "" {
			_ = os.RemoveAll(cleanupCloneDir)
		}
	}()

	skillsDir := filepath.Join(repoPath, group.path)
	allSkills, err := skills.Discover(skillsDir)
	if err != nil {
		return fmt.Errorf("unable to read skills at %s: %w", skillsDir, err)
	}
	names := uniqueSelectionNames(group.items)
	selected, missing := skills.Filter(allSkills, names)
	if len(missing) > 0 {
		return fmt.Errorf("skills not found: %s", strings.Join(missing, ", "))
	}

	report, err := scanSelectedSkills(selected)
	if err != nil {
		return fmt.Errorf("security scan failed: %w", err)
	}
	printSecurityScanReport(report)
	if err := enforceSecurityDecision(report, opts.force, opts.yes); err != nil {
		return err
	}

	selectedByName := map[string]skills.Skill{}
	for _, skill := range selected {
		selectedByName[skill.Name] = skill
	}

	now := config.NowTimestamp()
	for _, item := range group.items {
		skill := selectedByName[item.skill]
		if opts.dryRun {
			utils.PrintInfo("Dry run: would update %s in %s from %s", item.skill, item.entry.Destination, item.entry.Source)
			continue
		}
		if err := utils.EnsureDir(item.entry.Destination); err != nil {
			return fmt.Errorf("creating %s: %w", item.entry.Destination, err)
		}
		if err := skills.CopySkill(skill, item.entry.Destination, true); err != nil {
			return fmt.Errorf("copying %s to %s: %w", item.skill, item.entry.Destination, err)
		}
		item.entry.UpdatedAt = now
		if item.entry.InstalledAt == "" {
			item.entry.InstalledAt = now
		}
		st.RemoteInstalls[item.key] = item.entry
		utils.PrintInfo("Updated %s in %s", item.skill, item.entry.Destination)
	}
	return nil
}

func selectRemoteInstalls(installs map[string]config.RemoteInstallState, requested []string) ([]remoteInstallSelection, []string) {
	requestedSet := map[string]bool{}
	for _, name := range requested {
		name = strings.TrimSpace(name)
		if name != "" {
			requestedSet[name] = false
		}
	}

	keys := make([]string, 0, len(installs))
	for key := range installs {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	var selected []remoteInstallSelection
	for _, key := range keys {
		entry := installs[key]
		for _, name := range remoteInstallSkillNames(key, entry) {
			if len(requestedSet) > 0 {
				if _, ok := requestedSet[name]; !ok {
					continue
				}
				requestedSet[name] = true
			}
			selected = append(selected, remoteInstallSelection{
				key:   key,
				entry: entry,
				skill: name,
			})
		}
	}

	var missing []string
	for name, found := range requestedSet {
		if !found {
			missing = append(missing, name)
		}
	}
	sort.Strings(missing)
	return selected, missing
}

func groupRemoteInstalls(selections []remoteInstallSelection) []remoteUpdateGroup {
	groupsByKey := map[string]*remoteUpdateGroup{}
	var keys []string
	for _, selection := range selections {
		groupKey := strings.Join([]string{selection.entry.Source, selection.entry.Ref, selection.entry.Path}, "\x00")
		group := groupsByKey[groupKey]
		if group == nil {
			group = &remoteUpdateGroup{
				source: selection.entry.Source,
				ref:    selection.entry.Ref,
				path:   selection.entry.Path,
			}
			groupsByKey[groupKey] = group
			keys = append(keys, groupKey)
		}
		group.items = append(group.items, selection)
	}
	sort.Strings(keys)
	groups := make([]remoteUpdateGroup, 0, len(keys))
	for _, key := range keys {
		groups = append(groups, *groupsByKey[key])
	}
	return groups
}

func uniqueSelectionNames(items []remoteInstallSelection) []string {
	seen := map[string]bool{}
	for _, item := range items {
		seen[item.skill] = true
	}
	names := make([]string, 0, len(seen))
	for name := range seen {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func remoteInstallKey(dest, skillName string) string {
	return filepath.Clean(dest) + "::" + skillName
}

func remoteInstallSkillNames(key string, entry config.RemoteInstallState) []string {
	if len(entry.Skills) > 0 {
		return append([]string(nil), entry.Skills...)
	}
	parts := strings.Split(key, "::")
	if len(parts) == 0 {
		return nil
	}
	return []string{parts[len(parts)-1]}
}
