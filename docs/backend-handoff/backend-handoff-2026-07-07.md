# NEXUS-DNS-FALLBACK-1 出站请求 DNS 自愈（Nexus / Docker Hub / 公网 IP）

## 背景
- 真机（飞牛 NAS）现场：面板"模组管理 → 下载模组"搜索一直失败，前端提示"Nexus 网络连接失败"。
- SSH 诊断 `anxi-panel` 容器日志，稳定复现根因：
  `Post "https://api.nexusmods.com/v2/graphql": dial tcp: lookup api.nexusmods.com on 127.0.0.11:53: server misbehaving`。
- `127.0.0.11` 是 Docker 内嵌 DNS，转发给宿主机唯一上游——消费级路由器 `192.168.0.1`。该路由器 DNS 能解析 A 记录，但对 AAAA 返回空、高负载下**间歇性 SERVFAIL**，Go 解析器报 "server misbehaving"，于是所有出站到公网服务的请求都可能被打断。
- 同一根因也导致日志里大量 `docker hub tag fetch failed`（版本检查）。公共 DNS `223.5.5.5` 实测完全正常。
- 用户为普通玩家，不能指望其手动修路由器/改宿主机 DNS，所以在代码层做自愈。

## 改了什么
- 新增包 `backend/internal/netdns`：提供带 **DNS 兜底** 的 `http.Client` / `http.Transport`。
  - 解析顺序：先用系统解析器（`net.DefaultResolver`，尊重内网/自定义 DNS，健康环境零改变），失败后**直接向公共 DNS 查询**：`223.5.5.5`(AliDNS)、`119.29.29.29`(DNSPod)、`1.1.1.1`(Cloudflare)、`8.8.8.8`(Google)，按序尝试，命中即停。
  - SERVFAIL/空答复都会触发下一台 DNS；解析出的 IP 里 IPv4 优先，避免 v4-only 网络在 v6 上耗尽拨号预算。
  - 地址字面量（已是 IP）直接放行，不做多余解析。
- 三处出站客户端统一切到 `netdns.NewClient(...)`：
  - `nexus.go`：`nexusHTTPClient`、`nexusArchiveHTTPClient`。
  - `install_handlers.go`：新增 `dockerHubHTTPClient`，替换原来的 `http.DefaultClient.Do`（Docker Hub 版本检查）。
  - `public_ip.go`：公网 IP 探测客户端。

## 影响文件
- 新增 `backend/internal/netdns/netdns.go`、`backend/internal/netdns/netdns_test.go`
- `backend/internal/games/stardew_junimo/nexus.go`
- `backend/internal/web/install_handlers.go`
- `backend/internal/web/public_ip.go`

## 如何验证
- 单测：`cd backend; go test ./internal/netdns`（覆盖失败→兜底、命中即停、全失败、IPv4 优先、以及经假解析器完成完整拨号/HTTP 请求的路径）。
- 关联包：`go test ./internal/web ./internal/games/stardew_junimo`、`go vet ./internal/netdns ./internal/web ./internal/games/stardew_junimo` 全绿。
- 真机（新镜像发布后）：容器日志不再出现 `dial tcp: lookup ... server misbehaving`；模组搜索正常返回；`docker hub tag fetch failed` 消失。

## 现场临时处置（已在真机执行）
- 在 `/vol1/1000/docker/anxi_panel/.anxi-panel/docker-compose.yml` 的 `panel` 服务加了 `dns: [223.5.5.5, 119.29.29.29, 1.1.1.1]` 并重建容器，已即时恢复搜索（原文件备份为 `docker-compose.yml.bak-dnsfix`）。
- 代码级修复上线后，该 compose 改动可保留也可撤掉（保留无害）。发布模板里**不**默认写死 `dns:`，避免给内网/海外用户造成不必要的 DNS 约束——由二进制自愈即可。

## 下一步注意事项
- 公共 DNS 列表目前硬编码，中文优先。若未来面向海外部署有需要，可加一个 `PANEL_DNS_FALLBACK`（逗号分隔）环境变量覆盖，不改核心逻辑。
- 前提是容器允许出站 UDP/TCP 53 到公网 DNS（家用 NAS 一般允许）。若某网络同时封了本地和公网 53，则与现状一致，无法自愈。
- 后续任何新增的对外 HTTP 调用，直接用 `netdns.NewClient(timeout)`，不要再裸用 `http.DefaultClient` / `&http.Client{}`。
