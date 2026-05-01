package cli

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/saadjs/skillctl/internal/config"
	"github.com/saadjs/skillctl/internal/security"
	"github.com/saadjs/skillctl/internal/skills"
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

func TestAddListLocalSourcePrintsSkillsWithoutDestination(t *testing.T) {
	repoDir := t.TempDir()
	mustWriteTestFile(t, filepath.Join(repoDir, "skills", "beta", "SKILL.md"), "# beta\n")
	mustWriteTestFile(t, filepath.Join(repoDir, "skills", "alpha", "SKILL.md"), "# alpha\n")

	origScan := scanRepo
	defer func() {
		scanRepo = origScan
	}()
	scanRepo = func(root string) (security.Report, error) {
		return security.Report{}, errors.New("scan should not run in list mode")
	}

	restoreOutput, output := captureOutput(t)

	cmd := newAddCommand()
	positional, err := parseWithInterspersed(cmd.FlagSet, []string{
		"--list",
		"--yes",
		repoDir,
	})
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if err := cmd.Run(positional); err != nil {
		t.Fatalf("run failed: %v", err)
	}
	restoreOutput()
	if strings.TrimSpace(output.String()) != "alpha\nbeta" {
		t.Fatalf("expected sorted skill names, got: %q", output.String())
	}
}

func TestAddListLocalSourceFiltersSkills(t *testing.T) {
	repoDir := t.TempDir()
	mustWriteTestFile(t, filepath.Join(repoDir, "skills", "alpha", "SKILL.md"), "# alpha\n")
	mustWriteTestFile(t, filepath.Join(repoDir, "skills", "beta", "SKILL.md"), "# beta\n")

	restoreOutput, output := captureOutput(t)

	cmd := newAddCommand()
	positional, err := parseWithInterspersed(cmd.FlagSet, []string{
		"--list",
		"--skill", "alpha",
		repoDir,
	})
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if err := cmd.Run(positional); err != nil {
		t.Fatalf("run failed: %v", err)
	}
	restoreOutput()
	if strings.TrimSpace(output.String()) != "alpha" {
		t.Fatalf("expected alpha only, got: %q", output.String())
	}
}

func TestAddListLocalSourceReportsMissingSkill(t *testing.T) {
	repoDir := t.TempDir()
	mustWriteTestFile(t, filepath.Join(repoDir, "skills", "alpha", "SKILL.md"), "# alpha\n")

	cmd := newAddCommand()
	positional, err := parseWithInterspersed(cmd.FlagSet, []string{
		"--list",
		"--skill", "missing",
		repoDir,
	})
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	err = cmd.Run(positional)
	if err == nil || !strings.Contains(err.Error(), "skills not found: missing") {
		t.Fatalf("expected missing skill error, got: %v", err)
	}
}

func TestAddListRejectsOverwriteSkipConflict(t *testing.T) {
	repoDir := t.TempDir()
	mustWriteTestFile(t, filepath.Join(repoDir, "skills", "alpha", "SKILL.md"), "# alpha\n")

	cmd := newAddCommand()
	positional, err := parseWithInterspersed(cmd.FlagSet, []string{
		"--list",
		"--overwrite",
		"--skip",
		repoDir,
	})
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	err = cmd.Run(positional)
	if err == nil || !strings.Contains(err.Error(), "--overwrite and --skip cannot be used together") {
		t.Fatalf("expected overwrite/skip error, got: %v", err)
	}
}

func TestAddListRejectsInstallOnlyFlags(t *testing.T) {
	repoDir := t.TempDir()
	mustWriteTestFile(t, filepath.Join(repoDir, "skills", "alpha", "SKILL.md"), "# alpha\n")

	cmd := newAddCommand()
	positional, err := parseWithInterspersed(cmd.FlagSet, []string{
		"--list",
		"--dest", filepath.Join(t.TempDir(), "dest"),
		"--tool", "codex",
		"--scope", "project",
		"--force",
		"--dry-run",
		repoDir,
	})
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	err = cmd.Run(positional)
	if err == nil {
		t.Fatal("expected install-only flag error")
	}
	for _, flag := range []string{"--dest", "--tool", "--scope", "--force", "--dry-run", "--list"} {
		if !strings.Contains(err.Error(), flag) {
			t.Fatalf("expected error to mention %s, got: %v", flag, err)
		}
	}
}

