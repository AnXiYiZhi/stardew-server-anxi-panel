# v0.4.16 后端发布接手状态（2026-08-14，released）

- `REQUIRED-RUNTIME-STALE-STATUS-1` 已进入 `v0.4.16@5fa04d137bf760d2124b75cc5e3e8e2b44ff4c7c`。最终候选 `31799350642` 通过后端默认并行 test/vet/build、真实 SMAPI/runtime integration、fresh/restart、`v0.4.15` Web unhealthy 回滚与 healthy 升级；Tag `31799876171`、正式提升 `31799891830` 成功。
- 三仓 `0.4.16/latest` 统一 digest=`sha256:5f07910869d6d895e40ecb3954f5905d0cb6abf830e7cf57062bbcf97ca37e0f`。独立正式镜像重启前后 health/database/version/setup 正确；SQLite、初始化、Panel 数据、非目标游戏容器/volume 保持，完整候选故障与资源清理见 `docs/09-image-build.md`。

# REQUIRED-RUNTIME-STALE-STATUS-1 接手记录（2026-08-14，released in v0.4.16）

## 改了什么、影响哪些接口/文件

- 修复 Panel/运行栈已经成功更新后，`.local-container/junimo-update/required-status.json` 仍保留旧 `failed/runtime_update_save_failed` 并污染全栈升级弹窗的问题。
- `required_runtime_update.go` 在公开状态读取及 Panel 启动协调入口重算旧失败；`runtime_update_apply_runner.go` 在普通 apply 成功后立即触发重算。必须同时满足当前 Panel/stack、最新 apply succeeded、实际栈 up-to-date 才写 `succeeded` 并清空旧错误；`manual_action` 和真实当前失败不变。公开 HTTP DTO、phase 枚举及历史 dry-run/apply 文件均不变。
- `runtime_update_apply_test.go` 新增立即收敛、读取时迁移和失败不误清理回归。

## 如何验证、下一步注意事项

- 定向运行：`go test ./internal/games/stardew_junimo -run 'Test(RequiredRuntimeUpdateFailureIsPersistedAndNotRetriedOnSamePanel|SuccessfulRuntimeApplyResolvesStaleRequiredFailure|ReadRequiredRuntimeStatusResolvesHistoricalFailure)$' -count=1`。
- 升级后 fixture 应先放入同 Panel/stack 的旧 required failure，再提供 succeeded apply 与 up-to-date 运行栈；首次读取应返回并持久化 `succeeded`。删除任一证明条件时必须继续返回 `failed`。不要把 `manual_action` 纳入自动清理，也不要物理删除 apply/dry-run 诊断证据。

# SAVE-IMPORT-FIRST-UPLOAD-1 接手记录（2026-08-13，completed，未发布）

## 2026-08-14 发布门禁进展

- 发布已完成：annotated `v0.4.15` 固定在 `d84157dc8a3abc83d13d29c276d6ed332e901ce7`；Compatibility `31725203858` 与 Release workflow `31725256195` 成功，三仓 `0.4.15/latest` 统一 digest=`sha256:b91e3cfd8175305723e0b97feb7c4c202179f2e229aff4f6145fe60b354a5c33`。三个精确镜像 fresh/restart 与四项 Release 资产已验收，任务资源清理为零。

- 代码等价候选 `5fc7e4c` 已分别从 v0.4.14、v0.3.2 的标准 Compose 旧 Panel 走 Web 一键更新；两条链都保留数据库、管理员、实例、Mod、备份、审计、空 saves 与非目标游戏容器/volume，并在升级后 Panel 重启保持终态。
- 升级后的新 Panel 均完成 Nexus 同 key 重放、错误 runtime 409 且 transaction=0、同 token 恢复 `.125`、空 saves 首次上传及 Control 自动解绑。结果为 farmhand total=2/customized=1/bound=0，bootstrap=0、目标目录唯一、preimport 可读、journal completed，Panel 再重启仍保持；这证明 helper host-path 映射和 `${INSTANCE_HOST_DATA_DIR}` Compose bind 两层修复必须同时存在。
- unhealthy 候选从 v0.4.14 更新后以 `health_check_failed` 自动回滚，旧 Panel 与长期数据、备份和非目标 Docker 资源保持。完整 Go/前端/网站/兼容/脚本/Docker integration/Control 编译门禁也已通过，详情见 `docs/09-image-build.md`。
- 官网 docs-only 提交 `13f6af3` 进入远端后，两条本地修复被安全 rebase 为 `967647d` 与 `fd04ff0`。代码内容等价但 OCI revision 不同，因此下一位不得把 `5fc7e4c` 镜像用于 tag；必须从最终干净 main 重建精确候选并重跑 fresh、两条升级后功能和 unhealthy 回滚，再进入 push/tag。

## 改了什么、影响哪些接口/文件

- 修复空实例直接上传被误报“升级 Junimo”的两个原因：`ImportSaveAndStart` 在 journal/token ownership 前按 `.env IMAGE_VERSION=.125` 检查宿主 JunimoServer Mod，缺失或无效时复用 `lifecycleRunner.ensureJunimoServerMod` 从精确 image 原子同步；不启动游戏。明确 tag 不兼容仍是 `junimo_import_unsupported`，同步/复核失败为 `save_import_runtime_prepare_failed`。
- 标准 Compose 升级后复验进一步发现：Panel 内 `instance.DataDir=/data/instances/...` 不能直接作为宿主 Docker daemon 的 bind source。`DriverOptions` 现由 `cmd/panel` 注入 `PANEL_DATA_DIR` 与 `PANEL_HOST_DATA_DIR`，`dockerHostPath` 只允许数据根内路径并映射到宿主根；JunimoServer image 提取、runtime update recovery 提取、SMAPI bundled staging 和 SMAPI ZIP 安装挂载均使用映射后的 source。实例 Compose 的受管 `.local-container` bind 同样不能保留相对路径：新模板使用 `${INSTANCE_HOST_DATA_DIR}`，Prepare/runtime recovery 会原子写入宿主实例路径并迁移旧模板。缺失 host root 的旧同路径部署保持兼容，配置了 host root 却映射不完整/越界则 fail closed。
- `save_import_bootstrap.go` 为没有活动存档的 operation 创建确定性 `AnxiImportBootstrap_<operationId>`。它从指纹稳定的 staged target 克隆到事务私有 source，no-replace 发布，只重命名副本主文件，并把 gameloader 指向副本；Junimo maintenance 因此不会把上传目标当成当前活动档，也不会触发普通零存档自动建档。
- journal 记录 bootstrap 名、全树指纹、no-replace 发布 ownership 和 cleanup completed。取消只有在 ownership 已耐久、pointer 未漂移且上游可证明未生效时删除 pointer/bootstrap/source/target；发布与 ownership 落盘之间中断或同名碰撞时不删除未知目录，直接 recovery。成功则在目标 pointer、finalizer、Control/Junimo 零绑定、GameLoop.Saved 与磁盘稳定之后删除 bootstrap，再写 completed。清理失败留在 recovery，preimport 不删。
- 主要影响 `save_import_{bootstrap,transaction,durable}.go`、`driver.go`、`docker_host_path.go`、`junimo_mod_runtime.go`、`lifecycle.go`、`runtime_update_apply_runner.go`、`smapi_bundled_sync.go`、`installer.go`、`cmd/panel/main.go`、对应测试与 `internal/web/save_import_api.go`。公开上传 JSON、hostHandling、任务阶段和已有存档导入语义不变。

## 如何验证、下一步注意事项

- Go 专项覆盖首次 runtime asset 同步只执行一次、错 tag 不提取、空实例 staging/重试/取消、bootstrap 碰撞零覆盖、maintenance 接受精确 bootstrap、完成清理必须先看到 target pointer，并已通过相关包全量测试。前端稳定错误码测试与 production build 也通过。
- 不要用启动完整 server 代替静态资源物化，否则零存档会产生无关新档。不要放松 bootstrap 的 operationId 派生、no-replace、目标前后指纹或 pointer 检查，也不要在 target durable 之前删除维护世界。
- `eaae88f` 候选已经用真实有效 Stardew 存档完成空实例 Web 上传，bootstrap 实际加载并清理，最终只剩目标存档，runtime 提取失败同 token 可重试，自动解绑/durable save/Panel 重启均通过。仍需补复制/发布中断、Panel/Control 精确中断、cleanup 权限失败，并从 v0.4.14/v0.3.2 一键升级后的空实例再次验收。矩阵见 `docs/09-image-build.md`。
- 注意 `eaae88f` 独立夹具把 host data path 同名挂进 Panel，不能作为标准 Compose path mapping 的最终证据。v0.4.14 Web 升级本身、数据/非目标资源保留与重启已经通过；升级后首次上传在正确镜像重试时稳定返回 runtime-prepare 409，且二级容器没有生成宿主 Junimo manifest，从而定位 helper bind 的 namespace 缺口。`1961b40` 修复 helper 后任务已进入 `backup_created`，但 maintenance server 因旧实例 Compose 的相对 `.local-container` bind 被解析为 Panel `/data` 路径而启动失败；新模板、旧实例原子迁移及 idempotency 单测已补齐。必须从包含两层修复的最终 SHA 重建候选，并用标准 Compose（仅 `/data` container target）重跑同一升级后上传链。

# NEXUS-EXT-IDEMPOTENCY-1 接手记录（2026-08-13，completed，未发布）

## 改了什么、影响哪些接口/文件

- `POST /api/instances/:id/mods/remote/install` 可选读取 1–128 字节可见 ASCII `Idempotency-Key`。同 key 重放返回已有 `202 {jobId,deduped:true}`；不带 key 的旧扩展继续按原行为创建任务。
- migration `013_job_idempotency_key.sql` 为 jobs 增加私有列和 `type/target_type/target_id/key` 唯一索引。`storage.CreateIdempotentJob` 在 SQLite 事务中查找/创建 owner，唯一冲突回读已有 Job；`jobs.Manager` 只有真正创建者发布事件并启动 runner。
- 主要文件：`backend/migrations/013_job_idempotency_key.sql`、`internal/storage/jobs.go`、`internal/jobs/{types,manager}.go`、`internal/web/lifecycle_handlers.go` 及对应测试。公开 Job DTO、远程安装 JSON body、下载/解压和 Mod 导入逻辑未改变。

