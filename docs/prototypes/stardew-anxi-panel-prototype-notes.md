# Stardew Anxi Panel Prototype Notes

本文档记录 `docs/prototypes/stardew-anxi-panel-product-prototype.html` 和 PNG 原型图的设计依据。

## EMP Interaction References

参考 `E:\源码\emp_源码\dst-management-platform-web` 后，提炼出适合本项目的交互模式：

- 动态权限菜单：登录后根据用户权限生成侧边栏菜单。
- 当前实例驱动页面：用户从总面板选择某个游戏实例后，进入该游戏自己的控制面板、Mod/插件、存档/世界、日志等页面。
- 未安装/未选择实例引导：未满足前置条件时显示清晰的引导入口，而不是空页面。
- 安装终端：安装页通过 WebSocket + xterm 展示实时命令输出。
- 状态轮询：控制面板定时刷新基础信息和系统信息。
- 批量管理：存档、Mod、日志等页面支持搜索、刷新、批量删除、导出。
- 控制台能力：常用按钮 + 自定义命令输入 + 服务器喊话。

## Product Navigation Model

长期产品不是所有游戏共用一个 Stardew 页面，而是：

```text
总面板
  -> 游戏实例列表
  -> 进入某个游戏的专属面板
```

总面板负责登录、用户、全局任务、全局 Docker 状态和所有游戏实例摘要。游戏专属面板负责该游戏自己的安装、配置、状态、控制台、存档/世界和 Mod/插件。

当前原型展示的是第一版 Stardew game panel 的关键流程。首个上线版本应使用 Single Game Mode，用户登录后直接看到 Stardew 面板，不显示总面板游戏列表。后续 Minecraft、饥荒、泰拉瑞亚、幻兽帕鲁等游戏应各自拥有自己的 game module 和视觉/交互细节，不应把它们塞进 Stardew 原型页面。

## Stardew-Inspired Visual Direction

原型选择“原创像素农场管理面板”方向，不直接使用 Stardew Valley 原始素材。

视觉关键词：

- 木质边框侧栏
- 羊皮纸内容面板
- 像素物品栏导航
- 农田/草地背景
- 日历、时间、玩家数状态条
- 存档木牌
- Mod 仓库物品格
- 终端作为公告栏/木牌日志

核心色板：

- Sky: `#8fd1ef`
- Grass: `#5fab46`
- Soil: `#8f5a2d`
- Wood: `#9c5a2c`
- Dark Wood: `#673519`
- Paper: `#ffd995`
- Paper Light: `#ffe6b6`
- Ink: `#3c2415`
- Success Green: `#3f8f3d`
- Warning Gold: `#f4b23c`

## Prototype Files

- `stardew-anxi-panel-product-prototype.html`: 产品交互原型海报，集中展示首次打开面板、主界面安装入口、Steam Auth 循环、启动前选存档、状态/指令/喊话、存档/Mod/用户管理等关键状态。
- `stardew-anxi-panel-product-prototype.png`: 与 HTML 对应的静态产品原型图，适合文档预览和给后续模型快速理解视觉方向。

## V2 Direction

当前版本参考了 `C:\Users\cr\Desktop\prototypes` 中的原型方向，替换掉早期“后台换像素皮”的方案。

V2 重点：

- 原型以多屏状态海报呈现，不再只画一个仪表盘页面。
- 侧栏和窗口更接近游戏内菜单，而不是现代后台布局。
- 大量使用深色木框、浅黄面板、粗边框、网格背景和小型状态标签。
- 只参考视觉，不沿用参考原型中的业务文案和交互逻辑。
- 每个模块都明确服务 `stardew-server-anxi-panel` 的真实交互场景：管理员初始化、Junimo 工作目录、Steam 凭据写入、Steam Guard、镜像拉取、存档选择、服务器启动、Mod 管理、attach-cli 指令、面板用户和审计日志。
- 控制文字长度，优先使用短标签和表格，避免溢出。
- 未来如果继续细化，可以将每个海报模块拆成可点击的真实页面原型。

## Product-Specific Flow

当前原型中的交互内容应以本项目架构为准：

- 当前第一版只有 Stardew 实例，用户登录后应直接进入 Stardew 面板；总面板游戏列表暂时隐藏。未来出现第二个游戏面板时，再开启 Multi Game Mode 并显示总面板。
- 用户拉取并运行本面板 Docker 镜像后，面板后端自动创建 Junimo 工作目录、下载/写入 `docker-compose.yml` 和 `.env`，并开放面板端口。
- 首次访问面板时展示管理员初始化注册界面。创建管理员后进入主界面。
- 主界面显示 `安装游戏` 按钮；`启动服务器` 按钮在安装完成前不可用，点击时提示 `请先安装游戏`。
- 点击安装游戏后弹出 Steam 账号、Steam 密码、VNC 密码输入框。确认后后端写入 `.env`，执行 `docker compose pull`，再执行 `docker compose run --rm -i steam-auth download` 让 Junimo 使用 `.env` 中的凭据登录并下载游戏文件；不要用普通 stdin pipe 跑 `setup` 的账号密码分支。
- 如果 Steam 密码错误，前端重新弹出修改密码界面，后端重新写入 `.env` 并再次执行 Steam Auth，直到密码正确或用户取消。
- Steam Guard 阶段前端展示邮箱验证码输入或手机 App 等待确认；用户输入会被后端写入 stdin。账号密码安装路径应走 Junimo `steam-auth download` 的非交互登录，使用 `.env` 中的 Steam 凭据，避免 `setup` 账号密码分支里的 `Console.ReadKey()` 在后台任务中崩溃。二维码登录如果在生成二维码前出现 `SteamClient instance must be connected`，前端应提示这是上游 QR 流程不稳定并引导改用账号密码 / Steam Guard。
- 安装完成后启用 `启动服务器` 按钮。点击启动后先检测服务器侧是否已有存档；没有已有存档时弹出两个入口：自定义新建存档，或从打开面板网页的本机上传存档。上传必须先进入临时解析和预览，展示游戏时间、地图、已有玩家名称等信息，确认后才上传到服务器并启动。自定义新建存档由面板收集农场名、玩家名、地图类型和初始设置，并生成可被 Stardew/Junimo 读取的真实初始存档；不要假设上游 Junimo 支持完整自定义创建。
- 用户确认存档策略后，后端执行 `docker compose up -d`，随后自动执行 `docker compose exec server attach-cli` 并发送 `invitecode` 或 `info`，把邀请码展示到前端。
- 日常页面包括状态页、指令/喊话页、存档页、Mod 页、用户管理页。所有 Stardew 相关能力都通过 `games/stardew_junimo` driver 与 Junimo 容器通信。

## Implementation Notes

后续正式前端实现时建议：

- React 组件结构仍按业务页面拆分，不要把所有视觉写死在一个大页面里。
- 建立设计 token：颜色、边框、阴影、像素格尺寸、按钮状态。
- 组件优先封装：`WoodFrame`、`PaperPanel`、`InventorySlot`、`FarmStatusBadge`、`QuestStep`、`SaveCard`、`ModTile`、`TerminalBoard`。
- 控制台区域使用 xterm.js，但外层视觉包装成“公告栏”。
- 安装 Steam Guard 弹窗要支持二维码、邮箱验证码、手机确认和错误重试。
- 所有游戏相关页面仍通过 `GameDriver` 的 API 间接调用 Junimo 容器能力。
- 前端建议预留 `frontend/src/core`、`frontend/src/games/stardew`、`frontend/src/games/minecraft` 等结构。当前 Stardew 原型只落到 `games/stardew`，总面板和其他游戏模块后续再扩展。
