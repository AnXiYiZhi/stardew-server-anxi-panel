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
