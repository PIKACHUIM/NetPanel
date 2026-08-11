import request from './request'

// ===== 端口转发 =====
export const portForwardApi = {
  list: () => request.get('/v1/port-forward'),
  create: (data: any) => request.post('/v1/port-forward', data),
  update: (id: number, data: any) => request.put(`/v1/port-forward/${id}`, data),
  delete: (id: number) => request.delete(`/v1/port-forward/${id}`),
  start: (id: number) => request.post(`/v1/port-forward/${id}/start`),
  stop: (id: number) => request.post(`/v1/port-forward/${id}/stop`),
  getLogs: (id: number) => request.get(`/v1/port-forward/${id}/logs`),
  listCerts: () => request.get('/v1/port-forward/certs'),
}

// ===== STUN =====
export const stunApi = {
  list: () => request.get('/v1/stun'),
  create: (data: any) => request.post('/v1/stun', data),
  update: (id: number, data: any) => request.put(`/v1/stun/${id}`, data),
  delete: (id: number) => request.delete(`/v1/stun/${id}`),
  start: (id: number) => request.post(`/v1/stun/${id}/start`),
  stop: (id: number) => request.post(`/v1/stun/${id}/stop`),
  getStatus: (id: number) => request.get(`/v1/stun/${id}/status`),
}

// ===== FRP 客户端 =====
export const frpcApi = {
  list: () => request.get('/v1/frpc'),
  create: (data: any) => request.post('/v1/frpc', data),
  update: (id: number, data: any) => request.put(`/v1/frpc/${id}`, data),
  delete: (id: number) => request.delete(`/v1/frpc/${id}`),
  start: (id: number) => request.post(`/v1/frpc/${id}/start`),
  stop: (id: number) => request.post(`/v1/frpc/${id}/stop`),
  restart: (id: number) => request.post(`/v1/frpc/${id}/restart`),
}

// ===== FRP 服务端 =====
export const frpsApi = {
  list: () => request.get('/v1/frps'),
  create: (data: any) => request.post('/v1/frps', data),
  update: (id: number, data: any) => request.put(`/v1/frps/${id}`, data),
  delete: (id: number) => request.delete(`/v1/frps/${id}`),
  start: (id: number) => request.post(`/v1/frps/${id}/start`),
  stop: (id: number) => request.post(`/v1/frps/${id}/stop`),
}

// ===== NPS 服务端 =====
export const npsServerApi = {
  list: () => request.get('/v1/nps/server'),
  create: (data: any) => request.post('/v1/nps/server', data),
  update: (id: number, data: any) => request.put(`/v1/nps/server/${id}`, data),
  delete: (id: number) => request.delete(`/v1/nps/server/${id}`),
  start: (id: number) => request.post(`/v1/nps/server/${id}/start`),
  stop: (id: number) => request.post(`/v1/nps/server/${id}/stop`),
}

// ===== NPS 客户端 =====
export const npsClientApi = {
  list: () => request.get('/v1/nps/client'),
  create: (data: any) => request.post('/v1/nps/client', data),
  update: (id: number, data: any) => request.put(`/v1/nps/client/${id}`, data),
  delete: (id: number) => request.delete(`/v1/nps/client/${id}`),
  start: (id: number) => request.post(`/v1/nps/client/${id}/start`),
  stop: (id: number) => request.post(`/v1/nps/client/${id}/stop`),
  // 隧道管理（子表，参考 nps 隧道类型）
  listTunnels: (clientId: number) => request.get(`/v1/nps/client/${clientId}/tunnels`),
  createTunnel: (clientId: number, data: any) => request.post(`/v1/nps/client/${clientId}/tunnels`, data),
  updateTunnel: (clientId: number, tunnelId: number, data: any) => request.put(`/v1/nps/client/${clientId}/tunnels/${tunnelId}`, data),
  deleteTunnel: (clientId: number, tunnelId: number) => request.delete(`/v1/nps/client/${clientId}/tunnels/${tunnelId}`),
}

