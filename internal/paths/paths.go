package paths

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/saadjs/agent-skills/internal/utils"
)

type Tool string

const (
	ToolAgents   Tool = "agents"
	ToolCodex    Tool = "codex"
	ToolClaude   Tool = "claude"
	ToolCursor   Tool = "cursor"
	ToolWindsurf Tool = "windsurf"
	ToolCopilot  Tool = "copilot"
)

type Scope string

const (
	ScopeGlobal  Scope = "global"
	ScopeProject Scope = "project"
)

var tools = []Tool{ToolAgents, ToolCodex, ToolClaude, ToolCursor, ToolWindsurf, ToolCopilot}

func Tools() []Tool {
	return tools
}

func ParseTool(value string) (Tool, error) {
	v := strings.ToLower(strings.TrimSpace(value))
	for _, tool := range tools {
		if v == string(tool) {
			return tool, nil
		}
	}
	return "", fmt.Errorf("unknown tool: %s", value)
}

func ParseScope(value string) (Scope, error) {
	v := strings.ToLower(strings.TrimSpace(value))
	switch v {
	case string(ScopeGlobal):
		return ScopeGlobal, nil
	case string(ScopeProject):
		return ScopeProject, nil
	default:
		return "", fmt.Errorf("unknown scope: %s", value)
	}
}

func Resolve(tool Tool, scope Scope, cwd string) (string, error) {
	var path string
	switch tool {
	case ToolAgents:
		if scope == ScopeGlobal {
			path = "~/.agents/skills"
		} else {
			path = "./.agents/skills"
		}
	case ToolCodex:
		if scope == ScopeGlobal {
			path = "~/.codex/skills"
		} else {
			path = "./.codex/skills"
		}
	case ToolClaude:
		if scope == ScopeGlobal {
			path = "~/.claude/skills"
		} else {
			path = "./.claude/skills"
		}
	case ToolCursor:
		if scope == ScopeGlobal {
			path = "~/.cursor/skills"
		} else {
			path = "./.cursor/skills"
		}
	case ToolWindsurf:
		if scope == ScopeGlobal {
			path = "~/.codeium/windsurf/skills"
		} else {
			path = "./.windsurf/skills"
		}
	case ToolCopilot:
		if scope == ScopeGlobal {
			path = "~/.copilot/skills"
		} else {
			path = "./.github/skills"
		}
	default:
		return "", fmt.Errorf("unknown tool: %s", tool)
	}
	resolved, err := utils.ExpandHome(path)
	if err != nil {
		return "", err
	}
	if strings.HasPrefix(path, "./") {
		return filepath.Join(cwd, path[2:]), nil
	}
	return resolved, nil
}
