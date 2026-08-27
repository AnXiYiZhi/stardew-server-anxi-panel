# AGENTS.md

## Git 单主线硬规则

- 本项目所有功能、修复、文档、测试和发布工作只能直接在本地 `main` 上进行，并同步到远端 `origin/main`。
- 禁止代理自行创建、切换或保留任何非 `main` Git 分支，也禁止为任务或发布自行创建额外 Git worktree。除非用户在当次请求中明确要求特定分支，否则不得使用功能分支、发布分支、临时分支或 PR 分支。
- 发布 tag 必须从已与 `origin/main` 完全同步、工作树干净且通过全部发布门禁的 `main` 提交创建；不得从其它分支创建 tag，也不得移动已有 tag。
- 发现历史非 `main` 分支或 worktree 时，先确认其有效提交已进入 `main`，再删除对应 worktree 和本地/远端分支；不得因清理分支丢失未合并提交或未提交文件。

## 工作开始前

每次工作开始前先阅读 `docs/01-project-overview.md` 和 `.agents/error-notebook.md`，再按任务范围阅读：

- 后端任务：`docs/02-backend.md` 和 `docs/backend-handoff/` 下最新的后端接手文档。
- 前端任务：`docs/03-frontend.md` 和 `docs/frontend-handoff/` 下最新的前端接手文档。
- 前后端联调任务：`docs/06-integration.md`。
- 路线、排期、已完成状态：`docs/08-future-roadmap.md`。
- Docker 镜像、部署、发布：`docs/09-image-build.md`。

修改或新建功能时，优先寻找 Junimo 容器已有能力进行通信，不要绕过 `stardew_junimo` driver 直接在 API 层堆 Stardew 逻辑。

## 项目接手文档规则

每完成或修改一个功能，都必须同步更新对应长期文档：

- 后端功能：更新 `docs/02-backend.md`、最新后端接手文档，并在 `docs/08-future-roadmap.md` 标记状态变化。
- 前端功能：更新 `docs/03-frontend.md`、最新前端接手文档，并在 `docs/08-future-roadmap.md` 标记状态变化。
- 跨端接口或联调：更新 `docs/06-integration.md`。
- 镜像、部署、发布流程：更新 `docs/09-image-build.md`。
- 后期暂缓事项：更新 `docs/07-later-optimizations.md`。

接手文档至少记录：改了什么、影响哪些接口/文件、如何验证、下一步注意事项。不要只更新 README；README 面向使用者，接手文档面向下一位维护者。

## 正式版本与 Tag 发布硬门禁

任何创建、移动或推送 `v*` tag，更新 `latest`，推送正式镜像，或创建 GitHub Release 的工作，都必须先完成以下门禁。目标是用一次不可变候选证明“新旧功能没有回归、上一正式版能从 Web 一键升级”，不再为未变化链路机械重复长门禁。

