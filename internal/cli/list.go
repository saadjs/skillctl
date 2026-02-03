package cli

import (
	"flag"
	"fmt"

	"github.com/saadjs/agent-skills/internal/skills"
)

type listOptions struct {
	tool string
	scope string
	dest string
	yes bool
}

func newListCommand() *Command {
	opts := &listOptions{}
	fs := flag.NewFlagSet("list", flag.ContinueOnError)
	fs.StringVar(&opts.tool, "tool", "", "Tool name (codex|claude|cursor|windsurf|copilot)")
	fs.StringVar(&opts.scope, "scope", "", "Scope (global|project)")
	fs.StringVar(&opts.dest, "dest", "", "Destination path (overrides tool/scope)")
	fs.BoolVar(&opts.yes, "yes", false, "Non-interactive mode")

	cmd := &Command{
		Name:        "list",
		Short:       "List installed skills",
		Usage:       "skillctl list",
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
			for _, skill := range installed {
				fmt.Println(skill.Name)
			}
			return nil
		},
	}
	return cmd
}
