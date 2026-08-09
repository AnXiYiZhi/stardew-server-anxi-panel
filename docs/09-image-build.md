# v0.4.9 官网展示文档发布（2026-08-09，已发布）

- 范围仅为 `website/docs/index.md` 与 `website/docs/changelog.md`：首页 release、版本卡和摘要切换为 v0.4.9，更新日志置顶加入升级检测、三类一键修复、最多三次、失败回滚、未知状态脱敏支持包、重启续跑和 auth 保留说明。没有修改 Panel 镜像、API、tag、Release、主题组件、导航或依赖。
- 与 Pages 一致的 Node 20 Alpine 隔离副本执行全新 `npm ci` 和 `npm run docs:build` 成功，VitePress 1.6.4 构建本体 3.12 秒；源码不存在仍把 v0.4.8 标成最新的 frontmatter、版本卡或 changelog 标题。`npm audit` 报告既有 2 moderate/3 high，本次未改 lockfile、未自动升级依赖，另行评估而不混入文档发布。
- 应用内 Browser 默认桌面验证首页 `v0.4.9` 与摘要可见、标题/URL 正确、无 framework overlay/console warn/error；点击“查看本次更新”实际进入 `/stardew-server-anxi-panel/changelog.html`，三类修复、脱敏支持包和 v0.4.8 历史均存在。390×844 首页和 changelog 的 document/body `scrollWidth == clientWidth == 375`，无横向溢出，普通视口截图渲染正常。
- 提交 `705aa8063b14d6f65e776cc2a39508bbe95ee0f7` 推送 `main` 后，Pages workflow `31305028853` 与 compatibility workflow `31305028888` 均为 `completed/success`。线上 `https://anxiyizhi.github.io/stardew-server-anxi-panel/` 默认桌面和 390×844 手机再次验证：`v0.4.9` 首页卡/摘要可见，“查看本次更新”进入 `/changelog.html`，三类修复、支持包及 v0.4.8 历史存在，document/body 无横向溢出，console warn/error=0。任务预览容器先核对 owner 后精确删除，18110 无 listener；Browser 标签已 finalize，未遗留测试 volume/network，也未执行 prune。

# v0.4.9 发布门禁：升级故障目录与一键修复（2026-08-09，已发布）

- 目标正式版本为 `v0.4.9`；上一正式版为 `v0.4.8`。本版修改跨 Panel 版本的 Junimo runtime 检测、恢复、重试和升级闭环，因此除 `v0.4.8 → v0.4.9` 真实 Web 一键升级外，还按 runtime manifest 的 `minimumPanelVersion=0.3.2` 执行 `v0.3.2 → v0.4.9` 代表老版本直升。
- 上述门禁均已完成。annotated tag `v0.4.9` 固定指向 `6f3e4a28f6c5f983f0f891079fb0b7478bd5c1a9`；Release workflow `31299979401`、该提交的 compatibility workflow `31298881696` 均成功，GitHub Release、三个 registry 的 `0.4.9/latest` 和四项 Release 脚本资产已经正式发布。

## 变更清单、受影响链路与故障矩阵

- 本版新增由后端统一生成的 `repairPlan`。链路为 `GET 只读检测 → 页面/脚本展示 detection、method、steps、buttonLabel → 管理员确认 → POST 锁内重新检测 → 受限修复 → 完整 dry-run/保存/备份 → 新 apply → 目标验收或自动回滚`。数据库、实例部署格式、存档、Mod、game-data 与 steam-session 公开格式不变。
- 按钮直接描述修复方法：回滚失败为“修复：恢复旧版后升级”，可信旧候选为“修复：规范配置并升级”，安全回滚后的可重试故障为“修复：重新预检并升级”。不能证明安全时不提供自动修复，改为“保留现场并导出支持包”；任务仍在恢复或矩阵暂不安全时只显示等待。
- `deploy/repair-junimo-upgrade.sh` 的 trap cleanup 同时标注 ShellCheck `SC2317/SC2329`，兼容正式工作流可能使用的 0.10/0.11 规则集；只改变静态分析声明，不改变临时目录权限、清理动作或 API 行为。

| 故障/边界 | 只读检测依据 | 页面按钮与处理方法 | 验证/安全终态 |
| --- | --- | --- | --- |
| `rollback_failed` | status/manifest 的实例、apply ID、版本对、project、资源名、原 image IDs 与备份 SHA-256 全部一致 | 修复：恢复旧版后升级 | 幂等恢复并验收旧版，重新预检/备份后新事务升级；材料漂移时零 mutation |
| 历史候选配置 | 主镜像可信、`IMAGE_VERSION` 与 server tag 一致、候选仅来自固定当前/历史可信仓库 | 修复：规范配置并升级 | 0600 私有备份、原子写入、复检；失败恢复原 `.env` |
| `failed_rolled_back` 安全重试 | 旧版已经验收、当前仍是同一旧版本对、目标仍是当前推荐且可升级 | 修复：重新预检并升级 | 不重放旧事务；按错误码重查重启/磁盘文件/Docker/网络健康，以新 apply ID 重试并继续自动回滚 |
| 状态 JSON 损坏/不可读 | status 解析失败或缺关键字段 | 保留现场并导出支持包 | 不覆盖状态，不猜事务阶段 |
| 清单不匹配/缺失 | instance/apply/target/project/资源名不能精确绑定 | 保留现场并导出支持包 | 不猜旧镜像、卷或恢复路径 |
| 私有材料缺失/篡改/symlink | ordinary-file 与 SHA-256 校验失败 | 保留现场并导出支持包 | 在任何 Docker mutation 前拒绝 |
| 自定义镜像/主 tag 歧义/未知候选/不可读 `.env` | `unsupported/custom_images` 或 `invalid_config/*` | 保留现场并导出支持包 | 保持用户配置，要求人工确认来源，不自动覆盖 |
| 同一目标连续三次失败 | `attempts >= maxAttempts` | 保留现场并导出支持包 | 停止循环重试，保留全部证据 |
| 升级/启动恢复仍是非终态 | persisted phase 非 terminal | 等待自动恢复（禁用） | 不并发创建第二事务，依赖 WAL 自动续跑/回滚 |
| 推荐矩阵撤回/不推荐 | `withdrawn` / `not_recommended` | 等待安全版本（禁用） | 不修改实例，等待新的 tested manifest |
| 正常可升级/已最新/未安装 | `update_available` / `up_to_date` / `not_installed` 且无已知故障 | 普通升级、无修复按钮 | 不误把正常状态归类为故障 |

## 发布故障维度

- 正常路径：三类 repair 都必须走同一持久化 runner；当前 Go 覆盖 rollback、legacy config 和 safe retry 成功，浏览器实际完成 rollback 按钮链路。
- 边界输入、权限与安全：GET 计划不含私有路径/凭据；POST 只接受管理员严格 `{"confirm":true}`，匿名 401，调用方注入 `strategy` 为 400；自定义/歧义/材料漂移 fail closed。
- 网络超时/断流：下载、digest 和目标 health 失败在 `failed_rolled_back` 后可重新预检；新事务仍使用既有有界网络等待和自动回滚。没有把“重试”改成跳过摘要或健康检查。
- 部分成功、幂等与中断恢复：rollback 继续使用 schema 3 write-ahead 与幂等材料；safe retry 使用全新 apply ID。`resuming_upgrade`、新 manifest mutation 前中断与 Panel restart 继续由既有专项测试覆盖；浏览器关闭不会丢任务。
- 失败回滚与数据完整性：修复后 target 失败仍回滚旧版；活动存档重新保存并整档备份；不删除 game-data、用户 Mod、steam-session 或非目标容器/volume。三次失败停止自动操作。
- 资源清理：自动代码只按校验后的 apply/project/volume 所有权清理。本机验证也只清理 `repair-catalog-20260809`/`repairplan` 标签的任务资源，未运行任何 prune。
- 不适用项：本版没有数据库 migration、部署格式或长期数据格式变化；本目录只处理 Junimo server/auth/Control，不宣称覆盖 SMAPI staging、Panel updater 或未来 game/SDK apply。

## 当前验证证据

- 后端最终全量 `go test ./... -count=1`、`go vet ./...`、`go build ./...` 通过；专项新增 safe retry 成功、活动事务优先等待，以及损坏状态、自定义镜像、恢复材料篡改和三次耗尽 fail-closed。正式发布无缓存 test/vet/build 复验合计 50.88 秒通过；`go test -tags=integration ./internal/docker -count=1` 用时 10.697 秒通过。
- 前端 12 项状态测试和 production build 在 Node 24 Linux 隔离依赖卷中用时 17.88 秒通过；Bash 语法及三项脚本功能测试用时 2.53 秒通过，ShellCheck 0.11.0 精确使用 workflow 清单通过。脚本 `check` 已覆盖 rollback/config/safe retry 的按钮和方法输出。
- 兼容矩阵 validate、`0.4.9` panel version 契约和 19 项 Python 测试通过；真实远程制品固定来源/摘要校验用时 93.34 秒通过，多个可选镜像源不可达、SMAPI 候选 SSL EOF 和 Git traceability TLS 重试均由既有回退吸收。Linux `golang:1.25-alpine` 真实下载 41,889,142 字节 SMAPI 包并验证摘要/权限/无 `.part`，测试本体 5.84 秒通过。
- updater Docker integration 的独立 Compose 成功链 14.60 秒、目标 unhealthy 自动回滚链 22.84 秒通过，runtime Docker integration 10.697 秒和候选 helper reconcile 11.68 秒通过。正式精确候选随后完成官方 `v0.4.8 → v0.4.9`（16.78 秒）及 runtime 支持下限 `v0.3.2 → v0.4.9`（16.65 秒）真实直升，两条链均核对目标 health/version、持久状态和非目标资源保护。
- Docker Desktop Linux 29.5.3 最终构建隔离候选 `stardew-anxi-panel:repair-catalog-final-20260809`：version=`0.4.9-repair-catalog.dev.3`、revision=`5be8664b19e487819412e455201a83cdc86a4ff7-dirty-repair-catalog`、created=`2026-08-09T05:57:53Z`，image ID=`sha256:d4376de1fcaf1ba083c3ba28b291d4bf324a0d60860ef7aa9ee551d0c19e38dc`。任务专属容器/网络/volume/`127.0.0.1:18097` 验证 Docker health=`healthy`、`/health` 与 `/api/version` 精确匹配；初始化后重启仍保持 `setup.initialized=true` 和相同版本。
- 候选真实接口验证匿名 repair=401、带调用方 `strategy`=400、严格确认对不存在实例=404；测试管理员密码只存在内存和任务专属临时数据卷。镜像内 `/app/repair-junimo-upgrade.sh` 与源码 SHA-256 均为 `910d9df8dcf67818e1b3e7c5a7591bce33e7defe41fcc615d00327e2f0955042`。
- 候选真实支持包下载返回 200/`application/zip`，条目为 `version.json`、`health.json`、`instance-state.json`、`junimo-update.json`、`jobs.json`、`audit-logs.json`、`compose-ps.json`、脱敏 `docker-compose.yml` 和 `server-logs.txt`；新增条目包含 `inspection/repairPlan`，未出现 recovery 路径或原备份文件名。专项单测另以 apply 状态伪密码证明整项 JSON 会脱敏。
- 同一 Docker Desktop 的只读源码 Vite QA 容器由 Codex Browser 实际点击“修复：恢复旧版后升级”，事件为 `repair:POST,repair-apply:GET,repair-apply:GET`，最终显示升级成功；另行验证配置修复、安全重试、未知故障导出和活动事务等待按钮，console warn/error 均为 0。
- 每轮验收后先核对 owner label，再精确删除本任务容器、volume、network 和本地开发候选 tag；最终 `18096/18097` 无 listener，任务资源查询为空。没有执行 prune，也没有触碰其它容器、volume、network 或镜像。
- 干净 `main` 的最终候选 `stardew-anxi-panel:0.4.9-candidate-6f3e4a2` 使用 version=`0.4.9`、完整 revision=`6f3e4a28f6c5f983f0f891079fb0b7478bd5c1a9`、created=`2026-08-09T06:26:46Z` 构建，image ID=`sha256:333b5f5e3d44f528bb7d4925475d15293a8cb4d7a4680b3a517a6da0240b0bcf`。fresh health/version/setup/restart/support ZIP 均通过，镜像内 repair 脚本与发布源码摘要同为 `4f3c666770b6be77ed51895264f47c940b066d61386b66b3653a858e8929b4c2`。
- 隔离 DinD 中用官方 `v0.4.8` Panel、受控 HTTPS Release 和 TLS registry 完成真实 Web 一键更新。unhealthy 目标事务 `1b37b0584eba` 在 143.21 秒收敛为 `failed_rolled_back` 并恢复官方 `0.4.8`；健康目标事务 `7da04fbbe918` 在 8.95 秒到达 `succeeded`。升级后重启仍保留终态，管理员、初始化、默认实例、审计以及 panel/Mod/save/backup 哨兵均保持；非目标游戏容器 ID、StartedAt 和内容摘要不变。新 Panel 上支持包包含脱敏 `junimo-update.json`，repair 严格请求注入被 400 拒绝。
- 正式 workflow 产出的 Docker Hub、阿里云 ACR、GHCR 六个 `0.4.9/latest` 引用统一为 OCI index digest `sha256:e8fa5386b17d778612365bfa419b5ad5e2f447bb557856580efe262fea6f505f`，amd64 manifest 为 `sha256:fc369eaa9995a35b52814dda52db4b98912eb91765c1f78c26ec3f6d649f4281`；OCI version=`0.4.9`、revision=`6f3e4a28f6c5`、created=`2026-08-09T06:59:39Z`。六个引用全部实际回拉；三仓精确版本分别以隔离容器通过 Docker health、`/health`、`/api/version`、fresh setup 和伪登录 404 冒烟。GHCR/Hub 查询遇到的短暂 TLS EOF/handshake timeout 仅由有界重试恢复，未关闭 TLS、摘要或签名校验。
- GitHub Release `Stardew Server Anxi Panel 0.4.9` 于 `2026-08-09T07:01:13Z` 发布，四项资产摘要分别为 `migrate-fnos.sh=90510768d6636917fb7f15937a7dce34c34974dd8c9af5451030560eca57cbfd`、`repair-junimo-0.3.5.sh=38d06d09e5c17db3145ec3b938f4d6844d1f2f058c73fa5bc72c804335eee47b`、`repair-junimo-upgrade.sh=4f3c666770b6be77ed51895264f47c940b066d61386b66b3653a858e8929b4c2`、`run.sh=8f0040c11661f2e3f4060c66bf8ba205a33aa46fc65e3dec7cbf15b864c7387a`。发布后 smoke 的三个容器、三个数据卷和网络先核对 `anxi.test.owner=v049post` 再精确删除，18101–18103 无 listener，未执行 prune。

# RUNTIME-UPDATE-DIAGNOSE-REPAIR-2 Docker/发布门禁（2026-08-09，未发布）

## 变更与检测/修复矩阵

- 本版把原“一键安全恢复”升级为持久化“检测、修复并升级”。面板按钮和 `repair-junimo-upgrade.sh` 均先从后端读取 apply/stack 状态，只向严格 repair API 提交确认；后端拥有 detector、修复计划、恢复材料和目标版本权威。
- 本次生产问题的检测证据为：配置 tag 和运行容器 image ID 表明 server/auth 已匹配目标，Panel-owned Control 运行版本或 DLL 内容不匹配；apply change plan 因此只同步 Control/重建 server，未变化 auth 不 stop、不重建、不做 session 快照或网络 readiness。该判断现在由 `InspectManagedRuntimeStack + runtimeUpdateApplyPreflight` 每次实时重算，不依赖人工看日志。

| 故障分类 | 自动检测条件 | 修复动作 | 修复后必须通过 |
| --- | --- | --- | --- |
| 回滚事务失败 | `rollback_failed`；status/manifest 的实例、apply ID、版本对、project、目标 image/digest 与事务资源精确一致；恢复文件为普通非 symlink 且 SHA-256 一致 | 按 immutable 原 image IDs 幂等恢复 env/Compose/Control/Junimo/auth volume/server 与原运行状态；精确清理本事务资源 | 原 server 版本/health/info/Control 契约、auth（若变化）、原运行/停止状态 |
| 历史可信候选配置 | 主 server/auth 均属可信仓库；`IMAGE_VERSION` 与主 server tag 一致；候选项只来自当前/历史可信仓库且仅存在仓库或 tag 漂移 | 0600 私有备份原 `.env`；原子规范化两个候选列表；失败恢复原文件 | `InspectRuntimeStack` 只能得到 `update_available` 或 `up_to_date` |
| 未知或不可信状态 | 自定义主镜像、未知候选、损坏 status/manifest、摘要漂移、未知 Compose/project、缺失认证卷 | 不修复、不覆盖、不猜测；保留现场 | 返回稳定 409/失败码并可导出支持包 |
| 修复后升级 | 上述修复验收已通过 | 重新执行普通完整 dry-run；有活动存档时通告、保存、整档备份；创建新 apply ID 并按实时 change plan 升级 | manifest/Docker/Compose/current image/volume/target digest/Compose override、目标 health/API/SMAPI/Control、原运行状态 |
| 修复后再次失败 | dry-run、保存/备份或新 target 验收失败 | mutation 前停止并保留旧版，或 mutation 后走普通自动回滚；只有新回滚也失败时再次显示 repair | `failed_rolled_back` 必须是旧版安全且脚本退出非零；`succeeded` 才是最终成功 |
| Panel 中断 | `resuming_upgrade` 或 `resumeAfterRepair=true` | 旧事务材料仍在则重复幂等恢复；已清理则重跑检测/dry-run；新事务无 mutation 则重启 apply；有 intent 则普通回滚 | 续跑不需要浏览器存活，repair source 与尝试次数保留，最多三次 |

## 本轮故障与发布矩阵

- 正常路径：可信旧候选直接点击 repair；partial rollback 首次失败后点击 repair；两者都必须经新 dry-run 和新 apply 达到 `succeeded`。
- 边界/安全：严格 body/管理员权限、材料篡改零 Docker mutation、自定义镜像拒绝、三次上限、未知状态 fail closed；脚本只接受 Panel URL/实例/管理员凭据文件，不接收镜像、路径、命令或策略。
- 网络/断流：修复后 dry-run 拉取失败不得开始 mutation；新 target 网络/health 失败必须自动回滚；Panel 在修复清理后和新 manifest mutation 前重启必须自动续跑。
- 数据/恢复：存在活动存档时重新保存和整档备份；steam-session、game-data、用户 Mod、SQLite、非目标容器/volume 保留；清理只按 source/new apply 精确所有权。
- 资源/不适用：本入口不操作 Panel updater、SMAPI staging 或 game/SDK staging；这些链路不能因本测试通过而标记完成。

## 当前验证状态

- 已通过专项 Go：rollback 修复后升级成功、修复后 target 再失败安全回滚、历史可信候选直接检测/修复/升级、`resuming_upgrade` 在旧恢复目录清理后重启继续、新 retry manifest 已持久化但 mutation 尚未开始时重启继续同一 apply、诊断 checks/repair source 保留、材料篡改拒绝。
- 2026-08-09 全量代码门禁通过；补完 no-mutation 中断窗口后再次完整执行后端 test/vet/build，聚合用时 67.9 秒，`stardew_junimo` 56.770 秒、Web 40.283 秒。前端 12 项状态测试与 TypeScript production build 合计 18.4 秒；Git Bash 5.2 的两个脚本 `bash -n`/功能测试 8.2 秒；ShellCheck 0.10.0 官方发布脚本清单通过。兼容矩阵由 Python 3.12.13 验证 manifest 并通过 19 项测试；Docker integration `go test -tags=integration ./internal/docker -count=1` 14.226 秒。
- 其余不依赖真实 Junimo 故障夹具的正式门禁也已补齐：`test_run_sh_update.sh` 2.8 秒通过；panel version 契约和远程兼容产物验证 78.7 秒通过（两个可选 server mirror、一个可选 auth mirror及一次下载候选 SSL EOF 仅产生预期 warning，required 来源最终成功）；Linux `golang:1.25-alpine` 从真实上游下载 41,889,142 字节 SMAPI 包并验证摘要、`0600` 与无 `.part`，60.85 秒通过。Windows 宿主曾因 NTFS 权限语义报告 `0666`，未修改断言，已按错题本切回 Linux 复验。updater Docker integration 的隔离成功链 15.27 秒、目标失败自动回滚链 25.24 秒通过；真实旧版到候选及新 Panel helper reconcile 因未提供正式精确镜像环境变量而明确 skip，继续属于发布阻塞。Node 20 + Git 的 VitePress build 6.67 秒通过。
- 补完 no-mutation 中断窗口后，Docker Desktop Linux 29.5.3 重新构建最终隔离候选 `stardew-anxi-panel:diag-repair-final-20260809`：version=`0.4.9-diagnose-repair.dev.2`、revision=`91e7497fcfc101fc85f9cead7ab938f75cd0e824-dirty`、build date=`2026-08-08T18:09:43Z`，image ID/manifest digest=`sha256:c6fa33d732cee389cf57c4a8c550baf2f598b53b9bf3ffb9dba9d460bf1d270f`，构建 33.4 秒。任务专属容器、network、volume 与 loopback 端口验证 Docker health=`healthy`、`/health=ok`、`/api/version` 精确匹配；镜像内 `/app/repair-junimo-upgrade.sh` `bash -n` 通过且与源码 SHA-256 同为 `ae80598f66a6c13ab071e9e453becd47c8327a2ab134703eac690ccd0da4a088`。容器重启后 `/health`、精确 version 与 data volume 中 `setup.initialized=true` 仍保持。前一版 `.dev.1` 只作为中途证据，已由本最终候选替代。
- 候选真实 HTTP 接口验证：未初始化时 repair 写入口被 setup gate 以 503 拒绝；初始化任务专属管理员后，携带 caller-controlled `strategy` 的请求被 strict body 以 400 拒绝，严格 `{"confirm":true}` 才进入实例查询并因测试实例不存在返回 404。测试凭据只存在任务专属临时 data volume，未写日志、镜像层、提交或文档。
- UI 在同一 Docker Desktop 的任务专属 `node:22-alpine` QA 容器运行，源码只读 bind、`node_modules` 独立 volume、端口 18094；无代理 headless Chrome 原生 DevTools 实际点击“诊断”与“检测、修复并升级”，连续观察 `rollback_failed → rolling_back → resuming_upgrade → succeeded`，并展开核对 `repair_failure_state`、`repair_manifest`、`repair_materials`、`repair_original_runtime`、`repair_upgrade_preflight`、`change_plan`、`修复源 qa-junimo-apply`，console error/warn 为 0。Codex Browser 插件对本机 URL 返回 `ERR_BLOCKED_BY_CLIENT`，按前端测试技能记录后才改用不安装依赖的本机 Chrome；正式候选镜像按设计不包含开发 QA harness，真实 health/version/API 仍由上一个容器验证。
- 每轮验证完成后均重新执行 `docker info`，逐项核对 owner label 和精确资源集合。中途候选/QA 清理了 2 个容器、1 个 network、5 个临时 volume；最终 `.dev.2` 又精确清理 1 个容器、1 个 network、1 个 data volume 和本地候选 tag。18093、18094、18095、19223 均无 listener，两个 owner filter 均返回空。截图留在任务专属系统临时目录用于本次交接；未执行任何 prune，未触碰其它容器、镜像或 volume。
- 当前仍未创建/移动 tag，未更新 `latest`，未推送正式镜像或 GitHub Release。发布硬阻塞仍是缺少不含用户存档/长期凭据、可制造真实 `rollback_failed` 的完整 Junimo 游戏夹具；因此本轮不能把故障注入编排测试、真实 Docker 原语和 mock UI 串联冒充“最新正式版/最老受影响版 → 候选”的真实 Panel 一键升级及修复 E2E，也不能降低门禁提前打 tag。

