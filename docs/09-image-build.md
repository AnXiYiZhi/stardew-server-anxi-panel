# RUN-SH-DOCKER-APT-FALLBACK-1 Docker APT 源自动切换

- `deploy/run.sh` 的 Docker/Compose 自动安装在 apt 系系统上不再只依赖阿里云 Docker CE 源。
- 脚本会先获取 Docker APT GPG key，然后按顺序尝试 Docker CE apt 源：阿里云、清华 TUNA、中科大 USTC、Docker 官方源。
- 脚本现在只写入托管源 `/etc/apt/sources.list.d/anxi-panel-docker.list`；进入安装前会扫描 `/etc/apt/sources.list` 和 `/etc/apt/sources.list.d/`，把历史残留的 Docker CE 源行注释掉，Deb822 `.sources` 源文件会改名停用并留下 `.anxi-panel-bak` 备份。
- 每次切换 Docker CE apt 源前会清理 `/var/lib/apt/lists/` 下的 Docker 源索引，避免镜像站同步期间出现 `File has unexpected size ... Mirror sync in progress?` 后继续复用坏源或坏索引。
- 现场如果仍失败，通常说明服务器无法访问所有候选 Docker 源；可稍后重试 `bash run.sh docker`，或手动安装 Docker Engine 与 Docker Compose plugin 后再执行 `bash run.sh install`。

# JUNIMO-STATIC-INIT-FIX-1 JunimoServer 镜像启动兼容

- 部分上游 `sdvd/server:1.5.0-preview.121` 镜像在 `/etc/cont-env.d`、`/etc/cont-groups.d`、`/etc/cont-users.d` 内写入裸静态值，当前 init 会把它们当 shell 命令执行。真实失败可表现为 `DockerApp: not found`、`unix:path=/tmp/dbus.base: not found`、`linux/amd64: not found`、`72: not found`。
- 面板不再要求用户使用本地热修 server 镜像；实例目录会自动生成 `.local-container/cont-env/*`、`.local-container/cont-groups/*`、`.local-container/cont-users/*` 脚本，并 bind mount 到 server 容器内覆盖对应静态值文件。
- 该修复不改变 `SERVER_IMAGE` / `SERVER_IMAGE_CANDIDATES` 的选择逻辑，也不会影响镜像拉取兜底。离线部署时只需保证 panel 镜像更新到包含本修复的版本。
- 排查命令：`grep -n "cont-env\\|cont-groups\\|cont-users" /path/to/instance/docker-compose.yml`，以及查看实例目录 `.local-container/cont-env/`、`.local-container/cont-groups/`、`.local-container/cont-users/`。

# INVITE-BACKGROUND-POLL-1 启动不阻塞邀请码

- 面板镜像包含本次生命周期行为：启动/重启只负责把 server 拉起，邀请码在后台最多探测 20 次。探测失败不影响 IP 直连，不会关闭 server。
- 部署覆盖镜像后，旧实例无需手动进服务器修复；下一次启动/重启会自动清理旧 SMAPI `status.json` / `players.json` 快照，并使用新的后台邀请码探测逻辑。
- 前端通过 `/api/instances/:id/state.inviteCode` 接收后端后台探测到的邀请码；因此覆盖镜像时必须同时包含本次后端与前端构建产物。

# SMAPI 运行环境预安装

- 面板镜像本身不内置 SMAPI。安装 Stardew 时，后端会在游戏文件和 Steam SDK 完成后，用已选择的 JunimoServer 镜像启动一次性 `docker run --rm` 容器，挂载 `<project>_game-data:/data/game` 并安装 SMAPI。
- 这不是新增常驻容器，也不需要用户开放新端口；容器运行完自动删除。目的是稳定访问 Docker named volume，并复用 JunimoServer 镜像里的 Linux 运行环境。
- 默认下载源写入实例 `.env`：`SMAPI_VERSION=4.5.2`，`SMAPI_DOWNLOAD_URLS=https://gh.llkk.cc/... , https://github.dpik.top/... , https://ghfast.top/... , https://github.com/...`。可在 `.env` 中覆盖为自建 OSS/CDN 地址。
- 离线/企业部署若希望完全避免现场 GitHub 下载，建议把 SMAPI installer zip 放到自有对象存储/CDN，并把 `SMAPI_DOWNLOAD_URLS` 改为自有地址优先。

