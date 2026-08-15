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
    ssl_certificate /certs/releases.crt;
    ssl_certificate_key /certs/releases.key;
    default_type application/json;
    location / {
      return 200 '[{"tag_name":"v$version","html_url":"https://example.invalid/releases/v$version","draft":false,"prerelease":false,"published_at":"2026-01-01T00:00:00Z"}]';
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
  local control_asset=""
  local mobile_control_asset=""
  local saves_asset=""
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

  for prefix in ServerControlPage MobileControlPage SavesPage; do
    mapfile -t matches < <(grep -oE "(^|/)$prefix-[A-Za-z0-9_-]+\.js" "$output_dir/entry.js" | sort -u)
    if [[ "${#matches[@]}" -ne 1 ]]; then
      echo "candidate upgrade E2E: expected exactly one $prefix frontend chunk" >&2
      exit 1
    fi
    asset="/assets/${matches[0]#/}"
    curl --silent --show-error --fail "http://127.0.0.1:$panel_port$asset" >"$output_dir/$prefix.js"
    case "$prefix" in
      ServerControlPage) control_asset="$output_dir/$prefix.js" ;;
      MobileControlPage) mobile_control_asset="$output_dir/$prefix.js" ;;
      SavesPage) saves_asset="$output_dir/$prefix.js" ;;
    esac
  done

  for asset in "$control_asset" "$mobile_control_asset"; do
    if ! grep -Eq 'value:.FarmhouseStack.,hidden:!0,children:.FarmhouseStack（兼容已有配置）.' "$asset"; then
      echo "candidate upgrade E2E: upgraded frontend exposes FarmhouseStack or lost legacy-value compatibility" >&2
      exit 1
    fi
  done
  if ! grep -Eq 'kind===.auto.\?.游戏日回档.' "$saves_asset" ||
    ! grep -Eq 'farmerName\?.农民：' "$saves_asset" ||
    ! grep -Eq 'farmType\?.地图：' "$saves_asset"; then
    echo "candidate upgrade E2E: upgraded frontend lost game-day rollback hover details" >&2
    exit 1
  fi
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

  echo "candidate upgrade E2E: testing Panel Stop empty Compose save-import submission"
  mkdir -p "$instance_dir/.local-container/mods/JunimoServer"
  mkdir -p "$instance_dir/.local-container/saves/Saves/Existing_1"
  mkdir -p "$instance_dir/.local-container/saves/.smapi/mod-data/junimohost.server"
  printf 'IMAGE_VERSION=1.5.0-preview.125\nAPI_PORT=5110\n' >"$instance_dir/.env"
  printf '%s\n' '{"Name":"JunimoServer","Version":"1.5.0-preview.125","UniqueID":"JunimoHost.Server"}' >"$instance_dir/.local-container/mods/JunimoServer/manifest.json"
  printf 'release-candidate-fixture-dll\n' >"$instance_dir/.local-container/mods/JunimoServer/JunimoServer.dll"
  printf '%s\n' '<SaveGame><player><name>Existing</name></player></SaveGame>' >"$instance_dir/.local-container/saves/Saves/Existing_1/Existing_1"
  printf '%s\n' '<Farmer><name>Existing</name></Farmer>' >"$instance_dir/.local-container/saves/Saves/Existing_1/SaveGameInfo"
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

  # Make the asynchronous maintenance fixture fail immediately after the
  # submission has crossed the empty strict-Compose gate. This keeps the E2E
  # deterministic while still proving 202/job ownership on the upgraded Panel.
  printf 'name: %s\nservices:\n  server:\n    image: alpine:3.20\n    command: ["sh", "-c", "exit 1"]\n' "$import_project" >"$instance_compose"
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
  echo "candidate upgrade E2E: upgraded Panel accepted empty Compose save import and cleaned the controlled failure"
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
assert_upgraded_empty_compose_save_import_submission

echo "candidate upgrade E2E: previous release Web upgrade, rollback, persistence and restart passed"
