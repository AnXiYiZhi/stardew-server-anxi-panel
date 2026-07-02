# PERF-REVIEW-1 前端接手记录

## 改了什么
- `ModsPage` 把已安装 Mod 的排序、Nexus 展示列表、本地隐藏列表、解析错误数、同步分类统计和可打包数量合并进一个 `useMemo`。
- 删除不再需要的 `syncableMods` 中间数组，减少页面频繁 state 更新时的重复 `filter` 和临时数组分配。

## 影响文件/接口
- `frontend/src/games/stardew/pages/ModsPage.tsx`
- 接口不变；只是渲染侧派生数据计算方式调整。

## 如何验证
- 已执行：`cd frontend; npm.cmd run build`。

## 下一步注意事项
- 后续继续拆分 `ModsPage` 时，可以把 Nexus 搜索卡片、已安装卡片和配置列表拆成 memoized 子组件；本次先保持单文件结构，避免和当前扩展安装功能线产生大范围冲突。

# NEXUS-EXT-3 前端交接

## 改了什么

- `ModsPage` 的 Nexus 搜索结果“一键安装”不再直接调用 `POST /mods/nexus/install`，改为同页跳转到 Nexus 对应 Mod 的 `?tab=files&anxi_auto=1` 文件页，由浏览器扩展接手获取临时 ZIP 链接。
- 浏览器扩展会在 Nexus 文件页自动打开手动下载/慢速下载流程，拿到 `supporter-files.nexus-cdn.com/*.zip` 临时链接后只在右下角显示一个“提交到面板”按钮。用户点击提交后，扩展调用现有 `POST /api/instances/:id/mods/remote/install`，并立刻跳回面板任务页。
- 任务页支持 `?jobId=<id>`，扩展回跳到 `/instances/:id/jobs?jobId=...` 后会直接选中新创建的安装任务并打开 SSE 日志。
- `ModInstallMethod` 新增 `nexus_extension`，用于标记当前搜索卡片的安装方式已经切到扩展链路。

## 影响文件

- `frontend/src/games/stardew/pages/ModsPage.tsx`
- `frontend/src/games/stardew/pages/JobsLogsPage.tsx`
- `frontend/src/types.ts`
- `browser-extensions/nexus-slow-installer/content.js`
- `browser-extensions/nexus-slow-installer/background.js`
- `browser-extensions/nexus-slow-installer/shared.js`

## 如何验证

- 已执行：`cd frontend; npm.cmd run build`
- 已执行：`node --check browser-extensions/nexus-slow-installer/content.js`
- 已执行：`node --check browser-extensions/nexus-slow-installer/background.js`
- 已执行：`node --check browser-extensions/nexus-slow-installer/shared.js`
- 手动联调建议：面板停服、同浏览器登录面板管理员和 Nexus，点击搜索结果“一键安装”，确认跳到 Nexus 文件页后扩展自动捕获 ZIP；点击右下角提交按钮后应回到任务日志页并选中刚创建的 `mod_remote_install` 任务。

## 下一步注意

- 这个链路依赖浏览器扩展复用同浏览器里的面板登录 Cookie。正式云端部署若出现 401/403，应优先做扩展专用配对 token，而不是把管理员密码或完整 Nexus 链接长期保存到扩展状态。
- 扩展拿到的完整 Nexus CDN 临时 URL 只应作为请求体短暂提交，状态、日志、文档和支持包里只能保留脱敏 URL。
- Nexus 多文件 Mod 目前仍由扩展自动点击页面上可见的第一个手动下载入口；后续如果要精确文件选择，需要在面板或扩展里增加文件列表/版本选择 UI。
# NEXUS-REQ-1 前端接手

