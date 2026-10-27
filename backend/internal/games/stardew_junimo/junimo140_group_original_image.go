package stardew_junimo

import (
	"context"
	"errors"
)

// A never-started template has no container snapshot. Bind its declared source
// to a verified cached image without treating it as an existing/running service.
func captureJunimo140InactiveServerImage(ctx context.Context, docker junimo140GroupDocker, dir, image string, original *junimo140WorldOriginal) error {
	if original.ServerPresent {
		return nil
	}
	expected := junimo140Digest
	switch original.Current.Server.Tag {
	case "1.5.0-preview.125":
		expected = "sha256:10f438581d741fc146ce710cbe20099475ac68908e99f565cf449f0b8192ccf6"
	case "1.5.0-preview.140":
	default:
		return errors.New("shared runtime inactive source version is unsupported")
	}
	if image == "" {
		return errors.New("shared runtime inactive server image is missing")
	}
	metadata, err := docker.RuntimeImageInspect(ctx, dir, image)
	if err != nil || !runtimeImageDigestPattern.MatchString(metadata.ID) || metadata.Digest != expected {
		return errors.New("shared runtime inactive server does not match the fixed source image")
	}
	original.Server = RuntimeUpdateSelectedImage{Image: image, ImageID: metadata.ID, Digest: metadata.Digest}
	return nil
}
