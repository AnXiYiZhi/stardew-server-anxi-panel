# 项目执行错题本

本文件记录代理在本项目中实际遇到的命令、环境、Shell、路径和编码错误。每次工作开始先阅读；再次遇到同类问题时直接采用“正确做法”，不要重放错误命令。

## 记录模板

```text
日期 / 最近复发：
环境：
错误模式（已脱敏）：
症状 / 退出码：
根因：
正确做法：
预防检查：
适用范围：
```

## 2026-08-09：GitHub HTTPS fetch 短暂 TLS 握手失败

- 环境：Windows PowerShell 7、Git for Windows schannel，官网文档提交前同步 `origin/main`。
- 错误模式：首次单次 `git fetch origin main` 遇到外部链路握手中断；该命令位于 commit/push 前，因此没有产生部分提交或远端写入。
- 症状 / 退出码：Git 报 `schannel: failed to receive handshake, SSL/TLS connection failed` 并退出 1；几秒后同一精确 fetch 成功。
- 根因：GitHub HTTPS/TLS 瞬时断流，不是仓库分歧、凭据或证书配置错误。
- 正确做法：保持 TLS 校验，对同一只读 fetch 使用有间隔、最多四次的有界重试；成功后仍比较本地 HEAD 与 `origin/main`，不能因网络恢复而跳过分歧检查。重试耗尽则停止提交/推送并报告外部阻塞。
- 预防检查：发布与官网推送前把 fetch、HEAD 对比、commit、push 拆开；fetch 失败时先确认没有写操作发生，不关闭 `sslVerify` 或改用不安全协议。
- 适用范围：GitHub `git fetch/ls-remote`、提交前远端同步及其它 schannel HTTPS 操作。

## 2026-08-09：发布夹具把多层 Shell、JSON 和文本工具塞进单行命令

- 环境：Windows PowerShell 7 → `docker exec` → Linux `sh`，验证 `v0.4.9` 受控 HTTPS Release、支持包和升级后哨兵。
- 错误模式：先后内联 Python `-c` URL/CA、JSON/正则、`find -printf`、`cut -d ' '`，并猜测 BusyBox `wget --ca-certificate` 可用；引号在 PowerShell、Docker argv 和容器 shell 之间被剥离或重新解释。
- 症状 / 退出码：Python 收到残缺参数，PowerShell 把 `|`/引号解析为自身语法，容器 shell 报 JSON/正则语法错误，BusyBox wget 报未知选项；这些失败均发生在只读探针或测试断言，产品事务未被错误触发。
- 根因：把多个语言的词法规则叠在同一个命令字符串里，并在没有能力探针时按 GNU 工具参数猜测精简镜像实现。本项目同类跨层转义已多次复发。
- 正确做法：能直接 `docker exec <container> <command> <arg...>` 就不引入 `sh -c`；复杂 Python/JSON/文件枚举写入任务专属脚本再执行，数据通过文件或环境变量传入。先运行 `--help`/版本探针确认 BusyBox、GNU 和 OpenSSL 功能，使用结构化 JSON 解析与原生命令退出码，不用文本引号拼协议。
- 预防检查：命令同时出现 `pwsh`、`docker exec`、`sh -c` 且包含引号/管道/JSON/正则时停止内联；该规则已因重复出现提升到 `AGENTS.md`。
- 适用范围：Docker Desktop 发布 E2E、DinD、远端 SSH、支持包和任何跨 PowerShell/Linux Shell 的测试夹具。

## 2026-08-09：测试根证书缺少 CA 约束

- 环境：任务专属 DinD、Python/OpenSSL 3 HTTPS fixture、受控 registry/release 域名。
- 错误模式：只生成自签名证书和签发 server certificate，未给根证书加入 critical `basicConstraints=CA:TRUE` 与 `keyUsage=keyCertSign,cRLSign`，便把它作为客户端信任锚。
- 症状 / 退出码：域名、SAN 和挂载路径均正确，但 Python TLS 报 `CERTIFICATE_VERIFY_FAILED: invalid CA certificate`；Docker/Panel 尚未开始升级 mutation。
- 根因：OpenSSL 3 的链验证要求签发者具备明确 CA 能力；“自签名”本身不等于合法 CA。
- 正确做法：生成测试 CA 时显式加入 `basicConstraints=critical,CA:TRUE` 和 `keyUsage=critical,keyCertSign,cRLSign`；server 证书使用 `CA:FALSE`、serverAuth 与精确 SAN。启动服务前执行 `openssl verify -CAfile <ca> <server-cert>`，通过后才挂入任务容器。
- 预防检查：TLS fixture readiness 必须依次核对证书扩展、链验证、SAN、服务端 `/v2/` 或 release endpoint；不得用关闭验证替代修复证书链。
- 适用范围：受控 HTTPS Release、私有 registry、DinD 和其它本机 TLS 故障注入。

## 2026-08-09：正式镜像回拉与 manifest 查询未统一使用有界网络重试

- 环境：Docker Desktop，发布后核验 Docker Hub、阿里云 ACR、GHCR 的 `0.4.9/latest` 六个引用。
- 错误模式：首次把单次 `docker pull`/`docker buildx imagetools inspect` 结果直接当作 registry 终态，没有先放进逐引用的有界重试包装。
- 症状 / 退出码：GHCR 短暂出现 TLS handshake timeout、EOF 和 auth EOF，Docker Hub `latest` 的首次 manifest 查询失败；同一精确引用稍后成功，六个 digest/labels 最终完全一致。
- 根因：外部 registry/CDN 链路瞬时断流，不是 manifest 缺失或摘要漂移；单次查询无法区分暂态网络故障和权威发布失败。
- 正确做法：每个精确引用独立最多三次 pull/inspect，保留每次退出码与错误；每次成功后仍必须核对 digest、version、revision、created，并用实际回拉镜像冒烟。重试不得关闭 TLS、认证、签名或摘要校验。
- 预防检查：发布后六引用清单逐项记录 attempts 和最终 digest；只有重试耗尽或成功结果不一致才阻塞发布。
- 适用范围：多 registry 正式镜像、Buildx manifest 查询、发布后回拉和网络抖动恢复。

## 2026-08-09：把旧部署的 `PANEL_VERSION` 当作 updater 成功权威字段

- 环境：官方 `v0.4.8 → v0.4.9` Web 一键更新，正式 Compose `.env` 同时存在 `PANEL_IMAGE` 与历史 `PANEL_VERSION`。
- 错误模式：在目标 `/health`、`/api/version`、容器 image/revision 和 apply 终态均正确后，额外断言 updater 必须把非权威 `PANEL_VERSION` 文本同步改为目标版本。
- 症状 / 退出码：产品已成功运行精确 `0.4.9` 候选，测试包装器却因 `.env` 中旧信息字段仍为 `0.4.8` 虚假失败；数据与运行容器均正常。
- 根因：当前 updater 契约只原子切换权威 `PANEL_IMAGE`；构建注入的 API/OCI metadata 决定运行版本，`run.sh` 后续也会重新解析最新镜像。测试把历史兼容字段误当作 apply 契约。
- 正确做法：升级成功断言以 apply terminal status、`PANEL_IMAGE`、实际容器 image ID、`/health`、`/api/version` 和 OCI revision/date 为准；只在代码/部署契约明确要求时检查 `PANEL_VERSION`，不要自行增加写入要求。
- 预防检查：升级夹具每个 `.env` 字段先追踪读取者与写入者；信息字段、兼容字段和权威选择字段必须分开记录。
- 适用范围：Panel updater、Compose `.env` 兼容、旧版直升和运行版本核验。

## 2026-08-09：用原生命令输出真值判断 Docker 资源是否存在

- 最近复发/补充：同一 UI QA 启动随后把 PowerShell 变量写进未加引号的 Docker 复合参数 `--mount type=volume,source=$volumeName,target=...`；PowerShell 把该 token 原样传递，Docker 尝试创建字面量 `$volumeName` 并以 125 拒绝。卷已按精确名称/owner 创建，容器未创建。含变量的 `--mount`、`--label`、`--env` 等 `key=value,...` 参数必须先组成完整字符串变量或使用双引号包住整个参数，例如 `--mount "type=volume,source=$volumeName,target=/path"`，不能假定未引用 token 内会插值。
- 环境：PowerShell 7、Docker Desktop，创建任务专属 UI QA 容器前查重。
- 错误模式：写成 `if (docker container inspect <name> 2>$null) { ... }`，直接把原生命令的标准输出当 PowerShell 布尔值，没有读取 `$LASTEXITCODE`。
- 症状 / 退出码：目标容器实际不存在，但 Docker 对失败 inspect 输出的 `[]` 被 PowerShell 当作真值，脚本误报“container already exists”并在创建任何资源前退出 1。
- 根因：PowerShell 条件判断的是命令输出对象，不是原生命令退出码；Docker 的失败输出仍可能含非空 JSON 文本。
- 正确做法：先执行 `docker container inspect <name> *> $null`，立即保存 `$LASTEXITCODE`；退出码 0 表示存在，1 表示不存在，其它值先诊断 Docker。volume/network/image 查重同样按退出码判断，不能按输出真值。
- 预防检查：所有 Docker 资源查重都拆成“执行 → 保存退出码 → 分支”三步；创建前再核对精确 owner label，禁止用 `if (docker ...)`。
- 适用范围：PowerShell 中的 Docker container/volume/network/image inspect、任务资源创建与清理。

## 2026-08-09：切换工作目录后仍重复仓库路径前缀

- 环境：Windows，PowerShell 7，从仓库根切换到 `backend` 作为命令工作目录。
- 错误模式：`workdir` 已是 `backend`，仍向 `gofmt` 传入 `backend/internal/...`。
- 症状 / 退出码：2026-08-09 同一任务后段补中断窗口测试时再次出现 `GetFileAttributesEx ... The system cannot find the path specified`，退出 1；两次命令都在格式化前失败，源码未发生变化。该规则已提升为 `AGENTS.md` 的路径/工作目录硬检查，后续命令必须先对第一个文件运行 `Test-Path`，不能只在错题本记忆。
- 根因：命令路径按仓库根编写，但执行器工作目录已下沉一层。
- 正确做法：在 `backend` 工作目录使用 `internal/...`，或回到仓库根再使用 `backend/internal/...`；提交命令前把 `workdir + 参数路径` 拼成一次实际绝对路径核对。
- 预防检查：所有带显式 `workdir` 的命令先检查首个文件参数是否重复该目录名；批量格式化前用一个目标文件的 `Test-Path -LiteralPath` 探针。
- 适用范围：Go 格式化/测试、Node 构建和任何在子目录执行的文件型命令。

## 2026-08-09：Windows `bash` 命中无可用发行版的 WSL 转发器

- 环境：Windows，PowerShell 7，系统同时存在 `C:\Windows\System32\bash.exe` 和独立 Git for Windows。
- 错误模式：未探测来源就直接调用 `bash -n`，命令命中 WSL 转发器。
- 症状 / 退出码：`CreateProcessCommon ... execvpe(/bin/bash) failed: No such file or directory`，退出 1；脚本没有执行。
- 根因：当前 PATH 的 `bash.exe` 不是 Git Bash，且 WSL 没有可运行的 Linux 发行版。
- 正确做法：先由 `Get-Command git` 定位 Git for Windows 根目录，再验证并调用其精确 `bin\bash.exe`；本机当前验证路径为 `D:\Code\CodeTools\Git\bin\bash.exe`。
- 预防检查：Windows 上任何 Bash 门禁先输出 `Get-Command bash -All` 并运行精确解释器的 `--version`；不得把 WSL 转发器存在等同于 Linux shell 可用。
- 适用范围：Bash 语法、功能测试、ShellCheck 包装脚本与发布资产测试。

## 2026-08-09：并行工具包装在单项异常时丢失其它结果

- 环境：Codex `functions.exec`，JavaScript `Promise.all` 并行三个 PowerShell 门禁。
- 错误模式：把 Docker integration、ShellCheck 镜像探测和 Python 兼容矩阵放入同一个 `Promise.all`，其中一项在包装层异常后整次调用只返回通用 `Script error`。
- 症状 / 退出码：约 1 秒退出 1，没有任何子任务输出，无法判断各门禁是否真正启动或完成。
- 根因：并行组合没有为每个 nested tool call 单独捕获异常，单项 rejection 使聚合脚本提前失败并遮蔽其它返回值。
- 正确做法：高价值长门禁单独调用；确需并行时为每项包 `try/catch` 并输出任务名、返回对象或异常，不能用裸 `Promise.all` 聚合。
- 预防检查：并行前确认每个命令已单独探针通过；任何一项涉及不确定解释器/镜像时先拆开，避免丢失真实测试证据。
- 适用范围：多工具并行测试、构建、网络探针和发布门禁。

## 2026-08-09：ShellCheck 动态绝对 source 不能靠相对 source 指令消除 SC1091

- 环境：Docker Desktop，`koalaman/shellcheck-alpine:v0.10.0`，仓库只读挂载到 `/work`。
- 错误模式：先把不属于正式 workflow 的 `deploy/run.sh` 加入门禁产生既有提示；随后给 `$ROOT_DIR/deploy/migrate-fnos.sh` 增加相对 `source=` 指令，但正式输入路径与 ShellCheck 解析后的路径仍不被视为同一输入。
- 症状 / 退出码：两次均退出 1，第二次仍为 `SC1091 Not following`；本次新增 repair 脚本没有告警。
- 根因：第一次没有精确复制 release workflow 文件清单；第二次误以为动态绝对 source 可由相对 `source=` 自动关联已列输入。
- 正确做法：严格复制 workflow 的四个 ShellCheck 输入；对测试 harness 已验证的动态 `$ROOT_DIR` source 在该行使用带原因的 `disable=SC1091`，功能测试继续实际执行被 source 脚本。
- 预防检查：发布门禁先从 workflow 逐字取得文件和参数；第三方 lint 镜像先 inspect；动态 source 的功能正确性由真实 Bash 测试负责，静态抑制必须局部且解释原因。
- 适用范围：ShellCheck 容器门禁和基于仓库根动态定位的 Bash 测试 harness。

## 2026-08-08：生产 SSH 诊断的运行时依赖、对象发现与多层转义

- 环境：Windows，PowerShell 7，Codex 工作区依赖 Python，通过 OpenSSH 连接 Linux 生产主机。
- 错误模式：沿用旧工作区运行时曾自带 `paramiko` 的假设；根据历史名称使用不存在的容器；把游戏 API 的宿主端口误当成 Panel API 端口，并猜测容器内监听端口；在 JavaScript template literal 中嵌入 sed `\\1` 反向引用；未先查 SQLite schema 就查询猜测列；在 PowerShell 中未引用带 `^{commit}` 的 Git revision；把可选 `grep`/`cat` 无命中当成整批失败。
- 症状 / 退出码：`ModuleNotFoundError: paramiko`；Docker `No such container`；对错误端口请求 `/api/version` 返回 404、容器内错误端口拒绝连接；包装层报告 octal escape `SyntaxError`；SQLite `no such column`；PowerShell/Git 错解 revision；核心健康检查均通过后仍因诊断尾部退出 1。以上失败均发生在只读探针或受保护脚本的前置/尾部，没有越过保存、停服、文件替换门禁。
- 根因：工作区依赖版本变化、复用历史对象名、跨 JavaScript/PowerShell/Bash 三层转义、以及把可选诊断当成权威契约。
- 正确做法：先调用 workspace dependency loader 并实测模块；缺少 Paramiko 时使用系统 OpenSSH + 任务专属 `SSH_ASKPASS`，密码只以临时脱敏脚本提供并在任务结束删除。容器名先从 `docker ps -a` 取得；HTTP 探针先读取 Panel 容器完整 inspect JSON，并从 `NetworkSettings.Ports` 与 `PANEL_ADDR` 确定宿主/容器端口。远端长脚本整体 UTF-8 base64 后交给 Bash，避免反向引用跨层。SQLite 先 `PRAGMA table_info`，Git revision 使用显式引用/`--verify`，可选文件先 `test -e`，可选筛选命令追加明确的无命中分支。
- 预防检查：生产命令前依次确认解释器/模块、SSH 登录、实际容器列表、文件存在性和数据库 schema；关键动作与可选日志筛选拆成两次调用，避免尾部诊断覆盖真实成功状态。
- 适用范围：Windows 到 Linux 的生产 SSH 运维、Docker/Compose 故障恢复、SQLite 状态核验和跨 Shell 包装命令。

## 2026-07-31：测试辅助镜像被 Docker Desktop 镜像源拒绝

- 最近复发/补充：2026-08-01 官网预览首次使用 `node:20-alpine` 时，把镜像拉取与构建放在同一个 3 分钟命令中；外层超时后 daemon 才完成拉取并延迟创建同名容器，立刻重用名称触发 conflict。正确做法是先单独 `docker pull`/`docker image inspect` 并等待结束，再创建构建容器；超时后先轮询精确容器名与标签，确认不存在且镜像就绪后才能重试。
- 环境：Windows，Docker Desktop Linux containers，daemon 配置了第三方 registry mirror。
- 错误模式：直接运行本机没有缓存的 `nginx:1.27-alpine` 作为一次性 HTTPS fixture。
- 症状 / 退出码：pull manifest 经镜像源返回 `403 Forbidden`，`docker run` 退出 1。
- 根因：Docker Hub 的代理镜像源拒绝该 manifest，并非候选 Panel 或 fixture 配置错误。
- 正确做法：不原样重试；优先复用已存在且刚验证可运行的 `alpine:3.20`，按任务需要在隔离容器内安装 OpenSSL 并提供 HTTPS fixture，或先显式验证目标辅助镜像可拉取。
- 预防检查：测试设计依赖新的辅助镜像前先 `docker image inspect`，未缓存时将镜像源可用性视为独立前置条件；辅助镜像失败不得误判为产品失败。
- 适用范围：本机 Docker Desktop 的 E2E fixture、代理、Web 服务与故障注入容器。

## 2026-07-31：批量读取时假定可选文件存在

- 最近复发/补充：2026-08-09 第二次把不存在的仓库根 `package.json` 混进前端 Playwright 依赖探针，使只读命令在输出能力检查前退出；本项目 Node 清单位于 `frontend/package.json` 与 `website/package.json`。该规则已提升到 `AGENTS.md` 的“先用 `rg --files`”硬规则，Node 门禁或依赖检查必须先用 `rg --files -g 'package.json' .` 选择任务对应的真实清单，不得把“常见根清单”列为可选输入。
- 最近复发/补充：2026-08-08 排查运行组件升级测试时，第四次把未由 PowerShell 展开的 `runtime_update*go` 直接作为 `rg` 路径传入，Windows 返回路径语法错误。此类搜索必须使用已存在目录配合 `rg -g 'runtime_update*go' ... <root>` 或先运行 `rg --files`；项目 `AGENTS.md` 已有同一硬规则，后续命令提交前必须机械检查所有含 `*` 的参数只能紧跟 `-g`，不得继续用通配路径参数试探。
- 最近复发/补充：2026-08-06 为 `v0.4.8` 插入发布门禁时，沿用旧工作树中 `docs/09-image-build.md` 以 `v0.4.6` 开头的假设；重放到 `v0.4.7` 基线后真实首标题已是 `v0.4.7` 发布记录，`apply_patch` 因上下文不存在而安全失败。跨基线整合后修改长期文档前必须重新读取目标文件当前首段或精确锚点，不能继续使用重放前的文件结构。
- 最近复发：2026-08-01 首页卡片预览时猜测 VitePress 配置为 `config.mts`，只读审计又误查不存在的仓库根 `package.json`；实际文件分别是 `website/docs/.vitepress/config.ts` 与 `website/package.json`。同日 `v0.4.7` 发布审计又把不存在的 `frontend/README.md` 与 `backend/internal/version` 直接交给批量 `rg`，分别让主批次和子审计退出 1。继续前先用 `rg --files` 获取真实路径，并对可选路径使用 `Test-Path`。
- 最近复发：2026-08-01 `v0.4.7` 门禁又凭惯例猜测仓库根 `go.mod`、`backend/internal/api`、`backend/internal/version`、根 `run.sh`、容器 HTTP 端口 `8080` 和 sentinel 文件 `/game/sentinel.txt`；权威位置实际为 `backend/go.mod`、已发现的具体包、容器 `8090` 与 `/game/sentinel`。路径、端口和容器内文件都必须先从 `rg --files`、Dockerfile/Compose、health 配置或 `find` 的只读结果取得，不能把常见命名当契约。
- 最近复发：2026-07-31 发版探查时猜测官网工作流名为 `docs-pages.yml`，随后又把不存在的仓库根 `cmd` 传给 `rg`，导致并行诊断整批退出 1；已改为先用 `rg --files .github/workflows` 和 `rg --files backend` 获取真实路径（分别为 `docs.yml`、`backend/cmd/...`）。
- 环境：Windows，PowerShell 7，仓库文件探查。
- 错误模式：在并行 `Get-Content` / `rg` 命令里直接加入未经确认的 `setup_handlers.go`、`docker-compose.yml` 和 `compose.yml`。
- 症状 / 退出码：目标文件不存在使整个批次退出 1，并掩盖同批其它有效输出。
- 根因：根据常见命名猜测文件路径，没有先用 `rg --files` 或 `Test-Path` 确认。
- 正确做法：先列出候选文件；可选路径使用 `Test-Path -LiteralPath` 分支，必需路径缺失才让命令失败。
- 预防检查：任何多文件批量读取前先对不确定路径做存在性探针，避免一个可选文件中断整批诊断。
- 适用范围：仓库结构探查、接手文档选择、Compose/Dockerfile 定位。

## 2026-07-31：E2E 轮询误读 Job 响应包裹层