## 如何验证、下一步注意事项

- `go test ./internal/storage ./internal/jobs ./internal/web -run 'Idempotent|Idempotency|RemoteInstall' -count=1` 覆盖 12 路并发、终态复用、不同 key、单 runner、实例状态变化后的 HTTP 复用和非法 key；并已通过 `go test ./... -count=1`、`go vet ./...` 与 `go build ./...`。
- key 是一次用户动作身份，不是 `modId/fileId` 的永久身份。扩展必须在响应不确定时复用，在用户明确新开安装时轮换；后端不得按完整 CDN URL 建键或把临时 query/token 写入 jobs、日志和审计。
- `eaae88f` 已用任务专属 Chromium profile 加载真实 unpacked 0.1.3：Panel bridge 同源注册后 20 路同 capture 得到 1 owner/19 shared，Panel 只有 1 个 job；关闭/重开 Chromium 后 requestId/job 持久化，popup/Panel console error/warn 为 0。独立候选服务端在调用方不观察首次响应后重启活动 Panel，20 路及终态重放都复用原 failed job、runner 只启动 1 次；不同 key 只创建第二个受控失败 job，SQLite total/distinct=2/2，失败未留下 Mod，key 未进入 audit/log。测试使用合成账号和受控 Nexus-CDN 形态 URL，不依赖生产 Nexus 登录。
- 正式发布仍需在 v0.4.14/v0.3.2 Web 升级得到的新 Panel 上复验该链，并完成 tag 前/后镜像门禁；不要把受控失败 URL 写成真实 Nexus ZIP 成功下载。

# SAVE-IMPORT-AUTO-UNCLAIM-1 接手记录（2026-08-13，completed，未发布）

## 改了什么、影响哪些接口/文件

- 不修改 Junimo 上游、不新增前端选择。swap_to_player 在已有 swap-host-to finalizer 完成后，后台默认把 unbind-all-farmhands 动作附加到同一次耐久 save-now；virtual_host_takeover/as-is 维持原行为。
- Control 0.3.2 新增预保存动作契约、前置校验、全部 farmhandData.userID 清空、结果计数和 pending journal 恢复。动作要求精确目标存档、服务器主机、零在线 farmhand 和可读角色数据；重启恢复只重复同一 commandId，清空本身幂等。
- Go durable import 新增动作 payload、Control 结果身份/计数校验和 Junimo diagnostics 双证据；journal 记录 farmhandUnbindVerified、farmhandCount、customizedFarmhandCount。维护 runtime 同时检查当前内嵌 Control DLL 与 options.json 版本，旧/错 DLL 使用 save_import_maintenance_control_mismatch 停止推进。
- 候选真实 Web 首次上传发现 command result 生命周期竞争：通用历史同步可能在 durable gate 前导入并删除 Control 结果，而公开历史脱敏会丢弃动作身份/计数。`command_results.go` 新增 `SaveImportCommandResultProtected`/`DurableCommandResultProtected`，按未完成 journal 的精确 `DurableSaveCommandID` 保留原 JSON；journal 不可读时 fail closed，只有 `completed` 后才允许 `control_commands.go` 归档删除。没有扩大数据库公开详情白名单。
- 主要文件为 save_import_durable.go、save_import_evidence.go、save_import_maintenance.go、save_import_transaction.go、ControlContract.cs、DeferredCommandOutcomes.cs、ModEntry.cs、两份 Control manifest/DLL 与 runtime_stack_manifest.json。公开 Web DTO、审计字段和前端文件没有变化。

## 如何验证、下一步注意事项

- Control 纯契约覆盖动作解析、错档、在线玩家、恢复、Saved 后仍绑定和成功计数；真实 game-data 编译 0 errors。Linux 容器内 go test ./... -count=1、go vet ./...、三项 cmd build 均通过。
- 新增专项覆盖未完成导入保护精确结果、无关 commandId 不受保护、`completed` 后释放，以及 Web 同步器在保护期不入库/不删文件、完成后正常归档。`b15fa42` 旧候选的真实链已复现并证明 fail-closed，包含修复的最终候选必须重新跑完整首次上传、Panel 重启恢复和资源清理，旧候选不可用于 tag。
- `eaae88f` 候选已从重新克隆的 1,959,640,718 B 只读 game-data 基线跑过完整 Web 首次上传：无宿主 Junimo Mod、空 saves/无 pointer，受控 runtime 提取失败返回稳定 409 且 transaction=0，同 token 恢复后 job succeeded；bootstrap created/cleaned、目标目录唯一、preimport 可读、Control total=2/customized=1 全部解绑、durable save 完成。Panel 重启后 journal/server 保持，结果文件已归档且数据库 succeeded/ok，完整测试 platformId 不在 journal。Compose 容器/网络、两个运行期 volume、Panel 与 data 均精确清理/重置；只读源未修改。
- Docker Desktop project anxi-unbind-e2e-20260813-r1 使用任务专属端口、bind、game-data/steam-session 卷，从只读历史隔离夹具克隆 2 个 farmhand（1 customized、1 bound）。Control 0.3.2 的单个 save-now succeeded 后，Control details、Junimo diagnostics 和主 XML 均为 total=2/customized=1/bound=0；主文件 SHA-256 发生变化，server 重启后仍为 0。项目容器/网络/卷为零，克隆目录移入回收站，源夹具未改。
- 后续维护不得把动作移到 XML 离线改写，也不得在 Panel 看不到精确 GameLoop.Saved 或 diagnostics 绑定计数时放行 completed。若未来界面移除 SteamID，需要先另行定义 Panel 生成一次性平台 ID 的上游 swap 契约；本次没有实现该路线。
- 下一次正式版本必须按 docs/09-image-build.md 以最终 commit 重建 Control/Panel 候选，重新核对 DLL SHA、真实上传完整事务、故障注入、上一正式版/最低支持版一键升级、回滚和升级后再验收；当前实现未创建 tag、镜像或 Release。

# STARTUP-NEWGAME-DURABILITY-1 接手记录（2026-08-13，released in v0.4.14）

## 交付状态与下一步

- `3cdf43c5a2b3055add7ed5a6720d97e24794073c` 完成 Control runtime gate、安装诊断、lifecycle handoff、原子 new-game owner/幂等、双 writer、完整定制/磁盘耐久、save-now journal、rollback journal、mutation guard 与 bootstrap 保持停服。Control 已升为 0.3.1，内嵌 DLL/runtime manifest 摘要一致。
- Go 全量/vet/build、真实 Control 编译、startup+HTTP writer Docker E2E 已通过；v0.4.11/v0.3.2 真实 Web 升级、unhealthy 回滚、621 conversion 与升级后诊断/UI 也已通过，精确事务与数据完整性证据见 `docs/09-image-build.md`。
- 下一位只需继续发布收口：提交本轮证据后以最终 SHA 重建候选，重跑精确身份/fresh/关键 Web 门禁；工作树干净同步后才 tag。tag 后核对 Release workflow、三仓 digest/latest、run.sh swappiness 资产，再进入生产。不要恢复宿主重启自动开服语义。
- 发布阻断复跑补丁：Linux Compatibility matrix 的 12 contender owner claim 偶发读到 winner 发布中的空目录。`new_game_transaction_owner.go` 增加进程内 claim mutex，只消除同进程 loser 的可见性窗口；Linux `renameat2(RENAME_NOREPLACE)` / Windows no-replace 仍是跨进程唯一原子边界。两平台专项各 `-count=100` 及全量 test/vet/build通过，未知 owner 现场的 fail-closed 语义未放宽。
- `v0.4.12` 固定 tag 的 Release workflow `31679615132` 在任何 registry login/build/push/Release 之前被 `repair-junimo-0.3.5.sh` 的 SC2317 阻断；没有半发布镜像或 Release。脚本仅为 ERR/INT/TERM trap 间接调用的恢复函数补充局部 `SC2317,SC2329` 说明，ShellCheck 0.10.0/0.11.0 对 workflow 全输入通过。不得移动 `v0.4.12`；下一正式版本为 `v0.4.13`，仍需重建精确 revision 候选、关键 Web 回滚/升级、远端 Compatibility、tag 后三仓和生产收口。
- `v0.4.13` 的 release gates 通过，Buildx push 才因 ACR 拒绝默认 empty attestation manifest 失败；Docker Hub `0.4.13/latest` 已部分更新，GHCR/ACR 和 GitHub Release 未更新。不要移动或重用该 tag；workflow 已统一禁用 provenance/SBOM attestation，`v0.4.14` 成功后必须用三仓精确版和 latest 的相同 digest 覆盖这一部分发布状态。
- 正式 `v0.4.14` tag=`a70efc98feecd6b2db803435b59b0f31d1439cf3`；Compatibility `31682006066` 与 Release workflow `31682847388` 成功，GitHub Release 非 draft/prerelease。Docker Hub/GHCR/ACR 的精确版与 latest 统一 digest `sha256:5b58ad998da14726b655f4a965c0e3f74ae7839fe615b0f59dd8af1ee16a8ebd`，三个精确镜像分别完成首次启动/重启；四项 Release asset 与 tag 源字节一致。生产 `114.55.142.107:22` 的 `cz/root` 均认证失败，等待正确用户名后才可同步，游戏必须保持关闭直到用户手动启动。

# PANEL-UPDATE-GRAPHICAL-COMPOSE-1 接手记录（2026-08-12，completed，未发布）

## 改了什么、影响哪些接口/文件

