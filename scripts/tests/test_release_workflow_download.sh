#!/usr/bin/env bash
set -Eeuo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
workflow="${RELEASE_WORKFLOW_FILE:-$repo_root/.github/workflows/release-after-candidate.yml}"
[[ -f "$workflow" ]] || { echo 'release workflow download: workflow missing' >&2; exit 1; }
fixture_root="$(mktemp -d "${TMPDIR:-/tmp}/anxi-release-workflow-download.XXXXXX")"
fixture_root="$(cd "$fixture_root" && pwd -P)"
[[ "${fixture_root##*/}" == anxi-release-workflow-download.* ]] || exit 1
trap 'rm -rf -- "$fixture_root"' EXIT
mkdir -p "$fixture_root/bin" "$fixture_root/outside"
fixture_cwd="$(cd "$fixture_root/outside" && pwd -P)"
fixture_parent="$fixture_cwd"
while :; do
  if [[ -e "$fixture_parent/.git" || -L "$fixture_parent/.git" ]]; then
    echo 'release workflow download: fixture must be outside every Git worktree' >&2
    exit 1
  fi
  [[ "$fixture_parent" == / ]] && break
  fixture_parent="$(dirname "$fixture_parent")"
done

export GH_RUNLOG="$fixture_root/gh-calls"
export GH_EXPECTED_REPOSITORY='fixture-owner/fixture-panel'
export GH_EXPECTED_CWD="$fixture_cwd"
cat >"$fixture_root/bin/gh" <<'MOCK'
#!/usr/bin/env bash
set -euo pipefail
[[ "${1:-}" == run && "${2:-}" == download && -n "${3:-}" ]] || exit 80
fixture_run_id="$3"
shift 3
fixture_repository="" fixture_name="" fixture_dir=""
while (($#)); do
  [[ $# -ge 2 ]] || exit 81
  case "$1" in
    --repo|-R) fixture_repository="$2" ;;
    --name) fixture_name="$2" ;;
    --dir) fixture_dir="$2" ;;
    *) echo 'mock gh: unexpected argument' >&2; exit 82 ;;
  esac
  shift 2
done
if [[ "$fixture_repository" != "$GH_EXPECTED_REPOSITORY" ]]; then
  echo 'mock gh: explicit repository required outside Git' >&2
  exit 86
fi
[[ "$PWD" == "$GH_EXPECTED_CWD" && -n "$fixture_name" && -n "$fixture_dir" ]] || exit 87
printf '%s|%s|%s|%s\n' "$fixture_run_id" "$fixture_repository" "$fixture_name" "$fixture_dir" >>"$GH_RUNLOG"
MOCK
chmod +x "$fixture_root/bin/gh"
export PATH="$fixture_root/bin:$PATH"
[[ "$(command -v gh)" == "$fixture_root/bin/gh" ]] || exit 1

download_commands=()
while IFS= read -r line; do
  line="${line%$'\r'}"
  if [[ "$line" =~ ^[[:space:]]*(gh[[:space:]]+run[[:space:]]+download[[:space:]].*)$ ]]; then
    download_commands+=("${BASH_REMATCH[1]}")
  fi
done <"$workflow"
if [[ "${#download_commands[@]}" -ne 2 ]]; then
  echo 'release workflow download: both acceptance and candidate commands are required' >&2
  exit 1
fi
SOURCE_RUN_ID=101
CANDIDATE_RUN_ID=202
GITHUB_REPOSITORY="$GH_EXPECTED_REPOSITORY"
names=(release-acceptance-fixture)
artifact_names=(release-candidate-fixture)
acceptance_dir="$fixture_root/acceptance"
proof_dir="$fixture_root/candidate"
printf -v fixture_backtick '\x60'
for workflow_command in "${download_commands[@]}"; do
  # Execute only a single download command through the mock, never shell chains.
  case "$workflow_command" in
    *';'*|*'|'*|*'&'*|*'<'*|*'>'*|*"\$("*|*"$fixture_backtick"*)
      echo 'release workflow download: unexpected shell operator' >&2
      exit 1
      ;;
  esac
  (cd "$fixture_cwd" && eval "$workflow_command")
done
expected_calls="$(printf '%s|%s|%s|%s\n' \
  "$SOURCE_RUN_ID" "$GITHUB_REPOSITORY" "${names[0]}" "$acceptance_dir" \
  "$CANDIDATE_RUN_ID" "$GITHUB_REPOSITORY" "${artifact_names[0]}" "$proof_dir")"
if [[ "$(cat "$GH_RUNLOG")" != "$expected_calls" ]]; then
  echo 'release workflow download: actual workflow arguments differ from repository/run/artifact contract' >&2
  exit 1
fi
for fixture_repo_mode in missing wrong; do
  fixture_repo_args=()
  if [[ "$fixture_repo_mode" == wrong ]]; then
    fixture_repo_args=(--repo wrong-owner/wrong-repository)
  fi
  if (cd "$fixture_cwd" && gh run download "$SOURCE_RUN_ID" "${fixture_repo_args[@]}" \
      --name "${names[0]}" --dir "$acceptance_dir") >"$fixture_root/negative.log" 2>&1; then
    echo 'release workflow download: mock accepted missing or wrong repository' >&2
    exit 1
  else
    fixture_negative_status=$?
  fi
  [[ "$fixture_negative_status" -eq 86 ]] || exit 1
done
echo 'release workflow download: both real commands passed outside Git using only mock gh'
echo 'release workflow download: missing and wrong repository controls rejected'
