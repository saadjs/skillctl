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

	out := runSkillctl(t, "add", repoDir, "--dest", destDir, "--skill", "de-dupe", "--overwrite")
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

func TestDryRunDoesNotCreateDest(t *testing.T) {
	repoDir := t.TempDir()
	skillsDir := filepath.Join(repoDir, "skills")
	mustMkdir(t, skillsDir)
	mustWrite(t, filepath.Join(skillsDir, "de-dupe", "SKILL.md"), "# de-dupe\n")

	destBase := t.TempDir()
	destDir := filepath.Join(destBase, "dest")

	out := runSkillctl(t, "add", repoDir, "--dest", destDir, "--skill", "de-dupe", "--dry-run")
	if !strings.Contains(out, "Dry run") {
		t.Fatalf("expected dry-run output, got: %s", out)
	}
	if _, err := os.Stat(destDir); err == nil || !os.IsNotExist(err) {
		t.Fatalf("expected dest directory not to be created")
	}
}

func runSkillctl(t *testing.T, args ...string) string {
	t.Helper()
	cmdArgs := append([]string{"run", "./cmd/skillctl"}, args...)
	cmd := exec.Command("go", cmdArgs...)
	cmd.Dir = repoRoot(t)
	cmd.Env = append(os.Environ(),
		"GOCACHE=/tmp/go-build",
		"GOPATH=/tmp/gopath",
		"GOMODCACHE=/tmp/gomodcache",
	)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		t.Fatalf("skillctl failed: %v\nOutput:\n%s", err, out.String())
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
