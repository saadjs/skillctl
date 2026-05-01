package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadSaveConfigRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	want := &Config{
		Source: "~/src/my-skills/skills",
		Tools:  []string{"claude", "agents", "codex"},
	}
	if err := SaveConfig(path, want); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}
	got, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if got.Source != want.Source {
		t.Errorf("Source = %q, want %q", got.Source, want.Source)
	}
	if len(got.Tools) != len(want.Tools) {
		t.Fatalf("Tools len = %d, want %d", len(got.Tools), len(want.Tools))
	}
	for i, tool := range got.Tools {
		if tool != want.Tools[i] {
			t.Errorf("Tools[%d] = %q, want %q", i, tool, want.Tools[i])
		}
	}
}

func TestLoadSaveStateRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.yaml")

	want := &State{
		LastSync: "2026-04-13T10:00:00Z",
		Tools: map[string]map[string]SkillState{
			"claude": {
				"code-rules": {
					Checksum: "sha256:abc123",
					SyncedAt: "2026-04-13T10:00:00Z",
				},
			},
			"agents": {
				"de-ai-writing": {
					Checksum: "sha256:def456",
					SyncedAt: "2026-04-13T09:00:00Z",
				},
			},
		},
		RemoteInstalls: map[string]RemoteInstallState{
			"/tmp/skills::alpha": {
				Source:      "owner/repo",
				Ref:         "v1.0.0",
				Path:        "skills",
				Skills:      []string{"alpha"},
				Destination: "/tmp/skills",
				InstalledAt: "2026-04-13T08:00:00Z",
				UpdatedAt:   "2026-04-13T09:00:00Z",
			},
		},
	}
	if err := SaveState(path, want); err != nil {
		t.Fatalf("SaveState: %v", err)
	}
	got, err := LoadState(path)
	if err != nil {
		t.Fatalf("LoadState: %v", err)
	}
	if got.LastSync != want.LastSync {
		t.Errorf("LastSync = %q, want %q", got.LastSync, want.LastSync)
	}
	if len(got.Tools) != len(want.Tools) {
		t.Fatalf("Tools len = %d, want %d", len(got.Tools), len(want.Tools))
	}
	for toolName, wantSkills := range want.Tools {
		gotSkills, ok := got.Tools[toolName]
		if !ok {
			t.Errorf("missing tool %q", toolName)
			continue
		}
		for name, ws := range wantSkills {
			gs, ok := gotSkills[name]
			if !ok {
				t.Errorf("tool %q missing skill %q", toolName, name)
				continue
			}
			if gs.Checksum != ws.Checksum {
				t.Errorf("tool %q skill %q checksum = %q, want %q", toolName, name, gs.Checksum, ws.Checksum)
			}
			if gs.SyncedAt != ws.SyncedAt {
				t.Errorf("tool %q skill %q synced_at = %q, want %q", toolName, name, gs.SyncedAt, ws.SyncedAt)
			}
		}
	}
	gotRemote, ok := got.RemoteInstalls["/tmp/skills::alpha"]
	if !ok {
		t.Fatal("missing remote install")
	}
	wantRemote := want.RemoteInstalls["/tmp/skills::alpha"]
	if gotRemote.Source != wantRemote.Source {
		t.Errorf("remote source = %q, want %q", gotRemote.Source, wantRemote.Source)
	}
	if gotRemote.Ref != wantRemote.Ref {
		t.Errorf("remote ref = %q, want %q", gotRemote.Ref, wantRemote.Ref)
	}
	if gotRemote.Path != wantRemote.Path {
		t.Errorf("remote path = %q, want %q", gotRemote.Path, wantRemote.Path)
	}
	if gotRemote.Destination != wantRemote.Destination {
		t.Errorf("remote destination = %q, want %q", gotRemote.Destination, wantRemote.Destination)
	}
	if gotRemote.InstalledAt != wantRemote.InstalledAt {
		t.Errorf("remote installed_at = %q, want %q", gotRemote.InstalledAt, wantRemote.InstalledAt)
	}
	if gotRemote.UpdatedAt != wantRemote.UpdatedAt {
		t.Errorf("remote updated_at = %q, want %q", gotRemote.UpdatedAt, wantRemote.UpdatedAt)
	}
	if len(gotRemote.Skills) != 1 || gotRemote.Skills[0] != "alpha" {
		t.Errorf("remote skills = %v, want [alpha]", gotRemote.Skills)
	}
}

func TestLoadStateMissingFile(t *testing.T) {
	st, err := LoadState(filepath.Join(t.TempDir(), "nonexistent.yaml"))
	if err != nil {
		t.Fatalf("expected nil error for missing state, got: %v", err)
	}
	if st.Tools == nil {
		t.Error("Tools map should be initialized, got nil")
	}
	if len(st.Tools) != 0 {
		t.Errorf("Tools should be empty, got %d", len(st.Tools))
	}
	if st.RemoteInstalls == nil {
		t.Error("RemoteInstalls map should be initialized, got nil")
	}
	if len(st.RemoteInstalls) != 0 {
		t.Errorf("RemoteInstalls should be empty, got %d", len(st.RemoteInstalls))
	}
}

