# Conversation Handoff 2026-06-30

## MVP-UX-3: 无存档引导兼容 stopped + saves=0 状态

### 目标

用户截图反馈：页面顶部状态是“已停止”，存档指标卡显示 `0 / 暂无激活存档`，但“启动”按钮旁仍没有出现“当前没有存档，请点击此按钮去创建/上传存档。”和“创建/上传存档”按钮。上一轮只监听 `save_required` 状态或启动请求返回 `save_required`，没有覆盖当前截图里的 `stopped + saves.length === 0`。

### 改了什么

| 文件/范围 | 修改 |
|------|------|
| `frontend/src/games/stardew/pages/OverviewPage.tsx` | 新增 `noSavesDetected`，当 `dashboardData.saves` 已加载且 `saves.length === 0` 时也显示无存档引导 |
| `frontend/src/games/stardew/pages/OverviewPage.tsx` | 引导显示条件现在是 `save_required` / 本次启动检测到 `save_required` / 存档列表为 0，且状态不是 `running` 或 `starting` |
| `frontend/src/games/stardew/pages/ServerControlPage.tsx` | 同步使用 `dashboardData.saves.saves.length === 0` 触发无存档引导，覆盖服务器控制页 |

### 影响接口/文件

- 后端 API 未改。
- 存档列表接口读取失败时不会显示该引导，避免误把读取失败当成无存档。
- 截图中的 `stopped + saves=0` 状态现在会直接显示提示和按钮，不需要先点击启动。

### 如何验证

```powershell
cd E:\stardew-server-anxi-panel\frontend
npm.cmd run build
# exit 0，39 modules，CSS 89.96 kB，JS 327.17 kB
```

建议真实页面复测：

1. 进入截图同样状态：顶部“已停止”，存档卡 `0 / 暂无激活存档`。
2. 总览页“启动”按钮旁应出现无存档提示和“创建/上传存档”按钮。
3. 点击按钮进入存档页。
4. 上传或创建存档后，回到总览页，该提示应消失。

### 下一步注意

- 如果后续后端能够在 reconcile 时把 `stopped + saves=0` 自动收敛成 `save_required`，这段前端兼容仍然安全，因为两个条件等价显示。
- 若未来支持非管理员账号，可能需要给非管理员点击“创建/上传存档”后增加权限提示。

---

## MVP-UX-2: 无存档启动时在启动按钮旁显示创建/上传引导

### 目标

用户继续做 MVP 用户向验证后调整期望：没有存档时点击“启动服务器”，不应只有弹提示，也不应直接替用户跳进创建存档弹窗；应在启动服务器按钮旁边出现一个去创建/上传存档的按钮，并用文字提示“当前没有存档，请点击此按钮去创建/上传存档”。该文字和按钮只在服务器检测没有存档时出现。

### 改了什么

| 文件/范围 | 修改 |
|------|------|
| `frontend/src/games/stardew/pages/OverviewPage.tsx` | 新增 `saveRequiredDetected` 本地状态；当实例状态是 `save_required` 或本次启动请求收到 `save_required` 时，在启动按钮旁显示无存档引导 |
| `frontend/src/games/stardew/pages/OverviewPage.tsx` | `save_required` 状态下保留禁用的“启动”按钮，不再把生命周期主按钮直接替换成“创建存档” |
| `frontend/src/games/stardew/pages/ServerControlPage.tsx` | 生命周期控制区同样在启动按钮旁显示“当前没有存档，请点击此按钮去创建/上传存档。”和“创建/上传存档”按钮 |
| `frontend/src/games/stardew/pages/ServerControlPage.tsx` | 启动接口返回 `save_required` 时就地展示引导并刷新状态/存档列表，不再自动导航到存档页 |
| `frontend/src/games/stardew/StardewPanel.css` | 新增 `.sd-start-save-required` 样式，提示条用金色警告语义，支持换行，避免窄屏挤压 |

### 影响接口/文件

- 后端 API、Junimo 通信、存档创建/上传接口均未改。
- 仅当前端状态为 `save_required`，或启动请求刚收到 `save_required` 错误码时显示提示和按钮。
- “创建/上传存档”按钮只导航到存档页，不带 `saveAction`，让用户自己选择创建或上传。
- `MVP-UX-1` 新增的 `saveActionRequest` 机制仍保留，但本轮启动失败流程不再使用它自动打开新建弹窗。

### 如何验证

```powershell
cd E:\stardew-server-anxi-panel\frontend
npm.cmd run build
# exit 0，39 modules，CSS 89.96 kB，JS 327.04 kB
```

建议真实页面补测：

1. 无任何有效存档时进入总览页，状态为 `save_required` 后，启动按钮旁应显示无存档提示和“创建/上传存档”按钮。
2. 点击“创建/上传存档”应进入 `/instances/stardew/saves`，不自动打开新建弹窗。
3. 在服务器控制页重复同样检查。
4. 有存档、`ready_to_start`、`stopped`、`running` 状态下不应显示该提示条。

### 下一步注意

- 如果真实后端返回 `save_required` 但没有及时把 instance state 更新为 `save_required`，本轮本地 `saveRequiredDetected` 会先显示引导；状态恢复到其他值后会自动隐藏。
- 文案里统一使用“存档”，没有沿用用户消息里的“文档”字样。

