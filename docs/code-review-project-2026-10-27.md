# NetPanel 全项目审查报告

## 1. 审查信息

| 项目 | 结果 |
|---|---|
| 审查日期 | 2026-10-27 |
| 范围 | Go 后端、React/TypeScript 前端、API 权限、数据模型、部署配置、产品功能与用户旅程 |
| 方式 | 源码静态审查、API/权限路径追踪、攻击场景推演、官方产品与安全资料检索 |
| 动态验证 | 未启动服务、未向任何部署实例发包、未执行破坏性攻击；因此不是动态渗透测试或依赖 CVE 扫描 |
| 结论 | **暂不建议将当前版本直接暴露到公网供多用户使用；优先修复凭证暴露、密钥失败降级、对象级授权和存储只读问题** |
| 工作区状态 | Git 状态显示 `webpage/package-lock.json` 尚有未解决合并冲突、`.wt/` 未跟踪；本次未修改它们 |

## 2. 总体判断

NetPanel 的目标明确：让 NAS、家庭网络和小型服务器用户在一个面板管理穿透、组网、域名证书、反向代理、安全和运维。当前已覆盖的功能面很宽，服务按模块拆分，Go + Gin/GORM/SQLite 与 React/Ant Design 的组合适合单机、自托管和小规模场景。JWT 管理员路由、登录限流、部分敏感字段 `Secret` 脱敏、IPDB 导入的 SSRF 防护等实现是积极基础。

主要问题不是“功能不够多”，而是**功能扩张快于安全边界、端到端流程与体验收口**：若干普通登录用户可取回高敏感凭据；部分角色没有被 API 细粒度执行；若干关键操作前后端 HTTP 契约对不上；运维首页在请求失败时给出误导状态。项目自述仍处于开发阶段，与源码中多个已实现的模块并存，发布质量需要以端到端验收而非“页面/路由已存在”判断。

### 维度评分（静态审查估分，不是运行基准）

| 维度 | 评分 | 说明 |
|---|---:|---|
| 架构与可维护性 | 7/10 | 模块化与统一 API 层不错；功能面过宽，多个隧道/组网模型重叠 |
| 功能闭环与产品逻辑 | 5/10 | 能力齐全，但“选方案→配置→启动→验证→安全访问”尚未形成统一旅程 |
| 前端视觉与信息架构 | 6/10 | Ant Design 提供基础一致性；导航层级、密度、移动端和自定义样式仍需收敛；未进行浏览器像素级检查 |
| 前端逻辑与可靠性 | 5/10 | 存在已发现的下载契约、初始化流程、SSE 分块处理、状态误报问题 |
| 性能与扩展性 | 6/10 | SQLite WAL、连接池与懒加载有利于单机；大量轮询/全量读取会限制增长 |
| 安全性 | 4/10 | 管理员边界部分较好，但凭证泄露、密钥明文降级、AI 对象隔离、ReadOnly 落空等为高优先级问题 |
| 验证与质量门禁 | 5/10 | 后端已有一定测试和 CI；未见前端测试脚本/测试文件，当前工作区还存在 lockfile 冲突 |

## 3. 关键风险与问题

### P1：立即修复

#### 3.1 普通登录用户可读取多个未脱敏凭证/私钥

认证路由组 `auth` 只要求 JWT 登录，相关列表接口直接 JSON 序列化数据库模型；普通 `string` 敏感字段不会被 `model.Secret` 掩码保护。

已确认的代表路径：

