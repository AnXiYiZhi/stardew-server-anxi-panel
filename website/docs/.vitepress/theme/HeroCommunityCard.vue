<script setup lang="ts">
import { QQ_GROUP_JOIN_URL } from './community'
</script>

<template>
  <a
    class="hero-community-card"
    :href="QQ_GROUP_JOIN_URL"
    target="_blank"
    rel="noopener noreferrer"
    aria-label="加入 Anxi Panel QQ 交流群沟通反馈（在新窗口打开）"
  >
    <span class="hero-community-card__icon" aria-hidden="true">
      <svg viewBox="0 0 24 24" role="presentation">
        <path d="M7 18.5 3.5 21v-4.8A8.1 8.1 0 0 1 2 11.5C2 6.8 6.5 3 12 3s10 3.8 10 8.5S17.5 20 12 20a11.6 11.6 0 0 1-5-.9" />
        <path d="M7.7 11.7h.01M12 11.7h.01M16.3 11.7h.01" />
      </svg>
    </span>

    <span class="hero-community-card__copy">
      <strong>有问题或建议？</strong>
      <small>部署求助 · 反馈建议</small>
    </span>

    <span class="hero-community-card__action" aria-hidden="true">
      <span>加入交流群</span>
      <svg viewBox="0 0 20 20" role="presentation">
        <path d="m7 4 6 6-6 6" />
      </svg>
    </span>
  </a>
</template>

<style scoped>
.hero-community-card {
  --community-accent: #16745d;
  --community-ink: #18362d;
  --community-muted: #64766f;
  display: grid;
  grid-template-columns: auto minmax(0, 1fr) auto;
  align-items: center;
  gap: 13px;
  width: min(100%, 456px);
  min-height: 72px;
  margin-top: 20px;
  padding: 12px 15px;
  border: 1px solid color-mix(in srgb, var(--community-accent) 22%, transparent);
  border-radius: 16px;
  background:
    radial-gradient(circle at 9% 15%, rgb(255 255 255 / 88%), transparent 34%),
    linear-gradient(118deg, #f4fbf8 0%, #f0f7f5 58%, #edf6f8 100%);
  box-shadow: 0 10px 26px rgb(31 74 61 / 8%);
  color: var(--community-ink);
  text-decoration: none;
  touch-action: manipulation;
  transition: border-color 180ms ease, box-shadow 180ms ease, transform 180ms ease;
}

.hero-community-card__icon {
  display: grid;
  width: 44px;
  height: 44px;
  place-items: center;
  border: 1px solid color-mix(in srgb, var(--community-accent) 20%, transparent);
  border-radius: 13px;
  background: color-mix(in srgb, var(--community-accent) 10%, white);
  color: var(--community-accent);
}

.hero-community-card__icon svg {
  width: 23px;
  height: 23px;
  fill: none;
  stroke: currentColor;
  stroke-linecap: round;
  stroke-linejoin: round;
  stroke-width: 1.65;
}

.hero-community-card__copy {
  display: flex;
  align-items: flex-start;
  flex-direction: column;
  min-width: 0;
  text-align: left;
}

.hero-community-card__copy strong,
.hero-community-card__copy small {
  display: block;
  width: 100%;
  text-align: left;
}

.hero-community-card__copy strong {
  font-size: 14px;
  font-weight: 760;
  line-height: 1.35;
  letter-spacing: -0.015em;
}

.hero-community-card__copy small {
  margin-top: 4px;
  overflow: hidden;
  color: var(--community-muted);
  font-size: 11px;
  font-weight: 560;
  line-height: 1.35;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.hero-community-card__action {
  display: inline-flex;
  min-height: 44px;
  align-items: center;
  gap: 5px;
  color: var(--community-accent);
  font-size: 12px;
  font-weight: 760;
  white-space: nowrap;
}

.hero-community-card__action svg {
  width: 17px;
  height: 17px;
  fill: none;
  stroke: currentColor;
  stroke-linecap: round;
  stroke-linejoin: round;
  stroke-width: 1.8;
  transition: transform 180ms ease;
}

.hero-community-card:focus-visible {
  outline: 3px solid color-mix(in srgb, var(--community-accent) 34%, transparent);
  outline-offset: 3px;
}

:global(html.dark .hero-community-card) {
  --community-accent: #7ec8b0;
  --community-ink: #e7f1ed;
  --community-muted: #9eaea8;
  border-color: rgb(126 200 176 / 22%);
  background:
    radial-gradient(circle at 9% 15%, rgb(255 255 255 / 4%), transparent 35%),
    linear-gradient(118deg, #14241f 0%, #13211d 58%, #142126 100%);
  box-shadow: 0 12px 28px rgb(0 0 0 / 18%);
}

:global(html.dark .hero-community-card .hero-community-card__icon) {
  background: rgb(126 200 176 / 10%);
}

@media (hover: hover) and (pointer: fine) {
  .hero-community-card:hover {
    border-color: color-mix(in srgb, var(--community-accent) 42%, transparent);
    box-shadow: 0 14px 32px rgb(31 74 61 / 13%);
    transform: translateY(-2px);
  }

  .hero-community-card:hover .hero-community-card__action svg {
    transform: translateX(2px);
  }
}

@media (max-width: 639px) {
  .hero-community-card {
    width: 100%;
    min-height: 68px;
    margin-top: 18px;
    padding: 11px 13px;
    border-radius: 14px;
  }

  .hero-community-card__icon {
    width: 42px;
    height: 42px;
    border-radius: 12px;
  }
}

@media (max-width: 359px) {
  .hero-community-card {
    gap: 10px;
  }

  .hero-community-card__action > span {
    position: absolute;
    width: 1px;
    height: 1px;
    padding: 0;
    overflow: hidden;
    clip: rect(0, 0, 0, 0);
    white-space: nowrap;
    border: 0;
  }
}

@media (prefers-reduced-motion: reduce) {
  .hero-community-card,
  .hero-community-card__action svg {
    transition: none;
  }
}
</style>