# ENV-BOM-NORMALIZE-1 Compose 启动前配置校验

- 实例 `.env` 若被外部编辑器或复制粘贴插入 UTF-8 BOM 前缀，Docker Compose 会在解析阶段报 `unexpected character "\ufeff"`，表现为面板任务只显示 `docker compose up: docker command failed`。
- 后端已在 `.env` 读取/写回时归一化 BOM 前缀 key；部署排障时仍建议先执行 `docker compose -f /data/instances/<id>/docker-compose.yml config --quiet`，确认不是配置文件解析失败。
- 支持包和日志不要直接贴出 `.env` 敏感值；排查 BOM 只需要确认是否存在隐藏前缀 key。

# STEAMCMD-SELFUPDATE-CACHE-1 兜底容器自更新缓存

- SteamCMD 镜像本身仍按 `STEAMCMD_IMAGE_CANDIDATES` 选择和拉取；本地已有候选镜像时不会重新 pull。
- 镜像启动后 SteamCMD 可能输出 `[----] Downloading update (.. of 40,273 KB)`，这是容器内 SteamCMD 客户端更新，不是镜像源下载。面板日志会明确区分这一步。
- SteamCMD 客户端自更新目录已持久化到实例命名卷：`<project>_steamcmd-root-local:/root/.local/share/Steam` 和 `<project>_steamcmd-user-local:/home/steam/.local/share/Steam`。后续重试授权/下载应复用该缓存，减少重复 40MB 自更新。
- 离线或预热部署仍建议预先准备 `STEAM_SERVICE_IMAGE`、`sdvd/server:<IMAGE_VERSION>` 以及 `STEAMCMD_IMAGE_CANDIDATES` 中至少一个可用 SteamCMD 镜像。

# STEAMCMD-RETRY-RESUME-1 本地镜像优先

- SteamCMD 兜底镜像选择现在先 inspect 完整 `STEAMCMD_IMAGE_CANDIDATES` 列表；只要任意候选镜像已在本机 Docker 中存在，就直接用于 SteamCMD 兜底容器，不会先尝试拉取排在它前面但本地缺失的候选。
- 这意味着用户已经成功拉完 `docker.1ms.run/steamcmd/steamcmd:latest` 或其他候选后，后续因 Steam Guard 手机批准超时而重试安装时，会直接进入 SteamCMD 登录授权环节，不会重复下载该镜像。
- 如果所有候选镜像都不存在，仍按候选顺序依次 pull，并通过 `steamcmd_image_pulling` phase 和 `[pull:progress:done:total]` 日志给前端估算进度。

# 镜像构建文档

## SteamCMD 兜底镜像

- 面板运行镜像本身仍是单 Panel Docker 镜像，但安装 Stardew 时可能额外拉取 SteamCMD 作为 steam-auth 下载失败后的兜底工具镜像。
- 默认值在实例 `.env` 中写入：`STEAMCMD_IMAGE=docker.1ms.run/steamcmd/steamcmd:latest`，`STEAMCMD_IMAGE_CANDIDATES=docker.1ms.run/steamcmd/steamcmd:latest,docker.m.daocloud.io/steamcmd/steamcmd:latest,ghcr.io/steamcmd/steamcmd:latest,cm2network/steamcmd:latest`。后端会按候选列表逐个 `inspect/pull`，前一个镜像源 403 或超时后继续尝试下一个；旧实例如果仍是旧候选列表，安装时会补齐新候选并过滤直连 Docker Hub 的 `steamcmd/steamcmd:latest` 和已移除的 `docker.xuanyuan.me/steamcmd/steamcmd:latest`。单次镜像拉取默认等待 30 分钟，避免大镜像在慢链路下已经拉完层但尚未返回成功就被误判超时。
- SteamCMD 镜像不是 `docker-compose.yml` 里的 Junimo service；后端通过 Docker CLI/API 临时运行 TTY 容器，并挂载 `game-data`、`steamcmd-login`、`steamcmd-home` 命名卷。镜像缺失时会先执行单镜像拉取；候选全部失败时安装 phase 为 `steamcmd_image_pull_failed`。
- 发布或离线部署时，如果希望完全避免现场拉取，需要预先准备 `STEAM_SERVICE_IMAGE`、`sdvd/server:<IMAGE_VERSION>` 和 `STEAMCMD_IMAGE_CANDIDATES` 中至少一个可用的 SteamCMD 镜像。

