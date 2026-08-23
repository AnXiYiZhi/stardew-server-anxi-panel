# v0.5.11 Steam 凭据恢复与常驻更换账号入口（2026-08-22，released）

## 变更清单、受影响链路与专项矩阵

- 生产 `v0.5.10` 只读诊断确认 SteamCMD 在缓存未命中后的完整登录行同时输出 `Logging in user ... Invalid Password`，exit 5；旧 switch 先命中通用 progress，错误发布 `error/steamcmd_failed`。`v0.5.11` 把具体凭据失败标记置于通用登录进度前，组合行稳定发布既有 `steam_auth_failed/credentials_required`，网络、下载、磁盘等普通失败仍保持 `steamcmd_failed`。
- 管理员安装页保留复用保存凭据的“登录授权”，并新增常驻“更换 Steam 账号 / 重新认证”。新入口不受历史错误终态、部分安装或普通表单诊断 guard 阻断，始终要求完整新凭据并发送既有 `forceReauth=true`；后端清除 Steam/SteamCMD 授权缓存，但保留游戏文件和存档。没有新增 API/DTO、SQLite migration、Compose、Control/SMAPI/Junimo runtime manifest 或明文凭据回显。
- 本版专项由后端组合行/缓存回退回归、前端源码与状态机回归、production bundle、发布前应用内 Browser 桌面/390px 交互组成。正式候选另外完成不可变镜像 fresh/restart、`v0.5.10` 真实 Web unhealthy/healthy 升级和升级后长期状态/既有高风险链复验；为避免向 Steam 发送测试账号密码，没有使用生产或长期 Steam 凭据做真实错误登录注入。

| 维度 | 实际执行 | 通过标准与结果 |
| --- | --- | --- |
| 凭据分类 | 缓存登录失败后回退完整登录；同一行含 progress 与 `Invalid Password`，exit 5 | 两次 SteamCMD、零 steam-auth 误启；终态精确为 `steam_auth_failed/credentials_required`，已通过 |
| 前端恢复 | `steam_auth_failed/credentials_required` 与 `missing-files` 并存；管理员主动强制更换账号 | 认证失败优先；常驻入口可打开完整 3 项凭据表单并发送 `forceReauth=true`，已通过 |
| 权限与安全 | 普通用户、运行/启动中、表单已打开、凭据显示 | 入口 admin-only；busy/运行态禁用；不预填或打印保存密码，已通过 |
| 响应式 | 1280px 与 390px 安装页 | 无横向溢出、遮罩或 console warning/error，已通过 |
| 不可变候选 | fresh/restart、`v0.5.10` Web unhealthy rollback/healthy apply、升级后状态保持 | 同一候选 digest；`failed_rolled_back/health_check_failed` 后旧版恢复，健康升级与既有受影响链全绿 |

## 候选、Tag、正式提升与 Release 证据

- `main@a9e186249a5c70c2e6fe45b7ed10a09db0b0c8bb` 与 `origin/main` 同步后自动解析 `0.5.11`、previous=`0.5.10`、OCI build date=`2026-08-22T13:17:09Z`。Compatibility `32575311243` 用时约 `2m27s`，backend tests、frontend tests/build 与隔离 Docker integration 全绿。不可变候选 `32575311262` 用时 `10m23s`：selected code gates `3m55s`、Windows wrapper `5s`、image/fresh/restart/真实 Web 升级 `5m51s`，随后推送并封存 proof。
- 路径选择实际执行 compatibility contracts、部署脚本、backend test/vet/build、真实 Junimo network/runtime integration、frontend 全状态回归与 production build、website build；runtime manifest 输入未变，因此 remote artifact verification 按选择器跳过。候选从 `v0.5.10` 通过公开 Web API完成 unhealthy rollback 与同 digest healthy apply，并复验 SQLite/初始化/非目标容器与 volume、Mod update、legacy Junimo repair、存档导入 exact-target/FIFO no-effect 与下一管理员 mutation 恢复；最终输出 `all candidate gates passed`。
- proof artifact=`release-candidate-0.5.11-a9e186249a5c`，ID=`9476506539`，size=`483` bytes，archive digest=`sha256:d08adee5cae230f2f276eeedd055d74d4790409baa56fa272280c8ee5c7abd0c`。candidate ref=`ghcr.io/anxiyizhi/stardew-server-anxi-panel:candidate-0.5.11-a9e186249a5c`，唯一 manifest digest=`sha256:10c9813328370ae8ac92f11271fb76cd03787aab3b7f7fd523f20d66dfae8876`，config digest=`sha256:8108c82fc84e229d00493bd0c52174db551ba815114a0f073527ec68da708426`。
- 自动 Tag `32575807110` 成功；`v0.5.11` 为 annotated tag object=`d8bf5075d57f7aaf1b834ad62e12418a2db67ab7`，tagger date=`2026-08-22T13:27:30Z`，peeled commit 精确为候选 SHA，message 固定候选 workflow 与上述 digest。正式提升 `32575818623` 用时 `1m15s`，只提升 proof 中的精确 digest、没有 rebuild；候选身份、三仓精确版本、GHCR `/health`/`/api/version` smoke、三仓 `latest` 与 Release 创建步骤全部成功。
- GitHub Release `Stardew Server Anxi Panel 0.5.11` 于 `2026-08-22T13:28:49Z` 发布，非 draft/prerelease；从 push 到 Release 约 `12m00s`。Docker Hub、阿里云 ACR、GHCR 的 `0.5.11/latest` 六引用由 promotion workflow 核对为同一 digest。四项资产与正式提交保持相同 size/SHA-256：`migrate-fnos.sh`=`34269/90510768...cbfd`、`repair-junimo-0.3.5.sh`=`14585/13a07708...31cd0e`、`repair-junimo-upgrade.sh`=`8521/4f3c6667...9b4c2`、`run.sh`=`33793/7263bfa3...e130787`。
- 候选与 promotion 各有一条非阻断 GitHub Actions Node.js 20 deprecation annotation；hosted runner 已把受影响 action runtime 强制到 Node.js 24，所有步骤仍成功。它不改变候选内容、digest 或门禁结论；后续应在上游 action 提供对应版本时升级 action 引用，不能通过关闭安全检查压掉提示。
- 正式 runner 的候选和 promotion 均成功完成 owner/trap 清理。此前 v0.5.10 本机中断候选的精确目录 `.agents/anxi-release-candidate-1787235914-13036`（`candidate.tar`、`fixtures.tar`，共 `163578880` bytes）已删除；同 owner 唯一遗留的 exited DinD 容器已按精确名称连同匿名卷删除，复核 container/volume/network 与目录均为 0。该中断链从未作为候选证明。
- 发布后 docs-only `f545c169ded5edb11f8b2a1b1aad289bea77532b` 补齐官网 v0.5.8～v0.5.11 changelog、首页当前版本和 GitHub Release 用户摘要。Docs Portal `32576397782` 的 VitePress build `19s`、deploy `10s` 全绿，Compatibility `32576397780` 用时 `2m29s` 全绿；线上首页与 changelog 均为 200 且正文精确命中。本次 push 没有 Validate release candidate，不移动 `v0.5.11` tag，不重建或改变正式 digest。

# v0.5.10 存档导入真实候选门禁补齐（2026-08-20，released）

## 变更范围、上一版审计与发布边界

- `v0.5.9` 已由自动链从 `0657ff01f1216ffaa9362a800cf228bb4307aa8a` 不可变发布：Compatibility `32363876627`、候选 `32363876626`、自动 Tag `32364681064`、正式提升 `32364702253` 全绿；annotated tag object=`715f68197bf3e2092bb27ec2382bfa222dfdf5c8`，peeled commit 为上述提交，候选 digest=`sha256:f4698348603a34c51f49dae1d69570ecb12899b555786bf57b0ba1f7351d1112`，GitHub Release 于 `2026-08-20T11:38:43Z` 发布且含 4 项部署资产。
- 发布后审计确认 `v0.5.9` 候选跑过既有 maintenance/legacy jobs-cleared 升级专项，但没有执行本文件已经声明的两条真实场景：`saves info <exact-target>` 不可见时 pre-submit fail closed，以及 FIFO 已发送但磁盘零效果后的诊断、snapshot restore 和下一管理员 mutation 自动恢复。旧 tag、digest 与 Release 不移动、不重建；本次以 `v0.5.10` fix-forward 补齐门禁。
- 初始补丁只修改候选升级脚本、真实 Junimo integration test 与对应长期文档/错题本；第一次正式远端候选又暴露既有 runtime-update 成功终态早于 best-effort cleanup warning 落盘的竞态，因此当前范围新增 backend runner/test 的终态原子化修复。公开 API shape、前端 bundle、SQLite schema、Compose、Control/SMAPI/Junimo runtime manifest 仍不变。候选必须从同步干净 `main` 构建完整新镜像，并从当前上一正式版 `v0.5.9` 走真实 Web unhealthy/healthy 升级，不能复用任何旧证明。

## 正式候选、Tag 与提升证据

- 最终候选 `32380002010` 从同步的 `main@9b5a96233331b2050c930658d12eb6e49006f1f0` 自动解析版本 `0.5.10`，约 `11m00s` 全绿：selected code gates `4m00s`、Windows wrapper `17s`、image build/fresh/restart/真实 Web 升级与升级后专项 `5m56s`，随后推送并封存 proof。Compatibility `32380002025` 约 `2m33s` 全绿。proof artifact=`release-candidate-0.5.10-9b5a96233331`、ID=`9411011092`、artifact archive digest=`sha256:2912443cd681d1fe1d65f658ce675faa17d6faa4ca1ef2ebe1239da6dc2268ae`；候选 OCI build date=`2026-08-20T14:25:36Z`，candidate ref 与 proof digest=`sha256:f0887c383d0043934b0023cc150e732f6d514e789df2d81c786297c122dc3bb4`。
- 选择矩阵由 `v0.5.9...9b5a962` 路径差异自动决定：兼容清单、部署脚本/Bash/ShellCheck、后端 test/vet/build、updater/Docker integration、前端全状态回归与 production build 全跑；`backend/internal/games/stardew_junimo` 和 `docs` 有变化，因此 SMAPI 真实下载/Junimo runtime integration 与 website build 也执行；runtime manifest 及其校验器未变，remote artifact verification 按脚本规则跳过。候选镜像再完成 fresh install、未初始化状态、`/health`、`/api/version`、重启、`v0.5.9` unhealthy `failed_rolled_back/health_check_failed`、同候选 healthy apply，以及升级后 exact-target invisible、FIFO no-effect/脱敏诊断/snapshot restore/下一管理员 mutation 自动恢复和既有状态保持。
- 自动 Tag `32381115159` 约 `11s` 全绿；GitHub REST 复核 `v0.5.10` 为 annotated tag object=`c305b3ef0cea220bb27a24f08af140cf45d789fa`，peeled commit 精确为候选 SHA，tag message 固定 candidate workflow 与上述 digest。正式提升 `32381136325` 约 `8m20s` 全绿，只从 proof candidate digest 以 `--preserve-digests` 提升，没有 rebuild；其 `Verify candidate digest and OCI identity`、三仓精确版本、GHCR 精确版 `/health`/`/api/version` smoke、三仓 `latest` 与 Release 创建步骤全部成功。
- 2026-08-22 发布后再次对公开 manifests 逐一核对：Docker Hub、阿里云 ACR、GHCR 的 `0.5.10` 和 `latest` 六引用，以及 GHCR `candidate-0.5.10-9b5a96233331`，均为同一 manifest digest=`sha256:f0887c383d0043934b0023cc150e732f6d514e789df2d81c786297c122dc3bb4`、config digest=`sha256:87c410cfaabe5a15a3ed6a030ee25f7f4295fbe983f0da770cf70a381dfb4034`、`linux/amd64`。从公开 GHCR config blob 独立读取的 OCI labels 为 version=`0.5.10`、revision=`9b5a96233331b2050c930658d12eb6e49006f1f0`、created=`2026-08-20T14:25:36Z`；正式 smoke 工作流对同一 GHCR 精确版实际要求 `/health.status=ok` 且 `/api/version` 的 version/commit 精确相等，该步骤成功。
- latest GitHub Release 于 `2026-08-20T14:44:40Z` 发布，非 draft/prerelease。四项资产与 tag 源文件的 size/SHA-256 逐项一致：`migrate-fnos.sh`=`34269/90510768...cbfd`、`repair-junimo-0.3.5.sh`=`14585/13a07708...31cd0e`、`repair-junimo-upgrade.sh`=`8521/4f3c6667...9b4c2`、`run.sh`=`33793/7263bfa3...e130787`。从 push 到 Release 约 `19m37s`；远端 `main` 发布后仍精确等于候选 commit。
- 正式 runner 的候选和 promotion job 均以 owner label/trap 完成资源清理并成功退出。另一个同 SHA 本地复现候选在用户中断前已生成 image/fresh/restart 并进入 unhealthy Web 回滚，但没有取得终态，不能作为证明。2026-08-22 权限恢复后已删除 `.agents/anxi-release-candidate-1787235914-13036` 中 `candidate.tar` 与 `fixtures.tar`（共 `163578880` bytes），并按 exact owner/name 删除唯一 exited DinD 容器及匿名卷；复核 artifact directory、daemon container/volume/network 均为 0。该本地中断链仍不得记作全绿。

## 本版专项矩阵

| 维度 | 必测场景 | 通过标准 |
| --- | --- | --- |
| 精确可见性 | 升级后的真实 Panel 让 fake Junimo 只具备通用 API、但 staged target 对 runtime 不可见 | exact `saves info` 失败；job 终态失败；`phaseAFifoWriteAttempted/upstreamSubmitted/upstreamConfirmed=false`；maintenance snapshot 恢复 |
| FIFO 零效果 | target 可见，真实 FIFO 只接受一次 `saves import ... --swap-host-to`，但不改存档、pointer 或 pending intent | 只发送一次；复合证据重新分类 `command_failed_no_effect`；受控日志写入且原始平台 ID 不落 journal；snapshot 恢复 |
| 幂等恢复 | 上述两类失败后分别执行下一次管理员 `select-save` | 只清理 exact journal/staged/source/owned token；preimport 永久保留；主文件 hash、活动 pointer 和非目标备份保持；mutation 正常继续 |
| 真实世界链 | Junimo `.125` + Control + 官方 TestClient，用不带 world ID 的 ZIP 做 swap | preview 规范为 canonical runtime identity；finalizer 唯一一次、master=Server、farmhand 全解绑；durable save、重启、床位与 F9/F10 保持；客户端按名称选中 `OriginalOwner` 并睡到次日 |
| runtime-update 终态 | 旧镜像清理被阻塞/失败，正常 apply 与 Panel 重启续作各一条 | cleanup 未完成时 API 不得出现 terminal；warning/log 收集后一次性发布 `succeeded`；清理失败仍成功且 warning 不丢，回滚语义不变 |
| 升级回滚 | `v0.5.9 → v0.5.10` 对同一候选注入 unhealthy，再执行 healthy apply | unhealthy 终态 `failed_rolled_back/health_check_failed` 并恢复旧版；healthy 使用同一候选 digest，SQLite/初始化/非目标容器与 volume 保持；升级后重跑上述候选专项 |
| 资源清理 | 本地真实 runtime、候选 DinD、fresh/restart、两次升级与正式 smoke | 按任务 owner 精确清零 container/network/volume/bind/temp；不使用生产数据、不 prune |

## 候选、失败修复与本地证据

- `TestRealSwapHostRepairsBedManualControlAndSleepsOptIn` 已在任务专属 Linux Go 1.25、真实 Stardew 1.6.15 数据、Junimo `1.5.0-preview.125` 和精确 upstream revision `89abe8e6a07b3aaee1c0b4fad080683b948645d9` 编译的官方 TestClient 上通过，用时 `223.39s`。原始 `HostBedGate_2510107853107169260` 被打成非规范 `HostBedGate` ZIP，preview 规范为独立 canonical `HostBedGate_2510107853108169243`；最终 `original_host_selectable=true`、床位 `(9,8)`、Spring 1 Year 1 推进到 Spring 2 Year 1，owner 标记的容器/网络/卷全部为 0。
- 候选脚本新增升级 Panel 上的 exact-target invisible 与 FIFO no-effect 两条真实 Docker 夹具；Bash 5.2 `bash -n`、ShellCheck `v0.11.0` 和 backend integration 编译门禁通过。定向 preview 单元测试通过，新增真实测试完整运行通过。
- 任务专属 Linux Go 1.25 整仓 `go test ./... -count=1`（Junimo `79.016s`、Web `51.623s`）、`go vet ./...`、`go build ./...` 全绿。不可变候选 fresh/restart、`v0.5.9` Web unhealthy/healthy、升级后专项与资源清零仍是推送前门禁；此处不把定向证据冒充完整候选证明。正式候选、自动 Tag、三仓 digest/`latest`、版本接口和 GitHub Release 结果将在发布完成后回填本节。
- 首次本地完整候选已通过 build、fresh/restart、unhealthy rollback、healthy apply 与既有升级专项，但 invisible 夹具只观察 `150s`，早于产品明确的 `5m` exact-target readiness budget，因 job 仍为 `running` 安全退出 1；候选未推送。夹具改为只对 invisible 场景观察 `420s`（5 分钟产品门禁加 rollback 余量），仍要求最终 `failed` 和完整 pre-submit/snapshot 证据；没有缩短产品超时、接受 running 或跳过断言。修正后 Bash/ShellCheck 再次通过，完整候选必须从新 commit 重跑。
- 第二次本地候选再次通过前述升级链，终态安全失败为 `save_import_maintenance_fifo_unavailable`，没有冒充 exact-target 门禁成功。根因是夹具给 Compose 写了唯一 top-level `name`，而产品 `ComposeExecPipe` 按真实 instance data-dir basename 强制 project=`stardew`，因此 exec 查错项目；候选内同镜像/同脚本的独立容器探针已证明 FIFO、log 和 fake API 进程本身正常。夹具现先在既有 legacy repair 证据完成后精确 down 隔离 DinD 的 `stardew` project，再用无 top-level name 的真实项目契约启动 Phase A runtime；资源 owner/项目清零和所有原业务断言保持不变。Bash/ShellCheck 再次通过，仍须从新 commit 完整重跑。
- 第三次本地候选在新增 Phase A 用例前由既有资源门禁安全失败：两个函数都有相同 `local import_project` 行，前一补丁缺少函数级锚点，误把 empty-Compose fixture 改成 `stardew`，产生已完成 legacy repair 的 orphan container；候选未进入目标场景、未推送。现已按函数名精确恢复 empty-Compose 的任务唯一 project，并只把 Phase A fixture 设为真实 `stardew`；随后用 `rg` 同时核对两处赋值与 Compose `name` 位置，Bash/ShellCheck 再通过。没有放宽资源清零断言，完整候选必须从新 commit 第四次重跑。
- 第四次本地完整候选 `0.5.10@c45f0e09afa591a553a6e36e9efc87971a0c8fc0` 全绿；build date=`2026-08-20T13:35:20Z`，本地 image ID=`sha256:96e6f0e6f7262395533d9ae3b1098d81d7ab285971ec3357f1cdcfd49ac0dcb5`，约 `7m04s`。fresh install/restart、固定上一正式版 `v0.5.9@sha256:f4698348603a34c51f49dae1d69570ecb12899b555786bf57b0ba1f7351d1112`、Web unhealthy rollback、同候选 healthy apply、SQLite/初始化/非目标 game container+volume、Mod update、legacy repair/jobs-cleared，以及升级后 exact-target invisible 和 FIFO no-effect + 下一 admin mutation 全部通过；最终输出明确确认 diagnostic redaction 和自动恢复。正式远端候选仍必须从 push 后的最新同步 `main` 重建，不能提升本地 image。
- 本地收口已精确删除四个预演镜像、Go module/build cache、TestClient volume、外部 upstream source、candidate metadata 与中断候选残留 archive；owner container/network、上述 exact volume/image/temp 路径均为 0。首次外部源码删除在 3 个 Windows read-only Git pack 文件处部分停止，核对 exact 文件名/父目录/属性后只清除这些任务文件的 `ReadOnly` 并续作成功；未 prune 或扩大删除范围。
- 第一次正式远端候选 `32376230460` 在 `Run selected code gates` 安全失败，Windows wrapper/build/GHCR push/artifact 均未开始；失败用例为 `TestRuntimeUpdateApplyImageCleanupFailureIsWarning`。根因是成功链先持久化 terminal `succeeded`，再清理旧镜像并补 warning，测试/API 可在两次写盘之间读到缺 warning 的终态。两个成功入口现都先完成精确 snapshot/旧镜像 best-effort cleanup、汇总 warning/log，再由既有 `finish` 一次性发布终态与审计；清理失败仍不改变成功语义。加强后的用例会阻塞 cleanup，显式断言期间状态非 terminal，释放后 warning 必须存在；任务专属 Linux 连续 `count=20`、整仓 test（Junimo `59.446s`、Web `53.524s`）、vet/build 全绿。必须从新 commit 完整重跑本地与远端候选，失败 run 不重跑、不提升。
- 修复后的本地完整候选 `0.5.10@96e5161255e67727bbfd402eb26f359cd119e9d5` 全绿；build date=`2026-08-20T13:56:35Z`，image ID=`sha256:30c59742cd7760a9cb895205ac8adda3f944f382870c36575069b87b9a8254d0`，约 `6m56s`。除重新通过 fresh/restart、`v0.5.9` unhealthy rollback/同候选 healthy apply、状态保持、legacy repair/jobs-cleared 与两条 Phase A boundary 外，镜像已经包含 runtime-update terminal 原子化修复；随后该本地镜像、两个 fix cache volume、metadata 与 owner 资源精确清零。第一次 push 的 Compatibility `32376230445` 成功；失败候选 `32376230460` 保持失败且无 artifact。下一次 push 必须产生新 workflow/candidate identity，不能重跑旧 run。
- 第二次正式远端候选 `32378153924@2a9a4c9328e150ee3bf637054fedb1fa088e660d` 又在 `Run selected code gates` 安全失败；Windows wrapper、image build、registry login/push、候选 proof artifact 均未开始，同 SHA Compatibility `32378153951` 成功。唯一失败为 `TestBackupMaintenanceSchedulerCapturesConsecutiveGameDaysWithoutListingAPI` 在 2 秒内没有看到 day 1 备份。根因不是生产 scheduler 慢，而是并发测试用 `os.WriteFile` 直接发布最终 `event-1.json`，5 ms 消费者可能读到半写 JSON并按坏事件删除；生产 Control 的 `WriteSaveEvent → ContractFile.WriteJsonAtomic` 只在完整写入后原子 rename 最终 `.json`，不存在该窗口。
- 测试现改用已有 `atomicWriteValidatedJSON` 模拟同一生产发布边界，保留 5 ms scheduler 与 2 秒结果预算；提交前按函数级 diff 复核目标 writer 后，任务专属 Linux Go 1.25 精确用例连续 `count=100` 全绿（11.896s），随后整仓 test 全绿（Junimo `54.936s`、Web `50.676s`）、vet/build 通过。失败候选不重跑、不生成 tag；修复 commit 仍必须完整重跑本地与远端候选，不能把这些代码门禁冒充候选证明。

# v0.5.9 已发布：非规范上传目录与运行时 saveId 统一（2026-08-20；真实门禁补齐进入 v0.5.10）

## 变更清单、受影响链路与当前边界

- `SAVE-IMPORT-RUNTIME-IDENTITY-NORMALIZATION-1` 修复生产 `v0.5.8` 的真实 swap 导入：ZIP 顶层目录/主文件只有 3 个字符，Layer A 主机交换已确认，但 SMAPI runtime saveId 按 `<主文件首段>_<uniqueIDForThisGame>` 解析；Junimo pending intent 仍是旧目录，finalizer wrong-save guard 清 intent 且计数为 0，Panel 因而正确回滚。生产备份和当前主文件/指针一致，未发生角色丢失。
- preview 现在只在上传私有临时树中读取非零十进制世界 ID并 no-replace 重命名目录、主文件和 `_old`；规范后的 `saveName` 贯穿 token、journal、staging、FIFO 和 runtime evidence。XML、平台 ID、角色内容、Control/Junimo runtime 与数据库 schema 不变。
- 本变更影响 backend upload preview/driver tests 与跨端 `saveName` 值语义；前端 shape 不变并继续信任服务端响应。当前未创建 tag、未提升正式镜像；生产热更新必须保留原 Panel image/binary 和完整数据回滚点，不能替代候选门禁。

## 本版专项矩阵

| 维度 | 必测场景 | 通过标准 |
| --- | --- | --- |
| 正常路径 | 无 `_worldId` 的真实中文上传目录执行 swap_to_player | preview 返回 canonical 名；FIFO/intention/runtime saveId 完全一致；finalizer、解绑、durable save、completed 全部成立 |
| 关键边界 | 已规范、含额外 `_`、GBK 目录、可选 `_old`、缺 identity | 已规范零改名；非规范安全收敛；目录/主文件/旧档同步；既有兼容错误不被吞掉 |
| 权限安全 | canonical 名非法、identity 零/非数字/溢出、目标冲突 | token ownership 前拒绝；无覆盖、无路径逃逸、无 FIFO 提交 |
| 幂等/恢复 | preview 重试、commit 重放、Layer A 后故障回滚 | 同 token/operation 唯一；canonical identity 不漂移；preimport 可恢复且不得重复 import |
| 数据完整性 | 原主机、已有 farmhand、物品/关系/房屋与绑定 | 只改临时树路径；成功后原主机成为可选 farmhand，自动解绑为零，XML 内容门禁与重启一致 |
| 资源清理 | preview cancel、失败 token、候选 Docker project | 私有临时树、容器、网络、bind/volume 按 owner 精确清零；生产备份保留到验收完成 |

## 候选前本地证据

- 生产同形态的无后缀中文目录专项通过：preview 返回 canonical runtime identity，目录、主文件、`_old` 同步 no-replace 规范化，旧目录消失；正常 path、显式目录 entry、legacy GBK preview 同组回归通过。
- 任务专属 Linux Go 1.25 整仓 `go test ./... -count=1`、`go vet ./...`、`go build ./...` 全绿；精确 module prefetch 后 `go mod verify` 通过。生产热更新已完成，同一 ZIP 的重新导入仍属本版待验收项，不以单元测试代替。

## 生产热修证据（2026-08-20）

- 修复已以 `0657ff01f1216ffaa9362a800cf228bb4307aa8a` 推送并与 `origin/main` 同步。从该提交构建任务镜像 `anxi-panel-hotfix:0.5.8-0657ff01`，OCI version/commit/build date 为 `0.5.8`/`0657ff01f1216ffaa9362a800cf228bb4307aa8a`/`2026-08-20T11:29:31Z`；本地独立 volume 的 `/health` 与 `/api/version` 冒烟通过。不变更公开版本号，以 commit 区分热修，未创建 tag、未推送正式镜像或 `latest`。
- 62,979,072-byte 镜像包在上传前后的 SHA-256 均为 `a7650ec3d6fb0b96742ee706b6cfd57981163fa7a7f77cd97ce47cc1c7481b43`。首次替换时 HTTP/DB 已就绪，但 Docker healthcheck 仍在 `starting` 窗口，严格脚本退出并自动恢复原 `v0.5.8@8d5fe360c042`；复核原 Panel healthy 后，把 Docker health 改为覆盖 start-period/interval 的独立有界等待，第二次替换成功。
- 生产终态：Compose 精确使用热修镜像，容器 `healthy`、restart count=0，`/health` 包含 DB ok，`/api/version.commit=0657ff01f121...`；SQLite `integrity_check=ok`，实例 `stopped/stopped`，active jobs=0，两份现有 journal 均为 `rolled_back`，unfinished/invalid journal=0，Junimo pending intent=false。原正式镜像仍在服务器，一致 SQLite、Compose/.env、镜像 inspect、manifest 与可执行 rollback 保留在 `/root/.anxi-panel/manual-recovery/panel-hotfix-20260820-0657ff01-retry1`。
- 已在精确 owner/健康/回滚制品断言后删除服务器和本机的冗余镜像 tar；已加载的生产热修镜像和私有回滚目录可恢复。真实重导入尚未执行，因为原始平台 ID 按安全契约不会持久化；用户必须重新上传 ZIP 并提交 ID，之后才能验收 finalizer、自动解绑、durable save 与原主机可选。

# v0.5.9 已发布：终态 no-effect 普通操作自动解锁（2026-08-20；真实门禁补齐进入 v0.5.10）

## 变更清单、受影响链路与当前边界

- `SAVE-IMPORT-TERMINAL-MUTATION-RECOVERY-1` 修复生产 `v0.5.8` 的新现场：job 已 terminal failed、active jobs=0、Phase A 完整证据严格 no-effect 且 maintenance snapshot 已恢复，但 start/select 等普通 mutation 的通用 mutex 在 handler recovery 之前仍返回 `save_import_busy`。Web 现在先认证，仅管理员在通用 busy 检查前调用 existing exact-owner strict recovery；active/ambiguous/effect-bearing、身份冲突、非管理员和未认证请求继续 fail closed。
- maintenance readiness 由通用 `saves` 响应收紧为只读 `saves info <exact-target>`，Junimo 看不到或读不到 staged target 时停在 pre-submit。Phase A 超时会在 Down 前按 offset 有界截取最后 16 KiB server output，经 platform ID/控制字符脱敏和 1024 字符限制写入 journal；日志只用于诊断，不改变 disk composite success proof。
- 生产热修在 Panel pause 窗口内先完成不可变 SQLite/journal/token/存档/指针备份、integrity 和 dry-run，再只清理该 operation 的 transaction、owned token、staged target/source；preimport 与 receipt 保留。备份目录为 `/root/.anxi-panel/manual-recovery/save-import-20260820-98903e3c`，manifest SHA-256=`3930867885e251f4254b79734f6fa897b44cfd0c935d7a0b5c5d71b5e7ee6864`。首次安装 bootstrap cleanup 后 pointer 按产品契约不存在；Panel `/health=ok`、restart count=0、active jobs=0，未替换镜像。
- 影响 backend Web/driver/tests 和长期文档；公开 API/DTO、SQLite migration、Compose、frontend、Control/SMAPI/Junimo runtime manifest 均未改变。代码已随 `0657ff01f121` push 并进入生产热修；未生成通过正式门禁的候选证明、未创建 tag、未提升正式镜像。

## 本版专项矩阵

| 维度 | 必测场景 | 通过标准 |
| --- | --- | --- |
| 正常路径 | 真实 Docker 构造 terminal failed + strict Phase A no-effect + exact owned token，随后管理员 select/start | mutation 先自动 finalize journal/token/owned artifacts，再进入原 handler；preimport 保留，不返回 false busy |
| 关键边界 | Junimo 只响应 saves list 但看不到 exact target；target info 可读后 FIFO 零效果 | 前者 `upstreamSubmitted=false` 安全失败并恢复 maintenance；后者只发送一次 import，停机前留下脱敏 log detail 与完整 no-effect proof |
| 权限安全 | 未认证、普通用户、admin；伪造/冲突 job-token-journal；日志含原始 platform ID | 只有 admin + exact owner 可收敛；其它零删除；raw ID 不进入 journal/support evidence |
| 幂等恢复 | cleanup receipt/journal/token 各窗口中断；重复 admin mutation；Panel 在 no-effect/snapshot restore 后重启 | 可继续到唯一终态，不重发 FIFO、不跨 operation 删除；preimport 永久保留 |
| 数据完整性 | 普通 active pointer 与首次安装 bootstrap pointer；主文件 hash/pending/fingerprint 漂移 | 普通 pointer 保持；bootstrap 按 cleanup contract 删除；任何漂移转 manual recovery |
| 升级回滚 | 当前正式 v0.5.8 → 同一候选 unhealthy/healthy；升级后的新 Panel 跑上述正常路径 | unhealthy 回到 v0.5.8；healthy digest 固定；SQLite/初始化/非目标容器与 volume 保持，升级后专项通过 |
| 资源清理 | E2E Compose project、容器、网络、volume、bind、临时日志/缓存 | 按 task owner 精确清零，不使用生产数据、不 prune |

## 候选前本地证据

- 定向管理员 mutation、未认证零 cleanup、exact target info readiness、Phase A no-effect log/platform ID 脱敏与不完整证据矩阵通过。
- Windows Web 全包首轮仅既有 journal helper 2 秒预算在整包 I/O 下抖动；与 job helper 对齐为 7 秒后 33.337 秒通过。Windows Junimo 全包唯一失败是已记录 NTFS mode=`0666`/Linux `0640` 差异。
- 任务专属 Linux Go 1.25 Junimo 全包 60.493 秒、Linux 整仓 `go test ./... -count=1` 全绿；宿主 `go vet ./...`、`go build ./...` 通过。测试容器、两个缓存 volume、一次性恢复制品与上游源码副本均已精确清零。
- 这些只构成本地代码证据。正式发布前仍必须把本节两条真实 Docker import 专项接入不可变候选，并由同步干净 `main` 完成既有全部候选/升级门禁；不得用生产热修代替候选 proof。

# v0.5.8 Phase A no-effect 生产热修与代码修复正式发布（2026-08-20，released）

## 变更清单、受影响链路与发布边界

- `SAVE-IMPORT-PHASE-A-NO-EFFECT-RECOVERY-1` 修复 `FIFO attempted/submitted + command_failed_no_effect` 被旧一刀切门禁永久留在 manual recovery 的问题。新代码只在持久化 pre/after 证据重新分类仍为 no-effect、当前磁盘 hash/pointer/pending intent 未漂移时恢复 maintenance snapshot，并把同一复合证明传入 existing strict cleanup/finalizer；其它 submitted/unknown 路径不变。
- durable upload 的 exact job binding attach 不再重写 `token.json`，避免 v0.5.7 legacy jobs-cleared 自动恢复自行刷新 mtime、破坏“绑定早于清空审计”的时间证明。公开 API/DTO、SQLite migration、Compose、前端 bundle、Control/SMAPI/Junimo runtime manifest 均未变化。
- 生产 v0.5.7 现场已经通过一次性离线恢复程序收敛，但没有替换 Panel 镜像。不可变修复前备份位于 `/root/.anxi-panel/manual-recovery/save-import-20260820`，manifest SHA-256=`72717504f0b4e6d3af80316cc7ef598f5fa7b9d060606db044313566f182d83e`；修复后 journal/owned token 清零、cleanup receipt 存在、preimport 保留、实例状态 `game_installed`、Panel healthy、游戏 Compose 为空。
- 用户于 2026-08-20 明确要求“推送远端”，授权把本次修复直接提交到 `main` 并推送 `origin/main`。因为改动包含 `backend/**`，该推送会自动启动下一补丁版本候选；只有不可变候选完整通过本节矩阵、fresh/restart、v0.5.7 Web unhealthy/healthy 与升级后复验，并且候选 commit 仍是最新 `origin/main`，才允许自动 Tag 和正式提升。不能把这次生产数据收敛当成镜像发布门禁或候选证明，也不得手工绕过失败门禁。

## 本版专项矩阵

| 维度 | 必测场景 | 通过标准 |
| --- | --- | --- |
| 正常路径 | 真实 Docker 中让 FIFO 命令返回失败且 Junimo 零落盘；当前进程收尾与 Panel 重启各一条 | 返回原 `command_failed`；精确实例 snapshot 恢复；下一次 preview 自动 cleanup 并成功接受新上传 |
| 关键边界 | outcome 标签无证据、after 早于 pre、主文件/pointer 漂移、pending intent 出现、upstream confirmed | 全部保持 `manual_required/import_recovery_required`，journal/token/staged/preimport 零误删，不重放 FIFO |
| 权限与安全 | 普通用户触发上传；owned token/job/journal 身份冲突；多个 unfinished operation | 现有管理员门禁与 exact identity 不变；不因 no-effect 分支放宽身份或跨 operation 删除 |
| 幂等与恢复 | failure evidence 后重启、snapshot restore pending 后重启、cleanup receipt/journal/token 各阶段中断、重复 exact attach | 可续作收敛；exact attach 不改 token mtime；危险删除最多一次，preimport 永久保留 |
| 数据完整性 | 修复前/后主存档双文件、active pointer、实例 SQLite snapshot、bootstrap/source/staged fingerprint | no-effect 主存档与 pointer hash 不变；只删除 transaction-owned 且 fingerprint 未变资源；SQLite integrity 通过 |
| 升级与回滚 | 当前正式 `v0.5.7 → 候选` 的 Web unhealthy/healthy；升级后复现 no-effect + jobs-cleared 现场 | unhealthy 恢复 v0.5.7；healthy 使用同一候选 digest；升级后的真实 Panel 自动恢复且保留非目标容器/volume/初始化状态 |
| 资源清理 | 候选 DinD、fresh/restart、升级/回滚、no-effect 故障夹具 | 任务容器/网络/volume/bind/temp 按 owner 精确归零；不 prune、不使用生产数据或长期凭据 |

