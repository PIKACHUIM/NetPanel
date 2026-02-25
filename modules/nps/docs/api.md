# NPS Web API 文档

**注意：**  
在使用 Web API 前，请确保在 `nps.conf` 中配置了有效的 `auth_key`，并取消其注释。

## Web API 验证机制

每次 API 请求都需附带两个参数：

- **`auth_key`** ：
  - 生成规则：`auth_key = md5(配置文件中的auth_key + 当前时间戳)`
  - 示例（Java Hutool工具）：

```java
Long time = new Date().getTime() / 1000;
String authKey = MD5.create().digestHex("your_auth_key_here" + time.toString());
System.out.println(authKey);
```

- **`timestamp`** ：当前 Unix 时间戳（秒级）。

**示例请求：**

```bash
curl -X POST \
  --url http://127.0.0.1:8080/client/list \
  --data 'auth_key=your_generated_auth_key&timestamp=current_unix_timestamp&start=0&limit=10'
```

**安全提醒：** 为保障安全性，每次请求的时间戳有效范围为 20 秒内。

## 获取服务端时间

**接口：** `POST /auth/gettime`

- **返回值**：当前服务端 Unix 时间戳（单位：秒）。

## 获取服务端 authKey

**接口：** `POST /auth/getauthkey`

- **返回值**：AES CBC 加密后的 authKey。
- **注意事项**：
  - 需使用配置文件中的 `auth_crypt_key`（必须为16位字符）解密。
  - AES CBC 解密（128位，pkcs5padding，十六进制编码）。
    - 解密密钥长度128
    - 偏移量与密钥相同
    - 补码方式pkcs5padding
    - 解密串编码方式 十六进制

## 获取服务端证书

**接口：** `POST /auth/getcert`

- **返回值**：NPS的证书公钥。

## 仪表盘与导航接口

### 仪表盘页面

- **接口：** `GET /index/index`
- **功能**：渲染仪表盘页面，展示服务概览。

### 帮助页面

- **接口：** `GET /index/help`
- **功能**：提供使用帮助信息。

### 隧道导航页面

以下接口均使用 `GET` 请求渲染对应隧道类型页面：

| URL             | 类型说明      |
|-----------------|-----------|
| `/index/tcp`    | TCP 隧道    |
| `/index/udp`    | UDP 隧道    |
| `/index/socks5` | Socks5 隧道 |
| `/index/http`   | HTTP 代理   |
| `/index/file`   | 文件服务      |
| `/index/secret` | 私密代理      |
| `/index/p2p`    | P2P 隧道    |
| `/index/host`   | 域名解析      |
| `/index/all`    | 按客户端展示    |

## 隧道管理接口

### 获取隧道列表

- **接口：** `POST /index/gettunnel`
- **请求参数**：
  | 参数 | 说明 |
  |------|------|
  | `client_id` | 需要查询的客户端 ID（整数） |
  | `type` | 隧道类型（`tcp`, `udp`, `httpProxy`, `socks5`, `secret`, `p2p`） |
  | `search` | 关键词搜索（字符串） |
  | `sort` | 排序字段（如 `id`） |
  | `order` | 排序方式（`asc` 或 `desc`） |
  | `offset` | 分页起始位置（整数） |
  | `limit` | 每页显示条数（整数） |

### 添加/修改隧道

- **添加接口：** `POST /index/add`
- **修改接口：** `POST /index/edit`
- **请求参数**：
  | 参数 | 说明 |
  |------|------|
  | `client_id` | 关联客户端 ID（整数） |
  | `port` | 服务器监听端口，若 ≤0 则自动分配 |
  | `server_ip` | 服务器 IP 地址 |
  | `type` | 隧道类型（`tcp`, `udp`, `httpProxy`, `socks5`, `secret`, `p2p`） |
  | `target` | 目标地址（如 `127.0.0.1:8080`，支持多行换行 `\n`） |
  | `flow_reset` | 是否重置流量（`0` 否，`1` 是） |
  | `flow_limit` | 流量限制（单位 MB，空则不限制） |
  | `time_limit` | 时间限制（字符串，空则不限制） |
  | `proxy_protocol` | 代理协议标识（整数） |
  | `local_proxy` | 是否启用本地代理（`0` 否，`1` 是） |
  | `remark` | 隧道备注（字符串） |
  | `password` | 访问隧道的密码（字符串） |
  | `local_path` | 本地路径（适用于文件服务） |
  | `strip_pre` | URL 前缀转换（字符串） |
  | `id` | 隧道 ID（修改时必填） |

### 单个隧道操作

- **获取详情**：`POST /index/getonetunnel`，参数 `id`（隧道 ID）
- **启动隧道**：`POST /index/start`，参数 `id`（隧道 ID）
- **停止隧道**：`POST /index/stop`，参数 `id`（隧道 ID）
- **删除隧道**：`POST /index/del`，参数 `id`（隧道 ID）

