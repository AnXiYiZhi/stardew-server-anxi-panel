<script setup lang="ts">
import { withBase } from 'vitepress'
</script>

<template>
  <aside
    class="hero-invite"
    aria-labelledby="hero-invite-title"
    aria-describedby="hero-invite-description"
  >
    <header class="hero-invite__header">
      <img
        class="hero-invite__logo"
        :src="withBase('/logo.png')"
        alt=""
        aria-hidden="true"
        width="128"
        height="128"
      >
      <div>
        <span lang="en">CO-OP SERVER PASS</span>
        <strong id="hero-invite-title">专属联机邀请</strong>
      </div>
    </header>

    <div class="hero-invite__status">
      <span class="hero-invite__status-copy">
        <i aria-hidden="true" />
        服务器启动即可 IP 直连
      </span>
      <small lang="en">READY</small>
    </div>

    <div class="hero-invite__party" aria-hidden="true">
      <span>你</span>
      <i />
      <b>
        <svg viewBox="0 0 24 24" role="presentation">
          <rect x="5" y="3.5" width="14" height="17" rx="3" />
          <path d="M8.5 8h7M8.5 12h7M9 16h.01M12 16h.01" />
        </svg>
      </b>
      <i />
      <span>朋友</span>
    </div>
    <p class="hero-invite__party-copy">一起加入同一片农场</p>

    <dl class="hero-invite__capabilities">
      <div>
        <dt>联机</dt>
        <dd>直连常驻，邀请按需</dd>
      </div>
      <div>
        <dt>管理</dt>
        <dd>存档 · Mod · 玩家</dd>
      </div>
      <div>
        <dt>部署</dt>
        <dd>Docker 自托管</dd>
      </div>
    </dl>

    <p id="hero-invite-description" class="hero-invite__footer">
      <svg viewBox="0 0 24 24" aria-hidden="true">
        <rect x="5" y="10" width="14" height="10" rx="2" />
        <path d="M8 10V7a4 4 0 0 1 8 0v3" />
      </svg>
      数据留在自己的服务器
    </p>
  </aside>
</template>

<style scoped>
.hero-invite {
  --invite-cutout: #f7faf8;
  --invite-paper-start: #fffdf7;
  --invite-paper-end: #f3ecdd;
  --invite-ink: #2d352f;
  --invite-muted: #756d5c;
  --invite-line: rgb(91 76 45 / 18%);
  --invite-accent: #116b4f;
  position: relative;
  width: 100%;
  min-height: 318px;
  padding: 25px 25px 21px;
  overflow: hidden;
  border: 1px solid rgb(91 76 45 / 17%);
  border-radius: 19px;
  background:
    radial-gradient(circle at 14% 8%, rgb(255 255 255 / 35%), transparent 29%),
    linear-gradient(145deg, var(--invite-paper-start), var(--invite-paper-end));
  box-shadow:
    0 24px 58px rgb(25 63 51 / 14%),
    inset 0 1px rgb(255 255 255 / 72%);
  color: var(--invite-ink);
  isolation: isolate;
}

.hero-invite::before,
.hero-invite::after {
  content: "";
  position: absolute;
  z-index: 2;
  top: 116px;
  width: 20px;
  height: 20px;
  border-radius: 50%;
  background: var(--invite-cutout);
  box-shadow: 0 0 0 1px rgb(91 76 45 / 10%);
}

.hero-invite::before { left: -11px; }
.hero-invite::after { right: -11px; }

.hero-invite__header {
  display: flex;
  align-items: center;
  gap: 17px;
}

.hero-invite__logo {
  width: 72px;
  height: 72px;
  flex: 0 0 72px;
  border-radius: 12px;
  box-shadow: 0 7px 18px rgb(57 39 13 / 18%);
}

.hero-invite__header div {
  min-width: 0;
}

.hero-invite__header span {
  display: block;
  margin-bottom: 7px;
  color: var(--invite-muted);
  font-size: 9px;
  font-weight: 780;
  line-height: 1;
  letter-spacing: 0.15em;
}

.hero-invite__header strong {
  display: block;
  font-size: 22px;
  font-weight: 780;
  line-height: 1.2;
  letter-spacing: -0.035em;
}

.hero-invite__status {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  margin-top: 19px;
  padding: 13px 2px;
  border-top: 1px dashed var(--invite-line);
  border-bottom: 1px dashed var(--invite-line);
}

.hero-invite__status-copy {
  display: inline-flex;
  align-items: center;
  gap: 9px;
  font-size: 13px;
  font-weight: 720;
}

.hero-invite__status-copy i {
  width: 8px;
  height: 8px;
  flex: 0 0 8px;
  border-radius: 50%;
  background: var(--invite-accent);
  box-shadow: 0 0 0 4px color-mix(in srgb, var(--invite-accent) 13%, transparent);
}

