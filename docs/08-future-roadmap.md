# 2026-08-28 `!login` 凭据服务端转发抑制（未发布）

- [x] Control `0.3.8` 在 Junimo 完成认证之后精确拦截 `!login` 重广播，global、role、首次认领、错误次数、超时和传送语义不变；不修改上游、不要求客户端 Mod。
- [x] 分类覆盖大小写、前导/分隔空白与 Unicode 密码，并证明 `!loginfoo`、`!!login`、`/login`、正文命中和普通聊天不误伤；packet 读取或位置恢复异常时 fail closed，新增代码和日志不输出密码。
- [x] availability/detail 已进入 Control options/status，Panel 对缺失或 false 使用独立错误码停服；Control 两份 manifest、嵌入 DLL、stack identity、SHA-256 与升级夹具已同步。
- [x] C# 契约、本机精确游戏程序集 Control build、固定 SDK 6.0.428 + 真实 build `16826371` 的标准 Linux `/game` build、Linux Junimo 全包、Go vet/build、manifest/hash、生命周期和 ShellCheck 已通过；任务容器与缓存卷已精确清理。
- [x] 用户于 2026-08-28 明确确认未安装客户端 Mod 的实际联机测试通过，并授权恢复远端镜像构建。发送者自己的聊天框可能保留本地回显，这是设计边界，不列为服务端缺陷。
- [x] 标准 Linux `/game` Control 实编译已在本地任务容器完成，0 errors；远端 runner 无授权游戏 fixture，只负责 immutable embedded DLL/manifest 与运行契约，不把 contract/stub 冒充源码重编。
- [ ] 正式候选完成 fresh/restart、上一正式版与最老受影响支持版 Web 升级、unhealthy 回滚和升级后复验。
- [x] 首次 push-origin 候选 `33168728635` 因等待人工确认在镜像构建/推送前主动取消；人工确认后的手动候选 `33171764289` 通过代码门禁并构建镜像，但被只适用于 pre-`v0.6.0` 的 legacy Auth 夹具安全拦截，在 candidate push/proof 前退出且 Tag `33172703212` skipped。夹具已改为在 previous 前建立 unknown holder，并分别验证 pre-`v0.6.0` 与 `v0.6.0+` 的精确 Compose/fail-closed 边界。
- [x] 发布矩阵已把 `v0.6.1` 硬锁为 previous=`v0.6.0`、oldest=`v0.3.2`，并覆盖正确、缺 oldest、错 previous、错 oldest 的输入，防止不完整 proof 进入自动 Tag 链。
- [ ] 从修复后的新 commit 显式 dispatch `v0.6.1`、previous=`v0.6.0`、oldest=`v0.3.2`；成功后回填不可变 workflow、digest、耗时、正式提升和资源清理证据。

# 2026-08-27 已正式发布：v0.6.0 SteamCMD 默认安装与邀请码 opt-in

- [x] 最终不可变身份为 `v0.6.0@9c6d9c7696c6aa46f58405f0c02f187aa47111ba`；候选 `33073661356`、自动 annotated tag `33075599631`、正式提升 `33075622114` 全部成功。
- [x] 候选完成 full code gates、fresh/restart、`v0.5.13 → v0.6.0` unhealthy/healthy Web 升级与 `v0.3.2 → v0.6.0` 代表直升；proof artifact=`release-candidate-0.6.0-9c6d9c7696c6`（ID `9647693527`）。
- [x] annotated tag object=`c101344eaed4ff4f3d45d7b8949bdded8d6d69f1`，peeled commit 精确等于发布 commit；三仓 `0.6.0/latest` 六引用统一 digest=`sha256:e9c1613a7ffbd13d92d5a197d751cb5de6b08b65f74351e39a4ad0f9b4598d16`。
- [x] 正式 GHCR health/version smoke、OCI identity、GitHub Release 与四项部署资产摘要通过；发布后 evidence-only 提交不得移动 tag、重建候选或改变正式 digest。

# 2026-08-27 v0.6.0 最终安全预检：授权收尾与升级启动线性化（released in v0.6.0）

- [x] Auth 成功证据、invite enabled 与 `ready|cleanup_pending` 改为一次 `.env` 原子发布；one-shot 收尾失败保留 session 和完成态，启动/再次授权/Prepare 必须先在停服与 holder 全量安全分类成立后收敛，只删 holder、不删成功 session volume。
- [x] 修改 Steam 账号密码改为 driver 全局维护锁内最终重读：重新检查已安装状态、unfinished owner、active lifecycle/install/Auth job，并以无缓存 Compose 真值证明停服后才只写两个凭据键；并发 start 或旧状态快照不能穿透。
- [x] Panel 启动 required runtime coordinator 前逐实例 `Prepare`；迁移失败或 runtime/SMAPI recovery 活跃时只跳过对应实例。server health 验收修复为读取私有原始 stdout，公开结果仍脱敏。
- [x] 邀请码冷启动使用专用 `steam_invite_warmup_started_at` 运行代 marker：仅 enabled 的真实 start/restart/受控恢复刷新，普通 payload 更新不延长；后端 10 分钟内返回 generating、到期或坏 marker 返回 auth_unavailable。前端以 5 秒 × 125 次对齐，`starting → running` 重置并支持页面中途刷新；shared projection epoch、state→invite 串行与 deferred 逆序回归保证旧 state/invite 不覆盖 ready/disabled/终态，隐藏页不耗预算。
- [x] 权威 `auth_unavailable` 对普通刷新保持粘性，只有手动刷新或新 runtime 重开；job-finish 延迟回查在卸载时清理并有 mounted gate，避免终态回退或卸载后继续请求。
- [x] 候选 save-import 失败恢复断言已对齐 non-destructive scoped Stop：产品终态验证 stopped、零运行容器/零 Auth orphan，并允许 absent 或 exited/dead server 与默认 network；资源归零由后续精确 fixture teardown 承担。
- [x] 第五轮预演捕获真实 save-import 停服竞态：旧外层 30 秒与 Compose `--timeout 30` 同时到期会留下 maintenance journal。完整预算已统一为 150 秒并覆盖 Docker Down 的 2 分钟命令上限，独立 30 秒 fresh strict proof 覆盖 Ps 的 15 秒上限，且必须同时覆盖当前 intent 的 server/可选 Auth；只有严格停止且尚未跨过 FIFO 提交边界才逐字段恢复实例与 `snapshot_restored`，否则继续 fail closed 或写 `manual_required`。普通错误与 panic 均由同一恢复边界约束，Phase A 升级夹具同步改为比较完整实例 snapshot，而非只看 `state=stopped`。
- [x] Save-import 在启动前严格读取并将 `maintenanceSteamInviteEnabled` 冻结进 journal；后续错误、panic、Phase A、activation rollback 与重启恢复不再重读 `.env` 缩小服务范围。旧/缺 journal 保守尝试停 server+Auth，缺失证据禁止 snapshot restore；FIFO 成功后 journal reload 失败和首次读取失败均只停服、标记 recovery-required 且不重发，`snapshot_restore_pending` 也必须证明冻结范围内 Auth 已停。panic 原始值不进入 job 日志/API。
- [x] `Prepare` 在任何 journal 恢复前加入 active runner barrier，并重读权威实例；maintenance payload 持久化 `save_import_operation_id`。completed journal 到 SQLite running 的提交可幂等重试，只以 server 严格 running 作为基础运行权威，Auth 仅影响邀请码暖机；owner/payload 损坏、不匹配、多个旧 completed 候选或 orphan maintenance 均保守停 server+Auth 且不恢复 snapshot，唯一 owner 已确认后的探针/发布失败则零 stop 保留 committed runtime。running/running 重复发布为 no-op，不刷新暖机 marker，也不覆盖其它 phase。
- [x] 任一 save-import rollback substage 或缺少严格 no-effect 证据的 `manual_required` 优先级高于 activation resume；已证明 Phase A 无效果时只停服/恢复 snapshot，仍不 resume。swap/as-is/completed 三类 confirmed-step panic 分别执行冻结范围回滚、停服并保持 manual、只重试 running publication且不回滚。候选脚本对提交前后的 `state`、`state_message`（区分 NULL 与非 NULL 原始字节）、`driver_phase`、`driver_payload` 使用 SQLite BLOB hex 精确比较，不再用 JSON/string 近似等价代替数据完整性证明。
- [x] 正式 promotion 全局串行，annotated tag 唯一绑定 candidate workflow ID 与 digest；promotion 只下载该 run 的 proof，existing-tag 链也必须匹配同一身份。精确 version smoke 后、首个 `latest` copy 前再次核对 `origin/main` 与 GitHub latest；仅该绑定不变且同一 version 已创建 Release 的失败重跑可幂等修复 `latest`，其它 tag/commit/proof/digest 均 fail closed。
- [x] 最终 owner/恢复修复后的任务专属 Linux Go 1.25 已通过整仓 `go test ./... -count=1`（Junimo `208.566s`、Web `55.980s`）、`go vet ./...` 与 Panel build，精确 owner container/两个 cache volume 均为 0；Node 24 洁净卷已通过 `npm ci`、production audit（0 vulnerabilities）、19 项前端状态/响应式脚本和 `npm run build`（Vite 8.0.16，149 modules），容器与两个前端卷同样归零。
- [x] 第五轮旧候选因真实 recovery deadline 缺陷安全停止且作废；修复后的最终同一不可变候选 `33073661356` 已重新完成 `v0.5.13 → v0.6.0` unhealthy/healthy Web 升级和 `v0.3.2 → v0.6.0` 代表直升，并进入自动 tag、同 digest promotion 与 GitHub Release。

# 2026-08-27 v0.6.0 移动端终态与普通用户授权文案收口（released in v0.6.0）

- [x] 移动端 start/restart pending 改为复用桌面纯状态机：观察到 active lifecycle job 后，job 在成功、失败或取消终态消失都会解除“启动中…”，restart 不受提交前 running 快照误判。
- [x] 普通用户遇到 Steam 邀请授权失败时改为“授权异常，请联系管理员”；管理员继续保留重新授权操作，后端 admin-only 权限不变。
- [x] lifecycle 纯状态与 responsive 源码回归已覆盖上述边界；候选 `33073661356` 已执行完整前端回归与 production build，同一镜像的升级夹具验证权威 DTO/迁移状态。移动端实际渲染来自发布前本机 Browser，不属于候选 proof。

# 2026-08-26 v0.6.0 邀请码 opt-in 与升级兼容收口（released in v0.6.0）

- [x] 缺少 `steamInviteEnabled` 的旧实例改用强 Auth 证据迁移：SteamAuth completed、`steam_auth_done` 或非空邀请码才保留 enabled；单纯安装/运行状态、SteamCMD 完成/cache 均落 explicit false，已有 explicit true/false 始终优先。
- [x] disabled 的 runtime apply/dry-run/required/recovery 全部收口为 server-only Compose inspect/validate/ps/stop/up/health/rollback，schema 4 中断恢复零 Auth；schema 1～3 旧事务保留兼容恢复。
- [x] 旧 Compose 会幂等且仅删除 `server -> steam-auth` 依赖；`STEAM_INVITE_RUNTIME_SCOPE_VERSION=1` 使旧 disabled 的精确 Auth holder/session 只收敛一次。删除前会完整 inspect 同卷全部 holder，仅接受权威 Compose Auth 或带 Panel owner/project label 的一次性 Auth；未知/伪装 holder 时 fail closed，保留全部 holder/volume 且不落 marker，enabled 零 Docker 清理并保留 session。
- [x] 已补 server-only 无效 Auth 配置真实 Docker E2E、Linux Auth TTY/PTY/敏感信息/精确清理 E2E、disabled apply/rollback/recovery 测试，以及 SteamCMD-only/强 Auth 两类升级 fixture；候选 fixture 不得再在 apply 前预清理旧 Auth runtime。
- [x] 前端安装任务在 202/409 返回新 `jobId` 时先更新 URL 再接管日志，避免旧查询参数覆盖；SteamCMD 主链、LAN/Invite 分卡、disabled 零轮询、原版小屋与倒序日志规则保持统一。
- [x] v0.6.0 发布身份已 fail closed：带 `[manual-release-candidate]` 的 final commit 跳过 push 自动 `0.5.14` 候选和无 proof Tag 链，只接受同 SHA 的手动候选；候选/Tag/promotion 共用矩阵校验，强制 `0.3.2 < 0.5.13 < 0.6.0`，零 patch 漏 oldest 不得发布。
- [x] 候选 `33073661356` 从上一正式版 `v0.5.13` 完成 unhealthy 回滚、健康 Web apply、running 旧 disabled Auth/session 自收敛和升级后专项；全程使用真实 Panel Web 更新链。
- [x] 同一候选从受影响最老支持边界 `v0.3.2` 完成真实 Web 直升，并验证 SteamCMD-only→disabled 和强 Auth→enabled/session 保留。
- [x] 同一不可变候选的 fresh/restart、代码门禁、真实 Docker 与 proof artifact 全绿；自动 annotated tag、三仓同 digest 提升、`latest`、正式 smoke 与 GitHub Release 已完成，精确耗时、矩阵与资源清理证据见 `docs/09-image-build.md`。

# 2026-08-26 启动期邀请码等待态修正（released in v0.6.0）

- [x] 邀请码文件尚未生成时不再因 `cat` 非零退出误报 Auth 异常；缺文件与 starting 阶段执行瞬态统一为 `generating`。后续 release preflight 又为 running 增加专用 10 分钟暖机 marker，只有到期或 marker 不可信才返回 `auth_unavailable`。
- [x] 桌面总览、服务器摘要和移动首页统一显示“等待中…”并在有界预算内轮询；页面在暖机途中刷新会恢复轮询，后端权威异常或预算耗尽后才显示“Auth 异常（直连仍可用）”，且不把它等同于 session 失效。
- [x] Driver/HTTP DTO/前端状态与响应式测试通过；现有本机隔离实例真实启动和移动端重启均复现“等待中 → 新邀请码 ready”，LAN 直连持续展示，console 零异常且 390px 无溢出。
- [x] 发布前该本地阶段的 native `dev` 后端与 Vite HMR 保持运行供继续测试；该阶段没有创建新预览容器、镜像、branch、tag、Release 或 push。

# 2026-08-26 原版小屋默认与安装日志最新置顶（released in v0.6.0）

- [x] 新建存档 UI、后端空值归一、server-settings 写入以及运行时设置前后端 fallback 全部改为原版 `vanilla/None`；只有用户明确选择“堆叠”才继续使用兼容 wire 值 `recommended/CabinStack`。
- [x] 已有 `CabinStack`、`FarmhouseStack`、`None` 保持原样，不做启动迁移；hidden `FarmhouseStack` 和完整三值 API 兼容不变。
- [x] 安装页只在渲染层把最近 50 条日志按 sequence 降序展示，标题提示“最新日志在最上方（倒序显示）”；不强制抢回正在阅读旧记录的滚动位置，API、SSE、阶段解析和完整任务日志页仍按旧到新处理。
- [x] 已补 Junimo/Web 默认与兼容回归、前端 cabin/install/responsive 回归；发布前该本地阶段继续复用 Go/Vite 热预览，不新建容器、不触碰正式镜像/tag/Release。

# 2026-08-26 邀请码授权卡死与基础安装状态隔离（released in v0.6.0）

- [x] 现场 `Choice [1]: 1` 卡死确认是凭据模式误进 SteamAuth `setup` 后等待隐藏的 username/password 输入，不是正常下载或 Steam 网络进度；授权现按需检查/拉取 Auth 镜像，再用共享回退凭据驱动无游戏下载的 `serve` 一次性容器，QR 使用只做授权且自行退出的 `login + 2\n`，两条路径保存 session 后都停止且不再进入 `DownloadAll`。
- [x] Auth-only 只挂 `_steam-session`，使用 scratch `GAME_DIR`；已经 `ready` 且登录完成的 Auth session 受幂等保护，重复启用不创建 job、不清 session。failed/pending 重试、账号不一致与启动恢复统一使用全量 holder 安全分类器，未知 holder 时零部分删除；新 Linux/Windows one-shot 均带 Panel owner/project label。真实登录成功后以一次原子写入同时保存 completed、enabled 与 `ready|cleanup_pending`；精确收尾成功为 `ready`，失败为 `cleanup_pending`，后者可令 job 报错但不得撤销 session。SteamCMD cache、授权卷和 game-data 始终保留；disabled 零探针且不扰动运行 server。
- [x] 邀请授权从镜像拉取到 Guard/成功/失败全程保留基础实例 state，快照恢复只作中断兜底；独立 Auth job 不进入基础安装 classifier，旧无诊断 Auth 失败只显示诊断态，因此不会再误弹“游戏没有安装”。
- [x] accepted/active-conflict `jobId` 会立即进入安装页 URL 并接管日志，旧 dashboard 快照不能夺回选择；自动任务切换清空旧日志，倒序窗口不再在每条新日志到达时强制滚动。
- [x] 最新 Windows 定向 Auth/Guard/pull-failure/Web 测试、`go vet ./...`、`go build ./...`、前端 install/cabin/responsive 回归与 production build 已通过。Windows 整仓复跑仍命中已知 NTFS `0666/0640` 权限差异；同次一个无关存档导入异步用例发生一次竞态，精确单测复跑通过。用户随后在现有本机热预览自行完成 Steam Guard：session 保存、一次性 Auth 停止、基础安装保持完成且 SteamCMD cache 不变；代理未代填或记录凭据，也未新建容器。该阶段尚未验证的最终邀请码，后续已由上方桌面启动/移动重启 Browser 证据补齐；仍不属于候选 Browser 证据。

# 2026-08-26 SteamCMD 默认安装与 Steam 邀请码按需启用（released in v0.6.0）

- [x] 全新实例显式 `steamInviteEnabled=false`；旧实例只有历史 SteamAuth 完成态、`steam_auth_done` 状态/阶段或非空邀请码等强 Auth 证据才兼容迁移为 enabled，普通安装/运行/SteamCMD 证据迁移为 false，显式 true/false 优先。
- [x] SteamCMD 成为 Stardew 413150 + SDK 1007 + SMAPI/Junimo/Control 主安装/修复链；disabled 安装零 SteamAuth pull/create/run/probe，SteamCMD 仍为一次性工具且不进入 runtime 供应链或版本 pin。
- [x] SteamCMD 与 SteamAuth 共用同一份回退 Steam 账号密码，安装页常驻入口统一为“修改 Steam 账号密码”；管理员停服后只提交独立 `PUT .../steam-credentials` 的用户名/密码，表单不再要求 VNC/镜像，成功不创建 job、不导航日志且立即刷新 state。SteamCMD cache 与 SteamAuth session/完成状态继续隔离并优先复用；修改账号密码不得清除或重建成功 cache，也不得使成功 session 失效，只有运行时实际判定授权失效后才使用当前保存的凭据，邀请码 failed/pending 重授权也绝不清 SteamCMD cache。缓存失效转完整登录或 139 重试会重置单次尝试状态，SteamCMD 客户端自更新进度不会再被误记为有效授权。安装页已按用户反馈删除共享凭据、一次性 Auth、等待与授权失败的重复提示条，只保留操作入口、交互状态和任务日志。
- [x] 管理员按需启用 Steam 邀请码，复用 QR/Guard/job logs；独立一次性 auth 登录容器保存 session 后停止，auth job 从运行开始即保持基础实例 state，失败/中断只更新能力状态，快照恢复作为兜底，不清理 SteamCMD cache 或 game-data。ready 状态禁止重复登录，普通用户入口完全隐藏。
- [x] Compose 移除默认硬依赖；生命周期、runtime/SMAPI、save-import、诊断按 enabled 分流，disabled 为 server-only 且邀请码后台工作为零。disabled 支持包只调用 server-only Compose ps，接口不可用时写入条目错误，绝不回退全量 Compose 暗中解析/验收 Auth。
- [x] 桌面总览、服务器摘要和移动首页统一以权威意图渲染；“局域网直连”独立常驻，Steam 卡只在 enabled 显示并覆盖等待、失败、停服、生成、就绪和 Auth 异常。
- [x] 安装页与移动 installation-state 改为 SteamCMD 主链；`stardew_steam_auth` 不再污染基础安装失败。setup-status 同步运行时 `DEFAULT_INSTANCE_ID`，支持真实隔离部署。
- [x] 发布前本地 Go 定向/整包 test-vet-build、前端状态/响应式/build、真实 Docker service-scope E2E、fresh Panel health/version/setup 与应用内 Browser 桌面/移动验证均通过；隔离预览保持运行。该本地阶段没有创建候选、tag、latest、Release 或正式镜像，后续正式链证据见本文件顶部。
- [ ] “直连后再启用邀请码导致原角色不可见”及角色迁移工具明确延期，不属于本次实现。

# 2026-08-23 已发布于 v0.5.13：Control-only 认证健康改为有界告警并拆分 UI 阶段

- [x] 生产 `v0.5.12` 只读确认约 400 秒停顿来自未变化 `steam-auth-cn` 的 Steam 自动登录重试；真正 server/SMAPI/Control 验收约 24 秒，旧 UI 因阶段合并误显示为 SMAPI 验证。
- [x] Control-only 仅在 server/auth/JunimoServer Mod 均未变化时允许一次默认 2 秒 `/health` advisory；容器 running 与精确 image ID 保持硬门禁，HTTP 状态行/响应头/响应体各有 1 秒容器内读取上限，最终验证不重复探测，`/steam/ready` 始终禁用。
- [x] server、auth 或 JunimoServer Mod 变化继续执行严格 `/health`、认证卷保护和回滚；Control-only 容器停止/digest 偏离也安全失败。运行态、停止态、单次告警探测、身份硬失败与真实 Docker timeout/严格错误矩阵均已通过。
- [x] Web 新增公开 `verifying_auth` 阶段；前端顶栏、总览和详情时间线与 `verifying_runtime` 分开展示并由同一 selector 派生，状态机回归已覆盖。
- [x] Linux Go 1.25 整仓 test、Windows vet/build、前端 panel-update/响应式回归和 production build 全绿；Windows 整仓唯一失败为既有 NTFS mode 差异，任务专属 Go cache/container 已清零。
- [x] 首轮候选/Compatibility 在真实 auth probe integration 暴露后台 watchdog 竞态后停止且未提升；修复后的候选 `32648758732`、Compatibility `32648758687`、自动 annotated tag `32649334502` 与正式提升 `32649344923` 全部成功。`v0.5.13@be25fb3a`、GitHub Release 和三仓 `0.5.13/latest` 使用同一已证明 digest。
- [x] 官网首页/changelog 已回填并由 Docs Portal `32649797827` 发布，docs-only Compatibility `32649797822` 成功；线上两个页面均为 200 且包含 `v0.5.13` 与认证/SMAPI 独立阶段说明，没有触发新候选或移动 tag。

# 2026-08-22 已发布于 v0.5.11：Steam 密码错误恢复与常驻更换账号入口

- [x] 生产 v0.5.10 只读诊断确认完整登录返回 `Logging in user ... Invalid Password` 和 exit 5；旧 switch 被通用进度 case 抢先命中，导致错误发布 `error/steamcmd_failed`。
- [x] 后端把具体凭据失败标记置于通用登录进度之前，组合行稳定发布 `steam_auth_failed/credentials_required`；新增缓存回退和组合行回归，Windows 定向、Linux Junimo 全包（56.180s）、vet/build 全绿。
- [x] 管理员安装页新增常驻“更换 Steam 账号 / 重新认证”，完整新凭据表单发送既有 `forceReauth=true`；历史错误状态、部分安装和诊断未知时也可进入，运行中保持禁用，游戏文件/存档保留。
- [x] 前端状态、响应式和 production build 全绿；应用内 Browser 在部分安装证据场景完成桌面/390px 交互，常驻按钮、3 个输入项和强制提交按钮可见，无横向溢出、遮罩或 console warning/error。
- [x] 正式候选 `32575311262@a9e186249a5c70c2e6fe45b7ed10a09db0b0c8bb` 与 Compatibility `32575311243` 全绿；selected gates 完成 backend、frontend、真实 Junimo integration、website build，immutable image 完成 fresh/restart 和 `v0.5.10` Web unhealthy/healthy。真实 Steam 错误登录未使用生产/长期凭据注入，专项由确定性 driver 回归、状态/bundle 回归和发布前 Browser 交互覆盖。
- [x] 自动 Tag `32575807110`、正式提升 `32575818623` 与 GitHub Release 成功；annotated tag object=`d8bf5075d57f7aaf1b834ad62e12418a2db67ab7`，三仓 `0.5.11/latest` 六引用统一 digest=`sha256:10c9813328370ae8ac92f11271fb76cd03787aab3b7f7fd523f20d66dfae8876`，GHCR health/version smoke 和四项资产通过。
- [x] v0.5.10 本机中断候选的两个 tar、唯一 exited DinD 容器及其匿名卷已按精确 path/name/owner 清理；复核 artifact directory、container、volume、network 均为 0，未 prune 或触碰其它任务资源。
- [x] 官网 docs-only `f545c169ded5edb11f8b2a1b1aad289bea77532b` 已补齐 v0.5.8～v0.5.11 changelog 与首页版本；Pages `32576397782`、Compatibility `32576397780` 全绿，线上首页/changelog 为 200 且正文命中，没有触发新候选或改变 v0.5.11 digest。

# 2026-08-20 已发布于 v0.5.10：候选备份调度测试复用原子事件契约

- [x] 第二次正式候选 `32378153924` 在 build/push/artifact 前命中连续游戏日备份用例；同 SHA Compatibility `32378153951` 成功，失败候选保持不可提升。
- [x] 确认生产 Control 先写隐藏临时文件、完整后原子移动成 `.json`；只有测试用 `os.WriteFile` 直写消费者可见最终文件，产生生产不存在的半写竞态。
- [x] 并发夹具改用已有原子 JSON writer，不增加 scheduler 间隔或放宽结果预算；函数级核验后的 Linux Go 1.25 精确用例 `count=100` 全绿（11.896s），整仓 test/vet/build 随后全绿。
- [x] 最终候选 `32380002010` 与 Compatibility `32380002025` 全绿；自动 Tag `32381115159`、正式提升 `32381136325` 成功，三仓 `0.5.10/latest` 六引用统一 digest=`sha256:f0887c383d0043934b0023cc150e732f6d514e789df2d81c786297c122dc3bb4`，正式 smoke、版本接口与 GitHub Release 已核对。
- [x] 2026-08-22 权限恢复后已删除本地中断候选的两个精确 tar，并移除同 owner 唯一 exited DinD 容器及匿名卷；复核 daemon container/volume/network 与 artifact directory 均为 0。该中断链没有被记作 v0.5.10 或 v0.5.11 候选证明。

# 2026-08-20 已发布于 v0.5.10：runtime-update 成功终态原子包含 cleanup warning

- [x] 正式候选 `32376230460` 在 selected code gates 命中 `TestRuntimeUpdateApplyImageCleanupFailureIsWarning`；失败发生在 build/GHCR/artifact 前，旧 run 不重跑、不提升。
- [x] 根因是正常/重启续作成功链先发布 `succeeded`，再做 snapshot/旧镜像 cleanup 并补 warning，reader 可读到不完整 terminal；两条路径现统一为先 cleanup 汇总，再由既有 finish 一次性发布终态与审计。
- [x] 回归显式阻塞旧镜像清理，证明阻塞期间状态非 terminal；释放并注入删除失败后仍 `succeeded` 且 warning 完整。Linux 定向 `count=20`、整仓 test/vet/build 全绿。
- [x] 修复后的本地 `0.5.10@96e5161255e6` 完整候选全绿，包含 fresh/restart、`v0.5.9` unhealthy/healthy、升级后 legacy 与存档 Phase A 专项。
- [x] 修复后的最终 commit 已重跑正式 `v0.5.9 → v0.5.10` 全链，只从新候选 proof 自动 Tag/提升；失败候选没有构建、推送或复用。

# 2026-08-20 已发布于 v0.5.10：存档导入真实候选门禁补齐

- [x] 审计确认 `v0.5.9@0657ff01f121` 已自动正式发布，但候选没有执行已声明的 exact target invisible 与 FIFO submitted/no disk effect 两条真实 Docker 场景；旧 tag/digest 保持不可变，采用 `v0.5.10` fix-forward。
- [x] 升级 E2E 已加入 exact `saves info` 不可见时 pre-submit fail closed，以及真实 FIFO 接受一次 import 但零落盘后的脱敏诊断、复合 no-effect、snapshot restore 和下一管理员 mutation strict cleanup；preimport/hash/pointer/备份均有保持断言。
- [x] 真实 Junimo `.125` 测试改为导入无 world ID 的非规范 ZIP，preview 先 canonicalize，再完成 finalizer/解绑/durable restart；官方 TestClient 按名称选择 `OriginalOwner` 并睡到次日，直接证明原主机可选。真实链 `223.39s` 通过且 owner 资源为 0。
- [x] Bash 语法、ShellCheck、定向 Go、integration 编译以及任务专属 Linux Go 1.25 整仓 test/vet/build 通过；变更范围只有候选脚本、真实 integration test、长期文档和错题本，没有运行代码、前端/schema/Compose/runtime 资产变化。
- [x] 本地 `0.5.10@c45f0e09afa5` 完整候选全绿：fresh/restart、`v0.5.9` unhealthy rollback/healthy apply、升级后 exact-target invisible、FIFO no-effect、脱敏诊断、下一管理员 mutation 恢复与既有受影响链全部通过。
- [x] 正式 `v0.5.9 → v0.5.10` unhealthy/healthy、升级后专项、自动 Tag/提升全绿；三仓 digest/`latest`、OCI 身份、版本接口、GitHub Release 与四项资产已核对，完整证据见 `docs/09-image-build.md`。

# 2026-08-20 已发布于 v0.5.9：非规范上传目录统一为运行时 saveId

- [x] 生产 v0.5.8 证据确认 swap Layer A 已改盘，但 3 字符上传目录与 Stardew/SMAPI 解析出的 `<prefix>_<uniqueIDForThisGame>` 不一致；Junimo finalizer wrong-save guard 清 intent、计数不前进，Panel 随后正确完整回滚，原主机未丢失。
- [x] `SAVE-IMPORT-RUNTIME-IDENTITY-NORMALIZATION-1`：preview 在私有临时树中读取世界 ID，将目录、主文件和 `_old` no-replace 规范化；preview/token/journal/FIFO/runtime 从接管前即使用唯一 canonical saveName，不改 XML。
- [x] 生产同形态无后缀中文目录专项以及正常、显式目录、GBK preview 回归通过；规范名继续通过安全路径/命令 token 门禁。
- [x] 任务专属 Linux Go 1.25 整仓 `go test ./... -count=1`、`go vet ./...`、`go build ./...` 全绿。
- [x] `origin/main@0657ff01f121` 已推送并热更新到生产；Panel/DB/Docker health 全绿，无 active job、unfinished journal 或 pending intent，原镜像与可执行 rollback 保留。
- [x] 产品代码已随 `v0.5.9@0657ff01f121` 正式发布；真实候选的“非规范目录 → finalizer → 自动解绑 → durable save → 重启仍可选原主机”门禁已在上述 `v0.5.10` 补丁中实现并本地通过，待完整候选证明。

# 2026-08-20 已发布于 v0.5.9：终态 no-effect 普通操作自动解锁

- [x] 生产 v0.5.8 第二个现场已确认 job 终态失败、active jobs=0、Phase A 主文件/pointer/pending 均严格零效果，但通用 mutation mutex 在 handler recovery 前返回 busy；不可变备份与 dry-run 后已按 driver cleanup 契约热修，Panel 健康且未重启，preimport/receipt 保留。
- [x] `SAVE-IMPORT-TERMINAL-MUTATION-RECOVERY-1`：受 import mutex 保护的请求先认证，仅管理员在 busy 判定前收敛 exact-owner + terminal + strict no-effect 事务；未认证、非管理员、active/ambiguous/effect-bearing 继续零 cleanup/fail closed。
- [x] maintenance 用 `saves info <exact-target>` 证明 Junimo 真能读取 staged target 后才提交；Phase A 无效果会在 Down 前有界采集、脱敏 `server-output` 到 journal，日志仍不参与成功判定。
- [x] 管理员 mutation/未认证边界、target readiness、日志 platform ID 脱敏专项通过；Web 全包、Linux Junimo 全包、Linux 整仓 test、vet/build 全绿，任务容器/卷/临时恢复制品清零。
- [x] 代码已随 `0657ff01f121` push 并进入生产热修，最终 Panel/DB/Docker health 与事务终态复核通过。
- [x] 产品代码已随 `v0.5.9@0657ff01f121` 正式发布；上述 `v0.5.10` 补丁已把 target-invisible pre-submit 和 FIFO no-effect + 下一 mutation 自动清理两条真实 Docker 专项接入升级候选，待完整远端证明。

# 2026-08-20 已发布：v0.5.8 Phase A 无落盘效果导入恢复与生产热修

- [x] `SAVE-IMPORT-PHASE-A-NO-EFFECT-RECOVERY-1`：当 FIFO 已尝试但完整 pre/after 证据严格证明主存档 hash、活动 pointer 和 Junimo pending intent 均无变化时，恢复 pre-maintenance 实例快照并允许现有 fingerprint-guarded cleanup；证据缺失、伪造标签、磁盘漂移或 upstream confirmed 继续人工恢复。
- [x] Panel 重启可续作 stop-and-restore 与 `snapshot_restore_pending`；当前进程 no-effect 失败也先恢复实例快照再返回原 command failure，不重放 Junimo import。
- [x] durable upload 的 exact job binding attach 改为零写幂等，保留 `binding mtime < jobs_cleared` 的旧现场审计证明；补字段时才写入并返回真实新 mtime。
- [x] 生产 v0.5.7 已在完整备份、SQLite integrity、Compose 空和 strict disk/fingerprint 证明下完成一次性收敛；journal/token 清零、receipt 落盘、preimport 保留、Panel healthy，原正式镜像未替换。
- [x] Windows 精准专项、Linux Junimo/Web 全包、vet/build 通过，任务容器/缓存卷清零；Compatibility `32338102593`、候选 `32338102590`、自动 Tag `32338764800`、正式提升 `32338783267` 全绿，annotated `v0.5.8` 指向 `8d5fe360c04240d7ccb9f9ac61ffecaed128627c`，三仓 `0.5.8/latest` 统一 digest=`sha256:f192d7840e564fe6c0bba6ab895e1533764c21e53257fcbde3cea01b75d59b66`。
- [ ] 发布审计发现候选只复验了升级后的 legacy jobs-cleared pre-submit 导入，没有加入“FIFO 已写且完整证据严格 no-effect”的真实 Docker 升级场景；下一次相关发布前必须把该场景固化进候选并实跑，生产一次性恢复与 Go 回归不能替代这条门禁。v0.5.8 tag/digest 已不可变，不移动、不重建。

# 2026-08-20 已发布：v0.5.7 失败存档导入自动自修复

- [x] `SAVE-IMPORT-AUTO-RECOVERY-1`：新上传 preview/commit 会自动识别并收敛 job 已终态、三方身份一致且可严格证明从未提交 Junimo 的旧导入；不再要求用户保留原 token、关闭弹窗后返回手工取消或理解 recovery journal。
- [x] 自动链复用 strict offline、maintenance 恢复、FIFO 未尝试、pointer/全树 fingerprint 与 no-replace 门禁，并以 hash-keyed receipt 保证 journal/token 删除中断可继续；submitted/unknown、身份冲突或磁盘漂移继续 fail closed。
- [x] preview 在读取 ZIP 前完成恢复/阻断，commit 再覆盖任务状态竞争；专项已覆盖正常下一次上传自动成功、旧 journal/token/暂存目录清零、preimport 保留、无原 token receipt 收敛，以及 FIFO 模糊证据下 409 与零删除。
- [x] 支持包确认 v0.5.5 用户曾清空终态任务、但 unfinished journal 仍锁住启动/选档/上传；兼容恢复现要求 exact token+journal job binding、绑定记录早于后续 `jobs_cleared` 审计且当前零活动导入，现行任务中心也会在删除前收敛安全失败或对模糊事务 409 保留证据。
- [x] Windows 定向专项及 jobs/storage 通过；任务专属 Linux Go 1.25 完成 Web 全包、全仓 test/vet/build，测试容器与缓存卷清零。正式候选 `32284304749` 从 `v0.5.5` 完成 unhealthy 回滚、healthy 升级，并在升级后的真实 Docker Panel 重验 legacy jobs-cleared 自动恢复。
- [x] `v0.5.6` 提升在认证或 registry 写入前被 runner 的冗余 apt 工具安装阻塞，旧 annotated tag 保留但未发布；删除即时安装、改用预装 Skopeo 探针后，从新 commit 完整重建 `v0.5.7`，没有复用旧候选。
- [x] Compatibility `32284291347`、自动 Tag `32285201579`、正式提升 `32285223565` 和 GitHub Release 全部成功；annotated `v0.5.7` 指向 `f7cedaa31e9db71aa2291c8aa06ea857046caf81`，三仓 `0.5.7/latest` 六引用统一 digest=`sha256:0b2dbe649fd6ce7acce797e170fec9ad2f1da9f00730afe1bb39b4ea8d586290`，独立 health/version/OCI smoke 与资源清零通过。

# 2026-08-17 已完成：v0.5.3 聚合正式发布

- [x] 版本范围固定为 v0.5.2 以后本地 main 的全部功能：角色密码首次登录自助设置、Nexus 一键安装锁定最新版本、已安装 Mod 一键安全更新、建档后联机人数上限、增强诊断 ZIP、Mod 删除后刷新纠正与全前端刷新数据流审计。
- [x] 官网更新日志与首页已准备 v0.5.3，并按用户要求在密码功能中特别感谢群友「石头磊」、在 Mod 功能中特别感谢群友「鹈鹕镇的热心市民」。
- [x] 真实 Chrome + 扩展 0.1.8 已完成 Content Patcher `2.9.0 → 2.9.1` 一键更新，以及 Content Patcher `2.9.1/file_id=160463` 提交后再安装 Elle's New Barn Animals `1.1.3/file_id=34408` 的缺前置 ZIP 批次；manifest、旧配置/启用状态和零临时残留均已核对。
- [x] 用户于 2026-08-17 确认两个真人客户端角色密码矩阵通过：各自首次认领与重复正确登录、交叉失败、管理员清除后重认领、Panel 批准、server recreate/Panel 重启保持。
- [x] 首版聚合提交已同步 `origin/main`；自动候选 `32033542812` 的代码门禁通过，fresh production bundle 因运行设置已抽到共享懒加载块、旧门禁仍搜索桌面/移动控制块而在推送候选前安全失败。
- [x] 共享 `ServerRuntimeSettingsDialog` 产物门禁修复提交 `ede7fa3` 已同步；受控候选 `32034798704` 完成全部代码、远程制品、真实 runtime、fresh/restart、v0.5.2 Web unhealthy 回滚/healthy 升级和资源清理门禁。
- [x] 自动 Tag `32035705749` 与正式提升 `32035725325` 成功；annotated `v0.5.3` 精确指向 `ede7fa3`，三仓 `0.5.3/latest` 六引用统一 digest=`sha256:400ad1e92dc84bc62530d38e08ec2ddb20d4d385ee01dc2b35808d23d91bd1f8`，正式冒烟和 GitHub Release 四项资产通过。

# 2026-08-17 已完成：已安装 Mod 一键安全更新

- [x] `FE-NEXUS-MOD-ONECLICK-UPDATE-1`：在已安装 Mod 的更新提示旁增加管理员“一键更新”，与“一键安装”共用扩展批次、进度、失败跳转和 session 恢复；停止态、扩展连接、直接 Nexus ID 和单成员包是显式门禁，不能自动替换时保留“查看更新页”。
- [x] `NEXUS-MOD-ONECLICK-UPDATE-1`：扩展升为 0.1.8 并贯穿 `operation/replaceUniqueId`；后端复用现有远程安装接口和 Job，先下载及校验 UniqueID/目标版本/单成员，再备份替换，失败恢复旧目录，同时保留旧 `config.json` 和启用/禁用状态。普通安装批次的 Nexus 页面按提交成功顺序串行打开。
- [x] 后端成功替换、文件夹改名、错误 UID/版本零写入、配置与禁用状态回归，扩展幂等/请求体/串行回归、production build 和桌面/800px Browser QA 已通过；真实 Chrome CDN 捕获、安装/更新任务终态与落盘 manifest 也已通过。

# 2026-08-17 已完成：全前端刷新按钮数据流审计

- [x] `FE-REFRESH-ACTIONS-AUDIT-1`：核对桌面/移动所有可见刷新入口；诊断健康/Compose 独立结算，任务刷新清除已消失任务的旧详情，移动回档兼容旧版 null，dashboard 异步刷新函数统一 Promise 类型契约。
- [x] 刷新数据流静态回归、Mod 删除后对账回归、响应式回归和 production build 通过；不新增后端接口，不触发 tag、镜像或 Release。

# 2026-08-17 已完成：下载模组页刷新清除已删除状态

- [x] `FE-MODS-REFRESH-INSTALLED-1`：桌面/移动 Nexus 搜索结果与当前服务器 Mod 清单双向对账；删除后刷新会显式清除本体和前置的“已安装”、启用状态、版本及文件夹元数据，桌面刷新使用本次 `/mods` 响应立即更新卡片。
- [x] 共用 helper 的直接 Nexus ID、Nexus 包来源 ID、删除后空清单回归和前端 production build 已通过；不新增接口，不触发 tag、镜像或 Release。

# 2026-08-17 已完成：Nexus 扩展一键安装锁定最新版本

- [x] `NEXUS-EXT-LATEST-1`：扩展 0.1.5 从新面板接收本体及所有前置的 Nexus 当前版本，只在对应文件行中选择 `file_id`；兼容旧 Panel 缺少版本字段的 batch，由 Nexus 当前文件页补出目标后走相同严格匹配。版本随普通页面参数/session/capture 传递，不改 CDN 签名链接，页面也无法确认版本时 fail closed。
- [x] Nexus 搜索补齐 `requiredMods[].version`；远程安装接口接收 `expectedVersion/nexusFileId`，ZIP 下载后、Mods 落盘前读取 manifest 二次验真，旧包不再能以新元数据写入或覆盖。
- [x] 扩展幂等/版本候选回归、前端 production build、后端专项与任务专属 Linux Go 1.25 全量通过；已登录 Nexus 的右侧浏览器实际 DOM 选中 `2.9.1/file_id=160463`，并排除旧版 `2.9.0/file_id=153187`。普通 Chrome 真实 v0.5.2 Panel 首轮点击证明 0.1.4 会因旧 payload 缺版本而安全失败、未开页/建任务/写 Mods，随后以 0.1.5 页面推断兼容修正。本任务不创建 tag、不发布镜像或 Release。

# 2026-08-18 已完成、待发布：运行设置入口与保存重启体验修正

- [x] 修复服务器摘要“修改上限”按钮用负 margin 越出单元格的问题；按钮改为卡片内居中并在极窄摘要单列，总览在线玩家卡片增加同一管理员入口。
- [x] 最大人数输入用 44px 像素风 `− / +` 替代浏览器原生 spinner，保留直接输入、方向键和 `1~100` 校验。
- [x] 弹窗动作改为左侧“关闭 / 仅保存”、右侧“保存并重启”，运行态先确认在线玩家断线再复用既有生命周期，停止态禁用重启；保存失败零重启，重启失败明确配置已保存。
- [x] 安装进度及 Steam 认证占位图标去掉重复阴影并固定层级；seed/Steam/download 三枚带烤入黑色外扩像素的旧 PNG 按原图用 image2 再生为干净透明的 72×72 RGBA PNG，保留 Steam 圆标与下载底座。摘要存档与主机农民使用适合 22px 的专用图标/头像裁切，不再出现素材被遮挡或缩成碎片的观感。
- [x] 定向状态/响应式回归、production build，以及 1280×720、430×900、390×844 QA shell 布局、步进和零横向溢出验收；本任务仅本地提交，不推送 `main`、不触发候选或自动 Tag。

# 2026-08-17 已完成：建档后修改联机人数上限

- [x] `SERVER-RUNTIME-MAXPLAYERS-1`：复用现有 runtime settings GET/PUT，在 `server-settings.json` 增加 `MaxPlayers 1~100` 的默认读取、旧 PUT 保留、未知字段保留、安全原子写、owner/mutex 互斥和旧/新值审计；不新增专用 API、SQLite 表、存档级设置或 Mod/小屋门禁。
- [x] `FE-SERVER-RUNTIME-MAXPLAYERS-1`：桌面摘要管理员入口、桌面快捷操作与移动控制页共用同一 hook/弹窗/保存流；明确当前生效值与重启后配置，低于在线人数只警告，保存不静默重启，成功刷新 players 投影。
- [x] 后端专项与 Web 权限/审计、Linux Go 1.25 全量 test/vet/build、前端全部 19 项状态回归/production build、桌面与移动 QA shell 联调已通过；真实 Docker 克隆已有存档验证 live `11`、运行中 configured `12` 仍 live `11`、重启后 `/players=12`，且源夹具保持停止、任务资源清零。本任务不打 tag、不发布 Release 或镜像。

# 2026-08-17 已完成：无需 SSH 的增强诊断日志 ZIP 与页头入口

- [x] `SUPPORT-BUNDLE-LOG-CONTEXT-2`：在现有管理员支持包中加入 Panel、server、steam-auth 有界日志 tail，以及当前实例最近任务的进度日志；完整实例诊断状态替代基础状态摘要，邀请码、密码、Token、session、存档、恢复材料和任务 payload 继续排除或脱敏。
- [x] `FE-DIAGNOSTICS-EXPORT-ACTION-1`：将“导出诊断包”移到“服务器健康”页头，并固定在“重新检查”左侧；桌面并排、窄屏两列、极窄屏单列，无横向溢出。
- [x] Docker/Web 专项测试、前端响应式契约与 production build 通过；本地应用内 Browser 已在 1400×900 和 430×900 验证顺序、同级视觉、纵排和零横向溢出。当前任务只完成诊断材料导出，不加入自动分析、远程上传或修复脚本执行。

# 2026-08-16 已发布：v0.5.1 swap-host 主农舍床、睡眠与 VNC 手动状态闭环

- [x] `HOST-BED-INTEGRITY-1`：定位 `.125` swap/finalizer 把主 FarmHouse 全部内容迁入旧 owner cabin、同时因新主机已为 0 级而跳过默认家具重建的组合根因；Control 0.3.5 在游戏线程仅按实际主 FarmHouse map `DefaultBedPosition` 与游戏床常量幂等补床，不硬编码 XML/坐标，不触碰 Farm/cabin/其它家具，0..3 级与无法确定布局 fail-closed 均有契约测试。
- [x] `SAVE-IMPORT-HOST-BED-ROLLBACK-1`：swap 激活加入 `hostBed` 复合证据和 `host_bed_missing`；已确认 swap 后的激活或 durable-save 任一步失败都会停止服务器、验证 preimport 材料并恢复完整存档树、活动指针、Mod profile 和实例快照，证明不足保持 manual recovery。
- [x] `JUNIMO-SLEEP-BOUND-1`：缺床不再无限 `sleeping in place`，每个故障 episode 只报告一次结构化原因；合法床沿实际 BedFurniture/player bed spot 执行原生睡眠，单日最多四次有界动作。
- [x] `VNC-HOST-MANUAL-1`：F9 手动模式释放 automation 输入、覆盖 NoConnectedClients 暂停、强制完整人物可见并以 10 分钟无人租约恢复托管；F10 原子同步 sprite/displayFarmer/hidden/shadow，warp、读档、逐 tick、跨日复核。`hostBed/hostControl` 已进入现有结构化状态。
- [x] 真实 Docker `.125` E2E 完成问题副本 swap、SaveLoaded、地图床位、save-now/GameLoop.Saved、重启幂等、Unix-socket VNC F9/F10、无客户端移动、恢复自动暂停，以及官方测试客户端实际睡眠；春 1 日进入春 2 日，无黑屏、超时或强制结束。Control 真实游戏程序集标准构建 0 warning/0 error。`v0.5.1` 候选 `31942102917`、兼容 `31942102879`、Tag `31942624901`、正式提升 `31942631860` 全绿，同一 digest `sha256:70c1967eb36827dbbf78ec3c11683c994814961dcf6673ae365ec4f43c6c25a5` 已提升到三仓版本与 `latest`。

# 2026-08-16 已发布：v0.5.2 Mod 自动更新检查与配置页重构

- [x] `MOD-UPDATE-CHECK-1`：Stardew driver 通过 SMAPI update service 检查全部可识别的启用/禁用物理 Mod；请求携带实际或保守基线 SMAPI API version，6 小时缓存绑定本地清单与运行时版本指纹，支持管理员强制刷新、失败保留上次成功结果和安全 URL 过滤。
- [x] `FE-MOD-UPDATE-REMINDER-1`：提醒限定在「添加模组」页签徽标、页内状态条和可更新卡片；提供只看可更新与重新检查，不建立系统通知、已读状态或后台轮询。
- [x] `FE-MOD-CONFIG-CARDS-1`：移除「配置模组」常驻右侧说明占位，改成全宽双列小图片卡、动态上下文条、搜索排序、状态筛选与原有安全开关；窄屏收敛为单列；「添加模组」已安装卡片的删除操作使用红色像素填充并保留运行态灰化禁用。
- [x] 后端五条 driver 专项和 Web 权限/路由专项、前端 Mod 列表/响应式回归与 production build 全部通过；`v0.5.2@51fd82459e4ac8afbf362f7ad12c0651937879a1` 的 Compatibility `31945655121`、候选 `31945655119`、自动 Tag `31946063809`、正式提升 `31946073920` 全绿。v0.5.1 unhealthy 回滚、healthy Web 升级、升级后受控 SMAPI/API/production bundle、三仓 `0.5.2/latest` 六引用、正式镜像首次/重启和 GitHub Release 均通过，统一 digest=`sha256:42b5dae824f63d3b5ba44a1f33704a622a62c4d6170225d52a63ac39147aaaed`。

# 2026-08-16 已发布：v0.5.0 聚合可靠性与兼容更新

- [x] annotated `v0.5.0` 固定在 `9b18dd3fe5192692548bf11a85010dd35303da93`；Compatibility `31899107019`、候选 `31899107629`、自动 Tag `31899867310`、正式提升 `31899874927` 全部成功。
- [x] v0.4.19 的 none/global/role 加入保护、角色独立密码与旧全服密码兼容完整纳入用户可见汇总；v0.5.0 同时发布存档导入 strict/耐久/cleanup 恢复、真实最近活动语义和 Control 0.3.4 主机农舍等级保持。
- [x] `v0.4.19` Web unhealthy 回滚/healthy 升级、升级后受影响 E2E、最老受影响 `v0.4.11` 代表升级预演、三仓六引用、Release 资产和独立正式镜像首次/重启均通过；统一 digest=`sha256:92ea973d55c1f63b4eb356652d491f8d37ef5f69112df1f19c161e4b0e9b611a`，owner 测试资源清零。
- [x] 官网首页/changelog 的 v0.5.0 与补录 v0.4.19 内容已由 `242453ab631750689de467625346b6b0fb97c206` 发布；VitePress build 2.68 秒、Pages `31900873468`、Compatibility `31900873542` 和线上 1440×900/390×844 首页到日志真实点击、版本顺序、角色密码/legacy/存档恢复/真实最近活动/农舍正文、零横向溢出及零 console warn/error 全部通过。同一 SHA 没有候选 workflow，未移动 tag 或改写正式 digest。

# 2026-08-16 已发布：默认保留虚拟主机农舍等级（v0.5.0）

- [x] Control `0.3.4` 在 `SaveLoaded` 前按精确程序集/类型/签名 Harmony 跳过 JunimoServer `.125` 的 `ResetHostFarmhouseToLevelZero()`；默认启用且不提供开关，不携带或修改上游源码。
- [x] Control options/status 暴露补丁安装结果；Panel 要求 availability 明确为 true，目标漂移或安装失败时以 `control_runtime_host_farmhouse_patch_unavailable` 停服，避免静默回退到归零行为。
- [x] C# 契约、真实游戏程序集编译、Go runtime/lifecycle 回归通过；真实 `.125` Docker 读档与 `GameLoop.Saved` 已证明任务副本的主机房屋等级 2 保持为 2，测试资源归零。
- [x] 显式 v0.5.0 候选按 `docs/09-image-build.md` 聚合矩阵完成 Control-only Web 升级、升级后受影响真实链与 unhealthy 回滚，并从同一 digest 自动 tag/正式提升；导入存档仍没有为房屋等级增加特殊分支。

# 2026-08-15 已发布：存档导入 token/job/cleanup 崩溃恢复（v0.5.0）

- [x] primary job、operationId、journal 与 owned token 形成 `type + instance + save-import:<operationId>` 一对一身份；job runner 只在 journal attach、token attach、journal ready 全部成功后启动，attach 失败不再返回 202。
- [x] Panel 重启覆盖 job 创建前后、journal/token 两次 attach 与 runner release 前窗口；只按 exact idempotency identity 恢复，缺失、冲突、payload 不符或 exact job 不存在均 recovery required，不猜最近任务。
- [x] cleanup 先持久完整只读计划，再按 bootstrap/staged/source 的 removal-started/removed 子阶段执行；schema/stage/关键字段、maintenance/FIFO/upstream、pointer、source/bootstrap/staged 指纹任一不可证时零删除或停止在可恢复证据上，preimport 永久保留。
- [x] filesystem completed 后通过 0600 cleanup receipt 串联 journal finalize 与 token 删除；token 删除失败、journal 已删、重复/并发 cancel 可幂等收敛且危险删除最多一次。
- [x] succeeded token 过期后压缩为 exact-result tombstone，不删除 completed journal、preimport 或正式存档，也不破坏同 token 幂等查询。
- [x] jobs、pending upload、transaction、Web/API 故障注入专项与升级后真实 Docker E2E 已随 v0.5.0 候选完成；正式 digest 提升后另做首次/重启冒烟和资源归零。

# 2026-08-15 已发布：存档导入维护事务耐久闭环（v0.5.0）

- [x] maintenance phase、原始实例快照与 `MaintenanceStarted` 改为 write-ahead 门禁；任一数据库/journal 写失败均禁止 ComposeUp 或进入 recovery required，不再吞掉 LastError、清旗、快照恢复错误。
- [x] 精确恢复 state、driver phase、`state_message` NULL/空/普通三态及 payload 原始字节；空 phase/payload 不经过 storage 默认值归一化。
- [x] 失败恢复固定为 Down(0) → strict fresh stop → journal 清旗/pending → 快照恢复 → journal restored；缺任一步都保留 ownership/人工恢复证据，Reconcile 与 cleanup 不得降级为普通 stopped。
- [x] Phase A FIFO 调用前增加耐久 attempt 位；仅明确未尝试、未提交、未确认的 pre-submit 失败自动停机恢复，模糊 FIFO 结果只停机并保持 manual recovery/单次提交语义。
- [x] Panel 重启恢复覆盖 start-intent 前后、ComposeUp 返回、Down 后清旗前、清旗后快照恢复前四个崩溃窗口；可能部分启动先 Down+strict，可能 FIFO 提交不自动恢复/清理。
- [x] maintenance/Phase A/transaction/Web 专项、Linux 受影响包与全量 test、vet、build 通过；最终候选再次通过默认全量和真实 Web 升级并随 v0.5.0 发布。首次全量与同工作树 Control 制品更新交叠读到旧摘要，稳定后同命令重跑通过，不作为产品放宽理由。

# 2026-08-15 已发布：存档导入严格无缓存停机证明（v0.5.0）

- [x] `ComposePsStrict` 保持独立 `--all` fresh 调用，拒绝命令失败、输出截断、坏 JSON、`null`、缺字段与未知 Docker state；普通 `ComposePs` 的短缓存和 UI 语义不变。
- [x] server 停机分类收紧为仅“无 server”或“全部 `exited/dead`”通过；`running/Up/restarting/paused/created/removing/unknown/空状态` 和多个副本中的任一非稳定项全部拒绝。
- [x] ownership/journal/runtime asset 前、maintenance 初检、`ComposeUp` 前、失败 `ComposeDown` 后和 owned cleanup 前全部使用 strict；cache invalidation 仅维持普通查询一致性，不再被当作停机证明。
- [x] 数据库权威 `DataDir` 贯穿 maintenance 错误记录与后续事务；提交/cleanup 遇到调用方目录不一致时在文件或 token 变更前返回 recovery required。
- [x] 真实 Docker Client cache/parser 受控回归、driver 状态矩阵、`game_installed` fresh-running 零副作用和 Web reservation 释放回归通过；任务专属 Linux 最终默认全量 test、串行复核、vet、build 均通过。
- [x] v0.5.0 候选按 `docs/09-image-build.md` 聚合矩阵执行升级后真实 Docker E2E、unhealthy 回滚和正式提升；命令/JSON 不确定性仍保持 fail closed。

# 2026-08-15 已上线：官网更新日志同步 v0.4.18

- [x] 官网首页版本角标、入口摘要和 `CURRENT RELEASE` 切换到 v0.4.18；changelog 置顶新增停服空 Compose 导入、Control-only Junimo 恢复、共享确认框与控制命令分页，v0.4.17 保留为历史。
- [x] VitePress production build 2.96 秒通过；应用内 Browser 在本地 1440×900/390×844 完成首页到日志真实点击，版本顺序/正文正确，零横向溢出、零 overlay、零 console warn/error。
- [x] docs-only 提交 `09601de0d9b9064b88a56d091678194a65c333cd` 的 Pages `31886032569` 与 Compatibility `31886032526` 成功且没有触发候选重建；线上 1440×900/390×844 的首页到 changelog 真实点击、版本顺序、四类正文、横向溢出、overlay 和 console 全部通过，`v0.4.18` 与三仓 digest 未改变。
- [x] 最终证据回填 `93f6a8464962f597319e986ae3114bdcf7a64106` 的 Compatibility `31886298061` 成功；无 website 变化所以未重复 Pages，无候选 workflow。随后复核 annotated tag、GitHub latest Release、三仓六引用和不可变 digest 重启版版本接口全部保持，任务容器/volume 归零。

# 2026-08-15 已发布：v0.4.18 Control-only 升级与旧人工恢复补齐 JunimoServer

- [x] 支持包定位到 server/auth 未变化、仅 Control 升级时宿主 JunimoServer 缺失；SMAPI 跳过 Control，目标和旧版回滚均无法通过健康验收。确认离线校时/SteamAPI 不是人工干预根因。
- [x] 新 apply 独立验证并事务化补齐缺失/损坏的 JunimoServer；auth 容器、steam-session 和运行栈推荐清单不变。
- [x] 旧 `rollback_failed` 清单可从持久化原 server immutable image ID 补齐组件，先恢复原版本验收再继续新事务；失败保留材料并返回稳定错误码。
- [x] 新事务、旧 v0.4.17 风格恢复、既有回滚矩阵及真实 Docker image-ID 提取回归通过。
- [x] 首次候选 `31883713810` 暴露 Linux helper root-owned bind；提取树现归还当前 Panel 数值 UID/GID，非 root DinD 的真实写入/删除 integration 与默认 package test/vet/build 通过，失败候选没有构建或推送镜像。
- [x] 最终候选 `31884242692` 在 `v0.4.17` Web 升级后的 Panel 通过旧 `rollback_failed` 第三次 repair，从原 immutable image ID 补齐 Junimo、保留 auth 容器与 stopped 状态；自动 Tag `31884612425`、正式提升 `31884620508`、三仓六引用、版本接口、Release 与证据回填完成。

# 2026-08-15 已发布：v0.4.18 停服空 Compose 集合不再阻断存档导入

- [x] 生产只读取证确认 `v0.4.17` 的 Stop job、SQLite 和 Compose 配置正常，项目容器/活动 job/import journal 均为 0，上传 token 未被接管；问题是 strict probe 将 `compose down` 后成功的空 stdout 错当异常，并由 Web fallback 映射为 `save_in_progress`。
- [x] `ComposePsStrict` 仅把“命令成功 + 空 stdout”接受为 0 services；坏 JSON、缺 service/state、未知状态、非零退出和运行中的 server 继续 fail closed，四态、两次检查及事务 ownership 门禁不变。
- [x] Linux Docker/Junimo 定向全包、串行全量 test/vet/build 及真实 `ComposeUp → ComposeDown → 零容器 → strict 空集合` integration 通过，任务容器、网络与缓存卷归零；公开上传 DTO、错误码和前端不变。
- [x] 候选升级脚本已加入真实 `Panel Stop/compose down → 零容器/空 stdout → Web 上传 202/jobId/operationId → 受控 maintenance 失败 → stopped/容器/网络清理`，不使用 `compose create` 占位绕过；Bash 语法与 ShellCheck 已通过。
- [x] 最终候选在升级后的 Panel 实际跑通 `Stop → 空 Compose → Web 上传受理 → 受控失败清理`；候选/Compatibility/Tag/正式提升成功，三仓 `0.4.18/latest` 统一 digest，测试与发布后核验资源均清零。

# 2026-08-15 完成：官网更新日志同步 v0.4.17

- [x] 官网首页版本角标、入口摘要和 `CURRENT RELEASE` 切换到 v0.4.17，changelog 置顶新增三项正式变更并保留 v0.4.16 历史条目。
- [x] 内容与 v0.4.17 GitHub Release、候选/升级/回滚证据一致，不修改 tag、镜像、digest、latest 或 Release 资产。
- [x] VitePress production build 5.90 秒及本地/线上 1440×900、390×844 Browser 全部通过；发布提交 `94db6f6` 的 Pages `31871879333` 和 Compatibility `31871879299` 成功，未触发候选重建。

# 2026-08-15 已发布：v0.4.17 认证健康探针、首次上传状态机与文案修正

- [x] annotated `v0.4.17` 固定指向 `d63c93ffe7d65f8cdfcf2bedb9b336a6839be73f`；Compatibility `31823172972`、候选 `31823172958`、Tag `31823884131`、正式提升 `31823899038` 全部成功。
- [x] `v0.4.16` 真实 Web unhealthy 回滚、healthy 升级、SQLite/初始化/Panel 数据/非目标容器与 volume/重启保持通过；候选 auth 挂起式 Docker fixture 和首次上传全量状态机回归通过。
- [x] 三仓 `0.4.17/latest` 六引用统一 digest=`sha256:44c328cdf198ec888f3ec54bbe836ce114f5ac27c4ca5fb9cc63747a44083673`；独立正式镜像首次/重启 health/database/version、GitHub Release 四项资产和任务资源清理均通过。

# 2026-08-14 完成：官网与 GitHub Release 补齐 v0.4.15 / v0.4.16 说明

- [x] 官网首页切换到 v0.4.16，更新日志补齐遗漏的 v0.4.15，并保留 v0.4.14 及更早历史条目。
- [x] v0.4.15 与 v0.4.16 GitHub Release 同步用户可读变更、升级验证、精确版本身份和完整 compare 链接，不移动 tag、不改镜像或 Release 资产。
- [x] VitePress build、本地/线上桌面与移动 Browser 通过；发布提交 `2df79f9` 的 Pages `31802129359` 和 Compatibility `31802129284` 成功，线上首页与更新日志已复核。

# 2026-08-14 已发布：v0.4.16 历史运行栈状态收敛与备份控制体验

- [x] annotated `v0.4.16` 固定指向 `5fa04d137bf760d2124b75cc5e3e8e2b44ff4c7c`；候选 `31799350642`、Tag `31799876171`、正式提升 `31799891830` 全部成功。
- [x] `REQUIRED-RUNTIME-STALE-STATUS-1` 在当前 Panel/stack、apply succeeded、实时 up-to-date 三重证据下收敛旧失败；真实失败与 `manual_action` 保持。
- [x] 桌面/移动隐藏但兼容已有 `FarmhouseStack`，桌面游戏日回档与其它备份一致展示悬停详情；fresh 与升级后 production bundle 均完成专项验收。
- [x] `v0.4.15` Web unhealthy 回滚、healthy 升级、数据/非目标资源/重启通过；三仓 `0.4.16/latest` 六引用统一 digest=`sha256:5f07910869d6d895e40ecb3954f5905d0cb6abf830e7cf57062bbcf97ca37e0f`，版本接口与 Release 四项资产通过，任务资源清零。

# 2026-08-15 完成：安装完成后首次上传状态机修复（SAVE-IMPORT-FIRST-INSTALL-STATE-1，released in v0.4.17）

- [x] 统一 Web 与 driver 的导入维护离线状态为 `game_installed / save_required / ready_to_start / stopped`；`uninitialized`、安装/授权阶段、`starting/running` 与其它不安全状态在 ownership/journal 前拒绝。
- [x] driver 在接管上传前读取真实数据库状态并检查 Compose，maintenance 在 `ComposeUp` 前再次检查；数据库离线但 server 实际 running 仍 fail closed，`game_installed` 不再要求先经 Stop API 永久归一化。
- [x] maintenance 失败/取消/readiness 异常仅在 ComposeDown 与 ComposePs 双证据下恢复原始 state/phase/message/payload；`maintenanceStarted` 改为 ComposeUp 前预写，停机不可证时阻止 cleanup。
- [x] terminal failed/canceled owned token 增加受控 cancel：精确核对 job/instance/operation、运行时已停、未提交、bootstrap pointer 与 staged fingerprint 后清 journal/source/target/bootstrap/pointer/token，preimport 保留；活动、成功、submitted 或不确定证据保持 busy/recovery。
- [x] 发布前二次审计收紧为无缓存 `compose ps --all` 严格解析、多 server/未知状态 fail closed、权威 DataDir、状态与 journal 写错显式失败、`state_message` NULL 精确恢复，以及 Phase A FIFO 前失败的安全停机。
- [x] job 使用 operation 派生 idempotency key；jobId 绑定中断可恢复，无 durable job、cleanup 后 token 未删、token 已删 journal 未清三类窗口均有幂等契约。cleanup 所有只读证据先通过才开始删除，耐久 `canceled` marker 不再阻塞新上传；成功 token 到期回收。
- [x] 自动化覆盖真实 `game_installed` 空实例完整首次导入、四态矩阵、不安全状态零接管、Compose running 拒绝、精确恢复与 `backup_created` 历史失败解除 busy；正式候选、自动 tag、三仓提升和发布后证据均已回填到本文件顶部与 `docs/09-image-build.md`。

# 2026-08-14 完成：候选通过后全自动版本号、Tag 与正式发布

- [x] 产品镜像/部署资产路径推送 `main` 后自动运行候选；默认以最新 Release 为 Web 升级源，并在最高稳定 tag 上递增 patch，major/minor 仍可手动覆盖。
- [x] 新增成功候选的 `workflow_run` 收口器：校验证明 run/commit/version/digest，main 已前进则安全跳过，仍为最新则创建 annotated tag，并通过显式 `workflow_dispatch` 启动正式 digest 提升，规避 `GITHUB_TOKEN` 推 tag 不递归触发 workflow 的限制。
- [x] 文档、工作流和纯测试脚本提交不自动发布镜像；候选输入发生新产品提交时旧 run 自动取消或最终标记 superseded，不移动已有 tag。

# 2026-08-14 完成：升级历史失败收敛、FarmhouseStack 隐藏与回档详情补齐

- [x] `REQUIRED-RUNTIME-STALE-STATUS-1`：普通运行组件 apply 成功后立即重算 required 协调状态；启动/状态读取幂等兜底。只有当前 Panel/stack、成功 apply 与实时 up-to-date 三重证据才把旧 failed 改为 succeeded，真实失败和 manual_action 不清理。
- [x] `FE-CABIN-FARMHOUSESTACK-HIDE-1`：桌面与移动“小屋与联机高级设置”只向用户显示 `CabinStack`/`None`；`FarmhouseStack` 仅隐藏并保留已有配置与后端三值 API 兼容。
- [x] `FE-SAVE-GAMEDAY-HOVER-DETAILS-1`：桌面“游戏日回档”行复用“其他备份”的详情格式化逻辑，悬停展示备份类型、农民、游戏内日期和地图；新增专项回归脚本，接口和移动端常驻详情不变。

# 2026-08-14 完成：候选镜像一次构建与 digest 提升发布（RELEASE-CANDIDATE-PROMOTION-1）

- [x] 新增手动 `Validate release candidate` workflow：冻结同步 `main` 的版本、完整 commit 与 build date，自动执行代码回归、fresh/restart、上一正式版真实 Web 一键升级、unhealthy 自动回滚、SQLite/初始化/非目标资源保留，并只在全部成功后推送带 SHA 的 GHCR candidate。
- [x] 新增 `scripts/run-release-gates.sh`、Linux/CI `scripts/release-candidate.sh`、Windows Docker Desktop `scripts/release-candidate.ps1` 与受控 TLS DinD Web-upgrade E2E；基础回归始终执行，官网与 Junimo/SMAPI/远程制品长门禁按上一 tag 到候选 SHA 的受影响路径自动选择。
- [x] 正式 `release.yml` 改为查找同 SHA/版本的成功候选 artifact，验证 annotated tag 精确等于 `origin/main`，再把候选 digest 原样提升到三仓 exact/latest；一个正式镜像冒烟后创建 GitHub Release，不再 tag 后重建镜像或逐仓重复启动相同 digest。
- [x] 发布硬门禁默认只保留上一正式版升级；只有数据库迁移、部署格式、运行栈、长期数据或跨版本兼容实现变化时增加最老受影响版本。发布完成仍须回填两个 workflow ID、唯一 digest、选择/跳过矩阵、耗时与资源清理。
- [x] Windows Docker Desktop 合成 `0.4.16` 开发演练完整通过：fresh/restart、正式 `0.4.15` 真实 Web unhealthy 回滚、健康升级、SQLite/初始化/Panel 数据/非目标资源和重启耐久；最终轮 403.5 秒，全部任务资源归零且没有任何远端发布。该 `-AllowDirty` 结果只验证流程实现，正式候选仍须由同步干净 `main` 的 workflow 生成。

# 2026-08-14 已发布：v0.4.15 Nexus 幂等、自动解绑与无存档首次上传

- [x] annotated `v0.4.15` 固定指向 `d84157dc8a3abc83d13d29c276d6ed332e901ce7`；Compatibility `31725203858` 与 Release workflow `31725256195` 成功。
- [x] v0.4.14/v0.3.2 Web 一键升级后的 Nexus 重放、错误 runtime 同 token 恢复、空存档首次上传、2/1/0 自动解绑、Panel 重启耐久，以及 unhealthy 自动回滚全部通过。
- [x] Docker Hub、ACR、GHCR 的 `0.4.15/latest` 六引用统一 digest=`sha256:b91e3cfd8175305723e0b97feb7c4c202179f2e229aff4f6145fe60b354a5c33`；三个精确镜像 fresh/restart 与四项 Release 资产核验通过。
- [x] nanoid 最小升级到 3.3.18，production audit=0；全部任务临时文件、DinD、registry、容器/网络/卷按 owner 精确清理为零，历史只读夹具未改。

# 2026-08-14 已发布：发布门禁最小修复 nanoid high advisory（FE-DEPENDENCY-NANOID-SECURITY-1）

- [x] 确认依赖链 `vite@8.0.16 → postcss@8.5.25 → nanoid@3.3.17`，只把 lockfile 传递依赖升级到修复版 3.3.18；不新增直接依赖、不扩大其它包版本范围。
- [x] Node 24 洁净 `npm ci`、production audit 0 vulnerabilities、15 项状态测试与 production build 全部通过。
- [x] 从包含 lockfile 修复的最终 SHA `d84157d` 重建 0.4.15 候选，fresh、v0.4.14/v0.3.2 升级后功能、unhealthy 回滚及正式发布收口全部通过；旧 `df90240` 候选未用于 tag。

# 2026-08-13 完成：恢复部署页国内 HTTP 加速卡片（DOCS-INSTALL-HTTP-CARD-3，未发布）

- [x] README、新手指南、官网部署安装/一键脚本/Windows 页面及镜像部署文档，统一在官方 GitHub Release 命令正下方恢复“国内加速脚本（HTTP）”卡片，地址为 `http://anxinas.dpdns.org/run.sh`。
- [x] 六个活动入口各包含一个官方地址与一个国内地址，顺序全部为官方在上、国内卡片在下；未修改 `deploy/run.sh`、Panel、镜像候选或 Compose。
- [x] VitePress production build 通过；应用内 Browser 覆盖桌面部署页、390×844 一键脚本页与 Windows 页，卡片/命令/跳转可见，无横向溢出、framework overlay 或 console warn/error。本任务未创建 tag、未发布网站或镜像。

# 2026-08-13 完成：Windows 部署独立专页（DOCS-WINDOWS-STANDALONE-1，未发布）

- [x] 把系统要求页的 Windows + Docker Desktop 正文迁到 NAS 图形化部署之后的独立专页，补全 WSL2、Linux containers、WSL Integration、目录、部署、访问、端口、维护和排障。
- [x] 系统要求页只保留新页入口；quick-start、部署安装、首页、README、新手指南和文档门户架构同步链接，Panel API、脚本与 Compose 不变。
- [x] VitePress production build 通过；应用内 Browser 在 1440×900/390×844 验证新页、侧栏顺序、NAS→Windows 点击、系统要求正文移除、零页面横向溢出、零 overlay 与零 console warn/error。本任务未创建 tag 或发布网站。

# 2026-08-13 完成：NAS 默认推荐 SSH 一键部署（DOCS-NAS-SSH-DEFAULT-1，已发布）

- [x] NAS 图形化部署页改为“进阶”入口，首屏默认推荐开启 SSH 并运行 `run.sh`；图形化 Compose 只推荐给能自行处理宿主机路径、Docker Socket、端口、持久化挂载和环境变量的熟练用户。
- [x] 官网首页、系统要求、一键脚本、部署安装和侧栏，以及 README 与新手指南统一推荐顺序；Panel API、部署脚本和 Compose 内容不变。
- [x] VitePress production build 通过；应用内 Browser 在 1440×900/390×844 验证新文案、进阶标题、推荐链接跳转、零横向溢出、零 overlay 和零 console warn/error。提交 `5526ef214e1ff25b7e30b9861bf416302a39d08b` 的 Pages `31708671546` 与 compatibility `31708671729` 均成功，说明已上线；没有创建 tag 或发布镜像。

# 2026-08-13 完成：无存档实例可直接上传现有存档（SAVE-IMPORT-FIRST-UPLOAD-1，未发布）

- [x] 将“从未启动导致宿主 JunimoServer Mod 尚未物化”与真实版本不兼容分流：精确 `.125` image 在上传接管前复用 lifecycle 原子同步；新增 `save_import_runtime_prepare_failed`，不再误提示用户升级。
- [x] 修复标准 Compose 下 Panel `/data/...` 与宿主 `PANEL_HOST_DATA_DIR/...` 不同名导致二级容器挂错目录：Junimo/SMAPI helper bind 统一经过数据根内受限映射，实例 Compose 的受管 `.local-container` bind 改用自动写入的 `${INSTANCE_HOST_DATA_DIR}`，旧相对路径在 Prepare/runtime recovery 原子迁移；越界或配置不完整 fail closed，旧同路径部署保持兼容。
- [x] 空 saves/无 gameloader 时从只读目标创建 operation-owned bootstrap 维护世界，目标与 preimport 指纹保持；已有活动存档路径不变。
- [x] journal 固定 bootstrap 身份、指纹、发布 ownership 和清理状态；提交前取消、重复 staging、同名碰撞零删除、发布/ownership 崩溃窗、目标 pointer 门禁和完成清理均 fail closed，异常进入 recovery 而非误报 completed。
- [x] Web 稳定错误映射、前端通用/导入专用提示及 `test:save-import` 已更新；Go 专项、相关包全量测试和前端 production build 已通过。
- [x] `eaae88f` 同路径独立候选真实空实例 Web 上传已通过 runtime-prepare 故障同 token 重试、bootstrap 创建/清理、自动解绑、durable save、preimport、Panel 重启与资源归零；标准 Compose 的 v0.4.14→候选升级、数据/非目标资源保留和重启也已通过，并由升级后 409 定位上述 host bind 缺口。
- [x] 代码等价 `5fc7e4c` 候选已从 v0.4.14/v0.3.2 Web 一键升级后完成标准 Compose 空实例上传、同 token runtime 恢复、自动解绑、Panel 重启与数据/非目标资源保留；v0.4.14 unhealthy 自动回滚也通过。
- [ ] 官网 docs-only 提交并入后修复已 rebase 为 `967647d`/`fd04ff0`；按 `docs/09-image-build.md` 从最终 SHA 重建精确 `0.4.15` 候选，重跑 fresh、两条升级后功能、unhealthy 回滚和正式发布收口。完成前不创建或推送 tag。

# 2026-08-13 完成：Nexus 扩展重复提交持久幂等（NEXUS-EXT-IDEMPOTENCY-1，未发布）

- [x] 扩展 0.1.3 为每次 capture 持久化 requestId，自动/手动/下载事件竞争和 service worker 重启后复用；不同 fileId 与新安装动作轮换。
- [x] background/panel bridge 使用无 TTL 的 in-flight singleflight，失败立即释放并保留可重试 capture；两条 POST 路径统一发送 Idempotency-Key。
- [x] migration 013 与 jobs 创建层按 type/target/key 原子绑定原 job，重复 HTTP 返回 `202 {jobId,deduped:true}`，包括任务已经终态或首次响应丢失的情况。
- [x] Node VM 覆盖 20 路扩展并发、bridge、同 worker 失败重试/worker 重启和不同或未知文件身份；Go 覆盖 12 路存储并发、Manager 单 runner、HTTP 复用与非法 key。compatibility/release workflow 已纳入扩展专项。
- [x] `eaae88f` 候选已完成真实 Chromium unpacked 0.1.3 → Panel bridge → job 链：20 路同 capture 为 1 owner/19 shared、Panel 1 job，关闭/重开浏览器后 requestId/job 保持；独立候选在调用方丢弃首次响应和 Panel 活动重启后 20 路/终态重放仍只复用原 job、runner=1，受控失败无 Mod 残留且 key 不入 audit/log。
- [x] v0.4.14/v0.3.2 Web 升级得到的新 Panel 已复验同 key 重放、Panel 重启和单 runner；本次受控 Nexus-CDN 形态 URL只验证提交/失败清理，不宣称使用生产 Nexus 登录完成真实 ZIP 下载。
- [ ] 最终 rebase 后 SHA 的精确候选仍须重跑该复验并完成 tag 前/后发布收口。

# 2026-08-13 完成：上传存档切换玩家主机后自动解绑（SAVE-IMPORT-AUTO-UNCLAIM-1，未发布）

- [x] 保持当前前端和 hostHandling DTO 不变；swap_to_player 后台默认自动处理，virtual_host_takeover/as-is 不受影响。
- [x] Control 0.3.2 在精确目标、服务器、零在线 farmhand 门禁下清空全部 farmhandData.userID，并与原耐久 save-now 共用同一 commandId、pending journal 和 GameLoop.Saved。
- [x] Panel 同时验证 Control 动作结果与 Junimo diagnostics 的 total/customized/bound 计数；零绑定前不进入 completed，旧 Control/DLL、错档、玩家在线或证据缺失均 fail closed。
- [x] 候选真实 Web 首次上传捕获 command history 与 durable gate 竞争：未完成导入的精确 save-now 结果现在由 journal 所有权保护，后台不得提前脱敏归档/删除；事务完成后才恢复普通归档，专项测试覆盖保护与释放。
- [x] C# 契约、真实 game-data 编译、Linux Go 全量 test/vet/build通过；Docker Desktop 隔离真机把 2 个角色中的 1 个绑定降为 0，磁盘 hash 改变且重启后仍为 0，角色数和 customized 数保持。
- [x] 代码等价候选已完成 v0.4.14/v0.3.2 Web 升级后真实 UI→导入、2/1/0 自动解绑、Panel 重启与 unhealthy 回滚。
- [ ] 最终 rebase 后 SHA 仍需重建精确候选、重复关键链并完成 tag 后三仓收口；本任务尚未发布。

# 2026-08-13 已发布：v0.4.14 启动误判、安装错误映射与新建档耐久

- [x] Control 启动使用 pending/ready/mismatch/invalid 四态和完整预算，Start/Restart 共用 gate，Reconcile 不再只凭 server container 提升 running；宿主重启保持游戏关闭。
- [x] 后端 installationDiagnostic 与前端共享分类器修复 `error → 未安装/重装`，desktop/mobile 升级后 Browser 已用 files-ok error 实证。
- [x] 新建档强制持久幂等、单 writer、owner fencing、完整角色内存复核、同 ID durable save、双 XML 稳定门禁和可恢复回滚；真实 startup writer POST=0、HTTP writer POST=1 通过。
- [x] v0.4.11/v0.3.2 Web 升级、unhealthy 回滚、官方一次性 migrate-fnos 与 621 图形化 Web conversion 已通过隔离真机门禁。
- [x] Linux CI 12 路 owner 原子 claim 偶发竞态已修复：进程内发布串行、跨进程 no-replace 原子边界不变；Windows/Linux 各 100 次并发专项和全量门禁通过。
- [x] `v0.4.12` tag 的 Release workflow `31679615132` 在镜像构建/推送前被旧维修脚本 SC2317 阻断；无 Release、无三仓或 `latest` 变更。tag 保持不可移动，局部 ShellCheck 0.10.0/0.11.0 修复复验通过。
- [x] `v0.4.13` 全部 release gates 通过后，ACR 拒绝 Buildx 默认 attestation，造成 Docker Hub `0.4.13/latest` 已更新而 GHCR/ACR/Release 未更新的部分发布；workflow 已显式禁用 provenance/SBOM，tag 不移动。
- [x] `v0.4.14@a70efc98feec` 的精确候选、正式 Web unhealthy 回滚/健康升级、Compatibility `31682006066`、Release workflow `31682847388`、三仓统一 digest、逐仓首次启动/重启和四项资产核对全部通过。
- [x] post-release 提交 `c0cc94cb56ae` 的 Compatibility `31684078849` 与 Pages `31684078868` 成功；线上首页和更新日志 HTTP 正文精确命中 v0.4.14、swappiness 与宿主重启后手动启动边界。
- [ ] 生产真机同步等待 `114.55.142.107:22` 的正确 SSH 用户名；`cz/root` 均被拒绝，未执行生产变更。取得用户名后使用精确 `0.4.14`，游戏保持关闭并由用户手动启动。

# 2026-08-12 完成：图形化 Compose 一键升级自动标准化（PANEL-UPDATE-GRAPHICAL-COMPOSE-1，未发布）

- [x] Compose 身份反查之后新增持久 `PANEL_IMAGE` 契约探针；缺 `.env`、坏 env 或写死 image 不再被 dry-run 误判为可直接升级。
- [x] 满足 bind 数据目录、Docker Socket、端口、权限、secret 和挂载保真边界的完整-label NAS 图形化部署返回 `conversionRequired=true`，复用现有独立 helper、部署/数据库备份、旧容器保留、健康/版本/labels 验收与自动回滚，一键流程和前端 API 不变。
- [x] 自动标准化允许并保留可验证的额外 bind/volume，覆盖图形化部署因镜像声明产生的匿名 `/data` volume；tmpfs、设备、根目标、重复目标、privileged、自定义 user 和身份歧义继续 fail closed。
- [x] 专项 Go 契约覆盖标准/缺 env/写死 image、安全与不安全转换、service label 漂移、匿名 volume、helper conversion 参数和 `.env` 缺键追加；独立 DinD 已按反馈夹具从 Web 一次完成 `0.4.10 → 0.4.11` 自动标准化，非目标游戏容器 ID/匿名 volume 保持且旧 Panel 停止保留。受控目标失败与中断矩阵仍作为本功能下一正式版本发布硬门禁记录在 `docs/09-image-build.md`。

# 2026-08-11 已发布：v0.4.11 安装、认证清理与首次建档状态一致性（INSTALL-FIRST-RUN-CONSISTENCY-1 / FE-INSTALL-AUTHORITY-1 / AUTH-CANCEL-RESOURCE-CLEANUP-1）

- [x] 同一实例活动 `stardew_install` 具备数据库 partial unique index、原子 job 创建和 409 已有任务 ID 契约；历史任务失去 active owner 后不能覆盖实例阶段。
- [x] SMAPI 内置 Mods 在安装成功前和首次建档事务/指纹前由 Junimo driver 从实际 game-data volume 原子物化；SMAPI staging 升级及回滚同步同一生命周期。
- [x] SMAPI 发布中断恢复只处理 Panel-owned `.smapi-sync-*` / `.smapi-backup-*`：清理孤立 staging，destination 缺失时恢复最近有效 backup；单元中断夹具、真实 Docker helper、失败回滚和升级后首次建档矩阵均已通过。
- [x] 安装页按 job ID 单调合并 dashboard/detail，terminal 不被迟到 running 复活；只有当前 active job 的日志能影响当前阶段，409 自动接管已有任务。
- [x] Linux Steam auth one-off 使用随机精确容器名；真实 `.125/auth .2` 无账号流程进入 QR 后取消。首次精确删除仍暴露“先观察为空、随后 daemon 晚到 Created 容器”的竞态；生产清理与真实测试均改为连续 3 秒缺席窗口后，测试 9.78 秒通过，外层复核案例 container/volume 为 0。
- [x] 存储并发、迁移、owner CAS、Web 409、SMAPI staging/指纹/事务顺序和前端状态纯函数测试已补；后端全量 test/vet/build、前端 13 项状态测试/build 通过，精确 `.125` server 镜像的真实 Docker 首次 sync/幂等/清理专项通过。
- [x] 真实生命周期以只读克隆的完整 game-data、空 saves bind 和空 Steam 凭据两次完成第一次建档：分别 71.78/60.04 秒创建唯一可解析活动存档，job log 的 SMAPI 物化 sequence 9 早于事务快照 sequence 10，成功终态无 owned staging 残留，第二轮 Stop 后容器归零。
- [x] 最终候选 `ef2580d2e58b170b5e5aa0079496f969228dd3f6` 已完成 `v0.4.10` unhealthy 自动回滚后健康升级、`v0.3.2` 直升、迁移/数据/非目标资源/重启、升级后 1.96 GiB 真实首次建档及最终 QR cancel。main compatibility `31521174829`、Release workflow `31521478699` 成功；annotated tag `v0.4.11` 未移动。
- [x] Docker Hub、阿里云 ACR、GHCR 的 `0.4.11/latest` 六个引用统一为 OCI index `sha256:7c2fea3496ac1ec4afa2ae50f1087f469151e46b18a9c202bd7d4e70f16bb86e`、amd64 manifest `sha256:f916037c571eac6962a4f6448e08c425e8e0b8956679835808d4e2c10f78d02c`；三个精确引用均通过独立首次启动/重启 health、SQLite、版本和 fresh setup 冒烟，四项 Release 资产与 tag 源文件摘要一致。
- [x] 官网 v0.4.11 已由 post-release 提交 `e3d40b155dd29cefe1fc9410675bbc91eb91d455` 上线；Pages `31523817426`、deployment `5856456646` 与 compatibility `31523817397` 均成功。本地 Browser 的 1440×900/390×844 首页、实际点击更新日志、最新/历史、安装/存档内容、无横向溢出、overlay 与 console health 通过；线上四个公开 URL HTTP 200 且 SSR 正文精确命中 v0.4.11。线上 Browser 截图因会话过早 finalize 未伪报，执行问题已记入错题本。

# 2026-08-10 完成：官网首页 QQ 群沟通入口（DOCS-HOME-QQ-COMMUNITY-1）

- [x] 首页 Hero 两个主按钮下方新增整卡可点击的 QQ 交流群入口，直接打开用户提供的官方加群链接；不新增独立页面、顶栏项或功能入口卡。
- [x] 文档正文页统一帮助页尾从 GitHub Issues 改为同一“加群反馈”入口，群链接集中维护；联机邀请卡、六入口、导航和 Panel 功能不变。
- [x] VitePress production build 通过；提交 `63aff0380de337faf57a9a6bcac1323b6e3593f6` 已由 Pages workflow `31388822404` 成功发布。线上覆盖 1280×720、870×760、390×844，页面身份、加群卡、真实 QQ 新标签、顶栏上下留白、横向溢出、overlay 与 console warn/error 全部通过。
- [x] 平板断点跟进：修复 640–959px 纵向 Hero 的分裂中心线，main、品牌行、按钮、加群卡与邀请卡统一居中；640/768/870/959px 实测中心差为 0，390/1024/1440px 原布局无回归。
- [x] 宽屏首屏节奏跟进：桌面 Hero 整体上移 40px、平板上移 16px，导航到品牌行留白分别收敛为 80px/70px；手机保持 55px，390/870/1440/1700px 无溢出或交互回归。
- [x] 宽屏顶栏跟进：`>=960px` 无侧栏顶栏容器从 1376px 收到 1180px，与 Hero 左右边界一致；390/959/960/1024/1440/1700px 无搜索/菜单重叠或溢出，移动菜单开合正常。
- [x] 平板紧凑跟进：640–959px 顶栏限制为 640px，870px 下搜索到汉堡空档从 416px 收到 263px；Hero 与导航间距收至 50px。390/640/768/870/959/960/1700px 无重叠、溢出或菜单回归。
- [x] 顶栏垂直留白跟进：修复隐藏态 `.VPSkipLink` 仍占 16px 普通流高度、将 64px 导航整体下推的问题；跳转链接改为绝对定位且只在非聚焦态裁切。870px 下 Logo、搜索框和汉堡图标上下间距分别为 19.5/20.5px、12/12px、25/25px；键盘聚焦后链接可见且导航不移位，390–1700px 断点无重叠或横向溢出。
- [x] 桌面首屏二次收紧（已发布）：`>=960px` 的 Hero 顶/底 padding 收为 `nav + 10px / 50px`，功能卡区额外上提 12px；1700×1100 的“顶栏→品牌行”由约 72.36px 收到 48px，“加群卡→功能卡”由约 87.64px 收到 64px。提交 `0508dbef3bb21d751f6333948010dcf534252e85` 经 Pages `31390948240` 发布；线上 390/959/960/1700px 无横向溢出，959px 菜单交互正常。

# 2026-08-09 完成：安装脚本供应链收口（DOCS-INSTALL-HTTPS-2）

- [x] README、用户指南、官网部署页和镜像文档只推荐官方 GitHub Release HTTPS，不再从仅支持 HTTP 的镜像下载后直接执行脚本。
- [x] 明确 HTTP 200/长度不能代替完整性；国内网络不稳定时使用浏览器打开官方 Release 手工下载，未来镜像需可信 HTTPS 或独立签名/摘要才能恢复推荐。
- [x] 最终 main 活动用户入口中的 HTTP 可执行命令为 0；GitHub Release 与 `latest` 的 `run.sh` 均通过 HTTPS 下载为 30,437 B，SHA-256 同为 `8f0040c11661f2e3f4060c66bf8ba205a33aa46fc65e3dec7cbf15b864c7387a`。

# 2026-08-09 完成：弹窗高度约束隐患修复（FE-MODAL-HEIGHT-GUARD-1）

- [x] 通用危险操作确认框与 Steam 二维码弹窗把会相互覆盖的双 `max-height` 声明合并为 `min(90/92vh, 100%)`，并用 `border-box` 把 padding/border 纳入限宽限高，恢复视口比例上限并保留内部滚动。
- [x] 响应式测试同时固定通用确认框、存档弹窗和二维码弹窗的有效高度约束；全部 12 项前端状态测试与 production build 通过。
- [x] 1180×900、769×500 删除确认框/新建游戏验收通过；769×240、280×653 长 Joja 确认框也完全位于 overlay 内且无页面横溢出，低高度内部滚动 `scrollTop 0→93`。升级后的 v0.4.10 bundle 又通过两视口二维码弹窗实测：769×240 内部 `maxScrollTop=225` 并可滚到底，280×653 完整装入，四边、关闭交互和 console 均正常。

# 2026-08-09 完成：新建游戏弹窗错误拉伸修复（FE-NEW-GAME-MODAL-LAYOUT-1）

- [x] 把新建游戏 1100px/480px 容器查询从页面级 `sd-main-scroll` 隔离到宽版弹窗自身的 `ngc-modal`，桌面三栏不再被侧栏后的主内容宽度误降级为单列。
- [x] 弹窗补齐 border-box 和有效的 90vh 高度上限，避免 100% 宽度叠加边框/内边距造成横向撑宽；窄屏继续在弹窗内部滚动。
- [x] 全部 12 项前端状态测试、production build、1180×1063 桌面三栏交互和 769×500 窄屏单列 Browser QA 通过；两个视口 document overflow 为 0、console warn/error 为 0。

# 2026-08-09 历史记录（已撤销）：GitHub 安装命令协议修正（DOCS-INSTALL-HTTP-1）

- [x] 历史证据：当时曾统一到明文 HTTP，并验证端点/页面一致性；这不能证明脚本完整性。
- [x] 撤销结论：当前用户入口已恢复为官方 GitHub Release HTTPS，旧 HTTP 地址不得作为可执行脚本来源。

# 2026-08-09 已发布：v0.4.10

- [x] 弹窗高度、独立容器查询、Steam auth 离线验收和等待可见性代码/专项测试完成。
- [x] 洁净前端依赖审计发现并修复 `nanoid <3.3.17` high advisory；空 volume 复验 audit high/critical 必须为 0。
- [x] 官网 lockfile 同步修复 `nanoid` 与 `postcss` high advisories；VitePress 1.x 稳定链无修复的 1 high + 2 moderate dev-server 公告明确限定为不发布、不暴露的构建期工具，2.0 alpha 不进入正式依赖。
- [x] 精确候选的全量代码/脚本/兼容矩阵/文档门禁完成；候选提交为 `7d9d0e267d942952701bc14ac19d032951d2dfd7`，compatibility workflow `31321583191` 成功。
- [x] `v0.4.9` 与支持下限 `v0.3.2` 的真实 Web 一键升级、Panel 自更新 unhealthy 候选自动回滚、数据/非目标资源保护、重启和升级后认证等待/二维码 QA 完成；steam-auth unhealthy + 合法离线接口的成功链由独立 runtime Docker integration 覆盖。
- [x] 从同步且干净的 `main` 创建并推送 annotated tag `v0.4.10`；Release workflow `31325589153` 成功，GitHub Release 已发布并补正详细说明。
- [x] 三仓 `0.4.10/latest` 六个引用统一为 index `sha256:c37ad8e8d1498f377900b8a82e2ad1de761df23a06f1cb298ae349a362b111df`、amd64 manifest `sha256:7534a30c283e9497ee6533dae4dc82f443779700ee90eb72858dfc49d43d9070`；三个精确版本均通过回拉 health/version/restart 冒烟，四项 Release 资产摘要通过。
- [x] 官网 v0.4.10 已由 post-release 提交 `3457efea561f5fbb865eab440576e91cf2de6ec1` 上线；Pages `31326926817`、deployment `5821195957` 与 compatibility `31326926808` 均成功。线上 1440×900/390×844 首页、点击更新日志、v0.4.10 最新/v0.4.9 历史、无横向溢出及 console/page/request health 全部通过；最终截图等待 Hero 入场与平滑回顶稳定后视觉正常。极快历史切换可触发的 VitePress 1.6.4 outline 竞态属于既有官网框架项，另记 `docs/07-later-optimizations.md`，不冒险混入本版发布收口。

# 2026-08-09 已发布：官网展示 v0.4.9（DOCS-PORTAL-V049-1）

- [x] 首页 release、版本卡和摘要更新为 v0.4.9；保持既有 Hero、联机邀请卡、六入口、导航和视觉主题。
- [x] 更新日志置顶新增三类升级修复、最多三次、失败回滚、未知状态支持包、重启续跑和 auth 保留说明；历史版本条目完整保留。
- [x] 本机 Node 20 VitePress production build、桌面/390×844 Browser、首页到更新日志跳转、无横向溢出和 console health 已通过。
- [x] Pages workflow `31305028853`、compatibility workflow `31305028888` 成功；线上桌面/390×844 首页与更新日志、点击路径、无横向溢出和 console health 均通过。

# 2026-08-09 已发布：升级故障目录和具体修复按钮（RUNTIME-UPDATE-REPAIR-CATALOG-3，v0.4.9 released）

- [x] GET 检测与 POST 执行统一使用后端 `RuntimeUpdateRepairPlan`；错误码、证据、方法、步骤、按钮、动作和三次上限由一个闭集目录产生。
- [x] 自动闭环覆盖 rollback 恢复、可信旧候选规范化、`failed_rolled_back` 安全重试三类；所有路径都重新执行完整预检、备份、新事务和失败自动回滚。
- [x] 状态损坏、材料漂移、自定义镜像、配置歧义和次数耗尽明确切换到“保留现场并导出支持包”；任务运行中/矩阵不安全分别切换到等待自动恢复/等待安全版本。
- [x] 面板按钮直接写修复方法，脚本 `check` 同步输出相同方法；Go、前端、Bash/ShellCheck、Docker integration、Docker 候选镜像和浏览器点击验收通过。
- [x] 支持包新增脱敏 `junimo-update.json`，把 detector、repair plan 和公开 apply 终态随包带出，同时继续排除恢复清单/备份、存档、session 和凭据。
- [ ] 这仍是 Junimo runtime 的闭集目录；SMAPI、Panel updater 与未来 game/SDK 必须各自增加同等级 detector/repair plan，不能共享不匹配的事务材料。
- [x] `v0.4.9` 已从提交 `6f3e4a28f6c5f983f0f891079fb0b7478bd5c1a9` 发布；精确候选、`v0.4.8` Web 成功与 unhealthy 自动回滚、`v0.3.2` 直升、发布工作流、三仓 `0.4.9/latest` 回拉、Release 资产和发布后隔离冒烟全部通过。三仓统一 digest 为 `sha256:e8fa5386b17d778612365bfa419b5ad5e2f447bb557856580efe262fea6f505f`。

# 2026-08-09 完成：已知升级故障检测、修复与续跑闭环（RUNTIME-UPDATE-DIAGNOSE-REPAIR-2，未发布）

- [x] `rollback_failed` 与可信历史候选配置统一进入后端持久化 repair job；按钮不再只恢复旧版，也不依赖浏览器串联多个请求。
- [x] 修复前记录事务/清单/材料诊断，修复后重新执行普通完整 dry-run、运行时差异计划、存档保存/整档备份与全新 apply；仅目标验收通过记为成功。
- [x] `resuming_upgrade + resumeAfterRepair + repairSourceApplyId` 覆盖修复完成到新事务 mutation 前的 Panel 中断窗口；次数跨续跑事务保留且最多三次。
- [x] 未知/自定义状态继续 fail closed；自动修复规则必须是代码内闭集，并有备份、复检、失败回滚和故障注入测试后才能扩展。
- [ ] 该闭环仅覆盖 Junimo runtime。SMAPI staging、Panel helper 与未来 game/SDK apply 仍按 `UPGRADE-RECOVERY-UNIFICATION-1` 分别实现，不得复用不匹配的恢复材料。
- [ ] 尚未发布；候选 Docker、完整门禁与正式发布状态以 `docs/09-image-build.md` 为准。

# 2026-08-08 完成：跨版本运行组件恢复与一键修复（RUNTIME-UPDATE-WAL-REPAIR-1，未发布）

- [x] schema 3 为所有实例 mutation 增加 write-ahead intent；重启恢复按“操作可能已到达 Docker”保守、幂等回滚，消除完成标记落盘前的崩溃窗口。
- [x] 未修改实例的备份阶段重启自动收敛；成功/回滚终态先提交再清理；事务校验绑定持久化推荐版本，不因新 Panel 当前推荐版本变化而拒绝旧事务恢复。
- [x] schema 3 恢复文件 SHA-256、精确 apply ID/卷名、immutable helper image ID、幂等 Junimo restore 和三次有界 repair 已实现；篡改材料在任何 Docker mutation 前拒绝。
- [x] Panel 管理员“一键安全恢复”、严格 repair API 和 `repair-junimo-upgrade.sh` 已实现；脚本不直接操作 Docker，Release workflow 会执行功能测试/ShellCheck并附加该资产。
- [ ] 本项只覆盖旧 Panel 升级后触发的 required Junimo runtime 同步。SMAPI staging 与 Panel helper 自更新各自已有自动回滚，但各自的极端 `rollback_failed` 仍需统一成受校验的一键恢复入口；见 `docs/07-later-optimizations.md`，不能误记为已解决。
- [ ] 尚未打 tag 或发布镜像；正式发布前仍须完成 `docs/09-image-build.md` 中的旧正式版到候选版 Web 一键升级、目标失败回滚和发布后回拉门禁。

# 2026-08-08 完成：运行组件升级保留未变化 Auth（RUNTIME-UPDATE-PRESERVE-AUTH-1，未发布）

- [x] 修复 Control-only 升级仍停止、快照和重建 steam-auth 的缺陷；按 tag + immutable image ID 只操作真正变化的组件。
- [x] 未变化且运行中的 auth 容器 ID/会话保持，跳过 Steam readiness 网络链；CPU shares 通过原地 update 同步为 256，server 独立重启并加载新 Control。
- [x] 真正 auth 升级的等待预算从 90 秒增至 10 分钟；旧恢复 schema 按全量变更保守兼容，失败回滚不再触碰未变化 auth。
- [x] 单元/专项测试及 Docker Desktop stopped/running 真机 Control `0.2.0 → 0.3.0` 通过；运行态 auth ID 不变、Control 实载、资源权重和最终状态均验证。
- [ ] 尚未创建候选镜像、执行正式 Web Panel 一键升级矩阵或发布 tag；若进入发布流程，必须完成 `docs/09-image-build.md` 的全部门禁。

# 2026-08-07 已发布：玩家 Mod 查看；官网恢复正式布局（v0.4.8）

- [x] `v0.4.8` annotated tag 指向 `0c5e2c434a92e8c9a69f839b39f86508cccf9a77`；Release/compatibility workflow `31117969497/31117949897` 最终成功，GitHub Release 为正式版且三项脚本附件齐全。
- [x] Docker Hub、阿里云 ACR、GHCR 的 `0.4.8/latest` 六个 index digest 统一为 `sha256:5381009b807ad2c632075332e3538297b5069eff2f2b1b133ff7fffd2ac38f90`；三个精确镜像分别通过 isolated health/version smoke。
- [x] 玩家 Mod 第一至三阶段、列表 CJB 提示、内置项过滤与桌面/手机详情已发布；保持只读、自报、可绕过和不自动处罚边界。
- [x] 历史分支/worktree 已收敛并清理，但任务型门户草稿不属于玩家 Mod 发布授权范围；`6f34b8a` 的误发布已按用户要求撤回。官网恢复 `0c5e2c4` 的正式布局，只保留 v0.4.8 版本卡、更新日志与玩家 Mod 手册；本地 production build、Pages `31152244079` 与线上 1440×900/390×844 视觉和状态复核通过。
- [ ] PC 原版、官方两种 CJB、Android/iOS 官方客户端、Android 实验性 SMAPI及多个真实远端玩家并发仍未做实体联机；不把自动化/fixture 记成真机支持。

# 2026-08-06 完成：玩家 Mod 比较展示收敛（PLAYER-MOD-BUILTIN-FILTER-1 / FE-PLAYER-MOD-PRESENTATION-2，v0.4.8 released）

- [x] 比较与前端兼容层统一隐藏 `Pathoschild.SMAPI`、`JunimoHost.Server`、`AnXiYiZhi.StardewAnxiPanel.Control`；三类内置组件不再产生任何统计或分组提示。
- [x] 统计/分组调整为“玩家额外安装”第一、“玩家缺少 Mod”第二，随后“版本不同 / 匹配”；后端 items 同步按该结果顺序排序。
- [x] CJB 总横幅和条目移除两段解释，只保留明确 CJB 文字检测提示；没有改变玩家加入、认证、踢出、封禁或自动拦截。
- [x] Go 全量 test/vet/build、前端状态/响应式测试、TypeScript、production build 与桌面/390px Browser QA 通过。未发布、不打 Tag。

# 2026-08-06 完成：玩家列表与待认证卡 CJB 显式提示（PLAYER-MOD-CJB-LIST-1 / FE-PLAYER-MOD-CJB-LABEL-1，v0.4.8 released）

- [x] 高频 `GET /players` 只增加轻量 `modRiskFlags:["cjb"]`，不复制完整 Mod 清单；损坏/超限上下文不生成标记。
- [x] 桌面与手机玩家列表把命中项显示为“检测到 CJB 作弊”；桌面和手机待认证卡同步显示文字徽标；详情横幅改为“检测到该玩家使用了 CJB 作弊工具”。
- [x] 保留客户端自报与可绕过边界，未改变加入、认证批准、踢出、封禁或自动拦截。Go 定向测试、前端状态/响应式测试、TypeScript、production build 和桌面/390px QA 通过。
- [ ] 两种官方 CJB 的真实客户端联机仍未完成；本次 QA fixture 和契约测试不能替代实体客户端验证，不发布、不打 Tag。

# 2026-08-06 收尾：玩家 Mod 查看第三阶段（PLAYER-MOD-COMPAT-1，v0.4.8 released）

- [x] Control peer 事件共用可测试生命周期，补齐 pending 超时、context/connected 次序、断线、重连、服务端重启、多玩家隔离和旧上下文不串号回归。
- [x] 补齐异常/重复/超长/超量输入、两种 CJB、四种比较结果、面板内置组件过滤、server_only、只读无管理调用和真实 loopback HTTP/SQLite/文件系统联调。
- [x] 使用只读真实 game-data 编译 Control 并更新嵌入 DLL/运行栈 SHA；前端兼容 fixture 覆盖 unavailable 禁用统计、不同状态、完整分组、重复与长字段。
- [x] 明确记录清单是客户端自报；修改 CJB manifest UniqueID 可绕过提示，不能把该功能当自动处罚依据。本阶段不改加入流程、不实现 SDR 桥、不发布、不打 Tag。
- [x] 本机 PC + Stardew 1.6.15 + SMAPI 4.5.2 已通过标准 LAN/IP 加入隔离服：reported 三个真实 Mod，比较分组正确，主动断线 stale、同 ID 重连新 reportedAt、server 重启后旧 context stale 均通过；无踢出/封禁/拦截。
- [ ] 实体 PC 原版、两种官方 CJB、Android/iOS 官方客户端、Android 实验性 SMAPI和多个远端玩家同时在线仍未验证。单一 PC+SMAPI 结果不能勾掉这些真机门禁；真实登录后的详情页面视觉也仍待补。

# 2026-08-06 完成：玩家 Mod 查看第二阶段（FE-PLAYER-MOD-VIEW-1，v0.4.8 released）

- [x] 桌面玩家表为有 uniqueMultiplayerId 的玩家增加“查看上报 Mod”，新增 `/instances/stardew/player-mods?playerId=...` 路由、页面 switch 和后端 SPA 精确白名单。
- [x] 桌面/移动共用详情主体；移动玩家页完成列表/详情子视图与返回。姓名/状态/上报时间/游戏和 SMAPI 版本、CJB 警示、四项统计和分组顺序已固定。
- [x] unavailable 固定解释并隐藏统计；pending、stale、比较基准不可用、HTTP error 各自独立。CJB 横幅与条目均有红色文字标签；后续已按反馈移除附加解释段落。
- [x] 大小写不敏感去重、长名称/版本折行、每组分批展开、空/加载/重试及 280px 窄屏完成；server_only 只作服务器专用展示且有前端缺失防线。
- [x] 12 项前端状态测试、TypeScript、production build、Go driver/Web 测试和 1365×900/280×740 Browser 验收通过；未增加踢出、封禁、拦截，未开展发布或 Tag。
- [ ] 标准 peer 上下文以外的 Steam SDR、原版 PC、手机/平板与不兼容 SMAPI 仍可能没有 ModContext；后续若做兼容采集需单独设计，不能由页面猜测。

# 2026-08-06 完成：玩家 Mod 查看第一阶段（PLAYER-MOD-CONTEXT-1，v0.4.8 released）

- [x] Control `0.3.0` 监听标准 IP SMAPI peer context/连接/断开事件，以 uniqueMultiplayerId 原子写独立 `player-mod-contexts.json`；reported/pending/unavailable/stale 与 `mods:null`/真实空数组语义已固定。
- [x] 客户端字符串、数量与重复 UniqueID 完成边界、规范化和大小写不敏感去重；超量上下文整体 unavailable，不用部分清单比较。
- [x] 新增 `GET /api/instances/:id/players/:uniqueMultiplayerId/mods`，以实际 `options.json.loadedMods` 为服务器基线并用 ModInfo/syncKind 补元数据；match、missing_on_client、client_only、version_mismatch、unavailable 已覆盖。
- [x] server_only 不误报缺少；后续已从玩家比较中完全移除 SMAPI、Junimo 与面板 Control 三类内置运行组件；两条官方 CJB UniqueID 只返回 `riskFlags:["cjb"]`，不踢出、不封禁、不拦截。
- [x] C# 契约、真实引用编译、嵌入 DLL、Go 比较器单元测试与 HTTP 接口测试完成；未修改 Junimo/SMAPI 上游，未实现 Steam SDR 桥。
- [x] 第二阶段桌面/移动详情已完成；`mods:null` 解释为“未取得清单”，pending/unavailable/stale 不会显示成“玩家没有 Mod”。

# 2026-08-06 已完成：真实 Panel 手机侧栏预览

- [x] 正式前端路由可通过 `shell=mobile` 强制复用现有移动 Shell，方便在桌面侧栏并排检查真实玩家与 Mod 数据；不引入 mock、后端接口或管理动作。
- [x] 保持默认媒体查询适配与“完整版”手动切换，增加查询解析状态测试和 production build 门禁。

# 2026-08-01 已发布：官网首页联机邀请 Hero

- [x] Hero 改为开放式双栏，保留“一键部署你和朋友的专属联机服务器”和原 CTA，右侧使用联机邀请票表达“你、服务器、朋友”和自托管能力。
- [x] 使用 VitePress 官方 slot 与首页 frontmatter 门禁接入可访问邀请卡，不 fork Layout、不注入 DOM、不增加交互 tab stop。
- [x] 移除大面积实色底与高饱和绿色，完成浅/深主题的森林炭黑、暖石墨与灰鼠尾草低饱和配色。
- [x] 修复首行入口卡 hover 被 Features paint containment 裁切，保留卡片位移、阴影、焦点轮廓和正文区 paint containment。
- [x] Docker Desktop Node 20 production build、浅/深桌面、390/320/768px、主题切换、真实 hover、reduced-motion、键盘与 console 验收通过。
- [x] Pages workflow `30659672364` 与 deployment `5697130212` 成功；线上桌面浅/深、真实 hover、390px 首页、768px 正文菜单、导航与 console 复核全部通过。
# 2026-08-01 已发布：全窗口矩阵与平板/电脑浏览器响应式修复（FE-RESPONSIVE-VIEWPORT-1，v0.4.7）

- [x] 修复 `768px` 单一分流遗漏触控平板：手机与 1366px 内粗指针/无 hover 平板进入紧凑壳，普通窄电脑保留完整桌面功能。
- [x] 根 Shell 缩放改为 TypeScript 数值计算；wrapper 按真实内容盒在 resize/fullscreen 时重算。外层改为不可滚动裁剪并兼容归零，消除浏览器程序化滚动造成整屏上移/只显示一部分。
- [x] 登录页危险比例使用可滚动文档流；紧凑壳修复 Safari flex 滚动、四边 safe area、16px 输入和 44px 触控区，五个移动页面上限从 480px 扩到 1120px，并支持完整/适配版双向切换。
- [x] 弹窗、OpsRail、新建游戏窄容器、280px 操作区和 ResizeObserver 缺失降级完成；桌面/紧凑路由切换只重置各自主滚动区。
- [x] 专项测试逐像素扫描 280..3840 × 16 高度并覆盖 7680×4320；11 项前端测试、production build、移动六页/桌面九页/认证与弹窗 Browser 矩阵通过，console error/warn 为 0。
- [x] 2026-08-01 用户确认实体平板的横竖屏滑动、浏览器全屏、底栏切页和输入法冒烟通过。用户明确不要求曾复现问题的朋友电脑另行验收；该设备未复验如实保留为剩余风险，由本机桌面 Browser 矩阵和最终 Docker 候选真实页面验收补充覆盖。
- [x] 官网首页版本字段、CURRENT RELEASE 卡和 changelog 已上线 `v0.4.7` 响应式修复说明。
- [x] 正式 `0.4.6 → 0.4.7` Web 更新完成 unhealthy 自动回滚、重新 dry-run 后成功升级、Panel 重启恢复、数据/非目标游戏容器保护及升级后 390×844/1920×1080 响应式复验。
- [x] annotated tag 精确指向 `619d18dafa76`；Release/compatibility/Pages workflow `30662967983/30662818759/30662818712` 成功，三仓 `0.4.7/latest` digest 统一为 `sha256:3f336863ae5ec45a1997edcfc0922269250d5763e8ada49a7ba43f81d59edd7f`，正式 Release、三仓独立 health/version smoke 与官网 HTTP 200 均已核验。
# 2026-08-01 已发布：官网首页入口卡去序号与视觉优化

- [x] 删除六张入口卡的全部序号/`NEW` icon，确认 DOM 中 icon 数量为 0 且没有遗留空白占位。
- [x] 沿用六个栏目语义色，增加顶部短强调线、轻量色洗和底部对齐 CTA，收紧卡片高度与间距；保留“推荐”和动态 `v0.4.6` 角标。
- [x] hover 只作用于精细指针，键盘焦点可见，深色主题和 reduced-motion 行为完整。
- [x] Docker Desktop Node 20 production build、1280px 浅/深色、390×844 手机、导航与 console 验收通过；无横向溢出或 framework overlay。
- [x] Pages workflow `30655296293` 与 deployment `5696310887` 成功；线上六卡、零序号、动态版本角标、更新日志导航、390px 单列与深色主题均已复核。

# 2026-08-01 完成：移除官网首页四步流程区

- [x] 删除“START HERE / 从一台服务器，到朋友加入农场”及四张流程卡，不保留空白占位。
- [x] 清理所有专属 CSS 和响应式规则，保留入口卡、`CURRENT RELEASE` 版本卡与更新日志入口。
- [x] VitePress production build、1280px 桌面和 390×844 手机实页通过；入口卡到版本卡间距 22px，无横向溢出、overlay 或 console error/warn，更新日志导航正常。

# 2026-08-01 完成：官网首页 Hero 部署文案

- [x] 首页主文案调整为“一键部署你和朋友的专属联机服务器”，直接表达一键部署、朋友联机和专属服务器。
- [x] VitePress production build 通过；1280px 桌面与 390×844 手机实页无横向溢出、overlay 或 console error/warn，“浏览完整手册”导航正常。

# 2026-08-01 已发布：Mod 安装时间、搜索与排序（v0.4.6）

- [x] 后端按实例原子持久化每个 Mod 的 UTC 安装时间；同 ZIP 同时间，历史目录不伪造，远程幂等不刷新。
- [x] 覆盖安装时间写失败整批回滚、删除/同包删除/多 ZIP 回滚清理和损坏 sidecar 的可用性边界。
- [x] 桌面添加/配置页与移动服务器模组页支持名称、ID、UniqueID、文件夹、作者、包名和 Nexus 数字 ID 模糊搜索。
- [x] 默认最近安装优先，并提供名称 A–Z / Z–A；搜索不改变批量启停和统计的完整作用域。
- [x] Docker Desktop Linux 全量 test/vet/build、十项前端测试、脚本/兼容矩阵/ShellCheck、Docker integration、文档构建、本地候选镜像 smoke 与双视口 Browser QA 通过。
- [x] Docker Desktop 真实候选 Panel E2E 覆盖 setup/session、深层 ZIP、本地多次安装、浏览器扩展一键 HTTPS 下载、重启持久化、删除清理、多 ZIP 部分失败回滚和 sidecar 写失败回滚；实际右侧栏桌面/390×844 搜索排序验收通过。
- [x] 官网首页、更新日志、维护页与深度手册同步 `v0.4.6` 的上传边界、持久安装时间、搜索与排序说明。
- [x] 修复未安装游戏实例在 Panel 升级成功后仍停留 `fullStack=checking_runtime/42`；实例与聚合均返回 `not_needed/100`，专项回归通过。
- [x] 加固远程制品门禁的镜像/Git 有界重试与 SMAPI 跨受审源分块续传；真实公网在 TLS timeout、SSL EOF 与 429 故障后恢复并通过，安全校验未放宽。
- [x] 精确候选全量门禁、`v0.4.5 → v0.4.6` 真实 Web 一键升级/失败回滚、升级后新功能验收与 `v0.3.13` 代表性旧版本矩阵均已完成。
- [x] `v0.4.6` tag、镜像发布、兼容矩阵与官网工作流均成功；三仓 `0.4.6/latest` digest 统一为 `sha256:6fd03bb202e8083b3453e2351bd70251c1bc2fea0e5c0f779fc62d99af39e07f`，正式 Release、三仓独立 health/version smoke 与官网 HTTP 200 均已核验。

# 2026-07-29 完成：展示文档长目录自动跟随

- [x] 定位版本日志 active marker 已更新但独立目录容器 `scrollTop` 不变的根因。
- [x] 仅在 active 项离开目录中部舒适区时自动调整目录，支持向下/向上跟随、路由切换恢复和 reduced-motion，不持续抢夺手动目录滚动。
- [x] VitePress production build、1440×900 长目录真实滚轮/点击/跨页回归及 390×844 移动端验收通过。
- [x] Pages 工作流 `30423428794` 成功，线上 v0.1.x 区域向下/向上目录跟随和 console 复核通过。

# 2026-07-28 完成：GitHub Pages v0.4.5 展示同步

- [x] 首页、CURRENT RELEASE 和 changelog 更新为 `v0.4.5`，公开说明 SMAPI 受审国内加速、分块续传、安全回退与 GitHub 官方兜底。
- [x] 安装手册和 FAQ 增加旧版超时症状、升级后“重新安装 / 修复”恢复路线、最终安装包完整性校验与禁止任意代理的安全边界。
- [x] 更新页同步代表性 `v0.4.4/v0.4.3/v0.3.13 → v0.4.5` 生产升级证据，并提示在升级得到的新 Panel 上重试 SMAPI 功能。
- [x] VitePress production build、桌面两条真实导航和首页/四个正文页 390×844 响应式验收通过；无横向溢出、overlay 或 console error/warn。
- [x] Pages 工作流 `30372623636` 成功，线上五页 HTTP 200 与目标文案复核通过。

# 2026-07-28 实现：SMAPI 受审加速源分块续传（v0.4.5 released）

- [x] 在真实国内主机确认游戏和 SDK 正常、SMAPI 失败由 41.9 MiB 官方包在约 `40 KiB/s` 下超过整包 2 分钟客户端超时导致。
- [x] 改为 2 MiB HTTPS Range；单段超时/断流后按实际收到字节继续，连续 4 次无进展才切换候选，保留安装 job 2 小时总门限。
- [x] 恢复代码审核固定的 `gh.llkk.cc → github.dpik.top → ghfast.top → GitHub` 顺序；每项仍受精确 URL/host、固定大小、SHA-256 与 ZIP 安全结构硬门槛约束，`.env` 任意下载源不生效。
- [x] Docker Desktop Linux 冷缓存完整下载 `41,889,142` 字节并验证摘要、ZIP、`0600` 与临时文件清理，耗时 `2m26s`；本地 `0.4.5` Panel 镜像冒烟通过。
- [x] `v0.4.5` 发布工作流 `30369196944` 全绿；三仓 `0.4.5/latest` digest 均为 `sha256:a8155defc50690b8b1e90c95f5b107e818b5438c68c341f90f9ebf8b7be428ad`，ACR 正式镜像回拉 health/version 冒烟通过。
- [x] 发布后补验 ACR 正式 `0.4.4/0.4.3/0.3.13 → 0.4.5` 三条 `RunApply` 路径，SQLite/setup/404 与旁路 game 容器 ID 均保持；升级后发布代码真实空缓存下载 40 MiB 和全部续传/回退专项通过。

# SAVE-BACKUP-EAGER-MAINTENANCE-1：游戏日回档连续性修复（2026-07-28，completed）

- [x] 查明“五个回档点相隔很多游戏日”不是五档清理排序错误，而是 `save-events` 只在打开备份列表时才消费，积压事件全部读取最新存档日期并覆盖同一目标。
- [x] 增加 Panel 启动补扫 + 每 2 秒后台维护，继续以 `GameLoop.Saved` 文件事件作为落盘权威，不在 API 层重新实现 Stardew 保存逻辑。
- [x] 串行化后台与列表接口的维护调用；用连续 7 游戏日无页面访问回归和 8 路并发消费回归固定行为。
- [x] 同步后端、联调、接手、镜像发布与公开展示文档；纳入 v0.4.4 Docker Desktop 和正式发布门禁。
- [x] 发布 `v0.4.4` GitHub Release 与三仓同 digest 镜像，正式镜像 smoke 和 Pages 线上复核通过；生产主机当前在 SSH banner 前拒绝本次来源连接，待网络入口恢复后执行实例升级与下一游戏日验收。

# 2026-07-26 完成：Panel 一分钟健康监控与自恢复边界

- [x] Panel 启动一分钟后开始每分钟复用 `/health` 的 SQLite `Ping`，单次超时 5 秒；只累计原生 `SQLITE_INTERRUPT` code 9，成功、超时与其它错误清零。
- [x] 连续第三次 code 9 时只退出 Panel 主进程，由 `restart: unless-stopped` 拉起 Panel；不调用 Docker API 或操作游戏容器。镜像 HEALTHCHECK 同步改为一分钟，并记录 unhealthy 不会自动重启容器的边界。
- [x] 后端全量 test/vet/build、Docker integration 与 Docker Desktop 29.5.3 正式 `0.4.3` 镜像通过：60 秒/5 秒/3 retries 配置正确，测试 Panel 退出后 RestartCount `0 → 1`、PID 改变且 `/health` 恢复；Linux 回归连续 10 轮、公开镜像真实 `0.4.2 → 0.4.3` 一键升级及游戏容器隔离均通过。`v0.4.3` tag、三仓镜像、GitHub Release 与 Pages 展示已发布。

# 2026-07-24 完成：Panel 扫描路径与 SQLite 取消恢复

- [x] 初始化状态改为启动时读取一次并原子缓存，首个管理员事务成功后同步更新；请求路径不再逐次执行 `AdminExists`。
- [x] 未注册页面/API 直接返回 404，SPA fallback 收窄到 Stardew 的明确前端路由。
- [x] `modernc.org/sqlite` 升级到 `v1.54.0`，取消中的连接可由 `database/sql` 丢弃并在下一查询重建。
- [x] 连续三次原生 `SQLITE_INTERRUPT` 主动退出进程，交给 Docker restart policy 恢复；真实取消后下一查询回归、全量 test/vet/build 已通过。

# 2026-07-22 完成：全站文档设计系统与首页性能修复

- [x] 27 个公开页面统一栏目色、阅读进度、面包屑、知识库侧栏、正文组件、右侧目录、帮助页尾、深色主题和移动端文档菜单。
- [x] FAQ 问题卡、版本日志时间线、部署终端代码块、手册步骤层级与图片展示建立独立但一致的视觉语义，搜索及默认界面文案中文化。
- [x] 首页移除持续模糊动画和多层大面积毛玻璃，滚动合成从 1 个动画与 8 个首页额外模糊/滤镜层降为零，仅保留全站共用导航模糊。
- [x] production build、全部非首页路由桌面扫描、六类手机代表页和浅/深主题 console 验收通过。

# 2026-07-22 完成：展示文档现代化重构

- [x] 首页首屏、品牌色、入口卡与导航从 VitePress 默认观感重构为统一的现代设计系统，并保留现有内容路由与 GitHub Pages 部署结构。
- [x] 增加四步开服路径与当前版本摘要，正文侧栏、目录、代码块、表格及图片样式同步统一。
- [x] VitePress production build、1440px 桌面、1280px 深色正文和 390×844 手机视觉验收通过，无横向溢出或 console error/warn。

# 2026-07-20 发布：v0.4.1 飞牛 OS 迁移脚本更新

- [x] 迁移脚本修订 3 保留安全可验证的额外挂载，以唯一 Compose project 隔离飞牛残留 labels，并在完整升级能力终检通过后输出固定成功识别码。
- [x] 展示文档更新 v0.4.1 日志，收紧顶部快速上手导航样式，突出首页版本更新卡片。

# 2026-07-20 发布：v0.4.0 一键全栈安全升级

- [x] Compose 服务名不再硬编码为 `panel`；只有容器、Compose 配置、镜像身份与数据挂载反查完全一致时，才允许标准部署直接升级。
- [x] 飞牛旧容器可由独立 helper 转换为标准 Compose；转换前保护数据库、Compose、环境变量、容器 inspect 与旧镜像 digest，新 Panel 健康失败自动恢复旧容器和数据库。
- [x] 飞牛迁移脚本修订 2 会分类额外挂载：可验证的 bind mount 与 Docker volume 自动按原目标、读写属性和 propagation/external 语义迁移；tmpfs、宿主设备、不可写数据目录及无法无损表达的挂载在修改前拒绝。
- [x] 飞牛迁移成功回执与 Web 升级能力终检：标准 Compose 二次解析、四项 Compose labels、服务对应容器 ID、镜像引用/digest 和可写数据挂载全部一致后，才持久化 `upgrade_environment=supported` 并输出固定识别码 `ANXI_PANEL_WEB_UPDATE_READY`；任一失败自动恢复旧容器。
- [x] 飞牛残留 Compose labels 隔离：旧 project label 或全局同名项目与拟生成项目冲突时，迁移脚本改用旧容器 ID 派生的唯一 project，避免改名保留的旧容器被 Compose 误识别并重新启动。
- [x] Panel 更新后逐实例校验 Control 版本和 DLL hash；不匹配时执行游戏内通告、保存、整档备份、停止、更新、启动和 SMAPI 实载验证，失败实例保持停止。
- [x] 全栈升级状态持久化并支持 Panel 重启续跑；在线确认明确展示真人数量、保存、备份、重启和客户端断线影响。
- [x] Docker Desktop 已覆盖任意 Compose 服务名、连续多旧容器转换、成功与健康失败回滚、数据库 hash 恢复、`.125` stopped/running Control 更新及候选镜像 smoke。

# 2026-07-20 发布：v0.3.13 存档上传编码与删除一致性修复

- [x] GBK/GB18030 ZIP 名称在预览前规范成 UTF-8，并固定完整 Stardew 存档布局、重复路径、控制字符、长度与 Junimo 命令边界。
- [x] 历史非 UTF-8 目录以稳定公开身份列出；正常中文可读，无法识别或同名冲突使用哈希别名；允许保护性备份/导出/删除并禁止激活。
- [x] 删除事务消除“目录已删但指针清理报失败”的假失败，重复删除返回 404；前端无论结果都刷新权威列表且不会因刷新失败卡 busy。
- [x] Windows/Linux 单测、全量发布门禁和 Docker Desktop 隔离 HTTP E2E 覆盖上传、列表、激活拦截、删除、备份、二次删除及重启持久性。

# 2026-07-18 发布：v0.3.8 删除离线存档人物

- [x] 只在服务器运行时开放删除；只允许当前活动存档中离线、非主机、已认领人物，被删人物重新上线会中止。
- [x] 其他真人玩家在线不阻断；桌面/手机确认框显示小屋同步与重连警告，删除前后发送游戏内通告并确认 command result。
- [x] 删除前强制保存并创建整档保护备份，使用 Junimo 官方接口删除人物与小屋，删除后再次保存并执行运行态/磁盘双重校验。
- [x] Docker Desktop 隔离真机完成停服/确认门禁、真实删除、小屋变化、重启、重复删除、广播和保护备份恢复；后端/前端发布门禁通过后发布 `v0.3.8`。

# 2026-07-17 发布：v0.3.7 升级旧镜像安全清理

- [x] Panel 成功升级清理本次旧镜像、可信历史稳定 tag/陈旧 `latest` 和项目 dangling；保护当前目标、所有容器引用、自定义仓库和未知 tag。
- [x] 新 Panel 可收尾旧版本 helper 写入的 succeeded 状态，因此 `0.3.6 → 0.3.7` 当次生效；`cleanupCompleted` 保证新旧 helper 路径幂等。
- [x] Junimo server/auth 成对成功后按旧精确 image ID 清理；容器引用、tag 漂移和失败只 warning，任意失败/回滚路径不清理。
- [x] Docker Desktop 真机覆盖 Panel 成功、unhealthy 回滚、历史/自定义/引用保护、跨版本生产镜像收尾、Docker driver、真实 `.121 → .125` stopped/running；完整后端、前端、兼容矩阵、远程制品、run.sh 和生产镜像 smoke 通过。
- [x] Tag workflow 继续在门禁通过后发布 Docker Hub、阿里云 ACR、GHCR 的 `0.3.7/latest` 并创建带详细正文的 GitHub Release。

# 2026-07-17 已完成：IMAGE-CLEANUP-1

- [x] Panel 成功升级后按原 tag + 原 image ID 双重核对删除本次旧镜像，并用 OCI title + 可信仓库 + 现存容器引用门禁清理历史稳定 tag、陈旧 `latest` 和 dangling 镜像；没有强制 rmi 或 `prune -a`。
- [x] Junimo server/auth 成对升级在完整验收和恢复原运行状态后删除事务记录的旧 pair 镜像；失败、回滚和 `rollback_failed` 保留旧镜像与恢复材料。
- [x] tag 漂移、共享容器引用和删除失败只形成 warning，不改变已验收升级终态；容器、volume、存档、Mod 与其它项目镜像不在清理范围。
- [x] updater、Docker 和 stardew_junimo 三个相关包全量测试通过；Docker integration 增加旧 Panel 镜像删除断言且保持宿主隔离。

# 2026-07-17 发布：v0.3.6 存档导入复合证据适配

- [x] 发布范围包含持久上传事务、no-replace 暂存、preimport、Junimo `.125` 维护运行时、Phase A、激活/finalizer、Control save-now 持久化，以及桌面/手机显式 hostHandling。
- [x] 隔离 Docker 技术链路已覆盖 takeover/as-is、swap、相同 commandId 的 `GameLoop.Saved`、dayTransitionComplete、稳定 XML 与真实第二次重启；发布门禁 test/vet/build、前端专项/typecheck/build 和兼容矩阵通过。
- [ ] 本 tag 不把 `SAVE-IMPORT-JUNIMO-1` 标记 completed：八类完整非唯一夹具、人工客户端角色/住宅/家庭语义、桌面和手机同一真实 job 贯穿及完整故障注入矩阵仍待补齐。
- [x] 保留最初 blocked 历史；上游仍没有 commandId，Panel 发布的是基于磁盘事务痕迹、pending、saveId、finalizeCount、GameLoop.Saved 和 dayTransitionComplete 的黑盒复合证据适配。

# 2026-07-16 未完成：SAVE-IMPORT-E2E-RELEASE-1

- [x] 只读盘点隔离 spike、Docker 状态及已有归档；未操作生产、未启动实例、未发送 import。两份归档已有 SHA256，但不构成八类发布夹具。
- [x] 后端非缓存完整 test、专项 import test、vet/build 通过；前端 save-import 专项、typecheck/build 通过；静态门禁未发现旧导入回退、默认 takeover、日志/单指针成功判定或平台 ID 持久泄漏。
- [ ] 准备普通单机、多人主机、升级住宅、家具/冰箱/地窖、配偶/孩子/宠物、farmhand、Mod、自定义 takeover 八类非唯一测试 ZIP，并逐份保留原始 ZIP 与 SHA256。
- [ ] 使用隔离 Panel 实例执行 Phase A、finalize、持久保存、全部故障注入、人工游戏语义和真实第二次重启；桌面和手机都要贯穿同一真实 job。
- [ ] 当前不得把 `SAVE-IMPORT-JUNIMO-1` 标记 completed。上游仍没有 commandId；Panel 已实现但尚待真机闭环的方案是磁盘事务痕迹、pending、saveId、finalizeCount、GameLoop.Saved、dayTransitionComplete 复合证据。
- [ ] 全部 13 项门禁和人工语义通过后，再单独执行发布评审；本任务未提交、推送、创建 release、构建发布镜像或部署生产。

# 2026-07-16 已完成：FE-SAVE-IMPORT-HOST-1

- [x] 桌面和手机上传预览均强制选择原主机角色处理方式；swap 平台 ID 全程为纯十进制 string，takeover 必须二次确认，缺失选择不再有旧请求体或隐式 takeover 回退。
- [x] 两端已接入持久导入 job 的 `202` 响应、jobs/SSE 阶段恢复、防重复门禁与全部稳定错误码；`result_unconfirmed` 为中性警告，`import_recovery_required` 禁止重复操作。
- [x] 共用校验、请求体、阶段映射、刷新恢复和错误映射专项测试通过；`npx tsc -b`、`npm run build` 通过；桌面和 390×844 手机视觉 QA 无横向溢出且 console error/warn 为空。
- [x] 先前 blocked 条目作为后端契约落地前历史保留；当前前端已按 `SAVE-IMPORT-WEB-API-1` 完成适配。

# 2026-07-16 已完成：SAVE-IMPORT-WEB-API-1

- [x] 上传 commit 已接入完整持久导入事务，强制嵌套 `hostHandling`；缺失选择绝不默认 takeover。
- [x] swap 平台 ID 以十进制 string 做后端权威校验，只持久化 operation-salted SHA-256 fingerprint；日志、审计、响应和 journal 不保存原值。
- [x] token reserve、operation ownership、journal、专用 job 和 `202 {jobId,operationId,saveName}` 已形成 durable 链；同 token 重试返回原 job，owned token cancel 不删除事务数据。
- [x] Web 稳定错误码与运行中/活动导入/同名/低版本边界已固定；handler 不再调用旧覆盖导入、写指针或普通 Start。
- [x] `FE-SAVE-IMPORT-HOST-1` 已完成前端接入：显式提供 `swap_to_player` 或已确认的 `virtual_host_takeover`，并把 platformId 全程保持 string。
- [x] 原 `SAVE-IMPORT-JUNIMO-1 blocked` 历史记录继续保留；当前正式路线是 Panel 黑盒复合证据，缺少上游 commandId 不再构成 blocked。

# 2026-07-16 已完成：SAVE-IMPORT-DURABLE-SAVE-1

- [x] swap finalizer confirmed 后记录主文件 hash/mtime 和 status version，预持久化 commandId 后只提交一次 Control save-now。
- [x] 只接受同 commandId 的 GameLoop.Saved succeeded；failed/unknown/expired 不停止、不重启、不重发 import 或 save-now。
- [x] Saved 后从 post-Saved version 优先长轮询 `/wait/status` 等待新的 dayTransitionComplete=true；字段缺失和超时保持 unconfirmed warning。
- [x] 保存后稳定主文件、严格 XML、hash/mtime 变化均通过才写 `save_verified -> completed`；损坏或无变化进入 recovery。
- [x] as-is 不强制保存；Panel Prepare 可发现 `finalize_confirmed/save_persisting/save_verified` 并启动只观察原 commandId 的恢复 job。
- [x] save-event/自动游戏日回档和 preimport 保留策略不变。

# 2026-07-16 已完成：SAVE-IMPORT-ACTIVATION-1

- [x] Phase A 后优先等待进程内 reload，以 Control RuntimeSaveID 验证目标真实加载，并持续观察 ProcessIdentity/pending/diagnostics/world state。
- [x] reload 未完成时先保护在线玩家、应用目标 Mod profile，再至多受控重启一次；Phase A 已停机则 ComposeUp，任何路径都不重复发送 import。
- [x] swap 同进程要求 pre-submit finalizeCount +1；新进程要求 count=1 或新代 baseline +1，并与 pending clear、`masterName=Server`、目标 saveId、online/day-transition complete 联合确认。
- [x] as-is 不要求 finalizer count；partial/failed、wrong target、pending 持续、diagnostics missing/failed/unavailable 分别进入 recovery/timeout/unconfirmed 稳定分类。
- [x] 成功推进 `finalize_confirmed`，维护 runtime 仍不提前发布 ready/邀请码；DURABLE-SAVE-1 已接续完成写盘与 completed 门禁。
- [x] SAVE-IMPORT-JUNIMO-1 的历史 blocked 记录继续保留；缺上游 import commandId 不再是 blocker，正式路径仍使用黑盒复合证据且禁止自动重放。

# 2026-07-16 已完成：SAVE-IMPORT-PHASE-A-1

- [x] 在 runtime_ready 后重新验证 preimport、目标 hash/pointer/pending、ProcessIdentity、玩家连接和 log offset，并在导入互斥锁内只发送一次正式 FIFO 命令。
- [x] FIFO 写成功即持久化 `import_submitted`；成功日志和单指针变化不推进状态，超时不自动重试。
- [x] swap 以 hash+pending save/owner/fingerprint+pointer 复合确认；as-is 以 hash 不变+无 pending+pointer transition 确认，成功只推进 `import_confirmed`。
- [x] 超时先停 server 并确认退出，再覆盖 no-effect、迟到成功、pointer 缺失、半转换恢复和矛盾 unknown/recovery 分类。
- [x] 半转换从 preimport 隐藏解压并 rename 恢复，恢复后复核提交前 hash；原始 platform ID 不持久化。
- [x] 后续 activation/finalize 与 durable save 代码阶段已实现；`SAVE-IMPORT-JUNIMO-1` 仍未 completed 的剩余门禁是本页顶部记录的真实角色/住宅/家庭语义、故障注入及二次重启 E2E，而不是继续等待上游 commandId。

# 2026-07-16 已完成：SAVE-IMPORT-MAINTENANCE-RUNTIME-1

- [x] `.125` 与挂载 DLL 静态校验在持久 staging 前完成；维护启动后才检查 server/FIFO/log/API/运行中 manifest。
- [x] 新增 `save_import_maintenance` phase：不走普通 Start ready/邀请码/newgame，不改 active pointer，不发送 `saves import`，reconcile 不会误升为 running。
- [x] 以裸 `saves` 做无副作用命令注册探测；玩家连接时明确停止且不踢人。
- [x] `runtime_ready` 持久化 ImportEvidence baseline、server log offset、ProcessIdentity、finalizeCount、pointer/pending/目标 hash；证据不足不推进。
- [x] ready 前失败或取消会停止本 job 启动的 runtime、恢复原 stopped 状态，并保留 staged/preimport。
- [x] PHASE-A-1 已从 `runtime_ready` 实现正式 submission 与 Layer A 黑盒判定；Layer B/activation after-evidence 仍待后续，不能展示整个导入成功。

# 2026-07-16 已完成：SAVE-IMPORT-STAGING-1

- [x] journal 建立后把 token payload 转入 operation 专属持久 `source/`，token 标记 owned；handler 返回和 Panel 重启后均可发现，不再依赖进程内 token/tempDir。
- [x] 新增跨平台 no-replace 原子暂存；同名零修改拒绝，跨文件系统通过 Saves 下隐藏目录复制、全树 fingerprint 校验后发布，失败不暴露半成品。
- [x] `staged` 只在真实发布后写入；`backup_created` 只在上传目标的 `preimport` ZIP 与 SHA-256 成功后写入。`OriginalActiveSave` 不再被误当作 preimport 对象。
- [x] pre-submit cancel 删除 transaction source 与未变化的本次 staged target，保留 preimport；目标变化或 submitted 后拒绝自动清理。
- [x] preimport 可完整恢复且不参与 auto game-day pruning；旧覆盖型 `ImportSaveToVolume` 未被新事务复用。
- [x] STAGING-1 当时未启动/exec Junimo、未发送 import、未修改 XML；后续 MAINTENANCE-RUNTIME-1 接续 baseline，PHASE-A-1 再负责正式一次性提交。

# 2026-07-16 已完成：SAVE-IMPORT-EVIDENCE-1

- [x] 以 `.125` 保存实例卷只读核验 `JunimoHost.SaveImport` 实际文件为 `junimohost.saveimport.json`，实现 missing/null/完整/损坏四态读取，零写入存档与 SMAPI data。
- [x] 实现 `/diagnostics/state` 短超时读取及 API unavailable/timeout/field missing/failedFields 区分；实现双读稳定 hash、active pointer、Control saveId、day-transition 和容器/进程代际快照，无法读取的值保持 unknown。
- [x] 复用 operation-salted SHA-256 比较 pending UserID，结果只暴露 `match/mismatch/unknown`；原始 ID 不进入 JSON、错误、journal、API 或日志。
- [x] 未接上传/正式导入接口，未发送 `saves import`，未使用 `/test/*`，未修改 XML 或存档目录。
- [ ] 下一阶段在现有 transaction journal 上设计并实测黑盒复合判定状态机；单一 diagnostics/pointer/hash 均不得宣布成功，进程变化或证据冲突必须进入 unknown/recovery。
- [x] 保留 `SAVE-IMPORT-JUNIMO-1` blocked 历史；后续方向从等待上游 commandId 改为 Panel 黑盒复合证据适配，缺 commandId 不再单独构成 blocked。

# 2026-07-16 blocked：FE-SAVE-IMPORT-HOST-1

- [ ] **blocked**：SAVE-IMPORT-JUNIMO-1 尚无稳定终态；后端仍默认缺失 hostHandling，且枚举未与前端任务统一。
- [x] 未只靠前端勾选伪造安全保证，未让桌面/手机接入不稳定请求体，未展示未经验证的成功或阶段。
- [ ] 后端先强制拒绝缺失模式并发布稳定契约；随后桌面和手机共用校验、错误映射、job 恢复与视觉验收一次完成。

# 2026-07-16 blocked：SAVE-IMPORT-JUNIMO-1

- [ ] **blocked**：实现前复核确认 `.125` 正式 FIFO 仍无 commandId/机器可读终态；`ImportResult` 只在进程内和 test-only API 可用。
- [ ] 上游补充正式 schema-versioned import result 后再继续维护环境启动、FIFO 提交、受控重启、目标世界/虚拟主机验证、profile 应用和恢复续跑。
- [x] 未解析脆弱日志，未发送真实 import，未启用旧 Go 覆盖路径，未修改 Stardew XML。

# 2026-07-16 已完成：SAVE-IMPORT-TXN-1

- [x] 持久 pending upload、reserve/release/consume/cancel token 生命周期及同 token 并发互斥。
- [x] `stardew_import_save_and_start`、九阶段原子 journal、平台 ID 单向指纹、重启恢复分类和提交前/后清理边界。
- [x] `.125` + 宿主 DLL + live FIFO 门禁；同名 `save_exists` 零写入；相关操作统一 `409 save_import_busy`。
- [x] 上传提交入口断开旧 `ImportSaveToVolume + SetActiveSave` 链，不接前端、不修改 XML。
- [ ] 下一阶段：上游提供 commandId 机器回执后，才实现真实 Junimo 调用、激活验证、token consume 和前端交互。

# 2026-07-16 blocked：SAVE-IMPORT-SPIKE-1

- [x] 使用 JunimoServer `1.5.0-preview.125` 精确镜像、隔离卷和测试存档字节副本，真实覆盖活动/无活动世界、swap/as-is、reload 成功/跳过、非数字 ID、ID collision、损坏 XML、缺主文件、重复导入、pending 前后同/不同 ID 重试，以及 pointer/pending/Control saveId/`/status`/虚拟主机事实。
- [x] 实测 `dayTransitionComplete` 的 false → Saved → true 顺序和 `/wait/status` 字段过滤；明确初始 true 时必须先等 false 边沿。
- [ ] **blocked**：FIFO 导入没有 commandId 或机器可读终态；reload 拒绝/跳过/异步失败不能稳定关联，现场也未具备自动化游戏客户端重跑“玩家在线拒绝”分支。不得用日志文本猜测或自动重试。
- [ ] 推动上游提供带 commandId 的正式 JSON import endpoint/结果文件及全退出路径 reloadStatus；完成后另开实现任务，当前不实现面板导入功能。

# 2026-07-15 已完成：NEWGAME-TXN-1 官方农场创建事务安全化

- [x] handler 只做严格 DTO 解析/规范化/校验和 job 创建；规范化配置持久化为内部 job payload，不再提前写 settings/init/marker。
- [x] job 内建立私有原子事务，快照配置、指针、marker、profile、存档目录和 Mod 状态；结构化 marker 保持旧 Control Mod 的存在性兼容。
- [x] `/newgame` 每事务最多一次，超时/错误不重试；使用目录集合差、主文件稳定、完整 XML 与 whichFarm 做最终判定，多个新目录进入 ambiguous。
- [x] 失败恢复原文件/指针/Mod 状态，验证失败目录移入隔离区；回滚失败拥有独立状态和错误码，原始原因不被覆盖；事务记录支持面板重启后读取。
- [x] Standard、Meadowlands、配置/marker 故障、超时、无目录、多目录、损坏 XML、FarmType 不匹配、一次性调用、Mod 恢复和回滚失败已有 Go 测试。
- [ ] 隔离实例的真实官方农场冒烟需在有明确隔离环境时执行；本次不操作生产实例。
- [ ] 模组农场仍不可创建。下一阶段必须在该事务状态机上叠加 provider/依赖运行时验证，不能直接放宽 custom-new-game。

# 2026-07-15 已完成：FARM-NEWGAME-MOD-PREPARE-1 依赖闭包与一键准备

- [x] 复用现有 relationship index 计算 provider、required/package closure、optional、missing/disabled、conflict 和稳定 readiness；ContentPackFor 强制 required 并去重。
- [x] 新增不依赖 saveName、不持久化 profile 的 `NewGameModSelection`，目录 API 返回实际 dependenciesReady。
- [x] 新增管理员、停服、严格 farmTypeId 的一键准备；与 lifecycle/jobs/Mod 写操作互斥，失败反序回滚，不自动启动或创建。
- [x] 前端展示依赖完整/待启用/缺失/冲突，确认弹窗逐项列出将启用组件；模组 FarmType 仍不能进入创建表单。
- [x] 真实 SVE 只读闭包确认 ready：Frontier provider + SVE CP/FTM/Code + Frontier FTM + Content Patcher + Farm Type Manager；未改变实例状态。
- [ ] 下一阶段：启动前/运行时验证 SMAPI 实际加载 provider 与闭包，设计最终 saveName profile 提交和完整创建回滚；在此之前 custom-new-game 不开放模组 FarmType。

# 2026-07-15 已完成：FARM-CATALOG-READONLY-1 农场目录 API 与只读展示

- [x] 管理员目录 API 始终返回 8 种官方农场；扫描失败和部分损坏安全降级，modded 条目固定不可选并标记需要运行时验证。
- [x] 受控图标端点只消费扫描器 token，每次读取重新验证 provider containment、符号链接、文件头、格式、尺寸与大小，不暴露宿主路径。
- [x] 新建游戏页保留原官方静态选择与创建逻辑，新增 FrontierFarm 等模组农场只读卡、状态/冲突提示和图片/API 失败降级。
- [x] 模组卡片按实机截图收紧为单列全宽横卡；目录内部解析 warning 不再展示，只呈现成功识别的农场卡片。
- [x] 后端全量测试、前端目录状态测试与 production build 通过。
- [ ] 当前机器无面板监听，未启动真实实例，因此登录态页面的“边境农场”只读卡验收待下次已有面板运行时补做；阶段 2 的真实 SVE 文件扫描结果已确认。
- [ ] 下一阶段：计算依赖就绪状态、结合运行时加载证据验证 FarmType/provider，再设计可选与创建契约；在此之前 `custom-new-game` 继续只接受官方农场。

# 2026-07-15 已完成：FARM-CATALOG-DISPLAY-1 离线农场展示元数据

- [x] 按 `zh-CN/zh → default → manifest.Name → FarmType ID` 生成安全 `Label`，从第一个下划线拆分并限制说明。
- [x] 只解析同包 `Strings/UI` 的精确 i18n token，不执行任意 Content Patcher token 或条件。
- [x] 将 `IconTexture` 映射到同包 Load 相对文件，完成路径/符号链接 containment、格式、真实文件头、大小和尺寸校验。
- [x] 图标失败保持农场条目并返回空 `IconFile`；结果不含宿主绝对路径，`WorldMapTexture` 不参与图标或地图渲染。
- [x] 合成测试与真实 SVE 只读验证确认 `FrontierFarm → 边境农场 → Assets/Tilesheets/Icon.png (PNG 22×20)`。
- [ ] 当前仍未增加 Web API、前端页面或模组农场创建；后续阶段需单独设计受控图标读取契约与冲突 provider 选择。

# 2026-07-15 已完成：FARM-CATALOG-OFFLINE-1 离线模组农场扫描基础

- [x] 停服状态扫描 `.local-container/mods` 与 `mods-disabled`，记录 manifest/provider 与启用状态。
- [x] 仅从 Content Patcher `EditData -> Data/AdditionalFarms -> Entries` 读取权威 ID，排除 FTM 和所有普通 FarmType/FarmTypes 条件。
- [x] 复用 JSONC 解析，安全支持 Include、嵌套、条件保留、循环/逃逸/大小/深度限制。
- [x] 提供稳定结果、同 Mod 去重、跨 Mod 冲突状态与来源列表，并以合成 fixture 覆盖安全和错误边界。
- [x] 后续 `FARM-CATALOG-DISPLAY-1` 已补充农场显示名称、i18n 与安全图标元数据；仍无 Web API、前端入口和模组农场创建能力。

# 2026-07-15 已完成：PANEL-0.3.2 / JUNIMO-MOD-RUNTIME-SYNC-1

- [x] 识别 Compose bind mount 遮蔽镜像内新版 Junimo DLL 的真实升级缺口。
- [x] 将目标 Mod 提取、严格校验、原子替换和旧 Mod 恢复纳入升级事务。
- [x] 启动路径按 `.env` 目标版本自愈宿主 Mod，FIFO 验收精确比对实际加载版本。
- [x] 补充成功、目标包错误、实际加载旧版本及通用回滚测试，并将最低 Panel 版本提升到 `0.3.2`。

# 2026-07-15 已完成：PANEL-0.3.1 / JUNIMO-CONFIG-REPAIR-1

- [x] 可信旧候选混合配置可被精确标记为 `repairable`，自定义/未知配置继续安全拒绝。
- [x] 管理员一次点击完成私有备份、原子规范化、复检、dry-run 和 apply，不再要求手改 `.env`。
- [x] 安装流程不再混合不同 server tag 的候选，从源头阻止同类状态复发。
- [x] 后端配置/API 回归、前端生产构建、桌面与 390px 视觉验收覆盖完成。
- [x] 使用新 Tag `v0.3.1` 触发三仓库镜像与 GitHub Release 标准发布链路。

# 2026-07-15 已完成：MODBUNDLE-1

- [x] Mod ZIP 递归发现多层 manifest，支持整个 Mods 文件夹重新打包上传。
- [x] 禁止静默部分安装；深层无效 Mod、重名和嵌套歧义均原子失败。
- [x] 兼容 Windows GBK/GB18030 ZIP 中文路径、大小写入口文件和数字 `UpdateKeys`。
- [x] 桌面/手机端显示 ZIP、发现、安装和启用数量。
- [x] 用 3095 条目/38 manifest 的 `Mods1.zip` 在隔离目录和本机真实存档完成 38/38 导入与启用验证。
- [x] 聚合 ZIP 按真实子包记录稳定 `packageKey`，保留旧 Nexus 同包删除兼容，阻止无关 Mod 继承第一个 Nexus ID。
- [x] 内容包卡片恢复 `[CP]`/`[FTM]` 前缀；配偶助手数字 `UpdateKeys` 不再生成解析失败卡片。
- [x] 已安装列表展示全部本地 Mod，Nexus 来源仅增强卡片信息，不再决定用户是否有权查看该 Mod。

# 2026-07-15 已完成：PANEL-0.2.10-RELEASE

- [x] 组件升级任务代际修复已完成本地延迟竞态点击验证和完整发布门禁。
- [x] 发布门禁已包含 `test:component-update-flow`，后续未通过该回归测试不得打包。
- [x] 使用新 Tag `v0.2.10` 发布，不覆盖历史 Tag。

# 2026-07-14 已完成：FE-MAINTENANCE-SINGLE-CARD-1

- [x] 用户卡片内直接完成校验、下载、安装和验收，不再跳转到底部开发者详情。
- [x] 正常 Junimo 更新只保留“立即升级”；错误/人工恢复原因直接显示在卡片。
- [x] 清理总览中过期的“阶段一不会升级”文案。

# 2026-07-14 已完成：JUNIMO-ROLLBACK-TAG-RESTORE-1

- [x] 回滚重建期间使用精确 image ID，退出时恢复原始 tag 配置，避免裸 digest 破坏版本检测。
- [x] `rollback_failed`、`invalid_config` 和读取失败不再显示“已是推荐版本/不用做任何事”。
- [x] 增加后端回滚终态和前端维护判断回归测试。

# 2026-07-14 已完成：RUN-SH-LATEST-UPDATE-1

- [x] `run.sh update/force-update` 未指定版本时自动解析最新正式 Release，不再被 `.env` 中旧的精确镜像 tag 截停。
- [x] 日常启动/重启继续固定当前版本；最新版本解析失败时安全终止，不伪报旧版本更新成功。
- [x] 增加 shell 回归测试并接入 tag release gate，覆盖自动目标、显式目标与解析失败。

# 2026-07-14 已完成：RUNTIME-MATRIX-MIRRORS-1

- [x] Junimo server 与 steam-auth-cn 升级矩阵候选顺序和安装流程完全统一。
- [x] 所有别名强制绑定单一 canonical digest，拉取失败或校验失败自动回退，仍拒绝自定义升级目标。
- [x] 增加安装/矩阵顺序一致性测试，以及 Go/Python 同 digest 门禁。
- [x] release gate 区分必需仓库与可回退第三方代理，兼顾制品安全和发布可用性。

# 2026-07-14 已完成：PANEL-UPDATE-HISTORY-STALE-1

- [x] 真实当前版本高于历史成功目标时，不再出现“先显示新版本、再闪回旧版本”。
- [x] succeeded/failed_rolled_back 仅在与当前版本相关时主导页面；活动任务与 rollback_failed 优先级保持不变。
- [x] 增加异步加载旧 apply 状态的精确回归测试。

# 2026-07-14 已完成：PANEL-UPDATE-CONTINUOUS-1 连续升级修复

- [x] 历史成功升级不再覆盖后来发现的更高版本，也不再永久锁住下一次升级。
- [x] 旧 dry-run 必须与当前最新目标精确匹配，避免把上一次版本的环境检查误用于下一版本。
- [x] 增加“历史成功 + 新版本”回归测试，并保留 active、同目标成功和 rollback_failed 安全门禁。

# 2026-07-14 已完成：FE-DIAGNOSTICS-USER-FIRST-1 服务器健康页重构

- [x] 默认视图改为“整体是否正常 → 有没有要处理的版本维护 → 具体检查结果”，降低普通使用者阅读成本。
- [x] 状态来源、兼容矩阵、镜像/构建信息、预检与升级日志集中到默认折叠的维护详情，原功能与安全门禁均保留。
- [x] `.121 → .125` 以“不升级仍可继续使用”的可选推荐呈现；管理员可从摘要定位到预检区，普通用户不出现升级操作。
- [x] 增加 QA 更新状态 fixture，并完成桌面主视图、详情展开、无溢出和无控制台错误验收。

# 2026-07-14 已完成：Panel 0.2.2 推荐 JunimoServer preview.125

- 内嵌推荐矩阵、新实例默认镜像和 `TestedImageTag` 已切换到 `.125`，保留 auth-cn `1.5.0-anxi.2` 的真实 `.121` 溯源并记录跨版本协议兼容验证。
- `.121` 不强制升级：现有实例继续可用，只显示推荐更新；管理员仍通过现有 dry-run/确认/apply/回滚闭环自愿升级。
- `.125` 的 23 个 init 兼容挂载继续保留并通过实镜像脚本验证。后续可独立评估旧联机存档 host-swap 向导，不纳入本次 0.2.2 发版范围。

# 2026-07-13 已完成：JUNIMO-STACK-UPDATE-1 阶段二

- 已完成 server + steam-auth-cn 成对 dry-run：严格空 POST/GET、专用 job/双向互斥、可信候选 inspect/pull/digest、Compose/认证卷/运行态/磁盘 warning 检查、脱敏持久状态和前端刷新恢复/进度展示。
- 阶段一内置推荐版本对及推荐 tag 不变；阶段三 apply、备份、停服重建、成对写回与失败回滚尚未实现。

# 2026-07-13 已完成：JUNIMO-STACK-UPDATE-1 阶段一

- 已完成构建内置、强校验的 Junimo server + steam-auth-cn 推荐版本对清单，实例 `.env` 五字段只读检测、五态模型、管理员详情 API、脱敏 runtimeDiagnostic、总览更新提示和诊断页整体展示。
- 推荐版本保持当前实测的 server `1.5.0-preview.121` + steam-auth-cn `1.5.0-anxi.2`；不跟随远程 latest，不做 preview/anxi semver 排序，自定义镜像不判断可覆盖。
- 阶段一明确不包含 pull、修改 `.env`、stop/recreate、dry-run、apply、升级备份或回滚；阶段二/三列入 `docs/07-later-optimizations.md`，后续需独立安全设计和授权。

# PANEL-UPDATE-RELEASE-1 状态（2026-07-13）

- **已完成，随 v0.2.0 发布**：版本检测、独立 updater/dry-run、apply/回滚、完整 Web 交互和隔离真 Docker 发布闭环均已完成。
- 发布阻塞修复：helper 保持宿主 Compose 绝对路径，避免升级后 Compose labels 指向临时 `/deployment`；已新增 contract regression tests。
- 已验证成功升级与 unhealthy 自动回滚、数据库恢复、游戏服务容器不变、断线重连、权限/并发/unsupported 边界、桌面和移动布局。
- v0.2.0 作为首个包含 Web updater 的正式版本发布；从不含 updater 的历史版本进入 v0.2.0，需要沿用现有部署更新方式完成一次引导升级，后续版本即可在面板内升级。

# 2026-07-13 已完成：FE-PANEL-UPDATE-1 完整 Web 面板升级交互

- 已完成全局 `PanelUpdateProvider/usePanelUpdate`、顶栏/总览双入口同步、管理员二次确认、普通用户只读、完整阶段时间线和桌面/移动统一弹窗。
- 面板预期断线进入专用全屏状态，以 `/health`、`/api/version`、apply 状态退避重连；恢复后保留原路由并自动打开成功、已回滚或恢复失败结果。
- 状态机测试覆盖权限、成功、活动阶段、回滚、终态、双入口派生和退避；浏览器 QA 覆盖 1280 桌面、900 窄屏、390 移动端、普通用户、回滚、离线及断线后成功恢复。
- 完整 Web 升级闭环至此完成；后续只保留历史记录/通知增强，不扩展为任意镜像、任意服务或 shell 操作。

# 2026-07-13 已完成：PANEL-UPDATE-APPLY-1 面板升级后端执行链路

- 已完成管理员 apply API、SQLite 在线一致性备份、独立 helper 精确版本拉取、panel 单服务重建、Docker health + `/health` + `/api/version` 三项验收，以及失败自动恢复数据库/Compose/`.env`/旧镜像。
- 状态跨 panel 重启持久化，终态区分 `succeeded`、`failed_rolled_back`、`rollback_failed`；并发、dev/相同/降级、unsupported、任意 body/镜像和普通用户均拒绝。
- 隔离临时 Compose 真 Docker 测试确认 panel 可替换且 game 哨兵容器不变；脚本化 contract tests 覆盖拉取、重建、unhealthy、版本不匹配、备份失败和回滚成败。
- 后续完整升级交互已由 `FE-PANEL-UPDATE-1` 完成，不扩展服务范围或发布流程。

# 2026-07-13 已完成：PANEL-UPDATER-DRYRUN-1 独立 Updater 与部署环境演练

- 已完成 Docker 自容器 inspect、Compose labels/显式环境兜底识别、能力响应、管理员 dry-run API、独立镜像内 `panel-updater` 和跨主进程重启可读的原子状态文件。
- dry-run 目标只来自项目硬编码可信仓库并使用精确版本 tag；helper 只执行 image inspect/pull 与 Compose config，不执行当前面板的停止、删除、重建或重启。
- 管理员更新弹窗可发起“检查升级环境”并查看 supported/unsupported 与脱敏日志；普通用户保持只读版本展示。本阶段仍无“立即升级”。
- 支持模式：标准 run.sh/单文件 Compose panel 服务；或 inspect 挂载与四个显式宿主机变量完全一致的兜底。缺 Docker Socket/Compose、普通 docker run、缺失/冲突 labels、多 compose 文件和不可验证自定义编排均安全拒绝。

# 2026-07-13 已完成：PANEL-UPDATE-CHECK-1 面板版本自动检测与展示

- 后端已完成稳定 GitHub Release 检测、语义版本比较、启动检查、6 小时抖动调度、成功结果缓存与失败保留，并提供登录可读/管理员可刷新的版本状态接口。
- 前端已完成共享状态、桌面顶栏与总览原区块复用、移动端入口及统一更新详情弹窗；有更新时顶栏和总览统一显示“发现新版本 vX.Y.Z”，普通用户只读，管理员可手动刷新。
- 该阶段边界原为“检测和展示”；后续执行链路现已由 `PANEL-UPDATE-APPLY-1` 独立完成。

# 2026-07-10 已完成：手机端背景、顶栏与存档卡片继续优化

- `MOBILE-SHELL-SAVES-REFINE-1` completed：手机端整体背景改为复用 PC 端 `background_app_black.png` 深色纹理，主体区域复用 PC `.sd-main` 的 image2 页面框素材（四角、四边、中心 tile）；顶栏新增 `mobile_topbar_framed_generated_image2.png` 手机端专用四边框木纹横栏素材（imagegen 生成后整张缩放到 1170×174，不裁切，保留完整边框）并保留鸡图标做轻量化移动版，避免手机宽度下出现 PC 顶栏拼接接缝/撕裂感。移动壳固定为一屏，主体背景、顶栏和底栏保持不动；内容改由 `.sd-mshell-scroll` 内层滚动并被 `.sd-mshell-body` 上下边框安全区裁切，不再遮挡背景自带木纹框线；底部 Tab 占位挪到内层滚动区底部 padding，页面框背景铺满到屏幕底部，不再露出黑底；上方滚动内容额外 padding 收为 0，且 `.sd-mshell-body` 上方安全区减半，组件更贴近上方裁切区内侧顶格显示。
- 手机端存档页删除独立大农场原画卡，农场图缩小为核心信息卡左侧约 72px 宽的 16:9 缩略图，和存档名称、农场名称、农场主、游戏日期合并展示；当前使用中/可用状态徽章放在缩略图上方，不再覆盖图片。
- 手机端服务器模组页恢复隐藏内置/系统运行组件，只展示用户安装 Mod；搜索页和启用/禁用接口逻辑不变。
- 验证：`cd frontend; npx tsc --noEmit -p .` 通过；`cd frontend; npm run build` 通过（仅 Vite 主 chunk 大小提示）。

# 2026-07-10 已完成：手机端模组页 M7

- `MOBILE-MODS-M7-1` completed：手机端底部 Tab 已在“玩家”和“存档”之间新增“模组”，移动壳底栏调整为 6 个入口。新增 `MobileModsPage`，页面右上角保留刷新、导出、上传 Mod，内部二级 Tab 为“搜索 / 服务器模组”。
- 搜索页复用现有 Nexus 搜索接口和 `/mods` 安装状态数据，但按用户反馈做成移动端精简展示：删除搜索框上方 Nexus Key/扩展连接区，去掉安装按钮，移除来源/作者/下载/认可四项，搜索结果每页 4 个，卡片底部前置状态按钮与小号“跳转 N站”按钮同排平齐，分页控件收为单行。
- 服务器模组页把已安装信息和启用状态合并在同一张卡片中：左侧展示 N 站缩略图，右侧展示名称、状态、版本、文件夹、更新时间和同步类型；作者/来源字段已按反馈删除。真实可点击启用开关改为底部标签行右侧的无文字绿色小开关；进入该子页时主动刷新 `GET /mods`，切换启用状态后刷新当前列表。后续 `MOBILE-SHELL-SAVES-REFINE-1` 已将该页口径改回隐藏内置/系统运行组件。移动壳主体改为顶部对齐，避免切换搜索/服务器模组时顶部模块上下跳动。
- 验证：`cd frontend; npx tsc --noEmit -p .` 通过；`cd frontend; npm run build` 通过（仅 Vite 主 chunk 大小提示）。

# 2026-07-10 已完成：手机端卡片与底部 Tab 视觉统一优化 M6

- `MOBILE-VISUAL-UNIFY-M6-1` completed：手机端所有主要卡片（总览/控制/玩家/存档各页面的 `.sd-panel` 卡片）从直角方块升级为和 PC 总览页一致的圆角羊皮纸卡片。背景色取自 PC 总览页"存档/模组"指标卡（`linear-gradient(180deg, rgba(255,245,214,0.96), rgba(248,226,174,0.94)), #f7e3ad`），圆角/边框/阴影取自 PC 总览页"在线玩家"卡片（`border-radius:9px`、`border:2px solid #a06c2c`、三层 inset + drop shadow）。
- 实现方式：`StardewMobileShell.css` 新增 `:root` 变量 `--stardew-mobile-card-bg/border/radius/shadow`，配合 `.sd-mshell .sd-panel` 祖先限定选择器（优先级 (0,2,0) 稳赢全局单类 `.sd-panel`），一条规则覆盖所有手机端页面内的 `.sd-panel` 卡片——不需要逐个页面改 className。作用域限定在 `.sd-mshell` 内，桌面端 `StardewPanel.tsx`、登录页 `.sd-auth-shell` 不受影响。
- 玩家页内每张玩家行卡片（`.sd-mplay-player-card`）也同步引用同一组变量，从 `border:1px solid #dcc898; border-radius:2px; background:rgba(...)` 升级为和外层卡片一致的圆角羊皮纸样式。
- 底部 Tab 栏全面重做：从贴底直角硬条改为悬浮圆润胶囊式导航条（`border-radius:20px`、两侧各留 `10px` 间距、`bottom:10px + safe-area-inset-bottom`），每个 Tab 按钮变成垂直图标+文字的圆角 pill（`border-radius:14px`、`min-height:48px` 满足触控热区）；新增从桌面导航共用的像素图标（5 个 tab 各对应一个 image2 nav icon）；active 态 `background:var(--sd-green-bg)` 绿色高亮；`:active` 有 `scale(0.92)` 按压反馈；文案用 `text-overflow:ellipsis` 防溢出。
- `.sd-mshell` 的 `padding-bottom` 从 `56px + safe-area-inset-bottom` 提升到 `84px + safe-area-inset-bottom`，匹配新底栏更大的浮动占位。
- 未改动任何 TSX 业务逻辑（hooks/store/API/权限判断），仅在 `StardewMobileShell.tsx` 给 `MOBILE_TABS` 补了 `icon` 字段和 Tab 按钮内部的 `<img>` + `<span>` 结构。
- 影响文件：`frontend/src/games/stardew/StardewMobileShell.css`、`frontend/src/games/stardew/StardewMobileShell.tsx`、`frontend/src/games/stardew/mobile/MobilePlayersPage.css`。
- 验证：`cd frontend; npx tsc --noEmit -p . && npm run build` 通过；构建产物 CSS 确认 `.sd-mshell .sd-panel` 规则生效、tabbar `border-radius:20px` 规则生效。**未做真实浏览器/真机视觉验证**：建议下一位维护者 `npm run dev -- --host 0.0.0.0` 在手机访问 `http://192.168.0.5:5173/`，或 `qa-layout.html?shell=mobile&state=running` 在浏览器缩放到 390×844/393×852/430×932 确认：卡片圆角明显、底栏圆润浮动、Tab 图标和文字间距合理、active 态清晰、无横向滚动、登录页不受影响。

# 2026-07-10 已完成：移动端玩家页 M4

- `MOBILE-PLAYERS-M4-1` completed：`StardewMobileShell` 的“玩家”Tab 从占位卡换成真实页面 `frontend/src/games/stardew/mobile/MobilePlayersPage.tsx`。**最终结构**（经用户两轮反馈调整后）：页面只有一张“在线玩家”卡，卡片头部右上角是刷新按钮，下方是玩家卡片列表——每张卡片姓名+状态徽章（在线/等待/离线/未知）、主机/角色标签、最近活动文案、底部左侧位置信息 + 右侧“踢出”“封禁”按钮，空态显示“暂无在线玩家”。**不含**顶部统计卡和独立的“待授权玩家”同意/拒绝卡片（用户明确要求删除，待认证玩家的同意/拒绝能力保留在“总览”Tab 的 `MobileHomePage.tsx`）。
- 全部复用既有接口，未新增后端 API：`kickPlayer`/`banPlayer`/`dashboardData.refreshPlayers`，`disabled`/`title` 门控逐条对齐桌面 `PlayersPage.tsx` 行内图标按钮。
- 忙碌态精确到单个玩家（`kickBusyId`/`banBusyId` 存目标 ID 而非单一 boolean），操作中该玩家按钮显示“处理中…”，同时锁定其它玩家行的踢出/封禁按钮避免并发误触。
- 桌面端 `PlayersPage.tsx`/`StardewPanel.css` 未改动；新增样式 `frontend/src/games/stardew/mobile/MobilePlayersPage.css`（class 前缀 `sd-mplay-`），只用全局 `stardew-theme.css` 工具类。
- 详见 `docs/frontend-handoff/frontend-handoff-2026-07-10.md` 的 `MOBILE-PLAYERS-M4-1` 小节（含两轮反馈调整的完整记录）。验证：`cd frontend; npx tsc --noEmit -p . && npm run build` 通过。**未做真实浏览器/真机视觉验证**：当前环境没有可用的浏览器自动化/截图工具，建议下一位维护者用 `npm run dev -- --host 0.0.0.0` 或 `qa-layout.html?shell=mobile&state=running` 在 390×844/393×852/430×932 下确认无横向溢出、卡片底部“位置信息+操作按钮”一行不挤压文字、真实连一个运行实例走一遍踢出/封禁完整链路。

# 2026-07-10 已完成：移动端控制页 M3

- `MOBILE-CONTROL-M3-1` completed：`StardewMobileShell` 的“控制”Tab 从占位卡换成真实页面 `frontend/src/games/stardew/mobile/MobileControlPage.tsx`，按用户口径限定为桌面 `ServerControlPage.tsx` 的“全服消息”卡片 + “快捷操作”卡片（去掉手动备份和 VNC 显示相关按钮），不含生命周期启停（已在 `MobileHomePage` 的“快捷控制”卡提供）。快捷操作按单列纵向按钮列表排布 5 个操作：计划重启、服务器密码设置、小屋与联机高级设置、触发节日活动、永久启用 Joja 路线，4 个表单类操作各自弹出全屏弹窗（`max-height:88vh; overflow-y:auto` 防止内容溢出小屏视口）。
- 全部复用既有接口，未新增后端 API：`getRestartSchedule`/`updateRestartSchedule`/`getInstanceServerPassword`/`updateInstanceServerPassword`/`getInstancePasswordStatus`/`getInstanceServerRuntimeSettings`/`updateInstanceServerRuntimeSettings`/`triggerFestivalEvent`/`enableJojaRoute`/`sendSay`。未直接复用 `ServerControlPage.tsx` 组件或它挂在 `StardewPanel.css` 里的类名（移动端不加载该文件），只搬了状态判断逻辑，弹窗/按钮/提示条按移动端布局重排，视觉基础件继续用全局 `stardew-theme.css` 的 `.sd-panel`/`.sd-input`/`.sd-btn-*`/`.sd-notice--*`。
- 顺手清理：全服消息卡片里过时的“该命令当前版本可能返回‘命令不支持’”提示文案（PC 端 `ServerControlPage.tsx` 和移动端一起删除，SMAPI say 命令现已正常支持）；移动端“发送”按钮按用户要求从 `sd-btn-green` 改成 `sd-btn-restart`（棕色，和重启按钮同色），PC 端保持 `sd-btn-green` 不变，两端故意区分颜色。
- 桌面端 `ServerControlPage.tsx` 行为视觉不变（除上述文案删除）；新增样式 `frontend/src/games/stardew/mobile/MobileControlPage.css`（class 前缀 `sd-mctrl-`）。
- 详见 `docs/frontend-handoff/frontend-handoff-2026-07-10.md` 的 `MOBILE-CONTROL-M3-1` 小节。验证：`cd frontend; npx tsc --noEmit -p . && npm run build` 通过。**未做真实浏览器/真机视觉验证**：当前环境没有可用的浏览器自动化/截图工具，建议下一位维护者用 `npm run dev -- --host 0.0.0.0` 或 `qa-layout.html?shell=mobile&state=running` 在 390×844/393×852/430×932 下确认无横向溢出、5 个快捷操作按钮和 4 个弹窗渲染正常（尤其计划重启弹窗字段最多，需确认可纵向滚动到底部按钮）、非管理员禁用态。

# 2026-07-10 已完成：移动端总览页 M2

- `MOBILE-HOME-M2-1` completed：`StardewMobileShell` 的“总览”Tab 从 M0 的静态占位卡换成真实页面 `frontend/src/games/stardew/mobile/MobileHomePage.tsx`（新目录 `mobile/`，为后续 M3 移动端页面预留位置），按单列卡片流展示：①状态摘要卡（存档名/服务器状态/在线玩家/版本，状态文案区分“启动中/停止中”）；②邀请信息卡（邀请码 + 局域网邀请地址，各带复制按钮，长文本用等宽小字 + `break-all` 防撑破卡片）；③快捷控制卡（启动/停止/重启，按钮 `min-height:44px`，停止/重启带确认弹窗）；④待认证玩家批准卡（复用密码认证桥接批准能力）。
- 全部复用既有数据与接口，未新增后端 API：`useStardewDashboardData()` 的 `instanceState`/`saves`/`players`/`jobs`/`versionInfo`/`inviteCode`/`publicIP` 及其刷新函数；生命周期 `startInstance`/`stopInstance`/`restartInstance`；玩家页已有的 `getInstancePasswordStatus`/`approvePlayerAuth`。邀请码/局域网地址复制逻辑参照 `InviteCodeCard.tsx` 重写为移动端轻量展示（未复制其 API 请求逻辑，只读同一份 `dashboardData` 字段）。
- 桌面端 `OverviewPage.tsx`/`StardewPanel.css` 未改动；样式新增 `frontend/src/games/stardew/mobile/MobileHomePage.css`（class 前缀 `sd-mhome-`），按钮沿用 `stardew-theme.css` 全局 `.sd-btn-start`/`.sd-btn-stop`/`.sd-btn-restart`/`.sd-btn-green`/`.sd-btn-tan`/`.sd-btn-delete`/`.sd-panel`/`.sd-notice--*`/`.sd-tag` 等既有工具类（这些类在 `main.tsx` 全局加载，桌面专属的 `StardewPanel.css` 才不可用），只用 `min-height`/`width` 覆盖尺寸做触控热区放大，不重画按钮贴图。`StardewMobileShell` 新增接收 `user` prop（M0 遗留的“暂无 user”限制在本轮补上，`App.tsx`/`qa-layout-main.tsx` 同步传入）。
- 详见 `docs/frontend-handoff/frontend-handoff-2026-07-10.md` 的 `MOBILE-HOME-M2-1` 小节。验证：`cd frontend; npx tsc --noEmit -p . && npm run build` 通过。**未做真实浏览器/真机视觉验证**：当前环境没有可用的浏览器自动化工具，建议下一位维护者用 `npm run dev -- --host 0.0.0.0` 或 `qa-layout.html?shell=mobile` 在 390×844/393×852/430×932 下确认无横向滚动、四张卡片渲染正常、复制按钮反馈、启停按钮状态刷新、非管理员禁用态。

# 2026-07-10 已完成：修复登录页移动端布局崩坏

- `LOGIN-MOBILE-FIX-1` completed：用户提供真机截图反馈登录页在手机浏览器里布局崩坏（标题横条压住顶部鸡形徽标、卡片被横向撑爆、软键盘弹出后登录按钮被遮挡），且"没有输入法的时候也有突出，只是没有这么严重"。根因是登录/初始化页永远使用的 `.sd-auth-shell--image-login` 版式把整张桌面原型图当卡片背景，用固定 16:9 比例反算出 `position:absolute` 大盒子并用百分比坐标摆放输入框；手机竖屏宽高比和 16:9 差异很大，算出来的盒子宽度会远超视口（390px 宽视口下接近 1500px），被外层 `overflow:hidden` 裁掉，且已有的 `@media(max-width:700px)` 手工坐标补丁是按单一机型手算的偏移量，换一个宽高比就会跟着错位。
- 只改了 `frontend/src/App.css` 一个文件：新增 `@media (max-width:768px)` 覆盖，作用域限定在 `.sd-auth-shell`/`.sd-auth-shell--image-login` 及其子选择器，`<=768px` 时整体放弃绝对坐标定位，改回真实文档流的羊皮纸卡片（`width:min(100% - 24px, 420px)`，`min-height:100dvh` 处理地址栏收缩，`overflow-y:auto` 允许纵向滚动，输入框 `min-height:40px`/字号锁 16px 防 iOS 自动缩放，按钮 `min-height:44px`）。卡片装饰复用现有 `background_parchment_tile.png`/`button_primary_small_green_blank.png`；背景第一版复用 `background_login_home_image2.png`（桌面那张画死了假窗口 UI 的原图）铺满裁切，用户反馈"背景还是 PC 端的登陆窗口，很违和"后改用 `background_login_farm_generated.png`（非 image2 版登录页用的纯像素农场背景，没有假 UI 元素，设计上就是给"背景 + 悬浮真实卡片"用的）。三张都是仓库已有素材，未新增或替换任何图片；未改 API、session、权限或用户初始化逻辑；未改 `MOBILE-SHELL-M0-1` 的 `StardewMobileShell`（两者完全独立，作用域不重叠）。
- 用户看了背景返工后的效果又反馈卡片本体"太丑"，想要接近 PC 端右侧木框招牌的质感。没有去裁桌面大图里的招牌区域当卡片背景（那样等于把坐标错位的老问题换个更小的坐标系重演一遍），而是给卡片加了一枚固定像素尺寸的鸡形徽标（复用顶栏已有的 `icon_topbar_chicken_image2_v2.png`）骑在卡片顶部边框上，配合加深的投影和调整过的内边距，呼应桌面招牌"鸡+木框"的视觉但不依赖任何坐标换算。
- 详见 `docs/frontend-handoff/frontend-handoff-2026-07-10.md` 的 `LOGIN-MOBILE-FIX-1` 小节（含桌面版坐标选择器 specificity 的踩坑记录、背景素材两轮返工原因）。验证：`cd frontend; npx tsc --noEmit -p . && npm run build` 通过；`npm run dev -- --host 0.0.0.0` 已启动供真机复测。**未做真机复测确认**：还需要在真机上刷新页面确认标题横条不再压住图标、卡片不再横向撑爆、背景不再显示违和的桌面窗口、卡片顶部鸡形徽标视觉效果、软键盘弹出后登录按钮可滚动到可见范围。

# 2026-07-10 已完成：移动端基础入口 M0

- `MOBILE-SHELL-M0-1` completed：在同一前端工程内新增移动端入口，未新建独立项目。新增通用 `frontend/src/hooks/useMediaQuery.ts` hook；`App.tsx` 用 `useMediaQuery('(max-width: 768px)')` 在进入 Stardew 面板处分流，`<=768px` 渲染新增占位组件 `frontend/src/games/stardew/StardewMobileShell.tsx`（顶部品牌 + 状态徽章、复用 `.sd-panel` 羊皮纸样式的“移动端面板建设中”占位卡、底部 5 个静态 Tab：总览/控制/玩家/任务/更多），桌面端继续渲染既有 `StardewPanel`，行为视觉不变。
- M0 不做真实路由/分页、不改后端、不新增 API、不改登录和权限逻辑；`StardewMobileShell` 不接收 `user`/`onLogout`，暂无登出入口，留给 M1。样式独立在 `StardewMobileShell.css`（`sd-mshell-` 前缀），只复用 `stardew-theme.css` 已有变量和工具类，未新增图片素材、未引入新 UI 库。
- QA mock 入口 `frontend/qa-layout.html` 新增 `?shell=mobile` 参数，可配合既有 `?state=running/stopped` 用 mock 数据回归移动端占位壳，不用连真实后端。
- 详见 `docs/frontend-handoff/frontend-handoff-2026-07-10.md` 的 `MOBILE-SHELL-M0-1` 小节（含 M1 注意事项）。验证：`cd frontend; npx tsc --noEmit -p . && npm run build` 通过。**未做真实浏览器缩放/移动设备实测**，当前环境没有可用的浏览器自动化/截图工具，建议下一位维护者用 `qa-layout.html?shell=mobile` 在真实浏览器里确认 768px/390px/320px 下无横向滚动、底部 Tab 贴底、状态徽章和 Tab 高亮切换正常。

# 2026-07-09 已完成：用户管理新增"重置密码"，修正权限规则

- `USER-PASSWORD-RESET-1` completed：面板登录账号（注意不是游戏加入密码）此前完全没有改密码的入口——后端 `PATCH /api/users/{id}` 早就支持 `password` 字段，但从未在前端暴露，而且原有权限检查比预期更严格（连管理员改自己密码都会被拒绝）。按用户明确规则实现：普通用户不能改自己密码（无入口，接口本身就是 `requireAdmin`）；普通管理员能改自己的和普通用户的密码，不能改其他管理员的；第一个注册的超级管理员能改所有人的密码，包括自己。后端 `storage.UpdateUser` 权限检查补一个"改自己"豁免条件；前端"设置"页用户列表新增"重置密码"按钮和弹窗，改自己密码后自动跳转登录页（当前 session 会被撤销）。新增测试 `TestPasswordChangePermissions` 覆盖五种权限场景。详见 `docs/backend-handoff/backend-handoff-2026-07-09.md`、`docs/frontend-handoff/frontend-handoff-2026-07-09.md` 的 `USER-PASSWORD-RESET-1`，以及 `website/docs/handbook/accounts.md` 补充的重置密码说明。验证：`cd backend; go build ./... && go test ./...` 全绿；`cd frontend; npx tsc --noEmit -p . && npm run build` 通过。

# 2026-07-09 已完成：启动/停止/重启按钮补齐管理员权限门控

- `ADMIN-GATE-LIFECYCLE-1` completed：用户凭经验提出"普通用户不能启停服务器/上传 Mod"要求复查，全量排查发现 `ServerControlPage.tsx`（`canStart`/`canStop`/`canRestart` 未算 `isAdmin`）和 `OverviewPage.tsx`（组件根本没取 `user` prop，无 `isAdmin` 概念）的启动/停止/重启按钮缺少管理员权限门控——后端 `requireAdmin` 仍会拒绝，但普通用户会看到"可点击"的按钮，点了才被拒绝，体验不一致。两处均已补上 `isAdmin` 判断和对应 tooltip 提示。复查同时确认 Mod 上传、Nexus Mod 一键安装、游戏安装、玩家踢出、存档、设置、任务日志、诊断导出等其余功能本来就正确限制了管理员权限，未发现其它遗漏。详见 `docs/frontend-handoff/frontend-handoff-2026-07-09.md` 的 `ADMIN-GATE-LIFECYCLE-1`。验证：`cd frontend; npx tsc --noEmit -p . && npm run build` 通过；未做非管理员账号的浏览器实测。

# 2026-07-09 已完成：踢出玩家 + 服务器密码设置/认证状态查询

- `PLAYERS-KICK-1` completed：玩家页踢出功能（行内图标按钮 + "管理操作"卡片下拉+按钮）不再是禁用占位，接入真实后端。上游 JunimoServer 没有踢人 REST API，改为复用面板自带 `StardewAnxiPanel.Control` SMAPI Mod 已有的命令队列机制（和喊话 `say` 同一套通道）：面板写命令文件到 `control/commands/`，Mod 每 120 tick 消费一次，按 `UniqueMultiplayerID` 查找在线玩家并调用 `Game1.server.kick(...)`，禁止踢主机。**已重新编译并替换嵌入的 `StardewAnxiPanel.Control.dll`**，影响现有实例需要重启/重新准备 server 容器才能刷新到最新 Mod。fire-and-forget，前端只能提示"指令已提交"，拿不到精确执行结果。
- `PASSWORD-STATUS-1` completed：`ServerControlPage.tsx` 原"服务器设置"占位按钮按用户要求改名为"服务器密码设置"并接入弹窗（不是新建 `SettingsPage.tsx` 区块）。弹窗内可读写 `.env` 的 `SERVER_PASSWORD`（JunimoServer 只支持容器启动时生效，保存后提示需重启 server 容器），并只读展示 JunimoServer `GET /auth` 代理出的密码保护状态（是否启用、已认证/待认证人数、认证超时秒数、最大失败次数）。
- 详见 `docs/backend-handoff/backend-handoff-2026-07-09.md`、`docs/frontend-handoff/frontend-handoff-2026-07-09.md` 的对应小节。验证：`cd backend; go build ./... && go test ./...` 全绿；SMAPI Mod 用 Docker 官方命令重新编译 `Build succeeded, 0 Errors`；`cd frontend; npx tsc --noEmit -p . && npm run build` 通过。**未做真机联机验证和浏览器截图验证**，建议下一位维护者找测试实例走一遍"设密码→重启→玩家登录→踢人→查看认证状态"完整链路。
- 封禁玩家、白名单管理、权限设置三项仍是禁用占位，未在本轮改动范围内。

# 2026-07-09 已完成：手机端适配新一轮系统性修复

- `FE-MOBILE-FIXES-1` completed：不重构现有断点体系，针对调研定位到的具体移动端问题逐项修复：`.sd-input`（含 `.sd-mods-sync-select`、`.sd-players-action-select`）在 `max-width: 640px` 下字号提到 16px，避免 iOS Safari 聚焦表单自动放大整页；`viewport` meta 补 `viewport-fit=cover`，`#root:has(.sd-shell)` 补 `env(safe-area-inset-*)` 内边距；移动端顶部横向导航图标从 42×38px 提到 44×44px 触控热区（`.sd-shell` 第二行同步从 48px 调到 54px）；`.sd-confirm-dialog` 补 `max-height:90vh; overflow-y:auto` 避免长文案/矮视口溢出；Players 表格与存档备份表格的横向滚动容器补惯性滚动和右侧渐变提示，让移动端用户能发现可横滑。详见 `docs/frontend-handoff/frontend-handoff-2026-07-09.md`。
- 中期路线里的“更完整的移动端导航和表格卡片化”（见下文中期路线）不在本轮范围内，仍待后续单独排期。

# 2026-07-08 已完成：run.sh Docker APT 源同步失败兜底

- `RUN-SH-DOCKER-APT-FALLBACK-1` completed：修复一键脚本在 Ubuntu/Debian 安装 Docker 时只依赖阿里云 Docker CE apt 源的问题。遇到 `File has unexpected size ... Mirror sync in progress?` 这类镜像站同步期索引校验失败时，脚本会先停用系统里历史残留的 Docker apt 源并清理索引，再自动切换到阿里云、清华 TUNA、中科大 USTC、Docker 官方源继续重试。真实现场补充：阿里云 ECS 可能在 `/etc/apt/sources.list` 或其它源文件内残留 `http://mirrors.cloud.aliyuncs.com/docker-ce/...`，新脚本会注释这类旧源，只保留当前托管源 `/etc/apt/sources.list.d/anxi-panel-docker.list`。仅改部署脚本与镜像构建文档，未改面板运行镜像、后端 API 或前端逻辑。

# 2026-07-07 已完成：文档门户网站骨架上线（VitePress + GitHub Pages）

- `DOCS-PORTAL-1` completed：`website/` 下手动搭建 VitePress 骨架（`npm create vitepress@latest` 实测解析到无关第三方包 `create-vitepress@0.0.6`，改用 `npm init` + `npm install -D vitepress` 手动创建），新增 `.github/workflows/docs.yml`（push `website/**` 到 `main` 时自动 `docs:build` 并部署到 GitHub Pages），用 `gh api repos/.../pages -f build_type=workflow` 开通 Pages（Source: GitHub Actions）。已推送并验证线上首页 `https://anxiyizhi.github.io/stardew-server-anxi-panel/` 返回 200。
- `DOCS-PORTAL-2` completed：内容迁移（方案第三节映射表）已完成，`guide/`（2 页）、`deploy/`（4 页）、`maintain/`（4 页，比原方案多一页 `admin.md`）、`faq/`（1 页）共 11 个内容页从 `README.md` / `docs/user-guide/` 搬运改写完毕，`npm run docs:build` 验证无死链。待办：推送到 `main` 触发线上部署（当前线上仍是占位首页）。详见 `docs/11-docs-portal.md`。

# 2026-07-07 已完成：Nexus ZIP 下载断点续传与卡死检测

- `NEXUS-ARCHIVE-RESUME-1` completed：`mod_remote_install` / `mod_nexus_install` 的 ZIP body 下载窗口提升到 20 分钟，远程/Nexus Mod 安装 job 整体窗口确认为 30 分钟；下载阶段新增 `.part` 临时文件和 `Range` 断点续传，服务器支持 `206 Content-Range` 时从已下载字节继续，忽略 Range 返回 `200` 时自动丢弃半包并重下；新增 120 秒无新字节超时，连接卡死会取消当前 attempt 并在 20 分钟窗口内最多重试 4 次。验证：`cd backend; go test ./internal/games/stardew_junimo -run "NexusDownloadArchive"`；`cd backend; go test ./internal/games/stardew_junimo ./internal/web`。

# 2026-07-07 已完成：远程 Mod 重复安装幂等 + 批量失败定位

- `MOD-REMOTE-IDEMPOTENT-1` / `FE-MOD-BATCH-ERROR-FOCUS-1` completed：`mod_remote_install` / `mod_nexus_install` 下载类任务遇到已存在 `UniqueID` 时改为跳过重复目录并成功结束，避免浏览器扩展缓存刷新造成的重复提交把“已经装好”的 Mod 误判为失败；普通手动上传仍保留 `400 mod_exists`。批量进度按钮会标明失败的具体 Mod，带 `jobId` 时点击跳转任务与日志；同时用最新 `GET /mods` 兜底校正旧失败 job 中其实已安装的项。新建 Mod 下载类任务展示名改为 `Mod 名 · 任务类型`，例如 `Ridgeside Village · mod_remote_install`。验证：`cd backend; go test ./internal/games/stardew_junimo ./internal/web`、`cd frontend; npm.cmd run build`。

# 2026-07-07 已完成：修复 mods.go / lifecycle_handlers.go 历史乱码 + 备份分类 bug

- `MOJIBAKE-FIX-1` completed：修复 `mods.go` 和 `lifecycle_handlers.go` 里历史遗留的中文乱码（错误提示、注释），根因是早期某次保存把正确 UTF-8 中文按 GBK 误解码又存回 UTF-8，已用确定性可逆公式全量修复。顺带修复真实功能 bug：备份删除/恢复接口原本因乱码字符串匹配失败，`400`/`404`/`409` 分类全部静默退化成通用 `500`，现已修正。已验证 `go build`、`go vet`、`go test ./...` 全绿，并对 backend 全目录做过乱码特征扫描确认清零。

# 2026-07-07 已完成：Nexus 浏览器扩展适配 Shadow DOM 下载入口

- `NEXUS-MODPAGE-DL-2` completed：扩展 `0.1.1 → 0.1.2`，新增 `deepQueryAll()` 遍历 shadow root 定位下载控件，新增按 `data-tracking` 属性分类的 `findManualDownloadControl()`，新增 `openNexusFileList()`/`waitForFileIdOnPage()` 轮询 `file_id`，替代旧的两步按钮点击模型。仅改浏览器扩展，未改后端接口；发布新镜像后旧实例扩展缓存 ZIP 会被已有版本感知逻辑自动刷新。已验证 `node --check` 和 `go test ./internal/games/stardew_junimo -run TestEnsureNexusInstallerExtensionZip`。

# 2026-07-07 已完成：JunimoServer 静态 init 文件兼容挂载

- `JUNIMO-STATIC-INIT-FIX-1` completed：为 server 容器新增 `.local-container/cont-env/*`、`.local-container/cont-groups/*`、`.local-container/cont-users/*` 兼容挂载，批量遮罩上游 `sdvd/server:1.5.0-preview.121` 中裸静态值被 init 当命令执行的问题。
- 已覆盖 `APP_NAME`、`DBUS_SESSION_BUS_ADDRESS`、`DOCKER_IMAGE_PLATFORM`、XDG 路径、用户/组 id 等真实飞牛现场触发项；旧实例会自动迁移 compose，重启时如本次刚新增挂载会用 `docker compose up` 重建 server 容器。
- 验证：`cd backend; go test ./internal/games/stardew_junimo`；真机热修后 `stardew-server-1` healthy，Junimo API `/health` 返回 ok。

# 2026-07-07 已完成：Nexus 搜索防短断与局域网邀请地址修正

- 已完成：Nexus 搜索后端改用独立 20 秒上下文，避免浏览器刷新、切页、FRP/NAS 链路短断时把上游 GraphQL 请求提前取消并误报 `nexus request failed`。
- 已完成：Nexus 网络类错误新增 `nexus_network_failed`，后端日志保留真实底层错误，前端展示明确网络提示。
- 已完成：“局域网邀请”改为读取当前进入面板的 host；用户用什么 IP/域名加 `:8090` 打开面板，就展示什么 host。

# 2026-07-07 已完成：steam-auth 授权标志收口

- 已完成：`steamAuthLoggedIn` 收口为邀请码卡主 UI 授权标志；在 steam-auth 登录成功日志（`[SteamAuth:*] Logged in as ...` / `[SteamService] ... Logged in as ...`）出现后写 `STEAM_AUTH_COMPLETED=true`，启动/手动刷新成功拿到非空邀请码时也写 true。
- 已完成：启动/重启后如果 server 日志明确出现 `no logged-in accounts`，后端会清空 `STEAM_AUTH_COMPLETED`，前端下一轮状态刷新后显示【登录授权】；`Steam-auth service not ready` 不直接清 false，已有 true 时自动刷新一次 `steam-auth` 服务。
- 已完成：邀请码卡不再用 `steamAuthReady=false` 直接显示重新授权；`steamAuthReady` 保留为诊断字段，主按钮只按 `steamAuthLoggedIn` 显示。
- 已完成：生命周期启动/重启不再等待邀请码，server running 后 job 完成；后端后台最多探测 20 次邀请码，成功后写 `STEAM_AUTH_COMPLETED=true` 与 `/state.inviteCode`，失败不影响服务器和 IP 直连。前端启动中状态改按 active lifecycle job + running/stopping 状态，不再依赖邀请码、在线玩家或 SMAPI 存档加载日志。验证：`cd backend; go test ./internal/games/stardew_junimo ./internal/games/stardew_junimo/config ./internal/web`、`cd backend; go test ./...`、`cd frontend; npm.cmd run build`。

# 2026-07-06 已完成：SteamCMD HOME 与缓存清理加固

- 已完成：SteamCMD fallback 以 `steam` 用户运行时显式设置 `HOME=/home/steam`，避免继续使用 `/root/.local/share/Steam` 自更新缓存引发 139 段错误。
- 已完成：Docker volume 清理改为逐个删除并忽略缺失卷，139 后自动清理 SteamCMD runtime cache 的成功率更高；真实失败会把 Docker 输出写入任务日志。
- 已完成：139 后清理 SteamCMD runtime cache 前，先按 volume 查找并删除残留 one-shot SteamCMD 容器，解决真实云服上 `volume is in use - [container_id]` 导致缓存无法清理的问题。

# 2026-07-06 已完成：SteamCMD SDK 分段下载

- 已完成：SteamCMD fallback 在同一个容器内拆成两次 SteamCMD 进程，分别下载/校验 `413150` 和 `1007`，避免 SDK 阶段因登录后切换 `force_install_dir` 触发 SteamCMD 139 段错误。
- 已完成：过滤旧 daocloud SteamCMD 候选，并在 SteamCMD exit code 139 时自动清理自更新缓存卷后重试一次。

# 2026-07-06 已完成：SDK 后置 SMAPI 预安装

- 已完成：安装流程在游戏文件与 Steam SDK 完成后，通过 JunimoServer 一次性容器预安装 SMAPI，减少 JunimoServer 首次启动时因 GitHub 下载 SMAPI 卡住的概率。
- 已完成：前端把 SMAPI 作为“下载游戏”步骤里的最后一个子状态展示，并区分 `smapi_install_failed` 后置失败。
- 后续可优化：如果后续维护自定义 JunimoServer 镜像，可考虑把 SMAPI 安装包内置进镜像，进一步减少现场 GitHub 下载依赖。

# FE-STEAM-GUARD-SUBMITTED-FEEDBACK-1 状态

- `FE-STEAM-GUARD-SUBMITTED-FEEDBACK-1` completed：Steam / SteamCMD 验证码提交成功后，安装页会显示“验证码已提交，正在等待响应”的本地等待态，不再回到同一个空输入框。等待态保留“重新输入”入口，并在 phase 进入下载、失败或完成后自动清除。

# STEAMCMD-EMAIL-GUARD-PROMPT-1 状态

- `STEAMCMD-EMAIL-GUARD-PROMPT-1` completed：SteamCMD 首次新机器登录时的邮箱 Steam Guard 分行提示已纳入后端和前端识别。后端会把 `This computer has not been authenticated...` / `Please check your email...` / `code from that message` / `set_steam_guard_code` 等日志切到 `steamcmd_guard_required`；前端也会在 job 日志先到时展示 SteamCMD 验证码输入框，避免安装页卡在下载/自更新进度。

# STEAMCMD-CREDENTIAL-REUSE-1 状态

- `STEAMCMD-CREDENTIAL-REUSE-1` completed：基于 SteamCMD 自身的缓存机制实现“一次批准、后续复用”，不共享 steam-auth token。首次完整登录成功后写 `STEAMCMD_AUTH_COMPLETED`；后续使用 `+login <username>` 与 `@NoPromptForPassword` 读取 SteamCMD 缓存，缓存明确失效时同一 job 自动回退完整登录。root/steam 候选镜像的 `Steam` 与 `.local/share/Steam` 统一映射到 `steamcmd-login`，139 重试不再删除授权卷；空统一卷会自动从旧 root/user local 卷迁移 `config/` 与 `ssfn*`。自动化测试通过；真实 Docker 中从旧 root-local 迁移后连续两个全新 SteamCMD 容器均命中 cached credentials、退出码 0、未再次触发 Steam Guard。

# INSTALL-ROUTING-SPLIT-1 状态
- `INSTALL-ROUTING-SPLIT-1` completed：安装流程按用户口述的完整期望重构。把 `reuseCredentials` 粗暴驱动的单一 `steamCMDRetry` 拆成 `reuse` / `steamCMDDirect` / `steamCMDUseCache` 三个正交决策，用已持久化的 driverPhase/state 判“是否已过认证”、用新 `.env` 标志 `STEAMCMD_AUTH_COMPLETED` 判“SteamCMD 是否有缓存”。修复：①镜像拉取失败重试重新拉镜像 + steam-auth（不再误跳 SteamCMD）；②SteamCMD 认证超时重试回到登录验证界面（不再秒报“授权缓存不可用”）；③认证成功后跨会话可靠跳过 steam-auth。并新增「更换 Steam 账号 / 强制重新认证」入口（`forceReauth`：清 steam-session/steamcmd 授权卷 + 重置标志，保留 game-data）；已安装态只保留卡片内换号按钮，操作区不再重复渲染。已验证 `cd backend; go test ./...`、`cd frontend; npm.cmd run build`。

# STEAMCMD-REPAIR-DIRECT-1 状态
- `STEAMCMD-REPAIR-DIRECT-1` completed：安装页“重新安装 / 修复”与复用凭据重试入口已改为直达 SteamCMD 下载/校验。前端不再要求输入 Steam/VNC 凭据；后端收到 `reuseCredentials=true` 后跳过 `steam-auth`，并用已有 SteamCMD 授权缓存执行 app_update。已验证 `cd backend; go test ./internal/games/stardew_junimo ./internal/web` 与 `cd frontend; npm.cmd run build`。

# FE-TOPBAR-BRAND-LIGHTER-2 状态
- `FE-TOPBAR-BRAND-LIGHTER-2` completed：Stardew Shell 左上角品牌标题已按“再细 200”反馈继续减重，`.sd-topbar-brand-text` 字重从 `700` 降到 `500`，暗色描边/投影不透明度同步降低；仅改前端 CSS，未改顶栏状态牌、存档/用户框、API、权限、路由或 Junimo 通信。验证：`cd frontend; npm.cmd run build`；Browser QA 标题 computed `fontWeight=500`，总览/服务器往返后仍为 `500`。

# FE-TOPBAR-BRAND-LIGHTER-1 状态
- `FE-TOPBAR-BRAND-LIGHTER-1` completed：Stardew Shell 左上角品牌标题已按用户反馈再调细，`.sd-topbar-brand-text` 字重从 `800` 降到 `700`，并减少暗色描边/投影层数；仅改前端 CSS，未改顶栏状态牌、存档/用户框、API、权限、路由或 Junimo 通信。验证：`cd frontend; npm.cmd run build`。

# FE-OVERVIEW-HEALTH-SHARE-1 状态
- `FE-OVERVIEW-HEALTH-SHARE-1` completed：诊断页成功执行健康检查后会把结果同步到公共 dashboard 数据层，用户从诊断页返回总览页时“系统健康”卡会显示最新评分和状态，不再保持 `— / 未检查`。普通 dashboard 初始化仍不主动调用 `/api/health/diagnostics`，保留诊断按需触发策略。验证：`cd frontend; npm.cmd run build`；Browser QA 通过总览 `—` -> 诊断页 6 项正常 -> 回总览 `100% / 6项全部通过 / 优秀`。

# PUBLIC-IP-LOOKUP-1 状态
- `PUBLIC-IP-LOOKUP-1` completed：新增 `GET /api/instances/:id/public-ip`，由面板后端检测服务器公网出口 IP，返回 `{ ip, checkedAt, source?, cached }`，成功结果缓存 `10min`，前端手动刷新会强制重新探测。邀请卡片已在邀请码下方新增公网 IP 框和复制/刷新按钮，上方标题保持“邀请码”，下方标题显示“局域网邀请”，公网 IP 失败态不显示复制按钮但两行值框保持同宽，并按用户反馈移除邀请码说明文字。未改 Junimo driver、Docker/Compose、实例状态或邀请码接口。验证：`cd backend; go test ./internal/web`，`cd frontend; npm.cmd run build`。

# FE-MOD-COUNT-FILTER-BUILTIN-1 状态
- `FE-MOD-COUNT-FILTER-BUILTIN-1` completed：总览页模组统计已过滤 SMAPI runtime、`StardewAnxiPanel.Control`、`JunimoServer` / `JunimoHost.Server` 三类内置运行组件；模组统计卡、`已启用 N 个` 和同步包摘要里的已启用/已停用数量均按用户可见 Mod 计算。系统运行组件识别已抽为 `mod-visibility.ts`，由总览页和模组页共用。仅改前端展示口径，未改后端 API、启用切换接口、同步包导出或 Junimo 通信。验证：`cd frontend; npm.cmd run build`。

# DOCKER-POLL-PERF-1 状态

- `DOCKER-POLL-PERF-1` completed：`ComposePs` 已增加默认 `1.5s` 短 TTL 成功结果缓存，并在 `compose up/down/restart` 前后失效，减少状态页、诊断和支持包短时间重复触发 Docker CLI 的开销。
- 前端已停止普通 dashboard 初始化里的健康诊断请求，也移除右侧栏 `/metrics` 常驻轮询；资源指标只在诊断页可见时按 `8s` 间隔采样，后台 tab 隐藏时暂停。
- `DockerVersion` / `ComposeVersion` 保持在 Diagnostics、Docker 状态页、安装前检查或用户手动刷新路径，不进入普通总览轮询。验证：`cd backend; go test -count=1 ./internal/docker`，`cd frontend; npm.cmd run build`。

# ASSET-RUNTIME-SLIM-1 状态
- `ASSET-RUNTIME-SLIM-1` completed：浏览器扩展安装包已从 runtime `zip -r` 改为 `extension-builder` 构建阶段产物，最终 Alpine runtime 不再安装 `zip`；`docs/prototypes` 已从 109 个历史截图/提取文件收敛为轻量索引和 2 张关键总览基准图，完整原型截图改由外部制品承接；超过 300 KB 的运行 PNG 已做无损重压缩并通过像素等价校验；登录背景因色调变化已回退为 PNG-only；favicon 已改为 `.ico` 加 32/64/128 PNG。

# SUPPORT-BUNDLE-STREAM-1 状态

- `SUPPORT-BUNDLE-STREAM-1` completed：支持包导出已改为直接对 HTTP 响应流写 ZIP，不再用内存 `bytes.Buffer` 聚合整个压缩包后一次性返回。下载接口、文件名和 ZIP 条目保持不变；响应不再设置 `Content-Length`，前端应按 Blob 下载处理。已验证 `cd backend; go test ./internal/web -run "SupportBundle|Docker|Metrics"` 与 `cd backend; go test ./...`。

# FE-CLEANUP-UNUSED-ASSETS-1 状态
- `FE-CLEANUP-UNUSED-ASSETS-1` completed：前端生产素材已清理 79 个源码零引用旧 PNG，主要是旧右栏整图、旧顶栏三段、旧导航/字段/图标 sheet 与早期装饰 sprite；`frontend/public/assets` 从约 39.52MB 降到约 18.56MB。同步删除无引用组件 `CommandOutput`、`StatusPill`、`StatusBadge`、`InstanceStateCard`。未改业务逻辑、API、路由或 Junimo 通信；保留动态路径使用的 `new-game` 素材和 QA 入口。已验证素材复扫无非 `new-game` 零引用文件，`cd frontend; npm.cmd run build` 通过。

# FE-MODS-HIDE-SYSTEM-RUNTIME-1 状态
- `FE-MODS-HIDE-SYSTEM-RUNTIME-1` completed：模组页已隐藏 SMAPI runtime、`StardewAnxiPanel.Control` 和 `JunimoServer` / `JunimoHost.Server` 这类系统运行组件；它们不再出现在“添加模组”的已安装卡片或“配置模组”的当前存档启用状态列表中。用户可见“已安装”和解析失败统计同步只计算普通 Mod；玩家同步统计和导出仍保留完整列表逻辑，避免影响基础运行依赖处理。仅改前端 `ModsPage.tsx` 和文档，未改后端 API、启用切换接口、玩家同步包导出或 Junimo 通信。验证：`cd frontend; npm.cmd run build`。

# JUNIMO-MOD-MOUNT-RESTORE-1 状态
- `JUNIMO-MOD-MOUNT-RESTORE-1` completed：启动/重启前自动从当前 `sdvd/server` 镜像同步官方 `JunimoServer` Mod 到宿主 `.local-container/mods`，修复 mods 挂载遮住镜像内置组件后 Junimo API/VNC rendering 不可用的问题。`JunimoHost.Server` 现在是内置强制启用组件，物理 `smapi` 运行时目录不再显示为重复 Mod。验证：`go test ./internal/games/stardew_junimo -run "ListMods|ApplyNewSaveDefault|ApplyModProfileKeeps|Rendering|JunimoServerMod"`、`go test ./internal/web -run "Rendering|VNCConfig"`。

# ENV-BOM-NORMALIZE-1 状态
- `ENV-BOM-NORMALIZE-1` completed：实例 `.env` 读取/写回会剥离 UTF-8 BOM 前缀 key，修复 `﻿IMAGE_VERSION` 导致 `docker compose up` 报 `unexpected character "\ufeff"` 的启动失败。当前 `data/instances/stardew/.env` 已热修，`docker compose config --quiet` 和 `docker compose up -d` 已验证通过；后续排查启动失败时应先区分 Compose 配置解析错误和容器进程错误。

# STEAMCMD-SELFUPDATE-CACHE-1 状态
- `STEAMCMD-SELFUPDATE-CACHE-1` completed：SteamCMD 兜底容器已持久化 `/root/.local/share/Steam` 与 `/home/steam/.local/share/Steam`，用于缓存 SteamCMD 客户端自更新文件；前端将登录前 `[steamcmd] [ N%] Downloading update (... of 40,273 KB)` 标记为 SteamCMD 客户端更新，不再误显示为镜像拉取或游戏文件下载。验证：`cd backend; go test ./internal/games/stardew_junimo -run "SteamCMD|InstallFallsBack|InstallResumes|InstallUsesExistingLater|InstallSteamCMD"`，`cd frontend; npm.cmd run build`。

# STEAMCMD-RETRY-RESUME-1 状态
- `STEAMCMD-RETRY-RESUME-1` completed：SteamCMD 兜底授权超时/失败后，复用凭据重试会直接恢复 SteamCMD fallback，不再先跑 Junimo compose pull 或 `steam-auth`；SteamCMD 镜像候选会先全量 inspect，本地已有任意候选即直接使用，避免重复拉取已下载镜像。前端安装页已补充“重试 SteamCMD 授权/下载”和“不重新拉取已有 SteamCMD 镜像”的提示。验证：`cd backend; go test ./internal/games/stardew_junimo -run "SteamCMD|InstallFallsBack|InstallResumes|InstallUsesExistingLater|InstallSteamCMD"`，`cd frontend; npm.cmd run build`。

# STEAMCMD-FALLBACK-1 状态

- `STEAMCMD-FALLBACK-1` completed：安装流程在 `steam-auth` 已认证成功但游戏文件下载失败时，会自动切换到 SteamCMD 兜底下载，复用已保存 Steam 账号密码并持久化 SteamCMD 授权缓存；需要验证时前端展示 SteamCMD 专属“手机 App 批准 / App 或邮箱验证码”选项。新增 `steamcmd_*` 安装 phase、Docker 通用 TTY 容器执行和 SteamCMD 镜像拉取能力；SteamCMD 镜像按 `STEAMCMD_IMAGE_CANDIDATES` 候选列表兜底，默认顺序为 `docker.1ms.run/steamcmd/steamcmd:latest`、`docker.m.daocloud.io/steamcmd/steamcmd:latest`、`ghcr.io/steamcmd/steamcmd:latest`、`cm2network/steamcmd:latest`，旧实例会补齐新候选并过滤直连 Docker Hub 的 `steamcmd/steamcmd:latest` 和已移除的 `docker.xuanyuan.me/steamcmd/steamcmd:latest`，单次 pull 默认等待 30 分钟，避免单个镜像源 403 或慢速拉取后直接失败。已验证 `cd backend; go test ./internal/docker ./internal/web ./internal/games/stardew_junimo` 与 `cd frontend; npm.cmd run build` 通过。

# FE-STEAM-AUTH-DOWNLOAD-PROGRESS-RESTORE-1 状态
- `FE-STEAM-AUTH-DOWNLOAD-PROGRESS-RESTORE-1` completed：安装页 Steam 认证区现在按最新日志识别下载阶段，日志出现 `Downloading app 413150` / `Progress:` 后会显示游戏下载卡和真实文件/体积进度条，不再停留在手机 App 批准登录；历史 QR URL 也不再压过后续 Guard 验证码输入。仅改 `frontend/src/games/stardew/pages/InstallPage.tsx`，未改后端接口、SSE 或 Junimo 通信。已验证 `cd frontend; npm.cmd run build` 通过；Browser QA 壳加载正常、无 overlay、console error/warn 为空。

# INSTALL-INTERRUPTED-STATE-1 状态
- `INSTALL-INTERRUPTED-STATE-1` completed：修复安装任务失败/面板重启后实例仍残留 `steam_auth_running` 导致前端卡在 48% 的问题。后端恢复 interrupted `stardew_install` 时会同步写 `install_interrupted`，steam-auth 容器运行错误会写 `steam_auth_failed`；前端安装页以活跃 job 为准，没有活跃安装任务时把旧运行中 phase 视为中断失败，并加载最近安装任务日志。已验证 `cd backend; go test ./internal/jobs ./internal/games/stardew_junimo` 与 `cd frontend; npm.cmd run build` 通过。

# DEPLOY-RUN-SH-1 状态

- `DEPLOY-RUN-SH-1` completed：新增 `deploy/run.sh` 作为用户优先使用的一键启动/维护脚本，产品口径收敛为快速模式：默认通过 `http://服务器IP:8090` 直接访问面板。脚本默认使用国内阿里云 ACR 面板镜像、默认 `latest` tag，并支持 `PANEL_VERSION=0.1.0` 固定版本；首次运行会生成 `~/.anxi-panel/.env`、`docker-compose.yml` 和 `~/.anxi-panel/data`，自动创建强随机 `PANEL_SECRET`。新版脚本使用宿主机数据目录与容器内同名绝对路径持久化面板数据；菜单覆盖 Docker 安装修复、镜像候选兜底、启动、停止、重启、更新、状态、日志、镜像源切换和显示访问地址；同时支持 `install/docker/stop/restart/update/status/logs/url` 等非交互命令。已同步 `docs/09-image-build.md`。

# FE-OPSRAIL-MAINTENANCE-PHASE-1 状态

- `FE-OPSRAIL-MAINTENANCE-PHASE-1` completed：右栏“进行中”卡的计划维护展示已从纯 `nextShutdownAt/nextStartupAt` 倒计时改为当前维护窗口阶段状态。到关机点后显示 `关机中 / 等待关机结束`，关机完成后保留自动开机倒计时，到开机点后显示 `开机中 / 等待开机结束`，开机 job 成功后才切回下一天倒计时；计划维护对应的生命周期 job 不再重复作为普通任务显示。仅改前端 `StardewPanel.tsx`，未改后端接口、调度器、权限或 Junimo 通信。已验证 `cd frontend; npm.cmd run build` 通过；Browser QA 全壳右栏渲染无 overlay、console error/warn 为空。

# FE-OVERVIEW-METRIC-TYPE-UNIFY-1 状态

- `FE-OVERVIEW-METRIC-TYPE-UNIFY-1` completed：总览页四张统计卡（存档 / 模组 / 系统健康 / 运行任务）已按用户截图反馈统一字体节奏。`.sd-ov-metric-strip` 下标题、数字、单位、说明和状态徽章现在使用同一 Verdana / Microsoft YaHei / SimHei 字体链，标题为 `14px/800`，数字为 `34px/800`，并减轻数字阴影，避免截图中的字体割裂和过粗跳字。仅改 `StardewPanel.css`，未改 TSX、API、权限、轮询或 Junimo 通信。已验证 `cd frontend; npm.cmd run build` 通过；内置 Browser QA 覆盖 1536x1024、点击服务器再回总览、390x844，console error/warn 为空，overlay 为 0，无横向溢出。

# FE-RESTART-SCHEDULE-PUT-WRITE-MODEL-1 状态

- `FE-RESTART-SCHEDULE-PUT-WRITE-MODEL-1` completed：服务器控制页“计划重启”保存请求体已收口为独立写入模型，避免把 `GET /restart-schedule` 返回的 `instanceId/nextShutdownAt/lastStatus` 等只读展示字段原样回传，触发后端 `DisallowUnknownFields()` 后显示 `request body must be valid JSON`。仅改前端 `types.ts` 与 `api.ts`，未改接口路径、后端调度器、权限或 Junimo 通信。已验证 `cd frontend; npm.cmd run build` 通过。

# FE-STOPPED-STATUS-RED-1 状态

- `FE-STOPPED-STATUS-RED-1` completed：总览页“服务器控制”状态行、服务器控制页顶部摘要状态和生命周期状态行的 `已停止` 已按用户截图反馈改为红色字样，停止态状态点同步为红点。仅改前端展示类名与 CSS，未改生命周期 API、按钮 handler、权限、轮询或 Junimo 通信。已验证 `cd frontend; npm.cmd run build` 通过；内置 Browser QA 覆盖 stopped 总览、服务器页和 390x844 窄屏，目标文字 computed color 均为 `rgb(192, 32, 32)`，console error/warn 为空，无页面级横向溢出。

# PLAYERS-SAVE-ROSTER-1 状态

- `PLAYERS-SAVE-ROSTER-1` completed：玩家名册已恢复从当前 Stardew 存档主 XML 合并 `<player>` 与 `<farmhands><Farmer>`，解决 `players.json` 在线快照只有 host、缓存也只有 host 时，存档里的 farmhand `test` 不显示的问题。存档离线玩家返回 `status=offline/source=save_file`，在线快照仍覆盖同一玩家；前端“等待加入”只统计 `waiting/pending/joining`，不再把离线名册行算进去。已验证 `cd backend; go test ./internal/games/stardew_junimo` 与 `cd frontend; npm.cmd run build` 通过。

# FE-JOBS-LOG-SCROLL-LOCK-1 状态

- `FE-JOBS-LOG-SCROLL-LOCK-1` completed：修复点击“任务与日志”后 Stardew Shell 被浏览器外层页面向下滚走、顶栏消失、底部露黑的问题。根因是缩放 Shell 的未缩放布局盒子让 `body/#root` 产生页面级纵向滚动，任务日志 `scrollIntoView()` 又触发了外层滚动。已在 `App.css` 对 `.sd-shell` 运行态锁定 `body/#root` 为 `100dvh + overflow:hidden`，并把任务日志、安装日志自动滚到底改为滚动各自日志窗口自身。未改 API、SSE、权限、轮询、路由或 Junimo 通信。已验证 `cd frontend; npm.cmd run build` 通过；内置 Browser QA 1112x920 下修复前 `window.scrollY=351/.sd-shell.top=-351`，修复后点击任务日志并强制 `window.scrollTo(0,600)` 仍保持 `scrollY=0/.sd-shell.top=0`，console error 为空。

# FE-MOBILE-NAV-BAR-SIZE-1 状态

- `FE-MOBILE-NAV-BAR-SIZE-1` completed：单栏状态下最上方横向选择栏已按截图反馈适量放大。`max-width: 640px` 下导航行从 `40px` 提到 `48px`，按钮从 `36x30` 提到 `42x38`，图标从 `20px` 提到 `23px`。仅改前端 CSS 和接手文档，未改路由、页面逻辑、API、权限或 Junimo 通信。已验证 `cd frontend; npm.cmd run build` 通过，内置 Browser QA 覆盖 490x844 和 390x844，点击“服务器”导航后 active route 正常切换，console error/warn 为空，页面级横向溢出为 0。

# FE-OVERVIEW-LIFECYCLE-LEFT-1 状态
- `FE-OVERVIEW-LIFECYCLE-LEFT-1` completed：总览页“服务器控制”区启动/停止/重启按钮已按用户截图反馈从生命周期区域中间移回左侧，与标题和状态行对齐。根因是旧生命周期 flex 规则残留 `flex-wrap: wrap` + `align-content: center`，本次在总览最终覆盖段改为 `flex-wrap: nowrap`、`align-content: flex-start`，并让按钮行 `align-self: flex-start`。仅改前端 CSS，未改生命周期 API、按钮 handler、邀请码刷新、权限、轮询或 Junimo 通信。已验证 `cd frontend; npm.cmd run build` 通过；内置 Browser QA 覆盖默认视口、点击启动交互和 390x844 窄屏，console error/warn 为空且无页面级横向溢出。

# FE-PLAYERS-ACTION-ICONS-IMAGE2-1 状态
- `FE-PLAYERS-ACTION-ICONS-IMAGE2-1` completed：玩家页活动列表文字挤压已修正，分页由上一轮的每页 3 条改为每页 2 条并提高事件行高度；管理操作四个 CSS 临时图标已替换为 imagegen 生成、透明抠底后的 image2/Stardew 像素风 PNG（踢出、封禁、白名单、权限）。未改后端 API、玩家事件接口、权限或 Junimo 通信。已验证 `cd frontend; npm.cmd run build` 通过；内置 Browser QA 覆盖 1536x1024 与 390x844，console error/warn 为空、无页面级横向溢出。

# FE-SAVES-BACKUP-POLICY-LAYOUT-1 状态
- `FE-SAVES-BACKUP-POLICY-LAYOUT-1` completed：存档页“自动备份策略”卡片已按用户反馈修正文字错乱。定时备份项调整为“勾选框 / 定时备份 / 每天 / 时间选择框”同一行顺序，每日快照保留行与滑杆重新分配宽度，左侧策略卡不再被右侧列表拉伸。仅改前端 `SavesSection.tsx` 与 `StardewPanel.css`，未改备份策略保存、恢复/删除、权限、后端 API 或 Junimo 通信。已验证清理过期 tsbuildinfo 后 `cd frontend; npm.cmd run build` 通过；QA mock 全壳截图确认策略卡无文字错乱且 console error/warn 为空。

# FE-TOPBAR-SAVE-STATUS-TYPE-1 状态
- `FE-TOPBAR-SAVE-STATUS-TYPE-1` completed：Stardew Shell 顶栏已按用户截图反馈微调。标题字体从过粗的海报感改为更轻、更小的像素描边风格；运行中/已停止状态改用现有像素状态牌素材；存档框移除下拉箭头，农场图标左移贴边，文本改为“农场名：简略游戏时间”（如 `AnxiFarm：第一年春`）；用户角色框也移除下拉箭头。仅改前端 `StardewPanel.tsx` / `StardewPanel.css`，未改 API、权限、路由或 Junimo 通信。已验证 `cd frontend; npm.cmd run build` 通过；内置 Browser QA 覆盖 1536x720 running/stopped 和 390x844，无横向溢出、无 overlay、console error/warn 为空。

# FE-MODS-PROTOTYPE-V02-LAYOUT-1 状态
- `FE-MODS-PROTOTYPE-V02-LAYOUT-1` completed：模组管理页已按 version-02 原型 `06-mods.png` 回正卡片顺序和比例。首屏固定为标题操作区、三段标签页、Nexus 连接横条、搜索卡、2x2 搜索结果卡和分页；下载结果卡恢复原型两按钮结构，移除额外的会员安装按钮；按用户截图反馈移除底部扩展安装进度条和“全部类别”下拉框；搜索提示改为“输入英文模组名称、ID 或关键词...”，热门标签改为 `UI Info`、`Fishing Mod`、`Backpack Upgrades`、`Tractor` 并保持真实快捷搜索。模组页卡片复用其它页面统一羊皮纸卡片变量；前置状态统一放在统计行“认可”后面，无前置也显示“前置：无”，保证每张卡操作按钮垂直位置一致。仅改前端 `ModsPage.tsx` 与 `StardewPanel.css`，未改后端 API、上传/删除/导出、启用切换、玩家同步包或 Junimo 通信。已验证 `cd frontend; npm.cmd run build` 通过；内置 Browser QA 覆盖 1536x1024 和 390x844，无页面级横向溢出，热门标签点击后搜索框为 `Tractor`。

# FE-DIAGNOSTICS-GAUGE-INNER-SAFE-1 状态
- `FE-DIAGNOSTICS-GAUGE-INNER-SAFE-1` completed：诊断页资源趋势圆环卡已按用户反馈修正数字安全区，保持三张卡片与原型比例，扩大中心底色圆并降低数字/百分号字号，避免红色弧线遮挡 `27.1%` 等百分比读数。仅改前端 CSS，未改资源指标接口、健康检查、轮询、导出诊断包或 Junimo 通信。已验证 `cd frontend; npm.cmd run build` 通过；内置 Browser QA 点击“诊断”后确认三张资源卡数字不再被弧线遮挡，console error/warn 为空。

# FE-PLAYERS-TIME-EVENTS-PAGING-1 状态
- `FE-PLAYERS-TIME-EVENTS-PAGING-1` completed：玩家管理页在线时长列已压缩为“今天/昨天/N天前 HH:mm”短格式并保留旧数据回退，避免遮挡收入列；在线玩家表收入列顺序调整为“玩家收入 / 农场收入”；玩家活动改为每页 3 条分页，桌面下与右侧管理操作卡同高，移动端自然堆叠且无页面级横向溢出。未改后端 API、玩家轮询、权限或 Junimo 通信。已验证 `cd frontend; npm.cmd run build` 通过；内置 Browser QA 覆盖 1536x1024 与 390x844，console error/warn 为空。

# FE-SETTINGS-API-PORT-REMOVE-1 状态
- `FE-SETTINGS-API-PORT-REMOVE-1` completed：设置与审计页“端口信息”卡片已移除只读“API 端口”字段，仅保留“面板端口 / VNC 端口 + 保存/刷新”。只改前端展示 JSX 和设置页端口行 CSS，未改 VNC 端口保存接口、权限判断、后端 API、Junimo 通信或轮询逻辑。已验证 `cd frontend; npm.cmd run build` 通过。

# FE-SAVES-UPLOAD-BLUE-BG-1 状态
- `FE-SAVES-UPLOAD-BLUE-BG-1` completed：存档页上传横条背景已按用户反馈从羊皮纸虚线样式恢复为之前的蓝色天空版本（蓝色渐变 + 白色像素云 + 木色实线边框）。仅改 `StardewPanel.css` 视觉样式，上传弹窗、ZIP 预览、导入并启动、权限和禁用逻辑不变。已验证 `cd frontend; npm.cmd run build` 通过；QA mock 全壳截图确认上传条为蓝色背景且 console error/warn 为空。

# FE-SAVES-V02-PROTOTYPE-LAYOUT-1 状态
- `FE-SAVES-V02-PROTOTYPE-LAYOUT-1` completed：存档管理页已按 version-02 原型 `03-saves.png` 回正卡片位置和比例。激活存档区改为“信息卡 + 右操作卡”，存档库工具按钮上移到标题右侧，桌面主宽下三张存档卡同排，上传条与底部“自动备份策略 / 备份列表”双栏按原型落位。仅改前端 TSX/CSS 展示结构，创建/上传/选择/删除/导出/备份/恢复接口和权限逻辑不变。已验证 `cd frontend; npm.cmd run build` 通过；QA mock 全壳 1536x1024 对照原型、390x844 无横向溢出且 console error/warn 为空。

# FE-SETTINGS-PROTOTYPE-V02-LAYOUT-2 状态
- `FE-SETTINGS-PROTOTYPE-V02-LAYOUT-2` completed：设置与审计页已按 version-02 原型 `09-settings.png` 回正卡片结构和比例。左列为面板版本、用户管理、端口信息、其他设置，右列为安全与权限、审计日志、安全建议；版本卡补右侧图像槽，安全摘要改单列表，端口卡当时改三端口横排，后续 `FE-SETTINGS-API-PORT-REMOVE-1` 已移除重复的“API 端口”；审计首屏 7 条，安全建议改三条状态徽章。仅改前端页面与 CSS，未改 API、权限、轮询或后端逻辑。已验证 `cd frontend; npm.cmd run build` 通过；内置浏览器临时 mock QA 覆盖 1536x1024 与 390x844，console error/warn 为空、无页面级横向溢出，QA 临时文件已删除。

# FE-SERVER-PROTOTYPE-V02-LAYOUT-2 状态
- `FE-SERVER-PROTOTYPE-V02-LAYOUT-2` completed：服务器控制页已按 version-02 原型 `02-server.png` 回正卡片结构和比例。顶部摘要改为服务器专用整行大卡（状态/在线玩家/当前存档/主机农民/游戏日期 + 邀请码横条），中部恢复生命周期左列、快捷操作右列，快捷操作改为原型式浅色工具行，底部控制台命令横跨整行且终端满宽；移动端恢复单列顺序并无页面级横向溢出。未改 API、权限、轮询、Junimo 通信或后端逻辑。已验证 `cd frontend; npm.cmd run build` 通过；内置浏览器临时 mock QA 覆盖 1536x1024 与 390x760，console error/warn 为空，QA 临时文件已删除。

# FE-OVERVIEW-BANNER-SCENE-IMAGE2-1 状态
- `FE-OVERVIEW-BANNER-SCENE-IMAGE2-1` completed：总览页顶部农场横幅场景已替换为从 image2 原型 `01-overview.png` 裁出的运行时素材 `overview_banner_scene_image2.png`，不再由 CSS 田野/云层和旧 `sprite_farmhouse_scene.png` 叠加生成。裁切只包含农场场景，不含统计条或页面外壳；仅改前端 CSS 与静态素材，未动组件逻辑/API/后端。已验证新 PNG 预览正常，`cd frontend; npm.cmd run build` 通过。

# FE-PROTOTYPE-SHELL-ALIGN-1 状态
# FE-DIAGNOSTICS-PROTOTYPE-V02-LAYOUT-1 状态
- `FE-DIAGNOSTICS-PROTOTYPE-V02-LAYOUT-1` completed：诊断页已按 version-02 current-frontend-code 原型 `07-diagnostics.png` 回正卡片位置与比例。页面专属收紧主 frame inset，顶部状态横卡约 `975x160`，中部检查项/资源趋势回到等宽双列约 `482x392`，底部告警建议横跨全宽并留在首屏内；资源仪表卡改为标题在上、圆环居中、说明在下，趋势图压回原型短图。未改后端接口、权限、轮询或业务状态。已验证 `cd frontend; npm.cmd run build` 通过；Playwright + 本机 Chrome 回退 QA：1536x1024 与 390x844 无横向溢出、console error/warn 为空。

- `FE-PROTOTYPE-SHELL-ALIGN-1` completed：九页前端布局已对齐 image2 version-02 原型。根因是右信息栏(414px)/左导航(252px)过肥把主内容挤到 791px；已把 `--sd-opsrail-width` 收到 `clamp(268px,19vw,300px)`、`--sd-sidebar-width` 收到 `clamp(196px,14vw,216px)`，主内容区回到 937px，总览恢复 4 卡一行、控制区与邀请码并排。逐页修：服务器(生命周期|快捷操作并排、快捷操作改竖排)、任务日志(列表|详情两列)、玩家(表整行+活动|管理两列，逆转 FE-PLAYERS-LIST-LEFT-1)、诊断(检查信息单行不折)、设置(用户/审计表列不裁切)、存档(上传区改羊皮纸虚线)、模组(结果卡两列网格)、顶栏版本不折行。仅改 `StardewPanel.css`，未动逻辑/接口。已验证 `cd frontend; npm run build` 通过；mock-fetch harness + Playwright 1536 逐页对比原型、pageerror 为 0，真实登录态截图待补。

# FE-PLAYERS-LIST-LEFT-1 状态
- `FE-PLAYERS-LIST-LEFT-1` completed：玩家管理页桌面首屏已调整为左侧宽列显示在线玩家表、右侧窄列显示最近事件，取消旧规则导致玩家表落到右侧第三行的问题，减少中间空白；服务器信息（Junimo）保留为底部整行调试信息。已验证 `cd frontend; npm.cmd run build` 通过。

# FE-DIAG-GAUGE-TOMIK-1 状态
- `FE-DIAG-GAUGE-TOMIK-1` completed：诊断页 CPU/内存/磁盘三资源圆环从铜钱 conic 样式改为 Tomik23 circular-progress-bar 风格（灰底环 + yellow→#ff0000 渐变圆头描边 + 中心百分比），纯 SVG 实现无新依赖；空态只画底环。已验证 `cd frontend; npm.cmd run build` 通过，Playwright 真实登录态 1366/390 截图正常、pageerror 为 0。

# FE-UNIFIED-CARD-PARCHMENT-TONE-1 状态
- `FE-UNIFIED-CARD-PARCHMENT-TONE-1` completed：总览统计卡当前的浅羊皮纸暖黄已提升为统一小卡片背景色，文件尾部覆盖 `--sd-save-card-bg` / `--sd-save-card-bg-strong`，所有复用统一小卡片变量的非模组页小框都会跟随；总览 `.sd-mc` 保持同色且无斜纹。未改卡片尺寸、边框、圆角、阴影、文字布局或业务逻辑。已验证 `cd frontend; npm.cmd run build` 通过。

# FE-INSTALL-STEAM-AUTH-ICON-1 状态
- `FE-INSTALL-STEAM-AUTH-ICON-1` completed：安装页“Steam 认证”卡片中间占位图标和栏目标题小图标均已改为复用安装进度第三步的 `icon_install_step_steam_image2.png`，不再使用 CSS 渐变画出的蓝色 Steam 圆球。未改 Steam 认证、Steam Guard、扫码、日志或后端接口。已验证 `cd frontend; npm.cmd run build` 通过；真实安装页受登录页限制，登录态截图待补。

# FE-SETTINGS-FILL-GAP-1 状态
- `FE-SETTINGS-FILL-GAP-1` completed：设置页已从三段式布局改为左右两列堆叠，左列为“面板版本 / 用户管理 / 端口信息 / 其他设置”，右列为“安全与权限 / 审计日志 / 安全建议”，让端口信息和其他设置上移补足短列空位；`780px` 以下再切回单列。已验证 `cd frontend; npm.cmd run build` 通过。

# FE-OVERVIEW-METRIC-CLEAN-BG-1 状态
- `FE-OVERVIEW-METRIC-CLEAN-BG-1` completed：总览页四个统计卡 `.sd-mc` 已移除斜向纸纹背景，并改为干净、偏浅的羊皮纸暖黄渐变；后续按反馈从偏白略微压黄，但不恢复旧的高饱和黄色。保留卡片尺寸、边框、角饰、文字布局和状态徽章。已验证 `cd frontend; npm.cmd run build` 通过。

# FE-INSTALL-HERO-SCENE-REMOVE-1 状态
- `FE-INSTALL-HERO-SCENE-REMOVE-1` completed：安装页顶部状态横幅已移除右侧大农舍场景图 `.sd-install-farm-scene`，不再渲染安装页顶部的 `sprite_farmhouse_scene.png`；状态横幅改为“小土芽图标 + 状态信息”两列。未改安装状态、Steam 认证、日志、进度或后端接口。已验证 `cd frontend; npm.cmd run build` 通过。

# FE-INVITE-CARD-COPY-ORDER-1 状态
- `FE-INVITE-CARD-COPY-ORDER-1` completed：邀请码卡片新增共享 `InviteCodeCard`，服务器摘要卡和总览页服务器控制区统一复用；复制按钮调整到刷新按钮左侧，仅在已有邀请码时渲染，未有码状态不保留空按钮列。总览页已移除旧 `sd-invite-panel` JSX、本地复制状态和独立 `handleCopy()`，后续邀请码展示/复制/刷新只维护一套组件。已验证 `cd frontend; npm.cmd run build` 通过；内置浏览器在未登录状态下确认服务器页与总览页应用壳非空、console error/warn 为空，登录态卡片截图待补。

# FE-SETTINGS-ACCOUNT-CARD-REMOVE-1 状态
- `FE-SETTINGS-ACCOUNT-CARD-REMOVE-1` completed：设置与审计页已移除顶部“当前账号”卡片，避免和顶栏用户入口重复；顶部摘要区从三卡改为“面板版本 / 安全与权限”两卡，并清理 `sd-settings-account-*` 死样式。登出仍保留在 Stardew Shell 顶栏，未改 session、权限、用户管理或审计日志逻辑。已验证 `cd frontend; npm.cmd run build` 通过。

# FE-PAGE-HEADER-SHADOW-1 状态
- `FE-PAGE-HEADER-SHADOW-1` completed：Stardew 各路由页头已移除标题文字、导航图标和右侧虚线分隔的阴影背景，只保留干净标题、图标和分隔线；未改页面结构、按钮、卡片布局、API 或业务逻辑。已验证 `cd frontend; npm.cmd run build` 通过。

# FE-PAGE-TOP-ALIGN-1 状态
- `FE-PAGE-TOP-ALIGN-1` completed：Stardew 各 routed 页面已通过文件尾部 CSS 兜底统一贴齐主内容 frame 顶部，覆盖任务、诊断、安装、设置等页面后置 `padding` 造成的顶部下沉；未改页面结构、卡片布局、API 或业务逻辑。已验证 `cd frontend; npm.cmd run build` 通过。

# FE-SERVER-ACTION-CARDS-1 状态
- `FE-SERVER-ACTION-CARDS-1` completed：服务器控制页生命周期控制卡已下移到顶部摘要卡下方左侧，快捷操作卡放到同一行右侧；快捷操作按钮统一叠加 `.sd-btn--lg`，与生命周期按钮共用 40px lg 尺寸令牌，并移除原 64px 卡片式按钮和伪图标。已验证 `cd frontend; npm.cmd run build` 通过；真实路由受登录页限制，仅完成应用非空、无框架覆盖、console error/warn 为空的烟测，未完成登录态截图验证。

# FE-SERVER-INVITE-IN-SUMMARY-1 状态
- `FE-SERVER-INVITE-IN-SUMMARY-1` completed：服务器控制页移除中部独立“邀请代码”卡片，邀请码复制/刷新入口收敛到顶部服务器摘要卡；“刷新”按钮位于“邀请加入码”显示区右侧，运行中/启动中可用，未运行时禁用。删除 `ServerControlPage` 第二套复制状态和 `handleCopy()`，后续统一复用 `ServerSummaryCard`；“全服消息”在删除邀请码卡片后改为横跨整行。已验证 `cd frontend; npm.cmd run build` 通过；真实路由受登录页限制，内置浏览器确认应用加载非空且 console error/warn 为空，未完成登录态截图验证。

# NEXUS-ERROR-TEXT-1 状态
- `NEXUS-ERROR-TEXT-1` completed：Nexus 搜索/安装错误响应中的中文 mojibake 已修复，`nexus_request_failed` 等错误不再显示为 `璇锋眰澶辫触`。前端 `errorCodeMap` 同步增加 Nexus 错误码中文兜底，后续即使后端 message 异常也会优先显示正常中文。已验证 `cd backend; go test ./internal/web -run TestWriteNexusErrorMessagesAreReadableChinese` 和 `cd frontend; npm.cmd run build` 通过。

# FE-BTN-UNIFY-1 状态
- `FE-BTN-UNIFY-1` completed：9 个页面按钮与操作区统一化。按钮尺寸收敛为 lg 40px / md 28px / sm 22px 三档令牌（`.sd-btn--lg` / 默认 / `.sd-btn--sm`），语义色收敛为绿(主)/棕(次)/红(危险)，删除 `.sd-btn-blue`、`.sd-btn-xs` 与死样式 `sd-btn-gold`/`sd-btn-red`；危险确认弹窗统一红色确认键与"取消+确认"顺序；新增共享 `.sd-actionbar` / `.sd-rowactions` 操作区布局类；删除全部逐页按钮尺寸覆写与 JSX 内联尺寸；统一"刷新/保存/X并启动"等文案；修复总览页无邀请码时邀请面板被 1px 网格列挤成竖排的问题。已验证 `cd frontend; npm.cmd run build` 通过，并用 Playwright 真实登录态对 9 页 × 4 视口做改前/改后截图对比，console 无新增报错。

# FE-MAIN-PAGE-FRAME-SLICES-1 状态
- `FE-MAIN-PAGE-FRAME-SLICES-1` completed：所有 Stardew 页面共用的 `.sd-main` 主内容 frame 已从整图 `100% 100%` 拉伸改为 image2 空框 9 切片平铺。新增四角、上下左右边 tile 和中心羊皮纸 tile，运行时用 9 层 CSS background 实现四角固定、上下 `repeat-x`、左右 `repeat-y`、中心 `repeat`，窗口缩放时边框纹理不再被拉伸；`.sd-main-scroll` 统一滚动视口保持不变。已验证 `cd frontend; npm.cmd run build` 通过，并用内置浏览器临时 QA 页检查 1280x720 与 390x760 下背景层/重复规则正确、无横向溢出、滚动正常、console error/warn 为空。

# FE-MODS-DEPENDENCY-POPOVER-1 状态
- `FE-MODS-DEPENDENCY-POPOVER-1` completed：下载模组页 Nexus 搜索结果的前置信息弹层已改为受控按钮 + 弹层，修复固定高度搜索卡片裁切导致前置信息看起来无法展开的问题；点击信息弹层外部会自动收起，不再需要回点前置按钮。已验证 `cd frontend; npm.cmd run build` 通过，并用内置浏览器临时 QA 页检查 1280x720 与 390x760 展开/外部点击收起、无水平溢出、console error/warn 为空。

# JOB-DISPLAY-NAME-1 状态
- `JOB-DISPLAY-NAME-1` completed：jobs 表新增 `display_name`，任务 API/SSE 返回 `displayName`；Nexus/远程 Mod 安装任务会写入 `mod_nexus_install · <Mod 名>` / `mod_remote_install · <Mod 名>`，前端任务页、右栏进行中、右栏近期任务和总览近期事件均优先展示该字段，解决并行依赖安装时多个任务同名不可区分的问题。已验证后端 storage/jobs/web 测试和前端构建通过。

# MODUPLOAD-DUPLICATE-CODE-1 状态
- `MODUPLOAD-DUPLICATE-CODE-1` completed：Mod ZIP 上传遇到已安装相同 `UniqueID` 时，后端响应错误码改为 `mod_exists`，避免前端显示成“Mod ZIP 无效/解析失败”；真正坏 ZIP、无 manifest、XNB 替换包等仍保持 `invalid_mod_zip`。已验证 `cd backend; go test ./internal/web -run "TestModUpload"` 通过。

# FE-OPSRAIL-DOWNLOAD-PROGRESS-1 状态
- `FE-OPSRAIL-DOWNLOAD-PROGRESS-1` completed：右侧 OpsRail“进行中”卡已接入远程 Mod 安装下载日志进度，`mod_remote_install` / `mod_nexus_install` 会解析 `下载进度：已下载 ...（xx.x%）` 并映射到右栏进度条；扩展 batch 一旦返回新的 `jobId` 会立即刷新 jobs，让任务尽快出现在右栏，Premium 安装路径也同步主动刷新。已验证 `cd frontend; npm.cmd run build` 通过。

# NEXUS-EXT-DOWNLOAD-GUARD-1 状态
- `NEXUS-EXT-DOWNLOAD-GUARD-1` completed：远程 Mod 安装任务新增下载可见进度和错误分界日志，显示连接远程下载服务器、HTTP 响应、Content-Type、压缩包大小、已下载/总量/剩余/百分比；无总量时显示已下载大小。浏览器扩展提交前强制校验 Nexus CDN `.zip`，后台页未真正拿到 ZIP 时不再创建面板安装任务；后端收到 `text/html` 会立即失败并提示这是网页不是 ZIP。已验证后端相关测试与扩展 JS 语法检查通过。

# FE-SERVER-PROTOTYPE-IMAGE2-1 状态
- `FE-SERVER-PROTOTYPE-IMAGE2-1` completed：服务器控制页已按 image2 原型重皮肤为羊皮纸控制台结构，包含当前状态大卡、生命周期控制卡、邀请代码、全服消息、黑色命令终端和快捷操作条；原型图未作为运行时资源，纸纹、铜边、内阴影、绿屏、终端和分隔线均由 CSS 实现。业务逻辑/API/权限/disabled 状态保持不变，按钮和标题图标复用既有 Stardew PNG/图标素材。已验证 `cd frontend; npm.cmd run build` 通过，并用临时 QA 页检查 1280x900 与 390x760 无横向溢出、按钮文字不溢出、命令执行输出可读。

# FE-DIAGNOSTICS-IMAGE2-ICONS-1 状态
- `FE-DIAGNOSTICS-IMAGE2-ICONS-1` completed：诊断页首轮 CSS 自绘图标已替换为 image2 风格透明 PNG 素材。新增 `frontend/public/assets/stardew/ui/diagnostics/`，包含 4x5 生成 sprite sheet 与状态盾牌、三色宝石、检查项、建议区、资源趋势、实时绿点、导出下载等 20 个单图；运行时使用单图背景，不使用整页原型图或整块截图。已验证 `cd frontend; npm.cmd run build` 通过，内置浏览器临时 QA 页检查 1280x900 与 390x760 无横向溢出、按钮不溢出、console error/warn 为空，可见诊断页图标背景均来自 `/assets/stardew/ui/diagnostics/`。

# FE-DIAGNOSTICS-PROTOTYPE-IMAGE2-1 状态
- `FE-DIAGNOSTICS-PROTOTYPE-IMAGE2-1` completed：诊断与健康页已按 image2 原型重皮肤为顶部双操作、系统状态横卡、三计数卡、检查项表、资源趋势卡和底部告警建议区；原型图未作为运行时背景或整块资源引用，业务逻辑/API/权限保持不变。已验证 `cd frontend; npm.cmd run build` 通过，并用内置浏览器临时 QA 页检查 1280x900 与 390x760 无横向溢出、无面板重叠、按钮文字不溢出、console error/warn 为空。注：首轮 CSS 自绘图标已在 `FE-DIAGNOSTICS-IMAGE2-ICONS-1` 中替换为 image2 PNG 素材。

# FE-SETTINGS-PROTOTYPE-IMAGE2-1 状态
- `FE-SETTINGS-PROTOTYPE-IMAGE2-1` completed：设置与审计页已按 image2 `09-settings - 副本.png` 原型重皮肤为顶部三卡、中部用户/审计双栏、底部设置/安全建议双栏结构。原型图未作为运行时背景或整块素材引用；纸纹、铜边、角钉、表头、分隔线和提示面板均为 CSS 实现，按钮/标题图标复用现有 Stardew PNG 素材。功能逻辑、API 调用、权限判断、loading/error/empty/disabled 状态保留。已验证 `cd frontend; npm.cmd run build` 通过；内置浏览器真实路由停在登录页，因此使用已删除的临时 `settings-qa.html` 加载同 CSS/素材/同结构 DOM 验证 1280x900 桌面、点击新建用户展开态、390x760 窄屏和底部状态均无页面级横向溢出，console error/warn 为空。

# FE-INSTALL-PROTOTYPE-IMAGE2-1 状态
- `FE-INSTALL-PROTOTYPE-IMAGE2-1` completed：安装页已按 image2 `08-install - 副本.png` 原型重皮肤为顶部状态横幅、五步安装时间线、安装配置/Steam 认证/安装日志三栏工作区。原型图未作为运行时资源；纸张纹理、边框、分隔线、步骤卡、进度条和日志终端均为 CSS 实现，按钮继续复用既有 PNG 按钮体系。已验证 `cd frontend; npm.cmd run build` 通过；内置浏览器临时 QA 覆盖 1280x900 认证态、未安装态点击展开表单、390x760 窄屏无横向溢出，console error/warn 为空。

# FE-OPSRAIL-AUTO-COLLAPSE-1 状态
- `FE-OPSRAIL-AUTO-COLLAPSE-1` completed：Stardew Shell 已按主内容预计宽度自动收起右侧 OpsRail，首次实现使用 `820/880px` 历史阈值；当前已由顶部 `FE-RESPONSIVE-VIEWPORT-1` 按数值缩放后的实际主内容宽改为 `400/460px`，维护时以 `responsive-layout.ts` 为准。总览页 1180px 响应式规则补为 `.sd-main-scroll` 容器查询，避免窗口缩放时中间内容被右栏挤压后仍按桌面布局排布。首次实现已验证 `cd frontend; npm.cmd run build` 通过；内置浏览器 QA 覆盖 1200x760 自动收起、1200→1600 无刷新自动展开、390x760 移动布局，均无横向溢出且 console error/warn 为空。

# FE-OVERVIEW-PROTOTYPE-IMAGE2-1 状态
- `FE-OVERVIEW-PROTOTYPE-IMAGE2-1` completed：总览页已按 image2 原型重皮肤为农场横幅 + 羊皮纸控制台 + 四摘要卡 + 三列清单结构；页面背景、纸纹、卡片、分隔线、绿屏邀请码和状态标签均由 CSS 实现，未把原型图作为运行时资源。按钮和小图标复用现有 Stardew PNG/图标素材，业务逻辑/API/权限不变。已验证 `cd frontend; npm.cmd run build` 通过，并用内置浏览器临时 QA 页检查 1280x900 与 390x760 无横向溢出、按钮文字不溢出、console error/warn 为空。

# FE-SAVES-SIMPLIFY-3 状态
- `FE-SAVES-SIMPLIFY-3` completed：存档页页头精简为左上角纯文字标题（去框去描述）；全部按钮换回面板既有 PNG 素材按钮（`sd-btn-green/tan/delete`），撤销自绘 `.sd-pxbtn`；备份与恢复规则压缩为单行控件条，说明文字移入悬浮提示，文案精简。已验证前端构建通过。

# FE-SAVES-MOCKUP-2 状态
- `FE-SAVES-MOCKUP-2` completed：存档页按用户完整设计稿改版：眉标行、激活卡图标字段双列表、存档库只显示非激活存档（选择/删除糖块按钮）、横排虚线创建卡、上传横条（📮 + 上传并启动）、备份六列表格（状态徽章 + 5 行折叠）；新增纯 CSS `.sd-pxbtn` 糖块按钮体系；导出/手动备份/启动收敛到激活卡。已验证前端构建通过。

# FE-SAVES-PROTO-CSS-1 状态
- `FE-SAVES-PROTO-CSS-1` completed：存档页按 image2 原型重构为纯 CSS 皮肤（无图片资源）：铜框铆钉激活卡、双列虚线字段、圆角存档卡、虚线创建卡、像素云天空上传横条、表头带备份表格；创建/上传入口从页头移入创建卡与上传横条，业务逻辑未变。已验证前端构建通过。
- `FE-SAVES-PROTO-CSS-1` follow-up：激活存档卡预览槽接入真实农场地图，按存档 `farmType` 复用新建游戏的 `/assets/stardew/new-game/farms/<farmType>.png` 素材，未知类型回落为空羊皮纸块。已验证前端构建通过。

# FE-RIGHT-RAIL-ACTIVE-CARD-1 状态
- `FE-RIGHT-RAIL-ACTIVE-CARD-1` completed：右栏"进行中"卡接入自动关机/自动开机倒计时（restart-schedule）、定时备份倒计时（backups/policy，仅管理员行）和运行中任务进度条（同类型历史耗时中位数估算，封顶 95%）；倒计时行为蓝色档、每秒走动，任务完成后行自动消失；dashboard 30s 轮询同时刷新 jobs 兜底调度器触发的任务。已验证前端构建通过。

# FE-RIGHT-RAIL-HEALTH-STATS-1 状态
- `FE-RIGHT-RAIL-HEALTH-STATS-1` completed：右栏"系统健康"卡按原型接入真实数据：CPU/内存/磁盘使用率（复用 `GET /api/instances/:id/metrics`，5s 轮询，圆润胶囊形进度条）、在线玩家（`onlineCount/当前存档 MaxPlayers`）、网络延迟（metrics 请求前端往返耗时）；按钮改为"查看详情 →"。已验证前端构建通过。
- `FE-RIGHT-RAIL-HEALTH-STATS-1` follow-up：五行增加阈值配色，浆果点/进度条/数值随数值绿→黄→红（使用率 60/85、延迟 100/300ms 分档），在线玩家为 0 时显示红色。已验证 `npm run build` 通过。

# PLAYERS-MAXPLAYERS-1 状态
- `PLAYERS-MAXPLAYERS-1` completed：`stardew_junimo` driver 的 `ListPlayers` 现在用当前存档 `server-settings.json` 的 `Server.MaxPlayers` 兜底 `maxPlayers` 字段（junimo info 解析出的值仍优先），玩家接口在服务器未运行或 info 不含上限时也能返回人数上限。已验证 `go test ./internal/games/stardew_junimo/` 通过。

# FE-RIGHT-RAIL-CARD-FIX-1 状态
- `FE-RIGHT-RAIL-CARD-FIX-1` completed：右栏三卡（系统健康/进行中/近期任务）去掉内部滚动与等高拉伸逻辑，行高改为随内容、与左侧栏按钮一致只随栏宽缩放；四角藤蔓拉伸修复为每卡单一缩放系数换算 border-width/margin，角部切片横纵等比。已验证 `npm run build` 通过。详见 `docs/03-frontend.md` 与 `docs/frontend-handoff/frontend-handoff-2026-07-02.md` 对应条目。

# FE-MAIN-PAGE-FRAME-3 状态
- `FE-MAIN-PAGE-FRAME-3` completed：按用户红框示意，把所有 Stardew 页面共用的中间内容滚动视口重定界为 frame 内侧大矩形。`.sd-main` 四向 inset 改为 top `5.2%`、right `5%`、bottom `6%`、left `4%`（带移动下限和桌面上限），`.sd-main-scroll` 继续在该红框边界内滚动，所有路由统一生效。已验证 `cd frontend; npm.cmd run build` 通过；1750x1113 QA 下主内容区 `1068x1033` 时 inset 为 `55.5/53.4/64.1/42.7px`，390x760 下无横向溢出且滚动正常。

# FE-MAIN-PAGE-FRAME-2 状态
- `FE-MAIN-PAGE-FRAME-2` completed：修复中间内容区 frame 裁切方案导致模组页无法滚动的回归。`.sd-main` 继续以上一步界定的内侧 frame 边界作为裁切框，负责背景、内框 padding 和 `overflow:hidden`；新增 `.sd-main-scroll` 作为统一滚动视口，负责滚轮/触控板滚动和隐藏原生滚动条；各路由页面继续返回普通 `.sd-page`，避免复杂页面布局被滚动容器规则影响。已验证 `cd frontend; npm.cmd run build` 通过，内置浏览器 QA 在 1280x720 和 390x760 下滚动正常、无横向溢出、console error/warn 为空。

# FE-TOPBAR-IMAGE2-REGEN-1 状态
- `FE-TOPBAR-IMAGE2-REGEN-1` completed：按用户要求用 image2 参考 `01-overview.png` / `Top bar.png` 重新生成顶栏拆分素材，替换上一批不合格 topbar 资源。外壳为 `topbar_shell_left.png` + `topbar_shell_middle_tile.png` repeat-x + `topbar_shell_right.png`，控件框为独立 `*_9slice.png`，鸡/农场/头像/叶子/绿点/登出/下拉箭头为独立 v2 图标；文字继续由 React 渲染。右端缺失问题已通过重新归一化 `topbar_shell_right.png` 到完整高度修复。已验证前端构建通过、1920x900 顶栏无横向溢出、390x760 移动端顶栏策略正常。

# FE-RIGHT-RAIL-TOP-FROM-BOTTOM-1 状态
- `FE-RIGHT-RAIL-TOP-FROM-BOTTOM-1` completed：右侧栏上段素材已改为“底段去南瓜/向日葵后旋转 180 度”的版本。处理过程保留原底段文件不变，仅覆盖运行时使用的 `right_rail_shell_top_line_image2.png`，并按新图实测横梁范围同步更新 `.sd-opsrail::before` 的定位/尺寸常量和 stack 顶部留白。已验证前端构建通过，PNG 为 `1871x840` RGBA，alpha 范围 `0..255`，人工预览确认上段不再含南瓜/向日葵且横梁无破洞。

# FE-ASSET-SIDEBAR-3PIECE-1 状态
- `FE-ASSET-SIDEBAR-3PIECE-1` completed：已从 image2 左侧栏生成三段式可复用背景素材 `panel_side_rail_top_image2.png`、`panel_side_rail_middle_tile_image2.png`、`panel_side_rail_bottom_image2.png`。顶部段保留木质顶部外框和横梁，中段按方案 A 调整为可 `repeat-y` 的纯连续木板/立柱 tile，不含横向分隔线、按钮槽位、暗条、分层隔板或固定行高结构；底部段保留书架、灯笼、盆栽、紫水晶和书本/盒子装饰。三张均为 RGBA 透明 PNG，不含导航按钮、中文文字、菜单图标或按钮高亮。本次仅入库生产素材，尚未改运行时代码。

# FE-SIDEBAR-ROW-BG-1 状态
- `FE-SIDEBAR-ROW-BG-1` completed：左侧栏运行时已接入三段式背景素材，替换整张空壳背景 `100% 100%` 拉伸。导航 DOM 新增 `.sd-nav-list` / `.sd-nav-row`，每个按钮背后的木板段改由行容器渲染并随按钮布局走，解决背景固定槽位在放大/缩小时与按钮错位的问题。已验证 `cd frontend; npm.cmd run build` 通过；浏览器登录态侧栏截图验证受当前本地账号不可用阻塞。
- `FE-SIDEBAR-ROW-BG-1` follow-up：为避免 `.sd-nav-list` 滚动条压缩行容器宽度导致中段素材右边框左移，完整 `panel_side_rail_middle_tile_image2.png` 改为只在 `.sd-sidebar` 外层绘制，`.sd-nav-row` 只保留上下阴影槽位感。已验证 `cd frontend; npm.cmd run build` 通过。
- `FE-SIDEBAR-ROW-BG-1` follow-up：导航按钮宽度改用 `min(86cqi, 210px)`，以侧栏容器宽度为基准，不再因 `.sd-nav-list` 滚动条或行容器宽度变化而缩小。已验证 `cd frontend; npm.cmd run build` 通过。
- `FE-SIDEBAR-ROW-BG-1` follow-up：桌面 `.sd-nav-list` 保留可滚动但隐藏滚动条，避免滚动条预留宽度把导航行居中区域压窄、导致按钮整体左移。已验证 `cd frontend; npm.cmd run build` 通过。

# SERVER-SAY-1 状态
- `SERVER-SAY-1` completed：服务器控制页喊话入口已打通。后端 `POST /commands/say` 校验消息后写入 `.local-container/control/commands/broadcast*.json`，由 `StardewAnxiPanel.Control` 在游戏 tick 中调用 Stardew multiplayer chat message 向所有在线玩家发送 `[Panel]` 公告；不依赖不存在的上游 `say` SMAPI 命令。已验证当前 Junimo 镜像包含上游 `/ws chat_send` relay，但面板暂用控制模组文件通道以保持容器网络和部署兼容。

# FE-ASSET-RIGHT-RAIL-SHELL-3PIECE-1 状态
- `FE-ASSET-RIGHT-RAIL-SHELL-3PIECE-1` completed：已用 image2 重新生成右侧栏三段空壳与三张卡片九宫格素材：`right_rail_shell_top.png`、`right_rail_shell_middle_tile.png`、`right_rail_shell_bottom.png`、`right_card_health_9slice.png`、`right_card_progress_9slice.png`、`right_card_recent_9slice.png`。三段 shell 分别只保留顶部横梁/上边框/藤蔓角饰、左右木柱 + 纯木板 repeat-y 中段、底部木梁 + 南瓜 + 向日葵 + 藤蔓；三张卡片框不含标题、图标、CPU/内存/磁盘文字、进度条、任务列表或按钮文字，内容继续由前端渲染。已用 RGBA/alpha/洋红残留检查和棋盘底人工预览验证。

# FE-RIGHT-RAIL-SPLIT-ASSETS-1 状态
- `FE-RIGHT-RAIL-SPLIT-ASSETS-1` completed：Stardew Shell 右侧 OpsRail 已从整张 `panel_right_rail_image2.png` 背景方案迁移为拆分素材组合。运行时使用右栏空壳、外层边框、三张九宫格卡片框和三枚标题图标分层渲染；中文标题、健康摘要、任务列表、按钮文字和 Mod 重启提示继续由 React 数据层渲染，`.sd-dot*` 状态点复用原 CSS 动态效果。已验证 `cd frontend; npm.cmd run build`、1280×720 总览页三卡可见、右栏“查看诊断/查看全部任务”跳转成功、390×760 移动端右栏隐藏且无水平溢出，console 无 error/warn。

# FE-SAVE-START-NAV-1 状态
- `FE-SAVE-START-NAV-1` completed：从存档页发起的选择并启动、创建并启动、上传并启动成功创建任务后会跳转总览页，并触发新邀请码等待态；不再默认跳到任务页。

# FE-QUICK-BACKUP-1 状态
- `FE-QUICK-BACKUP-1` completed：服务器控制页“快捷操作”里的“备份已保存进度”已接入 `POST /api/instances/:id/saves/:name/backup`，按当前激活存档创建手动备份；“保存世界 / 立即保存”占位已移除，避免误导用户以为面板能强制保存尚未写盘的游戏进度。

# SCHEDULED-RESTART-DESIGN-1 状态
- `SCHEDULED-RESTART-1` completed：计划重启第一阶段已接入。管理员可在服务器控制页配置每日关闭/开启维护窗口，后端持久化到 `restart_schedules`，后台调度器提前广播、关闭前备份已落盘存档，并复用现有 Stop/Start 生命周期 job；暂不实现强制保存世界。

# FE-LIFECYCLE-WAIT-1 状态
- `FE-LIFECYCLE-WAIT-1` completed：总览页和服务器控制页的启动/重启按钮现在会在请求发出后显示旋转圆圈与“启动中…”，并持续等待新邀请码出现；停止按钮也会在停止过程中显示旋转圆圈与“停止中…”，直到状态完全停止后才恢复启动按钮。

# PLAYERSYNC-PACK-10 状态
- `PLAYERSYNC-PACK-10` completed：玩家同步安装脚本彻底禁用终端进度渲染，移除 `Write-Progress` 并设置 `$ProgressPreference = "SilentlyContinue"`；进度仅写入日志，控制台只显示独立任务行和最终摘要。当前测试解压包已热修并在真实游戏目录验证不再出现中文双字重叠。
# PLAYERSYNC-PACK-9 状态
- `PLAYERSYNC-PACK-9` completed：玩家同步安装完成摘要移除单独的 `SMAPI 路径`，只保留可直接复制到 Steam 的 `Steam 启动项文本`。当前测试解压包已热修并真实验证。
# PLAYERSYNC-PACK-8 状态
- `PLAYERSYNC-PACK-8` completed：玩家同步安装脚本移除自绘 carriage-return 动态进度行，避免中文终端输出重复字和残留；控制台只输出独立任务日志，进度只写入日志文件。安装摘要单独列出完整 `Steam 启动项文本`，当前测试解压包已热修并真实验证。
# PLAYERSYNC-PACK-7 状态
- `PLAYERSYNC-PACK-7` completed：玩家同步安装脚本新增 Mod 目录内容指纹比对。目标 Mod 与 payload 完全一致时跳过备份和复制，并在 `installed.json.mods[]` 写入 `skippedIdentical=true`；任意文件内容、大小或路径不同都会触发备份旧目录并覆盖安装。全部跳过且未真实备份时 `backupId=null`。当前测试解压包已热修并验证。
# PLAYERSYNC-PACK-6 状态
- `PLAYERSYNC-PACK-6` completed：玩家同步安装脚本的文本进度条改为单行动态刷新。`Show-InstallProgress` 使用 `[Console]::Write([char]13...)` 原地更新当前进度，普通安装事件打印前会清除进度行、打印后重绘，避免 checksum 和 Mod 复制阶段刷出大量重复进度行。当前测试解压包已热修。
# PLAYERSYNC-PACK-5 状态
- `PLAYERSYNC-PACK-5` completed：玩家同步包安装 SMAPI 时改为调用随包官方 Windows 安装器 `internal/windows/SMAPI.Installer.exe`，传入 `--install --game-path "<Stardew Valley>" --no-prompt`；脚本不再直接解包 `install.dat`，也不做本机 `.NET` / `runtimeconfig` 特调。安装器超过 120 秒未退出时会提示玩家检查安装器窗口是否在等待按键/输入。
# PLAYERSYNC-PACK-4 状态

- `PLAYERSYNC-PACK-4` completed：玩家同步安装脚本新增进度条，使用 PowerShell `Write-Progress` + 文本进度行显示安装阶段；checksum 按文件数推进，Mod 安装按待复制 Mod 数推进，SMAPI 阶段显示解压安装包、释放官方安装文件和完成。

# PLAYERSYNC-PACK-3 状态

- `PLAYERSYNC-PACK-3` completed：修复玩家安装脚本对 `[CP]` 等带方括号 Mod 路径的误判。checksum 校验、payload source 检查、目标 Mod 检查、卸载恢复检查都改用 PowerShell `-LiteralPath`，避免把 `[CP] MultipleConstructionOrders` 当成通配符字符集。

# PLAYERSYNC-PACK-2 状态

- `PLAYERSYNC-PACK-2` completed：玩家同步 ZIP 已升级为可执行安装包结构，包含 `安装玩家同步包.bat`、`卸载本同步包.bat`、`tools/`、`payload/mods/`、`payload/smapi/`、`pack-manifest.json` 和 `checksums.sha256`。玩家端脚本会校验 payload、定位 Stardew Valley、备份同名 Mod、复制本包 Mod、尽力设置 Steam 启动项，并把安装状态写入游戏目录 `.anxi-sync/`。SMAPI 采用服务端随包优先策略：导出时若实例目录下已有 `SMAPI*.zip` 则随包携带并校验，否则继续导出 Mod 包并提示玩家自行安装 SMAPI。

# NEXUS-SMAPI-THUMB-1 状态
- `NEXUS-SMAPI-THUMB-1` completed：虚拟内置 SMAPI 条目现在带 Nexus:2400 元数据，`GET /mods` 会通过现有 Nexus GraphQL 补全链路为它缓存并返回缩略图；同包内容包也会从同 Nexus ID 的完整缓存继承 `pictureUrl`，避免半截来源缓存挡住图片。前端继续统一使用 `pictureUrl`，失败时显示来源文字占位。

# MODORIGIN-1 状态
- `MODORIGIN-1` completed：后端区分 Mod 自己的 `nexusModId` 与同包来源 `originNexusModId`，手动上传 Nexus 多 Mod ZIP 时会把 `[CP]` 内容包标记为“来源 N站包，随主 Mod 安装”，并继承来源包缩略图；前端已按 `来源：N站 + Nexus:<id>` / `来源：N站包 + 随 <名称> 安装` 展示。同包 Mod 在已安装列表相邻显示，删除任意成员时弹窗提示并由后端一次性捆绑删除整组。

# MODSYNC-AUTO-1 状态
- `MODSYNC-AUTO-1` completed：玩家同步分类增加自动识别默认值：面板控制组件为服务器专用，SMAPI 内容包和第三方 Mod 默认玩家需同步，并在 `syncNote` 提供自动识别说明；分类下拉改为任意登录用户可修改，仍只写面板自有 `mod-sync.json`。

# NEXUS-INSTALLED-1 状态
- `NEXUS-INSTALLED-1` completed：添加模组页的已安装网格只展示 Nexus 视角数据，隐藏纯本地文件项和服务端控制组件；SMAPI 虚拟项按 Nexus:2400 展示，跳转按钮指向 Nexus 页面，无图时使用来源文字占位而不是文件夹图标。

# MODZIP-3 状态
- `MODZIP-3` completed：Mod manifest 解析兼容 UTF-8 BOM，修复部分 Nexus 原包中 `manifest.json` 以 BOM 开头导致上传显示 `Mod ZIP 无效` 的问题；不放宽非法 JSON 校验。

# MODZIP-4 状态
- `MODZIP-4` completed：Mod manifest 解析兼容 JSONC 风格注释和尾随逗号，修复 SpaceCore 等 Nexus 原包通过远程 CDN 安装时因 `manifest.json` 含 `//` 注释而失败的问题；字符串里的 URL 不会被误处理，ZIP 安全校验和 SMAPI 必填字段规则不变。

# MODDEPS-1 状态
- `MODDEPS-1` completed：后端已解析 SMAPI `Dependencies` 和 `ContentPackFor` 并通过 `GET /mods` 返回 `dependencies[]`；前端已在已安装 Mod 卡片底部给普通用户展示短名“前置：...”标签，完整依赖信息放在悬浮提示中。当前不自动安装依赖，也不判断缺失/版本不满足；完整依赖检查仍列为后续能力。

# MODUPLOAD-2 状态
- `MODUPLOAD-2` completed：Mod 上传弹窗和后端接口已支持一次选择并上传多个 `.zip`。后端按批次逐个导入，任意 ZIP 失败会回滚本次请求已导入的前序 Mod，成功时返回所有导入 Mod；`restartRequired` 继续遵循停服上传不额外提示重启的语义。前端 `ModsPage` / 旧 `ModsSection` 均已改为多文件选择，`uploadMod` 仍保留为兼容封装。

# NEXUS-META-1 状态
- `NEXUS-META-1` completed：后端已通过 Nexus GraphQL v2 无 Key 按 `gameId=1303 + modId` 拉取展示元数据；数字 ID 搜索、手动上传后缩略图补全都不再依赖 Nexus API Key。`GET /mods` 会对带 `UpdateKeys: ["Nexus:<id>"]` 的本地 Mod 自动补齐 Nexus 卡片字段并写入 sidecar 缓存。后续优化可考虑后台异步刷新、缓存过期时间，以及 CurseForge/ModDrop/GitHub 等多来源元数据补全。

# MODRESTART-1 状态
- `MODRESTART-1` completed：停服状态下上传/删除/安装 Mod 不再设置“需要重启”标记；停止服务器和停服 Mod 写入会清理历史标记，前端上传成功文案改为下次启动自动加载。

# MODZIP-1 状态
- `MODZIP-1` completed：Mod ZIP 上传增加 XNB 替换包识别和友好错误提示。当前仍不安装 XNB 内容替换包，只提示用户该类 Nexus 下载不是 SMAPI Mod，不能放入服务器 `Mods` 目录。

# MODZIP-2 状态
- `MODZIP-2` completed：Mod ZIP 上传支持 Nexus 常见单层外壳目录包，能自动剥离外壳并导入内部带 `manifest.json` 的真实 SMAPI Mod 子目录；上传、Nexus 一键安装和远程 ZIP 安装都复用该能力，不再要求用户手动解压重打包。
- 后续仍可优化：缺失依赖检测；更复杂安装说明型 ZIP 的识别和人工确认。

# MODSEARCH-1 / NEXUS-PAGED-1 状态
- `MODSEARCH-1` reverted：统一搜索/统一安装骨架已撤回，后端 `/mods/search` 与 `/mods/search/install` 不再注册，`mod_search.go` 和对应测试已删除。
- `NEXUS-PAGED-1` completed：当前 Mods 下载页只保留 Nexus 专用搜索/安装路径，支持默认热门、GraphQL 下载量排序、分页、数字 ID 查询和 Nexus 一键安装。
- 后续仍可优化：如重新做多来源搜索，需要重新设计接口；候选包括 StardewModDataset 本地/缓存索引、CurseForge Core API、GitHub Release asset、ModDrop 稳定下载来源、provider 去重排序和文件选择 UI；7z/rar 安全解压仍未开放。

# REMOTE-MOD-1 状态
- `REMOTE-MOD-1` completed：管理员可在 Mods 下载页粘贴 `nxm://` 或 `.zip` 直链创建 `mod_remote_install` job；NXM 链路支持非 Premium Nexus 用户通过 `key/expires` 获取 API 下载链接；直链/CDN 链路直接下载 ZIP 并复用现有安全导入，来源文案已覆盖 ModDrop/GitHub/CurseForge。
- 后续仍可优化：接入 CurseForge API Key 与 `download-url` 真一键；GitHub Release asset 安装；StardewModDataset 本地索引用于依赖与搜索；ModDrop 稳定下载来源识别；7z/rar 解压安全支持；多文件 Nexus/CurseForge/ModDrop 文件选择 UI。

# NEXUS-EXT-1 状态
- `NEXUS-EXT-1` prototype：新增 `browser-extensions/nexus-slow-installer` 私用浏览器扩展实验包。扩展可在 Nexus 文件页捕获慢速下载生成的临时 CDN ZIP 链接，并提交给现有 `POST /api/instances/:id/mods/remote/install` 远程安装接口；当前未集成进 ModsPage，也未新增扩展专用 token 后端接口。
- 后续仍可优化：在 ModsPage 增加“扩展安装”入口并带上下文打开 Nexus 文件页；新增扩展配对 token 和更窄 host 权限；扩展安装任务状态回传面板 UI；多文件选择和依赖链自动引导。

# NEXUS-3 状态

- `NEXUS-3` completed：Nexus Mods 无 Key GraphQL 搜索、v1 REST/下载链路 Key-gated、一键安装、`mod_nexus_install` job 进度、下载安装后复用现有 ZIP 安全导入、已安装 Mod Nexus 元数据 sidecar、前端搜索/已安装同款卡片展示已完成。
- 后续仍可优化：真实 Nexus 权限差异（手动下载限制、会员下载、OAuth）下的错误提示细分；多文件 Mod 的文件选择 UI；依赖/更新检查。

# FE-SIDEBAR-SPLIT-ASSETS-1 状态
- `FE-SIDEBAR-SPLIT-ASSETS-1` completed：Stardew Shell 左侧栏已从整张带文字按钮的 `panel_side_rail_image2.png` 透明热区方案迁移为拆分素材组合。桌面端使用左栏空壳作为唯一背景并填满侧栏格子，default / hover / active 三张按钮底图分别承担未选中、悬停、当前页状态，9 个独立导航图标和 React 中文 label；保留现有 9 路由、点击跳转、active、高亮、focus-visible 和移动端横向图标导航。侧栏四周用 CSS 像素边框补强；底部独立装饰层暂不接入运行时，避免与空壳底部残留装饰重复。已验证 `cd frontend; npm.cmd run build`、桌面 `overview -> server -> diagnostics` 点击跳转、390×760 移动视口无水平溢出，console 无 error/warn。

# 未来路线

## 当前路线判断

当前产品继续保持：

```text
Single Game Mode now
Multi Game Mode later
```

也就是说，用户体验上只看到 Stardew 面板；代码内部保留 `instances + driver_id + GameDriver`，后续新增第二个游戏时再显示总面板。

## 已完成里程碑摘要

| 阶段 | 状态 | 摘要 |
| --- | --- | --- |
| M0 Repo Skeleton | completed | 仓库、目录、基础文档 |
| M1 Backend Foundation | completed | Go 后端、配置、健康检查 |
| M2 Storage/Auth | completed | SQLite、用户、session、登录 |
| M3 Docker/Compose | completed | Docker 封装和调试接口 |
| M4 Jobs/State | completed | 长任务、日志、SSE、实例状态 |
| M5 GameDriver Registry | completed | driver 注册和实例模型 |
| M6 Junimo Prepare/Install | completed | Junimo compose、Steam Auth、安装 |
| M7 Lifecycle | completed | 启动、停止、重启、邀请码 |
| M7.5 New Game | completed | 自定义新建存档和素材 catalog |
| M8 Frontend MVP | completed | 登录、安装、主面板基础 |
| M9 Saves | completed | 存档管理、上传、删除、备份 |
| M10 Mods | completed | Mod 上传、删除、导出 |
| M11 Console | completed | allowlist 命令、Control Mod 文件通道喊话 |
| M12 Packaging | completed | Dockerfile、静态前端嵌入、部署 |
| M13 Hardening | completed | 审计、脱敏、权限、诊断、支持包 |
| M14 Release Candidate | completed | 发布检查、版本信息、支持包 |
| FE-R1 至 FE-R13 | completed | Stardew 像素风 Shell 与 9 路由 |
| UI-R7 至 UI-R12 | completed | 登录页和 UI 位图高级重绘 |
| PLAYERS-4 至 PLAYERS-6 | completed | 玩家精确位置与中文映射 |
| PLAYERS-7 | completed | 玩家页拆分农场收入与个人收入字段 |
| PLAYERS-8 | completed | 玩家活动最近事件，基于快照差分记录首次记录、加入和离开 |
| STATE-INVITE-1 至 4 | completed | 状态校准、重启后新邀请码等待与 server-only restart |
| AUTOPAUSE-1 至 7 | completed | 真人玩家菜单暂停、多人全员菜单共识暂停与 gameTimeInterval 哨兵时钟冻结 |
| DOCS-1 | completed | 文档归并为九份长期维护文档 |
| LIFECYCLE-JOBS-1 | completed | 停止/重启/再次启动会取消同实例旧生命周期任务，避免旧启动任务长期 running |
| FE-SHELL-SCROLL-1 | completed | Stardew Shell 固定视口高度，长页面仅中间内容区滚动，左右栏保持固定 |
| FE-LOGIN-IMAGE2-1 | completed | 登录/首次注册首页切换为 image2 原型图整页背景，前端覆盖绘制账号、密码区域和登录/注册按钮 |
| MODSYNC-1 | completed | Mod 玩家同步包第一阶段：`syncKind` 分类、面板自有 `mod-sync.json`、sync-plan/sync-classification/sync-pack 导出接口、ModsPage 玩家同步区域 |
| MODSYNC-AUTO-1 | completed | Mod 同步分类自动识别默认值，并允许任意登录用户手动修改服务器专用/玩家需同步/待确认标签 |
| PLAYERSYNC-PACK-2 | completed | 玩家同步包升级为 Windows 安装包结构，内置安装/卸载脚本、payload checksum、pack-manifest、Mod 备份恢复和 Steam 启动项尽力配置 |
| PLAYERSYNC-PACK-3 | completed | 玩家同步安装脚本改用 `-LiteralPath` 处理 Mod 路径，修复 `[CP]` 方括号目录导致 checksum 误报缺文件 |
| PLAYERSYNC-PACK-4 | completed | 玩家同步安装脚本新增 PowerShell 原生进度条和文本进度行，覆盖环境检查、checksum、SMAPI、Mod、Steam 和完成阶段 |
| NEXUS-2 | completed | Mod 管理第二阶段：Nexus Mods 只读搜索（`GET mods/nexus/search`，ID 精确查询走官方 v1 REST、关键词走 GraphQL v2）、`UpdateKeys`/`NexusModID` manifest 解析、已安装匹配、ModsPage 在线搜索区域；不做下载/安装 |
| FE-MODS-WORKBENCH-1 | completed | ModsPage 参考 EMP Mod 管理台改为“下载模组 / 添加模组 / 配置模组”三段式工作台，Nexus 搜索卡片化，已安装与玩家同步归入添加页，配置页预留 SMAPI 配置入口 |
| NEXUS-SETTINGS-1 | completed | Nexus API Key 改为管理员在前端配置并持久化到 SQLite `panel_settings`，后端搜索请求即时读取，不再使用环境变量 |
| MODRESTART-1 | completed | 停服状态下 Mod 写操作不再提示需要重启，旧重启标记会在停止/停服写入时清理 |
| MODZIP-1 | completed | Mod ZIP 上传识别 XNB 替换包并返回明确错误提示，不再误导为 ZIP 损坏 |
| MODZIP-2 | completed | Mod ZIP 上传支持 Nexus 单层外壳目录包，自动剥离外壳并导入内部真实 SMAPI Mod 子目录 |
| MODZIP-3 | completed | Mod manifest 解析兼容 UTF-8 BOM，避免 Nexus 原包因 BOM manifest 被误判为无效 Mod ZIP |
| MODZIP-4 | completed | Mod manifest 解析兼容 JSONC 注释和尾随逗号，避免 SpaceCore 等原包远程安装失败 |
| MODDEPS-1 | completed | Mod 列表解析并展示 SMAPI 前置依赖声明，普通用户可在已安装 Mod 卡片看到需要的前置依赖 |
| MODORIGIN-1 | completed | Nexus 多 Mod ZIP 的内容包记录来源包字段，已安装卡片区分主 N站 Mod 与随包内容包，并支持同包相邻展示与捆绑删除 |
| NEXUS-PAGED-1 / NEXUS-PAGER-2 | completed | 下载页回到 Nexus-only 搜索，支持默认热门、下载量排序、分页和 Nexus 一键安装；旧 `/mods/search` 统一搜索骨架已撤回 |
| MOBILE-SHELL-M0-1 | completed | 移动端基础入口 M0：`useMediaQuery` hook + `App.tsx` 按 `<=768px` 分流到占位组件 `StardewMobileShell`（顶部品牌/状态、羊皮纸占位卡、5 个静态 Tab），桌面端 `StardewPanel` 行为视觉不变；真实移动端页面内容和 Tab 路由映射留给 M1 |
| MOBILE-HOME-M2-1 | completed | 移动端总览页 M2：`StardewMobileShell` 的“总览”Tab 接入真实 `mobile/MobileHomePage.tsx`（状态摘要/邀请信息/快捷控制/待认证玩家批准四张卡片），全部复用现有 API 和 `useStardewDashboardData()`，未新增后端接口，桌面端总览页未改 |
| MOBILE-CONTROL-M3-1 | completed | 移动端控制页 M3：`StardewMobileShell` 的“控制”Tab 接入真实 `mobile/MobileControlPage.tsx`（全服消息卡 + 快捷操作卡：计划重启/密码设置/小屋高级设置/触发节日活动/永久启用Joja路线，去掉手动备份和VNC显示），全部复用现有 API，未新增后端接口，桌面端 `ServerControlPage.tsx` 未改（除删除过时的 say 命令提示文案） |
| MOBILE-PLAYERS-M4-1 | completed | 移动端玩家页 M4：`StardewMobileShell` 的“玩家”Tab 接入真实 `mobile/MobilePlayersPage.tsx`（单卡“在线玩家”：右上角刷新按钮 + 玩家卡片列表[名称/状态徽章/主机角色/最近活动/位置信息 + 踢出/封禁]），踢出/封禁复用 `kickPlayer`/`banPlayer`，未新增后端接口，桌面端 `PlayersPage.tsx` 未改；不含待授权玩家同意/拒绝（保留在 `MOBILE-HOME-M2-1` 的总览 Tab） |
| LOGIN-MOBILE-FIX-1 | completed | 修复登录/初始化页在手机端的布局崩坏：`App.css` 新增 `@media(max-width:768px)` 覆盖，`.sd-auth-shell--image-login` 放弃固定 16:9 比例的绝对坐标定位，改回真实文档流羊皮纸卡片，复用现有三张素材，未新增图片、未改 API/权限 |
| FE-LAZYLOAD-1 | completed | 前端拆分阶段一：桌面端 9 路由（`StardewPanel.tsx`）与移动端 5 页面（`StardewMobileShell.tsx`）改为 `React.lazy` + `Suspense` 按需加载，主 JS chunk 从约 579 KB 降到约 243 KB，构建 chunk 大小警告消失；hook 拆分与 CSS 按页面拆分列入阶段二/三，见 `docs/07-later-optimizations.md` |
| FE-LIFECYCLE-ACTIONS-1 | completed | 前端拆分阶段二第一项：新增 `useStardewLifecycleActions.ts`，把 `OverviewPage.tsx`/`ServerControlPage.tsx` 重复的启停 action、pending 状态、确认弹窗逻辑合并为一个 hook；`OverviewPage.tsx` 555→456 行，`ServerControlPage.tsx` 减少约 90 行重复逻辑；阶段二后续（ServerControlPage 其他领域 hook、SavesSection、ModsPage）见 `docs/07-later-optimizations.md` |
| FE-SERVER-DOMAIN-HOOKS-1 | completed | 前端拆分阶段二第二项：`ServerControlPage.tsx` 拆成 9 个独立领域 hook（备份/计划重启/VNC/密码/运行时设置/节日/Joja/控制台/喊话），页面从 1437 行降到 979 行；阶段二剩余项（SavesSection、ModsPage）见 `docs/07-later-optimizations.md` |
| FE-MODS-MANAGEMENT-HOOK-1 | completed | 前端拆分阶段二 ModsPage 项：新增 `useModsManagement.ts`，集中本服 Mod 列表、上传/删除/导出、玩家同步与当前存档启用切换；`ModsPage.tsx` 2536→2360 行，Nexus 扩展批量状态机保持原有时序 |
| FE-CSS-SPLIT-1 | completed | 前端拆分阶段三：`StardewPanel.css` 拆为共享 Shell CSS 与 9 个桌面页面 CSS，各懒加载页面自行 import；共享 CSS 约 16586→4551 行，页面样式进入独立按需 chunk |
| FE-SAVES-DOMAIN-HOOKS-1 | completed | 前端拆分阶段二 SavesSection 项：新增 `useSaveBackups.ts`（备份列表/策略/手动备份/删除备份）与 `useSaveRestore.ts`（回档确认弹窗），`SavesSection.tsx` 1236→1131 行；存档列表 CRUD、新建游戏、上传存档弹窗因不属于回档领域且低耦合，保留在页面内 |

## 近期优先级

0. 玩家缓存按 `saveId` 隔离已修复；真实新建/切换存档后确认上一存档玩家不再出现在当前玩家列表。
1. 真实运行环境验证邀请码重启刷新、SMAPI DLL 加载，以及玩家页 `farmIncome`/`personalIncome` 显示。
2. 验证玩家页在真实多人场景下的位置、在线状态、中文地图名和最近事件。
3. 继续排查联机角色槽异常，保持只诊断不破坏存档。
4. 做一次完整 release checklist 冒烟测试。
5. 持续清理 UI 中已无 JSX 引用的旧 CSS 规则和旧组件残留；本轮已删除无引用的旧 Stardew Section 组件与前端死 API 封装。
6. ~~用真实 Nexus API Key 验证 Nexus 关键词搜索的 GraphQL v2 返回结构~~ 已完成：通过对 `https://api.nexusmods.com/v2/graphql` 做 schema introspection 和真实搜索请求，确认并修复了 `nexus.go` 里 `mods` 查询的参数结构（游戏域名和关键词都要放进 `filter: ModsFilter` 而不是顶层 `gameDomain` 参数），关键词搜索本身不需要个人 API Key。
7. 为 ModsPage 的依赖缺失检查、更新检查和 SMAPI 配置编辑补齐后端能力；Nexus 安装与按存档启用/禁用已接入。

## 中期路线

- 玩家事件驱动 SSE。
- 完整服务器日志 tail。
- 更完善备份策略。
- 计划重启：管理员配置每日维护窗口（几点关闭、几点开启）、提前广播、关闭前备份，并复用现有停止/启动生命周期 job。
- Mod 依赖缺失/版本检查和更新提示。
- Nexus 多文件选择、权限差异提示和 OAuth/非 Premium 下载体验优化。
- 设置页中的审计过滤、会话管理、安全策略。
- 更完整的紧凑端导航和表格卡片化：总览、控制、玩家、模组、存档已是真实页面；“更多”已提供完整桌面版和退出登录。任务、诊断、安装与设置仍没有原生紧凑页面，当前通过完整桌面版进入；后续若补原生页面，继续复用既有 API/权限，不在前端另造 Stardew 逻辑。

## 长期路线

### Multi Game Mode

启用条件：

- 至少新增第二个可用游戏 driver。
- 前端具备 game module registry。
- 总面板能展示实例列表、状态摘要和入口。

建议未来游戏：

- Minecraft
- Don't Starve Together
- Terraria
- Palworld
- Valheim

### 插件化

长期可以把 driver、前端模块、Compose 模板和文档模板进一步插件化，但不要在 Stardew MVP 阶段提前做复杂市场系统。

## 不要过早做

- 不要一开始做多游戏市场。
- 不要把未来游戏页面硬塞进 Stardew 模块。
- 不要绕过 GameDriver 在 handler 里堆游戏分支。
- 不要允许前端任意 shell。
- 不要用截图/OCR/VNC 解析游戏状态。
- 不要做会破坏存档的自动修复工具。
# SMAPI-RUNTIME-1 状态
- `SMAPI-RUNTIME-1` completed：Mods 列表现在会在面板控制 Mod 已安装时置顶显示虚拟内置组件 `SMAPI`，提醒玩家客户端需要先安装 SMAPI；该条目带 `builtIn=true`，前端禁用删除和同步分类编辑，玩家同步统计/导出排除该虚拟运行组件。
# NEXUS-PAGED-1 状态

- `NEXUS-PAGED-1` completed：ModsPage 在线搜索回到 Nexus-only 路径，GraphQL 关键词搜索按下载量降序下推排序，并支持 `page/pageSize/total/hasMore` 分页。旧统一搜索前端入口已移除。
- `MODSEARCH-1` 统一搜索骨架已撤回：后端 `/mods/search` 与 `/mods/search/install` 路由、`mod_search.go` 和对应测试已移除；当前只保留 Nexus 搜索/安装源。
# NEXUS-PAGER-2 状态

- `NEXUS-PAGER-2` completed：Nexus 搜索结果顶部和底部都有完整分页控件，支持首页/末页/上一页/下一页/指定页跳转。

# SMAPI-SYNC-2 状态

- `SMAPI-SYNC-2` completed：SMAPI 现在作为内置但玩家必需的同步项，进入玩家同步统计与同步包 `pack-manifest.json`；`StardewAnxiPanel.Control` 标为内置服务端控制组件，前端不显示删除按钮，后端拒绝删除，且永远不打包进玩家同步 ZIP。

# PLAYERSYNC-PACK-11 状态
- `PLAYERSYNC-PACK-11` completed：玩家同步安装脚本恢复 ASCII-only 动态进度条，动态行只显示英文阶段和百分比，中文详细状态仍写日志并通过独立任务行输出；继续禁用 `Write-Progress`，当前测试解压包已热修并真实验证中文任务行不再重叠。
# PLAYERSYNC-PACK-12 状态
- `PLAYERSYNC-PACK-12` completed：玩家同步安装脚本将日志写入改为 `Write-LogLine` 短重试且非致命，修复 `install-*.log` 被短暂占用时中断 SMAPI 安装的问题。当前测试解压包已热修并真实安装验证通过。
# PLAYERSYNC-PACK-13 状态
- `PLAYERSYNC-PACK-13` completed：玩家同步安装包终端输出将启动项标题高亮为 Yellow、可复制启动项文本高亮为 Cyan，并保持启动项独立一行。当前测试解压包已热修并真实安装验证正常。
# PLAYERSYNC-PACK-14 状态
- `PLAYERSYNC-PACK-14` completed：玩家同步安装包在启动项标题后新增“请复制到 Steam 的游戏启动项中。”提示，可复制启动项文本仍独立一行。当前测试解压包已热修并真实安装验证正常。
# PLAYERSYNC-PACK-15 状态
- `PLAYERSYNC-PACK-15` completed：新增不带 SMAPI 的 `stardew-player-mods-update-pack.zip` 导出模式。完整版同步包继续用于首次玩家；模组更新包用于已运行过同步包的玩家，客户端检查已有 `StardewModdingAPI.exe` 后只安装/更新真实玩家 Mod，并沿用指纹跳过相同 Mod、不同内容备份覆盖的策略。

# PLAYERSYNC-PACK-16 状态
- `PLAYERSYNC-PACK-16` completed：模组更新包安装脚本不再尝试读取或写入 Steam 启动项，沿用完整版同步包已经设置好的 SMAPI 启动项；更新包摘要只显示已跳过 Steam 配置，不再输出复制启动项文本。完整同步包的 Steam 配置行为保持不变。

# MODPROFILE-1 状态

- `MODPROFILE-1` completed：完成按存档启用/禁用 Mod 第一阶段。新增 `mods-disabled` 目录、`mod-profiles.json`、`PUT /mods/:id/enabled`，配置页可在停服时按当前存档切换 Mod。新建/新导入存档默认禁用所有非内置 Mod。
# MODPROFILE-2 状态

- `MODPROFILE-2` completed：修复切换存档后前端仍显示旧存档 Mod 启用状态的问题；公共数据层会在 active save 变化时刷新 mods，并补充后端 profile 物理目录切换测试。

# NEXUS-DEFAULT-1 状态

- `NEXUS-DEFAULT-1` completed：下载模组页默认展示 Nexus Stardew Valley 热门列表前 20 条；空 `q` 搜索现在作为默认热门列表处理，仍支持分页和已安装匹配。
# NEWGAME-CABINS-1 状态
- `NEWGAME-CABINS-1` completed：自定义新存档的初始联机小屋数现在按真实小屋数显示和提交；后端 `startingCabins` 契约对齐 0-7，并同时写入 Junimo settings、SMAPI control init 与 `new-game-pending` 一次性标记；控制模组只在标记存在时于 Junimo 新建存档前同步 Stardew 原生小屋数/布局参数。后续如仍出现“存档里有 Cabin 但地图不可见”，需要针对 Junimo/存档 XML 的建筑坐标做专项验证。
- `CONTROL-NATIVE-CREATE-REMOVE-1` completed：Anxi Control 模组的历史原生创建存档路径已移除；自定义新存档只通过 Junimo `POST /newgame` 创建，Control 保留新建前参数同步、角色定制和运行期控制。

# FE-BACKUP-COPY-1 状态
- `FE-BACKUP-COPY-1` completed：备份设置区文案从 `latest`/`scheduled` 等内部术语改为用户语义说明；每个自动备份选项解释覆盖行为和保留规则，备份列表类型标签改为“手动备份 / 最新备份 / 每日快照 / 定时备份”。


# SAVE-BACKUP-POLICY-1 ??
- `SAVE-BACKUP-POLICY-1` completed?????????????????????????????? latest????????????????????? 3 ???? 14 ??? daily ???????SMAPI Control ????????????????????????????????????????????? scheduler ?????/?????

# SAVE-BACKUP-SCHEDULE-HOUR-1 状态
- `SAVE-BACKUP-SCHEDULE-HOUR-1` completed：定时备份已从“每隔 N 小时”改为“每天 HH:00 执行一次”，前端使用 00:00-23:00 下拉框，后端以 `scheduledHour` 存储和判断，并兼容读取旧 `scheduledIntervalHours`。
# MODDEPS-2 状态
- `MODDEPS-2` completed：Mod 依赖检测已从“只展示 manifest 声明”升级为后端状态判断。`GET /mods` 会标记依赖是否安装、当前存档是否启用、最低版本是否满足；Nexus 搜索会把当前存档禁用的已安装 Mod 标为 `installed=true, installedEnabled=false`，前端显示“已安装但未启用”。配置页依赖诊断已放在 Mod 名称区域下方，避免长英文名和状态列互相挤压。后续仍可优化：依赖自动安装入口、Nexus/SMAPI 更新提示、多来源依赖索引。

# MODREL-1 状态
- `MODREL-1` completed：Mod 同步分类与启用状态已按关系联动。同步分类按必需依赖连通组一起变，避免“待确认”后切回其它标签时后置 Mod 停留在旧状态；启用会补同包和前置，禁用会关同包和下游但保留 Content Patcher 等共享前置。两个 PUT 接口都会返回本次受影响的 `mods[]`，前端按返回结果批量更新。
# NEXUS-EXT-2 状态
- `NEXUS-EXT-2` completed：修复 Nexus/远程安装任务日志中的核心乱码文案；安装/上传成功后后端会把本次导入的 Mod 标记为当前激活存档启用，ModsPage 也会自动切到“添加模组”页并刷新已安装列表，避免扩展提交成功但用户看不到或用不上新 Mod。

# NEXUS-EXT-3 状态
- `NEXUS-EXT-3` completed：Nexus 搜索结果“一键安装”改为同页跳转到 Nexus 文件页并带 `anxi_auto=1`，由浏览器扩展自动获取临时 ZIP；扩展右下角只保留提交按钮，提交后创建 `mod_remote_install` 任务并跳回 `/instances/:id/jobs?jobId=...`，任务页会直接选中新任务。
# NEXUS-REQ-1 状态
- `NEXUS-REQ-1` completed：Nexus 搜索结果现在返回 `requiredMods[]`，前端搜索卡片会提示缺失/未启用的 Nexus 前置，并可对缺失前置走同一套扩展一键安装。浏览器扩展已支持 Nexus “Additional files required” 弹窗自动点击 `Download` 继续。
# NEXUS-PREMIUM-2 状态
- `NEXUS-PREMIUM-2` completed：Mods 下载页已移除“粘贴链接安装”人工入口和对应前端冗余 API/type。未配置 Nexus Key 时仅在配置按钮左侧提示 Premium 用户填写 NexusKey；配置后提示消失，并在每个 Nexus 搜索结果卡片底部显示 `N站会员专属安装`，走现有 Nexus API Key/Premium 直连安装。普通 `一键安装` 继续服务非 Premium 用户的浏览器扩展流程。
# NEXUS-CARD-UI-1 状态
- `NEXUS-CARD-UI-1` completed：Nexus 搜索结果卡片完成布局整理，主操作按钮固定在统一操作行，会员安装和前置依赖状态进入底部次操作区。前置依赖只显示 `缺少前置mod` / `前置已满足`，点击或悬停后展开具体 Mod 名、NexusId 和状态。

# NEXUS-EXT-BATCH-1 状态

- `NEXUS-EXT-BATCH-1` completed：普通 Nexus 一键安装已改为后台批量扩展流程。面板页保持不跳转，扩展后台同时打开当前 Mod 与未安装前置 Mod 的 Nexus 下载页，自动捕获并提交 ZIP 链接；搜索卡片主按钮显示扩展提交流程百分比，失败时显示 `失败请手动安装`。
- 补充：ModsPage 会把 Nexus 搜索条件、结果、分页和扩展批量安装状态保存到 `sessionStorage`。用户切到任务日志等页面再返回时，不会重新加载默认热门或清空搜索结果；若扩展批量安装仍在进行，会继续通过 `GET_BATCH_STATUS` 轮询并恢复按钮进度。
- 补充：扩展在后台标签页处理 `Manual download` / 前置确认 `Download` 时优先读取 `href` 直跳并保留批量参数；Manual 为 JS 按钮时改用页面主世界 `button.click()`，不再把 debugger 坐标点击作为唯一入口，修复后台页卡在“正在进入下载页”的问题。
- 补充：批量自动提交按 ZIP 来源分流。直接生成链接走同一 message 生命周期继续推进；下载事件捕获链接则回到 content 再发 `SUBMIT_CAPTURED_URL`，避免 MV3 service worker 在 `downloads.onCreated` 长 fetch 时卡在 `posting`。Nexus content script 会用 `sessionStorage` 记住批量安装上下文，跳转丢参后仍会自动触发提交，不再停在人工“提交到面板”按钮。批量任务面板提交优先经已登录面板页 `panel-bridge.js` 同源转发，复用 Cookie/Vite proxy；面板提交请求增加 30 秒超时和失败回写。
- 补充：远程 ZIP 下载改用独立 15 分钟 archive HTTP client，修复 Ridgeside Village 等大包在 10 秒 Nexus API timeout 下读 body 失败。扩展安装按钮进度也改为继续跟踪面板 job 最终状态：job 创建只到 90%，全部 succeeded 才 100%，任一 failed/canceled 则显示失败。content 直接生成 ZIP 和 downloads 捕获 ZIP 都会统一触发原提交按钮逻辑，background 只做消息丢失兜底。
- 补充：无 `jobId` 但本地 Mod 列表已按 `nexusModId/originNexusModId` 命中的扩展 item 会被视为完成，修复实际安装成功但前端进度卡住的问题。
- 补充：扩展提交消息现在显式携带并恢复 `batchId/itemId/autoSubmit`，background 可在捕获/提交阶段把丢失的 batch 上下文补回 capture，确保新任务 `jobId` 写回 batch item；本地已安装匹配只是兜底。
- 补充：ModsPage 新增扩展安装 `重置状态` 入口，通过 `CLEAR_STATE` 清前端 session 和扩展存储，解决前后端重启后旧 batch 仍卡在浏览器里的问题。
# NEXUS-EXT-BATCH-2 状态

- `NEXUS-EXT-BATCH-2` completed：扩展批量安装终态收敛已修复，`done/failed` 不再被旧 batch 轮询覆盖；完成后会用最新本地 Mod 列表同步搜索结果缓存，避免返回下载页后已安装项又显示“一键安装”。
- 补充：Nexus 多 Mod ZIP 来源纠偏已接入。显式 Nexus/远程安装不再先写推断来源；如果批量上下文传错 `result.modId`，后端会优先使用 ZIP 内唯一正数 `UpdateKeys: ["Nexus:<id>"]` 写 sidecar。当前测试实例的 Ridgeside Village 组件来源已从 SpaceCore 1348 修回 Ridgeside 7286。
# NEXUS-EXT-BATCH-3 状态
- `NEXUS-EXT-BATCH-3` completed：浏览器扩展批量安装入口已增加目标去重和 batch 幂等保护。相同 Nexus `modId` 只打开一个后台页，同一 `batchId` 重复发送不会重复开页，修复 Ridgeside Village 批量安装时本体页面被打开两次、其中一页成功关闭另一页遗留后台的问题。
# NEXUS-EXT-CONNECT-1 状态
- `NEXUS-EXT-CONNECT-1` completed：Mods 下载页在“配置 Nexus Key”旁新增“检测扩展”按钮；扩展会自动识别当前面板 `origin` 并写入 `panelBaseUrl`，连通后普通“一键安装”才开放，未连通时按钮灰色禁用。该握手通过 `GET /api/auth/me` 确认当前页是已登录面板，Premium Key 直连安装不受影响。
# NEXUS-EXT-PACK-1 状态
- `NEXUS-EXT-PACK-1` completed：面板已提供浏览器扩展下载引导。Mods 下载页在 `配置 Nexus Key` 右侧提示 Nexus 普通用户安装扩展，并提供 `下载浏览器扩展` 按钮；Docker 构建期会生成 `/app/browser-extensions/anxi-nexus-installer.zip`，后端优先复用实例目录或镜像中的合法预打包文件，缺失或损坏时才从 `browser-extensions/nexus-slow-installer` 兜底生成。
# NEWGAME-PLAYERLIMIT-1 状态
- `NEWGAME-PLAYERLIMIT-1` completed：自定义新建存档新增 `maxPlayers` 联机人数上限，前端可在“新建存档”界面把总在线人数调到 7 个初始小屋之外；后端写入 Junimo `Server.MaxPlayers`，并显式使用 `CabinStack` 自动小屋管理。`startingCabins` 保持 0-7，继续只表达初始小屋数量。
# PERF-REVIEW-1 状态
- `PERF-REVIEW-1` completed：完成一轮低风险性能/冗余/内存优化。后端存档主 XML 的 farm type 兜底改为流式扫描，备份 ZIP 元数据不再无条件读入完整主存档 entry；Nexus sidecar 展示元数据判断改为按 modId 预建 map。前端 `ModsPage` 合并已安装 Mod 派生数据统计并用 `useMemo` 缓存，减少频繁局部 state 更新下的重复排序/过滤和临时数组分配。
# VNC-CONTROL-1 状态
- `VNC-CONTROL-1` completed：服务器控制页新增管理员 VNC 操作入口。页面刷新后会先通过面板后端代理 Junimo `GET /rendering` 恢复真实渲染状态；`打开VNC显示` 通过 `POST /rendering?fps=15` 打开服务端画面渲染，成功后切换为 `关闭VNC显示` 并可通过 `fps=0` 关闭；`跳转VNC控制` 默认隐藏，仅在显示渲染打开后出现，按当前面板 hostname + 自定义 `vncPort` 打开 noVNC 页面。前端不接触 Junimo API key，VNC 密码不回显。
# FE-PROTOTYPE-LAYOUT-1 状态
- `FE-PROTOTYPE-LAYOUT-1` completed：前端主要 Stardew 页面已按 `external artifact stardew-page-prototypes-image2-2026-06-30` 的信息架构重新排布。总览页对齐农场横幅、生命周期控制、邀请码、摘要指标和三列摘要；存档页新增当前激活存档重点卡；服务器、任务、玩家、模组、诊断、设置页通过页面级布局 class 调整为原型式分区。现有 API 和功能不变，`ModsPage` 保留三段式工作台。
# FE-SHELL-IMAGE2-1 状态
- `FE-SHELL-IMAGE2-1` completed：Stardew Shell 顶栏已替换为 image2 `Top bar.png`，左侧导航迁移到 `Left panel.png`，右侧任务栏迁移到 `01-overview-right-sidebar-empty-image2.png`。顶栏继续显示运行/停止状态、当前农场名、面板版本、管理员/普通用户和登出入口；左侧栏用透明热区承接原九路由点击逻辑，移动端保留横向图标导航；右侧 OpsRail 保留健康和任务状态逻辑。

# FE-TOPBAR-SPLIT-ASSETS-1 状态
- `FE-TOPBAR-SPLIT-ASSETS-1` completed：Stardew Shell 顶栏已从整张 `panel_top_bar_image2.png` 可见背景迁移为 `frontend/public/assets/stardew/ui/topbar/` 下的拆分素材组合。横栏空壳使用三段式 shell，品牌、状态框、农场框、版本框、用户框、头像、下拉箭头和登出按钮均分层渲染；状态红绿点继续复用现有 `.sd-dot` / `.sd-dot-pulse` 动态逻辑，没有改成图片。现有状态/存档/版本/用户/登出点击行为和数据来源保持不变，移动端无横向溢出。

# FE-ASSET-TOP-BAR-SHELL-1 状态
- `FE-ASSET-TOP-BAR-SHELL-1` completed：已从 image2 `Top bar.png` 生成可复用顶栏木质背景空壳素材 `panel_top_bar_shell_empty_image2.png`。素材为透明 PNG，移除鸡图标、品牌字、状态徽章、农场/版本/角色槽、登出按钮和所有图标文字，只保留木纹横栏、金棕边框、四角装饰和像素阴影；当前仅入库生产素材，尚未接入 Shell。

# FE-ASSET-TOP-BAR-CORNERS-1 状态
- `FE-ASSET-TOP-BAR-CORNERS-1` completed：已生成并入库顶栏四角像素装饰素材。新增左上、右上、左下、右下 4 个透明 PNG 和 `topbar_corner_ornaments_sprite_sheet_2x2_image2.png`；素材只保留金棕木质/金属角标、像素阴影和高光，不含顶栏背景、文字、按钮、图标或状态徽章，当前仅入库生产素材，尚未接入 Shell。

# FE-ASSET-TOP-BAR-CHICKEN-1 状态
- `FE-ASSET-TOP-BAR-CHICKEN-1` completed：已生成并入库顶栏左侧品牌鸡图标素材 `icon_topbar_chicken_image2.png`。素材为透明 PNG，只保留白色鸡本体、红色鸡冠、黄色喙、橙色脚、像素描边、阴影和高光，不含品牌文字、顶栏木质背景或其它 UI 元素；当前仅入库生产素材，尚未接入 Shell。

# FE-SHELL-SCALE-1 状态
- `FE-SHELL-SCALE-1` completed：Stardew Shell 首次接入全局等比缩放时，以 `1536x1024` 为设计基准并在 CSS 中计算 `--sd-ui-scale = max(0.72, min(100vw/1536, 100dvh/1024))`。该单位相除方案已被顶部 `FE-RESPONSIVE-VIEWPORT-1` 的 TypeScript 数值计算替代，不能从此历史条目恢复；当前仍使用逻辑宽高 + `transform: scale(...)` 填满可用视口。首次实现已验证前端构建通过，并用临时 HTTP QA 页确认 760x504 为最小三栏基准、1920x1080 可继续等比放大且无页面溢出。

# FE-ASSET-TOP-BAR-BRAND-GLOW-1 状态
- `FE-ASSET-TOP-BAR-BRAND-GLOW-1` completed：已生成并入库顶栏品牌文字暖黄色发光/阴影占位素材 `topbar_brand_text_glow_placeholder_image2.png`。素材为透明 PNG，不含实际文字、鸡图标或木质顶栏背景，仅供前端动态渲染品牌文字时作为底层光效；当前仅入库生产素材，尚未接入 Shell。

# FE-ASSET-FARM-SELECT-FRAME-1 状态
- `FE-ASSET-FARM-SELECT-FRAME-1` completed：已生成并入库顶栏农场选择框空底图 `field_topbar_farm_select_empty_image2.png`。素材为透明 PNG，只保留金棕像素边框、暗棕木纹内容底、像素阴影和下拉框外形，已移除农场图标、农场名、下拉箭头和顶栏背景；当前仅入库生产素材，尚未接入 Shell。

# FE-ASSET-FARM-SELECT-3PIECE-1 状态
- `FE-ASSET-FARM-SELECT-3PIECE-1` completed：已生成并入库农场选择框三段式透明素材和 `field_topbar_farm_select_3piece_sheet_image2.png`。左端/中段/右端均不含农场图标、农场名或下拉箭头，中段可横向平铺；当前仅入库生产素材，尚未接入 Shell。

# FE-ASSET-DROPDOWN-ARROW-1 状态
- `FE-ASSET-DROPDOWN-ARROW-1` completed：已生成并入库顶栏浅金色下拉箭头图标 `icon_dropdown_arrow_gold_image2.png`。素材为透明 PNG，只保留像素箭头、暗色描边和阴影，不含农场框、用户框或文字；当前仅入库生产素材，尚未接入 Shell。

# FE-ASSET-VERSION-BADGE-FRAME-1 状态
- `FE-ASSET-VERSION-BADGE-FRAME-1` completed：已生成并入库版本号小框空底图 `field_topbar_version_badge_empty_image2.png`。素材为透明 PNG，只保留棕色/金色像素边框、暗木纹内部、阴影和高光，不含版本号文字或顶栏背景；当前仅入库生产素材，尚未接入 Shell。

# FE-ASSET-USER-ROLE-FRAME-1 状态
- `FE-ASSET-USER-ROLE-FRAME-1` completed：已生成并入库用户角色框空底图 `field_topbar_user_role_empty_image2.png`。素材为透明 PNG，只保留木质/金色边框、暗棕内容底、像素阴影和高光，已移除头像、角色文字、下拉箭头和顶栏背景；当前仅入库生产素材，尚未接入 Shell。

# FE-ASSET-USER-ROLE-3PIECE-1 状态
- `FE-ASSET-USER-ROLE-3PIECE-1` completed：已生成并入库用户角色框三段式透明素材和 `field_topbar_user_role_3piece_sheet_image2.png`。左端/中段/右端均不含头像、角色文字或下拉箭头，中段可横向平铺；当前仅入库生产素材，尚未接入 Shell。

# FE-ASSET-TOP-BAR-USER-AVATAR-1 状态
- `FE-ASSET-TOP-BAR-USER-AVATAR-1` completed：已生成并入库顶栏用户头像图标 `icon_topbar_user_avatar_image2.png`。素材为透明 PNG，只保留橙色头发、蓝色衣服、脸部细节、像素描边和高光，不含用户框背景、文字或箭头；当前仅入库生产素材，尚未接入 Shell。

# FE-ASSET-LOGOUT-BUTTON-FRAME-1 状态
- `FE-ASSET-LOGOUT-BUTTON-FRAME-1` completed：已生成并入库红色登出按钮空底图 `button_topbar_logout_empty_image2.png`。素材为透明 PNG，只保留红色按钮底、暗红/金棕边框、像素阴影、高光和按键质感，已移除登出图标、文字和顶栏背景；当前仅入库生产素材，尚未接入 Shell。

# FE-ASSET-LEFT-RAIL-SHELL-1 状态
- `FE-ASSET-LEFT-RAIL-SHELL-1` completed：已从 image2 `Left panel.png` 生成可复用左侧栏木质背景空壳素材 `panel_side_rail_shell_empty_image2.png`。素材为透明 PNG，移除导航按钮、菜单文字、菜单图标和按钮状态残影，保留木框、深色木纹、横向分隔、底部置物架与装饰区；当前仅入库生产素材，尚未切换 Shell 引用。
# FE-ASSET-NAV-BUTTON-DEFAULT-1 状态
- `FE-ASSET-NAV-BUTTON-DEFAULT-1` completed：已从 image2 `Left panel.png` 提取并重绘默认态左侧导航按钮空底图 `nav_item_default_wood_blank_image2.png`。素材为透明 PNG，移除中文文字、菜单图标和侧栏背景，只保留木质按钮、金棕边框、像素角饰、内侧阴影和高光；当前仅入库生产素材，尚未接入 Shell。
# FE-ASSET-NAV-BUTTON-ACTIVE-1 状态
- `FE-ASSET-NAV-BUTTON-ACTIVE-1` completed：已生成并抠图入库激活态左侧导航按钮空底图 `nav_item_active_wood_blank_image2.png`。素材为透明 PNG，宽度对齐默认态按钮，保留木质中心、亮金色双边框、像素角饰和轻微暖色发光；不含文字、图标和侧栏背景，当前仅入库生产素材，尚未接入 Shell。
# FE-ASSET-NAV-BUTTON-HOVER-1 状态
- `FE-ASSET-NAV-BUTTON-HOVER-1` completed：已生成并入库悬停态左侧导航按钮空底图 `nav_item_hover_wood_blank_image2.png`。素材为透明 PNG，尺寸对齐默认态按钮 `442x138`，在木质主体上加入轻微金色高光，状态介于默认态和激活态之间；不含文字、图标和侧栏背景，当前仅入库生产素材，尚未接入 Shell。
# FE-ASSET-SIDEBAR-DECOR-PROPS-1 状态
- `FE-ASSET-SIDEBAR-DECOR-PROPS-1` completed：已从 image2 `Left panel.png` 重生成左侧栏底部装饰整组与单件素材。覆盖整组 `sidebar_bottom_decor_props_group_image2.png`，以及单独灯笼、盆栽、紫水晶三个透明 PNG；整组可带木架，单件均不带菜单文字、导航按钮或侧栏背景，当前仅入库生产素材，尚未接入 Shell。
# FE-ASSET-NAV-ICONS-IMAGE2-1 状态
- `FE-ASSET-NAV-ICONS-IMAGE2-1` completed：已生成并入库 image2 左侧导航 9 枚透明图标和 3x3 sprite sheet。图标包括总览地图、服务器机柜、存档宝箱、任务日志卷轴、玩家头像、模组绿色晶体、诊断监视器、安装纸箱和设置齿轮；均不含按钮底图、中文文字或侧栏背景，当前仅入库生产素材，尚未接入 Shell。
# FE-ASSET-RIGHT-RAIL-SHELL-1 状态
- `FE-ASSET-RIGHT-RAIL-SHELL-1` completed：已生成并入库右侧栏木质背景空壳素材 `panel_right_rail_shell_empty_image2.png`。素材为透明 PNG，移除原右侧栏内部三块卡片、标题文字、图标、状态点、进度条和任务内容，只保留外层立柱、完整顶部横梁、深棕内底、藤蔓、底部基座和南瓜/向日葵装饰；当前仅入库生产素材，尚未接入 Shell。
# FE-ASSET-RIGHT-RAIL-BORDER-1 状态
- `FE-ASSET-RIGHT-RAIL-BORDER-1` completed：已生成并入库右侧栏外层木质边框素材 `panel_right_rail_outer_border_image2.png`。素材为透明 PNG，中间区域完全透明，只保留最外侧左右竖梁、顶部/底部边缘、像素阴影、金棕木质雕刻和外框藤蔓点缀；不含内部卡片、文字、图标、进度条、任务内容和底部南瓜/向日葵装饰，当前仅入库生产素材，尚未接入 Shell。
# FE-ASSET-RIGHT-RAIL-CARDS-1 状态
- `FE-ASSET-RIGHT-RAIL-CARDS-1` completed：已生成并入库右侧栏三张卡片空框素材 `panel_right_rail_card_health_empty_image2.png`、`panel_right_rail_card_in_progress_empty_image2.png`、`panel_right_rail_card_recent_tasks_empty_image2.png`。三张素材为透明 PNG，分别对应系统健康、进行中和近期任务卡片，只保留木质边框、深棕内容底、金棕角饰、藤蔓和像素阴影；不含标题文字、图标、状态点、进度条、内部横线、任务列表或其它动态内容，当前仅入库生产素材，尚未接入 Shell。
# FE-ASSET-RIGHT-RAIL-CARDS-NINESLICE-1 状态
- `FE-ASSET-RIGHT-RAIL-CARDS-NINESLICE-1` completed：已生成并入库右侧栏三张九宫格卡片框素材 `panel_right_rail_card_health_nineslice_image2.png`、`panel_right_rail_card_in_progress_nineslice_image2.png`、`panel_right_rail_card_recent_tasks_nineslice_image2.png`。三张素材为透明 PNG，四角装饰完整，边框中段更规则，中心保留深棕木纹/皮革纹理；不含文字、图标、状态点、进度条、内部横线、任务列表或参考线，当前仅入库生产素材，尚未接入 Shell。
# FE-ASSET-RIGHT-RAIL-TITLE-ICONS-1 状态
- `FE-ASSET-RIGHT-RAIL-TITLE-ICONS-1` completed：已生成并入库右侧栏三枚标题图标 `icon_right_rail_health_heart_image2.png`、`icon_right_rail_in_progress_clock_image2.png`、`icon_right_rail_recent_tasks_clipboard_image2.png`。三枚素材为透明 PNG，分别对应系统健康红心、进行中蓝色时钟和近期任务剪贴板，只保留图标本体、像素描边、阴影和高光；不含文字、卡片框、右侧栏背景、进度条、状态点或列表内容，当前仅入库生产素材，尚未接入 Shell。
# FE-RIGHT-RAIL-3PIECE-RUNTIME-1 状态

- `FE-RIGHT-RAIL-3PIECE-RUNTIME-1` completed：右侧 OpsRail 运行时已接入新三段外壳，顶部固定段使用 `right_rail_shell_top.png`，中段使用 `right_rail_shell_middle_tile.png` 纵向 `repeat-y`，底部固定段使用 `right_rail_shell_bottom.png`。不再在生效规则里用整张右栏 shell 或整张截图做 `100% 100%` 拉伸。
- 三张信息卡片已改用 `right_card_health_9slice.png`、`right_card_progress_9slice.png`、`right_card_recent_9slice.png` 作为 `border-image` 九宫格卡片框；标题、图标、健康状态、任务列表、按钮文字和状态点仍由 React/CSS 动态渲染。
- 已验证 `cd frontend; npm.cmd run build` 通过；本地浏览器 QA 覆盖 1280x720、1280x900、1280x560、390x760，确认中段不纵向拉伸、矮窗口 stack 内部滚动、移动端隐藏右栏、console error/warn 为空。
# FE-RIGHT-RAIL-PROTOTYPE-ALIGN-1 状态

- `FE-RIGHT-RAIL-PROTOTYPE-ALIGN-1` completed：右侧 OpsRail 已按 `01-overview-right-sidebar-empty-image2.png` 原型继续调整运行时比例。右栏列宽改为 `clamp(340px, 27vw, 430px)`；顶部 shell 裁掉上方透明安全边后贴顶；底部 shell 按可见装饰区贴底；卡片收回左右木柱内侧并移除外投影，修复上下边框不贴边和左右边框被横向阴影切断的问题。已验证 `cd frontend; npm.cmd run build` 通过，本地 QA 页面 1280x900 截图确认顶部贴边、侧柱连续、卡片位于内框范围且 console error/warn 为空。

# FE-RIGHT-RAIL-BLACK-EDGE-FIX-1 状态

- `FE-RIGHT-RAIL-BLACK-EDGE-FIX-1` completed：修复右侧 OpsRail 三段 shell 接入后左右两侧露黑的问题。中段 `right_rail_shell_middle_tile.png` 改为 121% 横向 overscan 后居中 `repeat-y`，top/bottom 固定段按 108% 横向 overscan 并同步调整高度和 stack 扣底部装饰高度；兜底色改为木板棕。已验证 `cd frontend; npm.cmd run build` 通过，本地 QA 页面 1280x720 / 1280x560 截图确认黑边消失且矮窗口仍内部滚动。

# FE-MAIN-PAGE-FRAME-1 状态
- `FE-MAIN-PAGE-FRAME-1` completed：所有 Stardew 路由的中间主内容区 `.sd-main` 已统一替换为 image2 存档页空框背景 `main_page_frame_empty_image2.png`。资源从 `external artifact stardew-page-prototypes-image2-2026-06-30 (03-saves-page-frame-empty-image2.png)` 复制到 `frontend/public/assets/stardew/ui/panels/` 供运行时和 Docker 静态发布使用；主内容背景改为居中、不重复、`100% 100%` 铺满，并把页面整体 padding 调整为 `clamp(28px, 2.4vw, 42px)` 避免压到木框角饰。主内容区仍保留 `overflow-y: auto`，但已隐藏 Firefox/Chromium/WebKit 原生滚动条，避免白色竖条压住右侧 frame 边框。已验证前端构建通过，生产 CSS 临时 Shell QA 页在 1280x720 和 390x760 下背景加载、滚动条隐藏、滚动能力保留、无横向溢出、console error/warn 为空。
# FE-MODS-DYNAMIC-PAGESIZE-1 状态
- `FE-MODS-DYNAMIC-PAGESIZE-1` completed：模组下载页 Nexus 搜索结果已改为固定搜索卡片高度 + 动态 pageSize。`.sd-mods-nexus-search-list` 卡片高度固定 `246px`，前端按真实结果网格在 `.sd-main-scroll` 内的可见高度和实际列数计算每页数量，并传给既有 Nexus 搜索 API 的 `pageSize`；加载骨架不参与测量，避免 loading/结果态来回触发刷新；顶部翻页器同步显示“每页 N 个”，底部重复翻页器移除。已验证 `cd frontend; npm.cmd run build` 通过，并用临时本地 QA 页面确认 1040x1120 为 pageSize=4、1040x720 为 pageSize=2、520x720 为 pageSize=1，卡片高度均为 `246px`。
# FE-JOBS-PROTOTYPE-IMAGE2-1 状态
- `FE-JOBS-PROTOTYPE-IMAGE2-1` completed：任务与日志页已按 image2 原型重皮肤为羊皮纸双栏任务台，包含顶部标题虚线、按钮工具条、左侧任务列表、右侧任务详情/进度/SSE 状态/深色日志终端和 VNC 修复提示。原型图未作为运行时资源；纸纹、铜边、选中态、状态徽章、进度条、终端扫描线和警告纸条均由 CSS 实现，按钮/图标复用既有 Stardew PNG/图标素材。业务逻辑、API、权限、loading/error/empty/disabled 状态保持不变。已验证 `cd frontend; npm.cmd run build` 通过，并用临时 QA 页检查 1280x900 与 390x760 无横向溢出、按钮文字不溢出、移动端底部 VNC 提示可滚动完整查看、console error/warn 为空。
# FE-PLAYERS-PROTOTYPE-IMAGE2-1 状态
- `FE-PLAYERS-PROTOTYPE-IMAGE2-1` completed：玩家管理页已按 image2 `05-players - 副本.png` 原型重皮肤为六摘要卡、邀请加入码横条、Junimo 终端、在线玩家表、活动历史和管理员操作区结构。原型图未作为运行时背景或整块素材引用；纸纹、铜边、角钉、分隔线、绿字终端、表格和禁用操作按钮均为 CSS 实现。功能逻辑、API 调用、权限判断、loading/error/empty/disabled 状态保留；按钮/图标复用现有 Stardew PNG/CSS 按钮体系。已验证 `cd frontend; npm.cmd run build` 通过；内置浏览器临时 QA 覆盖 1280x900 与 390x760 无页面级横向溢出，桌面表格操作列首屏可见，窄屏表格仅自身横向滚动，console error/warn 为空。
# FE-DIAGNOSTICS-GAUGE-CODE-1 状态
- `FE-DIAGNOSTICS-GAUGE-CODE-1` completed：诊断与健康页 CPU / 内存 / 磁盘三枚资源仪表已完成代码级视觉优化。数值与 `%` 拆分排版，修复圆心文字拥挤；像素分段进度环、硬边描边、羊皮纸内芯、阴影和高光均由 CSS gradient / custom properties / box-shadow 实现。未新增图片素材，未使用原型图或截图作为资源，业务逻辑、API、权限和状态处理保持不变。
# FE-INSTALL-IMAGE2-ICONS-2 状态
- `FE-INSTALL-IMAGE2-ICONS-2` completed：安装页 CSS 自绘图标已替换为从 image2 安装页原型提取/抠图生成的透明 PNG 小素材。新增 `frontend/public/assets/stardew/ui/install/` 下 6 个单图，覆盖顶部状态土芽和五步时间线图标；未使用整页原型图作为背景或整块资源，页面纸卡、边框、分隔线、进度条和日志终端仍由 CSS 实现。已验证 `cd frontend; npm.cmd run build` 通过，并用临时 QA 页在内置浏览器检查 1280x900、390x760 和“安装游戏”展开表单交互，确认图标资源加载、无横向溢出、按钮文字不溢出、console error/warn 为空。
# FE-CARD-UNIFY-SAVES-1 状态
- `FE-CARD-UNIFY-SAVES-1` completed：除模组管理页外，Stardew 其他页面小框已统一为存档管理页卡片基准，使用暖色纸面、铜色 2px 边框、9px 圆角、内描边和轻微阴影；背景按最新反馈改为干净浅色高光 + 纯色纸面，不再铺密集点状纹理。同步收敛标题/说明字号、padding、gap 和窄屏容器查询，模组页 `.sd-mods-*` 主体卡片保持原状。已验证 `cd frontend; npm.cmd run build` 通过，并用已删除的临时 QA 页检查 1280x720/390x760 无横向溢出、无点状纹理、模组卡未误套新样式。
# FE-CARD-UNIFY-SAVES-1 follow-up
- `FE-CARD-UNIFY-SAVES-1` follow-up completed：总览页四个统计卡 `.sd-mc`（存档/模组/系统健康/运行任务）已按反馈仅移除点状 `radial-gradient` 纹理，保留原有结构、尺寸、边框、阴影和布局。已验证 `cd frontend; npm.cmd run build` 通过。
# FE-SERVER-PLAYERS-CARD-LAYOUT-1 状态
- `FE-SERVER-PLAYERS-CARD-LAYOUT-1` completed：服务器控制页已使用共享 `ServerSummaryCard` 替换原大状态卡并移除独立邀请码卡；玩家管理页已移除顶部摘要卡，将“服务器信息（Junimo）”置底；在线玩家表删除“角色”列，主机标识并入玩家名右侧，并新增可见“农场收入 / 玩家收入”列。已验证 `cd frontend; npm.cmd run build` 通过；内置浏览器 DOM 快照接口本次返回兼容错误，未完成截图式 QA。

# FE-PLAYERS-PROTOTYPE-CURRENT-1 状态
- `FE-PLAYERS-PROTOTYPE-CURRENT-1` completed：玩家管理页已继续按 version-02 `05-players.png` 原型校准卡片比例和首屏节奏，并通过玩家页专属 `.sd-main:has(.sd-players-page)` 收紧主 frame inset。在线玩家表为整行首块，标题改为 `在线: N` / `等待加入: N` 徽章；活动/最近事件与管理操作保持第二行左右两列，管理操作隐藏原型不存在的说明文字；Junimo 终端整行置底并在桌面首屏可见。已验证 `cd frontend; npm.cmd run build` 通过；内置浏览器 QA 覆盖 1536x1024 与 390x844，console error/warn 为空，桌面表格无自身横向滚动条，窄屏无页面级横向溢出。

# FE-MISSING-GAME-INSTALL-PROMPT-1 状态
- `FE-MISSING-GAME-INSTALL-PROMPT-1` completed：每次登录或已有 session 进入 Stardew 面板后，若实例状态未检测到游戏文件，会弹出“请先安装游戏”引导弹窗；主按钮跳转到 `/instances/stardew/install`。已安装状态、正在运行的 `stardew_install` 任务和当前已在安装页时不会触发该弹窗。已验证 `cd frontend; npm.cmd run build` 通过。

# STEAM-QR-PHASE-CLASSIFY-1 状态
- `STEAM-QR-PHASE-CLASSIFY-1` completed：Steam QR 扫码登录后端阶段识别已修正，`Choice [1]: 2` 不再误判为 Steam Guard 手机批准，QR 模式下保持 `steam_qr_required` 供前端展示扫码入口。现场诊断确认 steam-auth 容器基础网络可解析并连通 Steam Web API/CM 端口，当前失败属于 SteamClient QR 登录连接未建立而非 Docker 完全无网络。已验证 `cd backend; go test ./internal/games/stardew_junimo` 通过。

# FE-STEAM-QR-LOG-FALLBACK-1 状态
- `FE-STEAM-QR-LOG-FALLBACK-1` completed：安装页增加 QR 日志兜底，当后端阶段短暂误显 `steam_guard_mobile_required` 但最新日志证明用户选择 QR（例如 `Choice [1]: 2`）时，前端按 `steam_qr_required` 展示扫码交互，避免 QR 流程误显示 Steam Guard 手机批准。已验证 `cd frontend; npm.cmd run build` 通过。

# FE-STEAM-QR-IMAGE-CODE-1 状态
- `FE-STEAM-QR-IMAGE-CODE-1` completed：安装页 QR 弹窗不再把终端字符画作为主扫码对象，而是从最新 `Or open: https://s.team/q/...` 提取 Steam 登录 URL 并用本地 `qrcode` 生成标准二维码图片；字符画仅作为图片生成失败时的备用。已验证 `cd frontend; npm.cmd run build` 通过，后端 QR 阶段识别相关定向测试通过。

# FE-STEAM-AUTH-OPTIMISTIC-PHASE-1 状态
- `FE-STEAM-AUTH-OPTIMISTIC-PHASE-1` completed：安装页 Steam 认证选择已加入本地乐观阶段，点击扫码登录/手机批准/输入验证码后立即切换到对应交互区，不再等待实例状态轮询；当前有效的 QR URL 日志可修正落后的选择按钮，但后续 Guard/下载/失败日志优先。已验证 `cd frontend; npm.cmd run build` 通过，后端 QR 阶段识别相关定向测试通过。
# STEAM-POST-AUTH-RETRY-1 状态
- `BE-STEAM-POST-AUTH-RETRY-1` / `FE-STEAM-POST-AUTH-RETRY-1` completed：Steam 登录成功后的下载/CDN/磁盘/后续安装失败现在归类为 `error/download_failed` 或 `error/post_auth_failed`，前端通过日志兜底识别认证已成功，并只允许复用已保存凭据重试，不再要求用户重新输入 Steam 账号密码。已验证 `go test ./internal/games/stardew_junimo -run "DownloadFailedAfterSuccessfulAuth|InstallMarksSteamAuthFailedWhenRunErrors"` 与 `cd frontend; npm.cmd run build`。
# STEAMCMD-PULL-PROGRESS-1 状态

- `STEAMCMD-PULL-PROGRESS-1` completed：Junimo 镜像拉取与 SteamCMD 兜底镜像拉取都会通过 `[pull:progress:done:total]` 给前端提供估算百分比；安装页顶部总进度、镜像拉取卡和任务日志详情可展示“约 N%”，避免用户只能看 layer 日志猜测。已验证 `go test ./internal/games/stardew_junimo -run "SteamCMD|DownloadFailed|InstallMarksSteamAuthFailedWhenRunErrors"`、`go test ./internal/docker ./internal/games/stardew_junimo/config` 与 `cd frontend; npm.cmd run build`。
# FE-STEAMCMD-BRACKET-PROGRESS-1 状态
- `FE-STEAMCMD-BRACKET-PROGRESS-1` completed：安装页已支持 SteamCMD 原生 `[ 28%] Downloading update (done of total KB)` 进度日志，并继续支持 SteamCMD 手机 App 批准提示。验证：`cd frontend; npm.cmd run build`。
# JUNIMO-IMAGE-CANDIDATES-1 状态
- `JUNIMO-IMAGE-CANDIDATES-1` completed：`server` 与 `steam-auth-cn` 镜像拉取已接入与 SteamCMD 类似的候选兜底机制，默认顺序为 `docker.1ms.run`、`docker.m.daocloud.io`、`ghcr.io`、原始仓库；本地已有任意候选会直接复用，拉取成功后写回 `.env`，避免后续 compose 回到单一镜像源。验证见后端接手文档。
# JUNIMO-IMAGE-CANDIDATES-2 已完成

- 已完成 JunimoServer 与 steam-auth cn 版镜像候选源自动补齐：旧实例单候选 `.env` 会被扩展为默认候选；steam-auth cn 当前顺序为 1ms、阿里云 ACR 新版个人版、DaoCloud、GHCR、Docker Hub。
- 已完成选中镜像写回：后端会把实际使用的 `SERVER_IMAGE` / `STEAM_SERVICE_IMAGE` 和补齐后的候选列表写回实例 `.env`，方便后续重试复用。
# FE-GAME-INSTALLED-STARTABLE-1 已完成

- 已修复安装完成态 `game_installed` 在前端不可启动的问题：总览页和服务器控制页都会把它作为可启动的未运行状态展示。
- 若没有可用存档，点击启动后仍由后端返回 `save_required` 并引导用户创建/上传/选择存档。
# FE-OPSRAIL-METRICS-RESTORE-1 状态
- `FE-OPSRAIL-METRICS-RESTORE-1` completed：右侧 OpsRail CPU / 内存 / 磁盘已恢复轻量实时显示，Stardew 面板挂载期间调用现有 `/api/instances/:id/metrics`，首次立即采样并按 `2s` 刷新；没有用户打开前端页面时自然不会产生浏览器轮询，页面卸载时停止 timer。普通 dashboard 初始化仍不触发 `/api/health/diagnostics`，保留此前诊断降轮询优化。验证：`cd frontend; npm.cmd run build`；Browser QA 打开 `qa-layout.html?state=running` 确认右侧栏显示 mock metrics 百分比而非空值。
# RELEASE-TAG-CI-1 状态
- `RELEASE-TAG-CI-1` completed：面板仓库已新增 GitHub tag 发版 workflow，推送 `v*` tag 后自动构建面板 Docker 镜像，发布 Docker Hub 与阿里云 ACR 的版本 tag / `latest`，并在 GitHub Release 上传 `deploy/run.sh`。配套 steam-service-cn 仓库已改造 tag workflow，可发布 `junimo-steam-service-cn` 到 Docker Hub、阿里云 ACR 和 GHCR。
# RUN-SH-QUICK-MODE-1 状态
- `RUN-SH-QUICK-MODE-1` completed：`deploy/run.sh` 已扩展为完整一键启动菜单，包含 Docker/Compose 自动安装修复、面板镜像候选兜底拉取、启动/停止/重启、普通更新/强制更新、镜像源切换、脚本自更新、虚拟内存、开机自启、状态/日志/访问地址。脚本默认使用宿主机 `~/.anxi-panel/data` 与容器内同名绝对路径持久化数据，避免面板通过 Docker socket 编排游戏容器时出现 bind mount 路径不一致。
- `RUN-SH-QUICK-MODE-1` docs follow-up：README 与镜像构建文档已补充最低系统要求、推荐配置、多人游玩规格和云服务器安全组口径；对外只要求开放 TCP `8090`、UDP `24642` / `27015`，VNC/noVNC TCP `5800` 按需开放，Junimo API TCP `8080` 明确不要开放公网。
- `RUN-SH-QUICK-MODE-1` docs follow-up：README 与镜像构建文档已补充“一键启动脚本”的国内加速安装入口，推荐国内用户通过自有轻量服务器静态分发 `run.sh`，GitHub Release 地址作为备用；Docker 镜像仍由脚本内候选源拉取，不通过该轻量服务器中转。
- `RELEASE-TAG-CI-1` follow-up：面板仓库 ACR 发布地址已切换到阿里云新版个人版实例域名 `crpi-9z3bkb9g7fxeohrg.cn-hangzhou.personal.cr.aliyuncs.com`；GitHub Actions 和 `deploy/run.sh` 默认国内镜像源同步更新。`ALIYUN_REGISTRY_USERNAME` 使用 ACR 访问凭证登录命令中的 `--username` 值。
- `RELEASE-TAG-CI-1` follow-up：配套 `junimo-steam-service-cn` tag 发布 workflow 已切换到同一 ACR 新版个人版域名；面板内 `STEAM_SERVICE_IMAGE_CANDIDATES` 默认把该 ACR 镜像放在第二候选，顺序为 1ms、ACR、DaoCloud、GHCR、Docker Hub。
- `RELEASE-TAG-CI-1` follow-up：面板仓库 tag 发版 workflow 已增加 GHCR 发布目标 `ghcr.io/anxiyizhi/stardew-server-anxi-panel`，并给 `deploy/run.sh` 增加 GHCR 镜像源选项；配套 steam-service-cn workflow 保持发布 `ghcr.io/<owner>/junimo-steam-service-cn`。

# SAVE-POINTER-SUFFIX-HEAL-1 状态
- `SAVE-POINTER-SUFFIX-HEAL-1` completed（代码已修复+测试通过，尚未部署）：修复 JunimoServer 新建存档时把 `gameloader.json` 存档名前缀写错导致"当前激活存档"永久显示"未知"、新建存档轮询误报超时的问题，面板现在能按数字后缀自动识别并修正真实存档目录。详见 `docs/backend-handoff/backend-handoff-2026-07-07.md`。

# INVITE-COPY-CLIPBOARD-FALLBACK-1 状态
- `INVITE-COPY-CLIPBOARD-FALLBACK-1` completed（代码已修复+构建通过，尚未部署）：修复邀请码/局域网 IP 复制按钮在非 HTTPS 访问下因 `navigator.clipboard` 不可用而完全无反应的问题，新增 `execCommand('copy')` 降级方案。详见 `docs/frontend-handoff/frontend-handoff-2026-07-07.md`。

# FESTIVAL-EVENT-1 / JOJA-ROUTE-1 状态
- `FESTIVAL-EVENT-1`/`JOJA-ROUTE-1` completed（代码已完成 + 后端 build/vet/test 全绿 + 前端 typecheck/build 全绿 + 嵌入 SMAPI Mod 已用 Docker 重新编译替换，尚未真机联机验证）：服务器控制页新增"触发节日活动"（模拟游戏内 `!event`）和"永久启用 Joja 路线"（模拟 `!joja IRREVERSIBLY_ENABLE_JOJA_RUN`，需强确认弹窗逐字输入）两个按钮。后者因上游要求触发者持有 admin 角色，后端会先调用 JunimoServer 自带的 `POST /roles/admin` 把主机提升为管理员再模拟指令。详见 `docs/backend-handoff/backend-handoff-2026-07-10.md`、`docs/frontend-handoff/frontend-handoff-2026-07-10.md`。

# CABIN-STRATEGY-1 状态
- `CABIN-STRATEGY-1` completed（代码已完成 + 后端 build/vet/test 全绿 + 前端 typecheck/build 全绿，尚未浏览器实测和真机联机验证）：按用户明确设计口径把小屋策略设置分层——新建存档页只给简化二选一"小屋模式：推荐/原版"（`NewGameConfig.cabinMode`），服务器控制页新增"小屋与联机高级设置"弹窗给完整设置（`CabinStrategy`/`ExistingCabinBehavior`/`NetworkBroadcastPeriod`），新增 `GET/PUT /api/instances/:id/config/server-runtime-settings` 接口，两层共用同一份 `server-settings.json`。详见 `docs/backend-handoff/backend-handoff-2026-07-10.md`、`docs/frontend-handoff/frontend-handoff-2026-07-10.md` 的 `CABIN-STRATEGY-1` 小节。

# APPROVE-PENDING-AUTH-1 状态
- `APPROVE-PENDING-AUTH-1` completed（代码已完成 + 后端 build/vet/test 全绿 + 前端 typecheck/build 全绿 + 嵌入 SMAPI Mod 已用 Docker 重新编译替换，尚未真机联机验证）：玩家管理页新增独立"待认证玩家"卡片，管理员可一键批准密码保护下卡在隔离小屋的玩家，不需要玩家自己正确输入 `!login <password>`。上游 JunimoServer REST API 没有对应端点（`GET /auth` 只有计数，没有名单/批准接口），改为让内嵌 `StardewAnxiPanel.Control` SMAPI 模组反射调用 JunimoServer 内部单例 `PasswordProtectionService.TryAuthenticate`——这是控制模组第一次真正反射进 JunimoServer 私有实现（而非公开契约、非游戏内聊天指令模拟）。新增模组启动时的"反射能力自检"（写入 `status.json` 的 `passwordBridgeAvailable`/`passwordBridgeDetail`），前端据此提前禁用"批准"按钮，而不是等用户点击后才发现没生效。新增 `POST /api/instances/:id/players/approve-auth`，`GET /players` 每个玩家新增 `isAuthenticated`。详见 `docs/backend-handoff/backend-handoff-2026-07-10.md`、`docs/frontend-handoff/frontend-handoff-2026-07-10.md` 的 `APPROVE-PENDING-AUTH-1` 小节。

# PLAYERS-BAN-1 状态
- `PLAYERS-BAN-1` completed（代码已完成 + 后端 build/vet/test 全绿 + 前端 typecheck/build 全绿 + 嵌入 SMAPI Mod 已用 Docker 重新编译替换）：玩家管理页“封禁玩家”已接通。后续 command-result 阶段改为优先按 `uniqueMultiplayerId` 直接调用 `Game1.server.ban`，只有直接 API 不可用才降级唯一名字 `!ban`。用户已在真实实例确认 `Game1.bannedUsers` 随服务器容器重启丢失，UI 使用确定性限制提示；未做面板侧持久化、封禁名单或解封入口。

# PLAYERS-WARP-HOME-1 状态
- `PLAYERS-WARP-HOME-1` completed（代码已完成 + 后端相关包测试通过 + 前端 typecheck/build 通过 + 嵌入 SMAPI Mod 已用 Docker 重新编译替换，尚未真机联机验证）：玩家管理新增“回家”按钮，桌面端和手机端均放在“踢出”左侧；后端新增 `POST /api/instances/:id/players/warp-home`，由嵌入式 SMAPI 控制模组反射调用 JunimoServer `FarmerExtensions.WarpHome(Farmer)`，用于把在线 farmhand 传送回自己的小屋。该能力明确不复用 `TryAuthenticate`，因为已认证玩家不会再次触发认证传送。详见 `docs/backend-handoff/backend-handoff-2026-07-10.md`、`docs/frontend-handoff/frontend-handoff-2026-07-10.md` 的 `PLAYERS-WARP-HOME-1` 小节。
# FE-INSTALL-STEAM-AUTH-BUTTON-1 状态

- `FE-INSTALL-STEAM-AUTH-BUTTON-1` completed：安装页原“更换 Steam 账号 / 重新认证”已替换为总览页同逻辑的“登录授权”按钮，并改为常驻显示；两页现共用 `useSteamAuthLogin`。已验证 `cd frontend; npm.cmd run build` 通过。

# SAVE-BACKUP-GAMEDAY-1 状态

- `SAVE-BACKUP-GAMEDAY-1` completed（代码已完成 + 后端 build/vet/test 全绿 + 前端 typecheck/build 全绿，未做真机联机端到端验证）：存档回档功能重构为完全按"游戏内日期"（年/季/日）管理自动回档点，**取代 `SAVE-BACKUP-POLICY-1`/`SAVE-BACKUP-SCHEDULE-HOUR-1` 里的定时备份/每日快照机制**（取消定时备份，不再有"最新备份"/"每日快照"）。`BackupPolicy` 简化为 `{ gameSaveBackups, retainGameDays }`（默认保留最近 5 个游戏日，1-14 可调）；新增 `auto`/`predelete`/`prerestore` 三类备份 kind，`manual`（手动备份）不再占用自动回档配额；回档前保护备份失败会中止回档且不破坏当前存档；旧 `latest`/`daily`/`scheduled` 磁盘文件不删除，归入前端"其他备份→历史备份"展示。触发时机复用现有 SMAPI `GameLoop.Saved` 事件管线，**未改动、未重新编译嵌入 DLL**。前端"存档"页备份区拆成"游戏日回档"（主列表，按游戏日排序）+"其他备份"（手动/删除前/回档前/历史）两个区块，回档行按钮不再因服务器运行中被无说明地禁用，改为始终可点开确认弹窗并在弹窗内引导先停服。详见 `docs/backend-handoff/backend-handoff-2026-07-11.md`、`docs/frontend-handoff/frontend-handoff-2026-07-11.md`。
- `SAVE-BACKUP-GAMEDAY-MOBILE-1` completed（同日追加，前端 typecheck/build 全绿，未做真机/窄屏实测）：手机端存档页删除恒禁用的"回档"占位按钮，在"存档操作"卡片上面新增和桌面同名的"游戏日回档"卡片（仅管理员可见），复用桌面同一套 `getSaveBackups`/`restoreSaveBackup` API，数据字段和排序口径与桌面一致，仅把 6 列表格改成手机堆叠行展示；回档确认弹窗同样把"服务器运行中"从无说明禁用改为弹窗内醒目引导先停服。详见 `docs/frontend-handoff/frontend-handoff-2026-07-11.md` 的 `SAVE-BACKUP-GAMEDAY-MOBILE-1` 小节。
- `SAVE-RESTORE-AUTORESTART-1` completed（同日追加，后端 build/vet/test 全绿 + 前端 typecheck/build 全绿，未做真机联机端到端验证）：**取代上面两条里"服务器运行中回档需要先手动停服"的交互**——现在确认回档时，如果服务器正在运行，面板会自动完成"停止服务器 → 回档 → 重新启动服务器"整个流程，不需要用户离开弹窗手动操作。后端把这三步编排成一个 lifecycle job（复用现有 `doStop`/`doStart`，不重新实现 compose/Mod 同步/邀请码轮询），`POST .../saves/backups/restore` 新增 `autoRestart` 字段，运行中且传 `true` 时返回 `202 {jobId}`（和启动/停止服务器同一套 job 轮询/SSE 机制），已停止时行为不变。前端桌面和手机端回档按钮和弹窗提交按钮不再因服务器运行中被禁用，弹窗文案改为说明会自动停止/重启服务器。详见 `docs/backend-handoff/backend-handoff-2026-07-11.md`、`docs/frontend-handoff/frontend-handoff-2026-07-11.md` 的 `SAVE-RESTORE-AUTORESTART-1` 小节。

# FE-STARTUP-HOST-CONFIRM-1 状态

- `FE-STARTUP-HOST-CONFIRM-1` completed（同日追加，前端 typecheck 全绿，未做真机联机端到端验证）：修复两个用户反馈的问题。一是服务器控制页"启动/重启"按钮**切换过早**——原本纯按 `active stardew_lifecycle job + state=running` 判定完成（见上文 `FE-LIFECYCLE-BACKGROUND-INVITE-1`），现在叠加一层"主机玩家上线确认"：`state=running` 后，只要在线玩家列表里还没出现 `isHost && status==='online'` 的条目就继续按"启动中"展示，这个判断独立于是否点过启动按钮（刷新页面/换设备打开也生效），同时新增超时兜底避免像 2026-07-06 那次一样因为玩家快照闪烁/不可用导致按钮永久卡死转圈。**上线联调时改过三次**：第一版超时判断挂在了"仅当次点击启动才生效"的本地状态上，刷新页面就失效，已改成完全独立的派生状态；超时阈值也从最初拍脑袋定的 90 秒（用 `docker exec` 只读对比容器内 `status.json`/`players.json` 时间戳实测发现主机上线可能要几分钟）调大到 10 分钟；第三处是 `useStardewDashboardData.ts` 里服务器停止时没有清空在线玩家列表缓存，导致同一浏览器标签页"运行过→停止→再启动"时用旧快照误判主机已在线、按钮一点启动就切回正常态，改成离开 `running` 时同步清空。二是服务器停止后邀请码卡片会残留上一次运行时的旧邀请码——根因是 `refreshInstanceState` 只要后端返回的 `inviteCode` 非空就无条件写入本地状态，没检查 `state`，而后端 `doStop` 按设计不清空该字段；修复为只在 `state` 为 `running`/`starting` 时才采纳该字段，否则清空。局域网邀请（面板访问地址）本来就不受服务器状态影响，确认无需改动。详见 `docs/frontend-handoff/frontend-handoff-2026-07-11.md` 的 `FE-STARTUP-HOST-CONFIRM-1` 小节。
# FE-OVERVIEW-STARTUP-HOST-CONFIRM-1 状态
- `FE-OVERVIEW-STARTUP-HOST-CONFIRM-1` completed：总览页启动按钮现在会等待在线玩家列表中的主机真正在线后才脱离“启动中…”，刷新页面后同样生效，并保留 10 分钟玩家快照异常兜底。前端构建已通过，尚未做真实大存档端到端启动验证。
# PLAYER-OFFLINE-SAVE-FALLBACK-1 状态
- `PLAYER-OFFLINE-SAVE-FALLBACK-1` completed：离线玩家现在可从存档 Farmer 数据补齐最后睡眠位置、坐标和独立钱包收入，且不会覆盖已有运行时缓存；玩家缓存兼容基础 saveId 与带数字后缀的完整存档目录 ID，降低重启/切换标识后历史信息被整体丢弃的风险。后端全仓库测试通过，尚未在生产多人存档验证。
# PLAYER-ROSTER-SQLITE-1 状态
- `PLAYER-ROSTER-SQLITE-1` completed：新增 `save_identities` / `player_roster` / `player_events` SQLite 模型，以 `instance_id + stable_save_id + player_id` 为联合身份，持久化首次出现、最后在线、位置、收入快照和 seen/joined/left 活动；基础 `saveId` 会归一化到完整存档目录 ID。`players.json` 与存档 XML 保持事实输入，旧 `players-cache.json` / `players-events.json` 首次成功导入后删除且不再写入。API 名册与 `recentEvents` 结构不变，后端全量测试通过；真实升级实例仍待验证。
# FE-PLAYER-LOCATION-NORMALIZE-1 状态
- `FE-PLAYER-LOCATION-NORMALIZE-1` completed：新增共享位置格式化工具，统一桌面玩家表、最近事件、移动玩家页和总览展示；`FarmHouse/Cabin/Cellar/Shed/Barn/Coop` 等数字或 UUID 后缀实例名会映射为中文逻辑位置并附坐标，原始唯一名继续保留在 API、SQLite 和桌面悬停标题中。前端 typecheck/build 通过。
# FE-SHARED-WALLET-PERSONAL-INCOME-1 状态
- `FE-SHARED-WALLET-PERSONAL-INCOME-1` completed：玩家表共享钱包的个人收入从误导性的 `0g` 改为“共享模式不统计”，分开钱包个人累计收入与农场团队累计收入展示不变。
# REAL-INSTANCE-CRITICAL-FLOWS-VERIFIED-1 状态

- `REAL-INSTANCE-CRITICAL-FLOWS-VERIFIED-1` completed：用户已确认以下关键链路均完成真实实例验证：大存档启动并等待主机上线；运行中回档自动停止、回档并重启；多人认证批准、踢出、封禁和回家；睡觉后生成游戏日回档点；Steam/SteamCMD 授权与镜像候选降级。本状态覆盖对应旧条目中的“尚未真机/端到端验证”，但不覆盖未明确确认的移动端视觉、封禁跨重启持久化等独立验证项。

# FE-LIFECYCLE-STATE-MACHINE-1 状态

- `FE-LIFECYCLE-STATE-MACHINE-1` completed：总览页与服务器控制页已改为共用 `useStardewLifecycleState`，统一 active lifecycle job、driver stopping、启动/停止 pending、等待主机上线和 10 分钟超时的判定；启动、停止、重启及运行中回档自动重启均消费同一套派生状态。前端构建通过，未修改 API。
# UI-LIFECYCLE-STATUS-1（已完成，2026-07-11）

- [x] 后端提供七态 UI 生命周期语义，前端停止自行拼装状态。
- [x] 现有诊断页展示实例、Driver、SMAPI status/players 来源及更新时间。
- [x] 诊断页补充 Compose 快照、存档/缓存身份、控制文件新鲜度、启动阶段耗时和控制模组/Junimo 版本矩阵。
- [ ] 后续将当前基于三类更新时间差值的启动耗时升级为持久化阶段事件，支持跨重启历史趋势。
# FE-LIFECYCLE-LIVE-SIGNAL-PRIORITY-1 状态
- `FE-LIFECYCLE-LIVE-SIGNAL-PRIORITY-1` completed：在线玩家列表确认主机在线后立即结束启动中间态，不再等待邀请码或滞后的后端 `uiStatus`；点击停止后本地 pending 状态立即展示“停止中”。桌面总览、服务器控制页共享 hook 与手机总览均已统一。相关文件独立 TypeScript 校验通过；完整构建受工作区另一批未完成的 ServerControlPage hook 拆分影响。
# COMMAND-RESULT-PROTOCOL-1 状态

- `COMMAND-RESULT-PROTOCOL-1` 阶段 1 completed：控制命令已有稳定 commandId、命令/结果原子文件协议、`command-results/`、七状态与结构化错误码、`commandResultVersion: 1`、非阻塞提交和只读查询 API；旧控制模组继续兼容。结果闸门保证已有结果不重复消费，崩溃歧义返回 unknown 且不自动重试；终态结果采用 7 天 + 24 小时 expired 墓碑清理策略。嵌入控制模组已重新编译并更新 DLL。阶段 2 的玩家操作精确 succeeded/failed 尚未开始。
# PLAYER-COMMAND-RESULTS-1 状态

- `PLAYER-COMMAND-RESULTS-1` implemented：command result v1 已为 warp-home、kick、approve-auth 接入真实 succeeded/failed 回执，包含完整结构化错误码；桌面与手机共用 500ms/10s 轮询、旧模组兼容、按玩家 busy 和 unknown 不重试语义。ban、broadcast、event、joja 保持 dispatched，未提前扩展。
# BROADCAST-BAN-RESULTS-1 状态

- `BROADCAST-BAN-RESULTS-1` implemented：broadcast/say 已能确认交给游戏聊天系统后返回 succeeded；ban 优先 uniqueMultiplayerId + `Game1.server.ban` 精确调用，名字降级存在重名时拒绝且只能返回 dispatched。桌面/手机已覆盖 succeeded/dispatched/failed/unknown/旧模组。用户真机确认封禁随容器重启丢失；名单持久化与解封入口不在本阶段。event、joja、save-now 保持未接入精确结果。

# EVENT-JOJA-SAVE-RESULTS-1 状态

- `EVENT-JOJA-SAVE-RESULTS-1` completed：trigger-event 与 enable-joja 已区分明确失败、聊天 dispatched 和可持久确认的 succeeded；save-now 已通过 commandId tracker 关联 `GameLoop.Saved`，两分钟超时为 save_timeout，崩溃歧义为 unknown 且不重试。桌面/手机统一轮询与精确文案，新增游戏内保存入口并与 ZIP 备份区分。Go、前端和模组构建全绿，嵌入 DLL 已更新；现有存档实测 event 明确失败、Joja dispatched、save Saved succeeded。

# COMMAND-RESULT-PRODUCTIZATION-1 状态

- `COMMAND-RESULT-PRODUCTIZATION-1` completed：回执历史已 SQLite 化并支持幂等导入、面板重启恢复、安全文件交接、30 天/数量保留和最终审计；任务与日志页及诊断页已展示协议历史与卡死信号。旧模组继续退化为“已提交，无法获取精确结果”，没有新增任何游戏命令或改变命令保证语义。
# SAVE-BACKUPS-EMPTY-LIST-1（已完成，v0.1.12）

- 修复全新服务器零备份时存档页黑屏，恢复首次进入存档页创建新存档的主流程；后端空列表契约与前端兼容保护均已完成。
# INSTALL-RUNTIME-VERIFICATION-1（已完成，v0.1.13）

- 修复新服务器游戏文件未真正安装完成却被误判为“安装成功”的问题；安装、启动和状态协调统一验证完整游戏运行文件，Steam 授权操作不再覆盖安装错误状态。
# 2026-07-13 已完成：JUNIMO-STACK-UPDATE-1 阶段三

- 已完成 server + steam-auth-cn 不可拆分成对 apply：管理员固定确认、实例独占互斥、关键预检重跑、可信镜像精确 digest、认证卷私有快照、五字段原子配置、auth-first/server-second 验收、原运行态恢复和审计。
- 已完成失败成对回滚与重启恢复：恢复原配置/认证卷/旧 digest pair，终态为 `failed_rolled_back` 或 `rollback_failed`；不确定状态禁止猜测，后者保留材料并仅提供人工处理。
- 安全边界固定：不接受目标/服务/命令，不用 latest，不执行 `down -v`，不删除 game-data/steam-session/存档/Mods/settings/control，不复用 Panel updater。真实 Steam 账号与真实推荐上游镜像的发布前长流程验收仍是发布门禁。
# 2026-07-14 已完成：GAME-RUNTIME-VERSION-1 游戏与 SDK 只读版本检测

- 已完成 App 413150 / 1007 推荐矩阵、ACF tokenizer、只读 volume 检测、六态比较、管理员 API、诊断详情、tested 总览提示、只读预检以及手动候选发现 workflow。
- 推荐 buildid 随 Panel 发布且必须经过发布前验证；workflow 输出始终标记 `discovered`，只写 summary/artifact，不改 main、推荐矩阵或 tag。
- **阶段六未实现**：不创建 staging volume、不下载 depot、不执行 app_update、不写 game-data、不停服/重建容器，也没有游戏/SDK更新按钮。后续阶段须单独设计容量复核、账号授权、原子切换、兼容矩阵、健康验收和完整回滚。

## SMAPI 推荐升级子阶段（2026-07-14，已完成代码实现）

- 完成 Panel 内置 SMAPI 4.5.2 推荐 URL/SHA/大小与 game+SDK+Junimo+auth+Control 兼容矩阵；用户实例不追踪 GitHub latest，维护者候选发现不改矩阵/tag。
- 完成实际 game-data DLL/安装产物检测、七态 API、runtime-components 扩展、诊断 UI、严格无目标 POST、可信下载与 ZIP 安全验证。
- 完成显式 GAME_DATA_VOLUME staging 克隆、官方 Linux installer、精确验版、Control 联动、原子切换、完整 stack 验收、原状态恢复、失败自动回滚和重启恢复。
- 完成实际 game-data 容量估算、JunimoServer/Control 日志加载证据、Control 原先不存在时的精确回滚、初装与升级共用可信下载边界，以及隔离 staging installer Docker integration 测试。
- 完成玩家完整包推荐 installer/version/SHA 联动；增量包继续不携带 SMAPI，旧包不可变。
- **阶段八统一发布列车待办**：在真实 release-candidate Panel/Junimo/auth 镜像上跑完整长链路与故障注入；复核 4.5.2 对推荐 Stardew/SDK buildid 的游戏内兼容；生成并实机验证新版 Windows 玩家完整同步包；审查镜像 SBOM/签名与发布说明；之后才允许更新正式版本/tag/镜像。本次未提交、推送、打 tag 或发布包。

## 2026-07-14 阶段八统一发布列车状态

- 已完成：统一 schema v1、内嵌 recommended 快照、五组件精确版本与 digest/checksum、状态机与越级保护、同通道唯一 recommended 校验、withdrawn 新装/升级门禁、minimumPanelVersion 门禁。
- 已完成：取消 auth-cn 发布驱动的 repository_dispatch/自动 PR；每个 Panel 版本直接指定完整 server/auth 及其他组件版本，普通 PR 和 tag 发布继续运行无真实 Steam/registry secrets 的完整 CI 与隔离 Docker integration。
- 已完成：诊断页统一运行环境版本视图；实际更新仍拆分为 Junimo 对、游戏/SDK、SMAPI 三个安全事务。
- 后续发版只需：确认目标 Junimo server 和对应 auth-cn 精确版本，更新 Panel 内嵌组件清单及 digest/buildid/SHA，在本机 Docker Desktop 和 CI 完成测试，然后创建 Panel tag。用户升级 Panel 后收到对应组件升级提示。无需 candidate/tested/recommended 晋级、Environment reviewer、`APPROVED_STACK_VERSION` 或 Steam E2E artifact。当前工作未创建 tag、PR、镜像或生产变更。
# 2026-07-14 更新安全审查问题关闭

- [x] SMAPI 使用真实 Compose 状态决定停服，并在停服后/切卷前 Panel 中断时恢复原运行服务器。
- [x] SMAPI 回滚增加旧栈全链路验收，验收失败保留 `rollback_failed` 与恢复材料。
- [x] Junimo 回滚记录运行容器真实 ImageID，回滚重建固定不可变 ID；auth-only 状态漂移先停服再快照。
- [x] 兼容矩阵 CI 对基线与当前目录按 stackVersion 强制逐级迁移，push 分支修正为 `main`；发布验收记录绑定当前仓库、当前提交、成功 run 与当前 stack 的未过期 artifact。
# 2026-07-14 已完成：真实镜像 inspect 与 .NET auth 探针兼容

- [x] Docker 镜像/容器元数据读取缩小为格式化安全字段，真实 `sdvd/server:1.5.0-preview.121` 不再因环境变量脱敏导致 JSON 解析失败。
- [x] steam-auth ready/ticket 探针移除 Node.js 运行时假设，并用无 Node fixture 与真实 `steam-auth-cn:1.5.0-anxi.2` 验证。
- [ ] 真实已登录 Steam 测试账号的 session 保持仍属于发布 Environment 长链路验收，不能用无凭据探针替代。
# 2026-07-14 已完成：JUNIMO-UPDATE-PROGRESS-1

- [x] 修复 `.121/.125` 均无 `wget` 导致的新版本验收与旧版本回滚连续误判，统一改用 Junimo 镜像已有的 Bash health 契约。
- [x] Junimo dry-run/apply 输出镜像层下载进度，并保留初始失败与具体回滚失败原因。
- [x] 版本维护卡内一键展示校验、下载、安装和验收；技术详情降级为开发者排障信息，失败状态不再被“无需处理”覆盖。
- [x] 游戏/SDK 只展示已有安全预检，不把未实现的 apply 伪装成可在线升级。
# 2026-07-14 已完成：RUNTIME-VERIFY-FIFO-1

- [x] 定位 `.125` 健康运行仍等待五分钟的根因：非 TTY `attach-cli` 验收必然失败。
- [x] 改用正式 FIFO 控制通道验证 `info` 新响应，保留全部其他验收门槛与自动回滚。
- [x] 增加成功路径禁止 `attach-cli` 和控制契约失败回滚测试。
# 2026-07-14 已完成：FE-COMPONENT-UPDATE-GENERATION-1

- [x] 修复历史 dry-run `succeeded` 导致新 dry-run 与 apply 同时 POST 的竞态。
- [x] Junimo 与 SMAPI 均绑定新预检任务 ID，并阻止同一 ID 重复提交 apply。
- [x] 修复历史失败 apply 覆盖较新预检进度的问题。
- [x] 增加纯状态测试、本地延迟竞态 QA 场景，并将测试加入两个正式 CI/发布门禁。

# 2026-07-15 已完成：MOD-FARM-RUNTIME-CATALOG-1

- [x] 控制 Mod 0.2.0 从本次运行的 `DataLoader.AdditionalFarms` 输出 schema 2 农场目录。
- [x] 用 transactionId/requestId 绑定后端 catalog request 与控制 Mod options，拒绝旧缓存。
- [x] 以已加载 Mod `UniqueID + Version` 稳定排序计算 SHA256，并在创建请求前与准备集合比较。
- [x] 显式解析官方别名、Meadowlands、模组 ID 和 `modded`；未知 ID 回落 Standard 但保持 unresolved。
- [x] 结构化 marker 校验 schema、transactionId、expiresAt，取消 SaveLoaded 的无条件删除。
- [x] 后端在 `/newgame` 前校验运行时目标；失败返回结构化错误并走事务恢复，绝不继续 POST。
- [x] 构建并嵌入 Control 0.2.0 DLL，SHA256 `5e82eb847734d81c08f7295525944e53f343fc3e67715868198bc551e96b24ce`。
- [x] 在独立临时 Compose project/volumes 中启用真实 SVE 1.15.11；matching transactionId/fingerprint 的 fresh options 经 Content Patcher 注入后包含 `FrontierFarm`，旧 options 与早期缺目标 catalog 均不能放行。
- [ ] 模组创建入口继续关闭。下一阶段应在上述真实验证和待创建 Mod 集合接线完成后，单独评审是否开放；不得仅依赖离线目录。

# 2026-07-15 已完成代码：MOD-FARM-CREATE-1

- [x] 统一官方/显式 custom/`modded` 契约，未知值不回落 Standard。
- [x] 精确依赖集准备、fresh runtime 门禁、XML ID 验证、rollback 与正式 save profile。
- [x] feature flag 开启时前端可选择 ready FrontierFarm；存档页桌面/移动端显示 label/ID。
- [x] 默认开关关闭；未发布、未创建 tag。
- [x] 真正隔离实例已完成 SVE 1.15.11 显式创建、XML=`FrontierFarm`、重启、Standard/FrontierFarm 往返和依赖保持；既有实例未操作。技术 E2E 门槛已满足，现已完成默认开放。
- [x] `MOD-FARM-RELEASE-GATE-1`：故障注入发现并修复 Junimo 启动期自动建档与后端 POST 叠加产生双目录；唯一启动期结果现在跳过 POST，ambiguous/unknown 不重试。
- [x] 导入 custom save 根据 XML FarmType 写 profile：强制启用精确依赖并保留当前已启用 Mod；真实 FrontierFarm 完成备份、恢复、导出、删除/导入并重新加载，7 个必要组件保持启用。
- [x] disabled required dependency 的直接创建请求在 job/容器/`/newgame` 前返回 `farm_dependencies_missing`；必须先管理员确认一键准备。
- [ ] 发版前补一次 900px 浏览器与 console-error 人工走查；已有 1280/390 证据，当前 localhost 被 in-app Browser 客户端策略阻止。该视觉走查不改变后端门禁，默认开关现已开启。
# 2026-07-16 状态更新
- [x] 修复模组农场新存档把依赖闭包误当成完整启用集合的问题；profile 现在保留创建前已启用 Mod、强制补齐农场依赖，并保持原先 disabled 的 Mod 关闭。
- [x] 桌面端和移动端增加当前存档 Mod 一键全部启用/禁用；后端以单次 profile 写入应用所有可切换 Mod，并保护内置运行组件。

- [x] 多 Mod ZIP 逐 manifest 导入，同时跳过 SMAPI 内置重复件并返回可解释统计。
- [x] 启动/重启安全隔离既有 SMAPI 顶层重复件，保留可恢复文件。
- [x] 修复隔离 SMAPI 顶层重复件后新建存档误报 `mod_fingerprint_mismatch`：fingerprint 直接读取 `mods/smapi` bundled 组件并规范化短语义版本。
- [x] 无声卡容器的 OpenAL/SDL dummy 后端与旧实例 compose 自动迁移。
- [x] SVE 旧存档 28 人/旧世界数据兼容性检测与桌面、移动端提示。
- [ ] 通用世界大修 Mod 的存档迁移继续暂缓；没有 Mod 官方迁移器和可逆 schema 前不自动重写地形、树木、任务或事件。
- [x] 模组地图创建在完整真实 SVE 与故障注入验证后改为默认开启；仍支持环境变量显式关闭，全部安全门禁保持。
# 2026-07-16 已完成：AUTH-CAPABILITY-DECOUPLE-1

- [x] v0.4.10 历史方案曾让 Junimo server/auth 升级、重启续验和回滚验收 `/steam/ready`；该方案仍会在 Steam 网络挂起时阻塞，现已由 `RUNTIME-AUTH-HEALTH-PROBE-1` 的严格 `/health` 服务契约取代。
- [x] Junimo 与 SMAPI 升级/回滚不再把邀请码作为成功条件，LAN-only、从未登录 Steam 的实例可以正常完成维护事务。
- [x] Steam 在线状态保留为独立能力：未建立完整会话仅提示 warning；端点不可达、JSON/schema 损坏仍失败并按原事务安全规则回滚。
- [x] 增加 LAN-only 成功与 Auth 服务失败回滚测试；定向 Stardew Junimo 与 Docker 测试通过。
# 2026-07-16 已完成：REQUIRED-RUNTIME-BUNDLE-1

- [x] 内嵌矩阵显式声明 Junimo/auth `runtimeUpdatePolicy=required`，当前 Panel 强制使用 server 125 + auth 1.5.0-anxi.2。
- [x] 新 Panel 启动后自动处理 121：可信配置修复 → dry-run → apply → 恢复原运行态，全程不再二次确认。
- [x] 新安装与已是 125 不重复迁移；自定义/不可信配置和 rollback_failed 不自动覆盖；同一 Panel/stack 确定性失败不随重启无限循环。
- [x] 未完成 required 更新时禁止启动旧运行栈，保留停止、诊断、回滚和人工重试能力。
- [x] 前端移除当前版本的“可选更新”语义，兼容矩阵 Go/Python schema、后端状态机及 LAN-only 验收测试已覆盖。

# 2026-07-16 已完成：GAME-LANGUAGE-1

- [x] 服务器游戏语言默认简体中文，并支持 Stardew Valley 官方 12 种语言。
- [x] 老实例首次升级保留已有合法语言；面板保存后每次启动前同步 `startup_preferences`。
- [x] 桌面与移动端提供“保存”和运行中“保存并重启”，并与面板界面语言解耦。
# 2026-07-17 partial: SAVE-IMPORT-E2E-RELEASE-1 real technical E2E

- [x] Isolated `.125` takeover/as-is and swap technical chains reached their strict evidence gates; both survived a real second server restart. Swap included matching `GameLoop.Saved`, dayTransitionComplete, stable XML/hash and completed runtime-state promotion.
- [x] Real-run defects in version matching, command registration timing, process identity, baseline selection, restart recovery, BOM parsing, file stabilization, result archiving and Control save-now were fixed and covered by tests.
- [ ] Eight non-unique rich fixtures, human Stardew client semantic verification, desktop/mobile traversal of the same live job, and the complete fault-injection matrix remain outstanding.
- [ ] Keep `SAVE-IMPORT-JUNIMO-1` incomplete. Preserve its original blocked history: upstream still lacks commandId, while the implemented replacement is Panel composite black-box evidence.

## 2026-07-17 partial: local rich-save takeover

- [x] Reused a downloaded local game only as a read-only source, cloned it into an isolated volume, retained the original ZIP/SHA256, and completed a real upload/takeover job plus second restart.
- [x] Verified the copied save retained two farmhands, three cabins, furniture/fridge data and three cellar assignments; fixed diagnostics-baseline startup polling and the missing-original-pointer pre-start gate.
- [ ] noVNC/game-client visual semantics, swap role selection with the real player's platform identity, spouse/children/pet, reconnect and sleep remain unverified. `SAVE-IMPORT-JUNIMO-1` is still not completed.
# 2026-07-18 已完成：PANEL-POLL-LEAK-1

- [x] 邀请码普通查询只读 `/tmp/invite-code.txt`，空值返回 `n/a`，完全移除 attach-cli 回退。
- [x] 邀请码和资源指标按实例增加 5 秒缓存与 singleflight，多浏览器并发不重复启动 Docker exec/stats。
- [x] 活动重启 job 拒绝第二次重启提交并返回稳定 `409 restart_in_progress`，不取消原任务。
- [x] 页面隐藏/关闭停止玩家、邀请码和指标轮询；恢复可见后继续。
- [x] 单元并发回归、后端/前端构建与 Docker Desktop 隔离真实 Compose exec/stats 验证通过；无测试容器和 attach-cli 遗留。

# 2026-07-19 已完成：CONTROL-PAUSE-COMPAT-1

- [x] Control 0.2.1 使用真实连接数修复删除人物后无人状态无法自动暂停的问题，不修改 Junimo 上游。
- [x] 暂停协调器改为前后帧单向补写 `IsPaused=true`，移除 `gameTimeInterval` 旁路和全局暂停清理，避免与上游及其他 Mod 抢写解除状态。
- [x] 覆盖登录/捏人过渡连接、单人/多人菜单、删除、断线模型、普通日、节日、新日和 2:00 结算边界。
- [x] Control 契约矩阵、真实 Mod 编译与 Docker Desktop Junimo `.125` 删除/时间/节日/重启验证通过。
# RUNTIME-COLD-START-1：低资源升级稳定性（2026-07-19，completed）

- [x] server 冷启动验收从 5 分钟扩展到 20 分钟，保留完整 digest/health/SMAPI/Control/FIFO 门禁。
- [x] server/auth stop 短时 Docker 超时在 10 分钟内幂等重试，持续失败才进入人工恢复。
- [x] 新旧 Compose 增加相对 CPU shares，低资源 Docker 容量只读探针与 apply warning 已覆盖。
- [x] 阿里云 2 vCPU/1.6 GiB 现场恢复完成；Docker Desktop 隔离 `.121 -> .125` stopped/running 分别在 173.86 秒和 106.34 秒升级成功并恢复原状态，完整发布门禁与 `0.3.11-rc` smoke 已通过。
# CONTROL-PAUSE-FEEDBACK-1：在线暂停反馈锁（2026-07-19，completed）

- [x] 生产三人现场复现并确认 `requestingTimePause -> IsPaused -> requestingTimePause` 自激锁，排除容器健康、性能和存档故障。
- [x] Control 0.2.2 删除 connected-client/menu 强制暂停分支，仅保留真实零连接兼容；契约矩阵固定任意正连接数都返回 `None`。
- [x] 运行时检查把旧 `options.json.controlModVersion` 识别为 `control_update_available`，保证 Panel 升级后通过受控重启加载新 DLL。
- [x] Docker Desktop 真实 `.121 -> .125` 与 `.125 old-Control -> .125 Control 0.2.2` stopped/running 四条链通过；无 test seam 或生产凭据进入候选镜像。

# FNOS-COMPOSE-MIGRATION-1：飞牛旧容器升级引导（2026-07-20，implemented / Linux E2E pending）

- [x] 提供 SSH 一键迁移脚本，自动识别最高版本的运行中健康 Anxi Panel，并对多数据目录歧义安全停止。
- [x] 迁移生成标准 `panel` Compose 服务和完整宿主部署变量，使后续 Panel 内升级获得 canonical labels 与严格路径兜底。
- [x] 使用精确稳定版本、国内镜像候选、OCI identity/version 校验、旧容器保留、部署文件备份和失败自动回滚；不删除数据、游戏容器或 volume。
- [x] Release workflow 上传 `migrate-fnos.sh` 并执行脚本语法/纯函数测试。
- [x] 隔离 Docker 29 dind 成功链已完成：真实 `0.3.7` 独立容器经 ACR 升到 `0.3.13`，健康、精确版本、canonical labels、旧容器保留和 result 文件全部通过。
- [x] 停止新 Panel 的健康失败注入已验证自动回滚：原 `0.3.7` 容器名称、运行状态、`restart=no` 与部署文件全部恢复，新容器和临时旧名称没有残留。
- [ ] 中断和真实多候选容器矩阵仍待补齐（最高 SemVer 纯函数已有回归）；当前不得宣称已经在飞牛真机完成迁移。
- [ ] 后续在 Panel 内设计受控“遗留部署转标准 Compose”入口；已经被旧版前置校验拦截的实例仍必须先执行本脚本一次，无法由尚未运行的新版本自行修复。
# 2026-07-20：一键全栈升级第二阶段

- [x] Compose 服务名安全反查，不再硬编码 `panel`。
- [x] 兼容飞牛部分 labels，并强制核对容器、Compose、镜像与数据挂载。
- [x] 独立 helper 转换标准部署，转换前备份且新 Panel 失败自动恢复旧容器。
- [x] Panel 更新后遍历全部实例校验 Control 版本/DLL hash，并持久化续跑。
- [x] 运行实例执行通告、保存、整档备份、停服、Control 更新、重启与 SMAPI 实载验证；同步失败保持停服。
- [x] 前端在线玩家升级警告、转换入口和逐实例状态。
- [ ] 正式 tag/三仓镜像发布（仅在本阶段 Docker 真机矩阵全部通过后执行）。
- [x] 多个旧 Panel 转换 project 隔离、确认后镜像 ID 防竞态复核。
- [x] 多实例 API 端口内外分离与陈旧 Compose project 环境兼容。
- [x] Docker Desktop `.125` stopped/running Control 实载矩阵及整档备份验证。
# PANEL-SQLITE-INTERRUPT-1（2026-07-24，completed / v0.4.2 release）

- [x] 启动初始化状态单次读取并缓存，首次管理员创建后同步更新；未知页面/API 在 SPA 与数据库前置检查前直接 404。
- [x] SQLite 驱动升级到 `modernc.org/sqlite v1.54.0`，增加“取消一个真实查询后下一次查询必须成功”的回归测试。
- [x] 连续三次原生 `SQLITE_INTERRUPT` 主动退出交给 Docker 重建；成功查询或非 interrupt 错误重置连续计数。
- [x] Docker Desktop 候选镜像、100 条扫描路径、持久卷重启、Linux 容器内 10 轮取消恢复、真实 `0.4.1 → 0.4.2` 升级引擎均通过。
- [x] 后端全量 test/vet/build、updater/runtime Docker integration、兼容矩阵、脚本/ShellCheck、前端九项状态测试、Panel production build 与展示站 production build 通过。
- [x] tag 后正式远端一键升级闭环通过；GitHub Release、Docker Hub/ACR/GHCR 精确 tag（同一 `sha256:0204c2eef8da781e78048292c3352519f3d58f6eb592d6883747626695fb97f8`）与 Pages 展示站均已核对。
# DOCS-PORTAL-0.4.2-VISUAL（2026-07-24，completed）

- [x] 更新卡版本角标从 CSS 硬编码迁移到首页 frontmatter，当前显示 `v0.4.2`。
- [x] 修复首页深色四步流程区标题、副标题和步骤文字被全站正文颜色覆盖的问题。
- [x] Pages 部署成功；线上桌面计算值、截图、零横向溢出和 console 均已复核，窄屏布局沿本地同构产物验证为单列。

# DOCS-PORTAL-DRAFT-REVERT-1（2026-07-29，completed / not published）

- [x] 用户否决未发布首页草稿后，将 `website/` 精确恢复到当前线上版本并移除两个草稿主题文件。
- [x] 删除本轮隔离预览和旧构建产物；右侧预览切回真实 GitHub Pages。
- [x] VitePress production build、源码哈希、线上标题/导航/流程结构与 console 完成复核。
- [x] 先前独立完成的无用素材清理保持不变，不随展示站回退恢复。

# FE-MOD-UPLOAD-GUIDANCE-1（2026-07-31，completed）

- [x] 桌面上传入口悬停/聚焦可见 ZIP 能力边界。
- [x] 桌面与移动上传弹窗常驻展示多 ZIP、单 ZIP 多 Mod 文件夹和 ZIP 套 ZIP 处理方式。
- [x] 两端复用共享文案组件，不改变现有上传 API 或后端解包逻辑。

# RUNTIME-AUTH-OFFLINE-ACCEPTANCE-1 / FE-STEAM-AUTH-WAIT-VISIBILITY-1（2026-08-09，released in v0.4.10）

- [x] 运行栈升级以 steam-auth 容器 running、目标 digest 和可解析服务接口为硬门槛，不再等待 Steam 在线相关 Docker health。
- [x] Steam 未登录或无 ticket 降级为后台重试能力警告，接口故障与 digest 错误继续失败并回滚。
- [x] 前端认证阶段显示“正在尝试 Steam 连接”、累计等待、自动重试和“不是卡死”。
- [x] 探针校验 HTTP status，current `accounts` 必须是数组；只对白名单 HTTP 503 + legacy `ready=false` 离线合约放行，500、503 current/畸形 schema 和真实 Docker 404 均 fail closed。动态计时从读屏 live region 隔离，避免轮询重复播报。
- [x] 后端全量 test/vet/build、前端全部 12 个状态脚本与 production build、Docker Desktop unhealthy/offline 实机、正式 Web 升级/回滚，以及升级后 769×240/280×653 Browser QA 已通过；tag、Release workflow、三仓回拉与隔离冒烟均已完成。

# RUN-SH-SWAPPINESS-1（2026-08-13，completed / not released）

- [x] 确认 `deploy/run.sh swap` 过去只创建并启用 `/swapfile`，没有设置 `vm.swappiness`；已有 `/swapfile` 时的提前返回也不会补齐调优，因此低内存主机即使有 swap 仍可能长期保持 `swappiness=0`。
- [x] 新旧两条路径现在都立即设置并验证 `vm.swappiness=60`，通过 `sysctl.d` 管理文件持久化，并规范化 `/etc/sysctl.conf` 中已有冲突值；缺少 `sysctl.d` 的系统安全回退到 `/etc/sysctl.conf`。
- [x] 新增 `scripts/tests/test_run_sh_swap.sh`，覆盖运行态写入、drop-in、既有冲突值、无 `sysctl.d` 回退、幂等重跑、已启用 swap 的补修和缺少内核参数时安全失败；Git Bash、Linux `bash:5.2`、`bash -n` 与 ShellCheck 0.10.0 通过。
- [x] Release workflow 已纳入 swap 专项测试，并把 `deploy/run.sh` 与两个 run.sh 测试脚本加入 ShellCheck；本任务未创建 tag、未推送镜像或更新 `latest`。
# 2026-08-13 已完成、待发布：安装错误分类与安全操作门禁（FE-INSTALL-DIAGNOSTIC-MAPPING-1）

- [x] 桌面壳、安装页和移动端总览共用安装状态纯分类器；`error` 不再一律显示“未安装”或开放重装，首次安装提示只接受明确未安装证据。
- [x] 必需文件、Compose、镜像、Control、Docker 不可用和证据矛盾分别映射为修复、运行诊断或未知；Control `not_observed` 不误报版本错误，明确 mismatch/invalid 才进入 Control 异常。
- [x] 移动端安装/修复/诊断按钮切到完整桌面版的精确目标路由；安装表单按分类门禁，普通运行错误无法提交安装。
- [x] 安装状态表测、响应式源码门禁和 production build 已通过。候选镜像 Browser fixture 与正式真机矩阵仍是发布门禁，不得因本项完成提前打 tag。

# 2026-08-13 代码完成、发布门禁进行中：启动与新建档耐久性（STARTUP-NEWGAME-DURABILITY-1）

- [x] Control 启动验收区分 pending、ready 与明确失败；缺少本次 `options.json` 保持 starting，完整等待预算耗尽后才安全停服，只有合法快照报告错误版本才使用 version mismatch。invalid snapshot、旧快照清理失败和停服失败各有独立错误码。
- [x] Reconcile 不会越过活动 lifecycle/new-game owner 提前发布 running，外部容器也需要 Control ready。普通宿主重启自动恢复明确排除：游戏保持关闭，由用户手动启动。
- [x] `/state.installationDiagnostic` 与前端共享分类器已把必需文件、Compose、镜像、server 容器、Control static/runtime 和 Docker unknown 分层；`state=error` 不再一律显示“未安装”。
- [x] 新建档强制 `Idempotency-Key`（缺失为 428）、exclusive job、持久 request/config/transaction/job/owner token；相同请求返回原 job，不同配置或其它 owner fail closed。前端相同未接受配置复用 key，配置变化换 key。owner 由完整 staging+fsync+no-replace rename 原子抢占。
- [x] startup/http 单写入者固定；Control、gameloader 或目录任一进展出现后永久禁止第二次 `/newgame`，loader 先行但目录未落盘也只观察。unknown/ambiguous 保存现场，手动启动恢复原事务。
- [x] Control `0.3.1` 与 Go 四段耐久门禁已实现：事务目标 save-loaded、内存完整身份/外观/颜色与唯一 host、同 command ID 的持久 pending journal + GameLoop.Saved、稳定磁盘 XML/SaveGameInfo 与身份收敛。source/embedded manifest、DLL 和运行栈清单已同步。
- [x] 新建档回滚使用 `rolling_back` 和每步 write-ahead journal，中断后手动 Start 只停服并继续原回滚。未完成 owner 阻断存档/安装/更新/玩家/重启计划变更；Panel 启动时 Runtime/SMAPI 升级恢复也强制保持游戏关闭。
- [x] 安装脚本 swap 修复已由 `92f3be6bb2731358420ba315ac18029c2506d81f` 进入 `origin/main`：复用或新建 `/swapfile` 都设置并持久化 `vm.swappiness=60`。下一正式 Release 必须重新上传本提交后的 `run.sh`，不能复用 v0.4.11 附件。
- [x] 图形化 Compose 自动标准化已由 `621c5645e0048da7c4793035615438ed78fc7002` 进入 `origin/main`；安全探针、conversionRequired、外部 volume 保留与回滚代码/专项测试已实现。
- [x] 当前源码 pre-candidate 门禁已通过：Control 契约/真实 game-data 编译与三方 SHA、Go 全量 test/vet/build、前端 14 项/audit/build、脚本/ShellCheck、兼容矩阵/remote artifacts、runtime/updater Docker integration、网站 build；隔离真实 startup/HTTP writer 双路径用时 143.59 秒并通过，POST 分别为 0/1、旧档哈希保持、四段耐久和资源清理成立。
- [ ] 在最终候选提交重新跑 Control 真实 game-data 编译/契约、后端全量 test/vet/build、前端全部状态测试/build、脚本/ShellCheck、兼容矩阵、Docker integration、网站 build 与 Browser 桌面/移动验收；任何失败先修复再重跑。
- [ ] 使用唯一隔离 Docker 资源完成 fresh 与升级后真实建档：分别覆盖 startup writer POST=0、HTTP writer POST=1、loader 先行、请求断流/重复 key、Panel/容器中断、wrong Control、save-now unknown/旧结果、XML/身份 mismatch、owner 恢复与资源清理。
- [ ] 从上一正式版 `v0.4.11` 和运行栈最低支持版 `v0.3.2` 走完整 Web 一键更新；注入 unhealthy/版本错误/转换窗口中断验证自动回滚，并在升级得到的 Panel 上复验安装诊断、启动门和新建档四段耐久链。
- [ ] 构建精确候选并完成全部本地/隔离发布门禁，重点覆盖 `621c564` 的图形化 Compose Web 转换与回滚、Control 0.3.1 实载、新建档四段耐久和现有存档完整性。门禁未完成前不得创建 tag 或更新 `latest`。
- [ ] tag 只能从干净且与 `origin/main` 同步、所有门禁通过的最终 `main` 创建；随后等待 Release workflow，回拉三仓精确版核对 digest/OCI/latest，并核对 Release `run.sh` 确实包含 `92f3be6` 的 swap/swappiness 修复后再做隔离 smoke。
- [ ] 正式镜像发布并回拉通过后再进行生产真机同步验收：用精确 tag 验证 `621c564` 图形化 Compose 转换、Control 0.3.1、现有存档/容器/volume/SQLite 完整性与宿主手动启服边界。完成前不得宣称真机通过；发现高风险问题停止灰度并按正式事务回滚。

# FE-NEWGAME-COMMUNITY-BUNDLE-COPY-1（2026-08-14，released in v0.4.17）

- [x] 新建存档高级设置把误写的“社区中心手机包”更正为“社区中心收集包”。
- [x] 保持 `remixedCommunityCenter` 默认值、勾选行为、提交字段与后端契约不变。
- [x] 前端 production build 以及源码/构建产物正反向文案检查通过。

# RUNTIME-AUTH-HEALTH-PROBE-1（2026-08-14，released in v0.4.17）

- [x] 确认 Issue #9 根因：运行组件验收错误调用会触发 Steam 登录和 App Ticket 的 `/steam/ready`，少数 Steam 网络受阻环境中反复撞上 15 秒单探针与 10 分钟总预算，最终误报认证服务损坏。
- [x] Runtime、最终目标复验、旧版本回滚与 SMAPI 复验统一改为容器 running + 精确 image ID/digest + 严格 `GET /health`；禁止 fallback `/steam/ready`，也不接受 Docker health 代替。
- [x] 固定 HTTP 200 与 `status="ok"`、布尔 `logged_in`、数组 `accounts` 契约；`logged_in=false` 只追加 warning，不阻断 Control-only、Junimo 或 LAN/IP 模式。
- [x] 固定六类错误码并在失败/回滚终态保留最后一次脱敏探针原因；接口不可达、超时、非 200、坏 schema 和 digest 漂移继续 fail closed。
- [x] 增加挂起式本地 Docker fixture，证明 `/health` 快速成功且 Panel 未请求永久阻塞的 `/steam/ready`；覆盖不可达、超时、404、500、坏 JSON、auth 变化/未变化、最终复验、旧栈回滚和 digest mismatch。
- [x] 核实内置及历史清单只支持已审计 `steam-auth-cn 1.5.0-anxi.2`；运行栈与兼容矩阵对其它未审计 tag 返回 `unsupported/auth_health_contract`，不猜测兼容性。
- [x] 严格单测、Docker integration、真实镜像 opt-in、兼容矩阵以及 Linux 全量 Go test/vet/build 已通过，任务 Docker 资源已精确清理。
- [x] 已在 `v0.4.17` 完成候选、`v0.4.16` 真实 Panel Web 升级、unhealthy 回滚、annotated tag、三仓 digest 提升、正式镜像回拉和 Release 资产验收；证据见本文件顶部与 `docs/09-image-build.md`。

# FE-MODAL-VIEWPORT-1（2026-08-15，released in v0.4.18）

- [x] 设置页删除用户与任务日志清理弹窗改为 `body` Portal，不再受页面直接子元素 `position: relative` 规则影响；桌面存档、玩家、服务器、Mod、总览和首次安装确认同步统一。
- [x] 桌面/移动模态统一标题语义、危险 `alertdialog`、初始焦点、Tab 循环、Esc、焦点归还、滚动锁定和背景 `inert`/`aria-hidden`；Joja“填入”按钮保持不变。
- [x] 待认证玩家使用独立三列网格，取消七列表的 `870px` 最小宽度与桌面横向滚动；批准按钮完整可见。
- [x] 登录/初始化流式回退覆盖 16:10 与 3:2；1536×1024 表单完整且无横向溢出。
- [x] 全部 17 项前端状态/布局测试、production build、1536×1024 与 390×844 Browser QA 通过，console error/warn 为 0。
- [x] 候选镜像及 `v0.4.17` Web 升级后的 production bundle 已抽验共享 modal、桌面/移动路由和焦点契约；随自动 `v0.4.18` tag 与同 digest 正式镜像发布。

# FE-CONTROL-COMMAND-PAGINATION-1（2026-08-15，released in v0.4.18）

- [x] 任务日志页最近控制命令固定每页 3 条，显示总条数、当前页/总页数与上一页/下一页边界状态。
- [x] 复用现有 5 秒数据刷新，不增加 API 请求；数据缩短时自动校正当前页，避免空白末页。
- [x] 表格横向溢出限制在内部滚动容器，右侧栏窄宽度下分页按钮保持完整可见。
- [x] 全部 17 项前端状态/布局测试、production build 和 967×732 Browser 真实翻页通过，console error/warn 为 0。
- [x] 候选镜像及 `v0.4.17` Web 升级后的 production bundle 已抽验 JobsLogs/Players 分页契约；随自动 `v0.4.18` tag 与同 digest 正式镜像发布。

# PLAYER-AUTH-MODES-1（2026-08-15，released in v0.4.19，included in v0.5.0）

- [x] 玩家加入保护改为显式 `none / global / role`，旧 `SERVER_PASSWORD` 安装自动兼容为 global/none。
- [x] 角色密码按 `UniqueMultiplayerID` 绑定，只持久化每实例 HMAC verifier；内部 Junimo guard、密钥与 payload 纳入 Docker/API/审计脱敏边界。
- [x] 新增 driver 原子配置事务、revision 乐观锁、当前角色完整性校验和管理员 `GET/PUT /config/player-auth`；旧密码 API 在 role 模式 fail closed。
- [x] Control `0.3.3` 通过 Harmony prefix 只重写 `TryAuthenticate` 输入，继续复用上游 Junimo 的 attempts、timeout、隔离小屋、清理和传送；损坏配置与未知角色拒绝认证，Panel 批准 guard 保持可用。
- [x] 桌面/移动端共用“玩家加入保护”弹窗，支持三模式、角色状态、密码重置、待重启与补丁状态；1280×720 和 390×844 Browser QA 无横向溢出。
- [x] Go 聚焦测试、C# 策略契约、真实 game-data Control 0-error 编译、前端 responsive-layout 与 production build 已通过。
- [x] v0.4.19 候选完成自动认证策略、角色隔离、错误边界、revision/重启、旧实例兼容、Control-only 更新/回滚和 Web 升级验证并正式发布；v0.5.0 从 v0.4.19 的真实 Web 回滚/升级再次覆盖该能力。
- [ ] 两个真人客户端实际输入独立角色密码的人工联机记录仍未补齐；自动策略/Control 夹具不能表述成真人联机已验证。以后修改认证目标、attempts/timeout 或客户端交互时补各自密码、交叉失败和 Panel 批准矩阵。
- [ ] 设备 ID 绑定暂不实现。浏览器 device ID 无法证明 Stardew 客户端身份，且指纹可复制/清理；若以后需要免密码设备授权，应先设计有客户端参与的签名 challenge，不用 localStorage/Cookie 伪装安全绑定。

# PLAYER-LAST-SEEN-SEMANTICS-1（2026-08-15，released in v0.5.0）

- [x] 修复存档名册观测时间被误当作最后在线时间：API `lastSeen` 只读取 `last_online_at`，从未上线角色不再显示“上次 今天 HH:mm”。
- [x] 保留 `last_seen_at` 作为内部观测审计，不迁移或清洗数据库；真正在线过的角色离线后仍保留真实最后在线时间。
- [x] 增加真实 SQLite 双轮 `ListPlayers` 回归，Linux Stardew/storage 全包测试与 Node 22 production build 通过；桌面列名改为“在线 / 最近活动”。

# PLAYER-AUTH-SELF-ENROLL-1（2026-08-17，未发布）

- [x] role 模式允许空角色与 waiting 角色；新/无凭据角色可用第一次合法 `!login` 为当前存档角色自助设置密码，管理员代设、重置和清除继续可用。
- [x] 新增按 saveId 隔离的 `role-passwords.json`、initialized marker、跨 Panel/Control 文件锁、原子 `0600` 写入、legacy payload 迁移和损坏 fail-closed；Panel store/`.env` 更新支持事务回滚。
- [x] API/前端增加 waiting/configured/error、store ready/detail、error/orphan 计数；空列表可先启用，凭据异常不会伪装成待设置。
- [x] Control 升到 `0.3.6`，运行栈清单和嵌入 DLL 摘要同步；角色补丁仍复用 Junimo attempts/timeout/隔离/传送。
- [x] 旧 Compose 自动补四个 SAP 环境变量，restart 强制只重建 server；role 无法安全迁移时阻止，none/global 自定义 inline environment 保留并告警继续。
- [x] 修复 restart 请求后 stale running 立即清 pending 的重复提交窗口，新增纯状态回归。
- [x] Docker Desktop Linux 全量 Go test/vet/build、Compose integration、Control 契约/标准编译、前端 18 项全量/build、本地镜像 fresh/restart 冒烟均通过；任务容器/volume/image/端口/临时副本清理为 0，证据见 `docs/09-image-build.md`。
- [x] 用户于 2026-08-17 确认两个真人客户端已完成首次认领、各自/交叉密码、清除后重认领、Panel 批准和重启保持矩阵。
- [x] 本次按用户要求未打 tag、未创建 Release、未更新 `latest`；以后发布必须重新走不可变候选和上一正式版 Web 升级/回滚门禁。

# INSTALL-SMAPI-LIVE-PROGRESS-1 / STEAMCMD-MIGRATED-AUTH-REUSE-1（2026-08-18，released in v0.5.4）

- [x] SMAPI 受审查安装包下载器增加逐写入字节回调、候选序号和已校验缓存命中事件；installer 节流写入 job marker 与实例状态，下载完成和完整性校验成功保持两个阶段。
- [x] 前端第四步改为“下载与环境”，右栏按认证/镜像/下载/SMAPI 动态命名；SMAPI 显示真实字节、百分比、下载源、持续活动提示和 reduced-motion/ARIA 契约，不再沿用 SteamCMD 100% 造成假卡死。
- [x] legacy SteamCMD 授权卷迁移成功后，同一次非强制重装立即先尝试 username-only 缓存登录；失效时只自动回退一次完整登录，成功标记仍由真实 SteamCMD 登录/下载产生。
- [x] 新增流式进度、缓存命中、迁移缓存即时复用和前端 marker/布局回归；Windows 定向 Go 测试、`test:install-state`、`test:responsive-layout`、前端 production build 已通过。
- [x] 候选 `32108845520` 在 Linux 门禁完成全量 Go test/vet/build、前端 production build、fresh/restart、真实 Web unhealthy 回滚与 healthy 升级；Compatibility `32108845544` 同时成功。
- [x] 自动 Tag `32109534507` 与正式提升 `32109555161` 成功；`v0.5.4@e0b888c` 和三仓正式引用统一 digest=`sha256:4d5dbc6faf23cb15aa859deca62022e7e03dd896a7fc4c77086ac805ddb33cb2`。

# JOB-LOG-LATEST-TAIL-1 / FE-JOB-LOG-LATEST-TAIL-1（2026-08-18，released in v0.5.5）

- [x] 修复长任务完成后 UI 只加载最早 1000 行、因此停在 SteamCMD 自更新日志并隐藏最终成功/失败结论的问题。
- [x] 后端新增 `latest=true` 有界尾页，返回正序日志与精确 `hasEarlier`；原 `after` 增量和 SSE 契约保持兼容。
- [x] 任务页、安装页和右栏活动任务改取最新尾页；运行中任务从尾部 sequence 继续接 SSE，确有更早日志时显示准确提示。
- [x] 存储/HTTP 专项、storage/Web 包、`go vet/build`、前端 responsive/install 状态回归和 production build 已通过；Web 首轮命中的无关存档导入时序用例单独及整包重跑均通过。
- [x] 候选 `32127766494`、Compatibility `32127766392`、自动 Tag `32128518008` 与正式提升 `32128533342` 全部成功；最新日志尾页已随 `v0.5.5@a77fbe6` 发布。

# PANEL-UPDATE-LATEST-RELEASE-API-1（2026-08-18，released in v0.5.5）

- [x] 面板正式更新检查从 GitHub Releases 列表首项推导改为官方 `/releases/latest` 单对象接口；draft、prerelease 和非 SemVer 继续 fail closed，并保留上次成功缓存。
- [x] 更新检查专项锁定精确 URL、响应形态与错误边界；updatecheck/Web、`go vet/build`、升级脚本 Bash/ShellCheck 门禁通过，发布后官方 latest 为 `v0.5.5`。
- [x] 候选升级受控 TLS 夹具同时服务旧 Panel 的列表数组和新候选的 latest 对象，其它 GitHub API 路径返回 404。
- [x] 候选 `32127766494`、自动 Tag `32128518008` 和正式提升 `32128533342` 成功；`v0.5.5` 与三仓 `latest` 使用同一已证明 digest。

# FE-SERVER-RUNTIME-SETTINGS-UX-2（2026-08-18，released in v0.5.5）

- [x] 服务器摘要、总览和移动控制共用人数设置入口与 hook/dialog；摘要按钮不再越界，原生 spinner 改为 44px 像素风 `− / +`，1/100 边界自动禁用。
- [x] 底部拆分“关闭 / 仅保存 / 保存并重启”；运行中重启先确认在线玩家，保存失败零重启，保存成功但重启失败明确保留已保存配置，停止态不偷换为启动。
- [x] 安装时间线 seed/Steam/download 使用重新生成的透明 image2 素材，Steam 认证卡复用同源，摘要存档与农民头像按各自容器裁切；19 项前端回归、production build 和 Browser QA 通过。
- [x] 候选、升级后 production bundle 与正式提升均通过，能力已随 `v0.5.5@a77fbe6` 发布。

# FE-NEW-GAME-MODAL-COMPACT-LAYOUT-2（2026-08-18，released in v0.5.5）

- [x] 修复半屏宽度仍被 1100px 断点直接压成单列的问题；弹窗按 1100/780/560/480/360px 分级使用压缩三栏、两栏、单栏和极窄屏布局。
- [x] 两栏模式把农场选择改为底部四列，单栏模式把联机设置内部改为紧凑双列；移除 `transform:scale()`，container query 与无容器查询回退保持一致。
- [x] 响应式专项、production build、948×805、840×720、769×500 Browser QA 通过；页面级横向溢出和 console warn/error 均为 0。
- [x] 候选 `32127766494`、自动 Tag `32128518008`、正式提升 `32128533342` 和 GitHub Release 成功，能力已随 `v0.5.5@a77fbe6` 发布。

# NEW-GAME-FARM-CAVE-CHOICE-1（2026-08-23，released in v0.5.12）

- [x] 新建存档弹窗在农场类型之后提供原版事件、果蝠、蘑菇三选一，默认原版事件；字段贯通前端类型、请求、后端校验与事务配置。
- [x] 不修改 Junimo 上游；Panel Control `0.3.7` 在严格新建事务保护内把 Junimo 的蘑菇预置转换为精确目标，并回读山洞值、事件 `65` 和蘑菇设施。
- [x] Panel durability verifier 增加 Control status 与主存档 XML 双重证据；三种选择、非法值、错误状态和幂等场景自动回归通过。
- [x] Docker Desktop 真实双写者 E2E 连续创建蝙蝠洞与蘑菇洞并通过双验证，源游戏卷及旧存档哈希不变；Control 真实程序集编译、Linux Go 整包、前端 idempotency/build 与桌面/移动 Browser QA 通过。
- [x] 候选 `32623320406`、自动 annotated tag `32623853636` 与正式提升 `32623863894` 全部成功；`v0.5.12@5141cd54`、GitHub Release 和三仓 `0.5.12/latest` 使用同一已证明 digest。