- `updater.DockerCLI.ResolveComposeDeployment` 在完成容器/Compose/service/image/data mount 反查后，使用隔离 runner 显式 `--env-file <install>/.env` 并把进程级 `PANEL_IMAGE` 设置为固定不可拉取探针；目标 service 的 `config --images` 必须精确跟随探针。缺 `.env`/坏 env 返回 `deployment_env_invalid`，写死或使用其它 image 变量返回 `compose_image_unmanaged`。
- `DetectCapability` 只对“身份已经唯一解析、仅持久镜像契约不合格”的部署尝试既有安全标准化；service label 不一致、Compose 反查失败仍 fail closed。安全图形化 Compose 返回现有 `conversionRequired=true`，前端已有说明、按钮和确认流程可直接复用；apply spec 会启动 `panel-updater convert`，无需新增 API 或前端状态。
- `legacyConversionCapability` 取消与脚本能力不一致的“挂载总数必须为 2”限制，改为核心数据/Socket 唯一、额外目标唯一且仅 bind/volume；脚本仍在切换前验证 RW、propagation、name、tmpfs/device/network。这样 NAS 示例中未覆盖镜像声明 `/data` 而产生的匿名 volume 会按 external volume 保留。
- `updateEnvImage` 对已证明 Compose 消费 `PANEL_IMAGE`、但 `.env` 尚无该键的部署原子追加目标值；已有键仍原位更新。影响文件：`internal/updater/{types,deployment,docker_cli,service,legacy_conversion,apply_helper}.go` 及 `*_test.go`。

## 如何验证、下一步注意事项

- Go 专项覆盖：标准变量契约成功、显式 env 不可用、硬编码 image、完整 labels 的安全自动转换、privileged 拒绝、service label 漂移不转换、匿名 `/data` volume 保留、conversion helper 参数和 env 缺键追加。运行 `go test ./internal/updater -count=1`。
- Docker Desktop 独立 DinD 已创建完整 Compose labels、写死 image、无 `.env`、宿主绝对路径数据 bind 和额外匿名 `/data` volume 的 `0.4.10` 夹具，并从真实 Web API 完成 check、dry-run、`confirmFullStack` apply、预期断线重连和正式 `0.4.11` 终态恢复。结果为 `succeeded`；生成 `.env + image: ${PANEL_IMAGE}`，新 project=`anxi-panel-anxi-panel-graphical-e2e`，游戏容器 ID 与匿名 volume 精确不变，旧 Panel 停止保留，任务 DinD/container/network/volume/镜像已按唯一 owner 精确清理。
- 前两轮隔离环境的非受控转换尝试返回 `failed_rolled_back`，旧 `0.4.10` 都自动恢复并重新 healthy；同一迁移脚本在恢复现场成功，精确目标镜像核对 digest 后，第三个全新 DinD 的完整 Web 请求 79.3 秒闭环成功。非受控失败不能归因成产品故障注入证据，但证明这两次没有把原部署留在半切换状态；下一正式版本仍须补受控 unhealthy/版本不符和 helper 中断矩阵。
- 不要把任意 Compose 解析错误映射为 conversion：只有 resolver 已返回唯一 service 的 `composeUpdateContractError` 可以进入标准化。不要在主 Panel 进程直接编辑宿主 Compose；所有备份、生成、rename、重建和恢复继续由独立 helper 与 `migrate-fnos.sh` 完成。

# INSTALL-FIRST-RUN-CONSISTENCY-1 接手记录（2026-08-11，released in v0.4.11）

## 改了什么、影响哪些接口/文件

- `backend/migrations/012_exclusive_stardew_install_jobs.sql` 清理历史重复活动安装 owner 并创建 active install partial unique index；`storage/jobs.go` 新增 `ActiveJobExistsError` 与事务型 `CreateExclusiveJob`，`jobs.Spec.Exclusive` 由 Stardew `Driver.Install` 启用。`install_handlers.go` 将冲突映射为 HTTP 409 `install_in_progress`，details 只包含已有 `jobId`，没有凭据。
- `storage/instances.go` 新增 `UpdateInstanceStateForActiveJob`；`driver.updatePhase` 对有 job ID 的生产 store 做 active target 条件更新，终态任务的迟到写入返回 `ErrJobNotActive` 并只记 debug。`jobs.Manager.RecoverInterruptedJobs` 在批量 fail 前由仍 active 的 install owner 写中断状态。
- 新文件 `smapi_bundled_sync.go` 用当前 server image 的 `/bin/sh` 只读挂载 current game-data volume，把实际 `/data/game/Mods` 复制到 host sibling staging，做 manifest/UniqueID/版本/必需组件/symlink/类型/大小校验与全树 SHA-256；相同树 no-op，变化树原子替换 `.local-container/mods/smapi`。发布恢复识别仅限 Panel-owned `.smapi-sync-*` / `.smapi-backup-*`：清理中断 staging，destination 缺失时恢复最近有效 backup，避免进程死在 old→backup 后留下空 managed namespace。调用点是 `installer.completeInstall`、首次 `lifecycle.doStart` 事务创建前、SMAPI apply switch 后及 rollback 切回旧卷后；安装器和最终 volume 校验都把 ConsoleCommands/SaveBackup manifest 视为必需文件，旧状态只有 SMAPI 可执行文件但缺支持 Mod 时不会被误判完整。
- `runtime_farm_catalog.go` 现在把 managed smapi 目录内所有有效 manifest 加入 ExpectedFingerprint，仍保留当前两个官方支持 Mod 的顶层兼容 fallback/隔离规则。失败 phase/code 为 `smapi_bundled_sync_failed`。
- `backend/internal/docker/tty_run_unix.go` 为每次 Steam auth Compose one-off 指定随机唯一容器名；context 取消/超时时只按该精确名字强制删除，并要求连续 3 秒确认缺席。不能在第一次 list 为空时返回，因为前台 `docker compose run --rm` 被杀后 daemon 仍可能晚到创建 `Created` 容器。此修复不改变正常安装、认证 volume 或 Windows Engine API 路径。

## 如何验证

- `go test ./internal/storage`：exclusive 并发 12 请求、历史重复 owner 迁移和 partial index。
- `go test ./internal/jobs`：Manager 返回现有 owner、取消后可再次创建、启动恢复顺序。
- `go test ./internal/web -run TestWriteActiveInstallConflict`：409 code/message/details.jobId。
- `go test ./internal/games/stardew_junimo`：物化、幂等、坏 staging 保留旧目录、中断 backup/stage 恢复、未来 bundled manifest 指纹及同步失败早于 new-game transaction；全包已通过。最终后端 `go test ./... -count=1`（65.8 秒）、vet、build 全绿，前端 13 项状态测试和 production build 通过。
- 真实 Docker 专项以精确 `dockerproxy.net/sdvd/server:1.5.0-preview.125` 创建唯一临时 game-data volume，第一次 sync 发布两个 support manifest、第二次 tree digest no-op；测试后 `anxi-smapi-sync-*` 容器/卷均为 0。最终候选又完成 SMAPI apply/rollback、中断恢复、两条真实 Web 升级及升级后首次建档，不能只以单元测试替代这些发布证据。
- `TestFreshInstall125ReachesSteamLoginOptIn` 使用真实 `.125/auth .2`、空凭据与 QR 登录复现取消泄漏。第一版只在首次观察不到容器时返回，发布外层随后抓到 daemon 晚到的 `Created` one-off 和两个无法删除的案例卷；测试本身也改为 job 终态后连续 3 秒确认缺席。最终 tag 源码真实测试 9.64 秒通过，外层案例 container/volume 为 0；Linux `internal/docker` 可控晚到创建单测和全量门禁均通过。
- 稳定缺席修复后的 Windows 后端全量 59 秒、vet/build 通过；Linux 全量空缓存结构化复跑 77.6 秒、vet/build 通过，任务容器和缓存卷均清理。一次普通文本 Linux 全量的 Web 包失败输出被正常 HTTP 日志截断，随后定向 Web 包与结构化全量均通过；该次无断言非零不算通过证据，执行方式已记入错题本。
- `TestRealNewGameMaterializesSMAPIModsBeforeFirstSaveOptIn` 只读克隆历史测试所有的完整 game-data 到唯一任务卷，以空 saves bind/空 Steam 凭据运行真实 Junimo/SMAPI；两轮分别 71.78/60.04 秒创建唯一可解析活动存档。job log 的物化 sequence 9 早于事务快照 sequence 10，两个 support manifest 非空且终态无 `.smapi-*` owned artifact，第二轮 Stop 后容器归零；源卷未被写入。最终候选在由 v0.4.10 升级得到的 Panel Web 上再以 1.96 GiB game-data 创建 `Tag Release Gate` 并通过 Panel restart、存档可读和 server/auth ID 保持。

## 正式发布结果

- tag `v0.4.11` 固定指向 `ef2580d2e58b170b5e5aa0079496f969228dd3f6`；compatibility `31521174829` 与 Release workflow `31521478699` 成功。三仓 `0.4.11/latest` 统一 index digest=`sha256:7c2fea3496ac1ec4afa2ae50f1087f469151e46b18a9c202bd7d4e70f16bb86e`、amd64 manifest=`sha256:f916037c571eac6962a4f6448e08c425e8e0b8956679835808d4e2c10f78d02c`，三个精确镜像均通过首次启动/重启 health、SQLite、版本与 fresh setup 冒烟。Release 资产、事务时序和清理记录见 `docs/09-image-build.md`。

## 下一步注意事项

- 任何新安装入口都必须走 `Exclusive:true`，不得先 list 再无约束 CreateJob；任何异步实例阶段写入必须传真实 job ID。不要把 409 转成通用 500，也不要自动取消旧安装后重开。
- server image 若改变 `/data/game/Mods` 或 entrypoint `init_mods` 契约，必须同时更新 sync helper、真实 Docker fixture 和指纹测试。目标命名空间只允许 managed `smapi`，不能扩展删除范围到玩家顶层 Mods。
- 迁移新增数据库约束，后续发版属于受数据库迁移影响的版本，必须执行上一正式版和最老受支持代表版本的一键升级、数据保持、重启恢复与回滚验证。
- TTY 取消清理必须保留随机精确名字、20 秒总边界和连续 3 秒缺席窗口；禁止退回按 project、service 或 volume 宽泛删除，也不能只杀 Docker CLI、看到一次空结果或假定 `--rm` 已在 daemon 生效。对应真实测试必须在 job 终态后另做稳定缺席检查，并让外层确认测试 volume 可删除。

# RELEASE-V0.4.10-BACKEND-1 接手记录（2026-08-09，released）

