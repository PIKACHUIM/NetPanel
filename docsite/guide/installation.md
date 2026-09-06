# 安装部署

本页面介绍如何在各平台上安装和运行 NetPanel。

## 系统要求

- **操作系统**：Linux、Windows 或 macOS
- **架构**：x86_64 (amd64) 或 ARM64
- **内存**：建议 256MB 以上
- **磁盘**：建议 100MB 以上可用空间（不含数据）
- **网络**：需要能访问互联网（用于 DDNS、证书申请等功能）

---

## 方式一：直接下载运行（推荐）

从 [GitHub Releases](https://github.com/PIKACHUIM/NetPanel/releases) 页面下载对应平台的压缩包。

### 下载包说明

| 平台 | 架构 | 文件名 |
|------|------|--------|
| Linux | x86_64 | `netpanel-linux-amd64.tar.gz` |
| Linux | ARM64 | `netpanel-linux-arm64.tar.gz` |
| Windows | x86_64 | `netpanel-windows-amd64.zip` |
| Windows | ARM64 | `netpanel-windows-arm64.zip` |
| macOS | Intel | `netpanel-darwin-amd64.tar.gz` |
| macOS | Apple Silicon | `netpanel-darwin-arm64.tar.gz` |

### Linux / macOS

```bash
# 下载（以 Linux amd64 为例）
wget https://github.com/PIKACHUIM/NetPanel/releases/latest/download/netpanel-linux-amd64.tar.gz

# 解压
tar -xzf netpanel-linux-amd64.tar.gz
cd netpanel-linux-amd64

# 运行
./netpanel
```

### Windows

```powershell
# 解压 netpanel-windows-amd64.zip 后，在目录内运行：
.\netpanel.exe
```

启动后，打开浏览器访问 `http://localhost:8080` 即可进入管理界面。

---

## 方式二：Linux 一键安装脚本

适用于 Linux 系统（包括 Ubuntu、Debian、CentOS、OpenWrt 等），自动下载最新版本并注册为系统服务。

```bash
# 使用 curl（推荐）
curl -fsSL https://raw.githubusercontent.com/PIKACHUIM/NetPanel/main/scripts/install.sh | sudo bash

# 或使用 wget
wget -qO- https://raw.githubusercontent.com/PIKACHUIM/NetPanel/main/scripts/install.sh | sudo bash
```

::: tip 需要 root 权限
安装脚本需要以 root 权限运行（`sudo bash install.sh`），用于注册 systemd 服务。
:::

### 脚本参数

```bash
bash install.sh [选项]
```

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `--version <ver>` | `latest` | 指定版本，如 `v1.0.0` |
| `--port <port>` | `8080` | 监听端口 |
| `--dir <path>` | `/opt/netpanel` | 安装目录 |
| `--no-service` | — | 不注册 systemd 服务 |

### 国内镜像加速

安装脚本自动检测网络并回退到国内镜像（ghproxy / fastgit），也可手动指定：

```bash
# 强制使用 ghproxy 加速
NETPANEL_MIRROR=ghproxy curl -fsSL https://raw.githubusercontent.com/.../install.sh | sudo bash

# 仅直连 GitHub（不使用镜像）
NETPANEL_MIRROR=direct curl -fsSL .../install.sh | sudo bash
```

### 服务管理

安装完成后，可使用以下命令管理服务：

```bash
# 启动
systemctl start netpanel

# 停止
systemctl stop netpanel

# 重启
systemctl restart netpanel

# 查看状态
systemctl status netpanel

# 查看实时日志
journalctl -u netpanel -f
```

---

## 方式三：Windows 安装脚本

适用于 Windows 系统，自动下载并注册为 Windows 服务。

```powershell
# 以管理员身份运行 PowerShell，执行：
irm https://raw.githubusercontent.com/PIKACHUIM/NetPanel/main/scripts/install.ps1 | iex
```

或下载脚本后本地运行：

```powershell
# 下载脚本
Invoke-WebRequest -Uri "https://raw.githubusercontent.com/PIKACHUIM/NetPanel/main/scripts/install.ps1" -OutFile "install.ps1"

# 运行（以管理员身份）
.\install.ps1 -Port 8080
```

### 国内镜像加速

```powershell
# 指定镜像源
$env:NETPANEL_MIRROR = "ghproxy"
.\install.ps1
```

---

## 方式四：Docker 部署

### 最小化部署（推荐新手）

仅需 Web 面板和基础功能（端口转发、DDNS、反向代理等），无需额外权限：

```bash
# 使用 docker-compose（推荐）
curl -fsSL https://raw.githubusercontent.com/PIKACHUIM/NetPanel/main/docker-compose.minimal.yml -o docker-compose.yml
docker compose up -d
```

或手动运行：

```bash
docker run -d \
  --name netpanel \
  -p 8080:8080 \
  -v ./data:/app/data \
  --restart unless-stopped \
  ghcr.io/netpanel/netpanel:latest
```

### 完整部署（含组网能力）

需要 EasyTier 异地组网、WireGuard VPN、Mesh 节点等 TUN 网络功能：

```bash
curl -fsSL https://raw.githubusercontent.com/PIKACHUIM/NetPanel/main/docker-compose.full.yml -o docker-compose.yml
docker compose up -d
```

或手动运行：

```bash
docker run -d \
  --name netpanel \
  -p 8080:8080 \
  -v ./data:/app/data \
  --cap-add NET_ADMIN \
  --cap-add SYS_MODULE \
  --device /dev/net/tun:/dev/net/tun \
  --sysctl net.ipv4.ip_forward=1 \
  --sysctl net.ipv6.conf.all.forwarding=1 \
  -e TZ=Asia/Shanghai \
  ghcr.io/netpanel/netpanel:latest
```

::: warning 关于 Docker 网络功能
EasyTier 异地组网和部分网络功能需要 `NET_ADMIN` 权限和 TUN 设备支持。如果不使用这些功能，可以使用最小化部署，无需额外权限。
:::
      - SYS_MODULE
    devices:
      - /dev/net/tun:/dev/net/tun
    sysctls:
      - net.ipv4.ip_forward=1
      - net.ipv6.conf.all.forwarding=1
```

启动：

```bash
docker-compose up -d
```

::: warning 关于 Docker 网络功能
EasyTier 异地组网和部分网络功能需要 `NET_ADMIN` 权限和 TUN 设备支持。如果不使用这些功能，可以去掉相关配置。
:::

---

## 方式五：从源码构建

需要 **Go 1.21+** 和 **Node.js 20+**。

```bash
# 1. 克隆仓库
git clone https://github.com/PIKACHUIM/NetPanel.git
cd netpanel

# 2. 构建前端
cd webpage
npm install
npm run build
cd ..

# 3. 构建后端（前端产物会自动嵌入到二进制中）
cd backend
go build -o ../netpanel .
cd ..

# 4. 运行
./netpanel
```

---

## 启动参数

```bash
./netpanel [选项]
```

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `-port` | `8080` | HTTP 监听端口 |
| `-data` | `./data` | 数据目录（存放数据库和配置文件） |
| `--service` | — | 以服务模式运行（由系统服务管理器调用） |

**示例：**

```bash
# 修改端口和数据目录
./netpanel -port 9090 -data /var/lib/netpanel

# 后台运行（Linux）
nohup ./netpanel -port 8080 -data ./data > netpanel.log 2>&1 &
```

---

## 首次访问

安装完成后，打开浏览器访问：

```
http://localhost:8080
```

或将 `localhost` 替换为服务器的实际 IP 地址。

::: info 默认账号
首次启动时，系统会引导你创建管理员账号。请妥善保管账号密码。
:::

---

## 开发模式

如果你想参与开发，可以分别启动前后端开发服务器：

```bash
# 终端 1：启动后端（开发模式）
cd backend
go run . -port 8080

# 终端 2：启动前端开发服务器
cd webpage
npm run dev
```

前端开发服务器默认运行在 `http://localhost:5173`，已配置代理将 `/api` 请求转发到后端 `8080` 端口。