func TestAddListRemoteSourceClonesSkipsScanAndCleansUp(t *testing.T) {
	cloneBase := filepath.Join(t.TempDir(), "clone")
	clonePath := filepath.Join(cloneBase, "repo")
	mustWriteTestFile(t, filepath.Join(clonePath, "skills", "alpha", "SKILL.md"), "# alpha\n")

	origClone := cloneRepo
	origScan := scanRepo
	defer func() {
		cloneRepo = origClone
		scanRepo = origScan
	}()

	cloneCalls := 0
	cloneRepo = func(repoURL, ref string) (string, error) {
		cloneCalls++
		if repoURL != "https://github.com/owner/repo.git" {
			t.Fatalf("unexpected repo URL: %s", repoURL)
		}
		if ref != "main" {
			t.Fatalf("unexpected ref: %s", ref)
		}
		return clonePath, nil
	}
	scanRepo = func(root string) (security.Report, error) {
		return security.Report{}, errors.New("scan should not run in list mode")
	}

	restoreOutput, output := captureOutput(t)

	cmd := newAddCommand()
	positional, err := parseWithInterspersed(cmd.FlagSet, []string{
		"--list",
		"--ref", "main",
		"owner/repo",
	})
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if err := cmd.Run(positional); err != nil {
		t.Fatalf("run failed: %v", err)
	}
	restoreOutput()
	if cloneCalls != 1 {
		t.Fatalf("expected clone called once, got %d", cloneCalls)
	}
	if strings.TrimSpace(output.String()) != "alpha" {
		t.Fatalf("expected alpha, got: %q", output.String())
	}
	if _, err := os.Stat(cloneBase); err == nil || !os.IsNotExist(err) {
		t.Fatalf("expected cloned repo temp dir removed, stat err: %v", err)
	}
}

func TestAddRemoteDryRunSkipsCloneAndScan(t *testing.T) {
	destDir := filepath.Join(t.TempDir(), "dest")

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
		return "", errors.New("clone should not run in remote dry-run mode")
	}
	scanRepo = func(root string) (security.Report, error) {
		scanCalls++
		return security.Report{}, errors.New("scan should not run in remote dry-run mode")
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
	if cloneCalls != 0 {
		t.Fatalf("expected clone not called, got %d", cloneCalls)
	}
	if scanCalls != 0 {
		t.Fatalf("expected scan not called, got %d", scanCalls)
	}
	restoreOutput()
	if !strings.Contains(output.String(), "Dry run: remote source was not cloned; security scan skipped.") {
		t.Fatalf("expected remote dry-run skip message, got: %s", output.String())
	}
	if !strings.Contains(output.String(), "Would install alpha") {
		t.Fatalf("expected skill preview in dry-run output, got: %s", output.String())
	}
	if _, err := os.Stat(destDir); err == nil || !os.IsNotExist(err) {
		t.Fatalf("expected dest directory not to be created")
	}
}

func TestAddRemoteDryRunRequiresSkill(t *testing.T) {
	destDir := filepath.Join(t.TempDir(), "dest")

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
		return "", errors.New("clone should not run in remote dry-run mode")
	}
	scanRepo = func(root string) (security.Report, error) {
		scanCalls++
		return security.Report{}, errors.New("scan should not run in remote dry-run mode")
	}

	cmd := newAddCommand()
	positional, err := parseWithInterspersed(cmd.FlagSet, []string{
		"--dest", destDir,
		"--dry-run",
		"--yes",
		"owner/repo",
	})
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	err = cmd.Run(positional)
	if err == nil || !strings.Contains(err.Error(), "--dry-run with remote source requires at least one --skill") {
		t.Fatalf("expected missing skill error, got: %v", err)
	}
	if cloneCalls != 0 {
		t.Fatalf("expected clone not called, got %d", cloneCalls)
	}
	if scanCalls != 0 {
		t.Fatalf("expected scan not called, got %d", scanCalls)
	}
}

