# STEAM-CREDENTIAL-RECOVERY-1 跨端契约（2026-08-22，released in v0.5.11）

- SteamCMD 明确输出 `Invalid Password` 等凭据失败标记时，driver 必须发布既有 `state=steam_auth_failed` 与 `driverPhase=credentials_required`；即使同一行还包含 `Logging in user`，具体失败也优先于通用 progress。网络、下载、磁盘或其它 SteamCMD 非零退出继续使用 `steamcmd_failed`，前端不得把所有下载失败都解释为密码错误。
- 安装页根据 `steam_auth_failed/credentials_required` 优先展示重新输入凭据路径，不得因同时存在 `installationDiagnostic.status=incomplete/requiredFiles=missing` 而误导用户先修复并继续复用错误凭据。为兼容历史错误终态和诊断不可用，管理员还始终拥有“更换 Steam 账号 / 重新认证”入口。
- 强制入口沿用既有安装请求字段 `steamUsername`、`steamPassword`、`vncPassword`、`imageTag`、`forceReauth=true`。后端据此清除保存的 Steam session 与 SteamCMD 授权卷后重新认证，但保留已下载游戏文件和存档；普通“登录授权”仍复用保存凭据，两种动作的 UI 文案必须保持可区分。
- 联调回归覆盖组合错误行终态、缓存授权回退、部分安装诊断与认证失败的显示优先级、常驻入口完整表单，以及桌面/390px 响应式交互。API/DTO shape、权限边界和凭据不回显契约不变。
- 正式候选 `32575311262` 与 Compatibility `32575311243` 完成后端、前端、真实 Junimo integration、production bundle、不可变 image fresh/restart 和 `v0.5.10` Web unhealthy/healthy；自动 Tag `32575807110` 与正式提升 `32575818623` 成功。`v0.5.11` 三仓版本/`latest` 统一 digest=`sha256:10c9813328370ae8ac92f11271fb76cd03787aab3b7f7fd523f20d66dfae8876`，正式 GHCR health/version smoke 与 Release 均通过。
- 候选没有向 Steam 发送生产或长期测试密码；“组合 progress + Invalid Password”由确定性 driver 回归验证，管理员 force 表单由状态回归、production bundle 和发布前应用内 Browser 验证。任何后续真实 Steam E2E 必须使用可撤销的受控测试账号，且密码/验证码不得进入 job log、artifact 或发布文档。

# v0.5.10 跨端契约正式发布证据（2026-08-20，released）

- 最终候选 `32380002010@9b5a96233331b2050c930658d12eb6e49006f1f0` 从 `v0.5.9` 经真实 Web API 完成同一候选 unhealthy `failed_rolled_back/health_check_failed`、旧版恢复和 healthy apply，并在升级后的新 Panel 重验 exact-target invisible、FIFO no-effect、snapshot restore 与下一管理员 mutation 自动恢复。自动 Tag `32381115159`、正式提升 `32381136325` 全绿；三仓 `0.5.10/latest` 六引用统一 digest=`sha256:f0887c383d0043934b0023cc150e732f6d514e789df2d81c786297c122dc3bb4`，正式 smoke 的 `/health` 与 `/api/version` 精确匹配版本和 commit。

# RUNTIME-UPDATE-TERMINAL-SNAPSHOT-1 跨端终态契约（2026-08-20，released in v0.5.10）

- runtime update API shape 不变，但 `phase=succeeded` 现在只会在认证 snapshot 与旧镜像 best-effort cleanup 已完成、所有 warning/log 已汇总后首次可见。前端一旦观察到 terminal，无需为同一 apply 再轮询等待“迟到的 warning”，终态视图与审计读取同一完整快照。
- cleanup 被阻塞时状态继续保持最后一个非 terminal phase；cleanup 删除失败不把已经通过运行验收的更新改成回滚或失败，而是在同一 `succeeded` 响应的 `warnings` 中给出人工检查提示。失败/回滚 phase、错误码、进度接口和 restart recovery 的安全边界不变。
- 正式候选 `32376230460` 因旧双写窗口在代码门禁失败，未构建/推送镜像；修复后的最终候选 `32380002010` 已完整重跑并发布。回归在 cleanup 阻塞点读取 API backing file，证明 terminal 不早到；失败 run 从未被重新提升。

# SAVE-IMPORT-RELEASE-GATES-1 跨端候选契约（2026-08-20，released in v0.5.10）

- 对外接口 shape 不变。本次把既有跨端契约固化到真实候选：升级 Panel 的 preview/commit 仍以服务端 canonical `saveName` 为唯一目标；Junimo 必须先通过 `saves info <exact-target>` 证明 staged save 可见，失败时 job 终态但正式 import FIFO 零尝试，前端后续管理员选档可触发现有严格恢复并继续请求。
- exact target 可见但 import FIFO 零落盘时，任务返回既有失败语义，journal 只保存有界脱敏诊断和完整复合 no-effect 证据；不得把日志文字升级为成功。snapshot 恢复后，同一管理员选档请求只清理 exact owned transaction/token/staging，保留 preimport 与现有 save/pointer/hash，然后按原接口继续完成选档。
- 真实 runtime 回归先把正常主档打成不带 world ID 的 ZIP，再由 preview 返回 canonical identity；commit 使用该 token/staging 完成 swap/finalizer。官方客户端必须在 `/farmhands` 中按名称找到并选择 `OriginalOwner`，证明“原主机可选”是用户可达结果，而不是仅靠 XML/解绑计数推断。
- `v0.5.9` 已发布产品实现但候选漏跑上述两条 Phase A boundary；旧 tag/digest 保持不变。`v0.5.10` 最终候选已经从 `v0.5.9` 真实 Web unhealthy/healthy 升级并在新 Panel 上复验后才自动 Tag 和正式提升。

# SAVE-IMPORT-RUNTIME-IDENTITY-NORMALIZATION-1 跨端契约（2026-08-20，released in v0.5.9）

- `POST /api/instances/:id/saves/upload-preview` 的 JSON shape 不变，但成功响应中的 `saveName` 现在是后端按 Stardew 实际加载身份规范化后的名称，不保证逐字等于 ZIP 顶层目录。桌面和手机必须继续以响应 `saveName` 展示并提交既有 token，不得从本地文件名或 ZIP 目录重算 commit 目标。
- 规范规则来自主存档身份：主文件名首个 `_` 前缀与非零 `uniqueIDForThisGame` 组合为 `<prefix>_<worldId>`；后端在 token 接管前对私有临时树的目录、主文件和 `_old` 同步 no-replace rename。之后 preview/token/journal/FIFO/Control saveId 使用同一值，Junimo pending finalizer 的 wrong-save guard 不再把同一世界的非规范上传目录当成另一档。
- 没有新增前端选择、提示字段或错误码；既有 `save_exists` 必须对规范后的最终名称判断，因此一个非规范 ZIP 若与现有 canonical 世界同 ID 会在任何覆盖前拒绝。原始 XML、角色、平台 ID 和绑定不会由预览层修改。
- 联调专项至少覆盖：无 `_worldId` 的中文顶层目录成功返回 canonical saveName；commit/journal/一次性 `saves import` 使用该值；runtime `status.saveId` 与 intent 精确一致并推进 finalizer；已规范 ZIP 值不变；canonical 冲突零覆盖；前端刷新/移动端切换后仍只使用服务端 token+saveName。

# SAVE-IMPORT-AUTO-RECOVERY-1 跨端契约（2026-08-19，released in v0.5.7）

- 管理员开始新存档上传时不需要理解或手动处理旧 operation/journal/token。`POST /api/instances/:id/saves/upload-preview` 在读取请求体前、`upload-commit-and-start` 在 reserve 新 operation 前，都会自动尝试收敛“job 已 failed/canceled 且可严格证明未提交 Junimo”的旧事务；成功后原请求直接继续，不新增恢复按钮、恢复 DTO 或前端 token 持久化。
- 自动恢复复用既有安全取消门禁和顺序：exact journal/job/owned-token identity → strict offline/no FIFO ambiguity/fingerprint checks → filesystem cleanup → durable receipt → canceled journal finalize → old token removal。token 只以服务端 SHA-256 目录身份参与恢复，API、日志、审计和浏览器状态均不返回原 token。
- 若 v0.5.5 已通过任务中心清空了终态 job，后端只在 confirmed journal 与 owned token 仍精确绑定同一 job、token 绑定记录早于之后成功的 `jobs_cleared` 审计、当前无活动 import/recovery job 时把该审计作为兼容终态证明；普通 job missing 仍不足以触发删除。现行 `DELETE /api/jobs` 会先收敛可安全恢复的导入，模糊/已提交事务返回 409 并保留任务证据。
- preview 若仍存在 running/queued job、submitted/unknown、身份冲突、运行态或磁盘漂移，必须在接收 ZIP 前返回稳定 409；安全链给出的 `import_recovery_required/save_in_progress` 优先于笼统 busy，不能静默删除或自动重提。前端沿用现有错误码展示即可。
- 联调最低矩阵：pre-submit maintenance/staging 失败后丢失原浏览器 token，再次正常 preview 自动成功且旧 journal/owned token/暂存目标清零、preimport 保留；先清空终态任务后同一正常 preview 也能依据精确审计恢复；模糊事务清空任务中心返回 409 且 job/journal/token 均保留；receipt 后中断可重入；`phaseAFifoWriteAttempted=true` 时 preview 409 且旧材料逐项保持；preview 与 commit 之间旧任务转终态时 commit 再执行同一恢复门。
- 正式候选 `32284304749` 已在 `v0.5.5 → v0.5.7` 的升级后 Panel 中制造 maintenance 失败、按旧版顺序清空终态 jobs 并保留之后的 `jobs_cleared` 审计，再由普通 preview 完成自动恢复；unhealthy 回滚、healthy apply、Panel restart、SQLite/初始化/现有存档/preimport/非目标容器与 volume 保持均通过。正式提升 `32285223565` 未 rebuild，三仓 `0.5.7/latest` 六引用统一 digest=`sha256:0b2dbe649fd6ce7acce797e170fec9ad2f1da9f00730afe1bb39b4ea8d586290`。

# PANEL-UPDATE-LATEST-RELEASE-API-1 跨端契约（2026-08-18，未发布）

- Panel 后端检查正式更新时必须调用 GitHub `GET /repos/anxiyizhi/stardew-server-anxi-panel/releases/latest`，解析单个 Release；不得依赖 `/releases` 列表顺序自行挑第一项。前端仍只消费现有 `latestVersion/updateAvailable/releaseUrl/publishedAt/checkedAt/checkStatus/checkError`，无需改接口。
- latest 对象若为 draft、prerelease 或 tag 不是稳定 SemVer，后端返回现有错误状态并保留上次成功缓存，不允许进入 dry-run/apply。受控升级 E2E 同时提供旧版列表响应和新版 latest 对象，验证上一正式版与候选各自的真实更新检查协议。

# JOB-LOG-LATEST-TAIL-1 跨端契约（2026-08-18，未发布）

- `GET /api/jobs/:id/logs?latest=true&limit=N` 返回该任务最新 N 行日志，但 `logs` 始终按 `sequence` 升序，便于直接渲染；`limit` 最大 1000。响应为 `{ logs, hasEarlier }`，只有服务端确认存在更早行时 `hasEarlier=true`。
- 不带 `latest=true` 时继续使用既有 `after=<sequence>` 正向增量语义，SSE `/stream?after=<sequence>` 也不变。前端详情应先读取 job 再读取最新尾页：终态据此获得完成后的最终日志，非终态随后以尾页最后 sequence 建立 SSE，避免重复显示又不漏掉 GET 与订阅之间的新日志。
- 安装完成页与任务完成页必须显示日志结尾的真实成功/失败信息，不能把最早 1000 行称为“最近日志”。若 `hasEarlier=true`，只提示更早日志未加载，不把它误报成任务仍在运行。

# NEXUS-MOD-ONECLICK-UPDATE-1 跨端契约（2026-08-17，released in v0.5.3）

- 继续复用 `POST /api/instances/:id/mods/remote/install`、现有 `mod_remote_install` Job 与 `Idempotency-Key`。安装请求仍为 `{url, mod, expectedVersion, nexusFileId}`；一键更新额外发送 `replaceUniqueId`，且服务端要求正数 `mod.modId/nexusFileId` 和非空 `expectedVersion`，任一缺失都在建 Job 前返回 400。未提供 `replaceUniqueId` 的旧客户端和普通安装行为不变。
- 前端只对管理员、停止态、扩展已连接、具备直接 Nexus ID 且物理包只有一个成员的已安装 Mod开放入口。扩展 0.1.8 把 `operation/replaceUniqueId` 与版本、file ID 一起贯穿 batch、session、capture、background 直连和 panel bridge；不会修改 Nexus CDN 签名 URL。批量安装的 Nexus 页面按项串行打开，当前项成功提交给 Panel 后才开始下一项。
- 后端在同一 Mod profile 锁内先把 ZIP 下载到临时区，校验目标 manifest 的 `UniqueID == replaceUniqueId`、版本等价于 `expectedVersion` 且 ZIP 只含一个目标 Mod；全部通过后才把旧目录同盘重命名为临时备份并替换。新目录安装失败会删除半成品并恢复旧目录，校验失败则 Mods 目录零写入。
- 替换继承原来的启用/禁用目录，并把旧包根目录 `config.json` 覆盖到新包，避免更新清空用户配置。Nexus sidecar 只有在新元数据持久化成功后才清理旧文件夹条目；更新时不会调用“为当前存档自动启用导入 Mod”的安装后逻辑。
- 聚合包、内置 Mod、目标不存在、UniqueID/版本不符、服务器运行中、普通用户和扩展断连均 fail closed；“查看更新页”外链继续作为手工路径。自动化覆盖成功替换、文件夹改名、配置与禁用状态保留、错误 UID/版本零写入、请求校验和扩展上下文。真实 Chrome + 0.1.8 已验证 CDN 捕获、Panel 任务终态及 Content Patcher `2.9.0 → 2.9.1` 后的 manifest/config/启用状态。
- 聚合跨端契约已随 `v0.5.3@ede7fa34231600cbfa83050b4ddb6fd650373ae1` 发布；候选 `32034798704` 在 `v0.5.2` 升级后的真实 Panel 上重验受影响 API/production bundle，自动 Tag 与同 digest 正式提升均成功。

# NEXUS-EXT-LATEST-1 跨端契约（2026-08-17，released in v0.5.3）

