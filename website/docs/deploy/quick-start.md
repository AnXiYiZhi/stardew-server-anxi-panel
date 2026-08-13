# 一键脚本部署

Linux 云服务器和 NAS 用户都优先使用一键启动脚本 `run.sh`。NAS 用户请先在系统设置中开启 SSH，再登录 NAS 终端运行本页命令。脚本会生成 `~/.anxi-panel/.env`、`docker-compose.yml` 和 `~/.anxi-panel/data`，自动创建 `PANEL_SECRET`，首次启动时可自动选择可用镜像源，也可手动切换阿里云 ACR、Docker Hub 加速链路、DaoCloud、GHCR、Docker Hub 官方或自定义镜像地址。

Windows 10/11 用户也使用同一脚本，但必须先完成 WSL2、Docker Desktop、Linux containers 和 WSL Integration 设置，并从 WSL2 的 Linux 终端运行；不要直接在 PowerShell 中执行 Bash 命令。完整步骤见 [Windows + Docker Desktop](/deploy/windows)。项目不提供原生 Windows `.exe`。

NAS 或特殊 Linux 环境中，如果 `$HOME` 不存在或不可写，脚本会自动把安装目录放到当前可写目录下的 `.anxi-panel`，例如在 `/vol1/1000/docker` 执行时会使用 `/vol1/1000/docker/.anxi-panel`。

只有已经非常熟悉 NAS 的 Docker 图形界面，并且能够自行核对宿主机路径、Docker Socket、端口、挂载和环境变量时，才建议改用 [NAS 图形化部署（进阶）](/deploy/nas)。

默认访问方式：

```text
http://服务器IP:8090
```

## 安装

官方 GitHub Release 安装（推荐）：

```bash
curl -fsSL -o run.sh https://github.com/anxiyizhi/stardew-server-anxi-panel/releases/latest/download/run.sh && chmod +x run.sh && bash run.sh
```

::: tip 国内加速脚本（HTTP）
GitHub Release 下载较慢时，可以使用国内加速地址：

```bash
curl -fsSL -o run.sh http://anxinas.dpdns.org/run.sh && chmod +x run.sh && bash run.sh
```
:::

Windows 用户完成后访问 `http://localhost:8090`。建议把 `~/.anxi-panel` 保存在 WSL2 的 Linux 文件系统中，并保持 Docker Desktop 运行；不要从 PowerShell 直接执行这份 Bash 脚本。

无论使用哪个脚本下载入口，面板镜像仍由脚本自动从阿里云 ACR、Docker Hub 加速链路、DaoCloud、GHCR 或 Docker Hub 候选源拉取。

固定版本安装：

```bash
PANEL_VERSION=0.1.0 PANEL_PORT=8090 bash run.sh install
```

## 脚本菜单功能

脚本菜单支持 Docker/Compose 安装修复、镜像候选兜底、启动、停止、重启、普通更新、强制更新、镜像源切换、脚本自更新、虚拟内存、开机自启、状态、日志和访问地址。

## 更新面板

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

## 下一步

- 开放端口说明：看 [端口与安全组](/deploy/ports)。
- 打开面板后要做什么：看 [首次进入面板](/guide/first-login)。
