package cli

import (
	"errors"
	"flag"
	"fmt"

	"github.com/saadjs/skillctl/internal/prompts"
	"github.com/saadjs/skillctl/internal/skills"
)

type removeOptions struct {
	tool   string
	scope  string
	dest   string
	skills multiString
	yes    bool
}

func newRemoveCommand() *Command {
	opts := &removeOptions{}
	fs := flag.NewFlagSet("remove", flag.ContinueOnError)
	fs.StringVar(&opts.tool, "tool", "", "Tool name (agents|codex|claude|cursor|windsurf|copilot)")
	fs.StringVar(&opts.scope, "scope", "", "Scope (global|project)")
	fs.StringVar(&opts.dest, "dest", "", "Destination path (overrides tool/scope)")
	fs.Var(&opts.skills, "skill", "Skill to remove (repeatable)")
	fs.BoolVar(&opts.yes, "yes", false, "Non-interactive mode")

	cmd := &Command{
		Name:        "remove",
		Short:       "Remove installed skills",
		Usage:       "skillctl remove --skill <name>",
		FlagSet:     fs,
		AllowNoArgs: true,
		Run: func(args []string) error {
			dest, err := resolveDestination(opts.tool, opts.scope, opts.dest, opts.yes)
			if err != nil {
				return err
			}
			installed, err := skills.ListInstalled(dest)
			if err != nil {
				return err
			}
			if len(installed) == 0 {
				fmt.Printf("No skills installed in %s\n", dest)
				return nil
			}
			if len(opts.skills.values) == 0 {
				if opts.yes {
					return errors.New("--yes requires --skill for remove")
				}
				var options []string
				for _, skill := range installed {
					options = append(options, skill.Name)
				}
				selections, err := prompts.AskMulti("Select skills to remove", options)
				if err != nil {
					return err
				}
				opts.skills.values = selections
			}
			_, missing := skills.Filter(installed, opts.skills.values)
			if len(missing) > 0 {
				return fmt.Errorf("skills not found: %v", missing)
			}
			for _, name := range opts.skills.values {
				if err := skills.RemoveSkill(name, dest); err != nil {
					return err
				}
				fmt.Printf("Removed %s\n", name)
			}
			return nil
		},
	}
	return cmd
}