## 候选前代码门禁证据

- Windows 精准 Phase A/restart/mtime 专项通过；Windows 包级尝试的唯一 Junimo 失败是已知 NTFS mode=`0666`/Linux `0640` 差异，Web 包通过。
- 任务专属 Linux Go 1.25 中 Junimo 全包 96.726 秒、隔离 Web 全包 35.436 秒通过。首次整仓组合仅既有 `TestJobsAPIPermissionsAndLifecycle` 因 15 秒轮询超时失败；随后 `-p=1` 整仓尝试中 Web 全包 33.146 秒通过，但 Junimo 的既有 `TestReadRequiredRuntimeStatusResolvesHistoricalFailure` 在 20 秒异步等待内未结束。本次 no-effect 用例在各轮均无失败；`go vet ./...`、`go build ./...` 通过，两轮 owner 容器与缓存卷均清零。
- 上述本地证据只证明代码级修复，不作为不可变候选 artifact/digest 证明；正式身份以下述 GitHub 候选 proof、Tag、digest 提升和 Release 为准。

## 正式候选、Tag、提升与 Release 结果

- 正式 commit=`8d5fe360c04240d7ccb9f9ac61ffecaed128627c`。Compatibility `32338102593` 于 `2026-08-20T06:04:53Z..06:07:28Z` 成功；候选 `32338102590` 于 `06:04:53Z..06:14:31Z` 成功，用时 578 秒。artifact=`release-candidate-0.5.8-8d5fe360c042`（ID `9395558561`），previous=`0.5.7`，build date=`2026-08-20T06:05:17Z`，candidate ref=`ghcr.io/anxiyizhi/stardew-server-anxi-panel:candidate-0.5.8-8d5fe360c042`，唯一 digest=`sha256:f192d7840e564fe6c0bba6ab895e1533764c21e53257fcbde3cea01b75d59b66`。
- 候选完成 selected code gates、Windows 包装器解析、fresh install/restart、v0.5.7 真实 Panel Web unhealthy rollback/healthy apply、SQLite/初始化/非目标容器保持、升级后 Mod 更新、legacy Junimo repair，以及既有 `jobs=[] + jobs_cleared + unfinished journal` 存档恢复；候选上传 proof 后按脚本 owner/trap 清理任务容器、网络、volume/bind/temp。前端、Control/SMAPI/Junimo runtime manifest、Compose 和 migration 未变，路径差异对应的非相关长链按既有选择器处理。
- 自动 Tag `32338764800` 于 `06:14:32Z..06:14:51Z` 成功；`v0.5.8` 是 tagger date=`2026-08-20T06:14:45Z` 的 annotated tag，解引用精确指向上述 commit。正式提升 `32338783267` 于 `06:14:47Z..06:16:10Z` 成功，按 candidate proof preserve-digests 提升，没有重新 build。
- Docker Hub、阿里云 ACR、GHCR 的 `0.5.8` 与 `latest` 六引用经发布 workflow 和本地独立 `docker buildx imagetools inspect` 均命中同一 digest。GHCR OCI labels 独立核对为 `version=0.5.8`、`revision=8d5fe360c04240d7ccb9f9ac61ffecaed128627c`、`created=2026-08-20T06:05:17Z`；正式 workflow 的 health/version smoke 成功。
- GitHub Release `Stardew Server Anxi Panel 0.5.8` 于 `2026-08-20T06:16:07Z` 发布，非 draft/prerelease；`migrate-fnos.sh`、`repair-junimo-0.3.5.sh`、`repair-junimo-upgrade.sh`、`run.sh` 四项资产齐全，Release 正文记录同一唯一 digest。本地 proof 下载首次遇到一次 TLS handshake timeout、Actions 汇总首次遇到一次 EOF；均只做固定 run/artifact 的有界只读重试，没有重放 push/候选/Tag/提升。本地 proof 临时目录和两轮 Go 门禁 owner 容器/缓存卷均清零。

## 发布后门禁审计缺口

- 发布后逐项对照本节专项矩阵时确认：候选升级脚本复验的是 maintenance Compose 在 FIFO 前失败的 legacy jobs-cleared 自动恢复；它没有制造 `phaseAFifoWriteAttempted/upstreamSubmitted + command_failed_no_effect`，因此没有在升级后的 v0.5.8 Panel 上覆盖本版新增的 post-FIFO 窄恢复路径。候选成功、生产一次性恢复和 Go driver/Web 回归都不能替代矩阵要求的这条真实 Docker E2E。
- 自动 Tag 与提升已经按现有 workflow 在候选成功后完成；`v0.5.8` tag、Release 和六引用 digest 视为不可变审计事实，不移动、不删除、不重建，也不以 post-release 观察伪装成候选前门禁。下一次涉及该链路的发布前必须先把真实 no-effect + Panel restart + upgraded Web preview cleanup 场景固化到候选脚本并实际通过；在补齐前不得声称本版专项矩阵全部完成。

# v0.5.7 存档导入失败自动恢复正式发布（2026-08-20，released）

## 变更清单与受影响链路

- `SAVE-IMPORT-AUTO-RECOVERY-1`：修复存档导入在 maintenance/staging 已失败、前端又已丢弃原始上传 token 后，unfinished journal 永久阻断启动、选档和再次上传的问题。普通终态 job 只在 journal、job payload/idempotency 与 owned upload 三方身份一致时自动收敛；真正删除继续委托 `stardew_junimo` driver 的 strict cleanup，不在 Web 层重放 Stardew 导入。
- 兼容 v0.5.5 已清空任务中心的真实现场：只有 confirmed journal、owned token 的 job/type/idempotency/token-hash 绑定、绑定记录早于之后成功的 `jobs_cleared` 审计，且当前没有活动 import/recovery job 时，才允许把缺失 job 视为旧终态证明。submitted/unknown、FIFO 已尝试、身份不一致或任何恢复证据模糊时继续 fail closed。
- 现行 `DELETE /api/jobs` 在删除终态任务前先逐实例收敛可安全恢复的存档导入；不能安全恢复时返回 409 并保留 job/journal/token 证据。公开存档上传 DTO、job 类型、SQLite schema、前端 bundle 与运行栈清单不变。
- `scripts/tests/test_release_candidate_upgrade.sh` 在 `v0.5.5 → 候选` 升级后的真实 Panel/Docker 中制造 maintenance Compose 失败，再按旧版顺序删除终态 jobs 并留下更晚的 `jobs_cleared` 审计；下一次普通 preview 必须自动清旧 journal/staged target、保留现有存档和 preimport 备份并返回新 token。
- 候选预取上一正式版及 `registry/nginx/alpine` 固定夹具时，Windows/Linux 包装器对同一精确引用增加最多三次的有界 pull，并在成功后 inspect；一次 token/TLS/EOF 瞬断不再浪费已经通过的 build/fresh 结果，也不降低认证、TLS、引用或 digest 门禁。
- 正式提升不再对 GitHub-hosted `ubuntu-24.04` 运行即时 `apt-get update/install skopeo`；精确 runner image 已由官方软件清单证明预装 `Skopeo 1.13.3`，workflow 改为直接验证二进制与版本，缺失即 fail closed。候选 proof、三仓 digest copy、OCI identity、smoke 和 latest 一致性门禁均不改变。

`上传 preview → commit 202 / 前端丢弃 token → maintenance 失败 → terminal job + unfinished journal → 下一次 preview / 清空任务 → 精确身份恢复 → driver strict cleanup → 新上传可继续`

`v0.5.5 清空任务 → jobs 缺失 + jobs_cleared 审计 + owned token/journal 留存 → 新候选普通 preview → legacy 精确门禁 → 安全收敛`

## 本版专项矩阵

| 维度 | 必测场景 | 通过标准 |
| --- | --- | --- |
| 正常路径 | terminal failed/canceled job 且三方身份一致；现行任务中心清空；恢复后再次 preview | 同一次请求先收敛旧事务再接受新 ZIP；清空任务先恢复后删除；实例回到 stopped 且无旧 journal/token/staged target |
| 关键边界 | job 缺失但有更晚 `jobs_cleared`；审计缺失/更早、binding 缺失、多个 unfinished、活动 import/recovery job | 只放行 v0.5.5 精确旧现场；任一证据不全返回 `save_import_busy` / `import_recovery_required`，零猜测、零删除 |
| 权限与安全 | 普通用户调用存档上传或清空任务；token/hash 与日志/DTO/审计 | 继续由管理员权限门禁；原 bearer token 不持久化到新增引用、不写日志/响应，hash 不可反推；普通用户不能触发恢复写入 |
| 幂等与中断恢复 | filesystem cleanup 后、receipt 后、journal finalize 前或 token removal 前中断；重复 preview/clear | cleanup receipt 使每个阶段可重复收敛；已删除资源视为成功，不重放 import/FIFO，不破坏下次重试 |
| 数据完整性 | 既有选中存档、pointer/instance snapshot、旧 staged target、preimport 备份 | 既有存档内容/hash 与 pointer 保持；旧 staged target 和 unfinished journal 删除；preimport 备份保留；SQLite integrity 通过 |
| 升级与回滚 | 同一候选 `v0.5.5 → 候选` unhealthy/healthy；升级后复现 v0.5.5 `jobs=[] + jobs_cleared` 现场 | unhealthy 为 `failed_rolled_back/health_check_failed` 并恢复 v0.5.5；healthy 使用同一 digest；升级后的真实 Panel 自动恢复并接受新 preview |
| 资源清理 | maintenance 失败、legacy 恢复、候选 DinD、fresh、升级/回滚 | 任务 Compose 容器/网络、候选容器/volume/bind/temp 按 owner 精确归零；不 prune，不使用生产数据或长期凭据 |

## 候选前证据、v0.5.6 失败身份与重试边界

- Windows 定向恢复专项、jobs/storage 包已通过；任务专属 Linux Go 1.25 容器内 Web 全包、全仓 `go test ./... -count=1`、`go vet ./...`、`go build ./...` 已通过，容器与缓存卷精确清零。候选升级 E2E 新增脚本已通过 Git Bash `bash -n` 与 ShellCheck 0.11.0。首次本地候选预演的 build、fresh/restart 已通过，预取 `ghcr.io/...:0.5.5` 时匿名 token 请求单次 EOF；包装器改为同引用有界重试后真实证明 GHCR 第二次、Alpine 第二次成功。第二轮已通过 unhealthy/healthy、Mod 与 Junimo 修复，但新增夹具把项目 `job_<32hex>` 错猜成 UUID，在执行 legacy 恢复前安全停止；正则随后改为生产生成器契约并拆分 journal 断言。
- 最终本地不发布预演在 commit=`3eecf79e82a3d364fad5ce56ef71811b62b181fb` 全部通过：image ID=`sha256:6760eea49bad3363d78ca89f8c82601f44d49251ce8ba5e86c72d491ab30f24d`，完成 fresh/restart、`v0.5.5` 真实 Web unhealthy 回滚与 healthy 升级、SQLite/初始化/非目标容器与 volume/Panel restart、Mod 更新、legacy Junimo 修复，以及升级后真实 maintenance 失败 + v0.5.5 jobs-clear 现场自动恢复；既有存档双文件 hash、preimport 备份与 stopped 状态保持，新 preview 成功。动态候选容器、volume、DinD 目录均为 0；该本地预演不生成候选 artifact、不推 registry、不创建 tag，正式证明仍只能来自同步干净 `main` 的 GitHub workflow。
- 首个正式 `v0.5.6` 候选 commit=`d1e61e33bce067a5816c3b354f7a41709573c664`：Compatibility `32276557209` 成功；候选 `32276557208` 用时 9 分 18 秒成功，artifact=`release-candidate-0.5.6-d1e61e33bce0`、build date=`2026-08-19T16:33:43Z`、digest=`sha256:c29760b02c26ea5f0b08d71cc4523275a4d53e7c0beacb7c538c4607f17cdc6c`，包含本版升级后真实恢复专项；自动 Tag `32277449499` 成功创建不可移动的 annotated `v0.5.6`。
- `v0.5.6` 正式提升未完成，不能视为 Release：首条 `32277471754` 在 `Install registry promotion tool` 静默 20 分钟后、尚未认证 registry 时受控取消；同 tag 重试 `32279520480` 在第二个 runner 同一 `apt-get update -qq` 从 `17:04:55Z` 挂到 45 分钟上限并 timeout。两条 run 的候选 OCI 复核、三仓 version/latest、正式 smoke 和 GitHub Release 都未开始。官方 runner image `ubuntu24/20260810.271` 清单确认已预装 Skopeo，因此修复删除多余 apt 网络安装，只做本机工具探针；旧 tag 不移动、不删除，也不补写虚假 Release。
- 受控重建身份固定为 `v0.5.7`，previous release 仍为 `v0.5.5`；从包含提升工具修复、与 `origin/main` 同步且干净的 `main@f7cedaa31e9db71aa2291c8aa06ea857046caf81` 重新构建不可变候选，完整重跑自动门禁、真实 Web unhealthy/healthy 和升级后存档恢复。没有复用 `v0.5.6` 的 candidate digest，也没有移动或删除旧 tag。
- 相对 `v0.5.5` 修改后端 Web/storage、候选与提升 workflow、升级 E2E 脚本和长期文档，没有 frontend、Control、SMAPI/Junimo runtime manifest、Compose 部署格式、数据库 migration 或长期数据 schema 变化。自动候选必须执行后端 test/vet/build、前端全量状态回归/audit/build、脚本测试/ShellCheck、compatibility、fresh/restart、updater/Docker integration，以及上一正式版真实 Web unhealthy 回滚与 healthy 升级；其它长链的选择/跳过只由 `scripts/run-release-gates.sh` 按 `v0.5.5..candidate SHA` 路径差异判定。
- 用户于 2026-08-20 明确要求“发布”，允许最终本地 `main` 提交并推送 `origin/main`，以及为上述失败后的受控重建手动 dispatch `v0.5.7` 候选。以下新候选 proof、升级专项、自动 Tag、三仓提升、正式与独立 smoke 均已成功，`v0.5.7` 已成为最新正式 Release。

## 正式候选与发布结果

- Compatibility `32284291347` 于 `2026-08-19T17:54:15Z..17:56:46Z` 成功。正式候选 `32284304749` 于 `17:54:24Z..18:03:42Z` 成功；artifact=`release-candidate-0.5.7-f7cedaa31e9d`（ID `9377319437`），build date=`2026-08-19T17:54:58Z`，image ID=`sha256:dca1d4224297e14477387eb8f179f66349c0c2c6624987d08803d2c385a36139`，不可变 candidate digest=`sha256:0b2dbe649fd6ce7acce797e170fec9ad2f1da9f00730afe1bb39b4ea8d586290`。
- 本次自动选择 compatibility contracts、部署脚本与 ShellCheck、后端全量 test/vet/build、updater/Docker integration、前端全部状态回归/audit/production build、官网 build、fresh/restart，以及 `v0.5.5` 真实 Panel Web unhealthy 回滚与 healthy 升级；升级后的 Panel 再次通过 Mod 更新、legacy Junimo 修复和 `jobs=[] + jobs_cleared + unfinished journal` 存档恢复专项。runtime manifest 与 Junimo runtime 输入未变，因此远程制品验证和 Junimo 真实 network/runtime 长链按路径差异自动跳过，没有口头降级。
- 自动 Tag `32285201579` 于 `18:03:44Z..18:04:02Z` 成功；`v0.5.7` 为 annotated tag（tag object `61a3e18a709576ce5d675d376b86e84dd6c4cec3`），解引用精确指向 `f7cedaa31e9db71aa2291c8aa06ea857046caf81`。正式提升 `32285223565` 于 `18:03:59Z..18:05:19Z` 成功，先验证 runner 预装 Skopeo，再以 preserve-digests 提升候选，未重新 build。
- Docker Hub、阿里云 ACR、GHCR 的 `0.5.7` 与 `latest` 六个引用经独立 `docker buildx imagetools inspect` 一次全部命中同一候选 digest。精确 GHCR digest 回拉后的 `/health=ok`、`/api/version.version=0.5.7`、完整 commit 与 OCI `version/revision/created=0.5.7/f7cedaa31e9db71aa2291c8aa06ea857046caf81/2026-08-19T17:54:58Z` 均通过；独立 smoke 容器与 volume 按 owner 清理后计数均为 0。
- GitHub Release `Stardew Server Anxi Panel 0.5.7` 于 `2026-08-19T18:05:16Z` 发布，非 draft/prerelease 且为 latest。`migrate-fnos.sh`、`repair-junimo-0.3.5.sh`、`repair-junimo-upgrade.sh`、`run.sh` 四项资产均为 uploaded，Release API SHA-256 与本地正式提交逐项一致；正文已补齐用户可读恢复边界、三个 workflow、唯一 digest 和 `v0.5.5...v0.5.7` compare。正文编辑未移动 tag、重推镜像或改变资产。
- `v0.5.6` annotated tag 与失败 run 继续作为不可变审计记录保留，但它没有 GitHub Release、正式版本镜像或 `latest`；当前正式身份只认 `v0.5.7`。候选 workflow 与本地独立检查均已清理任务容器、网络、volume/bind/temp；本地下载的 proof 副本也已删除，没有触碰生产数据、长期凭据或其它任务资源。

# v0.5.5 更新检查 / 运行设置交互 / 新建游戏半屏布局正式发布（2026-08-18，released）

## 变更清单与受影响链路

- `PANEL-UPDATE-LATEST-RELEASE-API-1`：更新检查从 GitHub Releases 列表首项改用官方 `/releases/latest` 单对象；候选升级夹具同时兼容上一正式版的列表协议和新候选的 latest 协议。影响 updatecheck、管理员更新检查和真实 Web 升级门禁。
- `FE-SERVER-RUNTIME-SETTINGS-UX-2`：服务器摘要与总览增加同一人数设置入口；人数控件使用 44px 像素风步进；底部拆分“关闭 / 仅保存 / 保存并重启”，重启继续经过在线玩家确认和既有生命周期状态机。影响桌面服务器、总览、移动控制与运行设置共享 hook/dialog，不改变后端 DTO/API。
- 安装与摘要素材修复：seed、Steam、download 三张时间线图标按 image2 重新生成透明 72×72 RGBA，移除重复阴影/裁切；Steam 认证占位继续复用同源；服务器摘要/顶栏头像使用明确的独立图标与裁切。影响前端静态资源和 production bundle，不影响安装状态机。
- `FE-NEW-GAME-MODAL-COMPACT-LAYOUT-2`：新建游戏弹窗改为 1100px 压缩三栏、780px 两栏、560px 单栏及 480/360px 极窄屏细化；不再在半屏直接变成约 2048px 高的单列，也不再使用 `transform:scale()`。影响 CSS/响应式门禁，不改变建档请求或 Junimo。

`GitHub latest → updatecheck cache/API → 管理员更新检查 → dry-run/apply → unhealthy 回滚 / healthy 升级`

`三个运行设置入口 → 共用 dialog/hook → PUT runtime settings → players 刷新 → 可选在线确认 → 既有 restart lifecycle → pending/job/终态`

`Saves 新建游戏 Portal → ngc-modal 内容宽度 → 三栏 / 两栏+底部农场 / 单栏 → 弹窗内部滚动`

## 本版专项矩阵

| 维度 | 场景 | 通过标准 |
| --- | --- | --- |
| 正常路径 | GitHub latest 单对象；仅保存；运行态保存并重启；948px 半屏新建档弹窗 | 更新检查得到最新稳定 SemVer；PUT 与 restart 顺序正确；三入口共用同一状态；半屏保持三栏且操作可达 |
| 关键边界 | draft/prerelease/非 SemVer、latest 请求失败缓存；人数 1/100、低于在线人数、停止态；1100/780/560/480/360px | 非正式版本 fail closed、保留最近成功缓存；边界可保存、警告不伪装硬失败、停止态不偷换 start；各断点无页面横向溢出 |
| 权限安全 | 普通用户入口、管理员重启确认、更新接口异常 schema | 普通用户看不到编辑入口；运行态重启先显示在线玩家确认；坏 schema/HTTP 错误不冒充可升级 |
| 幂等与恢复 | 重复保存/重启点击、PUT 失败、PUT 成功但 restart 失败、更新检查重试 | pending 门禁不重复提交；保存失败零重启；部分成功明确提示配置已保存且可重试重启；缓存语义稳定 |
| 数据完整性 | 运行设置四字段、未知配置字段、现有存档/Mod/安装任务；纯 UI 弹窗重排 | 继续由后端原子写保留非目标字段；不改存档/Mod/SQLite；图标和 CSS 不改变安装 job/Steam 授权；新建档 payload 不变 |
| UI / 可访问性 | 1280/948/840/769/430/390px，键盘、disabled、reduced motion、PNG alpha | 44px 控件、可见焦点和禁用语义成立；三/两/单栏按内容宽度切换；弹窗内部滚动可达；图片不裁切且 console 无 error/warn |
| 升级与回滚 | `v0.5.4 → v0.5.5` 同候选 unhealthy/healthy；升级后复验更新检查、运行设置和新建档弹窗 | unhealthy 为 `failed_rolled_back/health_check_failed` 并恢复 0.5.4；healthy 使用同一 digest；SQLite/初始化/非目标容器与 volume 保持；升级后生产 bundle 通过专项 |
| 资源清理 | 候选 DinD、fresh、升级/回滚、Browser/Vite | 只按任务 owner/精确 project 清理容器、网络、volume、bind/temp；不执行 prune；本地预览不进入正式镜像 |

## 推送前证据与自动门禁选择

- 更新检查专项、Web 包、`go vet/build`、升级脚本语法/ShellCheck 已在对应提交完成；运行设置 19 项前端状态/布局回归和 production build 已通过；新建游戏修复的 `test:responsive-layout`、production build、948×805、840×720、769×500 Browser QA 已通过，页面级横向溢出和 console warn/error 为 0。
- 本次相对 `v0.5.4` 同时修改后端 updatecheck、候选升级脚本、前端页面/CSS/PNG 和公开长期文档。自动候选必须执行后端 test/vet/build、前端全部状态回归/audit/build、脚本测试/ShellCheck、compatibility、fresh/restart、updater/Docker integration 与 `v0.5.4` 真实 Web unhealthy/healthy 升级；网站、Control、SMAPI/Junimo runtime 长链是否跳过只能由 `scripts/run-release-gates.sh` 按路径差异决定。
- 用户在 2026-08-18 明确授权把本地 `main` 提交到 `origin/main`，因此允许触发自动 release-candidate/Tag 链。此记录只表示推送授权与本地前置证据；候选 workflow、不可变 artifact/digest、自动 Tag 和正式提升未完成前不得标记 `v0.5.5` 已发布。

## 正式候选与发布结果

- 正式发布 commit=`a77fbe61e2423cac39233c6796a3024bd146b365`。Compatibility `32127766392` 于 `2026-08-18T10:38:41Z..10:41:20Z` 成功；自动候选 `32127766494` 于 `10:38:41Z..10:47:42Z` 成功，使用上一正式版 `v0.5.4` 完成 fresh/restart、真实 Panel Web unhealthy 回滚与 healthy 升级，并在升级后的 production bundle 复验更新检查、运行设置、最新任务日志与新建游戏布局。
- 自动 Tag `32128518008` 于 `10:47:43Z..10:47:57Z` 成功；正式提升 `32128533342` 于 `10:47:55Z..10:49:24Z` 成功，只提升候选证明中的 digest=`sha256:584a460c90103966394e71c67fe5416822985c9b8246013b5d2cff80400174de`，没有重新 build。annotated `v0.5.5`、三仓精确版本与 `latest`、GitHub Release 均对应同一 commit/digest。
- GitHub Release 正文已在发布后补齐用户可读汇总、候选与正式提升链接、唯一 digest 和完整 compare 链接；只编辑 Release 正文，没有移动 tag、改变资产、重推镜像或触发第二个候选。官网首页/changelog 的 v0.5.4/v0.5.5 补写由 docs-only `95f190d` 部署：首条 Pages run 的 build 成功、deploy runner 长时间排队后取消，仅手动重跑同一 `docs.yml`；`32135628751` build/deploy 成功，线上两页 200 且内容复核通过。该文档链没有 Validate release candidate，不改变本候选证明。

# v0.5.4 安装进度与 SteamCMD 授权复用正式发布（2026-08-18，released）

- 正式发布 commit=`e0b888c4f8bd0d33cb8475b180d6029b76fddac5`。安装向导增加 SMAPI 真实下载进度、阶段标题和持续活动提示；旧 SteamCMD 持久卷迁移后，同一次非强制重装先尝试缓存登录，只有失效才自动回退一次完整登录。下载校验、Steam 登录真值和事务边界没有放宽。
- Compatibility `32108845544` 与候选 `32108845520` 成功；候选完成上一正式版 Web 升级、异常目标自动回滚、fresh/restart 与升级后安装状态复验。自动 Tag `32109534507`、正式提升 `32109555161` 随后成功，正式 digest=`sha256:4d5dbc6faf23cb15aa859deca62022e7e03dd896a7fc4c77086ac805ddb33cb2`。
- GitHub Release `v0.5.4` 正文已在不改变 annotated tag、正式镜像、四项资产和发布时间的前提下补齐安装进度、授权复用、安全边界、候选 run、digest 与 compare 链接；本次元数据修订不重建或重发版本。

# PANEL-UPDATE-LATEST-RELEASE-API-1 候选升级夹具兼容（2026-08-18，released in v0.5.5）

- 面板更新检查迁移到 GitHub `/releases/latest` 单对象接口。候选 DinD 的受控 TLS `api.github.com` 夹具现在精确区分两条路径：`/repos/anxiyizhi/stardew-server-anxi-panel/releases/latest` 为新候选返回单对象，`/repos/anxiyizhi/stardew-server-anxi-panel/releases` 为上一正式版返回兼容数组；其它路径 404。
- 这项双协议夹具是上一正式版真实 Web 升级门禁的一部分，不能只把所有路径统一改成新对象，否则旧 Panel 会在 update check 阶段解析失败；也不能只保留旧数组，否则新候选无法证明生产 latest 契约。
- updatecheck/Web 专项、`go vet/build`、升级脚本 Git Bash `bash -n` 与 ShellCheck 0.11.0 均通过；候选 `32127766494` 已实际验证旧版检查、候选检查、dry-run、unhealthy 回滚、healthy apply 与升级后接口，能力随 `v0.5.5` 正式发布。

# v0.5.3 角色密码 / Nexus 安装更新 / 运行设置聚合正式发布（2026-08-17，released）

## 变更清单与受影响链路

- 下一补丁版本由自动候选从当前正式版 `v0.5.2` 递增为 `0.5.3`；不手工创建、移动或提前推送 tag。用户明确把 `v0.5.2` 以后本地 `main` 上的已提交与未提交功能作为同一版本整体发布，范围固定为本节、专项/回归测试、升级 E2E、公开更新日志和长期接手文档。
- `PLAYER-AUTH-SELF-ENROLL-1` 允许 role 模式在空角色或 waiting 角色存在时启用；首次合法 `!login` 为当前存档角色原子写入不可逆 verifier，管理员代设、清除与重新认领继续可用。凭据按 saveId 存入 `role-passwords.json`，带 initialized marker、Panel/Control 跨进程锁、原子权限写入、legacy 迁移和 store/`.env` 事务回滚；状态明确区分 waiting/configured/error/orphan。Control 升到 `0.3.6`，旧 Compose 自动补齐四个 SAP 环境变量，restart 只强制重建 server。特别感谢群友「石头磊」对密码功能的建议与帮助。
- `NEXUS-EXT-LATEST-1` 把主 Mod 与未满足前置的 Nexus 当前版本加入安装批次，只选择版本完全匹配的文件行并传递 `expectedVersion/nexusFileId`；服务端在 ZIP 下载后、Mods 落盘前复核 manifest 版本。`NEXUS-MOD-ONECLICK-UPDATE-1` 再把扩展升到 `0.1.8`：管理员可在已安装单成员 Nexus Mod 的更新提示旁复用同一批次一键更新，扩展额外发送 `replaceUniqueId`，后端先校验、再备份替换并保留 `config.json` 与启用状态；普通安装批次在当前项成功提交后才打开下一 Nexus 页，避免不同文件捕获状态交叉。缺版本、页面无匹配、错误 UID、聚合包或旧包都 fail closed；`FE-MODS-REFRESH-INSTALLED-1` 同时修复删除后刷新仍显示已安装的单向 merge。特别感谢群友「鹈鹕镇的热心市民」对 Mod 功能的反馈与帮助。
- `SERVER-RUNTIME-MAXPLAYERS-1` 在既有 runtime settings GET/PUT 中增加 `maxPlayers=1~100`，保留旧客户端省略字段和配置文件未知字段；运行中显示 Control 当前生效值，保存只更新重启后配置且不静默重启。桌面摘要、快捷操作和移动控制页共用同一 hook/弹窗；真实 Docker 专项已覆盖 `11 → 配置 12 → 重启后 12`。
- `SUPPORT-BUNDLE-LOG-CONTEXT-2` 为管理员诊断 ZIP 增加有界且脱敏的 Panel、server、steam-auth 与当前实例最近任务日志，保持邀请码、session、token、角色凭据、存档和恢复材料不外泄；导出入口移到诊断页页头。`FE-REFRESH-ACTIONS-AUDIT-1` 让诊断资源独立结算、任务空列表清理旧详情、移动备份兼容 null，并统一 dashboard 刷新的 Promise 契约。
- 受影响链路覆盖 backend Web/driver、Control 源码与内嵌 DLL、runtime stack manifest、Compose 迁移与生命周期、Nexus 浏览器扩展、desktop/mobile frontend、支持包、公开网站和长期数据文件。SQLite schema、游戏存档 XML、server/steam-auth/game/SDK/SMAPI 镜像版本未改变；但运行栈清单、Control 制品、部署环境和角色凭据长期结构已改变，因此必须选择远程制品/Control 真实编译、Junimo/SMAPI 长 integration、Docker/updater、fresh/restart、网站 build 与上一正式版 Web 升级/回滚。代表升级保持唯一 `v0.5.2 → v0.5.3`：`v0.5.2` 正是所有新增长期结构与 Compose 迁移的最老受影响正式输入，不再机械增加更老版本。

## 本版专项矩阵

| 维度 | 场景 | 正式候选断言 |
| --- | --- | --- |
| 正常路径 | role 空列表启用、waiting 首次认领/重复正确登录/管理员代设清除；Nexus 本体+前置最新版安装、单成员 Mod 一键更新；运行人数设置；诊断导出与刷新 | Control/Panel 状态一致且不回显明文；扩展选择精确 file ID，安装/更新后 manifest 匹配；更新保留旧配置与启用状态；配置保存和重启生效分离；ZIP 条目完整；刷新使用本次响应 |
| 关键边界 | 角色错误/孤立/损坏；并发首次认领；`2.9.0/2.9.1/2.9.10`、缺版本、无匹配文件、错误 UID/版本、聚合包与替换失败；人数 `0/1/100/101`；`backups:null`、任务消失和单项诊断失败 | 认证与安装/更新 fail closed；并发只有一个写入者；版本不前缀误匹配；更新校验失败零写入、替换失败恢复旧目录；边界值准确；独立资源成功不被其它失败抹掉，旧详情不残留 |
| 权限与敏感信息 | 普通用户/管理员认证配置、运行设置、远程安装与支持包；恶意 URL/ZIP/日志 | 写操作维持管理员门禁；verifier/key/guard、Cookie、session、token、邀请码、CDN 签名 query、路径和恢复材料不进入 API/日志/ZIP；URL 与 manifest 双重验真 |
| 幂等、并发与恢复 | Panel/Control 同时写 store；active/stale lock；store 成功但 `.env` 失败；扩展安装/更新重复点击与后台恢复；替换中断；刷新重入；Panel 重启 | 单写入者、原子终态和稳定错误码；事务失败完整回滚；同一安装/更新复用原任务；旧 Mod 备份可恢复且临时目录清理；busy 门禁不重复请求；重启后凭据、任务与配置保持 |
| Compose 与运行栈兼容 | v0.5.2 mapping/list Compose；inline role；inline none/global；restart server；Control `0.3.6` 清单/哈希 | 安全结构幂等补四项环境；无法证明的 role 阻止；none/global 自定义 inline 原样保留并告警；只重建 server、steam-auth ID 不变；远程制品和嵌入 DLL 精确一致 |
| 数据完整性 | saveId 隔离、legacy verifier 迁移、未知 runtime settings 字段、旧 Mod 配置/启用状态与错误 ZIP、SQLite/存档/非目标 volume | 不串档、不丢 verifier、不覆盖未知字段；更新保留旧 `config.json` 与 active/disabled 位置；错误 ZIP 落盘前删除且不改 Mods；升级/回滚前后 SQLite、初始化、存档、非目标容器/volume 与长期任务数据保持 |
| 前端与响应式 | 桌面/移动密码与人数弹窗、Mod 删除后刷新、一键更新按钮/禁用提示、诊断/任务/备份刷新，桌面与窄屏 | 两端共用状态流；running/configured 文案准确；删除后清空本体和前置旧安装元数据；更新入口与查看页同组且门禁可解释；无横向溢出、重复入口或 console error/warn |
| 真实交互 | 两个真人客户端首次设置不同角色密码、重复正确、交叉失败、管理员清除后重新认领、Panel 批准、server recreate/Panel 重启；真实 Chrome 0.1.8 登录 Panel/Nexus 安装本体+前置并更新一个已安装 Mod | 用户于 2026-08-17 确认两客户端完整矩阵通过；Chrome 扩展真实捕获 CDN、提交任务并在停止态实例完成安装/更新，manifest/config/启用状态均已核对。自动 C#/Go/DOM 夹具只作为补充证据 |
| 升级与回滚 | `v0.5.2 → v0.5.3` 同一候选 unhealthy 与 healthy，升级后认证/扩展/人数/支持包/刷新专项 | unhealthy 必须 `failed_rolled_back/health_check_failed` 并恢复 0.5.2；healthy 使用同一 digest；断线重连、Panel restart 与受影响长期状态保持；升级后再次通过真实 E2E |
| 资源清理 | 本地验证、候选 DinD、人工 E2E、成功/失败/回滚 | 只按任务 owner/精确 Compose project 清理容器、网络、volume、bind/temp/测试镜像；终态计数为 0，禁止 prune，不触碰生产数据 |

## 发布状态