- `GET /api/instances/:id/mods/nexus/search` 的 `results[].version` 与 `results[].requiredMods[].version` 表示 Nexus 当前元数据版本。一键安装的每个本体/前置目标都必须把该值作为 `expectedVersion` 发送给扩展；缺失版本是可见失败，不允许退化为任意文件。
- 扩展仅在普通 Nexus 页面 URL 使用 `anxi_version` 维持导航状态。最终 CDN URL 必须保持 Nexus 原样签名；面板请求为 `{url, mod, expectedVersion, nexusFileId}`，并继续携带原 `Idempotency-Key`。同一批量目标的版本变化会轮换 capture 身份，旧请求不能与新版本合并。
- 扩展先在单文件 DOM 上下文精确匹配版本并确定 `nexusFileId`；面板随后下载临时 ZIP，并在任何 Mods 写入前要求至少一个 SMAPI manifest 版本与 `expectedVersion` 等价。两层任一失败都终止该 item/job，UI 显示目标名称和失败原因，原 Mod 保持不变。
- 旧客户端省略新增字段仍可调用远程安装接口；0.1.5 新 Panel 会显式传目标版本，旧 Panel 批量 payload 缺字段时由扩展从 Nexus 当前文件页补出版本，再走相同文件行匹配。日志、审计和扩展持久状态只记录安全版本/file ID/脱敏 URL，不得保存 CDN key、expires 或完整 query。
- 自动化已覆盖前置版本补全、旧/新候选选择、旧 Panel 缺版本字段兼容、批次串行、两条扩展 POST、后端 manifest 匹配/不匹配与零写入；已登录 Nexus 的经典文件页验证了 `<dt>Content Patcher 2.9.1</dt> + <dd data-id="160463">` 关联，旧版 2.9.0 为 153187，不存在的 2.9.10 不匹配。真实 Chrome + 当前 0.1.8 进一步在停止态测试实例完成 Content Patcher `2.9.1/file_id=160463` 与 Elle's New Barn Animals `1.1.3/file_id=34408` 的串行安装，以及 Content Patcher 更新；两条成功链均由 Panel 任务与落盘 manifest 交叉确认，临时制品为零。远程安装仍只接受 ZIP，发现旧目标实际提供 `.rar` 时没有放宽安全契约或把手工按钮跳转冒充成功。

# FE-SERVER-RUNTIME-SETTINGS-UX-2 显式保存重启契约（2026-08-18，未发布）

- 后端契约不变：配置仍由管理员 `PUT /api/instances/:id/config/server-runtime-settings` 保存，生命周期仍由 `POST /api/instances/:id/restart` 提交；没有把两者合并成新接口，也没有让 PUT 隐式重启。
- 前端三个入口（服务器摘要、总览在线玩家卡片、移动控制快捷操作）共用同一 hook/dialog。选择“仅保存”只 PUT 并刷新 players；选择“保存并重启”必须在运行态先显示在线玩家断线确认，确认后顺序执行 PUT → players 刷新 → 页面既有 `handleRestart`。PUT 失败不得发 restart；restart 失败必须保留已保存配置并提示可重试或稍后手动重启。
- 停止态“保存并重启”保持可见但禁用，只允许仅保存并等待下次启动；不得把 restart 偷换成 start。运行态重启继续由 `useStardewLifecycleActions` 持有 pending/job/状态刷新，设置 hook 不直接调用 `restartInstance`。
- 联调最低覆盖：管理员三入口、普通用户入口隐藏、`1/100` 边界与 44px 自定义步进、在线人数确认、保存失败零重启、保存成功/重启失败的部分成功文案、重启提交后的 pending 状态，以及 520/430/390px 无横向溢出。

# SERVER-RUNTIME-MAXPLAYERS-1 / FE-SERVER-RUNTIME-MAXPLAYERS-1 跨端契约（2026-08-17，released in v0.5.3）

- 继续复用管理员 `GET/PUT /api/instances/:id/config/server-runtime-settings`。GET 结构为 `{ maxPlayers, cabinStrategy, existingCabinBehavior, networkBroadcastPeriod }`，`maxPlayers` 缺失或无效时默认 `10`；合法范围 `1~100`。新前端 PUT 始终提交四个实际值，旧客户端省略 `maxPlayers` 时后端在 driver 锁内保留磁盘原值。
- 写回仍受 `Driver.runtimeUpdateMu` 与 unfinished new-game owner 保护，并使用已有安全原子 JSON 写；只更新 `Server` 的四个目标 key，保留根级、`Game`、`Server` 其它字段。审计事件仍为 `instance_server_runtime_settings_update`，新增 `maxPlayers/previousMaxPlayers` metadata。
- 服务器运行时，dashboard `/players.maxPlayers` 表示 Junimo `info` 读出的**当前生效上限**；配置文件值只表示**重启后配置**，两者不同必须显示待重启。服务器停止时没有 live 值，`/players.maxPlayers` 可投影下次启动配置。运行中 live info 不可读时返回 `null`，不得用待生效配置冒充。
- 桌面摘要只有管理员看到“修改上限”，它与快捷操作、移动控制页都调用同一个 `openRuntimeSettings`、共享 hook/弹窗/保存流。目标值低于当前在线人数只显示警告，仍可“仅保存”；保存本身不重启。成功后刷新 players/dashboard，但运行中当前值要等真实容器重启后才变化。
- `startingCabins 0~7` 与 `maxPlayers 1~100` 继续分离；不按高人数检测 Mod、小屋或加入硬门禁，不把该值做成 SQLite/单存档设置。
- 联调验证已在任务专属 Compose project 中只读克隆真实已有存档和游戏卷：先保存/启动 `11`，`GET /players` 返回 live `11`；运行中 PUT 为 `12` 后同接口仍返回 `11`；调用既有 restart 生命周期并完成就绪后返回 `12`，配置 GET 同时保持 `12`。该 opt-in 场景固化在 `internal/web/server_runtime_settings_real_integration_test.go`，并在 Compose 前断言克隆 `.env` 的 project 身份；源夹具不写入、最终保持停止，任务容器/网络/卷/bind 临时目录均清零。

# SUPPORT-BUNDLE-LOG-CONTEXT-2 / FE-DIAGNOSTICS-EXPORT-ACTION-1 跨端契约（2026-08-17，released in v0.5.3）

- 管理员仍通过 `POST /api/instances/:id/support-bundle` 下载流式 `application/zip`，不依赖 `Content-Length`。前端只把入口移到诊断页标题栏、“重新检查”左侧；鉴权、请求方法、Blob 下载和文件名契约不变。
- ZIP 的日志契约扩展为：`server-logs.txt` 最近 1000 行、`steam-auth-logs.txt` 最近 500 行、`panel-logs.txt` 最近 1000 行，以及 `job-logs.json` 中当前实例最近 10 个任务各最多 200 条日志。`jobs.json` 只列当前实例最近任务，`instance-state.json` 使用诊断页完整状态投影但强制移除邀请码。
- 日志与 JSON 消息均在后端脱敏；存档内容、Steam session、密码库、上传/恢复事务、备份和任务 payload 不导出。某个容器或数据源不可读时，对应 ZIP 文件保留采集失败说明，其余条目继续生成；前端不需要按缺失条目做分支。
- 联调最低覆盖：匿名/普通用户权限不回归、管理员得到有效 ZIP、四类新增日志条目存在、已知密码/Token/邀请码不出现、按钮在桌面位于重新检查左侧、430px 无横向滚动。后端 Docker/Web 专项、前端响应式契约、production build 和本地浏览器双视口已通过。

# HOST-BED-MANUAL-CONTROL-1 跨端/运行时契约（2026-08-16，released in v0.5.1）

- 调用链保持 `Panel save import transaction → JunimoServer saves import --swap-host-to → Control game-thread integrity/finalizer evidence → Control save-now/GameLoop.Saved`。Web 层仍只提交 hostHandling/platformId 与展示事务结果，不解析或改写 Stardew XML。交换主机的激活成功现在要求 Control `status.json.hostBed` 明确 healthy、houseUpgradeLevel 存在、期望/实际床型匹配，且 bed tile 与 player bed spot 关系能由实际主 FarmHouse 验证；缺床使用稳定错误码 `host_bed_missing`。
- Control 自愈只操作 `Game1.getLocationFromName("FarmHouse")` 且其 owner 为 `Game1.MasterPlayer` 的主屋。0 级使用游戏单床常量，1/2/3 级使用游戏 double-wide 转换；坐标来自当前 FarmHouse `Back` map 的 `DefaultBedPosition`。已有床、其它家具以及 Farm/cabin/其他位置完全不动；无法证明布局时激活失败，并由已确认 swap 的 preimport 事务回滚整树、活动指针、Mod profile 与实例快照。
- 现有实例状态响应 `statusSource` 透传 `hostBed` 与 `hostControl`。`hostControl.mode` 为 `automation|manual|unknown`；F9 手动态须同时满足 automation=false、manualControl=true、pauseReason=ManualControl、paused=false、visibilityConsistent=true，零网络客户端不再触发 NoConnectedClients。再次 F9、10 分钟无人租约到期、读档/跨日都会恢复或重算策略。F10 的可见/隐藏结果要求 `hostVisible`、`displayFarmer`、`farmerHidden` 与 shadow 同步，不能只依赖某一个静态 flag。
- 睡眠契约不再容忍无限 `sleeping in place`：缺主屋床时单次结构化告警后阻断该故障 episode；有床时按实际 BedFurniture/player bed spot 调用原生睡眠路径，单日游戏线程动作上限为 4。真实 Docker E2E 已通过独立客户端进入下一日，并断言无强制结束、无超时兜底；该结果是实际游戏跨日，不是只检查 XML 或模拟状态。
- 联调回归应覆盖：0..3 级地图床型/床位、重复启动/导入/自愈幂等、其它家具保留、Farm/cabin 床不误判、角色非目标数据不串位、激活与 durable-save 故障完整回滚、无客户端 F9 输入、退出手动后 NoConnectedClients 恢复、F10 多次切换及 warp/重启/跨日一致。Junimo 容器内 REST 一律使用固定 8080，不能把宿主映射端口带入容器内部探针。
- 发布证据：`v0.5.1` annotated tag 精确指向 `427a295ab905701069b7f710300ba09b6afd21f0`；候选、兼容、Tag、正式提升四个 workflow 全绿，三仓版本/`latest` 使用同一 digest `sha256:70c1967eb36827dbbf78ec3c11683c994814961dcf6673ae365ec4f43c6c25a5`，正式镜像 smoke 与 GitHub Release 成功。

# MOD-UPDATE-CHECK-1 跨端契约（2026-08-16，released in v0.5.2）

- `GET /api/instances/:id/mod-updates` 供任意已登录用户在打开 Mod 页面时自动调用；返回 `{ status, checkedAt?, updates, eligibleCount, skippedCount, checkError?, cached }`。`updates[]` 为 `{ id, uniqueId, name, folderName, currentVersion, latestVersion, url }`。`status=error` 是可展示的上游降级态而非 HTTP 失败，前端必须继续展示同响应内的上次成功 `updates`。
- `POST /api/instances/:id/mod-updates/check` 仅管理员可用且强制绕过 6 小时 TTL；响应 DTO 与 GET 相同。前端只在「添加模组」页签提供该按钮，普通用户仍可看到自动检查结果。上传/删除后前端触发强制刷新，本地清单从其它路径变化时则由清单指纹自动失效并通过 GET 重查。
- 后端 Stardew driver 负责清单扫描、资格过滤、SMAPI update service 通信、缓存和 URL 校验；请求根级 `apiVersion` 优先来自 Control `options.json`，缺少运行时快照时使用 v4 基线 `4.0.0`，可用时同时发送实际 `gameVersion` 与 Linux platform。Web 层只做鉴权、driver capability 断言与序列化。没有新增 SQLite 表、后台定时器、系统通知、Nexus Key 读取或无人值守 Mod 替换链路。
- 联调时至少覆盖：首次成功、同指纹缓存、本地清单或 SMAPI API/game version 变化后失效、无 eligible Mod 不出网、上游失败保留旧结果、非 HTTP(S) 建议链接被丢弃，以及页签徽标/筛选/卡片外链。SMAPI 公共接口变化必须表现为页内降级，不能阻塞 `GET .../mods` 主列表；候选升级 E2E 使用受控 TLS SMAPI 服务固定验证首次强刷与 cached GET。
- `v0.5.2@51fd82459e4ac8afbf362f7ad12c0651937879a1` 已完成该契约的正式收口：候选 `31945655119` 从 v0.5.1 真实 Web 升级后同时命中匿名 401、管理员强刷、缓存 GET 和 production bundle 契约，并完成同一候选的 unhealthy 回滚；Compatibility `31945655121`、Tag `31946063809`、正式提升 `31946073920` 全部成功。三仓 `0.5.2/latest` 六引用统一 digest=`sha256:42b5dae824f63d3b5ba44a1f33704a622a62c4d6170225d52a63ac39147aaaed`，正式镜像首次/重启的健康与版本接口均通过。

# v0.5.0 跨端发布结果（2026-08-16，released）

- v0.4.19 已发布的 none/global/role 加入保护、角色独立密码、旧 `SERVER_PASSWORD`/旧 API 兼容和 Control `TryAuthenticate` fail-closed patch 继续保留；v0.5.0 新增存档导入 strict/maintenance/job-token-cleanup 恢复、真实 `lastSeen` 语义，以及 Control 0.3.4 的主机农舍等级保持。公开存档提交 DTO、玩家 DTO shape、数据库 schema 与前端路由不变。
- Compatibility `31899107019` 与候选 `31899107629` 成功；候选从 `v0.4.19` 经公开 update check/dry-run/apply 验证 unhealthy `failed_rolled_back/health_check_failed` 和 healthy 升级，并在升级后的 Panel 重跑受影响 E2E。存档长期结构的 `v0.4.11 → 0.5.0` 代表升级已在本地候选预演通过；首轮 CI 取消时序测试失败未构建或推送镜像，确定性修复后重新走完整 workflow。
- 自动 Tag `31899867310` 与正式提升 `31899874927` 成功，annotated `v0.5.0` 解引用到 `9b18dd3fe5192692548bf11a85010dd35303da93`。Docker Hub、阿里云 ACR、GHCR 的 `0.5.0/latest` 六引用统一 digest=`sha256:92ea973d55c1f63b4eb356652d491f8d37ef5f69112df1f19c161e4b0e9b611a`；独立正式镜像首次/重启均为 health ok、版本/commit/build date 精确、setup initialized=false，任务资源归零。

# HOST-FARMHOUSE-PRESERVE-1 Panel/Control 联调契约（2026-08-16，released in v0.5.0）

- Control `0.3.4` 的 `options.json` 与 `status.json` 新增 `hostFarmhousePreservationPatchAvailable` 和诊断用 `hostFarmhousePreservationPatchDetail`。Panel 启动只接受当前 Control 版本且 availability 明确为 `true`；字段缺失或 false 返回稳定码 `control_runtime_host_farmhouse_patch_unavailable` 并停服。
- 补丁默认启用、没有 Web/API 开关。它在 `SaveLoaded` 前精确跳过 JunimoServer `.125` 的主机农舍归零方法；公开实例/存档/任务 DTO、前端调用与数据库契约均不变化，客户端无需增加字段或分支。
- 联调不得只伪造 options=true：正式候选必须用精确推荐镜像完成真实 `SaveLoaded`，再经 Control `save-now` 的同 commandId `GameLoop.Saved` 结果确认升级房屋等级仍在磁盘。目标签名漂移必须表现为 fail-closed 停服，而不是继续读档。

