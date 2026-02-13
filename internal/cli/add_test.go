package cli

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/saadjs/agent-skills/internal/security"
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

func TestAddRemoteDryRunRunsCloneAndScan(t *testing.T) {
	repoDir := t.TempDir()
	destDir := filepath.Join(t.TempDir(), "dest")
	mustWriteTestFile(t, filepath.Join(repoDir, "skills", "alpha", "SKILL.md"), "# alpha\n")

	origClone := cloneRepo
	origScan := scanRepo
	defer func() {
		cloneRepo = origClone
		scanRepo = origScan
	}()

	cloneCalls := 0
	scanCalls := 0
	cloneRepo = func(repoURL, ref string) (string, error) {
		cloneCalls++
		return repoDir, nil
	}
	scanRepo = func(root string) (security.Report, error) {
		scanCalls++
		return security.Report{}, nil
	}

	restoreOutput, output := captureOutput(t)

	cmd := newAddCommand()
	positional, err := parseWithInterspersed(cmd.FlagSet, []string{
		"--dest", destDir,
		"--skill", "alpha",
		"--dry-run",
		"--yes",
		"owner/repo",
	})
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if err := cmd.Run(positional); err != nil {
		t.Fatalf("run failed: %v", err)
	}
	if cloneCalls != 1 {
		t.Fatalf("expected clone called once, got %d", cloneCalls)
	}
	if scanCalls != 1 {
		t.Fatalf("expected scan called once, got %d", scanCalls)
	}
	restoreOutput()
	if !strings.Contains(output.String(), "Dry run: security scan executed.") {
		t.Fatalf("expected security dry-run message, got: %s", output.String())
	}
	if _, err := os.Stat(destDir); err == nil || !os.IsNotExist(err) {
		t.Fatalf("expected dest directory not to be created")
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

func captureOutput(t *testing.T) (func(), *bytes.Buffer) {
	t.Helper()
	origStdout := os.Stdout
	origStderr := os.Stderr
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe failed: %v", err)
	}
	os.Stdout = writer
	os.Stderr = writer
	var out bytes.Buffer
	done := make(chan struct{})
	go func() {
		_, _ = io.Copy(&out, reader)
		close(done)
	}()

	restore := func() {
		_ = writer.Close()
		<-done
		_ = reader.Close()
		os.Stdout = origStdout
		os.Stderr = origStderr
	}
	return restore, &out
}

func mustWriteTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write failed: %v", err)
	}
}