- tag `v0.4.10` 已从同步且干净的 `main` 提交 `7d9d0e267d942952701bc14ac19d032951d2dfd7` 发布；上一正式版 `v0.4.9`、代表老版本 `v0.3.2`。后端发布内容是 steam-auth 验收由 Docker health 切换为 running + digest + 可解析接口；联机登录能力降级为 warning，接口或 digest 故障仍回滚。
- `.github/workflows/release.yml` 已加入 `TestRuntimeUpdateAuthAcceptanceDoesNotWaitForDockerHealth`，确保正式 tag 在真实 Docker unhealthy/logged-out fixture 上复验。
- 候选构建前最终差异通过后端全量 test/vet/build、完整 runtime Docker integration、真实官方 `.125 / auth .2` 无凭据 503 offline 合约和 unhealthy/logged-out 专项；公网兼容矩阵在有界网络恢复后通过。
- 正式 Web E2E 使用受控 HTTPS Release/registry：v0.4.9 的 Panel 自更新 unhealthy 候选事务 `6453365330c7` 在真实观察到目标 running+unhealthy 后以 `failed_rolled_back / health_check_failed` 收口；同一受控 registry 镜像引用切回健康候选后的事务 `33d98c68a74f` 成功，Git tag 未移动；v0.3.2 历史空 body 事务 `d6a33fe33ce8` 也成功。两条链均验证 SQLite/live backup、初始化、用户、实例、审计、存档/Mod/备份哨兵、非目标 game container/volume 和 Panel restart。
- compatibility workflow `31321583191` 与 Release workflow `31325589153` 成功。三仓 `0.4.10/latest` 统一 index digest 为 `sha256:c37ad8e8d1498f377900b8a82e2ad1de761df23a06f1cb298ae349a362b111df`，三个精确版本均通过回拉 health/version/restart 冒烟；完整资产摘要、耗时、升级后前端交叉验证与清理记录见 `docs/09-image-build.md`。

# RUNTIME-UPDATE-REPAIR-CATALOG-3 接手记录（2026-08-09，completed，v0.4.9 released）

## 改了什么、影响文件、验证与下一步

- `runtime_update_repair.go` 新增公开 `RuntimeUpdateRepairPlan` 和唯一 detector；Web GET 通过 `junimo_update_handlers.go` 返回该计划，POST 在实例锁内复用 detector 后才执行。闭集修复含 `rollback_failed`、`legacy_candidates`、`safe_retry`；手工导出含状态/清单/材料异常、自定义/歧义配置和次数耗尽；非终态/不安全矩阵只等待。
- 已知配置修复 helper 泛化为同一持久化 repair runner。safe retry 不重放旧步骤，先核对旧版与同一目标，再做完整 diagnostics/dry-run、保存/备份和新 apply；最多三次，目标失败仍走普通自动回滚。影响对应 apply/Web 单测，数据库、存档和实例格式不变。
- 全量 `go test ./... -count=1`、vet、build、Docker integration、脚本语法/功能/ShellCheck、前端状态/build、Docker Desktop 候选 health/version/restart/API 与浏览器点击均通过。具体镜像和故障矩阵见 `docs/09-image-build.md`。
- `support_bundle.go` 新增 `junimo-update.json`：公开 inspection、repair plan 和可读取的 apply status 先序列化、再统一脱敏；没有遍历或读取 recovery manifest/目录/原文件。`support_bundle_test.go` 使用 apply 状态伪密码和既有存档/事务/恢复诱饵验证全部不泄露。
- 后续增加新错误时，必须只扩展 detector 与受限执行器并补材料漂移、重启、部分成功、回滚和 Docker 故障注入测试；不要在 handler、页面或脚本中复制判断，也不要把该目录用于 SMAPI/Panel updater。
- 正式发布矩阵以 `v0.4.8` 为上一正式版、以 runtime manifest 支持下限 `v0.3.2` 为受影响最老代表版本；必须从 Web API 完整走检查、dry-run、管理员确认、apply、断线重连、终态恢复，并在升级得到的新 Panel 上复验 repair plan 与支持包。ShellCheck 0.11 的 trap 间接调用规则已通过 `SC2317/SC2329` 双版本声明兼容。
- `v0.4.9` 已从同步且干净的 `main` 提交 `6f3e4a28f6c5f983f0f891079fb0b7478bd5c1a9` 发布。官方 `v0.4.8` 和支持下限 `v0.3.2` 到精确候选的升级、Web 成功链、unhealthy 自动回滚、升级后重启/repair plan/支持包、数据与非目标容器保护均通过；Release/compatibility workflow `31299979401/31298881696` 成功。
- Docker Hub、阿里云 ACR、GHCR 的 `0.4.9/latest` 六个引用统一为 index digest `sha256:e8fa5386b17d778612365bfa419b5ad5e2f447bb557856580efe262fea6f505f`，三个精确镜像分别通过隔离 health/version/setup 冒烟。发布后任务容器、volume、network 和端口已精确清理；完整时序、资产摘要及故障矩阵见 `docs/09-image-build.md`。

# RUNTIME-UPDATE-DIAGNOSE-REPAIR-2 接手记录（2026-08-09，completed，未发布）

## 改了什么、影响与验证

- repair API 现在接受精确 `rollback_failed` 或 `repairable/legacy_candidates`，并在同一个实例 job 内完成诊断、受限修复、旧版验收、完整 dry-run、升级前保存/整档备份和新 apply；目标未验收成功就不返回成功。
- 新状态字段/阶段为 `resuming_upgrade`、`repairSourceApplyId`、`resumeAfterRepair`。恢复逻辑覆盖旧恢复目录仍在、旧目录已清、新 apply 尚无 manifest、以及新 manifest 已写但 mutation 未开始四个窗口；mutation intent 发生后回到原 write-ahead rollback 规则。
- detector 闭集为事务 status/manifest/material SHA 与历史可信候选配置；修复后 apply preflight 使用运行容器 image ID、配置 tag 和 Control 文件/运行版本生成 change plan。自定义镜像、未知 Compose、损坏状态或摘要漂移继续拒绝。
- 核心测试覆盖 partial rollback 修复后升级成功、修复后目标再失败并安全回滚、历史配置入口、旧恢复目录已清/新 retry manifest 无 mutation 两个重启续跑窗口和诊断 checks 保留。全量 Go/vet/build、脚本、兼容矩阵、Docker integration 与精确候选镜像 health/version/API/内置脚本均已通过；候选 metadata、SHA 和 UI 串联证据见 `docs/09-image-build.md`。仍不可把本入口用于 SMAPI、Panel 自更新或 game-data staging，也不能在缺少无用户数据的真实 Junimo 故障夹具时提前发布。

# RUNTIME-UPDATE-WAL-REPAIR-1 接手记录（2026-08-08，completed，未发布）

## 改了什么

- required runtime recovery manifest 升为 schema 3；stop、Control、snapshot、Junimo、config、auth/server 等每个 mutation 都先写 intent。崩溃发生在 Docker 调用返回与完成标志落盘之间时，恢复仍会处理“可能已发生”的动作。
- pre-mutation 无 manifest 重启直接终止为安全回滚；成功和回滚成功先写 terminal status，再清 snapshot/旧 image/recovery。恢复版本信任绑定 status 中该事务的 Target/Selected，不再绑定新 Panel 当前内置推荐版本。
- `runtime_update_repair.go` 新增最多三次的幂等 rollback retry；schema 3 对 env/Compose/Control 备份做 SHA-256 和 regular-file/symlink 校验。认证卷 helper 使用原 immutable image ID，Junimo Mod restore 可从部分成功点重复执行。
- Web 新增严格管理员 `POST /api/instances/:id/junimo-update/repair`；`deploy/repair-junimo-upgrade.sh` 只走 API，已接入 Dockerfile、release asset、脚本功能测试和 ShellCheck。

## 影响文件、验证与下一步

- 核心文件：`runtime_update_apply.go`、`runtime_update_apply_runner.go`、`runtime_update_repair.go`、`runtime_update_rollback.go`、`junimo_mod_runtime.go`、`junimo_update_handlers.go` 及对应测试。JSON 只增加 `repairAttempts`；数据库和实例部署格式不变。
- 自动测试覆盖部分回滚第二次成功、材料摘要漂移零 Docker mutation、schema 3 Control intent 重启、备份阶段无 manifest 重启、跨 Panel 推荐版本恢复和快照 create crash window 所有权。完整 Go、Docker 与脚本证据见 `docs/09-image-build.md`。
- 后续不要重新把 current built-in manifest 当作旧事务恢复的唯一信任源；事务必须自洽绑定其持久化 status。新增 mutation 必须先加 intent，再加幂等 rollback 和中断测试。
- 当前一键入口仅覆盖 required Junimo runtime。SMAPI staging 和 Panel helper 的极端 rollback_failed 统一入口仍在 `UPGRADE-RECOVERY-UNIFICATION-1`，不可由本接口代替。正式发版仍需完整旧正式版升级矩阵，本次未 tag。

# RUNTIME-UPDATE-PRESERVE-AUTH-1 接手记录（2026-08-08，completed，未发布）

## 改了什么

- required runtime apply 不再把“版本对推荐”误解为“两个容器都必须重建”。恢复清单 schema 2 按当前/目标 tag 和 immutable image ID 分别记录 server/auth change plan；旧 schema 1 继续按全量变更保守处理。
- auth 未变化且原来运行时：只停止 server，保留 auth 容器与 steam-session，不发起 Steam readiness 请求；通过 image ID、容器状态和 health 验收，再原地更新 CPU shares=256。auth 未变化但原来停止时使用 `compose up --no-recreate` 启动现有容器做验收。server 未变化时跳过 JunimoServer Mod 提取/替换，但仍重建 server 以加载新 Control。
- auth 真正变化时原快照、重建、readiness 和回滚边界保持；auth 等待由 90 秒增至 10 分钟。回滚只停止/恢复实际变更的 auth，Control-only 失败不会因同一 Steam 网络故障再次把回滚升级成 `rollback_failed`。

## 影响、验证与下一步