## 域名解析管理接口

### 获取域名解析列表

- **接口：** `POST /index/hostlist`
- **请求参数**：
  | 参数 | 说明 |
  |------|------|
  | `search` | 搜索关键词（可以搜索域名、备注等） |
  | `offset` | 分页起始位置（整数） |
  | `limit` | 每页显示条数（整数） |
  | `client_id` | 需要查询的客户端 ID（整数） |

### 添加/修改域名解析

- **添加接口：** `POST /index/addhost`
- **修改接口：** `POST /index/edithost`
- **请求参数**：
  | 参数 | 说明 |
  |------|------|
  | `client_id` | 关联的客户端 ID（整数） |
  | `host` | 域名（如 `example.com`） |
  | `target` | 内网目标（`ip:端口`，支持多个，用\n分隔） |
  | `flow_reset` | 是否重置流量（`0` 否，`1` 是） |
  | `flow_limit` | 流量限制（单位 MB，空则不限制） |
  | `time_limit` | 时间限制（字符串，空则不限制） |
  | `proxy_protocol` | 代理协议标识（整数） |
  | `local_proxy` | 是否启用本地代理（`0` 否，`1` 是） |
  | `header` | 修改的请求头（字符串） |
  | `hostchange` | 修改的 `Host` 值（字符串） |
  | `remark` | 备注信息（字符串） |
  | `location` | URL 路由（字符串，空则不限制） |
  | `scheme` | 协议类型（`all`、`http`、`https`） |
  | `https_just_proxy` | 是否仅代理 HTTPS（`0` 否，`1` 是） |
  | `key_file` | HTTPS 证书密钥文本或路径（字符串） |
  | `cert_file` | HTTPS 证书公钥文本或路径（字符串） |
  | `auto_https` | 是否自动启用 HTTPS（`0` 否，`1` 是） |
  | `auto_cors` | 是否自动添加 CORS 头（`0` 否，`1` 是） |
  | `target_is_https` | 目标是否为 HTTPS（`0` 否，`1` 是） |
  | `id` | 域名解析 ID（修改时必填） |

### 单个域名解析操作

- **获取详情**：`POST /index/gethost`（参数 `id`）
- **删除域名解析**：`POST /index/delhost`（参数 `id`）

## 客户端管理接口

### 获取客户端列表

- **接口：** `POST /client/list`
- **请求参数**：
  | 参数 | 说明 |
  |------|------|
  | `search` | 搜索关键字（字符串） |
  | `order` | 排序方式（`asc` 正序，`desc` 倒序） |
  | `offset` | 分页起始位置（整数） |
  | `limit` | 每页显示条数（整数） |

### 添加/修改客户端

- **添加接口：** `POST /client/add`
- **修改接口：** `POST /client/edit`
- **请求参数**：
  | 参数 | 说明 |
  |------|------|
  | `remark` | 备注信息（字符串） |
  | `u` | Basic 认证用户名（字符串） |
  | `p` | Basic 认证密码（字符串） |
  | `vkey` | 客户端验证密钥（字符串） |
  | `config_conn_allow` | 是否允许客户端以配置文件模式连接（`0` 否，`1` 是） |
  | `compress` | 是否启用数据压缩（`0` 否，`1` 是） |
  | `crypt` | 是否启用加密（`0` 否，`1` 是） |
  | `flow_reset` | 是否重置流量（`0` 否，`1` 是） |
  | `flow_limit` | 流量限制（单位 MB，空则不限制） |
  | `time_limit` | 时间限制（字符串，空则不限制） |
  | `rate_limit` | 带宽限制（单位 KB/s，空则不限制） |
  | `max_conn` | 最大连接数量（整数，空则不限制） |
  | `max_tunnel` | 最大隧道数量（整数，空则不限制） |
  | `id` | 客户端 ID（修改时必填） |

### 单个客户端操作

- **获取详情**：`POST /client/getclient`（参数 `id`）
- **延迟检查**：`POST /client/pingclient`（参数 `id`）
- **修改状态**：`POST /client/changestatus`（参数 `id`、`status`）（`0` 否，`1` 是）
- **删除客户端**：`POST /client/del`（参数 `id`）

## 用户认证接口

### 用户登录

- **接口：** `POST /login/verify`
- **请求参数**：
  | 参数 | 说明 |
  |------|------|
  | `username` | 登录用户名（字符串） |
  | `password` | 通过 RSA 公钥加密的 JSON 字符串（包含 n: nonce、t: 毫秒级时间戳、p: 明文密码） |
  | `captcha` | 验证码（可选，依据配置决定是否需要） |

### 用户登出

- **接口：** `GET /login/out`

### 用户注册

- **接口：** `POST /login/register`
- **请求参数**：
  | 参数 | 说明 |
  |------|------|
  | `username` | 注册用户名（字符串） |
  | `password` | 注册密码（字符串） |

