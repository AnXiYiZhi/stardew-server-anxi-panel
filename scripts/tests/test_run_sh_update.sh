#!/usr/bin/env bash

set -Eeuo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

unset PANEL_VERSION
source "$ROOT_DIR/deploy/run.sh"

tmp_dir="$(mktemp -d)"
trap 'rm -rf "$tmp_dir"' EXIT

INSTALL_DIR="$tmp_dir"
ENV_FILE="$tmp_dir/.env"
PANEL_VERSION="latest"
PANEL_VERSION_WAS_SET=""
FAKE_INSTALLED_VERSION="0.2.2"
installed_panel_version() { printf '%s\n' "$FAKE_INSTALLED_VERSION"; }
cat >"$ENV_FILE" <<'EOF'
PANEL_IMAGE=anxiyizhi/stardew-server-anxi-panel:0.2.2
PANEL_VERSION=0.2.2
EOF

curl() {
  printf '%s\n' '{"tag_name":"v0.2.6"}'
}

prepare_update_target
[[ "$PANEL_VERSION" == "0.2.6" ]]
[[ "$PANEL_VERSION_WAS_SET" == "resolved-latest" ]]
mapfile -t candidates < <(candidate_images)
first_candidate="${candidates[0]}"
[[ "$first_candidate" == *":0.2.6" ]]
[[ "$first_candidate" != *":0.2.2" ]]

PANEL_VERSION="0.2.5"
PANEL_VERSION_WAS_SET="x"
curl() {
  return 99
}
prepare_update_target
[[ "$PANEL_VERSION" == "0.2.5" ]]

PANEL_VERSION="latest"
PANEL_VERSION_WAS_SET=""
curl() {
  return 1
}
if prepare_update_target >/dev/null 2>&1; then
  echo "expected latest-version resolution failure" >&2
  exit 1
fi
[[ "$PANEL_VERSION" == "latest" ]]

FAKE_INSTALLED_VERSION=0.7.2
PANEL_VERSION=0.8.0
PANEL_VERSION_WAS_SET=x
if prepare_update_target >/dev/null 2>&1; then
  echo "legacy installation bypassed the preparation release" >&2
  exit 1
fi
FAKE_INSTALLED_VERSION=0.7.3
PANEL_VERSION=latest
PANEL_VERSION_WAS_SET=""
prepare_update_target
[[ "$PANEL_UPDATE_HANDOFF" == 1 ]]
[[ "$PANEL_VERSION" == latest ]]
PANEL_VERSION=0.7.3
PANEL_VERSION_WAS_SET=resolved-latest
prepare_update_target
[[ "$PANEL_UPDATE_HANDOFF" == 1 ]]
FAKE_INSTALLED_VERSION=0.8.0
PANEL_VERSION=0.8.0
PANEL_VERSION_WAS_SET=x
prepare_update_target
[[ "$PANEL_UPDATE_HANDOFF" == 0 ]]
PANEL_VERSION=0.7.3
prepare_update_target
[[ "$PANEL_UPDATE_HANDOFF" == 1 ]]
[[ "$PANEL_VERSION" == latest && -z "$PANEL_VERSION_WAS_SET" ]]

check_prerequisites() { :; }
print_access_info() { :; }
ensure_files() { echo 'handoff mutated deployment files' >&2; return 99; }
resolve_panel_image() { echo 'handoff pulled an image' >&2; return 99; }
compose_cmd() { echo 'handoff recreated containers' >&2; return 99; }
PANEL_VERSION=latest
PANEL_VERSION_WAS_SET=""
update_panel
force_update_panel

choose_image_source_interactive() { :; }
ensure_files() { :; }
compose_cmd() { :; }
resolve_panel_image() { [[ "$1" == "$fixture_expected_pull" ]]; }
PANEL_HOST_DATA_DIR="$tmp_dir/fresh-data"
mkdir -p "$PANEL_HOST_DATA_DIR/updater"
FAKE_INSTALLED_VERSION=""
PANEL_VERSION=latest
fixture_expected_pull=true
start_panel

touch "$PANEL_HOST_DATA_DIR/panel.db"
fixture_expected_pull=false
start_panel
rm "$PANEL_HOST_DATA_DIR/panel.db"
touch "$PANEL_HOST_DATA_DIR/updater/apply-status.json"
start_panel
rm "$PANEL_HOST_DATA_DIR/updater/apply-status.json"
FAKE_INSTALLED_VERSION=0.7.3
start_panel
FAKE_INSTALLED_VERSION=""
PANEL_VERSION=0.8.0
start_panel

echo "run.sh update target tests passed"
