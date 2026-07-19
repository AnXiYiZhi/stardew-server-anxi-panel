# SAVE-NAME-ENCODING-DELETE-1 联调契约（2026-07-20，completed）

- `POST /api/instances/:id/saves/upload-preview` 对 GBK/GB18030 ZIP 路径名先转为 UTF-8，再返回 `saveName` 并持久化 pending token；前端不得自行重新解码名称。结构不完整、路径重复/冲突或名称不安全仍返回 `400 invalid_zip`。
- `GET /api/instances/:id/saves` 的每项可包含 `nameWarning`。有该字段时前端必须禁止 select/select-and-start，但可按响应中的 `name` 调用备份、导出和删除；`encoding_error_<hash>` 是稳定的历史目录公开身份，不是磁盘原始路径。
- `POST .../saves/select` 与 `select-and-start` 对历史非 UTF-8 目录返回 `409 save_name_encoding_invalid`。`DELETE .../saves/:name` 成功返回 `200 {ok:true,backupCreated:true}`；目标已不存在返回 `404 save_not_found`；事务未完成才返回 `500 save_delete_failed`。
- 删除响应后前端始终重新 GET saves/backups，以后端列表为权威；不得仅凭 DELETE 的异常保留旧卡片。删除活动存档成功后实例进入 `save_required`。

# FARMHAND-DELETE-1 联调契约（2026-07-18，completed）

`POST /api/instances/:id/players/delete-farmhand` 请求：

```json
{"uniqueMultiplayerId":"-3583227484444031316","expectedName":"1","expectedSaveId":"1111_442923526","acknowledged":true}
```

- 受理返回 `202 {"jobId":"..."}`，最终状态由 `stardew_farmhand_delete` job 提供。未确认返回 `400 confirmation_required`；停服、存档切换、其它活动任务、目标在线或世界未就绪返回 `409`；当前 Junimo 不支持人物删除时返回 `501`。
- `GET /players` 新增 `saveCharacterPresent/canDeleteCharacter/deleteCharacterBlockReason`。前端必须以 capability 为准，不能仅根据 `status=offline` 推断可删；目标上线时后端二次校验仍是最终权威。
- 其他真人玩家在线是允许状态：确认框告知小屋同步风险；job 在删除前后提交游戏内 broadcast。被删除人物本人在线永远阻断。删除成功前必须完成前保存、整档备份、Junimo 删除、运行态复核、后保存与磁盘复核。
- Docker Desktop 真实链路验证了 API、job、Control、Junimo `.125`、保存、备份、人物/小屋变化、重启持久性、重复删除和整档恢复；“其他两名真人在线允许、目标上线拒绝”的分支另由纯逻辑测试固定，广播文件协议在真实容器取得 succeeded。

# SAVE-IMPORT-E2E-RELEASE-1 联调门禁（2026-07-16，未通过）

- 安全环境盘点只确认一个已停止的 direct-Junimo `save-import-spike` 项目和两个 SHA256 固定的 `.tgz`。没有八类原始 ZIP/逐份 hash、隔离 Panel 数据库与完整 UI→job 运行记录，也没有人工游戏客户端验证条件；本轮未启动任何实例、未发送 `saves import`。
- 自动化已确认：上传接口没有旧 Go 覆盖回退；缺 hostHandling 被拒绝；平台 ID 不写 journal/API/审计；Phase A 不接受日志或单指针；preimport 可由测试恢复且不被自动清理；journal 可在各阶段分类；finalize 后 save-now 只认相同 commandId 的 GameLoop.Saved；后端 test/vet/build 与前端 test/typecheck/build 均通过。
- 尚未真机确认：preimport 实际恢复、Panel 各退出点恢复、pending/count 异常、diagnostics/transition 故障、平台冲突、无活动世界、真实二次重启、角色/住宅/家庭/Mod 语义，以及桌面/手机贯穿完整事务。因此 13 项发布门禁没有全部通过。
- 上游仍无 import commandId；正式联调判定继续使用磁盘事务痕迹、pending、SaveNameToLoad、RuntimeSaveID、ProcessIdentity、finalizeCount、Control GameLoop.Saved 和 dayTransitionComplete 的复合证据。任何单项或日志文本均不能宣布成功。
- `SAVE-IMPORT-JUNIMO-1` 保持未完成，原始 blocked 历史继续保留。只有补齐上述真实 E2E 证据后才能改为 completed。

# FE-SAVE-IMPORT-HOST-1 联调完成（2026-07-16）

- 桌面和手机均按 `SAVE-IMPORT-WEB-API-1` 提交 `{token, hostHandling}`；swap 为 `{mode:"swap_to_player",platformId:string}`，takeover 为 `{mode:"virtual_host_takeover",acknowledged:true}`。前端没有缺省模式，也没有旧请求体回退。
- 成功响应按 `202 {jobId, operationId, saveName}` 消费。页面通过既有 jobs/SSE 按 `stardew_import_save_and_start` 恢复阶段，因此刷新或关闭预览弹窗不会取消后端事务，也不会重复创建 job。
- 前端只消费结构化错误码：`host_decision_required`、`platform_id_invalid`、`save_exists`、`junimo_import_unsupported`、`save_import_busy`、`import_command_failed`、`import_result_unconfirmed`、`import_recovery_required`、`import_activation_timeout`、`save_in_progress`。其中 unconfirmed 不是成功，recovery 禁止重复点击。
- 两端使用同一校验函数；平台 ID 只做 trim/十进制字符串检查且不转 number，后端校验仍是权威。专项测试、类型检查、生产构建和双端浏览器 QA 均通过。
- 下方 blocked 段落保留为 WEB API 正式契约完成前的历史记录。

# SAVE-IMPORT-WEB-API-1 联调契约（2026-07-16，completed）

上传预览响应保持不变。提交接口只接受以下显式主机处理决策；旧的顶层 `hostHandling/platformId/operationId` 形状不再进入导入流程，缺少新对象时返回 `400 host_decision_required`：

```json
{"token":"...","hostHandling":{"mode":"swap_to_player","platformId":"7656119..."}}
```

```json
{"token":"...","hostHandling":{"mode":"virtual_host_takeover","acknowledged":true}}
```

- `platformId` 必须是 JSON string；trim 后为非零、仅 ASCII 十进制数字且在 uint64 范围内，否则 `400 platform_id_invalid`。前端不得把它转成 JavaScript number。
- 受理固定返回 `202 {"jobId":"...","operationId":"...","saveName":"..."}`。operationId 由后端创建；相同 token/相同决策重试返回原三元组，不产生第二个 job。token 转为事务所有后跨 Panel 重启仍可发现。
- 预览阶段的 cancel 仍可删除 available token；一旦 reserve/ownership 已进入事务，token cancel 返回 `409 save_in_progress` 且不删除事务材料。已有活动/未完成导入返回 `409 save_import_busy`；实例已经运行或启动中返回 `409 save_in_progress`；同名存档返回 `409 save_exists`。
- 后续 job 错误使用 `junimo_import_unsupported / import_command_failed / import_result_unconfirmed / import_recovery_required / import_activation_timeout`。客户端不得根据日志文本推断成功，也不得对 submitted/unknown 结果自动重试。
- 审计只出现 `mode/saveName/operationId/jobId`，不会出现 platformId 或 pending.UserID。本任务未修改前端页面；当前旧客户端会得到明确错误，后续前端接入必须展示两个 mode 的不可逆含义并收集显式确认。
- 下方 `SAVE-IMPORT-JUNIMO-1 blocked` 继续作为历史调查记录保留；正式实现已经改用 Panel 黑盒复合证据，缺少上游 commandId 不是当前阻塞原因。

# SAVE-IMPORT-DURABLE-SAVE-1 联调契约（2026-07-16，completed）

- swap finalizer confirmed 后，后台 job 会通过现有 Panel Control 文件协议提交一次 save-now，并只轮询同一 commandId。只有 Control 因 `GameLoop.Saved` 写出的 succeeded 才表示写盘完成；`dayTransitionComplete=true`、文件 hash/mtime 或日志文本均不能单独完成事务。
- Saved 后还必须获得 post-Saved status version 之后的新 `dayTransitionComplete=true`，并通过稳定主文件、严格 XML、hash/mtime 变化检查，才依次写 `save_verified` 和 `completed`。客户端只有看到 completed 才可把导入事务视为持久完成。
- save-now failed 为 `import_recovery_required`；unknown/expired、transition timeout/字段缺失为 `import_result_unconfirmed`。这些状态都禁止自动重试 import/save-now，且后台不会停止或重启服务器。Saved 已发生但世界未稳定时 journal 会保留 warning，供恢复继续观察。
- as-is 不提交 save-now，只复核目标世界稳定后 completed。自动游戏日回档仍由原 `save-events` 消费链处理；preimport 保留不变。

# SAVE-IMPORT-ACTIVATION-1 联调契约（2026-07-16，completed）

- 上传提交 HTTP 形状不变。Phase A 确认后同一后台 job 会继续等待目标 RuntimeSaveID 和 Layer B 证据；`SaveNameToLoad`、reload 日志、pending 清空或 finalizeCount 单点变化都不能让客户端展示成功。
- swap 的 `save_verified` 同时要求目标 runtime saveId、pending 清空、当前 ProcessIdentity 代际 count +1、diagnostics 完整且 `masterName=Server`、世界 online/day-transition complete。as-is 不要求 count，只要求目标运行时存档、无 pending和稳定世界。
- reload 未完成最多受控重启一次；重启前应用目标 Mod profile，绝不重发 import。检测到玩家时返回 `save_import_players_connected` 且不踢人/重启；diagnostics 缺失或 failedFields 返回 `import_result_unconfirmed`，partial finalizer 返回 `import_recovery_required`，客户端不得提供“重试导入”快捷动作。
- activation 成功停在 journal `finalize_confirmed`；DURABLE-SAVE-1 随后完成 save-now/稳定磁盘门禁并写 completed。维护运行时仍不会因容器在线而提前发布邀请码或普通 `ready`。

# SAVE-IMPORT-PHASE-A-1 联调契约（2026-07-16，completed）

- 上传提交 HTTP 形状不变，后台 job 现在会从 `runtime_ready` 继续正式写一次 Junimo `.125` FIFO import。`import_submitted` 仅表示 FIFO 写入已完成；`import_confirmed` 仅表示 Layer A 黑盒复合证据成立，二者都不是整个导入完成，前端不得展示“导入成功/可加入”。
- swap 的 `import_confirmed` 必须同时具备主文件 hash 变化、目标 pending、非零 OwnerUid、operation-salted platform fingerprint match 和目标 SaveNameToLoad；as-is 必须 hash 不变、无 pending、指针从非目标切换到目标。日志成功文本和单一指针变化均不会确认。
- 超时/取消会先停止维护 runtime 并确认 server 退出，再分类最终磁盘证据；不会自动重试 FIFO。`import_command_failed` 的 no-effect 事务可安全 cleanup，`import_recovery_required`/`import_result_unconfirmed` 必须保留 journal、preimport 与证据等待人工或后续阶段。
- 半转换（hash 变化且无 pending）会在 server 停止后从 operation preimport 恢复上传目标并核验原 hash；该恢复不代表 Phase A 成功，仍返回 recovery required。平台 ID 不出现在 API、journal、job 错误或持久日志详情。

# SAVE-IMPORT-MAINTENANCE-RUNTIME-1 联调契约（2026-07-16，completed）

- 上传提交仍返回 `202 + jobId + operationId`。job 在 staging/preimport 后启动维护 runtime；实例对 UI 继续呈现 `stopped`，phase 为 `save_import_maintenance`，不会发布 `ready` 或邀请码。客户端不得把 server 容器存在、job 成功或 `runtime_ready` 单独解释为导入成功。
- `runtime_ready` 仅表示 `.125` 容器、FIFO、可读日志、health/status API、`saves` 列表命令和复合 baseline 均已就绪。该段是 MAINTENANCE-RUNTIME-1 的历史边界；PHASE-A-1 已在其后发送正式 import，但依然没有 Panel 指针预写、新游戏或 XML 修改。
- 维护 phase 的邀请码读取被拒绝；若 `/status.playerCount > 0`，job 以 `save_import_players_connected` 明确失败且不会踢人。`runtime_ready` 前失败/取消恢复原停止状态并保留 staged/preimport。
- journal 新增 `maintenanceStarted`、`serverOutputLogOffset`、`runtimeBaseline`；baseline 中 pending UserID 仍为 `json:"-"`，不得通过 API、日志或 journal 外泄。

# SAVE-IMPORT-STAGING-1 联调契约（2026-07-16，completed）

- `upload-preview` 响应不变。`upload-commit-and-start` reserve token 后，driver 先建立 journal，再同步把 payload 转入 operation/source；只有 source 所有权落盘且 job 创建成功才返回既有 `202 {jobId,operationId,saveName}`。因此响应后不再依赖 handler 内存中的 tempDir。
- token 新增内部 `owned` 状态：同 operation 可重新发现 source，其他 operation 拒绝占用。cancel 对 available token 仍删除 pending upload；对 owned token 调用提交前 transaction cleanup，删除 source 和本 operation 创建且未变化的 staged target，保留 preimport。submitted 后返回 recovery 冲突，不自动删除。
- `202` 仍不代表导入成功。该段记录的是 STAGING-1 完成时边界；后续 MAINTENANCE-RUNTIME-1 与 PHASE-A-1 已扩展到维护 runtime 和正式 Phase A 提交，但前端仍不得展示整个导入成功。
- journal 新增 `sourceOwned/stagedSaveCreated/stagedSaveFingerprint/preimportBackupSha256`；`staged` 现在严格表示 `Saves/<saveName>` 已完整原子可见，`backup_created` 严格表示上传目标的 preimport ZIP 及 SHA-256 已落盘。
- `preimport` 是长期恢复材料，不参与自动游戏日清理。用户若取消提交，目标目录被安全清理但 preimport 仍可从现有备份列表恢复；删除该 ZIP 只能走现有显式备份删除操作。

# SAVE-IMPORT-EVIDENCE-1 联调边界（2026-07-16，completed）

- 本任务只增加 driver 内部只读证据能力，没有新增或开放 Panel HTTP API，前端仍不接上传、正式提交或成功提示。
- 后续黑盒回执必须组合目标主存档/`SaveGameInfo` hash、精确 active pointer、pending intent、operation-salted platform fingerprint、进程代际、Control runtime saveId、diagnostics finalize count/masterName 及 day-transition 状态。`/diagnostics/state`、pointer、FIFO 写入或任一单源均不能独立表示成功。
- unknown 是正式状态语义：文件/API/运行态字段不可读、字段缺失、`failedFields` 命中、采集期间文件变化、进程代际变化或来源矛盾时，不得填默认值或推断成功/失败。pending JSON 损坏必须显式报错，不能降格成 `Exists=false`。
- `JunimoSaveImportIntent.UserID` 不属于未来 API DTO：只允许进程内计算 `sha256(operationID + "\x00" + platformID)`，对外只能出现 `match/mismatch/unknown`。支持包、journal、API 和日志不得包含原始值。
- 保留下面 `SAVE-IMPORT-JUNIMO-1` 的 blocked 历史，作为“不能等待上游单一 commandId 回执”的调查记录；后续实施改为 Panel 黑盒复合证据，不再要求修改 Junimo 上游，也不得使用 `/test/import_save`。

# FE-SAVE-IMPORT-HOST-1 联调状态（2026-07-16，blocked）

- 前端任务未接入：后端阶段 2未稳定，且当前缺失 hostHandling 会被默认成 `server_owns_original`，不符合强制用户同意要求。
- 后端现有 `server_owns_original/swap_host_to` 与拟定 `virtual_host_takeover/swap_to_player` 必须先形成唯一公开 DTO；客户端不得自行猜测映射。
- 解锁时后端必须先证明：空值/未知值稳定拒绝、takeover 无默认、platformId 保持字符串、job 阶段和 unknown/recovery 可恢复。随后桌面与手机使用同一共享校验和请求体测试共同上线。

# SAVE-IMPORT-JUNIMO-1 联调状态（2026-07-16，blocked）

- `.125` 当前没有正式 import commandId/JSON 终态，无法满足本任务要求的 `submitted/succeeded/failed/unknown` 等待契约；`/test/import_save` 不得用于生产。
- 前端和其他调用方不得把 `202`、FIFO 写入成功、pointer 变化、Monitor 日志或观察超时解释为导入成功/失败。现阶段继续不接入上传正式提交 UI。
- 解锁后仍沿用 SAVE-IMPORT-TXN-1 的 operationId、journal、token reserve、互斥和恢复边界；上游结果必须能与同一 operation/command ID 精确关联。

# SAVE-IMPORT-TXN-1 导入 job/API 契约（2026-07-16，completed）

- `upload-preview` 成功后 token 对应持久 staged 数据；Panel 重启不会自然丢失。
- `upload-commit-and-start` 接受 `token`、可选 `operationId`、`hostHandling=server_owns_original|swap_host_to` 和 swap 使用的十进制 `platformId`。受理返回 `202 {jobId, operationId, saveName}`，job type 固定为 `stardew_import_save_and_start`。平台 ID 不进入响应、journal、审计或普通日志。
- `202` 只表示事务受理。未来仅 journal `completed` 可判成功；此前阶段全部非终态。当前 runner 会执行真实 FIFO 门禁但不发送 import，前端暂不接入。
- 稳定 409：`save_exists`、`save_import_busy`、`token_reserved`、`junimo_import_unsupported`。未知/恢复：`import_result_unconfirmed`、`import_recovery_required`、`import_activation_timeout`。
- token 先 reserve，不删除；journal 后迁入 operation/source 并标为 owned；同步所有权失败可 release，未来 `completed` 才 consume。cancel 可删除 available，或对 owned 执行保留 preimport 的安全 pre-submit cleanup；从 `import_submitted` 起禁止自动清理、重放、覆盖或切 active save。

# SAVE-IMPORT-SPIKE-1 拟采用导入契约（2026-07-16，blocked）

- 正式 UI/API 暂不开放。JunimoServer `1.5.0-preview.125` 的 FIFO `saves import` 没有 commandId 或机器终态；成功结构化日志也无法关联请求，错误、reload 拒绝/跳过/异步失败只有展示日志。前端不得解析这些文案，也不得在观察超时后自动重发。
- 未来 queued 至少要求机器回执或同时证明 `SaveNameToLoad==target`；swap 还要求 pending-finalize 与本次 request identity 一致。loaded/finalized 至少要求 pointer、Control `status.json.state=save-loaded/saveId==target`、`/status.isOnline=true`、pending=null，并由机器字段证明虚拟 `Server` master 和原 owner customized+bound。任一来源矛盾均为 unknown。
- 无活动世界时不依赖 `--reload`：命令只做 Layer A/pointer/pending，随后必须受控重启并按上述多源事实验收。不提供平台 ID 的 as-is 会让虚拟主机直接接管原 `<player>` 角色；不会把原 owner 自动变成可选 farmhand。
- 提交前可稳定拒绝目录/主文件缺失、当前活动存档、XML 不可解析、非 invariant ulong ID 和已知 ID collision；FIFO 已写之后，缺少终态回执时不得把 pointer 未变化或某条日志当成失败。
- 最小上游依赖：正式 `POST /saves/import` 或带 caller commandId 的 FIFO 命令，schema-versioned JSON 覆盖 queued/running/succeeded/failed/unknown 以及 reload 的 not_requested/started/succeeded/refused/skipped/failed/timed_out，并返回 `loadedSaveId/finalized/errorCode/timestamps`；所有退出路径产生恰好一个可关联终态，平台 ID 脱敏。
- `dayTransitionComplete` 可用于下一次跨日边沿：先以 baseline version 等 `false`，再从 false version 等 `true`；`/wait/status` 的确支持该字段过滤。禁止初始 true 时直接等 true，因为普通下一快照就会误命中。需要精确保存回执时仍使用 Control `Saved` machine event/command result，而不把这个电平当成持久“保存成功”标志。

# NEWGAME-TXN-1 custom-new-game 联调契约（2026-07-15）

