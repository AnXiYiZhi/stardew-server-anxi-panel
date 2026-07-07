# 文档门户网站建设方案

本文档规划 `stardew-server-anxi-panel` 的公开文档门户网站：面向普通终端用户（部署/使用面板的人），风格对标 [Miracle SDV 文档站](https://docs.miraclesses.top/quick-start/install.html) 和 [JunimoServer 文档站](https://stardew-valley-dedicated-server.github.io/server/admins/quick-start/installation.html)（两者均为 VitePress 构建）。

状态：**步骤 1-2、4-7 已完成，门户网站骨架已上线；步骤 3（内容迁移）待实施**。以下决策已和用户对齐：

线上地址：https://anxiyizhi.github.io/stardew-server-anxi-panel/（目前只有占位首页，尚无实际内容页面）

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

`website/` 与现有 `frontend/`（面板本体前端）、`docs/`（内部维护文档）完全独立，互不影响构建流程。

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
│  ├─ getting-started.md       项目是什么、能做什么、准备什么
│  └─ first-login.md           首次进面板：建管理员、装游戏、建/传存档、拿邀请码
├─ deploy/
│  ├─ requirements.md          系统要求（云服务器/NAS 最低与推荐配置）
│  ├─ quick-start.md           一键脚本部署（Linux 云服务器）
│  ├─ nas.md                   NAS 图形化 Compose 部署
│  └─ ports.md                 端口与安全组/防火墙说明
├─ maintain/
│  ├─ update.md                更新/强制更新/更新脚本本身
│  ├─ saves-backup.md          存档新建/上传/备份/恢复
│  └─ mods.md                  Mod 上传/Nexus 安装/导出
└─ faq/
   └─ index.md                 故障排查（Steam Guard、端口不通、启动失败等）
```

## 三、内容来源映射

不重新创作内容，而是把已有材料改写、拆分成门户页面（Markdown 搬运 + 排版微调，语气保持面向新手）：

| 门户页面 | 现有内容来源 |
| --- | --- |
| `guide/getting-started.md` | `README.md` "新手先看" + `docs/user-guide/getting-started.md` 第一、二节 |
| `guide/first-login.md` | `docs/user-guide/getting-started.md` 第四节"首次进入面板" |
| `deploy/requirements.md` | `README.md` "系统要求" |
| `deploy/quick-start.md` | `README.md` "推荐：一键启动脚本" + `docs/user-guide/getting-started.md` 第二节 |
| `deploy/nas.md` | `README.md` "NAS 图形化 Docker Compose 部署" |
| `deploy/ports.md` | `README.md` "云服务器安全组" + `docs/user-guide/getting-started.md` 第五节 |
| `maintain/*.md` | `docs/user-guide/maintenance.md` 按主题拆分 |
| `faq/index.md` | `docs/user-guide/troubleshooting.md` |

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

### 步骤 3：搭建目录并迁移内容

按第三节的映射表，在 `website/docs/`（或脚手架生成的默认内容目录，需与 `config.ts` 的 `srcDir` 对应）下创建对应 `.md` 文件，把 README 和 `docs/user-guide/` 里的对应段落改写进去。这一步是纯内容搬运工作，建议先把骨架（步骤 1-2、6-9）跑通、确认部署链路没问题后再逐页填内容，避免在还没验证部署成功前投入大量写作时间。

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
