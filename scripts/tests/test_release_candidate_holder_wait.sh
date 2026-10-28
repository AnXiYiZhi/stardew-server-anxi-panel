#!/usr/bin/env bash
# Production functions loaded below deliberately share these fixture globals.
# shellcheck disable=SC2034,SC2154
set -euo pipefail

# Observer functions read SECONDS. Unsetting its Bash special behavior gives the
# fixture a virtual clock that advances only through the mocked sleep below.
unset SECONDS
SECONDS=0

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
command -v jq >/dev/null
for function_name in read_legacy_runtime_fingerprint observe_unknown_holder_bootstrap_rejection report_legacy_bootstrap_diagnostics assert_unknown_session_holder_boundary wait_unknown_session_holder_convergence assert_unknown_session_holder_fail_closed legacy_steam_invite_persistence_ready record_update_http_result; do
  # Load the actual observer only; never execute the candidate/Docker fixture.
  # shellcheck disable=SC1090
  source <(sed -n "/^$function_name() {/,/^}/p" "$repo_root/scripts/tests/test_release_candidate_upgrade.sh")
  declare -F "$function_name" >/dev/null
done

root="$(mktemp -d /tmp/anxi-holder-wait-test.XXXXXX)"
cleanup_holder_test() {
  local test_status=$?
  if ((test_status != 0)); then
    printf 'candidate holder wait regression failed: case=%s ticks=%s reason=%s intentReady=%s composeReady=%s holderRejected=%s\n' \
      "${fixture:-setup}" "${ticks:-0}" "${unknown_holder_boundary_reason:-not-observed}" \
      "${unknown_holder_intent_ready:-0}" "${unknown_holder_compose_ready:-0}" "${unknown_holder_rejection_seen:-0}" >&2
  fi
  if [[ "$root" == /tmp/anxi-holder-wait-test.* ]]; then rm -rf -- "$root"; fi
  exit "$test_status"
}
trap cleanup_holder_test EXIT
data_dir="$root/data"
mkdir -p "$data_dir/instances/stardew"
instance_env="$data_dir/instances/stardew/.env"
instance_compose="$data_dir/instances/stardew/docker-compose.yml"
legacy_expected_migrated_compose="$root/expected-compose"
printf 'safe dependency migration\n' >"$legacy_expected_migrated_compose"
legacy_server_container_id="server-original"
legacy_dependency_container_id="dependency-original"
legacy_auth_container_id="auth-original"
unknown_session_holder_id="unknown-original"
authorized_auth_container_id="$legacy_auth_container_id"
legacy_server_started_at="started-original"
steam_session_volume="fixture-session"
panel_container="fixture-panel"
panel_port=18080
cookie_file="$root/fixture.cookies"
response_file="$root/response.json"
legacy_disabled_session_hash="session-original"
fixture=""
ticks=0
fixture_session_hash="$legacy_disabled_session_hash"
auth_changed=0
restart_changed=0
replacement_changed=0
receipt_ready=0
auth_present=1
volume_present=1

docker() {
  case "$1" in
    ps)
      case "$*" in
        *service=server*)
          if ((replacement_changed == 1)); then printf 'server-replacement\n'; else printf '%s\n' "$legacy_server_container_id"; fi
          ;;
        *service=fixture-ready*) printf '%s\n' "$legacy_dependency_container_id" ;;
        *service=steam-auth*) if ((auth_present == 1)); then printf '%s\n' "$legacy_auth_container_id"; fi ;;
        *) return 1 ;;
      esac
      ;;
    inspect)
      shift
      local id started
      for id in "$@"; do
        started="started-original"
        if ((restart_changed == 1)) && [[ "$id" == "$legacy_auth_container_id" ]]; then started="started-again"; fi
        jq -nc --arg id "$id" --arg started "$started" '{Id:$id,Image:"immutable-image",State:{StartedAt:$started,Running:true}}'
      done | jq -s .
      ;;
    volume) [[ "$2" == inspect && "$volume_present" == 1 ]] ;;
    exec)
      [[ "$3" == sha256sum ]] || return 1
      printf '%s  private-session-file\n' "$fixture_session_hash"
      ;;
    logs)
      if ((receipt_ready == 1)); then
        jq -nc '{instance:"stardew",msg:"skip required runtime coordinator because instance prepare failed",error:"converge optional Steam Auth runtime scope: remove legacy optional Auth container: Steam invite session volume has an unknown container holder at holder 2",private:"never-print-private-fixture"}'
      elif [[ "$fixture" == wrong-reason || "$fixture" == wrong-instance ]]; then
        local instance="stardew" error="different private failure never-print-private-fixture"
        if [[ "$fixture" == wrong-instance ]]; then
          instance="other-private-instance"
          error="converge optional Steam Auth runtime scope: remove legacy optional Auth container: Steam invite session volume has an unknown container holder at holder 2"
        fi
        jq -nc --arg instance "$instance" --arg error "$error" '{instance:$instance,msg:"skip required runtime coordinator because instance prepare failed",error:$error}'
      fi
      ;;
    *) printf '%s\n' "$1" >>"$root/forbidden-docker-writes"; return 1 ;;
  esac
}