- `POST /api/instances/:id/saves/custom-new-game` 的 URL、管理员权限和 `202 {jobId}` 保持不变；body 现在限制 1 MiB、只允许一个 JSON 对象并拒绝未知字段。官方八种 FarmType 的前端调用不变，模组 ID 返回 `409 {error:{code:"modded_farm_creation_disabled"}}`。
- handler 不再提前落盘；收到 202 只表示 lifecycle transaction job 已创建。前端必须继续按 job 终态展示，不得把容器 running 或 marker 消失解释为新存档成功。
- job 成功意味着后端已发现唯一新目录、同名主文件稳定、XML 可解析且实际 whichFarm 与请求一致。`unknown/ambiguous` 会作为失败 job 呈现，错误字符串含稳定码 `new_game_outcome_unknown` 或 `new_game_ambiguous`，客户端不得自动重发 `/custom-new-game`。
- 失败时后端停止本次启动的服务并恢复旧配置、gameloader 和 Mod 状态；验证失败的新目录进入隔离区而非删除。`new_game_rollback_failed` 表示恢复不完整，前端应明确要求管理员查看任务日志，不应提供自动重试。
- 本阶段没有新增前端字段或页面，也没有改变官方农场创建表单；模组目录卡仍只读不可提交。

# FARM-NEWGAME-MOD-PREPARE-1 依赖与准备联调契约（2026-07-15）

- `GET /api/instances/:id/saves/farm-types` 的 modded 项新增 `modSelection`：provider、required/optional、当前 enabled、disabled required、missing required、conflicting providers、components、readiness 和 dependenciesReady。key 是受控逻辑标识，不是宿主路径。
- `POST /api/instances/:id/saves/farm-types/prepare` 仅管理员，严格 body `{farmTypeId:string}`。服务器运行/启动中为 `409 server_running`，活动实例任务为 `409 instance_busy`，未知 ID 为 404，冲突/缺 required 为 409。普通移动失败且已恢复为 `500 farm_prepare_failed`；若自动恢复本身未完整成功则为 `500 farm_prepare_rollback_failed`，客户端必须提示不要启动。客户端不得提交 provider、folder、path 或 Mod 列表。
- 前端必须在 POST 前用响应 components 列出所有 `enabled=false` 项并取得用户确认；成功响应 `changedModKeys` 是实际启用结果。API 不启动服务器、不创建 save、不生成正式 profile。
- `dependenciesReady=true` 仍不使卡片 selectable；`POST .../custom-new-game` 继续只接受官方 FarmType。下一阶段需增加 SMAPI 运行时加载验证和创建事务。

# FARM-CATALOG-READONLY-1 农场目录只读联调契约（2026-07-15）

- 管理员新建游戏页调用 `GET /api/instances/:id/saves/farm-types`。响应的 `farmTypes` 始终含 8 个 `kind=builtin`；扫描失败仍为 200，并通过 `catalogWarnings` 给非阻断提示。modded 条目的 `dependenciesReady` 当前为 `null`、`selectable=false`、`requiresRuntimeValidation=true`。
- `iconUrl` 只能指向 `GET /api/instances/:id/saves/farm-types/:farmId/icon`；前端不得把 providerFolder 或 Farm ID 自行转换成本地路径。图标 404 时使用固定占位图并保留卡片。
- 前端官方选择列表保持静态，模组目录只展示；后端 `POST /api/instances/:id/saves/custom-new-game` 的官方 allowlist 同样保持不变。因此即使绕过 UI，模组 FarmType 仍无法创建。
- 两个新 GET 与新建游戏现有可见性一致，均仅管理员；普通用户不会收到 providerFolder。未来开放前必须新增依赖/运行时验证及明确的 selectable 契约。

# JUNIMO-CONFIG-REPAIR-1 联调契约（2026-07-15）

- `GET /api/instances/:id/junimo-update` 新增 `repairable:boolean`、可选 `repairCode/repairReason`。旧客户端可忽略；只有 `repairable=true` 才允许前端提供自动修复入口。
- `POST /api/instances/:id/junimo-update/repair-config` 仅管理员，请求体为空或严格 `{}`；目标镜像、tag、候选、命令或任意额外字段返回 `400 config_repair_body_not_allowed`。匿名/普通用户仍为 401/403。
- 成功返回 HTTP 200，包含修复后的完整 `JunimoUpdateInfo` 以及 `repaired=true/backupId`。预期旧 `.121` 候选修复后直接变为 `supported=true,status=update_available,available=true,repairable=false`，前端随后才能 POST 新 dry-run。
- 自动修复仅规范化 `SERVER_IMAGE_CANDIDATES` 和 `STEAM_SERVICE_IMAGE_CANDIDATES`；不接受客户端目标，不改主镜像、当前版本、凭据、Compose 或容器。完整原 `.env` 只保存在实例私有备份中，不进入 API、日志或支持包。
- `runtime_update_busy`、`manual_recovery_required`、自定义/未知镜像和无法消除歧义的版本字段返回 409；前端停止链路并显示错误，不得绕过修复直接 apply。
- 用户一次点击“修复并升级”的顺序固定为：修复成功并复检 → 创建本次新 dry-run → 轮询同一 dryRunId 成功 → 单次 apply。历史成功预检不得复用。

# MODBUNDLE-1 合包上传联调契约（2026-07-15）

- `POST /api/instances/:id/mods/upload` 仍使用重复 multipart `mod` 字段；每个 ZIP 可以是单 Mod、Nexus 外壳包，或用户将整个 Mods 文件夹重新压缩后得到的多层合包。
- 成功响应仍是 `ModsListResult`，并额外携带 `upload: { archiveCount, discoveredCount, importedCount, enabledCount, activeSaveName? }`。前端应显示该摘要；成功时发现数和导入数必须相等。
- 合包中任一已发现 Mod 解析失败时整批失败，不能返回部分成功。有激活存档时，导入后的 profile 启用写入也是成功契约的一部分；失败返回 `500 mod_enable_failed` 并回滚本批导入。
- `ModInfo` 新增可选 `packageKey/packageName`。`packageKey` 表示物理 ZIP 子包归属，不表示 Nexus 身份；前端删除预览与排序应优先使用它，缺失时才回退 `originNexusModId/nexusModId`。聚合 ZIP 内不同第一层子包不得因为其中一个包有 Nexus ID 而互相继承来源。
- 内容包名称仍以 manifest `name` 为事实字段；前端结合 `isContentPack/contentPackFor/folderName` 添加 `[CP]`/`[FTM]` 展示前缀，不要求后端篡改 manifest 名称。
- 联调回归：两个独立 ZIP 应返回 `2/2/2`；`Mods1.zip` 实包应返回 `1 个 ZIP / 38 个发现 / 38 个安装 / 38 个启用`。

# MAINTENANCE-SINGLE-CARD-1 联调补充（2026-07-14）

- 用户从版本维护卡片点击“立即升级”后，前端内部串联当前目标 dry-run 与 apply，所有用户态进度继续在同一卡片消费既有 GET 状态接口。
- 技术详情不再作为用户流程跳转目标；接口、状态机、确认请求与安全门禁均未变化。

# JUNIMO-ROLLBACK-STATE-1 联调补充（2026-07-14）

- 回滚期间 Compose 仍消费临时 digest pin；回滚流程退出前必须恢复 `original.env` 中的可信 tag 引用，确保 `GET /junimo-update` 能继续返回正确版本状态。
- `rollback_failed` 即使容器已恢复健康也仍是安全锁；前端必须优先呈现人工处理状态，不能因同时出现 `invalid_config` 或接口读取失败而显示“无需操作”。
- 本次没有变更接口 schema；新增内部错误码 `rollback_restore_final_env_failed` 用于最终 `.env` 恢复失败。

# PANEL-0.2.2 / JUNIMO-125 联调契约（2026-07-14）

- `0.2.2` 推荐版本对为 server `.125` + auth-cn `1.5.0-anxi.2`。当前 `.121` + auth-cn 配置返回 `supported=true,status=update_available,available=true`，不会返回 unsupported，也不会阻止其他实例 API。
- 新安装写入 `.125`；旧实例只有管理员显式 POST dry-run、确认 apply 后才会改变 `.env` 和容器。仅查看提示、升级 Panel 或普通用户登录不得自动拉取、停服或修改实例。
- `.125` 继续使用现有 HTTP 路由、server/steam-auth Compose 服务、设置契约、邀请码文件和 Control 文件协议；`/status` 新增字段由旧客户端忽略。
- 验收必须覆盖 `.121` 继续运行、`.121`→`.125` 成功、失败回滚到 `.121`、23 个 init 兼容挂载、auth ticket、邀请码、Control 状态/玩家文件及 VNC/字体。

# JUNIMO-STACK-UPDATE-1 阶段二 dry-run 联调契约（2026-07-13）

- `POST /api/instances/:id/junimo-update/dry-run` 仅管理员，空 body 或严格 `{}`，成功 `202`；任何目标/命令字段返回 `400 dry_run_body_not_allowed`。不支持/忙碌返回 409（如 `not_installed`、`unsupported/custom_images`、`invalid_config/*`、`runtime_update_busy`）。
- `GET /api/instances/:id/junimo-update/dry-run` 仅管理员，返回最近持久状态；首次为 `idle`。字段为 `dryRunId/jobId/phase/progress/current/target/selected/checks/warnings/logs/serverRunning/errorCode/error/startedAt/updatedAt/finishedAt`。
- `selected.server` 与 `selected.steamAuth` 仅在可信候选取得 digest 后出现，始终属于同一 target 版本对。状态文件是详细事实来源，jobs 是生命周期/互斥来源；响应不回显 Docker stderr、Compose 展开环境或 registry/Steam 凭据。

# JUNIMO-STACK-UPDATE-1 阶段一联调契约（2026-07-13）

- `GET /api/instances/:id/junimo-update` 仅管理员、仅 GET。普通用户返回 403；未登录返回 401；非 GET 返回 405。请求不接受镜像、tag、digest、registry 或 stackVersion。
- 成功响应包含：`available`、`supported`、`status`、`code`、`reason`、`current.server|steamAuth.{image,tag}`、`recommended.{stackVersion,minimumPanelVersion,server,steamAuth,releaseNotes,tested}`、顶层 `releaseNotes`、`serverRunning`、`steamAuthLoggedIn`。
- `status` 仅为 `up_to_date | update_available | not_installed | custom_images | invalid_config`。只有可信仓库且当前 server/auth tag 对不等于推荐对时 `available=true`；自定义镜像固定 `supported=false,status=custom_images,code=unsupported/custom_images`。
- 普通用户通过 `/state.runtimeDiagnostic` 读取 `serverVersion/expectedServerVersion`、`steamAuthVersion/expectedSteamAuthVersion`、`junimoStackVersion`、`junimoUpdateStatus/code/reason/supported`、`junimoVersionMatches`，不会收到镜像仓库引用。
- 接口和页面均不得返回 Steam 密码、refresh token 或完整 `.env`。本阶段没有 dry-run/apply、镜像拉取、`.env` 修改、停服或容器重建，也与 `/api/system/update` 无关。

# PANEL-UPDATE-RELEASE-1 联调验收（2026-07-13）

- 隔离项目以本地构建的 `0.1.13`、`0.1.14` 和故意 unhealthy 的 `0.1.14` 镜像完成 Web 端到端验证。成功链路覆盖自动发现、双入口提示、管理员点击、备份、独立 helper 替换、浏览器重连、三项健康验收与 `succeeded` 持久终态。
- 回滚链路覆盖数据库故意写入后 unhealthy、120 秒健康超时、恢复旧 Compose/`.env`/镜像/数据库、旧 panel 重新 healthy，以及前端 `failed_rolled_back` 结果。`PRAGMA integrity_check` 为 `ok`，故意写入的用户名已恢复。
- 两个项目的 Stardew、server、steam-auth 哨兵容器 ID 和 `StartedAt` 全程不变；升级命令仍固定 `--no-deps panel`。
- 无 Docker Socket 返回 `docker_unavailable`；有 Socket 但无 Compose labels 的自定义 `docker run` 返回 `compose_labels_missing`。权限、并发拒绝、非白名单镜像、draft/prerelease、网络失败保留上次成功状态由接口与单元/contract tests 覆盖。
- 发布前隔离验收没有创建临时 tag，因此镜像预置在本地，并用测试专用 Docker CLI wrapper 跳过远端 pull；helper 生命周期、Compose 重建、健康检查和回滚均为真实 Docker。v0.2.0 Tag 推送后由发布 workflow 构建正式镜像，正式 registry `--pull always` 闭环留待镜像发布完成后复验。

# FE-PANEL-UPDATE-1 升级期间联调行为

- 前端只由全局 Provider 请求 `GET /api/system/update`、管理员 dry-run/apply 接口；顶栏、总览和弹窗是同一状态的消费者，不得各自轮询。
- `POST /api/system/update/apply` 仍必须无 body。若 POST 在响应前发生网络中断，前端进入“结果待确认”状态并查询持久任务，不能自动重复 POST；在线连续 404 后才提示用户重新检查。
- apply 活动阶段请求失败视为预期面板重启。前端以退避策略请求公开 `/health` 与 `/api/version`，并在 HTTP 恢复后读取管理员 apply 状态；目标版本成功需 `/api/version` 等于 `toVersion`，最终 UI 以 `succeeded|failed_rolled_back|rollback_failed` 为准。
- `succeeded` 自动恢复原页面并显示新版本结果；`failed_rolled_back` 显示“升级失败，已恢复”；`rollback_failed` 明确提示联系面板管理员，不向普通用户展示 shell 命令或要求 SSH。游戏实例 API 的短暂失败不会覆盖专用升级全屏状态。

# PANEL-UPDATE-APPLY-1 联调约定

- `POST /api/system/update/apply`：仅管理员；请求必须无 body。后端从共享更新检查结果读取 `currentVersion/latestVersion`，只有已确认 `updateAvailable=true` 才创建任务，成功返回 HTTP 202 和持久化 apply 状态。前端不得提交任意版本或镜像。
- `GET /api/system/update/apply`：仅管理员；无状态时 404。响应字段包含 `updateId/phase/progress/fromVersion/toVersion/errorCode/error/result/logs/startedAt/updatedAt/finishedAt`，以及管理员诊断用的原/目标镜像与 digest。
- 活动阶段为 `checking|backing_up|pulling|recreating|waiting_health|rolling_back`；终态为 `succeeded|failed_rolled_back|rollback_failed`。HTTP 请求在 helper detached 启动后尽快返回；panel 重启期间前端可在恢复连接后继续读取同一状态文件。
- 409 表示无已确认更新、版本不合法、部署 unsupported 或已有任务；SQLite 备份失败也在修改部署前终止。升级只允许 self Compose project 的 `panel` service，禁止操作 Stardew 实例 Compose。

# PANEL-UPDATER-DRYRUN-1 联调约定

- `GET /api/system/update/capability`：仅管理员。返回 `supported/reason/code/composeProject/composeFile/installDir/currentContainer/currentImage/dataMount/dockerAvailable/composeAvailable`；这是唯一可返回完整部署路径的前端接口。
- `POST /api/system/update/dry-run`：仅管理员，请求 `{"targetVersion":"0.1.15"}`，成功启动返回 202 和持久状态。targetVersion 必须是无 prerelease/build metadata 的精确语义版本；客户端不能提交镜像仓库。
- `GET /api/system/update/dry-run`：仅管理员，读取 `<PANEL_DATA_DIR>/updater/status.json`；状态为 `starting|running|succeeded|failed|unsupported`，包含 capability、目标版本/镜像、脱敏日志和时间。
- unsupported 是正常能力结果，不等于 API 故障。前端只显示 reason/code，不自行推导宿主机路径或补猜部署方式。
- 本阶段没有 apply/upgrade API。dry-run 允许 image inspect/pull 和 Compose config，禁止 stop/rm/up/down/restart 当前面板。

# PANEL-UPDATE-CHECK-1 联调约定

- `GET /api/system/update`：任意已登录用户可读取。响应固定包含 `currentVersion`、`latestVersion`、`updateAvailable`、`releaseUrl`、`publishedAt`、`checkedAt`、`checkStatus`、`checkError`，并附带 `currentCommit/currentBuildDate`。
- `POST /api/system/update/check`：仅管理员可触发即时检查，返回相同结构；普通用户调用返回 403。
- `checkStatus` 为 `pending|checking|ok|error|unavailable`。若存在成功缓存，后续网络失败时仍返回缓存的 latest/release/checkedAt，同时以 `error` 和 `checkError` 表明刷新失败；前端不得把此状态显示为“已是最新”。
- 前端只由 dashboard 数据层请求这两个接口，顶栏、总览与弹窗共享结果。当前没有 `apply`、`upgrade`、容器替换、自动重启或回滚契约。

# MOBILE-HOME-M2-1 联调约定

- 移动端总览页（`frontend/src/games/stardew/mobile/MobileHomePage.tsx`）只复用现有 API，无新增后端契约：生命周期用 `POST /api/instances/:id/start|stop|restart`；邀请码/局域网地址用现有 `GET /api/instances/:id/invite-code` 和前端本地 `window.location.hostname`；待认证玩家批准用现有 `GET /api/instances/:id/password-status` 和 `POST /api/instances/:id/players/approve-auth`。
- 后端无需为本轮改动做任何调整。

# NEXUS-ARCHIVE-RESUME-1 联调约定

- `mod_remote_install` / `mod_nexus_install` 的 ZIP 下载阶段可能在 job 日志中出现“继续断点下载”“服务器未接受断点续传，重新从头下载”“下载连接卡住，正在重试”等提示。前端无需新增接口，继续展示 `GET /api/jobs/:jobId/logs` 和 SSE `log` 事件即可。
- 远程/Nexus Mod install job 的整体超时为 **30 分钟**；单个 ZIP body 下载窗口为 **20 分钟**；如果 **120 秒没有收到新字节**，后端会判定当前连接卡死并重试。
- ZIP 下载进度仍以日志里的“已下载/总量/百分比”为准。断点续传时百分比会从 `.part` 已有字节继续推进；如果服务器忽略 Range，后端会从 0 重新下载，前端按新日志刷新即可。
- 403/410 这类 CDN 临时链接过期仍会作为任务失败暴露给用户查看日志；本次没有新增“重新获取 Nexus 链接”的 API 契约。

# MOD-REMOTE-IDEMPOTENT-1 / FE-MOD-BATCH-ERROR-FOCUS-1 联调约定

- `POST /api/instances/:id/mods/remote/install` 和 `POST /api/instances/:id/mods/nexus/install` 创建的后端安装 job 对“目标 Mod 已经安装”是幂等的：ZIP 内某个 `UniqueID` 已存在时跳过该目录；整个包都已安装时 job 应为 `succeeded`，日志显示已安装并跳过重复导入，不应返回 failed。
- 手动上传 `POST /api/instances/:id/mods/upload` 仍保持严格重复校验，重复 `UniqueID` 返回 `400 mod_exists`。
- Mod 下载类 job 的 `displayName` 为“Mod 名 · 任务类型”，例如 `Ridgeside Village · mod_remote_install`；`type` 字段仍是机器可读值，前端不要从 displayName 反推类型。
- Nexus 普通一键安装批量进度中，后端 job 失败时前端按钮应显示失败的具体 Mod 名；若该项有 `jobId`，点击按钮跳转 `/instances/stardew/jobs?jobId=<jobId>` 查看任务与日志。
- 批量进度应以最新 `GET /mods` 作为兜底：如果某项已经能通过 `nexusModId` 或 `originNexusModId` 匹配到本地安装结果，即使旧 job 曾因重复安装失败，也应视为完成，不应把整批任务标成失败。

# JUNIMO-APPNAME-CONTENV-FIX-1 联调契约

- 如果 server 容器日志出现 `APP_NAME: DockerApp: not found`、`DBUS_SESSION_BUS_ADDRESS: unix:path=/tmp/dbus.base: not found`、`DOCKER_IMAGE_PLATFORM: linux/amd64: not found` 或 `/etc/cont-groups.d/...: 72: not found` 这类 init 静态值执行错误，优先检查实例 compose 是否包含 `.local-container/cont-env/*`、`.local-container/cont-groups/*`、`.local-container/cont-users/*` 兼容挂载，以及实例目录是否存在对应脚本。
- 该修复由后端 Prepare/安装/启动自动完成，前端不需要新增接口或特殊状态；用户只需要更新面板镜像后重新运行启动/安装流程。
- 如果旧容器已经按旧 compose 创建，新增挂载后必须通过 `docker compose up -d` 重建 server 容器才会生效；后端 `doRestart()` 在检测到 compose 被迁移时会自动走 `ComposeUp`。

