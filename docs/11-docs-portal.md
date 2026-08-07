# 文档门户网站建设方案

本文档规划 `stardew-server-anxi-panel` 的公开文档门户网站：面向普通终端用户（部署/使用面板的人），风格对标 [Miracle SDV 文档站](https://docs.miraclesses.top/quick-start/install.html) 和 [JunimoServer 文档站](https://stardew-valley-dedicated-server.github.io/server/admins/quick-start/installation.html)（两者均为 VitePress 构建）。

状态：**步骤 1-8 全部完成并已上线；任务型首页、五项导航、按设备开服路径、FAQ/手册重排与 v0.4.8 玩家 Mod 展示已合并到 `main`，等待本次 Pages 部署后的最终线上复核**。以下决策已和用户对齐：

## 2026-08-07：任务型门户与 v0.4.8 展示合并

- 用户明确要求把所有历史分支/worktree 的有效内容收敛到 `main`，因此 2026-07-29 隔离评审的 `docs-portal-redesign` 获得发布授权；不再保留“用户确认前不得提交”的旧草稿约束。
- 首页改用代码原生 `DocsHome.vue`，桌面按“我还没有服务器 / Linux / NAS / Windows”四宫格、手机同序单列；导航收敛为开始开服、使用面板、存档和 Mod、排障、参考。国内 HTTP 与 GitHub HTTPS 两条安装命令继续保留。
- FAQ 改为按现象折叠，手册按任务重排，并纠正首次存档顺序、临时封禁、模组农场、Docker Socket 与停服修改 Mod 的事实。首页版本、changelog 和玩家手册同步 v0.4.8 的玩家 Mod 查看、自报边界及 CJB 只读提示。
- 影响只在 `website/` 与长期文档，不改变 Panel API 或玩家流程。Linux Node 24 全新依赖卷 production build 已通过；Pages workflow、线上 HTTP、桌面/手机布局、深色模式和 console 证据将在本次 `main` 提交部署后补记。

## 2026-08-01：首页联机邀请 Hero（已发布）

- Hero 使用默认主题 `home-hero-info-before` / `home-hero-image` slot，在左侧产品主张之外增加右侧“专属联机邀请”票；票面直接说明部署后联机、邀请码、存档/Mod/玩家管理、Docker 自托管和数据自持。首页用 `heroInviteCard: true` 显式启用，其他页面不受影响。
- 视觉从整块实色背景收敛为开放留白和邀请卡周围的局部静态网格。浅色主 CTA 为森林炭黑；暗色邀请卡为暖石墨，灰鼠尾草只用于状态点/图标。没有恢复持续 blur/filter、毛玻璃卡片或放大的 128px Logo。
- `.VPHomeFeatures` 从 `contain: layout paint style` 改为 `contain: layout style`，修复入口卡 hover 上移 4px 时顶部被裁切；正文 `.VPHomeContent` 继续 paint containment。Docker Desktop Node 20 build、应用内 Browser 浅/深主题和隔离 Chrome 的真实 hover/390px 响应式均已通过，console error/warn 为空。
- 影响 `website/docs/index.md` 与 `.vitepress/theme/{ThemeLayout.vue,HeroInviteCard.vue,custom.css}`；不改变公开路由、六张入口、`v0.4.6` 角标或 Panel 功能。Pages workflow `30659672364` 的 build/deploy 均成功，deployment `5697130212` 状态 `success` 并绑定提交 `81b5716`。线上 cache-bust 复核桌面浅/深主题、邀请卡、六入口和 console；隔离 Chrome 验证 1280px hover 越界仍可绘制、390×844 首页无溢出、768×1024 正文菜单可打开且无溢出。

## 2026-07-29：长目录 active 项自动跟随

- 版本日志目录因自身 `overflow-y:auto`，正文滚到历史版本后只移动 active marker，目录仍停在原位置。主题现观察 `.outline-link.active` 变化，仅在 active 离开中部舒适区时调整目录 `scrollTop`，使当前版本持续可见并保留前后上下文。
- 自动跟随尊重 reduced-motion；路由切换重连 observer，resize 重新校正，卸载清理资源。active 未变化时不干预目录自身滚动；短目录和移动端隐藏目录不执行。
- 本地 build 通过。应用内 Browser 在 1440×900 验证 v0.2.9、v0.1.14、反向滚动、锚点点击和跨页面返回；390×844 验证无横向溢出、overlay 或 console error/warn。发布后需在线复核同一长目录路径。
- Pages 工作流 `30423428794` 已成功。线上 cache-bust 页面滚轮向下至 v0.1.9、反向至 v0.1.13，目录 `scrollTop=496` 且 active 始终可见；页面身份、非空、overlay 和 console error/warn 均通过。

## 2026-07-28：v0.4.5 SMAPI 加速与升级验证展示

- 首页 frontmatter、版本更新卡、CURRENT RELEASE 和 changelog 切换到 `v0.4.5`；首页只保留用户能理解的“受审加速、续传、安全回退、官方兜底”，详细候选顺序和校验边界放在安装手册与版本日志。
- `handbook/install.md`、`faq/index.md` 增加旧版 SMAPI 整包两分钟超时的症状和升级后“重新安装 / 修复”恢复路线；`maintain/update.md` 同步三个代表性老版本的一键升级验证，明确旁路游戏容器未重建。
- 版本角标继续由 `ThemeLayout.vue` 读取首页 `release`，没有修改主题样式。发布验收覆盖首页 → changelog、FAQ → 安装手册 SMAPI 锚点，以及桌面/390px 的版本与长链接布局。
- 本地 `npm.cmd --prefix website run docs:build` 已通过。应用内 Browser 桌面真实点击两条导航成功；首页、changelog、FAQ、安装手册和更新页在 390×844 下内容存在、无横向溢出、无 framework overlay，console error/warn 为 0。
- Pages 工作流 `30372623636` 已成功。线上首页、changelog、FAQ、安装手册和更新页使用 cache-bust 请求逐页复核为 HTTP 200；首页/日志为 `v0.4.5`，其余三页均命中新增 SMAPI 与升级说明。

## 2026-07-28：v0.4.4 游戏日回档连续性修复展示

- 首页 frontmatter、更新卡、CURRENT RELEASE 与 changelog 已切换到 `v0.4.4`，说明后台即时消费保存事件以及旧版本缺失历史日不可重建的边界。
- `handbook/saves.md`、`maintain/saves-backup.md` 已删除旧 `latest/scheduled/daily` 现实时间策略，改为当前按游戏日创建回档点、默认保留最近 5 日（可设 1–14）和保护备份隔离语义。
- 版本角标仍由 `ThemeLayout.vue` 注入 `--home-release-label`；没有修改 `custom.css` 或建立第二份版本来源。发布验收需覆盖首页 → changelog，以及维护/手册两份存档页面的桌面和手机渲染。
- 本地 `npm run docs:build` 已通过。应用内 Browser 在 1440×900 验证首页唯一 `v0.4.4` 链接进入 changelog、手册索引进入存档管理、日常维护进入存档与备份；390×844 验证存档手册无横向溢出。页面身份、非空、framework overlay、console error/warn 均通过。Pages 工作流 `30293213908` 成功后，线上首页和存档手册再次确认已展示 `v0.4.4`、后台即时处理与历史缺失不可补齐边界。

首页自定义主题约定：全站使用墨绿/薄荷/暖金语义变量；顶部“快速上手”导航使用固定 30px 胶囊；Hero 使用开放式文字 + 联机邀请票，局部静态网格只围绕邀请卡。入口区固定为 6 张无序号的三列卡；“版本更新日志”通过 `/changelog` 链接定位并使用暖色版本角标。入口卡后直接承接当前版本摘要，不恢复已删除的四步流程区；改版时必须同步检查浅色、深色、桌面、390px、reduced-motion 与卡片 hover 越界。

版本角标约定（2026-07-24）：暖色角标内容读取 `website/docs/index.md` frontmatter 的 `release`，禁止再在 `custom.css` 的 `content` 中写死版本。深色四步流程区使用 `--home-path-*` 独立高对比变量；局部 `p/strong/span` 选择器必须高于全站 `.vp-doc` 正文规则。

非首页主题约定：`ThemeLayout.vue` 根据剥离 `site.base` 后的路由为六个栏目提供独立语义色，并统一注入阅读进度、知识库侧栏、面包屑/栏目徽标和帮助页尾。正文 Markdown 不需要为视觉效果添加一次性 HTML；标题、步骤、列表、代码、表格、提示块、图片和翻页均由 `custom.css` 自动接管。新增顶级栏目时同步扩展 `ThemeLayout.vue` 的 `sections` 与 `section-*` CSS 变量。

性能约定：首页禁止持续 blur/filter 动画、大面积 `backdrop-filter` 卡片或覆盖整个滚动区域的固定透明层。Hero 与卡片使用静态近实色合成和 `contain`；导航栏是唯一保留的共用轻量毛玻璃。视觉验收除溢出和 console 外，需复核首页计算样式中没有持续动画及额外大面积滤镜。

线上地址：https://anxiyizhi.github.io/stardew-server-anxi-panel/（当前已发布 `v0.4.6` 文档）

| 决策项 | 结论 |
| --- | --- |
| 仓库位置 | 同仓库子目录 `website/`，随主仓库一起走 PR 流程 |
| 部署方式 | GitHub Actions 构建 + GitHub Pages 托管，免费 |
| 访问域名 | 先用 GitHub Pages 默认域名，后续可换绑自定义域名 |
| 语言范围 | 先做中文单语，预留后续接入双语的结构空间 |

## 一、技术选型

| 层 | 选择 | 理由 |
| --- | --- | --- |
| 站点生成器 | [VitePress](https://vitepress.dev/) | 两个参考站点都是 VitePress；内置搜索、深色模式、侧边栏/大纲、Markdown 编写，学习成本低，构建产物是纯静态文件 |
| 托管 | GitHub Pages（Project Pages） | 免费，和仓库同源，不需要额外账号 |
| CI/CD | GitHub Actions | push 到 `main` 且 `website/**` 有改动时自动构建部署，无需手动上传 |
| 搜索 | VitePress 内置本地搜索（`search.provider: 'local'`） | 免费，不依赖 Algolia 账号 |
| 图片点击放大 | [medium-zoom](https://github.com/francoischalifour/medium-zoom)（2026-07-08 新增） | VitePress 默认主题不带图片 lightbox；medium-zoom 零依赖、体积小，官方博客同款方案 |

`website/` 与现有 `frontend/`（面板本体前端）、`docs/`（内部维护文档）完全独立，互不影响构建流程。

### 图片点击放大实现说明

新增 `website/docs/.vitepress/theme/index.ts`：`extends: DefaultTheme`，在 `setup()` 里对 `.vp-doc img:not(.no-zoom)` 绑定 `mediumZoom()`，并 `watch(route.path)` 在切页后 `nextTick` 重新绑定（否则新页面的图片点不开）。配套 `website/docs/.vitepress/theme/custom.css` 加 `cursor: zoom-in` 提示和 overlay 层级。之前项目没有自定义主题目录，本次是从零新建。若某张图片不想被点击放大，给它加 `.no-zoom` class（Markdown 图片语法本身不支持加 class，需要写成 HTML `<img class="no-zoom" .../>`）。

## 二、信息架构

参考两个站点的导航习惯，规划如下顶级导航：

```text
首页 | 快速开始 | 部署指南 | 日常维护 | 常见问题
```

侧边栏与目录规划：

```text
website/docs/
├─ index.md                    首页（Hero + 特性卡，VitePress 默认主页布局）
├─ guide/
│  ├─ getting-started.md       快速上手：项目是什么、能做什么、三步跑起来（导航页）
│  ├─ choose-server.md         服务器选择：部署前确认 + 没有服务器先领阿里云免费试用
│  ├─ deploy.md                部署安装：一键脚本 run.sh 全流程（含真机截图）
│  └─ first-login.md           首次进面板：建管理员、装游戏、建/传存档、拿邀请码
├─ deploy/
│  ├─ requirements.md          系统要求（云服务器/NAS 最低与推荐配置）
│  ├─ quick-start.md           一键脚本部署（Linux 云服务器）
│  ├─ nas.md                   NAS 图形化 Compose 部署
│  └─ ports.md                 端口与安全组/防火墙说明
├─ handbook/                   深度文档：按面板 9 个功能页逐页精讲（2026-07-08 新增）
│  ├─ index.md                 深度文档导航页
│  ├─ ui.md                    界面总览（顶栏/导航/总览页）
│  ├─ accounts.md              账号与权限
│  ├─ install.md               安装游戏（Steam 全流程/Steam Guard/SteamCMD 兜底）
│  ├─ server-control.md        服务器控制（生命周期/邀请码/计划重启/控制台命令/喊话）
│  ├─ saves.md                 存档管理（新建/上传/自动备份策略/恢复）
│  ├─ mods.md                  Mod 管理（下载/添加/配置三个工作台，Nexus 一键安装，玩家同步包）
│  ├─ players.md               玩家管理（含明确标注待接入的踢出/封禁/白名单/权限设置）
│  ├─ jobs-logs.md             任务与日志
│  ├─ diagnostics.md           诊断与支持包
│  └─ settings.md              面板设置
├─ maintain/
│  ├─ update.md                更新/强制更新/更新脚本本身
│  ├─ saves-backup.md          存档新建/上传/备份/恢复
│  ├─ mods.md                  Mod 上传/Nexus 安装/导出
│  └─ admin.md                 面板用户与权限/日志诊断/安全维护清单（实施时新增，原方案未列）
└─ faq/
   └─ index.md                 故障排查（Steam Guard、端口不通、启动失败等）
```

## 三、内容来源映射（已完成）

不重新创作内容，而是把已有材料改写、拆分成门户页面（Markdown 搬运 + 排版微调，语气保持面向新手）：

| 门户页面 | 现有内容来源 |
| --- | --- |
| `guide/getting-started.md` | `README.md` "新手先看" + `docs/user-guide/getting-started.md` 第一、二节（后拆分为导航页，规格/部署细节移到下面两页，参考 [Miracle SDV 文档站](https://docs.miraclesses.top/quick-start/) 的"服务器选择/部署安装"拆页结构） |
| `guide/choose-server.md` | 从 `guide/getting-started.md` 拆出的"部署前确认"+ 阿里云免费试用真机截图流程 |
| `guide/deploy.md` | 从 `guide/getting-started.md` 拆出的"一键部署"run.sh 真机截图流程 |
| `guide/first-login.md` | `docs/user-guide/getting-started.md` 第四节"首次进入面板" |
| `deploy/requirements.md` | `README.md` "系统要求" |
| `deploy/quick-start.md` | `README.md` "推荐：一键启动脚本" + `docs/user-guide/getting-started.md` 第二节 |
| `deploy/nas.md` | `README.md` "NAS 图形化 Docker Compose 部署" |
| `deploy/ports.md` | `README.md` "云服务器安全组" + `docs/user-guide/getting-started.md` 第五节 |
| `maintain/update.md` | `docs/user-guide/maintenance.md` "更新面板" |
| `maintain/saves-backup.md` | `docs/user-guide/maintenance.md` "存档备份"、"计划重启" |
| `maintain/mods.md` | `docs/user-guide/maintenance.md` "Mod 管理" |
| `maintain/admin.md` | `docs/user-guide/maintenance.md` "面板用户与权限"、"日志与诊断"、"安全维护清单"（原方案的 `maintain/*.md` 三页装不下这些内容，实施时新增第四页，并同步补了 `config.ts` 的 sidebar） |
| `faq/index.md` | `docs/user-guide/troubleshooting.md` 全文 |
| `handbook/*.md`（11 页） | 新创作，不是搬运；来源是直接阅读 `frontend/src/games/stardew/` 各页面组件源码（`StardewPanel.tsx` 导航、`ModsPage.tsx` 三个工作台、`ServerControlPage.tsx` 控制台命令 allowlist、`PlayersPage.tsx`、`SettingsPage.tsx`、`NewGameCreator.tsx`）+ `backend/internal/games/stardew_junimo/console.go` 命令定义 + `docs/02-backend.md`、`docs/03-frontend.md` 接手记录，确保和当前代码行为一致。玩家页踢出/封禁/白名单/权限设置在源码里明确标注"待接入"，文档已如实标注，不夸大功能完成度。 |

已用 `npm run docs:build` 验证过全部页面互相链接无死链、构建通过。

`docs/user-guide/` 三份文档定位不变：继续作为仓库内 Markdown 速查（GitHub 网页直接可读）。门户网站是面向公网用户的正式入口，内容更完整、图文更友好；后续任一侧更新，另一侧应同步核对，避免两处描述不一致（尤其是端口号、脚本地址、系统要求这类会变的数值）。

## 四、准备工作清单

- [x] 本机已安装 Node.js 20+（`node -v` 确认，实测 v22.22.2）
- [x] 对 `AnXiYiZhi/stardew-server-anxi-panel` 仓库有 push 权限
- [x] 对该仓库 Settings 有管理员权限（用于开启 Pages，实测用 `gh api` 直接开通，未走网页操作）
- [ ] （可选，换自定义域名时才需要）一个你能配置 DNS 的域名

## 五、实施步骤

### 步骤 1：本地脚手架 VitePress 项目（已完成）

`npm create vitepress@latest` 实测会解析到一个同名但无关的第三方包 `create-vitepress@0.0.6`（作者 choysen，非 VitePress 官方，生成的是过时的 `1.0.0-alpha.28`），**不要使用**。改为手动搭建骨架，效果等价于官方 `vitepress init` 向导：

```bash
cd e:/stardew-server-anxi-panel
mkdir website && cd website
npm init -y
npm pkg set type=module
npm install -D vitepress
```

然后手写 `website/package.json` 的 `scripts` 字段（`docs:dev` / `docs:build` / `docs:preview`，见步骤 2 之后的说明），并手动创建 `website/docs/.vitepress/config.ts`（步骤 2）和 `website/docs/index.md` 占位首页，而不是依赖交互式向导（该向导在非 TTY 环境下不可靠）。

已验证：`npm run docs:build` 构建成功，产物在 `website/docs/.vitepress/dist/`（注意是 `docs/` 子目录下的 `.vitepress`，不是 `website/.vitepress`，步骤 5 的 workflow 路径已同步修正）。

### 步骤 2：配置 `website/docs/.vitepress/config.ts`

关键点：`base` 必须设为仓库名（GitHub Pages 的 Project Pages 会挂在 `/仓库名/` 子路径下，漏配这一项是最常见的资源 404 坑）。

```ts
import { defineConfig } from 'vitepress'

export default defineConfig({
  title: 'Anxi Panel 文档',
  description: '星露谷物语专用服务器 Web 管理面板 - 部署与使用文档',
  lang: 'zh-CN',
  base: '/stardew-server-anxi-panel/',
  lastUpdated: true,
  themeConfig: {
    nav: [
      { text: '首页', link: '/' },
      { text: '快速开始', link: '/guide/getting-started' },
      { text: '部署指南', link: '/deploy/requirements' },
      { text: '日常维护', link: '/maintain/update' },
      { text: '常见问题', link: '/faq/' }
    ],
    sidebar: {
      '/guide/': [
        {
          text: '新手指南',
          items: [
            { text: '快速开始', link: '/guide/getting-started' },
            { text: '首次进入面板', link: '/guide/first-login' }
          ]
        }
      ],
      '/deploy/': [
        {
          text: '部署',
          items: [
            { text: '系统要求', link: '/deploy/requirements' },
            { text: '一键脚本部署', link: '/deploy/quick-start' },
            { text: 'NAS 图形化部署', link: '/deploy/nas' },
            { text: '端口与安全组', link: '/deploy/ports' }
          ]
        }
      ],
      '/maintain/': [
        {
          text: '日常维护',
          items: [
            { text: '更新面板', link: '/maintain/update' },
            { text: '存档与备份', link: '/maintain/saves-backup' },
            { text: 'Mod 管理', link: '/maintain/mods' }
          ]
        }
      ],
      '/faq/': [{ text: '常见问题', link: '/faq/' }]
    },
    search: { provider: 'local' },
    socialLinks: [
      { icon: 'github', link: 'https://github.com/AnXiYiZhi/stardew-server-anxi-panel' }
    ],
    outline: { label: '本页目录' },
    docFooter: { prev: '上一页', next: '下一页' }
  }
})
```

### 步骤 3：搭建目录并迁移内容（已完成）

按第三节的映射表，在 `website/docs/` 下创建了 10 个内容页（含新增的 `maintain/admin.md`），把 README 和 `docs/user-guide/` 里的对应段落改写进去。`npm run docs:build` 验证全部页面构建通过、内部链接无死链。

### 步骤 4：本地预览

```bash
cd website
npm run docs:dev
```

打开命令行输出的地址（通常是 `http://localhost:5173`）逐页检查导航、侧边栏和链接是否正确。

已验证：`npm run docs:dev` 正常启动，实际访问地址是 `http://localhost:5173/stardew-server-anxi-panel/`（带 `base` 子路径），首页 `curl` 返回 200。

### 步骤 5：新增 GitHub Actions 部署工作流（已完成）

新建 `.github/workflows/docs.yml`：

```yaml
name: Deploy docs portal

on:
  push:
    branches: [main]
    paths:
      - 'website/**'
      - '.github/workflows/docs.yml'
  workflow_dispatch:

permissions:
  contents: read
  pages: write
  id-token: write

concurrency:
  group: pages
  cancel-in-progress: false

jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-node@v4
        with:
          node-version: 20
      - uses: actions/configure-pages@v5
      - name: Install dependencies
        run: cd website && npm ci
      - name: Build with VitePress
        run: cd website && npm run docs:build
      - uses: actions/upload-pages-artifact@v3
        with:
          path: website/docs/.vitepress/dist

  deploy:
    needs: build
    runs-on: ubuntu-latest
    environment:
      name: github-pages
      url: ${{ steps.deployment.outputs.page_url }}
    steps:
      - id: deployment
        uses: actions/deploy-pages@v4
```

`paths` 过滤确保改动只涉及面板本体代码（`backend/`、`frontend/`）时不会触发文档站重新部署。

### 步骤 6：仓库开启 GitHub Pages（已完成）

网页操作路径：打开仓库 `Settings` → `Pages` → `Build and deployment` → `Source` 选择 **GitHub Actions**（不要选 "Deploy from a branch"）。

实测发现这一步其实可以用命令代替，不需要网页操作：

```bash
gh api -X POST repos/AnXiYiZhi/stardew-server-anxi-panel/pages -f build_type=workflow
```

返回 `"build_type":"workflow"` 即代表开启成功。

### 步骤 7：提交并推送（已完成）

```bash
git add website .github/workflows/docs.yml docs/01-project-overview.md docs/11-docs-portal.md
git commit -m "docs: 新增文档门户网站（VitePress + GitHub Pages）"
git push
```

推送后打开仓库的 `Actions` 标签，确认 `Deploy docs portal` workflow 跑绿。首次运行成功后访问：

```text
https://anxiyizhi.github.io/stardew-server-anxi-panel/
```

（GitHub Pages 域名大小写不敏感，用户名部分习惯写小写。）

### 步骤 8（可选，后续需要时再做）：换绑自定义域名

1. 在 `website/public/CNAME` 写入你的域名，例如：

```text
docs.anxinas.dpdns.org
```

2. 在该域名的 DNS 服务商处添加一条 `CNAME` 记录，指向 `anxiyizhi.github.io`。
3. 回到仓库 `Settings` → `Pages`，`Custom domain` 填入同一域名，等待 DNS 校验通过后勾选 `Enforce HTTPS`。
4. `config.ts` 的 `base` 改回 `/`（自定义域名部署在根路径，不再需要仓库名子路径）。

## 六、维护规则

- 门户网站页面改动走和代码一样的 PR 流程，`website/**` 有改动会自动触发重新部署，不需要手动操作。
- 端口号、脚本下载地址、系统要求等会变的数值，如果同时出现在 `README.md`、`docs/user-guide/`、`website/docs/` 三处，改动时需要一起核对更新，避免用户在门户网站和仓库 README 上看到不一致的信息。
- 新增门户页面时，同步更新本文件第二节的目录规划和 `config.ts` 的 `sidebar`。

## 七、2026-07-29 未发布首页草稿撤回

- 用户否决本地重构草稿后，门户源码已恢复到当前线上基线；`theme/DocsHome.vue` 与 `theme/calm-docs.css` 已删除，没有触发 Pages 发布。
- 以后若再次探索首页方向，应从已上线版本建立独立预览；用户确认前不得覆盖正式源码、提交或发布。
- 产品页面只能引用仓库中有明确运行时或文档用途的素材。清理前同时检查字面量引用和动态模板路径；`NewGameCreator.tsx` 的宠物品种图属于动态引用，不能按“文件名无搜索结果”误删。
- 代理生成图、临时站点和 Browser QA 截图只属于一次性输出；方向撤回后应清理，不得复制进产品素材目录或在后续设计中反复复用。
