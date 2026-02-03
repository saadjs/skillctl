package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/saadjs/agent-skills/internal/paths"
	"github.com/saadjs/agent-skills/internal/prompts"
	"github.com/saadjs/agent-skills/internal/utils"
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
		if tool == "" {
			options := []string{}
			for _, t := range paths.Tools() {
				options = append(options, string(t))
			}
			selection, err := prompts.AskSelect("Select tool", options)
			if err != nil {
				return "", err
			}
			tool, _ = paths.ParseTool(selection)
		}
		if scope == "" {
			selection, err := prompts.AskSelect("Select scope", []string{"global", "project"})
			if err != nil {
				return "", err
			}
			scope, _ = paths.ParseScope(selection)
		}
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return paths.Resolve(tool, scope, cwd)
}

func expandDest(dest string) (string, error) {
	if strings.HasPrefix(dest, "./") {
		cwd, err := os.Getwd()
		if err != nil {
			return "", err
		}
		return filepath.Join(cwd, dest[2:]), nil
	}
	return utils.ExpandHome(dest)
}
