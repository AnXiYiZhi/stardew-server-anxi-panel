#!/usr/bin/env bash
set -Eeuo pipefail

usage() {
  echo "Usage: $0 --candidate-tar <path> --fixtures-tar <path> --candidate-image <ref> --version <x.y.z> --previous-version <x.y.z>" >&2
}

candidate_tar=""
fixtures_tar=""
candidate_image=""
version=""
previous_version=""
while (($# > 0)); do
  case "$1" in
    --candidate-tar)
      candidate_tar="${2:-}"
      shift 2
      ;;
    --candidate-image)
      candidate_image="${2:-}"
      shift 2
      ;;
    --fixtures-tar)
      fixtures_tar="${2:-}"
      shift 2
      ;;
    --version)
      version="${2:-}"
      shift 2
      ;;
    --previous-version)
      previous_version="${2:-}"
      shift 2
      ;;
    *)
      usage
      exit 2
      ;;
  esac
done

semver_pattern='^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$'
if [[ -z "$candidate_tar" || -z "$fixtures_tar" || -z "$candidate_image" || ! "$version" =~ $semver_pattern || ! "$previous_version" =~ $semver_pattern ]]; then
  usage
  exit 2
fi
if [[ ! -f "$candidate_tar" ]]; then
  echo "candidate upgrade E2E: candidate tar not found: $candidate_tar" >&2
  exit 1
fi
if [[ ! -f "$fixtures_tar" ]]; then
  echo "candidate upgrade E2E: fixture tar not found: $fixtures_tar" >&2
  exit 1
fi

for command_name in docker curl grep jq openssl sha256sum sort sqlite3 zip; do
  if ! command -v "$command_name" >/dev/null 2>&1; then
    echo "candidate upgrade E2E: missing required command: $command_name" >&2
    exit 1
  fi
done
docker info >/dev/null
if [[ "${ANXI_RELEASE_CANDIDATE_ISOLATED_DOCKER:-}" != 1 ]]; then
  echo "candidate upgrade E2E: refusing to run outside the task-owned isolated Docker daemon" >&2
  exit 1
fi
if [[ ! -f /.dockerenv ]] || ! tr '\0' ' ' </proc/1/cmdline | grep -Eq '(^|[ /])(dockerd|dockerd-entrypoint\.sh)( |$)'; then
  echo "candidate upgrade E2E: isolated Docker runtime proof is missing" >&2
  exit 1
fi
if [[ -n "$(docker ps --all --quiet)" || -n "$(docker image ls --quiet)" || -n "$(docker volume ls --quiet)" || -n "$(docker network ls --filter type=custom --quiet)" ]]; then
  echo "candidate upgrade E2E: isolated Docker daemon is not empty before fixture loading" >&2
  exit 1
fi

suffix="$(date +%s)-$$"
owner="anxi-release-candidate-$suffix"
root="/tmp/$owner"
install_dir="$root/deployment"
data_dir="$root/data"
tls_dir="$root/tls"
web_dir="$root/web"
compose_file="$install_dir/docker-compose.yml"
env_file="$install_dir/.env"
cookie_file="$root/admin.cookies"
response_file="$root/response.json"
project="anxirc${suffix//-/}"
panel_container="$project-panel"
game_container="$project-game"
network="$project-network"
registry_container="$project-registry"
release_container="$project-releases"
session_seed_container="$project-steam-session-seed"
unknown_session_holder_container="$project-unknown-steam-session-holder"
target_ref="ghcr.io/anxiyizhi/stardew-server-anxi-panel:$version"
previous_ref="ghcr.io/anxiyizhi/stardew-server-anxi-panel:$previous_version"
previous_fixture_ref="$project/previous-fixture:$previous_version"
steam_session_volume="stardew_steam-session"
candidate_runtime_manifest="/workspace/backend/internal/games/stardew_junimo/config/runtime_stack_manifest.json"
legacy_expected_migrated_compose="$root/legacy-disabled-migrated-compose.yml"
legacy_auth_image_snapshot_before="$root/legacy-disabled-auth-images.before"
legacy_server_container_id=""
legacy_server_started_at=""
legacy_dependency_container_id=""
legacy_auth_container_id=""
legacy_auth_image_ref=""
legacy_auth_image_id=""
legacy_disabled_session_hash=""
unknown_session_holder_id=""
authorized_session_hash=""
authorized_auth_container_id=""
panel_port=18080
compose_ready=0

remove_owned_session_volume() {
  local labels=""

  if ! docker volume inspect "$steam_session_volume" >/dev/null 2>&1; then
    return 0
  fi
  labels="$(docker volume inspect "$steam_session_volume" | jq -c '.[0].Labels // {}')"
  if [[ "$(jq -r --arg owner "$owner" '."com.anxi-panel.test-owner" == $owner' <<<"$labels")" != true ]]; then
    echo "candidate upgrade E2E: refusing to remove unowned Steam session volume $steam_session_volume" >&2
    return 1
  fi
  docker volume rm "$steam_session_volume" >/dev/null
}

cleanup() {
  local cleanup_status=$?
  local cleanup_failed=0
  local remaining=""
  set +e
  if [[ -f "$data_dir/instances/stardew/docker-compose.yml" ]]; then
    docker compose --project-name stardew --project-directory "$data_dir/instances/stardew" -f "$data_dir/instances/stardew/docker-compose.yml" down --volumes --remove-orphans >/dev/null 2>&1
  fi
  if ((compose_ready == 1)); then
    docker compose --project-name "$project" --env-file "$env_file" -f "$compose_file" down --volumes --remove-orphans >/dev/null 2>&1
  fi
  docker rm -f "$unknown_session_holder_container" "$session_seed_container" "$release_container" "$registry_container" >/dev/null 2>&1
  remove_owned_session_volume >/dev/null 2>&1 || cleanup_failed=1
  docker network rm "$network" >/dev/null 2>&1
  if [[ "$root" == /tmp/anxi-release-candidate-* && -d "$root" ]]; then
    rm -rf -- "$root" || cleanup_failed=1
  fi
  remaining="$(docker ps --all --quiet --filter "label=com.anxi-panel.test-owner=$owner")$(docker ps --all --quiet --filter "label=com.docker.compose.project=$project")$(docker ps --all --quiet --filter 'label=com.docker.compose.project=stardew')"
  remaining+="$(docker volume ls --quiet --filter "label=com.anxi-panel.test-owner=$owner")$(docker volume ls --quiet --filter "label=com.docker.compose.project=$project")$(docker volume ls --quiet --filter 'label=com.docker.compose.project=stardew')"
  remaining+="$(docker network ls --quiet --filter "label=com.anxi-panel.test-owner=$owner")$(docker network ls --quiet --filter "label=com.docker.compose.project=$project")$(docker network ls --quiet --filter 'label=com.docker.compose.project=stardew')"
  if [[ -n "$remaining" ]] || docker network inspect "$network" >/dev/null 2>&1; then
    echo "candidate upgrade E2E: task-owned Docker resources remained after cleanup" >&2
    cleanup_failed=1
  fi
  if ((cleanup_status == 0 && cleanup_failed == 1)); then
    cleanup_status=1
  fi
  exit "$cleanup_status"
}
trap cleanup EXIT

mkdir -p "$install_dir" "$data_dir" "$tls_dir" "$web_dir"
chmod 700 "$root" "$install_dir" "$data_dir" "$tls_dir"

echo "candidate upgrade E2E: loading exact candidate and pre-fetched fixtures"
docker load -i "$candidate_tar" >/dev/null
docker load -i "$fixtures_tar" >/dev/null
docker image inspect "$candidate_image" >/dev/null
docker image inspect "$previous_ref" registry:2 nginx:alpine alpine:3.20 >/dev/null
docker tag "$previous_ref" "$previous_fixture_ref"

openssl req -x509 -newkey rsa:2048 -nodes -days 2 -sha256 -subj "/CN=Anxi Release Candidate Test CA" -keyout "$tls_dir/ca.key" -out "$tls_dir/ca.crt" >/dev/null 2>&1

make_server_certificate() {
  local hostname="$1"
  local prefix="$2"
  openssl req -newkey rsa:2048 -nodes -sha256 -subj "/CN=$hostname" -keyout "$tls_dir/$prefix.key" -out "$tls_dir/$prefix.csr" >/dev/null 2>&1
  printf 'subjectAltName=DNS:%s\nextendedKeyUsage=serverAuth\n' "$hostname" >"$tls_dir/$prefix.ext"
  openssl x509 -req -days 2 -sha256 -in "$tls_dir/$prefix.csr" -CA "$tls_dir/ca.crt" -CAkey "$tls_dir/ca.key" -CAcreateserial -extfile "$tls_dir/$prefix.ext" -out "$tls_dir/$prefix.crt" >/dev/null 2>&1
}

make_server_certificate ghcr.io registry
make_server_certificate api.github.com releases
make_server_certificate smapi.io smapi

mkdir -p /etc/docker/certs.d/ghcr.io
cp "$tls_dir/ca.crt" /etc/docker/certs.d/ghcr.io/ca.crt

docker run -d --name "$registry_container" --label "com.anxi-panel.test-owner=$owner" --publish 127.0.0.1:443:443 --volume "$tls_dir:/certs:ro" --env REGISTRY_HTTP_ADDR=0.0.0.0:443 --env REGISTRY_HTTP_TLS_CERTIFICATE=/certs/registry.crt --env REGISTRY_HTTP_TLS_KEY=/certs/registry.key registry:2 >/dev/null

printf '\n127.0.0.1 ghcr.io\n' >>/etc/hosts
for _ in $(seq 1 30); do
  if curl --silent --show-error --fail --cacert "$tls_dir/ca.crt" --resolve ghcr.io:443:127.0.0.1 https://ghcr.io/v2/ >/dev/null; then
    break
  fi
  sleep 1
done
curl --silent --show-error --fail --cacert "$tls_dir/ca.crt" --resolve ghcr.io:443:127.0.0.1 https://ghcr.io/v2/ >/dev/null

docker network create --label "com.anxi-panel.test-owner=$owner" --subnet 172.31.250.0/24 "$network" >/dev/null

cat >"$web_dir/nginx.conf" <<EOF
events {}
http {
  server {
    listen 443 ssl;
    server_name api.github.com;
    ssl_certificate /certs/releases.crt;
    ssl_certificate_key /certs/releases.key;
    default_type application/json;
    location = /repos/anxiyizhi/stardew-server-anxi-panel/releases/latest {
      return 200 '{"tag_name":"v$version","html_url":"https://example.invalid/releases/v$version","draft":false,"prerelease":false,"published_at":"2026-01-01T00:00:00Z"}';
    }
    location = /repos/anxiyizhi/stardew-server-anxi-panel/releases {
      return 200 '[{"tag_name":"v$version","html_url":"https://example.invalid/releases/v$version","draft":false,"prerelease":false,"published_at":"2026-01-01T00:00:00Z"}]';
    }
    location / { return 404; }
  }
  server {
    listen 443 ssl;
    server_name smapi.io;
    ssl_certificate /certs/smapi.crt;
    ssl_certificate_key /certs/smapi.key;
    default_type application/json;
    location = /api/v4.0.0/mods {
      limit_except POST { deny all; }
      return 200 '[{"id":"Pathoschild.ContentPatcher","suggestedUpdate":{"version":"2.9.1","url":"https://www.nexusmods.com/stardewvalley/mods/1915"},"errors":[]}]';
    }
  }
}
EOF

docker run -d --name "$release_container" --label "com.anxi-panel.test-owner=$owner" --network "$network" --ip 172.31.250.10 --volume "$tls_dir:/certs:ro" --volume "$web_dir/nginx.conf:/etc/nginx/nginx.conf:ro" nginx:alpine >/dev/null

docker run --rm --entrypoint cat "$previous_ref" /etc/ssl/certs/ca-certificates.crt >"$tls_dir/ca-bundle.crt"
cat "$tls_dir/ca.crt" >>"$tls_dir/ca-bundle.crt"

create_unhealthy_candidate() {
  local temp_container="$project-unhealthy-source"
  local unhealthy_image="$project/unhealthy:$version"
  docker rm -f "$temp_container" >/dev/null 2>&1 || true
  docker create --name "$temp_container" "$candidate_image" >/dev/null
  docker commit --change 'HEALTHCHECK --interval=1s --timeout=1s --start-period=0s --retries=2 CMD /bin/false' "$temp_container" "$unhealthy_image" >/dev/null
  docker rm "$temp_container" >/dev/null
  docker tag "$unhealthy_image" "$target_ref"
  docker push "$target_ref" >/dev/null
}

push_healthy_candidate() {
  docker tag "$candidate_image" "$target_ref"
  docker push "$target_ref" >/dev/null
}

echo "candidate upgrade E2E: publishing controlled unhealthy target"
create_unhealthy_candidate

cat >"$env_file" <<EOF
PANEL_IMAGE=$previous_ref
PANEL_PORT=$panel_port
PANEL_HOST_INSTALL_DIR=$install_dir
PANEL_HOST_COMPOSE_FILE=$compose_file
PANEL_HOST_DATA_DIR=$data_dir
PANEL_COMPOSE_PROJECT=$project
PANEL_SECRET=release-candidate-secret-$suffix
EOF
chmod 600 "$env_file"

