# 项目减负审计（2026-09-18—19，本地清理与验证完成，未发布）

本次在已有未提交工作上继续，基线 HEAD 为 `3236500ec6d5112f5b8cef15347e4aaea3752308`，始终使用本地 `main` 和原 worktree。开始时有 114 个已跟踪差异文件及 5 个业务未跟踪文件。已有 48 个素材删除、旧页面清理和其它代码调整不计入本次成果；没有暂存、提交、推送或发布。

本机证据位于 `output/project-load-20260918/`。`recovery-baseline.zip` 保留 before/source 与初始二进制 diff；`compact-evidence.tar.xz` 保留原路径命名的文件清单和完整长测试日志，分别有逐项 SHA-256 清单，归档读取验证通过后才删除重复副本。`verified-before` 是修正 Windows 文件身份统计后的有效基线，`after-final` 是最终快照；`task-only.diff` 只比较本任务与初始工作区。统计区分逻辑字节、硬链接去重、Docker 共享层与宿主实际分配，不把删除工作树文件写成 Git 历史缩小。

## 覆盖清单

| 范围 | 状态 | 证据与边界 |
| --- | --- | --- |
| A 素材/静态资源 | 已检查 | 素材 SHA-256、CSS/HTML/Markdown/动态拼接/glob/映射/嵌入与复制入口、52 组渲染；本轮没有素材改写/删除。字体、透明度、像素缩放与九宫格保持；历史设计与授权保留 |
| B 前端代码/样式 | 已检查 | AST 展开懒加载，状态脚本、production、桌面/手机/普通用户/初始化/登录与导航通过；无新增 CSS 删除，不以页面覆盖率裁剪规则 |
| C 后端/Control | 已检查；Control 仅静态核对 | 391 个 Go 文件、路由/接口/平台/构建标签/嵌入核对；9 个内部孤立实现清理并经 Linux 全量/真实镜像验证。Control 源码、NuGet、DLL/manifest 未改，宿主无 SDK，未重编译 |
| D 依赖/构建 | 已检查 | npm 55 个锁条目解析保持、Go tidy 无差异、NuGet 条件依赖保留；严格安装、真实 COPY 输入与最终镜像实测。没有无关升级 |
| E 脚本/部署/测试/文档 | 已检查 | CI/发布/人工入口、137 个本地链接、55 个外链；修复 1 个 404，SMAPI 的 HEAD 405 经 GET 200 确认有效。官网构建通过；独立安全/恢复回归与发布证明保留 |
| F 本地文件/缓存 | 已检查；未知边界保留 | 包括隐藏/忽略项、大小/文件身份、重建依据及进程/挂载；仅删两份可重建旧二进制。个人目录不遍历，业务数据只读元数据，长期授权与历史恢复现场保留 |
| G Docker/运行残留 | 已检查，恢复与任务清理完成 | 原三台运行容器均 healthy，原六容器/23 卷保留；任务容器/卷/镜像和八条上下文缓存精确回收，任务监听端口清零。默认 builder 的可复用/共享缓存保留 |
| H Git | 已检查；续查经追加授权重新打包 | main 单分支/单 worktree，无 LFS；初始 78,537 loose objects。用户明确授权后，78,947 个对象全部保留并重新打包，.git 2,045,047,302 → 1,190,609,173 B；两次完整 fsck 通过，refs/reflog/index 不变，未清除不可达对象或改历史 |

## 候选与验证

