#!/usr/bin/env bash
set -euo pipefail
repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
# shellcheck disable=SC1090
source <(sed -n '/^finish_candidate_upgrade() {/,/^}/p' "$repo_root/scripts/release-candidate.sh")
root="$(mktemp -d)"
trap 'rm -rf -- "$root"' EXIT
printf '{}\n' >"$root/source.safe.jsonl"
copy_fails=0
cp() {
  if ((copy_fails == 1)); then return 12; fi
  command cp "$@"
}
for original in 0 42; do
  for copy_fails in 0 1; do
    result=0
    finish_candidate_upgrade "$original" "$root/source.safe.jsonl" "$root/output.safe.jsonl" >"$root/public.log" || result=$?
    expected="$original"
    if ((original == 0 && copy_fails == 1)); then expected=12; fi
    [[ "$result" == "$expected" ]]
    grep -Fq "upgradeExit=$original" "$root/public.log"
    if ((copy_fails == 1)); then grep -Fq 'exportExit=12 reason=copy_failed' "$root/public.log"; fi
  done
done
echo 'candidate diagnostics: export failures preserve the original upgrade exit status'
