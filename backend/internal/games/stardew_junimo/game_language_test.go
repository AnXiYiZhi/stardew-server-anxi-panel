package stardew_junimo

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGameLanguageDefaultsToChineseAndCreatesPreferences(t *testing.T) {
	dir := t.TempDir()
	settings, err := EnsureGameLanguagePreferences(dir)
	if err != nil {
		t.Fatalf("EnsureGameLanguagePreferences: %v", err)
	}
	if settings.LanguageCode != "zh" {
		t.Fatalf("language = %q, want zh", settings.LanguageCode)
	}
	data, err := os.ReadFile(startupPreferencesPath(dir))
	if err != nil {
		t.Fatalf("read startup_preferences: %v", err)
	}
	text := string(data)
	if !strings.Contains(text, "<languageCode>zh</languageCode>") || !strings.Contains(text, "<useChineseSmoothFont>true</useChineseSmoothFont>") {
		t.Fatalf("Chinese preferences not written:\n%s", text)
	}
	if _, err := os.Stat(gameLanguageSettingsPath(dir)); err != nil {
		t.Fatalf("game-language.json not written: %v", err)
	}
}

func TestGameLanguageAdoptsExistingValidLanguageThenBecomesAuthoritative(t *testing.T) {
	dir := t.TempDir()
	path := startupPreferencesPath(dir)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	original := `<StartupPreferences><languageCode>de</languageCode><clientOptions><useChineseSmoothFont>true</useChineseSmoothFont><other>true</other></clientOptions></StartupPreferences>`
	if err := os.WriteFile(path, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}
	settings, err := EnsureGameLanguagePreferences(dir)
	if err != nil {
		t.Fatalf("EnsureGameLanguagePreferences: %v", err)
	}
	if settings.LanguageCode != "de" {
		t.Fatalf("language = %q, want de", settings.LanguageCode)
	}
	if err := os.WriteFile(path, []byte(strings.ReplaceAll(original, ">de<", ">en<")), 0o644); err != nil {
		t.Fatal(err)
	}
	settings, err = EnsureGameLanguagePreferences(dir)
	if err != nil {
		t.Fatalf("second EnsureGameLanguagePreferences: %v", err)
	}
	if settings.LanguageCode != "de" {
		t.Fatalf("authoritative language = %q, want de", settings.LanguageCode)
	}
	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), "<languageCode>de</languageCode>") || !strings.Contains(string(data), "<other>true</other>") {
		t.Fatalf("preferences not safely preserved:\n%s", data)
	}
}

func TestUpdateGameLanguageRejectsUnsupportedValue(t *testing.T) {
	if err := UpdateGameLanguageSettings(t.TempDir(), GameLanguageSettings{LanguageCode: "xx"}); err == nil {
		t.Fatal("expected unsupported language to fail")
	}
}
