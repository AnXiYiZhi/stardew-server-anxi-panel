# 快速上手

`Anxi Panel` 是围绕 [JunimoServer](https://stardew-valley-dedicated-server.github.io/server/) 构建的星露谷物语（Stardew Valley）专用服务器 Web 管理面板。

星露谷联机常卡在房主在线、跨平台加入、异地网络、端口转发、Steam 邀请、存档回档和 Mod 对齐上。
Anxi Panel 基于 JunimoServer 把专用服务器搬进浏览器：用云服务器或 NAS 长期托管农场，统一处理安装、Steam 认证、启停、存档、Mod 同步和诊断；不看源码、不懂 Docker，也能跟着本指南部署。

## 这个面板能做什么

运行一个 Docker 镜像，打开浏览器就能：

- 创建管理员账号并登录。
- 一键安装 Stardew 服务器（自动完成 Steam 认证）。
- 新建或上传自己的正式农场存档。
- 启动 / 停止 / 重启服务器，查看邀请码和局域网直连地址。
- 管理存档备份、Mod、控制台命令和面板用户。

## 三步跑起来

1. **[服务器选择](/guide/choose-server)**：确认系统要求，没有服务器的话可以先领阿里云免费试用。
2. **[部署安装](/guide/deploy)**：一条命令跑一键脚本，装好 Docker、拉镜像、启动面板。
3. **[首次进入面板](/guide/first-login)**：建管理员账号、装游戏、建/传存档、拿邀请码。

## 下一步

- 还没有服务器，或者不确定该选什么配置：看 [服务器选择](/guide/choose-server)。
- 已经有服务器了，直接部署：看 [部署安装](/guide/deploy)。
- 部署遇到问题：看 [常见问题](/faq/)。