- `GET /api/v1/monitor/servers` 和详情直接返回 `MonitorServer`，其中 `AgentToken` 是普通 JSON 字段：`backend/api/router.go:550-557`、`backend/api/handlers/monitor.go:61-90`、`backend/model/models.go:1407-1415`。
- `GET /api/v1/domain/certs` 返回证书模型，其中 `KeyContent` 为普通字符串：`backend/api/router.go:325-338`、`backend/api/handlers/domain.go:1181-1185`、`backend/model/models.go:835-840`。
- `GET /api/v1/frpc` 返回代理配置，`HTTPPassword`、`SecretKey` 等为普通字符串：`backend/api/router.go:161-174`、`backend/api/handlers/frp.go:74-80`、`backend/model/models.go:187-193`。
- `GET /api/v1/caddy` 暴露 `BasicAuthPasswd`：`backend/api/router.go:281-289`、`backend/model/models.go:711-712`。
- `GET /api/v1/wireguard/:id/peers` 返回 `PresharedKey`：`backend/api/router.go:253-268`、`backend/model/models.go:1249-1260`。
- `StorageConfig.Password` 是普通字段，但列表 handler 明确将其替换为掩码，因此未把存储列表判定为明文 API 泄露；它仍未通过 `Secret` 加密落库，且 `GET /api/v1/storage` 仅要求登录。参考 `backend/model/models.go:934-945`、`backend/api/router.go:382-389`、`backend/api/handlers/misc.go:412-422`。

**影响：**viewer/普通用户凭据泄露可扩展到隧道接入、远程 Agent、TLS 私钥或 VPN peer。不能只靠前端隐藏字段来修复。

**建议：**所有 API 响应使用 DTO 白名单；默认不回传秘密，单独设置/轮换秘密；敏感读取如确有需要要单独授权、审计和强认证；新增低权限角色的列表、详情、导出响应断言测试。同步盘点所有 `json:"...password/token/key..."` 的普通字符串字段。

#### 3.2 加密密钥初始化失败会继续运行并明文写入 `Secret`

`backend/main.go:190-195` 在 `crypto.InitKey()` 失败时仅警告“敏感配置将以明文存储”并继续启动；`backend/model/secret.go:78-86` 在加密模块未初始化时直接返回原文。未配置/配置错误时保存使用 `Secret` 的字段就会明文入库。部分凭证（例如 `StorageConfig.Password`）本身不是 `Secret`，不受该加密保护。

**建议：**默认失败关闭：生产模式下密钥缺失或无效即拒绝启动，或至少拒绝创建/更新任何敏感字段；清点并迁移已写入的明文数据；部署示例生成稳定密钥并限制数据库、密钥、备份文件权限。不要把明文兼容回退当成健康运行模式。

#### 3.3 AI 对话 API 缺少用户归属检查，存在水平越权

AI 对话和助手路由挂在认证组，handler 按请求里的 ID 读取、修改、删除、导出、继续发送消息；对话模型没有用户归属字段，服务层按 ID 全局操作。参考 `backend/api/router.go:519-532`、`backend/api/handlers/ai.go:112-188`、`backend/service/ai/chat.go:12-50`、`backend/model/models.go:1306-1338`。

**攻击条件：**用户 A 已登录，取得/猜到用户 B 的对话 ID 后尝试读取或操作该对话。若产品定义为共享对话，也应显式声明共享，并仅让合适角色读写。

**建议：**增加 `OwnerUserID`/共享范围；每次对象访问在服务端校验所有者或 ACL；不同用户不能凭 ID 读取、删除或追加消息。对助手 CRUD 也明确是否管理员专属或团队共享。

#### 3.4 WebDAV/SFTP 的 `ReadOnly` 配置未真正限制写操作

`StorageConfig` 有 `ReadOnly` 字段；SMB 启动配置会使用它，但 WebDAV 在 Basic Auth 后直接交给文件系统处理，SFTP 也未见只读 filesystem/操作过滤。见 `backend/model/models.go:934-945`、`backend/service/storage/manager.go:121-138,164-184,281-294,372-403`。

**攻击条件：**管理员将共享配置成 `ReadOnly=true` 并提供账号，使用该账号执行上传、覆盖、删除等写请求。按当前项目代码，WebDAV/SFTP 未应用该开关。

**建议：**在协议层强制只读，不依赖 UI 字段或客户端约定；覆盖 PUT/CREATE/REMOVE/RENAME/MKDIR 等写操作的集成测试。并对 SFTP `..`、绝对路径、符号链接，以及 WebDAV 符号链接越界做隔离测试；当前对底层库 jail 行为未作动态验证。

#### 3.5 首次安装时公开的 Setup 接口可被抢先注册管理员