# NEXUS-NETWORK-DIAGNOSTICS-1 / PANEL-ACCESS-HOST-INVITE-1 联调契约

- Nexus 搜索 `GET /api/instances/:id/mods/nexus/search` 后端会用独立 20 秒上下文访问 Nexus GraphQL，避免浏览器请求取消导致上游搜索被提前打断。网络类失败返回 `502 nexus_network_failed`，前端展示“请确认面板服务器能访问 api.nexusmods.com”。
- 如果日志出现 `nexus_network_failed`，先看后端日志中的底层错误；若只是旧版 `nexus request failed`，需要结合耗时判断，4 秒以内失败更可能是浏览器/代理链路取消，不一定是 Nexus 不通。
- 邀请卡“局域网邀请”由前端 `window.location.hostname` 得出，不再消费 `/api/instances/:id/public-ip`。因此用户从什么 host 加 `:8090` 进入面板，就展示什么 host。
- `/api/instances/:id/public-ip` 仍保留为后端公网出口 IP 检测接口，但当前邀请卡不再依赖它。

# STEAM-AUTH-FLAG-1 联调契约

- `GET /api/instances/:id/state` 返回 `steamAuthLoggedIn`、`steamAuthReady` 与可选 `inviteCode`：
  - `steamAuthLoggedIn`：主 UI 授权标志，表示 `.env` 中 `STEAM_AUTH_COMPLETED=true`。该值在 steam-auth 登录成功日志出现后写 true；启动/手动刷新成功获取非空邀请码时也写 true；server 启动日志明确出现 `no logged-in accounts` 后写回 false。
  - `steamAuthReady`：当前运行态诊断字段，表示当前 `steam-auth:3001/steam/ready` 可用并返回 200。它不再直接控制邀请码卡的授权按钮。
  - `inviteCode`：后端后台邀请码探测写入 driver payload 后回传的最后一次有效邀请码。前端可直接展示它，避免每次切页都重新 exec 容器。
- 邀请码卡按钮只按 `steamAuthLoggedIn` 显示：false/空时显示【登录授权】；true 时显示刷新/获取状态。服务器运行中且需要授权时，按钮提示先停服。
- 启动/重启生命周期 job 不等待邀请码：server 拉起后即进入 running，后台最多探测 20 次邀请码。探测失败不影响生命周期任务成功、不停止服务器，用户仍可走 IP 直连；探测成功后写 `steamAuthLoggedIn=true` 并通过 `/state.inviteCode` 展示。
- 服务器运行中重新授权仍受 `POST /api/instances/:id/steam-auth/login` 的既有约束：必须先停服，否则返回 `409 server_running`。前端应提示“先停止服务器再重新授权”。
- 验证：安装时 fake/真实日志出现 `[SteamAuth:A0] Logged in as ...` 后，`/state` 应返回 `steamAuthLoggedIn=true`；启动或 `GET /api/instances/:id/invite-code` 成功返回 `SG...` 这类邀请码后也应返回 true；让服务器启动日志出现 `Steam-auth service has no logged-in accounts` 后，后续 `/state` 应返回 `steamAuthLoggedIn=false`，邀请码卡显示【登录授权】。
- 如果 server 日志只有 `Steam-auth service not ready` / `Steam auth service request failed`，不要直接当未授权；已有 `steamAuthLoggedIn=true` 时后端会自动刷新一次 `steam-auth` 服务。

# INSTALL-SMAPI-PREINSTALL-1 安装链路联调说明

- 安装顺序现在为：准备目录/镜像 -> Steam/SteamCMD 授权 -> 游戏文件下载/校验 -> Steam SDK 下载/校验 -> `smapi_installing` -> `game_installed`。
- `smapi_installing` 只在游戏文件和 Steam SDK 完成后进入；不要把它提前到游戏文件下载结束前。失败 phase 为 `smapi_install_failed`。
- 前端应把 `smapi_installing` 归入“下载游戏”大步骤的最后一个子状态；`smapi_install_failed` 允许用 `reuseCredentials=true` 重试，不要求用户重新输入 Steam 账号。
- 后端日志前缀为 `[smapi]`，前端可用该前缀做日志兜底，以免实例状态轮询慢时仍停在 SteamCMD 下载阶段。
- 联调验证：在干净实例安装完成 Steam SDK 后，应看到任务日志 `[smapi] 使用 JunimoServer 镜像 ... 预安装 SMAPI。`，随后进入安装完成；若 `/data/game/StardewModdingAPI` 已存在，应看到 skip 日志。

# FE-STEAM-GUARD-SUBMITTED-FEEDBACK-1 联调契约

- `POST /api/instances/:id/steam-guard/input` 返回成功只表示验证码/选项已经写入当前交互进程，不代表 Steam 已经完成验证。前端应在成功返回后立刻显示“已提交，等待 Steam/SteamCMD 响应”的本地等待态，而不是继续展示空输入框。
- 等待态应保留重新输入入口；如果后端后续仍停在 `steam_guard_required` 或 `steamcmd_guard_required`，管理员可以重新提交验证码。若 phase 进入下载、失败或完成，前端应自动退出等待态。

# STEAMCMD-EMAIL-GUARD-PROMPT-1 联调契约

- SteamCMD 邮箱验证码提示可能不是单行 `Enter Steam Guard code sent to ...`，也可能拆成多行：`This computer has not been authenticated for your account using Steam Guard.`、`Please check your email ... enter the Steam Guard`、`code from that message.`、`set_steam_guard_code`。
- 后端看到这些 `[steamcmd]` 输出时应进入 `driverPhase=steamcmd_guard_required`；前端如果先通过 job 日志看到这些文本，也应按 `steamcmd_guard_required` 渲染验证码输入框。
- 验证码提交接口不变：`POST /api/instances/:id/steam-guard/input`。该阶段文案仍应明确是 SteamCMD 兜底授权，不要显示普通 steam-auth 下载或 Docker 镜像拉取状态。

# INSTALL-ROUTING-SPLIT-1 联调契约（安装路由 + forceReauth）

- `POST /api/instances/:id/install` 请求体：`steamUsername/steamPassword/vncPassword/imageTag/reuseCredentials` 之外新增 `forceReauth`（布尔，选填）。三选一语义：
  - 全新安装 / 账密错误重输：发完整 `{ steamUsername, steamPassword, vncPassword, imageTag }`。
  - 复用重试（镜像拉取失败、超时、下载失败、SteamCMD 重试、已安装重装等）：发 `{ reuseCredentials: true, imageTag }`。
  - 更换账号 / 强制重新认证：发 `{ steamUsername, steamPassword, vncPassword, imageTag, forceReauth: true }`。
- 路由由后端按**实例已持久化 driverPhase/state** 决定，前端**不需要**自己区分该走 steam-auth 还是 SteamCMD：
  | 触发 phase/state | 后端走向 |
  | --- | --- |
  | `pull_failed` / 认证前超时(`install_timeout`,`steam_auth_connection_failed`) | 重新拉镜像 → steam-auth（自动账号密码，跳过方式选择） |
  | `credentials_required` | 前端弹表单重输，等同全新安装 |
  | `download_failed` / `post_auth_failed` / 已安装态 / `steamcmd_*` | 直达 SteamCMD（有缓存则仅用户名秒过，无缓存则完整登录一次） |
  | `forceReauth=true` | 清授权卷 + 重置标志 → 重新拉镜像 → 完整认证 |
- 关键期望：SteamCMD **首次授权超时**后重试，联调应看到 SteamCMD **重新出现 guard 选择/验证码/批准提示**（而非 `state=error, steamcmd_failed` 秒退）；SteamCMD 成功登录（`logged in ok`）后 `.env` 会写 `STEAMCMD_AUTH_COMPLETED=true`，此后重装/更新命中缓存路径。
- 更换账号联调：任务日志应出现 `更换账号：正在清除已保存的 Steam / SteamCMD 授权缓存...`，随后进入正常 `auth_method_required` / 扫码 / Steam Guard 流程；游戏文件（`game-data` 卷）保留。

# STEAMCMD-REPAIR-DIRECT-1 联调契约

- 前端在“重新安装 / 修复”、认证后下载失败重试、SteamCMD 重试这类复用凭据入口中，应继续调用 `POST /api/instances/:id/install`，请求体为 `{ "reuseCredentials": true, "imageTag": "..." }`，不要再提交 Steam 用户名、密码或 VNC 密码。
- 后端收到 `reuseCredentials=true` 后会读取实例 `.env` 中的已保存凭据，并显式让 `stardew_junimo` driver 直达 SteamCMD 下载/校验路径：跳过 `steam-auth`，也跳过重新选择 Steam 登录方式。
- SteamCMD 直达修复预期使用已保留的 SteamCMD 授权缓存登录；正常联调日志应出现 `[steamcmd] 跳过 steam-auth，优先使用已保留的 SteamCMD 登录授权直接下载/校验。`，不应出现新的 `[steam]` / `steam-auth` 认证流程。
- 若 SteamCMD 缓存不可用，后端返回 `state=error, driverPhase=steamcmd_failed`，前端应提示查看日志/重试，不应展示 Steam 账号密码输入框或 `credentials_required` 文案。
- 验证：已安装实例点击“重新安装 / 修复”后，表单不出现凭据输入；提交后任务日志直接进入 `[steamcmd]`，不出现 `auth_method_required`、二维码、Steam Guard 选择或 steam-auth 容器登录。

# PUBLIC-IP-LOOKUP-1 联调契约

- 新增 `GET /api/instances/:id/public-ip`：任意已登录用户可调用，返回面板后端所在服务器检测到的公网出口 IP，而不是浏览器客户端 IP。
- 响应结构为 `{ "ip": string, "checkedAt": string, "source"?: string, "cached": boolean }`。默认返回后端 `10min` 成功缓存；前端点击刷新时请求 `?refresh=1` 强制重新探测。
- 后端只接受合法公网地址；外部检测服务失败、返回内网/非法地址或超时时，接口返回 `502 public_ip_failed`。前端应显示“检测失败”，并允许用户手动刷新。
- 该接口不依赖 JunimoServer、Docker Compose 或服务器运行状态；它检测的是面板容器/宿主当前出口公网 IP，主要用于用户配置端口转发、直连排查和确认服务器对外地址。
- 验证：`cd backend; go test ./internal/web`，`cd frontend; npm.cmd run build`。

# DOCKER-POLL-PERF-1 Docker 状态与资源轮询契约

- 后端 `ComposePs` 有默认 `1.5s` 短 TTL 缓存，供状态页、支持包、诊断等短时间重复读取复用。接口响应结构不变，前端无需感知缓存命中。
- `ComposeStats --no-stream` 仍只由 `/api/instances/:id/metrics` 触发，不做高频全局轮询。前端资源指标采样应只在诊断页/资源页可见时运行，刷新间隔保持 `5-10s`，当前实现为 `8s`。
- 浏览器 tab 隐藏时前端必须停止资源指标 timer；恢复可见后可以立即采样一次。非资源页和后台 tab 不应持续请求 `/metrics`。
- `/api/health/diagnostics` 会执行 Docker daemon 与 Compose 可用性检查，可能调用 `DockerVersion` / `ComposeVersion`；该接口只用于 Diagnostics、Docker 状态页、安装前检查或用户手动刷新，不进入普通总览初始化和常驻轮询。
- 支持包中的 `ComposeLogs` 仍是一次性 tail 导出；后续大日志 tail 或安装进度应优先保持流式/SSE，不要等待长命令完成后一次性返回。
- 验证：`cd backend; go test -count=1 ./internal/docker`，`cd frontend; npm.cmd run build`。

# SUPPORT-BUNDLE-STREAM-1 联调契约

- `POST /api/instances/:id/support-bundle` 仍由管理员触发，仍返回 `application/zip`，文件名形如 `support-bundle-YYYYMMDD-HHMMSS.zip`。
- 后端现在流式写 ZIP，不再设置 `Content-Length`。前端下载逻辑应以 HTTP 成功和 Blob 内容为准，不要依赖总长度或进度百分比。
- ZIP 内条目和脱敏语义保持不变：版本、健康检查、实例状态、近期任务、审计摘要、Compose 状态、Compose 配置摘要和 server 日志 tail。
- 如果后续支持包单个条目采集失败，应在 ZIP 内写入对应 error/note 条目；流式响应开始后不能再切换成 JSON 错误体。

# JUNIMO-MOD-MOUNT-RESTORE-1 联调契约

- `/data/Mods` 由宿主 `.local-container/mods` 挂载提供；后端必须保证其中包含官方 `JunimoServer` Mod，否则 Junimo API、邀请码和 VNC rendering 都不会就绪。
- `JunimoServer`、`StardewAnxiPanel.Control` 和虚拟 `SMAPI` 都是内置组件：前端应展示为已启用/不可切换，不参与“第三方 Mod 默认禁用”。
- 前端不应展示物理 `smapi` 文件夹；接口层会跳过该目录，只返回虚拟 `SMAPI` 卡。
- VNC 显示失败如果收到 `junimo_api_unavailable`，文案应提示“JunimoServer API 未就绪/官方组件未加载”，不要只显示 Docker 操作失败。

# ENV-BOM-NORMALIZE-1 联调契约

- 启动服务器前后，实例 `.env` 必须是 Docker Compose 可解析的普通 `KEY=value` 文件；如果混入 UTF-8 BOM 前缀，例如 `﻿IMAGE_VERSION`，旧流程会在 `docker compose up` 前置解析阶段失败。
- 后端 `ReadEnvFile` / `UpdateEnvFile` 已对 BOM 前缀 key 做归一化；前端无需新增接口，只需要把生命周期 job 的失败日志展示出来即可。
- 联调排查顺序：先运行 `docker compose -f data/instances/stardew/docker-compose.yml config --quiet` 验证配置解析，再看容器启动日志；不要只根据面板里的 `docker compose up: docker command failed` 判断是镜像或游戏进程问题。

# STEAMCMD-SELFUPDATE-PROGRESS-1 联调契约

- SteamCMD 兜底镜像命中本地后，job 日志会先出现“本地已有 SteamCMD 镜像 ... 直接使用”和“Docker 镜像检查已完成”；之后的 `[steamcmd] [ N%] Downloading update (... of 40,273 KB)` 属于 SteamCMD 客户端自更新，不代表重新拉 Docker 镜像。
- 前端根据登录前的 SteamCMD bracket progress 展示客户端自更新进度；进入 `Logging in user`、`Waiting for user info` 或 app 安装后，后续进度再按游戏/SDK 下载处理。
- SteamCMD 手机 App 批准仍以 `steamcmd_guard_mobile_required` 驱动；日志里的 `Please confirm the login in the Steam Mobile app` 和 `Waiting for confirmation` 都应让页面提示打开 Steam App 批准。
- 验证：`cd backend; go test ./internal/games/stardew_junimo -run "SteamCMD|InstallFallsBack|InstallResumes|InstallUsesExistingLater|InstallSteamCMD"`；`cd frontend; npm.cmd run build`。

# STEAMCMD-RETRY-RESUME-1 联调契约

- 当 `steamcmd_failed` 或 `steamcmd_image_pull_failed` 后用户点击复用凭据重试，前端仍提交 `POST /api/instances/:id/install` 且 `reuseCredentials=true`；后端根据持久化 `driverPhase` 直接进入 SteamCMD fallback，不再先跑 `steam-auth`。
- 直达 SteamCMD 重试仍使用同一个 Steam Guard 输入接口：`POST /api/instances/:id/steam-guard/input`。前端看到 `steamcmd_guard_mobile_required` 时提示打开 Steam 手机 App 批准；看到 `steamcmd_guard_required` 时显示验证码输入框。
- 后端会先 inspect 所有 `STEAMCMD_IMAGE_CANDIDATES`。如果用户机器已有任意候选镜像，本次 job 日志会显示使用本地镜像并直接启动 SteamCMD；只有所有候选都缺失时才进入 `steamcmd_image_pulling`。
- 联调复现：先让 SteamCMD 手机批准超时，使实例落到 `state=error, driverPhase=steamcmd_failed`；再点安装页重试。预期不出现新的 `[steam]` / steam-auth 下载流程，不出现已存在 SteamCMD 镜像的 pull，直接出现 `[steamcmd] Logging in user...` 和授权提示。
- 验证：`cd backend; go test ./internal/games/stardew_junimo -run "SteamCMD|InstallResumes|InstallUsesExistingLater"`，`cd frontend; npm.cmd run build`。

# STEAMCMD-FALLBACK-1 联调契约

- 安装任务中只要 `steam-auth` 已经登录成功并进入游戏下载阶段，后续任何游戏文件下载失败都由后端自动尝试 SteamCMD 兜底，不再把用户带回 Steam 账号密码表单。典型日志顺序为 `[steam] [SteamAuth:A0] Logged in as ... -> Downloading app 413150 -> Download failed` 后继续出现 `[steamcmd] ...`。
- SteamCMD 兜底继续使用同一个 `stardew_install` job、同一条 SSE 流和同一个 `POST /api/instances/:id/steam-guard/input` 输入接口。前端只需要根据 `driverPhase`/日志展示 SteamCMD 专属授权 UI，不需要新增接口。
- SteamCMD 授权 phase：`steamcmd_guard_choice_required` 展示两个选择（`1`=手机 App 批准，`2`=App/邮箱验证码）；`steamcmd_guard_required` 提交验证码字符串；`steamcmd_guard_mobile_required` 只提示用户在手机 App 批准。提交成功后后端会乐观推进 phase，最终以 job 日志和实例 state 为准。
- SteamCMD 下载 phase：`steamcmd_image_pulling` 表示正在按 `STEAMCMD_IMAGE_CANDIDATES` 拉取兜底镜像，单个候选 403/超时会继续尝试下一个；`steamcmd_auth_running` 表示使用已保存账号密码登录；`steamcmd_downloading` 表示已授权并正在下载/校验 `413150`（并尝试 `1007` SDK）。
- 失败契约：`steamcmd_failed` / `steamcmd_image_pull_failed` 属于下载/环境失败，可重试并复用已保存凭据；`steamcmd_image_pull_failed` 表示全部候选镜像都不可用，运维可在实例 `.env` 中把可用内网镜像放入 `STEAMCMD_IMAGE_CANDIDATES`；`credentials_required` 表示 SteamCMD 认为账号、密码或验证码失败，前端应要求重新输入 Steam 凭据。
- 验证建议：模拟 `[SteamAuth:A0] Logged in as ... -> Downloading app 413150 -> Download failed -> [steamcmd] Success! App '413150' fully installed. -> [steamcmd] Success! App '1007' fully installed.`，最终实例应为 `game_installed`，job 应为 succeeded。

# INSTALL-INTERRUPTED-STATE-1 安装任务与实例状态联调契约

- 安装页不能只相信 `instance.driverPhase` 判断任务是否仍在运行，必须同时看 `GET /api/jobs` 中是否存在 queued/running 的 `stardew_install`。没有活跃安装 job 时，残留的运行中 phase 应按 `install_interrupted` 展示。
- 后端启动恢复 interrupted jobs 时，`stardew_install` 会同步更新实例为 `state=error`、`driverPhase=install_interrupted`；steam-auth 容器运行错误会同步更新为 `state=steam_auth_failed`、`driverPhase=steam_auth_failed`。
- 前端收到 `install_interrupted` 应显示失败/可重试，不应继续显示 QR、Steam Guard 或“正在使用已保存凭据认证并下载游戏”。
- 验证：启动安装后中断面板进程再重启，最新 `stardew_install` job 应为 failed，安装页应显示中断并加载该 job 日志，而不是卡在 48%。

# FE-STEAM-AUTH-DOWNLOAD-PROGRESS-RESTORE-1 联调契约