# RUNTIME-UPDATE-WAL-REPAIR-1 Docker/发布门禁（2026-08-08，未发布）

## 变更范围与受影响链路

- 本版修改 Panel 在旧正式版升级后执行 required Junimo runtime 同步时的事务恢复：manifest schema 3 write-ahead intent、跨 Panel 版本的持久化事务校验、恢复材料 SHA-256、终态先提交后清理、幂等 rollback retry、管理员 repair API/页面和 `repair-junimo-upgrade.sh`。
- 受影响链路为 `Panel 启动/全栈协调 → Junimo dry-run/apply → server/auth/Control 差异升级 → target verification → rollback/restart recovery → terminal cleanup`。数据库 schema、存档、game-data、Mod 用户目录、Panel 自更新 helper 和 SMAPI staging 格式没有改变。
- `deploy/repair-junimo-upgrade.sh` 已复制到候选镜像 `/app/repair-junimo-upgrade.sh`，并作为后续 GitHub Release asset。脚本只登录 Panel 并调用严格 repair API，不直接挂载/调用 Docker；因此 Panel API 完全不可达时不能用于修复 Panel 容器自身。

## 故障矩阵

| 类别 | 必须成立的行为 | 本版机制/验证状态 |
| --- | --- | --- |
| 正常路径 | stopped/running、Control-only 与真正 auth 变更都只操作差异组件，目标健康后恢复原运行状态 | 既有 stopped/running Docker Desktop Control-only 真机通过；本版候选 Docker 回归见下方当前证据 |
| 边界输入 | apply ID、project、snapshot 名、target pair 和原/目标版本必须精确；旧事务不因新 Panel 推荐版本变化失效 | `apply_[a-f0-9]{24}`、事务 `Target == Selected`、持久化 trusted candidates、image digest/ID 检查；跨版本事务与漂移拒绝单测通过 |
| 权限与安全 | 只有管理员可 repair；请求不得注入路径、镜像、命令或策略；恢复材料不可被 symlink/篡改替换 | 401/403、严格 `{"confirm":true}`、schema 3 SHA-256/regular-file 校验、篡改材料零 Docker mutation 单测通过；脚本密码不进入 argv/日志 |
| 网络超时/断流 | 镜像拉取、auth readiness、server 冷启动和脚本轮询有界；网络失败触发原版本回滚，不放宽 digest/health | auth 10 分钟、server 20 分钟、stop 10 分钟、job 2 小时；脚本 connect/max/总等待均有界；真实网络异常仍需按发布候选矩阵重跑 |
| 部分成功后的重试与幂等 | 回滚在 Mod 已恢复、配置已恢复或容器已重建后失败，重复执行不得丢原材料或重复选择目标 | Junimo restore 识别 durable partial state；repair 只重试同一 manifest，最多三次；认证卷恢复注入首次失败、第二次成功单测通过 |
| 进程/容器中断后的恢复 | 每个 mutation 调用可能已到达 Docker 但完成标记未写时仍可恢复；纯备份阶段重启不得要求人工 | stop/Control/snapshot/Junimo/config/auth/server write-ahead intents；备份阶段无 manifest 自动 `failed_rolled_back`；schema 3 intent 重启单测通过 |
| 失败回滚 | target 验收失败自动回滚；rollback_failed 保留材料并可一键安全恢复；材料不可信时拒绝猜测 | 原 rollback 路径保留；repair API/页面/脚本新增；摘要漂移返回 `recovery_material_invalid`，三次失败后停止自动操作 |
| 数据完整性 | 原 `.env`、Compose、Control、server/auth image IDs、steam-session 和原运行状态必须精确恢复；不触碰存档/game-data | 四类文件备份/摘要、immutable image ID pin/volume helper、原状态验收；真实 volume 与候选链路仍需下方 Docker 门禁给证据 |
| 资源清理 | 成功/回滚终态未持久化前不得删 snapshot、旧 image 或 recovery；重启只清理本 apply 精确拥有的资源 | terminal status 先写；snapshot create intent 覆盖 daemon 已创建但返回前崩溃窗口；terminal 启动清理和精确卷名单测通过 |
| 不适用 | 本版无数据库迁移、部署格式变化或 GAME_DATA_VOLUME 切换 | 数据库迁移/SMAPI staging/game+SDK apply 故障不由本 repair 处理；其独立风险见 `docs/07-later-optimizations.md` |

## 旧版一键安全恢复用法（已由上方闭环取代）

1. 先下载对应正式 Release 的 `repair-junimo-upgrade.sh` 到 NAS/Linux 宿主机并检查文件；不要把脚本粘进 Panel 容器终端，也不要给它 Docker socket。
2. 只检查、不修改：`PANEL_URL=http://127.0.0.1:8090 bash repair-junimo-upgrade.sh check`。
3. repair：交互终端直接运行 `PANEL_URL=http://127.0.0.1:8090 bash repair-junimo-upgrade.sh repair`；自动化环境把管理员密码放入权限受控的普通文件，再设置 `PANEL_PASSWORD_FILE=/path/to/file`。新脚本会同时识别 `rollback_failed` 与后端声明的 `repairable` 配置，并等待最终 `succeeded`；`failed_rolled_back` 退出非零，不会自行执行 Compose/volume 命令。
4. 若返回 `recovery_material_invalid/recovery_state_uncertain` 或三次耗尽，停止操作并保留实例目录和任务精确卷；不能通过改 JSON、改 tag 或 `volume prune` 绕过。

## 当前验证证据与发布状态

- Go 专项目前通过：部分 rollback 首次认证卷恢复失败后 repair 幂等成功；`original.env` 摘要篡改在零 Docker mutation 下拒绝；Control write-ahead intent 重启恢复；备份阶段无 manifest 安全终止；事务使用持久化候选跨 Panel 版本有效；snapshot create intent 保留精确清理所有权。
- 2026-08-09 收口全量门禁通过：`go test ./... -count=1` 51.6 秒、`go vet ./...` 5.5 秒、`go build ./...` 8.3 秒；前端 12 项状态/响应式测试 8.2 秒、TypeScript + production build 10.5 秒；Bash 5.2 `bash -n` + 脚本功能测试 2.8 秒、ShellCheck 0.10.0 2.3 秒。Web 权限测试覆盖 repair 的匿名/普通用户拒绝，strict confirmation 单测拒绝 image/tag/service/command 注入。
- Docker Desktop 29.5.3 的真实低层门禁 11.886 秒通过：steam-session volume 创建/克隆/恢复、精确旧 image ID 与容器引用保护、无 Node 镜像 inspect/auth probe、SMAPI staging 克隆/installer、server health 及 runtime/SMAPI 参数注入拒绝。测试只使用自身临时容器/卷，未触碰生产实例或长期 volume。
- 本地候选 `stardew-anxi-panel:wal-repair-20260808` 以 version=`0.4.9-wal-repair.dev.1`、revision=`4d8ea24cb141-dirty`、created=`2026-08-08T15:56:17Z` 构建；image ID/manifest digest=`sha256:96c6d1f00f616ae5a3bb9957d74df80d4f7d9d8c17e5b9b0f2df8c2e134ada9e`。前两次构建分别遇到 Go ZIP EOF 和 Alpine index SSL EOF，未生成 tag；官方 Alpine host-network 探针完整通过后第三次有界构建用时 36.6 秒成功，没有关闭 TLS、APK 签名或 Go 校验。
- 候选隔离运行验证 `/health`、`/api/version`，镜像内 `/app/repair-junimo-upgrade.sh` `bash -n`，HTTP repair 匿名 401、额外 image 字段 400、无可恢复事务 409；重启后容器 loopback `/health`/version 与持久 volume `setup.initialized=true` 均通过。宿主随机 published port 在 `docker restart` 后出现响应回程卡住，但容器日志和独立 loopback 请求均确认 Panel 正常，按环境问题记录于错题本，未误写为产品失败。嵌入脚本与源码 SHA-256 均为 `6e8140ffded33b7a241e4ff160b8f29111025cdf8eea965ee4273d828cff30c5`。
- 所有 `walrepair-*` 测试容器和 volume 已按 ownership label/精确名称删除，候选本地 tag 也在确认无容器引用后删除；未执行 prune。由于没有可公开复用、包含完整真实游戏但不含用户存档/凭据的专用夹具，本轮没有把真实 Junimo 游戏容器制造成 `rollback_failed` 再点击 repair；编排由故障注入单测覆盖，Docker volume/容器原语由上述真实 integration 覆盖。这一缺口在正式发布前仍是阻塞项，不能用本轮证据冒充完整旧版 Web 升级 E2E。
- 本次没有创建或移动 tag、没有更新 `latest`、没有推送正式镜像或 GitHub Release。若决定发版，仍必须从最新正式版和受本次跨版本恢复影响的代表老版本完成真实 Web 一键更新、目标失败回滚、Panel 中断恢复、升级后 repair/新功能验收及 AGENTS.md 全部发布门禁。

# RUNTIME-UPDATE-PRESERVE-AUTH-1 Docker 验证记录（2026-08-08，未发布）

- 变更范围：runtime apply 根据 current/target tag 与 image ID 选择 server/auth 操作；Control-only 只重启 server，保留健康 auth 容器和认证卷。auth 真正变化时等待由 90 秒扩大为 10 分钟。新增 `compose up --no-recreate` 与闭集 CPU shares 原地更新；公开 API、数据库、镜像清单和部署格式不变。
- 正常路径：Docker Desktop Linux 29.5.3 使用只读 `save-import-local-rich` 测试源（server `.125`、auth `.2`、Control `0.2.0`）复制出唯一 `anxirealupgrade*` project、game-data 与空 steam-session；`TestRequiredRuntimeRealControlUpgradeOptIn` stopped 42.38 秒、running 65.34 秒，合计 107.72 秒全部通过。运行态 auth ID 不变，Control 0.3.0 实载，server/auth shares=768/256，最终状态恢复。
- 边界/网络：单元故障注入让 auth readiness 始终报错，Control-only 仍成功，证明未变化 auth 不再依赖 Steam 出站链路。真实 auth 变更仍必须经过 readiness；10 分钟预算覆盖生产观察到约 400 秒的五轮退避。没有把未登录/ticket 当硬失败。
- 权限与安全：新增 Docker 方法继续限定合法 Compose project、`server/steam-auth` 服务闭集和固定 shares 768/256；不接受任意容器、权重或 shell。测试凭据在复制 `.env` 时清空，未读取源 steam-session 内容。
- 部分成功、重试与幂等：同 digest/tag 重跑不会重建 auth；原地 shares update 可重复执行。首轮运行态真机测试因保留容器仍为 shares=0 安全失败，加入原地 update 后同一完整用例重跑通过；未以修改断言掩盖缺口。
- 中断/恢复与回滚：恢复 manifest schema 2 持久化 change plan/snapshot 状态；schema 1 按旧全量事务保守恢复。Control-only 回滚只处理 server/Control/config，不停止、不恢复卷、不重建 auth；auth 变化仍保留快照和成对恢复材料。
- 数据完整性与资源清理：测试使用复制实例和克隆 game-data，源实例/卷只读，steam-session 为空且唯一；Go integration cleanup 只按精确 project/volume 删除本轮资源，不执行 prune、不触碰已有容器或长期卷。后端全量 `go test ./... -count=1` 用时 63.4 秒，vet/build 通过；发布候选门禁仍未执行。
- 发布状态：本节是代码和真机修复证据，不是正式候选记录；未打 tag、未推送镜像、未修改 latest。进入发布时仍须构建带精确 version/revision/date 的候选镜像，执行 Panel Web 一键升级、失败回滚、老版本矩阵、全量门禁及 tag 后三仓回拉。

# v0.4.8 发布记录：玩家 Mod 自报、比较与详情页（2026-08-07，已发布）

- 官网误发布修复：发布完成后的 `6f34b8a` 错误合入未授权的本地 `docs-portal-redesign` 隔离稿；2026-08-07 按用户要求以 tag commit `0c5e2c4` 的正式网站树恢复主题、导航、FAQ 与指南，只保留 v0.4.8 版本卡、changelog 和玩家手册。Node 24 production build、Pages `31152244079`、线上桌面/手机视觉及无溢出/console 检查通过；这不移动 `v0.4.8` tag，不重发镜像，也不回退 Panel 功能。

- 候选范围：Control `0.3.0` 在标准 IP/SMAPI peer 事件中按 `uniqueMultiplayerId` 采集客户端自报的游戏、SMAPI 与 Mod 清单，原子写入独立 `player-mod-contexts.json`；Go 后端以服务器进程实际 `options.json.loadedMods` 为基线提供 `GET /api/instances/:id/players/:uniqueMultiplayerId/mods`；桌面和移动端提供只读详情、四组比较、CJB 明示风险及 unavailable/pending/stale/error 状态。本版同时清理确认未引用的旧原型图和前端生成素材，并为真实 Panel 增加 `shell=mobile` 预览参数。未修改 Junimo/SMAPI 上游，不实现 Steam SDR 桥，不改变加入、认证、踢出、封禁或自动拦截流程。
- 受影响链路：`SMAPI peer event → Control 输入规范化/生命周期 → 原子 sidecar → stardew_junimo driver → Web 鉴权接口 → TypeScript 状态归一化 → 桌面/移动共享详情`。服务器比较集合只能来自当前运行进程的 `loadedMods`；磁盘 `ModInfo/syncKind` 只补名称和分类。`Pathoschild.SMAPI`、`JunimoHost.Server`、`AnXiYiZhi.StardewAnxiPanel.Control` 必须完全退出比较和统计；普通 `server_only` 可作信息展示但不得进入“玩家缺少 Mod”。
- 正常路径：新鲜隔离实例必须验证 host/玩家列表不受影响，真实 PC+SMAPI 通过 IP 加入并写出 `reported`，ID/名称/版本准确；接口和桌面/移动详情按“玩家额外安装 → 玩家缺少 Mod → 版本不同 → 匹配”展示。两条官方 CJB ID 必须只显示红色且带文字的“检测到 CJB 作弊”，玩家仍可正常加入且没有任何管理请求。
- 边界输入：覆盖 `mods:null` 与真实 `mods:[]` 的严格区别、pending 超时、unavailable、stale、断线/重连/重启、多玩家隔离、旧 ID 不串号、大小写重复 UniqueID、控制字符、超长 playerId/UniqueID/name/version、超过 1024 个不同 Mod 与超过 2048 个原始条目的整份拒绝。前端覆盖重复项、256 字名称、超长版本、空组、分批展开、280px 窄屏和无横向溢出。
- 权限与安全：详情接口继续要求登录并受实例可见性约束；未知实例/玩家/非法 ID 必须返回稳定错误且不能回退到其它玩家。客户端字符串和清单始终按不可信自报处理；CJB 仅按 `CJBok.CheatsMenu` / `CJBok.ItemSpawner` 大小写不敏感精确匹配，修改 manifest ID 可绕过提示。接口、Control 和 UI 不允许新增 kick、ban、拒绝连接、自动处罚或任意文件读取。
- 网络超时/断流：采集链路不新增外网请求；SMAPI peer 未在 10 秒内提供完整 context 时安全转为 `unavailable + mods:null`，不得断开玩家。Web 请求失败与比较基准不可用各自显示可重试错误，不得降级为“0 个 Mod”“完全一致”或“安全”。Steam SDR 和不提供标准 peer context 的客户端明确不在本阶段支持范围。
- 部分成功、重试与幂等：同一 playerId 重复连接/事件按规范化 ID 覆盖同一记录，重复 UniqueID 去重；超量或损坏整份报告不得保留部分清单。断线只转 stale 并保留最后报告，重连先 pending 再以新 `reportedAt` 替换；重复读取接口不得改 sidecar、玩家、Mod 或管理状态。Control 原子临时文件必须 replace 成功后才成为权威记录。
- 进程或容器中断后的恢复：Control 启动把上个进程遗留记录统一标为 stale；Panel/游戏容器重启后旧清单不得伪装成当前 reported。候选必须覆盖服务器重启、Panel 重启、连接后等待 context、断线与重连；sidecar 临时文件不能被当成权威数据，损坏/超限文件只导致 unavailable，不影响加入或玩家列表。
- 失败回滚：本功能只读且没有跨资源写事务，失败时产品回退是保持玩家连接、返回 unavailable/error 并保留最后有效 sidecar；不得通过踢出或清空玩家状态“修复”。Panel `v0.4.7 → 0.4.8` 更新目标 unhealthy/版本错误时仍必须由 updater 自动回滚到 `v0.4.7`，SQLite、实例目录、存档、Mod、游戏容器和旧 Control 保持可读且非目标容器 ID 不变。
- 数据完整性：嵌入 Control DLL、两份 manifest 与 `runtime_stack_manifest.json.controlMod.dllSha256` 必须一致；新鲜 C# 真实 game-data 编译、契约测试和实际加载日志同时通过。升级前后核对 `/health`、`/api/version`、OCI version/revision、SQLite 初始化/用户/实例、活动存档、Mod 集合、备份、审计、game container ID 和 player context schema；完整清单不得进入高频 `players.json` 或列表 API。
- 资源清理：所有 tag 前真实门禁使用唯一 `v048-player-mod-release-*` Compose project、容器、网络、端口、bind 目录和 volumes，创建前查重并打 ownership label；不得复用当前 `sap-player-mod-live-20260806` 实例、用户存档、长期凭据或既有唯一卷。结束时只按精确名称和 label 清理本轮资源，禁止 prune；本机真实游戏安装只允许作为只读源复制到隔离环境。
- 升级矩阵：使用正式 `v0.4.7` 和本地精确 `0.4.8` 候选在隔离 DinD/Compose 中执行 Web 更新检查、dry-run、管理员确认、apply、预期断线重连和终态恢复；注入 unhealthy 目标验证自动回滚。成功升级得到的新 Panel 必须再次验证玩家 Mod 静态路由、API、Control manifest/hash、旧长期数据保留和新功能真实/受控 fixture 主路径，不能以 fresh 安装替代。
- 真机矩阵与限制：已存在的隔离证据只有 PC+SMAPI 4.5.2/游戏 1.6.15 标准 IP 加入、真实三个 Mod、断线、重连和服务器重启；本次候选必须重新核对其证据或在新隔离实例复跑。PC 原版、两种官方 CJB、Android/iOS 官方客户端和 Android 实验性 SMAPI若缺少实体环境，必须逐项记录“未验证”及安全 unavailable 边界，不能猜测支持；其中 Android 实验性 SMAPI不属于承诺支持。任何测试都不得使用真实用户存档。
- Tag 前全量门禁：Control Docker .NET 6 契约与真实引用编译；后端 `go test ./... -count=1`、vet、build；前端全部 12 项状态测试、TypeScript 与 production build；兼容矩阵 validate/版本/远端制品及 Python 测试；`run.sh`、`migrate-fnos.sh`、ShellCheck；updater/runtime Docker integration；VitePress production build；精确候选 fresh/upgrade/rollback E2E；桌面、390px 手机和 280px 窄屏 Browser QA。任一失败先修复并重新跑受影响范围，证据不完整时禁止 tag。
- Tag 后核验：等待 release、compatibility-matrix、Pages 工作流成功；从 Docker Hub、阿里云 ACR、GHCR 回拉精确 `0.4.8`，核对三仓 digest、OCI version/revision、`latest`、GitHub Release 与 release assets，并分别完成隔离 `/health`、`/api/version`、未初始化数据库和 Docker/Compose smoke。随后更新公开展示站首页、更新日志、玩家手册与维护说明，完成 production build、桌面/手机视觉和线上 HTTP/文案复核，再把 workflow ID、digest、耗时、故障与清理结果回填本节。
- 精确候选与 tag 前门禁：release commit/tag 为 `0c5e2c434a92e8c9a69f839b39f86508cccf9a77`。本机最终候选 image ID=`sha256:2f8c7ddc0347011068424034d1e4ccaa266b927d2f60e717075d03cc211f6763`，version=`0.4.8`、revision=`0c5e2c434a92e8c9a69f839b39f86508cccf9a77`、created=`2026-08-06T15:43:16Z`。Control 契约与真实 game-data 编译、后端 test/vet/build、12 项前端状态测试/TypeScript/build、网站 build、脚本/ShellCheck、兼容矩阵 19 项、真实 SMAPI 下载、runtime/updater Docker integration 全部通过。
- 真实更新与失败恢复：正式 `v0.4.7` 通过真实 Web 检查、dry-run、管理员确认和 apply 升级到上述候选；update `51f5d3919c61` 成功并在 Panel 重启后保持 `succeeded/cleanupCompleted=true`。SQLite、setup、实例代表数据、game/mock sentinel ID 与 sentinel 文件保持；升级后重新验证 player Mod fixture、sidecar 只读哈希、内置项过滤、server_only 排除和零 kick/ban/block 命令。另用不存在的远端候选注入 pull 失败，得到 `image_pull_failed/failed_rolled_back` 并恢复官方 `0.4.7` healthy。
- 发布工作流：annotated tag `v0.4.8` 保持不移动。GitHub Actions 在 2026-08-06 官方 outage 期间先后造成初始 run 零步骤取消和第二次 release `Set up job` 失败；服务恢复后，同一 release run `31117969497` 第三次尝试从 `2026-08-07T04:46:59Z` 到 `04:53:09Z` 用时 6 分 10 秒成功。compatibility run `31117949897` 第二次尝试约 2 分 03 秒成功。GitHub Release `Stardew Server Anxi Panel 0.4.8` 于 `04:53:00Z` 发布，非草稿/非预发布，`run.sh`、`migrate-fnos.sh`、`repair-junimo-0.3.5.sh` 三项附件齐全。
- 发布后回拉：Docker Hub、阿里云 ACR、GHCR 的 `0.4.8` 与 `latest` 六个 OCI index digest 均为 `sha256:5381009b807ad2c632075332e3538297b5069eff2f2b1b133ff7fffd2ac38f90`，amd64 manifest 为 `sha256:928e1a65e06d332a2e00553dabec909b76addba3883d58d62e005d9b5e33ab2d`。六个 tag 实际 pull 后均为 version=`0.4.8`、revision=`0c5e2c434a92`、created=`2026-08-07T04:50:35Z`、size=`62,484,444` bytes。
- 三仓冒烟：Docker Hub、ACR、GHCR 精确镜像分别使用 `v048-release-smoke-{dockerhub,acr,ghcr}` 与独立 data volume/端口 `18061/18062/18063` 启动为 healthy；三组 `/health=ok/0.4.8`、`/api/version=0.4.8/0c5e2c434a92/2026-08-07T04:50:35Z` 精确通过。
- 展示站与提交后门禁：公开门户合并提交 `6f34b8a0b043dd27dcaacfb2c25ae2ada748b71e` 已推送 `main`。Pages workflow `31150162173` 的 build（18 秒）与 deploy（8 秒）成功；同提交 compatibility workflow `31150162180` 从 `05:18:31Z` 到 `05:20:19Z` 成功，后端、前端 production build 与 isolated Docker integration 均实际执行。线上首页、changelog 和玩家手册返回正确页面，桌面 1440×900 与手机 390×844 均无横向溢出；`v0.4.8`、玩家 Mod、自报/CJB/未上报边界可见，console warn/error 为 0。
- 最终清理：逐项复核 ownership label `v048-player-mod-release-20260806` 后，精确删除 8 个 `v048-*` 容器、12 个任务卷、1 个任务网络，并确认 `18050/18061/18062/18063/18070/24760` 无监听；未执行 prune，`stardew_game-data` 保留。`E:\\v048-player-mod-release-harness-20260806` 已移入 Windows 回收站。三个历史 worktree 的有效内容进入 `main` 后删除，五条非 `main` 本地分支均确认已合并后删除；远端始终只有 `origin/main`。

