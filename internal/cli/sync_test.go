package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/saadjs/agent-skills/internal/config"
	"github.com/saadjs/agent-skills/internal/paths"
	"github.com/saadjs/agent-skills/internal/security"
)

func setupSyncTest(t *testing.T) (sourceDir string, destDirs map[paths.Tool]string, cleanup func()) {
	t.Helper()

	// Create source with two skills
	sourceDir = filepath.Join(t.TempDir(), "skills")
	mustWriteTestFile(t, filepath.Join(sourceDir, "alpha", "SKILL.md"), "---\nname: alpha\ndescription: Alpha skill\n---\nAlpha instructions.")
	mustWriteTestFile(t, filepath.Join(sourceDir, "beta", "SKILL.md"), "---\nname: beta\ndescription: Beta skill\n---\nBeta instructions.")
	mustWriteTestFile(t, filepath.Join(sourceDir, "beta", "scripts", "run.sh"), "#!/bin/bash\necho hello")

	// Create destination parent dirs (simulating installed tools)
	destDirs = map[paths.Tool]string{}
	for _, tool := range []paths.Tool{paths.ToolAgents, paths.ToolClaude} {
		base := filepath.Join(t.TempDir(), string(tool))
		if err := os.MkdirAll(base, 0o755); err != nil {
			t.Fatal(err)
		}
		destDirs[tool] = filepath.Join(base, "skills")
	}

	// Stub resolvePath to use temp dirs
	origResolve := resolvePath
	resolvePath = func(tool paths.Tool, scope paths.Scope, cwd string) (string, error) {
		if dest, ok := destDirs[tool]; ok {
			return dest, nil
		}
		// Return a path whose parent doesn't exist → tool gets skipped
		return filepath.Join(t.TempDir(), "nonexistent", string(tool), "skills"), nil
	}

	// Stub scanRepo to pass
	origScan := scanRepo
	scanRepo = func(root string) (security.Report, error) {
		return security.Report{}, nil
	}

	cleanup = func() {
		resolvePath = origResolve
		scanRepo = origScan
	}
	return
}

func TestSyncCopiesToMultipleTools(t *testing.T) {
	sourceDir, destDirs, cleanup := setupSyncTest(t)
	defer cleanup()

	cfgDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", cfgDir)

	cfg := &config.Config{Source: sourceDir, Tools: []string{"agents", "claude"}}
	if err := config.SaveConfig(config.ConfigPath(), cfg); err != nil {
		t.Fatal(err)
	}

	restoreOutput, _ := captureOutput(t)
	opts := &syncOptions{yes: true, all: true}
	if err := runSync(opts); err != nil {
		restoreOutput()
		t.Fatalf("runSync: %v", err)
	}
	restoreOutput()

	for tool, dest := range destDirs {
		for _, skill := range []string{"alpha", "beta"} {
			skillMd := filepath.Join(dest, skill, "SKILL.md")
			if _, err := os.Stat(skillMd); err != nil {
				t.Errorf("expected %s installed at %s (%s)", skill, dest, tool)
			}
		}
		// Check multi-file skill
		scriptPath := filepath.Join(dest, "beta", "scripts", "run.sh")
		if _, err := os.Stat(scriptPath); err != nil {
			t.Errorf("expected beta/scripts/run.sh at %s (%s)", dest, tool)
		}
	}
}

func TestSyncSkipsMissingToolDirs(t *testing.T) {
	sourceDir, _, cleanup := setupSyncTest(t)
	defer cleanup()

	cfgDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", cfgDir)

	// codex dest has no parent dir → should be skipped
	cfg := &config.Config{Source: sourceDir, Tools: []string{"agents", "codex"}}
	if err := config.SaveConfig(config.ConfigPath(), cfg); err != nil {
		t.Fatal(err)
	}

	restoreOutput, output := captureOutput(t)
	opts := &syncOptions{yes: true, all: true}
	if err := runSync(opts); err != nil {
		restoreOutput()
		t.Fatalf("runSync: %v", err)
	}
	restoreOutput()

	if !strings.Contains(output.String(), "Skipping codex") {
		t.Errorf("expected skip message for codex, got: %s", output.String())
	}
}