- 前端安装页解释 Steam 认证/下载日志时，以最新日志上下文为准：认证方式菜单下的 `Choice [1]: 2` 表示 QR；`Steam Guard Authentication` 菜单下的 `Choice [1]: 2` 表示输入手机 App/邮箱验证码。
- 历史 `Or open: https://s.team/q/...` URL 只能作为当前 QR 阶段的兜底信号；如果后续日志已经出现 Steam Guard 菜单、`Enter Steam Guard code`、手机批准等待、下载开始或失败 phase，前端不得继续显示扫码窗口。
- 日志出现 `Downloading app 413150`、`Target directory: /data/game`、`Manifest contains` 或 `Progress: N/M files - done/total (...)` 后，前端应显示 `game_downloading` 下载卡。`Progress:` 日志应渲染文件数、体积和进度条；后续 SDK 下载同理显示 `steam_sdk_downloading`。
- 联调复现场景：手机批准后日志出现 `[SteamAuth:A0] Logged in as ...`、`Downloading app 413150`、`Progress: 300/1470 files ...`，右侧认证区应显示“下载 Stardew Valley 游戏文件中…”和进度条，不应继续显示“请打开 Steam 手机 App，批准此次登录请求”。
- 验证：`cd frontend; npm.cmd run build`；活跃安装任务手动联调上述日志顺序。

# JOB-DISPLAY-NAME-1 联调契约

- `GET /api/jobs`、`GET /api/jobs/:id` 和 job SSE 的 job payload 可能返回 `displayName`；前端应优先展示该字段，没有时回退 `type`。
- `type` 仍是机器可读任务类型，例如 `mod_remote_install`，不要在前端用它判断具体 Mod；Nexus/远程 Mod 安装的用户可读名称在 `displayName` 中，例如 `mod_remote_install · Farm Type Manager (FTM)`。
- 扩展普通一键安装提交 `POST /api/instances/:id/mods/remote/install` 时应继续传 `mod.name`，这样后端能给并行依赖任务写入不同展示名。
- 验证：`cd backend; go test ./internal/storage ./internal/jobs ./internal/web`、`cd frontend; npm.cmd run build`。

# MODUPLOAD-DUPLICATE-CODE-1 联调契约

- `POST /api/instances/:id/mods/upload` 上传合法 SMAPI ZIP 时，如果实例里已存在相同 `UniqueID` 的 Mod，响应应为 `400 { error: { code: "mod_exists", ... } }`。
- 该错误表示“已安装相同 ID 的 Mod”，不是 ZIP 结构损坏；前端可直接显示已有 `mod_exists` 文案。
- 损坏 ZIP、缺少 manifest、XNB 替换包、manifest 解析失败等仍属于 `invalid_mod_zip`。
- 验证：`cd backend; go test ./internal/web -run "TestModUpload"`。

# FE-OPSRAIL-DOWNLOAD-PROGRESS-1 联调契约

- 扩展普通一键安装在成功创建面板后端任务后，应尽快把 `jobId` 返回给面板页；面板收到新的 batch `jobId` 后会立即刷新 `GET /api/jobs`，右栏“进行中”不应再等 30s 轮询才出现。
- 右栏远程安装进度依赖后端 job 日志：`GET /api/jobs/:jobId/logs` 和 `GET /api/jobs/:jobId/stream` 的 `log` 事件需要包含 `下载进度：已下载 ...（xx.x%）` 这类消息，面板据此显示下载阶段进度。
- 下载百分比只代表 ZIP body 下载阶段；右栏会把它映射到任务整体进度的中前段，下载 100% 后仍会显示校验/安装阶段，任务真正完成以后由 `finished` 事件刷新 jobs 并移除进行中行。
- 联调验收：普通一键安装点击后，扩展返回 `jobId` 时右栏立即出现 `mod_remote_install`；下载日志从 0% 到 100% 时右栏进度同步推进；若扩展没有拿到 ZIP 链接，则不应出现后端 job。
- 验证：`cd frontend; npm.cmd run build`。

# NEXUS-EXT-DOWNLOAD-GUARD-1 联调契约

- 扩展只有在捕获到真实 Nexus CDN ZIP 链接时才允许调用 `POST /api/instances/:id/mods/remote/install`。合法链接需满足 HTTPS、host 为 `supporter-files.nexus-cdn.com` 或其它 `*.nexus-cdn.com`，路径以 `.zip` 结尾。
- 后台页仍停在 Nexus 文件页、Manual Download 页、Slow Download 页、Additional files 弹窗或错误页时，不应创建面板安装任务；前端批量进度只能显示捕获中/超时失败，不能显示 queued/jobId。
- 后端远程安装任务日志现在必须能区分卡点：`正在连接远程下载服务器` 表示已拿到 URL 正在等响应头；`远程下载服务器已响应：HTTP ...` + `远程压缩包大小...` / `下载进度...` 表示已经开始读取 ZIP body。
- 如果后端收到 `text/html`，任务应失败并提示远程下载返回网页而不是 ZIP；联调时优先检查扩展是否真的捕获到 CDN ZIP，而不是只打开了 Nexus 下载页面。
- 验证：`go test ./internal/games/stardew_junimo ./internal/web`，以及扩展三个脚本 `node --check`。

# PLAYERSYNC-PACK-2 联调契约

- `POST /api/instances/:id/mods/sync-pack/export` 下载文件名仍是 `stardew-player-sync-pack.zip`，但 ZIP 内容升级为安装包：根目录含 `安装玩家同步包.bat`、`卸载本同步包.bat`、`README.txt`、`pack-manifest.json`、`checksums.sha256`、`tools/` 和 `payload/`。
- 前端无需解析 ZIP；仍按 Blob 下载即可。下游若需要读取包内容，应以 `pack-manifest.json` 为准，普通 Mod 文件在 `payload/mods/<folderName>/`，SMAPI 元数据在 `payload/smapi/smapi.json`。
- `checksums.sha256` 校验 `payload/mods` 和随包 SMAPI ZIP；玩家端脚本会在复制 Mod 前校验。若包内没有 SMAPI ZIP，`pack-manifest.json.smapi.bundled=false`，脚本继续安装 Mod 并提示玩家自行安装 SMAPI。
- 玩家端安装状态落在游戏目录 `.anxi-sync/installed.json`、`.anxi-sync/backups/`、`.anxi-sync/logs/`。卸载脚本按该记录移除本包安装的 Mod，可用 `-RestoreBackup` 恢复备份；不会默认卸载玩家已有 SMAPI。
- 验证：`go test ./internal/games/stardew_junimo ./internal/web`、`go test ./...`。

# NEXUS-SMAPI-THUMB-1 联调约定

- `GET /api/instances/:id/mods` 返回的虚拟 SMAPI 条目也会带 `nexusModId=2400` 和 `nexusUrl`，并在后端 Nexus GraphQL 补全成功后带 `pictureUrl`。
- 前端无需为 SMAPI 写死图片 URL；统一使用 `mods[].pictureUrl`。如果首次请求时 Nexus 不可用或未返回图片，保持现有 `NEXUS` 文字占位即可。
- 该行为不要求用户安装真实 SMAPI Mod 文件夹；它只用于面板展示和玩家同步清单语义。
- 对随 Nexus 包安装的内容包，如果其自己的缓存记录没有 `pictureUrl`，后端会按同一个 Nexus `modId` 合并主 Mod 的完整缓存；前端仍只读 `mods[].pictureUrl`，不需要自己按来源包查找图片。

# MODDEPS-1 联调约定

- `GET /api/instances/:id/mods` 的 `mods[]` 可能包含 `dependencies[]`，每项结构为 `{ "uniqueId": string, "minimumVersion"?: string, "required": boolean }`。
- 字段来源是 SMAPI `manifest.json` 的 `Dependencies` 和 `ContentPackFor`；`ContentPackFor` 统一按必需依赖返回，重复 UniqueID 会去重。
- 前端已安装 Mod 卡片只把 `required=true` 的依赖展示为“需要前置依赖：...”标签；`required=false` 的可选依赖暂不展示。
- 该字段不代表后端已验证依赖是否存在，也不代表安装接口会自动补装依赖。后续缺失依赖检查应基于同一 `dependencies[]` 与已安装 `uniqueId` 列表继续扩展。

# MODUPLOAD-2 联调约定
- `POST /api/instances/:id/mods/upload` 仍是管理员专用，服务器 running/starting 时仍返回 `409 server_running`；请求格式是 `multipart/form-data`。
- 前端可以在同一个请求里重复追加多个 `mod` 文件字段，例如 `form.append('mod', fileA)`、`form.append('mod', fileB)`；后端同时兼容字段名 `mods`，但推荐继续使用重复 `mod` 字段。
- 成功响应仍是 `ModsListResult`：`mods[]` 包含本次批量上传导入出的所有 Mod，`restartRequired` 继续遵循现有 Mod 重启语义；停服上传成功时应为 `false`，下次启动会直接加载新 Mod。前端成功后应刷新 `GET /api/instances/:id/mods` 和仪表盘缓存。
- 任意一个 ZIP 校验/解压/导入失败时，后端会回滚本请求已导入的前序 Mod，并返回错误；前端应把这次上传视为失败，不要假设部分成功。
- 单个 ZIP 内含多个顶层 SMAPI Mod 的能力仍由 `UploadModZip` 提供；“一次选择多个 ZIP”和“一个 ZIP 里多个 Mod”可以同时工作。

# NEXUS-META-1 联调约定
- `GET /api/instances/:id/mods` 可能在返回前触发一次 Nexus GraphQL v2 元数据补全：当本地 Mod manifest 有 `UpdateKeys` 中的 `Nexus:<id>` 且 sidecar 尚无缓存时，后端会无 Key 查询 Nexus 缩略图和展示字段，成功后写入 `.local-container/control/nexus-mods.json`。
- 该补全不改变接口结构，只让 `mods[]` 里的 `pictureUrl/nexusSummary/nexusUrl/downloadCount/endorsementCount/updatedAt` 更完整；Nexus 请求失败时接口仍返回 200 和本地 Mod 信息。
- `GET /api/instances/:id/mods/nexus/search?q=<数字ID>` 未配置 Nexus API Key 时也应返回 GraphQL v2 精确 ID 结果；API Key 只影响 v1 REST 查询和 Nexus 下载安装，不再是展示缩略图/元数据的前置条件。

# MODZIP-1 上传错误约定

- `POST /api/instances/:id/mods` 只接受标准 SMAPI Mod ZIP。若用户上传 Nexus 上的老式 XNB 替换包（例如只包含 `Characters/*.xnb`、`Portraits/*.xnb`，没有 `manifest.json`），后端返回 `400 invalid_mod_zip`，message 会明确提示这是 XNB 替换包，不是 SMAPI Mod，不能上传到服务器 `Mods` 目录。
- SMAPI `manifest.json` 解析兼容 UTF-8 BOM，以及字符串外的 `//` / `/* ... */` 注释和尾随逗号；这只用于 manifest 读取，不代表上传接口接受非 ZIP、非 SMAPI Mod 或 XNB 替换包。

# NEXUS-PAGED-1 联调约定
- `GET /api/instances/:id/mods/nexus/search?q=...&page=...&pageSize=...`：任意登录用户可用，返回 Nexus 专用模型 `{ query, results, page, pageSize, total, hasMore }`。
- 空 `q` 合法，用于默认热门列表；关键词搜索由后端通过 Nexus GraphQL v2 下推 `downloads DESC` 排序和 `offset/count` 分页；纯数字 ID 按 Nexus Mod ID 精确查询。
- `POST /api/instances/:id/mods/nexus/install`：管理员专用，服务器 running/starting 时返回 `409 server_running`；需要 Nexus API Key 和后端可用的文件下载权限，成功返回 `202 { "jobId": "..." }`。
- `/api/instances/:id/mods/search` 与 `/api/instances/:id/mods/search/install` 已撤下，不再作为联调契约。
- 前端创建安装任务后订阅 `GET /api/jobs/:jobId/stream`，完成后拉 `GET /api/jobs/:jobId` 并刷新 `GET /api/instances/:id/mods`。粘贴 URL / 上传文件仍作为兜底入口。

# REMOTE-MOD-1 联调约定
- `POST /api/instances/:id/mods/remote/install`：管理员专用，服务器 running/starting 时返回 `409 server_running`。请求体 `{ "url": string, "mod"?: NexusModSearchResult-like }`，成功返回 `202 { "jobId": "..." }`。
- `url` 为 `nxm://...` 时，后端解析 `modId/fileId/key/expires` 并读取 SQLite 中的 Nexus API Key 调 v1 `download_link.json?key=...&expires=...`；未配置 Key 时任务失败为 `ErrNexusAPIKeyMissing`。
- `url` 为 `https://...zip` 时，后端直接下载该 ZIP，再走现有 `UploadModZip` 校验/解压/导入；该直链来源可以是 Nexus CDN、ModDrop、GitHub、CurseForge 等公网 HTTPS ZIP。当前不支持 7z/rar。
- 前端创建任务后订阅 `GET /api/jobs/:jobId/stream`，与 `mod_nexus_install` 相同。任务成功后刷新 `GET /api/instances/:id/mods`。
- 为防止临时授权泄漏，前端和审计日志不保存粘贴 URL；失败信息不应包含完整 NXM/CDN URL。

# NEXUS-EXT-1 联调约定
- `browser-extensions/nexus-slow-installer` 是独立浏览器扩展实验包，用于免费 Nexus 用户的慢速下载链路：本地浏览器登录 Nexus -> 扩展在文件页点击/等待 `Slow download` -> 捕获浏览器下载任务中的 Nexus CDN `.zip` 临时链接 -> 调用面板 `POST /api/instances/:id/mods/remote/install`。
- 扩展第一版不新增后端接口，请求体仍是 `{ "url": string, "mod"?: { "modId": number, "name"?: string, "nexusUrl"?: string } }`；后端按 REMOTE-MOD-1 的 `.zip` 直链规则下载并安装。
- 扩展调用面板接口时使用 `credentials: "include"` 复用同浏览器中的面板管理员登录态。若正式云端部署遇到 SameSite/Cookie 或跨域策略导致无法带登录态，应新增受限的扩展配对 token，而不是让扩展保存管理员密码。
- 联调前置：面板管理员已登录、服务器已停止、Nexus 账号已登录、Nexus CDN 临时链接可由云端后端访问。测试失败时优先区分三类问题：扩展未捕获下载、面板鉴权 401/403、后端下载/导入 ZIP 失败。
- 扩展状态与日志必须脱敏 `md5/expires/user_id/key`；完整临时 URL 只作为请求体短暂发送给面板，不应落入长期文档、审计或支持包。

# NEXUS-EXT-3 联调约定

- 前端 Nexus 搜索结果的“一键安装”主路径已经切到扩展链路：点击后同页跳转到 Nexus Mod 文件页，并附加 `anxi_auto=1`。前端不再为该按钮直接调用 `POST /api/instances/:id/mods/nexus/install`，也不再要求用户配置 Nexus API Key。
- 扩展进入带 `anxi_auto=1` 的 Nexus 页面后自动打开手动下载/慢速下载流程，捕获到 Nexus CDN `.zip` 临时链接后只等待用户点击“提交到面板”。提交时复用 `POST /api/instances/:id/mods/remote/install`，成功响应仍为 `202 { "jobId": "..." }`。
- 扩展提交成功后跳回 `/instances/:id/jobs?jobId=<jobId>`；`JobsLogsPage` 应优先读取 `jobId` 查询参数并加载该任务详情和日志。若任务不在第一页列表里，右侧详情仍应通过 `GET /api/jobs/:jobId` 加载。
- 旧 `POST /mods/nexus/install` 可以保留给后续 Premium/API Key 直连或调试使用，但当前用户入口以扩展 + remote install 为准。

# NEXUS-3 联调约定

- `GET /api/instances/:id/mods/nexus/search?q=...`：无 Nexus API Key 时也应能走 GraphQL v2 搜索；纯数字 query 未配置 Key 时按 GraphQL v2 的 `gameId=1303 + modId` 精确查询，已配置 Key 时仍可按 v1 REST 精确 ID 查询。
- `POST /api/instances/:id/mods/nexus/install`：管理员专用，请求体为当前 Nexus 搜索卡片字段（至少 `modId`，建议带 `name/summary/version/pictureUrl/nexusUrl/downloadCount/endorsementCount`）。未配置 Key 返回 `503 nexus_api_key_missing`；服务器运行中返回 `409 server_running`；成功返回 `202 { jobId }`。
- 前端安装后订阅 `GET /api/jobs/:jobId/stream`，展示 `log` 事件，`finished` 后拉取 `GET /api/jobs/:jobId` 判断 succeeded/failed，并刷新 `GET /api/instances/:id/mods`。
- `GET /api/instances/:id/mods` 的 `mods[]` 现在可能包含 Nexus 卡片字段：`nexusSummary`、`pictureUrl`、`nexusUrl`、`downloadCount`、`endorsementCount`、`updatedAt`。前端可用这些字段把已安装 Mod 渲染成与搜索结果一致的卡片。
- 安装流程不新增前端直连 Nexus；所有 Nexus 文件列表、下载链接、下载 ZIP、解压安装都由后端代理和现有 Mod ZIP 安全校验完成。

# 前后端联调文档

## 联调目标

前后端联调必须验证三件事：

1. API 结构和错误码稳定。
2. 长任务、SSE、Steam Guard、邀请码刷新等异步流程可恢复。
3. UI 状态和后端实例状态一致，不误导用户。

## 本地启动

后端：

```powershell
cd E:\stardew-server-anxi-panel\backend
$env:GOCACHE='E:\stardew-server-anxi-panel\.gocache'
go run .\cmd\panel
```

前端：

```powershell
cd E:\stardew-server-anxi-panel\frontend
npm.cmd run dev
```

访问 Vite 地址，前端开发代理应能访问后端 API。完整打包验证见 `docs/09-image-build.md`。

## 关键联调流程

### 1. 初始化与登录

- 首次访问显示管理员初始化。
- 创建管理员后自动登录。
- 登出后回到登录页。
- 错误密码显示中文错误提示。
- 未登录访问 API 返回 401，权限不足返回 403。

### 2. 安装与 Steam Auth

- `prepare` 创建实例目录和 compose/env。
- `install` 创建 job。
- 前端订阅 job SSE。
- Steam Guard / QR / 验证码阶段前端能提交输入。
- job 结束后状态刷新。
- 日志不包含 Steam/VNC 密码。

### 3. 生命周期与邀请码

- 未安装时启动返回结构化错误，前端提示“请先安装游戏”。
- 有可用存档后可启动。
- 同一实例的启动、停止、重启任务必须互斥；用户点击停止后，旧启动任务应变为 `canceled`，不能继续显示 running。
- 启动/重启提交成功后前端清空旧邀请码，但启动按钮等待 active lifecycle job 与实例 running 状态，不再等待新邀请码、在线玩家或 SMAPI 存档加载日志。
- 后端过滤容器内旧 `/tmp/invite-code.txt`。
- 后端启动前会清理旧 SMAPI `status.json` / `players.json` 快照，避免旧玩家/存档状态造成前端启动完成闪烁。
- 停止后前端清空邀请码。

### 4. 存档

- 上传 ZIP 先预览，不直接写正式目录。
- 用户确认后再提交导入并启动。
- 新建存档必须生成 Stardew/Junimo 可读真实存档，不能只写表单摘要。
- 删除前自动备份。
- 运行中禁止删除或覆盖危险操作。

### 5. Mod

