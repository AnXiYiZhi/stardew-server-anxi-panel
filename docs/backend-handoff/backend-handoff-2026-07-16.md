# SAVE-NAME-ENCODING-DELETE-1 接手记录（2026-07-20，completed）

## 改了什么

- `saves.go` 在 ZIP 解压前严格规范 GBK/GB18030 文件名到 UTF-8，并验证准确的 Stardew 双文件布局。新名称限制为有效 UTF-8、180 字节内、无控制字符且满足 Junimo 安全 token；重复或仅大小写不同的路径会拒绝。
- 历史原始字节目录通过 `publicSaveNameAtRoot` 映射。可解码且唯一时显示中文；不可解码或与 UTF-8/其它旧目录冲突时使用 SHA-256 派生别名。所有备份、导出、存在检查与删除都重新解析公开身份到原始 `DirEntry.Name`，禁止把 JSON replacement string 当路径。
- `DeleteSaveWithBackup` 先备份、再清活动指针、后删除目录；清指针失败时不删，删除失败时恢复指针。Web 删除先做存在校验，重复请求返回 `save_not_found`；成功删除活动存档后写 `save_required`。select 两条接口对旧编码目录返回 `save_name_encoding_invalid`。

## 影响文件、验证与后续注意

- 主要文件：`backend/internal/games/registry/types.go`、`backend/internal/games/stardew_junimo/saves.go`、`saves_test.go`、`backend/internal/web/lifecycle_handlers.go`、`saves_handlers_test.go`。
- Windows 全量 test/vet/build、Linux 原始 GBK 名/同名冲突测试、Docker integration、兼容矩阵和独立 Panel HTTP E2E 均通过。E2E 证明 GBK ZIP 返回中文，旧目录有 warning、不能激活、可生成 UTF-8 备份后删除，二次删除为 404，重启不复现。
- 后续如把保存管理完整迁入 Junimo driver，应保留本次“公开身份与原始目录字节分离”和“删除失败不能掩盖磁盘终态”的契约；不要自动重命名未知历史目录，也不要恢复 API 层覆盖导入。

# FARMHAND-DELETE-1 接手记录（2026-07-18，completed）

## 改了什么

- `farmhand_delete.go` 新增 `stardew_farmhand_delete` 事务：两次 `/farmhands` 与玩家在线校验、可确认 broadcast、前保存、`prefarmhanddelete` 整档备份、Junimo DELETE、运行态复核、后保存和磁盘 farmhand 复核。只允许运行实例；目标在线/主机/未认领人物不删除，其他在线真人只触发风险通告。
- `players.go` 输出存档人物 capability；`player_roster.go` 与迁移 011 用墓碑隐藏成功删除的人物，权威存档重新出现时清墓碑。生命周期启停、重启、回档会拒绝与活动删除 job 并发。
- Web 接口位于 `players_handlers.go` 与 `instance_handlers.go`。存档备份类型在 `saves.go` 增加 `prefarmhanddelete`。不要把这条链路改回 API 层直接写 XML；Junimo DELETE 是删除人物、小屋和 slot 的唯一执行者。

## 影响文件与验证

- 主要影响：`backend/internal/games/stardew_junimo/{farmhand_delete,players,saves,lifecycle}.go`、`backend/internal/storage/player_roster.go`、迁移 011、Web 路由与对应测试。
- 相关包测试通过。Docker Desktop 隔离真实 `.125` 存档验证停服拒绝、显式确认、删除/保存/备份、人物与小屋计数、重启、重复删除、真实 broadcast succeeded，以及保护备份整档恢复。

## 下一步注意事项

- 上游当前删除小屋不会向已连接客户端广播 demolition；因此不要删除确认框和重连通告。若上游未来提供明确建筑同步事件，需用真实两个客户端验证后才能收窄警告。
- 保护备份是整档恢复，不是单人物恢复。任何失败若发生在 Junimo DELETE 之后、最终保存确认之前，错误必须明确提示 backup 名称，禁止自动重复 DELETE。

# IMAGE-CLEANUP-1 接手记录（2026-07-17，completed）

## 改了什么

- Panel apply 成功验收后会再次核对原 tag 的 image ID，并无强制删除升级前镜像；随后枚举同 OCI title 的镜像，只清理可信仓库中未被任何现存容器引用的旧稳定 tag/陈旧 `latest`，最后执行同 label 的非 `-a` dangling prune。清理失败只落脱敏 warning，不改变成功终态。
- `0.3.6 → 0.3.7` 的 helper 来自旧镜像，不能直接携带本功能；因此新 Panel main 启动 `ReconcileCompletedImageCleanup`，只观察本次目标版本的 active/succeeded 状态，并在旧 helper 成功后补做一次清理、原子写 `cleanupCompleted=true`。新 helper 正常路径也写该字段，保证幂等。
- Junimo/auth pair 成功后使用 recovery manifest 已保存的旧引用和旧 image ID 调用受限 `RuntimeRemoveImage`；正常完成和 Panel 重启续验成功均覆盖，任意失败/回滚路径不清理旧镜像。
- Docker 删除始终不带 `--force`。共享容器、tag 漂移或本地镜像缺失时 Docker/核对门禁会拒绝，升级状态保留明确 warning；容器、volume、game-data、steam-session、数据库、存档和 Mod 均不在本功能范围。

## 影响文件与验证