# shellcheck disable=SC2016 # Compose must receive this literal placeholder.
compose_image_literal='${PANEL_IMAGE}'
cat >"$compose_file" <<EOF
services:
  panel:
    image: $compose_image_literal
    container_name: $panel_container
    restart: unless-stopped
    ports:
      - "127.0.0.1:$panel_port:8090"
    extra_hosts:
      - "api.github.com:172.31.250.10"
      - "smapi.io:172.31.250.10"
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock
      - $data_dir:/data
      - $tls_dir/ca-bundle.crt:/etc/ssl/certs/ca-certificates.crt:ro
    environment:
      PANEL_ADDR: ":8090"
      PANEL_DATA_DIR: "/data"
      PANEL_HOST_DATA_DIR: "$data_dir"
      PANEL_HOST_INSTALL_DIR: "$install_dir"
      PANEL_HOST_COMPOSE_FILE: "$compose_file"
      PANEL_COMPOSE_PROJECT: "$project"
      PANEL_SECRET: "release-candidate-secret-$suffix"
      PANEL_VERSION: "$previous_version"
  game:
    image: alpine:3.20
    container_name: $game_container
    command: ["sleep", "3600"]
    volumes:
      - game-data:/game
networks:
  default:
    external: true
    name: $network
volumes:
  game-data:
EOF
chmod 600 "$compose_file"
compose_ready=1

docker compose --project-name "$project" --env-file "$env_file" -f "$compose_file" up -d >/dev/null

wait_version() {
  local expected="$1"
  local timeout_seconds="$2"
  local deadline=$((SECONDS + timeout_seconds))
  while ((SECONDS < deadline)); do
    if curl --silent --show-error --max-time 2 "http://127.0.0.1:$panel_port/api/version" >"$response_file" 2>/dev/null; then
      if [[ "$(jq -r '.version // empty' "$response_file")" == "$expected" ]]; then
        return 0
      fi
    fi
    sleep 1
  done
  echo "candidate upgrade E2E: timed out waiting for Panel version $expected" >&2
  docker logs "$panel_container" 2>&1 | tail -n 80 >&2 || true
  return 1
}

assert_upgraded_frontend_contract() {
  local output_dir="$root/upgraded-frontend"
  local entry_asset=""
  local prefix=""
  local asset=""
  local runtime_settings_asset=""
  local saves_asset=""
  local jobs_asset=""
  local players_asset=""
  local mods_asset=""
  local -a matches=()

  mkdir -p "$output_dir"
  curl --silent --show-error --fail "http://127.0.0.1:$panel_port/" >"$output_dir/index.html"
  mapfile -t matches < <(grep -oE '/assets/index-[A-Za-z0-9_-]+\.js' "$output_dir/index.html" | sort -u)
  if [[ "${#matches[@]}" -ne 1 ]]; then
    echo "candidate upgrade E2E: expected exactly one frontend entry asset" >&2
    exit 1
  fi
  entry_asset="${matches[0]}"
  curl --silent --show-error --fail "http://127.0.0.1:$panel_port$entry_asset" >"$output_dir/entry.js"

  for prefix in ServerControlPage MobileControlPage ServerRuntimeSettingsDialog SavesPage JobsLogsPage PlayersPage ModsPage; do
    mapfile -t matches < <(grep -oE "(^|/)$prefix-[A-Za-z0-9_-]+\.js" "$output_dir/entry.js" | sort -u)
    if [[ "${#matches[@]}" -ne 1 ]]; then
      echo "candidate upgrade E2E: expected exactly one $prefix frontend chunk" >&2
      exit 1
    fi
    asset="/assets/${matches[0]#/}"
    curl --silent --show-error --fail "http://127.0.0.1:$panel_port$asset" >"$output_dir/$prefix.js"
    case "$prefix" in
      ServerRuntimeSettingsDialog) runtime_settings_asset="$output_dir/$prefix.js" ;;
      SavesPage) saves_asset="$output_dir/$prefix.js" ;;
      JobsLogsPage) jobs_asset="$output_dir/$prefix.js" ;;
      PlayersPage) players_asset="$output_dir/$prefix.js" ;;
      ModsPage) mods_asset="$output_dir/$prefix.js" ;;
    esac
  done

  if ! grep -Eq 'value:.FarmhouseStack.,hidden:!0,children:.FarmhouseStack（兼容已有配置）.' "$runtime_settings_asset"; then
    echo "candidate upgrade E2E: upgraded frontend exposes FarmhouseStack or lost legacy-value compatibility" >&2
    exit 1
  fi
  if ! grep -Eq 'kind===.auto.\?.游戏日回档.' "$saves_asset" ||
    ! grep -Eq 'farmerName\?.农民：' "$saves_asset" ||
    ! grep -Eq 'farmType\?.地图：' "$saves_asset"; then
    echo "candidate upgrade E2E: upgraded frontend lost game-day rollback hover details" >&2
    exit 1
  fi
  if ! grep -Fq '最近控制命令分页' "$jobs_asset" || ! grep -Fq '玩家活动分页' "$players_asset"; then
    echo "candidate upgrade E2E: upgraded frontend lost bounded pagination contracts" >&2
    exit 1
  fi
  if ! grep -Fq '只看可更新' "$mods_asset" || ! grep -Fq '重新检查' "$mods_asset" ||
    ! grep -Fq '配置模组筛选' "$mods_asset" || ! grep -Fq '发现新版本' "$mods_asset"; then
    echo "candidate upgrade E2E: upgraded frontend lost Mod update reminder or configuration-card contracts" >&2
    exit 1
  fi
  if ! grep -FRq 'data-modal-initial-focus' "$output_dir" || ! grep -FRq 'aria-modal' "$output_dir"; then
    echo "candidate upgrade E2E: upgraded frontend lost shared modal accessibility contract" >&2
    exit 1
  fi
}

assert_upgraded_mod_update_check() {
  local instance_dir="$data_dir/instances/stardew"
  local mod_dir="$instance_dir/.local-container/mods/ContentPatcher"
  local cache_file="$instance_dir/.local-container/control/mod-updates.json"
  local code=""

  echo "candidate upgrade E2E: testing upgraded Mod update API against controlled SMAPI service"
  mkdir -p "$mod_dir"
  rm -f -- "$cache_file"
  printf '%s\n' '{"Name":"Content Patcher","UniqueID":"Pathoschild.ContentPatcher","Version":"2.0.0","Author":"Pathoschild","UpdateKeys":["Nexus:1915"]}' >"$mod_dir/manifest.json"

  code="$(curl --silent --show-error --output "$response_file" --write-out '%{http_code}' "http://127.0.0.1:$panel_port/api/instances/stardew/mod-updates")"
  if [[ "$code" != 401 ]]; then
    echo "candidate upgrade E2E: anonymous Mod update GET returned HTTP $code, want 401" >&2
    exit 1
  fi
  code="$(curl --silent --show-error --output "$response_file" --write-out '%{http_code}' --cookie "$cookie_file" --request POST "http://127.0.0.1:$panel_port/api/instances/stardew/mod-updates/check")"
  if [[ "$code" != 200 || "$(jq -r '.status // empty' "$response_file")" != ok ||
    "$(jq -r '.eligibleCount // 0' "$response_file")" != 1 ||
    "$(jq -r '.updates | length' "$response_file")" != 1 ||
    "$(jq -r '.updates[0].uniqueId // empty' "$response_file")" != Pathoschild.ContentPatcher ||
    "$(jq -r '.updates[0].currentVersion // empty' "$response_file")" != 2.0.0 ||
    "$(jq -r '.updates[0].latestVersion // empty' "$response_file")" != 2.9.1 ||
    "$(jq -r '.updates[0].url // empty' "$response_file")" != https://www.nexusmods.com/stardewvalley/mods/1915 ||
    "$(jq -r 'if has("cached") then .cached else true end' "$response_file")" != false ]]; then
    echo "candidate upgrade E2E: upgraded Mod update check contract failed with HTTP $code" >&2
    cat "$response_file" >&2
    exit 1
  fi
  if [[ ! -s "$cache_file" ]]; then
    echo "candidate upgrade E2E: upgraded Mod update check did not persist its instance cache" >&2
    exit 1
  fi

  code="$(curl --silent --show-error --output "$response_file" --write-out '%{http_code}' --cookie "$cookie_file" "http://127.0.0.1:$panel_port/api/instances/stardew/mod-updates")"
  if [[ "$code" != 200 || "$(jq -r '.cached // false' "$response_file")" != true ||
    "$(jq -r '.updates[0].latestVersion // empty' "$response_file")" != 2.9.1 ]]; then
    echo "candidate upgrade E2E: upgraded Mod update cached GET contract failed with HTTP $code" >&2
    cat "$response_file" >&2
    exit 1
  fi
  echo "candidate upgrade E2E: upgraded Mod update API returned and cached the controlled suggestion"
}

compose_project_is_strictly_stopped() {
  local project="$1"
  local project_container_count=""
  local server_container_count=""
  local running_container_count=""
  local server_container_id=""
  local server_state=""
  local project_network_count=""
  local project_network_name=""

  project_container_count="$(docker ps -a --filter "label=com.docker.compose.project=$project" --format '{{.ID}}' | wc -l | tr -d '[:space:]')" || return 2
  server_container_count="$(docker ps -a --filter "label=com.docker.compose.project=$project" --filter 'label=com.docker.compose.service=server' --format '{{.ID}}' | wc -l | tr -d '[:space:]')" || return 2
  running_container_count="$(docker ps --filter "label=com.docker.compose.project=$project" --format '{{.ID}}' | wc -l | tr -d '[:space:]')" || return 2
  project_network_count="$(docker network ls --filter "label=com.docker.compose.project=$project" --format '{{.Name}}' | wc -l | tr -d '[:space:]')" || return 2

  # Runtime Stop is intentionally scoped and non-destructive: it may leave one
  # exited server container and the default project network in place. It must
  # not leave any running service, materialize steam-auth, or retain an orphan.
  if [[ "$running_container_count" != 0 || "$project_container_count" != "$server_container_count" ]] ||
    ((server_container_count > 1 || project_network_count > 1)); then
    return 1
  fi
  if [[ "$server_container_count" == 1 ]]; then
    server_container_id="$(docker ps -a --filter "label=com.docker.compose.project=$project" --filter 'label=com.docker.compose.service=server' --format '{{.ID}}')" || return 2
    server_state="$(docker inspect "$server_container_id" | jq -r '.[0].State.Status // empty' | tr '[:upper:]' '[:lower:]')" || return 2
    if [[ "$server_state" != exited && "$server_state" != dead ]]; then
      return 1
    fi
  fi
  if [[ "$project_network_count" == 1 ]]; then
    project_network_name="$(docker network ls --filter "label=com.docker.compose.project=$project" --format '{{.Name}}')" || return 2
    if [[ "$project_network_name" != "${project}_default" ]]; then
      return 1
    fi
  fi
  return 0
}

