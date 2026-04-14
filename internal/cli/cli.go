package cli

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"
)

type Command struct {
	Name        string
	Short       string
	Usage       string
	FlagSet     *flag.FlagSet
	Run         func(args []string) error
	AllowNoArgs bool
}

func Execute(args []string) {
	commands := []*Command{
		newAddCommand(),
		newListCommand(),
		newRemoveCommand(),
		newSyncCommand(),
	}
	if len(args) > 0 {
		switch args[0] {
		case "-v", "--version", "-V":
			fmt.Fprintln(os.Stdout, Version)
			return
		}
	}
	if len(args) == 0 {
		printUsage(commands)
		os.Exit(1)
	}
	if args[0] == "-h" || args[0] == "--help" {
		printUsage(commands)
		return
	}
	cmd := findCommand(commands, args[0])
	if cmd == nil {
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", args[0])
		printUsage(commands)
		os.Exit(1)
	}
	cmd.FlagSet.Usage = func() {
		fmt.Fprintf(os.Stderr, "%s\n\n%s\n", cmd.Usage, cmd.Short)
		cmd.FlagSet.PrintDefaults()
	}
	positional, err := parseWithInterspersed(cmd.FlagSet, args[1:])
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return
		}
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if !cmd.AllowNoArgs && len(positional) == 0 {
		cmd.FlagSet.Usage()
		os.Exit(1)
	}
	if err := cmd.Run(positional); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func findCommand(commands []*Command, name string) *Command {
	for _, cmd := range commands {
		if cmd.Name == name {
			return cmd
		}
	}
	return nil
}

func printUsage(commands []*Command) {
	fmt.Fprintln(os.Stderr, "skillctl - install and manage agent skills")
	fmt.Fprintln(os.Stderr, "\nUsage:")
	fmt.Fprintln(os.Stderr, "  skillctl <command> [flags]")
	fmt.Fprintln(os.Stderr, "\nCommands:")
	ordered := append([]*Command{}, commands...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].Name < ordered[j].Name })
	for _, cmd := range ordered {
		fmt.Fprintf(os.Stderr, "  %-8s %s\n", cmd.Name, cmd.Short)
	}
	fmt.Fprintln(os.Stderr, "\nRun 'skillctl <command> --help' for details.")
}

func parseWithInterspersed(fs *flag.FlagSet, args []string) ([]string, error) {
	if len(args) == 0 {
		return nil, fs.Parse(args)
	}
	var flags []string
	var positional []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			positional = append(positional, args[i+1:]...)
			break
		}
		if strings.HasPrefix(arg, "-") && arg != "-" {
			if strings.Contains(arg, "=") {
				flags = append(flags, arg)
				continue
			}
			name := strings.TrimLeft(arg, "-")
			if name == "" {
				positional = append(positional, arg)
				continue
			}
			flagDef := fs.Lookup(name)
			flags = append(flags, arg)
			if flagDef != nil && !isBoolFlag(flagDef) {
				if i+1 >= len(args) {
					return nil, fmt.Errorf("flag needs an argument: %s", arg)
				}
				flags = append(flags, args[i+1])
				i++
			}
			continue
		}
		positional = append(positional, arg)
	}
	if err := fs.Parse(flags); err != nil {
		return nil, err
	}
	return positional, nil
}

type boolFlag interface {
	IsBoolFlag() bool
}

func isBoolFlag(f *flag.Flag) bool {
	if f == nil {
		return false
	}
	if bf, ok := f.Value.(boolFlag); ok {
		return bf.IsBoolFlag()
	}
	return false
}
