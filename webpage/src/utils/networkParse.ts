/**
 * 网络地址解析工具函数
 * 从 EasytierClient.tsx / EasytierServer.tsx 提取的公共工具
 */

export const genRpcPort = (): string => String(Math.floor(Math.random() * 10000) + 15000)

export const genNetworkName = (): string => Math.random().toString(36).slice(2, 10)

export const genNetworkPassword = (): string =>
  Math.random().toString(36).slice(2, 10) + Math.random().toString(36).slice(2, 10)

export const parseAddrStr = (s: string): { proto: string; host: string; port: string } => {
  s = s.trim()
  const m = s.match(/^(\w+):\/\/(.+):(\d+)$/)
  if (m) return { proto: m[1], host: m[2], port: m[3] }
  const parts = s.split(':')
  if (parts.length === 3) return { proto: parts[0], host: parts[1], port: parts[2] }
  if (parts.length === 2) return { proto: 'tcp', host: parts[0], port: parts[1] }
  return { proto: 'tcp', host: s, port: '' }
}

export const serializeAddr = (item: { proto: string; host: string; port: string }): string => {
  if (!item?.host) return ''
  return `${item.proto || 'tcp'}://${item.host}:${item.port || ''}`
}

export const parseAddrList = (str: string): Array<{ proto: string; host: string; port: string }> => {
  if (!str) return [{ proto: 'tcp', host: '', port: '' }]
  return str.split(',').map(s => parseAddrStr(s)).filter(i => i.host)
}

export const parseListenPorts = (s: string): Array<{ proto: string; port: string }> => {
  if (!s) return []
  return s.split(',').map(p => {
    p = p.trim()
    if (p.includes(':')) {
      const [proto, port] = p.split(':')
      return { proto, port }
    }
    return { proto: 'tcp', port: p }
  }).filter((p): p is { proto: string; port: string } => Boolean(p))
}

export const parseListenPortsRaw = (s: string): string[] => {
  if (!s) return []
  return s.split(',').map(p => p.trim()).filter(Boolean)
}

export const joinListenPorts = (ports: string[]): string => (ports || []).filter(Boolean).join(',')

export const parseSimpleList = (str: string): Array<{ value: string }> => {
  if (!str) return []
  return str.split(',').map(s => s.trim()).filter(Boolean).map(s => ({ value: s }))
}

export const parsePortForwards = (str: string): Array<{
  proto: string; listen_ip: string; listen_port: string; target_ip: string; target_port: string
}> => {
  if (!str) return []
  return str.split('\n').map(s => s.trim()).filter(Boolean).map(s => {
    const p = s.split(':')
    if (p.length >= 5) return { proto: p[0], listen_ip: p[1], listen_port: p[2], target_ip: p[3], target_port: p[4] }
    return { proto: 'tcp', listen_ip: '0.0.0.0', listen_port: '', target_ip: '', target_port: '' }
  })
}

export const randomStr = (len: number, chars = 'abcdefghijklmnopqrstuvwxyz0123456789'): string =>
  Array.from({ length: len }, () => chars[Math.floor(Math.random() * chars.length)]).join('')
