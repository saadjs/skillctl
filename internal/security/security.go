package security

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"
)

const maxFileSizeBytes int64 = 1024 * 1024 // 1 MiB

type Severity string

const (
	SeverityLow      Severity = "low"
	SeverityMedium   Severity = "medium"
	SeverityHigh     Severity = "high"
	SeverityCritical Severity = "critical"
)

type Finding struct {
	RuleID   string
	Severity Severity
	Path     string
	Line     int
	Message  string
	Evidence string
}

type Report struct {
	Findings     []Finding
	FilesScanned int
	FilesSkipped int
	SkipReasons  map[string]int
}

func Scan(root string) (Report, error) {
	if strings.TrimSpace(root) == "" {
		return Report{}, errors.New("scan root is required")
	}

	report := Report{
		SkipReasons: make(map[string]int),
	}

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			if d.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		if d.Type()&os.ModeSymlink != 0 {
			report.skip("symlink")
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if info.Size() > maxFileSizeBytes {
			report.skip("too_large")
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read %s: %w", path, err)
		}
		if isBinary(data) {
			report.skip("binary")
			return nil
		}

		report.FilesScanned++
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		report.Findings = append(report.Findings, scanContent(rel, string(data))...)
		return nil
	})
	if err != nil {
		return Report{}, err
	}

	sort.Slice(report.Findings, func(i, j int) bool {
		if report.Findings[i].Path != report.Findings[j].Path {
			return report.Findings[i].Path < report.Findings[j].Path
		}
		if report.Findings[i].Line != report.Findings[j].Line {
			return report.Findings[i].Line < report.Findings[j].Line
		}
		return report.Findings[i].RuleID < report.Findings[j].RuleID
	})
	return report, nil
}

func (r *Report) skip(reason string) {
	r.FilesSkipped++
	r.SkipReasons[reason]++
}

func isBinary(data []byte) bool {
	if len(data) == 0 {
		return false
	}
	if len(data) > 8192 {
		data = data[:8192]
	}
	if !utf8.Valid(data) {
		return true
	}
	for _, b := range data {
		if b == 0 {
			return true
		}
	}
	return false
}
