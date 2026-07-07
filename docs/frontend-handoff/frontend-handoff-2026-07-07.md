# FE-MOD-BATCH-ERROR-FOCUS-1 Nexus 批量安装失败定位

## 背景
- 扩展批量安装主 Mod + 前置 Mod 时，旧前端只要任一关联后端 job 进入 failed/canceled，就把整个搜索卡片按钮显示成失败；按钮本身还是 disabled，用户无法从按钮直接跳到出错任务和日志。
- 当后端旧逻辑因为重复 `UniqueID` 把已安装 Mod 标成 failed 时，前端也没有用最新 `GET /mods` 结果兜底校正，导致“东西已经装好了”却显示整批失败。

## 改了什么
- `NexusExtensionInstallState` 新增 `errorItemName` 和 `errorJobId`。后端 job 失败时，按钮文案显示具体 Mod 名，例如 `SpaceCore 失败`；如果有 `jobId`，按钮保持可点击，点击跳转 `/instances/stardew/jobs?jobId=<jobId>`。
- 批量协调器 `markInstalledBatchItems()` 现在也会校正带 `jobId` 的条目：只要最新 Mod 列表能通过 `nexusModId` 或 `originNexusModId` 命中，该项即视为 done，不再因旧 failed job 误伤整批状态。
- 无 `jobId` 的扩展捕获/提交失败仍按旧逻辑显示错误和“重置状态”，不会伪造任务日志入口。

## 影响文件
- `frontend/src/games/stardew/pages/ModsPage.tsx`

## 如何验证
- `cd frontend; npm.cmd run build`
- 联调时可复用旧失败 job 场景：若对应 Mod 已能在 `GET /mods` 中匹配，搜索卡片批量状态应恢复完成；若是真实后端 job 失败，按钮显示具体 Mod 名，点击能进入任务日志页并选中该 job。

## 下一步注意事项
- 失败按钮的可点击条件依赖 `errorJobId`；扩展自身未拿到 ZIP、未创建 job 的失败不要跳任务页。
- 后续如果任务页路由支持更正式的 query navigate，可把当前 `history.pushState + popstate` 小跳转收敛到公共导航工具里。

# NEXUS-MODPAGE-DL-2 浏览器扩展适配 Nexus Shadow DOM 下载入口（0.1.1 → 0.1.2）

## 背景
- `NEXUS-MODPAGE-DL-1`（0.1.1，见 [[nexus-ext-version-cache-1]] 所在的 `backend-handoff-2026-07-07.md`）按可见按钮文案匹配下载控件；后续发现 Nexus 部分改版 Mod 页把下载控件（Vortex / Manual 按钮、文件链接）渲染进 Web Component（shadow root），纯 `document.querySelectorAll` 看不到这些节点，文案匹配偶发失效或撞上无关按钮。

## 改了什么
- `browser-extensions/nexus-slow-installer/content.js`：
  - 新增 `deepQueryAll(selector)`：递归遍历 `document` 及每个已打开的 shadow root 做选择器匹配。`findFileIdOnPage()` 和新的 `findManualDownloadControl()` 都改用它，而不是原生 `querySelectorAll`。
  - 新增 `controlLabel(node)`：合并可见文本与 `data-tracking`/`aria-label`/`title` 属性，供分类使用。
  - 新增 `findManualDownloadControl()`：优先按 Nexus 自带的 `data-tracking*="Download" i` 属性筛出下载控件，用 `manual` 关键字命中、排除 `vortex`/`mod manager`；按控件是否已带 `file_id`（href 可解析）排序，优先选真实文件链接而不是弹出模态的头部按钮；找不到才回退到旧的文案匹配（`isManualDownloadLabel` / `isShortManualLabel`）。
  - 新增 `openNexusFileList()` + `waitForFileIdOnPage(timeoutMs)`：`file_id` 未就绪时，先尝试点击 `findManualDownloadControl()` 打开文件列表/模态，否则在非 files tab 时跳转到 `?tab=files`；然后用 `setInterval` + `MutationObserver` 双重轮询等待 `findFileIdOnPage()` 返回非零值，最长等 20 秒。
  - `clickManualDownloadWhenReady` 的两步点击模型（短 `Manual` 开模态 → 模态内 `Manual download`）合并为统一逻辑：`findManualDownloadControl()` 返回的控件若 href 已带 `file_id` 直接当真实下载链接跳转/点击；否则当作列表/模态开关，节流点击后继续观察。
  - 移除不再使用的 `currentAnxiParams()`（旧 `TRIGGER_NEXUS_MANUAL_DOWNLOAD` 消息路径的参数收集，随两步点击模型一起被新逻辑取代）。
- 版本同步：`manifest.json`（`0.1.1 → 0.1.2`）、`background.js` 与 `panel-bridge.js` 的 `X-Anxi-Nexus-Installer` 请求头同步到 `0.1.2`。
- 文档：`browser-extensions/nexus-slow-installer/README.md` 新增 `NEXUS-MODPAGE-DL-2` 小节；`docs/03-frontend.md` 追加同名条目。

## 影响文件
- `browser-extensions/nexus-slow-installer/content.js`
- `browser-extensions/nexus-slow-installer/manifest.json`
- `browser-extensions/nexus-slow-installer/background.js`
- `browser-extensions/nexus-slow-installer/panel-bridge.js`
- `browser-extensions/nexus-slow-installer/README.md`
- `docs/03-frontend.md`

## 如何验证
- `node --check browser-extensions/nexus-slow-installer/content.js background.js panel-bridge.js`（语法检查，已过）。
- `cd backend; go test ./internal/games/stardew_junimo -run TestEnsureNexusInstallerExtensionZip`（确认版本感知缓存逻辑在 0.1.2 下仍能判定旧 0.1.1 缓存 ZIP 过期并重新打包，已过）。
- 手动验证：在 Chrome/Edge 重新加载已解压扩展，打开一个渲染进 shadow root 的改版 Nexus Mod 页，确认能直接识别 `file_id` 或正确点开文件列表/模态，而不是报"未找到下载入口"。

## 下一步注意事项
- 本次未改动后端接口和面板前端 React 代码，纯浏览器扩展内部适配；发布新镜像后旧实例缓存 ZIP 会被后端版本感知逻辑自动刷新，无需手动清理。
- 如果 Nexus 后续继续改版页面结构，优先扩展 `data-tracking` 关键字匹配和 shadow root 遍历深度，避免退回纯文案匹配（改版后文案本身也不稳定）。
