#!/usr/bin/env bash
set -Eeuo pipefail

usage() {
  echo "Usage: $0 --version <x.y.z> [--base-ref <git-ref>]" >&2
}

version=""
base_ref=""
while (($# > 0)); do
  case "$1" in
    --version)
      version="${2:-}"
      shift 2
      ;;
    --base-ref)
      base_ref="${2:-}"
      shift 2
      ;;
    *)
      usage
      exit 2
      ;;
  esac
done

if [[ ! "$version" =~ ^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$ ]]; then
  echo "release gates: --version must be an exact stable semantic version" >&2
  exit 2
fi

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo_root"

python_bin="${PYTHON_BIN:-python3}"
for command_name in git go npm docker shellcheck "$python_bin"; do
  if ! command -v "$command_name" >/dev/null 2>&1; then
    echo "release gates: missing required command: $command_name" >&2
    exit 1
  fi
done

if [[ -n "$base_ref" ]]; then
  git rev-parse --verify "$base_ref^{commit}" >/dev/null
fi

changed_since_base() {
  if [[ -z "$base_ref" ]]; then
    return 0
  fi
  ! git diff --quiet "$base_ref...HEAD" -- "$@"
}

echo "release gates: compatibility contracts"
"$python_bin" scripts/compatibility_matrix.py validate backend/internal/games/stardew_junimo/config/runtime_stack_manifest.json
"$python_bin" scripts/compatibility_matrix.py check-panel-version backend/internal/games/stardew_junimo/config/runtime_stack_manifest.json --version "$version"
"$python_bin" -m unittest discover -s scripts/tests -p 'test_compatibility_matrix.py'

if changed_since_base backend/internal/games/stardew_junimo/config/runtime_stack_manifest.json scripts/compatibility_matrix.py; then
  echo "release gates: runtime manifest changed; verifying remote artifacts"
  "$python_bin" scripts/compatibility_matrix.py verify-remote-artifacts backend/internal/games/stardew_junimo/config/runtime_stack_manifest.json
else
  echo "release gates: remote artifact verification skipped; manifest inputs are unchanged"
fi

echo "release gates: deployment scripts"
bash scripts/tests/test_run_sh_update.sh
bash scripts/tests/test_run_sh_swap.sh
bash scripts/tests/test_migrate_fnos.sh
bash scripts/tests/test_repair_junimo_upgrade.sh

release_shell_files=(
  deploy/run.sh
  deploy/migrate-fnos.sh
  deploy/repair-junimo-0.3.5.sh
  deploy/repair-junimo-upgrade.sh
  scripts/run-release-gates.sh
  scripts/release-candidate.sh
  scripts/tests/test_run_sh_update.sh
  scripts/tests/test_run_sh_swap.sh
  scripts/tests/test_migrate_fnos.sh
  scripts/tests/test_repair_junimo_upgrade.sh
  scripts/tests/test_release_candidate_upgrade.sh
)
for shell_file in "${release_shell_files[@]}"; do
  bash -n "$shell_file"
done
shellcheck "${release_shell_files[@]}"

echo "release gates: backend"
(
  cd backend
  go test ./...
  go vet ./...
  go build ./...
  PANEL_RUN_DOCKER_UPDATE_TEST=1 go test ./internal/updater -run '^TestDockerIntegration' -count=1
  go test -tags=integration ./internal/docker
)

if changed_since_base backend/internal/games/stardew_junimo scripts/compatibility_matrix.py; then
  echo "release gates: Junimo runtime changed; running real network/runtime integration"
  (
    cd backend
    PANEL_RUN_SMAPI_DOWNLOAD_TEST=1 go test -tags=integration ./internal/games/stardew_junimo -run '^TestSMAPIArchiveRealDownload$' -count=1 -v
    go test -tags=integration ./internal/games/stardew_junimo -run '^TestRuntimeUpdateAuthAcceptanceUsesPureHealthAndNeverCallsSteamReady$' -count=1 -v
  )
else
  echo "release gates: Junimo real network/runtime integration skipped; affected inputs are unchanged"
fi

echo "release gates: frontend regression and production build"
(
  cd frontend
  npm ci
  npm audit --omit=dev --audit-level=high
  npm run test:command-results
  npm run test:update-status
  npm run test:panel-update
  npm run test:junimo-update
  npm run test:component-update-flow
  npm run test:runtime-components
  npm run test:smapi-update
  npm run test:install-state
  npm run test:new-game-idempotency
  npm run test:nexus-extension-idempotency
  npm run test:cabin-strategy-options
  npm run test:farm-catalog
  npm run test:save-import
  npm run test:save-backup-details
  npm run test:mod-list
  npm run test:responsive-layout
  npm run test:player-mods
  npm run build
)

if changed_since_base website docs README.md; then
  echo "release gates: public documentation changed; building website"
  (
    cd website
    npm ci
    npm audit --omit=dev --audit-level=high
    npm run docs:build
  )
else
  echo "release gates: website build skipped; public documentation inputs are unchanged"
fi

echo "release gates: all selected gates passed for $version"
