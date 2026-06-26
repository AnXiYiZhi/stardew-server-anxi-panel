# New PC Development And Test Environment

本文档用于在新电脑上尽量复刻当前 `stardew-server-anxi-panel` 的开发、构建、测试环境。

目标：

```text
在新电脑上可以启动后端、启动前端、运行测试、连接 Docker、继续开发 Milestone 8+。
```

## Current Machine Baseline

当前机器检测到的版本：

```text
OS: Microsoft Windows NT 10.0.26100.0
PowerShell: 5.1.26100.8655
Go: go1.26.4 windows/amd64
Node.js: v22.22.2
npm: 10.9.7
Git: 2.53.0.windows.2
Docker CLI: 29.5.3
Docker API: 1.54
Docker Compose: v5.1.4
```

Docker 当前检测说明：

```text
docker compose version 可以正常返回 v5.1.4。
docker version 能读到 CLI 版本，但当前这次检测 Docker daemon 连接报 permission denied。
新电脑需要确保 Docker Desktop 已启动，并且当前 Windows 用户可以访问 Docker daemon。
```

项目 Go module：

```text
backend/go.mod: go 1.25.0
当前实际 Go 工具链: 1.26.4
```

前端 package-lock 精确版本：

```text
@vitejs/plugin-react: 6.0.2
vite: 8.0.16
typescript: 6.0.3
react: 19.2.7
react-dom: 19.2.7
@types/react: 19.2.17
@types/react-dom: 19.2.3
lockfileVersion: 3
```

## Recommended Software

新电脑建议安装：

```text
Windows 11 24H2 or newer
PowerShell 5.1+，Windows 自带即可
Git for Windows 2.53.x
Go 1.26.4，或至少 Go 1.25+
Node.js 22.22.x LTS/current compatible
npm 10.9.x
Docker Desktop with Docker Engine 29.x
Docker Compose v5.x
Microsoft Edge or Google Chrome
```

最低要求：

```text
Docker Engine 20+
Docker Compose V2+
Go 1.25+
Node.js 22+
npm 10+
```

推荐尽量使用当前版本，避免 Vite / TypeScript / Go 行为差异。

## Required Tools

### 1. Git

安装：

```text
Git for Windows
```

验证：

```powershell
git --version
```

期望接近：

```text
git version 2.53.0.windows.2
```

### 2. Go

安装 Go 1.26.4 或至少 Go 1.25+。

验证：

```powershell
go version
```

期望接近：

```text
go version go1.26.4 windows/amd64
```

建议在本项目测试时使用项目内 Go cache，避免 Windows 用户目录权限问题：

```powershell
$env:GOCACHE="E:\stardew-server-anxi-panel\.gocache"
```

### 3. Node.js And npm

安装 Node.js 22.22.x。

验证：

```powershell
node -v
npm.cmd -v
```

期望接近：

```text
v22.22.2
10.9.7
```

注意：

```text
在 PowerShell 中优先使用 npm.cmd，而不是 npm。
```

原因：

```text
Windows PowerShell 可能因为 ExecutionPolicy 禁止执行 npm.ps1。
使用 npm.cmd 可以绕开这个问题。
```

### 4. Docker Desktop

安装 Docker Desktop。

建议：

```text
Use WSL2 backend
Enable Docker Compose V2
Start Docker Desktop before running backend Docker tests
```

验证：

```powershell
docker version
docker compose version
docker ps
```

期望接近：

```text
Docker CLI: 29.5.3
Docker Compose version v5.1.4
```

如果出现：

```text
permission denied while trying to connect to the docker API
```

检查：

```text
Docker Desktop 是否已启动
当前用户是否有 Docker 访问权限
Docker context 是否正确
C:\Users\<user>\.docker\config.json 权限是否异常
是否需要重新登录 Windows 或重启 Docker Desktop
```

### 5. Browser

安装或使用系统自带：

```text
Microsoft Edge
Google Chrome
```

前端开发默认地址通常是：

```text
http://localhost:5173
```

后端默认地址：

```text
http://localhost:8090
```

## Recommended Directory Layout

建议新电脑保持类似路径：

```text
E:\stardew-server-anxi-panel
```

如果需要参考源码，也建议：

```text
E:\源码\JunimoServer_源码
E:\源码\emp_源码
```

但项目开发不强依赖这两个目录，除非要继续参考 Junimo 或 EMP 源码。

## Clone And Install

进入 E 盘：

```powershell
cd E:\
```

克隆项目：

```powershell
git clone <your-repo-url> stardew-server-anxi-panel
cd E:\stardew-server-anxi-panel
```

如果是从压缩包复制，确保包含：

```text
backend
frontend
docs
README.md
README.en.md
```

## Backend Development

进入后端：

```powershell
cd E:\stardew-server-anxi-panel\backend
```

建议设置本地 Go cache：

```powershell
$env:GOCACHE="E:\stardew-server-anxi-panel\.gocache"
```

运行测试：

```powershell
go test ./...
```

启动后端：

```powershell
$env:PANEL_DATA_DIR="E:\stardew-server-anxi-panel\data"
$env:PANEL_DB_PATH="E:\stardew-server-anxi-panel\data\panel.db"
$env:PANEL_MODE="single"
$env:DEFAULT_INSTANCE_ID="stardew"
$env:DEFAULT_DRIVER_ID="stardew_junimo"
go run ./cmd/panel
```

后端默认监听：

```text
:8090
```

健康检查：

```text
http://localhost:8090/health
```

## Frontend Development

进入前端：

```powershell
cd E:\stardew-server-anxi-panel\frontend
```