| 对象/精确路径 | 类别、规模 | 用途与证据（含动态/外部边界） | 风险、恢复方式、结论 |
| --- | --- | --- | --- |
| `.codex-test/steam-invite-optin-local-20260826/local-dev/panel-local-dev.exe` | 生成二进制，24,663,040 B | `go version -m` 确认为 cmd/panel，干净 commit `7b08b6484e0a226e9519f6f99605cdf8c2d3e55a` 已在 main；现存 Panel PID 的路径指向 `.gocache`；无容器挂载此目录 | 低；可从现存 commit 和锁定 go.mod 重建；已清理，仅删除可执行文件，保留相邻数据/日志 |
| `.codex-test/steam-invite-optin-local-20260826/docker-linux-amd64.test` | 生成测试程序，5,523,685 B | Go 元数据指向 internal/docker.test；Linux 测试程序，无现存容器挂载或命令使用 | 低；`GOOS=linux go test -c ./internal/docker` 重建；已清理 |
| `.dockerignore` | 构建上下文 | 现有仅根 `.codex-test` 排除，`backend/.codex-test` 238,468,225 B 随 COPY backend 进入 builder；前端/官网依赖、C# bin/obj、Python/Vite 缓存属于生成内容；嵌入仅显式 DLL/manifest、迁移、catalog/runtime manifest 和 frontend_dist | 低；增加精确忽略项，保留源码、锁文件、真实 Control DLL 和所有测试夹具；用实际 Docker 构建验证 |
| `frontend/package.json`、`frontend/package-lock.json` | 依赖分类 | Vite、TS、React 插件仅由 build/dev 使用；React/React DOM 是运行依赖；CSS/import/CLI 均已查 | 低；移动三项到 devDependencies，保留全部版本/完整性记录，用独立 npm ci 与生产构建验证 |
| `Dockerfile` frontend 安装行 | 可复现构建 | `npm install --frozen-lockfile` 不是 npm 的严格锁安装入口，失败再 install 会掩盖问题；CI 既有入口用 npm ci | 中；改为 `npm ci --include=dev`，保留构建工具；最终真实镜像构建与 24 项初始化/权限/数据卷重启验证通过 |
| `backend/.codex-test/phase2-candidate.tar` | OCI 镜像归档，63,069,184 B | 确认为 image tar，缺少足够的构建来源与可恢复候选证明关联 | 保留；仅修改 Docker 上下文排除，不删除唯一归档 |
| `backend/.codex-test/save-import-e2e-release*`、`save-import-local-rich`、`farmhand-delete-e2e` | 恢复夹具/持久数据 | 最新后端接手明确要求保护 finalizer-confirmed / Saved-unconfirmed 恢复现场，第三组曾被后续真实回归复用 | 必须保留，不因任务已旧删除 |
| `.gocache`、两处 node_modules | 缓存/依赖 | 初查 Panel/Vite 使用中；重启后续查确认旧 Panel 缓存已无运行进程，编译包与 Node 依赖仍有开发价值 | 续查仅将 4 份旧可执行缓存无损归档，保留最新一份、编译包缓存和两处 node_modules |
| 2 组完全相同图片（favicon.ico、favicon.png/logo.png） | 素材，50,692 B 重复 | 分属独立 frontend 与 website 发布根，各有真实引用和不同 URL 契约 | 保留；跨发布根共享增加耦合，不删除 |
| `frontend/public/assets/stardew/new-game/pets/{cat,dog}-{0..4}.png` | 10 张动态素材 | `NewGameCreator.tsx` 的 petType/breed 组合生成路径；不能按无字面文件名引用删除 | 必须保留；其他素材亦无已证明闲置项 |
| `frontend/src/vite-env.d.ts` | 编译声明 | TS AST 从生产/QA/测试导入图仅此文件无入边；由 tsconfig include 和三斜线指令消费 | 必须保留；107 个生产模块、141 个合并可达模块，14 个动态 import 均已展开 |
| `scripts/generate-stardew-page-prototypes.py` | 历史设计工具 | 无自动调用者，但有独立 main、Windows 中文字体和 docs/prototypes 输出入口；开发人工用途未被否定 | 保留，旧素材依赖标为待核查，不以无搜索命中删除 |
| Docker 原有 6 容器、23 volumes、镜像/共享 cache | 运行与恢复资源 | 存在活动游戏/Panel/安装测试；停止容器仍挂 game-data/Steam；部分匿名卷无足够来源 | 保留；不以 dangling、未挂载或名称相似推断可删除 |
| `.git` | Git 历史与不可达对象 | 初查只读；续查完整 fsck、全部对象集合与 refs/reflog/index 摘要核验，并获用户单独授权 | 普通 repack/cruft，明确 never 过期条件，保留全部 78,947 对象；只消除重复存储，减少 854,438,129 B。未改历史、reflog 或删除独有工作 |

