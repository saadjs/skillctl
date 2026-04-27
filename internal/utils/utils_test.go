package utils

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNormalizeRepo(t *testing.T) {
	cases := map[string]string{
		"saadjs/skillctl":                        "saadjs/skillctl",
		"https://github.com/saadjs/skillctl":     "saadjs/skillctl",
		"https://github.com/saadjs/skillctl.git": "saadjs/skillctl",
		"git@github.com:saadjs/skillctl.git":     "saadjs/skillctl",
	}
	for input, expected := range cases {
		got, err := NormalizeRepo(input)
		if err != nil {
			t.Fatalf("unexpected error for %s: %v", input, err)
		}
		if got != expected {
			t.Fatalf("expected %s, got %s", expected, got)
		}
	}
	if _, err := NormalizeRepo("not-a-repo"); err == nil {
		t.Fatalf("expected error for invalid repo")
	}
}

func TestResolveLocalPath(t *testing.T) {
	dir := t.TempDir()
	got, ok, err := ResolveLocalPath(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatalf("expected local path to be detected")
	}
	if got == "" {
		t.Fatalf("expected resolved path")
	}
	file := filepath.Join(dir, "file.txt")
	if err := os.WriteFile(file, []byte("hi"), 0o644); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	if _, ok, err := ResolveLocalPath(file); err == nil || ok {
		t.Fatalf("expected error for file path")
	}
}
