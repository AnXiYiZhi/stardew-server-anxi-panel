package stardew_junimo

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	paneldocker "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/docker"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/stardew_junimo/config"
)

const (
	junimo140InputFIFO = "/tmp/junimo/smapi-input"
	junimo140Digest    = "sha256:f23fdc3a3e999032be363fa43291c2d72626555913ef2b3dcf71b0aec53c49db"
)

type consoleRuntimeContract struct {
	fifo string
	uid  string
	gid  string
}

// Select from the instance's deployed image, never the recommended target: the
// same Panel must still be able to save and verify a rolled-back .125 runtime.
func readConsoleRuntimeContract(dataDir string) (consoleRuntimeContract, error) {
	legacy := consoleRuntimeContract{fifo: serverInputFIFO}
	values, err := config.ReadEnvFile(filepath.Join(dataDir, ".env"))
	if os.IsNotExist(err) {
		return legacy, nil
	}
	if err != nil {
		return legacy, fmt.Errorf("read console runtime contract: %w", err)
	}
	image := strings.TrimSpace(values["SERVER_IMAGE"])
	ref, digest, _ := strings.Cut(image, "@")
	tag := ref[strings.LastIndex(ref, ":")+1:]
	if image == "" || (strings.HasPrefix(image, "sha256:") && len(image) == 71) {
		tag = values["IMAGE_VERSION"]
	}
	if tag != "1.5.0-preview.140" && digest != junimo140Digest {
		return legacy, nil
	}
	contract := consoleRuntimeContract{fifo: junimo140InputFIFO, uid: "1000", gid: "1000"}
	for _, field := range []struct {
		key string
		dst *string
	}{{"USER_ID", &contract.uid}, {"GROUP_ID", &contract.gid}} {
		if value := strings.TrimSpace(values[field.key]); value != "" {
			id, parseErr := strconv.ParseUint(value, 10, 32)
			if parseErr != nil || id == 4294967295 {
				return legacy, fmt.Errorf("invalid console runtime %s", field.key)
			}
			*field.dst = strconv.FormatUint(id, 10)
		}
	}
	return contract, nil
}

func probeServerInputFIFO(ctx context.Context, exec commandExecutor, dir string) (consoleRuntimeContract, paneldocker.CommandResult, error) {
	contract, err := readConsoleRuntimeContract(dir)
	if err != nil {
		return contract, paneldocker.CommandResult{ExitCode: -1}, err
	}
	result, err := exec.ComposeExecPipe(ctx, dir, "server", "", "test", "-p", contract.fifo)
	if err == nil && result.ExitCode != 0 {
		err = fmt.Errorf("SMAPI input is not a FIFO (exit %d)", result.ExitCode)
	}
	return contract, result, err
}

// nocreat prevents an absent/replaced FIFO from becoming a regular command file.
// nonblock rejects a FIFO without a reader; the in-container timeout also bounds
// the writer if the Docker CLI is cancelled without terminating its exec child.
func consoleWriterArgs(contract consoleRuntimeContract) []string {
	args := []string{"timeout", "--kill-after=1s", "5s"}
	if contract.uid != "" {
		args = append(args, "setpriv", "--reuid="+contract.uid, "--regid="+contract.gid, "--clear-groups", "--")
	}
	return append(args, "dd", "of="+contract.fifo, "oflag=nonblock", "conv=nocreat", "status=none")
}

func writeServerInput(ctx context.Context, exec commandExecutor, dir, command string) (paneldocker.CommandResult, error) {
	contract, result, err := probeServerInputFIFO(ctx, exec, dir)
	if err != nil {
		return result, err
	}
	result, err = exec.ComposeExecPipe(ctx, dir, "server", command+"\n", consoleWriterArgs(contract)...)
	if err == nil && result.ExitCode != 0 {
		err = fmt.Errorf("SMAPI FIFO writer exited with code %d", result.ExitCode)
	}
	return result, err
}

func serverCommandResponseMatches(command, output string) bool {
	output = stripControlChars(output)
	switch command {
	case "info":
		return strings.Contains(output, "Players:") || strings.Contains(output, "--- Server Info ---")
	case "invitecode":
		return strings.Contains(output, "Invite code:") || strings.Contains(output, "No invite code available.") || strings.Contains(output, "Server is not running.")
	case "settings show":
		return strings.Contains(output, "Current Configuration (server-settings.json)")
	case "settings validate":
		return strings.Contains(output, "checks passed.")
	case "rendering status":
		return strings.Contains(output, "Rendering is disabled") || strings.Contains(output, "Rendering is at ")
	case "host-auto":
		return strings.Contains(output, "Host automation:")
	case "host-visibility":
		return strings.Contains(output, "Host farmhand is now ")
	default:
		if save, ok := strings.CutPrefix(command, "saves info "); ok {
			return isSaveInfoOutput(output, save) || strings.Contains(output, "Save '"+save+"' not found.") || strings.Contains(output, "Could not read save info for '"+save+"'.")
		}
		return false
	}
}