安装依赖：

```powershell
npm.cmd install
```

启动开发服务器：

```powershell
npm.cmd run dev
```

前端通常打开：

```text
http://localhost:5173
```

构建：

```powershell
npm.cmd run build
```

预览：

```powershell
npm.cmd run preview
```

## Project Environment Variables

后端主要环境变量：

```text
PANEL_ADDR=:8090
PANEL_DATA_DIR=/data
PANEL_DB_PATH=$PANEL_DATA_DIR/panel.db
PANEL_SECRET=
PANEL_VERSION=dev
PANEL_MODE=single
DEFAULT_INSTANCE_ID=stardew
DEFAULT_DRIVER_ID=stardew_junimo
```

本地 Windows 推荐：

```powershell
$env:PANEL_DATA_DIR="E:\stardew-server-anxi-panel\data"
$env:PANEL_DB_PATH="E:\stardew-server-anxi-panel\data\panel.db"
$env:PANEL_MODE="single"
$env:DEFAULT_INSTANCE_ID="stardew"
$env:DEFAULT_DRIVER_ID="stardew_junimo"
```

生产环境必须设置强随机：

```text
PANEL_SECRET
```

## Current Product Mode

当前产品模式必须保持：

```text
Single Game Mode
```

也就是：

```text
登录后直接进入 Stardew 面板
不显示总面板
不显示游戏列表
内部仍使用 instances + driver_id + GameDriver
```

默认 instance：

```text
id: stardew
driver_id: stardew_junimo
name: Stardew Valley
data_dir: $PANEL_DATA_DIR/instances/stardew
```

未来第二个游戏出现后才开启：

```text
PANEL_MODE=multi
```

## Docker And Junimo Requirements

Milestone 6+ 会需要真实 Docker 能力。

需要确保：

```text
Docker Desktop running
docker ps works
docker compose version works
current user can access Docker daemon
```

Junimo 流程需要：

```text
Docker Engine 20+
Docker Compose V2+
Steam account with Stardew Valley license
Steam Guard access
```

当前面板安装逻辑使用 JunimoServer 容器，不重新实现 Stardew 服务端。

## Development Test Checklist

### Backend

```powershell
cd E:\stardew-server-anxi-panel\backend
$env:GOCACHE="E:\stardew-server-anxi-panel\.gocache"
go test ./...
```

### Frontend

```powershell
cd E:\stardew-server-anxi-panel\frontend
npm.cmd install
npm.cmd run build
```

### Docker

```powershell
docker version
docker compose version
docker ps
```

### Full Local Dev

Terminal 1:

```powershell
cd E:\stardew-server-anxi-panel\backend
$env:GOCACHE="E:\stardew-server-anxi-panel\.gocache"
$env:PANEL_DATA_DIR="E:\stardew-server-anxi-panel\data"
$env:PANEL_DB_PATH="E:\stardew-server-anxi-panel\data\panel.db"
$env:PANEL_MODE="single"
$env:DEFAULT_INSTANCE_ID="stardew"
$env:DEFAULT_DRIVER_ID="stardew_junimo"
go run ./cmd/panel
```

Terminal 2:

```powershell
cd E:\stardew-server-anxi-panel\frontend
npm.cmd run dev
```

Browser:

```text
http://localhost:5173
```

Expected:

```text
首次进入管理员初始化
登录后直达 Stardew 面板
不显示总面板游戏列表
```

## Common Problems

### npm.ps1 cannot be loaded

问题：

```text
npm : 无法加载文件 ... npm.ps1，因为在此系统上禁止运行脚本
```

解决：

```powershell
npm.cmd install
npm.cmd run dev
npm.cmd run build
```

### Go build cache access denied

问题：

```text
open C:\Users\<user>\AppData\Local\go-build\... Access is denied
```

解决：

```powershell
$env:GOCACHE="E:\stardew-server-anxi-panel\.gocache"
go test ./...
```

### Docker daemon permission denied

问题：

```text
permission denied while trying to connect to the docker API
```

检查：

```text
Docker Desktop 是否已启动
Windows 用户权限
Docker context
C:\Users\<user>\.docker\config.json 文件权限
是否需要重启 Docker Desktop 或 Windows
```

### Frontend cannot call backend

检查：

```text
后端是否运行在 http://localhost:8090
前端 Vite 是否运行在 http://localhost:5173
frontend/vite.config.ts 是否代理 /api 和 /health 到 8090
浏览器控制台是否有 CORS / proxy 错误
```

## Version Pinning Notes

当前 `frontend/package.json` 使用 `latest`，但 `package-lock.json` 已锁定实际版本。

新电脑建议：

```text
使用 npm.cmd install，不要删除 package-lock.json。
```

如果删除 lock file，可能安装到更新的 Vite / React / TypeScript，导致行为和当前机器不同。

Go 后端以 `go.mod` 和 `go.sum` 为准。

## Do Not Change On New PC

新电脑搭环境时不要顺手改这些产品决策：

```text
不要提前显示总面板
不要把 PANEL_MODE 默认改成 multi
不要把 Stardew 逻辑挪出 games/stardew_junimo
不要允许前端提交任意 shell
不要在日志中输出 Steam 密码 / VNC 密码 / session token
不要删除 package-lock.json
```

## Useful Docs

继续开发前阅读：

```text
README.md
docs/architecture.md
docs/handoff-roadmap.md
stardew-anxi-panel-conversation-summary-for-prompts.md
```

E 盘根目录也应有：

```text
E:\stardew-anxi-panel-conversation-summary-for-prompts.md
```