1. 发布前在 `docs/09-image-build.md` 写出本版变更清单、受影响链路和本版专项矩阵。专项矩阵只覆盖本版新增或修改功能的正常路径、关键边界、权限安全、幂等/恢复、数据完整性与资源清理；与本版无关的历史故障不重复手工注入，由既有自动化回归负责。
2. 正式候选只能从工作树干净、与 `origin/main` 完全同步的本地 `main` 构建，版本、完整 commit 和 UTC build date 必须固定。影响镜像或正式部署资产的提交推送到 `main` 后，由 `.github/workflows/release-candidate.yml` 自动取最新正式版并递增补丁版本；手动 dispatch 只用于明确覆盖 major/minor 版本或受控重试。Windows Docker Desktop 本机复现使用 `pwsh -NoLogo -NoProfile -File scripts/release-candidate.ps1`，Linux/CI 使用 `scripts/release-candidate.sh`。所有测试使用任务专属 Compose project、容器、网络、端口、bind 和 volume，不得使用生产数据或长期凭据。
3. 同一版本和 commit 只允许一个待发布候选身份。候选完成测试后以 `candidate-<version>-<sha12>` 推到 GHCR，并把精确 digest、版本、commit、build date、上一正式版和候选 workflow run 写入不可变候选证明 artifact。任何会改变镜像内容、运行契约或测试输入的提交都会使旧证明失效；纯 tag、Release 说明和发布后证据不得触发重建。
4. 候选代码门禁始终包含后端 test/vet/build、前端状态回归与 production build、脚本测试/ShellCheck、兼容清单、updater/Docker integration。网站只在 `website/**`、公开文档或 README 变化时构建；SMAPI 真实下载、远程制品和 Junimo 长 integration 只在运行栈清单或对应实现变化时执行。跳过必须由 `scripts/run-release-gates.sh` 基于上一 tag 到候选 SHA 的路径差异自动判定，不能凭口头判断。
5. 每个候选必须完成全新安装、`/health`、`/api/version`、未初始化状态和 Panel 重启冒烟；本版新增或修改功能至少有一条真实 Docker E2E，并在升级得到的新 Panel 上再次验收。未变化旧功能由全量自动回归覆盖，升级后再抽验初始化/用户、实例、存档/Mod/备份等受影响的核心状态。
6. 必须从“当前上一正式版”通过真实 Panel Web API 完成更新检查、dry-run、管理员确认、apply、预期断线重连和终态恢复，验证 SQLite、初始化状态、任务相关长期数据、非目标游戏容器/volume 及 Panel 重启后的状态。必须对同一候选引用注入一次 unhealthy 目标，确认 `failed_rolled_back/health_check_failed` 和旧版恢复。不能用直接调用 updater、手工改 Compose 或仅 `docker compose up` 替代这条验收。
7. 默认不再每版测试更老版本。只有本版改变数据库迁移、部署格式、运行栈、长期数据结构或跨版本兼容逻辑时，才增加“受影响的最老支持版本 → 候选”一条代表升级；版本边界必须来自迁移代码和长期文档。Control/SMAPI/Junimo/网站未变化时不重复对应真实长链。
8. Tag 必须是 annotated tag，指向候选证明中的完整 commit，且该 commit 必须仍精确等于 `origin/main`。候选 workflow 成功后，`.github/workflows/release-after-candidate.yml` 只有在该 commit 仍是最新 `origin/main` 时才自动创建 tag 并通过 `workflow_dispatch` 启动正式提升；被后续提交取代的候选只记录 superseded，不发布。`.github/workflows/release.yml` 只能提升候选证明里的精确 digest 到三仓正式版本和 `latest`，禁止重新 build；三仓六引用 digest 与 OCI 元数据必须一致。digest 一致时只从一个正式仓库回拉做 health/version 冒烟，不再启动三份相同镜像。
9. 候选 workflow、任一必跑门禁、真实 Web 升级、unhealthy 回滚、digest 提升或正式冒烟失败时停止发布。只重跑失败及其受影响的下游步骤；候选输入未变化时不重复已经成功的独立门禁。高风险必测场景受外部条件阻塞时仍不得降级为“发布后观察”。
10. 发布完成后把候选 workflow ID、正式 release workflow ID、唯一 digest、选择/跳过的矩阵、实际耗时、故障和资源清理结果写入 `docs/09-image-build.md`、对应接手文档和路线图。发布后证据提交不得移动既有 tag，也不得使已经发布的同 digest 候选重新走门禁。

如果任何高风险场景因外部条件无法验证，停止发布并向用户说明阻塞项；不得擅自降低为“发布后再观察”。

## 命令执行与编码错题本

项目级错题本固定为 `.agents/error-notebook.md`。它是每次工作的必读输入，不是任务结束时可选的复盘。

