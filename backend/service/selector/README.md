# selector — 多工具穿透线路统一抽象 + 自动测速选线

NetPanel 内置 frp / nps / easytier / wireguard 等多个穿透工具，各自维护自己的
Manager。本包提供**线路层**的公共抽象：任何工具只要把它的可用线路描述为
`[]Line`（地址 + 可选 HTTP 204 探测地址），就能接入统一的并发测速、自动选线
（容差防抖）、手动锁线与后台守护。**零外部依赖**（仅标准库），可独立测试。

## 线路模型

```go
type Line struct {
    ID        string // 线路唯一标识（如 "frp:1" / "nps:2"）
    Name      string // 展示名
    Tool      string // 来源工具（frp / nps / easytier / wireguard / cloudflare…）
    Layer     string // ""（端口层）或 "domain"（域名层，如 Cloudflare Tunnel）
    Address   string // host:port，做 TCP 握手延迟测量
    ProbeURL  string // 可选；非空时额外做一次 HTTP 204 出网探测
}
```

- **端口层**（默认）：只做 TCP 握手延迟，最快、最普适。
- **域名层**（`Layer: "domain"`）：`Address` 是域名层入口，未配置 `ProbeURL`
  时自动探测 `https://<host>`。

## 测速指标（ProbeResult）

单条线路的一次测速返回两个延迟：

| 字段 | 含义 |
|---|---|
| `TCPLatency`  | TCP 握手延迟（毫秒级，快速筛掉不通/高延迟线路） |
| `HTTPLatency` | HTTP 204 出网探测延迟（仅配置了 `ProbeURL` 时非 0） |

**排序使用 `effectiveLatency`**：配置了 `ProbeURL` 且 HTTP 探测成功时，**以
HTTP 出网延迟为准**（它反映真实的出网可用性）；否则回退到 TCP 握手延迟。

## 选线算法

一次 `Select()` 的完整决策流程：

```mermaid
flowchart TD
    A[Select] --> B{有手动锁线?}
    B -- 是 --> C{锁线线路可用?}
    C -- 是 --> D[返回锁线线路 Locked=true]
    C -- 否 --> E[自动解除锁线]
    B -- 否 --> E
    E --> F[bestUsable: 可用线路中 effectiveLatency 最小者]
    F --> G{存在可用线路?}
    G -- 否 --> H[返回空 Selection]
    G -- 是 --> I{当前线路仍可用 且<br/>lat(cur) - lat(best) ≤ tolerance?}
    I -- 是 --> J[保持当前线路 不切换 防抖]
    I -- 否 --> K[切换到 best]
    J --> L[返回 Selection]
    K --> L
```

### 三条核心规则

1. **手动锁线优先**：`Lock(lineID)` 后完全跳过自动选线，除非被锁线路失效
   （探测不可用且连续失败达阈值），此时自动解除锁并退回自动模式。
2. **最快可用优先**：未锁线时，在**可用**线路中取 `effectiveLatency` 最小者。
   可用 = 本次探测成功；或失败但连续失败次数未达 `failureThreshold` 且有最近
   成功结果兜底。
3. **容差防抖（hysteresis）**：当前线路仍可用且不比最优慢超过 `tolerance`
   （默认 50ms）时**保持现状不切换**，避免两条相近线路上来回抖动。只有差距
   超过容差（或当前线路失效）才真正切换。

### 失败阈值（failureThreshold）

- `SetFailureThreshold(n)`：连续失败达 n 次才判线路不可用（默认 1）。
- 失败但未达阈值时，用**最近一次成功结果**的延迟参与排序（`latencyFor` 兜底），
  避免瞬时抖动把延迟抬到异常高。
- 无任何成功记录的线路始终视为不可用。

### 后台守护建议

`ProbeAll(ctx)` 并发探测全部线路（信号量限流 `SetMaxConcurrent`，默认 8），
返回 `map[lineID]ProbeResult` 并更新连续失败计数与最近成功缓存。调用方通常
用 goroutine + ticker 周期调用（如 60s），再调用 `Select()` 取当前选择；
`Snapshot()` 提供线程安全的线路/选择/锁线状态快照供 UI 展示。

## 并发安全

`Selector` 内部用 `sync.Mutex` 保护全部状态；`ProbeAll` / `Select` / `Lock` /
`Unlock` / `SetLines` / `Snapshot` 均可并发调用。`TCPProber.Probe` 本身并发安全。

## 测试

`selector_test.go` 通过**注入式 `Prober` 假实现**（`fakeProber`）模拟线路质量
变化，不依赖真实网络，覆盖：

- 选最快 / HTTP 延迟优先 / 跳过不可用 / 全部不可用
- 容差防抖（相近线路不抖动、差距超容差才切换）
- 手动锁线优先 / 锁线失效自动解除 / 未知锁线忽略
- `SetLines` 清理过期结果与失效锁线 / 快照线程安全
- 并发限流（信号量生效）
- 失败阈值：瞬时失败保持线路、持续失败切换、恢复回到最快、无兜底记录判不可用
- `domainHost` 解析

运行：`cd backend && go test ./service/selector/... -v`