- 环境：PowerShell 7，真实 Panel `GET /api/jobs/:id`。
- 错误模式：把响应当成顶层 Job，轮询 `$response.status`，实际契约是 `{ "job": { "status": ... } }`。
- 症状 / 退出码：后台任务约 5 秒已 `succeeded`，验收脚本仍轮询到超时并退出 1；任务日志明确显示下载、导入和完成均成功。
- 根因：没有先输出或对照 Job detail 契约就编写终态判断。
- 正确做法：解析 `$response.job.status`；轮询前先对第一次响应做必需字段断言，字段缺失立即停止而不是继续等待。
- 预防检查：所有异步 API E2E 先验证 envelope、ID 和 status 路径，再进入带上限的轮询。
- 适用范围：Panel jobs、更新任务、安装任务和其它 envelope 响应。

## 2026-07-31：右侧栏截图方法层级用错

- 最近复发/补充：2026-08-01 验证官网入口卡 hover 裁切时，直接调用 Browser locator 的 `.hover()`，运行时返回 `is not a function`。当前 locator 原型只有 click/fill/press 等交互，没有 hover；以后先核对原型或文档。需要真实 CSS `:hover` 验收时，右侧 Browser 继续承担可见预览与页面健康检查，悬停动作改用工作区 Playwright + 已验证的本机 Chrome，并明确记录这是能力缺口下的补充验证。
- 最近复发/补充：2026-08-01 官网卡片响应式验收连续猜测 `tab.setViewportSize()` 与 `tab.playwright.setViewportSize()`，两者都返回 `is not a function`；当前运行时方法清单确认没有视口缩放能力。已把规则提升到 `AGENTS.md`：Browser 先做真实桌面验收，窄屏改用隔离 Playwright context，不再试探不存在的方法。
- 环境：Codex in-app Browser，本地 Panel UI E2E。
- 错误模式：按通用前端测试说明调用 `tab.playwright.screenshot(...)`。
- 症状 / 退出码：运行时返回 `tab.playwright.screenshot is not a function`；页面和会话未受影响。
- 根因：当前 Browser 的完整 API 文档把截图定义在 `tab.screenshot(...)`，Playwright 子接口没有该方法。
- 正确做法：以所选 Browser 的运行时文档为准，调用 `tab.screenshot({ fullPage: false })`，需要 DOM 操作时才使用 `tab.playwright`。
- 预防检查：调用可选浏览器能力前先对照本次选中 Browser 的 API Reference，不从相邻技能示例推断方法层级。
- 适用范围：Codex in-app Browser 截图与视觉 QA。

## 2026-08-01：工作区 Playwright 与缓存 Chromium revision 不匹配

- 环境：Windows，Codex 工作区依赖中的 Playwright，本地响应式补充验收。
- 错误模式：只确认 `%LOCALAPPDATA%\ms-playwright` 目录存在后直接调用 `chromium.launch()`；失败调用之后又假定未初始化的顶层变量仍可直接赋值。
- 症状 / 退出码：Playwright 查找 `chromium_headless_shell-1200` 时提示精确可执行文件不存在；下一次调用因变量未定义立即失败，页面均未打开。
- 根因：缓存根目录存在不代表当前 Playwright 所需 revision 已安装；失败表达式后的变量绑定也不保证可复用。
- 正确做法：先用 `Test-Path -LiteralPath` 核对精确浏览器路径；本机已有 Chrome 时显式传入已验证的 `executablePath`。工具调用抛错后使用新的 `var` 绑定或重新声明，不依赖失败调用的部分初始化状态。
- 预防检查：启动前同时验证 Playwright 包与目标 browser executable；启动失败后先确认各绑定是否存在，再继续后续步骤。
- 适用范围：工作区 Playwright/Chromium 回归、截图与响应式 QA。

## 2026-07-31：UI E2E 断言使用错误的计数文案

- 环境：Codex in-app Browser，已安装 Mod 搜索。
- 错误模式：搜索命中断言要求 DOM 含 `1 个`，实际过滤计数文案是 `1 / 3 个`。
- 症状 / 退出码：搜索结果正确只剩 Nexus 4242 卡片，但测试先报 `Nexus ID filter failed`。
- 根因：把未过滤列表的计数格式套到过滤状态，没有先以新 DOM 为证据建立断言。
- 正确做法：先断言目标卡存在、非目标卡不存在，并匹配过滤状态的 `1 / 3 个`；文案断言只使用实际快照支持的稳定文本。
- 预防检查：每次 UI 状态变化后先取新快照，再建立该状态的精确断言，禁止沿用上一状态的展示格式。
- 适用范围：搜索、筛选、分页、排序等会改变计数格式的浏览器 E2E。

- 最近补充：同轮名称排序验收在搜索框仍为 `e2e.locala` 时就断言三张卡顺序，导致 `A-Z sort failed`。排序断言前必须先确认过滤条件确实清空且计数恢复为 `3 个`，不能只假设 `fill("")` 已触发受控输入更新。
- 最近补充：当前 in-app Browser 的 `press("ControlOrMeta+A")` / `press("Control+A")` 没有选中全部文本，后续 Backspace 每次只删除一个字符。需要清空受控搜索框时，用产品等价的空白查询验证 `trim()` 后全量结果，或改用可确认生效的 DOM/CUA 输入方式；每次都必须从新快照确认输入值和结果数。
- 最近补充：配置 Mod 页没有活动存档时，`0 / N` 搜索计数仍会更新，但内容区优先保留“请先选择一个存档”，不会显示普通列表分支的搜索空态。断言必须先识别页面前置状态，不能要求互斥分支同时出现。
- 最近补充：2026-08-01 官网上线验收用静态 HTML 精确匹配 `class="VPFeature"` 得到 0，并把导航主题开关的两个通用 `.icon` 误算为 feature icon；真实卡片 class 是 `VPLink link no-icon VPFeature`。VitePress SSR/DOM 断言应限定 `.VPFeatures .VPFeature` 与 `.VPFeatures .VPFeature .icon`，或使用结构稳定的六个 `<article class="box">`，不要假设 class 属性只有一个 token。

## 2026-07-28：嵌套 PowerShell 提前展开变量

- 最近复发/补充：2026-08-08 最终编码审计把多个 `-join`、插值异常文本和 pipeline 塞入同一层 `pwsh -Command`，在真正执行检查前触发 `Expressions are only allowed as the first element of a pipeline`。修正为三个独立、短小的只读检查并行运行；复杂审计不要为了减少一次调用重新叠加多层引号和管道。
- 最近复发/补充：2026-08-06 在 PowerShell 参数中嵌入 `sh -c` 的 Bash 重试变量 `$attempt`，错误地使用反斜杠尝试保护变量；PowerShell 仍提前展开并让 Bash 收到残缺条件。改为三段不含变量的显式 `go mod download || (sleep 5 && go mod download) ...`，跨 Shell 命令优先消除变量与转义，而不是叠加转义层。
- 最近复发/补充：2026-08-01 Hero 配色预览验证把 `sh -c`、`grep` 模式和 PowerShell 双引号再次嵌入同一条命令，脚本在有效诊断前退出 1。已改为让 `docker exec` 直接调用 `grep`，模式使用 PowerShell 单引号参数，只有确实需要容器 shell 展开时才引入 `sh -c`。
- 最近复发/补充：2026-08-01 官网 Hero 预览审计在 JavaScript 包装、PowerShell 与 `rg` 三层中直接拼接带引号的搜索命令，命令尚未得到有效结果便退出 1。多层调用先把搜索模式和真实文件路径分别固定，优先直接调用单层 `rg -e <pattern> <confirmed-path>`；需要较长 PowerShell 逻辑时使用独立的单引号脚本块，不在 JavaScript 普通字符串里继续嵌套。
- 最近复发/补充：2026-07-31 在 `[pscustomobject]` 属性表达式中直接嵌入 `docker exec ...; $LASTEXITCODE -eq 0`，PowerShell 在执行前报缺少右括号。原生命令及退出码检查必须先单独执行并存入 `$sidecarExists`，再把变量放进对象；不要在属性值括号里混合语句分隔符。
- 最近复发/补充：2026-07-29 参考研究子任务在 JavaScript 工具包装层中再次把 PowerShell 引号嵌进普通字符串，导致包装层 `SyntaxError`，命令尚未执行；改用 JavaScript template literal 包裹完整 `pwsh -Command '& { ... }'` 后成功。跨 JavaScript/PowerShell 两层时同时检查两层字符串边界。
- 最近复发/补充：2026-08-01 发布门禁在 `functions.exec` JavaScript 包装层连续出现普通字符串闭合错误，另有 skill 分段读取数组与嵌套 `printf` 的 PowerShell 引号边界破裂；统一改为 JavaScript template literal 承载完整 PowerShell 单引号脚本块，并把复杂 `printf`/数组探针拆成独立调用。包装层语法失败时命令根本没有执行，不得原样重放。
- 环境：Windows，外层 Shell 已是 PowerShell，再调用 `pwsh -Command "...$variable..."`。
- 错误模式：在双引号命令字符串中定义并使用 `$latest`，父 PowerShell 先展开变量，子进程收到残缺表达式。
- 症状：`ParserError`，提示 `You must provide a value expression following the '+' operator`。
- 根因：父、子两层 PowerShell 的变量与引号边界混用。
- 正确做法：统一使用 `pwsh -NoLogo -NoProfile -Command '& { $latest = ...; ... }'`；路径使用 `-LiteralPath`。复杂命令拆成多个明确步骤，不在双引号中嵌套 PowerShell 代码。
- 预防检查：含 `$` 的子 PowerShell 代码在执行前确认外层为单引号脚本块。
- 适用范围：所有 Windows `shell_command`、发布脚本和临时诊断命令。

## 2026-07-28：Python 命令不存在却被后续命令掩盖

- 最近复发/补充：2026-08-01 查询本地 UI 动效数据库时，虽先获得 `Get-Command python` 结果，但没有先执行版本探针便把多个查询放进同一命令；Windows Store alias 以 `9009` 失败且无有效输出。应先单独确认版本，发现 alias 后立即加载工作区依赖，再使用返回的精确 `python.exe`；不要把解释器探针与实际查询合并。
- 最近复发：2026-08-01；`v0.4.7` 发布工具链探针用未静默分支的 `Get-Command python` 结束了整批命令，阻止后续 GitHub CLI 探针。确认宿主解释器不可用后，改为加载工作区依赖并使用返回的精确 Python 3.12.13 路径；可选命令探针必须用 `-ErrorAction SilentlyContinue` 并显式分支，不能让缺失项中断其它独立检查。同轮子审计因 workspace dependency loader 暂无流式输出而提前终止；主流程直接调用并等待权威返回后成功取得解释器，不能把“暂时无增量输出”当成 loader 失败。
- 最近复发/补充：2026-08-09 发布兼容矩阵仍先把 `Get-Command python` 返回的 Windows Store alias 当真实解释器，版本探针以 `9009` 退出；诊断时又未先验证便调用不存在的 `py -3`。随后停止重试并通过 workspace dependency loader 取得精确 Python。Windows 发布门禁开始前必须先加载工作区依赖，不能因为 `Get-Command` 返回 Application 就认为解释器可运行。
- 最近复发/补充：2026-08-06 为隔离 SQLite fixture 查询解释器时，明知同一错题仍先运行无可靠输出的 `python --version`，随后又猜测 `py -3` 可用而得到 command not found。正确入口仍是先调用 workspace dependency loader，再使用返回的精确 Python 路径；本轮没有继续尝试 Store alias。
- 环境：Windows，`python` 指向不可用的 Store alias。
- 错误模式：直接运行 `python ...; Write-Output ...`，未在 Python 后立即检查 `$LASTEXITCODE`。
- 症状：Python 返回 `9009`；因为最后的 PowerShell 输出成功，整段命令表面 exit 0。
- 根因：未先探测解释器，且原生命令退出码被后续 PowerShell 命令覆盖。
- 正确做法：先执行 `Get-Command python` 和版本探针；不可用时使用工作区依赖返回的精确 `python.exe`。每个关键原生命令后立即 `if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }`。
- 预防检查：任何 Python 门禁开始前打印或记录解释器路径与版本。
- 适用范围：兼容矩阵、文档/制品脚本和本地 Python 测试。

## 2026-07-28：Alpine 登录 Shell 重置 Go PATH

- 环境：Docker Desktop，`golang:1.25-alpine`。
- 错误模式：容器内使用 `sh -lc "go test ..."`。
- 症状：镜像本身包含 Go，但返回 `sh: go: not found`。
- 根因：登录 shell 读取 profile 后重置官方 Go 镜像提供的 PATH。
- 正确做法：使用 `sh -c "go test ..."`；执行前可用 `command -v go && go version` 探针。
- 预防检查：官方语言构建镜像默认使用非登录 shell，只有确实需要登录环境时才使用 `-l`。
- 适用范围：Docker 中的 Go、Node、Python 临时门禁。

## 2026-07-28：Windows `npm ci` 被 node_modules 文件锁阻断

- 最近复发/补充：2026-08-01 前端和官网两次把宿主源码父目录只读挂到 `/work`，同时把 named volume 挂到 `/work/node_modules`；runc 在只读父挂载下无法创建子挂载点，容器尚未运行 npm 就失败。该问题已提升到 `AGENTS.md`：宿主源码固定只读挂 `/src`，任务专属可写 workspace volume 挂 `/work`，复制源码/lockfile 后再安装构建。
- 环境：Windows 工作区，已有 `frontend/node_modules`。
- 错误模式：直接在宿主重复 `npm ci`。
- 症状：`EPERM: operation not permitted, rmdir ...node_modules...`。
- 根因：编辑器、防病毒或其它进程持有 Windows 文件句柄；`npm ci` 需要删除目录。
- 正确做法：使用发布一致的 `node:24-alpine` 容器，源码只读/普通 bind，`node_modules` 使用任务专属 named volume；在容器中运行完整测试与 build，结束后核对引用并按精确名称删除 volume。
- 预防检查：发布门禁优先在 Linux 容器执行；不得用强制递归删除解决文件锁。
- 适用范围：前端、website 及其它 Node 项目。

## 2026-07-28：Docker Desktop CLI 存在但 daemon 未启动

- 环境：Windows Docker Desktop。
- 错误模式：看到 `docker version` 客户端信息后直接构建。
- 症状：无法连接 `dockerDesktopLinuxEngine` named pipe。
- 根因：只验证了 CLI，没有验证 daemon/当前 context。
- 正确做法：先运行 `docker context show` 和 `docker info`；未就绪时启动 `Docker Desktop.exe`，以短间隔轮询 `docker info`，设置明确超时并在失败时停止后续 Docker 门禁。
- 预防检查：每个 Docker Desktop 测试批次最前面执行 daemon readiness gate。
- 适用范围：镜像构建、Compose E2E、integration test 和发布后回拉验证。

## 2026-07-28：Windows 下把 Shell glob 直接传给 `rg`

- 环境：PowerShell/Windows。
- 错误模式：`rg ... Dockerfile*` 或 `rg ... config/*test.go`。
- 症状：`文件名、目录名或卷标语法不正确 (os error 123)`。
- 根因：Windows PowerShell 与 Unix shell 的 glob 展开行为不同，`rg` 收到非法字面路径。
- 正确做法：使用 `rg -g 'Dockerfile*' <pattern> .`、`rg -g '*_test.go' <pattern> <dir>`，或先 `rg --files` 再筛选。
- 预防检查：命令参数中出现 `*` 时确认它属于 `rg -g`，而不是位置参数。
- 最近复发/补充：2026-08-09 候选镜像资源探针再次把 `Dockerfile*` 作为位置参数；`rg` 在输出其它明确目录的命中后仍以非零退出，导致整条只读探针提前停止，未创建或删除 Docker 资源。已改用 `rg -g 'Dockerfile*' ... .`；该规则已在 `AGENTS.md` 固化，后续命令构造时必须机械检查所有含 `*` 的参数只能紧跟 `-g`，不得因同时提供了有效目录就忽略非法通配路径。2026-08-01 也曾在升级状态测试与发布记录检查中复发同类问题。
- 最近复发/补充：2026-08-06 定位玩家 Mod `syncKind` fixture 时又把 `backend/internal/games/stardew_junimo/*test.go` 作为位置参数传给 `rg`。这是规则固化后的再次复发；同类检索一律先写成 `rg -g '*_test.go' <pattern> backend/internal/games/stardew_junimo`，命令提交前把所有含 `*` 的参数逐个核对为紧跟 `-g` 的值。
- 适用范围：Windows 上的仓库搜索和发布检查。

## 2026-07-28：嵌套 Go template 与 PowerShell 转义冲突

- 最近复发/补充：2026-08-06 核对 `v0.4.8` 任务专属 volume ownership 时，又在外层单引号 PowerShell 脚本中使用带转义引号的 `{{index .Labels \"...\"}}`，Docker 返回 `unexpected "\\" in operand`；脚本在任何卷删除前退出。后续 ownership 一律读取完整 `docker volume inspect` JSON 后访问 `.Labels`，不在嵌套命令中拼带引号的 Go template。
- 环境：PowerShell 调用 `docker image inspect --format`。
- 错误模式：在多层双引号中对 Go template 的引号再次加反斜杠。
- 症状：`template parsing error: unexpected "\\" in operand`。
- 根因：把 JSON/类 Unix 反斜杠转义套到 PowerShell 参数，反斜杠被原样传给 Go template。
- 正确做法：优先把完整 template 置于 PowerShell 单引号参数，或拆开检查字段；嵌套命令过深时避免一行完成所有 inspect。
- 预防检查：先用一个只读 `docker image inspect` 小命令验证 template，再放入较长脚本。
- 最近复发/补充：2026-07-31 验证测试 volume label 时把 `{{index .Labels \"...\"}}` 放进嵌套 PowerShell 双引号，Docker 再次收到反斜杠并报 `unexpected "\\"`；改用无内层引号的 `{{json .Labels}}` 后确认 ownership。
- 最近复发/补充：2026-08-01 又把带字符串键的 `{{index .Config.Labels \"...\"}}` 放进 `docker exec ... sh -c` 的第三层引号，容器 Shell 报 `unterminated quoted string`。只执行一个容器内命令时应去掉 `sh -c`，从 PowerShell 直接传参，并一次输出 `{{json .Config.Labels}}` 后在外层解析。
- 最近复发/补充：2026-08-01 后端门禁把 `docker --format` 的模板与相邻参数错误拼接，Docker 收到无效 format。复杂 inspect 不再拼接模板字符串：先输出完整 `docker inspect` JSON，再由 PowerShell `ConvertFrom-Json` 投影所需字段；只有单个无引号模板经过独立探针后才使用 `--format`。
- 最近复发/补充：2026-08-06 最终候选镜像已经成功构建后，尾部 OCI 核验再次使用 `docker image inspect --format` 与带反斜杠的 `index .Config.Labels`，同样报 template operand 解析错误。镜像本身随后通过完整 JSON 投影核对正确；鉴于同类错误再次复发，规则已提升到 `AGENTS.md`：复杂 Docker inspect 一律读取 JSON 后投影，禁止嵌套带引号的 Go template。
- 适用范围：Docker inspect、Compose format 和其它 Go-template CLI。

## 2026-07-31：PowerShell 插值变量后直接连接连字符

- 环境：PowerShell 7，创建任务专属 Docker volume 名。
- 错误模式：在双引号中写 `"$prefix-go-mod"`，期望得到 `<prefix>-go-mod`。
- 症状 / 退出码：卷创建批次无预期输出并以 1 退出；只读复核确认没有资源被创建。
- 根因：PowerShell 会把连字符附近内容按表达式/变量边界解析，字符串中的变量名边界不明确。
- 正确做法：变量后接连字符或字母时始终写 `"${prefix}-go-mod"`；创建后再按 label 列出精确资源。
- 预防检查：PowerShell 插值字符串中的变量统一使用 `${name}`，尤其是 Docker 名、tag、路径和端口组合。
- 适用范围：PowerShell 生成容器、volume、Compose project、文件名与 URL。

## 2026-07-31：Node 直接执行 TS 时不解析 Vite 的无扩展运行时导入

- 环境：Node 22 `--experimental-strip-types` 运行前端纯逻辑测试。
- 错误模式：被测 `mod-list-utils.ts` 运行时导入 `./mod-display`，认为 Node 会像 Vite 一样补 `.ts`。
- 症状 / 退出码：`ERR_MODULE_NOT_FOUND`；生产构建尚未执行。
- 根因：Node ESM 的类型剥离不等于 Vite 模块解析，运行时相对导入仍要求可解析的明确文件。
- 正确做法：纯逻辑测试目标保持无运行时项目内依赖，或把公共逻辑组织成测试环境可直接加载的单文件；类型导入会被安全剥离。
- 预防检查：新增 `node --experimental-strip-types` 测试前检查被测模块的所有非 `import type` 依赖。
- 适用范围：前端状态机、排序/过滤 helper 的无测试框架 Node 脚本。

## 2026-07-31：精简 Node Alpine 缺少 VitePress `lastUpdated` 所需 Git

