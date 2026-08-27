package stardew_junimo

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	paneldocker "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/docker"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
	sjconfig "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/stardew_junimo/config"
)

var ErrSteamInviteComposeDependencyUnsupported = errors.New("Steam invite Compose dependency layout is unsupported")
var ErrSteamInviteCleanupPending = errors.New("Steam invite authorization holder cleanup is pending")

type steamInviteAuthSessionHolderCleaner interface {
	RemoveSteamInviteAuthSessionHolders(ctx context.Context, workDir, project, volume string) (paneldocker.CommandResult, error)
}

func steamInviteSessionTarget(dataDir string) (string, string, error) {
	if !filepath.IsAbs(dataDir) {
		return "", "", errors.New("Steam invite session cleanup requires an absolute instance data directory")
	}
	project := strings.ToLower(filepath.Base(filepath.Clean(dataDir)))
	if !runtimeComposeProjectPattern.MatchString(project) {
		return "", "", errors.New("Steam invite session cleanup project cannot be derived safely")
	}
	return project, project + "_steam-session", nil
}

func (d *Driver) removeSteamInviteAuthSessionHolders(ctx context.Context, dataDir string) (paneldocker.CommandResult, string, error) {
	if d.docker == nil {
		return paneldocker.CommandResult{WorkDir: dataDir, ExitCode: -1}, "", errors.New("Docker service is required to clean a Steam invite session")
	}
	project, volume, err := steamInviteSessionTarget(dataDir)
	if err != nil {
		return paneldocker.CommandResult{WorkDir: dataDir, ExitCode: -1}, "", err
	}
	holderCleaner, ok := d.docker.(steamInviteAuthSessionHolderCleaner)
	if !ok {
		return paneldocker.CommandResult{WorkDir: dataDir, ExitCode: -1}, volume, errors.New("Docker service cannot safely classify Steam invite session holders")
	}
	result, err := holderCleaner.RemoveSteamInviteAuthSessionHolders(ctx, dataDir, project, volume)
	return result, volume, err
}

// convergeSteamInviteCleanupPending completes only the holder cleanup that
// follows a successful authorization. The authenticated session volume is
// deliberately preserved; any unclassified holder keeps the state pending.
func (d *Driver) convergeSteamInviteCleanupPending(ctx context.Context, instance registry.Instance) error {
	if sjconfig.SteamInviteAuthState(instance.DataDir) != sjconfig.SteamInviteAuthStateCleanupPending {
		return nil
	}
	if !sjconfig.SteamAuthLoggedIn(instance.DataDir) {
		return fmt.Errorf("%w: successful Auth session evidence is missing", ErrSteamInviteCleanupPending)
	}
	if d.docker == nil {
		return fmt.Errorf("%w: Docker service is not configured", ErrSteamInviteCleanupPending)
	}
	ps, err := d.docker.ComposePs(ctx, instance.DataDir)
	if err != nil {
		return fmt.Errorf("%w: inspect server before holder cleanup: %v", ErrSteamInviteCleanupPending, err)
	}
	if serverServiceUp(ps.Services) {
		return fmt.Errorf("%w: server is running", ErrSteamInviteCleanupPending)
	}
	result, _, err := d.removeSteamInviteAuthSessionHolders(ctx, instance.DataDir)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrSteamInviteCleanupPending, err)
	}
	if result.ExitCode != 0 {
		return fmt.Errorf("%w: holder cleanup exited with code %d", ErrSteamInviteCleanupPending, result.ExitCode)
	}
	if err := sjconfig.SetSteamInviteAuthState(instance.DataDir, sjconfig.SteamInviteAuthStateReady); err != nil {
		return fmt.Errorf("%w: persist ready state: %v", ErrSteamInviteCleanupPending, err)
	}
	return nil
}

// EnsureServerSteamAuthDependencyRemoved removes only the legacy server ->
// steam-auth dependency from an existing Compose file. The optional service
// definition and every unrelated dependency remain user-owned and unchanged.
func EnsureServerSteamAuthDependencyRemoved(dataDir string) (bool, error) {
	if strings.TrimSpace(dataDir) == "" {
		return false, nil
	}
	return migrateServerSteamAuthDependency(filepath.Join(dataDir, "docker-compose.yml"))
}

func migrateServerSteamAuthDependency(path string) (bool, error) {
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
		return false, nil
	}
	updatedSection, changed, err := removeSteamAuthDependencyFromServerSection(text[start:end])
	if err != nil || !changed {
		return false, err
	}
	updated := text[:start] + updatedSection + text[end:]
	if err := atomicWriteRaw(path, []byte(updated), info.Mode().Perm()); err != nil {
		return false, err
	}
	return true, nil
}

type composeDependencyEntry struct {
	lineIndex int
	name      string
}

