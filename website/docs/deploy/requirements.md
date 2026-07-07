# 系统要求

## 最低系统要求

```text
系统：Linux x86_64
发行版：Ubuntu 20.04+ / Debian 11+ / CentOS 8+ / Rocky Linux 8+ / AlmaLinux 8+ / Alibaba Cloud Linux 3+
Docker：Docker Engine 24+
Compose：Docker Compose plugin v2+
CPU：2 核
内存：2 GB
磁盘：20 GB 可用空间
网络：公网 IP
端口：TCP 8090，UDP 24642 / 27015
```

## 推荐配置

```text
系统：Ubuntu 22.04 LTS / Ubuntu 24.04 LTS / Debian 12 / Alibaba Cloud Linux 3
CPU：2 核以上
内存：4 GB 以上
磁盘：40 GB SSD 以上
带宽：5 Mbps 以上
Docker：Docker Engine 25+ / 26+ / 27+
```

## 多人游玩推荐

```text
1-2 人：2 核 2 GB，建议开启 2 GB swap
3-4 人：2 核 4 GB
5-8 人：4 核 8 GB
大量 Mod：4 核 8 GB 起步，磁盘 60 GB+
```

推荐使用 Ubuntu 22.04 LTS、2 核 4G、40G SSD 的云服务器；小型自用服务器最低 2 核 2G 可运行，但建议开启虚拟内存。

## NAS / 飞牛等家用设备建议

```text
轻量自用 NAS：2 核 4 线程、6 GB 内存可运行，建议开启 2-4 GB swap
1-2 人：推荐，可在局域网内稳定使用
3-4 人：原版或少量 Mod 可尝试，存档切日、自动保存、VNC 画面可能短暂卡顿
5 人以上：不建议使用老款低压/移动 CPU NAS，建议换云服务器或更强 NAS
大量 Mod：不建议，尤其是大型内容 Mod、多个框架 Mod、HDD 机械盘环境
```

以 Intel i3 M380 / 2 核 4 线程 / 6 GB DDR3 / HDD 的飞牛 NAS 为例：面板和 Stardew 服务端可以跑，适合家庭局域网或 1-2 个外网好友自用；如果同时跑下载、媒体库转码、虚拟机等 NAS 任务，建议先暂停这些高占用任务。GPU 对 Stardew 服务端基本没有帮助，千兆内网足够。

## 下一步

- 云服务器用户：看 [一键脚本部署](/deploy/quick-start)。
- NAS 用户：看 [NAS 图形化部署](/deploy/nas)。
