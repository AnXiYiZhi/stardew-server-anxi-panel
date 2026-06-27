# 发布检查清单 (Release Checklist)

本文档用于 Milestone 14 发版前的逐项验收。每项必须实际验证通过后打勾。

---

## 1. 构建验证

- [ ] `go test ./...` 全部通过
- [ ] `npm run build` 通过，无 TypeScript 错误
- [ ] `docker build -t stardew-server-anxi-panel:rc .` 成功
- [ ] 镜像内包含 docker-cli 和 docker-cli-compose
- [ ] `docker exec <panel> docker version` 正常
- [ ] `docker exec <panel> docker compose version` 正常

## 2. Clean Install 验收（全新空数据目录）

使用临时 volume 启动：

```powershell
docker run -d --name anxi-panel-test `
  -p 18090:8090 `
  -v /var/run/docker.sock:/var/run/docker.sock `
  -v anxi-panel-fresh-test:/data `
  stardew-server-anxi-panel:rc
```

- [ ] 容器启动正常，日志无 panic
- [ ] 访问 `http://localhost:18090` 能加载前端页面
- [ ] 未初始化管理员时，静态资源（JS/CSS/图片）不被鉴权拦截
- [ ] 显示管理员初始化注册页面
- [ ] 创建第一个管理员后自动登录进入面板
- [ ] `/health` 返回 200，包含 version/commit/buildDate
- [ ] `/api/version` 返回版本信息
- [ ] 默认 stardew instance 能创建
- [ ] 健康检查能显示 Docker / Compose / 数据目录状态
- [ ] 没安装游戏时，「启动服务器」不误启动，显示「请先安装游戏」提示
- [ ] 审计日志记录初始化管理员和登录行为
- [ ] 清理测试容器和 volume

## 3. Existing Data Upgrade 验收（已有数据目录升级）

使用已有开发数据目录验证：

- [ ] 旧数据库能迁移到最新 schema（migration 幂等）
- [ ] 已有 `instance_state` 不丢
- [ ] 已有 active save 不丢
- [ ] 已有 saves/mods/backups 不丢
- [ ] 已有 audit logs 不被破坏
- [ ] 已有配置文件（.env、docker-compose.yml）不会被无故覆盖
- [ ] compose 模板迁移只修改必要内容
- [ ] 重复启动后不重复插入默认数据

## 4. Docker Compose 部署验收

```powershell
cd deploy
docker compose up -d
```

- [ ] 面板正常启动
- [ ] 挂载 socket 后 Docker 功能可用
- [ ] 数据持久化：重启容器后数据不丢

## 5. 管理员初始化与登录

- [ ] 首次访问显示管理员初始化页
- [ ] 用户名/密码/确认密码表单正常
- [ ] 密码最小长度 6 位校验
- [ ] 创建成功后自动登录
- [ ] 登出后显示登录页
- [ ] 用刚创建的管理员能重新登录
- [ ] 错误密码显示中文错误提示
- [ ] Session 过期后自动跳转登录页

## 6. 游戏安装流程

- [ ] 点击「安装游戏」弹出凭据 Modal
- [ ] Steam 用户名/密码/VNC 密码输入正常
- [ ] 安装任务日志通过 SSE 实时推送
- [ ] Steam Guard 二维码/验证码输入正常
- [ ] 安装成功后状态变为 `game_installed`
- [ ] 安装失败后能重试
- [ ] 安装过程中 job log 不包含密码明文

## 7. 存档管理

- [ ] 创建存档并启动
- [ ] 上传存档预览确认
- [ ] 存档列表显示
- [ ] 选择存档为启动存档
- [ ] 删除存档前自动备份
- [ ] 恢复备份
- [ ] 运行中禁止删除存档
- [ ] 路径穿越攻击被拒绝

## 8. Mod 管理

- [ ] 上传 Mod ZIP
- [ ] Mod 列表显示
- [ ] 删除 Mod
- [ ] 导出 Mod 为 ZIP
- [ ] 运行中禁止上传/删除 Mod
- [ ] 上传后显示「需要重启服务器」

## 9. 控制台命令

- [ ] 服务器运行中显示命令按钮
- [ ] 点击命令按钮显示输出
- [ ] 服务器喊话功能
- [ ] 普通用户只能看到 info/invitecode
- [ ] 管理员能看到所有命令
- [ ] 服务器未运行时按钮禁用

## 10. 审计日志

