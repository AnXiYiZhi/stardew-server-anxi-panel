// Package installerrors explains installation failures without exposing log content.
package installerrors

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"sync"
)

//go:embed catalog.json
var catalogJSON []byte

type Rule struct {
	ID      string `json:"id"`
	Pattern string `json:"pattern"`
	Message string `json:"message"`
	Group   string `json:"group"`
	matcher *regexp.Regexp
}

var catalog = func() struct {
	Rules   []Rule            `json:"rules"`
	Phases  map[string]string `json:"phases"`
	Exits   map[string]string `json:"exits"`
	Unknown string            `json:"unknown"`
} {
	var result struct {
		Rules   []Rule            `json:"rules"`
		Phases  map[string]string `json:"phases"`
		Exits   map[string]string `json:"exits"`
		Unknown string            `json:"unknown"`
	}
	if err := json.Unmarshal(catalogJSON, &result); err != nil {
		panic(err)
	}
	for i := range result.Rules {
		result.Rules[i].matcher = regexp.MustCompile("(?i)" + result.Rules[i].Pattern)
	}
	return result
}()

var exitPattern = regexp.MustCompile(`(?i)(?:exit(?:ed)?(?: with)?(?: code| status)?|exitcode|退出码|\bcode)\s*[:=]?\s*([1-9][0-9]*)\b`)

func Match(text string) *Rule {
	for i := range catalog.Rules {
		if catalog.Rules[i].matcher.MatchString(text) {
			return &catalog.Rules[i]
		}
	}
	return nil
}

// Evidence keeps only a catalog entry, never credentials, raw output or paths.
// Each command attempt resets it; successful authentication discards auth errors.
type Evidence struct {
	mu   sync.Mutex
	rule *Rule
}

func (e *Evidence) Reset() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.rule = nil
}

func (e *Evidence) Authenticated() {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.rule != nil && (e.rule.Group == "auth" || e.rule.Group == "network") {
		e.rule = nil
	}
}

func (e *Evidence) Observe(line string) {
	rule := Match(line)
	if rule == nil {
		return
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	// The app's final bitmask is less specific than an earlier concrete cause.
	if (rule.ID != "steam_app_state" && rule.ID != "missing_success") || e.rule == nil {
		e.rule = rule
	}
}

func (e *Evidence) Message(cause error, phase string) string {
	e.mu.Lock()
	defer e.mu.Unlock()
	if rule := Match(cause.Error()); rule != nil && rule.ID != "steam_app_state" && rule.ID != "missing_success" {
		return rule.Message
	}
	if e.rule != nil {
		return e.rule.Message
	}
	if rule := Match(cause.Error()); rule != nil {
		return rule.Message
	}
	if match := exitPattern.FindStringSubmatch(cause.Error()); len(match) > 1 {
		if message := catalog.Exits[match[1]]; message != "" {
			return message
		}
		return fmt.Sprintf("安装进程异常退出（退出码 %s），请展开本次任务日志查看失败前的原因后重试。", match[1])
	}
	if message := catalog.Phases[phase]; message != "" {
		return message
	}
	if strings.Contains(strings.ToLower(cause.Error()), "timed out") {
		return catalog.Phases["install_timeout"]
	}
	return catalog.Unknown
}

type ExplainedError struct {
	Message string
	Cause   error
}

func (e *ExplainedError) Error() string { return e.Message }
func (e *ExplainedError) Unwrap() error { return e.Cause }
