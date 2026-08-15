#!/usr/bin/env bash
set -Eeuo pipefail

usage() {
  echo "Usage: $0 --version <x.y.z> --previous-version <x.y.z> [--commit <sha>] [--build-date <UTC>] [--image <local-ref>] [--metadata-output <path>]" >&2
}

version=""
previous_version=""
commit=""
build_date=""
candidate_image=""
metadata_output=""
while (($# > 0)); do
  case "$1" in
    --version)
      version="${2:-}"
      shift 2
      ;;
    --previous-version)
      previous_version="${2:-}"
      shift 2
      ;;
    --commit)
      commit="${2:-}"
      shift 2
      ;;
    --build-date)
      build_date="${2:-}"
      shift 2
      ;;
    --image)
      candidate_image="${2:-}"
      shift 2
      ;;
    --metadata-output)
      metadata_output="${2:-}"
      shift 2
      ;;
    *)
      usage
      exit 2
      ;;
  esac
done

semver_pattern='^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$'
if [[ ! "$version" =~ $semver_pattern || ! "$previous_version" =~ $semver_pattern ]]; then
  usage
  exit 2
fi

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo_root"

for command_name in docker git grep jq sort timeout; do
  if ! command -v "$command_name" >/dev/null 2>&1; then
    echo "release candidate: missing required command: $command_name" >&2
    exit 1
  fi
done
docker info >/dev/null

head_commit="$(git rev-parse HEAD)"
commit="${commit:-$head_commit}"
if [[ "$commit" != "$head_commit" || ! "$commit" =~ ^[0-9a-f]{40}$ ]]; then
  echo "release candidate: --commit must be the full current HEAD SHA" >&2
  exit 1
fi

if [[ "${ANXI_CANDIDATE_ALLOW_DIRTY:-0}" != 1 ]]; then
  if [[ "$(git rev-parse --abbrev-ref HEAD)" != main ]]; then
    echo "release candidate: formal candidates must run on main" >&2
    exit 1
  fi
  if [[ -n "$(git status --porcelain)" ]]; then
    echo "release candidate: formal candidates require a clean worktree" >&2
    exit 1
  fi
  git fetch --no-tags origin main:refs/remotes/origin/main
  if [[ "$(git rev-parse origin/main)" != "$head_commit" ]]; then
    echo "release candidate: main must exactly match origin/main" >&2
    exit 1
  fi
fi

build_date="${build_date:-$(date -u +'%Y-%m-%dT%H:%M:%SZ')}"
if ! date -u -d "$build_date" +'%Y-%m-%dT%H:%M:%SZ' >/dev/null 2>&1; then
  echo "release candidate: --build-date must be an ISO 8601 UTC timestamp" >&2
  exit 1
fi

short_commit="${commit:0:12}"
candidate_image="${candidate_image:-anxi-release-candidate:$version-$short_commit}"
if [[ -z "$metadata_output" ]]; then
  metadata_output="$repo_root/release-candidate.json"