---

## MVP-UX-1: 无存档启动时直达创建存档界面

### 目标

用户向功能验证发现：MVP 基本完成后，在没有存档的情况下点击“启动服务器”，前端只弹出“没有可用存档，请先创建或上传存档”的提示，没有跳转到创建存档界面。目标是让这条首次启动路径自动进入下一步，而不是让用户自己找入口。

### 改了什么

| 文件/范围 | 修改 |
|------|------|
| `frontend/src/games/stardew/stardew-routes.ts` | 新增 `StardewNavigateOptions`、`StardewSaveActionRequest`，让页面导航可以携带一次性的存档操作意图 |
| `frontend/src/games/stardew/StardewPanel.tsx` | 保存 `saveActionRequest` 并传给当前页面；`navigate('saves', { saveAction: 'new' })` 即可触发存档页动作 |
| `frontend/src/games/stardew/pages/OverviewPage.tsx` | `startInstance()` 返回 `save_required` 时刷新状态/存档并跳到存档页，同时自动打开新建游戏弹窗；已处于 `save_required` 状态时按钮改为“创建存档” |
| `frontend/src/games/stardew/pages/ServerControlPage.tsx` | 生命周期启动失败遇到 `save_required` 时执行同样跳转和弹窗打开逻辑 |
| `frontend/src/games/stardew/pages/SavesPage.tsx` | 将 `saveActionRequest` 传入 `SavesSection` |
| `frontend/src/games/stardew/SavesSection.tsx` | 收到 `saveActionRequest.action === 'new'` 时滚动到存档区域并打开“新建游戏”弹窗；保留 `upload` 动作扩展入口 |

### 影响接口/文件

- 前端内部页面导航签名从 `onNavigate(route)` 扩展为 `onNavigate(route, options?)`；现有只传 route 的调用保持兼容。
- 后端 API、Junimo 通信、存档创建接口、上传接口均未改动。
- `save_required` 会直达新建存档弹窗；`active_save_required` / `active_save_missing` 只跳转存档页，避免已有存档时误导用户新建。

### 如何验证

```powershell
cd E:\stardew-server-anxi-panel\frontend
npm.cmd run build
# exit 0，39 modules，CSS 89.68 kB，JS 326.33 kB
```

建议真实页面补测：

1. 准备一个已安装但没有任何有效存档的实例。
2. 在总览页点击“启动”。
3. 预期跳转到 `/instances/stardew/saves`，并直接显示“新建游戏”弹窗。
4. 在服务器控制页重复同样操作，预期一致。
5. 如果后端返回 `active_save_required` 或 `active_save_missing`，预期只进入存档页让用户选择已有存档。

### 下一步注意

- 这次只修 UX 串联，不改变启动状态机；如果真实联调中发现后端没有返回 `save_required` code，而是包在 `start_failed` message 里，需要继续收口后端错误码。
- `saveAction: 'upload'` 已预留并在 `SavesSection` 支持，但当前启动失败默认选择新建存档，因为用户反馈的是“跳转到创建存档界面”。

---

## UI-R18: Stardew wood strip 背景按 image2 风格重新生成

### 目标

用户要求对 `E:\stardew-server-anxi-panel\frontend\dist\assets\stardew\ui\backgrounds\background_frame_wood_strip.png` 用 image2 风格重新生成，明确要求不要简单重绘，而是按满意参考图的高级 Stardew 像素质感重新做对应素材。实际执行时修改 `frontend/public/assets/stardew/ui/backgrounds/background_frame_wood_strip.png` 源素材，再通过 `npm.cmd run build` 同步到 dist。

### 改了什么

| 文件/范围 | 修改 |
|------|------|
| `frontend/public/assets/stardew/ui/backgrounds/background_frame_wood_strip.png` | 按 image2 木条参考重新生成 256x14 不透明 wood strip |
| `frontend/dist/assets/stardew/ui/backgrounds/background_frame_wood_strip.png` | 通过 `npm.cmd run build` 同步 |
| `tmp/stardew-background-frame-wood-strip-readable-contact-sheet.png` | 新增本轮 wood strip 可读性 contact sheet |

### 视觉实现

- 先用 image2 生成高阶 Stardew 横向木条参考，再本地按原尺寸重新导出最终 PNG。
- 保留原文件名、原尺寸 `256x14`、原目录结构。
- 保持不透明位图，因为该素材在 `--sd-img-bg-wood-strip` 中作为结构性背景使用。
- 根据页面截图反馈，第一版过亮且细节过多，导致 topbar 上白字不清晰；已改为更暗、更克制的深胡桃木条。
- 设计为可横向 `repeat-x` 的低干扰木条：深棕描边、低对比金色顶光、轻量木纹颗粒、少量木板接缝和像素级阴影，优先保证 topbar 文案可读。
- 未触碰 `new-game`、业务逻辑、API、CSS 路径。

### 验证

```powershell
cd E:\stardew-server-anxi-panel\frontend
npm.cmd run build
# exit 0，39 modules，CSS 89.68 kB，JS 325.25 kB
```

补充检查：

```powershell
# public/dist background_frame_wood_strip.png hash 一致
# alpha 扫描确认 HasAlpha=False，尺寸仍为 256x14
# git diff --name-only | Select-String 'frontend/public/assets/stardew/new-game|frontend/dist/assets/stardew/new-game' 无匹配
```

