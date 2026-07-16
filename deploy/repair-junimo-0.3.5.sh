#!/usr/bin/env bash

set -Eeuo pipefail

# Repairs trusted legacy Junimo image candidates, then lets Panel 0.3.5 run
# its built-in required update. Never removes volumes, saves, mods, or sessions.

EXPECTED_PANEL_VERSION="0.3.5"
TARGET_SERVER_TAG="1.5.0-preview.125"
TARGET_AUTH_TAG="1.5.0-anxi.2"
SCRIPT_REVISION="4"

green() { printf '\033[0;32m%s\033[0m\n' "$*"; }
yellow() { printf '\033[0;33m%s\033[0m\n' "$*"; }
red() { printf '\033[0;31m%s\033[0m\n' "$*" >&2; }

echo "========================================"
echo " Anxi Panel 0.3.5 Junimo 一键修复"
echo " 脚本修订：r$SCRIPT_REVISION"
echo "========================================"
echo

if docker info >/dev/null 2>&1; then
  DOCKER=(docker)
elif command -v sudo >/dev/null 2>&1 && sudo docker info >/dev/null 2>&1; then
  DOCKER=(sudo docker)
else
  red "错误：当前账号无法使用 Docker。请先执行 sudo -i，再重新运行本脚本。"
  exit 1
fi

# Only accept one running Panel whose immutable OCI label is exactly 0.3.5.
mapfile -t panel_matches < <(
  "${DOCKER[@]}" ps -q | while read -r container_id; do
    [[ -n "$container_id" ]] || continue
    title="$("${DOCKER[@]}" inspect -f '{{index .Config.Labels "org.opencontainers.image.title"}}' "$container_id" 2>/dev/null || true)"
    version="$("${DOCKER[@]}" inspect -f '{{index .Config.Labels "org.opencontainers.image.version"}}' "$container_id" 2>/dev/null || true)"
    if [[ "$title" == "stardew-server-anxi-panel" ]] && [[ "$version" == "$EXPECTED_PANEL_VERSION" || "$version" == "v$EXPECTED_PANEL_VERSION" ]]; then
      "${DOCKER[@]}" inspect -f '{{.Name}}' "$container_id" | sed 's#^/##'
    fi
  done
)