func TestSyncFilterByTool(t *testing.T) {
	sourceDir, destDirs, cleanup := setupSyncTest(t)
	defer cleanup()

	cfgDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", cfgDir)

	cfg := &config.Config{Source: sourceDir, Tools: []string{"agents", "claude"}}
	if err := config.SaveConfig(config.ConfigPath(), cfg); err != nil {
		t.Fatal(err)
	}

	restoreOutput, _ := captureOutput(t)
	opts := &syncOptions{
		yes:   true,
		all:   true,
		tools: multiString{values: []string{"claude"}},
	}
	if err := runSync(opts); err != nil {
		restoreOutput()
		t.Fatalf("runSync: %v", err)
	}
	restoreOutput()

	// Claude should have skills
	claudeDest := destDirs[paths.ToolClaude]
	if _, err := os.Stat(filepath.Join(claudeDest, "alpha", "SKILL.md")); err != nil {
		t.Error("expected alpha at claude dest")
	}

	// Agents should NOT have skills (not in --tool override)
	agentsDest := destDirs[paths.ToolAgents]
	if _, err := os.Stat(filepath.Join(agentsDest, "alpha", "SKILL.md")); err == nil {
		t.Error("agents should not have skills when --tool claude was used")
	}
}

func TestSyncFilterBySkill(t *testing.T) {
	sourceDir, destDirs, cleanup := setupSyncTest(t)
	defer cleanup()

	cfgDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", cfgDir)

	cfg := &config.Config{Source: sourceDir, Tools: []string{"agents"}}
	if err := config.SaveConfig(config.ConfigPath(), cfg); err != nil {
		t.Fatal(err)
	}

	restoreOutput, _ := captureOutput(t)
	opts := &syncOptions{
		yes:    true,
		all:    true,
		skills: multiString{values: []string{"alpha"}},
	}
	if err := runSync(opts); err != nil {
		restoreOutput()
		t.Fatalf("runSync: %v", err)
	}
	restoreOutput()

	agentsDest := destDirs[paths.ToolAgents]
	if _, err := os.Stat(filepath.Join(agentsDest, "alpha", "SKILL.md")); err != nil {
		t.Error("expected alpha at agents dest")
	}
	if _, err := os.Stat(filepath.Join(agentsDest, "beta", "SKILL.md")); err == nil {
		t.Error("beta should not be synced when --skill alpha was used")
	}
}

func TestSyncSkipsUnchangedSkills(t *testing.T) {
	sourceDir, destDirs, cleanup := setupSyncTest(t)
	defer cleanup()

	cfgDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", cfgDir)

	cfg := &config.Config{Source: sourceDir, Tools: []string{"agents"}}
	if err := config.SaveConfig(config.ConfigPath(), cfg); err != nil {
		t.Fatal(err)
	}

	// First sync
	restoreOutput, _ := captureOutput(t)
	opts := &syncOptions{yes: true, all: true}
	if err := runSync(opts); err != nil {
		restoreOutput()
		t.Fatalf("first sync: %v", err)
	}
	restoreOutput()

	// Remove synced file to detect if second sync copies again
	agentsDest := destDirs[paths.ToolAgents]
	os.RemoveAll(filepath.Join(agentsDest, "alpha"))

	// Second sync without --all should skip (checksum matches)
	restoreOutput, output := captureOutput(t)
	opts = &syncOptions{yes: true}
	if err := runSync(opts); err != nil {
		restoreOutput()
		t.Fatalf("second sync: %v", err)
	}
	restoreOutput()

	if !strings.Contains(output.String(), "unchanged alpha") {
		t.Errorf("expected unchanged message for alpha, got: %s", output.String())
	}
	// alpha should still be removed since it was skipped
	if _, err := os.Stat(filepath.Join(agentsDest, "alpha", "SKILL.md")); err == nil {
		t.Error("alpha should not be re-copied when unchanged")
	}
}

func TestSyncAllIgnoresChecksums(t *testing.T) {
	sourceDir, destDirs, cleanup := setupSyncTest(t)
	defer cleanup()

	cfgDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", cfgDir)

	cfg := &config.Config{Source: sourceDir, Tools: []string{"agents"}}
	if err := config.SaveConfig(config.ConfigPath(), cfg); err != nil {
		t.Fatal(err)
	}

	// First sync
	restoreOutput, _ := captureOutput(t)
	if err := runSync(&syncOptions{yes: true, all: true}); err != nil {
		restoreOutput()
		t.Fatal(err)
	}
	restoreOutput()

	// Remove a synced file
	agentsDest := destDirs[paths.ToolAgents]
	os.RemoveAll(filepath.Join(agentsDest, "alpha"))

	// Second sync with --all should re-copy
	restoreOutput, _ = captureOutput(t)
	if err := runSync(&syncOptions{yes: true, all: true}); err != nil {
		restoreOutput()
		t.Fatal(err)
	}
	restoreOutput()

	if _, err := os.Stat(filepath.Join(agentsDest, "alpha", "SKILL.md")); err != nil {
		t.Error("alpha should be re-copied with --all")
	}
}

