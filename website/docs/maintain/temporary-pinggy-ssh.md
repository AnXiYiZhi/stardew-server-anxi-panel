# 无公网 IP 时，用 Pinggy 临时开放 root SSH

当服务器没有公网 IPv4、路由器无法做端口转发，或设备运行在 WSL2、飞牛 fnOS 等内网环境中时，可以用 Pinggy 建立一条**临时 TCP 反向隧道**，让协助排障的人通过公网 SSH 进入服务器。

本文默认提供 `root` 登录，适合需要检查 Docker、WSL 文件系统、Anxi Panel 数据或系统服务的维修场景。root 权限风险很高，请只在双方已经约定好的维修时间内操作，完成后立即关闭。

::: danger 不要提供无密码 root
Pinggy 只负责把公网 TCP 流量转到本地 SSH 端口，不会替 SSH 服务设置密码。`root` 必须使用临时强密码，不能留空，也不要把长期使用的 root、面板、Steam 或 Windows 密码发送给别人。
:::

## 工作原理

```text
协助者的电脑
  └─ ssh root@Pinggy临时主机 -p 临时端口
       └─ Pinggy TCP 隧道（服务器主动向外连接 443）
            └─ 服务器本机 127.0.0.1:22222
                 └─ 临时 root sshd
```

这是服务器主动建立的出站连接，因此通常不需要公网 IP、不需要路由器端口转发，也不需要把 `22` 端口暴露到路由器。Pinggy 官方将 SSH 归类为 TCP 隧道，当前免费命令使用 `tcp@free.pinggy.io`；免费地址和端口会随重连变化，并受免费套餐时长限制，具体以 [Pinggy TCP 文档](https://pinggy.io/docs/tcp_tunnels/) 和 [帮助页](https://pinggy.io/help/) 的最新说明为准。

本文使用单独的 `127.0.0.1:22222` 临时 sshd，而不是直接永久修改系统原有的 `/etc/ssh/sshd_config`。这样公网只能经 Pinggy 到达这个临时端口，维修结束后也更容易完整清理。

## 开始前准备

需要满足：

- 你能在服务器本机、局域网终端或 NAS Web 终端里执行命令。
- 服务器能访问互联网的 TCP `443` 出站端口。
- 系统中有 OpenSSH 客户端和服务端：`ssh`、`sshd`、`ssh-keygen`。
- 你知道如何在维修结束后回到服务器终端执行清理命令。
- Windows/WSL2 电脑在维修期间不能关机、休眠或退出 WSL；飞牛/NAS 也不能重启。

先检查工具：

```bash
ssh -V
sudo /usr/sbin/sshd -V 2>&1 | head -n 1
```

如果第二条提示文件不存在，按系统类型安装：

```bash
# Ubuntu / Debian / WSL Ubuntu
sudo apt update
sudo apt install -y openssh-client openssh-server

# Rocky / AlmaLinux / Fedora
sudo dnf install -y openssh-clients openssh-server
```

## 通用 Linux：建立临时 root SSH

### 1. 记录 root 当前状态并设置临时密码

先查看 root 账号当前是锁定状态还是已有密码：

```bash
sudo passwd -S root
```

记住第二列：

- `L`：root 原本锁定。维修结束后需要重新锁定。
- `P`：root 原本已有密码。维修结束后需要重新设置一个不对外泄露的新密码。

设置本次维修专用的临时强密码：

```bash
sudo passwd root
```

密码输入时终端不显示字符，这是正常现象。建议使用至少 16 位、同时包含大小写字母、数字和符号的随机密码；不要把密码直接写在命令行里，以免进入 Shell 历史。

### 2. 创建独立的临时 sshd 配置

进入 root Shell：

```bash
sudo -i
```

确认下面的配置文件尚不存在：

```bash
test ! -e /etc/ssh/sshd_config_pinggy_root
```

如果命令没有任何输出并返回提示符，可以继续；如果提示文件已存在，先停止，不要覆盖未知配置。

创建仅监听本机回环地址、只允许 root 密码登录的临时服务：

```bash
cat > /etc/ssh/sshd_config_pinggy_root <<'EOF'
Port 22222
ListenAddress 127.0.0.1
HostKey /etc/ssh/ssh_host_ed25519_key
PidFile /run/sshd-pinggy-root.pid
PermitRootLogin yes
PasswordAuthentication yes
KbdInteractiveAuthentication no
PermitEmptyPasswords no
PubkeyAuthentication no
AuthenticationMethods password
UsePAM yes
AllowUsers root
LoginGraceTime 30
MaxAuthTries 3
MaxSessions 2
X11Forwarding no
AllowAgentForwarding no
AllowTcpForwarding no
PermitTunnel no
GatewayPorts no
Subsystem sftp internal-sftp
LogLevel VERBOSE
EOF

chmod 600 /etc/ssh/sshd_config_pinggy_root
ssh-keygen -A
/usr/sbin/sshd -t -f /etc/ssh/sshd_config_pinggy_root
```

最后一条没有输出才代表语法检查通过。然后启动独立 sshd：

```bash
/usr/sbin/sshd -f /etc/ssh/sshd_config_pinggy_root
ss -lntp | grep '127.0.0.1:22222'
```

应看到 `sshd` 正在监听 `127.0.0.1:22222`，不能是 `0.0.0.0:22222`。

退出 root Shell：

```bash
exit
```

### 3. 本机先测试 root 登录

在同一台服务器上运行：

```bash
ssh -p 22222 root@127.0.0.1
```

首次连接会询问是否信任本机主机密钥，确认显示的是这台设备后输入 `yes`，再输入刚设置的临时 root 密码。成功进入后执行：

```bash
whoami
exit
```

`whoami` 必须输出 `root`。如果本机都无法登录，不要启动 Pinggy，先看后面的故障排查。

### 4. 输出主机指纹

把 SSH 主机指纹一起发给协助者，便于第一次公网连接时核对目标服务器：

```bash
sudo ssh-keygen -lf /etc/ssh/ssh_host_ed25519_key.pub
```

发送 `SHA256:...` 指纹即可，不要发送 `/etc/ssh/ssh_host_ed25519_key` 私钥文件。

### 5. 启动 Pinggy TCP 隧道

在服务器终端运行：

```bash
ssh -p 443 \
  -R0:127.0.0.1:22222 \
  -o ExitOnForwardFailure=yes \
  -o ServerAliveInterval=30 \
  -o ServerAliveCountMax=3 \
  tcp@free.pinggy.io
```

说明：

- 必须是 `tcp@free.pinggy.io`，不能省略 `tcp@`，否则可能创建成 HTTP 隧道，SSH 无法使用。
- 第一次连接 Pinggy 会询问主机密钥，核对目标是 `free.pinggy.io` 后输入 `yes`。
- 如果 Pinggy 自身询问登录密码，按其免费隧道提示直接回车；这和稍后登录服务器时使用的 root 密码不是一回事。
- **这个终端窗口必须保持打开。** 按 `Ctrl+C`、关闭终端、网络断开、电脑休眠或免费隧道到期都会立刻断开公网 SSH。

成功后终端会显示类似：

```text
tcp://example.a.free.pinggy.online:37315
```

域名后缀和端口可能不同，必须复制终端实际显示的完整地址，不要照抄示例。

协助者会把它转换成：

```bash
ssh -p 37315 root@example.a.free.pinggy.online
```

Pinggy 的 SSH 反向转发语法和 TCP 模式可在 [官方 Usage 文档](https://pinggy.io/docs/usages/) 与 [SSH 设备示例](https://pinggy.io/docs/guides/ssh_iot/) 中核对。

## WSL2：应该在哪个终端执行

如果 Anxi Panel 和 Docker 项目运行在 WSL2 中，临时 sshd 与 Pinggy 命令都应在**同一个 WSL 发行版**里执行。这样 `127.0.0.1:22222` 明确指向该 WSL，不需要设置 Windows `portproxy`。

### 1. 从 Windows 进入正确的 WSL

在 Windows Terminal 或 PowerShell 中查看发行版：

```powershell
wsl -l -v
```

找到实际部署项目且 `VERSION` 为 `2` 的发行版，然后进入，例如：

```powershell
wsl -d Ubuntu
```

进入后检查：

```bash
uname -a
pwd
ls -la ~/.anxi-panel
```

能看到原来的 `~/.anxi-panel` 才说明进入了正确发行版。不要在另一个 Ubuntu、PowerShell、CMD 或 Docker 容器终端中误建隧道。

### 2. 在 WSL 内完成通用步骤

仍在这个 WSL 终端中执行：

1. 安装 `openssh-client openssh-server`。
2. 按“通用 Linux”章节设置临时 root 密码。
3. 启动 `127.0.0.1:22222` 临时 sshd。
4. 本机测试 `ssh -p 22222 root@127.0.0.1`。
5. 在同一个 WSL 中运行 Pinggy 命令。

这套临时 sshd 不依赖 systemd，因此通常不需要为了本次排障修改 `/etc/wsl.conf`。如果你本来就要让 WSL 服务由 systemd 管理，可参考 [Microsoft 的 WSL systemd 文档](https://learn.microsoft.com/windows/wsl/systemd)；WSL2 的 localhost 与网络模式说明见 [Microsoft WSL 网络文档](https://learn.microsoft.com/windows/wsl/networking)。

::: warning Windows 休眠会断线
即使 Docker 容器仍配置了重启策略，Windows 休眠、关机、`wsl --shutdown` 或退出 Docker Desktop，都会让本次临时隧道失效。维修期间不要执行这些操作。
:::

## 飞牛 fnOS：从 Web 管理进入终端

飞牛新版可能同时控制“全局 SSH 服务”和“用户是否允许 SSH”。当前常见路径是：

1. 登录飞牛 Web 管理页面。
2. 打开 **系统设置 → SSH**，临时开启 SSH 服务。
3. 打开 **系统设置 → 用户管理**，找到管理员用户，在“更多”中启用该用户的 SSH 权限。
4. 在同一局域网电脑上先连接 NAS：

```bash
ssh 管理员用户名@NAS局域网IP
```

5. 登录后运行 `sudo -i` 进入 root Shell，再按本文“通用 Linux”章节创建临时 root sshd。

飞牛初始化时创建的首位管理员通常具有较高管理权限；具体界面和权限以当前系统版本为准，可参考 [飞牛初始化说明](https://help.feiniuos.com/10001.html)。较新的飞牛版本可能在升级后重新关闭 SSH，因此如果局域网连接被拒绝，要同时检查 SSH 总开关和用户 SSH 权限。

在飞牛 Shell 中启动 Pinggy：

```bash
ssh -p 443 \
  -R0:127.0.0.1:22222 \
  -o ExitOnForwardFailure=yes \
  -o ServerAliveInterval=30 \
  -o ServerAliveCountMax=3 \
  tcp@free.pinggy.io
```

如果飞牛中只有 `sshd`、没有可用的 `ssh` 客户端，可先通过系统包管理器补齐 `openssh-client`。不要随意替换飞牛系统仓库或执行大版本升级来安装一个临时工具。

### 飞牛 Docker 备用方式

如果飞牛的宿主 Shell 能用 Docker，但确实无法安装 `ssh` 客户端，可以使用 Pinggy 官方容器并让它访问宿主机回环端口：

```bash
docker run --rm -it --network host pinggy/pinggy \
  -p 443 -R0:127.0.0.1:22222 tcp@free.pinggy.io
```

只在 Linux/FNOS 宿主机使用 `--network host`。容器退出后会由 `--rm` 自动删除；如果镜像拉取或 Host 网络被 NAS 限制，回到原生 `ssh` 方案，不要为了隧道修改 Anxi Panel 的 Compose。

## 把哪些凭据发给我

确认本机 root 登录成功、Pinggy 已显示 TCP 地址后，在**当前私聊**中按下面模板发送纯文本：

```text
目标环境：WSL2 / 飞牛 fnOS / 普通 Linux
Pinggy 地址：tcp://实际域名:实际端口
SSH 用户：root
临时密码：本次新设置的 root 密码
ED25519 指纹：SHA256:实际指纹
授权范围：例如“只读诊断”或“允许备份、停服并修复当前实例”
隧道预计保持到：例如“今晚 23:00 前”
```

注意：

- 不要只发原始公网 IP；连接需要的是 Pinggy 输出的域名和端口。
- 不要把 `tcp://` 地址误当成密码。
- 不要发送 Windows 登录密码、Steam 密码、面板 Session、Cookie、Token 或私钥。
- 不要把凭据贴到群聊、论坛、GitHub Issue、截图或公开日志中。
- 如果隧道重启，旧地址会失效，需要重新发送新地址和端口。
- 对方首次连接时应核对你发来的 ED25519 指纹，不应通过关闭主机密钥校验来绕过不一致。

## 维修结束后的强制清理

### 1. 关闭公网隧道

回到运行 Pinggy 的终端，按：

```text
Ctrl+C
```

然后确认没有残留隧道进程：

```bash
ps -ef | grep '[s]sh .*free\.pinggy\.io'
```

正常情况下没有输出。

### 2. 停止并删除临时 sshd

先查看 PID 和实际命令，确认它确实是本文创建的 `sshd_config_pinggy_root`：

```bash
sudo sh -c 'pid=$(cat /run/sshd-pinggy-root.pid); ps -p "$pid" -o pid=,args='
```

确认无误后停止并清理：

```bash
sudo sh -c 'pid=$(cat /run/sshd-pinggy-root.pid); kill "$pid"'
sudo rm -f /run/sshd-pinggy-root.pid
sudo rm -f /etc/ssh/sshd_config_pinggy_root
ss -lntp | grep '127.0.0.1:22222' || true
```

最后一条应没有输出。

### 3. 回收 root 密码

如果开始前 `passwd -S root` 显示 `L`，重新锁定：

```bash
sudo passwd -l root
```

如果开始前显示 `P`，不要锁定正在被系统使用的 root；应立即运行下面的命令，把已发给协助者的临时密码换成新的私有密码：

```bash
sudo passwd root
```

### 4. 恢复平台开关

- WSL2：临时 sshd 和 Pinggy 都关闭后，无需保留额外服务。
- 飞牛 fnOS：如果维修前 SSH 总开关或管理员 SSH 权限是关闭的，在 Web 管理中恢复为关闭。
- 普通 Linux：本文没有修改系统原 sshd 配置；原有 SSH 服务保持原状。

## 故障排查

### `Connection refused` 或 Pinggy 能启动但公网 SSH 连不上

先在服务器本机检查：

```bash
ss -lntp | grep '127.0.0.1:22222'
ssh -p 22222 root@127.0.0.1
```

本机不通就是临时 sshd 问题，不是 Pinggy 问题。重新运行：

```bash
sudo /usr/sbin/sshd -t -f /etc/ssh/sshd_config_pinggy_root
```

根据它输出的具体配置错误修正，不能跳过语法检查。

### `Permission denied` 或 root 密码一直失败

检查：

```bash
sudo passwd -S root
sudo /usr/sbin/sshd -T -f /etc/ssh/sshd_config_pinggy_root | \
  grep -E 'permitrootlogin|passwordauthentication|authenticationmethods|allowusers'
```

应看到 root 未锁定、`permitrootlogin yes`、`passwordauthentication yes` 和 `allowusers root`。密码输入不回显，不代表键盘没有输入。

### Pinggy 只给 HTTP/HTTPS 地址，没有 `tcp://`

说明启动成了 HTTP 隧道。停止当前命令，确认用户名部分是：

```text
tcp@free.pinggy.io
```

### WSL 中找不到项目或连进去不是目标系统

你可能进入了错误的 WSL 发行版，或者在 Windows 原生 SSH 上建了隧道。回到 PowerShell运行：

```powershell
wsl -l -v
```

进入真正保存 `~/.anxi-panel` 的发行版，并在里面重新建立临时 sshd 和 Pinggy 隧道。

### 运行一段时间后地址失效

免费隧道会超时，网络波动、关闭终端、WSL 退出或系统休眠也会中断。重新运行 Pinggy 后会获得新地址，必须把新域名和端口重新发给协助者。不要把免费临时隧道当作长期远程管理方案。

## 长期远程管理建议

Pinggy 免费 TCP 隧道适合一次性排障，不适合长期暴露 root。长期使用请改为：

- Tailscale、ZeroTier 或 WireGuard 等私有组网；
- SSH 公钥登录并关闭密码登录；
- 使用普通管理员账号登录后再 `sudo`；
- 限制来源 IP、启用登录审计和失败尝试防护；
- 面板、VNC、Docker Socket 和 Junimo 内部 API 不直接暴露公网。

每次临时维修都应遵循：**先本机验证 → 开隧道 → 私聊发送凭据与指纹 → 明确授权范围 → 完成后关隧道、停临时 sshd、回收 root 密码**。