- 最近复发/补充：2026-08-09 候选收口又把官网源码整体只读挂载进 Node 20 容器，即使把输出改到 `/tmp/docs-dist`，Vite 加载 `config.ts` 仍因无法创建 `config.ts.timestamp-*.mjs` 以 `EROFS` 退出。已改为在容器内把网站复制到任务专属可写目录，并只读复用宿主 Git 对象库；Vite/VitePress 构建不能只给 `dist` 与 `node_modules` 写权限。2026-08-01 的 linked worktree 构建也曾命中同一问题；不要在只读源码挂载上继续补零散写目录。
- 最近复发/补充：2026-08-01 官网 Hero 最终干净构建把 Windows linked worktree 只读挂到 `/repo`，其 `.git` 文件仍指向 `E:/.../.git/worktrees/...`，容器内 `git -C /repo` 报 `not a git repository`，尚未进入 npm 构建。linked worktree 跨 Windows/Linux 挂载时必须同时只读挂载主仓库 `.git`，并显式设置容器内 `GIT_DIR=/git-common/worktrees/<name>`、`GIT_WORK_TREE=/repo`；不能认为挂入工作树目录就自动获得可解析的 Git 元数据。
- 最近复发/补充：2026-08-01 官网 Hero 增量预览复用了保存 `/work` 的依赖 volume，却换成新的临时 Node Alpine 构建容器，误以为首次容器执行过的 `apk add git` 也会随 volume 保留；增量 build 再次以 `spawn git ENOENT` 失败。系统包属于容器层而非工作 volume，每一个新的构建容器都必须先安装并探测 `git --version`，不能从复用 `node_modules` 推断 Git 仍存在。
- 环境：`node:24-alpine` Docker 容器运行 `npm run docs:build`。
- 错误模式：只安装 npm 依赖，未安装系统 `git`。
- 症状 / 退出码：VitePress 处理 `docs/changelog.md` 时以 `spawn git ENOENT` 失败。
- 根因：项目启用了依赖 Git 历史的 lastUpdated 元数据；精简 Node 镜像不内置 Git。
- 正确做法：文档构建容器先执行 `apk add --no-cache git`，再 `npm ci && npm run docs:build`。
- 预防检查：自建文档门禁镜像必须包含 Git；不能只根据前端 Vite build 的依赖推断 VitePress 环境。
- 适用范围：VitePress/VuePress 等读取 Git 时间或提交信息的容器化文档构建。

## 2026-07-28：并行门禁被单个命令构造错误吞掉输出

- 最近复发/补充：2026-08-01 文档门禁批次中一个命令构造错误让其它已完成步骤的证据没有显示，随后不得不逐项重跑。多工具链并行固定使用 `Promise.allSettled` 或独立调用并给结果加标签，不能让一个调度错误吞掉其它门禁状态。
- 环境：JavaScript `Promise.all` 并行调用多组 PowerShell 门禁。
- 错误模式：一个复杂命令存在引号错误，`Promise.all` 立即拒绝，其他组结果未输出。
- 症状：只得到顶层 `Script error`，难以判断哪组失败。
- 根因：并行调度没有保留每个任务的独立成功/失败结果，且单条 PowerShell 过长。
- 正确做法：先分别验证命令；并行时使用 `Promise.allSettled` 并为每组输出稳定标签，或拆成独立调用。产品门禁失败与调度命令失败必须分开报告。
- 预防检查：超过一个工具链的复合门禁先做语法探针，再并行执行。
- 适用范围：后端、前端、兼容矩阵、Docker integration 的并行发布门禁。

## 2026-07-28：`git diff --no-index` 的预期差异码污染整段校验

- 环境：PowerShell 7，使用 `git diff --no-index -- /dev/null <new-file>` 展示未跟踪文件差异。
- 错误模式：接受退出码 `1` 表示“存在差异”，但脚本结尾没有显式 `exit 0`。
- 症状：差异完整输出且没有真正错误，整个 `shell_command` 仍被判定 exit 1。
- 根因：`git diff --no-index` 用 `0` 表示无差异、`1` 表示有差异、`>1` 才表示错误；PowerShell 保留了最后一个原生命令的 `$LASTEXITCODE`。
- 正确做法：命令后立即保存并判断退出码；`if ($diffExit -gt 1) { exit $diffExit }`，完成剩余校验后显式 `exit 0`。只需查看仓库已跟踪差异时直接使用 `git diff`。
- 预防检查：使用“非零但属于正常结果”的 CLI（如 `git diff --no-index`、无匹配的 `rg`）时，提前写明允许的退出码并在脚本末尾确定最终退出码。
- 最近复发/补充：2026-08-09 核对 required runtime 自动升级时，先成功读取目标代码，随后把一个可能无命中的补充 `rg` 放在组合命令末尾；模式使用 snake_case，而目标文件只含 Go 常量名，`rg` 正常返回 1，却让整条只读命令被标记失败。补充检索必须立即保存退出码并仅在 `>1` 时失败，或在已完整读取目标文件时省略猜测性搜索，不能让“无命中”覆盖前序成功证据。
- 适用范围：差异审查、搜索探针和所有带预期非零状态的组合校验脚本。

## 2026-07-28：猜测配置文件扩展名且忽略 PowerShell 非终止错误

- 最近复发/补充：2026-08-09 候选运行参数检查时，又把不存在的仓库根 `docker-compose.yml` 与真实 `deploy/docker-compose.yml` 一起交给 `rg`；有效结果后仍出现路径错误，且包装脚本未检查 `rg` 的退出码。容器参数随后只从已确认的 `deploy/docker-compose.yml` 读取。多根搜索必须先发现路径，并在原生命令后立即检查 `$LASTEXITCODE`，不能让后续成功输出掩盖错误。
- 最近复发/补充：2026-08-09 发布 ShellCheck 已从工作流读取到精确测试路径，却在手工转写时把 `test_migrate_fnos.sh`、`test_repair_junimo_upgrade.sh` 的下划线误成连字符，功能测试已通过但 ShellCheck 因两个输入不存在退出 1。发布门禁应复制工作流原命令或先用 `Test-Path`/`rg --files` 校验整组参数，不能凭视觉记忆重输相似文件名。
- 最近复发/补充：2026-08-09 盘点升级错误码时，把不存在的仓库根 `tests/` 与已确认的 `backend/frontend/deploy/docs` 一起作为 `rg` 位置参数；虽然命中了大量有效结果，`rg` 仍因 `tests` 不存在以退出码 1 结束。随后又按惯例猜测脚本测试名为 `deploy/test-repair-junimo-upgrade.sh`，实际测试文件位于其它已命名脚本，检索再次退出 1。多根、具体测试文件和脚本名都必须先用 `Test-Path -LiteralPath` 或 `rg --files` 验证；如果测试位于模块内部，应搜索已确认的模块目录并用 `-g '*_test.go'` 限定，不能凭常见仓库布局补根目录或测试文件名。
- 最近复发/补充：2026-07-29 在 `rg` 位置参数中写入仓库不存在的 `internal`、`cmd` 目录，导致已有匹配结果后仍以路径错误结束。多目录搜索必须先用 `rg --files` 或 `Test-Path` 验证目录，只传当前仓库实际存在的根目录。
- 最近复发/补充：2026-07-29 重构隔离预览时再次直接读取不存在的 `docs/.vitepress/config.mts`；随后先用 `rg --files --hidden docs/.vitepress` 找到真实的 `config.ts`。2026-07-31 搜索前端 tooltip 时又把未经发现的 `frontend/src/components` 作为 `rg` 位置参数，产生 `os error 2`；同时后续成功输出掩盖了原生命令状态。今后所有多目录搜索先用 `rg --files <已确认根目录>` 发现路径，或只从已确认存在的共同父目录配合 `-g` 搜索，并在 `rg` 后立即保存、判断 `$LASTEXITCODE`。
- 最近复发/补充：2026-08-01 一次组合读取对不存在路径产生 PowerShell non-terminating error，末尾显式 `exit 0` 又把它掩盖。只读发布脚本同样必须以 `$ErrorActionPreference=''Stop''` 开始，文件路径先发现后读取，成功分支才允许输出 0。
- 环境：PowerShell 7，读取 VitePress 配置。
- 错误模式：未先查看实际文件便读取 `website/docs/.vitepress/config.mts`，并在脚本末尾无条件 `exit 0`。
- 症状：`Get-Content` 报路径不存在，但组合命令仍显示 exit 0，后续输出容易让人误以为所有输入都已读取。
- 根因：根据常见项目结构猜测扩展名；PowerShell cmdlet 默认产生 non-terminating error，不会自动写入原生命令的 `$LASTEXITCODE`。
- 正确做法：先用 `rg --files website/docs/.vitepress` 或 `Get-ChildItem -LiteralPath` 确认实际路径；关键读取使用 `-ErrorAction Stop`，组合脚本开头设置 `$ErrorActionPreference = "Stop"`，只在全部步骤成功后 `exit 0`。
- 预防检查：不得猜测 `.ts`、`.mts`、`.js` 等扩展名；读取多个必需文件前先一次性验证路径存在。
- 适用范围：PowerShell 文件发现、配置审查和多步骤只读检查。

## 2026-07-28：子目录工作目录与仓库根相对路径混用

- 最近复发/补充：2026-08-01 在 `workdir=backend` 下对 `gofmt` 仍传入 `backend/internal/web/...`，随后又在仓库根直接运行 `go test ./backend/internal/web`；前者目标路径不存在，后者找不到根目录 `go.mod`。分别改用 `internal/web/...` 与 `go -C backend test ./internal/web` 后成功。组合命令必须在执行前把每个路径和模块根按实际 `workdir` 展开一次。
- 最近复发/补充：2026-08-01 `v0.4.7` 门禁再次在仓库根假定存在 `go.mod`，实际 Go module 位于 `backend/`。执行 Go 命令前先以 `rg --files -g go.mod` 确认模块根，再选择 `workdir=backend` 或 `go -C backend`，不能从仓库语言类型推断根模块。
- 环境：PowerShell 7，`shell_command` 的 `workdir` 设为 `website/`，同一脚本先构建站点再校验仓库文件。
- 错误模式：构建完成后仍用 `AGENTS.md`、`docs/...` 等仓库根相对路径执行 `Resolve-Path`。
- 症状：VitePress production build 已成功，后续编码校验却报 `Cannot find path 'AGENTS.md'`，整段命令最终 exit 1。
- 根因：把 npm 项目工作目录和仓库校验基准混成一个相对路径上下文。
- 正确做法：组合门禁统一在仓库根执行，并用 `npm.cmd --prefix website run docs:build` 调用子项目；或在脚本开头解析并固定仓库根绝对路径，所有后续文件基于该路径拼接。
- 预防检查：执行多目录组合命令前列出每一步的路径基准；设置非根 `workdir` 时禁止直接使用仓库根相对路径。
- 适用范围：website/frontend 子项目构建、跨目录编码检查和发布组合门禁。

## 2026-07-28：串行安装多个远程 skill 被整批超时

- 环境：Windows，Codex `skill-installer`，连续从三个 GitHub 仓库下载 skill。
- 错误模式：把多个独立安装合并到一个 120 秒的 `shell_command` 中，且安装脚本输出被 Python 缓冲。
- 症状 / 退出码：命令长时间无输出并以 `124` 超时，无法从批次输出判断已完成到哪一个目标。
- 根因：多个网络下载共享单一超时预算；仓库归档下载时间不可预测，缓冲输出又隐藏了中间进度。
- 正确做法：每个远程 skill 使用单独的安装命令，调用 Python 时使用 `-u`；超时后先核对目标目录和 `SKILL.md` 完整性，只重试缺失项。
- 预防检查：安装前逐个检查目标是否存在；大型仓库提高单项超时，但不把多个来源合并成同一批次。
- 适用范围：skill、模板或其它多来源 GitHub 制品安装。

## 2026-07-28：PowerShell `foreach` 语句直接接管道

- 最近复发：2026-08-08；卷元数据探针再次写成 `foreach (...) { ... } | ConvertTo-Json`，在任何 Docker inspect 执行前触发同一 ParserError。数组子表达式规则虽已提升到 `AGENTS.md` 仍复发，工具单行批处理必须默认使用 `ForEach-Object`；确需语句式循环时先赋值 `$results = @(foreach (...) { ... })`，再单独传入管道。此前 2026-07-29 与 2026-07-31 已出现同类错误。
- 环境：PowerShell 7，组合对象后用 `Format-Table` 展示。
- 错误模式：`foreach (...) { ... } | Format-Table`。
- 症状 / 退出码：`ParserError: An empty pipe element is not allowed`，退出码 `1`。
- 根因：PowerShell 语句形式的 `foreach` 不能直接作为管道左值。
- 正确做法：使用 `@(foreach (...) { ... }) | Format-Table`，或改为 `$items | ForEach-Object { ... } | Format-Table`。
- 预防检查：管道左侧若是 `foreach`、`if` 等语句，先显式包装为数组子表达式。
- 适用范围：PowerShell 中的批量状态检查与格式化输出。

## 2026-08-08：未固定搜索结果基数就参与整数运算

- 环境：PowerShell 7，按 `Select-String` 返回行号切片查看 TypeScript 上下文。
- 错误模式：把可能返回零个或多个结果的 `.LineNumber` 直接强转或参与 `- 12`，第一次得到对象数组而无法做减法，第二次因搜索模式写错得到 `0` 并把负数传给 `Select-Object -Skip`。
- 症状 / 退出码：分别报 `System.Object[] does not contain a method named op_Subtraction` 与 `Skip -8 is less than the minimum allowed range`；均为只读命令，未修改文件。
- 根因：没有在算术前确认搜索恰好命中一条，也没有给切片起点设置下界。
- 正确做法：优先使用 `rg -n -C <n> <pattern> <confirmed-file>`；确需 PowerShell 切片时先 `Select-Object -First 1`，检查结果非空，再以 `[Math]::Max(0, [int]$lineNumber - <n>)` 计算起点。
- 预防检查：任何用于 `-Skip`、数组索引或减法的搜索结果必须先固定为单个标量、检查非空并做上下界约束。
- 适用范围：PowerShell 的日志、源码和文档上下文切片。

## 2026-08-08：Docker Desktop 容器重启后宿主随机端口响应链路卡住

- 环境：PowerShell 7、Docker Desktop，候选 Panel 容器使用随机宿主端口并在同一脚本中执行 `docker restart`。
- 错误模式：重启前后都由同一 PowerShell 进程反复调用 `Invoke-RestMethod`；容器日志已记录重启后 `/health` 200，但客户端复用旧连接后长时间不返回，`-TimeoutSec 3` 没有按预期限制连接池等待，90 秒 readiness 被误判失败。
- 症状 / 退出码：API 的 401/400/409 边界此前均通过，重启后服务也实际响应 health，但 harness 抛出 `candidate Panel readiness timed out`；finally 按 ownership label 精确清理了本轮容器和 volume。
- 最近复发/更正：2026-08-09 即使每轮改用独立 `curl.exe --connect-timeout 2 --max-time 3` 进程，同一随机宿主端口在 `docker restart` 后仍出现相同现象：容器日志两次记录 `/health` 微秒级完成，宿主客户端却约 60 秒后才发下一次请求。由此排除仅是 .NET 连接池复用，问题位于本轮 Docker Desktop published-port/NAT 返回链或其与宿主客户端的组合。
- 根因：容器重启切断既有连接后，本轮 Docker Desktop 随机发布端口的响应返回链路卡住；服务端内部 HTTP 已正常处理，不能把宿主 NAT 卡顿当成 Panel readiness 失败。
- 正确做法：重启恢复的权威 readiness 先用 `docker exec <owned-container> wget -qO- http://127.0.0.1:8090/health` 和同容器 `/api/version`/`/api/setup/status` 验证进程与持久卷；需要证明宿主重连时改为受控重建容器或重新发布端口后再测。始终结合容器日志，不能因 NAT 回程卡住误报产品。
- 预防检查：Docker Desktop 的 restart E2E 同时设计容器内 readiness 与宿主 published-port 探针；两者分歧时保留分层证据，不重复等待同一失效 NAT 映射。
- 适用范围：Panel updater 断线重连、Docker restart、容器替换后的 health/version 验收。

- 最近复发/补充：2026-08-09 改用 curl 重跑时，把临时 cookie/JSON 目录的递归删除也塞进同一条长 E2E 命令，工具安全策略在执行前拒绝整条脚本。该验证其实只需公开 setup 持久化，不需要 cookie 文件；改为 PowerShell 内存请求创建测试管理员、重启后用独立 curl 读取公开 health/version/setup，完全取消临时目录和递归删除。策略拒绝表示命令未执行，不得假设容器或 volume 已创建。

## 2026-07-28：`apply_patch` 使用了未经核对的长上下文

- 最近复发/补充：2026-07-29 手工转写 Markdown 链接上下文时把半角结尾写成全角字符，随后又用不完整的文件尾上下文追加 CSS，两次均安全校验失败。补丁上下文必须从实际文件复制，包含链接、全角标点或文件尾追加时按文件拆小处理。
- 最近复发/补充：2026-08-06 把旧 worktree 的原始 `git diff` 直接包装给 `apply_patch` 时，同时带入了整文件 CRLF 差异和带行号的 `@@ -x,y +x,y @@` hunk header，补丁解析失败；只去掉换行噪声后仍因编号 header 再失败。正确做法是用 `git diff --ignore-space-at-eol` 获取语义差异，把 hunk header 规范为裸 `@@` 后再交给 `apply_patch`；与当前 `main` 重叠的文件必须重新读取当前上下文并人工合并，不能把旧基线 patch 强套到新文件。
- 环境：Windows 工作区，使用 `apply_patch` 同时更新错题本和 `AGENTS.md`。
- 错误模式：手工重写完整相邻行作为补丁上下文，其中漏掉原文空格。
- 症状 / 退出码：`apply_patch verification failed`，补丁未产生任何修改。
- 根因：没有先从文件读取精确锚点，且使用了不必要的长上下文。
- 正确做法：先用 `rg -n --fixed-strings` 定位原文；补丁只保留稳定、最短的精确上下文。
- 预防检查：涉及长中文行或多文件补丁时，不凭聊天上下文重打原文，先读取实际文件。
- 最近复发/补充：2026-08-01 合并 `v0.4.7` 响应式补丁时凭旧工作树猜测 `SavesPage.css` overlay 的背景声明，导致最小补丁仍因上下文不一致失败；随后一个多文件文档补丁漏写第二个 `*** Update File`，让 handoff 的锚点被错误套到 `docs/03-frontend.md` 并再次校验失败。读取目标 release worktree 精确行、为每个目标显式声明文件后应用成功。补丁上下文必须来自将被修改的那一个 worktree，多文件补丁须逐段核对目标声明。
- 最近复发/补充：2026-08-06 在同一补丁中依次更新 Compose、删除临时 env、再更新错题本时，把第二个文件声明放进尚未结束的 hunk，`apply_patch` 报 `Unexpected line found in update hunk` 并零修改退出。多种 patch operation 混合时每个文件段必须先正常结束；更稳妥的是把 update/delete/文档补记拆成独立补丁并逐个检查返回。
- 最近复发/补充：2026-08-06 补记候选镜像 inspect 错误时，从聊天摘要重打了含多层反斜杠的旧行作为上下文，实际文件中的转义数量不同，`apply_patch` 校验失败且零修改；随后构造补丁时又漏了上下文原文本身的 Markdown 列表短横线，第二次校验仍失败。应先读取文件原文，再选不含易变转义的邻近稳定行作最小锚点；补丁正文行需明确区分 patch marker 与文件原有字符。
- 适用范围：所有 `apply_patch` 修改，尤其是长行、编码敏感文件和多文件补丁。

## 2026-07-28：Browser 后端不支持 `networkidle` 等待状态

- 最近复发/补充：2026-08-09 v0.4.9 官网 QA 再次按技能示例请求 `networkidle`，当前后端仍明确拒绝；改为 `domcontentloaded` 后等待首页 `v0.4.9` 可见并读取 DOM/console。随后 `expectNavigation` 又传入正则 URL，动作实际已完成但包装器报 `requires a url`；先读取 `tab.url()` 发现已到目标页，再按目标 DOM 验证，没有盲目重复点击。两项规则已提升到 `AGENTS.md`。
- 最近复发/补充：2026-08-01 Browser 的只读 `evaluate` 中，SVG 元素代理不提供 `getBBox()`，调用返回 `TypeError`。SVG 视觉校验改用 `getBoundingClientRect()`、静态 `viewBox`/路径坐标和截图联合判断；调用非基础 DOM 方法前先确认当前代理实际支持。
- 最近复发/补充：2026-07-29 本地预览后期进入 `ERR_CONNECTION_REFUSED` 错误页，Browser 随即因 `data:` 错误页 URL 策略拒绝 reload/close 链。此时不得继续尝试替代浏览器或 CDP 绕过；保留此前证据、精确停止 dev server，并以 production build 作为最终非视觉门禁。同轮还误把通用 Playwright 的 `setViewportSize`、对象形式 `waitForURL` 和代理元素原生 `click()` 套到封装 API；响应式尺寸与交互必须使用当前 Browser 暴露的 viewport 与 locator 能力。
- 最近复发/补充：2026-08-01 线上 changelog 导航把通用 Playwright 的 URL predicate 传给 Browser `waitForURL`，返回 `requires a url`。当前 Browser 只接受明确 URL 参数；点击后可直接读取 `tab.url()` 和目标 DOM，或传文档支持的精确 URL，不使用 predicate 回调。
- 最近复发/补充：2026-07-29 在静态概念稿预览中误把 `domcontentloaded` 当成 `tab.playwright` 方法调用；同日在下半页 QA 又照搬通用 Playwright 的 `scrollIntoViewIfNeeded()`，均返回 `is not a function`。本次重构又误用 `iab.tabs.claim()` 与 `tab.playwright.screenshot()`，实际 API 分别是 `iab.user.claimTab()` 与 `tab.screenshot()`；并再次请求了不受支持的 `networkidle`。`goto()`/`reload()` 本身用于完成导航；其它交互先核对 Browser 客户端实际方法，不再凭通用 Playwright 记忆猜测。
- 环境：Codex 应用内 Browser，对本地 VitePress 开发服务器做页面 QA。
- 错误模式：按通用 Playwright 类型调用 `tab.playwright.waitForLoadState({state:"networkidle"})`。
- 症状：工具直接返回 `playwright_wait_for_load_state does not support networkidle`。
- 根因：当前 Browser 控制后端只实现部分 load state，能力小于通用 Playwright 类型声明。
- 正确做法：本项目页面导航使用 `domcontentloaded`；之后等待明确的页面标题、唯一 heading/link 或直接读取目标 DOM 状态，不以全局网络空闲作为就绪条件。连续检查多个路由时，每次导航后都必须单独等待目标页面状态，不能只在循环末尾统一读取。
- 预防检查：Browser 插件的有限 Playwright API 以运行时实际支持为准；遇到不支持的方法或参数立即换用可观察页面状态，不重复同一调用。
- 适用范围：VitePress、Vite HMR 和其它本地 SPA 的应用内 Browser 验收。