// ===== EasyTier 客户端 =====
export const easytierClientApi = {
  list: () => request.get('/v1/easytier/client'),
  create: (data: any) => request.post('/v1/easytier/client', data),
  update: (id: number, data: any) => request.put(`/v1/easytier/client/${id}`, data),
  delete: (id: number) => request.delete(`/v1/easytier/client/${id}`),
  start: (id: number) => request.post(`/v1/easytier/client/${id}/start`),
  stop: (id: number) => request.post(`/v1/easytier/client/${id}/stop`),
  getStatus: (id: number) => request.get(`/v1/easytier/client/${id}/status`),
  getLogs: (id: number) => request.get(`/v1/easytier/client/${id}/logs`),
  getPeers: (id: number) => request.get(`/v1/easytier/client/${id}/peers`),
}

// ===== EasyTier 服务端 =====
export const easytierServerApi = {
  list: () => request.get('/v1/easytier/server'),
  create: (data: any) => request.post('/v1/easytier/server', data),
  update: (id: number, data: any) => request.put(`/v1/easytier/server/${id}`, data),
  delete: (id: number) => request.delete(`/v1/easytier/server/${id}`),
  start: (id: number) => request.post(`/v1/easytier/server/${id}/start`),
  stop: (id: number) => request.post(`/v1/easytier/server/${id}/stop`),
  getLogs: (id: number) => request.get(`/v1/easytier/server/${id}/logs`),
  getPeers: (id: number) => request.get(`/v1/easytier/server/${id}/peers`),
}

// ===== Cloudflare Tunnel =====
export const cftunnelApi = {
  list: () => request.get('/v1/cftunnel'),
  create: (data: any) => request.post('/v1/cftunnel', data),
  update: (id: number, data: any) => request.put(`/v1/cftunnel/${id}`, data),
  delete: (id: number) => request.delete(`/v1/cftunnel/${id}`),
  start: (id: number) => request.post(`/v1/cftunnel/${id}/start`),
  stop: (id: number) => request.post(`/v1/cftunnel/${id}/stop`),
  getStatus: (id: number) => request.get(`/v1/cftunnel/${id}/status`),
  getLogs: (id: number) => request.get(`/v1/cftunnel/${id}/logs`),
  getBinaryPath: () => request.get('/v1/cftunnel/binary'),
  getDownloadInfo: () => request.get('/v1/cftunnel/download/info'),
  downloadBinary: () => '/api/v1/cftunnel/download', // 返回 SSE 流式 URL
}

// ===== 穿透服务（统一内网穿透管理） =====
export const tunserviceApi = {
  list: () => request.get('/v1/tunservice'),
  get: (id: number) => request.get(`/v1/tunservice/${id}`),
  create: (data: any) => request.post('/v1/tunservice', data),
  update: (id: number, data: any) => request.put(`/v1/tunservice/${id}`, data),
  delete: (id: number) => request.delete(`/v1/tunservice/${id}`),
  start: (id: number) => request.post(`/v1/tunservice/${id}/start`),
  stop: (id: number) => request.post(`/v1/tunservice/${id}/stop`),
  candidates: (id: number) => request.get(`/v1/tunservice/${id}/candidates`),
  history: (id: number, limit = 100) => request.get(`/v1/tunservice/${id}/history?limit=${limit}`),
}

// ===== WireGuard =====
export const wireguardApi = {
  list: () => request.get('/v1/wireguard'),
  create: (data: any) => request.post('/v1/wireguard', data),
  update: (id: number, data: any) => request.put(`/v1/wireguard/${id}`, data),
  delete: (id: number) => request.delete(`/v1/wireguard/${id}`),
  start: (id: number) => request.post(`/v1/wireguard/${id}/start`),
  stop: (id: number) => request.post(`/v1/wireguard/${id}/stop`),
  getStatus: (id: number) => request.get(`/v1/wireguard/${id}/status`),
  generateKeyPair: () => request.post('/v1/wireguard/generate-keypair'),
  // 对等节点管理
  listPeers: (wgId: number) => request.get(`/v1/wireguard/${wgId}/peers`),
  createPeer: (wgId: number, data: any) => request.post(`/v1/wireguard/${wgId}/peers`, data),
  updatePeer: (wgId: number, peerId: number, data: any) => request.put(`/v1/wireguard/${wgId}/peers/${peerId}`, data),
  deletePeer: (wgId: number, peerId: number) => request.delete(`/v1/wireguard/${wgId}/peers/${peerId}`),
  // 对等节点配置文件下载 & 二维码（直接返回 URL，由前端触发下载/展示）
  getPeerConfigUrl: (wgId: number, peerId: number) => `/api/v1/wireguard/${wgId}/peers/${peerId}/config`,
  getPeerQRCodeUrl: (wgId: number, peerId: number) => `/api/v1/wireguard/${wgId}/peers/${peerId}/qrcode`,
}