## 构建目标

项目发布为单个 Panel Docker 镜像，镜像内包含：

- Go 后端二进制。
- React/Vite 构建产物并嵌入后端。
- docker CLI。
- docker compose plugin。
- 必要 CA、时区和运行工具。

运行时通过挂载宿主机 Docker Socket 控制 JunimoServer 容器。

## 构建上下文排除

- `.dockerignore` 已显式排除 `docs/prototypes/`，历史原型图不应进入 Docker 构建上下文或镜像产物。
- 当前 Dockerfile 也采用精确 `COPY frontend`、`COPY backend`、`COPY browser-extensions` 的方式，不依赖 `COPY .`。后续如调整 Dockerfile，仍需确认文档、原型图、本地构建产物、`node_modules` 不会进入运行镜像。

## 构建镜像

```powershell
cd E:\stardew-server-anxi-panel
docker build -t stardew-server-anxi-panel:local .
```

多阶段流程：

1. `frontend-builder`: `node:22-alpine`，执行 `npm install` 和 `npm run build`。
2. `extension-builder`: `alpine:3.20`，安装构建期 `zip`，把 `browser-extensions/nexus-slow-installer` 预打包为 `browser-extensions/anxi-nexus-installer.zip`。
3. `backend-builder`: `golang:1.25-alpine`，复制前端 dist 到 `internal/static/frontend_dist/`，`CGO_ENABLED=0 go build`。
4. `runtime`: `alpine:3.20`，只安装 docker CLI / compose plugin、CA 与时区数据，复制 `/app/panel` 和 extension-builder 的浏览器扩展产物。

## 构建带版本号镜像

```powershell
$commit = git rev-parse --short HEAD
$date = (Get-Date -AsUTC -Format 'yyyy-MM-ddTHH:mm:ssZ')

docker build -t stardew-server-anxi-panel:1.0.0 `
  --build-arg VERSION=1.0.0 `
  --build-arg COMMIT=$commit `
  --build-arg BUILD_DATE=$date .
```

版本信息会出现在：

```text
GET /health
GET /api/version
```

## 运行容器

```powershell
docker run -d `
  --name anxi-panel `
  -p 8090:8090 `
  -v /var/run/docker.sock:/var/run/docker.sock `
  -v anxi-panel-data:/data `
  stardew-server-anxi-panel:local
```

访问：

```text
http://localhost:8090
```

Windows Docker Desktop 使用 WSL2 后端时，socket 仍按 `/var/run/docker.sock` 挂载；面板控制的容器运行在 Docker Desktop/WSL2 环境中。

## Docker Compose 部署

```powershell
cd E:\stardew-server-anxi-panel\deploy
docker compose up -d
```

## NAS / 图形化 Compose 部署

- NAS（飞牛、群晖、绿联、威联通等）用户可通过图形化 Docker / Container Manager / Compose / 项目 / 应用栈部署，不要求使用 `run.sh`。
- 面板容器必须挂载宿主机 Docker Socket：`/var/run/docker.sock:/var/run/docker.sock`。如果 NAS 图形界面禁止挂载 Docker Socket，面板无法继续创建 JunimoServer、SteamCMD 等游戏容器。
- NAS 部署推荐使用宿主机真实绝对路径挂载数据目录，并让容器内 `PANEL_DATA_DIR` 与宿主机路径保持一致。例如：

```yaml
services:
  anxi-panel:
    image: crpi-9z3bkb9g7fxeohrg.cn-hangzhou.personal.cr.aliyuncs.com/anxi-panel/stardew-server-anxi-panel:latest
    container_name: anxi-panel
    restart: unless-stopped
    ports:
      - "8090:8090"
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock
      - /vol1/1000/docker/anxi-panel/data:/vol1/1000/docker/anxi-panel/data
    environment:
      PANEL_ADDR: ":8090"
      PANEL_DATA_DIR: "/vol1/1000/docker/anxi-panel/data"
      PANEL_DB_PATH: "/vol1/1000/docker/anxi-panel/data/panel.db"
      PANEL_MODE: "single"
      PANEL_SECRET: "please-change-to-a-long-random-string"
```

