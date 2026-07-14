import { defineConfig } from 'vitepress'

export default defineConfig({
  title: 'Anxi Panel 文档',
  description: '星露谷物语专用服务器 Web 管理面板 - 部署与使用文档',
  lang: 'zh-CN',
  base: '/stardew-server-anxi-panel/',
  lastUpdated: true,
  head: [
    ['link', { rel: 'icon', href: '/stardew-server-anxi-panel/favicon.ico' }]
  ],
  themeConfig: {
    logo: '/logo.png',
    nav: [
      { text: '首页', link: '/' },
      { text: '快速上手', link: '/guide/getting-started' },
      { text: '部署指南', link: '/deploy/requirements' },
      { text: '日常维护', link: '/maintain/update' },
      { text: '深度文档', link: '/handbook/' },
      { text: '常见问题', link: '/faq/' }
    ],
    sidebar: {
      '/guide/': [
        {
          text: '新手指南',
          items: [
            { text: '快速上手', link: '/guide/getting-started' },
            { text: '服务器选择', link: '/guide/choose-server' },
            { text: '部署安装', link: '/guide/deploy' },
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
            { text: 'Mod 管理', link: '/maintain/mods' },
            { text: '面板管理与诊断', link: '/maintain/admin' }
          ]
        }
      ],
      '/handbook/': [
        {
          text: '深度文档',
          items: [
            { text: '总览', link: '/handbook/' },
            { text: '界面总览', link: '/handbook/ui' },
            { text: '账号与权限', link: '/handbook/accounts' },
            { text: '安装游戏', link: '/handbook/install' },
            { text: '服务器控制', link: '/handbook/server-control' },
            { text: '存档管理', link: '/handbook/saves' },
            { text: 'Mod 管理', link: '/handbook/mods' },
            { text: '玩家管理', link: '/handbook/players' },
            { text: '任务与日志', link: '/handbook/jobs-logs' },
            { text: '诊断与支持包', link: '/handbook/diagnostics' },
            { text: '面板设置', link: '/handbook/settings' }
          ]
        }
      ],
      '/faq/': [
        { text: '常见问题', link: '/faq/' },
        { text: '已知问题（上游）', link: '/faq/known-issues' }
      ]
    },
    search: { provider: 'local' },
    socialLinks: [
      { icon: 'github', link: 'https://github.com/AnXiYiZhi/stardew-server-anxi-panel' }
    ],
    outline: { label: '本页目录' },
    docFooter: { prev: '上一页', next: '下一页' }
  }
})
