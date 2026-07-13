package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/updater"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "panel-updater requires a fixed operation")
		os.Exit(2)
	}
	switch os.Args[1] {
	case "dry-run":
		runDryRun()
	case "apply":
		runApply()
	default:
		fmt.Fprintln(os.Stderr, "unsupported panel-updater operation")
		os.Exit(2)
	}
}

func runDryRun() {
	flags := flag.NewFlagSet("dry-run", flag.ContinueOnError)
	targetVersion := flags.String("target-version", "", "exact target version")
	composeProject := flags.String("compose-project", "", "compose project")
	composeFile := flags.String("compose-file", "", "compose file")
	stateFile := flags.String("state-file", "/data/updater/status.json", "state file")
	currentImage := flags.String("current-image", "", "current panel image")
	if err := flags.Parse(os.Args[2:]); err != nil || flags.NArg() != 0 {
		os.Exit(2)
	}
	if err := updater.RunDryRun(context.Background(), updater.DryRunOptions{
		TargetVersion: *targetVersion, CurrentImage: *currentImage,
		ComposeProject: *composeProject, ComposeFile: *composeFile, StateFile: *stateFile,
	}); err != nil {
		fmt.Fprintln(os.Stderr, "panel updater dry-run failed; inspect the persisted status for details")
		os.Exit(1)
	}
}

func runApply() {
	flags := flag.NewFlagSet("apply", flag.ContinueOnError)
	fromVersion := flags.String("from-version", "", "exact current version")
	targetVersion := flags.String("target-version", "", "exact target version")
	currentImage := flags.String("current-image", "", "current panel image")
	originalDigest := flags.String("original-digest", "", "current image digest")
	currentContainer := flags.String("current-container", "", "current panel container")
	composeProject := flags.String("compose-project", "", "compose project")
	composeFile := flags.String("compose-file", "", "compose file")
	stateFile := flags.String("state-file", "/data/updater/apply-status.json", "state file")
	backupDir := flags.String("backup-dir", "", "protected backup directory")
	databaseRelative := flags.String("database-relative", "panel.db", "database path relative to /data")
	if err := flags.Parse(os.Args[2:]); err != nil || flags.NArg() != 0 {
		os.Exit(2)
	}
	if err := updater.RunApply(context.Background(), updater.ApplyOptions{
		FromVersion: *fromVersion, TargetVersion: *targetVersion, CurrentImage: *currentImage,
		OriginalDigest: *originalDigest, CurrentContainer: *currentContainer,
		ComposeProject: *composeProject, ComposeFile: *composeFile,
		StateFile: *stateFile, BackupDir: *backupDir, DatabaseRelative: *databaseRelative,
	}); err != nil {
		fmt.Fprintln(os.Stderr, "panel updater apply failed; inspect persisted status for rollback result")
		os.Exit(1)
	}
}
