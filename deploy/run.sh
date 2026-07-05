#!/usr/bin/env bash

set -Eeuo pipefail

# Stardew Server Anxi Panel one-click launcher.
# Examples:
#   bash run.sh
#   PANEL_VERSION=0.1.0 PANEL_PORT=9000 bash run.sh install

PANEL_NAME="${PANEL_NAME:-anxi-panel}"
PANEL_CONTAINER_NAME="${PANEL_CONTAINER_NAME:-anxi-panel}"
PANEL_VERSION_WAS_SET="${PANEL_VERSION+x}"
PANEL_VERSION="${PANEL_VERSION:-latest}"
PANEL_PORT="${PANEL_PORT:-8090}"
INSTALL_DIR="${INSTALL_DIR:-$HOME/.anxi-panel}"
PANEL_HOST_DATA_DIR="${PANEL_HOST_DATA_DIR:-$INSTALL_DIR/data}"
PANEL_CONTAINER_DATA_DIR="${PANEL_CONTAINER_DATA_DIR:-$PANEL_HOST_DATA_DIR}"

CN_REGISTRY_IMAGE="${CN_REGISTRY_IMAGE:-crpi-9z3bkb9g7fxeohrg.cn-hangzhou.personal.cr.aliyuncs.com/anxi-panel/stardew-server-anxi-panel}"
DOCKERHUB_IMAGE="${DOCKERHUB_IMAGE:-anxiyizhi/stardew-server-anxi-panel}"
GHCR_IMAGE="${GHCR_IMAGE:-ghcr.io/anxiyizhi/stardew-server-anxi-panel}"
DEFAULT_MIRROR="${DEFAULT_MIRROR:-cn}"
PANEL_IMAGE_CANDIDATES="${PANEL_IMAGE_CANDIDATES:-}"

RUN_SH_URL="${RUN_SH_URL:-https://github.com/anxiyizhi/stardew-server-anxi-panel/releases/latest/download/run.sh}"
RUN_SH_URL_CANDIDATES="${RUN_SH_URL_CANDIDATES:-https://gh.llkk.cc/${RUN_SH_URL},https://github.dpik.top/${RUN_SH_URL},https://ghfast.top/${RUN_SH_URL},${RUN_SH_URL}}"

COMPOSE_FILE="$INSTALL_DIR/docker-compose.yml"
ENV_FILE="$INSTALL_DIR/.env"
SCRIPT_PATH="${BASH_SOURCE[0]}"

SUDO=""
SELECTED_PANEL_IMAGE=""

color() {
  local code="$1"
  shift
  printf "\033[%sm%s\033[0m\n" "$code" "$*"
}

green() { color "0;32" "$*"; }
yellow() { color "0;33" "$*"; }
red() { color "0;31" "$*"; }
cyan() { color "0;36" "$*"; }

pause() {
  printf "\n"
  read -r -p "按回车键继续..." _
}

need_root_prefix() {
  if [[ "$(id -u)" -eq 0 ]]; then
    printf ""
    return
  fi
  if command -v sudo >/dev/null 2>&1; then
    printf "sudo"
    return
  fi
  red "当前操作需要 root 权限，请使用 root 用户运行，或先安装 sudo。"
  exit 1
}

run_as_root() {
  local prefix
  prefix="$(need_root_prefix)"
  if [[ -n "$prefix" ]]; then
    sudo "$@"
  else
    "$@"
  fi
}

require_command() {
  local name="$1"
  if ! command -v "$name" >/dev/null 2>&1; then
    red "缺少命令：$name"
    exit 1
  fi
}

detect_sudo() {
  if docker info >/dev/null 2>&1; then
    SUDO=""
    return 0
  fi
  if command -v sudo >/dev/null 2>&1 && sudo docker info >/dev/null 2>&1; then
    SUDO="sudo"
    return 0
  fi
  return 1
}

docker_cmd() {
  if [[ -n "$SUDO" ]]; then
    sudo docker "$@"
  else
    docker "$@"
  fi
}

