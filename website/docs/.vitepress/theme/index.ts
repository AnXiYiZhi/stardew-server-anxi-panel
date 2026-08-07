import DefaultTheme from 'vitepress/theme'
import type { Theme } from 'vitepress'
import { useRoute } from 'vitepress'
import { nextTick, onMounted, watch } from 'vue'
import mediumZoom from 'medium-zoom'
import DocsHome from './DocsHome.vue'
import ThemeLayout from './ThemeLayout.vue'
import './custom.css'

export default {
  extends: DefaultTheme,
  Layout: ThemeLayout,
  enhanceApp({ app }) {
    app.component('DocsHome', DocsHome)
  },
  setup() {
    const route = useRoute()
    const zoom = () => {
      mediumZoom('.vp-doc img:not(.no-zoom)', {
        background: 'var(--vp-c-bg)',
        margin: 24,
      })
    }
    onMounted(zoom)
    watch(
      () => route.path,
      () => nextTick(zoom),
    )
  },
} satisfies Theme