- 上例中的 `/vol1/1000/docker/anxi-panel/data` 只是示例路径，实际部署时必须替换成 NAS 图形界面显示的宿主机绝对路径，并保持 volume 左右路径和 `PANEL_DATA_DIR` 一致。
- 上例只在 `anxi-panel` 服务里绑定 `8090`，因为它只是面板容器。`24642/udp`、`27015/udp` 和 `5800/tcp` 由面板后续创建的 JunimoServer 游戏容器绑定，不要写进 `anxi-panel` 的 `ports`，否则面板容器会提前占用游戏端口。
- NAS 防火墙/路由器端口：面板 `TCP 8090`，游戏 `UDP 24642`，查询 `UDP 27015`，VNC `TCP 5800` 按需；不要开放 `TCP 8080`。
- 低配 NAS 口径：i3 M380 / 2 核 4 线程 / 6 GB DDR3 / HDD 可跑 1-2 人自用，3-4 人原版或少量 Mod 可尝试，5 人以上或大量 Mod 不建议。

## 系统要求与安全组

最低系统要求：

```text
系统：Linux x86_64
发行版：Ubuntu 20.04+ / Debian 11+ / CentOS 8+ / Rocky Linux 8+ / AlmaLinux 8+ / Alibaba Cloud Linux 3+
Docker：Docker Engine 24+
Compose：Docker Compose plugin v2+
CPU：2 核
内存：2 GB
磁盘：20 GB 可用空间
网络：公网 IP
端口：TCP 8090，UDP 24642 / 27015
```

推荐配置：

```text
系统：Ubuntu 22.04 LTS / Ubuntu 24.04 LTS / Debian 12 / Alibaba Cloud Linux 3
CPU：2 核以上
内存：4 GB 以上
磁盘：40 GB SSD 以上
带宽：5 Mbps 以上
Docker：Docker Engine 25+ / 26+ / 27+
```

多人游玩推荐：

```text
1-2 人：2 核 2 GB，建议开启 2 GB swap
3-4 人：2 核 4 GB
5-8 人：4 核 8 GB
大量 Mod：4 核 8 GB 起步，磁盘 60 GB+
```

云服务器安全组：

```text
必须开放：
TCP 8090
UDP 24642
UDP 27015

按需开放：
TCP 5800

不要开放：
TCP 8080
```

`TCP 8080` 是 Junimo API，供面板和容器网络内部访问，不需要也不建议公网开放。

## 一键启动脚本（推荐给用户）

面向普通 Linux 云服务器用户，优先推荐使用 `deploy/run.sh` 的快速模式。默认部署方式是公网 IP + `8090` 端口直接访问面板，用户只需要在云服务器安全组中放行对应端口。脚本会在用户主目录生成运行目录：

```text
~/.anxi-panel
├─ .env
├─ docker-compose.yml
└─ data/
```

默认行为：

- 默认面板端口：`8090`。
- 默认访问方式：`http://服务器IP:8090`。
- 默认镜像 tag：`latest`，便于新用户快速启动；正式服可通过 `PANEL_VERSION=0.1.0` 固定版本。
- 首次启动时会选择镜像源：自动候选、国内阿里云 ACR、Docker Hub 加速链路、DaoCloud 加速链路、GitHub GHCR、Docker Hub 官方，或自定义完整镜像地址；默认推荐自动候选。
- 面板镜像拉取复用后端候选镜像思路：先检查本地是否已有任意候选镜像；本地没有时按候选顺序逐个 `docker pull`，第一个成功的镜像会写回 `~/.anxi-panel/.env` 的 `PANEL_IMAGE`。
- 自动生成强随机 `PANEL_SECRET` 并写入 `~/.anxi-panel/.env`。
- 使用宿主机目录 `~/.anxi-panel/data` 持久化面板数据，并把容器内 `PANEL_DATA_DIR` 设置为同一个绝对路径，确保面板容器通过宿主机 Docker socket 编排游戏容器时，bind mount 路径在宿主机和面板容器中一致。
- 挂载 `/var/run/docker.sock`，让面板继续按现有设计控制 JunimoServer 容器。
- NAS 或特殊 Linux 环境中，如果 `$HOME` 不存在或不可写，默认安装目录会回退到当前可写目录下的 `.anxi-panel`，避免飞牛等系统中 `/home/<user>` 不存在时 `mkdir` 失败。用户也可以显式设置 `INSTALL_DIR=/vol1/1000/docker/.anxi-panel` 指定安装目录。
- 菜单 `[9] 设置虚拟内存` 会优先通过 `/proc/swaps` 判断 `/swapfile` 是否已启用，并兼容 `swapon` / `mkswap` 位于 `/sbin` 或 `/usr/sbin` 的 NAS 环境；如已有 `/swapfile` 但未启用，会先尝试移除后重建，避免直接覆盖导致 `Text file busy`。

