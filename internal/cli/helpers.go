package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/saadjs/skillctl/internal/paths"
	"github.com/saadjs/skillctl/internal/prompts"
	"github.com/saadjs/skillctl/internal/utils"
)

func resolveDestination(toolFlag, scopeFlag, destFlag string, yes bool) (string, error) {
	if destFlag != "" {
		return expandDest(destFlag)
	}
	var tool paths.Tool
	var scope paths.Scope
	var err error
	if toolFlag != "" {
		tool, err = paths.ParseTool(toolFlag)
		if err != nil {
			return "", err
		}
	}
	if scopeFlag != "" {
		scope, err = paths.ParseScope(scopeFlag)
		if err != nil {
			return "", err
		}
	}
	if yes {
		if tool == "" || scope == "" {
			return "", fmt.Errorf("--yes requires --tool and --scope or --dest")
		}
	} else {
		if scope == "" {
			selection, err := prompts.AskSelect("Select scope", []string{"global", "project"})
			if err != nil {
				return "", err
			}
			scope, _ = paths.ParseScope(selection)
		}
		if tool == "" {
			cwd, err := os.Getwd()
			if err != nil {
				return "", err
			}
			options, toolMap := toolPromptOptions(scope, cwd)
			selection, err := prompts.AskSelect("Select tool", options)
			if err != nil {
				return "", err
			}
			tool = toolMap[selection]
		}
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return paths.Resolve(tool, scope, cwd)
}

func resolveAddDestinations(toolFlags []string, scopeFlag, destFlag string, yes bool) ([]string, error) {
	if len(toolFlags) == 0 {
		if destFlag != "" {
			dest, err := expandDest(destFlag)
			if err != nil {
				return nil, err
			}
			return []string{dest}, nil
		}
		if yes {
			return nil, fmt.Errorf("--yes requires --tool and --scope or --dest")
		}
	}
	if destFlag != "" {
		return nil, fmt.Errorf("--dest cannot be combined with --tool")
	}

	var scope paths.Scope
	var err error
	if scopeFlag != "" {
		scope, err = paths.ParseScope(scopeFlag)
		if err != nil {
			return nil, err
		}
	}
	if scope == "" {
		if yes {
			return nil, fmt.Errorf("--yes requires --scope when --tool is used")
		}
		selection, err := prompts.AskSelect("Select scope", []string{"global", "project"})
		if err != nil {
			return nil, err
		}
		scope, _ = paths.ParseScope(selection)
	}

	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	if len(toolFlags) == 0 {
		options, toolMap := toolPromptOptions(scope, cwd)
		selections, err := prompts.AskMulti("Select tools", options)
		if err != nil {
			return nil, err
		}
		toolFlags = make([]string, 0, len(selections))
		for _, selection := range selections {
			toolFlags = append(toolFlags, string(toolMap[selection]))
		}
	}
	seenTools := make(map[paths.Tool]bool, len(toolFlags))
	destinations := make([]string, 0, len(toolFlags))
	for _, toolFlag := range toolFlags {
		tool, err := paths.ParseTool(toolFlag)
		if err != nil {
			return nil, err
		}
		if seenTools[tool] {
			continue
		}
		seenTools[tool] = true
		dest, err := paths.Resolve(tool, scope, cwd)
		if err != nil {
			return nil, err
		}
		destinations = append(destinations, dest)
	}
	return destinations, nil
}

func toolPromptOptions(scope paths.Scope, cwd string) ([]string, map[string]paths.Tool) {
	options := []string{}
	toolMap := make(map[string]paths.Tool)
	for _, tool := range paths.Tools() {
		resolved, err := paths.Resolve(tool, scope, cwd)
		if err != nil {
			continue
		}
		// Shorten home directory to ~ for display.
		display := resolved
		if home, err := os.UserHomeDir(); err == nil && strings.HasPrefix(resolved, home) {
			display = "~" + resolved[len(home):]
		}
		option := fmt.Sprintf("%s (%s)", tool, display)
		options = append(options, option)
		toolMap[option] = tool
	}
	return options, toolMap
}

func expandDest(dest string) (string, error) {
	expanded, err := utils.ExpandHome(dest)
	if err != nil {
		return "", err
	}
	if filepath.IsAbs(expanded) {
		return expanded, nil
	}
	return filepath.Abs(expanded)
}