func TestSyncDryRunMakesNoChanges(t *testing.T) {
	sourceDir, destDirs, cleanup := setupSyncTest(t)
	defer cleanup()

	cfgDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", cfgDir)

	cfg := &config.Config{Source: sourceDir, Tools: []string{"agents"}}
	if err := config.SaveConfig(config.ConfigPath(), cfg); err != nil {
		t.Fatal(err)
	}

	restoreOutput, output := captureOutput(t)
	opts := &syncOptions{yes: true, all: true, dryRun: true}
	if err := runSync(opts); err != nil {
		restoreOutput()
		t.Fatalf("runSync: %v", err)
	}
	restoreOutput()

	agentsDest := destDirs[paths.ToolAgents]
	if _, err := os.Stat(filepath.Join(agentsDest, "alpha")); err == nil {
		t.Error("dry-run should not create files")
	}
	if !strings.Contains(output.String(), "would sync") {
		t.Errorf("expected dry-run output, got: %s", output.String())
	}
	// State file should not be created
	if _, err := os.Stat(config.StatePath()); err == nil {
		t.Error("dry-run should not create state file")
	}
}

func TestSyncSecurityScanBlocks(t *testing.T) {
	sourceDir, destDirs, cleanup := setupSyncTest(t)
	defer cleanup()

	// Override scan to return findings
	scanRepo = func(root string) (security.Report, error) {
		return security.Report{
			Findings: []security.Finding{
				{RuleID: "test", Severity: security.SeverityHigh, Message: "bad"},
			},
		}, nil
	}

	cfgDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", cfgDir)

	cfg := &config.Config{Source: sourceDir, Tools: []string{"agents"}}
	if err := config.SaveConfig(config.ConfigPath(), cfg); err != nil {
		t.Fatal(err)
	}

	restoreOutput, _ := captureOutput(t)
	opts := &syncOptions{yes: true, all: true}
	err := runSync(opts)
	restoreOutput()

	if err == nil || !strings.Contains(err.Error(), "security scan found potential malicious content") {
		t.Fatalf("expected security block, got: %v", err)
	}
	// Verify no files were copied
	agentsDest := destDirs[paths.ToolAgents]
	if _, err := os.Stat(filepath.Join(agentsDest, "alpha")); err == nil {
		t.Error("no skills should be copied when security scan blocks")
	}
	// Verify no state was written
	if _, err := os.Stat(config.StatePath()); !os.IsNotExist(err) {
		t.Error("state file should not exist when security scan blocks")
	}
}

func TestSyncMissingSourceDir(t *testing.T) {
	cfgDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", cfgDir)

	origScan := scanRepo
	defer func() { scanRepo = origScan }()

	cfg := &config.Config{Source: "/nonexistent/path", Tools: []string{"agents"}}
	if err := config.SaveConfig(config.ConfigPath(), cfg); err != nil {
		t.Fatal(err)
	}

	opts := &syncOptions{yes: true}
	err := runSync(opts)
	if err == nil || !strings.Contains(err.Error(), "source directory does not exist") {
		t.Fatalf("expected source dir error, got: %v", err)
	}
}

func TestSyncYesWithoutConfigErrors(t *testing.T) {
	cfgDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", cfgDir)

	opts := &syncOptions{yes: true}
	err := runSync(opts)
	if err == nil || !strings.Contains(err.Error(), "no config found") {
		t.Fatalf("expected no config error, got: %v", err)
	}
}

