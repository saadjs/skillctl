package cli

import "testing"

func TestFindCommandAliases(t *testing.T) {
	commands := []*Command{
		{Name: "add"},
		{Name: "list"},
		{Name: "remove"},
		{Name: "update"},
	}

	tests := map[string]string{
		"install": "add",
		"ls":      "list",
		"rm":      "remove",
		"upgrade": "update",
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

func TestUpgradeAliasRequiresUpdateCommand(t *testing.T) {
	commands := []*Command{
		{Name: "add"},
		{Name: "list"},
		{Name: "remove"},
	}

	if cmd := findCommand(commands, "upgrade"); cmd != nil {
		t.Fatalf("expected upgrade alias to be unavailable without update command, got %q", cmd.Name)
	}
}
