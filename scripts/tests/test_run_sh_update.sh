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

echo "run.sh update target tests passed"
