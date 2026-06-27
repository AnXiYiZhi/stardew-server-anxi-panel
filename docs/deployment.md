# 部署指南

本文档说明如何构建和运行 Anxi Panel Docker 镜像。

## 前置要求

- Docker Engine 20.10+（含 Compose V2 插件）
- 或 Docker Desktop 4.x+

验证 Docker 和 Compose 可用：

```bash
docker version
docker compose version
```

## 构建镜像

在项目根目录执行：

```bash
docker build -t stardew-server-anxi-panel:local .
```

构建过程为多阶段：

1. **frontend-builder**：`node:22-alpine`，执行 `npm install` + `npm run build`，产出 `dist/`。
2. **backend-builder**：`golang:1.25-alpine`，将前端 `dist/` 复制到 `internal/static/frontend_dist/`，执行 `CGO_ENABLED=0 go build`。
3. **runtime**：`alpine:3.20`，包含后端二进制、docker CLI、docker-cli-compose。

### 构建带版本号镜像

通过 `--build-arg` 注入版本信息，这些信息会在 `/health` 和 `/api/version` 中返回：

```bash
# Linux / macOS
docker build -t stardew-server-anxi-panel:1.0.0 \
  --build-arg VERSION=1.0.0 \
  --build-arg COMMIT=$(git rev-parse --short HEAD) \
  --build-arg BUILD_DATE=$(date -u +%Y-%m-%dT%H:%M:%SZ) .
```

```powershell
# Windows PowerShell
$commit = git rev-parse --short HEAD
$date = (Get-Date -AsUTC -Format 'yyyy-MM-ddTHH:mm:ssZ')

docker build -t stardew-server-anxi-panel:1.0.0 `
  --build-arg VERSION=1.0.0 `
  --build-arg COMMIT=$commit `
  --build-arg BUILD_DATE=$date .
```

## 运行容器

### Linux / macOS

```bash
docker run -d \
  --name anxi-panel \
  -p 8090:8090 \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -v anxi-panel-data:/data \
  stardew-server-anxi-panel:local
```

### Windows（Docker Desktop）

Docker Desktop 的 socket 路径为 `//var/run/docker.sock`（通过 WSL2 转发），命令与 Linux 相同：

```powershell
docker run -d `
  --name anxi-panel `
  -p 8090:8090 `
  -v /var/run/docker.sock:/var/run/docker.sock `
  -v anxi-panel-data:/data `
  stardew-server-anxi-panel:local
```

> **注意**：Windows Docker Desktop 使用 WSL2 后端时，挂载的 socket 实际是 WSL2 内的 Docker daemon。面板通过此 socket 控制的容器也运行在 WSL2 中，而非 Windows 原生 Hyper-V 容器。这是预期行为。

### 使用 Docker Compose

```bash
cd deploy
docker compose up -d
```

## 首次访问

1. 打开浏览器访问 `http://localhost:8090`。
2. 首次进入会显示管理员初始化注册页。
3. 输入管理员用户名和密码，完成初始化。
4. 进入 Stardew 面板，点击"安装游戏"开始。

## 数据持久化

面板所有数据存储在容器内 `/data` 目录：

```text
/data
├─ panel.db              # SQLite 数据库（用户、session、任务、状态）
├─ instances
│  └─ stardew            # Stardew 实例目录
│     ├─ docker-compose.yml
│     ├─ .env
│     ├─ .local-container/
│     ├─ saves/
│     └─ mods/
└─ backups/              # 预留备份目录
```

使用 named volume（如 `anxi-panel-data`）挂载 `/data` 可确保数据在容器重建后保留。

## 环境变量

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `PANEL_ADDR` | `:8090` | HTTP 监听地址 |
| `PANEL_DATA_DIR` | `/data` | 数据目录 |
| `PANEL_DB_PATH` | `$PANEL_DATA_DIR/panel.db` | SQLite 路径 |
| `PANEL_SECRET` | 空 | Session secret（生产环境必须设置） |
| `PANEL_VERSION` | `dev` | 版本标识（构建镜像时通过 `--build-arg VERSION` 注入更佳） |
| `PANEL_COMMIT` | 空 | Git commit hash（构建时自动注入） |
| `PANEL_BUILD_DATE` | 空 | 构建时间（构建时自动注入） |
| `PANEL_MODE` | `single` | `single` 或 `multi` |
| `DEFAULT_INSTANCE_ID` | `stardew` | 默认实例 ID |
| `DEFAULT_DRIVER_ID` | `stardew_junimo` | 默认 driver ID |

