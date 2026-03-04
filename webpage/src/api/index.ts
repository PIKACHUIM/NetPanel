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
}

// ===== EasyTier 服务端 =====
export const easytierServerApi = {
  list: () => request.get('/v1/easytier/server'),
  create: (data: any) => request.post('/v1/easytier/server', data),
  update: (id: number, data: any) => request.put(`/v1/easytier/server/${id}`, data),
  delete: (id: number) => request.delete(`/v1/easytier/server/${id}`),
  start: (id: number) => request.post(`/v1/easytier/server/${id}/start`),
  stop: (id: number) => request.post(`/v1/easytier/server/${id}/stop`),
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