.hero-invite__status small {
  color: var(--invite-accent);
  font-size: 9px;
  font-weight: 800;
  letter-spacing: 0.16em;
}

.hero-invite__party {
  display: grid;
  grid-template-columns: 42px 1fr 38px 1fr 42px;
  align-items: center;
  gap: 7px;
  margin-top: 17px;
}

.hero-invite__party > span {
  display: grid;
  min-height: 30px;
  place-items: center;
  border: 1px solid var(--invite-line);
  border-radius: 999px;
  background: rgb(255 255 255 / 18%);
  font-size: 11px;
  font-weight: 740;
}

.hero-invite__party > i {
  height: 1px;
  background: var(--invite-line);
}

.hero-invite__party > b {
  display: grid;
  width: 38px;
  height: 38px;
  place-items: center;
  border: 1px solid color-mix(in srgb, var(--invite-accent) 42%, transparent);
  border-radius: 12px;
  background: color-mix(in srgb, var(--invite-accent) 11%, transparent);
  color: var(--invite-accent);
}

.hero-invite__party svg {
  width: 20px;
  height: 20px;
  fill: none;
  stroke: currentColor;
  stroke-linecap: round;
  stroke-linejoin: round;
  stroke-width: 1.7;
}

.hero-invite__party-copy {
  margin: 7px 0 0;
  color: var(--invite-muted);
  font-size: 10px;
  font-weight: 620;
  text-align: center;
  letter-spacing: 0.04em;
}

.hero-invite__capabilities {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 0;
  margin: 17px 0 0;
}

.hero-invite__capabilities div {
  min-width: 0;
  padding: 0 10px;
  border-left: 1px solid var(--invite-line);
  text-align: center;
}

.hero-invite__capabilities div:first-child {
  padding-left: 0;
  border-left: 0;
}

.hero-invite__capabilities div:last-child {
  padding-right: 0;
}

.hero-invite__capabilities dt {
  color: var(--invite-muted);
  font-size: 9px;
  font-weight: 760;
  line-height: 1;
  letter-spacing: 0.1em;
}

.hero-invite__capabilities dd {
  margin: 6px 0 0;
  font-size: 10px;
  font-weight: 700;
  line-height: 1.35;
  white-space: nowrap;
}

.hero-invite__footer {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
  margin: 17px 0 0;
  color: var(--invite-muted);
  font-size: 10px;
  font-weight: 620;
}

.hero-invite__footer svg {
  width: 13px;
  height: 13px;
  fill: none;
  stroke: currentColor;
  stroke-linecap: round;
  stroke-linejoin: round;
  stroke-width: 1.8;
}

:global(html.dark .hero-invite) {
  --invite-cutout: #07130f;
  --invite-paper-start: #242823;
  --invite-paper-end: #181c19;
  --invite-ink: #ece8df;
  --invite-muted: #a9aaa4;
  --invite-line: rgb(207 205 196 / 16%);
  --invite-accent: #8ead9e;
  border-color: rgb(222 225 218 / 13%);
  background:
    radial-gradient(circle at 14% 8%, rgb(226 216 197 / 6%), transparent 31%),
    linear-gradient(145deg, var(--invite-paper-start), var(--invite-paper-end));
  box-shadow:
    0 24px 56px rgb(0 0 0 / 28%),
    inset 0 1px rgb(255 255 255 / 5%);
}

:global(html.dark .hero-invite .hero-invite__party > span) {
  background: rgb(255 255 255 / 2.5%);
}

@media (max-width: 639px) {
  .hero-invite {
    min-height: 292px;
    padding: 20px 20px 18px;
    border-radius: 16px;
  }

  .hero-invite::before,
  .hero-invite::after {
    top: 99px;
    width: 16px;
    height: 16px;
  }

  .hero-invite::before { left: -9px; }
  .hero-invite::after { right: -9px; }

  .hero-invite__header {
    gap: 14px;
  }

  .hero-invite__logo {
    width: 60px;
    height: 60px;
    flex-basis: 60px;
    border-radius: 10px;
  }

  .hero-invite__header strong {
    font-size: 19px;
  }

  .hero-invite__status {
    margin-top: 17px;
    padding: 12px 2px;
  }

  .hero-invite__party {
    margin-top: 15px;
  }

  .hero-invite__capabilities {
    margin-top: 15px;
  }

  .hero-invite__capabilities div {
    padding: 0 6px;
  }

  .hero-invite__capabilities dd {
    font-size: 9px;
  }

  .hero-invite__footer {
    margin-top: 15px;
  }
}

@media (max-width: 350px) {
  .hero-invite__capabilities dd {
    white-space: normal;
  }
}
</style>