- 正式提交为 `ede7fa34231600cbfa83050b4ddb6fd650373ae1`，与 `origin/main` 精确同步；上一正式版固定为 `v0.5.2@51fd82459e4ac8afbf362f7ad12c0651937879a1`。Compatibility `32034735122` 与官网部署 `32033542832` 成功；受控重试候选 `32034798704` 自动解析 `0.5.3`，固定 build date=`2026-08-17T13:23:16Z`，10m08s 内完成远程制品、部署脚本、后端 test/vet/build、updater/Docker integration、Junimo/SMAPI 真实 network/runtime、19 项前端回归/build、网站 build、fresh/restart 与 `v0.5.2` Web unhealthy 回滚/healthy 升级。候选 artifact=`release-candidate-0.5.3-ede7fa342316`（ID `9290572787`），不可变引用 digest=`sha256:400ad1e92dc84bc62530d38e08ec2ddb20d4d385ee01dc2b35808d23d91bd1f8`。
- 升级 E2E 先发布同候选 unhealthy 目标并确认 `failed_rolled_back/health_check_failed` 与旧版恢复，再替换为精确健康候选；升级后受控 Mod 更新检查/缓存、legacy `rollback_failed` Junimo 第三次修复、Panel Stop 空 Compose 存档导入、SQLite/初始化/长期状态/非目标容器与 volume、Panel restart 全部通过。候选脚本按唯一 Compose project/owner 清理 DinD 容器、网络、volume、导出文件和临时目录；workflow 成功收口且没有保留测试资源。
- 首次自动候选 workflow `32033542812` 固定提交 `2862d8b3c04768fed0e48acb8c8fdb5d95e30876`，版本解析为 `0.5.3`；全部 selected code gates 和 Windows wrapper 校验通过，但 fresh smoke 的 production bundle 检查仍只在 `ServerControlPage/MobileControlPage` 搜索已经抽到共享懒加载块的 `FarmhouseStack` 隐藏兼容选项，因此在任何候选上传、tag 或正式提升前安全失败。修复把 fresh 与升级后契约都改为精确拉取 `ServerRuntimeSettingsDialog` 块并在该块断言 `hidden` 兼容值，同时保留桌面/移动控制块存在性检查；Git Bash 5.2 `bash -n`、ShellCheck 0.10.0、真实 production chunk 正则与 `test:responsive-layout` 已通过。该失败没有产生候选 artifact、registry 引用、tag、Release 或待清理 Docker 资源；修复提交后必须重新走完整候选，不能重用 run `32033542812`。
- 自动 Tag workflow `32035705749` 成功；`v0.5.3` 是 annotated tag（tag object `76b5d1db5797b47d6b8f16cebbd04e1ba96f9785`），解引用精确指向同步的 `origin/main@ede7fa34231600cbfa83050b4ddb6fd650373ae1`。正式提升 `32035725325` 用时 1m25s，只以 preserve-digests 提升候选、没有 rebuild；Docker Hub、阿里云 ACR、GHCR 的 `0.5.3/latest` 六引用经独立 `docker buildx imagetools inspect` 复核均为同一 digest，OCI `version/revision/created` 分别为 `0.5.3`、完整 commit 与固定 build date。
- 正式 workflow 已从一个精确版本引用完成 health/version 冒烟；GitHub Release `Stardew Server Anxi Panel 0.5.3` 于 `2026-08-17T13:34:50Z` 发布，非 draft/prerelease，四个脚本资产均为 uploaded 且带 SHA-256。Release 正文已补齐聚合功能、两处群友感谢、真人密码矩阵、Chrome 扩展实测、workflow 和 digest；正文编辑不移动 tag、不改变镜像或候选证明。

## 真实 Chrome 0.1.8 扩展证据（2026-08-17，passed）

- 更新链：停止态隔离实例预置 Content Patcher `2.9.0/file_id=153187` 与配置哨兵；管理员点击同一 Mod 卡片的“一键更新”后，扩展提交 `expectedVersion=2.9.1`、`nexusFileId=160463`、`replaceUniqueId=Pathoschild.ContentPatcher`，Panel Job 成功。落盘 manifest 为 `2.9.1`，旧 `config.json` 哨兵与启用状态保持，`.remote-mod-*`、`.partial*`、`.mod-update-backup-*` 为零。
- 缺前置安装链：先删除测试实例中的 Content Patcher 并刷新下载页，搜索 ZIP 目标 Elle's New Barn Animals `1.1.3`。扩展只先打开前置页；前置 `2.9.1/file_id=160463` 被 Panel 接受后才打开目标页，目标独立提交 `1.1.3/file_id=34408`。两个 Job 均成功，最终 manifest 分别为 `Pathoschild.ContentPatcher 2.9.1` 与 `Elle.NewBarnAnimals 1.1.3`，临时制品为零。
- 失败保护：0.1.6 并发页面曾使目标沿用前置 file ID，后端 manifest 版本验真以“期望 1.2.11、实际 2.9.1”安全拒绝，目标未落盘；0.1.7/0.1.8 改为串行后不再交叉。原成功夹具候选实际下载格式为 `.rar`，超出当前只接受 ZIP 的远程安装契约，因此没有放宽 URL/解包安全校验或把手工下载跳转记为成功，改用真实 ZIP 内容包完成成功门禁。

# v0.5.2 MOD-UPDATE-CHECK-1 / FE-MOD-CONFIG-CARDS-1 正式候选与发布结果（2026-08-16，released）

## 变更清单与受影响链路

- 下一补丁版本固定由自动候选从当前正式版 `v0.5.1` 递增为 `0.5.2`；不手工创建或移动 tag。后端新增登录用户 GET `/api/instances/:id/mod-updates`、管理员 POST `/api/instances/:id/mod-updates/check`，Stardew driver 扫描启用/禁用物理 Mod，并通过 SMAPI v4 update service 取得建议版本。成功结果按本地 Mod 清单、实际 SMAPI API/game version 与 Linux platform 指纹缓存 6 小时；上游失败保留上次成功结果，不自动下载或替换 Mod，不读取 Nexus Key。
- 发布前真实 SMAPI 探针发现根级 `apiVersion` 缺失时上游不会返回 `suggestedUpdate`；实现已在候选前修正为优先读取 Control `options.json` 的实际 `apiVersion/gameVersion`，未生成运行时快照时保守使用与 v4 端点匹配的 `4.0.0`，并把这些运行时条件纳入缓存指纹。公开 Content Patcher 探针已得到 HTTPS 更新建议；测试同时锁定请求根级 `apiVersion`、Linux platform 与运行时版本变化后的缓存失效。
- 前端只在 Mod 工作台提供页签数量徽标、页内状态条、可更新筛选与外链；没有系统/浏览器通知。配置页删除无效右侧说明栏，改成全宽双列小图片卡、上下文提示、搜索/排序/状态筛选与原有安全开关；窄屏单列。已安装卡片的删除按钮使用既有红色像素填充，运行态仍灰化禁用。
- 变更影响 backend Web/driver、frontend Mod 页面、响应式/QA fixture、发布升级 E2E 和长期文档；没有数据库 migration、Compose/部署格式、runtime manifest、Control/SMAPI/Junimo 制品或长期业务数据结构变化。因此升级版本矩阵保持唯一 `v0.5.1 → v0.5.2`，但仍必须完成同一候选的 unhealthy 回滚、healthy Web 升级和升级后 Mod 更新专项。路径选择应运行默认全量代码门禁、Stardew Junimo/SMAPI integration、frontend 全回归、website build、fresh/restart 与 updater/Docker；runtime manifest 未变，远程制品校验由脚本自动跳过。

## 本版专项矩阵

| 维度 | 场景 | 正式候选断言 |
| --- | --- | --- |
| 正常路径 | 启用/禁用物理 Mod；SMAPI 返回一个建议更新；页面进入、筛选与配置 | 根级 `apiVersion` + Mod 版本/UpdateKeys 发往受控服务；API 返回 current→latest 与安全 URL；页签徽标、只看可更新、双列图片卡和开关同时存在 |
| 关键边界 | 内置、损坏、缺 UniqueID/版本/UpdateKeys、重复 UniqueID、零 eligible、超过 50 条；运行时 API/game version 变化 | 不合格条目不出网；零 eligible 仍返回确定空数组；请求有界分批；运行时条件变化立即使缓存失效，不继续复用旧建议 |
| 外部服务/恢复 | 首次成功、6 小时同指纹缓存、超时/非 2xx/坏或超大响应、服务恢复 | 成功原子缓存；失败以 `status=error` 保留最后成功结果且不阻塞主 Mod 列表；恢复后管理员强刷可覆盖旧状态 |
| 权限安全 | 匿名 GET、普通用户 GET/POST、管理员 POST；恶意建议 URL | 匿名 401、普通用户可读但强刷 403、管理员强刷成功并审计；只接受带 host 的 HTTP/HTTPS，不外发 Nexus Key、Cookie 或本地路径 |
| 幂等/并发 | 同实例并发进入页面、上传/删除后重查、GET 与 force 交错 | driver 按实例串行；同指纹缓存稳定；清单或运行时指纹变化后重查；前端保留最后结果且无后台轮询风暴 |
| 数据完整性 | Mod 启用/禁用目录、manifest、存档配置、SQLite | 更新检查全程只读 Mod/存档；唯一写入为实例 control 目录的原子缓存；没有自动安装、数据库 schema 或现有 Mod 状态改写 |
| UI/响应式 | 默认桌面、820px 窄屏、服务器运行态、图片缺失 | 配置桌面两列/窄屏一列且零横向溢出；固定图片占位；删除按钮正常态红色填充、运行态保留贴图但 disabled；console 无 error/warn |
| 升级/回滚 | `v0.5.1 → v0.5.2` unhealthy 与 healthy；升级后受控 SMAPI、缓存与前端 bundle | unhealthy 必须 `failed_rolled_back/health_check_failed` 并恢复 0.5.1；healthy 使用同一候选 digest；升级后真实 API 首次 POST + cached GET 与 Mod 页面契约通过 |
| 资源清理 | 本地门禁、候选 DinD、成功/失败/回滚 | HTTP body 有界并关闭；原子临时文件不残留；仅按任务 owner/精确 Compose project 清理容器、网络、volume、bind/temp，禁止 prune |

## 发布状态

- 发布范围已限定为上述产品代码、专项/回归测试、升级 E2E 和相关长期文档；正式发布 commit=`51fd82459e4ac8afbf362f7ad12c0651937879a1`，上一正式版固定为 `v0.5.1@427a295ab905701069b7f710300ba09b6afd21f0`。Compatibility `31945655121` 于 `2026-08-16T11:55:37Z..11:57:33Z` 成功；自动候选 `31945655119` 于 `11:55:37Z..12:04:28Z` 成功，候选 job 实际用时 8 分 48 秒并明确输出 `release gates: all selected gates passed for 0.5.2`。
- 不可变 artifact `release-candidate-0.5.2-51fd82459e4a`（ID `9263297274`）固定 schema=1、previous=`0.5.1`、build date=`2026-08-16T11:55:58Z`、local image ID=`sha256:2d269e4d1a8e26138ea06db77f9851827e8217569eb357f97de1f872e38ba64f`、candidate ref=`ghcr.io/anxiyizhi/stardew-server-anxi-panel:candidate-0.5.2-51fd82459e4a` 与 digest=`sha256:42b5dae824f63d3b5ba44a1f33704a622a62c4d6170225d52a63ac39147aaaed`，workflow attempt=1。路径矩阵因 runtime manifest 未变自动跳过远程制品复验；Junimo/SMAPI 路径和公开文档有变化，因此真实运行集成、网站构建及其余默认代码、Docker/updater、fresh/restart 门禁均实际执行，没有口头跳过。
- 候选前本地门禁：任务专属 Linux Go 1.25 环境中 `go test -p 1 ./... -count=1`（Stardew 60.377s、Web 44.540s）、`go vet ./...`、`go build ./...` 全绿；Node 24 Linux 洁净 `npm ci` 后前端 17 组状态回归、production audit/build 全绿，website production audit/build 全绿；兼容矩阵 validate/version 与 20 项单测通过。升级脚本 `bash -n`、ShellCheck 通过；公开 Content Patcher 请求向真实 SMAPI 返回 `2.9.1`、HTTPS Nexus URL 和零 errors。
- 不发布的 `-AllowDirty` 完整候选预演以 `v0.5.1` 正式 digest 为升级源：fresh/restart、unhealthy `failed_rolled_back/health_check_failed`、healthy Web 升级、SQLite/初始化/Panel 数据/非目标容器与 volume 保持、Panel restart、升级后受控 TLS SMAPI 首次强刷 + cached GET、前端 bundle、既有 Junimo repair 与存档导入回归全部通过。首轮专项断言误用 jq `.cached // true` 把合法 `false` 当缺失而失败，产品响应本身正确；修为 `has("cached")` 后从 fresh 开始完整重跑成功，没有跳过或放宽门禁。
- 正式候选再次从 `v0.5.1` 通过真实 Panel Web update check、dry-run、管理员确认、apply、断线重连与终态恢复：unhealthy 目标得到 `failed_rolled_back/health_check_failed` 并恢复旧版，healthy 目标使用同一候选 digest；SQLite、初始化状态、任务长期数据、非目标游戏容器/volume 和 Panel restart 状态均保持。升级后的 Panel 重新通过受控 TLS SMAPI 首次管理员 POST、缓存 GET、Mod 页面 production bundle、Junimo repair 与存档导入专项。
- 自动 Tag workflow `31946063809` 于 `12:04:30Z..12:04:48Z` 成功；`v0.5.2` 为 annotated tag，tag object=`29436c45b204f053bc338a93a622a628713e87ef`，精确指向正式发布 commit。正式提升 `31946073920` 于 `12:04:44Z..12:06:17Z` 成功，只提升候选证明中的精确 digest，没有 rebuild；Docker Hub、阿里云 ACR、GHCR 的 `0.5.2` 与 `latest` 六个引用均解析为 `sha256:42b5dae824f63d3b5ba44a1f33704a622a62c4d6170225d52a63ac39147aaaed`。
- 从 GHCR 精确 digest 独立回拉的正式镜像首次与 Panel restart 后均为 `/health=ok`；`/api/version` 两次都返回 version=`0.5.2`、完整 commit=`51fd82459e4ac8afbf362f7ad12c0651937879a1`、buildDate=`2026-08-16T11:55:58Z`，未初始化状态保持 `false`。GitHub Release `Stardew Server Anxi Panel 0.5.2` 于 `12:06:14Z` 发布，为 latest、非 draft、非 prerelease。四项资产已与 tag 源逐项一致：`run.sh` 33793 bytes / `7263bfa323b2bf4eb94674bde9c77a57a8b86734c606055c9cdef2fc1e130787`，`migrate-fnos.sh` 34269 / `90510768d6636917fb7f15937a7dce34c34974dd8c9af5451030560eca57cbfd`，`repair-junimo-0.3.5.sh` 14585 / `13a07708d23e02c002c979eef28639bc2fe283a2e5988e228afc0c068f51cd0e`，`repair-junimo-upgrade.sh` 8521 / `4f3c666770b6be77ed51895264f47c940b066d61386b66b3653a858e8929b4c2`。
- 正式 workflows 没有产品或门禁失败；唯一 annotation 是旧 action 被平台强制使用 Node 24 的弃用提醒，不影响结论。候选前真实 SMAPI 探针暴露的根级 `apiVersion` 缺口和本地预演的 jq 布尔断言均在推送前修复并全量重跑，未降低门禁。本地 wrapper/trap、候选预演和发布后 smoke 的任务容器、网络、volume、bind、temp 与 synthetic 镜像均按 owner/OCI identity 精确清理为 0；未执行 prune，历史 `v046-release-20260731` 的非目标资源及生产数据保持未动。

# v0.5.1 HOST-BED-MANUAL-CONTROL-1 正式候选与发布结果（2026-08-16，released）

## 变更清单与受影响链路

- Control `0.3.4 → 0.3.5`：新增主 FarmHouse 床完整性自愈、Junimo 睡眠有界保护、F9 manual-control/NoConnectedClients 协调和 F10 sprite/displayFarmer/hidden/shadow 原子同步；内嵌 DLL SHA-256=`918badd470622cdc5b18df57879bec4f87c2ffd58588f84ccedda13fd6bd3605`。server/auth/game/SDK/SMAPI 与 Compose/数据库格式不变。
- stardew_junimo 的 swap 激活新增 `hostBed` 复合证据、`host_bed_missing` 和后置失败整树回滚；容器内 Junimo API 统一固定 8080。现有实例状态 `statusSource` 新增只读 `hostBed/hostControl`，没有新增写路由或 Web XML 特判。
- 变更影响 Control/runtime stack、存档导入 activation/durable/rollback、Junimo 真实运行与实例状态，因此正式候选选择后端全量 test/vet/build、Control 契约与真实程序集编译、runtime manifest/远程制品、Junimo/SMAPI 长 integration、Docker/updater、fresh/restart、上一正式版 Web unhealthy 回滚/healthy 升级和升级后本专项 E2E；公开文档变化同时选择 website build。候选没有口头跳过受影响长链，无关历史人工故障不重复注入。

## 本版专项矩阵

| 维度 | 场景 | 断言/证据 |
| --- | --- | --- |
| 正常路径 | 0 级空主屋；1/2/3 级缺床；真实 swap/save/restart/sleep | 实际 map `DefaultBedPosition`；0=Single、1..3=Double；恰好一床；GameLoop.Saved 后重启保持；实际进入次日 |
| 关键边界 | 已有床；其它家具；Farm/cabin 有床但主屋无床；地图属性/等级/类型异常 | 已有床完全零修改；其它家具与 cabin 不动；只检查 master 主 FarmHouse；无法证明布局返回 `host_bed_missing` |
| 权限安全 | Web 导入、平台 ID、status/log | handler 不解析 XML；敏感玩家关联标识不进入新增日志/投影；仍沿用管理员导入权限 |
| 幂等/恢复 | 重复导入/启动/自愈；activation 或 durable-save 任一步失败 | 不增加第二张床；停止运行时并恢复 preimport 整树、pointer、Mod profile、instance snapshot；材料不完整时 manual_required |
| 数据完整性 | current、`_old`、SaveGameInfo、原/新主机与 farmhand 非目标字段 | SHA/fingerprint 精确恢复；角色、背包、任务、关系、邮件及其它家具不由修复代码改写；源夹具 hash 不变 |
| VNC/无人值守 | 零客户端 F9、方向键、再次 F9/租约到期；F10 多次、warp/load/day | manual 时不被 NoClients 暂停且可移动；退出后自动暂停恢复；人物与 shadow 无半状态；10 分钟无人租约防止长期流逝 |
| 睡眠故障 | 主屋缺床；合法床自动睡眠 | 每故障 episode 一次 `host_bed_missing`，不无限 sleeping-in-place；合法床每日动作最多 4，真实客户端从 spring 1/Y1 进入 spring 2/Y1 |
| 资源清理 | 成功与各次诊断失败 | 仅按 `sap.task=host-bed-20260816`/精确 Compose project 清理容器、网络、volume、bind/temp；禁止 prune，生产数据只读/隔离 |

## 本地验证记录

- Control 已以只读真实 `/game` 和标准 `dotnet build -c Release /p:GamePath=/game /p:EnableModDeploy=false` 编译通过（0 warning/0 error），产物大小 195072 bytes，SHA 与 manifest 一致；纯策略 C# 契约通过。任务专属 Linux Go 1.25 容器中 `go test -p 1 ./... -count=1`、`go vet ./...`、`go build ./...` 全部通过；全量首轮发现 Web rendering 两条旧 fixture 仍误用宿主映射 18080，改为容器内固定 8080 后先定向、再同参数全量通过，没有放宽产品实现。
- `TestRealSwapHostRepairsBedManualControlAndSleepsOptIn` 在 `sdvd/server:1.5.0-preview.125` 真实 Docker runtime 通过，用时 180.16s：独立身份存档 swap 后床位为运行地图给出的 `(9,8)`，SaveLoaded、save-now/GameLoop.Saved、restart、Xvnc Unix-socket F9/F10/方向键、NoClients 恢复，以及独立官方测试客户端实际睡眠跨日全部成功；日志无 timeout/force-day-end。该坐标只作为实测证据，产品实现和测试期望均未写死。
- 收口按 `sap.task=host-bed-20260816` 复核容器/network 均为 0，并逐一验证 owner label 后删除 7 个精确 cache/output volume；13 个 `anxihostbed<timestamp>` 失败诊断目录在确认位于系统 Temp、名称匹配且非 reparse point 后送入回收站。未执行 prune、未删除源夹具或生产数据。
- 正式发布 commit=`427a295ab905701069b7f710300ba09b6afd21f0`。Compatibility `31942102879` 于 `10:36:19Z..10:38:32Z` 成功；候选 `31942102917` 于 `10:36:19Z..10:48:04Z` 成功，job 实际 11m42s，日志明确 `release gates: all selected gates passed for 0.5.1`。不可变 artifact `release-candidate-0.5.1-427a295ab905`（ID `9262422503`）固定 build date=`2026-08-16T10:36:46Z`、candidate ref=`ghcr.io/anxiyizhi/stardew-server-anxi-panel:candidate-0.5.1-427a295ab905` 与 digest=`sha256:70c1967eb36827dbbf78ec3c11683c994814961dcf6673ae365ec4f43c6c25a5`。
- 自动 Tag workflow `31942624901` 于 `10:48:06Z..10:48:19Z` 成功；`v0.5.1` 为 annotated tag，tag object=`6cb0d50b189bc68dd999c39fbabe181b61dd4f8f`，精确指向上述 commit。正式提升 `31942631860` 于 `10:48:16Z..10:49:37Z` 成功，只提升候选证明 digest，没有 rebuild；三仓精确版本与 `latest` 六引用通过同 digest 校验，并从一个正式引用回拉完成 health/version smoke。
- GitHub Release `Stardew Server Anxi Panel 0.5.1` 于 `2026-08-16T10:49:34Z` 发布，为非 draft、非 prerelease；四项既有发布资产上传成功。候选/兼容/Tag/提升没有产品失败；唯一 annotation 是 Actions 将旧 Node 20 action 强制运行于 Node 24 的弃用提醒，不影响门禁结论。工作树本地首轮 Web fixture 端口断言失败已在候选前修正并以全量重跑闭环，不能算作候选放宽。
- 资源终态：本地 `sap.task=host-bed-20260816` 容器/network/volume 均为 0，失败诊断目录已送回收站；GitHub 候选由 workflow 自身完成 builder/临时资源清理。发布后证据只更新长期文档，不移动 `v0.5.1`、不改变 digest，也不重新触发已发布候选门禁。

# v0.5.0 正式候选与发布结果（2026-08-16，released）

## 聚合变更清单与受影响链路

- 本次明确覆盖自动 patch 版本，目标固定为 `0.5.0`，当前上一正式版固定为 `0.4.19`；推送 `main` 后如果路径触发默认 `0.4.20` 候选，必须由同一 commit 的手动 `version=0.5.0`、`previous_version=0.4.19` 候选取代，不得创建、移动或手工补推 tag。
- `PLAYER-AUTH-MODES-1`：v0.4.19 已发布 none/global/role 玩家加入保护，v0.5.0 的用户可见汇总和回归仍必须完整包含该能力：角色密码按稳定角色 ID 保存为 HMAC verifier，配置使用 revision/原子替换，Control 对 Junimo `TryAuthenticate` 只改写输入并继续复用原 attempts/timeout/lobby/warp，runtime patch/revision 缺失或不一致时 fail closed。旧全服 `SERVER_PASSWORD` 升级为 global，role 模式下旧密码 API 返回 409 且不得泄露内部 guard。
- `SAVE-IMPORT-STRICT-OFFLINE-PROBE-1`：存档导入的每个破坏性边界都使用无 cache 的 strict Compose 探针，只接受无 server 或全部 `exited/dead`；命令失败、截断、坏/空 JSON、缺字段、未知或过渡状态全部 fail closed，并始终使用数据库权威 `DataDir`。
- `SAVE-IMPORT-MAINTENANCE-DURABILITY-1`：maintenance、SQLite 精确快照、Compose Up/Down、FIFO attempt 和恢复阶段全部 write-ahead；NULL、空值和原始 payload 可精确恢复，无法证明未写 FIFO 或已完整回滚时保留 `import_recovery_required`，不做第二次危险动作。
- `SAVE-IMPORT-TOKEN-CLEANUP-RECOVERY-1`：primary job 通过 `BeforeRun` 与 journal/token 精确绑定；成功 token 使用 tombstone，取消使用带 fingerprint 的 schema 1 cleanup plan、持久 removal 子阶段和 0600 receipt，使 staged/bootstrap 危险删除最多一次并可在 Panel restart 后继续证明。
- `FE-PLAYER-LAST-SEEN-SEMANTICS-1`：玩家 API 的 `lastSeen` 只来自真实在线时间 `last_online_at`，不再把面板扫描到离线存档角色的时间伪装成最近在线；前端表头同步改为“在线 / 最近活动”，API shape 与数据库 schema 不变。
- `HOST-FARMHOUSE-PRESERVE-1`：Control `0.3.3 → 0.3.4` 以 Harmony prefix 默认跳过 JunimoServer `.125` 的 `HostFarmhouseUpgradeGuard.ResetHostFarmhouseToLevelZero()`，不携带上游源码且没有开关；Control options/status 和 Panel runtime gate 对角色认证与农舍两个精确 patch availability 均 fail closed。内嵌 DLL SHA-256 固定为 `5ab089610b0ae2b9368c0abd87165b98373206a80270ac58f237d29a8a13b982`。
- 受影响路径横跨后端、Control 源/DLL、runtime stack manifest、前端和长期文档，因此统一门禁必须选择后端全量 test/vet/build、Docker/updater integration、远程制品、Junimo/SMAPI/Control 真实长链、前端全部状态回归/audit/production build、网站 build、兼容清单、部署脚本、fresh install/restart，以及真实 Web unhealthy 回滚与 healthy 升级；不得按单个子任务的“不变路径”跳过聚合后的必跑项。

## v0.5.0 专项矩阵

| 维度 | 必测场景 | 正式候选门禁 |
| --- | --- | --- |
| 正常路径 | none/global/两个角色独立密码与 Panel 批准；离线存档上传、primary job 精确绑定、maintenance/FIFO/cleanup；房屋等级 0 与 2；从未在线与曾在线角色 | 角色交叉登录隔离且 runtime revision 一致；上传从受理到 durable save/cleanup 完整终态；等级 2 经真实 SaveLoaded、save-now、Panel restart 后仍为 2；save-only 角色不返回伪造时间，曾在线角色保留最后在线时间 |
| 关键边界 | 未配置角色、revision 冲突、旧 API/损坏 guard；Compose `running/Up/restarting/paused/created/removing`、坏/截断/null JSON；journal/token/job 不匹配；fingerprint/pointer 漂移；Control 类型/方法/owner/options 缺失 | 全部 fail closed 且在认证放行、ownership、FIFO、删除或读档前停止；不得回退开放认证、cache 探针、第二个 primary runner、无证明 cleanup 或旧 level-zero 行为 |
| 权限安全 | 管理员提交/取消/升级；token owner、路径与 symlink；密码、平台 ID、原始 Docker 输出 | 管理员与实例边界不变；只操作数据库权威目录和已验 fingerprint；日志/API/证明不泄露凭据或玩家关联标识 |
| 幂等/恢复 | attach 三写崩溃窗、maintenance start/down/restore 窗口、FIFO 结果模糊、cleanup 各子阶段、并发 cancel、Panel restart | exact recovery job 收敛；危险动作最多一次；不能证明安全时持久保留 manual recovery，而不是清旗、删 token 或重试 FIFO |
| 数据完整性 | SQLite snapshot 的 NULL/空/raw、preimport/finished save、主存档双 XML、房屋 XML、非目标游戏容器/volume | exact restore；正式/完成存档不属于 token cleanup；除目标字段和预期事务外保持不变；失败/回滚恢复上一版运行状态和长期数据 |
| 资源清理 | 成功、受控失败、取消、崩溃恢复、Control 真实测试和候选 DinD | staged/bootstrap/receipt/journal 按状态清理，证明不足材料保留；测试容器、网络、volume、bind 和临时制品按 owner 精确归零 |
| 升级/回滚 | `v0.4.19 → 0.5.0` Web unhealthy 与 healthy；Control 0.3.3→0.3.4；升级后重跑受影响 E2E | unhealthy 必须 `failed_rolled_back/health_check_failed` 并恢复 v0.4.19；healthy 只使用同一候选 digest，升级后的 Panel 再验角色认证回归、存档导入、lastSeen 和房屋等级保持 |
| 最老受影响版本 | 存档导入事务首次正式存在的 `v0.4.11 → 0.5.0` 代表升级 | 因 journal/maintenance/cleanup 长期结构变化，增加一条最老受影响版本真实 Web 代表升级；其它更老版本不机械重复 |
| 已知取舍 | 旧 #346 对历史 farmhand 镜像污染的 level-zero 自愈被禁用 | 只承诺不再强制归零并保留当前存档值；不把本版描述成自动修复已经污染的旧存档，导入存档不增加专门分支 |

## 发布状态

- 最终发布 commit=`9b18dd3fe5192692548bf11a85010dd35303da93`。Compatibility `31899107019` 于 `2026-08-15T17:43:12Z..17:45:07Z` 成功；显式候选 `31899107629` 使用 `version=0.5.0`、`previous_version=0.4.19`，实际 job 于 `17:48:17Z..17:59:19Z` 用时 11 分 02 秒并成功。自动 push 产生的同 commit patch 候选 `31899107063` 被取消，未形成第二个候选身份。
- 不可变 artifact `release-candidate-0.5.0-9b18dd3fe519`（ID `9250770198`）固定 schema=1、previous=`0.4.19`、build date=`2026-08-15T17:48:42Z`、local image ID=`sha256:55693feb011ae79261ad76dfcb3540ec9ba870fc3eb3c83f274717bc652b0e71`、candidate ref=`ghcr.io/anxiyizhi/stardew-server-anxi-panel:candidate-0.5.0-9b18dd3fe519` 和 digest=`sha256:92ea973d55c1f63b4eb356652d491f8d37ef5f69112df1f19c161e4b0e9b611a`。候选没有重新使用首轮失败的证明。
- 路径矩阵没有跳过本版受影响链：runtime manifest 变化触发远程制品验证，Junimo 实现变化触发 SMAPI 真实下载与运行栈 integration，docs 变化触发网站 build；兼容清单、部署脚本/ShellCheck、后端全量 test/vet/build、updater/Docker integration、前端全部状态回归/audit/build、fresh install/restart、`v0.4.19` Web unhealthy 回滚/healthy 升级和升级后受影响 E2E 全部通过。存档长期结构最老受影响的 `v0.4.11 → 0.5.0` 已在本地 release-candidate 预演中用同一产品代码完成代表 Web 升级；最终 commit 只在其后收紧取消时序测试和发布记录，没有改变运行产品逻辑。
- 自动 Tag workflow `31899867310` 于 `17:59:21Z..17:59:35Z` 成功；annotated tag object=`8a51ad3638f5057ebca1f1555ff652ba378e1c87`，tagger time=`2026-08-15T17:59:30Z`，解引用精确等于发布 commit，tag message 固定 candidate workflow 与 digest。正式提升 `31899874927` 于 `17:59:32Z..18:01:05Z` 成功，只用 `skopeo --all --preserve-digests` 提升 proof digest，没有 rebuild。
- Docker Hub、阿里云 ACR、GHCR 的 `0.5.0` 与 `latest` 六引用已独立复核，全部精确等于 `sha256:92ea973d55c1f63b4eb356652d491f8d37ef5f69112df1f19c161e4b0e9b611a`。GitHub Release `Stardew Server Anxi Panel 0.5.0` 于 `2026-08-15T18:01:02Z` 发布，为 latest、非 draft、非 prerelease；四项资产与 tag 源逐字节一致：`run.sh`=`7263bfa323b2bf4eb94674bde9c77a57a8b86734c606055c9cdef2fc1e130787`、`migrate-fnos.sh`=`90510768d6636917fb7f15937a7dce34c34974dd8c9af5451030560eca57cbfd`、`repair-junimo-0.3.5.sh`=`13a07708d23e02c002c979eef28639bc2fe283a2e5988e228afc0c068f51cd0e`、`repair-junimo-upgrade.sh`=`4f3c666770b6be77ed51895264f47c940b066d61386b66b3653a858e8929b4c2`。
- GitHub Release 正文已在不改变 tag、digest、发布时间和四项资产的前提下补齐用户可读汇总：v0.4.19 的角色独立密码/全服模式/旧配置兼容，以及 v0.5.0 的存档恢复、真实最近活动、农舍保持、升级和 digest 证据。官网首页/changelog 已由发布后 docs-only 提交同步并完成线上验收。
- 从 GHCR 正式 digest 拉取的独立空数据卷容器在首次启动和 restart 后均返回 `health=ok`、version=`0.5.0`、完整 commit、build date=`2026-08-15T17:48:42Z`、setup initialized=false；owner 容器与 volume 终态为 0。候选证明与 Release 资产临时目录已逐文件和空目录精确清理，未执行 prune，也未触碰生产数据或既有非目标 Docker 资源。
- 首个显式 `0.5.0` 候选 `31897973357` 在 selected code gates 期间主动取消，没有构建或推送候选镜像；必跑 Compatibility `31897972627` 的 Linux 默认全量后端测试暴露 `TestControlRuntimeContextCancellationDoesNotCleanup` 仍用 30ms timeout 同时承担“进入 starting”和“取消”的竞态，在共享 runner 负载下先取消、后读取到初始 stopped。该轮 `ComposeDown` 次数仍为 0，说明不是产品误清理。测试改为先观察精确 `control_runtime_starting` phase、再通过 manager 显式取消；目标测试连续 50 次通过，Linux Go 1.25 默认全量 `go test ./... -count=1`、`go vet ./...`、`go build ./...` 全部通过。最终 Compatibility 与候选已重新跑默认全量并成功，未把局部复验当作候选证明。
- 发布后官网与证据提交 `242453ab631750689de467625346b6b0fb97c206` 只触发 Pages `31900873468`（build `18:20:53Z..18:21:11Z`、deploy `18:21:14Z..18:21:28Z`）和 Compatibility `31900873542`（validate `18:20:53Z..18:22:48Z`），两者均成功且没有候选 workflow。公开 Pages 在 1440×900/390×844 从首页真实点击到 `/changelog.html`，v0.5.0/v0.4.19/v0.4.18 顺序，以及角色独立密码、旧 `SERVER_PASSWORD`、存档恢复、真实最近活动与农舍保持正文均命中；root/body 无横向溢出，console warn/error=0。该 docs-only 发布没有重建镜像、创建或移动 tag，也没有改变 `sha256:92ea973d55c1f63b4eb356652d491f8d37ef5f69112df1f19c161e4b0e9b611a`。

# v0.4.19 正式发布基线（2026-08-15，released）

- `v0.4.19` 由候选 workflow `31892497427` 验证并从 commit `c289ccbdffdb8a6ecbeb4a5080b7db1040d2d0ee` 自动创建 annotated tag；正式候选 digest 为 `sha256:2df4df07362bb34e5ce4e97e1a0f3415f2366677d319ca4d01e9a5e946210d17`。本版加入 none/global/role 三种玩家认证模式、按稳定角色 ID 保存的独立密码与 Control fail-closed runtime patch；它的 Release、annotated tag 与三仓精确 `0.4.19` 保持不可变，并已作为 v0.5.0 healthy/unhealthy Web 升级的唯一上一正式版基线。三仓 `latest` 在 v0.5.0 正式提升成功后才统一前移到新 digest。

# HOST-FARMHOUSE-PRESERVE-1 候选前门禁记录（2026-08-16，released in v0.5.0）

## 变更清单与受影响链路

- Control `0.3.3 → 0.3.4`，新增对 JunimoServer `.125` 精确方法 `HostFarmhouseUpgradeGuard.ResetHostFarmhouseToLevelZero()` 的默认 Harmony skip；server/auth image、Compose/部署格式、数据库、公开 API 和前端 bundle 不变。runtime stack manifest 与内嵌 Control DLL/SHA 已更新，最终候选因此选择并通过 Control/runtime 真实长链，没有按“server 镜像没变”跳过。
- Control options/status 新增补丁 availability/detail；Panel runtime gate 只有在 DLL/版本匹配且 availability=true 时才 ready，缺字段/false 使用 `control_runtime_host_farmhouse_patch_unavailable` 停服。该兼容层不携带上游源码，也没有运行期开关。
- 本地已完成 .NET 6 契约测试、只读真实游戏程序集标准构建，以及精确 `.125` Docker `SaveLoaded → save-now → GameLoop.Saved`：任务副本 `houseUpgradeLevel=2` 保存后仍为 2，owner 容器/卷清零。任务专属 Linux Go 1.25 容器的后端全量 test/vet/build 全绿并精确删除两个 cache volume；Windows 全量仅有既有 NTFS mode 差异。该证据验证当前工作树，不替代不可变候选和上一正式版 Web 升级证明。

## 本任务专项矩阵