- 影响：`backend/internal/updater/apply_helper.go`、`backend/internal/docker/runtime_apply.go`、`backend/internal/games/stardew_junimo/runtime_update_apply.go`、`runtime_update_apply_runner.go` 和对应测试。
- 已通过：后端全量 test/vet/build、前端全状态脚本/build、兼容矩阵本地/远端制品、run.sh 门禁；Docker Desktop 覆盖 Panel 成功清理、历史 tag、容器引用、自定义仓库、unhealthy 回滚、精确 image ID、真实 server/auth 探针、volume/SMAPI staging，以及 `.121 → .125` stopped/running。真实 `0.3.7` 生产镜像还覆盖 fresh volume health/version/OCI/updater 和旧 helper succeeded 后的新 Panel 收尾。

## 下一步注意事项

- 下次成功 Panel 升级会同时处理宿主已积累的可信历史 tag、陈旧 `latest` 和带 OCI title 的 `<none>` 镜像；自定义仓库、未知 tag 与任意现存容器引用继续保留。若 Docker 因共享引用拒绝删除，apply warning 会提示管理员核对。
- 若未来提供 Web 手动清理入口，必须先返回预览并继续限制可信仓库、OCI title、容器引用和 active/rollback_failed 门禁；禁止直接暴露 `docker image prune -a`。
- `cleanupCompleted` 是 apply 状态新增的可选字段；旧前端可忽略。不要把它当作升级成功门槛：镜像清理失败仍是 warning，`succeeded` 的健康验收语义不变。

# SAVE-IMPORT-E2E-RELEASE-1 接手记录（2026-07-16，真实 E2E 缺失）

## 本轮完成的检查

- 保护并保留了工作区全部未提交修改；只读检查 `.codex-test/save-import-spike`、Docker 容器/卷和已有归档，没有启动、停止或改写任何实例。
- 隔离 spike 的两个归档 SHA256 为：`original-saves-before-import.tgz = 8bfd50bf7c5f9b8aa94385d5c0c80afcf892c168b49f801e9495033bf1c5109d`，`generated-sources-before-import.tgz = 9863d782aa397ff693be9753b2050666529cbbd8aaaa8a3ab5566e5741a64d95`。归档只含早期技术 spike 存档，不是普通单机/多人主机/住宅家庭/Mod/takeover 八类 ZIP 夹具。
- 非缓存完整 Go test、专项 import test、vet 和 build 均通过；静态审计确认 Web 提交走事务 driver、缺 hostHandling 被拒绝、平台 ID 只持久化 operation-salted fingerprint、preimport 不参与自动清理、日志/单指针不能独立确认成功。

## 未完成与下一步

- 未执行真实角色选择、技能/物品/金钱/关系、家具/冰箱/地窖、配偶/孩子/宠物、住宅绑定、断线重连、跨日、第二次重启及故障注入。当前没有满足安全要求的完整存档 ZIP 组合和游戏客户端测试条件。
- `SAVE-IMPORT-JUNIMO-1` 必须继续保持未完成；不得把单元测试或早期 direct-Junimo spike 改写成真实 Panel E2E。
- 上游仍没有 import commandId。后续验收必须沿用 Panel 的磁盘事务痕迹 + pending + saveId + finalizeCount + GameLoop.Saved + dayTransitionComplete 复合证据，并保存每个夹具的原始 ZIP、SHA256 和测试后证据副本。

# SAVE-IMPORT-WEB-API-1 接手记录（2026-07-16，completed）

## 改了什么

- 新增 `internal/web/save_import_api.go`，正式定义嵌套 hostHandling DTO、公开 mode 到 driver mode 的映射、平台 ID 字符串校验和稳定事务错误映射。commit handler 不再接受缺省 takeover，也不再调用旧覆盖导入/指针/普通 Start 链。
- durable pending token 新增持久 `jobId`、`reserveOrReuse/lookup/attachJob`。operationId 只由后端生成；相同 token 与同一 fingerprint 决策可在 handler 返回后或 Panel 重启后拿回原 `202`，不会重复建 job。
- owned token 的 Web cancel 现在只返回 `save_in_progress`，绝不删除事务 source/journal/staged/preimport。driver 在 ownership 后 job 创建失败时返回并记录 `save_in_progress`；handler 仅在 token 仍是 reserved 时 release。
- `CreateImportJournal` 的幂等比较扩展到 instance/save/hostHandling/platform fingerprint，避免同 operation 被不同主机决策复用。原始 platformId 只在请求闭包内存中存在；审计仅记录公开 mode、saveName、operationId、jobId。

## 接口与影响文件

- 请求：`{token, hostHandling:{mode:"swap_to_player", platformId:"..."}}`，或 `{token, hostHandling:{mode:"virtual_host_takeover", acknowledged:true}}`。
- 成功：`202 {jobId,operationId,saveName}`。入口/冲突错误见 `docs/06-integration.md` 顶部契约。
- 主要文件：`backend/internal/web/{save_import_api.go,pending_uploads.go,lifecycle_handlers.go}`、对应测试、`backend/internal/games/stardew_junimo/save_import_transaction.go`。未修改前端页面、Junimo 上游、XML，也未增加 `/test/*` 调用。

## 验证与下一步

- 专项覆盖两种 mode、缺失/未确认选择、格式与 uint64 大整数、稳定错误映射、token 重放/取消/重启发现、202、journal/audit 脱敏和运行中保护；全量验证命令为 `go test ./...`、`go vet ./...`、`go build ./...`。
- 下一步是独立前端接入任务：不得把 platformId 转 number，不得恢复旧顶层字段或默认 takeover。若 job 已 submitted/unknown，UI 不得提供盲重试。
- 下方原 `SAVE-IMPORT-JUNIMO-1 blocked` 是历史记录，必须继续保留；当前实现已经使用 Panel 黑盒复合证据，缺少 commandId 不是 blocker。