# SAVE-IMPORT-TOKEN-CLEANUP-RECOVERY-1 联调契约（2026-08-15，released in v0.5.0）

- `POST /api/instances/:id/saves/upload-commit-and-start` 的 JSON 形状不变；新的 `202` 硬前提是数据库 primary job、owned token 与 journal 已共同记录同一 `operationId/jobId/jobType/idempotencyKey`，且 journal 为 `jobBindingState=ready`。job 创建成功但 token attach/journal confirm 失败时返回 `import_recovery_required`，runner 不进入 staging/preimport/bootstrap/maintenance/FIFO，不能先 202 后靠日志补偿。
- commit 重试与 Panel 重启只查询 `type=stardew_import_save_and_start + target=instance/:id + save-import:<operationId>`，并校验 job payload；不按 createdAt、最近任务或 token 内单独的 jobId 猜测。三方缺失/冲突、exact job 不存在或 payload 不符均 409 recovery required；验证一致时才幂等补 journal/token binding 并返回原 `jobId/operationId/saveName`。
- `{token,cancel:true}` 对 available token 保持原语义；owned token 只允许 exact job 为 failed/canceled、strict fresh stop 通过、journal 的 maintenance/FIFO/upstream/ownership/pointer/全树指纹与 cleanup schema 全部可证时继续。queued/running/succeeded/submitted/confirmed、未知字段/阶段、缺证、目录漂移均 fail closed。删除采用持久子阶段，filesystem completed 后写独立 receipt，再删 journal/token；重复请求和并发请求都返回同一终态，危险删除最多一次，preimport 始终保留。
- succeeded token TTL 到期后只压缩为 exact-result tombstone，同 token 仍返回原 202 幂等结果；completed journal、preimport、正式存档和 job 记录不删。公开前端不新增字段或按钮，现有任务轮询、单次 FIFO、同名 no-replace、邀请码与 durable save 流程不变。

# SAVE-IMPORT-MAINTENANCE-DURABILITY-1 联调契约（2026-08-15，released in v0.5.0）

- 公开 `upload-preview`、`upload-commit-and-start` 请求/响应与 hostHandling 形状不变；变化只在后台事务耐久边界。维护 runtime 只有在权威快照 journal、数据库 maintenance phase、`MaintenanceStarted=true` 三次持久化都成功后才会 `ComposeUp`，因此 phase 或 journal 故障会保持 `ComposeUp=0`，Web job 返回稳定失败而不会短暂暴露普通 running/invite 状态。
- 自动失败恢复必须同时完成 ComposeDown(0)、strict fresh stop、`MaintenanceStarted=false + snapshot_restore_pending` journal、精确数据库快照恢复和 `snapshot_restored` journal。缺任一证据时 Web 继续得到 `import_recovery_required`/busy 语义，owned upload token、journal、bootstrap/staged target 都保留，取消接口不能把 pending/manual recovery 误当成可安全清理。
- Phase A 新增内部 `phaseAFifoWriteAttempted` 写前证据，不进入公开 DTO。该位为 false 且两个 upstream flag 均 false时，pre-submit 失败会安全停机并恢复原状态；该位为 true 而 submitted receipt 尚未持久化时，Panel 重启只会停机并保持人工恢复，不会二次写 FIFO、恢复普通离线状态或释放 token。
- Panel 重启分类覆盖 start intent、ComposeUp returned、Down 后未清旗、清旗后未恢复四个窗口。前三类在恢复前执行无缓存停机证明，最后一类再次 strict probe 后幂等恢复；任何 Docker/JSON/journal/storage 不确定性都 fail closed。Web pending-upload、首次安装提交、取消与错误映射专项及后端全量回归通过，并已由 v0.5.0 候选的升级后受影响 E2E 与正式 proof 收口。

# SAVE-IMPORT-STRICT-OFFLINE-PROBE-1 联调契约（2026-08-15，released in v0.5.0）

- `POST /api/instances/:id/saves/upload-commit-and-start` 的请求体、`202/jobId/operationId/saveName`、权限和前端流程不变。数据库处于 `game_installed/save_required/ready_to_start/stopped` 只代表允许尝试；driver 必须在接管上传前用无缓存 `docker compose ps --all --format json` 证明真实 server 已稳定终止。
- 只有项目无 server，或所有 server 条目均为 `exited/dead`，才允许继续。任一 `running`、`Status` 以 `Up` 开头、`restarting`、`paused`、`created`、`removing`、空/未知 state、坏/`null`/缺字段 JSON、命令错误或输出截断均 fail closed。普通页面继续走缓存 `ComposePs`，不会因安全门禁收紧而增加轮询或改变展示 DTO。
- strict 失败发生在 runtime asset、journal、bootstrap/staged target 与 token ownership 之前：Web 返回既有 409 导入错误并把尚未 owned 的 reservation 释放回 `available`，上传源仍可重试。maintenance 启动前、`ComposeUp` 前、失败 `ComposeDown` 后与 owned cancel cleanup 前均重复独立 strict 证明；不得以 cache invalidation 或等待 TTL 代替。
- Web 传入的实例目录与数据库权威 `DataDir` 不一致时，driver 返回 `import_recovery_required`，不在任一目录创建/删除事务数据。专项 Web 回归固定 `game_installed + 普通 ComposePs=exited + strict=fresh running`：commit 必须 409、token 回到 available、staged source 保留且无 import journal。v0.5.0 候选已把受影响链纳入升级后真实 Docker E2E；后续修改 strict/ownership 时仍需重跑。

# v0.4.18 跨端发布结果（2026-08-15，released）

- 存档导入的公开 DTO、错误码和前端提交流程保持不变；后端把成功且空 stdout 的 Compose 查询识别为 0 services。运行栈 repair/apply DTO 只增加稳定的内部失败码，Control-only 事务会独立物化宿主 JunimoServer；共享 body Portal 与本地分页不增加 API 请求或后端字段。
- 最终候选 `31884242692` 从 `v0.4.17` 通过公开 update check/dry-run/apply 完成 unhealthy 回滚与 healthy 升级；升级后的 production bundle、旧 `rollback_failed` 第三次 repair 和空 Compose 存档上传均通过。Compatibility `31884242697`、自动 Tag `31884612425`、正式提升 `31884620508` 成功。
- Docker Hub、阿里云 ACR、GHCR 的 `0.4.18/latest` 六引用统一 digest=`sha256:b304e3b9c83620e94e3a16f33f5730991f74e470820a7481e696b54738eb8d74`。独立正式镜像首次/重启均为 Docker healthy、`/health`/database ok、版本/完整 commit/build date 精确、setup initialized=false；GitHub latest Release 为正式 `v0.4.18`，四项资产与 tag 源一致。

# RUNTIME-UPDATE-JUNIMO-MATERIALIZE-1 联调契约（2026-08-15，released in v0.4.18）

- `GET/POST /api/instances/:id/junimo-update*` 的请求与响应结构不变；repair/apply 可能新增稳定 `rollbackCode=rollback_repair_junimo_mod_failed`。Control-only 不再表示“完全不碰 JunimoServer”：只有宿主组件通过目标版本、UniqueID、DLL、普通文件和无 symlink 校验时才跳过，缺失/损坏会从选定目标 image 事务化补齐，auth 容器和 steam-session 仍保持不变。
- 已由旧版本写入 `rollback_failed`、且清单显示 server image 未变/Junimo 未替换的实例，在本补丁中点击现有“修复：恢复旧版后升级”时，后端可从清单绑定的原 immutable server image ID 补齐组件，先通过原版本验收，再重新检测和创建新事务；前端不得自行删除状态、恢复目录或减少 repairAttempts。
- 候选升级后专项必须构造 server/auth 已是推荐 tag、旧 Control、宿主 JunimoServer 缺失的实例，确认 SMAPI 缺依赖的旧故障可由新 Panel 的公开 repair/apply 路径收敛为 succeeded；同时断言 auth 容器 ID/认证卷保持、恢复材料按成功终态清理。无法提取原 image、路径映射失败、manifest/DLL 不合法仍 fail closed 并保留材料。

# SAVE-IMPORT-COMPOSE-EMPTY-SET-1 联调契约（2026-08-15，released in v0.4.18）

- `POST /api/instances/:id/saves/upload-commit-and-start` 的请求、202 响应、错误码和管理员权限不变。Panel Stop 使用 `docker compose down` 后，`docker compose ps --all --format json` 成功返回空 stdout 代表该项目当前没有容器；后端必须把它作为空服务集合并继续既有 server stopped 判定，不能误映射成 `save_in_progress`。
- 非零退出、非空坏 JSON、条目缺 service/state、未知状态及真实运行中的 server 仍拒绝；前端无需增加字段或文案。候选升级测试已加入真实 `Panel Stop → 零 Compose 容器/空 stdout → upload-preview → commit 202/jobId/operationId`，并让受控 maintenance 容器立即退出以验证失败终态后的 stopped 恢复及 Docker 资源归零；该链必须在候选中通过。

# v0.4.17 跨端发布结果（2026-08-15，released）

- `RUNTIME-AUTH-HEALTH-PROBE-1`、`SAVE-IMPORT-FIRST-INSTALL-STATE-1` 与社区中心收集包文案已在同一 `v0.4.17@d63c93ffe7d65f8cdfcf2bedb9b336a6839be73f` 候选中验收。auth 服务健康与 Steam 在线能力已分层；首次上传的 Web/driver 四态契约、strict Compose 实停门禁、精确恢复与安全取消保持同一后端权威；前端字段和公开上传 DTO 均未改变。
- 候选 run `31823172958` 从 `v0.4.16` 通过公开 update check/dry-run/apply 先验证不健康目标 `failed_rolled_back/health_check_failed`，再升级精确候选；SQLite、初始化、Panel 数据、非目标游戏容器/volume 与重启均保持。Compatibility `31823172972`、Tag `31823884131`、正式提升 `31823899038` 成功。
- Docker Hub、阿里云 ACR、GHCR 的 `0.4.17/latest` 六引用统一 digest=`sha256:44c328cdf198ec888f3ec54bbe836ce114f5ac27c4ca5fb9cc63747a44083673`。独立正式镜像首次/重启均为 Docker healthy、`/health`/database ok，`/api/version` 精确返回版本、完整 commit 与 build date；四项 Release 资产齐全。

# v0.4.16 跨端发布结果（2026-08-14，released）

- 后端 required-runtime 历史失败收敛、前端 `FarmhouseStack` 隐藏兼容和游戏日回档悬停详情已在同一不可变候选中完成 fresh 及 `v0.4.15` Web 升级后验收；公开 update、server runtime settings 与 backup API 契约均未变化。
- 候选 run `31799350642` 验证同引用 unhealthy 目标返回 `failed_rolled_back/health_check_failed` 并恢复旧版，随后健康目标升级为 `0.4.16@5fa04d137bf760d2124b75cc5e3e8e2b44ff4c7c`；SQLite、初始化、Panel 哨兵、非目标游戏容器/volume 和 Panel restart 均保持。Tag `31799876171` 与正式提升 `31799891830` 成功，三仓 exact/latest 统一 digest=`sha256:5f07910869d6d895e40ecb3954f5905d0cb6abf830e7cf57062bbcf97ca37e0f`。

# REQUIRED-RUNTIME-STALE-STATUS-1 / FE-CABIN-FARMHOUSESTACK-HIDE-1 联调契约（2026-08-14）

- `GET /api/system/update/apply` 的公开结构和 full-stack phase 枚举不变。后端读取 required 协调状态时，只有当前 Panel/stack、最新 runtime apply=`succeeded`、实时 managed stack=`up_to_date` 三项同时成立，才把同 pair 的旧 `required-status.phase=failed` 持久化为 `succeeded` 并清空旧错误；`manual_action` 与任何当前未达标状态不得被前端或后端当作历史故障隐藏。
- 普通 Junimo/Control 运行组件 apply 成功会立即触发该收敛；Panel 重启自动协调和后续升级状态轮询再提供幂等兜底。前端不需要新增“清理历史故障”按钮，也不自行比较文件时间或版本字符串。
- `GET/PUT /api/instances/:id/config/server-runtime-settings` 仍接受并可能返回 `CabinStack|FarmhouseStack|None`。桌面和移动下拉框仅隐藏 `FarmhouseStack` 选择入口，已有该值时必须保持受控值兼容，不能在未明确选择并保存时自动改写服务端配置。

# RELEASE-CANDIDATE-PROMOTION-1 联调契约（2026-08-14）

- 候选门禁不新增生产 API，也不允许测试专用 release URL 进入 Panel 配置。真实旧版仍访问固定 `https://api.github.com`，但任务专属 DinD 用私有 CA、受控 TLS endpoint 和容器级 host 映射返回唯一候选 Release；Docker daemon 同时只在该 DinD 内把 `ghcr.io` 映射到受控 TLS registry。CA、cookie、registry、Compose、bind 和 volume 都随 DinD 清理，不进入镜像、GitHub artifact 或日志。
- E2E 必须通过公开接口依次调用 `POST /api/setup/admin`、`POST /api/system/update/check`、`POST/GET /api/system/update/dry-run` 和 `POST/GET /api/system/update/apply`。上一正式版使用当前 `{"confirmFullStack":true}`；代表老版本如果仍采用历史空 body，只有第一次请求明确返回 400 且尚未创建 apply 时才允许按旧契约重发空 body。
- 相同精确版本引用先指向带失败 HEALTHCHECK 的候选派生镜像，必须恢复上一正式版并返回 `failed_rolled_back/health_check_failed`；随后 registry 原子切回本次构建的健康候选，再创建新的 check/dry-run/apply。健康链要求目标 `/health`、`/api/version` 的 version/full commit、SQLite integrity、初始化状态、Panel 哨兵、非目标游戏容器 ID/volume hash 和 Panel 重启后 apply 终态全部正确。
- 正式 tag workflow 不调用上述 API，也不重新测试另一份构建；它只消费成功候选 workflow 上传的 `candidate.json`，验证 tag/main/commit/version/build date/digest 后提升原对象。候选 artifact 不匹配、过期或 registry 对象被覆盖时 fail closed。
- 产品路径 push 自动触发候选；成功 run 由独立 `workflow_run` 工作流校验证明并再次读取 `origin/main`。仅当前 SHA 未被后续提交取代时创建 annotated tag，并显式 dispatch 正式提升 workflow；候选被取代时不调用 Panel API、不移动 tag、不发布 registry。

# SAVE-IMPORT-FIRST-UPLOAD-1 联调契约（2026-08-13，completed，未发布）