compose_cmd() {
  docker_cmd compose --project-name "$PANEL_NAME" --env-file "$ENV_FILE" -f "$COMPOSE_FILE" "$@"
}

docker_ready() {
  command -v docker >/dev/null 2>&1 && detect_sudo && docker_cmd compose version >/dev/null 2>&1
}

check_prerequisites() {
  if docker_ready; then
    return
  fi

  if [[ -t 0 ]]; then
    yellow "未检测到可用的 Docker Engine / Docker Compose plugin。"
    read -r -p "是否现在安装/修复 Docker？[Y/n]: " install_now
    install_now="${install_now:-Y}"
    case "$install_now" in
      y|Y|yes|YES)
        install_docker
        ;;
      *)
        red "已取消。请先安装 Docker 后再启动面板。"
        exit 1
        ;;
    esac
  else
    red "未检测到可用的 Docker Engine / Docker Compose plugin。请先执行：bash run.sh docker"
    exit 1
  fi

  if ! docker_ready; then
    red "Docker 仍不可用。请检查 Docker 服务状态或重新登录 SSH。"
    exit 1
  fi
}

install_docker() {
  cyan "正在安装/修复 Docker Engine 与 Docker Compose plugin..."

  if docker_ready; then
    green "Docker 已可用，无需重复安装。"
    return
  fi

  if [[ ! -r /etc/os-release ]]; then
    red "无法识别系统版本：缺少 /etc/os-release。"
    exit 1
  fi

  # shellcheck disable=SC1091
  . /etc/os-release

  local os_id="${ID:-}"
  local os_like="${ID_LIKE:-}"
  local codename="${VERSION_CODENAME:-}"

  if command -v apt-get >/dev/null 2>&1; then
    local repo_os="$os_id"
    if [[ "$repo_os" != "ubuntu" && "$repo_os" != "debian" ]]; then
      if [[ "$os_like" == *"debian"* ]]; then
        repo_os="debian"
      elif [[ "$os_like" == *"ubuntu"* ]]; then
        repo_os="ubuntu"
      fi
    fi
    if [[ -z "$codename" ]]; then
      codename="$(. /etc/os-release && printf "%s" "${UBUNTU_CODENAME:-}")"
    fi
    if [[ -z "$codename" ]]; then
      red "无法识别 apt 系统 codename，请手动安装 Docker。"
      exit 1
    fi

    run_as_root apt-get update
    run_as_root apt-get install -y ca-certificates curl gnupg
    run_as_root install -m 0755 -d /etc/apt/keyrings
    run_as_root rm -f /etc/apt/keyrings/docker.gpg
    curl -fsSL "https://mirrors.aliyun.com/docker-ce/linux/${repo_os}/gpg" | run_as_root gpg --dearmor -o /etc/apt/keyrings/docker.gpg
    run_as_root chmod a+r /etc/apt/keyrings/docker.gpg
    printf "deb [arch=%s signed-by=/etc/apt/keyrings/docker.gpg] https://mirrors.aliyun.com/docker-ce/linux/%s %s stable\n" \
      "$(dpkg --print-architecture)" "$repo_os" "$codename" | run_as_root tee /etc/apt/sources.list.d/docker.list >/dev/null
    run_as_root apt-get update
    run_as_root apt-get install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin
  elif command -v dnf >/dev/null 2>&1 || command -v yum >/dev/null 2>&1; then
    local pkg="yum"
    command -v dnf >/dev/null 2>&1 && pkg="dnf"
    run_as_root "$pkg" install -y yum-utils
    run_as_root yum-config-manager --add-repo https://mirrors.aliyun.com/docker-ce/linux/centos/docker-ce.repo
    run_as_root "$pkg" install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin
  else
    red "当前系统暂不支持自动安装 Docker。请手动安装 Docker Engine 与 Docker Compose plugin。"
    exit 1
  fi

  if command -v systemctl >/dev/null 2>&1; then
    run_as_root systemctl enable --now docker
  else
    run_as_root service docker start || true
  fi

  green "Docker 安装/修复完成。"
  yellow "如果当前用户刚加入 docker 组，可能需要重新登录 SSH；脚本会优先尝试 sudo docker。"
}