// ===== DDNS =====
export const ddnsApi = {
  list: () => request.get('/v1/ddns'),
  create: (data: any) => request.post('/v1/ddns', data),
  update: (id: number, data: any) => request.put(`/v1/ddns/${id}`, data),
  delete: (id: number) => request.delete(`/v1/ddns/${id}`),
  start: (id: number) => request.post(`/v1/ddns/${id}/start`),
  stop: (id: number) => request.post(`/v1/ddns/${id}/stop`),
  runNow: (id: number) => request.post(`/v1/ddns/${id}/run`),
  getHistory: (id: number) => request.get(`/v1/ddns/${id}/history`),
}

// ===== Caddy =====
export const caddyApi = {
  list: () => request.get('/v1/caddy'),
  create: (data: any) => request.post('/v1/caddy', data),
  update: (id: number, data: any) => request.put(`/v1/caddy/${id}`, data),
  delete: (id: number) => request.delete(`/v1/caddy/${id}`),
  start: (id: number) => request.post(`/v1/caddy/${id}/start`),
  stop: (id: number) => request.post(`/v1/caddy/${id}/stop`),
}

// ===== WOL =====
export const wolApi = {
  list: () => request.get('/v1/wol'),
  create: (data: any) => request.post('/v1/wol', data),
  update: (id: number, data: any) => request.put(`/v1/wol/${id}`, data),
  delete: (id: number) => request.delete(`/v1/wol/${id}`),
  wake: (id: number) => request.post(`/v1/wol/${id}/wake`),
}

// ===== 域名账号 =====
export const domainAccountApi = {
  list: () => request.get('/v1/domain/accounts'),
  create: (data: any) => request.post('/v1/domain/accounts', data),
  update: (id: number, data: any) => request.put(`/v1/domain/accounts/${id}`, data),
  delete: (id: number) => request.delete(`/v1/domain/accounts/${id}`),
  test: (id: number) => request.post(`/v1/domain/accounts/${id}/test`),
}

// ===== 证书账号 =====
export const certAccountApi = {
  list: () => request.get('/v1/domain/cert-accounts'),
  create: (data: any) => request.post('/v1/domain/cert-accounts', data),
  update: (id: number, data: any) => request.put(`/v1/domain/cert-accounts/${id}`, data),
  delete: (id: number) => request.delete(`/v1/domain/cert-accounts/${id}`),
  verify: (id: number) => request.post(`/v1/domain/cert-accounts/${id}/verify`),
}

// ===== 域名证书 =====
export const domainCertApi = {
  list: () => request.get('/v1/domain/certs'),
  create: (data: any) => request.post('/v1/domain/certs', data),
  update: (id: number, data: any) => request.put(`/v1/domain/certs/${id}`, data),
  delete: (id: number) => request.delete(`/v1/domain/certs/${id}`),
  apply: (id: number) => request.post(`/v1/domain/certs/${id}/apply`),
  renew: (id: number) => request.post(`/v1/domain/certs/${id}/renew`),
  getStatus: (id: number) => request.get(`/v1/domain/certs/${id}/status`),
  // ACME 分步操作
  stepCreateOrder: (id: number) => request.post(`/v1/domain/certs/${id}/step/create-order`),
  stepSetDNS: (id: number) => request.post(`/v1/domain/certs/${id}/step/set-dns`),
  stepValidate: (id: number) => request.post(`/v1/domain/certs/${id}/step/validate`),
  stepObtain: (id: number) => request.post(`/v1/domain/certs/${id}/step/obtain`),
  // 手动确认 DNS 已设置
  confirmDNS: (id: number) => request.post(`/v1/domain/certs/${id}/confirm-dns`),
  // 解析 PEM 证书内容，返回域名列表
  parseCert: (data: { cert_content: string }) => request.post('/v1/domain/certs/parse', data),
}