- `upload-preview`、`upload-commit-and-start` 的请求/响应和管理员权限不变。过去必须先启动一次才能上传的空实例，现在可直接 commit；后端会在接管上传 token 前同步精确 `.125` image 内的 JunimoServer Mod，并在无活动存档时创建 operation-owned bootstrap 维护世界。
- 新稳定错误码 `save_import_runtime_prepare_failed` 表示 image 提取、宿主原子替换或静态复核失败，前端应保留上传并提示检查 Docker/网络后重试；`junimo_import_unsupported` 只表示明确的 image/tag/协议不兼容。两者都发生在导入 journal 与上传所有权转移前，不能出现半提交 job。
- bootstrap 完全是后端事务细节，不进入公开 DTO。API 接受后的 `202` 仍只代表 job 已创建，不能视为导入成功；最终 completed 仍要求目标存档已激活、Control/Junimo/durable-save/磁盘门禁通过且 bootstrap 已清理。提交前取消恢复到无 pointer；指针漂移、碰撞、清理失败或不确定执行结果均进入既有 recovery 契约。
- 候选联调必须从空 `Saves`、无 gameloader、宿主 JunimoServer Mod 缺失的真实实例开始走完整 Web 上传；同时覆盖同步断流可重试、bootstrap 复制/发布中断、Panel/游戏容器中断恢复、取消和资源归零，并在 v0.4.14/v0.3.2 Web 一键升级得到的新 Panel 上再次执行。

# NEXUS-EXT-IDEMPOTENCY-1 联调契约（2026-08-13，completed，未发布）

- 扩展 0.1.3 对每次用户安装动作生成稳定 `requestId`，后台直连和 panel bridge 都以 `Idempotency-Key` 请求头提交到既有 `POST /api/instances/:id/mods/remote/install`；请求体 `{url, mod?}` 不变，`fileId/requestId` 只在扩展内部消息中传递。
- 首次创建仍返回 `202 {jobId}`；相同 key 重放返回 `202 {jobId, deduped:true}`，其中 jobId 必须与首次完全一致。前端/扩展应继续观察该任务，不得因为 deduped 再开任务；旧扩展不带键时保持兼容，但没有跨进程幂等保证。
- key 的服务端作用域为 `mod_remote_install + instance + key`，终态也保持绑定。网络超时、页面刷新或 MV3 worker 重启必须复用同一 key；用户明确重新选择文件或在任务终态后重新开始安装时生成新 key。非法 key 返回 `400 invalid_idempotency_key`。
- 进程内 singleflight 只负责减少重复 HTTP；SQLite 唯一索引才是任务创建权威。不同 Mod 文件即使 modId 相同也不得共用 requestId；失败 POST 要恢复可重试 capture，不缓存 rejected Promise。

# SAVE-IMPORT-AUTO-UNCLAIM-1 联调契约（2026-08-13，completed，未发布）

- upload-preview 与 upload-commit-and-start 的请求/响应完全不变；桌面和手机继续提交现有 hostHandling。前端不新增“国内 IP / Steam / 自动解绑”选项，也不改变当前 platformId 输入、校验或提示。
- 后端仅对 hostHandling.mode=swap_to_player 默认自动解绑。Junimo 先继续用 platformId 完成既有 swap-host-to 主机迁移；finalizer 确认后，Control 在同一个耐久 save-now 的保存前清空全部 farmhand 绑定并落盘。virtual_host_takeover 不清空 ID，也不额外保存。
- 202 受理、runtime_ready、import_confirmed、finalize_confirmed 都不是完成。swap_to_player 新增完成硬条件：同一 commandId 的 GameLoop.Saved 结果必须证明动作和目标存档一致、boundFarmhandCount=0，Junimo diagnostics 必须再次证明相同角色集合为零绑定，随后仍要通过 dayTransition、稳定 XML 和磁盘变化门禁。
- 旧 Control、DLL hash 不一致、动作结果缺失、错档、有人在线、角色数据不可读或任一角色仍绑定时，job 保持失败/恢复语义，不发布邀请码，不把实例提升为普通 running。前端继续只根据现有 job/journal 终态显示成功，不解析日志；新增内部 maintenance 错误不会要求新请求字段。
- 结果/journal 只保存 total/customized/bound 计数，不回传或持久化原始 userID/SteamID。自动解绑不删除角色、小屋、物品或关系；隔离真机证明 2 个角色和 1 个 customized 标记保存、重启后仍在，仅绑定数从 1 变 0。
- 通用 command history 同步不得抢占导入的 durable result：只要任一未完成 save-import journal 的 `DurableSaveCommandID` 与文件相同，`DurableCommandResultProtected` 就要求保留原 JSON，供当前任务或 Panel 重启恢复读取动作身份和聚合计数；journal 不可读时停止同步。只有导入写入 `completed` 后才按原公开白名单入库并删除文件。该保护不改变任何 Web DTO，也不会把存档名、SteamID 或 Control 私有字段扩大到公开历史。

# STARTUP-NEWGAME-DURABILITY-1 联调终态（2026-08-13，released in v0.4.14）

- `/api/instances/:id/state` 的 `installationDiagnostic` 是前端操作映射的普通用户权威：明确完整安装的 lifecycle error 返回 `installed/retry_start`，缺文件才 `incomplete/repair_install`，不可读证据为 `unknown/diagnose`。Control runtime 只有观察到合法非期望版本才 mismatch，缺 snapshot 在预算内是 pending。
- `POST /saves/custom-new-game` 强制 `Idempotency-Key`；同 key/config 返回原 job，缺 key 为 428，同 key 不同配置/其它 owner 为 409。完成只在 transaction-bound save-loaded、完整内存角色、同 ID GameLoop.Saved、双 XML 稳定正确四段证据全部成立后返回成功。
- 正式 v0.4.11/v0.3.2 真实 Web 升级、unhealthy 回滚、621 hard-coded/no-env conversion 及升级后 desktop/mobile error 映射均已通过；最终 `v0.4.14@a70efc98feec` 的 Release workflow、三仓统一 digest、资产和逐仓重启也已通过。生产真机因 `114.55.142.107:22` 的正确 SSH 用户名缺失而未执行升级；取得用户名后仍须按 `docs/09-image-build.md` 以精确 `0.4.14` 同步，游戏保持关闭。

# PANEL-UPDATE-GRAPHICAL-COMPOSE-1 联调契约（2026-08-12，completed，未发布）

- `GET /api/system/update/capability` 与 dry-run/apply 的 JSON shape 不变。完整 Compose labels 的图形化 NAS 部署若缺少可持久化 `.env`，或当前 service 的 `image` 不随 `PANEL_IMAGE` 变化，但容器、项目、服务、镜像、数据挂载及迁移安全边界均可证明，响应现在为 `supported=true`、`code=supported`、`conversionRequired=true`；前端继续显示“将先转换为标准部署”和“转换为标准部署并升级”。
- dry-run 对 conversion 路径只完成身份、目标版本、可信镜像候选和安全标准化前置检查，不停止或重建容器。apply 仍只接受严格 `{"confirmFullStack":true}`；独立 helper 二次 inspect 后备份 SQLite/Compose/容器环境/旧 digest，生成 `.env + image: ${PANEL_IMAGE}`，切换成功后由新 Panel 继续既有全实例运行栈协调。
- Compose service label 与实际 resolver 结果不一致、容器无法唯一反查、privileged/自定义 user、缺 Docker Socket/非 bind 数据目录、危险或不可保真的额外挂载等情况不得因为“缺 `.env`”而自动迁移；返回 unsupported 和具体 `deployment_env_invalid|compose_image_unmanaged` 原因。前端不得按错误字符串自行把 unsupported 改成可执行。
- 标准部署探针使用无网络 runner 和固定不可拉取 image 字符串，仅执行 `docker compose config --images`；不访问 registry、不读取 Docker credentials、不把 `.env` 内容或 `PANEL_SECRET` 写入状态/日志。图形化部署常见的镜像声明匿名 `/data` volume 可作为 external volume 保留，非目标游戏容器和 volume 不参与标准化。
- 真实联调已用无 `.env`、写死 image、完整 Compose labels、宿主绝对路径 bind 和匿名 `/data` volume 的 `0.4.10` 夹具验证：dry-run 返回 `supported=true + conversionRequired=true`，一次 `confirmFullStack` apply 经断线重连到 `0.4.11/succeeded`，标准 `.env`/Compose 自动落盘，游戏容器 ID 和匿名 volume 保持、旧 Panel 停止保留。前端无需新增分支或临时操作说明。

# INSTALL-FIRST-RUN-CONSISTENCY-1 联调契约（2026-08-11，released in v0.4.11）

## 重复安装与授权请求

- `POST /api/instances/:id/install` 和 `POST /api/instances/:id/steam-auth/login` 在同一实例已有 queued/running `stardew_install` 时返回 HTTP 409：`{"error":{"code":"install_in_progress","message":"...","details":{"jobId":"job_..."}}}`。这不是一次新失败；前端必须接管 `details.jobId`、刷新 jobs/instance state 并连接该任务日志。
- 同一安装任务的状态具有单调性。前端按 job ID 合并 jobs 列表与详情：terminal 胜过 queued/running；实例状态决定已安装等最终事实；仅当前 active job 的日志可补充阶段与进度。历史日志不得重新生成当前下载/认证状态。

## 安装/二维码认证取消后的资源终态

- Web 取消 job 或 Panel 关闭导致安装 context 取消时，Linux 后端会在前台 `docker compose run` 退出后按该次随机唯一容器名删除 auth one-off。调用方可先看到 job 进入 canceled；30 秒内同一名字的容器必须不存在，不能继续刷新 QR。
- 清理只针对本次 auth one-off，不删除 `GAME_DATA_VOLUME`、steam-session 或其它 Compose service。重复点击仍按活动 owner 契约返回同一 job；取消终态释放 partial unique index 后才允许新的安装任务。
- 发布联调使用无真实账号的 QR 路径：首次安装到 `auth_method_required`，输入 `2` 到 `steam_qr_required`，取消 job 后验证任务终态和容器稳定消失。最初 3.48 秒的单次缺席检查会漏掉 daemon 晚到 create，不能作为通过证据；连续 3 秒缺席修复后的最终 tag 源码真实 `.125/auth .2` gate 9.64 秒通过，外层案例 container/volume 复核为 0。

## 首次建档前 SMAPI 运行目录

- 安装任务只有在 `/data/game/Mods` 已经通过受控 one-shot container 物化并验证到 `.local-container/mods/smapi` 后，才能写 `game_installed`。新建存档在事务快照、Mod 选择和 `ExpectedFingerprint` 计算前再次幂等同步；同步失败返回事务码/driver phase `smapi_bundled_sync_failed`，不得启动 server 或留下 new-game transaction。
- 同步源是当前 `GAME_DATA_VOLUME` 中实际安装的 SMAPI Mods，目标只限 Panel 管理的 `mods/smapi` 命名空间。用户顶层 Mod、disabled 目录和存档不在替换范围。SMAPI 升级切换与自动回滚也必须分别从新卷/旧卷同步，确保 entrypoint 后续 `init_mods` 不改变预检集合。
- 联调主验收为全新实例一次完成：并发双提交只返回一个 owner → 安装终态页面无历史下载卡 → 不重试直接第一次创建存档成功 → Panel/server 重启后指纹与存档保持。复制失败、坏 manifest、Panel 中断、Compose 启动失败和 SMAPI staging 回滚由单元、真实 helper 与两条升级链的故障矩阵共同覆盖。
- 已完成的真实 Docker helper 专项使用当前精确 server `.125` 镜像和唯一临时 game-data volume，验证 server 启动前首次发布两个 manifest、第二次幂等不替换且清理无残留。真实生命周期 gate 以空 saves bind 启动 Junimo/SMAPI 并两次创建唯一首存，分别 71.78/60.04 秒成功；job log 明确物化 sequence 9 早于事务快照 sequence 10，存档可解析、为活动存档且无 owned staging 残留，第二轮 Stop 后容器归零。最终 tag 候选又从 `v0.4.10`/`v0.3.2` 真实 Web 升级，并在升级得到的 Panel 上用 1.96 GiB game-data 创建存档 `Tag Release Gate`；Panel restart 后存档和 server/auth 容器 ID 保持。完整证据见 `docs/09-image-build.md`。

# RUNTIME-AUTH-OFFLINE-ACCEPTANCE-1 联调契约（2026-08-09，released in v0.4.10）

- required runtime apply 的 `verifying_auth` 硬门槛为 steam-auth 容器 running、目标 image ID 精确匹配，以及容器内 `GET /steam/ready` 命中受支持 HTTP/schema 合约；Docker health、Steam 登录和 ticket 不再是升级成功条件。
- HTTP 200 legacy schema 要求 `ready:boolean`，`has_ticket` 可选；current schema 要求 `status=ok`、`logged_in:boolean`、`accounts:array`。真实镜像未配置账号的 HTTP 503 只接受 legacy `ready=false`；HTTP 500/其它状态、503 current/畸形 schema、坏 JSON、`accounts=null/number/object`、接口不可达和 image ID 漂移均不得继续 server 重建，必须进入原事务失败/回滚链。
- 前端继续只消费既有 `phase/progress/updatedAt`。`verifying_auth` 显示“正在尝试 Steam 连接”和累计等待；读屏 live region 只宣布阶段标题，动态秒数不重复播报。无新增 API 字段、权限或凭据暴露。
- 发布联调证据：v0.4.9 与 v0.3.2 均通过真实 Web check/dry-run/apply 升级到精确 v0.4.10；Panel 自更新候选 `HEALTHCHECK=false` 会自动回滚，健康 Panel 候选重试成功，Panel restart 和数据/非目标资源保护通过。另一个 runtime Docker integration 独立证明 steam-auth Docker health=unhealthy 但 `/steam/ready` 命中合法离线 schema 时升级继续。在升级得到的新 Panel 上，769×240 与 280×653 的 `verifying_auth` 计时跨轮询增长、唯一 live headline 和二维码 overlay 几何/滚动/关闭交互均通过。完整事务 ID、耗时、镜像摘要与清理见 `docs/09-image-build.md`。

# RUNTIME-UPDATE-REPAIR-CATALOG-3 联调契约（2026-08-09，completed，未发布）

- `GET /api/instances/:id/junimo-update` 可返回可选 `repairPlan`：`actionAvailable:boolean`、`action:repair|export|wait`、稳定 `code`、`title`、`detection`、`method`、`buttonLabel`、`steps[]`、`attempts/maxAttempts`。它是后端当前快照的只读判断，不代表浏览器可以缓存后稍后盲目执行。
- `action=repair && actionAvailable=true` 才可提交严格 `POST .../repair {"confirm":true}`。后端在实例锁内用同一 detector 重判并二次验证材料；当前三类为 `repair/rollback_failed`、`repairable/legacy_candidates` 与 `repair/safe_retry`。计划变化、并发任务、材料漂移或次数耗尽均拒绝，而不是执行旧的前端选择。
- `action=export` 表示零自动 mutation：按钮调用支持包导出，供人工核对损坏状态/清单、材料摘要、自定义镜像、配置歧义或三次耗尽。ZIP 的 `junimo-update.json` 白名单条目包含公开 `inspection + repairPlan + applyStatus` 并整体脱敏，不包含 recovery manifest/目录/原文件；`action=wait` 不提交写请求，用于持久事务非终态或推荐矩阵暂不可用。
- 前端必须展示后端 `detection + method`，并把 `buttonLabel` 作为实际按钮文案；脚本 `check` 也输出同一按钮和修复方法。这样网页、脚本和执行端不会各维护一套可能漂移的故障判断。

