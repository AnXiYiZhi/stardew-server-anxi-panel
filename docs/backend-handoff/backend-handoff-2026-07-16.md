# 2026-07-16 后端接手：required 125 与 Auth 解耦

## 改了什么

- runtime manifest 新增 `runtimeUpdatePolicy=required`。新 Panel 启动后，仅对已经安装且属于可信旧矩阵的实例自动复用 repair、dry-run、apply、snapshot、rollback 链路，将 JunimoServer 121 升到精确 125；全新实例仍只在管理员点击安装后开始安装。
- Auth 硬验收只要求目标镜像/容器健康和 `/steam/ready` JSON 契约可解析。真实 `1.5.0-anxi.2` 未配置账号返回 `{"ready":false,"error":"Account 0 not configured"}`，没有 `has_ticket`；该状态允许 LAN-only 升级成功，ticket、登录和邀请码只作为在线能力。
- Windows TTY runner 在任务取消时显式停止一次性容器并断开 attach，修复 Steam 登录提示处取消后容器与卷残留。

## 影响文件与接口

- 升级协调：`required_runtime_update.go`、`driver.go`、`lifecycle.go`、`cmd/panel/main.go`、runtime manifest/config。
- Auth 契约与 TTY：`internal/docker/runtime_apply.go`、`internal/docker/tty_run_windows.go`。
- 公共 runtime components 响应新增 `runtimeUpdatePolicy`；现有 dry-run/apply/SMAPI API 阶段和请求体不变。

## 如何验证

- 单元与全量：`go test ./...`、`go vet ./...`、Python compatibility matrix、前端全部状态脚本与生产 build。
- 真实升级：`TestRequiredRuntimeReal121To125OptIn` 对 121 源实例/游戏卷的隔离只读副本分别验证 stopped/running；空 Steam 会话下 125/Auth、宿主 Junimo Mod、FIFO `info` 与原运行状态均通过。
- 全新安装：`TestFreshInstall125ReachesSteamLoginOptIn` 验证新 Panel Prepare 不创建容器/卷；点击安装后直接选择 125/Auth 并真实进入 QR 登录，随后取消且无容器/卷残留。

## 下一步注意事项

- required 只覆盖 JunimoServer/Auth pair，不得隐式扩大到 game、SDK 或 SMAPI。
- 自定义镜像、不可信候选和 `rollback_failed` 必须继续进入人工处理，不得自动覆盖或删除恢复材料。
- 未来 Auth 返回格式变化时可扩展契约解析，但不得重新把 `has_ticket=true`、实际登录或邀请码加入升级硬门槛。
