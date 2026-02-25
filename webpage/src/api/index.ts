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

// ===== 域名证书 =====
export const domainCertApi = {
  list: () => request.get('/v1/domain/certs'),
  create: (data: any) => request.post('/v1/domain/certs', data),
  update: (id: number, data: any) => request.put(`/v1/domain/certs/${id}`, data),
  delete: (id: number) => request.delete(`/v1/domain/certs/${id}`),
  apply: (id: number) => request.post(`/v1/domain/certs/${id}/apply`),
}

// ===== 域名解析 =====
export const domainRecordApi = {
  list: (accountId?: number) => request.get('/v1/domain/records', { params: { account_id: accountId } }),
  create: (data: any) => request.post('/v1/domain/records', data),
  update: (id: number, data: any) => request.put(`/v1/domain/records/${id}`, data),
  delete: (id: number) => request.delete(`/v1/domain/records/${id}`),
  sync: (accountId: number) => request.post(`/v1/domain/records/sync/${accountId}`),
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
  batchImport: (data: any) => request.post('/v1/ipdb/import', data),
  query: (ip: string) => request.get('/v1/ipdb/query', { params: { ip } }),
}

// ===== 访问控制 =====
export const accessApi = {
  list: () => request.get('/v1/access'),
  create: (data: any) => request.post('/v1/access', data),
  update: (id: number, data: any) => request.put(`/v1/access/${id}`, data),
  delete: (id: number) => request.delete(`/v1/access/${id}`),
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
