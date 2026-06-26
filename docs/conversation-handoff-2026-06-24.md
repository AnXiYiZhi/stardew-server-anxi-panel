# Conversation Handoff 2026-06-24

## Milestone 6 补丁：docker compose pull 实时进度流

### 问题现象

点击安装游戏后，后端执行 `docker compose pull` 拉取 Junimo 镜像期间（可能持续数分钟），前端任务日志窗口完全空白，只有静态文字"正在拉取 JunimoServer 镜像..."，用户无法判断进度是否卡住。

### 根因

旧的 `ComposePull` 通过 `runner.go` 的 `c.run()` 执行，使用 `cmd.Run()` 等待命令完成后一次性返回 stdout/stderr 缓冲，期间没有任何增量输出写入 job log。

### 改了什么

**后端新增文件：**

- `backend/internal/docker/streaming.go`
  - 新增 `ComposePullStreaming(ctx, dir, lineHandler)` 方法。
  - 使用 `io.Pipe` + `bufio.Scanner` 逐行读取 `docker compose pull --progress=plain` 的 stdout/stderr 合并输出。
  - 每行经过 `stripANSI`（去除 ANSI 转义序列和 `\r`）和 `RedactString`（脱敏）后调用 `lineHandler`，空行跳过。
  - `--progress=plain` 抑制 TUI 风格的进度条，改为每层操作单独输出一行，适合日志流。
  - 超时、工作目录校验、退出码处理逻辑和 `runner.go` 保持一致。

**后端修改文件：**

- `backend/internal/games/stardew_junimo/driver.go`
  - `DockerService` 接口：将 `ComposePull` 替换为 `ComposePullStreaming(ctx, dir, lineHandler func(string)) (CommandResult, error)`。

- `backend/internal/games/stardew_junimo/installer.go`
  - Step 2（docker compose pull）改为调用 `ComposePullStreaming`，每行以 `[pull] ` 前缀写入 `jobCtx.Info`。
  - 错误处理中移除了对已缓冲 `pullResult.Stdout/Stderr` 的二次日志（输出已实时写入 job log，无需重复记录）。
  - 移除了不再使用的 `paneldocker` import。

- `backend/internal/games/stardew_junimo/driver_test.go`
  - `fakeDocker` 更新：`ComposePull` 方法替换为 `ComposePullStreaming`，成功时调用 lineHandler 模拟两条 pull 输出行。

### 影响的文件

```text
backend/internal/docker/streaming.go            （新增）
backend/internal/games/stardew_junimo/driver.go （接口变更）
backend/internal/games/stardew_junimo/installer.go
backend/internal/games/stardew_junimo/driver_test.go
```

### 无影响的内容

- `ComposePull` 方法仍保留在 `compose.go`（仅用于 `compose_test.go` 的集成测试），未被删除。
- 前端无任何改动：任务日志窗口已经通过 SSE 实时接收后端写入的每条 `jobCtx.Info`，pull 进度会自动出现在日志区域。
- phase 状态机（`pull_running` / `pull_failed` / 35% 进度条）不变。

### 如何验证

```bash
cd backend && go test ./...   # 通过
cd frontend && npm run build  # 通过
```

端到端验证：

1. 删除本地 `sdvd/steam-service` 和 `sdvd/server` 镜像（`docker rmi sdvd/steam-service:... sdvd/server:...`）。
2. 打开安装游戏 Modal，填写凭据，点击确认。
3. 任务日志窗口应实时出现 `[pull] steam-auth Pulling`、`[pull] 0abc123: Pulling fs layer`、`[pull] 0abc123: Downloading [==>...] 5.12MB/100MB` 等逐层进度行。
4. pull 完成后进入 Steam 认证流程，日志继续实时输出。
5. 确认 job log 中没有 STEAM_PASSWORD / VNC_PASSWORD 明文。

---

## Milestone 6 补丁：docker compose pull 实时进度条

### 问题

用户安装时点击"安装游戏"，拉取镜像期间任务展示框内无任何可视进度，只有静态文字。

实际上还有两个问题被一并修复：
1. 第一次尝试用 `--progress=plain` 导致命令直接失败（该 flag 在用户的旧版 Docker Compose 不支持）。
2. 移除该 flag 后，输出可以流式传输，但缺少视觉进度条。

### 修复一：移除 `--progress=plain`

`streaming.go` 里的 `ComposePullStreaming` 去掉了 `--progress=plain` flag，直接使用 `docker compose pull`。非 TTY 管道环境下 Docker Compose 会自动使用 plain 输出，`stripANSI` 负责清理残余转义序列。

### 修复二：进度条（后端 + 前端）

**后端：**

- `installer.go` 新增包级正则 `regexPullCount`、`regexPullStart`、`regexPullDone`，新增 `makePullLineHandler(jobCtx)` 函数。
- 该函数返回一个有状态的行处理器（通过闭包持有 `total`/`done`/`lastEmit`）：
  - 每行写入 job log（带 `[pull] ` 前缀，如之前）。
  - 同时尝试两种格式解析进度：
    - 现代格式：`[+] Pulling X/Y` → `done=X, total=Y`
    - 旧格式：`Pulling <service> (...)` 计数 total；`Status: Downloaded/up to date` 计数 done。
  - 每次 done 或 total 变化时，向 job log 写一条特殊行 `[pull:progress:X:Y]`（X=完成数，Y=总数）。
- `installer.go` 的 pull 步骤改为调用 `makePullLineHandler` 生成的处理器。
- import 新增 `regexp` 和 `strconv`。

**前端：**