- 上传、删除、导出可用。
- 运行中危险操作禁用。
- 上传/删除/安装 Mod 写操作要求服务器停止；停服修改后下次启动会自动加载，不再提示需要重启。`restartRequired` 只用于运行中已有 Mod 变更待应用的历史/兼容场景，服务器停止时接口应返回 `false`。
- 玩家同步分类（`syncKind`：`server_only`/`client_required`/`unknown`）随 `GET /api/instances/:id/mods` 一起返回，前端不用单独再拉一次。没有手动覆盖时，后端会自动把面板控制组件标为 `server_only`，把 SMAPI 内容包和其他第三方 Mod 标为 `client_required`，并在 `syncNote` 写入自动识别说明。
- `GET /api/instances/:id/mods/sync-plan` 返回分类统计；`PUT /api/instances/:id/mods/:modId/sync-classification` 任意登录用户可用，编辑不受运行状态限制；`POST /api/instances/:id/mods/sync-pack/export` 任何登录用户可用，运行中也允许导出，导出包含 `pack-manifest.json`、`checksums.sha256`、安装脚本和 `payload/mods/`，且永远不含面板自带的 `StardewAnxiPanel.Control`。
- 无 `client_required` Mod 时导出接口返回 `400 no_sync_mods`，前端按钮直接禁用避免命中。
- `GET /api/instances/:id/mods/nexus/search?q=关键词`：任意登录用户可用（不需要管理员权限），后端代理 Nexus Mods 官方 API，前端不直连 N站。鉴权按能力拆开：关键词搜索和无 Key 纯数字 ID 展示查询都走公开只读的 GraphQL v2，**不需要个人 API Key**；配置 Key 后纯数字 ID 可优先走 v1 REST 精确查询。只有当 Nexus 自己因鉴权拒绝 GraphQL 查询时才返回 `502 nexus_auth_required`（提示需要 OAuth/更高权限，配置 Key 不一定能解决）。空关键词作为默认热门列表返回 200；其余上游非 2xx 映射为 `404 nexus_mod_not_found` / `502 nexus_unauthorized`（v1 REST Key 无效/权限不足）/ `429 nexus_rate_limited` / `502 nexus_request_failed`。后端 message 必须保持正常 UTF-8 中文，前端也会按这些 Nexus 错误码兜底显示稳定中文。返回结果按本地已装 Mod 的 manifest `UpdateKeys`（`Nexus:<id>`）匹配 `installed`，本阶段不做版本新旧判断。
- `GET /api/settings/nexus` / `PUT /api/settings/nexus/api-key` / `DELETE /api/settings/nexus/api-key`：管理员专用的 Nexus Key 配置接口。PUT 请求体 `{ "apiKey": string }`，保存后当前进程立即生效；GET 只返回 `{ configured, last4? }`，不会回显完整 Key；DELETE 清除配置。

### 6. 控制台命令

- 普通用户只看到允许的只读命令。
- 管理员看到完整 allowlist。
- 不允许任意 shell。
- 服务器未运行时命令禁用或返回 `server_not_running`。
- `POST /api/instances/:id/commands/say` 请求体为 `{ "message": string }`；成功返回 `CommandRunResult{ command: "say", output, exitCode: 0, durationMs }`，表示后端已把喊话命令写入控制目录。
- 喊话由 `StardewAnxiPanel.Control` 消费 `.local-container/control/commands/*.json` 后发送到游戏聊天，实际玩家可见文本前缀为 `[Panel]`。如果服务器已运行但世界尚未 ready，控制模组会在 `status.json` 记录忽略原因，后端不会暴露任意 SMAPI 命令入口。
- 前端仍应在非 running 状态禁用喊话输入；运行中发送失败时按结构化错误码展示，成功时提示“已提交/已发送”即可，不需要等待聊天回执。

### 7. 玩家页

- `GET /api/instances/:id/players` 返回在线快照和缓存名册。
- 玩家名册会合并当前存档主 XML 中的 `<player>` 与 `<farmhands><Farmer>`；存档存在但当前不在线、也没进入缓存的玩家应显示为 `status=offline`、`source=save_file`，例如 `saveId=test` 可匹配 `Saves/test_数字` 下的 farmhand。
- `maxPlayers` 默认取当前存档 `server-settings.json` 的 `Server.MaxPlayers`（junimo info 解析出的值优先）；服务器未运行时也会返回，供前端显示"在线数/人数上限"。
- 前端显示 online/offline、host、位置、tile/pixel。
- 未知地图 key 保留原值。
- 玩家页固定展示 `money`、`farmIncome`、`personalIncome` 和 `walletMode`；`farmIncome` 是农场/团队累计收入，`personalIncome` 是玩家个人累计收入，不随钱包模式改变含义。
- `recentEvents` 返回最近玩家活动，至少覆盖首次记录、加入和离开；事件必须按 `saveId` 隔离。
- 新建/切换存档后，玩家缓存必须按 `saveId` 隔离；上一存档玩家不应出现在当前存档列表。
- `POST /api/instances/:id/players/warp-home`：管理员专用，body `{ "uniqueMultiplayerId": string, "name"?: string }`。实例必须为 `running`，且控制模组 `status.json` 必须暴露 `warpHomeBridgeAvailable=true`。成功返回 `CommandRunResult{ command:"warp-home", exitCode:0 }`，只表示已提交到控制模组；实际游戏内传送由下一次 SMAPI tick 消费命令后调用 JunimoServer `FarmerExtensions.WarpHome(Farmer)` 完成。
- 回家按钮的前端禁用条件应和后端约束一致：非管理员、服务器未运行、目标离线、目标为 host、缺少 `uniqueMultiplayerId` 时禁用。失败时优先按结构化 `error.code` 显示中文提示；常见错误为 `server_not_running`、`warp_home_bridge_unavailable`、`invalid_player`。

## API 约定

错误响应应保持结构化：

```json
{
  "error": {
    "code": "server_not_running",
    "message": "服务器未运行"
  }
}
```

前端优先使用 `code` 映射中文提示，`message` 作为兜底。

## 状态校准

面板状态机是 UI 流程来源，但 Docker 和 Junimo 是运行事实来源。后端启动和关键操作前应校准：

```text
docker compose ps
docker compose logs --tail
Junimo HTTP status（如启用）
SMAPI / control files
.env、docker-compose.yml、.local-container
active save metadata
```

典型规则：

- 面板记录 running，但 compose 显示 server 停止，应回到 stopped 或 error。
- install 完成但未选择存档，应保持 `save_required`。
- start 无有效存档，应返回 `save_required` 并引导前端到 Saves。

## 常用验证命令

后端：

```powershell
cd E:\stardew-server-anxi-panel\backend
$env:GOCACHE='E:\stardew-server-anxi-panel\.gocache'
go test ./...
```

前端：

```powershell
cd E:\stardew-server-anxi-panel\frontend
npm.cmd run build
```

冒烟：

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\smoke-test.ps1
```

可选参数：

- `-SkipDocker`
- `-SkipFrontend`
- `-SkipBackend`
# SMAPI-RUNTIME-1 联调约定

- `GET /api/instances/:id/mods` 可能返回 `mods[0].builtIn=true` 的 `SMAPI` 虚拟条目；前端应把它作为置顶内置组件展示。
- `builtIn=true` 条目不是服务器 Mods 目录中的真实文件夹，不能调用删除接口，也不能调用同步分类更新接口；后端同步包导出会忽略该条目。
- 该条目的 `syncKind=client_required` 表达“玩家客户端需要先安装 SMAPI”，但它不会进入玩家同步 ZIP；前端同步统计应排除 `builtIn`，普通玩家看到的是提示而不是可下载内容。
- SMAPI 条目只有在面板内置控制 Mod `StardewAnxiPanel.Control` 已出现在实例 Mods 目录时才注入，用来避免未准备实例显示运行组件。

# MODORIGIN-1 Nexus 包来源字段

- `GET /api/instances/:id/mods` 的 `mods[]` 可能同时包含 `nexusModId` 和 `originNexusModId` 两类字段。`nexusModId` 只表示该 Mod 文件夹自己的 SMAPI `UpdateKeys` 声明；`originNexusModId` 表示它随某个 Nexus 下载包一起安装。
- 前端展示规则：`nexusModId>0` 显示为主 Nexus Mod；`nexusModId` 为空且 `originSource="nexus"` 时显示为“来源：N站包 / 随 <originModName> 安装”。不要把 `originNexusModId` 当作该内容包自己的 `nexusModId`，否则 `[CP]` 内容包会被误认为独立 N站 Mod。
- 后端会把来源包的 `pictureUrl/downloadCount/endorsementCount/updatedAt` 填到内容包卡片上，用于展示缩略图和统计；同步分类、玩家同步包导出仍按真实 Mod 文件夹处理。
- 删除是例外：`DELETE /api/instances/:id/mods/:modId` 会按来源包 bundle 删除同组真实 Mod 文件夹。前端删除确认应根据当前 `mods[]` 计算同 `nexusModId/originNexusModId` 的组成员并提示用户会一起删除；确认后只调用一次 DELETE，不要在前端循环多次删除。
# NEXUS-PAGED-1 联调契约

- 模组下载页在线搜索只调用 `GET /api/instances/:id/mods/nexus/search?q=...&page=...&pageSize=...`。
- 响应结构为 `NexusModSearchResponse{query, results, page, pageSize, total, hasMore}`；前端用 `hasMore` 控制下一页，用 `page > 1` 控制上一页。
- 关键词搜索在后端通过 Nexus GraphQL v2 下推 `downloads DESC` 排序和 `offset/count` 分页；前端不再调用 `/mods/search` 统一搜索骨架。
- Nexus 一键安装继续调用 `POST /api/instances/:id/mods/nexus/install`；管理员粘贴 Nexus `nxm://` 或 Nexus CDN 临时 ZIP 仍走 `POST /api/instances/:id/mods/remote/install`。
- `/api/instances/:id/mods/search` 与 `/api/instances/:id/mods/search/install` 已撤下，不再作为联调契约。
- 验证：`go test ./internal/games/stardew_junimo ./internal/web`、`npm.cmd run build`。

# SMAPI-SYNC-2 联调契约

- `GET /api/instances/:id/mods` 可能同时返回两个 `builtIn=true` 条目：虚拟 `SMAPI` 与真实 `StardewAnxiPanel.Control`。前端不要仅凭 `builtIn` 判断是否排除玩家同步；SMAPI 需要进入同步统计和同步清单，Control 不进入。
- `SMAPI`：`uniqueId=Pathoschild.SMAPI`、`syncKind=client_required`、`builtIn=true`。它代表玩家客户端前置要求，导出同步包时进入 `pack-manifest.json` 的 `mods[]` 和 `smapi` 元数据；只有服务端已缓存 SMAPI ZIP 时才会写入 `payload/smapi/SMAPI*.zip`。
- `StardewAnxiPanel.Control`：`folderName=StardewAnxiPanel.Control`、`builtIn=true`、`syncKind=server_only`。前端不得显示删除按钮或同步分类下拉；后端也会拒绝删除并排除同步包。
- `pack-manifest.json` 条目包含 `builtIn` 与 `packaged`：下游如果做玩家同步安装器，应只自动复制 `packaged=true` 的 Mod；`packaged=false` 的 SMAPI 是玩家前置要求，不是 Mod 文件夹。
- 验证：`go test ./internal/games/stardew_junimo ./internal/web` 与 `npm.cmd run build`。
# PLAYERSYNC-PACK-15 联调契约

- 完整同步包接口保持不变：`POST /api/instances/:id/mods/sync-pack/export`，下载文件名 `stardew-player-sync-pack.zip`，用于首次加入玩家，可按服务端缓存情况携带 `payload/smapi/SMAPI*.zip`。
- 新增模组更新包接口：`POST /api/instances/:id/mods/sync-pack/export-update`，下载文件名 `stardew-player-mods-update-pack.zip`，用于已经运行过完整版同步包的玩家。
- 更新包 ZIP 内 `pack-manifest.json.packType=mods_update`，不包含 `payload/smapi/`，`checksums.sha256` 只校验 `payload/mods/`。安装脚本会要求玩家游戏目录已存在 `StardewModdingAPI.exe`，否则提示先运行完整版同步包。
- 前端只按 Blob 下载，不需要解析 ZIP；UI 上应把完整包和更新包区分展示。更新包没有真实可打包 Mod 时后端返回 `400 no_sync_mods`，前端按钮也应在只有虚拟 SMAPI 时禁用。
- 验证：`go test ./internal/games/stardew_junimo ./internal/web`、`npm.cmd run build`。

# PLAYERSYNC-PACK-16 联调契约

- 模组更新包 `stardew-player-mods-update-pack.zip` 的 `tools/install.ps1` 不再读取或修改 Steam 启动项，ZIP 内也不包含 `tools/steam-launch-options.ps1`。
- 更新包仍要求玩家游戏目录已有 `StardewModdingAPI.exe`；缺失时提示先运行完整版玩家同步包。
- 更新包安装完成摘要显示 `Steam 启动项：已跳过，沿用已有设置`，不会再输出 `Steam 启动项文本` 或复制提示。
- 完整同步包契约不变：首次玩家包仍会尽力自动配置 Steam 启动项，失败时输出可复制 launch options。
- 前端无需新增字段；继续区分“完整同步包”和“模组更新包”的下载按钮即可。

# MODPROFILE-1 联调契约

- `GET /api/instances/:id/mods` 返回的 `mods[]` 新增 `enabled/canToggle/enableNote`，用于展示当前激活存档下的 Mod 启用状态。
- 禁用的 Mod 仍会出现在 `GET /mods` 响应中；前端必须读取 `enabled`，不要用是否出现在列表里判断启用。
- 新增 `PUT /api/instances/:id/mods/:modId/enabled`。管理员专用、服务器 running/starting 时不可用；请求体 `{ "enabled": true|false, "saveName"?: string }`，不传 `saveName` 时使用当前激活存档。
- 新建存档和新导入存档默认只启用内置组件，第三方 Mod 需要在配置页手动开启。旧存档没有 profile 时保持当前物理目录状态。
- 启动前后端会按当前存档 profile 移动 Mod 目录，因此玩家同步包导出仍只打包当前启用目录里的玩家 Mod。
- 验证：`go test ./internal/games/stardew_junimo ./internal/web`、`npm.cmd run build`。
# MODPROFILE-2 联调契约

- `POST /api/instances/:id/saves/select` 和 `POST /api/instances/:id/saves/select-and-start` 切换存档后，后端会应用对应存档的 Mod profile；前端在收到切换成功并刷新 saves 后必须刷新 `GET /api/instances/:id/mods`，用新的 `enabled/canToggle/enableNote` 渲染当前存档状态。
- 模组农场创建成功后的 profile 会保留创建时已启用的 Mod，并额外强制启用农场 provider 与必要依赖；不能把依赖闭包解释为该存档唯一允许启用的 Mod 集合。创建前已关闭的 Mod 保持关闭，后续新安装 Mod 仍按该 profile 的默认关闭策略处理。
- 批量启停使用 `PUT /api/instances/:id/mods/enabled` 与 `{enabled: boolean, saveName?: string}`；它不是逐 Mod endpoint 的前端循环。成功响应为 `{mods, enabled, saveName, changedCount}`。权限、停服和活动存档门禁与单项 `PUT .../mods/:modId/enabled` 一致，built-in 组件不在 `mods`/`changedCount` 内。
- 公共数据层现在监听 `activeSaveName`，活动存档变化会触发 mods 刷新；页面不要缓存旧 `mods` 当作跨存档状态。
- 验证：`go test ./internal/games/stardew_junimo ./internal/web`、`npm.cmd run build`。

# NEXUS-DEFAULT-1 联调契约

- `GET /api/instances/:id/mods/nexus/search?q=&page=1&pageSize=20` 合法，返回 Nexus Stardew Valley 默认热门列表，不再返回 `invalid_query`。
- 空 `q` 响应结构仍是 `{ query, results, page, pageSize, total, hasMore }`，其中 `query` 为 `""`；前端用同一套 Nexus 结果卡片和分页控件展示。
- 关键词、数字 ID、安装接口契约不变。只有下载页默认态和空输入刷新热门依赖这个新行为。
- 验证：`go test ./internal/games/stardew_junimo ./internal/web`、`npm.cmd run build`。
# NEWGAME-CABINS-1 自定义新存档小屋联调契约

- `POST /api/instances/:id/saves/custom-new-game` 的 `startingCabins` 表示“初始联机小屋数量”，不是总玩家数；合法范围为 0-7。
- 前端新建存档 UI 必须直接显示并提交 `startingCabins`，不要把主玩家加 1 后当作“小屋数”展示。
- 后端会同时写 Junimo `server-settings.json` 的 `Game.StartingCabins`、控制模组 `server-init.json` 的 `cabinCount/cabinLayout`，以及 `.local-container/control/new-game-pending` 一次性标记；控制模组只在该标记存在时提前同步 Stardew 原生新建参数。
- 控制模组不再作为存档创建方；联调时应以 Junimo HTTP `POST /newgame` 和 Junimo 生成的存档目录为准，Control 只负责新建前参数同步和新建后角色定制。
- 联调验证建议：创建 0、1、2 间小屋的新存档后，分别检查 `.local-container/settings/server-settings.json`、`.local-container/control/server-init.json`、`.local-container/control/new-game-pending`，并解析生成存档 XML 中可见/有效 Cabin building 数；已有存档不会因 `server-init.json` 残留而重新应用新建参数。

# FE-QUICK-BACKUP-1 快捷备份联调契约

- 服务器控制页“备份存档”复用现有 `POST /api/instances/:id/saves/:name/backup`，其中 `name` 必须来自 `GET /api/instances/:id/saves` 的 `activeSaveName`。
- 前端仅管理员可点；无激活存档时禁用。成功响应仍为 `{ "backupName": string }`，前端只展示文件名并不解析 ZIP。
- 该操作在 UI 中显示为“备份已保存进度”，语义是“对当前磁盘存档目录打 ZIP 手动备份”，不是“强制 Stardew 立即保存当前游戏内世界”。游戏内未触发保存事件的进度不会因为备份按钮自动写入主存档。
- 手动验证：运行中和停服状态分别点击快捷备份，确认能创建手动备份；随后进入存档页备份列表应能看到同一备份文件。


# SAVE-BACKUP-POLICY-1 / SAVE-BACKUP-SCHEDULE-HOUR-1（已废弃，被 SAVE-BACKUP-GAMEDAY-1 取代）

> 说明：本节原始记录在历史保存中损坏为不可逆的 `?` 占位符乱码，且这两版描述的"最新备份/每日快照/定时备份"机制已被下方 `SAVE-BACKUP-GAMEDAY-1` 全部取代（不再有 scheduled 分支，`BackupPolicy` 字段也已改变）。保留本节标题仅作为历史索引，具体契约请直接看 `SAVE-BACKUP-GAMEDAY-1`。

# SAVE-BACKUP-GAMEDAY-1 存档回档联调契约（游戏内日期驱动）

- 触发链路：SMAPI Control `GameLoop.Saved`（存档写盘完成后触发）→ 写 `.local-container/control/save-events/*.json` → 前端请求 `GET /api/instances/:id/saves/backups`（打开/刷新"存档"页）时后端顺带跑 `RunBackupMaintenance()` 消费事件 → 若 `policy.gameSaveBackups` 为真，为对应存档创建/覆盖 `auto_<save>_<游戏日序号六位>.zip` 并清理超出 `policy.retainGameDays` 的旧游戏日。
- 排序和保留完全基于**游戏内日期序号**（`(year-1)*112 + seasonIndex*28 + day`），不是现实创建时间；`GET /saves/backups` 返回的每个 `BackupInfo` 带 `gameDayOrdinal` 字段供前端直接排序，不需要前端自己实现季节序号映射。
- `GET|PUT /api/instances/:id/saves/backups/policy` 请求/响应体：`{ "gameSaveBackups": boolean, "retainGameDays": number }`（1-14，默认 5）。不再有 `scheduledBackups`/`scheduledHour`/`scheduledIntervalHours`/`dailySnapshots`/`dailyRetentionDays` 字段；旧客户端/旧 `policy.json` 传这些字段不会报错，只是被忽略。
- `GET /saves/backups` 返回的 `BackupInfo.kind` 取值：`auto`（游戏日自动回档点，主列表展示）、`manual`（手动备份/服务器页快捷备份/计划重启关闭前备份）、`predelete`（删除存档前保护备份）、`prerestore`（回档前保护备份）、`latest`/`daily`/`scheduled`（历史遗留，不再产生新文件，仅供查看/回档/删除）。
- 回档接口 `POST /api/instances/:id/saves/backups/restore` 请求体 `{ backupName, overwrite, autoRestart }`；覆盖已有同名存档时后端会先创建 `prerestore_*` 保护备份，失败则整体中止（`500 restore_failed`），不会破坏当前存档。
- 服务器运行/启动中时：`autoRestart` 缺省或为 `false` → 保持原有 `409 server_running`；`autoRestart: true`（`SAVE-RESTORE-AUTORESTART-1`）→ 后端把"停止服务器 → 回档 → 重新启动服务器"编排成一个 lifecycle job，返回 `202 { jobId }`，前端用现有的 job 轮询/SSE 机制跟踪进度，和点"启动服务器"按钮拿到的响应形状、跟踪方式完全一致；服务器已停止时无论 `autoRestart` 是什么值都走同步路径，返回 `200 { saveName }`。前端应据此按 `jobId`/`saveName` 二选一分支处理响应，不要假设固定返回哪一个字段。
- 删除存档接口 `DELETE /saves/:name`（`save_delete`）内部创建的保护备份现在是 `predelete_*` 前缀，响应体 `backupCreated` 字段语义不变。
- 验证：`go test ./internal/games/stardew_junimo ./internal/web`、`cd frontend; npx tsc --noEmit -p . && npm run build`。
# SCHEDULED-RESTART-1 计划重启联调契约