# SAVE-IMPORT-DURABLE-SAVE-1 接手记录（2026-07-16，completed）

## 改动

- 新增 `save_import_durable.go`：activation 从 `finalize_confirmed` 交接，swap 先采集主文件与 status baseline、预持久化 Control commandId，再发布一次 save-now；只接受同 ID 的 GameLoop.Saved succeeded。
- Saved 后读取 post-Saved status cursor，优先使用 8 秒 `/wait/status` 长轮询等待更新的 dayTransitionComplete=true；404/405 才降级低频 `/status`。optional bool/version 缺失不伪造默认值。
- 最终门禁为稳定主文件、严格 `SaveGame` XML、hash/mtime 变化；通过后写 `save_verified` 再写 `completed`。as-is 不保存，只复核目标世界。
- journal 新增 durable command/before/after/status/Saved/transition/warning 字段和 `finalize_confirmed/save_persisting` 阶段。`Prepare` 将这些阶段标记 `resume_save_verification` 并启动仅观察原 commandId 的恢复 job。
- 没有停止/重启逻辑、XML 写入、preimport 清理或 import writer。Control 原有 OnSaved 仍生成 save-event，自动游戏日回档链未改变。

## 影响文件与验证

- 核心文件：`save_import_durable.go`、`save_import_durable_test.go`、`save_import_activation.go`、`save_import_transaction.go`、`console.go`、`driver.go`。
- 测试覆盖 Saved 成功、只有 transition=true、false→true、transition timeout/missing、提交/command failed、unknown、稳定文件、损坏 XML、无文件变化、as-is、completed 严格门禁、重启后同 commandId 恢复和零重复 import。
- REST 文档补充 `/wait/status.dayTransitionComplete` 与 Panel durable-save 使用边界。
- 2026-07-16 已通过专项 `go test ./internal/games/stardew_junimo -run "ImportDurable|ImportActivation|RecoverImport|RequestSaveNow" -count=1`、driver 全包、`go test ./...`、`go vet ./...` 与 `go build ./...`。

## 下一步注意

- `DurableGameLoopSaved=true` 但 stage 仍为 `save_persisting` 表示写盘已经发生、世界或文件验证尚未完成；不得停止 runtime 或再次提交 save-now/import。
- completed 事务仍保留 preimport。首次 GameLoop.Saved 的 save-event 由既有备份维护请求消费，不要在 import runner 内提前删除。

# SAVE-IMPORT-ACTIVATION-1 接手记录（2026-07-16，completed）

## 改动

- 新增 `save_import_activation.go`，Phase A confirmed 后先观察原进程 reload，再在必要时执行一次受控 activation restart。重启前调用 `ApplyModProfile(saveName)` 并持久化 `save_activating`；Phase A timeout 已停机时走 ComposeUp，运行中走仅 server service restart。
- 新增只读 activation evidence：Control RuntimeSaveID、pending、ProcessIdentity、diagnostics count/master/错误码、`/status` online/ready/dayTransitionComplete/playerCount。swap 同进程严格比较 pre-submit count+1；新进程比较该代 baseline+1，或首次可见已完成时要求 count=1。
- swap 成功还要求 pending clear、`masterName=Server`、目标 RuntimeSaveID 与稳定世界。as-is 不要求 count。pending clear/count unchanged 或 wrong target 进入 recovery；diagnostics/status 不完整进入 unconfirmed；pending 持续且目标不加载进入 activation timeout。
- 玩家检查失败不会假定无人在线；检测到玩家明确返回且不踢人、不重启。整个文件没有 FIFO import writer、pointer/XML 写入或 `/test/*` 调用。

## 影响文件与验证

- 核心文件：`save_import_activation.go`、`save_import_activation_test.go`、`save_import_transaction.go`；journal 兼容新增 `activationEvidence/activationOutcome/activationRestarted/activationProcessBaseline`。
- 专项覆盖 reload 直接成功、reload skipped 后重启、RuntimeSaveID 不切换、同进程 +1、新进程 count=1/baseline+1、pending clear/count unchanged、pending 持续、diagnostics failed/unavailable、as-is、零重复 import 和玩家保护。
- 2026-07-16 已通过 `go test ./internal/games/stardew_junimo -run ImportActivation -count=3`、`go test ./internal/games/stardew_junimo -count=1`、`go test ./...`、`go vet ./...` 与 `go build ./...`。
- activation 现写 `finalize_confirmed`；DURABLE-SAVE-1 已接续 GameLoop.Saved 与磁盘门禁并写 completed。维护 runtime 在持久化完成前仍保持不可加入语义。

## 下一步注意

- 上游 `TryFinalizeOnLoad` 在 wrong-save、前置条件失败、catch 和 finally 路径都会清 pending，只有完整成功路径递增 count。因此后续不得放宽为“pending 清空即成功”。
- 保留下方 `SAVE-IMPORT-JUNIMO-1 blocked` 历史；它记录旧的单一 commandId 诉求，不代表当前黑盒证据适配被阻塞。

# SAVE-IMPORT-PHASE-A-1 接手记录（2026-07-16，completed）

## 改动