# RUNTIME-UPDATE-DIAGNOSE-REPAIR-2 联调契约（2026-08-09，completed，未发布）

- `POST /api/instances/:id/junimo-update/repair` 现在是持久化“检测、修复并继续升级”入口，可在两种后端已证明的状态接受：精确 `rollback_failed` 事务，或 `GET /junimo-update` 返回 `repairable=true` 的可信历史候选配置。其它状态返回 `runtime_repair_not_needed`；损坏状态/材料、未知配置、任务冲突与三次耗尽继续 409，权限和严格 body 不变。
- 状态序列为 `rolling_back`（有失败事务时）→ `resuming_upgrade`（已恢复，正在识别历史配置和完整 dry-run）→ 新 `applyId` 的普通 apply phases → `succeeded|failed_rolled_back|rollback_failed`。`repairSourceApplyId` 关联首次检测事务，`resumeAfterRepair` 只用于后端崩溃恢复；前端不得据此自行重放 POST。
- `checks` 会保留 `repair_failure_state/repair_manifest/repair_materials/repair_original_runtime`，历史配置命中时增加 `known_issue_detection/known_legacy_config_repaired`，完整预检通过后增加 `retry_*`、`repair_upgrade_preflight` 和按实际 image ID/tag/Control 内容生成的 `change_plan`。这些是脱敏诊断证据，不包含恢复路径、token 或任意命令。
- repair job 在重新 apply 前再次执行在线玩家通告、存档持久化确认和整档保护备份（存在活动存档时）。修复后的 dry-run 或维护门禁失败会收敛到 `failed_rolled_back` 并保持旧版本可用；脚本此时退出非零，只有 `succeeded` 返回成功。

# RUNTIME-UPDATE-WAL-REPAIR-1 联调契约（2026-08-08，completed，未发布）

- 新增管理员接口 `POST /api/instances/:id/junimo-update/repair`。请求体必须严格为 `{"confirm":true}`；匿名返回 401、普通用户 403，额外的 image/tag/path/applyId/command/strategy 字段返回 400，当前不是 `rollback_failed`、材料不一致、任务冲突或三次耗尽返回 409，接受后返回 202 与原 `RuntimeUpdateApplyStatus` shape。
- repair 不执行新的检查/下载/升级，也不接受前端选择恢复对象。后端从实例私有 status + manifest 取得唯一 apply ID、原 image IDs、原配置和快照卷，校验 schema 3 SHA-256 后启动同一个实例级 job 锁；响应新增可选 `repairAttempts`。
- 前端只在 `rollback_failed` 显示管理员按钮；202 后按 `GET /api/instances/:id/junimo-update/apply` 轮询 `rolling_back → failed_rolled_back|rollback_failed`。页面刷新或 Panel 在 repair 中重启时，`repairAttempts>0 + rolling_back` 会从持久化清单继续同一次幂等回滚。
- Release asset `/app/repair-junimo-upgrade.sh` 只调用上述登录、状态和 repair API。它不挂 Docker socket、不拼接 Compose 命令、不读取恢复目录；交互式密码不回显，非交互仅接受普通文件 `PANEL_PASSWORD_FILE`，临时 cookie/JSON 权限 0600 并在退出时清理。
- 这是 required Junimo server/steam-auth/Control 运行组件升级的恢复入口，不改变 Panel 自更新或 SMAPI staging 的接口。独立升级链的剩余差异记录在 `docs/07-later-optimizations.md`，不得把本接口宣传成任意 Docker 故障修复器。

# RUNTIME-UPDATE-PRESERVE-AUTH-1 联调契约（2026-08-08，completed，未发布）

- required runtime 状态/API shape 不变。apply 仍使用既有 phase；内部根据 current/target tag + image ID 生成 change plan。server/auth 均相同而 Control 旧时，前端仍看到完整维护进度和最终 `succeeded`，但实际 Docker 操作只重启 server。
- auth 未变化的运行态升级必须保持 steam-auth container ID，禁止 stop、force-recreate、steam-session snapshot/restore 和 Steam readiness 网络探针；只允许校验容器 image ID/running/healthy，并原地设置 cpu shares=256。server 必须重启并实载推荐 Control，最终 `/health/status/info` 仍按原契约验收。
- auth 真正变化时继续走快照、重建、最多 10 分钟 readiness 与自动回滚；Steam 登录/ticket 仍是能力而非硬门槛。Control-only 失败回滚不得重建未变化 auth，也不得把同一出站网络故障放大为 auth rollback failure。
- Docker Desktop 实测覆盖 stopped/running 两条 Control `0.2.0 → 0.3.0` 链；每条使用复制的独立 instance/game-data/steam-session，凭据清空。运行态 auth ID 不变、shares=256，server shares=768、Control 0.3.0 实载、状态恢复；首轮 shares=0 的失败属于有效产品缺口，修复后同一真机矩阵完整通过。

# v0.4.8 玩家 Mod 联调发布结论（2026-08-07，released）

- 发布接口为 `GET /api/instances/:id/players/:uniqueMultiplayerId/mods`；`context.status` 为 `reported/pending/unavailable/stale`，未取得清单时 `mods:null`。comparison item 只使用 `match/missing_on_client/client_only/version_mismatch`，比较不可用由 `comparison.status=unavailable` 表示。
- `missing_on_client` 只统计服务器实际加载、启用且 `client_required` 的 Mod；`server_only` 不告警，SMAPI、Junimo Controller、Anxi Panel Control 不参与普通比较。前端顺序固定为玩家额外安装、玩家缺少 Mod、版本不同、匹配。
- CJB 只按两条官方 UniqueID 不区分大小写提示，列表和详情均有明确文字，但没有管理操作。清单是客户端自报，修改 manifest ID 可绕过；此功能不能作为自动处罚依据。
- `v0.4.7 → v0.4.8` 真实 Web 更新、unhealthy 自动回滚、升级后 API/UI fixture、三仓正式镜像回拉和独立 health/version smoke 均完成。尚未具备的实体客户端矩阵继续按下表记录为未验证。

# PLAYER-MOD-PRESENTATION-2 联调契约（2026-08-06，completed，v0.4.8 released）

- 详情接口不再把面板自带运行组件纳入比较：`Pathoschild.SMAPI`、`JunimoHost.Server`、`AnXiYiZhi.StardewAnxiPanel.Control` 必须从 `serverContext.loadedMods`、comparison items 和 summary 排除。玩家/服务器 `apiVersion` 字段继续返回，但不再合成虚拟 SMAPI 比较项。
- 可见结果顺序固定为 `client_only / missing_on_client / version_mismatch / match`；前端对应文案为“玩家额外安装 / 玩家缺少 Mod / 版本不同 / 匹配”。前端还按相同三条 UniqueID 做兼容过滤，避免混合版本缓存重新显示内置项。
- CJB 横幅只显示“检测到该玩家使用了 CJB 作弊工具”，条目只显示“检测到 CJB 作弊”；不再显示两段解释。只读、不处罚和客户端自报边界仍是接口安全约束，只是不在这两个 UI 位置重复文案。
- 桌面/390×844 手机 QA 已验证分组次序、内置项与旧解释均不存在，手机 root/body 无横向溢出；后端全量 test/vet/build、前端状态/响应式测试、TypeScript 与 production build 通过。该验收使用隔离 fixture，不替代尚未完成的实体 CJB/移动客户端联机。

# PLAYER-MOD-CJB-LIST-1 列表提示联调契约（2026-08-06，completed，v0.4.8 released）

- `GET /api/instances/:id/players` 的玩家项可带 `"modRiskFlags":["cjb"]`。这是后端从独立 `player-mod-contexts.json` 提取的轻量标记；响应不包含 `mods`，完整清单仍只能通过 `GET /api/instances/:id/players/:uniqueMultiplayerId/mods` 获取。
- 字段缺失或空数组表示没有可展示的 CJB 标记，不能反推“安全”。存在 `cjb` 时，桌面/手机玩家列表按钮显示“检测到 CJB 作弊”，桌面与手机待认证卡同样显示文字徽标；详情横幅显示“检测到该玩家使用了 CJB 作弊工具”。
- 标记来源始终是客户端自报，stale 可保留最后一次检测结果；修改 manifest UniqueID、未上报或不兼容链路仍可能绕过。所有提示均只读，不改变加入、批准、踢出、封禁或自动拦截逻辑。
- 隔离 QA 已覆盖 desktop reported 列表/待认证/详情及 390×844 mobile 总览/列表/详情，手机 root/body 横向溢出为 0；Go 测试覆盖高频玩家响应不泄漏完整 Mod 数据。

# PLAYER-MOD-COMPAT-1 第三阶段联调矩阵（2026-08-06，PC+SMAPI 单客户端真机通过，其余实体矩阵受限，v0.4.8 released）

证据等级必须分开记录：C# 契约测试验证 Control 实际复用的状态转换；Go loopback 测试通过真实 HTTP handler、SQLite 与隔离文件系统；真实 game-data 编译验证 SMAPI/game API 可用。2026-08-06 又增加一台本机真实 Stardew `1.6.15` + SMAPI `4.5.2` 客户端的标准 LAN/IP 联机证据，但它仍不能替代原版、CJB、移动端或多客户端矩阵。

| 场景 | 当前证据 | 结论 |
| --- | --- | --- |
| PC 原版经 IP 加入 | pending→10 秒 unavailable、`mods:null`、HTTP/UI 禁止 0/一致/安全已自动化 | 实体 PC 加入与不断线未验证；不得宣称真机通过 |
| PC + SMAPI 经 IP 加入 | 本机真实客户端加入 `127.0.0.1:24682`；reported，Content Patcher 2.9.0、Farm Type Manager 1.26.1、Save Backup 4.5.2 的 ID/名称/版本均准确；游戏/SMAPI 为 1.6.15/4.5.2 | 单客户端真机通过；加入和保持在线正常，无管理动作 |
| 官方 CJB Cheats Menu | 官方 ID、红色文字提示、只读采集路径与无管理调用回归通过 | 实体客户端正常加入未验证；不会自动踢出/封禁 |
| 官方 CJB Item Spawner | 与 Cheats Menu 相同，单独 fixture 通过 | 实体客户端正常加入未验证；不会自动踢出/封禁 |
| 缺少/额外/版本不同 | Go loadedMods 比较和前端分组 fixture 通过 | 自动化通过 |
| server_only | 未上报时不生成 missing；若客户端也上报，可作为“服务器专用”match 展示 | 自动化通过 |
| Android/iOS 官方客户端 | unavailable/null 与页面稳定状态已覆盖 | 本机无真实移动客户端，IP 加入、不报错/不断线均未验证 |
| Android 实验性 SMAPI | 无可用真实环境 | 未验证，不推断支持 |
| 等待、断线、重连、重启、多玩家 | 同一真实客户端主动断线后 stale；重连同一 ID 生成新 reportedAt 并恢复 reported；隔离 server 受控重启成功且旧记录为 stale。pending/unavailable、多 ID 隔离仍有 C# 回归 | 单客户端断线/重连/server 重启真机通过；多个远端玩家同时在线未验证 |
| 异常/重复/超长字段 | C# 采集端与 Go 读取端双层长度、控制字符、大小写去重、数量上限回归，前端长文案/重复归组通过 | 自动化通过 |

真机联调另启动了任务临时根目录中的真实 Panel/Junimo server：根目录 `%LOCALAPPDATA%/Temp/sap-player-mod-real-20260806`，游戏端口 `24682/udp`，Panel `18766/tcp`，使用从 `stardew_game-data` 只读克隆出的独立 game-data volume 和新建的 `ModContextLab` 测试存档；没有挂载或复用用户存档/设置，也没有修改既有实例。该次旧口径抓取为虚拟 SMAPI 与 Save Backup match、Content Patcher/Farm Type Manager client_only；按当前契约应隐藏 SMAPI/Control/Junimo，只保留用户 Mod 比较。production dist 的静态详情路径实际返回 HTTP 200；本轮隔离 QA 已补桌面/手机视觉，但仍不是实体 CJB/移动客户端真机证据。

隔离例外必须保留：游戏 driver 的最终 Compose project 是实例目录 basename `stardew`，没有继承任务的 Panel project 名；启动前只检查任务 label/端口，遗漏了创建于 2026-07-06 的既有 `stardew_steam-session`，本次 steam-auth 因而复用了该卷。未读取其中 token 内容，清理时也不得删除该旧卷。LAN/IP ModContext 证据不依赖邀请码，仍可用于验证本功能，但认证卷完全隔离与专用测试凭据没有得到证明，后续真实矩阵必须在启动前预检最终 project/network/全部 volume 名。

Mod 清单始终是客户端通过 SMAPI peer context 自报，不是服务端对客户端文件系统的可信审计。CJB 只按 `CJBok.CheatsMenu` / `CJBok.ItemSpawner` 不区分大小写精确提示；修改 manifest UniqueID、使用未上报/不兼容链路或篡改客户端都可能绕过。该功能只用于人工参考，不能作为自动处罚依据。

# PLAYER-MOD-CONTEXT-1 / FE-PLAYER-MOD-VIEW-1 联调契约（2026-08-06，前后端两阶段完成，v0.4.8 released）

本阶段只提供登录态只读接口，不新增前端页面，也不会拦截、踢出或封禁玩家：

```http
GET /api/instances/:id/players/:uniqueMultiplayerId/mods
```

- `uniqueMultiplayerId` 是带可选负号的十进制字符串，最长 32 个字符；非法值返回 `400 invalid_player`，未登录返回 `401`，非 GET 返回 `405 method_not_allowed`。
- 有可比较上报时返回 `200`。`contextStatus` 为 `reported`，`mods` 是玩家实际上报的数组（真实“零 Mod”才是 `[]`）；`serverContext.loadedMods` 只来自该服务器进程实际生成的 `options.json.loadedMods`，物理 Mod 扫描只用现有 `ModInfo/syncKind` 补名称和分类，不能把未加载目录加入比较基线。
- `comparison.items[].result` 只会是 `match / missing_on_client / client_only / version_mismatch`。`missing_on_client` 仅针对实际加载、启用且 `syncKind=client_required` 的服务器 Mod；`server_only` 和无法确认分类的条目不得因客户端未上报而产生缺失告警。
- 面板自带的 `SMAPI`、`JunimoHost.Server`、`StardewAnxiPanel.Control` 不参与玩家 Mod 比较；服务端和 peer 的 `apiVersion` 仍作为上下文元数据返回。普通 Mod 版本沿用后端规范化比较，因此例如 `1.0` 与 `1.0.0` 等价。
- 玩家清单里不区分大小写精确命中 `CJBok.CheatsMenu` 或 `CJBok.ItemSpawner` 时，相关 item 和顶层都包含 `riskFlags:["cjb"]`。这是只读提示，不触发任何管理操作。

