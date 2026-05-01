package utils

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func ExpandHome(path string) (string, error) {
	if path == "" {
		return path, nil
	}
	if path[0] != '~' {
		return path, nil
	}
	if path != "~" && !strings.HasPrefix(path, "~/") {
		return "", fmt.Errorf("unsupported home path: %s", path)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	if path == "~" {
		return home, nil
	}
	return filepath.Join(home, path[2:]), nil
}

func EnsureDir(path string) error {
	if path == "" {
		return errors.New("empty path")
	}
	return os.MkdirAll(path, 0o755)
}

func NormalizeRepo(input string) (string, error) {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return "", errors.New("repo is required")
	}
	trimmed = strings.TrimSuffix(trimmed, "/")
	trimmed = strings.TrimSuffix(trimmed, ".git")
	if strings.HasPrefix(trimmed, "git@github.com:") {
		trimmed = strings.TrimPrefix(trimmed, "git@github.com:")
	}
	if strings.HasPrefix(trimmed, "https://github.com/") {
		trimmed = strings.TrimPrefix(trimmed, "https://github.com/")
	}
	parts := strings.Split(trimmed, "/")
	if len(parts) != 2 {
		return "", fmt.Errorf("invalid repo: %s", input)
	}
	return parts[0] + "/" + parts[1], nil
}

func RepoURL(repo string) string {
	return "https://github.com/" + repo + ".git"
}

// CloneURL returns the git clone URL for a GitHub repo-like input, preserving
// SSH input when the user provided git@github.com:owner/repo.
func CloneURL(input string) (string, error) {
	repo, err := NormalizeRepo(input)
	if err != nil {
		return "", err
	}
	trimmed := strings.TrimSuffix(strings.TrimSpace(input), "/")
	if strings.HasPrefix(trimmed, "git@github.com:") {
		return "git@github.com:" + repo + ".git", nil
	}
	return RepoURL(repo), nil
}

func ResolveLocalPath(input string) (string, bool, error) {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return "", false, nil
	}
	candidate, err := ExpandHome(trimmed)
	if err != nil {
		return "", false, err
	}
	info, err := os.Stat(candidate)
	if err != nil {
		if os.IsNotExist(err) {
			return "", false, nil
		}
		return "", false, err
	}
	if !info.IsDir() {
		return "", false, fmt.Errorf("path is not a directory: %s", candidate)
	}
	abs, err := filepath.Abs(candidate)
	if err != nil {
		return "", false, err
	}
	return abs, true, nil
}

func PrintInfo(format string, args ...any) {
	fmt.Printf(format+"\n", args...)
}

func PrintWarn(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "warning: "+format+"\n", args...)
}

func PrintError(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "error: "+format+"\n", args...)
}