用户首次启动：

国内加速安装：

```bash
curl -fsSL -o run.sh https://anxinas.dpdns.org/run.sh && chmod +x run.sh && bash run.sh
```

GitHub Release 安装：

```bash
curl -fsSL -o run.sh https://github.com/anxiyizhi/stardew-server-anxi-panel/releases/latest/download/run.sh && chmod +x run.sh && bash run.sh
```

如果直接从仓库文件启动：

```bash
cd deploy
chmod +x run.sh
bash run.sh
```

菜单入口：

```text
[0] 拉取并启动面板
[1] 安装/修复 Docker 与 Compose
[2] 启动/恢复面板
[3] 停止面板
[4] 重启面板
[5] 更新面板镜像并重建容器
[6] 强制更新面板镜像
[7] 切换镜像源/加速节点
[8] 更新 run.sh 启动脚本
[9] 设置虚拟内存
[10] 设置开机自启
[11] 查看面板状态
[12] 查看面板日志
[13] 显示访问地址
[14] 退出
```

非交互命令：

```bash
bash run.sh install
bash run.sh stop
bash run.sh restart
bash run.sh update
bash run.sh status
bash run.sh logs
bash run.sh docker
bash run.sh force-update
bash run.sh switch-image
bash run.sh update-script
bash run.sh swap 2
bash run.sh autostart
```

更新面板：

```bash
cd ~ && bash run.sh update
```

如果更新后仍显示旧版本，强制重新拉取镜像并重建容器：

```bash
cd ~ && bash run.sh force-update
```

如果启动脚本本身也有更新，先更新脚本再更新面板：

```bash
cd ~ && bash run.sh update-script
cd ~ && bash run.sh update
```

更新面板只会重建面板容器，不会删除 `~/.anxi-panel/data`，存档、Mod、数据库和备份会继续保留。

固定版本启动示例：

```bash
PANEL_VERSION=0.1.0 PANEL_PORT=8090 bash run.sh install
```

改用 Docker Hub 优先：

```bash
DEFAULT_MIRROR=dockerhub bash run.sh install
```

改用 GitHub GHCR：

```bash
DEFAULT_MIRROR=ghcr bash run.sh install
```

注意：

- 脚本支持自动安装/修复 Docker Engine 与 Docker Compose plugin。Ubuntu/Debian 使用阿里云 Docker CE apt 源；CentOS/RHEL/Anolis/Rocky/Alibaba Cloud Linux 类系统使用阿里云 Docker CE yum/dnf 源。无法识别的发行版仍需手动安装 Docker。
- 如果云服务器外部无法访问面板，优先检查安全组/防火墙是否放行 TCP `8090`。
- Stardew 游戏本身还需要按实例配置放行 UDP `24642` / `27015`；VNC/noVNC 默认 `TCP 5800`，仅需要浏览器查看游戏画面时按需放行；`TCP 8080` 是 Junimo API，不要开放公网。
- 快速模式默认使用 HTTP 明文访问，适合用户自有云服快速开服；首次进入面板后必须设置强管理员密码，不要使用默认或弱密码。
- 不要手动删除 `~/.anxi-panel/data`；该目录保存面板数据库、实例 compose、存档、mod、备份和审计日志。

## 数据目录

容器内 `/data`：