func TestAddRemoteSSHURLIsPreservedForClone(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	destDir := filepath.Join(t.TempDir(), "dest")
	cloneBase := filepath.Join(t.TempDir(), "clone")
	clonePath := filepath.Join(cloneBase, "repo")
	if err := os.MkdirAll(filepath.Join(clonePath, "skills", "alpha"), 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(clonePath, "skills", "alpha", "SKILL.md"), []byte("# alpha\n"), 0o644); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	origClone := cloneRepo
	defer func() {
		cloneRepo = origClone
	}()

	var gotRepoURL string
	cloneRepo = func(repoURL, ref string) (string, error) {
		gotRepoURL = repoURL
		return clonePath, nil
	}

	cmd := newAddCommand()
	positional, err := parseWithInterspersed(cmd.FlagSet, []string{
		"--dest", destDir,
		"--skill", "alpha",
		"--yes",
		"git@github.com:owner/repo.git",
	})
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if err := cmd.Run(positional); err != nil {
		t.Fatalf("run failed: %v", err)
	}
	if gotRepoURL != "git@github.com:owner/repo.git" {
		t.Fatalf("expected SSH clone URL, got %s", gotRepoURL)
	}
}

func TestAddRepeatableToolInstallsToEachResolvedDestination(t *testing.T) {
	projectDir := t.TempDir()
	t.Chdir(projectDir)

	repoDir := t.TempDir()
	skillsDir := filepath.Join(repoDir, "skills")
	mustMkdir(t, skillsDir)
	mustWrite(t, filepath.Join(skillsDir, "alpha", "SKILL.md"), "# alpha\n")

	cmd := newAddCommand()
	positional, err := parseWithInterspersed(cmd.FlagSet, []string{
		repoDir,
		"--tool", "codex",
		"--tool", "claude",
		"--scope", "project",
		"--skill", "alpha",
		"--yes",
	})
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if err := cmd.Run(positional); err != nil {
		t.Fatalf("run failed: %v", err)
	}

	for _, dest := range []string{
		filepath.Join(projectDir, ".codex", "skills", "alpha", "SKILL.md"),
		filepath.Join(projectDir, ".claude", "skills", "alpha", "SKILL.md"),
	} {
		if _, err := os.Stat(dest); err != nil {
			t.Fatalf("expected skill installed at %s: %v", dest, err)
		}
	}
}

func TestAddRejectsDestWithTool(t *testing.T) {
	repoDir := t.TempDir()
	skillsDir := filepath.Join(repoDir, "skills")
	mustMkdir(t, skillsDir)
	mustWrite(t, filepath.Join(skillsDir, "alpha", "SKILL.md"), "# alpha\n")

	cmd := newAddCommand()
	positional, err := parseWithInterspersed(cmd.FlagSet, []string{
		repoDir,
		"--dest", filepath.Join(t.TempDir(), "dest"),
		"--tool", "codex",
		"--scope", "project",
		"--skill", "alpha",
		"--yes",
	})
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	err = cmd.Run(positional)
	if err == nil || !strings.Contains(err.Error(), "--dest cannot be combined with --tool") {
		t.Fatalf("expected --dest/--tool error, got: %v", err)
	}
}

func TestAddYesWithToolRequiresScope(t *testing.T) {
	repoDir := t.TempDir()
	skillsDir := filepath.Join(repoDir, "skills")
	mustMkdir(t, skillsDir)
	mustWrite(t, filepath.Join(skillsDir, "alpha", "SKILL.md"), "# alpha\n")

	cmd := newAddCommand()
	positional, err := parseWithInterspersed(cmd.FlagSet, []string{
		repoDir,
		"--tool", "codex",
		"--skill", "alpha",
		"--yes",
	})
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	err = cmd.Run(positional)
	if err == nil || !strings.Contains(err.Error(), "--yes requires --scope when --tool is used") {
		t.Fatalf("expected --yes/--scope error, got: %v", err)
	}
}