## 改了什么
- `NexusModSearchResult` 增加 `requiredMods[]` 类型字段；`ModsPage` 在 Nexus 搜索结果卡片 footer 显示前置状态。
- 缺失前置会显示“安装前置”按钮，点击后用该前置的 Nexus modId 生成 `?tab=files&anxi_auto=1` 链接，复用浏览器扩展安装链路。
- `browser-extensions/nexus-slow-installer/content.js` 会在捕获流程开始后检测 Nexus “Additional files required” 弹窗，并自动点击弹窗里的 `Download` 按钮。

## 影响文件/接口
- `frontend/src/types.ts`
- `frontend/src/games/stardew/pages/ModsPage.tsx`
- `frontend/src/games/stardew/StardewPanel.css`
- `browser-extensions/nexus-slow-installer/content.js`
- 依赖后端 `GET /api/instances/:id/mods/nexus/search` 返回 `results[].requiredMods[]`。

## 如何验证
- `cd frontend; npm.cmd run build`
- `node --check browser-extensions/nexus-slow-installer/content.js`
- `node --check browser-extensions/nexus-slow-installer/background.js`
- `node --check browser-extensions/nexus-slow-installer/shared.js`

## 下一步注意事项
- `requiredMods[]` 是 Nexus 页面声明的前置，不等同于 ZIP 内 manifest 的 SMAPI `Dependencies`。安装后仍要看已安装卡片的 `dependencies[]` 状态。
- SMAPI 这类运行时依赖通常由虚拟内置条目表达；不要把它当作普通服务器 Mod 文件夹处理。
# NEXUS-PREMIUM-2 前端接手

## 改了什么
- 删除 `ModsPage` 下载页的“粘贴链接安装”人工入口，包括按钮、弹窗、相关 state、`installRemoteMod()` 前端 API 封装和 `RemoteModInstallRequest` 类型。
- Nexus Key 未配置时，在“配置 Nexus Key”按钮左侧显示 `如果您是尊贵的 Nexus Premium 用户，请填您的 NexusKey`；Key 已配置后该提示消失，只显示已配置状态。
- Key 已配置时，Nexus 搜索结果每个卡片底部新增 `N站会员专属安装` 按钮，复用现有 `installNexusMod()` 直连安装任务。普通 `一键安装` 继续走扩展跳转流程，服务于非 Premium 用户。

## 影响文件
- `frontend/src/games/stardew/pages/ModsPage.tsx`
- `frontend/src/games/stardew/StardewPanel.css`
- `frontend/src/api.ts`
- `frontend/src/types.ts`

## 如何验证
- `cd frontend; npm.cmd run build`
- 手动联调建议：未配置 Nexus Key 时确认只显示提示和“配置 Nexus Key”，搜索卡片不显示会员按钮；配置 Key 后提示消失，每个搜索结果卡片底部出现 `N站会员专属安装`，普通 `一键安装` 仍跳转 Nexus 扩展页面。

## 下一步注意事项
- `POST /api/instances/:id/mods/remote/install` 仍由浏览器扩展提交临时 ZIP 使用，前端页面不再提供手动粘贴入口。
- 会员按钮要求管理员、服务器停止、目标 Mod 未安装且当前无安装任务；失败时仍沿用任务日志和安装错误面板展示。
# NEXUS-CARD-UI-1 前端接手

## 改了什么
- 优化 `ModsPage` 的 Nexus 搜索结果卡片布局：卡片改成内容区、主操作区、次操作区，`N站页面/一键安装` 这类统一主按钮固定在同一行，减少不同卡片按钮上下漂移。
- `N站会员专属安装` 放入底部次操作区，不再挤在主按钮行里；前置依赖状态也放在该区域。
- 前置依赖展示从“逐个标签 + 安装前置按钮”改成一个状态入口，仅显示 `缺少前置mod` 或 `前置已满足`。点击或悬停会展开详情，包含前置 Mod 名、NexusId 和当前状态。

## 影响文件
- `frontend/src/games/stardew/pages/ModsPage.tsx`
- `frontend/src/games/stardew/StardewPanel.css`