- `App.tsx` 新增 `pullProgressRe` 正则和 `extractPullProgress(logs, jobType)` 函数，扫描所有日志找最后一条 `[pull:progress:X:Y]`，返回 `{done, total, percent}` 或 `null`。
- `JobsSection` 的 job-detail 渲染：
  - 在日志窗口上方插入进度条区域（仅当 `extractPullProgress` 有结果时）。
  - 进度条复用现有 `progress-bar-wrap` / `progress-bar-track` / `progress-bar-fill` / `progress-bar-percent` 样式，完成时自动变绿（`done` class）。
  - 日志列表过滤掉 `[pull:progress:X:Y]` 行（避免原始进度行污染日志显示）。
- `App.css` 新增 `.pull-progress-container` 和 `.pull-progress-header`，与卡片风格一致（米色背景、棕色边框）。

### 涉及文件

```text
backend/internal/docker/streaming.go              （移除 --progress=plain）
backend/internal/games/stardew_junimo/installer.go （新增 makePullLineHandler）
frontend/src/App.tsx                               （进度条 UI + 过滤）
frontend/src/App.css                               （进度条样式）
```

### 如何验证

```bash
cd backend && go test ./...   # 通过
cd frontend && npm run build  # 通过
```

端到端：
1. 删除本地 Junimo 镜像，启动安装。
2. 点击任务中心里的 `stardew_install` 任务。
3. 拉取镜像期间应在日志窗口上方出现"拉取镜像 0/3 服务 ── 0%"进度条，随着服务完成逐步更新至 100%（完成变绿）。
4. 日志列表显示 `[pull] ...` 行的实时输出，但不显示内部进度行。

### 下一步注意事项

- `ComposePull`（批量缓冲版）仍在 `compose.go` 中，但 driver 层不再使用；如其他地方也不需要，可后续删除。
- `makePullLineHandler` 的两种解析格式已覆盖现代和旧版 Docker Compose V2 管道输出。若日后遇到其他格式，在 `regexPullCount` / `regexPullStart` / `regexPullDone` 里扩展即可。
- Milestone 7 实现 Server Lifecycle（start/stop/restart/status/invite-code）。

---

## Milestone 6 补丁：InstallSection 用户友好的 pull 状态卡

### 问题

进度条和日志进度条都在任务中心（JobsSection）里，但任务中心是开发调试工具。普通用户在安装区域（InstallSection）看到的是静态的"正在拉取 JunimoServer 镜像..."文字，无法判断是网络慢、下载失败还是软件卡死。

### 解法

**后端（`installer.go`）：**

- `makePullLineHandler` 增加 `onProgress func(done, total int)` 参数。
- 每次进度变化时，除写入 `[pull:progress:X:Y]` 日志行外，同步调用 `onProgress`。
- 在 `run` 方法里，`onProgress` 回调调用 `updatePhase`，把 `stateMessage` 更新为人类可读的内容，例如：
  - `"正在拉取 Junimo 镜像（1/3 完成），首次下载可能需要 10-30 分钟，请耐心等待..."`
  - `"所有 3 个镜像已拉取完成，准备启动 Steam 认证..."`
- 这样 `InstanceStateCard` 每 2.5 秒轮询一次后，状态卡里的文字会自动更新。

**前端（`App.tsx`）：**

- `Dashboard` 向 `InstallSection` 新增两个 props：
  - `stateMessage`：来自 `instanceState?.stateMessage`，后端 pull 时会随进度更新。
  - `pullProgress`：`extractPullProgress(jobLogs, selectedJob?.type)` 的结果（之前只在 JobsSection 内部使用）。
- `InstallSection` 在 pull 阶段（`phase === 'pull_running' && isInstalling`）显示专用状态卡：
  - 弹跳动画的下载箭头图标，表示"还在动，没卡死"。
  - 大字 `stateMessage`（来自后端），实时显示已完成 X/Y 镜像。
  - 镜像进度条（来自 `pullProgress`），独立于总安装进度条。
  - 提示文字："首次下载约需 10–30 分钟……如果超过 15 分钟仍无变化，请检查网络后重试。"
- 原来进度条下方的静态标签 `progress.label` 在 pull 阶段隐藏（卡片已替代它）。

**前端（`App.css`）：**

- 新增 `.pull-status-card`、`.pull-status-header`、`.pull-status-spinner`（带 `pull-bounce` 动画）、`.pull-images-bar`、`.pull-images-bar-track`、`.pull-images-bar-fill`、`.pull-images-bar-label`、`.pull-images-waiting`、`.pull-status-hint`。

### 涉及文件

```text
backend/internal/games/stardew_junimo/installer.go  （makePullLineHandler 增加 onProgress 回调）
frontend/src/App.tsx                                （新 props + pull 状态卡）
frontend/src/App.css                                （pull 状态卡样式）
```

### 如何验证

```bash
cd backend && go test ./...   # 通过
cd frontend && npm run build  # 通过
```

端到端：
1. 删除本地 Junimo 镜像，启动安装。
2. **不用打开任务中心**，直接观察安装区域（InstallSection）。
3. 进入 pull 阶段后，应看到带弹跳动画的"↓ 正在下载 JunimoServer 镜像"卡片。
4. 后端每完成一个镜像，卡片文字（stateMessage）更新为"正在拉取 (1/3)..."，进度条随之推进。
5. 所有镜像完成后文字变"所有 X 个镜像已拉取完成，准备启动 Steam 认证..."，卡片消失，进入 steam_auth 阶段。

### 下一步注意事项

- Milestone 7 实现 Server Lifecycle（start/stop/restart/status/invite-code）。