- 新增 `save_import_phase_a.go`：从 `runtime_ready` 做严格 pre-submit evidence/preimport 校验，只写一次正式 `.125` FIFO import，并以磁盘 hash、pending 与 SaveNameToLoad 判定 Layer A。
- `save_import_transaction.go` journal 增加 pre-submit/Phase A evidence、log offset、outcome、restore hash、脱敏失败详情和 submitted 时间；job runner 用 `saveImportRunMu` 覆盖 staging、maintenance 和 Phase A。
- swap 只在 changed hash + matching pending(save/owner/fingerprint) + target pointer 时 confirmed；as-is 只在 unchanged hash + no pending + pointer transition 时 confirmed。日志内容不参与成功判定。
- timeout/cancel 先 ComposeDown 并等待 server 退出；no-effect 可 cleanup，matching pending/旧 pointer 进入 recovery，hash changed/no pending 从 preimport 恢复，其他矛盾进入 unconfirmed。没有自动 retry，也没有旧 Go import/XML 路径。

## 影响文件与验证

- 核心文件：`save_import_phase_a.go`、`save_import_phase_a_test.go`、`save_import_transaction.go`、`driver.go`；HTTP 请求/响应形状不变，journal schema 向后兼容新增可选字段。
- 专项测试覆盖完整 swap/as-is、成功日志、单指针、半转换、pending save/fingerprint/OwnerUid 异常、FIFO 失败、停机后迟到成功、恢复及 hash 不一致、日志脱敏、单次写入和重启识别 submitted。
- 2026-07-16 已通过 `go test ./internal/games/stardew_junimo -run ImportPhaseA -count=1` 与 `go test ./...`；最终 vet/build 见本任务交付。

## 下一步

- `import_confirmed` 只是 Phase A。下一阶段必须在已停止或受控重启边界完成 clean activation，比较 baseline/after 的 process identity、finalizeCount、pending clear、runtime saveId、master/farmhand 与迁移结果；不能因本阶段 confirmed 把任务标 completed。
- 下方 `SAVE-IMPORT-JUNIMO-1 blocked` 是历史调查记录，继续保留；缺 commandId 不再是 blocker，但矛盾证据仍必须保持 unknown/recovery，禁止盲重试。

# SAVE-IMPORT-MAINTENANCE-RUNTIME-1 接手记录（2026-07-16，completed）

## 改动

- 新增 `save_import_maintenance.go`：独立 ComposeUp/就绪探针，不复用普通 Start 成功判定。顺序为 `.125`/DLL 静态校验、staging/preimport、ComposeUp、server running、FIFO、log、health/status、容器内 manifest、裸 `saves`、玩家复查、baseline。
- `save_import_transaction.go` 的 job 在 `backup_created` 后进入维护 runtime；journal 新增 `maintenanceStarted/runtimeBaseline/serverOutputLogOffset`，全部证据成功后才写 `runtime_ready`。
- `driver.go` reconcile 保留 `stopped/save_import_maintenance`；`lifecycle.go` 的邀请码读取在维护 phase 拒绝。维护状态会隐藏旧 `invite_code`，失败时恢复原 payload。
- 任一 ready 前失败或 cancel 会停止本 job 启动的 Compose runtime，恢复原 stopped state/phase/message/payload；不清 staged/preimport。玩家连接返回 `save_import_players_connected`，没有 kick 行为。

## 影响文件与接口

- 后端：`save_import_maintenance.go`、`save_import_maintenance_test.go`、`save_import_transaction.go`、`driver.go`、`lifecycle.go`、测试 fake。
- 上传提交 HTTP 形状不变；实例 phase 新增可观察值 `save_import_maintenance`。这是 MAINTENANCE-RUNTIME-1 完成时边界；PHASE-A-1 已在 `runtime_ready` 后发送正式 import，但仍只确认 Layer A。

## 验证与下一步

- 专项覆盖启动顺序、FIFO/API/版本/启动失败、cancel、状态恢复、邀请码隐藏、指针不变、无 import/newgame、baseline、ProcessIdentity 变化和玩家连接。2026-07-16 已通过 `go test ./internal/games/stardew_junimo -run ImportMaintenance -count=1`、`go test ./...`、`go vet ./...`、`go build ./...`。
- 下一任务必须从已持久化的 `runtime_ready` baseline/log offset 继续；提交前再次检查玩家与 ProcessIdentity。保留下面 SAVE-IMPORT-JUNIMO-1 blocked 历史，但不要把缺 commandId 当成新 blocker。

# SAVE-IMPORT-STAGING-1 接手记录（2026-07-16，completed）

## 改了什么

- commit 的同步安全边界改为 journal 创建成功后调用 token store ownership callback，把持久 preview payload 移入 `save-import-transactions/<operationId>/source` 并标记 token `owned`；job 不再引用会随 handler/进程失效的 tempDir。
- 新增 `StageImportedSaveNoReplace` 与 Linux/Windows/其它平台 rename helper。Linux 用 `RENAME_NOREPLACE`，Windows 用 `MoveFile`；EXDEV 走 Saves 内隐藏临时目录复制、required-file + 全树 fingerprint 校验和原子发布。没有调用旧 `ImportSaveToVolume`。
- journal 增加 `sourceOwned`、`stagedSaveCreated`、`stagedSaveFingerprint`、`preimportBackupSha256`。阶段只在对应磁盘事实成功后推进。
- 新 `BackupPreImport` 备份上传目标，名称携带 save、operation 摘要和纳秒时间，kind 为 `preimport`。`OriginalActiveSave` 只保留上下文，不创建额外备份。
- cleanup 策略固定为：pre-submit 删除 source；staged target 仅在 fingerprint 未变化时删除；preimport 永久保留供恢复；target 变化或 submitted 后返回 recovery required。staging/cleanup 共用锁避免 cancel 与异步 job 竞态。