## 如何验证
- `cd frontend; npm.cmd run build`
- 手动联调建议：登录面板后进入 `模组 -> 下载模组`，搜索带前置依赖的 Nexus Mod，确认卡片主按钮对齐、底部会员按钮不挤占主操作、依赖状态可悬停/点击展开详情。

## 下一步注意事项
- 搜索结果卡片不再直接提供“安装前置”小按钮；如果后续要恢复依赖一键安装，建议放到展开详情内或单独依赖管理弹窗，避免重新撑乱卡片布局。
- 本次没有改后端 `requiredMods[]` 数据结构，前端仍依赖 `name/modId/installed/installedEnabled/installedVersion` 字段渲染详情。

# NEXUS-EXT-BATCH-1 前端接手

## 改了什么
- 普通 `一键安装` 不再让面板页跳转 Nexus。`ModsPage` 通过浏览器扩展 `panel-bridge.js` 发送 `START_BATCH_INSTALL`，扩展后台用非激活标签页同时打开当前 Mod 和未安装 Nexus 前置 Mod 的下载页。
- Nexus content script 读取 `anxi_batch/anxi_item/anxi_auto_submit` 参数，捕获 ZIP 链接后自动提交到面板；批量模式提交成功后关闭后台标签页，不跳任务日志页。
- 搜索卡片的一键安装按钮变成百分比进度条。扩展获取/提交阶段按单项 `opening=10 / capturing=35 / ready=65 / posting=80 / queued=90` 计算，多项取平均；拿到面板 `jobId` 后继续轮询 `GET /api/jobs/:id`，所有 job `succeeded` 才 100%，任一 job `failed/canceled` 会显示失败和对应 Mod 名。扩展无响应、后台页超时或任一项提交失败时显示 `失败请手动安装`。
- 无 `jobId` 的 item 会刷新本地 Mod 列表按 `nexusModId/originNexusModId` 兜底匹配，命中则视为完成，解决 Custom Companions 这类实际已安装但扩展状态卡住的问题。
- 根因修复：content 在 `CAPTURE_URL` / `SUBMIT_CAPTURED_URL` 中显式传 `batchId/itemId/autoSubmit`，background 会从这些字段或 `captureKey=batch:item` 恢复 capture 的 batch 上下文，避免后端创建了任务但 batch item 没有 `jobId`。
- Nexus 文件列表页的 `Manual download` 和前置确认弹窗的 `Download` 不再只依赖 debugger 坐标点击；content script 会优先读取按钮 `href` 并直接跳转，同时把当前 `anxi_batch/anxi_item/anxi_auto_submit` 参数带到下一页。Manual 若是 JS 按钮，会通过 background 在页面主世界触发 `button.click()`，避免后台标签页卡在“正在进入下载页”。
- 批量模式自动提交按来源分流：content 直接生成 ZIP 链接时，`CAPTURE_URL` message 内继续推进；Chrome `downloads.onCreated` 捕获 ZIP 时，background 只保存链接并通知 Nexus 页，content 再发 `SUBMIT_CAPTURED_URL` 提交，避免 MV3 service worker 在下载事件里长 fetch 导致卡在 `posting`。Nexus content script 会把批量安装上下文写入 `sessionStorage`，跳转丢失 URL 参数时仍可识别自动提交，收到 ZIP ready 后直接调用原提交按钮逻辑。批量任务提交面板时优先通过已登录面板标签页里的 `panel-bridge.js` 发同源 `POST /api/instances/:id/mods/remote/install`，复用面板 Cookie/Vite proxy；面板桥接不可达才回退 background 直连。面板提交请求有 30 秒超时，失败会回写 batch 状态。

## 影响文件
- `frontend/src/games/stardew/pages/ModsPage.tsx`
- `frontend/src/games/stardew/StardewPanel.css`
- `browser-extensions/nexus-slow-installer/background.js`
- `browser-extensions/nexus-slow-installer/content.js`
- `browser-extensions/nexus-slow-installer/panel-bridge.js`
- `browser-extensions/nexus-slow-installer/manifest.json`

