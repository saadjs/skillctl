package git

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

func Clone(repoURL, ref string) (string, error) {
	base, err := os.MkdirTemp("", "skillctl-")
	if err != nil {
		return "", err
	}
	target := filepath.Join(base, "repo")
	args := []string{"clone", "--quiet", "--depth", "1"}
	if ref != "" {
		args = append(args, "--branch", ref, "--single-branch")
	}
	args = append(args, repoURL, target)
	cmd := exec.Command("git", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git clone failed: %w", err)
	}
	return target, nil
}