## 影响文件与接口

- 新增 `save_import_staging.go`、`save_import_rename_{linux,windows,other}.go`、`save_import_staging_test.go`。
- 修改 `save_import_transaction.go`、`saves.go`、registry `SaveImportRequest`、`pending_uploads.go`、`lifecycle_handlers.go` 及测试。
- HTTP URL/请求/成功响应不变；内部 token 增加 `owned`。cancel owned token 现在执行安全 transaction cleanup，而非一律 `token_reserved`。

## 如何验证

- `cd backend; go test ./...`
- `cd backend; go vet ./...`
- `cd backend; go build ./...`
- 专项覆盖 ownership、handler 返回后 source、重启发现、同名零修改、原子 rename、EXDEV copy、copy interruption、journal 阶段、上传目标 preimport、完整 restore、auto prune 隔离、cancel-before-submit 和 submitted cleanup 拒绝。全量 `saves_test.go` 继续覆盖 ZIP slip、绝对路径、多顶层目录、大小限制等旧安全矩阵。
- 2026-07-16 实际执行三条要求命令均通过；另以 `GOOS=linux go build ./internal/games/stardew_junimo` 验证 Linux `renameat2(RENAME_NOREPLACE)` 分支可编译。

## 下一步注意事项

- 这是 STAGING-1 完成时的历史边界；MAINTENANCE-RUNTIME-1 已在 `backup_created` 后启动专用 runtime，并只发送裸 `saves` 探测、采集 baseline 后停在 `runtime_ready`。正式 import 与 after 快照仍留给后续任务。
- preimport 是保留策略，不应加入 auto prune；显式删除仍可复用现有备份删除 API。
- 若 staged target 被外部修改，cleanup 会拒绝删除。不要为了“取消成功”绕过 fingerprint 或恢复旧 RemoveAll/overwrite 路径。

# SAVE-IMPORT-EVIDENCE-1 接手记录（2026-07-16，completed）

## 改了什么

- 新增只读 pending-finalize、Junimo diagnostics、运行证据快照和 journal fingerprint 比较。没有发送导入命令、接上传接口、写存档或修改 XML。
- 对保留的 `.125` 隔离实例 volume 使用 `--read-only --network none` 临时容器核验：`JunimoHost.SaveImport` 的真实文件是 `.local-container/saves/.smapi/mod-data/junimohost.server/junimohost.saveimport.json`。上游源码同时确认 `Pending` schema、finalizer clear 行为和 diagnostics 字段来源。
- pending 缺失/null 明确为 absent，损坏 JSON 明确失败；UserID 只在内存做 operation-salted SHA-256，类型禁止 JSON 序列化，比较仅三态。
- snapshot 双读并比对主存档/`SaveGameInfo` hash，读取精确 pointer、Control saveId、diagnostics、`/status.dayTransitionComplete` 和 hostname + PID 1 start ticks。部分不可读进入排序后的 `unknownFields`；pending 损坏不允许被吞掉。

## 影响文件与接口

- 新增 `internal/games/stardew_junimo/save_import_evidence.go` 与 `_test.go`；更新 `docs/02-backend.md`、`docs/06-integration.md`、`docs/08-future-roadmap.md` 和本接手文档。
- 没有新增 Web/API 契约。内部主要入口为 `ReadJunimoSaveImportIntent`、`ReadJunimoDiagnosticsState`、`ComparePendingIntent`、`CaptureJunimoImportEvidence`。
- 复用现有 `ComposeExecPipe` 和 `.env` API 配置；curl max-time 3 秒、Go context 4 秒。稳定错误区分 API unavailable/timeout、目标字段 missing/failed 和 JSON 损坏。

## 如何验证

- `cd backend; go test ./internal/games/stardew_junimo -run ImportEvidence -count=1`
- `cd backend; go test ./internal/games/stardew_junimo -count=1`
- `cd backend; go test ./...`
- `cd backend; go vet ./...`
- 专项用例覆盖任务要求的 pending、fingerprint、diagnostics、hash 变化、process identity 和 UserID 脱敏矩阵。
- 2026-07-16 实际执行以上四条命令均通过；全量 `go test ./...` 也覆盖了工作区已有未提交功能。

## 下一步注意事项

- 原 `SAVE-IMPORT-JUNIMO-1` blocked 段落必须保留为历史，不要再把“没有 commandId”单独当成阻塞；后续在 transaction runner 上做 Panel 黑盒复合回执。
- 本次 snapshot 是采集基础，不是成功判定器。下一阶段必须定义跨快照 baseline/after、进程代际变化、计数边沿、pending 消失、目标 saveId/pointer/hash 一致及冲突进入 unknown/recovery 的规则。
- 不得把 `UserID` 从 intent 结构复制到可序列化 DTO、日志、support bundle 或 journal；只传播 salted fingerprint 比较三态。

# SAVE-IMPORT-JUNIMO-1 接手记录（2026-07-16，blocked）