- 主要文件：`runtime_update_apply{,_runner,_test}.go`、`runtime_update_rollback.go`、`driver.go`、`internal/docker/runtime_apply{,_test}.go` 和真实 required runtime integration。公开 HTTP/JSON、数据库、manifest 推荐版本均不变；新恢复 manifest 字段只位于实例私有恢复目录。
- 单元测试覆盖 Control-only + auth readiness 注入失败、auth 容器不 stop/up、无认证卷快照、server 独立重启、原地资源权重和 10 分钟默认预算；既有成对升级/回滚测试保持通过，最终全量 `go test ./... -count=1`、vet、build 全绿。
- Docker Desktop 真机从只读 `.125/auth .2/Control 0.2.0` 测试源复制到唯一临时 project、game-data 和 steam-session，stopped/running 两条链均通过。运行态 auth ID 不变，Control 0.3.0、server/auth CPU shares 768/256、最终健康和原状态均通过；首轮测试发现保留容器未应用新 shares，加入原地 `docker update` 后同用例完整重跑通过。
- 后续发布前仍须按 `docs/09-image-build.md` 构建精确候选镜像并跑完整 Web 一键更新/回滚和正式发布门禁；本次未创建 tag、未更新 latest、未发布镜像。

# v0.4.8 玩家 Mod 后端发布交接（2026-08-07，released）

- 玩家 Mod 采集、比较接口、列表轻量 CJB 标记、三类内置运行组件过滤和 Control `0.3.0` 已随 `v0.4.8` 发布。tag commit 为 `0c5e2c434a92e8c9a69f839b39f86508cccf9a77`；release/compatibility workflow `31117969497/31117949897` 最终成功。
- 三仓 `0.4.8/latest` digest 统一为 `sha256:5381009b807ad2c632075332e3538297b5069eff2f2b1b133ff7fffd2ac38f90`，三个精确镜像已分别通过 `/health` 与 `/api/version`。完整升级、回滚、数据保护与发布故障证据见 `docs/09-image-build.md`。
- 接口、sidecar schema 与限制不变；尚需真机补 PC 原版、官方 CJB、Android/iOS、Android 实验性 SMAPI和真实多玩家并发。清单始终是可篡改的客户端自报，不能用于自动管理。

# PLAYER-MOD-BUILTIN-FILTER-1 接手记录（2026-08-06，completed，v0.4.8 released）

## 改了什么、影响与验证

- `player_mods.go` 的服务器基线和客户端比较 map 现在统一按 UniqueID 忽略 `Pathoschild.SMAPI`、`JunimoHost.Server`、`AnXiYiZhi.StardewAnxiPanel.Control`。这三类面板自带运行组件不再出现在 `serverContext.loadedMods`、items 或 summary；旧虚拟 SMAPI 版本比较已取消，双方 `apiVersion` 元数据仍保留。
- comparison item 排序改为 `client_only → missing_on_client → version_mismatch → match`。普通第三方 `server_only` 规则不变：未上报不生成缺失，双方上报时仍可作服务器专用信息。
- 影响 `player_mods.go`、`player_mods_test.go` 与 Web API 契约测试；定向测试及最终 `go test ./...`、`go vet ./...`、`go build ./...` 通过。Control Mod、sidecar schema、玩家连接/认证和所有管理通道均未修改。
- 下一步若再增加面板运行组件，应把其官方 UniqueID 同时加入后端 allowlist 与前端兼容过滤，并补大小写回归；不要按模糊名称过滤普通玩家 Mod。

# PLAYER-MOD-CJB-LIST-1 接手记录（2026-08-06，completed，v0.4.8 released）

## 改了什么与影响

- `PlayerInfo` 新增可选 `modRiskFlags`。`persistPlayerRoster` 的统一收尾一次读取 `player-mod-contexts.json`，复用详情接口的文件上限和单条规范化校验，按联机 ID 只补 `cjb` 标记；完整 Mod 数组仍不进入 `GET /players`。
- `player_mods.go` 把 sidecar 文件读取与单玩家条目规范化拆为可复用 helper，详情接口行为、`mods:null`、比较结果和错误边界不变。stale 记录可继续带最后一次自报检测标记，但 UI 必须把它当自报提示，不能当处罚依据。
- 影响 `players.go`、`player_mods.go`、`players_test.go` 与玩家 API 契约；没有修改 Control Mod、Junimo/SMAPI 上游、玩家连接、认证、踢出或封禁链路。

## 如何验证与下一步

- 定向 `go test ./internal/games/stardew_junimo ./internal/web` 通过；测试断言玩家响应只含 `modRiskFlags:["cjb"]`，不泄漏 CJB 名称或 `mods` 字段。
- 后续若增加其它风险类别，仍须保持小型 allowlist 标记，不能把客户端完整清单复制到玩家轮询响应，也不能让标记参与自动管理。

# PLAYER-MOD-COMPAT-1 第三阶段接手记录（2026-08-06，PC+SMAPI 真机部分通过，其余受限，v0.4.8 released）

## 改了什么

- 新增 `PlayerModContextLifecycle`，Control 的 `PeerContextReceived/PeerConnected/PeerDisconnected` 与 pending 超时使用同一组可测试状态转换。实际行为保持只读：连接写 pending、完整 SMAPI context 写 reported、10 秒无 context 写 unavailable、断线和启动恢复写 stale，重连会先清掉旧 Mod 数组再等待新报告。
- C# 契约补齐乱序事件、原版/移动 unavailable、断线重连、进程重启、多玩家隔离、两种 CJB、重复/超长/超量字段；Go 补齐读取端二次限制、server_only 信息展示/缺失排除和 Mod peer handler 不得进入 kick/ban/命令通道的回归。
- Web 增加真实 loopback TCP 测试，以真实 handler、SQLite、临时实例文件验证四类 context、两种 CJB 和未知 ID 隔离；Docker ps 仅用测试替身。接口契约、Control 版本 `0.3.0` 和 sidecar schema 均未改变。
- 本机随后用 Steam Stardew `1.6.15` / SMAPI `4.5.2` 真实加入隔离 Junimo server。Control 收到 Content Patcher `2.9.0`、Farm Type Manager `1.26.1`、Save Backup `4.5.2`，详情 API 的版本/分组准确；主动断线 stale、同 ID 重连新 reportedAt、server 重启后旧记录 stale 均通过，全程没有管理动作。

## 影响文件、验证与下一步

- 主要影响 `smapi-mod-src/{ModEntry,PlayerModContexts}.cs`、C# `Program.cs`、`player_mods_test.go`、`players_mods_handlers_test.go`、嵌入 DLL、运行栈清单和联调文档。真实 game-data 编译 0 errors、1 个既有 analyzer warning；当前已提升并完成真实 LAN 联调的嵌入 DLL SHA256 为 `b15479eda376f386fdadc1cc9d0c31815cabc0a919d0655debe6d7117352c05a`，与 runtime manifest 一致。不同容器项目路径的最终 fresh build 仍通过但字节摘要不同，不宣称跨路径 reproducible build；若要以 fresh SHA 作为门禁，必须固定 SDK digest、PathMap、restore 与项目挂载路径。
- Docker C# 契约、`go test ./... -count=1`、`go vet ./...`、`go build ./...`、玩家 Mod 定向 verbose 测试、前端 12 项状态脚本、独立 TypeScript 检查与 production build 全部通过；源码/嵌入/运行栈三方 SHA 一致。
- 自动化隔离编译使用 `sap-pmods-p3-*`；真机联调使用 `%LOCALAPPDATA%/Temp/sap-player-mod-real-20260806`、独立端口、克隆 game-data volume 与新建测试存档，不挂或复用用户存档/设置。没有启动、停止或修改宿主既有 Stardew 实例。
- 注意一次隔离缺口：driver 的最终 Compose project 仍为实例 basename `stardew`，未继承任务 Panel project，导致 steam-auth 复用了 2026-07-06 已存在的 `stardew_steam-session`。没有读取 token 内容，清理时保留旧卷；LAN peer context 不依赖邀请码，因此功能证据仍有效，但不能宣称认证卷完全隔离。以后启动真实组件前必须检查最终 project、network 和全部 named volume，而不能只查任务 label。
- 下一位维护者仍需按 `docs/06-integration.md` 补 PC 原版、两种官方 CJB、Android/iOS、多个远端玩家并发与真实登录页面视觉；记录平台/游戏/SMAPI/Mod 版本、连接方式、Control 日志和截图。没有 Android 实验性 SMAPI 环境时继续写“未验证”，不要推断支持。
- ModContext 是客户端自报；只按官方 UniqueID 标 CJB，改 manifest ID 可绕过。不要据此自动踢出、封禁或拦截；Steam SDR 兼容仍不在本阶段。

# PLAYER-MOD-CONTEXT-1 接手记录（2026-08-06，completed，v0.4.8 released）

## 改了什么

- Control `0.3.0` 新增 `PlayerModContexts.cs` 纯契约和三类 Multiplayer event 接线。`PeerContextReceived` 使用 `peer.PlayerID/HasSmapi/GameVersion/ApiVersion/Mods` 生成完整报告；`PeerConnected` 在没有先到上下文时生成 pending，10 秒后变 unavailable；`PeerDisconnected` 变 stale。历史文件在新进程载入时统一 stale，sidecar 独立为 `control/player-mod-contexts.json`，原子替换，不增加 `players.json` 高频体积。
- 输入边界固定为 512 个玩家、1024 个唯一 Mod、2048 个原始条目与 32/256/256/64 字符；控制字符被移除，UniqueID 大小写不敏感去重。过量报告不截断后比较，而是 unavailable + `mods:null`。Control `options.json` 同时新增实际 `gameVersion/apiVersion`，schema 仍兼容为 2。
- Go 新增 `player_mods.go`：安全读取 4 MiB sidecar 和 2 MiB options，以 `options.loadedMods` 建服务器 map，再用物理 `ModInfo`/`syncKind` 只补元数据。新增任意登录用户可读的 `GET /api/instances/:id/players/:uniqueMultiplayerId/mods`；非法 ID 返回 `400 invalid_player`。
- 比较 items 使用 `match/missing_on_client/client_only/version_mismatch`，整体不可比较时为 `comparison.status=unavailable`。只对实际 loaded + client_required 生成 missing；server_only/unknown 不生成 missing。后续已取消虚拟 SMAPI item，并统一过滤 SMAPI、JunimoHost.Server 与面板 Control；两条官方 CJB UniqueID 只增加 `riskFlags:["cjb"]`，没有任何管理副作用。

## 影响、验证与下一步

