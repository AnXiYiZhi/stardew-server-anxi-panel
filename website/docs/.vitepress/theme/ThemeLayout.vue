<script setup lang="ts">
import DefaultTheme from 'vitepress/theme'
import { useData, useRoute, withBase } from 'vitepress'
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import HeroCommunityCard from './HeroCommunityCard.vue'
import HeroInviteCard from './HeroInviteCard.vue'
import { QQ_GROUP_JOIN_URL } from './community'

const route = useRoute()
const { frontmatter, page, site } = useData()
const progress = ref(0)
let outlineObserver: MutationObserver | undefined
let outlineFrame: number | undefined

const sections = [
  { prefix: '/guide/', key: 'guide', label: '新手指南', eyebrow: 'GETTING STARTED', icon: '✦' },
  { prefix: '/deploy/', key: 'deploy', label: '部署指南', eyebrow: 'DEPLOYMENT', icon: '⬡' },
  { prefix: '/handbook/', key: 'handbook', label: '深度文档', eyebrow: 'HANDBOOK', icon: '◫' },
  { prefix: '/maintain/', key: 'maintain', label: '日常维护', eyebrow: 'OPERATIONS', icon: '⌁' },
  { prefix: '/faq/', key: 'faq', label: '问题排查', eyebrow: 'TROUBLESHOOTING', icon: '?' },
  { prefix: '/changelog', key: 'changelog', label: '版本更新', eyebrow: 'RELEASE NOTES', icon: '↗' },
]

const isHome = computed(() => frontmatter.value.layout === 'home')
const showInviteHero = computed(() => isHome.value && frontmatter.value.heroInviteCard === true)
const showCommunityHero = computed(() => isHome.value && frontmatter.value.heroCommunityCard === true)
const releaseLabel = computed(() => JSON.stringify(String(frontmatter.value.release ?? '')))
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

const keepActiveOutlineLinkVisible = (behavior: ScrollBehavior = 'smooth') => {
  if (typeof document === 'undefined') return
  if (outlineFrame !== undefined) window.cancelAnimationFrame(outlineFrame)

  outlineFrame = window.requestAnimationFrame(() => {
    outlineFrame = undefined
    const outline = document.querySelector<HTMLElement>('.VPDocAsideOutline')
    const activeLink = outline?.querySelector<HTMLElement>('.outline-link.active')
    if (!outline || !activeLink || outline.clientHeight <= 0 || outline.scrollHeight <= outline.clientHeight) return

    const outlineRect = outline.getBoundingClientRect()
    const activeRect = activeLink.getBoundingClientRect()
    const comfortTop = outlineRect.top + outline.clientHeight * 0.28
    const comfortBottom = outlineRect.bottom - outline.clientHeight * 0.28
    if (activeRect.top >= comfortTop && activeRect.bottom <= comfortBottom) return

    const target = outline.scrollTop
      + activeRect.top
      - outlineRect.top
      - (outline.clientHeight - activeRect.height) * 0.42
    const maxScroll = outline.scrollHeight - outline.clientHeight
    const reducedMotion = window.matchMedia('(prefers-reduced-motion: reduce)').matches
    outline.scrollTo({
      top: Math.min(maxScroll, Math.max(0, target)),
      behavior: reducedMotion ? 'auto' : behavior,
    })
  })
}

const observeActiveOutlineLink = () => {
  outlineObserver?.disconnect()
  outlineObserver = undefined

  nextTick(() => {
    const outline = document.querySelector<HTMLElement>('.VPDocAsideOutline')
    if (!outline) return

    outlineObserver = new MutationObserver((mutations) => {
      const activeLinkChanged = mutations.some((mutation) => (
        mutation.type === 'attributes'
        && mutation.target instanceof HTMLElement
        && mutation.target.classList.contains('outline-link')
      ))
      if (activeLinkChanged) keepActiveOutlineLinkVisible()
    })
    outlineObserver.observe(outline, {
      attributes: true,
      attributeFilter: ['class'],
      subtree: true,
    })
    keepActiveOutlineLinkVisible('auto')
  })
}

const updateViewportState = () => {
  updateProgress()
  keepActiveOutlineLinkVisible('auto')
}

onMounted(() => {
  updateViewportState()
  observeActiveOutlineLink()
  window.addEventListener('scroll', updateProgress, { passive: true })
  window.addEventListener('resize', updateViewportState, { passive: true })
})

onBeforeUnmount(() => {
  window.removeEventListener('scroll', updateProgress)
  window.removeEventListener('resize', updateViewportState)
  outlineObserver?.disconnect()
  if (outlineFrame !== undefined) window.cancelAnimationFrame(outlineFrame)
})

watch(() => route.path, () => {
  requestAnimationFrame(updateViewportState)
  observeActiveOutlineLink()
})
</script>

<template>
  <DefaultTheme.Layout
    :class="['anxi-layout', sectionClass]"
    :style="{ '--home-release-label': releaseLabel }"
  >
    <template #layout-top>
      <div v-if="!isHome" class="reading-progress" aria-hidden="true">
        <span :style="{ width: `${progress}%` }" />
      </div>
    </template>

    <template v-if="showInviteHero" #home-hero-info-before>
      <div class="home-hero-kicker">
        <strong>Anxi Panel</strong>
        <span aria-hidden="true" />
        <small>开源自托管 · 中文优先</small>
      </div>
    </template>

    <template v-if="showInviteHero" #home-hero-image>
      <HeroInviteCard />
    </template>

    <template v-if="showCommunityHero" #home-hero-actions-after>
      <HeroCommunityCard />
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
          <a class="secondary" :href="QQ_GROUP_JOIN_URL" target="_blank" rel="noopener noreferrer">加群反馈 ↗</a>
        </div>
      </section>
    </template>
  </DefaultTheme.Layout>
</template>