### 下一步注意

- 真实页面里重点看 topbar 背景重复后的木板接缝是否仍然过密；如果显得太花，继续单独调低接缝和颗粒密度即可。
- 这个文件被 `stardew-theme.css` 的 `--sd-img-bg-wood-strip` 引用，后续不要改路径；如要继续微调，优先保持 `256x14` 和不透明结构。

---

## UI-R17: Stardew panels 按 image2 风格重新生成

### 目标

用户要求对 `E:\stardew-server-anxi-panel\frontend\dist\assets\stardew\ui\panels` 下的 panels 素材用 image2 重绘，明确要求不要简单重绘，而是按文件名一一对应生成更好看、更高级的 Stardew 像素面板皮肤。实际执行时修改 `frontend/public/assets/stardew/ui/panels/` 源素材，再通过 `npm.cmd run build` 同步到 dist。

### 改了什么

| 文件/范围 | 修改 |
|------|------|
| `frontend/public/assets/stardew/ui/panels/*.png` | 从 image2 panel sheet 参考重新生成 6 个 panel |
| `frontend/dist/assets/stardew/ui/panels/*.png` | 通过 `npm.cmd run build` 同步 |
| `tmp/stardew-panels-image2-generated-contact-sheet.png` | 新增本轮 panels contact sheet |

### 视觉实现

- 先用 image2 生成高阶 Stardew panel sheet 参考，再按每个文件名裁切和重采样到原尺寸。
- 6 个文件均保留原文件名、原尺寸、原目录结构。
- 6 个文件均保持不透明位图，因为它们作为结构性背景皮肤使用。
- `panel_metric_card_blank.png`：小型指标卡片，木框 + 羊皮纸内底。
- `panel_mod_card_blank.png`：中型模组卡片，紧凑木框面板。
- `panel_parchment_form_blank.png`：表单面板，羊皮纸底和四角铆钉。
- `panel_parchment_section_blank.png`：宽 section 面板，适合横向内容区。
- `panel_table_area_blank.png`：表格区域，保留浅色网格行列。
- `panel_warning_row_blank.png`：warning row，暖红/琥珀警告底色。
- 未触碰 `new-game`、业务逻辑、API、CSS 路径。

### 验证

```powershell
cd E:\stardew-server-anxi-panel\frontend
npm.cmd run build
# exit 0，39 modules，CSS 89.68 kB，JS 325.25 kB
```

补充检查：

```powershell
# public/dist panels PNG hash 一致
# alpha 扫描确认 6 个 panels PNG 均 HasAlpha=False
# git diff --name-only | Select-String 'frontend/public/assets/stardew/new-game|frontend/dist/assets/stardew/new-game' 无匹配
```

### 下一步注意

- panels 是结构性背景皮肤，后续微调时优先保持原尺寸和不透明背景。
- 如果真实页面中某个大面板拉伸后边框或角钉显重，单独微调对应 PNG，不要整批回退。

---

## UI-R16: Stardew sprites 按 image2 风格重新生成

### 目标

用户要求对 `E:\stardew-server-anxi-panel\frontend\dist\assets\stardew\ui\sprites` 下的 sprites 素材用 image2 重绘，明确要求不要简单重绘，而是按文件名一一对应生成更好看、更高级的 Stardew 像素 sprite。实际执行时修改 `frontend/public/assets/stardew/ui/sprites/` 源素材，再通过 `npm.cmd run build` 同步到 dist。

### 改了什么

| 文件/范围 | 修改 |
|------|------|
| `frontend/public/assets/stardew/ui/sprites/*.png` | 从 image2 sprite sheet 参考重新生成 8 个 sprite |
| `frontend/dist/assets/stardew/ui/sprites/*.png` | 通过 `npm.cmd run build` 同步 |
| `tmp/stardew-sprites-image2-generated-contact-sheet.png` | 新增本轮 sprites contact sheet |

### 视觉实现

- 先用 image2 生成高阶 Stardew sprite sheet 参考，再按每个文件名裁切、抠图和重采样到原尺寸。
- 8 个文件均保留原文件名、原尺寸、原目录结构。
- 7 个小 sprite 保持透明背景：`sprite_blue_device.png`、`sprite_blue_gem.png`、`sprite_chest.png`、`sprite_cloud_left.png`、`sprite_cloud_right.png`、`sprite_fence.png`、`sprite_tree.png`。
- `sprite_farmhouse_scene.png` 保持不透明 158x92 场景图；它被 Overview banner 作为背景图使用，不适合透明黑底裁切。
- 未触碰 `new-game`、业务逻辑、API、CSS 路径。

### 验证

```powershell
cd E:\stardew-server-anxi-panel\frontend
npm.cmd run build
# exit 0，39 modules，CSS 89.68 kB，JS 325.25 kB
```

补充检查：

```powershell
# public/dist sprites PNG hash 一致
# alpha 扫描确认 7 个小 sprite HasAlpha=True，sprite_farmhouse_scene.png HasAlpha=False
# git diff --name-only | Select-String 'frontend/public/assets/stardew/new-game|frontend/dist/assets/stardew/new-game' 无匹配
```

### 下一步注意

