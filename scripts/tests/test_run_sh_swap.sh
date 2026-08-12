#!/usr/bin/env bash

set -Eeuo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

source "$ROOT_DIR/deploy/run.sh"

tmp_dir="$(mktemp -d)"
trap 'rm -rf -- "$tmp_dir"' EXIT INT TERM

run_as_root() {
  "$@"
}

proc_value="$tmp_dir/swappiness"
sysctl_conf="$tmp_dir/sysctl.conf"
dropin_dir="$tmp_dir/sysctl.d"
mkdir -p "$dropin_dir"
printf '0\n' >"$proc_value"
printf 'vm.overcommit_memory = 1\nvm.swappiness=0\n' >"$sysctl_conf"

configure_swappiness "$proc_value" "$sysctl_conf" "$dropin_dir"
[[ "$(<"$proc_value")" == "60" ]]
dropin="$dropin_dir/99-zz-stardew-anxi-panel-swappiness.conf"
grep -qE '^[[:space:]]*vm[.]swappiness[[:space:]]*=[[:space:]]*60[[:space:]]*$' "$dropin"
[[ "$(grep -cE '^[[:space:]]*vm[.]swappiness[[:space:]]*=' "$sysctl_conf")" == "1" ]]
grep -qE '^vm[.]swappiness = 60$' "$sysctl_conf"
grep -qE '^vm[.]overcommit_memory = 1$' "$sysctl_conf"

printf '10\n' >"$proc_value"
printf 'vm.swappiness = 1\n' >"$dropin"
configure_swappiness "$proc_value" "$sysctl_conf" "$dropin_dir"
[[ "$(<"$proc_value")" == "60" ]]
[[ "$(grep -cE '^[[:space:]]*vm[.]swappiness[[:space:]]*=' "$dropin")" == "1" ]]

fallback_root="$tmp_dir/fallback"
fallback_conf="$fallback_root/sysctl.conf"
missing_dropin="$fallback_root/missing-sysctl.d"
mkdir -p "$fallback_root"
printf 'vm.overcommit_memory = 1\nvm.swappiness=0\n  vm.swappiness = 10\n' >"$fallback_conf"
printf '20\n' >"$proc_value"
configure_swappiness "$proc_value" "$fallback_conf" "$missing_dropin"
[[ "$(<"$proc_value")" == "60" ]]
[[ "$(grep -cE '^[[:space:]]*vm[.]swappiness[[:space:]]*=' "$fallback_conf")" == "1" ]]
grep -qE '^vm[.]swappiness = 60$' "$fallback_conf"
grep -qE '^vm[.]overcommit_memory = 1$' "$fallback_conf"

fake_swaps="$tmp_dir/proc-swaps"
printf 'Filename Type Size Used Priority\n/swapfile file 2097148 0 -2\n' >"$fake_swaps"
printf '0\n' >"$proc_value"
setup_swap 2 "$fake_swaps" "$proc_value" "$sysctl_conf" "$dropin_dir" >/dev/null
[[ "$(<"$proc_value")" == "60" ]]

if configure_swappiness "$tmp_dir/missing-proc" "$sysctl_conf" "$dropin_dir" >/dev/null 2>&1; then
  echo "expected missing vm.swappiness path to fail" >&2
  exit 1
fi

echo "run.sh swap/swappiness tests passed"
