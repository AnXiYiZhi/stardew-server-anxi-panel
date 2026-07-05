#!/usr/bin/env bash

set -Eeuo pipefail

# Stardew Server Anxi Panel one-click launcher.
# User-editable defaults can be overridden before running the script, for example:
#   PANEL_VERSION=0.1.0 PANEL_PORT=9000 bash run.sh

PANEL_NAME="${PANEL_NAME:-anxi-panel}"
PANEL_CONTAINER_NAME="${PANEL_CONTAINER_NAME:-anxi-panel}"
PANEL_VERSION_WAS_SET="${PANEL_VERSION+x}"
PANEL_VERSION="${PANEL_VERSION:-latest}"
PANEL_PORT="${PANEL_PORT:-8090}"
INSTALL_DIR="${INSTALL_DIR:-$HOME/.anxi-panel}"

CN_REGISTRY_IMAGE="${CN_REGISTRY_IMAGE:-registry.cn-hangzhou.aliyuncs.com/anxi-panel/stardew-server-anxi-panel}"
DOCKERHUB_IMAGE="${DOCKERHUB_IMAGE:-anxiyizhi/stardew-server-anxi-panel}"
DEFAULT_MIRROR="${DEFAULT_MIRROR:-cn}"

COMPOSE_FILE="$INSTALL_DIR/docker-compose.yml"
ENV_FILE="$INSTALL_DIR/.env"

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

detect_sudo() {
  if docker info >/dev/null 2>&1; then
    SUDO=""
    return
  fi
  if command -v sudo >/dev/null 2>&1 && sudo docker info >/dev/null 2>&1; then
    SUDO="sudo"
    return
  fi
  red "无法连接 Docker daemon。请先安装并启动 Docker，或把当前用户加入 docker 用户组。"
  exit 1
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

require_command() {
  local name="$1"
  if ! command -v "$name" >/dev/null 2>&1; then
    red "缺少命令：$name"
    exit 1
  fi
}

check_prerequisites() {
  require_command docker
  detect_sudo
  if ! docker_cmd compose version >/dev/null 2>&1; then
    red "当前 Docker 不支持 'docker compose'。请安装 Docker Compose plugin。"
    exit 1
  fi
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

current_image() {
  if [[ -n "$SELECTED_PANEL_IMAGE" ]]; then
    printf "%s\n" "$SELECTED_PANEL_IMAGE"
    return
  fi

  if [[ -f "$ENV_FILE" && -z "$PANEL_VERSION_WAS_SET" ]]; then
    local existing
    existing="$(grep -E '^PANEL_IMAGE=' "$ENV_FILE" | tail -n1 | cut -d= -f2- || true)"
    if [[ -n "$existing" ]]; then
      printf "%s\n" "$existing"
      return
    fi
  fi

  if [[ "$DEFAULT_MIRROR" == "dockerhub" ]]; then
    printf "%s:%s\n" "$DOCKERHUB_IMAGE" "$PANEL_VERSION"
  else
    printf "%s:%s\n" "$CN_REGISTRY_IMAGE" "$PANEL_VERSION"
  fi
}

choose_image_source_interactive() {
  if [[ -f "$ENV_FILE" || ! -t 0 ]]; then
    return
  fi

  cyan "面板镜像需要从容器镜像仓库拉取，请选择一个镜像源："
  green "1. 国内阿里云 ACR（推荐）：${CN_REGISTRY_IMAGE}:${PANEL_VERSION}"
  green "2. Docker Hub：${DOCKERHUB_IMAGE}:${PANEL_VERSION}"
  green "3. 自定义完整镜像地址"
  read -r -p "请输入选择 [1-3，默认 1]: " selected_mirror
  selected_mirror="${selected_mirror:-1}"

  case "$selected_mirror" in
    1)
      SELECTED_PANEL_IMAGE="${CN_REGISTRY_IMAGE}:${PANEL_VERSION}"
      ;;
    2)
      SELECTED_PANEL_IMAGE="${DOCKERHUB_IMAGE}:${PANEL_VERSION}"
      ;;
    3)
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

  green "已选择镜像：$SELECTED_PANEL_IMAGE"
}