- 真实页面里重点看 Overview 顶部横幅的农舍背景是否与文字 overlay 对比足够。
- 小 sprite 当前按原尺寸保守输出；如果后续要让它们在页面里大尺寸展示，可以单独为对应使用处加 CSS 显示尺寸，而不是改变所有源图尺寸。

---

## UI-R15: Stardew navigation 按 image2 风格重新生成

### 目标

用户要求对 `E:\stardew-server-anxi-panel\frontend\dist\assets\stardew\ui\navigation` 下的 navigation 素材用 image2 重绘，明确要求不要简单重绘，而是按文件名一一对应生成更好看、更高级的 Stardew 像素 UI 皮肤。实际执行时修改 `frontend/public/assets/stardew/ui/navigation/` 源素材，再通过 `npm.cmd run build` 同步到 dist。

### 改了什么

| 文件/范围 | 修改 |
|------|------|
| `frontend/public/assets/stardew/ui/navigation/*.png` | 重新生成 7 个 navigation PNG |
| `frontend/dist/assets/stardew/ui/navigation/*.png` | 通过 `npm.cmd run build` 同步 |
| `tmp/stardew-navigation-image2-generated-contact-sheet.png` | 新增本轮 navigation contact sheet |

### 视觉实现

- 先用 image2 生成高阶 Stardew 导航皮肤方向，再按每个文件原尺寸本地导出，避免破坏现有 CSS 背景拉伸。
- 7 个文件均保留原文件名、原尺寸、原目录结构。
- 7 个文件仍保持不透明位图，因为它们作为背景皮肤使用，不应改成透明大图。
- `nav_item_default_blank.png`：深木纹默认导航条。
- `nav_item_active_green_blank.png`：绿色 active 导航条，带厚木框、角钉、高光和内阴影。
- `nav_item_active_saves_blank.png`：存档专用 active，羊皮纸内底 + 绿色下划强调。
- `nav_quick_help_blank.png`：小型木质帮助按钮。
- `tab_content_active_blank.png` / `tab_content_inactive_blank.png`：active/inactive tab 内容皮肤区分。
- `tab_top_green_blank.png`：绿色顶部 tab，带 raised tab 结构。
- 未触碰 `new-game`、业务逻辑、API、CSS 路径。

### 验证

```powershell
cd E:\stardew-server-anxi-panel\frontend
npm.cmd run build
# exit 0，39 modules，CSS 89.68 kB，JS 325.25 kB
```

补充检查：

```powershell
# public/dist navigation PNG hash 一致
# alpha 扫描确认 7 个 navigation PNG 均 HasAlpha=False
# git diff --name-only | Select-String 'frontend/public/assets/stardew/new-game|frontend/dist/assets/stardew/new-game' 无匹配
```

### 下一步注意

- navigation 是结构性背景皮肤，后续微调时优先保持原尺寸和不透明背景。
- 如果真实页面中某个 tab 或左侧导航拉伸后边框显重，单独微调对应 PNG，不要整批回退。

---

## UI-R14: Stardew 图标按满意参考图直接裁切

### 目标

用户明确确认 `C:\Users\anxi\AppData\Local\Temp\codex-clipboard-b6edf34e-1046-4b35-b2ac-4b3dd6d502b7.png` 这张 4x4 图标图满意，要求“就按照这张图来”，且像素大小可以和图中元素一致。本轮目标是停止继续风格再生成，直接使用这张参考图作为最终视觉源。

### 改了什么

| 文件/范围 | 修改 |
|------|------|
| `frontend/public/assets/stardew/ui/icons/*.png` | 从参考图手工框裁切并抠透明，输出 16 个大尺寸 PNG 图标 |
| `frontend/dist/assets/stardew/ui/icons/*.png` | 通过 `npm.cmd run build` 同步 |
| `tmp/stardew-icons-reference-crop-contact-sheet.png` | 新增本轮裁切版 contact sheet |

### 映射关系

参考图按 4x4 从左到右、从上到下对应：

1. `icon_button_play.png`
2. `icon_button_restart.png`
3. `icon_button_stop.png`
4. `icon_nav_diagnostics.png`
5. `icon_nav_mods.png`
6. `icon_nav_overview_home.png`
7. `icon_nav_players.png`
8. `icon_nav_saves.png`
9. `icon_nav_server_control.png`
10. `icon_nav_settings.png`
11. `icon_nav_tasks.png`
12. `icon_sidebar_chicken.png`
13. `icon_top_summary_players.png`
14. `icon_top_summary_save.png`
15. `icon_top_summary_time.png`
16. `icon_top_summary_version.png`

### 实现细节

- 直接按参考图元素裁切，不再压缩回原来的 13x13/16x17 小尺寸。
- 本地抠图保留高光、深棕描边、像素阴影和主体细节，输出 PNG 透明背景。
- CSS 已对导航、页面标题、顶部摘要和按钮内图标做显示尺寸约束，所以大尺寸源图不会撑坏布局。
- 未触碰 `new-game`、业务逻辑、API、CSS 路径。

### 验证

```powershell
cd E:\stardew-server-anxi-panel\frontend
npm.cmd run build
# exit 0，39 modules，CSS 89.68 kB，JS 325.25 kB
```

补充检查：