func TestAddRepeatableToolDedupesDestinations(t *testing.T) {
	projectDir := t.TempDir()
	t.Chdir(projectDir)

	destinations, err := resolveAddDestinations([]string{"codex", "codex"}, "project", "", true)
	if err != nil {
		t.Fatalf("resolve failed: %v", err)
	}
	want := filepath.Join(projectDir, ".codex", "skills")
	if len(destinations) != 1 || destinations[0] != want {
		t.Fatalf("expected one deduped destination %s, got: %v", want, destinations)
	}
}

func TestAddRepeatableToolDryRunPreviewsEachDestination(t *testing.T) {
	projectDir := t.TempDir()
	t.Chdir(projectDir)

	repoDir := t.TempDir()
	skillsDir := filepath.Join(repoDir, "skills")
	mustMkdir(t, skillsDir)
	mustWrite(t, filepath.Join(skillsDir, "alpha", "SKILL.md"), "# alpha\n")

	restoreOutput, output := captureOutput(t)

	cmd := newAddCommand()
	positional, err := parseWithInterspersed(cmd.FlagSet, []string{
		repoDir,
		"--tool", "codex",
		"--tool", "claude",
		"--scope", "project",
		"--skill", "alpha",
		"--dry-run",
		"--yes",
	})
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if err := cmd.Run(positional); err != nil {
		t.Fatalf("run failed: %v", err)
	}
	restoreOutput()

	codexDest := filepath.Join(projectDir, ".codex", "skills")
	claudeDest := filepath.Join(projectDir, ".claude", "skills")
	out := output.String()
	if !strings.Contains(out, "Dry run: would install 1 skill(s) to "+codexDest) {
		t.Fatalf("expected codex dry-run destination, got: %s", out)
	}
	if !strings.Contains(out, "Dry run: would install 1 skill(s) to "+claudeDest) {
		t.Fatalf("expected claude dry-run destination, got: %s", out)
	}
	for _, path := range []string{
		filepath.Join(projectDir, ".codex"),
		filepath.Join(projectDir, ".claude"),
	} {
		if _, err := os.Stat(path); err == nil || !os.IsNotExist(err) {
			t.Fatalf("expected dry-run not to create %s", path)
		}
	}
}

func TestAddRepeatableToolSkipAppliesPerDestination(t *testing.T) {
	projectDir := t.TempDir()
	t.Chdir(projectDir)

	repoDir := t.TempDir()
	skillsDir := filepath.Join(repoDir, "skills")
	mustMkdir(t, skillsDir)
	mustWrite(t, filepath.Join(skillsDir, "alpha", "SKILL.md"), "# alpha\nnew\n")

	existingPath := filepath.Join(projectDir, ".claude", "skills", "alpha", "SKILL.md")
	mustWrite(t, existingPath, "# alpha\nexisting\n")

	cmd := newAddCommand()
	positional, err := parseWithInterspersed(cmd.FlagSet, []string{
		repoDir,
		"--tool", "codex",
		"--tool", "claude",
		"--scope", "project",
		"--skill", "alpha",
		"--skip",
		"--yes",
	})
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if err := cmd.Run(positional); err != nil {
		t.Fatalf("run failed: %v", err)
	}

	if _, err := os.Stat(filepath.Join(projectDir, ".codex", "skills", "alpha", "SKILL.md")); err != nil {
		t.Fatalf("expected codex skill installed: %v", err)
	}
	contents, err := os.ReadFile(existingPath)
	if err != nil {
		t.Fatalf("read existing skill failed: %v", err)
	}
	if string(contents) != "# alpha\nexisting\n" {
		t.Fatalf("expected claude skill to be skipped, got: %q", string(contents))
	}
}

