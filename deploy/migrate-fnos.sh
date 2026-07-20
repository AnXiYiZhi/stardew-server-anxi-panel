#!/usr/bin/env bash

set -Eeuo pipefail

# One-time migration for legacy fnOS/NAS Panel containers which cannot use the
# built-in updater because their Docker Compose labels are incomplete. This
# script never deletes Panel data, game containers, volumes, saves, or mods.

SCRIPT_REVISION="1"
GITHUB_LATEST_API="https://api.github.com/repos/anxiyizhi/stardew-server-anxi-panel/releases/latest"
GITHUB_LATEST_API_CN="https://gh-proxy.com/https://api.github.com/repos/anxiyizhi/stardew-server-anxi-panel/releases/latest"
OCI_TITLE="stardew-server-anxi-panel"
DEFAULT_PROJECT="anxi-panel-managed"

green() { printf '\033[0;32m%s\033[0m\n' "$*"; }
yellow() { printf '\033[0;33m%s\033[0m\n' "$*"; }
red() { printf '\033[0;31m%s\033[0m\n' "$*" >&2; }

die() {
  red "错误：$*"
  exit 1
}

normalize_version() {
  local value="${1#v}"
  [[ "$value" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]] || return 1
  printf '%s\n' "$value"
}

version_gt() {
  local left right highest
  left="$(normalize_version "$1")" || return 1
  right="$(normalize_version "$2")" || return 1
  [[ "$left" != "$right" ]] || return 1
  highest="$(printf '%s\n%s\n' "$left" "$right" | sort -V | tail -n 1)"
  [[ "$highest" == "$left" ]]
}

trusted_panel_image() {
  local image="${1%@*}" repo="${1%@*}"
  repo="${repo%:*}"
  case "$repo" in
    anxiyizhi/stardew-server-anxi-panel|\
    ghcr.io/anxiyizhi/stardew-server-anxi-panel|\
    crpi-9z3bkb9g7fxeohrg.cn-hangzhou.personal.cr.aliyuncs.com/anxi-panel/stardew-server-anxi-panel|\
    docker.1ms.run/anxiyizhi/stardew-server-anxi-panel|\
    docker.m.daocloud.io/anxiyizhi/stardew-server-anxi-panel) return 0 ;;
    *) return 1 ;;
  esac
}

yaml_quote() {
  local value="$1"
  [[ "$value" != *$'\n'* && "$value" != *$'\r'* ]] || return 1
  value="${value//\\/\\\\}"
  value="${value//\"/\\\"}"
  value="${value//\$/\$\$}"
  printf '"%s"' "$value"
}

highest_candidate_version() {
  local highest_version="" candidate version
  for candidate in "$@"; do
    IFS='|' read -r _name version _rest <<< "$candidate"
    normalize_version "$version" >/dev/null 2>&1 || return 1
    if [[ -z "$highest_version" ]] || version_gt "$version" "$highest_version"; then
      highest_version="$version"
    fi
  done
  [[ -n "$highest_version" ]] || return 1
  printf '%s\n' "$highest_version"
}

managed_project_name() {
  local suffix
  suffix="$(printf '%s' "$1" | tr '[:upper:]' '[:lower:]' | tr -cd 'a-z0-9_-')"
  suffix="${suffix:0:48}"
  [[ -n "$suffix" ]] || return 1
  printf 'anxi-panel-%s\n' "$suffix"
}

if [[ "${ANXI_MIGRATE_LIBRARY:-}" == "1" ]]; then
  if [[ "${BASH_SOURCE[0]}" != "$0" ]]; then
    return 0
  fi
  exit 0
fi

echo "=============================================="
echo " Anxi Panel 飞牛/NAS 标准 Compose 一键迁移"
echo " 脚本修订：$SCRIPT_REVISION"
echo "=============================================="
echo

if docker info >/dev/null 2>&1; then
  DOCKER=(docker)
elif command -v sudo >/dev/null 2>&1 && sudo docker info >/dev/null 2>&1; then
  DOCKER=(sudo docker)
