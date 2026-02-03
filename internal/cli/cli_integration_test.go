package cli

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestAddListRemoveLocalRepo(t *testing.T) {
	repoDir := t.TempDir()
	skillsDir := filepath.Join(repoDir, "skills")
	mustMkdir(t, skillsDir)
	mustWrite(t, filepath.Join(skillsDir, "de-dupe", "SKILL.md"), "# de-dupe\n")
	mustWrite(t, filepath.Join(skillsDir, "readme-maintainer", "SKILL.md"), "# readme\n")

	destDir := t.TempDir()

	out := runSkillctl(t, "add", repoDir, "--dest", destDir, "--skill", "de-dupe", "--overwrite", "--yes")
	if !strings.Contains(out, "Installed de-dupe") {
		t.Fatalf("expected install output, got: %s", out)
	}

	out = runSkillctl(t, "list", "--dest", destDir)
	if strings.TrimSpace(out) != "de-dupe" {
		t.Fatalf("expected list output to be de-dupe, got: %s", out)
	}

	out = runSkillctl(t, "remove", "--dest", destDir, "--skill", "de-dupe", "--yes")
	if !strings.Contains(out, "Removed de-dupe") {
		t.Fatalf("expected remove output, got: %s", out)
	}

	out = runSkillctl(t, "list", "--dest", destDir)
	if !strings.Contains(out, "No skills installed") {
		t.Fatalf("expected empty list message, got: %s", out)
	}
}

func TestAddLocalRepoYesInstallsAll(t *testing.T) {
	repoDir := t.TempDir()
	skillsDir := filepath.Join(repoDir, "skills")
	mustMkdir(t, skillsDir)
	mustWrite(t, filepath.Join(skillsDir, "alpha", "SKILL.md"), "# alpha\n")
	mustWrite(t, filepath.Join(skillsDir, "beta", "SKILL.md"), "# beta\n")

	destDir := t.TempDir()

	out := runSkillctl(t, "add", repoDir, "--dest", destDir, "--yes")
	if !strings.Contains(out, "Installing 2 skill(s)") {
		t.Fatalf("expected install output, got: %s", out)
	}

	out = runSkillctl(t, "list", "--dest", destDir)
	if !strings.Contains(out, "alpha") || !strings.Contains(out, "beta") {
		t.Fatalf("expected alpha and beta, got: %s", out)
	}
}

func TestDryRunDoesNotCreateDest(t *testing.T) {
	repoDir := t.TempDir()
	skillsDir := filepath.Join(repoDir, "skills")
	mustMkdir(t, skillsDir)
	mustWrite(t, filepath.Join(skillsDir, "de-dupe", "SKILL.md"), "# de-dupe\n")

	destBase := t.TempDir()
	destDir := filepath.Join(destBase, "dest")

	out := runSkillctl(t, "add", repoDir, "--dest", destDir, "--skill", "de-dupe", "--dry-run", "--yes")
	if !strings.Contains(out, "Dry run") {
		t.Fatalf("expected dry-run output, got: %s", out)
	}
	if _, err := os.Stat(destDir); err == nil || !os.IsNotExist(err) {
		t.Fatalf("expected dest directory not to be created")
	}
}

func TestVersionFlag(t *testing.T) {
	out := runSkillctl(t, "--version")
	if strings.TrimSpace(out) != "dev" {
		t.Fatalf("expected dev version, got: %s", out)
	}
}

func TestAddLocalRepoPromptSelectSubset(t *testing.T) {
	repoDir := t.TempDir()
	skillsDir := filepath.Join(repoDir, "skills")
	mustMkdir(t, skillsDir)
	mustWrite(t, filepath.Join(skillsDir, "alpha", "SKILL.md"), "# alpha\n")
	mustWrite(t, filepath.Join(skillsDir, "beta", "SKILL.md"), "# beta\n")

	destDir := t.TempDir()

	out := runSkillctlWithInput(t, "2\n", "add", repoDir, "--dest", destDir)
	if !strings.Contains(out, "Installed alpha") {
		t.Fatalf("expected alpha install, got: %s", out)
	}
	if _, err := os.Stat(filepath.Join(destDir, "beta")); err == nil {
		t.Fatalf("expected beta not to be installed")
	}
}

func TestAddLocalRepoPromptSelectAll(t *testing.T) {
	repoDir := t.TempDir()
	skillsDir := filepath.Join(repoDir, "skills")
	mustMkdir(t, skillsDir)
	mustWrite(t, filepath.Join(skillsDir, "alpha", "SKILL.md"), "# alpha\n")
	mustWrite(t, filepath.Join(skillsDir, "beta", "SKILL.md"), "# beta\n")

	destDir := t.TempDir()

	out := runSkillctlWithInput(t, "1\n", "add", repoDir, "--dest", destDir)
	if !strings.Contains(out, "Installing 2 skill(s)") {
		t.Fatalf("expected all install, got: %s", out)
	}
}

