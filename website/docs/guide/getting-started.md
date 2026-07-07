# 快速开始

`Anxi Panel` 是围绕 [JunimoServer](https://stardew-valley-dedicated-server.github.io/server/) 构建的星露谷物语（Stardew Valley）专用服务器 Web 管理面板。

如果你只是想把星露谷物语服务器部署到一台云服务器或 NAS 上，不需要看源码、不需要懂 Docker 细节，跟着本页和后面几页操作即可。

## 这个面板能做什么

运行一个 Docker 镜像，打开浏览器就能：

- 创建管理员账号并登录。
- 一键安装 Stardew 服务器（自动完成 Steam 认证）。
- 新建或上传自己的正式农场存档。
- 启动 / 停止 / 重启服务器，查看邀请码和局域网直连地址。
- 管理存档备份、Mod、控制台命令和面板用户。

## 部署前确认

- 一台 Linux 服务器（云服务器）或支持 Docker 的 NAS（飞牛、群晖、绿联、威联通等）。
- 已安装 Docker Engine 24+ 和 Docker Compose V2（或使用 NAS 自带的 Docker / Container Manager 应用）。
- 最低 2 核 2 GB 内存、20 GB 可用磁盘；推荐 2 核 4 GB 以上。详细配置建议见 [系统要求](/deploy/requirements)。
- 云服务器需要能开放公网端口；NAS 家用场景至少要能在局域网访问。

## 一键部署（Linux 云服务器推荐）

国内加速安装：

```bash
curl -fsSL -o run.sh https://anxinas.dpdns.org/run.sh && chmod +x run.sh && bash run.sh
```

GitHub Release 安装（海外服务器或国内加速不可用时）：

```bash
curl -fsSL -o run.sh https://github.com/anxiyizhi/stardew-server-anxi-panel/releases/latest/download/run.sh && chmod +x run.sh && bash run.sh
```

运行后会出现菜单，按提示一步步选择即可：安装 Docker/Compose（如缺失）、选择镜像源、启动面板。脚本会自动生成 `~/.anxi-panel/.env`、`docker-compose.yml` 和数据目录 `~/.anxi-panel/data`。

安装完成后访问：

```text
http://服务器公网IP:8090
```

更完整的一键脚本用法（更新、强制更新、镜像源切换等）见 [一键脚本部署](/deploy/quick-start)。NAS 用户请看 [NAS 图形化部署](/deploy/nas)。

## 下一步

- 打开面板后要做什么：看 [首次进入面板](/guide/first-login)。
- 部署遇到问题：看 [常见问题](/faq/)。