if [[ ${#panel_matches[@]} -eq 0 ]]; then
  red "错误：没有找到正在运行且内置版本为 0.3.5 的 Panel。"
  echo "当前检测到的 Panel 容器："
  "${DOCKER[@]}" ps -a -q | while read -r container_id; do
    title="$("${DOCKER[@]}" inspect -f '{{index .Config.Labels "org.opencontainers.image.title"}}' "$container_id" 2>/dev/null || true)"
    [[ "$title" == "stardew-server-anxi-panel" ]] || continue
    "${DOCKER[@]}" inspect -f '容器={{.Name}} 镜像={{.Config.Image}} 版本={{index .Config.Labels "org.opencontainers.image.version"}} 状态={{.State.Status}}' "$container_id" | sed 's#容器=/#容器=#'
  done
  exit 1
fi
if [[ ${#panel_matches[@]} -gt 1 ]]; then
  red "错误：找到多个正在运行的 0.3.5 Panel，为防止修错实例，脚本没有修改任何内容："
  printf '  - %s\n' "${panel_matches[@]}"
  exit 1
fi

panel_container="${panel_matches[0]}"
panel_image="$("${DOCKER[@]}" inspect -f '{{.Config.Image}}' "$panel_container")"
panel_version="$("${DOCKER[@]}" inspect -f '{{index .Config.Labels "org.opencontainers.image.version"}}' "$panel_container")"
echo "已定位目标 Panel："
echo "  容器：$panel_container"
echo "  镜像：$panel_image"
echo "  版本：$panel_version"
echo

container_data_dir="$("${DOCKER[@]}" inspect -f '{{range .Config.Env}}{{println .}}{{end}}' "$panel_container" | awk -F= '$1 == "PANEL_DATA_DIR" {sub(/^[^=]*=/, ""); print; exit}')"
[[ -n "$container_data_dir" ]] || container_data_dir="/data"
mount_lines="$("${DOCKER[@]}" inspect -f '{{range .Mounts}}{{printf "%s|%s\n" .Destination .Source}}{{end}}' "$panel_container")"
host_data_dir="$(printf '%s\n' "$mount_lines" | awk -F'|' -v target="$container_data_dir" '$1 == target {print $2; exit}')"

# Some NAS stacks keep PANEL_DATA_DIR at a host-style path while mounting it
# elsewhere in the container. If the exact destination does not contain the
# instance, accept only one mount source that proves it owns the Stardew .env.
if [[ -z "$host_data_dir" || ! -f "$host_data_dir/instances/stardew/.env" ]]; then
  mapfile -t data_mount_matches < <(
    while IFS='|' read -r _destination source; do
      [[ -n "$source" && -f "$source/instances/stardew/.env" ]] && printf '%s\n' "$source"
    done <<< "$mount_lines"
  )
  if [[ ${#data_mount_matches[@]} -eq 1 ]]; then
    host_data_dir="${data_mount_matches[0]}"
    yellow "PANEL_DATA_DIR 与 mount destination 不同；已用唯一包含实例 .env 的挂载源安全定位。"
  elif [[ ${#data_mount_matches[@]} -gt 1 ]]; then
    red "错误：多个 Panel 挂载都包含 Stardew 实例，为防止修错目录，脚本已停止："
    printf '  - %s\n' "${data_mount_matches[@]}"
    exit 1
  else
    red "错误：无法从 Panel 挂载中确定飞牛宿主机数据目录。"
    echo "容器内 PANEL_DATA_DIR：$container_data_dir"
    exit 1
  fi
fi

instance_dir="$host_data_dir/instances/stardew"
env_file="$instance_dir/.env"
update_dir="$instance_dir/.local-container/junimo-update"
required_status="$update_dir/required-status.json"
apply_status="$update_dir/apply-status.json"
echo "已定位实例目录：$instance_dir"
echo
if [[ ! -f "$env_file" ]]; then
  red "错误：没有找到 Stardew 实例配置：$env_file"
  exit 1
fi

read_env() {
  local key="$1"
  awk -F= -v key="$key" '
    {
      sub(/^\xef\xbb\xbf/, "")
      candidate_key=$1
      gsub(/^[[:space:]]+|[[:space:]]+$/, "", candidate_key)
    }
    candidate_key == key {
      sub(/^[^=]*=/, "")
      sub(/\r$/, "")
      gsub(/^[[:space:]]+|[[:space:]]+$/, "")
      if (length($0) >= 2 && ((substr($0,1,1) == "\"" && substr($0,length($0),1) == "\"") || (substr($0,1,1) == "\047" && substr($0,length($0),1) == "\047"))) {
        result=substr($0,2,length($0)-2)
      } else {
        result=$0
      }
    }
    END { if (result != "") print result }
  ' "$env_file"
}

server_image="$(read_env SERVER_IMAGE)"
auth_image="$(read_env STEAM_SERVICE_IMAGE)"
image_version="$(read_env IMAGE_VERSION)"
if [[ -z "$server_image" || -z "$auth_image" ]]; then
  red "错误：实例缺少 SERVER_IMAGE 或 STEAM_SERVICE_IMAGE；脚本没有修改任何文件。"
  exit 1
fi
if [[ "$server_image" == *"@"* || "$auth_image" == *"@"* || "$server_image" != *":"* || "$auth_image" != *":"* ]]; then
  red "错误：主镜像引用格式异常，禁止自动处理。"
  exit 1
fi

server_repo="${server_image%:*}"
server_tag="${server_image##*:}"
auth_repo="${auth_image%:*}"
auth_tag="${auth_image##*:}"
case "$server_repo" in
  dockerproxy.net/sdvd/server|docker.1ms.run/sdvd/server|docker.1panel.live/sdvd/server|docker.jiaxin.site/sdvd/server|dockerproxy.link/sdvd/server|sdvd/server) ;;
  *) red "错误：检测到自定义或未知 server 主镜像：$server_image；脚本已停止。"; exit 1 ;;
esac
case "$auth_repo" in
  docker.1ms.run/anxiyizhi/junimo-steam-service-cn|crpi-9z3bkb9g7fxeohrg.cn-hangzhou.personal.cr.aliyuncs.com/anxi-panel/junimo-steam-service-cn|docker.m.daocloud.io/anxiyizhi/junimo-steam-service-cn|ghcr.io/anxiyizhi/junimo-steam-service-cn|anxiyizhi/junimo-steam-service-cn) ;;
  *) red "错误：检测到自定义或未知 steam-auth 主镜像：$auth_image；脚本已停止。"; exit 1 ;;
esac
if [[ "$server_tag" != "1.5.0-preview.121" && "$server_tag" != "$TARGET_SERVER_TAG" ]]; then
  red "错误：server 主镜像 tag=$server_tag，不属于脚本唯一支持的 .121 → .125 路径。"
  exit 1
fi
if [[ "$auth_tag" != "$TARGET_AUTH_TAG" ]]; then
  red "错误：steam-auth 主镜像 tag=$auth_tag，不是已验证的 $TARGET_AUTH_TAG。"
  exit 1
fi
normalize_image_version=0
if [[ "$image_version" != "$server_tag" ]]; then
  if [[ -z "$image_version" || "$image_version" == "latest" ]]; then
    yellow "检测到旧版遗留 IMAGE_VERSION=${image_version:-<空>}；官方可信主镜像已唯一钉定为 $server_tag，将随完整备份一起收敛为该精确 tag。"
    normalize_image_version=1
  else
    red "错误：IMAGE_VERSION=$image_version，但 SERVER_IMAGE tag=$server_tag。脚本不能擅自选择版本。"
    exit 1
  fi
fi
echo "当前运行组件：JunimoServer $server_tag，steam-auth $auth_tag"

if [[ -f "$required_status" ]]; then
  required_phase="$(awk -F'"' '/"phase"[[:space:]]*:/ {print $4; exit}' "$required_status")"
  case "$required_phase" in
    checking|repairing|preflighting|applying)
      red "错误：Panel 内置 required update 正处于 $required_phase；请等待现有任务结束后再运行。"
      exit 1
      ;;
  esac
fi

if [[ -f "$apply_status" ]]; then
  apply_phase="$(awk -F'"' '/"phase"[[:space:]]*:/ {print $4; exit}' "$apply_status")"
  case "$apply_phase" in
    ""|succeeded|failed_rolled_back) ;;
    rollback_failed) red "错误：上一次 Junimo 升级回滚失败；恢复材料必须保留，脚本禁止继续。"; exit 1 ;;
    *) red "错误：检测到尚未结束的 Junimo 升级阶段：$apply_phase。请等待现有任务结束。"; exit 1 ;;
  esac
fi

smapi_apply_status="$instance_dir/.local-container/smapi-update/apply-status.json"
if [[ -f "$smapi_apply_status" ]]; then
  smapi_phase="$(awk -F'"' '/"phase"[[:space:]]*:/ {print $4; exit}' "$smapi_apply_status")"
  case "$smapi_phase" in
    ""|idle|succeeded|failed_rolled_back) ;;
    rollback_failed) red "错误：上一次 SMAPI 升级回滚失败；脚本禁止中断或覆盖恢复现场。"; exit 1 ;;
    *) red "错误：检测到尚未结束的 SMAPI 升级阶段：$smapi_phase。请等待现有任务结束。"; exit 1 ;;
  esac
fi

server_candidates="dockerproxy.net/sdvd/server:${server_tag},docker.1ms.run/sdvd/server:${server_tag},docker.1panel.live/sdvd/server:${server_tag},docker.jiaxin.site/sdvd/server:${server_tag},dockerproxy.link/sdvd/server:${server_tag},sdvd/server:${server_tag}"
auth_candidates="docker.1ms.run/anxiyizhi/junimo-steam-service-cn:${auth_tag},crpi-9z3bkb9g7fxeohrg.cn-hangzhou.personal.cr.aliyuncs.com/anxi-panel/junimo-steam-service-cn:${auth_tag},docker.m.daocloud.io/anxiyizhi/junimo-steam-service-cn:${auth_tag},ghcr.io/anxiyizhi/junimo-steam-service-cn:${auth_tag},anxiyizhi/junimo-steam-service-cn:${auth_tag}"
stamp="$(date +%Y%m%d-%H%M%S)"
backup_file="${env_file}.before-junimo-fix-${stamp}"
tmp_file="${env_file}.tmp-${stamp}"
panel_was_stopped=0

restart_panel_on_error() {
  if [[ "$panel_was_stopped" -eq 1 ]]; then
    yellow "发生错误，正在重新启动 Panel……"
    "${DOCKER[@]}" start "$panel_container" >/dev/null 2>&1 || true
  fi
}
trap restart_panel_on_error ERR INT TERM

echo "正在暂停 Panel 以安全修改配置……"
"${DOCKER[@]}" stop --time 30 "$panel_container" >/dev/null
panel_was_stopped=1
cp -p "$env_file" "$backup_file"
chmod 600 "$backup_file" 2>/dev/null || true

awk -v server_candidates="$server_candidates" -v auth_candidates="$auth_candidates" -v normalized_image_version="$server_tag" -v normalize_image_version="$normalize_image_version" '
  BEGIN { image_done=0; server_done=0; auth_done=0 }
  {
    sub(/^\xef\xbb\xbf/, "")
    sub(/\r$/, "")
    line_key=$0
    sub(/=.*/, "", line_key)
    gsub(/^[[:space:]]+|[[:space:]]+$/, "", line_key)
    if (normalize_image_version == "1" && line_key == "IMAGE_VERSION") {
      if (!image_done) { print "IMAGE_VERSION=" normalized_image_version; image_done=1 }
      next
    }
    if (line_key == "SERVER_IMAGE_CANDIDATES") {
      if (!server_done) { print "SERVER_IMAGE_CANDIDATES=" server_candidates; server_done=1 }
      next
    }
    if (line_key == "STEAM_SERVICE_IMAGE_CANDIDATES") {
      if (!auth_done) { print "STEAM_SERVICE_IMAGE_CANDIDATES=" auth_candidates; auth_done=1 }
      next
    }
    print
  }
  END {
    if (normalize_image_version == "1" && !image_done) print "IMAGE_VERSION=" normalized_image_version
    if (!server_done) print "SERVER_IMAGE_CANDIDATES=" server_candidates
    if (!auth_done) print "STEAM_SERVICE_IMAGE_CANDIDATES=" auth_candidates
  }
' "$env_file" > "$tmp_file"
chmod --reference="$env_file" "$tmp_file" 2>/dev/null || chmod 600 "$tmp_file"
chown --reference="$env_file" "$tmp_file" 2>/dev/null || true
mv -f "$tmp_file" "$env_file"

mkdir -p "$update_dir"
chmod 700 "$update_dir" 2>/dev/null || true
if [[ -f "$required_status" ]]; then
  cp -p "$required_status" "${required_status}.before-fix-${stamp}"
  rm -f "$required_status"
fi
green "候选配置已修复；原配置备份：$backup_file"
echo "正在启动 Panel 0.3.5，并触发内置升级事务……"
"${DOCKER[@]}" start "$panel_container" >/dev/null
panel_was_stopped=0
trap - ERR INT TERM

echo "目标：JunimoServer $TARGET_SERVER_TAG，steam-auth $TARGET_AUTH_TAG"
echo "通常需要 5～20 分钟；本脚本最多显示等待 30 分钟。"
last_phase=""
for ((i=1; i<=360; i++)); do
  sleep 5
  if [[ ! -f "$required_status" ]]; then
    printf '\r等待 Panel 创建升级任务……已等待 %4s 秒' "$((i*5))"
    continue
  fi
  phase="$(awk -F'"' '/"phase"[[:space:]]*:/ {print $4; exit}' "$required_status")"
  error_code="$(awk -F'"' '/"errorCode"[[:space:]]*:/ {print $4; exit}' "$required_status")"
  if [[ "$phase" != "$last_phase" ]]; then
    echo
    case "$phase" in
      checking) echo "阶段：检查当前 Junimo 配置" ;;
      repairing) echo "阶段：修复旧版候选配置" ;;
      preflighting) echo "阶段：下载并校验目标镜像" ;;
      applying) echo "阶段：安装、启动并验收新版本" ;;
      succeeded) echo "阶段：升级成功" ;;
      failed) echo "阶段：升级失败" ;;
      manual_action) echo "阶段：需要人工处理" ;;
      *) echo "阶段：${phase:-等待中}" ;;
    esac
    last_phase="$phase"
  fi
  printf '\r已等待：%4s 秒' "$((i*5))"
  case "$phase" in
    succeeded)
      echo
      if [[ "$server_tag" == "$TARGET_SERVER_TAG" ]]; then
        green "修复成功：候选配置已规范化，Panel 已确认 Junimo 配置为目标 .125。"
      else
        green "修复成功：Junimo 已通过 Panel 0.3.5 的 .121 → .125 升级和验收。"
      fi
      echo "IMAGE_VERSION=$(read_env IMAGE_VERSION)"
      echo "SERVER_IMAGE=$(read_env SERVER_IMAGE)"
      echo "STEAM_SERVICE_IMAGE=$(read_env STEAM_SERVICE_IMAGE)"
      exit 0
      ;;
    failed|manual_action)
      echo
      red "自动升级没有完成，错误码：${error_code:-unknown}"
      echo "候选配置已经备份并修复；请勿删除 Docker 卷或 game-data。"
      echo "请从面板诊断页导出支持包。若错误码为 target_image_candidates_failed，请先恢复飞牛的出站网络。"
      exit 2
      ;;
  esac
done

echo
yellow "等待超过 30 分钟；脚本只停止显示进度，不会终止后台升级任务。"
echo "请打开面板的“诊断 → 版本维护”查看最终状态。"
exit 3