random_secret() {
  if command -v openssl >/dev/null 2>&1; then
    openssl rand -hex 32
    return
  fi
  if [[ -r /dev/urandom ]]; then
    od -An -N32 -tx1 /dev/urandom | tr -d ' \n'
    printf '\n'
    return
  fi
  date +%s%N | sha256sum | awk '{print $1}'
}

read_env_value() {
  local key="$1"
  if [[ ! -f "$ENV_FILE" ]]; then
    return
  fi
  grep -E "^${key}=" "$ENV_FILE" | tail -n1 | cut -d= -f2- || true
}

is_unique() {
  local value="$1"
  shift
  local item
  for item in "$@"; do
    [[ "$item" == "$value" ]] && return 1
  done
  return 0
}

default_candidate_images() {
  local selected=()
  case "$DEFAULT_MIRROR" in
    dockerhub) selected+=("${DOCKERHUB_IMAGE}:${PANEL_VERSION}") ;;
    ghcr) selected+=("${GHCR_IMAGE}:${PANEL_VERSION}") ;;
    *) selected+=("${CN_REGISTRY_IMAGE}:${PANEL_VERSION}") ;;
  esac

  selected+=(
    "${CN_REGISTRY_IMAGE}:${PANEL_VERSION}"
    "docker.1ms.run/${DOCKERHUB_IMAGE}:${PANEL_VERSION}"
    "docker.m.daocloud.io/${DOCKERHUB_IMAGE}:${PANEL_VERSION}"
    "${GHCR_IMAGE}:${PANEL_VERSION}"
    "${DOCKERHUB_IMAGE}:${PANEL_VERSION}"
  )

  local seen=()
  local image
  for image in "${selected[@]}"; do
    if is_unique "$image" "${seen[@]}"; then
      seen+=("$image")
      printf "%s\n" "$image"
    fi
  done
}

candidate_images() {
  local refs=()
  local item

  if [[ -n "$SELECTED_PANEL_IMAGE" ]]; then
    refs+=("$SELECTED_PANEL_IMAGE")
  elif [[ -f "$ENV_FILE" && -z "$PANEL_VERSION_WAS_SET" ]]; then
    item="$(read_env_value PANEL_IMAGE)"
    [[ -n "$item" ]] && refs+=("$item")
  fi

  if [[ -n "$PANEL_IMAGE_CANDIDATES" ]]; then
    local custom_refs=()
    IFS=',' read -r -a custom_refs <<<"$PANEL_IMAGE_CANDIDATES"
    for item in "${custom_refs[@]}"; do
      item="$(printf "%s" "$item" | xargs)"
      [[ -n "$item" ]] && refs+=("$item")
    done
  else
    while IFS= read -r item; do
      [[ -n "$item" ]] && refs+=("$item")
    done < <(default_candidate_images)
  fi

  local seen=()
  for item in "${refs[@]}"; do
    if is_unique "$item" "${seen[@]}"; then
      seen+=("$item")
      printf "%s\n" "$item"
    fi
  done
}

current_image() {
  if [[ -n "$SELECTED_PANEL_IMAGE" ]]; then
    printf "%s\n" "$SELECTED_PANEL_IMAGE"
    return
  fi
  if [[ -f "$ENV_FILE" && -z "$PANEL_VERSION_WAS_SET" ]]; then
    local existing
    existing="$(read_env_value PANEL_IMAGE)"
    if [[ -n "$existing" ]]; then
      printf "%s\n" "$existing"
      return
    fi
  fi
  default_candidate_images | head -n1
}