- `GET /api/instances/:id/restart-schedule` 返回 `{ schedule }`，字段包括 `enabled`、`shutdownTime`、`startupTime`、`timezone`、`warningMinutes`、`backupBeforeShutdown`、`skipIfPlayersOnline`、`nextShutdownAt`、`nextStartupAt`、`lastStatus`、`lastMessage`。
- `PUT /api/instances/:id/restart-schedule` 仅管理员可用，请求体使用同一组配置字段；时间格式为 `HH:MM`，时区默认 `Asia/Shanghai`。
- 后端后台调度器每 30 秒检查启用计划。关闭时间前通过现有喊话通道写 `.local-container/control/commands/*.json`；关闭时可调用现有存档备份能力，再提交 `Stop` 生命周期 job；开启时提交 `Start` 生命周期 job。
- 关闭前备份语义与快捷备份一致：只备份当前已经落盘的 active save，不强制保存游戏内尚未写盘的进度。
- 联调建议：把关闭时间设置到当前时间后 1-2 分钟，确认弹窗保存后返回 `nextShutdownAt`；服务器运行中确认提醒文件写入 control commands；到点后任务中心出现 stop job。开启时间设置到停止后 1-2 分钟，确认 start job 被提交。
# MODDEPS-2 联调契约

- `GET /api/instances/:id/mods` 的 `mods[].dependencies[]` 结构扩展为 `{ uniqueId, minimumVersion?, required, installed, enabled, installedVersion?, satisfied, status? }`。前端应以 `satisfied=false` 或 `status` 判断展示诊断，不要再只把它当作纯 manifest 声明。
- 常见 `status`：`satisfied`、`missing`、`disabled`、`version_mismatch`、`unknown_version`；可选依赖对应 `optional_missing`、`optional_disabled`、`optional_version_mismatch`、`optional_unknown_version`。可选依赖缺失默认不算硬失败。
- `GET /api/instances/:id/mods/nexus/search` 的 `results[]` 新增 `installedEnabled`。当 `installed=true` 且 `installedEnabled=false` 时，表示该 Nexus Mod 已在服务器安装，但当前激活存档没有启用；前端应提示“已安装但未启用”，并禁止重复安装。
- 搜索的安装匹配按当前激活存档计算。后端会读取 `GetActiveSaveName(dataDir)` 并用 `ListModsWithState` 合并 active/disabled 目录，确保禁用目录里的 Mod 仍能被 Nexus ID 匹配到。
- 验证：`cd backend; go test ./...`、`cd frontend; npm.cmd run build`。

# MODREL-1 联调契约

- `PUT /api/instances/:id/mods/:modId/sync-classification` 响应从单个 `{ folderName, syncKind, syncNote }` 升级为 `{ mods, syncKind }`。`mods[]` 是本次按依赖/同包关系被同步分类影响的 Mod，前端必须按返回列表批量更新。
- 同步分类没有方向性：设置 `client_required`、`server_only` 或 `unknown` 时，都包含同 Nexus 包成员、所有已安装必需前置依赖、前置的前置，以及依赖它的已安装下游。这样用户先点“待确认”再切回其它标签时，后置 Mod 不会停留在旧状态。
- `PUT /api/instances/:id/mods/:modId/enabled` 响应从单个 `{ folderName, enabled, saveName }` 升级为 `{ mods, enabled, saveName }`。启用时包含同包成员和必需前置，禁用时包含同包成员和依赖它的下游。
- 共享前置不随某个业务包禁用：例如启用 `[CP] Multiple Construction Orders` 会启用 `Multiple Construction Orders` 和 `Content Patcher`；禁用 `Multiple Construction Orders` 会禁用同包 `[CP]`，但不会禁用 `Content Patcher`，因为它可能仍被其他 Mod 使用。
- 前端不要自行复刻关系图算法；以后联动规则调整时以后端返回的 `mods[]` 为准。
- 验证：`cd backend; go test ./...`、`cd frontend; npm.cmd run build`。
# NEXUS-EXT-2 安装完成可见性与日志

- `mod_remote_install` / `mod_nexus_install` 新任务的安装进度日志应显示正常中文；旧任务历史日志如果已经以乱码入库，不做迁移。
- 前端订阅安装 job 的 `finished` 事件后，成功状态会切到“添加模组”页并刷新 `GET /mods`。后端会把本次导入的 Mod 标记为当前激活存档启用；联调时如果任务已完成但页面没看到，应先确认是否刷新到了添加页，以及 `mods/` / `mods-disabled/` 目录和当前存档 profile。
- 验证：`go test ./internal/games/stardew_junimo ./internal/web`、`npm.cmd run build`。
# NEXUS-REQ-1 联调约定

- `GET /api/instances/:id/mods/nexus/search` 的 `results[]` 可能包含 `requiredMods[]`。字段来自 Nexus GraphQL 的 `modRequirements.nexusRequirements`，用于前端搜索卡片提示缺少的 Nexus 前置 Mod。
- `requiredMods[]` 每项包含 `modId/name/notes/nexusUrl/installed/installedEnabled/installedFolderName/installedVersion`。前端可把 `installed=false` 的项渲染为“安装前置”，并跳转该前置 Mod 的 Nexus 文件页。
- 前端主 Mod 与前置 Mod 的扩展安装入口都统一追加 `tab=files&anxi_auto=1`；扩展捕获 ZIP 后仍调用 `POST /api/instances/:id/mods/remote/install`。
- Nexus 页面出现 “Additional files required” 弹窗时，扩展应自动点击弹窗内 `Download` 按钮继续，不要求用户手点。该动作只发生在扩展已开始捕获的上下文里。
- 验证：`cd backend; go test ./internal/games/stardew_junimo ./internal/web`、`cd frontend; npm.cmd run build`、扩展脚本 `node --check`。
# NEXUS-PREMIUM-2 联调约定

- 前端页面不再暴露“粘贴链接安装”按钮；`POST /api/instances/:id/mods/remote/install` 仍作为浏览器扩展提交 Nexus CDN 临时 ZIP 的接口保留。
- `GET /api/settings/nexus` 返回 `configured=false` 时，前端仅提示 Premium 用户配置 NexusKey，不展示会员安装按钮。
- `configured=true` 时，Nexus 搜索结果卡片显示 `N站会员专属安装`，点击后调用 `POST /api/instances/:id/mods/nexus/install`，请求体仍使用当前 `NexusModSearchResult` 字段映射。
- 普通 `一键安装` 不调用后端安装接口，继续跳转 Nexus 文件页并交给扩展获取 ZIP 链接。

# NEXUS-EXT-BATCH-1 联调约定

- 普通 Nexus `一键安装` 现在由面板页向浏览器扩展发送 `START_BATCH_INSTALL` 消息，不直接跳转 Nexus，也不直接调用后端安装接口。
- 扩展后台为每个目标打开非激活标签页，URL 会附加 `anxi_auto=1&anxi_auto_submit=1&anxi_batch=<batchId>&anxi_item=<itemId>`；Nexus content script 会把这些参数短期写入 `sessionStorage`，后续 Nexus 跳转即使丢失查询参数，捕获 ZIP 后仍自动提交到现有 `POST /api/instances/:id/mods/remote/install`。批量任务优先让面板页 `panel-bridge.js` 代表扩展发起同源请求，复用面板登录态与 Vite proxy；桥接不可达时才回退扩展 background 直连面板地址。
- 面板按钮百分比只表示“扩展 ZIP 提交流程”的完成度，不表示后端解压导入 job 的完成度。后端真实安装结果仍以 `mod_remote_install` 任务日志为准。
- 进度折算：扩展阶段单项 `opening=10`、`capturing=35`、`ready=65`、`posting=80`、`queued=90`；批量进度取所有目标平均值。`queued` 只表示面板已创建 job，前端拿到 `items[].jobId` 后继续轮询 `GET /api/jobs/:id`，所有 job `succeeded` 才显示 100%，任一 job `failed/canceled` 时整体失败并显示对应 Mod 名。没有任何 jobId 时才适用扩展提交超时。
- 兜底：若 item 没有 `jobId`，但刷新 `GET /api/instances/:id/mods` 后可通过 `nexusModId` 或 `originNexusModId` 匹配到该 Nexus modId，前端可视为该 item 已完成。
- `CAPTURE_URL` / `SUBMIT_CAPTURED_URL` 消息必须携带 `batchId/itemId/autoSubmit` 或可解析的 `captureKey=batch:item`。background 会用这些字段恢复 capture 的 batch 上下文，并在 `POST /mods/remote/install` 返回后把 `jobId` 写入对应 item。
- 面板可发送 `CLEAR_STATE` 给扩展桥接，用于清理扩展 batch/capture 和前端卡住的 session 状态；该操作只清浏览器进度，不删除已经安装到服务器的 Mod。
- 浏览器扩展新增 `panel-bridge.js`，只在当前页面 origin 等于扩展配置的 `panelBaseUrl` origin 时响应面板消息；正式部署时仍需保证同浏览器已登录面板管理员和 Nexus。
# NEXUS-EXT-BATCH-2 联调契约

- 扩展 batch 的 `done/failed` 在前端视为终态；后续 `GET_BATCH_STATUS` 只能用于补充仍在进行中的 batch，不得覆盖已经完成或失败的按钮状态。
- 搜索页安装进度的最终成功仍以面板 job 为准：所有关联 `mod_remote_install` job 均为 `succeeded` 才显示 100% 完成；任一 job `failed/canceled` 显示失败。没有 `jobId` 的异常情况才允许用 `GET /mods` 的 `nexusModId/originNexusModId` 命中做兜底。
- Nexus 多 Mod ZIP 的来源以 `stardew_junimo.SaveInstalledNexusMetadata` 写入的 sidecar 为准；如果扩展 batch 上下文和 ZIP 内唯一正数 `UpdateKeys: ["Nexus:<id>"]` 冲突，后端会用 ZIP 内声明纠偏。
- Ridgeside Village 验证点：`RidgesideVillage`、`[CC] Ridgeside Village`、`[FTM] Ridgeside Village` 应显示随 `Ridgeside Village [Content Patcher component] / Nexus:7286` 安装，不应显示随 `SpaceCore / Nexus:1348` 安装。
# NEXUS-EXT-BATCH-3 联调约定

- 浏览器扩展批量安装入口 `START_BATCH_INSTALL` 现在是幂等的：同一个 `batchId` 重复发送只返回已有 batch，不再重复打开 Nexus 后台页。
- 扩展会按 Nexus `modId` 对目标去重；缺少 `modId` 时按移除 `anxi_auto/anxi_auto_submit/anxi_batch/anxi_item` 后的 URL 去重。同一 Mod 同时作为前置和本体出现时，本体目标优先。
- 验证 Ridgeside Village 这类“本体 + 多前置”时，预期每个 Nexus modId 只打开一个后台下载页；如果仍看到重复页，先检查面板传入 targets 是否缺失 modId 或 URL 是否指向不同 Nexus Mod。
# NEXUS-EXT-CONNECT-1 联调约定

- 面板下载页通过 `window.postMessage` 向浏览器扩展桥接脚本发送 `PING`，payload 至少包含 `{ panelBaseUrl: window.location.origin, instanceId: "stardew" }`。
- `panel-bridge.js` 收到 `PING` 时允许绕过旧的 `panelBaseUrl` origin 校验，但会先调用当前面板页的 `GET /api/auth/me` 并要求返回已登录用户；验证成功后再向 background 发送 `REGISTER_PANEL`，由 background 保存 `panelBaseUrl` 和 `instanceId`。
- `PING` 成功返回 `{ ok: true, config, state }` 后，前端显示“扩展已连通”，普通 Nexus “一键安装”开放。失败时按钮保持可重试，普通一键安装禁用；Premium Key 直连安装不依赖扩展连通。
- 除 `PING` 外，`START_BATCH_INSTALL`、`GET_BATCH_STATUS`、`CLEAR_STATE` 和后台页提交仍要求当前页面 origin 与扩展配置的 `panelBaseUrl` 匹配，避免普通网页改写扩展安装目标。
- 联调时浏览器扩展更新后需要在扩展管理页重新加载，并刷新面板页，让新的 `panel-bridge.js` 注入当前标签页。
# NEXUS-EXT-PACK-1 联调契约

- 新增扩展包下载接口：`GET /api/instances/:id/mods/nexus/extension/download`。
- 请求要求已登录面板；响应为 `application/zip`，`Content-Disposition` 文件名固定为 `anxi-nexus-installer.zip`。
- 后端优先返回实例目录 `.local-container/browser-extensions/anxi-nexus-installer.zip` 中已有且合法的预打包 ZIP；不存在时优先复制镜像/仓库中的 `browser-extensions/anxi-nexus-installer.zip`，预包不存在或损坏时才兜底生成。合法性至少要求 ZIP 根目录包含 `manifest.json` 和 `background.js`。
- 复用是**版本感知**的：只有缓存 / 预包 ZIP 里 `manifest.json` 的版本与源码 `manifest.json` 版本完全一致才复用，否则从源码重新打包。升级扩展（bump manifest 版本）后用户重新下载即可拿到新版，无需手动清缓存。
- 前端 `下载浏览器扩展` 按钮只负责下载扩展包；安装后仍需用户在 Chrome/Edge 扩展管理页加载解压目录，再回面板点击 `检测扩展` 完成地址同步与连通校验。
- 验证：`cd backend; go test ./internal/games/stardew_junimo ./internal/web`；`cd frontend; npm.cmd run build`。
# NEWGAME-PLAYERLIMIT-1 自定义新存档人数上限联调契约

- `POST /api/instances/:id/saves/custom-new-game` 新增可选字段 `maxPlayers`，表示最大同时在线人数，合法范围 `1-100`；旧客户端不传时后端默认写入 `10`。
- `startingCabins` 仍表示初始联机小屋数量，范围 `0-7`；`maxPlayers` 是总在线人数上限，必须大于等于 `startingCabins + 1`。
- 后端会写 `server-settings.json`：`Server.MaxPlayers=<maxPlayers>`、`Server.ExistingCabinBehavior="KeepExisting"`，以及原有 `Game.StartingCabins` / `Game.CabinLayoutNearby`。`Server.CabinStrategy` 从 `2026-07-10` 起改由新字段 `cabinMode`（`recommended|vanilla`，默认 `recommended`）派生：`recommended → "CabinStack"`，`vanilla → "None"`，详见下方 `CABIN-STRATEGY-1` 契约。
- 联调验证建议：新建存档时分别提交 `startingCabins=7,maxPlayers=8` 与 `startingCabins=7,maxPlayers=16`，确认配置文件写入正确；提交 `startingCabins=7,maxPlayers=7` 应返回结构化错误。
# VNC-CONTROL-1 联调契约

- `GET /api/instances/:id/rendering`：管理员专用，服务器必须处于 `running`。用于刷新页面后读取 Junimo 当前服务端渲染状态，成功返回 `{ "fps": 0|N, "output"?: string }`。
- `POST /api/instances/:id/rendering`：管理员专用，服务器必须处于 `running`。请求体 `{ "fps": 15 }` 用于打开 Junimo 服务端渲染，`{ "fps": 0 }` 用于关闭，成功返回 `{ "fps": number, "output"?: string }`。
- 该接口由面板后端在 `server` 容器内代理 JunimoServer `POST /rendering?fps=...`，并按实例 `.env` 注入 `API_KEY`；代理请求会显式带 `Content-Length: 0` 以满足 Junimo 空 POST 要求。前端不得直连 Junimo `API_PORT`，也不得读取 API key。
- 服务器页 `跳转VNC控制` 通过已有 `GET /api/instances/:id/config/vnc-port` 读取宿主 VNC/noVNC 端口，并打开 `http://<当前面板hostname>:<vncPort>/`。VNC 密码只在 noVNC 页面中输入，不在面板前端回显。
- 联调顺序建议：启动服务器 -> 点击 `打开VNC显示` -> 点击 `跳转VNC控制` -> noVNC 页面出现后输入安装时配置的 VNC 密码。
- 验证：`go test ./internal/games/stardew_junimo -run Rendering`、`go test ./internal/web -run "Rendering|VNCConfig"`、`npm.cmd run build`。

# STEAM-QR-PHASE-CLASSIFY-1 联调契约

- 前端安装页继续以 `instance.driverPhase` 决定认证交互区：`steam_qr_required` 显示“Steam 手机扫码”和打开扫码窗口按钮，`steam_guard_mobile_required` 才显示“Steam Guard 验证 / 请打开 Steam 手机 App 批准登录”。
- 后端在用户选择扫码登录（`POST /api/instances/stardew/steam-guard/input`，`input="2"`，当前 phase 为 `auth_method_required`）后应保持 `driverPhase=steam_qr_required`，不应被上游日志 `Choice [1]: 2` 覆盖成 `steam_guard_mobile_required`。
- 前端安装页有防御性兜底：如果当前 `driverPhase=steam_guard_mobile_required`，但最近安装日志显示 `Choice [1]: 2` 或“已选择扫码登录”，且之后没有真正的 Steam Guard 菜单，则按 `steam_qr_required` 渲染 QR 区域。
- QR 弹窗契约：前端应从最新 `Or open: https://s.team/q/...` 日志行提取 Steam 登录 URL，并在本地生成标准二维码图片；终端字符画只能作为备用显示，不能作为主扫码源，也不能把最近多段 `[steam]` 日志直接塞进二维码窗口。
- 前端交互契约：管理员提交 Steam 认证选择后，页面应立即进入对应的本地乐观阶段，不等待后端 `driverPhase` 下一轮刷新。`auth_method_required + input=2` 立即显示 QR 等待；`steam_guard_choice_required + input=1/2` 分别立即显示手机批准等待/验证码输入框。若提交失败，前端回退并显示错误。
- 如果 QR 流程最终出现 `QR authentication failed: SteamClient did not connect...`，应进入 `qr_auth_failed` 或连接失败类状态；前端应提示 QR 登录失败/网络连接问题，而不是继续显示 Guard 手机批准。
- 联调网络判断：容器能解析 Steam 域名、连通 `api.steampowered.com:443` 与 Steam CM 端口，只说明 Docker 基础网络可用；SteamClient 仍可能因 CM 会话不稳定、地区网络或上游 QR 流程问题连接失败。
- 验证：`cd backend; go test ./internal/games/stardew_junimo -run "QRCodeChoice|SteamMobileApproval|SteamAuthMenus"`。
# STEAM-POST-AUTH-RETRY-1 联调契约
- Steam 认证成功后，任何游戏下载、Steam CDN、磁盘、SDK 或后续安装步骤失败，都不得再把用户引导回 Steam 账号密码输入。后端应使用 `state=error` 搭配 `driverPhase=download_failed` 或 `post_auth_failed`；不要把这类失败写成 `state=steam_auth_failed`。
- 安装页视觉状态可把 `[steam] [SteamAuth:A0] Logged in as`、`Token expires`、`Game license verified`、`Got depot decryption key`、`Downloading app 413150`、`Target directory: /data/game` 视为“认证已成功/已进入后续下载”的日志信号。注意这只是安装页展示与重试入口判断；持久 `STEAM_AUTH_COMPLETED` 只由真实 steam-auth 登录成功日志或非空邀请码写入。
- 只有真正凭据错误才使用 `credentials_required` 并要求重新输入账号密码；QR 登录未成功也可以提示用户改用账号密码。下载失败、CDN 403、manifest 失败、磁盘不足、后续容器步骤失败都不属于凭据错误。
- 验证建议：模拟日志顺序 `[SteamAuth:A0] Logged in as ... -> Downloading app 413150 -> Download failed: ...403`，实例最终应为 `error/download_failed`，前端按钮应为“重试下载（不重新输入账号）”，表单不出现 Steam 用户名/密码字段。
# PULL-PROGRESS-1 镜像拉取进度契约