// ===== 域名管理（域名列表）=====
export const domainInfoApi = {
  list: (params?: { account_id?: number; keyword?: string }) => request.get('/v1/domain/domains', { params }),
  create: (data: any) => request.post('/v1/domain/domains', data),
  update: (id: number, data: any) => request.put(`/v1/domain/domains/${id}`, data),
  delete: (id: number) => request.delete(`/v1/domain/domains/${id}`),
  refresh: (id: number) => request.post(`/v1/domain/domains/${id}/refresh`),
  // 更新自动同步配置（触发后端定时器注册/取消）
  updateAutoSync: (id: number, data: { auto_sync: boolean; sync_interval: number }) =>
    request.put(`/v1/domain/domains/${id}/auto-sync`, data),
  // 从服务商拉取账号下的域名列表（含已添加状态）
  fetchFromProvider: (accountId: number) => request.get('/v1/domain/domains/fetch', { params: { account_id: accountId } }),
}

// ===== 域名解析（子域名解析记录）=====
export const domainRecordApi = {
  list: (params?: { domain_info_id?: number; account_id?: number }) => request.get('/v1/domain/records', { params }),
  create: (data: any) => request.post('/v1/domain/records', data),
  update: (id: number, data: any) => request.put(`/v1/domain/records/${id}`, data),
  delete: (id: number) => request.delete(`/v1/domain/records/${id}`),
  sync: (domainInfoId: number) => request.post(`/v1/domain/records/sync/${domainInfoId}`),
}

// ===== DNSMasq =====
export const dnsmasqApi = {
  getConfig: () => request.get('/v1/dnsmasq/config'),
  updateConfig: (data: any) => request.put('/v1/dnsmasq/config', data),
  start: () => request.post('/v1/dnsmasq/start'),
  stop: () => request.post('/v1/dnsmasq/stop'),
  listRecords: () => request.get('/v1/dnsmasq/records'),
  createRecord: (data: any) => request.post('/v1/dnsmasq/records', data),
  updateRecord: (id: number, data: any) => request.put(`/v1/dnsmasq/records/${id}`, data),
  deleteRecord: (id: number) => request.delete(`/v1/dnsmasq/records/${id}`),
}

// ===== 计划任务 =====
export const cronApi = {
  list: () => request.get('/v1/cron'),
  create: (data: any) => request.post('/v1/cron', data),
  update: (id: number, data: any) => request.put(`/v1/cron/${id}`, data),
  delete: (id: number) => request.delete(`/v1/cron/${id}`),
  enable: (id: number) => request.post(`/v1/cron/${id}/enable`),
  disable: (id: number) => request.post(`/v1/cron/${id}/disable`),
  runNow: (id: number) => request.post(`/v1/cron/${id}/run`),
}

// ===== 网络存储 =====
export const storageApi = {
  list: () => request.get('/v1/storage'),
  create: (data: any) => request.post('/v1/storage', data),
  update: (id: number, data: any) => request.put(`/v1/storage/${id}`, data),
  delete: (id: number) => request.delete(`/v1/storage/${id}`),
  start: (id: number) => request.post(`/v1/storage/${id}/start`),
  stop: (id: number) => request.post(`/v1/storage/${id}/stop`),
}

// ===== IP 地址库 =====
export const ipdbApi = {
  list: (params?: any) => request.get('/v1/ipdb', { params }),
  create: (data: any) => request.post('/v1/ipdb', data),
  update: (id: number, data: any) => request.put(`/v1/ipdb/${id}`, data),
  delete: (id: number) => request.delete(`/v1/ipdb/${id}`),
  // 手动批量导入（文本格式，每行支持多个 IP/CIDR，空格/逗号/分号分隔）
  batchImport: (data: { entries?: any[], text?: string, location?: string, tags?: string }) => request.post('/v1/ipdb/import', data),
  // 从URL下载导入
  importFromUrl: (data: { url: string, location?: string, tags?: string, clear_first?: boolean }) => request.post('/v1/ipdb/import-url', data),
  // 查询IP归属地
  query: (ip: string) => request.get('/v1/ipdb/query', { params: { ip } }),
  // 订阅管理
  listSubscriptions: () => request.get('/v1/ipdb/subscriptions'),
  createSubscription: (data: any) => request.post('/v1/ipdb/subscriptions', data),
  updateSubscription: (id: number, data: any) => request.put(`/v1/ipdb/subscriptions/${id}`, data),
  deleteSubscription: (id: number) => request.delete(`/v1/ipdb/subscriptions/${id}`),
  refreshSubscription: (id: number) => request.post(`/v1/ipdb/subscriptions/${id}/refresh`),
}