`POST /api/v1/init/setup` 是公开路由：`backend/api/router.go:97-105`。服务默认监听所有网络接口：`backend/main.go:390-417`。在首个管理员创建前，能访问端口的攻击者可先调用 setup 建立管理员。初始化后的重复调用有保护，因此风险窗口仅存在于新部署未完成初始化时。

**建议：**新实例默认只监听 loopback，或启动后要求一次性安装令牌/本机确认；初始化成功后关闭 setup 能力并记录安全事件；部署说明明确首启不得直连公网。

#### 3.6 默认 HTTP 监听与高权限宿主能力需要部署收口

主服务监听 `:<port>` 且 README 默认 8080/HTTP：`backend/main.go:390-417`、`README.md:116-119`。完整 Compose 还赋予 `NET_ADMIN`、`SYS_MODULE` 与 TUN 设备（`docker-compose.full.yml:40-48`）；Dockerfile 目前仍以非特权 USER 未启用为已知关注点（`Dockerfile:75-100`）。面板同时管理防火墙、隧道、Shell 任务和宿主网络，凭据被窃取时影响面远高于普通 Web 面板。

**建议：**生产部署默认 TLS（本体 HTTPS 或强制可信反向代理）、明确可信代理与安全 Cookie 配置；按功能拆分容器能力和权限；无必要不启用完整 Compose 权限；提供启动时安全配置检查和暴露端口清单。

### P2：高优先级修复

#### 3.7 登出清除 Cookie，但不撤销 JWT

`Logout` 只调用 `ClearSessionCookie`，JWT 本身有效期最长约 24 小时，校验 token version 的机制也没有在 Logout 时递增：`backend/api/handlers/auth.go:135-142`、`backend/api/middleware/auth.go:39-52,101-128`。前端移除本地 token 不会使已复制的 Bearer token 失效。

**建议：**提供服务端 token 撤销/会话 ID 黑名单、刷新令牌轮换，或采用短时 access token；明确登出、改密、禁用用户和角色回收的一致失效语义。

#### 3.8 管理员降级可能没有真正移除管理员权限

用户更新角色时仅改 `roles`，未同步清除 `is_admin`；鉴权仍从数据库 `is_admin` 判定管理员。参考 `backend/api/handlers/admin.go:273-317`、`backend/api/middleware/auth.go:120-152`。管理员将用户角色改成 viewer 后，用户可能仍保留 AdminOnly 权限。

**建议：**规范角色数据源，禁止 `roles` 与 `is_admin` 双重事实；角色更改应立即撤销旧 token，并增加管理员降权端到端测试。

#### 3.9 OAuth 登录后的 Token 放在 URL 中

OAuth callback 把 JWT 加到 redirect URL 的 `token` 查询参数（`backend/api/handlers/oauth.go:150-162`）。URL 可能被浏览器历史、代理/服务日志或后续 Referer 链路记录，实际暴露取决于部署。

**建议：**用一次性短期 code 换 token、HttpOnly Secure Cookie 或同源回调页面 postMessage；避免长期 bearer token 出现在 URL。

#### 3.10 用户配置 URL 的 SSRF 与资源耗尽面不统一

IPDB URL 导入使用了 `safehttp`；但管理员可配置的 Mesh ping 与 Cron HTTP 请求等路径仍可由后端请求任意目标，需检查重定向、DNS rebinding、IPv4/IPv6 内网地址以及响应大小/超时。`backend/api/router.go:497-507`、`backend/api/handlers/meshnode.go:198-212`、`backend/service/meshnode/manager.go:838-856`、`backend/service/cron/manager.go:216-237`、对照 `backend/pkg/safehttp/safehttp.go:18-119`。

**界定：**已确认的是受管理员权限保护的出站请求能力，未确认普通用户/匿名用户可利用为任意 SSRF。Cron Shell 和 WireGuard hook 也是管理员有意配置的宿主命令能力，并非本次证实的匿名命令注入。

**建议：**统一封装安全出站客户端；默认拒绝 loopback、私网、链路本地、IPv6 ULA 和云元数据地址，重定向逐跳校验，连接绑定到已校验解析结果；限制响应体、并发与时间；容器层再限制出站网络。