# v0.4.7 发布记录：全窗口响应式与平板全屏/滚动修复（2026-08-01，已发布）

- 变更范围：本版只发布 `FE-RESPONSIVE-VIEWPORT-1`。前端按视口真实内容盒计算桌面 Shell 缩放，修复隐藏外层被程序化滚动后全屏只剩一部分；手机及 1366px 内粗指针/无 hover 设备自动进入紧凑壳，主滚动、safe area、低高度认证页、弹窗内部滚动、44px 触控区、280px 操作区和旧浏览器回退统一收口。新增逐像素响应式测试并接入 release/compatibility workflow；官网首页和 changelog 同步 `v0.4.7` 用户说明。后端 API、鉴权、数据库 schema、Junimo 运行栈、Compose 格式与长期数据均未改变。
- 正常路径：最终候选必须在 Docker Desktop Linux containers 中以精确 `0.4.7`、最终 commit 和 UTC build date 构建；fresh Panel 完成 setup、登录、九路由桌面壳和紧凑壳主页面访问。桌面验证滚动、窗口缩放和浏览器全屏；紧凑壳验证横竖屏、底栏切页、长内容滑动、完整/适配版切换、输入框与更新/确认弹窗。真实候选页面不能只由 `qa-layout.html` mock 代替。
- 边界输入：公开支持从 `280 CSS px` 起。自动门禁逐宽度扫描 `280..3840` × 16 个高度 `240..2160`，另验 7680×4320、768/769 分界、1366px 粗指针条件、超宽低高度认证页、280px 图标底栏、低高度 OpsRail、新建游戏窄容器和无 `ResizeObserver`/旧 `MediaQueryList` 回退。低于 280px 与缺少基础 flex/grid/ES module 的内核不在承诺内。
- 权限与安全：本版不改变 session、角色、管理员权限、CSRF/同源请求或后端路由。未登录真实根页面必须继续显示登录/初始化流程，普通用户不能因切壳获得管理员操作；QA harness 只返回合成 mock 数据，不能作为真实鉴权成功证据，也不能访问真实实例数据。生产 build、现有鉴权测试和真实候选登录必须通过。
- 网络超时/断流：响应式逻辑本身不增加外网请求；资源加载、SSE/轮询和 Panel 更新仍沿用现有契约。发布 E2E 必须走 `0.4.6 → 0.4.7` Web 检查、dry-run、管理员确认、apply、预期断线重连和终态恢复；断线页、重连成功弹窗与低高度/窄屏布局均要可操作。
- 部分成功、重试与幂等：resize、`ResizeObserver`、`fullscreenchange` 和路由切换可以重复触发，rAF 计算、scroll 归零与监听清理不得累积或改变服务端状态；Shell 双向切换只影响当前 React 会话。升级 apply 的重复提交、活动任务与历史 dry-run 继续由后端现有幂等/冲突门禁处理，本版不放宽。
- 进程或容器中断后的恢复：页面刷新或 Panel 重启后按当前媒体条件重新选择壳并从权威 API 恢复数据；不持久化像素缩放或滚动偏移。候选容器重启后 `/health`、`/api/version`、setup/session、路由和主滚动仍须正常，更新状态必须保持可恢复。
- 失败回滚：先把更新目标替换为同版本标识的 unhealthy fixture，必须进入 `failed_rolled_back` 并恢复精确 `0.4.6`；旧 SQLite、setup、用户、实例、Compose、环境文件、Panel 数据、非目标 game sentinel 容器和测试 volume 内容均保持。随后换回精确候选并完成成功升级；不得用手工改 tag 或直接 `compose up` 代替 Web apply。
- 数据完整性：本版无迁移，但仍核对升级前后 SQLite 可读、初始化状态、管理员、实例、存档/Mod/备份与审计代表数据、事务备份和非目标容器 ID/volume hash；目标 `/health`、`/api/version` 及 OCI version/revision/created 必须精确。升级后重启 Panel 再做真实页面响应式验收，证明不是只有 fresh 安装有效。
- 资源清理：所有本轮资源使用 `v047-*` 专属 Compose project、容器、网络、唯一环回端口、bind 目录和 volume，并带 ownership label；清理前逐项核对归属，只删除本轮精确资源。不得复用或清理现存 `v046final-*` 验收资源、真实用户数据、长期凭据、41792 平板 QA 服务或执行任何全局 prune。
- 设备证据与剩余风险：应用内 Browser 已覆盖移动六页、桌面九页、认证页、断点和弹窗矩阵；用户于 2026-08-01 确认实体平板横竖屏滑动、浏览器全屏、底栏切页与输入法冒烟通过。用户明确不要求曾复现问题的朋友电脑另行验收，该设备未复验如实保留为剩余风险；本机桌面 Browser 矩阵与最终 Docker 候选真实登录页补充覆盖。Browser 仍不能直接制造所有厂商非零 safe-area 或浏览器栏组合。
- 升级矩阵：必须完成最新正式版 `0.4.6 → 0.4.7` 全套 Web 成功与 unhealthy 回滚，并在升级得到的新 Panel 上复验本版 UI。由于本版没有数据库迁移、部署格式、运行栈、长期数据或跨版本兼容逻辑变化，更老版本直升专项标记为不适用；若候选范围扩入任何上述变化，必须补跑至少 `0.3.13 → 0.4.7` 代表矩阵后才能 tag。
- Tag 前全量门禁：后端 `go test ./... -count=1`、vet/build；前端全部状态脚本、`test:responsive-layout`、audit 与 production build；兼容矩阵 validate/unit/远程制品；`run.sh`/迁移脚本功能测试、语法与 ShellCheck；updater 成功/回滚和 runtime Docker integration；VitePress build；候选 fresh/upgrade E2E；真实候选桌面/紧凑 Browser QA。任一失败先修复并重跑受影响范围。
- Tag 后核验：annotated `v0.4.7` 只能指向完成上述门禁的最终 commit。等待 Release、compatibility 和 Pages 工作流成功；分别从 Docker Hub、阿里云 ACR、GHCR 回拉 `0.4.7` 与 `latest`，核对六个 OCI index digest、version/revision/created、GitHub Release 正式状态及三项附件，并让三个精确镜像各自完成隔离 health/version smoke。最后把 workflow ID、digest、耗时、故障、清理和官网 HTTP/版本证据回填本节、前端接手文档与路线图。
- 精确候选：最终 release commit `619d18dafa76a9d99b90a218cc063949b79a26bf`；Docker Desktop 本机候选 `anxiyizhi/stardew-server-anxi-panel:0.4.7` image ID=`sha256:a610abda7524247f4254e8167d2e06f5ca6f985ef93f1a24ec2be12791369677`，OCI version=`0.4.7`、revision=`619d18dafa76`、created=`2026-07-31T19:44:13Z`。fresh 容器的 `/health`、`/api/version`、setup、管理员/普通用户鉴权和权限拒绝均精确通过。
- Tag 前全量门禁：Linux 后端 `go test ./... -count=1`、vet、build 分别用时 79.405/7.152/3.236 秒；前端 11 项状态/响应式测试、production audit 0 与 build 全绿；Node 20/24 VitePress build、兼容矩阵 19 项、脚本语法/功能测试及正式范围 ShellCheck、41,889,142 字节真实 SMAPI 下载、31 项 runtime Docker integration、27 项 updater 顶层用例及 5 个子用例全部通过。`TestDockerIntegrationRealPanelCandidateUpgrade` 的 `0.4.6 → 0.4.7` 精确候选链路用时 32.53 秒。
- 真实候选响应式：实际登录候选在 320×568、390×844、768×1024、1024×768、1440×900、1920×1080 六个视口均无页面级横向溢出；长内容可由内部主滚动区到达，1920×1080 根布局完整填满视口，console error/warn 为空。实体平板通过与朋友电脑豁免继续保留在上方“设备证据与剩余风险”，没有扩写成“所有设备均已证明”。
- 本机真实 Web 失败回滚：正式 `0.4.6` 经页面完成更新检查、全新 dry-run、管理员确认和 apply。最初 named-volume fixture 因 Compose 声明源与 inspect 主机路径不一致而被 `compose_metadata_invalid` 安全拒绝，改为任务专属 bind 并重跑全新 dry-run 后，受控 registry 的同版本 unhealthy fixture 进入 `waiting_health`；apply `a7fb752ba700` 用时 139.947 秒进入 `failed_rolled_back/health_check_failed` 并恢复精确 `0.4.6`。SQLite/setup/用户、Compose 与 `.env`、Panel 数据及 updater 备份均可读；非目标 game sentinel 的容器 ID、StartedAt 和 volume 文件摘要完全不变，浏览器自动重连并显示“升级失败，已恢复”。
- 本机真实 Web 成功升级：受控 registry 切回上述精确健康候选后，重新执行更新检查、全新 dry-run、确认和 apply；apply `df8c79592e66` 用时 14.236 秒成功，`/health` 与 `/api/version` 精确为 `0.4.7/619d18dafa76`，运行 image ID 为 `sha256:a610abda7524247f4254e8167d2e06f5ca6f985ef93f1a24ec2be12791369677`，终态为 `succeeded`、`fullStack=not_needed/100`、`cleanupCompleted=true`。Panel 重启后终态、数据库、setup、管理员会话及三组事务备份仍可读，game sentinel 保持不变；升级得到的新 Panel 在 390×844 由真实 CUA 将内部主滚动区 `scrollTop 0 → 553`，1920×1080 根布局完整填满视口，两者零横向溢出且 console error/warn 为空。
- 正式发布：annotated tag `v0.4.7` 精确指向 `619d18dafa76a9d99b90a218cc063949b79a26bf`。Release workflow `30662967983` 用时 5 分 05 秒、兼容矩阵 `30662818759` 用时约 1 分 40 秒、Pages `30662818712` 用时约 36 秒，三者结论均为 `success`；GitHub Release `Stardew Server Anxi Panel 0.4.7` 为非草稿、非预发布版本，`run.sh`、`migrate-fnos.sh`、`repair-junimo-0.3.5.sh` 三项附件齐全。
- 发布后回拉：全新隔离 DinD 从 Docker Hub、阿里云 ACR、GHCR 回拉 `0.4.7` 与 `latest`，六个 OCI digest 均为 `sha256:3f336863ae5ec45a1997edcfc0922269250d5763e8ada49a7ba43f81d59edd7f`。六个 tag 的 OCI version=`0.4.7`、revision=`619d18dafa76`、created=`2026-07-31T20:31:24Z` 完全一致；三个精确镜像分别启动为 healthy，并返回 `/health=ok/0.4.7`、`/api/version=0.4.7/619d18dafa76`。官网首页与 changelog 均返回 HTTP 200 并包含 `v0.4.7`。
- 实际清理：`v047-*` Compose/DinD project、容器、网络、环回端口、bind 目录、volume、fixture/候选镜像及 Browser tab 均在核对 ownership 后按精确名称清理；没有执行全局 prune，也未触碰真实数据或其它版本资源。按用户此前“让平板局域网访问”的要求，仅保留原有 `41792` 平板 QA Vite 服务，端口终验仍有且仅有一个监听者。

# v0.4.6 发布记录：Mod 安装时间、搜索排序与上传说明（2026-08-01，已发布）

- 2026-08-01 阻塞修复：未安装实例不再把全栈状态卡在 42%，`uninitialized/admin_created` 以及“全部实例无需同步”的聚合结果均为 `not_needed/100`。远程制品门禁为 required image inspect、Git traceability fetch 和 SMAPI Range 下载增加三轮有界重试；SMAPI 可保留已验证分块并轮换受审 URL，最终 SHA 不匹配则从另一来源整包重下。digest、trusted host、Range、大小、SHA 和 Git ancestry 均未放宽。
- 代码级验证：后端专项和完整聚合测试通过；兼容矩阵 19 项测试覆盖跨源续传、截断、恶意重定向、重试耗尽、错摘要重新整包下载、镜像与 Git 重试。真实 `verify-remote-artifacts` 在 required Docker Hub 首次 TLS 握手超时、SMAPI offset `33,554,432` 处 SSL EOF 与 HTTP 429 后恢复，301 秒通过；生产 Go 空缓存下载 `41,889,142` 字节用时 87 秒并通过固定摘要/ZIP。
- 当前全量门禁：Linux 后端第二轮 test/vet/build 全绿；首轮仅既有农场恢复测试在并行负载下碰到 5 秒边界，目标用例随后连续 10 次通过且完整后端重跑通过。前端十项状态测试和 production build、VitePress build、脚本功能/ShellCheck、兼容矩阵、updater 成功/回滚 integration 与 runtime Docker integration 全绿。
- 精确候选：commit `0c83a257e11cabac289a55f9b5f74a93c2a64e93`，本机镜像 `anxiyizhi/stardew-server-anxi-panel:0.4.6` 的 image ID=`sha256:dd7f97a4e005c8dc50820d676fb30c9e926f93d962bb4e34477782bb88b02940`，OCI version=`0.4.6`、revision=`0c83a257e11c`、created=`2026-07-31T16:51:04Z`。fresh 隔离容器的 `/health` 与 `/api/version` 精确通过，镜像 archive 导入任务 DinD 后 image ID 一致。
- `v0.4.5 → v0.4.6` Web 一键更新：真实完成检查、dry-run、管理员确认、apply、预期断线重连和终态恢复。先把目标替换为 unhealthy fixture，约 205 秒后进入 `failed_rolled_back` 并自动恢复 `v0.4.5`；随后更新精确候选约 50 秒成功，版本/revision 精确，`.env`、事务备份与 SQLite 可读，非目标 game sentinel 容器 ID、volume 文件摘要和 Panel 数据均未变化。Panel 重启后状态继续为 `fullStack=not_needed/100`，`admin_created` 实例不再出现 42%。
- 升级后新功能：在升级得到的 Panel 上实际上传本地 ZIP，并通过本地 HTTPS fixture 执行一键远程安装；两条 `installedAt` 严格递增且重启保留。右侧真实页面的桌面与 390×844 视口均通过 Nexus ID、名称模糊搜索、最近安装/A–Z/Z–A 排序，无横向溢出，console error/warn 为空；一键下载 Mod 默认排在稍早上传 Mod 前面。
- 老版本矩阵：任务 DinD 内真实 `ghcr.io/...:0.3.13` 通过生产 `RunApply` 升级到上述精确候选，`TestDockerIntegrationRealPanelCandidateUpgrade` 用时 24.76 秒并通过数据库、健康版本和非目标容器保护断言。Tag 前门禁至此完成；正式发布状态仍须等待 tag workflow、三仓回拉、`latest`、GitHub Release 和正式镜像隔离 smoke 后才能记为已发布。
- 正式发布：annotated tag `v0.4.6` 精确指向 `0c83a257e11cabac289a55f9b5f74a93c2a64e93`。Release workflow `30650678965` 用时约 5 分 9 秒、兼容矩阵 `30650677894` 用时约 1 分 56 秒、官网部署 `30650677818` 用时约 45 秒，三者结论均为 `success`；GitHub Release `Stardew Server Anxi Panel 0.4.6` 为正式非草稿版本，`run.sh`、`migrate-fnos.sh`、`repair-junimo-0.3.5.sh` 三项附件齐全。
- 发布后回拉：Docker Hub、阿里云 ACR、GHCR 的 `0.4.6` 与 `latest` 六个 OCI index 均为 `sha256:6fd03bb202e8083b3453e2351bd70251c1bc2fea0e5c0f779fc62d99af39e07f`。三个精确版本分别 `docker pull` 后，本机 image ID、OCI version=`0.4.6`、revision=`0c83a257e11c`、created=`2026-07-31T17:22:17Z` 完全一致；三个独立容器均返回 `/health=ok`、`/api/version=0.4.6/0c83a257e11c`。官网 `https://anxiyizhi.github.io/stardew-server-anxi-panel/` 返回 HTTP 200 且首页包含 `v0.4.6`。
- 变更范围：后端为手动上传、Nexus Premium 直连和浏览器扩展一键安装统一持久化 `installedAt`；前端已安装/配置列表默认最近安装优先，支持名称正反序及名称、文件夹、作者、`UniqueID`、包来源、Nexus Mod ID 模糊搜索；上传区明确多 ZIP、单 ZIP 多 Mod、任意外层目录和嵌套 ZIP 不递归解压的边界。官网首页、更新日志、维护页与深度手册同步 `v0.4.6`。
- 正常路径：必须在 Docker Desktop Linux containers 中用最终 commit 构建精确 `0.4.6` 候选，真实完成 setup、手动多 ZIP/单 ZIP 多 Mod、浏览器扩展一键下载、列表搜索排序、重启持久化与删除清理；Nexus Premium 路径因需要专用 Premium 测试账号时可由共享导入函数的自动测试补齐，但不得把浏览器扩展测试误写为 Premium 实测。
- 边界输入：覆盖任意目录深度、同 ZIP 多 manifest、多 ZIP、重复 `UniqueID`、旧 Mod 无历史时间、标点或分隔符搜索、空白搜索、中文/大小写规范化、嵌套 ZIP 明确不递归；同一批任一坏包必须整体回滚。
- 权限与安全：写操作继续只允许管理员且要求停服；sidecar 限制 2 MiB、版本化 schema、临时文件 `0600` 和原子替换；ZIP 路径穿越、symlink、重复目标与非法 manifest 沿用现有拒绝门禁。搜索只作用于已返回的可见 Mod，不扩大接口权限或读取任意文件。
- 网络超时/断流：本功能只有 Nexus 下载入口涉及网络，继续复用现有下载 job 的 HTTPS、大小/摘要、超时和失败清理；安装时间只在完整 ZIP 导入成功后提交，下载中断不得生成时间记录。手动上传和本地列表不依赖外网。
- 部分成功、重试与幂等：同一 ZIP 的多个 Mod 使用同一时间；批次后续失败回滚先前移动目录及时间记录；重复提交已安装 `UniqueID` 只报告跳过，不刷新原安装时间；并发安装串行化 sidecar 读改写，不能互相覆盖时间；删除同时清理对应时间，重复删除保持现有 404/幂等边界。
- 进程或容器中断后的恢复：时间 sidecar 与 Mod 目录同属实例长期数据，Panel/容器重启后排序不变；临时文件不作为权威状态，启动读取损坏 sidecar 时列表可用但新安装安全拒绝覆盖，防止静默丢历史。
- 失败回滚：验证 ZIP 解包/manifest 失败、sidecar 路径不可写或被目录占用时不留下本批 Mod、时间或临时文件；更新器目标 unhealthy 必须自动恢复 `v0.4.5`，数据库、Mod、sidecar、Compose 和非目标容器保持原状。
- 数据完整性：升级前后核对 SQLite 初始化/用户/实例、Mod 目录及 sidecar 字节、已安装与启用集合、备份和审计范围；`/health`、`/api/version`、OCI version/revision 必须精确。旧 Mod 不伪造文件时间，缺少 `installedAt` 时稳定排在新记录之后。
- 资源清理：全部门禁使用 `v046-*` 专属 Compose project、容器、网络、端口、bind 目录和 volume，并以 ownership label 核对后精确删除；禁止复用真实用户存档、长期凭据、现有唯一卷或执行全局 prune。
- 实际清理：三仓回拉 smoke 的三个容器、volume、network，旧候选 fresh smoke、旧 Panel/HTTPS/website fixture，以及 DinD 内旧 gate/fix 项目均按精确名称和 ownership/Compose project 核对后删除。为满足用户继续查看真实操作页的要求，仅保留 `v046final-upgrade-panel`、game sentinel、受控 HTTPS fixture、registry 与其所属任务 DinD/网络/volume；它们只含专用测试账号和合成 Mod，不含真实存档或长期凭据，可在右侧页面不再需要时整组精确删除。
- 升级矩阵：在隔离 Docker-in-Docker 中运行正式 `v0.4.5 → 0.4.6` Web 更新完整链路（检查、dry-run、管理员确认、apply、断线重连、终态恢复），并从受 sidecar 新增影响的最老代表版本验证旧 Mod 无历史时间兼容；升级得到的新 Panel 必须再次执行上传、一键下载、搜索排序、重启和删除。候选镜像只通过本地镜像源提供，不在门禁完成前推送正式 registry tag。
- Tag 前全量门禁：后端 test/vet/build、前端全部状态测试与 production build、兼容矩阵、脚本功能测试与 ShellCheck、Docker integration、VitePress build、候选 fresh/upgrade E2E 和桌面/390px Browser QA。任一失败先修复并重跑受影响范围。
- Tag 后核验：等待 `release.yml`、兼容矩阵和 Pages 工作流成功；从 Docker Hub、阿里云 ACR、GHCR 回拉精确 `0.4.6`，核对三仓 digest、OCI version/revision、`latest`、GitHub Release，并逐一完成隔离 health/version smoke；把 workflow ID、digest、耗时、故障与清理结果回填本节。