- 影响文件：`embedded/smapi-mod-src/{ModEntry,ControlContract,PlayerModContexts}.cs`、`smapi-mod-contract-tests`、两份 manifest、嵌入 DLL、`config/runtime_stack_manifest.json`、`player_mods{,_test}.go`、`players_handlers.go`、`instance_handlers.go`、`players_mods_handlers_test.go` 和四份长期文档。Control 版本 `0.3.0`，第三阶段重编译后的嵌入 DLL SHA256 `b15479eda376f386fdadc1cc9d0c31815cabc0a919d0655debe6d7117352c05a`。
- Docker .NET 6 纯契约测试通过；以只读 `stardew_game-data` 实际 SMAPI/game 引用执行 `dotnet build -c Release /p:GamePath=/game /p:EnableModDeploy=false`，0 errors、1 个既有 analyzer/compiler warning。定向 `go test ./internal/games/stardew_junimo ./internal/web -count=1` 通过；最终 `go test ./... -count=1`、`go vet ./...`、`go build ./...` 全部通过。最终源码 DLL、嵌入 DLL 与运行栈清单三方 SHA256 一致。
- 现有运行实例必须通过受控运行组件同步并重启 server 才会加载 Control `0.3.0`；只替换宿主 DLL 而不重启的旧进程不会产生新 sidecar。前端页面属于下一阶段，本阶段不要开始 UI。
- 仍无法取得当前 ModContext 的边界：未安装 SMAPI 的客户端；peer context 握手未完成/中断或字段超限的客户端；Steam SDR/其它未把标准 SMAPI context 送达服务端的链路（本阶段没有兼容桥）；更新前旧 Control 进程；已断开玩家（只保留 stale 历史）；以及 headless host 本身（不是远端 peer）。这些情况必须展示 unavailable/pending/stale，不能把 `mods:null` 解释成“没有 Mod”。

# MOD-INSTALL-TIME-1 接手记录（2026-08-01，v0.4.6 已发布）

## 改了什么

- `registry.ModInfo` 增加可选 `installedAt`；`uploadModZip` 在所有目录移动成功后，将同一 ZIP 的 UTC RFC3339Nano 时间原子写入 `<dataDir>/.local-container/control/mod-install-times.json`。人工上传、Nexus 与远程复用安装都走该入口。
- 新 sidecar schema=1、上限 2 MiB、临时文件 `0600`。安装和删除的读改写由进程内互斥串行，避免人工上传与异步一键安装并发完成时覆盖另一条记录。旧目录无记录时不伪造；条目的 UniqueID 与当前 manifest 不匹配或时间非法时不应用。列表读取补充元数据失败时保持 Mod 管理可用，新的安装不会覆盖损坏文件。
- sidecar 提交失败会回滚本次移动目录；跨文件系统 copy 失败额外清理当前半目录。幂等跳过不刷新时间；删除、同包删除和 Web 多 ZIP 回滚通过 `DeleteMod` 清理记录。

## 影响、验证与下一步

- 主要文件：`registry/types.go`、`stardew_junimo/{mods.go,mod_install_times.go,mods_test.go}`；接口契约见 `docs/06-integration.md`。未增加数据库迁移，也没有绕过 Junimo driver 在 Web 层保存 Mod 状态。
- 测试覆盖单/多 Mod、同批时间、并发安装不丢记录、重新列表、写失败回滚、幂等旧时间和删除清理；Docker Linux 全量 test/vet/build、Docker runtime/updater integration 与本地候选 health/version smoke 均通过。
- Docker Desktop 真实 Panel E2E 已补齐：实际 HTTP setup/session、深层 ZIP、本地第二包、浏览器扩展一键公网 HTTPS 下载 job、重启、删除全部通过；一键下载与人工上传均返回并持久化 `installedAt`。多 ZIP 第二包失败和 sidecar 原子提交失败均验证目录/时间回滚，原数据保持完整。
- 发布阻塞修复：`fullStackInstanceStatus` 对 `uninitialized/admin_created` 直接返回 `not_needed/100`，全部实例均无需运行时同步时聚合状态也规范为 `not_needed/100`；专项单元测试覆盖单实例与真实 store 聚合。兼容矩阵的 required image/Git/SMAPI 网络访问加入有界重试与严格分块续传，未放宽 trusted host、digest、Range、大小、SHA 或 ancestry。
- 精确候选 `0c83a257e11c` / image ID `sha256:dd7f97a4e005c8dc50820d676fb30c9e926f93d962bb4e34477782bb88b02940` 已在 Docker Desktop 完成 `v0.4.5` Web 一键更新的 unhealthy 自动回滚与成功更新；更新后重启、上传、一键远程安装、安装时间顺序和桌面/390px 搜索排序均通过，42% 状态未复现。真实 `v0.3.13 → v0.4.6` 生产 `RunApply` 也通过，非目标 game 容器与数据受保护。
- 正式 tag、Release、兼容矩阵和官网部署均成功；Docker Hub、阿里云 ACR、GHCR 的 `0.4.6/latest` digest 统一为 `sha256:6fd03bb202e8083b3453e2351bd70251c1bc2fea0e5c0f779fc62d99af39e07f`。三仓精确 tag 回拉后的 OCI revision 均为 `0c83a257e11c`，各自独立 health/version smoke 通过；官网首页实际 HTTP 200 并展示 `v0.4.6`。
- 后续若支持手工重命名 Mod 文件夹，需要先设计 sidecar key 迁移；当前管理流程不提供重命名。不要把 Nexus `updatedAt` 当安装时间，也不要为历史 Mod 用目录 mtime 回填。

# SMAPI-DOWNLOAD-RESUME-1 接手记录（2026-07-28，v0.4.5 released）

## 改了什么

- 生产主机的 Stardew/SDK 均成功，SMAPI 4.5.2 官方包却以约 `40 KiB/s` 下载；旧 `netdns.NewClient(2*time.Minute)` 把 `41,889,142` 字节（约 40 MiB）整包限制在一次请求中，连续多次在读取 body 时超时。
- `smapi_archive.go` 现在按 2 MiB 请求当前受审候选的 Range。单次请求超时或断流后保留已写入的区间前缀，下一请求从精确偏移继续；连续 4 次零字节进展才切换候选。没有改变安装 job 的 2 小时总门限、Junimo 安装容器或 SMAPI 更新 staging 流程。
- embed 清单恢复固定 `gh.llkk.cc → github.dpik.top → ghfast.top → GitHub 官方`。Go/Python 都只接受按版本生成的精确 URL 序列与六项 host allowlist；实例 `.env` 地址仍不能参与选择。
- 每段必须返回匹配的 `206/Content-Range`；最终继续核对固定大小、SHA-256 与 ZIP 安全结构。候选整包不合格时清空临时文件再切下一项，不跨源拼包。

## 影响、验证与下一步

- 影响文件：`backend/internal/games/stardew_junimo/{smapi_archive.go,smapi_archive_test.go}`；初装与 SMAPI 更新事务共用同一缓存函数，HTTP API、阶段名和前端请求体不变。
- 回归覆盖正常分块、半包续传、坏代理切官方、连续无进展失败、`Content-Range` 解析与最终 SHA 拒绝；release workflow 新增真实下载 gate。
- Docker Desktop Linux 容器真实冷缓存下载 `41,889,142` 字节耗时 `2m26s`，摘要/ZIP、缓存 `0600` 与临时文件清理均通过；本地 `0.4.5` Panel 镜像冒烟通过。发布后仍不得通过放宽 trusted hosts 或重新读取 `.env.SMAPI_DOWNLOAD_URLS` 绕过清单。
- 发布工作流 `30369196944` 全绿；三仓 `0.4.5/latest` digest 均为 `sha256:a8155defc50690b8b1e90c95f5b107e818b5438c68c341f90f9ebf8b7be428ad`。ACR 正式镜像回拉返回 `version=0.4.5`、`commit=09250ed68ce9`，隔离 health/version/Docker/Compose 冒烟通过。
- 发布后补验 ACR 正式 `0.4.4/0.4.3/0.3.13 → 0.4.5` 三条生产 `RunApply`，目标 health/version、SQLite/setup、404 和 game 容器隔离均通过。随后以 tag 同代码重新执行真实空缓存 archive gate，40 MiB 下载耗时 56 秒，摘要/ZIP/`0600`/临时清理与全部续传/回退专项通过。

# SAVE-BACKUP-EAGER-MAINTENANCE-1 接手记录（2026-07-28，completed）

## 改了什么

- 根因在 `saves.go` 的调用时机：Control 每次 Saved 都正确写事件，但 `RunBackupMaintenance` 只由备份列表 GET 触发。积压几天后每个旧事件都读取当前存档日并覆盖同一个最新 ordinal，所以保留数量是 5，日期却不连续。
- `Driver.RunBackupMaintenanceScheduler` 在 Panel 启动时立即运行一次，之后每 2 秒对启动时发现的所有 Stardew 实例消费事件。`cmd/panel/main.go` 用 `signalCtx` 管理生命周期，Panel 退出时调度器同步停止。
- `RunBackupMaintenance` 以规范化 `dataDir` 对应的进程内 mutex 串行，避免后台任务和备份列表 GET 并发打包。没有修改 Control、Junimo API、备份文件命名、策略接口或前端状态结构。

## 影响、验证与下一步

- 主要文件：`backend/cmd/panel/main.go`、`backend/internal/games/stardew_junimo/{driver,saves,saves_test}.go`。公开文档同时纠正了仍描述旧 `latest/scheduled/daily` 机制的两页内容。
- 测试：连续模拟 7 日、全程不调用列表 API，最终只存在连续第 3–7 日五档；8 路并发维护只消费一个事件。后端全量 test/vet/build 通过，Docker Desktop 候选与正式发布结果见 `docs/09-image-build.md`。
- `v0.4.4` 已正式发布：commit `d5d815d365cb`，三仓 digest 统一为 `sha256:446b168c8784b3c7e77c5006b85adcbe2c1b106e80992281a929a75108fd572a`；正式 GHCR 镜像实际启动后 `/health`、`/api/version` 和 OCI revision 均正确。生产实例仍须在升级后等待下一次 `GameLoop.Saved`，才能确认真实游戏链路开始连续生成。
- 旧版本已经错过的历史日没有可恢复的旧磁盘内容，升级后不能伪造补齐；只能从下一次 Saved 起保持连续。若未来支持运行时新增实例，必须让调度目标动态刷新，而不是继续依赖启动时实例快照。

