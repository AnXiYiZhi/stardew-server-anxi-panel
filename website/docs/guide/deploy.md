# 部署安装

有了服务器之后（自己的云服务器/NAS，或按 [服务器选择](/guide/choose-server) 领的阿里云免费试用），在服务器终端里跑一键脚本即可完成部署。

## 一键部署（Linux 云服务器推荐）

国内加速安装：

```bash
curl -fsSL -o run.sh http://anxinas.dpdns.org/run.sh && chmod +x run.sh && bash run.sh
```

GitHub Release 安装（海外服务器或国内加速不可用时）：

```bash
curl -fsSL -o run.sh https://github.com/anxiyizhi/stardew-server-anxi-panel/releases/latest/download/run.sh && chmod +x run.sh && bash run.sh
```

运行后会出现菜单：

```text
[0] 下载/检查环境并启动面板（推荐）
[1] 安装/修复 Docker 与 Compose
[2] 启动/恢复面板
[3] 停止面板
[4] 重启面板
[5] 更新面板镜像并重建容器
[6] 强制更新面板镜像
[7] 切换镜像源/加速节点
[8] 更新 run.sh 启动脚本
[9] 设置虚拟内存
[10] 设置开机自启
[11] 查看面板状态
[12] 查看面板日志
[13] 显示访问地址
[14] 退出
```

建议先输入 `9` 回车，给内存较小的服务器开一个 2-4 GB 的 swap 虚拟内存，避免安装或运行过程中因为内存不够被系统杀掉进程：

![run.sh 菜单，选项 9 设置虚拟内存，输入大小后回车继续](./run-sh-menu-swap.png)

设置完成按回车键回到菜单，再输入 `0` 回车，开始下载/检查环境并启动面板：

- 如果服务器还没装 Docker，会提示"安装/修复 Docker"，输入 `y` 回车即可，脚本会自动安装 Docker 和 Compose。
- 如果安装过程中提示选择镜像源，直接回车用默认选项 `1`（国内最快）即可。
- 安装完成后会自动启动容器，显示访问地址和端口提醒：

![run.sh 安装完成，显示本机访问/公网访问地址和端口提醒](./run-sh-install-done.png)

::: warning 脚本显示的"公网访问"地址不一定准
脚本探测到的公网地址有时其实是内网 IP（比如 `172.x.x.x`），打不开是正常的。以云服务器控制台"实例详情"页显示的公网 IP 为准：

![阿里云实例详情页，公网 IP 和"网络与安全组"标签位置](./aliyun-instance-public-ip.png)
:::

用控制台显示的真实公网 IP 打开 `http://公网IP:8090`。如果打不开，去实例详情页的"网络与安全组"标签，添加入方向规则放行需要的端口（协议类型选"自定义 TCP"，访问来源选 `0.0.0.0/0`，端口填 `8090`）：

![添加入方向规则弹窗：自定义 TCP，访问来源 0.0.0.0/0，端口 8090](./aliyun-security-group-add-rule.png)

必须放行：

```text
TCP 8090      面板访问
UDP 24642     Stardew 游戏端口
UDP 27015     查询端口
```

按需放行 `TCP 5800`（VNC/noVNC，浏览器看游戏画面才需要）。完整端口说明见 [端口与安全组](/deploy/ports)。

打开 `http://公网IP:8090` 后，尽快注册自己的管理员账号并设置强密码。

更完整的一键脚本用法（更新、强制更新、镜像源切换等）见 [一键脚本部署](/deploy/quick-start)。NAS 用户请看 [NAS 图形化部署](/deploy/nas)。

## 下一步

- 打开面板后要做什么：看 [首次进入面板](/guide/first-login)。
- 部署遇到问题：看 [常见问题](/faq/)。
