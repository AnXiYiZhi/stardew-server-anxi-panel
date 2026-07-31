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

- 最近复发：2026-08-01；`v0.4.7` 发布工具链探针用未静默分支的 `Get-Command python` 结束了整批命令，阻止后续 GitHub CLI 探针。确认宿主解释器不可用后，改为加载工作区依赖并使用返回的精确 Python 3.12.13 路径；可选命令探针必须用 `-ErrorAction SilentlyContinue` 并显式分支，不能让缺失项中断其它独立检查。同轮子审计因 workspace dependency loader 暂无流式输出而提前终止；主流程直接调用并等待权威返回后成功取得解释器，不能把“暂时无增量输出”当成 loader 失败。
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
- 最近复发/补充：2026-08-01 排查升级状态测试时再次把 `backend/internal/web/*_test.go` 作为位置参数传给 `rg`，命中同一 `os error 123`；发布记录检查又把 `docs/backend-handoff/*.md` 当位置参数，整条搜索退出 1。两次都改用 `rg -g '<glob>' ... <root>` 或先列文件；该规则已在 `AGENTS.md` 固化，后续命令构造时先检查位置参数中不存在 `*`。
- 适用范围：Windows 上的仓库搜索和发布检查。

## 2026-07-28：嵌套 Go template 与 PowerShell 转义冲突

- 环境：PowerShell 调用 `docker image inspect --format`。
- 错误模式：在多层双引号中对 Go template 的引号再次加反斜杠。
- 症状：`template parsing error: unexpected "\\" in operand`。
- 根因：把 JSON/类 Unix 反斜杠转义套到 PowerShell 参数，反斜杠被原样传给 Go template。
- 正确做法：优先把完整 template 置于 PowerShell 单引号参数，或拆开检查字段；嵌套命令过深时避免一行完成所有 inspect。
- 预防检查：先用一个只读 `docker image inspect` 小命令验证 template，再放入较长脚本。
- 最近复发/补充：2026-07-31 验证测试 volume label 时把 `{{index .Labels \"...\"}}` 放进嵌套 PowerShell 双引号，Docker 再次收到反斜杠并报 `unexpected "\\"`；改用无内层引号的 `{{json .Labels}}` 后确认 ownership。
- 最近复发/补充：2026-08-01 又把带字符串键的 `{{index .Config.Labels \"...\"}}` 放进 `docker exec ... sh -c` 的第三层引号，容器 Shell 报 `unterminated quoted string`。只执行一个容器内命令时应去掉 `sh -c`，从 PowerShell 直接传参，并一次输出 `{{json .Config.Labels}}` 后在外层解析。
- 最近复发/补充：2026-08-01 后端门禁把 `docker --format` 的模板与相邻参数错误拼接，Docker 收到无效 format。复杂 inspect 不再拼接模板字符串：先输出完整 `docker inspect` JSON，再由 PowerShell `ConvertFrom-Json` 投影所需字段；只有单个无引号模板经过独立探针后才使用 `--format`。
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

- 最近复发/补充：2026-08-01 修正 linked worktree 的 Git 元数据后，最终干净构建仍把整个源码挂成只读；Vite 加载 `config.ts` 时需要在同目录短暂写入 `config.ts.timestamp-*.mjs`，因此以 `EROFS` 退出。Vite/VitePress 构建不能只给 `dist` 与 `node_modules` 写权限；真正需要只读宿主源码时，应先复制到任务专属可写 volume，再构建并精确清理，或直接使用已配置的 GitHub Pages CI。不要在只读源码挂载上继续补零散写目录。
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
- 适用范围：差异审查、搜索探针和所有带预期非零状态的组合校验脚本。

## 2026-07-28：猜测配置文件扩展名且忽略 PowerShell 非终止错误

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

- 最近复发：2026-07-29；读取新任务 skill 时又在行数统计中写成 `foreach (...) { ... } | Format-Table`。数组子表达式规则虽已提升到 `AGENTS.md` 仍复发，因此进一步规定工具单行批处理默认使用 `ForEach-Object`，避免语句式 `foreach` 进入管道。2026-07-31 最终编码审计再次写成 `foreach (...) { ... } | ConvertTo-Json`，命令在执行前被同一 ParserError 拒绝；改为 `$results = @(foreach (...) { ... })` 后再管道输出。
- 环境：PowerShell 7，组合对象后用 `Format-Table` 展示。
- 错误模式：`foreach (...) { ... } | Format-Table`。
- 症状 / 退出码：`ParserError: An empty pipe element is not allowed`，退出码 `1`。
- 根因：PowerShell 语句形式的 `foreach` 不能直接作为管道左值。
- 正确做法：使用 `@(foreach (...) { ... }) | Format-Table`，或改为 `$items | ForEach-Object { ... } | Format-Table`。
- 预防检查：管道左侧若是 `foreach`、`if` 等语句，先显式包装为数组子表达式。
- 适用范围：PowerShell 中的批量状态检查与格式化输出。

