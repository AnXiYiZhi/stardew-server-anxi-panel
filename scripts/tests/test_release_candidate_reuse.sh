#!/usr/bin/env bash
# The dynamically sourced production function reads/writes these test globals.
# shellcheck disable=SC2034,SC2154
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
command -v jq >/dev/null
# Exercise the production image selection function without a Docker daemon.
# shellcheck disable=SC1090
source <(sed -n '/^prepare_candidate_image() {/,/^}/p' "$repo_root/scripts/release-candidate.sh")
declare -F prepare_candidate_image >/dev/null

calls="$(mktemp)"
trap 'rm -f -- "$calls"' EXIT
version=0.7.3
commit=aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa
build_date=2026-10-27T00:00:00Z
candidate_image=anxi-reuse-test:immutable
fixture=""
docker() {
  printf '%s\n' "$1" >>"$calls"
  case "$1" in
    build) [[ "$fixture" != build-failed ]] ;;
    image)
      [[ "${2:-}" == inspect && "$fixture" != missing ]] || return 1
      local actual_version="$version" actual_commit="$commit" actual_date="$build_date" actual_id
      actual_id="sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
      case "$fixture" in
        wrong-version) actual_version=0.8.0 ;;
        wrong-commit) actual_commit=cccccccccccccccccccccccccccccccccccccccc ;;
        wrong-date) actual_date=2026-10-27T00:01:00Z ;;
        invalid-id) actual_id=unknown ;;
      esac
      jq -n --arg id "$actual_id" --arg version "$actual_version" --arg commit "$actual_commit" --arg date "$actual_date" \
        '[{Id:$id,Config:{Labels:{"org.opencontainers.image.version":$version,"org.opencontainers.image.revision":$commit,"org.opencontainers.image.created":$date}}}]'
      ;;
    *) echo "forbidden Docker operation in image selection: $1" >&2; return 1 ;;
  esac
}

reuse_image=0
prepare_candidate_image
[[ "$(cat "$calls")" == $'build\nimage' ]]
[[ "$image_id" == sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb ]]

reuse_image=1
: >"$calls"
prepare_candidate_image
[[ "$(cat "$calls")" == image ]]

for fixture in missing wrong-version wrong-commit wrong-date invalid-id; do
  : >"$calls"
  if prepare_candidate_image; then
    echo "candidate reuse unexpectedly accepted $fixture" >&2
    exit 1
  fi
  [[ "$(cat "$calls")" == image ]]
  echo "candidate reuse: $fixture rejected without build or pull"
done

reuse_image=0
fixture=build-failed
: >"$calls"
if prepare_candidate_image; then
  echo 'candidate reuse ignored the failed primary build' >&2
  exit 1
fi
[[ "$(cat "$calls")" == build ]]
echo 'candidate reuse: immutable identity and no-rebuild contract passed'
