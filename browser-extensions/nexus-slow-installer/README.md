# 当前测试流程（NEXUS-EXT-3）

1. 面板和 Nexus 需要在同一个浏览器里提前登录，面板用户必须是管理员，服务器需要停服。
2. 在面板 Mods 下载页点击 Nexus 搜索结果的“一键安装”，页面会同页跳转到 Nexus Mod 文件页并带上 `anxi_auto=1`。
3. 扩展会自动进入下载流程，获取 `supporter-files.nexus-cdn.com/*.zip` 临时链接后，右下角只保留“提交到面板”按钮。
4. 点击提交后，扩展调用面板现有 `POST /api/instances/:id/mods/remote/install` 接口创建任务，并立刻跳回 `/instances/:id/jobs?jobId=...` 查看任务日志。

默认面板地址现在是 `http://127.0.0.1:5173`，正式部署或本机后端端口不同时，请在扩展弹窗里改成实际面板地址。

# Stardew Anxi Nexus Installer

本目录是一个可手动加载的 Chrome / Edge Manifest V3 扩展，用来测试免费 Nexus 用户的“慢速下载 -> 面板远程安装”链路。

## 它做什么

1. 在 `nexusmods.com` 的 Mod 文件下载页识别 `file_id`。
2. 可自动点击 `Slow download`。
3. 等浏览器开始下载 `supporter-files.nexus-cdn.com/*.zip?...`。
4. 取消浏览器本地下载。
5. 把临时 ZIP 链接提交给面板已有接口：

```text
POST /api/instances/:id/mods/remote/install
```

请求体：

```json
{
  "url": "https://supporter-files.nexus-cdn.com/....zip?...",
  "mod": {
    "modId": 1348,
    "name": "",
    "nexusUrl": "https://www.nexusmods.com/stardewvalley/mods/1348?tab=files&file_id=145986"
  }
}
```

## 安装测试

1. 打开 Chrome / Edge 的扩展管理页。
2. 开启“开发者模式”。
3. 点击“加载已解压的扩展程序”。
4. 选择本目录：`browser-extensions/nexus-slow-installer`。
5. 点扩展图标，填写：
   - 面板地址，例如 `https://panel.example.com` 或 `http://127.0.0.1:8080`
   - 实例 ID，默认 `stardew`
6. 在同一个浏览器里登录面板管理员账号。
7. 确认服务器处于停止状态。
8. 在同一个浏览器里登录 Nexus。
9. 打开 Nexus 的文件下载页，扩展会开始捕获并尝试点击 `Slow download`。

## 注意事项

- 第一版复用面板的登录 Cookie。面板 session 是 HttpOnly Cookie，扩展通过 `credentials: "include"` 发请求；如果云端部署的浏览器策略导致 Cookie 没有带上，会看到 401/403。产品化时建议新增扩展专用 token 接口。
- 扩展只把完整 CDN 链接发给面板，不把完整链接写入状态；状态里会脱敏 `md5/expires/user_id/key`。
- 面板后端仍负责校验 ZIP、解压和导入 Mod。扩展不直接写服务器文件。
- 当前 manifest 为了测试云端/本地面板地址，临时使用了 `http://*/*` 和 `https://*/*` host 权限；正式发布前应收窄为用户配置的面板域名加 Nexus 域名。
# NEXUS-REQ-1

- Nexus 出现 “Additional files required” 弹窗时，扩展会自动点击弹窗里的 `Download` 按钮继续下载流程。
- 这个自动点击只在扩展已经开始捕获时启用，并且只匹配包含前置提示文案的弹窗容器，避免误点普通页面按钮。