# MOD-INSTALL-TIME-1 本地 Docker Desktop 门禁（2026-07-31，completed，未发布）

- 按用户要求只做本地候选，不创建/移动/push `v*` tag，不更新 `latest`，不推 registry 或 GitHub Release。最终候选为 `stardew-server-anxi-panel:mod-install-time-qa`，OCI version=`0.4.5-modtime-qa`、revision=`local-dirty-120a140ef38f`、created=`2026-07-31T07:47:19Z`，带测试 ownership label。
- Docker Desktop 29.5.3 Linux containers 使用专属 `mod-install-time-20260731-*` 容器、缓存卷、fresh Panel 数据卷和端口。候选 fresh volume 返回 `/health=ok`、`/api/version=0.4.5-modtime-qa/local-dirty-120a140ef38f`、`initialized=false`，OCI title/version/revision 精确。
- Linux 容器通过：后端 `go test ./... -count=1`、vet/build；前端 `npm ci`、十项状态/功能测试、`npm audit --omit=dev`（0 漏洞）和 production build；兼容清单 validate、10 项 Python unittest、`run.sh`/`migrate-fnos.sh` 语法与功能测试、ShellCheck、VitePress build。
- Docker integration 通过 runtime staging、steam-session 克隆恢复、镜像删除保护、参数注入拒绝、updater 隔离 Compose 成功及 unhealthy 回滚。需要正式候选/旧正式镜像环境变量的 real-image upgrade/reconcile 用例按设计 skip；本轮不是发版，未伪造这些变量，也未运行正式升级/tag 后门禁。
- 应用内 Browser 使用隔离本地 API fixture 验证 1440×900 与 390×844 的默认安装时间排序、旧数据兜底、UniqueID/Nexus ID 模糊搜索、名称正反序和配置页复用；无横向溢出、overlay 或 console error/warn。测试资源按 ownership 精确清理，不操作现有实例。
- 追加真实 E2E 候选为 `stardew-server-anxi-panel:modtime-e2e-20260731`，OCI version=`0.4.5-modtime-e2e.20260731`、revision=`120a140ef38f`、created=`2026-07-31T10:33:59Z`。Docker Desktop 29.5.3 Linux containers 使用独立容器、network、端口 `127.0.0.1:18090` 和 fresh volume；health/version/setup 精确匹配。
- 实际 Panel HTTP 完成初始化、本地深层 ZIP、浏览器扩展一键公网 HTTPS 下载 job、本地第二包、列表、容器重启和删除。三个时间严格递增且重启不变；多 ZIP 第二包无 manifest 与 sidecar 路径变目录两类失败均回滚新目录/时间并保留旧数据。
- 右侧栏真实登录页面验证桌面与 390×844：默认最近安装、一键下载来源 Nexus ID `4242`、UniqueID 片段搜索、名称正反序、添加/配置页共享查询、安装时间显示均通过；两视口无横向溢出，overlay 未出现，console error/warn 为空。内置 Nexus Premium `/mods/nexus/install` 未使用真实专用 API Key；本轮真实一键下载覆盖浏览器扩展 `/mods/remote/install` 路径，二者的文件导入继续共享 `uploadModZip`。

# v0.4.5 发布记录：SMAPI 受审加速源分块续传（2026-07-28，已发布）

- `0.4.4` 及更早版本仍把 SMAPI 官方 installer 的整段 body 限制为 2 分钟；约 `40 KiB/s` 的国内链路会在 Stardew/SDK 已完成后稳定失败。正式 `0.4.5` 已包含 `SMAPI-DOWNLOAD-RESUME-1`。
- 固定候选顺序为 `gh.llkk.cc → github.dpik.top → ghfast.top → GitHub 官方`。清单校验必须拒绝缺项、换序、任意代理和额外 host；运行时整包摘要/ZIP 错误时必须清空该候选临时内容再切下一项。
- Docker Desktop 已用 `golang:1.25-alpine` Linux 容器和生产 `ensureRecommendedSMAPIArchive` 从空缓存完成真实下载：`41,889,142` 字节、`2m26s`，固定 SHA/ZIP、缓存 `0600` 与无 `.part` 残留均通过；本地 `stardew-server-anxi-panel:0.4.5-local` health/version、Docker/Compose 和隔离 volume 冒烟通过。
- 验收必须核对正式缓存模式 `0600`、大小、SHA256 `dd01ddca7b566bfe0d3b3d2d03833496abc56c53da976241f2ab443f5484acc4`、SMAPI `4.5.2` 元数据、`INSTALL_REQUIRED_FILES_OK`，以及只有 Panel 常驻容器、无临时下载/安装容器残留。
- 安全门禁必须保留：embed 精确候选模板、六项 redirect allowlist、严格 `206/Content-Range`、固定大小/SHA/ZIP 结构；`.env.SMAPI_DOWNLOAD_URLS` 仍不参与选择。
- `.github/workflows/release.yml` 新增真实 SMAPI 下载 gate；工作流 `30369196944` 已完成全量 test/vet/build、Docker integration、前端/脚本门禁、三仓推送和 GitHub Release。Docker Hub、阿里云 ACR、GHCR 的 `0.4.5/latest` OCI index digest 均为 `sha256:a8155defc50690b8b1e90c95f5b107e818b5438c68c341f90f9ebf8b7be428ad`。
- 从 ACR 回拉精确 `0.4.5` 后，隔离 Docker Desktop 容器返回 `version=0.4.5`、`commit=09250ed68ce9`；health、未初始化空库、Docker `29.5.3` 与 Compose `2.27.0` 均通过，测试容器和 volume 已删除。
- 发布后真实升级复验使用 ACR 精确 tag：`0.4.4 → 0.4.5`、`0.4.3 → 0.4.5`、`0.3.13 → 0.4.5` 均通过生产 `RunApply`。每次目标版本/健康、非空 SQLite/setup 查询、扫描路径 404 正确，同 Compose game 容器 ID 未变化。
- `v0.4.5` tag 后只有文档提交，后端/脚本/工作流代码无差异；升级路径通过后再次在 Docker Linux 空缓存运行正式 SMAPI archive gate，`41,889,142` 字节耗时 56 秒，SHA/ZIP、缓存 `0600` 与无 `.part` 残留通过。半包续传、坏代理切官方、连续无进展和错摘要拒绝专项全部通过。

# v0.4.4 发布门禁：游戏日回档即时消费（2026-07-28）

- 候选镜像必须证明 Panel 启动后无需访问备份列表 API，也会在每个 `GameLoop.Saved` 事件后生成当日 `auto_<save>_<ordinal>.zip`；连续模拟 7 个游戏日后只能保留连续最近 5 日，不能只检查 ZIP 数量。
- Docker Desktop 隔离验证使用独立 bind 数据目录、Panel 数据库和端口；直接写入 Control 事件与存档日期，逐日等待后台调度生成 ZIP。不得复用或修改本机现有游戏实例、存档和 volume。
- Docker Desktop 29.5.3 已构建 `0.4.4-rc` 并用独立容器、volume、端口 `18094` 验证：第 1–7 日事件均在后台轮询后生成非空 ZIP，最后严格只剩 `000003..000007`；停掉 Panel 后排队第 8 日事件再启动，启动补扫立即生成 `000008` 并推进为 `000004..000008`。全程未请求备份列表 API，事件目录最终为空；`/api/version=0.4.4`、OCI title/version 正确。隔离容器和 volume 已清理。
- 发布前后端全量 test/vet/build、updater/runtime Docker integration、兼容矩阵与脚本门禁、前端九项状态脚本和 production build、VitePress production build 均已通过；兼容矩阵工作流 `30293214212` 成功。
- annotated tag `v0.4.4` 已由发布工作流 `30293219220` 完成 GitHub Release 与 Docker Hub、阿里云 ACR、GHCR 的 `0.4.4/latest` 推送。三仓精确 tag 的 OCI index digest 均为 `sha256:446b168c8784b3c7e77c5006b85adcbe2c1b106e80992281a929a75108fd572a`。
- Docker Desktop 从 GHCR 实际拉取正式 `0.4.4`，以独立容器、volume 和端口 `18095` 启动；`/health` 与 `/api/version` 均报告 `0.4.4 / d5d815d365cb`，OCI version/revision 同样正确，隔离资源已按 ownership label 清理。远端生产实例升级前仍必须备份 Panel 数据与当前存档，升级后确认下一次游戏日保存产生连续回档点。

# v0.4.3 发布门禁：Panel 健康监控与重启边界（2026-07-26，已发布）

- 镜像 HEALTHCHECK 每分钟请求一次 `/health`，单次超时 5 秒、连续三次失败后只把容器标记为 unhealthy。Docker restart policy 不会因为 unhealthy 自动重启；它只在容器主进程退出后生效。
- Panel 同时运行独立的进程内健康监控：启动一分钟后首次执行，之后每分钟复用 `/health` 的 `Store.Ping`。只有连续三次探针返回 SQLite 原生 `SQLITE_INTERRUPT`（code 9）时，Panel 才以状态码 1 主动退出；成功、超时或其它错误都重置计数。标准 Compose 的 `restart: unless-stopped` 随后只重新启动 Panel，不操作数据库、volume、游戏容器、存档或 Mod。
- Docker Desktop 29.5.3 已构建并复验最终候选与正式 `ghcr.io/anxiyizhi/stardew-server-anxi-panel:0.4.3`：`/health`、`/api/version`、404、OCI title/version、interval `60000000000ns`、timeout `5000000000ns`、retries 3 均正确；正式镜像主进程退出后 PID `14558 → 14689`、RestartCount `0 → 1`，重启后 `/health` 为 ok。
- Linux `golang:1.25-alpine` 内取消查询后下一查询成功与健康门限回归连续 10 轮通过；发布后正式 updater 的真实 `0.4.2 → 0.4.3` apply 再次成功，SQLite 数据卷、setup 状态和 404 保留，隔离 game 容器 ID 未改变；新 Panel 对旧 helper 成功状态的镜像清理续验也通过。
- tag 前兼容矩阵/远端制品、9 个 Python 单测、脚本功能/ShellCheck、后端全量 test/vet/build、updater/runtime Docker integration、前端九项状态测试/build、VitePress build 与浅/深/390px Browser QA 全部通过。发布工作流 `30188473585`、兼容矩阵 `30188469333`、Pages 部署 `30188469313` 均成功；Docker Hub、阿里云 ACR、GHCR 的 `0.4.3` 均为 digest `sha256:e1ffb4132610607305ce2316e5f3683b6413765bc61057ffd019b2841d38e559`。

# PANEL-SQLITE-INTERRUPT-1 容器恢复边界（2026-07-24）

- Panel 连续收到三次原生 `SQLITE_INTERRUPT` 时以状态码 1 主动退出，避免单连接 SQLite 池永久停留在中断状态。标准 Compose 的 restart policy 负责重新启动同一 Panel 容器服务；这不是镜像升级，也不删除或重建数据库、volume、游戏容器、存档或 Mod。
- 单个 HTTP 请求取消不会触发退出。`modernc.org/sqlite v1.54.0` 会把已中断的文件连接标记为不可复用，正常路径由 `database/sql` 丢弃该连接并让下一查询恢复；三次门限仅是连续原生中断仍未恢复时的最后保护。
- 候选镜像 smoke 除 `/health` 与 `/api/version` 外，应补一个“取消长查询后下一次数据库查询成功”的回归；部署若显式禁用 Docker restart policy，则连续中断退出后需要管理员自行启动 Panel，不能宣称自动恢复。

# v0.4.1 发布记录：飞牛 OS 迁移脚本更新（2026-07-20）

- `v0.4.1` 发布 `migrate-fnos.sh` 修订 3：支持保留可验证的额外 bind mount 与具名 Docker volume；迁移后复核 Compose、四项 labels、服务对应容器 ID、镜像引用/digest 和可写数据挂载，成功输出 `ANXI_PANEL_WEB_UPDATE_READY`。
- 修复飞牛残留 Compose project/service labels 与新项目同名时，改名保留的旧容器被 Compose 误识别并重新启动的问题；冲突时使用旧容器 ID 派生的隔离 project，旧容器仍保留用于失败回滚。
- 展示文档新增 `v0.4.1` 更新日志，收紧顶部“快速上手”导航胶囊高度，并突出首页第二行左侧的版本更新卡片。Docker Desktop 隔离 dind 已覆盖残留 labels、额外挂载和 `0.3.7 → 0.4.0` 成功迁移；完整后端 test/vet/build、两项 Docker integration、前端九项状态脚本与 production build、兼容矩阵、Shell 函数测试、ShellCheck、VitePress build 及桌面/390px 视觉验收均通过。`0.4.1-rc` 候选镜像返回 healthy、精确 `/api/version=0.4.1`、正确 OCI identity，并内置可执行 updater 与 `migrate-fnos.sh` 修订 3。

# v0.4.0 发布记录：一键全栈安全升级（2026-07-20）

- `v0.4.0` 将 Panel 自更新扩展为可恢复的一键全栈升级：安全识别任意 Compose 服务名，并对飞牛残缺 labels 执行容器、Compose、镜像和数据挂载四方反查；无法证明一致的部署拒绝自动操作。
- 飞牛式旧容器可由镜像内独立 `/app/panel-updater` 转换为标准 Compose。重建前备份数据库、Compose、环境变量、容器 inspect 和旧镜像 digest；新 Panel 健康、版本或 labels 验收失败时恢复旧容器、部署文件与数据库。
- 新 Panel 会逐实例校验 Control 版本和 DLL SHA-256。不匹配时严格执行游戏内通告、保存、整档保护备份、停服安装、重新启动和 SMAPI 实载版本验证；同步失败的实例保持停止，禁止带旧 DLL 继续运行。状态持久化保证 Panel 自身重启后继续剩余步骤。
- Docker Desktop 门禁已覆盖自定义 Compose 服务、成功升级、健康失败回滚、多旧容器连续转换、数据库 hash 恢复、`.125` stopped/running Control 更新、在线通告与保存证据，以及 `0.4.0` 候选镜像 health/version/OCI/helper/script smoke。annotated tag `v0.4.0` 继续触发 Docker Hub、阿里云 ACR、GHCR 的 `0.4.0/latest` 和 GitHub Release。
- `migrate-fnos.sh` 修订 2 不再因存在任意第三挂载而统一拒绝：脚本从 Docker inspect 分类额外挂载，可验证的 bind mount 保留 source/target/read-only/propagation，Docker volume 以原名称声明为 external 并保留 read-only；标准 Docker Socket也保留原只读属性。Panel 数据目录必须可写；tmpfs、宿主设备、未知 mount type 或不安全字段会在拉镜像、备份和切换前拒绝。`extra-mounts.txt` 随迁移保护材料落盘。
- 修订 2 的成功门禁不再只检查新容器能否启动：切换后必须再次验证精确版本、Compose 可解析、project/service/config_files/working_dir labels、Compose `panel` 服务与当前容器 ID、配置镜像与运行 image digest、可写数据挂载全部一致。任一失败都回滚旧容器；全部通过时 `result.txt` 写入 `upgrade_environment=supported` 与 `success_code=ANXI_PANEL_WEB_UPDATE_READY`，终端明确输出“支持后续新版本通过 Panel Web 一键安全升级”和同一成功识别码，便于远程管理员确认。
- 修订 3 修复飞牛部分残留 labels 与脚本生成 project 同名时，旧容器改名后仍被 Compose 当作目标服务重新启动的问题。脚本在修改前检查旧 project label 和全局同 project 容器；存在冲突时使用基于旧容器不可变 ID 的 `anxi-panel-migrated-<12位ID>` 隔离项目名，并再次确认该项目未被任何容器占用。这样既保留旧容器用于回滚，又保证新 Compose 只能创建并管理新 Panel。
- Docker Desktop 真机使用真实 `0.3.7` 容器验证“只读时区 bind + 可写外部 volume + 只读 Docker Socket”迁移到 `0.4.0` 后四项 mount inspect 完全保持；另注入 `--tmpfs /tmp/custom`，脚本在修改前退出，旧容器仍为原镜像、原名称、`unless-stopped` 且持续健康，未生成 Compose 或事务目录。

# v0.3.13 发布记录：存档上传编码与删除一致性（2026-07-20）

- `v0.3.13` 修复旧式中文 ZIP 路径名未规范化、历史非 UTF-8 目录经 JSON 变成 `�` 后无法寻址，以及删除已落盘成功却因后续清理报失败导致前端保留幽灵卡片的问题。新上传统一 UTF-8；遗留目录通过稳定公开身份执行备份、导出和删除，编码异常目录不允许激活。
- 删除前备份、活动指针和目录删除形成有补偿的顺序；目标不存在明确返回 404。前端在成功与失败后都重新读取 saves/backups，并用 `allSettled` 保证刷新异常不会锁死页面。
- 本地发版门禁：Control 0.2.2 Docker .NET 6 契约与只读 game-data 编译（0 errors）、后端全量 test/vet/build、Docker integration、兼容矩阵远端制品、9 个 Python 测试、`run.sh`、九项前端状态脚本与 production build均通过。可选 server 镜像源不可达仅产生 warning，canonical 制品校验成功。
- Docker Desktop 使用独立 `save-name-fix-013-panel` 容器、`save-name-fix-013-panel-data` 卷和 18087 端口：验证 UTF-8/GBK ZIP HTTP 预览、原始 GBK 目录中文显示与告警、409 激活拦截、删除前中文备份、重复删除 404、目录确实消失及重启后不复现；未复用或修改现有实例。正式镜像继续由 annotated tag `v0.3.13` 触发三仓 `0.3.13/latest` 和 GitHub Release。

# v0.3.8 发布记录：删除离线存档人物（2026-07-18）

- `v0.3.8` 新增运行中删除当前存档离线 farmhand：Panel 通过 Junimo `.125` 官方 `/farmhands` 接口删除人物、小屋和 slot 记录，不直接编辑 XML；其他真人玩家在线允许操作，被删除目标在线仍硬拒绝。
- 删除事务包含前保存、`prefarmhanddelete_*` 整档保护备份、删除前二次校验、运行态复核、后保存与磁盘 XML 复核；在线玩家收到删除前后游戏内通告，UI 明确建议重连以刷新小屋建筑状态。
- Docker Desktop 使用独立 `farmhand-delete-e2e` Compose project、数据目录、端口和 game-data volume 克隆真实双人物/三小屋存档；验证人物 `2→1`、小屋 `3→2`、重启持久、重复删除安全失败，并用生成的保护备份自动停服/恢复/重启后确认人物和小屋复原。原来源实例未启动或改写。
- 发布沿用 `release.yml`：完整门禁通过后 annotated tag `v0.3.8` 触发 Docker Hub、阿里云 ACR、GHCR 的 `0.3.8/latest` 与 GitHub Release。

# v0.3.7 发布记录：升级旧镜像安全清理（2026-07-17）

- `v0.3.7` 发布 Panel 与 Junimo/auth 成功升级后的旧镜像定向清理，并包含新 Panel 对旧版本 helper 成功状态的幂等收尾，使 `0.3.6 → 0.3.7` 当次升级即可生效。失败、回滚、共享容器、自定义仓库、未知 tag、容器和 volume 安全边界不变。
- Docker Desktop 真机门禁通过：Panel 成功升级真实删除本次旧 tag/可信历史 tag并保留容器引用和自定义镜像；unhealthy 目标真实回滚 Panel/数据库且不清理；真实 `0.3.7` 生产镜像从旧 helper `succeeded` 状态启动后写入 `cleanupCompleted`，测试前已有镜像全部受临时容器保护并确认未误删。
- Junimo/driver 真机门禁通过：精确 image ID 与容器引用保护、steam-session clone/restore、真实 `.125` server/auth HTTP 探针、SMAPI staging，以及从只读真实 game-data 克隆的 `.121 → .125` stopped/running 升级；测试旧 `.121` tag 被清理后按测试前 image ID 恢复，随机容器/卷零泄漏。
- 发布总门禁通过：后端 `go test ./... -count=1`、vet/build、Docker integration；前端 `npm ci`、九项状态脚本和生产 build；兼容矩阵 validate/check-panel-version/9 项 unittest、远程制品校验、`run.sh` 更新测试；本地 `0.3.7` 镜像 fresh volume 的 health、`/api/version`、OCI labels 和 `/app/panel-updater` 均通过。
- Tag workflow 沿用往期流程发布 Docker Hub、阿里云 ACR、GHCR 的 `0.3.7` 与 `latest`，并附加 `run.sh`、0.3.5 修复脚本和 annotated tag 详细说明创建 GitHub Release。

# IMAGE-CLEANUP-1 升级后旧镜像清理边界（2026-07-17）