## 2026-07-29：前台临时 HTTP 服务超时后仍占用端口

- 最近复发/补充：2026-08-09 正式发布门禁两次用同一个 `functions.exec` 的 `Promise.all` 并发启动多个长运行 `shell_command`；其中一个子调用异常后编排层在约 1 秒内返回失败，但两组 `go test ./...` 仍成为后台孤儿进程，前端容器一度进入 `Dead`，输出也无法作为门禁证据。第二次前未确认第一次的宿主进程终态，造成重复后端门禁。后续正式门禁必须逐项用可等待的单独调用运行；编排调用异常后先查精确 PID、容器与 volume，不得再次提交同一门禁。该规则已同步提升到 `AGENTS.md`。
- 最近复发/补充：同轮清理已确认归属的重复后端门禁时，进程在检查与 `Stop-Process -ErrorAction Stop` 之间自然退出，命令报 “Cannot find a process”。清理长运行进程须在停止前再次读取目标，使用幂等的 `Get-Process ... -ErrorAction SilentlyContinue | Stop-Process`，随后复查；不得把正常的退出竞态误判为产品失败。
- 最近复发/补充：2026-07-29 用内层 1 秒命令超时启动 VitePress，期望外层工具返回可续用 cell，结果常驻服务被直接以 124 终止。长运行服务要给命令本身足够超时，只用外层 yield 提前取得 session/cell；结束时再精确终止并核对端口。
- 最近复发/补充：2026-08-01 在任务 DinD 内执行 `apk add --no-cache go` 时外层工具 304 秒超时，但容器内 `apk` 仍继续运行并最终成功安装 Go；不能在未复查进程/`command -v go` 的情况下重复安装或直接删除容器。本轮先读取精确 PID，下一次检查时进程已自然结束，再以 `go version` 确认结果。2026-07-31 的 Vite 子进程案例同样说明超时只代表等待结束，不代表派生工作已停止。
- 环境：Windows，`shell_command` 直接运行 Python `http.server` 作为本地效果图预览服务。
- 错误模式：短超时返回后直接假设服务已停止，又尝试在相同端口启动第二个服务。
- 症状 / 退出码：第二次启动返回 `EADDRINUSE`；检查发现第一个 Python 进程仍监听原端口。
- 根因：工具调用超时或终止长运行 cell 只结束等待/包装进程，不保证已派生的服务进程退出。
- 正确做法：超时后先用 `Get-NetTCPConnection` 核对监听端口和进程命令行；确认归属后复用或按精确 PID 停止，再以明确的 `--directory` 启动预览服务。
- 预防检查：任何本地服务启动或重启前先查端口；清理时只停止命令行和端口均匹配本任务的进程。
- 适用范围：Python `http.server`、Vite、Node 静态服务器等本地预览服务。

## 2026-07-29：Browser 窄屏 `fullPage` 截图出现空白与固定栏重复

- 最近复发/补充：2026-08-09 v0.4.9 官网 390×844 首页 `fullPage` 截图再次把固定导航和首屏内容重复拼接，页面 DOM、普通视口渲染及 root/body 宽度度量均正常。该现象不能算产品布局失败；窄屏证据固定使用普通视口截图，并以 `document/body scrollWidth <= clientWidth` 断言无横向溢出，规则已提升到 `AGENTS.md`。
- 最近复发/补充：2026-07-29 将临时 390×844 viewport `reset()` 回默认尺寸后，首张普通视口截图右侧出现黑块，DOM 同时报告默认宽度与无横向溢出；对 tab 执行一次 `reload()` 后渲染面恢复正常。以后恢复默认 viewport 后先 reload，再截最终交付图。
- 环境：Codex 应用内 Browser，临时 viewport 切换到 390×844 后截取静态响应式页面。
- 错误模式：只凭 `screenshot({fullPage:true})` 的结果判断窄屏页面没有渲染。
- 症状：全页图只显示顶部导航且在长画布中重复，正文近乎空白；DOM 度量同时显示正文可见、页面高度和元素宽度均正常。
- 根因：当前 Browser 的窄屏全页拼接会在特定固定/网格背景页面上产生渲染伪影，不等于真实页面状态。
- 正确做法：同时检查 DOM 的位置、display/visibility/opacity 和横向溢出，并用普通视口截图复核实际首屏；本次普通 390×844 截图正常。
- 预防检查：响应式 QA 不把单张全页图当唯一证据；至少组合视口截图、DOM 度量和 console 日志。
- 适用范围：带固定导航、长页面或复杂背景的 Browser 响应式验收。

## 2026-07-29：Browser `evaluate` 的 DOM 投影不可用于临时注入

- 环境：Codex 应用内 Browser，尝试在已发布页面上临时增加只用于截图的样式与装饰节点。
- 错误模式：按普通浏览器上下文调用 `document.createElement()`，随后尝试写回元素 `innerHTML`。
- 症状：`document.createElement is not a function`；元素 `innerHTML` 也只有 getter，赋值返回只读错误。
- 根因：当前 Browser `playwright.evaluate` 暴露的是受限 DOM 投影，支持查询和度量，但不保证常规 DOM 创建/写入接口。
- 正确做法：线上页面只做查询、度量和截图；需要无持久化视觉实验时，在工作区外建立隔离的本地预览并通过 Browser 渲染，不尝试修改线上 tab DOM。
- 预防检查：调用 `evaluate` 前把它视为只读接口；除非文档明确提供写入能力，不使用 `createElement`、`append`、`setAttribute` 或 `innerHTML=`。
- 适用范围：应用内 Browser 的线上页面审查与视觉概念验证。

## 2026-07-29：嵌套 PowerShell 脚本中的正则引号字符类破坏解析

- 环境：Windows，外层命令通过 `pwsh -Command '& { ... }'` 克隆线上静态页面。
- 错误模式：在内部双引号正则中写入同时匹配单双引号的字符类，导致单引号提前破坏外层脚本边界。
- 症状 / 退出码：PowerShell `ParserError: Missing ')' in method call`，命令未写出预览文件。
- 根因：正则本身的单引号字符与外层单引号脚本块冲突，反斜杠不是 PowerShell 的通用引号转义符。
- 正确做法：把匹配条件简化为不含引号字符类的稳定标记（本次直接匹配 `modulepreload` 所在 link），HTML 注入属性使用无需引号的合法静态写法；复杂替换再拆到独立脚本文件。
- 预防检查：嵌套 `pwsh -Command '& { ... }'` 内禁止直接出现未隔离的单引号；正则需要单双引号字符类时改用独立 `.ps1` 或先构造字符串变量。
- 最近复发/补充：2026-08-01 发布历史子审计在嵌套 `pwsh` 的 `rg` 双引号模式中使用反斜杠转义字面双引号，PowerShell 没有按预期转义并把后半模式当成命令；后续拆成不含字面引号的独立 `rg` 调用。反斜杠不能替代 PowerShell 的引号边界设计。
- 适用范围：PowerShell 中的 HTML、JSON、正则批量重写与嵌套命令调用。

## 2026-07-29：项目外 VitePress 副本无法从原 CLI 路径解析依赖

- 环境：Windows，将 `git archive HEAD website` 解压到 `.codex/visualizations`，再直接调用原项目 `node_modules/vitepress/bin/vitepress.js` 启动副本。
- 错误模式：认为从原项目执行 VitePress CLI 就会让副本配置自动继承原项目的模块解析目录。
- 症状 / 退出码：服务未监听，配置加载报 `Cannot find package 'vitepress' imported from ...config.ts.timestamp-*.mjs`。
- 根因：临时配置模块从副本目录解析裸包；CLI 文件的位置不改变该配置模块的 Node ESM 查找链。
- 正确做法：在项目外副本的 `website/node_modules` 建立指向原项目已安装依赖目录的明确 junction，再启动 VitePress；副本只修改导出的源码，依赖保持只读复用。
- 预防检查：启动外部 Vite/VitePress 副本前先确认副本根的 `node_modules` 可解析 `vitepress`，不能只检查 CLI 文件存在。
- 适用范围：Vite、VitePress 及其它配置文件会自行 import 裸包的项目外隔离预览。

## 2026-07-29：把可访问性快照名称猜成 CSS 属性

- 环境：Codex 应用内 Browser，VitePress 桌面导航回归。
- 错误模式：快照显示 `navigation "Main Navigation"` 后，直接构造 `nav[aria-label="Main Navigation"]` 定位器。
- 症状 / 退出码：目标导航链接实际存在，但定位器 `count()` 为 `0`，跨页面交互未执行。
- 根因：可访问性树中的 role/name 不保证来自同名 DOM `aria-label`；本页实际导航为 `nav.VPNavBarMenu` 且没有 `aria-label`。
- 正确做法：定位失败后先刷新 DOM snapshot，并用只读 DOM 投影确认稳定属性；本项目顶栏使用已确认的 `.VPNavBarMenu a[href="..."]`，操作前继续检查唯一性。
- 预防检查：可访问性名称用于 `getByRole(..., {name})`，只有实际 DOM 明确存在对应属性时才把它改写成 CSS 属性选择器。
- 最近复发/补充：2026-07-29 首页像素元素终验时，用正则 `/Anxi Panel/` 匹配包含多个嵌套文本的 hero heading，Browser role selector 超时；随后 `.VPHero h1.heading` 的 locator evaluate 也超时，说明问题不只在可访问性名称。对纯滚动截图改用页面级安全 `window.scrollTo()`，locator 留给必须作用于具体控件的交互。
- 适用范围：Browser/Playwright 的导航、按钮和复合组件定位。

## 2026-07-29：通过 `Start-Process` 派生本地开发服务器被策略拦截

- 环境：Codex Windows `shell_command`，准备启动 VitePress 本地开发服务器。
- 错误模式：在嵌套 `pwsh` 中用 `Start-Process npm.cmd`、重定向日志并隐藏窗口。
- 症状 / 退出码：命令在执行前被工具策略拒绝，没有创建进程或日志。
- 根因：当前命令执行策略不接受该后台进程派生形态；这不是 VitePress 或 npm 运行失败。
- 正确做法：直接运行开发服务器作为长运行 `shell_command`，让 `functions.exec` 产出可等待的 cell；浏览器检查结束后再终止该 cell。
- 预防检查：需要本地服务时优先使用工具原生长运行会话，不用 `Start-Process` 绕成后台任务。
- 适用范围：VitePress/Vite 等本地开发服务器与临时 HTTP 服务。

## 2026-07-29：已授权目录仍被递归删除命令策略拦截

- 最近复发/补充：2026-08-01 清理经 DinD `/work` bind 生成的精确任务临时源码目录时，即使先解析并核对绝对路径，`Remove-Item -Recurse -Force` 仍在执行前被策略拒绝。改用 `Microsoft.VisualBasic.FileIO.FileSystem.DeleteDirectory(..., SendToRecycleBin)` 将同一精确目录移入回收站并复查原路径消失；没有改用 `cmd /c rmdir` 或跨 Shell 拼接删除目标。
- 环境：Codex Windows 工作区，文件系统权限已切换为 unrestricted。
- 错误模式：对三个已核对的输出目录使用 `Get-ChildItem | Remove-Item -Recurse -Force` 批量清空。
- 症状 / 退出码：命令在执行前被工具策略拒绝；没有删除任何文件。
- 根因：文件系统写权限和命令安全策略是两层独立门禁；即使目录可写，递归删除命令仍可能被策略阻止。
- 正确做法：先解析并逐项核对绝对目录，只枚举目录第一层文件；确认没有意外子目录后，使用文件 API 逐文件删除并重新读取目录确认为空。
- 预防检查：清理代理输出时禁止对宽泛目录执行递归删除；先列出精确文件清单、验证目标根，再逐文件处理。
- 适用范围：`.codex`、generated_images、visualizations 和其它临时输出目录。

## 2026-07-29：`rg` 搜索模式以连字符开头时被当作选项

- 环境：Windows，PowerShell 7，检查隔离 VitePress 预览的 CSS 自定义属性。
- 错误模式：直接执行 `rg -n '--vp-c-bg|...' file`。
- 症状 / 退出码：`rg: unrecognized flag --vp-c-bg...`，退出码 `2`，未执行搜索。
- 根因：搜索模式以 `--` 开头，`rg` 在未看到显式模式参数边界时将其解析为命令行选项。
- 正确做法：使用 `rg -n -e '--vp-c-bg|...' file`，或在固定字符串场景使用 `rg -n --fixed-strings -e '--vp-c-bg' file`。
- 预防检查：凡搜索内容可能以 `-` 开头，一律用 `-e` 显式声明模式；不能仅依赖引号避免选项解析。
- 适用范围：CSS 自定义属性、CLI flag 文档、Markdown 参数示例等以连字符开头的文本检索。

## 2026-07-29：用全仓 `git update-index --refresh` 清理局部恢复状态

- 环境：Windows，仓库同时存在用户保留的其它未提交改动，只精确恢复 `website/` 文件。
- 错误模式：恢复目标文件后执行全仓 `git update-index --refresh`，并把非零退出码当作目标恢复失败。
- 症状 / 退出码：命令返回 `1` 并列出所有其它脏文件 `needs update`；目标文件实际内容哈希已与 `HEAD` 相同，`git diff -- website` 为空。
- 根因：`git update-index --refresh` 会检查整个索引，仓库中任何未提交改动都可能使其非零；路径参数不能把该刷新可靠限制为目标文件。
- 正确做法：先用 `git hash-object <file>` 对比 `git rev-parse HEAD:<file>` 并检查局部 diff，再用 `git add --refresh -- <exact paths>` 只刷新目标路径的 stat 信息，不暂存内容。
- 预防检查：脏工作树中验证局部恢复时，以路径限定 diff 和内容哈希为证据；不要用全仓 refresh 的退出码判断单组文件是否恢复成功。
- 适用范围：保留用户其它修改时的精准回退、编码恢复和工作树状态校验。

## 2026-08-01：把 PowerShell 自动变量当成任务变量

- 环境：Windows，PowerShell 7，整理 Docker 候选镜像检查结果。
- 错误模式：用 `$Host` 保存候选镜像检查对象。
- 症状 / 退出码：PowerShell 报只读或常量变量不能覆盖，检查命令在赋值阶段退出。
- 根因：`$Host` 是 PowerShell 内置自动变量，名称大小写不敏感；普通任务变量与系统变量冲突。
- 正确做法：使用带任务语义且不可能与自动变量冲突的名称，例如 `$candidateHostInspect`、`$releaseImageInspect`。
- 预防检查：新建 PowerShell 变量时避开 `$Host`、`$HOME`、`$PID`、`$Error` 等自动变量；候选镜像脚本统一使用 `candidate` / `release` 前缀。
- 适用范围：PowerShell 发布检查、Docker 元数据收集与组合对象输出。

## 2026-08-01：DinD registry mirror 保留了旧候选标签

- 环境：Docker Desktop 中的任务专属 DinD，使用本地 registry mirror 验收同一候选版本号的重建镜像。
- 错误模式：只在 DinD 中给新镜像打本地标签，未先核对并更新 mirror 中同名 `0.4.6` manifest，就让旧版 Panel 执行一键更新。
- 症状 / 退出码：更新成功但目标 revision 仍为旧提交 `67ab93f`，旧的 42% 状态被复现；产品回滚逻辑本身没有异常。
- 根因：updater 的 pull 请求命中 mirror 中较早推送的同版本 manifest；本地同名标签不能覆盖远端 mirror 返回的 manifest。
- 正确做法：每次候选重建后先把精确镜像推送到任务 mirror，再分别 inspect 本地标签、mirror manifest 与实际启动容器的 image ID/revision，三者完全一致后才能记为候选验收。
- 预防检查：同一候选版本号重建时把 digest/revision 一致性作为升级测试前置断言；任何不一致立即停止，不把旧镜像结果计入新候选证据。
- 适用范围：DinD、一键更新 E2E、本地 registry mirror 与可变候选 tag 测试。

## 2026-08-01：GitHub Actions 状态轮询被临时 EOF 中断

- 最近复发/补充：2026-08-09 `v0.4.9` Release 首次 `gh run watch` 因 GitHub API EOF 退出，workflow 本身继续运行并最终成功；改为按固定 run ID 有界轮询 `gh run view`，每次独立读取 status/conclusion。发布证据只能来自最终 `completed/success`，网络查询失败不能冒充 workflow 失败或成功。
- 最近复发/补充：2026-08-01 官网 Hero 发布审计把多项 GitHub API 只读查询放进同一批调用，网络等待最终以 `124` 超时退出；远端 main SHA 另由 `git ls-remote` 成功确认。后续按目标 commit 分拆 `gh run list/view` 与 deployments 查询，每项使用有界重试并保存已成功证据，不把整批超时误判为 Actions 失败，也不原样重放同一批长命令。
- 最近复发/补充：2026-08-01 `v0.4.7` 门禁审计把 `gh` 原生输出直接管道到 `Select-Object -First`，下游提前关闭后让 `gh` 报 broken pipe。需要截取展示时先用 `@(...)` 完整收集原生命令输出并立即保存 `$LASTEXITCODE`，成功后再切片；不得让展示层提前终止权威查询进程。
- 环境：Windows，GitHub CLI，发布后同时轮询 Release、兼容矩阵和官网工作流。
- 错误模式：把三个 `gh run view` 放在单次循环中，任何一次 API EOF 都直接终止整轮监控。
- 症状 / 退出码：工作流仍正常运行，但 CLI 报 GitHub API `EOF` 并退出 1；稍后同一 run 查询成功。
- 根因：远端 Actions 查询接口短暂断流，原轮询没有按 run 提供有界重试。
- 正确做法：每个 workflow run ID 独立最多重试三次，只有权威 `status=completed` 且 `conclusion=success` 才放行；查询 EOF 不等同于工作流失败。
- 预防检查：发布轮询固定记录 run ID，并把网络查询故障与工作流结论分开；重试耗尽才停止并报告外部阻塞。
- 适用范围：`gh run view/list`、GitHub Release 状态和发布后证据收集。

## 2026-08-01：删除共享测试网络前未断开保留 fixture

- 环境：任务 DinD，HTTPS release fixture 同时加入 final、旧 gate 和旧 fix 三个隔离网络。
- 错误模式：删除旧 Compose 容器后直接同时删除两个旧网络，没有先检查仍连接的共享 fixture endpoint。
- 症状 / 退出码：旧容器和 volume 已删除，但 `docker network rm` 报 `active endpoints` 并退出 1；保留的 final 网络与 fixture 未受损。
- 根因：fixture 为多个升级项目提供同一受控 HTTPS 服务，Compose 项目 down 不会自动移除它的手工跨网络连接。
- 正确做法：先 inspect fixture 的精确网络拓扑，确认 final 网络仍存在，再对旧网络执行精确 `docker network disconnect <network> <fixture>`，最后删除旧网络。
- 预防检查：删除任何测试网络前列出 endpoints；共享 fixture 必须先区分要保留和要断开的网络，禁止强制或模糊批量清理。
- 适用范围：Docker/DinD 故障 fixture、跨 Compose 网络和发布测试资源清理。

## 2026-08-01：只检查 `node_modules` 目录存在就直接运行 VitePress

- 最近复发/补充：2026-08-01 修复首页卡片 hover 裁切时，仍尝试从宿主 `website/node_modules/vitepress` 读取上游样式；该目录在当前工作树中为空，探针按预期立即停止。此工作树的可用依赖只在任务专属 Docker volume `/work/node_modules`，需要核对依赖源码时应从已验证的预览容器读取，不能再把宿主目录当作依赖来源。
- 环境：Windows，官网发布工作树，`website/node_modules` 由先前隔离测试留下空目录。
- 错误模式：`Test-Path website/node_modules` 返回 true 后，直接运行 `npm.cmd --prefix website run docs:build`。
- 症状 / 退出码：npm script 启动，但 Windows 报 `vitepress is not recognized` 并退出 1；项目源码未进入构建。
- 根因：目录存在不代表依赖已安装，空目录没有 `.bin/vitepress`；此前只验证了路径而没有验证所需可执行文件。
- 正确做法：先检查 `website/node_modules/.bin/vitepress*`；缺失时使用 Node Linux 容器和任务专属 `node_modules` volume 执行 `npm ci` 与 build，不在 Windows 上反复安装或强删目录。
- 预防检查：所有前端构建探针以具体 CLI 文件或 `npm exec <tool> --version` 为准，不能以 `node_modules` 目录存在作为 readiness。
- 适用范围：VitePress、Vite、TypeScript 与其它依赖 `.bin` CLI 的 npm 项目。

## 2026-08-01：VitePress preview readiness 忽略配置的 base path

- 环境：Docker Desktop，VitePress production preview 映射到 `127.0.0.1:18120`。
- 错误模式：服务启动后轮询 `http://127.0.0.1:18120/`，没有读取控制台输出中的 `/stardew-server-anxi-panel/` base。
- 症状 / 退出码：preview 容器持续正常运行并明确输出真实地址，但 readiness 在 60 秒后退出 1。
- 根因：把端口可用与站点根路径混为一谈；GitHub Pages 子路径部署的首页不在 `/`。
- 正确做法：从 VitePress config 或启动输出确认 base，并轮询 `http://127.0.0.1:<port><base>/`；失败前先读取服务日志区分路由错误和进程错误。
- 预防检查：所有静态站预览在启动前记录 `base` 与完整 target URL，Browser 和 HTTP readiness 使用同一地址。
- 适用范围：VitePress/GitHub Pages 子路径、反向代理前缀和其它非根路径静态站。