可比较响应核心形状：

```json
{
  "instanceId": "...",
  "uniqueMultiplayerId": "-3583227484444031316",
  "hasSmapi": true,
  "gameVersion": "1.6.15",
  "apiVersion": "4.3.2",
  "mods": [{"uniqueId":"Example.Mod","name":"Example","version":"1.0.0"}],
  "contextStatus": "reported",
  "reportedAt": "2026-08-06T12:00:00Z",
  "updatedAt": "2026-08-06T12:00:00Z",
  "serverContext": {
    "gameVersion": "1.6.15",
    "apiVersion": "4.3.2",
    "generatedAt": "2026-08-06T12:00:00Z",
    "loadedMods": []
  },
  "comparison": {"status":"available","items":[],"summary":{"match":0,"missingOnClient":0,"clientOnly":0,"versionMismatch":0}},
  "riskFlags": [],
  "message": ""
}
```

- 未收到可靠清单、Control 尚在等待、客户端无 SMAPI、玩家已断开、实例未运行、sidecar 损坏或运行态基线不可读时仍返回 `200`，但 `comparison.status="unavailable"`，并由 `contextStatus/message` 说明原因。`pending` 和 `unavailable` 必须返回 `mods:null`，不得用 `[]` 冒充“没有 Mod”；`stale` 可以保留最后一次清单供展示，但禁止继续比较。
- Control 独立原子写入 `control/player-mod-contexts.json`，完整清单不会进入高频 `players.json`。它只监听标准 SMAPI peer 上下文；未触发该上下文的客户端（包括当前未实现的 Steam SDR 兼容路径）、无 SMAPI 客户端、超限或不可信上报、以及上下文超时/断开的玩家都只能得到 unavailable/pending/stale，前端不得据此推断其没有 Mod。

前端消费约定：

- 静态详情路由为 `/instances/stardew/player-mods?playerId=<uniqueMultiplayerId>`；SPA 只白名单该精确 pathname，playerId 由前端 URL 编码后传给上述只读 API。桌面玩家表和移动玩家卡只在 ID 存在时展示入口。
- pending、unavailable、stale、comparison unavailable 与 HTTP error 必须区分。客户端清单 unavailable 使用固定解释，不显示统计；只有 `comparison.status="available"` 才渲染四项统计与分组。当前 stale 响应的 comparison 为 unavailable，只能提示最后记录过期。
- 页面按不区分大小写的 UniqueID 去重并过滤三类面板内置组件，再从可见 items 计算统计；异常的 `server_only + missing_on_client` 在前端也会被丢弃。普通服务器专用条目可以在 match/version mismatch 组显示“服务器专用”，但不得成为玩家缺少 Mod 告警。
- CJB 总横幅和对应条目都包含明确文字标签，但不再附加解释段落。桌面和移动复用 `PlayerModsDetail`，移动端在 `MobilePlayersPage` 内切换列表/详情并提供返回按钮。
- API 加载失败只提供重试；不影响玩家加入、认证和已有管理按钮。页面不会提交踢出、封禁、批准认证或其它写请求。

# SMAPI-DOWNLOAD-RESUME-1 联调契约（2026-07-28，v0.4.5 released）

- HTTP API、安装请求、job 类型和阶段名均不变；仍是 `steamcmd_downloading → smapi_installing → game_installed`，失败仍为 `smapi_install_failed`。前端不提交 URL、Range、SHA 或超时参数。
- 后端按 embed 清单的固定顺序 `gh.llkk.cc → github.dpik.top → ghfast.top → GitHub 官方` 逐候选执行 2 MiB Range。网络读取中途超时但已有进展时从偏移继续；候选连续无进展或最终整包校验失败时清空临时文件并切下一项，因此页面可能在 `smapi_installing` 停留较久。
- 真实完成门槛仍是缓存固定大小/SHA/ZIP 结构验证、JunimoServer 一次性安装容器成功，以及 `INSTALL_REQUIRED_FILES_OK`。任何 `206/Content-Range` 不匹配、连续无进展或最终摘要错误都必须失败且不能留下正式缓存。
- `.env.SMAPI_DOWNLOAD_URLS` 不参与安全下载选择；新增/替换代理必须随 Panel 代码修改 embed 精确 URL 模板、host allowlist、Go/Python 校验和发布门禁，不能由实例配置绕过。当前无新增前端状态或文案要求。
- `v0.4.5` 发布后从 ACR 正式 `0.4.4/0.4.3/0.3.13` 分别直接执行生产 `RunApply` 到正式 `0.4.5`，目标版本/健康、SQLite setup、404 和 game 容器隔离均通过。升级后同一发布代码的真实空缓存 SMAPI 下载与确定性续传/回退测试通过；客户端升级 API、轮询和确认契约无需变更。

# SAVE-BACKUP-EAGER-MAINTENANCE-1 联调契约（2026-07-28，completed）

- HTTP API 与 `BackupPolicy {gameSaveBackups,retainGameDays}` 形状不变；前端仍按 `kind=auto` 和 `gameDayOrdinal` 展示最近游戏日回档点。
- 自动回档不再依赖 `GET /api/instances/:id/saves/backups` 被访问。Panel 启动即补扫 `save-events`，运行期间每 2 秒消费一次；列表接口保留原补扫调用以兼容混合版本和瞬时失败，但与后台消费者按实例串行。
- 前端不能把列表响应中的 `maintenance.createdBackupNames` 当作唯一新增通知：正常情况下事件已被后台消费，因此该字段多数为空。权威数据仍是响应中的完整 `backups` 列表。
- 已经被旧版本延迟消费并覆盖掉的历史游戏日无法从当前磁盘反向重建；升级只保证后续 `GameLoop.Saved` 及时形成对应游戏日 ZIP，不伪造缺失的旧回档点。

# PANEL-SQLITE-INTERRUPT-1 HTTP 契约（2026-07-24，completed）

- `/api/setup/status` 的响应形状仍为 `200 {"initialized":boolean}`，但值来自 Panel 启动时缓存；首个管理员创建成功后在同一进程内立即变为 `true`。前端无需改变初始化流程。
- SPA fallback 只接受 `/`、`/index.html`、`/instances/stardew` 及 `/instances/stardew/{install,overview,server,saves,jobs,players,mods,diagnostics,settings}`。其它未知页面和未注册 API 统一返回 `404 {"error":{"code":"not_found",...}}`，不再返回 `index.html` 或在未初始化时误报 `503 setup_required`。
- 请求取消可能返回既有的取消/网络错误，但不会污染下一次 SQLite 查询。若进程连续得到三次原生 `SQLITE_INTERRUPT`，连接池被视为无法恢复，Panel 主动退出并依赖部署的 Docker restart policy 恢复；客户端应按普通短时断线重连处理。

# MOD-INSTALL-TIME-1 / FE-MOD-LIST-SEARCH-1 联调契约（2026-07-31，completed，未发布）

- Docker Desktop 真实联调使用候选 Panel 的实际 setup/session、`POST /mods/upload`、`POST /mods/remote/install` + job 轮询、`GET /mods`、`DELETE /mods/:id` 和持久 volume。浏览器扩展一键下载路径返回的 Mod 与人工 ZIP 一样获得 `installedAt`；重启后 API 与 UI 时间不变。
- 真实失败矩阵覆盖多 ZIP 部分成功回滚和 sidecar 原子提交失败：两者均不得留下 Mod 目录或孤立安装时间，旧记录保持可读。`/mods/remote/install` 的公网 HTTPS 下载、Content-Type、完整进度、ZIP 校验和 job `succeeded` 均来自真实运行组件，不是 API mock。

- `GET /api/instances/:id/mods`、人工上传响应及所有返回 `ModInfo` 的接口新增可选字符串 `installedAt`，格式为 UTC RFC3339Nano。它表示 Panel 成功安装该物理 Mod 目录的时间；Nexus 上游更新时间仍是 `updatedAt`，两者不得混用。
- 同一 ZIP 中导入的全部 Mod 使用完全相同的 `installedAt`；一次选择多个 ZIP 时每个成功 ZIP 各自提交。若后续 ZIP 或当前存档启用事务失败，已导入目录和安装时间一起回滚。
- 人工上传和一键下载可以由不同 HTTP/job 入口完成；后端必须串行化 sidecar 读改写，两个不同 Mod 并发成功时两条时间都要保留。该互斥只保护小型补充元数据，不改变下载或 ZIP 处理的 job 并发模型。
- 历史 Mod、手工复制目录或损坏/不匹配的补充记录可以没有 `installedAt`。前端必须兼容字段缺失，不能使用浏览器当前时间、文件夹顺序或 `updatedAt` 填充。
- 搜索和排序在前端本地完成，不新增查询参数或服务端过滤接口。批量启用/禁用继续调用原接口并作用于完整第三方 Mod 集合，不能把当前搜索结果当请求范围。

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

- 2026-08-15 首次安装状态修复把提交与 driver 的共享离线集合固定为 `game_installed / save_required / ready_to_start / stopped`。Web 先按集合拒绝明显不安全状态；driver 在 journal/token ownership 前重新读取数据库，并让整条导入链只使用该权威 `DataDir`。接管前与 `ComposeUp` 前的 Docker 证据都使用无缓存 `docker compose ps --all` 严格解析；空/坏输出、缺字段、未知状态以及任一 running/restarting/paused/removing server 都拒绝。因此安装完成无需先调用 Stop API，也不能仅凭数据库离线或缓存的 Compose 结果放行实际运行中的 server。
- 上传提交仍返回 `202 + jobId + operationId`。job 在 staging/preimport 后启动维护 runtime；实例对 UI 继续呈现 `stopped`，phase 为 `save_import_maintenance`，不会发布 `ready` 或邀请码。客户端不得把 server 容器存在、job 成功或 `runtime_ready` 单独解释为导入成功。
- `runtime_ready` 仅表示 `.125` 容器、FIFO、可读日志、health/status API、`saves` 列表命令和复合 baseline 均已就绪。该段是 MAINTENANCE-RUNTIME-1 的历史边界；PHASE-A-1 已在其后发送正式 import，但依然没有 Panel 指针预写、新游戏或 XML 修改。
- 维护 phase 的邀请码读取被拒绝；若 `/status.playerCount > 0`，job 以 `save_import_players_connected` 明确失败且不会踢人。`runtime_ready` 前失败/取消会先 ComposeDown；只有随后 ComposePs 证明 server 已停才恢复进入维护前精确的 state/phase/message/payload，并保留 staged/preimport。停机不可证时保留 maintenance ownership，不能自动 cleanup。
- journal 新增 `maintenanceStarted`、`serverOutputLogOffset`、`runtimeBaseline`；baseline 中 pending UserID 仍为 `json:"-"`，不得通过 API、日志或 journal 外泄。

# SAVE-IMPORT-STAGING-1 联调契约（2026-07-16，completed）

- `upload-preview` 响应不变。`upload-commit-and-start` reserve token 后，driver 先建立 journal，再同步把 payload 转入 operation/source；只有 source 所有权落盘且 job 创建成功才返回既有 `202 {jobId,operationId,saveName}`。因此响应后不再依赖 handler 内存中的 tempDir。
- token 内部 `owned` 状态允许同 operation 重新发现 source，其他 operation 拒绝占用；job 使用 operation 派生的数据库 idempotency key。进程若在 job 创建与 token `jobId` 绑定之间退出，提交/cancel 重试从 key 找回同一个 job；完全不存在该 key 才可证明没有 durable runner。cancel 对 available token 仍删除 pending upload；owned token 仅在 terminal job（或无 durable job）、driver 四态离线、strict Compose server 实停、journal `MaintenanceStarted=false` 且未提交，并通过 bootstrap pointer 与 staged fingerprint 门禁时执行 transaction cleanup。所有只读证据先生成完整 cleanup plan，通过后才删除；先写 `stage=canceled`，再删除 token，最后清 journal，使任一进程中断都可幂等继续且不会永久 busy。它删除 source/本 operation 创建且未变化的 staged target/bootstrap/pointer/token，保留 preimport；活动、成功、submitted 或证据不确定均返回 busy/recovery。成功 token 短时保留幂等响应后按到期元数据回收。
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
- 本次流式改造当时保持了原 ZIP 条目；2026-08-17 的 `SUPPORT-BUNDLE-LOG-CONTEXT-2` 后续在同一协议上新增 Panel/steam-auth/任务进度日志，并把实例状态升级为完整诊断投影，当前条目以本文顶部契约为准。
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
- `maxPlayers` 在 running 状态只表示 Junimo `info` 解析出的当前生效值；服务器未运行时才取 `server-settings.json` 的 `Server.MaxPlayers` 作为下次启动配置。live info 暂时不可读时返回 `null`，不能回退到待重启配置并伪装成已生效。
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
- 新增 `GET /api/instances/:id/config/server-runtime-settings`：管理员专用，当前返回 `{ "maxPlayers": number, "cabinStrategy": "CabinStack"|"FarmhouseStack"|"None", "existingCabinBehavior": "KeepExisting"|"MoveToStack", "networkBroadcastPeriod": number }`。文件不存在或字段缺失时返回 `10`/`CabinStack`/`KeepExisting`/`1` 默认值，不报错。
- 新增 `PUT /api/instances/:id/config/server-runtime-settings`：管理员专用。新客户端传完整四字段结构；兼容旧三字段 PUT，省略 `maxPlayers` 时保留磁盘原值。后端校验 `maxPlayers` 为 `1~100`、小屋策略枚举和广播频率 `1~10`，失败返回 `400 invalid_settings`。成功只覆盖四个目标字段，`AllowIpConnections` 等其它 Server/Game/根级字段原样保留。
- 这两个接口是服务器控制页“联机人数与小屋设置”弹窗的完整版入口，和新建存档页的简化 `cabinMode` 共用同一份底层 `server-settings.json`，但**不是**同一层级：新建存档页只在建档瞬间写一次初始值，服务器控制页可以在存档已存在、服务器运行中或已停止的任何时候读写。两边都不会互相覆盖对方没有涉及的字段。
- **关键约束**：无论通过哪个入口修改，都只在 JunimoServer `server` 容器**下一次启动**时生效（和 `SERVER_PASSWORD`、`ExistingServerSettings` 同理），前端弹窗必须提示"需要重启服务器容器"，不能暗示实时生效。
- 联调验证建议：新建存档时分别提交 `cabinMode` 缺省、`"recommended"`、`"vanilla"`，确认 `Server.CabinStrategy` 分别正确；建档后用 PUT 设置 `maxPlayers=1/100` 并验证边界、`0/101` 拒绝、旧 PUT 省略时保留、未知 Server/Game 字段与原子写不回归。运行中设置与当前值不同后，GET 配置应返回新值但 `/players.maxPlayers` 仍返回 live 旧值，重启后才读到新值。
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
- Historical note (superseded on 2026-08-14): a fresh scaffolded database once failed before maintenance because its real post-install state was `game_installed`, and the test workaround used `CleanupUnsubmittedImport` plus Stop API normalization. The fixed contract now accepts `game_installed` directly while still requiring Compose server stopped; no permanent rewrite to `stopped` is needed. A legacy pre-submit failure can be canceled only after terminal-job identity, stopped-runtime, `MaintenanceStarted=false`, ownership/pointer/fingerprint and no-upstream-effect gates pass; the driver holds the lifecycle mutex across the final Compose check and cleanup, and preimport remains retained.
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
# 2026-07-20：Panel → 全实例 Control 升级契约

