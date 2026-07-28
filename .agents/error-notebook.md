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

## 2026-07-28：嵌套 PowerShell 提前展开变量

- 环境：Windows，外层 Shell 已是 PowerShell，再调用 `pwsh -Command "...$variable..."`。
- 错误模式：在双引号命令字符串中定义并使用 `$latest`，父 PowerShell 先展开变量，子进程收到残缺表达式。
- 症状：`ParserError`，提示 `You must provide a value expression following the '+' operator`。
- 根因：父、子两层 PowerShell 的变量与引号边界混用。
- 正确做法：统一使用 `pwsh -NoLogo -NoProfile -Command '& { $latest = ...; ... }'`；路径使用 `-LiteralPath`。复杂命令拆成多个明确步骤，不在双引号中嵌套 PowerShell 代码。
- 预防检查：含 `$` 的子 PowerShell 代码在执行前确认外层为单引号脚本块。
- 适用范围：所有 Windows `shell_command`、发布脚本和临时诊断命令。

## 2026-07-28：Python 命令不存在却被后续命令掩盖

- 最近复发：2026-07-28；本机同时缺少可调用的 `py` launcher，完成 `Get-Command python` 探针后应直接加载工作区依赖并使用返回的精确解释器路径，不再尝试 `py -3`。
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
- 适用范围：Windows 上的仓库搜索和发布检查。

## 2026-07-28：嵌套 Go template 与 PowerShell 转义冲突

- 环境：PowerShell 调用 `docker image inspect --format`。
- 错误模式：在多层双引号中对 Go template 的引号再次加反斜杠。
- 症状：`template parsing error: unexpected "\\" in operand`。
- 根因：把 JSON/类 Unix 反斜杠转义套到 PowerShell 参数，反斜杠被原样传给 Go template。
- 正确做法：优先把完整 template 置于 PowerShell 单引号参数，或拆开检查字段；嵌套命令过深时避免一行完成所有 inspect。
- 预防检查：先用一个只读 `docker image inspect` 小命令验证 template，再放入较长脚本。
- 适用范围：Docker inspect、Compose format 和其它 Go-template CLI。

## 2026-07-28：并行门禁被单个命令构造错误吞掉输出

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

- 环境：PowerShell 7，读取 VitePress 配置。
- 错误模式：未先查看实际文件便读取 `website/docs/.vitepress/config.mts`，并在脚本末尾无条件 `exit 0`。
- 症状：`Get-Content` 报路径不存在，但组合命令仍显示 exit 0，后续输出容易让人误以为所有输入都已读取。
- 根因：根据常见项目结构猜测扩展名；PowerShell cmdlet 默认产生 non-terminating error，不会自动写入原生命令的 `$LASTEXITCODE`。
- 正确做法：先用 `rg --files website/docs/.vitepress` 或 `Get-ChildItem -LiteralPath` 确认实际路径；关键读取使用 `-ErrorAction Stop`，组合脚本开头设置 `$ErrorActionPreference = "Stop"`，只在全部步骤成功后 `exit 0`。
- 预防检查：不得猜测 `.ts`、`.mts`、`.js` 等扩展名；读取多个必需文件前先一次性验证路径存在。
- 适用范围：PowerShell 文件发现、配置审查和多步骤只读检查。

## 2026-07-28：子目录工作目录与仓库根相对路径混用

- 环境：PowerShell 7，`shell_command` 的 `workdir` 设为 `website/`，同一脚本先构建站点再校验仓库文件。
- 错误模式：构建完成后仍用 `AGENTS.md`、`docs/...` 等仓库根相对路径执行 `Resolve-Path`。
- 症状：VitePress production build 已成功，后续编码校验却报 `Cannot find path 'AGENTS.md'`，整段命令最终 exit 1。
- 根因：把 npm 项目工作目录和仓库校验基准混成一个相对路径上下文。
- 正确做法：组合门禁统一在仓库根执行，并用 `npm.cmd --prefix website run docs:build` 调用子项目；或在脚本开头解析并固定仓库根绝对路径，所有后续文件基于该路径拼接。
- 预防检查：执行多目录组合命令前列出每一步的路径基准；设置非根 `workdir` 时禁止直接使用仓库根相对路径。
- 适用范围：website/frontend 子项目构建、跨目录编码检查和发布组合门禁。

## 2026-07-28：Browser 后端不支持 `networkidle` 等待状态

- 最近复发/补充：在手机页面数组中连续 `goto` 后立即读取 DOM，前三页正常，第四页仍是空标题/空正文；导航 Promise 完成不等于 VitePress 客户端内容已完成渲染。
- 环境：Codex 应用内 Browser，对本地 VitePress 开发服务器做页面 QA。
- 错误模式：按通用 Playwright 类型调用 `tab.playwright.waitForLoadState({state:"networkidle"})`。
- 症状：工具直接返回 `playwright_wait_for_load_state does not support networkidle`。
- 根因：当前 Browser 控制后端只实现部分 load state，能力小于通用 Playwright 类型声明。
- 正确做法：本项目页面导航使用 `domcontentloaded`；之后等待明确的页面标题、唯一 heading/link 或直接读取目标 DOM 状态，不以全局网络空闲作为就绪条件。连续检查多个路由时，每次导航后都必须单独等待目标页面状态，不能只在循环末尾统一读取。
- 预防检查：Browser 插件的有限 Playwright API 以运行时实际支持为准；遇到不支持的方法或参数立即换用可观察页面状态，不重复同一调用。
- 适用范围：VitePress、Vite HMR 和其它本地 SPA 的应用内 Browser 验收。

## 编码与换行快速检查

- 默认：UTF-8 无 BOM。
- `.env`：必须 UTF-8 无 BOM。
- `.sh`：UTF-8、LF，并运行 `bash -n` 与 ShellCheck。
- `.ps1`：遵循 `.gitattributes` 的 CRLF；既有 BOM 只有在已验证的 Windows PowerShell 5.1 兼容场景保留。
- Go/TS/JS/JSON/YAML/Markdown：UTF-8 无 BOM；修改后运行格式化、解析或构建检查。
- 交付前：`git diff --check`、`git status --short`、检查差异规模，并搜索意外 Unicode replacement character（`U+FFFD`）。发现整文件换行或编码变化时不要提交。