- 本次只做前置协议复核和文档记录，没有修改后端代码。工作区原有 SAVE-IMPORT-TXN-1 与其他未提交改动均保留。
- `.125` `SavesCommand` 仍返回 void；reload 仍为 fire-and-forget；跳过、拒绝、manager 未就绪和异步 fault 只有 Monitor 文本。正式 `ApiService` 无 import endpoint，返回 `ImportResult` 的 `/test/import_save` 明确是 test-only。
- Panel Control command-result v1 不消费 Junimo FIFO，不能作为 `saves import` 的回执。按任务要求不得解析日志绕过，故状态为 blocked。
- 下一步最小条件：上游提供 caller commandId + 持久 JSON 终态，覆盖所有失败/reload/崩溃路径；随后才能在现有 transaction runner 上实现 stage/backup/maintenance/FIFO/verify/profile/consume。不要改用 test endpoint，也不要恢复旧覆盖编排。

# SAVE-IMPORT-TXN-1 接手记录（2026-07-16）

- 调查/修改：基于 SAVE-IMPORT-SPIKE-1，新增持久 token、专用 import journal/job、版本/DLL/FIFO 门禁、重启恢复和全局互斥；commit 已断开旧覆盖链。本阶段不调用 Junimo import、不修改 XML、不接前端。
- 文件：`internal/games/registry/types.go`、`stardew_junimo/save_import_transaction.go`、`driver.go`、`lifecycle.go`、`internal/web/pending_uploads.go`、`lifecycle_handlers.go`、`instance_handlers.go` 及测试。
- 协议：job `stardew_import_save_and_start`；九阶段 journal；`202 {jobId,operationId,saveName}`；`save_exists/save_import_busy/token_reserved/junimo_import_unsupported`；恢复码 `import_result_unconfirmed/import_recovery_required/import_activation_timeout`。平台 ID 只持久化 salted SHA-256 fingerprint。
- 验证覆盖 token 并发/重放/过期/取消/释放/consume、同名字节不变、`.121/.125/.125+旧 DLL/FIFO 缺失`、每阶段重启、submitted 前可清理和 submitted 后禁止清理、敏感 ID 不进入 journal。
- 下一步：当前 stopped 提交不会启动 runtime，FIFO 门禁会阻止继续。上游提供 commandId + schema-versioned terminal receipt 后，才实现 start/FIFO submit/confirm/activate/verify/consume；禁止日志、sleep、pointer 单点或旧路径回退。

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

## GAME-LANGUAGE-1：服务器游戏语言（2026-07-16）

- 新增管理员 `GET|PUT /config/game-language`；默认中文，老实例首次接管继承已有合法 `startup_preferences` 语言。
- 权威配置位于 `.local-container/settings/game-language.json`；保存和每次 Compose 启动前同步 `languageCode` 与中文平滑字体开关。
- 影响 `game_language.go`、Web handler/route 和 `lifecycle.go`；`go test ./...` 通过。
- 后续若游戏增加官方语言，必须同时更新后端 allowlist 与前端语言清单，不要与面板 UI 国际化合并。

## SAVE-IMPORT-SPIKE-1：125 导入与跨日协议调查（blocked）

### 改了/调查了什么

- 未实现面板功能、未修改 Go 或 Stardew XML。只在 `save-import-spike*` 隔离实例和生成存档副本上验证 `.125` 的 `saves import`、FIFO、pointer、pending-finalize、Control saveId、`/status`、`/wait/status` 与虚拟主机结果。
- swap 在活动世界可 in-process reload/finalize；无活动世界会跳过 reload，可靠路径是 queued 事实后受控重启。as-is 会让 headless host 接管原 owner；swap 则创建 `Server` master 并把原 owner 变为 bound farmhand。
- 错误矩阵与 pending 前后重复/改 ID 行为已覆盖；损坏 fixture 导入前后字节哈希相同。平台 ID 未写入普通文档/显示日志。
- `dayTransitionComplete` 实测为 true → false → Control `Saved` → true；`/wait/status` 支持该字段过滤。初始 true 时要先等 false，再等 true。

### 涉及文件与协议

- 只更新 `docs/02-backend.md`、`docs/06-integration.md`、`docs/08-future-roadmap.md` 和本接手文档。实验材料位于未跟踪的 `.codex-test/save-import-spike/`，原始与生成源卷均有 SHA256 归档。
- 上游只读核对：`docs/admins/operations/importing-saves.md`、`SavesCommand.cs`、`SaveImportService.cs`、`ApiService.cs`、`SaveImportTests.cs`，镜像 revision `89abe8e6a07b3aaee1c0b4fad080683b948645d9`。
- FIFO 只有文本输入；内部 `ImportResult` 未暴露。`save_import_executed/finalized` 虽为 JSON log event，但没有 commandId，失败/reload 分支也不完整，不能作为面板正式终态协议。

### 如何验证

- 精确命令、镜像 digest、逐场景观察、时序表、成功/失败/unknown 判定均记录于 `docs/02-backend.md`。
- 无世界 swap：pointer + pending 后重启，最终 `/status` online/ready、Control `saveId` 命中、pending null、诊断 `masterName=Server` 且原 owner customized/bound。
- as-is：重启后诊断 `masterName=OwnerCollision`、`saveImportFinalizeCount=0`。
- 跨日：version 96 为 false，随后 Control Saved event，version 98 为 true；false filter 返回 200，idle false filter 返回 408。

### 下一步注意事项