func TestConflictSkipAndOverwrite(t *testing.T) {
	repoDir := t.TempDir()
	skillsDir := filepath.Join(repoDir, "skills")
	mustMkdir(t, skillsDir)
	skillPath := filepath.Join(skillsDir, "alpha", "SKILL.md")
	mustWrite(t, skillPath, "v1\n")

	destDir := t.TempDir()

	runSkillctl(t, "add", repoDir, "--dest", destDir, "--skill", "alpha", "--yes")

	mustWrite(t, skillPath, "v2\n")

	out := runSkillctl(t, "add", repoDir, "--dest", destDir, "--skill", "alpha", "--skip", "--yes")
	if !strings.Contains(out, "Skipping alpha") {
		t.Fatalf("expected skip output, got: %s", out)
	}
	content, err := os.ReadFile(filepath.Join(destDir, "alpha", "SKILL.md"))
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}
	if string(content) != "v1\n" {
		t.Fatalf("expected v1 content, got: %s", string(content))
	}

	out = runSkillctl(t, "add", repoDir, "--dest", destDir, "--skill", "alpha", "--overwrite", "--yes")
	if !strings.Contains(out, "Installed alpha") {
		t.Fatalf("expected overwrite output, got: %s", out)
	}
	content, err = os.ReadFile(filepath.Join(destDir, "alpha", "SKILL.md"))
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}
	if string(content) != "v2\n" {
		t.Fatalf("expected v2 content, got: %s", string(content))
	}
}

func TestRemovePromptSelection(t *testing.T) {
	repoDir := t.TempDir()
	skillsDir := filepath.Join(repoDir, "skills")
	mustMkdir(t, skillsDir)
	mustWrite(t, filepath.Join(skillsDir, "alpha", "SKILL.md"), "# alpha\n")
	mustWrite(t, filepath.Join(skillsDir, "beta", "SKILL.md"), "# beta\n")

	destDir := t.TempDir()
	runSkillctl(t, "add", repoDir, "--dest", destDir, "--yes")

	out := runSkillctlWithInput(t, "1\n", "remove", "--dest", destDir)
	if !strings.Contains(out, "Removed alpha") {
		t.Fatalf("expected alpha removed, got: %s", out)
	}
	if _, err := os.Stat(filepath.Join(destDir, "beta", "SKILL.md")); err != nil {
		t.Fatalf("expected beta to remain, err: %v", err)
	}
}

func TestRemoveYesRequiresSkill(t *testing.T) {
	repoDir := t.TempDir()
	skillsDir := filepath.Join(repoDir, "skills")
	mustMkdir(t, skillsDir)
	mustWrite(t, filepath.Join(skillsDir, "alpha", "SKILL.md"), "# alpha\n")

	destDir := t.TempDir()
	runSkillctl(t, "add", repoDir, "--dest", destDir, "--yes")

	out := runSkillctlExpectError(t, "", "remove", "--dest", destDir, "--yes")
	if !strings.Contains(out, "--yes requires --skill") {
		t.Fatalf("expected yes requires skill error, got: %s", out)
	}
}

func TestRemoteDryRunDoesNotCreateDest(t *testing.T) {
	destBase := t.TempDir()
	destDir := filepath.Join(destBase, "dest")

	out := runSkillctl(t, "add", "saadjs/agent-skills", "--dest", destDir, "--skill", "de-dupe", "--dry-run", "--yes")
	if !strings.Contains(out, "Dry run") {
		t.Fatalf("expected dry-run output, got: %s", out)
	}
	if _, err := os.Stat(destDir); err == nil || !os.IsNotExist(err) {
		t.Fatalf("expected dest directory not to be created")
	}
}

func TestOverwriteSkipConflictFlagError(t *testing.T) {
	destDir := t.TempDir()
	out := runSkillctlExpectError(t, "", "add", ".", "--dest", destDir, "--overwrite", "--skip", "--skill", "de-dupe", "--yes")
	if !strings.Contains(out, "--overwrite and --skip") {
		t.Fatalf("expected overwrite/skip error, got: %s", out)
	}
}

func runSkillctl(t *testing.T, args ...string) string {
	t.Helper()
	return runSkillctlWithInput(t, "", args...)
}

func runSkillctlWithInput(t *testing.T, input string, args ...string) string {
	t.Helper()
	cmdArgs := append([]string{"run", "./cmd/skillctl"}, args...)
	cmd := exec.Command("go", cmdArgs...)
	cmd.Dir = repoRoot(t)
	cmd.Env = append(os.Environ(),
		"GOCACHE=/tmp/go-build",
		"GOPATH=/tmp/gopath",
		"GOMODCACHE=/tmp/gomodcache",
	)
	if input != "" {
		cmd.Stdin = strings.NewReader(input)
	}
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		t.Fatalf("skillctl failed: %v\nOutput:\n%s", err, out.String())
	}
	return out.String()
}

func runSkillctlExpectError(t *testing.T, input string, args ...string) string {
	t.Helper()
	cmdArgs := append([]string{"run", "./cmd/skillctl"}, args...)
	cmd := exec.Command("go", cmdArgs...)
	cmd.Dir = repoRoot(t)
	cmd.Env = append(os.Environ(),
		"GOCACHE=/tmp/go-build",
		"GOPATH=/tmp/gopath",
		"GOMODCACHE=/tmp/gomodcache",
	)
	if input != "" {
		cmd.Stdin = strings.NewReader(input)
	}
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	err := cmd.Run()
	if err == nil {
		t.Fatalf("expected error, got success. Output:\n%s", out.String())
	}
	return out.String()
}

func mustMkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write failed: %v", err)
	}
}

func repoRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd failed: %v", err)
	}
	return filepath.Clean(filepath.Join(wd, "..", ".."))
}