- Panel 新版本完成容器健康、HTTP health 和 `/api/version` 精确验收后，helper 会重新核对旧 tag 的 image ID，再执行不带 `--force` 的 `docker image rm`。随后按 OCI title label 枚举镜像，仅删除可信仓库内、未被任何现存容器引用且 image ID 再核对一致的历史稳定 tag或陈旧 `latest`；最后运行同 label 的默认 dangling prune，不使用 `-a`，不会清理自定义仓库或未知 tag。
- Junimo server/steam-auth-cn 只在成对升级完整成功并清除事务恢复目录后，按 recovery manifest 中记录的旧精确引用和 image ID 定向删除。Docker 仍会拒绝删除被任何容器引用的镜像；tag 已漂移时 Panel 主动跳过。
- 失败、成功回滚、`rollback_failed` 和仍在运行的升级均不清理旧镜像。清理失败只产生 warning，不回滚已经验收的新服务；容器、volume、game-data、steam-session、数据库、存档、Mod、SteamCMD、FRPC 和其它宿主镜像不受影响。
- 下次成功 Panel 升级会处理此前积累且满足上述门禁的历史 tag；被容器引用或不在可信范围的镜像仍保留。管理员若要处理保留项，应先用 `docker ps -a` 核对引用，再无 `-f` 执行精确 `docker image rm <ref>`。禁止把 `docker image prune -a` 加入发布、升级或修复脚本。
- 发布门禁：`go test ./internal/updater ./internal/docker ./internal/games/stardew_junimo -count=1`；可选隔离 Docker integration 验证旧 Panel 精确镜像在成功 apply 后不存在，同时测试 wrapper 必须拦截 label prune，避免触碰宿主其它部署。

# v0.3.6 发布记录（2026-07-17）

- `v0.3.6` 发布存档导入复合证据适配；tag workflow 在推送三个 registry 的 `0.3.6`/`latest` 和创建 GitHub Release 前执行兼容清单、远程制品、完整 Go、Docker integration、前端 save-import 专项及生产构建门禁。
- 发布前本地门禁已通过；隔离 `.125` 技术 E2E 已覆盖 takeover/as-is、swap、持久保存和第二次重启。本版本不宣称 `SAVE-IMPORT-JUNIMO-1` 的人工游戏语义与完整故障注入总门禁 completed。
- 发布说明明确保留上游无 commandId 的事实，并记录 Panel 使用磁盘事务痕迹、pending、saveId、finalizeCount、GameLoop.Saved 与 dayTransitionComplete 的复合证据路线。

# SAVE-IMPORT-E2E-RELEASE-1 镜像发布门禁（2026-07-16，未放行）

- 本轮没有构建发布镜像、创建 release、推送 registry 或部署生产。现有隔离 spike 容器全部保持停止，只进行了只读卷/归档盘点。
- Go test/vet/build 和前端 typecheck/build 已通过，但真实八类存档 E2E、故障注入、人工游戏语义与第二次重启未完成，因此镜像发布门禁保持关闭。
- 在 `SAVE-IMPORT-JUNIMO-1` 获得真实 completed 证据前，release workflow 不得仅凭单元测试结果宣称存档导入可发布。后续发布记录必须引用隔离 Compose project、每份原始 ZIP/SHA256、事务 operation/job、二次重启及人工检查结果。

# PANEL-0.3.5-JUNIMO-REPAIR-SCRIPT 飞牛/NAS 一键修复（2026-07-16）

- 新增 `deploy/repair-junimo-0.3.5.sh`，处理旧 Panel 升至 `0.3.5` 后可信候选镜像 tag 混用、实例停在 `invalid_config/image_candidates` 的现场。脚本必须在 NAS/Linux 宿主机 SSH 中运行，不能在 Panel 容器终端内运行。
- 脚本只接受正在运行且 OCI title 为 `stardew-server-anxi-panel`、OCI version 精确为 `0.3.5` 的唯一容器，再通过 `PANEL_DATA_DIR` mount 反查宿主机实例目录；零个或多个匹配、自定义主镜像、主版本歧义、活动升级任务和 `rollback_failed` 都只报告并退出。
- 通过门禁后，脚本先以 `0600` 备份实例 `.env`，只规范化 server/auth 候选列表，备份旧 required coordinator 状态，然后重启 Panel，让 `0.3.5` 内置 required update 事务完成 `.125` 下载、安装、验收和失败回滚。它不执行 `down -v`、volume prune，也不删除 `game-data`、steam-session、存档或 Mod。
- `release.yml` 会把该脚本与 `deploy/run.sh` 一起附加到正式 Release。自有 HTTP 目录可直接提供 `curl -fsSL <URL>/repair-junimo-0.3.5.sh | sudo bash`；目标镜像仍必须联网拉取并通过内嵌 digest 校验，不能靠改 tag 绕过。
- 飞牛现场修正：Docker mount inspect 使用无空格的 `destination|source` 格式输出后再精确匹配 `PANEL_DATA_DIR`，避免 Go template `println` 自动插入分隔空格、导致已有 bind mount 被误报为“无法确定宿主机数据目录”。
- 旧部署若遗留 `IMAGE_VERSION=latest`，但 server 主镜像属于可信仓库且精确 tag 唯一为 `.121` 或目标 `.125`、auth 也精确为目标 `.2`，脚本允许在完整备份后把 `IMAGE_VERSION` 收敛为 server 主 tag；其它主字段歧义仍拒绝自动选择。
- 脚本还兼容空值或带引号/空白的旧 `IMAGE_VERSION`、UTF-8 BOM 和非标准 NAS mount destination；mount 回退只接受唯一实际包含 `instances/stardew/.env` 的源。主 server 仅允许当前可信仓库与 `.121/.125`，auth 仅允许精确 `.2`，旧候选别名只会被替换而不能借此晋升为主镜像。
- 执行前同时拦截 required coordinator、Junimo apply 与 SMAPI apply 的活动阶段或任一 `rollback_failed`，避免为修候选配置而中断另一项维护事务。已是 `.125` 的实例走同一幂等检查，配置规范化后由 Panel 判定 `up_to_date/succeeded`，不会无条件重建服务。

# RELEASE-NOTES-BACKFILL-1 Tag 发布说明补全（2026-07-16）

- 已为 `v0.1.0` 至 `v0.3.5` 的全部 32 个 GitHub Release 补充实际更新说明；内容优先取 annotated tag 的详细注释，其次取对应发布提交正文，并为历史短注释版本补充维护文档中的功能、修复与验证摘要。
- 原发布流程仅使用 `generate_release_notes: true`。仓库多数版本由单个直推提交创建 tag、没有合并 PR，GitHub 自动生成器因此经常只能输出 `Full Changelog` 比较链接，不能形成面向用户的更新说明。
- `.github/workflows/release.yml` 现在完整拉取 tag 历史，并在构建阶段生成 Release 正文：优先使用 tag 标题后的详细注释，缺失时回退到提交正文，再缺失时至少列出提交标题；正文统一附带上一版本的完整比较链接。
- 后续创建 tag 时仍建议使用 annotated tag，并在标题后写清主要功能、修复、兼容性或升级注意事项。Release workflow 会原样采用这些说明，不再依赖 PR 才能生成有效正文。
- 验证方式：检查 workflow YAML 可解析；创建测试 tag 或正式 tag 后，确认 GitHub Release 正文不只包含比较链接，且 `deploy/run.sh` 附件和三仓库镜像发布流程保持不变。

# PANEL-0.3.2 宿主 Junimo DLL 升级修复发布（2026-07-15）

- `v0.3.2` 修复 server 镜像升级后宿主 `./.local-container/mods/JunimoServer` 仍保留旧 DLL、导致容器 tag 与实际 Mod 版本不一致的问题。
- 发布门禁必须覆盖：目标镜像 Mod 提取与严格版本校验、宿主目录事务替换、任一 apply 失败后的旧 Mod 恢复、启动自愈，以及 FIFO `info` 实际版本不符时拒绝成功。
- runtime manifest 的 `minimumPanelVersion` 为 `0.3.2`。Tag `v0.3.2` 继续由 `.github/workflows/release.yml` 运行远程矩阵、Go/Docker integration、前端测试与构建，成功后推送 Docker Hub、阿里云 ACR、GHCR 的 `0.3.2/latest` 并创建 GitHub Release。
- 生产升级顺序：更新 Panel 镜像到 `0.3.2`，在维护窗口重启 Stardew server，确认 `info` 的 `Version:` 与 `.env IMAGE_VERSION` 一致。仅拉取 `.125` server 镜像不足以更新 bind-mounted Mod。

# PANEL-0.3.1 可信旧候选修复发布（2026-07-15）

- `v0.3.1` 修复旧 `.121` 候选与新版 `.125` 默认候选被安装流程合并后，维护卡片只能显示“配置无效”且无法升级的问题。
- 发布门禁必须覆盖：可信混合/退役候选返回 `repairable`；修复接口私有备份并规范化后返回 `.121 → .125 update_available`；自定义主镜像、未知候选和 `rollback_failed` 不得自动修复；安装候选不得跨 tag 混合。
- 前端验收使用 `qa-layout.html?junimoConfig=repairable`；桌面和 390px 窄屏必须显示唯一“修复并升级”按钮，无横向溢出和控制台错误。
- Tag `v0.3.1` 继续触发 `.github/workflows/release.yml`，通过远程矩阵制品、Go/Docker integration、前端全量状态测试和生产构建后，发布 Docker Hub、阿里云 ACR、GHCR 的 `0.3.1/latest` 并创建正式 GitHub Release。

# PANEL-0.2.10 组件升级竞态修复发布（2026-07-15）

- `v0.2.10` 修复 Junimo/SMAPI 一键升级误用历史成功预检、导致新预检与 apply 抢跑，以及旧失败终态覆盖新任务进度的问题。
- 发布前必须通过 `test:component-update-flow` 和本地 `junimoWorkflow=race-retry` 点击验证；预期一次点击只有一个新 dry-run POST、成功轮询后才有一个 apply POST，且不得出现提前 apply 的 409。
- Tag 继续触发 `.github/workflows/release.yml` 的完整矩阵、远程制品、Go/Docker integration、前端全量状态测试与生产构建，通过后发布三个仓库的 `0.2.10` 和 `latest`。

# PANEL-0.2.8 单卡片交互覆盖发布（2026-07-14）

- 按维护者要求复用并强制更新 `v0.2.8` tag；三个镜像仓库的 `0.2.8` 与 `latest` 都必须由新的 tag commit 重新构建覆盖，不得仅在服务器本地改文件。
- 本次覆盖只调整版本维护前端：Junimo/SMAPI 的校验、下载、安装、验收均在用户卡片内展示，移除跳往下方技术详情的操作按钮；刷新恢复到活动任务时，按钮保持“升级进行中…”并禁止重复提交。
- 覆盖发布完成后必须以远端镜像 digest 和 `/api/version` 验证服务器实际运行的是新的 `0.2.8` 产物，不能只依据 tag 文本判断。

# PANEL-0.2.8 回滚版本检测与维护提示修复（2026-07-14）

- `v0.2.8` 修复 Junimo 升级回滚把临时 image ID 永久写入 `.env`、导致 `.121 → .125` 推荐更新消失的问题。
- 回滚容器仍按升级前精确 digest 重建；完成或失败退出时恢复原始 tag 配置。最终恢复失败继续保留 recovery 并进入人工处理。
- 维护卡片对 `rollback_failed`、`invalid_config` 和读取失败显示需要关注，不再显示绿色“无需操作”。推荐矩阵、镜像候选和 23 个 init 兼容挂载不变。

# RUN-SH-LATEST-UPDATE-1 自动解析更新目标（2026-07-14）

- `deploy/run.sh update` 与 `force-update` 在未显式设置 `PANEL_VERSION` 时，会先从项目最新正式 GitHub Release 解析精确版本，再按该版本生成 ACR、1ms、DaoCloud、GHCR、Docker Hub 候选；不再优先拉取 `.env` 中保存的旧 `PANEL_IMAGE`。
- 启动、停止、重启、状态等日常操作仍使用已安装的固定镜像，不会隐式升级。若最新正式版本无法确认，更新操作直接失败并保留当前容器，不会把重新拉取旧 tag 报告成更新成功。
- 运维仍可用 `PANEL_VERSION=x.y.z bash run.sh update|force-update` 显式选择精确稳定版本。发布门禁执行 `scripts/tests/test_run_sh_update.sh`，覆盖旧 `.env` 自动切换、显式版本优先和解析失败拒绝三条路径。

# PANEL-0.2.7 运行组件升级可见性与验收修复（2026-07-14）

- `v0.2.7` 修复 Junimo `.121/.125` 镜像没有 `wget` 时，新版本验收与旧版本回滚均被错误判定为超时的问题；server health 改为复用镜像已有的 Bash `/dev/tcp` 契约。
- Junimo dry-run/apply 现在输出镜像层下载进度，并在 `rollback_failed` 同时保留初始失败和具体回滚失败步骤；维护卡片直接展示校验、下载、安装与验收，技术字段继续收进开发者详情。
- `run.sh update/force-update` 未显式指定版本时解析最新正式 GitHub Release，再生成精确镜像候选；无法确认最新版本时安全终止，不再重新拉取 `.env` 中的旧 tag。
- 本版本不改变 `.125` 推荐矩阵、`.121` 非强制升级语义、镜像候选顺序或 23 个 init 兼容挂载。发布继续由 tag workflow 执行完整矩阵、远程制品、Go/Docker integration、前端状态脚本和生产构建门禁。

# PANEL-0.2.6 升级镜像候选回退说明（2026-07-14）

- `v0.2.6` 将 Junimo server 和 steam-auth-cn 的升级矩阵候选扩展为安装流程的完整同序列表，解决国内环境只能直连 Docker Hub 拉取 `.125` 的问题。
- server canonical digest 仍为 `sha256:10f438581d741fc146ce710cbe20099475ac68908e99f565cf449f0b8192ccf6`；auth canonical digest 仍为 `sha256:99420ab30c09da019c425defd4d01796403ac03898ed261b9ee2a976f4bc6518`。每个别名必须与对应 canonical digest 完全一致。
- release gate 严格验证 Docker Hub canonical 和自有 ACR/GHCR；第三方代理临时不可达只产生 warning，可访问但 digest 不一致仍直接失败。运行时按候选顺序安全回退。
- 发布不改变 `.121` 可继续使用的语义，不自动执行预检/升级，也不改变 23 个 init 兼容挂载。

# PANEL-0.2.5 历史版本闪回修复说明（2026-07-14）

- `v0.2.5` 修复真实 `/api/version=0.2.4` 被旧 `apply-status.json` 中 `toVersion=0.2.2` 覆盖的问题；不删除历史状态、备份或日志。
- 当前版本以运行中镜像的后端版本为准，历史终态只有与当前版本相关时才影响主状态；活动升级与 rollback_failed 安全语义不变。
- 已安装 `0.2.4` 但出现版本闪回的实例需要再通过 `PANEL_VERSION=0.2.5 bash run.sh force-update` 过渡一次。进入 `0.2.5` 后，旧状态加载不会再倒写版本。
- tag `v0.2.5` 由标准 release workflow 执行完整门禁并发布三仓库精确版本及 latest。

# PANEL-0.2.4 连续 Web 升级修复说明（2026-07-14）

- `v0.2.4` 修复 `0.2.1 → 0.2.2` 成功记录覆盖后来 `0.2.3/0.2.4` 检测结果的问题；升级状态文件和备份仍保留，前端只调整当前状态的选择与安全门禁。
- 新目标必须重新运行精确目标版本的 dry-run，旧目标预检不得复用；活动升级和 `rollback_failed` 继续阻止新任务。
- `0.2.2/0.2.3` 已受旧成功记录影响的实例无法从旧前端直接进入下一次 Web 升级，需要通过现有 `run.sh` 指定 `PANEL_VERSION=0.2.4` 完成一次过渡。进入 `0.2.4` 后，后续连续 Web 升级恢复正常。
- tag `v0.2.4` 继续由 `.github/workflows/release.yml` 执行完整发布门禁，并发布 Docker Hub、ACR、GHCR 的 `0.2.4` 与 `latest`。

# PANEL-0.2.3 服务器健康页发布说明（2026-07-14）

- `v0.2.3` 发布用户视角重构后的“服务器健康”页：默认只展示整体结论、可处理的版本维护、检查结果和资源情况；镜像、Driver、digest/buildid、兼容矩阵、预检及升级日志统一收进默认折叠的维护详情。
- 本版本不改变 Junimo 推荐矩阵、镜像目标或升级状态机，继续推荐 `sdvd/server:1.5.0-preview.125`；`.121` 实例不强制升级，页面明确提示“不升级仍可继续使用”。
- tag 仍使用 `v0.2.3` 触发 `.github/workflows/release.yml`，由 release gate 验证远程组件溯源、后端与 Docker integration、全部前端状态脚本和生产构建，再发布 Docker Hub、ACR、GHCR 的 `0.2.3` 与 `latest` 镜像并创建正式 GitHub Release。

# PANEL-0.2.2 / JUNIMO-125 发布说明（2026-07-14）

- `v0.2.2` 内嵌推荐矩阵固定 `sdvd/server:1.5.0-preview.125@sha256:10f438581d741fc146ce710cbe20099475ac68908e99f565cf449f0b8192ccf6` 与现有 auth-cn `1.5.0-anxi.2`。release gate 必须执行远程 digest/auth 溯源校验。
- `.121`→`.125` 是管理员自愿的实例级升级，不与 Panel 镜像更新捆绑强制执行；新安装默认 `.125`，旧实例继续运行并显示推荐提示。
- `.125` 实镜像仍存在 `/etc/cont-*.d` 裸静态值问题，23 个 init 兼容 bind mount 不得删除。已用实际 `.125` 镜像确认兼容脚本可执行并输出预期静态值。
- 上游 steam-service 在 `.121`→`.125` 间无代码变化，auth-cn 镜像 tag/digest/upstreamRef/sourceRevision 保持不变；`server-settings.json` 字段也未变化。

# JUNIMO-STACK-UPDATE-1 阶段二构建与 Docker 边界（2026-07-13）

- Panel 镜像仍内置同一推荐清单，不查询 latest，也不改 Panel updater/发布流程。预检只执行 Docker/Compose version/ps/config、image inspect/pull、volume inspect；目标 Compose 验证用进程级两项镜像环境覆盖，不写 env 文件。
- Panel 看不到 volume mountpoint 或 Docker 数据盘精确空间时只返回 warning，不扩大宿主机挂载、不读取 token、不伪造数值。
- 阶段二没有 `compose up/down/restart/rm/stop`、容器/volume 删除、认证备份或数据卷修改；部署无需新增 volume，尤其不应直挂 `/var/lib/docker/volumes`。

# JUNIMO-STACK-UPDATE-1 阶段一构建与镜像边界（2026-07-13）

- `backend/internal/games/stardew_junimo/config/runtime_stack_manifest.json` 通过 `go:embed` 编入 Panel 二进制，推荐版本随 Panel 构建固定发布；运行时不访问远程 latest。构建/发版复核必须确认 server `1.5.0-preview.121` 与 steam-auth-cn `1.5.0-anxi.2` 作为同一版本对通过测试。
- 本阶段没有新增镜像拉取、registry 凭据、Compose 操作或部署环境变量；检测只读实例 `.env`。Panel 自身 `/api/system/update` 镜像升级链路与实例级 Junimo 检测保持完全独立。
- 阶段一不允许通过 API 指定镜像/tag/digest/registry，也不修改实例 `.env`、不停止/重建 server 或 steam-auth。未来阶段如增加执行能力，必须另行补 capability、可信候选拉取、配置备份、原子版本对切换、健康验收和自动回滚。

# PANEL-UPDATE-RELEASE-1 镜像与发布验收（2026-07-13）

- v0.2.0 是首个包含完整 Web updater 的正式版本；Tag 推送后由现有 GitHub tag workflow 构建并发布精确版本镜像。v0.2.0 之前的安装需要先用原部署更新方式完成一次引导升级。
- 隔离构建了旧版 `0.1.13`、目标版 `0.1.14` 与故意 unhealthy/写库的失败镜像，完成真实 Compose 成功替换和自动回滚；测试项目、端口、数据目录与现有部署完全隔离。
- 由于本阶段明确不打 tag/不推镜像，E2E 通过测试专用 wrapper 对已预置本地的精确可信镜像跳过远端 pull。生产镜像和代码仍执行 `docker compose ... up -d --pull always --force-recreate --no-deps panel`；正式仓库拉取闭环须在版本号确认并发布镜像后复验。
- helper 将宿主安装目录挂载到相同绝对路径，Compose config/file/working_dir labels 在升级后仍指向宿主真实路径；禁止恢复为 `/deployment` 固定挂载。
- `scripts/smoke-test.ps1` 已补 UTF-8 BOM 兼容 Windows PowerShell 5.1、修正 `BUILD_DATE` 参数传递，并在镜像构建失败时跳过依赖的容器健康步骤，避免误拉同名远端镜像。fresh named volume、`/health`、`/api/version` smoke 已通过。
- 首次升级兼容：已安装本次 updater 版本后，后续升级可全程 Web 完成；从尚未包含 updater 的历史发布进入首个 updater 发布，需使用该历史版本既有的部署更新流程完成一次引导升级。之后不再要求日常升级使用 SSH/run.sh。

# FE-PANEL-UPDATE-1 前端发布与恢复检查

- 正式镜像必须将同一构建版本注入 panel，并包含最新前端 bundle。升级恢复依赖公开 `/health` 返回 `{status:"ok"}`、`/api/version.version` 返回精确目标版本，以及登录后的 apply 状态接口；反向代理不得把这三个路径改写为 SPA HTML。
- 页面不依赖 SSH、run.sh 或 Docker 命令。断线期间浏览器保留当前 URL 和已加载的 JS/CSS，面板恢复后自动继续；发布时不要给 HTML/API 配置会跨版本保留的强缓存。
- 发布 QA 至少运行 `npm run test:panel-update`、`npm run test:update-status`、`npm run build`，并用 `qa-layout.html?update=available&apply=offline` 与 `apply=reconnect-success` 检查专用离线页和结果弹窗。桌面 1280、窄屏 900、移动 390 宽度均需检查顶栏不溢出。

# PANEL-UPDATE-APPLY-1 镜像升级与回滚约定

