package docker

import (
	"regexp"
	"strings"
)

const redacted = "[REDACTED]"

var sensitiveKeyPattern = regexp.MustCompile(`(?i)(STEAM_PASSWORD|VNC_PASSWORD|password|token|secret)`)
var assignmentPattern = regexp.MustCompile(`(?i)\b(STEAM_PASSWORD|VNC_PASSWORD|password|token|secret)\b\s*[:=]\s*([^\s\r\n,;}]+)`)
var jsonPattern = regexp.MustCompile(`(?i)("(?:STEAM_PASSWORD|VNC_PASSWORD|password|token|secret)"\s*:\s*")([^"]*)(")`)
var flagEqualsPattern = regexp.MustCompile(`(?i)(--(?:password|token|secret)=)([^\s]+)`)

func RedactString(input string) string {
	if input == "" {
		return ""
	}
	output := jsonPattern.ReplaceAllString(input, `${1}`+redacted+`${3}`)
	output = assignmentPattern.ReplaceAllString(output, `${1}=`+redacted)
	output = flagEqualsPattern.ReplaceAllString(output, `${1}`+redacted)
	return output
}

func RedactArgs(args []string) []string {
	redactedArgs := make([]string, len(args))
	hideNext := false
	for index, arg := range args {
		if hideNext {
			redactedArgs[index] = redacted
			hideNext = false
			continue
		}

		if strings.HasPrefix(arg, "--") && strings.Contains(arg, "=") {
			redactedArgs[index] = RedactString(arg)
			continue
		}

		redactedArgs[index] = RedactString(arg)
		if strings.HasPrefix(arg, "--") && sensitiveKeyPattern.MatchString(arg) {
			hideNext = true
		}
	}
	return redactedArgs
}