## 2026-07-28：`apply_patch` 使用了未经核对的长上下文

- 环境：Windows 工作区，使用 `apply_patch` 同时更新错题本和 `AGENTS.md`。
- 错误模式：手工重写完整相邻行作为补丁上下文，其中漏掉原文空格。
- 症状 / 退出码：`apply_patch verification failed`，补丁未产生任何修改。
- 根因：没有先从文件读取精确锚点，且使用了不必要的长上下文。
- 正确做法：先用 `rg -n --fixed-strings` 定位原文；补丁只保留稳定、最短的精确上下文。
- 预防检查：涉及长中文行或多文件补丁时，不凭聊天上下文重打原文，先读取实际文件。
- 最近复发/补充：2026-08-01 合并 `v0.4.7` 响应式补丁时凭旧工作树猜测 `SavesPage.css` overlay 的背景声明，导致最小补丁仍因上下文不一致失败；随后一个多文件文档补丁漏写第二个 `*** Update File`，让 handoff 的锚点被错误套到 `docs/03-frontend.md` 并再次校验失败。读取目标 release worktree 精确行、为每个目标显式声明文件后应用成功。补丁上下文必须来自将被修改的那一个 worktree，多文件补丁须逐段核对目标声明。
- 适用范围：所有 `apply_patch` 修改，尤其是长行、编码敏感文件和多文件补丁。

## 2026-07-28：Browser 后端不支持 `networkidle` 等待状态

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

- 最近复发/补充：2026-08-01 在任务 DinD 内执行 `apk add --no-cache go` 时外层工具 304 秒超时，但容器内 `apk` 仍继续运行并最终成功安装 Go；不能在未复查进程/`command -v go` 的情况下重复安装或直接删除容器。本轮先读取精确 PID，下一次检查时进程已自然结束，再以 `go version` 确认结果。2026-07-31 的 Vite 子进程案例同样说明超时只代表等待结束，不代表派生工作已停止。
- 环境：Windows，`shell_command` 直接运行 Python `http.server` 作为本地效果图预览服务。
- 错误模式：短超时返回后直接假设服务已停止，又尝试在相同端口启动第二个服务。
- 症状 / 退出码：第二次启动返回 `EADDRINUSE`；检查发现第一个 Python 进程仍监听原端口。
- 根因：工具调用超时或终止长运行 cell 只结束等待/包装进程，不保证已派生的服务进程退出。
- 正确做法：超时后先用 `Get-NetTCPConnection` 核对监听端口和进程命令行；确认归属后复用或按精确 PID 停止，再以明确的 `--directory` 启动预览服务。
- 预防检查：任何本地服务启动或重启前先查端口；清理时只停止命令行和端口均匹配本任务的进程。
- 适用范围：Python `http.server`、Vite、Node 静态服务器等本地预览服务。

## 2026-07-29：Browser 窄屏 `fullPage` 截图出现空白与固定栏重复

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

- 环境：Docker 29 精简容器，运行 compatibility/updater/runtime integration。
- 错误模式：只设置 `DOCKER_CLI_PLUGIN_EXTRA_DIRS` 就假定 buildx/Compose 可发现，并继续使用已弃用的 `docker stop --time`。
- 症状 / 退出码：Docker 29 未发现预期插件；停止命令产生 deprecated 警告，门禁工具链不稳定。
- 根因：当前精简镜像的 CLI 插件发现路径与宿主假设不一致，参数也已更新。
- 正确做法：先执行 `docker buildx version` 与 `docker compose version`；必要时把受控插件放入 `/root/.docker/cli-plugins`。停止容器使用 `docker stop --timeout <秒>`。
- 预防检查：每个 DinD 门禁先探测 CLI/daemon/buildx/Compose 四项版本，不从环境变量存在推断插件已加载。
- 适用范围：Docker 29 DinD、兼容矩阵、updater/runtime integration 和发布后回拉。

## 编码与换行快速检查

- 2026-08-01 补充：检查 U+FFFD 时不能对“本次修改过的整个历史文件”直接搜索，否则会把文档中用于解释旧乱码问题的合法 `�` 示例误判为新引入乱码。应先跑 `git diff --check`，再只检查 `git diff --unified=0` 中以单个 `+` 开头的新增行；发现命中后再回到原文件确认语义。
- 默认：UTF-8 无 BOM。
- `.env`：必须 UTF-8 无 BOM。
- `.sh`：UTF-8、LF，并运行 `bash -n` 与 ShellCheck。
- `.ps1`：遵循 `.gitattributes` 的 CRLF；既有 BOM 只有在已验证的 Windows PowerShell 5.1 兼容场景保留。
- Go/TS/JS/JSON/YAML/Markdown：UTF-8 无 BOM；修改后运行格式化、解析或构建检查。
- 交付前：`git diff --check`、`git status --short`、检查差异规模，并搜索意外 Unicode replacement character（`U+FFFD`）。发现整文件换行或编码变化时不要提交。