func TestSyncYesWithSourceAndToolFlagsSucceeds(t *testing.T) {
	sourceDir, destDirs, cleanup := setupSyncTest(t)
	defer cleanup()

	cfgDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", cfgDir)

	// No config file exists, but --source and --tool are provided
	restoreOutput, _ := captureOutput(t)
	opts := &syncOptions{
		yes:    true,
		all:    true,
		source: sourceDir,
		tools:  multiString{values: []string{"agents"}},
	}
	if err := runSync(opts); err != nil {
		restoreOutput()
		t.Fatalf("expected success with --source and --tool flags, got: %v", err)
	}
	restoreOutput()

	agentsDest := destDirs[paths.ToolAgents]
	if _, err := os.Stat(filepath.Join(agentsDest, "alpha", "SKILL.md")); err != nil {
		t.Error("expected alpha synced to agents with flag overrides")
	}
}

func TestSyncSkipsBootstrapWhenFlagsProvided(t *testing.T) {
	sourceDir, destDirs, cleanup := setupSyncTest(t)
	defer cleanup()

	cfgDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", cfgDir)

	// No config file, no --yes flag. Before the fix, this would call
	// loadOrCreateConfig and prompt for source/tools even though the user
	// already supplied them, persisting the interactive answers to config.yaml.
	opts := &syncOptions{
		all:    true,
		source: sourceDir,
		tools:  multiString{values: []string{"agents"}},
	}
	restoreOutput, _ := captureOutput(t)
	if err := runSync(opts); err != nil {
		restoreOutput()
		t.Fatalf("runSync: %v", err)
	}
	restoreOutput()

	agentsDest := destDirs[paths.ToolAgents]
	if _, err := os.Stat(filepath.Join(agentsDest, "alpha", "SKILL.md")); err != nil {
		t.Error("expected alpha synced via flag overrides")
	}
	// The bootstrap flow must not run, so no config file should be auto-created.
	if _, err := os.Stat(config.ConfigPath()); !os.IsNotExist(err) {
		t.Errorf("config.yaml should not be created when flags fully specify the sync, err=%v", err)
	}
}

func TestSyncSourceFlagOverridesConfig(t *testing.T) {
	_, destDirs, cleanup := setupSyncTest(t)
	defer cleanup()

	cfgDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", cfgDir)

	// Config points to a nonexistent source, but --source overrides it
	altSource := filepath.Join(t.TempDir(), "alt-skills")
	mustWriteTestFile(t, filepath.Join(altSource, "gamma", "SKILL.md"), "---\nname: gamma\ndescription: Gamma skill\n---\nGamma.")

	cfg := &config.Config{Source: "/nonexistent", Tools: []string{"agents"}}
	if err := config.SaveConfig(config.ConfigPath(), cfg); err != nil {
		t.Fatal(err)
	}

	restoreOutput, _ := captureOutput(t)
	opts := &syncOptions{yes: true, all: true, source: altSource}
	if err := runSync(opts); err != nil {
		restoreOutput()
		t.Fatalf("runSync: %v", err)
	}
	restoreOutput()

	agentsDest := destDirs[paths.ToolAgents]
	if _, err := os.Stat(filepath.Join(agentsDest, "gamma", "SKILL.md")); err != nil {
		t.Error("expected gamma from alt source at agents dest")
	}
}

func TestSyncSecurityScanForceBypass(t *testing.T) {
	sourceDir, destDirs, cleanup := setupSyncTest(t)
	defer cleanup()

	scanRepo = func(root string) (security.Report, error) {
		return security.Report{
			Findings: []security.Finding{
				{RuleID: "test", Severity: security.SeverityHigh, Message: "bad"},
			},
		}, nil
	}

	cfgDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", cfgDir)

	cfg := &config.Config{Source: sourceDir, Tools: []string{"agents"}}
	if err := config.SaveConfig(config.ConfigPath(), cfg); err != nil {
		t.Fatal(err)
	}

	restoreOutput, _ := captureOutput(t)
	opts := &syncOptions{yes: true, all: true, force: true}
	if err := runSync(opts); err != nil {
		restoreOutput()
		t.Fatalf("expected force to bypass security, got: %v", err)
	}
	restoreOutput()

	agentsDest := destDirs[paths.ToolAgents]
	if _, err := os.Stat(filepath.Join(agentsDest, "alpha", "SKILL.md")); err != nil {
		t.Error("expected alpha synced despite security findings with --force")
	}
}

func TestSyncEmptySourceConfig(t *testing.T) {
	cfgDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", cfgDir)

	cfg := &config.Config{Source: "", Tools: []string{"agents"}}
	if err := config.SaveConfig(config.ConfigPath(), cfg); err != nil {
		t.Fatal(err)
	}

	opts := &syncOptions{yes: true}
	err := runSync(opts)
	if err == nil || !strings.Contains(err.Error(), "source directory is not configured") {
		t.Fatalf("expected empty source error, got: %v", err)
	}
}