- `/app/panel-updater` 现支持固定 `apply` 子命令。面板以当前可信镜像 detached 启动 helper，挂载 Docker Socket、部署目录（apply 时可写）和 panel 数据目录；当前 panel 停止后 helper 继续运行。
- 正式升级目标始终是构建版本对应的精确 tag，不使用 `latest`。helper 依序尝试硬编码 Docker Hub、ACR、GHCR 候选，记录最终镜像 ID；Compose 命令固定 project/config/env-file，并以 `--no-deps panel` 限定服务。
- 标准 `deploy/run.sh` 部署须保留 `PANEL_HOST_INSTALL_DIR/PANEL_HOST_COMPOSE_FILE/PANEL_HOST_DATA_DIR/PANEL_COMPOSE_PROJECT`，以及 Docker Socket。缺失或无法安全识别的部署保持 unsupported，不尝试猜测宿主路径。
- 升级备份在 panel 数据目录 `updater/backups/<updateId>`，不进入镜像层、支持包或下载接口。发布镜像必须同时包含 docker-cli、Compose plugin、`wget`、panel 和 panel-updater，且 `/api/version` 必须返回精确构建版本，否则升级会自动回滚。
- 发布前除常规镜像构建外，执行 `PANEL_RUN_DOCKER_UPDATE_TEST=1 go test ./internal/updater -run TestDockerIntegrationApplyUsesIsolatedComposeProject`；该测试只创建随机临时 Compose project/镜像，禁止指向生产 panel-data。

# PANEL-UPDATER-DRYRUN-1 镜像与 run.sh 约定

- Panel 镜像现在同时构建 `/app/panel` 和独立 `/app/panel-updater`；运行层继续包含 docker-cli 与 docker-cli-compose。helper 通过覆盖 ENTRYPOINT 启动 updater，不复用面板 HTTP 进程。
- `run.sh` 写入并传入 `PANEL_HOST_INSTALL_DIR`、`PANEL_HOST_COMPOSE_FILE`、`PANEL_HOST_DATA_DIR`、`PANEL_COMPOSE_PROJECT`，作为 Compose labels 不可用时的严格兜底；Compose 命令统一使用 PANEL_COMPOSE_PROJECT。
- helper 只挂载 Docker Socket、部署目录（只读）和数据目录；状态写在数据目录 `updater/status.json`。不得挂载宿主机根目录、用户 HOME、Docker credential 目录或额外配置目录。
- dry-run 镜像仓库白名单与 run.sh 正式候选保持一致：项目 ACR、GHCR、Docker Hub、1ms 和 DaoCloud Docker Hub 镜像；只允许精确稳定版本 tag，禁止 latest 和用户提交仓库。
- 本阶段镜像行为仅增加 inspect/pull/config 校验，不执行 compose up/down、容器 stop/rm/restart，也不改变发布 tag 流程。

# PANEL-UPDATE-CHECK-1 构建版本与发布约定

- 面板更新检测继续使用现有构建参数 `version`、`commit`、`buildDate` 注入；正式镜像必须注入合法稳定语义版本（可带 `v` 前缀）。未注入时的 `dev`、空值或非法版本只显示“版本检测不可用”，不会误报可更新。
- GitHub Release 必须是非 draft、非 prerelease 的正式 Release 才会参与比较；Release tag 应使用可解析的语义版本。
- 本阶段不会拉取新镜像、替换容器、重启面板或操作数据库，也不改变现有镜像发布流程。后续升级执行必须单独设计和验证。

# RUN-SH-DOCKER-APT-FALLBACK-1 Docker APT 源自动切换

- `deploy/run.sh` 的 Docker/Compose 自动安装在 apt 系系统上不再只依赖阿里云 Docker CE 源。
- 脚本会先获取 Docker APT GPG key，然后按顺序尝试 Docker CE apt 源：阿里云、清华 TUNA、中科大 USTC、Docker 官方源。
- 脚本现在只写入托管源 `/etc/apt/sources.list.d/anxi-panel-docker.list`；进入安装前会扫描 `/etc/apt/sources.list` 和 `/etc/apt/sources.list.d/`，把历史残留的 Docker CE 源行注释掉，Deb822 `.sources` 源文件会改名停用并留下 `.anxi-panel-bak` 备份。
- 每次切换 Docker CE apt 源前会清理 `/var/lib/apt/lists/` 下的 Docker 源索引，避免镜像站同步期间出现 `File has unexpected size ... Mirror sync in progress?` 后继续复用坏源或坏索引。
- 现场如果仍失败，通常说明服务器无法访问所有候选 Docker 源；可稍后重试 `bash run.sh docker`，或手动安装 Docker Engine 与 Docker Compose plugin 后再执行 `bash run.sh install`。

# JUNIMO-STATIC-INIT-FIX-1 JunimoServer 镜像启动兼容

- 上游 `sdvd/server:1.5.0-preview.121` 与 `1.5.0-preview.125` 镜像在 `/etc/cont-env.d`、`/etc/cont-groups.d`、`/etc/cont-users.d` 内仍会出现裸静态值，当前 init 会把它们当 shell 命令执行。真实失败可表现为 `DockerApp: not found`、`unix:path=/tmp/dbus.base: not found`、`linux/amd64: not found`、`72: not found`。
- 面板不再要求用户使用本地热修 server 镜像；实例目录会自动生成 `.local-container/cont-env/*`、`.local-container/cont-groups/*`、`.local-container/cont-users/*` 脚本，并 bind mount 到 server 容器内覆盖对应静态值文件。
- 该修复不改变 `SERVER_IMAGE` / `SERVER_IMAGE_CANDIDATES` 的选择逻辑，也不会影响镜像拉取兜底。离线部署时只需保证 panel 镜像更新到包含本修复的版本。
- 排查命令：`grep -n "cont-env\\|cont-groups\\|cont-users" /path/to/instance/docker-compose.yml`，以及查看实例目录 `.local-container/cont-env/`、`.local-container/cont-groups/`、`.local-container/cont-users/`。

# INVITE-BACKGROUND-POLL-1 启动不阻塞邀请码

- 面板镜像包含本次生命周期行为：启动/重启只负责把 server 拉起，邀请码在后台最多探测 20 次。探测失败不影响 IP 直连，不会关闭 server。
- 部署覆盖镜像后，旧实例无需手动进服务器修复；下一次启动/重启会自动清理旧 SMAPI `status.json` / `players.json` 快照，并使用新的后台邀请码探测逻辑。
- 前端通过 `/api/instances/:id/state.inviteCode` 接收后端后台探测到的邀请码；因此覆盖镜像时必须同时包含本次后端与前端构建产物。

# SMAPI 运行环境预安装

- 面板镜像本身不内置 SMAPI。安装 Stardew 时，后端会在游戏文件和 Steam SDK 完成后，用已选择的 JunimoServer 镜像启动一次性 `docker run --rm` 容器，挂载 `<project>_game-data:/data/game` 并安装 SMAPI。
- 这不是新增常驻容器，也不需要用户开放新端口；容器运行完自动删除。目的是稳定访问 Docker named volume，并复用 JunimoServer 镜像里的 Linux 运行环境。
- `.env` 虽保留 `SMAPI_VERSION` / `SMAPI_DOWNLOAD_URLS` 兼容字段并写入与 embed 相同的固定加速顺序，但 installer 下载只取 Panel embed 清单，运行时覆盖不会改变选择。每个受审候选使用 Range 分块续传并做最终大小/SHA/ZIP 校验。
- 离线/企业部署如需完全避免现场 GitHub，应在部署前把经同一大小、SHA-256 与 ZIP 结构校验的 installer 放入实例私有 `.local-container/smapi-update/packages/SMAPI-<version>-installer.zip`；不要通过 `.env` 引入未审核下载域名。

# ENV-BOM-NORMALIZE-1 Compose 启动前配置校验

- 实例 `.env` 若被外部编辑器或复制粘贴插入 UTF-8 BOM 前缀，Docker Compose 会在解析阶段报 `unexpected character "\ufeff"`，表现为面板任务只显示 `docker compose up: docker command failed`。
- 后端已在 `.env` 读取/写回时归一化 BOM 前缀 key；部署排障时仍建议先执行 `docker compose -f /data/instances/<id>/docker-compose.yml config --quiet`，确认不是配置文件解析失败。
- 支持包和日志不要直接贴出 `.env` 敏感值；排查 BOM 只需要确认是否存在隐藏前缀 key。

# STEAMCMD-SELFUPDATE-CACHE-1 兜底容器自更新缓存

- SteamCMD 镜像本身仍按 `STEAMCMD_IMAGE_CANDIDATES` 选择和拉取；本地已有候选镜像时不会重新 pull。
- 镜像启动后 SteamCMD 可能输出 `[----] Downloading update (.. of 40,273 KB)`，这是容器内 SteamCMD 客户端更新，不是镜像源下载。面板日志会明确区分这一步。
- SteamCMD 客户端自更新目录已持久化到实例命名卷：`<project>_steamcmd-root-local:/root/.local/share/Steam` 和 `<project>_steamcmd-user-local:/home/steam/.local/share/Steam`。后续重试授权/下载应复用该缓存，减少重复 40MB 自更新。
- 离线或预热部署仍建议预先准备 `STEAM_SERVICE_IMAGE`、`sdvd/server:<IMAGE_VERSION>` 以及 `STEAMCMD_IMAGE_CANDIDATES` 中至少一个可用 SteamCMD 镜像。

# STEAMCMD-RETRY-RESUME-1 本地镜像优先

- SteamCMD 兜底镜像选择现在先 inspect 完整 `STEAMCMD_IMAGE_CANDIDATES` 列表；只要任意候选镜像已在本机 Docker 中存在，就直接用于 SteamCMD 兜底容器，不会先尝试拉取排在它前面但本地缺失的候选。
- 这意味着用户已经成功拉完 `docker.1ms.run/steamcmd/steamcmd:latest` 或其他候选后，后续因 Steam Guard 手机批准超时而重试安装时，会直接进入 SteamCMD 登录授权环节，不会重复下载该镜像。
- 如果所有候选镜像都不存在，仍按候选顺序依次 pull，并通过 `steamcmd_image_pulling` phase 和 `[pull:progress:done:total]` 日志给前端估算进度。

# 镜像构建文档

## SteamCMD 兜底镜像

- 面板运行镜像本身仍是单 Panel Docker 镜像，但安装 Stardew 时可能额外拉取 SteamCMD 作为 steam-auth 下载失败后的兜底工具镜像。
- 默认值在实例 `.env` 中写入：`STEAMCMD_IMAGE=docker.1ms.run/steamcmd/steamcmd:latest`，`STEAMCMD_IMAGE_CANDIDATES=docker.1ms.run/steamcmd/steamcmd:latest,docker.m.daocloud.io/steamcmd/steamcmd:latest,ghcr.io/steamcmd/steamcmd:latest,cm2network/steamcmd:latest`。后端会按候选列表逐个 `inspect/pull`，前一个镜像源 403 或超时后继续尝试下一个；旧实例如果仍是旧候选列表，安装时会补齐新候选并过滤直连 Docker Hub 的 `steamcmd/steamcmd:latest` 和已移除的 `docker.xuanyuan.me/steamcmd/steamcmd:latest`。单次镜像拉取默认等待 30 分钟，避免大镜像在慢链路下已经拉完层但尚未返回成功就被误判超时。
- SteamCMD 镜像不是 `docker-compose.yml` 里的 Junimo service；后端通过 Docker CLI/API 临时运行 TTY 容器，并挂载 `game-data`、`steamcmd-login`、`steamcmd-home` 命名卷。`steamcmd-login` 是 SteamCMD 专属统一授权卷，会同时映射到 root/steam 两种候选镜像的 `Steam` 与 `.local/share/Steam` 路径；它与 Junimo `steam-auth` 的 `steam-session` 卷、refresh token 完全独立。镜像缺失时会先执行单镜像拉取；候选全部失败时安装 phase 为 `steamcmd_image_pull_failed`。
- 发布/升级不能主动清理 `<instance>_steamcmd-login`。SteamCMD 退出码 139 的自动重试也只清理占用容器，不删除该授权卷，否则会丢失已批准设备身份并再次触发 Steam Guard。旧版本创建的 `steamcmd-user-local` / `steamcmd-root-local` 卷升级后不再作为运行目录挂载，也不自动删除；SteamCMD 启动前会把旧卷已有 `config/` 与 `ssfn*` best-effort 迁入空的统一卷，迁移源只读、目标已有缓存不覆盖。
- 发布或离线部署时，如果希望完全避免现场拉取，需要预先准备 `STEAM_SERVICE_IMAGE`、`sdvd/server:<IMAGE_VERSION>` 和 `STEAMCMD_IMAGE_CANDIDATES` 中至少一个可用的 SteamCMD 镜像。

## 构建目标

项目发布为单个 Panel Docker 镜像，镜像内包含：

- Go 后端二进制。
- React/Vite 构建产物并嵌入后端。
- docker CLI。
- docker compose plugin。
- 必要 CA、时区和运行工具。

运行时通过挂载宿主机 Docker Socket 控制 JunimoServer 容器。

## 构建上下文排除

- `.dockerignore` 已显式排除 `docs/prototypes/`，历史原型图不应进入 Docker 构建上下文或镜像产物。
- 当前 Dockerfile 也采用精确 `COPY frontend`、`COPY backend`、`COPY browser-extensions` 的方式，不依赖 `COPY .`。后续如调整 Dockerfile，仍需确认文档、原型图、本地构建产物、`node_modules` 不会进入运行镜像。

## 构建镜像

```powershell
cd E:\stardew-server-anxi-panel
docker build -t stardew-server-anxi-panel:local .
```

多阶段流程：

1. `frontend-builder`: `node:22-alpine`，执行 `npm install` 和 `npm run build`。
2. `extension-builder`: `alpine:3.20`，安装构建期 `zip`，把 `browser-extensions/nexus-slow-installer` 预打包为 `browser-extensions/anxi-nexus-installer.zip`。
3. `backend-builder`: `golang:1.25-alpine`，复制前端 dist 到 `internal/static/frontend_dist/`，`CGO_ENABLED=0 go build`。
4. `runtime`: `alpine:3.20`，只安装 docker CLI / compose plugin、CA 与时区数据，复制 `/app/panel` 和 extension-builder 的浏览器扩展产物。

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

## NAS / 图形化 Compose 部署

- NAS（飞牛、群晖、绿联、威联通等）用户可通过图形化 Docker / Container Manager / Compose / 项目 / 应用栈部署，不要求使用 `run.sh`。
- 面板容器必须挂载宿主机 Docker Socket：`/var/run/docker.sock:/var/run/docker.sock`。如果 NAS 图形界面禁止挂载 Docker Socket，面板无法继续创建 JunimoServer、SteamCMD 等游戏容器。
- NAS 部署推荐使用宿主机真实绝对路径挂载数据目录，并让容器内 `PANEL_DATA_DIR` 与宿主机路径保持一致。例如：

```yaml
services:
  anxi-panel:
    image: crpi-9z3bkb9g7fxeohrg.cn-hangzhou.personal.cr.aliyuncs.com/anxi-panel/stardew-server-anxi-panel:latest
    container_name: anxi-panel
    restart: unless-stopped
    ports:
      - "8090:8090"
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock
      - /vol1/1000/docker/anxi-panel/data:/vol1/1000/docker/anxi-panel/data
    environment:
      PANEL_ADDR: ":8090"
      PANEL_DATA_DIR: "/vol1/1000/docker/anxi-panel/data"
      PANEL_DB_PATH: "/vol1/1000/docker/anxi-panel/data/panel.db"
      PANEL_MODE: "single"
      PANEL_SECRET: "please-change-to-a-long-random-string"
```

- 上例中的 `/vol1/1000/docker/anxi-panel/data` 只是示例路径，实际部署时必须替换成 NAS 图形界面显示的宿主机绝对路径，并保持 volume 左右路径和 `PANEL_DATA_DIR` 一致。
- 上例只在 `anxi-panel` 服务里绑定 `8090`，因为它只是面板容器。`24642/udp`、`27015/udp` 和 `5800/tcp` 由面板后续创建的 JunimoServer 游戏容器绑定，不要写进 `anxi-panel` 的 `ports`，否则面板容器会提前占用游戏端口。
- NAS 防火墙/路由器端口：面板 `TCP 8090`，游戏 `UDP 24642`，查询 `UDP 27015`，VNC `TCP 5800` 按需；不要开放 `TCP 8080`。
- 低配 NAS 口径：i3 M380 / 2 核 4 线程 / 6 GB DDR3 / HDD 可跑 1-2 人自用，3-4 人原版或少量 Mod 可尝试，5 人以上或大量 Mod 不建议。

## 系统要求与安全组

最低系统要求：

```text
主要部署环境：Linux x86_64
发行版：Ubuntu 20.04+ / Debian 11+ / CentOS 8+ / Rocky Linux 8+ / AlmaLinux 8+ / Alibaba Cloud Linux 3+
Docker：Docker Engine 24+
Compose：Docker Compose plugin v2+
CPU：2 核
内存：2 GB
磁盘：20 GB 可用空间
网络：公网 IP
端口：TCP 8090，UDP 24642 / 27015
```

推荐配置：

```text
系统：Ubuntu 22.04 LTS / Ubuntu 24.04 LTS / Debian 12 / Alibaba Cloud Linux 3
CPU：2 核以上
内存：4 GB 以上
磁盘：40 GB SSD 以上
带宽：5 Mbps 以上
Docker：Docker Engine 25+ / 26+ / 27+
```

Windows 支持边界：

- 支持 `x86_64` Windows 10/11 + WSL2 + Docker Desktop，以 Linux containers 方式运行 Panel 与 JunimoServer；这也是本项目在 Windows 上开发、构建镜像和执行 Docker 真机门禁的环境。
- Docker Desktop 必须使用 WSL 2 based engine，并为实际运行命令的 WSL2 发行版启用 WSL Integration。先在 WSL2 终端确认 `docker version` 与 `docker compose version`，再运行 Linux 部署脚本。
- 建议将安装目录和 bind-mounted 数据保存在 WSL2 Linux 文件系统的 `~/.anxi-panel`，不要放在 `/mnt/c`；Docker 官方也建议 Linux 容器 bind mount 使用 Linux 文件系统以获得更好的性能。
- 当前不提供原生 Windows `.exe`、Windows Service 或 Windows containers 镜像。Docker Desktop 退出、Windows 注销/更新/休眠会中断服务，因此长期 24 小时部署仍优先推荐 Linux 云服务器或 NAS。
- README 的产品截图 `docs/screenshots/anxi-panel-overview-v0.4.3.png` 来自 Docker Desktop 29.5.3 运行正式 `v0.4.3` 镜像、完成隔离管理员初始化后的真实未安装总览；不得再以 `docs/prototypes` 中的设计/实现基准图冒充实际运行截图。

多人游玩推荐：

```text
1-2 人：2 核 2 GB，建议开启 2 GB swap
3-4 人：2 核 4 GB
5-8 人：4 核 8 GB
大量 Mod：4 核 8 GB 起步，磁盘 60 GB+
```

云服务器安全组：

```text
必须开放：
TCP 8090
UDP 24642
UDP 27015

按需开放：
TCP 5800

不要开放：
TCP 8080
```

`TCP 8080` 是 Junimo API，供面板和容器网络内部访问，不需要也不建议公网开放。

## 一键启动脚本（推荐给用户）

面向普通 Linux 云服务器用户，优先推荐使用 `deploy/run.sh` 的快速模式。Windows 用户可在已经启用 Docker Desktop WSL Integration 的 WSL2 Linux 终端运行同一脚本，不能从 PowerShell 直接执行。默认部署方式是公网 IP + `8090` 端口直接访问面板；Windows 本机访问使用 `http://localhost:8090`。脚本会在用户主目录生成运行目录：

```text
~/.anxi-panel
├─ .env
├─ docker-compose.yml
└─ data/
```

默认行为：

- 默认面板端口：`8090`。
- 默认访问方式：`http://服务器IP:8090`。
- 默认镜像 tag：`latest`，便于新用户快速启动；正式服可通过 `PANEL_VERSION=0.1.0` 固定版本。
- 首次启动时会选择镜像源：自动候选、国内阿里云 ACR、Docker Hub 加速链路、DaoCloud 加速链路、GitHub GHCR、Docker Hub 官方，或自定义完整镜像地址；默认推荐自动候选。
- 面板镜像拉取复用后端候选镜像思路：先检查本地是否已有任意候选镜像；本地没有时按候选顺序逐个 `docker pull`，第一个成功的镜像会写回 `~/.anxi-panel/.env` 的 `PANEL_IMAGE`。
- 自动生成强随机 `PANEL_SECRET` 并写入 `~/.anxi-panel/.env`。
- 使用宿主机目录 `~/.anxi-panel/data` 持久化面板数据，并把容器内 `PANEL_DATA_DIR` 设置为同一个绝对路径，确保面板容器通过宿主机 Docker socket 编排游戏容器时，bind mount 路径在宿主机和面板容器中一致。
- 挂载 `/var/run/docker.sock`，让面板继续按现有设计控制 JunimoServer 容器。
- NAS 或特殊 Linux 环境中，如果 `$HOME` 不存在或不可写，默认安装目录会回退到当前可写目录下的 `.anxi-panel`，避免飞牛等系统中 `/home/<user>` 不存在时 `mkdir` 失败。用户也可以显式设置 `INSTALL_DIR=/vol1/1000/docker/.anxi-panel` 指定安装目录。
- 菜单 `[9] 设置虚拟内存` 会优先通过 `/proc/swaps` 判断 `/swapfile` 是否已启用，并兼容 `swapon` / `mkswap` 位于 `/sbin` 或 `/usr/sbin` 的 NAS 环境；如已有 `/swapfile` 但未启用，会先尝试移除后重建，避免直接覆盖导致 `Text file busy`。

用户首次启动：

国内加速安装：

```bash
curl -fsSL -o run.sh https://anxinas.dpdns.org/run.sh && chmod +x run.sh && bash run.sh
```

GitHub Release 安装：

```bash
curl -fsSL -o run.sh https://github.com/anxiyizhi/stardew-server-anxi-panel/releases/latest/download/run.sh && chmod +x run.sh && bash run.sh
```

如果直接从仓库文件启动：

```bash
cd deploy
chmod +x run.sh
bash run.sh
```

菜单入口：

