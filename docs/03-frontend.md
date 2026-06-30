# 前端文档

## 总体结构

前端使用 React + TypeScript + Vite。`App.tsx` 负责启动、初始化、登录和进入 Stardew 面板；Stardew 专属页面放在 `frontend/src/games/stardew`。

推荐边界：

```text
frontend/src/api.ts                    后端 API 封装
frontend/src/types.ts                  前后端类型
frontend/src/core                      通用组件与 helper
frontend/src/games/stardew             Stardew 面板
frontend/public/assets/stardew/ui      生产 UI 素材
```

不要让业务组件依赖 `docs/prototypes` 路径；生产素材必须在 `frontend/public/assets/...` 下。

## 路由

当前保持 Single Game Mode。登录后默认进入：

```text
/instances/stardew/overview
```

Stardew 面板内部路由：

| 路由 | 用途 |
| --- | --- |
| `install` | 安装向导、Steam Auth、任务日志 |
| `overview` | 日常总览、邀请码、状态摘要、近期任务 |
| `server` | 启停重启、命令、喊话、控制信息 |
| `saves` | 存档列表、新建、上传预览、选择、删除、导出 |
| `jobs` | 任务与日志 |
| `players` | 玩家名册、在线状态、位置展示 |
| `mods` | Mod 工作台：下载模组、添加模组、配置模组 |
| `diagnostics` | 健康检查、Docker/Compose、支持包 |
| `settings` | 面板用户、审计日志、版本、登出 |

当前未使用 `react-router-dom`，路由通过内部 route + History API 管理。进入 Multi Game Mode 时再考虑正式路由库。

## 数据层

`useStardewDashboardData` 是 Stardew 页面共享数据层，集中维护：

- 实例状态。
- 邀请码。
- saves/mods/jobs/health/players 等摘要。
- 操作后的刷新函数。
- 启动/重启后等待新邀请码的轮询。

页面组件优先调用共享数据层和 `api.ts` 中已有函数，不要在页面里重复拼 API。

## UI 与素材

Stardew UI 使用像素风资源：

```text
frontend/public/assets/stardew/ui/backgrounds
frontend/public/assets/stardew/ui/buttons
frontend/public/assets/stardew/ui/fields
frontend/public/assets/stardew/ui/icons
frontend/public/assets/stardew/ui/navigation
frontend/public/assets/stardew/ui/panels
frontend/public/assets/stardew/ui/sprites
```

重要原则：

- 保留素材原文件名、尺寸和目录结构，避免 CSS 路径失效。
- 图标、按钮、输入框素材从 `public` 进入构建产物，`npm run build` 后同步到 `dist/assets/...`。
- `new-game` 资产和 UI 资产分开维护，不要误改角色/农场预览素材。
- UI 文案要短，按钮和卡片在 320px 宽度也不能溢出。

## 页面职责

| 页面 | 已接入重点 | 注意事项 |
| --- | --- | --- |
| Overview | 状态、邀请码、快速操作、当前存档、健康摘要 | 不承载全部复杂管理 |
| ServerControl | 生命周期、命令、喊话、邀请码刷新 | 危险操作要确认 |
| Install | install job、Steam Guard、日志流 | 不能丢失认证交互 |
| Saves | 新建、上传、选择、删除、导出、备份入口 | running/starting 禁止危险写操作 |
| JobsLogs | 任务列表、日志详情、SSE | 长日志要可滚动 |
| Players | 玩家名册、位置、tile/pixel、中文地图名 | 第三方地图 key 未知时保留原名 |
| Mods | 三段式 Mod 工作台：下载模组（Nexus 在线搜索）、添加模组（已安装列表/玩家同步包/上传删除导出）、配置模组（后续 SMAPI 配置入口） | 运行中限制危险操作；分类编辑仅管理员可写；Nexus 搜索任意登录用户可用；当前 Nexus 安装按钮仍为待接入 |
| Diagnostics | 健康检查、Docker、支持包 | 技术信息不要淹没用户 |
| Settings | 用户、审计、版本、登出 | 面板用户不要放进玩家页 |