assert_upgraded_stopped_compose_save_import_submission() {
  local instance_dir="$data_dir/instances/stardew"
  local instance_compose="$instance_dir/docker-compose.yml"
  # Product lifecycle operations derive the project from the instance data-dir
  # basename, so this fixture must use the real project instead of a top-level
  # Compose name that Panel intentionally ignores.
  local import_project="stardew"
  local fixture_root="$root/save-import-fixture"
  local save_zip="$root/Imported_123.zip"
  local commit_body="$root/save-import-commit.json"
  local stop_code=""
  local instance_state=""
  local compose_ps_output=""
  local server_container_count=""
  local running_server_count=""
  local compose_server_count=""
  local compose_server_state=""
  local preview_code=""
  local upload_token=""
  local commit_code=""
  local import_job_id=""
  local import_operation_id=""
  local job_status=""
  local stopped_restored=0
  local stopped_probe_status=0
  local existing_save_sha=""
  local existing_info_sha=""
  local active_job_count=""
  local cleared_job_count=""
  local cleared_audit_count=""
  local import_journal=""
  local retry_preview_code=""
  local retry_upload_token=""
  local retry_cancel_code=""
  local preimport_backup_count=""

  echo "candidate upgrade E2E: testing Panel Stop with an exited server before save-import submission"
  # The legacy-repair evidence is complete. Remove only this isolated DinD
  # project before replacing its Compose definition with the save-import
  # runtime; the separately named non-target game fixture remains untouched.
  docker compose --project-name "$import_project" --project-directory "$instance_dir" -f "$instance_compose" down --remove-orphans >/dev/null
  if [[ -n "$(docker ps -a --filter "label=com.docker.compose.project=$import_project" --quiet)" ||
    -n "$(docker network ls --filter "label=com.docker.compose.project=$import_project" --quiet)" ]]; then
    echo "candidate upgrade E2E: legacy stardew fixture resources remained before stopped-server import tests" >&2
    exit 1
  fi
  mkdir -p "$instance_dir/.local-container/mods/JunimoServer"
  mkdir -p "$instance_dir/.local-container/saves/Saves/Existing_1"
  mkdir -p "$instance_dir/.local-container/saves/.smapi/mod-data/junimohost.server"
  printf 'IMAGE_VERSION=1.5.0-preview.125\nAPI_PORT=5110\n' >"$instance_dir/.env"
  printf '%s\n' '{"Name":"JunimoServer","Version":"1.5.0-preview.125","UniqueID":"JunimoHost.Server"}' >"$instance_dir/.local-container/mods/JunimoServer/manifest.json"
  printf 'release-candidate-fixture-dll\n' >"$instance_dir/.local-container/mods/JunimoServer/JunimoServer.dll"
  printf '%s\n' '<SaveGame><player><name>Existing</name></player></SaveGame>' >"$instance_dir/.local-container/saves/Saves/Existing_1/Existing_1"
  printf '%s\n' '<Farmer><name>Existing</name></Farmer>' >"$instance_dir/.local-container/saves/Saves/Existing_1/SaveGameInfo"
  existing_save_sha="$(sha256sum "$instance_dir/.local-container/saves/Saves/Existing_1/Existing_1" | awk '{print $1}')"
  existing_info_sha="$(sha256sum "$instance_dir/.local-container/saves/Saves/Existing_1/SaveGameInfo" | awk '{print $1}')"
  printf '%s\n' '{"SaveNameToLoad":"Existing_1"}' >"$instance_dir/.local-container/saves/.smapi/mod-data/junimohost.server/junimohost.gameloader.json"
  printf 'name: %s\nservices:\n  server:\n    image: alpine:3.20\n    command: ["sh", "-c", "sleep 3600"]\n' "$import_project" >"$instance_compose"

  docker compose --project-directory "$instance_dir" -f "$instance_compose" up -d >/dev/null
  if [[ "$(sqlite3 -cmd '.timeout 5000' "$data_dir/panel.db" "UPDATE instances SET state='running', state_message='release candidate running fixture', driver_phase='running', driver_payload='{}' WHERE id='stardew'; SELECT changes();")" != 1 ]]; then
    echo "candidate upgrade E2E: failed to prepare the default instance state" >&2
    exit 1
  fi

  stop_code="$(curl --silent --show-error --output "$response_file" --write-out '%{http_code}' --cookie "$cookie_file" --request POST "http://127.0.0.1:$panel_port/api/instances/stardew/stop")"
  if [[ "$stop_code" != 202 ]]; then
    echo "candidate upgrade E2E: Panel Stop failed with HTTP $stop_code" >&2
    cat "$response_file" >&2
    exit 1
  fi
  for _ in $(seq 1 60); do
    if curl --silent --show-error --fail --cookie "$cookie_file" "http://127.0.0.1:$panel_port/api/instances/stardew" >"$response_file" 2>/dev/null; then
      instance_state="$(jq -r '.state // empty' "$response_file")"
      server_container_count="$(docker ps -a --filter "label=com.docker.compose.project=$import_project" --filter 'label=com.docker.compose.service=server' --format '{{.ID}}' | wc -l | tr -d '[:space:]')"
      running_server_count="$(docker ps --filter "label=com.docker.compose.project=$import_project" --filter 'label=com.docker.compose.service=server' --format '{{.ID}}' | wc -l | tr -d '[:space:]')"
      if [[ "$instance_state" == stopped ]] &&
        [[ "$server_container_count" == 1 && "$running_server_count" == 0 ]]; then
        break
      fi
    fi
    sleep 1
  done
  if [[ "$instance_state" != stopped ]] ||
    [[ "$server_container_count" != 1 || "$running_server_count" != 0 ]]; then
    echo "candidate upgrade E2E: Panel Stop did not leave exactly one exited server container" >&2
    exit 1
  fi
  compose_ps_output="$(docker exec --workdir /data/instances/stardew "$panel_container" docker compose ps --all --format json)"
  compose_server_count="$(printf '%s' "$compose_ps_output" | jq -r 'if type == "array" then [.[] | select((.Service // "") == "server")] | length elif (.Service // "") == "server" then 1 else 0 end')"
  compose_server_state="$(printf '%s' "$compose_ps_output" | jq -r 'if type == "array" then ([.[] | select((.Service // "") == "server")][0].State // "") elif (.Service // "") == "server" then (.State // "") else "" end' | tr '[:upper:]' '[:lower:]')"
  if [[ "$compose_server_count" != 1 || ("$compose_server_state" != exited && "$compose_server_state" != dead) ]]; then
    echo "candidate upgrade E2E: compose ps after Panel Stop did not prove one exited server" >&2
    printf '%s\n' "$compose_ps_output" >&2
    exit 1
  fi

  # Make the asynchronous maintenance ComposeUp fail immediately after the
  # submission has crossed the stopped-server strict-Compose gate. The source is
  # deliberately absent and Compose is forbidden from creating it, so this
  # exercises deterministic failure cleanup without shortening product health
  # budgets or waiting for a deliberately exited server to time out.
  cat >"$instance_compose" <<EOF
name: $import_project
services:
  server:
    image: alpine:3.20
    command: ["sh", "-c", "sleep 3600"]
    volumes:
      - type: bind
        source: $root/missing-maintenance-bind
        target: /fixture
        bind:
          create_host_path: false
EOF
  mkdir -p "$fixture_root/Imported_123"
  printf '%s\n' '<SaveGame><player><name>Imported</name></player></SaveGame>' >"$fixture_root/Imported_123/Imported_123"
  printf '%s\n' '<Farmer><name>Imported</name></Farmer>' >"$fixture_root/Imported_123/SaveGameInfo"
  (
    cd "$fixture_root"
    zip -q -r "$save_zip" Imported_123
  )

  preview_code="$(curl --silent --show-error --output "$response_file" --write-out '%{http_code}' --cookie "$cookie_file" --form "save=@$save_zip;filename=Imported_123.zip" "http://127.0.0.1:$panel_port/api/instances/stardew/saves/upload-preview")"
  upload_token="$(jq -r '.token // empty' "$response_file")"
  if [[ "$preview_code" != 200 || -z "$upload_token" || "$(jq -r '.saveName // empty' "$response_file")" != Imported_123 ]]; then
    echo "candidate upgrade E2E: save upload preview failed with HTTP $preview_code" >&2
    cat "$response_file" >&2
    exit 1
  fi
  jq -n --arg token "$upload_token" '{token:$token,hostHandling:{mode:"virtual_host_takeover",acknowledged:true}}' >"$commit_body"
  commit_code="$(curl --silent --show-error --output "$response_file" --write-out '%{http_code}' --cookie "$cookie_file" --header 'Content-Type: application/json' --data-binary "@$commit_body" "http://127.0.0.1:$panel_port/api/instances/stardew/saves/upload-commit-and-start")"
  import_job_id="$(jq -r '.jobId // empty' "$response_file")"
  import_operation_id="$(jq -r '.operationId // empty' "$response_file")"
  if [[ "$commit_code" != 202 || -z "$import_job_id" || -z "$import_operation_id" || "$(jq -r '.saveName // empty' "$response_file")" != Imported_123 ]]; then
    echo "candidate upgrade E2E: stopped-server save import was not accepted with HTTP 202/jobId" >&2
    cat "$response_file" >&2
    exit 1
  fi

  for _ in $(seq 1 90); do
    if curl --silent --show-error --fail --cookie "$cookie_file" "http://127.0.0.1:$panel_port/api/jobs/$import_job_id" >"$response_file" 2>/dev/null; then
      job_status="$(jq -r '.job.status // empty' "$response_file")"
      if [[ "$job_status" == failed || "$job_status" == canceled || "$job_status" == succeeded ]]; then
        break
      fi
    fi
    sleep 1
  done
  if [[ "$job_status" != failed ]]; then
    echo "candidate upgrade E2E: controlled maintenance fixture did not fail terminally" >&2
    cat "$response_file" >&2
    exit 1
  fi
  for _ in $(seq 1 60); do
    instance_state="$(curl --silent --show-error --fail --cookie "$cookie_file" "http://127.0.0.1:$panel_port/api/instances/stardew" | jq -r '.state // empty')"
    if [[ "$instance_state" == stopped ]]; then
      if compose_project_is_strictly_stopped "$import_project"; then
        stopped_restored=1
        break
      else
        stopped_probe_status=$?
        if ((stopped_probe_status > 1)); then
          echo "candidate upgrade E2E: Docker probe failed while checking controlled save-import stopped state" >&2
          exit 1
        fi
      fi
    fi
    sleep 1
  done
  if ((stopped_restored != 1)); then
    echo "candidate upgrade E2E: controlled save-import fixture did not restore stopped state with only an optional exited server/default network" >&2
    docker ps -a --filter "label=com.docker.compose.project=$import_project" --format 'container={{.Names}} service={{.Label "com.docker.compose.service"}} state={{.State}}' >&2
    docker network ls --filter "label=com.docker.compose.project=$import_project" --format 'network={{.Name}}' >&2
    exit 1
  fi

  if [[ ! "$import_job_id" =~ ^job_[0-9a-f]{32}$ ]]; then
    echo "candidate upgrade E2E: failed import returned an invalid project job ID" >&2
    exit 1
  fi
  if [[ ! "$import_operation_id" =~ ^[0-9a-f]{32}$ ]]; then
    echo "candidate upgrade E2E: failed import returned an invalid operation ID" >&2
    exit 1
  fi
  import_journal="$instance_dir/.local-container/control/save-import-transactions/$import_operation_id/journal.json"
  if [[ ! -f "$import_journal" ]]; then
    echo "candidate upgrade E2E: failed import did not retain its exact unfinished journal" >&2
    exit 1
  fi
  active_job_count="$(sqlite3 -cmd '.timeout 5000' "$data_dir/panel.db" "SELECT COUNT(*) FROM jobs WHERE status IN ('queued','running');")"
  cleared_job_count="$(sqlite3 -cmd '.timeout 5000' "$data_dir/panel.db" 'SELECT COUNT(*) FROM jobs;')"
  if [[ "$active_job_count" != 0 || ! "$cleared_job_count" =~ ^[1-9][0-9]*$ ]]; then
    echo "candidate upgrade E2E: legacy jobs-clear fixture is not terminal and non-empty" >&2
    exit 1
  fi

  # Reproduce the v0.5.5 task-center bug exactly: terminal rows disappear and
  # a later durable jobs_cleared audit remains, while the owned upload binding
  # and unfinished import journal are untouched. The next ordinary preview on
  # the upgraded Panel must recover only this exact identity before reading ZIP.
  sqlite3 -cmd '.timeout 5000' "$data_dir/panel.db" "PRAGMA foreign_keys=ON; BEGIN IMMEDIATE; DELETE FROM job_logs; DELETE FROM jobs; INSERT INTO audit_logs (actor_user_id, action, target_type, target_id, metadata_json) VALUES (NULL, 'jobs_cleared', 'jobs', 'all', '{\"count\":$cleared_job_count}'); COMMIT;"
  cleared_audit_count="$(sqlite3 -cmd '.timeout 5000' "$data_dir/panel.db" "SELECT COUNT(*) FROM audit_logs WHERE action='jobs_cleared' AND target_type='jobs' AND target_id='all';")"
  if [[ "$(sqlite3 -cmd '.timeout 5000' "$data_dir/panel.db" 'SELECT COUNT(*) FROM jobs;')" != 0 || "$cleared_audit_count" == 0 ]]; then
    echo "candidate upgrade E2E: failed to reproduce the legacy jobs-cleared durable state" >&2
    exit 1
  fi

  retry_preview_code="$(curl --silent --show-error --output "$response_file" --write-out '%{http_code}' --cookie "$cookie_file" --form "save=@$save_zip;filename=Imported_123.zip" "http://127.0.0.1:$panel_port/api/instances/stardew/saves/upload-preview")"
  retry_upload_token="$(jq -r '.token // empty' "$response_file")"
  if [[ "$retry_preview_code" != 200 || -z "$retry_upload_token" || "$(jq -r '.saveName // empty' "$response_file")" != Imported_123 ]]; then
    echo "candidate upgrade E2E: upgraded Panel did not auto-recover the v0.5.5 jobs-cleared import with HTTP $retry_preview_code" >&2
    cat "$response_file" >&2
    exit 1
  fi
  if [[ -e "$import_journal" || -d "$instance_dir/.local-container/saves/Saves/Imported_123" ]]; then
    echo "candidate upgrade E2E: legacy auto-recovery retained the old journal or staged target" >&2
    exit 1
  fi
  if [[ "$(sha256sum "$instance_dir/.local-container/saves/Saves/Existing_1/Existing_1" | awk '{print $1}')" != "$existing_save_sha" ||
    "$(sha256sum "$instance_dir/.local-container/saves/Saves/Existing_1/SaveGameInfo" | awk '{print $1}')" != "$existing_info_sha" ]]; then
    echo "candidate upgrade E2E: legacy auto-recovery changed the selected existing save" >&2
    exit 1
  fi
  preimport_backup_count="$(find "$instance_dir/.local-container/backups/saves" -maxdepth 1 -type f -name 'preimport_Imported_123_*.zip' | wc -l | tr -d '[:space:]')"
  if [[ ! "$preimport_backup_count" =~ ^[1-9][0-9]*$ ]]; then
    echo "candidate upgrade E2E: legacy auto-recovery removed the durable preimport backup" >&2
    exit 1
  fi

  jq -n --arg token "$retry_upload_token" '{token:$token,cancel:true}' >"$commit_body"
  retry_cancel_code="$(curl --silent --show-error --output "$response_file" --write-out '%{http_code}' --cookie "$cookie_file" --header 'Content-Type: application/json' --data-binary "@$commit_body" "http://127.0.0.1:$panel_port/api/instances/stardew/saves/upload-commit-and-start")"
  if [[ "$retry_cancel_code" != 200 || "$(jq -r '.cancelled // false' "$response_file")" != true ]]; then
    echo "candidate upgrade E2E: recovered preview token cleanup failed with HTTP $retry_cancel_code" >&2
    cat "$response_file" >&2
    exit 1
  fi
  echo "candidate upgrade E2E: upgraded Panel recovered the v0.5.5 jobs-cleared import, preserved data, and accepted a new preview"
}