## 如何验证
- 已执行：`cd frontend; npm.cmd run build`
- 已执行：`node --check browser-extensions/nexus-slow-installer/background.js`
- 已执行：`node --check browser-extensions/nexus-slow-installer/content.js`
- 已执行：`node --check browser-extensions/nexus-slow-installer/shared.js`
- 已执行：`node --check browser-extensions/nexus-slow-installer/panel-bridge.js`

## 下一步注意事项
- 扩展加载或更新后，需要刷新面板页，让 `panel-bridge.js` 注入当前面板页面。
- 如果按钮提示“浏览器扩展未响应”，优先确认扩展管理页已重新加载扩展并刷新面板页；如果提示 `panel origin mismatch`，需要把扩展弹窗里的面板地址改成当前面板访问地址，或在本地开发时使用 `localhost/127.0.0.1` 回环地址。
- Nexus 搜索词、搜索结果、分页和当前扩展 batch 状态都保存在 `sessionStorage`。切到任务日志再回到模组页时不应重新加载默认热门，也不应丢失按钮进度；恢复后会继续 `GET_BATCH_STATUS` 轮询扩展。
- 前端按钮百分比现在会继续追踪面板 `mod_remote_install` job：job 创建只到 90%，后端实际安装成功才 100%；后端失败会反映到按钮失败态，任务日志仍是排查细节的来源。
- 如果没有 job 记录但本地 Mod 已安装，前端以本地 `GET /mods` 结果为准收敛进度；任务日志缺失时仍能让按钮结束。
- 新流程应优先拿到 jobId；本地安装兜底只是消息丢失时的防卡死保护，不应作为常规路径。
- 卡住恢复入口：有扩展安装状态的搜索卡片会显示 `重置状态`。它会清前端 session、停止轮询，并让 `panel-bridge.js` 调用 background `CLEAR_STATE` 清扩展存储；前后端重启不会清掉这些浏览器状态。
- 当前只自动下载 `required.installed=false` 的 Nexus 前置。已安装但当前存档未启用的前置由配置模组页启用逻辑处理，不重复下载安装到服务器。
# NEXUS-EXT-BATCH-2 前端接手

## 改了什么
- `ModsPage` 修复扩展批量安装达到 100% 后仍显示安装中的问题：`done/failed` 现在是终态，旧的 `GET_BATCH_STATUS` running 响应不会覆盖终态。
- 安装完成或本地兜底命中后，会用最新 `GET /mods` 结果同步当前 Nexus 搜索结果和 `requiredMods[]` 的已安装字段，避免切到任务日志再回来后按钮又变回“一键安装”。

## 影响文件
- `frontend/src/games/stardew/pages/ModsPage.tsx`

## 如何验证
- `cd frontend; npm.cmd run build`
- `node --check browser-extensions/nexus-slow-installer/background.js`
- `node --check browser-extensions/nexus-slow-installer/content.js`
- `node --check browser-extensions/nexus-slow-installer/shared.js`
- `node --check browser-extensions/nexus-slow-installer/panel-bridge.js`

## 下一步注意事项
- 扩展 batch 的 job 创建仍只代表进入后端任务队列；按钮必须继续以 `GET /api/jobs/:id` 的最终 `succeeded/failed/canceled` 为准。
- 如果用户已经手动清理或重装 Mod，搜索结果缓存只会在下一次 `GET /mods` 同步后更新；不要只信任扩展 batch 的旧 storage。
# NEXUS-EXT-BATCH-3 前端接手

## 改了什么
- `browser-extensions/nexus-slow-installer/background.js` 的 `START_BATCH_INSTALL` 入口增加 `uniqueBatchTargets()`，先按 Nexus `modId` 去重，缺少 `modId` 时按清理批量参数后的 URL 去重。
- 同一个目标既以 `required` 又以 `target` 身份进入批量队列时，优先保留 `target`，避免当前 Mod 本体被打开两次。
- 同一个 `batchId` 重复发起时直接返回已有 batch；如果面板页刷新导致 tabId 变化，会更新 `panelTabId`，但不会再开 Nexus 标签页。

