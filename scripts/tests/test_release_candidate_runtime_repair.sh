#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
command -v jq >/dev/null
root="$(mktemp -d)"
trap 'rm -rf -- "$root"' EXIT
source_file="$repo_root/scripts/tests/test_release_candidate_upgrade.sh"
observer="$root/observer.sh"
awk '
  /^assert_upgraded_legacy_junimo_repair\(\) \{/ { active=1 }
  active && /^  for _ in \$\(seq 1 2100\); do/ { copying=1 }
  copying { print }
  copying && /^  chmod 600 "\$root\/runtime-repair-final.private.json"/ { exit }
' "$source_file" >"$observer"
[[ -s "$observer" ]]

# The extracted production observer reads these inputs.
# shellcheck disable=SC2034
cookie_file="$root/cookie"
# shellcheck disable=SC2034
panel_port=18080
response_file="$root/response.json"
repair_apply_id=apply-fixture
repair_job_id=job-fixture
repair_phase=""
case_name=valid

# shellcheck disable=SC2329 # Called by the sourced production observer.
seq() {
  printf '%s %s\n' "$1" "$2" >"$root/budget.txt"
  printf '1\n2\n'
}

# shellcheck disable=SC2329 # Called by the sourced production observer.
sleep() { :; }

# shellcheck disable=SC2329 # Called by the sourced production observer.
curl() {
  local count=0 observed_apply="$repair_apply_id" observed_job="$repair_job_id" phase=rolling_back
  if [[ -f "$root/get-count" ]]; then count="$(cat "$root/get-count")"; fi
  count=$((count + 1))
  printf '%s\n' "$count" >"$root/get-count"
  if ((count > 1)); then phase=succeeded; fi
  case "$case_name" in
    wrong-apply) observed_apply=another-apply ;;
    wrong-job) observed_job=another-job ;;
  esac
  jq -nc --arg applyId "$observed_apply" --arg jobId "$observed_job" --arg phase "$phase" \
    '{applyId:$applyId,jobId:$jobId,phase:$phase}'
}

# shellcheck disable=SC1090 # Exact observer extracted from the production fixture.
source "$observer"
[[ "$repair_phase" == succeeded && "$(cat "$root/get-count")" == 2 ]]
[[ "$(cat "$root/budget.txt")" == '1 2100' ]]
cmp "$response_file" "$root/runtime-repair-final.private.json"
for case_name in wrong-apply wrong-job; do
  rm "$root/get-count"
  # shellcheck disable=SC1090 # Same exact production observer, negative identity case.
  if (source "$observer") >"$root/$case_name.stdout" 2>"$root/$case_name.stderr"; then
    echo "runtime repair observer accepted another operation" >&2
    exit 1
  fi
  [[ "$(cat "$root/get-count")" == 1 ]]
  grep -q 'changed operation identity' "$root/$case_name.stderr"
done
printf 'candidate runtime repair observer tests passed\n'