- 命令、Shell、工具选择、路径、权限、环境、编码、换行或引号导致非预期失败时，先判断根因，再修改执行方式；禁止不改变假设地重复同一失败命令。
- 找到正确方式后，必须在继续同类操作前或至少在本次任务结束前更新错题本。记录日期、环境、错误命令或模式、症状/退出码、根因、正确命令或做法、预防检查和适用范围；所有密码、token、cookie、私有 URL 参数必须脱敏。
- 已有同类条目时更新“最近复发/补充”，不要堆重复条目。某错误重复出现两次，必须把预防规则提升到 `AGENTS.md`、脚本、测试或自动门禁，不能只继续记笔记。
- 产品测试按预期发现的业务失败不必逐条记；但测试命令写错、环境选错、乱码、误用 Shell、清理范围错误等执行问题必须记录。
- 任务交付前检查本次是否出现新的执行类错误；若有但错题本未更新，任务不得标记完成。
- 多代理协作只需即时状态时使用 `list_agents`；调用 `wait_agent` 等待结果时 `timeout_ms` 必须不低于工具 schema 的 `10000` 最小值，不得用短轮询试探参数边界。
- Codex Desktop 任务工具固定使用服务端分页上限：`list_threads limit<=50`、`read_thread turnLimit<=10`；需要更多任务或更早历史时只使用响应游标分页，不得试探更大的单页参数。

## Shell、工具与文件编码约定

### 生产 SSH

- Windows 当前用户已持久安装 `Posh-SSH 3.2.7`，模块路径为 `C:\Users\anxi\Documents\PowerShell\Modules\Posh-SSH\3.2.7`。连接飞牛服务器时优先在 PowerShell 7 中使用 `New-SSHSession`、`Invoke-SSHCommand` 和 `Remove-SSHSession`，不要再临时安装 Paramiko、Plink 或其它 SSH 客户端。
- 飞牛连接参数固定为主机 `121.40.29.22`、端口 `22000`、用户 `cz`。只使用用户在当前会话明确提供的密码构造内存中的 `SecureString`/`PSCredential`；密码不得写入本文件、脚本、PowerShell profile、环境持久化、日志或 Git。每次操作必须在 `finally` 中关闭 SSH session。
- 默认采用用户名密码认证。除非用户在当次请求中明确要求，不得创建本机 SSH 密钥、上传公钥、修改服务器 `authorized_keys` 或切换为密钥认证。首次主机指纹只允许在已核对目标主机时用 `-AcceptKey` 接受，后续不得绕过主机密钥校验。
- 普通只读或非交互命令使用 `Invoke-SSHCommand`；必须输入 `sudo` 密码时使用受控 `New-SSHShellStream`，密码只写入该会话流且不得回显或拼进远端命令行。传给 SSH 的 PowerShell 双引号字符串中禁止出现远端 `$变量`、`$()` 或反引号命令替换；简单探针改用不需要远端插值的独立命令，复杂远端诊断写任务专属脚本或使用 UTF-8 base64 载荷，避免 `pwsh → SSH → sh` 多层转义。

