package stardew_junimo

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

type steamAppManifest struct {
	AppID       string
	BuildID     string
	StateFlags  string
	InstallDir  string
	LastUpdated string
}

type vdfValue struct {
	text     string
	children map[string]vdfValue
}

type vdfParser struct {
	input []rune
	pos   int
}

// parseSteamAppManifest parses the small Valve KeyValues subset used by ACF
// app manifests. It tokenizes quoted strings/braces and handles escaped quoted
// values; it intentionally does not use whole-file regular expressions.
func parseSteamAppManifest(data []byte, expectedAppID string) (steamAppManifest, error) {
	if len(data) == 0 || len(data) > 1024*1024 {
		return steamAppManifest{}, errors.New("manifest is empty or too large")
	}
	p := &vdfParser{input: []rune(strings.TrimPrefix(string(data), "\ufeff"))}
	root, err := p.parseObject(false)
	if err != nil {
		return steamAppManifest{}, err
	}
	p.skipSpace()
	if p.pos != len(p.input) {
		return steamAppManifest{}, errors.New("unexpected trailing manifest content")
	}
	appState, ok := lookupVDF(root, "AppState")
	if !ok || appState.children == nil {
		return steamAppManifest{}, errors.New("AppState object is missing")
	}
	manifest := steamAppManifest{
		AppID:       vdfText(appState.children, "appid"),
		BuildID:     vdfText(appState.children, "buildid"),
		StateFlags:  vdfText(appState.children, "StateFlags"),
		InstallDir:  vdfText(appState.children, "installdir"),
		LastUpdated: vdfText(appState.children, "LastUpdated"),
	}
	if manifest.AppID != expectedAppID {
		return steamAppManifest{}, fmt.Errorf("appid mismatch: expected %s", expectedAppID)
	}
	if !positiveDecimal(manifest.BuildID) {
		return steamAppManifest{}, errors.New("buildid is missing or invalid")
	}
	if !positiveDecimal(manifest.StateFlags) {
		return steamAppManifest{}, errors.New("StateFlags is missing or invalid")
	}
	if strings.TrimSpace(manifest.InstallDir) == "" {
		return steamAppManifest{}, errors.New("installdir is missing")
	}
	if manifest.LastUpdated != "" && !positiveDecimal(manifest.LastUpdated) {
		return steamAppManifest{}, errors.New("LastUpdated is invalid")
	}
	return manifest, nil
}

func (p *vdfParser) parseObject(requireClose bool) (map[string]vdfValue, error) {
	values := make(map[string]vdfValue)
	for {
		p.skipSpace()
		if p.pos >= len(p.input) {
			if requireClose {
				return nil, errors.New("unterminated object")
			}
			return values, nil
		}
		if p.input[p.pos] == '}' {
			if !requireClose {
				return nil, errors.New("unexpected closing brace")
			}
			p.pos++
			return values, nil
		}
		key, err := p.parseString()
		if err != nil {
			return nil, err
		}
		p.skipSpace()
		if p.pos >= len(p.input) {
			return nil, errors.New("manifest value is missing")
		}
		var value vdfValue
		if p.input[p.pos] == '{' {
			p.pos++
			children, childErr := p.parseObject(true)
			if childErr != nil {
				return nil, childErr
			}
			value.children = children
		} else {
			value.text, err = p.parseString()
			if err != nil {
				return nil, err
			}
		}
		values[key] = value
	}
}

func (p *vdfParser) parseString() (string, error) {
	p.skipSpace()
	if p.pos >= len(p.input) || p.input[p.pos] != '"' {
		return "", errors.New("expected quoted string")
	}
	p.pos++
	var b strings.Builder
	for p.pos < len(p.input) {
		r := p.input[p.pos]
		p.pos++
		switch r {
		case '"':
			return b.String(), nil
		case '\\':
			if p.pos >= len(p.input) {
				return "", errors.New("unterminated escape")
			}
			next := p.input[p.pos]
			p.pos++
			switch next {
			case '\\', '"':
				b.WriteRune(next)
			case 'n':
				b.WriteRune('\n')
			case 't':
				b.WriteRune('\t')
			default:
				b.WriteRune(next)
			}
		default:
			b.WriteRune(r)
		}
	}
	return "", errors.New("unterminated quoted string")
}

func (p *vdfParser) skipSpace() {
	for p.pos < len(p.input) && unicode.IsSpace(p.input[p.pos]) {
		p.pos++
	}
}

func lookupVDF(values map[string]vdfValue, key string) (vdfValue, bool) {
	for candidate, value := range values {
		if strings.EqualFold(candidate, key) {
			return value, true
		}
	}
	return vdfValue{}, false
}

func vdfText(values map[string]vdfValue, key string) string {
	value, ok := lookupVDF(values, key)
	if !ok || value.children != nil {
		return ""
	}
	return strings.TrimSpace(value.text)
}

func positiveDecimal(value string) bool {
	n, err := strconv.ParseUint(strings.TrimSpace(value), 10, 64)
	return err == nil && n > 0
}
