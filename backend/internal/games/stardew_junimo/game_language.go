package stardew_junimo

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const DefaultGameLanguage = "zh"

var SupportedGameLanguages = []string{"zh", "en", "de", "es", "pt", "ru", "ja", "it", "fr", "ko", "tr", "hu"}

var validGameLanguages = func() map[string]bool {
	values := make(map[string]bool, len(SupportedGameLanguages))
	for _, value := range SupportedGameLanguages {
		values[value] = true
	}
	return values
}()

type GameLanguageSettings struct {
	LanguageCode string `json:"languageCode"`
}

type startupPreferencesLanguage struct {
	LanguageCode string `xml:"languageCode"`
}

var (
	languageCodeElementPattern = regexp.MustCompile(`(?s)<languageCode>.*?</languageCode>`)
	smoothFontElementPattern   = regexp.MustCompile(`(?s)<useChineseSmoothFont>.*?</useChineseSmoothFont>`)
	clientOptionsOpenPattern   = regexp.MustCompile(`<clientOptions(?:\s[^>]*)?>`)
	startupRootOpenPattern     = regexp.MustCompile(`<StartupPreferences(?:\s[^>]*)?>`)
)

func gameLanguageSettingsPath(dataDir string) string {
	return filepath.Join(dataDir, ".local-container", "settings", "game-language.json")
}

func startupPreferencesPath(dataDir string) string {
	return filepath.Join(dataDir, ".local-container", "saves", "startup_preferences")
}

func ValidateGameLanguage(languageCode string) error {
	if !validGameLanguages[languageCode] {
		return fmt.Errorf("languageCode 必须是受支持的 Stardew Valley 语言之一")
	}
	return nil
}

func readLanguageFromPreferences(dataDir string) (string, error) {
	data, err := os.ReadFile(startupPreferencesPath(dataDir))
	if err != nil {
		return "", err
	}
	var preferences startupPreferencesLanguage
	if err := xml.Unmarshal(data, &preferences); err != nil {
		return "", fmt.Errorf("parse startup_preferences: %w", err)
	}
	return strings.TrimSpace(preferences.LanguageCode), nil
}

// ReadGameLanguageSettings preserves a valid language already selected by an
// upgraded instance. New instances, missing preferences and invalid legacy
// values fall back to Simplified Chinese.
func ReadGameLanguageSettings(dataDir string) (GameLanguageSettings, error) {
	data, err := os.ReadFile(gameLanguageSettingsPath(dataDir))
	if err == nil {
		var settings GameLanguageSettings
		if err := json.Unmarshal(data, &settings); err != nil {
			return GameLanguageSettings{}, fmt.Errorf("parse game-language.json: %w", err)
		}
		if err := ValidateGameLanguage(settings.LanguageCode); err != nil {
			return GameLanguageSettings{}, err
		}
		return settings, nil
	}
	if !os.IsNotExist(err) {
		return GameLanguageSettings{}, fmt.Errorf("read game-language.json: %w", err)
	}

	if existing, prefErr := readLanguageFromPreferences(dataDir); prefErr == nil && validGameLanguages[existing] {
		return GameLanguageSettings{LanguageCode: existing}, nil
	} else if prefErr != nil && !os.IsNotExist(prefErr) {
		return GameLanguageSettings{}, prefErr
	}
	return GameLanguageSettings{LanguageCode: DefaultGameLanguage}, nil
}

func writeStartupPreferencesLanguage(dataDir, languageCode string) error {
	path := startupPreferencesPath(dataDir)
	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("read startup_preferences: %w", err)
	}
	if os.IsNotExist(err) {
		data = []byte(`<?xml version="1.0" encoding="utf-8"?>
<StartupPreferences xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xmlns:xsd="http://www.w3.org/2001/XMLSchema">
  <languageCode>` + languageCode + `</languageCode>
  <clientOptions>
    <useChineseSmoothFont>` + fmt.Sprint(languageCode == "zh") + `</useChineseSmoothFont>
  </clientOptions>
</StartupPreferences>
`)
	} else {
		text := string(data)
		languageElement := "<languageCode>" + languageCode + "</languageCode>"
		if languageCodeElementPattern.MatchString(text) {
			text = languageCodeElementPattern.ReplaceAllString(text, languageElement)
		} else if location := startupRootOpenPattern.FindStringIndex(text); location != nil {
			text = text[:location[1]] + "\n  " + languageElement + text[location[1]:]
		} else {
			return fmt.Errorf("startup_preferences missing StartupPreferences root")
		}

		smoothElement := "<useChineseSmoothFont>" + fmt.Sprint(languageCode == "zh") + "</useChineseSmoothFont>"
		if smoothFontElementPattern.MatchString(text) {
			text = smoothFontElementPattern.ReplaceAllString(text, smoothElement)
		} else if location := clientOptionsOpenPattern.FindStringIndex(text); location != nil {
			text = text[:location[1]] + "\n    " + smoothElement + text[location[1]:]
		} else if closeIndex := strings.LastIndex(text, "</StartupPreferences>"); closeIndex >= 0 {
			text = text[:closeIndex] + "  <clientOptions>\n    " + smoothElement + "\n  </clientOptions>\n" + text[closeIndex:]
		} else {
			return fmt.Errorf("startup_preferences missing closing StartupPreferences element")
		}
		data = []byte(text)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create saves directory: %w", err)
	}
	if err := atomicWriteRaw(path, data, 0o644); err != nil {
		return fmt.Errorf("write startup_preferences: %w", err)
	}
	return nil
}

func UpdateGameLanguageSettings(dataDir string, settings GameLanguageSettings) error {
	if err := ValidateGameLanguage(settings.LanguageCode); err != nil {
		return err
	}
	if err := writeStartupPreferencesLanguage(dataDir, settings.LanguageCode); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(gameLanguageSettingsPath(dataDir)), 0o755); err != nil {
		return fmt.Errorf("create settings directory: %w", err)
	}
	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal game-language.json: %w", err)
	}
	data = append(data, '\n')
	if err := atomicWriteRaw(gameLanguageSettingsPath(dataDir), data, 0o644); err != nil {
		return fmt.Errorf("write game-language.json: %w", err)
	}
	return nil
}

// EnsureGameLanguagePreferences runs immediately before Compose starts. The
// panel-owned setting is authoritative after the first save, while upgraded
// instances adopt their existing valid startup_preferences language once.
func EnsureGameLanguagePreferences(dataDir string) (GameLanguageSettings, error) {
	settings, err := ReadGameLanguageSettings(dataDir)
	if err != nil {
		return GameLanguageSettings{}, err
	}
	if err := UpdateGameLanguageSettings(dataDir, settings); err != nil {
		return GameLanguageSettings{}, err
	}
	return settings, nil
}
