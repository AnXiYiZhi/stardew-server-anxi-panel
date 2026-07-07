# NAS 图形化部署

NAS 用户通常没有 SSH 习惯，可以直接用飞牛 / 群晖 / 绿联 / 威联通等系统里的 Docker、容器、Container Manager、项目、Compose、应用栈功能部署。不同 NAS 菜单名字不一样，但核心步骤一致。

## 准备工作

1. 在 NAS 应用商店里安装 Docker / Container Manager。
2. 新建一个目录保存面板数据，例如：

```text
/vol1/1000/docker/anxi-panel/data
```

如果你的 NAS 实际路径不是 `/vol1/1000`，请换成图形界面里显示的真实绝对路径。这个路径必须是宿主机真实路径，不要写成 Windows 盘符。

3. 在 NAS 的防火墙或路由器端口转发里按需放行（详见 [端口与安全组](/deploy/ports)）。

## 部署步骤

1. 在 NAS 的"项目 / Compose / 应用栈"里新建项目，粘贴下面的 `docker-compose.yml`：

```yaml
services:
  anxi-panel:
    image: crpi-9z3bkb9g7fxeohrg.cn-hangzhou.personal.cr.aliyuncs.com/anxi-panel/stardew-server-anxi-panel:latest
    container_name: anxi-panel
    restart: unless-stopped
    ports:
      - "8090:8090"
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock
      - /vol1/1000/docker/anxi-panel/data:/vol1/1000/docker/anxi-panel/data
    environment:
      PANEL_ADDR: ":8090"
      PANEL_DATA_DIR: "/vol1/1000/docker/anxi-panel/data"
      PANEL_DB_PATH: "/vol1/1000/docker/anxi-panel/data/panel.db"
      PANEL_MODE: "single"
      PANEL_SECRET: "please-change-to-a-long-random-string"
```

::: warning 重要
上面两处 `/vol1/1000/docker/anxi-panel/data` 必须保持一致。因为面板会通过 Docker Socket 在宿主机上继续创建 JunimoServer、SteamCMD 等游戏容器，数据目录必须同时对"面板容器"和"NAS 宿主机 Docker"可见。
:::

2. 项目名称填写 `anxi-panel`。
3. 把 `PANEL_SECRET` 改成一串较长随机字符，例如 32 位以上字母数字组合。
4. 如果你的 NAS 数据目录不是 `/vol1/1000/docker/anxi-panel/data`，把 YAML 里两处数据路径一起替换。
5. 启动项目。
6. 打开浏览器访问：

```text
http://NAS局域网IP:8090
```

例如 NAS IP 是 `192.168.1.200`，则访问 `http://192.168.1.200:8090`。

7. 首次进入面板后创建管理员账号，再进入安装页安装 Stardew（详见 [首次进入面板](/guide/first-login)）。

## 为什么这份 compose 只绑定 8090

```text
8090       是面板容器端口，必须写在 anxi-panel 这个 compose 里
24642/udp  是 Stardew 游戏端口，由面板后续创建的 JunimoServer 容器绑定
27015/udp  是查询端口，由面板后续创建的 JunimoServer 容器绑定
5800/tcp   是 VNC/noVNC 端口，由面板后续创建的 JunimoServer 容器绑定
8080/tcp   是 Junimo API，内部使用，不要公网开放
```

不要把 `24642`、`27015`、`5800` 写到 `anxi-panel` service 的 `ports` 里，否则这些端口会被面板容器提前占用，游戏服务器容器启动时可能报端口冲突。NAS 防火墙或路由器端口转发仍然需要按 [端口与安全组](/deploy/ports) 放行，但 Docker Compose 端口映射由后续游戏容器自动处理。

## 常见问题

- 如果 NAS 图形界面不允许挂载 `/var/run/docker.sock`，本项目无法正常控制游戏容器，需要换支持 Docker Socket 挂载的部署方式。
- 如果打开面板正常，但启动游戏失败，先检查 NAS 是否允许容器继续创建其他容器，以及 Docker Socket 是否为读写挂载。
- 如果外网好友无法加入，优先检查路由器 UDP 24642 / UDP 27015 是否转发到 NAS，而不是只开放 TCP。
- NAS 上不建议长期开放 `TCP 8090` 到公网；更推荐局域网管理，外网管理使用 VPN、Tailscale、ZeroTier 或路由器自带 VPN。

更多故障排查见 [常见问题](/faq/)。
