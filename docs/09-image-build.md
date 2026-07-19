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
- 默认下载源写入实例 `.env`：`SMAPI_VERSION=4.5.2`，`SMAPI_DOWNLOAD_URLS=https://gh.llkk.cc/... , https://github.dpik.top/... , https://ghfast.top/... , https://github.com/...`。可在 `.env` 中覆盖为自建 OSS/CDN 地址。
- 离线/企业部署若希望完全避免现场 GitHub 下载，建议把 SMAPI installer zip 放到自有对象存储/CDN，并把 `SMAPI_DOWNLOAD_URLS` 改为自有地址优先。

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
系统：Linux x86_64
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

面向普通 Linux 云服务器用户，优先推荐使用 `deploy/run.sh` 的快速模式。默认部署方式是公网 IP + `8090` 端口直接访问面板，用户只需要在云服务器安全组中放行对应端口。脚本会在用户主目录生成运行目录：

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
- 新实例初装和后续升级都只能从 embed 清单取目标；旧 `.env` 的 `SMAPI_DOWNLOAD_URLS` 不再参与下载选择。初装先在 Panel 侧按 allowlist/SHA/ZIP 上限缓存，再只读 bind 给安装容器，避免容器内 curl 跟随到未审核域名。
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
