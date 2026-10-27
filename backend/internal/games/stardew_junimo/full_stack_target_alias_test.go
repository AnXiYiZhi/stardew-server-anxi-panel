package stardew_junimo

import (
	"maps"
	"strings"
	"testing"
)

func TestFullStackConfirmationBindsContentAcrossDeclaredMirrors(t *testing.T) {
	for _, scenario := range []struct {
		name string
		edit func(*junimo140GroupTransaction, *junimo140GroupTransaction)
		want bool
	}{
		{name: "declared mirror with identical content", want: true},
		{name: "different content ID", edit: func(_, b *junimo140GroupTransaction) { b.Target.Server.ImageID = "sha256:" + strings.Repeat("b", 64) }},
		{name: "different digest", edit: func(_, b *junimo140GroupTransaction) { b.Target.Server.Digest = "sha256:" + strings.Repeat("b", 64) }},
		{name: "unlisted mirror", edit: func(_, b *junimo140GroupTransaction) {
			b.Target.Server.Image = "example.invalid/sdvd/server:1.5.0-preview.140"
		}},
		{name: "mirror has different declared digest", edit: func(_, b *junimo140GroupTransaction) {
			b.Recommendation.Server.Digests = maps.Clone(b.Recommendation.Server.Digests)
			b.Recommendation.Server.Digests[b.Target.Server.Image] = "sha256:" + strings.Repeat("b", 64)
		}},
		{name: "missing content ID", edit: func(a, b *junimo140GroupTransaction) { a.Target.Server.ImageID = ""; b.Target.Server.ImageID = "" }},
		{name: "missing digest", edit: func(a, b *junimo140GroupTransaction) { a.Target.Server.Digest = ""; b.Target.Server.Digest = "" }},
		{name: "original source reference changed", edit: func(_, b *junimo140GroupTransaction) { b.Worlds[0].Original.Server.Image = "different-source" }},
		{name: "original running choice changed", edit: func(_, b *junimo140GroupTransaction) { b.Worlds[0].Original.ServerWasRunning = false }},
		{name: "authentication target changed", edit: func(_, b *junimo140GroupTransaction) {
			b.Target.SteamAuth = RuntimeUpdateSelectedImage{Image: "different-auth"}
		}},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			recipe, err := fixedFullStackJunimo140Target()
			if err != nil || len(recipe.Server.Images) < 2 {
				t.Fatal("fixed target requires two declared mirrors", err)
			}
			a := junimo140GroupTransaction{Recommendation: recipe, Target: RuntimeUpdateSelectedPair{Server: RuntimeUpdateSelectedImage{
				Image: recipe.Server.Images[0], Digest: junimo140Digest, ImageID: "sha256:" + strings.Repeat("a", 64),
			}}, Worlds: []junimo140WorldProgress{{Original: junimo140WorldOriginal{ServerWasRunning: true}}}}
			b := a
			b.Worlds = append([]junimo140WorldProgress(nil), a.Worlds...)
			b.Target.Server.Image = recipe.Server.Images[1]
			a.InputSHA256, b.InputSHA256 = "original-reference-proof", "recaptured-reference-proof"
			if scenario.edit != nil {
				scenario.edit(&a, &b)
			}
			if got := sameFullStackOriginal(a, b); got != scenario.want {
				t.Fatalf("same original=%v want=%v", got, scenario.want)
			}
		})
	}
}
