# Stardew Page Prototypes - image2 - 2026-06-30

本目录保存本轮按现有 Stardew 前端页面生成的页面级原型图。目标是统一左侧栏、顶部总栏和右侧任务栏，只替换每个页面的主内容区，作为后续 UI 重构或资产细化的视觉参考。

## 页面清单

| 文件 | 页面 | 路由 |
|---|---|---|
| `01-overview.png` | 总览 | `/instances/stardew/overview` |
| `02-server-control.png` | 服务器控制 | `/instances/stardew/server` |
| `03-saves.png` | 存档管理 | `/instances/stardew/saves` |
| `04-jobs-logs.png` | 任务与日志 | `/instances/stardew/jobs` |
| `05-players.png` | 玩家管理 | `/instances/stardew/players` |
| `06-mods.png` | 模组管理 | `/instances/stardew/mods` |
| `07-diagnostics.png` | 诊断与健康检查 | `/instances/stardew/diagnostics` |
| `08-install.png` | 首次安装向导 | `/instances/stardew/install` |
| `09-settings.png` | 设置与审计 | `/instances/stardew/settings` |

## 统一视觉约束

- 所有页面使用同一套 Stardew 风格外壳：深木左侧导航、顶部状态栏、右侧任务/健康摘要栏。
- 主内容区使用羊皮纸面板、木质边框、像素图标、状态灯和少量农场主题插画。
- 页面应优先呈现真实管理功能，不做营销式首页，不使用大段说明文字。
- 色彩以深木、羊皮纸、苔绿色、丰收金、浆果红、天空蓝为主，避免单色系。
- 这些图是视觉原型，不是 HTML，不应直接当作可点击页面或前端实现产物。

## 验证记录

- 生成方式：内置 image2 / imagegen。
- 生成日期：2026-06-30。
- 图片数量：9。
- 图片尺寸：全部为 `1672 x 941`。
- 保存策略：默认生成目录保留原图，本目录保存项目内可交接副本。
