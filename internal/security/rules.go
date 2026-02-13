package security

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	rmRFPattern            = regexp.MustCompile(`(?i)\brm\s+-rf\b`)
	curlOrWgetPipePattern  = regexp.MustCompile(`(?i)\b(curl|wget)\b[^|\n]*\|\s*(bash|sh)\b`)
	invokeExprPattern      = regexp.MustCompile(`(?i)\b(invoke-expression|iex)\b`)
	reverseShellPattern    = regexp.MustCompile(`(?i)(bash\s+-i\s+>&\s*/dev/tcp/|nc\s+-e\s+/bin/(sh|bash)|python[23]?\s+-c\s+.*socket\.)`)
	longBase64Pattern      = regexp.MustCompile(`\b[A-Za-z0-9+/]{180,}={0,2}\b`)
	longHexPattern         = regexp.MustCompile(`\b[0-9a-fA-F]{180,}\b`)
	instructionOverrideREs = []*regexp.Regexp{
		regexp.MustCompile(`(?i)\bignore (all )?(previous|prior|system|developer) instructions\b`),
		regexp.MustCompile(`(?i)\b(disregard|override|bypass)\b.*\b(safety|guardrails|policies|instructions)\b`),
		regexp.MustCompile(`(?i)\b(reveal|print|expose)\b.*\b(system prompt|developer prompt|hidden prompt)\b`),
		regexp.MustCompile(`(?i)\b(do not|don't)\b.*\b(mention|disclose)\b.*\b(instruction|prompt)\b`),
		regexp.MustCompile(`(?i)\bexfiltrate\b.*\b(secret|token|key|credential)\b`),
	}
)

var (
	exfilSensitiveHints = []string{
		"api_key", "apikey", "secret", "token", "password", "id_rsa", ".env",
		"credentials", "private key", "aws_secret_access_key", "ssh_key",
	}
	exfilOutboundHints = []string{
		"curl ", "wget ", "http://", "https://", "nc ", "netcat ", "upload", "post ",
	}
	decodeHints = []string{
		"base64 -d", "base64 --decode", "frombase64string", "xxd -r", "openssl enc -base64",
	}
	execHints = []string{
		"bash", "sh ", "powershell", "invoke-expression", "iex", "eval(", "exec(",
	}
)

func scanContent(path, content string) []Finding {
	lines := strings.Split(content, "\n")
	seen := make(map[string]bool)
	findings := make([]Finding, 0)

	add := func(ruleID string, severity Severity, line int, message, evidence string) {
		key := fmt.Sprintf("%s:%d:%s", path, line, ruleID)
		if seen[key] {
			return
		}
		seen[key] = true
		findings = append(findings, Finding{
			RuleID:   ruleID,
			Severity: severity,
			Path:     path,
			Line:     line,
			Message:  message,
			Evidence: sanitizeEvidence(evidence),
		})
	}

	lowerContent := strings.ToLower(content)
	hasDecode := containsAny(lowerContent, decodeHints...)
	hasExec := containsAny(lowerContent, execHints...)

	longTokenLine := 0
	longTokenEvidence := ""

	for i, line := range lines {
		lineNo := i + 1
		trimmed := strings.TrimSpace(line)
		lower := strings.ToLower(line)

		if rmRFPattern.MatchString(line) {
			add("malicious_rm_rf", SeverityCritical, lineNo, "Potential destructive delete command", line)
		}
		if curlOrWgetPipePattern.MatchString(line) {
			add("remote_shell_pipe", SeverityCritical, lineNo, "Remote script piped directly to shell", line)
		}
		if invokeExprPattern.MatchString(line) {
			add("powershell_invoke_expression", SeverityHigh, lineNo, "PowerShell dynamic execution detected", line)
		}
		if reverseShellPattern.MatchString(line) {
			add("reverse_shell_pattern", SeverityCritical, lineNo, "Potential reverse shell command detected", line)
		}
		if containsAny(lower, exfilSensitiveHints...) && containsAny(lower, exfilOutboundHints...) {
			add("secret_exfiltration", SeverityHigh, lineNo, "Sensitive data appears combined with outbound transfer", line)
		}
		for _, re := range instructionOverrideREs {
			if re.MatchString(line) {
				add("agent_instruction_override", SeverityHigh, lineNo, "Potential prompt-injection or policy-override instruction", line)
				break
			}
		}
		if longTokenLine == 0 {
			if token := longBase64Pattern.FindString(trimmed); token != "" {
				longTokenLine = lineNo
				longTokenEvidence = token
			} else if token := longHexPattern.FindString(trimmed); token != "" {
				longTokenLine = lineNo
				longTokenEvidence = token
			}
		}
	}

	if longTokenLine > 0 && hasDecode && hasExec {
		add(
			"encoded_payload_execution",
			SeverityMedium,
			longTokenLine,
			"Long encoded payload appears with decode and execution hints",
			longTokenEvidence,
		)
	}

	return findings
}

func containsAny(s string, needles ...string) bool {
	for _, needle := range needles {
		if strings.Contains(s, needle) {
			return true
		}
	}
	return false
}

func sanitizeEvidence(s string) string {
	const maxLen = 140
	clean := strings.TrimSpace(strings.ReplaceAll(s, "\t", " "))
	if len(clean) <= maxLen {
		return clean
	}
	return clean[:maxLen-3] + "..."
}
