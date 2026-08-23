# STEAM-CREDENTIAL-RECOVERY-1：SteamCMD 密码错误稳定进入凭据恢复（2026-08-22，released in v0.5.11）

- 生产 `v0.5.10` 只读诊断确认：缓存授权未命中后，SteamCMD 用完整账号密码登录并在同一行输出 `Logging in user ... Invalid Password`，随后退出码为 5。旧解析器先命中通用“正在登录”进度分支，导致凭据失败标记未设置，终态错误地落为 `state=error/driver_phase=steamcmd_failed`。
- `backend/internal/games/stardew_junimo/installer.go` 现在先识别 `Invalid Password` 等凭据失败标记，再匹配通用登录进度；上述组合行会稳定落为 `state=steam_auth_failed/driver_phase=credentials_required`，继续使用既有用户可读提示，不把网络或普通下载失败误判为密码错误。
- 新增 `TestDriverInstallClassifiesCombinedSteamCMDInvalidPasswordLineAsCredentialsRequired`，覆盖缓存授权失败后回退到完整登录、组合错误行、退出码 5 和最终状态。公开 API/DTO、SQLite、Compose、Steam 凭据保存方式及 Junimo 通信边界均未改变。
- 验证：Windows 定向新旧 SteamCMD 用例通过；任务专属 Linux Go 1.25 Junimo 全包通过（56.180s）；`go vet ./...`、`go build ./...` 通过。Windows Junimo 全包唯一失败仍是已记录的 NTFS `0666` 与 Linux `0640` 权限差异，不是本次产品回归。
- 正式 `v0.5.11@a9e186249a5c70c2e6fe45b7ed10a09db0b0c8bb` 候选 `32575311262` 与 Compatibility `32575311243` 全绿；候选 selected code gates 重新执行 backend test/vet/build、真实 Junimo network/runtime integration、frontend 回归/build 与 website build，immutable image 完成 fresh/restart 和 `v0.5.10` Web unhealthy/healthy 升级。
- 自动 Tag `32575807110` 与正式提升 `32575818623` 成功，annotated tag object=`d8bf5075d57f7aaf1b834ad62e12418a2db67ab7`；三仓 `0.5.11/latest` 与 candidate ref 统一 digest=`sha256:10c9813328370ae8ac92f11271fb76cd03787aab3b7f7fd523f20d66dfae8876`。正式 GHCR smoke 与 GitHub Release/四项资产均通过，完整 proof 和清理证据见 `docs/09-image-build.md`。

# v0.5.10 后端正式发布证据（2026-08-20，released）

- 最终候选 `32380002010@9b5a96233331b2050c930658d12eb6e49006f1f0` 与 Compatibility `32380002025` 全绿；候选 proof artifact=`release-candidate-0.5.10-9b5a96233331`（ID `9411011092`），OCI build date=`2026-08-20T14:25:36Z`，digest=`sha256:f0887c383d0043934b0023cc150e732f6d514e789df2d81c786297c122dc3bb4`。候选完整执行 fresh/restart、`v0.5.9` Web unhealthy rollback/healthy apply 与升级后存档 Phase A 专项。
- 自动 Tag `32381115159` 创建 annotated `v0.5.10`（tag object `c305b3ef0cea220bb27a24f08af140cf45d789fa`）并精确指向候选 commit；正式提升 `32381136325` 从 proof digest 原样提升，GHCR、Docker Hub、阿里云 ACR 的 `0.5.10/latest` 六引用完全一致。正式 GHCR smoke 的 `/health` 与 `/api/version=0.5.10@9b5a96233331...` 通过，latest GitHub Release 非 draft/prerelease且四项部署资产摘要与 tag 源文件一致。

# BACKUP-SCHEDULER-ATOMIC-EVENT-FIXTURE-1：候选回归复用 Control 原子事件契约（2026-08-20，released in v0.5.10）

- 第二次正式候选 `32378153924` 在 selected code gates 的 `TestBackupMaintenanceSchedulerCapturesConsecutiveGameDaysWithoutListingAPI` 安全失败，发生在 image build、GHCR push 和 proof artifact 前；同 SHA 的 Compatibility `32378153951` 成功。失败用例把事件直接写到最终 `*.json`，5 ms scheduler 可能在 `os.WriteFile` 完成前读取半截 JSON、按坏事件删除，继而永久等不到当天备份。
- 生产 Control Mod 并不存在这个发布窗口：`WriteSaveEvent → WriteJson → ContractFile.WriteJsonAtomic` 先写隐藏临时文件，完整序列化后才原子移动成最终 `*.json`；调度器 glob 只消费最终文件。并发测试现改用已有 `atomicWriteValidatedJSON` 发布完整事件，与生产契约一致，没有增加 scheduler 周期或放宽 2 秒结果预算。
- 任务专属 Linux Go 1.25 对函数级核验后的目标测试连续 `count=100` 全绿（11.896s）；随后整仓 test 全绿（Junimo `54.936s`、Web `50.676s`），vet/build 通过。影响仅 `saves_test.go` 的并发夹具；备份生产代码、API、持久结构、前端、Compose 和 runtime assets 均不变。最终不可变候选与正式提升证据见上节。

# RUNTIME-UPDATE-TERMINAL-SNAPSHOT-1：成功终态包含完整 cleanup warning（2026-08-20，released in v0.5.10）

- 正式候选 `32376230460` 在 selected code gates 暴露既有竞态：runtime update 正常链先写 `phase=succeeded`，随后才执行认证 snapshot/旧镜像 best-effort cleanup、追加 warning 并第二次写状态。API 与测试可能在两次原子写之间读到“已成功但清理警告尚未出现”的不自洽终态；镜像尚未构建或推送，旧候选保持失败。
- `runRuntimeUpdateApply` 的正常成功与 Panel 重启续作成功路径现在都先做 transaction-owned snapshot/旧镜像清理并把失败收集进 `Warnings/Logs`，最后只通过既有 `finish` 一次性写入 `succeeded`、时间、serverRunning、终态 log 与审计。清理仍是 best-effort：失败不回滚已经验收成功的运行栈，但 warning 在 terminal 首次可见时就完整存在。
- 回归把 fake 旧镜像清理显式阻塞，确认阻塞期间 `RuntimeUpdateApplyStatus` 绝不是 terminal；释放并注入删除失败后，最终仍为 `succeeded` 且包含“旧镜像”warning。任务专属 Linux 定向 `count=20`、整仓 test（Junimo `59.446s`、Web `53.524s`）、vet/build 全绿；最终候选已从修复后的新 commit 完整重跑并发布。
- 影响 `runtime_update_apply_runner.go` 与 `runtime_update_apply_test.go`；JSON shape、错误码、SQLite、Compose、前端和 runtime manifest 不变，只收紧成功终态的可见时点和快照一致性。

# SAVE-IMPORT-RELEASE-GATES-1：真实候选覆盖规范化、零效果恢复与原主机可选（2026-08-20，released in v0.5.10）

- `v0.5.9@0657ff01f121` 已发布存档目录 canonicalization、exact-target readiness 和 terminal no-effect mutation recovery；发布后审计发现候选没有执行已经声明的 exact target invisible 与 FIFO sent/no disk effect 两条真实 Docker 场景。不可变 `v0.5.9` 不移动，本次只补测试/发布门禁并以 `v0.5.10` fix-forward。
- `scripts/tests/test_release_candidate_upgrade.sh` 现在在健康升级得到的新 Panel 中创建真实 FIFO、server log、staged save 和受控 Junimo API：目标不可见必须停在 pre-submit；目标可见但 FIFO 零效果必须留下脱敏诊断、严格 no-effect 复合证据并恢复 snapshot。两条失败后再执行管理员选档，验证 exact journal/token/staged/source 自动清理、preimport/hash/pointer/备份保持，普通 mutation 不再被旧事务假锁死。
- `host_bed_real_integration_test.go` 不再只导入已经 canonical 的真实存档。它先把真实主存档打成缺少 world ID 的 ZIP，再由 `PreviewSaveZip` 规范化并把返回的 staging root 直接交给真实 `ImportSaveAndStart`；成功后官方 TestClient 按 farmhand 名称选中 `OriginalOwner`，从而直接证明导入后原主机可选，并继续覆盖 finalizer、解绑、durable save、重启、床位、F9/F10 与睡眠跨日。
- 真实 Junimo `.125` 链路已在 Linux Go 1.25 与精确 upstream revision `89abe8e6a07b3aaee1c0b4fad080683b948645d9` 上通过，用时 `223.39s`；canonical save=`HostBedGate_2510107853108169243`、`original_host_selectable=true`、Spring 1 → Spring 2，任务 owner 资源为 0。候选脚本 Bash/ShellCheck 与 integration 编译也已通过；完整候选和升级证据见 `docs/09-image-build.md`。

# SAVE-IMPORT-RUNTIME-IDENTITY-NORMALIZATION-1：上传目录与 Stardew 运行时 saveId 统一（2026-08-20，released in v0.5.9）

- 生产 `v0.5.8` 的真实 swap_to_player 导入证明：上传 ZIP 顶层目录/主文件名只有 3 个字符且不带世界 ID；Junimo Layer A 已完成主机交换，但 Stardew/SMAPI 加载后把世界标识解析为 `<主文件首个下划线前缀>_<uniqueIDForThisGame>`。Junimo pending intent 仍记录原 3 字符目录，finalizer 的 wrong-save guard 因精确名称不同清除 intent 且 `finalizeCount` 保持 0；Panel 检出 target runtime saveId 不一致后正确恢复完整 preimport，原主机因此回到不可由客户端选择的主 `<player>`，不是角色丢失。
- `PreviewSaveZip` 现在只在私有上传临时树中解析主 `<SaveGame>` 的非零十进制 `uniqueIDForThisGame`，按 Stardew `getLoadEnumerator`/SMAPI `SaveFolderName` 规则构造运行身份；名称变化时以 no-replace 方式同步重命名顶层目录、主文件和可选 `_old` 文件。预览响应、durable token、journal、staging、Junimo command 和后续 runtime evidence 从一开始使用同一 canonical saveName，不修改 XML 字节，也不在 finalizer 后猜测同档别名。
- 已经规范的 `<prefix>_<worldId>` 保持不变；缺失 identity 的历史/损坏夹具继续走既有 preview/后续解析错误语义，显式非数字、溢出或零 identity 会在接管前拒绝。规范名仍经过 UTF-8、长度、路径、保留字和 Junimo command token 全部门禁；重命名目标已存在时 fail closed，不覆盖临时树中的任何节点。
- 影响 `internal/games/stardew_junimo/{saves.go,saves_test.go}` 及上传预览返回的 `saveName` 值，不新增字段、SQLite migration、Control/Junimo 修改或 XML rewrite。专项覆盖生产同形态的无后缀中文目录、主文件与 `_old` 同步规范化、旧目录消失和正常/显式目录/GBK preview 不回归；任务专属 Linux Go 1.25 整仓 `go test ./... -count=1`、`go vet ./...`、`go build ./...` 均通过。
- 修复提交 `0657ff01f1216ffaa9362a800cf228bb4307aa8a` 已推送 `origin/main`，并以不更改公开版本比较的本地热修镜像部署到生产 `v0.5.8`。热修后 Panel/SQLite/Docker health 全绿，实例为 `stopped`、active jobs=0、两份 journal 均为 `rolled_back`、pending intent=false；原正式镜像、一致 SQLite 与 Compose/.env 回滚点保留。还需用户重新上传同一 ZIP 并重新提交平台 ID 才能完成真实 finalizer；回滚后原主机仍是主 `<player>`，因此在成功重导入前不会出现于可选 farmhand 列表。

# SAVE-IMPORT-TERMINAL-MUTATION-RECOVERY-1：终态零效果事务不再假 busy（2026-08-20，released in v0.5.9）

- 生产 `v0.5.8` 再次出现 `stardew_import_save_and_start` 已终态失败、Phase A 前后主文件 hash/active pointer 均相同、无 pending intent、maintenance snapshot 已恢复，但普通启动/选档仍提示“已有存档导入任务正在运行或等待恢复”。根因不是仍有 active job，而是 `handleInstanceByID` 的通用 import mutex 在具体 handler 之前只检查 unfinished journal；安全自动恢复原本只接在 preview/commit/jobs-clear，普通管理员 mutation 永远到不了恢复入口。
- 生产先在 Panel pause 窗口中完成 SQLite/journal/token/存档与指针不可变备份和 dry-run，再按 driver 的严格 Phase A no-effect cleanup 契约移除本 operation 的 transaction、owned token、staged target/source，保留 preimport ZIP 与 cleanup receipt。该现场属于首次安装 bootstrap，cleanup 后 gameloader pointer 按产品契约删除而不是保持；只读终态确认 Panel `/health=ok`、restart count 未变、active jobs=0、unfinished journal 已清零。备份留在生产私有恢复目录，没有写入仓库或日志凭据。
- `handleInstanceByID` 现在对受 import mutex 保护的 mutation 先认证；仅管理员会在通用 busy 判定前调用 `autoRecoverSafeFailedSaveImport`。只有 terminal failed/canceled job、journal/job/owned token 精确同源、runtime 严格停止、maintenance snapshot 已恢复且完整复合证据重新证明 Phase A 零效果时才收敛；未认证请求不能触发 cleanup，普通用户、active/ambiguous/effect-bearing 事务继续 fail closed。
- maintenance readiness 不再只用通用 `saves` 列表证明命令注册，而是发送只读 `saves info <exact-target>`，必须收到精确 `Save: <exact-target>` 才进入一次性正式 FIFO 提交，因此容器看不到 staged target、目标不可读或命令尚未就绪会在 `upstreamSubmitted=false` 时安全失败。saveName 继续经过命令 token 校验，不新增任意命令入口。
- Phase A 超时/取消会在 `ComposeDown` 删除容器前，按 pre-submit 字节 offset 有界读取最多最后 16 KiB `server-output.log`，清理控制字符、替换原始 platform ID 并将最多 1024 字符写入 `phaseALogDetail`；读取失败只留下通用诊断并参与错误 cause，不改变最终磁盘复合分类，也绝不以日志文本替代成功证据或重发 import。
- 影响 `internal/web/{instance_handlers.go,lifecycle_handlers.go,pending_uploads_test.go}` 与 `internal/games/stardew_junimo/{save_import_maintenance.go,save_import_phase_a.go}` 及测试；公开 API/DTO、SQLite schema、Compose、前端和 runtime manifest 不变。管理员 mutation 零效果恢复专项、精确目标 readiness、日志脱敏专项通过；Web 全包 33.337 秒、Linux Go 1.25 Junimo 全包 60.493 秒、Linux 整仓 `go test ./... -count=1`、`go vet ./...`、`go build ./...` 全部通过。Windows Junimo 全包唯一失败仍是已记录的 NTFS `0666`/Linux `0640` mode 差异；Web 首轮全包暴露既有 2 秒 journal 辅助等待抖动，改为与 job helper 相同的 7 秒预算后全包通过。

# SAVE-IMPORT-PHASE-A-NO-EFFECT-RECOVERY-1：Phase A 无落盘效果后的严格恢复（2026-08-20，released in v0.5.8）

- 根因是 Phase A 已写 FIFO 后，即使最终前后证据严格分类为 `command_failed_no_effect`，旧逻辑仍因 `phaseAFifoWriteAttempted/upstreamSubmitted/stage=import_submitted` 禁止恢复 maintenance 实例快照和 cleanup，留下永久 unfinished journal；v0.5.7 的 legacy jobs-cleared 恢复还会在 exact job binding 已完整时重写 `token.json`，把绑定 mtime 刷新到 `jobs_cleared` 之后，使本来有效的旧审计证明失效。
- `stardew_junimo` 新增复合 no-effect 证明：必须同时具有 `PhaseAOutcome=command_failed_no_effect`、完整 pre/after 磁盘证据、after 时间不早于 pre、主存档 hash 与活动 pointer 均未改变、没有 Junimo pending intent，且 `classifyImportPhaseA` 重新计算仍为 no-effect；单独修改 outcome 标签不能取得恢复权限。
- no-effect 在当前进程中会先持久失败证据，再重新读取 journal、重验当前磁盘仍等于 after 证据、恢复精确 pre-maintenance 实例快照，最终返回原 `command_failed`。Panel 若在失败记录或 snapshot restore 中断，`RecoverImportTransactions` 会分别进入 stop-and-restore 或 finish-snapshot-restore；磁盘漂移、pending intent、新的 upstream confirmed 或任何证据缺失继续落入 `manual_required`。
- strict cleanup/finalizer 只对上述复合 no-effect 开放历史 FIFO/submitted 标记的窄例外，仍要求 runtime 已严格停止、maintenance snapshot 已恢复、pointer 与 bootstrap/source/staged 全树 fingerprint 不变，并永久保留 preimport 备份。其它 submitted/unknown 路径仍禁止自动删除，也不会重放 `saves import`。
- `pending_uploads.go` 的 job attach 现在真正幂等：type/job/idempotency 三项已完全一致时不再写 `token.json`，因此不会篡改 legacy `binding mtime < jobs_cleared audit` 的时间证据；需要补齐部分字段时才原子写并返回写后真实 mtime。
- 影响 `internal/games/stardew_junimo/{save_import_phase_a.go,save_import_maintenance.go,save_import_transaction.go}`、`internal/web/pending_uploads.go` 及测试。公开 API/DTO、SQLite schema、前端和 Junimo runtime manifest 均未改变。定向测试覆盖当前进程恢复、Panel 重启恢复、snapshot-restore 中断续作、磁盘漂移、伪造标签拒绝、cleanup/finalizer 与 token mtime 幂等；Linux Junimo 全包、隔离 Web 全包、`go vet ./...`、`go build ./...` 已通过。
- 生产 v0.5.7 现场已在 Panel 停止、游戏 Compose 空且 SQLite `integrity_check=ok` 后用一次性严格恢复程序收敛：实例精确恢复为 `game_installed`，unfinished journal 与 owned token 删除，cleanup receipt 落盘，preimport 备份保留，Panel 重启后健康。代码随后随 `v0.5.8@8d5fe360c04240d7ccb9f9ac61ffecaed128627c` 正式发布，三仓版本与 `latest` 统一 digest=`sha256:f192d7840e564fe6c0bba6ab895e1533764c21e53257fcbde3cea01b75d59b66`。候选覆盖代码回归、fresh/restart、v0.5.7 Web unhealthy/healthy 和升级后 legacy jobs-cleared E2E，但未增加“FIFO 已写且严格 no-effect”在升级 Panel 上的真实 Docker 夹具；该门禁缺口已记录在 `docs/09-image-build.md`，后续相关发布前必须补齐，不能用本次 Go 回归或生产一次性恢复替代。

# SAVE-IMPORT-AUTO-RECOVERY-1：失败且未提交的导入事务自动收敛（2026-08-19，released in v0.5.7）

- 存档上传 preview 与 commit 在接受新事务前都会尝试自动收敛旧导入。正常路径要求 primary job 已为 `failed/canceled`，journal、job payload/idempotency key 与 durable upload record 三方精确绑定同一 instance/operation/job；针对 v0.5.5 已把终态任务清空的遗留状态，则要求 confirmed journal 与 owned token 仍绑定同一 job、token 绑定记录早于之后成功的 `jobs_cleared` 审计，且当前没有活动 import/recovery job。两条路径都必须由现有 `CleanupUnsubmittedSaveImport` 再次证明 Compose 停止、maintenance 已恢复、FIFO 从未进入模糊提交、pointer/源/目标/bootstrap 指纹未漂移后才自动清理；用户不需要保留旧上传 token 或点击恢复按钮。
- durable upload 新增仅供服务端恢复使用的 token-hash 引用。它不会反推或暴露 bearer token；自动链按 `operationId` 找到唯一 `owned` 记录，依次完成文件系统 cleanup → hash-keyed cleanup receipt → canceled journal finalization → 精确 owned token 删除。Panel/Web 在 receipt 已落盘、journal 或 token 删除中断后会在下一次上传入口继续同一收敛步骤。
- 任何 active job、多个未完成 operation、job/token/journal 身份缺失或冲突、运行时未停、`upstreamConfirmed`、指针或指纹变化仍 fail closed，返回原有 `save_import_busy/import_recovery_required/save_in_progress`，不会为了消除提示重放 FIFO、删除未知存档或覆盖同名目录。`phaseAFifoWriteAttempted/upstreamSubmitted` 只有在后续版本取得上节所述完整 Phase A no-effect 复合证据并重新校验当前磁盘时才有窄恢复例外；v0.5.7 本身仍按原边界处理。
- `POST .../saves/upload-preview` 现在在读取 multipart/ZIP 前先执行上述恢复并检查 unfinished journal：可安全恢复时用户本次正常上传直接继续；仍有活动或模糊事务时立即拒绝，不再先接收完整 ZIP 后才在 commit 报 busy。commit 同样保留一次恢复门，覆盖“preview 完成后旧 job 才失败”的竞争窗口。公开成功 JSON、hostHandling、job 类型和数据库 schema 不变。
- `DELETE /api/jobs` 现在会先逐实例收敛安全的失败导入；仍有模糊或已提交 journal 时返回 409 并保留任务证据，不再先删 job 再把实例留成永久 busy。`internal/storage/jobs.go` 新增最近一次成功整库清空审计时间读取，只用于兼容既有丢失 job row 的 v0.5.5 状态，不把普通“job 不存在”单独当成可删除证据。
- 主要影响 `internal/storage/jobs.go` 与 `internal/web/{jobs_handlers.go,lifecycle_handlers.go,pending_uploads.go,pending_uploads_test.go}`。专项覆盖原 token 已被 UI 丢弃后的正常新 preview 自动清理、任务已清空后的同请求恢复、preimport 保留、journal/owned token/目标残留清零、模糊事务阻止清空任务中心，以及 FIFO write-attempt 时 409 且全部材料保持；hash receipt 无原 token 的中断续收敛也有独立回归。
- 验证通过：Windows 定向恢复与 jobs/storage 包；任务专属 Linux Go 1.25 容器内 Web 全包（51.004 秒）、全仓 `go test ./... -count=1`、`go vet ./...` 和 `go build ./...` 全部通过，测试容器与两个缓存卷清零。正式候选 `32284304749` 又从 `v0.5.5` 完成真实 Web unhealthy 回滚、healthy 升级和升级后 legacy jobs-cleared 恢复；自动 Tag `32285201579`、正式提升 `32285223565` 成功，能力已随 `v0.5.7@f7cedaa31e9db71aa2291c8aa06ea857046caf81` 发布，三仓统一 digest=`sha256:0b2dbe649fd6ce7acce797e170fec9ad2f1da9f00730afe1bb39b4ea8d586290`。

# PANEL-UPDATE-LATEST-RELEASE-API-1：面板更新检查改用 GitHub latest（2026-08-18，未发布）

- Panel 更新检查不再请求 `/releases?per_page=20` 并假定列表首个稳定条目就是最新版本；默认源改为 GitHub 官方 `GET /repos/anxiyizhi/stardew-server-anxi-panel/releases/latest`，按单个 Release 对象解析。
- 返回对象仍须满足 `draft=false`、`prerelease=false` 且 tag 为有效 SemVer，否则进入现有 `checkStatus=error`，保留上次成功结果，不生成任意升级目标。请求超时、响应上限、NetDNS fallback、管理员手动刷新和 6 小时自动检查契约不变。
- `internal/updatecheck/service_test.go` 锁定精确 latest URL、单对象响应、draft/prerelease 拒绝和网络失败缓存；2026-08-18 只读核对官方 latest 与列表接口当前均为 `v0.5.4`，本修复纠正的是权威接口契约而非伪造版本差异。updatecheck/Web 专项、`go vet ./...`、`go build ./...` 均通过。

# JOB-LOG-LATEST-TAIL-1：任务日志最新尾页读取（2026-08-18，未发布）

- `GET /api/jobs/:id/logs` 新增兼容查询参数 `latest=true`。该模式按 `sequence DESC` 有界读取最新 `limit` 行，再在返回前恢复为时间正序；原有 `after=<sequence>` 增量读取与 SSE 契约不变。
- 响应新增 `hasEarlier`。存储层内部读取 `limit+1` 行判断是否确有更早日志，因此日志总数刚好等于上限时不会误报截断。`limit` 仍最多 1000，非法 `latest` 返回 `400 invalid_query`。
- 影响 `internal/storage/jobs.go`、`internal/jobs/manager.go`、`internal/web/jobs_handlers.go` 及专项测试。验证覆盖“5 行取最新 3 行得到 3/4/5 且正序”“总数等于 limit 时 `hasEarlier=false`”“HTTP 最新 2 行返回 4/5”与非法布尔查询；storage/Web 包、`go vet ./...` 和 `go build ./...` 最终均通过。

# NEXUS-EXT-MOD-UPDATE-1：管理员一键安全更新已安装 Mod（2026-08-17，released in v0.5.3）

- 复用管理员 `POST /api/instances/:id/mods/remote/install` 与 `mod_remote_install` 任务；请求新增可选 `replaceUniqueId`。字段存在时 Web 调用 `UpdateRemoteMod`，并强制要求正数 Nexus Mod ID/file ID 与非空 `expectedVersion`；省略时保持原安装/幂等行为。仅管理员、停止态可用，`replaceUniqueId` 限 1~256 字节且拒绝控制字符。
- Junimo 仍先下载完整 ZIP，再同时校验 `expectedVersion`、目标 SMAPI `UniqueID`、目标已安装且非内置、压缩包除 SMAPI 自带支持组件外只含一个目标 Mod。聚合安装包、多 Mod ZIP、错误版本或错误 UniqueID 均在旧目录移动前失败。
- 校验通过后在 `.local-container` 同文件系统创建任务临时备份，复制旧 `config.json` 覆盖新版默认配置，按目标当前物理根换入新版；安装时间持久化或文件换入失败会删除不完整新版并恢复旧目录。目标原来位于 `mods-disabled` 时更新后仍禁用，Web 不调用“为当前存档启用”逻辑。
- Nexus sidecar 只在新版写入成功后更新；sidecar 写入失败不会把已完成的文件替换误报为整体回滚。影响 `stardew_junimo/{mods.go,remote_install.go}`、`web/lifecycle_handlers.go` 与专项测试。
- 验证：单 Mod 替换、目录改名、旧配置覆盖新版默认配置、禁用态保持、聚合包拒绝、错误 UniqueID/目标版本零写入、安装时间写入失败恢复旧目录与清理临时备份，以及 Web 精确目标参数拒绝均有自动化；Windows 精确专项和权威 Linux 全量通过。真实 Chrome + 扩展 `0.1.8` 已把测试夹具中的 Content Patcher 从 `2.9.0/file_id=153187` 一键更新为 `2.9.1/file_id=160463`，请求携带 `replaceUniqueId=Pathoschild.ContentPatcher`，旧 `config.json` 哨兵与启用状态保留，临时下载/备份残留为零。
- 已随 `v0.5.3@ede7fa34231600cbfa83050b4ddb6fd650373ae1` 正式发布；候选 `32034798704` 完成后端全量、远程制品、真实 Junimo/SMAPI、fresh/restart、`v0.5.2` Web unhealthy 回滚/healthy 升级和升级后专项，正式提升 `32035725325` 未 rebuild，三仓统一 digest=`sha256:400ad1e92dc84bc62530d38e08ec2ddb20d4d385ee01dc2b35808d23d91bd1f8`。

# NEXUS-EXT-LATEST-1：扩展远程安装的最新版本锁定与落盘前验真（2026-08-17，released in v0.5.3）

- Nexus 搜索结果继续把主 Mod 的 `version` 作为当前版本；对 `requiredMods` 新增一次去重的 GraphQL 元数据补全并返回 `version`，使本体与所有未安装前置都能得到明确的最新目标。补全失败时搜索返回错误，不用缺失版本继续静默安装。
- 管理员 `POST /api/instances/:id/mods/remote/install` 在既有 `{url, mod}` 上新增可选 `expectedVersion` 与 `nexusFileId`。0.1.5 扩展会同时提交二者；旧客户端省略字段仍兼容。版本号只接受最多 64 位的字母、数字、点、下划线、加号和减号，`nexusFileId` 不得为负数；CDN URL 本身仍按原安全规则校验，签名 query 不会被拼接或改写。
- driver 下载 ZIP 到临时文件后、调用 `uploadModZip` 前读取所有 SMAPI `manifest.json`，按 SMAPI 的补零语义比较版本；至少一个 manifest 必须匹配 `expectedVersion`。该校验同时覆盖扩展 CDN 直链和携带目标版本的 NXM ticket 路径。不匹配时返回“期望/实际版本”错误并删除临时 ZIP，不创建 Mods 目录、不覆盖已有 Mod、不写安装时间或 Nexus sidecar。
- `mod_remote_install` 的任务日志、结构化日志与审计只记录安全的目标版本和 `nexusFileId`，不记录完整临时 CDN query。影响文件：`stardew_junimo/{nexus.go,remote_install.go,nexus_test.go}`、`web/lifecycle_handlers.go`。
- 验证：Nexus 搜索专项覆盖前置最新版本补全；远程 ZIP 专项覆盖 `v1.2.3` 等价匹配、`1.2.4` 不匹配和落盘前零写入；相关定向测试、扩展 ZIP 版本感知测试及任务专属 Linux Go 1.25 全量回归已覆盖该基础链。真实 Chrome + 扩展 `0.1.8` 在停止态测试实例先安装 Content Patcher `2.9.1/file_id=160463`，提交成功后才打开并安装 Elle's New Barn Animals `1.1.3/file_id=34408`；两份 manifest 精确匹配，Mods 临时制品为零。另一个旧 Mod 实际提供 `.rar`，因现有远程安装安全契约仅接受 ZIP，未把它作为成功夹具或放宽后端格式校验。

# SERVER-RUNTIME-MAXPLAYERS-1：建档后修改联机人数上限（2026-08-17，released in v0.5.3）

- 复用管理员 `GET/PUT /api/instances/:id/config/server-runtime-settings`；`ServerRuntimeSettings` 新增 `maxPlayers`，GET 在 `Server.MaxPlayers` 缺失或无效时返回 `10`，合法范围固定为 `1~100`。新前端始终提交实际值；兼容旧客户端省略 `maxPlayers` 的 PUT，并在同一把 `Driver.runtimeUpdateMu` 内保留磁盘当前值。
- 更新仍先通过未结束新建档 owner guard，再以 `atomicWriteValidatedJSON` 在目标目录创建、同步、关闭并 rename 临时文件。写回采用 map 合并，只改 `Server.MaxPlayers/CabinStrategy/ExistingCabinBehavior/NetworkBroadcastPeriod`，保留根级、`Game`、`Server` 的其它未知字段。返回值同时携带锁内读取的 previous/current，Web 审计记录 `maxPlayers` 与 `previousMaxPlayers`。
- 运行中 `/players` 不再用可能待重启的配置文件值冒充当前上限：Control `players.json` 提供在线快照时，driver 额外通过现有 Junimo `info` 命令读取当前生效 `maxPlayers`；读取失败返回 `null`。服务器停止时才把 `server-settings.json` 的值作为下次启动配置投影。
- 未新增人数专用 API、SQLite 设置表、存档级配置、Mod 检测或小屋门禁；建档参数 `startingCabins 0~7` 与运行配置 `maxPlayers 1~100` 仍是独立维度。
- 影响文件：`internal/games/stardew_junimo/{saves.go,driver.go,players.go}`、对应测试，`internal/web/server_runtime_settings_handlers.go`、Web API 测试与 opt-in `server_runtime_settings_real_integration_test.go`。
- 验证完成：Windows 相关专项、Web 权限/错误/审计、`go vet ./...`、`go build ./...` 通过；Windows 全包只命中既有 NTFS `0666/0640` 差异，完整仓库挂载的 Linux Go 1.25 `go test ./... -count=1` 全绿。真实 Docker E2E 只读克隆已有存档与游戏卷，依次证明配置 `11` 启动后 `/players.maxPlayers=11`、运行中 PUT `12` 后仍为 `11`、重启后 `/players.maxPlayers=12` 且配置 GET 保持 `12`；任务容器、网络、卷、临时目录清零，源夹具恢复原定义并保持停止。

# SUPPORT-BUNDLE-LOG-CONTEXT-2：诊断包补齐 Panel/Steam/任务日志（2026-08-17，released in v0.5.3）

- 管理员 `POST /api/instances/:id/support-bundle` 的路径、ZIP 流式响应、文件名和前端 Blob 下载契约不变；本轮只把内容收敛为“无需 SSH 也足够定位常见问题”的有界诊断材料，不加入自动分析或自动修复。
- ZIP 现在固定包含 `server-logs.txt`（server 最近 1000 行）、`steam-auth-logs.txt`（认证服务最近 500 行）、`panel-logs.txt`（Panel 容器最近 1000 行）和 `job-logs.json`（当前实例最近 10 个任务、每个最多 200 条进度日志）。已有版本、健康检查、Compose 状态/配置、Junimo 更新诊断、任务摘要和审计摘要继续保留。
- `instance-state.json` 改为复用诊断页的完整结构化实例状态，带 UI 生命周期、Control status/players、运行栈、安装和命令协议诊断；邀请码在序列化前强制清空。任务只选当前实例，payload 不导出；存档、Steam session、角色密码库、新建档事务、SMAPI/Junimo 恢复材料和备份仍不会进入 ZIP。
- Docker client 新增固定参数 `ContainerLogs`，只接受安全容器名和 `1..1000` 的 tail，用于通过 Panel 容器 hostname 读取自身 stdout/stderr。所有日志、任务消息、Compose 文本和错误输出继续经过统一脱敏；单项采集失败会写进对应 ZIP 条目，不会把已经开始的流式 ZIP 切换成 JSON 错误体。
- 同时修正支持包健康检查过去向 Docker runner 传空工作目录、因此总被判失败的问题，统一使用已验证的全局临时工作目录。专项测试覆盖 ZIP 条目、当前实例任务过滤、任务/三类容器日志、邀请码及密码/Token/恢复材料不泄露，以及容器名/tail 参数校验；`go test ./internal/docker ./internal/web` 已通过。

# HOST-BED-MANUAL-CONTROL-1：交换主机后的农舍/睡眠与 VNC 状态闭环（2026-08-16，released in v0.5.1）

- 根因位于 JunimoServer `.125` 的组合流程，而非 Web handler：`saves import --swap-host-to` 先创建 0 级 `Server` 主机/主 FarmHouse，`CabinManagerService.TryFinalizeOnLoad()` 随后把主 FarmHouse 的 objects、fridge、decor、terrain 与 furniture（含床）整体移入原主人的 cabin 并清空主屋；交换结果已经是 0 级，旧 `HostFarmhouseUpgradeGuard` 的“非 0 才重置”分支不会再生成默认家具。Control 0.3.4 又有意跳过破坏性的整屋归零，因此最终留下“等级/地图已切到新主机、主屋无床且家具为空”的不一致状态。人工补回合法 `BedFurniture` 后，`FarmHouse.GetPlayerBed/GetPlayerBedSpot` 能重新命中真实睡眠交互点，所以可缓解黑屏，但它没有修复导入事务、所有权或后续兼容性。
- 正式修复位于 Control `0.3.5` 的游戏线程与 stardew_junimo 导入事务。`HostBedIntegrity` 只检查当前 `Game1.MasterPlayer` 所属的主 `FarmHouse`，已有任意合法 `BedFurniture` 时零修改；缺床时只接受 0..3 级、实际 `Back` map 的 `DefaultBedPosition`，并使用游戏 `BedFurniture.DEFAULT_BED_INDEX` / `Utility.GetDoubleWideVersionOfBed` 创建 Single/Double 床。地图属性缺失、坐标无效、类型不符或添加后复核失败均返回 `host_bed_missing`，不猜坐标、不扫描或搬运 Farm/cabin 的床，也不重建其它家具。
- `swap_host_to` 激活必须同时取得目标 save、finalizer 与 `hostBed.healthy=true` 的复合证据；失败后停止运行时，校验 preimport archive，再原子恢复目标存档整树（含 current、`_old`、`SaveGameInfo`）、活动指针、Mod profile 和实例精确快照。该回滚覆盖激活与 durable save 的所有后置失败，错误码保留原始原因；证明不完整则进入 `manual_required`，禁止猜测恢复。Junimo API 的容器内 health/status 固定访问 8080，宿主映射端口只用于 Compose 暴露。
- `HostSleepSafetyPatch` 在游戏线程拦截 Junimo `.125` 的精确自动睡眠方法：缺床只记录一次结构化 `host_bed_missing` 并立即阻断无界等待；合法床沿真实 bed spot 进入原生睡眠确认，单日最多四次有界动作。F9 关闭 automation 时进入 10 分钟无人手动租约，释放输入抑制、强制主机完整可见并用 `ManualControl` 覆盖 `NoConnectedClients` 暂停；重新开启 automation 或租约到期恢复原无人暂停策略。F10、warp、SaveLoaded、DayStarted 与逐 tick 会原子同步 sprite、`displayFarmer`、`hidden` 和 shadow 状态。
- Control `status.json` 及现有实例状态的 `statusSource` 新增 `hostBed` 与 `hostControl`：后者包含 `mode/automationKnown/automationEnabled/manualControl/paused/pauseReason/hostVisible/displayFarmer/farmerHidden/visibilityConsistent/connectedClients/manualLeaseExpiresAt`，前者包含健康态、房屋等级、期望/实际床型、床 tile/player bed spot、自愈与错误信息。没有新增普通 Web 写接口或 Stardew XML handler 特判。
- 验证覆盖 0/1/2/3 级布局、已有床幂等、其它家具保留、Farm/cabin 床隔离、交换角色数据不串位、完整回滚与临时文件清理、手动租约/暂停/可见性契约。真实 `.125` Docker E2E 导入独立身份问题副本，完成 `SaveLoaded → 床证据 → save-now/GameLoop.Saved → 重启 → VNC F9/F10 → 官方测试客户端睡眠`，日期从春 1 日进入春 2 日，日志未出现超时/强制结束；实际 0 级床位为地图返回的 `(9,8)`，测试与实现均未硬编码该坐标。
- 本修复已随 `v0.5.1@427a295ab905701069b7f710300ba09b6afd21f0` 发布；候选 `31942102917`、兼容矩阵 `31942102879`、自动 Tag `31942624901`、正式提升 `31942631860` 全部成功，正式 digest=`sha256:70c1967eb36827dbbf78ec3c11683c994814961dcf6673ae365ec4f43c6c25a5`。

# MOD-UPDATE-CHECK-1：按本地清单自动检查 Mod 更新（2026-08-16，released in v0.5.2）

- Stardew Junimo driver 新增 `CheckModUpdates(ctx, instance, force)` 可选能力，读取启用与禁用目录中的物理 Mod；内置运行时、清单解析失败、缺少 `UniqueID`/版本/`UpdateKeys` 的条目不会请求上游。更新判断完全采用 SMAPI update service 返回的 `suggestedUpdate`，Panel 不自行比较第三方版本号，也不会自动覆盖本地 Mod。
- 新增 `GET /api/instances/:id/mod-updates`（已登录用户）与 `POST /api/instances/:id/mod-updates/check`（管理员强制刷新）。上游暂时失败时 HTTP 仍返回 `status=error`，并保留最后一次成功的 `updates/checkedAt`；本地列表或缓存读写失败才返回 500。建议链接只接受带 host 的 HTTP/HTTPS URL。
- 成功结果按实例原子写入 `<dataDir>/.local-container/control/mod-updates.json`，默认缓存 6 小时；缓存同时绑定本地物理 Mod 清单和实际 SMAPI API/game version、Linux platform 的 SHA-256 指纹，因此上传、删除、更换版本或运行时版本变化会立即使旧结果失效。请求根级 `apiVersion` 优先取 Control `options.json`，无运行时快照时保守使用 v4 端点基线 `4.0.0`；按 50 条分批、15 秒超时、响应上限 2 MiB，沿用 `netdns` 容错客户端，不读取或外发用户 Nexus API Key。
- 影响文件：`internal/games/registry/types.go`、`internal/games/stardew_junimo/mod_updates.go`、`internal/web/{instance_handlers,mod_update_handlers}.go` 及专项测试。五条 driver 专项覆盖缓存/运行时指纹失效/降级/URL 安全，Web 专项覆盖 401/403/管理员强刷/405；任务专属 Linux Go 1.25 的 `go test -p 1 ./... -count=1`、`go vet ./...`、`go build ./...` 与不发布的完整候选升级预演均通过。
- 本功能已随 `v0.5.2@51fd82459e4ac8afbf362f7ad12c0651937879a1` 发布。候选 `31945655119` 从 v0.5.1 完成 unhealthy 回滚、healthy Web 升级，并在升级后的 Panel 通过受控 TLS SMAPI 首次强刷和缓存 GET；Compatibility `31945655121`、自动 Tag `31946063809`、正式提升 `31946073920` 全绿。三仓版本/`latest` 统一 digest=`sha256:42b5dae824f63d3b5ba44a1f33704a622a62c4d6170225d52a63ac39147aaaed`，正式镜像版本接口与重启复核通过。

# v0.5.0 后端发布状态（2026-08-16，released）

- `SAVE-IMPORT-STRICT-OFFLINE-PROBE-1`、`SAVE-IMPORT-MAINTENANCE-DURABILITY-1`、`SAVE-IMPORT-TOKEN-CLEANUP-RECOVERY-1`、`PLAYER-LAST-SEEN-SEMANTICS-1` 与 `HOST-FARMHOUSE-PRESERVE-1` 已进入 `v0.5.0@9b18dd3fe5192692548bf11a85010dd35303da93`；Control 为 0.3.4，内嵌 DLL SHA-256=`5ab089610b0ae2b9368c0abd87165b98373206a80270ac58f237d29a8a13b982`。v0.4.19 的 none/global/role 认证、角色独立密码与旧全服密码兼容继续完整保留。
- Compatibility `31899107019`、显式候选 `31899107629`、自动 Tag `31899867310` 和正式提升 `31899874927` 全部成功。候选 proof `release-candidate-0.5.0-9b18dd3fe519` 固定 build date=`2026-08-15T17:48:42Z`、digest=`sha256:92ea973d55c1f63b4eb356652d491f8d37ef5f69112df1f19c161e4b0e9b611a`；`v0.4.19` Web unhealthy 回滚/healthy 升级、升级后受影响 E2E 与正式镜像首次/重启均通过。
- 首轮候选因取消时序测试把“进入 starting”和“超时取消”共用 30ms 而停止，未构建或推送镜像；修复后先等精确 phase 再 manager cancel，目标测试连续 50 次、Linux 全量 test/vet/build 与最终 CI 均通过。后续不要恢复基于极短 wall-clock 的竞态断言。

# HOST-FARMHOUSE-PRESERVE-1：默认保留虚拟主机农舍等级（2026-08-16，released in v0.5.0）

- JunimoServer `1.5.0-preview.125` 会在 `SaveLoaded` 调用 `JunimoServer.Services.AlwaysOn.HostFarmhouseUpgradeGuard.ResetHostFarmhouseToLevelZero()`，把非零的虚拟主机 `HouseUpgradeLevel`、室内布局和床重置为 0 级。上游已移除最初导致 #346 的 farmhand→host 房屋等级镜像活动，因此 Panel Control `0.3.4` 默认用 Harmony prefix 跳过这一残留过渡修复；不增加开关，也不修改 JunimoServer 镜像或上游源码。
- `HostFarmhousePreservationPatch` 只接受程序集名 `JunimoServer`、完整类型名、`public static void`、零参数这组精确签名，并在 Harmony patch info 中复核 owner。反射目标缺失、签名变化或补丁未登记时，Control 在 `options.json/status.json` 报 `hostFarmhousePreservationPatchAvailable=false`；Panel 启动门禁返回 `control_runtime_host_farmhouse_patch_unavailable` 并停服，不能在补丁未知时继续读档。
- 主要影响 Control 源码/内嵌 DLL、Control runtime gate 与 runtime stack manifest；Control 版本从 `0.3.3` 提升为 `0.3.4`，内嵌 DLL SHA-256 为 `5ab089610b0ae2b9368c0abd87165b98373206a80270ac58f237d29a8a13b982`。公开 HTTP API、前端、SQLite、存档 XML 格式和 server/auth 镜像不变。
- 可选真实 Docker 集成测试会只读复制游戏卷和源存档，在任务副本中把第一处虚拟主机 `houseUpgradeLevel` 设为 2，启动精确 `.125`、确认 Control gate ready、执行同一 Control `save-now` 并等待 `GameLoop.Saved`，最后断言磁盘仍为 2。2026-08-16 本地实跑通过并清理任务容器/卷为零；C# 契约测试和真实游戏程序集编译通过，任务专属 Linux 容器中的后端全量 `go test ./... -count=1`、`go vet ./...`、`go build ./...` 也全部通过。Windows 全量仅命中既有 NTFS mode `0666`/Linux `0640` 权限差异。
- 风险边界：此兼容层有意关闭上游的历史 #346 自愈。如果某份存档已经被旧 farmhand 镜像逻辑污染，Panel 不会替它自动降回 0 级；当前任务也不为导入存档增加特殊分支。后续 JunimoServer 变更目标类型/签名时会 fail closed，必须重新核对上游行为后更新兼容层，不能放宽为模糊反射。

# SAVE-IMPORT-TOKEN-CLEANUP-RECOVERY-1：job/token/journal 精确绑定与幂等取消（2026-08-15，released in v0.5.0）

- `jobs.Spec.BeforeRun` 在 job 行和 idempotency key 已提交后、runner goroutine 创建前同步执行持久 ownership 握手。存档导入固定按 `journal(jobType/jobId/jobIdempotencyKey=journal_attached) → owned token 同一身份 → journal jobBindingState=ready → runner` 发布；staging、preimport、bootstrap、maintenance 和 FIFO 在 `ready` 前均不可进入。握手失败会同步把 job 标成 failed、runner 调用次数为 0，并向 Web 返回 `import_recovery_required`，不再只记录 attachJob 错误后返回 202。
- primary job 的唯一身份为 `type=stardew_import_save_and_start + targetType=instance + targetId + idempotencyKey=save-import:<operationId>`，数据库唯一索引继续保证一对一。Panel 重启只按该四元组和 payload `operationId` 找回，不读取“最近任务”；journal/token 任一 jobId/type/key 缺失、冲突、payload 不符或 exact key 不存在都 fail closed。恢复验证按 journal attach、token attach、journal ready 幂等重做；submitted 后的自动观测任务改用独立 `stardew_import_save_recovery` 类型，不能冒充 operation 的 primary job。
- cleanup 在任何删除前建立并原子持久化 schema-1 `cleanupPlan`：冻结 operation/instance/save、source ownership 与全树指纹、staged 全树指纹、bootstrap 名称/ownership/指纹/source 指纹，以及 active pointer。未知 schema/stage/cleanup state、缺少 safety-critical 字段、maintenance/FIFO/submitted/confirmed、pointer 或任一 fingerprint 漂移均在零删除处返回 recovery required。删除状态机为 `planned → bootstrap_removal_started → bootstrap_removed → staged_removal_started → staged_removed → source_removal_started → source_removed → filesystem_completed+canceled`；每个 started 都是写前 intent，进程在删除前后退出均可按原指纹幂等收敛，preimport 永久不删。
- Web 在 filesystem completed 后先写 0600、token-hash 命名的 cleanup receipt，再 finalize canceled journal，最后按 receipt 精确删除 owned token。journal 已删而 token 尚在、token 删除失败、或两个并发 cancel 都能重复调用：receipt 与 token 的 instance/operation/job/type/key 必须完全相同，危险 token 删除最多一次；token 已不存在时 receipt 仍让同一 cancel 返回成功。receipt 不保存原 token、Steam 用户名/密码、session、ticket 或平台 ID。
- succeeded token 不再在 TTL 后整目录删除，而是压缩成只含 instance/save/operation/job 精确结果的 tombstone；payload/preview/lease/过期元数据被清空，同 token 仍可按 exact job 返回原幂等结果。该路径不调用 canceled cleanup，不删除 completed journal、preimport 或正式存档。主要影响 `internal/jobs`、registry `SaveImportRequest`、`save_import_transaction.go`、Web pending-upload/commit/cancel；公开 HTTP DTO、单次 FIFO、no-replace、durable save 与邀请码语义不变。Windows 专项/Web 全包、任务专属 Linux 全量 `go test ./... -count=1`、vet、build 全部通过，Docker owner 资源清零；最终候选再次通过升级后受影响链并随 v0.5.0 发布。

# SAVE-IMPORT-MAINTENANCE-DURABILITY-1：维护阶段、journal 与快照恢复闭环（2026-08-15，released in v0.5.0）

- `runImportMaintenance` 现在采用严格 write-ahead 顺序：先把数据库权威实例的 state、`state_message` NULL/空/普通值、driver phase 与 payload 原始字节编码进 operation 自有的 0600 journal；再持久化 `state=stopped / driverPhase=save_import_maintenance`；再写 `MaintenanceStarted=true + start_intent_persisted`；只有三步都成功才允许 `ComposeUp`。`updateImportMaintenancePhase`、`restoreInstanceState`、LastError 与所有清旗/恢复 journal 写入错误均沿调用链返回，不再只记日志或吞掉。
- 成功 `ComposeUp` 后立即写 `compose_up_returned`，完整 readiness 证据写入后再写 `runtime_ready_persisted`。任何启动/readiness/phase 失败只在 `ComposeDown` 成功且 exit code=0、`ComposePsStrict` fresh 证明全部 server 稳定终止、journal 成功写入 `MaintenanceStarted=false + snapshot_restore_pending`、数据库精确快照恢复成功、journal 成功写入 `snapshot_restored` 后，才回到原离线状态。任一环失败都返回 `import_recovery_required`，并让 maintenance flag 或 pending marker 继续阻止 Reconcile/cleanup 把现场伪装成普通 stopped。
- `RestoreInstanceStateSnapshot` 明确是无默认值的恢复专用 storage 契约：不会把空 phase 改为 `empty`，不会把空 payload 改为 `{}`，并保留 `state_message` 的 NULL、有效空串、普通字符串三态。journal 新快照使用 base64 保存 message/phase/payload 字节，避免 JSON UTF-8 归一化改变原始 payload；旧 schema-1 字段仍可只读恢复。
- Phase A 在真正调用 FIFO writer 前先持久化 `phaseAFifoWriteAttempted=true`。pre-submit 失败只有在该位为 false、`UpstreamSubmitted=false`、`UpstreamConfirmed=false` 且 stage 未到 submitted 时，才执行 Down + strict probe + 清旗 + 精确快照恢复；writer intent 写入后无论 FIFO 返回何种错误都视为可能提交，停止私有 runtime 但保留 maintenance ownership/manual recovery，禁止 staged/token 自动 cleanup。Panel 启动恢复覆盖 start intent、ComposeUp returned、Down 后未清旗、清旗后未恢复四个窗口；可能发生部分 ComposeUp 的窗口会先 Down 再 strict，可能发生 FIFO 写入的窗口只停机并保持 manual recovery。
- 专项覆盖 phase/journal/LastError 写入失败、Down/strict/清旗/快照恢复失败、四种离线 state、message 三态、原始字节、Phase A 零 FIFO pre-submit、FIFO 模糊提交以及四个重启崩溃点。Docker/storage/Junimo/Web 专项、Linux 受影响四包、任务专属 Linux 全量 `go test ./... -count=1`、`go vet ./...`、`go build ./...` 均通过；首次全量在同工作树的 Control DLL 与 manifest 并发更新窗口读到旧摘要而失败，二者稳定一致后以同一精确命令重跑通过。最终候选再次执行默认全量和真实升级链并随 v0.5.0 发布。

# SAVE-IMPORT-STRICT-OFFLINE-PROBE-1：存档导入严格无缓存停机证明（2026-08-15，released in v0.5.0）

- Docker Client 保留普通 `ComposePs` 的约 1.5 秒缓存、坏 JSON debug 降级与 UI/状态页语义；安全路径继续使用独立 `ComposePsStrict`，每次直接执行 `docker compose ps --all --format json`。strict 不读写普通 cache，命令失败、stdout/stderr 截断、非空坏 JSON、JSON `null`、缺 service/state 或 Docker 未知 state 均返回错误；Compose Down 后“命令成功且未截断的空 stdout”及合法 `[]` 仍表示 0 services，避免重新引入 v0.4.18 的空项目回归。
- `saveImportServerStoppedStrict` 现在只把 server 不存在，或所有 server 条目均为 `exited/dead`，认作稳定停机。任一 `running`、`Status=Up...`、`restarting`、`paused`、`created`、`removing` 均直接拒绝；空/未知状态返回不可分类错误。函数遍历全部 server 条目，不能由第一个 exited 副本遮住后续活跃或未知副本。
- 严格探针固定覆盖五个安全边界：`ImportSaveAndStart` 在 runtime asset、journal、staging/bootstrap 与 token ownership 之前；`runImportMaintenance` 初始权威检查；`ComposeUp` 前最终复验；maintenance/Phase A 失败后的 `ComposeDown` 终态证明；owned token cleanup 前。`ComposeUp/Down` 仍失效普通 cache，但 cache invalidation 不作为这些门禁的停机证据。
- 数据库实例读取成功后，maintenance 的错误 journal、staging/Phase A/activation/durable-save 与恢复均使用数据库权威 `DataDir`；调用方目录与权威目录不一致时，提交和 owned cleanup 在任何文件删除、journal 创建或 ownership 转移前返回 `import_recovery_required`。影响 `internal/docker/compose.go`、存档导入 maintenance/transaction、相应测试与 Web 提交回归；公开上传 DTO、数据库 schema 和普通状态接口不变。
- 回归覆盖 cache stopped/fresh running、cache running/fresh exited、坏/空值/缺字段/未知/截断 JSON、完整状态矩阵、多 server、`game_installed` 实际运行时零 journal/零 ownership/零 bootstrap、Web token 释放及权威目录隔离。任务专属 Linux 容器中的 Docker Client/Junimo/Web 专项、三包全量、最终默认 `go test ./... -count=1`、串行复核、`go vet ./...` 与 `go build ./...` 均通过；最终候选在升级后的 Panel 再验受影响链并随 v0.5.0 发布。

# v0.4.18 后端发布状态（2026-08-15，released）

- `RUNTIME-UPDATE-JUNIMO-MATERIALIZE-1` 与 `SAVE-IMPORT-COMPOSE-EMPTY-SET-1` 已随 `v0.4.18@56c437004b51763e77d12ffd9b716f39224d7b00` 正式发布。最终候选 `31884242692`、Compatibility `31884242697`、自动 Tag `31884612425` 与正式提升 `31884620508` 全部成功；首个候选 `31883713810` 在构建镜像前发现 Linux 非 root ownership 问题并停止，修复后没有跳过对应 integration。
- 候选从 `v0.4.17` 经公开 Web API 先完成 unhealthy `failed_rolled_back/health_check_failed`，再完成 healthy 升级；升级后的新 Panel 用原 immutable server image ID 收敛旧 `rollback_failed` 第三次 repair，保留未变化 auth 容器和 stopped 状态，并通过 `Panel Stop → 空 Compose → 存档上传受理 → 受控失败清理`。SQLite、初始化状态、Panel 数据、非目标容器/volume 与 Panel 重启均保持。
- Docker Hub、阿里云 ACR、GHCR 的 `0.4.18/latest` 六引用统一 digest=`sha256:b304e3b9c83620e94e3a16f33f5730991f74e470820a7481e696b54738eb8d74`。独立正式镜像首次启动和重启后均为 Docker healthy、`/health`/database ok、`/api/version=0.4.18@56c437004b51763e77d12ffd9b716f39224d7b00`、build date=`2026-08-15T12:18:12Z`、setup initialized=false；GitHub Release 为最新正式版且四项脚本资产与 tag 源 SHA-256 一致。完整证明和资源清理见 `docs/09-image-build.md`。

# RUNTIME-UPDATE-JUNIMO-MATERIALIZE-1：Control-only 升级补齐宿主 JunimoServer（2026-08-15，released in v0.4.18）

- 支持包证明 server/auth tag 均已是推荐版本、仅 Control `0.3.1 → 0.3.2`，但宿主 `.local-container/mods/JunimoServer` 缺失；整个 `/data/Mods` 又由宿主 bind 覆盖。旧 apply 把“server image ID 未变”等同于“不需要同步 JunimoServer”，SMAPI 因缺少 `JunimoHost.Server` 跳过 Control，目标验收报 `server_container_not_ready`，旧版回滚再报 `rollback_verify_server_failed`，两次 repair 均进入人工干预。离线校时和 SteamAPI 告警不是该终态的根因。
- apply 现在独立校验宿主 JunimoServer 的 manifest、UniqueID、版本、DLL、普通文件和 symlink 边界；server image 变化或宿主组件缺失/损坏任一成立时，均从已选可信目标 image 事务化提取并替换。server/auth tag 未变仍不重建 auth、不快照 steam-session，只把 Junimo 物化计入 server change plan。
- 回滚在启动原 server 验收前再次校验其 JunimoServer；缺失或损坏时只允许从恢复清单记录的原 immutable server image ID 提取，经过相同静态校验后原子补齐。该路径兼容旧 Panel 已生成且 `JunimoModReplaced=false` 的 `rollback_failed` 清单，使升级到本补丁后的“检测、修复并升级”能恢复现有现场；提取、路径映射或替换失败返回 `rollback_repair_junimo_mod_failed` 并继续保留私有材料。
- Linux 上 helper 容器以 root 写宿主 bind 时，提取结束会把目标树递归归还给当前 Panel 进程的数值 UID/GID；Windows 保持既有 Docker Desktop 行为。这样非 root Panel 仍可完成原子 rename、失败清理和后续 Mod 写入，不把目录永久留成 root-only。影响 `runtime_update_apply_runner.go`、`runtime_update_rollback.go`、`junimo_mod_runtime.go`、apply/integration 测试和候选升级 E2E；公开 API、数据库、runtime manifest、server/auth/Control 产物和存档格式不变。回归覆盖新 Control-only 事务缺失 Mod、旧 `rollback_failed` 恢复后继续升级、既有目标失败/成对回滚，并用真实 Docker image ID 提取内置 Mod；integration 还以非 root Panel UID 实际写入并删除物化目录。`v0.4.17 → 候选` 升级后的公开 repair API 会构造已失败 2 次的旧事务，删除原可信 tag 后要求第 3 次修复仍从 immutable image ID 补齐 Mod、保留未变化 auth 容器并恢复 stopped。

# SAVE-IMPORT-COMPOSE-EMPTY-SET-1：停服后的空 Compose 集合允许首次导入（2026-08-15，released in v0.4.18）

- 生产只读取证确认：`v0.4.17` 实例已通过 Panel Stop 成功执行 `docker compose down`，数据库为 `stopped/stopped`、活动 job 为 0、导入 journal 为 0、上传 token 仍为 `available`；实例 Compose 配置继续声明 `steam-auth/server`，但 `docker compose ps --all --format json` 以 0 退出且 stdout 为空。提交入口没有接管上传，却因 strict probe 把合法空集合误判为错误，再由 Web fallback 映射成 `save_in_progress`。
- `ComposePsStrict` 现在把“命令成功且 stdout 为空”解析为合法的空 `Services` 集合。该结果仍由 `saveImportServerStoppedStrict` 判定为没有 server 副本、可以进入维护；命令非零、非空坏 JSON、条目缺 service/state、未知 server 状态及任一 `running/restarting/paused/removing` 副本仍 fail closed。没有放宽数据库四态、两次无缓存检查、ownership/journal 前置顺序或 cleanup 门禁。
- 影响 `internal/docker/compose.go`、`compose_test.go` 与 `runtime_apply_integration_test.go`；公开 `upload-preview`、`upload-commit-and-start` DTO、错误码、hostHandling、journal 和前端代码均未变化。Linux 定向回归覆盖 strict cache bypass、成功空集合、坏 JSON、缺字段和完整 Stardew Junimo 包；随后串行全量 `go test ./...`、`go vet ./...` 与 panel build 均通过。真实 Docker integration 还实际执行 `ComposeUp → ComposeDown → 项目容器归零 → ComposePsStrict=0 services` 并清理容器/网络。
- `scripts/tests/test_release_candidate_upgrade.sh` 已加入升级后专项：候选 Panel 真实接管一个运行中的一次性 Compose 项目，经公开 Stop API 执行 `compose down`，确认项目容器为 0、Panel 容器内 `compose ps --all` stdout 为空，再上传伪存档并要求返回 `202/jobId/operationId`；受控 server 随后立即退出，job 必须失败终止、实例恢复 stopped 且项目容器/网络归零。下一正式候选必须实际跑过此链，不得用预先 `docker compose create` 的占位容器掩盖空集合路径。

# v0.4.17 后端发布状态（2026-08-15，released）

- `RUNTIME-AUTH-HEALTH-PROBE-1` 与 `SAVE-IMPORT-FIRST-INSTALL-STATE-1` 已随 `v0.4.17@d63c93ffe7d65f8cdfcf2bedb9b336a6839be73f` 正式发布。候选 run `31823172958` 和 Compatibility run `31823172972` 成功；真实 auth Docker fixture 证明 `/health` 在 Steam 离线时快速通过且不会请求挂起的 `/steam/ready`，404/500/坏 JSON/超时/不可达继续回滚。首次上传的真实 `game_installed`、四态矩阵、strict Compose、精确状态恢复和三类 job/token/journal 中断窗口由全量 Go/Web 回归固定。
- 同一候选从 `v0.4.16` 通过公开 Panel Web API 先验证不健康目标 `failed_rolled_back/health_check_failed`，再升级到精确健康候选；SQLite、初始化、Panel 数据、非目标游戏容器/volume 和重启均保持。Tag workflow `31823884131` 创建 annotated `v0.4.17`，正式提升 `31823899038` 成功。
- Docker Hub、阿里云 ACR、GHCR 的 `0.4.17/latest` 六引用统一 digest=`sha256:44c328cdf198ec888f3ec54bbe836ce114f5ac27c4ca5fb9cc63747a44083673`。独立回拉 GHCR 精确版在首次启动和重启后均为 Docker healthy、`/health`/database ok，`/api/version` 返回完整提交与 build date；GitHub Release 为正式状态且四项脚本资产齐全。完整候选证明、时间线与资源清理见 `docs/09-image-build.md`。

# v0.4.16 后端发布状态（2026-08-14，released）

- `REQUIRED-RUNTIME-STALE-STATUS-1` 已随 `v0.4.16@5fa04d137bf760d2124b75cc5e3e8e2b44ff4c7c` 正式发布。候选 run `31799350642` 通过默认并行 Go test/vet/build、真实 SMAPI/runtime integration、fresh/restart，以及从 `v0.4.15` 经 Panel Web API 的 unhealthy 回滚与 healthy 升级；SQLite、初始化状态、Panel 数据和非目标游戏容器/volume 均保持。
- Tag workflow `31799876171` 与正式提升 workflow `31799891830` 成功，三仓 `0.4.16/latest` 统一 digest=`sha256:5f07910869d6d895e40ecb3954f5905d0cb6abf830e7cf57062bbcf97ca37e0f`。独立正式镜像首次与重启后均返回 Docker healthy、`/health`/database ok 和完整 `0.4.16@5fa04d1...` 版本身份；详细失败修复、矩阵和资源清理见 `docs/09-image-build.md`。

# SAVE-IMPORT-FIRST-INSTALL-STATE-1：安装完成后首次上传沿用真实离线状态（2026-08-14，released in v0.4.17）

- 根因：安装成功会把实例写成 `game_installed`，Web 旧提交入口只排除 `running/starting`，因此允许创建 journal、接管 token、staging、preimport 和首次上传 bootstrap；但 `runImportMaintenance` 随后又把数据库状态硬限制为精确 `stopped`，在任何 `ComposePs/ComposeUp` 前返回 `instance must remain stopped before maintenance startup`。既有 maintenance 与 fresh-upload 夹具都伪造为 `stopped`，没有覆盖安装器的真实终态。
- driver 现在集中定义 `IsSaveImportMaintenanceOfflineState`：只有 `game_installed / save_required / ready_to_start / stopped` 可以启动导入维护运行时。Web commit 入口使用同一集合做早期拒绝；`ImportSaveAndStart` 在创建 journal 或转移上传 ownership 前重新读取数据库，随后整条 staging/maintenance/Phase A/activation/durable-save 链只使用该权威 `DataDir`。`runImportMaintenance` 在静态能力/指针校验后、真正 `ComposeUp` 前再检查一次。状态允许不等于 Docker 已停；安全检查固定调用无缓存、必须可解析的 `ComposePsStrict = docker compose ps --all --format json`，空响应、坏 JSON、缺失 service/state、未知 server 状态、任一 running/restarting/paused/removing 副本都 fail closed。`uninitialized`、安装/授权阶段、`starting/running`、`error` 等状态不会接管 token 或创建事务。
- maintenance 仍临时发布 `state=stopped / driverPhase=save_import_maintenance` 并隐藏邀请码，但原始状态不再被永久改成 `stopped`。启动、readiness、Phase A 提交前证据或取消失败时，只有 `ComposeDown` 成功且随后 strict Compose 证明 server 已停，才清除 `maintenanceStarted` 并通过 `RestoreInstanceStateSnapshot` 恢复进入维护前精确的 state、phase、`state_message` NULL/空值语义和 payload；状态发布、journal 清旗或恢复写入任一失败都会显式升级为 recovery required，不能被忽略。无法证明已停时保留维护 ownership，禁止伪装成可安全离线。
- `maintenanceStarted` 现在在 `ComposeUp` 前先按“可能已启动”写入 journal，只有停机复验后才清零。`CleanupUnsubmittedImport` 新增显式 `MaintenanceStarted=false` 门禁，并继续要求未提交/未确认、operation ownership、bootstrap pointer、staged target 全树 fingerprint 等既有证明；不会删除未知路径、变化后的同名存档或 preimport。
- `POST .../saves/upload-commit-and-start` 的 `{token,cancel:true}` 增加受控的历史失败清理契约。job 以 operation 派生的持久 idempotency key 创建；即使进程在 job 创建后、token 绑定前退出，重试也只从数据库 exact key 找回唯一 job。后续 `SAVE-IMPORT-TOKEN-CLEANUP-RECOVERY-1` 已收紧这里的早期结论：owned token 的 exact key 不存在不是“未启动”证明，而是身份缺失，必须 recovery required。driver 在同一个 `runtimeUpdateMu` 临界区完成四态校验、strict Compose 实停复验和 cleanup 全部门禁；完整只读计划成功后，以可恢复 cleanup 子阶段删除，写独立 receipt 后才 finalize journal 和精确删除 token。preimport ZIP 永久保留；活动、成功、已提交、身份不匹配、运行时不可证或 fingerprint 漂移都返回 busy/recovery。成功导入 token 保留可压缩的幂等结果 tombstone。
- 影响 `save_import_maintenance.go`、`save_import_transaction.go`、Web `lifecycle_handlers.go`、pending upload 测试夹具及对应测试；公开 commit DTO、hostHandling、单次 FIFO、同名 no-replace、journal 阶段和成功导入语义不变。专项覆盖真实 `game_installed` 首次上传完整走到 completed、四态允许矩阵、不安全状态零 ownership、数据库离线但 Compose running 拒绝、失败/取消/readiness 精确恢复，以及含 staged/bootstrap/preimport/owned token 的 Web 失败事务安全取消。

# REQUIRED-RUNTIME-STALE-STATUS-1：运行栈修复成功后收敛历史失败（2026-08-14，released in v0.4.16）

- 根因：`required-status.json` 既负责阻止同一 Panel/stack 的确定性失败无限重试，又被全栈升级弹窗作为当前状态读取；用户后来从维护页完成普通 runtime apply 后，`apply-status.json` 已是 `succeeded`，但普通 apply 成功路径没有同步这个 required 协调文件，旧的 `runtime_update_save_failed` 因而继续显示为当前故障。
- `ReadRequiredRuntimeUpdateStatus` 现在会对 `failed` 做最终一致性重算。只有 required 记录属于当前 Panel version 与内嵌 required stack、最新 `apply-status.json` 为 `succeeded`，且 `InspectManagedRuntimeStack` 实时结果为 `up_to_date` 时，才原子把 required phase 改为 `succeeded`、清空旧 errorCode/error 并更新时间；`manual_action`、当前 apply 失败、版本对不匹配和实际栈未达标均保持原状。
- 普通 runtime apply 写出成功终态后会立即触发同一重算；Panel 启动自动协调和升级状态 API 读取时还会再次收敛，兼容 v0.4.15 已经遗留在磁盘的失败文件。后端接口和状态枚举不变，不删除 dry-run/apply 诊断历史，也不自动掩盖需要人工恢复的状态。
- 影响 `required_runtime_update.go`、`runtime_update_apply_runner.go` 与 `runtime_update_apply_test.go`。回归覆盖真实当前失败不被清理、普通 apply 成功立即清理，以及已有成功 apply + 历史 required failure 在首次读取时持久化收敛。

# v0.4.15 后端发布状态（2026-08-14，released）

- `v0.4.15@d84157dc8a3abc83d13d29c276d6ed332e901ce7` 已正式发布。Nexus 持久幂等、Control 0.3.2 自动解绑、无存档首次上传 bootstrap、二级 helper 与实例 Compose 两层宿主路径映射均在 v0.4.14/v0.3.2 Web 升级得到的新 Panel 上完成真实复验；unhealthy 自动回滚也通过。
- Release workflow `31725256195`、Compatibility `31725203858` 成功；三仓 `0.4.15/latest` 统一 digest=`sha256:b91e3cfd8175305723e0b97feb7c4c202179f2e229aff4f6145fe60b354a5c33`，逐仓 fresh/restart health/database/version 通过。完整候选身份、事务证据、资产摘要与清理记录见 `docs/09-image-build.md`。

# SAVE-IMPORT-FIRST-UPLOAD-1：无存档实例首次上传可直接进入事务导入（2026-08-13，completed，未发布）

- 根因有两层：从未启动的实例尚未把 server image 内的 JunimoServer Mod 物化到宿主，旧静态检查因此误报“需要升级”；同时零存档没有可加载的非目标维护世界，直接启动会由游戏创建无关新档，而上游又禁止导入当前活动目标。
- `ImportSaveAndStart` 现在在创建 journal、接管上传 token 前先核对 `.env` 的精确 `.125` image。宿主 Mod 缺失或无效时复用 lifecycle 的原子提取/同步，再做静态复核，不启动长运行游戏；真实 tag 不兼容仍返回 `junimo_import_unsupported`，提取、替换或复核失败返回可重试的 `save_import_runtime_prepare_failed`。
- 标准 Compose 中 Panel 看到的实例路径是 `/data/instances/...`，Docker daemon 的 bind source 则必须使用 `PANEL_HOST_DATA_DIR/instances/...`。driver 现在显式保存 container/host 数据根映射，所有会把 Panel 本地文件交给二级容器的 JunimoServer 提取、SMAPI bundled staging 和 SMAPI 安装包挂载都先做受限相对路径转换；实例 Compose 的受管 `.local-container` bind 也统一引用 `${INSTANCE_HOST_DATA_DIR}`，`Prepare` 与 runtime recovery 会原子补写该环境值并迁移旧相对路径。路径逃出数据根、映射非绝对或配置不完整时 fail closed；未设置 host data dir 的旧式同路径/测试部署保持原行为。
- staging 发现原活动存档为空时，在 preimport 已耐久且目标指纹稳定后，创建事务专属 `AnxiImportBootstrap_<operationId>` 副本，只把副本主文件改为 bootstrap 名并将 gameloader 指向它。目标存档不被改写，Junimo 始终在非目标世界执行既有 maintenance/finalizer/durable-save 链。
- journal 新增 bootstrap 名称、指纹、no-replace 发布所有权和清理完成标记。提交前取消仅在 ownership 已耐久且 pointer 仍指向该 bootstrap 时删除 pointer、副本和临时源；成功只在目标 pointer 及既有 finalizer、Control、Junimo、XML、durable-save 门禁都通过后清理。碰撞、发布后 ownership 尚未落盘、指纹/指针漂移或清理失败全部进入 recovery，不能猜测所有权或提前写 completed；preimport 继续保留。
- 影响 `save_import_bootstrap.go`、`save_import_transaction.go`、`save_import_durable.go`、`docker_host_path.go`、Junimo/SMAPI 二级容器 bind、Panel driver 初始化、Web 稳定错误映射及对应测试。upload-preview/commit DTO、管理员权限、hostHandling 和已有活动存档路径不变；正式候选真实首次上传、故障注入与升级后复验见 `docs/09-image-build.md`。
- 代码等价 `5fc7e4c` 候选已经从 v0.4.14 与 v0.3.2 的真实 Web 一键升级后完成空 saves 首次上传：错误 runtime 稳定 409 且不接管 transaction，同 token 恢复精确 `.125` 后成功；host 路径映射同时覆盖 helper 与实例 Compose bind，bootstrap 最终为 0、目标目录唯一、preimport/journal 可读，Panel 重启后仍 completed。由于官网 docs-only 提交并入后本地修复 rebase 为 `967647d`/`fd04ff0`，正式 tag 仍须从最终 SHA 重建并复验，不能复用旧镜像身份。

# NEXUS-EXT-IDEMPOTENCY-1：浏览器扩展远程安装持久幂等（2026-08-13，completed，未发布）

- `POST /api/instances/:id/mods/remote/install` 现在可接收 `Idempotency-Key`。键限制为 1–128 字节可见 ASCII；缺失时保留旧客户端的非幂等行为，非法键返回 `400 invalid_idempotency_key`。
- migration `013_job_idempotency_key.sql` 为 jobs 增加私有 `idempotency_key`，并按 `type + target_type + target_id + idempotency_key` 建唯一索引。相同键无论原任务仍活动或已经终态，都返回原 `jobId`，不会发布第二个 runner；响应为 `202 {jobId, deduped:true}`，后端日志使用 `remote mod install reused existing job`。
- `jobs.Spec.IdempotencyKey` 与 `storage.CreateIdempotentJob` 负责原子创建/复用。幂等键不进入公开 Job DTO、普通日志或审计 metadata；审计只记录复用动作和已有 jobId。管理员明确开始新的安装动作时必须生成新键。
- 专项测试覆盖 12 路并发只有一个持久 owner、终态任务复用、不同键创建不同任务、Manager 不启动重复 runner、HTTP 复用响应和非法键拒绝。扩展侧并发、失败重试与 service worker 重启测试见 `docs/03-frontend.md`。

# SAVE-IMPORT-AUTO-UNCLAIM-1：切换玩家主机导入后自动解绑全部农舍角色（2026-08-13，completed，未发布）

- 前端与上传 HTTP 请求保持不变；仅 swap_to_player（内部 swap_host_to）在 Junimo finalizer 已确认后，由后台默认提交带 preSaveAction=unbind-all-farmhands 和精确目标 saveId 的同一次 Control save-now。virtual_host_takeover/as-is 不进入该动作。
- Control 0.3.2 只在世界已加载、当前进程为服务器、实际存档与动作目标完全一致、真人 farmhand 在线数为 0 且 farmhandData 可读时执行；它遍历全部正式 farmhandData 角色，不区分已定制与空白槽，将非空 userID 清空，再启动原生 SaveGameMenu。Server 虚拟主机不在 farmhandData 中，不会被改写。
- 动作和保存共用既有 pending-save-command journal、相同 commandId 与 GameLoop.Saved 终态。Control 进程中断后只恢复同一 journal，重复清空为空操作；不会发布第二个 save-now。结果仅记录角色总数、已定制数、剩余绑定数，不记录玩家名、平台 ID 或 userID。
- 候选真实 Web 导入发现过一条后台竞争：Control 已写出带完整聚合证明的 `succeeded`，但通用 command history 同步器可能先把文件导入数据库、按公开历史白名单丢弃导入私有字段并删除原文件，durable gate 随后只能回读缺字段结果而进入 recovery。`DurableCommandResultProtected` 现在同时检查未完成 new-game owner 与 save-import journal；精确 `DurableSaveCommandID` 所属导入未到 `completed` 前，历史同步器不得搬走或删除该结果，读取 journal 失败也 fail closed。事务完成后恢复普通归档，不扩大数据库敏感字段白名单。
- Panel 只有在同一 commandId 的 Control succeeded 同时证明动作名、目标 saveId、boundFarmhandCount=0，并且 Junimo /diagnostics/state 再次读到相同总数/已定制数和零绑定后，才继续 dayTransition、稳定 XML、磁盘变化和 completed。任一来源缺失、旧 Control/DLL、玩家在线、目标错档或仍有绑定都 fail closed，实例继续保持不可加入的导入维护状态。
- 影响文件：embedded/smapi-mod-src/{ControlContract,DeferredCommandOutcomes,ModEntry}.cs、embedded/smapi-mod 0.3.2 制品、runtime_stack_manifest.json、save_import_{durable,evidence,maintenance,transaction}.go、`command_results.go`、Web `control_commands.go` 及测试。新增内部错误码 save_import_maintenance_control_mismatch；公开上传 DTO、前端模式和 SteamID 输入均未变化。
- 验证：Control 契约测试通过；使用真实 Stardew game-data 编译 0 errors（仅既有 analyzer/compiler warning）；Linux Go 全量 test/vet/build 通过。Docker Desktop 唯一隔离 project 从只读旧夹具克隆 2 个 farmhand（1 customized、1 bound），真实 Control 保存后运行态与磁盘均为 2/1/0，主文件 hash 改变；重启 server 后仍为零绑定且同 commandId 结果保持 succeeded。任务容器、网络、卷和克隆目录均已精确清理，原夹具未修改。
- v0.4.14/v0.3.2 升级得到的新 Panel 上，Control 0.3.2 又随真实首次上传完成同一 2/1/0 解绑和 durable save，Nexus 重放、错误 runtime 同 token 重试与 Panel 重启也通过；unhealthy 目标已自动恢复旧 v0.4.14 及数据/非目标资源。最终 revision 重建、tag 后 registry 回拉与 Release 证据仍以 `docs/09-image-build.md` 为准。

# STARTUP-NEWGAME-DURABILITY-1：启动诊断、新建档耐久与手动恢复（2026-08-13，released in v0.4.14）

- 修复 Control 启动验收的 pending/mismatch 混淆、1 分钟截断与 Restart 旁路；Reconcile 只有新鲜 ready 证据才提升 running，宿主/Panel 重启不自动 ComposeUp，用户手动 Start 才继续。`/state` 新增安装文件、Compose、镜像、容器及 Control static/runtime 的权威 `installationDiagnostic`。
- 新建档采用强制 Idempotency-Key、原子持久 owner/token、startup/http 固定 writer、进展 write-ahead、同 ID save-now journal、完整角色外观内存复核和双 XML 稳定门禁；rollback journal 可在用户手动 Start 后只继续原回滚。unfinished owner 阻断所有会覆盖事务现场的 mutation。
- 代码提交 `3cdf43c5a2b3055add7ed5a6720d97e24794073c` 已在 `origin/main`；全量 Go、Control 0.3.1 真实编译、双 writer 真实 Docker 建档均通过。正式 v0.4.11/v0.3.2 Web 升级、unhealthy 自动回滚、621 图形化 conversion 与升级后普通用户诊断/UI 已在唯一 DinD 通过，事务 ID 与完整矩阵见 `docs/09-image-build.md`。当前仍待最终文档 commit 重建候选与 tag 后三仓/生产收口。
- Linux CI 暴露进程内 12 路原子 owner claim 的极短可见性竞态：loser 可能在 winner 发布/同步目录的瞬间把现场误判为缺 `owner.json`。新增仅限进程内的 claim publication mutex；跨进程线性化仍由 `RENAME_NOREPLACE` 负责，未知/遗留现场仍 fail closed。Windows 与 Linux 并发专项各连续 100 次、两平台全量 Go test/vet/build均通过。
- `v0.4.12` tag 已固定在 `21fa312656f18a8bfdef7db62e224f91b3830deb`，但 Release workflow `31679615132` 在镜像构建/推送前被旧维修脚本 trap handler 的 ShellCheck SC2317 阻断，未创建 Release、未更新三仓或 `latest`。局部 ShellCheck 指令现同时说明并抑制 SC2317/SC2329，0.10.0 与 0.11.0 workflow 精确输入复验通过；依据 tag 不可移动规则，修复后正式候选改为 `v0.4.13`。
- `v0.4.13` 的 release gates 全部通过，但 workflow `31681173485` 在多仓 push 时因 ACR 不支持 Buildx 默认 `application/vnd.oci.empty.v1+json` attestation 失败；Docker Hub 已部分写入 `0.4.13/latest`，GHCR/ACR 和 GitHub Release 未更新。workflow 现显式 `provenance:false`、`sbom:false`，下一不可移动候选为 `v0.4.14`，成功后必须覆盖三仓 latest 并验证统一 digest。
- `v0.4.14` 固定在 `a70efc98feecd6b2db803435b59b0f31d1439cf3`。Release workflow `31682847388` 成功；三仓 `0.4.14/latest` 统一 digest=`sha256:5b58ad998da14726b655f4a965c0e3f74ae7839fe615b0f59dd8af1ee16a8ebd`，三个精确镜像分别通过首次启动/重启 health、SQLite 和版本验收，四项 Release 资产与 tag 源一致。生产同步因 `114.55.142.107:22` 的正确 SSH 用户名缺失而待完成，未对生产执行升级。

# PANEL-UPDATE-GRAPHICAL-COMPOSE-1：图形化 Compose 一键升级自动标准化（2026-08-12，completed，未发布）

- Panel 自更新的部署反查新增持久镜像契约探针：在隔离、无网络 helper 中显式加载部署目录 `.env`，临时注入不会被拉取的 `PANEL_IMAGE` 探针值，并要求 `docker compose config --images <service>` 精确解析到该值。探针只在容器 ID、Compose project/service、当前镜像和数据挂载已经全部反查一致后执行，不把 Compose label 漂移或服务歧义降级为自动迁移。
- 完整 Compose labels 但缺少可用 `.env`，或服务 `image` 写死、没有由 `PANEL_IMAGE` 管理时，不再错误返回“可直接升级”后于 helper 备份阶段报 `deployment_backup_failed`。满足既有安全边界的 NAS 图形化部署会返回 `conversionRequired=true`，管理员仍只需执行同一个环境检查和一键升级；独立 helper 备份数据库、原 Compose、容器环境/inspect 和旧镜像 digest，生成标准 `.env + image: ${PANEL_IMAGE}` 后切换，目标健康/版本/Compose labels 任一失败都恢复旧容器和原部署文件。
- Go 前置检查继续要求 bind 数据目录、Docker Socket、8090 映射、非 privileged、默认 user、非空 `PANEL_SECRET` 和可信当前镜像。额外挂载不再被错误限制为总数恰好两个；只允许类型为 bind/volume、目标唯一且非根目录的可保真候选，迁移脚本在切换前仍用完整 inspect 二次校验读写属性、propagation、volume name、设备、tmpfs 和网络。图形化部署因镜像 `VOLUME /data` 产生的额外匿名 `/data` volume 因此可以被保留，而不是阻断标准化。
- 已经存在 `.env` 且 Compose 使用 `${PANEL_IMAGE:-...}`、但文件尚无 `PANEL_IMAGE=` 时，apply 会在备份后原子追加目标值，避免可验证的变量契约仍因缺键失败。新增稳定错误码 `deployment_env_invalid`、`compose_image_unmanaged` 只用于无法安全标准化时的明确拒绝；API shape、管理员权限和 `confirmFullStack=true` 不变。
- 主要影响 `internal/updater/{deployment,docker_cli,service,legacy_conversion,apply_helper,types}.go` 及契约测试。专项测试覆盖标准变量部署、缺 `.env`、写死 image、安全/不安全转换、Compose service label 漂移、额外匿名 volume、apply helper conversion 参数和缺键追加。Docker Desktop 独立 DinD 已按反馈原样完成真实 Web `0.4.10 → 0.4.11`：无 `.env`、写死 image、完整 labels 和额外匿名 `/data` volume 均被自动标准化，游戏容器 ID、匿名 volume、数据库和管理员会话保持，旧 Panel 停止保留；下一正式版本仍须按 `docs/09-image-build.md` 补齐受控目标失败、中断恢复和完整发布门禁。

# INSTALL-FIRST-RUN-CONSISTENCY-1：安装排他与首次建档 SMAPI 物化（2026-08-11，released in v0.4.11）

- `jobs.Spec.Exclusive` 与 `Store.CreateExclusiveJob` 为同一 `type + target` 提供原子创建语义；迁移 `012_exclusive_stardew_install_jobs.sql` 先把历史重复活动安装任务中较旧者标记失败，再创建仅覆盖 `stardew_install + queued/running` 的 partial unique index。重复安装或 Steam 授权请求返回 `409 install_in_progress`，`error.details.jobId` 指向已有任务，调用方应继续观察它而不是创建第二个 runner。
- 实例阶段写入新增 `UpdateInstanceStateForActiveJob`：只有目标为该实例且仍处于 queued/running 的任务才能更新。`updatePhase` 对带 job ID 的生产任务使用该持久化 owner 校验，终态或历史任务的迟到写入被忽略；明确没有任务上下文的既有同步维护路径继续使用普通更新。Panel 启动恢复在任务仍为 active 时先写 `install_interrupted`，再统一终结中断任务。
- `Driver.EnsureManagedSMAPIBundledMods` 使用精确 server image 和当前 `GAME_DATA_VOLUME`，只读复制 `/data/game/Mods` 到同文件系统 staging，校验 manifest、必需 SMAPI 支持组件、重复 UniqueID、symlink/非普通文件和 512 MiB 上限，树摘要相同时不替换，变化时原子发布到 `.local-container/mods/smapi` 并保留失败恢复语义。下次执行会清理精确的遗留 `.smapi-sync-*`，并在 destination 缺失时恢复最近的有效 `.smapi-backup-*`，覆盖 Panel 在 old→backup→new 原子发布窗口中断的恢复。它在安装成功标记前、首次建档事务/Mod 指纹前、SMAPI staging 切换后和回滚切回旧卷后执行；安装完整性校验现在要求两个官方支持 Mod manifest，已有 SMAPI 可执行文件但缺 manifest 时会重新运行安装器而不是错误跳过。
- Linux `RunSteamAuthTTY` 的 Compose one-off 现在带随机唯一 `anxi-steam-auth-<hex>` 名称。job context 取消或超时时，前台 Docker CLI 退出后按该精确名字有界执行 `docker container rm -f`；清理不能在第一次“尚不存在”时返回，必须连续 3 秒确认精确容器缺席，以覆盖 Docker daemon 在 CLI 被取消后才完成 create 的竞态。正常结束继续依赖 `--rm`。这是容器级清理，不按 Compose project/volume 批量删除，steam-session/game-data 的持久化边界不变。
- 预期 Mod 指纹现在包含 managed `smapi` 命名空间内所有有效 manifest，而不只硬编码当前两个组件；顶层旧版 ConsoleCommands/SaveBackup 仍只作为兼容 fallback/重复隔离对象。首次创建不再依赖 server entrypoint 启动后才执行的 `init_mods` 副作用。
- 自动测试覆盖并发 12 请求只有一个 owner、409 已有任务契约、终态任务迟到写入拒绝、历史重复任务迁移、首次物化/幂等/坏 staging 不覆盖、中断 backup/stage 恢复、未来内置 manifest 进入指纹，以及同步失败发生在新建存档事务创建之前。真实 Docker gate 使用精确 `.125` server image 与唯一临时 game-data volume 验证首次物化、第二次 no-op 和 old→backup 中断恢复；`TestRealNewGameMaterializesSMAPIModsBeforeFirstSaveOptIn` 又以只读克隆的完整游戏卷和空 saves bind 两次创建真实唯一首存，日志 sequence 9 的物化严格早于 sequence 10 的事务快照，分别 71.78/60.04 秒成功且存档可解析，第二轮 Stop 后容器归零。无账号首次安装真实进入 QR 后取消：门禁先抓到“第一次为空后 daemon 晚到 Created 容器”的竞态，加入生产清理和测试各自连续 3 秒缺席窗口后，真实测试通过，外层再确认案例 container/volume 为 0。
- 正式 tag `v0.4.11` 固定指向 `ef2580d2e58b170b5e5aa0079496f969228dd3f6`。正式 Web 门禁覆盖 `v0.4.10` unhealthy 目标自动回滚后重试成功、`v0.3.2` 直升、迁移 012、升级后 1.96 GiB 真实首次建档与 QR cancel 清理；Release workflow `31521478699` 成功，三仓 `0.4.11/latest` digest 一致且首次启动/重启 health、SQLite 和版本均通过。完整事务与摘要见 `docs/09-image-build.md`。

# RUNTIME-UPDATE-REPAIR-CATALOG-3：升级故障目录与对应修复方案（2026-08-09，completed，未发布）

- `DetectRuntimeUpdateRepairPlan` 成为 `GET /junimo-update` 展示与 `POST /junimo-update/repair` 执行共用的唯一只读检测目录。返回字段包含稳定 `code`、检测证据 `detection`、具体 `method`、操作 `steps`、后端给定的 `buttonLabel`、尝试次数和 `repair|export|wait` 动作；前端不再按错误文案自行猜修复方式。
- 闭集自动修复扩大为三类：校验通过的 `rollback_failed` 按原事务幂等恢复旧版再升级；可信 `repairable/legacy_candidates` 私有备份并规范候选配置再升级；已到 `failed_rolled_back` 且当前仍是同一旧版本对时执行 `repair/safe_retry`，重新检查 Docker/Compose、磁盘、目标 digest 和健康状态，以新 apply ID 重试并保留自动回滚。
- 其它情况都有明确但不越权的应对：损坏/不可读状态、清单不匹配、恢复材料缺失或摘要漂移、自定义镜像、主 tag 歧义、未知候选和三次耗尽只允许保留现场并导出支持包；已有非终态任务显示等待自动恢复；撤回或非推荐矩阵显示等待安全版本。以上动作不会覆盖配置或猜测镜像。
- `StartRuntimeUpdateRepair` 在持锁后重新调用同一 detector，并只接受 `actionAvailable=true` 的计划；执行前仍二次验证状态/清单/材料，修复后统一进入 `resuming_upgrade`、完整 dry-run、保存/整档备份和新事务。专项测试覆盖安全重试成功、损坏状态、自定义镜像、材料篡改、次数耗尽和 API 不泄露敏感内容。
- 支持包新增白名单条目 `junimo-update.json`，包含当前公开运行栈检查、`repairPlan` 和可读取时的公开 apply 状态，整项 JSON 再经过统一脱敏。它不读取恢复 manifest/目录或原备份文件；专项测试把伪密码写入 apply status，确认 ZIP 不泄露该值、存档、事务快照或私有恢复材料。

# RUNTIME-UPDATE-DIAGNOSE-REPAIR-2：检测、修复并继续升级（2026-08-09，completed，未发布）

- `StartRuntimeUpdateRepair` 不再把“修复完成”定义为只恢复旧版本。一次确认现在闭环执行：只读故障分类 → 已知修复 → 修复验收 → 与普通升级相同的完整 dry-run → 升级前保存/整档备份 → 新 apply 事务 → 目标验收或自动回滚。最终只有目标验收通过才是 `succeeded`；修复后预检或重试升级失败则明确失败，不把“已回到旧版”冒充升级成功。
- 当前闭集检测器覆盖两类历史问题：`rollback_failed` 必须精确匹配 status/manifest 的实例、apply ID、版本对、Compose project、目标 digest/image ID 和事务资源名，并验证原 `.env`、Compose、Control 文件是非 symlink 普通文件且 SHA-256 一致；`repairable/legacy_candidates` 必须证明主镜像仍在可信仓库、`IMAGE_VERSION` 与主 server tag 一致、候选只来自当前/历史可信仓库，才允许私有备份后原子规范化。自定义镜像、损坏状态、未知候选或材料漂移一律停止。
- 修复后 dry-run 重新检查内置 tested manifest、Docker/Compose、Compose 服务模型、当前镜像、steam-session volume、可信目标候选/digest 和目标 Compose 覆盖；apply 关键预检再按实际运行容器 image ID、配置 tag 与 Control 版本/DLL 内容生成 `change_plan`。这次生产故障对应“server/auth 已是目标、只有 Control 不一致”，因此计划只更新 Control/server，不再停止或重建未变化 auth。
- 新增持久化 `resuming_upgrade`、`resumeAfterRepair` 与 `repairSourceApplyId`。Panel 在“原版本已修好但尚未创建新事务”、完整预检中、或新事务 mutation 前退出时，启动恢复会继续同一闭环；新事务恢复清单没有 mutation intent 时继续同一个 apply，首个 write-ahead mutation intent 持久化后才切换到普通事务回滚语义。修复次数跨重新升级事务保留，最多三次。
- 主要影响 `runtime_update_repair.go`、apply/recovery 状态机、已知配置修复 helper、required runtime 终态同步及专项测试。数据库、存档、game-data 和认证数据格式不变；此入口仍只覆盖 Junimo server/auth/Control，不处理 Panel helper、SMAPI staging 或未来 game/SDK apply。

# RUNTIME-UPDATE-WAL-REPAIR-1：跨版本升级事务与一键安全恢复（2026-08-08，completed，未发布）

- required runtime apply 的恢复清单升级为 schema 3。停服、Control 同步、认证快照创建、Junimo Mod 替换、`.env` 切换、auth/server 重建等每个会改变实例的步骤都先持久化 write-ahead intent，再执行 Docker 或文件操作；Panel 在调用已经到达 Docker daemon、但完成标记尚未落盘的窗口退出时，也按“可能已发生”执行幂等回滚和精确资源清理。
- `checking/pulling/backing_up` 且没有恢复清单时现在直接收敛为 `failed_rolled_back/panel_restart_before_change`，不要求 Docker 可用，也不再把“尚未修改实例”的重启误报为人工恢复。目标验收成功和回滚成功都先持久化终态，再清理认证快照、旧镜像和恢复目录；终态写入失败则保留全部材料供下次启动继续。
- 恢复校验改为绑定该事务已持久化的 `Target/Selected`，不再要求它仍等于新 Panel 当前内置推荐版本。因此旧 Panel 发起事务、随后 Panel 版本变化时，仍可对原事务完成验收或回滚；目标 image/digest/image ID、原版本和 apply ID 必须与持久化状态精确一致。
- 私有 `original.env`、Compose、Control manifest/DLL 在 schema 3 中记录 SHA-256，恢复前拒绝缺失、symlink、非普通文件或摘要漂移。认证卷回滚使用升级前捕获的 immutable image ID；快照卷只按经过校验的 `project + applyId` 精确名称清理。
- `rollback_failed` 新增 `StartRuntimeUpdateRepair`：只重试同一份恢复清单中的原版本回滚，不接受调用方提供路径、镜像、apply ID、命令或策略；最多三次，成功后进入 `failed_rolled_back`，失败继续保留恢复目录和 volume。Junimo Mod 回滚已改成可重复调用，能够识别“失败目标已移开、原目录已恢复”的部分成功状态。
- 主要影响 `runtime_update_apply{,_runner,_repair,_rollback}.go`、`junimo_mod_runtime.go`、Web repair handler 和专项测试。公开状态只新增可选 `repairAttempts`；数据库 schema、存档、game-data 与普通 Steam 登录数据格式不变。Docker Desktop 和全量门禁证据记录在 `docs/09-image-build.md`。

# RUNTIME-UPDATE-PRESERVE-AUTH-1：运行组件按差异升级（2026-08-08，completed，未发布）

- 生产 `0.4.5 → 0.4.8` 后的 required runtime 同步暴露出事务缺陷：server/auth 已是 `.125 / 1.5.0-anxi.2`、仅 Control 需要 `0.2.2 → 0.3.0` 时，旧实现仍停止整个 Compose、快照认证卷并 `--force-recreate` steam-auth。目标认证在网络退避约 400 秒后才恢复，而默认 auth 验收只有 90 秒；目标与回滚先后超时，最终落入 `rollback_failed`。
- `runtimeUpdateRecoveryManifest` schema 2 记录 server/auth 是否真正变更及是否创建认证快照；schema 1 仍按旧事务保守恢复。Control-only 或其它 auth digest/tag 未变化的升级只停止并重建 server，不创建/恢复 steam-session 快照，也不调用 Steam readiness 网络接口；宿主 JunimoServer 已完整且匹配时不替换，缺失/损坏时会从同一 immutable server 版本补齐。原 auth 只核对 image ID、running/healthy，并以 `docker update --cpu-shares 256` 原地同步资源权重，容器 ID 和会话保持。
- auth 版本或 image ID 真正变化时仍执行停止、私有卷快照、强制重建、服务接口验收及成对回滚，默认 auth 验收窗口由 90 秒提高到 10 分钟，覆盖当前镜像的五轮连接退避。server 冷启动 20 分钟、停止重试 10 分钟和总 job 2 小时不变。
- 新增 `RuntimeComposeUpServicePreserve`（`compose up --no-recreate`）和受闭集约束的 `RuntimeUpdateServiceCPUShares`；服务名仍只允许 `server/steam-auth`，权重只接受 server=768、auth=256。公开 API、数据库和运行栈清单未改变。
- 自动测试以 auth readiness 强制失败证明 Control-only 不再进入网络探针，并断言无 auth stop/up、无认证卷快照/恢复、仅 server stop/up；最终 `go test ./... -count=1`（63.4 秒）、`go vet ./...`、`go build ./...` 全部通过。Docker Desktop 使用只读 `.125 + auth .2 + Control 0.2.0` 夹具复制出两个唯一隔离 project/volume，停止态与运行态真实升级均成功；运行态 auth 容器 ID 前后完全一致、CPU shares=256、server=768，Control 0.3.0 实载且原运行状态恢复。

# v0.4.8 发布收口：玩家 Mod 服务端链路（2026-08-07，released）

- annotated tag `v0.4.8` 精确指向 `0c5e2c434a92e8c9a69f839b39f86508cccf9a77`。Release workflow `31117969497` 第三次尝试在 GitHub Actions 恢复后用时 6 分 10 秒成功；compatibility run `31117949897` 的第二次尝试用时约 2 分 03 秒成功，远端 backend、frontend 与 isolated Docker integration 均实际执行通过。
- Docker Hub、阿里云 ACR、GHCR 的 `0.4.8/latest` 六个 OCI index digest 均为 `sha256:5381009b807ad2c632075332e3538297b5069eff2f2b1b133ff7fffd2ac38f90`；version=`0.4.8`、revision=`0c5e2c434a92`、created=`2026-08-07T04:50:35Z`。三个 registry 的精确镜像分别启动并通过 `/health` 与 `/api/version`。
- 接口契约保持 `GET /api/instances/:id/players/:uniqueMultiplayerId/mods`，列表仅带轻量 `modRiskFlags`，完整清单仍只在独立 sidecar 与详情响应中。发布没有新增加入拦截、踢出、封禁或自动处罚，也没有实现 Steam SDR 桥。
- 真机证据仍仅覆盖 PC + Stardew 1.6.15 + SMAPI 4.5.2 的标准 IP reported、断线、重连和服务器重启；PC 原版、两种官方 CJB、Android/iOS、Android 实验性 SMAPI及多个真实远端玩家并发未验证，不能由 fixture 外推。

# PLAYER-MOD-BUILTIN-FILTER-1：玩家 Mod 比较隐藏面板内置组件（2026-08-06，completed，v0.4.8 released）

- `GET /api/instances/:id/players/:uniqueMultiplayerId/mods` 的比较层现在按 UniqueID 大小写不敏感忽略 `Pathoschild.SMAPI`、`JunimoHost.Server` 和 `AnXiYiZhi.StardewAnxiPanel.Control`。它们不再进入 `serverContext.loadedMods`、comparison items 或 summary，也不会产生匹配、玩家缺少、玩家额外安装或版本不同提示。
- `apiVersion` 仍保留在玩家/服务器上下文元数据中，供详情页显示实际 SMAPI 版本；只是取消原来的虚拟 SMAPI match/version_mismatch 条目。其它第三方 `server_only` 仍遵守“缺失不告警、双方都有时可作服务器专用信息”的既有规则。
- 比较 items 的稳定顺序改为 `client_only → missing_on_client → version_mismatch → match`。该调整只影响只读展示，不改变 ModContext 采集、玩家加入、认证、踢出、封禁或拦截链路。
- 影响 `player_mods.go`、driver/API 契约测试；定向测试及最终 `go test ./...`、`go vet ./...`、`go build ./...` 全部通过。没有修改 Control DLL、Junimo/SMAPI 上游或 sidecar schema。

# PLAYER-MOD-CJB-LIST-1：玩家名册 CJB 轻量提示（2026-08-06，completed，v0.4.8 released）

- `GET /api/instances/:id/players` 的每个玩家新增可选 `modRiskFlags:string[]`；当前只可能包含 `"cjb"`。字段来自该联机 ID 最后一份通过既有上限、状态、ID 和去重校验的 `player-mod-contexts.json`，不会把 `mods` 或比较 items 塞进高频玩家响应。
- `ListPlayers` 在完成在线快照、SQLite 名册合并和人物 capability 后，一次读取 sidecar 并按 `uniqueMultiplayerId` 补轻量标记；损坏、超限、未知 ID 或无报告时省略字段。`stale` 历史仍可保留上次检测标记，因此它表示“最近一次有效客户端自报命中”，不是服务端文件审计或当前在线事实。
- 该字段只供列表/待认证卡片显示，不进入批准、踢出、封禁、连接拒绝或其它管理判断。官方 ID 与绕过边界仍为 `CJBok.CheatsMenu` / `CJBok.ItemSpawner` 的不区分大小写精确匹配；修改 manifest ID 仍可绕过。
- `TestListPlayersAddsCompactCJBFlagWithoutEmbeddingMods` 覆盖 CJB/非 CJB 两名玩家、大小写匹配、JSON 字段与完整 Mod 信息不泄漏；定向 driver/Web 测试通过。

# PLAYER-MOD-COMPAT-1：玩家 Mod 查看第三阶段（2026-08-06，PC+SMAPI 真机部分通过，其余矩阵受限，v0.4.8 released）

- `PlayerModContextLifecycle` 现在承载 Control peer 事件实际共用的连接、完整上报、断线和 pending 超时转换；`ModEntry` 的三个玩家 Mod 事件处理器只调用该只读生命周期并原子写 sidecar。契约测试直接覆盖 context 先于 connected、原版/移动端 pending→unavailable、reported→stale→重连 pending→新 reported、进程重启统一 stale、多玩家隔离和旧清单不串号。
- C# 边界补充两条官方 CJB 的精确 ID/名称/版本、大小写重复 ID、超长/控制字符、超量整份拒绝。Go 补充后端再次规范化与去重、1024 上限拒绝、`server_only` 缺失不告警但客户端确有上报时可作 match 展示，以及 peer 采集处理器不得引用 `Game1.server`、kick、ban 或命令目录的静态回归检查。
- Web 新增 loopback TCP 联调：使用真实 handler、SQLite store、实例文件系统、`options.json` 与 `player-mod-contexts.json`，覆盖 unavailable/reported/pending/stale、两类 CJB、未知 playerId 不继承其它玩家上下文；仅 Docker 进程状态使用测试替身。比较器覆盖 match、missing_on_client、client_only、version_mismatch、面板内置组件过滤与实际 loadedMods 基线。
- Control `0.3.0` 已在一次性 .NET 6 容器中，以只读 `stardew_game-data` 的真实 SMAPI/游戏程序集重新编译：0 errors、1 个既有 analyzer/compiler warning。阶段三提升时的源码构建产物、嵌入 DLL 与运行栈清单 SHA256 均为 `b15479eda376f386fdadc1cc9d0c31815cabc0a919d0655debe6d7117352c05a`；该嵌入 DLL随后完成真实 LAN 联调。最终复验在不同容器项目路径下仍为 0 errors，但 DLL 字节摘要不同，因此不把跨路径 fresh build 写成可复现摘要；嵌入 DLL 与 runtime manifest 仍保持一致。
- 最终门禁通过：Docker .NET 6 C# 契约；`go test ./... -count=1`、`go vet ./...`、`go build ./...`；玩家 Mod driver/Web 定向详细测试（含 loopback TCP）；前端 12 项状态测试、独立 `tsc -b` 与 production build。任务专属容器/卷均已精确清理。
- 补充真实联调：本机 Steam 游戏 `1.6.15` + SMAPI `4.5.2` 通过 LAN/IP 加入任务专属隔离服务器 `127.0.0.1:24682`，客户端实际加载 Content Patcher `2.9.0`、Farm Type Manager `1.26.1`、Save Backup `4.5.2`。Control 收到同一 peer 的完整 context，sidecar/API 均为 `reported`，三个 ID/名称/版本准确。该次历史抓取发生在内置过滤调整前；按当前契约 SMAPI/Control/Junimo 均不进入比较，只保留 Save Backup match 与前两项 client_only。
- 同一真实客户端主动断开后记录变为 `stale`，再次加入以相同 `uniqueMultiplayerId` 写入新的 `reportedAt` 并恢复 `reported`；隔离 server 受控重启成功且旧 context 仍为 stale、在线列表只剩 host。整个过程没有踢出、封禁或拦截调用。测试使用 `%LOCALAPPDATA%/Temp/sap-player-mod-real-20260806`、独立端口、克隆 game-data volume 和新建测试存档，未复用用户存档/设置。
- 隔离限制：driver 的游戏 Compose project 实际仍取实例目录 basename `stardew`，没有继承任务的 Panel project 名，因此 steam-auth 复用了创建于 2026-07-06 的既有 `stardew_steam-session` volume。测试没有读取 token 内容，清理时保留该旧卷；标准 LAN/IP peer context 结果不依赖邀请码，但本次不能宣称认证卷完全隔离。
- 尚未有条件完成 PC 原版、两种官方 CJB、Android/iOS 官方客户端、Android 实验性 SMAPI以及多个远端玩家同时在线；这些继续明确为未验证，不能从上述单一 SMAPI 客户端结果外推。实体页面视觉联调也未由真实客户端握手替代。详见 `docs/06-integration.md`。本阶段未实现 Steam SDR 桥，也未发布或打 Tag。

# PLAYER-MOD-CONTEXT-1：玩家 Mod 查看第一阶段（2026-08-06，completed，v0.4.8 released）

- Control Mod `0.3.0` 在标准 IP 联机链路订阅 SMAPI `PeerContextReceived`、`PeerConnected`、`PeerDisconnected`：以 `uniqueMultiplayerId` 保存 `hasSmapi/gameVersion/apiVersion/mods/contextStatus/reportedAt`。连接但尚无完整上下文先记 `pending`，10 秒后仍未收到则记 `unavailable`；断开记 `stale` 并保留最后一份完整报告。Control 启动时会把上个进程留下的记录统一转为 stale，绝不把旧清单当成当前在线事实。
- 独立 sidecar 为 `<dataDir>/.local-container/control/player-mod-contexts.json`，schema=1，继续复用 Control 既有同目录临时文件 + replace 原子写。完整 Mod 数组不进入每 120 tick 刷新的 `players.json`。`pending/unavailable` 与所有没有真正上报清单的记录固定为 `mods:null`；只有收到完整 peer context 才允许 `reported`，真实报告零项时才写 `mods:[]`。
- 客户端上下文按不可信输入处理：最多保留 512 名历史玩家、每名最多 1024 个不同 Mod、最多检查 2048 个原始条目；player ID、UniqueID、名称和版本分别限制为 32/256/256/64 字符，移除控制字符、trim，并按 UniqueID 不区分大小写去重。超过 Mod 数量边界时整份上下文降为 `unavailable + mods:null`，不会用截断清单继续比较而产生误报。
- 新增只读登录接口 `GET /api/instances/:id/players/:uniqueMultiplayerId/mods`。后端以本次服务器进程实际写入的 `options.json.source=smapi-runtime`、`options.json.loadedMods` 为唯一服务器集合；磁盘 Mod 目录只通过现有 `ModInfo` 和 `ApplyModSyncClassification` 补名称、`syncKind` 与 built-in 信息，不会把“已安装但本次未加载”的目录算入服务器集合。
- 比较 item 的 `result` 固定为 `match / missing_on_client / client_only / version_mismatch`；客户端或服务器上下文不能可靠使用时，`comparison.status=unavailable`。`missing_on_client` 只对实际加载、enabled 且 `syncKind=client_required` 的服务器 Mod 生成；`server_only` 和 `unknown` 缺失不会误报。当前比较会完全忽略 `Pathoschild.SMAPI`、`JunimoHost.Server` 与 `AnXiYiZhi.StardewAnxiPanel.Control`，但上下文元数据仍返回双方 apiVersion。
- 客户端报告中 `CJBok.CheatsMenu`、`CJBok.ItemSpawner` 按不区分大小写匹配，item 与顶层只返回 `riskFlags:["cjb"]`。接口、Control 事件处理和比较器均没有 kick、ban、拒绝连接或其它管理动作；本阶段不修改 Junimo/SMAPI 上游，也不实现 Steam SDR 兼容桥。
- 主要文件：`embedded/smapi-mod-src/{ModEntry,ControlContract,PlayerModContexts}.cs`、Control 契约测试、嵌入 manifest/DLL、`player_mods.go`、`player_mods_test.go`、`players_handlers.go`、`instance_handlers.go` 与 HTTP 测试。第三阶段重编译后的 Control DLL SHA256 为 `b15479eda376f386fdadc1cc9d0c31815cabc0a919d0655debe6d7117352c05a`；运行栈 identity 仍为 control `0.3.0`。
- 验证覆盖原子 null/empty 契约、字符串/数量/去重/断线 stale、真实 SMAPI game-data C# 编译、实际 loadedMods 基线、server_only 排除、三类面板内置运行组件过滤、两类 CJB、不可用边界、登录/方法/错误码和 HTTP JSON。完整接口和尚不能取得 ModContext 的客户端边界见最新后端接手文档和 `docs/06-integration.md`。

# MOD-INSTALL-TIME-1：Mod 安装时间持久化（2026-07-31，completed，未发布）

- `ModInfo` 新增可选 `installedAt`。所有人工 ZIP、Nexus 和远程安装路径继续统一经过 `stardew_junimo.uploadModZip`；每个成功 ZIP 在目录全部落盘后生成一次 UTC RFC3339Nano 时间，同一 ZIP 中的多个 Mod 共用该值。
- 时间存入实例私有的 `.local-container/control/mod-install-times.json`，schema 固定为 1、文件上限 2 MiB、临时文件权限 `0600` 并同目录原子替换。同一 Panel 内的安装/删除 sidecar 读改写受互斥保护，人工上传和异步一键安装并发完成时不会互相覆盖。旧实例没有记录时字段省略，不根据目录 mtime 或 Nexus `updatedAt` 伪造历史安装时间。
- 安装时间写入失败会删除本次刚移动的全部目录并让安装失败；远程幂等跳过已有 Mod 不刷新旧时间。删除单个 Mod、同包删除和多 ZIP 请求回滚都会清理对应 sidecar 项；损坏的补充元数据不会阻断既有 Mod 列表，后续安装则拒绝覆盖损坏文件。
- 回归覆盖单/多 Mod 同批时间、两个入口并发安装不丢记录、重启式重新列表、sidecar 写失败回滚、远程幂等不刷新和删除清理。Linux Docker 全量 `go test ./... -count=1`、vet/build 与 Docker integration 已通过；本地候选镜像 smoke 见 `docs/09-image-build.md`。
- Docker Desktop 真实 Panel HTTP E2E 另验证了深层目录 ZIP、本地第二包、浏览器扩展一键远程下载任务三者分别写入严格递增时间；Panel 容器重启后三值不变。有效包后接无 manifest 包会回滚目录和 sidecar；把 sidecar 路径注入为目录时，导入失败且新目录被删除；单 Mod 删除同步移除时间记录。

# SMAPI-DOWNLOAD-RESUME-1：受审加速源分块续传（2026-07-28，v0.4.5 released）

- 真实国内主机复现表明，SMAPI 4.5.2 官方 installer 为 `41,889,142` 字节，GitHub release-assets 链路约 `40 KiB/s`；旧实现把整个响应放在单个 2 分钟 `http.Client.Timeout` 内，游戏本体 413150 与 SDK 1007 已成功后仍会稳定报 `context deadline exceeded while reading body`。同一实例连续 7 次失败，磁盘、内存、Docker 与 Steam 授权均不是根因。
- `ensureRecommendedSMAPIArchive` 改为固定 2 MiB 的 HTTPS Range 下载。每个 Range 仍独立限制 2 分钟；响应在超时/断流前只要收到部分合法区间字节，下一次即从已写入偏移继续。只有连续 4 次完全无进展才失败，外层安装 job 的既有 2 小时总门限仍负责最终取消。
- 每个响应必须是 `206 Partial Content`，且 `Content-Range` 的起止、总长度必须与请求和内嵌清单精确一致。下载完成后仍执行固定 `archiveBytes`、SHA-256、ZIP 路径/条目数/解压上限/压缩比及 Linux+Windows installer 唯一结构校验，通过后才原子替换私有缓存。
- embed `runtime_stack_manifest.json` 固定候选顺序为 `gh.llkk.cc → github.dpik.top → ghfast.top → GitHub 官方`。Go 与 Python 发布门禁都按 SMAPI 版本拼出并逐项比对精确 URL 模板和六项 host allowlist；实例 `.env.SMAPI_DOWNLOAD_URLS` 仍不能新增、换序或覆盖下载目标。
- 同一候选超时/断流时先按偏移续传；连续 4 次无进展、Range 契约错误或整包校验失败才清空临时文件并切换下一候选，不跨来源拼接正式缓存。GitHub 官方始终为最后兜底。
- 主要文件：`backend/internal/games/stardew_junimo/{config/runtime_stack.go,config/runtime_stack_manifest.json,smapi_archive.go,smapi_archive_test.go,smapi_archive_integration_test.go}`、`scripts/compatibility_matrix.py`。回归覆盖正常多 Range、半包后精确偏移续传、坏代理整包后切官方、连续无进展有界失败、非法 `Content-Range` 和最终 SHA 不匹配不落盘。
- Docker Desktop Linux 容器从空缓存通过受审候选下载 `41,889,142` 字节耗时 `2m26s`，固定 SHA/ZIP、缓存 `0600` 和 `.part` 清理均通过；本地 `0.4.5` Panel 镜像 health/version、Docker/Compose 与隔离 volume 冒烟通过。
- 发布工作流 `30369196944` 全绿；Docker Hub、阿里云 ACR、GHCR 的 `0.4.5/latest` OCI index digest 均为 `sha256:a8155defc50690b8b1e90c95f5b107e818b5438c68c341f90f9ebf8b7be428ad`。从 ACR 重新拉取的正式镜像返回 `version=0.4.5`、`commit=09250ed68ce9`。
- 发布后 Docker Desktop 使用 ACR 正式镜像分别执行 `0.4.4 → 0.4.5`、`0.4.3 → 0.4.5`、`0.3.13 → 0.4.5` 的生产 `RunApply`：三次均为 succeeded，目标 health/version、非空 SQLite 与 setup 查询、扫描路径 404 正确，同 Compose game 容器 ID 保持不变。确认 tag 后后端代码未变化后，再从空缓存执行真实 SMAPI 下载，`41,889,142` 字节耗时 56 秒并通过 SHA/ZIP、`0600`、无 `.part` 残留；半包续传、坏代理回退、无进展与错摘要专项全绿。

# SAVE-BACKUP-EAGER-MAINTENANCE-1：游戏日回档即时消费（2026-07-28，completed）

- 根因：Control 在每次 `GameLoop.Saved` 后都会写入独立 `save-events/*.json`，但 Panel 过去只在管理员请求备份列表时调用 `RunBackupMaintenance`。多天事件积压后，消费每个事件时都读取同一份“当前”存档，因而全部覆盖成最新游戏日的 `auto_<save>_<ordinal>.zip`；最终虽有五档，日期可能跨过很多游戏日。
- Panel 启动后现在为所有 Stardew 实例启动独立维护循环：启动即补扫一次，之后每 2 秒消费事件。它不依赖浏览器打开存档页，也不轮询 Junimo API；权威触发仍是 Control 在存档落盘完成后产生的 `GameLoop.Saved` 文件事件。
- `RunBackupMaintenance` 增加按规范化实例目录的进程内互斥，后台循环与 `GET .../saves/backups` 的兼容性补扫不会同时打包或争抢同一事件。失败事件保留等待下一轮，成功后才删除；手动、删除前、回档前、人物删除前、导入前和运行组件升级前保护备份仍不参与五档清理。
- 回归测试连续模拟 7 个游戏日且从不请求备份列表，最终严格保留连续第 3–7 日五档；另以 8 个并发消费者证明单个事件只消费一次。后端全量 `go test ./... -count=1`、`go vet ./...`、`go build ./...` 通过。
- `v0.4.4` 已由 commit `d5d815d365cb` 正式发布；Docker Hub、阿里云 ACR、GHCR 三仓精确 tag 的 OCI index digest 均为 `sha256:446b168c8784b3c7e77c5006b85adcbe2c1b106e80992281a929a75108fd572a`。从 GHCR 实际拉取的正式镜像通过 `/health`、`/api/version` 与 OCI version/revision 复核。

# PANEL-HEALTH-WATCHDOG-1：一分钟健康监控与仅 Panel 自恢复（2026-07-26，completed）

- Panel 启动独立的进程内健康监控：首次检查在启动一分钟后执行，之后每分钟复用 `/health` 的 `Store.Ping` 数据库探针，单次超时 5 秒。只有探针返回 SQLite 原生 `SQLITE_INTERRUPT`（primary code 9）才累计；成功、超时及其它错误都清零。连续第三次 code 9 时 Panel 记录错误并以状态码 1 退出。
- 生产启动路径不再把任意业务数据库操作纳入退出计数。退出后只依赖标准 Compose 的 `restart: unless-stopped` 拉起 Panel；不调用 Docker API，不重启游戏容器。Docker HEALTHCHECK 同步为每分钟一次，但 unhealthy 只表达状态，本身不会触发 Docker 重启。
- 新增 `cmd/panel/health_monitor_test.go`，覆盖成功/非 interrupt 清零、错误文本不冒充原生 code 9、第三次连续 interrupt 才触发且触发后停止。后端全量 `go test ./... -count=1`、`go vet ./...`、`go build ./...` 均通过。
- Docker Desktop 29.5.3 正式 `0.4.3` 镜像验证：镜像 health interval=60 秒、timeout=5 秒、retries=3；`restart: unless-stopped` 下测试 Panel 主进程退出后 RestartCount `0 → 1`、PID `14558 → 14689`，重启前后 `/health` 均为 ok。Linux 容器内取消恢复和健康门限回归连续 10 轮通过；公开 `0.4.2 → 0.4.3` updater 一键升级、数据卷/404/游戏容器隔离与旧 helper 清理均通过。`v0.4.3`、三仓同 digest 镜像和 GitHub Release 已发布，测试容器和卷已清理。

# PANEL-SQLITE-INTERRUPT-1：扫描路径与 SQLite 取消恢复（2026-07-24，completed）

- Panel HTTP server 构造时只执行一次 `AdminExists`，并用原子布尔值缓存初始化状态；普通请求和 `/api/setup/status` 不再重复查询 SQLite。首个管理员事务成功后立即把缓存更新为 `true`，后续默认实例准备失败也不会让内存状态与已提交的管理员记录相反。
- HTTP 入口先识别公开 API、静态资源和 Stardew SPA 路由。未知页面/API 路径直接返回 `404 not_found`；只有 `/instances/stardew` 及九个明确子页面允许回退 `index.html`，扫描器的任意路径不会进入 SPA 或初始化门禁。
- SQLite 驱动从 `modernc.org/sqlite v1.29.10` 升到 `v1.54.0`。存储层使用观察包装器保留 `IsValid`/`ResetSession` 及 rows/stmt 可选接口，使被 `sqlite3_interrupt` 标记的文件连接在归还连接池时被淘汰；一个请求取消后，下一次查询会使用可用连接。
- 包装器按 SQLite 错误码观察完整数据库操作；成功结果清零计数，连续三次原生 `SQLITE_INTERRUPT` 时 Panel 记录错误并以状态码 1 退出，由 Docker restart policy 重启容器进程。单次正常的 `context.Canceled` 不触发退出保护。
- 回归覆盖真实长递归查询取消后立即 `SELECT 42` 成功、连续错误码 9 触发三次门限，以及关闭数据库后未知路径/缓存 setup status 仍不访问存储。验证通过 `go test ./... -count=1`、`go vet ./...`、`go build ./...`。

# SAVE-NAME-ENCODING-DELETE-1：存档上传编码与删除一致性（2026-07-20，completed）

- ZIP 预览在安全校验、布局校验和解压前统一把旧式 GBK/GB18030 路径名转换成 UTF-8；持久 token、事务 journal、API 名称和落盘目录因此使用同一组字节。上传包必须同时包含 `<存档名>/<存档名>` 与 `<存档名>/SaveGameInfo`，并拒绝重复、大小写冲突、控制字符、超长名称及 Junimo 命令不支持的名称。
- 旧版本遗留或手工复制的非 UTF-8 目录不再经过 JSON 的 `U+FFFD` 替换字符寻址。列表提供稳定、可逆查找的公开名称和 `nameWarning`；正常 GBK 名优先显示中文，无法识别或与现有 UTF-8 目录冲突时使用 `encoding_error_<hash>`，避免同名误删。此类目录只允许备份、导出和删除，不允许设为启动存档。
- 删除接口先确认目标存在，删除前强制创建 `predelete` 备份；活动指针先清除，指针失败时不删除，目录删除失败时尝试恢复指针。成功删除活动存档后实例状态回到 `save_required`；重复删除稳定返回 `404 save_not_found`，不再出现“磁盘已删但 API 报失败”的模糊结果。
- 验证：后端全量 test/vet/build、Linux 原始 GBK 文件名测试、Docker integration、兼容矩阵与远端制品校验均通过；Docker Desktop 隔离 Panel 实例完成 GBK ZIP HTTP 预览、历史乱码目录列表/激活拦截/删除/备份、重复删除和重启持久性验证。

# FARMHAND-DELETE-1：运行中删除离线存档人物（2026-07-18，completed）

- 新增管理员接口 `POST /api/instances/:id/players/delete-farmhand`，只在实例运行且请求显式确认、活动存档未变化、目标是当前存档中已认领且离线的非主机人物时创建 `stardew_farmhand_delete` job。被删人物上线会在删除前二次校验中中止；其他真人玩家在线不阻断。
- job 先通过 Control `save-now` 等待同一 commandId 的 `GameLoop.Saved`，再创建 `prefarmhanddelete_*` 整档保护备份；随后调用 Junimo 官方 `DELETE /farmhands?playerId=` 删除人物、小屋及 slot 记录，复核 `/farmhands`，再次持久保存并解析磁盘主存档确认人物消失。Panel 不直接改 Stardew XML。
- 有其他真人玩家在线时，删除前后都通过 Control `broadcast` 发游戏内通告，并等待命令结果；目标人物在线仍返回 `farmhand_online`。人物名册使用删除墓碑立即隐藏已删人物，后续权威存档再次发现同 ID 时自动清除墓碑，以支持整档恢复。
- 删除 job 活动期间拒绝启停、重启和回档，避免与保存/删除边界竞态。迁移 `011_player_roster_character_delete.sql` 增加墓碑与 operation ID；备份列表新增 `prefarmhanddelete` 类型。
- 验证：相关后端包非缓存测试通过；Docker Desktop 隔离 project `farmhand-delete-e2e` 使用真实 `.125`、真实双人物/三小屋存档完成停服拒绝、确认门禁、删除、两次保存、人物 `2→1`、小屋 `3→2`、重启不复现、重复删除安全失败、保护备份自动回档后人物和小屋恢复；真实 broadcast command result 为 `succeeded`。

# IMAGE-CLEANUP-1：升级成功后的旧镜像定向清理（2026-07-17，completed）

- Panel 自升级在新容器完成 health、HTTP health 和精确版本三项验收后，先重新 inspect 升级前 tag；只有 tag 仍指向事务记录的原 image ID 时，才执行不带 `--force` 的 `docker image rm <old-ref>`。tag 漂移、仍被其他容器引用或 Docker 删除失败只写 warning，不把已验收的新 Panel 回滚或伪报失败。
- 跨版本首发兼容不能依赖旧 helper 已包含新代码：新 Panel 启动后会后台观察同一 `apply-status.json`；仅当旧 helper 写入 `succeeded`、`toVersion` 等于当前构建版本且完整镜像元数据存在时，幂等补做清理并写 `cleanupCompleted=true`。活动阶段继续等待，失败/回滚/其它目标直接退出；新 helper 自己完成清理时也写同一标志，避免重复执行。
- Panel helper 随后按同一 OCI title label 枚举历史镜像，并只处理内置可信仓库中的稳定 semver tag 或陈旧 `latest`；当前目标、所有现存容器的镜像引用/ID、自定义仓库和未知 tag 均受保护。每个候选删除前再次核对 image ID且不带 `--force`；最后执行同 label 的默认 dangling prune，没有 `-a`，不会清理其它项目、容器、volume、数据库、存档或 Mod。隔离 Docker integration 会拦截宿主级 prune，只验证本测试旧精确镜像已删除。
- Junimo server/steam-auth-cn 成对升级仅在目标 pair 验收、原运行状态恢复、认证快照和 recovery 收尾之后，按 recovery manifest 中的旧引用与旧 image ID 分别核对并执行不带 `--force` 的删除。普通失败、成功回滚和 `rollback_failed` 都不进入镜像清理；Panel 重启后安全续验成功走同一清理路径。
- Docker 扩展新增受限 `RuntimeRemoveImage`：仍使用既有精确 tag 白名单，拒绝 `latest`、digest 混合引用和非法 image ID；删除前再次 inspect 防止 mutable tag 竞态。清理失败写入 apply warnings，管理员可据此检查共享容器引用。
- 主要文件：`cmd/panel/main.go`、`internal/updater/{apply_helper,service,types}.go`、`internal/docker/runtime_apply.go`、`internal/games/stardew_junimo/runtime_update_apply{,_runner}.go` 及对应测试。验证覆盖轻量全量、Panel Docker 成功/回滚、真实 `0.3.7` 镜像跨旧 helper 启动收尾、Docker image ID/容器引用保护，以及真实 `.121 → .125` stopped/running 成对升级。

# SAVE-IMPORT-E2E-RELEASE-1：发布前真实 E2E 门禁（2026-07-16，未完成）

- 本轮只读盘点发现隔离的 `save-import-spike` Docker 项目及三个已停止的 server/auth 组合；没有启动这些容器，也没有操作生产实例。现有证据归档仅有 `original-saves-before-import.tgz`（SHA256 `8bfd50bf7c5f9b8aa94385d5c0c80afcf892c168b49f801e9495033bf1c5109d`）和 `generated-sources-before-import.tgz`（SHA256 `9863d782aa397ff693be9753b2050666529cbbd8aaaa8a3ab5566e5741a64d95`）。它们不是本门禁要求的八类原始 ZIP 清单，无法证明住宅、家具、冰箱、地窖、配偶、孩子、宠物、farmhand 或 Mod 场景。
- 本机也没有已确认可用于人工语义验证的游戏客户端/测试账号组合。因此未执行真实上传、FIFO import、玩家选角、断线重连、睡觉跨日、故障注入或第二次重启；不得据此把 `SAVE-IMPORT-JUNIMO-1` 标记为 completed。
- 自动化门禁通过：`go test ./internal/games/stardew_junimo -run 'Import|PendingUpload|Preimport' -count=1`、`go test ./... -count=1`、`go vet ./...`、`go build ./...`。测试覆盖复合 Phase A、activation/finalize、GameLoop.Saved 持久保存、preimport 恢复、敏感 ID 脱敏和 journal 阶段恢复，但这些不能替代真机游戏语义与二次重启。
- 上游 `.125` 仍没有 import commandId。Panel 当前适配方案继续使用磁盘事务痕迹、pending、SaveNameToLoad、RuntimeSaveID、ProcessIdentity、`saveImportFinalizeCount`、同 commandId 的 Control `GameLoop.Saved` 与 `dayTransitionComplete` 复合证据；日志文本或单一指针均不是成功证据。
- 解除本门禁前必须准备明确非唯一、可销毁的八类测试 ZIP及逐份 SHA256、隔离 Panel 数据库/端口/Compose project/卷、可用测试平台账号与游戏客户端，并逐项留存 Phase A、finalize、持久化、人工语义、故障注入和第二次重启证据。

# SAVE-IMPORT-WEB-API-1：持久导入事务 Web 契约（2026-07-16，completed）

- `POST /api/instances/:id/saves/upload-commit-and-start` 已彻底断开旧 `ImportSaveToVolume -> SetActiveSave -> ApplyModProfile -> Start` 路径，只能创建 `stardew_import_save_and_start` 持久事务。请求必须携带嵌套 `hostHandling`：`swap_to_player` 使用十进制字符串 `platformId`；`virtual_host_takeover` 必须显式 `acknowledged=true`。缺失、未知 mode 或 takeover 未确认统一返回 `host_decision_required`，绝不默认 takeover。
- swap 的平台 ID 在 Web 入口 trim 后逐字符校验 ASCII 十进制，并以 `uint64` 非零范围复核；JSON DTO 始终使用 string，不经过浮点。原值仅传入 driver/FIFO 闭包，journal 只保存 `sha256(operationId + "\x00" + platformId)`，普通日志、审计、HTTP 响应和错误文本均不包含原值。
- commit 使用后端生成的 128-bit operation ID：durable token 从 `available` 原子 reserve；driver 成功创建 journal 后 callback 将 payload 转为 operation/source 所有；job 创建成功后 token 记录 `jobId`。成功固定返回 `202 {jobId, operationId, saveName}`。相同 token 与相同 host 决策重试从持久 token+journal 返回原响应，不创建第二个 job；不同决策返回 `save_import_busy`。
- cancel 仍可直接删除 `available` 预览 token；`reserved` 或活动/成功的 owned 事务继续返回冲突。owned token 只有在精确关联的导入 job 已 `failed/canceled`、driver 在生命周期互斥锁内证明数据库状态属于四态离线集合且 Compose server 实停、journal 仍为 `MaintenanceStarted=false` 且未提交/未确认，并通过 bootstrap ownership、pointer 与 staged fingerprint 门禁时，才调用 `CleanupUnsubmittedImport` 删除 source/journal/本次 staged target/bootstrap/pointer 和 token；preimport 保留。Compose 复验与文件清理不留可被 Start 抢入的窗口，任一身份、运行态或磁盘证据不确定都 fail closed。
- 本接口稳定映射 `host_decision_required / platform_id_invalid / save_exists / junimo_import_unsupported / save_import_busy / import_command_failed / import_result_unconfirmed / import_recovery_required / import_activation_timeout / save_in_progress`。原实例运行或启动中返回 `save_in_progress`；已有未完成导入返回 `save_import_busy`。
- 审计动作仍为 `save_import_submit`，仅含公开 mode、saveName、operationId、jobId。实现位于 `internal/web/save_import_api.go`、`pending_uploads.go`、`lifecycle_handlers.go` 与 driver transaction；本阶段未修改前端、Junimo 上游或 Stardew XML。

# SAVE-IMPORT-DURABLE-SAVE-1：GameLoop.Saved 持久化门禁（2026-07-16，completed）

- swap 在 `finalize_confirmed` 后读取目标主文件稳定 SHA-256/mtime/size，并读取 `/status.version/dayTransitionComplete` baseline；随后预生成 Control commandId、先把 commandId 和 `stage=save_persisting` 持久化，再原子发布一次 `save-now`。Panel 崩溃恢复只观察 journal 中原 commandId，绝不重新提交 save-now 或 `saves import`。
- 只接受同一 commandId 的 `queued -> running -> succeeded`；Control 的 succeeded 仅由同一次 `GameLoop.Saved` 写出，因此它是保存完成的权威事实。failed 返回 `import_recovery_required`；unknown/expired/状态或 ID 矛盾返回 `import_result_unconfirmed`，所有失败均保持维护 runtime 运行，不停止、不重启。
- Saved 后重新读取 `/status` 获得 post-Saved version 游标，再优先用容器内单次长轮询 `/wait/status?since=...&dayTransitionComplete=true&timeout=8000` 等待更新的 true；上游不支持 wait 时才以至少 2 秒间隔轮询 `/status`。字段使用 optional bool，缺失绝不补 true。Saved 已确认但 transition 超时时记录明确 durable warning 并保持 `save_persisting`。
- transition 稳定后对目标主文件再次执行稳定 hash/stat、严格 XML 流式解析（根必须是 `SaveGame`）及保存前后 hash 或 mtime 变化门禁；通过后依次原子写 `save_verified`、`completed`。XML 损坏或文件没有可验证变化进入 recovery，不覆盖、不恢复存档。
- as-is 不含内存 finalizer，不发送 save-now；只复核目标 RuntimeSaveID、无 pending 和稳定世界，然后推进 `save_verified/completed`，不会无意义改写原存档。
- `GameLoop.Saved` 原有 `save-events/*.json` 生成和游戏日自动回档维护链保持不变；durable stage 不消费事件、不清理 preimport，也不重复实现 reconciler。

# SAVE-IMPORT-ACTIVATION-1：目标加载与 Layer B 复合确认（2026-07-16，completed）

- Phase A `import_confirmed` 后，job 先在原进程内等待 `--reload`，只接受 Control `status.json.saveId` 作为 RuntimeSaveID；`SaveNameToLoad` 与日志文本均不替代运行时加载事实。观察有明确期限，并持续记录 pending、ProcessIdentity、diagnostics 与 `/status`。
- swap 只有在 RuntimeSaveID 为目标、pending 已清空、当前进程代际的 `saveImportFinalizeCount` 恰好增加 1、diagnostics 必需字段无 missing/failedFields、`masterName=Server`、`isOnline=true` 且 `dayTransitionComplete=true` 时推进 `finalize_confirmed`。同进程比较 Phase A pre-submit baseline；新进程接受该代首次 count=1，或接受已捕获的新进程 baseline +1，绝不跨代直接比较旧计数。
- as-is 不执行 finalizer，只要求目标 RuntimeSaveID、无 pending、在线且 day transition complete；不要求 finalizeCount 变化。`IsReady` 同样进入证据，但世界稳定使用上游为节日/婚礼场景设计的 `dayTransitionComplete`，避免把正常特殊日误判为不稳定。
- reload 未建立目标世界时，先检查真实玩家；有玩家返回 `save_import_players_connected`，不踢人、不重启。无玩家时先应用目标 Mod profile，写 `stage=save_activating`，再最多执行一次 server 受控重启或从 Phase A 已停机状态 ComposeUp；不会重发 `saves import`，不会改 pointer/XML。
- pending 清空但 count 未增加、错误目标且 pending 已清空进入 `import_recovery_required`；diagnostics API/字段/failedFields 或 status 证据不可用进入 `import_result_unconfirmed`；目标未加载且 pending 持续存在进入 `import_activation_timeout`。journal 新增 activation evidence/outcome/restart/process-baseline 字段，原始 pending UserID 继续由 `json:"-"` 排除。
- 本阶段成功边界现为 `finalize_confirmed`；DURABLE-SAVE-1 随后负责 GameLoop.Saved、磁盘稳定验证和 completed。

# SAVE-IMPORT-PHASE-A-1：正式 FIFO 提交与 Phase A 复合回执（2026-07-16，completed）

- `stardew_import_save_and_start` 现在在 `runtime_ready` 后继续进入 Phase A，并在同一个 `saveImportRunMu` 临界区内完成 pre-submit 校验、一次性 FIFO 写入和证据观察。提交前严格要求 runtime_ready、维护进程仍在、目标与 preimport 存在且 ZIP SHA/内部 saveName/主文件 hash 均匹配、目标指针尚未被改写、无既有 pending、无 farmhand、ProcessIdentity 与 runtime baseline 一致，并重新记录 `preSubmitEvidence/preSubmitLogOffset`。
- 命令仅有两种：`saves import <saveName> --reload` 与 `saves import <saveName> --swap-host-to <platformId> --reload`。saveName 只允许无空白/控制字符的 Unicode 字母数字及 `_.-`，platformId 必须是非零 ASCII uint64 且 salted fingerprint 与 journal 相符。原始 platform ID 只存在于 job 闭包和 FIFO 命令内，不进入 journal、日志详情或错误文本。
- FIFO writer 成功返回后，在读取任何日志或判断结果前立即写 `upstreamSubmitted=true`、`stage=import_submitted` 和 `upstreamSubmittedAt`。命令只写一次；超时、取消、矛盾或 journal 写入异常都不会自动重发。成功日志完全不参与状态推进。
- swap Phase A 仅在“主文件 hash 改变 + pending 存在且 saveName/OwnerUid/fingerprint 全匹配 + SaveNameToLoad 为目标”时写 `upstreamConfirmed=true/stage=import_confirmed`。as-is 仅在“主文件 hash 不变 + 无 pending + 指针从非目标变为目标”时确认。`import_confirmed` 只证明 Layer A，不证明住宅、家具、地窖或家庭迁移，也不写 completed。
- 观察超时或 job cancel 后先 `ComposeDown` 并轮询确认 server 完全退出，再读取最终磁盘证据：无变化为 `import_command_failed` 且允许按 staged fingerprint 安全 cleanup；完整证据为确认；hash 已变且 matching pending 但旧指针为 `import_recovery_required`；hash 已变且无 pending 时从本 operation preimport 隐藏目录解压、校验后以目录 rename 替换目标，并再次验证恢复 hash；其他矛盾为 `import_result_unconfirmed`。所有路径禁止调用旧 `ImportSaveToVolume/SetActiveSave` 或修改 XML。
- journal 新增 `preSubmitEvidence/preSubmitLogOffset/phaseAEvidence/phaseAOutcome/phaseARestoredSha256/phaseALogDetail/upstreamSubmittedAt`。FIFO 失败详情先替换原始 platform ID 再截断持久化；Panel 重启扫描 `import_submitted` 且没有安全 no-effect 证明的事务时仍进入 `manual_required/import_result_unconfirmed`。

# SAVE-IMPORT-MAINTENANCE-RUNTIME-1：导入专用维护运行时（2026-07-16，completed）

- `stardew_import_save_and_start` 在 `.125` 镜像/挂载 DLL 静态校验、真实 staging 和上传目标 preimport 完成后，走独立维护启动路径：`ComposeUp -> server running -> FIFO -> 可读 server-output.log -> /health 或 /status -> 容器内 Junimo manifest=.125 -> 无副作用 saves 列表探测`。FIFO 不再在服务器启动前检查。
- 维护运行时始终持久化为 `state=stopped`、`driverPhase=save_import_maintenance`，不复用普通 Start 的邀请码/ready/newgame 流程。状态 reconcile 不会因 server 容器运行而把它提升为 `running`；邀请码 API 在该 phase 拒绝读取，driver payload 中旧 `invite_code` 暂时隐藏。
- `runtime_ready` 前后均检查 `/status.playerCount`；发现 farmhand 时返回 `save_import_players_connected`，不踢人、不继续。唯一写入 FIFO 的探测是裸 `saves`，本阶段不构造或发送 `saves import`。
- `runtime_ready` journal 新增 `maintenanceStarted`、`serverOutputLogOffset` 和 `runtimeBaseline`。baseline 复用 SAVE-IMPORT-EVIDENCE-1，至少要求目标主文件 SHA-256、finalizeCount 和稳定 ProcessIdentity，并记录 active pointer、pending、runtime saveId、day-transition 等可用证据；启动期间指针或 ProcessIdentity 改变会拒绝推进。
- `runtime_ready` 前的启动、FIFO、日志、API、运行中版本、命令探测、玩家或证据失败，以及 job cancel，都会 `ComposeDown` 本 job 启动的运行时；只有随后 `ComposePs` 证明 server 已停才恢复进入维护前的精确 state/phase/message/payload，并保留 staged target 与 preimport。原状态允许为 `game_installed/save_required/ready_to_start/stopped`，不再要求或永久归一化成 `stopped`；停机不可证时保留 maintenance ownership 并禁止自动 cleanup。该段是 MAINTENANCE-RUNTIME-1 完成时边界；PHASE-A-1 现已从 `runtime_ready` 继续正式提交，但仍不能宣告整个导入成功。

# SAVE-IMPORT-STAGING-1：持久 source、无覆盖暂存与 preimport 备份（2026-07-16，completed）

- 修正导入事务的 `validated -> staged -> backup_created`：journal 成功建立后，commit handler 通过一次性 callback 把 token payload 从 `pending-save-uploads/<token-hash>/payload` 转移到 `.local-container/control/save-import-transactions/<operationId>/source/`，token 进入 `owned`。operation/source 与文件权限收紧为 0700/0600；handler 返回后 source 已由事务持有，Panel 重启可由 journal 扫描和 `sourceAvailable` 恢复发现。
- 新增 `StageImportedSaveNoReplace`。目标固定为 `Saves/<saveName>`，同名存在返回 `save_exists` 且不改任何字节；Linux 使用 `renameat2(RENAME_NOREPLACE)`，Windows 使用不替换的 `MoveFile`。跨文件系统时先复制到 Saves 下 `.import-stage-*` 隐藏目录，校验 required files 与全目录 SHA-256 fingerprint 后再无覆盖原子发布；复制/校验/发布失败只清隐藏目录，正常存档名下不会出现半成品。
- journal 仅在真实目标发布完成后写 `stagedSaveCreated/stagedSaveFingerprint` 并推进 `staged`。旧 `ImportSaveToVolume` 的“先删同名目录再覆盖”行为未复用，保留给既有旧业务，事务路径不调用它。
- 新增 `BackupPreImport` kind：备份对象是刚暂存的上传目标，不是 `OriginalActiveSave`；文件名为 `preimport_<saveName>_<operationId摘要>_<UTC纳秒时间>.zip`，journal 同时记录文件名和 SHA-256，成功后才推进 `backup_created`。preimport 可走既有完整 restore，且不匹配 `auto_` 清理前缀，不参与自动游戏日保留策略。
- 提交前取消与 staging 共用锁：删除 transaction source；仅当目标仍与 journal 的全目录 fingerprint 一致时删除本 operation 创建的目标；preimport ZIP 明确保留作为恢复材料。目标已变化时拒绝自动清理并进入 recovery。`import_submitted` 或其后阶段仍严格拒绝自动清理。
- 本阶段仍只执行静态 `.125`/DLL 只读门禁，不探测 live FIFO、不启动/exec Junimo、不发送 `saves import`、不调用 `/test/*`、不修改 Stardew XML。`OriginalActiveSave` 继续只作 journal 上下文，不再额外备份旧活动存档。
- 主要文件：`save_import_staging.go`、按平台 no-replace rename 文件、`save_import_transaction.go`、`saves.go`、`pending_uploads.go`、commit handler 与对应测试。

# SAVE-IMPORT-EVIDENCE-1：Junimo `.125` 导入只读证据采集（2026-07-16，completed）

- 新增 `save_import_evidence.go`，只读解析 SMAPI global-data key `JunimoHost.SaveImport`。经保留的真实 `.125` 隔离实例 saves volume 以只读 mount 核验，物理路径为 `.local-container/saves/.smapi/mod-data/junimohost.server/junimohost.saveimport.json`；注意实际文件名是 `saveimport`，不是预估的 `save-import`。文件不存在或 `Pending:null` 返回 `Exists=false`，损坏 JSON 返回 `pending_intent_invalid_json`，不会伪装成无 pending。代码不写、删、清空或修复该文件。
- `JunimoSaveImportIntent` 读取 `SaveName/OwnerUid/UserId`；`UserID` 仅在内存中用于 `sha256(operationID + "\x00" + platformID)` 比较，字段标记 `json:"-"`，比较只返回 `match/mismatch/unknown`，错误不包含原始值。saveName 不一致直接为 mismatch。
- 新增容器内 localhost 的 `GET /diagnostics/state` 只读调用，复用 `ComposeExecPipe` + curl、4 秒 context/3 秒 curl 短超时，解析 `saveImportFinalizeCount/masterName/failedFields`。API 不可用、超时、JSON 损坏、目标字段缺失和 `failedFields` 命中分别有稳定错误码；一个字段失败不会抹掉另一个已知字段。该状态只是复合证据，不能单点宣布导入成功。
- 新增 `CaptureJunimoImportEvidence`：对目标主存档与 `SaveGameInfo` 做双读稳定 SHA-256，读取精确 gameloader pointer、pending intent、diagnostics、Control `status.json.saveId`、`/status.dayTransitionComplete`，并以容器 hostname + `/proc/1/stat` start ticks 标识进程代际。任一非 pending 来源无法读取时进入 `unknownFields`，不填伪默认值；pending JSON 损坏则整体明确报错。所有采集均只读。
- 本阶段没有上传/API 接口，没有发送 `saves import`，没有调用 `/test/*`，没有修改 Stardew XML 或存档目录。原 `SAVE-IMPORT-JUNIMO-1` blocked 记录保留为当时“缺正式 commandId/单一终态回执”的历史结论；后续路线改为 Panel 黑盒复合证据适配，缺 commandId 本身不再是 blocked 理由，但复合来源矛盾或不足仍只能判 unknown。
- 测试覆盖 pending 缺失/null/完整/损坏、saveName 不一致、fingerprint 三态、diagnostics 正常/缺字段/failedFields/超时、主文件 hash、读取间变化、进程代际变化及 UserID 序列化/错误脱敏。

# SAVE-IMPORT-JUNIMO-1：正式 Junimo 导入执行（2026-07-16，blocked）

- 本任务在实现前置门禁处停止，未修改 Go 导入 runner。仓库内 SAVE-IMPORT-SPIKE-1 的真实结论仍为 blocked，并非“已提供稳定机器终态”：`.125` 正式 FIFO `saves import` 没有 commandId，也没有可查询的 submitted/succeeded/failed/unknown 结果。
- 再次复核本地上游源码：`SavesCommand.HandleCommand`/reload 路径仍是 `void` + `RequestReloadSave().ContinueWith(...)`；无活动世界、在线玩家拒绝、game manager 未就绪及异步 fault 都只写 Monitor 文本。`SaveImportService.ImportResult` 仍只被进程内代码和 `/test/import_save` test-only endpoint 消费，正式 `ApiService` 没有 `/saves/import` 端点。
- Panel Control 的 `commandResultVersion: 1` 只覆盖 Panel 自有 command 文件协议，不会关联经 Junimo FIFO 提交的 `saves import`，不能借用它伪造上游结果。
- 因此禁止实现“写 FIFO后解析日志/观察 pointer 即判成功”、超时自动重发或失败后回退 `ImportSaveToVolume + SetActiveSave`。现有 SAVE-IMPORT-TXN-1 journal/token/互斥骨架保持不变，仍不会发送真实 import。
- 最小解锁条件：上游正式端点或 FIFO `--command-id` 接受 caller-generated commandId，并为所有退出路径持久返回 schema-versioned `submitted/running/succeeded/failed/unknown`、结构化 `errorCode`、reloadStatus、pointer/pending/finalized/loadedSaveId 与时间戳；崩溃窗口必须落为 unknown 且禁止自动重放。

# SAVE-IMPORT-TXN-1：持久导入事务骨架（2026-07-16，completed）

- 上传预览数据迁入实例私有目录 `.local-container/control/pending-save-uploads/<token-sha256>/payload`；磁盘只使用 token 摘要。`token.json` 原子写入，Linux 权限为 `0600`，父目录为 `0700`。
- token 生命周期原骨架为 `available -> reserved -> consumed`；SAVE-IMPORT-STAGING-1 在 journal 后补充 `owned`，表示 payload 已迁入 operation/source。available 可直接 cancel；owned 的 cancel 走 transaction pre-submit cleanup；本阶段没有成功终态，生产路径不会 consume。
- 新增 job `stardew_import_save_and_start` 与 `.local-container/control/save-import-transactions/<operationId>/journal.json`。阶段固定为 `validated / staged / backup_created / runtime_ready / import_submitted / import_confirmed / save_activating / save_verified / completed`；记录实例、save、原 active save、hostHandling、平台 ID 的 operation-salted SHA-256 fingerprint、preimport backup、上游 submitted/confirmed、最后错误和时间戳。journal/目录为 `0600/0700`，同目录临时文件 fsync 后 rename。
- 同名 save 在 journal 前返回 `409 save_exists`，不删除、覆盖或先备份再覆盖，原目录字节不变。上传 commit 已停止调用旧 `ImportSaveToVolume + ApplyModProfile + SetActiveSave` 链；Go 不修改 Stardew XML。
- 能力门禁现要求精确 `.env IMAGE_VERSION=1.5.0-preview.125`、宿主 `JunimoServer` manifest 与 image tag 匹配、DLL 非空；FIFO 检查已由 MAINTENANCE-RUNTIME-1 移到 ComposeUp/server running 之后。维护路径只发送裸 `saves` 探测，不发送 `saves import`。
- `Prepare` 扫描未完成 journal：`import_submitted` 前为 `safe_to_resume_or_cleanup` 并报告 `sourceAvailable`；submitted 未确认是 `manual_required/import_result_unconfirmed`；confirmed 以后是 `manual_required/import_recovery_required`。提交前按 SAVE-IMPORT-STAGING-1 指纹规则清 source/本次目标并保留 preimport；提交后禁止自动清理、重放或覆盖。
- 未完成 journal 与 start/stop/restart、安装、回档、新建/选择/删除存档、Mod 写入/profile 切换、Junimo/SMAPI 更新互斥，统一 `409 save_import_busy`。另预留 `import_activation_timeout`。
- 本阶段只完成事务、门禁与恢复骨架。下一阶段必须先获得上游 commandId/机器可读终态，不得用日志文本、pointer 或固定 sleep 猜成功，也不得回退旧路径。

# SAVE-IMPORT-SPIKE-1：JunimoServer 125 存档导入与跨日协议实测（2026-07-16，blocked）

## 环境、材料与真实执行

- 目标镜像：`sdvd/server:1.5.0-preview.125`，image ID `sha256:10f438581d741fc146ce710cbe20099475ac68908e99f565cf449f0b8192ccf6`，repo digest 同值，OCI revision `89abe8e6a07b3aaee1c0b4fad080683b948645d9`；游戏 `1.6.15 build 24356`、SMAPI `4.5.2`、Steam sidecar `anxiyizhi/junimo-steam-service-cn:1.5.0-anxi.2`、Panel Control `0.2.0`。上游只读源码位于 `E:\junimo-server-steam-service-cn`，同时按镜像 revision 复核 `importing-saves.md`、`SavesCommand.cs`、`SaveImportService.cs`、`ApiService.cs` 和 `SaveImportTests.cs`。
- 所有实验均使用 `save-import-spike*` 临时 Compose project/volume 和生成的测试存档副本；未接触用户实例。原始卷在实验前分别归档为 `.codex-test/save-import-spike/evidence/original-saves-before-import.tgz`（SHA256 `8bfd50bf7c5f9b8aa94385d5c0c80afcf892c168b49f801e9495033bf1c5109d`）与 `generated-sources-before-import.tgz`（SHA256 `9863d782aa397ff693be9753b2050666529cbbd8aaaa8a3ab5566e5741a64d95`）。实验结束已停止容器，保留卷和归档。
- 实际通过 `/tmp/smapi-input` FIFO 执行了下列命令（平台 ID 在文档和显示日志中一律脱敏）：

```text
saves import Junimo_4479296333465318694 --swap-host-to [已脱敏] --reload
saves import RetrySource_6307649149911135086 --swap-host-to [同一ID]
saves import RetrySource_6307649149911135086 --swap-host-to [不同ID]
saves import RetrySource_6307649149911135086 --swap-host-to [重定向后的ID] --reload
saves import CollisionSource_16278551226863591264 --reload
saves import AsIsSource_18203729823860426272 --reload       # 连续两次
saves import AsIsSource_18203729823860426272 --swap-host-to not-a-number
saves import DefinitelyAbsent_100
saves import MissingMainProbe_18203729823860426272 --swap-host-to 123
saves import MalformedProbe_18203729823860426272 --swap-host-to 123
saves import CollisionSource_16278551226863591264 --swap-host-to [已有 farmhand 的ID]
saves import NoWorldSource_3556993608770656865 --swap-host-to [已脱敏] --reload
saves info __fifo_stage_probe__
```

- 还真实执行了 `GET /status`、`GET /health`、`GET /players`、`GET /farmhands`、`GET /diagnostics/state`、`GET /wait/status?since=...&dayTransitionComplete=...`、`POST /time?value=2600`、`docker restart`，以及主文件 SHA256、gameloader、pending-finalize 和 Control `status.json` 的只读核验。没有编写 Go/XML 修改代码；损坏样本是 55 字节隔离 fixture，导入前后 SHA256 均为 `eb5eb68b345a78d53375fdfadaa4fa48586a7892eadf0d039c86a26ca900b208`。

## 场景观察

| 场景 | 真实观察 | 可否作为机器契约 |
| --- | --- | --- |
| 活动世界，swap + reload | Layer A 写主文件、pending 和 `SaveNameToLoad`；随后进程内 reload，约 1 秒后目标世界在线，pending 清空，诊断为 `masterName=Server`、原 owner 为 customized/bound farmhand、`saveImportFinalizeCount=1` | 最终世界状态可交叉验证；命令本身无回执 |
| 无活动世界，swap + reload | Layer A 成功并留下 pointer + pending；reload 被跳过。真实 `docker restart` 后 `/status` online/ready、Control `status.json.saveId` 命中、pending 为 null、诊断为新 `Server` 主机且原 owner 已绑定 | 必须走受控重启；不能把 `--reload` 当成已加载 |
| 无活动世界，as-is + reload | pointer 改写，reload 跳过；重复两次均重复“成功”。重启后 `masterName=OwnerCollision`、`saveImportFinalizeCount=0` | 明确由虚拟主机接管原 `<player>` 角色；无平台 ID 不会保留原 owner 为可选 farmhand |
| 同 ID 重试，swap 尚未 finalize | 返回内部成功 no-op，pending 不变 | 只有日志展示；无 FIFO 结果对象 |
| 不同 ID 重试，swap 尚未 finalize | pending 与磁盘 owner bind 被重定向；之后用新 ID reload 可 finalize | 有机器状态可读，但无 commandId 关联 |
| 同/不同 ID 重试，已 finalize | 都拒绝；需 `/unlinkPlayer` 或恢复原始副本后重导 | 失败只在 Warn 文本 |
| 同一 as-is 重复导入 | 每次都成功改写同一 pointer；无幂等 operation 标识 | 无法区分首发、重放或迟到命令 |
| 非数字 ID | 拒绝；源主文件、pointer、pending 不变。实现实际要求 invariant `ulong` 十进制 | 失败只在 Warn 文本 |
| ID 已绑定其他 farmhand | 拒绝；真实 fixture 命中现有 farmhand，主文件未进入 swap | 失败只在 Warn 文本 |
| XML 损坏 | 拒绝且字节 SHA256 不变 | 失败只在 Warn 文本；本地可先做只读校验 |
| 缺主存档文件 | 即使目录及 `SaveGameInfo` 存在也拒绝 | 失败只在 Warn 文本；本地可先做只读校验 |
| reload 成功 | 活动世界、无其他玩家时真实完成 in-process reload/finalize | 只能事后观察世界状态 |
| reload 拒绝 | 精确 `.125` 源码与随镜像 revision 的 `SaveImportTests.Import_Reload_RefusesWhenClientConnected` 均规定：有其他玩家且非 force 时保持 pending、玩家不断开、旧 master 不变 | 本机没有 `.env.test`/自动化游戏客户端，未重新执行联机分支；现场结果记为未知，不伪装为实测 |
| reload 跳过 | 无活动世界时真实观察到 pointer/pending 已写，但 reload 不执行 | 只有 Warn 展示文本 |
| reload 超时/异步失败 | `RequestReloadSave().ContinueWith(...)` 没有 timeout、结果文件或 HTTP 状态；调用方观察超时无法区分慢、跳过、拒绝、失败和命令未消费 | 结果未知，禁止自动重发 |

## 导入时序与面板判定边界

| 顺序 | swap 路径事实 | as-is 路径事实 | 面板含义 |
| --- | --- | --- | --- |
| 1 | 目录/同名主文件、非活动世界、ID、XML、ID collision 校验 | 目录/同名主文件、非活动世界、XML 校验 | 可在提交前重复只读校验，但不能替代 Junimo 执行结果 |
| 2 | XML temp-write/reparse/atomic replace，并重建 `SaveGameInfo` | 不改 XML | swap 从这里开始已不可当成普通失败回滚 |
| 3 | 写 `junimohost.saveimport.json` pending | 清除其它旧 pending | swap 的 queued 证据必须含目标 save 与请求身份的安全比较；普通日志不得写完整 ID |
| 4 | 写 `junimohost.gameloader.json.SaveNameToLoad` | 写同一 pointer | pointer 只证明下次启动目标，不证明已加载 |
| 5 | 可选 in-process reload；无世界跳过、有玩家拒绝、异步异常无回执 | 同左 | `--reload` 不是终态协议 |
| 6 | 目标世界加载时 Layer B finalize，最终在 `finally` 清 pending | 目标原 owner 直接作为 headless host 加载 | pending=null 单独不够，必须与目标世界证据组合 |
| 7 | `/status` online/ready、Control `status.json.saveId`、pointer、诊断/farmhand 一致 | `/status` online/ready、Control saveId、pointer 一致，master 为原 owner | 才能确认“目标世界已加载”；swap 还需确认虚拟 `Server` 主机与原 owner bound |

当前可供后续面板原型使用的**保守成功判定**分两层：queued 仅表示 `(SaveNameToLoad==target)`，swap 还需 pending 的 save/owner/request identity 一致；loaded/finalized 必须同时满足目标 pointer、Control `status.json.state=save-loaded && saveId==target`、`/status.isOnline=true`、pending=null，并由受控 Control/上游新增端点证明 `master=Server` 与原 owner customized+bound。`/status` 不返回 saveId/master/bind，因此仅靠上游公开 API 仍不够。

可供面板直接使用的**失败判定**仅限提交前自身可证明的确定性条件（目录/主文件缺失、saveName 不安全、当前活动存档、XML 不可解析、平台 ID 不是 invariant ulong、已知 ID collision）以及未来机器回执明确返回的失败。FIFO 写失败可以判为“未提交”；FIFO 写成功后 pointer 未变、日志 Warn、HTTP 观察超时都不能单独判为命令失败。

以下必须归为**结果未知**：FIFO 写入后没有可关联结果；reload 拒绝/跳过/异步 fault/长时间未完成；pointer 已改但 pending 缺失或反之；Control saveId、pointer、`/status` 相互不一致；pending 被清但无法证明目标世界和虚拟主机；观察窗口届满；进程在 Layer A 原子替换后、intent/pointer 写入前后中断。unknown 不自动重试，不覆盖测试副本，交给恢复/人工检查。

无活动世界的可靠方案固定为：先保留原始 ZIP/目录字节副本 → 将副本放入隔离 save volume → 在 `/health` 可用后写 FIFO 命令 → 观察 pointer（swap 同时观察 pending）→ 执行一次受控容器重启，不依赖 `--reload` → 按 pointer + Control saveId + `/status` + pending + 主机/bind 事实验收。不得固定 sleep，也不得等待任意英文日志片段。

## FIFO 与机器回执结论

- 重启实测：容器启动后约 3.14 秒 FIFO 出现并成功接收命令，命令在约 9.21 秒被 Junimo 处理；`/health` 约 8.80 秒可用，世界约 11.93 秒 online。早写会由 FIFO/tail 缓冲到模组注册完成。正式发送门槛仍应采用 `/health` 成功且 FIFO 存在，而非上述耗时常数。
- FIFO 只是文本输入流。`SavesCommand.HandleCommand` 返回 `void`，命令没有 operation/command ID。`SaveImportService.ImportResult` 只在进程内及 test-only HTTP 使用，没有暴露给正式 HTTP/FIFO。
- `SDVD_EVENT save_import_executed` 与 `save_import_finalized` 是结构化日志事件，但没有 commandId；前者只在 Layer A 成功时出现，后者不能关联某次 FIFO 请求。参数错误、reload 拒绝/跳过、异步 fault 只有 Monitor 展示文本。事件名/字段可用于诊断，不能补成完整终态契约；普通英文/中文 Monitor 行完全不是协议。
- 因而 `SAVE-IMPORT-SPIKE-1` 为 **blocked**。推动上游的最小补充：正式 `POST /saves/import`（或 FIFO 命令携带 `--command-id`）接受 caller-generated commandId，并原子写/返回 schema-versioned JSON：`commandId/saveName/mode/status=queued|running|succeeded|failed|unknown/errorCode/reloadStatus=not_requested|started|succeeded|refused|skipped|failed|timed_out/pointerUpdated/pendingFinalize/finalized/loadedSaveId/timestamps`；所有退出路径恰好一个可关联终态，原始平台 ID 不进入日志或响应。面板在此之前不实现正式导入功能。

## `dayTransitionComplete` 准确语义与实测

- `.125` 实际镜像 `/status` 明确返回该字段：无世界/离线为 false；世界加载稳定后为 true。实现定义是：不处于 `Game1.loadingMode`、不是 intro/day-0，且 `newDaySync` 不存在或已 finished。它表示“跨日同步屏障（保存 + newDay handshake + map load）已结束”，特意忽略跨日后的 festival/wedding；普通日通常与 `isReady` 相同，但不能解释成“最近一次保存成功”的持久标志。
- 真实跨日时间线：初始 `true`（day 1）→ `POST /time?value=2600` → DayEnding/Saving → `/status` version 96 发布 `day=2,isReady=false,dayTransitionComplete=false` → Control `Saved` 文件事件 `2026-07-16T07:35:13.7895909Z` → version 98 发布 `day=2,isReady=true,dayTransitionComplete=true`。因此本次顺序为跨日开始/false → Saved → 跨日结束/true。
- `/wait/status` 返回完整 `/status`，并真实支持 `dayTransitionComplete=true|false` 过滤；还要求 `snapshot.version > since`，server timeout 上限 10 秒。idle 时等待 false 得到 408，跨日时从 baseline version 等 false 得到 200/version 96。
- 保存前本来就是 true 时，绝不能直接 `wait ...dayTransitionComplete=true`：下一张普通 true 快照会立即满足。正确边沿算法是先记录 baseline version，循环有界 long-poll 等 `false`；拿到 false snapshot version 后，再等 `true`。若还要严格确认 `Saved`，以 Panel Control 的 machine-readable save-event/command result 关联本次请求；若 never-false、超时或进程重启则 unknown，不用固定 sleep 推断。

# NEWGAME-TXN-1 官方农场创建事务安全化（2026-07-15）

- `POST /api/instances/:id/saves/custom-new-game` 现在只做管理员鉴权、停服检查、1 MiB body 限制、严格单 JSON/未知字段拒绝，以及 DTO 规范化/官方 FarmType 校验；handler 不再写 `server-settings.json`、`server-init.json` 或 marker，也不再提前推进实例状态。规范化配置作为内部 job payload 持久化，模组 FarmType 明确返回 `409 modded_farm_creation_disabled`。
- lifecycle job 在 `.local-container/control/new-game-transactions/<transactionId>/transaction.json` 建立 schema v1 事务记录。目录/文件在 Linux 上分别强制 0700/0600，原子写入，普通支持包不会遍历该目录。快照包含 settings/init/gameloader/pending marker/mod-profiles 的存在性与原内容、创建前存档目录集合、物理 Mod 启用状态、active save、规范化配置、时间、阶段及 `/newgame` 是否已调用。
- 状态机为 `preparing -> configured -> marker_written -> mods_prepared -> compose_started -> command_called -> observing -> success`；失败终态为 `failed`、`unknown`、`ambiguous` 或 `rollback_failed`。结构化 `new-game-pending` 含 `schemaVersion/transactionId/requestedFarmType/createdAt/expiresAt/state`；旧 Control Mod 仍只按文件存在性工作，任意 SaveLoaded 删除 marker 不会让 Go 侧提前成功。
- settings/init 先写同目录临时文件、重新 JSON 解析、fsync 后 rename；两份都成功后才写 marker。`/newgame` 调用事实必须先持久化，每事务最多 POST 一次；curl 超时/错误不重试，而是继续按创建前后目录集合差观察。
- 成功要求恰好一个新目录、同名主文件存在且大小/mtime 连续稳定、完整 XML 可解析，并且实际 `whichFarm` 与请求的八种官方农场之一相符。gameloader 不再是成功依据；仅在指针与唯一新目录共享唯一数字后缀时修复错误前缀。多个新目录为 `ambiguous`，无目录、损坏 XML或 FarmType 不一致均不能成功。
- 失败会先停服，再恢复 settings/init/gameloader/marker/mod-profiles 与原物理 Mod 状态；验证失败产生的新目录移动到 `.local-container/saves-quarantine/new-game/<transactionId>/`，不永久删除。原始失败保存在 `ErrorMessage`，恢复失败另记 `new_game_rollback_failed` 与 `RollbackError`。`LoadNewGameTransaction` 可在面板重启后只读恢复各阶段记录；不会自动再次 POST。
- 主要错误码：`new_game_payload_missing/invalid`、`new_game_snapshot_failed`、`new_game_config_write_failed`、`new_game_marker_write_failed`、`new_game_compose_start_failed`、`new_game_api_not_ready`、`new_game_command_failed`、`new_game_save_not_found`、`new_game_pointer_without_save`、`new_game_xml_invalid`、`new_game_farm_type_mismatch`、`new_game_outcome_unknown`、`new_game_ambiguous`、`new_game_rollback_failed`。
- 影响文件：`migrations/010_job_payload.sql`、`internal/storage/jobs.go`、`internal/jobs/*`、`internal/games/registry/types.go`、`stardew_junimo/new_game_transaction.go/_test.go`、`lifecycle.go`、`saves.go`、`internal/web/lifecycle_handlers.go` 与 handler 测试。本阶段仍只允许八种官方农场，不开放模组农场创建。

# FARM-NEWGAME-MOD-PREPARE-1 模组农场依赖集合与待创建准备（2026-07-15）

- 在现有 `manifestDependencies` 与 `modRelationshipIndex` 上增加 `NewGameModSelection`，没有维护第二套依赖规则。`ContentPackFor` 强制作为 required，并按大小写不敏感 UniqueID 与 `Dependencies` 去重；required closure 继续复用循环保护、稳定排序、反向关系和 packageKey bundle。optional 只记录、不阻断；缺失 required 保留原始 UniqueID；provider 自身禁用也计入待启用集合。
- 模型包含 `farmTypeId/providerModKey/requiredModKeys/optionalDependencyKeys/enabledModKeys/disabledRequiredModKeys/missingRequiredModKeys/conflictingProviderModKeys/components/warnings/readiness/dependenciesReady`。它每次从当前磁盘重新计算，不要求 saveName，也不写 `mod-profiles.json` 或任何正式存档 profile。
- `GET /api/instances/:id/saves/farm-types` 的 modded 条目新增 `modSelection`，`dependenciesReady` 改为实际布尔结果。readiness 为 `ready/needs_enable/missing_required/conflict`；所有模组农场仍固定 `selectable=false`、`requiresRuntimeValidation=true`。
- 新增管理员 `POST /api/instances/:id/saves/farm-types/prepare`，严格请求体仅 `{farmTypeId}`。后端重新扫描 farm/provider/依赖，不接受文件夹路径或客户端 Mod 列表；仅服务器停止且实例无活动 job 时执行。driver 与 lifecycle/runtime update 共用互斥锁，Mod 文件移动与现有 upload/delete/profile toggle 共用 Mod 锁。
- 准备动作只把 provider、required closure 和明确 packageKey 联动组件从 `mods-disabled` 移到 `mods`；修改前保留内存状态快照，任一步失败按反序恢复，回滚失败会明确进入错误链。成功返回实际 `changedModKeys`，不启动服务器、不调用 `/newgame`、不创建存档。
- 当前真实 SVE 只读结果为 ready：`flashshifter.FrontierFarm`、`FlashShifter.StardewValleyExpandedCP`、`FlashShifter.SVE-FTM`、`FlashShifter.SVECode`、`FlashShifter.FrontierFarmFTM`、`Pathoschild.ContentPatcher`、`Esca.FarmTypeManager`。optional 为 `Cherry.ExpandedPreconditionsUtility`、`MoreFish`、`Rose.craftables`、`Tanpoponoko.SeasonalOutfits`、`spacechase0.GenericModConfigMenu`；未改变真实 Mod 状态。
- 测试覆盖 Frontier 闭包、ContentPackFor 去重/强制 required、SVE 间接依赖、循环、缺 SVE Code、provider/Content Patcher 禁用、optional 非阻断、准备成功、移动失败回滚、活动 lifecycle job、运行中、未知/冲突 FarmType 和请求体注入。

# FARM-CATALOG-API-1 农场目录与受控图标 API（2026-07-15）

- 新增管理员只读 `GET /api/instances/:id/saves/farm-types`。权限与现有新建游戏入口一致；响应始终先返回 8 种官方农场，扫描器整体失败时仍返回 HTTP 200、官方列表和不含宿主路径的 `catalogWarnings`，损坏的单个 Mod 也不会阻断其它条目。
- 模组条目返回展示元数据、provider、启用/冲突/条件/置信度和 warnings；`dependenciesReady` 当前为 `null`，所有模组条目固定 `selectable=false`、`requiresRuntimeValidation=true`。接口仅管理员可见，因此本阶段返回 `providerFolder`；条件会移除扫描器内部来源文件字段。
- 新增管理员只读 `GET /api/instances/:id/saves/farm-types/:farmId/icon`。URL 参数只用于匹配本次扫描得到的唯一、非冲突 Farm ID，绝不拼接路径；读取前重新扫描并从受控 icon token 恢复 provider 根，重新执行相对路径、目录/符号链接 containment、格式、文件头、尺寸与 2 MiB 上限校验。响应设置准确 `Content-Type`、`Content-Length`、`nosniff`、`no-store`，文件删除/变化或歧义统一返回不泄露路径的 404。
- `custom-new-game` 的官方 FarmType allowlist 未改动；自 `NEWGAME-TXN-1` 起模组 FarmType 返回更明确的 `modded_farm_creation_disabled`。影响文件：`internal/web/farm_catalog_handlers.go`、路由/依赖注入、对应 handler 安全测试，以及扫描器的受控图标重读函数。
- 验证：`cd backend; go test ./...`。下一阶段才会做依赖与运行时验证、可选择 provider 和创建能力；当前 API 只服务只读展示。

# FARM-CATALOG-DISPLAY-1 离线农场展示元数据（2026-07-15）

- 在既有 `ScanFarmCatalog` 上增加 `Label/Description/IconFile/IconMediaType/IconWidth/IconHeight`，没有新增另一套扫描器。原入口默认按面板当前中文语言解析；内部 `ScanFarmCatalogWithOptions(..., FarmCatalogOptions{Language})` 为测试和未来调用方提供受控语言选择，仍不接 Web handler。
- 名称优先级固定为：规范化后的当前语言（当前为 `zh-CN`，同时尝试基础语言 `zh`）→ `i18n/default.json` → `manifest.Name` → FarmType ID。只支持 `TooltipStringPath=Strings/UI:<key>`，并从同一 Content Patcher 内容包的 `EditData Target=Strings/UI` 找 key；值必须是精确 `{{i18n:key}}`，不执行其它 token、查询或 Content Patcher 条件。
- i18n 值按第一个下划线拆为标题和说明；没有下划线时整段为标题，后续下划线完整保留。空标题继续下一层 fallback；控制字符会移除，标题限制 80 rune、说明限制 512 rune。语言文件/键/Strings entry 无法解析只产生 warning，农场条目继续返回。
- 图标只从同一包 `Action=Load` 且规范化 `Target` 等于 `IconTexture` 的 patch 获取 `FromFile`。路径必须是 Mod 根内不含 token 的相对路径，复用目录穿越与符号链接逃逸防护；只接受扩展名与真实文件头一致的 PNG/JPEG/WebP，拒绝 SVG，单文件沿用 2 MiB 限制，并检查尺寸不超过 4096×4096、总像素不超过 16 Mi。结果仅保存 `/` 分隔的 provider 相对路径和受控媒体元数据，不保存或返回宿主绝对路径。
- 图标缺失、穿越、符号链接逃逸、伪图片、格式/尺寸/大小不合法时 `IconFile` 保持空并追加 warning，不影响农场列表；未来前端使用固定占位图。`WorldMapTexture` 继续仅保留原元数据，不作为新建页图标，也不渲染 TMX/TBIN 或世界地图。
- 合成测试覆盖中文/default/manifest/ID fallback、token/Strings 缺失、下划线和长度规则，以及 Load 映射、穿越、真实符号链接逃逸、伪 PNG、SVG、缺失/超大图片和失败降级。真实实例只读验证得到 `FrontierFarm`、`Label=边境农场`、`IconFile=Assets/Tilesheets/Icon.png`、PNG `22×20`；没有复制或修改 SVE 文件。
- 本阶段仍无 Web API、前端改动、`custom-new-game` 放宽或模组农场创建。验证：`cd backend; go test ./internal/games/stardew_junimo -run FarmCatalog -count=1`、`go test ./internal/games/stardew_junimo -count=1`、`go test ./...`。

# FARM-CATALOG-OFFLINE-1 模组农场离线目录扫描（2026-07-15）

- 新增 `internal/games/stardew_junimo/farm_catalog.go`：服务器未启动时扫描实例 `.local-container/mods` 与 `.local-container/mods-disabled`，解析每个 Mod 的 `manifest.json`，记录名称、`UniqueID`、版本、`ContentPackFor`、依赖、文件夹和物理启用状态。缺失目录返回非 nil 空列表。
- 只把 Content Patcher 中 `Action=EditData`（大小写不敏感）、规范化 `Target=Data/AdditionalFarms` 的 `Entries` 识别为农场注册。Farm ID 优先取 entry 的 `ID`，其次 `Id`；仅在两者都不存在且 Entries key 是安全简单值时回退 key。命名空间 key（例如 `FlashShifter.FrontierFarm/FrontierFarm`）绝不作为 FarmType，权威结果为 entry 内的 `FrontierFarm`。
- 复用 Mod manifest 已有 JSONC 规范化能力，兼容 UTF-8 BOM、`//`、`/* */` 与尾随逗号。Content Patcher `Include` 仅允许 Mod 根内静态相对本地路径；拒绝绝对路径、`..`、模板路径和符号链接逃逸，检测循环并限制递归深度、单文件 2 MiB 与单 Mod 总读取 16 MiB。Include/patch 的 `When` 作为结构化条件保留，不推断为已满足。
- `FarmCatalogResult` 包含 `Mods/Farms/Conflicts/Warnings` 非 nil 稳定列表；农场条目包含 `ID/EntryKey/MapName/TooltipStringPath/IconTexture/WorldMapTexture`、完整 provider 信息、`Enabled/Confidence/Conditions/ParseWarnings`。同 Mod 完全相同声明确定性去重；不同 Mod 的同 ID 保留全部条目并设置 `Conflict/ConflictSources`，同时返回冲突分组，不静默覆盖。
- 明确不扫描任意 `FarmType`、`FarmTypes`、`When.FarmType`、FTM `File_Conditions.FarmTypes`、地图 patch 条件、i18n 或普通字符串。本阶段没有 Web API、前端入口，不放宽 `custom-new-game` 的 FarmType，也不开放模组农场创建。
- 测试位于 `farm_catalog_test.go`，使用最小合成 fixture，不包含 SVE 原包、地图、图标或文本素材。验证：`cd backend; go test ./internal/games/stardew_junimo -run FarmCatalog -count=1`、`go test ./internal/games/stardew_junimo -count=1`、`go test ./...`。显示名称、i18n 与安全图标解析已由 `FARM-CATALOG-DISPLAY-1` 完成。

# JUNIMO-MOD-RUNTIME-SYNC-1 宿主 Junimo DLL 事务化升级（2026-07-15）

- 根因是 Compose 将宿主 `./.local-container/mods` bind mount 到 `/data/Mods`，镜像内新版 `JunimoServer` 会被宿主旧 DLL 遮蔽。此前更新只改镜像 tag 并重建容器，所以会出现“容器为 `.125`、实际加载 Mod 为 `.121`”。
- `ensureJunimoServerMod` 现在读取 `.env` 的 `IMAGE_VERSION`，仅在宿主 manifest 与目标版本完全一致且 DLL 有效时跳过；版本不一致会从当前 server 镜像提取 Mod，经私有临时目录校验后原子替换。这样已处于错配状态的实例在下一次启动时也可自愈。
- runtime update apply 在改写 `.env` 前从目标镜像提取并校验 Junimo Mod，将旧目录保存到 recovery 后再替换；任一步失败都会回滚旧 Mod、旧配置及旧容器。软链接、空 DLL、错误 `UniqueID` 或版本不一致均拒绝发布。
- 更新验收不再只检查镜像 digest、健康状态和 FIFO 存在；`info` 输出必须包含与目标 tag 完全一致的 `Version:`。runtime manifest 的最低 Panel 版本提升为 `0.3.2`。
- 主要文件：`junimo_mod_runtime.go`、`lifecycle.go`、`runtime_update_apply_runner.go`、`runtime_update_rollback.go` 及对应测试。验证：`go test ./internal/games/stardew_junimo -run "TestEnsureJunimoServerMod|TestRuntimeUpdateApply" -count=1`、`go test ./...`。

# JUNIMO-CONFIG-REPAIR-1 可信旧候选配置修复（2026-07-15）

- `InspectRuntimeStack` 新增 `repairable/repairCode/repairReason`。只有 server/auth 主镜像仍属于当前可信仓库、`IMAGE_VERSION` 与 server 主镜像 tag 一致，且候选项全部属于当前可信仓库或固定旧版官方别名时，才把混合 tag/退役候选判为 `repairable/legacy_candidates`；自定义主镜像、未知候选、非法引用和版本主字段歧义继续拒绝自动处理。
- 新增管理员 `POST /api/instances/:id/junimo-update/repair-config`，请求体仅允许为空或严格 `{}`。接口与 install/lifecycle/Junimo/SMAPI 更新任务互斥，并在 `rollback_failed` 时保持人工恢复锁。
- 修复前完整 `.env` 私有备份到 `.local-container/junimo-update/config-repair/<backupId>/original.env`（目录 0700、文件 0600）；只原子改写 server/auth 候选字段，不改变主镜像、当前 tag、凭据、Compose、容器或数据卷。写后立即重新检查，未得到 `update_available/up_to_date` 时原子恢复原配置。
- server 安装候选生成不再把新版默认 tag 和旧实例候选 tag 合并；仅保留与当前安装目标 tag 相同的旧候选，从源头阻止 `.125 + .121` 混合配置再次出现。
- 修复响应不返回 `.env`、备份路径或凭据，只返回随机 `backupId` 和修复后的版本检查结果；审计事件为 `junimo_runtime_config_repaired`。

# MODBUNDLE-1 Mods 文件夹合包完整导入（2026-07-15）

- `UploadModZip` 不再只识别 ZIP 根目录或单层 Nexus 外壳；现在递归发现任意安全深度下所有包含 `manifest.json` 的 Mod 目录，再扁平化导入 `.local-container/mods` 根目录。分类目录和 `Mods1` 这类总外壳不会被当作 Mod 安装。
- 完整性语义改为严格原子导入：发现的任一 manifest 无效、目录名/`UniqueID` 重复，或 manifest Mod 目录互相包含无法安全扁平化时，整个 ZIP 失败且不留部分目录。
- 兼容 Windows 用户自己重新压缩的 Mods 目录：无 UTF-8 标志且名称为 GBK/GB18030 的 ZIP 条目会在校验前解码；Mod 根目录的 `Manifest.json` / `Content.json` 会规范为 Linux 可用的小写文件名。社区 Mod 的数字 `UpdateKeys` 兼容读取为字符串，不会被误认为 `Nexus:<id>`。
- `POST /api/instances/:id/mods/upload` 成功响应新增可选 `upload` 摘要：`archiveCount/discoveredCount/importedCount/enabledCount/activeSaveName`。成功时 `discoveredCount == importedCount`；当前存档启用写入失败则回滚本批目录并返回 `mod_enable_failed`。
- 同包归属不再等同于 Nexus ID。`ModInfo` 与 `nexus-mods.json` 新增 `packageKey/packageName`：`Mods`/`Mods1` 聚合根按第一层子目录划分真实安装包，普通单外壳 Nexus ZIP 仍将其多个组件视为一包。删除、依赖联动和列表 bundle 优先使用 `packageKey`，旧 sidecar 没有该字段时继续按 `originNexusModId/nexusModId`，不会丢失既有同包删除逻辑。
- Nexus 来源推断按每个真实子包分别执行；只有同一子包内无 Nexus ID 的组件才跟随该包主组件。无可用正数 Nexus ID 的 SVE 等包只记录 package，不伪造 Nexus 来源、缩略图或统计。
- 实包验证：`Mods1.zip` 含 3095 个条目、38 个 manifest；隔离目录与本机实例均完整导入 38/38。实例当前存档 `1111_442923526` 的 38 个新 Mod 均标记启用，旧规则会漏掉的 SVE、Frontier Farm、YetAnotherFishingMod 和 MarketTown 组件均已在 Mods 根目录找到。

# JUNIMO-ROLLBACK-TAG-RESTORE-1 回滚后版本检测修复（2026-07-14）

- Junimo 成对升级回滚仍先把 `SERVER_IMAGE` / `STEAM_SERVICE_IMAGE` 临时固定为升级前实际运行的 image ID，确保 tag 漂移时也只重建原 digest。
- 回滚函数现在在所有退出路径再次原子恢复 recovery 中的 `original.env`；成功回滚与 `rollback_failed` 都不会把裸 `sha256:` 固化进实例配置。
- 若最终配置恢复失败，状态使用 `rollback_restore_final_env_failed`，保留 recovery 并继续进入人工处理，不会伪报安全回滚。
- 回归测试覆盖成功回滚、各类失败及 `rollback_failed`，并确认恢复后的可信 tag 配置重新返回 `.121 → .125 update_available`。

# RUNTIME-MATRIX-MIRRORS-1 升级候选与安装顺序统一（2026-07-14）

- `runtime_stack_manifest.json` 的 server 与 steam-auth-cn `images` 已扩展为安装流程使用的完整候选列表，顺序分别与 `DefaultServerImageCandidates`、`DefaultSteamServiceImageCandidates` 完全一致。运行组件预检按该顺序 inspect/pull，失败自动继续下一候选。
- 每个镜像别名都必须绑定同一 canonical OCI index digest；Go 清单校验新增候选去重与同 digest 强制约束，禁止通过为代理记录不同 digest 放宽内容身份。
- Go 回归测试强制矩阵候选和安装默认候选逐项、逐序一致，并验证所有 alias digest 相同，防止两套流程再次漂移。
- 发布侧 `compatibility_matrix.py` 同步强制单一 canonical digest。Docker Hub canonical 及自有 ACR/GHCR 必须可访问且匹配；第三方代理可访问时必须匹配，临时不可达仅告警，因为运行时会安全回退。
- API、请求体、升级状态文件、停服/备份/回滚流程均未改变；用户 `.env` 中的任意自定义地址不会进入可信升级候选。

# PANEL-0.2.2 / JUNIMO-125 推荐矩阵（2026-07-14）

- Panel `0.2.2` 的内嵌推荐运行矩阵更新为 JunimoServer `1.5.0-preview.125` + steam-auth-cn `1.5.0-anxi.2`；server Docker Hub 索引 digest 固定为 `sha256:10f438581d741fc146ce710cbe20099475ac68908e99f565cf449f0b8192ccf6`，`minimumPanelVersion` 为 `0.2.2`。
- 这是推荐版本更新，不是强制升级：现有 `.121` 实例继续识别为受支持的 `update_available`，可照常使用；只有管理员主动完成 dry-run 和确认后才会停服、写回版本对并升级。新安装默认使用 `.125`。
- 上游 steam-service 与 `server-settings.json` 在 `.121`→`.125` 间契约未变化，现有 CN auth 镜像继续使用原 `upstreamRef/sourceRevision` 并已验证配对兼容；不得伪造其源码来源为 `.125`。
- `.125` 实镜像仍含 `/etc/cont-env.d`、`cont-groups.d`、`cont-users.d` 裸静态值；现有 23 个 init 兼容脚本和 bind mount 必须保留。兼容脚本已挂入 `.125` 实镜像验证能执行并输出预期值。
- 影响文件：`config/runtime_stack_manifest.json`、`config/env.go`、`driver.go` 及矩阵/安装/升级/API 测试。验证包括远程制品校验、相关 Go 测试和发布全量门禁。

# JUNIMO-STACK-UPDATE-1 阶段二：运行组件成对升级预检（2026-07-13）

- 新增管理员 `POST/GET /api/instances/:id/junimo-update/dry-run`。POST 请求体只能为空或严格 `{}`，目标 server + steam-auth-cn 只能取自 embed 清单，不接受镜像、tag、digest、registry、service 或命令。
- `stardew_junimo_update_dry_run` job 与 install、lifecycle、预留 apply 和另一 dry-run 双向互斥。预检验证实例/清单、Compose 文件/project/services、Docker/Compose、当前两镜像 digest、运行态、steam-session 卷、可信目标候选和受控 `compose config --quiet`；精确磁盘空间不可可靠获得时明确 warning。
- 详细事实原子写入 `.local-container/junimo-update/dry-run-status.json`（0600），jobs 只负责生命周期/互斥；Panel 重启后仍可恢复最近结果。Docker 扩展面只有受限 inspect/pull/config/ps，原始 stderr、Compose 展开环境和 pull 输出不写状态。
- 阶段二不会写 `.env`/Compose、创建认证备份、读取 token、修改数据卷或执行 `up/down/restart/rm/stop`/volume 删除。阶段三 apply 尚未实现。

# JUNIMO-STACK-UPDATE-1 阶段一：运行组件版本对只读检测（2026-07-13）

- 在 `stardew_junimo/config` 内新增 `go:embed` JSON 清单，把 Junimo server `1.5.0-preview.121` 与 steam-auth-cn `1.5.0-anxi.2` 建模为不可拆分的推荐版本对；清单含 `schemaVersion`、`stackVersion`、`minimumPanelVersion`、可信候选镜像、`releaseNotes`、`tested`，推荐值只随 Panel 构建发布，不查询远程 latest。
- 清单校验拒绝空 tag、`latest`、digest 充当 tag、非白名单仓库、缺 server/auth 或未测试清单。实例检测只读取 `.env` 的 `IMAGE_VERSION`、`SERVER_IMAGE(_CANDIDATES)`、`STEAM_SERVICE_IMAGE(_CANDIDATES)`，按可信仓库内 server/auth 精确 tag 对判断 `up_to_date`、`update_available`、`not_installed`、`custom_images`、`invalid_config`，不对 preview/anxi tag 做 semver 排序。
- 新增管理员只读 `GET /api/instances/:id/junimo-update`。响应提供 `available/supported/status/code/reason`、当前与推荐版本对、版本说明、`serverRunning`、`steamAuthLoggedIn`；只选取版本字段，不序列化 Steam 密码、refresh token 或完整环境。
- `/state.runtimeDiagnostic` 改用同一检测器，分别给出 server 与 steam-auth-cn 当前/期望 tag、整体匹配和 unsupported 原因，删除原 `strings.Contains` 单 server 判断；普通用户只看到必要 tag/状态，不看到镜像仓库。
- 阶段一没有 pull、`.env` 写入、容器 stop/recreate、dry-run 或 apply 路径，也不接入 `/api/system/update`。

# PANEL-UPDATE-RELEASE-1 发布闭环（2026-07-13）

- 完成从 `0.1.13` 到 `0.1.14` 的隔离 Compose 真 Docker 成功升级，以及故意 unhealthy 镜像触发的自动回滚。成功终态为 `succeeded`；失败终态为 `failed_rolled_back`，升级中写入的数据库变更已由 SQLite 一致性备份恢复。
- 修复 helper 将部署目录固定挂载到 `/deployment` 的发布阻塞问题：helper 现在把宿主安装目录按原绝对路径挂载并使用原 Compose 路径，避免新 panel 的 Compose labels 被写成 helper 私有路径，保证后续 Web 升级仍能识别部署。
- 成功与回滚测试均确认 `--no-deps panel` 的隔离边界：Stardew、server、steam-auth 哨兵容器 ID 与启动时间未变化。备份目录为 0700，数据库、Compose、`.env`、manifest 均为 0600；状态与日志未写入密钥、完整环境或 registry 凭据。
- 支持标准 `run.sh` 单 Compose、`service=panel`、Docker Socket 和可验证数据挂载部署。缺 Socket、缺 Compose labels 且无完整显式兜底、自定义 `docker run`、多 Compose 文件或自定义编排均返回 unsupported，不猜路径。
- 发布前全量验证：`go build ./...`、`go vet ./...`、`go test ./...`、Docker integration apply、镜像构建、fresh-volume smoke、成功/回滚 Web E2E 均通过。

# PANEL-UPDATE-APPLY-1 面板升级执行链路

- 新增管理员 `POST /api/system/update/apply` 与 `GET /api/system/update/apply`。POST 不接受目标镜像或目标版本请求体，只使用后端版本检测服务当前确认的最新正式 Release；dev、相同版本、降级、unsupported 和并发任务均拒绝。
- 主进程先用 SQLite `VACUUM INTO` 创建在线一致性备份并执行 `integrity_check`，再保存原镜像 ID/digest，随后 detached 启动独立 `panel-updater apply` helper。数据库备份失败或 helper 启动失败时不会修改部署。
- helper 仅从硬编码 Docker Hub、ACR、GHCR 可信候选拉取精确版本 tag，备份 Compose、`.env` 和升级清单，只改 `PANEL_IMAGE`，并固定执行 `docker compose --project-name <self-project> ... up -d --pull always --force-recreate --no-deps panel`。不接受 shell、任意镜像或任意 service 参数。
- 新容器必须同时满足 Docker health=healthy、容器内请求 `/health` 返回 ok、`/api/version` 等于目标版本，默认超时 120 秒。拉取、Compose、退出/unhealthy/超时/版本不匹配均进入自动回滚；回滚停止失败 panel、恢复数据库/Compose/`.env` 和原镜像 ID，重建并验收旧 panel。
- `<PANEL_DATA_DIR>/updater/apply-status.json` 原子持久化 `checking/backing_up/pulling/recreating/waiting_health/succeeded/rolling_back/failed_rolled_back/rollback_failed`、公开进度和脱敏摘要。私有备份位于 `updater/backups/<updateId>`，目录 0700、文件 0600；支持包采用白名单采集，不包含该目录。

# PANEL-UPDATER-DRYRUN-1 独立 Updater 与部署环境识别

- 新增 `internal/updater` 与独立 `cmd/panel-updater`。面板优先用 `HOSTNAME` 作为 container ID 调用 Docker inspect，只在 Docker/Compose 可用、Docker Socket 和数据挂载可验证、Compose project/service/config labels 完整时报告 supported。
- 标准 Compose 要求 `com.docker.compose.project`、`project.config_files`、`service=panel`；单一 compose 文件的父目录作为安装目录。缺 labels 时仅接受 `PANEL_HOST_INSTALL_DIR/PANEL_HOST_COMPOSE_FILE/PANEL_HOST_DATA_DIR/PANEL_COMPOSE_PROJECT` 全部存在且与 inspect 数据挂载一致的显式兜底。
- 新增管理员接口：`GET /api/system/update/capability`、`GET|POST /api/system/update/dry-run`。普通用户不能读取宿主机路径或演练日志。
- POST 只接收精确稳定 `targetVersion`，目标镜像由代码从 Docker Hub、ACR、GHCR、1ms/DaoCloud 项目候选生成；拒绝 latest、digest、任意仓库和 shell 参数。
- 面板通过 `docker run --detach --rm --entrypoint /app/panel-updater` 启动独立 helper。helper 只挂载 Docker Socket、只读部署目录和面板数据目录，只执行 `docker image inspect`、`docker pull`、`docker compose ... config --quiet`。
- 状态原子写入 `<PANEL_DATA_DIR>/updater/status.json`，日志只记录固定阶段、可信镜像引用和脱敏错误，不保存 registry 凭据、PANEL_SECRET、完整环境变量或 compose config 输出。
- 本阶段没有 stop/rm/up/down/restart 当前面板的路径，也没有 apply/upgrade 接口。

# PANEL-UPDATE-CHECK-1 面板版本检测

- 新增 `internal/updatecheck`：当前版本读取构建注入的 `version/commit/buildDate`，出站请求复用 `netdns.NewClient`，从 GitHub Releases 列表中忽略 draft/prerelease，并按语义版本比较 `v0.1.14` 与 `0.1.14`。
- 服务启动后立即检查一次，之后默认每 6 小时检查并加入正负 36 分钟随机抖动。成功结果保存在进程内；临时失败只更新 `checkStatus/checkError`，继续保留上次成功的版本、发布时间、链接和 `checkedAt`，避免把失败解释为“已是最新”。
- `dev`、空值和非法当前版本返回 `unavailable`，不发起远程请求，也不会报告可更新。
- 新增 `GET /api/system/update`（登录用户）与 `POST /api/system/update/check`（管理员）；本阶段没有 apply/upgrade 接口。
- 主要文件：`internal/updatecheck/service.go`、`internal/web/update_handlers.go`、`cmd/panel/main.go`。验证：`go test ./...`、`go build ./...`。

# NEXUS-ARCHIVE-RESUME-1 Nexus ZIP 下载断点续传与无进度超时

- `mod_remote_install` / `mod_nexus_install` 的远程 ZIP 下载现在使用 `.part` 临时文件；同一个任务内如果连接中断，会用 `Range: bytes=<已下载字节>-` 续传。服务器返回 `206` 且 `Content-Range` 起点匹配时追加写入；如果服务器忽略 Range 返回 `200`，会丢弃本次 `.part` 并从头下载，避免拼出坏包。
- ZIP body 下载窗口从 15 分钟提升到 **20 分钟**，并增加 **120 秒无新字节超时**：连接建立后只要 120 秒内没有收到任何新数据，就取消当前尝试并按可重试错误处理。单个 ZIP 最多重试 4 次，但必须仍在 20 分钟下载窗口内。
- 远程/Nexus Mod 安装 job 的整体超时保持并确认拉长为 **30 分钟**，给免费 Nexus 慢速下载留下足够空间；下载完成后的 ZIP 校验、导入、重复 Mod 幂等跳过仍走原逻辑。
- 保留现有安全边界：最大 ZIP 体积仍限制为 200MB；`text/html` 响应仍立即判定为“拿到网页而不是 ZIP”；4xx（例如过期 CDN 临时链接的 403/410）不做盲重试，后续如需可在拿链接层补刷新机制。
- 验证：`cd backend; go test ./internal/games/stardew_junimo -run "NexusDownloadArchive"`；完整相关包：`cd backend; go test ./internal/games/stardew_junimo ./internal/web`。

# NEXUS-DNS-FALLBACK-1 出站请求 DNS 自愈

- 背景：飞牛 NAS 现场 Nexus 搜索一直失败，容器日志根因为 `dial tcp: lookup api.nexusmods.com on 127.0.0.11:53: server misbehaving`——Docker 内嵌 DNS 转发给不稳定的消费级路由器 DNS，间歇 SERVFAIL。同一根因也让 Docker Hub 版本检查大量失败。用户是普通玩家，不能指望其修 DNS，因此在代码层自愈。
- 新增包 `backend/internal/netdns`：`netdns.NewClient(timeout)` 返回带 DNS 兜底传输层的 `http.Client`。解析顺序为系统解析器优先（健康环境零改变），失败后按序直接查公共 DNS `223.5.5.5 / 119.29.29.29 / 1.1.1.1 / 8.8.8.8`，SERVFAIL/空答复触发下一台，命中即停；解析结果 IPv4 优先。
- 三处出站客户端统一改用它：`nexusHTTPClient`、`nexusArchiveHTTPClient`（`nexus.go`）、Docker Hub 版本检查 `dockerHubHTTPClient`（`install_handlers.go`，替换原 `http.DefaultClient`）、公网 IP 探测（`public_ip.go`）。后续新增对外调用一律用 `netdns.NewClient`，不要裸用 `http.DefaultClient`。
- 发布 compose 模板**不**默认写死 `dns:`，避免约束内网/海外用户；真机已临时在 `panel` 服务加 `dns:` 应急，代码修复上线后可保留可撤。
- 验证：`cd backend; go test ./internal/netdns ./internal/web ./internal/games/stardew_junimo`。详见 `docs/backend-handoff/backend-handoff-2026-07-07.md`。

# JUNIMO-STATIC-INIT-FIX-1 JunimoServer 静态 init 文件兼容挂载

- 背景：真实飞牛部署中，`sdvd/server:1.5.0-preview.121` 的多处 `/etc/cont-*.d` 静态值文件会被容器 init 当 shell 脚本执行。先后表现为 `APP_NAME: DockerApp: not found`、`DBUS_SESSION_BUS_ADDRESS: unix:path=/tmp/dbus.base: not found`、`DOCKER_IMAGE_PLATFORM: linux/amd64: not found`、`/etc/cont-groups.d/cinit/id: 72: not found` 等，导致 `stardew-server-1` 以 127/1 退出。
- `EnsureServerContEnvFix(dataDir)` 保持原入口，但已扩展为写入一组本地兼容脚本：`.local-container/cont-env/*`、`.local-container/cont-groups/*`、`.local-container/cont-users/*`。脚本只输出原静态值，并通过 bind mount 覆盖镜像内对应 `/etc/cont-env.d/*`、`/etc/cont-groups.d/*`、`/etc/cont-users.d/*` 文件。
- 新实例 compose 模板已内置上述挂载；旧实例由同一函数幂等迁移 `docker-compose.yml`。`Driver.Prepare()`、安装流程、`doStart()` 和 `doRestart()` 继续沿用既有调用点；重启时如本次新增挂载，会用 `docker compose up` 重建 server 容器。
- 影响文件：`compose_template.go`、`server_env_fix.go`、`driver.go`、`installer.go`、`lifecycle.go` 及对应测试。
- 验证：`cd backend; go test ./internal/games/stardew_junimo`。真机热修后 `stardew-server-1` 为 healthy，`http://127.0.0.1:8080/health` 返回 `status=ok`。

# NEXUS-NETWORK-DIAGNOSTICS-1 Nexus 搜索防短断与错误诊断

- `handleModNexusSearch()` 调用 Nexus GraphQL 时不再直接使用浏览器请求的 `r.Context()`，改为 `context.WithTimeout(context.WithoutCancel(r.Context()), 20*time.Second)`，避免前端刷新、切页、FRP/NAS 链路短断把上游 Nexus 搜索提前取消并报 `nexus request failed`。
- `doNexusRequest()` 的 HTTP client / DNS / TLS / timeout 类失败现在包装为 `NexusRequestError`。Web 层返回 `502 nexus_network_failed`，同时在后端日志记录底层错误，便于区分 `context canceled`、超时、DNS、TLS 或 Cloudflare 抖动。
- 现场 SSH 诊断确认：部署 NAS 宿主机与 `anxi-panel` 容器内均可 POST 到 `https://api.nexusmods.com/v2/graphql`，完整 Stardew `tractor` 搜索返回 200；旧日志中的 `nexus request failed` 不能单独证明 Nexus 当前不可达。
- 验证：`cd backend; go test ./internal/games/stardew_junimo ./internal/web`。

# STEAM-AUTH-FLAG-1 steam-auth 授权标志

- `steamAuthLoggedIn` 是前端邀请码授权按钮的主判定，来自实例 `.env` 的 `STEAM_AUTH_COMPLETED=true`。该标志由两类强证据写入：真实 steam-auth 登录成功日志（`[SteamAuth:*] Logged in as ...` / `[SteamService] ... Logged in as ...`），或启动/刷新时成功获取到非空邀请码。
- SteamCMD 兜底登录、游戏/SDK 下载成功、`Downloading app`、`download_failed` 等不再写 `STEAM_AUTH_COMPLETED`；SteamCMD 只维护独立的 `STEAMCMD_AUTH_COMPLETED`。
- 启动/重启服务器后，如果 server 容器日志明确出现 `Steam-auth service has no logged-in accounts` / `no logged-in accounts`，后端会把 `STEAM_AUTH_COMPLETED` 清空。前端随后重新显示【登录授权】。
- `Steam-auth service not ready` / `Steam auth service request failed` 不直接清空授权标志；如果已有 `STEAM_AUTH_COMPLETED=true`，后端会 best-effort 自动刷新 `steam-auth` 服务，避免用户手动进服务器重启容器。
- `steamAuthReady` 仍由 `/state` 返回，但只作为诊断运行态字段：后端通过 `ComposeExecPipe` 在 `server` 服务内探测 `steam-auth:3001/steam/ready`，失败不直接驱动主 UI 授权按钮。

# INVITE-CODE-DECOUPLE-AUTHSTATUS-1 启动不卡邀请码 + auth 登录状态

- `lifecycle.go`：`doStart`/`doRestart` 只负责把 server 容器拉到运行态，启动/重启 job 不再等待邀请码，也不因邀请码获取失败而失败。发起启动前会清理 `.local-container/control/status.json` 与 `players.json`，避免旧 SMAPI/玩家快照导致前端误判“已启动”后又闪回。
- 启动/重启完成后后端会调用 `startInviteCodePolling()` 在后台最多尝试 20 次获取邀请码；成功拿到非空码时写入 driver payload、置 `STEAM_AUTH_COMPLETED=true`，`/state.inviteCode` 会带回给前端；失败只记录日志，不关闭服务器、不影响 IP 直连。
- `config.SteamAuthLoggedIn(dataDir)`（**`STEAM_AUTH_COMPLETED=="true"`**，由 steam-auth 登录成功日志或非空邀请码写入；不是看 `STEAM_REFRESH_TOKEN`）经 `instanceStateResponse.steamAuthLoggedIn` 暴露。前端 `InviteCodeCard` 未认证时提示「需登录 Steam 授权」+【登录授权】按钮跳转安装页；生命周期按钮按实例状态与 active lifecycle job 判定，不再依赖邀请码、SMAPI 存档加载日志或在线玩家。

# IP-DIRECT-CONNECT-DEFAULT-ON-1 默认开启 IP 直连

- `saves.go WriteServerSettings()` 的 `Server` 段新增 `"AllowIpConnections": true`；JunimoServer 默认关闭 IP 直连（日志 `IP connections disabled (default)`），邀请码走 Steam SDR/Galaxy P2P 可能单独失败（卡 `Invite Code: n/a`），故默认开 IP 直连作为可靠 join 通道。
- 新增 `EnsureServerSettingsDefaults(dataDir)`，`lifecycle.go doStart()` 在 `ComposeUp` 前调用：幂等确保 `server-settings.json` 的 `Server.AllowIpConnections`（缺失补 true、显式值尊重），让已有存档重启也生效。
- 真机需在安全组放行 UDP 24642 才能真正 IP 直连。详见接手文档 `IP-DIRECT-CONNECT-DEFAULT-ON-1`。

# NEWGAME-TIMEOUT-WRONG-SAVE-1 新建存档超时回退旧存档修复

- `lifecycle.go` `sendNewGameCommand()`：`POST /newgame` 是同步阻塞（要生成完整世界才返回），超时由 30s 提到 **4 分钟**；超时/出错不再直接判失败，改为 Warn 后继续走「等新存档落盘」轮询。
- 发请求前记录 `gameloader.json` 的现有 `SaveNameToLoad` 为 `prevSave`，轮询要求新存档名 `!= prevSave`，避免把持久化 `saves/` 里残留的旧存档误当成新建存档（删 `game-data` 卷不删存档，旧存档会残留）。
- 详见 `docs/backend-handoff/backend-handoff-2026-07-06.md` 的 `NEWGAME-TIMEOUT-WRONG-SAVE-1`。

# STEAMCMD-SPLIT-SDK-1 SteamCMD 游戏与 SDK 分段下载

- 修复云服上 SteamCMD 在 `Success! App '413150' fully installed.` 后继续同一会话切换 `+force_install_dir /data/game/.steam-sdk +app_update 1007` 时出现 `Please use force_install_dir before logon!` 并段错误退出 `139` 的问题。
- `buildSteamCMDOpts()` 仍使用同一个一次性 SteamCMD 容器和同一套缓存卷，但容器内拆成两次独立 SteamCMD 进程：第一次 `+force_install_dir /data/game +login ... +app_update 413150 validate +quit`，第二次 `+force_install_dir /data/game/.steam-sdk +login ... +app_update 1007 validate +quit`。
- 旧实例 `.env` 中残留的 `docker.m.daocloud.io/steamcmd/steamcmd:latest` 会被过滤，不再因为本地已有旧 daocloud 镜像而优先使用它。若 SteamCMD 仍以 `139` 段错误退出，后端只移除占用旧卷的一次性容器并自动重试，**不再删除任何 SteamCMD 授权卷**（游戏文件和已批准设备身份均保留）。
- **登录模型（当前实现，见 `STEAMCMD-CREDENTIAL-REUSE-1`）**：`buildSteamCMDOpts()` 里
  - **游戏段（413150）首次为完整登录** `+login "$STEAM_USERNAME" "$STEAM_PASSWORD"`；登录成功写入 `STEAMCMD_AUTH_COMPLETED=true`。后续修复/重装使用 SteamCMD 自己的缓存登录 `+@NoPromptForPassword 1 ... +login "$STEAM_USERNAME"`，不再提交密码、不主动创建新的 Steam Guard 挑战。
  - 缓存登录输出 `Cached credentials not found` / `No cached credentials` 等明确失效信号时，当前 job 会清空 `STEAMCMD_AUTH_COMPLETED` 并自动回退一次完整登录；`@NoPromptForPassword 1` 防止缓存缺失时卡死在交互式密码提示。
  - **SDK 段（1007）恒为匿名登录** `+login anonymous`。Steamworks SDK Redist 是公开可匿名下载的，不需要账号，也不触发 Steam Guard。参考 `E:\源码\StardewValleyServerKit` 的 `install_steamworks_sdk()` 同样用匿名。
  - 结果：正常情况下只在首次游戏段批准一次 Steam Guard；SDK 段永不需要批准。见 driver_test 的 `TestBuildSteamCMDOptsGameFullLoginSDKAnonymous`、`TestBuildSteamCMDOptsCachedLoginUsesOneCrossImageAuthorizationVolume`。
- **SteamCMD 与 steam-auth 严格独立**：这里没有复用 `steam-auth` 的 refresh token。SteamCMD 使用自己的 `config.vdf` / ConnectCache / machine authorization 文件；`STEAMCMD_AUTH_COMPLETED` 也不影响 `STEAM_AUTH_COMPLETED`。
- **统一登录卷**：候选镜像的运行用户和目录不同：官方 `steamcmd/steamcmd` 以 root、`HOME=/root` 运行，CM2 镜像以 `steam`、`HOME=/home/steam` 运行。`steamcmd-login` 现在同时挂到 `/root/Steam`、`/root/.local/share/Steam`、`/home/steam/Steam`、`/home/steam/.local/share/Steam`，让一次 SteamCMD 批准在候选镜像切换后仍可见。旧 `steamcmd-user-local` / `steamcmd-root-local` 卷不再作为运行目录挂载，由下一条迁移逻辑择一导入有效旧会话。
- **旧卷自动迁移**：运行 SteamCMD 前先启动一个短生命周期迁移容器；若统一卷尚无 `config/config.vdf`，按 root-local、user-local 顺序查找旧缓存，仅复制 SteamCMD `config/` 与卷根 `ssfn*` 到统一卷。旧卷只读挂载、不删除，已存在的统一缓存不覆盖，文件内容不进入日志。
- **真实 Docker 验证（2026-07-13）**：现场旧缓存位于 `stardew_steamcmd-root-local/config/config.vdf`，`steamcmd-login` 为空；迁移后使用统一卷连续启动两个全新 SteamCMD 容器，两次均只执行 `+@NoPromptForPassword 1 +login <username> +quit`，均输出 `Logging in using cached credentials`、完成 user info、退出码 0，未提交密码、未触发 Steam Guard。
- 影响文件：`backend/internal/games/stardew_junimo/installer.go`、`backend/internal/games/stardew_junimo/driver_test.go`。
- 验证：`cd backend; go test ./internal/games/stardew_junimo ./internal/web`。

# INSTALL-SMAPI-PREINSTALL-1 SDK 后置 SMAPI 预安装

- Stardew 安装流程在游戏文件和 Steam SDK 均完成后，新增最后一步 `smapi_installing`：后端使用 JunimoServer 镜像启动一次性 `docker run --rm` 容器，挂载同一个 `<project>_game-data:/data/game` volume，在 `/data/game` 内安装 SMAPI 运行环境。
- 该容器不是常驻服务，不新增需要用户维护的 compose service；它只用于稳定访问 Docker named volume 和复用 JunimoServer Linux 环境。若 `/data/game/StardewModdingAPI` 已存在且可执行，会直接跳过。
- `.env` 保留兼容字段 `SMAPI_VERSION=4.5.2` 与 `SMAPI_DOWNLOAD_URLS`，默认文本与 embed 清单的三个固定加速候选及官方兜底一致，但运行时仍只采用 embed 清单；用户修改 `.env` 不会改变 installer 选择。每个候选都执行 Range、固定大小、SHA-256 和 ZIP 安全结构校验。
- SMAPI 安装失败时实例 phase 为 `smapi_install_failed`，任务失败但 Steam/SteamCMD 授权仍视为已通过；用户可复用保存凭据重试后续安装步骤。
- 影响文件：`backend/internal/games/stardew_junimo/config/env.go`、`backend/internal/games/stardew_junimo/driver.go`、`backend/internal/games/stardew_junimo/installer.go`、`backend/internal/games/stardew_junimo/driver_test.go`。
- 验证：`cd backend; go test ./internal/games/stardew_junimo`。

# STEAMCMD-EMAIL-GUARD-PROMPT-1 SteamCMD 邮箱验证码分行提示识别

- 修复 SteamCMD 首次在新机器登录时输出邮箱 Steam Guard 提示但后端没有切到 `steamcmd_guard_required` 的问题。SteamCMD 原生日志会把提示拆成多行：`This computer has not been authenticated...`、`Please check your email... enter the Steam Guard`、`code from that message.`、`set_steam_guard_code`，旧 matcher 只识别 `steam guard code` / `code sent to` 等完整短语。
- `isSteamCMDGuardCodePrompt()` 现在额外识别上述分行提示；`runSteamCMDFallback()` 命中后继续复用现有逻辑，把实例 phase 更新为 `steamcmd_guard_required`，前端通过原有 `POST /api/instances/:id/steam-guard/input` 提交邮箱/App 验证码。
- 影响文件：`backend/internal/games/stardew_junimo/installer.go`、`backend/internal/games/stardew_junimo/driver_test.go`。
- 验证：`cd backend; go test ./internal/games/stardew_junimo -run "SteamCMDGuardCodePrompt|SteamGuardCodePrompt"`。

# INSTALL-ROUTING-SPLIT-1 安装路由三决策拆分 + 更换账号入口

- 把过去由 `reuseCredentials` 粗暴驱动、且一个 `steamCMDRetry` 字段兼管两职的安装路由，拆成三个正交决策，均按**实例已持久化的 driverPhase/state** + **一个新的 `.env` 标志**推导：
  - `reuse`：复用已存账密、不再弹表单（= `reuseCredentials`）。
  - `steamCMDDirect`（driver.go 计算）：`reuse && !forceReauth && (shouldResumeSteamCMD(phase) || authAlreadySucceeded(state, phase))`。为真才跳过拉镜像 + steam-auth 直达 SteamCMD。新增 `authAlreadySucceeded(state, phase)`：phase ∈ {download_failed,post_auth_failed,game_downloading,steam_sdk_downloading,game_installed} 或 state 属已安装态即视为“已过认证”，作为跨会话的持久判据。
  - `steamCMDUseCache`（installer.go run() 从 `.env` 读 `STEAMCMD_AUTH_COMPLETED` 得出）：SteamCMD 用“仅用户名缓存登录”还是“账号密码完整登录”。替换了 fallback 内部原先的 `r.steamCMDRetry` 判断。
- **两个对称的持久“认证成功”标志**（均写在实例 `.env`，`forceReauth` 时清空）：
  - `STEAM_AUTH_COMPLETED`：在 steam-auth 容器日志明确出现登录成功（`[SteamAuth:*] Logged in as ...` / `[SteamService] ... Logged in as ...`）后写入；启动或刷新成功拿到非空邀请码时也会写入。`driver.Install` 读它并纳入 `steamCMDDirect` 判据，作为 `authAlreadySucceeded(state, phase)` 相位推断的兜底——即使 phase 被重置为 `install_interrupted` 也能可靠跳过 steam-auth。
  - `STEAMCMD_AUTH_COMPLETED`：SteamCMD `logged in ok` 后写入，控制 `steamCMDUseCache`。
  - 二者**各自独立、互不牵连**（steam-auth 与 SteamCMD 是两套不同的登录凭证/会话）：`STEAM_AUTH_COMPLETED` 由 steam-auth 认证成功或非空邀请码置位、决定“是否还需要跑 steam-auth 登录步骤”；`STEAMCMD_AUTH_COMPLETED` 只由 SteamCMD 登录成功置位、决定“SteamCMD 用缓存还是完整登录”。SteamCMD 登录成功**不会**顺带把 `STEAM_AUTH_COMPLETED` 也置位。
- **“steam-auth 标志有效期间走 cmd”**：路由用 `STEAM_AUTH_COMPLETED` 作 `steamCMDDirect` 判据——首次安装 steam-auth 认证成功（用于建服/邀请码）并继续下载游戏文件与 SDK；该标志为 true 期间，后续 `reuse`（重试/重装/更新）走 SteamCMD，不再跑 steam-auth。只有“认证尚未成功过”或启动日志已把标志清回 false 的情况，才会重新走 steam-auth。
- **修复①（镜像拉取失败重试）**：`pull_failed` 不在上述直达条件里，因此复用凭据重试会**重新拉镜像 + 走 steam-auth**（`reuse` ⇒ autoMode 自动账号密码继续，跳过登录方式选择），不再误跳 SteamCMD。
- **修复②（SteamCMD 认证超时重试）**：`STEAMCMD_AUTH_COMPLETED` 未置位时 `steamCMDUseCache=false` ⇒ SteamCMD 走**完整登录**并展示 guard 提示（回到验证界面），不再一上来报“授权缓存不可用”。该标志在检测到 SteamCMD `logged in ok` 后写入 `.env`，此后任何操作命中缓存路径。
- **更换账号 / 强制重新认证**：`POST /api/instances/:id/install` 新增 `forceReauth`。为真时后端要求提供新账密（不复用）、清空 `.env` 的 `STEAM_REFRESH_TOKEN` 与 `STEAMCMD_AUTH_COMPLETED`，并调用新增的 `docker.RemoveVolumes` 清除 `<project>_steam-session` 与 `<project>_steamcmd-*` 授权卷（保留 `game-data`），随后走完整认证。卷删除 best-effort，占用中失败仅告警不阻断。
- `install_handlers.go` 不再传 `SteamCMDRetry: reuseCredentials`；`registry.InstallRequest` 增加 `ForceReauth`，`SteamCMDRetry` 保留为兼容字段。`config.EmptyEnvTemplate`/写序新增 `STEAMCMD_AUTH_COMPLETED`。
- 影响文件：`backend/internal/web/install_handlers.go`、`backend/internal/games/stardew_junimo/driver.go`、`installer.go`、`config/env.go`、`backend/internal/docker/compose.go`、`registry/types.go`、及相关 `_test.go`。
- 验证：`cd backend; go test ./...`（新增 `TestDriverInstallReRunsSteamAuthAfterPullFailureRetry`，并更新了 repair 用例先写入 `STEAMCMD_AUTH_COMPLETED=true`）。

# STEAMCMD-REPAIR-DIRECT-1 修复/重新安装直达 SteamCMD

- `POST /api/instances/:id/install` 在收到 `reuseCredentials=true` 时，除了继续从实例 `.env` 读取已保存 `STEAM_USERNAME` / `STEAM_PASSWORD` / `VNC_PASSWORD`，现在会显式传递 `SteamCMDRetry=true` 给 driver。
- 这条路径用于安装页“重新安装 / 修复”、认证后下载失败重试、SteamCMD 重试等复用凭据入口；后端会跳过 Junimo 镜像检查和 `steam-auth`，直接进入 SteamCMD 下载/校验，不再让用户重新输入 Steam 账号密码，也不再走一遍 `steam-auth` 登录流程。
- SteamCMD 直达模式会优先使用已有 SteamCMD 登录授权缓存执行 `+@NoPromptForPassword 1 ... +login "$STEAM_USERNAME" +app_update 413150 validate ...`，不再在命令里用账号密码触发新一轮 Steam Guard 批准；首次兜底且没有缓存时仍保留账号密码登录能力。
- 如果直达修复模式下缓存明确失效，后端会在同一个 job 内自动清空失效标志并回退完整登录；只有此时 SteamCMD 才可能再次要求批准。
- 影响文件：`backend/internal/web/install_handlers.go`、`backend/internal/games/stardew_junimo/installer.go`、`backend/internal/games/stardew_junimo/driver_test.go`。
- 验证：`cd backend; go test ./internal/games/stardew_junimo ./internal/web`。

# PUBLIC-IP-LOOKUP-1 服务器公网 IP 检测接口
- 新增 `GET /api/instances/:id/public-ip`，登录用户可调用，用于从面板后端所在服务器主动检测公网出口 IP；前端不要在浏览器里查 IP，避免拿到访问者客户端的公网 IP。
- 接口默认使用短超时 HTTP 客户端按顺序请求 `api.ipify.org`、`checkip.amazonaws.com`、`ifconfig.me/ip`，只接受 `netip.ParseAddr()` 可解析且非 private/loopback/link-local/multicast/unspecified 的公网地址。
- 响应结构为 `{ ip, checkedAt, source?, cached }`。后端默认缓存成功结果 `10min`；请求 `?refresh=1` 或 `?refresh=true` 会强制重新检测。失败返回 `502 public_ip_failed`，不会暴露外部服务原始错误。
- 影响文件：`backend/internal/web/public_ip.go`、`backend/internal/web/public_ip_test.go`、`backend/internal/web/handler.go`、`backend/internal/web/instance_handlers.go`。未改 Junimo driver、Docker/Compose 状态、邀请码接口或实例状态模型。
- 验证：`cd backend; go test ./internal/web`。

# SUPPORT-BUNDLE-STREAM-1 支持包流式 ZIP 导出

- `POST /api/instances/:id/support-bundle` 不再先用 `bytes.Buffer` 在内存中完整拼出 ZIP 后一次性写回，而是设置下载响应头后直接用 `zip.NewWriter(w)` 流式写入 `http.ResponseWriter`。
- 响应继续是 `application/zip` 和 `support-bundle-YYYYMMDD-HHMMSS.zip`；因为改为流式响应，不再设置 `Content-Length`，浏览器下载和前端 Blob 处理不受影响。
- 支持包内容不变：`version.json`、`health.json`、`instance-state.json`、`jobs.json`、`audit-logs.json`、`compose-ps.json`、`docker-compose.yml` 或说明、`server-logs.txt`，敏感信息仍通过 Docker redact 逻辑脱敏。
- 新增 `TestSupportBundleStreamsValidZip` 覆盖下载仍是合法 ZIP、关键条目存在且不写固定 `Content-Length`。
- 验证：`cd backend; go test ./internal/web -run "SupportBundle|Docker|Metrics"`，`cd backend; go test ./...`。

# DOCKER-POLL-PERF-1 Docker Compose 状态短缓存与轮询边界

- `backend/internal/docker.Client.ComposePs()` 现在对成功的 `docker compose ps --format json` 结果做短 TTL 缓存，默认 `1.5s`。同一实例在状态页、资源页、支持包或诊断路径短时间内重复读取 Compose 状态时，可复用同一份解析结果，减少 Docker CLI 进程启动开销。
- 缓存只覆盖 `ComposePs` 成功结果，不缓存失败；`ComposeUp`、`ComposeDown`、`ComposeRestart` 和 `ComposeRestartServices` 会在执行前后清理对应 workDir 的 `ComposePs` 缓存，避免生命周期命令后短时间读到旧状态。
- `ComposeStats --no-stream` 不做后端缓存，仍只通过 `/api/instances/:id/metrics` 按需执行。它比 `ComposePs` 重，前端应限制为诊断/资源可见页低频刷新。
- `DockerVersion` / `ComposeVersion` 仍用于 `/api/health/diagnostics`、Docker 状态页或安装前检查，不应进入普通总览常驻轮询。
- 验证：`cd backend; go test -count=1 ./internal/docker`。

# JUNIMO-MOD-MOUNT-RESTORE-1 官方 JunimoServer Mod 挂载修复

- 根因：实例 `.local-container/mods` 会完整挂载到容器 `/data/Mods`，如果宿主目录里没有 `JunimoServer/`，就会遮住 `sdvd/server` 镜像内置的官方 `JunimoServer` Mod，导致 8080 API、邀请码、VNC rendering 全部不可用。
- 启动/重启前现在会检查 `.local-container/mods/JunimoServer/manifest.json`；缺失时用当前 server 镜像临时容器把 `/data/Mods/JunimoServer` 同步回宿主 mods 目录。
- `JunimoHost.Server` 现在按内置服务端组件处理：永远启用、不可切换、不写入玩家同步包、不受新存档“默认禁用第三方 Mod”影响。
- 物理 `mods/smapi/` 是 SMAPI 自带运行时目录，不再作为本地 Mod 扫描，页面只保留虚拟的 `SMAPI` 内置卡，避免出现重复 `smapi` 且解析失败。
- VNC rendering 调用如果遇到 Junimo API connection refused，会返回 `junimo_api_unavailable` 结构化错误，不再只显示笼统 Docker 操作失败。
- 验证：`cd backend; go test ./internal/games/stardew_junimo -run "ListMods|ApplyNewSaveDefault|ApplyModProfileKeeps|Rendering|JunimoServerMod"`，`cd backend; go test ./internal/web -run "Rendering|VNCConfig"`。

# ENV-BOM-NORMALIZE-1 实例 .env 隐藏 BOM 归一化

- `config.ReadEnvFile()` 现在会剥离行首和 key 前缀的 UTF-8 BOM，避免实例 `.env` 中混入 `﻿IMAGE_VERSION` 这类不可见变量名后，被 Docker Compose 报 `unexpected character "\ufeff"`。
- `UpdateEnvFile()` 通过读取归一化后的 key 再写回 `.env`，会把 BOM 污染的重复 key 收敛成正常 `IMAGE_VERSION`，不再把非法变量名保留下来。
- 已热修当前测试实例 `data/instances/stardew/.env`，删除中间残留的 BOM 前缀 `IMAGE_VERSION` 行；`docker compose -f data/instances/stardew/docker-compose.yml config --quiet` 已通过。
- 验证：`cd backend; go test ./internal/games/stardew_junimo/config`。

# STEAMCMD-SELFUPDATE-CACHE-1 SteamCMD 客户端与授权缓存（已由 STEAMCMD-CREDENTIAL-REUSE-1 调整）

- 历史实现曾分别挂载 `steamcmd-root-local` 与 `steamcmd-user-local`；当前已改为统一使用 `steamcmd-login` 覆盖 root/steam 两种用户的 `Steam` 与 `.local/share/Steam` 路径，避免候选镜像切换后丢失 SteamCMD 自己的登录会话。
- 用户日志里的 `[steamcmd] [ 40%] Downloading update (.. of 40,273 KB)` 是 SteamCMD 客户端自更新，不是 Docker 镜像拉取；后端会在启动 SteamCMD 前写明镜像检查已完成，避免误解。
- `buildSteamCMDOpts()` 会创建并 chown 上述目录；仍只使用 SteamCMD 候选镜像和现有 `game-data`、`steamcmd-login`、`steamcmd-home` 命名卷，不新增 Junimo/server 镜像来源。旧 local 卷留在磁盘但不再挂载，避免自动删除潜在授权数据。
- 影响文件：`backend/internal/games/stardew_junimo/installer.go`、`backend/internal/games/stardew_junimo/driver_test.go`。
- 验证：`cd backend; go test ./internal/games/stardew_junimo -run "SteamCMD|InstallFallsBack|InstallResumes|InstallUsesExistingLater|InstallSteamCMD"`。

# STEAMCMD-RETRY-RESUME-1 SteamCMD 兜底重试直达

- 安装请求复用已保存凭据时，如果实例当前 `driverPhase` 已处于 `steamcmd_auth_running`、`steamcmd_guard_*`、`steamcmd_downloading`、`steamcmd_failed` 或 `steamcmd_image_pull_failed`，`stardew_junimo` 会跳过 Junimo compose pull 和 `steam-auth`，直接进入 `runSteamCMDFallback()`。
- 直达重试会重新注册当前 job 的 Steam Guard 输入通道，所以前端继续使用 `POST /api/instances/:id/steam-guard/input` 提交 SteamCMD 手机批准选择或 App/邮箱验证码。
- `ensureSteamCMDImage()` 现在先 inspect 完整 `STEAMCMD_IMAGE_CANDIDATES` 列表；只要任意候选本地存在就直接使用，不会因为前序候选缺失而先 pull。只有所有候选都不存在时才按顺序拉取。
- 新增 `registry.InstallRequest.SteamCMDRetry` 作为显式入口；当前 web 层仍通过 `reuseCredentials=true` + 已持久化 `driverPhase` 由 driver 自动推断。
- 影响文件：`backend/internal/games/registry/types.go`、`backend/internal/games/stardew_junimo/driver.go`、`backend/internal/games/stardew_junimo/installer.go`、`backend/internal/games/stardew_junimo/driver_test.go`。
- 验证：`cd backend; go test ./internal/games/stardew_junimo -run "SteamCMD|InstallFallsBack|InstallResumes|InstallUsesExistingLater|InstallSteamCMD"`；前端联动验证见 `FE-STEAMCMD-RETRY-RESUME-1`。

# STEAMCMD-FALLBACK-1 steam-auth 下载失败自动切 SteamCMD

- `stardew_junimo` 安装流程中，`steam-auth` 已经认证成功但游戏文件下载失败时，不再直接结束为 `download_failed`。后端会在同一个 `stardew_install` job 内自动切换到 SteamCMD 兜底下载，复用 `.env` 中保存的 `STEAM_USERNAME` / `STEAM_PASSWORD`，不要求用户重新输入账号密码。
- 新增通用 Docker TTY 执行能力 `RunContainerTTY()` 和单镜像拉取 `PullImageStreaming()`；SteamCMD 会按 `STEAMCMD_IMAGE_CANDIDATES` 候选列表逐个 `inspect/pull`，默认顺序为 `docker.1ms.run/steamcmd/steamcmd:latest`、`docker.m.daocloud.io/steamcmd/steamcmd:latest`、`ghcr.io/steamcmd/steamcmd:latest`、`cm2network/steamcmd:latest`。旧实例仍写着旧候选列表时，安装流程会补齐新候选并过滤直连 Docker Hub 的 `steamcmd/steamcmd:latest` 和已移除的 `docker.xuanyuan.me/steamcmd/steamcmd:latest`；单次 Docker pull 默认等待 30 分钟。兜底容器挂载 `game-data` 到 `/data/game`，并挂载独立 `steamcmd-login` / `steamcmd-home` 命名卷保存 SteamCMD 登录授权缓存。
- SteamCMD 阶段新增可见 phase：`steamcmd_image_pulling`、`steamcmd_auth_running`、`steamcmd_guard_choice_required`、`steamcmd_guard_required`、`steamcmd_guard_mobile_required`、`steamcmd_downloading`、`steamcmd_failed`、`steamcmd_image_pull_failed`。需要验证时继续复用 `POST /api/instances/:id/steam-guard/input`，但文案明确是“steam-auth 国内网络波动下载失败，需要重新授权 SteamCMD”。
- SteamCMD 手机 App 批准超时不再视为安装成功；即使容器退出码为 0，也必须看到 `Success! App '413150' fully installed.` 才会把兜底下载判定为成功。SteamCMD `Update state ... progress: N (done / total)` 继续作为任务日志输出，供前端显示下载百分比。
- SteamCMD 命令会下载/校验 Stardew Valley app `413150`，并尝试把 Steamworks SDK Redistributable app `1007` 安装到 `/data/game/.steam-sdk`。若 SDK 未输出完成标记只记录 warning，游戏 app 完成仍视为兜底下载成功。
- 影响文件：`backend/internal/docker/streaming.go`、`backend/internal/docker/tty_run*.go`、`backend/internal/games/stardew_junimo/installer.go`、`backend/internal/games/stardew_junimo/driver.go`、`backend/internal/web/install_handlers.go`、对应测试文件。
- 验证：`cd backend; go test ./internal/docker ./internal/web ./internal/games/stardew_junimo`；目标覆盖 `TestDriverInstallFallsBackToSteamCMDAfterSuccessfulAuthDownloadFailure` 和 `TestDriverInstallTriesNextSteamCMDImageCandidateAfterPullFailure`。

# INSTALL-INTERRUPTED-STATE-1 安装中断状态回收

- `jobs.Manager.RecoverInterruptedJobs()` 现在在面板重启时不仅把 queued/running job 标记为 failed，也会针对 `stardew_install` 的 instance target 同步把实例改为 `state=error`、`driverPhase=install_interrupted`，避免任务已失败但安装页仍读到旧的 `steam_auth_running`。
- `stardew_junimo` 安装流程中，`RunSteamAuthTTY` 自身返回错误时会把实例改为 `steam_auth_failed` / `driverPhase=steam_auth_failed`，并写入脱敏后的 job error log；未知非零退出码不再回退到误导性的 `credentials_required` phase。
- 影响文件：`backend/internal/jobs/manager.go`、`backend/internal/games/stardew_junimo/installer.go`、对应测试文件。接口路径不变，变化体现在实例状态和安装页可见 phase。
- 验证：`cd backend; go test ./internal/jobs ./internal/games/stardew_junimo`。

# NEXUS-ERROR-TEXT-1 Nexus 错误中文乱码修复

- `writeNexusError()` 的 Nexus 错误响应 message 已修复为正常 UTF-8 中文：未配置 Key、需要 OAuth/认证、未找到 Mod、Key 无效/权限不足、请求过频和通用请求失败不再返回 `璇锋眰澶辫触` 这类 mojibake。
- 错误码和 HTTP 状态不变：`nexus_api_key_missing`、`nexus_auth_required`、`nexus_mod_not_found`、`nexus_unauthorized`、`nexus_rate_limited`、`nexus_request_failed` 仍按原契约返回。
- 新增 `TestWriteNexusErrorMessagesAreReadableChinese`，直接覆盖 `writeNexusError` 的可读中文响应，防止 Nexus 搜索/安装失败时再次把乱码透给前端。
- 验证：`cd backend; go test ./internal/web -run TestWriteNexusErrorMessagesAreReadableChinese`。

# PLAYERS-SAVE-ROSTER-1 存档离线玩家名册恢复

- `GET /api/instances/:id/players` 的 Junimo driver 合并逻辑现在会在 `StardewAnxiPanel.Control players.json` 在线快照和 `players-cache.json` 之外，读取当前存档主 XML 的 `<player>` 与 `<farmhands><Farmer>`，把存档里存在但当前不在线、也尚未进入缓存的玩家补入名册。
- 存档目录优先取 Junimo gameloader 当前存档；找不到时按控制快照 `saveId` 匹配 `Saves/<saveId>` 或 `Saves/<saveId>_*`。因此控制文件里 `saveId=test`、真实目录为 `test_442477055` 时，farmhand `test` 会显示为 `status=offline`、`source=save_file`。
- 仍由 `backend/internal/games/stardew_junimo/players.go` 完成，不在 Web/API 层堆 Stardew XML 逻辑；在线快照会覆盖同一玩家的存档离线项，缓存仍按 `saveId` 隔离，避免上一存档玩家串到当前存档。
- 新增 `TestListPlayersMergesControlSnapshotWithSaveFarmhands` 覆盖“缓存只有 host / 在线快照只有 host / 存档 farmhands 有 test”的场景。
- 验证：`cd backend; go test ./internal/games/stardew_junimo`；`cd frontend; npm.cmd run build`。

# JOB-DISPLAY-NAME-1 任务展示名字段

- jobs 表新增 `display_name` 字段（迁移 `007_job_display_name.sql`），用于保存面向用户的任务展示名；`type` 继续保持机器可读任务类型，不用于拼接 Mod 名，避免影响任务筛选和历史耗时统计。
- `jobs.Spec` / `CreateJobParams` 增加 `DisplayName`，`GET /api/jobs`、`GET /api/jobs/:id` 和 job SSE 里的 job payload 增加 `displayName`。
- Nexus/远程 Mod 安装任务现在写入展示名：`mod_nexus_install · <Mod 名>`、`mod_remote_install · <Mod 名>`；若请求未带名称但有 `modId`，退回 `Nexus Mod #<id>`；普通任务和旧任务没有展示名时前端继续按 `type` 展示。
- 验证：`cd backend; go test ./internal/storage ./internal/jobs ./internal/web`。

# MODUPLOAD-DUPLICATE-CODE-1 重复 Mod 上传返回专用错误码

- `POST /api/instances/:id/mods/upload` 在 `UploadModZip` 命中已安装相同 `UniqueID` 时，现在返回错误码 `mod_exists`，不再统一归为 `invalid_mod_zip`。
- ZIP 结构校验、manifest 解析、XNB 替换包识别和真正损坏 ZIP 的行为不变；只有错误消息含后端重复标记 `(mod_exists)` 时才切换错误码。
- 前端已有 `mod_exists -> 已安装相同 ID 的 Mod` 文案，因此用户重复上传已安装 Mod 时不会再误以为 ZIP 无法解析。
- 验证：`cd backend; go test ./internal/web -run "TestModUpload"`。

# NEXUS-EXT-DOWNLOAD-GUARD-1 远程 Mod 下载进度与网页响应拦截

- `mod_remote_install` / `mod_nexus_install` 的 ZIP 下载阶段现在会向 job 日志输出可见进度：连接远程下载服务器、HTTP 响应码、响应 `Content-Type`、压缩包大小，以及下载进度“已下载 / 总量 / 剩余 / 百分比”。无 `Content-Length` 时仍会输出已下载大小与“总大小未知”。
- 下载进度在 `stardew_junimo.nexusDownloadArchive()` 内统一实现，NXM、Nexus API Key 直连和浏览器扩展捕获 CDN ZIP 三条路径共享；日志按 5 MB 或 2 秒节流，完成时补最终进度，避免大包刷屏。
- 如果远程响应 `Content-Type` 是 `text/html`，后端会立即失败并提示“远程下载返回的是网页，不是 ZIP 压缩包；请确认浏览器扩展已经拿到 Nexus CDN ZIP 下载链接”，避免把 Nexus 下载页/错误页当作 ZIP 任务继续跑。
- 浏览器扩展提交层额外兜底：`background.finishInstall()` / `postRemoteInstall()` 和 `panel-bridge` 只允许真实 Nexus CDN `.zip` 链接创建面板远程安装任务；还停在普通 Nexus 下载页时不会再提前触发后端任务。
- 验证：`cd backend; go test ./internal/games/stardew_junimo -run "InstallNexusMod|NexusDownloadArchive|Remote|Download"`、`go test ./internal/games/stardew_junimo ./internal/web`、扩展 `node --check`。

# PLAYERSYNC-PACK-10 禁用终端进度渲染

- 玩家同步包 `tools/install.ps1` 完全禁用控制台进度渲染：设置 `$ProgressPreference = "SilentlyContinue"`，并移除 `Show-InstallProgress` / `Complete-InstallProgress` 里的 `Write-Progress`。
- 进度百分比仍写入 `.anxi-sync/logs/install-*.log`，用于排查；玩家终端只显示 `Write-Step` 的独立任务行和最终摘要。
- 修复真实 Windows Terminal 中 `Write-Progress` 与中文 `Write-Host` 混用导致的“安安装装/已已跳跳过过”双字重叠和多行粘连。
- 当前测试解压包 `C:\Users\anxi\Downloads\stardew-player-sync-pack\tools\install.ps1` 已热修，并在 `D:\steam\steamapps\common\Stardew Valley` 真实安装验证输出干净。

# PLAYERSYNC-PACK-9 Steam 启动项摘要精简

- 玩家同步包安装完成摘要移除单独的 `SMAPI 路径` 输出，只保留玩家真正需要复制到 Steam 的 `Steam 启动项文本`。
- 最终输出形态为 `Steam 启动项：已设置/未自动设置` + `Steam 启动项文本：` + 完整 launch options，一行标题、一行可复制内容。
- 当前测试解压包 `C:\Users\anxi\Downloads\stardew-player-sync-pack\tools\install.ps1` 已热修，并真实验证输出不再显示单独路径。

# PLAYERSYNC-PACK-8 终端输出清理

- 玩家同步包 `tools/install.ps1` 移除自绘单行进度条的 carriage-return 覆盖输出，避免 Windows Terminal / bat 捕获中文宽度时出现重复字、残留字和多行粘连。
- `Show-InstallProgress` 后续在 PLAYERSYNC-PACK-10 中已改为只把进度写入日志文件；控制台只输出 `Write-Step` 任务日志，一行一条。
- `tools/steam-launch-options.ps1` 的建议启动项改为标题和启动项文本分两行输出。
- 安装完成摘要不再写“未自动设置，请查看上方提示”，而是固定输出 `Steam 启动项：已设置/未自动设置` 和 `Steam 启动项文本`，方便玩家直接复制。
- 当前测试解压包 `C:\Users\anxi\Downloads\stardew-player-sync-pack\tools\install.ps1` 与 `tools\steam-launch-options.ps1` 已热修，并用真实安装验证输出干净。

# PLAYERSYNC-PACK-7 相同 Mod 跳过安装

- 玩家同步包 `tools/install.ps1` 新增目录内容指纹比对：安装每个 Mod 前分别计算 `payload/mods/<folderName>` 与 `<Stardew Valley>/Mods/<folderName>` 的稳定指纹。
- 指纹由目录相对路径、文件相对路径、文件大小和 SHA256 组成；完全一致时认为目标 Mod 已经是同一份内容，脚本会跳过备份和复制，并在 `installed.json` 对该 Mod 写入 `skippedIdentical=true`。
- 版本不同或文件内容不同不会跳过：只要 manifest、DLL、图片、content.json 等任意文件内容/路径/大小变化，指纹就不同，脚本会先备份旧目录再复制新目录。版本号相同但文件变了也会更新。
- 全部 Mod 都跳过且没有实际备份时，`installed.json.backupId` 写入 `null`，避免指向一个不存在的备份目录。
- 已热修当前测试解压包 `C:\Users\anxi\Downloads\stardew-player-sync-pack\tools\install.ps1`，并用当前游戏目录验证三项已安装 Mod 均会 `已跳过相同 Mod`。

# PLAYERSYNC-PACK-6 单行动态进度

- 玩家同步包 `tools/install.ps1` 的文本进度条改为单行动态刷新，不再为 checksum / Mod 复制的每个进度 tick 都 `Write-Host` 新行。
- 新增 `Render-InstallProgressLine` / `Clear-InstallProgressLine` / `Finish-InstallProgressLine` / `Redraw-InstallProgressLine`：控制台使用 `[Console]::Write([char]13...)` 回到行首刷新当前进度；普通安装事件通过 `Write-Step` 先清除进度行、打印事件，再重绘进度行。
- 日志文件仍会保留关键安装步骤与进度记录；控制台上则只保留一条活动进度线，避免玩家看到大量重复进度行。
- 已热修当前测试解压包 `C:\Users\anxi\Downloads\stardew-player-sync-pack\tools\install.ps1`。

# PLAYERSYNC-PACK-5 SMAPI payload 安装修复

- 玩家同步包 `tools/install.ps1` 恢复调用 SMAPI 官方 Windows 安装器：解压随包 SMAPI ZIP 后定位 `internal/windows/SMAPI.Installer.exe`，传入 `--install --game-path "<Stardew Valley>" --no-prompt`。
- 真实 Windows 测试确认 SMAPI 4.5.2 安装器没有可用静默参数，非交互调用会进入交互流程并可能因为 Console 句柄失败；失败安装还可能把安装器运行时 DLL 散落到游戏根目录。
- 官方安装器调用通过 `Start-Process` 执行，脚本会等待安装器退出；超过 120 秒会终止并提示玩家关注安装器窗口是否在等待按键/输入。脚本不再直接解包 `install.dat`，也不做本机 `.NET` / `runtimeconfig` 特调。
- 热修复已同步到当前测试解压包 `C:\Users\anxi\Downloads\stardew-player-sync-pack\tools\install.ps1`。已用 `-GamePath "D:\steam\steamapps\common\Stardew Valley" -SkipSteamLaunchOptions` 真实安装验证，SMAPI 和玩家同步 Mod 均安装成功。
- 测试补充：`TestPlayerSyncInstallScriptUsesSMAPIInstallPayload` 防止脚本回退到交互安装器调用；`TestPlayerSyncPowerShellScriptsParse` 覆盖脚本语法。

# PLAYERSYNC-PACK-4 安装进度显示

- 玩家同步包的 `tools/install.ps1` 新增 `Show-InstallProgress` / `Complete-InstallProgress`，安装时会同时调用 PowerShell `Write-Progress` 和输出文本进度条。
- 进度阶段覆盖：开始、环境检查、读取清单、定位 Stardew Valley、进程/权限检查、payload checksum、SMAPI 解压/安装、Mod 备份与复制、Steam 启动项、写入安装记录、完成。
- checksum 阶段按 `checksums.sha256` 文件数推进，Mod 阶段按 `packaged=true` 的 Mod 数推进；SMAPI 阶段显示“解压 SMAPI 安装包 / 释放 SMAPI 官方安装文件 / 完成”。
- 已同步热修当前解压包 `C:\Users\anxi\Downloads\stardew-player-sync-pack\tools\install.ps1`；后续重新导出的包会自动带进度条。

# PLAYERSYNC-PACK-3 方括号 Mod 路径修复

- 玩家端安装脚本在校验 `checksums.sha256` 和复制/卸载 Mod 时，所有来自 payload 或 `installed.json` 的 Mod 路径必须使用 PowerShell `-LiteralPath`。
- 真实触发场景：`payload/mods/[CP] MultipleConstructionOrders/Assets/ConstructionWorker.png` 中的 `[CP]` 会被 `Test-Path` / `Get-FileHash -Path` 当作通配符字符集，导致明明存在的文件被误报为“checksum 指向的文件不存在”。
- 已修复 `tools/install.ps1` 的 checksum 校验、payload source 检查、目标 Mod 存在检查，以及 `tools/uninstall.ps1` 的安装记录、目标 Mod、备份目录检查。
- 测试补充：`TestPlayerSyncInstallScriptUsesLiteralPathsForModFolders` 防止模板退回通配符路径；真实解压包用等价 checksum 循环验证 9 个 payload 文件全部存在且 hash 匹配。

# PLAYERSYNC-PACK-2 玩家同步安装包

- `POST /api/instances/:id/mods/sync-pack/export` 仍返回 `stardew-player-sync-pack.zip`，但 ZIP 内容已升级为玩家可执行安装包：根目录包含 `安装玩家同步包.bat`、`卸载本同步包.bat`、`README.txt`、`pack-manifest.json`、`checksums.sha256`、`tools/` 和 `payload/`。
- 玩家需同步 Mod 写入 `payload/mods/<folderName>/`，不再放在 ZIP 根目录；`StardewAnxiPanel.Control` 继续永远排除。SMAPI 虚拟条目写入 `pack-manifest.json`，不会被当作普通 Mod 文件夹复制。
- `payload/smapi/smapi.json` 总会写入。导出逻辑会优先查找服务端实例目录下 `.local-container/smapi/`、`.local-container/control/smapi/`、`smapi/` 里的 `SMAPI*.zip`；找到时随包写入 `payload/smapi/` 并记录 SHA256，找不到时同步包仍可导出，但玩家脚本只安装 Mod 并提示自行安装 SMAPI。
- `checksums.sha256` 覆盖 `payload/mods` 文件和随包 SMAPI ZIP。安装脚本在玩家电脑上先校验 payload 完整性，再安装/更新 SMAPI、备份同名 Mod、复制新 Mod，并尽力设置 Steam 启动项。
- 玩家端安装状态写入游戏目录 `.anxi-sync/installed.json`、`.anxi-sync/backups/`、`.anxi-sync/logs/`。卸载脚本按 `installed.json` 移除本包安装的 Mod，并可通过 `-RestoreBackup` 恢复备份；不会默认卸载玩家已有 SMAPI。
- 验证：`go test ./internal/games/stardew_junimo ./internal/web`、`go test ./...`。

# NEXUS-SMAPI-THUMB-1 虚拟 SMAPI 缩略图补全

- `smapiRuntimeModInfo()` 现在给内置虚拟 SMAPI 条目补 `UpdateKeys: ["Nexus:2400"]`、`nexusModId=2400` 和 Nexus 页面 URL。
- `GET /api/instances/:id/mods` 继续通过 `EnrichNexusMetadataForMods()` 统一补全 Nexus 元数据；因此虚拟 SMAPI 没有真实 `manifest.json` 也会进入 GraphQL v2 补全链路，成功后返回 `pictureUrl/nexusSummary/downloadCount/endorsementCount/updatedAt`。
- `ApplyNexusMetadataToMods()` 现在会按同一个 Nexus `modId` 合并更完整的缓存记录，避免内容包自己的最小来源记录遮住主 Mod 已缓存的 `pictureUrl`。
- 缩略图仍走 `.local-container/control/nexus-mods.json` 缓存；首次拉取失败不会影响 Mod 列表，只会暂时显示前端占位。
- 验证：`go test ./internal/games/stardew_junimo ./internal/web` 通过。

# MODZIP-3 UTF-8 BOM manifest 兼容

- `readModInfo` 读取 `manifest.json` 后会剥离 UTF-8 BOM（`EF BB BF`）再交给 `encoding/json` 解析，兼容部分 Nexus ZIP 内由 Windows 工具保存的 manifest。
- 修复的真实场景：`MultipleConstructionOrders 1.6 V1.1.0-47289-1-1-0-1780961270.zip` 中 `[CP] MultipleConstructionOrders/manifest.json` 带 BOM，之前会报 `invalid character 'ï' looking for beginning of value`，Web 层最终显示为 `Mod ZIP 无效`。
- 该修复不放宽 JSON 语法校验，只处理文件开头 BOM；非法 JSON 仍会按原逻辑拒绝。
- `sanitizeError` 现在会保留常见 Mod 上传校验原因（目录已存在、UniqueID 重复、具体 Mod 的 manifest 解析失败等），避免前端只显示笼统的“第 N 个 Mod ZIP 无效”。
- 验证：`go test ./internal/games/stardew_junimo -run "UploadModZip|ReadModInfo"`。

# MODZIP-4 manifest JSONC 兼容

- `readModInfo` 新增 `decodeModManifest`，先按标准 JSON 解析；失败后仅对 SMAPI `manifest.json` 做 JSONC 兼容清理：移除字符串外的 `//` 行注释、`/* ... */` 块注释和对象/数组尾随逗号，再重新解析。
- 修复的真实场景：Nexus CDN 远程安装 `SpaceCore` 时，ZIP 下载和解压已成功，但 `manifest.json` 含注释导致 `invalid character '/' looking for beginning of object key string`，最终 `mod_remote_install` 失败。
- 清理逻辑会保留字符串内容，例如 `https://...` 中的 `//` 不会被当作注释；ZIP 路径校验、manifest 必填字段、重复 UniqueID、XNB 替换包识别等安全规则不变。
- 验证：`go test ./internal/games/stardew_junimo -run "ReadModInfo|UploadModZip"`。

# MODDEPS-1 Mod 前置依赖字段

- `GET /api/instances/:id/mods` 的 `mods[]` 现在会从每个 Mod 的 `manifest.json` 解析前置依赖并返回 `dependencies[]`：`{ uniqueId, minimumVersion?, required }`。
- 支持 SMAPI 标准 `Dependencies` 数组，并把 `ContentPackFor` 也归一为一个必需依赖；例如 Content Patcher 内容包会返回 `Pathoschild.ContentPatcher`，前端可直接展示“需要前置依赖：Content Patcher”。
- `IsRequired` 缺省按必需依赖处理，`false` 会作为可选依赖返回；同一个 `UniqueID` 在 `ContentPackFor` 和 `Dependencies` 中重复时会去重，优先保留 `ContentPackFor` 的最低版本信息。
- 该阶段只做解析和展示字段，不自动下载依赖，也不判断依赖是否已安装；真正的缺失依赖检查仍留给后续配置页能力。
- 验证：`go test ./internal/games/stardew_junimo ./internal/web` 覆盖 `Dependencies`、可选依赖和 `ContentPackFor` 去重。

# MODZIP-2 Nexus 外壳目录自动剥离

- `UploadModZip` 支持 Nexus 常见的“单层外壳目录”ZIP：如果压缩包根目录只有一个包装目录，且包装目录本身没有 `manifest.json`，后端会向内看一层，把其中带 `manifest.json` 的子目录作为真正的 SMAPI Mod 导入。
- 示例结构 `MultipleConstructionOrders/MultipleConstructionOrders/manifest.json` + `MultipleConstructionOrders/[CP] MultipleConstructionOrders/manifest.json` 会自动导入为服务器 `Mods/MultipleConstructionOrders` 和 `Mods/[CP] MultipleConstructionOrders`，不要求用户手动解压重打包。
- 仍只剥离一层外壳目录；zip-slip、绝对路径、重复目录名、重复 UniqueID、已安装冲突和 XNB 替换包识别等原有安全校验继续生效。
- 验证：`go test ./internal/games/stardew_junimo ./internal/web` 覆盖 Nexus 外壳目录多 Mod 包上传。

# MODRESTART-1 停服改 Mod 不再提示重启

- 当前所有 Mod 写操作（上传、删除、Nexus 一键安装、远程 ZIP 安装）都要求服务器不在 running/starting。既然写入发生在停服状态，下次正常启动会直接加载新 Mod，因此不再设置 `modsRestartRequired`。
- `GET /api/instances/:id/mods` 只会在实例状态为 running/starting 且底层标记存在时返回 `restartRequired=true`；实例 stopped/ready_to_start 等停服状态永远返回 `false`，避免用户停服改完 Mod 后仍看到“需要重启”。
- 停止服务器流程会清理旧的 `modsRestartRequired` 标记；停服上传/删除/安装成功后也会清理历史残留。
- 验证：`go test ./internal/games/stardew_junimo ./internal/web` 覆盖停服上传返回 `restartRequired=false` 和 Nexus 安装不再置位。

# MODUPLOAD-2 多 ZIP 批量上传
- `POST /api/instances/:id/mods/upload` 现在支持一次上传多个 Mod ZIP：前端可重复提交 `mod` 字段，后端也兼容 `mods` 字段。总请求大小仍沿用 `maxModFormSize` 限制，不为每个文件单独放宽。
- Handler 会逐个把上传文件写入临时 ZIP，再统一交给 `stardew_junimo.UploadModZip(dataDir, tmpPath)` 校验、解压和导入；因此单个 ZIP 里包含多个顶层 SMAPI Mod 的既有能力仍然保留。
- 本次批量上传采用“批次失败回滚”语义：如果第 N 个 ZIP 写入或导入失败，会调用 `rollbackImportedMods` 逆序删除本次请求前面已经导入的 Mod 文件夹，并返回结构化错误。这样避免用户一次选择多个 ZIP 时只成功一半。
- 成功时返回 `registry.ModsListResult`，其中 `mods` 是本次所有 ZIP 导入出来的 Mod 汇总；`restartRequired` 继续遵循 `MODRESTART-1` 语义，停服上传返回 `false`，运行中历史标记只在实例 running/starting 时展示。
- 新增回归测试 `TestModUpload_AcceptsMultipleZipFiles`，覆盖同一个 multipart 请求中重复 `mod` 字段上传两个 ZIP 并返回两个导入结果。

# NEXUS-META-1 GraphQL 无 Key 元数据补全
- `SearchNexusMods(ctx, query, apiKey)` 现在在“纯数字 query 且未配置 Nexus API Key”时也会走 GraphQL v2 精确 ID 查询：`ModsFilter` 使用 `gameId=1303` + `modId=<id>`，不再把无 Key 数字搜索降级成关键词搜索。配置了 Key 时仍保留 v1 REST 精确 ID 查询。
- `GET /api/instances/:id/mods` 改为调用 `EnrichNexusMetadataForMods(ctx, dataDir, mods)`：先读取已有 `.local-container/control/nexus-mods.json`，再对缺失 sidecar 的本地 Mod 解析 `UpdateKeys: ["Nexus:<id>"]`，通过 Nexus GraphQL v2 拉取 `pictureUrl/summary/downloads/endorsements/updatedAt` 等展示元数据。
- GraphQL 补全是非阻断逻辑：Nexus 请求失败不会影响 Mod 列表，只会暂时保留本地图标；成功后写回 sidecar，后续列表请求直接走缓存，不需要 API Key，也不需要重启。
- Nexus API Key 仍只用于 v1 文件列表/下载链路等受限能力；普通搜索、数字 ID 展示元数据、手动上传后缩略图补全都不再依赖 Key。

# NEXUS-PAGED-1 搜索排序与分页
- 当前在线搜索入口是 `GET /api/instances/:id/mods/nexus/search`。关键词搜索通过 Nexus GraphQL v2 下推 `downloads DESC` 排序，并使用 `page/pageSize` 计算 `offset/count`。
- 空查询作为 Stardew Valley 默认热门列表处理；数字 ID 搜索仍支持无 Key GraphQL 精确查询和有 Key v1 REST 精确查询。
- 旧 `SearchMods` / `ModSearchResult` 统一搜索骨架已撤回，不再作为当前接口契约。

# MODZIP-1 XNB 替换包提示

- `UploadModZip` 仍只安装标准 SMAPI Mod（顶层 Mod 目录需要 `manifest.json`）。Nexus 上的老式 XNB 替换包虽然也是 `.zip`，但只包含 `Characters/*.xnb`、`Portraits/*.xnb` 等游戏内容替换文件，不能放进服务器 `Mods` 目录。
- 新增 XNB 替换包识别：如果 ZIP 内没有任何 SMAPI `manifest.json`，但包含 `.xnb` 且路径像 `Characters/`、`Portraits/`、`Content/`、`Maps/`、`TileSheets/`，后端返回明确错误：这是 XNB 替换包，不是 SMAPI Mod，不能上传到服务器 Mods 目录。
- Web 层 `sanitizeError` 对 XNB/SMAPI/manifest 这类产品级错误保留友好提示，不再统一压成“压缩包格式错误或已损坏”。
- 验证：`go test ./internal/games/stardew_junimo ./internal/web` 覆盖 XNB 替换包识别和错误脱敏。

# MODSEARCH-1 撤回记录

- `/api/instances/:id/mods/search` 与 `/api/instances/:id/mods/search/install` 已撤回，当前不注册路由。
- `backend/internal/games/stardew_junimo/mod_search.go` 和对应测试已删除；当前只保留 Nexus 专用搜索/安装接口，以及管理员粘贴 NXM/CDN ZIP 的远程安装兜底。
- 后续如果重新做 StardewModDataset、CurseForge、ModDrop、GitHub Release 等多来源搜索，应重新设计接口和排序/去重策略，不要假设当前仍有统一搜索后端契约。

# NEXUS-3 下载安装与已安装卡片元数据

- `SearchNexusMods(ctx, query, apiKey)` 已调整为：纯数字 query 在已配置 Nexus API Key 时走 v1 REST 精确 ID 查询；无 Key 时走 GraphQL v2 的 `gameId=1303 + modId` 精确元数据查询，避免展示型 ID 搜索依赖 Key。
- 新增 `POST /api/instances/:id/mods/nexus/install`，管理员专用且要求服务器不在 running/starting。接口请求体复用搜索结果卡片字段，后端创建 `mod_nexus_install` job，通过 Nexus v1 `files.json` + `download_link.json` 获取主文件下载链接，下载 ZIP 后交给既有 `UploadModZip` 校验/解压/导入，并设置 Mod restart-required 标记。
- 新增 `.local-container/control/nexus-mods.json` 面板自有元数据文件，按已安装 Mod 文件夹保存 Nexus 卡片信息（summary、pictureUrl、nexusUrl、downloads、endorsements、updatedAt 等）。`GET /api/instances/:id/mods` 和 sync plan 会调用 `ApplyNexusMetadataToMods`，因此前端可把服务器已安装 Mod 展示成与搜索结果相同的卡片。
- 相关文件：`backend/internal/games/stardew_junimo/nexus_install.go`、`nexus_metadata.go`、`nexus.go`、`mod_sync.go`、`backend/internal/games/registry/types.go`、`backend/internal/web/lifecycle_handlers.go`、`backend/internal/web/instance_handlers.go`。
- 验证：`go test ./...` 已覆盖无 Key 数字查询 GraphQL fallback、Nexus ZIP 下载安装、缩略图元数据保存与已安装列表回填。

# 后端文档

## 总体结构

后端是 Go 单体服务，负责面板鉴权、任务编排、Docker/Compose 控制、Stardew Junimo driver、SQLite 持久化、审计和静态前端托管。

建议边界如下：

```text
backend/cmd/panel              启动入口
backend/internal/auth          登录、密码、session
backend/internal/docker        Docker / Compose 封装与脱敏
backend/internal/jobs          长任务、日志、SSE
backend/internal/storage       SQLite、迁移、用户、状态、审计
backend/internal/static        嵌入 frontend/dist
backend/internal/web           HTTP handler、鉴权、路由
backend/internal/games         GameDriver 与游戏实现
backend/internal/games/stardew_junimo
```

Stardew 业务逻辑优先放在 `backend/internal/games/stardew_junimo`，不要散落到通用 handler 或 Docker 层。

## GameDriver 边界

`GameDriver` 表示“面板对某个游戏实例下达通用意图”的后端接口。Stardew 当前 driver ID 是 `stardew_junimo`，默认实例 ID 是 `stardew`。

driver 应负责：

- 实例目录准备与 compose 模板。
- 安装流程与 Steam Auth。
- 启动、停止、重启、状态校准。
- 邀请码读取。
- 存档、Mod、控制台命令、玩家信息。
- 与 JunimoServer 容器通信。

通用 web 层只做鉴权、参数解析、调用 driver、写审计、返回结构化错误。

## Junimo 通信优先级

1. 挂载文件：`.env`、`docker-compose.yml`、settings、saves、mods、backups、control JSON。
2. Docker Compose：`pull`、`up -d`、`down`、`restart`、`ps`、`logs`、`exec`、`run`。
3. Steam Auth TTY：扫码登录走 `steam-auth setup`，账号密码安装优先走 `steam-auth download`。
4. Junimo / SMAPI 控制：`attach-cli` 或当前 FIFO / output log 通信。
5. Junimo HTTP API：仅在启用并具备 API key 时用于状态或监控。
6. VNC：只做高级调试，不作为常规控制通道。

## 状态与任务

核心状态包括：

```text
uninitialized
admin_created
junimo_scaffolded
credentials_required
steam_auth_running
steam_auth_failed
steam_auth_done
game_installed
save_required
ready_to_start
starting
running
stopped
error
```

`jobs` 和 `job_logs` 用于安装、认证、启动等长任务。日志通过：

```text
GET /api/jobs/:id/stream
```

推送 `log`、`finished`、`ping` 事件。写日志前必须脱敏。

生命周期任务必须可取消。同一实例的 `stardew_lifecycle` 任务采用“最后操作生效”：新的启动、停止或重启请求会先取消该实例仍在 queued/running 的旧生命周期任务。被取消的任务状态必须落到 `canceled`，不能继续挂在 running，也不能在取消后把实例状态重新写回 running。

## 主要 API 分组

| 分组 | 代表接口 |
| --- | --- |
| 健康与版本 | `GET /health`, `GET /api/version`, `GET /api/health/diagnostics` |
| 认证 | `GET /api/auth/status`, `POST /api/auth/setup`, `POST /api/auth/login`, `POST /api/auth/logout`, `GET /api/auth/me` |
| 用户 | `/api/users` 管理员接口 |
| 实例状态 | `GET /api/instances/:id/state` |
| 安装 | `POST /api/instances/:id/prepare`, `GET /api/instances/:id/install-options`, `POST /api/instances/:id/install`, Steam Guard 输入接口 |
| 生命周期 | `POST /api/instances/:id/start`, `stop`, `restart` |
| 邀请码 | `GET /api/instances/:id/invite-code` |
| 存档 | `GET /api/instances/:id/saves`, 上传预览、提交启动、选择、删除、导出、备份恢复 |
| Mods | `GET /api/instances/:id/mods`, 上传、删除、导出、玩家同步分类(`mods/sync-plan`、`mods/:modId/sync-classification`、`mods/sync-pack/export`)、Nexus 只读搜索(`mods/nexus/search`) |
| 命令 | `GET /api/instances/:id/commands`, `POST /commands/run`, `POST /commands/say` |
| 玩家 | `GET /api/instances/:id/players` |
| Docker 诊断 | compose ps/logs/status 相关调试接口 |
| 审计与支持包 | `GET /api/audit-logs`, `POST /api/instances/:id/support-bundle` |

新增接口时必须返回结构化错误码，前端通过错误码映射中文提示。

## Stardew 关键实现

### Steam Auth

- 官方服务名保留 `steam-auth`、`server`、`discord-bot`。
- 官方镜像版本变量是 `IMAGE_VERSION`。
- `steam-auth` sidecar 当前默认先使用 `docker.1ms.run/anxiyizhi/junimo-steam-service-cn:1.5.0-anxi.2`，第二候选使用阿里云 ACR 新版个人版镜像 `crpi-9z3bkb9g7fxeohrg.cn-hangzhou.personal.cr.aliyuncs.com/anxi-panel/junimo-steam-service-cn:1.5.0-anxi.2`，再保留 DaoCloud、GHCR、Docker Hub 兜底。
- `.env` 会写入 Steam 连接等待和认证重试相关变量。
- 不要用普通 stdin 重定向跑 `steam-auth setup` 的账号密码分支；该分支会用 `Console.ReadKey()` 读密码，后台 pipe 会失败。

### 邀请码

Junimo 会把邀请码写入容器内 `/tmp/invite-code.txt`。`docker compose restart` 可能保留旧文件，因此重启前后要清理或过滤旧码，前端启动/重启后也要等待非旧码的新邀请码。

Stardew 生命周期里的普通“重启服务器”必须只重启 Compose 的 `server` 服务，不要无条件重启 `steam-auth`。`steam-auth` 重启会重新登录 Steam，短时间多次尝试可能触发 `RateLimitExceeded`。例外：如果已有 `STEAM_AUTH_COMPLETED=true` 且 server 日志显示 `Steam-auth service not ready` / 请求失败，后端会 best-effort 自动刷新一次 `steam-auth`，用于让常驻服务重新读取已写入的 session。

### 控制台与喊话

- `POST /api/instances/:id/commands/run` 只执行 allowlist 中的固定命令，继续通过 Junimo FIFO `/tmp/smapi-input` 与 `/tmp/server-output.log` 读取结果，不允许任意 shell。
- `POST /api/instances/:id/commands/say` 不依赖上游 SMAPI `say` 命令；后端校验运行状态和 200 字符限制后，把 `broadcast` JSON 写入 `.local-container/control/commands/`。
- `StardewAnxiPanel.Control` 在 `UpdateTicked` 中消费 `broadcast` 命令，调用 Stardew multiplayer chat message 发送 `[Panel] <message>` 给所有玩家。消息会移除控制字符并按 CJK/Kana/Hangul/Cyrillic/Thai 粗略选择聊天字体语言码，避免中文公告显示为方块。
- 当前已验证 Junimo 镜像 `sdvd/server:1.5.0-preview.121` 也带上游 `/ws` + `chat_send` 聊天 relay；面板暂不直接依赖该 WebSocket，以避免面板容器网络和 API key 配置差异影响喊话。
- 更新了嵌入控制模组 DLL。已运行实例需要重启/重新准备 `server` 容器后，`installSMAPIMod` 才会把新 DLL 写入实例 Mods 目录。

### 玩家信息

当前通过 SMAPI 控制模组写 `.local-container/control/players.json`，后端合并 `players-cache.json` 输出玩家名册。前端 5 秒轮询，成本低且稳定。

收入口径由 SMAPI mod 写入并保持固定含义：`farmIncome` 始终表示农场/团队累计收入，`personalIncome` 始终表示玩家 `individualMoneyEarned` 个人累计收入；`totalMoneyEarned` 仅保留为旧字段兼容。修改嵌入 DLL 后必须同步 `embedded/smapi-mod` 和已部署实例，并重启 Stardew server 容器才会生效。

最近事件由后端根据当前在线快照和 `players-cache.json` 中上一轮状态差分生成，并写入 `.local-container/control/players-events.json`。当前支持首次记录、加入和离开事件，最多保留最近 50 条，并按 `saveId` 隔离，避免切换存档后显示旧存档活动。

`players-cache.json` 必须按存档隔离：缓存文件带 `saveId`，只有 `cache.saveId == players.json.saveId` 时才合并历史离线玩家。切换/新建存档后，旧存档名册不能混入新存档玩家列表；旧版无 `saveId` 的缓存遇到当前存档 ID 时应被忽略并在下一次读取后重写。

`maxPlayers` 兜底（PLAYERS-MAXPLAYERS-1）：`ListPlayers` 现在默认从当前存档的 `server-settings.json` 读取 `Server.MaxPlayers`（`readServerMaxPlayers`，文件缺失/解析失败/非正数时返回 nil），junimo info 解析出的上限仍会覆盖兜底值。这样服务器未运行或 info 输出不含上限时，`GET /api/instances/:id/players` 也能返回 `maxPlayers`，前端右栏/总览可显示"在线数/人数上限"。测试见 `players_test.go` 的 `TestReadServerMaxPlayers`。

### Mod 玩家同步包

`stardew_junimo` 提供 Mod 同步分类能力，逻辑全部在 `backend/internal/games/stardew_junimo/mod_sync.go`，不绕过 driver。

- `ModInfo` 新增 `syncKind`(`server_only` | `client_required` | `unknown`) 和可选 `syncNote`。`GET /api/instances/:id/mods` 在返回前会调用 `ApplyModSyncClassification` 补全这两个字段，前端不需要单独再拉一次分类。
- 分类持久化在面板自有文件 `.local-container/control/mod-sync.json`，绝不写入 Mod 自身的 `manifest.json`。没有手动覆盖时会自动识别：面板控制 Mod 默认 `server_only`；SMAPI `ContentPackFor` 内容包和常见 `[CP]`/`[AT]`/`[JA]` 等内容包前缀默认 `client_required`；其他第三方 Mod 为了联机安全也默认 `client_required`。用户可再手动改成 `server_only` 或 `unknown`。
- `GET /api/instances/:id/mods/sync-plan`：返回全部已装 Mod 及分类统计（`serverOnly`/`clientRequired`/`unknown`）。
- `PUT /api/instances/:id/mods/:modId/sync-classification`：登录用户可用，只写面板元数据，不受服务器运行状态限制。`:modId` 可以是文件夹名或 `UniqueID`（复用 `ResolveModFolder`，同 `DeleteMod` 的查找顺序）。
- `POST /api/instances/:id/mods/sync-pack/export`：导出全部 `syncKind == client_required` 的 Mod 为玩家同步安装包 ZIP，附带面板生成的 `pack-manifest.json`、`checksums.sha256`、安装/卸载脚本和 `payload/mods/`。服务器运行中也允许导出。`StardewAnxiPanel.Control` 无论分类如何，始终被排除在导出之外（双重保险：默认分类 + 导出时强制跳过）。没有任何 Mod 命中 `client_required` 时返回 `400 no_sync_mods`。

涉及：`backend/internal/games/registry/types.go`、`backend/internal/games/stardew_junimo/mod_sync.go`、`backend/internal/games/stardew_junimo/mods.go`（抽出共用的 `addModDirToZip`）、`backend/internal/web/lifecycle_handlers.go`、`backend/internal/web/instance_handlers.go`。

测试覆盖：分类文件读写、自动默认 client_required/server_only、导出只含 client_required、导出排除控制 Mod、`ResolveModFolder` 路径安全，见 `backend/internal/games/stardew_junimo/mod_sync_test.go`。

`mods.go` 的 `ExportModsZip` 原本在循环失败但 `w.Close()` 成功时会因 `if err := w.Close(); err != nil` 遮蔽外层 `err`，把失败误判为成功；已修复为循环内失败直接 `return`、`Close()` 结果直接赋值给外层 `err`，与 `ExportModSyncPackZip` 写法一致。

`ExportModSyncPackZip` 原本固定写入 `%TEMP%\stardew-player-sync-pack.zip`，两个并发导出请求会互相覆盖/截断对方的 ZIP，且一个请求失败后的 `defer` 清理可能删掉另一个请求正在 `ServeFile` 的文件；已改为 `os.CreateTemp("", "stardew-player-sync-pack-*.zip")` 生成唯一临时路径，对外 `Content-Disposition` 文件名固定用新增的 `PlayerSyncPackFileName` 常量，与实际磁盘路径解耦。

`SetModSyncClassification` 原本对 `mod-sync.json` 是无锁的 load-modify-save，两个几乎同时的分类更新请求会让后写入的覆盖先写入的；已加上按 `dataDir` 维度的 `sync.Mutex`（`modSyncLockFor`）包住整个读改写流程，并把 `saveModSyncStore` 改成临时文件 + `os.Rename` 的原子写入，避免中途崩溃留下半截 JSON。

### Nexus Mods 只读搜索（第二阶段）

`backend/internal/games/stardew_junimo/nexus.go` 提供 Stardew Valley（`stardewvalley` game domain）只读搜索，不做下载/安装。Nexus 个人 API Key 不再从环境变量读取，而是由管理员在前端配置后写入 SQLite `panel_settings`（key=`nexus_api_key`），当前请求即时读取，保存后无需重启后端即可生效。

- **鉴权按路径拆开，不是统一要求 Key**：
  - 查询关键词若是纯数字，配置 Nexus API Key 时按官方文档化的 v1 REST 接口 `GET https://api.nexusmods.com/v1/games/stardewvalley/mods/{id}.json` 精确查询；未配置 Key 时改走 GraphQL v2 `gameId=1303 + modId` 精确元数据查询。Key 不再是展示型 ID 搜索的前置条件。
  - 其余关键词走 GraphQL v2（`https://api.nexusmods.com/v2/graphql`，与 nexusmods.com 网站搜索框同源）做关键词搜索。这是**公开只读查询，不要求个人 API Key**——即使未配置 Key 也会正常发起请求（`setNexusHeaders` 在 Key 为空时直接不带 `apikey` 头，而不是发一个空头）；如果配置了 Key 会顺带带上（无害，可能有助于提升速率限制），但从不作为前置门槛。如果 Nexus 对这条公开查询本身返回认证类拒绝（HTTP 401/403，或 GraphQL `errors` 数组里出现 auth/forbidden/permission 相关字样），统一映射为哨兵错误 `ErrNexusAuthRequired`（不是 `ErrNexusAPIKeyMissing`，因为配置个人 Key 也未必能解决——这通常意味着需要 OAuth 或更高权限）。
  - **注意**：Nexus 官方 v1 REST API 文档中没有任何关键词全文搜索接口（只有按 ID 查询、最近更新列表、MD5 查询）。GraphQL v2 这条路径的查询结构（`nexusGraphQLSearchQuery`，见 `nexus.go`）**已用真实 schema introspection 和真实搜索请求验证过**（直接对 `https://api.nexusmods.com/v2/graphql` 发 `{ __type(name: "...") { ... } }` 内省查询确认的）：`mods` 根字段本身不接受 `gameDomain` 参数，游戏域名和关键词都要放进 `filter`（`ModsFilter`）变量里——`gameDomainName: [{value: "stardewvalley", op: EQUALS}]` 限定游戏，`name: [{value: <关键词>, op: WILDCARD}]` 做子串搜索（`WILDCARD` 不需要在值里手动加 `*`，会自动做子串匹配，比如关键词 `tractor` 能匹配到标题里包含 `Tractor` 的所有 Mod）。早期实现猜测过 `mods(gameDomain: ..., filter: ..., count: ...)` 这种顶层参数形式，被 Nexus 用 GraphQL `errors` 数组拒绝（`argumentNotAccepted`），导致每次关键词搜索都失败并兜底成 `nexus_request_failed`，已修复。
  - 结果数量上限 `nexusMaxResults = 20`。
  - `WILDCARD` 子串匹配只对得上 Mod 标题里实际出现的字符串：纯中文关键词（比如"拖拉机"）如果没有任何 Mod 标题恰好包含这几个汉字，会合法地返回 0 条结果（不是错误），因为 Nexus 上的 Mod 标题绝大多数是英文/其他语言；建议优先用英文关键词（如 `tractor`）或者数字 ID 搜索。
- **请求安全**：固定 10 秒超时（`nexusRequestTimeout`）、固定 `User-Agent`；非 2xx 响应一律不读取/转发响应体，只保留状态码包成 `*NexusAPIError`，避免上游错误页（可能回显请求细节）泄露给前端；API Key 只通过请求头发送，从不出现在 URL 或错误信息里，未配置时也不会发送空请求头。
- **query 校验**：空查询（trim 后）现在作为默认热门列表处理，返回 Nexus Stardew Valley 列表第一页；`ErrInvalidNexusQuery` 仅保留给安装/下载等缺少有效 Mod ID 的请求使用。
- **已安装匹配**：`ApplyNexusInstalledMatch(dataDir, results)` 读取本地已装 Mod，按 manifest `UpdateKeys` 中 `Nexus:<id>` 解析出的 `NexusModID` 做匹配，命中则把 `installed`/`installedFolderName`/`installedVersion` 填上；本阶段只判断"已安装"，不做版本新旧比较。
- **manifest 解析扩展**：`modManifest`/`registry.ModInfo` 新增 `UpdateKeys []string` 和 `NexusModID int`（由 `parseNexusModIDFromUpdateKeys` 从 `UpdateKeys` 里挑出 `Nexus:` 前缀的条目解析），`readModInfo` 在解析时一并填充，所以 `GET .../mods` 现有列表也会带上这两个新字段（向后兼容，新字段都是 `omitempty`）。

API：`GET /api/instances/:id/mods/nexus/search?q=关键词`，`requireAuth`（任意登录用户，普通玩家也能用，不需要管理员权限）。错误码映射：

| 场景 | HTTP | code |
| --- | --- | --- |
| 空查询 | 200 | 默认 Nexus 热门列表 |
| Nexus 下载/安装但面板未配置 Nexus API Key | 503 | `nexus_api_key_missing` |
| 关键词搜索被 Nexus 拒绝（需要 OAuth/更高权限） | 502 | `nexus_auth_required` |
| Nexus 返回 404（按 ID 查询未命中） | 404 | `nexus_mod_not_found` |
| Nexus 返回 401/403（ID 查询路径，Key 无效/权限不足） | 502 | `nexus_unauthorized` |
| Nexus 返回 429 | 429 | `nexus_rate_limited` |
| 其他非 2xx / 网络错误 | 502 | `nexus_request_failed` |

注意 `nexus_unauthorized` 只会从 ID 查询路径（v1 REST 直接返回 401/403）触发；关键词路径的 401/403 在 `nexusSearchByKeyword` 内部就已经转换成 `ErrNexusAuthRequired`，到 handler 时已经不是 `*NexusAPIError` 了，所以两个 code 不会互相覆盖。

### Nexus API Key 面板设置

管理员通过面板配置 Nexus Key，不再依赖环境变量：

- `GET /api/settings/nexus`：管理员专用，返回 `{ "configured": boolean, "last4"?: string }`，只暴露末 4 位。
- `PUT /api/settings/nexus/api-key`：管理员专用，请求体 `{ "apiKey": string }`；trim 后写入 SQLite `panel_settings`，立即对后续搜索请求生效，不需要重启。
- `DELETE /api/settings/nexus/api-key`：管理员专用，删除 `panel_settings.nexus_api_key`。

保存和删除都会写审计日志（`nexus_api_key_update` / `nexus_api_key_delete`），日志 metadata 不包含 Key 本体。`SearchNexusMods` 本身不访问存储，HTTP handler 负责读取 `panel_settings` 后以参数传入，避免把面板配置逻辑混进 `stardew_junimo` 客户端。

涉及文件：`backend/internal/games/registry/types.go`（`ModInfo` 新增字段）、`backend/internal/games/stardew_junimo/mods.go`（manifest 解析扩展）、`backend/internal/games/stardew_junimo/nexus.go`（Nexus 客户端）、`backend/internal/web/instance_handlers.go`（实例路由）、`backend/internal/web/lifecycle_handlers.go`（`handleModNexusSearch`/`writeNexusError`）、`backend/internal/web/settings_handlers.go`（Key 配置接口）、`backend/internal/storage/settings.go`（`panel_settings` 读写）。

测试覆盖（`backend/internal/games/stardew_junimo/nexus_test.go`）：无 Key 数字 ID 走 GraphQL v2 `gameId + modId` 且不带 `apikey` 头、关键词路径在无 Key 时仍能正常工作、空 query、ID 查询结果解析、关键词搜索结果解析、结果数量上限裁剪、非 2xx 状态码映射、GraphQL 鉴权失败映射、API Key 不泄露、`UpdateKeys`/`NexusModID` 解析与 `installed` 匹配，以及本地已安装 Mod 元数据补全和 sidecar 缓存。

### 单人菜单暂停

`StardewAnxiPanel.Control` SMAPI mod 支持真人玩家菜单暂停。当前逻辑仍在 Junimo 服务端读取远端 `Farmer.hasMenuOpen || Farmer.requestingTimePause` 来触发背包/菜单暂停；实测服务端能通过该字段看到背包打开。AUTOPAUSE-7 的规则是：只有 1 个真人玩家时，该玩家打开菜单即暂停；有 2 个及以上真人玩家时，必须所有真人玩家都打开菜单才暂停，避免单个玩家翻背包影响其他正在行动的人。时钟控制沿用 AUTOPAUSE-6 参考 `Pause Time in Multiplayer` 的 1.6 分支做法，不再靠 `IsTimePaused/pauseTime` 维持暂停，而是在 `UpdateTicking` 中保存 `Game1.gameTimeInterval` 并写入 `-100` 哨兵冻结时钟；关闭背包后发现哨兵则恢复保存的 interval，同时清理旧版本可能残留的 `Game1.netWorldState.Value.IsTimePaused`、`Game1.netWorldState.Value.IsPaused`、`Game1.isTimePaused` 和 `Game1.pauseTime`。如果暂停 tick 中抛异常，mod 会先释放时钟，避免再次永久卡住。

## 安全要求

- 控制台命令必须 allowlist，普通用户和管理员可见命令不同。
- 存档 ZIP 解压必须防 zip-slip、绝对路径和符号链接。
- Mod 上传必须限制文件类型、大小和运行中危险操作。
- 错误响应不暴露内部路径、堆栈、token、密码。
- 支持包不包含完整数据库、完整存档、完整 Mod 或 Steam session。
- Docker Socket 权限风险必须在部署和 UI 中提示。

## 后端验证

常用命令：

```powershell
cd E:\stardew-server-anxi-panel\backend
$env:GOCACHE='E:\stardew-server-anxi-panel\.gocache'
go test ./...
```

SMAPI mod 编译：

```powershell
docker run --rm `
  -v "E:\stardew-server-anxi-panel\backend\internal\games\stardew_junimo\embedded\smapi-mod-src:/src" `
  -v "E:\stardew-anxi-panel\runtime\game:/game" `
  -w /src mcr.microsoft.com/dotnet/sdk:6.0 `
  dotnet build -c Release /p:GamePath=/game
```

改动后如果影响嵌入 DLL，必须说明是否需要重启 Stardew server 容器。
# REMOTE-MOD-1 NXM / 直链远程安装
- 新增 `POST /api/instances/:id/mods/remote/install`，管理员专用且要求服务器不在 running/starting。请求体 `{ "url": string, "mod"?: Nexus 卡片字段 }`，后端创建 `mod_remote_install` job。
- `url` 支持两类：`nxm://stardewvalley/mods/{modId}/files/{fileId}?key=...&expires=...`，以及 `https://.../*.zip` 远程压缩包直链。NXM 路径会使用 SQLite 中保存的 Nexus API Key + 链接里的 `key/expires` 调 v1 `download_link.json`，用于非 Premium 用户的慢速下载授权；直链路径会直接下载 ZIP，来源可以是 Nexus CDN、ModDrop、GitHub、CurseForge 等公网 HTTPS ZIP。
- 远程下载完成后统一复用现有 `UploadModZip` 安全校验/解压/导入，并设置 restart-required。当前只承诺 ZIP；7z/rar 需要后续引入解压器和同等安全校验后再开放。
- 为避免泄露临时授权参数，Nexus/API/CDN 网络错误不会把完整 URL 写入 job error；审计日志只记录 jobId，不记录粘贴的 URL。
- 新增 `ParseNexusNXMURL`、`InstallNexusModWithTicket`、`InstallRemoteMod`，测试覆盖 NXM 解析、`key/expires` 透传、ZIP 下载导入。
# SMAPI-RUNTIME-1 Mod 列表置顶 SMAPI 运行组件

- `GET /api/instances/:id/mods` 现在会在检测到面板内置控制 Mod `StardewAnxiPanel.Control` 已安装时，在 `mods[]` 第一位注入一个虚拟内置条目：`SMAPI`（`uniqueId=Pathoschild.SMAPI`、`builtIn=true`、`syncKind=client_required`）。
- 该条目表示 Stardew Valley 的 SMAPI 加载器/运行组件，用于提醒玩家客户端也需要先安装 SMAPI；它不是 `.local-container/mods` 下的真实目录，不参与删除、上传覆盖或 ZIP 导出。
- `FindModByUniqueID` 会跳过 `builtIn` 条目，避免把虚拟 SMAPI 当作可删除的磁盘 Mod；玩家同步包导出也会跳过 `builtIn` 条目，避免尝试打包不存在的运行组件。
- `ModInfo` 新增 `builtIn?: boolean` 字段；前端可据此禁用危险操作并显示“内置/置顶/不打包进同步包”等状态。
- 验证：`go test ./internal/games/stardew_junimo ./internal/web` 覆盖 SMAPI 置顶注入和同步包不导出内置 SMAPI。

# MODORIGIN-1 Nexus 包来源元数据

- `ModInfo` 新增 `originSource/originNexusModId/originModName/originModUrl`，用于表达“这个文件夹本身没有 Nexus UpdateKey，但它是随某个 Nexus 包安装的内容包”。
- `UploadModZip` 在同一个 ZIP 导入出多个 Mod 时，会优先选择带 `Nexus:<id>` 的主 Mod 作为包来源，并通过 `.local-container/control/nexus-mods.json` 保存到所有同包导入的 Mod。典型例子：`Multiple Construction Orders` 自身显示 `nexusModId=47289`，`[CP] Multiple Construction Orders` 显示 `originSource=nexus/originNexusModId=47289/originModName=Multiple Construction Orders`，但自己的 `nexusModId` 仍为空。
- `ApplyNexusMetadataToMods` 不再把 sidecar 里的 `modId` 写回没有 `UpdateKeys` 的内容包 `nexusModId`；它只填 `origin*` 字段，并复用来源包的 `pictureUrl/downloadCount/endorsementCount/updatedAt` 作为卡片展示元数据。
- 后续 GraphQL 补齐主 Mod 缩略图时，`SaveInstalledNexusMetadata` 会合并并同步更新 sidecar 中同一个 Nexus ID 的内容包记录，确保内容包也能显示来源包缩略图。
- `DeleteMod` 现在会把同一个 Nexus 包来源的真实 Mod 文件夹视为捆绑组：删除主 Mod 或 `[CP]` 内容包任意一个，都会一并删除同 `nexusModId/originNexusModId` 的组成员，并清理 sidecar 里的 Nexus 元数据记录。
- 验证：`go test ./internal/games/stardew_junimo` 覆盖 Nexus 外壳 ZIP 内主 Mod + `[CP]` 内容包的来源字段、持久化、缩略图继承和捆绑删除。
# NEXUS-PAGED-1 Nexus 搜索排序与分页

- `GET /api/instances/:id/mods/nexus/search` 继续作为模组页唯一在线搜索入口；前端不再走 `/mods/search` 统一搜索骨架。
- 查询参数新增 `page`、`pageSize`（默认 `1/20`，后端最大页大小 50）。响应 `NexusModSearchResponse` 新增 `page/pageSize/total/hasMore`。
- Nexus GraphQL v2 关键词搜索会把排序下推到 Nexus：`sort: [{ downloads: { direction: DESC } }]`，并传 `offset/count`。这样热门官方 Mod 会在全量匹配结果里按下载量排序，而不是只在默认前 20 条里本地排序。
- 数字 ID 搜索仍保持原分流：有 Nexus API Key 走 v1 REST 精确查询，无 Key 走 GraphQL v2 `gameId + modId` 精确查询。
- 涉及文件：`backend/internal/games/stardew_junimo/nexus.go`、`backend/internal/web/lifecycle_handlers.go`、`backend/internal/games/stardew_junimo/nexus_test.go`。
- 验证：`go test ./internal/games/stardew_junimo ./internal/web`。
- 统一搜索骨架已移除：`/api/instances/:id/mods/search`、`/api/instances/:id/mods/search/install` 路由不再注册，`mod_search.go` / `mod_search_test.go` 已删除。

# SMAPI-SYNC-2 SMAPI 与面板控制 Mod 同步语义

- `ListMods` 在检测到 `StardewAnxiPanel.Control` 时仍注入虚拟 `SMAPI` 条目；该条目保持 `builtIn=true`、`uniqueId=Pathoschild.SMAPI`、`syncKind=client_required`，现在会进入玩家同步清单，用于提醒玩家客户端必须先安装 SMAPI。
- `StardewAnxiPanel.Control` 真实目录会被识别为 `builtIn=true`、`syncKind=server_only`。后端删除入口会拒绝删除该内置控制 Mod，避免通过 folderName 绕过前端按钮保护。
- 玩家同步 ZIP 导出会跳过 `StardewAnxiPanel.Control`，不管它的持久化分类是否被写成 `client_required`。SMAPI 写入 `pack-manifest.json` 的 `mods[]` 与 `smapi` 元数据；只有服务端已缓存 SMAPI 官方 ZIP 时才会额外随包写入 `payload/smapi/SMAPI*.zip`。
- `pack-manifest.json` 的每个 Mod 条目带 `builtIn` 与 `packaged` 字段：SMAPI 为 `builtIn=true, packaged=false`，普通玩家需同步 Mod 为 `packaged=true`；玩家安装脚本只复制 `packaged=true` 的 Mod。
- 验证：`go test ./internal/games/stardew_junimo ./internal/web`。

# PLAYERSYNC-PACK-11 ASCII 动态进度条

- 玩家同步包 `tools/install.ps1` 恢复动态进度条，但终端动态行只使用 ASCII：`[====    ]  42% CHECK/MODS/SMAPI/...`。
- `Show-InstallProgress` 继续把中文详细状态写入日志；控制台渲染由 `Render-InstallProgressLine` 根据百分比映射为英文阶段名，不把中文 `$Status` 放进原地刷新行。
- `Write-Step` 打印中文任务日志前会 `Clear-InstallProgressLine`，打印后再 `Redraw-InstallProgressLine`；调用 Steam 启动项辅助脚本、人工粘贴游戏目录、失败恢复提示前也会先清理进度行，避免中文提示和动态条粘在同一行。
- 仍禁用 PowerShell `Write-Progress`，避免 Windows Terminal 进度流重绘造成中文双字。当前测试解压包已热修，并在真实游戏目录验证普通中文任务行不再重叠。
# PLAYERSYNC-PACK-12 日志写入非致命化

- 玩家同步包 `tools/install.ps1` 新增 `Write-LogLine`，替代安装日志里的直接 `Add-Content`。
- 日志写入会短重试 5 次；如果日志文件被 Windows Terminal、杀软或其他短暂句柄占用，最终失败也只跳过该条日志，不再中断安装。
- 修复真实安装时 `install-*.log` 被占用导致 SMAPI 阶段失败的问题；安装主流程不再依赖日志文件可写。
- 当前测试解压包已热修，并在 `D:\steam\steamapps\common\Stardew Valley` 真实安装验证通过。
# PLAYERSYNC-PACK-13 启动项高亮

- 玩家同步包终端输出新增颜色提示：`Steam 启动项文本` / `建议 Steam 启动项` 标题用 Yellow，真正需要复制的启动项整行用 Cyan。
- 启动项文本本身仍单独占一行，只包含 `"<gameDir>\StardewModdingAPI.exe" %command%`，方便玩家直接选中复制。
- 当前测试解压包的 `tools/install.ps1` 与 `tools/steam-launch-options.ps1` 已热修，并真实安装验证输出流程正常。
# PLAYERSYNC-PACK-14 启动项复制提示

- 玩家同步包终端输出在 `Steam 启动项文本：` 和 `建议 Steam 启动项：` 后新增提示：`请复制到 Steam 的游戏启动项中。`

# PLAYERSYNC-PACK-15 模组更新包

- 新增轻量模组更新包导出：`POST /api/instances/:id/mods/sync-pack/export-update`，下载文件名为 `stardew-player-mods-update-pack.zip`。
- 新增 `ExportModSyncUpdatePackZip(dataDir)`，复用玩家同步包导出核心，但 `pack-manifest.json.packType=mods_update`，不写入 `payload/smapi/`、不写入 SMAPI ZIP，也不在 `checksums.sha256` 记录 SMAPI。
- 更新包只允许存在真实可打包的 `client_required` Mod 时导出；只有虚拟 SMAPI 前置项时返回 `400 no_sync_mods`。
- 更新包根目录使用 `安装模组更新.bat`、`卸载本次模组更新.bat` 和专属 README。安装脚本识别 `mods_update` 后会先确认 `<Stardew Valley>\StardewModdingAPI.exe` 已存在；缺失时停止并提示先运行完整版玩家同步包或手动安装 SMAPI。
- 完整同步包接口不变：`POST /api/instances/:id/mods/sync-pack/export` 仍返回 `stardew-player-sync-pack.zip`，继续按服务端缓存情况随包携带 SMAPI。
- 验证：`go test ./internal/games/stardew_junimo ./internal/web`、`npm.cmd run build`。
- 提示行使用 Yellow；真正可复制的 launch options 仍保持独立 Cyan 行，避免玩家复制到说明文字。
- 当前测试解压包的 `tools/install.ps1` 与 `tools/steam-launch-options.ps1` 已热修，并真实安装验证输出顺序正常。

# PLAYERSYNC-PACK-16 模组更新包跳过 Steam 启动项

- `packType=mods_update` 的玩家端安装脚本不再调用 `tools/steam-launch-options.ps1`，更新包 ZIP 也不再包含该辅助脚本；因此不会尝试读取或写入 Steam `localconfig.vdf`。
- 更新包仍会检查 `<Stardew Valley>\StardewModdingAPI.exe` 是否存在；存在后只安装/跳过/更新 Mod，Steam 启动项沿用完整版同步包或玩家已有设置。
- 更新包最终摘要显示 `Steam 启动项：已跳过，沿用已有设置`，不再打印可复制启动项文本，避免玩家误以为本次更新包还需要配置 Steam。
- 完整同步包行为不变：首次玩家使用 `POST /api/instances/:id/mods/sync-pack/export` 导出的完整包，仍会尽力配置 Steam 启动项或输出可复制文本。
- 验证：`go test ./internal/games/stardew_junimo ./internal/web`。

# MODPROFILE-1 按存档启用/禁用 Mod

- 新增 `.local-container/mods-disabled/` 作为禁用 Mod 的物理目录；`.local-container/mods/` 仍是容器启动时挂载到 `/data/Mods` 的启用目录。
- 新增 `.local-container/control/mod-profiles.json`，按 `saveName` 保存每个存档的 Mod 启用状态。新建/新导入存档的 profile 使用 `defaultEnabled=false`，即除内置组件外默认不启用任何第三方 Mod。
- `GET /api/instances/:id/mods` 现在通过 `ListModsWithState(dataDir, activeSaveName)` 返回启用与禁用目录的合并列表，并在 `ModInfo` 上返回 `enabled/canToggle/enableNote`。
- 新增 `PUT /api/instances/:id/mods/:modId/enabled`，管理员专用且要求服务器不在 running/starting。请求体 `{ "enabled": boolean, "saveName"?: string }`；未传 `saveName` 时使用当前激活存档。
- 启动链路在 `docker compose up` 前应用当前存档 profile；新建存档启动前先执行 `ApplyNewSaveDefaultModState`，把非内置 Mod 全部移动到禁用目录，待 Junimo 写出真实存档名后再持久化 disabled profile。
- 上传存档确认并启动时会立即为该导入存档写入默认禁用 profile 并应用，避免新接入面板的存档继承服务器已有 Mod。
- 删除与重复 UniqueID 检查改为同时扫描启用与禁用目录，避免禁用后无法删除或重复上传误判。
- 验证：`go test ./internal/games/stardew_junimo ./internal/web`。
# MODPROFILE-2 切换存档刷新 Mod 状态

- 后端原有 `handleSaveSelect` / `handleSaveSelectAndStart` 已在 `SetActiveSave` 后调用 `ApplyModProfile`；本次补充测试确保两个存档 profile 之间切换时，真实 Mod 文件夹会在 `.local-container/mods/` 与 `.local-container/mods-disabled/` 间移动到正确位置。
- 新增测试覆盖：`TestApplyModProfileSwitchesPhysicalStateBetweenSaves`，验证 SaveA 启用 ModA/禁用 ModB、SaveB 禁用 ModA/启用 ModB 的来回切换。
- 涉及文件：`backend/internal/games/stardew_junimo/mod_profiles_test.go`。
- 验证：`go test ./internal/games/stardew_junimo ./internal/web`。

# NEXUS-DEFAULT-1 下载页默认热门 Mod

- `GET /api/instances/:id/mods/nexus/search` 现在允许 `q` 为空；空查询不再返回 `400 invalid_query`，而是调用 Nexus GraphQL v2 的 Stardew Valley 列表查询。
- 默认列表只带 `gameDomainName=stardewvalley` 过滤，并按 `downloads DESC` 排序，继续使用 `page/pageSize/total/hasMore` 分页；前端默认请求第一页 20 条。
- 关键词搜索和数字 ID 搜索路径不变：关键词继续带 `name WILDCARD`，数字 ID 有 Key 时走 v1 REST，无 Key 时走 GraphQL 精确 ID。
- 涉及文件：`backend/internal/games/stardew_junimo/nexus.go`、`backend/internal/web/lifecycle_handlers.go`、`backend/internal/games/stardew_junimo/nexus_test.go`。
- 验证：`go test ./internal/games/stardew_junimo ./internal/web`。
# NEWGAME-CABINS-1 自定义新存档初始联机小屋

- `NewGameConfig.startingCabins` 后端契约从 `0-3` 对齐到 Stardew 1.6/控制模组使用的 `0-7`；`WriteServerSettings` 继续写入 Junimo `Game.StartingCabins` 和 `Game.CabinLayoutNearby`。
- `WriteInitConfig` 现在始终为自定义新建存档写 `.local-container/control/server-init.json`，即使请求没有角色外观字段，也会把 `cabinCount/cabinLayout` 交给 `StardewAnxiPanel.Control`。
- `WriteServerSettings` 会写 `.local-container/control/new-game-pending` 一次性标记；选择既有存档时 `SetActiveSave` 会清除该标记。
- SMAPI 控制模组不再执行 `Game1.game1.loadForNewGame(false)` 这条历史原生创建路径；真实创建统一由 Junimo HTTP `POST /newgame` 完成。
- 控制模组仅当 `new-game-pending` 存在时提前同步 `Game1.startingCabins`、`Game1.cabinsSeparate`、农场类型、利润率和钱包模式；存档创建/加载后会删除标记，避免后续启动既有存档时误触发新建参数。
- `server-init.json.mode` 新写为 `panel-newgame`；控制模组只为兼容旧文件保留读取旧 `native-create` 字符串，不再包含对应的创建执行逻辑。
- 已重编嵌入 DLL：`backend/internal/games/stardew_junimo/embedded/smapi-mod/StardewAnxiPanel.Control.dll`。影响已运行实例时，需要重启/重新准备服务端容器让内置控制模组刷新到实例目录。
- 验证：`go test ./internal/games/stardew_junimo -run "WriteServerSettings|WriteInitConfig|ValidateNewGameConfig"`、`go test ./internal/games/stardew_junimo ./internal/web` 通过；SMAPI mod 通过 Docker + `/p:GamePath=/game` 编译通过。


# SAVE-BACKUP-POLICY-1 存档自动备份策略

> 说明：本节原始记录在历史保存中损坏为不可逆的 `?` 占位符乱码（区别于 `MOJIBAKE-FIX-1` 那种可逆编码错位，本次已无法还原原文），以下依据当前代码重新整理。

- 新增可配置的存档自动备份策略 `BackupPolicy`，持久化在 `.local-container/backups/saves/policy.json`：`gameSaveBackups`（游戏保存后自动更新最新备份，默认开）、`dailySnapshots`（每日快照，默认开）、`dailyRetentionDays`（每日快照保留天数，默认 3，上限 14）、`scheduledBackups`（定时备份开关，默认关）。定时触发频率最初用小时间隔表示，字段现仍以 `scheduledIntervalHours` 保留为只读兼容项；实际调度早已改为按小时定点触发（`scheduledHour`），详见下一节 `SAVE-BACKUP-SCHEDULE-HOUR-1`。
- 新增接口：`POST /api/instances/:id/saves/:name/backup`（管理员手动备份指定存档）；`GET|PUT /api/instances/:id/saves/backups/policy`（读取/更新策略，仅管理员）；`GET /api/instances/:id/saves/backups` 返回备份列表、当前 `policy`，以及本次请求顺带执行的 `maintenance` 结果（消费的存档事件数、新建的备份文件名列表）。
- 三类备份文件按固定规则命名：`latest_<存档名>.zip`（游戏保存后最新一份，覆盖式）、`scheduled_<存档名>.zip`（定时备份，覆盖式）、`daily_<存档名>_<YYYYMMDD>.zip`（按日期保留，超过 `dailyRetentionDays` 天数的旧快照会被自动清理）。
- SMAPI Control mod（`embedded/smapi-mod-src/ModEntry.cs`）在游戏内 `GameLoop.Saved` 触发时，把存档事件写入 `.local-container/control/save-events/*.json`；后端 `RunBackupMaintenance()` 在每次请求 `GET /api/instances/:id/saves/backups`（即前端打开"存档"页或刷新备份列表）时消费这些事件文件并驱动 latest/daily 备份生成，不依赖独立的后台定时任务。
- 影响文件：`backend/internal/games/stardew_junimo/saves.go`（`BackupPolicy`、`ReadBackupPolicy`/`WriteBackupPolicy`、`BackupSave`/`BackupLatest`/`BackupScheduled`/`BackupDailySnapshot`、`RunBackupMaintenance`）、`backend/internal/games/stardew_junimo/embedded/smapi-mod-src/ModEntry.cs`、`backend/internal/web/lifecycle_handlers.go`。
- 验证：`go test ./internal/games/stardew_junimo ./internal/web`；SMAPI Control DLL 改动需要重新编译嵌入才能让游戏内保存事件生效。

# SAVE-BACKUP-SCHEDULE-HOUR-1 定时备份整点设置

- `BackupPolicy` 将定时备份从 `scheduledIntervalHours` 改为 `scheduledHour`，合法范围为 0-23，默认 4，表示每天 04:00 后触发一次。
- `scheduledIntervalHours` 仅作为旧配置读取兼容字段保留，写入策略时会清零并通过 `omitempty` 不再落盘。
- `RunBackupMaintenance` 的定时备份判断改为本地时间每日一次：未到 `scheduledHour` 不执行；当天已经执行过不再重复执行；下一天到点后覆盖同一份 `scheduled_<save>.zip`。
- 新增测试 `TestScheduledBackupRunsOncePerDayAtConfiguredHour` 覆盖未到点、当天重复、次日再次执行。
- 验证：`go test ./internal/games/stardew_junimo ./internal/web`。
# SCHEDULED-RESTART-1 计划重启后端

- 新增 `restart_schedules` SQLite 表（迁移 `006_restart_schedules.sql`），按 `instance_id` 保存每日维护窗口：`shutdown_time`、`startup_time`、`timezone`、`warning_minutes_json`、`backup_before_shutdown`、`skip_if_players_online`，以及 `last_shutdown_at/last_startup_at/last_status/last_message`。
- 新增接口：`GET /api/instances/:id/restart-schedule` 读取配置；`PUT /api/instances/:id/restart-schedule` 仅管理员可更新配置。响应统一为 `{ schedule }`，并返回 `nextShutdownAt/nextStartupAt` 供前端展示。
- `cmd/panel` 启动 `web.RestartScheduler` 后台轮询。调度器只做编排：到关闭时间前按配置调用现有 `SendSay` 写控制命令；到关闭窗口时可先调用 `BackupSave` 备份当前激活存档，再复用 driver `Stop` 提交生命周期 job；到开启时间附近复用 driver `Start` 提交启动 job。
- 关闭前备份只打包已经落盘的 active save，不强制保存游戏内实时进度。备份失败会记录 `backup_failed` 并阻止本次关闭。
- 计划开启只在到点后 5 分钟宽限窗口内触发，避免面板首次启动时补跑很久以前的开启时间；计划关闭可在维护窗口内补跑一次。状态通过 `last_*_at` 防止 30 秒轮询重复提交。
- 验证：`cd backend; go test ./internal/storage ./internal/web ./cmd/panel`。
# MODDEPS-2 依赖状态与按存档安装匹配

- `GET /api/instances/:id/mods` 现在会在每个 `dependencies[]` 项上补齐状态字段：`installed/enabled/installedVersion/satisfied/status`。状态覆盖必需依赖缺失、依赖已安装但当前存档未启用、最低版本不满足、版本无法确认，以及可选依赖缺失等场景。
- 依赖判断在 `stardew_junimo.ListModsWithState(dataDir, activeSaveName)` 内完成，基于合并后的 `mods` 与 `mods-disabled` 物理列表，并叠加当前存档 profile 的 enabled 状态；不绕过 Junimo driver，也不在 web handler 堆 Stardew 逻辑。
- `GET /api/instances/:id/mods/nexus/search` 的已安装匹配改为调用当前存档维度的 `ApplyNexusInstalledMatch(dataDir, activeSaveName, results)`，因此当前存档禁用的 Mod 仍会显示 `installed=true`，同时返回 `installedEnabled=false`。
- 版本比较采用保守的数字段比较，例如 `1.10` 大于 `1.9`；无法解析的版本会返回 `unknown_version`，供前端提示人工确认。本阶段只做检测和展示，不自动安装依赖，也不阻止启用。
- 验证：`cd backend; go test ./...`。

# MODREL-1 Mod 依赖与来源包联动

- 新增 `mod_relationships.go`，统一计算已安装 Mod 的必需依赖图、反向依赖图和 Nexus 来源包 bundle。来源包只使用已有 `nexusModId/originNexusModId` sidecar 信息，不靠 `[CP]` 文件夹名前缀猜测。
- `PUT /mods/:modId/sync-classification` 改为调用 `SetModSyncClassificationCascade` 并返回 `{ mods, syncKind }`。同步分类按已安装必需依赖连通组一起变：同包成员、前置依赖、前置的前置以及依赖它的下游都会跟随当前选择，避免先点“待确认”后再切回其它标签时下游停留在旧状态。
- `PUT /mods/:modId/enabled` 改为调用 `SetModEnabledForSaveCascade` 并返回 `{ mods, enabled, saveName }`。启用时会一起启用同 Nexus 包成员和必需前置；禁用时会一起禁用同包成员和依赖它的下游，但不会禁用共享前置，例如禁用 Multiple Construction Orders 包不会禁用 Content Patcher。
- 验证覆盖：同步分类的 `Content -> Core -> Framework` 依赖链方向；MCO 主 Mod 与 `[CP]` 内容包同包启停，同时 Content Patcher 作为共享前置保持独立。
- 验证：`cd backend; go test ./...`；`cd frontend; npm.cmd run build`。
# NEXUS-EXT-2 远程安装任务日志中文修复

- `mod_remote_install` 与 `mod_nexus_install` 的 job 进度日志写入点已修复为正常 UTF-8 中文：`准备从远程链接安装 Mod`、`准备安装 Nexus Mod #...`、`已导入：...`。
- 安装或上传 Mod 成功后，如果当前实例有激活存档，后端会把本次导入的 Mod 写入该存档 profile 并标记为启用，避免后续 profile 应用把刚安装的 Mod 移到 `mods-disabled/`。
- 这次只修任务日志里用户直接看到的安装阶段文案；历史已经写入数据库的乱码日志不会被回写修复。
- 验证：`go test ./internal/games/stardew_junimo ./internal/web`。
# NEXUS-REQ-1 搜索结果前置依赖

- `GET /api/instances/:id/mods/nexus/search` 的 GraphQL 查询新增读取 `mods.nodes[].modRequirements.nexusRequirements`，搜索结果可返回 `requiredMods[]`。
- `requiredMods[]` 每项结构为 `{ modId, name, notes?, nexusUrl, installed, installedEnabled, installedFolderName?, installedVersion? }`。来源是 Nexus 页面声明的 Nexus 前置 Mod，不是压缩包内 `manifest.json` 的 SMAPI UniqueID。
- `ApplyNexusInstalledMatch(dataDir, saveName, results)` 现在会同时给搜索结果本身和 `requiredMods[]` 标记本地安装/当前存档启用状态，判断仍基于已安装 Mod 的 `UpdateKeys: ["Nexus:<id>"]`。
- GraphQL 只拉取前 10 个 Nexus 前置；外部依赖和自引用会被过滤。已安装后的精确 SMAPI 依赖状态仍由 `GET /mods` 的 `dependencies[]` 负责兜底。
- 验证：`cd backend; go test ./internal/games/stardew_junimo ./internal/web`。
# REMOTE-MOD-DOWNLOAD-1 远程 ZIP 大包下载超时

- `stardew_junimo.nexusDownloadArchive()` 不再复用 Nexus 搜索/API 的 10 秒 `nexusHTTPClient`，改用专门的 `nexusArchiveHTTPClient`，ZIP body 读取 timeout 放宽到 15 分钟。
- 触发场景：Ridgeside Village 等大体积 Nexus CDN ZIP 在免费慢速下载链路下，10 秒内无法读完整个 body，旧实现会在 `io.Copy` 阶段报 `context deadline exceeded (Client.Timeout or context cancellation while reading body)`。
- 搜索、GraphQL、v1 REST 等接口仍使用 10 秒短超时；只有实际 ZIP 下载走长超时，并继续受 `maxModZipBytes` 200 MB 限制和 job 30 分钟总 timeout 约束。
- 验证：`cd backend; go test ./internal/games/stardew_junimo ./internal/web` 通过。
# NEXUS-EXT-BATCH-2 Nexus 来源纠偏

- `UploadModZip` 拆出内部 `uploadModZip(..., uploadModZipOptions)`；普通上传继续推断 Nexus 包来源，`mod_nexus_install` 与 `mod_remote_install` 这类已经携带显式 Nexus 上下文的安装不再先写推断来源，避免临时 ZIP 安装过程中产生错误 sidecar。
- `SaveInstalledNexusMetadata` 会在多 Mod 包中检查导入 Mod 自己的正数 `Nexus:<id>`。如果浏览器扩展批量上下文传来的 `result.modId` 与包内唯一正数 Nexus ID 冲突，会以包内声明为准纠偏，并清除旧的其它 Nexus ID 缓存字段后再写入。
- 修复场景：Ridgeside Village 包内 `[CP] Ridgeside Village` 声明 `Nexus:7286`，其它组件为 `Nexus:-1`；即使批量流程误把 result 带成 SpaceCore `1348`，三个 Ridgeside 组件也会归到 Ridgeside CP 组件，而不是显示“随 SpaceCore 安装”。
- 验证：`cd backend; go test ./internal/games/stardew_junimo ./internal/web`。

# NEXUS-EXT-PACK-1 浏览器扩展下载包

- 新增 `GET /api/instances/:id/mods/nexus/extension/download`，任意已登录用户可下载面板打包好的 Nexus 普通用户浏览器扩展 ZIP。
- 后端下载接口采用折中策略：优先复用实例目录 `.local-container/browser-extensions/anxi-nexus-installer.zip` 中已有且合法的预打包 ZIP；如果文件不存在，会优先复制镜像/仓库中的预打包 `browser-extensions/anxi-nexus-installer.zip`；如果预包也不存在或损坏，才从 `browser-extensions/nexus-slow-installer` 源码重新生成。
- 缓存与预包复用现在是**版本感知**的：`EnsureNexusInstallerExtensionZip()` 会读取源码 `manifest.json` 的 `version` 作为期望版本，只有实例缓存 / 预包 ZIP 内 `manifest.json` 的版本与之**完全一致**时才复用，否则视为过期并从源码重新打包。这样每次升级扩展（bump `manifest.json` 版本）后，用户重新下载即可拿到新版，无需手动删缓存。源码不可用时退回仅结构校验的旧行为。
- ZIP 根目录直接包含 `manifest.json`、`background.js`、`content.js` 等扩展文件，并额外写入 `安装说明.txt`。玩家解压后选择该解压目录即可加载扩展，不需要再进入内层文件夹。
- Docker 镜像运行层会复制 `browser-extensions/` 到 `/app/browser-extensions/`，并在构建时生成 `/app/browser-extensions/anxi-nexus-installer.zip`；正式部署优先使用这个预打包 ZIP。
- 验证：`cd backend; go test ./internal/games/stardew_junimo ./internal/web`。
# NEWGAME-PLAYERLIMIT-1 自定义新存档联机人数上限

- `NewGameConfig` 新增 `maxPlayers`，表示最大同时在线人数，合法范围 `1-100`；未传或为 `0` 时保持兼容并在写配置前默认成 `10`。
- `WriteServerSettings` 现在把人数上限写入 Junimo 官方 `server-settings.json` 的 `Server.MaxPlayers`，并显式写入 `Server.CabinStrategy="CabinStack"` 与 `Server.ExistingCabinBehavior="KeepExisting"`，继续让 Junimo 的自动小屋管理处理超过原版初始小屋上限的玩家加入。
- `startingCabins` 仍表示新建存档时地图上初始联机小屋数量，范围保持 `0-7`；`maxPlayers` 不能小于 `startingCabins + 1`，避免小屋数和总人数上限矛盾。
- 涉及文件：`backend/internal/games/registry/types.go`、`backend/internal/games/stardew_junimo/saves.go`、`backend/internal/games/stardew_junimo/saves_test.go`。
- 验证：`cd backend; go test ./internal/games/stardew_junimo -run "WriteServerSettings|ValidateNewGameConfig"` 通过；`cd frontend; npm.cmd run build` 通过。
# PERF-REVIEW-1 存档与 Nexus 元数据轻量优化

- 存档信息解析的 `whichFarm` 兜底读取改为流式扫描：`readWhichFarmFromMainFile` 不再 `os.ReadFile` 整个主存档 XML，备份 ZIP 元数据解析也不再为了文件大小提前读取主存档 entry。
- `enrichBackupInfo` 只通过 ZIP header 读取主存档未压缩大小；只有 `SaveGameInfo` 需要农场类型兜底时才打开主存档 entry 流式扫描 `<whichFarm>`。
- Nexus 已安装元数据补全把“同 Nexus modId 是否已有展示元数据”的判断预先构建为 map，避免每个 Mod 都遍历 sidecar store。
- 接口契约不变，主要收益是降低大存档/多 Mod 列表下的内存峰值和重复扫描。
- 验证：`cd backend; go test ./...`。
# VNC-CONTROL-1 服务器页 VNC 显示代理

- 新增 `GET/POST /api/instances/:id/rendering`，管理员专用且要求实例为 `running`。`GET` 返回当前 Junimo 服务端渲染 `{ "fps": number }`，用于前端刷新后恢复 VNC 按钮真实状态；`POST` 请求体 `{ "fps": number }`，当前前端用 `15` 打开 VNC 显示、`0` 关闭。
- Web 层只做鉴权、状态校准和路由，实际调用在 `stardew_junimo.GetRenderingFPS()` / `SetRenderingFPS()` 内完成：通过 `docker compose exec server curl http://localhost:<API_PORT>/rendering` 访问 JunimoServer REST API；POST 空 body 需要显式 `Content-Length: 0`。
- `API_PORT` / `API_KEY` 从实例 `.env` 读取，后端在容器内注入 `Authorization: Bearer ...`；浏览器前端不会接触 Junimo API key。
- 验证：`cd backend; go test ./internal/games/stardew_junimo -run Rendering`、`cd backend; go test ./internal/web -run "Rendering|VNCConfig"`。

# STEAM-QR-PHASE-CLASSIFY-1 Steam QR 阶段识别修正

- 修复扫码登录选择后，steam-auth 日志中的 `Choice [1]: 2` 被后端误识别为 Steam Guard 手机 App 批准的问题。该行只是上游菜单的默认值提示与用户选择回显，不能作为 `steam_guard_mobile_required` 的依据。
- `installer.go` 现在只把明确的 `waiting for approval`、`open steam app`、`approve in steam mobile` 文案识别为手机批准；QR 模式下不再让泛化的 `steam guard/two-factor/authenticator` 文案覆盖 `steam_qr_required`。
- 影响文件：`backend/internal/games/stardew_junimo/installer.go`、`backend/internal/games/stardew_junimo/driver_test.go`。接口不变，仍通过 `POST /api/instances/:id/steam-guard/input` 发送菜单选择或验证码。
- 现场诊断：同一 `anxiyizhi/junimo-steam-service-cn:1.5.0-anxi.2` 镜像在 `stardew_default` 网络内可解析 Steam 域名，TCP `api.steampowered.com:443` 与 `cm1-ord1.cm.steampowered.com:27017` 可连通，TLS 验证正常；本轮问题不是 Docker 容器完全断网，而是 QR 认证阶段 SteamClient 会话连接未建立成功。
- 验证：`cd backend; go test ./internal/games/stardew_junimo -run "SteamAuthMenus|SteamGuardCodePrompt|QRCodeChoice|SteamMobileApproval"`；`cd backend; go test ./internal/games/stardew_junimo`。
# BE-STEAM-POST-AUTH-RETRY-1 Steam 认证成功后的失败不再归类为认证失败
- 修复 steam-auth 已经登录成功后，游戏下载或后续安装步骤失败仍把实例写成 `steam_auth_failed` 的问题。`runSteamAuthAttempt()` 现在在 `authSucceeded=true` 后遇到容器错误会写 `state=error, driverPhase=post_auth_failed`；遇到 `Download failed:` / `Game download failed` 会写 `state=error, driverPhase=download_failed`。
- 这样前端可以明确区分“账号密码/Steam Guard 错误”和“认证后的 CDN/磁盘/后续安装失败”，后者重试时复用 `.env` 中已保存的 Steam 凭据，不要求用户重新输入账号密码。
- 影响文件：`backend/internal/games/stardew_junimo/installer.go`、`backend/internal/games/stardew_junimo/driver_test.go`。未改变 `POST /api/instances/:id/install` 请求体，继续使用已有 `reuseCredentials=true` 重试契约。
- 验证：`cd backend; go test ./internal/games/stardew_junimo -run "DownloadFailedAfterSuccessfulAuth|InstallMarksSteamAuthFailedWhenRunErrors"` 通过；同时保留原 steam-auth 容器启动失败仍写 `steam_auth_failed` 的覆盖测试。
# STEAMCMD-PULL-PROGRESS-1 镜像拉取进度估算

- 镜像拉取进度会写入隐藏 job 日志 `[pull:progress:done:total]`。Junimo compose pull 按服务镜像完成数估算；SteamCMD 单镜像 pull 按 Docker layer 的 `Pulling fs layer` / `Pull complete` / `Already exists` 估算。
- `pull_running` 和 `steamcmd_image_pulling` 阶段会同步更新实例 `stateMessage`，文案包含“约 N%”，让安装页顶部和镜像拉取卡不再停在纯等待状态。
# JUNIMO-IMAGE-CANDIDATES-1 Junimo/server 与 steam-auth-cn 镜像候选兜底

- 安装阶段不再对 `server` / `steam-auth` 走单点 `docker compose pull`；后端会分别按候选镜像列表执行 `ImageInspect`，本地已有任意候选即直接使用，全部缺失时才逐个 `docker pull`。
- 新增 `.env` 键：`SERVER_IMAGE`、`SERVER_IMAGE_CANDIDATES`、`STEAM_SERVICE_IMAGE_CANDIDATES`。拉取成功或命中本地镜像后，后端会把实际使用的镜像写回 `SERVER_IMAGE` / `STEAM_SERVICE_IMAGE`，`docker-compose.yml` 通过这些变量启动。
- 默认候选顺序：server 为 `docker.1ms.run/sdvd/server:<tag>`、`docker.m.daocloud.io/sdvd/server:<tag>`、`ghcr.io/sdvd/server:<tag>`、`sdvd/server:<tag>`；steam-auth-cn 为 `docker.1ms.run/anxiyizhi/junimo-steam-service-cn:1.5.0-anxi.2`、`crpi-9z3bkb9g7fxeohrg.cn-hangzhou.personal.cr.aliyuncs.com/anxi-panel/junimo-steam-service-cn:1.5.0-anxi.2`、`docker.m.daocloud.io/anxiyizhi/junimo-steam-service-cn:1.5.0-anxi.2`、`ghcr.io/anxiyizhi/junimo-steam-service-cn:1.5.0-anxi.2`、`anxiyizhi/junimo-steam-service-cn:1.5.0-anxi.2`。
- 旧实例安装时会把 `server` compose 镜像行从 `sdvd/server:${IMAGE_VERSION:-...}` 迁移为 `${SERVER_IMAGE:-sdvd/server:1.5.0-preview.121}`；`IMAGE_VERSION` 仍保留用于版本选择和默认候选生成。
- 拉取进度继续通过 `[pull:progress:done:total]` 隐藏日志给前端估算百分比；单个候选失败会记录 warning 并继续下一个候选，全部失败才进入 `pull_failed`。
- 影响文件：`backend/internal/games/stardew_junimo/installer.go`、`compose_template.go`、`driver.go`、`config/env.go` 及对应测试。
- 验证：`cd backend; go test ./internal/games/stardew_junimo/config`；`cd backend; go test ./internal/games/stardew_junimo -run "Prepare|Migrate.*ComposeImage|SteamCMD|InstallFallsBack|InstallResumes|InstallUsesExistingLater|InstallMarksSteamAuthFailed|SteamAuthMenus|SteamGuard|QRCode"`。
# JUNIMO-IMAGE-CANDIDATES-2 老实例候选源自动补齐

- 修复旧实例 `.env` 中 `SERVER_IMAGE_CANDIDATES` 或 `STEAM_SERVICE_IMAGE_CANDIDATES` 只有单个旧值时，安装流程只显示/尝试 `(1/1)` 的问题。
- `stardew_junimo` 现在会始终把默认候选源排在前面，再追加实例 `.env` 中已有候选和当前主镜像值并去重；server 默认顺序为 `docker.1ms.run/sdvd/server:<IMAGE_VERSION>`、`docker.m.daocloud.io/sdvd/server:<IMAGE_VERSION>`、`ghcr.io/sdvd/server:<IMAGE_VERSION>`、`sdvd/server:<IMAGE_VERSION>`。
- steam-auth cn 版同样补齐：`docker.1ms.run/anxiyizhi/junimo-steam-service-cn:1.5.0-anxi.2`、`crpi-9z3bkb9g7fxeohrg.cn-hangzhou.personal.cr.aliyuncs.com/anxi-panel/junimo-steam-service-cn:1.5.0-anxi.2`、`docker.m.daocloud.io/anxiyizhi/junimo-steam-service-cn:1.5.0-anxi.2`、`ghcr.io/anxiyizhi/junimo-steam-service-cn:1.5.0-anxi.2`、`anxiyizhi/junimo-steam-service-cn:1.5.0-anxi.2`。
- 安装流程写入 `.env` 时会同步写回补齐后的候选列表，并在镜像命中/拉取成功后写回实际使用的 `SERVER_IMAGE` / `STEAM_SERVICE_IMAGE`。
# STEAMCMD-HOME-CACHE-CLEANUP-1 SteamCMD HOME 与缓存清理加固

- 修复真实云服上 SteamCMD fallback 仍在 `Waiting for client config...` 阶段 exit code `139` 的问题。根因之一是 `su -m steam` 会保留 root 的 `HOME`，导致 SteamCMD 虽以 `steam` 用户运行但仍写入 `/root/.local/share/Steam` 自更新缓存；现在运行 SteamCMD 时显式设置 `HOME=/home/steam USER=steam LOGNAME=steam`。
- `docker.Client.RemoveVolumes()` 改为逐个执行 `docker volume rm -f <name>`，并忽略 `no such volume` / `volume not found`。这样 139 后清理 `steamcmd-user-local` / `steamcmd-root-local` 时，不会因为其中一个卷不存在而整次清理失败。
- 139 后清理 runtime cache 前会先按 volume 查找并强制删除残留的一次性 SteamCMD 容器，解决真实服务器上 `volume is in use - [container_id]` 导致缓存无法清理的问题。
- SteamCMD runtime cache 清理失败时，任务日志会附带 Docker stderr/stdout 的脱敏详情，便于区分“不存在”“被容器占用”“Docker 权限异常”等情况。
- 影响文件：`backend/internal/docker/compose.go`、`backend/internal/games/stardew_junimo/installer.go`、`backend/internal/docker/compose_test.go`、`backend/internal/games/stardew_junimo/driver_test.go`。
- 验证：`cd backend; go test ./internal/docker ./internal/games/stardew_junimo`、`cd backend; go test ./...`。

# MOJIBAKE-FIX-1 mods.go 与 lifecycle_handlers.go 历史乱码修复

- 用户反馈 `mod_remote_install` 任务日志报错显示乱码：`UniqueID "..." 宸插瓨鍦ㄤ簬 Mod "..." 涓? (mod_exists)`。排查发现 `backend/internal/games/stardew_junimo/mods.go` 和 `backend/internal/web/lifecycle_handlers.go` 里大量中文字符串字面量（错误提示、注释）早已是乱码，推测是早期某次保存时把正确的 UTF-8 中文按 GBK(cp936) 误解码又存回 UTF-8 导致（correct UTF-8 bytes → decode as GBK → re-encode as UTF-8）。
- 该乱码是**确定性、可逆**的：`原文 = UTF8.Decode(GBK936.Encode(乱码文本))`。用这个公式加脚本对两个文件做了全量修复（含错误提示、`fmt.Errorf`、注释、`// ──` 分隔线），已验证输出符合语境。少数原文标点在损坏当时就已经丢失（表现为文件中残留的半角 `?`），这部分靠上下文人工补全（如"已存在于 Mod...中""不是合法的 SMAPI Mod"）。
- **顺带修复一个真实功能 bug**：`lifecycle_handlers.go` 里 `handleSavesBackupDelete` / `handleSavesBackupRestore` 用 `strings.Contains(errMsg, "涓嶅悎娉?")` / `"涓嶅瓨鍦?")` / `"宸插瓨鍦?")` 这类乱码字符串去匹配 `saves.go` 返回的**正确** UTF-8 错误文本（如"不合法""不存在""已存在"），两边编码不一致导致匹配永远失败，备份删除/恢复接口原本应返回的 `400 invalid_backup_name` / `404 backup_not_found` / `409 save_exists` 全部静默退化成通用 `500`。已改为匹配正确的中文子串。
- 影响文件：`backend/internal/games/stardew_junimo/mods.go`、`backend/internal/web/lifecycle_handlers.go`。
- 验证：`cd backend; go build ./...`、`go vet ./...`、`go test ./...` 全绿；已用脚本对整个 `backend` 目录做 `[CJK]\?` 乱码特征扫描确认清零。
- 下一步注意事项：这次只清理了这两个文件；如果后续在其他文件的错误提示里再看到类似"字符像中文但读不通"的乱码，大概率是同一根因，可用上述可逆公式复查。

# MOD-REMOTE-IDEMPOTENT-1 远程/Nexus Mod 安装重复包幂等

- `mod_remote_install` / `mod_nexus_install` 现在只在下载类安装路径里启用 `allowAlreadyInstalled`：ZIP 中某个 Mod 的 `UniqueID` 已存在时跳过该目录，不再把 `(mod_exists)` 作为 job 失败；如果整个 ZIP 都已安装，任务记录"已安装，跳过重复导入"并以 succeeded 结束。
- 普通手动上传 `POST /mods/upload` 仍保持严格语义：重复 `UniqueID` 继续返回 `400 mod_exists`，避免用户误以为上传替换已发生。
- 同一包部分目录已存在、部分目录缺失时，远程/Nexus 安装会导入缺失目录并跳过已存在目录，用于修复浏览器扩展缓存/刷新导致重复提交时的误失败。
- 新建 Mod 下载类 job 的 `displayName` 改为 Mod 名在前，例如 `Ridgeside Village · mod_remote_install`，任务类型仍保留在 `type` 字段。
- 影响文件：`backend/internal/games/stardew_junimo/mods.go`、`remote_install.go`、`nexus_install.go`、`backend/internal/web/lifecycle_handlers.go` 及测试。

# SAVE-POINTER-SUFFIX-HEAL-1 gameloader 指针存档名前缀写错的自愈修复

- 现场实证：JunimoServer 官方 Mod 新建存档时会把 `junimohost.gameloader.json` 的 `SaveNameToLoad` 前缀写错（如指针 `test_443102605`，真实目录却是 `test2_443102605`，两者数字后缀一致），导致"当前激活存档"卡片农场主/日期/文件大小等字段永久显示"未知"，新建存档轮询也会误报"未检测到新存档目录"超时告警（尽管存档已生成、可正常联机）。
- `saves.go` 新增 `suffixMatchSaveDir()`：指针目录不存在时，按数字后缀在 `Saves/` 下找唯一匹配的真实目录；歧义（多个候选）时不纠正。`GetActiveSaveName()`（15+ 处调用方）接入该只读兜底，指针目录存在时行为不变。
- `lifecycle.go` `sendNewGameCommand()` 检测到指针目录不存在时，用同一兜底找到真实目录后调用新拆出的 `writeGameloaderPointer()` 持久化修正指针，按修正后的名字继续走正常成功路径，不再误报超时。
- 影响文件：`backend/internal/games/stardew_junimo/saves.go`、`lifecycle.go`、`saves_test.go`。详见 `docs/backend-handoff/backend-handoff-2026-07-07.md` 的 `SAVE-POINTER-SUFFIX-HEAL-1`。
- 验证：`cd backend; go test ./internal/games/stardew_junimo -run "GetActiveSaveName|DeleteSave_ActiveSaveCleanup"`；`go build ./... && go vet ./... && go test ./...`。
- 验证：`cd backend; go test ./internal/games/stardew_junimo ./internal/web`。

# PLAYERS-KICK-1 踢出玩家 + PASSWORD-STATUS-1 加入密码设置/认证状态

- JunimoServer 上游没有"运行时踢人"和"运行时改密码"的 REST API：密码只能靠容器启动时的 `SERVER_PASSWORD` 环境变量生效，踢人只有游戏内聊天指令 `!kick`（需要管理员权限的在线玩家发送，面板无法模拟）。详见 `docs/10-junimo-rest-api.md` 3.5 节。改为复用面板自带的 `StardewAnxiPanel.Control` SMAPI Mod 命令队列（和喊话 `say` 同一套机制）实现踢人；密码继续走 `.env` 读写 + JunimoServer `GET /auth` 只读状态代理。
- **踢人**：`console.go` 把原 `writePanelBroadcastCommand` 拆成通用的 `writePanelCommand(dataDir, name, payload)`；新增 `Driver.KickPlayer(ctx, instance, uniqueMultiplayerID, name)`，写一个 `{name:"kick", payload:{uniqueMultiplayerId, name}}` JSON 命令文件到 `<dataDir>/control/commands/`，fire-and-forget，要求实例处于 running。SMAPI Mod 侧（`embedded/smapi-mod-src/ModEntry.cs`）`HandleCommand` 新增 `case "kick"`：按 `UniqueMultiplayerID` 在线查找玩家，找不到或目标是主机（`Game1.MasterPlayer`）时忽略并写状态日志，否则调用 `Game1.server?.kick(targetId)`。**已重新编译并替换嵌入 DLL**（`embedded/smapi-mod/StardewAnxiPanel.Control.dll`），影响现有运行实例需要重启/重新准备 server 容器才能刷新到 `.local-container/mods`。
- **加入密码**：新增 `backend/internal/web/server_password_handlers.go`：`GET/PUT /api/instances/:id/config/server-password`，管理员权限，直接读写 `.env` 的 `SERVER_PASSWORD`（该字段此前已在 `EmptyEnvTemplate`/`writeEnvFile` 里预留，未新增 env schema），与 `ports_handlers.go` 的 VNC 端口读写完全同构。**该密码只在 server 容器下次启动时生效**，运行中的服务器不会热更新。
- **密码保护状态（只读）**：新增 `backend/internal/games/stardew_junimo/auth_status.go`，`Driver.GetAuthStatus(ctx, instance)` 复用 `rendering.go` 的"容器内 curl 代理 Junimo REST API"模式（`docker compose exec server curl ... http://localhost:$API_PORT/auth`，浏览器不会看到 Junimo `API_KEY`），返回 `enabled`/`authenticatedCount`/`pendingCount`/`timeoutSeconds`/`maxAttempts`。Web 层 `GET /api/instances/:id/password-status`（`server_password_handlers.go` 的 `handleInstancePasswordStatus`），要求实例 running，未就绪时映射 `junimo_api_unavailable` → `502`。
- 路由新增：`POST /api/instances/:id/players/kick`（`players_handlers.go` 的 `handlePlayerKick`，管理员权限）、`GET/PUT /api/instances/:id/config/server-password`、`GET /api/instances/:id/password-status`，均在 `instance_handlers.go` 里按现有 `strings.Split` 分支树风格接入。
- 审计日志新增 `player_kick`（metadata: `uniqueMultiplayerId`、`name`）、`instance_server_password_update`（metadata 只记录 `passwordSet: true/false`，不记录明文密码，避免审计表泄露密码）。
- 影响文件：`backend/internal/games/stardew_junimo/console.go`、`auth_status.go`（新增）、`backend/internal/web/players_handlers.go`、`server_password_handlers.go`（新增）、`instance_handlers.go`、`backend/internal/games/stardew_junimo/embedded/smapi-mod-src/ModEntry.cs`、`embedded/smapi-mod/StardewAnxiPanel.Control.dll`。
- 验证：`cd backend; go build ./... && go test ./...` 全绿；SMAPI Mod 用 Docker 官方文档命令重新编译（`docker run --rm -v ".../smapi-mod-src:/src" -v "E:\stardew-anxi-panel\runtime\game:/game" -w /src mcr.microsoft.com/dotnet/sdk:6.0 dotnet build -c Release /p:GamePath=/game`），Build succeeded，0 Errors（仅有历次已知的 ModBuildConfig analyzer 版本 warning）。**未做真机联机验证**：没有在真实运行中的 JunimoServer 实例上实际踢出过在线玩家，`Game1.server?.kick(...)` 的运行时行为、`GET /auth` 的真实响应字段名大小写只依据上游源码和文档确认，建议下一位维护者在真机上验证一次完整链路（设密码重启 → 玩家 `!login` → 踢人 → 状态查询）。
- 下一步注意事项：踢人和 say 一样是 fire-and-forget，Go 层拿不到 SMAPI Mod 侧的真实执行结果（成功/目标离线/主机保护触发都只会写 `control/status.json`，不回传给 API），前端只能提示"指令已提交"；如果以后要做"踢人失败原因"这类精确反馈，需要设计一个命令结果回传通道（例如命令文件里带 id，Mod 处理完写一个 `command-results/<id>.json`，后端轮询读取），目前为了和 `say` 保持一致故意没做。密码保存后没有自动提示/触发重启，是否要在 UI 上强提醒"需要手动重启"由后续迭代决定。

# USER-PASSWORD-RESET-1 面板账号"重置密码"权限修正

- 注意区分：这里的"密码"是**面板登录账号密码**，和上面 `PASSWORD-STATUS-1` 的 JunimoServer 加入密码是完全不同的两个东西。
- 起因：用户提出"普通用户不能改自己密码，管理员能改自己的和普通用户的，第一个注册的管理员能改所有，包括自己"。调研发现 `PATCH /api/users/{id}` 的 `password` 字段后端早就支持（`users_handlers.go`），但从未在前端暴露；同时 `storage.UpdateUser`（`backend/internal/storage/auth.go:217`）原有的权限检查 `target.Role == auth.RoleAdmin && !actor.IsSuperAdmin` 没有排除"改自己"的情况，导致连普通管理员改自己的密码都会被 `ErrSuperAdminRequired` 拒绝——这比用户想要的规则更严格。
- 改了什么：`storage/auth.go` 的 `UpdateUser` 权限检查加一个 `targetID != actorID` 条件，改成 `target.Role == auth.RoleAdmin && targetID != actorID && !actor.IsSuperAdmin`。效果：
  - 普通管理员：能改自己的密码（`targetID == actorID`，条件不成立，放行）；能改普通用户的密码（`target.Role != admin`，条件不成立，放行）；不能改其他管理员的密码（`targetID != actorID` 且 `target.Role == admin` 且不是超级管理员，拒绝）。
  - 超级管理员：`actor.IsSuperAdmin` 恒真，条件永远不成立，能改任何人的密码，包括自己。
  - 普通用户：整个 `/api/users/*` 都是 `requireAdmin`，普通用户连接口都摸不到，天然满足"不能改自己密码"。
  - 角色变更（`params.Role != nil && !actor.IsSuperAdmin`）和自我禁用（`ErrSelfDisable`）两个独立检查完全没动，行为不受影响。
- 影响文件：`backend/internal/storage/auth.go`（1 行条件修改）。
- 验证：新增 `backend/internal/web/auth_handlers_test.go` 的 `TestPasswordChangePermissions`，覆盖"管理员改自己密码成功且能用新密码登录"、"管理员改普通用户密码成功"、"管理员改另一个管理员密码被拒绝"、"超级管理员改任何人（含自己）密码成功"、"普通用户完全摸不到接口"五种场景；`cd backend; go build ./... && go test ./...` 全绿。注意测试里"超级管理员改自己密码"必须放在"改别人密码"**之后**执行，因为改自己密码会让当前 session 立即失效，这个坑在写测试时踩过一次。

# FESTIVAL-EVENT-1 触发节日活动 + JOJA-ROUTE-1 永久启用 Joja 路线

- 需求来源：`docs/handbook`（或 website 快速上手文档）里列出的 Junimo 容器"管理命令"表格中 `!event`（启动当前节日活动，卡住时用）和 `!joja IRREVERSIBLY_ENABLE_JOJA_RUN`（永久启用 Joja 路线，不可逆）此前只是文档列出、面板没有对应按钮。
- 调研结论（读 `E:\codex\junimo-server-upstream` 上游源码）：这两个都是 JunimoServer 的**游戏内聊天指令**，不是 SMAPI 控制台命令也没有 REST API，`docs/10-junimo-rest-api.md` 确认没有对应端点。触发它们的唯一挂载点是 Harmony 补丁的 `StardewValley.Menus.ChatBox.receiveChatMessage`（`ChatCommands.cs`），而这个方法只会在真实调用 `ChatBox.textBoxEnter(text)` 时才会被触发（`textBoxEnter` 源码：先 `Game1.multiplayer.sendChatMessage(...)` 广播文本，再本地调用 `receiveChatMessage(Game1.player.UniqueMultiplayerID, 0, ..., text)`）。面板的 WebSocket `chat_send` 走的是 `ApiService` 的 `OnExternalChatMessage → SendExternalMessage → helper.SendPublicMessage`，**不会**进入 `ChatWatcher`/命令解析链路，因此这条路走不通。用 `System.Reflection.MetadataLoadContext` + `ilspycmd` 反编译验证过 `ChatBox.receiveChatMessage(Int64 sourceFarmer, Int32 chatKind, LanguageCode language, String message)` 是实例方法、`textBoxEnter` 内部确实会本地回调它，确认了下面的实现方式可行。
- **权限差异（重要）**：`!event`（`AlwaysOnFestivals.StartEventCommand`）**没有任何权限校验**，任何人（包括模拟的主机身份）都能触发。`!joja`（`JojaCommand`）要求 `roleService.IsPlayerAdmin(msg.SourceFarmer)` 为真，而 `RoleService` 明确"没有默认管理员赋权"（`// No default admin assignment - admins are configured via ADMIN_STEAM_IDS`），dedicated server 的主机（host）**不会自动获得 admin**。因此 `!event` 可以直接模拟触发，`!joja` 必须先把主机提升为 admin。
- **实现方式**：
  - `embedded/smapi-mod-src/ModEntry.cs` 的 `HandleCommand` 新增 `case "trigger-event"` 和 `case "enable-joja"`，分别调用新方法 `TriggerFestivalEvent()` / `EnableJojaRoute()`：两者都先检查 `Context.IsWorldReady && Game1.chatBox is not null`，再调用 `Game1.chatBox.textBoxEnter("!event")` / `Game1.chatBox.textBoxEnter("!joja IRREVERSIBLY_ENABLE_JOJA_RUN")`，模拟主机在聊天框输入这两条指令（会广播到所有玩家聊天记录，属预期行为，保证透明度）。
  - `console.go` 新增 `Driver.TriggerFestivalEvent(ctx, instance)`：校验实例 running，写 `{"name":"trigger-event"}` 命令文件到 `control/commands/`，fire-and-forget，和 kick/say 完全同构。
  - 新增 `joja.go`：`Driver.EnableJojaRoute(ctx, instance, confirm string)`：
    1. 校验 `confirm` 精确等于 `IRREVERSIBLY_ENABLE_JOJA_RUN`（`jojaConfirmText` 常量），不等直接拒绝（`confirm_mismatch`）——这是后端侧的硬校验，不只依赖前端弹窗。
    2. 校验实例 running。
    3. `findHostPlayerID(dataDir)`：复用已有的 `readPlayersFromControl`（players.json）找 `IsHost == true` 的条目取 `UniqueMultiplayerID`；找不到返回 `host_unknown`。
    4. `callRolesAdminAPI(ctx, ld, instance, hostID)`：复用 `rendering.go` 的"容器内 curl 代理 Junimo REST API"模式，调用 JunimoServer 自带的 `POST /roles/admin?playerId=<hostID>` 把主机提升为 admin（幂等，即使已是 admin 也不会报错）。
    5. 提升成功后才写 `{"name":"enable-joja"}` 命令文件。
  - Web 层新增 `backend/internal/web/festival_handlers.go`：`handleFestivalEventTrigger`（`POST /api/instances/:id/festival/event`）、`handleJojaRouteEnable`（`POST /api/instances/:id/joja/enable`，请求体 `{"confirm": "..."}`），均 `requireAdmin`。路由接入 `instance_handlers.go`。
  - 审计日志新增 `festival_event_trigger`、`joja_route_enable`（metadata 只记 `confirmed: true`，不记录多余信息）。
  - **已重新编译并替换嵌入 DLL**（见下方"如何验证"）。
- 影响文件：`backend/internal/games/stardew_junimo/console.go`、`joja.go`（新增）、`backend/internal/web/festival_handlers.go`（新增）、`instance_handlers.go`、`embedded/smapi-mod-src/ModEntry.cs`、`embedded/smapi-mod/StardewAnxiPanel.Control.dll`。前端改动见 `docs/frontend-handoff/frontend-handoff-2026-07-10.md`。
- 验证：`cd backend; go build ./... && go vet ./... && go test ./internal/games/stardew_junimo/... ./internal/web/...` 全绿。SMAPI Mod 用文档命令通过 Docker 重新编译（`dotnet build -c Release /p:GamePath=/game`），`Build succeeded. 0 Errors`（仅历次已知的 ModBuildConfig analyzer 版本 warning），编译产物已 md5 校验复制覆盖嵌入 DLL，`go build ./...` 确认新 DLL 能正常被 `//go:embed` 打包。前端 `cd frontend; npx tsc --noEmit -p . && npm run build` 通过。**未做真机联机验证**：`Game1.chatBox.textBoxEnter` 在真实 dedicated server 进程里是否总是非空、`POST /roles/admin` 提升主机后 `!joja` 是否真的通过权限校验，只依据反编译的游戏程序集源码和 JunimoServer 上游源码交叉验证，没有在真实运行中的实例上跑过完整链路。
- 下一步注意事项：
  - `!event`/`!joja` 和 kick/say 一样是 fire-and-forget，前端只能提示"指令已提交"，拿不到"当天没有节日所以没生效"这类精确反馈；如果要做精确反馈需要设计命令结果回传通道（同 `PLAYERS-KICK-1` 提到的方案），这次故意没做。
  - `EnableJojaRoute` 每次调用都会先打一次 `POST /roles/admin`，如果以后要在面板里做"管理员列表"这类功能，需要注意主机会永久出现在 JunimoServer 的 admin 名单里（`RoleService` 没有自动撤销机制，`UnassignAdmin` 也明确禁止撤销 host）。
  - 建议下一位维护者找一个测试实例实际验证一次：当天有节日时点"触发节日活动"是否真的提前进入主线剧情倒计时；点"永久启用 Joja 路线"后 `GET /settings` 或存档内 `IsCommunityCenterRun` 是否真的变化。
- 下一步注意事项：这次只动了权限判断，没有新增"验证旧密码"这类二次校验（`validatePassword` 只校验新密码长度 ≥ 6 位）——管理员重置别人密码本来就不需要知道对方原密码，这是设计如此，不是遗漏。

# CABIN-STRATEGY-1 小屋策略设置分层（新建存档简化 + 服务器控制页完整高级设置）

- 需求来源：`CabinStrategy`（小屋策略）此前在 `WriteServerSettings` 里被硬编码为 `"CabinStack"`，`ExistingCabinBehavior` 硬编码为 `"KeepExisting"`，用户完全无法选择，也无法在已有存档上事后调整。用户给出的设计：新建存档页只暴露一个简化二选一（推荐/原版），服务器控制页给完整高级设置（`CabinStrategy`/`ExistingCabinBehavior`/`NetworkBroadcastPeriod`），两边必须共用同一份底层配置来源，不能各自为政。
- `registry.NewGameConfig` 新增 `CabinMode string`（json `cabinMode`），取值 `recommended|vanilla`，默认 `recommended`（`normalizeCfg` 兜底）。`WriteServerSettings` 不再硬编码，而是按 `cfg.CabinMode` 派生 `Server.CabinStrategy`：`recommended → "CabinStack"`，`vanilla → "None"`；`Server.ExistingCabinBehavior` 仍固定写 `"KeepExisting"`（新建存档场景下没有"已有小屋"需要处理，这个字段只在事后调整已有存档时才有意义）。`validateCfg` 新增 `cabinMode` 必须是 `recommended`/`vanilla` 之一的校验。
- 新增独立类型 `ServerRuntimeSettings{ MaxPlayers, CabinStrategy, ExistingCabinBehavior, NetworkBroadcastPeriod }`（`saves.go`），以及两个函数：
  - `ReadServerRuntimeSettings(dataDir) (ServerRuntimeSettings, error)`：读 `server-settings.json` 的 `Server` 段，字段缺失时兜底为 `10`/`CabinStack`/`KeepExisting`/`1`（不存在整个文件时同样返回这组默认值，不报错）。
  - `UpdateServerRuntimeSettings(dataDir, settings) (ServerRuntimeSettingsUpdateResult, error)`：除既有枚举/广播频率校验外，`MaxPlayers` 在提交时必须为 `1~100`；旧客户端省略时保留原值。成功只覆盖四个目标 key，**保留** `server-settings.json` 里其它已有字段，并以安全原子写替代直接覆盖写。这个函数可以在存档已存在、服务器随时运行/停止的情况下调用，不要求先新建存档。
- 新增 Web 层 `backend/internal/web/server_runtime_settings_handlers.go`：`handleInstanceServerRuntimeSettings` 处理 `GET/PUT /api/instances/:id/config/server-runtime-settings`，完全照抄 `server_password_handlers.go` 的 `handleInstanceServerPassword` 结构（`requireAdmin` → `loadInstance` → 读/校验/写 → 审计日志），PUT 成功后写审计 `instance_server_runtime_settings_update`（metadata 记录锁内取得的 `maxPlayers`/`previousMaxPlayers` 及小屋策略）。路由已接入 `instance_handlers.go`，紧跟在 `config/server-password` 分支之后。
- 这组设置和 `SERVER_PASSWORD` 一样，只在 JunimoServer `server` 容器**启动时**读取 `server-settings.json`，保存后必须重启服务器容器才会生效——前端已在弹窗里明确提示。
- 影响文件：`backend/internal/games/registry/types.go`、`backend/internal/games/stardew_junimo/saves.go`、`backend/internal/games/stardew_junimo/saves_test.go`、`backend/internal/web/server_runtime_settings_handlers.go`（新增）、`backend/internal/web/instance_handlers.go`。前端改动见 `docs/frontend-handoff/frontend-handoff-2026-07-10.md`。
- 验证：`cd backend; go build ./... && go vet ./... && go test ./...` 全绿（新增 `TestWriteServerSettings_CabinMode*`、`TestServerRuntimeSettings_*` 系列测试；同时修正了两处直接调用 `validateCfg` 而未设置 `CabinMode` 的既有测试）。
- 下一步注意事项：`ExistingCabinBehavior` 目前在新建存档阶段永远写 `KeepExisting`（因为新档没有"已有小屋"概念），只有通过服务器控制页的 `UpdateServerRuntimeSettings` 才能真正改成 `MoveToStack`；如果以后要在新建存档页也暴露这个字段，需要重新评估其语义是否适用于"从零创建"场景。

# APPROVE-PENDING-AUTH-1 批准待认证玩家（反射 JunimoServer PasswordProtectionService）

- 需求来源：`SERVER_PASSWORD` 密码保护开启后，玩家连接会被限制在隔离小屋，必须在游戏内输入 `!login <password>` 才能真正进入农场。用户提出希望管理员能从面板直接批准某个待认证玩家，不需要玩家自己正确输入密码。
- 调研结论：`docs/10-junimo-rest-api.md` 3.5 节确认 JunimoServer 官方 REST API 关于密码保护只有 `GET /auth`（五个计数字段）和 `POST /auth/timeout`，**没有**批准/待认证玩家名单接口。模拟聊天指令这条路（`FESTIVAL-EVENT-1`/`JOJA-ROUTE-1` 用过的模式）对本功能**不适用**：`!login` 按"消息来源 FarmerID"鉴权，主机在聊天框输入指令无法代替其他玩家认证，且主机本身完全绕过认证检查。已阅读上游源码 `PasswordProtectionService.cs`（`E:\codex\junimo-server-upstream\mod\JunimoServer\Services\PasswordProtection\`）确认：`TryAuthenticate(long playerId, string password)`/`IsPlayerAuthenticated(long playerId)` 是 public 方法，可反射调用；单例存放在 `private static PasswordProtectionService _instance` 字段（无公开 getter）；密码来自 `Environment.GetEnvironmentVariable("SERVER_PASSWORD")`，控制模组和 JunimoServer 运行在同一进程，可直接读同一环境变量，不需要额外反射。因此**反射调用 `TryAuthenticate` 是唯一可行路径**，这是控制模组第一次真正反射进 JunimoServer 的私有实现（而非公开契约），存在"JunimoServer 升级后悄悄改字段名/方法签名导致反射失效"的固有风险。
- **反射桥（`embedded/smapi-mod-src/PasswordProtectionBridge.cs`，新增）**：独立文件隔离反射逻辑。`Initialize(IMonitor)` 只在 `ModEntry.OnGameLaunched` 里、`isJunimoRuntime` 判定为真之后调用一次：遍历 `AppDomain.CurrentDomain.GetAssemblies()` 找程序集名为 `JunimoServer` 的已加载程序集（不新增编译期引用，纯运行时反射），逐步定位类型 → 静态字段 `_instance` → `TryAuthenticate(long,string)`/`IsPlayerAuthenticated(long)` 方法 → `AuthenticationResult` 的 `Success`/`Message`/`ShouldKick` 三个属性，每一步失败都记录具体哪一步失败，全程 try/catch 不上抛异常。`Available`/`Detail` 供外部读取自检结果——**这是启动时一次性判定，只能证明"反射链路建立成功"，不能保证"每次调用都成功"或"内部行为语义不变"**。
- `ModEntry.cs` 改动：新增 `passwordBridge` 字段并在 `OnGameLaunched` 里初始化；`WriteStatus` 构造 `RuntimeStatus` 时追加 `PasswordBridgeAvailable`/`PasswordBridgeDetail`（不改 `WriteStatus` 签名，所有调用点自动带上最新自检结果）；`WritePlayers()`/`BuildPlayerInfo` 为每个在线玩家追加 `IsAuthenticated`（`bool?`，host 直接视为 `true`，其余玩家反射调用 `IsPlayerAuthenticated`，桥不可用或单次查询失败时为 `null`，序列化时因 `WhenWritingNull` 自动省略）；`HandleCommand` 新增 `case "approve-auth"` → `ApproveAuth(string uniqueMultiplayerId)`，完全模仿现有 `KickPlayer` 的校验结构（`Context.IsWorldReady` → 桥可用性 → `long.TryParse` → 在线查找 → 排除 host → try/catch 调用 `TryAuthenticate` → `WriteStatus` 反馈）。`ShouldKick`（超限踢出）字段有意丢弃，交给 JunimoServer 内部的 `TryAuthenticate` 自行处理，模组侧不重复实现。
- `ControlContract.cs` 新增字段：`RuntimeStatus.PasswordBridgeAvailable`(bool)/`PasswordBridgeDetail`(string)；`PlayerInfo.IsAuthenticated`(`bool?`)。
- **已重新编译并替换嵌入 DLL**（见下方"如何验证"）。
- Go 后端：`players.go` 的 `PlayerInfo`/`playerCacheItem`/`controlPlayersFile` 新增 `IsAuthenticated *bool`（用指针而非 `bool`，区分"旧 DLL 没有该字段"和"确实未认证"，避免旧实例被误判为全员待认证）；`mergePlayerRoster` 从本次在线快照透传该字段，`playerInfoFromCacheItem` 在 `status != "online"` 时强制置 `nil`——"待认证"是瞬时在线状态，不进入离线花名册的长期缓存语义。
- 新增 `player_auth.go`：`readPasswordBridgeStatus(dataDir)` 只读 `control/status.json` 里模组写入的 `passwordBridgeAvailable`/`passwordBridgeDetail`（文件/字段缺失都返回 `Available:false`，不报错），独立于 `lifecycle.go` 现有的 `readSMAPIStatus`（后者只关心 `state` 字段用于生命周期日志）；`approveAuth(instance, uniqueMultiplayerID)` 完全模仿 `kickPlayer`：校验非空 → 校验 running → 校验反射桥可用性（不可用返回 `CommandError{Code:"password_bridge_unavailable"}`，防御性校验，不能只依赖前端提前禁用）→ `writePanelCommand(..., "approve-auth", ...)` → 返回 fire-and-forget 的乐观提示。`Driver.ApproveAuth` 薄封装供 web 层调用。
- `auth_status.go`：`AuthStatusResult` 新增 `PasswordBridgeAvailable`/`PasswordBridgeDetail`；`GetAuthStatus` 只在 REST 调用成功路径末尾追加读取 `readPasswordBridgeStatus`，不改动现有错误分支语义（服务器未运行/REST 未就绪时整体请求仍失败，前端按"待认证功能不可用"处理）。
- Web 层：新增 `players_handlers.go` 的 `handlePlayerApproveAuth`（`POST /api/instances/:id/players/approve-auth`），完全照抄 `handlePlayerKick` 骨架，`CommandError.Code` 映射 HTTP 状态（`server_not_running`/`password_bridge_unavailable` → 409，`not_supported` → 501）；审计日志 `player_approve_auth`（metadata: `uniqueMultiplayerId`）。路由接入 `instance_handlers.go`，紧邻 `players/kick` 分支。
- 影响文件：`backend/internal/games/stardew_junimo/embedded/smapi-mod-src/PasswordProtectionBridge.cs`（新增）、`ControlContract.cs`、`ModEntry.cs`、`embedded/smapi-mod/StardewAnxiPanel.Control.dll`、`backend/internal/games/stardew_junimo/players.go`、`player_auth.go`（新增）、`auth_status.go`、`backend/internal/web/players_handlers.go`、`instance_handlers.go`。前端改动见 `docs/frontend-handoff/frontend-handoff-2026-07-10.md` 的 `APPROVE-PENDING-AUTH-1` 小节。
- 验证：新增 `player_auth_test.go`（`readPasswordBridgeStatus` 文件缺失/字段解析、`approveAuth` 未运行/桥不可用/写命令文件三条路径）、`auth_status_test.go`（`GetAuthStatus` 成功路径正确合并反射桥字段）、`players_test.go` 新增 `TestReadPlayersFromControlParsesIsAuthenticated`（true/false/字段缺失三种输入）和 `TestListPlayersOfflinePlayersHideIsAuthenticated`（玩家下线后 `isAuthenticated` 强制 `nil`）。`cd backend; go build ./... && go vet ./... && go test ./...` 全绿。SMAPI Mod 用文档命令重新编译（`dotnet build -c Release /p:GamePath=/game`），`Build succeeded. 0 Errors`（仅历次已知的 ModBuildConfig analyzer 版本 warning），编译产物已 md5 校验复制覆盖嵌入 DLL。前端 `cd frontend; npx tsc --noEmit -p . && npm run build` 通过。
- **未做的验证**：没有在开启 `SERVER_PASSWORD` 的真实运行实例上做端到端联机测试（客户端连接但不 `!login` → 确认 `isAuthenticated:false` 且反射桥自检为 `true` → 点击批准 → 确认玩家真的被传送出隔离小屋）。反射桥的自检机制只能验证"启动时类型/字段/方法能被找到"，不能验证"调用后行为符合预期"，强烈建议下一位维护者找测试实例走一遍完整链路，并顺带验证"批准已离线/不存在玩家"和"重复批准已认证玩家"两个边界不崩溃。
- 下一步注意事项：
  - 反射目标是 JunimoServer **私有静态字段**和未公开为稳定契约的内部服务，非公共 API。JunimoServer 后续版本一旦重命名/重构该单例、改方法签名，反射会静默失效——`PasswordBridgeAvailable` 能捕捉"启动时反射链路建立失败"，但**不能**保证"每次调用都成功"或"内部行为语义不变"。如果以后 JunimoServer 升级后这个功能突然不生效，第一步应该检查 `control/status.json` 的 `passwordBridgeDetail` 诊断信息，定位是类型找不到、字段找不到还是方法签名不匹配。
  - 和 kick/say/`!event`/`!joja` 一样是 fire-and-forget，前端只能提示"指令已提交"；这次评估过是否要顺带做"命令文件带 id + `command-results/<id>.json`"精确反馈通道（历史文档多次提到但故意没做），最终决定不做，因为批准操作的实际失败面很窄（基本只有"反射桥不可用"这一种，而这一种已经被启动期自检覆盖，不需要逐次反馈）。
  - 待认证玩家能否被面板看到，依赖控制模组已经运行过至少一次 `WritePlayers()`（`OnUpdateTicked` 每 120 tick 一次，`Context.IsWorldReady` 为真时才写）；如果玩家刚连接、下一次 tick 还没跑到，前端"待认证玩家"卡片会有几秒钟的延迟可见，这是预期行为不需要额外处理。

# PLAYERS-BAN-1 封禁玩家（复用 JunimoServer `!ban` 聊天指令）+ 玩家行操作按钮精简

- 需求来源：用户希望精简"在线玩家"表格每行的图标按钮（原本 3 个：恒禁用的"发送消息"占位、可用的"踢出"、恒禁用的"更多操作"占位），只保留"踢出"并换成"管理操作"卡片同款真实 PNG 图标，旁边加一个新按钮。最初讨论方向是"取消认证"（`APPROVE-PENDING-AUTH-1` 的反操作），调研 `PasswordProtectionService.cs` 后确认认证状态是纯内存运行时字典、断线重连必然重置为未认证、没有持久化——这意味着"踢出"本身已经语义等价于"取消认证"，额外做一个"原地撤销认证但不踢人"的反射功能风险更高（要新增对 JunimoServer 私有可变字典的写操作）且实际效果很差（`AuthTimeoutSeconds>0` 时几乎立刻被自动踢出，`=0` 时玩家会静默卡死无法移动/聊天）。用户确认放弃这个方向，转而调研 JunimoServer 是否有真正的封禁能力。
- 调研结论：JunimoServer 上游有真正的 `!ban <名字>`/`!unban <id|名字>`/`!listbans` 聊天指令（`Services/Commands/BanCommand.cs`/`UnbanCommand.cs`/`ListBansCommand.cs`），机制和已实现的 `!joja` 完全一致——要求触发者持有 admin 角色（`RoleService.IsPlayerAdmin`），底层调用 vanilla `Game1.server.ban(uniqueMultiplayerID)`，写入 `Game1.bannedUsers`；`IsServerHost` 会被上游自己拒绝并提示"You can't ban the server host."，host 保护已由上游保证。匹配目标用 `Game1.getAllFarmers().FirstOrDefault(f => f.Name == name || GetFarmerUserNameById(id) == name)`——按游戏内农场主角色名或 Steam 用户名匹配，同时包含在线和离线的 farmhand，所以离线玩家也能被封禁。这正好对应玩家管理页"管理操作"卡片里已经存在但一直禁用、标着"待接入"的"封禁玩家"卡片。
- **已确认限制**：用户已在真实实例人工验证 `Game1.bannedUsers` 随服务器容器重启丢失。当前没有面板侧持久化补偿（不新增数据库表、不做启动后自动重新封禁），UI 明确提示重启后需要重新操作。
- **实现方式**：
  - `embedded/smapi-mod-src/ModEntry.cs` 的 `HandleCommand` 新增 `case "ban":`，解析 `payload["name"]`，调用新方法 `BanPlayer(string name)`：`Context.IsWorldReady && Game1.chatBox is not null` 检查 → `SanitizeChatText(name, 60)` 防御性清理 → `Game1.chatBox.textBoxEnter($"!ban {name}")`——完全模仿 `EnableJojaRoute()` 的"模拟聊天指令"结构（不是模仿 `KickPlayer` 的直接调用结构），模组侧不做 admin 提权，提权由 Go 后端先完成。
  - 新增 `backend/internal/games/stardew_junimo/ban.go`：`Driver.BanPlayer(ctx, instance, name, uniqueMultiplayerID)` → 核心 `banPlayer`：校验名字非空 → 校验实例 running → **完全复用** `joja.go` 已有的 `findHostPlayerID`/`callRolesAdminAPI`（这两个函数本来就是包级别可复用，不是 Joja 专属）把主机提升为 admin → `writePanelCommand(dataDir, "ban", {"name":...})`。返回文案明确写"如果服务器容器重启，此封禁可能失效，需要重新操作"。
  - Web 层新增 `players_handlers.go` 的 `handlePlayerBan`（`POST /api/instances/:id/players/ban`），完全照抄 `handlePlayerKick` 骨架，`CommandError.Code` 映射：`server_not_running`/`host_unknown`/`junimo_api_unavailable` → 409，`invalid_player` → 400，`not_supported` → 501；审计日志 `player_ban`（metadata: `name`、`uniqueMultiplayerId`）。路由接入 `instance_handlers.go`，紧邻 `players/kick`/`players/approve-auth` 分支。
  - **已重新编译并替换嵌入 DLL**。
- 影响文件：`backend/internal/games/stardew_junimo/embedded/smapi-mod-src/ModEntry.cs`、`embedded/smapi-mod/StardewAnxiPanel.Control.dll`、`backend/internal/games/stardew_junimo/ban.go`（新增）、`backend/internal/web/players_handlers.go`、`instance_handlers.go`。前端改动见 `docs/frontend-handoff/frontend-handoff-2026-07-10.md` 的 `PLAYERS-BAN-1` 小节。
- 验证：新增 `ban_test.go`（空名字拒绝、服务器未运行拒绝、`findHostPlayerID` 找不到主机返回 `host_unknown`、提权成功后正确写入 `ban` 命令文件）。`cd backend; go build ./... && go vet ./... && go test ./...` 全绿。SMAPI Mod 用文档命令重新编译，`Build succeeded. 0 Errors`，编译产物已 md5 校验复制覆盖嵌入 DLL。前端 `cd frontend; npx tsc --noEmit -p . && npm run build` 通过。
- **后续验证结论**：用户已实际验证封禁在容器重启后丢失；新版 command-result 实现已验证 broadcast succeeded 与 ban 主机保护，ban succeeded 仍待有在线测试 farmhand 时补验。
- 下一步注意事项：
  - 上游 `FindPlayerIdByFarmerNameOrUserName` 用 `FirstOrDefault` 按名字匹配，两个玩家重名时可能封错人，这是 JunimoServer 自身已知限制（其仓库 `.claude/plans/audit-security.md` 记录），面板侧无法修复。
  - 没有实现 `!unban`/`!listbans` 对应的"解封"管理界面——封禁操作失误目前只能等服务器重启（如果确认不持久化）或手动进入游戏聊天框输入 `!unban <id|名字>`。这是本次按"先做简单接通"明确排除的范围，如果后续反馈需要撤销入口，是自然的后续迭代。
  - 和 kick/say/`!event`/`!joja` 一样是 fire-and-forget，前端只能提示"指令已提交"，拿不到"目标是否真的被封禁成功"这类精确反馈。

# PLAYERS-WARP-HOME-1 玩家回家传送

- 新增 `POST /api/instances/:id/players/warp-home`，管理员专用。请求体为 `{ "uniqueMultiplayerId": string, "name"?: string }`，后端要求实例处于 `running`，并要求嵌入式控制模组启动自检确认 `warpHomeBridgeAvailable=true`。
- 该能力不复用 `PasswordProtectionService.TryAuthenticate`。`TryAuthenticate` 对已认证玩家会直接返回 `Already authenticated`，不会再次传送；认证流程里的 `WarpToDestination` 是 private，且绑定 lobby 清理语义。当前实现改为让 `StardewAnxiPanel.Control` 通过 `WarpHomeBridge` 反射 JunimoServer 程序集中的 `JunimoServer.Util.FarmerExtensions.WarpHome(Farmer)` 公共扩展方法。
- 控制模组新增 `warp-home` panel command：解析目标 `uniqueMultiplayerId`，在当前在线玩家中查找目标，拒绝 host（host 没有 farmhand cabin 可回），然后调用上游 `farmer.WarpHome()`。该调用和 JunimoServer 自己在非主机进入主机屋子时使用的回家逻辑一致。
- Go driver 新增 `Driver.WarpPlayerHome(ctx, instance, uniqueMultiplayerID, name)`，核心流程为：校验玩家 ID -> 校验 running -> 读取 `.local-container/control/status.json` 的 `warpHomeBridgeAvailable/detail` -> 写入 `.local-container/control/commands/*.json` 命令文件 `{ name:"warp-home", payload:{ uniqueMultiplayerId, name } }`。响应仍是 fire-and-forget，只表示控制命令已提交。
- Web 层错误映射：`invalid_player` -> 400，`server_not_running` / `warp_home_bridge_unavailable` -> 409，`not_supported` -> 501；成功写审计日志 `player_warp_home`，metadata 包含 `uniqueMultiplayerId` 和 `name`。
- 影响文件：`backend/internal/games/stardew_junimo/player_warp.go`、`player_warp_test.go`、`embedded/smapi-mod-src/WarpHomeBridge.cs`、`ControlContract.cs`、`ModEntry.cs`、`embedded/smapi-mod/StardewAnxiPanel.Control.dll`、`backend/internal/web/players_handlers.go`、`instance_handlers.go`。
- 验证：`go test ./internal/games/stardew_junimo ./internal/web` 通过；SMAPI 控制模组用 Docker `dotnet build -c Release /p:GamePath=/game` 重新编译通过，并已复制覆盖嵌入 DLL。尚未做真实多人联机端到端验证，后续应在在线 farmhand 上点击”回家”确认玩家被传送到自己小屋入口。

# SAVE-BACKUP-GAMEDAY-1 存档回档重构：游戏内日期驱动的自动回档点（取代 SAVE-BACKUP-POLICY-1 / SAVE-BACKUP-SCHEDULE-HOUR-1 的定时备份部分）

- **背景**：`SAVE-BACKUP-POLICY-1`/`SAVE-BACKUP-SCHEDULE-HOUR-1` 按”现实时间”管理三类自动备份（`latest_<save>.zip` 游戏保存后覆盖、`scheduled_<save>.zip` 每天到点覆盖、`daily_<save>_<YYYYMMDD>.zip` 按现实自然日保留 N 天）。用户要求彻底改为按”游戏内日期”（年/季/日）管理自动回档点：取消定时备份，自动回档点按游戏日序号排序和去重覆盖，默认保留最近 5 个不同游戏日（可设 1-14），回档前必须有保护备份且不能被自动清理误删。
- **关键前提（未改 SMAPI Mod，未重新编译嵌入 DLL）**：触发时机”游戏内睡觉并成功保存、存档已经落盘”由现有 `ModEntry.cs` 的 `OnSaved`（SMAPI `GameLoop.Saved`，在存档写盘完成后触发）已经满足，事件写入 `.local-container/control/save-events/*.json`，后端 `RunBackupMaintenance()` 消费即可；游戏年/季/日已由 `readSaveInfo`/`fillSaveInfoFromXML` 解析为 `GameYear`/`GameSeason`/`GameDay`，只需新增一个游戏日序号换算函数。
- **`BackupPolicy` 简化**（`saves.go`）：
  ```go
  type BackupPolicy struct {
      GameSaveBackups bool `json:”gameSaveBackups”`
      RetainGameDays  int  `json:”retainGameDays”`
  }
  ```
  删除 `DailySnapshots`/`DailyRetentionDays`/`ScheduledBackups`/`ScheduledHour`/`ScheduledIntervalHour`。默认 `GameSaveBackups=true, RetainGameDays=5`；`normalizeBackupPolicy` 把 `RetainGameDays` clamp 到 `[1,14]`（`<=0` 时回落默认 5）。旧 `policy.json` 里 `scheduledBackups`/`scheduledHour`/`dailySnapshots`/`dailyRetentionDays` 等字段被 `encoding/json` 静默忽略、不报错；`gameSaveBackups` 字段名未变，值自动延续。
- **游戏日序号**：新增 `gameDayOrdinal(year, season, day) = (year-1)*112 + seasonIndex(season)*28 + day`（`seasonIndex`: spring=0/summer=1/fall|autumn=2/winter=3），保证跨季节、跨年份正确排序。`BackupInfo` 新增 `GameDayOrdinal int json:”gameDayOrdinal,omitempty”`，由 `enrichBackupInfo` 一并算出返回给前端，避免前端重复实现季节序号映射。
- **备份文件命名与分类**（四类新前缀 + 三类历史前缀）：
  - `BackupManual(dataDir, saveName)` → `manual_<save>_<timestamp>.zip`：管理员手动备份、服务器页”备份已保存进度”快捷操作、计划重启关闭前备份统一改调这个（原来都直接调裸 `BackupSave`）。
  - `BackupPreDelete(dataDir, saveName)` → `predelete_<save>_<timestamp>.zip`：`DeleteSaveWithBackup` 内部改调。
  - `BackupPreRestore(dataDir, saveName)` → `prerestore_<save>_<timestamp>.zip`：`RestoreBackup` 覆盖前保护备份改调，失败仍然中止恢复（行为不变，只是换了前缀名）。
  - `BackupAutoGameDay(dataDir, saveName)`：读当前存档目录（不是 ZIP）算出 `gameDayOrdinal`，目标名 `auto_<save>_<ordinal六位补零>.zip`；同一 ordinal 被 `backupSaveAs` 的改名覆盖语义自然覆盖旧文件——这同时满足”同一游戏日多次保存覆盖”和”回档到早期再重新游玩到相同日覆盖旧点”两条需求，不需要额外状态文件判断。存档日期解析失败（`GameYear<=0`/`GameDay<=0`）时返回错误，不生成 ordinal=0 的脏文件。
  - `PruneAutoGameDayBackups(dataDir, saveName, retainGameDays)`：按 `auto_<save>_` 前缀枚举文件，解析出 ordinal 排序（不看文件 mtime），只保留最大的 N 个。
  - 删除（scheduled 机制整体移除）：`BackupLatest`/`BackupScheduled`/`BackupDailySnapshot`/`PruneDailySnapshots`/`dailySnapshotDate`/`runScheduledBackupIfDue`/`scheduledBackupDue`/`readBackupMaintenanceState`/`writeBackupMaintenanceState`/`backupMaintenanceState` 类型（`maintenance-state.json` 不再需要，游戏日序号本身就是确定性状态）。
  - `inferBackupKind`/`parseBackupSaveName` 新增 `auto_`/`manual_`/`predelete_`/`prerestore_` 前缀识别；保留旧 `latest_`/`daily_`/`scheduled_` 前缀识别（不再产生新文件，但已有 ZIP 继续被识别，交给前端归入”历史备份”，不做任何删除）。
- **`RunBackupMaintenance` 重写**：遍历 save-events，若 `policy.GameSaveBackups` 为真则对每个事件调用 `BackupAutoGameDay` + `PruneAutoGameDayBackups(policy.RetainGameDays)`，不再有 scheduled 分支。调用方 `handleSavesBackupsList` 不变（仍在每次 `GET .../saves/backups` 时触发维护）。
- **临时文件命名的连带修复**：重构过程中发现 `backupSaveAs`（`BackupLatest`/`BackupScheduled`/`BackupDailySnapshot` 时代就存在的”先建临时文件再改名覆盖”辅助函数）依赖 `BackupSave` 自带的秒级时间戳临时文件名（`<saveName>_<timestamp>.zip`）。如果这个临时名恰好和另一个正被打开读取的备份 ZIP 同名（例如 `RestoreBackup` 里”恢复源 ZIP”和”覆盖前保护备份”的临时文件在同一秒内先后创建，两者都用裸 `<saveName>_<timestamp>.zip` 命名模式时），Windows 下会因文件被占用而 `rename` 失败（测试中实际复现），理论上在其它平台会静默覆盖/损坏那份被打开的 ZIP。已抽出 `writeSaveZip(dataDir, saveName, backupName)` 核心逻辑，`backupSaveAs` 改用纳秒时间戳 + 目标名拼出的 `.tmp-*` 临时名，彻底消除这个此前已存在、和本次重构本身无关但被新增测试意外触发暴露的命名碰撞窗口。
- **Web / 调度器调用点**：`lifecycle_handlers.go: handleSaveBackupCreate` 改调 `sj.BackupManual`；`restart_schedule_handlers.go: backupActiveSave` 改调 `sj.BackupManual`（关闭前备份归入”手动备份”桶，不占用自动回档配额）。`handleSavesBackupPolicy`/`handleSavesBackupsList`/`handleSavesBackupRestore`/`handleSavesBackupDelete` 路由和权限逻辑不变，`ensureInstanceNotRunning` 继续在恢复前拦截运行中的实例（`409 server_running`）。
- **兼容策略**：旧 `policy.json` 含废弃字段不报错；已有 `latest_*.zip`/`daily_*_*.zip`/`scheduled_*.zip` 磁盘文件不删除、不迁移，`ListBackups`/恢复/删除接口 URL 和参数不变，只是 `kind`/`policy` 的取值集合变化。
- 影响文件：`backend/internal/games/stardew_junimo/saves.go`、`saves_test.go`、`backend/internal/web/lifecycle_handlers.go`、`backend/internal/web/restart_schedule_handlers.go`。前端改动见 `docs/frontend-handoff/frontend-handoff-2026-07-11.md` 的 `SAVE-BACKUP-GAMEDAY-1` 小节。
- 验证：新增/替换测试覆盖默认保留 5 个游戏日、legacy 字段兼容、跨季节跨年份排序（`TestGameDayOrdinal_CrossSeasonAndYear`）、同游戏日多次保存覆盖（`TestBackupAutoGameDay_OverwritesSameGameDay`）、回档到早期后重新游玩相同日覆盖旧点（`TestBackupAutoGameDay_RestoreEarlierThenReplaySameDayOverwrites`）、清理保留最近 5 个游戏日（`TestPruneAutoGameDayBackups_KeepsFiveMostRecentGameDays`）、自动清理不误删手动/保护备份（`TestRunBackupMaintenance_DoesNotTouchManualOrProtectionBackups`）、保护备份成功创建（`TestRestoreBackup_CreatesPreRestoreProtectionBackup`）、保护备份失败时中止回档且不破坏当前存档（`TestRestoreBackup_AbortsWhenProtectionBackupFails`）。`cd backend; go build ./... && go vet ./... && go test ./...` 全绿。
- **未做的验证**：没有连接真实运行中的 JunimoServer 实例走一遍”睡觉存档 → 自动回档点生成 → 回档到早期游戏日 → 重新游玩到相同日 → 确认覆盖旧回档点”的完整端到端链路；纯基于单元测试和对 SMAPI `GameLoop.Saved` 文档行为的信任。建议下一位维护者找测试实例验证一次。

## SAVE-BACKUP-GAMEDAY-1 追加修复：回档成功后遗留 `.restore-tmp-*` 临时目录

用户实际使用本次新做的手机端"游戏日回档"功能后反馈：存档库里出现一张解析失败的卡片，名字是 `.restore-tmp-2156104854`，提示"解析失败：未找到 SaveGameInfo 文件"。排查后确认这是 `RestoreBackup`（`saves.go`）里一个**本次重构之前就存在**的 bug，只是这次用户第一次实际点击回档才触发：

- `RestoreBackup` 用 `os.MkdirTemp(savesRoot, ".restore-tmp-*")` 在 `Saves/` 目录**内部**创建一个临时目录，把备份 ZIP 解压进去，再把里面的 `<saveName>` 子目录 `os.Rename` 移动到最终位置。原代码只在**失败**分支（`success == false`）才 `os.RemoveAll(tempDir)`；**成功**分支设置 `success = true` 后直接返回，从未清理这个临时目录本身——子目录被移走后，这个现在已经清空的 `.restore-tmp-*` 目录会永远留在 `Saves/` 里。
- `listSaveDirs`（`ListSaves` 的数据源，供 `GET /api/instances/:id/saves` 使用）此前不过滤目录名，把这个残留的空目录也当成一个"存档"返回给前端，`readSaveInfo` 找不到 `SaveGameInfo` 就报"解析失败"，正是用户截图里看到的现象。

修了两处，双重保险：

1. **根因修复**：`RestoreBackup` 的 `defer` 改为无条件 `_ = os.RemoveAll(tempDir)`（不再判断 `success`）。成功路径下子目录已经被 rename 移出，此时 `tempDir` 必然为空，直接删除安全；失败路径下也一并清理掉可能残留的部分解压内容。删除了不再需要的 `success` 局部变量。
2. **防御性修复**：`listSaveDirs` 跳过所有以 `.` 开头的目录名（`!strings.HasPrefix(e.Name(), ".")`）。这一条对**已经存在**的历史残留目录立即生效——不需要用户手动去 Docker 卷里删文件，重新部署这份代码后，`GET /saves` 就不会再把任何 `.` 前缀目录当存档返回。

影响文件：`backend/internal/games/stardew_junimo/saves.go`、`saves_test.go`。

新增测试：`TestListSaveDirs_SkipsDotPrefixedTempDirs`（手工构造一个 `.restore-tmp-*` 目录，断言 `listSaveDirs` 不返回它）、`TestRestoreBackup_Success` 追加断言（回档成功后扫描 `Saves/` 确认没有任何 `.restore-tmp-*` 残留）。`cd backend; go build ./... && go vet ./... && go test ./...` 全绿。

**注意**：这个修复只解决"以后不再产生新的残留、且不再把已有残留显示成存档"，**不会**主动删除磁盘上已经存在的 `.restore-tmp-*` 孤儿目录（比如用户截图里那个 `.restore-tmp-2156104854`）。这些目录物理上还在 Docker 卷里，只是不再出现在存档列表里，不影响功能，只占用一点磁盘空间；如果要彻底清理，可以手动进入 `.local-container/saves/Saves/` 目录删除对应文件夹，或者不用管它。

## SAVE-RESTORE-AUTORESTART-1 回档时自动停止/重启服务器

- **需求**：此前服务器运行中点击回档，弹窗只会提示"请先停止服务器"并把提交按钮禁用，用户必须离开存档页去服务器页手动停止、再回来重新走一遍回档流程。用户要求：运行中也能直接点，确认后由面板自动完成"停止服务器 → 回档 → 重新启动服务器"整个流程。
- **编排方式**：没有采用"HTTP 请求内阻塞轮询直到停止完成"的方案——`Driver.Stop` 本身是 fire-and-forget（内部提交一个 job 就立刻返回，不等待 `docker compose down` 真正跑完），在 HTTP handler 里自己写轮询等待既不符合这个仓库其它生命周期操作"提交即返回、前端轮询 job"的既有架构，也有代理超时风险。改为把"停止 → 回档 → 启动"三步实现成**同一个 lifecycle job** 内部顺序执行的三个阶段，复用 `lifecycleRunner` 已有的 `doStop`/`doStart` 方法（不是重新实现 compose down/up、Mod 同步、邀请码轮询等逻辑），前端拿到的还是一个普通 jobId，用现有的 job 轮询/SSE 机制展示进度——和"启动服务器"按钮的用户体验完全一致，没有引入任何新的前端等待逻辑。
- **后端改动**（`backend/internal/games/stardew_junimo/lifecycle.go`）：
  - `lifecycleRunner` 新增 `restoreBackupName`/`restoreOverwrite` 字段，`operation` 新增取值 `"restore_restart"`。
  - 新增 `doRestoreAndRestart`：若实例当前是 `running`/`starting`，先调用 `r.doStop(...)`（原样复用，会经历 `stopping → stopped` 两个 phase）；然后调用 `RestoreBackup(dataDir, backupName, overwrite)`（和同步回档路径完全同一个函数，行为一致，包括覆盖前的 `prerestore_` 保护备份和失败即中止）；若之前是运行中，最后调用 `r.doStart(...)`（原样复用，包含 Mod 同步、`docker compose up`、等待就绪、邀请码轮询等全部既有逻辑）。若回档本身失败，直接返回错误、**不会**尝试启动——避免在一个已知损坏的存档状态下把服务器拉起来。
  - 新增 `Driver.RestoreBackupWithRestart(ctx, instance, backupName, overwrite, actorID) (*registry.Job, error)`：和 `Start`/`Stop`/`Restart` 同构，`cancelActiveLifecycleJobs` 清理旧任务后 `d.jobs.Start(...)` 提交一个 `operation: "restore_restart"` 的 job，返回 jobId。
- **Web 层改动**（`backend/internal/web/lifecycle_handlers.go`）：
  - `POST /api/instances/:id/saves/backups/restore` 请求体新增 `autoRestart: boolean`。
  - 逻辑分支：`running/starting` 且 `!autoRestart` → 保持原有 `409 server_running`（不改变现有 API 调用方的行为，仍然要求显式选择自动重启）；`running/starting` 且 `autoRestart` → 通过新增的窄接口 `backupRestoreRestarter`（`interface { RestoreBackupWithRestart(...) }`，和 `festival_handlers.go` 里 `festivalEventTrigger`/`jojaRouteEnabler` 同一套"定义窄接口 + driver 类型断言 + 501 not_supported"风格）调用 `driver.RestoreBackupWithRestart`，返回 `202 {"jobId": ...}`；已停止 → 走原有同步 `sj.RestoreBackup` 路径不变，返回 `200 {"saveName": ...}`。
  - 是否运行的判断从 `ensureInstanceNotRunning`（会直接短路写 409）换成 `reconcileInstanceState` + 手动判断 `instance.State`，因为现在需要在"运行中"这个分支里继续往下走（调用 driver），而不是提前拦截。
- **顺带修复的临时文件命名碰撞发现的连带影响**：无——这次改动不涉及 `saves.go` 的备份命名逻辑，只是复用其中的 `RestoreBackup` 函数本身。
- 影响文件：`backend/internal/games/stardew_junimo/lifecycle.go`、`backend/internal/games/stardew_junimo/lifecycle_test.go`、`backend/internal/games/stardew_junimo/console_test.go`（`fakeConsoleDocker` 补充 `ComposeDown`/`ComposeUp` mock 支持）、`backend/internal/web/lifecycle_handlers.go`、`backend/internal/web/saves_handlers_test.go`。
- **测试与覆盖边界**：
  - `TestRestoreBackupWithRestart_RequiresJobManager`：job manager 未配置时返回错误（和 `Start`/`Stop` 同类防御但此前这两个也没有对应测试，一并补上同等级覆盖）。
  - `TestDoRestoreAndRestart_StoppedSkipsStopAndStart`：已停止时直接回档，不触发 `ComposeDown`/`ComposeUp`，最终 phase 为 `restored`。
  - `TestDoRestoreAndRestart_RunningStopsThenRestoresBeforeStarting`：运行中时先 `ComposeDown` 再回档（回档结果已经能在磁盘上验证到）再尝试 `ComposeUp`；`ComposeUp` 故意返回错误让测试在不用把 `doStart` 完整成功路径（等待容器就绪、邀请码轮询等）全部 mock 出来的前提下，仍能验证"停止 → 回档 → 尝试启动"这个顺序确实生效。
  - `TestSavesBackupRestore_RunningWithoutAutoRestartReturns409`/`_RunningWithAutoRestartBypassesRunningGate`/`_StoppedStillWorksSynchronously`：覆盖 web 层三条分支路由是否正确（不判断 `driver.RestoreBackupWithRestart` 是否真的成功，因为这个仓库的 web 测试历来没有为任何需要真实 game driver 的接口注册过 fake `registry.GameDriver`——`players_handlers.go`、`festival_handlers.go` 等一样没有 HTTP 层测试，只在 `stardew_junimo` 包内测试核心函数——这次保持同样的分层测试策略，不额外引入一个 15 方法的 fake driver）。
  - **未做的验证**：没有连接真实运行中的 JunimoServer 实例走一遍"运行中点击回档 → 确认 → 观察服务器真的自动停止、回档、重新启动、玩家能重新连进正确的存档"这个完整链路。`doStart`/`doStop` 本身在这个代码库里也一直没有端到端自动化测试（只能靠真机验证），这次新增的编排逻辑继承了同样的验证空白，建议下一位维护者用测试实例走一遍。
# PLAYER-OFFLINE-SAVE-FALLBACK-1 离线玩家存档信息兜底与缓存身份兼容

- 玩家名册合并现在会从当前存档 XML 的 Farmer 数据为离线玩家补齐 `lastSleepLocation`、`lastSleepPoint`、钱包模式和收入信息；前端现有“位置”列无需修改即可展示存档中的最后睡眠位置与坐标。
- 存档信息仅作为缺失字段兜底：如果 `players-cache.json` 已记录更准确的运行时位置、坐标或收入，不会被存档值覆盖。
- 独立钱包玩家会用存档中的 `totalMoneyEarned` 兜底 `personalIncome`；共享钱包仍不虚构个人收入。
- 玩家缓存的存档身份比较兼容 `FarmName` 与 `FarmName_<数字ID>` 两种形式，避免控制文件在基础 saveId 和完整存档目录名之间切换时整份丢弃历史名册。非数字后缀不会被误判为同一存档。
- 影响文件：`backend/internal/games/stardew_junimo/players.go`、`players_test.go`。API 字段和 URL 均未变化。
- 验证：`cd backend; go test ./internal/games/stardew_junimo/...; go test ./...` 全部通过。
# REAL-INSTANCE-CRITICAL-FLOWS-VERIFIED-1 关键流程真实实例验证标记

- 用户已确认在真实实例完成：大存档启动并等待主机上线、运行中回档自动停止/重启、多人认证/踢出/封禁/回家、睡觉后自动生成游戏日回档点、Steam 授权及镜像候选降级。
- 本标记取代历史接手记录中这些流程的“尚未真机验证”说明；未被列入上述范围的视觉、边界条件或其它功能验证状态不变。
# UI-LIFECYCLE-STATUS-1 后端统一生命周期状态（2026-07-11）

- `GET /api/instances/{id}/state` 新增 `uiStatus` 与 `uiStatusUpdatedAt`，状态固定为 `stopped / starting_container / loading_save / waiting_for_host / ready / stopping / failed`。
- 聚合顺序为实例错误、停止阶段、活动生命周期 job、实例启动状态、SMAPI `status.json` 的 `save-loaded`、`players.json` 主机在线；前端不再组合这些来源决定生命周期按钮状态。
- 同一响应只读暴露 `statusSource`、`playersSource`（含 saveId、更新时间和原始状态摘要），用于诊断页核对数据新鲜度。
- `runtimeDiagnostic` 进一步提供当前存档目录、`players.json` 快照 saveId 与身份匹配、控制模组版本、配置的 Junimo 镜像与测试版本匹配，以及容器→存档、存档→主机两段耗时。`status.json` 是阶段事件而非心跳；耗时只在时间戳有效、顺序正确且间隔不超过 30 分钟时返回，避免跨启动轮次误算。
# PLAYER-ROSTER-SQLITE-1 玩家正式数据模型

- migration `008_player_roster.sql` 新增 `save_identities` 与 `player_roster`。玩家联合身份为 `(instance_id, stable_save_id, player_id)`，其中 `player_id` 优先使用 `UniqueMultiplayerID`，缺失时暂用 `name:<lower-name>`，后续获得正式 ID 会合并临时记录。
- `stable_save_id` 优先解析为完整存档目录名（例如 `Farm_123`）；基础 `saveId` 只写入 `base_save_id` 作为兼容别名，避免同名新存档长期共用历史。
- `player_roster` 保存首次出现、最后观测、最后在线、最后位置/坐标、收入快照、钱包模式、角色、主机标记和数据来源。运行时在线快照优先于存档睡眠位置；缺字段时才使用已有数据库值。
- 同一 migration 还新增 `player_events`；`player_roster.current_status` 驱动 `seen/joined/left` 状态跃迁，`recentEvents` 直接从 SQLite 按时间倒序读取。
- `Driver.ListPlayers()` 仍读取 `players.json` 和存档 XML 作为事实输入，但返回前会同步 SQLite 并用数据库补齐离线历史。旧 `players-cache.json`、`players-events.json` 只在升级后的首次读取中作为兼容导入源；名册和事件全部成功写库后一起删除，后续不再写这些历史 JSON。
- `GET /api/instances/:id/players` URL 和 JSON 响应结构不变；`saveId` 现在尽量返回归一化后的完整存档目录 ID，数据库补齐的离线玩家 `source=sqlite_roster`。

验证：`cd backend; go test ./...`。存储层测试覆盖历史时间、最新快照、玩家改名、临时身份晋升和 seen/left/joined 事件跃迁；driver 集成测试覆盖旧名册/事件导入、JSON 退役、基础/完整存档 ID 归一化和 SQLite 离线恢复。
# COMMAND-RESULT-PROTOCOL-1 控制命令执行结果回执（阶段 1）

- `writePanelCommand` 现在生成 32 位小写十六进制 `commandId`（128-bit 随机值），同时写入命令 JSON 的 `id` 和文件名；命令经同目录临时文件写入、`fsync`、原子 rename 后才对控制模组可见。
- 共享目录新增 `control/command-results/`。结果文件名为 `<commandId>.json`，契约为 `{commandId,status,errorCode?,message?,createdAt,updatedAt,details?}`；状态集合固定为 `queued/running/succeeded/failed/dispatched/expired/unknown`，错误判断使用 `errorCode`，禁止解析英文日志。
- 内嵌 `StardewAnxiPanel.Control` 的 `PanelCommand` 新增 `Id`，新增 `CommandOutcome`/`CommandStatuses` 和公共原子 JSON 写入方法。消费顺序为：已有同 ID 结果则删除残留命令且不重复执行；否则先原子写 `running`，再调用返回 `CommandOutcome` 的 `HandleCommand`，最后原子写终态回执，只有成功后才删除命令文件。阶段 1 的既有命令统一返回 `dispatched`，未改变踢出、回家、认证、喊话等游戏行为。
- 该顺序刻意关闭自动重试：若执行完成后、终态回执落盘前崩溃，磁盘保留 `running` 闸门，重启后不会重复执行；超过 5 分钟由后端转为 `unknown / execution_interrupted`。这是协议已知的崩溃窗口，不能据此自动重放命令。
- `status.json` 新增 `commandResultVersion: 1`。新版实例提交文件队列命令时响应新增 `commandId` 和 `status: queued`；旧模组没有能力标志时仍返回原 `output`“已提交”文案并省略 `status`，保持滚动升级兼容。HTTP 请求从不等待模组。
- 新增 `GET /api/instances/:id/commands/:commandId`。driver 先读结果文件；无结果但命令文件存在返回 `queued`；两者都不存在返回 `unknown`。非法 ID 返回结构化 `invalid_command_id`，读取失败返回 `command_result_read_failed`。
- 清理策略：终态结果保留 7 天，之后原子改写为 `expired / result_expired` 墓碑并再保留 24 小时，最后删除；`queued` 和未超过 5 分钟的 `running` 永不清理，避免删除仍在处理的命令。
- 验证：`go test ./internal/games/stardew_junimo ./internal/web`；`go test ./...`。SMAPI 模组用 Docker + `/p:GamePath=/game` 编译成功（0 errors，1 个既有 analyzer warning），并已替换 `embedded/smapi-mod/StardewAnxiPanel.Control.dll`。
# PLAYER-COMMAND-RESULTS-1 三条玩家命令精确回执

- command result v1 的阶段 2 已只接入 `warp-home`、`kick`、`approve-auth`。`HandleCommand` 对这三条直接返回各自处理函数产生的 `CommandOutcome`，不再把全局 `status.json` 最后一条消息当作命令结果；其他文件队列命令仍保持 `dispatched`。
- 成功统一为 `succeeded / ok`，并在 `details` 返回 `playerId/playerName`。明确失败统一为 `failed`：warp-home 覆盖 `world_not_ready`、`bridge_unavailable`、`invalid_player_id`、`player_not_online`、`host_not_supported`、`warp_failed`；kick 覆盖 `world_not_ready`、`invalid_player_id`、`player_not_online`、`host_not_supported`、`kick_failed`；approve-auth 覆盖 `world_not_ready`、`bridge_unavailable`、`invalid_player_id`、`player_not_online`、`host_not_supported`、`already_authenticated`、`authentication_rejected`、`authentication_failed`。
- `PasswordProtectionBridge.TryAuthenticate` 新增 `InvocationFailed` 维度：JunimoServer 明确返回 `Success=false` 映射为 `authentication_rejected`，反射异常、服务实例未就绪或空结果映射为 `authentication_failed`，不解析英文日志。批准前先调用 `IsPlayerAuthenticated`，已认证玩家返回 `already_authenticated`。
- 主机保护仍在控制模组执行点强制检查；kick 额外确认 `Game1.server` 非空。目标在提交后离线会在消费时返回 `player_not_online`，前端不会把提交成功误报为执行成功。
- C# 无游戏运行时契约测试位于 `embedded/smapi-mod-contract-tests/`，覆盖全部要求错误码及 `succeeded/ok` 封装；Docker `dotnet run -c Release` 可执行。实际 Mod 仍用 `/p:GamePath=/game` 构建。
# BROADCAST-BAN-RESULTS-1 喊话与封禁执行回执

- `broadcast`/`say`：控制模组完成输入清理后依次检查空消息、世界状态和 `Game1.multiplayer` 聊天系统；只有 `sendChatMessage(...)` 调用正常返回才写 `succeeded/ok`。失败码为 `empty_message`、`world_not_ready`、`chat_unavailable`、`broadcast_failed`。成功仅表示消息已交给游戏聊天系统，不承诺每个客户端都实际收到。
- `ban`：Go driver 保留原权限与 Junimo admin 提升检查，但命令 payload 新增 `uniqueMultiplayerId` 与 `adminPromoted`。提升失败返回结构化 `admin_promotion_failed`，不创建命令。控制模组优先在 `Game1.getAllFarmers()` 按 ID 精确定位并拒绝主机；`Game1.server` 可用时直接调用 `Game1.server.ban(id)`，调用返回后写 `succeeded/ok`，异常写 `ban_failed`。
- 只有直接 server API 不可用时才降级模拟 `!ban <name>`。降级前按已由 ID 定位的目标名字检查重名，重名返回 `ambiguous_player`，不会静默封错；成功调用聊天派发只能写 `dispatched`，不能冒充最终封禁成功。另覆盖 `world_not_ready`、`player_not_found`、`host_not_supported`、`command_dispatch_failed`。
- 持久化结论已由用户在真实实例人工验证：`Game1.bannedUsers` 在服务器容器重启后丢失。前端已改成确定性提示“重启后会丢失，需要重新操作”；本阶段不实现面板侧封禁名单、持久化补偿或解封入口。
- 测试：C# contract tests 覆盖 broadcast 空消息/世界未就绪/聊天不可用、ban ID 精确匹配/不存在/重名/主机/派发失败封装；Go 测试覆盖精确 payload 与 admin 提升失败；完整 `go test ./...`。
- 真实实例：新版 DLL 加载后 broadcast 返回 `succeeded/ok`；ban 主机返回 `failed/host_not_supported`。因当时无 farmhand 在线，ban succeeded 真机分支未冒险伪造，待测试玩家在线时补验。

# EVENT-JOJA-SAVE-RESULTS-1 节日、Joja 与游戏内保存回执

- `trigger-event` 在控制模组执行点检查世界、当天节日、主机是否已处于节日现场以及聊天系统；明确失败码为 `world_not_ready`、`no_festival_today`、`festival_not_active`、`chat_unavailable`、`command_dispatch_failed`。当前 Junimo 只能通过 `!event` 聊天命令启动主活动倒计时，聊天调用返回仅写 `dispatched/ok`，不声称节日最终已经启动。
- `enable-joja` 继续要求后端精确确认文本并先调用 Junimo `POST /roles/admin`。命令 payload 新增 `adminPromoted`：新版控制模组在提升失败时写 `failed/admin_promotion_failed` 且不派发；旧模组无回执能力时仍沿用原 HTTP 错误。聊天异常为 `command_dispatch_failed`。只有存档已有持久 `JojaMember` 标志时才允许 `succeeded/ok`；单纯把 `!joja` 交给聊天系统只返回 `dispatched/ok`。
- 上游源码确认 `!joja` 当下只把 Junimo 内存中的 `AlwaysOnConfig.IsCommunityCenterRun` 设为 `false`；这个内存开关本身不是“永久存档状态”的充分证据，因此不能作为 succeeded 条件。
- 新增管理员接口 `POST /api/instances/:id/saves/save-now`。HTTP 只返回 commandId/queued；模组将 commandId 注册到单一 pending tracker，设置 `Game1.saveOnNewDay=true` 后维持 `running`。只有后续 `GameLoop.Saved` 才以同一 commandId 原子覆盖为 `succeeded/ok`。两分钟无 Saved 为 `failed/save_timeout`；并发请求为 `failed/save_already_pending`。
- 保存回执与自动回档事件共用 `GameLoop.Saved` 事实来源，但游戏内保存与面板 ZIP 备份是两件事。若保存发生后、终态结果落盘前崩溃，running 最终转为 `unknown/execution_interrupted`，绝不自动重试。
- 测试：C# contract tests 覆盖世界未 ready、节日条件、派发、Joja 提权失败/持久确认、保存 commandId 关联、并发、Saved 完成和超时；Go 覆盖提交和提权失败 payload；`go test ./...` 通过。嵌入 DLL SHA256：`ADF4473AF58BBFC58C1A4735389B07F269D73BC40AFD4F7626A3D0C68F2E7EBC`。
- 真实实例 `1111_442923526`：event 返回 `failed/no_festival_today`；Joja 经 Junimo 日志确认解析后结果为 `dispatched/ok`，未冒充永久完成；save-now 的同一 commandId `c1178eb65b034c96814416dc04c101f9` 从 running 在睡觉保存后转为 `succeeded/ok`。

# COMMAND-RESULT-PRODUCTIZATION-1 命令回执持久化与运维收尾

- migration `009_control_commands.sql` 新增 `control_commands`：按 `command_id` 主键幂等保存实例、命令、目标、actor 快照、七状态、结构化错误/白名单详情、提交/完成/导入时间和最终审计标记。面板重启后结果查询优先读 SQLite；文件只承担模组→面板交接。
- 后台每 5 秒扫描各 Stardew 实例的 `command-results/`。结果成功事务入库后才删除终态文件；入库失败保留文件。`running` 且对应命令文件仍存在时保留结果闸门，避免容器重启后重复执行。超过 5 分钟的数据库 running 转为 `unknown/execution_interrupted` 并写最终审计，不自动重试。
- 新增 `GET /api/instances/:id/control-commands?limit=50`（1–200），返回最近控制命令。既有单命令查询在文件被收走后从 SQLite 恢复。旧模组提交持久化为 `dispatched` 且 `resultSupported=false`，UI 必须显示“已提交，无法获取精确结果”。
- 每小时清理终态历史：默认保留 30 天且最多 1000 条，环境变量 `CONTROL_COMMAND_RETENTION_DAYS`、`CONTROL_COMMAND_RETENTION_COUNT` 可调整；queued/running 永不因保留期或数量清理，未入库结果文件也不由数据库清理任务处理。
- `runtimeDiagnostic.commandProtocol` 提供 commandResultVersion、待消费数、未入库结果数、最老待处理时间、最近消费时间及两个目录可写性；健康检查报告长时间 queued、旧模组和目录不可写。
- 审计以 `control_command_submitted`/`control_command_completed` 关联 actor、commandId、命令、目标、最终状态与错误码。不会保存广播正文或完整 payload；details 只接受 playerId/playerName，疑似密码、凭据或 Token 的结果消息整体脱敏。
- 验证：迁移、重复导入、重启恢复、文件删除闸门、终态保留边界和旧协议测试；`go test ./...`、前端构建。该阶段不修改控制模组和 DLL。
- 真实实例验证：隔离临时面板数据库通过真实 `stardew` 控制目录提交 `say`，commandId `64a0853e85c997d6b14ad6af48805f29` 从 HTTP `queued` 到模组 `succeeded/ok`，SQLite 保留 `commandType=say`、actor `command-validation` 和完成时间，结果目录最终为 0。上线前遗留的 24 个结果文件也已由新版 scheduler 幂等导入生产 `panel.db` 后清理；因这些旧文件早于提交审计，无法反推的 commandType/actor 显示 unknown，不伪造历史身份。
# SAVE-BACKUPS-EMPTY-LIST-1 空备份列表契约修复

- `stardew_junimo.ListBackups()` 在备份目录不存在或目录内没有 ZIP 时固定返回非 nil 空切片，`GET /api/instances/:id/saves/backups` 因此稳定输出 `"backups": []`，不再输出 `null`。
- 新增回归断言覆盖全新实例的空目录场景，避免后续重构重新引入 `nil` slice JSON 编码问题。
- 验证：`cd backend; go test ./internal/games/stardew_junimo ./internal/web`。
# INSTALL-RUNTIME-VERIFICATION-1 安装成功必须验证完整运行文件

- 安装流程在 SteamCMD `validate`、SMAPI 安装成功后，新增对同一 Docker `game-data` 卷的最终验证。只有以下运行文件均存在时才写入 `game_installed`：`StardewValley`、`Stardew Valley.dll`、`steamapps/appmanifest_413150.acf`、`StardewModdingAPI`、`StardewModdingAPI.dll`、`.steam-sdk/steamapps/appmanifest_1007.acf`，以及 `.steam-sdk` 下至少一个 `steamclient.so`。
- 缺失任一项时实例转为 `error / install_verification_failed`，安装 job 失败并提示重新安装或修复；启动服务器前也会重复执行同一检查，禁止空卷或残缺卷启动。
- `ReconcileState()` 对所有已安装/可启动/运行中状态检查该 Docker 卷，历史误写的 `game_installed` 会自动恢复为 `install_verification_failed`；Docker 或镜像暂时不可用时只保留原状态并记录告警，避免网络/daemon 短暂故障误报文件缺失。
- “仅 Steam 授权登录”只更新授权标记，不再篡改原实例的安装 state/phase，避免认证成功把下载失败覆盖成“游戏已安装”。
- 验证：`cd backend; go test ./...`。
# JUNIMO-STACK-UPDATE-1 阶段三：成对升级、认证卷保护与自动回滚（2026-07-13）

- 新增管理员 `POST/GET /api/instances/:id/junimo-update/apply` 和独立 `stardew_junimo_update_apply` job。POST 只接受严格 `{"confirm":true}`，目标仅来自当前构建内置且 `tested=true` 的推荐版本对；与 install、lifecycle、dry-run、apply 互斥，相同推荐版本拒绝重复执行。
- 执行器位于 `stardew_junimo` 服务层：重新执行清单/实例/Docker/Compose/当前 digest/认证卷/候选拉取/config 关键预检，保存原 `.env`、Compose、两镜像引用/digest 和运行态；运行实例复用 `lifecycleRunner.doStop`。
- 私有恢复目录 `.local-container/junimo-update/recovery/<applyId>` 为 0700、文件 0600；停服后把 `steam-session` 受控克隆到当前 Compose project 限定的临时 Docker volume，不枚举或输出 token。`.env` 五个版本字段经同目录临时文件原子替换。
- 新版先重建 `steam-auth`，硬验收要求目标 digest、容器 running/可选 health=healthy，以及 `/steam/ready` 可访问、JSON 合法并明确包含 `ready`/`has_ticket` 两个布尔字段；字段值只表示 Steam 在线能力，不要求为 true。再重建 `server`，验证容器/health、Junimo `/health`、目标 digest、SMAPI/Control 与 FIFO `info` 契约；Steam 登录、ticket 和邀请码均不参与升级成败。成功恢复升级前运行/停止状态并清理私有快照。
- 变更后失败进入 `rolling_back`：停止新版 pair、恢复原配置和认证卷、依次重建并验收旧 auth/server、恢复原状态。终态区分 `succeeded`、`failed_rolled_back`、`rollback_failed`；后者保留私有材料且禁止自动破坏性重试。`apply-status.json` 跨 Panel 重启持久；只在可证明时继续验收，否则回滚，不一致时不猜测。
- Docker 扩展只接受固定 `server`/`steam-auth` 和受限 project/volume/image，使用 `compose stop`、单服务 `up --no-deps --force-recreate --pull never` 与固定探针/克隆脚本；没有 `down -v`、目标卷删除或任意 shell 参数。支持包白名单不包含恢复目录。
# GAME-RUNTIME-VERSION-1：Stardew / Steamworks SDK 只读版本检测（2026-07-14）

- 扩展 `runtime_stack_manifest.json`：在既有 Junimo server/steam-auth 推荐对之外，固定记录 Stardew App `413150` 与 Steamworks SDK Redistributables App `1007` 的推荐 `buildId`、`manifestVersion`、说明、下载估算和共同 `tested` 标志。Panel 运行时不查询 Steam latest，也不按 buildid 数字大小判断升级/降级。
- 新增最小 Valve KeyValues/VDF tokenizer，解析固定 ACF 的 `appid/buildid/StateFlags/installdir/LastUpdated`；支持字段换序、空白、BOM 和转义，拒绝 appid 错配、缺失/非法 buildid、损坏结构。只读 Docker helper 先 inspect 已存在 `<project>_game-data` volume 和本地 server 镜像，再以 `--pull never --network none`、只读 mount 读取两份固定文件；不会因 volume 不存在而隐式创建。
- 管理员 API：`GET /api/instances/:id/runtime-components` 返回当前/推荐 game 与 SDK buildid、组件及整体状态、检测时间和 release notes；`GET|POST /api/instances/:id/runtime-components/dry-run` 恢复/运行只读预检。状态为 `up_to_date/update_available/game_missing/sdk_missing/manifest_invalid/custom_or_unknown`。未安装实例返回 `game_missing + not_installed`，不报 500。
- 预检只验证当前 manifest、内置 tested 目标、下载+staging 保守空间估算、本地 SteamCMD/账号缓存能力、Docker daemon 与受控 staging 命名能力。它不联网登录 Steam、不拉镜像、不执行 `app_info_print/app_update`、不创建 staging volume、不写 game-data。
- API 不返回完整 manifest、Steam 用户名/密码/token/ticket；错误只返回分类信息。阶段六的 depot 下载、staging 创建、校验、切换、停服/重启与回滚均未实现，也未接入 Junimo stack apply。

## SMAPI 推荐矩阵与 staging 安全升级（2026-07-14）

- `runtime_stack_manifest.json` 新增经发布验证的 SMAPI 4.5.2：官方 Release URL、精确 41,889,142 字节和 SHA256 `dd01ddca7b566bfe0d3b3d2d03833496abc56c53da976241f2ab443f5484acc4`；现行清单固定 Control 0.2.0、DLL SHA256、`commandResultVersion=1`。用户实例不查询 GitHub latest。
- `RuntimeReadSMAPIMetadata` 只读挂载当前显式 `GAME_DATA_VOLUME`，从 `StardewModdingAPI.dll` 的 AssemblyInformationalVersion 与 launcher/deps/runtimeconfig/config 固定产物判断实际安装；`.env` 的 `SMAPI_VERSION` 只作配置线索。状态为 `up_to_date/update_available/missing/invalid/incompatible_game/incompatible_junimo/custom_or_unknown`。
- `GET /runtime-components` 追加 `smapi`；另提供管理员 `GET /smapi-update`、`GET|POST /smapi-update/dry-run`、`GET|POST /smapi-update/apply`。POST 不接受 URL、版本、SHA、ZIP、image/service 或命令；apply 只接受严格 `{"confirm":true}`。
- dry-run 精确核对 Stardew buildid、SDK buildid、Junimo tag、steam-auth-cn tag、内置 Control 版本/DLL/协议和 staging/空间能力；只读 `du` 当前 game-data，并返回 `requiredBytes/freeBytes` 的克隆与安装缓冲估算。不匹配时拒绝，绝不顺便更新游戏、SDK、Junimo 或 auth，也不创建 volume、不下载 installer、不停服。
- apply 仅下载清单中的受审加速候选及官方兜底，限制精确 URL 模板/重定向域名、Range/Content-Length/读取上限，校验精确大小和 SHA256；ZIP 检查条目数、路径、symlink/device、压缩比与总解压上限，并要求官方 Linux/Windows installer 各一份。安装始终调用官方 Linux installer，不手拼 SMAPI 布局。
- 当前 game-data 只读克隆至受控 `<project>_anxi-smapi-update-<24hex>` staging volume；在 staging 离线安装和精确验版后停服，才替换 host bind 的内置 Control，原子改写 `.env` 的 `GAME_DATA_VOLUME`，再启动完整推荐 stack。验收包括 SMAPI 精确版本、Junimo `/health`、server 日志中的 JunimoServer/Control 实际加载证据、commandResultVersion、status/players，以及 auth 服务接口可访问且契约可解析；不要求 Steam 已登录、ticket 或邀请码。
- 失败切回旧 volume、按“原文件存在/不存在”精确恢复 Control，并用 rollback 专用启动路径避免 lifecycle 再次覆盖旧 Control，最后恢复原运行状态；成功也保留旧 game-data 作为恢复材料，只清除临时 recovery 文件。`rollback_failed` 保留两卷和人工恢复指引，禁止自动破坏性重试、`down -v` 或 volume prune。Panel 重启不续跑安装器，而按 recovery manifest 安全回滚。
- `cmd/smapi-candidate` 供维护者查询 `Pathoschild/SMAPI` 正式 Release，默认过滤 draft/prerelease，下载官方 installer 并输出候选 JSON/SHA256；失败不覆盖上次结果，也不修改矩阵/git/tag。
- 玩家完整同步包仅在缓存中存在精确推荐安装器且大小/SHA 匹配时携带它；所有 pack manifest 都记录推荐 `version/checksum/bundled`，增量 Mod 包保持 `bundled=false` 且无 `payload/smapi`，旧包不会被改写。
- 新实例初装也先由 Panel 的受控 HTTP client 下载并完整执行同一候选 URL/重定向 allowlist、Range、大小、SHA256 与 ZIP 校验，再以只读 bind 交给容器中的官方 installer；`.env` 的 `SMAPI_DOWNLOAD_URLS` 仅为兼容展示字段，不决定下载目标，其默认文本与 embed 固定候选顺序保持一致。

## 2026-07-14：统一兼容矩阵与发布门禁

Panel 的正式运行组合现在由 `backend/internal/games/stardew_junimo/config/runtime_stack_manifest.json` 单一内嵌快照描述。schema v1 固定 Junimo server、steam-auth-cn（含 `upstreamRef` 与源码 revision）、Stardew App 413150 buildid、Steamworks SDK App 1007 buildid、SMAPI URL/SHA256、控制 Mod 与 `commandResultVersion`，并增加 `channel`、`minimumPanelVersion`、`status` 和 release notes。Go 结构仍在内存中生成旧调用需要的 preferred/candidate 别名，但 API 和持久状态以 `images`/`digests`/`urls`/`controlMod` 为准。

每个 Panel 版本只内嵌一份当前组件清单，状态固定为 `recommended`；紧急停用时可标记 `withdrawn`。不再维护 candidate/tested 状态机或候选目录。正式新装、Junimo 成对升级、游戏/SDK 预检和 SMAPI dry-run/apply 读取这份内嵌清单，并检查当前 Panel 不低于 `minimumPanelVersion`。用户请求体仍不能传入目标 URL、tag、digest、ZIP 或命令；初始安装也不接受 `latest` 或任意 Junimo tag。

Junimo/auth dry-run 在拉取后将 tag 解析出的 RepoDigest 与矩阵逐项比较；digest 缺失返回 `target_image_digest_unavailable`，tag 指向其他内容返回 `target_image_digest_mismatch`。失败不会写实例配置。withdrawn 快照返回 `matrix_withdrawn`，阻止新安装/升级，同时保留已安装实例用于管理员按精确旧矩阵处理。

`scripts/compatibility_matrix.py` 只校验 Panel 内嵌清单的精确 tag、digest、auth `upstreamRef`/`sourceRevision`、game/SDK buildid 和 SMAPI SHA，不再负责候选生成或状态晋级。steam-auth-cn 发布不触发 Panel；维护者为某个 Panel 版本直接指定已经确认的 server/auth 等组件版本。生产 apply API 只读取当前 Panel 镜像内嵌清单，不依赖远程 latest。
# 2026-07-14 更新事务安全加固

- SMAPI apply 的 `serverWasRunning` 改由 `docker compose ps` 真实状态产生，并在修改共享 Control Mod/切换卷前再次读取 server 与 steam-auth 状态；任一仍运行都会先复用生命周期停服。Panel 在停服后、切卷前中断时，会按持久化的 `serverWasRunning` 重启旧栈。
- SMAPI 所有发生过 mutation 的回滚都必须通过旧 SMAPI 版本、Junimo health/API、auth 服务接口契约、Junimo/Control 加载日志与 Control status/players 验收，才能写入 `failed_rolled_back`；Steam 登录状态、ticket 和邀请码不参与回滚成败，否则纯局域网实例会被错误卡在 `rollback_failed`。
- Junimo 成对更新在停服前从运行中的 server/steam-auth 容器读取真实 ImageID；仅未运行的服务才回退检查 `.env` 镜像。恢复清单保存不可变 ImageID，回滚重建前将两项 image 临时固定为该 ImageID，避免同名 tag 漂移。
- server 停止但 steam-auth 仍运行时也会执行完整生命周期停服，确认 auth 静默后才克隆 steam-session。`AuthWasRunning` 写入私有恢复清单，最终仍恢复 server 原有运行/停止意图。
- 验证：`go test ./internal/games/stardew_junimo -run 'Test(SMAPIUpdate|SMAPIRollback|RuntimeUpdateApply)' -count=1`；新增状态漂移、中断恢复、旧栈验收失败、tag 漂移和 auth-only 漂移回归测试。
# 2026-07-14：真实运行镜像检查与 steam-auth 探针兼容修复

- `RuntimeImageInspect` 与 `RuntimeServiceInspect` 改为通过 Docker `--format` 只读取 ImageID、RepoDigests、配置镜像名、运行状态和 health；不再把包含 `Config.Env` 的完整 inspect JSON 经过日志脱敏后再解析，避免空 `VNC_PASSWORD=` 等环境项破坏 JSON，也减少凭据进入进程内输出的范围。
- `RuntimeSteamAuthReady` 不再假设 steam-auth 镜像内存在 Node.js；改用镜像已具备的 Bash `/dev/tcp` 请求容器内 `127.0.0.1:3001/steam/ready`，仍只反序列化 `ready` 与 `has_ticket` 两个布尔值。
- Docker integration 新增无 Node auth fixture：镜像故意包含敏感键环境变量，覆盖镜像 inspect、容器 inspect、真实 HTTP ready/ticket 探针。另提供 `ANXI_REAL_SERVER_IMAGE` / `ANXI_REAL_AUTH_IMAGE` opt-in 测试真实推荐镜像。
- 验证：`go test ./internal/docker -count=1`；`go test -tags=integration ./internal/docker -run 'TestRuntime(InspectAndAuthProbeWithoutNode|RealImagesOptIn)' -count=1 -v`。
# JUNIMO-UPDATE-PROGRESS-1 升级进度与验收探针修复（2026-07-14）

- Junimo 成对升级的 dry-run/apply 状态新增可选 `download`，按组件返回镜像引用、完成层数、总层数和百分比；拉取仍使用可信候选与 digest 校验，进度不包含 registry 凭据。
- Junimo server 验收不再依赖镜像中不存在的 `wget`，改用镜像已有的 Bash `/dev/tcp` 请求本机 `8080/health`。该探针同时用于新版本验收和旧版本回滚验收。
- apply 失败状态新增 `causeCode/causeError` 与 `rollbackCode/rollbackError`。进入 `rollback_failed` 时保留最初验收原因，并明确记录回滚失败步骤；原始命令输出仍不返回前端。
- 真实测试实例只读核验确认：旧版容器和认证服务已恢复健康，历史 `rollback_failed` 是旧 `wget` 探针连续误判造成。未自动改写该安全终态，也未删除恢复材料。
# RUNTIME-VERIFY-FIFO-1 Junimo 升级控制契约验收修复（2026-07-14）

- 成对升级不再用 `docker compose exec -T server attach-cli` 验证 Junimo 控制契约；`attach-cli` 是要求终端的 tmux UI，非 TTY 调用会稳定返回 `open terminal failed: not a terminal`。
- 验收改为复用面板正式控制台的 FIFO 契约：先记录 `/tmp/server-output.log` 字节偏移，再向 `/tmp/smapi-input` 写入只读 `info`，仅检查偏移后的新输出是否包含 Junimo `--- Server Info ---` 响应。
- 原有 digest、容器健康、Junimo `/health`、SMAPI 状态、`commandResultVersion` 与精确 Junimo 版本验收保留；Steam ticket、邀请码和登录状态已从维护事务硬门槛移除。FIFO 输出不写入 apply 状态或日志，避免泄漏运行时信息。

# AUTH-CAPABILITY-DECOUPLE-1：维护验收与 Steam 在线能力解耦（2026-07-16）

- Junimo server/auth 成对升级、Panel 重启续验和旧版本回滚统一复用服务级 Auth 门槛：精确镜像、容器运行/健康、`/steam/ready` 可访问且响应 schema 完整。`ready=false` 或 `has_ticket=false` 不再超时或触发回滚；apply 状态会给出非阻断提示，说明当前未建立完整 Steam 在线会话。
- Junimo server 硬验收在 FIFO `info` 精确版本响应后即可通过，不再读取实例邀请码；邀请码 API 继续作为用户主动使用的独立功能。SMAPI 新栈与旧栈回滚验收采用同一原则，不再读取邀请码，也不检查 Auth 两个能力布尔值。
- `RuntimeSteamAuthReady` 对 schema 更严格：缺失 `ready` 或 `has_ticket` 字段仍判为服务契约损坏。此变更只放宽业务能力值，不放宽镜像、容器、HTTP/JSON、Junimo、SMAPI、Control、状态文件或回滚安全门槛。
- 验证：`go test ./internal/games/stardew_junimo -run 'TestRuntimeUpdateApply|TestSMAPIUpdate' -count=1` 与 `go test ./internal/docker -count=1`。回归覆盖 `ready=false/has_ticket=false` 且邀请码不可用的 LAN-only 实例仍可升级，以及 Auth 端点不可用仍成对回滚。
- 回归测试覆盖成功路径不得调用 `attach-cli`，以及 FIFO 控制契约无响应时仍必须成对回滚。

## RUNTIME-FARM-CATALOG-1：按事务校验本次启动的农场目录（2026-07-15）

- 控制 Mod 升级为 `0.2.0`。`options.json` schema 2 由 SMAPI 在 `GameLaunched` 本次运行中生成，包含 `source=smapi-runtime`、`requestId`、`transactionId`、`generatedAt`、控制组件版本、已加载 Mod 的 `UniqueID + Version` 列表、稳定 SHA256 指纹和 `DataLoader.AdditionalFarms(Game1.content)` 返回的农场目录。目录不写宿主路径；单个图片 data URI 限制为 64 KiB，超限时输出空图。
- 新建事务在启动容器前原子写入 `control/farm-catalog-request.json`，请求 ID 与 transactionId 相同并带过期时间，同时删除旧 `options.json`。事务快照新增 catalog request/options，commit 前失败时与 settings、init、marker、Mod 状态一起恢复。
- 调用 `/newgame` 前必须读取不超过 2 MiB 的普通 `options.json`，确认 schema/source、双 ID、生成时间、Mod 指纹及目标 FarmType。目标不在本次 `AdditionalFarms` 时返回 `farm_type_not_loaded`；旧缓存、损坏 JSON、超大文件、指纹不一致和等待超时均不得继续 POST。
- `SetFarmType` 支持 8 种官方农场、`FourCorners` 别名、`MeadowlandsFarm`、显式模组 ID 和兼容入口 `modded`。显式模组 ID 仅从本次 `AdditionalFarms` 解析；`modded` 排除内置 `MeadowlandsFarm` 后稳定选择第一个真正模组农场。未知值即使安全回落 Standard，也保持 `farmTypeResolved=false` 并写 warning。
- 结构化 `new-game-pending` 由控制 Mod 校验 schema、transactionId 与 `server-init.json` 一致、未过期且 state 为 pending；任意旧存档的 `SaveLoaded` 不再删除活动 marker，marker 只由 Go 事务 commit/rollback 收尾。
- 兼容策略：旧控制 Mod 仍允许官方农场沿用阶段 5 流程；模组 FarmType 对旧 schema、缺失 fresh catalog 或 requestId 不匹配一律拒绝。当前 Web handler 仍拒绝全部模组 FarmType，前端模组卡仍为只读。

错误码：`runtime_catalog_prepare_failed`、`control_mod_catalog_unsupported`、`runtime_catalog_schema_invalid`、`runtime_catalog_stale`、`mod_fingerprint_mismatch`、`farm_type_not_loaded`、`runtime_catalog_invalid`、`runtime_catalog_too_large`、`runtime_catalog_timeout`、`runtime_catalog_wait_canceled`。

影响文件：`embedded/smapi-mod-src/ControlContract.cs`、`ModEntry.cs`、两份控制 Mod manifest、嵌入 DLL、`runtime_farm_catalog.go`、`runtime_farm_catalog_test.go`、`new_game_transaction.go`、`lifecycle.go`、`saves.go`、`config/runtime_stack_manifest.json` 及契约测试。

验证：C# contract tests 通过；控制 Mod Docker/.NET 6 构建 0 errors（1 个既有 analyzer warning）；DLL SHA256 `5e82eb847734d81c08f7295525944e53f343fc3e67715868198bc551e96b24ce`；`go test ./...` 与前端 `npm.cmd run build` 通过。当前仅有一个既有实例，未启动或修改它，因此尚未完成隔离实例 fresh SVE catalog 验证，不能用旧 options 缓存宣称 FrontierFarm 已通过运行时门禁。

## MOD-FARM-CREATE-1：显式模组农场创建闭环（2026-07-15）

- 新增统一 `NewGameFarmType`：接受数字 0～7、官方名称/别名、`MeadowlandsFarm`、显式模组 ID 和兼容值 `modded`。官方向 Junimo 写数字；模组 ID 保留大小写并写字符串；未知合法 ID 不回落 0，数字 8 以上拒绝。
- `ENABLE_MODDED_FARM_CREATION` 默认 true；部署方可显式设为 false 关闭。Web 仍只放行 enabled、依赖完整、无冲突、explicit confidence 的目录项；创建事务复用 provider/required/package closure，补开必要组件，并保留无关 Mod 在创建前的启停状态。
- XML 最终验证把 `<whichFarm>` 与运行时目标 ID 精确比较；mismatch 返回 `farm_type_mismatch`、隔离错误目录并恢复旧指针/配置/Mod。只读反编译 Stardew 1.6.15 确认 custom farm 的 `SaveGame.whichFarm` 由 `Game1.GetFarmTypeID()` 写为 `whichModFarm.Id`。
- 成功前原子写 `EnsureNewSaveModProfile`：`DefaultEnabled=false`，农场 provider/必要依赖强制为 true，同时保留创建时物理上已经启用的其他 Mod；创建前已关闭的 Mod 仍为 false。这样新存档首次重启不会静默关闭无关 Mod，新安装的 Mod 仍需按存档显式启用。失败返回 `mod_profile_commit_failed` 并进入 `profile_commit_pending`，保留正确新存档而不报告完整成功。
- 新增管理员 `PUT /api/instances/:id/mods/enabled`，请求 `{enabled, saveName?}`。仅停服且存档存在时执行，一次 profile 写入、一次物理状态应用，批量切换当前所有可切换第三方 Mod；SMAPI runtime、Control、Junimo 等 built-in 组件始终跳过。响应返回 `mods/enabled/saveName/changedCount`，审计事件为 `all_mods_enabled_update`。
- 存档 API 增加 `farmTypeLabel`；合法 custom ID 原样保留。影响 `farm_type.go`、依赖/事务/lifecycle/profile/saves、Web handlers、模型及测试。
- 验证：`go test ./...`、`go build ./...`、C# contract、Control Docker build、前端 build、Docker 面板镜像 build 均通过。随后使用独立临时 Panel 数据库、Compose project、game-data/steam-session volume、端口、控制目录与存档目录完成真实 SVE 1.15.11 E2E：显式创建 `FrontierFarm`，主 XML 为 `<whichFarm>FrontierFarm</whichFarm>`；重启后正常加载，并完成 `FrontierFarm → Standard → FrontierFarm` 双向切档，正式 profile 能禁用/恢复对应依赖。既有实例未被启动、停止或修改；该阶段开关仍关闭，现已由 `MOD-FARM-DEFAULT-ON-1` 改为默认开启。

### 真实 SVE E2E 补充（2026-07-15）

- 真机首次暴露两个仅运行时可见的边界：SMAPI 会从 `mods/smapi` 加载 bundled `SMAPI.ConsoleCommands`/`SMAPI.SaveBackup`。预期 fingerprint 现在直接扫描该运行时目录并以它为权威来源；即使旧顶层重复件已移入 quarantine，两个组件仍会被计入且按 UniqueID 去重。manifest 的 `7.4` 等短版本会按 SMAPI 语义规范化为 `7.4.0` 后再散列，避免同一版本误报 mismatch；仍排除 SMAPI loader 自身。
- Content Patcher 在 Control 的早期 `GameLaunched` 回调之后才完成 `Data/AdditionalFarms` 注入。Control 在活动 catalog request 存在且目标尚未出现时每 120 tick 原子刷新 `options.json`；Go 对 requestId/fingerprint 已正确但暂缺目标的早期 catalog 继续等待，直到目标出现或超时，超时才返回 `farm_type_not_loaded`，且不会调用 `/newgame`。
- 隔离创建结果：官方 `E2EStandard_443802038` 的 XML 为 `0`；模组存档 `E2EFrontier_443802727` 的 XML 为 `FrontierFarm`（6,970,352 bytes）。Frontier profile 强制启用 Content Patcher、Farm Type Manager、SVE CP/FTM/Code 与 Frontier CP，并保留创建前已经启用的其他 Mod；创建前 disabled 的组件继续保持 disabled。
- 当前嵌入 Control DLL SHA256：`21eedc867d5a051389e19a5941aeaa067a7c6e36dbced1c86193d8e44a9c8249`。在真实 SVE 创建、重启、切档、备份恢复和故障注入验证完成后，`ENABLE_MODDED_FARM_CREATION` 已改为默认 true；显式 false 仍可关闭。

## MOD-FARM-DEFAULT-ON-1：模组地图创建默认开启（2026-07-16）

- `config.Load` 在环境变量缺失、为空或值无效时将 `EnableModdedFarmCreation` 设为 true；只有显式布尔 false 才关闭。
- 默认变化只打开已经存在的入口，不降低门禁：目录项仍需 enabled、依赖完整、无冲突、explicit confidence，并在创建事务中通过 fresh runtime catalog、loaded Mod fingerprint 和最终 XML FarmType 验证。
- 影响 `internal/config/config.go` 与 `modded_farm_creation_test.go`；测试覆盖默认开启、显式关闭和无效值回落。

## MOD-FARM-RELEASE-GATE-1：发布前故障注入与兼容收尾（2026-07-15）

- 离线目录只用于发现、展示和依赖计划，不是创建最终事实。模组创建仍必须通过 matching transactionId/requestId、fresh `options.json`、loaded Mod fingerprint 和目标 ID；最终 success 必须再由唯一新目录中稳定、可解析的 XML 精确证明 FarmType。
- 真实故障注入发现 Junimo 的兼容 `server-init` 会在 API ready 前自行生成存档；后端原先随后仍 POST `/newgame`，可能产生两个目录并正确落入 ambiguous。现在 API ready 后先基于事务前后目录集合检查启动期结果：已有唯一新目录则不 POST，直接进入稳定性/XML/profile 验证；多个目录仍 ambiguous。测试固定“legacy startup 已生成时 POST=0”，原有路径仍保证单事务最多 POST 一次、unknown 不自动重试。
- `custom-new-game` 对模组 ID 新增后端 readiness 门禁：provider/任一 required/package component disabled 时返回 `farm_dependencies_missing`，必须先经过管理员确认的一键准备；请求不会创建 job、启动容器或进入 `/newgame`。missing/conflict/旧 Control/stale request/fingerprint mismatch 的原有前置拒绝不变。
- 导入存档不再一律写“禁用全部第三方 Mod”profile。`EnsureImportedSaveModProfile` 读取导入 XML：官方类型保持旧策略；custom ID 必须离线解析为唯一 provider 和完整依赖闭包，再原子写精确 profile。缺失或冲突时导入不得启动。真实导出→删除→导入 FrontierFarm 后，API/主 XML 仍为 `FrontierFarm`，7 个 SVE 组件均启用。
- 支持包测试显式放入事务快照、存档正文和假凭据，确认 ZIP 白名单不包含 `new-game-transactions`、Saves、SMAPI recovery 或敏感配置；日志和错误继续使用统一脱敏。
- 官方回归：数字 `7` 在已有 FrontierFarm 存档时创建 Meadowlands，配置写数字、实际 XML 为 `MeadowlandsFarm`，官方 profile 禁用第三方 Mod；切回 FrontierFarm 后 7 个依赖恢复。全部官方名称/空格/连字符/下划线别名已有表驱动测试。
- 当前 DLL SHA256 为 `21eedc867d5a051389e19a5941aeaa067a7c6e36dbced1c86193d8e44a9c8249`；该阶段 feature flag 默认 false，现行缺省值已由 `MOD-FARM-DEFAULT-ON-1` 改为 true。该阶段未创建 tag、未 push、未发布镜像、未修改既有实例。
# MOD-BUNDLE-RUNTIME-COMPAT-1：整包导入与运行时完整性（2026-07-16）

- 多 Mod ZIP 仍按每个 `manifest.json` 独立导入、启用、分组与删除；发现数包含 ZIP 内全部有效 manifest。`SMAPI.ConsoleCommands` 和 `SMAPI.SaveBackup` 属于 SMAPI 自带组件，不再复制为顶层 Mod，上传摘要返回跳过数量与名称。
- 启动和重启在应用 Mod profile 后检查顶层与 `mods/smapi` 的同 ID 重复件；仅在内置副本存在时将顶层重复件原样移动到 `.local-container/mod-quarantine/smapi-bundled-duplicates/<timestamp>`，不删除文件，也不影响同目录分组/Nexus ID 继承逻辑。
- server compose 默认注入 `ALSOFT_DRIVERS=null` 与 `SDL_AUDIODRIVER=dummy`，现有实例由 `EnsureServerContEnvFix` 迁移，避免无声卡服务器令 UI Info Suite 与 SVE `Data/AudioChanges` 加载中断。两个值均可由部署环境覆盖。
- `GET /mods` 和上传响应新增 `compatibilityWarnings`。启用 SVE 且活动存档的 Introductions 任务仍为 28 人时返回 `existing_save_world_overhaul_not_rebuilt`；面板不会自动改写旧存档树木、地形和任务数据，正确完整状态需新建存档。
- 影响文件：`registry/types.go`、`stardew_junimo/mods.go`、`lifecycle.go`、`compose_template.go`、`server_env_fix.go`、`mod_compatibility.go`、`web/lifecycle_handlers.go`。验证覆盖多 manifest 统计、内置件跳过/隔离、迁移幂等和 28/32 人存档识别。
- 本机隔离 E2E 使用 `Mods1.zip`：发现 38、导入 36、跳过 2，SMAPI 加载 26 个代码 Mod 与 14 个内容包；SVE 缺失/过期依赖均为 0，重复 ID、无声卡异常、加载失败和 ERROR 均为 0。旧存档识别 28，新建存档 `Introductions` 两份序列化记录均为 32。
# REQUIRED-RUNTIME-BUNDLE-1：Panel 启动后强制协调 JunimoServer 125（2026-07-16）

- 运行栈清单新增严格 `runtimeUpdatePolicy`，只允许 `recommended|required`；当前正式清单固定为 `required`，目标仍是 JunimoServer `1.5.0-preview.125` + steam-auth-cn `1.5.0-anxi.2` 精确 digest pair。该策略只约束 Junimo/auth，不自动升级 game/SDK/SMAPI。
- 新 Panel 启动并完成 interrupted job 恢复、实例 Prepare 后调用 `StartRequiredRuntimeUpdate`。未安装实例不执行迁移且新安装天然写入 125；已是 125 直接通过；可信旧候选先走现有私有备份/原子 config repair；受支持旧版自动串联现有 dry-run 与 apply，actor=system，不再要求第二次 Web 确认。
- 协调状态原子写入 `.local-container/junimo-update/required-status.json`（目录 0700、文件 0600），按 Panel version + stackVersion 防止确定性失败在每次重启无限重试。普通失败保留 dry-run/apply 详情和人工重试入口；`rollback_failed`/自定义镜像/不可信配置进入 `manual_action`，绝不覆盖自定义目标或删除恢复材料。
- 生命周期 `Start` 新增 `required_runtime_update` 门禁：安装完成但未达到内嵌 required pair 时不能重新启动旧运行组件；Stop、备份和人工恢复入口不受阻断。协调 apply 仍复用认证卷快照、auth-first/server-second、精确 digest、宿主 Junimo Mod 同步、LAN-only Auth 验收与成对回滚。
- 主要文件：`required_runtime_update.go`、`cmd/panel/main.go`、runtime manifest/config、`lifecycle.go`。测试覆盖 121 自动 dry-run/apply 到 125、未登录/无邀请码成功、可信旧候选修复、目标失败安全回滚、回滚失败人工状态、自定义镜像零修改、同版本失败不循环以及旧栈启动门禁。
- Auth 硬验收兼容真实 `1.5.0-anxi.2` 未配置账号返回：`/steam/ready` 可返回 `ready=false` 与错误说明而不包含 `has_ticket`；这代表服务契约可用、在线能力未配置，不触发升级回滚。旧版 `ready/has_ticket` 和当前 `status/logged_in/accounts` 契约也继续兼容。
- 全新 Panel 启动只执行 `Prepare`，不创建游戏容器/卷；管理员点击安装后才拉取内嵌 125/Auth pair 并进入 Steam 登录。Windows TTY runner 在任务取消时会显式 stop 一次性容器再断开 attach，避免登录提示处取消后容器和认证卷残留。

# GAME-LANGUAGE-1：服务器游戏语言（2026-07-16）

- `GET|PUT /api/instances/:id/config/game-language` 仅管理员可用，请求/响应为 `{languageCode}`；只接受 `zh/en/de/es/pt/ru/ja/it/fr/ko/tr/hu`。
- 新实例或无有效旧配置时默认 `zh`。升级实例尚未生成面板配置时继承 `startup_preferences` 中已有合法语言；首次保存后 `.local-container/settings/game-language.json` 成为权威来源。
- 保存和每次 `ComposeUp` 前都会同步 `startup_preferences` 的 `languageCode`，并按中文与否同步 `clientOptions/useChineseSmoothFont`。首次安装尚无该文件时生成可反序列化的最小配置。
- 影响 `game_language.go`、`game_language_handlers.go`、实例路由与 `lifecycle.go`。测试覆盖默认中文、旧语言继承、面板配置权威同步、未知代码拒绝和其他 XML 字段保留。
# SAVE-IMPORT-E2E-RELEASE-1 real isolated validation (2026-07-17)

- Real `.125` black-box runs passed takeover/as-is (including a second server restart) and swap end-to-end through `completed`: changed Phase-A hash, matching pending fingerprint, target pointer/saveId, finalizeCount generation delta, virtual host, matching Control commandId `GameLoop.Saved`, `dayTransitionComplete=true`, stable parseable XML, and a second restart with the saved hash unchanged.
- E2E fixes: full `1.5.0-preview.125` runtime matching; delayed `saves` registration polling; marked process-identity parsing; maintenance-baseline fallback for transient diagnostics gaps; activation-only restart recovery; BOM-aware Stardew XML validation; short stable-file retry; archived command-result lookup; post-completion runtime-state promotion; and a Control 0.2.0 save-now path using the native `SaveGameMenu` so SMAPI emits `GameLoop.Saved`.
- Two earlier swap attempts remain intentionally preserved as recovery evidence: direct `SaveGame.Save()` wrote bytes but emitted no SMAPI Saved event, and the previous `saveOnNewDay` implementation timed out. Neither was marked completed or restarted after finalization.
- This is technical E2E only. Eight rich save fixtures and human client checks for role selection, skills, relationships, furniture, fridge, cellar, spouse/children/pet, beds/cabins, reconnect and sleep remain missing; `SAVE-IMPORT-JUNIMO-1` stays incomplete.

## Local rich-save upload validation (2026-07-17)

- Cloned the existing `stardew_game-data` volume into the isolated `save-import-local-rich-game-data` volume and uploaded a ZIP copy of `1111_442923526`; the source main-file SHA256 stayed `70858a56eedbe494903749b91d1ad441dab248d3e3443655e0a6005f3f8c1339` and the retained original ZIP SHA256 is `eb16c72576be42883b60adfabe9f67153a4492abdf769a31796a323f7df119ef`.
- The real Web takeover request reached `completed` (`operationId=60953678b5ed8fd81bcca0252c9c17c0`) and survived a second restart with unchanged hash, parseable XML, target runtime saveId, `Pending=null`, `dayTransitionComplete=true`, three runtime cabins and two farmhands. The XML copy retained 62 furniture entries, fridge content nodes and three cellar assignments.
- Two maintenance races found by this run are now closed: evidence baseline polling waits for diagnostics/world readiness without weakening the hash/finalize/process/pointer gates, and maintenance refuses to start the container when the original active pointer is missing or changed, preventing Junimo from creating a new game in that path.
# PANEL-POLL-LEAK-1：轮询资源泄漏与并发门禁（2026-07-18，completed）

- 邀请码读取彻底取消 `attach-cli` 回退；`GET /api/instances/:id/invite-code` 只在 server 容器内读取 `/tmp/invite-code.txt`，空文件稳定返回 `n/a`。driver 对普通查询提供 5 秒缓存与按实例 singleflight；并发浏览器请求只产生一次 Docker exec，错误不缓存，生命周期清理旧邀请码时同步失效缓存。
- 指标接口增加按实例 5 秒缓存与 singleflight，磁盘与 `docker compose stats` 形成同一不可变采样；多个页面及右侧栏并发请求共享结果，不再各自启动 stats 子进程。
- restart lifecycle job 持久化 `{"operation":"restart"}` payload。创建新重启前检查活动 job；已有重启时返回 `409 restart_in_progress`，不取消、不替换当前任务。启动/停止覆盖旧生命周期任务的既有语义不变。
- 回归覆盖空邀请码无 attach、12 路邀请码/指标并发只执行一次、重复重启不取消原 job。Docker Desktop 29.5.3 隔离 Compose 真机验证真实文件 exec、真实 stats、并发共享与清理后零遗留容器/attach-cli。

# CONTROL-PAUSE-COMPAT-1：删除人物后的无人暂停兼容层（2026-07-19，completed）

- Junimo `.125` 的无人暂停使用 `Game1.otherFarmers.Count`，在线删除、断线清理或空角色运行态可能让该集合与真实连接脱节。Panel 不修改上游，Control 0.2.1 改用 `Game1.server.connectionsCount` 识别真实客户端，并在 `UpdateTicking`/`UpdateTicked` 两端仅补写 `Game1.netWorldState.Value.IsPaused=true`。
- 兼容层不写 `false`，不再修改 `gameTimeInterval`，也不再清除 `IsTimePaused`、`Game1.isTimePaused` 或 `pauseTime`，因此只会补足上游漏掉的暂停，不会解除上游睡眠、节日、事件或第三方 Mod 的暂停。Control manifest 必需依赖 `JunimoHost.Server`，保证后置纠错顺序。
- 无客户端只在普通日 `610..2500` 补暂停；节日、600 新日阶段和 2510..2600 结算窗口完全交还上游。菜单暂停要求所有已自定义在场玩家请求暂停，且真实连接数必须与这些玩家数一致；登录、捏人或其他过渡连接存在时不补写菜单暂停，避免连接黑屏。
- 纯策略矩阵覆盖功能关闭、世界未就绪、非服务器、未知连接数、0/1/2 人、部分/全员菜单、握手连接、节日以及 600/610/2500/2510 边界。Docker Desktop 29.5.3 + 真实 Junimo `.125` 验证删除离线人物后立即与 15 秒后均保持 `610/isPaused=true/playerCount=0`，并验证 2500 暂停、2510 放行、600→610 后暂停、节日 600→620 不暂停及重启一致。
# RUNTIME-COLD-START-1：低资源主机升级冷启动与幂等回滚（2026-07-19，completed）

- Junimo server/auth 成对升级的 server 运行验收窗口从固定 5 分钟扩展为 20 分钟，覆盖 2 vCPU、约 1.6 GiB 内存主机上 SMAPI/Junimo 冷启动超过 5 分钟但最终可用的场景；job 总超时仍为 2 小时，digest、health、SMAPI、Control 和 FIFO `info` 契约均未放宽。
- 回滚和“恢复原停止状态”不再把单次 `docker compose stop` 超时直接判成 `rollback_failed`。固定 server/auth allowlist 的 stop 会在 10 分钟边界内幂等重试；只有持续失败或上层 context 取消才进入人工恢复，绝不执行 `down -v`、volume prune 或删除存档/认证卷。
- 新旧实例 Compose 为 `steam-auth/server` 分别写入 `cpu_shares: 256/768`。这是相对调度权重而非 CPU 硬上限：空闲时仍可使用全部 CPU，争抢时为 Panel、Docker daemon 和宿主维护任务保留调度机会。旧 Compose 在升级预检和既有 prepare/start 兼容迁移中幂等补齐。
- Docker driver 只读取 `docker info` 的 `NCPU/MemTotal` 数字字段。CPU 不超过 2 或内存低于 3 GiB 时，apply 状态通过既有 warnings 提示最长 20 分钟冷启动窗口及宿主 swap/swappiness 建议；Panel 不获取完整 `docker info`，也不擅自修改宿主 sysctl。
- 现场根因证据：阿里云 2 vCPU/1.6 GiB 主机、4 GiB swap 但 `swappiness=0` 时，冷启动导致 Docker、SSH、Panel 和 systemd watchdog 长时间失去调度，旧 5 分钟验收误判并在回滚 stop 时再次超时。设置宿主 `swappiness=60` 与临时资源隔离后，`.125`、auth `.2`、Control 0.2.1、活动存档和 Panel 0.3.10 全部恢复健康。
# CONTROL-PAUSE-FEEDBACK-1：在线玩家永久暂停反馈锁（2026-07-19，completed）

- Control 0.2.1 同时读取远端 `Farmer.hasMenuOpen || requestingTimePause` 并写 `NetWorldState.IsPaused=true`。真实三人现场出现 `AllGameplayPlayersRequestedPause` 后，Control 写入的全局暂停反过来让 `requestingTimePause` 持续为真，形成每帧自激反馈；玩家已回到游戏仍卡在 17:50、`isPaused=true`。
- Control 0.2.2 删除全部 connected-client/menu 请求分支。兼容层现在只在 `Game1.server.connectionsCount == 0`、非节日且普通日 610..2500 时单向补写暂停；只要存在任意真实连接便完全交还 Junimo/Stardew 上游，仍不写 `false`，不会解除睡眠、事件或第三方 Mod 暂停。
- 推荐栈 identity 纳入 `control-0.2.2`，`InspectRuntimeStack` 读取运行时 `options.json.controlModVersion`。server/auth 已是目标版本但运行 Control 较旧时返回 `control_update_available`，使 Prepare 同步新 DLL 后仍执行一次受控运行栈重启，而不是只改磁盘文件、进程继续加载旧 DLL。
- Docker Desktop 29.5.3 真机验证：`.121 -> .125 + Control 0.2.2` stopped/running 为 127.72 秒/120.12 秒；v0.3.11 典型 `.125 + Control 0.2.0 -> .125 + Control 0.2.2` 同镜像受控重启为 144.61 秒/109.60 秒。四条链均恢复原运行状态，真实 `options.json` 明确报告 0.2.2。
# 2026-07-20：可恢复的一键全栈升级

- Panel 自升级不再假定 Compose 服务名为 `panel`。能力检查会在隔离 helper 中解析宿主机 Compose，使用 `compose ps -q` 反查当前容器，并同时核对服务、镜像 ID/引用和 `/data` 数据挂载；飞牛缺失部分 Compose labels 时只允许在上述信息全部一致后继续。
- 对可安全识别但非标准的飞牛容器，能力接口返回 `conversionRequired=true`。独立 updater helper 调用 `deploy/migrate-fnos.sh`，备份原 Compose/环境、容器 inspect、数据库和旧镜像 digest，生成标准 Compose 后重建；新 Panel 健康或精确版本校验失败时恢复数据库、部署文件和旧容器。
- Panel 镜像成功切换后，新进程会遍历全部 Stardew 实例并持久化协调状态。Control 判定同时校验 manifest 版本、嵌入 DLL SHA256 与运行中 `options.json.controlModVersion`。
- 运行实例不匹配时固定执行：游戏内通告、保存、`preruntimeupdate` 整档备份、停服、安装 Control、启动、验证 SMAPI 实际加载版本。Control 安装或实载验证失败会停服并进入安全失败状态，禁止旧 DLL 继续运行。
- `/api/system/update/apply` 需要 `confirmFullStack=true`；GET/POST 返回的 `fullStack.instances` 提供所有实例进度。Panel 重启后从 apply 状态、runtime recovery manifest 和每实例 required-runtime 状态续跑。
- 标准转换项目名由旧容器名安全派生，多个历史 Panel 依次转换不会共享 project/service 身份。helper 在切换前再次比对旧镜像引用与 image ID，防止确认后容器被替换。
- 运行实例 Compose 会把可变 `API_PORT` 仅作为宿主发布端口，容器内 Junimo API 固定为 8080；旧 Compose 自动迁移。FIFO/控制命令显式使用实例目录派生的 Compose project，不受复制来的旧 `.env` 项目名污染。
- Windows Docker Desktop 在已移除 bind 容器后仍可能短暂锁住 Mod 目录；事务先重试原子 rename，Windows 权限锁持续存在时先建立完整恢复副本再替换，失败仍可回滚。
# PANEL-SQLITE-INTERRUPT-1 发布验收（2026-07-24，v0.4.2）

- 初始化状态只在启动时从 SQLite 读取一次并缓存；首次管理员创建成功后更新缓存。未知页面/API 在进入 SPA 与初始化前置检查前直接 404。
- `modernc.org/sqlite` 已升级到 `v1.54.0`。真实长查询被 context 取消后，同一数据库的下一次 `SELECT 42` 必须成功；连续三次原生 `SQLITE_INTERRUPT` 才触发进程退出保护，任意成功或其它错误会重置计数。
- Docker Desktop 29.5.3 已验证候选镜像冷启动、100 条扫描路径 404、容器重启后初始化缓存从持久 SQLite 恢复；Linux 容器内取消恢复测试连续 10 轮通过。
- 真实 GHCR `0.4.1` 已通过项目 `RunApply` 升级到本地 `0.4.2` 候选镜像：三项健康验收、SQLite 数据卷查询、扫描路径 404 均成功，同 Compose 游戏容器 ID 未变化。
# FULL-STACK-UNINSTALLED-TERMINAL-1 与 RELEASE-REMOTE-RETRY-1（2026-08-01，completed，待 v0.4.6 发布）

- `/api/system/update/apply` 的全栈聚合现在先识别 `uninitialized` 与 `admin_created`。尚未安装游戏的实例直接返回实例级 `not_needed/100`，当全部实例均无需运行栈同步时，聚合结果同样为 `not_needed/100`，不再因缺少 required-runtime 状态文件长期停在 `checking_runtime/42`。
- 已安装或正在维护的实例仍走 driver、运行栈 inspection 与 required-runtime 持久状态；该短路不把真实运行栈失败伪装成完成。回归覆盖两个未安装状态以及含默认实例的完整聚合响应。
- `scripts/compatibility_matrix.py verify-remote-artifacts` 对必须存在的镜像 inspect、精确 Git 来源 fetch 和 SMAPI Range 下载加入三轮有界重试。SMAPI 每个已验证 2 MiB 分块完成后保留 offset/hash；单源 SSL EOF、429、超时或截断时只重试当前分块并轮换清单内受审源。最终 SHA 不匹配会丢弃本轮并从另一来源重新下载整包，绝不把错摘要结果放行。
- 安全边界不变：镜像 tag 仍需匹配受审 digest；Git 仍 fetch 精确 revision/ref 并验证 ancestry；SMAPI 仍要求精确 URL 顺序、trusted host、`206`、`Content-Range`、长度、总字节数和 SHA256。重试耗尽仍返回失败，不降级为 warning。
- 本机真实公网门禁在 Docker Hub 鉴权 TLS timeout 后重试成功，并在 SMAPI 32 MiB offset 遇到一个源 SSL EOF、另一个源 HTTP 429 后换源续传，最终 301 秒通过；生产 Go 下载器独立从空缓存下载 `41,889,142` 字节并在 87 秒内通过摘要与 ZIP 验收。

# RUNTIME-AUTH-OFFLINE-ACCEPTANCE-1（2026-08-09，released in v0.4.10）

- 运行栈升级的 steam-auth 验收不再等待容器 Docker health 变为 `healthy`。该 health 会把 Steam 在线登录纳入结果，生产环境曾因此让未变化的 auth 在 `verifying_auth` 停留约 4 分 13 秒。
- 新硬门槛为：目标容器处于 running、镜像 ID 精确匹配、容器内 `/steam/ready` 命中受支持 HTTP/schema 合约。HTTP 200 的 legacy `ready` 合约仍兼容；current `status/logged_in/accounts` 合约要求 `accounts` 是 JSON array。真实 auth 镜像未配置账号时返回 HTTP 503，只对白名单 legacy `ready=false` 离线 body 放行。未登录或无 ticket 只追加能力警告，不阻塞 LAN 模式或运行栈升级。
- 新版、未变化版以及最终目标复验统一走同一接口验收；HTTP 500/其它状态、503 current/畸形 schema、接口不可达、current schema 损坏或 digest 不匹配仍按原事务回滚，不把真实 auth 服务故障伪装成成功。
- 新增 Docker integration fixture：真实容器 health 固定 `unhealthy`，真实 HTTP 接口返回未登录，验收仍成功；另用真实容器删除 ready 文件产生 HTTP 404，确认 fail closed。单元覆盖 500 合法 JSON、503 current/ready=true 与 `accounts=null/number/object` 均拒绝，并固定真实 503 `ready=false` 合约。
- 正式发布的真实 Web updater 另从 v0.4.9 注入 Panel 自更新候选 `HEALTHCHECK=false`，等待完整 120 秒后精确收敛为 `failed_rolled_back / health_check_failed`；切回健康 Panel 候选后升级成功。该回滚故障与上方“steam-auth unhealthy 但接口合法时继续”是两条独立证据。v0.3.2 历史 apply body 兼容链也成功，数据、事务备份、非目标 game container/volume 与重启状态均通过。tag `v0.4.10`、Release workflow `31325589153` 和三仓回拉证据见 `docs/09-image-build.md`。

# STARTUP-NEWGAME-DURABILITY-1：启动验收、安装诊断与新建档单写入者（2026-08-13，代码完成，待正式发布）

## Control 启动验收语义

- 普通启动和重启不再把“尚未看到 Control 状态”当作版本不匹配。对外语义固定为三类：等待中、明确通过、明确失败；内部明确失败再细分为 `version_mismatch` 与 `invalid`，避免把坏 DLL、坏 JSON 和错误版本混成同一根因。
- `InspectControlRuntimeGate` 先验证内嵌清单、实例内 Control DLL 内容，再读取本次启动写出的 `control/options.json`。文件尚未出现时返回 `pending/control_runtime_pending`；只有合法 JSON 明确报告了非期望 `controlModVersion`，才返回 `version_mismatch/control_runtime_version_mismatch`；manifest、DLL、文件读取或 JSON/空版本问题分别返回 `invalid` 及稳定细分错误码。
- 生命周期保留实例为 `starting/control_runtime_starting`，使用完整 `runtimeUpdateServerTimeout`（当前默认 20 分钟）等待本次启动证据，不再截断为 1 分钟。等待期间日志包含已等待和剩余时间；一直 pending 到期限只会安全停服并写 `stopped/control_runtime_start_timeout`，不会提示重装或错误版本。
- 明确 mismatch/invalid 会先用独立有界 context 停止 Compose，再写错误终态；无法确认停服时进入 `control_runtime_cleanup_failed`。启动前清理旧 `status.json/players.json/options.json` 失败会以 `control_runtime_snapshot_cleanup_failed` fail closed，防止旧快照冒充本次加载。
- `ReconcileState` 不会在活动 lifecycle owner 或未结束的新建档 owner 存在时把实例提前提升为 running；外部启动的容器也必须已有 `ControlRuntimeGateReady` 才能提升。容器消失时 running/starting 仍收敛为 stopped，Docker 暂时不可读时保持原状态。

## 安装完整性诊断与错误状态

- `GET /api/instances/:id/state` 可选返回 `installationDiagnostic`。字段包括 `status=installed|incomplete|not_installed|unknown`、`requiredFiles=ok|missing|unknown`、Compose、镜像、server 容器、Control 静态/运行时状态、期望/观察版本、`recommendedAction` 与 UTC `checkedAt`。
- 后端只在明确的未初始化/scaffold 且缺少安装证据时给 `not_installed/install`；必需文件明确缺失、已安装场景 Compose 缺失/无效或镜像明确不存在时给 `incomplete/repair_install`。Docker、镜像或卷探针暂时失败固定为 `unknown/diagnose`，不得降级成“未安装”。
- Control 安装内容与运行进程分层：`control.static` 校验实例 Mod manifest 与 DLL；`control.runtime=not_observed` 只表示当前没有本次运行证据，`mismatch/invalid` 才是明确 Control 故障。必需文件证据按实例缓存 10 分钟；`error` 状态可触发一次真实 volume 校验，但探针错误不写“文件缺失”。
- 旧 `runtimeDiagnostic` 字段继续保留兼容；新前端以 `installationDiagnostic` 为操作门禁，顶层 `state=error` 不再等价于未安装或重装。

## 新建存档持久 owner、单写入者与幂等

- `POST /api/instances/:id/saves/custom-new-game` 强制要求非空 `Idempotency-Key`。当前前端为同一尚未得到 202 的配置持久复用同一个 request ID；缺失 header 直接返回 HTTP 428 / `idempotency_key_required`，且不创建 job、owner 或存档。请求 ID、规范化配置 SHA-256、transaction ID、原始 job ID、executor 与随机 owner token 持久化到 `.local-container/control/new-game-owner/owner.json`，事务正文保存在 `new-game-transactions/<transactionId>/transaction.json`。
- lifecycle 新建档 job 使用 exclusive owner。相同 request ID 与相同配置返回原 job；相同 key 配置不同返回 `new_game_request_conflict`；其它活动建档/lifecycle 返回 `new_game_in_progress`，不会先取消旧任务。owner token 每次写前校验，只有确认旧 job 非活动时才允许受控轮换接管；owner/transaction 损坏或身份不一致返回 `new_game_recovery_required`，禁止覆盖现场。
- owner 首次抢占使用完整 staging 目录：先以 `0600` 写入 `owner.json`，再 fsync 文件/目录，最后以 Linux `RENAME_NOREPLACE` 或 Windows 不覆盖 `MoveFile` 原子发布；并发抢占只有一个 winner。历史空 owner 目录只在早于当前进程、内容确实为空，且不存在 rotation、非终态事务、marker/catalog 或 Control 创建进展时才可隔离恢复；任何未知内容都 fail closed。
- 每个事务在创建前固定 `creationWriter=startup|http`。没有完整活动存档时只允许 Junimo 启动期自动建档，Panel 永远不 POST `/newgame`；存在完整旧存档时 HTTP writer 必须先观察事务绑定的旧 `save-loaded` 稳定基线和 API ready，再最多提交一次。
- Control `save-creating`、gameloader 从初始值前进、新存档目录出现任一项都是持久 creation progress。一旦 `progressObserved`、`commandCalled`、unknown 或 ambiguous 出现，任何恢复路径都只观察原事务，绝不再次提交 `/newgame`。loader 可先于目录出现，此时足以禁止 POST，但只有 loader/Control 与新目录收敛到唯一 ID 后才能绑定 `targetSaveId`；冲突或多个目录进入 ambiguous 并保留现场。
- Stop、Restart、Restore 与普通新启动不能取消未结束 owner。宿主或 Panel 中断后不会自动启动游戏；用户显式点击启动时才按原 request/config 受控恢复，并继续使用已持久的候选 saveId 与命令 ID。
- 事务失败回滚先可靠写入 `rolling_back`、冻结目录隔离计划与每步 write-ahead checkpoint，再执行任何 quarantine/文件/Mod 恢复。Panel 在任意步中断后，下一次手动 Start 只进入 rollback-only runner：先确认 Compose 已停，然后幂等继续原回滚；不同步进行 runtime 同步、ComposeUp 或 `/newgame`。只有 `rolled_back` 已持久才释放 owner，用户需再点一次 Start 启服。

## 角色定制四段耐久门禁与 Control 0.3.1

- 成功顺序固定为：本事务目标 `save-loaded` → 运行内存角色字段逐项正确 → 使用事务预留的同一 `commandId` 执行 `save-now` 并收到精确 `GameLoop.Saved` → 主存档 XML 与 `SaveGameInfo` 在磁盘上稳定且字段正确。任一阶段缺证据、身份冲突或超时都不能报告成功。
- Control `0.3.1` 只允许匹配 transaction/target save 的 `SaveCreating` 或精确恢复身份应用角色配置。`status.json` 冻结 `transactionId/saveId/creationObserved/customizationApplied/customizationVerified/customizationVerifiedAt/customization`；独立 `players.json` 还必须在验证时间之后报告同一 save 的唯一 host，且 host 名称与配置一致。
- `save-now` 的 command ID 在发布前写入事务，Control 又在打开 `SaveGameMenu` 前原子写入 `pending-save-command.json`。queued/running/terminal 及 Control/容器中断后只恢复同 ID；不会换 ID 重做。unknown、过期、无 `GameLoop.Saved`、不同 transaction/save 或旧于本次 `save-loaded` 的回执都禁止完成。成功回执必须同时携带 `event=GameLoop.Saved`、`transactionId`、`saveId`、`expectedSaveId`，终态落盘后才清 journal。
- 内存和磁盘均校验完整配置：人物名、农场名、最爱、性别、宠物类型/品种、skin/hair/shirt/pants/accessory、眼睛/头发/裤子 RGB 与 `isCustomized`。磁盘门禁连续两轮得到相同的主 XML/SaveGameInfo SHA-256 才稳定；主 XML 还校验运行时解析后的 `whichFarm`，SaveGameInfo 独立核对人物名和农场名。最后要求 Control、loader 和唯一新目录身份收敛，再原子提交 Mod profile、事务 success 并释放 owner。
- 当前运行栈 identity 为 `control-0.3.1`；source manifest、embedded manifest、嵌入 DLL 与 `runtime_stack_manifest.json` 必须保持一致。任何 DLL 变化都要重新计算清单 SHA-256，并在正式候选上完成真实 game-data 编译、契约测试和运行时实载验收。

## 影响文件、验证与边界

- 主要后端文件：`control_runtime_gate.go`、`installation_diagnostic.go`、`lifecycle.go`、`driver.go`、`new_game_transaction.go`、`new_game_transaction_owner.go`、`new_game_progress.go`、`new_game_lifecycle.go`、`new_game_durability*.go`、Web instance/lifecycle handlers、Control `ControlContract.cs/ModEntry.cs/DeferredCommandOutcomes.cs`、两份 manifest/DLL 与运行栈清单。
- 已增加纯函数、生命周期、Reconcile、installation diagnostic、owner/token、progress、durability、Web state 与前端幂等回归。2026-08-13 当前源码已通过全量 Go test/vet/build、Control 契约与真实 game-data 编译、前端 14 项状态测试/build、脚本/ShellCheck、兼容矩阵、runtime/updater Docker integration 和网站 build；精确结果见 `docs/09-image-build.md`。最终 commit 候选镜像、正式 Web 升级/回滚、升级后 Browser 与生产真机仍须重新取证，不能把 pre-candidate 通过写成已发布。
- 隔离 Docker 真实建档同时通过 startup writer（Panel POST=0）和已有活动旧档的 HTTP writer（POST=1、旧档双哈希保持），两条链均完成 Control 内存、同 ID `GameLoop.Saved` 和双 XML 耐久门禁。真实 Stardew 1.6 证明 `Gender/gender` 与 `shirtItem/pantsItem.itemId` 是当前权威磁盘字段，旧 `isMale` 可为 `xsi:nil`、旧衣服字段可为 `-1`；解析器已按该结构严格验证并保留旧格式兼容。当前编译产物、嵌入 DLL 与运行栈清单 SHA-256 均为 `3833769287e794d392296c52df760f8451b24a177243a0926d6f0ca9fd81b3ce`。
- 同一正式版本还必须纳入已进入 `origin/main` 的两个前置修复：`92f3be6bb2731358420ba315ac18029c2506d81f` 的 `run.sh` swap/swappiness 持久化，以及 `621c5645e0048da7c4793035615438ed78fc7002` 的图形化 Compose Web 自动标准化。代码已推送不等于 Release/真机通过；最终候选仍需验证新版 Release 附件确实包含前者，并在生产同步测试中覆盖后者的转换与回滚。
- 明确排除“宿主机重启后自动恢复原 running 实例”。产品预期是宿主重启后游戏保持关闭，由用户手动点击启动；不得为本任务增加 restart policy 或 Panel 启动时自动 ComposeUp。新建档 owner 的手动恢复只是保护未完成事务，不等同于普通运行实例自动恢复。
- Panel 启动时的 required-runtime、Runtime apply 和 SMAPI apply 协调器都先检查未完成 new-game owner；存在 owner 时零修复/停服/起服/替换。普通中断升级在 Panel/宿主重启后只做安全回滚和静态材料收敛，强制保持 server/auth 关闭并提示手动 Start，即使 recovery manifest 记录 `ServerWasRunning=true` 也不 ComposeUp。存档、导入、安装、Control/SMAPI/Runtime 更新、玩家删除与重启计划等变更入口均在未完成 owner 下 fail closed。

# RUNTIME-AUTH-HEALTH-PROBE-1：认证服务健康与 Steam 在线能力彻底解耦（2026-08-14，released in v0.4.17）

## 根因与职责边界

- Issue #9 不是容器、镜像或 digest 错误。旧运行组件验收在目标、最终目标复验、旧版本回滚和 SMAPI 运行栈复验中统一调用 `RuntimeSteamAuthReady`，容器内请求 `GET /steam/ready`。该端点会恢复会话、连接 Steam、登录并获取 App Ticket；少数 Steam 网络受阻或连接很慢的环境中，单次 15 秒 Docker 探针会反复超时，最终耗尽 10 分钟认证预算并把“Steam 暂时不可用”误报为“认证服务接口损坏”。网络正常时同一端点很快返回，所以普通环境不容易复现。
- 运行组件维护事务现在只调用 `RuntimeSteamAuthHealth` 的 `GET /health`。`/health` 是无副作用的服务状态快照，不触发登录；`/steam/ready`、`/steam/app-ticket` 和邀请码链仍保留给确实需要 Steam 登录、ticket 或在线大厅的业务。实例 `/state` 中现有 `steamAuthReady` 诊断字段仍属于在线能力探针，不得与运行组件验收混用。
- 本节取代本文此前所有把 `/steam/ready`、Docker health、`ready` 或 `has_ticket` 描述为 Runtime/SMAPI apply 硬门槛的历史说明；这些旧段落只记录当时版本行为，不能作为当前实现依据。验收不会在 `/health` 失败后回退请求 `/steam/ready`。

## 严格契约、错误与回滚

- 硬门槛顺序固定为：`steam-auth` 容器 `running`；实际 image ID 与目标不可变 digest 精确相等；容器内 `/health` 在单次短预算内可达；HTTP 状态必须为 `200`；响应必须是单一 JSON 文档，且 `status` 是大小写精确的字符串 `"ok"`、`logged_in` 是非 null 布尔值、`accounts` 是非 null JSON 数组。不能用 Docker `health=healthy`、任意 HTTP 200、任意合法 JSON 或额外一次 `/steam/ready` 代替。
- `logged_in=false` 返回 `RuntimeAuthServiceHealth{LoggedIn:false}` 并追加非阻断 warning，说明 Steam 在线能力暂不可用；它不阻止 Control-only 更新、Junimo 升级或 LAN/IP 模式启动。`accounts` 只用于计数，不进入日志或状态正文。
- 稳定错误码为 `auth_container_not_running`、`auth_digest_mismatch`、`auth_health_unreachable`、`auth_health_timeout`、`auth_health_http_status`、`auth_health_invalid_response`。HTTP body、账号名、密码、refresh token、session、ticket 和容器完整环境变量都不会进入错误；apply 最终失败、`causeCode/causeError` 和 `failed_rolled_back` 会保留脱敏后的最后一次具体探针原因，不再只写“认证服务接口验证失败”。接口、schema 或 digest 失败仍 fail closed，并进入既有认证卷快照、成对回滚与旧栈复验事务。
- 目标初验、最终目标复验、Panel 重启后的续验、SMAPI 新/旧栈验收和旧版本回滚统一复用这份纯健康契约。公开检查名 `checks[].name=steam_auth_ready` 暂时保留以兼容现有前端/API，但其语义已经是“容器 + digest + 严格 `/health` 服务契约”，不是 Steam 已登录。

## 实现、兼容性与验证

- 主要文件：`backend/internal/docker/runtime_apply.go`、`runtime_apply*_test.go`；`backend/internal/games/stardew_junimo/runtime_update_apply*.go`、`runtime_update_rollback.go`、`smapi_update_workflow.go` 及对应测试；`config/runtime_stack.go`、`runtime_stack_test.go`；`scripts/compatibility_matrix.py` 及单测。
- 内置推荐 `steam-auth-cn:1.5.0-anxi.2` 的精确源 revision 与真实镜像均已验证：`/health` 返回 HTTP 200 和 `status/logged_in/accounts`，且不会触发登录；`/steam/ready` 才执行登录与 ticket。Panel Git 历史运行栈 manifest 只出现过 `.2`。运行栈 inspection 与兼容矩阵现以显式 allowlist 固定该已审计契约；未审计 tag 返回 `unsupported/auth_health_contract`，拒绝自动升级且绝不静默回退 `/steam/ready`。
- 受控 Docker fixture 让 `/health` 立即返回 `logged_in=false`、让 `/steam/ready` 阻塞 60 秒，并通过请求日志证明维护验收从未访问 ready；另覆盖 `/health` 不可达、短超时、404、500、坏 JSON、目标 auth 发生变化、Control-only/auth 未变化、最终目标复验、旧版回滚复验与 digest 不匹配。真实镜像 opt-in 不使用真实 Steam 凭据，只验证 `/health` 契约。
- 2026-08-14 已通过：严格 parser 单测；Go 定向单测；两组 Docker integration；真实 `1.5.0-anxi.2` opt-in；兼容矩阵 20 项单测与 manifest validate；Linux `go test ./... -count=1`、`go vet ./...`、`go build ./...`。任务专属容器、镜像、网络和 volume 清理后均为 0。尚未构建正式候选、打 tag、创建 Release 或推送正式镜像。

# PLAYER-AUTH-MODES-1：IP 直连的全服密码与角色独立密码（2026-08-15，released in v0.4.19，included in v0.5.0）

## 配置与安全模型

- 玩家加入保护现在有显式 `none / global / role` 三种模式。旧实例没有 `SAP_PLAYER_AUTH_MODE` 时仍按 `SERVER_PASSWORD` 是否为空推断 `global / none`，不会因为升级突然改变行为。
- `.env` 新增 `SAP_PLAYER_AUTH_MODE`、`SAP_PLAYER_AUTH_REVISION`、`SAP_ROLE_AUTH_KEY`、`SAP_ROLE_PASSWORDS_B64`。角色身份使用存档中的数值 `UniqueMultiplayerID`，角色名只作展示；改名不会让密码失效，同一密码也不能跨角色使用。
- 角色密码不保存明文。后端生成每实例 32 字节随机密钥，并只存 `HMAC-SHA256(key, "sap-role-password-v1\0" + roleId + "\0" + password)` 的 base64url verifier。角色模式下 `SERVER_PASSWORD` 改为由同一密钥派生的内部随机 guard；旧密码 API 在角色模式返回 `409 role_auth_mode_active`，Docker 输出脱敏也覆盖 guard、密钥和 verifier payload。全服与角色密码均限制为 1–128 个 Unicode 字符并拒绝控制字符，避免无法通过游戏聊天输入或破坏 `.env` 行结构。
- `.env` 更新由读取合并改为同目录 `0600` 临时文件、文件 `fsync`、原子替换；Windows 使用 `MoveFileEx(REPLACE_EXISTING | WRITE_THROUGH)`。角色模式切换、角色 verifier、内部 guard 和 revision 在一次替换中提交，避免半套配置被容器启动读到。

## Driver、API 与 Control Mod

- `stardew_junimo.Driver` 新增 `GetPlayerAuthConfig` / `UpdatePlayerAuthConfig`。更新必须携带 `expectedRevision`，冲突返回 `409 player_auth_revision_conflict`；启用角色模式前会通过现有 `ListPlayers` 合并在线快照、缓存和存档 XML，并要求所有当前非主机角色已有或本次设置密码。更新继续进入 driver mutation ownership，不在 Web 层直接写 Stardew 状态。
- 新接口为管理员专用 `GET/PUT /api/instances/:id/config/player-auth`。响应只返回角色 `configured` 状态和计数，不返回角色密码/verifier/密钥；PUT 只提交本次 `rolePasswordUpdates`/`rolePasswordRemovals`。旧 `GET/PUT .../config/server-password` 保留兼容，分别映射显式 `global` 或 `none`。
- Control `0.3.3` 新增 Harmony prefix，补丁目标仍由既有 `PasswordProtectionBridge` 解析出的 Junimo `TryAuthenticate(long,string)` 提供。角色密码正确时把本次输入重写为内部 guard，错误、串角色、角色缺失或配置损坏时重写为不可能命中的 NUL sentinel；之后仍由上游 Junimo 执行失败次数、超时、隔离小屋清理和认证成功传送。Panel 的“批准认证”命令传入精确内部 guard，继续可用。
- `status.json` 新增 `playerAuthMode/playerAuthConfigRevision/rolePasswordPatchAvailable/rolePasswordPatchDetail`。`GET /password-status` 和新配置接口都返回 configured/runtime mode、revision、`restartRequired` 与补丁状态，前端能区分“已保存”和“当前进程已生效”。
- 运行栈 identity 更新为 `control-0.3.3`，两份 manifest 同步为 `0.3.3`；嵌入 DLL SHA-256 为 `7b304fc8c8e5913ba11d3081f48ba06b2cb38b35a125c705e2a09ac22132ab1e`。项目显式从 `$(GamePath)/smapi-internal/0Harmony.dll` 取得编译引用且不复制私有副本，运行时继续复用 SMAPI 已加载的 Harmony。

## 验证与后续边界

- Go 聚焦测试覆盖旧配置推断、角色隔离 verifier、无明文落盘/响应、全角色完整性、revision 冲突、旧 API 兼容、运行状态合并和 Docker 脱敏；干净基线叠加本功能差异后的 Linux `go test ./... -count=1`、`go vet ./...`、`go build ./...` 均通过。纯 C# 契约覆盖正确角色、串角色、缺失角色、Panel guard、损坏配置 fail-closed 与 global 旁路。
- Control 已在 `.NET 6 SDK + stardew_game-data` 上按标准 `/p:GamePath=/game /p:EnableModDeploy=false` 真实编译，结果为 0 errors；只有既有 ModBuildConfig analyzer 编译器版本 warning。前端与跨端验证见 `docs/03-frontend.md`、`docs/06-integration.md`。
- v0.4.19 候选已验证 none/global/role 配置、角色隔离、旧 API/旧 `SERVER_PASSWORD` 兼容、revision/补丁 fail-closed、Control-only 更新和回滚；v0.5.0 又从 v0.4.19 完成真实 Web 回滚/升级并保留该回归。尚未把“两个真人客户端实际输入不同角色密码”的人工联机记录纳入证据，维护者不得把自动策略/Control 夹具表述为真人联机已验证；后续涉及认证目标、attempts/timeout 或客户端交互的改动仍应补该人工矩阵。

# PLAYER-LAST-SEEN-SEMANTICS-1：从未在线角色的时间语义修复（2026-08-15，released in v0.5.0）

- 玩家 API 的 `lastSeen` 现在只允许来自 SQLite 名册的 `last_online_at`，不再回退到通用的 `last_seen_at`。后者只表示面板最近从 Control、缓存或存档 XML 观察到该名册项，不能证明玩家登录过。
- 存档 XML 补出的离线角色即使被连续轮询并写入名册，也会继续省略 `lastSeen`；真正在线过的角色离线后仍保留最后一次 `last_online_at`。现有污染数据无需清理，因为这类记录的 `last_online_at` 本来就是空值，部署修复后会自动停止展示假时间。
- `players_test.go` 使用真实 SQLite 连续执行两轮 `ListPlayers`，同时断言 API 始终不返回假 `lastSeen`、内部 `last_seen_at` 继续推进且 `last_online_at` 保持为空。Linux 容器中的 Stardew 玩家包与 storage 包全量测试通过。

# PLAYER-AUTH-SELF-ENROLL-1：角色密码首次登录自助设置（2026-08-17，released in v0.5.3）

## 当前契约与持久化

- 本节覆盖 `PLAYER-AUTH-MODES-1` 中“启用 role 前必须给所有当前角色预设密码”的旧约束。管理员现在可以在没有非主机角色、或存在待设置角色时直接启用 `role`；角色第一次提交合法 `!login 密码` 时，Control 会为当前活动存档的 `UniqueMultiplayerID` 计算 HMAC verifier、耐久写入后再放行本次认证。管理员仍可代设、重置或清除；清除后该角色回到 `waiting`，下一次首次登录重新认领。
- 角色 verifier 从实例级 `.env` payload 迁到每实例 Control 私有目录的 `role-passwords.json`，schema 按活动 `saveId -> roleId` 隔离；`role-passwords.initialized` 标记区分“从未初始化”与“已初始化但文件丢失”。标记存在而 store 缺失、JSON/schema/verifier 非法、活动 save 身份不可用或权限收敛失败时一律 fail closed，不能把损坏现场误判成可重新认领。
- Panel 与 Control 共用 `.role-passwords.lock` 的独占创建、随机 owner、5 秒有界等待和 2 分钟 stale lock 处理。写入采用同目录临时文件、flush/fsync、Linux `0600`、先耐久标记再原子替换 store；Panel 管理更新还把 store 与 `.env` revision/key 作为一个可回滚事务，任一步失败恢复原快照，回滚本身失败返回 `player_auth_transaction_rollback_failed`。
- 旧 `SAP_ROLE_PASSWORDS_B64` 中已有 verifier 不丢失：新 store 尚未初始化时，只把 legacy 记录绑定到当时活动存档并在首次 Control 登录或 Panel mutation 时写入新 store；新 store 一旦初始化，后续存档各自拥有独立记录，不能跨档复用或由 orphan 角色自动认领。

## 生命周期、Compose 与运行栈

- 旧实例启动/重启前由 `EnsureServerPlayerAuthEnvironment` 幂等补齐 `SAP_PLAYER_AUTH_MODE`、`SAP_PLAYER_AUTH_REVISION`、`SAP_ROLE_AUTH_KEY`、`SAP_ROLE_PASSWORDS_B64` 的 Compose 透传；重启改用 `docker compose up -d --no-deps --force-recreate server`，确保新环境进入 server 且不重启 `steam-auth`。迁移采用原权限原子替换，不覆盖用户其它 Compose 内容。
- 自定义 `server.environment` 若使用 inline/flow 或混合语法而无法安全改写：当前 `role` 模式继续阻止启动/重启并进入 `player_auth_compose_migration_failed`；旧 `none/global` 模式保留原 Compose、写入明确 job warning 后继续。这一兼容回退只适用于不依赖新增 SAP 透传的模式，不能用于绕过角色模式 fail-closed。
- Control 从 `0.3.3` 更新到 `0.3.6`；两份 manifest、标准 `/game` 实编译产物、嵌入 DLL 和 `runtime_stack_manifest.json` 同步，DLL SHA-256=`e7f3744b647c2f658ac3ad60d1dc27d958d935c7946f134b35447ab6c79bb422`。`RolePasswordPolicy` 仍只改写 Junimo `TryAuthenticate` 输入，attempts、timeout、隔离与传送继续由上游执行。

## 影响文件、验证与下一步

- 主要文件：`role_credential_store.go` 及平台原子替换文件、`player_auth_config.go`、`server_player_auth_compose.go`、`lifecycle.go`、Docker `compose.go`，Control `RoleCredentialStore.cs` / `RolePasswordPolicy.cs` / `RolePasswordPatch.cs` / `ModEntry.cs`，两份 manifest/DLL 与运行栈清单。
- Go 回归覆盖首次认领、重复/错误/串角色、按存档隔离、legacy 迁移、marker/store 损坏、并发锁、权限、Panel/Control 交叉写、事务回滚、Compose 迁移与 none/global 兼容回退；Docker Desktop、Control 标准编译和完整门禁结果记录在 `docs/09-image-build.md`。
- 两个真人客户端的首次认领、重复正确、交叉失败、管理员清除后重新认领、Panel 批准和 server recreate/Panel restart 保持已由用户确认；能力随 `v0.5.3@ede7fa34231600cbfa83050b4ddb6fd650373ae1` 正式发布。
- 前序阶段明确不打 tag、不创建 Release、不提升 `latest`；2026-08-17 用户已确认两个真人客户端的各自首次设置、重复正确登录、交叉密码失败、管理员清除后重新认领、Panel 批准和重启后保持矩阵全部通过，并授权进入正式候选。自动 C#/Go/Docker 夹具仍只作为补充契约证据。

# INSTALL-SMAPI-LIVE-PROGRESS-1 / STEAMCMD-MIGRATED-AUTH-REUSE-1（2026-08-18，未发布）

## SMAPI 下载进度契约

- 安装流程进入 `smapi_installing` 后，先写入可读日志说明正在检查本地安装包缓存；需要下载时，`smapi_archive.go` 从每次响应体写入中取得实时字节数，而不是等一个 2 MiB Range 请求全部结束后才更新。候选源切换会把进度归零并递增候选编号。
- job log 控制标记固定为 `[smapi:download:progress:<downloaded>:<total>:<candidate>:<candidateCount>:<cached>]`。`downloaded/total` 是十进制字节数，候选编号从 1 开始，`cached=true` 只在既有目标文件通过长度、SHA-256 和 ZIP 安全校验后产生。该标记不包含 URL、凭据或本地路径。
- `installer.go` 按候选切换、完成、缓存命中、累计至少 512 KiB 或间隔至少 2 秒节流写入 marker，并同步更新实例 `stateMessage`。下载结束后另写“正在校验完整性”，校验成功后再写“正在写入游戏运行目录”，不能把收到最后一个字节等同于完整性校验成功。
- 非生产测试 Docker provider 没有进度回调时，installer 在 provider 成功返回后补一条已校验缓存完成 marker，保持测试替身与生产前端契约一致；生产 Docker service 仍走受审查 URL、Range、长度、checksum 与 ZIP 限制的原下载器。

## 旧 SteamCMD 授权缓存即时复用

- `migrateLegacySteamCMDAuthCache` 现在只在迁移容器退出 0 且明确输出 `canonical cache already present` 或 `migrated legacy cache` 时返回可用。若本次不是更换账号/强制重新授权、`.env` 旧版本又尚无 `STEAMCMD_AUTH_COMPLETED=true`，同一次重装会立即把 `steamCMDUseCache` 置为 true，先执行 username-only、`NoPromptForPassword` 的免验证登录。
- 缓存缺失或失效仍沿用现有一次性自动回退：清除失效状态标记，随后用账号密码完整登录并按需进入 Steam Guard。迁移成功本身不会提前写 `STEAMCMD_AUTH_COMPLETED=true`；只有 SteamCMD 实际登录并开始/完成下载后才持久化该标记，避免把仅存在的旧文件误记为有效授权。
- 更换账号路径继续先清除 session、统一 SteamCMD 卷和两个 legacy 卷，且不启用迁移缓存；游戏数据卷不受影响。

## 影响文件与验证

- 主要文件：`backend/internal/games/stardew_junimo/{smapi_archive.go,installer.go,smapi_archive_test.go,driver_test.go}`。
- 新增回归覆盖流式中间字节、最终完成、已校验缓存命中，以及“没有完成标记但迁移到旧授权缓存”的修复安装直接免验证登录；既有缓存失效后完整登录回退与已标记缓存修复回归继续通过。
- 2026-08-18 Windows 宿主定向五项安装/下载回归通过。全包宿主测试仅命中既有 Linux 权限断言 `Compose mode=0666, want 0640`，不是本功能失败；正式候选仍必须在 Linux 文件系统完成全量 Go 门禁和真实 Docker 安装 E2E。本节代码尚未部署生产、未创建 tag/Release、未提升镜像。

## 新建存档农场山洞选择（2026-08-23，released in v0.5.12）

- `POST /api/instances/{id}/saves/new` 的 `NewGameConfig` 新增 `farmCaveChoice`，只接受 `vanilla | bats | mushrooms`；字段缺失时后端补为 `vanilla`，未知值在创建任务前返回参数错误。该值随其它面板专属新建参数写入 `server-init.json`，没有修改 Junimo 上游代码或协议。
- Control `0.3.7` 在既有 target marker、事务 ID、预期新建存档名和玩家身份全部吻合后执行一次精确转换：`vanilla` 写 `caveChoice=0`、移除事件 `65` 并清理蘑菇设施；`bats` 写 `1`、记录事件 `65` 并清理蘑菇设施；`mushrooms` 写 `2`、记录事件 `65`，并调用游戏公开的 `FarmCave.setUpMushroomHouse()`。这层转换兼容 Junimo 在 Control `SaveLoaded` 前已把新存档预置为蘑菇洞的实际行为，保护条件不满足时不会修改已有存档。
- Control 回读 `caveChoice`、事件 `65`、蘑菇箱/脱水机数量并写入 runtime status；Panel verifier 同时校验 Control 事务证据和主存档 XML。成功需要 `farmCaveChoiceApplied=true`、`farmCaveChoiceVerified=true`、事务/存档/时间匹配以及磁盘状态一致；错误码区分无效选择、Control 应用失败、Control 状态不一致和磁盘不一致。
- 主要文件：`registry/types.go`、`stardew_junimo/{saves.go,new_game_durability.go,new_game_real_integration_test.go}`、Control 源码/契约测试、嵌入 DLL 与 runtime stack manifest。最终嵌入 DLL SHA-256 为 `bf8ba2026e33f62007e3d1cfca59b055da94806cc17dc999d62a1c94b2e39423`。
- 验证：三种选择的 Go/Control 契约、默认值与非法输入、XML 正反例、Control 标准真实游戏程序集编译均通过；Docker Desktop 真实双写者 E2E 先创建蝙蝠洞、再创建蘑菇洞，二者均通过 runtime status 与主存档 XML 双重校验，源游戏卷及旧存档哈希不变。完整 `stardew_junimo/...` Linux 包回归通过。
- 正式候选 `32623320406` 完成路径选择的完整后端 test/vet/build、Junimo 真实网络/runtime integration、fresh/restart、`v0.5.11 → v0.5.12` Web 升级与 unhealthy 回滚；自动 tag `32623853636` 和正式提升 `32623863894` 成功。能力已随 `v0.5.12@5141cd54` 和唯一 digest `sha256:faf910075f4b25a3172fe4ee53341cf53b9c3c26c1065ce38b65c19fcc9af5a0` 发布。
