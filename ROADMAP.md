# NetPanel 优化路线图 v2

> 2026-09 全项目重新梳理的产物。基于四轮深度调研（穿透链路、选线机制、前端旅程/工程健康度、后端架构、安全运维），把"降低内网穿透使用门槛 / 提升穿透速度质量"扩展为全项目的系统性优化计划。
>
> 执行进度以本文件为唯一权威清单，完成一项勾一项。

## 现状一句话

产品功能面广、安全意识在线（secret 派生、init 防绕过、CORS 白名单都有认真做），但存在 **5 个高危安全缺口**（含一条真实提权路径）、**三类系统性技术债**（横切面各写 N 遍 / 契约无人执行 / 资源边界缺失）、**前端四类债**（any 铺满 / CRUD 手抄 / i18n 半途 / 双轨主题）。穿透模块的 6 条断链已在 [PR #107](https://github.com/PIKACHUIM/NetPanel/pull/107) 修复。

## 执行波次

### 第一波：安全止血（最高优先，先于一切功能工作）

| # | 事项 | 证据 | 量级 |
|---|------|------|------|
| S1 | `PUT /system/config` 收进 admin 组 + 写入键白名单；移除 legacy `admin_password` 明文登录路径（同一遗留概念散落 7 文件一并清理） | `handlers/system.go:119-133`、`router.go:103`、`middleware/auth.go:59-75` | S |
| S2 | 宿主机级写操作统一收进 admin 组：wireguard / firewall / caddy 站点 / storage / easytier / frp / cftunnel 二进制下载 / mesh 代理 / AI 配置执行；用表驱动测试锁住鉴权矩阵 | `router.go:103-570` 逐组核对 | M |
| S3 | WireGuard `PreUp/PostUp` 等 hook 字段：admin-only 后保留，但界面加风险警示文案 | `models.go:1171-1175` | S |
| S4 | SMB 配置注入修复：share name / username 白名单字符校验（禁换行与 `]`），RootPath 绝对路径校验 | `storage/manager.go:316-351` | S |
| S5 | SFTP 沙箱：限制在配置的 RootPath 内；禁止空用户名匿名放行 | `storage/manager.go:140-149,250-252` | M |
| S6 | Secret 契约执行：handler Update 前用 `Secret.IsEmpty()` 保留旧值，修复"掩码回显提交 → 清空真实凭据"的数据破坏 bug；17 处敏感字段逐步迁到 `Secret` 类型 | `model/secret.go:48-50`、`meshnode.go:244-248` 等 | M |
| S7 | 登录防爆破：失败计数 + 指数退避（复用 speedtest 限流器模式）；`/init/setup` 同步加频控 | `handlers/auth.go:31-95` | S |
| S8 | CORS 配置脚枪：拒绝 `*` 与 credentials 并存；SQLite 文件收权 0600/目录 0700 | `middleware/auth.go:158-160`、`db.go:19` | S |
| S9 | CI 加门禁：golangci-lint（含 gofmt/gosec）+ race detector + 前端 ESLint，杜绝 gofmt 违规入库 | `pr-checks.yml:74-84` | S |

### 第二波：稳定性与资源边界（后端质量）

| # | 事项 | 证据 | 量级 |
|---|------|------|------|
| Q1 | 日志写入改造：每条日志一个 goroutine + 同步 SQLite 写 + 单连接池 → 日志风暴下无界堆积。改带缓冲 channel 的单 writer + 批量插入 + 丢弃计数 | `pkg/logger/logger.go:120`、`syslog/manager.go:25-33` | M |
| Q2 | 无界增长表加保留策略：MonitorMetric / WafLog 每日定时清理（仿 syslog.Cleanup 与 ProbeHistory prune 的现成模式） | `monitor/collector.go:75`、`waf.go:84` | S |
| Q3 | 优雅关闭补全：aiMgr/certMgr/accessMgr 等未注册关闭；`logger.Init()` 二次调用破坏 DBHook；封装 `go safe()` 统一 recover（全后端 54 处 goroutine 仅 1 处 recover） | `main.go:135,287-307,407`、`cert/manager.go:86-104` | M |
| Q4 | handler 层三件套统一：Update 前置存在性检查（现 Save-upsert 会静默插入）、统一 `parseUintParam`、统一响应包络（现三种并存） | `ddns.go:60`、`stun.go:59`、`ai.go:76` | M |
| Q5 | main.go 组装整理：`Service` 接口 + `[]namedService` 统一 Start/Stop（消灭 18 参 registerStopHandlers）；回调注入集中到 `wire()` | `main.go:407-495` | M |
| Q6 | SystemConfig 键注册表：同一组键 3 处定义、读取器 4 份实现 → 建 `pkg/syscfg` 单一来源 | `handlers/linereg.go:20-25` 等四处 | S |

### 第三波：新用户上手（原"第二批"，方向已确认）

| # | 事项 | 说明 | 量级 |
|---|------|------|------|
| A1 | 合并 `feat/onboarding-wizard` + `feat/nat-check`，打通 Onboarding 入口（登录后首访 / Dashboard 入口卡），实现"我没有公网 IP → 推荐 CF Tunnel/FRP"的场景化分流 | 两分支前后端已完整，缺入口接线 | M |
| A2 | Dashboard 卡片可点击 + "未配置 → 去配置"引导卡 | `Dashboard.tsx:208-232` | S |
| A3 | 菜单治理：删除 7 个重复菜单 key（port-mapping 组与 tunnel 组重复入口）、getOpenKeys 补 3 组、命名去缩写 | `MainLayout.tsx:87-150,564-576` | S |
| A4 | 表单校验前置 + "测试连接"按钮：STUN/FRP Create 目前零校验，错误延迟到运行期才暴露 | `handlers/stun.go:43-73`、`frp.go:83-96` | M |
| A5 | NAT 类型不利时给引导（Symmetric → 建议 FRP/CF），接入 nat-check 的检测能力 | `stun/manager.go:481-549` | S |
| A6 | AI 一键配置闭环：create→填参→启动→验证；AiChat 无 provider 时给配置页引导 | `ai/configurator.go:193-218` | M |
| A7 | 文档互链：Help 卡片可点、表单 tooltip 链接 docsite 对应页（docsite 21 篇 features 文档目前 UI 零引用） | `Help.tsx:34-42` | S |

### 第四波：穿透质量（原"第三批"）

| # | 事项 | 说明 | 量级 |
|---|------|------|------|
| B1 | 合并补完 `feat/health-failover`：健康态机已有后端+测试，缺 API 暴露、前端展示、接入 monitor 通知渠道 | 线路故障时用户无感知的问题 | M |
| B2 | 失败即重探：故障收敛从最坏 60s 压到秒级（失败事件触发立即重探而非等周期） | `linereg.go:400-418` | S |
| B3 | 选线落地去重：选线未变化时不重建 Caddy server / 不写 DNS（现每 60s 无条件 DELETE+PUT） | `linereg.go:499,570` | S |
| B4 | per-service 选线：服务在自身 LineRefs 内选最快（现全局最优不在 refs 内则该服务永不切换）；服务级锁线做可用性检查 | `linereg.go:441-453,496-497` | M |
| B5 | P2P 优先修正：精确相等 tie-break 改为阈值内（≤10ms）P2P 优先；接入 nat-check 实测分数替换配置启发式 | `selector.go:573-579` | S |
| B6 | Caddy transport 调优：keepalive/连接池/FlushInterval/上游健康检查（现切线后连接池全冷、SSE 被缓冲） | `caddy/manager.go:300-305` | S |
| B7 | 防抖容差改相对值：绝对 50ms 对低延迟线路压制切换 | `selector.go:199-201` | S |
| B8 | 合并 `feat/selector-weighted` 补接线：引擎已有，缺 Line 模型 Weight 字段/API/UI | 加权选线 | M |

### 第五波：工程化与体验基建

| # | 事项 | 说明 | 量级 |
|---|------|------|------|
| E1 | 前端类型化：以 `models.go`（单文件 1541 行，全部实体）为源生成 `src/api/types.ts`，api/index.ts 泛型化 + 拦截器 `ApiResponse<T>`；新增代码禁 any | 现 627 处 any、前后端契约零类型 | M |
| E2 | `useCrud<T>(api)` hook + FormModal 推广：消除 30+ 页 CRUD 手抄（约 2000 行）；先迁 Wol/Storage/AiPlugin/AiCronTask 四个标准页验证 | `FormModal.tsx` 已有仅 1 页用 | L |
| E3 | SSE 事件总线：后端 `/v1/events/stream` 广播 hub（ai.go 的 SSE 写法现成模板），前端 `useStatusStream` 替换 8 处 setInterval 轮询 | 状态滞后 5-30s → 秒级推送 | M |
| E4 | 构建优化：echarts 按需引入（省 ~800KB）、manualChunks 拆 vendor、MapleMono 6MB 字体子集化、删除 0 引用的 @ant-design/charts（11MB） | 首屏 1.09MB gzip → 预期砍半 | S |
| E5 | 后端错误消息国际化起步：错误码枚举 + 消息表（现 346 处硬编码中文 + 260 处裸 err.Error() 透传） | 长期项的第一步 | L |
| E6 | 运维件：install.sh 补 SHA256SUMS 校验（release 已产出校验文件，只差 10 行）、Dockerfile HEALTHCHECK、govulncheck 进 CI、依赖升级（gin/gorm/lego/beego EOL） | `install.sh:133-175` | M |
| E7 | 数据库迁移版本管理：至少加 schema_version 表（AutoMigrate + 一份手写迁移的现状已出过事故） | `db.go:133-215` 注释记录 | M |
| E8 | JWT 可吊销：短效 token + 改密/禁用时按 iat 拒绝；Logout 不再是空操作 | `auth.go:98-100` | M |

### 长期池（按需启动）

- i18n 全量铺开：2618 行硬编码中文、7 个零 i18n 页面（机制健康：zh/en key 集合零缺失，可脚本化抽取）
- monitor 与 selector 双套探测实现整合（抽公共 probe 包，monitor 告警接线路健康）
- 数据面探测：经隧道 HTTP 探测替代控制面 TCP 握手；ProbeHistory 长期降采样
- STUN 常驻保活已随 P0 完成；剩余：NATMAP 协议实现、IPv6 STUN 支持
- 主题双轨收敛：868 行 CSS/184 个 !important 逐步收进 ConfigProvider token
- mTLS 分支收敛（`feat/remote-lines` = `feat/frpmaster` = `feat/mtls` 内容重叠，需先决策取舍）
- 仓库治理：fork main 与上游 main 已分叉（fork 有 init 向导/caddy 修复，上游有 MCP 诊断等），建议把 fork 独有提交 PR 到上游后对齐两边 main

## 关键原则

1. **安全项永远插队**：波次顺序执行，但 S 级项发现即修，不等排期。
2. **先基建后铺开**：E1/E2（类型 + useCrud）完成前，不做大规模页面重构；否则没有安全网。
3. **每个波次以 PR 交付**，合并一个再开下一个，避免长分支再次腐化（PR #107 的分叉冲突教训）。
4. **WIP 分支先合并或先废弃**：onboarding-wizard / nat-check / health-failover / selector-weighted 内容健康可直接接续；`feat/line-trend`、`fix/caddy-port-conflict` 已进 main 可删；frpmaster/remote-lines/mtls 三选一。

## 附：调研数据基线（2026-09）

- 后端 115 个 Go 文件、58 个模型、21 处 exec 调用点、54 处 goroutine（1 处 recover）
- 前端 63 页面 24,407 行、627 处 any、2618 行硬编码中文、8 处轮询、dist 3.53MB JS + 15MB 静态资源
- 测试：后端 15 个测试文件（穿透链路最厚，24 个 service/handler 零测试）；前端 0 测试
- P0 断链修复：PR #107（6 项，含 STUN 真实转发实现）