## 健康检查

容器内置 `HEALTHCHECK`，每 30 秒请求 `GET /health`。

手动检查：

```bash
curl http://localhost:8090/health
```

响应示例：

```json
{
  "status": "ok",
  "service": "stardew-anxi-panel",
  "version": "1.0.0",
  "commit": "abc1234",
  "buildDate": "2026-06-27T12:00:00Z",
  "database": {
    "status": "ok"
  }
}
```

版本信息端点：

```bash
curl http://localhost:8090/api/version
```

响应示例：

```json
{
  "version": "1.0.0",
  "commit": "abc1234",
  "buildDate": "2026-06-27T12:00:00Z"
}
```

## 镜像内工具验证

```bash
# 检查 Docker CLI
docker exec anxi-panel docker version

# 检查 Compose 插件
docker exec anxi-panel docker compose version
```

## 安全说明

### Docker Socket 权限

挂载 `/var/run/docker.sock` 等同于给面板容器 **root 级别的宿主机 Docker 控制权**。面板可以：

- 创建、启动、停止、删除任意容器
- 读取任意容器日志
- 管理 Docker volumes 和 networks

**建议**：

- 仅在受信任的内网环境中运行面板。
- 不要将面板端口暴露到公网，除非配合反向代理和 HTTPS。
- 使用 `PANEL_SECRET` 设置强随机字符串。
- 定期审计面板用户和操作日志。

### 敏感信息

面板已对以下信息做脱敏处理，不会写入日志或 API 响应：

- Steam 密码
- VNC 密码
- Session token

## 常见排错

### 容器内找不到 docker

```bash
docker exec anxi-panel docker version
```

如果报 `docker: not found`，说明镜像构建时未正确安装 docker-cli。检查 Dockerfile 的 `apk add docker-cli` 步骤。

### docker compose 不可用

```bash
docker exec anxi-panel docker compose version
```

如果报 `unknown command "compose"`，说明缺少 compose 插件。检查 Dockerfile 的 `apk add docker-cli-compose` 步骤。

### 没挂载 Docker Socket

面板启动后访问 Docker 状态 API 返回错误，或无法创建 Junimo 容器：

```json
{
  "error": {
    "code": "docker_command_failed",
    "message": "Cannot connect to the Docker daemon"
  }
}
```

确保运行容器时挂载了 socket：

```bash
-v /var/run/docker.sock:/var/run/docker.sock
```

### /data 权限问题

如果面板无法写入 `/data`，检查 volume 是否正确挂载：

```bash
docker inspect anxi-panel --format '{{json .Mounts}}' | jq
```

### 端口 8090 被占用

```bash
# Linux
lsof -i :8090

# Windows
netstat -ano | findstr :8090
```

更换映射端口：

```bash
docker run -d -p 9090:8090 ...
```

## 升级

1. 拉取或构建新版本镜像。
2. 停止旧容器：`docker stop anxi-panel`。
3. 删除旧容器：`docker rm anxi-panel`。
4. 用相同 volume 挂载启动新容器。

数据卷 `anxi-panel-data` 不会被删除，所有面板数据保留。

## 诊断与排错

### 导出诊断包

管理员可以在面板高级设置 → 健康检查区域点击「导出诊断包」，下载一个包含以下信息的 ZIP 文件：

- 版本信息
- 健康检查结果
- 实例状态
- 最近任务摘要
- 最近审计日志
- Docker Compose PS 结果
- docker-compose.yml 配置（已脱敏）
- 服务器日志尾部（已脱敏）

导出内容已脱敏，不包含密码、token、session 等敏感信息。

### 冒烟测试

在本地开发环境验证后端测试、前端构建和 Docker 镜像构建：

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\smoke-test.ps1
```

可选参数：
- `-SkipDocker`：跳过 Docker 镜像构建和容器测试
- `-SkipFrontend`：跳过前端构建
- `-SkipBackend`：跳过后端测试