| 维度 | 场景 | 下一正式候选要求 |
| --- | --- | --- |
| 正常路径 | 推荐 `.125`、Control 0.3.4、已有主机房屋等级 1/2/3 | 启动门禁 ready；至少等级 2 经真实读档、Panel restart/手动再启动、`save-now` 后等级与室内关键结构保持 |
| 签名/安全 | Junimo 类型或方法缺失、返回/参数形态变化、Harmony owner 未登记、options 缺字段/false | 一律 `control_runtime_host_farmhouse_patch_unavailable` 并停止 server，不允许继续读档或降级为旧归零逻辑 |
| 回归边界 | 等级 0 的新存档、正常停启、邀请/命令、Mod profile、新建存档 | 既有链继续通过；等级 0 不产生额外 XML 改写。导入存档本轮没有专门分支，按现有全量回归覆盖 |
| 数据完整性 | 主存档、SaveGameInfo、Control options/status、SQLite、非目标实例/卷 | 非房屋目标字节/状态按既有规则保持；测试只操作任务副本，失败不得删除或覆盖源存档 |
| 升级/回滚 | 当前上一正式版通过真实 Panel Web API 升到候选；同候选 unhealthy 注入 | Control-only apply 必须物化 0.3.4 并在升级后实测；unhealthy 必须恢复上一正式版 Control/状态，不能留下混合 DLL/manifest |
| 历史风险 | 已被旧 #346 farmhand 镜像污染的存档 | 明确不再自动 level-zero 自愈；不得把“保留当前存档值”描述成自动修复历史污染 |

# SAVE-IMPORT-TOKEN-CLEANUP-RECOVERY-1 候选前门禁记录（2026-08-15，released in v0.5.0）

## 变更清单与受影响链路

- 后端新增 job `BeforeRun` 持久绑定门、journal job identity/cleanup plan/substage、pending-token exact identity、cancel receipt 与 succeeded tombstone；公开 API、SQLite migration、Compose/runtime manifest、Control/SMAPI 制品、部署格式和前端 bundle 不变。
- primary job 只由 `stardew_import_save_and_start + instance + save-import:<operationId>` 标识；runner 在 journal/token/ready 三次写入后才启动。submitted 后的观测 runner 使用独立 recovery job type，不能形成第二个 primary identity。
- cancel 删除顺序固定为完整只读 proof → 持久 cleanup intent/substage → filesystem completed/canceled → 0600 receipt → journal finalize → exact token delete；preimport、completed journal、正式存档不属于 succeeded token metadata cleanup。

## 本任务专项矩阵

- 身份/崩溃：job 创建后 journal/token 均未 attach、journal 后 token 前、token 后 runner release 前、attach 写失败 runner=0、Panel restart exact recovery、missing/mismatched job recovery required。
- cleanup/幂等：staged 与 bootstrap fingerprint 漂移、pointer 漂移、未知 schema/stage/缺关键字段零删除；每个 removal 子阶段重启；filesystem complete 后 token 删除失败、journal 已不存在、两个并发 cancel 最多一次危险删除。
- 成功生命周期：succeeded tombstone 保留 exact result；completed journal、preimport、正式存档不删除；既有首次 `game_installed` 完整导入、单次 FIFO、no-replace、strict stop 与 durable save 继续回归。
- 实际验证：Windows jobs 全包、save-import/maintenance/Phase A/transaction 专项、pending-upload/save Web 专项及 Web 全包通过；Stardew Windows 全包唯一失败仍是已记录的 NTFS mode=`0666`/Linux 期望 `0640`。任务专属 `golang:1.25-alpine` 容器以只读仓库 bind 和两个独立 cache volume 完成 `go test ./... -count=1`，全部包通过；宿主 `go vet ./...`、`go build ./...` 通过。任务容器自动删除，两个精确 label volume 删除后 owner resource=0，未使用任何 prune。
- pre-candidate 阶段只运行本地/任务专属 Linux test、vet、build 与编码/差异审计；最终 v0.5.0 候选已在升级得到的新 Panel 上执行受影响的 upload/recovery/cancel 回归并精确清理任务资源，随后才自动 tag 和正式提升。

# SAVE-IMPORT-MAINTENANCE-DURABILITY-1 候选前门禁记录（2026-08-15，released in v0.5.0）

## 变更清单与受影响链路

- 仅修改 save-import maintenance/Phase A/recovery、storage 精确快照契约和测试/文档；公开 API、Compose 模板、镜像引用、runtime manifest、Control/SMAPI 制品、数据库 schema 与前端 bundle 不因本任务变化。
- 启动门禁固定为 0600 权威快照 journal → maintenance phase DB → `MaintenanceStarted=true/start_intent` journal → ComposeUp；成功后补 `compose_up_returned` 与 `runtime_ready_persisted`。失败恢复固定为 ComposeDown(0) → `ComposePsStrict` fresh stop → `MaintenanceStarted=false/snapshot_restore_pending` → exact storage restore → `snapshot_restored`。
- Phase A 在 FIFO 前持久化 attempt 位；pre-submit 零 attempt 可自动停机恢复，attempt 后缺 submitted receipt 一律停机并保留 manual recovery/ownership，禁止二次 FIFO、staged/token cleanup。

## 本任务专项矩阵与结果

- 正常/边界：四种允许离线 state、message NULL/空/普通、空 phase/payload、raw payload bytes、runtime ready、邀请码隐藏、单次 FIFO、activation/durable save 回归。
- 故障/恢复：phase 写库、MaintenanceStarted journal、LastError journal、ComposeDown、strict unknown/error、清旗 journal、snapshot storage、四个 Panel 崩溃窗口以及 FIFO 结果模糊；所有不能完成整条恢复证明的场景均保留 recovery required。
- Windows 精确专项与 Web 提交/取消通过；任务专属 `golang:1.25-alpine` 容器中受影响四包通过，最终 `go test ./... -count=1` 通过；宿主 `go vet ./...`、`go build ./...` 通过。首次全量恰逢保留的无关 Control 0.3.3 DLL 与 manifest 先后更新，编译读到旧摘要而失败；当前两者 SHA256 一致后以同一命令重跑通过。
- pre-candidate 阶段没有构建/推送候选或正式镜像；最终 v0.5.0 候选重新跑默认全量和升级后受影响矩阵，并以不可变 proof、自动 tag 和同 digest 正式提升补齐发布证据。本地自动化仍不能单独替代该候选证明。

# SAVE-IMPORT-STRICT-OFFLINE-PROBE-1 候选前门禁记录（2026-08-15，released in v0.5.0）

## 变更清单与受影响链路

- Docker Client 的普通 `ComposePs`、1.5 秒 cache、UI/状态页接口保持不变；`ComposePsStrict` 继续固定执行 `docker compose ps --all --format json`，新增 stdout/stderr truncation、JSON `null` 和未知 Docker state 拒绝。成功且未截断的空 stdout 仍保留 Compose Down 后 0 services 的真实契约。
- 存档导入仅允许无 server 或全部 server=`exited/dead`；`running/Up/restarting/paused/created/removing/unknown/空状态` 全部 fail closed。安全调用点覆盖 pre-ownership、maintenance 初检、pre-ComposeUp、失败 ComposeDown 终态和 owned cleanup。
- 数据库权威 `DataDir` 贯穿 journal/staging/maintenance/cleanup；调用方目录不一致时提交与 cleanup 直接返回 `import_recovery_required`。公开 API、SQLite schema、Compose 模板、runtime manifest、Control/SMAPI 制品、部署格式和前端 bundle 均不变。

## 本任务专项矩阵

| 维度 | 场景 | 结果/后续候选要求 |
| --- | --- | --- |
| 正常路径 | fresh 为 `exited/dead`；项目无 server；Compose Down 成功空 stdout | strict 通过；既有真实 Docker empty-project integration 继续保留 |
| cache/新鲜度 | cache stopped/fresh running；cache running/fresh exited | 前者拒绝且零事务副作用；后者必须读到 fresh exited，不等待 TTL |
| 解析边界 | 坏 JSON、`null`、缺 service/state、未知 state、stdout 截断 | 全部返回 strict error；不得转换成空 services |
| 状态安全 | `running/Up/restarting/paused/created/removing/unknown/空`；多个 server 中混入任一非稳定项 | 全部拒绝；只有全部 `exited/dead` 通过 |
| 权限/所有权 | `game_installed` 但 fresh server 运行；Web commit | journal、runtime asset、bootstrap、staged target 与 ownership 均不变，Web reservation 释放回 available |
| 数据完整性 | 调用方 `DataDir` 与数据库权威目录不同 | 提交/owned cleanup 在变更前 recovery required；maintenance 失败 journal 只写权威目录 |
| 幂等/恢复 | pre-ComposeUp 复验、失败 ComposeDown 复验、owned cancel cleanup | 每个边界重新运行 strict；普通 cache invalidation 不替代证明 |
| 升级后 E2E | 上一正式版升级得到的新 Panel 执行 cache/fresh 冲突和失败清理 | v0.5.0 候选已按差异选择并执行；没有用 fake driver 或连续普通 `ComposePs` 代替 |

## 本地验证与资源清理

- Windows 定向 `go test` 覆盖 Docker Client、save-import 与 Web 提交并通过；任务专属 `golang:1.25-alpine` 容器中，同一专项与 Docker/Junimo/Web 三包全量通过。受控测试使用真实 `docker.Client` 的 cache、command runner、limited buffer 与 JSON parser，只把 docker 可执行文件替换为确定性 fixture，不绕过生产解析层。
- 首次三包全量仅挂 `backend/`，两条既有 Nexus extension ZIP 测试因缺仓库根资产失败；改为完整仓库 `/workspace:ro` 后三包全绿。第一次默认并行 `go test ./... -count=1` 又在 Docker Desktop 资源竞争下触发既有 `TestControlRuntimeContextCancellationDoesNotCleanup` 时序失败；该测试单项通过，`go test -p 1 ./... -count=1` 全绿，随后用户要求的最终默认 `go test ./... -count=1` 也全绿（Stardew 57.881 秒、Web 46.576 秒）。两次环境/时序诊断没有修改产品或降低断言。
- `go vet ./...` 与 `go build ./...` 在同一 Linux 工具链和任务专属 Go module/build cache 中通过。容器均带 `com.openai.codex.owner=SAVE-IMPORT-STRICT-OFFLINE-PROBE-1` 且使用 `--rm`；交付前精确删除两个任务 cache volume 并复核容器、网络、volume 为零。
- 本任务不创建候选、不提交/推送、不打 tag、不提升 `latest`、不发布镜像或 GitHub Release。上述矩阵是下一正式候选的专项输入，不是发布证明。

# v0.4.18 正式候选与发布结果（2026-08-15，released）

## 变更清单与受影响链路

- 目标版本由自动候选从当前正式版 `v0.4.17` 递增补丁为 `0.4.18`；本地 `main` 只纳入三组已完成工作：`SAVE-IMPORT-COMPOSE-EMPTY-SET-1`、`RUNTIME-UPDATE-JUNIMO-MATERIALIZE-1`、`FE-MODAL-VIEWPORT-1` / `FE-CONTROL-COMMAND-PAGINATION-1`。不得手工创建或移动 tag；推送后必须由不可变候选、自动 tag 和 digest 提升链完成发布。
- 存档导入修复把 Compose 命令成功且 stdout 为空解释为 0 个 service，解除 Panel Stop 已完成后首次上传被错误挡在 ownership/journal 之前的 `save_in_progress`；非零退出、坏 JSON、缺字段和未知状态仍 fail closed。
- Junimo 运行栈修复不再把“server image 未变化”等同于“宿主 JunimoServer 一定完整”。Control-only apply 会在事务变更计划和 staging 前校验宿主挂载目录；缺失、损坏或版本不符时仍从已验签目标 server image 提取并事务替换。对历史 `rollback_failed`，回滚会先用事务保存的原 server immutable image ID 补回原版本 JunimoServer，再固定原镜像、启动验收并恢复原 running/stopped 状态；恢复材料、三次尝试上限和公开 repair 确认体不变。
- 前端新增共享 body Portal modal：集中处理 body scroll lock、背景 inert/`aria-hidden`、Esc、焦点圈定/归还和嵌套栈；桌面/移动现有对话框迁移到同一契约。最近控制命令固定每页 3 条并可翻页，玩家活动分页和认证布局边界同步消除窄高/宽矮视口的遮挡。公开后端 DTO、数据库、部署格式、runtime manifest、Control/SMAPI DLL 均不改变。

## 本版专项矩阵

| 维度 | 必测场景 | 正式候选门禁 |
| --- | --- | --- |
| 正常路径 | Stop 后 Compose 为空的首次存档上传；Control-only 且宿主 Junimo 缺失；历史 rollback_failed repair；桌面/移动 modal 与分页 | 公开上传 API 必须返回 `202/jobId/operationId`；apply/repair 均补齐推荐 Junimo 版本；第三次受限 repair 成功；17 项前端状态回归与 production bundle 契约通过 |
| 关键边界 | Compose 非零/坏 JSON/未知状态；Junimo 损坏或错版本；repair 已失败 2 次；宽矮与窄高视口、嵌套 modal、页尾不足 3 条 | 后端继续 fail closed；repair 第 3 次允许执行且成功后归零恢复目录，第 4 次仍禁止；modal 不越出视口、不穿透背景，分页页码夹紧 |
| 权限安全 | 管理员上传、升级、repair；恢复目录/镜像来源；键盘焦点 | API 鉴权和严格 `{"confirm":true}` 不变；只接受事务匹配、SHA-256 通过和 immutable image ID；Esc/Tab 只作用于顶层 modal，关闭后焦点归还 |
| 幂等/恢复 | 重复 strict probe、Control-only apply、旧事务 repair、Panel restart | 0 service 不创建资源；有效 Junimo 不重复提取；repair 仍绑定原 apply ID、尝试计数和恢复材料；中断后由既有 write-ahead 规则收敛 |
| 数据完整性 | 存档 ownership/journal、原 `.env`/Compose/Control、SQLite、非目标游戏数据 | 上传修复不改存档内容；rollback 必须恢复原配置和 Control、从原 image ID 补 Mod；升级 E2E 再验 SQLite、初始化状态、Panel 哨兵及非目标容器/volume |
| 资源清理 | 上传受控失败、Junimo extraction staging/recovery、modal 全局状态、候选 DinD | job 终止后项目容器/网络归零；临时提取目录和成功 recovery 删除，失败材料保留；最后一个 modal 关闭后恢复 body/background；所有测试资源按 owner 精确清理 |
| 升级/回滚 | `v0.4.17 → 0.4.18` healthy 与同引用 unhealthy；升级后的旧 rollback_failed 真实 repair | unhealthy 必须 `failed_rolled_back/health_check_failed` 并恢复 v0.4.17；healthy 升级后用公开 API 构造第 3 次 repair，删除可信 tag 后仍须从原 image ID 补齐 Junimo、保持 steam-auth container 与 stopped 状态 |
| 生产前端 | fresh 候选与 Web 升级后的精确 production chunks | ServerControl/MobileControl/Saves 既有契约继续通过；JobsLogs 必须包含最近控制命令分页、Players 包含玩家活动分页，加载产物必须包含共享 `aria-modal` / initial-focus 契约 |

## 门禁选择、候选失败与修复

- 因后端 `stardew_junimo`、Docker strict probe、前端和发布候选脚本同时变化，自动候选必须执行后端默认全量 test/vet/build、Docker integration、runtime/SMAPI 真实长链、前端全部状态测试/audit/production build、脚本语法与 ShellCheck、fresh install/restart、上一正式版 Web unhealthy 回滚和 healthy 升级。runtime manifest、Control 源/DLL、数据库 migration、部署格式和网站内容未变；远程制品、网站等是否跳过仍只允许 `scripts/run-release-gates.sh` 按 `v0.4.17..candidate SHA` 路径差异决定。
- 发布前专项已通过：Junimo Control-only 缺失目录单测、旧 schema 3 `rollback_failed` repair 单测、相邻回滚恢复矩阵，以及从真实 Docker immutable image ID 提取 Junimo 的 integration；Linux 默认全量 Go test/vet/build、Docker/updater/runtime integration、41,889,142 B 真实 SMAPI 下载、前端 17 项状态回归/audit/production build、兼容矩阵 20 项、部署脚本测试、网站 production build、`bash -n` 与 ShellCheck 0.11.0 均通过。Windows `npm ci` 因既有 `node_modules` 文件锁触发 `EPERM` 后没有删除或重试，改由 Node 22 Linux 容器与任务专属依赖/产物卷完成同一前端门禁。
- 本地完整候选演练以 `0.4.18`、上一正式版 `0.4.17` 构建未发布镜像，最终通过轮固定 build date=`2026-08-15T11:57:09Z`、local image ID=`sha256:f76b22fcec32007c55700807b1aaf9d28bc460668c250664f4bc3ebcda249f80`。同一轮通过 fresh health/version/setup/restart、公开 Web unhealthy `failed_rolled_back/health_check_failed`、healthy 升级、SQLite/初始化/非目标资源/Panel restart、旧 `rollback_failed` 第三次 repair 从原 immutable image ID 补回 Junimo 并保持 steam-auth container/stopped 状态，以及升级后 Stop 空 Compose 存档上传成功与受控失败清理。演练只用于验证当前工作树，metadata 中的 revision 是演练前基线，不可替代最终干净 `main` 的自动候选证明。
- 演练期间先后暴露并修复了候选夹具问题：DinD `apk` TLS 抖动改为同一必需工具集三次有界重试；原受控失败会进入产品真实 20 分钟 readiness，改为缺失且禁止自动创建的 bind source 立即触发 Compose Up 失败；旧事务 E2E 调整到会占用恢复 ownership 的受控失败之前；Compose 的 `$line` 正确转义为 `$$line`；Junimo IPC 夹具改为真实 `/tmp/smapi-input` 且 EOF 后重新打开。每次都保留产品真实超时、API 和终态断言，没有降低门禁。
- 最终提交 `3acf3ff8ceed7c0fb848fbf2e7d2c12ff3194478` 推送后，首个自动候选 `31883713810` 在 selected code gates 失败并于镜像构建前停止：Linux runner 中真实 Docker helper 以 root 创建提取树，Actions 用户在原子 rename 和 TempDir cleanup 均得到 `permission denied`。修复不是跳过 integration，而是让非 Windows helper 在静态校验后把目标树递归归还当前 Panel 进程的纯数值 UID/GID，并在 integration 中增加宿主写入/删除探针。任务专属 DinD 已用 root daemon + UID 1000 Go/Panel 精确复现并通过该 integration（5.42 秒），随后该包默认 test（27.90 秒）、vet/build 通过；失败候选没有构建、推送 candidate、创建 tag 或改动正式仓库。
- 最终修复提交 `56c437004b51763e77d12ffd9b716f39224d7b00` 推送并精确等于 `origin/main` 后，自动候选 `31884242692` 于 `2026-08-15T12:17:52Z..12:26:37Z` 成功（job 8 分 42 秒）。统一门禁执行兼容清单 20 项、部署脚本、默认 Go test/vet/build、Docker/updater integration、Junimo runtime 真实 integration、41,889,142 B SMAPI 真实下载、前端 17 项状态测试/audit/build和网站 build；runtime/公开文档路径变化使对应长门禁被选择，manifest 输入未变化使远程制品验证按脚本自动跳过，没有人工降级。并行 Compatibility `31884242697` 于 `12:17:52Z..12:19:57Z` 成功。

## 不可变候选、升级 E2E 与自动发布

- 候选证明 artifact `release-candidate-0.4.18-56c437004b51`（artifact ID `9246912273`，484 B）固定 version=`0.4.18`、previous=`0.4.17`、commit=`56c437004b51763e77d12ffd9b716f39224d7b00`、build date=`2026-08-15T12:18:12Z`、local image ID=`sha256:5d2d3c7ce75f6a9d72387b69035791df394cb34cc2b08e7670a05c32347aa8c8`、candidate ref=`ghcr.io/anxiyizhi/stardew-server-anxi-panel:candidate-0.4.18-56c437004b51` 和 digest=`sha256:b304e3b9c83620e94e3a16f33f5730991f74e470820a7481e696b54738eb8d74`。同一候选完成 fresh health/version/setup/restart，随后导入隔离 DinD，没有为升级链重建第二份镜像。
- `v0.4.17 → 0.4.18` 真实 Panel Web 链先把同一目标引用指向 unhealthy 派生对象，得到 `failed_rolled_back/health_check_failed` 并恢复旧 Panel、SQLite、初始化状态、Panel 哨兵和非目标游戏容器/volume；再把引用原子恢复到上述精确候选，完成 check/dry-run/管理员确认/apply、预期断线重连、版本/commit、数据保持和 Panel restart。升级后的新 Panel 继续构造 v0.4.17 风格、已失败 2 次的 `rollback_failed`，删除原可信 tag 后第 3 次公开 repair 仍从清单 immutable image ID 补回 Junimo、保持 auth 容器 ID 和 stopped 状态并清除 recovery；随后公开 Stop 产生 0 Compose 容器/空 stdout，存档上传返回 `202/jobId/operationId`，受控 maintenance 失败终止后实例、容器和网络清理正确。
- 自动 Tag workflow `31884612425` 于 `12:26:39Z..12:26:53Z` 成功创建 annotated `v0.4.18`；tag object=`01f2e52a140e404fbfbbeccd1a7c287ce40910bc`，解引用精确等于候选 commit，tagger time=`2026-08-15T12:26:48Z`。没有人工创建、移动或补推 tag。
- 正式提升 workflow `31884620508` 于 `12:26:50Z..12:28:26Z` 成功。它重新验证候选证明、tag/main、digest 与 OCI identity，以 `skopeo --all --preserve-digests` 原样提升 exact，冒烟一个正式 exact 后才提升三仓 `latest` 并创建 Release；全程没有重新 build。GitHub Release `v0.4.18` 于 `2026-08-15T12:28:22Z` 发布，为 latest、非 draft、非 prerelease。

## 发布后独立核验与资源清理

- Docker Hub、阿里云 ACR、GHCR 的 `0.4.18` 与 `latest` 六个公开引用逐一用 Buildx 读取，全部精确等于候选 digest=`sha256:b304e3b9c83620e94e3a16f33f5730991f74e470820a7481e696b54738eb8d74`。从 GHCR 不可变 digest 回拉的镜像 label 为 version=`0.4.18`、revision=`56c437004b51763e77d12ffd9b716f39224d7b00`、created=`2026-08-15T12:18:12Z`。
- 独立正式镜像使用任务专属 data volume 启动；首次和 `docker restart` 后均为 Docker health=`healthy`、`/health.status=ok`、database=`ok`、`/api/version=0.4.18`、完整 commit、build date=`2026-08-15T12:18:12Z`，`/api/setup/status.initialized=false`。容器与 volume 在 owner label 复核后终态均为 0。
- Release 四项资产与 API digest、`v0.4.18` tag 源逐字节一致：`run.sh`=`7263bfa323b2bf4eb94674bde9c77a57a8b86734c606055c9cdef2fc1e130787`、`migrate-fnos.sh`=`90510768d6636917fb7f15937a7dce34c34974dd8c9af5451030560eca57cbfd`、`repair-junimo-0.3.5.sh`=`13a07708d23e02c002c979eef28639bc2fe283a2e5988e228afc0c068f51cd0e`、`repair-junimo-upgrade.sh`=`4f3c666770b6be77ed51895264f47c940b066d61386b66b3653a858e8929b4c2`。下载目录在校验后逐文件清理，正式冒烟容器/卷、候选 DinD/Compose/网络/卷和本地演练资源均无残留；未执行 prune，也未触碰生产数据。

## v0.4.18 官网版本同步（2026-08-15，post-release docs-only）

- 官网首页和 changelog 已切换到 v0.4.18，公开说明只使用上述正式发布证据：停服空 Compose 存档导入、Control-only JunimoServer 物化/旧人工事务恢复、共享模态和最近控制命令分页。该提交只改 Markdown 与长期文档，不改变候选内容、annotated tag、三仓 digest、`latest`、版本接口或 Release 资产，也不得触发候选重建。
- VitePress production build 2.96 秒通过；应用内 Browser 在本地 1440×900 和 390×844 从首页真实点击到 `/changelog.html`，首页 v0.4.18、日志 v0.4.18/v0.4.17/v0.4.16 顺序和三组正文全部命中，root/body 横向溢出、framework overlay、console warn/error 均为 0。
- docs-only 提交 `09601de0d9b9064b88a56d091678194a65c333cd` 于 `2026-08-15T13:00:00Z` 触发的 Pages `31886032569` 成功（build 18 秒、deploy 10 秒），Compatibility `31886032526` 成功（2 分 5 秒，后端、前端 build 与隔离 Docker integration 全通过）；该 SHA 没有候选 workflow。部署后应用内 Browser 在公开 URL 的 1440×900/390×844 再次从首页真实点击到 changelog，版本顺序与 Compose/JunimoServer/确认框/每页 3 条四类正文全部命中，两页 root/body 均无横向溢出、overlay=0、console warn/error=0。该发布只改变官网和证据文档，没有重建镜像、创建或移动 tag。
- 最终证据回填提交 `93f6a8464962f597319e986ae3114bdcf7a64106` 只修改 5 个长期文档，Compatibility `31886298061` 在 1 分 59 秒内完成后端、前端 build 与隔离 Docker integration；因没有 `website/**` 变化而未重复 Pages，也没有候选 workflow。该提交后再次确认 annotated `v0.4.18` 仍解引用到 `56c437004b51763e77d12ffd9b716f39224d7b00`，GitHub latest Release 仍有四项原 digest 资产，三仓 `0.4.18/latest` 六引用仍统一为 `sha256:b304e3b9c83620e94e3a16f33f5730991f74e470820a7481e696b54738eb8d74`。从该不可变 digest 启动的独立容器在重启后再次通过 Docker healthy、health/database、精确 `/api/version`/build date 与 fresh setup，owner 容器和 volume 终态均为 0。

# SAVE-IMPORT-COMPOSE-EMPTY-SET-1 发布专项矩阵（2026-08-15，released in v0.4.18）

- 变更清单：修复 `v0.4.17` 在 Panel Stop 已成功 `docker compose down` 后，`ComposePsStrict` 把退出 0 的空 stdout 当成探针错误，导致上传事务在 ownership/journal 前被 Web fallback 误报为 `save_in_progress`。修复仅将“命令成功 + 空 stdout”解释为 0 services，并新增 Docker integration 及升级后 Web E2E；公开上传 API、前端、Junimo/Control/runtime manifest、数据库和部署格式不变。候选脚本仅为一次性 DinD 补充 `zip` fixture 工具。
- 受影响链路：`POST .../saves/upload-commit-and-start → ImportSaveAndStart pre-ownership gate → ComposePsStrict → saveImportServerStoppedStrict`，以及复用 strict probe 的 maintenance/Phase A/安全 cleanup 停机证明。没有改动 Compose Up/Down、存档内容、token/journal 结构或运行栈镜像。

| 专项 | 本版验证 |
| --- | --- |
| 正常路径 | integration 实际 `ComposeUp` 启动 server，`ComposeDown` 后证明项目容器为 0，再要求 `ComposePsStrict` 成功返回 0 services；升级 E2E 从公开 Panel Stop 得到同一空输出，再要求上传返回 `202/jobId/operationId`。 |
| 关键边界 | 非空 running 正常解析；真实运行中的 server 仍由 driver 返回 `save_in_progress`。坏 JSON、缺 service/state、未知 server 状态和 Docker/Compose 非零退出继续 fail closed。 |
| 权限安全 | 修复不改变管理员鉴权、路径校验、Docker 参数或前端字段；空集合只在 Compose 命令退出 0 时成立，错误输出和非空不可分类结果不降级。 |
| 幂等/恢复 | strict probe 继续绕过 UI cache；重复调用不创建资源。导入 ownership、journal、`MaintenanceStarted`、upstream 和 cleanup fingerprint 门禁不变。 |
| 数据完整性 | 生产取证全程只读，确认故障请求没有创建 import job/journal、没有接管 token；测试不挂载生产数据，也不读写真实存档。 |
| 资源清理 | integration 使用唯一前缀 Compose project，测试后容器和网络归零；升级 E2E 的受控 maintenance 容器立即退出，必须验证 job 失败终止、实例恢复 stopped 且项目容器/网络归零；本地 Go 门禁资源按精确 owner 校验后归零。 |

- 本地门禁：Linux 定向 `go test ./internal/docker ./internal/games/stardew_junimo -count=1`，串行全量 `go test -p 1 ./... -count=1`、`go vet ./...`、`go build -o /tmp/anxi-panel ./cmd/panel`，以及宿主 Docker 全套 `go test -tags=integration ./internal/docker -count=1 -v` 全部通过；新增真实空集合 E2E 与既有 runtime/updater integration 同轮通过，测试后对应容器和网络归零。`release-candidate.sh` 与升级 E2E 的 `bash -n`、ShellCheck 在只读仓库挂载的 Alpine 任务容器中通过，容器归零。
- 候选结果：产品后端、Junimo materialization、候选脚本、前端与公开文档路径变化，统一代码门禁、Docker/updater integration、Junimo runtime/SMAPI 真实长链、网站 build、候选 fresh/restart、`v0.4.17` Web unhealthy 回滚和 healthy 升级均被自动选择；manifest 输入未变化的远程制品验证按脚本跳过。升级后的 Panel 真实执行 Stop 空集合上传专项并清理受控失败，最终身份、run、digest 与资源证据见本文件顶部。

# v0.4.17 官网版本同步（2026-08-15，post-release docs-only）

- 官网首页与 changelog 同步到已发布的 v0.4.17，公开范围为 steam-auth `/health` 服务验收、安装完成后首次上传状态机和“社区中心收集包”文案修正；内容直接来自正式 Release 与本文件的不可变候选/升级/回滚证据。
- 本次只改官网 Markdown 和长期文档，不改变 v0.4.17 候选、annotated tag、三仓 digest=`sha256:44c328cdf198ec888f3ec54bbe836ce114f5ac27c4ca5fb9cc63747a44083673`、`latest`、版本接口、GitHub Release 正文或四项资产，也不触发候选重建。
- VitePress production build 5.90 秒通过；应用内 Browser 在本地及线上 1440×900/390×844 从首页真实点击到 `/changelog.html`，版本顺序、三项正文、零横向溢出、零 overlay 和零 console error/warn 均通过。发布提交 `94db6f6066120cba903204e6fe1e47d40e06cc95` 的 Pages `31871879333`（build 22 秒、deploy 10 秒）与 Compatibility `31871879299`（1 分 43 秒）成功；只触发这两个预期 workflow，没有候选重建。
- 发布后再次只读核对：Docker Hub、阿里云 ACR、GHCR 的 `0.4.17/latest` 六引用仍统一为 `sha256:44c328cdf198ec888f3ec54bbe836ce114f5ac27c4ca5fb9cc63747a44083673`；annotated `v0.4.17` 仍解引用到 `d63c93ffe7d65f8cdfcf2bedb9b336a6839be73f`。GitHub latest Release 仍为正式 `v0.4.17`，正文命中三项变更且四项资产保持；本次纯文档提交不改变已在正式发布后冒烟确认的 `/api/version=0.4.17` 证据。

# v0.4.15 / v0.4.16 发布说明补录（2026-08-14，post-release docs-only）

- 正式镜像、tag、digest 与 Release 资产均不变。本次只补齐此前遗漏的公开说明：官网首页由 v0.4.14 切换到 v0.4.16，changelog 同时补入 v0.4.15 与 v0.4.16；GitHub 两版 Release 正文同步为同一用户可读变更范围，并继续保留精确验证身份和 compare 链接。
- v0.4.15 公开范围为 Nexus 0.1.3 持久幂等、存档导入自动解绑、无存档实例首次上传和 nanoid 3.3.18；v0.4.16 公开范围为历史 runtime failure 安全收敛、桌面/移动隐藏但兼容 `FarmhouseStack`、桌面游戏日回档悬停详情。
- 这是发布后纯文档修正，不改变候选内容、运行契约或测试输入，不触发候选重建、不移动既有 tag，也不重新提升 digest。VitePress build 5.63 秒通过；应用内 Browser 在 1440×900 与 390×844 验证首页→更新日志真实导航、两版正文、零横向溢出、零 overlay 和零 console error/warn。GitHub 两版 Release 正文已更新且非 draft/prerelease、发布时间未变、每版四项资产名称/大小/digest 未变。
- 发布提交 `2df79f939a014e12ddf8d952e019fd1320691f22` 经 Pages `31802129359`（build 20 秒、deploy 38 秒）和 Compatibility `31802129284`（1 分 44 秒）成功。线上首页与 `/changelog.html` 在桌面/手机均命中 v0.4.16、v0.4.15 及对应正文，零溢出/overlay/console error/warn；本次没有候选、tag、registry、latest、版本接口或镜像 digest 变化。

# 候选制品一次构建与正式 digest 提升流程（2026-08-14）

- 正式发布拆成三个自动衔接的 workflow：影响镜像或正式部署资产的提交推送到 `main` 后，`.github/workflows/release-candidate.yml` 自动读取最新 GitHub Release 作为升级源，并在最高稳定 tag 上递增 patch 版本，冻结 version/full commit/build date、运行代码回归并调用 `scripts/release-candidate.sh`；Windows Docker Desktop 可用 `pwsh -NoLogo -NoProfile -File scripts/release-candidate.ps1 -Version <x.y.z> -PreviousVersion <x.y.z>` 复现同一镜像链。只有候选全部通过后才推送 GHCR `candidate-<version>-<sha12>` 并上传 `candidate.json` 证明。手动 dispatch 仅用于明确 major/minor 版本或受控重试。
- `scripts/run-release-gates.sh --version <x.y.z> --base-ref v<previous>` 始终运行兼容清单、脚本、后端 test/vet/build、updater/Docker integration、前端全部状态回归与 production build；只有 Junimo/runtime manifest 或实现变化才运行真实 SMAPI 下载、远端制品与运行栈长 integration，只有官网/公开文档变化才构建 VitePress。
- 候选镜像只构建一次。自动候选链先做 fresh health/version/setup/restart，再把完全相同的本地镜像导入任务专属 DinD；DinD 使用受控 TLS `api.github.com` Release endpoint 与受控 TLS `ghcr.io` registry，从上一正式版真实调用 setup cookie、update check、dry-run、管理员 apply、断线重连和终态查询。相同目标引用先发布 unhealthy 镜像验证 `failed_rolled_back/health_check_failed`，再切回健康候选验证成功升级、SQLite integrity、初始化状态、Panel 数据、非目标游戏容器/volume 和 Panel 重启。
- annotated `v*` tag 只能指向候选证明的完整 commit，且该 commit 必须仍精确等于 `origin/main`。候选成功后，`.github/workflows/release-after-candidate.yml` 读取已完成 run 的唯一证明；若 main 已前进则只标记 superseded，若仍相等则由 GitHub Actions bot 创建 annotated tag，并显式 dispatch `.github/workflows/release.yml`。正式 workflow 下载同 SHA/版本的成功候选 artifact，核对 GHCR digest 与 OCI version/full revision/build date 后，以 `skopeo --preserve-digests` 把同一对象提升为 Docker Hub、ACR、GHCR 的精确版本；一个正式仓库 health/version 冒烟成功后才更新三仓 `latest` 并创建 GitHub Release。发布 workflow 不再编译或重建镜像。
- 默认升级矩阵只有“上一正式版 → 候选”。数据库迁移、部署格式、运行栈、长期数据结构或跨版本兼容实现变化时，才根据迁移代码增加“受影响的最老支持版本 → 候选”；对应真实新功能验收仍需在升级后的 Panel 上执行。三仓六引用 digest 相同时不再启动三份重复镜像，只核对六引用并冒烟一个精确正式引用。
- 自动顺序：维护者只需把已经完成的产品变更和本版矩阵提交到同步的 `main`；产品路径过滤命中后自动候选、自动验证 main 未被取代、自动 tag、自动 digest 提升和 Release。文档、发布工作流或测试脚本自身的单独提交不自动发布镜像。任何候选证明、run ID、tag、digest、OCI 或 main 身份不符都会 fail closed；需要 major/minor 版本时才手动 dispatch 并填写版本覆盖值。
- 本流程实现阶段用 `-AllowDirty` 做过一次不发布的合成 `0.4.16` 本地演练，仅验证脚本本身，不构成正式候选证明：候选 image ID=`sha256:86057f4d36c5866d6365e463abc9a9f4a0bf1262afac957b1ae9d2562fc1fec9`，从正式 `0.4.15` 通过真实 Web 完成 check/dry-run/apply；同引用 unhealthy 先收敛为 `failed_rolled_back/health_check_failed`，切回健康候选后升级成功，并通过 SQLite integrity、初始化/Panel 哨兵、非目标 game container/volume 与 Panel restart。最终通过轮用时 403.5 秒，未 push candidate/正式 registry、未创建 tag/Release；任务 DinD、Compose、容器、网络、卷和临时目录归零。正式版本仍必须在干净且同步的 `main` 上运行 workflow 取得 artifact。

