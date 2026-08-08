#!/usr/bin/env bash
set -Eeuo pipefail

PANEL_URL="${PANEL_URL:-http://127.0.0.1:${PANEL_PORT:-8090}}"
INSTANCE_ID="${INSTANCE_ID:-stardew}"
REPAIR_TIMEOUT_SECONDS="${REPAIR_TIMEOUT_SECONDS:-7200}"
ACTION="${1:-repair}"

green() { printf '\033[0;32m%s\033[0m\n' "$*"; }
yellow() { printf '\033[0;33m%s\033[0m\n' "$*"; }
red() { printf '\033[0;31m%s\033[0m\n' "$*" >&2; }
die() { red "$*"; exit 1; }

case "$ACTION" in
  check|repair) ;;
  -h|--help)
    printf '用法：bash repair-junimo-upgrade.sh [check|repair]\n'
    printf '环境变量：PANEL_URL、INSTANCE_ID、PANEL_USERNAME、PANEL_PASSWORD_FILE、REPAIR_TIMEOUT_SECONDS\n'
    exit 0
    ;;
  *) die "不支持的操作：$ACTION（仅允许 check 或 repair）" ;;
esac

command -v curl >/dev/null 2>&1 || die "缺少 curl。"
[[ "$PANEL_URL" =~ ^https?://[^[:space:]/]+(:[0-9]+)?/?$ ]] || die "PANEL_URL 必须是仅含协议、主机和可选端口的 http(s) 地址。"
[[ "$INSTANCE_ID" =~ ^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$ ]] || die "INSTANCE_ID 格式不安全。"
if [[ ! "$REPAIR_TIMEOUT_SECONDS" =~ ^[0-9]+$ ]] || (( REPAIR_TIMEOUT_SECONDS < 30 || REPAIR_TIMEOUT_SECONDS > 14400 )); then
  die "REPAIR_TIMEOUT_SECONDS 必须在 30 到 14400 之间。"
fi
PANEL_URL="${PANEL_URL%/}"

task_tmp_dir="$(mktemp -d "${TMPDIR:-/tmp}/anxi-junimo-repair.XXXXXX")"
chmod 700 "$task_tmp_dir"
# shellcheck disable=SC2317 # invoked through EXIT/INT/TERM traps
cleanup() { rm -rf -- "$task_tmp_dir"; }
trap cleanup EXIT INT TERM
cookie_file="$task_tmp_dir/cookies.txt"
login_file="$task_tmp_dir/login.json"
response_file="$task_tmp_dir/response.json"
confirm_file="$task_tmp_dir/confirm.json"
: >"$cookie_file"
: >"$login_file"
: >"$response_file"
: >"$confirm_file"
chmod 600 "$cookie_file" "$login_file" "$response_file" "$confirm_file"

json_escape() {
  local value="$1"
  value="${value//\\/\\\\}"
  value="${value//\"/\\\"}"
  value="${value//$'\t'/\\t}"
  value="${value//$'\r'/\\r}"
  value="${value//$'\n'/\\n}"
  printf '%s' "$value"
}

username="${PANEL_USERNAME:-}"
if [[ -z "$username" ]]; then
  if [[ -r /dev/tty ]]; then
    read -r -p "Panel 管理员用户名 [admin]: " username </dev/tty
  fi
  username="${username:-admin}"
fi
password=""
if [[ -n "${PANEL_PASSWORD_FILE:-}" ]]; then
  [[ -f "$PANEL_PASSWORD_FILE" && ! -L "$PANEL_PASSWORD_FILE" ]] || die "PANEL_PASSWORD_FILE 必须是普通文件。"
  IFS= read -r password <"$PANEL_PASSWORD_FILE" || true
elif [[ -r /dev/tty ]]; then
  read -r -s -p "Panel 管理员密码: " password </dev/tty
  printf '\n' >/dev/tty
else
  die "非交互环境请通过 PANEL_PASSWORD_FILE 提供管理员密码。"
fi
[[ -n "$password" ]] || die "管理员密码不能为空。"
[[ ! "$username" =~ [[:cntrl:]] ]] || die "管理员用户名不能包含控制字符。"
[[ ! "$password" =~ [[:cntrl:]] ]] || die "管理员密码不能包含控制字符。"

printf '{"username":"%s","password":"%s"}\n' "$(json_escape "$username")" "$(json_escape "$password")" >"$login_file"
unset password
http_code="$(curl --silent --show-error --connect-timeout 10 --max-time 30 --output "$response_file" --write-out '%{http_code}' --cookie-jar "$cookie_file" --header 'Content-Type: application/json' --data-binary "@$login_file" "$PANEL_URL/api/auth/login")" || die "无法连接 Panel 登录接口。"
: >"$login_file"
[[ "$http_code" == "200" ]] || die "Panel 管理员登录失败（HTTP $http_code）。"

status_url="$PANEL_URL/api/instances/$INSTANCE_ID/junimo-update/apply"
repair_url="$PANEL_URL/api/instances/$INSTANCE_ID/junimo-update/repair"
http_code="$(curl --silent --show-error --connect-timeout 10 --max-time 30 --output "$response_file" --write-out '%{http_code}' --cookie "$cookie_file" "$status_url")" || die "读取升级状态失败。"
[[ "$http_code" == "200" ]] || die "读取升级状态失败（HTTP $http_code）。"
phase="$(sed -n 's/.*"phase":"\([^"]*\)".*/\1/p' "$response_file" | head -n1)"
apply_id="$(sed -n 's/.*"applyId":"\([^"]*\)".*/\1/p' "$response_file" | head -n1)"
rollback_code="$(sed -n 's/.*"rollbackCode":"\([^"]*\)".*/\1/p' "$response_file" | head -n1)"
yellow "当前状态：${phase:-unknown}  applyId=${apply_id:-none}  rollback=${rollback_code:-none}"

if [[ "$ACTION" == "check" ]]; then
  if [[ "$phase" == "rollback_failed" ]]; then
    yellow "存在可提交给后端校验的一键恢复事务。"
  else
    green "当前没有 rollback_failed 恢复锁。"
  fi
  exit 0
fi
[[ "$phase" == "rollback_failed" ]] || die "当前状态不是 rollback_failed；脚本不会创建或猜测新的恢复事务。"
printf '{"confirm":true}\n' >"$confirm_file"
http_code="$(curl --silent --show-error --connect-timeout 10 --max-time 30 --output "$response_file" --write-out '%{http_code}' --cookie "$cookie_file" --header 'Content-Type: application/json' --data-binary "@$confirm_file" "$repair_url")" || die "提交一键安全恢复失败。"
[[ "$http_code" == "202" ]] || {
  error_code="$(sed -n 's/.*"code":"\([^"]*\)".*/\1/p' "$response_file" | head -n1)"
  die "后端拒绝修复（HTTP $http_code，${error_code:-unknown}）；实例未被脚本直接修改。"
}

green "后端已接受安全恢复；正在等待原版本验收。"
deadline=$((SECONDS + REPAIR_TIMEOUT_SECONDS))
while (( SECONDS < deadline )); do
  sleep 3
  http_code="$(curl --silent --show-error --connect-timeout 10 --max-time 30 --output "$response_file" --write-out '%{http_code}' --cookie "$cookie_file" "$status_url")" || {
    yellow "Panel 暂时不可达，继续等待。"
    continue
  }
  [[ "$http_code" == "200" ]] || {
    yellow "状态接口暂时返回 HTTP $http_code，继续等待。"
    continue
  }
  phase="$(sed -n 's/.*"phase":"\([^"]*\)".*/\1/p' "$response_file" | head -n1)"
  case "$phase" in
    failed_rolled_back)
      green "一键安全恢复完成：原运行组件、认证卷、配置和运行状态已通过验收。"
      exit 0
      ;;
    rollback_failed)
      rollback_code="$(sed -n 's/.*"rollbackCode":"\([^"]*\)".*/\1/p' "$response_file" | head -n1)"
      die "安全恢复仍未完成（${rollback_code:-unknown}）；恢复材料已保留，脚本不会绕过后端继续操作。"
      ;;
    rolling_back) yellow "安全恢复进行中……" ;;
    *) yellow "等待恢复终态，当前阶段：${phase:-unknown}" ;;
  esac
done
die "等待安全恢复超时；任务可能仍在 Panel 后台运行，请稍后执行 check。"
