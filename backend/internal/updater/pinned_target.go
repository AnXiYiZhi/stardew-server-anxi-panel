package updater

import (
	"context"
	"encoding/json"
	"errors"
	"regexp"
	"strings"
	"time"
)

// PanelTarget is content-addressed and contains no credential or reusable
// authorization. The full-stack operation binds this identity to its own ID.
type PanelTarget struct {
	Version   string `json:"version"`
	Image     string `json:"image"`
	ImageID   string `json:"imageId"`
	Revision  string `json:"revision"`
	BuildDate string `json:"buildDate"`
}

var panelContentID = regexp.MustCompile(`^sha256:[a-f0-9]{64}$`)
var panelRevision = regexp.MustCompile(`^[a-f0-9]{40}$`)
var regexpFullStackOperation = regexp.MustCompile(`^stack_[a-f0-9]{32}$`)

func (p PanelTarget) Valid() bool {
	version, err := NormalizeTargetVersion(p.Version)
	_, dateErr := time.Parse(time.RFC3339, p.BuildDate)
	return err == nil && version == p.Version && panelContentID.MatchString(p.ImageID) && panelRevision.MatchString(p.Revision) && dateErr == nil
}

// Image is a display/reference alias. The running container may legitimately
// use the same confirmed content ID after the helper removes the mutable tag.
func (p PanelTarget) SameContent(other PanelTarget) bool {
	return p.Valid() && other.Valid() && p.Version == other.Version && p.ImageID == other.ImageID && p.Revision == other.Revision && p.BuildDate == other.BuildDate
}

func parsePinnedPanelTarget(raw []byte, image, version string) (PanelTarget, error) {
	var images []struct {
		ID     string `json:"Id"`
		Config struct{ Labels map[string]string }
	}
	if json.Unmarshal(raw, &images) != nil || len(images) != 1 {
		return PanelTarget{}, errors.New("Panel image metadata unavailable")
	}
	labels := images[0].Config.Labels
	target := PanelTarget{Version: version, Image: image, ImageID: images[0].ID, Revision: labels["org.opencontainers.image.revision"], BuildDate: labels["org.opencontainers.image.created"]}
	if !target.Valid() || strings.TrimPrefix(labels["org.opencontainers.image.version"], "v") != version {
		return PanelTarget{}, errors.New("Panel image OCI identity does not match the exact target")
	}
	return target, nil
}

func (d *DockerCLI) InspectPanelTarget(ctx context.Context, image, version string) (PanelTarget, error) {
	raw, err := d.command(ctx, 30*time.Second, "image", "inspect", image)
	if err != nil {
		return PanelTarget{}, err
	}
	return parsePinnedPanelTarget(raw, image, version)
}

func (d *DockerCLI) InspectRunningPanel(ctx context.Context, ref, version string) (PanelTarget, error) {
	container, err := d.InspectContainer(ctx, ref)
	if err != nil || !panelContentID.MatchString(container.ImageID) {
		return PanelTarget{}, errors.New("running Panel content identity unavailable")
	}
	target, err := d.InspectPanelTarget(ctx, container.ImageID, version)
	if err != nil || target.ImageID != container.ImageID {
		return PanelTarget{}, errors.New("running Panel OCI identity unavailable")
	}
	target.Image = container.Image
	return target, nil
}

// All downloads happen in the existing read-only dry-run first. This method
// only captures its exact local image/OCI identity for the confirmation screen.
func (s *Service) PreparedPanelTarget(ctx context.Context, current, target string) (PanelTarget, error) {
	version, err := NormalizeTargetVersion(target)
	if err != nil {
		return PanelTarget{}, err
	}
	capability := s.Capability(ctx)
	if !capability.Supported || capability.ConversionRequired {
		return PanelTarget{}, errors.New("finish deployment normalization before confirming a scoped upgrade")
	}
	image := capability.CurrentImage
	currentTarget := false
	if normalized, _ := NormalizeTargetVersion(current); normalized != version {
		dry, err := s.Status()
		if err != nil || dry.Phase != "succeeded" || dry.TargetVersion != version || ValidateTrustedImage(dry.TargetImage) != nil {
			return PanelTarget{}, errors.New("complete the exact Panel target dry-run first")
		}
		image = dry.TargetImage
	} else {
		// A locally retagged reference is not evidence of what this process
		// actually runs. Inspect the current container's immutable image ID.
		container, err := s.docker.InspectContainer(ctx, s.detect.ContainerRef)
		if err != nil || !panelContentID.MatchString(container.ImageID) {
			return PanelTarget{}, errors.New("running Panel content identity unavailable")
		}
		image, currentTarget = container.ImageID, true
	}
	inspector, ok := s.docker.(interface {
		InspectPanelTarget(context.Context, string, string) (PanelTarget, error)
	})
	if !ok {
		return PanelTarget{}, errors.New("Panel OCI inspection is unavailable")
	}
	pin, err := inspector.InspectPanelTarget(ctx, image, version)
	if err != nil || currentTarget && pin.ImageID != image {
		return PanelTarget{}, errors.New("exact Panel content identity unavailable")
	}
	if currentTarget {
		pin.Image = capability.CurrentImage
	}
	return pin, nil
}