# v0.4.16 正式候选与发布结果（2026-08-14，released）

## 变更清单与受影响链路

- 目标补丁版本由自动候选流程从最新正式版 `v0.4.15` 递增为 `0.4.16`。本版只包含三项已完成修复：`REQUIRED-RUNTIME-STALE-STATUS-1`、`FE-CABIN-FARMHOUSESTACK-HIDE-1`、`FE-SAVE-GAMEDAY-HOVER-DETAILS-1`。
- 后端在普通 runtime apply 成功、required 状态读取和 Panel 启动协调入口收敛同一当前 Panel/stack pair 的历史 `failed`：只有 apply=`succeeded` 且实时 managed stack=`up_to_date` 才改写为 `succeeded` 并清空旧错误。`manual_action`、当前真实失败、dry-run/apply 诊断文件和公开 HTTP DTO/phase 均不改变。
- 桌面与移动小屋策略选择器只隐藏 `FarmhouseStack`，继续保留已有受控值及后端 `CabinStack|FarmhouseStack|None` 三值契约；不会在页面加载或未保存时改写旧配置。桌面“游戏日回档”与“其他备份”统一使用同一整行悬停详情函数，未改备份 API、排序、按钮状态、CSS 或移动端常驻详情。
- 本版没有数据库 migration、部署/Compose 格式、runtime manifest、Control/SMAPI DLL、存档/Mod/备份结构或公开 API schema 变化。required 状态只允许匹配候选自身 Panel/stack pair 后收敛，不解释或迁移更老 Panel 身份，因此升级矩阵保持“当前上一正式版 `v0.4.15` → 候选”，不增加更老版本链。

## 本版专项矩阵

| 维度 | 必测场景 | 正式候选门禁 |
| --- | --- | --- |
| 正常路径 | 同候选 Panel/stack 的旧 required failure + 成功 runtime apply + 实时 up-to-date | Go 定向/全包回归必须返回并持久化 `succeeded`，旧 error/errorCode 清空；普通 apply 完成后立即收敛 |
| 关键边界 | apply 非成功、实时栈非 up-to-date、Panel/stack pair 不匹配 | 必须继续保留 `failed`；不得因单一版本字符串或历史文件存在而误清理 |
| 权限与兼容 | `manual_action`、已有 `FarmhouseStack`、管理员 API 门禁 | `manual_action` 不自动清理；前端隐藏旧选项但保持受控值，GET/PUT 三值契约和管理员权限不变 |
| 幂等与恢复 | 状态读取、Panel 重启和启动协调重复执行 | 已成功状态保持成功，不重复 apply；写入失败只记录 warning/返回错误，不删除 dry-run/apply 诊断 |
| 前端生产产物 | fresh 候选与 Web 升级后的生产 bundle | 桌面/移动 chunk 都必须包含 `FarmhouseStack` hidden 兼容 option；Saves chunk 必须包含游戏日回档类型、农民、日期和地图详情 |
| 数据完整性 | healthy 升级、unhealthy 目标自动回滚、Panel 重启 | SQLite integrity、初始化状态、Panel 哨兵、非目标 game container/volume 均保持；unhealthy 必须为 `failed_rolled_back/health_check_failed` |
| 资源清理 | 所有 fresh/DinD/Compose/registry/network/volume/bind | 只清理本轮 owner 资源，终态计数为 0，不执行 prune，不接触生产数据或凭据 |

## 选择的自动门禁与发布前记录

- `scripts/run-release-gates.sh` 已纳入 `test:cabin-strategy-options` 和 `test:save-backup-details`，Compatibility workflow 同步执行；responsive 契约锁定两项测试仍属于正式门禁。
- `scripts/release-candidate.sh` 的 fresh candidate smoke 新增生产 chunk 验收；`scripts/tests/test_release_candidate_upgrade.sh` 在 `v0.4.15` 真实 Web healthy apply 成功后再次下载升级 Panel 的精确 chunk 并执行同一产品契约。两条链均检查隐藏兼容 option 与回档悬停详情，不用源码或开发服务器代替生产 bundle。
- 由于 `backend/internal/games/stardew_junimo/**` 变化，自动路径选择必须执行后端 test/vet/build、SMAPI 真实下载和 runtime auth Docker integration；前端执行全部状态脚本、production audit/build。网站、Control 源/DLL、runtime manifest、部署脚本功能未变化，是否跳过由 `run-release-gates.sh` 基于 `v0.4.15..candidate SHA` 自动判定，不能人工缩减。
- 本地发布前预检已通过：17 项前端状态脚本、production audit=0、TypeScript/Vite build；三个变更 Shell 的 `bash -n`/ShellCheck 0.11.0；四份 release workflow YAML 解析；Linux 三项 required-runtime 定向测试；Linux `go test -p 1 ./... -count=1`、`go vet ./...`、`go build ./...`。桌面 QA 已确认两端隐藏选项、回档详情和 console/横向溢出。首次冷 Go cache 曾被 `proxy.golang.org unexpected EOF` 截断；改用同一 owner cache 有界预热后恢复。默认多包并行在本机 Docker Desktop 曾让既有 fake dry-run 超过 5 秒预算；单项与串行全包均通过，资源归零，正式候选仍必须执行仓库默认并行命令，不能以本地串行替代。上述预检不构成正式候选证明；只有同步干净 `main` 推送后 `Validate release candidate` 的不可变 artifact、unhealthy 回滚、healthy Web 升级和后续 digest 提升全部成功，才允许自动 tag/正式发布。本节将在发布后以 workflow ID、唯一 digest、实际矩阵、耗时、故障和资源清理结果回填。
- 首次自动候选 run `31798450997` 在精确提交 `e719f4b31fdbc4911a22c59cee326fbfd9747a95` 上停止于 fresh production bundle 断言；并行 Compatibility matrix run `31798451024` 成功。镜像 build、版本注入与 fresh health 已完成，但 `SavesPage-*.js` 的无边界提取同时匹配 `SavesPage` 和 `MobileSavesPage`，被安全判为非唯一 chunk；没有产生候选 artifact、tag、正式镜像或 Release。修复把 fresh 与升级后断言统一收紧为路径边界匹配并剥离前导 `/`，必须重新通过脚本语法/ShellCheck、合成双 chunk 回归、真实 production bundle 断言和新的完整自动候选，不能重用本次失败身份。

## 正式候选、Tag 与提升结果

- 产品修复提交为 `e719f4b31fdbc4911a22c59cee326fbfd9747a95`，候选 bundle 断言修复为 `b51b792433dbffcb3ee3581b0cad91ac96e5ba92`。随后用户账号提交仅修改 README 的 `5fa04d137bf760d2124b75cc5e3e8e2b44ff4c7c`，旧候选 run `31799008175` 在镜像构建前因 `origin/main` 已前进而 fail closed，未生成 artifact/tag；最终候选按最新 main 重建，没有把当时工作树中另一组未提交前端修改带入版本。
- 最终 `Validate release candidate` run `31799350642` 从 2026-08-14T12:12:33Z 到 12:20:04Z 成功（约 7 分 31 秒）。自动矩阵执行兼容契约、部署脚本、默认并行后端 test/vet/build、Junimo/SMAPI 真实网络与 runtime integration（真实 SMAPI 下载 41,889,142 B）、前端 17 项状态测试/audit/production build；因 README 变化额外构建网站。runtime manifest 未变，因此远端制品核验按脚本规则跳过。fresh health/version/setup/restart、生产 bundle 专项、`v0.4.15` Web check/dry-run/apply、同引用 unhealthy `failed_rolled_back/health_check_failed`、健康升级、SQLite/初始化/Panel 哨兵/非目标游戏容器与 volume、升级后 bundle、Panel restart 全部通过。
- 候选证明 artifact=`release-candidate-0.4.16-5fa04d137bf7`，version=`0.4.16`、previous=`0.4.15`、commit=`5fa04d137bf760d2124b75cc5e3e8e2b44ff4c7c`、build date=`2026-08-14T12:12:51Z`、image ID=`sha256:45e74b320bdd0e3304e6d582621d967e02a68873adaaf0349f5e46d4d036fd15`、唯一 digest=`sha256:5f07910869d6d895e40ecb3954f5905d0cb6abf830e7cf57062bbcf97ca37e0f`。Compatibility matrix `31798967632` 在相同产品/脚本提交上成功。
- `Tag validated release candidate` run `31799876171` 成功；annotated `v0.4.16` tag 对象=`a832e0d0be53e0a107cac34e2ccc260ce612d426`，剥离后精确指向 `5fa04d137bf760d2124b75cc5e3e8e2b44ff4c7c`。`Promote validated panel candidate` run `31799891830` 从 12:20:24Z 到 12:21:56Z 成功（约 1 分 32 秒），未重新 build，只提升上述候选 digest。

## 发布后独立核验与清理

- Docker Hub、阿里云 ACR、GHCR 的 `0.4.16` 与 `latest` 六个远端 manifest 全部为 `sha256:5f07910869d6d895e40ecb3954f5905d0cb6abf830e7cf57062bbcf97ca37e0f`；OCI label 为 version=`0.4.16`、完整 revision=`5fa04d137bf760d2124b75cc5e3e8e2b44ff4c7c`、created=`2026-08-14T12:12:51Z`。
- 从 GHCR 精确 digest 启动独立正式镜像，首次与重启后均为 Docker health=`healthy`、`/health.status=ok`、database=`ok`、`/api/version=0.4.16@5fa04d137bf760d2124b75cc5e3e8e2b44ff4c7c`、build date=`2026-08-14T12:12:51Z`、setup initialized=false。首次本地脚本只等待 HTTP 即读取 Docker health，命中正常 `starting` 窗口后已改为联合有界等待并完整重跑通过；没有降低镜像门禁。
- GitHub Release `Stardew Server Anxi Panel 0.4.16` 为非 draft、非 prerelease。下载资产与 tag 源字节一致：`run.sh` 33,793 B / `7263bfa323b2bf4eb94674bde9c77a57a8b86734c606055c9cdef2fc1e130787`，`migrate-fnos.sh` 34,269 B / `90510768d6636917fb7f15937a7dce34c34974dd8c9af5451030560eca57cbfd`，`repair-junimo-0.3.5.sh` 14,585 B / `13a07708d23e02c002c979eef28639bc2fe283a2e5988e228afc0c068f51cd0e`，`repair-junimo-upgrade.sh` 8,521 B / `4f3c666770b6be77ed51895264f47c940b066d61386b66b3653a858e8929b4c2`。
- 候选/正式 workflow 的 DinD、Compose、受控 registry、容器、网络、volume 与 bind 均由成功门禁清理；本地正式镜像冒烟终态容器/网络/volume 为 0，候选证明与 Release 资产临时目录也已逐文件核对后清零。没有 prune、没有生产数据或长期凭据参与测试。

# v0.4.15 正式发布结果（2026-08-14，released）

- annotated tag `v0.4.15` 固定指向 `d84157dc8a3abc83d13d29c276d6ed332e901ce7`，tag 对象为 `39038d64a7068a9233ef74757b011453dc86fbe5`；创建前本地 `main` 工作树干净且与 `origin/main` 精确同步。Compatibility matrix `31725203858` 成功，Release workflow `31725256195` 从 2026-08-13T17:21:02Z 运行到 17:28:48Z，7 分 43 秒内完成全部 release gates、三仓 push 和 GitHub Release 创建。
- 最终本地候选 image ID=`sha256:e47f0a4e6ba973e4256b6941e7eb4cfa8f7d6d6e37c3eb3c1fd948b13042da7d`，受控 registry digest=`sha256:ec550a39ce4b6353b4d0380a26708e17797739deabd9608f12b00f32107e4efa`，version=`0.4.15`、完整 revision=`d84157dc8a3abc83d13d29c276d6ed332e901ce7`、created=`2026-08-13T17:01:15Z`。fresh/restart 以及 v0.4.14、v0.3.2 两条真实 Web 一键升级均通过；升级后 Nexus 同 key 复用、错误 runtime 409 后同 token 恢复、无存档首次上传、Control 自动解绑 total/customized/bound=`2/1/0`、bootstrap 清理和 Panel 重启耐久全部成立。由同一候选制作的 unhealthy 目标以 `health_check_failed` 自动恢复 v0.4.14，长期数据与非目标 Docker 资源保持。
- Docker Hub、阿里云 ACR、GHCR 的 `0.4.15` 与 `latest` 六个实际回拉引用统一为 digest/image ID=`sha256:b91e3cfd8175305723e0b97feb7c4c202179f2e229aff4f6145fe60b354a5c33`，OCI version=`0.4.15`、revision=`d84157dc8a3a`、created=`2026-08-13T17:26:43Z`。正式 workflow 按既有契约使用 tag commit 的 12 位前缀；三个精确镜像分别用独立 container/network/volume 完成 fresh + restart，均返回 Docker health healthy、`/health.status=ok`、database=ok、`/api/version=0.4.15@d84157dc8a3a`、fresh setup 未初始化，测试运行资源终态为 0。
- GitHub Release `Stardew Server Anxi Panel 0.4.15` 为非 draft、非 prerelease。四项资产均与 tag 源字节一致：`run.sh` 33,793 B / `7263bfa323b2bf4eb94674bde9c77a57a8b86734c606055c9cdef2fc1e130787`，`migrate-fnos.sh` 34,269 B / `90510768d6636917fb7f15937a7dce34c34974dd8c9af5451030560eca57cbfd`，`repair-junimo-upgrade.sh` 8,521 B / `4f3c666770b6be77ed51895264f47c940b066d61386b66b3653a858e8929b4c2`，`repair-junimo-0.3.5.sh` 14,585 B / `13a07708d23e02c002c979eef28639bc2fe283a2e5988e228afc0c068f51cd0e`。
- tag 前新增的 nanoid high advisory 以最小 3.3.17 → 3.3.18 lockfile 修复收口，洁净 production audit=0、15 项状态测试和 production build 通过。最终候选首次 runtime layer 构建遇到一次 DinD VFS/APK 写 `/usr/bin/docker` I/O error；目标 tag 为 0、磁盘/inode 充足且独立可写探针通过后，唯一有界重试成功，没有关闭 TLS/APK 签名或降低门禁。全部 `.agents/v0415-*`、277 MB SDK tar、隔离 DinD、registry、任务容器/网络及 15 个任务卷已按 owner 精确清理；三个只读历史夹具源卷保持未改，没有执行 prune。

# 部署入口恢复国内 HTTP 加速卡片（2026-08-13，已完成，未发布）

- 所有活动 `run.sh` 部署命令入口现统一为“官方 GitHub Release 安装（推荐）”在上、“国内加速脚本（HTTP）”卡片在下；本文件的一键启动说明已同步恢复该卡片，完整地址只在实际命令区出现一次。
- 本次只修改文档展示，不改变 `deploy/run.sh`、镜像候选、Compose、Panel API 或 updater；没有构建/推送镜像、创建 tag、更新 `latest` 或创建 GitHub Release。
- README、新手指南、官网三页与本文共六处已通过地址数量/顺序检查；网站 production build 通过，桌面和 390×844 Browser 验证卡片渲染、页面跳转、无横向溢出及零 console warn/error。

# v0.4.15 正式候选范围与门禁状态（2026-08-13，pre-release）

## 2026-08-14 tag 前新增依赖安全门禁

- 最终前端洁净 production audit 新命中 `GHSA-2v37-7h3g-55p8`：Vite 8.0.16 经 PostCSS 8.5.25 锁定 nanoid 3.3.17，而 advisory 修复边界为 3.3.18。门禁按发布阻断处理，没有忽略 high，也没有直接运行无边界 `npm audit fix`。
- `frontend/package-lock.json` 只更新传递节点 nanoid 3.3.17 → 3.3.18、官方 npm tarball URL 与 integrity；PostCSS 原 `^3.3.16` 契约允许该补丁，`package.json` 与其它依赖节点不变。Node 24 重新洁净安装后 production audit=0，15 项前端状态测试与 production build 全部通过。
- 该 lockfile 位于镜像 build context，因此此前最终身份 `df90240` 的 fresh、v0.4.14/v0.3.2 升级后功能与 unhealthy 回滚只保留为发现修复前的代码证据，不能用于 tag。提交依赖修复及本节后，必须重建精确 version/revision/date 候选并重复三条关键门禁；正式结果随 tag 后三仓证据一起回填。

- 目标版本为 `0.4.15`，合并三组已完成且共用同一存档导入发布门禁的修复：`NEXUS-EXT-IDEMPOTENCY-1`、`SAVE-IMPORT-AUTO-UNCLAIM-1` 与 `SAVE-IMPORT-FIRST-UPLOAD-1`。Panel 新增 migration 013，浏览器扩展升为 0.1.3，内嵌 Control 升为 0.3.2；Junimo/SMAPI/game/auth 版本不变。
- 正式候选必须从本地 `main` 的最终提交构建，build args 固定 `VERSION=0.4.15`、完整 commit SHA、UTC build date；本节后三张功能矩阵全部完成前禁止 push `main`、创建 `v0.4.15`、更新 registry/latest 或创建 Release。
- 自动解绑必须在唯一隔离的真实旧导入夹具上走完整后台上传事务，而不是只调用 Control 命令；正常链、零 farmhand/在线玩家/错 Control/结果断流、Panel 或 Control 中断恢复、稳定 XML、角色/小屋/Mod/备份保留和资源归零均需有候选证据。
- `b15fa42` 候选的真实首次上传已证明 runtime-prepare 失败可重试、bootstrap maintenance、swap finalizer、Control 自动解绑与 GameLoop.Saved 均能运行，但同时捕获发布阻断竞争：command history 在 durable gate 读取前归档并删除结果，数据库公开白名单又不保留私有聚合证明，任务安全进入 recovery。生产修复改为由未完成 import journal 保护精确 command result；针对性 Go 测试已通过，旧候选证据作废，必须从包含该修复的最终 SHA 重建并重跑整链。
- `eaae88f` 的独立首次上传夹具把宿主数据目录以同一路径额外挂进 Panel，因而没有覆盖标准 Compose 的 namespace 边界。v0.4.14 Web 一键升级到该候选已通过 check/dry-run/apply、真实断线重连、health/version/commit、数据库/用户/Mod/备份/空 saves/非目标游戏容器与 volume 保留及重启；升级后 Nexus 同 key 复用也通过。随后空存档上传在错误镜像注入后正确 409/transaction=0，但恢复 `.125` 的同 token 重试仍 409：Panel 把容器内 `/data/.../junimo-mod-sync` 直接交给宿主 daemon，二级容器实际看不到目标目录。`1961b40` 候选加入 helper 路径映射后正确镜像重试已进入 maintenance，但实例 Compose 的 `./.local-container/...` 又由 Panel 内 Compose CLI 解析成 `/data/...`，server/auth 因宿主 daemon 看不到该路径而启动失败。当前修复同时把受管 Compose bind 迁移到 `${INSTANCE_HOST_DATA_DIR}`；两个候选的升级后新功能证据均作废，必须从新 SHA 重建并重跑。
- Nexus 幂等已在 `eaae88f` 候选覆盖真实 Chrome 0.1.3、Panel bridge、20 路同 capture、浏览器重启持久化，以及服务端首次响应被调用方丢弃、Panel 活动 runner 重启、20 路终态重放和不同 key；权威 SQLite/任务日志断言仍以 owner/job、distinct key 和 runner 启动次数为准，没有只看扩展 singleflight。
- 升级矩阵至少包含上一正式版 `v0.4.14` 与本次 migration/运行栈支持边界 `v0.3.2`，两者都必须使用 Panel Web 一键更新完整链；目标 unhealthy/版本错误必须恢复原版。升级后新 Panel 再执行三项新功能，其中首次上传必须从空 saves/无 gameloader pointer 开始；同时验证 SQLite、用户、实例、存档、Mod、备份、审计与非目标 Docker 资源保留。
- tag 前还须执行 Release workflow 的精确全量命令：兼容清单与远程制品、脚本功能/语法/ShellCheck、Linux Go test/vet/build 与 integration、SMAPI 真实下载、前端全量测试/audit/build、网站 audit/build、候选 fresh/restart 和镜像内 helper/扩展包检查。
- `.dockerignore` 现在明确排除任务级 `.agents` 目录；本地/CI build context 不再发送错题本、发布夹具、临时 ZIP 或 E2E 脚本。正式镜像仍只由 Dockerfile 的 frontend/browser-extension/backend/deploy 精确 COPY 组成，排除项不改变运行产物。
- tag 后等待 Release workflow 成功，再从 Docker Hub、ACR、GHCR 回拉 `0.4.15`，核对三仓 digest、OCI version/revision、latest 与 GitHub Release/四项资产，并逐仓执行隔离 health/version/restart；最终 workflow ID、digest、耗时、故障和清理结果必须回写本文件及接手/路线文档。

## 2026-08-14 代码等价候选门禁证据与最终候选边界

- 代码等价候选 `5fc7e4cf7f01f92e7a25c39c90a036cc86d3e122` 已在本机 Docker Desktop Linux containers 构建为 `0.4.15`：image ID=`sha256:b405f14accc8cbcba1b11af27a4e854341c902a8be4d99e6b622c77087462ba8`，OCI revision 精确匹配，受控本地 registry digest=`sha256:cbab69051512bd91e0f53a0e46807d947239e41d67220c7316be56d205383e49`。该镜像只用于本地门禁，没有推送互联网 registry、更新 `latest`、创建 tag 或 Release。
- `v0.4.14 → 5fc7e4c` 与 `v0.3.2 → 5fc7e4c` 都从真实旧 Panel Web 完成 update check、dry-run、管理员确认、apply、预期断线重连与终态恢复。两条链均精确核对 `/health`、`/api/version`、migration 013、数据库、初始化、管理员、实例、空 saves、Mod、备份、审计、非目标游戏容器/volume、升级状态与 Panel 重启；升级得到的新 Panel 再执行 Nexus 重放、错 Junimo runtime 409 后同 token 正确 runtime 重试、空 saves 首次上传以及自动解绑，最终 Control/Junimo/磁盘为 total=2、customized=1、bound=0，bootstrap 清理且 journal 在 Panel 重启后仍为 completed。
- 由同一候选制作的 unhealthy 目标从 `v0.4.14` Web 一键更新后进入 `failed_rolled_back`，错误为 `health_check_failed`；旧 `0.4.14`、数据库、用户、Mod、备份、非目标游戏容器/volume、升级备份与重启状态均恢复并保持可读。受控 registry 随后恢复健康候选，测试只清理本轮拥有的 Compose project、容器、网络、bind 与 volume，没有执行 prune。
- 全量门禁已通过：Linux `go test ./... -count=1`、`go vet ./...`、三个 Go 命令构建、41,889,142 B 真实 SMAPI 下载、runtime/updater/Docker integration（含 real candidate upgrade/reconcile）、前端 15 项状态测试与 production audit/build、网站 production audit/build、兼容矩阵 19 项、四个部署脚本功能测试、八个 Release 脚本 `bash -n` 与 ShellCheck 0.11.0。Control 0.3.2 使用官方签名的 ModBuildConfig 4.3.0 与真实只读 game-data 编译 0 errors；内嵌 DLL 150,528 B、SHA-256=`a62e525d07279c4c8e8ca13d94e4914cfee2eae79ae60077332c5f2d8b897b2d`，source/embedded/runtime manifest 均为 0.3.2，行为由上述真实升级后上传再次验证。
- 官网 docs-only 提交 `13f6af396a904474c53a4c0c4ed5132436ec6159` 发布后，本地两个 Docker namespace 修复已安全 rebase 为 `967647d` 与 `fd04ff0`；产品代码与 `5fc7e4c` 等价，但 commit 身份已经变化。因此 `5fc7e4c` 不能作为 tag 镜像。正式 tag 前必须先提交本节证据，从最终干净 `main` 重建精确 revision 候选，并至少重跑 fresh/restart、两条 Web 升级后的三项新功能以及 unhealthy 自动回滚；这些最终身份结果和 tag 后三仓证据将在发布收口提交回填。

# SAVE-IMPORT-FIRST-UPLOAD-1 候选门禁（2026-08-13，代码完成、未发布）

## 变更清单与受影响链路

- 新安装但从未启动的实例可能尚无宿主侧 `JunimoServer` Mod。提交导入前现在先从 `.env` 指定的精确 `.125` server image 只读提取、原子替换并重新静态校验；只有真实 image/tag 不兼容才返回 `junimo_import_unsupported`，同步/校验故障使用新的 `save_import_runtime_prepare_failed`。该步在 journal 和上传所有权转移前完成。
- 标准 Compose 下任何交给 Docker daemon 的 bind source 都不能使用 Panel 内 `/data` 路径。driver 由 `cmd/panel` 接收 container/host 数据根，只把容器数据根内路径按相对路径映射到 `PANEL_HOST_DATA_DIR`；越界、非绝对或不完整映射拒绝执行。相同规则覆盖 Junimo/runtime update 提取、SMAPI bundled staging 与 SMAPI 安装包挂载；实例 Compose 的受管 `.local-container` bind 改用 `${INSTANCE_HOST_DATA_DIR}`，Prepare/runtime recovery 自动补写宿主实例路径并原子迁移旧相对路径，避免 helper 修好后 server/auth Compose 仍复用错误 namespace。
- journal 创建时若没有活动存档，staging 在 preimport 已耐久后从未修改的上传目标创建事务专属 `AnxiImportBootstrap_<operationId>` 副本，仅把副本主文件重命名为 bootstrap 名，并把 gameloader 指向它。这为 Junimo 提供一个非目标的维护世界，防止零存档启动自动新建其它世界，也避免上游“不能导入当前活动存档”门禁。
- bootstrap 名称、全树指纹、no-replace 发布所有权和清理状态全部写入 operation journal。提交前取消只在 ownership 已耐久且 pointer 仍指向该 bootstrap 时恢复为“无 pointer”并删除事务自有副本；发布成功但 ownership 尚未落盘的崩溃窗口、指针冲突和同名碰撞均保留现场进入 recovery。导入完成则只在目标 pointer、finalizer/durable save/磁盘门禁全部通过后删除 bootstrap；清理失败不宣布 completed。
- 受影响文件为 `save_import_bootstrap.go`、`save_import_transaction.go`、`save_import_durable.go`、Web/前端错误码映射及对应测试。upload-preview/commit JSON、hostHandling、管理员权限和已有存档导入时序不变。

## 本功能故障矩阵

| 场景 | 预期措施 | 当前证据 / 正式发布前状态 |
| --- | --- | --- |
| 首次安装、无宿主 JunimoServer Mod | 从精确 image 同步一次并复核，不误报升级 | `eaae88f` 同路径夹具通过；标准 Compose 升级后稳定复现 host bind namespace 409 并完成代码修复，最终 SHA 候选待重跑 |
| 明确非 `.125` image | 仍返回 `junimo_import_unsupported`，不进入 journal/不接管 token | 旧版本专项通过；候选 API 待执行 |
| image 提取、网络、原子替换或校验失败 | 结构化 runtime-prepare 错误；上传 token 仍可重试，无导入 journal | 同路径候选恢复 `.125` 后同 token 成功；标准 Compose 错镜像分支同样精确 409/transaction=0，但恢复分支暴露错误 bind，映射修复后的最终候选待重跑 |
| 空 saves/无 gameloader 的正常首次上传 | preimport 后创建非目标 bootstrap，运行维护链，目标 durable 后删除 | `eaae88f` Web 实链成功：bootstrap created/cleaned，最终 saves 目录仅 1 个目标，preimport 可读，Panel 重启后 journal completed、server 保持 running |
| 已有活动存档 | 沿用原 pointer，不创建 bootstrap | 旧链与新断言通过；候选需与首次链 A/B 复验 |
| bootstrap 同名碰撞/指纹变化 | 不覆盖，不改现有字节，进入 recovery | 碰撞零修改回归通过；真实故障注入待候选 |
| 复制/发布中断 | 隐藏 staging + 指纹 + no-replace，重试只继续原 operation | 单元幂等边界通过；Panel kill 窗口待候选 |
| 提交前取消 | 删除 bootstrap pointer/副本、本 operation target/source，保留 preimport | 回归通过；真实 API cancel 和资源归零待候选 |
| Junimo/Control/API/FIFO 延迟、断流或容器退出 | 仍使用既有 maintenance fail-closed，不重发 import，保留 journal/preimport | 既有失败矩阵单测通过；首次链真实故障待候选 |
| 目标已耐久但 bootstrap 清理失败 | 不写 completed，目标保持活动，人工/恢复只清事务自有副本 | target-pointer 清理门禁回归通过；权限故障待候选 |
| 数据完整性 | 上传目标与 preimport 在 bootstrap 构建前后指纹一致；最终仅目标留在 saves | 候选真实存档完成 durable save；目标主文件、preimport、JunimoServer DLL/manifest 均可读，bootstrap=0、目标目录=1，Panel 重启保持 |
| 权限与敏感信息 | 仍限管理员；bootstrap 只含 operationId，无 platformId/姓名/凭据新增日志 | 合成管理员同进程 session；journal 扫描确认完整测试 platformId 不存在，公开输出仅聚合计数；support bundle 扫描待执行 |
| 升级/回滚 | v0.4.14/v0.3.2 升级后从空 saves 首传；unhealthy 候选不留 bootstrap/journal | v0.4.14→`eaae88f` 的 updater、保留、重启和升级后 Nexus 已通过；升级后首传定位并修复 host bind，最终候选全链、v0.3.2 与 unhealthy 回滚待重跑 |

# NEXUS-EXT-IDEMPOTENCY-1 发布门禁补充（2026-08-13，代码完成、未发布）

- compatibility 与 release workflow 的前端门禁新增 `npm run test:nexus-extension-idempotency`；PR path 同时覆盖 `browser-extensions/**`，扩展代码变化不会再绕过 compatibility。
- 自动专项覆盖扩展 20 路并发、panel bridge、失败重试、service worker 重启、不同 fileId，以及后端 SQLite 12 路原子 owner、终态复用和 HTTP 契约。它们只证明代码级幂等，不替代 Docker/真实浏览器验收。
- `eaae88f` 候选使用任务专属 Chromium profile 加载真实 unpacked 0.1.3：Panel 同源 bridge 注册后，20 路同 capture 消息得到 20 个成功响应、19 个 shared 结果、1 个 jobId；Panel 仅持久化 1 个 `mod_remote_install` job。弹窗标题/正文/queued 状态可见，Panel 与 popup console error/warn 为 0；关闭并重开 Chromium 后 storage 中原 requestId/job 仍精确保持。测试只使用合成账号和受控失败的 `*.nexus-cdn.com/*.zip` URL，没有使用生产 Nexus 登录或真实用户数据。
- 独立候选服务端 E2E 让标准 HTTP 客户端发出请求但不观察响应；看到一个活动 owner 后重启 Panel。恢复为 failed 后 20 路同 key 全部 202/deduped 并返回原 job，终态再重放仍复用；不同 key 创建且只创建第二个受控失败任务，129 字节 key 返回 400。SQLite 为 total=2/distinct_keys=2，首任务 runner 启动日志精确 1 次，失败前后 Mod manifest 数不变，随机 key 不在 audit metadata 或 Panel logs。
- 本轮仍未执行 v0.4.14/v0.3.2 Web 一键升级、创建 tag、推送 registry 或更新 latest；升级后复验和正式 release 收口完成前继续保持 pre-release。

## 本功能正式候选故障矩阵

| 场景 | 预期措施 | 当前证据 / 正式发布前状态 |
| --- | --- | --- |
| 正常单次安装 | 一个 requestId、一个 HTTP POST、一个 job，导入链和公开请求体保持兼容 | `eaae88f` 真实 Chrome → Panel bridge → 候选 API 通过；受控 URL 安全失败，未使用生产 Nexus 下载 |
| 快速重复点击、自动/手动/下载事件竞争 | 进行中 Promise 先登记后执行，followers 共享结果；SQLite 只允许一个 owner/runner | 真实 Chrome 20 路为 1 owner/19 shared/1 job；独立候选 20 路 HTTP 重放仍为原 job，runnerStarts=1 |
| 同 Mod 的不同文件、未知 fileId、新安装动作 | 已知 fileId 不同必须换 key；未知文件身份不合并；不同批量项各有独立 key | Node identity 回归通过；真实 Nexus 页面切换待执行 |
| 非法、缺失或碰撞 key | 非法 key 返回 400；缺失保持 0.1.2 兼容；同 key 在同实例只返回原 job | 129 字节 key 真实返回 400；同 key 终态复用；SQLite 2 个不同动作仅 2 个 distinct key；随机 key 不在日志/审计 |
| POST 前网络失败 | 不创建 job；rejected singleflight 立即清理，capture 保持 active 并沿用 key 重试 | 同 worker 立即重试回归通过；可控代理断网待候选 |
| job 已创建但 HTTP 响应断流/超时 | 重试同 key，返回原 jobId 与 deduped=true，不启动第二个 runner | 候选调用方丢弃首次响应后，20 路重放与终态重放均返回原 job，runnerStarts=1 |
| 扩展 worker 或 Panel 进程中断 | requestId 已进 chrome.storage；Panel 重启后由 SQLite 原 job 恢复响应 | 真实 Chromium 关闭/重开后 requestId/job 保持；活动 runner 时重启 Panel，原 job 安全恢复为 failed 并可复用 |
| 首次任务已部分成功或已经失败/终态 | 相同 key 仍绑定原任务，不自动重下或重导；用户明确新动作才换 key | Panel 中断后的 failed 终态仍绑定原 job；不同动作 key 只创建第二个 job；真实解压中断仍由既有远程安装矩阵覆盖 |
| 失败回滚和数据完整性 | 本功能不改变 Junimo 下载、校验、原子导入/回滚逻辑；重复请求不能额外写第二份 Mod | 两个受控失败任务前后 Mod manifest 数相同、临时归档由 defer 清理；坏包/升级后复验仍走既有矩阵 |
| 权限与敏感信息 | 仍要求管理员和目标实例权限；CDN query 不进入 key、公开 Job、审计 metadata 或普通日志 | 合成管理员真机通过；key 不在 audit metadata/Panel logs；浏览器/服务端输出只保留聚合计数 |
| 资源清理 | singleflight 成功/失败均立即删除；测试只清理任务自有浏览器配置、容器、网络、volume 和 bind | Chromium profile/cache、候选 Panel、精确 bind 均按 owner 清理；未 prune，外层 DinD 留给后续升级门禁 |
| 旧版升级与回滚 | migration 013 原子应用；旧扩展无 key 继续工作；目标候选失败时整版 Panel 按现有 updater 回滚 | 旧 schema → 当前 migration 全量测试通过；上一正式版/最老受影响版 Web 一键升级与 unhealthy 回滚待候选 |

# SAVE-IMPORT-AUTO-UNCLAIM-1 候选门禁（2026-08-13，代码完成、未发布）

## 变更清单与受影响链路

- Control 从 0.3.1 升为 0.3.2；source/embedded manifest 和 DLL 与 runtime_stack_manifest.json 一致，当前 DLL SHA-256 为 a62e525d07279c4c8e8ca13d94e4914cfee2eae79ae60077332c5f2d8b897b2d，运行栈后缀为 control-0.3.2。
- swap_to_player 导入链改为：Junimo swap-host-to/finalizer → Panel 持久化同一 save-now commandId → Control 保存前 unbind-all-farmhands → GameLoop.Saved → Control total/customized/bound 结果 → Junimo diagnostics 二次零绑定复核 → dayTransition/稳定 XML/磁盘变化 → completed。virtual_host_takeover/as-is、上传 DTO、前端页面和 SteamID 输入不变。
- Control 预保存动作只清空 Game1.netWorldState.farmhandData 中非空 userID，不改 Server 主机、角色数量、isCustomized、小屋或其它角色内容。pending journal 保存完整动作身份；恢复重复执行同一动作和同一 commandId，不生成第二次保存。
- Panel maintenance runtime 在进入导入前验证当前内嵌 Control DLL 与 options.json 版本；结果/journal 只含角色计数，原始 platformId/userID 继续不进入普通日志、HTTP、审计或事务 journal。