choose_image_source_interactive() {
  if [[ -f "$ENV_FILE" || ! -t 0 ]]; then
    return
  fi

  cyan "面板镜像需要从容器镜像仓库拉取，请选择一个镜像源："
  green "1. 自动选择最快可用源（推荐）"
  green "2. 国内阿里云 ACR：${CN_REGISTRY_IMAGE}:${PANEL_VERSION}"
  green "3. Docker Hub 加速链路：docker.1ms.run/${DOCKERHUB_IMAGE}:${PANEL_VERSION}"
  green "4. DaoCloud 加速链路：docker.m.daocloud.io/${DOCKERHUB_IMAGE}:${PANEL_VERSION}"
  green "5. GitHub GHCR：${GHCR_IMAGE}:${PANEL_VERSION}"
  green "6. Docker Hub 官方：${DOCKERHUB_IMAGE}:${PANEL_VERSION}"
  green "7. 自定义完整镜像地址"
  read -r -p "请输入选择 [1-7，默认 1]: " selected_mirror
  selected_mirror="${selected_mirror:-1}"

  case "$selected_mirror" in
    1) SELECTED_PANEL_IMAGE="" ;;
    2) SELECTED_PANEL_IMAGE="${CN_REGISTRY_IMAGE}:${PANEL_VERSION}" ;;
    3) SELECTED_PANEL_IMAGE="docker.1ms.run/${DOCKERHUB_IMAGE}:${PANEL_VERSION}" ;;
    4) SELECTED_PANEL_IMAGE="docker.m.daocloud.io/${DOCKERHUB_IMAGE}:${PANEL_VERSION}" ;;
    5) SELECTED_PANEL_IMAGE="${GHCR_IMAGE}:${PANEL_VERSION}" ;;
    6) SELECTED_PANEL_IMAGE="${DOCKERHUB_IMAGE}:${PANEL_VERSION}" ;;
    7)
      read -r -p "请输入完整镜像地址（例如 registry.example.com/ns/image:tag）: " custom_image
      if [[ -z "$custom_image" ]]; then
        red "镜像地址不能为空。"
        exit 1
      fi
      SELECTED_PANEL_IMAGE="$custom_image"
      ;;
    *)
      red "输入无效。"
      exit 1
      ;;
  esac

  if [[ -n "$SELECTED_PANEL_IMAGE" ]]; then
    green "已选择镜像：$SELECTED_PANEL_IMAGE"
  else
    green "已选择自动候选源。"
  fi
}

write_env_file() {
  mkdir -p "$INSTALL_DIR" "$PANEL_HOST_DATA_DIR"

  local secret image candidates host_data data_dir
  image="$(current_image)"
  candidates="$(candidate_images | paste -sd, -)"
  secret="$(read_env_value PANEL_SECRET)"
  host_data="$(read_env_value PANEL_HOST_DATA_DIR)"
  data_dir="$(read_env_value PANEL_DATA_DIR)"

  [[ -z "$host_data" ]] && host_data="$PANEL_HOST_DATA_DIR"
  [[ -z "$data_dir" ]] && data_dir="$PANEL_CONTAINER_DATA_DIR"
  if [[ "$data_dir" == "/data" ]]; then
    host_data="$PANEL_HOST_DATA_DIR"
    data_dir="$PANEL_CONTAINER_DATA_DIR"
  fi
  if [[ -z "$secret" || "$secret" == "change-me" ]]; then
    secret="$(random_secret)"
  fi

  cat >"$ENV_FILE" <<EOF
PANEL_IMAGE=$image
PANEL_IMAGE_CANDIDATES=$candidates
PANEL_CONTAINER_NAME=$PANEL_CONTAINER_NAME
PANEL_PORT=$PANEL_PORT
PANEL_SECRET=$secret
PANEL_VERSION=$PANEL_VERSION
PANEL_HOST_DATA_DIR=$host_data
PANEL_DATA_DIR=$data_dir
PANEL_MODE=single
EOF
  chmod 600 "$ENV_FILE"
}

write_compose_file() {
  mkdir -p "$INSTALL_DIR"
  cat >"$COMPOSE_FILE" <<'EOF'
services:
  panel:
    image: ${PANEL_IMAGE}
    container_name: ${PANEL_CONTAINER_NAME:-anxi-panel}
    restart: unless-stopped
    ports:
      - "${PANEL_PORT:-8090}:8090"
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock
      - ${PANEL_HOST_DATA_DIR}:${PANEL_DATA_DIR}
    environment:
      PANEL_ADDR: ":8090"
      PANEL_DATA_DIR: "${PANEL_DATA_DIR}"
      PANEL_SECRET: "${PANEL_SECRET}"
      PANEL_VERSION: "${PANEL_VERSION:-latest}"
      PANEL_MODE: "${PANEL_MODE:-single}"
EOF
}