逐项机器证据见 `asset-references.json`、`asset-duplicates.json`、`script-references.json`、`frontend-module-graph.json` 与 Docker 投影。静态未命中只作为调查入口，未作为单独删除理由。

后端第二批：Go AST 遍历全部 391 个源码/测试文件（包括平台文件与 build tags），登记 2,286 个函数/方法声明；交叉核对注释、接口、注册、路由、go:embed 与动态字符串后，8 个非导出实现只有自身声明，没有运行/测试入口。精确文件、字节范围、恢复补丁列于 `go-cleanup-plan.json`：旧 bool Control 等待包装、状态推断认证辅助、字符串截断、旧 pull 行处理器、Nexus 首页包装、未用错误脱敏包装、旧 Hub latest 比较、内存上传 claim 包装。随后跟进删除唯一调用者已消失的 dockerHubTagDigest 和孤立 client/三项正则，共 9 个实现、7 个 Go 文件、7,201 B。真实 Control gate、持久认证标记、Docker 进度、分页搜索、活动脱敏、driver 安装选项与 durable 上传事务继续由当前调用链承接。两个导出存档辅助函数保留，避免在本次构建减负中顺带收缩可见契约。

## 前后计量

| 口径 | 清理前 | 清理后/结论 |
| --- | ---: | --- |
| Go 源码 | 2,286 个函数/方法声明 | 2,277 个；净减 7,201 B。全部源码/构建配置/依赖文件合计净减 5,854 B，不含文档 |
| 前端 public | 134 文件，14,568,245 B | 保持；初始工作区已有素材删除不重复计数 |
| 前端 production dist | 228 文件，16,212,904 B | 保持；51→3 个 production 锁节点只是依赖分类变化 |
| 官网 production dist | 119 文件，3,691,272 B | 119 文件，3,691,208 B；链接修复后构建通过，不把该微小差异计作素材删除收益 |
| 三组 COPY 输入（backend/frontend/browser-extensions） | 1,294 文件，263,512,784 B | 762 文件，22,079,473 B；减少 241,433,311 B（91.6%）。包括移出 builder 的 238,468,225 B 恢复现场与 2,957,885 B 编译产物，宿主原文件仍保留 |
| 同参数本地镜像 Docker Size | 59,417,588 B | 59,417,134 B，仅减少 454 B；不宣称显著镜像瘦身 |
| 已核实删除的既有本地文件 | 30,186,725 B | 两份可重建二进制归零，删除前复核哈希、时间、进程与挂载；邻近数据/日志保持 |
| 审计证据新增占用 | 0 B | 约 39.3 MB；归档/日志/恢复副本仍是新增空间，精确口径见 after-final/evidence-size.json。证据新增大于已确认的 30.2 MB 文件删除，本轮不宣称宿主磁盘净减 |
| 全目录元数据快照 | 93,238 文件，3,360,976,575 B | 最终快照 93,234 文件，3,326,636,149 B；排除本任务 output 与未遍历的个人目录，两时点硬链接去重值与逻辑字节相同。活动数据/缓存变化不全部归因于本任务 |
| Docker 逻辑空间/宿主真实分配/VHDX | 初始 df 单独保存 | 仅报告实际资源清理，默认 builder 缓存仍保留；df 是含共享层的逻辑计量，不合并或推断宿主实际释放，VHDX 未缩容 |

## 验证结果

