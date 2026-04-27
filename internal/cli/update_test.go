package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/saadjs/skillctl/internal/config"
	"github.com/saadjs/skillctl/internal/security"
)

func TestUpdateReinstallsTrackedRemoteSkill(t *testing.T) {
	cfgDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", cfgDir)
	destDir := t.TempDir()
	sourceDir := t.TempDir()
	mustWrite(t, filepath.Join(sourceDir, "skills", "alpha", "SKILL.md"), "v1\n")

	origClone := cloneRepo
	defer func() {
		cloneRepo = origClone
	}()
	cloneRepo = fakeCloneRepo(t, sourceDir)

	if err := runAddCommand([]string{
		"--dest", destDir,
		"--skill", "alpha",
		"--yes",
		"owner/repo",
	}); err != nil {
		t.Fatalf("add failed: %v", err)
	}

	mustWrite(t, filepath.Join(sourceDir, "skills", "alpha", "SKILL.md"), "v2\n")
	if err := runUpdate(&updateOptions{yes: true}, nil); err != nil {
		t.Fatalf("update failed: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(destDir, "alpha", "SKILL.md"))
	if err != nil {
		t.Fatalf("read installed skill: %v", err)
	}
	if string(got) != "v2\n" {
		t.Fatalf("installed content = %q, want v2", string(got))
	}
	st, err := config.LoadState(config.StatePath())
	if err != nil {
		t.Fatalf("LoadState: %v", err)
	}
	entry := st.RemoteInstalls[remoteInstallKey(destDir, "alpha")]
	if entry.UpdatedAt == "" {
		t.Fatal("UpdatedAt should be set")
	}
}

func TestUpdateSpecificSkillOnly(t *testing.T) {
	cfgDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", cfgDir)
	destDir := t.TempDir()
	sourceDir := t.TempDir()
	mustWrite(t, filepath.Join(sourceDir, "skills", "alpha", "SKILL.md"), "alpha-v1\n")
	mustWrite(t, filepath.Join(sourceDir, "skills", "beta", "SKILL.md"), "beta-v1\n")

	origClone := cloneRepo
	defer func() {
		cloneRepo = origClone
	}()
	cloneRepo = fakeCloneRepo(t, sourceDir)

	if err := runAddCommand([]string{
		"--dest", destDir,
		"--yes",
		"owner/repo",
	}); err != nil {
		t.Fatalf("add failed: %v", err)
	}

	mustWrite(t, filepath.Join(sourceDir, "skills", "alpha", "SKILL.md"), "alpha-v2\n")
	mustWrite(t, filepath.Join(sourceDir, "skills", "beta", "SKILL.md"), "beta-v2\n")
	if err := runUpdate(&updateOptions{yes: true}, []string{"alpha"}); err != nil {
		t.Fatalf("update failed: %v", err)
	}

	alpha, err := os.ReadFile(filepath.Join(destDir, "alpha", "SKILL.md"))
	if err != nil {
		t.Fatalf("read alpha: %v", err)
	}
	beta, err := os.ReadFile(filepath.Join(destDir, "beta", "SKILL.md"))
	if err != nil {
		t.Fatalf("read beta: %v", err)
	}
	if string(alpha) != "alpha-v2\n" {
		t.Fatalf("alpha content = %q, want alpha-v2", string(alpha))
	}
	if string(beta) != "beta-v1\n" {
		t.Fatalf("beta content = %q, want beta-v1", string(beta))
	}
}

func TestUpdateMissingTrackedSkillErrors(t *testing.T) {
	cfgDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", cfgDir)
	if err := config.SaveState(config.StatePath(), &config.State{
		Tools: map[string]map[string]config.SkillState{},
		RemoteInstalls: map[string]config.RemoteInstallState{
			remoteInstallKey("/tmp/dest", "alpha"): {
				Source:      "owner/repo",
				Path:        "skills",
				Skills:      []string{"alpha"},
				Destination: "/tmp/dest",
			},
		},
	}); err != nil {
		t.Fatalf("SaveState: %v", err)
	}

	err := runUpdate(&updateOptions{yes: true}, []string{"beta"})
	if err == nil || !strings.Contains(err.Error(), "tracked remote skills not found: beta") {
		t.Fatalf("expected missing tracked skill error, got: %v", err)
	}
}

func TestUpdateDryRunDoesNotWriteDestinationOrState(t *testing.T) {
	cfgDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", cfgDir)
	destDir := t.TempDir()
	sourceDir := t.TempDir()
	mustWrite(t, filepath.Join(sourceDir, "skills", "alpha", "SKILL.md"), "v1\n")

	origClone := cloneRepo
	defer func() {
		cloneRepo = origClone
	}()
	cloneRepo = fakeCloneRepo(t, sourceDir)

	if err := runAddCommand([]string{
		"--dest", destDir,
		"--skill", "alpha",
		"--yes",
		"owner/repo",
	}); err != nil {
		t.Fatalf("add failed: %v", err)
	}
	stBefore, err := config.LoadState(config.StatePath())
	if err != nil {
		t.Fatalf("LoadState before: %v", err)
	}
	beforeUpdatedAt := stBefore.RemoteInstalls[remoteInstallKey(destDir, "alpha")].UpdatedAt

	mustWrite(t, filepath.Join(sourceDir, "skills", "alpha", "SKILL.md"), "v2\n")
	if err := runUpdate(&updateOptions{yes: true, dryRun: true}, nil); err != nil {
		t.Fatalf("update failed: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(destDir, "alpha", "SKILL.md"))
	if err != nil {
		t.Fatalf("read installed skill: %v", err)
	}
	if string(got) != "v1\n" {
		t.Fatalf("installed content = %q, want v1", string(got))
	}
	stAfter, err := config.LoadState(config.StatePath())
	if err != nil {
		t.Fatalf("LoadState after: %v", err)
	}
	afterUpdatedAt := stAfter.RemoteInstalls[remoteInstallKey(destDir, "alpha")].UpdatedAt
	if afterUpdatedAt != beforeUpdatedAt {
		t.Fatalf("UpdatedAt changed on dry-run: before=%q after=%q", beforeUpdatedAt, afterUpdatedAt)
	}
}

func TestUpdateSecurityFindingsRequireForceInYesMode(t *testing.T) {
	cfgDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", cfgDir)
	destDir := t.TempDir()
	sourceDir := t.TempDir()
	mustWrite(t, filepath.Join(sourceDir, "skills", "alpha", "SKILL.md"), "v1\n")

	origClone := cloneRepo
	origScan := scanRepo
	defer func() {
		cloneRepo = origClone
		scanRepo = origScan
	}()
	cloneRepo = fakeCloneRepo(t, sourceDir)

	if err := runAddCommand([]string{
		"--dest", destDir,
		"--skill", "alpha",
		"--yes",
		"owner/repo",
	}); err != nil {
		t.Fatalf("add failed: %v", err)
	}

	scanRepo = func(root string) (security.Report, error) {
		return security.Report{Findings: []security.Finding{{
			RuleID:   "test_rule",
			Severity: security.SeverityHigh,
			Path:     "SKILL.md",
			Line:     1,
			Message:  "blocked",
		}}}, nil
	}
	err := runUpdate(&updateOptions{yes: true}, nil)
	if err == nil || !strings.Contains(err.Error(), "security scan found potential malicious content") {
		t.Fatalf("expected security error, got: %v", err)
	}
	if err := runUpdate(&updateOptions{yes: true, force: true}, nil); err != nil {
		t.Fatalf("expected force update to succeed, got: %v", err)
	}
}

func runAddCommand(args []string) error {
	cmd := newAddCommand()
	positional, err := parseWithInterspersed(cmd.FlagSet, args)
	if err != nil {
		return err
	}
	return cmd.Run(positional)
}

func fakeCloneRepo(t *testing.T, sourceDir string) func(string, string) (string, error) {
	t.Helper()
	return func(repoURL, ref string) (string, error) {
		base := t.TempDir()
		target := filepath.Join(base, "repo")
		if err := copyTestTree(sourceDir, target); err != nil {
			return "", err
		}
		return target, nil
	}
}

func copyTestTree(src, dest string) error {
	return filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dest, rel)
		info, err := d.Info()
		if err != nil {
			return err
		}
		if d.IsDir() {
			return os.MkdirAll(target, info.Mode().Perm())
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		return os.WriteFile(target, data, info.Mode().Perm())
	})
}