```powershell
# public/dist icons PNG hash 一致
# 16 个图标四角 alpha 均为 0
# git diff --name-only | Select-String 'frontend/public/assets/stardew/new-game|frontend/dist/assets/stardew/new-game' 无匹配
```

### 下一步注意

- 如果真实页面里某些图标被 CSS 缩到 13px 后细节过密，可以只针对对应使用处调整显示尺寸或单独微调对应 PNG。
- 不建议再用生成模型重跑整批；当前用户已明确满意这张参考图风格。

---

## UI-R13: Stardew 图标按文件名重新生成

### 目标

用户反馈 UI-R12 的图标仍像简单重绘，要求“不要简单重绘”，而是按参考图风格和图片名一一对应重新生成一批更好看、更高级的图标。实际仍遵循项目资产规则：修改 `frontend/public/assets/stardew/ui/icons/` 源文件，再通过 `npm.cmd run build` 同步到 `frontend/dist/assets/stardew/ui/icons/`。

### 改了什么

| 文件/范围 | 修改 |
|------|------|
| `frontend/public/assets/stardew/ui/icons/*.png` | 重新生成 16 个透明 PNG 图标 |
| `frontend/dist/assets/stardew/ui/icons/*.png` | 通过 `npm.cmd run build` 同步 |
| `tmp/stardew-icons-image2-generated-contact-sheet.png` | 新增本轮图标 contact sheet |

### 视觉实现

- 使用 imagegen 生成高阶 Stardew 像素风图标参考，但最终导出仍按每个原始 PNG 的尺寸和透明边界本地生成，避免破坏现有 CSS 对齐。
- 16 个图标按文件名语义一一对应重新设计：
  - `icon_button_play.png`、`icon_button_restart.png`、`icon_button_stop.png`
  - `icon_nav_diagnostics.png`、`icon_nav_mods.png`、`icon_nav_overview_home.png`、`icon_nav_players.png`
  - `icon_nav_saves.png`、`icon_nav_server_control.png`、`icon_nav_settings.png`、`icon_nav_tasks.png`
  - `icon_sidebar_chicken.png`
  - `icon_top_summary_players.png`、`icon_top_summary_save.png`、`icon_top_summary_time.png`、`icon_top_summary_version.png`
- 风格上使用暖金高光、深棕描边、局部阴影和更明确的图标轮廓；13x13 小图标优先保证真实页面里的识别度。
- 未触碰 `new-game`、业务逻辑、API、CSS 路径。

### 验证

```powershell
cd E:\stardew-server-anxi-panel\frontend
npm.cmd run build
# exit 0，39 modules，CSS 89.68 kB，JS 325.25 kB
```

补充检查：

```powershell
# public/dist icons PNG hash 一致
# 16 个 icons alpha 扫描均 HasAlpha=True
# git diff --name-only | Select-String 'new-game' 无匹配
```

### 下一步注意

- 真实页面里重点看 13x13 导航图标在左侧按钮上的可读性。
- 如果某个极小图标仍不够清楚，建议只微调对应 PNG，不要整组回退。

---

## UI-R12: Stardew 图标位图高级重绘

### 目标

用户要求对 `E:\stardew-server-anxi-panel\frontend\dist\assets\stardew\ui\icons` 下图标素材用 image2 风格重绘，要求更高级、更好看。实际执行时修改 `frontend/public/assets/stardew/ui/icons/` 源文件，再运行 build 同步到 dist。

### 改了什么

| 文件/范围 | 修改 |
|------|------|
| `frontend/public/assets/stardew/ui/icons/*.png` | 重绘 16 个透明图标 PNG |
| `frontend/dist/assets/stardew/ui/icons/*.png` | 通过 `npm.cmd run build` 同步 |
| `tmp/stardew-icons-image2-premium-contact-sheet.png` | 新增 5 倍放大图标 contact sheet |

### 视觉实现

- 保留每个图标原尺寸、原文件名、原目录结构。
- 使用高级像素图标风格：深色投影、暖金主体、浅色高光。
- 保留透明背景，适配现有按钮/导航/摘要区域。
- 13x13 小导航图标优先保证语义可读；玩家图标已单独加强为多人轮廓。
- 未触碰 `new-game`、业务逻辑、API、CSS 路径。

### 验证

```powershell
cd E:\stardew-server-anxi-panel\frontend
npm.cmd run build
# exit 0，39 modules，CSS 89.68 kB，JS 325.25 kB
```

补充检查：

```powershell
git diff --name-only -- frontend/public/assets/stardew/ui/icons | Measure-Object
# 16
```

- public/dist icons PNG hash 一致。
- alpha 扫描确认 16 个图标均保留透明背景。
- `new-game` 无变更。

### 下一步注意

- 真实页面里重点看 13x13 导航图标在左侧按钮上的识别度。
- 如果某个图标语义不够清楚，建议单独微调该图标，不要回退整组。

---

## UI-R11: Stardew 输入框位图高级重绘

### 目标

用户要求对 `E:\stardew-server-anxi-panel\frontend\dist\assets\stardew\ui\fields` 下输入框素材用 image2 风格重绘，要求更高级、更好看。实际执行时修改 `frontend/public/assets/stardew/ui/fields/` 源文件，再运行 build 同步到 dist。

### 改了什么

