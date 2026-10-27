package stardew_junimo

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// This migration is selected only for an instance explicitly configured with
// the fixed .140 image. It does not change the recommended stack or image.
func ensureJunimo140ComposeContract(dataDir string, includeSteamAuth bool) (bool, error) {
	path := filepath.Join(dataDir, "docker-compose.yml")
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	updated, err := migrateJunimo140ComposeContract(string(raw), includeSteamAuth)
	if err != nil || updated == string(raw) {
		return false, err
	}
	info, err := os.Stat(path)
	if err != nil {
		return false, err
	}
	if err := atomicWriteRaw(path, []byte(updated), info.Mode().Perm()); err != nil {
		return false, err
	}
	return true, nil
}

func migrateJunimo140ComposeContract(text string, includeSteamAuth bool) (string, error) {
	newline := "\n"
	if strings.Contains(text, "\r\n") {
		newline = "\r\n"
	}
	updated := text
	services := []string{"server"}
	if includeSteamAuth {
		services = append(services, "steam-auth")
	}
	for _, service := range services {
		start, end := composeServiceBounds(updated, service)
		if start < 0 {
			return text, fmt.Errorf("junimo140_compose_manual_required: missing managed %s service", service)
		}
		section := updated[start:end]
		if strings.Count(section, "    environment:"+newline) != 1 || strings.Contains(section, newline+"      - USER_ID") {
			return text, fmt.Errorf("junimo140_compose_manual_required: %s environment must use the Panel mapping format", service)
		}
		if service == "server" {
			const publicAPIPort = `      - "${API_PORT:-8080}:8080/tcp"`
			const privateAPIPort = `      - "127.0.0.1:${API_PORT:-8080}:8080/tcp"`
			if strings.Contains(section, publicAPIPort) {
				section = strings.Replace(section, publicAPIPort, privateAPIPort, 1)
			} else if !strings.Contains(section, privateAPIPort) {
				return text, fmt.Errorf("junimo140_compose_manual_required: review API port binding for the public status endpoint")
			}
			const managedAPIPort = `      API_PORT: "${API_PORT:-8080}"`
			const fixedAPIPort = `      API_PORT: "8080"`
			if strings.Count(section, newline+"      API_PORT:") != 1 ||
				(!strings.Contains(section, newline+managedAPIPort+newline) && !strings.Contains(section, newline+fixedAPIPort+newline)) {
				return text, fmt.Errorf("junimo140_compose_manual_required: review internal API port before changing its host binding")
			}
			section = strings.Replace(section, managedAPIPort, fixedAPIPort, 1)
			lines := strings.SplitAfter(section, "\n")
			var kept []string
			for _, line := range lines {
				managed := false
				for _, value := range serverStaticInitValues {
					for _, source := range []string{"./", "${INSTANCE_HOST_DATA_DIR}/"} {
						if strings.TrimSpace(line) == "- "+source+value.localPath+":"+value.containerPath+":ro" {
							managed = true
						}
					}
				}
				if managed {
					continue
				}
				if strings.Contains(line, ":/etc/cont-env.d/") || strings.Contains(line, ":/etc/cont-users.d/") || strings.Contains(line, ":/etc/cont-groups.d/") {
					return text, fmt.Errorf("junimo140_compose_manual_required: unrecognized static init mount; original compose retained")
				}
				kept = append(kept, line)
			}
			section = strings.Join(kept, "")
		}
		values := []string{`      USER_ID: "${USER_ID:-1000}"`, `      GROUP_ID: "${GROUP_ID:-1000}"`}
		if service == "server" {
			values = append(values, `      SDVD_COMPOSE_REV: "4"`)
		} else {
			// A validator/repair is a game-data writer. Panel maintenance must own
			// the shared group's stop/backup/restore boundary before enabling it.
			values = append(values, `      VALIDATE_ON_BOOT: "false"`, `      AUTO_DOWNLOAD_ON_BOOT: "false"`)
		}
		var missing []string
		for _, value := range values {
			key, _, _ := strings.Cut(strings.TrimSpace(value), ":")
			if strings.Contains(section, newline+"      "+key+":") {
				if !strings.Contains(section, newline+value+newline) {
					return text, fmt.Errorf("junimo140_compose_manual_required: custom %s.%s requires review", service, key)
				}
				continue
			}
			missing = append(missing, value)
		}
		if len(missing) > 0 {
			section = strings.Replace(section, "    environment:"+newline, "    environment:"+newline+strings.Join(missing, newline)+newline, 1)
		}
		updated = updated[:start] + section + updated[end:]
	}
	return updated, nil
}