func TestPromptConflictIncludesDestination(t *testing.T) {
	restoreStdin := withStdin(t, "2\n")
	defer restoreStdin()
	restoreOutput, output := captureOutput(t)

	choice, err := promptConflict("alpha", "/tmp/skills")
	restoreOutput()
	if err != nil {
		t.Fatalf("prompt failed: %v", err)
	}
	if choice != "skip" {
		t.Fatalf("expected skip, got: %s", choice)
	}
	if !strings.Contains(output.String(), "Skill alpha exists in /tmp/skills. Choose action") {
		t.Fatalf("expected destination in prompt, got: %s", output.String())
	}
}

func TestAddRemoteCloneIsCleanedUpOnSecurityBlock(t *testing.T) {
	destDir := filepath.Join(t.TempDir(), "dest")
	cloneBase := filepath.Join(t.TempDir(), "clone")
	clonePath := filepath.Join(cloneBase, "repo")
	if err := os.MkdirAll(clonePath, 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}

	origClone := cloneRepo
	origScan := scanRepo
	defer func() {
		cloneRepo = origClone
		scanRepo = origScan
	}()

	cloneRepo = func(repoURL, ref string) (string, error) {
		return clonePath, nil
	}
	scanRepo = func(root string) (security.Report, error) {
		return security.Report{
			Findings: []security.Finding{
				{
					RuleID:   "test_rule",
					Severity: security.SeverityHigh,
					Path:     "skills/alpha/SKILL.md",
					Line:     1,
					Message:  "test finding",
				},
			},
		}, nil
	}

	cmd := newAddCommand()
	positional, err := parseWithInterspersed(cmd.FlagSet, []string{
		"--dest", destDir,
		"--yes",
		"owner/repo",
	})
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	err = cmd.Run(positional)
	if err == nil || !strings.Contains(err.Error(), "security scan found potential malicious content") {
		t.Fatalf("expected security block error, got: %v", err)
	}
	if _, err := os.Stat(cloneBase); err == nil || !os.IsNotExist(err) {
		t.Fatalf("expected cloned repo temp dir removed, stat err: %v", err)
	}
}

func TestAddRemoteRecordsInstallState(t *testing.T) {
	cfgDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", cfgDir)
	destDir := filepath.Join(t.TempDir(), "dest")
	sourceDir := t.TempDir()
	mustWrite(t, filepath.Join(sourceDir, "skills", "alpha", "SKILL.md"), "v1\n")
	mustWrite(t, filepath.Join(sourceDir, "skills", "beta", "SKILL.md"), "v1\n")

	origClone := cloneRepo
	defer func() {
		cloneRepo = origClone
	}()
	cloneRepo = fakeCloneRepo(t, sourceDir)

	cmd := newAddCommand()
	positional, err := parseWithInterspersed(cmd.FlagSet, []string{
		"--dest", destDir,
		"--skill", "alpha",
		"--ref", "main",
		"--yes",
		"owner/repo",
	})
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if err := cmd.Run(positional); err != nil {
		t.Fatalf("run failed: %v", err)
	}

	st, err := config.LoadState(config.StatePath())
	if err != nil {
		t.Fatalf("LoadState: %v", err)
	}
	entry, ok := st.RemoteInstalls[remoteInstallKey(destDir, "alpha")]
	if !ok {
		t.Fatalf("missing remote install entry: %#v", st.RemoteInstalls)
	}
	if entry.Source != "owner/repo" {
		t.Errorf("Source = %q, want owner/repo", entry.Source)
	}
	if entry.Ref != "main" {
		t.Errorf("Ref = %q, want main", entry.Ref)
	}
	if entry.Path != "skills" {
		t.Errorf("Path = %q, want skills", entry.Path)
	}
	if entry.Destination != destDir {
		t.Errorf("Destination = %q, want %q", entry.Destination, destDir)
	}
	if len(entry.Skills) != 1 || entry.Skills[0] != "alpha" {
		t.Errorf("Skills = %v, want [alpha]", entry.Skills)
	}
	if entry.InstalledAt == "" || entry.UpdatedAt == "" {
		t.Errorf("expected timestamps, got installed=%q updated=%q", entry.InstalledAt, entry.UpdatedAt)
	}
	if _, ok := st.RemoteInstalls[remoteInstallKey(destDir, "beta")]; ok {
		t.Fatal("beta should not be tracked")
	}
}