1. 管理员先执行 dry-run；响应中的 Compose 项目、服务、容器、镜像和数据挂载必须由后端反查一致。`conversionRequired=true` 表示 apply 将由独立 helper 标准化旧部署。
2. apply POST 必须携带 `confirmFullStack=true`。Panel 阶段成功后不能停止轮询：以 `fullStack.phase` 和 `fullStack.instances[]` 判断全栈终态。
3. 对运行且 Control 不匹配的实例，后端先通告在线玩家，再保存和整档备份；随后停服、更新、重启并以新鲜 `options.json.controlModVersion` 验证 SMAPI 实载版本。
4. `failed_safe`/`manual_action` 表示实例保持停止，前端不得提示升级完成或主动启动；`succeeded`/`not_needed` 才是全栈终态。
5. Panel 在流程中重启后，客户端只需重新连接并继续 GET apply 状态；剩余实例由新 Panel 自动恢复协调。
6. 实例 `API_PORT` 是宿主发布端口，容器内健康/API 契约固定为 8080。所有 FIFO `compose exec` 必须显式使用实例目录派生的 project，不能信任可能陈旧的 `COMPOSE_PROJECT_NAME`。
# 2026-07-24：v0.4.2 Panel 升级联调验收

- HTTP 更新契约和 `confirmFullStack=true` 不变。本次升级只替换 Panel 镜像；初始化缓存会在新容器启动时从原 `/data/panel.db` 重建。
- 新增 opt-in 真实镜像测试 `TestDockerIntegrationRealPanelCandidateUpgrade`。提供旧版/目标精确镜像与版本后，它使用正式 `RunApply` 路径升级真实 Panel，并断言目标版本健康、SQLite 状态可查询、未知路径 404、数据文件非空及同 Compose 游戏容器 ID 不变。
- Docker Desktop 已完成 `0.4.1 → 0.4.2` 候选升级引擎验收；正式发布后又在隔离 DinD Linux 宿主中由真实 `0.4.1` Web API 发现 GitHub Release、完成 dry-run、拉取 GHCR 精确镜像并 apply。终态 `succeeded`，新容器版本/健康、管理员初始化状态、404 与部署环境镜像引用全部正确。
# 2026-07-26：v0.4.3 健康监控与一键升级联调验收（已发布）

- `/health` 响应形状不变；Panel 内部每分钟调用同一个 `Store.Ping`，单次 5 秒。只有连续三次 SQLite 原生 code 9 才退出，成功/超时/其它错误清零，业务请求错误不参与计数。客户端仍按普通短时断线重连。
- Docker Desktop 29.5.3 先使用真实 GHCR `0.4.2` 和本地最终候选执行正式 `RunApply`，发布后又使用公开 GHCR `0.4.3` 精确 tag 复验：两次终态均为 succeeded，目标 `/health`/版本/SQLite setup 状态/未知路径 404 正确，数据卷非空，隔离 game 容器 ID 未变化。
- 候选 `restart: unless-stopped` 验证中 Panel PID `13602 → 13772`、RestartCount `0 → 1`，重启后 `/health` 恢复。Docker unhealthy 本身不触发 restart policy；进程退出是唯一恢复触发点，且没有游戏容器操作。
# 2026-08-01：未安装实例的全栈升级终态

- Panel apply 主状态为 `succeeded` 且目标版本等于当前 Panel 时，后端才补充 `fullStack`。实例状态为 `uninitialized` 或 `admin_created` 时，`fullStack.instances[]` 返回 `phase=not_needed`、`progress=100`，且 `runtimeRequired=false`。
- 如果全部实例都属于未安装、已是目标运行栈或 driver 无跟随升级要求，顶层 `fullStack` 同样返回 `phase=not_needed`、`progress=100`、`runtimeRequired=false`。前端应按终态展示，不再显示“正在全栈升级 42%”。
- 已安装实例缺少当前 Panel 对应的 required-runtime 状态时仍返回 `checking_runtime`；`failed_safe` 与 `manual_action` 优先级不变，不能只按 Panel 主状态 `succeeded` 隐藏游戏运行栈失败。

# 2026-08-09：Steam 认证在线能力与升级验收解耦

- `GET /api/instances/stardew/junimo-update/apply` 响应形状不变；前端继续使用 `phase`、`progress`、`updatedAt`。当 `phase=verifying_auth` 时，`updatedAt` 作为当前阶段起点显示累计等待时间和自动重试说明。
- 后端不再使用 steam-auth Docker health 判断运行栈升级是否可接受，因为该 health 可能等待外部 Steam 在线连接。容器 running、目标 digest 匹配且 `/steam/ready` 合法即通过；响应中的未登录/无 ticket 是 warning，不是失败。
- 如果 `/steam/ready` 在有界超时内始终不可达或不可解析，仍返回 `auth_service_not_ready` 并进入原有回滚链路。前端提示“正在尝试连接”不改变失败、回滚或终态契约。
# FE-INSTALL-DIAGNOSTIC-MAPPING-1 联调契约（2026-08-13，completed，未发布）

- `GET /api/instances/:id/state` 可选返回 `installationDiagnostic`：`status=installed|incomplete|not_installed|unknown`、`requiredFiles=ok|missing|unknown`、`compose=ready|missing|invalid|unavailable`、`image=available|missing|unavailable`、`serverContainer=running|stopped|missing|unknown`、`control.static=match|mismatch|missing|unknown`、`control.runtime=match|mismatch|not_observed|invalid|unknown`、可选 `observedVersion`、必填 `expectedVersion`、`recommendedAction=install|repair_install|retry_start|diagnose` 和 UTC `checkedAt`。
- 前端不得再从顶层 `state=error` 推导“未安装”。明确 `status=not_installed`，或仍是 `uninitialized/admin_created/junimo_scaffolded` 且诊断未明确 installed，才显示首次安装入口；`requiredFiles=missing`、非首装场景的 Compose 缺失/无效或 image 明确缺失才显示修复；Control 明确缺失/不匹配/运行时无效进入诊断；Compose/image unavailable、`status=unknown`、`serverContainer=running` 却同时声称未安装/不完整，或其它证据矛盾必须 fail closed 为诊断，不得提交安装。
- `control.runtime=not_observed` 表示当前启动尚未观察到 Control 或容器未运行，不是版本错误；只有 `mismatch/invalid` 才能显示明确 Control 异常。后端未提供新字段时，旧前端兼容分支只把 `install_verification_failed` 且消息明确包含“运行文件不完整”映射为修复，验证器错误和其它运行错误仍不映射成重装。
- 活动 `stardew_install` job 优先显示安装/修复中并抑制首次弹窗。桌面安装表单是否可打开由共享分类器决定；移动端对应动作先写入 `install` 或 `diagnostics` 桌面路由再切换完整桌面壳。

# STARTUP-NEWGAME-DURABILITY-1 联调契约（2026-08-13，代码完成，待正式发布）

## 启动状态与 Control 证据

- 点击启动/重启后，server 容器已 up 但本次 `control/options.json` 尚未出现时，`GET /api/instances/:id/state` 必须保持 `state=starting`、`driverPhase=control_runtime_starting`。前端显示“仍在启动”和轮询进度，不显示版本不匹配、未安装或重装按钮。
- 只有合法运行快照的 `controlModVersion` 与内嵌期望值不同时，后端才写 `state=error`、`driverPhase=control_runtime_version_mismatch`。manifest/DLL/options 损坏使用对应 invalid code；一直没有快照到完整等待期限时安全停服为 `state=stopped`、`driverPhase=control_runtime_start_timeout`。
- 启动验收失败后无法确认 Compose 已停止时为 `control_runtime_cleanup_failed`；启动前旧 snapshot 无法清理时为 `control_runtime_snapshot_cleanup_failed`。这两项都是诊断/人工检查，不是重装证据。
- `/state.installationDiagnostic.control.runtime=not_observed` 与上述 pending 语义一致。前端可以展示等待/重试启动，但不得把 not_observed 翻译为“Control 版本不匹配”。

## 新建存档请求幂等

- `POST /api/instances/:id/saves/custom-new-game` 请求体保持 `NewGameConfig`，当前客户端必须同时发送非空 `Idempotency-Key`。key 是当前用户动作的稳定 request ID，不是存档名；在 202 被接受前，同一规范化配置的网络重试复用原 key，用户修改配置后使用新 key。
- 缺失或空 `Idempotency-Key` 返回 HTTP 428 / `idempotency_key_required`；后端不会为旧客户端随机生成 key，也不会创建 job、owner 或事务。
- 首次接受与相同 key+相同配置的重复提交都返回 HTTP 202 和相同 `{jobId}`。相同 key 绑定不同配置返回 HTTP 409 `new_game_request_conflict`；其它 active lifecycle/new-game owner 返回 409 `new_game_in_progress`；owner/transaction 缺失、损坏或身份不一致返回 409 `new_game_recovery_required`。客户端不得在这些 409 后生成新 key 自动重提。
- 正式前端测试必须固定 header 存在、相同配置复用、配置变化换 key、网络/服务端失败不清 key、只在 202 resolve 后清除本地 pending key。
- job payload 持久化 `operation=new_game/requestId/config`。刷新页面后以返回的 job ID 继续读取 `/api/jobs/:id`/stream；不得通过重新 POST 轮询进展。

## 单写入者和不可逆进展

- 事务开始即返回固定 `creationWriter`：没有完整旧存档为 `startup`，只观察 Junimo 启动自动创建，Panel POST 次数必须为 0；有完整旧存档为 `http`，只有稳定观察到同 transaction 的旧 `save-loaded` 基线与 API ready 后，Panel 才允许一次 `/newgame`。
- `status.json` 当前 transaction 的 `save-creating/creationObserved`、gameloader 离开初始 save、任意新存档目录三者任一出现即视为不可逆进展并持久化。即使 loader 已前进但 XML 尚未落盘，也必须禁止 POST；后续只能等待 loader/Control 与目录收敛到唯一 saveId。
- 一旦 `commandCalled`、`progressObserved`、`unknown` 或 `ambiguous`，UI 的“重试”必须回到同一持久事务/人工诊断，不能启动第二次创建。Stop、Restart、Restore 也应得到 owner 冲突，而不是取消建档 job。
- owner 以已 fsync 的完整 staging 目录做 no-replace 原子发布，并发请求只允许一个 winner。事务进入 `rolling_back`后，用户手动 Start 只幂等继续已持久的 quarantine/restore 计划；完成前零 ComposeUp、零 `/newgame`、零新事务。

## 四段耐久成功条件

1. Control 对同一 `transactionId + targetSaveId` 返回 fresh `state=save-loaded`，并明确 `newGameCreationObserved=true`。
2. 同一快照冻结的角色配置逐项与请求一致：身份/农场/最爱/性别/宠物、skin/hair/shirt/pants/accessory、三组 RGB 和 `isCustomized`；`customizationApplied/customizationVerified=true`，且更新后的 `players.json` 为同一 save 提供唯一、同名 host 证据。
3. Panel 使用事务中预留的同一 `commandId` 发布/恢复 `save-now`；Control 在调用保存前持久同 ID pending journal。容器在 running 回执后或 Saved/终态回执前中断，都只恢复该 ID。只有终态 `succeeded/errorCode=ok` 且 details 精确包含 `GameLoop.Saved + transactionId + saveId + expectedSaveId` 才通过；unknown/expired/错身份不完成也不换 ID。
4. save-now 之后主 XML 与 `SaveGameInfo` 连续两轮 SHA-256 稳定；主 XML 的全部配置字段、`isCustomized` 与运行时解析后的 `whichFarm` 均正确，SaveGameInfo 独立校验人物/农场，Control、loader 和唯一新目录最终指向相同 saveId。通过后才能提交 Mod profile、transaction success 并释放 owner。

Control `0.3.1` 是该契约的最低内嵌实现。运行栈清单、两份 manifest、DLL hash 和实际 `options.json.controlModVersion` 必须一致；只覆盖宿主 DLL 而未验证进程实载不能算通过。

2026-08-13 隔离 Docker 联调已实际跑通两条 writer：startup writer 不调用 `/newgame`，HTTP writer 在真实旧 active save 基线后只调用一次且旧档双哈希保持；两条都拿到上述四段证据。真实 Stardew 1.6 主 XML 的性别使用 `Gender/gender` 文本，旧 `isMale` 可为 `xsi:nil`；shirt/pants 的有效物品 ID 位于 `shirtItem/pantsItem.itemId`，旧标量可为 `-1`。后端磁盘门禁按该当前结构读取并兼容旧格式，错误 item ID 仍必须失败。当前 Control 编译/嵌入/清单 SHA-256 为 `3833769287e794d392296c52df760f8451b24a177243a0926d6f0ca9fd81b3ce`；候选镜像升级后仍需再次核对运行态 `options.json`，本条不能替代正式升级和生产验收。

## 宿主重启边界

- 本任务明确不提供“重启前 running 的普通游戏实例自动恢复”。宿主重启后 game container 保持关闭，用户手动点击启动；前端不得把这种关闭状态伪装为自动恢复中。
- 若重启前存在未结束 new-game owner，手动启动会恢复同一 request/config/transaction 并继续观察或完成耐久门禁。这个显式恢复不能扩展成 Panel 启动时自动 ComposeUp，也不能绕过 owner 再创建新事务。
- Panel bootstrap 的 required-runtime、Runtime apply 与 SMAPI apply 恢复必须保持 server/auth 关闭。它们可收敛未完成的静态替换或回滚，但即使持久化状态记录了 `ServerWasRunning=true`，也必须返回“请手动启动”而不 ComposeUp。存在 unfinished new-game owner 时连静态替换也禁止。

# RUNTIME-AUTH-HEALTH-PROBE-1 联调契约（2026-08-14，released in v0.4.17）

## 运行组件验收与在线业务分层