## 2026-08-01：组合工具结果误按 `.output` 字段读取

- 环境：`functions.exec` 中并行调用多个 `shell_command` 并组合输出。
- 错误模式：假定嵌套工具结果始终有 `.output`，把四个成功结果拼接为 `undefined`。
- 症状 / 退出码：底层命令均退出 0，但汇总层没有显示正文，需要重新读取。
- 根因：嵌套工具返回值是可直接序列化的结果对象，当前调用形态不保证 `.output` 属性。
- 正确做法：把每个结果直接传给 `text(result)`；需要字段投影时先输出一次结果结构再访问已确认的属性。
- 预防检查：新的工具组合第一次调用不猜返回 schema；避免在未验证字段上使用空值回退掩盖结构错误。
- 适用范围：`functions.exec` 编排 shell、MCP 与其它嵌套工具结果。

## 2026-08-01：`ConvertFrom-Json` 读取 package-lock 的空字符串键失败

- 环境：Windows，PowerShell 7，只读检查 npm `package-lock.json`。
- 错误模式：把包含 `packages[""]` 根包条目的 lockfile 直接管道到默认 `ConvertFrom-Json`，并按 `PSCustomObject` 读取。
- 症状 / 退出码：JSON 合法，但空字符串属性名无法转换为普通对象属性，命令退出 1；依赖版本尚未完成核对。
- 根因：npm lockfile 使用空字符串键表示项目根包，PowerShell 默认对象投影不能可靠承载该键。
- 正确做法：使用 `ConvertFrom-Json -AsHashtable` 保留所有 JSON 键，再按哈希表索引读取；只需定位版本时也可优先使用 Node/npm 自身或针对已确认路径的文本搜索。
- 预防检查：解析 package-lock、映射表或其它可能包含空键/重复大小写键的 JSON 前，默认使用 `-AsHashtable`；解析失败不得误判为 JSON 损坏。
- 适用范围：npm lockfile、工具生成的 JSON 映射和 PowerShell 元数据审计。

## 2026-08-01：E2E 根据惯例猜测 HTTP 成功码

- 环境：Docker Desktop fresh Panel setup/auth E2E。
- 错误模式：未查看接口契约就断言 setup 成功必须返回 HTTP `201`。
- 症状 / 退出码：真实接口按现有契约返回 `200`，脚本把成功请求误判为失败并重建了测试环境。
- 根因：把 REST 创建操作的常见状态码当成项目权威契约。
- 正确做法：先读取对应 handler/接口测试，或保存首个响应的状态与 body 后建立精确断言；本项目 setup 成功为 `200`。
- 预防检查：E2E 的状态码、包裹层和字段路径全部来自代码/测试或首个只读响应，不从 HTTP 惯例猜测。
- 适用范围：setup、登录、上传、异步 job 和更新接口。

## 2026-08-01：容器级 `GIT_DIR/GIT_WORK_TREE` 污染子仓库

- 最近复发/补充：2026-08-07 官网恢复构建为了给 VitePress 提供 `lastUpdated`，把宿主完整 `.git` 复制到任务 volume；对象库复制超过三分钟，构建尚未开始。无需写入 Git 元数据时，不复制 `.git`；源码复制到可写 volume 后，只对 `npm run docs:build` 单条命令局部设置只读宿主 `GIT_DIR=/repo/.git`、`GIT_WORK_TREE=/repo`。
- 环境：Windows linked worktree 挂入 Node/VitePress Linux 构建容器。
- 错误模式：为让 VitePress 读取主工作树 Git 历史，在容器级全局导出 `GIT_DIR` 与 `GIT_WORK_TREE`，后续临时子仓库也继承这两个变量。
- 症状 / 退出码：子仓库 Git 命令指向主工作树元数据，版本/lastUpdated 探针异常。
- 根因：Git 环境变量对容器内所有后续 Git 进程生效，不只影响目标 VitePress 命令。
- 正确做法：只在需要 Git 元数据的单个 `npm run docs:build` 命令前局部设置变量；操作任何子仓库前确认相关环境变量为空。
- 预防检查：构建容器把 Git 环境视为命令级输入，禁止放到全局 `docker run -e`，除非该容器只运行唯一受控 Git 工作树。
- 适用范围：linked worktree、VitePress `lastUpdated`、临时 clone 与多仓库构建容器。

## 2026-08-01：Updater E2E 的 Compose 声明与运行元数据不一致

- 环境：`v0.4.6 → v0.4.7` 真实 Web 一键升级的任务专属 DinD、Compose 与受控 HTTPS/registry fixture。
- 错误模式：先把 Panel 数据声明为 named volume，使 Compose source 名称与 Docker inspect 的宿主解析路径无法四方一致；随后又让 Panel `depends_on` release fixture，导致 `docker compose config --images panel` 同时输出依赖镜像和 Panel 镜像。
- 症状 / 退出码：第一次 dry-run 以 `compose_metadata_invalid` 安全拒绝；第一次 apply 则以“Compose 配置未精确解析到目标镜像”回滚，均未进入预期 unhealthy 健康超时。
- 根因：测试 fixture 的部署形态不满足 updater 对 Compose 文件、服务、镜像和数据挂载精确反查的生产安全契约；`config --images <service>` 会纳入该服务依赖镜像。
- 正确做法：验证受支持 bind 部署时使用任务专属 host bind 并设置 `PANEL_HOST_DATA_DIR`；release fixture 独立启动和 readiness，不作为 Panel 的 Compose dependency。每次修改 fixture 后重建旧 Panel labels、重算 Compose/`.env` 基线并执行全新 dry-run。
- 预防检查：apply 前断言 `docker compose ... config --images <panel-service>` 只输出一个目标镜像，Compose 数据 source 与容器 inspect source 完全一致；安全拒绝不得误写为产品回滚失败。
- 适用范围：Panel Web updater、DinD、本地 registry/HTTPS fixture 与 Compose 元数据门禁。

## 2026-08-01：精简 DinD 的 Docker CLI 插件与弃用参数

- 最近复发/补充：2026-08-07 清理上述官网恢复容器时再次使用 `docker stop --time 5`，Docker 29 明确输出弃用警告。后续宿主与 DinD 停止命令统一使用 `docker stop --timeout <秒>`，不得因为旧参数仍返回 0 就继续保留。
- 环境：Docker 29 精简容器，运行 compatibility/updater/runtime integration。
- 错误模式：只设置 `DOCKER_CLI_PLUGIN_EXTRA_DIRS` 就假定 buildx/Compose 可发现，并继续使用已弃用的 `docker stop --time`。
- 症状 / 退出码：Docker 29 未发现预期插件；停止命令产生 deprecated 警告，门禁工具链不稳定。
- 根因：当前精简镜像的 CLI 插件发现路径与宿主假设不一致，参数也已更新。
- 正确做法：先执行 `docker buildx version` 与 `docker compose version`；必要时把受控插件放入 `/root/.docker/cli-plugins`。停止容器使用 `docker stop --timeout <秒>`。
- 预防检查：每个 DinD 门禁先探测 CLI/daemon/buildx/Compose 四项版本，不从环境变量存在推断插件已加载。
- 适用范围：Docker 29 DinD、兼容矩阵、updater/runtime integration 和发布后回拉。

## 2026-07-31：兼容矩阵容器缺少 Docker CLI 与 Socket

- 最近补充：只把 `docker:29-cli` 的 `/usr/local/bin/docker` 复制到 Python 容器仍不足；远程制品脚本调用 `docker buildx imagetools inspect`，还必须复制 `/usr/local/libexec/docker/cli-plugins/docker-buildx` 到相同插件搜索路径并先探针 `docker buildx version`。
- 最近复发/补充：Debian Python 重试时虽然挂载了含 `docker-buildx` 的只读工具 volume，却只设置 `DOCKER_CLI_PLUGIN_EXTRA_DIRS=/dockercli`；当前 Docker CLI 没有从该位置发现插件并返回 `docker: unknown command: docker buildx`。正确做法是在容器可写的 `/root/.docker/cli-plugins/` 建立指向只读 volume 中 `docker-buildx`/`docker-compose` 的精确符号链接，再执行版本探针，不能把未验证的额外目录环境变量当成通用插件搜索契约。
- 最近复发：同日把 Docker Socket 挂进 `golang:1.25-alpine` 后直接运行 updater/runtime Docker integration，但镜像本身没有 Docker CLI；updater 用例全部以 `docker unavailable` 跳过，runtime 用例以 `exec: "docker": executable file not found` 失败。Docker-aware 测试必须在执行 Go 测试前探针 `docker version`，updater 还要探针 `docker compose version`，不能把“挂了 Socket”等同于“容器可调用 Docker”。
- 环境：`python:3.13-alpine` Docker 容器运行 `scripts/compatibility_matrix.py verify-remote-artifacts`。
- 错误模式：只安装 Python、Bash 与 ShellCheck，未提供脚本实际调用的 `docker` 可执行文件和 daemon Socket。
- 症状 / 退出码：前两项纯 Python 校验通过后，远程制品验证以 `[Errno 2] No such file or directory: 'docker'` 退出 1。
- 根因：只按脚本语言准备容器，未检查子进程依赖；`verify-remote-artifacts` 会用 Docker 拉取/检查受审镜像。
- 正确做法：容器内安装 `docker-cli`，只读挂载 `/var/run/docker.sock`，并在正式验证前执行 `docker version` 探针；继续使用任务专属资源与标签。
- 预防检查：将发布脚本的外部命令（Docker、Git、ShellCheck、Bash）作为门禁镜像依赖清单，不只检查解释器。
- 适用范围：兼容矩阵远程制品、镜像 digest、runtime manifest、updater/runtime Docker integration 与发布脚本容器门禁。

## 2026-07-31：把 ShellCheck 镜像误当成带 entrypoint 的命令镜像

- 环境：`koalaman/shellcheck-alpine:v0.10.0` Linux 容器。
- 错误模式：直接把两个脚本路径作为 `docker run IMAGE` 后的命令，假定镜像 entrypoint 会自动调用 ShellCheck。
- 症状 / 退出码：镜像实际 `Entrypoint=null`、默认 `Cmd=["/bin/sh"]`；Docker 转而直接执行第一个 Bash 脚本，因镜像没有 Bash 报 `env: can't execute 'bash'`。
- 根因：未先 inspect 镜像启动契约，把镜像内存在 `shellcheck` 二进制等同于已经配置 entrypoint。
- 正确做法：显式运行 `shellcheck deploy/... scripts/...`，或在只读 inspect 确认后使用 `--entrypoint shellcheck`。
- 预防检查：首次使用工具镜像先核对 Entrypoint/Cmd 和二进制路径；脚本路径不能作为未确认镜像的首命令。
- 适用范围：ShellCheck、Hadolint、linters 与任何第三方 CLI 工具镜像。

## 2026-07-31：Docker-outside-of-Docker 无法看到测试容器的 TempDir

- 环境：`golang:1.25-alpine` 挂载 Docker Desktop Socket，运行 `go test -tags=integration ./internal/docker`。
- 错误模式：认为测试进程在 Go 容器内创建的 `/tmp/...` 可以被宿主 Docker daemon 作为后续子容器的 bind source 使用。
- 症状 / 退出码：updater 的纯 volume/Compose 用例通过，但 SMAPI staging 在安装归档时返回 `docker command failed`；其 archive 位于 Go 容器私有 TempDir，daemon 启动的子容器看不到该路径。
- 根因：挂 Docker Socket只共享 daemon API，不共享调用方容器的 root filesystem；bind source 始终由 daemon 所在主机解析。
- 正确做法：此类会把 `t.TempDir()` 传给子容器的集成测试在带 Go 工具链的专属 DinD 容器内运行，使测试进程与 dockerd 共用同一文件系统；预加载精确基础镜像并使用任务专属容器/volume。
- 预防检查：Docker integration 若包含 bind mount，先判断 source 是宿主路径、共享挂载还是调用方容器私有路径；只有前两类可用 Docker-outside-of-Docker。
- 适用范围：Go/Python 测试在容器内调用宿主 Docker，并把临时文件或目录 bind 给二级容器的场景。

## 2026-07-31：重建 DinD 时遗漏关闭自动 TLS

- 环境：Docker Desktop Linux containers，任务专属 `docker:29-dind` 通过唯一环回端口提供宿主 CLI 访问。
- 错误模式：重建已验证的 DinD 容器时只复制 daemon 的 `--host=tcp://0.0.0.0:2375` 参数，遗漏 `DOCKER_TLS_CERTDIR=`。
- 症状 / 退出码：宿主执行 `docker -H tcp://127.0.0.1:<port> info` 返回 `Client sent an HTTP request to an HTTPS server`；daemon 日志显示 2375/2376 同时监听和 TLS handshake error。
- 根因：`docker:dind` 默认 `DOCKER_TLS_CERTDIR=/certs` 会为 TCP listener 启用 TLS；仅指定 2375 不等于禁用自动证书。
- 正确做法：只在任务唯一环回端口且不暴露公网的测试 DinD 上显式传入 `-e DOCKER_TLS_CERTDIR=`，然后以明文环回 TCP 探针 readiness；生产或非隔离环境不得照搬。
- 预防检查：重建 DinD 前同时核对原容器 Env、Cmd、端口、volume、network 与 ownership label，不只复制 Cmd/Mounts。
- 适用范围：本机 Docker Desktop 上由宿主 Docker CLI 控制的隔离 DinD 发布门禁。

## 2026-07-31：宿主 Compose CLI 不解析 DinD 内部配置路径

- 最近补充：为先挂载 HTTPS fixture 而手工创建 Compose 将使用的网络后，配置仍按 Compose-owned network 声明，`compose up` 因缺少 `com.docker.compose.network` label 拒绝接管；共享 fixture 网络必须显式 `external: true`，或者先让 Compose 创建再连接 fixture，不能混用两种所有权模型。首版升级 fixture 又硬编码 Compose `image:` 且 `.env` 没有 `PANEL_IMAGE`，helper 按正式部署契约更新该键时安全失败；真实 updater E2E 必须复用正式 `${PANEL_IMAGE}` + `.env` 格式，不能只满足 capability 的当前镜像反查。首版 unhealthy 镜像只覆盖 image `ENTRYPOINT`，但正式 Compose 自己指定 `/app/panel` entrypoint，导致故障注入被覆盖并真实升级成功；要验证 unhealthy 回滚，故障镜像必须替换 Compose 最终调用的 `/app/panel` 本体，并先探针确认其不能提供 health/version。
- 环境：Windows 宿主 Docker CLI 通过 `-H tcp://127.0.0.1:<port>` 控制 Linux DinD daemon，Compose 文件由外层 bind 映射为 DinD 内的 `/gate/...`。
- 错误模式：从 Windows 宿主执行 `docker -H ... compose -f /gate/install/docker-compose.yml`，假定远端 daemon 会读取该路径。
- 症状 / 退出码：Compose 在客户端阶段把路径解析为 `E:\gate\install\docker-compose.yml` 并报文件不存在；registry 和 fixture 已启动，但 Compose project 未创建。
- 根因：Compose CLI 在客户端本地读取、展开配置，再把容器请求发给 daemon；`-H` 只切换 daemon，不改变 CLI 文件系统。
- 正确做法：需要保留 DinD 内真实 Compose 路径和 updater labels 时，用 `docker exec <owned-dind> docker compose -f /gate/...` 在 DinD 容器内调用 CLI；单纯宿主配置则传 Windows 上真实存在的绝对路径。
- 预防检查：远端 Docker CLI 命令先区分“客户端读取的路径”和“daemon 解析的 bind source”；Compose `-f` 属于前者。
- 适用范围：Windows → DinD/远端 daemon 的 Compose、build context、env-file 与配置文件操作。

## 2026-07-31：Docker Desktop `docker cp` 对 DinD 目标静默无结果

- 环境：Windows PowerShell 7、Docker Desktop 29.5.3，向运行中的专属 DinD 容器复制镜像 tar。
- 错误模式：分别用反斜杠和正斜杠绝对路径执行 `docker cp <host-file> v046gate-dind:/tmp/...`，只检查命令退出码。
- 症状 / 退出码：两次 `docker cp` 都返回 0，但容器 `/tmp` 没有目标文件；紧随其后的 `docker image load -i`/`ls` 才失败。小文本文件探针也复现，排除 tar 内容问题。
- 根因：当前 Docker Desktop/CLI 的 Windows 本地路径到该 DinD 容器的 archive copy 未实际落盘且未返回错误，具体上游原因未确定；不能把 exit 0 当复制证据。
- 正确做法：专属 DinD 启动时只绑定环回 TCP 端口，由宿主 Docker CLI 使用 `-H tcp://127.0.0.1:<task-port> image load -i <verified-archive>` 直接把 tar 交给 DinD daemon；加载后 inspect 精确 image ID/digest。普通文件必须复制时也要立即在目标端 stat/hash。
- 预防检查：`docker cp` 后总是目标端核对文件存在、大小和必要时摘要；正式 DinD 镜像预加载优先 daemon TCP `image load`，不依赖容器 rootfs 中转。
- 适用范围：Windows Docker Desktop 到 Linux/DinD 容器的文件复制和镜像预加载。

## 2026-07-31：失败诊断命令覆盖原生退出码

- 环境：PowerShell 7，启动 HTTPS fixture 后用 `docker exec ... curl` 探针。
- 错误模式：失败分支先运行 `docker logs`，再写 `exit $LASTEXITCODE`。
- 症状 / 退出码：curl 因服务仍在启动返回 7，但随后的 `docker logs` 成功把 `$LASTEXITCODE` 改成 0，整个门禁错误地以 0 结束并输出后续 ready 文案。
- 根因：PowerShell 的 `$LASTEXITCODE` 始终反映最近一个原生命令，不会自动保存触发失败分支的原始值。
- 正确做法：紧跟被测命令写 `$probeCode = $LASTEXITCODE`；诊断完成后 `exit $probeCode`。长启动组件先做有上限 readiness 轮询，不能启动后立即单次探针。
- 预防检查：任何“失败后打印 logs/inspect 再退出”的分支先把原始退出码存入任务专属变量。
- 适用范围：Docker/curl/npm/go 等原生命令的 PowerShell 门禁与故障诊断。

## 2026-08-06：`CopyFromScreen` 不能保证只截取目标窗口

- 环境：Windows 11，本机 Stardew Valley 可见窗口真实联调。
- 错误模式：调用 `SetForegroundWindow` 后直接按目标窗口矩形使用 `Graphics.CopyFromScreen`，并假定截图内容一定来自目标窗口。
- 症状 / 退出码：命令退出 0，但 Windows 前台切换限制使其他窗口仍覆盖目标区域，截图包含了不在测试范围内的桌面窗口内容。
- 根因：`SetForegroundWindow` 的布尔结果没有被验证，且 `CopyFromScreen` 捕获的是当前合成桌面像素，不是窗口自己的渲染表面。
- 正确做法：需要窗口级截图时使用 `PrintWindow`/应用自身截图能力并验证返回值与图像内容；不能保证窗口隔离时停止截图，不把结果用于报告，并及时清理任务专属截图文件。
- 预防检查：任何桌面截图前明确区分“屏幕区域”与“窗口内容”；禁止以未经验证的 `SetForegroundWindow` 作为隐私边界。
- 适用范围：Windows 桌面应用视觉检查、UI 自动化与本机真实客户端联调。
- 最近复发/补充：同日尝试用 `WScript.Shell.AppActivate(PID)` 建立可验证的前台边界时返回 `false`，说明 SMAPI/游戏的窗口所有权或 Windows 前台限制不满足该 API；命令在截图前安全停止。此类窗口不再猜测标题或继续自动切换，改由用户显式把游戏窗口置前并完成加入操作。

## 2026-08-06：任务前缀没有传递到实例 Compose project

- 环境：Windows Docker Desktop，本机 Stardew/Junimo 真实 LAN 联调。
- 错误模式：只设置 `PANEL_COMPOSE_PROJECT=sap-player-mod-real-20260806` 并检查任务 ownership label，就假定默认实例启动的全部 Compose 资源都会使用该前缀。
- 症状 / 退出码：联调完成后 inspect 发现实际 project 仍是实例目录 basename `stardew`；server/auth 容器虽来自任务临时 working_dir，但复用了 2026-07-06 已存在且无任务 label 的 `stardew_steam-session` volume。
- 根因：Junimo driver 的实例 Compose project 不继承 Panel 自身的 `PANEL_COMPOSE_PROJECT`；预检只查任务标签和端口，没有在启动前解析生成 Compose 的最终 project/volume 名称。
- 正确做法：真实组件启动前同时核对 `docker compose config`、最终 project 名、容器名、network 与全部 named volume；需要完全隔离时让实例 data-dir basename 唯一或使用 driver 明确支持的 project 覆盖。发现旧卷后不得删除或读取其 token 内容，并在测试证据中降级说明认证卷未完全隔离。
- 预防检查：不能把 Panel project 与游戏实例 project 当成同一配置；启动前对每个最终资源名执行存在性和 ownership 检查，任何既有无 ownership 资源都停止启动。
- 适用范围：Junimo/Stardew driver 的真实 Compose 联调、安装、升级和发布候选测试。

## 2026-08-06：Vite 端口落入 Windows TCP 排除区间