// ===== 访问控制 =====
export const accessApi = {
  list: () => request.get('/v1/access'),
  create: (data: any) => request.post('/v1/access', data),
  update: (id: number, data: any) => request.put(`/v1/access/${id}`, data),
  delete: (id: number) => request.delete(`/v1/access/${id}`),
}

// ===== WAF 防火墙 =====
export const wafApi = {
  list: () => request.get('/v1/security/waf'),
  create: (data: any) => request.post('/v1/security/waf', data),
  update: (id: number, data: any) => request.put(`/v1/security/waf/${id}`, data),
  delete: (id: number) => request.delete(`/v1/security/waf/${id}`),
  start: (id: number) => request.post(`/v1/security/waf/${id}/start`),
  stop: (id: number) => request.post(`/v1/security/waf/${id}/stop`),
  getLogs: (id: number, params?: any) => request.get(`/v1/security/waf/${id}/logs`, { params }),
  testRule: (id: number, data: any) => request.post(`/v1/security/waf/${id}/test`, data),
}

// ===== 系统防火墙 =====
export const firewallApi = {
  list: () => request.get('/v1/security/firewall'),
  create: (data: any) => request.post('/v1/security/firewall', data),
  update: (id: number, data: any) => request.put(`/v1/security/firewall/${id}`, data),
  delete: (id: number) => request.delete(`/v1/security/firewall/${id}`),
  // 应用规则到系统防火墙
  apply: (id: number) => request.post(`/v1/security/firewall/${id}/apply`),
  // 从系统防火墙移除规则（不删除数据库记录）
  remove: (id: number) => request.post(`/v1/security/firewall/${id}/remove`),
  // 检测当前系统防火墙后端
  detectBackend: () => request.get('/v1/security/firewall/backend'),
  // 触发异步同步系统防火墙规则到数据库
  syncSystem: () => request.post('/v1/security/firewall/sync-system'),
  // 获取同步状态（syncing/last_sync_at/last_sync_err/total）
  getSyncStatus: () => request.get('/v1/security/firewall/sync-status'),
}

// ===== 回调账号 =====
export const callbackAccountApi = {
  list: () => request.get('/v1/callback/accounts'),
  create: (data: any) => request.post('/v1/callback/accounts', data),
  update: (id: number, data: any) => request.put(`/v1/callback/accounts/${id}`, data),
  delete: (id: number) => request.delete(`/v1/callback/accounts/${id}`),
  test: (id: number) => request.post(`/v1/callback/accounts/${id}/test`),
}

// ===== 回调任务 =====
export const callbackTaskApi = {
  list: () => request.get('/v1/callback/tasks'),
  create: (data: any) => request.post('/v1/callback/tasks', data),
  update: (id: number, data: any) => request.put(`/v1/callback/tasks/${id}`, data),
  delete: (id: number) => request.delete(`/v1/callback/tasks/${id}`),
}

// ===== 系统 =====
export const systemApi = {
  getInfo: () => request.get('/v1/system/info'),
  getStats: () => request.get('/v1/system/stats'),
  getConfig: () => request.get('/v1/system/config'),
  updateConfig: (data: any) => request.put('/v1/system/config', data),
  changePassword: (data: any) => request.post('/v1/system/change-password', data),
  getInterfaces: () => request.get('/v1/system/interfaces'),
  login: (data: any) => request.post('/v1/auth/login', data),
}

// ===== 系统管理（日志 + 用户）=====
export const adminApi = {
  // 日志查看
  queryLogs: (params?: any) => request.get('/v1/admin/logs', { params }),
  getLogServices: () => request.get('/v1/admin/logs/services'),
  cleanupLogs: (days: number) => request.delete('/v1/admin/logs', { params: { days } }),
  // 用户管理
  listUsers: () => request.get('/v1/admin/users'),
  createUser: (data: any) => request.post('/v1/admin/users', data),
  updateUser: (id: number, data: any) => request.put(`/v1/admin/users/${id}`, data),
  deleteUser: (id: number) => request.delete(`/v1/admin/users/${id}`),
  getCurrentUser: () => request.get('/v1/admin/users/me'),
}