- Runtime apply、最终目标复验、旧栈回滚复验及 SMAPI apply 只通过容器内 `GET /health` 判断 steam-auth HTTP 服务是否正常；不会调用 `/steam/ready`，也不会在 health 失败后 fallback。`/steam/ready` 仍供实例 `steamAuthReady` 在线诊断、Steam 登录和 App Ticket 相关业务使用，两个职责不得重新合并。
- 硬验收必须同时满足：容器 state 为 `running`；实际 image ID 与本次目标 digest 精确匹配；`/health` 在单次短超时内返回；HTTP 为 200；body 是单一 JSON 文档；`status === "ok"`、`typeof logged_in === boolean`、`accounts` 为 JSON array 且三个字段都存在、非 null。Docker health 状态不是替代证据。
- `logged_in=false` 是成功的服务健康结果，同时在 `warnings[]` 追加 Steam 在线能力暂不可用的脱敏提示。它不阻断仅 Control 变化、Junimo 成对升级或 LAN/IP 模式。需要邀请码、在线大厅或 ticket 时，业务链再独立等待 Steam 登录。

## API 状态与失败语义

- 为兼容既有消费者，apply `checks[].name` 仍返回 `steam_auth_ready`；该名称当前表示“容器 running + digest + 严格 `/health`”，不表示 ready/ticket。阶段枚举和 apply-status JSON shape 不变。
- 失败码固定区分 `auth_container_not_running`、`auth_digest_mismatch`、`auth_health_unreachable`、`auth_health_timeout`、`auth_health_http_status`、`auth_health_invalid_response`。最后一次脱敏原因保留在 apply 错误、`causeCode/causeError` 以及回滚终态中；响应 body、Steam 用户名、密码、refresh token、session、ticket 和容器环境变量不进入 API 或日志。
- 上述任何硬失败都继续进入既有安全回滚。旧 auth 复验也必须通过相同 `/health` 契约，不能因回滚目标较旧而调用 `/steam/ready`；无法证明旧镜像契约时必须返回 `unsupported/auth_health_contract` 并在 mutation 前停止。

## 受控联调证据

- 本地 Docker fixture 的 `/health` 立即返回 `{"status":"ok","logged_in":false,"accounts":[]}`，`/steam/ready` 故意阻塞 60 秒。运行组件 apply 在短时间内成功，请求日志只出现 `/health`，证明没有触发登录端点；Docker health 同时可保持 `unhealthy`，仍不影响纯服务契约判断。
- 同一 fixture 用受控 mode 覆盖 `/health` 连接失败、短超时、404、500 和坏 JSON，全部得到对应稳定错误码并进入 `failed_rolled_back`；回滚后的旧 auth 再次以 `/health` 验收。另有单元表覆盖空 body、非 JSON、字段缺失、status null/非 ok、logged_in null/string/number、accounts null/object/string/number。
- 真实 `1.5.0-anxi.2` opt-in 只验证镜像 ID/digest 和 `/health` 严格契约，不配置或读取真实 Steam 账号。本文此前关于 Runtime/SMAPI apply 使用 `/steam/ready` 的段落属于旧版本历史行为，已由本节取代；实例在线诊断中的 `/steam/ready` 说明仍然有效。

# PLAYER-AUTH-MODES-1 跨端契约（2026-08-15，released in v0.4.19，included in v0.5.0）

## 配置接口

- 管理员 `GET /api/instances/:id/config/player-auth` 返回：
  - `mode: "none" | "global" | "role"`、不透明 `revision`；
  - `globalPassword` 只在 `global` 模式返回；
  - `roles[] = {roleId,name,configured,status?}`，以及 configured/unconfigured/orphaned 计数；
  - `runtimeMode/runtimeRevision/restartRequired/rolePasswordPatchReady/rolePasswordPatchDetail`。
- 管理员 `PUT` 请求为 `{expectedRevision,mode,globalPassword?,rolePasswordUpdates?,rolePasswordRemovals?}`。角色更新项固定为 `{roleId,password}`；响应与 GET 相同。revision 缺失或已过期返回 HTTP 409 / `player_auth_revision_conflict`，调用方必须重新 GET 后让用户确认，不得静默覆盖。
- 启用 `global` 必须有 1–128 字符密码；启用 `role` 必须存在非主机角色，且当前所有角色都有持久配置或本次 update。非法模式、空/超长角色密码、角色不存在、重复更新分别以稳定 400 错误返回；损坏的角色密钥/payload/内部 guard 以 409 fail closed，不自动降级为开放服务器。
- 兼容接口 `GET/PUT .../config/server-password` 仅服务旧消费者：非空 PUT 映射 `global`，空 PUT 映射 `none`；当前为 `role` 时 GET/PUT 均返回 `409 role_auth_mode_active`，绝不把内部 guard 当服务器密码返回。

## 保存、重启与运行时

- 配置写入成功不重启容器。运行中的 Control 仍使用启动时环境，因此前端必须依据 `restartRequired` 显示待重启；不能把 PUT 200 解释成当前玩家已应用新规则。
- `GET /api/instances/:id/password-status` 保留 Junimo 的 `enabled/authenticatedCount/pendingCount/timeoutSeconds/maxAttempts`，并增加 `configuredMode/configuredRevision/runtimeMode/runtimeRevision/restartRequired/rolePasswordPatchReady/rolePasswordPatchDetail`。服务器未运行仍返回既有 `409 server_not_running`。
- Control 只在 `role` 模式 patch Junimo 的 `TryAuthenticate` 输入，不跳过原方法。正确角色密码被重写为内部 guard，错误输入被重写为 fail-closed sentinel；全服和不设密码模式不改变上游认证。Panel 批准待认证玩家传入内部 guard，必须继续通过同一上游方法完成传送。
- 角色密码、HMAC verifier、实例密钥、内部 guard 和完整 `.env` 都不得进入 API、审计 metadata、Docker 命令结果、支持包或错误日志。审计 `instance_player_auth_update` 只记录 mode 及更新/删除/已配置数量。

## 发布前联调矩阵

- 正常路径：none、global、两个不同角色各自密码、Panel 批准认证。
- 关键边界：A 密码不能登录 B；缺少角色配置不能启用；错误密码仍累计 Junimo attempts/timeout；角色改名后密码仍有效；存档切换后的 orphan 记录不冒充当前角色。
- 恢复/幂等：同 revision 重复提交的第二个请求冲突；保存后未重启显示 configured/runtime 不一致；重启后 revision 一致且 patch ready；损坏 key/payload/guard 不开放访问。
- 升级：旧 `SERVER_PASSWORD` 实例升级后自动显示 global；无密码实例显示 none；上一正式版经 Panel Web 更新到候选后重新执行角色模式 E2E，并确认旧 API 在 role 模式不泄露 guard。

# 玩家列表 `lastSeen` 跨端契约（2026-08-15，released in v0.5.0）

- `GET /api/instances/:id/players` 中的 `players[].lastSeen` 表示“最后一次真实在线时间”，不是面板最后一次扫描到存档角色的时间。只存在于存档 XML、从未被在线快照观察到的离线角色必须省略该字段。
- SQLite `player_roster.last_seen_at` 是内部名册观测时间，仍可随轮询更新；只有在线快照会更新 `last_online_at`，API 仅从后者回填 `lastSeen`。前端不得用响应 `updatedAt`、浏览器当前时间或名册观测时间补造该字段。
- 真正在线过的角色离线后继续返回最后一次 `last_online_at`；本修复不改变玩家状态、在线人数、位置、收入或 API shape，也不需要迁移/清洗已有数据库。

# PLAYER-AUTH-SELF-ENROLL-1 跨端契约（2026-08-17，released in v0.5.3）

## API 与状态

- 本节覆盖旧 `PLAYER-AUTH-MODES-1` 中“role 必须存在非主机角色且所有角色已配置”的启用条件。`PUT /api/instances/:id/config/player-auth` 现在允许 `mode=role` 且角色列表为空或部分/全部为 waiting；`rolePasswordUpdates` 仅用于管理员代设/重置，`rolePasswordRemovals` 把已配置角色变回 waiting。
- `GET/PUT .../config/player-auth` 的 `roles[]` 增加 `credentialStatus: waiting|configured|error`，顶层增加 `credentialErrorCount`、`orphanedRoleCount`、`roleCredentialStoreReady` 与可选 `roleCredentialStoreDetail`。`configured` 继续保留兼容且只在 status 为 configured 时为 true；响应仍不包含密码、verifier、role key、内部 guard 或 store 正文。
- store 忙返回 `409 role_credential_store_busy`，store/marker/schema 损坏返回 `409 role_credential_store_invalid`，活动 save 身份不可用返回 `409 player_auth_save_unavailable`；跨 `.env`/store 回滚失败返回 `409 player_auth_transaction_rollback_failed`。这些错误均不得自动退回 none/global 或把 error 显示成 waiting。

## 游戏内首次认领与存档隔离

- 仅在 role 模式、策略/key/guard 有效、当前 active saveId 可验证、角色 ID 非主机零值且输入满足聊天密码约束时，Control 才允许首次认领。若当前 `saveId + roleId` 没有 verifier，本次输入先耐久写为该角色 verifier，再改写成内部 guard 交给同一个 Junimo `TryAuthenticate`；已有 verifier 时只做恒定时间校验，错误或串角色输入继续进入 fail-closed sentinel。
- 凭据按 saveId 隔离。切换、导入或回档到另一个存档后，相同数字 roleId 也不能自动继承旧档 verifier；当前角色列表之外的记录只计为 orphan，不作为当前角色已配置证据。
- legacy `.env` payload 只在新 store 尚未初始化时迁入当时活动存档。`role-passwords.initialized` 已存在而 store 缺失/损坏时禁止重新首次认领，避免崩溃或误删后账号被接管。

## 生命周期与前端联动

- start/restart 必须把四个 SAP 变量传入 server。block mapping/list 的旧 Compose自动迁移；role 模式的 inline/mixed environment 无法安全迁移时生命周期失败并报告 `player_auth_compose_migration_failed`。none/global 可保留原文件、记录 warning 后继续，因为它们不消费新增角色凭据变量。
- restart 使用 `docker compose up -d --no-deps --force-recreate server` 让配置进入新容器，不能用普通 restart 复用旧环境，也不能连带重启 `steam-auth`。
- 前端 restart pending 不能由请求前就存在的 `state=running` 清除；只有观察到 lifecycle job 并进入终态后才解锁。start 仍可在未观察到短任务时使用 stopped→running 作为完成证据。

## 发布联调矩阵

- 自动化：first enroll、重复正确登录、错误/串角色、Panel guard、空角色启用、清除后 waiting、save 隔离、legacy 迁移、store/marker 损坏、锁竞争、权限与事务回滚；Compose block/list/inline 以及 role fail-closed、none/global 兼容继续；restart 防重复提交。
- 真人客户端：两个客户端分别首次设置不同密码、重复正确登录、交叉密码失败、管理员清除后重新认领、Panel 批准、server recreate/Panel 重启后仍保持；用户已在正式候选前确认该矩阵通过。自动测试仍不能替代真人交互证据，最终能力已随 `v0.5.3` 发布。

# INSTALL-SMAPI-LIVE-PROGRESS-1 联调契约（2026-08-18，未发布）

## Job log → 前端进度

- 后端在现有安装 job 日志/SSE 中发送 `[smapi:download:progress:<downloaded>:<total>:<candidate>:<candidateCount>:<cached>]`，不新增 HTTP 接口或响应字段。五个字段依次为已写入字节、清单总字节、当前候选序号、候选总数、是否命中已校验缓存。
- 示例：`[smapi:download:progress:16777216:41889142:1:2:false]` 表示候选 1/2 已写入 16 MiB；`[smapi:download:progress:41889142:41889142:1:2:true]` 表示本地缓存已通过完整校验。前端必须拒绝负数、超过总量、非安全整数、0 候选或候选序号越界的 marker。
- marker 属于隐藏控制行，任务日志 UI 不展示；相邻 `[smapi]` 可读日志用于人工诊断。前端应从当前 active `stardew_install` job 派生进度，不能从旧 job 或已经完成的 SteamCMD/SDK 进度补造 SMAPI 百分比。
- 兼容旧后端：没有 marker 时仍显示 `smapi_installing` 活动状态和“检查缓存/状态会自动更新”，不能回退成 SteamCMD 100% 或静止的 Steam 认证卡。新后端缓存命中或 provider 成功时会给出 100% marker。

## 授权迁移边界

- 旧 SteamCMD volume 迁入统一授权卷不改变 API shape。迁移容器明确报告已存在/已迁移缓存后，本次非强制重新授权安装先走 username-only 缓存登录；失败再自动回退账号密码与 Steam Guard。
- UI 不应把“检测到缓存”表述为已登录成功；只有后端实际登录/下载后才写 `STEAMCMD_AUTH_COMPLETED=true`。更换账号继续清除所有认证卷并完整登录。

## 发布验收

- 真实 Docker E2E 至少覆盖：SMAPI 慢速分块下载时 marker 单调推进且页面字节/百分比变化；候选切换归零并显示新序号；缓存命中直接显示校验通过；最后字节到达后进入校验/写入而不是提前完成。
- 升级旧实例时准备仅有 legacy `config.vdf`、没有 `STEAMCMD_AUTH_COMPLETED` 的授权卷，点击修复后应先走免验证登录；再注入无效缓存，确认只自动回退一次完整登录并保留既有 Steam Guard 交互。正式候选、上一版 Web 升级和生产部署尚未执行。

# NEW-GAME-FARM-CAVE-CHOICE-1 联调契约（2026-08-23，未发布）

## 请求与默认值

- 新建存档 payload 新增 `farmCaveChoice: "vanilla" | "bats" | "mushrooms"`。新前端始终发送该字段；旧前端或其它客户端省略时，后端必须补 `vanilla`。未知值拒绝创建，不得静默降级成任一固定山洞。
- 后端将该字段写入事务专属 `server-init.json`，Control 只在 target marker、事务 ID、预期存档名和玩家身份精确匹配时消费。Junimo 接口与上游源码保持不变。

## 持久化终态

| 请求值 | 主存档 `caveChoice` | 事件 `65` | 蘑菇设施 |
| --- | ---: | --- | --- |
| `vanilla` | `0` | 不存在 | 不存在 |
| `bats` | `1` | 存在 | 不存在 |
| `mushrooms` | `2` | 存在 | 六个蘑菇箱和脱水机已就绪 |

- 真实 Junimo 创建链在 Control 首次处理前已预置蘑菇终态。Control 因此必须在受保护的新建事务内按上表精确转换，并在保存前回读；这不是允许普通已有存档在加载时被重写。
- Panel 只在 Control runtime status 的 applied/verified、事务/存档/时间和 snapshot 全部吻合，且落盘 XML 再次满足上表时宣布新建成功。任何不一致均进入现有恢复/失败链路，不能只信 UI 请求或单侧状态。

## 联调验收

- 前端 idempotency 必须证明重试时字段不丢失；后端覆盖缺省、三种合法值与非法值；Control 契约覆盖从 Junimo 蘑菇初态转到三种目标以及重复回读；真实 Docker E2E 至少连续创建两种固定选择并证明源游戏卷、旧存档及非目标资源不变。
- 2026-08-23 Docker Desktop 已连续创建蝙蝠洞和蘑菇洞，Control status 与主存档 XML 双重校验通过，旧主存档/`SaveGameInfo` 哈希不变。正式候选与上一正式版 Web 升级尚未执行。