```text
[0] 拉取并启动面板
[1] 安装/修复 Docker 与 Compose
[2] 启动/恢复面板
[3] 停止面板
[4] 重启面板
[5] 更新面板镜像并重建容器
[6] 强制更新面板镜像
[7] 切换镜像源/加速节点
[8] 更新 run.sh 启动脚本
[9] 设置虚拟内存
[10] 设置开机自启
[11] 查看面板状态
[12] 查看面板日志
[13] 显示访问地址
[14] 退出
```

非交互命令：

```bash
bash run.sh install
bash run.sh stop
bash run.sh restart
bash run.sh update
bash run.sh status
bash run.sh logs
bash run.sh docker
bash run.sh force-update
bash run.sh switch-image
bash run.sh update-script
bash run.sh swap 2
bash run.sh autostart
```

更新面板：

```bash
cd ~ && bash run.sh update
```

如果更新后仍显示旧版本，强制重新拉取镜像并重建容器：

```bash
cd ~ && bash run.sh force-update
```

如果启动脚本本身也有更新，先更新脚本再更新面板：

```bash
cd ~ && bash run.sh update-script
cd ~ && bash run.sh update
```

更新面板只会重建面板容器，不会删除 `~/.anxi-panel/data`，存档、Mod、数据库和备份会继续保留。

固定版本启动示例：

```bash
PANEL_VERSION=0.1.0 PANEL_PORT=8090 bash run.sh install
```

改用 Docker Hub 优先：

```bash
DEFAULT_MIRROR=dockerhub bash run.sh install
```

改用 GitHub GHCR：

```bash
DEFAULT_MIRROR=ghcr bash run.sh install
```

注意：

- 脚本支持自动安装/修复 Docker Engine 与 Docker Compose plugin。Ubuntu/Debian 使用阿里云 Docker CE apt 源；CentOS/RHEL/Anolis/Rocky/Alibaba Cloud Linux 类系统使用阿里云 Docker CE yum/dnf 源。无法识别的发行版仍需手动安装 Docker。
- 如果云服务器外部无法访问面板，优先检查安全组/防火墙是否放行 TCP `8090`。
- Stardew 游戏本身还需要按实例配置放行 UDP `24642` / `27015`；VNC/noVNC 默认 `TCP 5800`，仅需要浏览器查看游戏画面时按需放行；`TCP 8080` 是 Junimo API，不要开放公网。
- 快速模式默认使用 HTTP 明文访问，适合用户自有云服快速开服；首次进入面板后必须设置强管理员密码，不要使用默认或弱密码。
- 不要手动删除 `~/.anxi-panel/data`；该目录保存面板数据库、实例 compose、存档、mod、备份和审计日志。

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

一键脚本默认把宿主机 `~/.anxi-panel/data` 挂载到容器内同名绝对路径，保证容器重建后数据不丢，同时让宿主机 Docker daemon 能解析游戏实例的 bind mount 路径。

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
- 在 Mod 下载页用管理员账号配置 Nexus API Key 后，数字 ID 精确查询可用；未配置时返回 `nexus_api_key_missing` 而不是 500。普通关键词搜索不要求 Key。
- 健康检查和支持包导出可用且脱敏。
- 320px 以上窄屏无横向溢出。

## 安全说明

挂载 Docker Socket 等同于授予面板容器高权限 Docker 控制能力。当前用户入口按快速模式设计，默认通过 `http://服务器IP:8090` 直接访问。上线说明里应强调：

- 这是给用户自有云服开游戏服务器的管理面板，不建议多人共用同一台宿主机。
- 使用强 `PANEL_SECRET`。
- 初始化管理员必须使用强密码。
- 只放行必要端口：面板 TCP `8090`，游戏 UDP `24642` / `27015`，VNC/noVNC 默认 TCP `5800` 按需放行；不要开放 Junimo API 的 TCP `8080`。
- 定期查看审计日志。
- 支持包和日志确认无密码、token、session、邀请码明文。

## 常见问题

面向普通用户的完整版本见 [故障排查指南](user-guide/troubleshooting.md)；本节只保留和镜像构建/本地开发直接相关的条目。

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
# NEXUS-EXT-PACK-1 镜像内扩展资源

- Runtime 镜像现在会从 `extension-builder` 复制 `browser-extensions/` 到 `/app/browser-extensions/`。
- `anxi-nexus-installer.zip` 在 `extension-builder` 阶段生成；最终 runtime 不再安装 `zip`，也不在运行层执行打包命令。
- 后端 `GET /api/instances/:id/mods/nexus/extension/download` 会优先返回实例目录已有的 `.local-container/browser-extensions/anxi-nexus-installer.zip`；不存在时优先复制镜像预包；预包不存在或损坏时，才从 `/app/browser-extensions/nexus-slow-installer` 或开发环境仓库路径生成。
- 发布检查新增注意：正式镜像内应存在 `/app/browser-extensions/anxi-nexus-installer.zip`；兜底源码目录 `/app/browser-extensions/nexus-slow-installer/manifest.json` 也应保留，避免预包损坏时无法恢复。
# PULL-PROGRESS-1 镜像拉取百分比

- 拉取过程中，后端会把 Docker 输出折算成估算百分比：compose pull 按服务镜像完成数估算，SteamCMD 单镜像 pull 按 layer 完成数估算，并通过 job 日志隐藏标记 `[pull:progress:done:total]` 供前端展示。
# JUNIMO-IMAGE-CANDIDATES-1 运行期 Junimo 镜像候选

- 安装 Stardew 时，面板运行镜像会额外拉取/使用 `steam-auth-cn` 与 `JunimoServer server` 运行期镜像。二者已支持候选兜底，不再只依赖 `docker compose pull` 的单一源。
- 默认 `SERVER_IMAGE_CANDIDATES`：`docker.1ms.run/sdvd/server:<IMAGE_VERSION>`、`docker.m.daocloud.io/sdvd/server:<IMAGE_VERSION>`、`ghcr.io/sdvd/server:<IMAGE_VERSION>`、`sdvd/server:<IMAGE_VERSION>`。
- 默认 `STEAM_SERVICE_IMAGE_CANDIDATES`：`docker.1ms.run/anxiyizhi/junimo-steam-service-cn:1.5.0-anxi.2`、`crpi-9z3bkb9g7fxeohrg.cn-hangzhou.personal.cr.aliyuncs.com/anxi-panel/junimo-steam-service-cn:1.5.0-anxi.2`、`docker.m.daocloud.io/anxiyizhi/junimo-steam-service-cn:1.5.0-anxi.2`、`ghcr.io/anxiyizhi/junimo-steam-service-cn:1.5.0-anxi.2`、`anxiyizhi/junimo-steam-service-cn:1.5.0-anxi.2`。
- 离线或内网发布时，可预先 `docker pull` 上述任意候选，或在实例 `.env` 中把可用内网镜像写入 `SERVER_IMAGE_CANDIDATES` / `STEAM_SERVICE_IMAGE_CANDIDATES`。后端会优先复用本地已有候选，并把实际选中项写回 `SERVER_IMAGE` / `STEAM_SERVICE_IMAGE`。
- 注意：`ghcr.io/sdvd/server:*` 与 `ghcr.io/anxiyizhi/junimo-steam-service-cn:*` 只有在对应 GHCR 包真实发布且可公开拉取时才会成功；失败会自动继续后续候选。
# JUNIMO-IMAGE-CANDIDATES-2 Junimo 镜像候选源补齐

- JunimoServer 与 steam-auth cn 镜像不再依赖 `docker compose pull` 的单源解析；后端逐个 `inspect/pull` 候选镜像，成功后写回 `.env` 的 `SERVER_IMAGE` / `STEAM_SERVICE_IMAGE`。
- 旧实例如果已经保存了单值 `SERVER_IMAGE_CANDIDATES` 或 `STEAM_SERVICE_IMAGE_CANDIDATES`，安装流程会自动把默认候选源补到前面并写回 `.env`，避免只尝试 `(1/1)`。
- JunimoServer 默认顺序：`docker.1ms.run/sdvd/server:<IMAGE_VERSION>`、`docker.m.daocloud.io/sdvd/server:<IMAGE_VERSION>`、`ghcr.io/sdvd/server:<IMAGE_VERSION>`、`sdvd/server:<IMAGE_VERSION>`。
- steam-auth cn 默认顺序：`docker.1ms.run/anxiyizhi/junimo-steam-service-cn:1.5.0-anxi.2`、`crpi-9z3bkb9g7fxeohrg.cn-hangzhou.personal.cr.aliyuncs.com/anxi-panel/junimo-steam-service-cn:1.5.0-anxi.2`、`docker.m.daocloud.io/anxiyizhi/junimo-steam-service-cn:1.5.0-anxi.2`、`ghcr.io/anxiyizhi/junimo-steam-service-cn:1.5.0-anxi.2`、`anxiyizhi/junimo-steam-service-cn:1.5.0-anxi.2`。
- 发布或离线部署时，预拉上述任意候选即可；本地已有候选会优先复用，不会因为排在前面的候选缺失而重新拉取。
# RELEASE-TAG-CI-1 GitHub Tag 发版

- 面板仓库新增 `.github/workflows/release.yml`：推送 `v*` tag 时自动构建 `Dockerfile`，并推送到 Docker Hub、阿里云 ACR 与 GitHub GHCR。
- Git tag 使用 `v0.1.0` 形式；Docker 镜像 tag 会去掉前缀 `v`，发布为 `0.1.0`，同时更新 `latest`。
- 发布目标：
  - `anxiyizhi/stardew-server-anxi-panel:<version>` 与 `:latest`
  - `crpi-9z3bkb9g7fxeohrg.cn-hangzhou.personal.cr.aliyuncs.com/anxi-panel/stardew-server-anxi-panel:<version>` 与 `:latest`
  - `ghcr.io/anxiyizhi/stardew-server-anxi-panel:<version>` 与 `:latest`
- GitHub Release 会自动生成 release notes，并上传 `deploy/run.sh`，供用户一键下载启动。
- 仓库 secrets 需要配置：`DOCKERHUB_USERNAME`、`DOCKERHUB_TOKEN`、`ALIYUN_REGISTRY_USERNAME`、`ALIYUN_REGISTRY_PASSWORD`。GHCR 使用 GitHub Actions 自动注入的 `GITHUB_TOKEN`，workflow 需要 `packages: write` 权限；首次发布后如果包是私有，需要在 GitHub Package settings 中改为 Public。阿里云 ACR 新版个人版实例必须使用控制台“访问凭证”里显示的登录名和域名；当前实例域名为 `crpi-9z3bkb9g7fxeohrg.cn-hangzhou.personal.cr.aliyuncs.com`，`ALIYUN_REGISTRY_USERNAME` 填控制台命令 `docker login --username=...` 里的值，例如 `安西义之`。
# REAL-INSTANCE-STEAM-IMAGE-FALLBACK-VERIFIED-1 真实环境验证

- Steam/SteamCMD 授权流程与镜像候选降级已经在真实环境验证通过：候选源不可用时会继续尝试后续镜像，本地已有候选可直接复用，授权状态能够继续安装流程。
- 本标记取代相关历史章节的待验证说明；具体候选顺序与配置方式仍以 `JUNIMO-IMAGE-CANDIDATES-*`、`STEAMCMD-*` 小节为准。
# RELEASE-v0.1.12 新服务器存档页空列表修复

- 发布版本：`v0.1.12`，补丁修复全新实例尚无备份时进入存档页黑屏、无法创建首个存档的问题。
- 发布前验证：后端相关测试、前端生产构建、Docker 镜像构建；推送 tag 后由 `.github/workflows/release.yml` 发布三个镜像仓库的 `0.1.12` 与 `latest`。
- 升级后验证：全新数据目录完成安装但未生成备份时，进入“存档”应显示正常空态和新建入口；`GET .../saves/backups` 应返回 `backups: []`。
# RELEASE-v0.1.13 安装运行文件完整性校验

- 发布版本：`v0.1.13`，修复新服务器 `game-data` 卷仅创建 Steam 目录但无游戏文件时仍显示“安装完成”、允许创建游戏的问题。
- 发布验证：后端 `go test ./...`、前端 `npm run build`、Docker 镜像构建；CI 构建完成后检查三个镜像仓库的 `0.1.13` 与 `latest`。
- 升级验证：现有误判实例刷新面板后应显示“游戏运行文件不完整，请重新安装或修复”；执行安装/修复后，仅在 Stardew、SMAPI 与 Steam SDK 必需文件全部存在时才会进入创建存档流程。
# JUNIMO-STACK-UPDATE-1 阶段三发布/镜像验收（2026-07-13）

- Panel 镜像继续内置唯一 `tested=true` 推荐版本对；阶段三没有改变推荐 tag，也不查询远程 latest。发布新 Panel 前若调整清单，必须对 server/auth pair、Steam ticket、Junimo/SMAPI/邀请码和失败回滚一起验收。
- 真实环境发布门禁：隔离 Compose project、非生产 steam-session/game-data、专用 Steam 测试账号和两个真实上游推荐镜像；覆盖运行/停止实例成功升级、auth/server 故障回滚、认证迁移恢复、Panel 中断恢复，凭据不得进入日志、状态、镜像层或仓库。
- 自动 Docker integration test 使用本机 `alpine:3.20` 和唯一 `anxijunimotest*` 临时 volume，只验证受控 clone/restore/cleanup，不替代真实 Steam/上游镜像验收。
- 私有恢复目录和临时认证快照不进入镜像构建上下文、支持包或普通下载；`rollback_failed` 必须先人工保全材料，禁止自动重复恢复。
# GAME-RUNTIME-VERSION-1 构建、发现与发布边界（2026-07-14）

- Panel 镜像 embed 的 `runtime_stack_manifest.json` 同时固定 Junimo 镜像对及 App 413150/1007 推荐 buildid。发布者只能在真实运行环境验证 game+SDK+Junimo 兼容后更新 buildid、manifestVersion、notes 并保持 `tested=true`；运行中 Panel 不查询 Steam latest。
- `.github/workflows/discover-steam-builds.yml` 仅支持手动运行，绑定受保护 GitHub Environment `steam-build-discovery`。413150 凭据只从 Environment secrets 注入临时 0600 SteamCMD runscript，命令行和 workflow 日志不打印 login/password/token；1007 匿名查询。
- workflow 只上传 `steam-builds-discovered.json` 并写 summary，分类固定为 `discovered`；不写推荐矩阵、不提交/推送、不打 tag。阶段八可消费该 JSON 创建人工审查的兼容矩阵 PR。
- 运行时 ACF 检测用已有本地 server 镜像、`--pull never --network none` 和 game-data 只读 mount；发现 volume/镜像缺失只报告状态，不能隐式拉取或创建。发布检查增加 `npm run test:runtime-components` 与候选工具“无推荐矩阵/git mutation”静态测试。

## SMAPI 推荐清单与发布门槛（2026-07-14）

- Panel 镜像 embed 的同一 `runtime_stack_manifest.json` 固定 SMAPI 4.5.2 官方 installer URL、精确字节数、SHA256、下载/解压上限，以及 Control DLL/协议兼容值。更新这些字段视为正式 Panel 发布变更，必须与推荐 game/SDK/Junimo/auth 组合一起验收。
- `go run ./cmd/smapi-candidate --output <path>` 只发现正式 GitHub Release 并原子保存候选 JSON；`--tag` 才允许维护者显式检查 prerelease。网络/API/下载/摘要异常时保持旧文件并返回失败，不能写“已是最新”。
- `.github/workflows/discover-smapi-candidate.yml` 只允许手动触发，以只读 contents 权限运行上述 CLI；成功候选保存为 `discovered` cache/artifact，失败时 summary 明确展示的只是上次候选且任务最终失败。workflow 不编辑推荐矩阵、不提交/推送、不打 tag、不发布 installer。
- 正式镜像发布前必须运行后端全量测试、`go vet`、隔离 Docker integration、前端 build/状态测试，并在 release-candidate 环境跑真实安装/回滚长链路。Control hash 不匹配时先按既有 Docker/.NET 流程重建，严禁提交 `bin/obj`。
- SMAPI 下载缓存位于实例私有 `.local-container/smapi-update/packages`，不打入仓库；它用于后续完整玩家同步包。不得把候选 JSON、installer ZIP 或实例恢复材料提交、打 tag 或发布为本次产物。
- 新实例初装和后续升级都只能从 embed 清单取固定加速候选及官方兜底；旧 `.env` 的 `SMAPI_DOWNLOAD_URLS` 不参与下载选择。初装先在 Panel 侧按 allowlist/Range/大小/SHA/ZIP 上限缓存，再只读 bind 给安装容器，避免容器内 curl 跟随到未审核域名。
- 隔离 Docker 验证命令：`go test -tags=integration ./internal/docker -run 'TestRuntime(SMAPIIsolatedStagingCloneAndInstaller|ApplyIsolatedSteamSessionCloneRestore)' -count=1`。SMAPI 用唯一临时 volume 和临时 helper image 验证 clone、官方 installer CLI 边界与清理；它不使用真实实例，也不替代阶段八真实 RC 长链路。

## 2026-07-14：矩阵快照与发布 Environment

正式 Panel 镜像构建上下文必须包含 `runtime_stack_manifest.json`；运行时不从远程 latest 获取目标版本。`release.yml` 在 registry 登录和推送前验证内嵌清单、远程 digest/auth 溯源、全量 Go 测试、前端 build 和隔离 Docker integration。推送 `v*` Panel tag 后直接构建发布，不使用 `panel-release` Environment、required reviewer、`APPROVED_STACK_VERSION`、Actions run 或 E2E artifact。

矩阵中的镜像使用精确 tag 加 digest。运行时先按 tag 拉取，再 inspect RepoDigest；与内嵌 digest 不一致即拒绝，不把同名可变内容当作推荐镜像。auth-cn 新发布流程只推精确版本 tag，不再写 `latest`。已有旧镜像和旧矩阵信息必须保留用于人工确认后的回退；禁止 `docker compose down -v`、volume prune 或删除唯一 game-data 卷。

steam-auth-cn 发布与 Panel 发布解耦：auth 仓库不持有 Panel repository_dispatch token，也不向 Panel 自动创建 PR。维护者为新 Panel 版本直接填写已确认 server 及对应 auth 的精确 tag、digest、`upstreamRef` 和 `sourceRevision`；Panel CI 验证这组明确指定的版本对。
# 2026-07-14 矩阵与发布门禁加固

- `.github/workflows/compatibility-matrix.yml` 只验证当前 Panel 内嵌组件清单和相关测试，不再扫描候选目录或校验状态迁移历史。
- 本机维护事务基线对精确 server/auth 版本对确认镜像一致、Auth `/steam/ready` schema 可解析、server/Junimo API 可用及重启恢复；不要求测试实例已登录。`ready=true`、`has_ticket=true`、邀请码及重启后认证保持属于 Steam 在线模式专项验收，不得成为 LAN-only 用户升级/回滚的运行时门槛。任何验收笔记都必须脱敏，不得记录 Steam 密码、refresh token、App Ticket、二维码或 session volume 内容。
# 2026-07-14：推荐镜像运行时契约发布门槛

- Docker integration 必须运行 `TestRuntimeInspectAndAuthProbeWithoutNode`，验证含敏感键环境变量的 inspect 不破坏结构，并验证 auth 镜像不依赖 Node.js。
- 发布候选实机建议额外设置 `ANXI_REAL_SERVER_IMAGE` 与 `ANXI_REAL_AUTH_IMAGE` 运行 `TestRuntimeRealImagesOptIn`；该测试检查推荐镜像 digest/ID、真实 .NET auth `/steam/ready` 可解析和容器状态，不读取凭据。需要声明 Steam 邀请码在线能力时，应另跑有凭据专项验收；专项结果不改变维护事务的 LAN-only 基线。
- 镜像 inspect 实现只能让 Docker 输出审核过的字段；禁止恢复完整 inspect JSON 后再脱敏解析。
# JUNIMO-RUNTIME-HEALTH-PROBE-1 发布约束（2026-07-14）

- Junimo 运行组件升级/回滚验收不得假定 server 镜像包含 `wget`；`.121` 与 `.125` 均使用镜像已有的 Bash `/dev/tcp` 检查本机 `8080/health`。
- 发布门禁应保留“探针不含 wget”回归测试，并覆盖结构化镜像层下载进度、初始失败原因与回滚失败步骤。该变更不改变推荐矩阵、镜像候选或 23 个 init 兼容挂载。
# PANEL-0.2.9 Junimo FIFO 验收修复（2026-07-14）

- `v0.2.9` 修复 `.125` 容器和 Junimo health 已正常却因 `attach-cli -T` 固定失败、等待五分钟后错误回滚的问题。
- 正式镜像必须包含 FIFO `info` 控制契约验收与回归测试；不得删除 digest、Steam ticket、SMAPI/control、邀请码或恢复状态门槛。
- tag 发布继续由 `.github/workflows/release.yml` 执行完整矩阵、远程制品、Go/Docker integration、前端状态脚本与生产构建，并覆盖三个仓库的 `0.2.9` 和 `latest`。
# COMPONENT-UPDATE-FLOW-GATE-1 发布前一键升级编排验证（2026-07-14）

- 正式发布工作流与兼容矩阵工作流新增 `npm run test:component-update-flow`，验证新点击只能消费本次 dry-run ID、apply 不得抢跑或重复提交、较新工作流必须覆盖历史终态。
- 发版前还必须运行本地 QA `junimoWorkflow=race-retry`：初始提供旧 `succeeded` dry-run 与旧 `failed_rolled_back` apply，POST 新 dry-run 故意延迟；一次点击的事件必须是新 dry-run POST/成功轮询后才有 apply POST，且不得出现 `apply:POST-rejected`。
- 未通过上述状态测试、本地点击验证或生产构建时不得打 tag；该门禁从下一次 Panel 发布开始生效。

## Control 0.2.0 运行时农场目录构建（2026-07-15）

- source manifest、embedded manifest 与 `runtime_stack_manifest.json.controlMod.version` 必须同时为 `0.2.0`。
- 当前嵌入 DLL SHA256：`21eedc867d5a051389e19a5941aeaa067a7c6e36dbced1c86193d8e44a9c8249`；`runtime_stack_manifest.json.controlMod.dllSha256` 必须一致。
- 为避免构建过程把控制 Mod 部署进开发机游戏目录，游戏程序集应只读挂载，并显式关闭 ModBuildConfig deploy：