ensure_files() {
  write_env_file
  write_compose_file
}

set_env_value() {
  local key="$1"
  local value="$2"
  if [[ -f "$ENV_FILE" && "$(grep -c -E "^${key}=" "$ENV_FILE" || true)" -gt 0 ]]; then
    sed -i.bak "s#^${key}=.*#${key}=${value}#" "$ENV_FILE"
  else
    printf "%s=%s\n" "$key" "$value" >>"$ENV_FILE"
  fi
}

resolve_panel_image() {
  local force_pull="${1:-false}"
  local image refs=()

  while IFS= read -r image; do
    [[ -n "$image" ]] && refs+=("$image")
  done < <(candidate_images)

  if [[ "${#refs[@]}" -eq 0 ]]; then
    red "没有可用的面板镜像候选。"
    exit 1
  fi

  if [[ "$force_pull" != "true" ]]; then
    for image in "${refs[@]}"; do
      if docker_cmd image inspect "$image" >/dev/null 2>&1; then
        SELECTED_PANEL_IMAGE="$image"
        set_env_value PANEL_IMAGE "$image"
        green "本地已有面板镜像，直接使用：$image"
        return
      fi
    done
  fi

  local failures=()
  local index=1
  for image in "${refs[@]}"; do
    cyan "正在拉取面板镜像（${index}/${#refs[@]}）：$image"
    if docker_cmd pull "$image"; then
      SELECTED_PANEL_IMAGE="$image"
      set_env_value PANEL_IMAGE "$image"
      green "镜像拉取成功：$image"
      return
    fi
    failures+=("$image")
    yellow "镜像拉取失败，继续尝试下一个候选。"
    index=$((index + 1))
  done

  red "所有面板镜像候选都拉取失败："
  printf "  - %s\n" "${failures[@]}"
  exit 1
}

print_access_info() {
  local ip
  ip="$(hostname -I 2>/dev/null | awk '{print $1}')"
  [[ -z "$ip" ]] && ip="服务器IP"

  green "面板已按快速模式启动："
  cyan "  本机访问：http://localhost:${PANEL_PORT}"
  cyan "  公网访问：http://${ip}:${PANEL_PORT}"
  yellow "如果云服务器外部无法访问，请放行 TCP ${PANEL_PORT}。"
  yellow "Stardew 游戏端口还需要按实例配置放行 UDP 24642 / 27015。"
  yellow "首次进入面板后请初始化管理员并设置强密码。"
}

start_panel() {
  check_prerequisites
  choose_image_source_interactive
  ensure_files
  resolve_panel_image false
  cyan "正在启动面板..."
  compose_cmd up -d
  print_access_info
}

stop_panel() {
  check_prerequisites
  ensure_files
  compose_cmd down
  green "面板已停止，数据目录会保留：$(read_env_value PANEL_HOST_DATA_DIR)"
}

restart_panel() {
  check_prerequisites
  ensure_files
  compose_cmd restart || compose_cmd up -d
  print_access_info
}

update_panel() {
  check_prerequisites
  ensure_files
  resolve_panel_image true
  cyan "正在重建面板容器..."
  compose_cmd up -d
  green "更新完成。"
  print_access_info
}

force_update_panel() {
  check_prerequisites
  ensure_files
  local image
  image="$(current_image)"
  yellow "将尝试删除本地旧镜像并重新拉取：$image"
  docker_cmd image rm "$image" >/dev/null 2>&1 || true
  resolve_panel_image true
  compose_cmd up -d --force-recreate
  green "强制更新完成。"
  print_access_info
}

show_status() {
  check_prerequisites
  ensure_files
  compose_cmd ps
  printf "\n"
  docker_cmd exec "$PANEL_CONTAINER_NAME" wget -qO- http://localhost:8090/health 2>/dev/null || true
  printf "\n"
}