assert_upgraded_save_import_phase_a_boundaries() {
  local instance_dir="$data_dir/instances/stardew"
  local instance_compose="$instance_dir/docker-compose.yml"
  # The production Docker client derives the exec project from the instance
  # data-directory basename, so this fixture must use the real "stardew"
  # project instead of an arbitrary top-level Compose name.
  local import_project="stardew"
  local fixture_root="$root/save-import-phase-a-fixture"
  local archive_root="$fixture_root/archives"
  local hidden_saves="$fixture_root/hidden-saves"
  local runtime_fixture="$fixture_root/runtime"
  local control_source="/workspace/backend/internal/games/stardew_junimo/embedded/smapi-mod"
  local existing_main="$instance_dir/.local-container/saves/Saves/Existing_1/Existing_1"
  local existing_info="$instance_dir/.local-container/saves/Saves/Existing_1/SaveGameInfo"
  local active_pointer="$instance_dir/.local-container/saves/.smapi/mod-data/junimohost.server/junimohost.gameloader.json"
  local pending_root="$instance_dir/.local-container/control/pending-save-uploads"
  local transaction_root="$instance_dir/.local-container/control/save-import-transactions"
  local existing_save_sha=""
  local existing_info_sha=""
  local case_name=""
  local case_zip=""
  local case_token=""
  local case_job=""
  local case_operation=""
  local case_journal=""
  local case_status=""
  local case_code=""
  local case_instance_snapshot=""
  local case_current_snapshot=""
  local select_code=""
  local preimport_count=""
  local raw_platform_id="76561190000000456"

  echo "candidate upgrade E2E: testing exact target visibility and FIFO no-effect recovery"
  # The preceding controlled maintenance failure should already have stopped
  # its runtime. Defensively remove only that isolated DinD project before
  # replacing the instance Compose definition with the Phase A runtime.
  docker compose --project-name "$import_project" --project-directory "$instance_dir" -f "$instance_compose" down --remove-orphans >/dev/null
  if [[ -n "$(docker ps -a --filter "label=com.docker.compose.project=$import_project" --quiet)" ||
    -n "$(docker network ls --filter "label=com.docker.compose.project=$import_project" --quiet)" ]]; then
    echo "candidate upgrade E2E: legacy stardew fixture resources remained before Phase A boundary tests" >&2
    exit 1
  fi
  mkdir -p "$runtime_fixture" "$archive_root" "$hidden_saves"
  mkdir -p "$instance_dir/.local-container/mods/JunimoServer"
  mkdir -p "$instance_dir/.local-container/mods/StardewAnxiPanel.Control"
  mkdir -p "$instance_dir/.local-container/control"
  mkdir -p "$(dirname "$active_pointer")"
  mkdir -p "$(dirname "$existing_main")"
  printf 'IMAGE_VERSION=1.5.0-preview.125\nAPI_PORT=5110\n' >"$instance_dir/.env"
  printf '%s\n' '{"Name":"JunimoServer","Version":"1.5.0-preview.125","UniqueID":"JunimoHost.Server"}' >"$instance_dir/.local-container/mods/JunimoServer/manifest.json"
  printf 'release-candidate-fixture-dll\n' >"$instance_dir/.local-container/mods/JunimoServer/JunimoServer.dll"
  cp "$control_source/manifest.json" "$instance_dir/.local-container/mods/StardewAnxiPanel.Control/manifest.json"
  cp "$control_source/StardewAnxiPanel.Control.dll" "$instance_dir/.local-container/mods/StardewAnxiPanel.Control/StardewAnxiPanel.Control.dll"
  printf '%s\n' '{"controlModVersion":"0.3.7","hostFarmhousePreservationPatchAvailable":true,"hostAutomationBridgeAvailable":true,"hostSleepSafetyPatchAvailable":true}' >"$instance_dir/.local-container/control/options.json"
  printf '%s\n' '<SaveGame><player><name>Existing</name></player><uniqueIDForThisGame>1</uniqueIDForThisGame></SaveGame>' >"$existing_main"
  printf '%s\n' '<Farmer><name>Existing</name></Farmer>' >"$existing_info"
  printf '%s\n' '{"SaveNameToLoad":"Existing_1"}' >"$active_pointer"
  existing_save_sha="$(sha256sum "$existing_main" | awk '{print $1}')"
  existing_info_sha="$(sha256sum "$existing_info" | awk '{print $1}')"

  cat >"$runtime_fixture/fake-http.sh" <<'EOF'
#!/bin/sh
set -eu
if [ -f /tmp/anxi-target-missing ]; then
  body='{"dayTransitionComplete":true,"saveId":"Existing_1","saveImportFinalizeCount":0,"masterName":"Existing","failedFields":[]}'
else
  body='{"playerCount":0,"dayTransitionComplete":true,"saveId":"Existing_1","saveImportFinalizeCount":0,"masterName":"Existing","farmhandData":[],"failedFields":[]}'
fi
length="$(printf '%s' "$body" | wc -c | tr -d '[:space:]')"
printf 'HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: %s\r\nConnection: close\r\n\r\n%s' "$length" "$body"
EOF
  cat >"$runtime_fixture/fake-junimo.sh" <<'EOF'
#!/bin/sh
set -eu
rm -f /tmp/smapi-input /tmp/anxi-target-missing
mkfifo /tmp/smapi-input
: > /tmp/server-output.log
nc -lk -p 8080 -e /fixture/fake-http.sh &
exec 3<> /tmp/smapi-input
while IFS= read -r command <&3; do
  case "$command" in
    "saves info "*)
      save_name="${command#saves info }"
      if [ -f "/data/Saves/$save_name/$save_name" ]; then
        printf 'Save: %s\n  Farm Type: Standard\n' "$save_name" >> /tmp/server-output.log
      else
        printf 'Controlled fixture: exact save target is not visible: %s\n' "$save_name" >> /tmp/server-output.log
        : > /tmp/anxi-target-missing
      fi
      ;;
    "saves import "*)
      printf 'Controlled Junimo diagnostic: command accepted but produced no disk effect: %s\n' "$command" >> /tmp/server-output.log
      ;;
    *)
      printf 'Controlled fixture ignored command: %s\n' "$command" >> /tmp/server-output.log
      ;;
  esac
done
EOF
  chmod 700 "$runtime_fixture/fake-http.sh" "$runtime_fixture/fake-junimo.sh"

  create_phase_a_save_zip() {
    local name="$1"
    local identity="$2"
    local source_root="$archive_root/$name"
    local zip_path="$archive_root/$name.zip"
    mkdir -p "$source_root/$name"
    printf '<SaveGame><player><name>%s</name><farmName>Gate</farmName></player><uniqueIDForThisGame>%s</uniqueIDForThisGame></SaveGame>\n' "$name" "$identity" >"$source_root/$name/$name"
    printf '<Farmer><name>%s</name><farmName>Gate</farmName></Farmer>\n' "$name" >"$source_root/$name/SaveGameInfo"
    (
      cd "$source_root"
      zip -q -r "$zip_path" "$name"
    )
  }

  write_phase_a_compose() {
    local saves_source="$1"
    cat >"$instance_compose" <<EOF
services:
  server:
    image: $candidate_image
    entrypoint: ["sh", "/fixture/fake-junimo.sh"]
    volumes:
      - type: bind
        source: $runtime_fixture
        target: /fixture
        read_only: true
      - type: bind
        source: $instance_dir/.local-container/mods
        target: /data/Mods
        read_only: true
      - type: bind
        source: $saves_source
        target: /data/Saves
        read_only: true
EOF
  }

  read_phase_a_instance_snapshot() {
    sqlite3 -cmd '.timeout 5000' "$data_dir/panel.db" "SELECT hex(CAST(state AS BLOB)) || '|' || CASE WHEN state_message IS NULL THEN 'NULL' ELSE hex(CAST(state_message AS BLOB)) END || '|' || CASE WHEN driver_phase IS NULL THEN 'NULL' ELSE hex(CAST(driver_phase AS BLOB)) END || '|' || CASE WHEN driver_payload IS NULL THEN 'NULL' ELSE hex(CAST(driver_payload AS BLOB)) END FROM instances WHERE id='stardew';"
  }

  submit_phase_a_case() {
    local name="$1"
    local zip_path="$2"
    local mode="$3"
    local preview_code=""
    preview_code="$(curl --silent --show-error --output "$response_file" --write-out '%{http_code}' --cookie "$cookie_file" --form "save=@$zip_path;filename=$name.zip" "http://127.0.0.1:$panel_port/api/instances/stardew/saves/upload-preview")"
    case_token="$(jq -r '.token // empty' "$response_file")"
    if [[ "$preview_code" != 200 || -z "$case_token" || "$(jq -r '.saveName // empty' "$response_file")" != "$name" ]]; then
      echo "candidate upgrade E2E: $mode preview failed with HTTP $preview_code" >&2
      cat "$response_file" >&2
      exit 1
    fi
    if [[ "$mode" == invisible ]]; then
      jq -n --arg token "$case_token" '{token:$token,hostHandling:{mode:"virtual_host_takeover",acknowledged:true}}' >"$root/save-import-phase-a-commit.json"
    else
      jq -n --arg token "$case_token" --arg platform "$raw_platform_id" '{token:$token,hostHandling:{mode:"swap_to_player",platformId:$platform}}' >"$root/save-import-phase-a-commit.json"
    fi
    case_instance_snapshot="$(read_phase_a_instance_snapshot)"
    if [[ -z "$case_instance_snapshot" ]]; then
      echo "candidate upgrade E2E: $mode could not capture the exact pre-maintenance instance snapshot" >&2
      exit 1
    fi
    case_code="$(curl --silent --show-error --output "$response_file" --write-out '%{http_code}' --cookie "$cookie_file" --header 'Content-Type: application/json' --data-binary "@$root/save-import-phase-a-commit.json" "http://127.0.0.1:$panel_port/api/instances/stardew/saves/upload-commit-and-start")"
    case_job="$(jq -r '.jobId // empty' "$response_file")"
    case_operation="$(jq -r '.operationId // empty' "$response_file")"
    if [[ "$case_code" != 202 || ! "$case_job" =~ ^job_[0-9a-f]{32}$ || ! "$case_operation" =~ ^[0-9a-f]{32}$ ]]; then
      echo "candidate upgrade E2E: $mode submission did not return exact accepted identities with HTTP $case_code" >&2
      cat "$response_file" >&2
      exit 1
    fi
  }

  wait_phase_a_case_failed() {
    local mode="$1"
    # Bound the full legal path, not just the expected fast fixture path:
    # Compose Up 120s + readiness 300s + pre-submit/API/log probes and FIFO
    # up to about 90s + observation/log capture 35s + scoped stop/final proof
    # up to 180s, with scheduler and local staging margin.
    local terminal_wait_seconds=780
    local stopped_probe_status=0
    case_status=""
    for _ in $(seq 1 "$terminal_wait_seconds"); do
      if curl --silent --show-error --fail --cookie "$cookie_file" "http://127.0.0.1:$panel_port/api/jobs/$case_job" >"$response_file" 2>/dev/null; then
        case_status="$(jq -r '.job.status // empty' "$response_file")"
        if [[ "$case_status" == failed || "$case_status" == canceled || "$case_status" == succeeded ]]; then
          break
        fi
      fi
      sleep 1
    done
    if [[ "$case_status" != failed ]]; then
      echo "candidate upgrade E2E: $mode case did not fail terminally" >&2
      cat "$response_file" >&2
      exit 1
    fi
    case_journal="$transaction_root/$case_operation/journal.json"
    if [[ ! -f "$case_journal" ]]; then
      echo "candidate upgrade E2E: $mode case lost its durable journal" >&2
      exit 1
    fi
    for _ in $(seq 1 60); do
      if compose_project_is_strictly_stopped "$import_project"; then
        case_current_snapshot="$(read_phase_a_instance_snapshot)"
        if [[ "$case_current_snapshot" != "$case_instance_snapshot" ||
          "$(jq -r '.maintenanceStarted' "$case_journal")" != false ||
          "$(jq -r '.maintenanceRecoveryState // empty' "$case_journal")" != snapshot_restored ]]; then
          echo "candidate upgrade E2E: $mode reached a terminal job without exact instance and journal snapshot restoration" >&2
          jq '{maintenanceStarted,maintenanceRecoveryState,lastErrorCode,lastError,phaseAFifoWriteAttempted,upstreamSubmitted,upstreamConfirmed}' "$case_journal" >&2
          exit 1
        fi
        return
      else
        stopped_probe_status=$?
        if ((stopped_probe_status > 1)); then
          echo "candidate upgrade E2E: Docker probe failed while checking $mode stopped state" >&2
          exit 1
        fi
      fi
      sleep 1
    done
    echo "candidate upgrade E2E: $mode case did not restore its exact instance snapshot with only an optional exited server/default network" >&2
    docker ps -a --filter "label=com.docker.compose.project=$import_project" --format 'container={{.Names}} service={{.Label "com.docker.compose.service"}} state={{.State}}' >&2
    docker network ls --filter "label=com.docker.compose.project=$import_project" --format 'network={{.Name}}' >&2
    exit 1
  }

  recover_phase_a_case_with_admin_mutation() {
    local mode="$1"
    local name="$2"
    jq -n '{name:"Existing_1"}' >"$root/save-import-phase-a-select.json"
    select_code="$(curl --silent --show-error --output "$response_file" --write-out '%{http_code}' --cookie "$cookie_file" --header 'Content-Type: application/json' --data-binary "@$root/save-import-phase-a-select.json" "http://127.0.0.1:$panel_port/api/instances/stardew/saves/select")"
    if [[ "$select_code" != 200 || "$(jq -r '.activeSaveName // empty' "$response_file")" != Existing_1 ]]; then
      echo "candidate upgrade E2E: next admin mutation did not recover $mode case with HTTP $select_code" >&2
      cat "$response_file" >&2
      exit 1
    fi
    if [[ -e "$case_journal" || -d "$instance_dir/.local-container/saves/Saves/$name" || -d "$transaction_root/$case_operation/source" ]]; then
      echo "candidate upgrade E2E: next admin mutation retained $mode journal, staged target, or owned source" >&2
      exit 1
    fi
    if [[ -d "$pending_root" && -n "$(find "$pending_root" -mindepth 1 -maxdepth 1 -type d -print -quit)" ]]; then
      echo "candidate upgrade E2E: next admin mutation retained a durable pending-upload token after $mode" >&2
      exit 1
    fi
    if [[ "$(sha256sum "$existing_main" | awk '{print $1}')" != "$existing_save_sha" ||
      "$(sha256sum "$existing_info" | awk '{print $1}')" != "$existing_info_sha" ||
      "$(jq -r '.SaveNameToLoad // empty' "$active_pointer")" != Existing_1 ]]; then
      echo "candidate upgrade E2E: next admin mutation changed the original selected save after $mode" >&2
      exit 1
    fi
  }

  create_phase_a_save_zip Invisible_789 789
  case_name="Invisible_789"
  case_zip="$archive_root/$case_name.zip"
  write_phase_a_compose "$hidden_saves"
  submit_phase_a_case "$case_name" "$case_zip" invisible
  wait_phase_a_case_failed invisible
  if [[ "$(jq -r '.phaseAFifoWriteAttempted' "$case_journal")" != false ||
    "$(jq -r '.upstreamSubmitted' "$case_journal")" != false ||
    "$(jq -r '.upstreamConfirmed' "$case_journal")" != false ||
    "$(jq -r '.maintenanceRecoveryState // empty' "$case_journal")" != snapshot_restored ]]; then
    echo "candidate upgrade E2E: invisible target crossed the FIFO point of no return or did not restore its snapshot" >&2
    cat "$case_journal" >&2
    exit 1
  fi
  if [[ "$(jq -r '.lastErrorCode // empty' "$case_journal")" != save_import_maintenance_not_ready ]]; then
    echo "candidate upgrade E2E: invisible target did not retain the exact fail-closed readiness error" >&2
    cat "$case_journal" >&2
    exit 1
  fi
  recover_phase_a_case_with_admin_mutation invisible "$case_name"

  create_phase_a_save_zip NoEffect_456 456
  case_name="NoEffect_456"
  case_zip="$archive_root/$case_name.zip"
  write_phase_a_compose "$instance_dir/.local-container/saves/Saves"
  submit_phase_a_case "$case_name" "$case_zip" no-effect
  wait_phase_a_case_failed no-effect
  if [[ "$(jq -r '.phaseAFifoWriteAttempted' "$case_journal")" != true ||
    "$(jq -r '.upstreamSubmitted' "$case_journal")" != true ||
    "$(jq -r '.upstreamConfirmed' "$case_journal")" != false ||
    "$(jq -r '.phaseAOutcome // empty' "$case_journal")" != command_failed_no_effect ||
    "$(jq -r '.maintenanceRecoveryState // empty' "$case_journal")" != snapshot_restored ||
    "$(jq -r '.lastErrorCode // empty' "$case_journal")" != import_command_failed ]]; then
    echo "candidate upgrade E2E: FIFO no-effect case did not retain complete safe-cleanup evidence" >&2
    cat "$case_journal" >&2
    exit 1
  fi
  if ! jq -e '.phaseALogDetail | contains("Controlled Junimo diagnostic") and contains("produced no disk effect")' "$case_journal" >/dev/null ||
    grep -F "$raw_platform_id" "$case_journal" >/dev/null; then
    echo "candidate upgrade E2E: Phase A diagnostic was absent or retained the raw platform ID" >&2
    cat "$case_journal" >&2
    exit 1
  fi
  preimport_count="$(find "$instance_dir/.local-container/backups/saves" -maxdepth 1 -type f -name 'preimport_NoEffect_456_*.zip' | wc -l | tr -d '[:space:]')"
  if [[ ! "$preimport_count" =~ ^[1-9][0-9]*$ ]]; then
    echo "candidate upgrade E2E: FIFO no-effect case lost its durable preimport backup before cleanup" >&2
    exit 1
  fi
  recover_phase_a_case_with_admin_mutation no-effect "$case_name"
  if [[ "$(find "$instance_dir/.local-container/backups/saves" -maxdepth 1 -type f -name 'preimport_NoEffect_456_*.zip' | wc -l | tr -d '[:space:]')" != "$preimport_count" ]]; then
    echo "candidate upgrade E2E: FIFO no-effect auto-recovery removed its durable preimport backup" >&2
    exit 1
  fi
  echo "candidate upgrade E2E: invisible target stayed pre-submit; FIFO no-effect evidence was redacted and the next admin mutation recovered automatically"
}

