package stardew_junimo

import (
	"strings"
	"testing"
)

func TestFixedJunimo140DepotIdentityRequiresCompleteExactBuild(t *testing.T) {
	pins, err := fixedJunimo140GamePins()
	if err != nil {
		t.Fatal(err)
	}
	const manifest = `"AppState" { "appid" "413150" "buildid" "16826371" "StateFlags" "4" "installdir" "Stardew Valley" "InstalledDepots" { "413153" { "manifest" "6005910083361727734" } } }`
	if err := verifyFixedDepotManifest([]byte(manifest), pins.Game); err != nil {
		t.Fatal(err)
	}
	for _, item := range []struct{ from, to string }{
		{"16826371", "16826372"}, {`"StateFlags" "4"`, `"StateFlags" "6"`}, {"6005910083361727734", "6005910083361727735"},
		{`"InstalledDepots"`, `"UnknownDepots"`}, {`"appid" "413150"`, `"appid" "413150" "APPID" "413150"`},
		{`"413153" {`, `"413154" {`},
	} {
		if err := verifyFixedDepotManifest([]byte(strings.Replace(manifest, item.from, item.to, 1)), pins.Game); err == nil {
			t.Fatalf("incomplete or ambiguous identity accepted: %s", item.from)
		}
	}
	if _, err := fixedJunimo140ContentFiles([]byte(`{"Animals/BabyBlue Chicken.xnb":"B63CF439BDA6A3CA2CA546939C610DB8"}`), pins); err == nil {
		t.Fatal("pruned or altered content catalog accepted")
	}
}

func TestSteamManifestRejectsExcessiveDepthAndDuplicateCase(t *testing.T) {
	for _, manifest := range []string{
		`"AppState" { "appid" "413150" "APPID" "413150" }`,
		strings.Repeat(`"nested" { `, 65) + strings.Repeat(`}`, 65),
	} {
		if _, err := parseSteamAppManifest([]byte(manifest), "413150"); err == nil {
			t.Fatal("ambiguous or unbounded manifest accepted")
		}
	}
}
