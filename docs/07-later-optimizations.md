# 后期优化文档

本文只记录已经讨论过、但当前不急着做的优化。当前优先级仍是保持 Stardew 单实例面板稳定。

## 玩家事件驱动更新

当前玩家页通过 5 秒轮询：

```text
GET /api/instances/:id/players
```

后端读取：

- `.local-container/control/players.json`
- `.local-container/control/players-cache.json`

这个成本很低，不走 Docker exec，不扫大日志，当前足够稳定。

当前已完成第一版“最近事件”：后端通过 `players.json` 与 `players-cache.json` 差分生成首次记录、加入和离开事件，写入 `.local-container/control/players-events.json`，并通过 `/players` 响应的 `recentEvents` 返回。它仍属于轮询快照机制，不是实时推送。

未来如果要做更实时的事件推送：

1. SMAPI mod 继续写 `players.json` 当前快照。
2. SMAPI mod 新增 `player-events.jsonl`，追加 join/leave 事件。
3. 后端保留普通 `/players` 首次加载和兜底。
4. 新增：

```text
GET /api/instances/:id/players/stream
```

5. 前端先请求 `/players`，再订阅 SSE；断线后降级 30 秒低频轮询。

注意：

- 不要解析黄色弹窗、截图、VNC 或 OCR。
- 文件 watcher 在 Windows/WSL2/Docker bind mount 下可能不可靠，要有 mtime 兜底。
- 事件日志要轮转或截断，不能无限增长。

## 实时服务器日志流

当前任务日志和部分 command 输出已经可用，但完整 server log tail 仍可独立优化：

- 后端统一封装 compose logs tail。
- 前端 Jobs/Diagnostics 提供过滤、下载、暂停滚动。
- 支持包继续只导出尾部日志并脱敏。

## 存档与备份增强

可选方向：

- 自动备份策略：启动前、停止后、定时备份。
- 备份备注和保留策略。
- 存档对比：日期、农场、玩家、地图、大小。
- 恢复前二次确认和冲突预览。

禁止方向：

- 不做破坏性“修复存档角色槽”的自动工具。
- 不直接改 Stardew 主存档 XML 来猜测联机异常。

## Mod 管理增强

可选方向：

- 启用/禁用 Mod。
- 依赖关系和版本检查。
- Mod 更新提示。
- Mod 配置编辑。

需要注意：Stardew / SMAPI Mod 生态差异很大，先做只读检测和用户确认，不要自动修改复杂配置。

## 多游戏模式

未来进入 Multi Game Mode 后：

- 显示总面板实例列表。
- 每个游戏有自己的后端 driver 和前端 game module。
- 通用能力只包括用户、权限、任务、日志、诊断、备份入口。
- Stardew 专属 Steam Guard、邀请码、农场设置不能泄漏到其他游戏页面。

## UI 精修

可继续优化：

- 小屏表格改卡片。
- 右侧 rail 折叠。
- 部分 PNG 单独微调。
- 错误态和空态进一步统一。
- Overview 减负，把复杂管理收进对应页面。

每次视觉改动都要跑 `npm.cmd run build`，并检查 1280px、390px、320px。
