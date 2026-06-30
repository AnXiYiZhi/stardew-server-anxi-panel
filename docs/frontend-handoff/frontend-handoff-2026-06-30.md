# 前端接手文档 2026-06-30

## 本次文档归并

本次把旧的 UI 重构计划、原型说明、历史 handoff 和最新 UI 修正记录压缩进长期维护文档。前端接手入口改为本文和 `docs/03-frontend.md`。

影响文件：

- 新增 `docs/03-frontend.md`
- 新增 `docs/frontend-handoff/frontend-handoff-2026-06-30.md`
- 新增 `docs/06-integration.md`
- 删除旧的 `frontend-ui-refactor-implementation-plan.md` 和 `docs/prototypes` 下的说明 README
- 保留图片素材文件，删除的是旧说明文档
- 更新 `AGENTS.md` 与 `CLAUDE.md` 的阅读规则

验证方式：

```powershell
rg --files docs -g "*.md" -g "*.txt"
```

预期只出现九份维护文档。

## 前端当前状态

当前前端已经从单页 dashboard 收口为 Stardew 像素风面板：

- 登录/初始化页使用 Stardew 风格背景和表单。
- `StardewPanel` 管理内部 route、左侧导航、顶部状态、右侧摘要。
- 已有 9 个路由：install、overview、server、saves、jobs、players、mods、diagnostics、settings。
- UI 位图素材位于 `frontend/public/assets/stardew/ui`，构建后进入 `dist/assets/stardew/ui`。
- `useStardewDashboardData` 负责共享数据和刷新函数。

## 最近重点改动

### 邀请码刷新体验

启动/重启成功后调用 `requestInviteCodeRefresh()`，记录旧码、清空显示并轮询新码。Overview 邀请码区域新增刷新按钮。

涉及：

- `frontend/src/games/stardew/useStardewDashboardData.ts`
- `frontend/src/games/stardew/pages/OverviewPage.tsx`
- `frontend/src/games/stardew/pages/ServerControlPage.tsx`
- `frontend/src/games/stardew/stardew-routes.ts`
- `frontend/src/games/stardew/StardewPanel.css`

### 玩家页

玩家页显示在线/离线、host 标记、角色名、当前位置、tile/pixel 坐标。位置 key 映射为中文，原版 Stardew 1.6.15 的 `Data/Locations` 已覆盖，未知第三方地图保留原 key。

页面固定展示现金、农场收入、个人收入和钱包模式。农场收入来自 `farmIncome`，个人收入来自 `personalIncome`，二者不随共享或分开钱包切换含义；旧 `totalMoneyEarned` 仅作为兼容兜底，前端不自行重新计算。

“玩家活动 / 最近事件”已接入 `recentEvents`，展示首次记录、加入和离开事件。当前仍随玩家页 5 秒轮询刷新，SSE 实时推送留作后续优化。

涉及：

- `frontend/src/games/stardew/pages/PlayersPage.tsx`
- `frontend/src/types.ts`
- `frontend/src/api.ts`

### UI 资源

按钮、输入框、图标、导航、面板等 Stardew UI PNG 已多轮重绘。后续如果微调，优先单独调整对应 PNG，不要大批回滚。

注意：

- 不要误改 `frontend/public/assets/stardew/new-game`。
- 改 public 素材后必须运行 `npm.cmd run build`，确认 dist 同步。

### 登录/首次注册首页 image2 换图

登录态和首次注册态首页已直接使用 `docs/prototypes/stardew-page-prototypes-image2-2026-06-30/00-login.png` 作为生产背景，复制到 `frontend/public/assets/stardew/ui/backgrounds/background_login_home_image2.png`。`App.tsx` 在 `view === 'login'` 或 `view === 'setup'` 时给外层追加 `sd-auth-shell--image-login`，并用 `sd-auth-shell--login` / `sd-auth-shell--setup` 区分两种表单布局。`App.css` 通过图片坐标百分比把真实用户名、密码、确认密码、显示密码按钮、错误提示和主按钮覆盖到木框空白区；输入框和按钮文字由前端按背景图风格绘制。首次注册态底部提示固定为“请尽快注册管理员账号”，按钮显示“注册”；登录态按钮显示“登录”。主按钮 hover/active/disabled 状态使用叠层和位移实现点击反馈。

涉及：

- `frontend/src/App.tsx`
- `frontend/src/App.css`
- `frontend/src/core/LoginPanel.tsx`
- `frontend/src/core/SetupPanel.tsx`
- `frontend/public/assets/stardew/ui/backgrounds/background_login_home_image2.png`

验证方式：

- `npm.cmd run build`
- 浏览器访问开发服务，退出登录后检查登录页 1280x720、390x844、320x720。
- 填入错误账号密码并点击登录，确认真实错误提示覆盖原图错误条，按钮热区可点击。

### Shell 滚动固定

Stardew Shell 已修复长页面滚动时左右栏跟随页面上移的问题。`.sd-shell` 固定为视口高度，第二行使用 `minmax(0, 1fr)`，文档级滚动被限制在 Shell 外，实际长内容由 `.sd-main` 独立 `overflow-y: auto` 承担；左侧 `.sd-sidebar` 和右侧 `.sd-opsrail` 只在自身内容超出时内部滚动。

涉及：

- `frontend/src/games/stardew/StardewPanel.css`

验证方式：

- `npm.cmd run build`
- 进入任一长内容页面，滚动到底部时左侧导航、顶部状态栏和右侧任务栏仍停留在视口内；移动端宽度下顶部栏与横向导航也保持固定，仅页面内容区域滚动。

## 前端验证建议

构建：

```powershell
cd E:\stardew-server-anxi-panel\frontend
npm.cmd run build
```

手动检查：

- 登录页 1280px、390px、320px。
- Overview 邀请码刷新按钮和等待新码状态。
- Server 启动/停止/重启按钮和命令区。
- Players 位置中文名、未知 key 兜底、坐标展示。
- Saves/Mods 运行中危险操作禁用。
- Diagnostics 健康检查和支持包按钮。

## 下一步注意事项

- 右侧 rail 和部分页面还有继续精简空间，避免把所有功能堆在 Overview。
- 玩家事件推送未做，当前轮询足够轻量。
- 如果引入 `react-router-dom`，必须先保证 Single Game Mode 直达体验不变。
- 新增 API 前端封装时同步更新 `frontend/src/types.ts`、`frontend/src/api.ts` 和 `docs/06-integration.md`。