- 前端：清理前与首批后共 27 项当前声明状态脚本及 production build 通过。两个脚本/build 遇过宿主 native memory allocation 错误，恢复后仅重跑失败项，全部通过。后续仅修改官网链接和文档，前端源码/素材没有再次变化。
- 浏览器：已有 Chrome/Playwright 完成 52 组检查和 5 个手机导航交互，覆盖 9 路由、初始化/登录、游戏库/安装、普通用户及 320/390/1024/1486/1920 宽度；0 页面/控制台错误、0 HTTP 失败、0 破图/横向溢出。6 张代表截图保留，桌面与手机总览目视核验；这是隔离 API 夹具。
- 后端最终：同一 golang:1.25-alpine 镜像和源码，在 Linux tmpfs 临时目录下 1,924 个通过、8 个条件跳过，test/vet/build/tidy 均为 0。Docker 磁盘临时目录下，历史 TestInstallFailurePersistsActionableCause 的 space/dns 分支触发 15 秒等待超时，独立三轮也有一次 dns 超时；保留所有原断言与失败日志。tmpfs 通过不能说明磁盘时限波动已修复，后续记录于 docs/07-later-optimizations.md。
- 镜像：首批前后及最终真实构建均通过，隔离 builder 完成 npm ci。最终 dev/固定 commit/build date 镜像通过 24 项真实 HTTP 验证：health/version、未初始化、重复初始化拒绝、登录/注销、匿名/普通用户权限、HTML/静态资源、独立 Docker 数据卷上的完整用户列表和 session 跨重启、数据库健康。
- 官网：137 个本地文档链接无缺失；55 个外链核查后，原飞牛初始化 URL 404 替换为现官方页面，SMAPI GET 200。修改后 website 的 npm run docs:build 通过。
- 未覆盖项明确保留：本轮未重编译未修改的 Control Mod，未执行需要显式 opt-in 的真实 Steam/SMAPI/长期 Docker integration，也没有执行正式候选的 Web 升级/回滚矩阵。此次是内部孤立代码与构建减负，不把 build 或本次 API 冒烟当作正式发布证明。

## 保全、恢复与收尾

Docker 曾报告 containerd metadata 文件系统只读。经用户授权重启后，WSL 卡住；Windows 恢复后 Docker 29.5.3 与 WslService 就绪，核对镜像、脱敏挂载投影和容器早于审计的创建时间后恢复原游戏容器。stardew-2-server-1、anxi-install-test-20260905、anxi-panel 均 running/healthy，两个 Panel /health 与数据库状态正常；另外三台原停止容器保持停止。没有重置 Docker、注销发行版、删除虚拟盘或业务卷。

任务一个临时容器、三个测试卷、五个镜像标签已清理；八条本任务独占上下文缓存以精确 ID 和实时归属校验回收。原六容器、23 卷保留，任务 4319/18519 监听清零。三个外部零字节 IPC 备份目录保留为恢复证据，不作为磁盘收益；没有需要本次强删的业务数据或共享资源。

原有工作通过初始哈希、归档、任务增量和 index 比对保全；最终差异/编码检查记录于 preservation-check.json 与 diff-check.log。22 个临时辅助脚本已按精确清单校验后归档至 output 的 scripts 子目录，并从 .agents 移除。保留候选的用途/恢复缺口已经登记，后续不因目录名、修改时间或 Git ignore 再次误删。

最终保全复核中，860 个未由本任务修改的基线文件哈希相同、非任务路径异常差异为 0、index 字节不变；本任务修改的文件以 task-only.diff 对照初始工作区审查。证据不足的 candidate.tar、人工设计脚本关联素材及匿名恢复卷继续保留，未触发任何永久业务数据删除或共享资源审批。

## 2026-09-19 续查：历史缓存与归档