func TestSyncNoStateUpdateWhenAllToolsSkipped(t *testing.T) {
	sourceDir, _, cleanup := setupSyncTest(t)
	defer cleanup()

	cfgDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", cfgDir)

	// Only configure a tool whose parent dir won't exist
	cfg := &config.Config{Source: sourceDir, Tools: []string{"codex"}}
	if err := config.SaveConfig(config.ConfigPath(), cfg); err != nil {
		t.Fatal(err)
	}

	restoreOutput, _ := captureOutput(t)
	opts := &syncOptions{yes: true, all: true}
	if err := runSync(opts); err != nil {
		restoreOutput()
		t.Fatal(err)
	}
	restoreOutput()

	// State file should not exist since no tools received any skills
	if _, err := os.Stat(config.StatePath()); !os.IsNotExist(err) {
		t.Error("state file should not exist when all tools were skipped")
	}
}

func TestSyncUpdatesStateFile(t *testing.T) {
	sourceDir, _, cleanup := setupSyncTest(t)
	defer cleanup()

	cfgDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", cfgDir)

	cfg := &config.Config{Source: sourceDir, Tools: []string{"agents"}}
	if err := config.SaveConfig(config.ConfigPath(), cfg); err != nil {
		t.Fatal(err)
	}

	restoreOutput, _ := captureOutput(t)
	if err := runSync(&syncOptions{yes: true, all: true}); err != nil {
		restoreOutput()
		t.Fatal(err)
	}
	restoreOutput()

	st, err := config.LoadState(config.StatePath())
	if err != nil {
		t.Fatal(err)
	}
	if st.LastSync == "" {
		t.Error("expected last_sync to be set")
	}
	agentsState, ok := st.Tools["agents"]
	if !ok {
		t.Fatal("expected agents tool in state")
	}
	if _, ok := agentsState["alpha"]; !ok {
		t.Error("expected alpha in agents state")
	}
	if _, ok := agentsState["beta"]; !ok {
		t.Error("expected beta in agents state")
	}
	for name, ss := range agentsState {
		if !strings.HasPrefix(ss.Checksum, "sha256:") {
			t.Errorf("skill %s checksum should start with sha256:, got %q", name, ss.Checksum)
		}
		if ss.SyncedAt == "" {
			t.Errorf("skill %s synced_at should be set", name)
		}
	}
}

func TestSyncNewToolGetsSkillsAlreadySyncedElsewhere(t *testing.T) {
	sourceDir, destDirs, cleanup := setupSyncTest(t)
	defer cleanup()

	cfgDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", cfgDir)

	// First sync: only to agents
	cfg := &config.Config{Source: sourceDir, Tools: []string{"agents"}}
	if err := config.SaveConfig(config.ConfigPath(), cfg); err != nil {
		t.Fatal(err)
	}

	restoreOutput, _ := captureOutput(t)
	if err := runSync(&syncOptions{yes: true}); err != nil {
		restoreOutput()
		t.Fatal(err)
	}
	restoreOutput()

	agentsDest := destDirs[paths.ToolAgents]
	if _, err := os.Stat(filepath.Join(agentsDest, "alpha", "SKILL.md")); err != nil {
		t.Fatal("expected alpha synced to agents")
	}

	// Now add claude as a tool and sync again (without --all).
	// Skills are unchanged but claude has never received them.
	cfg.Tools = []string{"agents", "claude"}
	if err := config.SaveConfig(config.ConfigPath(), cfg); err != nil {
		t.Fatal(err)
	}

	restoreOutput, _ = captureOutput(t)
	if err := runSync(&syncOptions{yes: true}); err != nil {
		restoreOutput()
		t.Fatal(err)
	}
	restoreOutput()

	claudeDest := destDirs[paths.ToolClaude]
	if _, err := os.Stat(filepath.Join(claudeDest, "alpha", "SKILL.md")); err != nil {
		t.Error("expected alpha synced to newly added claude tool")
	}
	if _, err := os.Stat(filepath.Join(claudeDest, "beta", "SKILL.md")); err != nil {
		t.Error("expected beta synced to newly added claude tool")
	}
}