## 本功能故障矩阵

| 场景 | 预期措施 | 当前证据 / 正式发布前状态 |
| --- | --- | --- |
| 正常 swap_to_player | finalizer 后同一次 save-now 自动清空全部绑定，双证据和磁盘门禁后 completed | `eaae88f` 候选完整 Web 首次上传成功；journal 为 total=2/customized=1、unbind verified，durable save 与 completed 通过，Panel 重启后 server 仍运行 |
| virtual_host_takeover/as-is | 不提交解绑动作、不额外保存 | Go as-is 回归保持原分支；正式候选继续覆盖 |
| 空/不可读 farmhandData、错目标存档、非服务器 | Control 在保存前失败，不改盘、不发布 completed | C# precondition/结果测试；真实故障注入待候选 |
| 真人 farmhand 在线 | 不踢人，动作失败并保持维护态 | C# 在线门禁与 Go maintenance 测试；真实客户端在线分支待候选 |
| 旧 Control、坏 DLL/hash、options 缺失或错版本 | pending 只等待启动；明确 mismatch/invalid fail closed并停止导入推进 | maintenance/control gate 单测通过；代表老版本升级必须验证自动同步到 0.3.2 |
| diagnostics 网络超时、断流、failedFields 或结果缺计数 | 不把 GameLoop.Saved 单点当成功，返回 unconfirmed/recovery 并保留事务材料 | Go 失败矩阵通过；可控代理/真实断流待候选 |
| command history 与 durable gate 并发读取终态结果 | 未完成 journal 的精确 commandId 结果保留原文件；不可读 fail closed；completed 后才归档删除 | `b15fa42` 稳定复现 recovery；修复专项通过；`eaae88f` 同链 job succeeded，重启后原文件已归档且数据库精确为 succeeded/ok |
| 保存已成功但后续验证暂时失败 | 不重发 import/save-now；journal 保留同 commandId，恢复只继续观察 | Go durable 恢复和 C# pending journal 契约通过；Panel/Control 中断窗口真机待候选 |
| Control 在清空后、Saved 前退出 | 同一 pending journal 恢复，重复清空幂等，只启动原 commandId 的保存 | C# recovery 契约通过；真实 kill 窗口待候选 |
| 重复提交或旧结果 | 动作/目标 saveId/commandId 任一不一致均拒绝，禁止把旧 succeeded 复用到新事务 | C#/Go identity tests 通过 |
| 部分成功后的回滚 | 不自动猜测重绑 userID；维护态与 preimport 保留，只有权威继续验收或显式整档恢复可处理 | 设计 fail closed；正式候选须验证恢复操作不重复解绑、不丢 preimport |
| 权限与敏感信息 | Web 仍使用既有管理员/实例权限；结果只记录计数，不记录姓名或 ID | API 未改；单测与差异审查通过，候选支持包/日志脱敏扫描待执行 |
| 数据完整性 | 仅 userID 变空，角色数、customized、主机、小屋及存档 XML 必须保留 | 隔离真机角色数 2、customized 1 保持，主文件 hash 改变且 XML/重启均为 bound=0；更丰富家庭/Mod 存档待候选 |
| 资源清理 | 仅删除本任务精确 project/container/network/volume/bind；源夹具只读不变 | anxi-unbind-e2e-20260813-r1 容器/网络/两卷归零，克隆目录移入回收站；源 project/volume 未启动或修改 |

## 已完成验证与下一正式版本阻断项

- Control 纯契约测试通过；Docker .NET 6 使用真实 Stardew game-data 和标准 GamePath/EnableModDeploy=false 编译成功，0 errors、1 个既有 analyzer/compiler warning。两份 manifest 版本均为 0.3.2，嵌入 DLL 实际摘要与运行栈清单一致。
- 后端 Linux Go 1.25 容器执行 go test ./... -count=1、go vet ./...、panel/panel-updater/smapi-candidate build 全部通过。专项覆盖动作 payload、结果缺失/错档/仍绑定、Junimo live disagreement、maintenance Control mismatch 和 diagnostics 字段失败。
- Docker Desktop 隔离实测使用只读 save-import-e2e-release3 派生独立 bind 和 game-data/steam-session 卷，不复用生产。Control 0.3.2 实际加载；初始 diagnostics 为 2/1/1、零在线玩家。单一 commandId 返回 succeeded/ok、动作和 saveId 精确匹配、2/1/0；实时 diagnostics 与主 XML 均为零绑定，SHA-256 从 33961e281f503277b9c7feaecaa11a072eb2145b37917a4375d7d1a58dae52e5 变为 2e42f5574d8bc74543ac157bcfee81ee29b38415d01f5f27ca19f8f197153dcd；server 重启后仍为 2/1/0且结果保留。
- 本轮不是正式发布：没有构建/推送 Panel 候选镜像，没有 tag、latest、registry 或 GitHub Release 变更。下一正式版本在 tag 前仍必须用最终 commit 构建精确候选，补完整 UI→上传→job 事务、上述真实故障注入、v0.4.14 和最低支持 v0.3.2 Web 一键升级/失败回滚、升级后的再次自动解绑、前端/脚本/兼容矩阵/Docker integration/网站全量门禁；任一缺失不得发布。

# STARTUP-NEWGAME-DURABILITY-1 正式发布门禁（2026-08-13，v0.4.14 已发布，生产 SSH 待授权）

## 本版候选范围与已进入 main 的前置修复

- `v0.4.14` 已从不可移动 tag `a70efc98feecd6b2db803435b59b0f31d1439cf3` 正式发布；Release workflow、三仓精确版/latest、Release 资产和逐仓首次启动/重启均已通过。生产目标 `114.55.142.107:22` 的 `cz` 与 `root` password authentication 均被服务端拒绝，未建立 session、未执行生产升级；等待用户提供正确 SSH 用户名，不降低为猜账号或绕过认证。
- 启动链修复 Panel false negative：缺少本次 Control 状态是 pending/starting，使用完整 20 分钟预算；只有合法 `options.json` 明确报告了错误版本才是 mismatch，坏 manifest/DLL/options 使用 invalid，pending 超时安全停服。
- 安装/前端链新增 `installationDiagnostic`，把必需文件、Compose、镜像、server 容器与 Control static/runtime 分开；`state=error` 不再一律打开首次安装/重装，Docker 不可读或证据冲突 fail closed 到 diagnostics。
- 新建档链强制 `Idempotency-Key`（缺失返回 428、零写入），增加实例级持久 owner/token、exclusive job、startup/http 固定单写入者和不可逆进展记录。owner 使用 staging+fsync+no-replace rename 原子抢占；loader、Control 或目录任一前进后禁止第二次 `/newgame`。unknown/ambiguous/中断保留现场，用户手动启动恢复同一事务。
- 角色定制成功门禁升级为四段：transaction-bound `save-loaded` → 内存完整身份/外观/颜色与唯一 host 正确 → 同一持久 `save-now commandId` 通过 pending journal 恢复并收到精确 `GameLoop.Saved` → 主 XML/SaveGameInfo 两轮 hash 稳定、完整角色/农场字段和 Control/loader/目录身份一致。Control 升为 `0.3.1`，source/embedded manifest、DLL 与运行栈清单必须同源。
- 新建档失败回滚新增 `rolling_back` 步骤 journal，中断后手动 Start 只停服并继续原回滚。unfinished owner 阻断所有存档/安装/更新/玩家/重启计划变更；Panel bootstrap 的 Runtime/SMAPI 恢复仅收敛/回滚，游戏保持关闭。
- `92f3be6bb2731358420ba315ac18029c2506d81f` 已在 `origin/main` 修复 `deploy/run.sh`：复用已启用 swap 或新建 `/swapfile` 都立即并持久化 `vm.swappiness=60`，带专项测试和 Release workflow 门禁。下一 Release 必须上传此后的新版 `run.sh`，不能复用 v0.4.11 的附件。
- `621c5645e0048da7c4793035615438ed78fc7002` 已在 `origin/main` 实现图形化 Compose 自动标准化。其本地专项与隔离成功链见下一节；本次正式版本仍必须在最终候选及生产真机同步验收，不能因提交已 push 就视为已发布。
- 明确不实现宿主机重启自动恢复 running 游戏实例。宿主重启后游戏保持关闭，用户手动开启；不得添加 Docker restart policy、Panel 启动自动 ComposeUp 或“恢复原 running 状态”的发布断言。仅 unfinished new-game owner 在用户显式启动后恢复原事务。

## 受影响链路

`Panel 启动/重启 → 清理旧 Control snapshot → ComposeUp → Control pending/ready/mismatch/invalid → instance state/Reconcile → /state installationDiagnostic → 桌面/移动操作映射`

`新建档 UI → 强制 Idempotency-Key → exclusive lifecycle job → atomic persistent owner/token + transaction → startup/http writer → Control/loader/目录进展 → targetSaveId → Control 0.3.1 完整内存复核 → 同 commandId pending journal/save-now → GameLoop.Saved → 双 XML 稳定/身份收敛 → Mod profile → success/owner release`

`正式 Panel 更新 → 621c564 图形化 Compose capability/conversion → v0.4.11/v0.3.2 升级与回滚 → 新 Panel 运行栈 Control 0.3.1 同步 → 升级后功能复验 → 92f3be6 run.sh Release 资产`

本版不新增数据库 migration；new-game owner/transaction 使用实例私有 JSON 文件并保持旧 transaction 兼容读取。部署识别与 Release 附件发生变化，且 Control/长期实例状态与建档恢复路径改变，因此必须执行代表老版本升级和数据完整性矩阵。

## 正式发布前故障矩阵

| 场景 | 预期措施与硬门槛 | 最终候选必须提供的证据 |
| --- | --- | --- |
| 正常冷启动 | 旧 snapshot 清理后 ComposeUp；pending 保持 starting，Control 0.3.1 ready 后才 running | 生命周期单测、真实 `.125` 冷启动日志、`/state` 与实际 `options.json` 版本一致 |
| Control 启动较慢/状态尚未出现 | 使用完整 20 分钟预算，持续报告 elapsed/remaining；不得提前 mismatch 或重装 | 可控延迟在旧 1 分钟之后再写 options，最终成功且无第二次 ComposeUp |
| Control 始终未观察到 | 到期安全 ComposeDown，终态 stopped/`control_runtime_start_timeout`，文件与存档保留 | 缩短测试 timeout 的单元/真实 fault injection，容器归零、game-data/save hash 不变 |
| 明确错误版本 | 只有合法 options 报非期望版本才 `control_runtime_version_mismatch` 并停服 | wrong-version Control fixture；UI 显示诊断而非未安装 |
| manifest/DLL/options 缺失、hash 错、坏 JSON/空版本 | 分别 invalid code，fail closed 停服；不得伪装 mismatch/timeout | 表驱动 gate + 候选镜像嵌入 hash + 实例文件故障注入 |
| 旧 snapshot 清理失败 | 启动前 `control_runtime_snapshot_cleanup_failed`，不得让旧文件通过本次验收 | 权限/文件类型注入，ComposeUp 调用数为 0 |
| gate 失败后的 ComposeDown 超时/断流 | `control_runtime_cleanup_failed`，不能宣称 server 已停；保留人工诊断 | fake/真实 Docker 可控失败，状态与容器事实一致，重试不删除 volume |
| Reconcile/Panel 轮询竞态 | lifecycle/new-game owner 活动时不提升 running；外部容器缺 ready snapshot 也不提升 | Reconcile 并发回归、Panel restart/state refresh |
| 明确未安装 | scaffold/初始化状态且无安装文件证据才显示 install | `/state` fixture、桌面/移动按钮与权限回归 |
| 文件/Compose/镜像明确不完整 | 显示 repair，保留存档/实例配置；不把运行错误当首次安装 | required-files/Compose/image 单项故障 fixture、修复入口 |
| Docker/镜像/卷暂时不可读或证据冲突 | `unknown/diagnose`，不提供重装写操作 | daemon 断流、running+incomplete 矛盾 fixture、前端分类纯测试 |
| Control `not_observed` | 仅等待/重试启动，不报版本错误 | starting/stopped state + missing options 浏览器 fixture |
| 正常 startup writer 新建档 | 固定 writer=startup，Panel `/newgame` POST=0；唯一目录/loader/Control 收敛 | 真实空 saves `.125` E2E、请求计数 0、四段耐久证据 |
| 正常 HTTP writer 新建档 | 旧完整存档达到 transaction-bound save-loaded 基线后 POST 恰好 1 次 | 真实活动旧存档 E2E、POST 计数 1、旧档 hash 保持 |
| 同 key 同配置重复/网络响应丢失 | 返回/恢复原 job；不创建第二 owner/job/transaction，不重复 POST | 12 路并发、HTTP 202 相同 jobId、断流重试和持久目录基数 |
| 缺失/空 Idempotency-Key | HTTP 428 `idempotency_key_required`；零 job、owner、transaction、Compose 和存档变更 | Web 回归、Driver 调用数和私有目录基数 |
| 同 key 不同配置/另一个 active owner | 409 conflict/in-progress，不取消旧 job，不生成新事务 | Web/driver 回归与原 owner token/transaction 内容不变 |
| owner 首次 claim 在 mkdir/写文件/发布窗口中断 | 不暴露半 owner；no-replace 保证单 winner；仅零证据历史空目录可安全隔离 | Windows/Linux 原子 rename、12 路 contender、staging/legacy empty/未知文件故障注入 |
| loader 先前进、目录/XML 延后 | 一见 loader delta 即持久 progress，禁止 POST；等目录同 ID 后才能 bind target | 可控时序 fixture，POST=0、最终唯一目录 |
| Control save-creating/新目录先出现 | 同样抑制 POST，候选只在证据唯一时绑定 | 各信号独立时序测试与 ambiguous 对照 |
| 多目录或信号指向不同 save | `new_game_ambiguous`，保留现场/owner，禁止猜目标、回滚删除或重提 | 故障注入、目录/hash/loader/support bundle 边界检查 |
| `/newgame` timeout/断流/非零但可能已执行 | intent 已持久化；只观察，最终 unknown 也不重提 | POST 服务端接受后断流 fixture，调用计数 1，恢复后仍 1 |
| Panel/runner/container 在 intent、progress、target bind 各窗口中断 | 用户手动启动后接管同 request/transaction/token generation；只继续缺失阶段 | 每个 write-ahead 窗口 restart fixture、job/owner fencing、无重复 mutation |
| owner/token 丢失、损坏或轮换中断 | recovery_required/fail closed；rotation 可恢复，未知 owner 不删除 | token mismatch、owner.json 坏文件、rotation dir 中断测试 |
| Control save-loaded 属于旧/其它 save/tx | 等待精确旧基线仅适用于 HTTP writer；其它 fresh mismatch 立即失败 | durability identity matrix，禁止 save-now 发布 |
| 内存任一身份/外观/颜色/`isCustomized` 或唯一 host 不匹配 | 终止当前成功链，保留现场；不能依据 XML 已出现提前通过 | status customization 逐字段与 players.json 独立故障矩阵 |
| save-now running 结果后或 Saved/终态结果前 Control/容器中断 | 持久 journal 恢复同一 commandId；不得发布第二 command | C# recovery contract + 真实 Control 故障注入，命令/journal/结果基数为 1 |
| save-now unknown/expired/failed/旧结果/错 tx-save | 不通过、不换 ID 自动重提；给稳定 recovery/诊断错误 | C# DeferredCommandOutcomes 契约 + Go 结果矩阵 |
| 磁盘 XML 尚未稳定/临时坏 XML | 有界等待；必须连续两轮相同双 hash 才通过 | 分段写入/暂时 malformed fixture |
| XML 人物字段、SaveGameInfo、whichFarm 错误 | 精确 mismatch，禁止 profile/success | 官方与显式模组农场 fixture，字段逐项故障注入 |
| profile commit 失败 | 耐久存档保留为 profile pending；恢复只重试 profile，不建新档或 save-now | profile failure/restart、save hash 和 command count 不变 |
| 用户在 active owner 时 Stop/Restart/Restore | 拒绝覆盖事务；原 job/owner/容器证据不被取消 | driver/Web 冲突测试与 UI 行为 |
| 回滚计划持久失败，或 quarantine/文件/Mod restore 后中断 | plan 失败零破坏性操作；已开始时按原 `rolling_back` 步骤幂等重放，必须先确认停服 | 每个 write-ahead/checkpoint kill fixture，ComposeDown 失败时零 restore/owner 保留 |
| unfinished owner 下其它 mutation | 存档选择/删除/上传/导入、安装、Control/SMAPI/Runtime 更新、玩家与重启计划均 409/fail closed | state=error + server running/stopped 组合，所有 API/driver 零 Compose/文件变更 |
| 宿主机/Panel 重启 | 普通游戏保持关闭；Runtime/SMAPI 中断恢复仅回滚/收敛，即使 `ServerWasRunning=true` 也不 ComposeUp | 检查 Compose restart policy，bootstrap/recovery 每个 mutation checkpoint 零 ComposeUp；用户手动 Start 后才开服 |
| 621c564 标准 Compose | 普通 Web check/dry-run/apply，不走 conversion，声明结构不被改写 | v0.4.11→候选隔离 Web 更新 |
| 621c564 图形化 Compose 无 env/写死 image | 唯一可信身份后 `conversionRequired=true`，Web helper 标准化并保留外部 volume | tag 前最终候选隔离 E2E + tag 后生产 NAS 真机同步验收 |
| 621c564 目标 unhealthy/版本错/转换窗口中断 | 恢复旧数据库、Compose/env、容器名称/状态和版本；非目标游戏资源不变 | 每个 mutation 窗口 fault injection、`failed_rolled_back` 或明确 `rollback_failed` |
| 92f3be6 已启用 swap/new swap/无 sysctl.d/冲突值 | 运行值和持久值均为 60；幂等，失败不得误报成功 | `test_run_sh_swap.sh`、bash/ShellCheck、候选 Release asset 字节核对 |
| 权限与敏感信息 | install/new-game/update 仍需登录及管理员权限；key/owner 不含凭据，日志/support bundle 不含 env/session/save/transaction 正文 | Web 401/403、脱敏扫描、support bundle 白名单 |
| 数据完整性 | v0.4.11/v0.3.2 升级、失败回滚、Panel restart 后 SQLite、初始化、用户、实例、存档、Mod、备份、审计和 new-game 证据按范围保留 | live/backup SQLite integrity、文件 SHA、transaction/owner 终态、非目标容器/volume ID |
| 资源清理 | 只删除本测试唯一 owner/project/container/network/port/bind/volume；禁止 prune | 每轮前后精确 ownership 查询和零残留报告 |

## 2026-08-13 代码候选与真实升级验证证据

- Control `0.3.1` 纯契约测试通过；使用只读真实 `stardew_game-data` 与标准 `/p:GamePath=/game /p:EnableModDeploy=false` 编译成功，0 errors、1 个既有 analyzer/compiler warning。最终编译产物、嵌入 DLL 与 `runtime_stack_manifest.json` 的 SHA-256 三方一致：`3833769287e794d392296c52df760f8451b24a177243a0926d6f0ca9fd81b3ce`。
- 后端最终源码全量 `go test ./... -count=1` 用时 73.4 秒并通过，`go vet ./...`、`go build ./...` 通过；另修正“安全回滚状态已落盘但原 job 尚差 SQLite 终态写入”的完成竞态，只对 status 中同一个 job 有界等待，其它活动任务仍立即拒绝，专项连续 5 次通过。
- Docker Desktop 隔离真实建档 E2E 用时 143.59 秒并通过：空 saves 的 startup writer 创建 `ReleaseGate_*` 且 Panel POST `/newgame` 为 0；随后以该存档作为旧 active save，HTTP writer 只 POST 1 次并创建 `HTTPReleaseGate_*`，旧主文件/SaveGameInfo SHA-256 保持。两条链都通过完整 Control 内存定制、唯一 host、同 ID `GameLoop.Saved`、主 XML/SaveGameInfo 稳定校验、owner/marker 清理；隔离 project/container/network/两卷终态均为 0。
- 真实 E2E 同时锁定 Stardew 1.6 磁盘契约：性别以 `Gender/gender` 文本为权威，旧 `isMale` 可为 `xsi:nil`；有效衣服 ID 从 `shirtItem/pantsItem.itemId` 读取，旧 `shirt/pants=-1` 只作兼容占位。旧格式仍兼容，错误 item ID 专项必须返回 `new_game_disk_character_mismatch`。
- 前端 14 项 `test:*`、production audit（0 vulnerabilities）和 production build 通过；Bash 四项功能测试、全部脚本 `bash -n`、ShellCheck 通过；compatibility validate、目标 Panel `0.4.12`、19 项 Python 测试、真实 remote artifacts（59.1 秒）通过；runtime Docker integration 12.043 秒、updater 成功/失败回滚 integration 34.801 秒、VitePress production build 通过。
- 代码修复提交为 `3cdf43c5a2b3055add7ed5a6720d97e24794073c`，已推送并与 `origin/main` 同步。本机精确代码候选 `ghcr.io/anxiyizhi/stardew-server-anxi-panel:0.4.12` 的 image ID=`sha256:85be8243c8a61f4d0be2f0d91e22f4e90d5edfb17b07d75fbdec411fbb97cd3c`，内嵌 version=`0.4.12`、完整 revision=`3cdf43c5a2b3055add7ed5a6720d97e24794073c`、build date=`2026-08-13T05:33:59Z`；fresh health/version/restart 通过。
- 唯一任务 DinD 使用受控 TLS `api.github.com` 与受控 TLS GHCR/ACR registry，完整走 setup/login cookie、更新检查、capability、dry-run、管理员确认、apply、预期断线重连与持久终态。正式 `v0.4.11` 先实际运行 unhealthy `0.4.12`，收敛到 `failed_rolled_back / health_check_failed` 并恢复 v0.4.11，再切换相同引用到精确健康候选，事务 `342a58d4135f` 成功；最低支持 `v0.3.2` 以真实零长度历史 apply body 升级，事务 `b3acbb48b87f` 成功。两条链均验证 live/backup SQLite integrity、管理员/实例、save/Mod/backup 哨兵、非目标 game container ID/volume 内容及重启后终态；受控 Release 命中 13 次、registry `/v2/` 命中 226 次。
- `621c564` 图形化链完成两层验收：官方 v0.4.11 hard-coded/no-env 容器先通过新版 Release `migrate-fnos.sh` 一次性标准化并保留旧容器、bind、named/anonymous volume 和 SQLite；随后用包含 `621c564`、对外报告 0.4.11 的来源镜像走真实 Web conversion，先观察 unhealthy 目标并完整回滚，再以健康候选重试，事务 `9afdaa53abb0` 成功。转换后 capability=`supported` 且 `conversionRequired=false`，Panel 重启正常，Release 命中 5 次、registry 命中 178 次。
- 在由 v0.4.11 Web 升级得到的候选 Panel（事务 `16e630f9e439`）上重新构造 `state=error + requiredFiles=ok + compose=ready + image=available` 的隔离证据；普通用户 `/state` 精确返回 `installationDiagnostic.status=installed`、`recommendedAction=retry_start`。应用内 Browser 验证桌面端显示“错误 + 查看诊断”且没有“未安装/重装”弹窗，点击进入 `/instances/stardew/diagnostics`；390×844 移动端显示“切换电脑端查看诊断”，root/body `scrollWidth==clientWidth==390`，两视口 console error/warn 为 0。浏览器转发与嵌套 Panel/container/network/volume 最终精确查询均为 0。
- 上述真实升级仍基于代码提交候选；本次发布证据文档和错题本收口会产生仅文档差异的最终 tag 候选 commit。必须以最终 commit 重建带新 revision 的镜像并重跑身份、fresh smoke、关键 Web 升级/回滚与 Release asset 门禁后才能 tag；当前仍未推送 tag、正式镜像或 `latest`，生产同步也尚未开始。
- 首次文档收口 SHA `4ed7c5ad120ed36caf4613a30037f727a66f75b6` 的远端 Compatibility matrix `31676618033` 在 Linux 12 路 `TestNewGameOwnerAtomicClaimAllowsExactlyOneWinner` 暴露真实并发竞态，因此明确阻止 tag。根因是同一进程 loser 在 winner rename 后、目录同步完成前可能把 owner 目录瞬时不可读误判为 recovery_required。修复增加进程内 claim publication mutex，跨进程仍使用 no-replace rename；Windows/Linux 该专项各连续 100 次通过，Windows全量 74.0 秒、Linux全量 152.2 秒及两平台 vet/build通过。必须等包含此修复的新 SHA 远端 Compatibility 成功后才可继续 tag。
- 最终 `21fa312656f18a8bfdef7db62e224f91b3830deb` 候选镜像 version=`0.4.12`、image ID=`sha256:65af4e0c7dcba236e9e47230c0037b81667499cd8ee19cdabf29300ffd9fab1f`，fresh health/version/restart 通过；同一精确候选在受控 TLS DinD 中先被 unhealthy healthcheck 驱动回滚到官方 v0.4.11，再以健康镜像成功升级，最终 `phase=succeeded`、SQLite integrity=`ok`、save/Mod/backup 三类哨兵保留、Panel restart 后终态持久，任务资源归零。该 SHA 的 Compatibility workflow `31678353960` 成功。
- annotated `v0.4.12` 推送后，Release workflow `31679615132` 在 `Run release gates` 的 ShellCheck 阶段报告 `deploy/repair-junimo-0.3.5.sh` ERR trap handler 内命令 SC2317 并退出 1；metadata、registry login、build/push 和 GitHub Release 均未执行。脚本原有 SC2329 注释只抑制“函数未调用”，未抑制 trap 间接调用函数体的 SC2317；修复把局部解释性指令限定为 `SC2317,SC2329`，并用 ShellCheck 0.10.0/0.11.0 对 workflow 精确输入复验。由于 tag 不可移动，下一候选使用 `v0.4.13`，所有最终身份和关键 Web 门禁必须按新 SHA/版本重跑。
- `v0.4.13` 的最终候选 `26e7a1e4f5949349a316484bf173c0653e7b6ac3`（image ID=`sha256:5efcb8cd73038b2823ae6cefe16f8ea37685d05ea4a6c1af356c1668a8f630d7`）通过 fresh/restart 和官方 v0.4.11 的 unhealthy 回滚+健康升级，远端 Compatibility `31680079482` 成功。Release workflow `31681173485` 的全部 release gates 也通过，但多仓 Buildx push 因 ACR 拒绝 `application/vnd.oci.empty.v1+json` attestation 失败；未创建 GitHub Release。远端审计确认 Docker Hub `0.4.13/latest` 已成为 digest `sha256:6393f178f308aaa20b6f1141aa5ca98f9c4bc518faf055db21ef6eeb5c4d8f9b`，GHCR/ACR 仍为 v0.4.11，属于明确的部分发布。下一成功版本必须以 `v0.4.14` 覆盖三仓 `latest` 并统一 digest；workflow 使用 `provenance:false`、`sbom:false`，避免向 ACR 推送不支持的空 attestation manifest。任务专属 DinD/registry 已用 Buildx 0.35.0 对同一参数做 push 探针，远端 tag 直接解析为单一 `application/vnd.oci.image.manifest.v1+json`，没有 index/empty attestation；容器和卷终态为 0。
- 最终 `v0.4.14` 候选 `a70efc98feecd6b2db803435b59b0f31d1439cf3` 的本机 image ID=`sha256:df24cd01c7e86c9d7ca562784d3adc335d89d786fbd9b226d513b5d14f991404`，version=`0.4.14`、完整 revision、build date=`2026-08-13T08:25:59Z`；fresh/restart 通过。官方 v0.4.11 → 该精确候选再次先观察 unhealthy 目标并恢复 v0.4.11，再以健康镜像升级成功，事务 `5191eb70656c`，SQLite `ok`、save/Mod/backup 三哨兵与重启终态保持，任务资源为 0。该 SHA Compatibility workflow `31682006066` 成功。
- annotated tag `v0.4.14` 的 Release workflow `31682847388` / job `94392047913` 于 `2026-08-13T08:37:12Z` 开始、`08:44:32Z` 完成，总时长 7 分 20 秒；release gates 4 分 02 秒、build/push 2 分 24 秒，所有步骤成功。GitHub Release `Stardew Server Anxi Panel 0.4.14` 于 `08:44:23Z` 发布，非 draft/prerelease。
- Docker Hub、GHCR、阿里云 ACR 的 `0.4.14/latest` 六个引用统一为单一 OCI image manifest digest=`sha256:5b58ad998da14726b655f4a965c0e3f74ae7839fe615b0f59dd8af1ee16a8ebd`，version=`0.4.14`、revision=`a70efc98feec`、created=`2026-08-13T08:41:50Z`。三个精确镜像分别用独立 container/network/volume 首次启动并重启，均 Docker health=`healthy`、database=`ok`、版本/commit 精确、fresh setup 未初始化，最终资源为 0；成功覆盖了 v0.4.13 的 Docker Hub 部分发布。
- Release 四项资产均与 tag 源逐字节一致：`run.sh`=`7263bfa323b2bf4eb94674bde9c77a57a8b86734c606055c9cdef2fc1e130787`、`migrate-fnos.sh`=`90510768d6636917fb7f15937a7dce34c34974dd8c9af5451030560eca57cbfd`、`repair-junimo-0.3.5.sh`=`13a07708d23e02c002c979eef28639bc2fe283a2e5988e228afc0c068f51cd0e`、`repair-junimo-upgrade.sh`=`4f3c666770b6be77ed51895264f47c940b066d61386b66b3653a858e8929b4c2`；下载的 `run.sh` 明确包含运行态和持久态 `vm.swappiness=60` 逻辑。

## Tag 前收口（已完成）

1. 最终 commit `a70efc98feecd6b2db803435b59b0f31d1439cf3` 只包含 attestation 兼容修复、规则和发布证据；tag 前 `main==origin/main`、单 worktree/单 main branch、工作树洁净。
2. 精确 `0.4.14` 候选已完成 OCI identity、fresh/restart、无 attestation 的受控 registry manifest，以及关键 Web unhealthy rollback + healthy apply。
3. annotated `v0.4.14` 从该提交创建并保持不可移动；失败的 `v0.4.12/v0.4.13` 未移动或复用。

## Tag 后收口

- [x] Release workflow、三仓精确版/latest digest/metadata、逐仓独立 health/version/restart smoke 已完成。
- [x] GitHub Release 四项资产与 tag 源字节一致，`run.sh` swappiness 修复已核对。
- [x] 最终候选的 Control 0.3.1、双 writer 新建档、图形化 conversion、正式 Web unhealthy rollback/健康升级和升级后 UI 证据已完成。
- [x] post-release `main` 提交 `c0cc94cb56ae` 的 Compatibility `31684078849` 与 Pages `31684078868` 成功；官网首页和更新日志的线上 HTTP 正文命中 v0.4.14、Control/新建存档摘要、swappiness 与宿主重启后手动启动边界。
- [ ] 生产真机同步被 SSH 用户名阻塞：`114.55.142.107:22` 可达，但 `cz`/`root` 均被拒绝。取得正确用户名后，先完整备份并确认无人写入，使用精确 `0.4.14` 走 Web 更新/图形化 Compose 转换；游戏保持关闭，核对 Panel health/version、Control 0.3.1、SQLite/存档/Mod/备份、非目标资源和手动启服边界。

# PANEL-UPDATE-GRAPHICAL-COMPOSE-1 发布门禁：图形化 Compose 自动标准化（2026-08-12，未发布）

## 变更清单与受影响链路

- 部署能力链从“Compose 身份反查成功即支持直接 apply”改为“身份反查成功后还必须证明部署目录 `.env` 可显式解析，且目标 service 的 image 精确受 `PANEL_IMAGE` 控制”。探针在当前可信 Panel 镜像的隔离、无网络 runner 内执行，只输出解析后的单一 image，不读取或记录 env 内容。
- 完整 Compose labels 但缺 `.env`、坏 env 或写死 image 的安全 NAS 部署不再进入普通 apply 后报 `deployment_backup_failed`；能力返回 `conversionRequired=true`，Web 一键升级复用 `/app/panel-updater convert` 与 `/app/migrate-fnos.sh`，在数据库在线备份后标准化部署并切到精确目标版本。身份歧义或特殊权限/挂载仍拒绝。
- Go 标准化前置检查允许额外 bind/volume 且要求目标唯一、非根目录；迁移脚本继续负责 RW、propagation、volume name、tmpfs、设备和网络的权威二次校验。NAS Compose 未覆盖镜像 `VOLUME /data` 时产生的匿名 volume 必须作为 external volume 保留，不能删除或误当用户数据卷清理。
- 标准 `.env` 中缺少 `PANEL_IMAGE=`、但 Compose 已被探针证明消费该变量时，apply 备份后原子追加精确目标；已有键仍只改值。正式镜像必须继续包含 docker-cli、Compose plugin、`/app/panel-updater` 和 `/app/migrate-fnos.sh`。

## 故障矩阵

| 场景 | 预期措施与发布证据 |
| --- | --- |
| 标准 `.env + ${PANEL_IMAGE}` | capability 直接 supported、conversionRequired=false；dry-run/apply 成功且不重写 Compose 结构 |
| 图形化 Compose 无 `.env`、image 写死 | 身份先唯一反查，再返回 conversionRequired=true；Web 一键完成备份、标准化、精确目标切换和断线恢复 |
| `.env` 存在但无 `PANEL_IMAGE` | Compose 确实消费变量时原子追加并成功；不消费时进入安全转换或 unsupported，不能在 pull/recreate 后才发现 |
| service label/容器 ID/镜像/数据挂载漂移 | fail closed 为 compose_metadata_invalid；不得借标准化覆盖有歧义的声明 Compose |
| privileged、自定义 user、缺 Socket、非 bind 数据目录、根/重复/不可保真挂载 | 修改前 unsupported；错误指向具体 env/image 契约及安全标准化失败，不创建 helper cutover |
| 额外匿名 `/data` volume、合法 bind/具名 volume | 二次 inspect 后按 external 语义保留；新旧容器核对相同目标、RW/propagation/name，非目标 volume 不删除 |
| 目标 registry 超时/断流/部分候选失败 | 按可信候选有界回退；全部失败时旧容器仍运行、数据库/部署不变，可幂等重试 |
| 目标 unhealthy、版本不符、labels/config/image ID 不符 | 删除失败新容器，恢复原 Compose/env/数据库、旧容器名称与 restart policy，并重新通过旧版 health/version |
| helper/Panel 在备份、文件替换、旧容器 rename/stop、新容器启动后中断 | 从持久 apply/fnos-migration 材料恢复；至少分别注入一次，终态只能 succeeded、failed_rolled_back 或 rollback_failed，不猜测成功 |
| 重复点击或失败后重试 | active apply 继续拒绝并返回同一任务；安全终态修正条件后使用新 update ID，旧备份不覆盖新事务 |
| 数据完整性与资源清理 | SQLite、初始化、用户、实例、存档、Mod、备份、审计保持；游戏容器/volume ID 不变；只清理本事务拥有的失败新 Panel/临时文件，旧容器按迁移契约保留 |
| 权限与敏感信息 | 仅管理员可 dry-run/apply 且必须 confirmFullStack；`.env`/inspect/secret 不进入公开日志、状态、支持包、镜像层或命令参数 |

## 当前验证与后续正式版本要求