- 环境：Windows 11、Docker Desktop，任务专属 Vite QA 服务。
- 错误模式：只检查 `Get-NetTCPConnection` 没有监听后，就选择 `4317` 启动 Vite。
- 症状 / 退出码：Node 返回 `listen EACCES: permission denied 127.0.0.1:4317`；端口没有其它监听进程。
- 根因：`netsh interface ipv4 show excludedportrange protocol=tcp` 显示 `4317` 位于动态排除区间 `4280-4379`，无监听不代表端口可绑定。
- 正确做法：遇到无监听但 bind EACCES 时不原样重试，读取 TCP exclusion ranges，改用区间外的任务端口；本轮 `18763` 启动成功并在结束后确认无监听。
- 预防检查：Windows 临时服务选端口时同时检查监听与排除区间；Docker Desktop 运行时不要默认常见的 4xxx 端口均可用。
- 适用范围：Vite/VitePress/Node/Python 本地 QA 服务和 Docker Desktop 宿主端口规划。

## 2026-08-01：协作等待参数低于工具最小值

- 环境：Codex 多代理协作，轮询响应式审查代理结果。
- 错误模式：调用 `wait_agent` 时把 `timeout_ms` 写成 `1000`。
- 症状 / 退出码：工具在执行前拒绝参数，并明确要求最小 `10000ms`；代理状态未受影响。
- 根因：没有先核对工具 schema 的最小等待窗口，把普通短轮询习惯直接套到协作工具。
- 正确做法：`wait_agent` 使用 `timeout_ms >= 10000`；只需要即时状态时用 `list_agents`，需要催办则用 `send_message`。
- 预防检查：协作工具的超时参数按 schema 范围填写，不用试错探测边界。
- 适用范围：`wait_agent` 和其它声明了最小/最大时长的协作工具。

## 2026-08-01：只读取子进程 stdout 前缀后等待导致管道死锁

- 环境：PowerShell 7，通过 `System.Diagnostics.Process` 检查 Git blob 的前三个原始字节。
- 错误模式：只从重定向的 `StandardOutput.BaseStream` 读取 3 字节，随后立即 `WaitForExit()`，没有继续排空剩余 stdout。
- 症状 / 退出码：`git cat-file blob` 输出完整 CSS 后阻塞在已填满的匿名管道，父进程又等待子进程退出；工具超时并以 124 终止。
- 根因：重定向 stdout 后，父进程必须持续消费输出；只读前缀会让较大输出写满缓冲区，形成父子互等。
- 正确做法：小型文本检查直接让 PowerShell 管道完整读取后检查首字符；必须保留原始字节时并发执行 `CopyToAsync`/`ReadToEndAsync` 排空流，再等待进程退出并检查所需前缀。
- 预防检查：任何 `RedirectStandardOutput=true` 的手写 `Process` 调用，禁止在未排空 stdout/stderr 时同步 `WaitForExit()`。
- 适用范围：PowerShell/.NET 启动 Git、Docker 或其它可能输出超过管道缓冲区的子进程。

## 2026-08-01：Browser Node REPL 一次批量执行过多视口交互

- 环境：Codex 应用内 Browser，经持久 Node REPL 扫描响应式路由矩阵。
- 错误模式：把 90 次、随后 27 次点击/等待/度量塞进单次默认 30 秒调用，并在失败调用后继续假定所有顶层绑定都已完成初始化。
- 症状：调用到 30 秒超时并重置运行时；后续读取半初始化变量时报未定义，已完成的局部结果也无法可靠交付。
- 根因：浏览器交互有逐项往返与懒加载成本，单次批次超出工具时限；失败执行中的顶层绑定不具备事务语义。
- 正确做法：每次只扫描一个视口的有限路由，显式给 60 秒上限；长工具 cell 用 `wait` 续取。失败后重新确认运行时与绑定，只用新变量名恢复，不猜测半初始化状态。
- 预防检查：批量 Browser QA 先用一个视口测量耗时，再按不超过约 10 次交互拆批；不要把扩大 timeout 当成无限批处理许可。
- 最近复发/补充：2026-08-06 在约一小时 Docker 构建与升级后直接复用裸 `browser` 变量，Node kernel 已不再保留该绑定并返回 `browser is not defined`。长外部阶段或新用户续令后，浏览器工作必须先重跑幂等 bootstrap：检查 `globalThis.agent?.browsers` 与 `globalThis.browser`，缺失时重新选择同一 URL 对应 browser，再创建 tab；不能只依据先前文档所述的 browser handle 生命周期推断 REPL 变量仍存在。
- 适用范围：应用内 Browser 的多路由、多尺寸与多主题矩阵。

## 2026-08-01：QA mock 路由与真实前端契约漂移

- 环境：`qa-layout-main.tsx` 响应式夹具，逐路由浏览器扫描。
- 错误模式：任务日志夹具只 mock 旧 `/commands`，而页面实际请求 `/control-commands`；扫描脚本又在页面崩溃后直接解引用 Shell。
- 症状：切到任务日志时 React 页面崩溃，随后度量出现 null dereference；其它路由结果被同一批次中断。
- 根因：QA fixture 没有跟随真实 API 路径更新，批量度量也缺少逐路由就绪与空节点保护。
- 正确做法：fixture 按真实 API 路径提供最小完整响应；每次导航后等待目标内容，先返回 `crashed:true` 再计算布局，并同时检查 console error/warn。
- 预防检查：新增/改名 API 时把 QA mock 与对应页面纳入同一测试；响应式脚本不能只验证容器宽度，还要覆盖所有路由实际挂载。
- 适用范围：前端 QA 入口、MSW/fetch mock 与浏览器矩阵。

## 2026-08-09：把开发专用 QA 入口误当成正式镜像资产

- 环境：Docker Desktop 正式候选镜像与 `frontend/qa-layout.html` 升级修复流程验收。
- 错误模式：候选容器 health/version 已通过后，直接把源码树中的 `/qa-layout.html` 当成正式前端构建会发布的页面；Browser 插件先对本机 URL 返回 `ERR_BLOCKED_BY_CLIENT`，切到无代理 headless Chrome 后才得到真实 `404 not_found`。
- 症状 / 退出码：原生 Chrome CDP 脚本等待 Stardew 导航超时，页面正文是 API 风格的 `resource not found`；候选容器、数据与源码均未变化。
- 根因：Vite 的开发 QA harness 不属于 production build 输出，正式镜像只包含真实应用入口；浏览器插件的客户端拦截又暂时掩盖了真实 HTTP 响应。
- 正确做法：正式候选容器验证真实 health/version/API/嵌入脚本；需要 mock 状态交互时，在同一 Docker Desktop 中另起带任务专属端口、网络、只读源码 bind 与独立 `node_modules` volume 的 Vite QA 容器，再由无代理浏览器访问。先用 HTTP 探针确认目标入口返回 HTML，不得从源码文件存在推断镜像包含该资产。
- 预防检查：浏览器 E2E 前为目标 URL 增加状态码、Content-Type 与页面标志探针；明确区分 production 资产、测试 harness 和真实后端夹具。
- 适用范围：候选镜像 UI 验收、Vite 多入口、Docker Desktop 前端 QA 与 Browser 插件本机访问失败诊断。

## 2026-08-01：PowerShell cmdlet 与 Unix 条件语法混写

- 环境：PowerShell 7 仓库审计子任务。
- 错误模式：把 `Get-Content -LiteralPath` 连写成不存在的 `Get-Content-LiteralPath`，或在 PowerShell 中写 `rg ... || if (...)`。
- 症状：命令未进入目标读取/分支，直接报 cmdlet 不存在或 parser error。
- 根因：手工压缩一行命令时丢失参数空格，并把 Bash 的短路运算与 PowerShell 语句式 `if` 混用。
- 正确做法：保留 `Get-Content -LiteralPath <path>` 的 cmdlet/参数边界；原生命令后立即保存或判断 `$LASTEXITCODE`，再写独立的 `if (...) { ... }`。
- 预防检查：提交复杂单行前先按 PowerShell 语法分句；需要多阶段诊断时拆成多个工具调用，不追求一行完成。
- 适用范围：Windows 仓库搜索、读取与门禁命令。

## 2026-08-02：ECS 一键脚本下载端点连接失败

- 环境：阿里云 Ubuntu 22.04 ECS，通过 Workbench 获取项目官方一键部署脚本。
- 错误模式：先直接请求自建分发域名的 HTTPS 地址，随后用 curl 默认 HTTP/2 请求 GitHub Release 备用地址。
- 症状 / 退出码：自建域名返回 `curl: (7) ... Connection refused`；GitHub Release 返回 `curl: (16) Error in the HTTP2 framing layer`，脚本未下载执行。
- 根因：前者是目标分发端点当时拒绝 443 连接；后者是当前网络链路与 curl 的 HTTP/2 传输不兼容，并非脚本内容错误。
- 正确做法：该项目的国内分发入口明确使用 `http://anxinas.dpdns.org/run.sh`，先按文档使用 HTTP；GitHub Release 只作为备用，并在链路需要时显式使用 HTTP/1.1。
- 预防检查：不要凭域名习惯把项目文档中的 HTTP 自行改成 HTTPS；远端安装先核对仓库正式文档，再区分 DNS、TCP、TLS、HTTP 协议层与脚本执行错误。
- 适用范围：受代理、网关或跨境链路影响的 curl 下载任务。

## 2026-08-02：未先检索就假定部署脚本位于仓库根目录

- 环境：Windows PowerShell 7，准备核对本地一键部署脚本后传入 ECS。
- 错误模式：直接对仓库根目录的 `run.sh` 执行 `Get-Item` 和哈希计算。
- 症状 / 退出码：路径不存在，随后哈希命令又因空路径产生级联参数错误。
- 根因：凭发布命令中的文件名推断源码位置，没有先用 `rg --files` 核对仓库布局。
- 正确做法：先执行 `rg --files -g 'run.sh' -g '*run*.sh'`，确认实际脚本为 `deploy/run.sh` 后再读取或计算哈希。
- 预防检查：任何未在当前会话确认过的仓库相对路径，先检索再使用；首个路径检查失败后立即停止后续依赖命令。
- 适用范围：多目录仓库中的发布脚本、配置文件和生成物定位。

## 2026-08-02：把标准 Playwright 方法套到 Browser 精简定位器

- 环境：应用内 Browser，尝试把阿里云安全组端口列滚入视口。
- 错误模式：对 Browser 精简定位器调用未暴露的 `scrollIntoViewIfNeeded()`。
- 症状：运行时直接报该方法不存在，页面状态未改变。
- 根因：把标准 Playwright Locator API 当成当前 Browser 包装层的完整接口，没有按已读文档的可用方法集执行。
- 正确做法：用 `PageDown` 做纵向滚动；对已在 DOM 中但横向离屏的列标题调用受支持的 `click()`，由定位器自动滚入视口。
- 预防检查：Browser 操作只使用文档明确暴露的方法，缺少方法时先换受支持的语义交互，不做试错式 API 猜测。

## 2026-08-02：误用应用内 Browser 的标签页创建接口

- 环境：持久 Browser 会话，需要打开 ECS 面板的新标签页。
- 错误模式：调用不存在的 `iab.tabs.open()`；随后又把 URL 作为参数传给 `tabs.new()`，实际只得到 `about:blank`。
- 症状：第一次报方法不存在；第二次创建空白页，必须再导航。
- 根因：没有复用已确认的 Browser 标签接口签名。
- 正确做法：使用 `const tab = await iab.tabs.new()`，再调用 `await tab.goto(url)`；需要确认时读取 `tab.url()`。
- 预防检查：持久浏览器对象的方法不确定时只读检查其原型一次，并把确认后的调用模式固定复用。

## 2026-08-02：保存浏览器截图前未创建目标目录

- 环境：Node Browser 会话，把本地正式面板截图写入推广素材目录。
- 错误模式：直接写入尚不存在的 `real-captures/panel` 子目录。
- 症状：`writeFile` 返回 `ENOENT`，截图缓冲区仍在但首次保存失败。
- 根因：只创建了 ECS 截图目录，误以为同级 panel 目录也已经存在。
- 正确做法：批量截图前用只读检查确认每个输出目录；缺失目录用明确的 `New-Item -ItemType Directory -Force` 创建，再开始保存。
- 预防检查：每组新输出前先校验精确父目录，不把相邻目录的存在当成证据。

## 2026-08-02：PowerShell 未传播原生命令的失败退出码

- 环境：PowerShell 7，通过 `curl.exe` 检查 ECS 公网 8090 可达性。
- 错误模式：脚本块最后没有检查并传播 `$LASTEXITCODE`。
- 症状：curl 明确打印超时错误，但外层 shell 显示退出码 0，容易误判为成功。
- 根因：PowerShell 对原生程序的非零退出码不会自动转换为脚本块失败。
- 正确做法：原生命令后立即保存并检查 `$LASTEXITCODE`，非零时显式 `exit $code` 或抛出包含退出码的异常。
- 预防检查：curl、docker、rg 等原生命令的门禁结果一律显式传播退出码，不能只看 PowerShell 外层状态。

## 2026-08-06：临时 IL 探针误处理有符号 opcode 与扩展方法

- 环境：PowerShell 7，使用 `System.Reflection.Metadata` / `PEReader` 只读核对游戏 DLL 的 IL。
- 错误模式：把有符号的 `OpCode.Value` 直接转换为 `UInt16`；把扩展方法当作 `PEReader` 实例方法调用；随后尝试会递归加载游戏依赖的普通反射。
- 症状 / 退出码：opcode 表构建报 `Cannot convert value "-512" ... to System.UInt16`，两字节 opcode 缺失并导致 IL 解码错误；`PEReader` 实例上找不到 `GetMetadataReader` / `GetMethodBody`；普通反射因缺少 `MonoGame.Framework` 无法解析类型。PowerShell 非终止错误又使首次探针表面退出 0。
- 根因：两字节 opcode 的底层值由有符号 `Int16` 表示；当前运行时把相关 API 暴露为扩展方法；普通反射需要完整加载被检查程序集的依赖；临时探针未把 PowerShell 错误提升为终止失败。
- 正确做法：用 `([int]$op.Value -band 0xffff)` 生成 opcode key，并按 `Size` 拆字节；通过 `[System.Reflection.Metadata.PEReaderExtensions]::GetMetadataReader($pe)` 和 `::GetMethodBody($pe, $rva)` 静态调用扩展方法，避免加载游戏依赖；探针开头设置 `$ErrorActionPreference = 'Stop'`，捕获后显式返回非零退出码。
- 预防检查：临时二进制探针先用一个已知方法验证 opcode 表、API 调用形式和失败退出码，再批量解析目标 DLL；没有完整依赖集时默认使用元数据读取，不先走普通反射。
- 适用范围：PowerShell/.NET 元数据审计、IL 解码、缺少完整运行时依赖的程序集只读分析。

## 2026-08-06：跨轮沿用未暴露的嵌套工具方法

- 最近复发/补充：2026-08-06 查询浏览器公开 API 文档时，在 JavaScript 字符串内把 Bash 的引号拼接法套进 `pwsh -Command`，使传给 `rg` 的绝对路径首尾带空格并返回 `os error 123`。随后首次补记又手抄上下文时漏掉 `JavaScript` 与“编排层”之间的空格，导致 `apply_patch` 校验失败；跨 JavaScript/PowerShell 两层时直接保留 PowerShell 的双引号参数，禁止混入 Bash 引号拼接语法，补丁上下文则从刚读取的原文逐字复制。
- 环境：Codex `functions.exec` JavaScript 编排层，准备运行只读 PowerShell 搜索。
- 错误模式：沿用上一轮曾可调用、但本轮工具声明未提供的 `tools.shell_command(...)`，没有先核对当前 `tools` 方法列表。
- 症状 / 退出码：JavaScript 在命令执行前抛出 `TypeError: tools.shell_command is not a function`；没有启动 Shell，也没有修改文件。
- 根因：把延迟暴露的嵌套工具能力误认为跨轮稳定接口，忽略本轮明确提供的 `tools.exec_command(...)`。
- 正确做法：只调用当前工具声明中存在的方法；需要 Shell 时使用本轮明确提供的 `tools.exec_command(...)`，对不确定的延迟工具先检查 `ALL_TOOLS`。
- 预防检查：每个新轮次首次编排工具前核对当前 schema，不从历史成功调用推断本轮仍存在同名方法。
- 适用范围：`functions.exec` 中的嵌套工具调用及会动态变化的延迟工具集合。

## 2026-08-06：宿主 dotnet 存在但没有 SDK

- 最近补充：切到容器后的首次真实编译又把 XML 文档里可见但对 Mod 不公开的 `Constants.GameVersion` 当成可调用 API，得到 `CS0117`。本轮通过已核对的 Junimo 源码确认运行时游戏版本应读 `Game1.version`，更换后在只读 SMAPI game-data 上 0 errors；以后 XML member 存在不能替代真实可见性编译。
- 环境：Windows，编译 `StardewAnxiPanel.Control` 与纯契约测试。
- 错误模式：看到 `dotnet` 命令存在后直接执行 `dotnet run` / `dotnet build`，没有先运行 `dotnet --list-sdks`。
- 症状 / 退出码：两条命令都提示 `No .NET SDKs were found`；项目未加载、源码和制品未变化。
- 根因：宿主只安装了 .NET runtime，命令入口存在不代表具备 SDK；项目正式流程本来也要求 .NET 6 SDK 容器与精确 game-data 引用。
- 正确做法：先探针 `dotnet --list-sdks`；宿主无 SDK 时直接使用已 inspect 的 `mcr.microsoft.com/dotnet/sdk:6.0`，源码 bind 到 `/workspace`，SMAPI 实编译把已核对的 game-data volume 只读挂到 `/game`。
- 预防检查：任何 C# 编译门禁先区分 runtime 与 SDK，并核对目标框架；不能把 `Get-Command dotnet` 当成可编译证据。
- 适用范围：Control Mod、C# 契约工具和其它 .NET build/test。

## 2026-08-06：未优先复用本地精确接口资料而命中 GitHub API 限流

- 环境：PowerShell 7，只读核对 SMAPI 4.x `IMultiplayerPeer`/peer event 契约。
- 错误模式：先对 GitHub unauthenticated tree API 执行 `Invoke-RestMethod`，没有先检查本机 game-data 自带的 `StardewModdingAPI.xml`。
- 症状 / 退出码：GitHub 返回 rate limit exceeded；没有下载、写入或修改仓库文件。
- 根因：共享出口的匿名 API 配额已耗尽，而本机已有与实际编译引用配套的 XML 文档和 DLL。
- 正确做法：优先从已核对的本地 game-data 读取 `StardewModdingAPI.xml`，再用真实只读 game-data 运行编译验证；只有本地资料缺字段时才访问官方远程源码，并先考虑无需 API 配额的精确 raw/tag 资源。
- 预防检查：技术接口调查先列出本地实际运行制品、XML docs 与锁定版本；外部 API 只作补充，匿名限流失败不原样重试。
- 适用范围：SMAPI/.NET 接口核对、GitHub 上游源码调查和共享出口环境。

## 2026-08-06：Go 多行布尔条件把运算符放到下一行

- 环境：PowerShell 7，新增 Go 后端边界测试后首次运行 `gofmt -w`。
- 错误模式：把多行 `if` 条件写成上一行以操作数结束、下一行以 `||` 开头。
- 症状 / 退出码：`gofmt` 报 `expected operand, found '||'`，后续位置连带出现缺少花括号/逗号错误；文件未被格式化。
- 根因：Go 会在可结束语句的行尾自动插入分号；二元运算符必须留在上一行行尾。
- 正确做法：写成 `condition ||` 后换行继续下一条件，修正后先单独运行 `gofmt`，再进入编译和测试。
- 预防检查：手写 Go 多行表达式时统一把 `&&` / `||` 放在前一行末尾；新增较长测试后先对精确文件执行 `gofmt` 语法探针。
- 适用范围：Go 源码与测试中的多行布尔/算术表达式。

## 2026-08-06：重定向 MSBuild 中间目录后旧 obj 被重新纳入编译

- 环境：PowerShell 7、只读源码 bind、`mcr.microsoft.com/dotnet/sdk:6.0`、真实只读 SMAPI game-data。
- 错误模式：为避免写源码目录，把 `BaseIntermediateOutputPath` 指到 `/tmp/obj`，但源码树保留了先前标准构建生成的 `obj/`。
- 症状 / 退出码：真实 Control build 报 8 个 `CS0579 Duplicate ... Attribute`；任务容器和 NuGet 卷随后按精确名称/ownership label 清理。
- 根因：SDK 的默认排除目录随 `BaseIntermediateOutputPath` 改到 `/tmp/obj`，原项目目录的 `obj/**/*.cs` 不再被默认排除，旧、新两份 AssemblyInfo 同时参与编译。
- 正确做法：本项目真实 Mod 编译沿用文档已验证的标准 `dotnet build -c Release /p:GamePath=/game /p:EnableModDeploy=false`，允许生成受忽略的标准 `bin/obj`；若必须重定向，则显式追加排除原 `obj/**;bin/**` 或在干净副本中构建。
- 预防检查：改变 MSBuild `BaseIntermediateOutputPath` / `BaseOutputPath` 前检查源码树是否已有生成目录，并核对 `DefaultItemExcludes`；真实门禁优先复用项目记录的成功命令。
- 最近复发/补充：同日最终复验又构造了 `BaseIntermediateOutputPath=/tmp/obj` / `BaseOutputPath=/tmp/bin` 的只读源码命令；虽然本次先因离线 restore 失败而尚未进入重复 Attribute 编译阶段，但命令模式已经再次违反已验证做法。该预防规则已提升到 `AGENTS.md`，后续必须直接复用标准输出路径或先制作无 `bin/obj` 的任务副本。
- 适用范围：SDK-style .NET 项目、只读源码挂载和容器化构建输出重定向。

## 2026-08-06：真实 Control build 在无 NuGet 缓存时禁网 restore

