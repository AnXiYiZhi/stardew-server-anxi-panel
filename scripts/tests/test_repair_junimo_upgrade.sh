#!/usr/bin/env bash
set -Eeuo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
test_root="$(mktemp -d "${TMPDIR:-/tmp}/anxi-repair-script-test.XXXXXX")"
# shellcheck disable=SC2317 # invoked through EXIT/INT/TERM traps
cleanup() { rm -rf -- "$test_root"; }
trap cleanup EXIT INT TERM
mkdir -p "$test_root/bin" "$test_root/state"
printf 'admin-password\n' >"$test_root/password"
chmod 600 "$test_root/password"

cat >"$test_root/bin/curl" <<'EOF'
#!/usr/bin/env bash
set -Eeuo pipefail
output=""
data=""
url="${!#}"
while (($#)); do
  case "$1" in
    --output) output="$2"; shift 2 ;;
    --data-binary) data="$2"; shift 2 ;;
    *) shift ;;
  esac
done
case "$url" in
  */api/auth/login)
    printf '{"user":{"role":"admin"}}' >"$output"
    printf '200'
    ;;
  */junimo-update/apply)
    if [[ -f "$FAKE_REPAIR_STATE/repaired" ]]; then
      printf '{"phase":"succeeded","applyId":"apply_bbbbbbbbbbbbbbbbbbbbbbbb","repairSourceApplyId":"apply_aaaaaaaaaaaaaaaaaaaaaaaa","repairAttempts":1}' >"$output"
		elif [[ "${FAKE_KNOWN_CONFIG:-0}" == "1" ]]; then
			printf '{"phase":"failed_rolled_back","applyId":"apply_aaaaaaaaaaaaaaaaaaaaaaaa"}' >"$output"
    else
      printf '{"phase":"rollback_failed","applyId":"apply_aaaaaaaaaaaaaaaaaaaaaaaa","rollbackCode":"rollback_restore_auth_volume_failed"}' >"$output"
    fi
    printf '200'
    ;;
  */junimo-update/repair)
    [[ "$data" == @* ]] || exit 91
    cp -- "${data#@}" "$FAKE_REPAIR_STATE/repair-body"
    if [[ "${FAKE_REPAIR_REJECT:-0}" == "1" ]]; then
      printf '{"code":"recovery_material_invalid","message":"rejected"}' >"$output"
      printf '409'
      exit 0
    fi
    : >"$FAKE_REPAIR_STATE/repaired"
    printf '{"phase":"rolling_back","repairAttempts":1}' >"$output"
    printf '202'
    ;;
  */junimo-update)
		if [[ "${FAKE_KNOWN_CONFIG:-0}" == "1" ]]; then
			printf '{"repairable":true,"repairCode":"repairable/legacy_candidates"}' >"$output"
		else
			printf '{"repairable":false}' >"$output"
		fi
		printf '200'
		;;
  *) exit 92 ;;
esac
EOF
cat >"$test_root/bin/sleep" <<'EOF'
#!/usr/bin/env bash
exit 0
EOF
chmod +x "$test_root/bin/curl" "$test_root/bin/sleep"

run_script() {
  PATH="$test_root/bin:$PATH" \
    PANEL_URL="http://127.0.0.1:18090" \
    PANEL_USERNAME="admin" \
    PANEL_PASSWORD_FILE="$test_root/password" \
    FAKE_REPAIR_STATE="$test_root/state" \
    FAKE_REPAIR_REJECT="${FAKE_REPAIR_REJECT:-0}" \
		FAKE_KNOWN_CONFIG="${FAKE_KNOWN_CONFIG:-0}" \
    bash "$repo_root/deploy/repair-junimo-upgrade.sh" "$@"
}

run_script check >/dev/null
[[ ! -e "$test_root/state/repair-body" ]] || { echo "check unexpectedly submitted repair" >&2; exit 1; }

run_script repair >/dev/null
[[ "$(tr -d '\r\n' <"$test_root/state/repair-body")" == '{"confirm":true}' ]] || { echo "repair body was not strict confirmation" >&2; exit 1; }

rm -f -- "$test_root/state/repaired" "$test_root/state/repair-body"
if FAKE_REPAIR_REJECT=1 run_script repair >"$test_root/rejected.out" 2>&1; then
  echo "backend rejection unexpectedly succeeded" >&2
  exit 1
fi
grep -q 'recovery_material_invalid' "$test_root/rejected.out" || { echo "backend rejection code was not reported" >&2; exit 1; }

rm -f -- "$test_root/state/repaired" "$test_root/state/repair-body"
FAKE_KNOWN_CONFIG=1 run_script repair >/dev/null
[[ "$(tr -d '\r\n' <"$test_root/state/repair-body")" == '{"confirm":true}' ]] || { echo "known config repair body was not strict confirmation" >&2; exit 1; }

echo "repair-junimo-upgrade script tests passed"
