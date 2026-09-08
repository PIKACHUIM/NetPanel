<div align="center">

# 🌐 NetPanel

**面向家庭与小型网络环境的一体化内网穿透 · 异地组网 · 反向代理管理面板**

*An all-in-one panel for NAT traversal, site-to-site networking & reverse proxy — built for home labs, NAS and small networks.*

[![License](https://img.shields.io/badge/license-AGPL--3.0-orange.svg)](LICENSE)
[![Go Version](https://img.shields.io/badge/Go-1.25+-00ADD8?logo=go)](https://golang.org/)
[![React](https://img.shields.io/badge/React-18-61DAFB?logo=react)](https://reactjs.org/)
[![Ant Design](https://img.shields.io/badge/Ant%20Design-5-1677FF?logo=antdesign)](https://ant.design/)
[![Vite](https://img.shields.io/badge/Vite-7-646CFF?logo=vite)](https://vitejs.dev/)
[![Platform](https://img.shields.io/badge/platform-Linux%20%7C%20Windows%20%7C%20macOS-lightgrey)](#支持平台)
[![PRs Welcome](https://img.shields.io/badge/PRs-welcome-brightgreen)](#贡献)

</div>

---

## 📖 简介 / Introduction

如果你有一台 NAS、软路由或家里的小服务器，想从外网访问它，或者需要把几台不同地方的设备组成一个局域网——**NetPanel** 可以帮你把这些事情都管起来：

端口转发、内网穿透（FRP / NPS / STUN / Cloudflare Tunnel）、异地组网（EasyTier / WireGuard）、动态域名、SSL 证书、反向代理、WAF 防护……全部在一个界面里配置，**不用再东拼西凑多个工具**。

> NetPanel is a unified web panel that brings port forwarding, NAT traversal (FRP / NPS / STUN / Cloudflare Tunnel), site-to-site VPN (EasyTier / WireGuard), DDNS, SSL certificates, reverse proxy and WAF into **one single UI** — no more juggling half a dozen tools.

> ⚠️ 项目仍在开发阶段，部分功能尚未完全实现；欢迎提 [Issue](../../issues) 或 [Pull Request](../../pulls) 一起完善。
>
> ⚠️ This project is still under active development — issues and pull requests are very welcome.

📚 详细使用与配置文档见 [docsite/](docsite/)（VitePress 文档站）。

---

## ✨ 功能总览 / Feature Overview

### 🤖 智能管理（AI / MCP）

| 功能 | 说明 |
|------|------|
| **MCP 服务端** MCP Server | 遵循 Model Context Protocol 的标准服务端，LLM / AI Agent 可通过标准协议查询探测配置、隧道健康状态并执行异常诊断 |
| **智能线路选择** Line Auto-Selection | 基于 selector + linereg 的自动选线引擎：探测多条线路，自动择优，支持锁线 / 失败阈值 / 工具过滤 |
| **并发测速弹窗** Speed Test | 对穿透服务线路即时并发测速，一键比较各条线路的延迟与可用性 |
| **探测监控与告警** Monitor & Alerts | 服务/线路探测失败自动告警，支持多种告警规则与通知 |
| **AI 辅助** AI Assistant | 内置 AI 接入层，支持面向运维场景的智能问答与操作建议 |

### 🔗 网络穿透与组网

| 功能 | 说明 |
|------|------|
| **端口转发** Port Forwarding | 基于 Go 原生实现的 TCP/UDP 端口转发，支持监听指定 IP 和协议，内置 HTTP/SOCKS5 代理 |
| **内网穿透（STUN）** NAT Traversal | 利用 STUN 协议打洞，支持 UPnP、NATMAP，IP 变化时可触发回调 |
| **内网穿透（FRP）** | 集成 FRP 客户端，支持 TCP/UDP/HTTP/HTTPS/STCP/XTCP 等代理类型 |
| **FRP 服务端** frps | 同时支持运行 frps，方便自建穿透服务 |
| **内网穿透（NPS）** | 集成 NPS 客户端与服务端，提供更多穿透方案选择 |
| **内网穿透（CF）** Cloudflare Tunnel | 集成 cloudflared，支持 Quick / Named / Token 三种模式，免公网 IP、边缘直接接入 |
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
| **网络防护（WAF）** | 集成 Coraza WAF 引擎，攻击事件实时分析、态势统计、IP 封禁联动 |
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

## 🏗️ 技术栈 / Tech Stack

| 层 | 技术 |
|------|------|
| **后端** Backend | Go 1.25 · Gin · GORM + SQLite · JWT 认证 |
| **前端** Frontend | React 18 · TypeScript · Ant Design 5 · Vite 7 · react-i18next |
| **桌面端** Desktop | Electron（跨平台桌面应用封装） |
| **集成库** Integrations | FRP · NPS · Caddy · Coraza · DDNS-Go · lego (ACME) · pion/stun · miekg/dns · WireGuard |
| **EasyTier** | Rust 编写，通过命令行方式管理进程，构建时自动下载对应平台二进制 |

---

## 🚀 快速开始 / Quick Start

> 最新发布版见 [Releases](../../releases)（当前 v0.3.0）。默认监听 `8080` 端口，浏览器访问 `http://localhost:8080` 即可。

### 方式一：直接下载运行（推荐）

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
curl -fsSL https://raw.githubusercontent.com/PIKACHUIM/NetPanel/main/scripts/install.sh | bash

# Windows（使用 PowerShell 脚本）
.\scripts\install.ps1
```

### 方式四：从源码构建

需要 Go 1.25+ 和 Node.js 20.19+（CI 环境：Go 1.25 / Node 24）。

```bash
# 克隆仓库
git clone https://github.com/PIKACHUIM/NetPanel.git
cd NetPanel

# 构建前端
cd webpage
npm install
npm run build
cd ..

# 构建后端（前端静态文件会嵌入到后端二进制中）
cd backend
go build -o ../netpanel .
cd ..

# 运行
./netpanel
```

---

## 🗂️ 目录结构 / Repo Layout

```
NetPanel/
├── backend/                # Go 后端
│   ├── main.go             # 程序入口（嵌入前端 dist）
│   ├── api/                # 路由与 Handler（handlers / middleware / router.go）
│   ├── model/              # 数据库模型（GORM + SQLite）
│   ├── service/            # 功能服务：portforward / stun / frp / nps / cftunnel /
│   │                       #   easytier / wireguard / meshnode / caddy / cert /
│   │                       #   firewall / access / waf / mcp / linereg / selector /
│   │                       #   tunservice / monitor / ddns / dnsmasq / cron /
│   │                       #   storage / wol / syslog / ai / oauth / callback ...
│   ├── pkg/                # 公共库（config / logger / secret / sysutil / utils ...）
│   └── embed/              # 前端静态文件嵌入（embed.go + dist/）
├── webpage/                # React 前端
│   └── src/
│       ├── pages/          # 各功能页面
│       ├── components/     # 公共组件
│       ├── api/            # 接口请求封装
│       └── i18n/           # 国际化（中文/英文）
├── desktop/                # Electron 桌面端
├── docsite/                # VitePress 文档站（含自动化部署到 GitHub Pages）
├── scripts/                # 安装与服务管理脚本（install.sh / install.ps1 / setup.iss ...）
├── Makefile                # 跨平台构建入口（make linux-amd64 / windows-amd64 ...）
├── Dockerfile
├── docker-compose.yml
└── .github/workflows/      # CI/CD：PR 门禁（编译+测试+前端类型检查）、发布、文档部署
```

---

## 💻 支持平台 / Supported Platforms

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

## 🛠️ 开发指南 / Development

### 环境要求

- Go 1.25+（参考 `backend/go.mod`，CI 使用 Go 1.25）
- Node.js 20.19+（CI 使用 Node 24）
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

- 后端遵循 Go 惯用法，PR 门禁会执行 `go vet`、`go build` 与单元测试（Go 1.25）
- 前端使用 TypeScript 严格模式，PR 门禁会执行类型检查与生产构建
- 提交前请确保本地能通过 `go vet ./... && go test ./...` 与前端构建
- UI 文案国际化：新增界面文案请同时补充 `webpage/src/i18n` 中的中文与英文条目

---

## 🏛️ 架构亮点：统一安全入口与 IPv6 场景 / Architecture Highlights

### 目标架构

```
            ┌──────────────── 统一入口安全网关 ────────────────┐
公网 ──▶ [防火墙 IPv4+IPv6] ──▶ [WAF(Coraza)] ──▶ [Caddy TLS] ──▶ 反代/选线 ──▶ 内网服务
                                                        │
IPv6 直连 ──▶ [DDNS AAAA] ──▶ [防火墙 IPv6 放行] ──▶ [Caddy 双栈] ──▶ 内网服务
IPv6 客户端 ──▶ [DNS64 + NAT64 网关] ──▶ IPv4 服务
```

NetPanel 将防火墙、WAF、SSL 证书、Caddy 反代集中为统一安全入口，内网服务无需逐个暴露端口。

### 场景：外网 IPv4 客户端访问「家里只有公网 IPv6、无公网 IPv4」的内网服务

很多家庭：IPv4 为 CGNAT（无公网 IPv4），但有公网 IPv6；内网服务仍只监听 IPv4；外部访客是纯 IPv4 设备。

> **关键约束**：纯 IPv4 客户端无法路由到 IPv6 地址，公网 IPv6 不能直接作为该场景入口。必须通过「IPv4 可达中转 + 反向穿透（家里主动出站）」。

| 方案 | 路径 | 说明 |
|---|---|---|
| Cloudflare Tunnel（cftunnel） | 外部 IPv4 → CF 边缘（IPv4） → 家里出站隧道 → 内网 IPv4 | 免费，无需 VPS |
| VPS + frp/nps | 外部 IPv4 → VPS IPv4:port → 家里出站隧道 → 内网 IPv4 | 自控带宽，需 VPS |

家里公网 IPv6 的价值：① 出站链路走 IPv6（绕开 CGNAT 出站的不稳定）；② 未来有 IPv6 的外部访客可直接 IPv6 直达（零中转，最快）。

### 代理限制与要求

| 中转方式 | 限制 | 说明 |
|---|---|---|
| Cloudflare Tunnel（免费） | 单请求体约 **100MB 上限**（大文件受限，以官方文档为准） | 适合 Web/API/中小文件，不适合大文件/视频直传 |
| Cloudflare Tunnel（免费） | 带宽受 CF Fair Use 约束，吞吐看边缘节点 | 国内访问多走 HK/SG 边缘 |
| VPS + frp/nps | 无协议层文件大小限制 | 瓶颈在 VPS 带宽与流量费（frp 可配 `bandwidth_limit`） |
| 任意中转 | 单点故障，流量经第三方/自建节点 | 时延 = 客户端→中转 + 中转→家 |

### 速度优化建议

1. **出站走 IPv6**：家里客户端到中转的连接优先 IPv6，绕开 CGNAT NAT 转发开销与波动。
2. **中转就近**：CF 走 Anycast 就近边缘；自建 VPS 选离家里和访客都近的机房。
3. **保持双栈**：中转节点开启 IPv4+IPv6，未来 IPv6 访客可直达家中（零中转）。
4. **大文件避开 CF**：>100MB 的文件/备份走自建 VPS+frp 或 IPv6 直连。

---

## 🤝 贡献 / Contributing

欢迎提交 Issue 和 Pull Request！无论是新功能、Bug 修复、文档改进还是使用反馈，我们都非常欢迎。

*Issues and pull requests are always welcome — new features, bug fixes, docs improvements and usage feedback.*

建议流程：

1. Fork 本仓库
2. 创建你的功能分支：`git checkout -b feat/your-feature`
3. 提交你的更改（见下方规范）
4. 推送到你的分支：`git push origin feat/your-feature`
5. 在 GitHub 上开启一个 Pull Request，目标分支 `main`

**一些约定：**

- **提交信息**：使用 Conventional Commits（`feat` / `fix` / `docs` / `refactor` …），中文描述为主，可附简短英文
- **小而聚焦的 PR**：一个 PR 只做一件事，便于 review 与合并
- **PR 门禁**：CI 会自动执行后端 `go vet` / 测试与前端类型检查 / 构建，请确保通过
- **新增文案**：记得同步 `webpage/src/i18n` 下的中文与英文条目
- 实现大功能前，欢迎先开 Issue 讨论设计，避免返工

---

## 📄 许可证 / License

本项目基于 [AGPL-3.0](LICENSE) 许可证开源。

<div align="center">

**⭐ 如果 NetPanel 对你有帮助，欢迎 Star / Fork / 贡献代码！**

</div>




