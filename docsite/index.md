---
layout: home

hero:
  name: "NetPanel"
  text: "家庭网络管理面板"
  tagline: 一个界面，管理所有网络需求。端口转发、内网穿透、异地组网、动态域名、反向代理——全部集中在一处。
  image:
    src: /logo.svg
    alt: NetPanel Logo
  actions:
    - theme: brand
      text: 快速开始
      link: /guide/installation
    - theme: alt
      text: 查看功能文档
      link: /features/port-forward

features:
  - icon: 🔀
    title: 端口转发
    details: 基于 Go 原生实现的 TCP/UDP 端口转发，支持监听指定 IP 和协议，零依赖、高性能。
    link: /features/port-forward
    linkText: 了解更多

  - icon: 🌐
    title: 内网穿透
    details: 支持 STUN 打洞（UPnP/NATMAP）和 FRP 客户端/服务端，轻松实现从外网访问内网设备。
    link: /features/stun
    linkText: 了解更多

  - icon: 🔗
    title: 异地组网
    details: 集成 EasyTier，支持多节点虚拟局域网，让不同地点的设备像在同一局域网内互联互通。
    link: /features/easytier-client
    linkText: 了解更多

  - icon: 📡
    title: 动态域名 (DDNS)
    details: 支持阿里云、腾讯云、Cloudflare、DNSPod 等主流服务商，IP 变化时自动更新 DNS 解析记录。
    link: /features/ddns
    linkText: 了解更多

  - icon: 🔒
    title: 域名证书
    details: 通过 ACME 协议自动申请和续期 Let's Encrypt / ZeroSSL 证书，支持 DNS 验证，全程自动化。
    link: /features/ssl-cert
    linkText: 了解更多

  - icon: 🌍
    title: 反向代理 (Caddy)
    details: 基于 Caddy 提供反向代理、静态文件服务、重定向和自动 HTTPS，配置简单，功能强大。
    link: /features/caddy
    linkText: 了解更多

  - icon: 🛡️
    title: 网络防护 (WAF)
    details: 集成 Coraza WAF，对 HTTP 流量进行规则过滤和拦截，有效防御常见 Web 攻击。
    link: /features/waf
    linkText: 了解更多

  - icon: ⏰
    title: 计划任务
    details: 基于 Cron 表达式的定时任务，支持执行 Shell 命令或发送 HTTP 请求，灵活自动化运维。
    link: /features/cron
    linkText: 了解更多

  - icon: 💡
    title: 回调系统
    details: 当 STUN 外网 IP/端口变化时，自动触发回调更新 Cloudflare、阿里云 ESA、腾讯云 EO 等配置。
    link: /features/callback
    linkText: 了解更多
---

<div class="vp-doc" style="max-width: 960px; margin: 0 auto; padding: 48px 24px;">

## 为什么选择 NetPanel？

如果你有一台 NAS、软路由或家里的小服务器，想从外网访问它，或者需要把几台不同地方的设备组成一个局域网，NetPanel 可以帮你把这些事情都管起来——**不用到处找工具，一个面板搞定一切**。

### 核心优势

| 特性 | 说明 |
|------|------|
| 🖥️ **统一管理** | 所有网络功能集中在一个 Web 界面，左侧导航，操作直观 |
| 🌏 **多语言支持** | 内置中文/英文国际化，界面语言随时切换 |
| 📦 **开箱即用** | 单二进制文件，无需安装依赖，解压即运行 |
| 🔧 **跨平台** | 支持 Linux、Windows、macOS，覆盖 x64 和 ARM64 架构 |
| 🐳 **容器友好** | 提供 Docker 镜像，支持 docker-compose 一键部署 |
| 🔓 **开源免费** | GPL-3.0 许可证，代码完全开放 |

### 支持平台

| 平台 | 架构 | 说明 |
|------|------|------|
| Linux | x86_64 | 主要测试平台，推荐 |
| Linux | ARM64 | 树莓派、NAS、ARM 软路由 |
| Windows | x86_64 | 完整支持 |
| Windows | ARM64 | 完整支持 |
| macOS | Intel (x86_64) | 完整支持 |
| macOS | Apple Silicon (ARM64) | 完整支持 |

### 技术栈

- **后端**：Go 1.21+，Gin，GORM + SQLite，JWT 认证
- **前端**：React 18，TypeScript，Ant Design 5，Vite，react-i18next
- **集成**：FRP、Caddy、Coraza WAF、DDNS-Go、lego (ACME)、pion/stun、EasyTier

> ⚠️ **注意**：项目目前仍在积极开发阶段，部分功能尚未完全实现。欢迎提交 [Issue](https://github.com/PIKACHUIM/NetPanel/issues) 或 [PR](https://github.com/PIKACHUIM/NetPanel/pulls)。

</div>