func TestAddDoesNotTrackDryRunSkippedFailedOrLocalInstalls(t *testing.T) {
	t.Run("dry-run", func(t *testing.T) {
		cfgDir := t.TempDir()
		t.Setenv("XDG_CONFIG_HOME", cfgDir)
		destDir := filepath.Join(t.TempDir(), "dest")

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
		assertNoRemoteInstalls(t)
	})

	t.Run("skipped", func(t *testing.T) {
		cfgDir := t.TempDir()
		t.Setenv("XDG_CONFIG_HOME", cfgDir)
		destDir := t.TempDir()
		sourceDir := t.TempDir()
		mustWrite(t, filepath.Join(destDir, "alpha", "SKILL.md"), "installed\n")
		mustWrite(t, filepath.Join(sourceDir, "skills", "alpha", "SKILL.md"), "remote\n")

		origClone := cloneRepo
		defer func() {
			cloneRepo = origClone
		}()
		cloneRepo = fakeCloneRepo(t, sourceDir)

		cmd := newAddCommand()
		positional, err := parseWithInterspersed(cmd.FlagSet, []string{
			"--dest", destDir,
			"--skill", "alpha",
			"--skip",
			"--yes",
			"owner/repo",
		})
		if err != nil {
			t.Fatalf("parse failed: %v", err)
		}
		if err := cmd.Run(positional); err != nil {
			t.Fatalf("run failed: %v", err)
		}
		assertNoRemoteInstalls(t)
	})

	t.Run("failed", func(t *testing.T) {
		cfgDir := t.TempDir()
		t.Setenv("XDG_CONFIG_HOME", cfgDir)
		destDir := t.TempDir()
		sourceDir := t.TempDir()
		mustWrite(t, filepath.Join(sourceDir, "skills", "alpha", "SKILL.md"), "remote\n")

		origClone := cloneRepo
		origScan := scanRepo
		defer func() {
			cloneRepo = origClone
			scanRepo = origScan
		}()
		cloneRepo = fakeCloneRepo(t, sourceDir)
		scanRepo = func(root string) (security.Report, error) {
			return security.Report{Findings: []security.Finding{{
				RuleID:   "test_rule",
				Severity: security.SeverityHigh,
				Path:     "SKILL.md",
				Line:     1,
				Message:  "blocked",
			}}}, nil
		}

		cmd := newAddCommand()
		positional, err := parseWithInterspersed(cmd.FlagSet, []string{
			"--dest", destDir,
			"--skill", "alpha",
			"--yes",
			"owner/repo",
		})
		if err != nil {
			t.Fatalf("parse failed: %v", err)
		}
		err = cmd.Run(positional)
		if err == nil {
			t.Fatal("expected add to fail")
		}
		assertNoRemoteInstalls(t)
	})

	t.Run("local", func(t *testing.T) {
		cfgDir := t.TempDir()
		t.Setenv("XDG_CONFIG_HOME", cfgDir)
		destDir := t.TempDir()
		sourceDir := t.TempDir()
		mustWrite(t, filepath.Join(sourceDir, "skills", "alpha", "SKILL.md"), "local\n")

		cmd := newAddCommand()
		positional, err := parseWithInterspersed(cmd.FlagSet, []string{
			"--dest", destDir,
			"--skill", "alpha",
			"--yes",
			sourceDir,
		})
		if err != nil {
			t.Fatalf("parse failed: %v", err)
		}
		if err := cmd.Run(positional); err != nil {
			t.Fatalf("run failed: %v", err)
		}
		assertNoRemoteInstalls(t)
	})
}

func assertNoRemoteInstalls(t *testing.T) {
	t.Helper()
	st, err := config.LoadState(config.StatePath())
	if err != nil {
		t.Fatalf("LoadState: %v", err)
	}
	if len(st.RemoteInstalls) != 0 {
		t.Fatalf("expected no remote installs, got %#v", st.RemoteInstalls)
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
