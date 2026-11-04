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

for function_name in record_panel_stop_fixture_result wait_for_panel_stop_fixture; do
  # shellcheck disable=SC1090 # Load the actual production observer without Docker setup.
  source <(sed -n "/^$function_name() {/,/^}/p" "$source_file")
done
for stop_mode in succeeded failed wrong-job wrong-server timeout; do (
  root="$root/stop-$stop_mode"
  mkdir "$root"
  unset safe_http_evidence
  expected_job="job_$(printf 'a%.0s' {1..32})"
  expected_server="$(printf 'b%.0s' {1..64})"
  # shellcheck disable=SC2329 # Called by the sourced production observer.
  curl() {
    local output="" argument url="" id="$expected_job" status=succeeded
    while (($#)); do
      argument="$1"; shift
      case "$argument" in
        --output) output="$1"; shift ;;
        --write-out|--cookie) shift ;;
        --silent|--show-error|--fail) ;;
        http://*) url="$argument" ;;
        *) return 1 ;;
      esac
    done
    if [[ "$url" == */api/jobs/* ]]; then
      case "$stop_mode" in
        failed) status=failed ;;
        wrong-job) id=another-job ;;
        timeout) status=running ;;
      esac
      jq -nc --arg id "$id" --arg status "$status" '{job:{id:$id,status:$status,targetId:"stardew",type:"stardew_lifecycle",operation:"stop",errorMessage:"fixture-secret"}}' >"$output"
      printf 200
    else
      printf '{"state":"stopped"}'
    fi
  }
  # shellcheck disable=SC2329 # Called by the sourced production observer.
  docker() {
    if [[ "$1" == ps ]]; then
      if [[ "$stop_mode" == wrong-server ]]; then printf 'another-server\n'; else printf '%s\n' "$expected_server"; fi
    elif [[ "$1" == inspect ]]; then
      printf '[{"State":{"Status":"exited"}}]\n'
    else
      return 1
    fi
  }
  result=0
  wait_for_panel_stop_fixture "$expected_job" "$expected_server" stardew || result=$?
  [[ "$(cat "$root/budget.txt")" == '1 150' ]]
  safe="$root/update-http-evidence.safe.jsonl"
  [[ -s "$safe" ]]
  jq -e '.errorMessageSHA256 | test("^[a-f0-9]{64}$")' "$safe" >/dev/null
  jq -e '.reason == "other"' "$safe" >/dev/null
  if [[ "$stop_mode" == succeeded ]]; then
    [[ "$result" == 0 ]]
    jq -e '.phase == "succeeded" and .sameId and .targetMatches and .errorCode == "absent"' "$safe" >/dev/null
  else
    [[ "$result" != 0 ]]
    case "$stop_mode" in
      failed) expected_error=panel_stop_job_failed ;;
      wrong-job) expected_error=panel_stop_identity_changed ;;
      wrong-server) expected_error=panel_stop_target_changed ;;
      timeout) expected_error=panel_stop_observation_timeout ;;
    esac
    jq -e --arg code "$expected_error" '.errorCode == $code' "$safe" >/dev/null
  fi
  if grep -F -e fixture-secret -e "$expected_job" -e "$expected_server" "$safe" >/dev/null; then
    echo "Panel Stop diagnostic retained a raw identifier or message" >&2
    exit 1
  fi
) done
printf 'candidate Panel Stop observer tests passed\n'
