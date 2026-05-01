package cli

import "testing"

func TestFindCommandAliases(t *testing.T) {
	commands := []*Command{
		{Name: "add"},
		{Name: "list"},
		{Name: "remove"},
	}

	tests := map[string]string{
		"install": "add",
		"ls":      "list",
		"rm":      "remove",
	}

	for alias, canonical := range tests {
		cmd := findCommand(commands, alias)
		if cmd == nil {
			t.Fatalf("expected %q alias to resolve", alias)
		}
		if cmd.Name != canonical {
			t.Fatalf("expected %q alias to resolve to %q, got %q", alias, canonical, cmd.Name)
		}
	}
}
