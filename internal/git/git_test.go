package git

import (
	"bytes"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestCloneSuppressesSuccessfulGitProgress(t *testing.T) {
	source := t.TempDir()
	runGit(t, "init", "--quiet", source)
	runGit(t, "-C", source, "config", "user.name", "Test User")
	runGit(t, "-C", source, "config", "user.email", "test@example.com")
	if err := os.WriteFile(filepath.Join(source, "README.md"), []byte("test\n"), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	runGit(t, "-C", source, "add", "README.md")
	runGit(t, "-C", source, "commit", "--quiet", "-m", "initial")

	restoreOutput, output := captureOutput(t)
	clonePath, err := Clone("file://"+source, "")
	restoreOutput()
	if err != nil {
		t.Fatalf("clone failed: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(filepath.Dir(clonePath)) })
	if output.String() != "" {
		t.Fatalf("expected successful clone to be quiet, got: %q", output.String())
	}
}

func runGit(t *testing.T, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v failed: %v: %s", args, err, output)
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
	var output bytes.Buffer
	done := make(chan struct{})
	go func() {
		_, _ = io.Copy(&output, reader)
		close(done)
	}()
	return func() {
		_ = writer.Close()
		<-done
		_ = reader.Close()
		os.Stdout = origStdout
		os.Stderr = origStderr
	}, &output
}