assert_upgraded_legacy_junimo_repair() {
  local instance_dir="$data_dir/instances/stardew"
  local instance_compose="$instance_dir/docker-compose.yml"
  local runtime_manifest="/workspace/backend/internal/games/stardew_junimo/config/runtime_stack_manifest.json"
  local fixture_image_dir="$root/runtime-repair-image"
  local control_source="/workspace/backend/internal/games/stardew_junimo/embedded/smapi-mod"
  local server_ref=""
  local auth_ref=""
  local server_candidates=""
  local auth_candidates=""
  local server_version=""
  local stack_version=""
  local control_version=""
  local server_image_id=""
  local auth_image_id=""
  local apply_id="apply_666666666666666666666666"
  local recovery_dir="$instance_dir/.local-container/junimo-update/recovery/$apply_id"
  local selected_file="$root/runtime-repair-selected.json"
  local inspection_file="$root/runtime-repair-inspection.json"
  local manifest_file="$recovery_dir/manifest.json"
  local apply_status_file="$instance_dir/.local-container/junimo-update/apply-status.json"
  local original_env_sha=""
  local original_compose_sha=""
  local original_control_manifest_sha=""
  local original_control_dll_sha=""
  local server_container_id=""
  local auth_container_id=""
  local repair_code=""
  local repair_job_id=""
  local repair_phase=""
  local repair_source=""
  local repaired_version=""
  local instance_state=""
  local response_code=""
  local now=""

  echo "candidate upgrade E2E: testing legacy rollback_failed Junimo materialization through public repair API"
  server_ref="$(jq -r '.server.images[0]' "$runtime_manifest")"
  auth_ref="$(jq -r '.steamAuth.images[0]' "$runtime_manifest")"
  server_candidates="$(jq -r '.server.images | join(",")' "$runtime_manifest")"
  auth_candidates="$(jq -r '.steamAuth.images | join(",")' "$runtime_manifest")"
  server_version="$(jq -r '.server.tag' "$runtime_manifest")"
  stack_version="$(jq -r '.stackVersion' "$runtime_manifest")"
  control_version="$(jq -r '.controlMod.version' "$runtime_manifest")"

  mkdir -p "$fixture_image_dir/JunimoServer" "$instance_dir/.local-container/mods/StardewAnxiPanel.Control" "$instance_dir/.local-container/control"
  cat >"$fixture_image_dir/bash" <<'EOF'
#!/bin/sh
if [ "${1:-}" = "-c" ]; then
  printf '{"status":"ok"}\n'
  exit 0
fi
exec /bin/sh "$@"
EOF
  chmod 755 "$fixture_image_dir/bash"
  printf '%s\n' '{"Name":"JunimoServer","Version":"'"$server_version"'","UniqueID":"JunimoHost.Server"}' >"$fixture_image_dir/JunimoServer/manifest.json"
  printf 'release-candidate-runtime-repair-dll\n' >"$fixture_image_dir/JunimoServer/JunimoServer.dll"
  cat >"$fixture_image_dir/Dockerfile" <<'EOF'
FROM alpine:3.20
COPY bash /bin/bash
COPY JunimoServer /data/Mods/JunimoServer
EOF
  docker build --network none --tag "$project/runtime-repair-server:$version" "$fixture_image_dir" >/dev/null
  server_image_id="$(docker image inspect "$project/runtime-repair-server:$version" | jq -r '.[0].Id')"
  auth_image_id="$(docker image inspect alpine:3.20 | jq -r '.[0].Id')"
  docker tag "$server_image_id" "$server_ref"
  docker tag "$auth_image_id" "$auth_ref"

  cp "$control_source/manifest.json" "$instance_dir/.local-container/mods/StardewAnxiPanel.Control/manifest.json"
  cp "$control_source/StardewAnxiPanel.Control.dll" "$instance_dir/.local-container/mods/StardewAnxiPanel.Control/StardewAnxiPanel.Control.dll"
  printf '%s\n' '{"controlModVersion":"'"$control_version"'"}' >"$instance_dir/.local-container/control/options.json"
  printf '%s\n' '{"state":"save-loaded","commandResultVersion":1}' >"$instance_dir/.local-container/control/status.json"
  rm -rf -- "$instance_dir/.local-container/mods/JunimoServer"

  cat >"$instance_dir/.env" <<EOF
IMAGE_VERSION=$server_version
SERVER_IMAGE=$server_ref
SERVER_IMAGE_CANDIDATES=$server_candidates
STEAM_SERVICE_IMAGE=$auth_ref
STEAM_SERVICE_IMAGE_CANDIDATES=$auth_candidates
STEAM_INVITE_ENABLED=true
STEAM_INVITE_AUTH_STATE=ready
STEAM_AUTH_COMPLETED=true
INSTANCE_HOST_DATA_DIR=$instance_dir
EOF
  chmod 600 "$instance_dir/.env"
  cat >"$instance_compose" <<'EOF'
services:
  server:
    image: ${SERVER_IMAGE}
    command:
      - sh
      - -c
      - |
        rm -f /tmp/smapi-input /tmp/server-output.log
        mkfifo /tmp/smapi-input
        : > /tmp/server-output.log
        while true; do
          while IFS= read -r line; do
            if [ "$$line" = info ]; then
              printf '[INFO JunimoServer] --- Server Info ---\n[INFO JunimoServer] Version: __SERVER_VERSION__\n[INFO JunimoServer] Status: Ready\n' >> /tmp/server-output.log
            fi
          done < /tmp/smapi-input
        done &
        exec tail -f /dev/null
    volumes:
      - ${INSTANCE_HOST_DATA_DIR}/.local-container/mods:/data/Mods
      - ${INSTANCE_HOST_DATA_DIR}/.local-container/control:/data/control
  steam-auth:
    image: ${STEAM_SERVICE_IMAGE}
    command: ["sleep", "3600"]
EOF
  sed -i "s/__SERVER_VERSION__/$server_version/g" "$instance_compose"
  chmod 600 "$instance_compose"

  docker compose --project-name stardew --project-directory "$instance_dir" -f "$instance_compose" up -d server steam-auth >/dev/null
  docker compose --project-name stardew --project-directory "$instance_dir" -f "$instance_compose" stop server steam-auth >/dev/null
  server_container_id="$(docker compose --project-name stardew --project-directory "$instance_dir" -f "$instance_compose" ps --all --format json server | jq -r 'if type == "array" then .[0].ID else .ID end')"
  auth_container_id="$(docker compose --project-name stardew --project-directory "$instance_dir" -f "$instance_compose" ps --all --format json steam-auth | jq -r 'if type == "array" then .[0].ID else .ID end')"
  if [[ -z "$server_container_id" || "$server_container_id" == null || -z "$auth_container_id" || "$auth_container_id" == null ]]; then
    echo "candidate upgrade E2E: failed to create the stopped server + preserved steam-auth fixture" >&2
    exit 1
  fi
  if [[ "$(sqlite3 -cmd '.timeout 5000' "$data_dir/panel.db" "UPDATE instances SET state='stopped', state_message='legacy runtime repair fixture', driver_phase='stopped', driver_payload='{}' WHERE id='stardew'; SELECT changes();")" != 1 ]]; then
    echo "candidate upgrade E2E: failed to prepare the runtime repair instance state" >&2
    exit 1
  fi

  curl --silent --show-error --fail --cookie "$cookie_file" "http://127.0.0.1:$panel_port/api/instances/stardew/junimo-update" >"$inspection_file"
  if [[ "$(jq -r '.status // empty' "$inspection_file")" != up_to_date || "$(jq -r '.recommended.stackVersion // empty' "$inspection_file")" != "$stack_version" ]]; then
    echo "candidate upgrade E2E: runtime repair fixture is not the recommended current stack" >&2
    cat "$inspection_file" >&2
    exit 1
  fi

  jq -n --arg server_image "$server_ref" --arg server_id "$server_image_id" --arg auth_image "$auth_ref" --arg auth_id "$auth_image_id" '{server:{image:$server_image,digest:$server_id,imageId:$server_id},steamAuth:{image:$auth_image,digest:$auth_id,imageId:$auth_id}}' >"$selected_file"
  mkdir -p "$recovery_dir"
  cp "$instance_dir/.env" "$recovery_dir/original.env"
  cp "$instance_compose" "$recovery_dir/original-compose.yml"
  cp "$instance_dir/.local-container/mods/StardewAnxiPanel.Control/manifest.json" "$recovery_dir/original-control-manifest.json"
  cp "$instance_dir/.local-container/mods/StardewAnxiPanel.Control/StardewAnxiPanel.Control.dll" "$recovery_dir/original-control-StardewAnxiPanel.Control.dll"
  original_env_sha="$(sha256sum "$recovery_dir/original.env" | awk '{print $1}')"
  original_compose_sha="$(sha256sum "$recovery_dir/original-compose.yml" | awk '{print $1}')"
  original_control_manifest_sha="$(sha256sum "$recovery_dir/original-control-manifest.json" | awk '{print $1}')"
  original_control_dll_sha="$(sha256sum "$recovery_dir/original-control-StardewAnxiPanel.Control.dll" | awk '{print $1}')"
  now="$(date -u +'%Y-%m-%dT%H:%M:%SZ')"

  jq -n --slurpfile selected "$selected_file" --arg apply_id "$apply_id" --arg server_version "$server_version" --arg env_sha "$original_env_sha" --arg compose_sha "$original_compose_sha" --arg control_manifest_sha "$original_control_manifest_sha" --arg control_dll_sha "$original_control_dll_sha" '{schemaVersion:3,applyId:$apply_id,actorId:1,project:"stardew",steamSessionVolume:"stardew_steam-session",snapshotVolume:("stardew_anxi-junimo-update-"+($apply_id|sub("^apply_";""))+"-steam-session"),serverWasRunning:false,authWasRunning:false,serverImageChanged:false,authImageChanged:false,authSnapshotCreated:false,originalState:"stopped",originalServer:$selected[0].server,originalAuth:$selected[0].steamAuth,target:$selected[0],originalServerVersion:$server_version,targetServerVersion:$server_version,junimoModOriginalPresent:false,junimoModPrepared:false,junimoModReplaced:false,configWritten:false,authRecreated:false,serverRecreated:false,controlManifestPresent:true,controlDLLPresent:true,controlUpdated:true,mutationStarted:true,stopIntent:true,controlUpdateIntent:true,lastIntent:"control_update",originalEnvSha256:$env_sha,originalComposeSha256:$compose_sha,originalControlManifestSha256:$control_manifest_sha,originalControlDllSha256:$control_dll_sha}' >"$manifest_file"
  jq --slurpfile selected "$selected_file" --arg apply_id "$apply_id" --arg now "$now" '{applyId:$apply_id,phase:"rollback_failed",progress:100,current:.current,target:.recommended,selected:$selected[0],checks:[],warnings:[],logs:[],serverWasRunning:false,serverRunning:false,errorCode:"rollback_failed",error:"legacy Control-only fixture",causeCode:"server_container_not_ready",causeError:"新版 Junimo server 运行验证失败。",rollbackCode:"rollback_verify_server_failed",rollbackError:"升级前的 Junimo server 未能恢复就绪。",repairAttempts:2,manualAction:"fixture",startedAt:$now,updatedAt:$now,finishedAt:$now}' "$inspection_file" >"$apply_status_file"
  chmod 600 "$manifest_file" "$apply_status_file" "$recovery_dir"/*

  docker image rm "$server_ref" >/dev/null
  curl --silent --show-error --fail --cookie "$cookie_file" "http://127.0.0.1:$panel_port/api/instances/stardew/junimo-update" >"$response_file"
  repair_code="$(jq -r '.repairPlan.code // empty' "$response_file")"
  if [[ "$repair_code" != repair/rollback_failed || "$(jq -r '.repairPlan.actionAvailable // false' "$response_file")" != true || "$(jq -r '.repairPlan.attempts // -1' "$response_file")" != 2 ]]; then
    echo "candidate upgrade E2E: upgraded Panel did not expose the bounded legacy repair plan" >&2
    cat "$response_file" >&2
    exit 1
  fi

  response_code="$(curl --silent --show-error --output "$response_file" --write-out '%{http_code}' --cookie "$cookie_file" --header 'Content-Type: application/json' --data '{"confirm":true}' "http://127.0.0.1:$panel_port/api/instances/stardew/junimo-update/repair")"
  repair_job_id="$(jq -r '.jobId // empty' "$response_file")"
  if [[ "$response_code" != 202 || -z "$repair_job_id" || "$(jq -r '.repairAttempts // 0' "$response_file")" != 3 ]]; then
    echo "candidate upgrade E2E: legacy runtime repair was not accepted as the third bounded attempt" >&2
    cat "$response_file" >&2
    exit 1
  fi

  for _ in $(seq 1 180); do
    if curl --silent --show-error --fail --cookie "$cookie_file" "http://127.0.0.1:$panel_port/api/instances/stardew/junimo-update/apply" >"$response_file" 2>/dev/null; then
      repair_phase="$(jq -r '.phase // empty' "$response_file")"
      if [[ "$repair_phase" == succeeded || "$repair_phase" == failed_rolled_back || "$repair_phase" == rollback_failed ]]; then
        break
      fi
    fi
    sleep 1
  done
  repair_source="$(jq -r '.repairSourceApplyId // empty' "$response_file")"
  if [[ "$repair_phase" != succeeded || "$(jq -r '.repairAttempts // 0' "$response_file")" != 3 || "$repair_source" != "$apply_id" ]]; then
    echo "candidate upgrade E2E: legacy Junimo repair did not converge successfully" >&2
    cat "$response_file" >&2
    docker logs "$panel_container" 2>&1 | tail -n 120 >&2 || true
    exit 1
  fi
  repaired_version="$(jq -r '.Version // empty' "$instance_dir/.local-container/mods/JunimoServer/manifest.json")"
  instance_state="$(curl --silent --show-error --fail --cookie "$cookie_file" "http://127.0.0.1:$panel_port/api/instances/stardew" | jq -r '.state // empty')"
  if [[ "$repaired_version" != "$server_version" || "$instance_state" != stopped || -d "$recovery_dir" ]]; then
    echo "candidate upgrade E2E: repaired Junimo material, stopped state, or recovery cleanup is incorrect" >&2
    exit 1
  fi
  if [[ "$(docker compose --project-name stardew --project-directory "$instance_dir" -f "$instance_compose" ps --all --format json steam-auth | jq -r 'if type == "array" then .[0].ID else .ID end')" != "$auth_container_id" ]]; then
    echo "candidate upgrade E2E: unchanged steam-auth container was recreated during legacy repair" >&2
    exit 1
  fi
  server_container_id="$(docker compose --project-name stardew --project-directory "$instance_dir" -f "$instance_compose" ps --all --format json server | jq -r 'if type == "array" then .[0].ID else .ID end')"
  if [[ -z "$server_container_id" || "$server_container_id" == null ||
    "$(docker inspect "$server_container_id" | jq -r '.[0].State.Running')" != false ||
    "$(docker inspect "$auth_container_id" | jq -r '.[0].State.Running')" != false ]]; then
    echo "candidate upgrade E2E: legacy repair did not restore the original stopped runtime state" >&2
    exit 1
  fi
  echo "candidate upgrade E2E: legacy third-attempt repair restored Junimo from the immutable original image ID"
}

wait_version "$previous_version" 120

setup_code="$(curl --silent --show-error --output "$response_file" --write-out '%{http_code}' --cookie-jar "$cookie_file" --header 'Content-Type: application/json' --data '{"username":"admin","password":"release-candidate-password","confirmPassword":"release-candidate-password"}' "http://127.0.0.1:$panel_port/api/setup/admin")"
if [[ "$setup_code" != 201 && "$setup_code" != 200 ]]; then
  echo "candidate upgrade E2E: admin setup failed with HTTP $setup_code" >&2
  cat "$response_file" >&2
  exit 1
fi

printf 'panel-sentinel\n' >"$data_dir/release-candidate-sentinel.txt"
docker exec "$game_container" sh -c 'printf "game-sentinel\n" > /game/sentinel.txt'
game_id_before="$(docker inspect "$game_container" | jq -r '.[0].Id')"
game_hash_before="$(docker exec "$game_container" sha256sum /game/sentinel.txt | awk '{print $1}')"

post_update_check() {
  local code
  code="$(curl --silent --show-error --output "$response_file" --write-out '%{http_code}' --cookie "$cookie_file" --request POST "http://127.0.0.1:$panel_port/api/system/update/check")"
  if [[ "$code" != 200 || "$(jq -r '.latestVersion // empty' "$response_file")" != "v$version" || "$(jq -r '.updateAvailable // false' "$response_file")" != true ]]; then
    echo "candidate upgrade E2E: update check failed with HTTP $code" >&2
    cat "$response_file" >&2
    return 1
  fi
}

run_dry_run() {
  local code phase deadline
  code="$(curl --silent --show-error --output "$response_file" --write-out '%{http_code}' --cookie "$cookie_file" --header 'Content-Type: application/json' --data "{\"targetVersion\":\"$version\"}" "http://127.0.0.1:$panel_port/api/system/update/dry-run")"
  if [[ "$code" != 202 ]]; then
    echo "candidate upgrade E2E: dry-run start failed with HTTP $code" >&2
    cat "$response_file" >&2
    return 1
  fi
  deadline=$((SECONDS + 180))
  while ((SECONDS < deadline)); do
    if curl --silent --show-error --max-time 3 --cookie "$cookie_file" "http://127.0.0.1:$panel_port/api/system/update/dry-run" >"$response_file" 2>/dev/null; then
      phase="$(jq -r '.phase // empty' "$response_file")"
      if [[ "$phase" == succeeded ]]; then
        return 0
      fi
      if [[ "$phase" == failed || "$phase" == unsupported ]]; then
        echo "candidate upgrade E2E: dry-run ended in $phase" >&2
        cat "$response_file" >&2
        return 1
      fi
    fi
    sleep 1
  done
  echo "candidate upgrade E2E: dry-run timed out" >&2
  return 1
}

start_apply() {
  local code
  set +e
  code="$(curl --silent --show-error --max-time 15 --output "$response_file" --write-out '%{http_code}' --cookie "$cookie_file" --header 'Content-Type: application/json' --data '{"confirmFullStack":true}' "http://127.0.0.1:$panel_port/api/system/update/apply")"
  local curl_status=$?
  set -e
  if ((curl_status == 0)) && [[ "$code" == 202 ]]; then
    return 0
  fi
  if ((curl_status == 0)) && [[ "$code" == 400 ]]; then
    set +e
    code="$(curl --silent --show-error --max-time 15 --output "$response_file" --write-out '%{http_code}' --cookie "$cookie_file" --request POST "http://127.0.0.1:$panel_port/api/system/update/apply")"
    curl_status=$?
    set -e
    if ((curl_status == 0)) && [[ "$code" == 202 ]]; then
      return 0
    fi
  fi
  if ((curl_status != 0)) || [[ "$code" == 000 ]]; then
    echo "candidate upgrade E2E: apply connection closed during expected Panel replacement"
    return 0
  fi
  echo "candidate upgrade E2E: apply start failed with HTTP $code" >&2
  cat "$response_file" >&2
  return 1
}

wait_apply_phase() {
  local expected_phase="$1"
  local timeout_seconds="$2"
  local phase deadline
  deadline=$((SECONDS + timeout_seconds))
  while ((SECONDS < deadline)); do
    if curl --silent --show-error --max-time 3 --cookie "$cookie_file" "http://127.0.0.1:$panel_port/api/system/update/apply" >"$response_file" 2>/dev/null; then
      phase="$(jq -r '.phase // empty' "$response_file")"
      if [[ "$phase" == "$expected_phase" ]]; then
        return 0
      fi
      if [[ "$phase" == rollback_failed ]]; then
        echo "candidate upgrade E2E: apply reached rollback_failed" >&2
        cat "$response_file" >&2
        return 1
      fi
      if [[ "$expected_phase" == succeeded && "$phase" == failed_rolled_back ]]; then
        echo "candidate upgrade E2E: healthy candidate was rolled back" >&2
        cat "$response_file" >&2
        return 1
      fi
    fi
    sleep 1
  done
  echo "candidate upgrade E2E: timed out waiting for apply phase $expected_phase" >&2
  if [[ -s "$response_file" ]]; then
    cat "$response_file" >&2
  fi
  return 1
}

write_legacy_steam_invite_fixture() {
  local mode="$1"
  local instance_dir="$data_dir/instances/stardew"
  local instance_env="$instance_dir/.env"
  local changed=""
  local instance_state="running"
  local instance_phase="running"

  if [[ ! -f "$instance_env" ]]; then
    echo "candidate upgrade E2E: legacy Steam invite fixture has no instance .env" >&2
    return 1
  fi
  sed -i \
    -e '/^STEAM_INVITE_ENABLED=/d' \
    -e '/^STEAM_INVITE_AUTH_STATE=/d' \
    -e '/^STEAM_INVITE_RUNTIME_SCOPE_VERSION=/d' \
    -e '/^STEAM_AUTH_COMPLETED=/d' \
    -e '/^STEAMCMD_AUTH_COMPLETED=/d' \
    "$instance_env"
  printf 'STEAMCMD_AUTH_COMPLETED=true\n' >>"$instance_env"
  case "$mode" in
    no-auth)
      ;;
    authorized)
      printf 'STEAM_AUTH_COMPLETED=true\n' >>"$instance_env"
      instance_state="game_installed"
      instance_phase="game_installed"
      ;;
    *)
      echo "candidate upgrade E2E: unknown legacy Steam invite fixture mode: $mode" >&2
      return 1
      ;;
  esac
  chmod 600 "$instance_env"

  changed="$(sqlite3 -cmd '.timeout 5000' "$data_dir/panel.db" "UPDATE instances SET state='$instance_state', state_message='legacy Steam invite migration fixture', driver_phase='$instance_phase', driver_payload='{}' WHERE id='stardew'; SELECT changes();")"
  if [[ "$changed" != 1 ]]; then
    echo "candidate upgrade E2E: failed to prepare the legacy Steam invite instance row" >&2
    return 1
  fi
}

snapshot_candidate_auth_images() {
  local output_file="$1"
  local auth_image=""
  local image_id=""

  if [[ ! -f "$candidate_runtime_manifest" ]]; then
    echo "candidate upgrade E2E: candidate runtime manifest is missing" >&2
    return 1
  fi
  : >"$output_file"
  while IFS= read -r auth_image; do
    [[ -z "$auth_image" ]] && continue
    image_id="absent"
    if docker image inspect "$auth_image" >"$response_file" 2>/dev/null; then
      image_id="$(jq -r '.[0].Id' "$response_file")"
    fi
    printf '%s\t%s\n' "$auth_image" "$image_id" >>"$output_file"
  done < <(jq -r '.steamAuth.images[]' "$candidate_runtime_manifest" | sort -u)
}

assert_candidate_auth_images_unchanged() {
  local current_snapshot="$root/legacy-disabled-auth-images.current"

  snapshot_candidate_auth_images "$current_snapshot"
  if [[ "$(sha256sum "$current_snapshot" | awk '{print $1}')" != "$(sha256sum "$legacy_auth_image_snapshot_before" | awk '{print $1}')" ]]; then
    echo "candidate upgrade E2E: disabled migration pulled, removed, or retagged an optional Auth candidate image" >&2
    echo "before:" >&2
    cat "$legacy_auth_image_snapshot_before" >&2
    echo "after:" >&2
    cat "$current_snapshot" >&2
    return 1
  fi
  if [[ "$(docker image inspect "$legacy_auth_image_ref" | jq -r '.[0].Id')" != "$legacy_auth_image_id" ]]; then
    echo "candidate upgrade E2E: the pre-existing optional Auth image ID changed" >&2
    return 1
  fi
}

write_legacy_runtime_compose() {
  local output_file="$1"
  local include_auth_dependency="$2"
  # shellcheck disable=SC2016 # Compose must receive this literal placeholder.
  local auth_image_literal='${STEAM_SERVICE_IMAGE}'

  {
    cat <<EOF
services:
  fixture-ready:
    image: alpine:3.20
    command: ["sleep", "3600"]
    labels:
      com.anxi-panel.test-owner: "$owner"
      com.anxi-panel.fixture-role: "legacy-harmless-dependency"
  steam-auth:
    image: $auth_image_literal
    cpu_shares: 256
    command: ["sleep", "3600"]
    labels:
      com.anxi-panel.test-owner: "$owner"
      com.anxi-panel.fixture-role: "legacy-optional-auth"
    volumes:
      - steam-session:/session
  server:
    image: alpine:3.20
    cpu_shares: 768
    command: ["sleep", "3600"]
    labels:
      com.anxi-panel.test-owner: "$owner"
      com.anxi-panel.fixture-role: "legacy-server"
    depends_on:
EOF
    if [[ "$include_auth_dependency" == true ]]; then
      cat <<'EOF'
      steam-auth:
        condition: service_started
EOF
    fi
    cat <<'EOF'
      fixture-ready:
        condition: service_started
    environment:
      SAP_PLAYER_AUTH_MODE: ""
      SAP_PLAYER_AUTH_REVISION: ""
      SAP_ROLE_AUTH_KEY: ""
      SAP_ROLE_PASSWORDS_B64: ""
      ALSOFT_DRIVERS: "null"
      SDL_AUDIODRIVER: "dummy"
      FIXTURE_OTHER_DEPENDENCY: "preserved"
volumes:
  steam-session:
    external: true
    name: stardew_steam-session
EOF
  } >"$output_file"
  chmod 600 "$output_file"
}

assert_no_steam_auth_artifacts() {
  if [[ -n "$(docker ps -a --filter 'label=com.docker.compose.project=stardew' --filter 'label=com.docker.compose.service=steam-auth' --quiet)" ]]; then
    echo "candidate upgrade E2E: disabled legacy fixture still has a steam-auth container" >&2
    return 1
  fi
  if docker volume inspect "$steam_session_volume" >/dev/null 2>&1; then
    echo "candidate upgrade E2E: disabled legacy fixture still has its exact Steam session volume" >&2
    return 1
  fi
  assert_candidate_auth_images_unchanged
}

assert_disabled_legacy_runtime_converged() {
  local instance_dir="$data_dir/instances/stardew"
  local instance_env="$instance_dir/.env"
  local instance_compose="$instance_dir/docker-compose.yml"
  local compose_config="$root/legacy-disabled-compose-config.json"
  local current_server_id=""
  local current_dependency_id=""

  current_server_id="$(docker ps -a --filter 'label=com.docker.compose.project=stardew' --filter 'label=com.docker.compose.service=server' --format '{{.ID}}')"
  current_dependency_id="$(docker ps -a --filter 'label=com.docker.compose.project=stardew' --filter 'label=com.docker.compose.service=fixture-ready' --format '{{.ID}}')"
  if [[ "$current_server_id" != "$legacy_server_container_id" || "$current_dependency_id" != "$legacy_dependency_container_id" ]]; then
    echo "candidate upgrade E2E: disabled migration recreated the server or harmless dependency" >&2
    return 1
  fi
  if [[ "$(docker inspect "$legacy_server_container_id" | jq -r '.[0].State.StartedAt')" != "$legacy_server_started_at" ]] ||
    [[ "$(docker inspect "$legacy_server_container_id" | jq -r '.[0].State.Running')" != true ]] ||
    [[ "$(docker inspect "$legacy_dependency_container_id" | jq -r '.[0].State.Running')" != true ]]; then
    echo "candidate upgrade E2E: disabled migration stopped or restarted a required server-side container" >&2
    return 1
  fi
  if docker inspect "$legacy_auth_container_id" >/dev/null 2>&1; then
    echo "candidate upgrade E2E: disabled migration retained the exact legacy steam-auth container" >&2
    return 1
  fi
  if [[ "$(sha256sum "$instance_compose" | awk '{print $1}')" != "$(sha256sum "$legacy_expected_migrated_compose" | awk '{print $1}')" ]]; then
    echo "candidate upgrade E2E: Compose migration changed content beyond the server steam-auth dependency" >&2
    diff -u "$legacy_expected_migrated_compose" "$instance_compose" >&2 || true
    return 1
  fi
  if [[ "$(grep -c '^STEAM_INVITE_RUNTIME_SCOPE_VERSION=1$' "$instance_env")" != 1 ]]; then
    echo "candidate upgrade E2E: disabled migration did not persist the runtime-scope convergence marker exactly once" >&2
    return 1
  fi
  docker compose --project-name stardew --project-directory "$instance_dir" -f "$instance_compose" config --format json >"$compose_config"
  if ! jq -e '.services.server.depends_on | (has("steam-auth") | not) and has("fixture-ready")' "$compose_config" >/dev/null ||
    ! jq -e '.services | has("steam-auth") and has("fixture-ready")' "$compose_config" >/dev/null ||
    ! jq -e '.services.server.environment.FIXTURE_OTHER_DEPENDENCY == "preserved"' "$compose_config" >/dev/null; then
    echo "candidate upgrade E2E: migrated Compose lost its optional service or unrelated dependency" >&2
    cat "$compose_config" >&2
    return 1
  fi
  assert_no_steam_auth_artifacts
}

prepare_legacy_disabled_runtime_fixture() {
  local instance_dir="$data_dir/instances/stardew"
  local instance_env="$instance_dir/.env"
  local instance_compose="$instance_dir/docker-compose.yml"
  local auth_candidates=""

  docker compose --project-name "$project" --env-file "$env_file" -f "$compose_file" stop panel >/dev/null
  if [[ -n "$(docker ps -a --filter 'label=com.docker.compose.project=stardew' --quiet)" ]] || docker volume inspect "$steam_session_volume" >/dev/null 2>&1; then
    echo "candidate upgrade E2E: refusing to overwrite unexpected stardew runtime resources before the legacy fixture" >&2
    return 1
  fi

  legacy_auth_image_ref="$(jq -r '.steamAuth.images[0]' "$candidate_runtime_manifest")"
  auth_candidates="$(jq -r '.steamAuth.images | join(",")' "$candidate_runtime_manifest")"
  if [[ -z "$legacy_auth_image_ref" || "$legacy_auth_image_ref" == null || -z "$auth_candidates" ]]; then
    echo "candidate upgrade E2E: candidate has no optional Auth image candidates" >&2
    return 1
  fi
  docker tag alpine:3.20 "$legacy_auth_image_ref"
  legacy_auth_image_id="$(docker image inspect "$legacy_auth_image_ref" | jq -r '.[0].Id')"

  write_legacy_steam_invite_fixture no-auth
  sed -i -e '/^STEAM_SERVICE_IMAGE=/d' -e '/^STEAM_SERVICE_IMAGE_CANDIDATES=/d' "$instance_env"
  printf 'STEAM_SERVICE_IMAGE=%s\nSTEAM_SERVICE_IMAGE_CANDIDATES=%s\n' "$legacy_auth_image_ref" "$auth_candidates" >>"$instance_env"
  chmod 600 "$instance_env"
  write_legacy_runtime_compose "$instance_compose" true
  write_legacy_runtime_compose "$legacy_expected_migrated_compose" false

  legacy_disabled_session_hash="$(seed_legacy_steam_session disabled-legacy)"
  if [[ -z "$legacy_disabled_session_hash" ]]; then
    echo "candidate upgrade E2E: disabled legacy Steam session sentinel was not created" >&2
    return 1
  fi
  snapshot_candidate_auth_images "$legacy_auth_image_snapshot_before"
  docker compose --project-name stardew --project-directory "$instance_dir" -f "$instance_compose" up -d fixture-ready server steam-auth >/dev/null

  docker tag "$previous_fixture_ref" "$previous_ref"
  sed -i "s|^PANEL_IMAGE=.*$|PANEL_IMAGE=$previous_ref|" "$env_file"
  docker compose --project-name "$project" --env-file "$env_file" -f "$compose_file" up -d --no-deps --force-recreate panel >/dev/null
  wait_version "$previous_version" 120
  if [[ "$(sha256sum "$instance_compose" | awk '{print $1}')" == "$(sha256sum "$legacy_expected_migrated_compose" | awk '{print $1}')" ]]; then
    echo "candidate upgrade E2E: previous release unexpectedly removed the legacy steam-auth dependency" >&2
    return 1
  fi

  legacy_server_container_id="$(docker ps -a --filter 'label=com.docker.compose.project=stardew' --filter 'label=com.docker.compose.service=server' --format '{{.ID}}')"
  legacy_dependency_container_id="$(docker ps -a --filter 'label=com.docker.compose.project=stardew' --filter 'label=com.docker.compose.service=fixture-ready' --format '{{.ID}}')"
  legacy_auth_container_id="$(docker ps -a --filter 'label=com.docker.compose.project=stardew' --filter 'label=com.docker.compose.service=steam-auth' --format '{{.ID}}')"
  docker run -d --name "$unknown_session_holder_container" --label "com.anxi-panel.test-owner=$owner" --label "com.anxi-panel.fixture-role=unknown-steam-session-holder" --volume "$steam_session_volume:/untrusted-session" alpine:3.20 sleep 3600 >/dev/null
  unknown_session_holder_id="$(docker inspect "$unknown_session_holder_container" | jq -r '.[0].Id')"
  if [[ -z "$legacy_server_container_id" || -z "$legacy_dependency_container_id" || -z "$legacy_auth_container_id" ]] ||
    [[ -z "$unknown_session_holder_id" ]] ||
    [[ "$(docker inspect "$legacy_auth_container_id" | jq -r '.[0].Image')" != "$legacy_auth_image_id" ]] ||
    ! docker inspect "$legacy_auth_container_id" | jq -e --arg volume "$steam_session_volume" '.[0].Mounts | any(.Name == $volume)' >/dev/null; then
    echo "candidate upgrade E2E: failed to establish the running legacy server + steam-auth fixture" >&2
    return 1
  fi
  legacy_server_started_at="$(docker inspect "$legacy_server_container_id" | jq -r '.[0].State.StartedAt')"
  if [[ "$(read_steam_session_sentinel_hash)" != "$legacy_disabled_session_hash" ]]; then
    echo "candidate upgrade E2E: previous release changed the disabled legacy session sentinel" >&2
    return 1
  fi
  echo "candidate upgrade E2E: previous release is running the legacy server + steam-auth dependency with an unknown same-volume holder" >&2
}

assert_unknown_session_holder_fail_closed() {
  local instance_dir="$data_dir/instances/stardew"
  local instance_env="$instance_dir/.env"
  local instance_compose="$instance_dir/docker-compose.yml"
  local current_server_id=""
  local current_dependency_id=""

  if [[ "$(grep -c '^STEAM_INVITE_ENABLED=false$' "$instance_env")" != 1 ]] ||
    grep -q '^STEAM_INVITE_RUNTIME_SCOPE_VERSION=' "$instance_env"; then
    echo "candidate upgrade E2E: unknown holder did not preserve explicit disabled intent with the runtime-scope marker still absent" >&2
    return 1
  fi
  current_server_id="$(docker ps -a --filter 'label=com.docker.compose.project=stardew' --filter 'label=com.docker.compose.service=server' --format '{{.ID}}')"
  current_dependency_id="$(docker ps -a --filter 'label=com.docker.compose.project=stardew' --filter 'label=com.docker.compose.service=fixture-ready' --format '{{.ID}}')"
  if [[ "$current_server_id" != "$legacy_server_container_id" || "$current_dependency_id" != "$legacy_dependency_container_id" ]] ||
    ! docker inspect "$legacy_auth_container_id" "$unknown_session_holder_id" >/dev/null 2>&1 ||
    [[ "$(docker inspect "$legacy_server_container_id" | jq -r '.[0].State.StartedAt')" != "$legacy_server_started_at" ]] ||
    [[ "$(docker inspect "$legacy_server_container_id" | jq -r '.[0].State.Running')" != true ]] ||
    [[ "$(docker inspect "$legacy_auth_container_id" | jq -r '.[0].State.Running')" != true ]] ||
    [[ "$(docker inspect "$unknown_session_holder_id" | jq -r '.[0].State.Running')" != true ]]; then
    echo "candidate upgrade E2E: unknown holder caused a partial container deletion, stop, or recreation" >&2
    return 1
  fi
  if ! docker volume inspect "$steam_session_volume" >/dev/null 2>&1 ||
    [[ "$(read_steam_session_sentinel_hash)" != "$legacy_disabled_session_hash" ]]; then
    echo "candidate upgrade E2E: unknown holder caused the Steam session volume or sentinel to change" >&2
    return 1
  fi
  if [[ "$(sha256sum "$instance_compose" | awk '{print $1}')" != "$(sha256sum "$legacy_expected_migrated_compose" | awk '{print $1}')" ]]; then
    echo "candidate upgrade E2E: unknown-holder Prepare changed Compose beyond the safe dependency migration" >&2
    diff -u "$legacy_expected_migrated_compose" "$instance_compose" >&2 || true
    return 1
  fi
  assert_candidate_auth_images_unchanged

  curl --silent --show-error --fail --cookie "$cookie_file" "http://127.0.0.1:$panel_port/api/instances/stardew/state" >"$response_file"
  if [[ "$(jq -r '.steamInviteEnabled' "$response_file")" != false ||
    "$(jq -r '.steamInviteAuthState' "$response_file")" != disabled ||
    "$(jq -r '.steamAuthLoggedIn' "$response_file")" != false ]]; then
    echo "candidate upgrade E2E: unknown-holder migration did not expose the explicit disabled state" >&2
    cat "$response_file" >&2
    return 1
  fi
  curl --silent --show-error --fail --cookie "$cookie_file" "http://127.0.0.1:$panel_port/api/instances/stardew/invite-code" >"$response_file"
  if [[ "$(jq -r '.status' "$response_file")" != disabled || "$(jq -r '.steamInviteEnabled' "$response_file")" != false ]] ||
    ! docker inspect "$legacy_auth_container_id" "$unknown_session_holder_id" >/dev/null 2>&1 ||
    [[ "$(read_steam_session_sentinel_hash)" != "$legacy_disabled_session_hash" ]]; then
    echo "candidate upgrade E2E: disabled invite GET changed the fail-closed holder/session evidence" >&2
    cat "$response_file" >&2
    return 1
  fi
  echo "candidate upgrade E2E: unknown same-volume holder kept all holders, session data, and runtime marker fail closed"
}

remove_owned_unknown_session_holder() {
  if [[ -z "$unknown_session_holder_id" ]] ||
    ! docker inspect "$unknown_session_holder_id" | jq -e --arg owner "$owner" --arg volume "$steam_session_volume" '.[0].Config.Labels["com.anxi-panel.test-owner"] == $owner and (.[0].Mounts | any(.Type == "volume" and .Name == $volume and .Destination == "/untrusted-session"))' >/dev/null; then
    echo "candidate upgrade E2E: refusing to remove an unproven unknown Steam session holder" >&2
    return 1
  fi
  docker rm -f "$unknown_session_holder_id" >/dev/null
  if docker inspect "$unknown_session_holder_id" >/dev/null 2>&1; then
    echo "candidate upgrade E2E: exact unknown Steam session holder remained after removal" >&2
    return 1
  fi
  unknown_session_holder_id=""
}

read_steam_session_sentinel_hash() {
  docker rm -f "$session_seed_container" >/dev/null 2>&1 || true
  docker run --rm --name "$session_seed_container" --label "com.anxi-panel.test-owner=$owner" --volume "$steam_session_volume:/session" alpine:3.20 sha256sum /session/authorized-session.txt | awk '{print $1}'
}

seed_legacy_steam_session() {
  local sentinel_prefix="${1:-authorized}"
  if docker volume inspect "$steam_session_volume" >/dev/null 2>&1; then
    echo "candidate upgrade E2E: refusing to overwrite an existing Steam session volume" >&2
    return 1
  fi
  docker volume create --label "com.anxi-panel.test-owner=$owner" "$steam_session_volume" >/dev/null
  docker rm -f "$session_seed_container" >/dev/null 2>&1 || true
  docker run --rm --name "$session_seed_container" --label "com.anxi-panel.test-owner=$owner" --env "SESSION_SENTINEL=$sentinel_prefix-session-$suffix" --volume "$steam_session_volume:/session" alpine:3.20 sh -c 'umask 077; printf "%s\n" "$SESSION_SENTINEL" > /session/authorized-session.txt'
  read_steam_session_sentinel_hash
}

assert_authorized_legacy_runtime_preserved() {
  local instance_dir="$data_dir/instances/stardew"
  local instance_compose="$instance_dir/docker-compose.yml"

  if [[ -z "$authorized_auth_container_id" ]] || ! docker inspect "$authorized_auth_container_id" >/dev/null 2>&1 ||
    [[ "$(docker inspect "$authorized_auth_container_id" | jq -r '.[0].State.Running')" != true ]]; then
    echo "candidate upgrade E2E: authorized legacy steam-auth container was removed or restarted into a stopped state" >&2
    return 1
  fi
  if [[ "$(sha256sum "$instance_compose" | awk '{print $1}')" != "$(sha256sum "$legacy_expected_migrated_compose" | awk '{print $1}')" ]]; then
    echo "candidate upgrade E2E: authorized migration did not remove only the legacy server dependency" >&2
    return 1
  fi
  assert_candidate_auth_images_unchanged
}

assert_legacy_steam_invite_migration() {
  local expected="$1"
  local expected_session_hash="${2:-}"
  local instance_env="$data_dir/instances/stardew/.env"
  local api_enabled=""
  local api_auth_state=""
  local api_logged_in=""
  local invite_status=""

  for _ in $(seq 1 60); do
    if curl --silent --show-error --max-time 10 --cookie "$cookie_file" "http://127.0.0.1:$panel_port/api/instances/stardew/state" >"$response_file" 2>/dev/null; then
      api_enabled="$(jq -r '.steamInviteEnabled' "$response_file")"
      api_auth_state="$(jq -r '.steamInviteAuthState // empty' "$response_file")"
      api_logged_in="$(jq -r '.steamAuthLoggedIn' "$response_file")"
      if [[ "$api_enabled" == "$expected" ]]; then
        break
      fi
    fi
    sleep 1
  done
  if [[ "$api_enabled" != "$expected" ]]; then
    echo "candidate upgrade E2E: legacy Steam invite intent migrated to $api_enabled, want $expected" >&2
    cat "$response_file" >&2
    return 1
  fi
  if [[ "$(grep -c "^STEAM_INVITE_ENABLED=$expected$" "$instance_env")" != 1 ]]; then
    echo "candidate upgrade E2E: legacy Steam invite intent was not persisted exactly once as $expected" >&2
    return 1
  fi

  curl --silent --show-error --fail --cookie "$cookie_file" "http://127.0.0.1:$panel_port/api/instances/stardew/invite-code" >"$response_file"
  invite_status="$(jq -r '.status // empty' "$response_file")"
  if [[ "$expected" == false ]]; then
    if [[ "$api_auth_state" != disabled || "$api_logged_in" != false || "$invite_status" != disabled ||
      "$(jq -r '.steamInviteEnabled' "$response_file")" != false ]]; then
      echo "candidate upgrade E2E: no-Auth legacy fixture did not expose the disabled API contract" >&2
      cat "$response_file" >&2
      return 1
    fi
    assert_disabled_legacy_runtime_converged
    # GET invite-code must remain side-effect free while disabled.
    curl --silent --show-error --fail --cookie "$cookie_file" "http://127.0.0.1:$panel_port/api/instances/stardew/invite-code" >/dev/null
    assert_disabled_legacy_runtime_converged
    echo "candidate upgrade E2E: legacy running instance without Auth evidence migrated to explicit false without touching server or Auth images"
    return 0
  fi

  if [[ "$api_auth_state" != ready || "$api_logged_in" != true || "$invite_status" != server_stopped ||
    "$(jq -r '.steamInviteEnabled' "$response_file")" != true ]]; then
    echo "candidate upgrade E2E: authorized legacy fixture did not retain its ready API contract" >&2
    cat "$response_file" >&2
    return 1
  fi
  if [[ -z "$expected_session_hash" || "$(read_steam_session_sentinel_hash)" != "$expected_session_hash" ]]; then
    echo "candidate upgrade E2E: authorized legacy Steam session was not preserved" >&2
    return 1
  fi
  assert_authorized_legacy_runtime_preserved
  echo "candidate upgrade E2E: legacy STEAM_AUTH_COMPLETED intent stayed enabled and preserved its session"
}

restart_previous_release_with_authorized_legacy_fixture() {
  local instance_dir="$data_dir/instances/stardew"
  local instance_compose="$instance_dir/docker-compose.yml"

  docker compose --project-name "$project" --env-file "$env_file" -f "$compose_file" stop panel >/dev/null
  docker tag "$previous_fixture_ref" "$previous_ref"
  sed -i "s|^PANEL_IMAGE=.*$|PANEL_IMAGE=$previous_ref|" "$env_file"
  write_legacy_steam_invite_fixture authorized
  write_legacy_runtime_compose "$instance_compose" true
  write_legacy_runtime_compose "$legacy_expected_migrated_compose" false
  authorized_session_hash="$(seed_legacy_steam_session authorized)"
  docker compose --project-name stardew --project-directory "$instance_dir" -f "$instance_compose" up -d --no-deps steam-auth >/dev/null
  docker compose --project-name "$project" --env-file "$env_file" -f "$compose_file" up -d --no-deps --force-recreate panel >/dev/null
  wait_version "$previous_version" 120
  authorized_auth_container_id="$(docker ps -a --filter 'label=com.docker.compose.project=stardew' --filter 'label=com.docker.compose.service=steam-auth' --format '{{.ID}}')"
  if [[ -z "$authorized_auth_container_id" || "$(read_steam_session_sentinel_hash)" != "$authorized_session_hash" ]]; then
    echo "candidate upgrade E2E: previous release changed the seeded Steam session before upgrade" >&2
    return 1
  fi
  assert_candidate_auth_images_unchanged
}

echo "candidate upgrade E2E: testing unhealthy target rollback through public Web API"
post_update_check
run_dry_run
start_apply
wait_apply_phase failed_rolled_back 360
if [[ "$(jq -r '.errorCode // empty' "$response_file")" != health_check_failed ]]; then
  echo "candidate upgrade E2E: unhealthy target did not report health_check_failed" >&2
  cat "$response_file" >&2
  exit 1
fi
wait_version "$previous_version" 120

echo "candidate upgrade E2E: preparing a running legacy instance with SteamCMD evidence and the old Auth-coupled runtime"
prepare_legacy_disabled_runtime_fixture

echo "candidate upgrade E2E: replacing controlled target with the exact healthy candidate"
push_healthy_candidate
post_update_check
run_dry_run
start_apply
wait_version "$version" 300
wait_apply_phase succeeded 120
assert_upgraded_frontend_contract
assert_unknown_session_holder_fail_closed
remove_owned_unknown_session_holder
docker restart "$panel_container" >/dev/null
wait_version "$version" 120
wait_apply_phase succeeded 60
assert_legacy_steam_invite_migration false

if [[ ! -s "$data_dir/panel.db" || ! -f "$data_dir/release-candidate-sentinel.txt" ]]; then
  echo "candidate upgrade E2E: Panel data was not preserved" >&2
  exit 1
fi
if [[ "$(sqlite3 "$data_dir/panel.db" 'PRAGMA integrity_check;')" != ok ]]; then
  echo "candidate upgrade E2E: SQLite integrity check failed" >&2
  exit 1
fi
if [[ "$(curl --silent --show-error --cookie "$cookie_file" "http://127.0.0.1:$panel_port/api/setup/status" | jq -r '.initialized')" != true ]]; then
  echo "candidate upgrade E2E: initialized state was not preserved" >&2
  exit 1
fi
if [[ "$(docker inspect "$game_container" | jq -r '.[0].Id')" != "$game_id_before" ]]; then
  echo "candidate upgrade E2E: non-target game container was recreated" >&2
  exit 1
fi
if [[ "$(docker exec "$game_container" sha256sum /game/sentinel.txt | awk '{print $1}')" != "$game_hash_before" ]]; then
  echo "candidate upgrade E2E: non-target game volume changed" >&2
  exit 1
fi

docker restart "$panel_container" >/dev/null
wait_version "$version" 120
wait_apply_phase succeeded 60
assert_legacy_steam_invite_migration false
if [[ "$(docker inspect "$game_container" | jq -r '.[0].Id')" != "$game_id_before" ]]; then
  echo "candidate upgrade E2E: Panel restart changed the game container" >&2
  exit 1
fi

echo "candidate upgrade E2E: replaying the same previous release with durable SteamAuth evidence"
restart_previous_release_with_authorized_legacy_fixture
post_update_check
run_dry_run
start_apply
wait_version "$version" 300
wait_apply_phase succeeded 120
assert_legacy_steam_invite_migration true "$authorized_session_hash"
if [[ "$(docker inspect "$game_container" | jq -r '.[0].Id')" != "$game_id_before" ]] ||
  [[ "$(docker exec "$game_container" sha256sum /game/sentinel.txt | awk '{print $1}')" != "$game_hash_before" ]]; then
  echo "candidate upgrade E2E: legacy Steam invite replay changed the non-target game resource" >&2
  exit 1
fi
assert_upgraded_mod_update_check
assert_upgraded_legacy_junimo_repair
assert_upgraded_stopped_compose_save_import_submission
assert_upgraded_save_import_phase_a_boundaries

echo "candidate upgrade E2E: previous release Web upgrade, rollback, unknown-holder fail-closed recovery, Steam invite migration, persistence, restart, Mod checks, legacy runtime repair, and save-import safety boundaries passed"