#### 3.11 前端站点认证登录支持任意外部 Redirect

`Login.tsx` 接受任意 `http(s)` `redirect` 参数，并在认证后直接跳转：`webpage/src/pages/Login.tsx:158-161,186-200`。可被构造为登录后跳转到钓鱼站点的入口。

**建议：**重定向目标使用服务端登记的允许域名/目标 ID，验证 scheme、host、端口和路径；拒绝协议相对 URL、userinfo、编码绕过与任意外域。

#### 3.12 CORS 通配符与凭据模式组合存在危险配置

配置 `NETPANEL_ALLOWED_ORIGINS=*` 时，代码会回显任意 Origin 并启用 credentials：`backend/api/middleware/auth.go:256-299`。默认未配置时较安全；这不是本次确认的 JWT 绕过，但会扩大跨域访问风险。

**建议：**启动配置拒绝 `*` + credentials 组合；配置来源必须做精确 origin 匹配；补充跨域和 Cookie 行为测试。

## 4. 静态攻击场景推演

以下是基于源码的安全验证场景，不是真实网络利用记录：

| 场景 | 推演步骤 | 预期/当前结果 | 严重度 |
|---|---|---|---|
| Viewer 读取 Agent Token | 用普通有效 JWT 请求 `GET /api/v1/monitor/servers` | 路由仅 JWT 认证，handler 返回完整模型，`AgentToken` 为 JSON 普通字符串，存在泄露 | 高 |
| Viewer 读取证书私钥 | 用普通有效 JWT 请求 `GET /api/v1/domain/certs` | 返回完整证书模型，`KeyContent` 未由 `Secret` 脱敏，存在泄露 | 高 |
| AI 对话 IDOR | 用户 A 请求其他用户的对话 ID 并调用消息列表/删除 | handler 与服务按 ID 全局查询，无所有者过滤，存在跨用户访问 | 高（多用户） |
| 首管抢注 | 新实例开放后，在合法管理员完成 setup 前提交 `POST /api/v1/init/setup` | 公开 setup 路由可创建首个管理员，风险限首启窗口 | 高（条件触发） |
| ReadOnly 绕过 | 对标记 `ReadOnly=true` 的 WebDAV 配置使用有效凭据提交写请求 | handler 未检查该设置，具备写入路径；需协议集成测试确定库行为 | 中高 |
| CF Tunnel 下载 | 浏览器用 EventSource 对 `/api/v1/cftunnel/download` 发 GET | 前端 EventSource 是 GET；后端只注册管理员 POST，方法不匹配，无法正常下载 | 功能阻断 |
| 证书下载 | 点击下载按钮打开 `/api/v1/domain/certs/{id}/download` | 前端有 URL，但后端 router 未注册对应 GET 路由；打开新窗口也不带 Axios Bearer header | 功能阻断 |
| 管理员 SSRF/探测 | 管理员调用 Mesh ping，目标指向内网 URL | 后端代发出站请求；有管理员权限时可用于内网连通性探测。没有确认低权限可触发 | 中（管理员能力） |
| Logout 后重放旧 JWT | Logout 后复用先前复制的 Bearer token | Logout 不使 JWT token version 失效，令牌到期/其他失效条件前仍可能有效 | 中 |

**真实攻击边界：**本次没有启动面板、没有触发宿主机命令/防火墙/隧道/存储服务，也没有访问公网或外部系统。SFTP 越界、WebDAV 符号链接越界、动态 CORS、OAuth Token 实际落日志等属于后续动态验证项，不能表述为已利用。

## 5. 前端体验、逻辑与性能

### 已确认的功能缺陷/体验断点