assert_candidate_auth_images_unchanged() { ((auth_changed == 0)); }
read_steam_session_sentinel_hash() { printf '%s\n' "$fixture_session_hash"; }

sleep() {
  ticks=$((ticks + 1))
  SECONDS=$((ticks * 10))
  case "$fixture:$ticks" in
    delayed:1) printf 'STEAM_INVITE_ENABLED=false\n' >"$instance_env" ;;
    delayed:2) cp "$legacy_expected_migrated_compose" "$instance_compose" ;;
    false-first:2) cp "$legacy_expected_migrated_compose" "$instance_compose"; receipt_ready=1 ;;
    delayed:3|pre-ready:2) receipt_ready=1 ;;
    marker:1) printf 'STEAM_INVITE_RUNTIME_SCOPE_VERSION=1\n' >>"$instance_env" ;;
    unsafe-compose:1) printf 'unsafe content never-print-private-fixture\n' >"$instance_compose" ;;
    true-intent:1) printf 'STEAM_INVITE_ENABLED=true\n' >"$instance_env" ;;
    duplicate-intent:1) printf 'STEAM_INVITE_ENABLED=false\nSTEAM_INVITE_ENABLED=false\n' >"$instance_env" ;;
    container-restart:1) restart_changed=1 ;;
    container-replacement:1) replacement_changed=1 ;;
    session-change:1) fixture_session_hash="changed-private-session" ;;
    auth-image-change:1) auth_changed=1 ;;
    intent-regression:1) : >"$instance_env" ;;
    compose-regression:1) printf 'original compose\n' >"$instance_compose" ;;
  esac
}

curl() {
  case "$*" in
    */state*)
      printf '{"steamInviteEnabled":false,"steamInviteAuthState":"disabled","steamAuthLoggedIn":false}\n'
      if [[ "$fixture" == state-get-marker ]]; then printf 'STEAM_INVITE_RUNTIME_SCOPE_VERSION=1\n' >>"$instance_env"; fi
      ;;
    */invite-code*)
      printf '{"status":"disabled","steamInviteEnabled":false}\n'
      if [[ "$fixture" == invite-get-session ]]; then fixture_session_hash="changed-private-session"; fi
      if [[ "$fixture" == invite-get-marker ]]; then printf 'STEAM_INVITE_RUNTIME_SCOPE_VERSION=1\n' >>"$instance_env"; fi
      ;;
    *) return 1 ;;
  esac
}

assert_public_log_private() {
  local log_file="$1" forbidden grep_status
  for forbidden in never-print-private-fixture other-private-instance server-original private-accepted-operation-id; do
    if grep -qF "$forbidden" "$log_file"; then
      echo 'candidate holder wait: public output contained a forbidden private value' >&2
      return 1
    else
      grep_status=$?
      if ((grep_status != 1)); then
        echo 'candidate holder wait: public output privacy check could not be completed' >&2
        return 1
      fi
    fi
  done
  return 0
}

reset_fixture() {
  fixture="$1"
  ticks=0
  SECONDS=0
  : >"$instance_env"
  printf 'original compose\n' >"$instance_compose"
  legacy_pre_upgrade_compose_hash="$(sha256sum "$instance_compose" | awk '{print $1}')"
  fixture_session_hash="$legacy_disabled_session_hash"
  auth_changed=0
  restart_changed=0
  replacement_changed=0
  receipt_ready=0
  auth_present=1
  volume_present=1
  unknown_holder_rejection_seen=0
  case "$fixture" in
    false-first|intent-regression) printf 'STEAM_INVITE_ENABLED=false\n' >"$instance_env" ;;
    pre-ready|wrong-reason|wrong-instance|state-get-marker|invite-get-session|invite-get-marker)
      printf 'STEAM_INVITE_ENABLED=false\n' >"$instance_env"
      cp "$legacy_expected_migrated_compose" "$instance_compose"
      ;;
    compose-regression) cp "$legacy_expected_migrated_compose" "$instance_compose" ;;
  esac
  if [[ "$fixture" == state-get-marker || "$fixture" == invite-get-session || "$fixture" == invite-get-marker ]]; then receipt_ready=1; fi
  legacy_disabled_runtime_fingerprint="$(read_legacy_runtime_fingerprint "$legacy_server_container_id" "$legacy_dependency_container_id" "$legacy_auth_container_id" "$unknown_session_holder_id")"
}