```text
/data
├─ panel.db
├─ instances
│  └─ stardew
│     ├─ docker-compose.yml
│     ├─ .env
│     ├─ .local-container
│     ├─ saves
│     └─ mods
└─ backups
```

一键脚本默认把宿主机 `~/.anxi-panel/data` 挂载到容器内同名绝对路径，保证容器重建后数据不丢，同时让宿主机 Docker daemon 能解析游戏实例的 bind mount 路径。

## 环境变量

| 变量 | 默认值 | 说明 |
| --- | --- | --- |
| `PANEL_ADDR` | `:8090` | HTTP 监听地址 |
| `PANEL_DATA_DIR` | `/data` | 数据目录 |
| `PANEL_DB_PATH` | `$PANEL_DATA_DIR/panel.db` | SQLite 路径 |
| `PANEL_SECRET` | 空 | Session secret，生产必须设置强随机值 |
| `PANEL_VERSION` | `dev` | 版本号 |
| `PANEL_COMMIT` | 空 | commit hash |
| `PANEL_BUILD_DATE` | 空 | 构建时间 |
| `PANEL_MODE` | `single` | 当前默认单游戏模式 |
| `DEFAULT_INSTANCE_ID` | `stardew` | 默认实例 |
| `DEFAULT_DRIVER_ID` | `stardew_junimo` | 默认 driver |

## 镜像内工具验证

```powershell
docker exec anxi-panel docker version
docker exec anxi-panel docker compose version
curl http://localhost:8090/health
curl http://localhost:8090/api/version
```

## 冒烟测试

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\smoke-test.ps1
```

可选参数：

- `-SkipDocker`
- `-SkipFrontend`
- `-SkipBackend`

## 发布检查清单

发布前至少确认：

- `go test ./...` 通过。
- `npm run build` 通过。
- `docker build` 成功。
- 镜像内 `docker version` 和 `docker compose version` 正常。
- 全新空 volume 能初始化管理员。
- 旧数据目录升级不丢 saves/mods/backups/audit logs。
- 未登录 API 返回 401，普通用户访问管理员接口返回 403。
- 安装、启动、停止、重启、邀请码刷新可用。
- 存档上传预览、提交启动、删除备份、恢复可用。
- Mod 上传、删除、导出可用。
- 在 Mod 下载页用管理员账号配置 Nexus API Key 后，数字 ID 精确查询可用；未配置时返回 `nexus_api_key_missing` 而不是 500。普通关键词搜索不要求 Key。
- 健康检查和支持包导出可用且脱敏。
- 320px 以上窄屏无横向溢出。

## 安全说明

挂载 Docker Socket 等同于授予面板容器高权限 Docker 控制能力。当前用户入口按快速模式设计，默认通过 `http://服务器IP:8090` 直接访问。上线说明里应强调：

- 这是给用户自有云服开游戏服务器的管理面板，不建议多人共用同一台宿主机。
- 使用强 `PANEL_SECRET`。
- 初始化管理员必须使用强密码。
- 只放行必要端口：面板 TCP `8090`，游戏 UDP `24642` / `27015`，VNC/noVNC 默认 TCP `5800` 按需放行；不要开放 Junimo API 的 TCP `8080`。
- 定期查看审计日志。
- 支持包和日志确认无密码、token、session、邀请码明文。

## 常见问题

面向普通用户的完整版本见 [故障排查指南](user-guide/troubleshooting.md)；本节只保留和镜像构建/本地开发直接相关的条目。

### 镜像拉取失败或 403

检查 Docker Desktop 镜像源配置，必要时临时移除不可用镜像源。

### 容器内找不到 docker

检查 Dockerfile runtime 阶段是否安装 `docker-cli`。

### `docker compose` 不可用

检查 runtime 阶段是否安装 `docker-cli-compose`。

### 面板无法连接 Docker daemon

确认启动时挂载：

```text
-v /var/run/docker.sock:/var/run/docker.sock
```

### 端口 8090 被占用

改用其他宿主机端口：

```powershell
docker run -d -p 9090:8090 ...
```
# NEXUS-EXT-PACK-1 镜像内扩展资源

