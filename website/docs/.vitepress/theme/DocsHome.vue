<script setup lang="ts">
import { useData, withBase } from 'vitepress'
import { computed } from 'vue'

const { frontmatter } = useData()
const release = computed(() => String(frontmatter.value.release ?? 'v0.4.8'))

const devicePaths = [
  {
    key: 'new',
    title: '我还没有服务器',
    detail: '先看配置与选择建议',
    link: '/guide/choose-server',
    featured: true,
    icon: 'new-server',
  },
  {
    key: 'linux',
    title: '我有 Linux 服务器',
    detail: '直接进入一键脚本部署',
    link: '/guide/deploy',
    icon: 'server',
  },
  {
    key: 'nas',
    title: '我用 NAS',
    detail: '按图形化 Compose 部署',
    link: '/deploy/nas',
    icon: 'nas',
  },
  {
    key: 'windows',
    title: '我在 Windows 上',
    detail: '先配置 WSL2 与 Docker Desktop',
    link: '/deploy/requirements',
    icon: 'windows',
  },
]

const steps = [
  { index: '01', title: '准备服务器', detail: '确认系统、配置与端口' },
  { index: '02', title: '部署面板', detail: '选择脚本或 NAS 方式' },
  { index: '03', title: '创建或导入存档', detail: '先有存档，再启动服务器' },
  { index: '04', title: '启动并邀请朋友', detail: '确认运行后分享邀请码或 IP' },
]

const tasks = [
  {
    title: '使用面板',
    detail: '从安装游戏到服务器控制、玩家与设置。',
    link: '/handbook/',
    icon: 'panel',
  },
  {
    title: '存档和 Mod',
    detail: '创建、导入、备份存档并管理同步包。',
    link: '/handbook/saves',
    icon: 'folder',
  },
  {
    title: '遇到问题',
    detail: '按现象查原因，必要时导出诊断信息。',
    link: '/faq/',
    icon: 'warning',
  },
]
</script>

<template>
  <main class="docs-home" aria-labelledby="docs-home-title">
    <section class="docs-home-hero">
      <div class="docs-home-heading">
        <h1 id="docs-home-title">第一次开服，从你现在的设备开始</h1>
        <p>选择你的情况，只看接下来真正需要做的步骤。</p>
      </div>

      <nav class="device-paths" aria-label="按设备选择开服路径">
        <a
          v-for="item in devicePaths"
          :key="item.key"
          :class="['device-path', { 'is-featured': item.featured }]"
          :href="withBase(item.link)"
        >
          <span class="device-path-icon" aria-hidden="true">
            <svg v-if="item.icon === 'server'" viewBox="0 0 48 48"><rect x="8" y="7" width="32" height="9" rx="3"/><rect x="8" y="20" width="32" height="9" rx="3"/><rect x="8" y="33" width="32" height="9" rx="3"/><circle cx="34" cy="11.5" r="1.5"/><circle cx="34" cy="24.5" r="1.5"/><circle cx="34" cy="37.5" r="1.5"/></svg>
            <svg v-else-if="item.icon === 'nas'" viewBox="0 0 48 48"><path d="M9 12 13 7h22l4 5v28H9z"/><rect x="14" y="15" width="7" height="20" rx="2"/><rect x="27" y="15" width="7" height="20" rx="2"/><circle cx="17.5" cy="30.5" r="1"/><circle cx="30.5" cy="30.5" r="1"/></svg>
            <svg v-else-if="item.icon === 'windows'" viewBox="0 0 48 48"><path d="m7 10 15-2v15H7zm18-2 16-2v17H25zM7 26h15v15L7 39zm18 0h16v17l-16-2z"/></svg>
            <svg v-else viewBox="0 0 48 48"><rect x="6" y="8" width="27" height="10" rx="3"/><rect x="6" y="23" width="27" height="10" rx="3"/><circle cx="28" cy="13" r="1.25"/><circle cx="28" cy="28" r="1.25"/><circle cx="37" cy="34" r="7"/><path d="M37 30v8M33 34h8"/></svg>
          </span>
          <strong>{{ item.title }}</strong>
          <small>{{ item.detail }}</small>
          <span class="device-path-arrow" aria-hidden="true">
            <svg viewBox="0 0 24 24"><path d="m9 5 7 7-7 7"/></svg>
          </span>
        </a>
      </nav>

      <a class="installed-link" :href="withBase('/handbook/')">
        已经安装？进入面板使用手册
        <svg aria-hidden="true" viewBox="0 0 24 24"><path d="m9 5 7 7-7 7"/></svg>
      </a>
    </section>

    <section class="opening-sequence" aria-labelledby="opening-sequence-title">
      <div class="sequence-heading">
        <h2 id="opening-sequence-title">真正的开服顺序</h2>
        <p>先准备存档，再启动服务器；这样不会卡在“需要选择存档”。</p>
      </div>
      <ol>
        <li v-for="step in steps" :key="step.index">
          <span>{{ step.index }}</span>
          <div>
            <strong>{{ step.title }}</strong>
            <small>{{ step.detail }}</small>
          </div>
        </li>
      </ol>
    </section>

    <section class="home-task-rail" aria-labelledby="home-task-title">
      <div class="task-rail-heading">
        <h2 id="home-task-title">接下来你可能需要</h2>
        <p>按你要完成的事情找文档，不必记住栏目名称。</p>
      </div>
      <div class="task-links">
        <a v-for="task in tasks" :key="task.title" :href="withBase(task.link)">
          <span class="task-icon" aria-hidden="true">
            <svg v-if="task.icon === 'panel'" viewBox="0 0 32 32"><rect x="4" y="5" width="24" height="22" rx="4"/><path d="M4 11h24M10 16h5v5h-5zm9 0h5v2h-5zm0 4h5v2h-5z"/></svg>
            <svg v-else-if="task.icon === 'folder'" viewBox="0 0 32 32"><path d="M3 9h10l3 3h13v13a3 3 0 0 1-3 3H6a3 3 0 0 1-3-3z"/><path d="M3 9V7a3 3 0 0 1 3-3h7l3 5"/></svg>
            <svg v-else viewBox="0 0 32 32"><path d="M16 4 29 27H3z"/><path d="M16 11v8"/><circle cx="16" cy="23" r="1"/></svg>
          </span>
          <span>
            <strong>{{ task.title }}</strong>
            <small>{{ task.detail }}</small>
          </span>
          <svg class="task-arrow" aria-hidden="true" viewBox="0 0 24 24"><path d="m9 5 7 7-7 7"/></svg>
        </a>
      </div>
    </section>

    <footer class="home-release">
      <div>
        <span>当前文档对应</span>
        <strong>{{ release }}</strong>
        <p>查看新增能力、兼容性变化和升级前注意事项。</p>
      </div>
      <a :href="withBase('/changelog')">查看更新日志 <span aria-hidden="true">→</span></a>
    </footer>
  </main>
</template>
