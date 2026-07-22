<script setup lang="ts">
import DefaultTheme from 'vitepress/theme'
import { useData, useRoute, withBase } from 'vitepress'
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'

const route = useRoute()
const { frontmatter, page, site } = useData()
const progress = ref(0)

const sections = [
  { prefix: '/guide/', key: 'guide', label: '新手指南', eyebrow: 'GETTING STARTED', icon: '✦' },
  { prefix: '/deploy/', key: 'deploy', label: '部署指南', eyebrow: 'DEPLOYMENT', icon: '⬡' },
  { prefix: '/handbook/', key: 'handbook', label: '深度文档', eyebrow: 'HANDBOOK', icon: '◫' },
  { prefix: '/maintain/', key: 'maintain', label: '日常维护', eyebrow: 'OPERATIONS', icon: '⌁' },
  { prefix: '/faq/', key: 'faq', label: '问题排查', eyebrow: 'TROUBLESHOOTING', icon: '?' },
  { prefix: '/changelog', key: 'changelog', label: '版本更新', eyebrow: 'RELEASE NOTES', icon: '↗' },
]

const isHome = computed(() => frontmatter.value.layout === 'home')
const sitePath = computed(() => {
  const base = site.value.base.replace(/\/$/, '')
  return base && route.path.startsWith(base) ? route.path.slice(base.length) || '/' : route.path
})
const section = computed(() => (
  sections.find((item) => sitePath.value.startsWith(item.prefix)) ??
  { key: 'docs', label: '文档中心', eyebrow: 'DOCUMENTATION', icon: '◇' }
))
const sectionClass = computed(() => `section-${section.value.key}`)
const headingCount = computed(() => page.value.headers?.filter((item) => item.level === 2).length ?? 0)

const updateProgress = () => {
  if (typeof document === 'undefined') return
  const scrollable = document.documentElement.scrollHeight - window.innerHeight
  progress.value = scrollable > 0 ? Math.min(100, Math.max(0, window.scrollY / scrollable * 100)) : 0
}

onMounted(() => {
  updateProgress()
  window.addEventListener('scroll', updateProgress, { passive: true })
  window.addEventListener('resize', updateProgress, { passive: true })
})

onBeforeUnmount(() => {
  window.removeEventListener('scroll', updateProgress)
  window.removeEventListener('resize', updateProgress)
})

watch(() => route.path, () => requestAnimationFrame(updateProgress))
</script>

<template>
  <DefaultTheme.Layout :class="['anxi-layout', sectionClass]">
    <template #layout-top>
      <div v-if="!isHome" class="reading-progress" aria-hidden="true">
        <span :style="{ width: `${progress}%` }" />
      </div>
    </template>

    <template #sidebar-nav-before>
      <div class="sidebar-brand">
        <span class="sidebar-brand-mark">A</span>
        <div>
          <strong>Anxi Knowledge</strong>
          <small>中文知识库</small>
        </div>
      </div>
    </template>

    <template #doc-before>
      <div class="doc-context">
        <nav class="doc-breadcrumb" aria-label="面包屑">
          <a :href="withBase('/')">文档中心</a>
          <span aria-hidden="true">/</span>
          <strong>{{ section.label }}</strong>
        </nav>
        <div class="doc-context-meta">
          <span class="doc-section-chip"><b>{{ section.icon }}</b>{{ section.eyebrow }}</span>
          <span v-if="headingCount" class="doc-section-count">{{ headingCount }} 个主题</span>
          <span class="doc-maintained"><i /> 持续维护</span>
        </div>
      </div>
    </template>

    <template #doc-after>
      <section class="doc-help-card" aria-label="文档帮助">
        <div>
          <span>NEED MORE HELP?</span>
          <strong>没有找到想要的答案？</strong>
          <p>先从常见问题按现象排查，仍未解决时再携带诊断信息反馈。</p>
        </div>
        <div class="doc-help-actions">
          <a :href="withBase('/faq/')">查看常见问题</a>
          <a class="secondary" href="https://github.com/AnXiYiZhi/stardew-server-anxi-panel/issues" target="_blank" rel="noreferrer">反馈问题 ↗</a>
        </div>
      </section>
    </template>
  </DefaultTheme.Layout>
</template>