1. **CF Tunnel 下载失效：**页面使用 `EventSource` GET，后端下载路由为 admin POST；浏览器 EventSource 也不能按 Axios 封装添加 Bearer header。位置：`webpage/src/pages/CfTunnel.tsx:167-205`、`webpage/src/api/index.ts:99-112`、`backend/api/router.go:396-408`。
2. **证书下载失效：**`DomainCert.tsx` 打开 `/domain/certs/:id/download`，路由没有对应 GET；新窗口不自动附带 Axios Bearer token。位置：`webpage/src/pages/DomainCert.tsx:543-547`、`backend/api/router.go:325-338`。
3. **初始化向导按钮语义冲突：**首步按钮同时是 submit 和“下一步”点击操作，可能在进入安全建议步骤时就触发 `handleFinish`；第二步再调用 `form.submit()`，而表单本身已被条件渲染卸载。验证规则和步骤状态容易错位。位置：`webpage/src/pages/InitSetup.tsx:107-169`。需用浏览器回归测试确认具体事件时序，并将每步提交行为明确分开。
4. **Dashboard 把请求失败当成零配置：**服务列表某项请求失败时，直接落为 stopped/count=0；用户无法区分“未配置”和“API 出错/无权限”。位置：`webpage/src/pages/Dashboard.tsx:466-484,541-562`。
5. **轮询负载偏高：**Dashboard 每轮并发请求约 15 个服务列表，再请求网卡、防火墙、WAF 等数据；10 秒轮询不等待上轮完成，慢网络时可能请求重叠。位置：`webpage/src/pages/Dashboard.tsx:462-520,541-575`。
6. **AI SSE 解析可能丢片段：**按 fetch 的每个流数据块直接按换行拆分，未缓存跨 chunk 的半行，解析异常被忽略。位置：`webpage/src/pages/AiChat.tsx:148-210`。
7. **登录外跳**见安全风险 3.11；**角色菜单体验**方面前端主要检查是否存在 Token，菜单并不按角色过滤，页面可能显示随后被后端 403 的操作。后端仍是最终权限边界，未将其误报为单纯的前端提权。
8. **移动端布局不足：**固定侧栏/左边距，缺少窄屏抽屉替代；登录页固定宽度，监控统计栅格无小屏断点。位置：`webpage/src/layouts/MainLayout.tsx:282-305,386-406`、`webpage/src/pages/Login.tsx:227-234,352-369`、`webpage/src/pages/MonitorDashboard.tsx:267-308`。
9. **视觉一致性：**Ant Design 5 提供统一控件基础，但页面混用 Design Token 与大量手写样式，多个相似隧道页面和 Dashboard 卡片造成信息密度偏高。适合功能型后台，但目前更像功能聚合工具箱，不够突出“安全暴露一个服务”的核心任务。
10. **状态与内容问题：**初始化状态请求失败时按“已初始化”处理，用户缺少重试恢复入口；帮助页存在没有实际导航行为的卡片；Onboarding 页面有路由但未接入登录后的首次用户路径。对应 `InitSetup.tsx:21-30`、`App.tsx:95-98`、`Onboarding.tsx:25-45,168-187`、`Help.tsx:10-24,33-55`。
11. **本地化与策略一致性：**i18n 框架存在，但初始化、帮助、用户管理等仍有大量硬编码中文；密码最小长度规则在不同入口不一致；版本展示也需统一。参考 `webpage/src/i18n/index.ts:6-20`、`InitSetup.tsx:119-125`、`UserManagement.tsx:344-352`、`Settings.tsx:193-205`、`MainLayout.tsx:371-380`。

### 性能判断

- 前端路由采用懒加载是正向选择，初始 bundle 可避免包含所有管理页。
- Dashboard 以多列表接口拼总览，用户每打开一屏数据就加载大量配置列表；应使用精简聚合 API、服务端缓存/摘要、分页与增量刷新。
- 多个页面存在高频刷新，建议页面不可见时暂停，采用有界并发、请求去重/取消、指数退避，避免打开多个标签后负载叠加。
- 后端 SQLite + WAL 适合单机/低并发；监控历史、系统日志、配置全表查询需要统一 retention、分页和索引策略。需用实际并发/数据规模压测后再判断何时拆分数据库。
- 本次没有浏览器截图、Lighthouse、性能剖析或设备测试，因此评分是结构判断，不是实测指标；视觉精细度与真实移动端体验应在代表页面跑浏览器审查。

## 6. 产品功能合理性与行业对照

### 当前能力分组