## 近期前端修正摘要

- `ModsPage` 参考 `E:\源码\emp_源码\dst-management-platform-web\src\views\game\mod.vue` 的 Mod 管理结构，改为页内三段工作台：`下载模组`、`添加模组`、`配置模组`。下载页承载 Nexus 搜索和卡片网格；添加页承载本服已安装 Mod、玩家同步统计、同步包导出、上传/删除/整包导出；配置页先提供启用/禁用、依赖检查、更新检查、SMAPI 配置的占位入口，等待后端能力接入。该改动只调整前端组织和视觉层级，不新增接口。
- `ModsPage` 新增”在线搜索（Nexus Mods）”区域（未新建路由，无需管理员权限）：搜索框 + 搜索按钮，关键词为空时按钮禁用并提示；搜索中按钮显示”搜索中…”；失败显示中文错误条（复用 `sd-mods-list-error`）。结果用 `NexusResultCard` 卡片展示名称、作者、版本、更新时间、下载量、认可数、已安装标记（绿色 `sd-tag`，标注已安装版本号）和”打开 N 站”按钮（`window.open` 新窗口打开 `nexusUrl`）；本阶段不放安装按钮，只放一个禁用的”安装待接入”小标记。涉及 `frontend/src/games/stardew/pages/ModsPage.tsx`、`frontend/src/games/stardew/StardewPanel.css`（新增 `.sd-mods-nexus-*` 一组类）、`frontend/src/types.ts`（`NexusModSearchResult`/`NexusModSearchResponse`，`ModInfo` 新增 `updateKeys?`/`nexusModId?`）、`frontend/src/api.ts`（`searchNexusMods`）。
- `ModsPage` 新增”玩家同步”区域（未新建路由）：Mod 卡片用 `sd-tag` 展示同步标签（服务器专用/玩家需同步/待确认），管理员可用下拉框就地修改分类；区域顶部显示三类统计 tag 和“导出玩家同步包”按钮，无 `client_required` Mod 时按钮禁用，导出中显示 loading，失败显示中文错误。普通用户只能查看分类和点击导出，看不到分类下拉框。涉及 `frontend/src/games/stardew/pages/ModsPage.tsx`、`frontend/src/games/stardew/StardewPanel.css`、`frontend/src/types.ts`、`frontend/src/api.ts`。
- 登录/首次注册页已接入 `image2` 原型图整页背景，账号/密码区域、错误提示和按钮文字由前端按背景图风格覆盖绘制；首次注册态底部提示“请尽快注册管理员账号”，按钮显示“注册”，登录态按钮显示“登录”。
- 左侧导航、按钮、输入框、图标、面板等位图资源经过多轮重绘。
- StardewShell 已拆出 9 个路由。
- 服务器控制页、存档页、任务页、玩家页、Mod 页、诊断页和设置页已真实化。
- 邀请码启动/重启后会等待新码，Overview 也提供刷新按钮。
- 玩家位置支持 SMAPI 精确字段、tile/pixel 坐标和原版地图中文映射。
- 玩家页固定展示现金、农场收入、个人收入和钱包模式；农场收入/个人收入不随共享或分开钱包切换含义。
- 玩家页“玩家活动 / 最近事件”已接入后端 `recentEvents`，展示首次记录、加入和离开事件。
- Stardew Shell 已固定为视口高度；长页面只滚动中间 `.sd-main` 内容区，左侧导航、顶部状态栏和右侧任务栏保持固定，移动端顶部栏与横向导航同样不参与页面文档滚动。

## 前端验证

常用命令：

```powershell
cd E:\stardew-server-anxi-panel\frontend
npm.cmd run build
```

开发服务器：

```powershell
cd E:\stardew-server-anxi-panel\frontend
npm.cmd run dev
```

视觉 QA 至少覆盖：

- 桌面宽屏。
- 390px 手机宽度。
- 320px 极窄宽度。
- 登录页、Overview、Server、Saves、Players、Diagnostics。
- 长中文按钮、错误提示、Modal、表格转窄屏布局。
