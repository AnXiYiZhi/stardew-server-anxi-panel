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

# NEXUS-EXT-VERSION-CACHE-1 浏览器扩展交付版本感知 + 适配 Nexus 新版下载入口

## 背景
- 真机截图：Nexus 改版后 Mod 页头部下载按钮由 `Manual download` 变短标签 `Manual`（旁 `Vortex`）。有依赖的 Mod（如 SpaceCore 依赖 SMAPI）点 `Manual` 会先弹 `Download mod file` 模态框，框内才有真正的 `Manual download`；无依赖的 Mod 不弹。旧扩展只匹配 `Manual download` 文案，在 Mod 页直接报“未找到 Manual download 按钮”。
- 另一问题：面板下载扩展接口只按结构校验（有无 `manifest.json`/`background.js`）复用实例目录缓存 ZIP，不看版本。导致扩展升级后，服务器上旧缓存会一直遮住新版，必须手动删缓存才能生效。

## 改了什么
- 扩展（`browser-extensions/nexus-slow-installer`，`0.1.0 → 0.1.1`）：
  - 主路径改为**直接拼链接**：`content.js` 新增 `findFileIdOnPage()`，在 Mod 页从 DOM（仅限当前 Mod 的链接/`data-file-id`）恢复 `file_id`，直接走 `generateNexusDownloadUrl()` 拿临时 ZIP 链接，跳过按钮/模态框；对弹与不弹模态的页面都适用。
  - 兜底路径：抽不到 `file_id` 或直接生成失败时才点按钮。`findManualDownloadButton` 收紧为严格 `manual download`，新增 `findShortManualButton` 匹配短 `Manual`；`clickManualDownloadWhenReady` 改成两步（点短 `Manual` 开模态 → 点模态里 `Manual download`），带 4 秒节流。
  - 同步 `manifest.json` + `background.js`/`panel-bridge.js` 的 `X-Anxi-Nexus-Installer` 版本头到 `0.1.1`（后端不校验该头，仅信息用）。
- 后端 `nexus_extension_pack.go`：`EnsureNexusInstallerExtensionZip()` 改为**版本感知**。读源码 `manifest.json` 版本为期望值，实例缓存 / 预包 ZIP 仅在其 `manifest.json` 版本与之完全一致时复用，否则从源码重新打包。新增 `expectedNexusInstallerExtensionVersion()`、`readManifestVersionFromZip/Dir()`、`parseManifestVersion()`、`nexusInstallerExtensionZipMatchesVersion()`。源码不可用时退回旧的仅结构校验行为。

## 影响文件
- `browser-extensions/nexus-slow-installer/content.js`、`manifest.json`、`background.js`、`panel-bridge.js`、`README.md`
- `backend/internal/games/stardew_junimo/nexus_extension_pack.go`
- `backend/internal/games/stardew_junimo/mod_sync_test.go`（复用测试改用动态源码版本；新增 `TestEnsureNexusInstallerExtensionZip_RefreshesStaleVersionPackage`）

## 如何验证
- `cd backend; go test ./internal/games/stardew_junimo ./internal/web`、`go vet ./internal/games/stardew_junimo`。
- 扩展：`node --check content.js background.js panel-bridge.js shared.js`。
- 版本感知：预置一个旧版本 manifest 的实例缓存 ZIP，调用下载接口后 ZIP 应被刷新为源码版本（新测试覆盖）。

## 下一步注意事项
- 以后升级扩展只需 bump `manifest.json` 的 `version` 并重建镜像，用户重新下载即可拿到新版，无需再手动删实例缓存。
- 首次发布本版本到已部署服务器时，旧实例缓存版本仍是 `0.1.0`（< `0.1.1`）会被自动刷新；但如果旧缓存恰好没有版本或结构损坏，走的仍是重打包兜底，同样安全。
- `findFileIdOnPage()` 只信当前 Mod 的链接，避免抓到依赖（如 SMAPI）的 `file_id`；多文件 Mod 取页面上第一个当前 Mod 的 `file_id`，通常是主文件。