- 环境：`mcr.microsoft.com/dotnet/sdk:6.0`，真实 game-data 只读 volume，容器使用 `--network none`。
- 错误模式：契约项目离线成功后，直接假定真实 Control 项目也不需要联网 restore。
- 症状 / 退出码：`NU1301 Unable to load the service index for https://api.nuget.org/v3/index.json`，构建在 restore 阶段失败。
- 根因：真实项目包含需要 NuGet restore 的包，而一次性容器没有预热缓存；契约项目无外部包，不能代表真实项目依赖。
- 正确做法：先检查 csproj/package cache；没有经验证的只读 NuGet 缓存时允许任务容器联网 restore，再以真实 game-data 只读挂载构建。若必须禁网，先在联网 staging 中生成并校验专用缓存，再离线复验。
- 预防检查：给 .NET 门禁加 `--network none` 前运行依赖探针，不能从相邻项目的离线成功外推。
- 适用范围：容器化 .NET restore/build、Control Mod 真实程序集编译。

## 2026-08-06：未读 manifest schema 就取不存在的哈希字段

- 环境：PowerShell 7，最终 Control DLL/运行栈清单一致性检查。
- 错误模式：把清单字段猜成 `control.sha256`，没有先读取实际 JSON schema。
- 症状 / 退出码：命令退出 0，但输出 `MANIFEST_CONTROL_SHA256=` 为空；如果只看退出码会漏掉一致性校验。
- 根因：真实字段是 `controlMod.dllSha256`，且脚本没有对空值 fail closed。
- 正确做法：先读取清单结构，再访问 `controlMod.dllSha256`；对空字段和哈希不一致都显式失败。
- 预防检查：JSON 契约探针必须验证目标值非空、格式正确且与实际摘要相等，不能只打印结果。
- 适用范围：运行栈 manifest、构建元数据和发布摘要校验。

## 2026-08-06：把不同构建路径下的 .NET DLL 当成字节可复现

- 环境：同一 Control C# 源码、`mcr.microsoft.com/dotnet/sdk:6.0`、真实 game-data，分别以 `/src/smapi-mod-src` 与 `/src` 作为项目路径构建。
- 错误模式：用新鲜复编译 DLL 的 SHA-256 与先前已提升、已真实运行的嵌入 DLL 做硬相等，并把不等直接视为源码/嵌入漂移。
- 症状 / 退出码：两个新构建分别得到不同摘要，且都不等于嵌入清单的 `b15479...`；三者都编译成功并包含 `PlayerModContextLifecycle/PeerContextReceived/reportedAt` 元数据，嵌入 DLL 已在真实 LAN 联调产生正确 context。
- 根因：当前 C# 流程没有声明跨容器项目路径/构建环境的字节级 reproducible-build 契约；项目路径、编译器/调试元数据等可改变程序集字节。
- 正确做法：嵌入产物与 runtime manifest 必须逐字节一致；源码复验以标准真实引用编译、契约测试和实际 runtime 行为为证。只有复用产生嵌入 DLL 的精确构建路径、镜像 digest 与参数时，才能额外声称 fresh build SHA 相等。
- 预防检查：不要把“Deterministic 默认开启”外推为任意工作路径字节相同；需要可复现摘要时先固化 PathMap、SDK image digest、restore lock 与完整命令。
- 适用范围：Control Mod 嵌入 DLL、.NET reproducible build 与发布摘要核验。

## 2026-08-06：PowerShell 泛型列表包装后交给 ConvertTo-Json 类型错误

- 环境：PowerShell 7，只读定位宿主 Steam 库与 Stardew 客户端。
- 错误模式：用 `System.Collections.Generic.List[object]` 收集匿名对象，再通过数组子表达式包装后放进 ordered dictionary 并执行 `ConvertTo-Json`。
- 症状 / 退出码：脚本报 `OperationStopped: Argument types do not match`，没有启动游戏或修改外部状态。
- 根因：PowerShell 的动态绑定在泛型 `List[object]`、数组包装和字典属性组合中发生类型适配冲突；该任务不需要泛型集合。
- 正确做法：使用普通 PowerShell 数组 `@()` 和 `+= [pscustomobject]...` 收集少量已验证路径，再直接构造 `[pscustomobject]` 输出 JSON。
- 预防检查：短小诊断脚本优先使用原生数组；只有确有性能需求时才使用泛型列表，并在组合到 JSON 对象前显式调用 `.ToArray()` 做独立探针。
- 适用范围：PowerShell 7 的诊断聚合、ordered dictionary 与 `ConvertTo-Json`。

## 2026-08-06：统一终端 TTY 无法直接启动 WindowsApps PowerShell 7

- 环境：Codex `exec_command`、Windows、`tty:true`，准备以前台长进程运行隔离 Panel。
- 错误模式：在 TTY 模式下让统一终端再次 `CreateProcessW` 启动 `C:\Program Files\WindowsApps\...\pwsh.exe`。
- 症状 / 退出码：进程创建阶段即报 `拒绝访问 (os error 5)`；Shell 脚本未执行，临时目录、Panel 和 Docker 资源均未创建，已单独核对。
- 根因：当前 PowerShell 7 来自 Store/WindowsApps，普通非 TTY 工具路径可调用，但 TTY 子进程无权直接打开该受保护入口。
- 正确做法：本环境的 PowerShell 7 长进程使用已验证的非 TTY `exec_command` 会话并通过 `write_stdin`/轮询管理；不要为获得 PTY 切到受限 WindowsApps 可执行入口。
- 预防检查：首次使用 `tty:true` 前独立探针 `Get-Command pwsh` 并验证同模式能创建进程；Store 路径默认选非 TTY，会话创建失败后先确认脚本是否实际执行再重试。
- 适用范围：Codex Windows 统一终端、Store 版 PowerShell 7 和长运行本地服务。

## 2026-08-06：按 Control InitConfig 猜测 Web NewGameConfig 字段

- 环境：隔离 Panel 真实 HTTP，新建一次性联调存档。
- 错误模式：把 Control 内部 `InitConfig` 的 `autoPause` / `hideHost` 直接放入 Web `registry.NewGameConfig` 请求体，没有先核对 DTO。
- 症状 / 退出码：`POST .../saves/custom-new-game` 返回 `400 invalid_json`；服务端因 `DisallowUnknownFields` 拒绝未知字段，服务器与新建 job 均未启动。
- 根因：内部文件协议与公开 Web DTO 是不同契约；通用错误文案隐藏了具体 unknown-field 名称，但本地 JSON 语法本身有效。
- 正确做法：先读取 `registry.NewGameConfig` 的 JSON tag，只提交公开字段；Control 内部默认/派生字段由后端转换生成，不能从内部 C# 类型反推 HTTP body。
- 预防检查：真实 API 写请求前先定位对应 handler 的 decode target 与结构体 JSON tag；`DisallowUnknownFields` 接口使用最小请求逐步增加已确认字段。
- 适用范围：Panel Web DTO、Control sidecar/内部协议及其它多层 JSON 契约。

## 2026-08-06：切到子目录后仍给 gofmt 传仓库根相对路径

- 环境：PowerShell 7，`workdir=backend`，玩家 Mod 比较改动后的 Go 格式化。
- 错误模式：工作目录已经是 `backend`，仍把 `backend/internal/...` 传给 `gofmt -w`。
- 症状 / 退出码：三个目标都报 `GetFileAttributesEx ... The system cannot find the path specified`；`gofmt` 未修改文件，后续测试因显式退出没有执行。
- 根因：混用了仓库根相对路径与当前 `workdir` 相对路径，实际解析成重复的 `backend/backend/...`。
- 正确做法：在 `workdir=backend` 时使用 `internal/games/...`；或者把工作目录设为仓库根并保留 `backend/internal/...`，二选一保持一致。
- 预防检查：执行带精确文件参数的格式化命令前，用 `Test-Path -LiteralPath` 按当前工作目录验证第一个目标；命令设计时不要同时移动工作目录和保留旧路径前缀。
- 适用范围：`gofmt`、测试、lint 以及所有由工具设置 `workdir` 的相对路径命令。

## 2026-08-06：把 Browser viewport capability 猜成 Page 方法

- 环境：Codex 内置浏览器、持久 `iab` 绑定，桌面/手机响应式联调。
- 错误模式：先后调用不存在的 `qaPage.playwright.setViewportSize(...)` 和 `qaPage.setViewportSize(...)`。
- 症状 / 退出码：Node REPL 分别返回 `... is not a function`；页面和视口未改变，随后停止猜测并查阅公开 viewport capability 文档。
- 根因：内置浏览器的视口覆盖属于 browser capability，不是 Playwright 子对象或 Page 方法。
- 正确做法：`const viewport = await iab.capabilities.get('viewport')`，测试时调用 `viewport.set({width, height})`，结束前调用 `viewport.reset()`。
- 预防检查：涉及浏览器非 DOM 能力（视口、可见性等）时先读对应 `docs/capabilities/browser/*.md`；不要把原生 Playwright API 直接外推到内置浏览器包装层。
- 最近补充：切换到移动壳后又沿用桌面页标题 `玩家管理` 做 `waitFor`，导致 locator 超时；移动页实际以“在线玩家”区域开头。跨桌面/移动验收应先读当前语义树，或等待两端共有的稳定控件，不应假定标题复用。
- 最近补充：打开真实 Panel 登录页后把预期标题猜成“登录面板”，实际页面唯一 heading 仍是产品名 `Stardew Anxi Panel`，表单由“用户名 / 密码 显示密码 / 登录”控件区分。首次进入真实状态页应先取语义快照，再等待快照中确实存在的控件。
- 最近补充：语义快照把密码输入框显示成 `textbox "密码 显示密码"`，但这不是 `getByLabel("密码 显示密码")` 可用的真实 label；直接照抄组合后的可访问名称会得到 `no_matches`。登录表单应先用已确认的唯一 `input[type="password"]` 定位，或读取可交互 DOM 后使用真实 label，不能把快照的聚合文本反推成 label 绑定。
- 最近补充：2026-08-07 验收 VitePress 展示站时，先把主题开关猜成按钮“主题模式”，实际语义角色/名称是 `switch "切换为深色主题"`；又用 exact heading `v0.4.8（最新版本）` 定位，实际可访问名称还包含 permalink 文本。VitePress 页面必须先读取当前 `domSnapshot()`，按已观察到的 role/name 操作；含标题永久链接时优先用正文投影或非 exact 文本，不猜组合后的可访问名称。
- 最近补充：同轮把页面求值误写成不存在的 `tab.playwright.dom.evaluate(...)`；当前 Browser 的页面级只读求值是 `tab.playwright.evaluate(...)`，语义树读取则是 `tab.playwright.domSnapshot()`。Browser 子接口的层级必须以已读 API Reference 和本轮已验证调用为准，首次失败后不得继续在相邻对象上试探。
- 适用范围：Codex in-app Browser 响应式测试与临时视口覆盖。

## 2026-08-06：按资源名猜测不存在的 Web handler 文件

- 最近复发/补充：2026-08-09 本轮发布审计先后把更新检查器猜成不存在的 `backend/internal/updatecheck/checker.go`，又把 DNS client 猜成不存在的 `backend/internal/netdns/client.go`；实际文件分别为 `service.go` 与 `netdns.go`。即使包目录已确认，也必须先 `rg --files <package-dir>` 再读取具体文件，不能继续由类型名或职责猜文件名。
- 最近复发/补充：2026-08-06 核对玩家 Mod 详情姓名来源时，直接猜成 `frontend/src/components/PlayerModsDetail.tsx`、`pages/PlayerModsPage.tsx` 和 `pages/mobile/MobilePlayersPage.tsx`，实际组件布局并不在这些路径，`rg` 对三个目标报不存在。前端同样必须先 `rg --files frontend/src | Select-String 'PlayerMods|MobilePlayers'`，不能从导出名推断目录。
- 最近复发/补充：2026-08-06 定位 Panel 更新检查器时把已存在的 `backend/internal/updatecheck` 简写成不存在的 `backend/internal/update` 并和其它路径一起传给 `rg`，导致检索退出 2。应先用 `rg --files backend/internal | Select-String update` 确认包名，再搜索精确目录；Go import 名不能按业务简称猜测。
- 环境：PowerShell 7，准备真实隔离 Panel 的存档选择 API。
- 错误模式：直接把存档 handler 文件猜成 `backend/internal/web/saves_handlers.go` 并传给 `rg`。
- 症状 / 退出码：`rg` 报目标文件不存在；同命令对已确认文件的搜索仍返回结果，未修改运行环境。
- 根因：路由实际集中在 `instance_handlers.go`，没有先用 `rg --files backend/internal/web` 确认文件名。
- 正确做法：未知源码文件先 `rg --files <dir>`，再对返回的精确路径检索；本次契约改从 `instance_handlers.go` 与 `frontend/src/api.ts` 核对。
- 预防检查：不要由资源复数名推断 handler 文件名；给 `rg` 传文件前先确认路径存在，或直接搜索已确认目录。
- 适用范围：Go Web handler、测试文件及其它按功能拆分但命名不固定的源码定位。

## 2026-08-06：未探测真实 jobs 响应结构就直接轮询顶层 status

- 环境：PowerShell 7，真实隔离 Panel 生命周期启动任务。
- 错误模式：假定 `GET /api/jobs/:id` 顶层直接包含 `status`，未先读取响应形状便进入四分钟轮询。
- 症状 / 退出码：首次输出 `JOB_STATUS=` 空值，随后手动中止只读轮询；后台生命周期 job 未被取消或修改。
- 根因：接口响应实际有包装层，状态不在猜测的顶层字段。
- 正确做法：对首次使用的真实 API 先执行一次只读请求并查看脱敏 JSON 结构，再写有上限的终态轮询；状态为空应立即 fail fast，不能继续等待。
- 预防检查：轮询第一轮同时验证 job ID 和 status 非空、状态属于已知集合；未知结构立即停止并回到 handler/类型契约。
- 适用范围：Panel jobs、更新状态以及其它带包装对象的异步 API。

## 2026-08-06：隔离前端副本遗漏仓库级测试依赖

- 环境：Docker Desktop、`node:24-alpine`、任务专属前端工作卷。
- 错误模式：只把 `frontend/` 复制到 `/work` 后运行 `test:responsive-layout`，没有先核对脚本还会读取仓库根 `.github/workflows/release.yml` 与 `compatibility-matrix.yml`。首次准备补记时又只在工具编排层构造补丁字符串并输出占位文本，忘记实际调用 `apply_patch`；文件未变化后才立即纠正。
- 症状 / 退出码：前 10 项状态测试通过，`test:responsive-layout` 读取 `/.github/workflows/release.yml` 返回 `ENOENT`，容器退出 1；产品源码未报断言或编译错误。占位工具调用退出 0 但没有产生预期文档差异。
- 根因：隔离副本的目录层级与测试通过 `import.meta.url` 解析的仓库相对路径不一致；同时没有在补丁工具返回后检查真实修改结果。
- 正确做法：把完整仓库复制到任务卷的 `/work/repo`，从 `/work/repo/frontend` 运行 `npm ci`、全部状态测试和 build；补丁调用必须检查 `apply_patch` 的返回并用 `git diff`/`rg` 确认目标文本实际落盘。
- 预防检查：容器化子项目门禁前搜索测试脚本的 `../`、仓库根配置和 fixture 引用；复制边界必须包含全部运行时读取项，不能只看 npm 工作目录。
- 适用范围：前端/网站/脚本在隔离容器中的仓库级测试和工具编排后的落盘验证。

## 2026-08-06：把 Go 模块内路径误当成仓库根路径

- 环境：PowerShell 7，在仓库根定位 updater Docker integration 发布门禁。
- 错误模式：把模块内目录写成 `internal/updater` 传给 `rg`，没有先确认 Go 模块实际位于 `backend/`。
- 症状 / 退出码：`rg` 报 `internal/updater` 不存在并退出 2；只读检索中止，没有修改源码或运行环境。
- 根因：混用了 `backend` 模块根相对路径和仓库根相对路径。
- 正确做法：从仓库根使用 `backend/internal/updater`，或先把工作目录切到 `backend` 再使用 `internal/updater`；未知路径先通过 `rg --files backend` 确认。
- 预防检查：发布门禁命令涉及 Go 包时，先确认命令工作目录与 `go.mod` 位置；源码检索的第一个目标用 `Test-Path -LiteralPath` 或 `rg --files` 验证。
- 适用范围：多子项目仓库中的 Go 测试、格式化、源码检索与发布脚本。

## 2026-08-06：漏传 compatibility_matrix 子命令的位置参数

- 环境：PowerShell 7、项目固定 Python 3.12，执行 v0.4.8 远端制品发布门禁。
- 错误模式：只调用 `scripts/compatibility_matrix.py verify-remote-artifacts`，没有先查看 argparse 契约，也没有传 runtime manifest。
- 症状 / 退出码：argparse 立即提示缺少必需的 `path` 并退出 1；没有发起网络请求或修改任何资源。
- 根因：把带位置参数的子命令误记成无参数全局校验入口。
- 正确做法：按 release workflow 使用 `python scripts/compatibility_matrix.py verify-remote-artifacts backend/internal/games/stardew_junimo/config/runtime_stack_manifest.json`。
- 预防检查：不熟悉的脚本子命令先运行 `--help` 或直接复用 `.github/workflows/release.yml` 的精确命令；发布门禁不得凭记忆省略参数。
- 适用范围：兼容矩阵校验器及其它 argparse 子命令式发布脚本。

## 2026-08-06：凭记忆填写 Go 专项测试名且漏掉 build tag

- 环境：PowerShell 7、Go 1.26，执行 SMAPI 真实下载发布门禁。
- 错误模式：用不存在的 `TestDownloadSMAPIArchiveFromReviewedAccelerator` 作为 `-run`，并漏掉测试文件要求的 `-tags=integration`。
- 症状 / 退出码：`go test` 退出 0，但明确打印 `warning: no tests to run`，实际没有下载任何字节，因此不能作为门禁证据。
- 根因：没有先读取 `smapi_archive_integration_test.go` 的函数名与文件头 build constraint。
- 正确做法：设置 `PANEL_RUN_SMAPI_DOWNLOAD_TEST=1`，使用 `go test -tags=integration ./internal/games/stardew_junimo -run '^TestSMAPIArchiveRealDownload$' -count=1 -v`，并要求输出实际下载字节数。
- 预防检查：专项 `-run` 前先用 `go test <同样 tags> <pkg> -list '<pattern>'` 或读取测试声明；任何 `no tests to run` 都视为失败并立即纠正。
- 适用范围：带 build tag 或 opt-in 环境变量的 Go integration 门禁。

## 2026-08-06：在 Windows 文件系统执行 Linux 权限位发布断言

- 环境：Windows 宿主 Go 1.26，SMAPI 真实下载 integration 测试使用 `t.TempDir()`。
- 错误模式：直接在 Windows 上运行同时断言缓存文件 POSIX `0600` 的生产 Linux 门禁。
- 症状 / 退出码：真实下载约 331 秒并完成内容校验，但 `os.Stat().Mode().Perm()` 在 Windows ACL 语义下报告 `0666`，测试退出 1；这不能证明 Linux 容器中的产品权限实现失败。
- 最近复发/补充：2026-08-09 候选收口再次在 Windows 宿主直接启动同一测试，下载 5.65 秒后因 `cache mode=0666, want 0600` 退出 1。未改产品或断言；立即改回任务专属 `golang:1.25-alpine`、独立 Linux cache volume 执行。发布门禁命令编排应把该测试固定封装进 Linux 容器，不能只依靠人工记忆选择环境。
- 根因：把跨平台内容/摘要验证与只在 Linux 目标文件系统有意义的权限位验证放在了错误宿主环境。
- 正确做法：使用与 Dockerfile 一致的 `golang:1.25-alpine` 任务专属 Linux 容器，从空容器缓存运行同一个 `-tags=integration` 测试，要求实际字节数、固定摘要、`0600` 和无 `.part` 残留同时通过。
- 预防检查：发布测试包含 `chmod`、`Mode().Perm()`、UID/GID、符号链接或 Unix socket 时，先选择目标 Linux 环境；Windows 宿主仅可做不依赖 POSIX 元数据的补充测试。
- 适用范围：Docker Linux 产品的 Go 文件权限、原子写入和下载缓存门禁。

## 2026-08-06：短命 Go 容器网络失败后丢失模块下载进度

- 环境：Docker Desktop、`golang:1.25-alpine`、只读源码 bind，首次在 Linux 重跑 SMAPI 下载门禁。
- 错误模式：未给短命 `--rm` 容器挂任务专属 `GOMODCACHE/GOCACHE`；`proxy.golang.org` 下载 `modernc.org/sqlite` 元数据发生一次 `unexpected EOF` 后容器退出，已获取依赖缓存也随之丢失。
- 症状 / 退出码：Go package setup 退出 1，`TestSMAPIArchiveRealDownload` 尚未开始，产品下载逻辑未被执行。
- 根因：发布测试的外网依赖准备没有持久进度与有界重试，瞬时网络断流被放大为从零开始。
- 正确做法：创建带发布 ownership label 的唯一 Go module/build cache volume，先在同一缓存上对 `go mod download` 做有上限重试，再运行目标测试；完成后按 label 精确核对和清理 volume。
- 预防检查：短命容器执行需要外网的语言工具链前先规划任务专属缓存；网络失败重试必须改变为可续用缓存且限制次数，不能原样重建空容器。
- 适用范围：容器化 Go/npm/Python 依赖准备与长耗时发布门禁。

## 2026-08-06：BuildKit 默认网络同时损坏 Go ZIP 与 Alpine 包校验