# PANEL-HEALTH-WATCHDOG-1 接手记录（2026-07-26，completed，已发布 v0.4.3）

## 改了什么

- 新增 `cmd/panel/health_monitor.go`。首次探针在启动一分钟后执行，之后每分钟复用 `/health` 的 `Store.Ping`，每次最多 5 秒；只按 SQLite 原生 primary code 9 累计，成功、超时和其它错误都会清零，连续第三次才调用进程退出回调。
- `cmd/panel/main.go` 改回普通 `storage.Open`，不再让任意业务数据库操作触发生产退出。门限到达后 `os.Exit(1)`，标准 Compose 的 `restart: unless-stopped` 只拉起 Panel；没有 Docker API 调用，也不会重启 Stardew/Junimo 容器。
- Dockerfile HEALTHCHECK 从 30 秒调整为 1 分钟，timeout 仍为 5 秒、retries 仍为 3。Docker 把容器标记为 unhealthy 并不会执行 restart policy；真正触发恢复的是内部监控让主进程退出。

## 影响、验证与下一步

- 主要文件：`cmd/panel/{main,health_monitor,health_monitor_test}.go`、`internal/storage/sqlite_driver.go`、`Dockerfile`。HTTP 接口及 `/health` 响应形状没有变化。
- 后端全量 test/vet/build、updater/runtime Docker integration 均通过。Docker Desktop 29.5.3 最终 `0.4.3` 候选的 image inspect 为 interval 60 秒、timeout 5 秒、retries 3；测试 Panel 退出后 RestartCount `0 → 1`、PID `13602 → 13772`，重启前后 `/health` 均返回 ok。Linux 容器回归连续 10 轮通过。
- `v0.4.3` 已由 commit `6db97e685386` 正式发布。三仓精确 tag 均解析到 digest `sha256:e1ffb4132610607305ce2316e5f3683b6413765bc61057ffd019b2841d38e559`；正式镜像内 `modernc.org/sqlite` 为 `v1.54.0`。发布后再次执行真实 `0.4.2 → 0.4.3` updater apply，SQLite 数据、setup 状态、404 保留，隔离 game 容器 ID 不变。

# PANEL-SQLITE-INTERRUPT-1 接手记录（2026-07-24，completed）

## 改了什么

- `web.NewHandlerWithError` 在 HTTP server 构造时读取一次管理员存在状态，`server.initialized` 用 `atomic.Bool` 服务 setup status 和初始化门禁；`CreateFirstAdminWithSession` 成功后立即发布新状态。未知路径在初始化门禁前返回 404，SPA 只允许明确的 Stardew 页面。
- 存储连接改用 `sqlite_driver.go` 的观察包装器，保留 modernc driver 的连接、statement、rows 与 transaction 接口，并按原生错误码统计连续 `SQLITE_INTERRUPT`。生产 `cmd/panel` 在第三次连续中断时记录日志并 `os.Exit(1)`，让 Docker restart policy 恢复进程。
- `modernc.org/sqlite` 从 `v1.29.10` 升为 `v1.54.0`（匹配 `modernc.org/libc v1.74.1`）；新版连接 `IsValid/ResetSession` 会淘汰被 interrupt 的文件连接。

## 影响、验证与下一步

- 主要文件：`backend/internal/web/{handler,auth_handlers}.go`、`backend/internal/storage/{db,sqlite_driver}.go`、`backend/cmd/panel/main.go`、`backend/go.{mod,sum}` 及对应回归测试。
- 验证通过：真实递归查询取消后下一次 `SELECT 42` 成功；连续三个 SQLite code 9 触发恢复回调；关闭 SQLite 后未知页面/API 仍为 404、setup status 仍读缓存；`go test ./... -count=1`、`go vet ./...`、`go build ./...` 全部通过。
- 后续增加前端 Browser History 路由时，必须同步扩展 `isKnownRequestPath` 白名单，否则深链接会按安全默认返回 404。不要把初始化检查重新放回每请求中间件。连续中断门限只统计原生 code 9，普通 `context.Canceled` 不应导致进程退出。

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
# 2026-07-20：飞牛旧容器标准 Compose 迁移脚本

- 新增 `deploy/migrate-fnos.sh`，解决旧飞牛/NAS 容器因 Compose labels 不完整而无法使用 Panel 内置升级的问题。脚本自动选择最高版本的运行中健康 Panel；多个最高版本指向不同数据目录时拒绝自动选择，可用 `PANEL_CONTAINER` 显式指定。
- 影响文件：`deploy/migrate-fnos.sh`、`scripts/tests/test_migrate_fnos.sh`、`.github/workflows/release.yml`、`docs/09-image-build.md`、`docs/08-future-roadmap.md`。正式 Release 现在会附加该迁移脚本。
- 安全边界：仅接受可信镜像身份、稳定 SemVer、bind data mount、Docker Socket、单一逻辑端口、默认用户和无额外挂载的部署；默认 bridge 直接迁移，合法现有自定义/Compose 网络作为 external network 复用，host/container 模式拒绝。迁移先拉取并校验目标镜像，再备份 inspect/部署文件，旧容器改名保留。新容器健康、精确版本或 canonical labels 任一失败即恢复部署文件和旧容器。
- 国内执行入口暂用 `https://gh-proxy.com/https://github.com/.../releases/latest/download/migrate-fnos.sh`；`anxinas.dpdns.org/migrate-fnos.sh` 当前返回 404，不得写进用户命令，待自托管站实际同步后再切换。镜像按 ACR、1ms、DaoCloud、GHCR、Docker Hub 顺序尝试；无法访问 GitHub latest API 时必须显式传入 `TARGET_VERSION=x.y.z`。
- 已验证：Docker `bash:5.2` 中 `scripts/tests/test_migrate_fnos.sh` 与 ShellCheck 通过；隔离 Docker 29 dind 中，真实 `0.3.7` 独立容器已通过 ACR `0.3.13` 成功迁移，新容器健康/精确版本/Compose labels、旧容器停止保留及 result 文件均验收成功。停止新 Panel 的健康失败注入也已恢复原容器名称、运行状态、`restart=no` 和原部署文件。中断和真实多候选容器矩阵仍需补齐，不能直接宣称飞牛真机已经验证。
- 下一步注意：脚本成功只完成 Panel 部署标准化。若运行时 Control 较旧，管理员仍须在新版 Panel 执行“运行组件升级”完成受控游戏重启；不得把飞牛直接 restart 游戏容器当作 Control 更新。
# 2026-07-20 handoff：一键全栈升级第二阶段

- 影响文件：`internal/updater/{deployment,docker_cli,service,helper,apply_helper,legacy_conversion,types}.go`、`cmd/panel-updater/main.go`、`cmd/panel/main.go`、`internal/web/updater_handlers.go`，以及 `internal/games/stardew_junimo` 的 required-runtime、lifecycle、runtime apply/rollback/inspection/save 流程。
- Compose 服务名由反查结果驱动；部分飞牛 labels 不能单独作为信任依据。legacy conversion 必须满足单一 bind `/data`、Docker socket、8090 发布、非 privileged/自定义 user 等安全边界。
- Control 更新只允许在受控停服事务中写入。失败时旧 Control 可留作人工恢复材料，但 lifecycle 启动门会阻止它再次运行；成功必须取得本次启动产生的新鲜 Control manifest。
- 验证：`go test ./...`、`go vet ./...`；Docker updater integration 覆盖非 `panel` 服务名、成功切换与健康失败回滚。发布前还须运行 legacy conversion 成功/失败注入和候选镜像 smoke。
- 下一步注意：新增 required-runtime 阶段时同步更新 web 聚合和前端 phase 集合；不得绕开 `stardew_junimo` driver 在 API handler 内直接操作 Control 或游戏容器。
- Docker Desktop 真机补充：双飞牛容器依次转换不会互相重建；健康失败恢复旧容器。真实 `.125` 的 stopped/running Control 0.2.1→0.2.2 均通过，running 路径验证了通告、`GameLoop.Saved`、整档备份、停服、更新、重启和实载版本。测试同时推动修复内部 8080/宿主 API 端口混用与 Windows bind 目录 rename 锁。
# 2026-07-24 handoff：PANEL-SQLITE-INTERRUPT-1 / v0.4.2

## 改了什么

- `web.Handler` 在启动时读取一次初始化状态并用原子缓存服务后续请求；首次管理员创建成功后更新缓存。显式页面/API allowlist 在初始化检查前拒绝未知路径。
- storage 使用可观测 SQLite driver；`modernc.org/sqlite` 升到 `v1.54.0`。连续原生 code 9 计数达到三次时主进程退出，交给 Docker restart policy 重建；其它结果清零。
- 新增真实取消查询恢复回归和真实镜像 opt-in 升级测试，后者从 GHCR `0.4.1` 通过正式 `RunApply` 切到本地 `0.4.2` 候选。

## 影响文件、验证与注意事项

- 主要文件：`cmd/panel/main.go`、`internal/storage/{db,sqlite_driver,db_interrupt_test}.go`、`internal/web/{handler,auth_handlers,auth_handlers_test}.go`、`internal/updater/apply_docker_integration_test.go`。
- Docker Desktop 29.5.3 已通过候选 smoke、100 条扫描、持久卷重启、真实升级；Linux 容器内取消恢复 10 轮通过。完整发布门禁包括 backend test/vet/build、两组 Docker integration、兼容矩阵、脚本/ShellCheck、前端/文档生产构建。
- 只统计 SQLite 原生错误码 9，不按错误字符串猜测。不要把单次请求取消当成进程级故障；阈值保护依赖 Docker 部署的 restart policy。正式 `v0.4.2` Web API 端到端升级已通过，三仓镜像同 digest，发布二进制 build info 已核对驱动版本。
# FULL-STACK-UNINSTALLED-TERMINAL-1 / RELEASE-REMOTE-RETRY-1 接手记录（2026-08-01，completed，待发布）