- 本任务状态为 blocked。没有上游 commandId + JSON 终态前不得实现正式面板导入，也不得解析 Monitor 文案、使用固定 sleep 或超时后自动重试。
- 最小上游补充是正式 import endpoint/结果文件，覆盖 import 与 reload 的全部退出路径；Control/Panel 继续确保平台 ID 脱敏。
- “玩家在线、非 force reload 拒绝”由精确源码与随 revision 的 E2E 测试固定，但本机缺 `.env.test`/游戏客户端，未重新执行现场联机分支；后续上游协议验收时补跑，不能把源码测试当成本次现场证据。
# SAVE-IMPORT-E2E-RELEASE-1 real-run addendum (2026-07-17)

- Real isolated `.125` takeover/as-is and swap reached strict technical completion and survived second restarts. The successful swap journal is in the local test artifact tree only; it records Phase A `swap_confirmed`, activation `swap_finalized`, matching Saved, transition=true and changed before/after hashes without raw platform ID.
- Real-run fixes touched `save_import_maintenance.go`, `save_import_evidence.go`, `save_import_activation.go`, `save_import_transaction.go`, `save_import_durable.go`, their tests, and the embedded Control mod. Control save-now must use native `SaveGameMenu`; direct `SaveGame.Save()` writes bytes but does not produce SMAPI `GameLoop.Saved`.
- Recovery of `import_confirmed/save_activating` is observation-only and never re-enters Phase A. Completed imports alone promote the instance from maintenance to running. Result lookup tolerates the scheduler archiving result files into SQLite.
- Verification passed on 2026-07-17: focused import tests, full `go test ./... -count=1`, `go vet ./...`, `go build ./...`, the Control contract executable, and a real `.125` completed/second-restart run. Remaining blocker is external semantic coverage, not upstream commandId; do not mark the umbrella item completed yet.
- Local isolation note: `save-import-e2e-release` and `save-import-e2e-release2` preserve finalizer-confirmed but Saved-unconfirmed recovery cases and remain running by design; `save-import-e2e-release3` is the accepted completed/second-restart case. Do not stop or delete these as generic test cleanup without reviewing their journals.

## 2026-07-17 local rich-save follow-up

- `save_import_maintenance.go` now waits for a complete baseline after `saves` registration instead of taking one timing-sensitive snapshot. While polling it still rejects connected farmhands, process-generation changes and pointer changes. `save_import_maintenance_test.go` covers delayed diagnostics.
- Maintenance also validates a non-empty, unchanged original active pointer before ComposeUp. The real no-pointer run had shown Junimo entering new-game creation; the new test proves no container starts in that case.
- Real isolated takeover operation `60953678b5ed8fd81bcca0252c9c17c0` used a copied local rich save and reached completed, then reloaded the target on a second restart with unchanged SHA256, `Pending=null`, ready/transition=true, three cabins and two farmhands. Full Go test/vet/build passed afterward.
- The successful `save-import-local-rich` runtime is intentionally still running for optional human inspection. Its noVNC WebSocket did not connect with the current FPS-zero configuration, so visual/game-client semantics remain open and the umbrella task remains incomplete.
# PANEL-POLL-LEAK-1 接手记录（2026-07-18，completed）

## 改了什么

- `lifecycle.go` 的邀请码路径只读 `/tmp/invite-code.txt`，空值由 Web 查询返回 `n/a`；driver 增加 5 秒按实例缓存与 singleflight，清理旧文件时失效缓存。禁止重新加入 attach-cli fallback。
- `resource_metrics.go` 把完整采样放入 5 秒按实例缓存并合并并发；`lifecycle.go` 为重启 job 写持久 operation payload，活动重启再次提交返回 `ErrRestartInProgress`，Web 映射 `409 restart_in_progress`。

## 影响与验证

- 主要文件：`backend/internal/games/stardew_junimo/{driver,lifecycle}.go`、`backend/internal/web/{handler,resource_metrics,lifecycle_handlers}.go` 及单元/Integration 测试。
- 专项测试覆盖 12 路并发邀请码/指标单次执行、空文件无 attach、重复重启保留原 job。Docker Desktop 29.5.3 用隔离 `bash:5.2` Compose project 验证真实 exec/stats；cleanup 后无运行测试容器或 attach-cli 进程。

## 下一步注意事项

- `n/a` 是“文件当前为空”，不是错误，也不能持久化进 driver payload。错误结果不缓存。若上游改变邀请码文件契约，应调整文件读取，不得恢复 attach-cli 轮询。
- 指标缓存时间戳代表实际采样时刻；不要在每个响应上伪造新时间戳。若增加多实例，应继续以 instance ID 隔离缓存和 flight。

# 2026-07-19：CONTROL-PAUSE-COMPAT-1

## 改了什么

- Control Mod 从 0.2.0 升至 0.2.1，新增纯函数 `PausePolicy`。无人暂停以 `Game1.server.connectionsCount` 为准，在每帧更新前后仅补写 `IsPaused=true`；删除了旧菜单暂停对 `gameTimeInterval` 和所有全局 pause flag 的保存、恢复与清除。
- 菜单暂停只接受“所有已自定义在场玩家都请求暂停，且连接数与在场玩家数完全一致”。这使密码认证、新角色捏人和连接过渡优先完成，不会被已有玩家的菜单状态卡住。
- source/embedded manifest 声明必需依赖 `JunimoHost.Server`，版本统一为 0.2.1；嵌入 DLL SHA256 为 `e01cfcdb8df3d06e541b4f011edd7b6f748ee351ed16f9bf0c8537fcc5b20015`，推荐运行栈清单已同步。

## 影响文件与接口