show_logs() {
  check_prerequisites
  ensure_files
  compose_cmd logs -f --tail=200
}

switch_image() {
  check_prerequisites
  ensure_files
  cyan "请选择面板镜像源："
  green "1. 自动选择最快可用源"
  green "2. 国内阿里云 ACR：${CN_REGISTRY_IMAGE}:${PANEL_VERSION}"
  green "3. Docker Hub 加速链路：docker.1ms.run/${DOCKERHUB_IMAGE}:${PANEL_VERSION}"
  green "4. DaoCloud 加速链路：docker.m.daocloud.io/${DOCKERHUB_IMAGE}:${PANEL_VERSION}"
  green "5. GitHub GHCR：${GHCR_IMAGE}:${PANEL_VERSION}"
  green "6. Docker Hub 官方：${DOCKERHUB_IMAGE}:${PANEL_VERSION}"
  green "7. 自定义完整镜像地址"
  read -r -p "请输入选择 [1-7]: " choice
  case "$choice" in
    1) SELECTED_PANEL_IMAGE="$(default_candidate_images | head -n1)" ;;
    2) SELECTED_PANEL_IMAGE="${CN_REGISTRY_IMAGE}:${PANEL_VERSION}" ;;
    3) SELECTED_PANEL_IMAGE="docker.1ms.run/${DOCKERHUB_IMAGE}:${PANEL_VERSION}" ;;
    4) SELECTED_PANEL_IMAGE="docker.m.daocloud.io/${DOCKERHUB_IMAGE}:${PANEL_VERSION}" ;;
    5) SELECTED_PANEL_IMAGE="${GHCR_IMAGE}:${PANEL_VERSION}" ;;
    6) SELECTED_PANEL_IMAGE="${DOCKERHUB_IMAGE}:${PANEL_VERSION}" ;;
    7)
      read -r -p "请输入完整镜像地址： " custom_image
      [[ -z "$custom_image" ]] && red "镜像地址不能为空。" && return 1
      SELECTED_PANEL_IMAGE="$custom_image"
      ;;
    *) red "输入无效。"; return 1 ;;
  esac

  write_env_file
  if [[ -n "$SELECTED_PANEL_IMAGE" ]]; then
    set_env_value PANEL_IMAGE "$SELECTED_PANEL_IMAGE"
  fi
  green "镜像源设置完成。当前候选："
  candidate_images | sed 's/^/  - /'
  yellow "如需立即生效，请执行菜单 [4] 更新面板或 [5] 强制更新面板。"
}

update_script() {
  require_command curl

  local script_dir tmp url urls=()
  script_dir="$(cd "$(dirname "$SCRIPT_PATH")" && pwd)"
  tmp="$(mktemp)"

  IFS=',' read -r -a urls <<<"$RUN_SH_URL_CANDIDATES"
  for url in "${urls[@]}"; do
    url="$(printf "%s" "$url" | xargs)"
    [[ -z "$url" ]] && continue
    cyan "正在尝试下载脚本：$url"
    if curl -fL --connect-timeout 15 --max-time 120 -o "$tmp" "$url"; then
      if head -n1 "$tmp" | grep -q "bash"; then
        cp "$tmp" "$script_dir/run.sh"
        chmod +x "$script_dir/run.sh"
        rm -f "$tmp"
        green "run.sh 已更新：$script_dir/run.sh"
        return
      fi
      yellow "下载内容不像 run.sh，继续尝试下一个地址。"
    fi
  done

  rm -f "$tmp"
  red "run.sh 更新失败，请稍后重试。"
  exit 1
}