func removeSteamAuthDependencyFromServerSection(section string) (string, bool, error) {
	lines := composeTextLines(section)
	headerIndex := -1
	for i, line := range lines {
		if line.content == "    depends_on:" {
			headerIndex = i
			break
		}
		if strings.HasPrefix(line.content, "    depends_on:") {
			value := strings.TrimSpace(strings.TrimPrefix(line.content, "    depends_on:"))
			if strings.Contains(value, "steam-auth") {
				return section, false, fmt.Errorf("%w: inline depends_on cannot be migrated safely", ErrSteamInviteComposeDependencyUnsupported)
			}
			return section, false, nil
		}
	}
	if headerIndex < 0 {
		return section, false, nil
	}

	blockEnd := len(section)
	blockEndLine := len(lines)
	for i := headerIndex + 1; i < len(lines); i++ {
		content := lines[i].content
		trimmed := strings.TrimSpace(content)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		indent, ok := composeLeadingSpaces(content)
		if !ok {
			return section, false, fmt.Errorf("%w: depends_on uses tab indentation", ErrSteamInviteComposeDependencyUnsupported)
		}
		if indent <= 4 {
			blockEnd = lines[i].start
			blockEndLine = i
			break
		}
	}

	style := ""
	entries := make([]composeDependencyEntry, 0)
	for i := headerIndex + 1; i < blockEndLine; i++ {
		content := lines[i].content
		trimmed := strings.TrimSpace(content)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		indent, ok := composeLeadingSpaces(content)
		if !ok {
			return section, false, fmt.Errorf("%w: depends_on uses tab indentation", ErrSteamInviteComposeDependencyUnsupported)
		}
		if indent > 6 {
			if len(entries) == 0 {
				return section, false, fmt.Errorf("%w: depends_on child has no dependency entry", ErrSteamInviteComposeDependencyUnsupported)
			}
			continue
		}
		if indent != 6 {
			return section, false, fmt.Errorf("%w: depends_on entry indentation is unsupported", ErrSteamInviteComposeDependencyUnsupported)
		}
		entryStyle, name, ok := parseComposeDependencyEntry(trimmed)
		if !ok {
			return section, false, fmt.Errorf("%w: depends_on entry is unsupported", ErrSteamInviteComposeDependencyUnsupported)
		}
		if style != "" && style != entryStyle {
			return section, false, fmt.Errorf("%w: depends_on mixes mapping and list syntax", ErrSteamInviteComposeDependencyUnsupported)
		}
		style = entryStyle
		entries = append(entries, composeDependencyEntry{lineIndex: i, name: name})
	}

	matching := make([]int, 0, 1)
	otherCount := 0
	for i, entry := range entries {
		if entry.name == "steam-auth" {
			matching = append(matching, i)
		} else {
			otherCount++
		}
	}
	if len(matching) == 0 {
		return section, false, nil
	}
	if otherCount == 0 {
		return section[:lines[headerIndex].start] + section[blockEnd:], true, nil
	}

	updated := section
	for i := len(matching) - 1; i >= 0; i-- {
		entryIndex := matching[i]
		removeStart := lines[entries[entryIndex].lineIndex].start
		removeEnd := blockEnd
		if entryIndex+1 < len(entries) {
			removeEnd = lines[entries[entryIndex+1].lineIndex].start
		}
		updated = updated[:removeStart] + updated[removeEnd:]
	}
	return updated, true, nil
}

func composeLeadingSpaces(line string) (int, bool) {
	spaces := 0
	for _, char := range line {
		switch char {
		case ' ':
			spaces++
		case '\t':
			return 0, false
		default:
			return spaces, true
		}
	}
	return spaces, true
}

func parseComposeDependencyEntry(trimmed string) (style, name string, ok bool) {
	if strings.HasPrefix(trimmed, "- ") {
		name = strings.TrimSpace(strings.TrimPrefix(trimmed, "- "))
		if comment := strings.Index(name, " #"); comment >= 0 {
			name = strings.TrimSpace(name[:comment])
		}
		name = strings.Trim(name, `"'`)
		return "list", name, name != ""
	}
	colon := strings.IndexByte(trimmed, ':')
	if colon <= 0 {
		return "", "", false
	}
	name = strings.Trim(strings.TrimSpace(trimmed[:colon]), `"'`)
	return "mapping", name, name != ""
}

// ensureSteamInviteRuntimeScope performs the one-time legacy runtime
// convergence after user intent has been made explicit. Enabled instances keep
// their existing session untouched. Disabled instances remove only holders of
// their exact project steam-session volume and then the volume itself. It does
// not inspect, pull, validate, start, or stop either runtime image/service.
func (d *Driver) ensureSteamInviteRuntimeScope(ctx context.Context, instance registry.Instance) error {
	if sjconfig.SteamInviteRuntimeScopeCurrent(instance.DataDir) {
		return nil
	}
	if sjconfig.SteamInviteEnabled(instance.DataDir) {
		return sjconfig.MarkSteamInviteRuntimeScopeCurrent(instance.DataDir)
	}
	if d.docker == nil {
		return errors.New("Docker service is required to converge a legacy disabled Steam invite runtime")
	}
	result, sessionVolume, err := d.removeSteamInviteAuthSessionHolders(ctx, instance.DataDir)
	if err != nil {
		return fmt.Errorf("remove legacy optional Auth container: %w", err)
	}
	if result.ExitCode != 0 {
		return fmt.Errorf("remove legacy optional Auth container exited with code %d", result.ExitCode)
	}
	result, err = d.docker.RemoveVolumes(ctx, instance.DataDir, []string{sessionVolume})
	if err != nil {
		return fmt.Errorf("remove legacy optional Auth session: %w", err)
	}
	if result.ExitCode != 0 {
		return fmt.Errorf("remove legacy optional Auth session exited with code %d", result.ExitCode)
	}
	return sjconfig.MarkSteamInviteRuntimeScopeCurrent(instance.DataDir)
}