- 主要文件：`embedded/smapi-mod-src/{ModEntry,PausePolicy}.cs`、两个 Control manifest、嵌入 DLL、`runtime_stack_manifest.json` 和 `smapi-mod-contract-tests`。
- Web/driver API 没有变化；Junimo `/status.isPaused` 继续反映真实 `NetWorldState.IsPaused`，人物删除事务和备份流程不变。

## 如何验证

- Docker .NET 6 SDK 中运行 Control 契约测试并用只读 `stardew_game-data` 编译真实 Mod：0 errors（仅既有 analyzer/compiler 版本 warning）。
- Docker Desktop 隔离 `farmhand-delete-e2e` 项目使用真实 `.125`、独立卷和测试存档验证删除后 15 秒时间不推进、重启、时间边界与节日放行；Control 错误日志为 0。
- 发布前继续要求后端全量 test/vet/build、前端状态脚本与 production build、Docker integration、候选 Panel 镜像 smoke 全部通过。

## 下一步注意事项

- 兼容层必须保持“只补写 true、永不写 false”；不要恢复旧 `ClearSinglePlayerMenuPause()` 的全局清理。
- `connectionsCount` 是握手安全边界，`otherFarmers`/`farmhandData` 不能替代。未来若上游连接模型改变，应先增加真实客户端连接/断线 E2E，再调整策略。
- 节日和 2510..2600 必须继续放行，否则会卡住节日自动化或 2:00 晕倒/日结。
# RUNTIME-COLD-START-1 接手记录（2026-07-19，completed）

## 改了什么

- server 升级验收默认 20 分钟；server/auth stop 失败在 10 分钟内按同一 allowlist 幂等重试，避免 Docker daemon 短时失去调度就进入 `rollback_failed`。
- 新 Compose 与旧实例迁移增加 `steam-auth=256/server=768` 相对 CPU shares；升级 apply preflight 会先补齐兼容迁移。
- Docker 容量探针只解析 `NCPU/MemTotal`，低资源主机通过现有 apply warnings 提示 swap/swappiness 和长冷启动预期。

## 影响文件/接口

- 主要文件：`internal/games/stardew_junimo/{driver,compose_template,server_env_fix,runtime_update_apply_runner,runtime_update_rollback}.go`、`internal/docker/runtime_apply.go` 及对应测试。
- HTTP 路径和请求体不变；`GET .../junimo-update/apply` 可能多一个低资源 warning。成功硬门槛、镜像 digest、认证快照、原运行状态恢复和人工恢复边界不变。

## 如何验证与下一步

- 专项：`go test ./internal/games/stardew_junimo -run 'RuntimeUpdate|EnsureServerContEnvFix' -count=1`、`go test ./internal/docker -run 'RuntimeApply|RuntimeHostCapacity' -count=1`。
- Docker Desktop 29.5.3 已用真正的 `.121` 镜像与宿主 Mod fixture 跑通隔离 stopped/running 真升级（173.86 秒/106.34 秒），确认 `.125`、原状态恢复及 Compose/实际容器 256/768 CPU shares；全量 test/vet/build、Docker integration、兼容矩阵、前端状态矩阵、production build 和 `0.3.11-rc` smoke 均通过。
- Panel 不修改宿主 sysctl。低配 Linux 部署仍应由管理员在宿主确认 swap 与 swappiness；不要把 privileged sysctl helper 加进 Panel 容器。
# CONTROL-PAUSE-FEEDBACK-1 接手记录（2026-07-19，completed）

## 根因与修改

- 生产三人在线现场在 17:50 稳定复现 `AllGameplayPlayersRequestedPause`：Control 0.2.1 消费 `requestingTimePause` 后写 `IsPaused=true`，该全局暂停又维持请求位，导致每帧反馈锁。`world_freezetime 0` 在连接数重新匹配后会立即失效，容器、存档和性能均不是根因。
- Control 0.2.2 的 `PausePolicy` 只接收连接数、节日和时间边界；删除 gameplay player/menu request 计数及对应 enum。任何正连接数都返回 `None`，菜单暂停完全交还上游；零连接 610..2500 单向补写逻辑不变。
- `runtime_stack_manifest.json` 纳入 Control 0.2.2 identity/hash。`InspectRuntimeStack` 读取运行时 `options.json.controlModVersion`，旧进程返回 `control_update_available`；真实升级测试先走 `Prepare` 同步内嵌 Control，再用 required runtime 事务重启并校验实际加载版本。

## 影响、验证与注意事项

- 主要文件：`embedded/smapi-mod-src/{PausePolicy,ModEntry}.cs`、契约矩阵、两个 Control manifest/DLL、`config/runtime_stack_{manifest,test}.json/.go`、真实升级 integration test。API 形状不变，只新增稳定检查码 `control_update_available`。
- Control Docker .NET 6 契约与只读真实 game-data 编译通过（0 errors，1 个既有 analyzer warning），source/embedded DLL SHA256 均为 `547c08d8761d0a50fd713077ba9b6d5aa3db091df44be3a6400b6fdcf183f3a9`。
- Docker Desktop 29.5.3：`.121 -> .125` stopped/running 127.72 秒/120.12 秒；`.125 + Control 0.2.0 -> .125 + Control 0.2.2` stopped/running 144.61 秒/109.60 秒，四条链均恢复原状态且运行 options 报告 0.2.2。
- 不要重新把 `hasMenuOpen` 或 `requestingTimePause` 引入兼容层，也不要增加“由 Control 清 false”的所有权猜测。若未来要扩展菜单暂停，必须在客户端/上游提供独立、可撤销且不会被全局暂停反向维持的权威信号后另做协议。