else
  die "当前账号无法访问 Docker。请执行 sudo -i 后重试。"
fi

"${DOCKER[@]}" compose version >/dev/null 2>&1 || die "缺少 Docker Compose plugin。"
command -v sort >/dev/null 2>&1 || die "缺少 sort 命令，无法安全比较版本。"
printf '0.0.1\n0.0.2\n' | sort -V >/dev/null 2>&1 || die "当前 sort 不支持语义版本排序（-V）。"
command -v curl >/dev/null 2>&1 || die "缺少 curl，无法查询版本和下载元数据。"

container_env() {
  local container="$1" key="$2"
  "${DOCKER[@]}" inspect -f '{{range .Config.Env}}{{println .}}{{end}}' "$container" 2>/dev/null |
    awk -F= -v key="$key" '$1 == key {sub(/^[^=]*=/, ""); print; exit}'
}

container_version() {
  local container="$1" version image tag
  version="$("${DOCKER[@]}" inspect -f '{{index .Config.Labels "org.opencontainers.image.version"}}' "$container" 2>/dev/null || true)"
  if normalize_version "$version" >/dev/null 2>&1; then
    normalize_version "$version"
    return
  fi
  version="$(container_env "$container" PANEL_VERSION)"
  if normalize_version "$version" >/dev/null 2>&1; then
    normalize_version "$version"
    return
  fi
  image="$("${DOCKER[@]}" inspect -f '{{.Config.Image}}' "$container" 2>/dev/null || true)"
  tag="${image##*:}"
  normalize_version "$tag"
}

container_identity_valid() {
  local container="$1" title image
  title="$("${DOCKER[@]}" inspect -f '{{index .Config.Labels "org.opencontainers.image.title"}}' "$container" 2>/dev/null || true)"
  [[ "$title" == "$OCI_TITLE" ]] && return 0
  image="$("${DOCKER[@]}" inspect -f '{{.Config.Image}}' "$container" 2>/dev/null || true)"
  trusted_panel_image "$image"
}

container_health_valid() {
  local container="$1" status health
  status="$("${DOCKER[@]}" inspect -f '{{.State.Status}}' "$container" 2>/dev/null || true)"
  health="$("${DOCKER[@]}" inspect -f '{{if .State.Health}}{{.State.Health.Status}}{{else}}none{{end}}' "$container" 2>/dev/null || true)"
  [[ "$status" == "running" && ( "$health" == "healthy" || "$health" == "none" ) ]]
}

