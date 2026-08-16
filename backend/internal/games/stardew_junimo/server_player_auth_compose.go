package stardew_junimo

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

var ErrPlayerAuthComposeUnsupported = errors.New("player authentication Compose layout is unsupported")

type playerAuthComposeEnvironmentEntry struct {
	key         string
	mappingLine string
}

var playerAuthComposeEnvironment = []playerAuthComposeEnvironmentEntry{
	{key: "SAP_PLAYER_AUTH_MODE", mappingLine: `      SAP_PLAYER_AUTH_MODE: "${SAP_PLAYER_AUTH_MODE:-}"`},
	{key: "SAP_PLAYER_AUTH_REVISION", mappingLine: `      SAP_PLAYER_AUTH_REVISION: "${SAP_PLAYER_AUTH_REVISION:-}"`},
	{key: "SAP_ROLE_AUTH_KEY", mappingLine: `      SAP_ROLE_AUTH_KEY: "${SAP_ROLE_AUTH_KEY:-}"`},
	{key: "SAP_ROLE_PASSWORDS_B64", mappingLine: `      SAP_ROLE_PASSWORDS_B64: "${SAP_ROLE_PASSWORDS_B64:-}"`},
}

// EnsureServerPlayerAuthEnvironment upgrades an existing instance Compose file
// without replacing user-owned content. The migration is idempotent and uses an
// atomic file replacement so an interrupted write cannot truncate the file.
func EnsureServerPlayerAuthEnvironment(dataDir string) (bool, error) {
	if dataDir == "" {
		return false, nil
	}
	return migrateServerPlayerAuthEnvironment(filepath.Join(dataDir, "docker-compose.yml"))
}

func migrateServerPlayerAuthEnvironment(path string) (bool, error) {
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	info, err := os.Stat(path)
	if err != nil {
		return false, err
	}

	text := string(raw)
	start, end := composeServiceBounds(text, "server")
	if start < 0 {
		return false, fmt.Errorf("%w: server service not found", ErrPlayerAuthComposeUnsupported)
	}
	section := text[start:end]
	newline := "\n"
	if strings.Contains(text, "\r\n") {
		newline = "\r\n"
	}

	updatedSection, changed, err := addPlayerAuthEnvironmentToServerSection(section, newline)
	if err != nil || !changed {
		return false, err
	}
	updated := text[:start] + updatedSection + text[end:]
	if err := atomicWriteRaw(path, []byte(updated), info.Mode().Perm()); err != nil {
		return false, err
	}
	return true, nil
}

type composeTextLine struct {
	start   int
	end     int
	content string
}

func addPlayerAuthEnvironmentToServerSection(section, newline string) (string, bool, error) {
	lines := composeTextLines(section)
	headerIndex := -1
	for i, line := range lines {
		if line.content == "    environment:" {
			headerIndex = i
			break
		}
		if strings.HasPrefix(line.content, "    environment:") {
			return section, false, fmt.Errorf("%w: server environment must use block syntax", ErrPlayerAuthComposeUnsupported)
		}
	}

	if headerIndex < 0 {
		return addPlayerAuthEnvironmentBlock(section, lines, newline), true, nil
	}

	style := ""
	existing := make(map[string]bool, len(playerAuthComposeEnvironment))
	for _, line := range lines[headerIndex+1:] {
		trimmed := strings.TrimSpace(line.content)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		indent := len(line.content) - len(strings.TrimLeft(line.content, " "))
		if indent <= 4 {
			break
		}
		entryStyle, key, ok := parseComposeEnvironmentEntry(line.content)
		if !ok {
			return section, false, fmt.Errorf("%w: server environment contains an unsupported entry", ErrPlayerAuthComposeUnsupported)
		}
		if style != "" && entryStyle != style {
			return section, false, fmt.Errorf("%w: server environment mixes mapping and list syntax", ErrPlayerAuthComposeUnsupported)
		}
		style = entryStyle
		existing[key] = true
	}
	if style == "" {
		style = "mapping"
	}

	missing := make([]string, 0, len(playerAuthComposeEnvironment))
	for _, entry := range playerAuthComposeEnvironment {
		if existing[entry.key] {
			continue
		}
		if style == "list" {
			missing = append(missing, "      - "+entry.key+"=${"+entry.key+":-}")
		} else {
			missing = append(missing, entry.mappingLine)
		}
	}
	if len(missing) == 0 {
		return section, false, nil
	}

	insertAt := lines[headerIndex].end
	prefix := ""
	if insertAt > 0 && section[insertAt-1] != '\n' {
		prefix = newline
	}
	block := prefix + strings.Join(missing, newline) + newline
	return section[:insertAt] + block + section[insertAt:], true, nil
}

func addPlayerAuthEnvironmentBlock(section string, lines []composeTextLine, newline string) string {
	insertAt := len(section)
	for _, line := range lines {
		if line.content == "    volumes:" {
			insertAt = line.start
			break
		}
	}

	mappingLines := make([]string, 0, len(playerAuthComposeEnvironment)+1)
	mappingLines = append(mappingLines, "    environment:")
	for _, entry := range playerAuthComposeEnvironment {
		mappingLines = append(mappingLines, entry.mappingLine)
	}
	block := strings.Join(mappingLines, newline) + newline
	if insertAt > 0 && section[insertAt-1] != '\n' {
		block = newline + block
	}
	return section[:insertAt] + block + section[insertAt:]
}

func parseComposeEnvironmentEntry(line string) (style, key string, ok bool) {
	trimmed := strings.TrimSpace(line)
	if strings.HasPrefix(trimmed, "- ") {
		value := strings.TrimSpace(strings.TrimPrefix(trimmed, "- "))
		if equals := strings.Index(value, "="); equals >= 0 {
			value = value[:equals]
		}
		key = strings.Trim(strings.TrimSpace(value), `"'`)
		return "list", key, key != ""
	}
	colon := strings.Index(trimmed, ":")
	if colon <= 0 {
		return "", "", false
	}
	key = strings.Trim(strings.TrimSpace(trimmed[:colon]), `"'`)
	return "mapping", key, key != ""
}

func composeTextLines(text string) []composeTextLine {
	if text == "" {
		return nil
	}
	lines := make([]composeTextLine, 0, strings.Count(text, "\n")+1)
	for start := 0; start < len(text); {
		end := len(text)
		if offset := strings.IndexByte(text[start:], '\n'); offset >= 0 {
			end = start + offset + 1
		}
		content := strings.TrimSuffix(strings.TrimSuffix(text[start:end], "\n"), "\r")
		lines = append(lines, composeTextLine{start: start, end: end, content: content})
		start = end
	}
	return lines
}