- Runtime 镜像现在会从 `extension-builder` 复制 `browser-extensions/` 到 `/app/browser-extensions/`。
- `anxi-nexus-installer.zip` 在 `extension-builder` 阶段生成；最终 runtime 不再安装 `zip`，也不在运行层执行打包命令。
- 后端 `GET /api/instances/:id/mods/nexus/extension/download` 会优先返回实例目录已有的 `.local-container/browser-extensions/anxi-nexus-installer.zip`；不存在时优先复制镜像预包；预包不存在或损坏时，才从 `/app/browser-extensions/nexus-slow-installer` 或开发环境仓库路径生成。
- 发布检查新增注意：正式镜像内应存在 `/app/browser-extensions/anxi-nexus-installer.zip`；兜底源码目录 `/app/browser-extensions/nexus-slow-installer/manifest.json` 也应保留，避免预包损坏时无法恢复。
# PULL-PROGRESS-1 镜像拉取百分比

- 拉取过程中，后端会把 Docker 输出折算成估算百分比：compose pull 按服务镜像完成数估算，SteamCMD 单镜像 pull 按 layer 完成数估算，并通过 job 日志隐藏标记 `[pull:progress:done:total]` 供前端展示。
# JUNIMO-IMAGE-CANDIDATES-1 运行期 Junimo 镜像候选

- 安装 Stardew 时，面板运行镜像会额外拉取/使用 `steam-auth-cn` 与 `JunimoServer server` 运行期镜像。二者已支持候选兜底，不再只依赖 `docker compose pull` 的单一源。
- 默认 `SERVER_IMAGE_CANDIDATES`：`docker.1ms.run/sdvd/server:<IMAGE_VERSION>`、`docker.m.daocloud.io/sdvd/server:<IMAGE_VERSION>`、`ghcr.io/sdvd/server:<IMAGE_VERSION>`、`sdvd/server:<IMAGE_VERSION>`。
- 默认 `STEAM_SERVICE_IMAGE_CANDIDATES`：`docker.1ms.run/anxiyizhi/junimo-steam-service-cn:1.5.0-anxi.2`、`crpi-9z3bkb9g7fxeohrg.cn-hangzhou.personal.cr.aliyuncs.com/anxi-panel/junimo-steam-service-cn:1.5.0-anxi.2`、`docker.m.daocloud.io/anxiyizhi/junimo-steam-service-cn:1.5.0-anxi.2`、`ghcr.io/anxiyizhi/junimo-steam-service-cn:1.5.0-anxi.2`、`anxiyizhi/junimo-steam-service-cn:1.5.0-anxi.2`。
- 离线或内网发布时，可预先 `docker pull` 上述任意候选，或在实例 `.env` 中把可用内网镜像写入 `SERVER_IMAGE_CANDIDATES` / `STEAM_SERVICE_IMAGE_CANDIDATES`。后端会优先复用本地已有候选，并把实际选中项写回 `SERVER_IMAGE` / `STEAM_SERVICE_IMAGE`。
- 注意：`ghcr.io/sdvd/server:*` 与 `ghcr.io/anxiyizhi/junimo-steam-service-cn:*` 只有在对应 GHCR 包真实发布且可公开拉取时才会成功；失败会自动继续后续候选。
# JUNIMO-IMAGE-CANDIDATES-2 Junimo 镜像候选源补齐

- JunimoServer 与 steam-auth cn 镜像不再依赖 `docker compose pull` 的单源解析；后端逐个 `inspect/pull` 候选镜像，成功后写回 `.env` 的 `SERVER_IMAGE` / `STEAM_SERVICE_IMAGE`。
- 旧实例如果已经保存了单值 `SERVER_IMAGE_CANDIDATES` 或 `STEAM_SERVICE_IMAGE_CANDIDATES`，安装流程会自动把默认候选源补到前面并写回 `.env`，避免只尝试 `(1/1)`。
- JunimoServer 默认顺序：`docker.1ms.run/sdvd/server:<IMAGE_VERSION>`、`docker.m.daocloud.io/sdvd/server:<IMAGE_VERSION>`、`ghcr.io/sdvd/server:<IMAGE_VERSION>`、`sdvd/server:<IMAGE_VERSION>`。
- steam-auth cn 默认顺序：`docker.1ms.run/anxiyizhi/junimo-steam-service-cn:1.5.0-anxi.2`、`crpi-9z3bkb9g7fxeohrg.cn-hangzhou.personal.cr.aliyuncs.com/anxi-panel/junimo-steam-service-cn:1.5.0-anxi.2`、`docker.m.daocloud.io/anxiyizhi/junimo-steam-service-cn:1.5.0-anxi.2`、`ghcr.io/anxiyizhi/junimo-steam-service-cn:1.5.0-anxi.2`、`anxiyizhi/junimo-steam-service-cn:1.5.0-anxi.2`。
- 发布或离线部署时，预拉上述任意候选即可；本地已有候选会优先复用，不会因为排在前面的候选缺失而重新拉取。
# RELEASE-TAG-CI-1 GitHub Tag 发版

