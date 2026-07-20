#!/usr/bin/env bash

set -Eeuo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
export ANXI_MIGRATE_LIBRARY=1
source "$ROOT_DIR/deploy/migrate-fnos.sh"

[[ "$(normalize_version v0.3.13)" == "0.3.13" ]]
if normalize_version latest >/dev/null 2>&1; then exit 1; fi
version_gt 0.3.13 0.3.12
if version_gt 0.3.12 0.3.13; then exit 1; fi
if version_gt 0.3.13 0.3.13; then exit 1; fi
[[ "$(highest_candidate_version 'old|0.3.7|bind|/data|image|1' 'new|0.3.13|bind|/data|image|2' 'middle|0.3.10|bind|/data|image|3')" == "0.3.13" ]]
if highest_candidate_version 'bad|latest|bind|/data|image|1' >/dev/null 2>&1; then exit 1; fi
[[ "$(managed_project_name 'FNOS.Panel_01')" == "anxi-panel-fnospanel_01" ]]
[[ "$(managed_project_name 'panel-a')" != "$(managed_project_name 'panel-b')" ]]
if managed_project_name '...' >/dev/null 2>&1; then exit 1; fi
[[ "$(collision_safe_project_name anxi-panel-panel abcdef1234567890 old-project false)" == 'anxi-panel-panel' ]]
[[ "$(collision_safe_project_name anxi-panel-panel abcdef1234567890 anxi-panel-panel false)" == 'anxi-panel-migrated-abcdef123456' ]]
[[ "$(collision_safe_project_name anxi-panel-panel ABCDEF1234567890 old-project true)" == 'anxi-panel-migrated-abcdef123456' ]]
if collision_safe_project_name anxi-panel-panel invalid old-project false >/dev/null 2>&1; then exit 1; fi

trusted_panel_image anxiyizhi/stardew-server-anxi-panel:0.3.13
trusted_panel_image crpi-9z3bkb9g7fxeohrg.cn-hangzhou.personal.cr.aliyuncs.com/anxi-panel/stardew-server-anxi-panel:0.3.13
if trusted_panel_image example.invalid/anxi-panel:0.3.13; then exit 1; fi

# Literal dollar signs intentionally exercise Compose escaping.
# shellcheck disable=SC2016
quote_input='a$b"c\d'
# shellcheck disable=SC2016
quote_expected='"a$$b\"c\\d"'
quoted="$(yaml_quote "$quote_input")"
[[ "$quoted" == "$quote_expected" ]]
if yaml_quote $'unsafe\nvalue' >/dev/null 2>&1; then exit 1; fi

[[ "$(normalize_extra_mount bind /etc/localtime '' /etc/localtime false rprivate)" == 'bind|/etc/localtime|/etc/localtime|false|rprivate' ]]
[[ "$(normalize_extra_mount bind /mnt/panel-certs '' /etc/panel-certs true private)" == 'bind|/mnt/panel-certs|/etc/panel-certs|true|private' ]]
[[ "$(normalize_extra_mount volume ignored panel-cache /var/cache/panel true '')" == 'volume|panel-cache|/var/cache/panel|true|' ]]
if normalize_extra_mount tmpfs '' '' /tmp true '' >/dev/null 2>&1; then exit 1; fi
if normalize_extra_mount bind / '' /host false rprivate >/dev/null 2>&1; then exit 1; fi
if normalize_extra_mount bind /safe '' / false rprivate >/dev/null 2>&1; then exit 1; fi
if normalize_extra_mount bind /safe '' /target maybe rprivate >/dev/null 2>&1; then exit 1; fi
if normalize_extra_mount bind /safe '' /target false exotic >/dev/null 2>&1; then exit 1; fi
if normalize_extra_mount volume ignored 'bad/name' /target true '' >/dev/null 2>&1; then exit 1; fi
if normalize_extra_mount bind $'/bad\nsource' '' /target false rprivate >/dev/null 2>&1; then exit 1; fi

receipt="$(migration_success_receipt 0.4.0 /opt/anxi-panel/docker-compose.yml)"
[[ "$receipt" == *"确认完成：标准 Compose 迁移与升级环境校验全部通过"* ]]
[[ "$receipt" == *"确认结果：支持后续新版本通过 Panel Web 一键安全升级"* ]]
[[ "$receipt" == *"成功识别码：ANXI_PANEL_WEB_UPDATE_READY"* ]]

bash -n "$ROOT_DIR/deploy/migrate-fnos.sh"
echo "migrate-fnos.sh tests passed"
