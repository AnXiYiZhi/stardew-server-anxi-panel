#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
command -v jq >/dev/null
for function_name in semver_at_least record_update_http_result start_apply; do
  # Load only these functions; never start the candidate fixture or Docker.
  # shellcheck disable=SC1090
  source <(sed -n "/^$function_name() {/,/^}/p" "$repo_root/scripts/tests/test_release_candidate_upgrade.sh")
  declare -F "$function_name" >/dev/null
done

root="$(mktemp -d)"
trap 'rm -rf -- "$root"' EXIT
response_file="$root/response.json"
# shellcheck disable=SC2034 # Read by the sourced production start_apply function.
cookie_file="$root/cookies"
# shellcheck disable=SC2034 # Read by the sourced production start_apply function.
panel_port=18080
# shellcheck disable=SC2034 # Read by the sourced production start_apply function.
full_stack_update=0
version=0.8.1
notice_digest=reviewed-notice-digest
# shellcheck disable=SC2034 # Updated by the sourced production HTTP recorder.
update_http_result_sequence=0
safe_http_evidence="$root/http.safe.jsonl"
fixture_status=202

curl() {
  local output="" data="" method=GET argument
  while (($#)); do
    argument="$1"
    shift
    case "$argument" in
      --output) output="$1"; shift ;;
      --data) data="$1"; shift ;;
      --request) method="$1"; shift ;;
      --max-time|--cookie|--header|--write-out) shift ;;
      --silent|--show-error|--fail) ;;
      http://*) ;;
      *) echo "unexpected mock curl argument" >&2; return 1 ;;
    esac
  done
  if [[ "$method" == GET ]]; then
    printf '{}'
    return 22
  fi
  jq -nc --arg data "$data" '{data:$data}' >>"$root/requests.jsonl"
  if [[ "$fixture_status" == 202 ]]; then
    printf '{"updateId":"accepted-test-update","phase":"checking"}' >"$output"
  else
    printf '{"error":{"code":"update_precondition_failed","message":"private-sensitive-message"}}' >"$output"
  fi
  printf '%s' "$fixture_status"
}

for previous_version in 0.7.3 0.8.0 0.8.1; do
  : >"$root/requests.jsonl"
  start_apply >"$root/stdout"
  [[ "$(wc -l <"$root/requests.jsonl")" == 1 ]]
  jq -e --arg target "$version" --arg notice "$notice_digest" \
    '.data | fromjson | . == {confirmFullStack:true,targetVersion:$target,noticeDigest:$notice}' "$root/requests.jsonl" >/dev/null
  [[ "$accepted_update_id" == accepted-test-update ]]
done

previous_version=0.7.2
: >"$root/requests.jsonl"
start_apply >"$root/stdout"
jq -e '.data | fromjson | . == {confirmFullStack:true}' "$root/requests.jsonl" >/dev/null
[[ "$(wc -l <"$root/requests.jsonl")" == 1 ]]

# Older product support is unchanged; this fixture only exercises audited schemas.
# shellcheck disable=SC2034 # Read by the sourced production start_apply function.
previous_version=0.7.1
: >"$root/requests.jsonl"
if start_apply >"$root/stdout" 2>"$root/stderr"; then exit 1; fi
[[ ! -s "$root/requests.jsonl" ]]

# shellcheck disable=SC2034 # Read by the sourced production start_apply function.
previous_version=0.8.0
for fixture_status in 400 409; do
  : >"$root/requests.jsonl"
  if start_apply >"$root/stdout" 2>"$root/stderr"; then
    echo "rejected apply unexpectedly succeeded" >&2
    exit 1
  fi
  [[ "$(wc -l <"$root/requests.jsonl")" == 1 && -z "$accepted_update_id" ]]
  grep -Fq 'errorCode=update_precondition_failed' "$root/stdout"
  if grep -Fq private-sensitive-message "$root/stdout"; then exit 1; fi
done

printf '{"error":{"code":"private-sensitive-code","message":"private-sensitive-message"}}' >"$response_file"
record_update_http_result diagnostic 409 0 >"$root/stdout"
grep -Fq 'errorCode=other' "$root/stdout"
if grep -Fq private-sensitive "$root/stdout"; then exit 1; fi
# shellcheck disable=SC2034 # Read by the sourced production HTTP recorder.
dry_run_id=private-test-dry-run
printf '{"id":"private-test-dry-run","phase":"succeeded","targetVersion":"v0.8.1"}' >"$response_file"
record_update_http_result dry-run-terminal 200 0 >"$root/stdout"
tail -n 1 "$safe_http_evidence" | jq -e '.sameId == true and .targetMatches == true and (.idSHA256 | test("^[a-f0-9]{64}$"))' >/dev/null
printf '{"id":"foreign-private-test-id","phase":"untrusted-phase","targetVersion":"0.8.2"}' >"$response_file"
record_update_http_result dry-run-terminal 200 0 >"$root/stdout"
tail -n 1 "$safe_http_evidence" | jq -e '.sameId == false and .targetMatches == false and .phase == "other"' >/dev/null
[[ "$(find "$root" -name '*.body.private.json' | wc -l)" == 9 ]]
if grep -Eq 'accepted-test-update|private-sensitive|reviewed-notice-digest' "$safe_http_evidence"; then exit 1; fi
if grep -Eq 'private-test-dry-run|foreign-private-test-id|untrusted-phase' "$safe_http_evidence"; then exit 1; fi
jq -e -s 'length == 9 and all(.[]; (keys | sort) == (["stage","httpStatus","curlExit","idSHA256","phase","errorCode","sameId","targetMatches"] | sort))' "$safe_http_evidence" >/dev/null
echo 'candidate apply: reviewed notice, strict protocol, single-write rejection and private diagnostics passed'