// ===== 组网节点管理 =====
export const meshNodeApi = {
  // 节点管理
  listNodes: () => request.get('/v1/mesh/nodes'),
  createNode: (data: any) => request.post('/v1/mesh/nodes', data),
  getNode: (id: number) => request.get(`/v1/mesh/nodes/${id}`),
  updateNode: (id: number, data: any) => request.put(`/v1/mesh/nodes/${id}`, data),
  deleteNode: (id: number) => request.delete(`/v1/mesh/nodes/${id}`),
  checkNode: (id: number) => request.post(`/v1/mesh/nodes/${id}/check`),
  // 拓扑
  getTopology: () => request.get('/v1/mesh/topology'),
  // 事件
  listEvents: (params?: any) => request.get('/v1/mesh/events', { params }),
  cleanEvents: (days: number) => request.delete('/v1/mesh/events', { params: { days } }),
  // Ping
  ping: (targetUrl: string) => request.post('/v1/mesh/ping', { target_url: targetUrl }),
  // 代理请求到远程节点
  proxyGet: (nodeId: number, path: string, params?: any) =>
    request.get(`/v1/mesh/proxy/${nodeId}${path}`, { params }),
  proxyPost: (nodeId: number, path: string, data?: any) =>
    request.post(`/v1/mesh/proxy/${nodeId}${path}`, data),
  proxyPut: (nodeId: number, path: string, data?: any) =>
    request.put(`/v1/mesh/proxy/${nodeId}${path}`, data),
  proxyDelete: (nodeId: number, path: string) =>
    request.delete(`/v1/mesh/proxy/${nodeId}${path}`),
}

// ===== 远程节点隧道 API 工厂 =====
// 根据隧道类型和节点ID，生成与本地 API 接口兼容的 proxy 版本
// 当 nodeId 为 0 或 isLocal 为 true 时，直接使用本地 API
const TUNNEL_API_PATHS: Record<string, string> = {
  'port-forward': '/port-forward',
  'stun': '/stun',
  'frpc': '/frpc',
  'frps': '/frps',
  'nps-client': '/nps/client',
  'nps-server': '/nps/server',
  'easytier-client': '/easytier/client',
  'easytier-server': '/easytier/server',
}

const LOCAL_APIS: Record<string, any> = {
  'port-forward': portForwardApi,
  'stun': stunApi,
  'frpc': frpcApi,
  'frps': frpsApi,
  'nps-client': npsClientApi,
  'nps-server': npsServerApi,
  'easytier-client': easytierClientApi,
  'easytier-server': easytierServerApi,
}

export function createRemoteTunnelApi(tunnelType: string, nodeId: number, isLocal: boolean) {
  // 本机节点直接使用本地 API
  if (isLocal) {
    return LOCAL_APIS[tunnelType] || null
  }

  const basePath = TUNNEL_API_PATHS[tunnelType]
  if (!basePath) return null

  return {
    list: () => meshNodeApi.proxyGet(nodeId, basePath),
    create: (data: any) => meshNodeApi.proxyPost(nodeId, basePath, data),
    update: (id: number, data: any) => meshNodeApi.proxyPut(nodeId, `${basePath}/${id}`, data),
    delete: (id: number) => meshNodeApi.proxyDelete(nodeId, `${basePath}/${id}`),
    start: (id: number) => meshNodeApi.proxyPost(nodeId, `${basePath}/${id}/start`),
    stop: (id: number) => meshNodeApi.proxyPost(nodeId, `${basePath}/${id}/stop`),
    restart: (id: number) => meshNodeApi.proxyPost(nodeId, `${basePath}/${id}/restart`),
    // FRP 客户端代理管理
    listProxies: (frpcId: number) => meshNodeApi.proxyGet(nodeId, `/frpc/${frpcId}/proxies`),
    createProxy: (frpcId: number, data: any) => meshNodeApi.proxyPost(nodeId, `/frpc/${frpcId}/proxies`, data),
    updateProxy: (frpcId: number, proxyId: number, data: any) => meshNodeApi.proxyPut(nodeId, `/frpc/${frpcId}/proxies/${proxyId}`, data),
    deleteProxy: (frpcId: number, proxyId: number) => meshNodeApi.proxyDelete(nodeId, `/frpc/${frpcId}/proxies/${proxyId}`),
    // NPS 客户端隧道管理
    listTunnels: (clientId: number) => meshNodeApi.proxyGet(nodeId, `/nps/client/${clientId}/tunnels`),
    createTunnel: (clientId: number, data: any) => meshNodeApi.proxyPost(nodeId, `/nps/client/${clientId}/tunnels`, data),
    updateTunnel: (clientId: number, tunnelId: number, data: any) => meshNodeApi.proxyPut(nodeId, `/nps/client/${clientId}/tunnels/${tunnelId}`, data),
    deleteTunnel: (clientId: number, tunnelId: number) => meshNodeApi.proxyDelete(nodeId, `/nps/client/${clientId}/tunnels/${tunnelId}`),
    // 端口转发证书
    listCerts: () => meshNodeApi.proxyGet(nodeId, '/port-forward/certs'),
    // STUN 状态
    getStatus: (id: number) => meshNodeApi.proxyGet(nodeId, `${basePath}/${id}/status`),
    // 端口转发日志
    getLogs: (id: number) => meshNodeApi.proxyGet(nodeId, `${basePath}/${id}/logs`),
    // EasyTier 节点信息
    getPeers: (id: number) => meshNodeApi.proxyGet(nodeId, `${basePath}/${id}/peers`),
  }
}

