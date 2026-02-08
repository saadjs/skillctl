package paths

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolvePaths(t *testing.T) {
	cwd := "/tmp/project"
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("home dir: %v", err)
	}

	cases := []struct {
		tool  Tool
		scope Scope
		want  string
	}{
		// agents should be first as the new standard
		{ToolAgents, ScopeGlobal, filepath.Join(home, ".agents", "skills")},
		{ToolAgents, ScopeProject, filepath.Join(cwd, ".agents", "skills")},
		{ToolCodex, ScopeGlobal, filepath.Join(home, ".codex", "skills")},
		{ToolCodex, ScopeProject, filepath.Join(cwd, ".codex", "skills")},
		{ToolClaude, ScopeGlobal, filepath.Join(home, ".claude", "skill")},
		{ToolClaude, ScopeProject, filepath.Join(cwd, ".claude", "skills")},
		{ToolCursor, ScopeGlobal, filepath.Join(home, ".cursor", "skills")},
		{ToolCursor, ScopeProject, filepath.Join(cwd, ".cursor", "skills")},
		{ToolWindsurf, ScopeGlobal, filepath.Join(home, ".codeium", "windsurf", "skills")},
		{ToolWindsurf, ScopeProject, filepath.Join(cwd, ".windsurf", "skills")},
		{ToolCopilot, ScopeGlobal, filepath.Join(home, ".copilot", "skills")},
		{ToolCopilot, ScopeProject, filepath.Join(cwd, ".github", "skills")},
	}

	for _, tc := range cases {
		got, err := Resolve(tc.tool, tc.scope, cwd)
		if err != nil {
			t.Fatalf("resolve %s/%s: %v", tc.tool, tc.scope, err)
		}
		if got != tc.want {
			t.Fatalf("resolve %s/%s: expected %s, got %s", tc.tool, tc.scope, tc.want, got)
		}
	}
}

func TestParseToolScopeErrors(t *testing.T) {
	if _, err := ParseTool("unknown"); err == nil {
		t.Fatalf("expected tool parse error")
	}
	if _, err := ParseScope("nope"); err == nil {
		t.Fatalf("expected scope parse error")
	}
}