## 影响文件
- `browser-extensions/nexus-slow-installer/background.js`

## 如何验证
- 已执行：`node --check browser-extensions/nexus-slow-installer/background.js`
- 手动验证建议：点击 Ridgeside Village 这类本体加多个前置的普通“一键安装”，确认每个 Nexus modId 只出现一个后台下载页，本体页不会重复打开。

## 下一步注意事项
- 如果以后前端 targets 里出现缺失 `modId` 的项目，扩展会退回按 URL 去重；请尽量继续保证 `requiredMods[]` 和当前本体都带真实 Nexus modId。
- `batchId` 幂等只阻止同一批次重复开页；用户重新点击产生新 batchId 时仍会开始新的安装批次。
# NEXUS-EXT-CONNECT-1 前端接手

## 改了什么
- `ModsPage` 在管理员下载页新增扩展连通状态，检测按钮放在“配置 Nexus Key”旁边。点击后会向浏览器扩展发送 `PING`，成功时显示“扩展已连通”，并把普通 Nexus “一键安装”从灰色禁用切回可用。
- `panel-bridge.js` 对 `PING` 做一次特殊放行：即使扩展里仍保存旧面板地址，也允许当前已登录面板页先完成注册。注册前会调用 `GET /api/auth/me`，确认当前页面是面板且用户已登录，再把 `window.location.origin` 和实例 ID `stardew` 写入扩展 background 配置。
- `background.js` 新增 `REGISTER_PANEL` 和 `PING` runtime message；其它安装消息仍维持 origin 校验，不会因为自动注册而放开普通网页调用权限。
- 普通“一键安装”现在依赖扩展连通状态；未连通时禁用并在 tooltip 提示先检测扩展。`N站会员专属安装` 仍只依赖 Nexus Key 配置。
- 检测按钮右侧会直接显示检测中、已连通或未响应错误信息；不要只依赖按钮 title，否则扩展未注入时用户会感觉点击没有反应。
- 连通检测不再只相信扩展回包成功；前端会校验 `config.panelBaseUrl` 的 origin 必须等于当前 `window.location.origin`。如果用户换端口而扩展仍保存旧地址，页面会显示旧地址错误，不会提前标成“已连通”。

## 影响文件
- `frontend/src/games/stardew/pages/ModsPage.tsx`
- `frontend/src/games/stardew/StardewPanel.css`
- `browser-extensions/nexus-slow-installer/background.js`
- `browser-extensions/nexus-slow-installer/panel-bridge.js`

## 如何验证
- `node --check browser-extensions/nexus-slow-installer/background.js`
- `node --check browser-extensions/nexus-slow-installer/content.js`
- `node --check browser-extensions/nexus-slow-installer/shared.js`
- `node --check browser-extensions/nexus-slow-installer/panel-bridge.js`
- `cd frontend; npm.cmd run build`
- 手动验证：重新加载浏览器扩展，刷新面板页，点击“检测扩展”。成功后按钮显示“扩展已连通”，普通“一键安装”按钮从禁用变为可点；换面板端口或 IP 后再次点击检测应自动更新扩展里的面板地址。

## 下一步注意
- 如果用户看到“浏览器扩展未响应”，优先确认扩展已重新加载且面板页已刷新。
- 如果 `PING` 失败但面板已登录，检查 `panel-bridge.js` 是否被 manifest 注入当前面板 origin，以及 `/api/auth/me` 是否在当前部署路径下可访问。
# NEXUS-EXT-PACK-1 前端接手记录

