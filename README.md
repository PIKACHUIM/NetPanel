<div align="center">

# 🌐 NetPanel

**面向家庭和小型网络环境的一体化内网穿透与网络管理面板**

[![License](https://img.shields.io/badge/license-GPL--3.0-blue.svg)](LICENSE)
[![Go Version](https://img.shields.io/badge/Go-1.21+-00ADD8?logo=go)](https://golang.org/)
[![React](https://img.shields.io/badge/React-18-61DAFB?logo=react)](https://reactjs.org/)
[![Platform](https://img.shields.io/badge/platform-Linux%20%7C%20Windows%20%7C%20macOS-lightgrey)](#支持平台)

</div>

---

## 📖 简介

如果你有一台 NAS、软路由或家里的小服务器，想从外网访问它，或者需要把几台不同地方的设备组成一个局域网，**NetPanel** 可以帮你把这些事情都管起来——端口转发、内网穿透、异地组网、动态域名、反向代理……全部在一个界面里配置，不用到处找工具。

> ⚠️ 目前项目还在开发阶段，部分功能尚未完全实现，欢迎提 [Issue](../../issues) 或 [PR](../../pulls)。

---

## ✨ 功能概览

### 🔗 网络穿透与组网

| 功能 | 说明 |
|------|------|
| **端口转发** | 基于 Go 原生实现的 TCP/UDP 端口转发，支持监听指定 IP 和协议，内置 HTTP/SOCKS5 代理 |
| **内网穿透（STUN）** | 利用 STUN 协议打洞，支持 UPnP、NATMAP，IP 变化时可触发回调 |
| **内网穿透（FRP）** | 集成 FRP 客户端，支持 TCP/UDP/HTTP/HTTPS/STCP/XTCP 等代理类型 |
| **FRP 服务端** | 同时支持运行 frps，方便自建穿透服务 |
| **内网穿透（NPS）** | 集成 NPS 客户端与服务端，提供更多穿透方案选择 |
| **内网穿透（CF）** | 集成 Cloudflare Tunnel（cloudflared），支持 Quick / Named / Token 三种模式，免公网 IP、边缘直接接入 |
| **异地组网（EasyTier）** | 管理 EasyTier 客户端进程，支持多节点组网，配置虚拟 IP 和网络密码 |
| **EasyTier 服务端** | 支持运行 EasyTier standalone 服务端 |
| **WireGuard** | 内置 WireGuard 组网支持，配置简单，性能优异 |
| **Mesh 节点** | 支持 Mesh 网络节点管理，实现多点互联 |

### 🌍 域名与证书

| 功能 | 说明 |
|------|------|
| **动态域名（DDNS）** | 支持阿里云、腾讯云、Cloudflare、DNSPod 等主流服务商，自动更新解析记录 |
| **域名账号** | 统一管理各服务商的 API 密钥，供 DDNS、证书、回调等功能复用 |
| **域名解析** | 直接在面板里增删改查 DNS 解析记录，无需登录各服务商控制台 |
| **域名证书** | 通过 ACME 协议自动申请和续期 Let's Encrypt / ZeroSSL 证书，支持 DNS 验证 |

### 🛡️ 网站与安全

| 功能 | 说明 |
|------|------|
| **网站服务（Caddy）** | 基于 Caddy 提供反向代理、静态文件服务、重定向、URL 跳转，支持自动 HTTPS |
| **网络防护（Coraza WAF）** | 集成 Coraza WAF，可对 HTTP 流量进行规则过滤和拦截 |
| **防火墙管理** | 系统防火墙规则管理，支持入站/出站规则配置 |
| **访问控制** | 支持 IP 黑白名单，可以只允许特定 IP 访问，或者屏蔽某些 IP |

### 🔧 辅助功能

| 功能 | 说明 |
|------|------|
| **网络唤醒（WOL）** | 发送 Magic Packet 远程唤醒局域网内的设备 |
| **解析服务（DNSMasq）** | 内置 DNS 服务，支持自定义解析规则和上游 DNS |
| **计划任务** | 基于 Cron 表达式的定时任务，支持执行 Shell 命令或 HTTP 请求 |
| **网络存储** | 对外提供 WebDAV、SFTP 访问，方便远程管理文件 |
| **IP 地址库** | 批量管理和查询 IP 归属地信息 |
| **系统日志** | 统一的系统日志查看与管理 |

### 🔔 回调系统

当 STUN 穿透的外网 IP 或端口发生变化时，可以自动触发回调，支持：

- 🔄 更新 **Cloudflare** 回源端口
- 🔄 更新 **阿里云 ESA** 规则
- 🔄 更新 **腾讯云 EO** 规则
- 🔄 发送自定义 **WebHook** 请求

---

## 🏗️ 技术栈

<table>
<tr>
<td><b>后端</b></td>
<td>Go 1.21 · Gin · GORM + SQLite · JWT 认证</td>
</tr>
<tr>
<td><b>前端</b></td>
<td>React 18 · TypeScript · Ant Design 5 · Vite · react-i18next</td>
</tr>
<tr>
<td><b>桌面端</b></td>
<td>Electron（跨平台桌面应用封装）</td>
</tr>
<tr>
<td><b>集成库</b></td>
<td>FRP · NPS · Caddy · Coraza · DDNS-Go · lego (ACME) · pion/stun · miekg/dns · WireGuard</td>
</tr>
<tr>
<td><b>EasyTier</b></td>
<td>Rust 编写，通过命令行方式管理进程，构建时自动下载对应平台二进制</td>
</tr>
</table>

---

## 🚀 快速开始

### 方式一：直接下载运行

从 [Releases](../../releases) 页面下载对应平台的压缩包，解压后直接运行：

```bash
# Linux / macOS
tar -xzf netpanel-linux-amd64.tar.gz
cd netpanel-linux-amd64
./netpanel
```

```powershell
# Windows，解压后在目录内运行
.\netpanel.exe
```

默认监听 `8080` 端口，打开浏览器访问 `http://localhost:8080` 即可。

**常用参数：**

```bash
./netpanel -port 8080 -data ./data
```

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `-port` | `8080` | HTTP 监听端口 |
| `-data` | `./data` | 数据目录（存放数据库和配置文件） |

### 方式二：Docker 部署

```bash
# 使用 docker-compose
docker-compose up -d

# 或直接运行
docker run -d \
  --name netpanel \
  -p 8080:8080 \
  -v ./data:/app/data \
  --restart unless-stopped \
  netpanel:latest
```

### 方式三：安装为系统服务

```bash
# Linux（使用安装脚本）
curl -fsSL https://raw.githubusercontent.com/your-username/netpanel/main/scripts/install.sh | bash

# Windows（使用 PowerShell 脚本）
.\scripts\install.ps1
```

### 方式四：从源码构建

需要 Go 1.21+ 和 Node.js 20+。

```bash
# 克隆仓库
git clone https://github.com/your-username/netpanel.git
cd netpanel

# 构建前端
cd webpage
npm install
npm run build
cd ..

# 构建后端（包含前端静态文件）
cd backend
go build -o ../netpanel .
cd ..

# 运行
./netpanel
```

---

## 🗂️ 目录结构

```
netpanel/
├── backend/                # Go 后端
│   ├── main.go             # 程序入口
│   ├── api/                # 路由和 Handler
│   │   ├── handlers/       # 各功能 API 处理器
│   │   ├── middleware/     # 中间件（认证等）
│   │   └── router.go       # 路由注册
│   ├── model/              # 数据库模型
│   ├── service/            # 各功能服务管理器
│   │   ├── portforward/    # 端口转发
│   │   ├── stun/           # STUN 穿透
│   │   ├── frp/            # FRP 客户端/服务端
│   │   ├── nps/            # NPS 客户端/服务端
│   │   ├── easytier/       # EasyTier 组网
│   │   ├── wireguard/      # WireGuard VPN
│   │   ├── meshnode/       # Mesh 节点管理
│   │   ├── ddns/           # 动态域名
│   │   ├── caddy/          # 反向代理
│   │   ├── cert/           # SSL 证书
│   │   ├── firewall/       # 防火墙管理
│   │   ├── access/         # 访问控制
│   │   ├── callback/       # 回调系统
│   │   ├── cron/           # 计划任务
│   │   ├── storage/        # 网络存储
│   │   ├── dnsmasq/        # DNS 服务
│   │   ├── wol/            # 网络唤醒
│   │   └── syslog/         # 系统日志
│   ├── pkg/                # 公共工具（日志、配置等）
│   └── embed/              # 嵌入的前端静态文件
├── webpage/                # React 前端
│   └── src/
│       ├── pages/          # 各功能页面
│       ├── components/     # 公共组件
│       ├── api/            # 接口请求封装
│       └── i18n/           # 国际化（中文/英文）
├── desktop/                # Electron 桌面端
├── modules/                # 集成的第三方模块
│   ├── EasyTier/
│   ├── caddy/
│   ├── coraza/
│   ├── frp/
│   ├── nps/
│   └── ...
├── scripts/                # 安装和服务管理脚本
│   ├── install.sh          # Linux 安装脚本
│   ├── install.ps1         # Windows 安装脚本
│   └── setup.iss           # Windows 安装包配置
├── docsite/                # 文档站点（VitePress）
├── Dockerfile
├── docker-compose.yml
└── .github/
    └── workflows/          # CI/CD 构建脚本
```

---

## 💻 支持平台

| 平台 | 架构 | 说明 |
|------|------|------|
| Linux | x86_64 (amd64) | 主要测试平台 |
| Linux | ARM64 | 树莓派、NAS 等 |
| Linux | ARMv7 | 低功耗嵌入式设备 |
| Windows | x86_64 (amd64) | 支持安装包和便携版 |
| Windows | ARM64 | Surface 等 ARM 设备 |
| macOS | Intel (amd64) | |
| macOS | Apple Silicon (arm64) | M 系列芯片 |

> 发布包内已包含对应平台的 EasyTier 二进制文件，无需单独下载。

---

## 🛠️ 开发指南

### 环境要求

- Go 1.21+
- Node.js 20+
- Git

### 启动开发环境

```bash
# 启动后端（开发模式）
cd backend
go run . -port 8080

# 启动前端开发服务器（另开一个终端）
cd webpage
npm install
npm run dev
```

前端开发服务器默认运行在 `http://localhost:5173`，已配置代理将 `/api` 请求转发到后端。

### 构建发布包

```bash
# 使用 Makefile 构建所有平台
make all

# 构建特定平台
make linux-amd64
make windows-amd64
```

### 代码规范

- 后端遵循 Go 惯用法，使用 `go fmt` 和 `golangci-lint` 检查
- 前端使用 TypeScript 严格模式，遵循 ESLint 规则
- 提交前请确保所有测试通过

---

## 🤝 贡献

欢迎提交 Issue 和 Pull Request！

1. Fork 本仓库
2. 创建你的功能分支 (`git checkout -b feature/amazing-feature`)
3. 提交你的更改 (`git commit -m 'feat: add amazing feature'`)
4. 推送到分支 (`git push origin feature/amazing-feature`)
5. 开启一个 Pull Request

---

## 📄 许可证

本项目基于 [GPL-3.0](LICENSE) 许可证开源。