setup_swap() {
  local size_gb="${1:-}"
  if [[ -z "$size_gb" && -t 0 ]]; then
    read -r -p "请输入虚拟内存大小 GB [默认 2]: " size_gb
  fi
  size_gb="${size_gb:-2}"
  if ! [[ "$size_gb" =~ ^[0-9]+$ ]] || [[ "$size_gb" -lt 1 ]]; then
    red "虚拟内存大小必须是正整数。"
    exit 1
  fi

  if swapon --show | grep -q '^/swapfile'; then
    green "虚拟内存已启用："
    swapon --show
    return
  fi

  cyan "正在创建 ${size_gb}G 虚拟内存 /swapfile..."
  if command -v fallocate >/dev/null 2>&1; then
    run_as_root fallocate -l "${size_gb}G" /swapfile
  else
    run_as_root dd if=/dev/zero of=/swapfile bs=1G count="$size_gb"
  fi
  run_as_root chmod 600 /swapfile
  run_as_root mkswap /swapfile
  run_as_root swapon /swapfile
  if ! grep -q '^/swapfile ' /etc/fstab; then
    printf "/swapfile none swap sw 0 0\n" | run_as_root tee -a /etc/fstab >/dev/null
  fi
  green "虚拟内存已启用。"
  swapon --show
}

enable_autostart() {
  check_prerequisites
  ensure_files
  if command -v systemctl >/dev/null 2>&1; then
    run_as_root systemctl enable docker
  fi
  compose_cmd up -d
  green "开机自启已设置：Docker 服务开机启动，面板容器 restart=unless-stopped。"
}

print_menu() {
  clear
  green "Stardew Server Anxi Panel 一键启动脚本（快速模式）"
  cyan "安装目录：$INSTALL_DIR"
  cyan "数据目录：$PANEL_HOST_DATA_DIR"
  cyan "当前镜像：$(current_image)"
  cyan "面板端口：$PANEL_PORT"
  yellow "————————————————————————————————————————————"
  green "[0] 下载/检查环境并启动面板（推荐）"
  green "[1] 安装/修复 Docker 与 Compose"
  green "[2] 启动/恢复面板"
  green "[3] 停止面板"
  green "[4] 重启面板"
  yellow "————————————————————————————————————————————"
  green "[5] 更新面板镜像并重建容器"
  green "[6] 强制更新面板镜像"
  green "[7] 切换镜像源/加速节点"
  green "[8] 更新 run.sh 启动脚本"
  yellow "————————————————————————————————————————————"
  green "[9] 设置虚拟内存"
  green "[10] 设置开机自启"
  green "[11] 查看面板状态"
  green "[12] 查看面板日志"
  green "[13] 显示访问地址"
  green "[14] 退出"
  yellow "————————————————————————————————————————————"
}

main() {
  case "${1:-menu}" in
    install|start|up)
      start_panel
      ;;
    docker|install-docker)
      install_docker
      ;;
    stop|down)
      stop_panel
      ;;
    restart)
      restart_panel
      ;;
    update|upgrade)
      update_panel
      ;;
    force-update|force-upgrade)
      force_update_panel
      ;;
    switch-image|mirror)
      switch_image
      ;;
    update-script|self-update)
      update_script
      ;;
    swap)
      setup_swap "${2:-}"
      ;;
    autostart)
      enable_autostart
      ;;
    status|ps)
      show_status
      ;;
    logs)
      show_logs
      ;;
    url)
      print_access_info
      ;;
    menu)
      while true; do
        print_menu
        read -r -p "请输入要执行的操作 [0-14]: " command
        case "$command" in
          0) start_panel; pause ;;
          1) install_docker; pause ;;
          2) start_panel; pause ;;
          3) stop_panel; pause ;;
          4) restart_panel; pause ;;
          5) update_panel; pause ;;
          6) force_update_panel; pause ;;
          7) switch_image; pause ;;
          8) update_script; pause ;;
          9) setup_swap; pause ;;
          10) enable_autostart; pause ;;
          11) show_status; pause ;;
          12) show_logs ;;
          13) print_access_info; pause ;;
          14) exit 0 ;;
          *) red "请输入 0-14 的数字。"; pause ;;
        esac
      done
      ;;
    *)
      red "未知命令：$1"
      echo "用法：bash run.sh [install|docker|stop|restart|update|force-update|switch-image|update-script|swap|autostart|status|logs|url]"
      exit 1
      ;;
  esac
}

main "$@"