// ===== AI 管理 =====

// AI API 来源
export const aiProviderApi = {
  list: () => request.get('/v1/ai/providers'),
  create: (data: any) => request.post('/v1/ai/providers', data),
  update: (id: number, data: any) => request.put(`/v1/ai/providers/${id}`, data),
  delete: (id: number) => request.delete(`/v1/ai/providers/${id}`),
  fetchModels: (id: number) => request.post(`/v1/ai/providers/${id}/fetch-models`),
  test: (id: number) => request.post(`/v1/ai/providers/${id}/test`),
}

// AI 对话
export const aiChatApi = {
  listConversations: () => request.get('/v1/ai/conversations'),
  createConversation: (data: any) => request.post('/v1/ai/conversations', data),
  updateConversation: (id: number, data: any) => request.put(`/v1/ai/conversations/${id}`, data),
  deleteConversation: (id: number) => request.delete(`/v1/ai/conversations/${id}`),
  listMessages: (id: number) => request.get(`/v1/ai/conversations/${id}/messages`),
  sendMessage: (id: number, content: string) => request.post(`/v1/ai/conversations/${id}/send`, { content }),
  exportConversation: (id: number) => request.get(`/v1/ai/conversations/${id}/export`),
  importConversation: (data: any) => request.post('/v1/ai/conversations/import', data),
}

// AI 助理
export const aiAssistantApi = {
  list: () => request.get('/v1/ai/assistants'),
  create: (data: any) => request.post('/v1/ai/assistants', data),
  update: (id: number, data: any) => request.put(`/v1/ai/assistants/${id}`, data),
  delete: (id: number) => request.delete(`/v1/ai/assistants/${id}`),
}

// AI 定时任务
export const aiCronTaskApi = {
  list: () => request.get('/v1/ai/cron-tasks'),
  create: (data: any) => request.post('/v1/ai/cron-tasks', data),
  update: (id: number, data: any) => request.put(`/v1/ai/cron-tasks/${id}`, data),
  delete: (id: number) => request.delete(`/v1/ai/cron-tasks/${id}`),
  enable: (id: number) => request.post(`/v1/ai/cron-tasks/${id}/enable`),
  disable: (id: number) => request.post(`/v1/ai/cron-tasks/${id}/disable`),
  run: (id: number) => request.post(`/v1/ai/cron-tasks/${id}/run`),
  getLogs: (id: number, limit?: number) => request.get(`/v1/ai/cron-tasks/${id}/logs`, { params: { limit } }),
}

// AI 插件
export const aiPluginApi = {
  list: (type?: string) => request.get('/v1/ai/plugins', { params: { type } }),
  create: (data: any) => request.post('/v1/ai/plugins', data),
  update: (id: number, data: any) => request.put(`/v1/ai/plugins/${id}`, data),
  delete: (id: number) => request.delete(`/v1/ai/plugins/${id}`),
  toggle: (id: number, isActive: boolean) => request.post(`/v1/ai/plugins/${id}/toggle`, { is_active: isActive }),
}