- 本轮重新建立 `output/project-load-20260919-followup/baseline.json`、文件身份清单和 index 快照；只修改本节及对应后端、镜像、路线与接手说明，5 份原文档以逐项 SHA-256 校验的 `docs-before.zip` 保存。业务源码和先前未提交工作继续保留。
- 重启后已无宿主 Panel/Go 进程使用项目缓存，Docker 无目标测试/cache bind。`go version -m` 识别出 5 份 Windows amd64 `cmd/panel` 可执行缓存；保留最新 `22/2219…/panel.exe`，4 份较旧的 `29/29554…`、`5f/5fcad…`、`b3/b3394…`、`fe/fe26…` 精确路径和摘要在 `compression-manifest.json`。
- 旧二进制没有足够的精确源码版本证明，因此先保留完整字节：71,020,544 B → `retired-panel-cache.tar.xz` 21,999,800 B。逐文件解压 SHA-256 相等、dry-run、进程/挂载/路径/reparse/文件身份/独占打开复核后，仅移除四个原文件；净减少 49,020,744 B，仍能从 archive 的原相对路径恢复。其余 Go 编译缓存及开发依赖保留。
- 镜像归档确认包含 `stardew-server-anxi-panel:0.3.14-rc`、声明 revision `07913e823880` 与 7 层；label 不是不可变候选证明。gzip 仅从 63,069,184 B 降到 62,779,029 B（0.46%），收益不足以承担路径/恢复维护，故保留原 tar 并清除本任务 gzip 探针。存档导入现场、Mod/备份、Steam 授权和个人目录继续保持。
- 清理后以保留的项目 `.gocache`、`GOPROXY=off`、本机 Go 1.26.4 执行 `go -C backend build -o NUL ./cmd/panel` 通过；构建新增 16 个可复用缓存文件 21,657,050 B，`.gocache` 实测 349,707,142 → 300,343,648 B。旧二进制 archive 与本轮审计证据另计，不把 49.02 MB 当作全目录或宿主磁盘净收益。此次未改产品代码，不重复前轮全量测试，也不将 Windows build 作为 Linux 权限/发布门禁证明。
- 首次 `git fsck --full --no-progress` 退出 0，无损坏，记录 16,815 个 dangling 对象。用户随后明确追加授权“允许保留全部对象的重新打包”；执行 `git -c gc.auto=0 repack -a -d --cruft --cruft-expiration=never --threads=2 --window-memory=64m`，保留所有不可达对象到 cruft pack。`.git` 2,045,047,302 → 1,190,609,173 B，减少 854,438,129 B；78,947 个对象 ID 集合、HEAD、所有 refs/reflog、index/config/gc.pid 摘要及 worktree 完全相同。第二次完整 fsck 退出 0，两次 dangling 结果集合相同。没有 reflog 过期、历史重写、提交或推送。
- 保全检查识别出另一任务在错题本新增的一条公开资料检索记录；按插入范围验证其余原文哈希后单独登记，未回退或归到本轮修改。其他非任务文件、既有删除和 index 均按续查基线核验；执行类保护断言正确阻止了把并发记录误报为本轮覆盖。
- 最终计量：扣除本轮约 27.36 MB 的归档/审计证据、21.66 MB 的构建新增缓存及文档增量后，含 Git 的已测范围逻辑占用净减少约 876.4 MB；具体逐项字节见 `metrics.json`（不含其自身）。硬链接去重值与逻辑值相同。本轮源码/素材、前端产物和运行镜像没有新增变化；没有测量宿主实际分配变化或缩容 VHDX。最终 877 个其他基线文件哈希相同、1 份并发错题本记录完整保留、51 个原有删除保持，index 字节相同；原 6 容器状态不变且 3 个运行容器健康。无本任务 Git 进程、临时 pack、gzip 探针或预览服务残留。

本轮所有辅助脚本、候选表、dry-run/执行记录和逐项恢复摘要保留在 `output/project-load-20260919-followup`。`compact.py verify` 可只读校验旧缓存 archive 和原始镜像 tar；恢复旧缓存时仅按 manifest 的精确成员恢复到不存在的原路径，禁止覆盖新的缓存文件。构建产物、发布镜像及应用行为没有新增变更。

轻量防复发措施已落地：npm ci 锁定安装、嵌套生成目录的 Docker 排除、带 dry-run/路径/哈希/归属断言的本次清理脚本，以及压缩输出的最终盘点。没有新增依赖、重型 CI 工具或发布绕过入口。