## 改了什么
- `ModsPage` 下载页在 `配置 Nexus Key` 右侧新增普通 Nexus 用户扩展安装引导文案。
- 新增 `下载浏览器扩展` 按钮，调用 `downloadNexusInstallerExtension()` 下载后端生成的 `anxi-nexus-installer.zip`。
- 按钮下载中显示 `打包中...`；失败时把错误写入现有 Nexus 安装错误区域。

## 影响文件/接口
- `frontend/src/api.ts`
- `frontend/src/games/stardew/pages/ModsPage.tsx`
- `frontend/src/games/stardew/StardewPanel.css`
- 依赖接口：`GET /api/instances/:id/mods/nexus/extension/download`

## 如何验证
- `cd frontend; npm.cmd run build`
- 手动验证：进入 Mods 下载页，点击 `下载浏览器扩展`，浏览器应下载 `anxi-nexus-installer.zip`；解压后目录根部应能看到 `manifest.json`。

## 下一步注意事项
- 下载扩展只解决安装包获取；用户仍需要在 Chrome/Edge 扩展管理页加载解压目录，然后回面板点击 `检测扩展` 同步当前面板地址。
# NEWGAME-PLAYERLIMIT-1 前端接手

## 改了什么

- `NewGameCreator` 在左侧联机设置中新增“联机人数上限”，默认 `10人`，范围 `1-100`，提交字段为 `maxPlayers`。
- “初始联机小屋”仍保持真实小屋数 `0-7`，不再承担总人数上限语义；增加小屋时会自动把人数上限至少推到 `startingCabins + 1`。

## 影响文件/接口

- `frontend/src/games/stardew/NewGameCreator.tsx`
- `frontend/src/types.ts`
- 依赖后端 `POST /api/instances/:id/saves/custom-new-game` 接收 `maxPlayers`。

## 如何验证

- 已执行：`cd frontend; npm.cmd run build`
- 已执行：`cd backend; go test ./internal/games/stardew_junimo -run "WriteServerSettings|ValidateNewGameConfig"`

## 下一步注意事项

- 不要把 `startingCabins` 范围直接放到 100；超过原生 7 个初始小屋的玩家容量应走 Junimo `Server.MaxPlayers` 和 `CabinStack` 自动小屋管理。
# VNC-CONTROL-1 前端接手记录

## 改了什么
- `ServerControlPage` 的“快捷操作”新增 VNC 显示切换按钮；`跳转VNC控制` 默认隐藏，仅在显示渲染打开后出现。
- 服务器运行时会先调用 `GET /api/instances/:id/rendering` 读取当前 Junimo 渲染 FPS，避免刷新页面后把已开启的 VNC 显示误显示成 `打开VNC显示`。
- `打开VNC显示` 调用 `POST /api/instances/:id/rendering`，当前固定传 `fps=15`；成功后按钮显示为 `关闭VNC显示`，再次点击传 `fps=0` 关闭服务端渲染。
- `跳转VNC控制` 读取 `GET /api/instances/:id/config/vnc-port` 返回的 `vncPort`，打开 `http://<当前hostname>:<vncPort>/`；关闭 VNC 显示后该按钮会重新隐藏。

## 影响文件/接口
- `frontend/src/games/stardew/pages/ServerControlPage.tsx`
- `frontend/src/api.ts`
- `frontend/src/types.ts`
- 新接口：`GET/POST /api/instances/:id/rendering`
- 复用接口：`GET /api/instances/:id/config/vnc-port`

## 如何验证
- `cd frontend; npm.cmd run build`
- 手动联调建议：服务器运行后先点 `打开VNC显示`，确认按钮变为 `关闭VNC显示`，再点 `跳转VNC控制`，确认 noVNC 页面能打开并用安装时的 VNC 密码进入控制画面；最后点 `关闭VNC显示` 验证可关闭渲染。

## 下一步注意事项
- VNC 控制页是宿主端口页面，不在 React 内嵌 iframe；如果部署在反向代理/HTTPS 后面，可能需要额外代理 noVNC 端口，否则当前按钮仍会按面板 hostname + 自定义 VNC 端口直连。
