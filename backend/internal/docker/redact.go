package docker

import (
	"regexp"
	"strings"
)

// Redacted is the replacement string for sensitive values.
const Redacted = "[REDACTED]"

// Pattern groups for sensitive keys. Each regex targets a different format.
var (
	// Matches JSON key-value pairs with sensitive keys.
	jsonPattern = regexp.MustCompile(`(?i)("(?:STEAM_PASSWORD|VNC_PASSWORD|password|token|secret|session|cookie|authorization|api_key|apikey)"\s*:\s*")([^"]*)(")`)
	// Matches key=value or key: value assignments.
	// Excludes "authorization" — handled separately by bearerPattern.
	assignmentPattern = regexp.MustCompile(`(?i)\b(STEAM_PASSWORD|VNC_PASSWORD|password|token|secret|session|cookie|api_key|apikey)\b\s*[:=]\s*([^\s\r\n,;}]+)`)
	// Matches --flag=value CLI flags.
	flagEqualsPattern = regexp.MustCompile(`(?i)(--(?:password|token|secret|api-key)=)([^\s]+)`)
	// Matches standalone sensitive keywords for flag detection.
	sensitiveKeyPattern = regexp.MustCompile(`(?i)(STEAM_PASSWORD|VNC_PASSWORD|password|token|secret|session|cookie|api_key|apikey)`)
	// Matches env-like flags whose next argument is sensitive.
	envFlagPattern = regexp.MustCompile(`(?i)^--(?:env|e)$`)
	// Matches invite code patterns (alphanumeric codes near keywords).
	inviteCodePattern = regexp.MustCompile(`(?i)(invite\s*code|邀请码)\s*[:=]\s*([A-Za-z0-9]{4,12})`)
	// Matches Bearer tokens in Authorization headers (must run BEFORE assignmentPattern).
	bearerPattern = regexp.MustCompile(`(?i)(Authorization\s*[:=]\s*)(Bearer\s+)([A-Za-z0-9._\-]{10,})`)
	// Matches standalone Bearer tokens (not in Authorization header).
	bearerStandalonePattern = regexp.MustCompile(`(?i)(Bearer\s+)([A-Za-z0-9._\-]{10,})`)
)

// RedactString redacts sensitive information from a string.
// Covers: passwords, tokens, secrets, session, cookie, authorization,
// API keys, invite codes, and Bearer tokens.
func RedactString(input string) string {
	if input == "" {
		return ""
	}
	// Bearer must run before assignment — "Authorization: Bearer eyJhb..."
	// would otherwise be partially matched by assignment as "Authorization: Bearer".
	output := bearerPattern.ReplaceAllString(input, "${1}${2}"+Redacted)
	output = bearerStandalonePattern.ReplaceAllString(output, "${1}"+Redacted)
	output = jsonPattern.ReplaceAllString(output, "${1}"+Redacted+"${3}")
	output = assignmentPattern.ReplaceAllString(output, "${1}="+Redacted)
	output = flagEqualsPattern.ReplaceAllString(output, "${1}"+Redacted)
	output = inviteCodePattern.ReplaceAllString(output, "${1}="+Redacted)
	return output
}

// RedactArgs redacts sensitive values from command-line arguments.
func RedactArgs(args []string) []string {
	redactedArgs := make([]string, len(args))
	hideNext := false
	for index, arg := range args {
		if hideNext {
			redactedArgs[index] = Redacted
			hideNext = false
			continue
		}

		if strings.HasPrefix(arg, "--") && strings.Contains(arg, "=") {
			redactedArgs[index] = RedactString(arg)
			continue
		}

		redactedArgs[index] = RedactString(arg)
		if strings.HasPrefix(arg, "--") && (sensitiveKeyPattern.MatchString(arg) || envFlagPattern.MatchString(arg)) {
			hideNext = true
		}
	}
	return redactedArgs
}