```powershell
docker run --rm `
  -v "E:\stardew-server-anxi-panel\backend\internal\games\stardew_junimo\embedded\smapi-mod-src:/src" `
  -v "E:\stardew-anxi-panel\runtime\game:/game:ro" `
  -w /src mcr.microsoft.com/dotnet/sdk:6.0 `
  dotnet build -c Release /p:GamePath=/game /p:EnableModDeploy=false
```

- 构建要求为 0 errors；ModBuildConfig analyzer 的编译器版本 warning 是已知提示。复制 `bin/Release/net6.0/StardewAnxiPanel.Control.dll` 覆盖 embedded DLL 后，必须重新计算 SHA256、更新运行栈清单，并执行 `go build ./...` 验证 `go:embed`。
- 纯契约测试仍从 `embedded/smapi-mod-contract-tests` 用 .NET 6 SDK 执行，不需要启动游戏。真实 `FrontierFarm` 运行时目录验证必须使用隔离实例，不能启动或改动生产实例，也不能用旧 options 缓存代替。

阶段 7 用只读 `stardew_game-data` volume 和 `/p:EnableModDeploy=false` 重建 Control 0.2.0，0 errors（1 个已知 analyzer warning），并同步新 DLL/运行栈 SHA。`docker build -t stardew-server-anxi-panel:phase7-local .` 成功，仅作本地验证，未推送或发布。真实 SVE E2E 已使用独立临时 Compose project、Panel DB、game-data/steam-session volumes、端口和实例目录完成；结果包含 fresh `FrontierFarm` catalog、XML `FrontierFarm`、重启及双向切档。既有实例未操作；该阶段临时 feature flag 不改变当时的关闭值，现行版本已默认开启。

## 模组农场灰度与发布门禁（2026-07-15）

1. 正式代码默认 `ENABLE_MODDED_FARM_CREATION=true`；需要禁用的部署必须显式设置 false。release/compatibility workflow 必须运行 `test:farm-catalog`，默认开启不豁免任何目录、依赖、runtime catalog、fingerprint 或 XML 验证。

## 2026-07-16：模组地图创建默认开启

- Panel 镜像未设置 `ENABLE_MODDED_FARM_CREATION` 时现在启用模组地图创建；设置为 `false` 可立即恢复关闭语义，设置无效值按安全的产品默认 true 回落。
- 发布验收必须同时覆盖默认未设置时 API 返回 true、显式 false 时返回 false，以及前端仅允许 `selectable=true` 项提交；不得通过镜像 compose 硬编码 true 而绕过后端配置测试。
2. 只在独立测试实例显式开启，且仅管理员看到/提交入口；请求只能携带 FarmType ID，不携带路径或任意 Mod 集合。
3. 至少完成一次显式创建、XML、容器重启、官方/模组往返、备份、恢复、导出、导入周期；确认事务目录没有活动残留，错误目录只在私有隔离区。
4. 观察日志必须脱敏；support bundle 不得包含事务快照、存档、认证/session 或恢复材料。
5. 版本号确认后再决定是否扩大灰度或改变默认值。未通过唯一目录/XML、单次 POST、回滚、profile、Control DLL/source 一致性或真实 SVE E2E 任一门禁时不得 tag/push/publish/latest。

当前 Control DLL SHA256：`21eedc867d5a051389e19a5941aeaa067a7c6e36dbced1c86193d8e44a9c8249`。阶段 8 已实际完成兼容清单校验、8 个矩阵脚本测试和 `docker build -t stardew-server-anxi-panel:phase8-release-gate .`；候选镜像只用于本机门禁并已清理。本阶段不创建 tag、不 push、不修改 latest 或生产容器。
# 2026-07-16：无声卡运行环境与整包 Mod 兼容性

- 新生成和已存在的 server compose 都应包含 `ALSOFT_DRIVERS=${ALSOFT_DRIVERS:-null}` 与 `SDL_AUDIODRIVER=${SDL_AUDIODRIVER:-dummy}`。这两个默认值只禁用真实音频输出，不禁用游戏音频资源加载；部署方可显式覆盖。
- 升级 Panel 后首次 prepare/start 会迁移旧 compose，并在启动前将可确认由 `mods/smapi` 提供的顶层 Console Commands/Save Backup 重复件移动到实例私有 quarantine。发布验收应检查 SMAPI 日志中不再出现 duplicate UniqueID 和 `NoAudioHardwareException`。
- 发布候选需使用真实多 Mod ZIP 验证：上传统计、SVE/CP/FTM 加载、旧存档警告和新存档 32 人 Introductions；不得用自动改写旧存档作为验收方案。
- 本次本机隔离验收已完成：`Mods1.zip` 发现 38、导入 36、跳过内置 2；SMAPI 26 个代码 Mod + 14 个内容包，SVE 自检通过，`Data/AudioChanges` 成功传播，新存档 Introductions 为 32，相关错误计数为 0。临时 Compose 项目、容器、卷和测试目录均已清理。
# v0.3.5：强制 JunimoServer 125 与维护验收解耦（2026-07-16）

- `v0.3.5` 内嵌 `runtimeUpdatePolicy=required`：从 `v0.3.4` 或更早版本点击 Panel 升级后，新 Panel 启动会自动把受支持的旧 JunimoServer 121 配置升级到 125，不再要求用户进入版本维护二次确认。新安装默认 125，已是 125 不重建。
- 发布门禁新增 required policy schema、自动协调成功/失败/修复/人工恢复/防循环/生命周期门禁测试。Docker 真机必须覆盖精确 121→125、Auth 未登录且无 ticket/邀请码、原运行/停止状态恢复、宿主 JunimoServer Mod 版本、FIFO `info` 精确 125，以及失败成对回滚不破坏 steam-session/game-data/saves。
- 可重复的升级真机命令：设置 `ANXI_REAL_UPGRADE_SOURCE_INSTANCE` 与 `ANXI_REAL_UPGRADE_SOURCE_GAME_VOLUME` 后运行 `go test -tags=integration ./internal/games/stardew_junimo -run TestRequiredRuntimeReal121To125OptIn -count=1 -v`。测试只读源实例/源卷，落盘前清空凭据，为 stopped/running 各建独立目录、game-data、空 steam-session 和 Compose project，结束自动清理。
- 全新安装边界用 `ANXI_REAL_FRESH_INSTALL=1 go test -tags=integration ./internal/games/stardew_junimo -run TestFreshInstall125ReachesSteamLoginOptIn -count=1 -v`：必须证明 Prepare 阶段没有容器/卷，点击安装后直接使用 125/Auth pair，并在真实 QR 登录阶段取消、清理所有一次性资源。
- Panel tag 继续使用不可变正式版本 `v0.3.5`，由 `release.yml` 在推送 registry 前运行远程 digest、全量 Go、Docker integration、前端状态脚本和生产构建；通过后发布 Docker Hub/ACR/GHCR 的 `0.3.5` 与 `latest` 并创建 GitHub Release。
# Save-import E2E image note (2026-07-17)

- Validation used `sdvd/server:1.5.0-preview.125` only in isolated Docker projects/volumes. The embedded Panel Control 0.2.0 DLL was rebuilt locally and validated with native `SaveGameMenu` -> SMAPI `GameLoop.Saved` behavior.
- No image was published and no production deployment was performed. A future image/release still requires the remaining human semantic and fault-injection gates.

## Local game-volume reuse note (2026-07-17)

- The existing `stardew_game-data` volume was mounted read-only and copied into the new `save-import-local-rich-game-data` test volume. Accepted testing used the explicit `save-import-local-rich` Compose project and dedicated ports; the source game volume and original instance save directory were never used as writable import targets.
- No image, release, registry push or production deployment was created.

## v0.3.6 save-import release gate

- `release.yml` and `compatibility-matrix.yml` run `npm run test:save-import` alongside the existing frontend state-machine tests before the production build. The runtime diagnostic's expected Control version is derived from the embedded compatibility manifest so the UI cannot drift back to a stale hard-coded version.
# v0.3.9 发布记录：轮询资源泄漏与重复重启门禁（2026-07-18）

- `v0.3.9` 修复 Panel 长时间运行时的轮询资源泄漏：邀请码查询只读 server 容器 `/tmp/invite-code.txt`，空值返回 `n/a`，不再启动交互式 `attach-cli`；邀请码与资源指标均增加按实例 5 秒缓存和 singleflight，多浏览器页面共享同一次 Docker exec/stats。
- 活动重启 job 使用持久 operation payload 标识；重复提交返回 `409 restart_in_progress`，不会取消或替换原重启。前端在页面隐藏或关闭时停止玩家、邀请码和指标轮询，恢复可见后再继续，并把 `n/a` 保持为未就绪状态。
- Docker Desktop 29.5.3 使用隔离 `bash:5.2` Compose project 验证真实文件 exec、空值、12 路并发邀请码、真实 stats 与 12 路并发共享；测试项目已 down，宿主没有遗留运行测试容器或 attach-cli 进程。后端全量 test/vet/build、前端 TypeScript/Vite production build 与并发专项测试通过。
- annotated tag `v0.3.9` 沿用 `.github/workflows/release.yml`，由远端发布门禁构建并推送 Docker Hub、阿里云 ACR、GHCR 的 `0.3.9/latest`，随后使用本 tag 的中文注释创建 GitHub Release。

# v0.3.10 发布记录：删除人物后自动暂停兼容（2026-07-19）

- `v0.3.10` 内嵌 Control 0.2.1：source/embedded manifest 均必需依赖 `JunimoHost.Server`，DLL SHA256 固定为 `e01cfcdb8df3d06e541b4f011edd7b6f748ee351ed16f9bf0c8537fcc5b20015`，`runtime_stack_manifest.json` 必须完全一致。
- 发布门禁必须在 Docker .NET 6 SDK 中运行 Control 契约矩阵并使用只读真实 game-data 编译 Mod；随后执行后端全量 test/vet/build、Docker integration、前端全部状态脚本和生产构建，再构建 `stardew-server-anxi-panel:0.3.10-rc` 做版本、Control manifest/hash 与运行态 smoke。
- Docker Desktop 真机验收使用隔离 `farmhand-delete-e2e` Compose project、独立存档目录和 game-data 卷；覆盖真实 `.125` 删除后暂停、15 秒时钟稳定、重启、600/610/2500/2510、节日 600→620 和错误日志扫描。不得把 test seam、测试覆盖文件或测试存档打入候选镜像。
- annotated tag `v0.3.10` 继续触发 `.github/workflows/release.yml`，由远端门禁构建并推送 Docker Hub、阿里云 ACR、GHCR 的 `0.3.10/latest`，再以中文 tag 注释生成 GitHub Release。
# v0.3.11 发布门禁：低资源主机升级冷启动修复（2026-07-19）

- `v0.3.11` 修复旧 Panel 升级触发 `.121 -> .125` 冷重建时，低资源主机在固定 5 分钟内未完成 Junimo/SMAPI 启动而误回滚的问题。目标验收最长 20 分钟，stop 在 Docker 短时超时后最长 10 分钟幂等重试；硬验收、digest、认证快照和卷安全边界不变。
- 候选镜像必须验证新生成与迁移后的 Compose 含 `steam-auth cpu_shares=256`、`server cpu_shares=768`，且这是相对权重而非 CPU quota。低资源 warning 只读取 Docker `NCPU/MemTotal`；镜像不得获得宿主 sysctl 写权限。
- Docker Desktop 真机门禁必须使用隔离实例目录、Compose project、game-data/steam-session 卷与端口，覆盖 `.121 -> .125` stopped/running、真实目标健康/FIFO 版本、原运行状态恢复，以及注入 stop 前两次超时后的安全回滚。不得使用生产实例或凭据。
- tag 前执行后端全量 test/vet/build、隔离 Docker integration、兼容矩阵脚本、全部前端状态测试与 production build，并构建 `stardew-server-anxi-panel:0.3.11-rc` 检查 `/health`、`/api/version`、OCI labels 与 Panel updater。annotated tag `v0.3.11` 沿用 `.github/workflows/release.yml` 发布三个 registry 的 `0.3.11/latest` 并生成 GitHub Release。
- 本地发布结果：Docker Desktop 29.5.3 上真正的 `.121` 镜像与宿主 Mod fixture 已通过 stopped/running（173.86 秒/106.34 秒），两条链均升级到 `.125`、恢复原状态并验证 256/768 实际 CPU shares；后端 test/vet/build、Docker integration、兼容矩阵远端制品、九项前端状态脚本、production build、`run.sh` 测试及 `0.3.11-rc` health/version/OCI smoke 全部通过。三个不可用的可选 server 镜像源仅产生 warning，canonical 制品摘要校验通过。
# v0.3.12 发布门禁：在线暂停反馈锁修复（2026-07-19）

- Control 版本为 0.2.2，source/embedded manifest 与推荐矩阵必须一致，嵌入 DLL SHA256 固定为 `547c08d8761d0a50fd713077ba9b6d5aa3db091df44be3a6400b6fdcf183f3a9`。契约测试必须证明任意正连接数均不由 Control 强制暂停，零连接普通日边界仍保留。
- 运行栈版本包含 `control-0.2.2`；真实 `options.json` 报告旧 Control 时必须给出 `control_update_available` 并通过受控同镜像重启加载新 DLL。不得只检查宿主 manifest/DLL，也不得把磁盘覆盖冒充进程已升级。
- Docker Desktop 门禁必须覆盖真正 `.121 -> .125 + Control 0.2.2` 和 `.125 old-Control -> .125 + Control 0.2.2` 的 stopped/running 四条链，断言运行 Control 版本、原状态恢复、目标健康和 CPU shares；源实例、game-data 与凭据保持只读/隔离。
- tag 前继续执行 Control Docker .NET 6 契约/真实 Mod 编译、后端全量 test/vet/build、Docker integration、兼容矩阵远端制品、全部前端状态脚本与 production build、`run.sh` 测试，并构建 `stardew-server-anxi-panel:0.3.12-rc` 检查 health/version/OCI/updater。annotated tag `v0.3.12` 触发三仓镜像和 GitHub Release。
- 本地发布结果：上述 Control 构建/契约、后端全量、Docker integration、兼容矩阵远端制品、9 个 Python 测试、`run.sh`、九项前端状态脚本与 production build 均通过；`0.3.12-rc` 返回 healthy、版本 0.3.12、数据库 ok，OCI version/revision/created 和 updater 可执行门禁通过。三个可选 server 镜像源不可达仅产生 warning，canonical digest 校验成功。

# FNOS-COMPOSE-MIGRATION-1：旧飞牛容器一次性标准化脚本（2026-07-20）

- 新增 `deploy/migrate-fnos.sh`，用于处理飞牛/NAS 通过“创建容器”或非标准 Compose 部署后，Panel 内置升级提示“当前容器的 Compose labels 不完整或不符合标准 panel 服务”的启动死锁。脚本必须在 Linux/NAS 宿主机 SSH 中运行，不得在 Panel 容器终端内运行。
- 脚本枚举运行中、健康、OCI identity 或可信仓库匹配且能解析稳定 SemVer 的 Panel，自动选择最高版本；最高版本同名候选共享数据目录时取创建时间最新者，不同数据目录时停止并要求通过 `PANEL_CONTAINER` 明确选择。不可按容器名称或可变 `latest` tag 猜测版本。
- 迁移仅支持可验证且可写的 bind-mounted Panel 数据目录、标准 Docker Socket、单一逻辑 `8090/tcp` 发布端口、非 privileged/root 默认用户；默认 bridge 可直接迁移，合法的现有自定义/Compose 网络会作为 external network 原样复用，不删除或重建。额外 bind mount 与具名 Docker volume 在 source/name、target、读写属性及传播语义均可验证时原样保留；tmpfs、宿主设备、匿名卷、host/container 网络模式、无法无损表达的额外挂载或多个独立部署一律停止，不擅自丢弃现场配置。
- 目标版本默认从 GitHub 最新稳定 Release 获取，也可通过 `TARGET_VERSION=x.y.z` 精确指定。镜像依次尝试阿里云 ACR、1ms、DaoCloud、GHCR、Docker Hub，拉取后必须校验 OCI title 和精确 version。
- 事务顺序为：备份 `docker inspect` 与已有部署文件、生成并校验标准 Compose、旧容器改名保留并关闭自动重启、新容器创建、容器健康和 `/api/version` 精确验收、Compose canonical labels 验收。任何失败都会删除失败的新容器、恢复原部署文件、恢复旧容器名称/重启策略并启动旧容器；绝不删除 Panel 数据、游戏容器、volume、存档或 Mod。
- 成功后的旧容器保持停止、`restart=no` 并保留，供管理员确认稳定后人工处理；不得再从飞牛旧项目启动/更新它。确认新版本稳定后可删除旧容器，但不能删除数据目录或新版仍作为 external 使用的旧网络。迁移只解决 Panel 部署/升级能力；运行中旧 Control 仍须登录新版 Panel 执行“运行组件升级”的受控保存、备份和游戏重启，禁止仅从飞牛直接重启游戏容器。
- 国内加速命令：

```bash
curl -fL -o migrate-fnos.sh https://gh-proxy.com/https://github.com/anxiyizhi/stardew-server-anxi-panel/releases/latest/download/migrate-fnos.sh && chmod +x migrate-fnos.sh && sudo bash migrate-fnos.sh
```

- GitHub Release 命令：

```bash
curl -fsSL -o migrate-fnos.sh https://github.com/anxiyizhi/stardew-server-anxi-panel/releases/latest/download/migrate-fnos.sh && chmod +x migrate-fnos.sh && sudo bash migrate-fnos.sh
```

- `anxinas.dpdns.org/migrate-fnos.sh` 当前尚未部署，文档不得提前给出该 404 地址；待自托管下载站真正同步该文件并通过 HTTP 校验后才可替换第三方 GitHub 加速入口。
- `release.yml` 将脚本作为正式 Release asset 上传，并运行 `scripts/tests/test_migrate_fnos.sh`。本地 Docker `bash:5.2` 语法/函数和 ShellCheck 已通过；隔离 Docker 29 dind 中，真实旧 `0.3.7` 独立容器已通过 ACR `0.3.13` 成功迁移、健康/精确版本/labels 验收、旧容器停止保留和 result 落盘。另通过停止新 Panel 注入健康失败，确认原 `0.3.7` 名称、运行状态、`restart=no` 和部署文件全部恢复。发布前仍应补充中断和真实多候选容器矩阵。
# FULL-STACK-UPDATE-2 发布门禁（2026-07-20）

- 后端：`go test ./...`、`go vet ./...`，并设置 `PANEL_RUN_DOCKER_UPDATE_TEST=1` 运行 updater Docker integration；至少覆盖任意 Compose 服务名成功升级、新镜像健康失败自动回滚、非目标服务/游戏容器不被重建。
- 飞牛转换：ShellCheck、`bash -n`、函数单测，以及隔离 Docker 中真实旧容器的成功转换和健康失败回滚。备份目录必须存在 Compose/环境、inspect、数据库和 `original-image-digest.txt`。
- Control：覆盖所有实例聚合、运行中玩家通告、保存、整档备份、停服安装、实载版本验证、Panel 中断续跑，以及安装/实载失败后实例保持停止。
- 前端：生产 build 和更新状态机测试；确认框必须明确在线人数、保存、整档备份、停服/重启和断线影响。
- 候选镜像：检查 `/health`、`/api/version`、OCI version/title、`/app/panel-updater` 和 `/app/migrate-fnos.sh`。上述任一门禁失败时禁止 tag、push、release 或更新 `latest`。
- 飞牛多部署真机必须至少连续转换两个旧容器，并断言第二次转换不改变第一次的新容器 ID；每个标准部署使用由旧容器名派生的独立 Compose project。
- `.125` Control 真机须同时覆盖 stopped/running；running 必须看到通告命令成功、`GameLoop.Saved` 成功、非空 `preruntimeupdate` ZIP，最终 `options.json.controlModVersion` 与嵌入 DLL hash 一致并恢复原运行状态。
- 本次 Docker Desktop 最终验收已完成：候选镜像内的独立 `/app/panel-updater convert` 将真实 `0.3.7` 飞牛式旧容器转换为 `0.3.13` 标准 Compose，持久化状态到达 `succeeded`，并保留非空 `container-inspect.json`、`environment.json`、`original-image-digest.txt`、数据库和迁移结果。故障注入在新容器切换后强制停止新 Panel，helper 以非零码结束并写入 `failed_rolled_back`；旧容器恢复原名称、端口、重启策略与健康状态，恢复后的数据库 SHA-256 与切换前保护副本完全一致。
# v0.4.2 发布门禁与一键升级验证（2026-07-24）

- 候选镜像：`ghcr.io/anxiyizhi/stardew-server-anxi-panel:0.4.2`（本地构建）。镜像 `/api/version` 为 `0.4.2`，OCI title/version 正确；从 `/app/panel` 的 Go build info 确认 `modernc.org/sqlite v1.54.0`，不是只更新 `go.mod`。
- Docker Desktop 29.5.3 冷启动 smoke 覆盖未初始化状态、未知页面/API 404、创建管理员、100 条扫描路径、持久卷重启后初始化恢复。Linux 容器内真实取消查询后的恢复回归连续 10 轮通过。
- 新增 opt-in `TestDockerIntegrationRealPanelCandidateUpgrade`，以真实 GHCR `0.4.1` 和本地 `0.4.2` 候选运行正式 `RunApply`；目标健康/版本、SQLite 数据卷、404 与游戏容器隔离全部通过。
- tag 前门禁：兼容矩阵 validate/version/远端制品及 9 个 Python 单测；`run.sh`、`migrate-fnos.sh`、ShellCheck；后端全量 test/vet/build；updater 与 runtime Docker integration；前端九项状态脚本和 production build；VitePress production build，全部通过。
- annotated tag `v0.4.2` 已由 `.github/workflows/release.yml` 成功发布 Docker Hub、阿里云 ACR、GHCR 的 `0.4.2/latest` 和 GitHub Release。三仓精确镜像 OCI version/revision 与 image ID 一致；隔离 DinD 中真实 `0.4.1` Web 更新 API 已完成发现、dry-run、apply 和三项健康验收。发布镜像 `/app/panel` build info 确认 `modernc.org/sqlite v1.54.0`。