| 文件/范围 | 修改 |
|------|------|
| `frontend/public/assets/stardew/ui/fields/*.png` | 重绘 4 个输入框 PNG |
| `frontend/dist/assets/stardew/ui/fields/*.png` | 通过 `npm.cmd run build` 同步 |
| `tmp/stardew-fields-image2-premium-contact-sheet.png` | 新增输入框 contact sheet |

### 视觉实现

- 保留每个输入框原尺寸、原文件名、原目录结构。
- 使用高级像素输入框皮肤：深木边框、羊皮纸内底、内高光、底部阴影、细纸纹。
- 搜索框保留右侧像素放大镜；下拉框保留右侧控制区和像素箭头。
- 4 个输入框全部保持不透明，避免 CSS 背景拉伸漏底。
- 未触碰 `new-game`、业务逻辑、API、CSS 路径。

### 验证

```powershell
cd E:\stardew-server-anxi-panel\frontend
npm.cmd run build
# exit 0，39 modules，CSS 89.68 kB，JS 325.25 kB
```

补充检查：

```powershell
git diff --name-only -- frontend/public/assets/stardew/ui/fields | Measure-Object
# 4
```

- public/dist fields PNG hash 一致。
- alpha 扫描确认 4 个输入框均不透明。
- `new-game` 无变更。

### 下一步注意

- 真实页面里重点看输入框文字、placeholder 和右侧图标的对比度。
- 如果某个输入框在 CSS 拉伸后边框太重，优先单独微调该 PNG，不要回退整组。

---

## UI-R10: Stardew 按钮位图高级重绘

### 目标

用户要求对 `E:\stardew-server-anxi-panel\frontend\dist\assets\stardew\ui\buttons` 下按钮用 image2 风格重绘，要求更高级、更好看。实际执行时修改 `frontend/public/assets/stardew/ui/buttons/` 源文件，再运行 build 同步到 dist。

### 改了什么

| 文件/范围 | 修改 |
|------|------|
| `frontend/public/assets/stardew/ui/buttons/*.png` | 重绘 12 个按钮 PNG |
| `frontend/dist/assets/stardew/ui/buttons/*.png` | 通过 `npm.cmd run build` 同步 |
| `tmp/stardew-buttons-image2-premium-contact-sheet.png` | 新增按钮 contact sheet |

### 视觉实现

- 保留每个按钮原尺寸、原文件名、原目录结构。
- 使用高级像素按钮皮肤：外层木框硬边、内高光、底部阴影、细颗粒纹理。
- 按语义重绘 green/red/tan/gold/wood 变体。
- 12 个按钮全部保持不透明，避免 CSS 背景拉伸漏底。
- 未触碰 `new-game`、业务逻辑、API、CSS 路径。

### 验证

```powershell
cd E:\stardew-server-anxi-panel\frontend
npm.cmd run build
# exit 0，39 modules，CSS 89.68 kB，JS 325.25 kB
```

补充检查：

```powershell
git diff --name-only -- frontend/public/assets/stardew/ui/buttons | Measure-Object
# 12
```

- public/dist buttons PNG hash 一致。
- alpha 扫描确认 12 个按钮均不透明。
- `new-game` 无变更。

### 下一步注意

- 真实页面里重点看按钮文字在绿色/红色按钮上的对比度。
- 如果某个按钮因 CSS 拉伸显得边框过重，优先单独微调该按钮，不要回退整组。

---

## UI-R9: 左侧导航点击态尺寸统一

### 目标

用户反馈左侧栏除“服务器”外，其他导航按钮点击后的效果尺寸长宽不一致。目标是让所有桌面端左侧导航按钮的点击/激活态视觉尺寸与“服务器”项一致。

### 改了什么

| 文件 | 修改 |
|------|------|
| `frontend/src/games/stardew/StardewPanel.css` | 将 `.sd-sidebar .sd-nav-item` 默认尺寸改为服务器项尺寸：`--sd-nav-w: 110`、`--sd-nav-active-w: 106`、`--sd-nav-active-h: 29` |
| `frontend/src/games/stardew/StardewPanel.css` | 删除 `overview/server/saves/jobs/players/mods/diagnostics/install/settings` 各自单独设置 `--sd-nav-w` / `--sd-nav-active-w` / `--sd-nav-active-h` 的规则 |
| `frontend/src/games/stardew/StardewPanel.css` | `saves` 继续使用 `nav_item_active_saves_blank.png` 专用激活贴图，但不再覆盖激活态尺寸 |

### 影响范围

- 只影响 Stardew 面板左侧导航的桌面端按钮宽度与激活态贴图尺寸。
- 不改导航 PNG 素材、不改路由、不改页面组件、不改后端。
- 移动端侧栏规则保持原样：图标按钮仍为 `min-width: 36px; height: 30px`。

### 验证

```powershell
cd E:\stardew-server-anxi-panel\frontend
npm.cmd run build
# exit 0，39 modules，CSS 89.68 kB，JS 325.25 kB
```

### 下一步注意

- 当前工作区已有较多 UI PNG 与 CSS 改动，`StardewPanel.css` 的 diff 会混入之前的状态语义色等改动；本轮只新增/调整左侧导航尺寸规则。
- 如果后续继续微调左侧栏，优先改 CSS 尺寸变量，避免为了尺寸问题再次改导航贴图素材。