- Windows 上所有 PowerShell 命令使用 PowerShell 7：`pwsh -NoLogo -NoProfile -Command '& { ... }'`，禁止调用 `powershell.exe`。外层使用单引号脚本块，避免父 PowerShell 提前展开 `$变量`；路径操作优先 `-LiteralPath`。调用 `git`、`go`、`npm`、`docker`、`python` 等原生命令后显式检查 `$LASTEXITCODE`。禁止为压缩单行而删除 PowerShell 关键字、cmdlet 与参数之间的空白；`throw 'message'`、`exit $LASTEXITCODE`、`Get-Content -LiteralPath` 必须保持明确分词，不能写成会被解析为新命令名的连写形式。关键清理包装器应设置 fail-fast 或把安全断言拆成可读语句，不能让非终止解析/命令错误被后续成功掩盖。
- PowerShell 中原生命令的可执行名必须直接处于命令位置（如 `rg ...`），或通过调用运算符执行字符串/变量（如 `& $rgExe @args`）；禁止把可执行名和参数生成成相邻的独立字符串字面量（如 `'rg' '-n' ...`），该形态只会在解析阶段报错。批量生成命令时优先生成参数数组并调用一次，不拼 PowerShell 源码文本。
- 嵌套 `pwsh -Command` 的文本检索默认拆成多次 `rg -F` 或使用 `Select-String -SimpleMatch`；含单双引号、反引号或复杂字符类的正则必须写入任务专属脚本，禁止继续内联到多层命令字符串中。
- 原生命令失败后若还要运行 `docker logs`、`inspect` 等诊断，必须先把原始 `$LASTEXITCODE` 保存到任务专属变量，诊断完成后退出该保存值；不得在其它原生命令之后再直接 `exit $LASTEXITCODE`。长运行服务先做有上限 readiness 轮询。
- Docker `inspect` 需要读取嵌套 label、数组或多个字段时，必须输出完整 JSON 并由 PowerShell `ConvertFrom-Json` 投影；禁止在多层 PowerShell 命令中拼接带引号或反斜杠的 Go template。只有经过独立探针的单个无引号字段才可使用 `--format`。生产投影除 `Config.Env` 等凭据外，还必须默认剔除匿名 volume hash、容器/网络完整 ID、存档 GUID 和玩家关联标识；只输出完成判断所需的布尔值、类型、计数或脱敏短形态。
- Windows 发布夹具不得把 Python `-c`、内联 JSON、正则、`find -printf`、`cut` 或其它含多层引号的逻辑继续嵌入 `pwsh → docker exec → sh -c`。优先直接传递单个命令及独立参数；确需多步逻辑时使用 `apply_patch` 创建任务专属脚本后执行，并先探测 BusyBox/GNU/OpenSSL 等实际能力，禁止靠猜测 flag。
- PowerShell 通过 `ConvertFrom-Json` 或 `Invoke-RestMethod` 读取 ISO 8601 时间时，不得直接与原始字符串比较；先确认值类型，统一转为 UTC 并以不变区域格式规范化，再做精确断言。
- PowerShell 的语句形式 `foreach (...) { ... }`、`if (...) { ... }` 不能直接接管道；需要继续传给 `Format-Table`、`ConvertTo-Json` 等命令时，先用 `@(...)` 收集输出，或改用 `ForEach-Object`。工具调用中的单行批处理默认使用 `ForEach-Object`，不要写语句式 `foreach` 后再接管道。
- Bash 脚本必须在 Git Bash、WSL2 或 Linux 容器中执行；发布一致性测试优先 Linux 容器。脚本保持 LF，并运行 `bash -n`、功能测试和 ShellCheck。不要把 Windows `cmd`/PowerShell 的转义规则混入 Bash 命令。
- Python 必须先确认解释器：Windows 上运行 `Get-Command python` 并执行版本探针；若不可用或返回 `9009`，立即改用工作区依赖提供的精确 Python 路径或已验证的 `py -3`，不要继续重试 Store alias。CI 使用 workflow 明确配置的 Python。
- Docker 操作前先运行 `docker info`；Docker Desktop 未启动时先启动并轮询就绪。临时资源必须使用任务专属前缀/label，创建前查重，清理前核对归属；禁止 `docker system prune`、`docker volume prune` 或模糊批量删除。`golang:*-alpine` 中执行 Go 命令使用 `sh -c`，不要用可能重置 PATH 的 `sh -lc`。
- 正式候选预取上一正式版和固定 fixture 镜像时，每个精确引用必须使用最多三次的有界 `docker pull`，单次 GHCR/Docker Hub token、TLS 或 EOF 失败不得直接判成镜像缺失；成功后必须 `docker image inspect` 再打包进入隔离 DinD。重试不关闭 TLS/认证、不改变引用，也不得重放 push、tag 或 workflow dispatch。
- 本地 Vite、VitePress、Python HTTP 等长运行预览服务必须直接作为可等待的 `shell_command` cell 运行；Windows 当前策略会拒绝嵌套 `pwsh` 中用 `Start-Process` 派生后台预览，禁止再次使用该形态。工具超时或终止 cell 后不得假定子进程已退出。启动前和清理后都要用 `Get-NetTCPConnection -State Listen` 检查精确监听端口，不能把同号 outbound/Bound 连接误判为服务残留；清理时同时核对 PID、进程名、工作区命令行和端口，只停止本任务拥有的进程。
- 正式发布门禁不得在同一个工具编排调用中以 `Promise.all` 等方式并发启动多个长运行 Shell；必须逐项使用可等待、可取得完整退出码与输出的独立调用。编排层异常、超时或提前返回后，先核对精确宿主进程、容器和 volume，再决定恢复或重跑，禁止在终态未知时重复启动同一门禁。
- Windows `exec_command` 内部 `Start-Sleep`/轮询等待必须明显短于对应 `yield_time_ms`，禁止用 30 秒 sleep 吃满 30000 ms 工具上限；状态复查默认立即查询。长命令返回 session 时必须保留完整返回对象并用 `write_stdin(session_id)` 续接，不能只投影 output 或把缺失 exit code 当失败。
- Windows 上 `npm ci` 若因现有 `node_modules` 文件锁报 `EPERM`，不得强删目录或反复重试；改用与发布版本一致的 Node Linux 容器和独立 `node_modules` volume 完成门禁，再按精确名称清理测试 volume。
- 进入任一 Node 子项目执行门禁前必须先读取该目录当前 `package.json.scripts`，只能调用实际声明的脚本；不得把相邻 package 的脚本名直接复用。官网正式构建入口是 `website` 下的 `npm run docs:build`，不是 `npm run build`。
- 前端洁净发布门禁必须把完整仓库挂到容器内稳定根目录，并从 `<repo>/frontend` 运行；`test:responsive-layout` 会读取仓库根 `.github/workflows`，禁止只挂 `frontend/` 后把它误解析成 `/.github`。`frontend/node_modules` 与 `frontend/dist` 使用任务专属独立 volume。
- `TestSMAPIArchiveRealDownload` 会断言 Linux `0600` 权限，正式发布门禁只能在任务专属 Linux 容器与独立 Go module/build cache 中运行；禁止先在 Windows 宿主试跑并把必然的 `0666` 当成产品失败。其它涉及 `Mode().Perm()`、UID/GID、symlink 或 Unix socket 的发布测试同样先选择目标 Linux 文件系统。
- 应用内 Browser 验证本地 Vite/VitePress 时使用 `domcontentloaded` 后等待唯一可见 DOM，不使用当前后端不支持的 `networkidle`；导航断言只传文档支持的精确 URL，不能传正则/predicate。静态站的精确目标必须从当前 DOM `href` 与实际 SPA/普通文档路由模式解析，不得硬编码 `.html` 规范化假设；主测试与 A/B/补充脚本共用同一目标契约。VitePress 的复合链接可能含图标/箭头，标题 accessible name 可能附带 permalink；定位前先读 DOM snapshot，链接优先用唯一 role/href，标题顺序从 `main h1/h2` 可见文本或首文本节点断言，禁止把肉眼主文案直接传给 `exact:true` 重放已知超时。窄屏固定导航页面不得用 `fullPage` 拼接截图判断渲染，必须结合普通视口截图与 root/body `scrollWidth <= clientWidth` 度量。
- 应用内 Browser 从移动壳恢复桌面验收时不得把 `viewport.reset()` 当成路由或断点恢复保证；必须显式设置已知桌面宽度、重新导航到精确目标 URL、读取当前 DOM snapshot 确认 shell/route，再等待桌面专属 heading。后端重启、HMR reload 或移动壳自动改写内部路由后同样执行此顺序，禁止直接复用上一视口的 locator 假设。
- 应用内 Browser 当前截图接口固定使用 `tab.screenshot({ fullPage: false })` 并用 `nodeRepl.emitImage` 展示；`tab.playwright` 只用于 DOM/locator，禁止使用不存在的 `tab.playwright.screenshot()`，即使上层技能示例这么写也以 runtime API 为准。
- 应用内 Browser 的持久 tab 引用跨用户中断、turn 或自动清理后可能失效。关闭、读取或继续操作旧 tab 前，必须先用当前 browser 的 `tabs.list()` 按 id 核对仍存在；缺失时直接丢弃旧引用并按需要重新取得标签，不得对已知 stale 引用重放 `url()`、`close()` 或其它操作。
- 精简容器运行项目门禁前必须核对子进程依赖：VitePress `lastUpdated` 构建使用 Node Alpine 时先安装 `git`；兼容矩阵需要 Docker CLI 与 buildx，updater/runtime Docker integration 需要 Docker CLI 与 Compose；挂载任务允许的 Docker Socket 后仍必须先通过相应 `docker version`、`docker buildx version`、`docker compose version` 探针。第三方 lint 镜像首次使用前先 inspect Entrypoint/Cmd，ShellCheck 命令必须显式调用 `shellcheck`。
- GitHub-hosted runner 上的发布工具先以精确 runner image 官方 software readme 和 `command -v`/版本探针为准；已预装的 Skopeo、Docker、gh、jq 等不得在正式提升里再次即时 `apt-get update/install`。确需安装新工具时必须有版本固定、完整性校验和有界网络等待，不能让静默软件源占满整个发布 timeout。
- GitHub CLI 的 `gh release view --json` 不支持 `isLatest`，禁止在发布核验中使用该字段。读取当前 latest 固定使用 `gh api repos/{owner}/{repo}/releases/latest --jq .tag_name`，或不带 tag 的 `gh release view --json tagName`；其它 JSON 字段也必须先以对应子命令帮助列出的字段集为准。
- VitePress/Vite 构建会在 `.vitepress` 配置目录创建 `config.ts.timestamp-*.mjs`；禁止再使用“完整源码只读 bind，仅给 `node_modules`/cache/dist 写卷”的门禁形态。使用任务副本，或允许配置目录临时可写并在构建后用 Git 状态确认只剩预期修改；产物、缓存和依赖仍用任务专属 volume 隔离。
- Control Mod 真实 C# 编译必须复用项目已验证的标准 `dotnet build -c Release /p:GamePath=/game /p:EnableModDeploy=false` 输出路径；不得在含既有 `bin/obj` 的源码树上改写 `BaseIntermediateOutputPath`/`BaseOutputPath`，否则旧生成源码会重新进入编译。需要只读源码隔离时先制作排除 `bin/obj` 的任务副本；一次性 SDK 容器没有已验证 NuGet 缓存时不得强制 `--network none` 后假定 restore 可用。
- 容器内测试调用宿主 Docker 时，daemon 看不到调用方容器私有的 `t.TempDir()`/`/tmp`。凡测试会把临时路径作为二级容器 bind source，必须改在带所需工具链的任务专属 DinD 容器内执行，或使用双方明确共享的宿主 bind；不能仅挂 Docker Socket后假定路径可见。
- Windows Docker Desktop 向 DinD 预加载镜像时，优先为任务容器绑定唯一环回 TCP 端口，并用宿主 CLI `docker -H ... image load -i`；若使用 `docker cp`，即使退出 0 也必须立即在目标端核对存在、大小和摘要，不能仅凭退出码继续。
- `rg` 在 Windows 上不要传递未由 Shell 展开的 `path/*` 或 `Dockerfile*`；使用 `rg -g '<glob>' <pattern> <root>`、明确目录或先用 `rg --files`。搜索模式以 `-` 开头时必须使用 `-e '<pattern>'` 显式声明，或在其它参数后用 `--` 结束选项解析；引号与 `-F` 都不能替代该边界。文本搜索优先 `rg`，文件列表优先 `rg --files`。同一任务一旦复发该错误，余下每条 `rg` 的位置参数只允许一个已确认、无通配的目录；即使组合命令前段已有有效输出，也禁止追加第二个猜测路径或通配路径。
- 按函数、类型或文本定位源码时，`rg` 返回的真实文件路径是后续读取的唯一依据；禁止在同一组合命令后半继续读取凭记忆猜测的文件名。先完成检索，再以独立 fail-fast 命令读取精确命中路径；PowerShell 只读组合也必须使用 `$ErrorActionPreference='Stop'` 或逐项检查，不能让后续成功命令掩盖 `Get-Content` 失败。
- Git tag/ref 发布审计中，`git for-each-ref --format` 禁止使用属于 `git log --format` 的 `%n` 换行占位符；结构化证据默认一字段一命令，只有经过独立探针确认时才使用该子命令支持的 `%0a`。tag type、object、peeled commit、tagger date 与 subject 分别核对，不能让退出 0 的字面量占位符冒充有效格式。
- Web 搜索编排层若连续两次在执行前返回同类解析错误，停止改写并重放该搜索形态；改用已确认的官方精确 URL，或使用已验证的 CLI/API 读取同一主来源。`functions.exec` 中编排 `web__run` 对象数组时，只从已验证骨架复制 `{q: "..."}`、`{ref_id: "..."}` 等完整键值结构，不得手写混合 JavaScript 与 JSON 的键名。
- 所有新建文本文件默认 UTF-8 无 BOM。修改前保留原文件编码和换行，不得为了改几行重编码整个文件。Go/TS/JS/JSON/YAML/Markdown 使用 UTF-8 无 BOM；`.env` 必须 UTF-8 无 BOM，否则 Docker Compose 会把 BOM 当作键名字符。
- 换行遵循 `.gitattributes`：`.sh` 为 LF，`.ps1` 为 CRLF；只有明确兼容 Windows PowerShell 5.1 的既有脚本可以保留已验证的 BOM，例外必须写入错题本或对应文档。
- 文件修改使用 `apply_patch`。完成后至少运行 `git diff --check`，查看 `git status --short` 和差异范围；Go 文件运行 `gofmt`，JSON/YAML/脚本运行对应解析或语法检查。U+FFFD 审计只检查 `git diff --unified=0` 中单个 `+` 开头且排除 `+++` 文件头的本次新增行，不得扫描整个历史文件后把合法示例误报为新乱码；BOM 仍检查完整变更文件。发现新增 Unicode replacement character（`U+FFFD`）、BOM、整文件异常换行变化或中文乱码时立即停止，先恢复正确编码再继续。
- `gofmt`、`go test` 与其它按相对路径执行的命令在发送前必须同时核对 `exec_command.workdir`：从仓库根执行时使用 `backend/...`，从 `backend` 子目录执行时使用 `internal/...`，禁止把子目录 workdir 与仓库根前缀叠加。需要同时格式化跨包文件时默认从仓库根执行已确认的根相对路径。
- 发布说明与 Release 资产验收不得把外部更新、下载、校验和 `Remove-Item` 清理合在同一长 Shell cell；先独立完成并确认校验结果。工作区内任务专属的已知文本临时文件使用精确 `apply_patch` 删除，不用动态循环或递归 Shell 删除；空目录不进入 Git，可在不扩大删除权限的前提下保留。
- Windows 上需要递归删除工作区任务目录时，不得把 `Remove-Item -Recurse` 直接内联到工具命令；从一开始就用 `apply_patch` 创建任务专属 `.ps1`，脚本内核对解析后的绝对路径精确等于预期目标且位于 `.agents` 等任务边界内，执行并验证清零后再用 `apply_patch` 删除脚本。策略拒绝视为零执行，不得原样重试。
- `apply_patch` 的多文件或 update/delete 混合操作默认拆成独立补丁；确需多文件时必须先结束当前 hunk，再写下一个 `*** Update File`/`*** Delete File` 声明。出现 `Unexpected line found in update hunk` 时视为零修改，先检查实际 diff，再按文件拆分，禁止原样重放。
- `apply_patch` 的 update hunk 按完整行匹配；即使只改行内一个数字，也必须从当前文件读取并提供完整的删除行与新增行，不能把半行或行内片段当作可匹配的删除行。长行先用 `rg -F`/精确邻域取得真实文本，不从聊天摘要或记忆手抄。
- 工具返回对象或 JSON 序列化输出中的 `\"` 可能只是传输层对真实 `"` 的转义，不能直接抄进 `apply_patch`。含引号/反斜杠的行在首次补丁匹配失败后，必须先用字节/字符码或读取到独立原始文件确认真实字符；不得仅换 JavaScript 字符串写法原样重放同一假设。
- 同一文件存在重复代码片段时，`apply_patch` hunk 必须包含目标函数、测试或章节名等唯一语义锚点；补丁成功后、任何长门禁开始前，必须用精确 `git diff -- <file>` 与逐处检索确认修改命中目标且未改相邻副本。测试偶然通过不能替代这项审查。
- 跨 worktree 整合旧差异时先用 `git diff --ignore-space-at-eol` 去除换行噪声；不得把带行号的原始 Git hunk header 直接重放给 `apply_patch`。对已在 `main` 变化的文件必须重新读取当前上下文并做语义合并，禁止用整文件覆盖掩掉新版本内容。
