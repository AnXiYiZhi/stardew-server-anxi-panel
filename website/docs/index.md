---
layout: home
release: v0.4.2

hero:
  name: Anxi Panel
  text: 把开服这件事，变得像打开网页一样简单
  tagline: 为星露谷物语专服打造的中文管理面板。部署、存档、Mod 与日常维护，一处完成。
  image:
    src: /logo.png
    alt: Anxi Panel
  actions:
    - theme: brand
      text: 快速上手
      link: /guide/getting-started
    - theme: alt
      text: 浏览完整手册
      link: /handbook/

features:
  - icon: '01'
    title: 快速上手
    details: 从服务器选择到首次登录，用一条清晰路径完成第一次开服。
    link: /guide/getting-started
    linkText: 开始了解
  - icon: '02'
    title: 部署指南
    details: 系统要求、一键脚本部署、NAS 图形化 Compose 部署、端口与安全组说明。
    link: /deploy/requirements
    linkText: 查看部署方式
  - icon: '03'
    title: 日常维护
    details: 在面板内直接升级、存档备份与计划重启、Mod 管理、面板用户与权限、日志诊断。
    link: /maintain/update
    linkText: 查看维护操作
  - icon: '04'
    title: 深度文档
    details: 按 9 个功能页逐页精讲：安装、服务器控制、存档、Mod 管理、玩家管理等完整手册。
    link: /handbook/
    linkText: 查看深度文档
  - icon: NEW
    title: 版本更新日志
    details: 当前最新 v0.4.2。查看面板扫描路径与 SQLite 取消恢复修复，以及从 v0.1.0 至今的完整更新记录。
    link: /changelog
    linkText: 查看更新日志
  - icon: '05'
    title: 常见问题
    details: 装不上、连不上、邀请码不显示、Mod 装不上……按现象查找对应解法。
    link: /faq/
    linkText: 查看常见问题
---

<section class="home-path" aria-labelledby="home-path-title">
  <div class="home-section-heading">
    <span>START HERE</span>
    <h2 id="home-path-title">从一台服务器，到朋友加入农场</h2>
    <p>文档按真实操作顺序组织，不需要先理解 Docker 的全部细节。</p>
  </div>
  <div class="home-path-grid">
    <div><b>01</b><strong>准备环境</strong><span>确认配置、系统与端口</span></div>
    <div><b>02</b><strong>部署面板</strong><span>运行脚本或 NAS Compose</span></div>
    <div><b>03</b><strong>创建世界</strong><span>安装游戏并选择存档</span></div>
    <div><b>04</b><strong>邀请朋友</strong><span>获取邀请码，开始联机</span></div>
  </div>
</section>

<section class="home-note">
  <div>
    <span class="home-note-kicker">CURRENT RELEASE</span>
    <strong>v0.4.2</strong>
    <p>修复扫描路径与 SQLite 请求取消后的连接恢复，并保留一键全栈安全升级能力。</p>
  </div>
  <a href="./changelog">查看本次更新 <span aria-hidden="true">→</span></a>
</section>