- 修复内容：`backend/internal/web/updater_handlers.go` 对 `uninitialized/admin_created` 直接产出 `not_needed/100`，全部实例均无需同步时顶层也产出相同终态，解决 v0.4.5→候选升级后未安装实例长期显示 42%。已安装实例的 inspection/coordinator 分支未放宽。
- 发布门禁：`scripts/compatibility_matrix.py` 的 required image inspect、Git traceability fetch 与 SMAPI 分块下载均改为三轮有界重试。已验分块可跨受审 URL 续传；截断分块不计入 hash；最终 SHA 错误会从不同起始源整包重下。白名单、Range、大小、digest 与 ancestry 仍是硬失败。
- 测试：`internal/web` 覆盖两个未安装状态和真实 Store 聚合；Python 共 19 项，新增跨源续传、截断、恶意重定向、重试耗尽、错摘要整包重下、镜像 inspect 重试与 Git fetch 边界。真实公网门禁在 Docker Hub TLS timeout、SMAPI 32 MiB 处 SSL EOF/429 后恢复并通过；生产 Go 空缓存下载及 updater/runtime Docker integration 均通过。
- 下一步：必须以包含本修复的最终 commit 重建 `0.4.6`，重新执行 v0.4.5 Web 一键升级、unhealthy 回滚、升级后 Mod 功能和右侧栏终态 QA；在这些证据完成前不得 tag。不要通过把 `checking_runtime` 普遍改成成功、移除 trusted hosts 或跳过最终 SHA 来规避网络问题。

# RUNTIME-AUTH-OFFLINE-ACCEPTANCE-1 接手记录（2026-08-09，released in v0.4.10）

- 改动：`runtime_update_apply_runner.go` 的 `waitRuntimeAuth` 在容器 running 后直接探测 `/steam/ready`，不再先要求 Docker health；删除只检查容器 health 的旁路，auth 新旧版本和最终复验统一使用服务接口。`runtime_apply.go` 保留并校验 HTTP status：200 接受受支持 schema，真实镜像的 503 仅接受 legacy `ready=false`；current `accounts` 必须是 array。未登录/无 ticket 写 warning，其它状态、坏 schema、接口不可用与 digest 不匹配仍触发回滚。
- 影响：运行栈 apply/恢复验证和 `steam_auth_ready` 检查文案；无 API 路由或 JSON shape 变化，无认证卷、Compose 重建顺序或凭据处理变化。
- 验证：聚焦单元覆盖 HTTP 500、503 current/ready=true 以及 current `accounts=null/number/object` 拒绝，并固定真实 503 `ready=false` 合约；Docker integration 同时验证 200 ready 成功和真实 404 fail closed。正式 v0.4.10 又完成 v0.4.9/v0.3.2 Web 一键升级、Panel 自更新候选 120 秒 unhealthy 自动回滚、健康 Panel 候选重试成功、数据/非目标资源保护和重启；独立 runtime Docker integration 证明 steam-auth unhealthy + 合法接口会继续。Release/三仓结果见本文件顶部与 `docs/09-image-build.md`。
- 下一步：不要重新把 Steam 登录/ticket 或 auth Docker health 提升为升级硬门槛。若扩展 schema/status 白名单，必须同时补 HTTP 状态、坏 schema、真实容器与完整 apply/rollback 证据。

# STARTUP-NEWGAME-DURABILITY-1 接手记录（2026-08-13，代码完成，待正式发布）

## 改了什么

- 启动验收增加独立 `ControlRuntimeGate`。缺少本次 `options.json` 是 pending，实例保持 `starting/control_runtime_starting` 并等待完整 20 分钟预算；合法快照明确给出错误版本才是 `control_runtime_version_mismatch`。manifest/DLL/JSON 损坏使用独立 invalid code，超时安全停服为 `control_runtime_start_timeout`，停服也失败才是 `control_runtime_cleanup_failed`。
- `ReconcileState` 尊重活动 lifecycle owner 和持久 new-game owner；容器存在但 Control 未 ready 时不得提前发布 running。启动前旧 Control snapshot 无法安全清理时 fail closed。
- `/state` 新增 `installationDiagnostic`，把必需文件、Compose、镜像、server 容器、Control static/runtime 和推荐动作分开。Docker/镜像探针暂时不可用时返回 unknown/diagnose，绝不能由 `state=error` 推导“未安装”。
- 新建档入口强制使用 `Idempotency-Key`、exclusive lifecycle job 和实例级持久 owner/token；缺 key 返回 428，零 job/零 owner。startup 与 HTTP writer 在事务创建时固定；Control、gameloader 或新目录任一前进后永久禁止再次 POST。相同 key+配置返回原 job，不同配置冲突；只有证明旧 job 已终态，才可轮换 token 并恢复原事务。owner 由完整 staging + fsync + no-replace rename 原子抢占，历史空目录仅在零事务/零运行进展证据时才可隔离恢复。
- Control 升至 `0.3.1`。成功门禁固定为当前事务 `save-loaded`、完整内存定制和唯一 host 正确、同一持久 `save-now commandId` 收到精确 `GameLoop.Saved`、两轮稳定 XML/SaveGameInfo 字段与 SHA-256 正确；随后才写 profile、success 并释放 owner。Control 在打开保存菜单前持久 `pending-save-command.json`，崩溃后只恢复同 ID，终态回执落盘后才清除。
- 普通 Stop/Restart/Restore 不能取消未结束 owner。回滚使用 `rolling_back` 与每步 write-ahead journal；中断后手动 Start 只停服并继续回滚，不 ComposeUp/POST。Panel 或宿主中断后不自动启动；Runtime/SMAPI 恢复也只收敛/回滚静态材料并保持停服，用户手动启动才恢复同一建档事务或普通运行。

## 影响文件与接口

- 后端：`control_runtime_gate.go`、`installation_diagnostic.go`、`driver.go`、`lifecycle.go`、`new_game_{transaction,transaction_owner,progress,lifecycle,durability,durability_transaction}.go`，以及对应 Web state/lifecycle handlers 和测试。
- Control：`embedded/smapi-mod-src/{ControlContract,ModEntry,DeferredCommandOutcomes}.cs`、契约可执行程序、两份 manifest、嵌入 DLL 与 `config/runtime_stack_manifest.json`；运行栈 identity 为 `control-0.3.1`。
- HTTP：`GET /api/instances/:id/state` 可选增加 `installationDiagnostic`；`POST /api/instances/:id/saves/custom-new-game` 必须带 `Idempotency-Key`，成功仍以 202 返回 `{jobId}`，缺 header 返回 HTTP 428 / `idempotency_key_required`。
- 前端/API：`createNewGame` 显式发送 request ID；新建档弹窗只在请求尚未被 202 接受时为相同配置复用 key。安装页、桌面/移动壳共用诊断分类器；详情见最新 frontend handoff。

## 如何验证

- Control 启动：覆盖 pending→ready、pending timeout、明确 mismatch、invalid snapshot、Compose cleanup 失败、snapshot cleanup 失败、context cancel 和 Reconcile 不提前提升。
- 建档：覆盖 owner 并发 winner、same-key same-config 原 job、same-key different-config 冲突、token 丢失/轮换恢复、startup writer POST=0、loader 先前进、多个目录 ambiguous、unknown 后不重提、目标绑定与手动恢复。
- 耐久：用精确 transaction/save fixture 覆盖旧/错 `save-loaded`、所有身份/外观/颜色字段独立错误、players 无唯一 host、save-now 旧/错/unknown/expired、同 ID pending journal 恢复、GameLoop.Saved 身份、主 XML/SaveGameInfo 字段与双稳定 hash；Control 必须用真实 game-data 编译并跑契约矩阵。
- 回滚/互斥：覆盖 rollback plan 持久失败零变更、quarantine/每个 restore 步骤中断恢复、ComposeDown 失败保留 owner，以及所有存档/安装/更新/玩家/重启计划变更入口在 owner 下零写入。宿主恢复用例要断言 `ServerWasRunning=true` 仍零 ComposeUp。
- 2026-08-13 当前源码已通过：Control 契约与真实只读 game-data 0-error 编译，编译/嵌入/清单 SHA-256=`3833769287e794d392296c52df760f8451b24a177243a0926d6f0ca9fd81b3ce`；Go 全量 test/vet/build；前端 14 项状态测试/audit/build；脚本/ShellCheck；兼容矩阵/remote artifacts；runtime/updater Docker integration；网站 build。隔离真实 new-game E2E 的 startup writer POST=0、HTTP writer POST=1 且旧档双哈希保持，两条都完成四段耐久门禁。实测还固定了 Stardew 1.6 `Gender/gender`、可空旧 `isMale` 和 `shirtItem/pantsItem.itemId` 磁盘契约。
- 上述是 pre-candidate 证据。最终 commit 候选仍须跑正式 Web 升级/回滚、图形化 Compose conversion、升级后功能/Browser 与生产真机，证据写入 `docs/09-image-build.md` 后才能 tag。

## 下一步注意事项

- 不要把 `options.json` 不存在、Docker probe unknown 或 `control.runtime=not_observed` 改回版本 mismatch/未安装。只有明确、可解析且错误的版本才使用 mismatch。
- 一旦 transaction 的 `commandCalled`、`progressObserved`、unknown 或 ambiguous 为真，任何自动恢复、按钮重试和运维脚本都不得调用第二次 `/newgame`。不要只以“尚无完整 XML”判断是否可以重提。
- Control 成功状态必须继续是 `save-loaded` 且包含冻结的 customization identity；不得用旧的临时状态、只读内存字段或“磁盘 XML 已出现”替代同 ID save-now 与 XML 终态。
- owner 文件是安全锁，不是缓存。损坏/缺失 transaction 时返回 recovery required 并保留目录，不得静默删除；support bundle 继续排除 owner、事务、存档和命令结果正文。
- 本次正式版本还要一起收口 `92f3be6bb2731358420ba315ac18029c2506d81f`（Release `run.sh` 必须含 swap/swappiness=60 修复）与 `621c5645e0048da7c4793035615438ed78fc7002`（图形化 Compose 自动标准化）。两者虽已在 `origin/main`，最终候选/升级后/生产真机证据仍未完成，不能在 handoff 或 Release notes 中提前写成真机通过。
- 不要为修复 transaction recovery 顺手启用 Docker restart policy 或 Panel 启动自动 ComposeUp。宿主重启后的普通实例由用户手动开启，这是当前明确产品策略。
