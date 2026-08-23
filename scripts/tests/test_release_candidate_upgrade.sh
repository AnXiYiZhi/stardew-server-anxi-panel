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
target_ref="ghcr.io/anxiyizhi/stardew-server-anxi-panel:$version"
previous_ref="ghcr.io/anxiyizhi/stardew-server-anxi-panel:$previous_version"
panel_port=18080
compose_ready=0

cleanup() {
  local cleanup_status=$?
  set +e
  if [[ -f "$data_dir/instances/stardew/docker-compose.yml" ]]; then
    docker compose --project-name stardew --project-directory "$data_dir/instances/stardew" -f "$data_dir/instances/stardew/docker-compose.yml" down --volumes --remove-orphans >/dev/null 2>&1
  fi
  if ((compose_ready == 1)); then
    docker compose --project-name "$project" --env-file "$env_file" -f "$compose_file" down --volumes --remove-orphans >/dev/null 2>&1
  fi
  docker rm -f "$release_container" "$registry_container" >/dev/null 2>&1
  docker network rm "$network" >/dev/null 2>&1
  if [[ "$root" == /tmp/anxi-release-candidate-* && -d "$root" ]]; then
    rm -rf -- "$root"
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

assert_upgraded_empty_compose_save_import_submission() {
  local instance_dir="$data_dir/instances/stardew"
  local instance_compose="$instance_dir/docker-compose.yml"
  local import_project="$project-save-import"
  local fixture_root="$root/save-import-fixture"
  local save_zip="$root/Imported_123.zip"
  local commit_body="$root/save-import-commit.json"
  local stop_code=""
  local instance_state=""
  local compose_ps_output=""
  local preview_code=""
  local upload_token=""
  local commit_code=""
  local import_job_id=""
  local import_operation_id=""
  local job_status=""
  local cleaned=0
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

  echo "candidate upgrade E2E: testing Panel Stop empty Compose save-import submission"
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
      if [[ "$instance_state" == stopped ]] &&
        [[ -z "$(docker ps -a --filter "label=com.docker.compose.project=$import_project" --quiet)" ]]; then
        break
      fi
    fi
    sleep 1
  done
  if [[ "$instance_state" != stopped ]] ||
    [[ -n "$(docker ps -a --filter "label=com.docker.compose.project=$import_project" --quiet)" ]]; then
    echo "candidate upgrade E2E: Panel Stop did not reach stopped with zero project containers" >&2
    exit 1
  fi
  compose_ps_output="$(docker exec --workdir /data/instances/stardew "$panel_container" docker compose ps --all --format json)"
  if [[ -n "$(printf '%s' "$compose_ps_output" | tr -d '[:space:]')" ]]; then
    echo "candidate upgrade E2E: compose ps after Panel Stop was not empty" >&2
    printf '%s\n' "$compose_ps_output" >&2
    exit 1
  fi

  # Make the asynchronous maintenance ComposeUp fail immediately after the
  # submission has crossed the empty strict-Compose gate. The source is
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
    echo "candidate upgrade E2E: empty Compose save import was not accepted with HTTP 202/jobId" >&2
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
    if [[ "$instance_state" == stopped ]] &&
      [[ -z "$(docker ps -a --filter "label=com.docker.compose.project=$import_project" --quiet)" ]] &&
      [[ -z "$(docker network ls --filter "label=com.docker.compose.project=$import_project" --quiet)" ]]; then
      cleaned=1
      break
    fi
    sleep 1
  done
  if ((cleaned != 1)); then
    echo "candidate upgrade E2E: controlled save-import fixture did not restore stopped state and clean Docker resources" >&2
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
  local select_code=""
  local preimport_count=""
  local raw_platform_id="76561190000000456"

  echo "candidate upgrade E2E: testing exact target visibility and FIFO no-effect recovery"
  # The preceding legacy-repair fixture intentionally leaves its exact stopped
  # stardew containers in place long enough to prove preservation. Its evidence
  # is complete now; clear only that isolated DinD project before replacing the
  # instance Compose definition with the import runtime.
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
    local terminal_wait_seconds=150
    if [[ "$mode" == invisible ]]; then
      # The product intentionally gives a newly starting Junimo runtime up to
      # five minutes to make the exact staged target visible before failing
      # closed. Observe that full contract plus rollback overhead; do not turn
      # a still-running readiness probe into a false terminal success.
      terminal_wait_seconds=420
    fi
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
      if [[ "$(sqlite3 -cmd '.timeout 5000' "$data_dir/panel.db" "SELECT state FROM instances WHERE id='stardew';")" == stopped ]] &&
        [[ -z "$(docker ps -a --filter "label=com.docker.compose.project=$import_project" --quiet)" ]] &&
        [[ -z "$(docker network ls --filter "label=com.docker.compose.project=$import_project" --quiet)" ]]; then
        return
      fi
      sleep 1
    done
    echo "candidate upgrade E2E: $mode case did not restore stopped state and remove Compose resources" >&2
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
  auth_container_id="$(docker compose --project-name stardew --project-directory "$instance_dir" -f "$instance_compose" ps --all --format json steam-auth | jq -r 'if type == "array" then .[0].ID else .ID end')"
  if [[ -z "$auth_container_id" || "$auth_container_id" == null ]]; then
    echo "candidate upgrade E2E: failed to create the preserved steam-auth fixture" >&2
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
  if [[ -n "$(docker compose --project-name stardew --project-directory "$instance_dir" -f "$instance_compose" ps --status running --quiet)" ]]; then
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

echo "candidate upgrade E2E: replacing controlled target with the exact healthy candidate"
push_healthy_candidate
post_update_check
run_dry_run
start_apply
wait_version "$version" 300
wait_apply_phase succeeded 120
assert_upgraded_frontend_contract

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
if [[ "$(docker inspect "$game_container" | jq -r '.[0].Id')" != "$game_id_before" ]]; then
  echo "candidate upgrade E2E: Panel restart changed the game container" >&2
  exit 1
fi
assert_upgraded_mod_update_check
assert_upgraded_legacy_junimo_repair
assert_upgraded_empty_compose_save_import_submission
assert_upgraded_save_import_phase_a_boundaries

echo "candidate upgrade E2E: previous release Web upgrade, rollback, persistence, restart, Mod checks, legacy runtime repair, and save-import safety boundaries passed"