---

## UI-R8: Stardew UI 位图资产按参考图重制（已完成）

### 目标

按用户要求只优化 `frontend/public/assets/stardew/ui/` 目录下的 UI 位图素材，解决原素材截图感、压缩感、缺失/破损感明显的问题。不要改 `new-game` 目录，不改业务逻辑、不改 API、不改 CSS 路径。

### 改了什么

按用户提供的参考图 `codex-clipboard-da9ce68b-ffb8-448e-bd80-030206b9aa24.png` 重制 `frontend/public/assets/stardew/ui/` 下 56 个 tracked PNG。实现方式不是 imagegen 大图硬缩，而是从参考图中裁切对应 UI 元素，按项目原尺寸重采样导出。`background_sidebar_wood_tile.png` 已按用户要求单独回退到原始版本，并通过 build 同步到 dist。

| 范围 | 修改 |
|------|------|
| `frontend/public/assets/stardew/ui/backgrounds` | 重制黑底、木条、羊皮纸 tile；保留 `background_login_farm_generated.png`；`background_sidebar_wood_tile.png` 单独回退 |
| `frontend/public/assets/stardew/ui/buttons` | 按参考图重制绿色、红色、tan、gold、wood 和方形按钮 |
| `frontend/public/assets/stardew/ui/fields` | 按参考图重制邀请码框、路径输入框、搜索框、下拉框 |
| `frontend/public/assets/stardew/ui/navigation` | 重制默认/激活导航和 tab 皮肤 |
| `frontend/public/assets/stardew/ui/panels` | 重制大羊皮纸面板、表格区域、表单卡、warning row、metric/mod 卡 |
| `frontend/public/assets/stardew/ui/icons` | 从参考图按钮网格裁切图标并扣透明背景 |
| `frontend/public/assets/stardew/ui/sprites` | 从参考图裁切蓝宝石、箱子、云、农舍场景、栅栏、树等 sprite |
| `tmp/stardew-ui-assets-reference-crop-contact-sheet.png` | 新增本轮视觉总览图 |

### 实现原则

- 只处理 `frontend/public/assets/stardew/ui/`，未触碰 `frontend/public/assets/stardew/new-game/`。
- 保留所有旧文件名、原尺寸、原目录结构。
- 结构性 UI 资产强制不透明：backgrounds/buttons/fields/navigation/panels。
- 透明图只保留在 icons 和部分 sprites；`sprite_farmhouse_scene.png` 按不透明场景图处理。
- `background_login_farm_generated.png` 是 UI-R7 登录页背景，本轮保留，不纳入 tracked 旧资产重制。
- `background_sidebar_wood_tile.png` 已单独回退；当前 UI diff 计数因此为 56。

### 如何验证

```powershell
cd E:\stardew-server-anxi-panel\frontend
npm.cmd run build
# exit 0，39 模块，JS 325.25 kB，CSS 90.39 kB
```

补充检查：

```powershell
git diff --name-only -- frontend/public/assets/stardew/ui | Measure-Object
# 56

git diff --name-only -- frontend/public/assets/stardew/ui | Select-String new-game
# 无输出
```

视觉检查：
- `tmp/stardew-ui-assets-reference-crop-contact-sheet.png` 已生成。
- contact sheet 中结构件尺寸保持原样，透明图未变黑底。

### 下一步注意事项

- 重点看真实页面里的按钮、导航、面板背景和 13x13 小图标语义。
- 如果某几个图标不够清楚，建议单独微调，不要整批回退。
- `new-game` 下农场/宠物/角色预览图本轮未处理。

---

## UI-R7: 登录首页视觉重构（Stardew 风格统一）

### 目标

将登录/初始化页面从原有的普通后台 SaaS 风格完全重构为 Stardew Valley 像素风，使其与面板内部 (`StardewPanel`) 视觉系统归属同一套产品体系。不改动登录 API、认证逻辑、后端，只调整组件结构、className 和 CSS。

### 改了什么

| 文件 | 修改 |
|------|------|
| `frontend/src/App.css` | 末尾新增 `/* ── UI-R7: Stardew Auth Shell ──` 块，约 230 行，包含 `.sd-auth-shell`、`.sd-auth-card`、`.sd-auth-eyebrow`、`.sd-auth-title`、`.sd-auth-version`、`.sd-auth-error`、`.sd-auth-loading`，及 `.sd-auth-card .form-grid/field/field input/password-input/password-toggle/button/form-hint` 上下文覆盖样式，加 `@media (max-width: 430px)` 和 `@media (max-width: 340px)` 断点 |
| `frontend/src/App.tsx` | 非 stardew 视图的 return 从 `main.shell > section.panel-card` 改为 `main.sd-auth-shell > section.sd-auth-card`；`p.eyebrow` → `p.sd-auth-eyebrow`；`h1` 加 `className="sd-auth-title"`；`p.version-info` → `p.sd-auth-version`；`div.error-banner` → `div.sd-auth-error`；`p.summary` → `p.sd-auth-loading`（仅 booting 状态） |
| `frontend/src/core/LoginPanel.tsx` | 移除 `<p className="summary">请输入面板账号登录……</p>` 段落，表单直接从字段开始，符合"不使用大段说明文字"要求 |
| `frontend/public/assets/stardew/ui/backgrounds/background_login_farm_generated.png` | 新增登录页原创像素农场背景图，替代直接拉伸 `sprite_farmhouse_scene.png` |