- 安装 job 日志中的 `[pull:progress:done:total]` 是前端专用隐藏进度信号。
- `pull_running` 阶段的 `done/total` 表示 Junimo 镜像数量；`steamcmd_image_pulling` 阶段的 `done/total` 表示 SteamCMD 镜像 layer 数量。前端应展示为估算百分比，不要要求用户从 Docker layer 日志里猜进度。

# STEAMCMD-DOWNLOAD-PROGRESS-1 游戏文件进度契约

- SteamCMD 游戏文件下载进度不新增 API；前端从 job 日志中的 `[steamcmd] ... progress: N (done / total)` 解析百分比。
- `Success! App '413150' fully installed.` 是 Stardew Valley 游戏文件完成标记；`Success! App '1007' fully installed.` 仅表示 Steam SDK 运行文件完成。
- SteamCMD 手机 App 批准提示包括 `Please confirm the login in the Steam Mobile app` 和 `Waiting for confirmation`；批准超时属于 `steamcmd_failed`，不是安装成功。
# STEAMCMD-BRACKET-PROGRESS-1 兜底下载进度契约补充

- SteamCMD 兜底下载进度来源包括两类日志：`[steamcmd] ... progress: N (done / total)` 和 SteamCMD 原生 `[steamcmd] [ 28%] Downloading update (11,467 of 40,273 KB)...`。
- 前端应把上述两类日志都视为 `steamcmd_downloading`，并展示百分比与已下载/总大小；后端无需新增进度 API。
- SteamCMD 授权提示仍以日志和 `driverPhase` 双兜底：`Please confirm the login in the Steam Mobile app` / `Waiting for confirmation` 对应 `steamcmd_guard_mobile_required`。
# JUNIMO-IMAGE-CANDIDATES-1 联调契约

- 安装页看到 `driverPhase=pull_running` 时，后端可能正在拉取 `steam-auth-cn` 或 `JunimoServer` 候选镜像；日志前缀分别为 `[steam-auth:pull]`、`[server:pull]`，进度仍通过隐藏日志 `[pull:progress:done:total]` 给前端估算。
- 候选顺序为国内镜像源优先，然后 `ghcr.io`，最后原始仓库。单个候选失败会继续尝试下一项，不应立即把安装视为失败；只有全部候选失败时才显示 `pull_failed`。
- 成功命中的候选镜像会写回实例 `.env` 的 `STEAM_SERVICE_IMAGE` 或 `SERVER_IMAGE`，后续 compose / steam-auth TTY 均使用该选中镜像。
- 前端无需新增接口；继续展示 job 日志和 `pull_running` 进度即可。
# JUNIMO-IMAGE-CANDIDATES-2 安装页镜像候选联调

- 安装流程进入 Junimo 镜像检查时，后端会对 `steam-auth cn` 与 `server` 两类镜像分别展开默认候选源；旧 `.env` 中只有单候选值时也会被补齐。
- 前端日志应能看到 `server` 缺失时最多按 `(1/4)` 到 `(4/4)` 尝试：`docker.1ms.run/sdvd/server:<IMAGE_VERSION>`、`docker.m.daocloud.io/sdvd/server:<IMAGE_VERSION>`、`ghcr.io/sdvd/server:<IMAGE_VERSION>`、`sdvd/server:<IMAGE_VERSION>`。
- `steam-auth cn` 同理最多五个候选：`docker.1ms.run/anxiyizhi/junimo-steam-service-cn:1.5.0-anxi.2`、`crpi-9z3bkb9g7fxeohrg.cn-hangzhou.personal.cr.aliyuncs.com/anxi-panel/junimo-steam-service-cn:1.5.0-anxi.2`、`docker.m.daocloud.io/anxiyizhi/junimo-steam-service-cn:1.5.0-anxi.2`、`ghcr.io/anxiyizhi/junimo-steam-service-cn:1.5.0-anxi.2`、`anxiyizhi/junimo-steam-service-cn:1.5.0-anxi.2`。

# CABIN-STRATEGY-1 小屋策略分层联调契约

- `POST /api/instances/:id/saves/custom-new-game` 新增可选字段 `cabinMode`（`"recommended"|"vanilla"`），不传时默认 `"recommended"`。这是新建存档页给用户的简化二选一：`recommended` 对应 `Server.CabinStrategy="CabinStack"`（隐藏小屋堆叠，共用位置各自只看到自己的小屋），`vanilla` 对应 `Server.CabinStrategy="None"`（原版行为，小屋出现在真实农场地图位置，此时 `cabinLayout: "nearby"|"separate"` 才会在视觉上产生实际差异）。
- 新增 `GET /api/instances/:id/config/server-runtime-settings`：管理员专用，返回 `{ "cabinStrategy": "CabinStack"|"FarmhouseStack"|"None", "existingCabinBehavior": "KeepExisting"|"MoveToStack", "networkBroadcastPeriod": number }`。文件不存在或字段缺失时返回 `CabinStack`/`KeepExisting`/`1` 三个默认值，不报错。
- 新增 `PUT /api/instances/:id/config/server-runtime-settings`：管理员专用，请求体是同一个 `ServerRuntimeSettings` 结构（三个字段都必须传）。后端会校验 `cabinStrategy` 必须是三选一、`existingCabinBehavior` 必须是二选一、`networkBroadcastPeriod` 必须在 `1~10`，校验失败返回 `400 invalid_settings`。成功后只覆盖这三个字段，`server-settings.json` 里的 `MaxPlayers`、`AllowIpConnections` 等其它字段原样保留。
- 这两个接口是服务器控制页"小屋与联机高级设置"弹窗的完整版入口，和新建存档页的简化 `cabinMode` 共用同一份底层 `server-settings.json`，但**不是**同一层级：新建存档页只在建档瞬间写一次初始值，服务器控制页可以在存档已存在、服务器运行中或已停止的任何时候读写。两边都不会互相覆盖对方没有涉及的字段。
- **关键约束**：无论通过哪个入口修改，都只在 JunimoServer `server` 容器**下一次启动**时生效（和 `SERVER_PASSWORD`、`ExistingServerSettings` 同理），前端弹窗必须提示"需要重启服务器容器"，不能暗示实时生效。
- 联调验证建议：新建存档时分别提交 `cabinMode` 缺省、`"recommended"`、`"vanilla"`，确认 `server-settings.json` 的 `Server.CabinStrategy` 分别是 `CabinStack`/`CabinStack`/`None`；存档创建后调用 `PUT .../server-runtime-settings` 把 `cabinStrategy` 改成 `"FarmhouseStack"`、`networkBroadcastPeriod` 改成 `3`，重新 `GET` 确认改动生效且 `MaxPlayers` 未被覆盖；提交非法枚举值（如 `cabinStrategy: "Bogus"`）应返回 `400`。
- 命中本地任一候选时应直接显示“本地已有镜像 ... 直接使用”，不应先拉取排在前面的缺失候选。
# REAL-INSTANCE-CRITICAL-FLOWS-VERIFIED-1 真实实例联调结论

- 已完成真实实例联调：大存档启动→等待在线玩家列表出现主机；运行中回档→自动停止→完成回档→重新启动；多人待认证批准、踢出、封禁、回家；睡觉存档→生成游戏日回档点；Steam/SteamCMD 授权及镜像候选源失败后自动降级。
- 上述结果取代对应历史章节里的“待联调/未真机验证”标记，现有 API 契约不变。
# UI-LIFECYCLE-STATUS-1 联调契约（2026-07-11）

`GET /api/instances/{id}/state` 新增 `uiStatus`、`uiStatusUpdatedAt`、`statusSource`、`playersSource`。新前端以 `uiStatus` 为生命周期展示的唯一判定；字段缺失时才启用旧逻辑兼容回退。

同一响应的 `runtimeDiagnostic` 是只读排障摘要；Compose 服务快照继续复用 `GET /api/instances/{id}/docker/ps`，不放进高频 `/state` 轮询。
# PLAYER-ROSTER-SQLITE-1 联调说明

- `GET /api/instances/:id/players` 接口结构不变，前端无需迁移。
- `saveId` 会从控制模组可能提供的基础名归一化为完整存档目录名；调用方不应再自行按 `_数字` 截断。
- 离线玩家可能来自 `source=sqlite_roster`。该来源表示面板持久名册，不影响现有 `status=offline` 展示和管理动作使用的 `uniqueMultiplayerId`。
- `players.json` 仍是运行时在线快照输入；`players-cache.json` 与 `players-events.json` 升级后仅导入一次并退役。响应中的 `recentEvents` 字段结构不变，但数据源已经是 SQLite。
# FE-PLAYER-LOCATION-NORMALIZE-1 联调说明

- 后端与 SQLite 的 `location`、`locationName`、`locationDisplayName` 保持原始值，不改变接口契约；例如 `FarmHouseeb266bf0-3eb0-4174-b9b7-f22a893a70bd` 会原样存储。
- 前端共享格式化层负责把内部实例名归一化为逻辑位置并展示中文，坐标仍使用 `tileX` / `tileY`。不要在后端截断 UUID，否则会损失诊断和区分内部位置实例所需的信息。
# FE-LIFECYCLE-LIVE-SIGNAL-PRIORITY-1 联调判定

- 生命周期按钮的最终前端优先级：停止提交/停止 phase > 在线玩家列表主机在线 > 后端 `uiStatus` > lifecycle job/state 兜底。
- `isHost && status==='online'` 出现后必须立即结束“启动中”，即使邀请码尚未生成、lifecycle job 尚在等待后台探测或实例状态响应里的 `uiStatus` 尚未刷新。
- 用户确认停止后必须立即展示“停止中”，不等待后端下一轮 `/state` 返回 `stopping`。
# 命令结果回执协议 v1（阶段 1）

- 提交型控制接口继续立即返回，不阻塞等待游戏进程。对支持 `status.json.commandResultVersion >= 1` 的实例，响应会包含 `commandId` 与 `status: "queued"`；旧实例保留原“已提交”响应语义，`commandId` 仍可用于审计，但不承诺产生回执。
- 查询：`GET /api/instances/:id/commands/:commandId`，响应状态只会是 `queued/running/succeeded/failed/dispatched/expired/unknown`。`dispatched` 表示控制模组已把既有行为派发到游戏逻辑，不等价于玩家操作精确成功；阶段 1 不改前端按钮交互。
- 结果 JSON：`{ commandId, status, errorCode?, message?, createdAt, updatedAt, details? }`。客户端必须读取 `status/errorCode`，不得从 `message` 或 SMAPI 英文日志推断成功失败。
- `unknown` 是终态歧义提示，不会自动重试。典型场景是命令已开始或已经执行、但控制模组在写终态回执前崩溃；为避免重复执行，结果闸门存在时模组不会再次消费同 ID 命令。
- 兼容策略：旧模组无 `commandResultVersion` 时提交行为不变；查询不到结果时返回 `queued`（命令文件仍在）或 `unknown`（无可靠证据），不会伪造成功。
# 三条玩家命令精确结果契约

- `POST .../players/warp-home|kick|approve-auth` 对新版控制模组仍立即返回 `{commandId,status:"queued",output,...}`；前端随后调用 `GET /api/instances/:id/commands/:commandId`，500ms 一次、最多 10 秒。
- 三条命令的终态现在是 `succeeded/ok` 或 `failed/<结构化错误码>`；玩家提交后立即离线返回 `failed/player_not_online`。若执行与终态回执之间崩溃则按 v1 协议返回 unknown，客户端只显示未确认且不重试。
- 前端不得读取 `status.json.message` 或英文 `CommandOutcome.message` 决定结果，中文展示只根据 `status/errorCode`。旧模组无能力标志时维持 fire-and-forget “指令已提交”。
- 本阶段不改变 ban、broadcast、event、joja 的 dispatched 行为。
# broadcast/say 与 ban 回执契约

- `POST /commands/say` 对新版模组返回 queued 后轮询结果；`succeeded/ok` 只表示游戏聊天发送 API 已接受调用，不表示所有客户端已收到。结构化失败码：`empty_message`、`world_not_ready`、`chat_unavailable`、`broadcast_failed`。
- `POST /players/ban` payload 链路现在携带 `uniqueMultiplayerId`。终态可能是：直接 `Game1.server.ban(id)` 调用返回后的 `succeeded/ok`；只能模拟 Junimo `!ban` 时的 `dispatched/ok`；或结构化 failed。客户端不得把 dispatched 显示成已封禁。
- unknown、expired、查询异常和 10 秒超时统一显示未确认且不重试。旧模组无 v1 能力时继续显示“指令已提交”。event、joja、save-now 未接入本阶段。
- 已确认运行限制：封禁名单随服务器容器重启丢失；API 不提供封禁名单持久化或解封接口。

# trigger-event / enable-joja / save-now 回执契约

- `POST /festival/event`、`POST /joja/enable` 和新增 `POST /saves/save-now` 都立即返回 `{commandId,status:"queued",...}`，HTTP 不等待游戏。客户端通过既有 `GET /api/instances/:id/commands/:commandId` 查询，不读取 `status.json.message`。
- `trigger-event` 的世界/节日条件可明确判断时 failed；模拟 `!event` 成功只保证命令交给 Junimo，返回 dispatched，不等价于“节日已触发”。
- `enable-joja` 的不可逆确认和权限检查不变。admin 提升证据进入 payload；提升失败为 failed，聊天派发为 dispatched。只有持久 `JojaMember` 状态可见时才 succeeded。unknown/dispatched 不自动重试。
- `save-now` 写 running 后删除命令文件并在内存 tracker 保留 commandId；同一次后续 `GameLoop.Saved` 原子更新同一结果为 succeeded。两分钟超时为 `failed/save_timeout`；崩溃丢失关联则最终 unknown。ZIP 备份不会完成该 commandId。
- 前端 event/joja 每 500ms 轮询最多 10 秒；save-now 每 500ms 最多 125 秒。超时/unknown 只提示无法确认。旧模组无能力标志时维持“指令已提交”。
- 实例验证：`trigger-event=failed/no_festival_today`；`enable-joja=dispatched/ok` 且 Junimo 日志确认命令解析；`save-now` 的 `c1178eb65b034c96814416dc04c101f9` 由 running 经 `GameLoop.Saved` 转 succeeded。

# COMMAND-RESULT-PRODUCTIZATION-1 文件交接、SQLite 与保证边界

- 提交响应仍不阻塞等待模组；Web 在拿到 commandId 后立即写 queued（旧模组写兼容 dispatched/resultSupported=false）。模组原子写结果，后台幂等 UPSERT SQLite，提交与结果可任意先后到达。
- 安全删除顺序固定为“SQLite 事务成功 → 删除结果文件”。入库失败或文件仍是 active running 闸门时保留文件；同 commandId 重复扫描不会生成重复记录或重复最终审计。
- 查询顺序为同步可见结果文件、读取 SQLite、最后才使用 driver 队列推断；不得把 status.json 消息当单命令回执。unknown/failed 均不触发自动重试。
- 崩溃窗口不变：游戏效果已经发生、终态文件尚未原子写成时，只能最终 unknown。SQLite 持久化提高可查询性，但不把这个窗口伪装成成功。
- 保证等级：warp-home/kick/approve-auth 与 direct ban 能确认调用效果；broadcast 只精确保证交给聊天系统；event/Joja 聊天路径和 ban 名字降级仅 dispatched；save-now 只有关联 Saved 才 succeeded。所有命令在进程/磁盘异常窗口都可能 unknown。
- 真实完整链路已用隔离面板数据库连接运行实例验证：`say` 返回 queued，控制模组写 succeeded/ok，后台导入 SQLite 并关联临时 actor，历史 API 可见完成时间，结果文件在入库后删除。commandId 为 `64a0853e85c997d6b14ad6af48805f29`；测试广播正文不写入 SQLite。
# SAVE-BACKUPS-EMPTY-LIST-1 空备份列表联调契约

- 全新服务器尚无任何备份时，`GET /api/instances/:id/saves/backups` 的 `backups` 字段固定为 JSON 数组 `[]`，不得为 `null`。
- 前端同时兼容历史 `null` 响应并降级为空数组，防止动态加载的存档页运行时崩溃。
- 联调验证重点：完成游戏安装但尚未创建存档/备份时，管理员可正常进入存档页并使用“新建存档”。
# INSTALL-RUNTIME-VERIFICATION-1 安装完成状态联调契约

- 前端将 `game_installed`、`save_required` 等状态视为可创建/选择存档的前提；后端现保证这些状态只能在 Docker `game-data` 卷完整包含 Stardew、SMAPI 和 Steam SDK 所需文件时出现。
- 后端发现缺文件时返回实例 `state=error`、`driverPhase=install_verification_failed`、中文状态消息“游戏运行文件不完整，请重新安装或修复。”；前端沿用既有安装失败/重试入口，不应继续跳转或提示创建游戏。
- 联调验证：在新服务器故意保留仅 `steamapps/` 与 `.steam-sdk/` 空目录的卷时，安装任务必须失败；旧实例刷新状态后必须从“安装完成”切到可重试安装错误；授权登录完成后安装状态保持原值。
# JUNIMO-STACK-UPDATE-1 阶段三 apply 联调契约（2026-07-13）

- `POST /api/instances/:id/junimo-update/apply`：仅管理员；请求必须严格为 `{"confirm":true}`；返回 202。拒绝任意 image/tag/digest/registry/service/Compose/shell 字段、未成功的当前 dry-run、相同推荐版本、unsupported 配置和并发任务。
- `GET /api/instances/:id/junimo-update/apply`：仅管理员；返回最近持久状态：`applyId/jobId/phase/progress/current/target/selected/checks/warnings/logs/serverWasRunning/serverRunning/errorCode/error/manualAction/startedAt/updatedAt/finishedAt`。不返回密码、refresh token、完整 env 或原始 Docker/registry stderr。
- 阶段为 `checking/pulling/backing_up/stopping/writing_config/recreating_auth/verifying_auth/recreating_server/verifying_server/restoring_state/succeeded/rolling_back/failed_rolled_back/rollback_failed`。刷新从 GET 恢复；`rollback_failed` 只能转人工处理。
- apply 与 Panel `/api/system/update` 独立；目标只取 `stardew_junimo/config` embed 清单。隔离 Docker integration test 只创建 `anxijunimotest*` 临时卷，不接触生产 Compose project 或 volume。
# GAME-RUNTIME-VERSION-1 联调契约（2026-07-14）

- `GET /api/instances/:id/runtime-components`（管理员）：返回 `{available,supported,status,code,reason,current:{game,sdk},recommended:{game,sdk,tested,releaseNotes},detectedAt}`。组件包含 `appId/buildId/stateFlags/installDir/lastUpdated/manifestPath/status/code/reason`；推荐项包含 `appId/buildId/manifestVersion/notes/estimatedDownloadBytes`。
- 整体状态只有 `up_to_date/update_available/game_missing/sdk_missing/manifest_invalid/custom_or_unknown`。只有两份 manifest 合法、StateFlags 含完整安装位、推荐矩阵 tested 且任一 buildid 不精确匹配时才是 `update_available`；不比较 buildid 数值大小。未安装实例为 `game_missing`、`code=not_installed`、HTTP 200。
- `GET|POST /api/instances/:id/runtime-components/dry-run`（管理员）：POST 只能空 body/严格 `{}`；响应为 `{phase,progress,target,checks,warnings,requiredBytes,freeBytes,errorCode,error,updatedAt}`。本阶段只读且同步完成，GET 恢复最近 0600 状态。
- 浏览器不得提交 appid/buildid/path/volume/image/command。后端固定读取 `steamapps/appmanifest_413150.acf` 与 `.steam-sdk/steamapps/appmanifest_1007.acf`，响应/诊断不得包含完整 ACF、Steam username/password/token/ticket。

## SMAPI 更新接口（2026-07-14）