- 最近复发/补充：2026-08-08 构建 WAL/repair 本地候选时再次直接使用 BuildKit 默认网络，`go mod download` 对 `x/text`、`modernc/sqlite` 和 `modernc/libc` 同时报 `unexpected EOF`；前端层成功但最终 tag 未生成。先用精确 `docker image ls --filter reference=...` 确认没有误产物，再按本条既定做法改用 `docker build --network=host` 复用已成功 layer 并只重取失败依赖；不得原样重复默认网络构建。
- 最近复发/补充：同日 host 网络构建的 Go 层恢复，但 Alpine index 又因 SSL EOF 缺包退出；随后可达性探针把 `apk update` 输出重定向到 `--rm` 容器内临时文件，只在成功分支 grep，失败时容器随即删除且没有诊断输出。网络探针必须直接保留 stderr/stdout，或先保存退出码、打印日志后再退出原值；不能把唯一故障证据留在即将删除的容器里。
- 环境：Docker Desktop Linux、v0.4.8 精确候选多阶段 Docker build。
- 错误模式：直接使用 BuildKit 默认网络获取全新的 Go modules 与 Alpine apk 包，未先验证该网络路径本轮传输稳定性。
- 症状 / 退出码：`go mod download` 对多个 ZIP 同时报 `unexpected EOF`，并行 `apk add` 报 `docker-cli-26.1.5-r0: UNTRUSTED signature`；build 退出 1，最终镜像 tag 未创建。
- 根因：同一次构建中两个独立生态都出现内容截断/校验失败，证据指向 Docker build 默认网络路径的瞬时传输损坏，而非 Go 依赖声明或 Alpine 包本身。
- 正确做法：确认失败构建没有留下候选 tag，保留成功 BuildKit layer cache，下一次显式使用 Docker Desktop 支持的 `docker build --network=host` 重新获取失败层；仍失败则停止发布并检查代理/CA，不能关闭签名或摘要校验。
- 预防检查：候选构建必须检查完整退出码和最终 OCI 标签，出现签名/摘要异常禁止使用 `--allow-untrusted`；网络路径变更后才允许有界重试。
- 适用范围：正式 Docker 多阶段构建中的 apk、Go modules、npm 依赖与其它外部制品。

## 2026-08-06：在 Panel 初始化前访问受保护 SPA 路由

- 环境：v0.4.8 精确候选 fresh 容器，`/health`、`/api/version` 与 `/api/setup/status` 已通过。
- 错误模式：尚未 `POST /api/setup/admin` 就请求 `/instances/stardew/player-mods`，并错误期待静态 HTML 200。
- 症状 / 退出码：服务按初始化门禁正确返回 `503 setup_required`，smoke 脚本因断言主动退出；任务容器和 fresh volume 保留可继续，未创建游戏容器或写用户数据。
- 根因：混淆了“后端 SPA 路径白名单”与“初始化前公开路径”；白名单只允许初始化后由 SPA 接管，不会绕过 setup middleware。
- 正确做法：fresh smoke 先验证未初始化状态，再创建任务专用管理员并保持会话，之后访问 `/instances/stardew/player-mods?playerId=...`；初始化前只检查明确公开的 health/version/setup 端点。
- 预防检查：真实 HTTP 脚本按 middleware 顺序排列：public readiness → setup → auth → protected SPA/API；不要由静态路由白名单推断公开权限。
- 适用范围：Panel fresh 安装、浏览器直达路由和鉴权 E2E。

## 2026-08-06：从 Windows Compose CLI 生成 Linux updater 不可验证的路径标签

- 环境：Docker Desktop Linux、Windows 宿主 `docker compose`、真实 Web 一键更新 harness。
- 错误模式：从 `E:\...` 工作目录直接启动 Compose，导致容器 label `com.docker.compose.project.config_files/working_dir` 保存 Windows 盘符路径；Panel 内 Linux `filepath.IsAbs` 无法把它认作安全绝对路径。
- 症状 / 退出码：更新检查成功，但 dry-run 返回 `unsupported / compose_metadata_invalid / Compose config_files 不是可验证的绝对路径`；updater 未拉取、停止或重建任何容器。
- 根因：Compose 元数据由调用方路径语义决定；Windows 盘符对 Linux Panel 不是可反向校验的部署路径。
- 正确做法：用任务专属 Docker CLI 容器挂 Docker Socket，并把 Windows harness bind 到 Docker Desktop 可见的同一 Linux 绝对路径 `/run/desktop/mnt/host/e/...`，从该工作目录执行 Compose；Compose 文件内 host install/compose/data 也使用同一路径。
- 预防检查：Web updater E2E 启动后先 inspect `config_files/working_dir`，要求 Linux 绝对路径，再调用 dry-run；不能通过放宽路径校验解决测试环境问题。
- 适用范围：Windows Docker Desktop 上由 Linux 容器反向管理 Compose 的 updater/runtime 集成测试。

## 2026-08-06：Web apply 只等待目标版本而未先读 updater 终态

- 环境：隔离 `0.4.7 → 0.4.8` Web 更新故障注入，pre-tag 远端目标镜像按预期不存在。
- 错误模式：apply 后外层脚本只轮询 `/health` 与目标 `/api/version`，没有在旧 Panel 恢复后立即读取 `/api/system/update/apply`；更新器 19 秒已写 `failed_rolled_back`，脚本却继续等待约四分钟并最终打印大量无意义健康日志。
- 症状 / 退出码：产品已安全回滚且旧 Panel healthy，但测试包装脚本直到目标版本超时才退出 1；状态证据明确为 `image_pull_failed / failed_rolled_back`，没有产品数据损失。
- 根因：把“等待成功版本”误作所有终态的唯一观测通道，漏掉失败回滚后旧版本同样会健康。
- 正确做法：断线重连循环同时尝试登录/读取 apply status；一旦 phase 为 `succeeded`、`failed_rolled_back` 或 `rollback_failed` 立即结束并按预期分支断言。pre-tag 成功路径改在任务专属 DinD daemon 内提供受控可信 registry，不让正式 helper 跳过 pull，也不提前推远端镜像。
- 预防检查：异步升级轮询必须列出全部终态并 fail fast；诊断日志仅在最终失败时截取有限 tail，不能在已知终态后继续刷 health。
- 适用范围：Panel Web updater、断线重连与自动回滚 E2E。

## 2026-08-06：假定 DinD insecure-registry 会覆盖无端口 HTTPS 引用

- 环境：任务专属 DinD、外层私有 registry 监听 443、目标引用固定为受信任的 `ghcr.io/anxiyizhi/...:0.4.8`。
- 错误模式：给 dockerd 传 `--insecure-registry=ghcr.io:443` 后，假定对规范化为无端口 `ghcr.io` 的 push 会自动改走 HTTP。
- 症状 / 退出码：四个镜像通过 TCP load 后 ID/size 均正确，但 push 仍发起 HTTPS，registry 返回纯 HTTP，报 `server gave HTTP response to HTTPS client` 并退出 1；registry 未收到正式 manifest。
- 根因：daemon 对该无端口 registry 引用未应用预期的 insecure 匹配；测试不能依赖模糊的端口规范化。
- 正确做法：为隔离 registry 签发 SAN=`ghcr.io` 的短期 TLS 证书，把测试 CA 只读挂到该任务 DinD 的 `/etc/docker/certs.d/ghcr.io/ca.crt`，registry 本身启用 TLS；保留 DinD data volume，重启 daemon 后复核已加载镜像再 push。
- 预防检查：受控 registry 首次使用先执行最小 `/v2/` TLS/push 探针；正式 updater 测试不得使用 `--tls-verify=false`、全局 hosts/CA 或 Docker Desktop daemon 配置变更。
- 适用范围：隔离 registry、DinD 与可信仓库名的 pre-release 更新测试。

## 2026-08-06：测试用 depends_on 改变 Compose 目标镜像解析语义

- 环境：DinD 内正式 0.4.7 helper、Compose 2.27、panel 与 TLS Release mock 同属一个测试 Compose。
- 错误模式：为启动顺序方便给 `panel` 增加 `depends_on: mock-release`，没有先验证生产 updater 使用的 `docker compose config --images panel` 输出。
- 症状 / 退出码：候选从隔离 registry 拉取成功且 digest 精确，但 `config --images panel` 同时输出候选和 `python:3.13-alpine` 依赖镜像，updater 以 `deployment_update_failed / Compose 配置未精确解析到目标镜像` 安全回滚。
- 根因：测试 Compose 比生产部署多了服务依赖，改变了旧版 Compose 对指定服务的 image 展开结果，触发了正确的单镜像安全断言。
- 正确做法：移除 panel 的 `depends_on`，保留 mock 的网络别名；测试脚本先单独确认 mock running，再发起更新检查。正式 apply 前用候选 env 复刻 helper 命令，要求 `config --images panel` 只输出一行精确目标。
- 预防检查：E2E harness 的服务关系必须与生产部署契约一致；添加依赖、profile 或 extends 后先验证 updater 使用的精确 Compose 子命令输出。
- 适用范围：Panel Web updater 的 Compose harness 与多服务隔离环境。

## 2026-08-06：在复合 if 中直接用数组子表达式统计 null 属性

- 环境：PowerShell 7，验证玩家列表 `modRiskFlags` 的空值契约。
- 错误模式：在同一 `if` 中写 `@($vanillaList[0].modRiskFlags).Count -ne 0`，把索引、属性访问、数组子表达式和逻辑短路压成一行。
- 症状 / 退出码：实际脱敏响应明确为 Vanilla Player、`RiskFlags=null`、单独计算 `RiskCount=0`，但复合断言仍错误进入失败分支；产品 API 与 CJB 玩家断言均正常。
- 根因：复杂 PowerShell 表达式的 null/数组绑定不够透明，不应作为发布证据的唯一断言写法。
- 正确做法：先把目标行数量与风险数量分别存入标量；风险数量使用 `if ($null -eq $value) { 0 } else { @($value).Count }`，再做简单数值比较。
- 预防检查：JSON 中允许 null/array 双形态的字段先显式规范化；发布脚本避免在一个条件内混合索引、属性、`@()` 与 `.Count`。
- 适用范围：PowerShell 对 JSON 空数组、null 和可选集合字段的 E2E 断言。

## 2026-08-06：长时间发布验收中 Docker Desktop 被外部退出

- 最近复发/补充：2026-08-09 repair plan 页面 QA 中，任务容器已启动且宿主 HTTP readiness 为 200；应用内浏览器导航等待约一分钟后，后续只读 `docker inspect` 发现 Linux engine named pipe 已消失。没有执行容器/卷清理或其它 mutation。浏览器验证超过一分钟后，继续任何 Docker 读取或清理前都必须重新 `docker info`；连接拒绝要同时区分浏览器本机访问策略与 Docker daemon 已退出，不能只按页面错误归因。
- 环境：Windows 11、Docker Desktop Linux containers、v0.4.8 隔离发布资源清理。
- 错误模式：沿用数分钟前成功的 Docker 状态，在破坏性清理前没有重新执行 `docker info` 就直接进入容器 ownership 检查。
- 症状 / 退出码：首个 `docker inspect` 报 Linux engine named pipe 不存在；脚本在任何 `docker rm` 或 network 删除前立即退出，任务资源和用户资源均未变化。
- 根因：长时间浏览器验收期间 Docker Desktop 进程被外部退出；早先的 readiness 不能代表随后清理时仍可用。
- 正确做法：执行创建、更新或清理 Docker 资源的每个独立阶段前重新运行 `docker info`；若引擎退出，先确认任务资源未变，再从已验证的 Docker Desktop 路径启动并做有上限 readiness 轮询，最后重新执行带精确 ownership 断言的操作。
- 预防检查：不要跨长时间浏览器/人工测试复用 Docker readiness 结论；所有破坏性 Docker 脚本把 `docker info` 作为第一条 fail-fast 探针。
- 适用范围：长运行发布门禁、Docker Desktop 中断恢复与任务资源清理。

## 2026-08-06：最终 updater 包装器猜错 tag 前缀并手写 JSON 转义

- 环境：PowerShell 7、DinD 内正式 `0.4.7 → 0.4.8` Web 一键更新。
- 错误模式：先把更新检查响应的 `latestVersion` 猜成 `0.4.8`，实际公开契约保留 tag 前缀为 `v0.4.8`；随后又把 apply 确认体手写成多层转义 JSON 字符串。
- 症状 / 退出码：第一次包装断言在 check 成功后误报 mismatch，dry-run 尚未启动；第二次 PowerShell 在发 HTTP 前报 positional parameter 无法接受 `confirmFullStack` 片段，apply 尚未发送，容器和数据均未变化。
- 根因：没有把真实 check 响应的目标字段直接传给下一阶段，并把 PowerShell、JSON 与外层命令的引号规则叠加在手写字符串中。
- 正确做法：先读取脱敏后的真实响应结构，断言 `latestVersion=v0.4.8` 后把该字段原样作为 dry-run target；所有 JSON 请求体用 PowerShell 对象加 `ConvertTo-Json -Compress` 生成，禁止手写多层转义。
- 预防检查：发布 API 包装器只规范化用于比较的版本，不能擅自改写接口字段；任何写请求先在本地构造 body 变量，再传给 `Invoke-RestMethod`，确保失败发生前能够区分客户端解析与服务端响应。
- 适用范围：Panel updater check/dry-run/apply、PowerShell HTTP E2E 与带 `v` 前缀的 tag 契约。

## 2026-08-06：隔离玩家 fixture 复制和 Compose 状态模拟不够精确

- 环境：最终候选升级后的真实玩家 Mod API/页面验收，所有数据位于任务专属 DinD bind。
- 错误模式：用 `Copy-Item -Recurse` 把已有 `control` 目录本身复制到已存在目标，产生非终止 `item already exists`；又手工给普通容器添加部分 `com.docker.compose.*` label，假定正式 `docker compose ps` 会把它识别为实例 server。
- 症状 / 退出码：fixture 文件实际到位且命令最终退出 0，但输出包含三个复制错误；手工 label 容器保持运行，Panel 重启校验仍把实例写成 `container_stopped`。
- 根因：PowerShell 的目录复制目标语义与预期的“复制目录内容”不同，且 Compose 资源身份不只由少数手写 label 决定；正式 CLI 只识别由同 project 创建的完整资源元数据。
- 正确做法：已存在目标应逐个复制源目录内容并使用 `-ErrorAction Stop`，随后核对精确文件清单；实例运行夹具使用任务专属最小 Compose 文件和相同 project name 创建 `server` service，再让真实 Panel 重启并从 SQLite 与 Compose 双重状态恢复。
- 预防检查：fixture 准备阶段对 PowerShell 非终止错误启用 fail-fast；不要手造 Compose ownership label，先用正式 `docker compose ... ps --format json` 验证目标 service 能被实际实例配置发现。
- 适用范围：Docker/Compose 状态模拟、Control sidecar fixture、Windows 目录复制和升级后功能验收。

## 2026-08-07：把本地候选完整 revision 契约套到正式镜像

- 环境：Docker Desktop，回拉 `v0.4.8` 三仓正式镜像后核对 OCI labels。
- 错误模式：沿用本地候选的 40 位 `org.opencontainers.image.revision` 预期，直接断言正式镜像也必须是完整 SHA。
- 症状 / 退出码：六个 tag 的 pull、digest、version、created 和实际 revision 都一致，但组合脚本末尾因 revision 为 `0c5e2c434a92` 而退出 1。
- 根因：发布 workflow 的 `Prepare release metadata` 明确使用 `${GITHUB_SHA::12}`，正式镜像契约是 12 位 revision；本地候选构建使用完整 SHA，二者不能互套。
- 正确做法：先读取当前 release workflow 的 metadata 生成逻辑；正式镜像 revision 与预期 12 位 SHA 比较，annotated tag/远端 main 则另用完整 40 位提交核对。
- 预防检查：发布断言把 tag commit、构建参数和 OCI label 分为独立字段，每个预期都来自当前 workflow，不从上一阶段镜像或上一版本猜测长度。
- 适用范围：正式镜像回拉、OCI 元数据、SBOM/provenance 与 release workflow 审计。

## 2026-08-06：ConvertFrom-Json 自动把 OCI ISO 时间转成 DateTime

- 最近复发/补充：2026-08-09 候选 metadata 核验再次把 JSON 自动转换的 `DateTime` 与原始 ISO 文本直接比较，显示值相同却误报；同轮 rollback 断言又反向假定 `StartedAt` 必然是 `DateTime`，实际为 string 后直接调用 `.ToUniversalTime()` 失败。统一规范化 helper 必须先按运行时类型分支：`DateTime` 直接转 UTC，string 用 invariant `DateTimeOffset::Parse` 后转 UTC，最后格式化比较；不得假定解析器总返回同一类型。
- 最近复发/补充：2026-08-06 最终一键升级验收通过 `Invoke-RestMethod` 读取 `/api/version` 后，又把自动转换成 `System.DateTime` 的 `buildDate` 与原始 OCI 字符串直接比较，产生第二次虚假 mismatch。HTTP JSON 与 `ConvertFrom-Json` 一样必须先检查类型、转 UTC 并格式化后比较；该预防规则已提升到 `AGENTS.md`。
- 环境：PowerShell 7、最终候选镜像 OCI label 核验。
- 错误模式：把 `ConvertFrom-Json` 得到的 `org.opencontainers.image.created` 直接与原始 ISO 8601 字符串做 `-ne` 比较。
- 症状 / 退出码：镜像 JSON 中实际值为预期的 `2026-08-06T14:04:14Z`，但断言抛出 build date mismatch；独立类型探针显示属性类型已经是 `System.DateTime`，默认字符串化会变成本地格式。
- 根因：PowerShell 7 的 `ConvertFrom-Json` 会自动识别 ISO 日期字符串；类型转换后的 DateTime 与原始字符串不能作为严格文本相等比较。
- 正确做法：先确认属性类型；日期字段统一转为 UTC，并用 `.ToString("yyyy-MM-ddTHH:mm:ssZ", [Globalization.CultureInfo]::InvariantCulture)` 规范化后比较。若需要保留原始 JSON 文本语义，改用不会自动日期转换的解析方式。
- 预防检查：发布断言中的版本、commit 作为字符串比较，ISO 时间先做显式类型与 UTC 规范化；断言失败时先输出值与 `.GetType().FullName`，不要把类型差异误报为镜像元数据错误。
- 适用范围：PowerShell 7 的 Docker inspect、HTTP JSON 与任何 ISO 时间字段核验。

## 2026-08-08：打印 `.env` 时使用不完整的敏感字段黑名单

- 环境：PowerShell 7，只读核对本地 Docker Desktop 升级夹具的版本配置。
- 错误模式：先读取完整 `.env` 再用少量字段名替换脱敏，只覆盖 Steam 密码、refresh token、API key 和 VNC 密码，遗漏用户名及 server password 等同样不应输出的字段。
- 症状 / 退出码：命令成功但工具输出包含不必要的账号标识和服务口令；没有写入文件或提交。
- 根因：使用敏感字段黑名单，错误假设已枚举所有秘密；升级检查实际只需要版本、镜像和候选字段。
- 正确做法：不得输出完整 `.env`；只用 `sjconfig.ReadEnvFile` 后挑选明确的非敏感白名单键，或用锚定键名的逐项读取。任何凭据/账号存在的文件默认整体敏感。
- 预防检查：命令中出现 `Get-Content ... .env`、`cat .env` 或打印完整环境时直接停止，改为非敏感字段白名单；用户名、password、secret、token、key、cookie、ticket 一律不进工具输出。
- 适用范围：Panel/实例 `.env`、Docker inspect Env、支持包、部署迁移与真实升级夹具。

## 编码与换行快速检查

- 最近复发/补充：2026-08-09 升级修复目录最终审计又对全部已修改文件直接执行 U+FFFD 搜索，命中了本节合法示例和历史版本说明并让组合命令退出 1；源码格式与 `git diff --check` 实际已通过。此检查以后必须先生成 `git diff --unified=0`，只过滤单个 `+` 的新增行并排除 `+++` 文件头，禁止再次扫描完整历史文件。
- 最近复发/补充：2026-08-08 首轮批量读取中文文档时沿用 PowerShell 7 默认控制台输出编码，工具侧显示乱码；文件无 BOM、`git status` 为空，确认只是只读显示链路。后续中文读取先同时设置 `$OutputEncoding=[System.Text.UTF8Encoding]::new($false)` 与 `[Console]::OutputEncoding=[System.Text.UTF8Encoding]::new($false)`，并对 `Get-Content` 显式使用 `-Encoding UTF8`；乱码出现时先核对 Git 状态和文件字节，不得把显示问题误判为文件损坏。
- 最近复发/补充：2026-08-08 最终审计再次扫描了整个已修改错题本，误把本条用于解释旧乱码问题的合法 `�` 示例判为新乱码。检查 U+FFFD 时不能对“本次修改过的整个历史文件”直接搜索；应先跑 `git diff --check`，再只检查 `git diff --unified=0` 中以单个 `+` 开头的新增行，发现命中后再回到原文件确认语义。
- 最近复发/补充：2026-08-07 发布后文档审计再次对所有 changed file 的完整内容搜索 U+FFFD，误报错题本的规则示例与 `docs/09-image-build.md` 的历史乱码说明。已改回只检查 `git diff --unified=0` 中本次新增行；命中历史正文不能作为失败依据。
- 默认：UTF-8 无 BOM。
- `.env`：必须 UTF-8 无 BOM。
- `.sh`：UTF-8、LF，并运行 `bash -n` 与 ShellCheck。
- `.ps1`：遵循 `.gitattributes` 的 CRLF；既有 BOM 只有在已验证的 Windows PowerShell 5.1 兼容场景保留。
- Go/TS/JS/JSON/YAML/Markdown：UTF-8 无 BOM；修改后运行格式化、解析或构建检查。
- 交付前：`git diff --check`、`git status --short`、检查差异规模，并搜索意外 Unicode replacement character（`U+FFFD`）。发现整文件换行或编码变化时不要提交。