func TestLoadConfigMissingFile(t *testing.T) {
	_, err := LoadConfig(filepath.Join(t.TempDir(), "nonexistent.yaml"))
	if err == nil {
		t.Fatal("expected error for missing config")
	}
}

func TestChecksumDeterministic(t *testing.T) {
	dir := t.TempDir()
	skillDir := filepath.Join(dir, "my-skill")
	must(t, os.MkdirAll(filepath.Join(skillDir, "scripts"), 0o755))
	must(t, os.MkdirAll(filepath.Join(skillDir, "references"), 0o755))

	writeFile(t, filepath.Join(skillDir, "SKILL.md"), "---\nname: my-skill\n---\nInstructions here.")
	writeFile(t, filepath.Join(skillDir, "scripts", "run.sh"), "#!/bin/bash\necho hello")
	writeFile(t, filepath.Join(skillDir, "references", "REF.md"), "# Reference\nDetails.")

	c1, err := ChecksumSkill(skillDir)
	if err != nil {
		t.Fatalf("ChecksumSkill: %v", err)
	}
	c2, err := ChecksumSkill(skillDir)
	if err != nil {
		t.Fatalf("ChecksumSkill: %v", err)
	}
	if c1 != c2 {
		t.Errorf("checksums differ: %q vs %q", c1, c2)
	}
	if c1[:7] != "sha256:" {
		t.Errorf("checksum should start with sha256:, got %q", c1)
	}
}

func TestChecksumChangesOnFileChange(t *testing.T) {
	dir := t.TempDir()
	skillDir := filepath.Join(dir, "my-skill")
	must(t, os.MkdirAll(skillDir, 0o755))
	writeFile(t, filepath.Join(skillDir, "SKILL.md"), "version 1")

	c1, err := ChecksumSkill(skillDir)
	if err != nil {
		t.Fatalf("ChecksumSkill: %v", err)
	}

	writeFile(t, filepath.Join(skillDir, "SKILL.md"), "version 2")

	c2, err := ChecksumSkill(skillDir)
	if err != nil {
		t.Fatalf("ChecksumSkill: %v", err)
	}
	if c1 == c2 {
		t.Error("checksum should change when file content changes")
	}
}

func TestChecksumChangesOnNewFile(t *testing.T) {
	dir := t.TempDir()
	skillDir := filepath.Join(dir, "my-skill")
	must(t, os.MkdirAll(skillDir, 0o755))
	writeFile(t, filepath.Join(skillDir, "SKILL.md"), "content")

	c1, err := ChecksumSkill(skillDir)
	if err != nil {
		t.Fatalf("ChecksumSkill: %v", err)
	}

	must(t, os.MkdirAll(filepath.Join(skillDir, "scripts"), 0o755))
	writeFile(t, filepath.Join(skillDir, "scripts", "run.sh"), "#!/bin/bash")

	c2, err := ChecksumSkill(skillDir)
	if err != nil {
		t.Fatalf("ChecksumSkill: %v", err)
	}
	if c1 == c2 {
		t.Error("checksum should change when a new file is added")
	}
}

func TestChecksumChangesOnModeChange(t *testing.T) {
	dir := t.TempDir()
	skillDir := filepath.Join(dir, "my-skill")
	must(t, os.MkdirAll(filepath.Join(skillDir, "scripts"), 0o755))
	script := filepath.Join(skillDir, "scripts", "run.sh")
	writeFile(t, script, "#!/bin/bash\necho hi")
	must(t, os.Chmod(script, 0o644))

	c1, err := ChecksumSkill(skillDir)
	if err != nil {
		t.Fatalf("ChecksumSkill: %v", err)
	}

	must(t, os.Chmod(script, 0o755))

	c2, err := ChecksumSkill(skillDir)
	if err != nil {
		t.Fatalf("ChecksumSkill: %v", err)
	}
	if c1 == c2 {
		t.Error("checksum should change when file mode changes (e.g. chmod +x)")
	}
}

func TestChecksumChangesOnSymlinkRetarget(t *testing.T) {
	dir := t.TempDir()
	skillDir := filepath.Join(dir, "my-skill")
	must(t, os.MkdirAll(skillDir, 0o755))
	writeFile(t, filepath.Join(skillDir, "SKILL.md"), "content")
	link := filepath.Join(skillDir, "ref")
	must(t, os.Symlink("a.md", link))

	c1, err := ChecksumSkill(skillDir)
	if err != nil {
		t.Fatalf("ChecksumSkill: %v", err)
	}

	must(t, os.Remove(link))
	must(t, os.Symlink("b.md", link))

	c2, err := ChecksumSkill(skillDir)
	if err != nil {
		t.Fatalf("ChecksumSkill: %v", err)
	}
	if c1 == c2 {
		t.Error("checksum should change when a symlink retargets")
	}
}

func TestDirRespectsXDG(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/custom/config")
	got := Dir()
	want := "/custom/config/skillctl"
	if got != want {
		t.Errorf("Dir() = %q, want %q", got, want)
	}
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