// ===== 服务监控 =====
export const monitorApi = {
  // 服务器管理
  listServers: (params?: any) => request.get('/v1/monitor/servers', { params }),
  getServer: (id: number) => request.get(`/v1/monitor/servers/${id}`),
  createServer: (data: any) => request.post('/v1/monitor/servers', data),
  updateServer: (id: number, data: any) => request.put(`/v1/monitor/servers/${id}`, data),
  deleteServer: (id: number) => request.delete(`/v1/monitor/servers/${id}`),
  syncFromMeshNode: (nodeId: number) => request.post(`/v1/monitor/servers/sync/${nodeId}`),
  
  // 监控指标
  getLatestMetrics: (id: number) => request.get(`/v1/monitor/servers/${id}/metrics/latest`),
  getMetricsHistory: (id: number, params: any) => request.get(`/v1/monitor/servers/${id}/metrics/history`, { params }),
  
  // 服务探测
  listProbes: (params?: any) => request.get('/v1/monitor/probes', { params }),
  createProbe: (data: any) => request.post('/v1/monitor/probes', data),
  updateProbe: (id: number, data: any) => request.put(`/v1/monitor/probes/${id}`, data),
  deleteProbe: (id: number) => request.delete(`/v1/monitor/probes/${id}`),
  getProbeResults: (id: number, params?: any) => request.get(`/v1/monitor/probes/${id}/results`, { params }),
  
  // 任务管理
  listTasks: (params?: any) => request.get('/v1/monitor/tasks', { params }),
  createTask: (data: any) => request.post('/v1/monitor/tasks', data),
  updateTask: (id: number, data: any) => request.put(`/v1/monitor/tasks/${id}`, data),
  deleteTask: (id: number) => request.delete(`/v1/monitor/tasks/${id}`),
  executeTask: (id: number) => request.post(`/v1/monitor/tasks/${id}/execute`),
  getTaskLogs: (params?: any) => request.get('/v1/monitor/tasks/logs', { params }),
  
  // 告警规则
  listAlerts: (params?: any) => request.get('/v1/monitor/alerts', { params }),
  createAlert: (data: any) => request.post('/v1/monitor/alerts', data),
  updateAlert: (id: number, data: any) => request.put(`/v1/monitor/alerts/${id}`, data),
  deleteAlert: (id: number) => request.delete(`/v1/monitor/alerts/${id}`),
  getAlertRecords: (params?: any) => request.get('/v1/monitor/alerts/records', { params }),
  
  // DDNS 绑定
  listDDNSBindings: (params?: any) => request.get('/v1/monitor/ddns', { params }),
  createDDNSBinding: (data: any) => request.post('/v1/monitor/ddns', data),
  updateDDNSBinding: (id: number, data: any) => request.put(`/v1/monitor/ddns/${id}`, data),
  deleteDDNSBinding: (id: number) => request.delete(`/v1/monitor/ddns/${id}`),
  triggerDDNSUpdate: (id: number) => request.post(`/v1/monitor/ddns/${id}/trigger`),
  
  // 通知渠道
  listNotifications: (params?: any) => request.get('/v1/monitor/notifications', { params }),
  createNotification: (data: any) => request.post('/v1/monitor/notifications', data),
  updateNotification: (id: number, data: any) => request.put(`/v1/monitor/notifications/${id}`, data),
  deleteNotification: (id: number) => request.delete(`/v1/monitor/notifications/${id}`),
  sendTestNotification: (data: any) => request.post('/v1/monitor/notifications/test', data),
  
  // 隧道绑定
  listTunnelBindings: (params?: any) => request.get('/v1/monitor/tunnels', { params }),
  createTunnelBinding: (data: any) => request.post('/v1/monitor/tunnels', data),
  updateTunnelBinding: (id: number, data: any) => request.put(`/v1/monitor/tunnels/${id}`, data),
  deleteTunnelBinding: (id: number) => request.delete(`/v1/monitor/tunnels/${id}`),
  syncTunnelStatus: (id: number) => request.post(`/v1/monitor/tunnels/${id}/sync`),
}