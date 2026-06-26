# Conversation Handoff 2026-06-25

## TTY 修复：Steam Guard 方式选择

### 问题

`docker compose run -i steam-auth download` 以管道方式提供 stdin，
.NET 的 `Console.ReadKey()` 无法工作，导致 Steam Guard 方式选择菜单（[1] 手机/[2] 验证码）
的"选2"点了没反应，60 秒后自动选了选项1。

### 最终方案：平台分离

| 平台 | 方案 |
|------|------|
| Linux（生产） | `creack/pty` 创建宿主 PTY，`docker compose run --tty` 在 PTY slave 上运行，Docker CLI 看到真实终端，容器获得真正 TTY |
| Windows（开发） | 通过 `go-winio` 拨号 Docker 命名管道，直接调用 Docker Engine HTTP API 创建 `Tty:true` 容器，绕过 Docker CLI 的终端检查 |

**注**：早期尝试过 `creack/pty` 的 Windows ConPTY 路径，报 `unsupported`；
也尝试过全量 `docker/docker` SDK，拖入了 OpenTelemetry 等大量间接依赖，已放弃。

### 修改文件

| 文件 | 改动 |
|------|------|
| `backend/go.mod` | 添加 `github.com/creack/pty v1.1.21`（Linux）和 `github.com/Microsoft/go-winio v0.6.2`（Windows）；移除了 `docker/docker` SDK |
| `backend/internal/docker/tty_run.go` | 公共类型 `SteamAuthRunOpts` + `RunSteamAuthTTY` 分发到平台实现 |
| `backend/internal/docker/tty_run_unix.go` | `//go:build !windows` — creack/pty 实现 |
| `backend/internal/docker/tty_run_windows.go` | `//go:build windows` — 原生 Docker Engine API 实现（go-winio 命名管道） |
| `backend/internal/games/stardew_junimo/driver.go` | `DockerService` 接口加入 `RunSteamAuthTTY` |
| `backend/internal/games/stardew_junimo/installer.go` | `runSteamAuthAttempt` 替换 exec.Command+io.Pipe 为 `RunSteamAuthTTY`；新增 `buildSteamAuthOpts()` |
| `backend/internal/games/stardew_junimo/driver_test.go` | `fakeDocker` 加入 `RunSteamAuthTTY` stub |
| `backend/internal/web/install_handlers.go` | 菜单选择（ReadKey）不加 `\n`，验证码输入（ReadLine）加 `\n` |
| `frontend/src/App.tsx` | 恢复"输入邮箱验证码"按钮；加 `handleAuthMethodSelect`；`onAuthMethodSelect` prop |

### 关键逻辑

**stdin 换行规则**（在 install_handlers.go 里判断）：
- phase = `steam_guard_choice_required` / `auth_method_required` → 发 `"2"` (无 `\n`，ReadKey 单字符)
- 其他 phase（验证码）→ 发 `code + "\n"` (ReadLine 需要换行)

**Windows 命名管道地址**：`\\.\pipe\docker_engine`（Docker Desktop 默认）

**ANSI 剥离**：TTY 输出包含转义码，`stripANSI()` 已处理（streaming.go 同包可见）。

### 如何验证

1. 启动面板，点"安装游戏"
2. Steam Guard 方式选择阶段会看到"输入邮箱验证码"按钮
3. 点击后 steam-auth 容器选择了选项 2，过渡到验证码输入 phase
4. 输入验证码后 Steam Guard 通过，继续安装

编译与测试：`go build ./...` + `go test ./...` 均通过（无 docker 守护进程也可测试）

### 下一步注意事项

- Windows 生产场景未测试（生产跑 Linux，无需 Windows 路径）
- Docker Desktop 命名管道名在企业版可能不同（`docker_engine_windows` 等），出错时检查
- 如果 steam-auth 未来改变菜单格式（"choose authentication method" 字符串），需要同步更新 installer.go 的 lineHandler
- `exitCode == 139`（SIGSEGV）的检测保留着，TTY 模式下不应再出现，但没有副作用

---

## stdin 换行 Bug 修复（2026-06-25 补充）

### 问题

点击"输入邮箱验证码"后，容器依旧 60 秒后自动选了 [1]，任务日志出现 `Choice [1]: 2`。

### 根因

`install_handlers.go` 原先对菜单选择不加 `\n`（假设容器用 `Console.ReadKey()`，单字符不需要换行）。
实际上 JunimoServer 的方式选择菜单用的是 `Console.ReadLine()`，且 PTY 处于 **canonical 模式**：PTY 行规程必须收到 `\n` 才会把输入交给应用。
没有 `\n` 时，`"2"` 被 PTY 缓冲住，容器的 ReadLine 一直阻塞，60 秒超时后容器自动选 [1]，此时 PTY 把缓存的 `"2"` 回显出来，就出现了 `Choice [1]: 2` 的输出。

诊断依据：加了 `[panel:stdin]` 日志确认字节确实写入了 Docker 连接，但容器没有响应，说明字节卡在 PTY canonical 模式缓冲区里。

### 修复

`install_handlers.go`：所有输入（包括菜单选择）统一追加 `\n`：

```go
// 旧：菜单选择不加 \n
input := body.Input
if instance.DriverPhase != "steam_guard_choice_required" && ... {
    input += "\n"
}

// 新：一律加 \n
input := body.Input + "\n"
```

### 其他改动（同次调试）

| 文件 | 改动 |
|------|------|
| `backend/internal/docker/tty_run_windows.go` | stdin goroutine 加日志：成功写入打印 `[panel:stdin] sent N byte(s)`，写入失败打印错误并退出 |
| `backend/internal/web/install_handlers.go` | handler 开头加 `steam guard input received` server log（含 phase 字段方便排查） |
| `frontend/src/App.tsx` | `handleAuthMethodSelect` / `handleGuardSubmit` 里 jobId 为空时改为显示错误，不再静默失败 |

### 下一步注意事项

- `[panel:stdin]` 诊断日志可在确认流程稳定后移除（或保留作运维信息）
- 如果未来 Steam Guard 验证码输入也出现类似问题（容器不响应），同样原因——确认 `input + "\n"` 已覆盖
