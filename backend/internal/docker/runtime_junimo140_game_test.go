package docker

import (
	"context"
	"strings"
	"testing"
)

func TestFixedGameChecksumInputRejectsUnsafePathsAndHashes(t *testing.T) {
	valid := strings.Repeat("a", 64)
	for _, name := range []string{"", ".", "../escape", "/absolute", "a/../escape", "a\\file", "a\nfile", "a\tfile", "a\x00file", "a\x7ffile"} {
		if _, err := fixedGameChecksumInput(map[string]string{name: valid}, "sha256"); err == nil {
			t.Fatalf("unsafe checksum path accepted: %q", name)
		}
	}
	for _, hash := range []string{"", strings.Repeat("a", 63), strings.Repeat("z", 64), valid + "\n"} {
		if _, err := fixedGameChecksumInput(map[string]string{"safe": hash}, "sha256"); err == nil {
			t.Fatal("invalid checksum accepted")
		}
	}
	if _, err := fixedGameChecksumInput(map[string]string{"safe": valid}, "sha1"); err == nil {
		t.Fatal("unknown algorithm accepted")
	}
	if _, err := fixedGameChecksumInput(nil, "sha256"); err == nil {
		t.Fatal("empty checksum set accepted")
	}
	input, err := fixedGameChecksumInput(map[string]string{"Content/A B.xnb": strings.Repeat("F", 32)}, "md5")
	if err != nil || input != strings.Repeat("f", 32)+"  Content/A B.xnb\n" {
		t.Fatalf("valid checksum contract lost: %q %v", input, err)
	}
}

func TestFixedGameDockerRejectsUnscopedEvidenceAndMarkers(t *testing.T) {
	c := NewClient(Options{DockerPath: "must-not-run"})
	ctx, dir := context.Background(), t.TempDir()
	if _, err := c.RuntimeReadJunimo140GameEvidence(ctx, dir, "production", "image", "invalid", "../../secret"); err == nil {
		t.Fatal("open evidence read accepted")
	}
	if _, err := c.RuntimeVerifyJunimo140GameFiles(ctx, dir, "production", "image", "invalid", "sha256", map[string]string{"../secret": strings.Repeat("a", 64)}); err == nil {
		t.Fatal("unsafe checksum invoked Docker")
	}
	if err := c.RuntimeCommitJunimo140GameMarkers(ctx, dir, "production", "image", "invalid", RuntimeGameDownloadMarker{}, RuntimeGameDownloadMarker{}); err == nil {
		t.Fatal("unproved marker accepted")
	}
}