for fixture in delayed false-first pre-ready; do
  reset_fixture "$fixture"
  wait_unknown_session_holder_convergence 50 >"$root/public.log" 2>&1
  [[ "$ticks" -ge 2 && "$unknown_holder_rejection_seen" == 1 ]]
  echo "candidate holder wait: $fixture converged after read-only observation"
done

for fixture in timeout wrong-reason wrong-instance marker unsafe-compose true-intent duplicate-intent container-restart container-replacement session-change auth-image-change intent-regression compose-regression; do
  reset_fixture "$fixture"
  if [[ "$fixture" == timeout ]]; then
    # Slow setup must not shift the observer deadline or its exact tick count.
    command sleep 1
    [[ "$SECONDS" == 0 ]]
  fi
  if wait_unknown_session_holder_convergence 30 >"$root/public.log" 2>&1; then
    echo "candidate holder wait unexpectedly accepted $fixture" >&2
    exit 1
  fi
  case "$fixture" in
    timeout|wrong-reason|wrong-instance) [[ "$ticks" == 3 ]] ;;
    *) [[ "$ticks" == 1 ]] ;;
  esac
  assert_public_log_private "$root/public.log" || exit 1
  echo "candidate holder wait: $fixture rejected safely without a write"
done

for fixture in state-get-marker invite-get-session invite-get-marker; do
  reset_fixture "$fixture"
  if assert_unknown_session_holder_fail_closed >"$root/public.log" 2>&1; then
    echo "candidate holder wait ignored the boundary change after $fixture" >&2
    exit 1
  fi
  echo "candidate holder wait: $fixture rechecked after GET"
done

# A disabled API fallback does not prove persisted cleanup after Panel restart.
reset_fixture persistence
printf 'STEAM_INVITE_ENABLED=false\n' >"$instance_env"
cp "$legacy_expected_migrated_compose" "$instance_compose"
if legacy_steam_invite_persistence_ready false ''; then exit 1; fi
printf 'STEAM_INVITE_RUNTIME_SCOPE_VERSION=1\n' >>"$instance_env"
if legacy_steam_invite_persistence_ready false ''; then exit 1; fi
auth_present=0
volume_present=0
legacy_steam_invite_persistence_ready false ''
printf 'STEAM_INVITE_ENABLED=true\nSTEAM_INVITE_RUNTIME_SCOPE_VERSION=1\n' >"$instance_env"
auth_present=1
volume_present=1
legacy_steam_invite_persistence_ready true "$fixture_session_hash"
if legacy_steam_invite_persistence_ready true different-session; then exit 1; fi
echo 'candidate holder wait: restart requires persisted scope and exact resource convergence'

printf '{"updateId":"private-accepted-operation-id","phase":"succeeded","error":"never-print-private-fixture"}\n' >"$response_file"
record_update_http_result apply 202 0 >"$root/public.log"
jq -e '.httpStatus == "202" and .curlExit == 0 and .id == "private-accepted-operation-id"' "$root/update-http-evidence.private.jsonl" >/dev/null
assert_public_log_private "$root/public.log" || exit 1
record_update_http_result connection-closed 000 28 >"$root/public.log"
jq -se '.[1].httpStatus == "000" and .[1].curlExit == 28 and .[1].id == ""' "$root/update-http-evidence.private.jsonl" >/dev/null
[[ ! -e "$root/forbidden-docker-writes" ]]
echo 'candidate holder wait: HTTP identity stays private and observers never mutate Docker'

printf 'deliberate leak never-print-private-fixture\n' >"$root/privacy-probe.log"
if assert_public_log_private "$root/privacy-probe.log" >"$root/privacy-probe-output.log" 2>&1; then
  echo 'candidate holder wait: privacy assertion accepted deliberately leaking input' >&2
  exit 1
fi
assert_public_log_private "$root/privacy-probe-output.log" || exit 1
echo 'candidate holder wait: privacy assertion rejects deliberately leaking input'
