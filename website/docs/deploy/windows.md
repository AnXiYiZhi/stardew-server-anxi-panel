# Windows + Docker Desktop

Anxi Panel 可以在 x86_64 Windows 10/11 上运行，但运行方式不是原生 `.exe`、Windows Service 或 Windows containers，而是：

```text
Windows 10/11
└─ WSL2 Linux 发行版（推荐 Ubuntu）
   └─ Docker Desktop 的 Linux containers
      └─ Anxi Panel + JunimoServer
```

::: warning 更适合个人电脑测试或轻量自用
Docker Desktop 必须保持运行，Windows 休眠、重启或退出 Docker Desktop 都会中断服务器。需要长期 24 小时稳定运行时，仍优先推荐 Linux 云服务器或 NAS。
:::

## 准备条件

```text
系统：Docker Desktop 当前支持的 64 位 Windows 10/11
项目架构：x86_64（Windows ARM 设备暂不支持）
虚拟化：在 BIOS/UEFI 中启用，并能正常运行 WSL2
WSL：WSL2 2.1.5+，建议保持最新
容器模式：Linux containers
内存：至少 8 GB 物理内存，建议为 WSL/Docker 留出 4 GB 以上
磁盘：至少 40 GB 可用空间
```

具体 Windows 版本与硬件要求以 [Docker Desktop 官方系统要求](https://docs.docker.com/desktop/setup/install/windows-install/#system-requirements) 为准。Docker 官方要求 WSL 至少为 2.1.5，并建议始终使用最新版。

## 1. 安装或更新 WSL2

如果电脑还没有 WSL，请以管理员身份打开 PowerShell，运行：

```powershell
wsl --install
```

安装完成后按提示重启 Windows。该命令默认会安装一个 Linux 发行版；不同系统上的实际发行版以 PowerShell 输出为准。完整说明见 [Microsoft WSL 安装文档](https://learn.microsoft.com/en-us/windows/wsl/install)。

如果已经安装过 WSL，先更新并检查版本：

```powershell
wsl --update
wsl --version
wsl -l -v
```

`wsl -l -v` 中准备使用的发行版必须显示 `VERSION 2`。如果显示为 `1`，将下面的 `Ubuntu` 换成列表里的真实发行版名称后运行：

```powershell
wsl --set-version Ubuntu 2
```

首次打开该 Linux 发行版时，按提示创建 Linux 用户名和密码。这个密码与 Windows 登录密码相互独立，之后在 Linux 终端执行 `sudo` 时会使用。

## 2. 安装和设置 Docker Desktop

1. 从 [Docker 官方页面](https://docs.docker.com/desktop/setup/install/windows-install/) 下载并安装 Docker Desktop。
2. 启动 Docker Desktop，等待左下角或托盘状态显示引擎已经运行。
3. 打开 **Settings → General**，启用 **Use the WSL 2 based engine**。
4. 打开 **Settings → Resources → WSL Integration**，为刚才实际使用的 Linux 发行版开启集成。
5. 确认 Docker Desktop 正在使用 **Linux containers**。如果托盘菜单显示 **Switch to Linux containers...**，点击它完成切换。

::: danger 不要使用 Windows containers
Anxi Panel、JunimoServer 和配套镜像都是 Linux 容器。切到 Windows containers 后，脚本即使能找到 Docker，也无法正常启动这些镜像。
:::

## 3. 在 WSL2 中验证 Docker

从开始菜单打开已经启用 WSL Integration 的 Linux 发行版。下面所有 Linux 命令都在这个终端运行，不要直接粘贴到 PowerShell 或命令提示符。

```bash
uname -m
docker version
docker compose version
docker info
```

检查结果：

- `uname -m` 应显示 `x86_64`。
- `docker version` 应同时显示 Client 和 Server，不能只有 Client。
- `docker compose version` 应正常显示 Compose V2 版本。
- `docker info` 的容器系统应为 Linux，不能连接到 Windows containers。

如果提示 `docker: command not found`，回到 Docker Desktop 的 **WSL Integration** 开启当前发行版，然后关闭并重新打开 Linux 终端。如果提示无法连接 daemon，先确认 Docker Desktop 已经启动完成。

## 4. 选择正确的安装目录

先在 WSL2 Linux 终端运行：

```bash
cd ~
pwd
```

路径通常类似 `/home/你的用户名`。后续脚本会把部署文件和数据保存到：

```text
~/.anxi-panel/
├─ .env
├─ docker-compose.yml
└─ data/
```

不要在 `/mnt/c`、`/mnt/d` 等 Windows 盘符挂载目录中部署。根据 [Docker Desktop 的 WSL2 文件系统建议](https://docs.docker.com/desktop/features/wsl/best-practices/)，Linux 容器使用的 bind mount 数据放在 WSL2 Linux 文件系统中，性能和文件事件兼容性都更好。

::: warning 不要混用两套路径
部署后继续在同一个 WSL2 发行版中运行更新命令。不要一会儿从 Ubuntu 的 `~/.anxi-panel` 操作，一会儿又从 PowerShell 或另一个发行版重新部署。
:::

## 5. 一键部署 Anxi Panel

仍在 WSL2 Linux 终端中运行官方 GitHub Release 脚本：

```bash
cd ~
curl -fsSL -o run.sh https://github.com/anxiyizhi/stardew-server-anxi-panel/releases/latest/download/run.sh && chmod +x run.sh && bash run.sh
```

::: tip 国内加速脚本（HTTP）
GitHub Release 下载较慢时，可以在同一个 WSL2 Linux 终端使用国内加速地址：

```bash
cd ~
curl -fsSL -o run.sh http://anxinas.dpdns.org/run.sh && chmod +x run.sh && bash run.sh
```
:::

脚本打开菜单后，选择 `0` 执行“下载/检查环境并启动面板”。Docker Desktop 已经提供 Docker Engine，不需要在 WSL2 中再安装第二套 Docker daemon。

部署完成后，在当前 Windows 电脑的浏览器打开：

```text
http://localhost:8090
```

首次进入后创建管理员账号，再按 [首次进入面板](/guide/first-login) 完成游戏安装和存档设置。

## 6. 局域网和外网访问

同一台 Windows 电脑访问面板时使用 `localhost:8090`。局域网内的手机或其它电脑需要使用 Windows 主机的局域网 IPv4，可以在 PowerShell 运行：

```powershell
ipconfig
```

找到当前实际联网网卡的 IPv4，例如 `192.168.1.100`，其它设备访问：

```text
http://192.168.1.100:8090
```

不要把 WSL2 或 Docker 内部的 `172.x.x.x` 地址发给其它设备。局域网访问失败时，先检查 Windows Defender 防火墙是否允许 Docker Desktop 和对应端口。

需要让外网好友加入时，还要在 Windows 防火墙和路由器中按需放行或转发到这台 Windows 主机：

```text
TCP 8090      面板访问（建议只在可信局域网开放）
UDP 24642     Stardew 游戏端口
UDP 27015     查询端口
TCP 5800      VNC/noVNC（只有需要浏览器画面时开放）
```

不要开放 `TCP 8080`，这是 Junimo 内部 API。远程管理面板更推荐 VPN、Tailscale、ZeroTier 或路由器自带 VPN。完整说明见 [端口与安全组](/deploy/ports)。

## 7. 日常启动和更新

- 每次 Windows 重启后先启动 Docker Desktop，等待 Docker Engine 就绪，再访问面板。
- 关闭终端窗口不会停止已经运行的容器，但退出 Docker Desktop、Windows 休眠或关机会停止服务。
- 面板和存档数据保存在 WSL2 的 `~/.anxi-panel`，不要把 WSL 发行版当作临时环境删除。
- 更新优先使用面板内的一键升级；面板无法工作时再看 [更新面板](/maintain/update) 的命令行方法。
- 更新 Windows、Docker Desktop、WSL 或大型 Mod 前，先在面板中创建一次完整存档备份。

## 常见问题

### `wsl --version` 不显示版本详情

通常表示 WSL 版本较旧。在管理员 PowerShell 运行 `wsl --update`，按提示重启后再检查。Docker Desktop 官方建议 WSL 至少为 2.1.5。

### `wsl -l -v` 显示 VERSION 1

使用 `wsl --set-version <发行版名称> 2` 转换到 WSL2，然后在 Docker Desktop 中重新开启该发行版的 WSL Integration。

### WSL2 中找不到 `docker` 命令

确认 Docker Desktop 已启动，并在 **Settings → Resources → WSL Integration** 中开启当前发行版。保存设置后重新打开 WSL2 终端。

### `docker version` 只有 Client 或提示无法连接

Docker Desktop 的 Linux Engine 尚未运行。启动 Docker Desktop 并等待完成；如果刚修改过 WSL Integration，重新打开终端后再检查。不要在 WSL2 中另外启动一套独立 Docker daemon。

### `localhost:8090` 能打开，但手机打不开

手机不能使用 `localhost`。改用 Windows 实际联网网卡的局域网 IPv4，并检查 Windows Defender 防火墙；不要使用 WSL2/Docker 的 `172.x.x.x` 内部地址。

### Windows 重启后面板打不开

先启动 Docker Desktop 并等待引擎就绪。容器会按重启策略恢复；如果仍未恢复，进入原 WSL2 发行版和原安装目录，再按 [更新与维护说明](/maintain/update) 检查状态，不要在另一个目录重新部署第二套。

更多问题见 [常见问题](/faq/)。