- 已完成 updater 专项 Go 契约测试：变量探针、显式 env 失败、写死 image、安全/不安全标准化、service label 漂移、额外匿名 volume、conversion helper 参数和 env 缺键追加。后端 `go test ./... -count=1`（14 个包）、`go vet ./...`、`go build ./...`，迁移脚本 `bash -n`/功能测试/ShellCheck，前端 panel-update/update-status 状态测试与 production build，以及 VitePress production build 均通过。
- Docker Desktop Linux 29.5.3 构建未发布测试候选 `anxi-panel-graphical-e2e:0.4.10`（image ID `sha256:882c8acd175659b67caee50d42fc97ae99d1c52e1d0838e2c540b9c77068a75f`），在唯一 DinD 中用完整 Compose labels、无 `.env`、写死 image、宿主绝对路径 bind 和镜像声明产生的匿名 `/data` volume 创建反馈同款旧 Panel。真实 Web check、dry-run、`confirmFullStack` apply、断线重连在 79.3 秒内升级到正式 `0.4.11`（image ID `sha256:7c2fea3496ac1ec4afa2ae50f1087f469151e46b18a9c202bd7d4e70f16bb86e`）并进入 `succeeded`；标准 `.env + image: ${PANEL_IMAGE}` 自动生成，project 变为 `anxi-panel-anxi-panel-graphical-e2e`，游戏容器 ID与匿名 volume `7dad24607ba6c957236457ec5389d31060dc44a0a92a91e61474097a35193423` 保持，旧 Panel 停止保留。
- 前两轮隔离环境的非受控转换尝试均收敛为 `failed_rolled_back` 且旧 `0.4.10` 重新 healthy；同一迁移脚本在恢复现场成功，第三个全新 DinD 的完整 Web 请求成功。这两次失败未做受控注入，不能归因或充当目标失败门禁证据。全部三轮任务 container/network/volume、临时 tar/脚本和测试候选镜像都已按精确 owner/name 清理；未使用 prune，未触碰宿主其它 Panel/游戏资源。功能仍未发布、未创建 tag、未更新 latest。
- 下一正式版本前仍须注入目标 unhealthy/版本不符及 helper 在文件替换、旧容器停止和新容器启动窗口的中断，验证上述回滚/恢复矩阵；随后执行兼容矩阵、Docker updater integration、精确候选镜像与上一正式版/最老受影响版升级等完整发布门禁。当前成功 E2E 不能替代这些尚未完成的正式发布证据。
- 若本版发布，代码或文档继续变化后须重跑受影响门禁，并按本文件总门禁补齐兼容矩阵、Docker updater integration、精确正式候选 health/version/OCI、上一正式版及受影响最老支持版升级、三仓回拉与 digest 核验。

# v0.4.11 发布门禁：安装排他、终态一致性与首次建档（2026-08-11，已发布）

- 目标正式版本为 `v0.4.11`，上一正式版为 `v0.4.10`。本版发布 `INSTALL-FIRST-RUN-CONSISTENCY-1`、`FE-INSTALL-AUTHORITY-1` 与 `AUTH-CANCEL-RESOURCE-CLEANUP-1`：同一实例安装任务只有一个持久 owner，终态安装不会被迟到状态或旧日志复活，SMAPI 内置支持 Mod 在首次 server entrypoint 之前完成原子物化，二维码认证取消/超时不会遗留 Compose one-off 容器。
- 本版新增数据库迁移 `012_exclusive_stardew_install_jobs.sql`，并改变安装、Steam 授权、首次创建存档、SMAPI 升级/回滚及安装页状态合并链路。因此除 `v0.4.10 → v0.4.11` 正式 Web 一键升级外，还必须从 runtime manifest 最低支持版 `v0.3.2` 直升候选；两条链都要验证数据库迁移、初始化、用户、实例、存档、Mod、备份、审计、非目标容器/volume、Panel 重启恢复和升级后第一次建档。
- 本节已完成 tag 前门禁与 tag 后收口。annotated tag `v0.4.11` 固定指向 `ef2580d2e58b170b5e5aa0079496f969228dd3f6`；正式 Release、三仓 `0.4.11/latest`、四项资产与回拉隔离冒烟均已核验，tag 未移动。

## 变更清单与受影响链路

- 安装 owner：`jobs.Spec.Exclusive → Store.CreateExclusiveJob → jobs partial unique index` 形成跨 goroutine/进程的活动任务排他。重复安装和 Steam 授权返回 `409 install_in_progress + details.jobId`；任务终态后允许重试。实例阶段写入以 queued/running job row 作为持久 lease，历史 runner 的迟到写入返回 `ErrJobNotActive` 并忽略。
- 安装终态 UI：dashboard jobs 与详情按 job ID 合并，terminal 快照单调胜过 queued/running；仅当前 active job 的日志可以推导下载、认证、QR 和进度。重复提交若返回同一 job ID，保留现有 SSE/日志；返回另一 ID 才切换观察对象。
- SMAPI 首次物化：当前 `GAME_DATA_VOLUME` 以只读方式挂入精确 server image 的 one-shot container，把实际 `/data/game/Mods` 复制到 Panel 管理 staging。manifest、必需组件、重复 UniqueID、版本、symlink/文件类型和 512 MiB 上限通过后才原子替换 `.local-container/mods/smapi`；相同全树 SHA-256 为 no-op。安装成功、首次建档事务/指纹、SMAPI staging 切换和 rollback 切回旧卷都使用同一 helper。
- 中断恢复：Panel 启动先由仍 active 的安装 owner 写 `install_interrupted`，再终结任务；SMAPI 发布下次执行会清理精确 `.smapi-sync-*`，并在 destination 缺失时恢复最近的有效 `.smapi-backup-*`。用户 Mod、存档、disabled 目录和其它 Docker 资源不在清理范围。
- 认证取消清理：Linux `RunSteamAuthTTY` 为每次 `docker compose run` 分配随机唯一 `anxi-steam-auth-<hex>` 名称；job context 取消或超时时，在前台 Docker CLI 退出后只查询并强制删除该精确名字。第一次 list 为空不能代表 daemon 不会晚到 create，生产清理必须在 20 秒边界内连续 3 秒确认精确容器缺席；正常完成仍由 `--rm` 清理，不按 project、volume 或模糊前缀删除其它容器。
- 发布工作流：`release.yml` 与 `compatibility-matrix.yml` 纳入 `test:install-state`，避免新状态机只在维护者本机验证。

## 故障矩阵

| 场景 | 预期措施与门槛 | 发布证据要求 |
| --- | --- | --- |
| 单次正常安装 | 只有一个 job owner；SMAPI 支持 Mod 物化并完成 volume 完整性校验后才写 `game_installed` | 后端全量、真实 Panel 安装链和候选镜像日志/终态 |
| 同实例并发安装/授权 | 数据库 partial unique index 保证一个 winner；其它请求 409 并返回同一 `jobId`，不启动第二 runner | 12 路 storage 并发、Web 409、真实 Panel 双提交与 job/日志基数 |
| 跨进程竞争与历史重复活动任务 | 原子 insert/index 拒绝第二 owner；迁移只保留最新活动安装，较旧者标记 failed | migration fixture、并发 storage 测试、升级后 index/任务状态查询 |
| 终态后迟到写入 | terminal job 不能再更新实例；前端 terminal 不被迟到 running 或历史日志复活 | `ErrJobNotActive`、Manager/driver 回归、前端 install-state 测试与真实页面刷新 |
| 任务终态后重试 | succeeded/failed/canceled 释放活动唯一约束，新请求可创建新 job；旧 ID 日志只作审计 | storage retry、真实失败后重试和新旧 ID/SSE 观察 |
| SMAPI 正常首次物化 | 当前 game-data volume 两个官方支持 Mod 和未来合法 bundled Mod 全量发布，预检与 entrypoint 实载集合一致 | unit + 真实 Docker helper + 真实第一次创建存档 |
| 相同树重复执行 | 全树摘要一致时 no-op，不改变目录 inode/内容，不重复堆积 backup/stage | unit、真实 Docker 二次同步和资源基数 |
| 缺失/坏 manifest、重复 ID、空版本 | staging fail closed，当前 managed tree 保持不变，不启动 server 或创建 transaction | unit 故障注入、API phase/code、原目录摘要 |
| symlink、非普通文件、超 512 MiB | 拒绝发布，不能越出 staging 读取或覆盖用户路径 | unit/受控文件树故障注入；不适合大文件实写时使用稀疏文件并核对上限 |
| Docker/image/volume 不可用、copy 非零退出 | `smapi_bundled_sync_failed`，现有 tree 和存档不变；修复环境后同一操作可安全重试 | fake/真实 Docker 故障、日志脱敏、重试成功 |
| Panel 在 stage 后中断 | 下次执行只清理精确 `.smapi-sync-*`，重新从权威 volume 构建 | unit 中断夹具、真实 Docker orphan staging 恢复 |
| Panel 在 old→backup 后中断 | destination 缺失时恢复有效 sibling backup，再幂等同步；坏 backup 不被发布 | unit + 真实 Docker rename 故障夹具、最终无 owned artifact |
| 发布 rename/清理部分成功 | rename 失败立即恢复旧 tree；恢复也失败时保留并报告精确 backup 路径，不继续建档 | unit 故障注入或文件系统代理、摘要与可恢复材料检查 |
| 新建存档事务失败/Compose 启动失败 | SMAPI 同步必须早于事务快照和 ExpectedFingerprint；失败不得留下 transaction，后续 server 失败走既有回滚 | lifecycle 测试、真实第一次建档失败注入、存档/事务目录基数 |
| SMAPI staging 升级失败/回滚 | 新卷切换后同步新 bundled tree；验收失败切回旧卷并同步旧 tree，不混用版本 | SMAPI update/rollback 测试、代表老版本升级后实载指纹 |
| Panel/容器重启恢复 | active 安装终结为 `install_interrupted`，实例状态与 job 一致；重启后不会恢复历史下载卡 | Manager recovery、候选 Panel restart、页面/job/API 复核 |
| QR 登录取消、超时或 Panel 停止 | 取消前台 Compose CLI 后按本次随机精确容器名删除 one-off auth，并连续 3 秒确认缺席以吸收 daemon 晚到 create；game-data/steam-session 是否保留仍由安装语义决定 | 可控晚到创建单测、真实 `.125/auth .2` 到 QR 后 cancel、job 终态后稳定缺席、案例容器/volume 归零 |
| 权限与安全 | 未登录 401、非管理员安装/授权 403；响应/log/support bundle 不含 Steam 密码、token、二维码或 session 内容 | Web 权限回归、脱敏扫描、测试仅用专用凭据且不写入制品 |
| 网络超时/断流 | 沿用有界镜像/Steam/SMAPI 下载重试；失败不会绕过完整性校验或留下假 installed | 远程制品门禁、真实 SMAPI 下载、受控断流/坏包回退 |
| 数据完整性 | 数据库 `integrity_check`、迁移集合、用户/实例/存档/Mod/备份/审计哨兵在升级、失败回滚和重启后保持 | `v0.4.10`、`v0.3.2` 两条升级链的 live/backup 摘要 |
| 非目标资源与清理 | 不重建/删除非目标 game container/volume；只删除本测试唯一 label/prefix 资源，不执行 prune | 前后容器 ID/StartedAt/volume inspect；终态容器/network/volume/端口为零 |
| 官网与发布产物 | 官网首页/changelog/安装/存档手册展示 v0.4.11 与恢复边界；三仓精确版/latest、Release 资产和 OCI metadata 一致 | VitePress 洁净 build、桌面/手机线上 QA、三仓回拉 digest/health/version |

## 候选与发布证据

- 候选提交前代码门禁已通过：后端 `go test ./... -count=1` 53.4 秒（`stardew_junimo` 49.258 秒、Web 33.564 秒）、`go vet ./...` 1.3 秒、`go build ./...` 2.4 秒；`internal/docker` integration 26.380 秒、真实 auth unhealthy/offline acceptance 6.357 秒、updater Docker integration 38.579 秒。
- SMAPI 专项已通过：精确 `.125` server image 的真实 Docker bundled sync 3.679 秒，覆盖首次发布、相同树 no-op、模拟 old→backup 中断恢复和 owned artifact 清零；Linux `golang:1.25-alpine` 空任务缓存真实下载 41,889,142 字节，11 秒完成摘要、ZIP、`0600` 和临时文件清理。Windows 权限语义的无效试跑已按错题本纠正，不作为产品失败或发布证据。
- compatibility manifest validate、panel version `0.4.11`、19 项 Python 测试和 remote artifacts 78.6 秒通过；三个 Bash 功能脚本、`bash -n` 与正式 ShellCheck 输入全部通过。前端唯一 Node 24 Linux 空卷执行 `npm ci`、production audit=0、13 项状态/响应式测试和 production build，正确完整仓库挂载的整轮为 31.3 秒，任务 container/volume 已清理。
- pre-final 候选 `fca8c01483a2efd9d6bd05db25a4d99015df36ed` 已完成 fresh health/version/restart；隔离 DinD 内 `v0.4.10` 先对 unhealthy `0.4.11` 收敛为 `failed_rolled_back / health_check_failed`，再对健康候选成功，`v0.3.2` 依历史空 apply body 直升成功。两条链的 live/backup SQLite integrity、迁移 012、用户/实例/审计、存档/Mod/备份哨兵、非目标 container ID/StartedAt/volume 及 Panel restart 均通过；升级后的候选双提交返回同一活动 job 的 `409 install_in_progress`。
- 追加取消清理与文档后的临时候选 `3628358482231b0a8245533a4237a63daa324a22`（build date `2026-08-11T17:08:42Z`）完成 fresh health/version/restart；`v0.4.10` unhealthy 自动回滚后健康升级成功，`v0.3.2` 直升、迁移、重复安装 409、备份/哨兵/非目标资源及重启恢复均通过。升级得到的 Panel 又以真实 1.96 GiB game-data 创建活动存档 `Upgrade Gate Final`，SMAPI 物化 sequence 9 早于事务 sequence 10，Panel 重启不重建 server/auth 容器。该候选随后被更严格的取消资源门禁淘汰，不得用于 tag。
- 无账号真实首次安装 gate 使用 `.125` server/auth 正式组件到达 QR 后取消。第一版精确删除测试 3.48 秒即返回，但更外层资源审计随后复现一个 daemon 晚到的 `Created` auth one-off 和两个仍被占用的案例卷，证明“一次为空”不是清理终态。生产清理改为连续 3 秒缺席窗口，真实测试也在 job 终态后另做连续 3 秒确认；修正后的源代码真实 gate 9.78 秒通过，外层复核案例 container/volume 为 0。早期 3.48 秒结果不再作为发布证据。
- 稳定缺席修复后的 Windows 后端全量 59 秒、vet 0.5 秒、build 1.8 秒通过；Linux `internal/docker` 晚到创建单测通过，Linux 全量以 JSON 事件过滤的空缓存复跑 77.6 秒通过，vet 26.3 秒、build 3.4 秒通过且任务容器/缓存卷为 0。此前一轮 Linux `internal/web` 包级非零的普通文本输出被正常 HTTP 日志截断，按错题本改用结构化失败投影后定向包和完整全量均通过；那次无具体断言的非零不作为通过证据。
- 真实首次建档 gate 只读克隆历史测试所有的完整 game-data volume 到唯一任务卷，以空 saves bind、空 Steam 凭据和当前 `.125/auth .2` 运行栈启动 Junimo/SMAPI；两轮分别 71.78/60.04 秒创建唯一活动存档 `Release Gate`，存档元数据可解析。job log 中 SMAPI 物化为 sequence 9、建档事务准备为 sequence 10，两个 support manifest 均非空，成功终态无 `.smapi-*` staging/backup 残留，第二轮还经 driver Stop 证明 Compose 容器归零；源卷从未可写挂载。
- 取消清理与首次建档补丁后的 Windows 后端全量为 59.8 秒，vet/build 通过；Linux 冷缓存全量首次暴露异步 apply 测试 8 秒观察窗不足，目标子例在同一 Linux 环境连续 5 次约 3.18 秒通过。测试观察窗改为 20 秒后受影响包 44.631 秒、Linux 全量 54.2 秒及 vet/build 全绿，产品注入超时仍保持毫秒级。
- 最终 tag 候选为 `ef2580d2e58b170b5e5aa0079496f969228dd3f6`，本机候选 build date=`2026-08-11T17:58:15Z`、OCI index ID=`sha256:dd6c25d97c4c3b3fc181c4766a6a4a280d3b6af822f967e189d5d8eba8585af6`，内嵌 version=`0.4.11`、完整 revision 与提交一致；fresh health/version/restart 通过。Windows 后端全量 test/vet/build 与 Linux JSON 结构化全量 test/vet/build 复跑通过，最终 tag 源码真实 QR cancel gate 9.64 秒通过并连续确认 one-off 缺席。
- 最终隔离一键升级夹具以唯一 DinD/受控 Release+registry 完成 `v0.4.10 → ef2580d`：unhealthy 目标先精确收敛为 `failed_rolled_back / health_check_failed` 并恢复 0.4.10，健康目标随后升级到精确 0.4.11；`v0.3.2 → ef2580d` 使用历史空 apply body 成功。两条链均验证 SQLite integrity、迁移 012/index、初始化、用户/实例/审计、存档/Mod/备份哨兵、可读事务备份、非目标容器/volume 不变及 Panel restart。升级后的 Panel 双安装请求返回同一活动 job 的 202/409；再以真实 1.96 GiB game-data 创建活动存档 `Tag Release Gate`，SMAPI 物化 sequence 9 早于事务 sequence 10，重启后存档可读且 server/auth 容器 ID 不变。
- tag 前本地 `main` 干净且与 `origin/main` 同步；同一提交的 compatibility workflow `31521174829` / job `93878270277` 1 分 36 秒成功。annotated tag 推送后 Release workflow `31521478699` / job `93879265541` 于 `2026-08-11T18:12:11Z` 开始、`18:19:03Z` 成功结束；job 6 分 52 秒、run 7 分 03 秒，其中 release gates 3 分 40 秒、镜像 build/push 2 分 23 秒。GitHub Release `Stardew Server Anxi Panel 0.4.11` 于 `18:18:54Z` 发布，非 draft/prerelease，自动简略正文随后按真实用户变更补为完整中文说明，没有移动 tag。
- Docker Hub、阿里云 ACR、GHCR 的 `0.4.11` 与 `latest` 六个远端引用统一为 OCI index `sha256:7c2fea3496ac1ec4afa2ae50f1087f469151e46b18a9c202bd7d4e70f16bb86e`、linux/amd64 manifest `sha256:f916037c571eac6962a4f6448e08c425e8e0b8956679835808d4e2c10f78d02c`；OCI version=`0.4.11`、revision=`ef2580d2e58b`、created=`2026-08-11T18:16:22Z`。三个精确引用分别实际回拉，以独立容器/network/volume 启动；首次及 restart 后均 Docker health=`healthy`、`/health.status=ok`、`database.status=ok`、`/api/version=0.4.11@ef2580d2e58b`、fresh setup 未初始化，任务容器/network/volume 终态均为 0。
- 四项 Release 资产实际下载后与 tag 源文件逐字节一致：`run.sh` 30,437 B / `8f0040c11661f2e3f4060c66bf8ba205a33aa46fc65e3dec7cbf15b864c7387a`，`migrate-fnos.sh` 34,269 B / `90510768d6636917fb7f15937a7dce34c34974dd8c9af5451030560eca57cbfd`，`repair-junimo-0.3.5.sh` 14,413 B / `38d06d09e5c17db3145ec3b938f4d6844d1f2f058c73fa5bc72c804335eee47b`，`repair-junimo-upgrade.sh` 8,521 B / `4f3c666770b6be77ed51895264f47c940b066d61386b66b3653a858e8929b4c2`。
- 本次测试仅清理唯一任务标签/前缀的容器、网络、volume、候选镜像与隔离 bind；未使用任何 prune，真实来源卷从未可写。空的本地资产下载目录不进入 Git，四个临时下载文件与 Release 说明文件已由精确补丁删除。
- 官网 post-release 提交 `e3d40b155dd29cefe1fc9410675bbc91eb91d455` 的全新 Node 20 Linux production audit=0、critical audit=0，VitePress build 6.88 秒通过；Pages workflow `31523817426` 用时 42 秒、deployment `5856456646` 成功，compatibility `31523817397` 用时 1 分 42 秒并成功。本地 Browser 在 1440×900/390×844 完成首页、实际点击 changelog、v0.4.11 最新/v0.4.10 历史、安装/存档新增内容、横向溢出、overlay 和 console health 验收。线上首页、changelog、安装、存档四个 URL 均 HTTP 200 且 SSR 正文命中精确 v0.4.11 内容；本 turn 因本地检查后过早 finalize Browser，会话规则不允许追加线上截图，未把线上 HTTP 证据夸写成线上视觉证据，执行问题已记入错题本。

# v0.4.10 发布门禁：弹窗拉伸与 Steam 认证等待（2026-08-09，已发布）

- 目标正式版本为 `v0.4.10`，上一正式版为 `v0.4.9`。本版同时发布 FE-MODAL-HEIGHT-GUARD-1、FE-NEW-GAME-MODAL-LAYOUT-1、RUNTIME-AUTH-OFFLINE-ACCEPTANCE-1 与 FE-STEAM-AUTH-WAIT-VISIBILITY-1。
- 运行栈验收语义发生变化，因此除 `v0.4.9 → v0.4.10` 正式 Web 一键升级外，还必须从 runtime manifest 支持下限 `v0.3.2` 直升候选；两条链都要验证 Panel health/version、数据库与初始化、用户/实例/存档/Mod/备份/审计哨兵、非目标游戏容器/volume、终态重启恢复及升级后新功能。
- 本节已完成 tag 前门禁和 tag 后发布收口。annotated tag `v0.4.10` 固定指向 `7d9d0e267d942952701bc14ac19d032951d2dfd7`；正式 Release、三仓 `0.4.10/latest`、回拉隔离冒烟和 Release 资产均已核验，证据见下文。

## 变更清单、受影响链路与故障矩阵

- 前端弹窗链路：危险确认框、Steam 二维码和存档/新建游戏宽弹窗统一获得同时受 viewport 与 overlay 约束的有效高度，并用 `border-box` 把 padding/border 纳入限宽限高；新建游戏 1100/480px 容器查询只读取 `ngc-modal` 自身宽度，不再被桌面侧栏后的 `sd-main-scroll` 错误触发。
- 认证升级链路：`recreating_auth → verifying_auth → recreating_server → verifying_server` 不再把 Steam 在线相关 Docker health 当硬门槛；只要求 steam-auth 容器 running、目标 image ID 匹配、`/steam/ready` 命中受支持的 HTTP/schema 合约。HTTP 200 接受 legacy/current schema，current `accounts` 必须是 JSON array；真实 auth 镜像的 HTTP 503 只接受受支持的 legacy `ready=false` 离线 schema。HTTP 500、其它状态、503 current/畸形 schema、接口不可达或 digest 漂移均 fail closed 并回滚。
- 状态展示链路：`verifying_auth` 继续使用既有 apply `phase/progress/updatedAt`，用户卡与技术详情显示“正在尝试 Steam 连接”、累计等待、自动刷新及“不是卡死”，不新增 API 字段。读屏实时区域只播报静态阶段标题，持续变化的等待秒数不进入 live region，避免每 1.8 秒重复播报。
- 发布工作流同步加入真实 unhealthy/logged-out auth integration，保证 tag workflow 不能只跑 mock 单测就发布。
- 安装入口供应链：用户文档只允许从官方 GitHub Release HTTPS 下载 `run.sh` 后执行；仅支持 HTTP 的镜像不再作为可执行脚本入口。HTTP 200/长度不能替代传输完整性，后续只有在镜像提供可信 HTTPS 或独立签名/摘要校验后才能恢复推荐。
- 洁净 `npm ci` 发现 GitHub Reviewed high advisory `GHSA-2v37-7h3g-55p8`；Vite/PostCSS 的传递依赖 `nanoid` 从 `3.3.16` 精确锁定到修复版 `3.3.17`。必须在空 Node volume 重新 `npm ci`、`npm audit --omit=dev --audit-level=high`、12 项状态测试和 production build，audit high/critical 必须为 0。
- `compatibility-matrix.yml` 与 `release.yml` 均把前端 production audit 加入 `npm ci` 后的正式门禁，避免 lockfile 后续回退时只有本地审计能发现。
- 官网洁净依赖审计同样发现旧 lockfile 的 `nanoid 3.3.15`，并发现 `postcss 8.5.16` 命中 source-map path traversal high advisory；分别锁定修复版 `3.3.17`、`8.5.25`。npm 当前稳定 `vitepress latest=1.6.4`，其 Vite 5/esbuild 0.21.5 链仍报告 1 high + 2 moderate 的开发服务器读取/路径绕过公告且无稳定修复；2.0 只有 alpha，不作为正式发布依赖。正式产物只执行 build/发布静态文件，不运行或暴露 Vite/VitePress dev server，因此记录为不适用运行时边界，不能把完整 dev audit 误报为 0。

| 场景 | 预期措施与门槛 | 发布证据要求 |
| --- | --- | --- |
| 正常 auth 已在线 | running + digest + 可解析接口通过，升级继续 | 单测、Docker integration、真实候选升级 |
| Docker health unhealthy、Steam 未登录/无 ticket | 不等待在线登录；追加后台重试 warning | 真实容器固定 unhealthy 且接口返回 logged-out，必须在 auth timeout 前成功 |
| auth 接口超时/断流/坏 JSON | 有界轮询后 `auth_service_not_ready`，按事务回滚 | 故障注入与旧版恢复/状态终态检查 |
| auth HTTP 500/其它状态或 current schema `accounts` 非数组 | 即使响应体含 `ready/status=ok` 也拒绝；仅白名单 HTTP 503 + legacy `ready=false` | 单元 500、503 current/畸形 schema、坏 accounts + 真实 Docker 404/503 探针 |
| auth digest 不匹配 | 立即 `auth_digest_mismatch`，禁止继续 server 重建 | Go 回归与回滚状态检查 |
| 部分成功、重复探测与幂等 | `/steam/ready` 只读；最终目标复验走同一门槛 | 同一事务重复状态读取不产生额外 mutation |
| Panel/容器中断 | 沿用 schema 3 write-ahead 恢复或回滚，不猜阶段 | 既有全量恢复测试 + 候选升级后重启终态 |
| 失败回滚与数据完整性 | 不删除 game-data、steam-session、存档、Mod、备份；非目标容器不重建 | Panel 自更新 unhealthy 候选自动回滚、哨兵摘要与容器 ID/volume 核对 |
| 权限与安全 | 状态、日志、文档不含 Steam 账号/密码/token/ticket；匿名升级接口仍拒绝 | diff/支持包/接口权限检查；测试只用专用本地账号 |
| 安装脚本供应链 | 不从 HTTP 下载后直接执行；只使用官方 Release HTTPS | README、用户指南、官网命令检索；HTTP 可执行入口为 0 |
| 桌面宽弹窗 | 以弹窗自身宽度保持三栏，边框/内边距不撑破 overlay | 1180×1063 三列尺寸、交互、无横向溢出 |
| 窄屏/低高度弹窗 | 预期切单列，内容只在弹窗内部滚动；确认框/QR 不越界 | 769×500、390×844 普通视口截图与 root/body 宽度 |
| 网络与远程制品 | 受审镜像/SMAPI/Git 候选仍有界重试、摘要与来源 fail closed | compatibility remote artifacts 与真实 SMAPI 下载 |
| 前端/官网依赖安全 | 所有有稳定补丁的 high 均修复；官网唯一无修复 high 限定为不发布的 dev server | 两个空 volume npm ci/audit、前端 high/critical=0；官网 production=0、critical=0、静态 build 通过 |
| 资源清理 | 只清理本测试唯一 project/label/端口/volume/image | 每轮终态精确查询为空，禁止任何 system/volume prune |

- 补齐 `border-box` 后的浏览器几何回归：769×240 与 280×653 长 Joja 确认框四边均在 overlay 内，root/body 横向溢出为 0、console warn/error 为 0；769×240 卡片 `scrollHeight=303 > clientHeight=210`，可从 `scrollTop=0` 滚到 `93`。升级得到的 v0.4.10 Panel 又用合成、非敏感 Steam QR URL 实测 769×240 与 280×653 二维码弹窗：卡片四边均在 overlay 内且无横向溢出，低高度内部 `maxScrollTop=225` 并可滚到底，280×653 完整装入，关闭交互与 console 均正常。

## Tag 前代码、依赖与公网制品门禁证据（候选构建前）

- 后端最终差异执行 `go test ./... -count=1` 64.8 秒（`stardew_junimo` 59.223 秒、Web 40.235 秒）、`go vet ./...` 1.5 秒、`go build ./...` 2.6 秒，全部通过。完整 `internal/docker` integration 12.048 秒通过；真实 `.125 / auth .2` 无凭据合约 2.71 秒通过；真实 HTTP 503 + Docker unhealthy/offline auth 不等待 health 专项 11.64 秒通过。
- 真实 SMAPI 空 Linux 缓存下载 41,889,142 字节并完成摘要、ZIP、0600 和临时文件清理；测试本体 7.25 秒。updater 隔离 Compose 成功、失败回滚与 helper 清理基础集 34.641 秒通过；精确正式候选直升在候选镜像生成后补跑。
- 兼容矩阵 validate、panel version、19 项 Python 测试通过；`verify-remote-artifacts` 在可选镜像源、Docker Hub、SMAPI 分块及 Git TLS 有界恢复后 120.3 秒通过。三个 Bash 功能脚本与语法 2.7 秒、正式 ShellCheck 2.4 秒通过。
- 前端在唯一空 Node 24 volume 全新 `npm ci` 后 production audit 0、12 项状态/响应式测试及 build 全部通过，总计 30.1 秒（build 1.55 秒）；任务容器/volume 已按精确 label 清理。官网空 volume 的 production audit 为 0、完整 audit critical=0；剩余 Vite dev-server 1 high + 2 moderate 无稳定修复且不进入静态产物，修正可写配置边界后 VitePress build 本体 4.48 秒、整轮 47.9 秒通过，任务容器/volume 已清理。
- 候选构建前门禁、精确候选、fresh smoke、两条代表版本升级、Panel 自更新 unhealthy 候选自动回滚、升级后 UI、同步干净 `main`、tag workflow、三仓回拉、Release 资产和隔离冒烟现已全部完成；正式发布证据如下。

## 正式候选、真实升级与升级后功能证据

- 最终候选提交为 `7d9d0e267d942952701bc14ac19d032951d2dfd7`。本机 Docker Desktop Linux 候选 `anxi-v0410-candidate:7d9d0e2` 的 image ID 为 `sha256:846435294a70844a6a0d00a65e7bb496bac2d47465141eac2e6d50f9973d2d1b`，内嵌 version=`0.4.10`、完整 revision=`7d9d0e267d942952701bc14ac19d032951d2dfd7`、created=`2026-08-09T15:39:43Z`；fresh health/version/restart 通过。tag 前 `main` 工作树干净且与 `origin/main` 同步，compatibility workflow `31321583191` 在同一 SHA 成功。
- 正式 Web 一键升级门禁使用唯一任务 DinD、受控 TLS `api.github.com` Release endpoint 和受控 TLS `ghcr.io` registry，全程走 setup/login cookie、update check、capability、dry-run、管理员 apply、断线重连与 apply 终态，而不是直接调用 updater。`v0.4.9` 的 Panel 自更新 unhealthy 候选事务 `6453365330c7` 用时 2 分 14 秒，实际观察到目标容器 `running + unhealthy` 且容器内 health/version 为目标构建，最终精确为 `failed_rolled_back / health_check_failed` 并恢复旧镜像；同一受控 registry 的 `0.4.10` 镜像引用切回健康候选后，新事务 `33d98c68a74f` 用时 12 秒成功并通过重启，Git tag 从未移动。`v0.3.2` 按历史空 apply body 契约执行事务 `d6a33fe33ce8`，10 秒成功并通过重启。完整夹具 3 分 49 秒，Release fixture 命中 12 次、目标 registry manifest 命中 16 次。
- 两条升级链和失败回滚均检查 live/backup SQLite `PRAGMA integrity_check`、迁移集合、初始化、用户、实例与审计；Panel/Mod/save/有效 ZIP backup 文件哨兵摘要保持不变。每个 updater backup 的数据库、Compose、deployment env 与 manifest 均可读；非目标 game container 的 ID/StartedAt、volume inspect 字段、挂载和 sentinel 均未变化，没有第二个 game 或残留 updater helper。
- 在上述升级得到的新 v0.4.10 前端 bundle 上复验 `verifying_auth`：769×240 的累计等待从 664 秒增至 666 秒，280×653 从 667 秒增至 669 秒；两视口均只有一个 `role=status`，标题为“正在尝试 Steam 连接”，动态秒数和技术详情不在 live region，root/body 无横向溢出且 console error/warn 为 0。二维码弹窗的真实 DOM、aria-modal、data image、内部滚动和关闭交互也在同一升级后 bundle 上通过；测试二维码为合成 URL，不含真实 Steam token。

## Tag、Release、三仓与资产核验

- annotated tag `v0.4.10` 的 peeled commit 精确为 `7d9d0e267d942952701bc14ac19d032951d2dfd7`，未移动历史 tag。Release workflow `31325589153` / job `93275302795` 于 `2026-08-09T17:07:48Z` 开始，`17:13:50Z` 成功结束；总耗时 6 分 02 秒，job 5 分 59 秒，其中 release gates 3 分 34 秒、镜像 build/push 1 分 39 秒。GitHub Release `Stardew Server Anxi Panel 0.4.10` 于 `17:13:42Z` 发布，非 draft/prerelease；自动正文因 tag message 缺少详情而错误回退到最后一个文档提交，发布后按真实变更修正 Release 正文，没有移动 tag。
- workflow 推送和发布后独立远端查询一致：Docker Hub、阿里云 ACR、GHCR 的 `0.4.10` 与 `latest` 六个 OCI index digest 均为 `sha256:c37ad8e8d1498f377900b8a82e2ad1de761df23a06f1cb298ae349a362b111df`，linux/amd64 manifest 均为 `sha256:7534a30c283e9497ee6533dae4dc82f443779700ee90eb72858dfc49d43d9070`；OCI version=`0.4.10`、revision=`7d9d0e267d94`、created=`2026-08-09T17:11:52Z`。三个精确版本均按不可变 index digest 实际回拉，以独立容器/network/volume/端口启动并在首次和 restart 后返回 Docker health=`healthy`、`/health=ok`、database=`ok`、精确 `/api/version` 与 fresh setup；三个仓库结果完全一致。
- Release 的四项 HTTPS 资产均重新下载并按字节复算 SHA-256：`run.sh` 30,437 B / `8f0040c11661f2e3f4060c66bf8ba205a33aa46fc65e3dec7cbf15b864c7387a`，`migrate-fnos.sh` 34,269 B / `90510768d6636917fb7f15937a7dce34c34974dd8c9af5451030560eca57cbfd`，`repair-junimo-0.3.5.sh` 14,413 B / `38d06d09e5c17db3145ec3b938f4d6844d1f2f058c73fa5bc72c804335eee47b`，`repair-junimo-upgrade.sh` 8,521 B / `4f3c666770b6be77ed51895264f47c940b066d61386b66b3653a858e8929b4c2`；`releases/latest/download/run.sh` 与精确版本资产同为 30,437 B 和相同摘要。
- 正式 Web E2E、升级后 UI 与三仓回拉的任务容器、network、volume、bind、临时脚本和端口均按精确 owner 清理；三仓冒烟端口 `18150/18151/18152` 无 listener，未执行任何 `system prune`、`volume prune` 或模糊删除。正式回拉内容保留在 Docker Desktop 内容存储中，不删除用户已有镜像 tag。
- post-release 提交 `3457efea561f5fbb865eab440576e91cf2de6ec1` 的 Pages workflow `31326926817` 用时 33 秒、deployment `5821195957` 成功，compatibility workflow `31326926808` 用时 1 分 43 秒并成功。线上 1440×900 与 390×844 普通视口复核首页、实际点击更新日志、v0.4.10 最新条目和 v0.4.9 历史；root/body 宽度均等于视口，framework overlay、console warning/error、page error 与 request failure 均为 0。首轮截图命中 0.32 秒 Hero 入场与平滑回顶中间帧；记录 opacity/scrollY 时间线并等待稳定后，两视口均正确回到日志顶部且视觉正常。额外的极快“进入 → 返回 → 再进入/锚点”压力序列可触发 VitePress 1.6.4 默认 outline 空引用，A/B 在当前线上正式 CSS 也复现，证明不是 v0.4.10 或本轮布局改动引入；为避免以普通文档导航换来 back scroll 丢失，本次不改导航语义，后续项见 `docs/07-later-optimizations.md`。

