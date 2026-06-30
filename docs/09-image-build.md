# 镜像构建文档

## 构建目标

项目发布为单个 Panel Docker 镜像，镜像内包含：

- Go 后端二进制。
- React/Vite 构建产物并嵌入后端。
- docker CLI。
- docker compose plugin。
- 必要 CA、时区和运行工具。

运行时通过挂载宿主机 Docker Socket 控制 JunimoServer 容器。

## 构建镜像

```powershell
cd E:\stardew-server-anxi-panel
docker build -t stardew-server-anxi-panel:local .
```

多阶段流程：

1. `frontend-builder`: `node:22-alpine`，执行 `npm install` 和 `npm run build`。
2. `backend-builder`: `golang:1.25-alpine`，复制前端 dist 到 `internal/static/frontend_dist/`，`CGO_ENABLED=0 go build`。
3. `runtime`: `alpine:3.20`，安装 docker CLI / compose plugin，复制 `/app/panel`。

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

使用 named volume 保证容器重建后数据不丢。

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
- 健康检查和支持包导出可用且脱敏。
- 320px 以上窄屏无横向溢出。

## 安全说明

挂载 Docker Socket 等同于授予面板容器高权限 Docker 控制能力。建议：

- 仅在可信内网使用。
- 不直接暴露公网。
- 使用强 `PANEL_SECRET`。
- 配置反向代理和 HTTPS。
- 定期查看审计日志。
- 支持包和日志确认无密码、token、session、邀请码明文。

## 常见问题

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