**SetupPanel.tsx 未改动**（保留初始化说明文字，对首次使用者有必要性）。

### 视觉设计实现细节

**背景：**
- `.sd-auth-shell::before`：`background-image: url('/assets/stardew/ui/backgrounds/background_login_farm_generated.png')`，`background-size: cover`，`background-position: center`
- 不再把 `sprite_farmhouse_scene.png` 直接拉伸成整页背景，避免低分辨率素材放大后发糊
- `.sd-auth-shell::after`：`rgba(12, 6, 1, 0.48)` 半透明暗色蒙层，保证面板可读性而不遮挡背景氛围
- 新背景图已在 `frontend/public/assets/stardew/ui/backgrounds/`，build 后以 `/assets/stardew/ui/backgrounds/background_login_farm_generated.png` 访问

**面板：**
- 桌面端 `.sd-auth-shell` 使用 `justify-content: flex-end`，并通过 `padding-right: clamp(36px, 11vw, 180px)` 将卡片放到右侧背景留白区，避免遮挡左侧农舍主视觉
- 900px 以下回到居中，避免窄桌面/平板卡片贴边
- 5px 深棕边框 `#5b2f18`，零圆角（像素风）
- `background-image` 使用暖色渐变叠加 `background_parchment_tile.png`，`background-color: #e7b96f`，降低白纸感
- `box-shadow` 使用内描边 + 短木质底边 + 柔和投影，让卡片更接近背景里的木框告示牌
- 宽度 `min(420px, 100%)`，比旧版略窄

**表单覆盖（`.sd-auth-card .xxx`）：**
- 输入框：34px 高，2px 棕色边框，无圆角，`#fff0c7` 底色，13px 正文字号
- 密码切换：同上高度，`#ffe6b6` 底色
- 登录按钮：高 36px，使用 `button_primary_small_green_blank.png` PNG 底图（`background-size: 100% 100%`），无 `box-shadow`，颜色白色文字
- 错误条 `.sd-auth-error`：`border: 3px solid #b54837`，`background: #ffe0d6`，`color: #681111`（与全站 `sd-notice--error` 语义一致）

**CSS 变量依赖：** 无，`sd-auth-*` 全部使用硬编码颜色值，避免 stardew-theme.css 加载顺序依赖问题（尽管 stardew-theme.css 已全局导入）。

### 影响的接口/文件

- `frontend/src/App.css`：新增末尾样式块，不影响已有规则
- `frontend/src/App.tsx`：只改 login/setup/booting 三个视图的 className，stardew 视图路径 (`view === 'stardew'`) 完全不动
- `frontend/src/core/LoginPanel.tsx`：减少一个 `<p>` 段落，不影响 form 提交逻辑
- 旧的 `.shell`、`.panel-card`、`.eyebrow`、`.version-info` CSS 类在 App.css 中保留（不删），但不再被任何 JSX 引用（死规则，后续可按需清理）

### 如何验证

```powershell
cd E:\stardew-server-anxi-panel\frontend
npm.cmd run build
# 预期：exit 0，39 模块，JS ~325 kB，CSS ~90 kB
```

**视觉验证（Playwright 已执行）：**
- `localhost:5173`（Vite dev server）- 1280px：原创像素农场背景全屏，面板位于右侧留白区，所有元素渲染正常
- 390px：面板 100% 宽，背景图下方可见，无横向滚动
- 320px：面板紧凑，无横向滚动
- 填入账号密码后：输入框 focus 绿色 outline，密码 toggle 可用
- 错误状态：显示 Stardew 风格红色错误条

**手动检查（如需重现）：**
1. 启动 Vite dev server（已在运行）：访问 `http://localhost:5173`
2. 登录页应第一眼显示原创像素农场背景，不再显示被拉伸的 farmhouse sprite
3. 使用浏览器 DevTools，将宽度调至 390px / 320px，确认无横向滚动条
4. 错误状态：输入错误密码后查看错误条样式

### 下一步注意事项

**UI-R8（如需）：进一步精细化方向**
- 登录按钮 PNG 在某些显示器/缩放下可能偏浅，可考虑在按钮上叠加一个 `color: rgba(0,0,0,0.15)` 文字阴影增强可读性
- SetupPanel 的「创建管理员」按钮同样通过 `.sd-auth-card .button` 覆盖为 Stardew 绿色按钮，视觉一致
- 如果未来需要在登录页加 logo 图（如 farmhouse icon），可在 `.sd-auth-card` 顶部加一个 `img.sd-auth-logo`，推荐复用 `sprite_chest.png` 或 `sprite_blue_gem.png`

**清理（可选，低优先级）：**
- App.css 中 `.shell`、`.panel-card`、`.eyebrow`、`.version-info` 已无 JSX 引用，可择机删除

**后端嵌入 frontend：**
- 当前 Vite dev server (5173) 与 Go backend (8090) 分离运行
- `npm.cmd run build` 生成的 `frontend/dist/` 需要重新编译 Go binary 才能被嵌入生效
- 如果需要测试完整部署效果，需 `go build` 后重启 backend
