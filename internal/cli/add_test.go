package cli

import (
	"bytes"
	"io"
	"os"
	"testing"

	"github.com/saadjs/agent-skills/internal/skills"
)

func TestChooseSkillsRequestedByName(t *testing.T) {
	all := []skills.Skill{
		{Name: "alpha"},
		{Name: "beta"},
	}
	selected, missing := chooseSkills(all, []string{"beta"}, false)
	if len(missing) != 0 {
		t.Fatalf("unexpected missing: %v", missing)
	}
	if len(selected) != 1 || selected[0].Name != "beta" {
		t.Fatalf("expected beta, got: %v", selected)
	}
}

func TestChooseSkillsYesSelectsAll(t *testing.T) {
	all := []skills.Skill{
		{Name: "alpha"},
		{Name: "beta"},
	}
	selected, missing := chooseSkills(all, nil, true)
	if len(missing) != 0 {
		t.Fatalf("unexpected missing: %v", missing)
	}
	if len(selected) != 2 {
		t.Fatalf("expected all skills, got: %v", selected)
	}
}

func TestChooseSkillsPromptAll(t *testing.T) {
	all := []skills.Skill{
		{Name: "alpha"},
		{Name: "beta"},
	}
	restore := withStdin(t, "1\n")
	defer restore()

	selected, missing := chooseSkills(all, nil, false)
	if len(missing) != 0 {
		t.Fatalf("unexpected missing: %v", missing)
	}
	if len(selected) != 2 {
		t.Fatalf("expected all skills, got: %v", selected)
	}
}

func TestChooseSkillsPromptSubset(t *testing.T) {
	all := []skills.Skill{
		{Name: "alpha"},
		{Name: "beta"},
	}
	restore := withStdin(t, "2\n")
	defer restore()

	selected, missing := chooseSkills(all, nil, false)
	if len(missing) != 0 {
		t.Fatalf("unexpected missing: %v", missing)
	}
	if len(selected) != 1 || selected[0].Name != "alpha" {
		t.Fatalf("expected alpha, got: %v", selected)
	}
}

func withStdin(t *testing.T, input string) func() {
	t.Helper()
	orig := os.Stdin
	reader := bytes.NewBufferString(input)
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe failed: %v", err)
	}
	if _, err := io.Copy(w, reader); err != nil {
		t.Fatalf("copy failed: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close failed: %v", err)
	}
	os.Stdin = r
	return func() {
		os.Stdin = orig
		_ = r.Close()
	}
}
