package security

import (
	"fmt"
	"sort"
	"strings"
)

func Summary(report Report) string {
	return fmt.Sprintf(
		"Security scan completed: %d file(s) scanned, %d skipped, %d finding(s).",
		report.FilesScanned,
		report.FilesSkipped,
		len(report.Findings),
	)
}

func DetailLines(report Report) []string {
	lines := make([]string, 0, len(report.Findings)+1)
	if len(report.SkipReasons) > 0 {
		lines = append(lines, fmt.Sprintf("Skipped files: %s", formatSkipReasons(report.SkipReasons)))
	}
	for _, finding := range report.Findings {
		lines = append(lines, fmt.Sprintf(
			"[%s] %s (%s:%d) %s | evidence: %s",
			finding.Severity,
			finding.RuleID,
			finding.Path,
			finding.Line,
			finding.Message,
			finding.Evidence,
		))
	}
	return lines
}

func formatSkipReasons(reasons map[string]int) string {
	keys := make([]string, 0, len(reasons))
	for key := range reasons {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, fmt.Sprintf("%s=%d", key, reasons[key]))
	}
	return strings.Join(parts, ", ")
}
