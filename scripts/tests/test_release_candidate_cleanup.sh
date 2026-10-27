#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
command -v jq >/dev/null
# Load only the cleanup function; never execute a candidate build or Docker call.
# shellcheck disable=SC1090
source <(sed -n '/^remove_owned_dind() {/,/^}/p' "$repo_root/scripts/release-candidate.sh")
declare -F remove_owned_dind >/dev/null

owner="cleanup-test"
dind_volume="cleanup-test-data"
container_removed=0
volume_removed=0
fixture=""
docker() {
  case "$1 $2" in
    'container inspect')
      [[ "$fixture" != inspect-failed ]] || return 1
      local actual_owner="$owner" actual_volume="$dind_volume"
      [[ "$fixture" != foreign-container ]] || actual_owner="foreign"
      [[ "$fixture" != wrong-mount ]] || actual_volume="foreign-data"
      jq -n --arg owner "$actual_owner" --arg volume "$actual_volume" '[{Config:{Labels:{"com.anxi-panel.test-owner":$owner}},Mounts:[{Destination:"/var/lib/docker",Type:"volume",Name:$volume}]}]'
      ;;
    'volume inspect')
      local actual_owner="$owner"
      [[ "$fixture" != foreign-volume ]] || actual_owner="foreign"
      jq -n --arg owner "$actual_owner" '[{Labels:{"com.anxi-panel.test-owner":$owner}}]'
      ;;
    'rm -f')
      [[ "$fixture" != remove-failed ]] || return 1
      container_removed=$((container_removed + 1))
      ;;
    'volume rm') volume_removed=$((volume_removed + 1)) ;;
    *) echo "unexpected Docker operation: $1 $2" >&2; return 1 ;;
  esac
}

for fixture in success foreign-container wrong-mount foreign-volume inspect-failed remove-failed not-created; do
  dind_container_id="owned-container-id"
  dind_volume_created=1
  container_removed=0
  volume_removed=0
  if [[ "$fixture" == not-created ]]; then
    dind_container_id=""
    dind_volume_created=0
  fi
  result=0
  remove_owned_dind || result=$?
  case "$fixture" in
    success)
      [[ "$result" == 0 && "$container_removed" == 1 && "$volume_removed" == 1 && -z "$dind_container_id" && "$dind_volume_created" == 0 ]]
      remove_owned_dind
      [[ "$container_removed" == 1 && "$volume_removed" == 1 ]]
      ;;
    not-created) [[ "$result" == 0 && "$container_removed" == 0 && "$volume_removed" == 0 ]] ;;
    foreign-volume) [[ "$result" != 0 && "$container_removed" == 1 && "$volume_removed" == 0 && "$dind_volume_created" == 1 ]] ;;
    *) [[ "$result" != 0 && "$container_removed" == 0 && "$volume_removed" == 0 && -n "$dind_container_id" ]] ;;
  esac
  echo "candidate cleanup: $fixture passed"
done