- 面板仓库新增 `.github/workflows/release.yml`：推送 `v*` tag 时自动构建 `Dockerfile`，并推送到 Docker Hub、阿里云 ACR 与 GitHub GHCR。
- Git tag 使用 `v0.1.0` 形式；Docker 镜像 tag 会去掉前缀 `v`，发布为 `0.1.0`，同时更新 `latest`。
- 发布目标：
  - `anxiyizhi/stardew-server-anxi-panel:<version>` 与 `:latest`
  - `crpi-9z3bkb9g7fxeohrg.cn-hangzhou.personal.cr.aliyuncs.com/anxi-panel/stardew-server-anxi-panel:<version>` 与 `:latest`
  - `ghcr.io/anxiyizhi/stardew-server-anxi-panel:<version>` 与 `:latest`
- GitHub Release 会自动生成 release notes，并上传 `deploy/run.sh`，供用户一键下载启动。
- 仓库 secrets 需要配置：`DOCKERHUB_USERNAME`、`DOCKERHUB_TOKEN`、`ALIYUN_REGISTRY_USERNAME`、`ALIYUN_REGISTRY_PASSWORD`。GHCR 使用 GitHub Actions 自动注入的 `GITHUB_TOKEN`，workflow 需要 `packages: write` 权限；首次发布后如果包是私有，需要在 GitHub Package settings 中改为 Public。阿里云 ACR 新版个人版实例必须使用控制台“访问凭证”里显示的登录名和域名；当前实例域名为 `crpi-9z3bkb9g7fxeohrg.cn-hangzhou.personal.cr.aliyuncs.com`，`ALIYUN_REGISTRY_USERNAME` 填控制台命令 `docker login --username=...` 里的值，例如 `安西义之`。
# REAL-INSTANCE-STEAM-IMAGE-FALLBACK-VERIFIED-1 真实环境验证

- Steam/SteamCMD 授权流程与镜像候选降级已经在真实环境验证通过：候选源不可用时会继续尝试后续镜像，本地已有候选可直接复用，授权状态能够继续安装流程。
- 本标记取代相关历史章节的待验证说明；具体候选顺序与配置方式仍以 `JUNIMO-IMAGE-CANDIDATES-*`、`STEAMCMD-*` 小节为准。
# RELEASE-v0.1.12 新服务器存档页空列表修复

- 发布版本：`v0.1.12`，补丁修复全新实例尚无备份时进入存档页黑屏、无法创建首个存档的问题。
- 发布前验证：后端相关测试、前端生产构建、Docker 镜像构建；推送 tag 后由 `.github/workflows/release.yml` 发布三个镜像仓库的 `0.1.12` 与 `latest`。
- 升级后验证：全新数据目录完成安装但未生成备份时，进入“存档”应显示正常空态和新建入口；`GET .../saves/backups` 应返回 `backups: []`。
# RELEASE-v0.1.13 安装运行文件完整性校验

- 发布版本：`v0.1.13`，修复新服务器 `game-data` 卷仅创建 Steam 目录但无游戏文件时仍显示“安装完成”、允许创建游戏的问题。
- 发布验证：后端 `go test ./...`、前端 `npm run build`、Docker 镜像构建；CI 构建完成后检查三个镜像仓库的 `0.1.13` 与 `latest`。
- 升级验证：现有误判实例刷新面板后应显示“游戏运行文件不完整，请重新安装或修复”；执行安装/修复后，仅在 Stardew、SMAPI 与 Steam SDK 必需文件全部存在时才会进入创建存档流程。