- [ ] 管理员可以查看审计日志
- [ ] 普通用户不能查看
- [ ] 初始化管理员操作被记录
- [ ] 登录/登出被记录
- [ ] 安装/启动/停止/重启被记录
- [ ] 存档操作被记录
- [ ] Mod 操作被记录
- [ ] 命令执行被记录

## 11. 健康检查

- [ ] 点击「开始检查」显示诊断结果
- [ ] Docker daemon 状态检查
- [ ] Docker Compose 状态检查
- [ ] 数据目录可写性检查
- [ ] 实例目录检查
- [ ] Compose 文件检查
- [ ] Active save 状态检查
- [ ] 导出诊断包按钮可用
- [ ] 导出的 ZIP 包含 version.json、health.json、instance-state.json 等
- [ ] 导出内容已脱敏（无密码、token、session）

## 12. 权限验收

- [ ] 未登录时访问 API 返回 401
- [ ] 普通用户访问管理员接口返回 403
- [ ] 最后一个管理员不能被禁用
- [ ] 当前登录管理员不能禁用自己
- [ ] 普通用户不能看到用户管理区域
- [ ] 普通用户不能看到 Docker 调试区域

## 13. 敏感信息脱敏

- [ ] Steam 密码不写入 job log
- [ ] VNC 密码不写入 job log
- [ ] Session token 不写入日志
- [ ] Bearer token 被脱敏
- [ ] 邀请码在日志中被脱敏
- [ ] 支持包导出中无密码/token
- [ ] 错误响应不暴露内部路径和堆栈

## 14. 版本信息

- [ ] `/health` 返回 version、commit、buildDate
- [ ] `/api/version` 返回版本信息
- [ ] 前端页面显示版本号
- [ ] Dockerfile 支持 `--build-arg VERSION/COMMIT/BUILD_DATE`
- [ ] 构建时版本信息注入到二进制中

## 15. 支持包导出

- [ ] POST `/api/instances/:id/support-bundle` 管理员可用
- [ ] 普通用户返回 403
- [ ] 导出 ZIP 包含：version.json、health.json、instance-state.json、jobs.json、audit-logs.json、compose-ps.json、docker-compose.yml、server-logs.txt
- [ ] 所有日志内容已脱敏
- [ ] 不包含完整存档、完整 Mod、Steam session、数据库原文件
- [ ] 前端「导出诊断包」按钮可下载

## 16. 前端体验

- [ ] 删除存档有二次确认
- [ ] 删除 Mod 有二次确认
- [ ] 删除用户有二次确认
- [ ] 长任务按钮有 loading/disabled 状态
- [ ] 常见错误码显示中文消息
- [ ] 未登录时跳转登录页
- [ ] 登录过期时跳转登录页
- [ ] 权限不足时显示中文提示
- [ ] 窄屏（320px+）不横向溢出
- [ ] Modal 不被遮挡，滚动正常
- [ ] 健康检查入口能找到
- [ ] 审计日志入口能找到
- [ ] 备份恢复入口能找到
- [ ] 支持包导出入口能找到

## 17. 已知问题

### 联机角色槽异常

同一邀请码下重复出现「新农夫」入口。这是 JunimoServer/Stardew 联机层面的行为，面板不会误触发 `newgame`。

**处理方式**：
- 发布检查清单已记录
- troubleshooting 文档已记录
- 支持包能带上 instance state 诊断
- 不做破坏性存档修改工具

### 控制台通信方式

当前通过 `/tmp/smapi-input` FIFO 和 `/tmp/server-output.log` 通信，不使用 `attach-cli` TTY。`say` 不是可用 SMAPI 命令。

---

## Smoke Test 脚本

运行冒烟测试：

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\smoke-test.ps1
```

可选参数：
- `-SkipDocker`：跳过 Docker 镜像构建和容器测试
- `-SkipFrontend`：跳过前端构建
- `-SkipBackend`：跳过后端测试

## 构建带版本号镜像

```powershell
# 获取当前 commit hash
$commit = git rev-parse --short HEAD
$date = (Get-Date -AsUTC -Format 'yyyy-MM-ddTHH:mm:ssZ')

# 构建
docker build -t stardew-server-anxi-panel:1.0.0 `
  --build-arg VERSION=1.0.0 `
  --build-arg COMMIT=$commit `
  --build-arg BUILD_DATE=$date .
```

## 发布后验证

- [ ] 生产环境面板能正常启动
- [ ] 健康检查返回 200
- [ ] 版本号正确显示
- [ ] 旧数据能正常迁移
