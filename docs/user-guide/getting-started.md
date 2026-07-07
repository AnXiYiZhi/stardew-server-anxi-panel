# 新手使用指南

本文写给只想把面板部署到自己服务器/NAS 上使用的普通用户，不需要看源码。如果你想参与开发，请看 [项目总纲](../01-project-overview.md)。

## 一、部署前确认

- 一台 Linux 服务器（云服务器）或支持 Docker 的 NAS（飞牛、群晖、绿联、威联通等）。
- 已安装 Docker Engine 24+ 和 Docker Compose V2（或使用 NAS 自带的 Docker/Container Manager 应用）。
- 最低 2 核 2 GB 内存、20 GB 可用磁盘；推荐 2 核 4 GB 以上。详细配置建议见 [README 系统要求](../../README.md#系统要求)。
- 云服务器需要能开放公网端口；NAS 家用场景至少要能在局域网访问。

## 二、一键部署（Linux 云服务器推荐）

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

## 三、NAS 图形化部署

没有 SSH 习惯的 NAS 用户可以直接在 Docker / Container Manager / 项目 / 应用栈里粘贴 compose 内容部署，完整步骤（含常见坑）见 [README 的 NAS 图形化部署](../../README.md#nas-图形化-docker-compose-部署)。

## 四、首次进入面板

1. 打开浏览器访问面板地址，首次会进入管理员初始化页，设置用户名和**强密码**。
2. 登录后进入"安装"页，填写 Steam 账号、Steam 密码和 VNC 密码，点击开始安装。
3. 安装会自动拉取 JunimoServer 相关镜像并运行 Steam 登录；如果 Steam 需要二次验证，页面会提示手机 App 批准或输入验证码。
4. 安装完成后回到"总览"或"服务器"页，点击启动服务器。
5. 首次启动如果没有存档，会引导你选择：
   - **新建存档**：在面板里填农场名、玩家名、地图类型等设置，直接生成新农场。
   - **上传存档**：上传自己电脑上的 Stardew 存档 ZIP，面板会先预览游戏时间、地图、玩家名再确认导入。
6. 服务器启动后，去"玩家"页复制邀请码（Steam 好友加入）或"局域网邀请"里的 IP 直连地址给你的朋友。

## 五、开放端口提醒

云服务器安全组 / 路由器端口转发必须放行：

```text
TCP 8090      面板访问
UDP 24642     Stardew 游戏端口
UDP 27015     查询端口
```

按需放行 `TCP 5800`（VNC/noVNC，浏览器看游戏画面）。**不要**把 `TCP 8080`（Junimo 内部 API）开放到公网。完整安全组说明见 [README 云服务器安全组](../../README.md#云服务器安全组)。

## 六、下一步

- 部署或使用中遇到问题：看 [故障排查](troubleshooting.md)。
- 想了解更新、备份、Mod 管理等日常操作：看 [日常维护](maintenance.md)。