container_data_mount() {
  local container="$1" container_data exact mounts type source
  container_data="$(container_env "$container" PANEL_DATA_DIR)"
  [[ -n "$container_data" ]] || container_data="/data"
  mounts="$("${DOCKER[@]}" inspect -f '{{range .Mounts}}{{printf "%s|%s|%s\n" .Destination .Type .Source}}{{end}}' "$container" 2>/dev/null)"
  exact="$(printf '%s\n' "$mounts" | awk -F'|' -v target="$container_data" '$1 == target {print $2 "|" $3; exit}')"
  if [[ -n "$exact" ]]; then
    printf '%s\n' "$exact"
    return
  fi
  local -a fallback=()
  while IFS='|' read -r _destination type source; do
    [[ "$type" == "bind" && -n "$source" ]] || continue
    [[ "$source" != *'|'* && "$source" != *$'\n'* && "$source" != *$'\r'* ]] || continue
    if [[ -f "$source/panel.db" || -d "$source/instances" ]]; then
      fallback+=("$type|$source")
    fi
  done <<< "$mounts"
  [[ ${#fallback[@]} -eq 1 ]] && printf '%s\n' "${fallback[0]}"
}

declare -a candidates=()
while IFS= read -r container_id; do
  [[ -n "$container_id" ]] || continue
  container_identity_valid "$container_id" || continue
  container_health_valid "$container_id" || continue
  version="$(container_version "$container_id" 2>/dev/null || true)"
  [[ -n "$version" ]] || continue
  name="$("${DOCKER[@]}" inspect -f '{{.Name}}' "$container_id" | sed 's#^/##')"
  data_mount="$(container_data_mount "$container_id")"
  data_type="${data_mount%%|*}"
  data_source="${data_mount#*|}"
  image="$("${DOCKER[@]}" inspect -f '{{.Config.Image}}' "$container_id")"
  created="$("${DOCKER[@]}" inspect -f '{{.Created}}' "$container_id")"
  candidates+=("$name|$version|$data_type|$data_source|$image|$created")
done < <("${DOCKER[@]}" ps -q)

if [[ ${#candidates[@]} -eq 0 ]]; then
  die "没有找到运行中、健康且版本可识别的 Anxi Panel 容器。"
fi

selected=""
if [[ -n "${PANEL_CONTAINER:-}" ]]; then
  for candidate in "${candidates[@]}"; do
    IFS='|' read -r name _rest <<< "$candidate"
    [[ "$name" == "$PANEL_CONTAINER" ]] && selected="$candidate"
  done
  [[ -n "$selected" ]] || die "PANEL_CONTAINER=$PANEL_CONTAINER 不是有效候选。"
else
  highest_version="$(highest_candidate_version "${candidates[@]}")" || die "候选 Panel 版本无法安全排序。"
  declare -a highest=()
  for candidate in "${candidates[@]}"; do
    IFS='|' read -r _name version _rest <<< "$candidate"
    [[ "$version" == "$highest_version" ]] && highest+=("$candidate")
  done
  if [[ ${#highest[@]} -gt 1 ]]; then
    first_mount="$(printf '%s' "${highest[0]}" | cut -d'|' -f4)"
    for candidate in "${highest[@]:1}"; do
      mount="$(printf '%s' "$candidate" | cut -d'|' -f4)"
      if [[ "$mount" != "$first_mount" ]]; then
        red "发现多个最高版本 Panel，且数据目录不同；为防止迁错，未执行任何修改："
        printf '  - %s\n' "${highest[@]}" >&2
        die "请使用 PANEL_CONTAINER=容器名 sudo -E bash migrate-fnos.sh 明确选择。"
      fi
    done
    selected="$(printf '%s\n' "${highest[@]}" | sort -t'|' -k5,5 | tail -n 1)"
  else
    selected="${highest[0]}"
  fi
fi

IFS='|' read -r panel_container panel_version data_type host_data_dir panel_image _created <<< "$selected"
DEFAULT_PROJECT="$(managed_project_name "$panel_container")" || die "无法从旧容器名称生成安全的 Compose 项目名。"
if [[ -n "${EXPECTED_ORIGINAL_IMAGE:-}" ]]; then
  [[ "$EXPECTED_ORIGINAL_IMAGE" != *$'\n'* && "$EXPECTED_ORIGINAL_IMAGE" != *$'\r'* && "$panel_image" == "$EXPECTED_ORIGINAL_IMAGE" ]] || die "旧容器镜像引用已在确认后变化，拒绝迁移。"
fi
if [[ -n "${EXPECTED_ORIGINAL_DIGEST:-}" ]]; then
  [[ "$EXPECTED_ORIGINAL_DIGEST" =~ ^sha256:[0-9a-f]{64}$ ]] || die "预期旧镜像 digest 格式无效。"
  actual_original_digest="$("${DOCKER[@]}" inspect -f '{{.Image}}' "$panel_container")"
  [[ "$actual_original_digest" == "$EXPECTED_ORIGINAL_DIGEST" ]] || die "旧容器镜像 digest 已在确认后变化，拒绝迁移。"
fi
[[ "$data_type" == "bind" ]] || die "Panel 数据目录不是 bind mount（当前类型：${data_type:-未知}），脚本拒绝自动迁移。"
[[ "$host_data_dir" == /* && "$host_data_dir" != "/" ]] || die "Panel 宿主数据路径不安全：$host_data_dir"
[[ "$host_data_dir" != *'|'* && "$host_data_dir" != *$'\n'* && "$host_data_dir" != *$'\r'* ]] || die "Panel 宿主数据路径包含不支持的分隔字符。"
[[ -d "$host_data_dir" ]] || die "Panel 数据目录不存在：$host_data_dir"

container_data_dir="$(container_env "$panel_container" PANEL_DATA_DIR)"
[[ -n "$container_data_dir" ]] || container_data_dir="/data"
[[ "$container_data_dir" == /* && "$container_data_dir" != "/" ]] || die "容器内 PANEL_DATA_DIR 不安全：$container_data_dir"

socket_source="$("${DOCKER[@]}" inspect -f '{{range .Mounts}}{{if eq .Destination "/var/run/docker.sock"}}{{printf "%s|%s" .Type .Source}}{{end}}{{end}}' "$panel_container")"
[[ "$socket_source" == "bind|/var/run/docker.sock" ]] || die "当前容器缺少标准 Docker Socket bind mount。"

mount_count="$("${DOCKER[@]}" inspect -f '{{len .Mounts}}' "$panel_container")"
[[ "$mount_count" == "2" ]] || die "当前容器包含除数据目录和 Docker Socket 外的额外挂载；为避免丢失自定义配置，脚本拒绝猜测。"

privileged="$("${DOCKER[@]}" inspect -f '{{.HostConfig.Privileged}}' "$panel_container")"
[[ "$privileged" == "false" ]] || die "当前 Panel 使用 privileged 模式，脚本不自动迁移特殊部署。"
container_user="$("${DOCKER[@]}" inspect -f '{{.Config.User}}' "$panel_container")"
[[ -z "$container_user" ]] || die "当前 Panel 使用自定义 user=$container_user，脚本不自动迁移。"
network_mode="$("${DOCKER[@]}" inspect -f '{{.HostConfig.NetworkMode}}' "$panel_container")"
declare -a external_networks=()
if [[ "$network_mode" != "default" && "$network_mode" != "bridge" ]]; then
  case "$network_mode" in host|none|container:*) die "当前 Panel 使用不支持的网络模式 $network_mode。" ;; esac
  # Docker Go template, not shell variables.
  # shellcheck disable=SC2016
  mapfile -t external_networks < <(
    "${DOCKER[@]}" inspect -f '{{range $name, $_ := .NetworkSettings.Networks}}{{printf "%s\n" $name}}{{end}}' "$panel_container" |
      awk 'NF'
  )
  [[ ${#external_networks[@]} -ge 1 ]] || die "无法枚举当前 Panel 的自定义网络。"
  network_mode_found=0
  for network_name in "${external_networks[@]}"; do
    [[ "$network_name" =~ ^[A-Za-z0-9][A-Za-z0-9_.-]*$ ]] || die "自定义网络名称不安全：$network_name"
    [[ "$network_name" == "$network_mode" ]] && network_mode_found=1
  done
  [[ "$network_mode_found" == "1" ]] || die "容器网络模式与实际连接网络不一致，拒绝猜测。"
fi

# Docker Go template, not a shell expression.
# shellcheck disable=SC2016
mapfile -t port_bindings < <(
  "${DOCKER[@]}" inspect -f '{{range $p := (index .NetworkSettings.Ports "8090/tcp")}}{{printf "%s|%s\n" $p.HostIp $p.HostPort}}{{end}}' "$panel_container" |
    awk 'NF'
)
[[ ${#port_bindings[@]} -ge 1 ]] || die "8090/tcp 不存在宿主端口映射。"
IFS='|' read -r host_ip host_port <<< "${port_bindings[0]}"
[[ "$host_port" =~ ^[0-9]+$ && "$host_port" -ge 1 && "$host_port" -le 65535 ]] || die "宿主端口不合法：$host_port"
for binding in "${port_bindings[@]:1}"; do
  IFS='|' read -r binding_ip binding_port <<< "$binding"
  [[ "$binding_port" == "$host_port" ]] || die "8090/tcp 同时映射到多个宿主端口，拒绝猜测。"
  if [[ "$host_ip" == "0.0.0.0" && "$binding_ip" == "::" ]] || [[ "$host_ip" == "::" && "$binding_ip" == "0.0.0.0" ]]; then
    host_ip="0.0.0.0"
  elif [[ "$binding_ip" != "$host_ip" ]]; then
    die "8090/tcp 同时绑定多个不同宿主地址，拒绝猜测。"
  fi
done

if [[ -z "${TARGET_VERSION:-}" ]]; then
  yellow "正在查询最新稳定版本；直连 GitHub 失败时自动切换国内加速。"
  for release_api in "$GITHUB_LATEST_API" "$GITHUB_LATEST_API_CN"; do
    latest_json="$(curl -fsSL --connect-timeout 10 --max-time 30 "$release_api" 2>/dev/null || true)"
    TARGET_VERSION="$(printf '%s' "$latest_json" | sed -n 's/.*"tag_name"[[:space:]]*:[[:space:]]*"v\?\([0-9][0-9.]*\)".*/\1/p' | head -n 1)"
    normalize_version "${TARGET_VERSION:-}" >/dev/null 2>&1 && break
  done
fi
TARGET_VERSION="$(normalize_version "${TARGET_VERSION:-}")" || die "无法确定合法目标版本；请使用 TARGET_VERSION=x.y.z 重试。"
if version_gt "$panel_version" "$TARGET_VERSION"; then
  die "目标版本 $TARGET_VERSION 低于当前最高有效版本 $panel_version，拒绝降级。"
fi

image_candidates=(
  "crpi-9z3bkb9g7fxeohrg.cn-hangzhou.personal.cr.aliyuncs.com/anxi-panel/stardew-server-anxi-panel:$TARGET_VERSION"
  "docker.1ms.run/anxiyizhi/stardew-server-anxi-panel:$TARGET_VERSION"
  "docker.m.daocloud.io/anxiyizhi/stardew-server-anxi-panel:$TARGET_VERSION"
  "ghcr.io/anxiyizhi/stardew-server-anxi-panel:$TARGET_VERSION"
  "anxiyizhi/stardew-server-anxi-panel:$TARGET_VERSION"
)

echo "已自动选择最高版本的有效 Panel："
echo "  容器：$panel_container"
echo "  当前版本：$panel_version"
echo "  当前镜像：$panel_image"
echo "  数据目录：$host_data_dir"
echo "  访问端口：$host_port"
echo "  目标版本：$TARGET_VERSION"
echo

if [[ "${YES:-0}" != "1" ]]; then
  printf '输入 MIGRATE 确认迁移（不会删除 Panel 数据或游戏容器）：'
  read -r confirmation
  [[ "$confirmation" == "MIGRATE" ]] || die "用户取消。"
fi

selected_image=""
for candidate_image in "${image_candidates[@]}"; do
  echo "尝试拉取：$candidate_image"
  if ! "${DOCKER[@]}" pull "$candidate_image"; then
    yellow "拉取失败，继续下一个国内/官方候选。"
    continue
  fi
  image_title="$("${DOCKER[@]}" image inspect -f '{{index .Config.Labels "org.opencontainers.image.title"}}' "$candidate_image" 2>/dev/null || true)"
  image_version="$("${DOCKER[@]}" image inspect -f '{{index .Config.Labels "org.opencontainers.image.version"}}' "$candidate_image" 2>/dev/null || true)"
  image_version="$(normalize_version "$image_version" 2>/dev/null || true)"
  if [[ "$image_title" == "$OCI_TITLE" && "$image_version" == "$TARGET_VERSION" ]]; then
    selected_image="$candidate_image"
    break
  fi
  yellow "镜像 OCI 身份或版本不匹配，拒绝使用：$candidate_image"
done
[[ -n "$selected_image" ]] || die "所有可信镜像源均拉取失败或未通过 OCI 版本校验；当前容器未改变。"

parent_dir="$(dirname "$host_data_dir")"
if [[ "$(basename "$host_data_dir")" == "data" ]]; then
  install_dir="$parent_dir"
else
  safe_name="$(printf '%s' "$panel_container" | tr -cd 'A-Za-z0-9_.-')"
  install_dir="$parent_dir/.anxi-panel-$safe_name"
fi
[[ "$install_dir" == /* && "$install_dir" != "/" ]] || die "推导出的安装目录不安全：$install_dir"
mkdir -p "$install_dir"

timestamp="$(date -u +%Y%m%dT%H%M%SZ)"
backup_dir="$host_data_dir/updater/fnos-migration/$timestamp"
mkdir -p "$backup_dir"
chmod 700 "$host_data_dir/updater" "$host_data_dir/updater/fnos-migration" "$backup_dir" 2>/dev/null || true
"${DOCKER[@]}" inspect "$panel_container" > "$backup_dir/container-inspect.json"
chmod 600 "$backup_dir/container-inspect.json"
"${DOCKER[@]}" inspect -f '{{.Image}}' "$panel_container" > "$backup_dir/original-image-digest.txt"
"${DOCKER[@]}" inspect -f '{{json .Config.Env}}' "$panel_container" > "$backup_dir/environment.json"
chmod 600 "$backup_dir/original-image-digest.txt" "$backup_dir/environment.json"

compose_file="$install_dir/docker-compose.yml"
env_file="$install_dir/.env"
compose_staging="$install_dir/.docker-compose.migrate-$timestamp.yml"
env_staging="$install_dir/.env.migrate-$timestamp"
[[ ! -e "$compose_file" ]] || cp -p "$compose_file" "$backup_dir/docker-compose.yml.before"
[[ ! -e "$env_file" ]] || cp -p "$env_file" "$backup_dir/deployment.env.before"
compose_existed=0; env_existed=0
[[ -e "$compose_file" ]] && compose_existed=1
[[ -e "$env_file" ]] && env_existed=1

panel_secret="$(container_env "$panel_container" PANEL_SECRET)"
[[ -n "$panel_secret" ]] || die "当前容器缺少 PANEL_SECRET；为避免登录会话和安全配置变化，拒绝迁移。"
panel_mode="$(container_env "$panel_container" PANEL_MODE)"; [[ -n "$panel_mode" ]] || panel_mode="single"
panel_addr="$(container_env "$panel_container" PANEL_ADDR)"; [[ -n "$panel_addr" ]] || panel_addr=":8090"
original_restart_policy="$("${DOCKER[@]}" inspect -f '{{.HostConfig.RestartPolicy.Name}}' "$panel_container")"
case "$original_restart_policy" in always|unless-stopped|on-failure|no|'') ;; *) die "不支持的重启策略：$original_restart_policy" ;; esac
restart_policy="$original_restart_policy"
[[ -n "$restart_policy" && "$restart_policy" != "no" ]] || restart_policy="unless-stopped"

cat > "$env_staging" <<EOF
PANEL_IMAGE=$selected_image
EOF
chmod 600 "$env_staging"

{
  echo 'services:'
  echo '  panel:'
  # Compose interpolation must remain literal.
  # shellcheck disable=SC2016
  echo '    image: ${PANEL_IMAGE}'
  printf '    container_name: %s\n' "$(yaml_quote "$panel_container")"
  printf '    restart: %s\n' "$(yaml_quote "$restart_policy")"
  echo '    ports:'
  echo '      - target: 8090'
  printf '        published: %s\n' "$(yaml_quote "$host_port")"
  [[ -z "$host_ip" || "$host_ip" == "0.0.0.0" ]] || printf '        host_ip: %s\n' "$(yaml_quote "$host_ip")"
  echo '        protocol: tcp'
  echo '    volumes:'
  echo '      - type: bind'
  echo '        source: /var/run/docker.sock'
  echo '        target: /var/run/docker.sock'
  echo '      - type: bind'
  printf '        source: %s\n' "$(yaml_quote "$host_data_dir")"
  printf '        target: %s\n' "$(yaml_quote "$container_data_dir")"
  echo '    environment:'
  printf '      PANEL_ADDR: %s\n' "$(yaml_quote "$panel_addr")"
  printf '      PANEL_DATA_DIR: %s\n' "$(yaml_quote "$container_data_dir")"
  printf '      PANEL_HOST_DATA_DIR: %s\n' "$(yaml_quote "$host_data_dir")"
  printf '      PANEL_HOST_INSTALL_DIR: %s\n' "$(yaml_quote "$install_dir")"
  printf '      PANEL_HOST_COMPOSE_FILE: %s\n' "$(yaml_quote "$compose_file")"
  printf '      PANEL_COMPOSE_PROJECT: %s\n' "$(yaml_quote "$DEFAULT_PROJECT")"
  printf '      PANEL_SECRET: %s\n' "$(yaml_quote "$panel_secret")"
  printf '      PANEL_VERSION: %s\n' "$(yaml_quote "$TARGET_VERSION")"
  printf '      PANEL_MODE: %s\n' "$(yaml_quote "$panel_mode")"
  for optional_key in PANEL_DB_PATH DEFAULT_INSTANCE_ID CONTROL_COMMAND_RETENTION_DAYS CONTROL_COMMAND_RETENTION_COUNT ENABLE_MODDED_FARM_CREATION TZ; do
    optional_value="$(container_env "$panel_container" "$optional_key")"
    [[ -n "$optional_value" ]] && printf '      %s: %s\n' "$optional_key" "$(yaml_quote "$optional_value")"
  done
  if [[ ${#external_networks[@]} -gt 0 ]]; then
    echo '    networks:'
    for network_index in "${!external_networks[@]}"; do
      printf '      - legacy_network_%s\n' "$network_index"
    done
    echo 'networks:'
    for network_index in "${!external_networks[@]}"; do
      printf '  legacy_network_%s:\n' "$network_index"
      echo '    external: true'
      printf '    name: %s\n' "$(yaml_quote "${external_networks[$network_index]}")"
    done
  fi
} > "$compose_staging"
chmod 600 "$compose_staging"

"${DOCKER[@]}" compose --project-name "$DEFAULT_PROJECT" --env-file "$env_staging" -f "$compose_staging" config --quiet || die "生成的 Compose 未通过校验；当前容器未改变，备份位于 $backup_dir。"

mv -f "$compose_staging" "$compose_file"
mv -f "$env_staging" "$env_file"

legacy_name="${panel_container}-legacy-${timestamp,,}"
cutover_started=0
files_replaced=1
restore_deployment_files() {
  if [[ "$compose_existed" == "1" ]]; then
    cp -p "$backup_dir/docker-compose.yml.before" "$compose_file"
  else
    rm -f "$compose_file"
  fi
  if [[ "$env_existed" == "1" ]]; then
    cp -p "$backup_dir/deployment.env.before" "$env_file"
  else
    rm -f "$env_file"
  fi
  files_replaced=0
}
rollback() {
  local reason="$1"
  trap - ERR INT TERM
  red "$reason"
  if [[ "$cutover_started" == "1" ]]; then
		"${DOCKER[@]}" rm -f "$panel_container" >/dev/null 2>&1 || true
		if "${DOCKER[@]}" inspect "$legacy_name" >/dev/null 2>&1; then
			if [[ -n "${MIGRATION_DATABASE_BACKUP:-}" && -n "${MIGRATION_DATABASE_RELATIVE:-}" ]]; then
				[[ "$MIGRATION_DATABASE_BACKUP" == /data/updater/backups/*/panel.db ]] || die "数据库备份路径不安全。"
				[[ "$MIGRATION_DATABASE_RELATIVE" != /* && "$MIGRATION_DATABASE_RELATIVE" != *..* && "$MIGRATION_DATABASE_RELATIVE" != *$'\n'* ]] || die "数据库相对路径不安全。"
				cp -p "$MIGRATION_DATABASE_BACKUP" "$host_data_dir/$MIGRATION_DATABASE_RELATIVE" || die "恢复旧 Panel 数据库失败。"
				rm -f "$host_data_dir/$MIGRATION_DATABASE_RELATIVE-wal" "$host_data_dir/$MIGRATION_DATABASE_RELATIVE-shm"
			fi
			"${DOCKER[@]}" rename "$legacy_name" "$panel_container" >/dev/null 2>&1 || true
      "${DOCKER[@]}" update --restart="${original_restart_policy:-no}" "$panel_container" >/dev/null 2>&1 || true
      "${DOCKER[@]}" start "$panel_container" >/dev/null 2>&1 || true
    fi
  fi
  [[ "$files_replaced" == "0" ]] || restore_deployment_files
  red "已尝试恢复旧 Panel。备份目录：$backup_dir"
  exit 1
}
trap 'rollback "迁移被异常中断。"' ERR INT TERM

"${DOCKER[@]}" rename "$panel_container" "$legacy_name" || die "无法保留旧容器，当前服务未改变。"
cutover_started=1
"${DOCKER[@]}" stop -t 45 "$legacy_name" >/dev/null || rollback "旧 Panel 未能安全停止。"
"${DOCKER[@]}" update --restart=no "$legacy_name" >/dev/null || rollback "无法关闭旧容器的自动重启策略。"

if ! "${DOCKER[@]}" compose --project-name "$DEFAULT_PROJECT" --env-file "$env_file" -f "$compose_file" up -d --pull never --no-deps panel; then
  rollback "新版 Panel 创建失败。"
fi

health_timeout="${MIGRATION_HEALTH_TIMEOUT_SECONDS:-180}"
[[ "$health_timeout" =~ ^[0-9]+$ && "$health_timeout" -ge 30 && "$health_timeout" -le 600 ]] || rollback "MIGRATION_HEALTH_TIMEOUT_SECONDS 必须在 30..600 秒之间。"
health_attempts=$((health_timeout / 2))
healthy=0
for ((_attempt = 1; _attempt <= health_attempts; _attempt++)); do
  state="$("${DOCKER[@]}" inspect -f '{{.State.Status}}' "$panel_container" 2>/dev/null || true)"
  health="$("${DOCKER[@]}" inspect -f '{{if .State.Health}}{{.State.Health.Status}}{{else}}none{{end}}' "$panel_container" 2>/dev/null || true)"
  if [[ "$state" == "running" && ( "$health" == "healthy" || "$health" == "none" ) ]]; then
    version_json="$("${DOCKER[@]}" exec "$panel_container" wget -qO- --timeout=5 http://127.0.0.1:8090/api/version 2>/dev/null || true)"
    if printf '%s' "$version_json" | grep -Eq '"version"[[:space:]]*:[[:space:]]*"v?'"$TARGET_VERSION"'"'; then
      healthy=1
      break
    fi
  fi
  sleep 2
done
[[ "$healthy" == "1" ]] || rollback "新版 Panel 未在 ${health_timeout} 秒内通过容器健康和精确版本验收。"

new_labels="$("${DOCKER[@]}" inspect -f '{{index .Config.Labels "com.docker.compose.project"}}|{{index .Config.Labels "com.docker.compose.service"}}|{{index .Config.Labels "com.docker.compose.project.config_files"}}' "$panel_container")"
IFS='|' read -r label_project label_service label_config <<< "$new_labels"
[[ "$label_project" == "$DEFAULT_PROJECT" && "$label_service" == "panel" && -n "$label_config" ]] || rollback "新版容器仍缺少标准 Compose labels。"
trap - ERR INT TERM

cat > "$backup_dir/result.txt" <<EOF
result=succeeded
old_container=$legacy_name
new_container=$panel_container
from_version=$panel_version
to_version=$TARGET_VERSION
image=$selected_image
compose_file=$compose_file
data_dir=$host_data_dir
EOF
chmod 600 "$backup_dir/result.txt"

green "迁移成功：Panel $panel_version -> $TARGET_VERSION"
green "标准 Compose：$compose_file"
green "旧容器已停止并保留：$legacy_name"
green "备份目录：$backup_dir"
echo
yellow "旧容器仅用于回退，已关闭自动重启。请勿再从飞牛旧项目启动或更新它；确认新版稳定后再人工删除旧容器，但不要删除数据目录或仍被新版使用的 external 网络。"
yellow "下一步：登录新版 Panel，在[运行组件升级]中执行可用更新；若仅 Control 版本不匹配，也必须通过 Panel 完成受控游戏重启，不能只在飞牛里重启容器。"