1. **公网访问/穿透：**端口转发、STUN、FRP/NPS、Cloudflare Tunnel。
2. **私有组网：**EasyTier、WireGuard、Mesh、Mesh Tunnel。
3. **域名与入口：**DDNS、DNS 解析、ACME 证书、Caddy 反代、WAF、访问控制、防火墙。
4. **运行管理：**监控、日志、Shell/Cron、WebDAV/SFTP、WOL、DNSMasq、AI/MCP、回调。

每类都有需求基础，但各自的配置模型和导航入口目前倾向平行堆叠。新用户首先得理解 CGNAT、端口转发、STUN、FRP、Cloudflare Tunnel、Mesh、WireGuard 的差异，才知道点哪里；多个页面可以建立“隧道”，但缺少统一服务/资源对象。

### 在线资料带来的判断

- [OWASP Authorization Cheat Sheet](https://cheatsheetseries.owasp.org/cheatsheets/Authorization_Cheat_Sheet.html)：强调默认拒绝、每次请求授权、最小权限和对象级授权。对照 NetPanel，不能只依赖 auth/admin 两层和 UI 菜单，AI 对话所有权、普通用户能读秘密、角色回收都应纳入逐对象/逐动作策略。
- [OWASP SSRF Prevention Cheat Sheet](https://cheatsheetseries.owasp.org/cheatsheets/Server_Side_Request_Forgery_Prevention_Cheat_Sheet.html)：强调出站 URL 允许列表、逐跳重定向校验、DNS/IP 检查与网络层出站限制。NetPanel 的回调、探测、Cron HTTP、IPDB 导入需要共用安全客户端而不是逐功能自建。
- [Pangolin 官方 Sites 文档](https://docs.pangolin.net/manage/sites/understanding-sites)：把远端 Site、具体 Resource 与用户访问分开，默认 Site 不自动暴露流量，并强调资源级访问。可借鉴“定义服务资源，再选择公共/私有访问方式”，避免用户先选底层隧道协议。
- [NetBird 官方文档](https://docs.netbird.io/)：定位自托管 Zero Trust Networking，聚焦设备间加密覆盖网络和访问策略。NetPanel 的组网模块可以借鉴以身份/设备/资源策略为中心，而不是仅配置 VPN 参数。
- [Nginx Proxy Manager 官网](https://nginxproxymanager.com/)：聚焦 Proxy Host、Let's Encrypt 证书和容易上手的反向代理配置，证明单一任务的简洁体验有明确用户价值。NetPanel 可在广度之外把“一个服务安全上线”作为主要流程。
- [NetBox 官方文档](https://netboxlabs.com/docs/netbox/)：定位网络事实来源，重点是 IPAM/DCIM/网络建模和 API；它与 NetPanel 是相邻产品而非直接替代。产品可考虑同步设备、网段、地址清单，不宜在没有用户验证前复制一整套 DCIM。
- [Netdata 官方告警文档](https://learn.netdata.cloud/docs/monitor/configure-alarms)：展示了阈值/时间窗、告警生命周期、通知路由、重复通知抑制和维护静默等成熟监控能力。NetPanel 监控建议围绕告警可信度、抑制噪音、维护窗口与通知闭环建设，而非先堆更多图表。

### 产品经理建议：优先做减法与流程化

#### P0：建立统一“服务资源”模型

用户只需描述“我要让哪个内网服务可访问”，系统采集协议、目标地址、网络条件、是否公网公开、允许用户、域名和安全等级；底层再映射到 CF Tunnel、Caddy、FRP、端口转发、Mesh 等驱动。避免独立模块各自保存近似目标、凭证、健康状态。

#### P1：做真正端到端首次成功向导

建议路径：

1. 探测公网 IPv4/IPv6、CGNAT、UDP 可达性、TUN 权限、DNS 与外部端口。
2. 根据条件给出 1 个首选方案及 1 个备选方案，并解释代价与暴露面，不让用户先选协议。
3. 创建服务后自动配置域名/证书/访问控制，展示实际生效规则。
4. 从外部探测健康、证书、DNS 和可达性，失败时给出诊断步骤。
5. 生成可撤销的分享权限或连接凭据，并记录创建人、变更和审计事件。

#### P1：建立统一安全中心，而非更多零散开关

提供公网暴露资产清单、监听端口、反代目标、证书到期、弱配置、存储只读状态、凭据轮换时间、最近权限变更和高风险能力（Shell、WireGuard hooks、完整容器权限）提示。支持一键撤销、变更审计、备份恢复演练和安全配置导出。

#### P1：完成 RBAC 与资源级分享

定义明确的 viewer/operator/admin 权限矩阵；operator 可操作已授权服务但不可更改宿主全局安全设置；共享链接/临时访问策略有期限、用途限制和撤销能力。API 测试覆盖角色 × 资源 × 动作矩阵，前端再用同一能力模型隐藏不可用操作。

#### P2：运维闭环优先于新增集成

补齐告警规则、通知渠道、事件去重/静默、维护窗口、执行历史、健康探测与统一审计时间线。提供异步任务进度接口和可恢复失败，不要依赖浏览器长连接/页面停留完成下载或运维作业。

#### P2：模块取舍建议

- 主导航围绕“服务、网络、域名、安全、运维”五个用户任务组织，不按底层第三方工具平铺所有入口。
- FRP/NPS/EasyTier/WireGuard/CF 作为“连接器/驱动”呈现；保持高级设置折叠，避免普通用户看到同级协议清单。
- Mesh Tunnel、独立协议页面、统一 TunService 的边界需要产品决策，优先合并重复入口和数据模型。
- IPAM/DCIM 先提供 NetBox 导入/同步，不直接扩成庞大的资产系统，除非目标用户明确要求。
- AI 建议必须说明将要改变的资源、影响范围和回滚方式；任何会触发宿主命令、防火墙或公网暴露的操作都要求预览与人工确认，默认只读诊断。

## 7. 推荐整改顺序

### 立即（发布阻断）

- 普通登录用户 API 中敏感字段改 DTO/白名单，并加入 viewer 回归测试。
- 缺失加密密钥时停止明文降级；迁移检查现有数据库与备份。
- 修复 AI 对话/助手对象授权和角色回收逻辑。
- WebDAV/SFTP 落实 ReadOnly 与根目录隔离。
- 首次初始化加一次性引导保护；生产 HTTP/TLS 和容器最小权限默认值安全化。

### 下一迭代（稳定性）

- 修复 CF 下载和证书下载 API 契约。
- 修复初始化向导的表单提交/步骤状态；补浏览器测试。
- 修正 Dashboard 的错误态语义、轮询节流与取消。
- 修复 AI SSE chunk 缓冲、外部登录 redirect 白名单。
- 统一密码策略、角色菜单、版本和中英文文案；完善移动端导航。

### 后续（产品差异化）

- 做服务资源统一模型、方案推荐、外部连通性验证、安全暴露资产表。
- 做告警通知与静默、审计、备份恢复和凭据轮换闭环。
- 根据用户研究决定协议适配器、Mesh 与 TunService 的合并边界。

## 8. 后续动态验证清单

在隔离的测试容器和临时数据目录进行，不对生产或公网实例操作：

1. 建立 admin/editor/viewer 账户，逐一调用所有列表、详情、导出、下载 API，断言任何低权限响应不含密码、token、私钥。
2. 在独立测试数据库分别测试密钥缺失、长度错误、密钥轮换与历史明文迁移，确认生产模式失败关闭。
3. 用户 A/B 创建 AI 对话，交换 ID 测试读、更新、删除、发消息、导出均拒绝。
4. 对 WebDAV 发 GET/PUT/MKCOL/DELETE/MOVE；对 SFTP 发相对 `..`、绝对路径、符号链接请求；设置只读与可写两组配置比较。
5. 测试初始化竞态与第一次 setup 保护；测试 Logout 后旧 JWT、改密后旧 JWT、禁用账号和降权 token。
6. 对所有 URL 出站功能测试 loopback、RFC1918、IPv6 ULA、链路本地/metadata、DNS rebinding、重定向到私网、大响应体、慢响应。
7. 浏览器端回归 CF 下载、证书下载、初始化流程、OAuth redirect、SSE 分块边界；执行桌面/移动视口与键盘可访问性检查。
8. 在处理 lockfile 合并后运行前端 build/test/lint、Go tests/vet/race（适合的套件）、依赖漏洞扫描和 Lighthouse；保留 SBOM 与扫描结果。

## 9. 参考链接

- OWASP Authorization Cheat Sheet: https://cheatsheetseries.owasp.org/cheatsheets/Authorization_Cheat_Sheet.html
- OWASP SSRF Prevention Cheat Sheet: https://cheatsheetseries.owasp.org/cheatsheets/Server_Side_Request_Forgery_Prevention_Cheat_Sheet.html
- Pangolin Sites: https://docs.pangolin.net/manage/sites/understanding-sites
- NetBird documentation: https://docs.netbird.io/
- Nginx Proxy Manager: https://nginxproxymanager.com/
- NetBox documentation: https://netboxlabs.com/docs/netbox/
- Netdata alert configuration: https://learn.netdata.cloud/docs/monitor/configure-alarms

## 10. 2026-10-28 整改阶段复审

### 已复核的整改

- 敏感配置列表响应补充字段脱敏；证书与私钥的下载仅由管理员路由提供，并使用 `no-store`、安全响应头和下载审计。
- 登录回跳限于面板同源路径，或服务端已登记、启用且受页面登录/OIDC 规则保护的 Caddy 站点；校验 scheme/host/port、拒绝 IP literal/userinfo/异常通配层级，并对每个受保护路由附带 Host 白名单以阻断 Host 头重定向。OAuth 将校验后的回跳放入 CSRF 绑定的、每流程独立 Cookie 的 `state`，回调后再次校验。Caddy 会话增加签名的 `token_version`，登出/权限撤销后旧平台会话失效；已认证但不在站点 ACL 中的用户返回 403，不再循环跳登录。
- Caddy 页面登录、OIDC 与 Basic Auth 配置在 Provider、用户列表、规则查询、IPDB、ACL/站点绑定或重复规则异常时失败关闭；无效认证模式和空的 Basic Auth 账号集不会退化成无认证代理。
- Gin 与 Caddy 共用严格 IP/CIDR、IPDB 缺失/空条目检查；全局规则读取、IPDB/站点查询、模式或绑定数据异常时返回拒绝结果，不将坏配置当作无规则继续放行。空 IP 白名单、无站点 OIDC、未知认证/IP 模式均有失败关闭回归用例。
- EAB HMAC Secret 更新时保留空字段对应的旧值；初始化状态未知时阻止进入受保护页面；Dashboard 明确部分失败状态、隐藏标签暂停轮询，并采用请求去重与失败退避。
- 引导与帮助入口接入已有功能路由；登录后首次进入引导，菜单组按路由同步展开。

### 本轮验证

| 检查 | 结果 |
|---|---|
| `go test ./... -skip 'TestForwarderE2E_DataPlane\|TestForwarderE2E_MultiplePackets\|TestTCPForwardKeepaliveAndRelay'` | 通过；跳过 3 个受 Windows 套接字复用限制的 STUN 数据面集成用例 |
| `go vet ./...` | 通过 |
| `npm run build` | 通过；Vite 提示现有若干 chunk 大于 500 kB |
| IDE 诊断、`git diff --check` | 未发现新增 lint 错误或补丁空白错误 |
| Playwright 隔离浏览器回归 | 未执行：当前工作区未安装 Playwright CLI/依赖，未安装新依赖；也未启动应用实例 |
| `webpage/package-lock.json` | 本轮未读写；最终 `git status`、未合并索引与冲突标记检查均未发现该文件改动或未解决冲突 |

### 剩余风险与发布门槛

- 当前验证为源码测试与构建，不等同于隔离服务的端到端/动态渗透测试；仍应按第 8 节执行证书下载、OAuth 浏览器回跳、初始化并发、SSE 分块及存储协议场景。
- 三个 STUN 数据面用例需在支持其 UDP/socket 行为的 Linux 隔离环境复测。
- Go 全量测试与前端构建通过不代表适合直接公网部署；上线前仍需逐项执行权限矩阵、密钥备份/迁移、TLS/可信代理和容器能力检查。