## 不适用项与保留边界

- 本版没有数据库 migration、部署格式、存档格式、Mod 格式或公开 API shape 变化；无需新增迁移脚本，但仍必须验证代表老版本数据完整保留。
- Steam 是否在线是联机邀请码能力，不再是 Panel/运行栈升级成功条件；这不等于忽略认证服务本身，接口不可解析仍是硬失败。
- 前端布局修复不改变表单字段、创建请求、危险确认动作或 Steam 二维码内容；真实安装页面使用合成 QR URL 完成 overlay 几何、内部滚动、data image 与关闭交互复核，不记录或截图真实 Steam token。

# 历史记录：安装入口协议统一为 HTTP（2026-08-09，已撤销）

- 本节方案已在 `v0.4.10` 发布门禁中撤销：HTTP 200 与字节数不能抵御中间人替换，当前用户入口统一回到官方 GitHub Release HTTPS。以下仅保留当时问题背景，不再代表有效安装建议。
> 以下两点仅记录已撤销方案的历史证据，不得复制为当前安装指令：当时曾把入口指向明文 HTTP 镜像，并只验证 200、内容类型与字节数；提交 `55effaffb7f6bdae6091e8fef3eba5e017000e07` 及 compatibility workflow `31305603385` 只证明旧页面一致性，不能证明脚本传输完整性。当前有效命令以本文件顶部供应链说明和官方 GitHub Release HTTPS 为准。

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
    image: ${PANEL_IMAGE:-crpi-9z3bkb9g7fxeohrg.cn-hangzhou.personal.cr.aliyuncs.com/anxi-panel/stardew-server-anxi-panel:latest}
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
- 只粘贴单个 Compose、没有单独 `.env` 也可以首次启动。包含本修复的 Panel 在后续第一次 Web 一键升级时，会先证明容器/Compose/镜像/数据挂载唯一一致；满足安全边界后自动备份并转换成标准 `.env + ${PANEL_IMAGE}` 部署，再继续升级。用户不需要手工补文件；特殊权限、挂载或身份歧义会在修改前明确拒绝。
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
- 菜单 `[9] 设置虚拟内存` 会优先通过 `/proc/swaps` 判断 `/swapfile` 是否已启用，并兼容 `swapon` / `mkswap` 位于 `/sbin` 或 `/usr/sbin` 的 NAS 环境；如已有 `/swapfile` 但未启用，会先尝试移除后重建，避免直接覆盖导致 `Text file busy`。无论是复用已启用的 `/swapfile` 还是新建 swap，脚本都会立即把宿主 `vm.swappiness` 设为 `60` 并持久化；优先写 `/etc/sysctl.d/99-zz-stardew-anxi-panel-swappiness.conf`，同时规范化 `/etc/sysctl.conf` 中已有的冲突值，无 `sysctl.d` 时直接安全更新 `/etc/sysctl.conf`。运行值或持久化校验失败时命令明确失败，不把“swap 已启用”误报成完整成功。

用户首次启动：

官方 GitHub Release 安装（推荐）：

```bash
curl -fsSL -o run.sh https://github.com/anxiyizhi/stardew-server-anxi-panel/releases/latest/download/run.sh && chmod +x run.sh && bash run.sh
```

> [!TIP]
> **国内加速脚本（HTTP）**
>
> GitHub Release 下载较慢时，可以使用国内加速地址：
>
> ```bash
> curl -fsSL -o run.sh http://anxinas.dpdns.org/run.sh && chmod +x run.sh && bash run.sh
> ```

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

# Steam 在线等待解耦专项验证（2026-08-09，非发布记录）

- 历史说明：以下是 v0.4.10 当时使用 `/steam/ready` 的验收证据；该探针仍会在 Steam 网络挂起时阻塞，当前契约已由后文 `RUNTIME-AUTH-HEALTH-PROBE-1` 的严格 `/health` 方案取代，不得按本节复刻实现。
- 变更链路：运行栈 apply `recreating_auth → verifying_auth → recreating_server`；auth 验收从 Docker health 改为容器 running + 精确 image ID + 可解析 `/steam/ready`，前端消费原有 phase/updatedAt 展示等待详情。
- 故障矩阵：正常已登录接口通过；边界未登录/无 ticket 通过并警告；Steam 外网波动由 auth 后台重试且不阻塞升级；接口超时/断流/坏 JSON 仍失败回滚；digest 不匹配仍立即失败；重复目标复验使用同一幂等只读探针；Panel/容器中断恢复、认证卷快照、失败回滚和数据完整性逻辑未改；不读取或记录账号凭据；测试仅创建唯一前缀容器/镜像并精确清理。
- Docker Desktop 29.5.3 Linux containers 实机 fixture 将 healthcheck 固定为失败，同时由真实容器 HTTP 服务返回 `{"ready":false,"has_ticket":false}`。`TestRuntimeUpdateAuthAcceptanceDoesNotWaitForDockerHealth` 12.53 秒通过；旧实现会等待 auth timeout。清理后 `anxiauthoffline*` 容器和镜像均为空。
- 代码门禁：后端全量 `go test ./...`、`go vet ./...`、`go build ./...`；前端全部 12 个 `test:*` 脚本与 `npm run build` 均通过。Browser 桌面与 390×844 窄屏提示可见，窄屏 root/body 无横向溢出，console error/warn 为空。
- 本轮没有构建正式候选镜像、没有打 tag 或推送 registry。发布前仍须按本文件总门禁从最新正式版执行 Panel 一键升级、升级后功能复验、目标失败回滚和全量门禁，不能把本专项测试替代正式发布验收。

# RUNTIME-AUTH-HEALTH-PROBE-1 发布专项矩阵（2026-08-14，released in v0.4.17）

## 变更清单与受影响链路

- 根因：v0.4.10 至当前 v0.4.16 的运行组件验收请求 `/steam/ready`。该业务端点会恢复 Steam 会话、连接/登录并获取 App Ticket；在少数 Steam 网络受阻或连接很慢的环境中，Panel 的 15 秒单次 Docker 探针反复超时，最终耗尽 10 分钟认证预算并进入回滚。容器、目标 digest 和 HTTP 服务本身可能一直正常，因此问题只在特定网络环境稳定暴露。
- Runtime target/final verify、rollback old target verify、required-runtime/SMAPI 相关复验全部改为纯 `GET /health`；`/steam/ready` 保留给登录、ticket、邀请码和在线诊断，health 失败后没有 fallback。Docker health 也不作为替代证据。
- 受影响文件为后端 Docker runtime probe、Junimo runtime apply/rollback/SMAPI workflow、运行栈兼容 inspection、兼容矩阵脚本及测试；无数据库迁移、Compose 格式、认证卷布局、公开路由或 apply-status JSON shape 变化。兼容检查名 `steam_auth_ready` 保留，但文案和含义已改为严格 service health。
- 本节取代本文件此前所有把 `/steam/ready` 写成当前 Runtime/SMAPI 维护验收契约的说明；旧段落只保留版本历史价值。

## 本版专项矩阵

| 维度 | 场景 | 预期与现有证据 |
| --- | --- | --- |
| 正常路径 | `/health` HTTP 200，`status=ok`、`logged_in=true`、`accounts` 数组 | 单元与真实 `1.5.0-anxi.2` opt-in 通过 |
| Steam 离线边界 | `/health` 立即返回 `logged_in=false`，`/steam/ready` 阻塞 60 秒 | apply 快速成功并产生 warning；请求日志证明未访问 ready |
| schema 安全 | 非 200、空 body、非 JSON、缺字段、null/错误类型、status 非 ok、trailing JSON | 全部 fail closed，稳定映射 health 错误码 |
| 网络失败 | 不可达、短超时、404、500、坏 JSON | Docker integration 全部进入 `failed_rolled_back`，保留脱敏最后原因 |
| 镜像完整性 | auth image 变化、未变化、实际 image ID 与目标 digest 不同 | 变化与 Control-only 两条流程通过；mismatch 立即失败 |
| 重复/恢复 | 目标初验、最终目标复验、旧栈回滚、SMAPI 复验 | 统一调用严格 `/health`，无业务登录副作用 |
| 数据与资源 | steam-session、凭据、临时容器/网络/镜像/volume | 不读取或输出敏感值；任务资源精确清理后计数均为 0 |
| 兼容性 | 内置/历史清单和未审计旧 tag | 仅 `.2` 有审计证据；其它 tag 在 mutation 前以 `unsupported/auth_health_contract` 停止，绝不回退 ready |

## 已执行门禁与正式发布待办

- 已通过：严格 parser/错误码定向单测；Junimo Runtime/SMAPI/rollback 定向测试；Docker `/health` fixture；完整 apply/rollback Docker integration；真实推荐 server/auth opt-in；兼容矩阵 20 项单测与当前 manifest validate。
- Linux `golang:1.25-alpine` 隔离门禁已通过 `go test ./... -count=1`、`go vet ./...`、`go build ./...`。Windows 宿主定向包只命中既知 POSIX `0600`/`0666` 语义差异，已由 Linux 目标文件系统全量门禁覆盖，不是产品失败。
- 该修复阶段当时建议进入下一个补丁版 `v0.4.17`，且自身没有创建 tag、Release、候选或正式镜像；随后已由下节记录的最终 main 完成上一正式版 Web API 升级、unhealthy 回滚、数据/非目标资源保持、候选证明、digest 提升和正式回拉冒烟。

# v0.4.17 正式发布与候选矩阵（2026-08-15，released）

## 固定变更范围

- `RUNTIME-AUTH-HEALTH-PROBE-1`：Runtime/SMAPI/目标复验/旧栈回滚统一使用严格 `/health` 服务契约，Steam 离线只 warning；接口、schema、digest 或容器失败继续安全回滚。发布脚本的 Linux integration 筛选名已同步为真实测试 `TestRuntimeUpdateAuthAcceptanceUsesPureHealthAndNeverCallsSteamReady`，防止旧测试名导致 “no tests to run” 被误记为通过。
- `SAVE-IMPORT-FIRST-INSTALL-STATE-1`：真实安装终态 `game_installed` 与 `save_required / ready_to_start / stopped` 共享离线导入契约；driver 保留最终权威状态和 Docker 双门禁，不放宽 starting/running/安装中/未初始化。
- 首次上传发布前安全复审：Compose 安全证据改为无缓存、严格解析的 `docker compose ps --all`；权威 `DataDir` 贯穿导入链；状态/journal 写入错误显式失败；原 state/phase/payload 与 message NULL 语义精确恢复；Phase A FIFO 前失败会停 maintenance。job 使用 operation idempotency key，cleanup 使用先计划后删除与耐久 canceled marker，覆盖 jobId/token/journal 三类中断窗口；成功 token 到期精确回收。
- `FE-NEWGAME-COMMUNITY-BUNDLE-COPY-1`：仅把新建存档高级设置误写的“社区中心手机包”更正为“社区中心收集包”，不改字段、默认值或后端语义。
- 无数据库 schema migration、Compose 文件格式、运行镜像清单版本、公开 API shape 或 Release 资产变化；storage 只增加内部精确快照恢复方法。目标补丁版本固定为 `0.4.17`，上一正式版为 `0.4.16`。

## 本版专项矩阵

| 维度 | 必测场景 | 放行条件 |
| --- | --- | --- |
| auth 正常/离线 | `/health` logged_in true/false；`/steam/ready` 永久挂起 | 两者均快速通过服务验收；false 仅 warning；请求日志零 ready |
| auth 故障/回滚 | unreachable、timeout、404、500、坏 JSON、字段/type/status 错、digest mismatch | 稳定错误码、保留脱敏最后原因并 `failed_rolled_back`；旧栈仍按 `/health` 验收 |
| 首次上传正常路径 | `game_installed`、无活动存档、bootstrap→maintenance→Phase A→activation→durable save | 单次 FIFO import、runtime_ready 到 completed，目标/指针/备份保持 |
| 离线状态与 Docker | 四态允许；未初始化/安装中/starting/running 拒绝；缓存 stale、多个 server、restarting/paused/未知/坏 JSON | ownership 前拒绝所有不安全或不可分类证据；不创建危险事务 |
| 失败与精确恢复 | 状态发布失败、ComposeUp/readiness/cancel、Phase A 提交前失败、恢复写入失败 | 未提交时 strict stop 后精确恢复；写入/停机不可证转 recovery required，不伪装离线 |
| 中断与清理 | job 创建后 token 未 attach；无 durable job；cleanup 后 token 未删；token 已删 journal 未清 | idempotency key/terminal canceled marker 幂等收敛，不永久 busy、不删 preimport/未知文件 |
| cleanup 数据安全 | staged fingerprint 漂移、bootstrap pointer/ownership 漂移、未知 schema/stage | 所有只读门禁在首个删除前完成；任一不确定零部分清理、fail closed |
| 兼容与 UI | auth `.2` allowlist、20 项兼容矩阵；前端文案正反检查和全状态回归 | 未审计 tag mutation 前拒绝；正确文案进入 production bundle，字段契约不变 |
| 正式升级 | `v0.4.16` Web check/dry-run/apply；同候选 unhealthy 注入 | healthy 升级与 unhealthy 回滚均通过，SQLite、初始化、任务长期数据、非目标容器/volume 保持 |
| 正式提升 | candidate proof→annotated tag→三仓 version/latest→Release | 六引用同一 digest/OCI；正式镜像 health/version/重启通过；四项 Release 资产存在 |

## 推送前已完成证据

- Windows 定向与受影响包：save-import/maintenance/Web/token/storage/docker/jobs 专项与包回归通过；Docker 包 integration `16.495s`，updater Docker integration `38.202s`。
- Linux `golang:1.25-alpine`：`go test ./... -count=1` 全包通过，随后 `go vet ./...`、`go build ./...` 退出 0。strict Compose parser/cache、多状态、精确恢复、Phase A pre-submit、cleanup plan、未知 journal、权威 DataDir、job/token 中断测试均包含在该次全量门禁。
- 门禁自检发现并修复两处“假通过/易抖动”风险：发布脚本旧 auth 测试名会产生 `no tests to run`，现已绑定真实测试名；共享 job 终态观察器原 5 秒预算短于 fixture 自身 10 秒 job timeout，在全包 I/O 压力下曾提前结束，现改为有界 15 秒并以同一最终代码重跑 Linux 全量通过。没有跳过或降低产品断言。
- Linux DinD 真实 auth fixture：`TestRuntimeUpdateAuthAcceptanceUsesPureHealthAndNeverCallsSteamReady` 及 rollback 404/500/bad-json/timeout/unreachable 子例全部通过，`47.18s`；内层容器、network、volume 与 fixture image 清理后均为 0。真实 SMAPI 下载 `41,889,142` bytes，`77.06s` 通过。
- 兼容矩阵：Python 3.12.13 validate、`check-panel-version --version 0.4.17`、20 项单测通过；远程制品校验 `85.7s` 通过，只有清单允许的可选镜像 mirror unavailable warning。
- 前端 Node 24 隔离卷：`npm ci`、production audit（0 vulnerability）、全部 17 个 `test:*` 脚本与 production build 通过；完整仓库挂载保证 responsive-layout 读取真实 workflow，`node_modules/dist` 使用任务专属卷。
- 部署脚本：Git Bash 5.2.37 的全清单 `bash -n`、四项功能测试及 ShellCheck v0.10.0 通过。以上是 commit 前证据；自动候选、Web 升级 E2E、unhealthy 回滚、tag、正式 digest 与 Release 的不可变结果记录如下。

## 自动候选与上一正式版升级证据

- 发布提交为 `d63c93ffe7d65f8cdfcf2bedb9b336a6839be73f`，由干净且与 `origin/main` 同步的本地 `main` fast-forward 推送。候选 workflow `31823172958`（attempt 1）从上一正式版 `0.4.16` 自动解析目标 `0.4.17`，固定 build date=`2026-08-14T17:16:15Z`；Compatibility workflow `31823172972` 同步成功。
- 候选 artifact=`release-candidate-0.4.17-d63c93ffe7d6`，其中 `candidate.json` 固定 schema 1、完整 commit、workflow run 和引用 `ghcr.io/anxiyizhi/stardew-server-anxi-panel:candidate-0.4.17-d63c93ffe7d6`；候选 digest=`sha256:44c328cdf198ec888f3ec54bbe836ce114f5ac27c4ca5fb9cc63747a44083673`，本地构建 image ID=`sha256:840344e9148a0b6a2947cf4f6700cf1b53a3c0407c8da03505ee08dc6ceae6d1`。
- `Run selected code gates` 明确执行兼容契约与远程制品、部署脚本、后端、Junimo 真实 network/runtime integration、前端回归/build 和网站 build，最终输出 `all selected gates passed for 0.4.17`。真实 auth fixture `TestRuntimeUpdateAuthAcceptanceUsesPureHealthAndNeverCallsSteamReady` 及 rollback 404/500/bad-json/timeout/unreachable 子例全部通过；没有退回旧测试名或跳过挂起式用例。
- 候选镜像完成 fresh install/restart。隔离 DinD 从 `v0.4.16` 通过公开 Panel Web API 执行 check/dry-run/apply：同版本引用先切到受控 unhealthy 目标，确认 `failed_rolled_back/health_check_failed` 和旧版恢复；再原子替换为精确健康候选并升级。最终日志明确输出 `previous release Web upgrade, rollback, persistence and restart passed`，覆盖 SQLite integrity、初始化状态、Panel 哨兵、非目标游戏容器/volume 和 Panel restart。
- 候选 workflow 用时约 `9m6s`，全部步骤成功。唯一 GitHub annotation 是托管 action 在 Node 24 强制运行时提示其声明的 Node 20 已弃用；它没有改变构建、测试、候选对象或结论。兼容矩阵仅出现清单允许的可选镜像 mirror unavailable warning，必需制品与 source revision 均通过。

## 自动 Tag、正式提升与发布后独立复核

- 自动 Tag workflow `31823884131` 验证候选证明和仍为当前的 `origin/main` 后成功；`v0.4.17` 是 annotated tag（tag object `7c9ee9e1611cf692f0660908ae1eba3367d9b359`），解引用精确指向 `d63c93ffe7d65f8cdfcf2bedb9b336a6839be73f`。没有移动旧 tag，也没有从其它构建重新生成镜像。
- 正式提升 workflow `31823899038` 成功校验候选 digest/OCI 身份，以 preserve-digests 提升精确版本，再完成单仓正式镜像冒烟、三仓 `latest` 提升和 GitHub Release 创建。Docker Hub、阿里云 ACR、GHCR 的 `0.4.17` 与 `latest` 六个远端引用经独立 `docker buildx imagetools inspect` 复核，全部精确等于候选 digest `sha256:44c328cdf198ec888f3ec54bbe836ce114f5ac27c4ca5fb9cc63747a44083673`。
- 独立回拉 `ghcr.io/anxiyizhi/stardew-server-anxi-panel:0.4.17` 后，首次启动第 6 次有界轮询即同时达到 running、Docker health=`healthy`、`/health.status=ok`、database=`ok`；`/api/version` 返回 `0.4.17`、完整 commit 和固定 build date。受控 `docker restart` 后第 6 次轮询得到完全相同结果。
- GitHub Release `Stardew Server Anxi Panel 0.4.17` 于 `2026-08-14T17:26:48Z` 发布，为非 draft、非 prerelease，URL=`https://github.com/AnXiYiZhi/stardew-server-anxi-panel/releases/tag/v0.4.17`。正文已补齐 `/health` 验收、首次上传状态机和“社区中心收集包”三项用户可读说明及升级/digest 证据；资产 `run.sh`、`migrate-fnos.sh`、`repair-junimo-0.3.5.sh`、`repair-junimo-upgrade.sh` 均为 uploaded 状态并具有 GitHub SHA256 摘要。
- 本机发布后冒烟容器 `anxi-v0417-postrelease-smoke` 和 volume `anxi-v0417-postrelease-data` 在删除前完成 owner label 与 `/data` mount 复核，随后精确清理；owner=`v0417-postrelease` 的容器和 volume 均归零。候选证明临时目录只含已审计 `candidate.json`，验收后逐文件与空目录精确删除。未使用 `docker system prune`、`docker volume prune` 或模糊批量删除。

## 发布结论与后续边界

- `v0.4.17` 已完成自动候选、上一正式版真实 Web 升级、unhealthy 回滚、annotated tag、三仓 digest 提升、正式镜像版本/重启和 GitHub Release 门禁，没有兼容性或正式发布阻塞。证据回填属于 tag 后文档提交，不移动 `v0.4.17`，也不改变已发布候选 digest。
- steam-auth 兼容范围仍只包含已审计 `1.5.0-anxi.2`；增加新 tag 前必须先固定源 revision、真实 `/health` 契约与 digest，再更新双 allowlist 和测试。不得恢复 `/health -> /steam/ready` fallback，也不得把 `logged_in=false` 升级为运行组件发布阻塞。
- 首次上传离线集合继续严格限定为 `game_installed / save_required / ready_to_start / stopped`。任何状态扩展、cleanup 自动化或 journal schema 变化都必须重新覆盖 strict Compose 实停、上游提交边界、ownership/fingerprint 和精确状态恢复，不能用这次发布证据替代新变更的门禁。

# PLAYER-AUTH-MODES-1 发布记录（2026-08-15，released in v0.4.19，included in v0.5.0）

## 本版变更与受影响链路

- 新增 IP 直连的 `none / global / role` 玩家加入保护、角色 HMAC verifier、revision 原子配置、Control `TryAuthenticate` 输入补丁、运行时 revision/patch 状态和桌面/移动共享设置弹窗。
- 受影响链路为：Panel API → stardew_junimo driver → `.env` 原子替换 → Compose 环境 → Control 0.3.3 → Junimo `PasswordProtectionService.TryAuthenticate` → lobby/attempts/timeout/warp；以及运行状态反向从 `status.json` → driver → API → 前端。
- 运行栈从 `control-0.3.2` 升到 `control-0.3.3`，Control DLL SHA-256=`7b304fc8c8e5913ba11d3081f48ba06b2cb38b35a125c705e2a09ac22132ab1e`。server、steam-auth、game、SDK 和 SMAPI 版本/digest 未变化。任何正式候选都必须把 Control-only required runtime update、停服/备份/重启/实载版本与回滚纳入真实 Docker E2E。

## 本版专项矩阵

| 类别 | 必测场景 | 放行证据 |
| --- | --- | --- |
| 正常路径 | none、global、两个不同角色各自登录、Panel 批准认证 | 双客户端游戏内结果、Junimo 认证计数、Control runtime revision/patch ready |
| 权限安全 | 普通用户不能读写；A 密码不能登录 B；旧 API 在 role 模式 409；API/日志/支持包/Docker 输出无 key/verifier/guard | HTTP 与脱敏断言、交叉登录失败、支持包扫描 |
| 关键边界 | 未配置全角色、角色不存在、超长密码、角色改名、存档切换 orphan、损坏 key/payload/guard | 稳定错误码、fail-closed、不改变当前运行配置 |
| 幂等与恢复 | revision 冲突；原子写入无临时残留；保存后未重启显示 pending；重启后 revision 一致；Control patch 失败 | 并发测试、磁盘检查、重启/故障注入 |
| 数据完整性 | 旧 `SERVER_PASSWORD` 自动迁移为 global；空密码为 none；角色密码不明文落盘；Panel SQLite/存档/非目标 volume 不变 | 升级前后 API、`.env` 脱敏投影、SQLite integrity 与资源快照 |
| 资源清理 | Control-only apply 成功/回滚的任务容器、network、bind、volume 精确归零 | owner label 和精确资源名清单 |

## 发布证据与后续边界

- 已完成：在干净 `HEAD` 验证卷只叠加本功能差异后，Linux `go test ./... -count=1`、`go vet ./...`、`go build ./...` 全部通过；C# 纯策略契约；`.NET 6 SDK + stardew_game-data` 标准 Control 实编译（0 errors，1 个既有 analyzer warning）；Node 22 Linux 全部 17 项前端状态/布局回归与 production build；1280×720、390×844 Browser 视觉 QA。
- `v0.4.19` 候选 `31892497427`、自动 Tag `31893049884` 与正式提升 `31893060495` 已完成 Control required-update、fresh/restart、上一正式版 Web unhealthy 回滚/healthy 升级、三仓统一 digest 与正式冒烟；v0.5.0 候选 `31899107629` 又从 v0.4.19 完成真实 Web 回滚/升级并保留该认证回归。自动策略、Control fixture 和升级证据不等同于两个真人客户端实际输入角色密码；该人工记录仍是后续涉及认证交互改动时必须补的验证边界。

# PLAYER-AUTH-SELF-ENROLL-1 发布前验证记录（2026-08-17，人工矩阵 passed）

## 变更清单与受影响链路

- 角色模式新增首次 `!login` 自助认领；verifier 从实例级 `.env` payload 演进为按 saveId 隔离的 Control 私有 store，并增加 initialized marker、Panel/Control 跨进程锁、原子权限写入、legacy 迁移和 store/`.env` 事务回滚。
- API/前端新增 credential waiting/configured/error 与 store 健康状态，移除“启用前所有角色必须由管理员预设”的旧门禁；Control 从 `0.3.3` 升到 `0.3.6`。PR 自带 DLL 与项目标准 `/game` 构建不一致，已用 0-error 标准构建产物同步，最终 DLL SHA-256=`e7f3744b647c2f658ac3ad60d1dc27d958d935c7946f134b35447ab6c79bb422`。
- 旧 Compose 在生命周期前补四个 SAP 环境变量；restart 改为 `up -d --no-deps --force-recreate server`。role 模式迁移失败继续阻止，none/global 的自定义 inline environment 保留并告警继续。前端 restart pending 不再被 stale running 提前清除。
- 受影响链路：Panel player-auth API → driver mutation owner → `.env`/role credential store → Compose server environment/recreate → Control RolePasswordPatch → Junimo TryAuthenticate；反向状态为 Control/store + players → driver DTO → 桌面/移动共用弹窗。server/steam-auth/game/SDK/SMAPI 镜像版本未改变。

## 本版专项矩阵

| 类别 | 必测场景 | 本次放行条件 |
| --- | --- | --- |
| 正常路径 | 空角色启用；waiting 首次认领；已有凭据正确登录；管理员代设/清除 | Go+C# 契约与真实 Control 标准编译通过；API 不回显敏感值 |
| 边界与安全 | 错误/串角色、无效输入、跨 saveId、orphan、Panel guard、marker/store/key/guard 损坏 | 全部 fail closed；错误与 waiting 可区分；verifier/key/guard 不进入日志/响应 |
| 并发与恢复 | Panel/Control 同时写、活动/stale lock、marker/store 崩溃窗口、store 成功但 `.env` 失败及回滚失败 | 单写入者、原子终态、稳定错误码、无静默重新认领 |
| Compose 兼容 | mapping/list 幂等迁移；inline role 阻止；inline none/global 继续；restart 只重建 server | 文件内容/权限保持，环境进入新 server，steam-auth 不重启 |
| 前端生命周期 | restart 请求前 state=running、任务 active/terminal、普通 start 快任务 | restart 观察任务前持续锁定；terminal 后解锁；start fallback 保留 |
| 数据完整性 | legacy `.env` verifier 迁入活动 save；其它 save 与存档/SQLite/非目标 volume 不变 | verifier 不丢失不串档；测试前后非目标资源一致 |
| 真人交互 | 两客户端各自首次设置、交叉失败、清除重认领、Panel 批准、重启保持 | 正式发布前必须补齐；本次自动 Docker 验证不替代该项 |
| 资源清理 | 所有任务容器、network、volume、bind 与临时镜像 | 仅按任务前缀/owner 精确清理，结束计数为 0 |

## 当前证据与明确边界

- 宿主定向验证通过：玩家认证/Compose/生命周期 Go 专项 `7.171s`；`test:lifecycle-action-state`、`test:responsive-layout` 与 production build。Windows Docker Compose integration `TestComposeRecreateServicesAppliesChangedEnvironmentWithoutRestartingDependency` `6.155s` 通过，证明 `.env` 从 none 改为 role 后只重建 server、运行时环境生效且 dependency 容器 ID 不变；测试项目容器/网络均归零。
- Docker Desktop 的 `golang:1.25-alpine` 使用任务专属 module/build cache：冷缓存首次访问官方 Go Proxy 遇到 TLS handshake timeout，保留进度并经两次上限内预热切到 `goproxy.cn` 后成功；随后 `go test -p 1 ./... -count=1` 全包、`go vet ./...`、`go build ./...` 均通过。同步最终 Control DLL/manifest 后又重跑 Stardew 全子包（57.033s + config 0.020s）、Web（32.118s）与全量 build 通过，没有降低断言。
- `.NET 6 SDK + stardew_game-data`：纯 Control 契约通过；标准 `dotnet build -c Release /p:GamePath=/game /p:EnableModDeploy=false` 为 0 errors、1 个既有 CS9057 analyzer/compiler warning。标准产物 204288 bytes，最终嵌入 DLL 与 runtime manifest SHA-256 均为 `e7f3744b647c2f658ac3ad60d1dc27d958d935c7946f134b35447ab6c79bb422`。
- Node 22 Alpine 使用完整仓库 bind 和独立 `node_modules/dist` volume，`npm ci`、production audit 0 vulnerability、新增脚本在内的全部 18 项 `test:*` 与 Vite production build（142 modules，2.22s）通过。`compatibility-matrix.yml` 已加入新脚本并通过 YAML parse；`run-release-gates.sh` 已加入新脚本并通过 Bash 5.2 `bash -n` 与 ShellCheck。
- 本地 `sap-player-auth:test-20260817` dev 镜像完成 Dockerfile 全阶段构建；全新数据卷首次启动和 `docker restart` 后均得到 `/health.status=ok`，`/api/version.version=dev-player-auth-20260817` 且 commit 为当前工作树标识。该本地引用未推送，冒烟后与任务容器、6 个测试 volume、Control 临时副本、18092 监听均精确清理；`sap.task=player-auth-20260817` 的容器/volume/image 最终计数全部为 0。
- 前序阶段按用户要求未创建/移动 `v*` tag、GitHub Release 或正式镜像；2026-08-17 用户确认人工矩阵通过并明确授权正式发布，后续仍只允许由不可变候选自动创建 annotated tag 和提升同一 digest。
- 用户确认两个真实客户端已完成：各自首次设置不同角色密码、重复正确登录、交叉密码失败、管理员清除后重新认领、Panel 批准、server recreate 与 Panel 重启后保持。该用户确认作为本次人工交互证据，自动夹具继续只证明代码/容器契约。

# NEW-GAME-FARM-CAVE-CHOICE-1 构建、候选与发布证据（2026-08-23，released in v0.5.12）

## 变更范围与专项矩阵

- 本次变更只涉及新建存档 payload/事务耐久校验、Panel 自带 Control Mod 和新建弹窗；没有修改 Junimo 上游、基础镜像、Compose 运行格式、数据库迁移或 updater。Control/stack 版本由 `0.3.6` 升为 `0.3.7`，最终嵌入 DLL SHA-256 为 `bf8ba2026e33f62007e3d1cfca59b055da94806cc17dc999d62a1c94b2e39423`，与 runtime manifest 一致。

| 范围 | 场景 | 结果 |
| --- | --- | --- |
| 正常路径 | 缺省原版、显式原版、果蝠、蘑菇 | 请求/配置/Control/落盘契约通过 |
| 边界与安全 | 未知值、事务/存档/玩家不匹配、Control 或 XML 不一致 | 创建前拒绝或 fail closed；已有存档不写入 |
| 幂等与恢复 | Junimo 已预置蘑菇、重复回读、两次连续创建 | 精确转换到目标；状态和磁盘证据一致 |
| 数据完整性 | 源游戏卷、旧主存档与 `SaveGameInfo` | 真实 E2E 前后不变 |
| 前端 | 默认态、切换、重复提交、桌面/移动布局 | 选中态唯一、字段保留、无横向溢出或 console error |
| 资源清理 | 任务容器、网络、volume、临时构建/预览夹具 | 按任务 owner/精确路径清理，不触碰生产资源 |

## 本地与正式证据

- Control 纯契约测试通过；在 `.NET 6 SDK` 中使用本机真实 Stardew 1.6.15 游戏程序集执行标准 `dotnet build -c Release /p:GamePath=/game /p:EnableModDeploy=false`，结果 0 error、1 个既有 CS9057 analyzer/compiler warning。最终 DLL、源码/嵌入 manifest 和 runtime stack manifest 已同步为 `0.3.7`。
- Docker Desktop 真实 `TestRealNewGameMaterializesSMAPIModsBeforeFirstSaveOptIn` 运行约 153 秒并通过：从只读源游戏 volume 克隆到任务 volume，第一轮创建果蝠、第二轮创建蘑菇；两轮均验证 Control transaction/snapshot 与主存档 XML，源游戏卷及旧存档哈希不变，Compose 夹具终态清理成功。
- `golang:1.25-alpine` 以完整仓库只读 bind 和任务专属 module/build cache 执行 `go test ./internal/games/stardew_junimo/... -count=1`，包测试 58.081 秒、config 0.026 秒通过。前端 `test:new-game-idempotency` 与 Vite production build 通过；应用内 Browser 在 1280×720 和 390×844 完成默认/切换、命名容器响应式、overflow 与 console QA。
- 正式候选 workflow `32623320406`（attempt 1）固定版本 `0.5.12`、上一正式版 `0.5.11`、commit `5141cd54dca1752419a9d738f873623a4871f884` 与 UTC build date `2026-08-23T06:35:45Z`，约 12 分 15 秒完成。路径矩阵因 runtime manifest、Junimo runtime 与公开 docs 均变化，选择了远程运行栈制品核验、SMAPI 真实下载、Junimo 真实 network/runtime integration 和网站 production build；本次三个条件长门禁均未按路径跳过。常驻门禁同时完成兼容契约、部署脚本 Bash/ShellCheck、后端全量 test/vet/build、updater/Docker integration、全部前端状态回归与 production build。
- 同一候选执行 fresh install、`/health`、`/api/version`、未初始化态、Panel restart，并从真实 `v0.5.11` 通过 Web API 完成检查、dry-run、apply、断线重连和 `v0.5.12` 终态；unhealthy 目标得到失败回滚和旧版恢复。SQLite integrity、初始化状态、长期 sentinel、非目标游戏容器/volume、Panel 重启、Mod 检查、legacy runtime repair 与存档导入边界均保持。候选封存为 `ghcr.io/anxiyizhi/stardew-server-anxi-panel:candidate-0.5.12-5141cd54dca1@sha256:faf910075f4b25a3172fe4ee53341cf53b9c3c26c1065ce38b65c19fcc9af5a0`，不可变 proof artifact 为 `release-candidate-0.5.12-5141cd54dca1`；独立 Compatibility workflow `32623320473` 同样成功。
- 自动 tag workflow `32623853636` 只在候选 commit 仍精确等于 `origin/main` 后创建 annotated `v0.5.12`；tag type 为 `tag`，peeled commit 为 `5141cd54dca1752419a9d738f873623a4871f884`。正式提升 `32623863894` 约 1 分 10 秒完成且未重新 build：Docker Hub、阿里云 ACR、GHCR 的 `0.5.12` 与 `latest` 六个引用经独立 `buildx imagetools inspect` 复核，全部等于候选唯一 digest。回拉一个正式 GHCR 版本的 health/version 冒烟通过，GitHub Release `v0.5.12` 为非 draft、非 prerelease 且成为 latest。
- 本轮候选、Compatibility、自动 tag 与正式提升均一次成功，没有失败步骤或重跑。候选使用 GitHub-hosted 临时 runner 和脚本 EXIT trap 回收 fresh/DinD 容器、网络、volume 与临时文件；本机 Docker `sap-farm-cave` owner 的容器/网络/volume、4179 监听、Control 构建目录和失败夹具均为 0，下载的候选 proof 临时目录也已按精确路径删除。没有移动既有 tag，也没有在正式提升中重建镜像。