elif [[ "$metadata_output" != /* ]]; then
  metadata_output="$repo_root/$metadata_output"
fi

suffix="$(date +%s)-$$"
owner="anxi-release-candidate-$suffix"
fresh_container="$owner-fresh"
fresh_volume="$owner-data"
dind_container="$owner-dind"
temp_root="$(mktemp -d -t "$owner-XXXXXX")"
candidate_tar="$temp_root/candidate.tar"
fixtures_tar="$temp_root/fixtures.tar"
previous_ref="ghcr.io/anxiyizhi/stardew-server-anxi-panel:$previous_version"

cleanup() {
  local cleanup_status=$?
  set +e
  docker rm -f "$fresh_container" "$dind_container" >/dev/null 2>&1
  if docker volume inspect "$fresh_volume" >/dev/null 2>&1; then
    docker volume rm "$fresh_volume" >/dev/null 2>&1
  fi
  if [[ "$temp_root" == /tmp/anxi-release-candidate-* && -d "$temp_root" ]]; then
    rm -rf -- "$temp_root"
  fi
  exit "$cleanup_status"
}
trap cleanup EXIT

echo "release candidate: building $candidate_image"
docker build --provenance=false --sbom=false --file Dockerfile --tag "$candidate_image" --build-arg "VERSION=$version" --build-arg "COMMIT=$commit" --build-arg "BUILD_DATE=$build_date" .

image_json="$(docker image inspect "$candidate_image")"
image_id="$(jq -r '.[0].Id' <<<"$image_json")"
label_version="$(jq -r '.[0].Config.Labels["org.opencontainers.image.version"] // empty' <<<"$image_json")"
label_revision="$(jq -r '.[0].Config.Labels["org.opencontainers.image.revision"] // empty' <<<"$image_json")"
label_created="$(jq -r '.[0].Config.Labels["org.opencontainers.image.created"] // empty' <<<"$image_json")"
if [[ "$label_version" != "$version" || "$label_revision" != "$commit" || "$label_created" != "$build_date" ]]; then
  echo "release candidate: OCI metadata does not match the frozen candidate identity" >&2
  exit 1
fi

echo "release candidate: fresh install and restart smoke"
docker volume create --label "com.anxi-panel.test-owner=$owner" "$fresh_volume" >/dev/null
docker run -d --name "$fresh_container" --label "com.anxi-panel.test-owner=$owner" --mount "type=volume,src=$fresh_volume,dst=/data" "$candidate_image" >/dev/null

wait_fresh() {
  local deadline=$((SECONDS + 120))
  while ((SECONDS < deadline)); do
    if docker exec "$fresh_container" wget -qO- http://127.0.0.1:8090/health >"$temp_root/health.json" 2>/dev/null; then
      if [[ "$(jq -r '.status // empty' "$temp_root/health.json")" == ok ]]; then
        docker exec "$fresh_container" wget -qO- http://127.0.0.1:8090/api/version >"$temp_root/version.json"
        if [[ "$(jq -r '.version // empty' "$temp_root/version.json")" == "$version" && "$(jq -r '.commit // empty' "$temp_root/version.json")" == "$commit" ]]; then
          return 0
        fi
      fi
    fi
    sleep 1
  done
  docker logs "$fresh_container" 2>&1 | tail -n 80 >&2 || true
  return 1
}

assert_frontend_contract_from_container() {
  local container="$1"
  local output_dir="$2"
  local entry_asset=""
  local prefix=""
  local asset=""
  local control_asset=""
  local mobile_control_asset=""
  local saves_asset=""
  local -a matches=()

  mkdir -p "$output_dir"
  docker exec "$container" wget -qO- http://127.0.0.1:8090/ >"$output_dir/index.html"
  mapfile -t matches < <(grep -oE '/assets/index-[A-Za-z0-9_-]+\.js' "$output_dir/index.html" | sort -u)
  if [[ "${#matches[@]}" -ne 1 ]]; then
    echo "release candidate: expected exactly one frontend entry asset" >&2
    exit 1
  fi
  entry_asset="${matches[0]}"
  docker exec "$container" wget -qO- "http://127.0.0.1:8090$entry_asset" >"$output_dir/entry.js"

  for prefix in ServerControlPage MobileControlPage SavesPage; do
    mapfile -t matches < <(grep -oE "(^|/)$prefix-[A-Za-z0-9_-]+\.js" "$output_dir/entry.js" | sort -u)
    if [[ "${#matches[@]}" -ne 1 ]]; then
      echo "release candidate: expected exactly one $prefix frontend chunk" >&2
      exit 1
    fi
    asset="/assets/${matches[0]#/}"
    docker exec "$container" wget -qO- "http://127.0.0.1:8090$asset" >"$output_dir/$prefix.js"
    case "$prefix" in
      ServerControlPage) control_asset="$output_dir/$prefix.js" ;;
      MobileControlPage) mobile_control_asset="$output_dir/$prefix.js" ;;
      SavesPage) saves_asset="$output_dir/$prefix.js" ;;
    esac
  done

  for asset in "$control_asset" "$mobile_control_asset"; do
    if ! grep -Eq 'value:.FarmhouseStack.,hidden:!0,children:.FarmhouseStack（兼容已有配置）.' "$asset"; then
      echo "release candidate: production frontend exposes FarmhouseStack or lost legacy-value compatibility" >&2
      exit 1
    fi
  done
  if ! grep -Eq 'kind===.auto.\?.游戏日回档.' "$saves_asset" ||
    ! grep -Eq 'farmerName\?.农民：' "$saves_asset" ||
    ! grep -Eq 'farmType\?.地图：' "$saves_asset"; then
    echo "release candidate: production frontend lost game-day rollback hover details" >&2
    exit 1
  fi
}

wait_fresh
if [[ "$(docker exec "$fresh_container" wget -qO- http://127.0.0.1:8090/api/setup/status | jq -r '.initialized')" != false ]]; then
  echo "release candidate: fresh setup state is not uninitialized" >&2
  exit 1
fi
assert_frontend_contract_from_container "$fresh_container" "$temp_root/fresh-frontend"
docker restart "$fresh_container" >/dev/null
wait_fresh
docker rm -f "$fresh_container" >/dev/null
docker volume rm "$fresh_volume" >/dev/null

echo "release candidate: exporting exact image for isolated Web-upgrade E2E"
docker save -o "$candidate_tar" "$candidate_image"
for fixture_image in "$previous_ref" registry:2 nginx:alpine alpine:3.20; do
  echo "release candidate: pre-fetching $fixture_image"
  timeout --foreground 300 docker pull "$fixture_image"
done
docker save -o "$fixtures_tar" "$previous_ref" registry:2 nginx:alpine alpine:3.20
docker run -d --privileged --name "$dind_container" --label "com.anxi-panel.test-owner=$owner" --env DOCKER_TLS_CERTDIR= --volume "$repo_root:/workspace:ro" --volume "$temp_root:/candidate:ro" docker:29-dind >/dev/null

dind_ready=0
for _ in $(seq 1 90); do
  if docker exec "$dind_container" docker info >/dev/null 2>&1; then
    dind_ready=1
    break
  fi
  sleep 1
done
if ((dind_ready != 1)); then
  docker logs "$dind_container" 2>&1 | tail -n 120 >&2 || true
  echo "release candidate: isolated Docker daemon did not become ready" >&2
  exit 1
fi

docker exec "$dind_container" apk add --no-cache bash curl jq openssl sqlite docker-cli-compose zip >/dev/null
docker exec "$dind_container" bash /workspace/scripts/tests/test_release_candidate_upgrade.sh --candidate-tar /candidate/candidate.tar --fixtures-tar /candidate/fixtures.tar --candidate-image "$candidate_image" --version "$version" --previous-version "$previous_version"
docker rm -f "$dind_container" >/dev/null

mkdir -p "$(dirname "$metadata_output")"
jq -n --arg version "$version" --arg previousVersion "$previous_version" --arg commit "$commit" --arg buildDate "$build_date" --arg image "$candidate_image" --arg imageId "$image_id" '{schemaVersion:1,version:$version,previousVersion:$previousVersion,commit:$commit,buildDate:$buildDate,localImage:$image,imageId:$imageId}' >"$metadata_output"

echo "release candidate: all candidate gates passed"
echo "release candidate: metadata written to $metadata_output"