write_env_file() {
  mkdir -p "$INSTALL_DIR"

  local secret image
  image="$(current_image)"
  if [[ -f "$ENV_FILE" ]]; then
    secret="$(grep -E '^PANEL_SECRET=' "$ENV_FILE" | tail -n1 | cut -d= -f2- || true)"
  else
    secret=""
  fi
  if [[ -z "$secret" || "$secret" == "change-me" ]]; then
    secret="$(random_secret)"
  fi

  cat >"$ENV_FILE" <<EOF
PANEL_IMAGE=$image
PANEL_CONTAINER_NAME=$PANEL_CONTAINER_NAME
PANEL_PORT=$PANEL_PORT
PANEL_SECRET=$secret
PANEL_VERSION=$PANEL_VERSION
PANEL_DATA_DIR=/data
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
      - panel-data:/data
    environment:
      PANEL_ADDR: ":8090"
      PANEL_DATA_DIR: "${PANEL_DATA_DIR:-/data}"
      PANEL_SECRET: "${PANEL_SECRET}"
      PANEL_VERSION: "${PANEL_VERSION:-latest}"
      PANEL_MODE: "${PANEL_MODE:-single}"

volumes:
  panel-data:
EOF
}

ensure_files() {
  write_env_file
  write_compose_file
}

print_access_info() {
  local ip
  ip="$(hostname -I 2>/dev/null | awk '{print $1}')"
  [[ -z "$ip" ]] && ip="服务器IP"

  green "面板已按快速模式启动："
  cyan "  本机访问：http://localhost:${PANEL_PORT}"
  cyan "  公网访问：http://${ip}:${PANEL_PORT}"
  yellow "如果云服务器外部无法访问，请放行 TCP ${PANEL_PORT} 端口。"
  yellow "Stardew 游戏端口还需要按实例配置放行 UDP 24642 / 27015。"
  yellow "首次进入面板后请设置强管理员密码。"
}

start_panel() {
  check_prerequisites
  choose_image_source_interactive
  ensure_files
  cyan "正在拉取镜像：$(current_image)"
  compose_cmd pull
  cyan "正在启动面板..."
  compose_cmd up -d
  print_access_info
}

stop_panel() {
  check_prerequisites
  ensure_files
  compose_cmd down
  green "面板已停止，数据卷 panel-data 会保留。"
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
  cyan "正在更新镜像：$(current_image)"
  compose_cmd pull
  compose_cmd up -d
  green "更新完成。"
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
  green "1. 国内阿里云 ACR：${CN_REGISTRY_IMAGE}:${PANEL_VERSION}"
  green "2. Docker Hub：${DOCKERHUB_IMAGE}:${PANEL_VERSION}"
  read -r -p "请输入选择 [1-2]: " choice
  case "$choice" in
    1)
      sed -i.bak "s#^PANEL_IMAGE=.*#PANEL_IMAGE=${CN_REGISTRY_IMAGE}:${PANEL_VERSION}#" "$ENV_FILE"
      ;;
    2)
      sed -i.bak "s#^PANEL_IMAGE=.*#PANEL_IMAGE=${DOCKERHUB_IMAGE}:${PANEL_VERSION}#" "$ENV_FILE"
      ;;
    *)
      red "输入无效。"
      return 1
      ;;
  esac
  green "镜像源已切换为：$(current_image)"
  yellow "如需立即生效，请执行菜单 [4] 更新面板。"
}

print_menu() {
  clear
  green "Stardew Server Anxi Panel 一键启动脚本（快速模式）"
  cyan "安装目录：$INSTALL_DIR"
  cyan "当前镜像：$(current_image)"
  cyan "面板端口：$PANEL_PORT"
  yellow "————————————————————————————————————————————"
  green "[0] 拉取并启动面板"
  green "[1] 启动/恢复面板"
  green "[2] 停止面板"
  green "[3] 重启面板"
  yellow "————————————————————————————————————————————"
  green "[4] 更新面板镜像并重建容器"
  green "[5] 查看面板状态"
  green "[6] 查看面板日志"
  green "[7] 切换镜像源（国内 ACR / Docker Hub）"
  yellow "————————————————————————————————————————————"
  green "[8] 显示访问地址"
  green "[9] 退出"
  yellow "————————————————————————————————————————————"
}

main() {
  case "${1:-menu}" in
    install|start|up)
      start_panel
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
        read -r -p "请输入要执行的操作 [0-9]: " command
        case "$command" in
          0|1) start_panel; pause ;;
          2) stop_panel; pause ;;
          3) restart_panel; pause ;;
          4) update_panel; pause ;;
          5) show_status; pause ;;
          6) show_logs ;;
          7) switch_image; pause ;;
          8) print_access_info; pause ;;
          9) exit 0 ;;
          *) red "请输入 0-9 的数字。"; pause ;;
        esac
      done
      ;;
    *)
      red "未知命令：$1"
      echo "用法：bash run.sh [install|start|stop|restart|update|status|logs|url]"
      exit 1
      ;;
  esac
}

main "$@"