- `GET /api/instances/:id/runtime-components` 在原响应上可选追加 `smapi`；`GET /api/instances/:id/smapi-update` 返回 `{available,supported,status,code,reason,current,recommended,detectedAt}`。`current.version` 来自实际 DLL 元数据，不信任 `.env`。
- `recommended` 固定 `{version,downloadUrl,sha256,archiveBytes,compatibility}`；compatibility 包含 game/sdk buildid、Junimo/auth tag、Control 版本/DLL SHA 和 commandResultVersion。它来自 Panel embed 的 tested 清单，不是运行时 latest。
- `POST /smapi-update/dry-run` 只允许空 body/严格 `{}`；GET 恢复最近状态。`POST /smapi-update/apply` 只允许严格 `{"confirm":true}`，GET 轮询持久状态。任何 target/url/version/sha/zip/shell/image/service 字段均以 400 拒绝。
- dry-run/apply 状态共用 `{updateId,jobId,phase,progress,current,target,checks,warnings,logs,serverWasRunning,requiredBytes,freeBytes,errorCode,error,manualAction,startedAt,updatedAt,finishedAt}`；`requiredBytes/freeBytes` 来自当前 game-data 只读容量与清单上限。apply 的终态为 `succeeded/failed_rolled_back/rollback_failed`，后者禁止自动重试并保留恢复材料。
- `GET /runtime-components` 的顶层 `smapi` 与独立 `GET /smapi-update` 契约一致；均为管理员只读响应，不返回完整安装器/manifest、Steam 凭据、token、ticket 或 recovery 内容。
- apply 阶段为 `checking/downloading/validating_archive/creating_staging/cloning/installing/verifying_staging/stopping/switching/starting/verifying_stack/restoring_state/succeeded/rolling_back/failed_rolled_back/rollback_failed`。日志、checks 和错误必须先脱敏，不能返回 token、密码或 app ticket。
- 完整同步包 `pack-manifest.json.smapi` 始终记录推荐 version/SHA 与准确 bundled；只有 bundled=true 才有 installerFile 和 `payload/smapi`。`mods_update` 包仍记录兼容要求，但 bundled=false、installerFile 空且无 SMAPI payload。

## 2026-07-14：跨仓库兼容矩阵发布列车

简化发布流程：维护者确认 Junimo server 精确版本 -> 指定对应的 steam-auth-cn 版本 -> 在当前 Panel 的 `runtime_stack_manifest.json` 中直接写入 server/auth、game/SDK、SMAPI/Control 的目标版本和校验值 -> 本机及 CI 测试 -> 创建 Panel tag。用户升级到该 Panel 版本后，Panel 比较当前实例与内嵌清单并提示相应组件升级。不再创建 candidate 文件、不做 tested/recommended 晋级，也不需要 GitHub Environment 审批变量。

Panel 不接收 steam-auth-cn 的 `repository_dispatch`，也不根据 auth 发布自动生成分支或 PR。auth 版本只是维护者为指定 Junimo server 选择的配套组件，不能单独推动 Panel 发布。内嵌清单中 server/auth 任一精确 tag、digest 或 auth 源码溯源缺失都会被 CI 拒绝。

升级事务保持拆分：Junimo server/auth 共享认证卷快照并成对回滚；游戏/SDK 与 SMAPI 使用显式 GAME_DATA_VOLUME 的 staging/切换/回滚。上游“停止服务、删除当前 game-data 卷、重启后自动下载”只适用于无事务的重装，不被 Panel 复用；Panel 只复用既有容器下载/官方安装器能力，绝不删除当前唯一卷后再尝试恢复。

真实验收分层：维护事务基线至少包含全新安装、旧 recommended 到 candidate、steam-session 保留、server/auth 成对回滚、game/SDK 与 SMAPI staging 回滚、两个 Mod 加载、Junimo `/health`、auth `/steam/ready` schema 可解析、建档/读档、status.json/players.json、commandResultVersion、关键命令回执、运行状态恢复、Panel 重启恢复和支持包脱敏。Steam 登录、`has_ticket=true` 与邀请码属于可选在线模式专项验收，不是 LAN-only 实例升级/回滚的硬门槛。
# 2026-07-14 更新状态语义加固

- `POST /smapi-update/apply` 返回及后续 GET 状态中的 `serverWasRunning` 现在表示任务开始时 Docker Compose 的真实 server 状态，不再复述数据库 instance state。Docker 状态不可读取时 POST 拒绝启动并返回 `runtime_state_unavailable`。
- `failed_rolled_back` 现在保证旧栈已通过完整运行验收；如旧栈启动、auth ticket、Junimo/Control、状态文件或邀请链路任一失败，终态为 `rollback_failed`，前端必须继续展示人工恢复提示，不能将其当作已安全回滚。
# 2026-07-14：运行镜像元数据与 auth ready 契约修复

- Junimo/auth dry-run、apply、rollback 读取镜像和容器事实时，只消费 Docker 格式化输出的安全字段，不读取或返回完整容器环境。
- `/steam/ready` 验收不再依赖 steam-auth 镜像携带 Node.js。当前 .NET steam-auth-cn 镜像通过容器内 Bash HTTP 探针返回 JSON；维护事务只要求 HTTP 可达、JSON 合法且 `ready`/`has_ticket` 两个布尔字段存在。字段为 false 表示 Steam 在线能力当前不可用，但不阻止 LAN-only 升级；端点不可达、JSON 非法或字段缺失仍失败。

# 2026-07-16：维护事务与 Steam 在线能力解耦

- Junimo server/auth apply 与 rollback 的成功契约不再包含登录成功、app ticket 或邀请码。`checks[].name=steam_auth_ready` 保持兼容，但其含义调整为 Auth 容器/镜像/服务接口通过；未登录时在 `warnings[]` 返回非阻断说明。
- `junimo_runtime` 与 SMAPI `full_stack` 检查不再主动获取邀请码；邀请码端点和前端展示契约没有删除，用户需要 Steam 联机时仍可独立登录、刷新和排障。
- 前端无需新增字段或迁移状态结构；现有阶段、checks、warnings 与终态枚举不变。
- API 请求体、响应 JSON 和前端字段不变。
# JUNIMO-UPDATE-PROGRESS-1 联调契约（2026-07-14）

- Junimo dry-run/apply 响应可选新增 `download: { component, image, doneLayers, totalLayers, percent }`；旧状态没有该字段时前端继续显示阶段级进度。
- apply 响应可选新增 `causeCode/causeError` 与 `rollbackCode/rollbackError`。前端在 `rollback_failed` 同时展示初始失败和恢复失败，不能用 rollback 错误覆盖 cause。
- 用户点击维护卡片“立即升级”后，前端必须重新发起一次 dry-run；成功后自动发起 apply。POST 仍不接收版本、镜像或 digest，目标继续由后端内嵌矩阵决定。
- `rollback_failed` 仍是安全锁：本次仅改善可见性，不允许前端自动清理状态、删除恢复材料或再次执行升级。
# COMPONENT-UPDATE-GENERATION-1 一键升级串行契约（2026-07-14）

- 前端 POST Junimo/SMAPI dry-run 后必须保存本次响应的 `dryRunId`/`updateId`；历史成功状态不得作为新点击的 apply 前置条件。
- 只有轮询返回的成功状态 ID 与本次点击 ID 完全一致时才 POST apply。apply POST 在新 dry-run 尚未完成时出现属于前端编排回归，后端 `runtime_update_busy` 门禁继续拒绝该请求。
- 展示层按工作流 `startedAt` 选择最新代际，历史终态可保留用于技术详情，但不得覆盖正在执行的新预检或新 apply。

## RUNTIME-FARM-CATALOG-1 联调契约（2026-07-15）

- 后端启动新建事务前写 `farm-catalog-request.json`：`schemaVersion=1`，`requestId` 与 `transactionId` 必须相等，并包含 requestedFarmType/generatedAt/expiresAt。控制 Mod 仅对未过期请求回写身份。
- 控制 Mod 0.2.0 写 `options.json` schema 2：`source=smapi-runtime`、双 ID、generatedAt、controlModVersion、loadedMods、modFingerprint、farmTypes。`farmTypes` 是本次 `DataLoader.AdditionalFarms` 的运行时事实，不是离线扫描结果。
- 后端在 `/newgame` 前校验 options 文件上限 2 MiB、事务 ID、时间、指纹和目标 FarmType；失败不会探测或调用 `/newgame`。模组目标缺失使用 `farm_type_not_loaded`。
- `status.json` 增加 `newGameTransactionId`、`requestedFarmType`、`resolvedFarmType`、`farmTypeResolved`、`catalogGenerated` 和 `newGameWarning`，用于诊断而非替代 options 门禁。
- 新控制组件不会在任意 `SaveLoaded` 删除 pending marker。marker commit/rollback 所有权属于后端事务。
- 兼容：官方农场可在旧控制组件上继续阶段 5 流程；模组农场必须使用 schema 2 且 requestId fresh。当前前端没有模组创建动作，custom-new-game 仍拒绝模组 FarmType。

验证：C# contract tests、Go 全量测试、前端 production build 已通过。后续已在独立临时 Compose project 和独立 volumes 中完成真实 SVE fresh catalog：matching transactionId/fingerprint 的刷新后 catalog 包含 `FrontierFarm`；早期尚未注入 AdditionalFarms 的 catalog 只触发继续等待，绝不作为通过依据。

## MOD-FARM-CREATE-1 联调契约（2026-07-15）

- `GET farm-types` 返回 `moddedCreationEnabled`；前端必须同时服从服务端 `selectable`。关闭时 custom-new-game 返回 `modded_farm_creation_disabled`。
- 自 `MOD-FARM-DEFAULT-ON-1` 起服务端缺省为 `moddedCreationEnabled=true`；显式 `ENABLE_MODDED_FARM_CREATION=false` 时仍返回 false。默认变化不改变 `selectable`、依赖闭包、运行时目录和最终 XML 验证契约。
- custom 固定顺序：唯一 provider/依赖闭包 → 事务快照 → 精确 Mod 集合 → fresh runtime catalog/requestId/fingerprint/ID → 单次 `/newgame` → XML `<whichFarm>` 匹配 → 原子 save profile → commit。
- 隔离 E2E 已完整走通上述顺序。结果 XML 为 `<whichFarm>FrontierFarm</whichFarm>`；容器重启以及 `FrontierFarm → Standard → FrontierFarm` 双向切档均成功，Standard profile 禁用第三方 Mod，切回后 Frontier 必需依赖恢复。既有实例未操作；该阶段 feature gate 默认关闭，现已由 `MOD-FARM-DEFAULT-ON-1` 改为默认开启。

### 发布前兼容与故障注入补充（2026-07-15）

- 创建事实顺序进一步明确为：启动前目录快照 → fresh runtime catalog → API ready → 检查 Junimo 启动期是否已产生唯一新目录 → 必要时最多一次 POST `/newgame` → 唯一目录稳定/XML → profile commit。启动期已生成时 POST 次数为 0；多个目录为 ambiguous，绝不猜测。
- 离线目录、manifest、Content Patcher 静态解析和旧 options 都不能替代 fresh runtime catalog；fresh catalog 也不能替代最终 XML。C# 动态注入或运行中注册的农场只要最终出现在本次 `DataLoader.AdditionalFarms(Game1.content)` 且 fingerprint/ID 匹配即可通过，不要求离线解析器完整理解其注入实现；但显式选择仍需一个可解析的已安装 provider/依赖集合。
- `modded` 仅为兼容高级值：运行时只有一个真正模组农场时才可解析；多个时选择受加载顺序影响，不稳定，UI 不推荐并要求显式 ID。
- 导入 custom save 会按 XML ID 重建精确 profile；官方导入仍禁用第三方 Mod。真实 SVE 1.15.11 已通过创建、重启、Meadowlands 往返、备份、覆盖恢复、导出、删除后重新导入与依赖恢复。
- 错误覆盖 `farm_type_not_installed`、`farm_dependencies_missing`、`farm_type_not_loaded`、`farm_catalog_stale`、`farm_type_mismatch`、`new_game_result_unknown`、`new_game_rollback_failed`、`mod_profile_commit_failed`。保存响应新增 `farmTypeLabel`。
# MOD-BUNDLE-RUNTIME-COMPAT-1 联调契约（2026-07-16）

- `POST /api/instances/:id/mods/upload` 的 `upload.discoveredCount` 表示所有上传 ZIP 中有效 manifest 数；`importedCount/enabledCount` 表示实际写入和启用数；`skippedBuiltInCount/skippedBuiltInNames` 表示由 SMAPI 运行时提供、因此未重复安装的组件。
- `GET /api/instances/:id/mods` 与上传响应均可携带 `compatibilityWarnings[]`，当前代码 `existing_save_world_overhaul_not_rebuilt` 附带 `title/message/saveName`。该数组为提示，不把已安装 Mod 标为失败，也不授权客户端改写存档。
- 无来源 Mod 继续返回并展示；ZIP 中各 manifest 仍是独立 Mod，原有物理目录分组、同目录联动删除、Nexus ID 继承和 `[CP]` 名称均保持。
# 2026-07-16：Panel → required Junimo 运行栈自动协调

- runtime manifest/API 推荐对象新增 `runtimeUpdatePolicy`。值为 `required` 时，新 Panel 启动后对已安装默认实例自动串联 `POST junimo-update/dry-run` 与 apply 的同等服务层能力，但不伪造 HTTP 请求，也不绕过 driver/job/锁/恢复状态机。
- 跨版本顺序固定：旧 Panel 只完成自身容器替换；新 Panel 健康启动后读取自己的内嵌清单并自动把旧 121 升到 125。用户对 Panel 的一次升级操作不再产生第二个确认弹窗。新安装、已经 125、custom/invalid、rollback_failed 与 interrupted apply 分别走不适用、直接成功、人工处理和先恢复后协调分支。
- `required-status.json` 只保存协调阶段和脱敏错误；详细进度仍来自既有 dry-run/apply API。未达到 required pair 时生命周期启动返回 `409 required_runtime_update`；停止、诊断和恢复仍可用。

# 2026-07-16：服务器游戏语言契约

- `GET|PUT /api/instances/:id/config/game-language` 仅管理员可用，请求/响应为 `{languageCode}`，合法值为 `zh/en/de/es/pt/ru/ja/it/fr/ko/tr/hu`。
- 保存成功表示宿主配置已写入；运行中的游戏不会热切换。前端按实例状态提示下次启动生效，或调用现有 restart API。
- 后端每次服务器启动前再次同步；新实例默认中文，升级实例首次接管保留已有合法游戏语言。
# SAVE-IMPORT-E2E-RELEASE-1 isolated integration evidence (2026-07-17)

- Isolated projects `save-import-e2e-release*`, dedicated game/Steam volumes and non-production ports were used. Source ZIP SHA-256 values were retained; no production deployment, release, push or publish occurred.
- Retained test ZIP hashes: takeover `20303be32a9dd51432d9786620a52346ee6d7a092510896aeebd6aabf46ad4c6d`, first swap `ab90d59373cabfad959f5bce546b86ba25df376379e389571903cfa6069ee0b1`, renamed SaveGame.Save experiment `74479b90e079b9cca07702a2ad7f29e51943796505efbaca3e04cb128c44f3cb`, and accepted SaveGameMenu run `b36504916442b876b4edfcc68b5ab6ea3791fc13f90c23ab83c3255d641ae0e4`.
- The first startup briefly inherited Docker's default `stardew` compose project before an explicit isolated `COMPOSE_PROJECT_NAME` was added. It was stopped before any import submission; the historical Steam-session volume may have been mounted during that short window and was not modified or cleaned afterward. All accepted evidence runs used explicit isolated projects.
- Takeover/as-is completed with unchanged target hash, no pending and target runtime saveId, then survived a second restart. Swap completed with Phase A composite evidence, finalizer count/virtual-host evidence, matching Control commandId Saved, transition=true, changed stable XML hash, running-state promotion and a second restart retaining the same hash/saveId with pending absent.
- Upstream still has no commandId. Panel uses disk transaction traces, pending, saveId, finalizeCount, `GameLoop.Saved` and dayTransitionComplete as a composite adapter; logs and pointer-only changes remain non-authoritative.
- Human semantic/game-client and full fault-injection matrices are not complete, so the umbrella release gate remains open.
- Two finalized-but-unsaved negative-test runtimes are intentionally still running under recovery protection; the accepted completed runtime is also running. They must not be stopped as routine cleanup until their evidence is handed off or an authorized recovery decision is made.

## Local downloaded-game/rich-save run (2026-07-17)

- A copied local save with two farmhands, three cabins, furniture, fridge content and cellar assignments was uploaded through multipart preview and the 202 commit endpoint. Missing host handling and unacknowledged takeover both returned `host_decision_required`; acknowledged takeover returned a dedicated job/operation and completed with unchanged as-is hash.
- A fresh scaffolded database first failed before maintenance because it was not yet `stopped`; the unsubmitted transaction was cleaned with `CleanupUnsubmittedImport`, its preimport was retained, and the formal Stop API normalized state before retry.
- The run exposed and fixed two timing/safety gaps: command registration can precede diagnostics baseline readiness, so baseline capture now polls within the same deadline; and a missing original pointer is rejected before ComposeUp so upstream cannot enter new-game creation. Neither path sent `saves import` before its evidence gate.
- After completion, a real restart reloaded `1111_442923526`; XML/hash, `Pending=null`, diagnostics, runtime cabin/farmhand counts and day-transition state remained valid. Human client role selection, reconnect/sleep and spouse/child/pet semantics remain open.
# IMAGE-CLEANUP-1：Panel apply 状态兼容字段（2026-07-17）

- `GET /api/system/update/apply` 的既有状态对象新增可选 `cleanupCompleted: true`，表示当前版本 helper 或新 Panel 跨版本收尾已经尝试完成旧镜像清理。旧后端缺字段、活动阶段和旧历史记录均按 false/缺失兼容；前端不依赖该字段判断升级成功。
- `phase=succeeded` 仍只由新 Panel 三项健康验收决定。旧镜像因共享容器、tag 漂移或 Docker 错误无法删除时，phase 不回退，详情 logs 增加 warning；失败、`failed_rolled_back`、`rollback_failed` 不执行清理。
- 本次没有新增请求体、按钮或轮询接口，现有离线重连与结果弹窗保持不变。
# PANEL-POLL-LEAK-1 联调契约（2026-07-18）

- `GET /api/instances/:id/invite-code` 成功仍为 `200 {"inviteCode":"..."}`；文件存在但内容为空时现在明确返回 `{"inviteCode":"n/a"}`。接口不会启动 `attach-cli`。前端必须把 `n/a` 当作暂未就绪，而不是可复制的邀请码。
- 邀请码与指标 GET 均允许多页面并发；服务端按实例使用 5 秒缓存和 singleflight。客户端仍应在 `document.visibilityState !== "visible"` 时停止玩家、邀请码和指标轮询，不能把服务端缓存当成隐藏页继续轮询的理由。
- `POST /api/instances/:id/restart` 在已有重启 job queued/running 时返回 `409`，错误码 `restart_in_progress`；原 job 保持运行。客户端应展示服务端中文错误并等待任务终态，不得自动重试。
- Docker Desktop 隔离真机使用真实 Compose exec/stats 验证 12 路并发共享；测试容器已清理，未操作或启动既有存档导入证据实例。
# RUNTIME-COLD-START-1 联调说明（2026-07-19，completed）

- Junimo 运行组件升级的 API 路径、请求体和 phase 枚举不变。低资源 Docker 主机会在 apply `warnings` 增加 CPU、内存、最长冷启动等待及 swap/swappiness 建议；前端应按普通 warning 展示，不能把它当成失败或要求用户重复提交 apply。
- `verifying_server` 最长可持续 20 分钟。客户端继续轮询同一个 applyId；不得因 5 分钟无终态自行触发第二次 dry-run/apply，也不得把 Docker 单次超时解释为 `rollback_failed`。
- `rolling_back` 期间 stop 可在 10 分钟内幂等重试。只有服务端写出 `failed_rolled_back`、`rollback_failed` 或 `succeeded` 才是终态；页面刷新/断线恢复逻辑不变。
# CONTROL-PAUSE-FEEDBACK-1 联调说明（2026-07-19，completed）

- HTTP 路径、请求体、apply phase 和终态枚举不变。server/auth 镜像已匹配但运行时 `options.json.controlModVersion` 旧于推荐 Control 时，运行栈检查现在返回 `status=update_available`、`code=control_update_available`。
- 前端继续使用现有 Junimo required-update 流程展示并跟踪同一个 applyId；不要因为镜像 tag 看起来未变化就隐藏更新或跳过 apply。该事务会同步内嵌 Control 并受控重启游戏进程，使新 DLL 真正加载。
- Control 0.2.2 不再参与任何有连接玩家的菜单暂停判断。`/status.isPaused` 仍是游戏真实状态；页面无需增加新的暂停字段或自行推断菜单请求。
